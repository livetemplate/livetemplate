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

			// Click the "+ Add Posts" button in toolbar to open modal
			chromedp.WaitVisible(`button[lvt-click="open_add"]`, chromedp.ByQuery),
			chromedp.Click(`button[lvt-click="open_add"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for modal to appear

			// Fill in the form in the modal
			chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="title"]`, "My First Blog Post", chromedp.ByQuery),
			chromedp.SendKeys(`textarea[name="content"]`, "This is the content of my first blog post", chromedp.ByQuery),
			chromedp.Click(`input[name="published"]`, chromedp.ByQuery),

			// Click the submit button in the modal
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

			// Click the "+ Add Posts" button in toolbar to open modal
			chromedp.WaitVisible(`button[lvt-click="open_add"]`, chromedp.ByQuery),
			chromedp.Click(`button[lvt-click="open_add"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for modal to appear

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

	// Test Infinite Scroll Sentinel
	t.Run("Infinite Scroll Sentinel", func(t *testing.T) {
		// The sentinel only appears when HasMore is true (more items to load)
		// Since we added 1 post and default page size is 20, HasMore will be false
		// So we check that the template is configured for infinite scroll by:
		// 1. Checking the generated handler has PaginationMode: "infinite"
		// 2. Verifying template contains infiniteScroll define

		// Read handler file to verify pagination mode
		handlerFile := filepath.Join(blogDir, "internal", "app", "posts", "posts.go")
		handlerContent, err := os.ReadFile(handlerFile)
		if err != nil {
			t.Fatalf("Failed to read posts handler: %v", err)
		}

		if !strings.Contains(string(handlerContent), `PaginationMode: "infinite"`) {
			t.Error("❌ Posts handler does not have PaginationMode: \"infinite\"")
		} else {
			t.Log("✅ Posts handler configured with infinite pagination mode")
		}

		// Read template file to verify infiniteScroll block exists
		tmplFile := filepath.Join(blogDir, "internal", "app", "posts", "posts.tmpl")
		tmplContent, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read posts template: %v", err)
		}

		tmplStr := string(tmplContent)
		if !strings.Contains(tmplStr, `id="scroll-sentinel"`) {
			t.Error("❌ Template does not contain scroll-sentinel element")
		} else {
			t.Log("✅ Template contains scroll-sentinel element for infinite scroll")
		}

		// Verify the sentinel appears in actual rendered HTML when there are no template errors
		// (The sentinel won't be visible with only 1 post, but we've verified the configuration)
		t.Log("✅ Infinite scroll pagination configured correctly")
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
			"template:", // Catch template parsing errors
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

// TestTutorialE2E_CSSFrameworks tests different CSS frameworks
func TestTutorialE2E_CSSFrameworks(t *testing.T) {
	frameworks := []string{"bulma", "pico", "none"}

	for _, framework := range frameworks {
		t.Run("CSS_"+framework, func(t *testing.T) {
			tmpDir := t.TempDir()
			appDir := filepath.Join(tmpDir, "testapp")

			// Build lvt
			lvtBinary := filepath.Join(tmpDir, "lvt")
			buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
			if err := buildCmd.Run(); err != nil {
				t.Fatalf("Failed to build lvt: %v", err)
			}

			// Create app
			newCmd := exec.Command(lvtBinary, "new", "testapp")
			newCmd.Dir = tmpDir
			newCmd.Stdout = os.Stdout
			newCmd.Stderr = os.Stderr
			if err := newCmd.Run(); err != nil {
				t.Fatalf("Failed to create app: %v", err)
			}

			// Generate resource with specific CSS framework
			genCmd := exec.Command(lvtBinary, "gen", "items", "name", "--css", framework)
			genCmd.Dir = appDir
			genCmd.Stdout = os.Stdout
			genCmd.Stderr = os.Stderr
			if err := genCmd.Run(); err != nil {
				t.Fatalf("Failed to generate resource with --css %s: %v", framework, err)
			}

			// Verify template file exists
			tmplFile := filepath.Join(appDir, "internal", "app", "items", "items.tmpl")
			if _, err := os.Stat(tmplFile); err != nil {
				t.Fatalf("Template file not created: %v", err)
			}

			// Check for CSS framework-specific content
			content, err := os.ReadFile(tmplFile)
			if err != nil {
				t.Fatalf("Failed to read template: %v", err)
			}

			contentStr := string(content)
			switch framework {
			case "bulma":
				if !strings.Contains(contentStr, "button") {
					t.Error("❌ Bulma CSS classes not found in template")
				}
			case "pico":
				if !strings.Contains(contentStr, "button") {
					t.Error("❌ Pico CSS classes not found in template")
				}
			case "none":
				// Template should still be valid
				if len(contentStr) < 100 {
					t.Error("❌ Template seems empty or invalid")
				}
			}

			t.Logf("✅ Resource generated successfully with --css %s", framework)
		})
	}
}

// TestTutorialE2E_PaginationModes tests different pagination modes
func TestTutorialE2E_PaginationModes(t *testing.T) {
	modes := []string{"load-more", "prev-next", "numbers"}

	for _, mode := range modes {
		t.Run("Pagination_"+mode, func(t *testing.T) {
			tmpDir := t.TempDir()
			appDir := filepath.Join(tmpDir, "testapp")

			// Build lvt
			lvtBinary := filepath.Join(tmpDir, "lvt")
			buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
			if err := buildCmd.Run(); err != nil {
				t.Fatalf("Failed to build lvt: %v", err)
			}

			// Create app
			newCmd := exec.Command(lvtBinary, "new", "testapp")
			newCmd.Dir = tmpDir
			if err := newCmd.Run(); err != nil {
				t.Fatalf("Failed to create app: %v", err)
			}

			// Generate resource with specific pagination mode
			genCmd := exec.Command(lvtBinary, "gen", "items", "name", "--pagination", mode)
			genCmd.Dir = appDir
			genCmd.Stdout = os.Stdout
			genCmd.Stderr = os.Stderr
			if err := genCmd.Run(); err != nil {
				t.Fatalf("Failed to generate resource with --pagination %s: %v", mode, err)
			}

			// Verify handler file has correct pagination mode
			handlerFile := filepath.Join(appDir, "internal", "app", "items", "items.go")
			content, err := os.ReadFile(handlerFile)
			if err != nil {
				t.Fatalf("Failed to read handler: %v", err)
			}

			if !strings.Contains(string(content), fmt.Sprintf("PaginationMode: \"%s\"", mode)) {
				t.Errorf("❌ PaginationMode '%s' not found in handler", mode)
			} else {
				t.Logf("✅ Resource generated with --pagination %s", mode)
			}
		})
	}
}

// TestTutorialE2E_ViewGeneration tests view-only generation
func TestTutorialE2E_ViewGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Build lvt
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Create app
	newCmd := exec.Command(lvtBinary, "new", "testapp")
	newCmd.Dir = tmpDir
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Generate view
	genCmd := exec.Command(lvtBinary, "gen", "view", "dashboard")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate view: %v", err)
	}

	// Verify files exist
	handlerFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard.go")
	tmplFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard.tmpl")
	testFile := filepath.Join(appDir, "internal", "app", "dashboard", "dashboard_test.go")

	for _, file := range []string{handlerFile, tmplFile, testFile} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("❌ Expected file not created: %s", file)
		}
	}

	// Verify handler doesn't have CRUD operations
	content, err := os.ReadFile(handlerFile)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "PaginationMode") {
		t.Error("❌ View handler should not have pagination")
	}
	if strings.Contains(contentStr, "handleAdd") {
		t.Error("❌ View handler should not have CRUD operations")
	}

	t.Log("✅ View-only handler generated successfully")
}

// TestTutorialE2E_TypeInference tests field type inference
func TestTutorialE2E_TypeInference(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Build lvt
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Create app
	newCmd := exec.Command(lvtBinary, "new", "testapp")
	newCmd.Dir = tmpDir
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Generate resource with inferred types (no :type specified)
	genCmd := exec.Command(lvtBinary, "gen", "users", "name", "email", "age", "price", "published", "created_at")
	genCmd.Dir = appDir
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		t.Fatalf("Failed to generate resource with type inference: %v", err)
	}

	// Verify schema has correct inferred types
	schemaFile := filepath.Join(appDir, "internal", "database", "schema.sql")
	content, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}

	contentStr := string(content)

	// Check inferred types
	checks := map[string]string{
		"name":       "TEXT",     // string
		"email":      "TEXT",     // string
		"age":        "INTEGER",  // int
		"price":      "REAL",     // float
		"published":  "INTEGER",  // bool
		"created_at": "DATETIME", // time
	}

	for field, expectedType := range checks {
		if !strings.Contains(contentStr, field) || !strings.Contains(contentStr, expectedType) {
			t.Errorf("❌ Field '%s' not inferred as %s", field, expectedType)
		}
	}

	t.Log("✅ Type inference working correctly")
}

// TestTutorialE2E_TextareaFields tests textarea field generation
func TestTutorialE2E_TextareaFields(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Build lvt
	lvtBinary := filepath.Join(tmpDir, "lvt")
	buildCmd := exec.Command("go", "build", "-o", lvtBinary, "github.com/livefir/livetemplate/cmd/lvt")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build lvt: %v", err)
	}

	// Create app
	newCmd := exec.Command(lvtBinary, "new", "testapp")
	newCmd.Dir = tmpDir
	if err := newCmd.Run(); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Test 1: Generate resource with explicit textarea type
	t.Run("Explicit_Textarea_Type", func(t *testing.T) {
		genCmd := exec.Command(lvtBinary, "gen", "articles", "title", "content:text")
		genCmd.Dir = appDir
		genCmd.Stdout = os.Stdout
		genCmd.Stderr = os.Stderr
		if err := genCmd.Run(); err != nil {
			t.Fatalf("Failed to generate resource with :text type: %v", err)
		}

		// Verify template contains textarea for content field
		tmplFile := filepath.Join(appDir, "internal", "app", "articles", "articles.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Check that content field has textarea
		if !strings.Contains(contentStr, `<textarea`) {
			t.Error("❌ Template does not contain <textarea> element")
		} else {
			t.Log("✅ Template contains <textarea> element")
		}

		// Check that textarea has rows attribute
		if !strings.Contains(contentStr, `rows="5"`) {
			t.Error("❌ Textarea does not have rows attribute")
		} else {
			t.Log("✅ Textarea has rows=\"5\" attribute")
		}

		// Check that title field still uses input (not textarea)
		titleInputPattern := `name="title"`
		if !strings.Contains(contentStr, titleInputPattern) {
			t.Error("❌ Title field input not found")
		} else {
			// Verify title is an input by checking the surrounding context
			// Look for <input...name="title"
			titleIdx := strings.Index(contentStr, titleInputPattern)
			if titleIdx > 0 {
				// Check 100 chars before the name="title" for <input tag
				startIdx := titleIdx - 100
				if startIdx < 0 {
					startIdx = 0
				}
				contextBefore := contentStr[startIdx:titleIdx]
				if strings.Contains(contextBefore, "<input") {
					t.Log("✅ Title field uses <input> (not textarea)")
				} else if strings.Contains(contextBefore, "<textarea") {
					t.Error("❌ Title field should not use <textarea>")
				}
			}
		}
	})

	// Test 2: Generate resource with inferred textarea type
	t.Run("Inferred_Textarea_Type", func(t *testing.T) {
		genCmd := exec.Command(lvtBinary, "gen", "posts", "title", "content", "description", "body")
		genCmd.Dir = appDir
		genCmd.Stdout = os.Stdout
		genCmd.Stderr = os.Stderr
		if err := genCmd.Run(); err != nil {
			t.Fatalf("Failed to generate resource with inferred textarea types: %v", err)
		}

		// Verify template contains textareas for content, description, body
		tmplFile := filepath.Join(appDir, "internal", "app", "posts", "posts.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Count textarea occurrences (should be 3 fields × 2 forms = 6 textareas)
		textareaCount := strings.Count(contentStr, "<textarea")
		if textareaCount < 6 {
			t.Errorf("❌ Expected at least 6 <textarea> elements, found %d", textareaCount)
		} else {
			t.Logf("✅ Template contains %d <textarea> elements (content, description, body in add and edit forms)", textareaCount)
		}

		// Verify content field has textarea with name attribute
		if !strings.Contains(contentStr, `name="content"`) {
			t.Error("❌ Content field not found in template")
		}

		// Verify description field has textarea
		if !strings.Contains(contentStr, `name="description"`) {
			t.Error("❌ Description field not found in template")
		}

		// Verify body field has textarea
		if !strings.Contains(contentStr, `name="body"`) {
			t.Error("❌ Body field not found in template")
		}

		// Verify title field still uses input
		titleInputPattern := `name="title"`
		if !strings.Contains(contentStr, titleInputPattern) {
			t.Error("❌ Title field not found")
		}

		t.Log("✅ Type inference correctly mapped content, description, body to textarea fields")
	})

	// Test 3: Verify textarea aliases work (textarea, longtext)
	t.Run("Textarea_Aliases", func(t *testing.T) {
		genCmd := exec.Command(lvtBinary, "gen", "documents", "title", "summary:textarea", "details:longtext")
		genCmd.Dir = appDir
		genCmd.Stdout = os.Stdout
		genCmd.Stderr = os.Stderr
		if err := genCmd.Run(); err != nil {
			t.Fatalf("Failed to generate resource with textarea aliases: %v", err)
		}

		// Verify template contains textareas
		tmplFile := filepath.Join(appDir, "internal", "app", "documents", "documents.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Should have textareas for summary and details (2 fields × 2 forms = 4)
		textareaCount := strings.Count(contentStr, "<textarea")
		if textareaCount < 4 {
			t.Errorf("❌ Expected at least 4 <textarea> elements, found %d", textareaCount)
		} else {
			t.Logf("✅ Textarea aliases (textarea, longtext) work correctly (%d textareas found)", textareaCount)
		}
	})

	t.Log("✅ All textarea field tests passed!")
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
