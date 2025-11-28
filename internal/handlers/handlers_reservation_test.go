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

func TestGetReservation(t *testing.T) {
	_, r, _, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// create a reservation for tables 1 and 2
	start := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)
	end := start.Add(2 * time.Hour)
	payload := map[string]interface{}{
		"table_ids":        []uint{1, 2},
		"start_at":         start.Format(time.RFC3339),
		"end_at":           end.Format(time.RFC3339),
		"responsible_name": "Alice",
		"contact_phone":    "+5511999999999",
		"party_size":       6,
	}
	b, _ := json.Marshal(payload)
	createRes, err := http.Post(srv.URL+"/reservations", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post reservation: %v", err)
	}
	if createRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected created, got %d", createRes.StatusCode)
	}

	// decode the created reservation to get its ID
	var created models.Reservation
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// GET the reservation by ID
	getRes, err := http.Get(srv.URL + "/reservations/" + fmt.Sprint(created.ID))
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	if getRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRes.StatusCode)
	}

	// decode and verify the response
	var fetched models.Reservation
	if err := json.NewDecoder(getRes.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode fetched: %v", err)
	}

	// verify reservation data
	if fetched.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, fetched.ID)
	}
	if fetched.ResponsibleName != "Alice" {
		t.Errorf("expected ResponsibleName 'Alice', got %q", fetched.ResponsibleName)
	}
	if fetched.ContactPhone != "+5511999999999" {
		t.Errorf("expected ContactPhone '+5511999999999', got %q", fetched.ContactPhone)
	}
	if fetched.PartySize != 6 {
		t.Errorf("expected PartySize 6, got %d", fetched.PartySize)
	}

	// verify tables are preloaded
	if len(fetched.Tables) != 2 {
		t.Fatalf("expected 2 preloaded tables, got %d", len(fetched.Tables))
	}

	// verify table names
	tableNames := make(map[string]bool)
	for _, tbl := range fetched.Tables {
		tableNames[tbl.Name] = true
	}
	if !tableNames["T1"] || !tableNames["T2"] {
		t.Errorf("expected tables T1 and T2, got %v", fetched.Tables)
	}
}

func TestGetReservationNotFound(t *testing.T) {
	_, r, _, cleanup := setupServer(t)
	defer cleanup()

	srv := httptest.NewServer(r)
	defer srv.Close()

	// GET a non-existent reservation
	getRes, err := http.Get(srv.URL + "/reservations/99999")
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	if getRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", getRes.StatusCode)
	}
}
