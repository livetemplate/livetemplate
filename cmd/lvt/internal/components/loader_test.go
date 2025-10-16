package components

import (
	"os"
	"path/filepath"
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

// Unit tests for ComponentLoader core functionality

func TestNewLoader_Initialization(t *testing.T) {
	loader := NewLoader(nil)

	if loader == nil {
		t.Fatal("Expected non-nil loader")
	}

	if loader.cache == nil {
		t.Error("Expected cache to be initialized")
	}

	if loader.searchPaths == nil {
		t.Error("Expected searchPaths to be initialized")
	}
}

func TestLoad_FromLocalPath(t *testing.T) {
	// Create temporary component
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-component")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid component
	manifest := `name: test-component
version: 1.0.0
description: A test component
category: layout
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	template := `[[ define "test" ]]
<div>Test Component</div>
[[ end ]]
`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	// Create loader and add search path
	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load component
	comp, err := loader.Load("test-component")
	if err != nil {
		t.Fatalf("Failed to load component: %v", err)
	}

	if comp.Manifest.Name != "test-component" {
		t.Errorf("Expected name 'test-component', got '%s'", comp.Manifest.Name)
	}

	if comp.Source != SourceLocal {
		t.Errorf("Expected source 'local', got '%s'", comp.Source)
	}

	if len(comp.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(comp.Templates))
	}

	if _, ok := comp.Templates["test.tmpl"]; !ok {
		t.Error("Expected template 'test.tmpl' to be loaded")
	}
}

func TestLoad_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "cached-comp")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: cached-comp
version: 1.0.0
description: Cached component
category: base
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// First load
	comp1, err := loader.Load("cached-comp")
	if err != nil {
		t.Fatalf("Failed to load component: %v", err)
	}

	// Second load should come from cache
	comp2, err := loader.Load("cached-comp")
	if err != nil {
		t.Fatalf("Failed to load cached component: %v", err)
	}

	// Should be the same pointer (from cache)
	if comp1 != comp2 {
		t.Error("Expected cached component to be same instance")
	}
}

func TestLoad_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "invalid-comp")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create invalid manifest (missing required fields)
	manifest := `name: invalid-comp
version: 1.0.0
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	_, err := loader.Load("invalid-comp")

	if err == nil {
		t.Error("Expected error for invalid manifest")
	}
}

func TestLoad_MissingTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "missing-template")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Manifest references a template that doesn't exist
	manifest := `name: missing-template
version: 1.0.0
description: Component with missing template
category: base
templates:
  - missing.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	_, err := loader.Load("missing-template")

	if err == nil {
		t.Error("Expected error for missing template file")
	}

	// Load() returns ErrComponentNotFound when it fails to load from any source
	if _, ok := err.(ErrComponentNotFound); !ok {
		t.Errorf("Expected ErrComponentNotFound (failed to load), got %T: %v", err, err)
	}
}

func TestLoad_NameMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "dir-name")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Name in manifest doesn't match directory name
	manifest := `name: wrong-name
version: 1.0.0
description: Test component
category: base
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	_, err := loader.Load("dir-name")

	if err == nil {
		t.Error("Expected error for name mismatch")
	}

	// Load() returns ErrComponentNotFound when it fails to load from any source
	if _, ok := err.(ErrComponentNotFound); !ok {
		t.Errorf("Expected ErrComponentNotFound (failed to load), got %T: %v", err, err)
	}
}

func TestList_FilterByCategory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create components with different categories
	categories := []struct {
		name string
		cat  string
	}{
		{"comp1", "base"},
		{"comp2", "layout"},
		{"comp3", "form"},
		{"comp4", "layout"},
	}

	for _, tc := range categories {
		componentDir := filepath.Join(tmpDir, tc.name)
		if err := os.MkdirAll(componentDir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := "name: " + tc.name + "\nversion: 1.0.0\ndescription: Test\ncategory: " + tc.cat + "\ntemplates:\n  - test.tmpl\n"
		if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
		if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Filter by layout category
	opts := &ComponentSearchOptions{
		Category: CategoryLayout,
	}

	list, err := loader.List(opts)
	if err != nil {
		t.Fatalf("Failed to list components: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 layout components, got %d", len(list))
	}

	for _, comp := range list {
		if comp.Manifest.Category != CategoryLayout {
			t.Errorf("Expected category 'layout', got '%s'", comp.Manifest.Category)
		}
	}
}

func TestList_FilterByQuery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create components with different names/descriptions/tags
	testCases := []struct {
		name        string
		description string
		tags        []string
	}{
		{"user-profile", "User profile component", []string{"user", "profile"}},
		{"product-card", "Product card component", []string{"product", "card"}},
		{"search-bar", "Search bar component", []string{"search"}},
	}

	for _, tc := range testCases {
		componentDir := filepath.Join(tmpDir, tc.name)
		if err := os.MkdirAll(componentDir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := "name: " + tc.name + "\nversion: 1.0.0\ndescription: " + tc.description + "\ncategory: base\ntemplates:\n  - test.tmpl\ntags:\n"
		for _, tag := range tc.tags {
			manifest += "  - " + tag + "\n"
		}

		if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
		if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	tests := []struct {
		query         string
		expectedCount int
		expectedName  string
	}{
		{"user", 1, "user-profile"},
		{"product", 1, "product-card"},
		{"search", 1, "search-bar"},
		{"card", 1, "product-card"},
		{"component", 3, ""},
		{"nonexistent", 0, ""},
	}

	for _, tt := range tests {
		t.Run("Query_"+tt.query, func(t *testing.T) {
			opts := &ComponentSearchOptions{
				Query: tt.query,
			}

			list, err := loader.List(opts)
			if err != nil {
				t.Fatalf("Failed to list components: %v", err)
			}

			if len(list) != tt.expectedCount {
				t.Errorf("Expected %d components matching '%s', got %d", tt.expectedCount, tt.query, len(list))
			}

			if tt.expectedName != "" && len(list) > 0 {
				if list[0].Manifest.Name != tt.expectedName {
					t.Errorf("Expected component '%s', got '%s'", tt.expectedName, list[0].Manifest.Name)
				}
			}
		})
	}
}

func TestList_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two search paths with same component name
	dir1 := filepath.Join(tmpDir, "path1", "test-comp")
	dir2 := filepath.Join(tmpDir, "path2", "test-comp")

	for _, dir := range []string{dir1, dir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := `name: test-comp
version: 1.0.0
description: Test component
category: base
templates:
  - test.tmpl
`
		if err := os.WriteFile(filepath.Join(dir, "component.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}

		template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
		if err := os.WriteFile(filepath.Join(dir, "test.tmpl"), []byte(template), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(filepath.Join(tmpDir, "path1"))
	loader.AddSearchPath(filepath.Join(tmpDir, "path2"))

	// List should not contain duplicates (first path wins)
	list, err := loader.List(nil)
	if err != nil {
		t.Fatalf("Failed to list components: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 component (no duplicates), got %d", len(list))
	}
}

func TestAddSearchPath(t *testing.T) {
	loader := NewLoader(nil)
	initialPaths := len(loader.GetSearchPaths())

	customPath := "/custom/path"
	loader.AddSearchPath(customPath)

	paths := loader.GetSearchPaths()
	if len(paths) != initialPaths+1 {
		t.Errorf("Expected %d search paths, got %d", initialPaths+1, len(paths))
	}

	found := false
	for _, p := range paths {
		if p == customPath {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected custom path to be in search paths")
	}
}

func TestAddSearchPath_ClearsCache(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "cached-comp")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: cached-comp
version: 1.0.0
description: Cached component
category: base
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load to populate cache
	_, err := loader.Load("cached-comp")
	if err != nil {
		t.Fatal(err)
	}

	if len(loader.cache) != 1 {
		t.Error("Expected cache to have 1 entry")
	}

	// Add search path should clear cache
	loader.AddSearchPath("/another/path")

	if len(loader.cache) != 0 {
		t.Error("Expected cache to be cleared after adding search path")
	}
}

func TestClearCache_EmptiesCache(t *testing.T) {
	tmpDir := t.TempDir()
	componentDir := filepath.Join(tmpDir, "test-comp")
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: test-comp
version: 1.0.0
description: Test component
category: base
templates:
  - test.tmpl
`
	if err := os.WriteFile(filepath.Join(componentDir, "component.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	template := `[[ define "test" ]]<div>Test</div>[[ end ]]`
	if err := os.WriteFile(filepath.Join(componentDir, "test.tmpl"), []byte(template), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load to populate cache
	_, err := loader.Load("test-comp")
	if err != nil {
		t.Fatal(err)
	}

	if len(loader.cache) != 1 {
		t.Error("Expected cache to have 1 entry")
	}

	// Clear cache
	loader.ClearCache()

	if len(loader.cache) != 0 {
		t.Error("Expected cache to be empty after ClearCache()")
	}
}

func TestGetSearchPaths_ReturnsCopy(t *testing.T) {
	loader := NewLoader(nil)
	loader.AddSearchPath("/path1")

	paths := loader.GetSearchPaths()
	originalLen := len(paths)

	// Modify the returned slice
	paths = append(paths, "/modified")

	// Original should be unchanged
	newPaths := loader.GetSearchPaths()
	if len(newPaths) != originalLen {
		t.Error("GetSearchPaths should return a copy, not original slice")
	}
}

func TestMatchesOptions_NilOptions(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:     "test",
			Category: CategoryBase,
		},
		Source: SourceLocal,
	}

	if !matchesOptions(comp, nil) {
		t.Error("Expected nil options to match any component")
	}
}

func TestMatchesOptions_EmptyOptions(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:     "test",
			Category: CategoryBase,
		},
		Source: SourceLocal,
	}

	opts := &ComponentSearchOptions{}

	if !matchesOptions(comp, opts) {
		t.Error("Expected empty options to match any component")
	}
}

func TestMatchesOptions_SourceFilter(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:     "test",
			Category: CategoryBase,
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		source  ComponentSource
		matches bool
	}{
		{"matching source", SourceLocal, true},
		{"non-matching source", SourceSystem, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ComponentSearchOptions{Source: tt.source}
			result := matchesOptions(comp, opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v", tt.matches, result)
			}
		})
	}
}

func TestMatchesOptions_CategoryFilter(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:     "test",
			Category: CategoryLayout,
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name     string
		category ComponentCategory
		matches  bool
	}{
		{"matching category", CategoryLayout, true},
		{"non-matching category", CategoryForm, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ComponentSearchOptions{Category: tt.category}
			result := matchesOptions(comp, opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v", tt.matches, result)
			}
		})
	}
}

func TestMatchesOptions_QueryFilter(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:        "user-profile",
			Description: "A component for displaying user information",
			Category:    CategoryLayout,
			Tags:        []string{"user", "profile", "display"},
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		query   string
		matches bool
	}{
		{"matches name", "user", true},
		{"matches description", "displaying", true},
		{"matches tag", "profile", true},
		{"no match", "product", false},
		{"case insensitive", "USER", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ComponentSearchOptions{Query: tt.query}
			result := matchesOptions(comp, opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v for query '%s'", tt.matches, result, tt.query)
			}
		})
	}
}

func TestMatchesOptions_MultipleFilters(t *testing.T) {
	comp := &Component{
		Manifest: ComponentManifest{
			Name:        "user-form",
			Description: "Form for user input",
			Category:    CategoryForm,
			Tags:        []string{"user", "input"},
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		opts    *ComponentSearchOptions
		matches bool
	}{
		{
			"all match",
			&ComponentSearchOptions{Source: SourceLocal, Category: CategoryForm, Query: "user"},
			true,
		},
		{
			"source fails",
			&ComponentSearchOptions{Source: SourceSystem, Category: CategoryForm, Query: "user"},
			false,
		},
		{
			"category fails",
			&ComponentSearchOptions{Source: SourceLocal, Category: CategoryLayout, Query: "user"},
			false,
		},
		{
			"query fails",
			&ComponentSearchOptions{Source: SourceLocal, Category: CategoryForm, Query: "product"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesOptions(comp, tt.opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v", tt.matches, result)
			}
		})
	}
}
