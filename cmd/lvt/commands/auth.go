package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	var structName, tableName string
	var positionalArgs []string

	// Separate flags from positional arguments
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
		default:
			if !startsWithDash(arg) {
				positionalArgs = append(positionalArgs, arg)
			}
		}
	}

	// Parse positional arguments: [struct_name] [table_name]
	if len(positionalArgs) > 0 {
		structName = positionalArgs[0]
	}
	if len(positionalArgs) > 1 {
		tableName = positionalArgs[1]
	}

	// Default to User/users if not specified
	if structName == "" {
		structName = "User"
	}
	if tableName == "" {
		tableName = pluralizeNoun(structName)
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
		StructName:          structName,
		TableName:           tableName,
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

// startsWithDash checks if a string starts with a dash (flag indicator)
func startsWithDash(s string) bool {
	return strings.HasPrefix(s, "-")
}

// pluralizeNoun converts a singular noun to plural (simple English rules)
func pluralizeNoun(word string) string {
	if word == "" {
		return ""
	}

	lower := strings.ToLower(word)

	// Handle special cases
	if lower == "user" {
		return "users"
	}
	if lower == "account" {
		return "accounts"
	}
	if lower == "admin" {
		return "admins"
	}
	if lower == "member" {
		return "members"
	}

	// General rules
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") ||
		strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return lower + "es"
	}

	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		// Check if preceded by consonant
		secondLast := lower[len(lower)-2]
		if !isVowel(secondLast) {
			return lower[:len(lower)-1] + "ies"
		}
	}

	return lower + "s"
}

// isVowel checks if a character is a vowel
func isVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}
