package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestChatE2E tests the chat app end-to-end with a real browser
func TestChatE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Use fixed port for simplicity
	serverPort := 8096
	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)

	// Start chat server
	t.Logf("Starting test server on port %d", serverPort)
	serverCmd := exec.Command("go", "run", "main.go")
	serverCmd.Env = append(serverCmd.Environ(), fmt.Sprintf("PORT=%d", serverPort))

	// Don't capture stdout/stderr to avoid I/O blocking
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
			// Don't call Wait() - it blocks on I/O
		}
	}()

	// Wait for server to be ready
	ready := false
	for i := 0; i < 50; i++ { // 5 seconds
		resp, err := http.Get(serverURL)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		serverCmd.Process.Kill()
		t.Fatal("Server failed to start within 5 seconds")
	}

	t.Logf("✅ Test server ready at %s", serverURL)

	// Use local Chrome instead of Docker for simplicity
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancelBrowser()

	// Set timeout for the entire test
	browserCtx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelTimeout()

	t.Run("Initial_Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(browserCtx,
			chromedp.Navigate(serverURL),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for WebSocket
			chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify welcome message visible
		if !strings.Contains(initialHTML, "Welcome") {
			t.Errorf("Initial page should show welcome message")
		}

		// Verify join form visible
		if !strings.Contains(initialHTML, `name="username"`) {
			t.Errorf("Initial page should show username input")
		}

		// Verify no template expressions leaked
		if strings.Contains(initialHTML, "{{") {
			t.Errorf("Initial HTML contains unprocessed template expressions")
		}

		t.Logf("✅ Initial page loaded correctly")
	})

	t.Run("Join_Flow", func(t *testing.T) {
		var initialStatsText string
		var initialFormVisible bool
		var afterStatsText string
		var afterChatVisible bool
		var afterFormVisible bool

		err := chromedp.Run(browserCtx,
			// Capture initial state
			chromedp.Text(".stats", &initialStatsText, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('form[lvt-submit="join"]') !== null`, &initialFormVisible),

			// Fill and submit join form
			chromedp.SetValue(`input[name="username"]`, "testuser", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for WebSocket update

			// Capture after-join state
			chromedp.Text(".stats", &afterStatsText, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('.messages') !== null`, &afterChatVisible),
			chromedp.Evaluate(`document.querySelector('form[lvt-submit="join"]') !== null`, &afterFormVisible),
		)

		if err != nil {
			t.Fatalf("Join flow failed: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialStatsText, "Welcome") {
			t.Errorf("Initial stats should show welcome message, got: %q", initialStatsText)
		}
		if !initialFormVisible {
			t.Error("Join form should be visible initially")
		}

		// Verify after-join state
		if !strings.Contains(afterStatsText, "Logged in as testuser") {
			t.Errorf("After join, stats should show logged in state, got: %q", afterStatsText)
		}
		if !strings.Contains(afterStatsText, "user") && !strings.Contains(afterStatsText, "online") {
			t.Errorf("After join, stats should show online users, got: %q", afterStatsText)
		}
		if !strings.Contains(afterStatsText, "message") {
			t.Errorf("After join, stats should show message count, got: %q", afterStatsText)
		}
		if !afterChatVisible {
			t.Error("Chat interface should be visible after join")
		}
		if afterFormVisible {
			t.Error("Join form should NOT be visible after join")
		}

		t.Logf("✅ Chat join flow test passed")
		t.Logf("   Initial: %q", initialStatsText)
		t.Logf("   After:   %q", afterStatsText)
	})

	t.Run("Send_Message", func(t *testing.T) {
		var messageCount int

		err := chromedp.Run(browserCtx,
			// Send a message
			chromedp.SetValue(`input[name="message"]`, "Hello, world!", chromedp.ByQuery),
			chromedp.Click(`form[lvt-submit="send"] button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for WebSocket update

			// Count messages in DOM
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &messageCount),
		)

		if err != nil {
			t.Fatalf("Send message failed: %v", err)
		}

		if messageCount != 1 {
			t.Errorf("Expected 1 message in chat, got %d", messageCount)
		}

		t.Logf("✅ Message send test passed")
	})

	t.Run("WebSocket_Updates", func(t *testing.T) {
		var finalHTML string

		err := chromedp.Run(browserCtx,
			chromedp.OuterHTML(`[data-lvt-id]`, &finalHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to get final HTML: %v", err)
		}

		// Verify no template expressions leaked through
		if strings.Contains(finalHTML, "{{") {
			t.Errorf("Final HTML contains template expressions")
		}

		// Verify message is present
		if !strings.Contains(finalHTML, "Hello, world!") {
			t.Errorf("Final HTML should contain sent message")
		}

		t.Logf("✅ WebSocket updates working correctly")
	})

	t.Logf("\n============================================================")
	t.Logf("🎉 All Chat E2E tests passed!")
	t.Logf("============================================================")
}
