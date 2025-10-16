package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/components"
	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
	"github.com/livefir/livetemplate/cmd/lvt/internal/validator"
)

func Components(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: list, create, info, validate")
	}

	command := args[0]

	switch command {
	case "list":
		return listComponents(args[1:])
	case "create":
		return createComponent(args[1:])
	case "info":
		return infoComponent(args[1:])
	case "validate":
		return validateComponent(args[1:])
	default:
		return fmt.Errorf("unknown command: %s (expected: list, create, info, validate)", command)
	}
}

func listComponents(args []string) error {
	// Parse flags
	filter := "all"   // default: show all
	format := "table" // default: table format
	category := ""    // default: all categories
	search := ""      // default: no search
	var filteredArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--filter" && i+1 < len(args) {
			filter = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--format" && i+1 < len(args) {
			format = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--category" && i+1 < len(args) {
			category = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--search" && i+1 < len(args) {
			search = args[i+1]
			i++ // skip next arg
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	// Validate filter
	validFilters := map[string]bool{"all": true, "system": true, "local": true, "community": true}
	if !validFilters[filter] {
		return fmt.Errorf("invalid filter: %s (valid: all, system, local, community)", filter)
	}

	// Validate format
	validFormats := map[string]bool{"table": true, "json": true, "simple": true}
	if !validFormats[format] {
		return fmt.Errorf("invalid format: %s (valid: table, json, simple)", format)
	}

	// Build search options
	opts := &components.ComponentSearchOptions{
		Query: search,
	}

	// Set source filter if not "all"
	if filter != "all" {
		opts.Source = components.ComponentSource(filter)
	}

	// Set category filter if specified
	if category != "" {
		opts.Category = components.ComponentCategory(category)
	}

	// Load components with filtering
	loader := components.DefaultLoader()
	filtered, err := loader.List(opts)
	if err != nil {
		return fmt.Errorf("failed to list components: %w", err)
	}

	// Output in requested format
	switch format {
	case "json":
		return outputComponentsJSON(filtered)
	case "simple":
		return outputComponentsSimple(filtered)
	case "table":
		return outputComponentsTable(filtered)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func outputComponentsTable(comps []*components.Component) error {
	if len(comps) == 0 {
		fmt.Println("No components found")
		return nil
	}

	// Calculate column widths
	maxName := len("NAME")
	maxCategory := len("CATEGORY")
	maxSource := len("SOURCE")
	maxDescription := len("DESCRIPTION")

	for _, comp := range comps {
		if len(comp.Manifest.Name) > maxName {
			maxName = len(comp.Manifest.Name)
		}
		if len(comp.Manifest.Category) > maxCategory {
			maxCategory = len(comp.Manifest.Category)
		}
		if len(comp.Source) > maxSource {
			maxSource = len(comp.Source)
		}
		// Limit description to 50 chars for display
		desc := comp.Manifest.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		if len(desc) > maxDescription {
			maxDescription = len(desc)
		}
	}

	// Print header
	fmt.Printf("%-*s  %-*s  %-*s  %-*s\n",
		maxName, "NAME",
		maxCategory, "CATEGORY",
		maxSource, "SOURCE",
		maxDescription, "DESCRIPTION")
	fmt.Println(strings.Repeat("-", maxName+maxCategory+maxSource+maxDescription+6))

	// Print rows
	for _, comp := range comps {
		desc := comp.Manifest.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		// Add source indicator
		sourceDisplay := string(comp.Source)
		switch comp.Source {
		case components.SourceSystem:
			sourceDisplay = "📦 " + string(comp.Source)
		case components.SourceLocal:
			sourceDisplay = "🔧 " + string(comp.Source)
		case components.SourceCommunity:
			sourceDisplay = "🌐 " + string(comp.Source)
		}

		fmt.Printf("%-*s  %-*s  %-*s  %-*s\n",
			maxName, comp.Manifest.Name,
			maxCategory, string(comp.Manifest.Category),
			maxSource, sourceDisplay,
			maxDescription, desc)
	}

	fmt.Printf("\nTotal: %d component(s)\n", len(comps))
	return nil
}

func outputComponentsSimple(comps []*components.Component) error {
	for _, comp := range comps {
		fmt.Println(comp.Manifest.Name)
	}
	return nil
}

func outputComponentsJSON(comps []*components.Component) error {
	data, err := json.MarshalIndent(comps, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func createComponent(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("component name required")
	}

	componentName := args[0]

	// Parse flags
	category := "ui"  // default category
	kit := "tailwind" // default kit
	var filteredArgs []string

	for i := 1; i < len(args); i++ {
		if args[i] == "--category" && i+1 < len(args) {
			category = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--kit" && i+1 < len(args) {
			kit = args[i+1]
			i++ // skip next arg
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	// Validate kit exists
	kitLoader := kits.DefaultLoader()
	if _, err := kitLoader.Load(kit); err != nil {
		return fmt.Errorf("invalid kit: %s (run 'lvt kits list' to see available kits)", kit)
	}

	// Get current directory or use .lvt/components
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create in .lvt/components/[name]
	componentDir := filepath.Join(currentDir, ".lvt", "components", componentName)
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return fmt.Errorf("failed to create component directory: %w", err)
	}

	// Create component.yaml
	componentYAML := fmt.Sprintf(`name: %s
version: 1.0.0
description: A custom component
category: %s
tags: []
templates:
  - %s.tmpl
`, componentName, category, componentName)

	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(componentYAML), 0644); err != nil {
		return fmt.Errorf("failed to create component.yaml: %w", err)
	}

	// Create template file
	templateContent := `[[ define "` + componentName + `" ]]
<div class="[[ containerClass ]]">
  <!-- Your component markup here -->
  <p>Component: ` + componentName + `</p>
</div>
[[ end ]]
`

	if err := os.WriteFile(filepath.Join(componentDir, componentName+".tmpl"), []byte(templateContent), 0644); err != nil {
		return fmt.Errorf("failed to create template file: %w", err)
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s

A custom LiveTemplate component.

## Usage

`+"```"+`go
// In your template:
[[ template "%s" . ]]
`+"```"+`

## Description

Add your component description here.

## Properties

Document any data properties your component expects here.

## Examples

Add usage examples here.
`, componentName, componentName)

	if err := os.WriteFile(filepath.Join(componentDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	fmt.Println("✅ Component created successfully!")
	fmt.Println()
	fmt.Println("Files created:")
	fmt.Printf("  %s/component.yaml\n", componentDir)
	fmt.Printf("  %s/%s.tmpl\n", componentDir, componentName)
	fmt.Printf("  %s/README.md\n", componentDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit the component: %s/%s.tmpl\n", componentDir, componentName)
	fmt.Printf("  2. Update metadata: %s/component.yaml\n", componentDir)
	fmt.Printf("  3. Use in templates: [[ template \"%s\" . ]]\n", componentName)
	fmt.Println()

	return nil
}

func infoComponent(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("component name required")
	}

	componentName := args[0]

	// Load component
	loader := components.DefaultLoader()
	comp, err := loader.Load(componentName)
	if err != nil {
		return fmt.Errorf("failed to load component %q: %w", componentName, err)
	}

	// Display component info
	fmt.Printf("Component: %s\n", comp.Manifest.Name)
	fmt.Printf("Description: %s\n", comp.Manifest.Description)
	fmt.Printf("Category: %s\n", string(comp.Manifest.Category))
	fmt.Printf("Version: %s\n", comp.Manifest.Version)
	fmt.Printf("Source: %s\n", string(comp.Source))

	if comp.Manifest.Author != "" {
		fmt.Printf("Author: %s\n", comp.Manifest.Author)
	}

	if len(comp.Manifest.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(comp.Manifest.Tags, ", "))
	}

	if len(comp.Manifest.Dependencies) > 0 {
		fmt.Printf("Dependencies: %s\n", strings.Join(comp.Manifest.Dependencies, ", "))
	}

	if len(comp.Manifest.Templates) > 0 {
		fmt.Printf("Templates: %s\n", strings.Join(comp.Manifest.Templates, ", "))
	}

	fmt.Printf("Path: %s\n", comp.Path)

	// Show README if available
	readmePath := filepath.Join(comp.Path, "README.md")
	if content, err := os.ReadFile(readmePath); err == nil {
		fmt.Println()
		fmt.Println("Documentation:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println(string(content))
	}

	return nil
}

func validateComponent(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("component path required")
	}

	componentPath := args[0]

	// Run validation
	result := validator.ValidateComponent(componentPath)

	// Print results
	fmt.Println(result.Format())

	// Return error if validation failed
	if !result.Valid {
		return fmt.Errorf("validation failed with %d error(s)", result.ErrorCount())
	}

	return nil
}
