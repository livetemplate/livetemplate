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
			name:    "both password and magic-link disabled",
			args:    []string{"--no-password", "--no-magic-link"},
			wantErr: true,
			errMsg:  "at least one authentication method (password or magic-link) must be enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We only test flag validation here, not actual file generation
			// File generation is tested in TestAuthCommand_Integration
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

func TestPluralizeNoun(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"User", "users"},
		{"Account", "accounts"},
		{"Admin", "admins"},
		{"Member", "members"},
		{"Category", "categories"}, // y -> ies
		{"Box", "boxes"},           // x -> xes
		{"Buzz", "buzzes"},         // z -> zes
		{"Class", "classes"},       // ss -> sses
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pluralizeNoun(tt.input)
			if got != tt.want {
				t.Errorf("pluralizeNoun(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAuthCommand_CustomNames(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedTable   string
		expectedStruct  string
		expectedQueries []string // Snippets to look for in queries.sql
	}{
		{
			name:           "default names",
			args:           []string{},
			expectedTable:  "users",
			expectedStruct: "User",
			expectedQueries: []string{
				"-- name: CreateUser :one",
				"INSERT INTO users",
				"-- name: GetUserByEmail :one",
				"SELECT * FROM users",
			},
		},
		{
			name:           "custom struct name",
			args:           []string{"Account"},
			expectedTable:  "accounts",
			expectedStruct: "Account",
			expectedQueries: []string{
				"-- name: CreateAccount :one",
				"INSERT INTO accounts",
				"-- name: GetAccountByEmail :one",
				"SELECT * FROM accounts",
			},
		},
		{
			name:           "custom struct and table",
			args:           []string{"Admin", "admin_users"},
			expectedTable:  "admin_users",
			expectedStruct: "Admin",
			expectedQueries: []string{
				"-- name: CreateAdmin :one",
				"INSERT INTO admin_users",
				"-- name: GetAdminByEmail :one",
				"SELECT * FROM admin_users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create minimal project structure
			if err := os.MkdirAll(filepath.Join(tmpDir, "internal", "database"), 0755); err != nil {
				t.Fatalf("failed to create directories: %v", err)
			}
			if err := os.WriteFile(filepath.Join(tmpDir, ".lvtrc"), []byte(`module="testapp"`), 0644); err != nil {
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

			// Run auth command with custom names
			err = Auth(tt.args)
			if err != nil {
				t.Fatalf("auth command failed: %v", err)
			}

			// Read migration file
			migrationsDir := filepath.Join(tmpDir, "internal", "database", "migrations")
			files, err := os.ReadDir(migrationsDir)
			if err != nil {
				t.Fatalf("failed to read migrations directory: %v", err)
			}

			var migrationContent string
			for _, f := range files {
				if strings.Contains(f.Name(), "create_auth_tables") {
					content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
					if err != nil {
						t.Fatalf("failed to read migration: %v", err)
					}
					migrationContent = string(content)
					break
				}
			}

			if migrationContent == "" {
				t.Fatal("migration file not found")
			}

			// Check migration contains correct table name
			if !strings.Contains(migrationContent, "CREATE TABLE "+tt.expectedTable) {
				t.Errorf("migration does not contain 'CREATE TABLE %s'", tt.expectedTable)
			}

			// Read queries.sql
			queriesPath := filepath.Join(tmpDir, "internal", "database", "queries.sql")
			queriesContent, err := os.ReadFile(queriesPath)
			if err != nil {
				t.Fatalf("failed to read queries.sql: %v", err)
			}

			queriesStr := string(queriesContent)

			// Check queries contain expected snippets
			for _, expected := range tt.expectedQueries {
				if !strings.Contains(queriesStr, expected) {
					t.Errorf("queries.sql does not contain %q", expected)
				}
			}
		})
	}
}
