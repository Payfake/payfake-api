package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/payfake/payfake-api/internal/config"
	"github.com/payfake/payfake-api/internal/database"
	"github.com/payfake/payfake-api/internal/router"
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

	result := router.Setup(
		db.DB,
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiryMinutes,
		cfg.JWT.RefreshExpiryDays,
		cfg.App.FrontendURL,
		cfg.App.Env,
	)

	addr := fmt.Sprintf(":%s", cfg.App.Port)

	// Use http.Server directly instead of r.Run() so we can shut down
	// gracefully. r.Run() blocks and never returns, we can't intercept
	// OS signals with it. http.Server.Shutdown() lets in-flight requests
	// complete before the server stops accepting new ones.
	srv := &http.Server{
		Addr:         addr,
		Handler:      result.Engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	/// Start background workers before the server.
	workerCtx, _ := context.WithCancel(context.Background())

	// Webhook retry, re-delivers failed webhooks every 60 seconds.
	result.WebhookSvc.StartRetryWorker(workerCtx)

	// Transaction expiry, marks stale pending transactions as abandoned
	// after 1 hour, matching real Paystack behavior.
	result.TxSvc.StartExpiryWorker(workerCtx)

	// Start the server in a goroutine so the main goroutine can listen
	// for OS signals without being blocked by ListenAndServe.
	go func() {
		log.Printf("[payfake] server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[payfake] server error: %v", err)
		}
	}()

	// Block until we receive SIGINT (Ctrl+C) or SIGTERM (Docker stop, Railway deploy).
	// SIGTERM is what orchestrators send when they want to stop the container —
	// without handling it the container gets force-killed after a timeout
	// and in-flight requests are dropped.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[payfake] shutting down gracefully...")

	// Give in-flight requests 10 seconds to complete before forcing shutdown.
	// This covers MoMo async goroutines and webhook delivery goroutines
	// that may be mid-flight when the signal arrives.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[payfake] forced shutdown: %v", err)
	}

	log.Println("[payfake] server stopped")
}
