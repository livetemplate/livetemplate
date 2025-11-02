package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestModalFunctionality tests all modal interactions end-to-end
// This test verifies the critical modal bug fix where modals wouldn't reopen after being closed
func TestModalFunctionality(t *testing.T) {
	// Note: Not parallel because it uses isolated Chrome container which needs sequential execution

	// Find the client file
	clientPath, err := filepath.Abs(filepath.Join("..", "..", "..", "client", "dist", "livetemplate-client.browser.js"))
	if err != nil {
		t.Fatalf("Failed to resolve client path: %v", err)
	}

	// Start a simple HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// For Chrome access (Docker networking)
	chromeURL := getTestURL(port)

	mux := http.NewServeMux()

	// Serve the client file
	mux.HandleFunc("/livetemplate-client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, clientPath)
	})

	// Create a test HTML page with modal
	testHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Modal Test</title>
</head>
<body>
    <div data-lvt-id="test-wrapper">
        <button id="open-btn" lvt-modal-open="add-modal">Add Product</button>

        <!-- Modal -->
        <div id="add-modal" hidden aria-hidden="true" role="dialog" data-modal-backdrop data-modal-id="add-modal"
             style="position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000;">
            <div style="background: white; border-radius: 8px; padding: 2rem; max-width: 600px; width: 90%;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
                    <h2>Add New Product</h2>
                    <button id="close-x" type="button" lvt-modal-close="add-modal"
                            style="background: none; border: none; font-size: 1.5rem; cursor: pointer;">&times;</button>
                </div>

                <form>
                    <div style="margin-bottom: 1rem;">
                        <label>Name</label>
                        <input type="text" name="name" placeholder="Enter name" required>
                    </div>
                    <div>
                        <button type="submit">Add Product</button>
                        <button id="cancel-btn" type="button" lvt-modal-close="add-modal">Cancel</button>
                    </div>
                </form>
            </div>
        </div>
    </div>

	<script>
		window.__lvtClientLoaded = false;
		window.__lvtClientLoadError = "";
		window.__markLvtClientLoaded = function () {
			window.__lvtClientLoaded = true;
		};
		window.__markLvtClientError = function (event) {
			if (event && event.message) {
				window.__lvtClientLoadError = event.message;
			} else if (event && event.type) {
				window.__lvtClientLoadError = event.type;
			} else {
				window.__lvtClientLoadError = "unknown error";
			}
		};
	</script>
	<script src="/livetemplate-client.js" defer onload="window.__markLvtClientLoaded()" onerror="window.__markLvtClientError(event)"></script>
</body>
</html>`

	// Serve the test HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(testHTML))
	})

	// Start server
	go func() { _ = http.Serve(listener, mux) }()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Collect console logs for debugging
	var consoleLogs []string
	var consoleLogsMutex sync.Mutex

	// Use isolated Chrome container for parallel execution
	ctx, cancel := getIsolatedChromeContext(t)
	defer cancel()

	// Enable Runtime domain and listen for console messages
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if e, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			consoleLogsMutex.Lock()
			for _, arg := range e.Args {
				consoleLogs = append(consoleLogs, fmt.Sprintf("[Console] %s", arg.Value))
			}
			consoleLogsMutex.Unlock()
		}
	})

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Run the tests
	err = chromedp.Run(ctx,
		// Enable Runtime to capture console logs
		chromedp.ActionFunc(func(ctx context.Context) error {
			return runtime.Enable().Do(ctx)
		}),

		// Navigate to test page
		chromedp.Navigate(chromeURL),
		chromedp.WaitReady("body"),
		// Wait for the client bundle to load or fail deterministically
		waitFor(`window.__lvtClientLoaded === true || window.__lvtClientLoadError !== ''`, 15*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var loadErr string
			if err := chromedp.Evaluate(`window.__lvtClientLoadError || ''`, &loadErr).Do(ctx); err != nil {
				return fmt.Errorf("failed to inspect client load state: %v", err)
			}
			if loadErr != "" {
				return fmt.Errorf("failed to load client bundle: %s", loadErr)
			}
			return nil
		}),
		// Force client auto-initialization if the script hasn't attached the instance yet
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`
				(() => {
					if (!window.liveTemplateClient && window.LiveTemplateClient?.LiveTemplateClient?.autoInit) {
						window.LiveTemplateClient.LiveTemplateClient.autoInit();
					}
				})();
			`, nil).Do(ctx)
		}),
		// Wait for client to fully initialize
		waitFor(`typeof window.liveTemplateClient !== 'undefined'`, 15*time.Second),

		// Test 1: Modal should be hidden initially
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hidden bool
			if err := chromedp.Evaluate(`document.getElementById('add-modal').hasAttribute('hidden')`, &hidden).Do(ctx); err != nil {
				return fmt.Errorf("failed to check hidden attribute: %v", err)
			}
			if !hidden {
				return fmt.Errorf("modal should be hidden initially")
			}
			t.Log("✓ Test 1: Modal is hidden initially")
			return nil
		}),

		// Test 1.5: Check if client loaded
		chromedp.ActionFunc(func(ctx context.Context) error {
			var clientLoaded bool
			if err := chromedp.Evaluate(`typeof window.liveTemplateClient !== 'undefined'`, &clientLoaded).Do(ctx); err != nil {
				return fmt.Errorf("failed to check client: %v", err)
			}
			if !clientLoaded {
				return fmt.Errorf("liveTemplate client not loaded")
			}
			t.Log("✓ Client loaded successfully")
			return nil
		}),

		// Test 2: Click button to open modal (simulate click via JavaScript for reliability)
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Simulate click via JavaScript to ensure it triggers
			if err := chromedp.Evaluate(`document.getElementById('open-btn').click()`, nil).Do(ctx); err != nil {
				return fmt.Errorf("failed to click open button: %v", err)
			}
			t.Log("✓ Clicked open button")
			return nil
		}),
		// Wait for modal to open
		waitFor("document.getElementById('add-modal').style.display === 'flex'", 3*time.Second),

		// Test 3: Verify modal is visible and centered (display: flex)
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Get display style
			var display string
			if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
				return fmt.Errorf("failed to get display style: %v", err)
			}

			if display != "flex" {
				// Log more details for debugging
				var hidden bool
				_ = chromedp.Evaluate(`document.getElementById('add-modal').hasAttribute('hidden')`, &hidden).Do(ctx)
				return fmt.Errorf("modal should have display: flex, got: %s (hidden=%v)", display, hidden)
			}

			// Check hidden attribute is removed
			var result bool
			if err := chromedp.Evaluate(`document.getElementById('add-modal').hasAttribute('hidden')`, &result).Do(ctx); err != nil {
				return fmt.Errorf("failed to check hidden attribute: %v", err)
			}
			if result {
				return fmt.Errorf("modal should not have hidden attribute")
			}

			t.Log("✓ Test 2 & 3: Modal opens and is centered (display: flex)")
			return nil
		}),

		// Test 4: Close modal by clicking the X button using real browser click
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("✓ Attempting to click close button...")
			// Check if button is visible and clickable
			var visible bool
			if err := chromedp.Evaluate(`
				var btn = document.getElementById('close-x');
				var rect = btn.getBoundingClientRect();
				rect.width > 0 && rect.height > 0
			`, &visible).Do(ctx); err != nil {
				return fmt.Errorf("failed to check visibility: %v", err)
			}
			t.Logf("✓ Close button visible: %v", visible)

			if err := chromedp.Click("#close-x", chromedp.ByQuery).Do(ctx); err != nil {
				return fmt.Errorf("failed to click close button: %v", err)
			}
			t.Log("✓ Clicked close button successfully")
			return nil
		}),
		// Wait for modal to close
		waitFor("document.getElementById('add-modal').style.display === 'none'", 3*time.Second),

		// Test 5: Verify modal is hidden after close
		chromedp.ActionFunc(func(ctx context.Context) error {
			var display string
			if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
				return fmt.Errorf("failed to get display style: %v", err)
			}
			if display != "none" {
				return fmt.Errorf("modal should have display: none after close, got: %s", display)
			}

			var hidden bool
			if err := chromedp.Evaluate(`document.getElementById('add-modal').hasAttribute('hidden')`, &hidden).Do(ctx); err != nil {
				return fmt.Errorf("failed to check hidden attribute: %v", err)
			}
			if !hidden {
				return fmt.Errorf("modal should have hidden attribute after close")
			}

			t.Log("✓ Test 4 & 5: Modal closes with X button")
			return nil
		}),

		// Test 6: Reopen modal (critical test - was broken before)
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.Evaluate(`document.getElementById('open-btn').click()`, nil).Do(ctx); err != nil {
				return fmt.Errorf("failed to reopen modal: %v", err)
			}
			return nil
		}),
		// Wait for modal to reopen
		waitFor("document.getElementById('add-modal').style.display === 'flex'", 3*time.Second),

		// Test 7: Verify modal reopened successfully
		chromedp.ActionFunc(func(ctx context.Context) error {
			var display string
			if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
				return fmt.Errorf("failed to get display style: %v", err)
			}
			if display != "flex" {
				return fmt.Errorf("modal should reopen with display: flex, got: %s", display)
			}

			var hidden bool
			if err := chromedp.Evaluate(`document.getElementById('add-modal').hasAttribute('hidden')`, &hidden).Do(ctx); err != nil {
				return fmt.Errorf("failed to check hidden attribute: %v", err)
			}
			if hidden {
				return fmt.Errorf("modal should not have hidden attribute after reopen")
			}

			t.Log("✓ Test 6 & 7: Modal REOPENS successfully (critical fix)")
			return nil
		}),

		// Test 8: Close modal by clicking Cancel button using real browser click
		chromedp.Click("#cancel-btn", chromedp.ByQuery),
		// Wait for modal to close
		waitFor("document.getElementById('add-modal').style.display === 'none'", 3*time.Second),

		// Test 9: Verify modal closed with cancel
		chromedp.ActionFunc(func(ctx context.Context) error {
			var display string
			if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
				return fmt.Errorf("failed to get display style: %v", err)
			}
			if display != "none" {
				return fmt.Errorf("modal should close with cancel button")
			}
			t.Log("✓ Test 8 & 9: Modal closes with Cancel button")
			return nil
		}),

		// Test 10: Open modal again
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.Evaluate(`document.getElementById('open-btn').click()`, nil).Do(ctx); err != nil {
				return fmt.Errorf("failed to open modal for escape test: %v", err)
			}
			return nil
		}),
		// Wait for modal to open
		waitFor("document.getElementById('add-modal').style.display === 'flex'", 3*time.Second),

		// Test 11: Close with Escape key
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Send Escape key
			if err := chromedp.KeyEvent("\x1b").Do(ctx); err != nil {
				return fmt.Errorf("failed to send Escape key: %v", err)
			}
			return nil
		}),
		// Wait for modal to close
		waitFor("document.getElementById('add-modal').style.display === 'none'", 3*time.Second),

		// Test 12: Verify modal closed with Escape key
		chromedp.ActionFunc(func(ctx context.Context) error {
			var display string
			if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
				return fmt.Errorf("failed to get display style: %v", err)
			}
			if display != "none" {
				return fmt.Errorf("modal should close with Escape key")
			}
			t.Log("✓ Test 11 & 12: Modal closes with Escape key")
			return nil
		}),

		// Test 13: Multiple open/close cycles with actual button clicks
		chromedp.ActionFunc(func(ctx context.Context) error {
			t.Log("Testing multiple open/close cycles with real browser clicks...")
			for i := 1; i <= 3; i++ {
				// Open
				if err := chromedp.Click("#open-btn", chromedp.ByQuery).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: failed to open modal: %v", i, err)
				}
				// Wait for modal to open
				if err := waitFor("document.getElementById('add-modal').style.display === 'flex'", 3*time.Second).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: modal failed to open: %v", i, err)
				}

				// Verify opened
				var display string
				if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: failed to check display: %v", i, err)
				}
				if display != "flex" {
					return fmt.Errorf("cycle %d: modal should be open (display: flex), got: %s", i, display)
				}

				// Close by clicking X button with real browser click
				if err := chromedp.Click("#close-x", chromedp.ByQuery).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: failed to click close button: %v", i, err)
				}
				// Wait for modal to close
				if err := waitFor("document.getElementById('add-modal').style.display === 'none'", 3*time.Second).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: modal failed to close: %v", i, err)
				}

				// Verify closed
				if err := chromedp.Evaluate(`document.getElementById('add-modal').style.display`, &display).Do(ctx); err != nil {
					return fmt.Errorf("cycle %d: failed to check display: %v", i, err)
				}
				if display != "none" {
					return fmt.Errorf("cycle %d: modal should be closed (display: none), got: %s", i, display)
				}

				t.Logf("✓ Cycle %d: Open and close successful", i)
			}
			t.Log("✓ Test 13: Multiple open/close cycles work correctly")
			return nil
		}),
	)

	if err != nil {
		// Print console logs for debugging
		consoleLogsMutex.Lock()
		if len(consoleLogs) > 0 {
			t.Log("\n📋 Console Logs:")
			for _, log := range consoleLogs {
				t.Log("  " + log)
			}
		}
		consoleLogsMutex.Unlock()

		t.Fatalf("Browser automation failed: %v", err)
	}

	t.Log("\n✅ ALL MODAL TESTS PASSED!")
	t.Log("   ✓ Modal opens centered (display: flex)")
	t.Log("   ✓ Modal closes with X button")
	t.Log("   ✓ Modal closes with Cancel button")
	t.Log("   ✓ Modal closes with Escape key")
	t.Log("   ✓ Modal can reopen after closing (CRITICAL FIX)")
	t.Log("   ✓ Multiple open/close cycles work")

	// Print console logs even on success for debugging
	consoleLogsMutex.Lock()
	if len(consoleLogs) > 0 {
		t.Log("\n📋 Console Logs:")
		for _, log := range consoleLogs {
			t.Log("  " + log)
		}
	}
	consoleLogsMutex.Unlock()
}
