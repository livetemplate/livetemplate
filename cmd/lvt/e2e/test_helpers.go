package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// AppOptions contains options for creating a test app
type AppOptions struct {
	Kit     string // Kit name (multi, single, simple)
	Module  string // Go module name
	DevMode bool   // Use local client library
}

// buildLvtBinary builds the lvt binary in the temp directory
func buildLvtBinary(t *testing.T, tmpDir string) string {
	t.Helper()
	t.Log("Building lvt binary...")

	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	t.Log("✅ lvt binary built")
	return lvtBinary
}

// runLvtCommand executes an lvt command with args and returns error if it fails
func runLvtCommand(t *testing.T, lvtBinary, workDir string, args ...string) error {
	t.Helper()
	t.Logf("Running: lvt %s", strings.Join(args, " "))

	cmd := exec.Command(lvtBinary, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: lvt %s: %w", strings.Join(args, " "), err)
	}

	return nil
}

// createTestApp creates a new test application and sets it up for testing
func createTestApp(t *testing.T, lvtBinary, tmpDir, appName string, opts *AppOptions) string {
	t.Helper()
	t.Logf("Creating test app: %s", appName)

	// Set defaults
	if opts == nil {
		opts = &AppOptions{
			Kit:     "multi",
			DevMode: true,
		}
	}

	// Build lvt new command
	args := []string{"new", appName}

	if opts.Kit != "" && opts.Kit != "multi" {
		args = append(args, "--kit", opts.Kit)
	}

	if opts.Module != "" {
		args = append(args, "--module", opts.Module)
	}

	if opts.DevMode {
		args = append(args, "--dev")
	}

	// Create app
	if err := runLvtCommand(t, lvtBinary, tmpDir, args...); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	appDir := filepath.Join(tmpDir, appName)

	// Add replace directive to use local livetemplate (for testing with latest changes)
	cwd, _ := os.Getwd()
	livetemplatePath := filepath.Join(cwd, "..", "..", "..")

	replaceCmd := exec.Command("go", "mod", "edit", fmt.Sprintf("-replace=github.com/livefir/livetemplate=%s", livetemplatePath))
	replaceCmd.Dir = appDir
	if err := replaceCmd.Run(); err != nil {
		t.Fatalf("Failed to add replace directive: %v", err)
	}

	// Run go mod tidy
	t.Log("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = appDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}

	// Copy client library for dev mode
	if opts.DevMode {
		t.Log("Copying client library...")
		clientSrc := "../../../client/dist/livetemplate-client.browser.js"
		clientDst := filepath.Join(appDir, "livetemplate-client.js")
		clientContent, err := os.ReadFile(clientSrc)
		if err != nil {
			t.Fatalf("Failed to read client library: %v", err)
		}
		if err := os.WriteFile(clientDst, clientContent, 0644); err != nil {
			t.Fatalf("Failed to write client library: %v", err)
		}
		t.Logf("✅ Client library copied (%d bytes)", len(clientContent))
	}

	t.Log("✅ Test app created")
	return appDir
}

// runSqlcGenerate runs sqlc generate to generate database code
func runSqlcGenerate(t *testing.T, appDir string) {
	t.Helper()
	t.Log("Running sqlc generate...")

	sqlcCmd := exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@latest", "generate", "-f", "internal/database/sqlc.yaml")
	sqlcCmd.Dir = appDir
	sqlcCmd.Stdout = os.Stdout
	sqlcCmd.Stderr = os.Stderr
	if err := sqlcCmd.Run(); err != nil {
		t.Fatalf("Failed to run sqlc generate: %v", err)
	}
	t.Log("✅ sqlc generate complete")
}

// buildGeneratedApp builds the generated application binary
func buildGeneratedApp(t *testing.T, appDir string) string {
	t.Helper()
	t.Log("Building generated app...")

	appName := filepath.Base(appDir)
	appBinary := filepath.Join(appDir, appName)

	buildCmd := exec.Command("go", "build", "-o", appBinary, "./cmd/"+appName)
	buildCmd.Dir = appDir

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("❌ Generated app failed to compile: %v\n%s", err, output)
	}

	t.Log("✅ Generated app compiled successfully")
	return appBinary
}

// startAppServer starts the application server on the given port
func startAppServer(t *testing.T, appBinary string, port int) *exec.Cmd {
	t.Helper()
	t.Logf("Starting app server on port %d...", port)

	cmd := exec.Command(appBinary)
	cmd.Dir = filepath.Dir(appBinary)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	t.Logf("✅ Server started (PID: %d)", cmd.Process.Pid)
	return cmd
}

// waitForServer waits for the server to be ready and responding
func waitForServer(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	t.Logf("Waiting for server at %s...", url)

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				t.Log("✅ Server ready")
				return
			}
			lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("❌ Server failed to respond within %v. Last error: %v", timeout, lastErr)
}

// verifyNoTemplateErrors checks that the page has no template errors
func verifyNoTemplateErrors(t *testing.T, ctx context.Context, url string) {
	t.Helper()

	var bodyText string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
		chromedp.Text("body", &bodyText, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	// Check for common template error patterns
	errorPatterns := []string{
		"template:",
		"<no value>",
		"{{.",
		"executing template",
		"parse error",
	}

	for _, pattern := range errorPatterns {
		if strings.Contains(bodyText, pattern) {
			t.Errorf("❌ Template error found on page: contains %q", pattern)
		}
	}
}

// verifyWebSocketConnected checks that WebSocket connection is established
func verifyWebSocketConnected(t *testing.T, ctx context.Context, url string) {
	t.Helper()

	var wsConnected bool
	var wsURL string
	var wsReadyState int

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
		chromedp.Evaluate(`window.liveTemplateClient && window.liveTemplateClient.ws ? window.liveTemplateClient.ws.url : null`, &wsURL),
		chromedp.Evaluate(`window.liveTemplateClient && window.liveTemplateClient.ws ? window.liveTemplateClient.ws.readyState : -1`, &wsReadyState),
		chromedp.Evaluate(`(() => {
			return window.liveTemplateClient &&
			       window.liveTemplateClient.ws &&
			       window.liveTemplateClient.ws.readyState === WebSocket.OPEN;
		})()`, &wsConnected),
	)
	if err != nil {
		t.Fatalf("Failed to check WebSocket: %v", err)
	}

	t.Logf("WebSocket URL: %s, ReadyState: %d (1=OPEN)", wsURL, wsReadyState)

	if !wsConnected {
		t.Errorf("❌ WebSocket not connected (readyState: %d)", wsReadyState)
	} else {
		t.Log("✅ WebSocket connected")
	}
}

// readLvtrc reads and parses the .lvtrc file
func readLvtrc(t *testing.T, appDir string) (kit string) {
	t.Helper()

	lvtrcPath := filepath.Join(appDir, ".lvtrc")
	content, err := os.ReadFile(lvtrcPath)
	if err != nil {
		t.Fatalf("Failed to read .lvtrc: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kit=") {
			kit = strings.TrimPrefix(line, "kit=")
		}
	}

	return kit
}
