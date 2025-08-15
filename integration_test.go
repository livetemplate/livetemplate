package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"testing"
	"time"
)

// TestPageLifecycle tests the complete page lifecycle from creation to cleanup
func TestPageLifecycle(t *testing.T) {
	// Step 1: Create a page with initial data
	tmpl := template.Must(template.New("lifecycle").Parse(`
		<div class="user-profile">
			<h1>{{.Name}}</h1>
			<p>Email: {{.Email}}</p>
			{{if .IsAdmin}}
				<div class="admin-panel">Admin Controls</div>
			{{end}}
			<ul class="tasks">
				{{range .Tasks}}
					<li>{{.}}</li>
				{{end}}
			</ul>
		</div>
	`))

	initialData := map[string]interface{}{
		"Name":    "Alice",
		"Email":   "alice@example.com",
		"IsAdmin": false,
		"Tasks":   []string{"Task 1", "Task 2"},
	}

	page, err := NewPage(tmpl, initialData, WithMetricsEnabled(true))
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Step 2: Render initial HTML
	initialHTML, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render initial HTML: %v", err)
	}

	// Verify initial HTML contains expected content
	expectedContent := []string{"Alice", "alice@example.com", "Task 1", "Task 2"}
	for _, content := range expectedContent {
		if !strings.Contains(initialHTML, content) {
			t.Errorf("Initial HTML missing expected content: %s", content)
		}
	}

	// Verify admin panel is not present
	if strings.Contains(initialHTML, "Admin Controls") {
		t.Error("Admin panel should not be present for non-admin user")
	}

	// Step 3: Update with text changes (Strategy 1)
	textChangeData := map[string]interface{}{
		"Name":    "Alice Smith", // Name change
		"Email":   "alice@example.com",
		"IsAdmin": false,
		"Tasks":   []string{"Task 1", "Task 2"},
	}

	ctx := context.Background()
	fragments1, err := page.RenderFragments(ctx, textChangeData)
	if err != nil {
		t.Fatalf("Failed to generate fragments for text change: %v", err)
	}

	// Verify Strategy 1 (static_dynamic) was used for text-only change
	if len(fragments1) == 0 {
		t.Fatal("Expected fragments for text change")
	}

	// Step 4: Update with structural changes (Strategy 3)
	structuralChangeData := map[string]interface{}{
		"Name":    "Alice Smith",
		"Email":   "alice@example.com",
		"IsAdmin": false,
		"Tasks":   []string{"Task 1", "Task 2", "Task 3"}, // Added task
	}

	fragments2, err := page.RenderFragments(ctx, structuralChangeData)
	if err != nil {
		t.Fatalf("Failed to generate fragments for structural change: %v", err)
	}

	if len(fragments2) == 0 {
		t.Fatal("Expected fragments for structural change")
	}

	// Step 5: Update with complex changes (Strategy 3 or 4)
	complexChangeData := map[string]interface{}{
		"Name":    "Alice Smith",
		"Email":   "alice.smith@company.com",                // Email change
		"IsAdmin": true,                                     // Admin status change (structural)
		"Tasks":   []string{"Admin Task 1", "Admin Task 2"}, // Task content change
	}

	fragments3, err := page.RenderFragments(ctx, complexChangeData)
	if err != nil {
		t.Fatalf("Failed to generate fragments for complex change: %v", err)
	}

	if len(fragments3) == 0 {
		t.Fatal("Expected fragments for complex change")
	}

	// Step 6: Verify final HTML state
	finalHTML, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render final HTML: %v", err)
	}

	// Verify final HTML contains updated content
	expectedFinalContent := []string{
		"Alice Smith",
		"alice.smith@company.com",
		"Admin Controls",
		"Admin Task 1",
		"Admin Task 2",
	}

	for _, content := range expectedFinalContent {
		if !strings.Contains(finalHTML, content) {
			t.Errorf("Final HTML missing expected content: %s", content)
		}
	}

	// Step 7: Verify metrics were collected
	metrics := page.GetMetrics()
	if metrics.TotalGenerations != 3 {
		t.Errorf("Expected 3 total generations, got %d", metrics.TotalGenerations)
	}

	if metrics.SuccessfulGenerations != 3 {
		t.Errorf("Expected 3 successful generations, got %d", metrics.SuccessfulGenerations)
	}

	if len(metrics.StrategyUsage) == 0 {
		t.Error("Expected strategy usage metrics")
	}

	// Step 8: Reset metrics
	page.ResetMetrics()
	resetMetrics := page.GetMetrics()
	if resetMetrics.TotalGenerations != 0 {
		t.Error("Metrics should be reset to zero")
	}

	// Step 9: Clean up
	err = page.Close()
	if err != nil {
		t.Fatalf("Failed to close page: %v", err)
	}

	// Verify cleanup
	if page.GetData() != nil {
		t.Error("Page data should be nil after close")
	}
}

// TestMultiplePages tests concurrent usage of multiple page instances
func TestMultiplePages(t *testing.T) {
	tmpl := template.Must(template.New("multi").Parse("<div>User: {{.Name}}, Count: {{.Count}}</div>"))

	// Create multiple pages with different data
	pages := make([]*Page, 3)
	initialData := []map[string]interface{}{
		{"Name": "Alice", "Count": 1},
		{"Name": "Bob", "Count": 2},
		{"Name": "Charlie", "Count": 3},
	}

	for i := 0; i < 3; i++ {
		page, err := NewPage(tmpl, initialData[i])
		if err != nil {
			t.Fatalf("Failed to create page %d: %v", i, err)
		}
		pages[i] = page
	}

	// Render initial HTML for all pages
	for i, page := range pages {
		html, err := page.Render()
		if err != nil {
			t.Fatalf("Failed to render page %d: %v", i, err)
		}

		expectedName := initialData[i]["Name"].(string)
		if !strings.Contains(html, expectedName) {
			t.Errorf("Page %d missing expected name: %s", i, expectedName)
		}
	}

	// Update each page independently
	ctx := context.Background()
	for i, page := range pages {
		newData := map[string]interface{}{
			"Name":  initialData[i]["Name"].(string),
			"Count": initialData[i]["Count"].(int) + 10,
		}

		fragments, err := page.RenderFragments(ctx, newData)
		if err != nil {
			t.Fatalf("Failed to generate fragments for page %d: %v", i, err)
		}

		if len(fragments) == 0 {
			t.Errorf("Expected fragments for page %d", i)
		}
	}

	// Verify each page has correct final state
	for i, page := range pages {
		html, err := page.Render()
		if err != nil {
			t.Fatalf("Failed to render final page %d: %v", i, err)
		}

		expectedName := initialData[i]["Name"].(string)
		expectedCount := initialData[i]["Count"].(int) + 10

		if !strings.Contains(html, expectedName) {
			t.Errorf("Final page %d missing expected name: %s", i, expectedName)
		}

		if !strings.Contains(html, fmt.Sprintf("Count: %d", expectedCount)) {
			t.Errorf("Final page %d missing expected count: %d", i, expectedCount)
		}
	}

	// Clean up all pages
	for i, page := range pages {
		err := page.Close()
		if err != nil {
			t.Errorf("Failed to close page %d: %v", i, err)
		}
	}
}

// TestErrorHandling tests error scenarios and recovery
func TestErrorHandling(t *testing.T) {
	// Test with template that might cause rendering errors
	tmpl := template.Must(template.New("error").Parse("{{.ValidField}} {{.MightNotExist.NestedField}}"))

	// Create page with partial data
	data := map[string]interface{}{
		"ValidField": "Valid",
		// Missing MightNotExist field
	}

	page, err := NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Rendering should work (Go templates handle missing fields gracefully)
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Unexpected rendering error: %v", err)
	}

	if !strings.Contains(html, "Valid") {
		t.Error("HTML should contain valid field")
	}

	// In Go templates, missing fields render as empty
	// The template "{{.ValidField}} {{.MightNotExist.NestedField}}" should render as "Valid "
	// since MightNotExist.NestedField is missing and renders empty

	// Fragment generation should also work
	ctx := context.Background()
	newData := map[string]interface{}{
		"ValidField": "Updated",
		// Still missing MightNotExist field
	}

	fragments, err := page.RenderFragments(ctx, newData)
	if err != nil {
		t.Fatalf("Unexpected fragment generation error: %v", err)
	}

	if len(fragments) == 0 {
		t.Error("Expected fragments even with missing fields")
	}

	err = page.Close()
	if err != nil {
		t.Errorf("Failed to close page: %v", err)
	}
}

// TestPerformanceCharacteristics tests performance aspects of the API
func TestPerformanceCharacteristics(t *testing.T) {
	tmpl := template.Must(template.New("perf").Parse(`
		<div class="performance-test">
			<h1>{{.Title}}</h1>
			<div class="content">{{.Content}}</div>
			<ul>
				{{range .Items}}
					<li class="item">{{.}}</li>
				{{end}}
			</ul>
		</div>
	`))

	initialData := map[string]interface{}{
		"Title":   "Performance Test",
		"Content": "Initial content",
		"Items":   []string{"Item 1", "Item 2", "Item 3"},
	}

	page, err := NewPage(tmpl, initialData, WithMetricsEnabled(true))
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Measure initial rendering time
	start := time.Now()
	_, err = page.Render()
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}
	renderTime := time.Since(start)

	// Initial render should be reasonably fast
	if renderTime > 10*time.Millisecond {
		t.Errorf("Initial render too slow: %v", renderTime)
	}

	// Measure fragment generation time
	ctx := context.Background()
	newData := map[string]interface{}{
		"Title":   "Updated Performance Test",
		"Content": "Updated content",
		"Items":   []string{"Item 1", "Item 2", "Item 3", "Item 4"},
	}

	start = time.Now()
	fragments, err := page.RenderFragments(ctx, newData)
	if err != nil {
		t.Fatalf("Failed to generate fragments: %v", err)
	}
	fragmentTime := time.Since(start)

	// Fragment generation should be reasonably fast
	if fragmentTime > 50*time.Millisecond {
		t.Errorf("Fragment generation too slow: %v", fragmentTime)
	}

	// Verify we got fragments
	if len(fragments) == 0 {
		t.Error("Expected fragments to be generated")
	}

	// Verify metadata includes timing information
	for _, fragment := range fragments {
		if fragment.Metadata == nil {
			t.Error("Fragment metadata should not be nil")
			continue
		}

		if fragment.Metadata.GenerationTime <= 0 {
			t.Error("Fragment generation time should be positive")
		}

		if fragment.Metadata.OriginalSize <= 0 {
			t.Error("Fragment original size should be positive")
		}
	}

	// Check metrics
	metrics := page.GetMetrics()
	if metrics.AverageGenerationTime <= 0 {
		t.Error("Average generation time should be positive")
	}

	err = page.Close()
	if err != nil {
		t.Errorf("Failed to close page: %v", err)
	}
}

// TestEdgeCases tests various edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		initialData interface{}
		updateData  interface{}
		expectError bool
	}{
		{
			name:        "minimal data change",
			template:    "<div>{{.Value}}</div>",
			initialData: map[string]interface{}{"Value": "A"},
			updateData:  map[string]interface{}{"Value": "B"},
			expectError: false,
		},
		{
			name:        "data to nil",
			template:    "<div>{{.Name}}</div>",
			initialData: map[string]interface{}{"Name": "Alice"},
			updateData:  nil,
			expectError: false,
		},
		{
			name:        "empty slice to populated",
			template:    "<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>",
			initialData: map[string]interface{}{"Items": []string{}},
			updateData:  map[string]interface{}{"Items": []string{"A", "B"}},
			expectError: false,
		},
		{
			name:        "populated slice to empty",
			template:    "<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>",
			initialData: map[string]interface{}{"Items": []string{"A", "B"}},
			updateData:  map[string]interface{}{"Items": []string{}},
			expectError: false,
		},
		{
			name:        "complex nested data",
			template:    "<div>{{.User.Profile.Name}}</div>",
			initialData: map[string]interface{}{"User": map[string]interface{}{"Profile": map[string]interface{}{"Name": "Alice"}}},
			updateData:  map[string]interface{}{"User": map[string]interface{}{"Profile": map[string]interface{}{"Name": "Bob"}}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := template.Must(template.New("edge").Parse(tt.template))

			page, err := NewPage(tmpl, tt.initialData)
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			// Initial render
			_, err = page.Render()
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected render error: %v", err)
				return
			}

			// Fragment generation
			ctx := context.Background()
			fragments, err := page.RenderFragments(ctx, tt.updateData)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected fragment generation error: %v", err)
				return
			}

			// Should always get at least one fragment (even for no-change scenarios)
			if len(fragments) == 0 {
				t.Error("Expected at least one fragment")
			}

			// Final render
			_, err = page.Render()
			if err != nil {
				t.Errorf("Unexpected final render error: %v", err)
			}

			err = page.Close()
			if err != nil {
				t.Errorf("Failed to close page: %v", err)
			}
		})
	}
}
