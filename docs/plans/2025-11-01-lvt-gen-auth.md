# lvt gen auth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `lvt gen auth` command to generate a complete authentication system with password and magic-link support, email confirmation, password reset, session management, and CSRF protection.

**Architecture:** Single command with feature flags (--no-*) that generates database migrations, auth handlers with LiveTemplate integration, custom authenticator, route protection middleware, email sender interface, and CSRF middleware. Auth sessions are separate from app state (user_tokens table vs MemorySessionStore).

**Tech Stack:** Go, LiveTemplate, sqlc, goose migrations, gorilla/csrf, bcrypt, chromedp for E2E tests

---

## Task 1: Add auth command structure and flags

**Files:**
- Create: `cmd/lvt/commands/auth.go`
- Modify: `cmd/lvt/main.go` (add auth command)

**Step 1: Write the failing test**

Create `cmd/lvt/commands/auth_test.go`:

```go
package commands

import (
	"testing"
)

func TestAuthCommand_Flags(t *testing.T) {
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
			cmd := NewAuthCommand()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

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
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt
go test ./commands -run TestAuthCommand_Flags -v
```

Expected: FAIL with "undefined: NewAuthCommand"

**Step 3: Write minimal implementation**

Create `cmd/lvt/commands/auth.go`:

```go
package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

type AuthFlags struct {
	NoRegistration  bool
	NoPassword      bool
	NoMagicLink     bool
	NoEmailConfirm  bool
	NoPasswordReset bool
	NoSessionsUI    bool
	NoCSRF          bool
}

func NewAuthCommand() *cobra.Command {
	flags := &AuthFlags{}

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Generate authentication system with password and magic-link support",
		Long: `Generate a complete authentication system including:
  - User registration and login
  - Password and/or magic-link authentication
  - Email confirmation
  - Password reset flow
  - Session management UI
  - CSRF protection
  - Database migrations and queries`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate flags
			if flags.NoPassword && flags.NoMagicLink {
				return errors.New("at least one authentication method (password or magic-link) must be enabled")
			}

			fmt.Println("Generating authentication system...")
			return nil
		},
	}

	cmd.Flags().BoolVar(&flags.NoRegistration, "no-registration", false, "Skip user registration flow")
	cmd.Flags().BoolVar(&flags.NoPassword, "no-password", false, "Disable password authentication")
	cmd.Flags().BoolVar(&flags.NoMagicLink, "no-magic-link", false, "Disable magic-link authentication")
	cmd.Flags().BoolVar(&flags.NoEmailConfirm, "no-email-confirm", false, "Skip email confirmation requirement")
	cmd.Flags().BoolVar(&flags.NoPasswordReset, "no-password-reset", false, "Skip password reset flow")
	cmd.Flags().BoolVar(&flags.NoSessionsUI, "no-sessions-ui", false, "Skip session management UI")
	cmd.Flags().BoolVar(&flags.NoCSRF, "no-csrf", false, "Skip CSRF protection")

	return cmd
}
```

**Step 4: Run test to verify it passes**

```bash
cd cmd/lvt
go test ./commands -run TestAuthCommand_Flags -v
```

Expected: PASS

**Step 5: Register command in main.go**

Modify `cmd/lvt/main.go` to add auth command to root:

```go
// Find the rootCmd initialization and add:
rootCmd.AddCommand(commands.NewAuthCommand())
```

**Step 6: Test CLI manually**

```bash
./cmd/lvt/lvt auth --help
```

Expected: Shows auth command help with all flags

**Step 7: Commit**

```bash
git add cmd/lvt/commands/auth.go cmd/lvt/commands/auth_test.go cmd/lvt/main.go
git commit -m "feat(lvt): add auth command with feature flags

Add lvt gen auth command structure with flags for controlling
authentication features. Validates that at least one auth method
(password or magic-link) is enabled.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Create password utilities package

**Files:**
- Create: `cmd/lvt/internal/kits/system/multi/templates/auth/password.go.tmpl`
- Create: `cmd/lvt/internal/generator/testdata/auth/password_test.go.expected`

**Step 1: Write the failing test for password generation**

Create `cmd/lvt/internal/generator/auth_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_PasswordUtilities -v
```

Expected: FAIL with "undefined: GenerateAuth"

**Step 3: Create password template**

Create `cmd/lvt/internal/kits/system/multi/templates/auth/password.go.tmpl`:

```go
package password

import (
	"golang.org/x/crypto/bcrypt"
)

// Hash generates a bcrypt hash from a plain-text password.
// Uses bcrypt.DefaultCost (currently 10) for a good security/performance balance.
func Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify checks if a plain-text password matches a bcrypt hash.
// Returns true if the password matches, false otherwise.
func Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
```

**Step 4: Add GenerateAuth function**

Modify `cmd/lvt/internal/generator/generator.go`:

```go
// Add AuthConfig type
type AuthConfig struct {
	ModuleName       string
	EnablePassword   bool
	EnableMagicLink  bool
	EnableEmailConfirm bool
	EnablePasswordReset bool
	EnableSessionsUI bool
	EnableCSRF       bool
}

// Add GenerateAuth function
func GenerateAuth(projectRoot string, config *AuthConfig) error {
	// Create directories
	passwordDir := filepath.Join(projectRoot, "internal", "shared", "password")
	if err := os.MkdirAll(passwordDir, 0755); err != nil {
		return fmt.Errorf("failed to create password directory: %w", err)
	}

	// Generate password.go if password auth enabled
	if config.EnablePassword {
		templatePath := "cmd/lvt/internal/kits/system/multi/templates/auth/password.go.tmpl"
		outputPath := filepath.Join(passwordDir, "password.go")

		tmpl, err := template.ParseFiles(templatePath)
		if err != nil {
			return fmt.Errorf("failed to parse password template: %w", err)
		}

		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create password.go: %w", err)
		}
		defer file.Close()

		if err := tmpl.Execute(file, config); err != nil {
			return fmt.Errorf("failed to execute password template: %w", err)
		}
	}

	return nil
}
```

**Step 5: Run test to verify it passes**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_PasswordUtilities -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/kits/system/multi/templates/auth/password.go.tmpl cmd/lvt/internal/generator/auth_test.go cmd/lvt/internal/generator/generator.go
git commit -m "feat(lvt): add password utilities template

Add bcrypt-based password hashing utilities template for auth
generation. Includes Hash and Verify functions with sensible defaults.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Create email sender interface template

**Files:**
- Create: `cmd/lvt/internal/kits/system/multi/templates/auth/email.go.tmpl`

**Step 1: Write the failing test**

Add to `cmd/lvt/internal/generator/auth_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_EmailSender -v
```

Expected: FAIL - email.go not generated

**Step 3: Create email template**

Create `cmd/lvt/internal/kits/system/multi/templates/auth/email.go.tmpl`:

```go
package email

import (
	"fmt"
	"log"
)

// EmailSender defines the interface for sending emails.
// Implementations can use any email service (SMTP, Mailgun, SendGrid, etc.)
type EmailSender interface {
	Send(to, subject, body string) error
}

// ConsoleEmailSender logs emails to stdout for development.
// Replace with a real email sender for production.
type ConsoleEmailSender struct{}

// NewConsoleEmailSender creates a new console email sender.
func NewConsoleEmailSender() *ConsoleEmailSender {
	return &ConsoleEmailSender{}
}

// Send logs the email to stdout instead of actually sending it.
func (s *ConsoleEmailSender) Send(to, subject, body string) error {
	log.Printf("📧 EMAIL (Console Mode)\n")
	log.Printf("To: %s\n", to)
	log.Printf("Subject: %s\n", subject)
	log.Printf("Body:\n%s\n", body)
	log.Printf("---\n")
	return nil
}

// Example SMTP sender implementation (commented out):
//
// import "net/smtp"
//
// type SMTPEmailSender struct {
//     Host     string
//     Port     int
//     Username string
//     Password string
//     From     string
// }
//
// func (s *SMTPEmailSender) Send(to, subject, body string) error {
//     auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
//     msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
//         s.From, to, subject, body)
//     addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
//     return smtp.SendMail(addr, auth, s.From, []string{to}, []byte(msg))
// }

// Example Mailgun sender (commented out):
//
// import "github.com/mailgun/mailgun-go/v4"
//
// type MailgunEmailSender struct {
//     Domain string
//     APIKey string
//     From   string
// }
//
// func (s *MailgunEmailSender) Send(to, subject, body string) error {
//     mg := mailgun.NewMailgun(s.Domain, s.APIKey)
//     message := mg.NewMessage(s.From, subject, body, to)
//     _, _, err := mg.Send(context.Background(), message)
//     return err
// }
```

**Step 4: Update GenerateAuth to create email.go**

Modify `cmd/lvt/internal/generator/generator.go` in `GenerateAuth`:

```go
// Add after password.go generation
if config.EnableEmailConfirm || config.EnablePasswordReset {
	emailDir := filepath.Join(projectRoot, "internal", "shared", "email")
	if err := os.MkdirAll(emailDir, 0755); err != nil {
		return fmt.Errorf("failed to create email directory: %w", err)
	}

	templatePath := "cmd/lvt/internal/kits/system/multi/templates/auth/email.go.tmpl"
	outputPath := filepath.Join(emailDir, "email.go")

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create email.go: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, config); err != nil {
		return fmt.Errorf("failed to execute email template: %w", err)
	}
}
```

**Step 5: Run test to verify it passes**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_EmailSender -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/kits/system/multi/templates/auth/email.go.tmpl cmd/lvt/internal/generator/auth_test.go cmd/lvt/internal/generator/generator.go
git commit -m "feat(lvt): add email sender interface template

Add EmailSender interface with console logger implementation.
Includes commented examples for SMTP and Mailgun integration.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Create auth database migration template

**Files:**
- Create: `cmd/lvt/internal/kits/system/multi/templates/auth/migration.sql.tmpl`

**Step 1: Write the failing test**

Add to `cmd/lvt/internal/generator/auth_test.go`:

```go
func TestGenerateAuth_Migration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create migrations directory
	migrationsDir := filepath.Join(tmpDir, "internal", "database", "migrations")
	os.MkdirAll(migrationsDir, 0755)

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
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_Migration -v
```

Expected: FAIL - migration file not found

**Step 3: Create migration template**

Create `cmd/lvt/internal/kits/system/multi/templates/auth/migration.sql.tmpl`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    {{- if .EnablePassword }}
    hashed_password TEXT NOT NULL,
    {{- end }}
    confirmed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE user_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    context TEXT NOT NULL, -- "session", "confirm", "reset", "magic"
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE INDEX idx_user_tokens_user_id ON user_tokens(user_id);
CREATE INDEX idx_user_tokens_token ON user_tokens(token);
CREATE INDEX idx_user_tokens_context ON user_tokens(context, expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_tokens;
DROP TABLE users;
-- +goose StatementEnd
```

**Step 4: Update GenerateAuth to create migration**

Modify `cmd/lvt/internal/generator/generator.go` in `GenerateAuth`:

```go
import (
	"time"
)

// Add after email.go generation
// Generate migration
migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
if err := os.MkdirAll(migrationsDir, 0755); err != nil {
	return fmt.Errorf("failed to create migrations directory: %w", err)
}

timestamp := time.Now().Format("20060102150405")
migrationFile := fmt.Sprintf("%s_create_auth_tables.sql", timestamp)
migrationPath := filepath.Join(migrationsDir, migrationFile)

templatePath := "cmd/lvt/internal/kits/system/multi/templates/auth/migration.sql.tmpl"
tmpl, err := template.ParseFiles(templatePath)
if err != nil {
	return fmt.Errorf("failed to parse migration template: %w", err)
}

file, err := os.Create(migrationPath)
if err != nil {
	return fmt.Errorf("failed to create migration file: %w", err)
}
defer file.Close()

if err := tmpl.Execute(file, config); err != nil {
	return fmt.Errorf("failed to execute migration template: %w", err)
}
```

**Step 5: Run test to verify it passes**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_Migration -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/kits/system/multi/templates/auth/migration.sql.tmpl cmd/lvt/internal/generator/auth_test.go cmd/lvt/internal/generator/generator.go
git commit -m "feat(lvt): add auth database migration template

Add goose migration for users and user_tokens tables.
Supports case-insensitive email, configurable password field,
and token-based session/confirmation/reset tracking.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Create sqlc queries template

**Files:**
- Create: `cmd/lvt/internal/kits/system/multi/templates/auth/queries.sql.tmpl`

**Step 1: Write the failing test**

Add to `cmd/lvt/internal/generator/auth_test.go`:

```go
func TestGenerateAuth_Queries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database directory
	dbDir := filepath.Join(tmpDir, "internal", "database")
	os.MkdirAll(dbDir, 0755)

	err := GenerateAuth(tmpDir, &AuthConfig{
		EnablePassword:      true,
		EnableEmailConfirm:  true,
		EnablePasswordReset: true,
	})
	if err != nil {
		t.Fatalf("GenerateAuth failed: %v", err)
	}

	queriesPath := filepath.Join(dbDir, "queries.sql")
	if _, err := os.Stat(queriesPath); os.IsNotExist(err) {
		t.Errorf("queries.sql not generated/updated at %s", queriesPath)
	}

	content, err := os.ReadFile(queriesPath)
	if err != nil {
		t.Fatalf("failed to read queries.sql: %v", err)
	}

	contentStr := string(content)

	requiredQueries := []string{
		"-- name: CreateUser :one",
		"-- name: GetUserByEmail :one",
		"-- name: GetUserByID :one",
		"-- name: CreateUserToken :one",
		"-- name: GetUserToken :one",
		"-- name: DeleteUserToken :exec",
	}

	for _, query := range requiredQueries {
		if !strings.Contains(contentStr, query) {
			t.Errorf("queries.sql missing: %s", query)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_Queries -v
```

Expected: FAIL - queries.sql not generated

**Step 3: Create queries template**

Create `cmd/lvt/internal/kits/system/multi/templates/auth/queries.sql.tmpl`:

```sql
-- Auth Queries

-- name: CreateUser :one
INSERT INTO users (
    id,
    email,
    {{- if .EnablePassword }}
    hashed_password,
    {{- end }}
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    {{- if .EnablePassword }}
    ?,
    {{- end }}
    ?,
    ?
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? COLLATE NOCASE
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ?
LIMIT 1;

{{- if .EnableEmailConfirm }}

-- name: ConfirmUser :exec
UPDATE users
SET confirmed_at = ?, updated_at = ?
WHERE id = ?;

{{- end }}

{{- if .EnablePassword }}

-- name: UpdateUserPassword :exec
UPDATE users
SET hashed_password = ?, updated_at = ?
WHERE id = ?;

{{- end }}

-- name: CreateUserToken :one
INSERT INTO user_tokens (
    id,
    user_id,
    token,
    context,
    created_at,
    expires_at
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserToken :one
SELECT * FROM user_tokens
WHERE token = ? AND (expires_at IS NULL OR expires_at > ?)
LIMIT 1;

-- name: DeleteUserToken :exec
DELETE FROM user_tokens
WHERE token = ?;

-- name: DeleteUserTokensByContext :exec
DELETE FROM user_tokens
WHERE user_id = ? AND context = ?;

-- name: DeleteExpiredTokens :exec
DELETE FROM user_tokens
WHERE expires_at < ?;

{{- if .EnableSessionsUI }}

-- name: ListUserSessions :many
SELECT * FROM user_tokens
WHERE user_id = ? AND context = 'session'
ORDER BY created_at DESC;

-- name: DeleteUserSession :exec
DELETE FROM user_tokens
WHERE id = ? AND user_id = ? AND context = 'session';

{{- end }}
```

**Step 4: Update GenerateAuth to append to queries.sql**

Modify `cmd/lvt/internal/generator/generator.go` in `GenerateAuth`:

```go
// Add after migration generation
// Append to queries.sql (or create if doesn't exist)
queriesPath := filepath.Join(projectRoot, "internal", "database", "queries.sql")

templatePath = "cmd/lvt/internal/kits/system/multi/templates/auth/queries.sql.tmpl"
tmpl, err = template.ParseFiles(templatePath)
if err != nil {
	return fmt.Errorf("failed to parse queries template: %w", err)
}

// Open in append mode (create if doesn't exist)
file, err = os.OpenFile(queriesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
if err != nil {
	return fmt.Errorf("failed to open queries.sql: %w", err)
}
defer file.Close()

// Add separator if file already has content
stat, _ := file.Stat()
if stat.Size() > 0 {
	file.WriteString("\n\n")
}

if err := tmpl.Execute(file, config); err != nil {
	return fmt.Errorf("failed to execute queries template: %w", err)
}
```

**Step 5: Run test to verify it passes**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_Queries -v
```

Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/kits/system/multi/templates/auth/queries.sql.tmpl cmd/lvt/internal/generator/auth_test.go cmd/lvt/internal/generator/generator.go
git commit -m "feat(lvt): add auth sqlc queries template

Add sqlc queries for user CRUD, token management, and session
handling. Conditional queries based on enabled features.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Wire auth command to generator

**Files:**
- Modify: `cmd/lvt/commands/auth.go`

**Step 1: Write integration test**

Add to `cmd/lvt/commands/auth_test.go`:

```go
func TestAuthCommand_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal project structure
	os.MkdirAll(filepath.Join(tmpDir, "internal", "database"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".lvtrc"), []byte(`module = "testapp"`), 0644)

	cmd := NewAuthCommand()
	cmd.SetArgs([]string{})

	// Override project root for testing
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	err := cmd.Execute()
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
	files, _ := os.ReadDir(migrationsDir)
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
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/commands
go test -run TestAuthCommand_Integration -v
```

Expected: FAIL - files not created

**Step 3: Update auth command to call generator**

Modify `cmd/lvt/commands/auth.go`:

```go
import (
	"os"
	"path/filepath"

	"github.com/livefir/livetemplate/cmd/lvt/internal/config"
	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

// Update RunE:
RunE: func(cmd *cobra.Command, args []string) error {
	// Validate flags
	if flags.NoPassword && flags.NoMagicLink {
		return errors.New("at least one authentication method (password or magic-link) must be enabled")
	}

	// Get project root
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load project config
	cfg, err := config.Load(filepath.Join(wd, ".lvtrc"))
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	// Create generator config
	genConfig := &generator.AuthConfig{
		ModuleName:          cfg.Module,
		EnablePassword:      !flags.NoPassword,
		EnableMagicLink:     !flags.NoMagicLink,
		EnableEmailConfirm:  !flags.NoEmailConfirm,
		EnablePasswordReset: !flags.NoPasswordReset,
		EnableSessionsUI:    !flags.NoSessionsUI,
		EnableCSRF:          !flags.NoCSRF,
	}

	// Generate auth files
	fmt.Println("Generating authentication system...")
	if err := generator.GenerateAuth(wd, genConfig); err != nil {
		return fmt.Errorf("failed to generate auth: %w", err)
	}

	fmt.Println("✅ Authentication system generated successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run migrations: lvt migration up")
	fmt.Println("  2. Generate sqlc code: sqlc generate")
	fmt.Println("  3. Update main.go to register auth handler")

	return nil
},
```

**Step 4: Run test to verify it passes**

```bash
cd cmd/lvt/commands
go test -run TestAuthCommand_Integration -v
```

Expected: PASS

**Step 5: Test CLI end-to-end**

```bash
cd /tmp
mkdir test-auth-app
cd test-auth-app
echo 'module = "testapp"' > .lvtrc
mkdir -p internal/database
/path/to/lvt auth
```

Expected: Creates all auth files and shows next steps

**Step 6: Commit**

```bash
git add cmd/lvt/commands/auth.go cmd/lvt/commands/auth_test.go
git commit -m "feat(lvt): wire auth command to generator

Connect auth command to generator, load project config, and
generate all auth files. Provides clear next steps for users.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Add go.mod dependencies update

**Files:**
- Modify: `cmd/lvt/internal/generator/generator.go`

**Step 1: Write test for dependency updates**

Add to `cmd/lvt/internal/generator/auth_test.go`:

```go
func TestGenerateAuth_UpdateDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module testapp

go 1.21

require (
	github.com/livefir/livetemplate v0.1.0
)
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644)

	err := GenerateAuth(tmpDir, &AuthConfig{
		EnablePassword: true,
		EnableCSRF:     true,
	})
	if err != nil {
		t.Fatalf("GenerateAuth failed: %v", err)
	}

	// Read updated go.mod
	content, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	contentStr := string(content)

	// Check for bcrypt
	if !strings.Contains(contentStr, "golang.org/x/crypto") {
		t.Error("go.mod missing golang.org/x/crypto dependency")
	}

	// Check for gorilla/csrf
	if !strings.Contains(contentStr, "github.com/gorilla/csrf") {
		t.Error("go.mod missing github.com/gorilla/csrf dependency")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_UpdateDependencies -v
```

Expected: FAIL - dependencies not added

**Step 3: Add dependency update logic**

Modify `cmd/lvt/internal/generator/generator.go` in `GenerateAuth`:

```go
import (
	"os/exec"
)

// Add at end of GenerateAuth function
// Update go.mod dependencies
dependencies := []string{}
if config.EnablePassword {
	dependencies = append(dependencies, "golang.org/x/crypto@latest")
}
if config.EnableCSRF {
	dependencies = append(dependencies, "github.com/gorilla/csrf@latest")
}

if len(dependencies) > 0 {
	args := append([]string{"get"}, dependencies...)
	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update dependencies: %w\n%s", err, output)
	}
}

return nil
```

**Step 4: Run test to verify it passes**

```bash
cd cmd/lvt/internal/generator
go test -run TestGenerateAuth_UpdateDependencies -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add cmd/lvt/internal/generator/auth_test.go cmd/lvt/internal/generator/generator.go
git commit -m "feat(lvt): auto-update dependencies for auth

Automatically add golang.org/x/crypto and gorilla/csrf
dependencies to go.mod when generating auth system.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Summary

This plan creates the foundational infrastructure for `lvt gen auth`. The next phases would include:

**Phase 2**: Auth handler templates (registration, login, logout)
**Phase 3**: Custom authenticator template
**Phase 4**: Middleware templates (route protection, CSRF)
**Phase 5**: Kit-specific templates (Tailwind, Bulma, Pico, None)
**Phase 6**: E2E tests with chromedp

Each phase follows the same TDD approach with bite-sized tasks.

---

## Testing the Generated Code

After completing these tasks, test the generator:

```bash
# Create test app
mkdir /tmp/testapp
cd /tmp/testapp
echo 'module = "github.com/test/testapp"' > .lvtrc
mkdir -p internal/database

# Generate auth
/path/to/lvt auth

# Verify files
ls internal/shared/password/  # Should have password.go
ls internal/shared/email/     # Should have email.go
ls internal/database/migrations/  # Should have auth migration
cat internal/database/queries.sql  # Should have auth queries

# Run tests
go test ./...
```
