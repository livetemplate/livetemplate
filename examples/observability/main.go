package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
	"github.com/livetemplate/livetemplate/internal/observe"
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
	// CONFIGURATION - Load from environment variables
	// ============================================================

	// Load configuration from environment variables (LVT_* prefix)
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// ============================================================
	// OBSERVABILITY SETUP - Production-ready logging and metrics
	// ============================================================

	// Setup structured logging (JSON for production, Text for development)
	// Log level is controlled by LVT_LOG_LEVEL environment variable
	var handler slog.Handler
	var level slog.Level

	// Parse log level from config
	switch envConfig.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	if os.Getenv("ENV") == "production" {
		// JSON format for production (easy to parse by log aggregators)
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		// Text format for development (human-readable)
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := observe.NewLogger(level, handler)
	logger.Info("LiveTemplate Counter Server starting with observability enabled",
		"log_level", envConfig.LogLevel,
		"metrics_enabled", envConfig.MetricsEnabled,
		"dev_mode", envConfig.DevMode)

	// Setup operational metrics
	metrics := observe.NewMetrics(logger.Logger)

	// Start periodic metrics emission in background (every 30 seconds)
	// Metrics can be disabled with LVT_METRICS_ENABLED=false
	if envConfig.MetricsEnabled {
		go metrics.EmitPeriodically(30 * time.Second)
	}

	// ============================================================
	// APPLICATION SETUP - With environment-based configuration
	// ============================================================

	// Create initial state
	state := &CounterState{
		Title:       "Live Counter (with Observability)",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	// Template operations are now automatically logged!
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.New("counter", envConfig.ToOptions()...)

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

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
