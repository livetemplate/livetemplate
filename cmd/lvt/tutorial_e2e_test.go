package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

	// Step 1: lvt new testblog
	t.Log("Step 1: Creating new blog app...")
	newCmd := exec.Command(lvtBinary, "new", "testblog")
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
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = blogDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		t.Fatalf("Failed to run go mod tidy: %v", err)
	}
	t.Log("✅ Dependencies resolved")

	// Step 7: Start the app
	t.Log("Step 7: Starting blog app...")
	serverPort := 8765 // Use fixed port for testing
	portStr := fmt.Sprintf("%d", serverPort)

	// Use package path instead of file path to avoid internal package import issues
	serverCmd := exec.Command("go", "run", "./cmd/testblog")
	serverCmd.Dir = blogDir
	serverCmd.Env = append(os.Environ(), "PORT="+portStr)
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Wait for server to be ready
	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(serverURL + "/posts")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !ready {
		t.Fatal("Server failed to start within 10 seconds")
	}
	t.Log("✅ Blog app running")

	// Step 8: E2E UI Testing with Chrome
	t.Log("Step 8: Testing UI with Chrome...")

	// Start Chrome in Docker
	debugPort := 9222
	chromeCmd := startDockerChrome(t, debugPort)
	defer stopDockerChrome(t, chromeCmd, debugPort)

	// Connect to Chrome
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Determine URL for Chrome to access (Docker networking)
	testURL := getTestURL(serverPort)

	// Test Posts Page
	t.Run("Posts Page", func(t *testing.T) {
		var pageTitle string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			chromedp.Text(`h1`, &pageTitle, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to test posts page: %v", err)
		}

		if !strings.Contains(pageTitle, "Posts") {
			t.Errorf("Expected 'Posts' in title, got: %s", pageTitle)
		}
		t.Log("✅ Posts page loads correctly")
	})

	// Test Categories Page
	t.Run("Categories Page", func(t *testing.T) {
		var pageTitle string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/categories"),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			chromedp.Text(`h1`, &pageTitle, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to test categories page: %v", err)
		}

		if !strings.Contains(pageTitle, "Categories") {
			t.Errorf("Expected 'Categories' in title, got: %s", pageTitle)
		}
		t.Log("✅ Categories page loads correctly")
	})

	// Test Comments Page
	t.Run("Comments Page", func(t *testing.T) {
		var pageTitle string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/comments"),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			chromedp.Text(`h1`, &pageTitle, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to test comments page: %v", err)
		}

		if !strings.Contains(pageTitle, "Comments") {
			t.Errorf("Expected 'Comments' in title, got: %s", pageTitle)
		}
		t.Log("✅ Comments page loads correctly")
	})

	// Test Add Post Form
	t.Run("Add Post", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(testURL+"/posts"),
			chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="title"]`, "Test Post Title", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="content"]`, "Test post content", chromedp.ByQuery),
			chromedp.Click(`input[name="published"]`, chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for WebSocket update
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}

		if !strings.Contains(html, "Test Post Title") {
			t.Error("Post not added to page")
		}
		t.Log("✅ Post added successfully via UI")
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
	if runtime.GOOS == "linux" {
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
	if runtime.GOOS == "linux" {
		return fmt.Sprintf("http://localhost:%d", port)
	}
	return fmt.Sprintf("http://host.docker.internal:%d", port)
}
