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

func TestGetCardPreloadsItems(t *testing.T) {
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	h := NewHandler(d)
	r := chi.NewRouter()
	r.Post("/tables", h.CreateTable)
	r.Post("/cards", h.CreateCard)
	r.Get("/cards/{id}", h.GetCard)
	r.Post("/cards/{id}/items", h.CreateItem)

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
		t.Fatalf("unexpected status creating table: %d", resp.StatusCode)
	}

	data, _ := io.ReadAll(resp.Body)
	var table models.Table
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatalf("invalid table json: %v", err)
	}

	// 2. Create card
	cardPayload := map[string]interface{}{"table_id": table.ID, "token": "card-preload-test"}
	b, _ = json.Marshal(cardPayload)
	resp, err = http.Post(ts.URL+"/cards", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed create card: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status creating card: %d", resp.StatusCode)
	}

	data, _ = io.ReadAll(resp.Body)
	var card models.Card
	if err := json.Unmarshal(data, &card); err != nil {
		t.Fatalf("invalid card json: %v", err)
	}

	idStr := strconv.Itoa(int(card.ID))

	// 3. Add multiple items to the card
	items := []map[string]interface{}{
		{"name": "Pizza", "quantity": 2, "price": 2500},
		{"name": "Burger", "quantity": 1, "price": 1500},
	}

	for _, itemPayload := range items {
		b, _ = json.Marshal(itemPayload)
		resp, err = http.Post(ts.URL+"/cards/"+idStr+"/items", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("failed create item: %v", err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("unexpected status creating item: %d", resp.StatusCode)
		}
	}

	// 4. Get the card and verify items are preloaded
	resp, err = http.Get(ts.URL + "/cards/" + idStr)
	if err != nil {
		t.Fatalf("failed to get card: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status getting card: %d", resp.StatusCode)
	}

	data, _ = io.ReadAll(resp.Body)
	var retrievedCard models.Card
	if err := json.Unmarshal(data, &retrievedCard); err != nil {
		t.Fatalf("invalid card json: %v", err)
	}

	// Verify the card has the expected ID
	if retrievedCard.ID != card.ID {
		t.Fatalf("expected card ID %d, got %d", card.ID, retrievedCard.ID)
	}

	// Verify items are preloaded
	if len(retrievedCard.Items) != 2 {
		t.Fatalf("expected 2 items preloaded, got %d", len(retrievedCard.Items))
	}

	// Verify item details
	itemNames := make(map[string]bool)
	for _, item := range retrievedCard.Items {
		itemNames[item.Name] = true
	}

	if !itemNames["Pizza"] {
		t.Error("expected Pizza item to be preloaded")
	}
	if !itemNames["Burger"] {
		t.Error("expected Burger item to be preloaded")
	}
}

func TestGetCardNotFound(t *testing.T) {
	d, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	h := NewHandler(d)
	r := chi.NewRouter()
	r.Get("/cards/{id}", h.GetCard)

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Try to get a card that doesn't exist
	resp, err := http.Get(ts.URL + "/cards/9999")
	if err != nil {
		t.Fatalf("failed to get card: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}
