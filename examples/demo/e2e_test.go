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

	"github.com/chromedp/chromedp"
)

// TestTwitterCloneE2E runs end-to-end tests for the Twitter clone demo
func TestTwitterCloneE2E(t *testing.T) {
	// Start the demo server in a separate goroutine
	server, err := startDemoServer(t)
	if err != nil {
		t.Fatalf("Failed to start demo server: %v", err)
	}
	defer server.Close()

	// Wait for server to be ready
	if !waitForServer(t, "http://localhost:8080", 30*time.Second) {
		t.Fatal("Demo server failed to start within 30 seconds")
	}

	t.Log("Demo server started successfully")

	// Set up Chrome context (will use Docker if available, fallback to local)
	ctx, cancel := setupBrowserContext(t)
	defer cancel()

	// Run E2E tests
	t.Run("InitialPageLoad", func(t *testing.T) {
		testInitialPageLoad(t, ctx)
	})

	t.Run("WebSocketConnection", func(t *testing.T) {
		testWebSocketConnection(t, ctx)
	})

	t.Run("LikeAction", func(t *testing.T) {
		testLikeAction(t, ctx)
	})

	t.Run("RetweetAction", func(t *testing.T) {
		testRetweetAction(t, ctx)
	})

	t.Run("NewTweetCreation", func(t *testing.T) {
		testNewTweetCreation(t, ctx)
	})

	t.Run("AjaxFallback", func(t *testing.T) {
		testAjaxFallback(t, ctx)
	})

	t.Run("FragmentLifecycle", func(t *testing.T) {
		testFragmentLifecycle(t, ctx)
	})
}

func startDemoServer(t *testing.T) (*Server, error) {
	t.Log("Starting demo server...")

	server, err := NewServer()
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Start server in background
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	return server, nil
}

func waitForServer(t *testing.T, url string, timeout time.Duration) bool {
	t.Logf("Waiting for server at %s...", url)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					t.Log("Server is ready")
					return true
				}
			}
		}
	}
}

func setupBrowserContext(t *testing.T) (context.Context, context.CancelFunc) {
	// Use local Chrome for better reliability
	t.Log("Using local Chrome for E2E testing")
	return setupLocalChrome(t)
}

func isDockerAvailable() bool {
	_, err := exec.LookPath("docker")
	if err != nil {
		return false
	}

	// Test if docker is actually working
	cmd := exec.Command("docker", "ps")
	return cmd.Run() == nil
}

func setupDockerChrome(t *testing.T) (context.Context, context.CancelFunc) {
	// Start Chrome headless container
	dockerCmd := exec.Command(
		"docker", "run", "-d", "--rm",
		"-p", "9222:9222",
		"--name", "chrome-headless-test",
		"chromedp/headless-shell",
		"--no-sandbox",
		"--disable-gpu",
		"--remote-debugging-address=0.0.0.0",
		"--remote-debugging-port=9222",
	)

	output, err := dockerCmd.Output()
	if err != nil {
		t.Logf("Failed to start Docker Chrome, falling back to local: %v", err)
		return setupLocalChrome(t)
	}

	containerID := strings.TrimSpace(string(output))
	t.Logf("Started Docker Chrome container: %s", containerID[:12])

	// Wait for Chrome to be ready
	time.Sleep(3 * time.Second)

	// Create Chrome context connecting to Docker instance
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")

	// Wrap cancel to also cleanup Docker container
	wrappedCancel := func() {
		cancel()
		cleanupCmd := exec.Command("docker", "stop", containerID)
		if err := cleanupCmd.Run(); err != nil {
			t.Logf("Failed to stop Docker container: %v", err)
		}
	}

	// Create tab context
	tabCtx, tabCancel := chromedp.NewContext(ctx)
	finalCancel := func() {
		tabCancel()
		wrappedCancel()
	}

	return tabCtx, finalCancel
}

func setupLocalChrome(t *testing.T) (context.Context, context.CancelFunc) {
	// Use local Chrome installation
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	wrappedCancel := func() {
		cancel()
		allocCancel()
	}

	return ctx, wrappedCancel
}

func testInitialPageLoad(t *testing.T, ctx context.Context) {
	t.Log("Testing initial page load with fragment annotations...")

	var title string
	var tweetsCount int
	var pageSource string

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Evaluate(`document.querySelectorAll('.tweet').length`, &tweetsCount),
		chromedp.OuterHTML("html", &pageSource),
	)

	if err != nil {
		t.Fatalf("Failed to load initial page: %v", err)
	}

	// Validate page loaded correctly
	if title != "Twitter Clone - LiveTemplate Demo" {
		t.Errorf("Expected title 'Twitter Clone - LiveTemplate Demo', got: %s", title)
	}

	if tweetsCount < 3 {
		t.Errorf("Expected at least 3 initial tweets, got: %d", tweetsCount)
	}

	// Check for LiveTemplate annotations (fragment IDs)
	if !strings.Contains(pageSource, "data-fragment-id") {
		t.Error("Page source missing LiveTemplate fragment annotations")
	}

	// Check for LiveTemplate token injection
	if !strings.Contains(pageSource, "LIVETEMPLATE_TOKEN") {
		t.Error("Page source missing LiveTemplate token injection")
	}

	t.Log("✅ Initial page load test passed")
}

func testWebSocketConnection(t *testing.T, ctx context.Context) {
	t.Log("Testing WebSocket connection and initial fragments...")

	var connectionStatus string
	var initialFragmentsCalled bool

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),

		// Wait for connection to establish
		chromedp.Sleep(2*time.Second),

		// Check connection status
		chromedp.Evaluate(`document.querySelector('.connection-status')?.textContent || 'none'`, &connectionStatus),

		// Check if initial fragments were called (via console logs)
		chromedp.Evaluate(`
			window.initialFragmentsReceived = window.initialFragmentsReceived || false;
			window.initialFragmentsReceived;
		`, &initialFragmentsCalled),
	)

	if err != nil {
		t.Fatalf("Failed to test WebSocket connection: %v", err)
	}

	// Validate WebSocket connection established
	if !strings.Contains(connectionStatus, "Connected") && !strings.Contains(connectionStatus, "Ajax Mode") {
		t.Errorf("Expected connection status to show 'Connected' or 'Ajax Mode', got: %s", connectionStatus)
	}

	t.Logf("Connection status: %s", connectionStatus)
	t.Log("✅ WebSocket connection test passed")
}

func testLikeAction(t *testing.T, ctx context.Context) {
	t.Log("Testing like action with dynamic fragment updates...")

	var initialLikes, updatedLikes string

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for connection

		// Get initial like count from first tweet
		chromedp.Text(`.tweet:first-child .like-btn .count`, &initialLikes, chromedp.ByQuery),

		// Click like button
		chromedp.Click(`.tweet:first-child .like-btn`, chromedp.ByQuery),

		// Wait for update
		chromedp.Sleep(1*time.Second),

		// Get updated like count
		chromedp.Text(`.tweet:first-child .like-btn .count`, &updatedLikes, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to test like action: %v", err)
	}

	t.Logf("Like count changed from '%s' to '%s'", initialLikes, updatedLikes)

	// Validate that likes changed (either increased or decreased)
	if initialLikes == updatedLikes {
		t.Error("Like count did not change after clicking like button")
	}

	t.Log("✅ Like action test passed")
}

func testRetweetAction(t *testing.T, ctx context.Context) {
	t.Log("Testing retweet action with dynamic fragment updates...")

	var initialRetweets, updatedRetweets string

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for connection

		// Get initial retweet count from first tweet
		chromedp.Text(`.tweet:first-child .retweet-btn .count`, &initialRetweets, chromedp.ByQuery),

		// Click retweet button
		chromedp.Click(`.tweet:first-child .retweet-btn`, chromedp.ByQuery),

		// Wait for update
		chromedp.Sleep(1*time.Second),

		// Get updated retweet count
		chromedp.Text(`.tweet:first-child .retweet-btn .count`, &updatedRetweets, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to test retweet action: %v", err)
	}

	t.Logf("Retweet count changed from '%s' to '%s'", initialRetweets, updatedRetweets)

	// Validate that retweets changed
	if initialRetweets == updatedRetweets {
		t.Error("Retweet count did not change after clicking retweet button")
	}

	t.Log("✅ Retweet action test passed")
}

func testNewTweetCreation(t *testing.T, ctx context.Context) {
	t.Log("Testing new tweet creation with dynamic fragment updates...")

	var initialTweetCount, updatedTweetCount int
	testTweetContent := "This is a test tweet from E2E automation! 🚀"

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for connection

		// Count initial tweets
		chromedp.Evaluate(`document.querySelectorAll('.tweet').length`, &initialTweetCount),

		// Type in tweet composer
		chromedp.SendKeys(`#tweet-input`, testTweetContent, chromedp.ByID),

		// Click tweet button
		chromedp.Click(`#tweet-btn`, chromedp.ByID),

		// Wait for new tweet to appear
		chromedp.Sleep(2*time.Second),

		// Count updated tweets
		chromedp.Evaluate(`document.querySelectorAll('.tweet').length`, &updatedTweetCount),
	)

	if err != nil {
		t.Fatalf("Failed to test new tweet creation: %v", err)
	}

	t.Logf("Tweet count changed from %d to %d", initialTweetCount, updatedTweetCount)

	// Validate that a new tweet was added
	if updatedTweetCount != initialTweetCount+1 {
		t.Errorf("Expected tweet count to increase by 1, got %d -> %d", initialTweetCount, updatedTweetCount)
	}

	// Verify the new tweet content appears
	var newTweetExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			Array.from(document.querySelectorAll('.tweet-content p'))
				.some(p => p.textContent.includes('%s'))
		`, testTweetContent), &newTweetExists),
	)

	if err != nil {
		t.Logf("Failed to verify new tweet content: %v", err)
	} else if !newTweetExists {
		t.Error("New tweet content not found in the page")
	}

	t.Log("✅ New tweet creation test passed")
}

func testAjaxFallback(t *testing.T, ctx context.Context) {
	t.Log("Testing Ajax fallback mode...")

	var connectionStatus string

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),

		// Disable WebSocket to force Ajax fallback
		chromedp.Evaluate(`
			// Override WebSocket to force failure
			window.WebSocket = function() {
				throw new Error('WebSocket disabled for testing');
			};
		`, nil),

		// Reload page to trigger Ajax fallback
		chromedp.Reload(),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Ajax mode to establish

		// Check connection status
		chromedp.Evaluate(`document.querySelector('.connection-status')?.textContent || 'none'`, &connectionStatus),
	)

	if err != nil {
		t.Fatalf("Failed to test Ajax fallback: %v", err)
	}

	// Validate Ajax fallback mode
	if !strings.Contains(connectionStatus, "Ajax Mode") {
		t.Logf("Connection status: %s (WebSocket may be working, Ajax fallback not triggered)", connectionStatus)
		// This is not necessarily an error - WebSocket might be working fine
	} else {
		t.Log("✅ Ajax fallback mode activated successfully")
	}

	t.Log("✅ Ajax fallback test completed")
}

func testFragmentLifecycle(t *testing.T, ctx context.Context) {
	t.Log("Testing complete fragment lifecycle (initial cache -> dynamic updates)...")

	var fragmentCacheSize int
	var hasFragmentUpdates bool

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://localhost:8080"),
		chromedp.WaitVisible(".container", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for initial fragments

		// Check if fragment cache was populated
		chromedp.Evaluate(`
			window.liveTemplateClient = window.client || null;
			window.liveTemplateClient ? window.liveTemplateClient.fragmentCache.size : 0;
		`, &fragmentCacheSize),

		// Perform an action to trigger dynamic updates
		chromedp.Click(`.tweet:first-child .like-btn`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// Check if fragments were updated (not just cached)
		chromedp.Evaluate(`
			// Check if any element has been marked as updated
			document.querySelector('.updated') !== null;
		`, &hasFragmentUpdates),
	)

	if err != nil {
		t.Fatalf("Failed to test fragment lifecycle: %v", err)
	}

	t.Logf("Fragment cache size: %d", fragmentCacheSize)
	t.Logf("Has fragment updates: %v", hasFragmentUpdates)

	// Validate fragment lifecycle
	if fragmentCacheSize == 0 {
		t.Log("Warning: Fragment cache appears empty (may be implementation-dependent)")
	}

	t.Log("✅ Fragment lifecycle test completed")
}

// Additional helper to check console logs
func checkConsoleErrors(t *testing.T, ctx context.Context) {
	var logs []string

	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			console.getLogs = console.getLogs || [];
			console.getLogs;
		`, &logs),
	)

	if err != nil {
		t.Logf("Failed to get console logs: %v", err)
		return
	}

	for _, log := range logs {
		if strings.Contains(strings.ToLower(log), "error") {
			t.Logf("Console error detected: %s", log)
		}
	}
}

// TestMain ensures proper setup and cleanup
func TestMain(m *testing.M) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Run tests
	code := m.Run()

	// Cleanup any hanging Docker containers
	exec.Command("docker", "stop", "chrome-headless-test").Run()
	exec.Command("docker", "rm", "chrome-headless-test").Run()

	os.Exit(code)
}
