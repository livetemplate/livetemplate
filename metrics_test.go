package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestState is a simple test state
type MetricsTestState struct {
	Value int
}

func (s *MetricsTestState) Change(ctx *ActionContext) error {
	return nil
}

func TestLiveHandler_MetricsHandler(t *testing.T) {
	tmpl := Must(New("metrics-handler-test"))
	handler := tmpl.Handle(&MetricsTestState{})

	metricsHandler := handler.MetricsHandler()

	// Test GET request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	metricsHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected content-type text/plain, got %s", contentType)
	}

	body := rec.Body.String()

	// Verify key metrics are present
	requiredMetrics := []string{
		"livetemplate_connections_active",
		"livetemplate_actions_processed_total",
		"livetemplate_templates_executed_total",
	}

	for _, metric := range requiredMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Metrics output missing: %s", metric)
		}
	}

	// Verify Prometheus format
	if !strings.Contains(body, "# HELP") {
		t.Error("Metrics should include HELP directives")
	}

	if !strings.Contains(body, "# TYPE") {
		t.Error("Metrics should include TYPE directives")
	}
}

func TestLiveHandler_MetricsHandler_MethodNotAllowed(t *testing.T) {
	tmpl := Must(New("metrics-method-test"))
	handler := tmpl.Handle(&MetricsTestState{})

	metricsHandler := handler.MetricsHandler()

	// Test POST request (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	rec := httptest.NewRecorder()

	metricsHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

func TestLiveHandler_MetricsHandler_Format(t *testing.T) {
	tmpl := Must(New("metrics-format-test"))
	handler := tmpl.Handle(&MetricsTestState{})

	metricsHandler := handler.MetricsHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	metricsHandler.ServeHTTP(rec, req)

	body := rec.Body.String()
	lines := strings.Split(body, "\n")

	// Check format of each line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Should be either a comment or a metric line
		if strings.HasPrefix(line, "#") {
			// Comment line - should be HELP or TYPE
			if !strings.HasPrefix(line, "# HELP") && !strings.HasPrefix(line, "# TYPE") {
				t.Errorf("Invalid comment line: %s", line)
			}
		} else {
			// Metric line - should have format: metric_name{labels} value
			// or metric_name value
			parts := strings.Fields(line)
			if len(parts) < 2 {
				t.Errorf("Invalid metric line (missing value): %s", line)
			}

			// Metric name should start with livetemplate_
			metricName := strings.Split(parts[0], "{")[0]
			if !strings.HasPrefix(metricName, "livetemplate_") {
				t.Errorf("Metric name should have livetemplate_ prefix: %s", metricName)
			}
		}
	}
}
