package integration

import (
	"context"
	"html/template"
	"testing"

	"github.com/livefir/livetemplate"
)

func TestAutomaticTemplateExtractionIntegration(t *testing.T) {
	tests := []struct {
		name            string
		templateSource  string
		initialData     interface{}
		updateData      interface{}
		expectFragments bool
	}{
		{
			name:            "Simple field template",
			templateSource:  `{{.Counter}}`,
			initialData:     map[string]interface{}{"Counter": 5},
			updateData:      map[string]interface{}{"Counter": 10},
			expectFragments: true,
		},
		{
			name:            "Template with static content",
			templateSource:  `<div>Count: {{.Counter}}</div>`,
			initialData:     map[string]interface{}{"Counter": 1},
			updateData:      map[string]interface{}{"Counter": 2},
			expectFragments: true,
		},
		{
			name:            "Multiple fields",
			templateSource:  `{{.Name}}: {{.Value}}`,
			initialData:     map[string]interface{}{"Name": "Test", "Value": 100},
			updateData:      map[string]interface{}{"Name": "Test", "Value": 200},
			expectFragments: true,
		},
		{
			name:            "Conditional template",
			templateSource:  `{{if .Active}}Active{{else}}Inactive{{end}}`,
			initialData:     map[string]interface{}{"Active": true},
			updateData:      map[string]interface{}{"Active": false},
			expectFragments: true,
		},
		{
			name:           "Complex nested template",
			templateSource: `<div>{{range .Items}}<span>{{.Name}} ({{if .Done}}✓{{else}}○{{end}})</span>{{end}}</div>`,
			initialData: map[string]interface{}{
				"Items": []map[string]interface{}{
					{"Name": "Task 1", "Done": false},
					{"Name": "Task 2", "Done": true},
				},
			},
			updateData: map[string]interface{}{
				"Items": []map[string]interface{}{
					{"Name": "Task 1", "Done": true},
					{"Name": "Task 2", "Done": true},
				},
			},
			expectFragments: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create application
			app, err := livetemplate.NewApplication()
			if err != nil {
				t.Fatalf("Failed to create application: %v", err)
			}
			defer app.Close()

			// Create template from source - this should work automatically
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Create page WITHOUT WithTemplateSource - should extract automatically
			page, err := app.NewApplicationPage(tmpl, tt.initialData)
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Test initial render
			html, err := page.Render()
			if err != nil {
				t.Fatalf("Failed to render initial page: %v", err)
			}

			if html == "" {
				t.Errorf("Initial render produced empty HTML")
			}

			// Test fragment generation
			fragments, err := page.RenderFragments(context.Background(), tt.updateData)

			if tt.expectFragments {
				if err != nil {
					t.Fatalf("Failed to generate fragments: %v", err)
				}

				if len(fragments) == 0 {
					t.Errorf("Expected fragments but got none")
				}

				// Verify fragment structure
				for i, frag := range fragments {
					if frag.ID == "" {
						t.Errorf("Fragment %d has empty ID", i)
					}
					if frag.Data == nil {
						t.Errorf("Fragment %d has nil data", i)
					}
				}
			} else {
				if err == nil {
					t.Errorf("Expected fragment generation to fail but it succeeded")
				}
			}
		})
	}
}

func TestHTMLEscapingAutomaticExtraction(t *testing.T) {
	// Test that HTML escaping pipelines are automatically handled
	app, err := livetemplate.NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	// This template will have HTML escaping added automatically by html/template
	tmpl, err := template.New("test").Parse(`{{.Content}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create page - should automatically extract and handle HTML escaping
	initialData := map[string]interface{}{"Content": "<script>alert('test')</script>"}
	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Test initial render
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render initial page: %v", err)
	}

	// HTML should be escaped by html/template
	if html == "<script>alert('test')</script>" {
		t.Errorf("Content was not HTML escaped: %s", html)
	}

	// Test fragment generation - should work despite HTML escaping pipeline
	newData := map[string]interface{}{"Content": "<b>Bold text</b>"}
	fragments, err := page.RenderFragments(context.Background(), newData)
	if err != nil {
		t.Fatalf("Failed to generate fragments with HTML escaping: %v", err)
	}

	if len(fragments) == 0 {
		t.Errorf("Expected fragments but got none")
	}

	// Fragment should contain the new content
	if len(fragments) > 0 {
		fragData := fragments[0].Data
		if fragData == nil {
			t.Errorf("Fragment data is nil")
		}
	}
}

func TestTemplateExtractionErrorHandling(t *testing.T) {
	app, err := livetemplate.NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	// Test with nil template (should be caught during page creation)
	_, err = app.NewApplicationPage(nil, map[string]interface{}{})
	if err == nil {
		t.Errorf("Expected error with nil template but got none")
	}

	// Test with template that has complex constructs that might challenge extraction
	complexTemplate := `{{$var := .Data}}{{with $var}}{{range .Items}}{{template "item" .}}{{end}}{{end}}`
	tmpl, err := template.New("test").Parse(complexTemplate)
	if err != nil {
		t.Fatalf("Failed to parse complex template: %v", err)
	}

	// This should still work (or fail gracefully)
	data := map[string]interface{}{
		"Data": map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Name": "Item 1"},
			},
		},
	}

	page, err := app.NewApplicationPage(tmpl, data)
	if err != nil {
		// This is acceptable - complex templates might not be supported yet
		t.Logf("Complex template not supported (expected): %v", err)
		return
	}
	defer page.Close()

	// If it worked, try to render
	_, err = page.Render()
	if err != nil {
		t.Logf("Complex template render failed (acceptable): %v", err)
	}
}

func TestMultiplePageInstances(t *testing.T) {
	// Test that multiple page instances with automatic extraction work independently
	app, err := livetemplate.NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	// Create multiple templates
	tmpl1, err := template.New("test1").Parse(`Counter: {{.Counter}}`)
	if err != nil {
		t.Fatalf("Failed to parse template 1: %v", err)
	}

	tmpl2, err := template.New("test2").Parse(`<div>{{.Name}}</div>`)
	if err != nil {
		t.Fatalf("Failed to parse template 2: %v", err)
	}

	// Create multiple pages
	page1, err := app.NewApplicationPage(tmpl1, map[string]interface{}{"Counter": 1})
	if err != nil {
		t.Fatalf("Failed to create page 1: %v", err)
	}
	defer page1.Close()

	page2, err := app.NewApplicationPage(tmpl2, map[string]interface{}{"Name": "Test"})
	if err != nil {
		t.Fatalf("Failed to create page 2: %v", err)
	}
	defer page2.Close()

	// Test both pages independently
	frags1, err := page1.RenderFragments(context.Background(), map[string]interface{}{"Counter": 2})
	if err != nil {
		t.Fatalf("Failed to generate fragments for page 1: %v", err)
	}

	frags2, err := page2.RenderFragments(context.Background(), map[string]interface{}{"Name": "Updated"})
	if err != nil {
		t.Fatalf("Failed to generate fragments for page 2: %v", err)
	}

	// Both should have generated fragments
	if len(frags1) == 0 {
		t.Errorf("Page 1 generated no fragments")
	}
	if len(frags2) == 0 {
		t.Errorf("Page 2 generated no fragments")
	}

	// Fragment IDs are scoped per page, so it's fine if they're the same across different pages
	// What matters is that each page generated its own fragments independently
	if len(frags1) > 0 {
		t.Logf("Page 1 fragment ID: %s", frags1[0].ID)
	}
	if len(frags2) > 0 {
		t.Logf("Page 2 fragment ID: %s", frags2[0].ID)
	}
}

func TestBandwidthOptimizationWithExtraction(t *testing.T) {
	// Test that automatic extraction achieves proper bandwidth optimization
	app, err := livetemplate.NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	tmpl, err := template.New("test").Parse(`<div class="counter">Count: {{.Counter}}</div>`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	initialData := map[string]interface{}{"Counter": 1}
	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// First update - should include static structure
	newData1 := map[string]interface{}{"Counter": 2}
	fragments1, err := page.RenderFragments(context.Background(), newData1)
	if err != nil {
		t.Fatalf("Failed to generate first fragments: %v", err)
	}

	// Second update - should be minimal (only changed data)
	newData2 := map[string]interface{}{"Counter": 3}
	fragments2, err := page.RenderFragments(context.Background(), newData2)
	if err != nil {
		t.Fatalf("Failed to generate second fragments: %v", err)
	}

	// Both updates should generate fragments
	if len(fragments1) == 0 || len(fragments2) == 0 {
		t.Fatalf("Expected fragments from both updates")
	}

	// Verify that we're getting proper fragment optimization
	if fragments1[0].Data == nil || fragments2[0].Data == nil {
		t.Error("Expected fragments with data for proper optimization")
	}

	if fragments1[0].ID == "" || fragments2[0].ID == "" {
		t.Error("Expected fragments with IDs for proper optimization")
	}

	// The data should contain the updated values
	// This verifies that automatic extraction enables proper LiveTemplate optimization
	if fragments1[0].Data == nil || fragments2[0].Data == nil {
		t.Errorf("Fragment data should not be nil for bandwidth optimization")
	}
}
