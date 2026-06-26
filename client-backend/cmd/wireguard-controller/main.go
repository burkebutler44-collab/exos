package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relay/client-backend/internal/config"
	"relay/client-backend/internal/store"
	wgcontroller "relay/client-backend/internal/wireguard"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	gatewayID := env("WG_GATEWAY_ID", "")
	if gatewayID == "" {
		log.Fatal("WG_GATEWAY_ID is required")
	}
	interval := envDuration("WG_RECONCILE_INTERVAL", 15*time.Second)

	repo, err := store.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer repo.Close()

	controller, err := wgcontroller.NewController(repo, gatewayID, interval)
	if err != nil {
		log.Fatalf("create wireguard controller: %v", err)
	}
	defer controller.Close()

	log.Printf("wireguard controller started gateway_id=%s interval=%s", gatewayID, interval)
	if err := controller.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("run wireguard controller: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("invalid duration %s=%q, using %s", key, value, fallback)
		return fallback
	}
	return parsed
}
