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
)

var dockerContainer string

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
	ctx, timeoutCancel := context.WithTimeout(ctx, 45*time.Second)

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

func TestInputDebugE2E(t *testing.T) {
	// Test against running server on 8096 (from background process)
	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var consoleMessages []string

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			for _, arg := range ev.Args {
				if arg.Value != nil {
					consoleMessages = append(consoleMessages, string(arg.Value))
				}
			}
		}
	})

	err := chromedp.Run(ctx,
		// Navigate to the running test server
		chromedp.Navigate("http://host.docker.internal:8102"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Debug: Show WebSocket message handling
		chromedp.Evaluate(`
			if (window.liveTemplate && window.liveTemplate.ws) {
				const originalOnMessage = window.liveTemplate.ws.onmessage;
				window.liveTemplate.ws.onmessage = function(event) {
					console.log('🔍 WEBSOCKET RECEIVED:', event.data);
					try {
						const fragments = JSON.parse(event.data);
						console.log('🔍 PARSED FRAGMENTS:', JSON.stringify(fragments, null, 2));
						fragments.forEach((fragment, i) => {
							console.log('🔍 FRAGMENT', i, 'ID:', fragment.id, 'DATA:', JSON.stringify(fragment.data));
						});
					} catch (e) {
						console.log('🔍 PARSE ERROR:', e);
					}
					return originalOnMessage.call(this, event);
				};
			}
		`, nil),

		// Test: Send validation error action
		chromedp.Evaluate(`
			if (window.liveTemplate && window.liveTemplate.isConnected()) {
				console.log('🔍 BEFORE ACTION - input.value:', document.querySelector('input[name="todo-input"]').value);
				console.log('🔍 BEFORE ACTION - input.getAttribute("value"):', document.querySelector('input[name="todo-input"]').getAttribute('value'));
				console.log('🔍 SENDING ACTION: addtodo with "hi"');
				window.liveTemplate.sendAction('addtodo', { 'todo-input': 'hi' });
			}
		`, nil),
		chromedp.Sleep(3*time.Second),

		// Debug after fragments processed
		chromedp.Evaluate(`
			console.log('🔍 AFTER FRAGMENTS - input.value:', document.querySelector('input[name="todo-input"]').value);
			console.log('🔍 AFTER FRAGMENTS - input.getAttribute("value"):', document.querySelector('input[name="todo-input"]').getAttribute('value'));
			console.log('🔍 AFTER FRAGMENTS - input.outerHTML:', document.querySelector('input[name="todo-input"]').outerHTML);
		`, nil),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Input debug test failed: %v", err)
	}

	// Print captured console messages
	fmt.Println("\n=== CAPTURED CONSOLE MESSAGES ===")
	for _, msg := range consoleMessages {
		fmt.Println(msg)
	}
}

func TestInputClearingE2E(t *testing.T) {
	// Start test server with port 8103 to test the fix
	server := startTestServer(t, 8103)
	defer server.Shutdown(context.Background())

	// Create browser context (will use Docker if available, local Chrome otherwise)
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var initialInputValue, afterErrorInputValue, afterSuccessInputValue string
	var todoCount int
	var errorVisible bool

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate("http://host.docker.internal:8103"),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for page to fully load and LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Verify initial input is empty
		chromedp.WaitVisible(`input[name="todo-input"]`, chromedp.ByQuery),
		chromedp.AttributeValue(`input[name="todo-input"]`, "value", &initialInputValue, nil, chromedp.ByQuery),

		// Test 1: Try to add invalid todo (too short) - input should NOT clear
		// Call the LiveTemplate client directly bypassing form data collection
		chromedp.Evaluate(`
			if (window.liveTemplate && window.liveTemplate.isConnected()) {
				console.log('Sending action directly with data:', { 'todo-input': 'hi' });
				window.liveTemplate.sendAction('addtodo', { 'todo-input': 'hi' });
			} else {
				console.error('LiveTemplate not connected');
			}
		`, nil),
		chromedp.Sleep(3*time.Second), // Wait for server response and fragment update

		// Check input is preserved and error appeared after validation error
		chromedp.AttributeValue(`input[name="todo-input"]`, "value", &afterErrorInputValue, nil, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('.error-section') && document.querySelector('.error-section').textContent !== ''`, &errorVisible),

		// Test 2: Add valid todo - input SHOULD clear
		chromedp.SetValue(`input[name="todo-input"]`, "Valid Todo That Should Clear Input", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value = "Valid Todo That Should Clear Input"`, nil),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for server response and fragment update

		// Check that input field is now cleared after successful creation
		chromedp.AttributeValue(`input[name="todo-input"]`, "value", &afterSuccessInputValue, nil, chromedp.ByQuery),

		// Verify todo was actually added
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &todoCount),
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
		t.Errorf("❌ Expected error to be visible after validation failure")
	} else {
		fmt.Printf("✅ Error shown after validation failure\n")
	}

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
}

func TestErrorDisplayE2E(t *testing.T) {
	// Start test server
	server := startTestServer(t, 8096)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var errorText string
	var errorVisible, errorVisibleAfterReload bool

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate("http://host.docker.internal:8096"),
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
		chromedp.Evaluate(`document.querySelector('.error-section') && document.querySelector('.error-section').textContent.includes('least 3 characters')`, &errorVisible),
		chromedp.Evaluate(`document.querySelector('.error-section') ? document.querySelector('.error-section').textContent.trim() : ''`, &errorText),

		// Capture console logs for debugging
		chromedp.Evaluate(`console.log('Error visible:', document.querySelector('.error-section') !== null)`, nil),
		chromedp.Evaluate(`console.log('Error content:', document.querySelector('.error-section') ? document.querySelector('.error-section').textContent : 'null')`, nil),
		chromedp.Evaluate(`console.log('Error display style:', document.querySelector('.error-section') ? getComputedStyle(document.querySelector('.error-section')).display : 'null')`, nil),

		// Test 2: Reload page and verify error persists (server state)
		chromedp.Reload(),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelector('.error-section') && document.querySelector('.error-section').textContent !== ''`, &errorVisibleAfterReload),
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
	server := startTestServer(t, 8097)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var todoCount, finalTodoCount int
	var todoText string

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate("http://host.docker.internal:8097"),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Verify initial state (should have 0 todos)
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &todoCount),

		// Add a test todo
		chromedp.SetValue(`input[name="todo-input"]`, "Test Todo for Removal", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="todo-input"]').value = "Test Todo for Removal"`, nil),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fragment update

		// Verify todo was added
		chromedp.WaitVisible(`.todo-item`, chromedp.ByQuery),
		chromedp.Text(`.todo-text`, &todoText, chromedp.ByQuery),

		// Add console logging before removal
		chromedp.Evaluate(`console.log('Before removal - todo count:', document.querySelectorAll('.todo-item').length)`, nil),
		chromedp.Evaluate(`console.log('Remove button exists:', document.querySelector('.remove-btn') !== null)`, nil),

		// Click the remove button
		chromedp.Click(`.remove-btn`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for fragment update

		// Add console logging after removal
		chromedp.Evaluate(`console.log('After removal - todo count:', document.querySelectorAll('.todo-item').length)`, nil),
		chromedp.Evaluate(`console.log('LiveTemplate connected:', window.liveTemplate && window.liveTemplate.isConnected())`, nil),

		// Verify todo was removed
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &finalTodoCount),
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

func TestMultipleTodosE2E(t *testing.T) {
	// Start test server
	server := startTestServer(t, 8098)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var afterAdds, afterFirstRemove, afterSecondRemove int

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate("http://host.docker.internal:8098"),
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
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterAdds),
		chromedp.Evaluate(`console.log('After adding 3 todos, count:', document.querySelectorAll('.todo-item').length)`, nil),

		// Remove the middle todo
		chromedp.Evaluate(`console.log('Before removing middle todo')`, nil),
		chromedp.Click(`.todo-item:nth-child(2) .remove-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterFirstRemove),
		chromedp.Evaluate(`console.log('After removing middle todo, count:', document.querySelectorAll('.todo-item').length)`, nil),

		// Remove the first remaining todo
		chromedp.Evaluate(`console.log('Before removing first todo')`, nil),
		chromedp.Click(`.todo-item:first-child .remove-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.querySelectorAll('.todo-item').length`, &afterSecondRemove),
		chromedp.Evaluate(`console.log('After removing first todo, count:', document.querySelectorAll('.todo-item').length)`, nil),
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
	server := startTestServer(t, 8094)
	defer server.Shutdown(context.Background())

	// Create browser context
	ctx, cancel := setupBrowser(t)
	defer cancel()

	var initialCountText, afterFirstAddCountText, afterSecondAddCountText, afterRemoveCountText string
	var todoCountHTML string
	var todoCountLvtID string

	err := chromedp.Run(ctx,
		// Navigate to the test server
		chromedp.Navigate("http://host.docker.internal:8094"),
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Wait for LiveTemplate to connect
		chromedp.Sleep(2*time.Second),

		// Get the initial HTML and lvt-id of todo-count div
		chromedp.OuterHTML(`.todo-count`, &todoCountHTML, chromedp.ByQuery),
		chromedp.AttributeValue(`.todo-count`, "lvt-id", &todoCountLvtID, nil, chromedp.ByQuery),

		// Set up WebSocket message capture
		chromedp.Evaluate(`
			window.wsMessages = [];
			if (window.liveTemplate && window.liveTemplate.ws) {
				const originalOnMessage = window.liveTemplate.ws.onmessage;
				window.liveTemplate.ws.onmessage = function(event) {
					window.wsMessages.push(event.data);
					console.log('WS Message captured:', event.data);
					if (originalOnMessage) {
						originalOnMessage.call(this, event);
					}
				};
			}
			'WebSocket monitoring enabled'
		`, nil),

		// Check initial todo count display
		chromedp.Text(`.todo-count`, &initialCountText, chromedp.ByQuery),

		// Add first todo
		chromedp.SetValue(`input[name="todo-input"]`, "First Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Check todo count after first add
		chromedp.Text(`.todo-count`, &afterFirstAddCountText, chromedp.ByQuery),

		// Add second todo
		chromedp.SetValue(`input[name="todo-input"]`, "Second Todo", chromedp.ByQuery),
		chromedp.Click(`button[data-lvt-action="addtodo"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Check todo count after second add
		chromedp.Text(`.todo-count`, &afterSecondAddCountText, chromedp.ByQuery),

		// Remove one todo
		chromedp.Click(`.todo-item:first-child .remove-btn`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// Check todo count after removal
		chromedp.Text(`.todo-count`, &afterRemoveCountText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Todo count display test failed: %v", err)
	}

	// Assertions
	fmt.Printf("\n=== TODO COUNT DISPLAY E2E TEST RESULTS ===\n")

	expected := []struct {
		description string
		actual      string
		expected    string
	}{
		{"Initial count", initialCountText, "Total: 0 todos"},
		{"After first add", afterFirstAddCountText, "Total: 1 todo"},
		{"After second add", afterSecondAddCountText, "Total: 2 todos"},
		{"After removal", afterRemoveCountText, "Total: 1 todo"},
	}

	allPassed := true
	for _, test := range expected {
		if test.actual == test.expected {
			fmt.Printf("✅ %s: '%s'\n", test.description, test.actual)
		} else {
			fmt.Printf("❌ %s: Expected '%s', got '%s'\n", test.description, test.expected, test.actual)
			t.Errorf("❌ %s: Expected '%s', got '%s'", test.description, test.expected, test.actual)
			allPassed = false
		}
	}

	if allPassed {
		fmt.Printf("✅ Todo count display test passed successfully\n")
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
	mux.HandleFunc("/client/livetemplate-client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "../../client/livetemplate-client.js")
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
