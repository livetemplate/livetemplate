package testing

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// E2ETest represents a configured e2e test environment with Chrome, server, and test context.
type E2ETest struct {
	T          *testing.T
	Context    context.Context
	Cancel     context.CancelFunc
	ServerPort int
	ChromePort int
	ServerCmd  *exec.Cmd
	ChromeCmd  *exec.Cmd
	AppDir     string
	AppPath    string
	serverURL  string

	// Loggers for debugging
	Console   *ConsoleLogger
	Server    *ServerLogger
	WebSocket *WSMessageLogger
}

// SetupOptions configures the test environment.
type SetupOptions struct {
	AppPath        string        // Path to main.go (e.g., "./main.go")
	Port           int           // Server port (auto-allocated if 0)
	Timeout        time.Duration // Test timeout (default 60s)
	CaptureConsole bool          // Capture browser console (default true)
	ChromeMode     ChromeMode    // Chrome mode (default: ChromeDocker)
	ChromePath     string        // Path to local Chrome binary (for ChromeLocal mode)
}

// ChromeMode specifies how Chrome should be launched.
type ChromeMode string

const (
	// ChromeDocker uses chromedp/headless-shell Docker container (default)
	ChromeDocker ChromeMode = "docker"
	// ChromeLocal uses locally installed Chrome/Chromium
	ChromeLocal ChromeMode = "local"
	// ChromeShared uses shared Chrome instance from TestMain
	ChromeShared ChromeMode = "shared"
)

// Setup creates a complete e2e test environment with Chrome, server, and test context.
// It automatically:
//   - Starts Chrome (Docker by default)
//   - Starts the test server
//   - Creates chromedp context
//   - Sets up console log capture (if enabled)
//
// Example:
//
//	test := lvttest.Setup(t, &lvttest.SetupOptions{
//	    AppPath: "./main.go",
//	})
//	defer test.Cleanup()
//
//	test.Navigate("/")
func Setup(t *testing.T, opts *SetupOptions) *E2ETest {
	t.Helper()

	// Apply defaults
	if opts == nil {
		opts = &SetupOptions{}
	}
	if opts.Timeout == 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.ChromeMode == "" {
		opts.ChromeMode = ChromeDocker
	}
	if opts.AppPath == "" {
		t.Fatal("AppPath is required in SetupOptions")
	}

	// Allocate ports
	serverPort := opts.Port
	if serverPort == 0 {
		var err error
		serverPort, err = GetFreePort()
		if err != nil {
			t.Fatalf("Failed to allocate server port: %v", err)
		}
	}

	chromePort, err := GetFreePort()
	if err != nil {
		t.Fatalf("Failed to allocate Chrome port: %v", err)
	}

	// Start server
	serverCmd := StartTestServer(t, opts.AppPath, serverPort)

	// Start Chrome based on mode
	var (
		chromeCmd       *exec.Cmd
		ctx             context.Context
		cancel          context.CancelFunc
		allocatorCancel context.CancelFunc
	)

	switch opts.ChromeMode {
	case ChromeDocker:
		chromeCmd = StartDockerChrome(t, chromePort)
		chromeURL := fmt.Sprintf("http://localhost:%d", chromePort)
		var allocCtx context.Context
		allocCtx, allocatorCancel = chromedp.NewRemoteAllocator(context.Background(), chromeURL)
		ctx, _ = chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	case ChromeLocal:
		// Use local Chrome installation
		allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
		)
		if opts.ChromePath != "" {
			allocOpts = append(allocOpts, chromedp.ExecPath(opts.ChromePath))
		}
		var allocCtx context.Context
		allocCtx, allocatorCancel = chromedp.NewExecAllocator(context.Background(), allocOpts...)
		ctx, _ = chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	case ChromeShared:
		// TODO: Implement shared Chrome instance support in Session 12
		t.Fatal("ChromeShared mode not yet implemented - coming in Session 12")

	default:
		t.Fatalf("Unknown ChromeMode: %s", opts.ChromeMode)
	}

	// Apply timeout - this is where we get the final cancel function
	ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	if allocatorCancel != nil {
		go func(doneCtx context.Context, cancelFunc context.CancelFunc) {
			<-doneCtx.Done()
			cancelFunc()
		}(ctx, allocatorCancel)
	}

	// Create loggers
	consoleLogger := NewConsoleLogger()
	serverLogger := NewServerLogger()
	wsLogger := NewWSMessageLogger()

	// Start loggers
	consoleLogger.Start(ctx)
	serverLogger.Start()
	wsLogger.Start(ctx)

	test := &E2ETest{
		T:          t,
		Context:    ctx,
		Cancel:     cancel,
		ServerPort: serverPort,
		ChromePort: chromePort,
		ServerCmd:  serverCmd,
		ChromeCmd:  chromeCmd,
		AppPath:    opts.AppPath,
		serverURL:  fmt.Sprintf("http://localhost:%d", serverPort),
		Console:    consoleLogger,
		Server:     serverLogger,
		WebSocket:  wsLogger,
	}

	return test
}

// Cleanup tears down all test resources (Chrome, server, contexts).
// This should be called with defer after Setup().
func (e *E2ETest) Cleanup() {
	e.T.Helper()

	// Stop loggers
	if e.Server != nil {
		e.Server.Stop()
	}

	// Cancel context
	if e.Cancel != nil {
		e.Cancel()
	}

	// Stop Chrome
	if e.ChromeCmd != nil {
		StopDockerChrome(e.T, e.ChromeCmd, e.ChromePort)
	}

	// Stop server
	if e.ServerCmd != nil && e.ServerCmd.Process != nil {
		_ = e.ServerCmd.Process.Kill()
	}
}

// Navigate navigates to the given path and waits for WebSocket to be ready.
// The path is relative to the server root (e.g., "/", "/products").
func (e *E2ETest) Navigate(path string) error {
	e.T.Helper()

	url := e.URL(path)

	return chromedp.Run(e.Context,
		chromedp.Navigate(url),
		WaitForWebSocketReady(5*time.Second),
	)
}

// URL returns the full test URL for the given path.
// For Docker Chrome, this uses GetChromeTestURL to handle host.docker.internal.
// For local Chrome, this uses localhost.
func (e *E2ETest) URL(path string) string {
	// For Docker Chrome, use GetChromeTestURL which handles host.docker.internal
	if e.ChromeCmd != nil {
		baseURL := GetChromeTestURL(e.ServerPort)
		if path == "" || path == "/" {
			return baseURL
		}
		return baseURL + path
	}

	// For local Chrome, use localhost
	if path == "" || path == "/" {
		return e.serverURL
	}
	return e.serverURL + path
}
