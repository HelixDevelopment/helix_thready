// Command thready-server runs the Helix Thready REST /v1 gateway wired over the
// real sibling domain modules (user_service, semantic_search, skill_dispatch,
// event_bus_service). It listens on $PORT (default 8080) with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"thready.server"
)

func main() {
	handler, err := server.NewServer()
	if err != nil {
		log.Fatalf("thready-server: failed to build server: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// signal.NotifyContext cancels ctx on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("thready-server: listening on :%s (real-wired /v1 surface)", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("thready-server: listen error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("thready-server: shutdown signal received, draining...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("thready-server: graceful shutdown failed: %v", err)
	}
	log.Printf("thready-server: stopped cleanly")
}
