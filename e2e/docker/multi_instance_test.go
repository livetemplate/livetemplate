package e2e

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

	"github.com/chromedp/chromedp"
)

const (
	// Chrome navigates to servers via Docker network service names (container-to-container).
	serverAURL  = "http://server-a:8080"
	serverBURL  = "http://server-b:8080"
	// Test runner connects to Chrome via exposed host port.
	chromeWSURL = "ws://localhost:19222"
)

func TestMain(m *testing.M) {
	// Start docker compose
	if err := dockerComposeUp(); err != nil {
		log.Fatalf("docker compose up failed: %v", err)
	}

	// Wait for services to be healthy (use localhost URLs, not docker-internal)
	if err := waitForHealth("http://localhost:18091", 30*time.Second); err != nil {
		dockerComposeDown()
		log.Fatalf("server-a not healthy: %v", err)
	}
	if err := waitForHealth("http://localhost:18092", 30*time.Second); err != nil {
		dockerComposeDown()
		log.Fatalf("server-b not healthy: %v", err)
	}
	if err := waitForChrome(30*time.Second); err != nil {
		dockerComposeDown()
		log.Fatalf("chrome not ready: %v", err)
	}

	code := m.Run()

	dockerComposeDown()
	os.Exit(code)
}

func dockerComposeUp() error {
	cmd := exec.Command("docker", "compose", "up", "-d", "--build", "--wait")
	cmd.Dir = composeDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerComposeDown() {
	cmd := exec.Command("docker", "compose", "down", "-v", "--remove-orphans")
	cmd.Dir = composeDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func composeDir() string {
	// Tests run from the e2e/docker directory
	return "."
}

func waitForHealth(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", baseURL)
}

func waitForChrome(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:19222/json/version")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for Chrome")
}

func newRemoteBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeWSURL)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	cancel := func() {
		ctxCancel()
		allocCancel()
	}
	return ctx, cancel
}

func newTab(t *testing.T, allocCtx context.Context) (context.Context, context.CancelFunc) {
	t.Helper()
	return chromedp.NewContext(allocCtx)
}

// collectConsoleLogs enables console log collection for debugging.
func collectConsoleLogs(t *testing.T, ctx context.Context) {
	t.Helper()
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ev, ok := ev.(interface{ GetType() string }); ok {
			if ev.GetType() == "log" {
				t.Logf("[console] %v", ev)
			}
		}
	})
}

// TestPerConnectionStateIsolation verifies that two tabs on different servers
// have independent per-connection state. Tab 1 joins as "Alice" on server-a,
// Tab 2 on server-b should NOT see "Alice" as CurrentUser.
func TestPerConnectionStateIsolation(t *testing.T) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeWSURL)
	defer allocCancel()

	// Tab 1: connect to server-a, join as Alice
	tab1Ctx, tab1Cancel := newTab(t, allocCtx)
	defer tab1Cancel()

	err := chromedp.Run(tab1Ctx,
		chromedp.Navigate(serverAURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
		chromedp.SendKeys(`input[name="username"]`, "Alice", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="join"] button').click()`, nil),
		chromedp.WaitVisible(`#user`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 1 (server-a) join failed: %v", err)
	}

	var tab1User string
	if err := chromedp.Run(tab1Ctx, chromedp.Text(`#user`, &tab1User, chromedp.ByID)); err != nil {
		t.Fatalf("Tab 1 read user failed: %v", err)
	}
	if !strings.Contains(tab1User, "Alice") {
		t.Errorf("Tab 1 expected 'Alice', got %q", tab1User)
	}

	// Tab 2: connect to server-b — should show join form, NOT Alice's session
	tab2Ctx, tab2Cancel := newTab(t, allocCtx)
	defer tab2Cancel()

	err = chromedp.Run(tab2Ctx,
		chromedp.Navigate(serverBURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 2 (server-b) expected join form: %v", err)
	}

	// Verify Tab 2 does NOT show a logged-in user
	var joinFormVisible bool
	if err := chromedp.Run(tab2Ctx, chromedp.Evaluate(
		`document.querySelector('#join-form') !== null`, &joinFormVisible,
	)); err != nil {
		t.Fatalf("Tab 2 check failed: %v", err)
	}
	if !joinFormVisible {
		t.Error("Tab 2 should show join form (per-connection state), not a logged-in user")
	}
}

// TestCrossInstanceBroadcast verifies that a message sent on server-a
// reaches a tab connected to server-b via Redis PubSub.
func TestCrossInstanceBroadcast(t *testing.T) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeWSURL)
	defer allocCancel()

	// Tab 1: server-a, join as Alice, wait for chat UI
	tab1Ctx, tab1Cancel := newTab(t, allocCtx)
	defer tab1Cancel()

	err := chromedp.Run(tab1Ctx,
		chromedp.Navigate(serverAURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
		chromedp.SendKeys(`input[name="username"]`, "Alice", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="join"] button').click()`, nil),
		chromedp.WaitVisible(`#user`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 1 join failed: %v", err)
	}

	// Tab 2: server-b, join as Bob
	tab2Ctx, tab2Cancel := newTab(t, allocCtx)
	defer tab2Cancel()

	err = chromedp.Run(tab2Ctx,
		chromedp.Navigate(serverBURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
		chromedp.SendKeys(`input[name="username"]`, "Bob", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="join"] button').click()`, nil),
		chromedp.WaitVisible(`#user`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 2 join failed: %v", err)
	}

	// Tab 1 sends a message
	err = chromedp.Run(tab1Ctx,
		chromedp.SendKeys(`input[name="message"]`, "Hello from Alice!", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="send"] button').click()`, nil),
		// Wait for own message to appear
		chromedp.WaitVisible(`.msg`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Tab 1 send failed: %v", err)
	}

	// Tab 2 should receive the message via Redis PubSub cross-instance broadcast
	err = chromedp.Run(tab2Ctx,
		chromedp.WaitVisible(`.msg`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Tab 2 did not receive broadcast message: %v", err)
	}

	var tab2Messages string
	if err := chromedp.Run(tab2Ctx, chromedp.Text(`#messages`, &tab2Messages, chromedp.ByID)); err != nil {
		t.Fatalf("Tab 2 read messages failed: %v", err)
	}
	if !strings.Contains(tab2Messages, "Hello from Alice!") {
		t.Errorf("Tab 2 expected message from Alice, got %q", tab2Messages)
	}

	// Tab 2 should still be logged in as Bob (per-connection state preserved)
	var tab2User string
	if err := chromedp.Run(tab2Ctx, chromedp.Text(`#user`, &tab2User, chromedp.ByID)); err != nil {
		t.Fatalf("Tab 2 read user failed: %v", err)
	}
	if !strings.Contains(tab2User, "Bob") {
		t.Errorf("Tab 2 expected 'Bob' (per-connection preserved), got %q", tab2User)
	}
}

// TestBroadcastPreservesPerConnectionState verifies that broadcast actions
// don't overwrite per-connection fields. Each tab has its own CurrentUser
// and receiving a broadcast should NOT change it.
func TestBroadcastPreservesPerConnectionState(t *testing.T) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeWSURL)
	defer allocCancel()

	// Tab 1: server-a, join as Alice
	tab1Ctx, tab1Cancel := newTab(t, allocCtx)
	defer tab1Cancel()

	err := chromedp.Run(tab1Ctx,
		chromedp.Navigate(serverAURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
		chromedp.SendKeys(`input[name="username"]`, "Alice", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="join"] button').click()`, nil),
		chromedp.WaitVisible(`#user`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 1 join failed: %v", err)
	}

	// Tab 2: server-b, join as Bob
	tab2Ctx, tab2Cancel := newTab(t, allocCtx)
	defer tab2Cancel()

	err = chromedp.Run(tab2Ctx,
		chromedp.Navigate(serverBURL),
		chromedp.WaitVisible(`#join-form`, chromedp.ByID),
		chromedp.SendKeys(`input[name="username"]`, "Bob", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="join"] button').click()`, nil),
		chromedp.WaitVisible(`#user`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 2 join failed: %v", err)
	}

	// Alice sends a message → triggers broadcast
	err = chromedp.Run(tab1Ctx,
		chromedp.SendKeys(`input[name="message"]`, "test msg", chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('form[name="send"] button').click()`, nil),
		chromedp.WaitVisible(`.msg`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Tab 1 send failed: %v", err)
	}

	// Wait for broadcast to reach Tab 2
	err = chromedp.Run(tab2Ctx,
		chromedp.WaitVisible(`.msg`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Tab 2 did not receive broadcast: %v", err)
	}

	// Verify Tab 1 is still Alice
	var user1 string
	chromedp.Run(tab1Ctx, chromedp.Text(`#user`, &user1, chromedp.ByID))
	if !strings.Contains(user1, "Alice") {
		t.Errorf("Tab 1 CurrentUser should be Alice, got %q", user1)
	}

	// Verify Tab 2 is still Bob (NOT overwritten by Alice's broadcast)
	var user2 string
	chromedp.Run(tab2Ctx, chromedp.Text(`#user`, &user2, chromedp.ByID))
	if !strings.Contains(user2, "Bob") {
		t.Errorf("Tab 2 CurrentUser should be Bob, got %q", user2)
	}
}

// TestInstanceIdentification verifies that tabs connected to different servers
// show the correct instance ID, confirming they are truly on different servers.
func TestInstanceIdentification(t *testing.T) {
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeWSURL)
	defer allocCancel()

	// Tab 1: server-a
	tab1Ctx, tab1Cancel := newTab(t, allocCtx)
	defer tab1Cancel()

	err := chromedp.Run(tab1Ctx,
		chromedp.Navigate(serverAURL),
		chromedp.WaitVisible(`#instance`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 1 navigate failed: %v", err)
	}

	var instance1 string
	chromedp.Run(tab1Ctx, chromedp.Text(`#instance`, &instance1, chromedp.ByID))
	if !strings.Contains(instance1, "server-a") {
		t.Errorf("Tab 1 expected instance server-a, got %q", instance1)
	}

	// Tab 2: server-b
	tab2Ctx, tab2Cancel := newTab(t, allocCtx)
	defer tab2Cancel()

	err = chromedp.Run(tab2Ctx,
		chromedp.Navigate(serverBURL),
		chromedp.WaitVisible(`#instance`, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Tab 2 navigate failed: %v", err)
	}

	var instance2 string
	chromedp.Run(tab2Ctx, chromedp.Text(`#instance`, &instance2, chromedp.ByID))
	if !strings.Contains(instance2, "server-b") {
		t.Errorf("Tab 2 expected instance server-b, got %q", instance2)
	}
}
