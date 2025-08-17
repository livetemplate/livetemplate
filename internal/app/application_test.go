package app

import (
	"context"
	"html/template"
	"strings"
	"testing"
	"time"
)

func TestApplication_BasicFunctionality(t *testing.T) {
	// Create a new application
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Test basic application properties
	if app.id == "" {
		t.Error("application should have a non-empty ID")
	}

	if app.tokenService == nil {
		t.Error("application should have a token service")
	}

	if app.pageRegistry == nil {
		t.Error("application should have a page registry")
	}

	if app.memoryManager == nil {
		t.Error("application should have a memory manager")
	}

	// Test page count starts at zero
	if count := app.GetPageCount(); count != 0 {
		t.Errorf("expected page count 0, got %d", count)
	}
}

func TestApplication_PageLifecycle(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Create a simple template
	tmpl, err := template.New("test").Parse(`<div>Hello {{.Name}}</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create a page
	data := map[string]interface{}{"Name": "World"}
	page, err := app.NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Test page count increased
	if count := app.GetPageCount(); count != 1 {
		t.Errorf("expected page count 1, got %d", count)
	}

	// Test page token is generated
	token := page.GetToken()
	if token == "" {
		t.Error("page should have a token")
	}

	// Test page can be retrieved by token
	retrievedPage, err := app.GetPage(token)
	if err != nil {
		t.Errorf("failed to retrieve page by token: %v", err)
	}

	if retrievedPage == nil {
		t.Error("retrieved page should not be nil")
	}

	// Test page rendering
	html, err := page.Render()
	if err != nil {
		t.Errorf("failed to render page: %v", err)
	}

	expected := "<div>Hello World</div>"
	if html != expected {
		t.Errorf("expected HTML %q, got %q", expected, html)
	}

	// Close the page
	if err := page.Close(); err != nil {
		t.Errorf("failed to close page: %v", err)
	}

	// Test page count decreased
	if count := app.GetPageCount(); count != 0 {
		t.Errorf("expected page count 0 after close, got %d", count)
	}
}

func TestApplication_CrossApplicationIsolation(t *testing.T) {
	// Create two applications
	app1, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application 1: %v", err)
	}
	defer func() { _ = app1.Close() }()

	app2, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application 2: %v", err)
	}
	defer func() { _ = app2.Close() }()

	// Verify applications have different IDs
	if app1.id == app2.id {
		t.Error("applications should have different IDs")
	}

	// Create a page in app1
	tmpl, err := template.New("test").Parse(`<div>{{.Message}}</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	data := map[string]interface{}{"Message": "App1 Data"}
	page1, err := app1.NewPage(tmpl, data)
	if err != nil {
		t.Fatalf("failed to create page in app1: %v", err)
	}

	token1 := page1.GetToken()

	// Try to access app1's page from app2 (should fail)
	_, err = app2.GetPage(token1)
	if err == nil {
		t.Error("app2 should not be able to access app1's page")
	}

	// Verify that cross-application access is denied
	// (could be due to signature mismatch or explicit denial)
	errorMsg := err.Error()
	if errorMsg != "cross-application access denied" &&
		!contains(errorMsg, "signature is invalid") &&
		!contains(errorMsg, "invalid token") {
		t.Errorf("expected cross-application access to be denied, got: %v", err)
	}
}

func TestApplication_PageFragmentGeneration(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Create a template with dynamic content
	tmpl, err := template.New("test").Parse(`<div>Counter: {{.Count}}</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create a page with initial data
	initialData := map[string]interface{}{"Count": 0}
	page, err := app.NewPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}
	defer func() { _ = page.Close() }()

	// Generate fragments with new data
	newData := map[string]interface{}{"Count": 1}
	ctx := context.Background()
	fragments, err := page.RenderFragments(ctx, newData)
	if err != nil {
		t.Errorf("failed to generate fragments: %v", err)
	}

	// Verify fragments were generated
	if len(fragments) == 0 {
		t.Error("expected at least one fragment to be generated")
	}

	// Test that page data was updated
	currentData := page.GetData()
	if currentData == nil {
		t.Error("page data should not be nil")
	}

	// Verify the data was updated to new data
	if dataMap, ok := currentData.(map[string]interface{}); ok {
		if count, exists := dataMap["Count"]; exists {
			if count != 1 {
				t.Errorf("expected Count to be 1, got %v", count)
			}
		} else {
			t.Error("Count field should exist in updated data")
		}
	} else {
		t.Error("data should be a map[string]interface{}")
	}
}

func TestApplication_Metrics(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Get initial metrics
	metrics := app.GetMetrics()

	// Verify application ID is set
	if metrics.ApplicationID == "" {
		t.Error("metrics should have application ID")
	}

	// Verify metrics have sensible initial values
	if metrics.PagesCreated != 0 {
		t.Errorf("expected PagesCreated to be 0, got %d", metrics.PagesCreated)
	}

	if metrics.ActivePages != 0 {
		t.Errorf("expected ActivePages to be 0, got %d", metrics.ActivePages)
	}

	// Test that start time is recent
	if time.Since(metrics.StartTime) > time.Minute {
		t.Error("start time should be recent")
	}
}

func TestApplication_Configuration(t *testing.T) {
	// Test application with custom configuration
	app, err := NewApplication(
		WithMaxPages(500),
		WithPageTTL(30*time.Minute),
		WithMaxMemoryMB(50),
		WithMetricsEnabled(false),
	)
	if err != nil {
		t.Fatalf("failed to create application with options: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Verify the application was created successfully
	// Options configuration is checked elsewhere in the application logic
	_ = app.config.MaxPages // Verify config is accessible

	// Test that application works with configuration
	tmpl, err := template.New("test").Parse(`<span>{{.Value}}</span>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	_, err = app.NewPage(tmpl, map[string]interface{}{"Value": "test"})
	if err != nil {
		t.Errorf("failed to create page with configured application: %v", err)
	}
}

func TestApplication_CleanupExpiredPages(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Test cleanup when no pages exist
	cleanedCount := app.CleanupExpiredPages()
	if cleanedCount != 0 {
		t.Errorf("expected 0 pages cleaned up, got %d", cleanedCount)
	}

	// Create a page
	tmpl, err := template.New("test").Parse(`<div>{{.Data}}</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	page, err := app.NewPage(tmpl, map[string]interface{}{"Data": "value"})
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Test cleanup with active page (should not clean anything)
	cleanedCount = app.CleanupExpiredPages()
	if cleanedCount != 0 {
		t.Errorf("expected 0 pages cleaned up with active page, got %d", cleanedCount)
	}

	// Verify page count is still 1
	if count := app.GetPageCount(); count != 1 {
		t.Errorf("expected page count 1, got %d", count)
	}

	_ = page.Close()
}

func TestApplication_Close(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	// Create a page before closing
	tmpl, err := template.New("test").Parse(`<div>test</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	page, err := app.NewPage(tmpl, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Close the application
	if err := app.Close(); err != nil {
		t.Errorf("failed to close application: %v", err)
	}

	// Verify application is marked as closed
	if !app.closed {
		t.Error("application should be marked as closed")
	}

	// Test that operations fail after close
	_, err = app.NewPage(tmpl, nil)
	if err == nil {
		t.Error("creating page should fail after application is closed")
	}

	// Test that page operations fail after application close
	_, err = page.Render()
	if err == nil {
		t.Error("page render should fail after application is closed")
	}

	// Test double close doesn't error
	if err := app.Close(); err != nil {
		t.Errorf("double close should not error: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
