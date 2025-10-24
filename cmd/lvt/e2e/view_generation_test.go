package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestViewGen_Basic tests generating a basic view-only handler
func TestViewGen_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate view
	t.Log("Generating dashboard view...")
	if err := runLvtCommand(t, appDir, "gen", "view", "dashboard"); err != nil {
		t.Fatalf("Failed to generate dashboard view: %v", err)
	}

	// Verify files created
	expectedFiles := []string{
		"internal/app/dashboard/dashboard.go",
		"internal/app/dashboard/dashboard.tmpl",
		"internal/app/dashboard/dashboard_test.go",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", file)
		}
	}

	// Verify handler does NOT import database packages
	handlerPath := filepath.Join(appDir, "internal/app/dashboard/dashboard.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	handlerContent := string(handler)
	if strings.Contains(handlerContent, "database") || strings.Contains(handlerContent, "models") {
		t.Error("View-only handler should not import database packages")
	}

	// Verify handler has Handler() function (not Handler(queries))
	if !strings.Contains(handlerContent, "func Handler()") {
		t.Error("View handler should have Handler() function without database parameter")
	}

	// Verify template file exists and is not empty
	tmplPath := filepath.Join(appDir, "internal/app/dashboard/dashboard.tmpl")
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	if len(tmpl) == 0 {
		t.Error("Template file should not be empty")
	}

	// Verify database files were NOT updated
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schemaContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	if strings.Contains(string(schemaContent), "dashboard") {
		t.Error("View generation should not modify schema.sql")
	}

	t.Log("✅ Basic view generation test passed")
}

// TestViewGen_Interactive tests generating an interactive view with dynamic content
func TestViewGen_Interactive(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate interactive view (like counter)
	t.Log("Generating counter view...")
	if err := runLvtCommand(t, appDir, "gen", "view", "counter"); err != nil {
		t.Fatalf("Failed to generate counter view: %v", err)
	}

	// Verify files created
	handlerPath := filepath.Join(appDir, "internal/app/counter/counter.go")
	if _, err := os.Stat(handlerPath); os.IsNotExist(err) {
		t.Fatal("Handler file not created")
	}

	tmplPath := filepath.Join(appDir, "internal/app/counter/counter.tmpl")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		t.Fatal("Template file not created")
	}

	// Verify handler can support LiveTemplate events
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	handlerContent := string(handler)
	// Handler should import livetemplate package
	if !strings.Contains(handlerContent, "livetemplate") {
		t.Error("Handler should import livetemplate package")
	}

	// Handler should have Handler() function
	if !strings.Contains(handlerContent, "func Handler()") {
		t.Error("Handler should have Handler() function")
	}

	t.Log("✅ Interactive view generation test passed")
}

// TestViewGen_MultipleViews tests generating multiple views in one app
func TestViewGen_MultipleViews(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate multiple views
	views := []string{"home", "about", "contact"}
	for _, view := range views {
		t.Logf("Generating %s view...", view)
		if err := runLvtCommand(t, appDir, "gen", "view", view); err != nil {
			t.Fatalf("Failed to generate %s view: %v", view, err)
		}
	}

	// Verify all views were created
	for _, view := range views {
		handlerPath := filepath.Join(appDir, "internal/app", view, view+".go")
		if _, err := os.Stat(handlerPath); os.IsNotExist(err) {
			t.Errorf("Handler for %s not created", view)
		}

		tmplPath := filepath.Join(appDir, "internal/app", view, view+".tmpl")
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			t.Errorf("Template for %s not created", view)
		}

		testPath := filepath.Join(appDir, "internal/app", view, view+"_test.go")
		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			t.Errorf("Test file for %s not created", view)
		}
	}

	// Verify main.go has routes for all views
	mainPath := filepath.Join(appDir, "cmd/testapp/main.go")
	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("Failed to read main.go: %v", err)
	}

	mainStr := string(mainContent)
	for _, view := range views {
		routePattern := `http.Handle("/` + view + `"`
		if !strings.Contains(mainStr, routePattern) {
			t.Errorf("Route for %s not found in main.go", view)
		}
	}

	// Verify database files were NOT updated for any view
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schemaContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	schemaStr := string(schemaContent)
	for _, view := range views {
		if strings.Contains(schemaStr, view) {
			t.Errorf("View %s should not appear in schema.sql", view)
		}
	}

	t.Log("✅ Multiple views generation test passed")
}
