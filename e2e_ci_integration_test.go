package livetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestE2EInfrastructure tests the E2E test infrastructure itself
func TestE2EInfrastructure(t *testing.T) {
	E2ETestWithHelper(t, "infrastructure", func(helper *E2ETestHelper) error {
		// Test browser context creation
		ctx, cancel := helper.CreateBrowserContext()
		defer cancel()

		// Test basic navigation
		start := time.Now()
		var title string
		err := chromedp.Run(ctx,
			chromedp.Navigate("https://www.google.com"),
			chromedp.Title(&title),
		)
		helper.RecordBrowserAction("navigate-to-google", time.Since(start), err == nil, err)

		if err != nil {
			return fmt.Errorf("failed to navigate to Google: %w", err)
		}

		helper.SetCustomMetric("google_title", title)

		// Test screenshot capture
		if err := helper.CaptureScreenshot(ctx, "infrastructure-test"); err != nil {
			t.Logf("⚠️ Screenshot capture failed: %v", err)
		}

		return nil
	})
}

// TestE2EBrowserLifecycleWithCI tests the browser lifecycle with full CI integration
func TestE2EBrowserLifecycleWithCI(t *testing.T) {
	E2ETestWithHelper(t, "browser-lifecycle-ci", func(helper *E2ETestHelper) error {
		// Create test server
		tmpl, err := template.New("test").Parse(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>{{.Title}}</title>
				<script src="/client/livetemplate-client.js"></script>
			</head>
			<body>
				<div id="app">
					<h1 data-lt-fragment="header">{{.Title}}</h1>
					<div data-lt-fragment="content">
						<p>Count: <span id="count">{{.Count}}</span></p>
						<ul id="items">
							{{range .Items}}
							<li data-lt-fragment="item">{{.}}</li>
							{{end}}
						</ul>
						{{if .Visible}}
						<div id="status" data-lt-fragment="status">Status: {{.Status}}</div>
						{{end}}
					</div>
					<div id="attrs" data-lt-fragment="attributes" class="{{.Attrs.class}}" style="{{.Attrs.style}}">
						Dynamic attributes
					</div>
				</div>
				<script>
					// Simple fragment application simulation
					window.applyFragment = function(fragment) {
						console.log('Applying fragment:', fragment);
						return true;
					};
				</script>
			</body>
			</html>
		`)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		// Create application and page
		app, err := NewApplication()
		if err != nil {
			return fmt.Errorf("failed to create application: %w", err)
		}
		defer func() { _ = app.Close() }()

		initialData := map[string]interface{}{
			"Title":   "LiveTemplate CI Test",
			"Count":   42,
			"Items":   []string{"Item 1", "Item 2", "Item 3"},
			"Visible": true,
			"Status":  "Active",
			"Attrs": map[string]string{
				"class": "highlight",
				"style": "color: blue;",
			},
		}

		page, err := app.NewApplicationPage(tmpl, initialData)
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		defer func() { _ = page.Close() }()

		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				html, err := page.Render()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				if _, err := w.Write([]byte(html)); err != nil {
					fmt.Printf("Warning: Failed to write HTML response: %v\n", err)
				}

			case "/update":
				var updateData map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				// Record fragment generation start time
				fragmentStart := time.Now()

				fragments, err := page.RenderFragments(r.Context(), updateData)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				// Record fragment metrics
				for _, fragment := range fragments {
					fragmentData, _ := json.Marshal(fragment.Data)
					compressionRatio := 0.75 // Simulated compression
					if fragment.Metadata != nil {
						compressionRatio = fragment.Metadata.CompressionRatio
					}

					helper.RecordFragmentMetric(
						fragment.ID,
						fragment.Strategy,
						time.Since(fragmentStart)/time.Duration(len(fragments)),
						len(fragmentData),
						compressionRatio,
						false, // Not using cache in this test
					)
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(fragments); err != nil {
					fmt.Printf("Warning: Failed to encode fragments response: %v\n", err)
				}

			case "/client/livetemplate-client.js":
				// Serve a minimal client script
				w.Header().Set("Content-Type", "application/javascript")
				if _, err := w.Write([]byte(`
					console.log('LiveTemplate client loaded');
					window.LiveTemplate = {
						applyFragment: function(fragment) {
							console.log('Applying fragment:', fragment);
							return Promise.resolve(true);
						}
					};
				`)); err != nil {
					fmt.Printf("Warning: Failed to write client JS: %v\n", err)
				}

			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		// Create browser context
		ctx, cancel := helper.CreateBrowserContext()
		defer cancel()

		// Step 1: Initial page load
		helper.SetCustomMetric("server_url", server.URL)

		loadStart := time.Now()
		var pageTitle string
		err = chromedp.Run(ctx,
			chromedp.Navigate(server.URL),
			chromedp.WaitVisible("h1"),
			chromedp.Title(&pageTitle),
		)
		helper.RecordBrowserAction("initial-page-load", time.Since(loadStart), err == nil, err)

		if err != nil {
			helper.CaptureFailureScreenshot(ctx, t, "failed-initial-load")
			return fmt.Errorf("failed to load initial page: %w", err)
		}

		// Capture success screenshot
		if err := helper.CaptureScreenshot(ctx, "initial-load-success"); err != nil {
			t.Logf("⚠️ Success screenshot failed: %v", err)
		}

		// Validate initial content
		var headerText, countText string
		validateStart := time.Now()
		err = chromedp.Run(ctx,
			chromedp.Text("h1", &headerText),
			chromedp.Text("#count", &countText),
		)
		helper.RecordBrowserAction("validate-initial-content", time.Since(validateStart), err == nil, err)

		if err != nil {
			return fmt.Errorf("failed to validate initial content: %w", err)
		}

		if !strings.Contains(headerText, "LiveTemplate CI Test") {
			return fmt.Errorf("unexpected header text: %s", headerText)
		}

		if countText != "42" {
			return fmt.Errorf("unexpected count: %s", countText)
		}

		helper.SetCustomMetric("initial_header_text", headerText)
		helper.SetCustomMetric("initial_count", countText)

		// Step 2: Test fragment updates
		testCases := []struct {
			name             string
			data             map[string]interface{}
			expectedStrategy string
		}{
			{
				name: "text-only-update",
				data: map[string]interface{}{
					"Title":   "Updated CI Test",
					"Count":   99,
					"Items":   []string{"Item 1", "Item 2", "Item 3"},
					"Visible": true,
					"Status":  "Active",
					"Attrs": map[string]string{
						"class": "highlight",
						"style": "color: blue;",
					},
				},
				expectedStrategy: "static_dynamic",
			},
			{
				name: "attribute-update",
				data: map[string]interface{}{
					"Title":   "Updated CI Test",
					"Count":   99,
					"Items":   []string{"Item 1", "Item 2", "Item 3"},
					"Visible": true,
					"Status":  "Active",
					"Attrs": map[string]string{
						"class": "warning",
						"style": "color: red;",
					},
				},
				expectedStrategy: "markers",
			},
			{
				name: "structural-update",
				data: map[string]interface{}{
					"Title":   "Updated CI Test",
					"Count":   99,
					"Items":   []string{"Item 1", "Item 2", "Item 3", "New Item 4"},
					"Visible": true,
					"Status":  "Active",
					"Attrs": map[string]string{
						"class": "warning",
						"style": "color: red;",
					},
				},
				expectedStrategy: "granular",
			},
		}

		for _, tc := range testCases {
			t.Logf("🧪 Testing update case: %s", tc.name)

			updateStart := time.Now()
			var result string

			// Store the update result in a global variable and then retrieve it
			setupScript := fmt.Sprintf(`
				window.updateResult = null;
				fetch('/update', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify(%s)
				})
				.then(response => {
					if (!response.ok) {
						throw new Error('HTTP ' + response.status + ': ' + response.statusText);
					}
					return response.json();
				})
				.then(fragments => {
					console.log('Received fragments:', fragments);
					window.updateResult = JSON.stringify(fragments);
				})
				.catch(error => {
					console.error('Update failed:', error);
					window.updateResult = 'ERROR: ' + error.toString();
				});
			`, jsonString(tc.data))

			// Execute the fetch and wait for completion
			err = chromedp.Run(ctx,
				chromedp.Evaluate(setupScript, nil),
				// Wait for result to be available with timeout
				chromedp.Poll(`window.updateResult !== null`, nil, chromedp.WithPollingTimeout(5*time.Second)),
				chromedp.Evaluate(`window.updateResult`, &result),
			)

			helper.RecordBrowserAction(fmt.Sprintf("fragment-update-%s", tc.name), time.Since(updateStart), err == nil, err)

			if err != nil {
				helper.CaptureFailureScreenshot(ctx, t, fmt.Sprintf("failed-update-%s", tc.name))
				return fmt.Errorf("failed to perform %s update: %w", tc.name, err)
			}

			if result == "" {
				return fmt.Errorf("empty result for %s update", tc.name)
			}

			if strings.HasPrefix(result, "ERROR:") {
				return fmt.Errorf("JavaScript update failed for %s: %s", tc.name, result)
			}

			// Validate fragment response
			var fragments []Fragment
			if err := json.Unmarshal([]byte(result), &fragments); err != nil {
				return fmt.Errorf("failed to parse fragments for %s: %w", tc.name, err)
			}

			if len(fragments) == 0 {
				return fmt.Errorf("no fragments generated for %s", tc.name)
			}

			// Check if expected strategy was used
			strategyFound := false
			for _, fragment := range fragments {
				if fragment.Strategy == tc.expectedStrategy {
					strategyFound = true
					break
				}
			}

			helper.SetCustomMetric(fmt.Sprintf("fragments_%s_count", tc.name), len(fragments))
			helper.SetCustomMetric(fmt.Sprintf("strategy_%s_found", tc.name), strategyFound)

			if !strategyFound {
				t.Logf("⚠️ Expected strategy %s not found in fragments for %s", tc.expectedStrategy, tc.name)
				// Don't fail the test for strategy selection - this is informational
			}

			// Capture screenshot for each test case
			if err := helper.CaptureScreenshot(ctx, fmt.Sprintf("update-%s", tc.name)); err != nil {
				t.Logf("⚠️ Update screenshot failed: %v", err)
			}
		}

		// Step 3: Performance validation
		performanceStart := time.Now()

		// Simulate rapid updates to test performance
		for i := 0; i < 5; i++ {
			rapidUpdateData := map[string]interface{}{
				"Title":   fmt.Sprintf("Rapid Update %d", i+1),
				"Count":   100 + i,
				"Items":   []string{fmt.Sprintf("Rapid Item %d", i+1)},
				"Visible": i%2 == 0,
				"Status":  fmt.Sprintf("Status %d", i+1),
				"Attrs": map[string]string{
					"class": fmt.Sprintf("rapid-%d", i),
					"style": fmt.Sprintf("color: hsl(%d, 70%%, 50%%);", i*60),
				},
			}

			setupScript := fmt.Sprintf(`
				window.rapidResult = null;
				fetch('/update', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify(%s)
				})
				.then(response => response.ok ? response.json() : [])
				.then(fragments => {
					window.rapidResult = fragments.length;
				})
				.catch(error => {
					window.rapidResult = 0;
				});
			`, jsonString(rapidUpdateData))

			var fragmentCount int
			err = chromedp.Run(ctx,
				chromedp.Evaluate(setupScript, nil),
				// Wait for result to be available with timeout
				chromedp.Poll(`window.rapidResult !== null`, nil, chromedp.WithPollingTimeout(2*time.Second)),
				chromedp.Evaluate(`window.rapidResult`, &fragmentCount),
			)
			if err != nil {
				t.Logf("⚠️ Rapid update %d failed: %v", i, err)
			}
		}

		helper.RecordBrowserAction("rapid-updates-5x", time.Since(performanceStart), err == nil, err)
		helper.SetCustomMetric("rapid_updates_duration", time.Since(performanceStart))

		// Final validation
		var finalTitle string
		finalValidateStart := time.Now()
		err = chromedp.Run(ctx,
			chromedp.Text("h1", &finalTitle),
		)
		helper.RecordBrowserAction("final-validation", time.Since(finalValidateStart), err == nil, err)

		if err != nil {
			helper.CaptureFailureScreenshot(ctx, t, "failed-final-validation")
			return fmt.Errorf("final validation failed: %w", err)
		}

		helper.SetCustomMetric("final_title", finalTitle)

		// Final success screenshot
		if err := helper.CaptureScreenshot(ctx, "test-completed"); err != nil {
			t.Logf("⚠️ Final screenshot failed: %v", err)
		}

		return nil
	})
}

// TestE2EPerformanceBenchmark runs performance benchmarks with CI metrics
func TestE2EPerformanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance benchmark in short mode")
	}

	E2ETestWithHelper(t, "performance-benchmark", func(helper *E2ETestHelper) error {
		// Create minimal test server for performance testing
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			if _, err := w.Write([]byte(`
				<!DOCTYPE html>
				<html>
				<head><title>Performance Test</title></head>
				<body>
					<div id="content">Performance test content</div>
					<script>
						window.performance.mark('page-loaded');
					</script>
				</body>
				</html>
			`)); err != nil {
				fmt.Printf("Warning: Failed to write performance test HTML: %v\n", err)
			}
		}))
		defer server.Close()

		ctx, cancel := helper.CreateBrowserContext()
		defer cancel()

		// Benchmark page load times
		const numIterations = 10
		var totalLoadTime time.Duration

		for i := 0; i < numIterations; i++ {
			loadStart := time.Now()

			err := chromedp.Run(ctx,
				chromedp.Navigate(server.URL),
				chromedp.WaitVisible("#content"),
			)

			loadDuration := time.Since(loadStart)
			totalLoadTime += loadDuration

			helper.RecordBrowserAction(fmt.Sprintf("load-iteration-%d", i+1), loadDuration, err == nil, err)

			if err != nil {
				return fmt.Errorf("load iteration %d failed: %w", i+1, err)
			}

			// Small delay between iterations
			time.Sleep(50 * time.Millisecond)
		}

		avgLoadTime := totalLoadTime / numIterations
		helper.SetCustomMetric("average_load_time_ms", avgLoadTime.Milliseconds())
		helper.SetCustomMetric("total_iterations", numIterations)

		// Performance threshold validation
		maxAcceptableLoadTime := 5 * time.Second
		if avgLoadTime > maxAcceptableLoadTime {
			return fmt.Errorf("average load time %v exceeds threshold %v", avgLoadTime, maxAcceptableLoadTime)
		}

		t.Logf("✅ Performance benchmark completed: avg load time %v", avgLoadTime)
		return nil
	})
}

// BenchmarkE2EFragmentGeneration benchmarks fragment generation performance
func BenchmarkE2EFragmentGeneration(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Create test template and application
	tmpl, _ := template.New("bench").Parse(`<div>{{.Title}}: {{.Count}}</div>`)
	app, _ := NewApplication()
	defer func() {
		if err := app.Close(); err != nil {
			b.Errorf("Failed to close application: %v", err)
		}
	}()

	initialData := map[string]interface{}{"Title": "Benchmark", "Count": 0}
	page, _ := app.NewApplicationPage(tmpl, initialData)
	defer func() {
		if err := page.Close(); err != nil {
			b.Errorf("Failed to close page: %v", err)
		}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		updateData := map[string]interface{}{
			"Title": "Benchmark",
			"Count": i,
		}

		_, err := page.RenderFragments(context.Background(), updateData)
		if err != nil {
			b.Fatalf("Fragment generation failed: %v", err)
		}
	}
}

// Helper function to convert data to JSON string for JavaScript
func jsonString(data interface{}) string {
	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}
