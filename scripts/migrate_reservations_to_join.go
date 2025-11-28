package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/heandroro/go-poc/internal/models"
)

// This script migrates legacy reservations that used a single `table_id`
// column into the many-to-many join table `reservation_tables` used by the
// newer Reservation model (gorm:"many2many:reservation_tables;").
// Flags:
//   --db PATH     : path to sqlite DB (default: orders.db)
//   --dry-run     : don't perform writes; show what would be migrated
//   --backup      : create a timestamped backup before migrating (default: true)

func backupFile(src string) (string, error) {
	ts := time.Now().Format("20060102150405")
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	dst := filepath.Join(dir, fmt.Sprintf("%s.%s.bak", base, ts))

	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return dst, nil
}

func main() {
	dbPath := flag.String("db", "orders.db", "path to sqlite DB")
	dryRun := flag.Bool("dry-run", false, "do not modify DB; show what would be migrated")
	backup := flag.Bool("backup", true, "create timestamped backup of DB before migrating")
	flag.Parse()

	if *backup {
		if _, err := os.Stat(*dbPath); err == nil {
			b, err := backupFile(*dbPath)
			if err != nil {
				log.Fatalf("backup failed: %v", err)
			}
			fmt.Println("Backup created:", b)
		} else {
			fmt.Println("Backup skipped: DB file not found at", *dbPath)
		}
	}

	fmt.Println("Opening DB:", *dbPath)
	gdb, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}

	// AutoMigrate Reservation (and Table) to ensure the join table exists.
	if err := gdb.AutoMigrate(&models.Table{}, &models.Reservation{}); err != nil {
		log.Fatalf("automigrate failed: %v", err)
	}

	// Check whether the legacy `table_id` column exists on reservations.
	row := gdb.Raw("SELECT COUNT(1) FROM pragma_table_info('reservations') WHERE name='table_id'").Row()
	var hasTableID int64
	if err := row.Scan(&hasTableID); err != nil {
		log.Fatalf("failed to inspect table schema: %v", err)
	}
	if hasTableID == 0 {
		fmt.Println("No legacy column 'table_id' found on reservations — nothing to migrate.")
		return
	}

	// Read legacy rows with a non-zero table_id.
	type legacyRow struct {
		ID      uint
		TableID sql.NullInt64
	}

	var rows []legacyRow
	if err := gdb.Raw("SELECT id, table_id FROM reservations WHERE table_id IS NOT NULL AND table_id != 0").Scan(&rows).Error; err != nil {
		log.Fatalf("failed to read legacy reservations: %v", err)
	}

	if len(rows) == 0 {
		fmt.Println("No legacy reservation rows to migrate.")
		return
	}

	fmt.Printf("Found %d legacy reservation rows to migrate\n", len(rows))

	migrated := 0
	for _, r := range rows {
		if !r.TableID.Valid {
			continue
		}

		// Check if mapping already exists.
		var cnt int64
		if err := gdb.Raw("SELECT COUNT(1) FROM reservation_tables WHERE reservation_id = ? AND table_id = ?", r.ID, r.TableID.Int64).Scan(&cnt).Error; err != nil {
			log.Fatalf("count check failed: %v", err)
		}
		if cnt > 0 {
			continue
		}

		if *dryRun {
			fmt.Printf("DRY-RUN: would insert mapping reservation %d -> table %d\n", r.ID, r.TableID.Int64)
		} else {
			// Insert into join table. Use Exec to bypass GORM model complexity.
			if err := gdb.Exec("INSERT INTO reservation_tables(reservation_id, table_id) VALUES(?, ?)", r.ID, r.TableID.Int64).Error; err != nil {
				log.Fatalf("insert into reservation_tables failed for reservation %d table %d: %v", r.ID, r.TableID.Int64, err)
			}
			migrated++
		}
	}

	if *dryRun {
		fmt.Println("Dry-run complete — no changes were made.")
	} else {
		fmt.Printf("Migration complete — migrated %d rows\n", migrated)
	}
	// NOTE: this script does not drop or null the legacy `table_id` column.
	// Review and drop it manually if desired after verification.
}
