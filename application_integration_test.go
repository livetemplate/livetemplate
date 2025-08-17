package livetemplate

import (
	"context"
	"html/template"
	"testing"
)

func TestApplication_Integration(t *testing.T) {
	// Create an Application instance
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Create a template
	tmpl, err := template.New("user-dashboard").Parse(`
		<div class="dashboard">
			<h1>Welcome, {{.User.Name}}!</h1>
			<p>You have {{.User.MessageCount}} new messages.</p>
			<div class="status">Status: {{.User.Status}}</div>
		</div>
	`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Initial user data
	initialData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name":         "Alice",
			"MessageCount": 3,
			"Status":       "online",
		},
	}

	// Create a page with initial data
	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}
	defer func() { _ = page.Close() }()

	// Test initial render
	initialHTML, err := page.Render()
	if err != nil {
		t.Fatalf("failed to render initial page: %v", err)
	}

	if !contains(initialHTML, "Welcome, Alice!") {
		t.Error("initial render should contain user name")
	}

	if !contains(initialHTML, "3 new messages") {
		t.Error("initial render should contain message count")
	}

	// Simulate user receiving a new message
	updatedData := map[string]interface{}{
		"User": map[string]interface{}{
			"Name":         "Alice",
			"MessageCount": 4, // New message received
			"Status":       "online",
		},
	}

	// Generate fragment updates
	ctx := context.Background()
	fragments, err := page.RenderFragments(ctx, updatedData)
	if err != nil {
		t.Fatalf("failed to generate fragments: %v", err)
	}

	// Verify fragments were generated
	if len(fragments) == 0 {
		t.Error("expected fragments to be generated for message count change")
	}

	// Test that page state was updated
	currentData := page.GetData()
	if userMap, ok := currentData.(map[string]interface{}); ok {
		if user, exists := userMap["User"]; exists {
			if userData, ok := user.(map[string]interface{}); ok {
				if count, exists := userData["MessageCount"]; exists {
					if count != 4 {
						t.Errorf("expected message count to be 4, got %v", count)
					}
				}
			}
		}
	}

	// Test JWT token functionality
	token := page.GetToken()
	if token == "" {
		t.Error("page should have a token")
	}

	// Retrieve page by token
	retrievedPage, err := app.GetApplicationPage(token)
	if err != nil {
		t.Fatalf("failed to retrieve page by token: %v", err)
	}

	// Verify retrieved page has same data
	retrievedData := retrievedPage.GetData()
	if retrievedData == nil {
		t.Error("retrieved page should have data")
	}

	// Test application metrics
	metrics := app.GetApplicationMetrics()
	if metrics.PagesCreated != 1 {
		t.Errorf("expected 1 page created, got %d", metrics.PagesCreated)
	}

	if metrics.ActivePages != 1 {
		t.Errorf("expected 1 active page, got %d", metrics.ActivePages)
	}

	if metrics.TokensGenerated < 1 {
		t.Errorf("expected at least 1 token generated, got %d", metrics.TokensGenerated)
	}

	// Test page metrics
	pageMetrics := page.GetApplicationPageMetrics()
	if pageMetrics.PageID == "" {
		t.Error("page metrics should have page ID")
	}

	if pageMetrics.TotalGenerations < 1 {
		t.Errorf("expected at least 1 fragment generation, got %d", pageMetrics.TotalGenerations)
	}
}

func TestApplication_MultiTenantIsolation(t *testing.T) {
	// Create two separate applications (simulating different tenants)
	ecommerceApp, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create e-commerce application: %v", err)
	}
	defer func() { _ = ecommerceApp.Close() }()

	analyticsApp, err := NewApplication()
	if err != nil {
		t.Fatalf("failed to create analytics application: %v", err)
	}
	defer func() { _ = analyticsApp.Close() }()

	// Create templates for each application
	ecommerceTmpl, err := template.New("product-page").Parse(`
		<div class="product">
			<h1>{{.Product.Name}}</h1>
			<p>Price: ${{.Product.Price}}</p>
			<p>In Stock: {{.Product.Stock}}</p>
		</div>
	`)
	if err != nil {
		t.Fatalf("failed to create e-commerce template: %v", err)
	}

	analyticsTmpl, err := template.New("dashboard").Parse(`
		<div class="analytics">
			<h2>Site Analytics</h2>
			<p>Visitors: {{.Stats.Visitors}}</p>
			<p>Page Views: {{.Stats.PageViews}}</p>
		</div>
	`)
	if err != nil {
		t.Fatalf("failed to create analytics template: %v", err)
	}

	// Create pages in each application
	ecommerceData := map[string]interface{}{
		"Product": map[string]interface{}{
			"Name":  "Laptop",
			"Price": 999,
			"Stock": 5,
		},
	}

	analyticsData := map[string]interface{}{
		"Stats": map[string]interface{}{
			"Visitors":  1250,
			"PageViews": 4800,
		},
	}

	ecommercePage, err := ecommerceApp.NewApplicationPage(ecommerceTmpl, ecommerceData)
	if err != nil {
		t.Fatalf("failed to create e-commerce page: %v", err)
	}
	defer func() { _ = ecommercePage.Close() }()

	analyticsPage, err := analyticsApp.NewApplicationPage(analyticsTmpl, analyticsData)
	if err != nil {
		t.Fatalf("failed to create analytics page: %v", err)
	}
	defer func() { _ = analyticsPage.Close() }()

	// Test that applications are isolated
	ecommerceToken := ecommercePage.GetToken()
	analyticsToken := analyticsPage.GetToken()

	// Verify tokens are different
	if ecommerceToken == analyticsToken {
		t.Error("different applications should generate different tokens")
	}

	// Test cross-application access is denied
	_, err = analyticsApp.GetApplicationPage(ecommerceToken)
	if err == nil {
		t.Error("analytics app should not be able to access e-commerce page")
	}

	_, err = ecommerceApp.GetApplicationPage(analyticsToken)
	if err == nil {
		t.Error("e-commerce app should not be able to access analytics page")
	}

	// Test that pages work correctly within their own applications
	ecommerceHTML, err := ecommercePage.Render()
	if err != nil {
		t.Fatalf("failed to render e-commerce page: %v", err)
	}

	if !contains(ecommerceHTML, "Laptop") || !contains(ecommerceHTML, "$999") {
		t.Error("e-commerce page should render product information")
	}

	analyticsHTML, err := analyticsPage.Render()
	if err != nil {
		t.Fatalf("failed to render analytics page: %v", err)
	}

	if !contains(analyticsHTML, "1250") || !contains(analyticsHTML, "4800") {
		t.Error("analytics page should render statistics")
	}

	// Verify separate metrics
	ecommerceMetrics := ecommerceApp.GetApplicationMetrics()
	analyticsMetrics := analyticsApp.GetApplicationMetrics()

	if ecommerceMetrics.ApplicationID == analyticsMetrics.ApplicationID {
		t.Error("applications should have different IDs")
	}

	if ecommerceMetrics.ActivePages != 1 || analyticsMetrics.ActivePages != 1 {
		t.Error("each application should have exactly 1 active page")
	}
}

func TestApplication_MemoryManagement(t *testing.T) {
	// Create application with memory constraints
	app, err := NewApplication(WithMaxMemoryMB(1)) // Very small limit for testing
	if err != nil {
		t.Fatalf("failed to create memory-constrained application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Create a template
	tmpl, err := template.New("test").Parse(`<div>{{.Data}}</div>`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create a page - should succeed
	page1, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Data": "test1"})
	if err != nil {
		t.Fatalf("failed to create first page: %v", err)
	}
	defer func() { _ = page1.Close() }()

	// Test that memory tracking is working
	metrics := app.GetApplicationMetrics()
	if metrics.MemoryUsage == 0 {
		t.Error("memory usage should be tracked")
	}

	// The memory limit is set very low, but the exact behavior depends on the memory estimation
	// For now, just verify that the application handles memory tracking
	if metrics.MemoryStatus == "" {
		t.Error("memory status should be reported")
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
