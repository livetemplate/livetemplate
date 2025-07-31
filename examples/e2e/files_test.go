package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// TestFilesExample tests the file-based template loading functionality
// This mirrors the examples/files/main.go example
func TestFilesExample(t *testing.T) {
	// Get the templates directory path
	templatesDir := getTemplatesDir(t)

	// Test 1: Directory loading
	tracker := statetemplate.NewTemplateTracker()

	err := tracker.AddTemplatesFromDirectory(templatesDir, ".html")
	if err != nil {
		t.Fatalf("Failed to load templates from directory: %v", err)
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	expectedTemplates := []string{"header", "sidebar", "footer", "user-profile", "dashboard"}

	if len(templates) < len(expectedTemplates) {
		t.Errorf("Expected at least %d templates, got %d", len(expectedTemplates), len(templates))
	}

	for _, expectedTemplate := range expectedTemplates {
		if _, exists := templates[expectedTemplate]; !exists {
			t.Errorf("Expected template %s not found in: %v", expectedTemplate, getKeys(templates))
		}
	}

	// Test 2: Specific file loading
	tracker2 := statetemplate.NewTemplateTracker()

	fileMap := map[string]string{
		"my-header":  filepath.Join(templatesDir, "header.html"),
		"my-sidebar": filepath.Join(templatesDir, "sidebar.html"),
		"my-footer":  filepath.Join(templatesDir, "footer.html"),
	}

	err = tracker2.AddTemplatesFromFiles(fileMap)
	if err != nil {
		t.Fatalf("Failed to load templates from files: %v", err)
	}

	templates2 := tracker2.GetTemplates()
	expectedMappedTemplates := []string{"my-header", "my-sidebar", "my-footer"}

	if len(templates2) != len(expectedMappedTemplates) {
		t.Errorf("Expected %d mapped templates, got %d", len(expectedMappedTemplates), len(templates2))
	}

	for _, expectedTemplate := range expectedMappedTemplates {
		if _, exists := templates2[expectedTemplate]; !exists {
			t.Errorf("Expected mapped template %s not found", expectedTemplate)
		}
	}

	// Test 3: Dependency analysis
	deps := tracker.GetDependencies()

	// Check that header template has expected dependencies
	headerDeps := deps["header"]
	expectedHeaderDeps := []string{"Title", "CurrentUser.Name", "CurrentUser.Role"}
	for _, expectedDep := range expectedHeaderDeps {
		if !headerDeps[expectedDep] {
			t.Errorf("Expected header template to depend on %s. Available deps: %v", expectedDep, getKeys(headerDeps))
		}
	}

	// Check that sidebar template has expected dependencies
	sidebarDeps := deps["sidebar"]
	expectedSidebarDeps := []string{"CurrentUser.Name", "CurrentUser.Email", "Stats.UserCount"}
	for _, expectedDep := range expectedSidebarDeps {
		if !sidebarDeps[expectedDep] {
			t.Errorf("Expected sidebar template to depend on %s. Available deps: %v", expectedDep, getKeys(sidebarDeps))
		}
	}

	// Test 4: Live updates
	testLiveUpdates(t, tracker)
}

// TestFilesExamplePathDetection tests the path detection functionality
func TestFilesExamplePathDetection(t *testing.T) {
	// Test that we can find templates directory
	templatesDir := getTemplatesDir(t)

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		t.Fatalf("Templates directory not found at %s", templatesDir)
	}

	// Test that required template files exist
	requiredFiles := []string{"header.html", "sidebar.html", "footer.html", "user-profile.html", "dashboard.html"}

	for _, file := range requiredFiles {
		filePath := filepath.Join(templatesDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Required template file not found: %s", filePath)
		}
	}
}

// testLiveUpdates tests the live update functionality
func testLiveUpdates(t *testing.T, tracker *statetemplate.TemplateTracker) {
	// Test data structures (same as files example)
	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	type Stats struct {
		UserCount  int    `json:"user_count"`
		PostCount  int    `json:"post_count"`
		LastUpdate string `json:"last_update"`
	}

	type AppData struct {
		Title       string `json:"title"`
		CurrentUser *User  `json:"current_user"`
		Stats       *Stats `json:"stats"`
	}

	// Set up live updates
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Test initial data
	initialData := &AppData{
		Title: "Files Example Test",
		CurrentUser: &User{
			ID:    1,
			Name:  "Test User",
			Email: "test@example.com",
			Role:  "Admin",
		},
		Stats: &Stats{
			UserCount:  150,
			PostCount:  75,
			LastUpdate: "2025-01-01",
		},
	}

	dataChannel <- statetemplate.DataUpdate{Data: initialData}

	// Wait for initial update
	select {
	case update := <-updateChannel:
		if len(update.TemplateNames) == 0 {
			t.Error("Expected some templates to need re-render on initial data")
		}
		if len(update.ChangedFields) == 0 {
			t.Error("Expected some changed fields on initial data")
		}

	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for initial update")
	}

	// Test targeted update (change only user name)
	updatedData := *initialData
	updatedData.CurrentUser = &User{
		ID:    1,
		Name:  "Updated User", // Changed
		Email: "test@example.com",
		Role:  "Admin",
	}

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData}

	// Wait for targeted update
	select {
	case update := <-updateChannel:
		// Should have changed fields related to CurrentUser.Name
		foundNameChange := false
		for _, field := range update.ChangedFields {
			if field == "CurrentUser.Name" {
				foundNameChange = true
				break
			}
		}
		if !foundNameChange {
			t.Errorf("Expected CurrentUser.Name in changed fields, got: %v", update.ChangedFields)
		}

	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for targeted update")
	}

	close(dataChannel)
}

// getTemplatesDir returns the correct path to the templates directory for testing
func getTemplatesDir(t *testing.T) string {
	// Try different possible paths
	possiblePaths := []string{
		"examples/files/templates",       // From project root
		"../files/templates",             // From examples/e2e
		"files/templates",                // From examples directory
		"../../examples/files/templates", // From deeper nested locations
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	t.Fatal("Could not find templates directory. Tried paths:", possiblePaths)
	return ""
}

// Helper function to get keys from a map
func getKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
