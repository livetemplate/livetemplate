package livetemplate

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestState for loading indicator tests
type LoadingTestState struct {
	Message string
}

// Implement Store interface
func (s *LoadingTestState) Change(ctx *ActionContext) error {
	return nil
}

// TestLoadingIndicator verifies the loading indicator appears and disappears correctly
func TestLoadingIndicator(t *testing.T) {
	state := &LoadingTestState{
		Message: "Hello, Loading Test!",
	}

	// Create template with loading indicator enabled (default)
	tmpl := New("loading-test")

	templateStr := `<!DOCTYPE html>
<html>
<head>
	<title>Loading Test</title>
</head>
<body>
	<h1>{{.Message}}</h1>
	<form>
		<input type="text" name="test" id="test-input" placeholder="Type here...">
		<button type="submit" id="test-button">Submit</button>
	</form>
	<script src="/client.js"></script>
</body>
</html>`

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.Handle("/", Mount(tmpl, state))

	// Serve client JavaScript
	mux.HandleFunc("/client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/Users/adnaan/code/livefir/livetemplate/client/dist/livetemplate-client.browser.js")
	})

	// Start test server
	port := 9001
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

	// FIRST: Fetch raw HTML directly with HTTP GET to verify server-side rendering
	// This bypasses all JavaScript execution
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	// Read the raw HTML
	rawHTML := make([]byte, 5000)
	n, _ := resp.Body.Read(rawHTML)
	rawHTMLStr := string(rawHTML[:n])

	// Verify loading attribute is present in the server-rendered HTML
	hasLoadingAttrInRawHTML := strings.Contains(rawHTMLStr, `data-lvt-loading="true"`)
	if !hasLoadingAttrInRawHTML {
		t.Error("❌ data-lvt-loading attribute should be present in server-rendered HTML")
		t.Logf("Raw HTML (first 1000 chars): %s", rawHTMLStr[:min(1000, len(rawHTMLStr))])
	} else {
		t.Log("✅ Loading indicator attribute present in server-rendered HTML")
	}

	// SECOND: Use browser to verify JavaScript removes the attribute after WebSocket init
	var loadingAttrAfterJS bool

	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(1*time.Second), // Give JavaScript time to initialize

		// Check if loading attribute still exists after JavaScript runs
		chromedp.Evaluate(`
			(function() {
				const wrapper = document.querySelector('[data-lvt-id]');
				return wrapper && wrapper.getAttribute('data-lvt-loading') === 'true';
			})()
		`, &loadingAttrAfterJS),
	)

	if err != nil {
		t.Fatalf("Chromedp error: %v", err)
	}

	// Verify loading attribute is removed by JavaScript after initialization
	if loadingAttrAfterJS {
		t.Error("❌ data-lvt-loading attribute should be removed by JavaScript after WebSocket initialization")
	} else {
		t.Log("✅ Loading attribute removed by JavaScript after WebSocket initialization")
	}

	// THIRD: Verify form inputs are enabled after WebSocket initialization
	var inputEnabledAfterInit bool
	var buttonEnabledAfterInit bool

	err = chromedp.Run(ctx,
		// Check if form inputs are enabled (they should be after initialization)
		chromedp.Evaluate(`!document.getElementById('test-input').disabled`, &inputEnabledAfterInit),
		chromedp.Evaluate(`!document.getElementById('test-button').disabled`, &buttonEnabledAfterInit),
	)

	if err != nil {
		t.Fatalf("Chromedp error (form enabled checks): %v", err)
	}

	// Verify inputs are enabled after WebSocket initialization
	if !inputEnabledAfterInit {
		t.Error("❌ Input should be enabled after initialization")
	} else {
		t.Log("✅ Input enabled after WebSocket initialization")
	}

	if !buttonEnabledAfterInit {
		t.Error("❌ Button should be enabled after initialization")
	} else {
		t.Log("✅ Button enabled after WebSocket initialization")
	}
}

// TestLoadingIndicatorDisabled verifies the loading indicator can be disabled
func TestLoadingIndicatorDisabled(t *testing.T) {
	state := &LoadingTestState{
		Message: "No Loading Test",
	}

	// Create template with loading indicator disabled
	tmpl := New("no-loading-test", WithLoadingDisabled())

	templateStr := `<!DOCTYPE html>
<html>
<head>
	<title>No Loading Test</title>
</head>
<body>
	<h1>{{.Message}}</h1>
	<form>
		<input type="text" name="test" id="test-input" placeholder="Type here...">
		<button type="submit" id="test-button">Submit</button>
	</form>
	<script src="/client.js"></script>
</body>
</html>`

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.Handle("/", Mount(tmpl, state))

	// Serve client JavaScript
	mux.HandleFunc("/client.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/Users/adnaan/code/livefir/livetemplate/client/dist/livetemplate-client.browser.js")
	})

	// Start test server
	port := 9002
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

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://localhost:%d", port)

	var hasLoadingAttr bool
	var loadingBarExists bool
	var inputDisabled bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),

		// Wait for form elements to be present
		chromedp.WaitVisible(`#test-input`, chromedp.ByID),

		// Check if data-lvt-loading attribute is present
		chromedp.Evaluate(`
			(function() {
				const wrapper = document.querySelector('[data-lvt-id]');
				return wrapper && wrapper.getAttribute('data-lvt-loading') === 'true';
			})()
		`, &hasLoadingAttr),

		// Check if loading bar exists
		chromedp.Evaluate(`
			(function() {
				const loadingBar = document.querySelector('[style*="position: fixed"][style*="top: 0"]');
				return loadingBar !== null;
			})()
		`, &loadingBarExists),

		// Check if input is disabled
		chromedp.Evaluate(`document.getElementById('test-input').disabled`, &inputDisabled),
	)

	if err != nil {
		t.Fatalf("Chromedp error: %v", err)
	}

	// Verify loading attribute is NOT present when disabled
	if hasLoadingAttr {
		t.Error("data-lvt-loading attribute should not be present when loading is disabled")
	}

	// Verify loading bar does NOT exist
	if loadingBarExists {
		t.Error("Loading bar should not exist when loading indicator is disabled")
	}

	// Verify inputs are NOT disabled
	if inputDisabled {
		t.Error("Input should not be disabled when loading indicator is disabled")
	}

	t.Log("✅ Loading indicator properly disabled via WithLoadingDisabled()")
}
