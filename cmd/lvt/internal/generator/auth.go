package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

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

	return nil
}
