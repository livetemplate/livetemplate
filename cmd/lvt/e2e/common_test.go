package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// startDockerChrome starts Chrome in Docker for E2E testing
func startDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
	t.Helper()

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping E2E test")
	}

	dockerImage := "chromedp/headless-shell:latest"

	// Pull image if needed
	checkCmd := exec.Command("docker", "image", "inspect", dockerImage)
	if err := checkCmd.Run(); err != nil {
		t.Log("Pulling Chrome Docker image...")
		pullCmd := exec.Command("docker", "pull", dockerImage)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			t.Fatalf("Failed to pull Docker image: %v", err)
		}
	}

	// Start container
	t.Log("Starting Chrome Docker container...")
	portMapping := fmt.Sprintf("%d:9222", debugPort)
	containerName := fmt.Sprintf("lvt-e2e-chrome-%d", debugPort)

	var cmd *exec.Cmd
	if goruntime.GOOS == "linux" {
		cmd = exec.Command("docker", "run", "--rm",
			"--network", "host",
			"--name", containerName,
			dockerImage,
		)
	} else {
		cmd = exec.Command("docker", "run", "--rm",
			"-p", portMapping,
			"--name", containerName,
			"--add-host", "host.docker.internal:host-gateway",
			dockerImage,
		)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Chrome: %v", err)
	}

	// Wait for Chrome to be ready
	chromeURL := fmt.Sprintf("http://localhost:%d/json/version", debugPort)
	for i := 0; i < 60; i++ {
		resp, err := http.Get(chromeURL)
		if err == nil {
			resp.Body.Close()
			t.Log("✅ Chrome ready")
			return cmd
		}
		time.Sleep(500 * time.Millisecond)
	}

	cmd.Process.Kill()
	t.Fatal("Chrome failed to start")
	return nil
}

// stopDockerChrome stops the Chrome container
func stopDockerChrome(t *testing.T, cmd *exec.Cmd, debugPort int) {
	t.Helper()
	containerName := fmt.Sprintf("lvt-e2e-chrome-%d", debugPort)

	// Stop the container and wait for it to complete
	stopCmd := exec.Command("docker", "stop", containerName)
	if err := stopCmd.Run(); err != nil {
		// Container might already be stopped, that's okay
		t.Logf("Docker stop returned: %v", err)
	}

	// Wait for the process to fully exit
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait() // Wait for process to complete I/O
	}
}

// getTestURL returns the URL for Chrome to access the test server
func getTestURL(port int) string {
	if goruntime.GOOS == "linux" {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}

// waitForWebSocketReady waits for the first WebSocket update to be applied
// by polling for the removal of data-lvt-loading attribute (condition-based waiting).
// This ensures E2E tests run after the WebSocket connection is established and
// the initial tree update has been applied to the DOM.
//
// The client removes data-lvt-loading after receiving the first WebSocket message,
// which makes this a reliable signal that the page is in its final state.
func waitForWebSocketReady(timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// First wait for wrapper to exist
		if err := chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery).Do(ctx); err != nil {
			return fmt.Errorf("wrapper element not found: %w", err)
		}

		// Poll for data-lvt-loading attribute removal (condition-based waiting)
		startTime := time.Now()
		for {
			var loadingRemoved bool
			err := chromedp.Evaluate(`
				(() => {
					const wrapper = document.querySelector('[data-lvt-id]');
					return wrapper && !wrapper.hasAttribute('data-lvt-loading');
				})()
			`, &loadingRemoved).Do(ctx)

			if err != nil {
				return fmt.Errorf("failed to check loading state: %w", err)
			}

			if loadingRemoved {
				// Loading indicator removed - WebSocket update applied
				return nil
			}

			if time.Since(startTime) > timeout {
				// Gather diagnostic information before timing out
				var wsState string
				chromedp.Evaluate(`
					(() => {
						const ws = window.livetemplate && window.livetemplate.ws;
						if (!ws) return "WebSocket object not found";
						const states = {0: "CONNECTING", 1: "OPEN", 2: "CLOSING", 3: "CLOSED"};
						return "State: " + (states[ws.readyState] || ws.readyState);
					})()
				`, &wsState).Do(ctx)

				var jsErrors string
				chromedp.Evaluate(`
					(() => {
						return window.lastError || "No JS errors captured";
					})()
				`, &jsErrors).Do(ctx)

				return fmt.Errorf("timeout waiting for WebSocket ready (data-lvt-loading not removed after %v). WebSocket: %s, JS Errors: %s", timeout, wsState, jsErrors)
			}

			// Poll every 10ms (condition-based, not arbitrary)
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// validateNoTemplateExpressions checks that the specified element does not contain
// raw Go template expressions like {{if}}, {{range}}, {{define}}, etc.
// This catches the bug where unflattened templates are used in WebSocket tree generation.
func validateNoTemplateExpressions(selector string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var innerHTML string
		if err := chromedp.InnerHTML(selector, &innerHTML, chromedp.ByQuery).Do(ctx); err != nil {
			return fmt.Errorf("failed to get innerHTML of %s: %w", selector, err)
		}

		// Check for common template expressions
		templateExpressions := []string{
			"{{if",
			"{{range",
			"{{define",
			"{{template",
			"{{with",
			"{{block",
			"{{else",
			"{{end}}",
		}

		for _, expr := range templateExpressions {
			if strings.Contains(innerHTML, expr) {
				// Find context around the expression for better error messages
				idx := strings.Index(innerHTML, expr)
				start := idx - 50
				if start < 0 {
					start = 0
				}
				end := idx + 100
				if end > len(innerHTML) {
					end = len(innerHTML)
				}
				context := innerHTML[start:end]

				return fmt.Errorf("raw template expression '%s' found in HTML. Context: ...%s...", expr, context)
			}
		}

		return nil
	})
}
