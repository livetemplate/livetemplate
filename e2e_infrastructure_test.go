package livetemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestE2EInfrastructureFoundation validates the core e2e testing infrastructure
// This test validates task-028 acceptance criteria without requiring browser automation
func TestE2EInfrastructureFoundation(t *testing.T) {
	t.Run("HTTP_Server_Setup", func(t *testing.T) {
		testHTTPServerSetup(t)
	})

	t.Run("LiveTemplate_Endpoints", func(t *testing.T) {
		testLiveTemplateEndpoints(t)
	})

	t.Run("Template_Rendering_Infrastructure", func(t *testing.T) {
		testTemplateRenderingInfrastructure(t)
	})

	t.Run("Fragment_Generation_Infrastructure", func(t *testing.T) {
		testFragmentGenerationInfrastructure(t)
	})
}

func testHTTPServerSetup(t *testing.T) {
	// Test data for HTTP server
	testData := &TestData{
		Title:   "Infrastructure Test",
		Count:   42,
		Items:   []string{"Test Item 1", "Test Item 2"},
		Visible: true,
		Status:  "testing",
		Attrs:   map[string]string{"data-test": "infrastructure", "class": "test-container"},
	}

	// Create application and page
	app, err := NewApplication(
		WithMaxMemoryMB(50),
		WithApplicationMetricsEnabled(true),
	)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	// Simple template for testing
	tmplStr := `
<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>
	<h1 id="title">{{.Title}}</h1>
	<div id="counter">Count: {{.Count}}</div>
	<div id="status" class="{{.Status}}">Status: {{.Status}}</div>
</body>
</html>`

	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	page, err := app.NewApplicationPage(tmpl, testData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() {
		if err := page.Close(); err != nil {
			t.Logf("Warning: Failed to close page: %v", err)
		}
	}()

	// Create HTTP server
	mux := http.NewServeMux()

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := page.Render()
		if err != nil {
			http.Error(w, "Render failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			fmt.Printf("Warning: Failed to write HTML response: %v\n", err)
		}
	})

	// Update endpoint
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var newData TestData
		if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		fragments, err := page.RenderFragments(r.Context(), &newData)
		if err != nil {
			http.Error(w, "Fragment generation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(fragments); err != nil {
			fmt.Printf("Warning: Failed to encode fragments response: %v\n", err)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test root endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to GET root endpoint: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", resp.Header.Get("Content-Type"))
	}

	t.Log("✓ HTTP server setup validated")
}

func testLiveTemplateEndpoints(t *testing.T) {
	// Create test application and page
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	tmplStr := `<div id="test">{{.Title}}</div>`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	initialData := &TestData{Title: "Initial", Count: 0}
	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() {
		if err := page.Close(); err != nil {
			t.Logf("Warning: Failed to close page: %v", err)
		}
	}()

	// Create server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := page.Render()
		if err != nil {
			http.Error(w, fmt.Sprintf("Render failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			// Log error but can't do much else since response is being written
			fmt.Printf("Warning: Failed to write HTML response: %v\n", err)
		}
	})
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		var newData TestData
		if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		fragments, err := page.RenderFragments(r.Context(), &newData)
		if err != nil {
			http.Error(w, fmt.Sprintf("Fragment generation failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(fragments); err != nil {
			// Log error but can't do much else since headers are already sent
			fmt.Printf("Warning: Failed to encode fragments response: %v\n", err)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test initial render endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to test render endpoint: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: Failed to close response body: %v", err)
		}
	}()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, "Initial") {
		t.Errorf("Expected HTML to contain 'Initial', got: %s", html)
	}

	// Test fragment update endpoint
	updateData := &TestData{Title: "Updated", Count: 5}
	updateJSON, _ := json.Marshal(updateData)

	resp, err = http.Post(server.URL+"/update", "application/json", bytes.NewBuffer(updateJSON))
	if err != nil {
		t.Fatalf("Failed to test update endpoint: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for update endpoint, got %d", resp.StatusCode)
	}

	var fragments []*Fragment
	if err := json.NewDecoder(resp.Body).Decode(&fragments); err != nil {
		t.Fatalf("Failed to decode fragments response: %v", err)
	}

	if len(fragments) == 0 {
		t.Error("Expected at least one fragment, got none")
	}

	t.Log("✓ LiveTemplate endpoints validated")
}

func testTemplateRenderingInfrastructure(t *testing.T) {
	// Test template parsing and rendering
	tmplStr := `
<html>
<head><title>{{.Title}}</title></head>
<body>
	<h1 id="title">{{.Title}}</h1>
	<div id="count">{{.Count}}</div>
	{{if .Visible}}
	<div id="content">Visible Content</div>
	{{end}}
	<ul>
	{{range .Items}}
	<li>{{.}}</li>
	{{end}}
	</ul>
</body>
</html>`

	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse complex template: %v", err)
	}

	testData := &TestData{
		Title:   "Template Test",
		Count:   100,
		Items:   []string{"Item A", "Item B", "Item C"},
		Visible: true,
		Status:  "active",
		Attrs:   map[string]string{"class": "test"},
	}

	// Test rendering
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	page, err := app.NewApplicationPage(tmpl, testData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() {
		if err := page.Close(); err != nil {
			t.Logf("Warning: Failed to close page: %v", err)
		}
	}()

	html, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate rendered content
	expectedContent := []string{
		"Template Test",
		"100",
		"Visible Content",
		"Item A",
		"Item B",
		"Item C",
		`id="title"`,
		`id="count"`,
		`id="content"`,
	}

	for _, expected := range expectedContent {
		if !strings.Contains(html, expected) {
			t.Errorf("Expected rendered HTML to contain '%s', but it was missing", expected)
		}
	}

	t.Log("✓ Template rendering infrastructure validated")
}

func testFragmentGenerationInfrastructure(t *testing.T) {
	// Test fragment generation pipeline
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	tmplStr := `<div id="dynamic">{{.Title}}</div><div id="counter">{{.Count}}</div>`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	initialData := &TestData{Title: "Initial", Count: 0}
	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() {
		if err := page.Close(); err != nil {
			t.Logf("Warning: Failed to close page: %v", err)
		}
	}()

	// Test different types of updates to trigger different strategies
	testCases := []struct {
		name             string
		updateData       *TestData
		expectStrategies []string
	}{
		{
			name:             "TextOnlyUpdate",
			updateData:       &TestData{Title: "Updated Text", Count: 42},
			expectStrategies: []string{"static_dynamic"},
		},
		{
			name:             "StructuralUpdate",
			updateData:       &TestData{Title: "Structural", Count: 100, Items: []string{"New Item"}},
			expectStrategies: []string{"granular", "static_dynamic", "replacement"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fragments, err := page.RenderFragments(context.TODO(), tc.updateData)
			if err != nil {
				t.Fatalf("Failed to generate fragments for %s: %v", tc.name, err)
			}

			if len(fragments) == 0 {
				t.Errorf("Expected fragments for %s, got none", tc.name)
				return
			}

			// Validate fragment structure
			for _, fragment := range fragments {
				if fragment.ID == "" {
					t.Errorf("Fragment ID should not be empty")
				}
				if fragment.Strategy == "" {
					t.Errorf("Fragment strategy should not be empty")
				}
				if fragment.Action == "" {
					t.Errorf("Fragment action should not be empty")
				}
				if fragment.Data == nil {
					t.Errorf("Fragment data should not be nil")
				}

				// Validate metadata
				if fragment.Metadata != nil {
					if fragment.Metadata.GenerationTime == 0 {
						t.Errorf("Generation time should be recorded")
					}
					if fragment.Metadata.Confidence < 0 || fragment.Metadata.Confidence > 1 {
						t.Errorf("Confidence should be between 0 and 1, got %f", fragment.Metadata.Confidence)
					}
				}

				t.Logf("Fragment: ID=%s, Strategy=%s, Action=%s, Confidence=%.2f",
					fragment.ID, fragment.Strategy, fragment.Action, fragment.Metadata.Confidence)
			}
		})
	}

	t.Log("✓ Fragment generation infrastructure validated")
}

// TestE2ETestModes validates that tests can run in both normal and short modes
func TestE2ETestModes(t *testing.T) {
	// This test validates that the testing infrastructure supports both modes

	t.Run("ShortMode", func(t *testing.T) {
		// In short mode, browser-dependent tests should be skipped
		if testing.Short() {
			t.Log("✓ Running in short mode - browser tests will be skipped")
		} else {
			t.Log("✓ Running in normal mode - all tests will execute")
		}
	})

	t.Run("NormalMode", func(t *testing.T) {
		// Basic infrastructure should work in both modes
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application in normal mode: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		// Verify core functionality works
		if app.GetPageCount() < 0 {
			t.Error("Page count should be non-negative")
		}

		metrics := app.GetApplicationMetrics()
		if metrics.ApplicationID == "" {
			t.Error("Application ID should not be empty")
		}

		t.Log("✓ Core infrastructure works in normal mode")
	})
}
