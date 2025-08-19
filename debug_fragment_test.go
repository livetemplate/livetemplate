package livetemplate

import (
	"context"
	"encoding/json"
	"html/template"
	"testing"
)

// TestFragmentGenerationDebug helps debug why fragments aren't being generated
func TestFragmentGenerationDebug(t *testing.T) {
	// Create application and page
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Simple template for testing
	tmplStr := `
<div id="test">
	<h1 id="title">{{.Title}}</h1>
	<div id="counter">Count: {{.Count}}</div>
	<div id="status" class="{{.Status}}">Status: {{.Status}}</div>
</div>`

	tmpl, err := template.New("debug").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Initial data
	initialData := &TestData{
		Title:   "Initial Title",
		Count:   0,
		Items:   []string{"Item 1", "Item 2"},
		Visible: true,
		Status:  "ready",
		Attrs:   map[string]string{"data-test": "initial"},
	}

	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() { _ = page.Close() }()

	// Test initial render
	initialHTML, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render initial HTML: %v", err)
	}
	t.Logf("Initial HTML: %s", initialHTML)

	// Test fragment generation with changed data
	newData := &TestData{
		Title:   "Updated Title",
		Count:   5,
		Items:   []string{"Item 1", "Item 2"},
		Visible: true,
		Status:  "active",
		Attrs:   map[string]string{"data-test": "updated"},
	}

	fragments, err := page.RenderFragments(context.Background(), newData)
	if err != nil {
		t.Fatalf("Failed to generate fragments: %v", err)
	}

	t.Logf("Generated %d fragments", len(fragments))
	for i, fragment := range fragments {
		fragmentJSON, _ := json.MarshalIndent(fragment, "", "  ")
		t.Logf("Fragment %d: %s", i, string(fragmentJSON))
	}

	if len(fragments) == 0 {
		t.Error("No fragments were generated - this indicates an issue with the fragment generation pipeline")
	}

	// Test new render to see what changed
	newHTML, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render new HTML: %v", err)
	}
	t.Logf("New HTML: %s", newHTML)
}
