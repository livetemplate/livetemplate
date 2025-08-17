package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/livefir/livetemplate/internal/memory"
)

func TestApplication_MemoryManagementIntegration(t *testing.T) {
	// Create application with constrained memory for testing
	app, err := NewApplication(
		WithMaxMemoryMB(10), // 10MB limit for integration testing
	)
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Set up memory pressure callbacks
	var memoryWarnings, memoryCritical int
	var mu sync.Mutex

	callbacks := &memory.PressureCallbacks{
		OnWarning: func(status memory.Status) {
			mu.Lock()
			memoryWarnings++
			mu.Unlock()
			t.Logf("Memory warning: %.1f%% usage", status.UsagePercentage)
		},
		OnCritical: func(status memory.Status) {
			mu.Lock()
			memoryCritical++
			mu.Unlock()
			t.Logf("Memory critical: %.1f%% usage", status.UsagePercentage)

			// Trigger cleanup when memory is critical
			cleanedCount := app.CleanupExpiredPages()
			t.Logf("Cleaned up %d expired pages due to memory pressure", cleanedCount)
		},
	}

	// Note: We would need to expose the memory manager to set callbacks
	// For now, this demonstrates the integration concept
	_ = callbacks

	// Create template for testing
	tmpl, err := template.New("memory-test").Parse(`
		<div class="user-dashboard">
			<h1>User: {{.User.Name}}</h1>
			<div class="stats">
				<p>Messages: {{.User.Messages}}</p>
				<p>Status: {{.User.Status}}</p>
				<p>Last Login: {{.User.LastLogin}}</p>
				<div class="data">{{.User.Data}}</div>
			</div>
		</div>
	`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Generate realistic data that will consume memory
	generateUserData := func(userID int, dataSize int) map[string]interface{} {
		// Create safe text data instead of binary to avoid regex issues
		dataChunks := make([]string, 0, dataSize/100)
		for i := 0; i < dataSize/100; i++ {
			dataChunks = append(dataChunks, fmt.Sprintf("Data chunk %d for user %d - this is sample text data", i, userID))
		}

		return map[string]interface{}{
			"User": map[string]interface{}{
				"Name":      generateUserName(userID),
				"Messages":  userID * 10,
				"Status":    "online",
				"LastLogin": "2024-01-01T10:00:00Z",
				"Data":      fmt.Sprintf("User %d data: %v", userID, dataChunks),
			},
		}
	}

	// Test memory usage with multiple pages
	const numPages = 50
	const dataSize = 50 * 1024 // 50KB per page

	var pages []*ApplicationPage
	var tokens []string

	// Create pages and monitor memory usage
	for i := 0; i < numPages; i++ {
		userData := generateUserData(i, dataSize)

		page, err := app.NewApplicationPage(tmpl, userData)
		if err != nil {
			// Expected behavior when running out of memory
			t.Logf("Failed to create page %d (expected under memory pressure): %v", i, err)
			break
		}

		pages = append(pages, page)
		tokens = append(tokens, page.GetToken())

		// Test initial render
		html, err := page.Render()
		if err != nil {
			t.Errorf("failed to render page %d: %v", i, err)
			continue
		}

		if len(html) == 0 {
			t.Errorf("page %d rendered empty HTML", i)
		}

		// Periodically check memory status
		if i%10 == 0 {
			metrics := app.GetApplicationMetrics()
			t.Logf("Created %d pages - Memory usage: %d bytes (%.1f%%)",
				i+1, metrics.MemoryUsage, metrics.MemoryUsagePercent)
		}
	}

	t.Logf("Successfully created %d pages", len(pages))

	// Test fragment generation under memory pressure
	ctx := context.Background()
	for i, page := range pages {
		newUserData := generateUserData(i, dataSize/2) // Smaller update

		fragments, err := page.RenderFragments(ctx, newUserData)
		if err != nil {
			t.Logf("Fragment generation failed for page %d (may be due to memory pressure): %v", i, err)
			continue
		}

		if len(fragments) == 0 {
			t.Logf("No fragments generated for page %d", i)
		}
	}

	// Test page retrieval by token
	for i, token := range tokens {
		retrievedPage, err := app.GetApplicationPage(token)
		if err != nil {
			t.Errorf("failed to retrieve page %d by token: %v", i, err)
			continue
		}

		if retrievedPage == nil {
			t.Errorf("page %d retrieved as nil", i)
		}
	}

	// Test memory cleanup
	initialMetrics := app.GetApplicationMetrics()
	cleanedCount := app.CleanupExpiredPages()
	finalMetrics := app.GetApplicationMetrics()

	t.Logf("Memory cleanup: %d pages cleaned", cleanedCount)
	t.Logf("Memory before cleanup: %d bytes", initialMetrics.MemoryUsage)
	t.Logf("Memory after cleanup: %d bytes", finalMetrics.MemoryUsage)

	// Close all pages
	for i, page := range pages {
		if err := page.Close(); err != nil {
			t.Errorf("failed to close page %d: %v", i, err)
		}
	}

	// Verify memory is released
	finalMetrics = app.GetApplicationMetrics()
	if finalMetrics.MemoryUsage > initialMetrics.MemoryUsage/10 {
		t.Errorf("memory not properly released after closing pages: %d bytes remaining",
			finalMetrics.MemoryUsage)
	}

	t.Logf("Integration test completed - Final memory usage: %d bytes", finalMetrics.MemoryUsage)
}

func TestApplication_MemoryPressureScenarios(t *testing.T) {
	// Test different memory pressure scenarios
	scenarios := []struct {
		name           string
		maxMemoryMB    int
		numPages       int
		pageSize       int
		expectFailures bool
	}{
		{
			name:           "Low memory pressure",
			maxMemoryMB:    50,
			numPages:       10,
			pageSize:       10 * 1024, // 10KB per page
			expectFailures: false,
		},
		{
			name:           "Medium memory pressure",
			maxMemoryMB:    1, // 1MB total
			numPages:       20,
			pageSize:       100 * 1024, // 100KB per page = 2MB total (exceeds limit)
			expectFailures: true,
		},
		{
			name:           "High memory pressure",
			maxMemoryMB:    1, // 1MB total
			numPages:       50,
			pageSize:       50 * 1024, // 50KB per page = 2.5MB total (far exceeds limit)
			expectFailures: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			app, err := NewApplication(WithMaxMemoryMB(scenario.maxMemoryMB))
			if err != nil {
				t.Fatalf("failed to create application: %v", err)
			}
			defer func() { _ = app.Close() }()

			tmpl, err := template.New("pressure-test").Parse(`<div>{{.Data}}</div>`)
			if err != nil {
				t.Fatalf("failed to create template: %v", err)
			}

			var successfulPages int
			var failedPages int
			var pages []*ApplicationPage

			// Check initial memory limit
			_ = app.GetApplicationMetrics()
			t.Logf("Memory limit configured: %.1fMB", float64(scenario.maxMemoryMB))

			for i := 0; i < scenario.numPages; i++ {
				// Generate safe text data instead of binary
				dataStr := fmt.Sprintf("Page %d data: %s", i,
					generateRepeatedText("sample text content ", scenario.pageSize/20))

				userData := map[string]interface{}{
					"Data": dataStr,
				}

				page, err := app.NewApplicationPage(tmpl, userData)
				if err != nil {
					t.Logf("Page %d failed (expected): %v", i, err)
					failedPages++
					continue
				}

				successfulPages++
				pages = append(pages, page)

				// Check current memory usage more frequently
				metrics := app.GetApplicationMetrics()
				if i%2 == 0 || metrics.MemoryUsagePercent > 50 {
					t.Logf("Page %d: Memory %d bytes (%.1f%%), Limit: %dMB", i, metrics.MemoryUsage, metrics.MemoryUsagePercent, scenario.maxMemoryMB)
				}

				// Break early if we hit memory issues to avoid infinite loops
				if metrics.MemoryUsagePercent > 150 {
					t.Logf("Breaking early due to high memory usage")
					break
				}
			}

			// Clean up all pages at the end
			for _, page := range pages {
				_ = page.Close()
			}

			t.Logf("Scenario %s: %d successful, %d failed pages",
				scenario.name, successfulPages, failedPages)

			if scenario.expectFailures && failedPages == 0 {
				t.Errorf("expected some failures under memory pressure, but all succeeded")
			}

			if !scenario.expectFailures && failedPages > 0 {
				t.Errorf("unexpected failures in low pressure scenario: %d failed", failedPages)
			}

			// Verify application is still functional after pressure
			finalMetrics := app.GetApplicationMetrics()
			if finalMetrics.ActivePages < 0 {
				t.Errorf("negative active pages after pressure test: %d", finalMetrics.ActivePages)
			}
		})
	}
}

func TestApplication_MemoryEfficiencyOptimizations(t *testing.T) {
	app, err := NewApplication(WithMaxMemoryMB(20))
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	defer func() { _ = app.Close() }()

	// Test template reuse efficiency
	tmpl, err := template.New("efficiency-test").Parse(`
		<div class="item">
			<h2>{{.Title}}</h2>
			<p>{{.Content}}</p>
			<span>{{.Timestamp}}</span>
		</div>
	`)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Create multiple pages with the same template (testing template reuse)
	const numPages = 30
	var pages []*ApplicationPage

	start := time.Now()
	for i := 0; i < numPages; i++ {
		data := map[string]interface{}{
			"Title":     generateTitle(i),
			"Content":   generateContent(i),
			"Timestamp": time.Now().Format(time.RFC3339),
		}

		page, err := app.NewApplicationPage(tmpl, data)
		if err != nil {
			t.Errorf("failed to create page %d: %v", i, err)
			continue
		}
		pages = append(pages, page)
	}
	createTime := time.Since(start)

	t.Logf("Created %d pages in %v (avg: %v per page)",
		len(pages), createTime, createTime/time.Duration(len(pages)))

	// Test fragment generation efficiency
	ctx := context.Background()
	start = time.Now()

	for i, page := range pages {
		newData := map[string]interface{}{
			"Title":     generateTitle(i) + " (updated)",
			"Content":   generateContent(i) + " - updated content",
			"Timestamp": time.Now().Format(time.RFC3339),
		}

		fragments, err := page.RenderFragments(ctx, newData)
		if err != nil {
			t.Errorf("failed to generate fragments for page %d: %v", i, err)
			continue
		}

		if len(fragments) == 0 {
			t.Errorf("no fragments generated for page %d", i)
		}
	}
	fragmentTime := time.Since(start)

	t.Logf("Generated fragments for %d pages in %v (avg: %v per page)",
		len(pages), fragmentTime, fragmentTime/time.Duration(len(pages)))

	// Test cleanup efficiency
	start = time.Now()
	for i, page := range pages {
		if err := page.Close(); err != nil {
			t.Errorf("failed to close page %d: %v", i, err)
		}
	}
	cleanupTime := time.Since(start)

	t.Logf("Cleaned up %d pages in %v (avg: %v per page)",
		len(pages), cleanupTime, cleanupTime/time.Duration(len(pages)))

	// Verify final state
	finalMetrics := app.GetApplicationMetrics()
	if finalMetrics.ActivePages != 0 {
		t.Errorf("expected 0 active pages after cleanup, got %d", finalMetrics.ActivePages)
	}

	// Calculate efficiency metrics
	totalTime := createTime + fragmentTime + cleanupTime
	pagesPerSecond := float64(len(pages)) / totalTime.Seconds()

	t.Logf("Overall efficiency: %.1f pages/second", pagesPerSecond)

	if pagesPerSecond < 100 { // Expect at least 100 pages/second
		t.Errorf("efficiency too low: %.1f pages/second", pagesPerSecond)
	}
}

// Helper functions for generating test data
func generateUserName(id int) string {
	names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
	return names[id%len(names)]
}

func generateRepeatedText(base string, times int) string {
	if times <= 0 {
		return base
	}
	result := ""
	for i := 0; i < times; i++ {
		result += base
	}
	return result
}

func generateTitle(id int) string {
	titles := []string{
		"Introduction to Go",
		"Advanced Memory Management",
		"Web Development Best Practices",
		"Database Optimization Techniques",
		"Security in Modern Applications",
		"Performance Monitoring",
		"Scaling Web Applications",
		"DevOps and Automation",
	}
	return titles[id%len(titles)]
}

func generateContent(id int) string {
	content := []string{
		"This is a comprehensive guide to understanding the fundamentals.",
		"Learn advanced techniques for optimizing your applications.",
		"Best practices that every developer should know and follow.",
		"Deep dive into performance optimization strategies.",
		"Security considerations for modern web applications.",
		"Monitoring and observability in production environments.",
		"Scaling strategies for high-traffic applications.",
		"Automation tools and practices for efficient development.",
	}
	return content[id%len(content)]
}
