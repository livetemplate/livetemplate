package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/livefir/livetemplate"
	e2etest "github.com/livefir/livetemplate/cmd/lvt/testing"
	"github.com/livefir/livetemplate/internal/observe"
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
	// ============================================================
	// OBSERVABILITY SETUP - Production-ready logging and metrics
	// ============================================================

	// Setup structured logging (JSON for production, Text for development)
	var handler slog.Handler
	var level slog.Level
	if os.Getenv("ENV") == "production" {
		// JSON format for production (easy to parse by log aggregators)
		level = slog.LevelInfo
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		// Text format for development (human-readable)
		level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := observe.NewLogger(level, handler)
	logger.Info("LiveTemplate Counter Server starting with observability enabled")

	// Setup operational metrics
	metrics := observe.NewMetrics(logger.Logger)

	// Start periodic metrics emission in background (every 30 seconds)
	go metrics.EmitPeriodically(30 * time.Second)

	// ============================================================
	// APPLICATION SETUP - Same as before
	// ============================================================

	// Create initial state
	state := &CounterState{
		Title:       "Live Counter (with Observability)",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template - auto-discovers counter.tmpl
	// Template operations are now automatically logged!
	tmpl := livetemplate.New("counter")

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	// All actions and WebSocket events are now logged and metered!
	http.Handle("/", tmpl.Handle(state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// ============================================================
	// SERVER START
	// ============================================================

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Server starting", "port", port, "url", "http://localhost:"+port)
	logger.Info("Metrics will be emitted every 30 seconds")
	logger.Info("Try these URLs:",
		"counter", "http://localhost:"+port,
		"health", "http://localhost:"+port+"/health",
	)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
