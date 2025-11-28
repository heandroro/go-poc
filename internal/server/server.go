package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/heandroro/go-poc/internal/db"
	"github.com/heandroro/go-poc/internal/handlers"
)

type Server struct {
	router *chi.Mux
	db     *db.DB
}

func NewServer() (*Server, error) {
	database, err := db.New("./orders.db")
	if err != nil {
		return nil, err
	}

	r := chi.NewRouter()
	h := handlers.NewHandler(database)

	// routes
	r.Post("/tables", h.CreateTable)
	r.Get("/tables/{id}", h.GetTable)

	r.Post("/cards", h.CreateCard)
	r.Get("/cards/{id}", h.GetCard)

	r.Post("/cards/{id}/items", h.CreateItem)
	r.Get("/cards/{id}/items", h.ListItems)

	// reservations
	r.Post("/reservations", h.CreateReservation)
	r.Get("/reservations/{id}", h.GetReservation)
	r.Post("/reservations/{id}/checkin", h.CheckInReservation)
	r.Post("/reservations/{id}/checkout", h.CheckOutReservation)
	r.Get("/tables/{id}/reservations", h.ListReservationsForTable)

	return &Server{router: r, db: database}, nil
}

func (s *Server) Run(port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("listening on %s\n", addr)
	return http.ListenAndServe(addr, s.router)
}
