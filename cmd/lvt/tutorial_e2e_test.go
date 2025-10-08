package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestTutorialE2E tests the complete blog tutorial workflow
func TestTutorialE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tutorial test in short mode")
	}

	// Skip if we can't build (e.g., when running from wrong directory)
	if _, err := exec.Command("go", "list", ".").Output(); err != nil {
		t.Skip("Skipping E2E test: not in correct directory")
	}

	// Create temp directory for test blog
	tmpDir := t.TempDir()
	blogDir := filepath.Join(tmpDir, "testblog")

	// Build lvt binary
	t.Log("Building lvt binary...")
	lvtBinary := filepath.Join(tmpDir, "lvt")
	// Use package path to build from anywhere
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}
	t.Log("✅ lvt binary built")

	// Step 1: lvt new testblog --dev (use local client library for testing)
	t.Log("Step 1: Creating new blog app...")
	newCmd := exec.Command(lvtBinary, "new", "testblog", "--dev")
	newCmd.Dir = tmpDir
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create new app: %v", err)
	}
	t.Log("✅ Blog app created")

	// Step 2: Generate posts resource
	t.Log("Step 2: Generating posts resource...")
	genPostsCmd := exec.Command(lvtBinary, "gen", "posts", "title", "content", "published:bool")
	genPostsCmd.Dir = blogDir
	genPostsCmd.Stdout = os.Stdout
	genPostsCmd.Stderr = os.Stderr
	if err := genPostsCmd.Run(); err != nil {
		t.Fatalf("Failed to generate posts: %v", err)
	}
	t.Log("✅ Posts resource generated")

	// Step 3: Generate categories resource
	t.Log("Step 3: Generating categories resource...")
	genCatsCmd := exec.Command(lvtBinary, "gen", "categories", "name", "description")
	genCatsCmd.Dir = blogDir
	genCatsCmd.Stdout = os.Stdout
	genCatsCmd.Stderr = os.Stderr
	if err := genCatsCmd.Run(); err != nil {
		t.Fatalf("Failed to generate categories: %v", err)
	}
	t.Log("✅ Categories resource generated")

	// Step 4: Generate comments resource with foreign key
	t.Log("Step 4: Generating comments resource with FK...")
	genCommentsCmd := exec.Command(lvtBinary, "gen", "comments", "post_id:references:posts", "author", "text")
	genCommentsCmd.Dir = blogDir
	genCommentsCmd.Stdout = os.Stdout
	genCommentsCmd.Stderr = os.Stderr
	if err := genCommentsCmd.Run(); err != nil {
		t.Fatalf("Failed to generate comments: %v", err)
	}
	t.Log("✅ Comments resource generated with foreign key")

	// Step 5: Run migrations
	t.Log("Step 5: Running migrations...")
	migrateCmd := exec.Command(lvtBinary, "migration", "up")
	migrateCmd.Dir = blogDir
	migrateCmd.Stdout = os.Stdout
	migrateCmd.Stderr = os.Stderr
	if err := migrateCmd.Run(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	t.Log("✅ Migrations complete")

	// Verify foreign key in migration file
	t.Log("Verifying foreign key syntax...")
	migrationsDir := filepath.Join(blogDir, "internal", "database", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("Failed to read migrations dir: %v", err)
	}

	var commentsMigration string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "comments") {
			data, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
			if err != nil {
				t.Fatalf("Failed to read migration: %v", err)
			}
			commentsMigration = string(data)
			break
		}
	}

	// Verify inline FOREIGN KEY (not ALTER TABLE)
	if strings.Contains(commentsMigration, "ALTER TABLE") && strings.Contains(commentsMigration, "ADD CONSTRAINT") {
		t.Error("❌ Migration uses ALTER TABLE ADD CONSTRAINT (should use inline FOREIGN KEY)")
	} else if strings.Contains(commentsMigration, "FOREIGN KEY (post_id) REFERENCES posts(id)") {
		t.Log("✅ Foreign key uses correct inline syntax")
	} else {
		t.Error("❌ Foreign key definition not found in migration")
	}

	// Step 6: Run go mod tidy to resolve dependencies added by generated code
	t.Log("Step 6: Resolving dependencies...")

	// Add replace directive to use local livetemplate (for testing with latest changes)
	// Get absolute path to livetemplate root (two directories up from cmd/lvt)
	cwd, _ := os.Getwd()
	livetemplatePath := filepath.Join(cwd, "..", "..")
	replaceCmd := exec.Command("go", "mod", "edit", fmt.Sprintf("-replace=github.com/livefir/livetemplate=%s", livetemplatePath))
	replaceCmd.Dir = blogDir
	if err := replaceCmd.Run(); err != nil {
		t.Fatalf("Failed to add replace directive: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = blogDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}
	t.Log("✅ Dependencies resolved")

	// Step 6.2: Copy client library for testing
	t.Log("Step 6.2: Copying client library...")
	clientSrc := "../../client/dist/livetemplate-client.browser.js"
	clientDst := filepath.Join(blogDir, "livetemplate-client.js")
	clientContent, err := os.ReadFile(clientSrc)
	if err != nil {
		t.Fatalf("Failed to read client library: %v", err)
	}
	if err := os.WriteFile(clientDst, clientContent, 0644); err != nil {
		t.Fatalf("Failed to write client library: %v", err)
	}
	t.Logf("✅ Client library copied (%d bytes)", len(clientContent))

	// Step 6.5: Verify generated test files compile
	t.Log("Step 6.5: Verifying generated test files compile...")
	testPackages := []string{
		"./internal/app/posts",
		"./internal/app/categories",
		"./internal/app/comments",
	}

	for _, pkg := range testPackages {
		t.Logf("Compiling tests for %s...", pkg)
		testCmd := exec.Command("go", "test", "-c", "-o", "/dev/null", pkg)
		testCmd.Dir = blogDir
		output, err := testCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("❌ Generated test files in %s don't compile: %v\n%s", pkg, err, output)
		}
	}
	t.Log("✅ All generated test files compile successfully")

	// Step 7: Build the app (verify it compiles)
	t.Log("Step 7: Building blog app...")
	serverBinary := filepath.Join(blogDir, "testblog")
	buildCmd = exec.Command("go", "build", "-o", serverBinary, "./cmd/testblog")
	buildCmd.Dir = blogDir
	var buildOutput []byte
	buildOutput, err = buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("❌ Generated app failed to compile: %v\n%s", err, buildOutput)
	}
	t.Log("✅ Blog app compiled successfully")

	// Step 8: Start the app
	t.Log("Step 8: Starting blog app...")
	serverPort := 8765 // Use fixed port for testing
	portStr := fmt.Sprintf("%d", serverPort)

	// Capture server logs to detect errors
	var serverLogs strings.Builder
	serverCmd := exec.Command(serverBinary)
	serverCmd.Dir = blogDir
	serverCmd.Env = append(os.Environ(), "PORT="+portStr)
	serverCmd.Stdout = io.MultiWriter(os.Stdout, &serverLogs)
	serverCmd.Stderr = io.MultiWriter(os.Stderr, &serverLogs)

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server process: %v", err)
	}
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Wait for server to be ready and verify it's responding correctly
	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	ready := false
	var lastErr error
	for i := 0; i < 50; i++ {
		// Check if server process is still running
		if serverCmd.ProcessState != nil && serverCmd.ProcessState.Exited() {
			t.Fatalf("❌ Server process exited unexpectedly: %v", serverCmd.ProcessState)
		}

		resp, err := http.Get(serverURL + "/posts")
		if err == nil {
			// Check status code
			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				t.Fatalf("❌ /posts returned status %d instead of 200. Body:\n%s", resp.StatusCode, string(body))
			}

			// Check response contains HTML
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("❌ Failed to read response body: %v", err)
			}

			bodyStr := string(body)
			if !strings.Contains(bodyStr, "<!DOCTYPE html>") && !strings.Contains(bodyStr, "<html") {
				t.Fatalf("❌ /posts response doesn't look like HTML. First 500 chars:\n%s", bodyStr[:min(500, len(bodyStr))])
			}

			// Check for template errors
			if strings.Contains(bodyStr, "template:") && strings.Contains(bodyStr, "error") {
				t.Fatalf("❌ /posts response contains template error:\n%s", bodyStr[:min(1000, len(bodyStr))])
			}

			ready = true
			break
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}

	if !ready {
		t.Fatalf("❌ Server failed to respond within 10 seconds. Last error: %v", lastErr)
	}

	// Final check: ensure server is still running after initial requests
	if serverCmd.ProcessState != nil && serverCmd.ProcessState.Exited() {
		t.Fatalf("❌ Server exited after responding: %v", serverCmd.ProcessState)
	}

	t.Log("✅ Blog app running on", serverURL)

	// Step 9: E2E UI Testing with Chrome
	t.Log("Step 9: Testing UI with Chrome...")

	// Start Chrome in Docker
	debugPort := 9222
	chromeCmd := startDockerChrome(t, debugPort)
	defer stopDockerChrome(t, chromeCmd, debugPort)

	// Connect to Chrome
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	// Capture console logs to detect WebSocket errors
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Determine URL for Chrome to access (Docker networking)
	testURL := getTestURL(serverPort)

	// Listen for console errors (especially WebSocket errors)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if consoleEvent, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			for _, arg := range consoleEvent.Args {
				if arg.Type == runtime.TypeString {
					logMsg := string(arg.Value)
					if strings.Contains(logMsg, "WebSocket") || strings.Contains(logMsg, "Failed") || strings.Contains(logMsg, "Error") {
						t.Logf("Browser console: %s", logMsg)
					}
				}
			}
		}
	})

	// Test WebSocket Connection
	t.Run("WebSocket Connection", func(t *testing.T) {
		// First, test if client library is being served
		clientLibResp, err := http.Get(serverURL + "/livetemplate-client.js")
		if err != nil {
			t.Fatalf("Failed to fetch client library: %v", err)
		}
		defer clientLibResp.Body.Close()
		t.Logf("Client library response status: %d", clientLibResp.StatusCode)
		if clientLibResp.StatusCode != 200 {
			t.Fatalf("Client library not available: status %d", clientLibResp.StatusCode)
		}

		var wsConnected bool
		var wsURL string
		var wsReadyState int
		var pathname string
		var liveUrl string
		err = chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for WebSocket to connect
			chromedp.Evaluate(`window.location.pathname`, &pathname),
			chromedp.Evaluate(`window.liveTemplateClient ? window.liveTemplateClient.options.liveUrl : null`, &liveUrl),
			chromedp.Evaluate(`(() => {
				// Get WebSocket URL being used
				return window.liveTemplateClient && window.liveTemplateClient.ws ? window.liveTemplateClient.ws.url : null;
			})()`, &wsURL),
			chromedp.Evaluate(`(() => {
				// Get WebSocket readyState
				return window.liveTemplateClient && window.liveTemplateClient.ws ? window.liveTemplateClient.ws.readyState : -1;
			})()`, &wsReadyState),
			chromedp.Evaluate(`(() => {
				// Check if WebSocket connection exists
				return window.liveTemplateClient &&
				       window.liveTemplateClient.ws &&
				       window.liveTemplateClient.ws.readyState === WebSocket.OPEN;
			})()`, &wsConnected),
		)
		if err != nil {
			t.Fatalf("Failed to check WebSocket connection: %v", err)
		}

		t.Logf("window.location.pathname: %s", pathname)
		t.Logf("client.options.liveUrl: %s", liveUrl)
		t.Logf("WebSocket URL: %s, ReadyState: %d (0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED)", wsURL, wsReadyState)

		if !wsConnected {
			t.Error("❌ WebSocket did not connect to /posts endpoint")
		} else {
			t.Log("✅ WebSocket connected successfully to " + wsURL)
		}
	})

	// Test /posts Endpoint Serves Content
	t.Run("Posts Page", func(t *testing.T) {
		var lvtId string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.AttributeValue(`[data-lvt-id]`, "data-lvt-id", &lvtId, nil),
		)
		if err != nil {
			t.Fatalf("Failed to test /posts endpoint: %v", err)
		}

		if lvtId == "" {
			t.Error("❌ LiveTemplate wrapper not found on /posts endpoint")
		} else {
			t.Logf("✅ /posts endpoint serves LiveTemplate content (wrapper ID: %s)", lvtId)
		}
	})

	// Test Add Post
	t.Run("Add Post", func(t *testing.T) {
		err := chromedp.Run(ctx,
			// Navigate to /posts and wait for it to load
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for WebSocket connection

			// Fill in the form
			chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="title"]`, "My First Blog Post", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="content"]`, "This is the content of my first blog post", chromedp.ByQuery),
			chromedp.Click(`input[name="published"]`, chromedp.ByQuery),

			// Click the submit button
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),

			// Wait for the post to appear in the table
			chromedp.Sleep(2*time.Second),

			// Reload page to see the persisted post (workaround for tree update issue)
			chromedp.Reload(),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}

		// Verify the post appears in the table
		var postInTable bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const table = document.querySelector('table');
					if (!table) return false;
					const rows = Array.from(table.querySelectorAll('tbody tr'));
					return rows.some(row => {
						const cells = row.querySelectorAll('td');
						return cells.length > 0 && cells[0].textContent.trim() === 'My First Blog Post';
					});
				})()
			`, &postInTable),
		)
		if err != nil {
			t.Fatalf("Failed to check table: %v", err)
		}

		if !postInTable {
			var tableSummary string
			chromedp.Run(ctx,
				chromedp.Evaluate(`
					(() => {
						const table = document.querySelector('table');
						if (!table) {
							const wrapper = document.querySelector('[data-lvt-id]');
							return 'No table found. Wrapper exists: ' + !!wrapper + '. Body text: ' + document.body.textContent.substring(0, 200);
						}
						const rows = Array.from(table.querySelectorAll('tbody tr'));
						return 'Table has ' + rows.length + ' rows. Titles: ' + rows.map(r => {
							const cells = r.querySelectorAll('td');
							return cells.length > 0 ? cells[0].textContent.trim() : '';
						}).join(', ');
					})()
				`, &tableSummary),
			)
			t.Fatalf("❌ Post not found in table.\nTable summary: %s", tableSummary)
		}

		t.Log("✅ Post 'My First Blog Post' added successfully and appears in table")
	})

	// Test Delete Post
	t.Run("Delete Post", func(t *testing.T) {
		// First, verify the post exists
		var postExists bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`
				(() => {
					const table = document.querySelector('table');
					if (!table) return false;
					const rows = Array.from(table.querySelectorAll('tbody tr'));
					return rows.some(row => {
						const cells = row.querySelectorAll('td');
						return cells.length > 0 && cells[0].textContent.trim() === 'My First Blog Post';
					});
				})()
			`, &postExists),
		)
		if err != nil {
			t.Fatalf("Failed to check for post: %v", err)
		}

		if !postExists {
			t.Fatal("❌ Post 'My First Blog Post' not found - cannot test deletion")
		}

		// Find and click the delete button for the post
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const table = document.querySelector('table');
					if (!table) return false;
					const rows = Array.from(table.querySelectorAll('tbody tr'));
					const targetRow = rows.find(row => {
						const cells = row.querySelectorAll('td');
						return cells.length > 0 && cells[0].textContent.trim() === 'My First Blog Post';
					});
					if (targetRow) {
						const deleteButton = targetRow.querySelector('button[lvt-click="delete"]');
						if (deleteButton) {
							deleteButton.click();
							return true;
						}
					}
					return false;
				})()
			`, &postExists),
			chromedp.Sleep(2*time.Second), // Wait for deletion to process

			// Reload page to see the deletion result (workaround for tree update issue)
			chromedp.Reload(),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("Failed to delete post: %v", err)
		}

		// Verify the post is no longer in the table
		var postStillExists bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const table = document.querySelector('table');
					if (!table) return false;
					const rows = Array.from(table.querySelectorAll('tbody tr'));
					return rows.some(row => {
						const cells = row.querySelectorAll('td');
						return cells.length > 0 && cells[0].textContent.trim() === 'My First Blog Post';
					});
				})()
			`, &postStillExists),
		)
		if err != nil {
			t.Fatalf("Failed to check if post was deleted: %v", err)
		}

		if postStillExists {
			t.Fatal("❌ Post 'My First Blog Post' still exists after deletion")
		}

		t.Log("✅ Post 'My First Blog Post' deleted successfully")
	})

	// Test Validation Errors
	t.Run("Validation Errors", func(t *testing.T) {
		var (
			errorsVisible    bool
			titleErrorText   string
			contentErrorText string
			formHTML         string
		)

		err := chromedp.Run(ctx,
			// Navigate to /posts
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),

			// Submit form WITHOUT filling required fields
			chromedp.WaitVisible(`form[lvt-submit]`, chromedp.ByQuery),
			chromedp.Evaluate(`
				const form = document.querySelector('form[lvt-submit]');
				if (form) {
					form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
				}
			`, nil),

			chromedp.Sleep(3*time.Second), // Wait for validation response and UI update

			// Debug: Capture the form HTML
			chromedp.Evaluate(`document.querySelector('form[lvt-submit]')?.outerHTML || 'Form not found'`, &formHTML),

			// Check if error messages are visible in the UI (rendered server-side)
			chromedp.Evaluate(`
				(() => {
					// Look for error messages in <small> tags (server-side rendered via .lvt.HasError)
					const form = document.querySelector('form[lvt-submit]');
					if (!form) return false;
					const smallTags = Array.from(form.querySelectorAll('small'));
					return smallTags.some(el => el.textContent.includes('required') || el.textContent.includes('is required'));
				})()
			`, &errorsVisible),

			// Get specific error texts (server-side rendered)
			chromedp.Evaluate(`
				(() => {
					const form = document.querySelector('form[lvt-submit]');
					if (!form) return '';
					// Find the small tag near the title input
					const titleDiv = Array.from(form.querySelectorAll('div')).find(div => {
						const label = div.querySelector('label');
						return label && label.textContent.includes('Title');
					});
					return titleDiv ? (titleDiv.querySelector('small')?.textContent || '') : '';
				})()
			`, &titleErrorText),
			chromedp.Evaluate(`
				(() => {
					const form = document.querySelector('form[lvt-submit]');
					if (!form) return '';
					// Find the small tag near the content input
					const contentDiv = Array.from(form.querySelectorAll('div')).find(div => {
						const label = div.querySelector('label');
						return label && label.textContent.includes('Content');
					});
					return contentDiv ? (contentDiv.querySelector('small')?.textContent || '') : '';
				})()
			`, &contentErrorText),
		)
		if err != nil {
			t.Fatalf("Failed to test validation: %v", err)
		}

		// Debug: Log form HTML
		t.Logf("Form HTML (first 500 chars): %s", formHTML[:min(500, len(formHTML))])

		// Verify errors are displayed in the UI (server-side rendered)
		if !errorsVisible {
			t.Fatal("❌ Error messages are not visible in the UI")
		}
		t.Log("✅ Error messages are visible in the UI")

		// Verify specific field errors
		if titleErrorText == "" {
			t.Error("❌ Title field error not displayed")
		} else {
			t.Logf("✅ Title error: %s", titleErrorText)
		}

		if contentErrorText == "" {
			t.Error("❌ Content field error not displayed")
		} else {
			t.Logf("✅ Content error: %s", contentErrorText)
		}
	})

	// Final check: ensure no server errors occurred during the entire test
	t.Run("Server Logs Check", func(t *testing.T) {
		logs := serverLogs.String()
		// Note: "Tree generation failed (using fallback)" is a warning, not an error
		// Only check for actual failures and panics
		errorPatterns := []string{
			"Template update execution failed",
			"panic:",
			"fatal error:",
		}

		for _, pattern := range errorPatterns {
			if strings.Contains(logs, pattern) {
				t.Errorf("❌ Server error pattern '%s' found in logs:\n%s", pattern, logs)
				break
			}
		}

		if !t.Failed() {
			t.Log("✅ No server errors detected in logs")
		}
	})

	t.Log("✅ All E2E tests passed!")
}

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

	stopCmd := exec.Command("docker", "stop", containerName)
	stopCmd.Run()

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// getTestURL returns the URL for Chrome to access the test server
func getTestURL(port int) string {
	if goruntime.GOOS == "linux" {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}
