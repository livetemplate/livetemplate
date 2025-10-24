package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

// Shared test resources that persist across all tests
var (
	sharedChrome      *exec.Cmd
	sharedChromePort  int = 9222
	sharedTestApp     string
	sharedTestAppDir  string
	sharedSetupOnce   sync.Once
	sharedCleanupOnce sync.Once
	sharedSetupError  error

	// Port allocation for parallel tests
	portMutex    sync.Mutex
	nextTestPort int = 8800
)

// TestMain sets up shared resources before running tests and cleans up after
func TestMain(m *testing.M) {
	// Setup shared resources
	if err := setupSharedResources(); err != nil {
		log.Printf("Failed to setup shared resources: %v", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup shared resources
	cleanupSharedResources()

	os.Exit(code)
}

// setupSharedResources initializes shared Chrome container and pre-built test app
func setupSharedResources() error {
	var setupErr error
	sharedSetupOnce.Do(func() {
		log.Println("🚀 Setting up shared test resources...")

		// 1. Start shared Docker Chrome container
		log.Printf("Starting shared Docker Chrome on port %d...", sharedChromePort)
		sharedChrome = e2etest.StartDockerChrome(&testing.T{}, sharedChromePort)
		if sharedChrome == nil {
			setupErr = fmt.Errorf("failed to start shared Chrome container")
			return
		}

		// Wait a bit for Chrome to be fully ready
		time.Sleep(2 * time.Second)
		log.Println("✅ Shared Docker Chrome started")

		// 2. Pre-compile a test application for reuse
		log.Println("Pre-compiling test application...")
		tmpDir, err := os.MkdirTemp("", "lvt-e2e-shared-*")
		if err != nil {
			setupErr = fmt.Errorf("failed to create temp dir: %w", err)
			return
		}
		sharedTestAppDir = tmpDir

		// Create a standard test app
		appDir := filepath.Join(tmpDir, "sharedapp")

		// Use createTestApp if we can, but do it manually for now
		if err := runLvtCommand(&testing.T{}, tmpDir, "new", "sharedapp", "--dev"); err != nil {
			setupErr = fmt.Errorf("failed to create shared app: %w", err)
			return
		}

		// Generate a posts resource
		if err := runLvtCommand(&testing.T{}, appDir, "gen", "posts", "title", "content:text", "published:bool"); err != nil {
			setupErr = fmt.Errorf("failed to generate posts resource: %w", err)
			return
		}

		// Run migrations
		if err := runLvtCommand(&testing.T{}, appDir, "migration", "up"); err != nil {
			setupErr = fmt.Errorf("failed to run migrations: %w", err)
			return
		}

		// Setup go.mod for local livetemplate
		// Protected by mutex to prevent race with parallel tests changing directory
		chdirMutex.Lock()
		cwd, _ := os.Getwd()
		livetemplatePath := filepath.Join(cwd, "..", "..", "..")
		chdirMutex.Unlock()
		replaceCmd := exec.Command("go", "mod", "edit", fmt.Sprintf("-replace=github.com/livefir/livetemplate=%s", livetemplatePath))
		replaceCmd.Dir = appDir
		if err := replaceCmd.Run(); err != nil {
			log.Printf("Warning: Failed to add replace directive: %v", err)
		}

		// Run go mod tidy with mutex protection
		goModMutex.Lock()
		log.Println("Running go mod tidy in TestMain...")
		tidyCmd := exec.Command("go", "mod", "tidy")
		tidyCmd.Dir = appDir
		tidyCmd.Env = append(os.Environ(), "GOWORK=off") // Disable workspace mode to avoid conflicts
		tidyOutput, tidyErr := tidyCmd.CombinedOutput()
		goModMutex.Unlock()
		if tidyErr != nil {
			setupErr = fmt.Errorf("failed to run go mod tidy: %w\nOutput: %s", tidyErr, string(tidyOutput))
			return
		}

		// Run sqlc generate
		sqlcCmd := exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@latest", "generate", "-f", "internal/database/sqlc.yaml")
		sqlcCmd.Dir = appDir
		if err := sqlcCmd.Run(); err != nil {
			setupErr = fmt.Errorf("failed to run sqlc: %w", err)
			return
		}

		// Build the app
		appBinary := filepath.Join(appDir, "sharedapp")
		buildCmd := exec.Command("go", "build", "-o", appBinary, "./cmd/sharedapp")
		buildCmd.Dir = appDir
		if output, err := buildCmd.CombinedOutput(); err != nil {
			setupErr = fmt.Errorf("failed to build shared app: %w\n%s", err, output)
			return
		}

		sharedTestApp = appBinary
		log.Printf("✅ Pre-compiled test application: %s", sharedTestApp)

		sharedSetupError = nil
	})

	return setupErr
}

// cleanupSharedResources tears down shared resources
func cleanupSharedResources() {
	sharedCleanupOnce.Do(func() {
		log.Println("🧹 Cleaning up shared test resources...")

		// Stop shared Chrome container
		if sharedChrome != nil {
			log.Println("Stopping shared Docker Chrome...")
			e2etest.StopDockerChrome(&testing.T{}, sharedChrome, sharedChromePort)
			log.Println("✅ Shared Docker Chrome stopped")
		}

		// Clean up shared test app directory
		if sharedTestAppDir != "" {
			log.Printf("Cleaning up shared test app directory: %s", sharedTestAppDir)
			os.RemoveAll(sharedTestAppDir)
		}

		log.Println("✅ Shared test resources cleaned up")
	})
}

// getSharedChromeContext creates a Chrome context using the shared Chrome container
func getSharedChromeContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	// Ensure shared resources are setup
	if err := setupSharedResources(); err != nil {
		t.Fatalf("Failed to setup shared resources: %v", err)
	}

	if sharedSetupError != nil {
		t.Fatalf("Shared setup failed: %v", sharedSetupError)
	}

	// Create allocator context for shared Chrome
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(),
		fmt.Sprintf("http://localhost:%d", sharedChromePort))

	// Create browser context
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Return a combined cancel function
	combinedCancel := func() {
		cancel()
		allocCancel()
	}

	return ctx, combinedCancel
}

// allocateTestPort returns a unique port for a test
func allocateTestPort() int {
	portMutex.Lock()
	defer portMutex.Unlock()

	port := nextTestPort
	nextTestPort++
	return port
}

// Unused: Kept for potential future use
// getSharedTestApp returns the path to the pre-compiled test application
// func getSharedTestApp(t *testing.T) string {
// 	t.Helper()
//
// 	// Ensure shared resources are setup
// 	if err := setupSharedResources(); err != nil {
// 		t.Fatalf("Failed to setup shared resources: %v", err)
// 	}
//
// 	if sharedSetupError != nil {
// 		t.Fatalf("Shared setup failed: %v", sharedSetupError)
// 	}
//
// 	return sharedTestApp
// }

// Unused: Kept for potential future use
// getSharedTestAppDir returns the directory containing the pre-built test app
// func getSharedTestAppDir(t *testing.T) string {
// 	t.Helper()
//
// 	if err := setupSharedResources(); err != nil {
// 		t.Fatalf("Failed to setup shared resources: %v", err)
// 	}
//
// 	if sharedSetupError != nil {
// 		t.Fatalf("Shared setup failed: %v", sharedSetupError)
// 	}
//
// 	return sharedTestAppDir
// }
