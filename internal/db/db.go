package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/heandroro/go-poc/internal/models"
)

type DB struct {
	*gorm.DB
}

func New(path string) (*DB, error) {
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// automigrate (includes join table `reservation_tables` for Reservation<>Table)
	if err := gdb.AutoMigrate(&models.Table{}, &models.Card{}, &models.Item{}, &models.Reservation{}); err != nil {
		return nil, err
	}

	return &DB{gdb}, nil
}
