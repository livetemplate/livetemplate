package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
	"github.com/livefir/livetemplate/cmd/lvt/internal/validator"
)

func Kits(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: list, create, info, validate")
	}

	command := args[0]

	switch command {
	case "list":
		return listKits(args[1:])
	case "create":
		return createKit(args[1:])
	case "info":
		return infoKit(args[1:])
	case "validate":
		return validateKit(args[1:])
	default:
		return fmt.Errorf("unknown command: %s (expected: list, create, info, validate)", command)
	}
}

func listKits(args []string) error {
	// Parse flags
	filter := "all"   // default: show all
	format := "table" // default: table format
	search := ""      // default: no search
	var filteredArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--filter" && i+1 < len(args) {
			filter = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--format" && i+1 < len(args) {
			format = args[i+1]
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
	opts := &kits.KitSearchOptions{
		Query: search,
	}

	// Set source filter if not "all"
	if filter != "all" {
		opts.Source = kits.KitSource(filter)
	}

	// Load kits with filtering
	loader := kits.DefaultLoader()
	filtered, err := loader.List(opts)
	if err != nil {
		return fmt.Errorf("failed to list kits: %w", err)
	}

	// Output in requested format
	switch format {
	case "json":
		return outputKitsJSON(filtered)
	case "simple":
		return outputKitsSimple(filtered)
	case "table":
		return outputKitsTable(filtered)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func outputKitsTable(kitList []*kits.KitInfo) error {
	if len(kitList) == 0 {
		fmt.Println("No kits found")
		return nil
	}

	// Calculate column widths
	maxName := len("NAME")
	maxSource := len("SOURCE")
	maxCDN := len("CDN")
	maxDescription := len("DESCRIPTION")

	for _, kit := range kitList {
		if len(kit.Manifest.Name) > maxName {
			maxName = len(kit.Manifest.Name)
		}
		if len(kit.Source) > maxSource {
			maxSource = len(kit.Source)
		}
		cdnStatus := "Yes"
		if kit.Manifest.CDN == "" {
			cdnStatus = "No"
		}
		if len(cdnStatus) > maxCDN {
			maxCDN = len(cdnStatus)
		}
		// Limit description to 50 chars for display
		desc := kit.Manifest.Description
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
		maxSource, "SOURCE",
		maxCDN, "CDN",
		maxDescription, "DESCRIPTION")
	fmt.Println(strings.Repeat("-", maxName+maxSource+maxCDN+maxDescription+6))

	// Print rows
	for _, kit := range kitList {
		desc := kit.Manifest.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}

		cdnStatus := "Yes"
		if kit.Manifest.CDN == "" {
			cdnStatus = "No"
		}

		// Add source indicator
		sourceDisplay := string(kit.Source)
		switch kit.Source {
		case kits.SourceSystem:
			sourceDisplay = "📦 " + string(kit.Source)
		case kits.SourceLocal:
			sourceDisplay = "🔧 " + string(kit.Source)
		case kits.SourceCommunity:
			sourceDisplay = "🌐 " + string(kit.Source)
		}

		fmt.Printf("%-*s  %-*s  %-*s  %-*s\n",
			maxName, kit.Manifest.Name,
			maxSource, sourceDisplay,
			maxCDN, cdnStatus,
			maxDescription, desc)
	}

	fmt.Printf("\nTotal: %d kit(s)\n", len(kitList))
	return nil
}

func outputKitsSimple(kitList []*kits.KitInfo) error {
	for _, kit := range kitList {
		fmt.Println(kit.Manifest.Name)
	}
	return nil
}

func outputKitsJSON(kitList []*kits.KitInfo) error {
	data, err := json.MarshalIndent(kitList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func createKit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("kit name required")
	}

	kitName := args[0]

	// Parse flags (none for now, but structure for future)
	var filteredArgs []string
	for i := 1; i < len(args); i++ {
		filteredArgs = append(filteredArgs, args[i])
	}

	// Get current directory or use .lvt/kits
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create in .lvt/kits/[name]
	kitDir := filepath.Join(currentDir, ".lvt", "kits", kitName)
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		return fmt.Errorf("failed to create kit directory: %w", err)
	}

	// Create kit.yaml
	kitYAML := fmt.Sprintf(`name: %s
version: 1.0.0
description: A custom CSS framework kit
framework: %s
author: ""
cdn: ""
`, kitName, kitName)

	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(kitYAML), 0644); err != nil {
		return fmt.Errorf("failed to create kit.yaml: %w", err)
	}

	// Create helpers.go stub
	helpersContent := `package ` + kitName + `

import "github.com/livefir/livetemplate/cmd/lvt/internal/kits"

// Helpers implements the kits.CSSHelpers interface
type Helpers struct{}

func NewHelpers() kits.CSSHelpers {
	return &Helpers{}
}

// Layout & Structure
func (h *Helpers) ContainerClass() string                    { return "" }
func (h *Helpers) RowClass() string                          { return "" }
func (h *Helpers) ColClass(width int) string                 { return "" }
func (h *Helpers) BoxClass() string                          { return "" }
func (h *Helpers) CardClass() string                         { return "" }
func (h *Helpers) SectionClass() string                      { return "" }

// Typography
func (h *Helpers) TitleClass(level int) string               { return "" }
func (h *Helpers) SubtitleClass() string                     { return "" }
func (h *Helpers) TextClass() string                         { return "" }
func (h *Helpers) TextMutedClass() string                    { return "" }
func (h *Helpers) TextDangerClass() string                   { return "" }
func (h *Helpers) TextSuccessClass() string                  { return "" }

// Forms
func (h *Helpers) FormClass() string                         { return "" }
func (h *Helpers) FieldClass() string                        { return "" }
func (h *Helpers) LabelClass() string                        { return "" }
func (h *Helpers) InputClass() string                        { return "" }
func (h *Helpers) TextareaClass() string                     { return "" }
func (h *Helpers) SelectClass() string                       { return "" }
func (h *Helpers) CheckboxClass() string                     { return "" }
func (h *Helpers) RadioClass() string                        { return "" }

// Buttons
func (h *Helpers) ButtonClass(variant string) string         { return "" }
func (h *Helpers) ButtonGroupClass() string                  { return "" }

// Tables
func (h *Helpers) TableClass() string                        { return "" }
func (h *Helpers) TableHeadClass() string                    { return "" }
func (h *Helpers) TableBodyClass() string                    { return "" }
func (h *Helpers) TableRowClass() string                     { return "" }
func (h *Helpers) TableHeaderClass() string                  { return "" }
func (h *Helpers) TableCellClass() string                    { return "" }

// Navigation
func (h *Helpers) NavbarClass() string                       { return "" }
func (h *Helpers) NavbarBrandClass() string                  { return "" }
func (h *Helpers) NavbarMenuClass() string                   { return "" }
func (h *Helpers) NavbarItemClass() string                   { return "" }
func (h *Helpers) BreadcrumbClass() string                   { return "" }
func (h *Helpers) BreadcrumbItemClass() string               { return "" }
func (h *Helpers) TabsClass() string                         { return "" }
func (h *Helpers) TabClass(active bool) string               { return "" }

// Components
func (h *Helpers) ModalClass() string                        { return "" }
func (h *Helpers) ModalOverlayClass() string                 { return "" }
func (h *Helpers) ModalContentClass() string                 { return "" }
func (h *Helpers) ModalHeaderClass() string                  { return "" }
func (h *Helpers) ModalBodyClass() string                    { return "" }
func (h *Helpers) ModalFooterClass() string                  { return "" }
func (h *Helpers) AlertClass(variant string) string          { return "" }
func (h *Helpers) BadgeClass(variant string) string          { return "" }
func (h *Helpers) DropdownClass() string                     { return "" }
func (h *Helpers) DropdownMenuClass() string                 { return "" }
func (h *Helpers) DropdownItemClass() string                 { return "" }

// Pagination
func (h *Helpers) PaginationClass() string                   { return "" }
func (h *Helpers) PaginationListClass() string               { return "" }
func (h *Helpers) PaginationItemClass() string               { return "" }
func (h *Helpers) PaginationButtonClass(state string) string { return "" }

// Loading & Progress
func (h *Helpers) SpinnerClass() string                      { return "" }
func (h *Helpers) ProgressClass() string                     { return "" }
func (h *Helpers) ProgressBarClass() string                  { return "" }

// Utility
func (h *Helpers) HiddenClass() string                       { return "" }
func (h *Helpers) VisibleClass() string                      { return "" }
func (h *Helpers) FlexClass() string                         { return "" }
func (h *Helpers) GridClass() string                         { return "" }
func (h *Helpers) SpacingClass(size string) string           { return "" }

// CDN & Assets
func (h *Helpers) CSSCDN() string                            { return "" }
func (h *Helpers) JSCDN() string                             { return "" }

// Template Utilities
func (h *Helpers) Dict(values ...interface{}) (map[string]interface{}, error) {
	return kits.Dict(values...)
}

func (h *Helpers) Until(n int) []int {
	return kits.Until(n)
}

func (h *Helpers) Add(a, b int) int {
	return kits.Add(a, b)
}
`

	if err := os.WriteFile(filepath.Join(kitDir, "helpers.go"), []byte(helpersContent), 0644); err != nil {
		return fmt.Errorf("failed to create helpers.go: %w", err)
	}

	// Create README.md
	readmeContent := fmt.Sprintf(`# %s Kit

A custom CSS framework kit for LiveTemplate.

## Description

Add your kit description here.

## Installation

This kit is available as a local kit. To use it:

`+"```"+`bash
lvt gen users name email --css %s
`+"```"+`

## Customization

Edit the CSS helper methods in `+"`helpers.go`"+` to match your framework's class names.

## CDN (Optional)

If your framework has a CDN, add it to `+"`kit.yaml`"+`:

`+"```"+`yaml
cdn: "https://cdn.example.com/your-framework.css"
`+"```"+`

## Documentation

Add usage examples and documentation for your kit here.
`, kitName, kitName)

	if err := os.WriteFile(filepath.Join(kitDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	fmt.Println("✅ Kit created successfully!")
	fmt.Println()
	fmt.Println("Files created:")
	fmt.Printf("  %s/kit.yaml\n", kitDir)
	fmt.Printf("  %s/helpers.go\n", kitDir)
	fmt.Printf("  %s/README.md\n", kitDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit helper methods: %s/helpers.go\n", kitDir)
	fmt.Printf("  2. Update metadata: %s/kit.yaml\n", kitDir)
	fmt.Printf("  3. Use the kit: lvt gen resource name --css %s\n", kitName)
	fmt.Println()

	return nil
}

func infoKit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("kit name required")
	}

	kitName := args[0]

	// Load kit
	loader := kits.DefaultLoader()
	kit, err := loader.Load(kitName)
	if err != nil {
		return fmt.Errorf("failed to load kit %q: %w", kitName, err)
	}

	// Display kit info
	fmt.Printf("Kit: %s\n", kit.Manifest.Name)
	fmt.Printf("Description: %s\n", kit.Manifest.Description)
	fmt.Printf("Framework: %s\n", kit.Manifest.Framework)
	fmt.Printf("Version: %s\n", kit.Manifest.Version)
	fmt.Printf("Source: %s\n", string(kit.Source))

	if kit.Manifest.Author != "" {
		fmt.Printf("Author: %s\n", kit.Manifest.Author)
	}

	if kit.Manifest.CDN != "" {
		fmt.Printf("CDN: %s\n", kit.Manifest.CDN)
	}

	if len(kit.Manifest.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(kit.Manifest.Tags, ", "))
	}

	fmt.Printf("Path: %s\n", kit.Path)

	// Show README if available
	readmePath := filepath.Join(kit.Path, "README.md")
	if content, err := os.ReadFile(readmePath); err == nil {
		fmt.Println()
		fmt.Println("Documentation:")
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println(string(content))
	}

	return nil
}

func validateKit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("kit path required")
	}

	kitPath := args[0]

	// Run validation
	result := validator.ValidateKit(kitPath)

	// Print results
	fmt.Println(result.Format())

	// Return error if validation failed
	if !result.Valid {
		return fmt.Errorf("validation failed with %d error(s)", result.ErrorCount())
	}

	return nil
}
