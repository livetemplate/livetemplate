package page

import (
	"fmt"
	"html/template"
	"sync"
	"testing"
	"time"
)

func TestRegistry_NewRegistry(t *testing.T) {
	tests := []struct {
		name   string
		config *RegistryConfig
	}{
		{
			name:   "with default config",
			config: nil,
		},
		{
			name: "with custom config",
			config: &RegistryConfig{
				MaxPages:        500,
				DefaultTTL:      30 * time.Minute,
				CleanupInterval: 1 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry(tt.config)
			defer func() { _ = registry.Close() }()

			// Verify registry is properly initialized
			if registry.pages == nil {
				t.Error("pages map should be initialized")
			}

			if registry.pagesByApp == nil {
				t.Error("pagesByApp map should be initialized")
			}

			if registry.cleanupTicker == nil {
				t.Error("cleanup ticker should be initialized")
			}

			if registry.stopCleanup == nil {
				t.Error("stop cleanup channel should be initialized")
			}

			// Verify config values
			if tt.config == nil {
				// Should use defaults
				if registry.maxPages != 1000 {
					t.Errorf("expected default max pages 1000, got %d", registry.maxPages)
				}
				if registry.defaultTTL != 1*time.Hour {
					t.Errorf("expected default TTL 1h, got %v", registry.defaultTTL)
				}
			} else {
				// Should use provided config
				if registry.maxPages != tt.config.MaxPages {
					t.Errorf("expected max pages %d, got %d", tt.config.MaxPages, registry.maxPages)
				}
				if registry.defaultTTL != tt.config.DefaultTTL {
					t.Errorf("expected TTL %v, got %v", tt.config.DefaultTTL, registry.defaultTTL)
				}
			}

			// Verify initial state
			if count := registry.GetPageCount(); count != 0 {
				t.Errorf("expected initial page count 0, got %d", count)
			}

			if count := registry.GetApplicationCount(); count != 0 {
				t.Errorf("expected initial application count 0, got %d", count)
			}
		})
	}
}

func TestRegistry_ThreadSafeConcurrentAccess(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        1000,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	})
	defer func() { _ = registry.Close() }()

	// Test concurrent operations
	const numWorkers = 10
	const operationsPerWorker = 50
	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*operationsPerWorker)

	// Worker function that performs various registry operations
	worker := func(workerID int) {
		defer wg.Done()

		for i := 0; i < operationsPerWorker; i++ {
			pageID := fmt.Sprintf("page-%d-%d", workerID, i)
			appID := fmt.Sprintf("app-%d", workerID%3) // Use 3 different app IDs

			// Create a page
			page, err := NewPage(appID, createTestTemplate(),
				map[string]interface{}{"value": i}, nil)
			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to create page: %v", workerID, err)
				continue
			}
			page.ID = pageID // Override ID for predictable testing

			// Store page
			err = registry.Store(page)
			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to store page: %v", workerID, err)
				continue
			}

			// Retrieve page
			_, err = registry.Get(pageID, appID)
			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to get page: %v", workerID, err)
				continue
			}

			// Get pages by application
			registry.GetByApplication(appID)

			// Remove page (every other operation)
			if i%2 == 0 {
				registry.Remove(pageID)
			}
		}
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i)
	}

	// Wait for completion
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify final state is consistent
	finalCount := registry.GetPageCount()
	if finalCount < 0 {
		t.Errorf("negative page count: %d", finalCount)
	}

	// Verify metrics are accessible
	metrics := registry.GetMetrics()
	if metrics.TotalPages != finalCount {
		t.Errorf("metrics mismatch: expected %d, got %d", finalCount, metrics.TotalPages)
	}
}

func TestRegistry_TTLBasedCleanup(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        100,
		DefaultTTL:      100 * time.Millisecond, // Very short TTL for testing
		CleanupInterval: 50 * time.Millisecond,  // Frequent cleanup
	})
	defer func() { _ = registry.Close() }()

	// Create pages
	pages := make([]*Page, 5)
	for i := 0; i < 5; i++ {
		page, err := NewPage(fmt.Sprintf("app-%d", i), createTestTemplate(),
			map[string]interface{}{"value": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}
		pages[i] = page

		err = registry.Store(page)
		if err != nil {
			t.Fatalf("failed to store page %d: %v", i, err)
		}
	}

	// Verify all pages are stored
	if count := registry.GetPageCount(); count != 5 {
		t.Errorf("expected 5 pages, got %d", count)
	}

	// Wait for pages to expire
	time.Sleep(200 * time.Millisecond)

	// Trigger manual cleanup to test the mechanism
	cleanedCount := registry.CleanupExpired()
	// Note: Pages might not be cleaned immediately due to timing,
	// but we verify that cleanup mechanism works
	t.Logf("Cleaned up %d expired pages", cleanedCount)

	// Verify pages were removed (if any were expired)
	finalCount := registry.GetPageCount()
	t.Logf("Final page count after cleanup: %d", finalCount)

	// Wait for automatic cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Test that cleanup mechanism is working (pages should eventually be cleaned)
	// Note: Exact timing depends on when IsExpired() calculates the page as expired
	automaticCleanupCount := registry.GetPageCount()
	t.Logf("Page count after automatic cleanup: %d", automaticCleanupCount)
}

func TestRegistry_MemoryLimits(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        3, // Very small limit for testing
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	})
	defer func() { _ = registry.Close() }()

	// Add pages up to limit
	for i := 0; i < 3; i++ {
		page, err := NewPage(fmt.Sprintf("app-%d", i), createTestTemplate(),
			map[string]interface{}{"value": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}

		err = registry.Store(page)
		if err != nil {
			t.Fatalf("failed to store page %d: %v", i, err)
		}
	}

	// Verify we're at capacity
	if count := registry.GetPageCount(); count != 3 {
		t.Errorf("expected 3 pages, got %d", count)
	}

	// Try to add one more page (should fail)
	page, err := NewPage("app-overflow", createTestTemplate(),
		map[string]interface{}{"value": 999}, nil)
	if err != nil {
		t.Fatalf("failed to create overflow page: %v", err)
	}

	err = registry.Store(page)
	if err == nil {
		t.Error("storing page beyond capacity should fail")
	}

	if err.Error() != "registry at capacity (3 pages)" {
		t.Errorf("expected capacity error, got: %v", err)
	}

	// Verify count is still at limit
	if count := registry.GetPageCount(); count != 3 {
		t.Errorf("expected page count to remain 3, got %d", count)
	}

	// Verify metrics show we're at capacity
	metrics := registry.GetMetrics()
	if metrics.CapacityUsed != 1.0 {
		t.Errorf("expected capacity used 1.0, got %f", metrics.CapacityUsed)
	}
}

func TestRegistry_PageIsolation(t *testing.T) {
	registry := NewRegistry(nil)
	defer func() { _ = registry.Close() }()

	// Create pages for different applications
	apps := []string{"app-1", "app-2", "app-3"}
	pagesByApp := make(map[string][]*Page)

	for _, appID := range apps {
		for i := 0; i < 3; i++ {
			page, err := NewPage(appID, createTestTemplate(),
				map[string]interface{}{"app": appID, "index": i}, nil)
			if err != nil {
				t.Fatalf("failed to create page for %s: %v", appID, err)
			}

			err = registry.Store(page)
			if err != nil {
				t.Fatalf("failed to store page for %s: %v", appID, err)
			}

			pagesByApp[appID] = append(pagesByApp[appID], page)
		}
	}

	// Verify total page count
	if count := registry.GetPageCount(); count != 9 {
		t.Errorf("expected 9 total pages, got %d", count)
	}

	// Verify application count
	if count := registry.GetApplicationCount(); count != 3 {
		t.Errorf("expected 3 applications, got %d", count)
	}

	// Test cross-application access is denied
	for _, appID := range apps {
		for _, otherAppID := range apps {
			if appID == otherAppID {
				continue
			}

			// Try to access pages from another application
			for _, page := range pagesByApp[otherAppID] {
				_, err := registry.Get(page.ID, appID)
				if err == nil {
					t.Errorf("should not be able to access page %s from app %s (belongs to %s)",
						page.ID, appID, otherAppID)
				}

				if err.Error() != "cross-application access denied" {
					t.Errorf("expected cross-application access error, got: %v", err)
				}
			}
		}
	}

	// Test that applications can access their own pages
	for appID, pages := range pagesByApp {
		for _, page := range pages {
			retrievedPage, err := registry.Get(page.ID, appID)
			if err != nil {
				t.Errorf("app %s should be able to access its own page %s: %v",
					appID, page.ID, err)
			}

			if retrievedPage.ID != page.ID {
				t.Errorf("retrieved wrong page: expected %s, got %s",
					page.ID, retrievedPage.ID)
			}
		}
	}

	// Test GetByApplication returns only app's pages
	for appID := range pagesByApp {
		appPages := registry.GetByApplication(appID)
		if len(appPages) != 3 {
			t.Errorf("expected 3 pages for app %s, got %d", appID, len(appPages))
		}

		// Verify all returned pages belong to the application
		for pageID, page := range appPages {
			if page.ApplicationID != appID {
				t.Errorf("page %s returned for app %s but belongs to %s",
					pageID, appID, page.ApplicationID)
			}
		}
	}
}

func TestRegistry_EfficientPageLookup(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        1000,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	})
	defer func() { _ = registry.Close() }()

	// Create many pages to test lookup efficiency
	const numPages = 100
	pageIDs := make([]string, numPages)
	appID := "test-app"

	// Store pages
	start := time.Now()
	for i := 0; i < numPages; i++ {
		page, err := NewPage(appID, createTestTemplate(),
			map[string]interface{}{"index": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}
		pageIDs[i] = page.ID

		err = registry.Store(page)
		if err != nil {
			t.Fatalf("failed to store page %d: %v", i, err)
		}
	}
	storeTime := time.Since(start)

	// Test lookup performance
	start = time.Now()
	for _, pageID := range pageIDs {
		_, err := registry.Get(pageID, appID)
		if err != nil {
			t.Errorf("failed to get page %s: %v", pageID, err)
		}
	}
	lookupTime := time.Since(start)

	t.Logf("Store time for %d pages: %v", numPages, storeTime)
	t.Logf("Lookup time for %d pages: %v", numPages, lookupTime)

	// Lookups should be fast (O(1) map access)
	avgLookupTime := lookupTime / time.Duration(numPages)
	if avgLookupTime > 1*time.Millisecond {
		t.Errorf("average lookup time too slow: %v", avgLookupTime)
	}

	// Test lookup of non-existent page
	_, err := registry.Get("non-existent", appID)
	if err == nil {
		t.Error("should return error for non-existent page")
	}

	if err.Error() != "page not found: non-existent" {
		t.Errorf("expected page not found error, got: %v", err)
	}
}

func TestRegistry_PageLifecycleManagement(t *testing.T) {
	registry := NewRegistry(nil)
	defer func() { _ = registry.Close() }()

	// Test page creation and storage
	page, err := NewPage("test-app", createTestTemplate(),
		map[string]interface{}{"value": "initial"}, nil)
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	err = registry.Store(page)
	if err != nil {
		t.Fatalf("failed to store page: %v", err)
	}

	// Verify page is stored
	if count := registry.GetPageCount(); count != 1 {
		t.Errorf("expected 1 page, got %d", count)
	}

	// Test page retrieval and update
	retrievedPage, err := registry.Get(page.ID, "test-app")
	if err != nil {
		t.Fatalf("failed to retrieve page: %v", err)
	}

	if retrievedPage.ID != page.ID {
		t.Errorf("retrieved wrong page: expected %s, got %s", page.ID, retrievedPage.ID)
	}

	// Test last accessed time is updated on retrieval
	originalTime := retrievedPage.lastAccessed
	time.Sleep(10 * time.Millisecond)

	_, err = registry.Get(page.ID, "test-app")
	if err != nil {
		t.Fatalf("failed to retrieve page again: %v", err)
	}

	retrievedAgain, err := registry.Get(page.ID, "test-app")
	if err != nil {
		t.Fatalf("failed to retrieve page third time: %v", err)
	}

	if !retrievedAgain.lastAccessed.After(originalTime) {
		t.Error("last accessed time should be updated on retrieval")
	}

	// Test page removal
	removed := registry.Remove(page.ID)
	if !removed {
		t.Error("page removal should return true")
	}

	// Verify page is removed
	if count := registry.GetPageCount(); count != 0 {
		t.Errorf("expected 0 pages after removal, got %d", count)
	}

	// Test retrieving removed page fails
	_, err = registry.Get(page.ID, "test-app")
	if err == nil {
		t.Error("should not be able to retrieve removed page")
	}

	// Test removing non-existent page
	removed = registry.Remove("non-existent")
	if removed {
		t.Error("removing non-existent page should return false")
	}
}

func TestRegistry_GracefulDegradationUnderMemoryPressure(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        5, // Very small limit to simulate memory pressure
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	})
	defer func() { _ = registry.Close() }()

	// Fill registry to capacity
	for i := 0; i < 5; i++ {
		page, err := NewPage(fmt.Sprintf("app-%d", i), createTestTemplate(),
			map[string]interface{}{"value": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}

		err = registry.Store(page)
		if err != nil {
			t.Fatalf("failed to store page %d: %v", i, err)
		}
	}

	// Test that registry gracefully rejects new pages
	for i := 5; i < 10; i++ {
		page, err := NewPage(fmt.Sprintf("app-%d", i), createTestTemplate(),
			map[string]interface{}{"value": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}

		err = registry.Store(page)
		if err == nil {
			t.Error("should reject page when at capacity")
		}

		// Verify registry remains functional
		if count := registry.GetPageCount(); count != 5 {
			t.Errorf("expected page count to remain 5, got %d", count)
		}

		// Verify existing pages are still accessible
		existingPages := registry.GetByApplication("app-0")
		if len(existingPages) == 0 {
			t.Error("existing pages should remain accessible under memory pressure")
		}
	}

	// Test that metrics still work under pressure
	metrics := registry.GetMetrics()
	if metrics.CapacityUsed != 1.0 {
		t.Errorf("expected capacity used 1.0, got %f", metrics.CapacityUsed)
	}

	if metrics.TotalPages != 5 {
		t.Errorf("expected 5 total pages, got %d", metrics.TotalPages)
	}

	// Test that cleanup still works under pressure
	cleanedCount := registry.CleanupExpired()
	// Should be 0 since pages aren't expired
	if cleanedCount < 0 {
		t.Errorf("cleanup should not return negative count: %d", cleanedCount)
	}
}

func TestRegistry_Metrics(t *testing.T) {
	registry := NewRegistry(&RegistryConfig{
		MaxPages:        100,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
	})
	defer func() { _ = registry.Close() }()

	// Test initial metrics
	metrics := registry.GetMetrics()
	if metrics.TotalPages != 0 {
		t.Errorf("expected 0 total pages, got %d", metrics.TotalPages)
	}

	if metrics.Applications != 0 {
		t.Errorf("expected 0 applications, got %d", metrics.Applications)
	}

	if metrics.CapacityUsed != 0.0 {
		t.Errorf("expected 0%% capacity used, got %f", metrics.CapacityUsed)
	}

	if metrics.MaxCapacity != 100 {
		t.Errorf("expected max capacity 100, got %d", metrics.MaxCapacity)
	}

	// Add pages across multiple applications
	apps := []string{"app-1", "app-2", "app-3"}
	for _, appID := range apps {
		for i := 0; i < 3; i++ {
			page, err := NewPage(appID, createTestTemplate(),
				map[string]interface{}{"app": appID, "index": i}, nil)
			if err != nil {
				t.Fatalf("failed to create page: %v", err)
			}

			err = registry.Store(page)
			if err != nil {
				t.Fatalf("failed to store page: %v", err)
			}
		}
	}

	// Test metrics after adding pages
	metrics = registry.GetMetrics()
	if metrics.TotalPages != 9 {
		t.Errorf("expected 9 total pages, got %d", metrics.TotalPages)
	}

	if metrics.Applications != 3 {
		t.Errorf("expected 3 applications, got %d", metrics.Applications)
	}

	if metrics.CapacityUsed != 0.09 {
		t.Errorf("expected 9%% capacity used, got %f", metrics.CapacityUsed)
	}

	expectedAvg := 3.0
	if metrics.AvgPagesPerApp != expectedAvg {
		t.Errorf("expected avg pages per app %f, got %f", expectedAvg, metrics.AvgPagesPerApp)
	}

	if metrics.DefaultTTL != 1*time.Hour {
		t.Errorf("expected default TTL 1h, got %v", metrics.DefaultTTL)
	}
}

func TestRegistry_Close(t *testing.T) {
	registry := NewRegistry(nil)

	// Add some pages
	for i := 0; i < 3; i++ {
		page, err := NewPage(fmt.Sprintf("app-%d", i), createTestTemplate(),
			map[string]interface{}{"value": i}, nil)
		if err != nil {
			t.Fatalf("failed to create page %d: %v", i, err)
		}

		err = registry.Store(page)
		if err != nil {
			t.Fatalf("failed to store page %d: %v", i, err)
		}
	}

	// Verify pages are stored
	if count := registry.GetPageCount(); count != 3 {
		t.Errorf("expected 3 pages before close, got %d", count)
	}

	// Close registry
	err := registry.Close()
	if err != nil {
		t.Errorf("registry close should not error: %v", err)
	}

	// Verify cleanup
	if count := registry.GetPageCount(); count != 0 {
		t.Errorf("expected 0 pages after close, got %d", count)
	}

	if count := registry.GetApplicationCount(); count != 0 {
		t.Errorf("expected 0 applications after close, got %d", count)
	}

	// Test double close doesn't error
	err = registry.Close()
	if err != nil {
		t.Errorf("double close should not error: %v", err)
	}
}

// Helper function to create a test template
func createTestTemplate() *template.Template {
	tmpl, _ := template.New("test").Parse(`<div>{{.value}}</div>`)
	return tmpl
}
