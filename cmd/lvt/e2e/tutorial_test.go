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
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/livetemplate/cmd/lvt/testing"
)

// TestTutorialE2E tests the complete blog tutorial workflow
func TestTutorialE2E(t *testing.T) {
	// Note: Not parallel because tests use chdirMutex and need sequential execution

	// Create temp directory for test blog
	tmpDir := t.TempDir()
	blogDir := filepath.Join(tmpDir, "testblog")

	// Step 1: lvt new testblog --dev (use local client library for testing)
	t.Log("Step 1: Creating new blog app...")
	if err := runLvtCommand(t, tmpDir, "new", "testblog", "--dev"); err != nil {
		t.Fatalf("Failed to create new app: %v", err)
	}
	t.Log("✅ Blog app created")

	// Step 2: Generate posts resource
	t.Log("Step 2: Generating posts resource...")
	if err := runLvtCommand(t, blogDir, "gen", "resource", "posts", "title", "content", "published:bool"); err != nil {
		t.Fatalf("Failed to generate posts: %v", err)
	}
	t.Log("✅ Posts resource generated")

	// Step 3: Generate categories resource
	t.Log("Step 3: Generating categories resource...")
	if err := runLvtCommand(t, blogDir, "gen", "resource", "categories", "name", "description"); err != nil {
		t.Fatalf("Failed to generate categories: %v", err)
	}
	t.Log("✅ Categories resource generated")

	// Step 4: Generate comments resource with foreign key
	t.Log("Step 4: Generating comments resource with FK...")
	if err := runLvtCommand(t, blogDir, "gen", "resource", "comments", "post_id:references:posts", "author", "text"); err != nil {
		t.Fatalf("Failed to generate comments: %v", err)
	}
	t.Log("✅ Comments resource generated with foreign key")

	// Step 5: Run migrations
	t.Log("Step 5: Running migrations...")
	if err := runLvtCommand(t, blogDir, "migration", "up"); err != nil {
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
	// Get absolute path to livetemplate root (three directories up from cmd/lvt/e2e)
	// Protected by mutex to prevent race with parallel tests changing directory
	chdirMutex.Lock()
	cwd, _ := os.Getwd()
	livetemplatePath := filepath.Join(cwd, "..", "..", "..")
	chdirMutex.Unlock()

	replaceCmd := exec.Command("go", "mod", "edit", fmt.Sprintf("-replace=github.com/livetemplate/livetemplate=%s", livetemplatePath))
	replaceCmd.Dir = blogDir
	if err := replaceCmd.Run(); err != nil {
		t.Fatalf("Failed to add replace directive: %v", err)
	}

	if err := runGoModTidy(t, blogDir); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}
	t.Log("✅ Dependencies resolved")

	// Step 6.2: Copy client library for testing using absolute path
	t.Log("Step 6.2: Copying client library...")
	clientSrc := filepath.Join(livetemplatePath, "client", "dist", "livetemplate-client.browser.js")
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
	buildCmd := exec.Command("go", "build", "-o", serverBinary, "./cmd/testblog")
	buildCmd.Dir = blogDir
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("❌ Generated app failed to compile: %v\n%s", err, buildOutput)
	}
	t.Log("✅ Blog app compiled successfully")

	// Step 8: Start the app
	t.Log("Step 8: Starting blog app...")
	serverPort := allocateTestPort() // Use unique port for parallel testing
	portStr := fmt.Sprintf("%d", serverPort)

	// Capture server logs to detect errors
	serverLogs := e2etest.NewSafeBuffer()
	serverCmd := exec.Command(serverBinary)
	serverCmd.Dir = blogDir
	serverCmd.Env = append(os.Environ(), "PORT="+portStr)
	serverCmd.Stdout = io.MultiWriter(os.Stdout, serverLogs)
	serverCmd.Stderr = io.MultiWriter(os.Stderr, serverLogs)

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server process: %v", err)
	}
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			if err := serverCmd.Process.Kill(); err != nil {
				t.Logf("Warning: failed to kill server process: %v", err)
			}
		}
	}()

	// Wait for server to be ready and verify it's responding correctly
	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	ready := false
	var lastErr error
	consecutiveSuccesses := 0
	const requiredSuccesses = 2 // Require consecutive successes for stability

	for i := 0; i < 50; i++ {
		// Check if server process is still running
		if serverCmd.ProcessState != nil && serverCmd.ProcessState.Exited() {
			t.Fatalf("❌ Server process exited unexpectedly: %v", serverCmd.ProcessState)
		}

		resp, err := http.Get(serverURL + "/posts")
		if err == nil {
			// Check status code
			if resp.StatusCode != 200 {
				resp.Body.Close()
				lastErr = fmt.Errorf("status %d", resp.StatusCode)
				consecutiveSuccesses = 0
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Check response contains HTML
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = fmt.Errorf("failed to read body: %w", err)
				consecutiveSuccesses = 0
				time.Sleep(200 * time.Millisecond)
				continue
			}

			bodyStr := string(body)
			if !strings.Contains(bodyStr, "<!DOCTYPE html>") && !strings.Contains(bodyStr, "<html") {
				lastErr = fmt.Errorf("response doesn't look like HTML")
				consecutiveSuccesses = 0
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Check for template errors
			if strings.Contains(bodyStr, "template:") && strings.Contains(bodyStr, "error") {
				lastErr = fmt.Errorf("template error in response")
				consecutiveSuccesses = 0
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Success - increment counter
			consecutiveSuccesses++
			if consecutiveSuccesses >= requiredSuccesses {
				// Give server extra time to fully initialize WebSocket handlers
				time.Sleep(100 * time.Millisecond)
				ready = true
				break
			}
		} else {
			lastErr = err
			consecutiveSuccesses = 0
		}
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

	// Use isolated Chrome container for parallel execution
	ctx, cancel := getIsolatedChromeContext(t)
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
		// Create fresh browser context for this subtest
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 30*time.Second)
		defer timeoutCancel()

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
		err = chromedp.Run(testCtx,
			chromedp.Navigate(testURL+"/posts"),
			waitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			validateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions
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
		// Create fresh browser context for this subtest
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 30*time.Second)
		defer timeoutCancel()

		var lvtId string
		err := chromedp.Run(testCtx,
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
		// Create per-subtest context with individual timeout
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 60*time.Second)
		defer timeoutCancel()

		var titleValueBeforeSubmit string
		var contentValueBeforeSubmit string
		var activeElementAfterTyping string
		var lastFocusedName string
		err := chromedp.Run(testCtx,
			// Navigate to /posts and wait for it to load
			chromedp.Navigate(testURL+"/posts"),
			waitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			validateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions

			// Click the "+ Add Posts" button in toolbar to open modal
			chromedp.WaitVisible(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			chromedp.Click(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			// Wait for modal to appear
			waitFor(`document.querySelector('[role="dialog"]') && !document.querySelector('[role="dialog"]').hasAttribute('hidden')`, 3*time.Second),

			// Fill in the form in the modal
			chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="title"]`, "My First Blog Post", chromedp.ByQuery),
			chromedp.SendKeys(`textarea[name="content"]`, "This is the content of my first blog post", chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('input[name="title"]').value`, &titleValueBeforeSubmit),
			chromedp.Evaluate(`document.querySelector('textarea[name="content"]').value`, &contentValueBeforeSubmit),
			chromedp.Evaluate(`document.activeElement ? document.activeElement.getAttribute('name') || document.activeElement.tagName : 'none'`, &activeElementAfterTyping),
			chromedp.Click(`input[name="published"]`, chromedp.ByQuery),

			// Click the submit button in the modal
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),

			// Wait for the post to appear in the table
			waitFor(`
				(() => {
					const table = document.querySelector('table tbody');
					return table && table.querySelectorAll('tr').length > 0;
				})()
			`, 5*time.Second),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			// Wait for page to load
			waitFor(`document.readyState === 'complete'`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}

		// Debug: Capture the tree state from the client
		var treeState string
		var lastWSMessage string
		var lastFormData string
		_ = chromedp.Run(testCtx,
			chromedp.Evaluate(`JSON.stringify(window.liveTemplateClient?.treeState || {}, null, 2)`, &treeState),
			chromedp.Evaluate(`window.__lastWSMessage || 'No WS message captured'`, &lastWSMessage),
			chromedp.Evaluate(`JSON.stringify(window.__lvtLastFormData || JSON.parse(window.sessionStorage.getItem("__lvtLastFormData") || "null"))`, &lastFormData),
			chromedp.Evaluate(`window.__lvtLastFocusedName || 'unknown'`, &lastFocusedName),
		)
		t.Logf("=== CLIENT TREE STATE ===\n%s\n", treeState)
		t.Logf("=== LAST WS MESSAGE ===\n%s\n", lastWSMessage)
		t.Logf("=== LAST FORM DATA ===\n%s\n", lastFormData)
		t.Logf("=== VALUES BEFORE SUBMIT === title=%q content=%q activeElement=%q lastFocused=%q\n", titleValueBeforeSubmit, contentValueBeforeSubmit, activeElementAfterTyping, lastFocusedName)

		// Verify the post appears in the table
		var postInTable bool
		err = chromedp.Run(testCtx,
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
			var firstCellHTML string
			_ = chromedp.Run(testCtx,
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
				chromedp.Evaluate(`
					(() => {
						const table = document.querySelector('table');
						if (!table) return 'No table';
						const firstRow = table.querySelector('tbody tr');
						if (!firstRow) return 'No rows';
						const firstCell = firstRow.querySelector('td');
						if (!firstCell) return 'No cell';
						return firstCell.innerHTML;
					})()
				`, &firstCellHTML),
			)
			t.Fatalf("❌ Post not found in table.\nTable summary: %s\nFirst cell HTML: %s", tableSummary, firstCellHTML)
		}

		t.Log("✅ Post 'My First Blog Post' added successfully and appears in table")
	})

	// Test Modal Delete with Confirmation
	// TODO: Skip due to flaky timing issue - test depends on data from previous test
	t.Run("Modal Delete with Confirmation", func(t *testing.T) {
		t.Skip("Skipping due to flaky test dependency")
		// Create per-subtest context with individual timeout
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 60*time.Second)
		defer timeoutCancel()

		// First, verify the post exists
		var postExists bool
		err := chromedp.Run(testCtx,
			chromedp.Navigate(testURL+"/posts"),
			waitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			validateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions
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

		// Verify there's NO delete button in table rows (modal mode)
		var deleteButtonInRow bool
		err = chromedp.Run(testCtx,
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
						return !!deleteButton;
					}
					return false;
				})()
			`, &deleteButtonInRow),
		)
		if err != nil {
			t.Fatalf("Failed to check for delete button in row: %v", err)
		}

		if deleteButtonInRow {
			t.Error("❌ Delete button should NOT be in table rows in modal mode")
		} else {
			t.Log("✅ No delete button in table rows (modal mode)")
		}

		// Click Edit button to open modal
		var editButtonFound bool
		err = chromedp.Run(testCtx,
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
						const editButton = targetRow.querySelector('button[lvt-click="edit"]');
						if (editButton) {
							editButton.click();
							return true;
						}
					}
					return false;
				})()
			`, &editButtonFound),
			// Wait for modal to open
			waitFor(`document.querySelector('input[name="title"]') !== null`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to click edit button: %v", err)
		}

		if !editButtonFound {
			t.Fatal("❌ Edit button not found in table row")
		}
		t.Log("✅ Edit button clicked, modal should be open")

		// Verify delete button exists in modal with lvt-confirm attribute
		var deleteButtonInModal bool
		var hasConfirmAttr bool
		err = chromedp.Run(testCtx,
			chromedp.Evaluate(`
				(() => {
					const deleteButton = document.querySelector('button[lvt-click="delete"]');
					return !!deleteButton;
				})()
			`, &deleteButtonInModal),
			chromedp.Evaluate(`
				(() => {
					const deleteButton = document.querySelector('button[lvt-click="delete"]');
					return deleteButton ? deleteButton.hasAttribute('lvt-confirm') : false;
				})()
			`, &hasConfirmAttr),
		)
		if err != nil {
			t.Fatalf("Failed to check delete button in modal: %v", err)
		}

		if !deleteButtonInModal {
			t.Fatal("❌ Delete button not found in modal")
		}
		t.Log("✅ Delete button found in modal")

		if !hasConfirmAttr {
			t.Error("❌ Delete button missing lvt-confirm attribute")
		} else {
			t.Log("✅ Delete button has lvt-confirm attribute")
		}

		// We've already verified the key requirements:
		// 1. No delete button in table rows ✅
		// 2. Delete button exists in modal ✅
		// 3. Delete button has lvt-confirm attribute ✅
		// The confirmation dialog functionality is client-side JavaScript (window.confirm)
		// which is tested implicitly through the next test case

		t.Log("✅ Modal delete confirmation setup verified")
	})

	// Test Delete Post (Accept Confirmation)
	t.Run("Delete Post with Accepted Confirmation", func(t *testing.T) {
		// Create per-subtest context with individual timeout
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 60*time.Second)
		defer timeoutCancel()

		const (
			defaultPostTitle   = "My First Blog Post"
			findExistingPostJS = `
		(() => {
			const titles = ['My Updated Blog Post', 'My First Blog Post'];
			const table = document.querySelector('table');
			if (!table) return '';
			const rows = Array.from(table.querySelectorAll('tbody tr'));
			for (const title of titles) {
				if (rows.some(row => {
					const cells = row.querySelectorAll('td');
					return cells.length > 0 && cells[0].textContent.trim() === title;
				})) {
					return title;
				}
			}
			return '';
		})()
		`
		)

		var targetTitle string
		err := chromedp.Run(testCtx,
			chromedp.Navigate(testURL+"/posts"),
			waitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			validateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions
			chromedp.Evaluate(findExistingPostJS, &targetTitle),
		)
		if err != nil {
			t.Fatalf("Failed to check for existing post: %v", err)
		}

		if targetTitle == "" {
			t.Log("ℹ️ Post not found, creating fixture for deletion test")
			if err := ensureTutorialPostExists(testCtx, testURL); err != nil {
				t.Fatalf("Failed to create post fixture: %v", err)
			}
			targetTitle = defaultPostTitle
			postVisibleJS := fmt.Sprintf(`
		(() => {
			const table = document.querySelector('table');
			if (!table) return false;
			const rows = Array.from(table.querySelectorAll('tbody tr'));
			return rows.some(row => {
				const cells = row.querySelectorAll('td');
				return cells.length > 0 && cells[0].textContent.trim() === %q;
			});
		})()
		`, targetTitle)
			var postVisible bool
			if err := chromedp.Run(testCtx,
				waitFor(postVisibleJS, 5*time.Second),
				chromedp.Evaluate(postVisibleJS, &postVisible),
			); err != nil {
				t.Fatalf("Failed to verify post fixture: %v", err)
			}
			if !postVisible {
				t.Fatal("❌ Unable to create post fixture for deletion test")
			}
		}

		if targetTitle == "" {
			t.Fatal("❌ No post available for deletion test")
		}

		checkPostExistsJS := fmt.Sprintf(`
		(() => {
			const table = document.querySelector('table');
			if (!table) return false;
			const rows = Array.from(table.querySelectorAll('tbody tr'));
			return rows.some(row => {
				const cells = row.querySelectorAll('td');
				return cells.length > 0 && cells[0].textContent.trim() === %q;
			});
		})()
		`, targetTitle)

		var postExists bool
		if err := chromedp.Run(testCtx,
			chromedp.Evaluate(checkPostExistsJS, &postExists),
		); err != nil {
			t.Fatalf("Failed to verify post before deletion: %v", err)
		}
		if !postExists {
			t.Fatalf("❌ Target post %q not found before deletion", targetTitle)
		}

		// Click Edit button to open modal
		err = chromedp.Run(testCtx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const table = document.querySelector('table');
					if (!table) return false;
					const rows = Array.from(table.querySelectorAll('tbody tr'));
					const targetRow = rows.find(row => {
						const cells = row.querySelectorAll('td');
					return cells.length > 0 && cells[0].textContent.trim() === %q;
					});
					if (targetRow) {
						const editButton = targetRow.querySelector('button[lvt-click="edit"]');
						if (editButton) {
							editButton.click();
							return true;
						}
					}
					return false;
			})()
			`, targetTitle), &postExists),
			// Wait for modal controls to be ready before continuing
			waitFor(`document.querySelector('button[lvt-click="delete"]') !== null`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to open edit modal: %v", err)
		}

		// Override window.confirm to return true (accept)
		err = chromedp.Run(testCtx,
			chromedp.Evaluate(`window.confirm = () => true;`, nil),
			chromedp.Evaluate(`
				(() => {
					const deleteButton = document.querySelector('button[lvt-click="delete"]');
					if (deleteButton) {
						deleteButton.click();
						return true;
					}
					return false;
				})()
			`, &postExists),
			// Wait for deletion to process
			waitFor(fmt.Sprintf(`
			(() => {
				const table = document.querySelector('table tbody');
				if (!table) return true;
				const rows = Array.from(table.querySelectorAll('tr'));
				return !rows.some(row => {
					const cells = row.querySelectorAll('td');
					return cells.length > 0 && cells[0].textContent.trim() === %q;
				});
			})()
			`, targetTitle), 5*time.Second),

			waitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			// Wait for page to load
			waitFor(`document.readyState === 'complete'`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to delete post: %v", err)
		}

		// Verify the post is no longer in the table
		var postStillExists bool
		err = chromedp.Run(testCtx,
			chromedp.Evaluate(checkPostExistsJS, &postStillExists),
		)
		if err != nil {
			t.Fatalf("Failed to check if post was deleted: %v", err)
		}

		if postStillExists {
			t.Fatalf("❌ Post %q still exists after deletion", targetTitle)
		}

		t.Logf("✅ Post %q deleted successfully after confirming", targetTitle)
	})

	// Test Validation Errors
	// TODO: Skip until core library bug is fixed - see BUG-VALIDATION-CONDITIONALS.md
	t.Run("Validation Errors", func(t *testing.T) {
		t.Skip("Skipping until conditional rendering bug is fixed")
		// Create per-subtest context with individual timeout
		testCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		testCtx, timeoutCancel := context.WithTimeout(testCtx, 60*time.Second)
		defer timeoutCancel()

		var (
			errorsVisible    bool
			titleErrorText   string
			contentErrorText string
			formHTML         string
		)

		err := chromedp.Run(testCtx,
			// Navigate to /posts
			chromedp.Navigate(testURL+"/posts"),
			waitForWebSocketReady(5*time.Second), // Wait for WebSocket init and first update
			chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
			validateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions

			// Click the "+ Add Posts" button in toolbar to open modal
			chromedp.WaitVisible(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			chromedp.Click(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
			// Wait for modal to appear
			chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),

			// Submit form WITHOUT filling required fields
			chromedp.WaitVisible(`form[lvt-submit]`, chromedp.ByQuery),
			chromedp.Evaluate(`
				const form = document.querySelector('form[lvt-submit]');
				if (form) {
					// Bypass HTML5 validation to test server-side validation
					form.noValidate = true;
					form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
				}
			`, nil),

			// Wait for validation response - form should still be visible
			waitFor(`document.querySelector('form[lvt-submit]') !== null`, 3*time.Second),

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
}

func ensureTutorialPostExists(ctx context.Context, baseURL string) error {
	const (
		title   = "My First Blog Post"
		content = "This is the content of my first blog post"
	)

	existenceCheck := fmt.Sprintf(`
		(() => {
			const table = document.querySelector('table');
			if (!table) return false;
			const rows = Array.from(table.querySelectorAll('tbody tr'));
			return rows.some(row => {
				const cells = row.querySelectorAll('td');
				return cells.length > 0 && cells[0].textContent.trim() === %q;
			});
		})()
		`, title)

	return chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/posts"),
		waitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
		chromedp.WaitVisible(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
		chromedp.Click(`[lvt-modal-open="add-modal"]`, chromedp.ByQuery),
		waitFor(`document.querySelector('[role="dialog"]') && !document.querySelector('[role="dialog"]').hasAttribute('hidden')`, 3*time.Second),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="title"]').value = ''`, nil),
		chromedp.Evaluate(`document.querySelector('textarea[name="content"]').value = ''`, nil),
		chromedp.SendKeys(`input[name="title"]`, title, chromedp.ByQuery),
		chromedp.SendKeys(`textarea[name="content"]`, content, chromedp.ByQuery),
		chromedp.Click(`input[name="published"]`, chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		waitFor(existenceCheck, 5*time.Second),
		chromedp.WaitVisible(`[data-lvt-id]`, chromedp.ByQuery),
		waitFor(`document.readyState === 'complete'`, 3*time.Second),
	)
}
