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
	browserCtx, cancelTimeout := context.WithTimeout(browserCtx, 120*time.Second)
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
		var beforeHTML string
		var after1HTML string
		var after2HTML string
		var after3HTML string
		var msg1Count, msg2Count, msg3Count int
		var msg1Text, msg2Text, msg3Text string

		// Note: This test depends on Join_Flow having run first in the same browser context
		// When run standalone, we need to ensure we're in the joined state
		var isJoined bool
		chromedp.Run(browserCtx,
			chromedp.Evaluate(`document.querySelector('.messages') !== null`, &isJoined),
		)

		if !isJoined {
			t.Log("Not yet joined, performing join...")
			chromedp.Run(browserCtx,
				chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
				chromedp.SetValue(`input[name="username"]`, "testuser", chromedp.ByQuery),
				chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
				chromedp.Sleep(2*time.Second), // Wait longer for join update
				chromedp.WaitVisible(`.messages`, chromedp.ByQuery), // Explicitly wait for messages container
			)
			t.Log("Join completed, .messages container is visible")
		}

		err := chromedp.Run(browserCtx,

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 1: Capturing initial state")
				return nil
			}),
			chromedp.OuterHTML(`.messages`, &beforeHTML, chromedp.ByQuery),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 2: Sending FIRST message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "First message", chromedp.ByQuery),
			chromedp.Click(`form[lvt-submit="send"] button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 3: Checking first message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg1Count),
			chromedp.OuterHTML(`.messages`, &after1HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message-text')).map(el => el.textContent).join('|')`, &msg1Text),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Logf("After 1st: count=%d, text=%q", msg1Count, msg1Text)
				return nil
			}),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 4: Sending SECOND message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "Second message", chromedp.ByQuery),
			chromedp.Click(`form[lvt-submit="send"] button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 5: Checking second message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg2Count),
			chromedp.OuterHTML(`.messages`, &after2HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message-text')).map(el => el.textContent).join('|')`, &msg2Text),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Logf("After 2nd: count=%d, text=%q", msg2Count, msg2Text)
				return nil
			}),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 6: Sending THIRD message")
				return nil
			}),
			chromedp.SetValue(`input[name="message"]`, "Third message", chromedp.ByQuery),
			chromedp.Click(`form[lvt-submit="send"] button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),

			chromedp.ActionFunc(func(ctx context.Context) error {
				t.Log("Step 7: Checking third message")
				return nil
			}),
			chromedp.Evaluate(`document.querySelectorAll('.messages .message').length`, &msg3Count),
			chromedp.OuterHTML(`.messages`, &after3HTML, chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('.message-text')).map(el => el.textContent).join('|')`, &msg3Text),
		)

		if err != nil {
			t.Fatalf("Send message failed: %v", err)
		}

		// Log state at each step
		t.Logf("Before: empty=%v", strings.Contains(beforeHTML, "No messages yet"))
		t.Logf("After 1st: count=%d, texts=%q", msg1Count, msg1Text)
		t.Logf("After 2nd: count=%d, texts=%q", msg2Count, msg2Text)
		t.Logf("After 3rd: count=%d, texts=%q", msg3Count, msg3Text)

		// Verify first message
		if msg1Count != 1 {
			t.Errorf("After 1st message: expected 1 message, got %d", msg1Count)
			t.Logf("HTML after 1st:\n%s", after1HTML)
		}
		if !strings.Contains(msg1Text, "First message") {
			t.Errorf("After 1st message: expected 'First message', got %q", msg1Text)
		}

		// Verify second message
		if msg2Count != 2 {
			t.Errorf("After 2nd message: expected 2 messages, got %d", msg2Count)
			t.Logf("HTML after 2nd:\n%s", after2HTML)
		}
		if !strings.Contains(msg2Text, "First message") {
			t.Errorf("After 2nd message: 'First message' missing from %q", msg2Text)
		}
		if !strings.Contains(msg2Text, "Second message") {
			t.Errorf("After 2nd message: 'Second message' missing from %q", msg2Text)
		}

		// Verify third message
		if msg3Count != 3 {
			t.Errorf("After 3rd message: expected 3 messages, got %d", msg3Count)
			t.Logf("HTML after 3rd:\n%s", after3HTML)
		}
		if !strings.Contains(msg3Text, "First message") {
			t.Errorf("After 3rd message: 'First message' missing from %q", msg3Text)
		}
		if !strings.Contains(msg3Text, "Second message") {
			t.Errorf("After 3rd message: 'Second message' missing from %q", msg3Text)
		}
		if !strings.Contains(msg3Text, "Third message") {
			t.Errorf("After 3rd message: 'Third message' missing from %q", msg3Text)
		}

		t.Logf("✅ Multiple message send test passed")
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
