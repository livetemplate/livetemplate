package validator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

// ValidateKit validates a kit directory
func ValidateKit(path string) *ValidationResult {
	result := NewValidationResult()

	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Kit directory not found: %s", path), path, 0)
		return result
	}

	if !info.IsDir() {
		result.AddError("Path is not a directory", path, 0)
		return result
	}

	// Validate structure
	structureResult := validateKitStructure(path)
	result.Merge(structureResult)

	// Validate manifest
	manifestResult := validateKitManifest(path)
	result.Merge(manifestResult)

	// Validate helpers (if helpers.go exists)
	helpersPath := filepath.Join(path, "helpers.go")
	if _, err := os.Stat(helpersPath); err == nil {
		helpersResult := validateKitHelpers(helpersPath)
		result.Merge(helpersResult)
	}

	// Validate README
	readmeResult := validateKitReadme(path)
	result.Merge(readmeResult)

	return result
}

// validateKitStructure checks if required files exist
func validateKitStructure(path string) *ValidationResult {
	result := NewValidationResult()

	// Check for kit.yaml
	manifestPath := filepath.Join(path, kits.ManifestFileName)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		result.AddError("Missing kit.yaml", path, 0)
	}

	// Check for helpers.go (recommended but not required for CDN-only kits)
	helpersPath := filepath.Join(path, "helpers.go")
	if _, err := os.Stat(helpersPath); os.IsNotExist(err) {
		result.AddWarning("Missing helpers.go - kit will need to use system helpers", path, 0)
	}

	return result
}

// validateKitManifest validates the kit.yaml file
func validateKitManifest(path string) *ValidationResult {
	result := NewValidationResult()

	manifestPath := filepath.Join(path, kits.ManifestFileName)

	// Load manifest
	manifest, err := kits.LoadManifest(path)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to load manifest: %v", err), manifestPath, 0)
		return result
	}

	// Validate manifest (uses built-in validation)
	if err := manifest.Validate(); err != nil {
		result.AddError(fmt.Sprintf("Manifest validation failed: %v", err), manifestPath, 0)
		return result
	}

	// Check for recommended fields
	if manifest.Author == "" {
		result.AddWarning("Author field is empty", manifestPath, 0)
	}

	if manifest.License == "" {
		result.AddWarning("License field is empty", manifestPath, 0)
	}

	if manifest.CDN == "" && manifest.CustomCSS == "" {
		result.AddWarning("No CDN or custom CSS specified", manifestPath, 0)
	}

	if len(manifest.Tags) == 0 {
		result.AddInfo("No tags specified - consider adding tags for discoverability", manifestPath, 0)
	}

	return result
}

// validateKitHelpers validates the helpers.go file
func validateKitHelpers(path string) *ValidationResult {
	result := NewValidationResult()

	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		result.AddError(fmt.Sprintf("Failed to parse helpers.go: %v", err), path, 0)
		return result
	}

	// Check package declaration
	if node.Name == nil {
		result.AddError("Missing package declaration", path, 0)
		return result
	}

	// Check for Helpers struct
	hasHelpersStruct := false
	hasNewHelpers := false
	implementedMethods := make(map[string]bool)

	for _, decl := range node.Decls {
		// Check for type declaration (Helpers struct)
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "Helpers" {
						hasHelpersStruct = true
					}
				}
			}
		}

		// Check for methods
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			// Check for NewHelpers constructor
			if funcDecl.Name.Name == "NewHelpers" {
				hasNewHelpers = true
			}

			// Track implemented methods on Helpers type
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				if starExpr, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
					if ident, ok := starExpr.X.(*ast.Ident); ok && ident.Name == "Helpers" {
						implementedMethods[funcDecl.Name.Name] = true
					}
				}
			}
		}
	}

	if !hasHelpersStruct {
		result.AddError("Missing Helpers struct", path, 0)
	}

	if !hasNewHelpers {
		result.AddWarning("Missing NewHelpers() constructor function", path, 0)
	}

	// Check for key CSSHelpers interface methods (sample check)
	requiredMethods := []string{
		"ContainerClass", "BoxClass", "TitleClass",
		"ButtonClass", "InputClass", "TableClass",
		"CSSCDN",
	}

	missingMethods := []string{}
	for _, method := range requiredMethods {
		if !implementedMethods[method] {
			missingMethods = append(missingMethods, method)
		}
	}

	if len(missingMethods) > 0 {
		result.AddWarning(fmt.Sprintf("Missing some key helper methods: %v", missingMethods), path, 0)
		result.AddInfo("Run 'lvt kits create' to generate a complete helpers.go template", "", 0)
	}

	return result
}

// validateKitReadme checks for README.md
func validateKitReadme(path string) *ValidationResult {
	result := NewValidationResult()

	readmePath := filepath.Join(path, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		result.AddWarning("Missing README.md - documentation is recommended", path, 0)
		return result
	}

	// Check if README has content
	data, err := os.ReadFile(readmePath)
	if err != nil {
		result.AddWarning(fmt.Sprintf("Failed to read README.md: %v", err), readmePath, 0)
		return result
	}

	if len(data) < 50 {
		result.AddWarning("README.md appears to be very short", readmePath, 0)
	}

	return result
}
