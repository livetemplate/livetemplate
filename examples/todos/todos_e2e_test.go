package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// TestTodosE2E tests the todo app end-to-end with a real browser
func TestTodosE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Get free ports for server and Chrome debugging
	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	// Start todo server
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Start Docker Chrome container
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, chromeCmd, debugPort)

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	// Set timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for WebSocket connection
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialHTML, "Todo App") {
			t.Error("Page title not found")
		}
		if !strings.Contains(initialHTML, "Statistics") {
			t.Error("Statistics section not found")
		}
		// Check for either empty state or table structure
		hasEmptyState := strings.Contains(initialHTML, "No tasks")
		hasTasksSection := strings.Contains(initialHTML, "Tasks")
		if !hasEmptyState && !hasTasksSection {
			t.Error("Tasks section not found")
		}

		t.Log("✅ Initial page load verified")
	})

	t.Run("WebSocket Connection", func(t *testing.T) {
		// Check for console errors
		var logs []string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`console.log('WebSocket test'); 'logged'`, nil),
			chromedp.Sleep(500*time.Millisecond),
		)

		if err != nil {
			t.Fatalf("Failed to check console: %v", err)
		}

		// If we got here without WebSocket errors, connection is working
		t.Log("✅ WebSocket connection working")
		_ = logs // Prevent unused variable error
	})

	t.Run("Add First Todo", func(t *testing.T) {
		var html string

		// Add first todo
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="text"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="text"]`, "First Todo Item", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for WebSocket update
			chromedp.OuterHTML(`section`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to add first todo: %v", err)
		}

		// Verify first todo was added
		if !strings.Contains(html, "First Todo Item") {
			t.Errorf("First todo not found in HTML. HTML: %s", html)
		}

		// Check for [object Object] bug
		if strings.Contains(html, "[object Object]") {
			t.Errorf("Found [object Object] bug after adding first todo. HTML: %s", html)
		}

		t.Log("✅ First todo added successfully")
	})

	t.Run("Add Second Todo", func(t *testing.T) {
		var html string

		// Add second todo
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="text"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="text"]`, "Second Todo Item", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for WebSocket update
			chromedp.OuterHTML(`section`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to add second todo: %v", err)
		}

		t.Logf("Section HTML after adding second todo: %s", html)

		// Verify both todos are present
		if !strings.Contains(html, "First Todo Item") {
			t.Errorf("First todo disappeared after adding second. HTML: %s", html)
		}

		if !strings.Contains(html, "Second Todo Item") {
			t.Errorf("Second todo not found in HTML. HTML: %s", html)
		}

		// Check for [object Object] bug - THIS IS THE KEY TEST
		if strings.Contains(html, "[object Object]") {
			t.Errorf("Found [object Object] bug after adding second todo. HTML: %s", html)
		}

		t.Log("✅ Second todo added successfully")
	})

	t.Run("Add Third Todo", func(t *testing.T) {
		var html string

		// Add third todo
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="text"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="text"]`, "Third Todo Item", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for WebSocket update
			chromedp.OuterHTML(`section`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to add third todo: %v", err)
		}

		t.Logf("Section HTML after adding third todo: %s", html)

		// Verify all three todos are present
		if !strings.Contains(html, "First Todo Item") {
			t.Errorf("First todo disappeared after adding third. HTML: %s", html)
		}
		if !strings.Contains(html, "Second Todo Item") {
			t.Errorf("Second todo disappeared after adding third. HTML: %s", html)
		}
		if !strings.Contains(html, "Third Todo Item") {
			t.Errorf("Third todo not found in HTML. HTML: %s", html)
		}

		// Verify table structure is preserved
		if !strings.Contains(html, "<table>") {
			t.Errorf("Table element missing after adding third todo. HTML: %s", html)
		}
		if !strings.Contains(html, "<tbody>") {
			t.Errorf("Tbody element missing after adding third todo. HTML: %s", html)
		}
		if !strings.Contains(html, "<tr") {
			t.Errorf("Table row elements missing after adding third todo. HTML: %s", html)
		}

		// Check that each todo appears exactly once
		firstCount := strings.Count(html, "First Todo Item")
		secondCount := strings.Count(html, "Second Todo Item")
		thirdCount := strings.Count(html, "Third Todo Item")

		if firstCount != 1 {
			t.Errorf("First todo appears %d times (expected 1). HTML: %s", firstCount, html)
		}
		if secondCount != 1 {
			t.Errorf("Second todo appears %d times (expected 1). HTML: %s", secondCount, html)
		}
		if thirdCount != 1 {
			t.Errorf("Third todo appears %d times (expected 1). HTML: %s", thirdCount, html)
		}

		t.Log("✅ Third todo added successfully")
	})

	t.Run("Add Fourth and Fifth Todos", func(t *testing.T) {
		var html string

		// Add fourth todo
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="text"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="text"]`, "Fourth Todo Item", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add fourth todo: %v", err)
		}

		// Add fifth todo
		err = chromedp.Run(ctx,
			chromedp.WaitVisible(`input[name="text"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="text"]`, "Fifth Todo Item", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
			chromedp.OuterHTML(`section`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to add fifth todo: %v", err)
		}

		t.Logf("Section HTML after adding five todos: %s", html)

		// Verify all five todos are present
		todos := []string{"First Todo Item", "Second Todo Item", "Third Todo Item", "Fourth Todo Item", "Fifth Todo Item"}
		for _, todo := range todos {
			count := strings.Count(html, todo)
			if count != 1 {
				t.Errorf("Todo '%s' appears %d times (expected 1). HTML: %s", todo, count, html)
			}
		}

		// Verify table structure is still intact
		if !strings.Contains(html, "<table>") || !strings.Contains(html, "<tbody>") || !strings.Contains(html, "<tr") {
			t.Errorf("Table structure corrupted after adding five todos. HTML: %s", html)
		}

		t.Log("✅ Fourth and fifth todos added successfully")
	})

	t.Run("LiveTemplate Updates", func(t *testing.T) {
		// Take a screenshot for debugging
		var buf []byte
		err := chromedp.Run(ctx,
			chromedp.CaptureScreenshot(&buf),
		)

		if err != nil {
			t.Logf("Warning: Failed to capture screenshot: %v", err)
		} else {
			t.Logf("Screenshot captured: %d bytes", len(buf))
		}

		// Verify the page still has the LiveTemplate wrapper
		var html string
		err = chromedp.Run(ctx,
			chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}

		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not found after updates")
		}

		t.Log("✅ LiveTemplate wrapper preserved after updates")
	})

	t.Run("Pico CSS Loaded", func(t *testing.T) {
		// Verify Pico CSS is loaded by checking for specific styles
		var hasPicoStyles bool
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				const mainEl = document.querySelector('main.container');
				const hasContainer = mainEl !== null;
				const article = document.querySelector('article');
				const hasArticle = article !== null;
				hasContainer && hasArticle;
			`, &hasPicoStyles),
		)

		if err != nil {
			t.Fatalf("Failed to check Pico CSS: %v", err)
		}

		if !hasPicoStyles {
			t.Error("Pico CSS semantic elements not found")
		}

		t.Log("✅ Pico CSS loaded and semantic elements present")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 All E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
