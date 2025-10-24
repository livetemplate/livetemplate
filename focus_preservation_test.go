package livetemplate

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// FocusTestState for focus preservation tests
type FocusTestState struct {
	Message string
	Counter int
}

// Implement Store interface
func (s *FocusTestState) Change(ctx *ActionContext) error {
	switch ctx.Action {
	case "increment":
		s.Counter++
	}
	return nil
}

// TestFocusPreservation verifies that input focus and cursor position are preserved during updates
func TestFocusPreservation(t *testing.T) {
	state := &FocusTestState{
		Message: "Focus Preservation Test",
		Counter: 0,
	}

	tmpl := New("focus-test")

	templateStr := `<!DOCTYPE html>
<html>
<head>
	<title>Focus Test</title>
</head>
<body>
	<h1>{{.Message}}</h1>
	<p>Counter: <strong id="counter">{{.Counter}}</strong></p>
	<form>
		<input type="text" name="username" id="username" placeholder="Type your name...">
		<input type="email" name="email" id="email" placeholder="Type your email...">
		<textarea name="bio" id="bio" placeholder="Type your bio..."></textarea>
		<input type="number" name="age" id="age" placeholder="Enter age">
		<button type="button" id="increment-btn" lvt-click="increment">Increment Counter</button>
	</form>
	<script src="/client.js"></script>
</body>
</html>`

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(state))

	// Serve client JavaScript
	mux.HandleFunc("/client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/Users/adnaan/code/livefir/livetemplate/client/dist/livetemplate-client.browser.js")
	})

	// Start test server
	port := 9003
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Ensure server is shut down after test
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Logf("Server shutdown warning: %v", err)
		}
	}()

	// Create chromedp context with console log capture
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			for _, arg := range ev.Args {
				log.Printf("Console: %s", string(arg.Value))
			}
		}
	})

	url := fmt.Sprintf("http://localhost:%d", port)

	var inputValue string
	var cursorPosition int
	var counterValue string
	var hasFocus bool

	// Test sequence
	err := chromedp.Run(ctx,
		// Navigate to page
		chromedp.Navigate(url),

		// Wait for elements to be present
		chromedp.WaitVisible(`#username`, chromedp.ByID),
		chromedp.WaitVisible(`#increment-btn`, chromedp.ByID),

		// Wait for WebSocket connection
		chromedp.Sleep(2*time.Second),

		// Type text into username input
		chromedp.SendKeys(`#username`, "HelloWorld", chromedp.ByID),

		// Move cursor to position 5 (between "Hello" and "World")
		chromedp.Evaluate(`
			(function() {
				const input = document.getElementById('username');
				input.setSelectionRange(5, 5);
			})()
		`, nil),

		// Click increment button to trigger update
		chromedp.Click(`#increment-btn`, chromedp.ByID),

		// Wait for update to complete
		chromedp.Sleep(500*time.Millisecond),

		// Get input value (should still be "HelloWorld")
		chromedp.Evaluate(`document.getElementById('username').value`, &inputValue),

		// Get cursor position (should still be 5)
		chromedp.Evaluate(`document.getElementById('username').selectionStart`, &cursorPosition),

		// Check if input still has focus
		chromedp.Evaluate(`document.getElementById('username') === document.activeElement`, &hasFocus),

		// Verify counter was incremented
		chromedp.Text(`#counter`, &counterValue, chromedp.ByID),
	)

	if err != nil {
		t.Fatalf("Chromedp error: %v", err)
	}

	// Verify input value was preserved
	if inputValue != "HelloWorld" {
		t.Errorf("Input value should be preserved. Expected 'HelloWorld', got '%s'", inputValue)
	}

	// Verify cursor position was preserved
	if cursorPosition != 5 {
		t.Errorf("Cursor position should be preserved. Expected 5, got %d", cursorPosition)
	}

	// Verify element still has focus
	if !hasFocus {
		t.Errorf("Input should still have focus after update")
	}

	// Verify counter was actually updated
	if counterValue != "1" {
		t.Errorf("Counter should be updated. Expected '1', got '%s'", counterValue)
	}

	t.Log("✅ Input value preserved:", inputValue)
	t.Log("✅ Cursor position preserved:", cursorPosition)
	t.Log("✅ Focus preserved:", hasFocus)
	t.Log("✅ Counter updated:", counterValue)
}

// TestFocusPreservationMultipleInputs tests focus preservation across different input types
func TestFocusPreservationMultipleInputs(t *testing.T) {
	state := &FocusTestState{
		Message: "Multiple Inputs Focus Test",
		Counter: 0,
	}

	tmpl := New("focus-multi-test")

	templateStr := `<!DOCTYPE html>
<html>
<head>
	<title>Multi Focus Test</title>
</head>
<body>
	<h1>{{.Message}}</h1>
	<p>Counter: <strong id="counter">{{.Counter}}</strong></p>
	<form>
		<textarea name="notes" id="notes" placeholder="Type notes..."></textarea>
		<input type="email" name="contact" id="contact" placeholder="your@email.com">
		<button type="button" id="trigger-btn" lvt-click="increment">Trigger Update</button>
	</form>
	<script src="/client.js"></script>
</body>
</html>`

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(state))

	// Serve client JavaScript
	mux.HandleFunc("/client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/Users/adnaan/code/livefir/livetemplate/client/dist/livetemplate-client.browser.js")
	})

	// Start test server
	port := 9004
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Logf("Server shutdown warning: %v", err)
		}
	}()

	// Create chromedp context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Capture console logs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			for _, arg := range ev.Args {
				log.Printf("Console: %s", string(arg.Value))
			}
		}
	})

	url := fmt.Sprintf("http://localhost:%d", port)

	var textareaValue string
	var textareaCursor int

	// Test textarea focus preservation
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#notes`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),

		// Type into textarea
		chromedp.SendKeys(`#notes`, "First line\nSecond line", chromedp.ByID),

		// Set cursor position in the middle
		chromedp.Evaluate(`
			(function() {
				const textarea = document.getElementById('notes');
				textarea.setSelectionRange(10, 10);
			})()
		`, nil),

		// Trigger update
		chromedp.Click(`#trigger-btn`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),

		// Verify preservation
		chromedp.Evaluate(`document.getElementById('notes').value`, &textareaValue),
		chromedp.Evaluate(`document.getElementById('notes').selectionStart`, &textareaCursor),
	)

	if err != nil {
		t.Fatalf("Chromedp error: %v", err)
	}

	if textareaValue != "First line\nSecond line" {
		t.Errorf("Textarea value not preserved. Expected 'First line\\nSecond line', got '%s'", textareaValue)
	}

	if textareaCursor != 10 {
		t.Errorf("Textarea cursor not preserved. Expected 10, got %d", textareaCursor)
	}

	t.Log("✅ Textarea value preserved:", textareaValue)
	t.Log("✅ Textarea cursor preserved:", textareaCursor)
}
