package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

func (s *CounterState) Change(ctx *livetemplate.ActionContext) error {
	switch ctx.Action {
	case "increment":
		s.Counter++
	case "decrement":
		s.Counter--
	case "reset":
		s.Counter = 0
	default:
		log.Printf("Unknown action: %s", ctx.Action)
		return nil
	}

	s.LastUpdated = formatTime()
	return nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Graceful Shutdown Example")
	log.Println("=======================================")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create initial state
	state := &CounterState{
		Title:       "Graceful Shutdown Demo",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	tmpl := livetemplate.New("counter", envConfig.ToOptions()...)

	// Get the LiveHandler for shutdown control
	handler := tmpl.Handle(state)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// Create HTTP server with timeouts
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on http://localhost:%s", port)
		log.Println()
		log.Println("Try these actions:")
		log.Println("  1. Open browser to http://localhost:" + port)
		log.Println("  2. Click increment/decrement buttons")
		log.Println("  3. Press Ctrl+C to trigger graceful shutdown")
		log.Println()
		log.Println("Press Ctrl+C to shutdown...")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println()
	log.Println("Shutdown signal received. Starting graceful shutdown...")

	// ============================================================
	// GRACEFUL SHUTDOWN SEQUENCE
	// ============================================================
	// This demonstrates the proper order of operations for
	// zero-downtime deployments:
	//
	// 1. Stop accepting new HTTP connections (http.Server.Shutdown)
	// 2. Close WebSocket connections gracefully (LiveHandler.Shutdown)
	// 3. Wait for in-flight operations to complete (with timeout)
	// ============================================================

	// Use configured shutdown timeout or default to 30 seconds
	shutdownTimeout := envConfig.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Printf("Shutdown timeout: %s", shutdownTimeout)
	log.Println()

	// Step 1: Shutdown HTTP server (stops accepting new connections)
	// This allows Kubernetes/load balancers to stop routing traffic
	log.Println("Step 1: Shutting down HTTP server (stops new connections)...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	} else {
		log.Println("✓ HTTP server shutdown complete")
	}

	// Step 2: Shutdown LiveHandler (closes WebSocket connections gracefully)
	// This sends close frames to all active WebSocket connections
	// and waits for in-flight actions to complete
	log.Println()
	log.Println("Step 2: Shutting down LiveHandler (closes WebSocket connections)...")
	if liveHandler, ok := handler.(interface{ Shutdown(context.Context) error }); ok {
		if err := liveHandler.Shutdown(ctx); err != nil {
			log.Printf("LiveHandler shutdown error: %v", err)
		} else {
			log.Println("✓ LiveHandler shutdown complete")
		}
	}

	log.Println()
	log.Println("========================================")
	log.Println("Graceful shutdown completed successfully")
	log.Println("========================================")
}
