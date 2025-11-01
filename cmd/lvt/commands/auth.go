package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/livefir/livetemplate/cmd/lvt/internal/config"
	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
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

func Auth(args []string) error {
	flags := &AuthFlags{}

	// Parse flags
	for _, arg := range args {
		switch arg {
		case "--no-registration":
			flags.NoRegistration = true
		case "--no-password":
			flags.NoPassword = true
		case "--no-magic-link":
			flags.NoMagicLink = true
		case "--no-email-confirm":
			flags.NoEmailConfirm = true
		case "--no-password-reset":
			flags.NoPasswordReset = true
		case "--no-sessions-ui":
			flags.NoSessionsUI = true
		case "--no-csrf":
			flags.NoCSRF = true
		}
	}

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
	cfg, err := config.LoadProjectConfig(wd)
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

	fmt.Println("Authentication system generated successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run migrations: lvt migration up")
	fmt.Println("  2. Generate sqlc code: sqlc generate")
	fmt.Println("  3. Update main.go to register auth handler")

	return nil
}
