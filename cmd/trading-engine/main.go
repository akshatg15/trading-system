package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-system/internal/config"
	"trading-system/internal/database"
	"trading-system/internal/server"
	"trading-system/internal/signals"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database connection
	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize signal processor
	signalProcessor := signals.New(db, cfg)

	// Initialize HTTP server
	httpServer := server.New(cfg, db, signalProcessor)

	// Start signal processing in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go signalProcessor.Start(ctx)

	// Start HTTP server
	serverAddr := ":" + cfg.Server.Port
	log.Printf("Starting trading engine server on %s", serverAddr)
	log.Printf("Environment: %s", cfg.Environment)
	log.Printf("MT5 Endpoint: %s", cfg.MT5.Endpoint)

	srv := &http.Server{
		Addr:    serverAddr,
		Handler: httpServer.Router(),
	}

	// Start server in background
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down trading engine...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Trading engine stopped")
} 