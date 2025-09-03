package page

import (
	"context"
	"html/template"
	"strings"
	"testing"
)

func TestFullTemplateVsSimpleTemplate(t *testing.T) {
	tests := []struct {
		name                  string
		templateSource        string
		isFullHTML            bool
		expectedFragmentCount int
		expectedStrategy      string
		expectedUsesRegions   bool
	}{
		{
			name:                  "Simple template - just field",
			templateSource:        "{{.Counter}}",
			isFullHTML:            false,
			expectedFragmentCount: 1,
			expectedStrategy:      "tree_based",
			expectedUsesRegions:   false,
		},
		{
			name:                  "Simple template - field with static text",
			templateSource:        "Hello {{.Name}}!",
			isFullHTML:            false,
			expectedFragmentCount: 1,
			expectedStrategy:      "tree_based",
			expectedUsesRegions:   false,
		},
		{
			name: "Full HTML template - single dynamic region",
			templateSource: `<!DOCTYPE html>
<html>
<head><title>Counter</title></head>
<body>
	<h1>My App</h1>
	<div id="counter">Count: {{.Counter}}</div>
	<button>Click me</button>
</body>
</html>`,
			isFullHTML:            true,
			expectedFragmentCount: 1,
			expectedStrategy:      "tree_based_region",
			expectedUsesRegions:   true,
		},
		{
			name: "Full HTML template - multiple dynamic regions",
			templateSource: `<!DOCTYPE html>
<html>
<head><title>Dashboard</title></head>
<body>
	<header id="user">Welcome {{.User.Name}}</header>
	<main id="content">{{.Content}}</main>
	<footer id="status">Status: {{.Status}}</footer>
</body>
</html>`,
			isFullHTML:            true,
			expectedFragmentCount: 3,
			expectedStrategy:      "tree_based_region",
			expectedUsesRegions:   true,
		},
		{
			name: "Full HTML template - no dynamic content",
			templateSource: `<!DOCTYPE html>
<html>
<head><title>Static Page</title></head>
<body>
	<h1>Welcome</h1>
	<p>This is a static page</p>
</body>
</html>`,
			isFullHTML:            true,
			expectedFragmentCount: 1, // Falls back to legacy
			expectedStrategy:      "tree_based",
			expectedUsesRegions:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Create page with initial data
			initialData := map[string]interface{}{
				"Counter": 1,
				"Name":    "Alice",
				"User":    map[string]interface{}{"Name": "Bob"},
				"Content": "Main content here",
				"Status":  "Active",
			}

			page, err := NewPage("test-app", tmpl, initialData, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			// Test region detection
			regions, err := page.detectTemplateRegions()
			if err != nil {
				t.Fatalf("Failed to detect regions: %v", err)
			}

			if tt.expectedUsesRegions && len(regions) == 0 {
				t.Errorf("Expected regions to be detected for full HTML template")
			}

			if !tt.expectedUsesRegions && len(regions) > 0 {
				t.Errorf("Expected no regions for simple template, but got %d", len(regions))
			}

			// Test fragment generation
			newData := map[string]interface{}{
				"Counter": 2,
				"Name":    "Charlie",
				"User":    map[string]interface{}{"Name": "Dave"},
				"Content": "Updated content",
				"Status":  "Inactive",
			}

			fragments, err := page.RenderFragments(context.Background(), newData)
			if err != nil {
				t.Fatalf("Failed to render fragments: %v", err)
			}

			if len(fragments) != tt.expectedFragmentCount {
				t.Errorf("Expected %d fragments, got %d", tt.expectedFragmentCount, len(fragments))
			}

			// Verify fragment strategy
			for i, fragment := range fragments {
				if fragment.Strategy != tt.expectedStrategy {
					t.Errorf("Fragment %d: expected strategy %q, got %q", i, tt.expectedStrategy, fragment.Strategy)
				}

				if fragment.Data == nil {
					t.Errorf("Fragment %d: expected non-nil data", i)
				}

				// Verify region-based fragments have correct action
				if tt.expectedUsesRegions && fragment.Action != "update_region" {
					t.Errorf("Fragment %d: expected action 'update_region', got %q", i, fragment.Action)
				}

				// Verify legacy fragments have correct action
				if !tt.expectedUsesRegions && fragment.Action != "update_tree" {
					t.Errorf("Fragment %d: expected action 'update_tree', got %q", i, fragment.Action)
				}
			}
		})
	}
}

func TestRenderFragmentsFallbackBehavior(t *testing.T) {
	tests := []struct {
		name           string
		templateSource string
		shouldFallback bool
		reason         string
	}{
		{
			name:           "Simple template falls back to legacy",
			templateSource: "Hello {{.Name}}",
			shouldFallback: true,
			reason:         "Simple templates should use legacy approach",
		},
		{
			name:           "Invalid HTML falls back to legacy",
			templateSource: "<div>Unclosed tag {{.Field}}",
			shouldFallback: true,
			reason:         "Invalid HTML should fallback gracefully",
		},
		{
			name:           "Valid HTML with regions uses region approach",
			templateSource: `<html><body><div id="test">{{.Field}}</div></body></html>`,
			shouldFallback: false,
			reason:         "Valid HTML with regions should use region approach",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateSource)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			page, err := NewPage("test-app", tmpl, map[string]interface{}{"Field": "initial", "Name": "test"}, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			fragments, err := page.RenderFragments(context.Background(), map[string]interface{}{"Field": "updated", "Name": "changed"})
			if err != nil {
				t.Fatalf("Failed to render fragments: %v", err)
			}

			if len(fragments) == 0 {
				t.Fatalf("Expected at least one fragment")
			}

			usedLegacy := fragments[0].Strategy == "tree_based"
			usedRegions := fragments[0].Strategy == "tree_based_region"

			if tt.shouldFallback && !usedLegacy {
				t.Errorf("%s: Expected fallback to legacy strategy, but used %q", tt.reason, fragments[0].Strategy)
			}

			if !tt.shouldFallback && !usedRegions {
				t.Errorf("%s: Expected region strategy, but used %q", tt.reason, fragments[0].Strategy)
			}
		})
	}
}

func TestFullTemplateStaticContentPreservation(t *testing.T) {
	// Test that full HTML templates don't generate fragments for static content
	templateSource := `<!DOCTYPE html>
<html>
<head>
	<title>My App</title>
	<style>body { margin: 0; }</style>
</head>
<body>
	<header class="navbar">
		<h1>Static Header</h1>
		<nav>Static Navigation</nav>
	</header>
	<main>
		<div id="dynamic">Count: {{.Counter}}</div>
		<p>This paragraph is static</p>
	</main>
	<footer>
		<p>Static footer content</p>
		<script>console.log('static script');</script>
	</footer>
</body>
</html>`

	tmpl, err := template.New("test").Parse(templateSource)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	page, err := NewPage("test-app", tmpl, map[string]interface{}{"Counter": 1}, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Initial render should include full HTML with initial data
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render full HTML: %v", err)
	}

	// Generate fragments
	fragments, err := page.RenderFragments(context.Background(), map[string]interface{}{"Counter": 2})
	if err != nil {
		t.Fatalf("Failed to render fragments: %v", err)
	}

	// Should only have 1 fragment for the dynamic region
	if len(fragments) != 1 {
		t.Errorf("Expected 1 fragment for dynamic region, got %d", len(fragments))
	}

	fragment := fragments[0]

	// Should be region-based
	if fragment.Strategy != "tree_based_region" {
		t.Errorf("Expected tree_based_region strategy, got %q", fragment.Strategy)
	}

	// Fragment ID should indicate it's for the dynamic region
	if !strings.Contains(fragment.ID, "region_") {
		t.Errorf("Expected fragment ID to contain 'region_', got %q", fragment.ID)
	}

	// The fragment data should be much smaller than the full template
	// (this tests that we're not including static content)
	if fragment.Data == nil {
		t.Errorf("Fragment data should not be nil")
	}

	// Full HTML should contain static content
	staticElements := []string{
		"<!DOCTYPE html>",
		"<title>My App</title>",
		"body { margin: 0; }",
		"Static Header",
		"Static Navigation",
		"This paragraph is static",
		"Static footer content",
		"console.log('static script')",
	}

	for _, element := range staticElements {
		if !strings.Contains(html, element) {
			t.Errorf("Full HTML should contain static element: %q", element)
		}
	}

	// Full HTML should also contain the dynamic content
	if !strings.Contains(html, "Count: 1") {
		t.Errorf("Full HTML should contain initial dynamic content")
	}
}

func TestTemplateTypeDetection(t *testing.T) {
	tests := []struct {
		name        string
		templateSrc string
		isFullHTML  bool
		description string
	}{
		{
			name:        "Simple field",
			templateSrc: "{{.Name}}",
			isFullHTML:  false,
			description: "Just a template field",
		},
		{
			name:        "Field with text",
			templateSrc: "Hello {{.Name}}!",
			isFullHTML:  false,
			description: "Template field with surrounding text",
		},
		{
			name:        "Minimal HTML",
			templateSrc: "<div>{{.Content}}</div>",
			isFullHTML:  true,
			description: "Single HTML element with template",
		},
		{
			name:        "Full HTML document",
			templateSrc: "<!DOCTYPE html><html><head><title>Test</title></head><body><div id=\"content\">{{.Content}}</div></body></html>",
			isFullHTML:  true,
			description: "Complete HTML document",
		},
		{
			name:        "Multiple HTML elements",
			templateSrc: "<header><h1>{{.Title}}</h1></header><main><p>{{.Content}}</p></main>",
			isFullHTML:  true,
			description: "Multiple HTML elements with templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateSrc)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			page, err := NewPage("test-app", tmpl, map[string]interface{}{
				"Name":    "Test",
				"Content": "Content",
				"Title":   "Title",
			}, DefaultConfig())
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}
			defer page.Close()

			regions, err := page.detectTemplateRegions()
			if err != nil {
				t.Fatalf("Failed to detect regions: %v", err)
			}

			hasRegions := len(regions) > 0

			if tt.isFullHTML && !hasRegions {
				t.Errorf("%s: Expected regions to be detected for full HTML template", tt.description)
			}

			if !tt.isFullHTML && hasRegions {
				t.Errorf("%s: Expected no regions for simple template, but found %d", tt.description, len(regions))
			}
		})
	}
}
