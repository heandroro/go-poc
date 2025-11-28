package main

import (
	"log"

	"github.com/heandroro/go-poc/internal/server"
)

func main() {
	s, err := server.NewServer()
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	if err := s.Run(8080); err != nil {
		log.Fatalf("server run failed: %v", err)
	}
}
