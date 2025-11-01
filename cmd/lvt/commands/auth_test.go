package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuth_Flags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "default - all features enabled",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "disable magic link",
			args:    []string{"--no-magic-link"},
			wantErr: false,
		},
		{
			name:    "disable password",
			args:    []string{"--no-password"},
			wantErr: false,
		},
		{
			name:    "both password and magic-link disabled",
			args:    []string{"--no-password", "--no-magic-link"},
			wantErr: true,
			errMsg:  "at least one authentication method (password or magic-link) must be enabled",
		},
		{
			name:    "disable email confirmation",
			args:    []string{"--no-email-confirm"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Auth(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuthCommand_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal project structure
	if err := os.MkdirAll(filepath.Join(tmpDir, "internal", "database"), 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".lvtrc"), []byte(`module = "testapp"`), 0644); err != nil {
		t.Fatalf("failed to create .lvtrc: %v", err)
	}

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	// Run auth command
	err = Auth([]string{})
	if err != nil {
		t.Fatalf("auth command failed: %v", err)
	}

	// Verify files were created
	expectedFiles := []string{
		"internal/shared/password/password.go",
		"internal/shared/email/email.go",
		"internal/database/queries.sql",
	}

	for _, path := range expectedFiles {
		fullPath := filepath.Join(tmpDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected file not created: %s", path)
		}
	}

	// Verify migration file exists
	migrationsDir := filepath.Join(tmpDir, "internal", "database", "migrations")
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}
	foundMigration := false
	for _, f := range files {
		if strings.Contains(f.Name(), "create_auth_tables") {
			foundMigration = true
			break
		}
	}
	if !foundMigration {
		t.Error("auth migration not created")
	}
}
