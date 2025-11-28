package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/heandroro/go-poc/internal/db"
	"github.com/heandroro/go-poc/internal/handlers"
	"github.com/heandroro/go-poc/internal/models"
)

func setupServer(t *testing.T) (*handlers.Handler, *chi.Mux, *db.DB, func()) {
	// in-memory sqlite
	dbPath := "file::memory:?cache=shared"
	d, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}

	h := handlers.NewHandler(d)
	r := chi.NewRouter()
	r.Post("/reservations", h.CreateReservation)
	r.Get("/reservations/{id}", h.GetReservation)
	r.Post("/reservations/{id}/checkin", h.CheckInReservation)
	r.Post("/reservations/{id}/checkout", h.CheckOutReservation)
	r.Get("/tables/{id}/reservations", h.ListReservationsForTable)

	// create some tables
	for i := 1; i <= 3; i++ {
		tbl := models.Table{Name: fmt.Sprintf("T%d", i), Status: "free"}
		if err := d.Create(&tbl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	cleanup := func() {
		nos, _ := d.DB.DB()
		nos.Close()
	}
	return h, r, d, cleanup
}

func TestReservationCreateConflictAndCheckinCheckout(t *testing.T) {
	_, r, d, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// create a reservation for table 1
	start := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	payload := map[string]interface{}{
		"table_ids":        []uint{1},
		"start_at":         start.Format(time.RFC3339),
		"end_at":           end.Format(time.RFC3339),
		"responsible_name": "Alice",
		"contact_phone":    "+5511999999999",
		"party_size":       4,
	}
	b, _ := json.Marshal(payload)
	res, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post reservation: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected created, got %d", res.StatusCode)
	}

	// attempt overlapping reservation should fail
	overlap := map[string]interface{}{
		"table_ids":        []uint{1},
		"start_at":         start.Add(30 * time.Minute).Format(time.RFC3339),
		"end_at":           end.Add(30 * time.Minute).Format(time.RFC3339),
		"responsible_name": "Bob",
		"contact_phone":    "+5511888888888",
		"party_size":       2,
	}
	b2, _ := json.Marshal(overlap)
	res2, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatalf("post overlap: %v", err)
	}
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", res2.StatusCode)
	}

	// fetch reservation id from first creation
	// read body
	var created models.Reservation
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// simulate checkin at start time by invoking checkin endpoint
	// For the handler, we need reservation start <= now; so update DB directly to set start_at to now-1m
	if err := d.Model(&models.Reservation{}).Where("id = ?", created.ID).Update("start_at", time.Now().Add(-1*time.Minute)).Error; err != nil {
		t.Fatalf("update start_at for checkin: %v", err)
	}

	// POST checkin
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/reservations/"+fmt.Sprint(created.ID)+"/checkin", nil)
	checkRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("checkin request failed: %v", err)
	}
	if checkRes.StatusCode != http.StatusOK {
		t.Fatalf("expected checkin 200, got %d", checkRes.StatusCode)
	}

	// POST checkout
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/reservations/"+fmt.Sprint(created.ID)+"/checkout", nil)
	coRes, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("checkout request failed: %v", err)
	}
	if coRes.StatusCode != http.StatusOK {
		t.Fatalf("expected checkout 200, got %d", coRes.StatusCode)
	}
}

func TestListReservationsForTable(t *testing.T) {
	_, r, _, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Test 1: List reservations for table with no reservations
	res, err := http.Get(srv.URL + "/tables/1/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 1: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var emptyList []models.Reservation
	if err := json.NewDecoder(res.Body).Decode(&emptyList); err != nil {
		t.Fatalf("decode empty list: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected empty list, got %d items", len(emptyList))
	}

	// Test 2: Create a reservation for table 1 and then list it
	start := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	payload := map[string]interface{}{
		"table_ids":        []uint{1},
		"start_at":         start.Format(time.RFC3339),
		"end_at":           end.Format(time.RFC3339),
		"responsible_name": "John Doe",
		"contact_phone":    "+5511999999999",
		"party_size":       4,
	}
	b, _ := json.Marshal(payload)
	createRes, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post reservation: %v", err)
	}
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected created, got %d", createRes.StatusCode)
	}

	// Now list reservations for table 1
	res2, err := http.Get(srv.URL + "/tables/1/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 1: %v", err)
	}
	if res2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res2.StatusCode)
	}
	var reservations []models.Reservation
	if err := json.NewDecoder(res2.Body).Decode(&reservations); err != nil {
		t.Fatalf("decode reservations: %v", err)
	}
	if len(reservations) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(reservations))
	}
	if reservations[0].ResponsibleName != "John Doe" {
		t.Fatalf("expected responsible name 'John Doe', got '%s'", reservations[0].ResponsibleName)
	}

	// Test 3: Create a second reservation for the same table (non-overlapping)
	start2 := end.Add(1 * time.Hour)
	end2 := start2.Add(2 * time.Hour)
	payload2 := map[string]interface{}{
		"table_ids":        []uint{1},
		"start_at":         start2.Format(time.RFC3339),
		"end_at":           end2.Format(time.RFC3339),
		"responsible_name": "Jane Smith",
		"contact_phone":    "+5511888888888",
		"party_size":       2,
	}
	b2, _ := json.Marshal(payload2)
	createRes2, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b2))
	if err != nil {
		t.Fatalf("post reservation 2: %v", err)
	}
	if createRes2.StatusCode != http.StatusCreated {
		t.Fatalf("expected created for reservation 2, got %d", createRes2.StatusCode)
	}

	// Now list reservations for table 1 - should have 2
	res3, err := http.Get(srv.URL + "/tables/1/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 1 after second reservation: %v", err)
	}
	if res3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res3.StatusCode)
	}
	var reservations2 []models.Reservation
	if err := json.NewDecoder(res3.Body).Decode(&reservations2); err != nil {
		t.Fatalf("decode reservations 2: %v", err)
	}
	if len(reservations2) != 2 {
		t.Fatalf("expected 2 reservations, got %d", len(reservations2))
	}

	// Test 4: List reservations for a different table (table 2) - should be empty
	res4, err := http.Get(srv.URL + "/tables/2/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 2: %v", err)
	}
	if res4.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for table 2, got %d", res4.StatusCode)
	}
	var reservations3 []models.Reservation
	if err := json.NewDecoder(res4.Body).Decode(&reservations3); err != nil {
		t.Fatalf("decode reservations for table 2: %v", err)
	}
	if len(reservations3) != 0 {
		t.Fatalf("expected 0 reservations for table 2, got %d", len(reservations3))
	}

	// Test 5: Create a reservation for multiple tables including table 2
	start3 := end2.Add(1 * time.Hour)
	end3 := start3.Add(2 * time.Hour)
	payload3 := map[string]interface{}{
		"table_ids":        []uint{1, 2},
		"start_at":         start3.Format(time.RFC3339),
		"end_at":           end3.Format(time.RFC3339),
		"responsible_name": "Multi Table",
		"contact_phone":    "+5511777777777",
		"party_size":       8,
	}
	b3, _ := json.Marshal(payload3)
	createRes3, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b3))
	if err != nil {
		t.Fatalf("post multi-table reservation: %v", err)
	}
	if createRes3.StatusCode != http.StatusCreated {
		t.Fatalf("expected created for multi-table reservation, got %d", createRes3.StatusCode)
	}

	// Table 1 should now have 3 reservations
	res5, err := http.Get(srv.URL + "/tables/1/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 1 after multi-table reservation: %v", err)
	}
	if res5.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res5.StatusCode)
	}
	var reservations4 []models.Reservation
	if err := json.NewDecoder(res5.Body).Decode(&reservations4); err != nil {
		t.Fatalf("decode reservations 4: %v", err)
	}
	if len(reservations4) != 3 {
		t.Fatalf("expected 3 reservations for table 1, got %d", len(reservations4))
	}

	// Table 2 should now have 1 reservation
	res6, err := http.Get(srv.URL + "/tables/2/reservations")
	if err != nil {
		t.Fatalf("get reservations for table 2 after multi-table reservation: %v", err)
	}
	if res6.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for table 2 after multi-table reservation, got %d", res6.StatusCode)
	}
	var reservations5 []models.Reservation
	if err := json.NewDecoder(res6.Body).Decode(&reservations5); err != nil {
		t.Fatalf("decode reservations for table 2 after multi-table: %v", err)
	}
	if len(reservations5) != 1 {
		t.Fatalf("expected 1 reservation for table 2, got %d", len(reservations5))
	}
	if reservations5[0].ResponsibleName != "Multi Table" {
		t.Fatalf("expected responsible name 'Multi Table', got '%s'", reservations5[0].ResponsibleName)
	}

	// Test 6: Invalid table ID format
	res7, err := http.Get(srv.URL + "/tables/invalid/reservations")
	if err != nil {
		t.Fatalf("get reservations for invalid table id: %v", err)
	}
	if res7.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid table id, got %d", res7.StatusCode)
	}

	// Test 7: Non-existent table (valid ID but table doesn't exist - should return empty list)
	res8, err := http.Get(srv.URL + "/tables/999/reservations")
	if err != nil {
		t.Fatalf("get reservations for non-existent table: %v", err)
	}
	if res8.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for non-existent table, got %d", res8.StatusCode)
	}
	var reservations6 []models.Reservation
	if err := json.NewDecoder(res8.Body).Decode(&reservations6); err != nil {
		t.Fatalf("decode reservations for non-existent table: %v", err)
	}
	if len(reservations6) != 0 {
		t.Fatalf("expected 0 reservations for non-existent table, got %d", len(reservations6))
	}
}

func TestListReservationsForTableWithMultipleTables(t *testing.T) {
	_, r, _, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Create a reservation for multiple tables at once
	start := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	payload := map[string]interface{}{
		"table_ids":        []uint{1, 2, 3},
		"start_at":         start.Format(time.RFC3339),
		"end_at":           end.Format(time.RFC3339),
		"responsible_name": "Large Party",
		"contact_phone":    "+5511999999999",
		"party_size":       12,
	}
	b, _ := json.Marshal(payload)
	createRes, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post reservation: %v", err)
	}
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected created, got %d", createRes.StatusCode)
	}

	// Verify all three tables show the same reservation
	for i := 1; i <= 3; i++ {
		res, err := http.Get(fmt.Sprintf("%s/tables/%d/reservations", srv.URL, i))
		if err != nil {
			t.Fatalf("get reservations for table %d: %v", i, err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for table %d, got %d", i, res.StatusCode)
		}
		var reservations []models.Reservation
		if err := json.NewDecoder(res.Body).Decode(&reservations); err != nil {
			t.Fatalf("decode reservations for table %d: %v", i, err)
		}
		if len(reservations) != 1 {
			t.Fatalf("expected 1 reservation for table %d, got %d", i, len(reservations))
		}
		if reservations[0].ResponsibleName != "Large Party" {
			t.Fatalf("expected responsible name 'Large Party' for table %d, got '%s'", i, reservations[0].ResponsibleName)
		}
	}
}
