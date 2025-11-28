package models

import "time"

// Table represents a table in the restaurant
type Table struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // free, reserved, in_use
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Card is associated with a table and holds orders
type Card struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TableID   uint      `json:"table_id"`
	Table     Table     `gorm:"foreignKey:TableID" json:"table,omitempty"`
	Token     string    `json:"token"` // some identifier
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Items     []Item    `gorm:"foreignKey:CardID" json:"items,omitempty"`
}

// Item represents an ordered item for a card
type Item struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CardID    uint      `json:"card_id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Price     int       `json:"price"`  // cents
	Status    string    `json:"status"` // e.g., "ordered", "preparing", "served"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Reservation represents a table reservation with start/end times
type Reservation struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// A reservation can be associated with multiple tables (e.g., large party)
	Tables          []Table   `gorm:"many2many:reservation_tables;" json:"tables,omitempty"`
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	ResponsibleName string    `json:"responsible_name"`
	ContactPhone    string    `json:"contact_phone"`
	PartySize       int       `json:"party_size"`
	Status          string    `json:"status"` // pending, confirmed, started, completed, cancelled
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
