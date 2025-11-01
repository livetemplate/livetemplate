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

func TestGenerateAuth_EmailSender(t *testing.T) {
	tmpDir := t.TempDir()

	err := GenerateAuth(tmpDir, &AuthConfig{
		EnablePassword:      true,
		EnableEmailConfirm:  true,
		EnablePasswordReset: true,
	})
	if err != nil {
		t.Fatalf("GenerateAuth failed: %v", err)
	}

	emailPath := filepath.Join(tmpDir, "internal", "shared", "email", "email.go")
	if _, err := os.Stat(emailPath); os.IsNotExist(err) {
		t.Errorf("email.go not generated at %s", emailPath)
	}

	content, err := os.ReadFile(emailPath)
	if err != nil {
		t.Fatalf("failed to read email.go: %v", err)
	}

	contentStr := string(content)

	// Check for EmailSender interface
	if !strings.Contains(contentStr, "type EmailSender interface") {
		t.Error("email.go missing EmailSender interface")
	}

	// Check for console logger implementation
	if !strings.Contains(contentStr, "type ConsoleEmailSender struct") {
		t.Error("email.go missing ConsoleEmailSender")
	}

	// Check for Send method
	if !strings.Contains(contentStr, "func (s *ConsoleEmailSender) Send") {
		t.Error("email.go missing Send method")
	}
}

func TestGenerateAuth_Migration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create migrations directory
	migrationsDir := filepath.Join(tmpDir, "internal", "database", "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create migrations directory: %v", err)
	}

	err := GenerateAuth(tmpDir, &AuthConfig{
		EnablePassword: true,
	})
	if err != nil {
		t.Fatalf("GenerateAuth failed: %v", err)
	}

	// Find migration file (should be timestamped)
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var migrationFile string
	for _, f := range files {
		if strings.Contains(f.Name(), "create_auth_tables") {
			migrationFile = filepath.Join(migrationsDir, f.Name())
			break
		}
	}

	if migrationFile == "" {
		t.Fatal("auth migration file not found")
	}

	content, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatalf("failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for users table
	if !strings.Contains(contentStr, "CREATE TABLE users") {
		t.Error("migration missing users table")
	}

	// Check for user_tokens table
	if !strings.Contains(contentStr, "CREATE TABLE user_tokens") {
		t.Error("migration missing user_tokens table")
	}

	// Check for goose directives
	if !strings.Contains(contentStr, "-- +goose Up") {
		t.Error("migration missing goose Up directive")
	}

	if !strings.Contains(contentStr, "-- +goose Down") {
		t.Error("migration missing goose Down directive")
	}

	// Check for case-insensitive email (COLLATE NOCASE for SQLite)
	if !strings.Contains(contentStr, "COLLATE NOCASE") {
		t.Error("migration missing case-insensitive email")
	}
}
