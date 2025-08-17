package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name          string
		template      *template.Template
		data          interface{}
		options       []PageOption
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid page creation",
			template:    template.Must(template.New("test").Parse("<div>{{.Name}}</div>")),
			data:        map[string]interface{}{"Name": "Alice"},
			options:     nil,
			expectError: false,
		},
		{
			name:          "nil template",
			template:      nil,
			data:          map[string]interface{}{"Name": "Alice"},
			options:       nil,
			expectError:   true,
			errorContains: "template cannot be nil",
		},
		{
			name:        "with metrics disabled",
			template:    template.Must(template.New("test").Parse("<div>{{.Name}}</div>")),
			data:        map[string]interface{}{"Name": "Alice"},
			options:     []PageOption{WithMetricsEnabled(false)},
			expectError: false,
		},
		{
			name:        "with fallback disabled",
			template:    template.Must(template.New("test").Parse("<div>{{.Name}}</div>")),
			data:        map[string]interface{}{"Name": "Alice"},
			options:     []PageOption{WithFallbackEnabled(false)},
			expectError: false,
		},
		{
			name:        "with max generation time",
			template:    template.Must(template.New("test").Parse("<div>{{.Name}}</div>")),
			data:        map[string]interface{}{"Name": "Alice"},
			options:     []PageOption{WithMaxGenerationTime(2 * time.Second)},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := NewPage(tt.template, tt.data, tt.options...)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if page == nil {
				t.Error("Expected page instance, got nil")
				return
			}

			// Verify page properties
			if page.template != tt.template {
				t.Error("Template not set correctly")
			}

			if !reflect.DeepEqual(page.data, tt.data) {
				t.Error("Data not set correctly")
			}

			if page.updateGenerator == nil {
				t.Error("Update generator not initialized")
			}

			if page.created.IsZero() {
				t.Error("Creation time not set")
			}
		})
	}
}

func TestPage_Render(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        interface{}
		expected    string
		expectError bool
	}{
		{
			name:        "simple text substitution",
			template:    "<div>Hello {{.Name}}</div>",
			data:        map[string]interface{}{"Name": "Alice"},
			expected:    "<div>Hello Alice</div>",
			expectError: false,
		},
		{
			name:        "multiple substitutions",
			template:    "<h1>{{.Title}}</h1><p>{{.Content}}</p>",
			data:        map[string]interface{}{"Title": "Welcome", "Content": "Hello World"},
			expected:    "<h1>Welcome</h1><p>Hello World</p>",
			expectError: false,
		},
		{
			name:        "conditional rendering - true",
			template:    "{{if .Show}}<div>Visible</div>{{end}}",
			data:        map[string]interface{}{"Show": true},
			expected:    "<div>Visible</div>",
			expectError: false,
		},
		{
			name:        "conditional rendering - false",
			template:    "{{if .Show}}<div>Hidden</div>{{end}}",
			data:        map[string]interface{}{"Show": false},
			expected:    "",
			expectError: false,
		},
		{
			name:        "range iteration",
			template:    "<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>",
			data:        map[string]interface{}{"Items": []string{"A", "B", "C"}},
			expected:    "<ul><li>A</li><li>B</li><li>C</li></ul>",
			expectError: false,
		},
		{
			name:        "empty data",
			template:    "<div>{{.Missing}}</div>",
			data:        map[string]interface{}{},
			expected:    "<div></div>",
			expectError: false,
		},
		{
			name:        "nil data",
			template:    "<div>Static content</div>",
			data:        nil,
			expected:    "<div>Static content</div>",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := template.Must(template.New("test").Parse(tt.template))
			page, err := NewPage(tmpl, tt.data)
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			result, err := page.Render()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Render() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPage_RenderFragments(t *testing.T) {
	tests := []struct {
		name            string
		template        string
		oldData         interface{}
		newData         interface{}
		expectError     bool
		expectFragments bool
	}{
		{
			name:            "text change",
			template:        "<div>{{.Name}}</div>",
			oldData:         map[string]interface{}{"Name": "Alice"},
			newData:         map[string]interface{}{"Name": "Bob"},
			expectError:     false,
			expectFragments: true,
		},
		{
			name:            "attribute change",
			template:        "<div class=\"{{.Class}}\">Content</div>",
			oldData:         map[string]interface{}{"Class": "old"},
			newData:         map[string]interface{}{"Class": "new"},
			expectError:     false,
			expectFragments: true,
		},
		{
			name:            "structural change",
			template:        "<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>",
			oldData:         map[string]interface{}{"Items": []string{"A"}},
			newData:         map[string]interface{}{"Items": []string{"A", "B"}},
			expectError:     false,
			expectFragments: true,
		},
		{
			name:            "complex change",
			template:        "{{if .ShowTable}}<table><tr><td>{{.Value}}</td></tr></table>{{else}}<div>{{.Value}}</div>{{end}}",
			oldData:         map[string]interface{}{"ShowTable": false, "Value": "Data"},
			newData:         map[string]interface{}{"ShowTable": true, "Value": "Updated"},
			expectError:     false,
			expectFragments: true,
		},
		{
			name:            "no change",
			template:        "<div>{{.Name}}</div>",
			oldData:         map[string]interface{}{"Name": "Alice"},
			newData:         map[string]interface{}{"Name": "Alice"},
			expectError:     false,
			expectFragments: true, // Still generates fragments even if no change
		},
		{
			name:            "empty to content",
			template:        "<div>{{.Content}}</div>",
			oldData:         map[string]interface{}{"Content": ""},
			newData:         map[string]interface{}{"Content": "Hello"},
			expectError:     false,
			expectFragments: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := template.Must(template.New("test").Parse(tt.template))
			page, err := NewPage(tmpl, tt.oldData)
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			ctx := context.Background()
			fragments, err := page.RenderFragments(ctx, tt.newData)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectFragments && len(fragments) == 0 {
				t.Error("Expected fragments but got none")
				return
			}

			// Verify fragment structure
			for i, fragment := range fragments {
				if fragment.ID == "" {
					t.Errorf("Fragment %d: ID should not be empty", i)
				}

				if fragment.Strategy == "" {
					t.Errorf("Fragment %d: Strategy should not be empty", i)
				}

				if fragment.Action == "" {
					t.Errorf("Fragment %d: Action should not be empty", i)
				}

				if fragment.Data == nil {
					t.Errorf("Fragment %d: Data should not be nil", i)
				}

				if fragment.Metadata == nil {
					t.Errorf("Fragment %d: Metadata should not be nil", i)
				} else {
					if fragment.Metadata.GenerationTime <= 0 {
						t.Errorf("Fragment %d: GenerationTime should be positive", i)
					}

					if fragment.Metadata.OriginalSize <= 0 {
						t.Errorf("Fragment %d: OriginalSize should be positive", i)
					}

					if fragment.Metadata.Strategy < 1 || fragment.Metadata.Strategy > 4 {
						t.Errorf("Fragment %d: Strategy number should be 1-4, got %d", i, fragment.Metadata.Strategy)
					}

					if fragment.Metadata.Confidence <= 0 || fragment.Metadata.Confidence > 1 {
						t.Errorf("Fragment %d: Confidence should be between 0 and 1, got %f", i, fragment.Metadata.Confidence)
					}
				}
			}

			// Verify data was updated
			updatedData := page.GetData()
			if !reflect.DeepEqual(updatedData, tt.newData) {
				t.Error("Page data was not updated after fragment generation")
			}
		})
	}
}

func TestPage_UpdateData(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("<div>{{.Name}}</div>"))
	oldData := map[string]interface{}{"Name": "Alice"}
	newData := map[string]interface{}{"Name": "Bob"}

	page, err := NewPage(tmpl, oldData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Verify initial data
	currentData := page.GetData()
	if !reflect.DeepEqual(currentData, oldData) {
		t.Error("Initial data not set correctly")
	}

	// Update data
	returnedData := page.UpdateData(newData)
	if !reflect.DeepEqual(returnedData, newData) {
		t.Error("UpdateData should return the new data")
	}

	// Verify data was updated
	updatedData := page.GetData()
	if !reflect.DeepEqual(updatedData, newData) {
		t.Error("Data was not updated correctly")
	}
}

func TestPage_SetTemplate(t *testing.T) {
	tmpl1 := template.Must(template.New("test1").Parse("<div>{{.Name}}</div>"))
	tmpl2 := template.Must(template.New("test2").Parse("<span>{{.Name}}</span>"))
	data := map[string]interface{}{"Name": "Alice"}

	page, err := NewPage(tmpl1, data)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Verify initial template
	if page.GetTemplate() != tmpl1 {
		t.Error("Initial template not set correctly")
	}

	// Update template
	err = page.SetTemplate(tmpl2)
	if err != nil {
		t.Errorf("Failed to set template: %v", err)
	}

	// Verify template was updated
	if page.GetTemplate() != tmpl2 {
		t.Error("Template was not updated correctly")
	}

	// Test setting nil template
	err = page.SetTemplate(nil)
	if err == nil {
		t.Error("Expected error when setting nil template")
	}
	if !strings.Contains(err.Error(), "template cannot be nil") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestPage_Metrics(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("<div>{{.Name}}</div>"))
	data := map[string]interface{}{"Name": "Alice"}

	// Test with metrics enabled
	page, err := NewPage(tmpl, data, WithMetricsEnabled(true))
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Generate some fragments to have metrics
	ctx := context.Background()
	newData := map[string]interface{}{"Name": "Bob"}
	_, err = page.RenderFragments(ctx, newData)
	if err != nil {
		t.Fatalf("Failed to generate fragments: %v", err)
	}

	// Check metrics
	metrics := page.GetMetrics()
	if metrics.TotalGenerations == 0 {
		t.Error("Expected non-zero total generations")
	}
	if metrics.SuccessfulGenerations == 0 {
		t.Error("Expected non-zero successful generations")
	}

	// Reset metrics
	page.ResetMetrics()
	resetMetrics := page.GetMetrics()
	if resetMetrics.TotalGenerations != 0 {
		t.Error("Metrics should be reset")
	}

	// Test with metrics disabled
	pageNoMetrics, err := NewPage(tmpl, data, WithMetricsEnabled(false))
	if err != nil {
		t.Fatalf("Failed to create page with metrics disabled: %v", err)
	}

	metricsDisabled := pageNoMetrics.GetMetrics()
	if metricsDisabled == nil {
		t.Error("GetMetrics should return empty metrics when disabled, not nil")
	}
}

func TestPage_Close(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("<div>{{.Name}}</div>"))
	data := map[string]interface{}{"Name": "Alice"}

	page, err := NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Verify page has data before closing
	if page.GetData() == nil {
		t.Error("Page should have data before closing")
	}

	// Close the page
	err = page.Close()
	if err != nil {
		t.Errorf("Failed to close page: %v", err)
	}

	// Verify data is cleared after closing
	if page.GetData() != nil {
		t.Error("Page data should be nil after closing")
	}
}

func TestPage_GetCreatedTime(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("<div>{{.Name}}</div>"))
	data := map[string]interface{}{"Name": "Alice"}

	before := time.Now()
	page, err := NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	after := time.Now()

	createdTime := page.GetCreatedTime()
	if createdTime.Before(before) || createdTime.After(after) {
		t.Errorf("Created time %v should be between %v and %v", createdTime, before, after)
	}
}

func TestPage_ConcurrentAccess(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse("<div>{{.Name}}</div>"))
	data := map[string]interface{}{"Name": "Alice"}

	page, err := NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Test concurrent access to data and rendering
	done := make(chan bool)
	errorChan := make(chan error, 20)

	// Start multiple goroutines for concurrent operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Update data
			newData := map[string]interface{}{"Name": fmt.Sprintf("User-%d", id)}
			page.UpdateData(newData)

			// Render
			_, err := page.Render()
			if err != nil {
				errorChan <- fmt.Errorf("render error in goroutine %d: %w", id, err)
				return
			}

			// Generate fragments
			ctx := context.Background()
			anotherData := map[string]interface{}{"Name": fmt.Sprintf("Updated-%d", id)}
			_, err = page.RenderFragments(ctx, anotherData)
			if err != nil {
				errorChan <- fmt.Errorf("fragment error in goroutine %d: %w", id, err)
				return
			}

			// Get data
			_ = page.GetData()
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	close(errorChan)
	for err := range errorChan {
		t.Error(err)
	}
}
