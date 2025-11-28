package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/heandroro/go-poc/internal/db"
	"github.com/heandroro/go-poc/internal/models"
)

type Handler struct {
	db *db.DB
}

func NewHandler(d *db.DB) *Handler { return &Handler{db: d} }

// CreateTable creates a new table
func (h *Handler) CreateTable(w http.ResponseWriter, r *http.Request) {
	var t models.Table
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.db.Create(&t).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&t)
}

func (h *Handler) GetTable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t models.Table
	if err := h.db.First(&t, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(&t)
}

// CreateCard associates a card with a table
func (h *Handler) CreateCard(w http.ResponseWriter, r *http.Request) {
	var c models.Card
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if c.TableID == 0 {
		http.Error(w, "table_id is required", http.StatusBadRequest)
		return
	}

	if err := h.db.Create(&c).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&c)
}

func (h *Handler) GetCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var c models.Card
	if err := h.db.Preload("Items").First(&c, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(&c)
}

// CreateItem adds an item to a card
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	cardIDStr := chi.URLParam(r, "id")
	cardID, err := strconv.ParseUint(cardIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}

	var item models.Item
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item.CardID = uint(cardID)

	if item.Quantity <= 0 {
		item.Quantity = 1
	}

	if item.Status == "" {
		item.Status = "ordered"
	}

	if err := h.db.Create(&item).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&item)
}

func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	cardIDStr := chi.URLParam(r, "id")
	cardID, err := strconv.ParseUint(cardIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}

	var items []models.Item
	if err := h.db.Where("card_id = ?", uint(cardID)).Find(&items).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(&items)
}

// CreateReservation creates a reservation for a table
func (h *Handler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	// Expect payload with table_ids (array) and reservation meta
	var payload struct {
		TableIDs        []uint    `json:"table_ids"`
		StartAt         time.Time `json:"start_at"`
		EndAt           time.Time `json:"end_at"`
		ResponsibleName string    `json:"responsible_name"`
		ContactPhone    string    `json:"contact_phone"`
		PartySize       int       `json:"party_size"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(payload.TableIDs) == 0 {
		http.Error(w, "table_ids is required and must contain at least one id", http.StatusBadRequest)
		return
	}

	// validate times
	if payload.StartAt.IsZero() || payload.EndAt.IsZero() || !payload.EndAt.After(payload.StartAt) {
		http.Error(w, "invalid start_at or end_at", http.StatusBadRequest)
		return
	}

	// ensure all tables exist
	var tables []models.Table
	if err := h.db.Where("id IN ?", payload.TableIDs).Find(&tables).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(tables) != len(payload.TableIDs) {
		http.Error(w, "one or more table_ids not found", http.StatusBadRequest)
		return
	}

	// Do the overlap check, creation and table updates in a transaction to avoid partial state
	tx := h.db.Begin()
	if tx.Error != nil {
		http.Error(w, tx.Error.Error(), http.StatusInternalServerError)
		return
	}

	// check overlapping reservations for any of the tables (inside tx)
	var count int64
	if err := tx.Model(&models.Reservation{}).
		Joins("JOIN reservation_tables rt ON rt.reservation_id = reservations.id").
		Where("rt.table_id IN ? AND reservations.status IN ? AND NOT (reservations.end_at <= ? OR reservations.start_at >= ?)", payload.TableIDs, []string{"confirmed", "started", "pending"}, payload.StartAt, payload.EndAt).
		Count(&count).Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count > 0 {
		tx.Rollback()
		http.Error(w, "overlapping reservation exists for one or more tables", http.StatusConflict)
		return
	}

	// create reservation and associate tables (inside tx)
	res := models.Reservation{
		StartAt:         payload.StartAt,
		EndAt:           payload.EndAt,
		ResponsibleName: payload.ResponsibleName,
		ContactPhone:    payload.ContactPhone,
		PartySize:       payload.PartySize,
		Status:          "confirmed",
		Tables:          tables,
	}

	if err := tx.Create(&res).Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// mark tables reserved
	for _, t := range tables {
		if err := tx.Model(&t).Update("status", "reserved").Error; err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(&res)
}

func (h *Handler) GetReservation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var res models.Reservation
	if err := h.db.Preload("Tables").First(&res, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(&res)
}

// CheckInReservation marks a reservation as started and sets table to in_use
func (h *Handler) CheckInReservation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var res models.Reservation
	if err := h.db.Preload("Tables").First(&res, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// optional: ensure now within reservation window
	now := time.Now()
	if now.Before(res.StartAt.Add(-15 * time.Minute)) {
		http.Error(w, "too early to check-in", http.StatusBadRequest)
		return
	}

	// Run check-in and table updates in a transaction
	tx := h.db.Begin()
	if tx.Error != nil {
		http.Error(w, tx.Error.Error(), http.StatusInternalServerError)
		return
	}

	res.Status = "started"
	if err := tx.Save(&res).Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set all associated tables to in_use
	for _, tbl := range res.Tables {
		if err := tx.Model(&tbl).Update("status", "in_use").Error; err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(&res)
}

// CheckOutReservation marks reservation completed and frees the table
func (h *Handler) CheckOutReservation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var res models.Reservation
	if err := h.db.Preload("Tables").First(&res, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Run checkout and table updates in a transaction
	tx := h.db.Begin()
	if tx.Error != nil {
		http.Error(w, tx.Error.Error(), http.StatusInternalServerError)
		return
	}

	res.Status = "completed"
	if err := tx.Save(&res).Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// free all associated tables
	for _, tbl := range res.Tables {
		if err := tx.Model(&tbl).Update("status", "free").Error; err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(&res)
}

// ListReservationsForTable lists reservations for a specific table
func (h *Handler) ListReservationsForTable(w http.ResponseWriter, r *http.Request) {
	tableIDStr := chi.URLParam(r, "id")
	tableID, err := strconv.ParseUint(tableIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid table id", http.StatusBadRequest)
		return
	}
	var res []models.Reservation
	if err := h.db.Joins("JOIN reservation_tables rt ON rt.reservation_id = reservations.id").
		Where("rt.table_id = ?", uint(tableID)).Find(&res).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(&res)
}
