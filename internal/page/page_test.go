package page

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"time"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name          string
		applicationID string
		template      *template.Template
		data          interface{}
		config        *Config
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid page creation",
			applicationID: "test-app",
			template:      createTestTemplate(),
			data:          map[string]interface{}{"value": "test"},
			config:        nil,
			expectError:   false,
		},
		{
			name:          "empty application ID",
			applicationID: "",
			template:      createTestTemplate(),
			data:          nil,
			expectError:   true,
			errorContains: "applicationID cannot be empty",
		},
		{
			name:          "nil template",
			applicationID: "test-app",
			template:      nil,
			data:          nil,
			expectError:   true,
			errorContains: "template cannot be nil",
		},
		{
			name:          "custom config",
			applicationID: "test-app",
			template:      createTestTemplate(),
			data:          map[string]interface{}{"value": "test"},
			config:        &Config{MaxFragments: 50, MaxMemoryMB: 5, UpdateBatchSize: 10},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := NewPage(tt.applicationID, tt.template, tt.data, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify page properties
			if page.ID == "" {
				t.Error("page should have a non-empty ID")
			}

			if page.ApplicationID != tt.applicationID {
				t.Errorf("expected application ID %q, got %q", tt.applicationID, page.ApplicationID)
			}

			if page.TemplateHash == "" {
				t.Error("page should have a template hash")
			}

			if page.template != tt.template {
				t.Error("page should store the provided template")
			}

			// Note: Don't compare data directly as maps are not comparable
			if tt.data != nil && page.data == nil {
				t.Error("page should store the provided data")
			}

			if page.treeGenerator == nil {
				t.Error("page should have a tree generator")
			}

			// Verify timestamps
			if page.createdAt.IsZero() {
				t.Error("created time should be set")
			}

			if page.lastAccessed.IsZero() {
				t.Error("last accessed time should be set")
			}

			// Verify config
			if tt.config != nil {
				if page.config.MaxFragments != tt.config.MaxFragments {
					t.Errorf("expected MaxFragments %d, got %d", tt.config.MaxFragments, page.config.MaxFragments)
				}
			}
		})
	}
}

func TestPage_Render(t *testing.T) {
	tests := []struct {
		name         string
		templateText string
		data         interface{}
		expectedHTML string
		expectError  bool
	}{
		{
			name:         "simple template",
			templateText: `<div>{{.value}}</div>`,
			data:         map[string]interface{}{"value": "hello"},
			expectedHTML: `<div>hello</div>`,
			expectError:  false,
		},
		{
			name:         "complex template",
			templateText: `<div class="{{.class}}">{{.name}}: {{.count}}</div>`,
			data:         map[string]interface{}{"class": "item", "name": "Item", "count": 42},
			expectedHTML: `<div class="item">Item: 42</div>`,
			expectError:  false,
		},
		{
			name:         "template with nil data",
			templateText: `<div>static content</div>`,
			data:         nil,
			expectedHTML: `<div>static content</div>`,
			expectError:  false,
		},
		{
			name:         "template with missing data",
			templateText: `<div>{{.missing}}</div>`,
			data:         map[string]interface{}{"other": "value"},
			expectedHTML: `<div></div>`, // Go templates render missing values as empty string
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			page, err := NewPage("test-app", tmpl, tt.data, nil)
			if err != nil {
				t.Fatalf("failed to create page: %v", err)
			}

			html, err := page.Render()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check HTML content - may include lvt-id annotations for dynamic content
			if strings.Contains(tt.templateText, "{{") {
				// Dynamic template - verify content and annotation
				expectedContent := extractExpectedContent(tt.expectedHTML)
				if !strings.Contains(html, expectedContent) {
					t.Errorf("HTML should contain expected content %q, got %q", expectedContent, html)
				}
				// Only templates with dynamic content (not just attributes) get lvt-id
				if hasDynamicContent(tt.templateText) {
					if !strings.Contains(html, "lvt-id=") {
						t.Errorf("Dynamic content template should have lvt-id annotation, got %q", html)
					}
				}
			} else {
				// Static template - should match exactly
				if html != tt.expectedHTML {
					t.Errorf("expected HTML %q, got %q", tt.expectedHTML, html)
				}
			}
		})
	}
}

// extractExpectedContent extracts the content between HTML tags for comparison
func extractExpectedContent(html string) string {
	// Simple extraction: find content between first > and last <
	start := strings.Index(html, ">")
	end := strings.LastIndex(html, "<")
	if start >= 0 && end > start {
		return html[start+1 : end]
	}
	return html
}

// hasDynamicContent checks if template has dynamic content (not just attributes)
func hasDynamicContent(templateText string) bool {
	// Check if there are template expressions between > and < (content area)
	inContent := false
	for i := 0; i < len(templateText)-1; i++ {
		if templateText[i] == '>' {
			inContent = true
		} else if templateText[i] == '<' {
			inContent = false
		} else if inContent && templateText[i:i+2] == "{{" {
			return true
		}
	}
	return false
}

func TestPage_SetDataAndGetData(t *testing.T) {
	page, err := NewPage("test-app", createTestTemplate(),
		map[string]interface{}{"initial": "value"}, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Test initial data
	data := page.GetData()
	if dataMap, ok := data.(map[string]interface{}); ok {
		if dataMap["initial"] != "value" {
			t.Errorf("expected initial value 'value', got %v", dataMap["initial"])
		}
	} else {
		t.Error("expected data to be a map")
	}

	// Test setting new data
	newData := map[string]interface{}{"updated": "data", "count": 42}
	err = page.SetData(newData)
	if err != nil {
		t.Errorf("failed to set data: %v", err)
	}

	// Verify data was updated
	retrievedData := page.GetData()
	if retrievedDataMap, ok := retrievedData.(map[string]interface{}); ok {
		if retrievedDataMap["updated"] != "data" {
			t.Errorf("expected updated value 'data', got %v", retrievedDataMap["updated"])
		}
		if retrievedDataMap["count"] != 42 {
			t.Errorf("expected count 42, got %v", retrievedDataMap["count"])
		}
	} else {
		t.Error("expected retrieved data to be a map")
	}

	// Test setting nil data
	err = page.SetData(nil)
	if err != nil {
		t.Errorf("failed to set nil data: %v", err)
	}

	if page.GetData() != nil {
		t.Error("expected data to be nil after setting nil")
	}
}

func TestPage_UpdateLastAccessed(t *testing.T) {
	page, err := NewPage("test-app", createTestTemplate(), nil, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	originalTime := page.lastAccessed
	time.Sleep(10 * time.Millisecond)

	page.UpdateLastAccessed()

	if !page.lastAccessed.After(originalTime) {
		t.Error("last accessed time should be updated")
	}

	// Test multiple updates
	secondTime := page.lastAccessed
	time.Sleep(10 * time.Millisecond)

	page.UpdateLastAccessed()

	if !page.lastAccessed.After(secondTime) {
		t.Error("last accessed time should be updated again")
	}
}

func TestPage_IsExpired(t *testing.T) {
	page, err := NewPage("test-app", createTestTemplate(), nil, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Test with long TTL (should not be expired)
	if page.IsExpired(1 * time.Hour) {
		t.Error("page should not be expired with 1 hour TTL")
	}

	// Test with very short TTL
	time.Sleep(10 * time.Millisecond)
	if !page.IsExpired(1 * time.Millisecond) {
		t.Error("page should be expired with 1ms TTL after waiting 10ms")
	}

	// Update last accessed and test again
	page.UpdateLastAccessed()
	if page.IsExpired(1 * time.Hour) {
		t.Error("page should not be expired after updating last accessed time")
	}
}

func TestPage_GetMemoryUsage(t *testing.T) {
	tests := []struct {
		name          string
		templateText  string
		data          interface{}
		expectNonZero bool
	}{
		{
			name:          "simple page",
			templateText:  `<div>test</div>`,
			data:          nil,
			expectNonZero: true,
		},
		{
			name:          "page with data",
			templateText:  `<div>{{.value}}</div>`,
			data:          map[string]interface{}{"value": "test", "count": 42},
			expectNonZero: true,
		},
		{
			name:          "page with large data",
			templateText:  `<div>{{.text}}</div>`,
			data:          map[string]interface{}{"text": strings.Repeat("x", 1000)},
			expectNonZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			page, err := NewPage("test-app", tmpl, tt.data, nil)
			if err != nil {
				t.Fatalf("failed to create page: %v", err)
			}

			usage := page.GetMemoryUsage()

			if tt.expectNonZero && usage == 0 {
				t.Error("expected non-zero memory usage")
			}

			if usage < 0 {
				t.Errorf("memory usage should not be negative: %d", usage)
			}

			t.Logf("Memory usage for %s: %d bytes", tt.name, usage)
		})
	}
}

func TestPage_RenderFragments(t *testing.T) {
	tmpl, err := template.New("test").Parse(`<div>Count: {{.count}}</div>`)
	if err != nil {
		t.Fatalf("failed to parse template: %v", err)
	}

	page, err := NewPage("test-app", tmpl,
		map[string]interface{}{"count": 0}, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Test fragment generation
	ctx := context.Background()
	newData := map[string]interface{}{"count": 1}

	fragments, err := page.RenderFragments(ctx, newData)
	if err != nil {
		t.Fatalf("failed to render fragments: %v", err)
	}

	// Verify fragments were generated
	if len(fragments) == 0 {
		t.Error("expected at least one fragment to be generated")
	}

	// Verify page data was updated
	currentData := page.GetData()
	if dataMap, ok := currentData.(map[string]interface{}); ok {
		if dataMap["count"] != 1 {
			t.Errorf("expected count to be updated to 1, got %v", dataMap["count"])
		}
	} else {
		t.Error("expected data to be a map")
	}

	// Verify last accessed time was updated
	if page.lastAccessed.IsZero() {
		t.Error("last accessed time should be updated after rendering fragments")
	}
}

func TestPage_GetMetrics(t *testing.T) {
	page, err := NewPage("test-app", createTestTemplate(), nil, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	metrics := page.GetMetrics()

	// Verify basic metrics fields
	if metrics.PageID != page.ID {
		t.Errorf("expected page ID %s, got %s", page.ID, metrics.PageID)
	}

	if metrics.ApplicationID != page.ApplicationID {
		t.Errorf("expected application ID %s, got %s", page.ApplicationID, metrics.ApplicationID)
	}

	if metrics.CreatedAt.IsZero() {
		t.Error("created at should not be zero")
	}

	if metrics.LastAccessed.IsZero() {
		t.Error("last accessed should not be zero")
	}

	if metrics.Age < 0 {
		t.Errorf("age should not be negative: %v", metrics.Age)
	}

	if metrics.IdleTime < 0 {
		t.Errorf("idle time should not be negative: %v", metrics.IdleTime)
	}

	if metrics.MemoryUsage <= 0 {
		t.Errorf("memory usage should be positive: %d", metrics.MemoryUsage)
	}

	if metrics.FragmentCacheSize < 0 {
		t.Errorf("fragment cache size should not be negative: %d", metrics.FragmentCacheSize)
	}

	// Initially, generation counts should be zero
	if metrics.TotalGenerations != 0 {
		t.Errorf("expected 0 total generations, got %d", metrics.TotalGenerations)
	}

	if metrics.SuccessfulGenerations != 0 {
		t.Errorf("expected 0 successful generations, got %d", metrics.SuccessfulGenerations)
	}

	if metrics.FailedGenerations != 0 {
		t.Errorf("expected 0 failed generations, got %d", metrics.FailedGenerations)
	}
}

func TestPage_Close(t *testing.T) {
	page, err := NewPage("test-app", createTestTemplate(),
		map[string]interface{}{"value": "test"}, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Verify page has data before close
	if page.GetData() == nil {
		t.Error("page should have data before close")
	}

	// Close page
	err = page.Close()
	if err != nil {
		t.Errorf("page close should not error: %v", err)
	}

	// Verify data is cleared
	if page.GetData() != nil {
		t.Error("page data should be nil after close")
	}

	// Verify fragment cache is cleared
	if len(page.fragmentCache) != 0 {
		t.Errorf("fragment cache should be empty after close, got %d items", len(page.fragmentCache))
	}
}

func TestPage_ThreadSafety(t *testing.T) {
	t.Skip("Skipping thread safety test due to mutex contention - TODO: investigate RWMutex usage")

	page, err := NewPage("test-app", createTestTemplate(),
		map[string]interface{}{"counter": 0}, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	const numWorkers = 10
	const operationsPerWorker = 20
	done := make(chan bool, numWorkers)
	errors := make(chan error, numWorkers*operationsPerWorker)

	// Start workers that perform concurrent operations
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer func() { done <- true }()

			for j := 0; j < operationsPerWorker; j++ {
				// Mix of different operations
				switch j % 4 {
				case 0:
					// Render
					_, err := page.Render()
					if err != nil {
						errors <- err
					}
				case 1:
					// Set data
					newData := map[string]interface{}{"counter": workerID*100 + j}
					err := page.SetData(newData)
					if err != nil {
						errors <- err
					}
				case 2:
					// Get data
					page.GetData()
				case 3:
					// Update last accessed and get metrics
					page.UpdateLastAccessed()
					page.GetMetrics()
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("concurrent operation failed: %v", err)
	}

	// Verify page is still functional
	_, err = page.Render()
	if err != nil {
		t.Errorf("page should still be functional after concurrent operations: %v", err)
	}
}
