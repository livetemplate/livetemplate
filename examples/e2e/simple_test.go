package e2e

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// TestSimpleExample tests the basic template tracking functionality
// This mirrors the examples/simple/main.go example
func TestSimpleExample(t *testing.T) {
	// Create template tracker
	tracker := statetemplate.NewTemplateTracker()

	// Define test templates (same as simple example)
	headerTemplate := `
	<header>
		<h1>{{.Title}}</h1>
		<div class="user-info">Welcome, {{.CurrentUser.Name}}!</div>
	</header>`

	sidebarTemplate := `
	<aside>
		<div class="user-count">Total Users: {{.UserCount}}</div>
		<div class="last-update">Last Update: {{.LastUpdate}}</div>
	</aside>`

	userProfileTemplate := `
	<div class="profile">
		<h2>{{.CurrentUser.Name}}</h2>
		<p>Email: {{.CurrentUser.Email}}</p>
		<p>ID: {{.CurrentUser.ID}}</p>
	</div>`

	// Create and add templates
	headerTmpl := template.Must(template.New("header").Parse(headerTemplate))
	tracker.AddTemplate("header", headerTmpl)

	sidebarTmpl := template.Must(template.New("sidebar").Parse(sidebarTemplate))
	tracker.AddTemplate("sidebar", sidebarTmpl)

	userProfileTmpl := template.Must(template.New("user-profile").Parse(userProfileTemplate))
	tracker.AddTemplate("user-profile", userProfileTmpl)

	// Verify templates were added
	templates := tracker.GetTemplates()
	expectedTemplates := []string{"header", "sidebar", "user-profile"}

	if len(templates) != len(expectedTemplates) {
		t.Errorf("Expected %d templates, got %d", len(expectedTemplates), len(templates))
	}

	for _, expectedTemplate := range expectedTemplates {
		if _, exists := templates[expectedTemplate]; !exists {
			t.Errorf("Expected template %s not found", expectedTemplate)
		}
	}

	// Test dependency tracking
	deps := tracker.GetDependencies()

	// Verify header dependencies
	headerDeps := deps["header"]
	expectedHeaderDeps := []string{"Title", "CurrentUser.Name"}
	for _, expectedDep := range expectedHeaderDeps {
		if !headerDeps[expectedDep] {
			t.Errorf("Expected header template to depend on %s", expectedDep)
		}
	}

	// Verify sidebar dependencies
	sidebarDeps := deps["sidebar"]
	expectedSidebarDeps := []string{"UserCount", "LastUpdate"}
	for _, expectedDep := range expectedSidebarDeps {
		if !sidebarDeps[expectedDep] {
			t.Errorf("Expected sidebar template to depend on %s", expectedDep)
		}
	}

	// Test live updates
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Test data structure (same as simple example)
	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	type AppData struct {
		Title       string `json:"title"`
		CurrentUser *User  `json:"current_user"`
		UserCount   int    `json:"user_count"`
		LastUpdate  string `json:"last_update"`
	}

	// Send initial data
	initialData := &AppData{
		Title: "Simple Example Test",
		CurrentUser: &User{
			ID:    1,
			Name:  "Test User",
			Email: "test@example.com",
		},
		UserCount:  100,
		LastUpdate: "2025-01-01",
	}

	dataChannel <- statetemplate.DataUpdate{Data: initialData}

	// Wait for update
	select {
	case update := <-updateChannel:
		// Verify all templates need re-render on initial data
		expectedTemplates := map[string]bool{
			"header":       true,
			"sidebar":      true,
			"user-profile": true,
		}

		for _, templateName := range update.TemplateNames {
			if !expectedTemplates[templateName] {
				t.Errorf("Unexpected template %s in update", templateName)
			}
			delete(expectedTemplates, templateName)
		}

		if len(expectedTemplates) > 0 {
			t.Errorf("Missing templates in update: %v", expectedTemplates)
		}

	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for initial update")
	}

	// Test targeted update (change only user name)
	updatedData := *initialData
	updatedData.CurrentUser = &User{
		ID:    1,
		Name:  "Updated User", // Changed
		Email: "test@example.com",
	}

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData}

	// Wait for targeted update
	select {
	case update := <-updateChannel:
		// Should only affect templates that depend on CurrentUser.Name
		expectedTemplates := []string{"header", "user-profile"}

		if len(update.TemplateNames) != len(expectedTemplates) {
			t.Errorf("Expected %d templates in update, got %d: %v",
				len(expectedTemplates), len(update.TemplateNames), update.TemplateNames)
		}

		templateMap := make(map[string]bool)
		for _, name := range update.TemplateNames {
			templateMap[name] = true
		}

		for _, expectedTemplate := range expectedTemplates {
			if !templateMap[expectedTemplate] {
				t.Errorf("Expected template %s in update", expectedTemplate)
			}
		}

		// Verify changed fields
		expectedChangedFields := []string{"CurrentUser.Name"}
		if len(update.ChangedFields) != len(expectedChangedFields) {
			t.Errorf("Expected %d changed fields, got %d: %v",
				len(expectedChangedFields), len(update.ChangedFields), update.ChangedFields)
		}

	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for targeted update")
	}

	close(dataChannel)
}

// TestSimpleExampleTemplateRendering tests that templates can actually be rendered
func TestSimpleExampleTemplateRendering(t *testing.T) {
	tracker := statetemplate.NewTemplateTracker()

	templateContent := `<h1>{{.Title}}</h1><p>User: {{.User.Name}}</p>`

	testTemplate := template.Must(template.New("test").Parse(templateContent))
	tracker.AddTemplate("test", testTemplate)

	templates := tracker.GetTemplates()
	testTemplate, exists := templates["test"]
	if !exists {
		t.Fatal("Test template not found")
	}

	// Test data
	type User struct {
		Name string `json:"name"`
	}
	type Data struct {
		Title string `json:"title"`
		User  *User  `json:"user"`
	}

	testData := &Data{
		Title: "Test Title",
		User:  &User{Name: "Test User"},
	}

	// Render template
	var output strings.Builder
	err := testTemplate.Execute(&output, testData)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := output.String()
	expectedTitle := "Test Title"
	expectedUser := "Test User"

	if !strings.Contains(result, expectedTitle) {
		t.Errorf("Expected output to contain %q, got: %s", expectedTitle, result)
	}

	if !strings.Contains(result, expectedUser) {
		t.Errorf("Expected output to contain %q, got: %s", expectedUser, result)
	}
}
