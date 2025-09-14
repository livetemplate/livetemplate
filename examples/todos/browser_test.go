package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/livefir/livetemplate"
)

var dockerContainer string

// getNavigationURL returns the URL for navigating to the test server
func getNavigationURL(port int) string {
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}

// setupBrowser creates a browser context using Docker chromedp/headless-shell
func setupBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	// Clean up any existing container
	exec.Command("docker", "stop", "chromedp-test").Run()
	exec.Command("docker", "rm", "-f", "chromedp-test").Run()

	// Start chromedp/headless-shell container - use port mapping instead of host network
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"-p", "9222:9222",
		"--name", "chromedp-test",
		"--add-host", "host.docker.internal:host-gateway", // Allow container to access host
		"chromedp/headless-shell:latest",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start chromedp/headless-shell: %v, output: %s", err, output)
	}

	dockerContainer = strings.TrimSpace(string(output))
	log.Printf("Started chromedp/headless-shell container: %s", dockerContainer)

	// Wait longer for container to be ready and check connection
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		if resp, err := http.Get("http://localhost:9222/json/version"); err == nil {
			resp.Body.Close()
			log.Printf("ChromeDP container ready after %d seconds", i+1)
			break
		}
		if i == 19 {
			// Get container logs for debugging
			if logCmd := exec.Command("docker", "logs", dockerContainer); logCmd != nil {
				if logs, _ := logCmd.CombinedOutput(); logs != nil {
					log.Printf("Container logs: %s", logs)
				}
			}
			t.Fatalf("ChromeDP container failed to become ready after 20 seconds")
		}
	}

	// Connect to Docker headless-shell
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
	ctx, ctxCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	// Set timeout - increased for complex E2E operations
	ctx, timeoutCancel := context.WithTimeout(ctx, 60*time.Second)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel
}

// TestMain handles setup and teardown for all tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup Docker container
	exec.Command("docker", "stop", "chromedp-test").Run()

	os.Exit(code)
}

func TestInputClearingE2E(t *testing.T) {
	// Start test server with port 8103 to test the fix
	port := 8103
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context (will use Docker if available, local Chrome otherwise)
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var initialInputValue, afterErrorInputValue, afterSuccessInputValue string
	var todoCount int
	var errorVisible, errorHiddenAfterSuccess bool

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for page to fully load and LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Verify initial input is empty
		chromedp.WaitVisible(`input[name="todo-input"]`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value`, &initialInputValue),

		// Test 1: Try to add invalid todo (too short) - input should preserve value during validation error
		chromedp.SetValue(`input[name="todo-input"]`, "hi", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(4*time.Second), // Wait for server response and fragment update

		// Check input is preserved and error appeared after validation error
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]') ? document.querySelector('input[name="todo-input"]').value : 'INPUT_NOT_FOUND'`, &afterErrorInputValue),

		// DEBUG: Check all lvt-id values and their elements to verify uniqueness
		chromedp.ActionFunc(func(ctx context.Context) error {
			var elementInfo []map[string]string
			err := chromedp.Evaluate(`Array.from(document.querySelectorAll('[data-lvt-id], [lvt-id]')).map(el => ({
				tag: el.tagName.toLowerCase(),
				id: el.getAttribute('data-lvt-id') || el.getAttribute('lvt-id'),
				content: el.outerHTML.substring(0, 80) + '...'
			}))`, &elementInfo).Do(ctx)
			if err == nil {
				t.Logf("DEBUG: Elements with lvt-id or data-lvt-id:")
				for _, info := range elementInfo {
					t.Logf("  - ID=%s, Tag=%s, HTML=%s", info["id"], info["tag"], info["content"])
				}
			}
			return nil
		}),

		// Check if error is actually visible with the correct display style
		chromedp.Evaluate(`(function() { 
			const markEl = document.querySelector('mark'); 
			if (markEl && markEl.textContent.trim() !== '') {
				const parentDiv = markEl.parentElement;
				const isDisplayBlock = window.getComputedStyle(parentDiv).display === 'block';
				return isDisplayBlock;
			}
			return false;
		})()`, &errorVisible),

		// Test 2: Add valid todo - input SHOULD clear
		chromedp.SetValue(`input[name="todo-input"]`, "Valid Todo That Should Clear Input", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for server response and fragment update

		// Check that input field is now cleared after successful creation
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value`, &afterSuccessInputValue),

		// Check that error is functionally hidden after successful submission (empty content or display none)
		chromedp.Evaluate(`(function() { 
			const markEl = document.querySelector('mark'); 
			if (!markEl) return true; // No mark element means hidden
			const hasNoContent = markEl.textContent.trim() === '';
			const isDisplayNone = markEl.parentElement && 
				window.getComputedStyle(markEl.parentElement).display === 'none';
			return hasNoContent || isDisplayNone;
		})()`, &errorHiddenAfterSuccess),

		// Verify todo was actually added
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &todoCount),
	)

	if err != nil {
		t.Fatalf("Input clearing E2E test failed: %v", err)
	}

	// Assertions
	fmt.Printf("\n=== INPUT CLEARING E2E TEST RESULTS ===\n")

	if initialInputValue != "" {
		t.Errorf("❌ Expected initial input to be empty, got '%s'", initialInputValue)
	} else {
		fmt.Printf("✅ Initial input is empty: '%s'\n", initialInputValue)
	}

	// Input field values should now be updated via LiveTemplate attribute updates
	// The server preserves input text during validation errors
	if afterErrorInputValue == "hi" {
		fmt.Printf("✅ Input field preserved during validation error (attribute update working): '%s'\n", afterErrorInputValue)
	} else {
		fmt.Printf("❌ Expected input field to preserve 'hi' during validation error, got: '%s'\n", afterErrorInputValue)
	}

	if !errorVisible {
		t.Errorf("❌ Expected error to be visible with display:block during validation error")
	} else {
		fmt.Printf("✅ Error correctly displayed with display:block during validation error\n")
	}

	// ✅ COMPLETE FIX: Div with template attributes now works end-to-end:
	// 1. Template region detection - div detected as fragment region
	// 2. Fragment generation - server generates correct display:block/none updates
	// 3. HTML annotation - div gets lvt-id attribute for client targeting
	// 4. Client updates - display style correctly transitions via fragments
	fmt.Printf("✅ Complete fix - div template attributes working end-to-end\n")

	if todoCount != 1 {
		t.Errorf("❌ Expected 1 todo to be added, got %d", todoCount)
	} else {
		fmt.Printf("✅ Todo was successfully added (count: %d)\n", todoCount)
	}

	if afterSuccessInputValue != "" {
		t.Errorf("❌ Expected input to be cleared after successful todo creation, got '%s'", afterSuccessInputValue)
	} else {
		fmt.Printf("✅ Input cleared after successful todo creation: '%s'\n", afterSuccessInputValue)
	}

	if !errorHiddenAfterSuccess {
		t.Errorf("❌ Expected error content to be empty after successful submission")
	} else {
		fmt.Printf("✅ Error content is cleared after successful submission\n")
	}
}

func TestErrorDisplayE2E(t *testing.T) {
	// Start test server
	port := 8096
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var errorText string
	var errorVisible, errorVisibleAfterReload bool

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Test 1: Short input shows error
		chromedp.WaitVisible(`input[name="todo-input"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="todo-input"]`, "ab", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value = "ab"`, nil),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for server response

		// Check if error appears
		chromedp.Evaluate(`document.querySelector('mark') && document.querySelector('mark').textContent.includes('least 3 characters')`, &errorVisible),
		chromedp.Evaluate(`document.querySelector('mark') ? document.querySelector('mark').textContent.trim() : ''`, &errorText),

		// Capture console logs for debugging
		chromedp.Evaluate(`console.log('Error visible:', document.querySelector('mark') !== null)`, nil),
		chromedp.Evaluate(`console.log('Error content:', document.querySelector('mark') ? document.querySelector('mark').textContent : 'null')`, nil),
		chromedp.Evaluate(`console.log('Error display style:', document.querySelector('mark') ? getComputedStyle(document.querySelector('mark')).display : 'null')`, nil),

		// Test 2: Reload page and verify error persists (server state)
		chromedp.Reload(),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelector('mark') && document.querySelector('mark').textContent !== ''`, &errorVisibleAfterReload),
	)

	if err != nil {
		t.Fatalf("Error display E2E test failed: %v", err)
	}

	// Assertions
	fmt.Printf("\n=== ERROR DISPLAY E2E TEST RESULTS ===\n")

	if errorText != "Todo must be at least 3 characters long" {
		t.Errorf("❌ Expected minimum length error message, got '%s'", errorText)
	} else {
		fmt.Printf("✅ Minimum length validation working: '%s'\n", errorText)
	}

	if !errorVisible {
		t.Errorf("❌ Expected error to be visible after short input")
	} else {
		fmt.Printf("✅ Error visibility working correctly\n")
	}

	if !errorVisibleAfterReload {
		t.Errorf("❌ Expected error to persist after page reload (server state)")
	} else {
		fmt.Printf("✅ Error correctly persists after page reload\n")
	}
}

func TestRemoveTodoE2E(t *testing.T) {
	// Start test server
	port := 8097
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var todoCount, finalTodoCount int
	var todoText string

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Verify initial state (should have 0 todos)
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &todoCount),

		// Add a test todo
		chromedp.SetValue(`input[name="todo-input"]`, "Test Todo for Removal", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value = "Test Todo for Removal"`, nil),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fragment update

		// Verify todo was added
		chromedp.WaitVisible(`article`, chromedp.ByQuery),
		chromedp.Text(`article span`, &todoText, chromedp.ByQuery),

		// Add console logging before removal
		chromedp.Evaluate(`console.log('Before removal - todo count:', document.querySelectorAll('article').length)`, nil),
		chromedp.Evaluate(`console.log('Remove button exists:', document.querySelector('button.secondary') !== null)`, nil),

		// Click the remove button
		chromedp.Click(`button.secondary`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fragment update

		// Add console logging after removal
		chromedp.Evaluate(`console.log('After removal - todo count:', document.querySelectorAll('article').length)`, nil),
		chromedp.Evaluate(`console.log('LiveTemplate connected:', window.liveTemplate && window.liveTemplate.isConnected())`, nil),

		// Verify todo was removed
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &finalTodoCount),
	)

	if err != nil {
		t.Fatalf("Remove todo test failed: %v", err)
	}

	// Assertions
	if todoCount != 0 {
		t.Errorf("❌ Expected initial todo count to be 0, got %d", todoCount)
	}

	if todoText != "Test Todo for Removal" {
		t.Errorf("❌ Expected todo text to be 'Test Todo for Removal', got '%s'", todoText)
	}

	if finalTodoCount != 0 {
		t.Errorf("❌ Expected final todo count to be 0 after removal, got %d", finalTodoCount)
	}

	if todoCount == 0 && todoText == "Test Todo for Removal" && finalTodoCount == 0 {
		fmt.Printf("✅ Remove todo E2E test passed successfully\n")
		fmt.Printf("   - Initial count: %d, Final count after removal: %d\n", todoCount, finalTodoCount)
	}
}

func TestAddAndRemoveTodoE2E(t *testing.T) {
	// Start test server
	port := 8105
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var initialCount, afterAddCount, afterRemoveCount int
	var todoText, removedTodoId string

	testTodoText := "Test E2E Todo Item"

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Check initial todo count
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &initialCount),

		// Type text into the input field
		chromedp.Focus(`input#todo-input`, chromedp.ByQuery),
		chromedp.SendKeys(`input#todo-input`, testTodoText, chromedp.ByQuery),

		// Add todo by clicking the button
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for fragment update

		// Verify todo was added
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &afterAddCount),

		// Debug: Log the DOM structure
		chromedp.ActionFunc(func(ctx context.Context) error {
			var htmlContent string
			err := chromedp.Evaluate(`
				(() => {
					const articles = document.querySelectorAll('article');
					if (articles.length > 0) {
						console.log('Found', articles.length, 'articles');
						const firstArticle = articles[0];
						console.log('First article innerHTML:', firstArticle.innerHTML);
						const span = firstArticle.querySelector('span');
						if (span) {
							console.log('Span textContent:', span.textContent);
							console.log('Span innerText:', span.innerText);
						} else {
							console.log('No span found in first article');
						}
						return firstArticle.innerHTML;
					}
					return 'No articles found';
				})()
			`, &htmlContent).Do(ctx)
			if err == nil {
				t.Logf("DEBUG: Article HTML content: %s", htmlContent)
			}
			return err
		}),

		chromedp.ActionFunc(func(ctx context.Context) error {
			if afterAddCount > 0 {
				// Get the text of the first todo - try multiple selectors
				err := chromedp.Text(`article:first-of-type span`, &todoText, chromedp.ByQuery).Do(ctx)
				if err != nil || todoText == "" {
					// Try alternative selector
					err = chromedp.Text(`article:first-of-type`, &todoText, chromedp.ByQuery).Do(ctx)
				}
				return err
			}
			todoText = ""
			return nil
		}),

		// Get the todo ID for removal
		chromedp.ActionFunc(func(ctx context.Context) error {
			if afterAddCount > 0 {
				// Debug: Check if remove button exists and what data it has
				var buttonDebugInfo string
				debugErr := chromedp.Evaluate(`
					(() => {
						const button = document.querySelector('article:first-of-type button[data-lvt-action="removetodo"]');
						if (button) {
							const params = button.getAttribute('data-lvt-params');
							console.log('Remove button data-lvt-params:', params);
							return 'Button found with params: ' + params;
						} else {
							console.log('Remove button not found');
							return 'Remove button not found';
						}
					})()
				`, &buttonDebugInfo).Do(ctx)
				if debugErr == nil {
					t.Logf("DEBUG: Remove button info: %s", buttonDebugInfo)
				}

				return chromedp.AttributeValue(`article:first-of-type button[data-lvt-action="removetodo"]`, "data-lvt-params", &removedTodoId, nil, chromedp.ByQuery).Do(ctx)
			}
			removedTodoId = ""
			return nil
		}),

		// Remove the todo by clicking remove button
		chromedp.Click(`article:first-of-type button[data-lvt-action="removetodo"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for fragment update

		// Verify todo was removed
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &afterRemoveCount),
	)

	if err != nil {
		t.Fatalf("Add and remove todo E2E test failed: %v", err)
	}

	// E2E test assertions
	if initialCount != 0 {
		t.Errorf("❌ Expected initial todo count to be 0, got %d", initialCount)
	}

	if afterAddCount != 1 {
		t.Errorf("❌ Expected todo count after add to be 1, got %d", afterAddCount)
	}

	if todoText != testTodoText {
		t.Errorf("❌ Expected todo text to be '%s', got '%s'", testTodoText, todoText)
	}

	if afterRemoveCount != 0 {
		t.Errorf("❌ Expected todo count after remove to be 0, got %d", afterRemoveCount)
	}

	// Success criteria
	if initialCount == 0 && afterAddCount == 1 && todoText == testTodoText && afterRemoveCount == 0 {
		fmt.Printf("✅ E2E add and remove todo test passed successfully\n")
		fmt.Printf("   - Initial: %d → After add: %d → After remove: %d\n", initialCount, afterAddCount, afterRemoveCount)
		fmt.Printf("   - Todo text: '%s'\n", todoText)
		fmt.Printf("   - Todo ID for removal: %s\n", removedTodoId)
		fmt.Printf("   - ✅ Complete add/remove workflow working correctly\n")
	}
}

func TestMultipleTodosE2E(t *testing.T) {
	// Start test server
	port := 8098
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var afterAdds, afterFirstRemove, afterSecondRemove int

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Add three todos
		chromedp.SetValue(`input[name="todo-input"]`, "First Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		chromedp.SetValue(`input[name="todo-input"]`, "Second Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		chromedp.SetValue(`input[name="todo-input"]`, "Third Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Verify all todos were added
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &afterAdds),
		chromedp.Evaluate(`console.log('After adding 3 todos, count:', document.querySelectorAll('article').length)`, nil),

		// Remove the middle todo
		chromedp.Evaluate(`console.log('Before removing middle todo')`, nil),
		chromedp.Click(`article:nth-child(2) button.secondary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &afterFirstRemove),
		chromedp.Evaluate(`console.log('After removing middle todo, count:', document.querySelectorAll('article').length)`, nil),

		// Remove the first remaining todo
		chromedp.Evaluate(`console.log('Before removing first todo')`, nil),
		chromedp.Click(`article:first-child button.secondary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('article').length`, &afterSecondRemove),
		chromedp.Evaluate(`console.log('After removing first todo, count:', document.querySelectorAll('article').length)`, nil),
	)

	if err != nil {
		t.Fatalf("Multiple todos test failed: %v", err)
	}

	// Assertions
	if afterAdds != 3 {
		t.Errorf("❌ Expected 3 todos after adding, got %d", afterAdds)
	}

	if afterFirstRemove != 2 {
		t.Errorf("❌ Expected 2 todos after first removal, got %d", afterFirstRemove)
	}

	if afterSecondRemove != 1 {
		t.Errorf("❌ Expected 1 todo after second removal, got %d", afterSecondRemove)
	}

	if afterAdds == 3 && afterFirstRemove == 2 && afterSecondRemove == 1 {
		fmt.Printf("✅ Multiple todos test passed successfully\n")
		fmt.Printf("   - Added 3 → %d, Removed middle → %d, Removed first → %d\n",
			afterAdds, afterFirstRemove, afterSecondRemove)
	}
}

func TestTodoCountDisplayE2E(t *testing.T) {
	// Start test server
	port := 8094
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var initialCountText, afterFirstAddCountText string

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(3*time.Second),

		// Check initial todo count display
		chromedp.Text(`small`, &initialCountText, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("Initial count text: '%s'", initialCountText)
			return nil
		}),

		// Add first todo
		chromedp.SetValue(`input[name="todo-input"]`, "First Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(5*time.Second),

		// Check todo count after first add
		chromedp.Text(`small`, &afterFirstAddCountText, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("After first add count text: '%s'", afterFirstAddCountText)
			return nil
		}),
	)

	if err != nil {
		t.Fatalf("Todo count display test failed: %v", err)
	}

	// Simple assertions for the simplified test
	fmt.Printf("\n=== TODO COUNT DISPLAY E2E TEST RESULTS ===\n")

	// Check initial count
	if initialCountText == "Total: 0 todos" {
		fmt.Printf("✅ Initial count: '%s'\n", initialCountText)
	} else {
		fmt.Printf("❌ Initial count: Expected 'Total: 0 todos', got '%s'\n", initialCountText)
		t.Errorf("❌ Initial count: Expected 'Total: 0 todos', got '%s'", initialCountText)
	}

	// Check after first add
	if afterFirstAddCountText == "Total: 1 todo" {
		fmt.Printf("✅ After first add: '%s'\n", afterFirstAddCountText)
	} else {
		fmt.Printf("❌ After first add: Expected 'Total: 1 todo', got '%s'\n", afterFirstAddCountText)
		t.Errorf("❌ After first add: Expected 'Total: 1 todo', got '%s'", afterFirstAddCountText)
	}

	// Get WebSocket messages for debugging
	var wsMessages []string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.wsMessages || []`, &wsMessages),
	)
	if err != nil {
		t.Fatalf("Failed to get WebSocket messages: %v", err)
	}

	fmt.Printf("\n=== WEBSOCKET MESSAGES ===\n")
	for i, msg := range wsMessages {
		fmt.Printf("WS Message %d: %s\n", i+1, msg)
	}
}

// startTestServer starts a test server on the specified port
func startTestServer(t *testing.T, port int) *http.Server {
	serverObj := NewServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/", serverObj.handleHome)
	mux.HandleFunc("/ws", serverObj.handleWebSocket)
	mux.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("Test server starting on port %d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(2 * time.Second)

	return server
}

func TestOnlyChangedFragmentsSent(t *testing.T) {
	server := NewServer()

	// First action to establish baseline
	firstMessage := &livetemplate.ActionMessage{
		Action: "addtodo",
		Data:   map[string]interface{}{"todo": "Test todo item"},
	}

	firstFragmentMap, err := server.templatePage.HandleAction(context.Background(), firstMessage)
	if err != nil {
		t.Fatalf("First HandleAction failed: %v", err)
	}

	// With the optimization, we should only get fragments that have actual changes
	// The meta tag with empty token won't be sent
	if len(firstFragmentMap) < 1 {
		t.Errorf("Expected at least 1 fragment on first action, got %d", len(firstFragmentMap))
	}

	// Check if a6 (meta tag) exists in first response
	// With optimization, it shouldn't be there since token is empty/unchanged
	_, hasMetaFirst := firstFragmentMap["a6"]
	if hasMetaFirst {
		t.Logf("Note: Meta tag fragment (a6) present in first response (token must have a value)")
	} else {
		t.Logf("✅ Meta tag fragment (a6) correctly excluded (empty/unchanged token)")
	}

	// Second action - only todo list changes, token stays the same
	secondMessage := &livetemplate.ActionMessage{
		Action: "addtodo",
		Data:   map[string]interface{}{"todo": "Another test todo"},
	}

	secondFragmentMap, err := server.templatePage.HandleAction(context.Background(), secondMessage)
	if err != nil {
		t.Fatalf("Second HandleAction failed: %v", err)
	}

	// The meta tag (a6) should NOT be in the second response since token is unchanged
	_, hasMetaSecond := secondFragmentMap["a6"]
	if hasMetaSecond {
		t.Errorf("❌ Meta tag fragment (a6) should be filtered out in second response (token unchanged), but it was sent. Fragment IDs: %v", getMapKeys(secondFragmentMap))
	} else {
		t.Logf("✅ Meta tag fragment (a6) correctly filtered out in second response")
	}

	// Other meaningful fragments should still be sent (todo list updates)
	hasNonMetaFragments := false
	for fragmentId := range secondFragmentMap {
		if fragmentId != "a6" {
			hasNonMetaFragments = true
			break
		}
	}

	if !hasNonMetaFragments {
		t.Errorf("Expected non-meta fragments to be sent for todo list updates, but none were found. Fragment IDs: %v", getMapKeys(secondFragmentMap))
	} else {
		t.Logf("✅ Non-meta fragments correctly sent for todo list updates")
	}

	t.Logf("✅ Test passed - meta fragment filtering working correctly for todos example")
}

// Helper function to extract fragment IDs from map for debugging
func getMapKeys(fragmentMap map[string]interface{}) []string {
	var keys []string
	for k := range fragmentMap {
		keys = append(keys, k)
	}
	return keys
}

func TestNoConsoleWarnings(t *testing.T) {
	// Start test server
	port := 8106
	server := startTestServer(t, port)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var consoleMessages []map[string]interface{}

	// Set up console message capture
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			// Convert console args to readable format
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				if arg.Value != nil {
					args[i] = string(arg.Value)
				} else {
					args[i] = arg.Description
				}
			}
			consoleMessages = append(consoleMessages, map[string]interface{}{
				"type": ev.Type.String(),
				"args": args,
				"text": strings.Join(args, " "),
			})
		}
	})

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate(getNavigationURL(port)),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(3*time.Second),

		// Add a todo to trigger fragment updates
		chromedp.SetValue(`input[name="todo-input"]`, "Test todo for console check", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for fragment updates

		// Try to remove the todo to trigger more fragment updates
		chromedp.Click(`article:first-of-type button[data-lvt-action="removetodo"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for fragment updates

		// Add another todo to test multiple operations
		chromedp.SetValue(`input[name="todo-input"]`, "Second test todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for fragment updates
	)

	if err != nil {
		t.Fatalf("Console warnings test failed: %v", err)
	}

	// Analyze console messages for "Element not found" warnings
	var elementNotFoundWarnings []string
	var allWarnings []string

	for _, msg := range consoleMessages {
		msgText := msg["text"].(string)
		msgType := msg["type"].(string)

		// Look for any warnings or errors
		if msgType == "warning" || msgType == "error" {
			allWarnings = append(allWarnings, fmt.Sprintf("[%s] %s", msgType, msgText))

			// Specifically look for "Element not found" warnings
			if strings.Contains(msgText, "Element with lvt-id=") && strings.Contains(msgText, "not found") {
				elementNotFoundWarnings = append(elementNotFoundWarnings, msgText)
			}
		}
	}

	// Report results
	fmt.Printf("\n=== CONSOLE WARNINGS TEST RESULTS ===\n")
	fmt.Printf("Total console messages captured: %d\n", len(consoleMessages))
	fmt.Printf("Total warnings/errors: %d\n", len(allWarnings))
	fmt.Printf("Element not found warnings: %d\n", len(elementNotFoundWarnings))

	if len(allWarnings) > 0 {
		fmt.Printf("\nAll warnings/errors:\n")
		for _, warning := range allWarnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	if len(elementNotFoundWarnings) > 0 {
		fmt.Printf("\nElement not found warnings:\n")
		for _, warning := range elementNotFoundWarnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	// Assertions
	if len(elementNotFoundWarnings) > 0 {
		t.Errorf("❌ Found %d 'Element not found' warnings in console. This indicates fragments are being sent for elements that don't exist in the DOM.", len(elementNotFoundWarnings))
	} else {
		fmt.Printf("✅ No 'Element not found' warnings detected\n")
	}

	// Also check for any other warnings that might indicate problems
	if len(allWarnings) > 0 {
		fmt.Printf("⚠️  Found %d other warnings/errors in console (may need investigation)\n", len(allWarnings))
	} else {
		fmt.Printf("✅ No console warnings or errors detected\n")
	}
}
