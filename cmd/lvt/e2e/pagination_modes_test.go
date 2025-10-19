package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPaginationModes tests different pagination modes
func TestPaginationModes(t *testing.T) {
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
