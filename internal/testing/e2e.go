package testing

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	dockerImage = "chromedp/headless-shell:latest"
)

// GetFreePort asks the kernel for a free open port that is ready to use
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// GetChromeTestURL returns the URL for Chrome (in Docker) to access the test server
// On Linux with host networking: use localhost
// On macOS/Windows: use host.docker.internal
func GetChromeTestURL(port int) string {
	portStr := fmt.Sprintf("%d", port)
	if runtime.GOOS == "linux" {
		return "http://localhost:" + portStr
	}
	return "http://host.docker.internal:" + portStr
}

// StartDockerChrome starts the chromedp headless-shell Docker container
func StartDockerChrome(t *testing.T, debugPort int) *exec.Cmd {
	t.Helper()

	// Check if Docker is available
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("Docker not available, skipping E2E test")
	}

	// Check if image exists, if not try to pull it (with timeout)
	checkCmd := exec.Command("docker", "image", "inspect", dockerImage)
	if err := checkCmd.Run(); err != nil {
		// Image doesn't exist, try to pull with timeout
		t.Log("Pulling chromedp/headless-shell Docker image...")
		pullCmd := exec.Command("docker", "pull", dockerImage)
		if err := pullCmd.Start(); err != nil {
			t.Fatalf("Failed to start docker pull: %v", err)
		}

		// Wait for pull with timeout
		pullDone := make(chan error, 1)
		go func() {
			pullDone <- pullCmd.Wait()
		}()

		select {
		case err := <-pullDone:
			if err != nil {
				t.Fatalf("Failed to pull Docker image: %v", err)
			}
			t.Log("✅ Docker image pulled successfully")
		case <-time.After(60 * time.Second):
			pullCmd.Process.Kill()
			t.Fatal("Docker pull timed out after 60 seconds")
		}
	} else {
		t.Log("✅ Docker image already exists, skipping pull")
	}

	// Start the container
	t.Log("Starting Chrome headless Docker container...")
	var cmd *exec.Cmd
	portMapping := fmt.Sprintf("%d:9222", debugPort)
	containerName := fmt.Sprintf("chrome-e2e-test-%d", debugPort) // Unique name per test

	if runtime.GOOS == "linux" {
		// On Linux, use host networking so container can access localhost
		cmd = exec.Command("docker", "run", "--rm",
			"--network", "host",
			"--name", containerName,
			dockerImage,
		)
	} else {
		// On macOS/Windows, map port for remote debugging
		// (container will use host.docker.internal to reach host)
		// Note: Don't pass Chrome flags - the image has a built-in setup
		cmd = exec.Command("docker", "run", "--rm",
			"-p", portMapping,
			"--name", containerName,
			"--add-host", "host.docker.internal:host-gateway",
			dockerImage,
		)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start Chrome Docker container: %v", err)
	}

	// Wait for Chrome to be ready (increased timeout for slower systems)
	t.Log("Waiting for Chrome to be ready...")
	chromeURL := fmt.Sprintf("http://localhost:%d/json/version", debugPort)
	ready := false
	for i := 0; i < 60; i++ { // 60 iterations × 500ms = 30 seconds
		resp, err := http.Get(chromeURL)
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !ready {
		cmd.Process.Kill()
		t.Fatal("Chrome failed to start within 30 seconds")
	}

	t.Log("✅ Chrome headless Docker container ready")
	return cmd
}

// StopDockerChrome stops the Chrome Docker container
func StopDockerChrome(t *testing.T, cmd *exec.Cmd, debugPort int) {
	t.Helper()
	t.Log("Stopping Chrome Docker container...")

	containerName := fmt.Sprintf("chrome-e2e-test-%d", debugPort)

	// Check if container exists before trying to stop it
	filterName := fmt.Sprintf("name=%s", containerName)
	checkCmd := exec.Command("docker", "ps", "-a", "-q", "-f", filterName)
	output, _ := checkCmd.Output()

	if len(output) > 0 {
		// Container exists, stop it gracefully with timeout
		stopCmd := exec.Command("docker", "stop", "-t", "2", containerName)
		stopDone := make(chan error, 1)
		go func() {
			stopDone <- stopCmd.Run()
		}()

		// Wait for stop with 5 second timeout
		select {
		case err := <-stopDone:
			if err != nil {
				t.Logf("Warning: Failed to stop Docker container: %v", err)
			}
		case <-time.After(5 * time.Second):
			// Force kill if graceful stop hangs
			t.Logf("Warning: docker stop timed out, forcing kill")
			exec.Command("docker", "kill", containerName).Run()
		}
	}

	// Kill the process if still running
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// StartTestServer starts a Go server on the specified port
// mainPath should be the path to main.go (e.g., "main.go" or "../../examples/counter/main.go")
func StartTestServer(t *testing.T, mainPath string, port int) *exec.Cmd {
	t.Helper()

	portStr := fmt.Sprintf("%d", port)
	serverURL := fmt.Sprintf("http://localhost:%d", port)

	t.Logf("Starting test server on port %s", portStr)
	cmd := exec.Command("go", "run", mainPath)
	cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)

	// Start the server
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

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
		cmd.Process.Kill()
		t.Fatal("Server failed to start within 5 seconds")
	}

	t.Logf("✅ Test server ready at %s", serverURL)
	return cmd
}

// ServeClientLibrary serves the LiveTemplate client browser bundle
// This is for development/testing purposes only. In production, serve from CDN.
func ServeClientLibrary(w http.ResponseWriter, r *http.Request) {
	// Try multiple paths for the client library
	paths := []string{
		"client/dist/livetemplate-client.browser.js",
		"../../client/dist/livetemplate-client.browser.js",
		"../client/dist/livetemplate-client.browser.js",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
	}

	http.Error(w, "Client library not found", http.StatusNotFound)
}

// WaitForWebSocketReady waits for the first WebSocket update to be applied
// by polling for the removal of data-lvt-loading attribute (condition-based waiting).
// This ensures E2E tests run after the WebSocket connection is established and
// the initial tree update has been applied to the DOM.
//
// The client removes data-lvt-loading after receiving the first WebSocket message,
// which makes this a reliable signal that the page is in its final state.
func WaitForWebSocketReady(timeout time.Duration) chromedp.Action {
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
				return fmt.Errorf("timeout waiting for WebSocket ready (data-lvt-loading not removed after %v)", timeout)
			}

			// Poll every 10ms (condition-based, not arbitrary)
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// ValidateNoTemplateExpressions checks that the specified element does not contain
// raw Go template expressions like {{if}}, {{range}}, {{define}}, etc.
// This catches the bug where unflattened templates are used in WebSocket tree generation.
func ValidateNoTemplateExpressions(selector string) chromedp.Action {
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
