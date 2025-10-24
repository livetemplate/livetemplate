package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/livefir/livetemplate/cmd/lvt/commands"
	"github.com/livefir/livetemplate/cmd/lvt/internal/serve"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// chdirMutex protects os.Chdir calls in parallel tests
// os.Chdir affects the entire process, so we need to serialize these operations
var chdirMutex sync.Mutex

// goModMutex protects go mod tidy operations in parallel tests
// go mod tidy operations can interfere with each other through the shared Go module cache
var goModMutex sync.Mutex

// runGoModTidy runs go mod tidy with mutex protection to avoid race conditions
func runGoModTidy(t *testing.T, dir string) error {
	t.Helper()

	goModMutex.Lock()
	defer goModMutex.Unlock()

	t.Log("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyCmd.Env = append(os.Environ(), "GOWORK=off") // Disable workspace mode to avoid conflicts

	output, err := tidyCmd.CombinedOutput()
	if err != nil {
		t.Logf("go mod tidy failed: %v\nOutput: %s", err, string(output))
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// AppOptions contains options for creating a test app
type AppOptions struct {
	Kit     string // Kit name (multi, single, simple)
	Module  string // Go module name
	DevMode bool   // Use local client library
}

// runLvtCommand executes an lvt command directly by calling the command functions
// This is much faster than shelling out and avoids working directory issues
func runLvtCommand(t *testing.T, workDir string, args ...string) error {
	t.Helper()
	_, err := runLvtCommandWithOutput(t, workDir, args...)
	return err
}

// runLvtCommandWithOutput executes an lvt command and returns its output
func runLvtCommandWithOutput(t *testing.T, workDir string, args ...string) (string, error) {
	t.Helper()
	t.Logf("Running: lvt %s", strings.Join(args, " "))

	if len(args) == 0 {
		return "", fmt.Errorf("no command specified")
	}

	// Lock to serialize directory changes across parallel tests
	// os.Chdir affects the entire process, not just the current goroutine
	chdirMutex.Lock()
	defer chdirMutex.Unlock()

	// Save and restore working directory
	origDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if workDir != "" {
		if err := os.Chdir(workDir); err != nil {
			return "", fmt.Errorf("failed to change directory to %s: %w", workDir, err)
		}
	}

	// Save and restore stdout/stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Capture output to buffer
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	var outputBuf strings.Builder
	outputDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&outputBuf, r)
		close(outputDone)
	}()

	// Execute the command directly
	command := args[0]
	cmdArgs := args[1:]

	var cmdErr error
	switch command {
	case "new":
		cmdErr = commands.New(cmdArgs)
	case "gen":
		cmdErr = commands.Gen(cmdArgs)
	case "migration":
		cmdErr = commands.Migration(cmdArgs)
	case "kits", "kit":
		cmdErr = commands.Kits(cmdArgs)
	case "serve":
		cmdErr = commands.Serve(cmdArgs)
	case "resource", "res":
		cmdErr = commands.Resource(cmdArgs)
	case "seed":
		cmdErr = commands.Seed(cmdArgs)
	case "parse":
		cmdErr = commands.Parse(cmdArgs)
	default:
		cmdErr = fmt.Errorf("unknown command: %s", command)
	}

	// Restore stdout/stderr and wait for output
	w.Close()
	<-outputDone

	output := outputBuf.String()

	if cmdErr != nil {
		return output, fmt.Errorf("command failed: lvt %s: %w", strings.Join(args, " "), cmdErr)
	}

	return output, nil
}

// ServerHandle provides control over a running test server
type ServerHandle struct {
	server  interface{ Shutdown() error }
	cancel  context.CancelFunc
	errChan chan error
}

// Shutdown stops the server gracefully
func (h *ServerHandle) Shutdown() error {
	if h.cancel != nil {
		h.cancel()
	}
	if h.server != nil {
		return h.server.Shutdown()
	}
	return nil
}

// Wait waits for the server to finish and returns any error
func (h *ServerHandle) Wait() error {
	return <-h.errChan
}

// startServeInBackground starts a development server in the background using a context
// Returns a handle that can be used to shut down the server via context cancellation
func startServeInBackground(t *testing.T, workDir string, args ...string) (*ServerHandle, error) {
	t.Helper()

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Change to work directory if specified
	if workDir != "" {
		if err := os.Chdir(workDir); err != nil {
			return nil, fmt.Errorf("failed to change directory to %s: %w", workDir, err)
		}
		defer func() { _ = os.Chdir(origDir) }()
	}

	// Parse serve arguments to create config
	config := serve.DefaultConfig()
	config.OpenBrowser = false // Never open browser in tests
	config.LiveReload = false  // Disable live reload by default in tests

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--port requires a value")
			}
			var port int
			if _, err := fmt.Sscanf(args[i+1], "%d", &port); err != nil {
				return nil, fmt.Errorf("invalid port: %s", args[i+1])
			}
			config.Port = port
			i++
		case "--mode", "-m":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--mode requires a value")
			}
			config.Mode = serve.ServeMode(args[i+1])
			config.AutoDetect = false
			i++
		case "--no-browser":
			config.OpenBrowser = false
		case "--no-reload":
			config.LiveReload = false
		}
	}

	// Create server
	server, err := serve.NewServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	// Create context with cancellation for server control
	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		// Change directory for the goroutine
		if workDir != "" {
			_ = os.Chdir(workDir)
		}

		// Start returns when server shuts down or errors
		if err := server.Start(ctx); err != nil {
			errChan <- err
		} else {
			errChan <- nil
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	return &ServerHandle{
		server:  server,
		cancel:  cancel,
		errChan: errChan,
	}, nil
}

// createTestApp creates a new test application and sets it up for testing
func createTestApp(t *testing.T, tmpDir, appName string, opts *AppOptions) string {
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
	if err := runLvtCommand(t, tmpDir, args...); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	appDir := filepath.Join(tmpDir, appName)

	// Add replace directive to use local livetemplate (for testing with latest changes)
	// Protected by mutex to prevent race with parallel tests changing directory
	chdirMutex.Lock()
	cwd, _ := os.Getwd()
	livetemplatePath := filepath.Join(cwd, "..", "..", "..")
	chdirMutex.Unlock()

	replaceCmd := exec.Command("go", "mod", "edit", fmt.Sprintf("-replace=github.com/livefir/livetemplate=%s", livetemplatePath))
	replaceCmd.Dir = appDir
	if err := replaceCmd.Run(); err != nil {
		t.Fatalf("Failed to add replace directive: %v", err)
	}

	// Run go mod tidy with mutex protection
	t.Log("Running go mod tidy...")
	if err := runGoModTidy(t, appDir); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}

	// Copy client library for dev mode
	if opts.DevMode {
		t.Log("Copying client library...")
		// Use absolute path to avoid issues with parallel test execution
		clientSrc := filepath.Join(livetemplatePath, "client", "dist", "livetemplate-client.browser.js")
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
