package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAuth_PasswordUtilities(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Generate auth files
	err := GenerateAuth(tmpDir, &AuthConfig{
		EnablePassword:  true,
		EnableMagicLink: false,
	})
	if err != nil {
		t.Fatalf("GenerateAuth failed: %v", err)
	}

	// Check password.go exists
	passwordPath := filepath.Join(tmpDir, "internal", "shared", "password", "password.go")
	if _, err := os.Stat(passwordPath); os.IsNotExist(err) {
		t.Errorf("password.go not generated at %s", passwordPath)
	}

	// Read and verify content
	content, err := os.ReadFile(passwordPath)
	if err != nil {
		t.Fatalf("failed to read password.go: %v", err)
	}

	contentStr := string(content)
	requiredFuncs := []string{"Hash", "Verify"}
	for _, fn := range requiredFuncs {
		if !strings.Contains(contentStr, "func "+fn) {
			t.Errorf("password.go missing function: %s", fn)
		}
	}

	// Verify imports bcrypt
	if !strings.Contains(contentStr, "golang.org/x/crypto/bcrypt") {
		t.Error("password.go missing bcrypt import")
	}
}
