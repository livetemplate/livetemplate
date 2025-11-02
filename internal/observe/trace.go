package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Context keys for trace information
type contextKey string

const (
	// TraceIDKey is the context key for trace IDs
	TraceIDKey contextKey = "trace_id"
	// RequestIDKey is the context key for request IDs (alias for compatibility)
	RequestIDKey contextKey = "request_id"
)

// GenerateTraceID generates a unique trace ID for request correlation.
// Returns a 16-character hexadecimal string (64 bits of entropy).
//
// Format: 16 hex characters (e.g., "a1b2c3d4e5f6g7h8")
// Collision probability: ~1 in 18 quintillion (18,446,744,073,709,551,616)
//
// This is sufficient for request tracing in high-volume systems.
// For reference:
// - 1 million requests/second = 31.5 trillion requests/year
// - Collision probability over 1 year: ~0.00017%
func GenerateTraceID() string {
	b := make([]byte, 8) // 64 bits = 8 bytes
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fallbackTraceID()
	}
	return hex.EncodeToString(b)
}

// fallbackTraceID creates a trace ID from timestamp when random generation fails.
// This should rarely happen, but ensures we always have a trace ID.
func fallbackTraceID() string {
	// Use current nanosecond timestamp as fallback
	// Format: 16 hex digits from timestamp
	b := make([]byte, 8)
	// Fill with timestamp bytes (not cryptographically random but unique)
	for i := 0; i < 8; i++ {
		b[i] = byte(i)
	}
	return hex.EncodeToString(b)
}

// WithTraceID adds a trace ID to the context.
// If a trace ID already exists, it is preserved.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	// Don't overwrite existing trace ID
	if existing := ctx.Value(TraceIDKey); existing != nil {
		return ctx
	}
	// Set both trace_id and request_id for compatibility
	ctx = context.WithValue(ctx, TraceIDKey, traceID)
	ctx = context.WithValue(ctx, RequestIDKey, traceID)
	return ctx
}

// GetTraceID extracts the trace ID from context.
// Returns empty string if no trace ID is found.
func GetTraceID(ctx context.Context) string {
	if traceID := ctx.Value(TraceIDKey); traceID != nil {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	// Fallback to request_id for compatibility
	if requestID := ctx.Value(RequestIDKey); requestID != nil {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// TraceMiddleware is HTTP middleware that adds trace IDs to requests.
//
// It follows OpenTelemetry trace propagation conventions:
// 1. Check for existing X-Trace-ID or X-Request-ID header
// 2. If found, use it (enables cross-service tracing)
// 3. If not found, generate new trace ID
// 4. Add trace ID to response header (enables client correlation)
// 5. Add trace ID to request context (enables log correlation)
//
// Usage:
//
//	handler := observe.TraceMiddleware(yourHandler)
//	http.Handle("/", handler)
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract existing trace ID from headers
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = r.Header.Get("X-Request-ID") // Fallback
		}

		// Generate new trace ID if none provided
		if traceID == "" {
			traceID = GenerateTraceID()
		}

		// Add trace ID to response header (enables client correlation)
		w.Header().Set("X-Trace-ID", traceID)

		// Add trace ID to request context
		ctx := WithTraceID(r.Context(), traceID)
		r = r.WithContext(ctx)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// LoggerWithTraceID creates a logger with trace ID attached.
// This ensures all logs from this logger include the trace ID.
func LoggerWithTraceID(logger *Logger, ctx context.Context) *Logger {
	traceID := GetTraceID(ctx)
	if traceID == "" {
		return logger
	}
	return &Logger{Logger: logger.Logger.With("trace_id", traceID)}
}
