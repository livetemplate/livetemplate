package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

type AuthConfig struct {
	ModuleName          string
	EnablePassword      bool
	EnableMagicLink     bool
	EnableEmailConfirm  bool
	EnablePasswordReset bool
	EnableSessionsUI    bool
	EnableCSRF          bool
}

func GenerateAuth(projectRoot string, config *AuthConfig) error {
	// Load kit loader
	kitLoader := kits.DefaultLoader()

	// Create directories
	passwordDir := filepath.Join(projectRoot, "internal", "shared", "password")
	if err := os.MkdirAll(passwordDir, 0755); err != nil {
		return fmt.Errorf("failed to create password directory: %w", err)
	}

	// Generate password.go if password auth enabled
	if config.EnablePassword {
		templateContent, err := kitLoader.LoadKitTemplate("multi", "auth/password.go.tmpl")
		if err != nil {
			return fmt.Errorf("failed to load password template: %w", err)
		}

		outputPath := filepath.Join(passwordDir, "password.go")

		tmpl, err := template.New("password").Parse(string(templateContent))
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

	// Generate email.go if email features enabled
	if config.EnableEmailConfirm || config.EnablePasswordReset {
		emailDir := filepath.Join(projectRoot, "internal", "shared", "email")
		if err := os.MkdirAll(emailDir, 0755); err != nil {
			return fmt.Errorf("failed to create email directory: %w", err)
		}

		templateContent, err := kitLoader.LoadKitTemplate("multi", "auth/email.go.tmpl")
		if err != nil {
			return fmt.Errorf("failed to load email template: %w", err)
		}

		outputPath := filepath.Join(emailDir, "email.go")

		tmpl, err := template.New("email").Parse(string(templateContent))
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

	// Generate migration
	migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")
	migrationFile := fmt.Sprintf("%s_create_auth_tables.sql", timestamp)
	migrationPath := filepath.Join(migrationsDir, migrationFile)

	templateContent, err := kitLoader.LoadKitTemplate("multi", "auth/migration.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to load migration template: %w", err)
	}

	tmpl, err := template.New("migration").Parse(string(templateContent))
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

	// Append to queries.sql (or create if doesn't exist)
	queriesPath := filepath.Join(projectRoot, "internal", "database", "queries.sql")

	templateContent, err = kitLoader.LoadKitTemplate("multi", "auth/queries.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to load queries template: %w", err)
	}

	tmpl, err = template.New("queries").Parse(string(templateContent))
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
		if _, err := file.WriteString("\n\n"); err != nil {
			return fmt.Errorf("failed to write separator: %w", err)
		}
	}

	if err := tmpl.Execute(file, config); err != nil {
		return fmt.Errorf("failed to execute queries template: %w", err)
	}

	// Update go.mod dependencies if go.mod exists
	goModPath := filepath.Join(projectRoot, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
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
	}

	return nil
}
