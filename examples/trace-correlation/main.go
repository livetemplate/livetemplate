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
	// Note: Trace IDs are logged automatically at the HTTP handler level
	// See the TraceMiddleware wrapper below for automatic correlation

	switch ctx.Action {
	case "increment":
		s.Counter++
		log.Printf("Counter incremented to %d", s.Counter)
	case "decrement":
		s.Counter--
		log.Printf("Counter decremented to %d", s.Counter)
	case "reset":
		oldValue := s.Counter
		s.Counter = 0
		log.Printf("Counter reset from %d to 0", oldValue)
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
	log.Println("LiveTemplate Trace Correlation Example")
	log.Println("======================================")

	// Load configuration
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Setup structured logging with trace ID support
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

	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	logger := observe.NewLogger(level, handler)
	logger.Info("Starting server with trace correlation",
		"log_level", envConfig.LogLevel,
		"dev_mode", envConfig.DevMode,
	)

	// Create initial state
	state := &CounterState{
		Title:       "Trace Correlation Demo",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template
	tmpl := livetemplate.New("counter", envConfig.ToOptions()...)
	liveHandler := tmpl.Handle(state)

	// Setup HTTP routes with trace middleware
	mux := http.NewServeMux()

	// Wrap main handler with trace middleware for automatic trace ID injection
	mux.Handle("/", observe.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create logger with trace ID from context
		traceLogger := observe.LoggerWithTraceID(logger, r.Context())
		traceID := observe.GetTraceID(r.Context())

		traceLogger.Info("Handling request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		// Log that we're about to handle the request
		log.Printf("[trace_id=%s] %s %s from %s",
			traceID, r.Method, r.URL.Path, r.RemoteAddr)

		// Call the actual handler
		liveHandler.ServeHTTP(w, r)

		traceLogger.Info("Request completed",
			"method", r.Method,
			"path", r.URL.Path,
		)
	})))

	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// Health check endpoint with trace support
	mux.Handle("/health", observe.TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceLogger := observe.LoggerWithTraceID(logger, r.Context())
		traceLogger.Debug("Health check",
			"status", "healthy",
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Server starting",
		"port", port,
		"url", "http://localhost:"+port,
	)

	log.Println()
	log.Println("=== Trace Correlation Features ===")
	log.Println("1. Every request gets a unique trace ID")
	log.Println("2. Trace ID appears in all logs for that request")
	log.Println("3. Trace ID is returned in X-Trace-ID response header")
	log.Println("4. Send X-Trace-ID header to preserve trace across services")
	log.Println()
	log.Println("Try these commands:")
	log.Println()
	log.Println("  # Browser: Open http://localhost:" + port)
	log.Println("  # Watch logs and see trace IDs correlate requests")
	log.Println()
	log.Println("  # cURL: Get trace ID in response")
	log.Println("  curl -v http://localhost:" + port + "/health")
	log.Println()
	log.Println("  # cURL: Send custom trace ID")
	log.Println("  curl -H 'X-Trace-ID: my-custom-trace-123' http://localhost:" + port + "/health")
	log.Println()

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
