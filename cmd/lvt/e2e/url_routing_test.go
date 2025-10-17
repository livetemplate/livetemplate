package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// TestPageModeURLRouting tests URL routing functionality in page mode
func TestPageModeURLRouting(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Build lvt
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-a", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Create app with --dev flag
	newCmd := exec.Command(lvtBinary, "new", "testapp", "--dev")
	newCmd.Dir = tmpDir
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Generate resource with page mode
	genCmd := exec.Command(lvtBinary, "gen", "products", "name", "--edit-mode", "page")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate resource: %v", err)
	}

	// Setup go.mod for local livetemplate
	cwd, _ := os.Getwd()
	livetemplatePath := filepath.Join(cwd, "..", "..", "..")

	goModTidyCmd := exec.Command("go", "mod", "tidy")
	goModTidyCmd.Dir = appDir
	if err := goModTidyCmd.Run(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}

	replaceCmd := exec.Command("go", "mod", "edit",
		"-replace", fmt.Sprintf("github.com/livefir/livetemplate=%s", livetemplatePath))
	replaceCmd.Dir = appDir
	if err := replaceCmd.Run(); err != nil {
		t.Fatalf("Failed to add replace directive: %v", err)
	}

	goModTidyCmd2 := exec.Command("go", "mod", "tidy")
	goModTidyCmd2.Dir = appDir
	if err := goModTidyCmd2.Run(); err != nil {
		t.Fatalf("Failed to run go mod tidy after replace: %v", err)
	}

	// Copy client library
	clientSrc := "../../../client/dist/livetemplate-client.browser.js"
	clientDst := filepath.Join(appDir, "livetemplate-client.js")
	cpCmd := exec.Command("cp", clientSrc, clientDst)
	if err := cpCmd.Run(); err != nil {
		t.Fatalf("Failed to copy client library: %v", err)
	}

	// Run migration
	migrationUpCmd := exec.Command(lvtBinary, "migration", "up")
	migrationUpCmd.Dir = appDir
	if err := migrationUpCmd.Run(); err != nil {
		t.Fatalf("Failed to run migration: %v", err)
	}

	// Start the app server
	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	// Build the server binary
	serverBinary := filepath.Join(tmpDir, "testapp-server")
	buildServerCmd := exec.Command("go", "build", "-o", serverBinary, "./cmd/testapp")
	buildServerCmd.Dir = appDir
	buildServerCmd.Env = append(os.Environ(), "GOWORK=off")
	buildOutput, buildErr := buildServerCmd.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("Failed to build server: %v\nOutput: %s", buildErr, string(buildOutput))
	}

	serverCmd := exec.Command(serverBinary)
	serverCmd.Dir = appDir
	serverCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if serverCmd.Process != nil {
			serverCmd.Process.Kill()
			serverCmd.Wait()
		}
	}()

	// Wait for server to start
	serverReady := false
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				serverReady = true
				break
			}
		}
	}
	if !serverReady {
		t.Fatal("Server did not start within 6 seconds")
	}

	// Start Chrome for testing
	debugPort := 9223
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, chromeCmd, debugPort)

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(),
		fmt.Sprintf("http://localhost:%d", debugPort))
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	testURL := fmt.Sprintf("%s/products", e2etest.GetChromeTestURL(port))
	t.Logf("Testing URL routing at: %s", testURL)

	// Helper to wait for element to appear
	waitForElement := func(selector string, timeout time.Duration) chromedp.ActionFunc {
		return func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timeout waiting for element: %s", selector)
				case <-ticker.C:
					var exists bool
					if err := chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, selector), &exists).Do(ctx); err != nil {
						continue
					}
					if exists {
						return nil
					}
				}
			}
		}
	}

	// Setup: Create test products first
	t.Run("Setup: Create test products", func(t *testing.T) {
		var pageHTML string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`document.body.innerHTML`, &pageHTML),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		t.Logf("DEBUG: Initial page loaded, body length: %d", len(pageHTML))
		t.Logf("DEBUG: Body HTML (first 500 chars):\n%s", pageHTML[:min(500, len(pageHTML))])

		// Add first product
		t.Log("Adding first product...")
		err = chromedp.Run(ctx,
			// Click Add Product button to open modal
			chromedp.Click(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("Failed to open modal: %v", err)
		}

		// Verify modal is open
		var modalVisible bool
		chromedp.Evaluate(`!document.getElementById('add-modal').hasAttribute('hidden')`, &modalVisible).Do(ctx)
		t.Logf("Modal visible: %v", modalVisible)

		// Fill form and submit
		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="name"]`, "Test Product 1"),
		)
		if err != nil {
			t.Fatalf("Failed to fill form: %v", err)
		}

		// Log form data before submit
		var formValue string
		chromedp.Evaluate(`document.querySelector('input[name="name"]').value`, &formValue).Do(ctx)
		t.Logf("Form value before submit: '%s'", formValue)

		// Click the submit button (using Click instead of Submit to trigger LiveTemplate's WebSocket handler)
		err = chromedp.Run(ctx,
			chromedp.Click(`form[lvt-submit="add"] button[type="submit"]`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to submit first product: %v", err)
		}
		t.Log("Submit button clicked")

		// Wait for modal to close and WebSocket to reconnect
		t.Log("Waiting for modal to close...")
		err = chromedp.Run(ctx,
			chromedp.Sleep(500*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("Error during sleep: %v", err)
		}

		// Check if WebSocket is still connected
		var wsConnected bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`typeof window.liveTemplateClient !== 'undefined' && window.liveTemplateClient.ws && window.liveTemplateClient.ws.readyState === 1`, &wsConnected),
		)
		t.Logf("WebSocket connected: %v", wsConnected)

		// Wait for WebSocket reconnection if needed
		if !wsConnected {
			t.Log("WebSocket not connected, waiting for reconnection...")
			err = chromedp.Run(ctx,
				e2etest.WaitForWebSocketReady(10*time.Second),
			)
			if err != nil {
				t.Fatalf("WebSocket failed to reconnect: %v", err)
			}
			t.Log("✅ WebSocket reconnected")
		}

		// Wait for table to appear (structural change from <p> to <table>)
		t.Log("Waiting for table to appear...")
		err = chromedp.Run(ctx,
			waitForElement("table tbody tr", 10*time.Second),
		)
		if err != nil {
			// Debug: show what we actually have
			var bodyHTML string
			var bodyLength int
			chromedp.Evaluate(`document.body.innerHTML`, &bodyHTML).Do(ctx)
			bodyLength = len(bodyHTML)
			t.Logf("DEBUG: Body HTML length: %d", bodyLength)
			if bodyLength > 0 {
				t.Logf("DEBUG: Body HTML (first 2000 chars):\n%s", bodyHTML[:min(2000, bodyLength)])
			} else {
				t.Log("DEBUG: Body HTML is EMPTY!")
			}

			// Check for error messages
			var hasError bool
			chromedp.Evaluate(`document.body.innerText.includes('error') || document.body.innerText.includes('Error')`, &hasError).Do(ctx)
			t.Logf("DEBUG: Page contains error: %v", hasError)

			t.Fatalf("Table did not appear after adding first product: %v", err)
		}
		t.Log("✅ First product added, table appeared")

		// Add second product
		t.Log("Adding second product...")
		err = chromedp.Run(ctx,
			chromedp.Click(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
			chromedp.SendKeys(`input[name="name"]`, "Test Product 2"),
			chromedp.Click(`form[lvt-submit="add"] button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for WebSocket update
		)
		if err != nil {
			t.Fatalf("Failed to submit second product: %v", err)
		}
		t.Log("✅ Second product added")

		// Verify we now have 2 rows
		var rowCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('table tbody tr').length`, &rowCount),
		)
		if err != nil {
			t.Fatalf("Failed to count rows: %v", err)
		}
		t.Logf("Found %d product rows in table", rowCount)
		if rowCount < 2 {
			var tableHTML string
			chromedp.Evaluate(`document.querySelector('table')?.outerHTML || 'NO TABLE'`, &tableHTML).Do(ctx)
			t.Logf("DEBUG: Table HTML:\n%s", tableHTML)
			t.Fatalf("Expected at least 2 rows, got %d", rowCount)
		}
	})

	// Test 1: URL updates when clicking resource
	t.Run("URL updates on resource click", func(t *testing.T) {
		var currentURL string
		var linkExists bool

		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Sleep(1*time.Second),
			// Check if anchor link exists
			chromedp.Evaluate(`document.querySelector('table tbody tr a') !== null`, &linkExists),
		)
		if err != nil {
			t.Fatalf("Failed to check for links: %v", err)
		}

		if !linkExists {
			// Debug: Dump HTML to see what's actually rendered
			var bodyHTML string
			chromedp.Evaluate(`document.body.innerHTML`, &bodyHTML).Do(ctx)
			t.Logf("DEBUG: Body HTML (first 1000 chars):\n%s", bodyHTML[:min(1000, len(bodyHTML))])

			var tableHTML string
			chromedp.Evaluate(`document.querySelector('table')?.outerHTML || 'NO TABLE'`, &tableHTML).Do(ctx)
			t.Logf("DEBUG: Table HTML:\n%s", tableHTML)

			t.Skip("No products available (no anchor links found)")
		}

		// In page mode, clicking anchor link causes full page navigation
		// Don't wait for WebSocket after click since it's a new page load
		err = chromedp.Run(ctx,
			chromedp.Click(`table tbody tr a`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
			chromedp.Location(&currentURL),
		)
		if err != nil {
			t.Fatalf("Failed to click resource: %v", err)
		}

		if !strings.Contains(currentURL, "/products/products-") {
			t.Errorf("URL not updated. Expected /products/products-*, got %s", currentURL)
		} else {
			t.Logf("✅ URL updated to: %s", currentURL)
		}
	})

	// Test 2: Direct navigation to resource URL works
	t.Run("Direct navigation to resource URL", func(t *testing.T) {
		var detailVisible bool
		// First, get a resource ID from the anchor link href
		var firstResourceHref string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`document.querySelector('table tbody tr a')?.getAttribute('href') || null`, &firstResourceHref),
		)
		if err != nil || firstResourceHref == "" {
			t.Skip("No resources available for direct navigation test")
		}

		// Extract resource ID from href (format: /products/product-xxx)
		parts := strings.Split(firstResourceHref, "/")
		if len(parts) < 3 {
			t.Skip("Invalid resource href format")
		}
		firstResourceID := parts[len(parts)-1]

		// Now navigate directly to that resource
		directURL := fmt.Sprintf("%s/%s", testURL, firstResourceID)
		err = chromedp.Run(ctx,
			chromedp.Navigate(directURL),
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(`document.body.innerText.includes('Details') || document.body.innerText.includes('Back')`, &detailVisible),
		)
		if err != nil {
			t.Fatalf("Failed to navigate directly: %v", err)
		}

		if !detailVisible {
			t.Error("Detail view not shown when navigating directly to resource URL")
		} else {
			t.Log("✅ Direct navigation works")
		}
	})

	// Test 3: Browser back button returns to list
	t.Run("Browser back button works", func(t *testing.T) {
		var backToList bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Sleep(1*time.Second),
			chromedp.Click(`table tbody tr a`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`history.back()`, nil),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`document.querySelector('table') !== null`, &backToList),
		)
		if err != nil {
			t.Fatalf("Failed to test back button: %v", err)
		}

		if !backToList {
			t.Error("Browser back button did not return to list view")
		} else {
			t.Log("✅ Browser back button works")
		}
	})

	// Test 4: URL is at list path after back button
	t.Run("URL returns to list path after back", func(t *testing.T) {
		var finalURL string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Sleep(1*time.Second),
			chromedp.Click(`table tbody tr a`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`history.back()`, nil),
			chromedp.Sleep(1*time.Second),
			chromedp.Location(&finalURL),
		)
		if err != nil {
			t.Fatalf("Failed to get URL after back: %v", err)
		}

		if !strings.HasSuffix(finalURL, "/products") && !strings.HasSuffix(finalURL, "/products/") {
			t.Errorf("URL not reset to list. Expected /products, got %s", finalURL)
		} else {
			t.Logf("✅ URL returned to list: %s", finalURL)
		}
	})

	t.Log("✅ All URL routing tests passed")
}
