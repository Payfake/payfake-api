package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/payfake/internal/config"
	"github.com/GordenArcher/payfake/internal/database"
	"github.com/GordenArcher/payfake/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[payfake] config error: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[payfake] database error: %v", err)
	}

	// Pass JWT config into the router so it can wire the auth service.
	r := router.Setup(db.DB, cfg.JWT.Secret, cfg.JWT.ExpiryHours, cfg.App.FrontendURL)

	addr := fmt.Sprintf(":%s", cfg.App.Port)
	log.Printf("[payfake] server starting on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("[payfake] server error: %v", err)
	}
}
