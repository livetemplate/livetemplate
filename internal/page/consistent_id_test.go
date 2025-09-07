package page

import (
	"context"
	"html/template"
	"strings"
	"testing"
)

// TestConsistentIDsBetweenRenders verifies that IDs remain consistent across multiple Render calls
func TestConsistentIDsBetweenRenders(t *testing.T) {
	// Create a template with dynamic content
	tmplStr := `<div style="color: {{.Color}}">Hello {{.Counter}} World</div>`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create initial page
	data := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff0000",
	}

	page, err := NewPage("app123", tmpl, data, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// First render
	html1, err := page.Render()
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	// Extract lvt-id from first render
	id1 := extractLvtID(html1)
	if id1 == "" {
		t.Fatal("No lvt-id found in first render")
	}

	// Second render (simulating what happens in ServeHTTP)
	html2, err := page.Render()
	if err != nil {
		t.Fatalf("Second render failed: %v", err)
	}

	// Extract lvt-id from second render
	id2 := extractLvtID(html2)
	if id2 == "" {
		t.Fatal("No lvt-id found in second render")
	}

	// IDs should be identical
	if id1 != id2 {
		t.Errorf("IDs are not consistent between renders: first=%s, second=%s", id1, id2)
	}
}

// TestFragmentIDMatchesRenderedID verifies that fragment IDs match the rendered HTML IDs
func TestFragmentIDMatchesRenderedID(t *testing.T) {
	// Create a template with dynamic content
	tmplStr := `<div style="color: {{.Color}}">Counter: {{.Counter}}</div>`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create initial page
	data := map[string]interface{}{
		"Counter": 0,
		"Color":   "#ff0000",
	}

	page, err := NewPage("app123", tmpl, data, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Render HTML
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Extract lvt-id from HTML
	htmlID := extractLvtID(html)
	if htmlID == "" {
		t.Fatal("No lvt-id found in rendered HTML")
	}

	// Update data and generate fragments
	newData := map[string]interface{}{
		"Counter": 1,
		"Color":   "#00ff00",
	}

	fragments, err := page.RenderFragments(context.Background(), newData)
	if err != nil {
		t.Fatalf("RenderFragments failed: %v", err)
	}

	if len(fragments) == 0 {
		t.Fatal("No fragments generated")
	}

	// Check that at least one fragment has the same ID as the HTML
	found := false
	for _, frag := range fragments {
		if frag.ID == htmlID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Fragment ID does not match rendered HTML ID. HTML ID: %s, Fragment IDs: %v",
			htmlID, getFragmentIDs(fragments))
	}
}

// TestMultipleRendersWithDataUpdates verifies IDs remain consistent even with data updates
func TestMultipleRendersWithDataUpdates(t *testing.T) {
	tmplStr := `<div class="counter">Value: {{.Value}}</div>`
	tmpl, err := template.New("test").Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	page, err := NewPage("app123", tmpl, map[string]interface{}{"Value": 0}, DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Collect IDs from multiple render cycles
	var ids []string

	for i := 0; i < 3; i++ {
		// Update data
		if err := page.SetData(map[string]interface{}{"Value": i}); err != nil {
			t.Fatalf("SetData %d failed: %v", i, err)
		}

		// Render
		html, err := page.Render()
		if err != nil {
			t.Fatalf("Render %d failed: %v", i, err)
		}

		id := extractLvtID(html)
		if id == "" {
			t.Fatalf("No lvt-id found in render %d", i)
		}
		ids = append(ids, id)
	}

	// All IDs should be the same
	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[0] {
			t.Errorf("ID changed between renders: render 0=%s, render %d=%s", ids[0], i, ids[i])
		}
	}
}

// Helper function to extract lvt-id from HTML
func extractLvtID(html string) string {
	start := strings.Index(html, `lvt-id="`)
	if start == -1 {
		return ""
	}
	start += 8
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// Helper function to get all fragment IDs
func getFragmentIDs(fragments []*Fragment) []string {
	var ids []string
	for _, f := range fragments {
		ids = append(ids, f.ID)
	}
	return ids
}
