package e2e

import (
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// TestFragmentsExample tests the automatic fragment extraction functionality
// This mirrors the examples/fragments/main.go example
func TestFragmentsExample(t *testing.T) {
	tracker := statetemplate.NewTemplateTracker()

	// Complex template with multiple data expressions (same as fragments example)
	templateContent := `
<div class="dashboard">
    <header class="header">
        <h1>{{.Title}}</h1>
        <p class="welcome">Welcome back, {{.User.Name}}!</p>
    </header>

    <div class="main-content">
        <div class="counter-section">
            <div class="counter-display">
                Current Count: {{.Counter.Value}}
            </div>
            <div class="counter-meta">
                Last updated: {{.Counter.LastUpdated}}
            </div>
            <div class="update-count">
                Total updates: {{.Counter.UpdateCount}}
            </div>
        </div>

        <div class="user-section">
            <div class="user-info">
                User: {{.User.Name}} ({{.User.Email}})
            </div>
            <div class="user-id">
                ID: {{.User.ID}}
            </div>
        </div>

        <div class="stats-section">
            <div class="clicks">
                Total clicks: {{.Stats.TotalClicks}}
            </div>
            <div class="visits">
                Unique visits: {{.Stats.UniqueVisits}}
            </div>
            <div class="session">
                Session length: {{.Stats.SessionLength}} minutes
            </div>
        </div>

        <div class="message-section">
            <div class="status-message">
                Status: {{.Message}}
            </div>
        </div>
    </div>

    <div class="static-content">
        <button id="increment">+</button>
        <button id="decrement">-</button>
        <p>These buttons don't contain template expressions, so they won't be extracted as fragments.</p>
    </div>
</div>`

	// Test fragment extraction
	tmpl, fragments, err := tracker.AddTemplateWithFragmentExtraction("dashboard", templateContent)
	if err != nil {
		t.Fatalf("Failed to process template with fragment extraction: %v", err)
	}

	// Verify template was created
	if tmpl == nil {
		t.Fatal("Expected template to be created")
	}

	// Verify fragments were extracted
	if len(fragments) == 0 {
		t.Fatal("Expected fragments to be extracted")
	}

	t.Logf("Extracted %d fragments", len(fragments))

	// Verify that fragments have the expected properties
	for i, fragment := range fragments {
		if fragment.ID == "" {
			t.Errorf("Fragment %d has empty ID", i)
		}
		if fragment.Content == "" {
			t.Errorf("Fragment %d has empty content", i)
		}
		if len(fragment.Dependencies) == 0 {
			t.Errorf("Fragment %d has no dependencies", i)
		}
		if fragment.StartPos < 0 || fragment.EndPos < 0 || fragment.StartPos >= fragment.EndPos {
			t.Errorf("Fragment %d has invalid position: %d-%d", i, fragment.StartPos, fragment.EndPos)
		}
	}

	// Test that fragments have expected dependencies
	expectedDependencies := map[string]bool{
		"Title":               true,
		"User.Name":           true,
		"User.Email":          true,
		"User.ID":             true,
		"Counter.Value":       true,
		"Counter.LastUpdated": true,
		"Counter.UpdateCount": true,
		"Stats.TotalClicks":   true,
		"Stats.UniqueVisits":  true,
		"Stats.SessionLength": true,
		"Message":             true,
	}

	foundDependencies := make(map[string]bool)
	for _, fragment := range fragments {
		for _, dep := range fragment.Dependencies {
			foundDependencies[dep] = true
		}
	}

	for expectedDep := range expectedDependencies {
		if !foundDependencies[expectedDep] {
			t.Errorf("Expected dependency %s not found in fragments", expectedDep)
		}
	}

	// Test live updates with fragment tracking
	testFragmentLiveUpdates(t, tracker, fragments)
}

// TestFragmentsExampleGranularUpdates tests that fragments provide granular update notifications
func TestFragmentsExampleGranularUpdates(t *testing.T) {
	tracker := statetemplate.NewTemplateTracker()

	// Simple template with multiple sections
	templateContent := `
<div>
    <h1>{{.Title}}</h1>
    <p>User: {{.User.Name}}</p>
    <p>Count: {{.Stats.Count}}</p>
</div>`

	_, fragments, err := tracker.AddTemplateWithFragmentExtraction("test", templateContent)
	if err != nil {
		t.Fatalf("Failed to extract fragments: %v", err)
	}

	if len(fragments) < 3 {
		t.Fatalf("Expected at least 3 fragments (Title, User.Name, Stats.Count), got %d", len(fragments))
	}

	// Test data structures
	type User struct {
		Name string `json:"name"`
	}
	type Stats struct {
		Count int `json:"count"`
	}
	type Data struct {
		Title string `json:"title"`
		User  *User  `json:"user"`
		Stats *Stats `json:"stats"`
	}

	// Set up live updates
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Send initial data
	initialData := &Data{
		Title: "Fragment Test",
		User:  &User{Name: "Test User"},
		Stats: &Stats{Count: 10},
	}

	dataChannel <- statetemplate.DataUpdate{Data: initialData}

	// Wait for initial update
	select {
	case <-updateChannel:
		// Initial update should include main template and all fragments
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for initial update")
	}

	// Test targeted update - change only user name
	updatedData := *initialData
	updatedData.User = &User{Name: "Updated User"}

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData}

	// Wait for targeted update
	select {
	case update := <-updateChannel:
		// Should only affect fragments/templates that depend on User.Name
		foundUserNameChange := false
		for _, field := range update.ChangedFields {
			if field == "User.Name" {
				foundUserNameChange = true
				break
			}
		}
		if !foundUserNameChange {
			t.Errorf("Expected User.Name in changed fields, got: %v", update.ChangedFields)
		}

	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for targeted update")
	}

	close(dataChannel)
}

// testFragmentLiveUpdates tests live updates with fragment tracking
func testFragmentLiveUpdates(t *testing.T, tracker *statetemplate.TemplateTracker, fragments []*statetemplate.TemplateFragment) {
	// Test data structures (same as fragments example)
	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	type Counter struct {
		Value       int    `json:"value"`
		LastUpdated string `json:"last_updated"`
		UpdateCount int    `json:"update_count"`
	}

	type Stats struct {
		TotalClicks   int `json:"total_clicks"`
		UniqueVisits  int `json:"unique_visits"`
		SessionLength int `json:"session_length"`
	}

	type AppData struct {
		Title   string   `json:"title"`
		User    *User    `json:"user"`
		Counter *Counter `json:"counter"`
		Stats   *Stats   `json:"stats"`
		Message string   `json:"message"`
	}

	// Set up live updates
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Test initial data
	testData := &AppData{
		Title: "Fragment Demo Dashboard",
		User: &User{
			ID:    42,
			Name:  "John Developer",
			Email: "john@example.com",
		},
		Counter: &Counter{
			Value:       15,
			LastUpdated: "2 seconds ago",
			UpdateCount: 23,
		},
		Stats: &Stats{
			TotalClicks:   1247,
			UniqueVisits:  89,
			SessionLength: 15,
		},
		Message: "All systems operational",
	}

	dataChannel <- statetemplate.DataUpdate{Data: testData}

	// Wait for initial update
	select {
	case update := <-updateChannel:
		if len(update.TemplateNames) == 0 {
			t.Error("Expected some templates to need re-render on initial data")
		}

	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for initial update")
	}

	// Test counter-only update
	newData := *testData
	newData.Counter = &Counter{
		Value:       16,         // Changed
		LastUpdated: "just now", // Changed
		UpdateCount: 24,         // Changed
	}

	dataChannel <- statetemplate.DataUpdate{Data: &newData}

	// Wait for counter update
	select {
	case update := <-updateChannel:
		// Should have counter-related changed fields
		hasCounterChange := false
		for _, field := range update.ChangedFields {
			if field == "Counter.Value" || field == "Counter.LastUpdated" || field == "Counter.UpdateCount" {
				hasCounterChange = true
				break
			}
		}
		if !hasCounterChange {
			t.Errorf("Expected counter-related changes, got: %v", update.ChangedFields)
		}

	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for counter update")
	}

	close(dataChannel)
}
