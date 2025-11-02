package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
	"github.com/livetemplate/livetemplate/internal/observe"
)

// AppState represents the application state
type AppState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

func (s *AppState) Change(ctx *livetemplate.ActionContext) error {
	switch ctx.Action {
	case "increment":
		s.Counter++
	case "decrement":
		s.Counter--
	case "reset":
		s.Counter = 0
	}
	s.LastUpdated = formatTime()
	return nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Production Example")
	log.Println("================================")

	// Load environment-based configuration
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Setup structured logging
	var handler slog.Handler
	var level slog.Level

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

	// Use JSON logging in production
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	logger := observe.NewLogger(level, handler)
	logger.Info("Starting production server",
		"log_level", envConfig.LogLevel,
		"dev_mode", envConfig.DevMode,
		"metrics_enabled", envConfig.MetricsEnabled,
		"shutdown_timeout", envConfig.ShutdownTimeout,
	)

	// Create initial state
	state := &AppState{
		Title:       "Production Demo",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template
	tmpl := livetemplate.New("app", envConfig.ToOptions()...)
	liveHandler := tmpl.Handle(state)

	// Setup HTTP routes with trace middleware
	mux := http.NewServeMux()

	// Main application handler with trace correlation
	mux.Handle("/", observe.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceLogger := observe.LoggerWithTraceID(logger, r.Context())
		traceLogger.Info("Handling request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		liveHandler.ServeHTTP(w, r)

		traceLogger.Info("Request completed",
			"method", r.Method,
			"path", r.URL.Path,
		)
	})))

	// Client library
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// Health check endpoint
	mux.Handle("/health", observe.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceLogger := observe.LoggerWithTraceID(logger, r.Context())
		traceLogger.Debug("Health check",
			"status", "healthy",
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})))

	// Readiness check endpoint (for Kubernetes)
	mux.Handle("/ready", observe.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In a real app, check database connectivity, external services, etc.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})))

	// Metrics endpoint (if enabled)
	if envConfig.MetricsEnabled {
		mux.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// In a real app, expose Prometheus metrics here
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("# Metrics endpoint placeholder\n"))
		}))
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server with production-ready timeouts
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("Server starting",
		"port", port,
		"url", "http://localhost:"+port,
	)

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Run server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	logger.Info("Server started successfully")
	log.Println()
	log.Println("=== Production Features ===")
	log.Println("✓ Environment-based configuration")
	log.Println("✓ Structured JSON logging (production mode)")
	log.Println("✓ Request tracing with trace IDs")
	log.Println("✓ Graceful shutdown")
	log.Println("✓ Health check endpoint: /health")
	log.Println("✓ Readiness check endpoint: /ready")
	log.Println("✓ Metrics endpoint: /metrics")
	log.Println()
	log.Println("Access the application at http://localhost:" + port)
	log.Println()

	// Wait for interrupt signal
	<-quit

	logger.Info("Shutdown signal received, starting graceful shutdown")

	// Graceful shutdown sequence
	shutdownTimeout := envConfig.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("Shutting down HTTP server", "timeout", shutdownTimeout)

	// Step 1: Shutdown HTTP server (stops accepting new connections)
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	} else {
		logger.Info("HTTP server shutdown successfully")
	}

	// Step 2: Shutdown LiveHandler (closes WebSocket connections gracefully)
	if shutdownHandler, ok := liveHandler.(interface{ Shutdown(context.Context) error }); ok {
		logger.Info("Shutting down LiveHandler")
		if err := shutdownHandler.Shutdown(ctx); err != nil {
			logger.Error("LiveHandler shutdown error", "error", err)
		} else {
			logger.Info("LiveHandler shutdown successfully")
		}
	}

	logger.Info("Graceful shutdown completed")
	log.Println("Server stopped")
}
