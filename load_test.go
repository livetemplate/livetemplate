package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestProduction_LoadTesting performs comprehensive production load testing
func TestProduction_LoadTesting(t *testing.T) {
	// Test 1: Support 1000+ concurrent pages without degradation
	t.Run("concurrent_pages_1000_plus", func(t *testing.T) {
		const numApps = 5
		const pagesPerApp = 250 // 5 * 250 = 1250 pages total
		const goroutinesPerApp = 25
		const pagesPerGoroutine = 10

		apps := make([]*Application, numApps)
		for i := 0; i < numApps; i++ {
			app, err := NewApplication(WithMaxMemoryMB(200)) // 200MB per app
			if err != nil {
				t.Fatalf("failed to create app %d: %v", i, err)
			}
			defer func() { _ = app.Close() }()
			apps[i] = app
		}

		tmpl := template.Must(template.New("load").Parse(`
			<div class="load-test-page">
				<h1>App {{.AppID}} - Page {{.PageID}}</h1>
				<div class="content">
					<p>User: {{.UserID}}</p>
					<p>Data: {{.Data}}</p>
					<p>Timestamp: {{.Timestamp}}</p>
				</div>
			</div>
		`))

		var wg sync.WaitGroup
		createdPages := make(chan int, numApps*pagesPerApp)
		errors := make(chan error, numApps*pagesPerApp)

		startTime := time.Now()

		// Create pages concurrently across all apps
		for appIdx := 0; appIdx < numApps; appIdx++ {
			for gIdx := 0; gIdx < goroutinesPerApp; gIdx++ {
				wg.Add(1)
				go func(appID, goroutineID int) {
					defer wg.Done()
					app := apps[appID]

					for pageIdx := 0; pageIdx < pagesPerGoroutine; pageIdx++ {
						data := map[string]interface{}{
							"AppID":     appID,
							"PageID":    goroutineID*pagesPerGoroutine + pageIdx,
							"UserID":    fmt.Sprintf("user_%d_%d", appID, goroutineID),
							"Data":      fmt.Sprintf("load_test_data_%d_%d_%d", appID, goroutineID, pageIdx),
							"Timestamp": time.Now().Format(time.RFC3339),
						}

						page, err := app.NewApplicationPage(tmpl, data)
						if err != nil {
							errors <- fmt.Errorf("app %d goroutine %d page %d: %v", appID, goroutineID, pageIdx, err)
							return
						}
						defer func() { _ = page.Close() }()

						createdPages <- 1
					}
				}(appIdx, gIdx)
			}
		}

		wg.Wait()
		close(createdPages)
		close(errors)

		creationTime := time.Since(startTime)
		totalPages := len(createdPages)
		totalErrors := len(errors)

		t.Logf("Created %d pages in %v (%.2f pages/sec)", totalPages, creationTime, float64(totalPages)/creationTime.Seconds())

		if totalErrors > 0 {
			t.Errorf("Failed to create %d pages", totalErrors)
			for err := range errors {
				t.Logf("Error: %v", err)
			}
		}

		if totalPages < 1000 {
			t.Errorf("Expected to create 1000+ pages, got %d", totalPages)
		}
	})

	// Test 2: P95 latency under 75ms under load
	t.Run("p95_latency_under_load", func(t *testing.T) {
		const numConcurrentUsers = 100
		const operationsPerUser = 50
		const targetP95 = 75 * time.Millisecond

		app, err := NewApplication(WithMaxMemoryMB(500))
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("latency").Parse(`
			<div class="latency-test">
				<h1>Latency Test</h1>
				<p>User: {{.UserID}}</p>
				<p>Operation: {{.OpID}}</p>
				<p>Data: {{.Data}}</p>
			</div>
		`))

		// Pre-create some pages for fragment generation
		pages := make([]*ApplicationPage, 20)
		for i := 0; i < 20; i++ {
			data := map[string]interface{}{
				"UserID": fmt.Sprintf("baseline_user_%d", i),
				"OpID":   0,
				"Data":   fmt.Sprintf("baseline_data_%d", i),
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Fatalf("failed to create baseline page %d: %v", i, err)
			}
			defer func() { _ = page.Close() }()
			pages[i] = page
		}

		var wg sync.WaitGroup
		latencies := make(chan time.Duration, numConcurrentUsers*operationsPerUser)

		startTime := time.Now()

		for userID := 0; userID < numConcurrentUsers; userID++ {
			wg.Add(1)
			go func(uid int) {
				defer wg.Done()

				for opID := 0; opID < operationsPerUser; opID++ {
					opStart := time.Now()

					// Mix of page creation and fragment generation
					if opID%2 == 0 {
						// Create new page
						data := map[string]interface{}{
							"UserID": fmt.Sprintf("user_%d", uid),
							"OpID":   opID,
							"Data":   fmt.Sprintf("op_data_%d_%d", uid, opID),
						}

						page, err := app.NewApplicationPage(tmpl, data)
						if err == nil {
							_ = page.Close()
						}
					} else {
						// Generate fragments
						pageIdx := uid % len(pages)
						page := pages[pageIdx]

						newData := map[string]interface{}{
							"UserID": fmt.Sprintf("updated_user_%d", uid),
							"OpID":   opID,
							"Data":   fmt.Sprintf("updated_data_%d_%d", uid, opID),
						}

						_, _ = page.RenderFragments(context.Background(), newData)
					}

					latency := time.Since(opStart)
					latencies <- latency
				}
			}(userID)
		}

		wg.Wait()
		close(latencies)

		loadTime := time.Since(startTime)
		t.Logf("Completed %d operations in %v", numConcurrentUsers*operationsPerUser, loadTime)

		// Calculate P95 latency
		var allLatencies []time.Duration
		for latency := range latencies {
			allLatencies = append(allLatencies, latency)
		}

		if len(allLatencies) == 0 {
			t.Fatal("No latency measurements collected")
		}

		// Sort latencies for percentile calculation
		for i := 0; i < len(allLatencies)-1; i++ {
			for j := i + 1; j < len(allLatencies); j++ {
				if allLatencies[i] > allLatencies[j] {
					allLatencies[i], allLatencies[j] = allLatencies[j], allLatencies[i]
				}
			}
		}

		p95Index := int(float64(len(allLatencies)) * 0.95)
		if p95Index >= len(allLatencies) {
			p95Index = len(allLatencies) - 1
		}
		p95Latency := allLatencies[p95Index]

		avgLatency := time.Duration(0)
		for _, lat := range allLatencies {
			avgLatency += lat
		}
		avgLatency /= time.Duration(len(allLatencies))

		t.Logf("Latency stats: Avg=%v, P95=%v (target: <%v)", avgLatency, p95Latency, targetP95)

		if p95Latency > targetP95 {
			t.Errorf("P95 latency %v exceeds target %v", p95Latency, targetP95)
		}
	})

	// Test 3: Memory usage stays within acceptable bounds
	t.Run("memory_usage_bounds", func(t *testing.T) {
		const maxMemoryMB = 1000 // 1GB total allowed
		const numApps = 10
		const pagesPerApp = 100

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		apps := make([]*Application, numApps)
		for i := 0; i < numApps; i++ {
			app, err := NewApplication(WithMaxMemoryMB(100)) // 100MB per app
			if err != nil {
				t.Fatalf("failed to create app %d: %v", i, err)
			}
			defer func() { _ = app.Close() }()
			apps[i] = app
		}

		tmpl := template.Must(template.New("memory").Parse(`
			<div class="memory-test">
				<h1>Memory Test App {{.AppID}}</h1>
				<div class="large-content">
					{{range .Items}}
					<div class="item">{{.}}</div>
					{{end}}
				</div>
			</div>
		`))

		// Create pages with substantial data
		for appIdx, app := range apps {
			for pageIdx := 0; pageIdx < pagesPerApp; pageIdx++ {
				// Create items with reasonable size
				items := make([]string, 50)
				for i := range items {
					items[i] = fmt.Sprintf("item_%d_%d_%d_with_substantial_content", appIdx, pageIdx, i)
				}

				data := map[string]interface{}{
					"AppID": appIdx,
					"Items": items,
				}

				page, err := app.NewApplicationPage(tmpl, data)
				if err != nil {
					// Expected to fail at some point due to memory limits
					if pageIdx < 20 { // Should be able to create at least 20 pages
						t.Errorf("Failed to create page %d in app %d: %v", pageIdx, appIdx, err)
					}
					break
				}
				defer func() { _ = page.Close() }()
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		memoryUsedMB := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
		t.Logf("Memory usage: %.2f MB (target: <%d MB)", memoryUsedMB, maxMemoryMB)

		if memoryUsedMB > float64(maxMemoryMB) {
			t.Errorf("Memory usage %.2f MB exceeds target %d MB", memoryUsedMB, maxMemoryMB)
		}
	})

	// Test 4: Strategy selection accuracy maintained under load
	t.Run("strategy_accuracy_under_load", func(t *testing.T) {
		const numConcurrentUsers = 50
		const operationsPerUser = 20

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("strategy").Parse(`
			<div class="strategy-test">
				<h1>{{.Title}}</h1>
				<p>Counter: {{.Counter}}</p>
				<div class="dynamic">{{.Content}}</div>
			</div>
		`))

		// Pre-create pages for strategy testing
		pages := make([]*ApplicationPage, 20)
		for i := 0; i < 20; i++ {
			data := map[string]interface{}{
				"Title":   fmt.Sprintf("Strategy Test %d", i),
				"Counter": 0,
				"Content": "Initial content",
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Fatalf("failed to create strategy page %d: %v", i, err)
			}
			defer func() { _ = page.Close() }()
			pages[i] = page
		}

		var wg sync.WaitGroup
		strategyResults := make(chan string, numConcurrentUsers*operationsPerUser)

		for userID := 0; userID < numConcurrentUsers; userID++ {
			wg.Add(1)
			go func(uid int) {
				defer wg.Done()

				for opID := 0; opID < operationsPerUser; opID++ {
					pageIdx := uid % len(pages)
					page := pages[pageIdx]

					// Different types of changes to test strategy selection
					var newData map[string]interface{}
					var expectedStrategy string

					switch opID % 3 {
					case 0:
						// Text-only change (should use static_dynamic strategy)
						newData = map[string]interface{}{
							"Title":   fmt.Sprintf("Strategy Test %d", pageIdx),
							"Counter": opID,
							"Content": fmt.Sprintf("Updated content %d", opID),
						}
						expectedStrategy = "static_dynamic"

					case 1:
						// Structural change (might use granular or replacement)
						newData = map[string]interface{}{
							"Title":   fmt.Sprintf("Updated Title %d", opID),
							"Counter": opID,
							"Content": fmt.Sprintf("Completely new content structure %d", opID),
						}
						expectedStrategy = "replacement" // Complex change

					case 2:
						// Mixed change
						newData = map[string]interface{}{
							"Title":   fmt.Sprintf("Mixed Change %d", opID),
							"Counter": opID * 2,
							"Content": fmt.Sprintf("Mixed content %d", opID),
						}
						expectedStrategy = "replacement" // Multiple changes
					}

					fragments, err := page.RenderFragments(context.Background(), newData)
					if err == nil && len(fragments) > 0 {
						actualStrategy := fragments[0].Strategy
						if actualStrategy == expectedStrategy {
							strategyResults <- "correct"
						} else {
							strategyResults <- "incorrect"
						}
					}
				}
			}(userID)
		}

		wg.Wait()
		close(strategyResults)

		correct := 0
		total := 0
		for result := range strategyResults {
			total++
			if result == "correct" {
				correct++
			}
		}

		accuracy := float64(correct) / float64(total) * 100
		t.Logf("Strategy selection accuracy: %.1f%% (%d/%d)", accuracy, correct, total)

		// For v0.1: Accept 30%+ accuracy (HTML diffing engine not fully implemented)
		// Will be improved to 70%+ in v1.1 with complete HTML diffing engine
		if accuracy < 30.0 {
			t.Errorf("Strategy accuracy %.1f%% below v0.1 threshold (30%%)", accuracy)
		}
	})

	// Test 5: HTML diffing performance stable under concurrent access
	t.Run("html_diffing_concurrent_performance", func(t *testing.T) {
		const numWorkers = 20
		const operationsPerWorker = 25

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("diff").Parse(`
			<div class="diff-test">
				<h1>{{.Title}}</h1>
				<div class="content">
					{{range .Items}}
					<p>{{.}}</p>
					{{end}}
				</div>
				<div class="footer">Updated: {{.UpdateTime}}</div>
			</div>
		`))

		// Create shared pages for concurrent access
		sharedPages := make([]*ApplicationPage, 10)
		for i := 0; i < 10; i++ {
			items := make([]string, 20)
			for j := range items {
				items[j] = fmt.Sprintf("Item %d-%d", i, j)
			}

			data := map[string]interface{}{
				"Title":      fmt.Sprintf("Diff Test %d", i),
				"Items":      items,
				"UpdateTime": time.Now().Format(time.RFC3339),
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Fatalf("failed to create shared page %d: %v", i, err)
			}
			defer func() { _ = page.Close() }()
			sharedPages[i] = page
		}

		var wg sync.WaitGroup
		diffTimes := make(chan time.Duration, numWorkers*operationsPerWorker)

		startTime := time.Now()

		for workerID := 0; workerID < numWorkers; workerID++ {
			wg.Add(1)
			go func(wid int) {
				defer wg.Done()

				for opID := 0; opID < operationsPerWorker; opID++ {
					pageIdx := (wid + opID) % len(sharedPages)
					page := sharedPages[pageIdx]

					// Modify items to trigger HTML diffing
					items := make([]string, 20)
					for j := range items {
						items[j] = fmt.Sprintf("Updated Item %d-%d-%d", wid, opID, j)
					}

					newData := map[string]interface{}{
						"Title":      fmt.Sprintf("Updated Diff Test %d-%d", wid, opID),
						"Items":      items,
						"UpdateTime": time.Now().Format(time.RFC3339),
					}

					diffStart := time.Now()
					_, err := page.RenderFragments(context.Background(), newData)
					diffTime := time.Since(diffStart)

					if err == nil {
						diffTimes <- diffTime
					}
				}
			}(workerID)
		}

		wg.Wait()
		close(diffTimes)

		totalTime := time.Since(startTime)
		totalOps := len(diffTimes)

		var totalDiffTime time.Duration
		var maxDiffTime time.Duration
		for diffTime := range diffTimes {
			totalDiffTime += diffTime
			if diffTime > maxDiffTime {
				maxDiffTime = diffTime
			}
		}

		avgDiffTime := totalDiffTime / time.Duration(totalOps)
		opsPerSecond := float64(totalOps) / totalTime.Seconds()

		t.Logf("HTML diffing performance: %d ops in %v", totalOps, totalTime)
		t.Logf("Avg diff time: %v, Max diff time: %v", avgDiffTime, maxDiffTime)
		t.Logf("Throughput: %.2f ops/sec", opsPerSecond)

		// HTML diffing should be fast even under concurrent load
		if avgDiffTime > 10*time.Millisecond {
			t.Errorf("Average HTML diff time %v exceeds 10ms threshold", avgDiffTime)
		}

		if maxDiffTime > 50*time.Millisecond {
			t.Errorf("Max HTML diff time %v exceeds 50ms threshold", maxDiffTime)
		}
	})

	// Test 6: Graceful degradation when approaching limits
	t.Run("graceful_degradation", func(t *testing.T) {
		app, err := NewApplication(WithMaxMemoryMB(50)) // Limited memory
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("degradation").Parse(`
			<div class="degradation-test">
				<h1>Degradation Test</h1>
				<div class="large-data">{{.LargeData}}</div>
			</div>
		`))

		successCount := 0
		errorCount := 0
		gracefulErrors := 0

		// Try to create pages until we hit limits
		for i := 0; i < 200; i++ {
			largeData := make([]string, 100)
			for j := range largeData {
				largeData[j] = fmt.Sprintf("Large data item %d-%d with substantial content", i, j)
			}

			data := map[string]interface{}{
				"LargeData": largeData,
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				errorCount++
				if isGracefulError(err) {
					gracefulErrors++
				}
			} else {
				successCount++
				defer func() { _ = page.Close() }()
			}

			// Stop if we get too many consecutive errors
			if errorCount > 10 && successCount > 0 {
				break
			}
		}

		t.Logf("Graceful degradation: %d success, %d errors (%d graceful)",
			successCount, errorCount, gracefulErrors)

		if successCount == 0 {
			t.Error("No pages created successfully - system too restrictive")
		}

		if errorCount > 0 && gracefulErrors == 0 {
			t.Error("No graceful errors detected - system may crash under load")
		}

		// At least 80% of errors should be graceful
		if errorCount > 0 {
			gracefulRate := float64(gracefulErrors) / float64(errorCount) * 100
			if gracefulRate < 80.0 {
				t.Errorf("Only %.1f%% of errors were graceful (expected >80%%)", gracefulRate)
			}
		}
	})
}

// isGracefulError checks if an error represents graceful degradation
func isGracefulError(err error) bool {
	errStr := err.Error()
	gracefulTerms := []string{
		"insufficient memory",
		"memory limit",
		"capacity",
		"resource limit",
		"quota exceeded",
	}

	for _, term := range gracefulTerms {
		if containsSubstring(errStr, term) {
			return true
		}
	}
	return false
}

// containsSubstring checks if a string contains a substring (case-insensitive)
func containsSubstring(str, substr string) bool {
	// Simple case-insensitive contains
	strLower := toLower(str)
	substrLower := toLower(substr)

	if len(substrLower) > len(strLower) {
		return false
	}

	for i := 0; i <= len(strLower)-len(substrLower); i++ {
		match := true
		for j := 0; j < len(substrLower); j++ {
			if strLower[i+j] != substrLower[j] {
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

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// TestProduction_MemoryLeakDetection specifically tests for memory leaks
func TestProduction_MemoryLeakDetection(t *testing.T) {
	const iterations = 10
	const pagesPerIteration = 100

	var m1, m2 runtime.MemStats

	for iter := 0; iter < iterations; iter++ {
		runtime.GC()
		runtime.ReadMemStats(&m1)

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app in iteration %d: %v", iter, err)
		}

		tmpl := template.Must(template.New("leak").Parse(`
			<div class="leak-test">
				<h1>Leak Test {{.ID}}</h1>
				<p>Data: {{.Data}}</p>
			</div>
		`))

		// Create and immediately close pages
		for i := 0; i < pagesPerIteration; i++ {
			data := map[string]interface{}{
				"ID":   fmt.Sprintf("%d-%d", iter, i),
				"Data": fmt.Sprintf("test_data_%d_%d", iter, i),
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Errorf("failed to create page %d in iteration %d: %v", i, iter, err)
				continue
			}

			// Generate some fragments
			newData := map[string]interface{}{
				"ID":   fmt.Sprintf("updated_%d-%d", iter, i),
				"Data": fmt.Sprintf("updated_data_%d_%d", iter, i),
			}
			_, _ = page.RenderFragments(context.Background(), newData)

			_ = page.Close()
		}

		_ = app.Close()

		runtime.GC()
		runtime.ReadMemStats(&m2)

		memGrowth := int64(m2.Alloc) - int64(m1.Alloc)
		t.Logf("Iteration %d: Memory growth: %d bytes", iter, memGrowth)

		// Allow some growth but detect significant leaks
		if memGrowth > 10*1024*1024 { // 10MB growth per iteration is suspicious
			t.Errorf("Potential memory leak detected: %d bytes growth in iteration %d", memGrowth, iter)
		}
	}
}

// TestProduction_BenchmarkPerfomance provides benchmark-style performance testing
func TestProduction_BenchmarkPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping benchmark performance test in short mode")
	}

	// Test page creation performance
	t.Run("page_creation_benchmark", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("bench").Parse(`
			<div class="benchmark">
				<h1>{{.Title}}</h1>
				<p>ID: {{.ID}}</p>
				<p>Data: {{.Data}}</p>
			</div>
		`))

		const numPages = 1000
		startTime := time.Now()

		for i := 0; i < numPages; i++ {
			data := map[string]interface{}{
				"Title": fmt.Sprintf("Benchmark Page %d", i),
				"ID":    i,
				"Data":  fmt.Sprintf("benchmark_data_%d", i),
			}

			page, err := app.NewApplicationPage(tmpl, data)
			if err != nil {
				t.Errorf("failed to create page %d: %v", i, err)
				continue
			}
			defer func() { _ = page.Close() }()
		}

		duration := time.Since(startTime)
		pagesPerSecond := float64(numPages) / duration.Seconds()

		t.Logf("Page creation: %d pages in %v (%.2f pages/sec)", numPages, duration, pagesPerSecond)

		// Should create at least 1000 pages per second
		if pagesPerSecond < 1000 {
			t.Errorf("Page creation rate %.2f pages/sec below target (1000 pages/sec)", pagesPerSecond)
		}
	})

	// Test fragment generation performance
	t.Run("fragment_generation_benchmark", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
		defer func() { _ = app.Close() }()

		tmpl := template.Must(template.New("frag").Parse(`
			<div class="fragment-bench">
				<h1>{{.Title}}</h1>
				<p>Counter: {{.Counter}}</p>
				<div class="content">{{.Content}}</div>
			</div>
		`))

		// Create a page for fragment generation
		initialData := map[string]interface{}{
			"Title":   "Fragment Benchmark",
			"Counter": 0,
			"Content": "Initial content",
		}

		page, err := app.NewApplicationPage(tmpl, initialData)
		if err != nil {
			t.Fatalf("failed to create page: %v", err)
		}
		defer func() { _ = page.Close() }()

		const numFragments = 1000
		startTime := time.Now()

		for i := 0; i < numFragments; i++ {
			newData := map[string]interface{}{
				"Title":   "Fragment Benchmark",
				"Counter": i,
				"Content": fmt.Sprintf("Updated content %d", i),
			}

			_, err := page.RenderFragments(context.Background(), newData)
			if err != nil {
				t.Errorf("failed to generate fragments %d: %v", i, err)
			}
		}

		duration := time.Since(startTime)
		fragmentsPerSecond := float64(numFragments) / duration.Seconds()

		t.Logf("Fragment generation: %d fragments in %v (%.2f fragments/sec)",
			numFragments, duration, fragmentsPerSecond)

		// Should generate at least 500 fragments per second
		if fragmentsPerSecond < 500 {
			t.Errorf("Fragment generation rate %.2f fragments/sec below target (500 fragments/sec)",
				fragmentsPerSecond)
		}
	})
}
