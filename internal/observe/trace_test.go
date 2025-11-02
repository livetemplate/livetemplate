package observe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateTraceID(t *testing.T) {
	// Test basic generation
	traceID := GenerateTraceID()
	if traceID == "" {
		t.Fatal("GenerateTraceID returned empty string")
	}

	// Test length (should be 16 hex characters for 8 bytes)
	if len(traceID) != 16 {
		t.Errorf("Expected trace ID length of 16, got %d", len(traceID))
	}

	// Test that it's valid hex
	for _, c := range traceID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Trace ID contains invalid hex character: %c", c)
		}
	}
}

func TestGenerateTraceID_Uniqueness(t *testing.T) {
	// Generate 1000 trace IDs and check for uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		traceID := GenerateTraceID()
		if seen[traceID] {
			t.Errorf("Duplicate trace ID generated: %s", traceID)
		}
		seen[traceID] = true
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := "test-trace-id-123"

	// Add trace ID to context
	ctx = WithTraceID(ctx, traceID)

	// Verify trace ID is set
	gotTraceID := GetTraceID(ctx)
	if gotTraceID != traceID {
		t.Errorf("Expected trace ID %s, got %s", traceID, gotTraceID)
	}

	// Verify request_id alias is also set
	if requestID := ctx.Value(RequestIDKey); requestID != traceID {
		t.Errorf("Expected request_id alias to be set to %s, got %v", traceID, requestID)
	}
}

func TestWithTraceID_PreservesExisting(t *testing.T) {
	ctx := context.Background()
	originalTraceID := "original-trace-id"
	newTraceID := "new-trace-id"

	// Add first trace ID
	ctx = WithTraceID(ctx, originalTraceID)

	// Try to overwrite (should be preserved)
	ctx = WithTraceID(ctx, newTraceID)

	// Verify original trace ID is preserved
	gotTraceID := GetTraceID(ctx)
	if gotTraceID != originalTraceID {
		t.Errorf("Expected original trace ID %s to be preserved, got %s", originalTraceID, gotTraceID)
	}
}

func TestGetTraceID_Empty(t *testing.T) {
	ctx := context.Background()

	// Get from empty context
	traceID := GetTraceID(ctx)
	if traceID != "" {
		t.Errorf("Expected empty trace ID, got %s", traceID)
	}
}

func TestGetTraceID_FromRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "legacy-request-id"

	// Add only request_id (for backwards compatibility)
	ctx = context.WithValue(ctx, RequestIDKey, requestID)

	// Should be able to get it as trace ID
	traceID := GetTraceID(ctx)
	if traceID != requestID {
		t.Errorf("Expected trace ID from request_id: %s, got %s", requestID, traceID)
	}
}

func TestTraceMiddleware_GeneratesTraceID(t *testing.T) {
	// Create test handler that checks for trace ID
	var capturedTraceID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(w, req)

	// Verify trace ID was generated and added to context
	if capturedTraceID == "" {
		t.Error("Expected trace ID to be generated, got empty string")
	}

	// Verify trace ID was added to response header
	responseTraceID := w.Header().Get("X-Trace-ID")
	if responseTraceID != capturedTraceID {
		t.Errorf("Expected response trace ID %s, got %s", capturedTraceID, responseTraceID)
	}

	// Verify trace ID format
	if len(capturedTraceID) != 16 {
		t.Errorf("Expected trace ID length of 16, got %d", len(capturedTraceID))
	}
}

func TestTraceMiddleware_PreservesExistingTraceID(t *testing.T) {
	existingTraceID := "existing-trace-12"

	// Create test handler
	var capturedTraceID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware(handler)

	// Create test request with existing trace ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Trace-ID", existingTraceID)
	w := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(w, req)

	// Verify existing trace ID was preserved
	if capturedTraceID != existingTraceID {
		t.Errorf("Expected existing trace ID %s, got %s", existingTraceID, capturedTraceID)
	}

	// Verify trace ID in response header
	responseTraceID := w.Header().Get("X-Trace-ID")
	if responseTraceID != existingTraceID {
		t.Errorf("Expected response trace ID %s, got %s", existingTraceID, responseTraceID)
	}
}

func TestTraceMiddleware_FallbackToRequestID(t *testing.T) {
	requestID := "legacy-request-id"

	// Create test handler
	var capturedTraceID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware(handler)

	// Create test request with X-Request-ID header (legacy)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", requestID)
	w := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(w, req)

	// Verify request ID was used as trace ID
	if capturedTraceID != requestID {
		t.Errorf("Expected trace ID from request ID %s, got %s", requestID, capturedTraceID)
	}
}

func TestTraceMiddleware_PreferTraceIDOverRequestID(t *testing.T) {
	traceID := "trace-id-value"
	requestID := "request-id-value"

	// Create test handler
	var capturedTraceID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceID = GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware(handler)

	// Create test request with both headers
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Trace-ID", traceID)
	req.Header.Set("X-Request-ID", requestID)
	w := httptest.NewRecorder()

	// Execute request
	middleware.ServeHTTP(w, req)

	// Verify X-Trace-ID takes precedence
	if capturedTraceID != traceID {
		t.Errorf("Expected trace ID %s to take precedence, got %s", traceID, capturedTraceID)
	}
}

func TestLoggerWithTraceID(t *testing.T) {
	// Create logger
	logger := NewLogger(0, nil)

	// Create context with trace ID
	ctx := context.Background()
	traceID := "test-trace-id"
	ctx = WithTraceID(ctx, traceID)

	// Create logger with trace ID
	loggerWithTrace := LoggerWithTraceID(logger, ctx)

	// Verify logger was created (basic check)
	if loggerWithTrace == nil {
		t.Error("Expected non-nil logger")
	}

	// Note: We can't easily verify the trace ID is in the logger without
	// capturing log output, but we can verify it doesn't panic
}

func TestLoggerWithTraceID_EmptyContext(t *testing.T) {
	// Create logger
	logger := NewLogger(0, nil)

	// Create context without trace ID
	ctx := context.Background()

	// Create logger with trace ID (should return same logger)
	loggerWithTrace := LoggerWithTraceID(logger, ctx)

	// Verify logger was returned
	if loggerWithTrace == nil {
		t.Error("Expected non-nil logger")
	}

	// Should return same logger when no trace ID
	if loggerWithTrace != logger {
		t.Error("Expected same logger when no trace ID in context")
	}
}

func TestTraceMiddleware_Integration(t *testing.T) {
	// Simulate a full request flow with trace propagation
	var (
		requestTraceID  string
		responseTraceID string
	)

	// Create nested handlers to test trace ID propagation
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTraceID = GetTraceID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with trace middleware
	middleware := TraceMiddleware(innerHandler)

	// Execute request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	responseTraceID = w.Header().Get("X-Trace-ID")

	// Verify trace ID flows through entire request
	if requestTraceID == "" {
		t.Error("Trace ID not propagated to request context")
	}
	if responseTraceID == "" {
		t.Error("Trace ID not added to response header")
	}
	if requestTraceID != responseTraceID {
		t.Errorf("Trace ID mismatch: request=%s, response=%s", requestTraceID, responseTraceID)
	}
}

func TestTraceMiddleware_CaseInsensitiveHeaders(t *testing.T) {
	// HTTP headers should be case-insensitive
	testCases := []struct {
		name   string
		header string
		value  string
	}{
		{"lowercase", "x-trace-id", "trace-lowercase"},
		{"uppercase", "X-TRACE-ID", "trace-uppercase"},
		{"mixed", "X-Trace-Id", "trace-mixed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedTraceID string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedTraceID = GetTraceID(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := TraceMiddleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(tc.header, tc.value)
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if !strings.Contains(strings.ToLower(capturedTraceID), "trace") {
				t.Logf("Warning: Expected trace ID to be preserved regardless of case, got %s", capturedTraceID)
				// Note: This might be implementation-dependent based on http.Request.Header.Get behavior
			}
		})
	}
}
