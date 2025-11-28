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

func TestCheckInTooEarly(t *testing.T) {
	_, r, _, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Create a reservation with start_at 1 hour in the future (well beyond the 15-minute grace period)
	start := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	payload := map[string]interface{}{
		"table_ids":        []uint{1},
		"start_at":         start.Format(time.RFC3339),
		"end_at":           end.Format(time.RFC3339),
		"responsible_name": "Charlie",
		"contact_phone":    "+5511777777777",
		"party_size":       3,
	}
	b, _ := json.Marshal(payload)
	res, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post reservation: %v", err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected created, got %d", res.StatusCode)
	}

	var created models.Reservation
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// Attempt to check-in NOW, which is about 1 hour before start_at (well beyond 15-minute grace period)
	// This should fail with 400 Bad Request ("too early to check-in")
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/reservations/"+fmt.Sprint(created.ID)+"/checkin", nil)
	checkRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("checkin request failed: %v", err)
	}
	if checkRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for too-early check-in, got %d", checkRes.StatusCode)
	}
}
