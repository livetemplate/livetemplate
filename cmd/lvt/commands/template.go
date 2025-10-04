package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

var templateFiles = map[string][]string{
	"resource": {
		"resource/handler.go.tmpl",
		"resource/template.tmpl.tmpl",
		"resource/queries.sql.tmpl",
		"resource/migration.sql.tmpl",
		"resource/ws_test.go.tmpl",
		"resource/e2e_test.go.tmpl",
	},
	"view": {
		"view/handler.go.tmpl",
		"view/template.tmpl.tmpl",
		"view/ws_test.go.tmpl",
		"view/e2e_test.go.tmpl",
	},
	"app": {
		"app/main.go.tmpl",
		"app/db.go.tmpl",
		"app/go.mod.tmpl",
		"app/sqlc.yaml.tmpl",
	},
}

func Template(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: copy <type>")
	}

	command := args[0]

	switch command {
	case "copy":
		if len(args) < 2 {
			return fmt.Errorf("template type required: resource, view, app, or all")
		}
		return copyTemplates(args[1])

	default:
		return fmt.Errorf("unknown command: %s (expected: copy)", command)
	}
}

func copyTemplates(templateType string) error {
	// Get current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create .lvt/templates/ directory
	templateBaseDir := filepath.Join(currentDir, ".lvt", "templates")

	var filesToCopy []string
	var typeName string

	switch templateType {
	case "resource":
		filesToCopy = templateFiles["resource"]
		typeName = "resource"
	case "view":
		filesToCopy = templateFiles["view"]
		typeName = "view"
	case "app":
		filesToCopy = templateFiles["app"]
		typeName = "app"
	case "all":
		filesToCopy = append(filesToCopy, templateFiles["resource"]...)
		filesToCopy = append(filesToCopy, templateFiles["view"]...)
		filesToCopy = append(filesToCopy, templateFiles["app"]...)
		typeName = "all"
	default:
		return fmt.Errorf("unknown template type: %s (expected: resource, view, app, or all)", templateType)
	}

	fmt.Printf("Copying %s templates to .lvt/templates/\n", typeName)

	copiedCount := 0
	for _, templateName := range filesToCopy {
		destPath := filepath.Join(templateBaseDir, templateName)

		if err := generator.CopyEmbeddedTemplate(templateName, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", templateName, err)
		}

		fmt.Printf("  ✓ %s\n", templateName)
		copiedCount++
	}

	fmt.Println()
	fmt.Printf("✅ Copied %d template(s) successfully!\n", copiedCount)
	fmt.Println()
	fmt.Println("Customize your templates in:")
	fmt.Printf("  %s\n", templateBaseDir)
	fmt.Println()
	fmt.Println("Next time you run 'lvt gen' or 'lvt new', your custom templates will be used.")
	fmt.Println()
	fmt.Println("To reset to defaults:")
	fmt.Println("  rm -rf .lvt/templates/")
	fmt.Println()

	return nil
}
