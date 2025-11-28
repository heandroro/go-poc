package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/heandroro/go-poc/internal/db"
	"github.com/heandroro/go-poc/internal/models"
)

func TestFullFlow(t *testing.T) {
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	h := NewHandler(d)
	r := chi.NewRouter()
	r.Post("/tables", h.CreateTable)
	r.Post("/cards", h.CreateCard)
	r.Post("/cards/{id}/items", h.CreateItem)
	r.Get("/cards/{id}/items", h.ListItems)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// 1. Create table
	tablePayload := map[string]interface{}{"name": "Test Table"}
	b, _ := json.Marshal(tablePayload)
	resp, err := http.Post(ts.URL+"/tables", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create table: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	data, _ := io.ReadAll(resp.Body)
	var table models.Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatalf("invalid table json: %v", err)
	}

	// 2. Create card
	cardPayload := map[string]interface{}{"table_id": table.ID, "token": "card-1"}
	b, _ = json.Marshal(cardPayload)
	resp, err = http.Post(ts.URL+"/cards", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create card: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	data, _ = io.ReadAll(resp.Body)
	var card models.Card
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("invalid card json: %v", err)
	}

	idStr := strconv.Itoa(int(card.ID))

	// 3. Add an item
	itemPayload := map[string]interface{}{"name": "Pizza", "quantity": 2, "price": 2500}
	b, _ = json.Marshal(itemPayload)
	resp, err = http.Post(ts.URL+"/cards/"+idStr+"/items", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create item: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	// 4. List items
	resp, err = http.Get(ts.URL + "/cards/" + idStr + "/items")
	if err != nil {
		t.Fatalf("failed list items: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	data, _ = io.ReadAll(resp.Body)
	var items []models.Item
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("invalid items json: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestCreateItemDefaultQuantity(t *testing.T) {
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	h := NewHandler(d)
	r := chi.NewRouter()
	r.Post("/tables", h.CreateTable)
	r.Post("/cards", h.CreateCard)
	r.Post("/cards/{id}/items", h.CreateItem)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Setup: Create table and card
	tablePayload := map[string]interface{}{"name": "Test Table"}
	b, _ := json.Marshal(tablePayload)
	resp, err := http.Post(ts.URL+"/tables", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create table: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	var table models.Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatalf("invalid table json: %v", err)
	}

	cardPayload := map[string]interface{}{"table_id": table.ID, "token": "card-1"}
	b, _ = json.Marshal(cardPayload)
	resp, err = http.Post(ts.URL+"/cards", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create card: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	data, _ = io.ReadAll(resp.Body)
	var card models.Card
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("invalid card json: %v", err)
	}
	idStr := strconv.Itoa(int(card.ID))

	tests := []struct {
		name             string
		quantity         int
		expectedQuantity int
	}{
		{"zero quantity defaults to 1", 0, 1},
		{"negative quantity defaults to 1", -5, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			itemPayload := map[string]interface{}{"name": "Burger", "quantity": tc.quantity, "price": 1500}
			b, _ := json.Marshal(itemPayload)
			resp, err := http.Post(ts.URL+"/cards/"+idStr+"/items", "application/json", bytes.NewReader(b))
			if err != nil {
				t.Fatalf("failed create item: %v", err)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("unexpected status: %d", resp.StatusCode)
			}

			data, _ := io.ReadAll(resp.Body)
			var item models.Item
			if err := json.Unmarshal(data, &item); err != nil {
				t.Fatalf("invalid item json: %v", err)
			}

			if item.Quantity != tc.expectedQuantity {
				t.Errorf("expected quantity %d, got %d", tc.expectedQuantity, item.Quantity)
			}
		})
	}
}
