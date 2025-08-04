package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// Test data structures for realtime testing
type Counter struct {
	Value       int    `json:"value"`
	LastUpdated string `json:"last_updated"`
	UpdateCount int    `json:"update_count"`
}

type Site struct {
	Name string `json:"name"`
}

type NavigationItem struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

type Navigation struct {
	MainItems []NavigationItem `json:"main_items"`
}

type PageData struct {
	Counter    *Counter    `json:"counter"`
	Site       *Site       `json:"site"`
	Navigation *Navigation `json:"navigation"`
}

// TestRealtimeExample tests the real-time web rendering functionality
func TestRealtimeExample(t *testing.T) {
	// Create real-time renderer with functional options
	renderer := statetemplate.NewRenderer(
		statetemplate.WithWrapperTag("div"),
		statetemplate.WithIDPrefix("fragment-"),
		statetemplate.WithPreserveBlocks(true),
	)

	// Template with block (similar to your example)
	templateContent := `<div>
	Current Count: {{.Counter.Value}}
	Last updated: {{.Counter.LastUpdated}}
	Total updates: {{.Counter.UpdateCount}}

	{{block "header" .}}
		<h1>{{.Site.Name}}</h1>
		<nav>
			{{range .Navigation.MainItems}}
				<a href="{{.URL}}">{{.Label}}</a>
			{{end}}
		</nav>
	{{end}}
</div>`

	// Add template
	err := renderer.Parse("main", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Verify fragments were created using stats
	stats := renderer.GetStats()
	if stats.TotalFragments == 0 {
		t.Error("Expected fragments to be extracted")
	}

	if stats.TemplateCount == 0 {
		t.Error("Expected at least one template")
	}

	t.Logf("Template count: %d", stats.TemplateCount)
	t.Logf("Fragment count: %d", stats.TotalFragments)
	t.Logf("Fragments by type: %+v", stats.FragmentsByType)

	// Initial data
	initialData := &PageData{
		Counter: &Counter{
			Value:       42,
			LastUpdated: "09:00:00",
			UpdateCount: 0,
		},
		Site: &Site{
			Name: "Test Site",
		},
		Navigation: &Navigation{
			MainItems: []NavigationItem{
				{URL: "/home", Label: "Home"},
				{URL: "/about", Label: "About"},
			},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Verify initial HTML contains expected content
	if !strings.Contains(fullHTML, "Current Count: 42") {
		t.Error("Initial HTML should contain counter value")
	}
	if !strings.Contains(fullHTML, "Test Site") {
		t.Error("Initial HTML should contain site name")
	}
	if !strings.Contains(fullHTML, "Home") && !strings.Contains(fullHTML, "About") {
		t.Error("Initial HTML should contain navigation items")
	}

	t.Logf("Initial HTML length: %d characters", len(fullHTML))

	// Start the renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Test real-time updates
	testRealtimeUpdatesSimple(t, renderer, updateChan, initialData)
}

// testRealtimeUpdates tests various real-time update scenarios
func testRealtimeUpdatesSimple(t *testing.T, renderer *statetemplate.Renderer, updateChan <-chan statetemplate.Update, baseData *PageData) {

	// Test 1: Update counter value
	t.Run("CounterUpdate", func(t *testing.T) {
		// Create fresh data for this test
		newData := &PageData{
			Counter: &Counter{
				Value:       43,
				LastUpdated: "09:01:00",
				UpdateCount: 1,
			},
			Site:       &Site{Name: baseData.Site.Name},
			Navigation: baseData.Navigation,
		}

		renderer.SendUpdate(newData)

		// Wait for update
		select {
		case update := <-updateChan:
			if update.FragmentID == "" {
				t.Error("Expected fragment ID in update")
			}
			if update.Action != "replace" {
				t.Errorf("Expected action 'replace', got %s", update.Action)
			}
			if !strings.Contains(update.HTML, "43") {
				t.Error("Updated HTML should contain new counter value")
			}
			t.Logf("Counter update - Fragment ID: %s", update.FragmentID)

		case <-time.After(3 * time.Second):
			t.Error("Timeout waiting for counter update")
		}
	})

	// Test 2: Update site name
	t.Run("SiteNameUpdate", func(t *testing.T) {
		// Create a fresh renderer for this test
		freshRenderer := statetemplate.NewRenderer(
			statetemplate.WithWrapperTag("div"),
			statetemplate.WithIDPrefix("fragment-"),
			statetemplate.WithPreserveBlocks(true),
		)

		// Use the same template content
		templateContent := `<div>
	Current Count: {{.Counter.Value}}
	Last updated: {{.Counter.LastUpdated}}
	Total updates: {{.Counter.UpdateCount}}

	{{block "header" .}}
		<h1>{{.Site.Name}}</h1>
		<nav>
			{{range .Navigation.MainItems}}
				<a href="{{.URL}}">{{.Label}}</a>
			{{end}}
		</nav>
	{{end}}
</div>`

		if err := freshRenderer.Parse("main", templateContent); err != nil {
			t.Fatalf("Failed to add template: %v", err)
		}

		// Set initial data
		_, err := freshRenderer.SetInitialData(baseData)
		if err != nil {
			t.Fatalf("Failed to set initial data: %v", err)
		}

		// Start the renderer and get update channel
		freshRenderer.Start()
		defer freshRenderer.Stop()
		freshUpdateChan := freshRenderer.GetUpdateChannel()

		// Create new data with only site name changed
		newData := &PageData{
			Counter:    baseData.Counter, // Keep original counter data
			Site:       &Site{Name: "Updated Test Site"},
			Navigation: baseData.Navigation,
		}

		t.Logf("About to send site update - only Site.Name should change from 'Test Site' to 'Updated Test Site'")
		freshRenderer.SendUpdate(newData)

		// Wait for update
		select {
		case update := <-freshUpdateChan:
			t.Logf("Site update HTML: %q", update.HTML)
			if !strings.Contains(update.HTML, "Updated Test Site") {
				t.Error("Updated HTML should contain new site name")
			}
			t.Logf("Site name update - Fragment ID: %s", update.FragmentID)

		case <-time.After(3 * time.Second):
			t.Error("Timeout waiting for site name update")
		}
	})

	// Test 3: No update for identical data
	t.Run("NoChangeUpdate", func(t *testing.T) {
		// Create a fresh renderer for this test
		freshRenderer := statetemplate.NewRenderer(
			statetemplate.WithWrapperTag("div"),
			statetemplate.WithIDPrefix("fragment-"),
			statetemplate.WithPreserveBlocks(true),
		)

		// Use the same template content
		templateContent := `<div>
	Current Count: {{.Counter.Value}}
	Last updated: {{.Counter.LastUpdated}}
	Total updates: {{.Counter.UpdateCount}}

	{{block "header" .}}
		<h1>{{.Site.Name}}</h1>
		<nav>
			{{range .Navigation.MainItems}}
				<a href="{{.URL}}">{{.Label}}</a>
			{{end}}
		</nav>
	{{end}}
</div>`

		if err := freshRenderer.Parse("main", templateContent); err != nil {
			t.Fatalf("Failed to add template: %v", err)
		}

		// Set initial data
		_, err := freshRenderer.SetInitialData(baseData)
		if err != nil {
			t.Fatalf("Failed to set initial data: %v", err)
		}

		// Start the renderer and get update channel
		freshRenderer.Start()
		defer freshRenderer.Stop()
		freshUpdateChan := freshRenderer.GetUpdateChannel()

		// Send identical data - make a complete copy of the original base data
		identicalData := &PageData{
			Counter: &Counter{
				Value:       baseData.Counter.Value,
				LastUpdated: baseData.Counter.LastUpdated,
				UpdateCount: baseData.Counter.UpdateCount,
			},
			Site: &Site{
				Name: baseData.Site.Name,
			},
			Navigation: &Navigation{
				MainItems: make([]NavigationItem, len(baseData.Navigation.MainItems)),
			},
		}
		// Copy navigation items
		copy(identicalData.Navigation.MainItems, baseData.Navigation.MainItems)

		freshRenderer.SendUpdate(identicalData)

		// Should not receive any updates
		select {
		case update := <-freshUpdateChan:
			t.Errorf("Unexpected update received for identical data: %+v", update)

		case <-time.After(1 * time.Second):
			// Expected - no update should be sent
			t.Log("No update received for identical data (expected)")
		}
	})
}

// TestRealtimeRendererWebsocketCompatibility tests JSON serialization for websocket use
func TestRealtimeRendererWebsocketCompatibility(t *testing.T) {
	renderer := statetemplate.NewRenderer()

	// Simple template
	templateContent := `<div>Message: {{.Message}}</div>`
	err := renderer.Parse("simple", templateContent)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test data
	type TestData struct {
		Message string
	}

	initialData := &TestData{Message: "Hello"}
	_, err = renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	renderer.Start()
	defer renderer.Stop()

	updateChan := renderer.GetUpdateChannel()

	// Send update
	newData := &TestData{Message: "Updated"}
	renderer.SendUpdate(newData)

	// Wait for update
	select {
	case update := <-updateChan:
		// Verify update structure is suitable for JSON serialization
		if update.FragmentID == "" {
			t.Error("FragmentID should not be empty")
		}
		if update.HTML == "" {
			t.Error("HTML should not be empty")
		}
		if update.Action == "" {
			t.Error("Action should not be empty")
		}

		// Verify JSON structure (fields are exported)
		t.Logf("Update suitable for WebSocket: FragmentID=%s, Action=%s, HTML length=%d",
			update.FragmentID, update.Action, len(update.HTML))

	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for websocket compatibility test update")
	}
}
