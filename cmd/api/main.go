package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/payfake/internal/config"
	"github.com/GordenArcher/payfake/internal/database"
	"github.com/GordenArcher/payfake/internal/router"
)

func main() {
	// Load and validate config first, if any required env vars are
	// missing we fail immediately with a clear error message rather
	// than panicking somewhere deep in the application later.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[payfake] config error: %v", err)
	}

	// Connect to the database and run AutoMigrate.
	// If the DB is unreachable at startup we fail fast, there's no
	// point starting the HTTP server if we have no database.
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[payfake] database error: %v", err)
	}

	// Wire up all routes with their middleware and handlers.
	r := router.Setup(db.DB)

	addr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("[payfake] server starting on %s", addr)

	// r.Run blocks until the server is stopped.
	// In production you'd use a graceful shutdown pattern here —
	// listen for OS signals (SIGINT, SIGTERM), stop accepting new
	// requests, wait for in-flight requests to complete, then exit.
	// We'll add graceful shutdown after the core features are built.
	if err := r.Run(addr); err != nil {
		log.Fatalf("[payfake] server error: %v", err)
	}
}
