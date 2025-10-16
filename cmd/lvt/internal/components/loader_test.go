package components

import (
	"testing"
)

// TestLoadSystemComponents tests loading all system components from embedded FS
func TestLoadSystemComponents(t *testing.T) {
	loader := DefaultLoader()

	// List of all system components that should be available
	expectedComponents := []string{
		"layout",
		"form",
		"table",
		"pagination",
		"toolbar",
		"detail",
	}

	for _, name := range expectedComponents {
		t.Run("Load_"+name, func(t *testing.T) {
			comp, err := loader.Load(name)
			if err != nil {
				t.Fatalf("Failed to load system component %q: %v", name, err)
			}

			// Verify component is loaded
			if comp == nil {
				t.Fatalf("Component %q is nil", name)
			}

			// Verify source is system
			if comp.Source != SourceSystem {
				t.Errorf("Component %q source = %v, want %v", name, comp.Source, SourceSystem)
			}

			// Verify manifest name matches
			if comp.Manifest.Name != name {
				t.Errorf("Component %q manifest name = %q, want %q", name, comp.Manifest.Name, name)
			}

			// Verify manifest is valid
			if err := comp.Manifest.Validate(); err != nil {
				t.Errorf("Component %q manifest validation failed: %v", name, err)
			}
		})
	}
}

// TestListSystemComponents tests listing all system components
func TestListSystemComponents(t *testing.T) {
	loader := DefaultLoader()

	// List all system components
	components, err := loader.List(&ComponentSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system components: %v", err)
	}

	// We should have at least 6 system components
	if len(components) < 6 {
		t.Errorf("Expected at least 6 system components, got %d", len(components))
	}

	// Verify all components are from system source
	for _, comp := range components {
		if comp.Source != SourceSystem {
			t.Errorf("Component %q source = %v, want %v", comp.Manifest.Name, comp.Source, SourceSystem)
		}
	}
}

// TestComponentManifestParsing tests that all component manifests parse correctly
func TestComponentManifestParsing(t *testing.T) {
	loader := DefaultLoader()

	components, err := loader.List(&ComponentSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system components: %v", err)
	}

	for _, comp := range components {
		t.Run("Manifest_"+comp.Manifest.Name, func(t *testing.T) {
			// Verify required manifest fields
			if comp.Manifest.Name == "" {
				t.Error("Component name is empty")
			}
			if comp.Manifest.Version == "" {
				t.Error("Component version is empty")
			}
			if comp.Manifest.Description == "" {
				t.Error("Component description is empty")
			}
			if comp.Manifest.Category == "" {
				t.Error("Component category is empty")
			}
			if comp.Manifest.Author == "" {
				t.Error("Component author is empty")
			}
			if comp.Manifest.License == "" {
				t.Error("Component license is empty")
			}
			if len(comp.Manifest.Templates) == 0 {
				t.Error("Component has no templates")
			}

			// Verify all templates are loaded
			if len(comp.Templates) != len(comp.Manifest.Templates) {
				t.Errorf("Component templates count mismatch: got %d, want %d",
					len(comp.Templates), len(comp.Manifest.Templates))
			}

			// Verify each template in manifest is actually loaded
			for _, templateFile := range comp.Manifest.Templates {
				if _, exists := comp.Templates[templateFile]; !exists {
					t.Errorf("Template %q declared in manifest but not loaded", templateFile)
				}
			}
		})
	}
}

// TestComponentTemplateParsing tests that all component templates parse correctly
func TestComponentTemplateParsing(t *testing.T) {
	loader := DefaultLoader()

	components, err := loader.List(&ComponentSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system components: %v", err)
	}

	for _, comp := range components {
		t.Run("Templates_"+comp.Manifest.Name, func(t *testing.T) {
			// Verify all templates parse correctly
			for filename, tmpl := range comp.Templates {
				if tmpl == nil {
					t.Errorf("Template %q is nil", filename)
					continue
				}

				// Verify template has a name
				if tmpl.Name() == "" {
					t.Errorf("Template %q has no name", filename)
				}

				// Verify template is parseable by checking for any defined templates
				if len(tmpl.Templates()) == 0 {
					t.Errorf("Template %q has no defined templates", filename)
				}
			}
		})
	}
}

// TestComponentCache tests that component caching works correctly
func TestComponentCache(t *testing.T) {
	loader := DefaultLoader()

	// Load a component
	comp1, err := loader.Load("layout")
	if err != nil {
		t.Fatalf("Failed to load component: %v", err)
	}

	// Load the same component again
	comp2, err := loader.Load("layout")
	if err != nil {
		t.Fatalf("Failed to load component again: %v", err)
	}

	// Verify it's the same instance (cached)
	if comp1 != comp2 {
		t.Error("Component not cached: different instances returned")
	}

	// Clear cache
	loader.ClearCache()

	// Load again after cache clear
	comp3, err := loader.Load("layout")
	if err != nil {
		t.Fatalf("Failed to load component after cache clear: %v", err)
	}

	// Should be a different instance
	if comp1 == comp3 {
		t.Error("Cache not cleared: same instance returned")
	}
}

// TestComponentNotFound tests error handling for non-existent components
func TestComponentNotFound(t *testing.T) {
	loader := DefaultLoader()

	_, err := loader.Load("nonexistent-component")
	if err == nil {
		t.Error("Expected error for non-existent component, got nil")
	}

	// Verify it's the right error type
	if _, ok := err.(ErrComponentNotFound); !ok {
		t.Errorf("Expected ErrComponentNotFound, got %T: %v", err, err)
	}
}

// TestComponentCategoryFiltering tests filtering components by category
func TestComponentCategoryFiltering(t *testing.T) {
	loader := DefaultLoader()

	categories := []ComponentCategory{
		CategoryLayout,
		CategoryForm,
		CategoryTable,
		CategoryNavigation,
		CategoryToolbar,
		CategoryDetail,
	}

	for _, category := range categories {
		t.Run("Category_"+string(category), func(t *testing.T) {
			components, err := loader.List(&ComponentSearchOptions{
				Source:   SourceSystem,
				Category: category,
			})
			if err != nil {
				t.Fatalf("Failed to list components for category %q: %v", category, err)
			}

			// Verify all returned components match the category
			for _, comp := range components {
				if comp.Manifest.Category != category {
					t.Errorf("Component %q has category %q, want %q",
						comp.Manifest.Name, comp.Manifest.Category, category)
				}
			}
		})
	}
}

// TestComponentInputsValidation tests that component inputs are properly defined
func TestComponentInputsValidation(t *testing.T) {
	loader := DefaultLoader()

	components, err := loader.List(&ComponentSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system components: %v", err)
	}

	for _, comp := range components {
		t.Run("Inputs_"+comp.Manifest.Name, func(t *testing.T) {
			// Components should have at least one input
			if len(comp.Manifest.Inputs) == 0 {
				t.Logf("Warning: Component %q has no inputs defined", comp.Manifest.Name)
			}

			// Verify each input has required fields
			for i, input := range comp.Manifest.Inputs {
				if input.Name == "" {
					t.Errorf("Input %d has empty name", i)
				}
				if input.Type == "" {
					t.Errorf("Input %d (%q) has empty type", i, input.Name)
				}
			}
		})
	}
}

// TestComponentDependencies tests that component dependencies are valid
func TestComponentDependencies(t *testing.T) {
	loader := DefaultLoader()

	components, err := loader.List(&ComponentSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system components: %v", err)
	}

	// Build a map of available components
	available := make(map[string]bool)
	for _, comp := range components {
		available[comp.Manifest.Name] = true
	}

	// Check each component's dependencies
	for _, comp := range components {
		t.Run("Dependencies_"+comp.Manifest.Name, func(t *testing.T) {
			for _, dep := range comp.Manifest.Dependencies {
				if !available[dep] {
					t.Errorf("Component %q depends on %q which is not available",
						comp.Manifest.Name, dep)
				}
			}
		})
	}
}
