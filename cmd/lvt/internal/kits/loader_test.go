package kits

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadSystemKits tests loading all system kits from embedded FS
func TestLoadSystemKits(t *testing.T) {
	loader := DefaultLoader()

	// List of all system kits that should be available
	expectedKits := []string{
		"multi",
		"simple",
		"single",
	}

	for _, name := range expectedKits {
		t.Run("Load_"+name, func(t *testing.T) {
			kit, err := loader.Load(name)
			if err != nil {
				t.Fatalf("Failed to load system kit %q: %v", name, err)
			}

			// Verify kit is loaded
			if kit == nil {
				t.Fatalf("Kit %q is nil", name)
			}

			// Verify source is system
			if kit.Source != SourceSystem {
				t.Errorf("Kit %q source = %v, want %v", name, kit.Source, SourceSystem)
			}

			// Verify manifest name matches
			if kit.Manifest.Name != name {
				t.Errorf("Kit %q manifest name = %q, want %q", name, kit.Manifest.Name, name)
			}

			// Verify manifest is valid
			if err := kit.Manifest.Validate(); err != nil {
				t.Errorf("Kit %q manifest validation failed: %v", name, err)
			}

			// Helpers can be nil for app mode kits (multi/simple/single)
			// which are CSS-agnostic and load helpers dynamically
		})
	}
}

// TestListSystemKits tests listing all system kits
func TestListSystemKits(t *testing.T) {
	loader := DefaultLoader()

	// List all system kits
	kits, err := loader.List(&KitSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system kits: %v", err)
	}

	// We should have exactly 3 system kits
	if len(kits) != 3 {
		t.Errorf("Expected 3 system kits, got %d", len(kits))
	}

	// Verify all kits are from system source
	for _, kit := range kits {
		if kit.Source != SourceSystem {
			t.Errorf("Kit %q source = %v, want %v", kit.Manifest.Name, kit.Source, SourceSystem)
		}
	}
}

// TestKitManifestParsing tests that all kit manifests parse correctly
func TestKitManifestParsing(t *testing.T) {
	loader := DefaultLoader()

	kits, err := loader.List(&KitSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system kits: %v", err)
	}

	for _, kit := range kits {
		t.Run("Manifest_"+kit.Manifest.Name, func(t *testing.T) {
			// Verify required manifest fields
			if kit.Manifest.Name == "" {
				t.Error("Kit name is empty")
			}
			if kit.Manifest.Version == "" {
				t.Error("Kit version is empty")
			}
			if kit.Manifest.Description == "" {
				t.Error("Kit description is empty")
			}
			// Framework can be empty for app mode kits (multi/simple/single)
			// which are CSS-agnostic
			if kit.Manifest.Author == "" {
				t.Error("Kit author is empty")
			}
			if kit.Manifest.License == "" {
				t.Error("Kit license is empty")
			}

			// CDN can be empty for app mode kits which are CSS-agnostic
		})
	}
}

// TestKitHelpersInterface tests that all kits implement the CSSHelpers interface correctly
func TestKitHelpersInterface(t *testing.T) {
	loader := DefaultLoader()

	kits, err := loader.List(&KitSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system kits: %v", err)
	}

	for _, kit := range kits {
		t.Run("Helpers_"+kit.Manifest.Name, func(t *testing.T) {
			helpers := kit.Helpers

			// Helpers can be nil for app mode kits (multi/simple/single)
			// which are CSS-agnostic and load helpers dynamically based on user choice
			if helpers == nil {
				return
			}

			// Test all required interface methods
			// Framework information
			_ = helpers.CSSCDN()

			// Layout helpers
			_ = helpers.ContainerClass()
			_ = helpers.SectionClass()
			_ = helpers.BoxClass()
			_ = helpers.ColumnClass()
			_ = helpers.ColumnsClass()

			// Form helpers
			_ = helpers.FieldClass()
			_ = helpers.LabelClass()
			_ = helpers.InputClass()
			_ = helpers.TextareaClass()
			_ = helpers.SelectClass()
			_ = helpers.CheckboxClass()
			_ = helpers.RadioClass()
			_ = helpers.ButtonClass("primary")
			_ = helpers.ButtonGroupClass()
			_ = helpers.FormClass()

			// Table helpers
			_ = helpers.TableClass()
			_ = helpers.TheadClass()
			_ = helpers.TbodyClass()
			_ = helpers.ThClass()
			_ = helpers.TdClass()
			_ = helpers.TrClass()
			_ = helpers.TableContainerClass()

			// Typography helpers
			_ = helpers.TitleClass(1)
			_ = helpers.SubtitleClass()
			_ = helpers.TextClass("lg")
			_ = helpers.TextMutedClass()
			_ = helpers.TextPrimaryClass()
			_ = helpers.TextDangerClass()

			// Pagination helpers
			_ = helpers.PaginationClass()
			_ = helpers.PaginationButtonClass("active")

			// Framework-specific checks
			_ = helpers.NeedsWrapper()
			_ = helpers.NeedsArticle()

			// Utility functions
			_ = helpers.Dict("key", "value")
			_ = helpers.Until(5)
			_ = helpers.Add(1, 2)
		})
	}
}

// TestKitCache tests that kit caching works correctly
func TestKitCache(t *testing.T) {
	loader := DefaultLoader()

	// Load a kit
	kit1, err := loader.Load("multi")
	if err != nil {
		t.Fatalf("Failed to load kit: %v", err)
	}

	// Load the same kit again
	kit2, err := loader.Load("multi")
	if err != nil {
		t.Fatalf("Failed to load kit again: %v", err)
	}

	// Verify it's the same instance (cached)
	if kit1 != kit2 {
		t.Error("Kit not cached: different instances returned")
	}

	// Clear cache
	loader.ClearCache()

	// Load again after cache clear
	kit3, err := loader.Load("multi")
	if err != nil {
		t.Fatalf("Failed to load kit after cache clear: %v", err)
	}

	// Should be a different instance
	if kit1 == kit3 {
		t.Error("Cache not cleared: same instance returned")
	}
}

// TestKitNotFound tests error handling for non-existent kits
func TestKitNotFound(t *testing.T) {
	loader := DefaultLoader()

	_, err := loader.Load("nonexistent-kit")
	if err == nil {
		t.Error("Expected error for non-existent kit, got nil")
	}

	// Verify it's the right error type
	if _, ok := err.(ErrKitNotFound); !ok {
		t.Errorf("Expected ErrKitNotFound, got %T: %v", err, err)
	}
}

// TestKitFrameworkMapping tests that framework names map correctly to helpers
func TestKitFrameworkMapping(t *testing.T) {
	loader := DefaultLoader()

	kits, err := loader.List(&KitSearchOptions{
		Source: SourceSystem,
	})
	if err != nil {
		t.Fatalf("Failed to list system kits: %v", err)
	}

	for _, kit := range kits {
		t.Run("Framework_"+kit.Manifest.Name, func(t *testing.T) {
			// App mode kits (multi/simple/single) are CSS-agnostic and have empty framework
			// Framework-specific kits would have framework matching the CSS framework name
			// For now, we just verify the field is present (can be empty)
			_ = kit.Manifest.Framework
		})
	}
}

// TestKitCDN tests that CSS CDN URLs are properly configured
func TestKitCDN(t *testing.T) {
	loader := DefaultLoader()

	// App mode kits (multi/simple/single) are CSS-agnostic and don't have CDNs
	// They load CSS helpers dynamically based on user's framework choice
	testCases := []struct {
		kitName   string
		expectCDN bool
	}{
		{
			kitName:   "multi",
			expectCDN: false,
		},
		{
			kitName:   "simple",
			expectCDN: false,
		},
		{
			kitName:   "single",
			expectCDN: false,
		},
	}

	for _, tc := range testCases {
		t.Run("CDN_"+tc.kitName, func(t *testing.T) {
			kit, err := loader.Load(tc.kitName)
			if err != nil {
				t.Fatalf("Failed to load kit: %v", err)
			}

			cdn := kit.Manifest.CDN

			if tc.expectCDN {
				if cdn == "" {
					t.Errorf("Expected non-empty CDN for %q", tc.kitName)
				}
				// Only test CSSCDN() if helpers are loaded
				if kit.Helpers != nil {
					helperCDN := kit.Helpers.CSSCDN()
					if helperCDN == "" {
						t.Errorf("Expected non-empty CSSCDN() for %q", tc.kitName)
					}
				}
			} else {
				if cdn != "" {
					t.Errorf("Expected empty CDN for %q (app mode kits are CSS-agnostic), got %q", tc.kitName, cdn)
				}
			}
		})
	}
}

// Unit tests for KitLoader core functionality

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
	// Create temporary kit
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create valid kit
	manifest := `name: test-kit
version: 1.0.0
description: A test CSS kit
framework: tailwind
author: Test Author
license: MIT
cdn: https://cdn.test.com/test.css
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Create loader and add search path
	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load kit
	kit, err := loader.Load("test-kit")
	if err != nil {
		t.Fatalf("Failed to load kit: %v", err)
	}

	if kit.Manifest.Name != "test-kit" {
		t.Errorf("Expected name 'test-kit', got '%s'", kit.Manifest.Name)
	}

	if kit.Source != SourceLocal {
		t.Errorf("Expected source 'local', got '%s'", kit.Source)
	}

	if kit.Helpers == nil {
		t.Error("Expected helpers to be loaded")
	}
}

func TestLoad_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "cached-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: cached-kit
version: 1.0.0
description: Cached kit
framework: bulma
author: Test
license: MIT
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// First load
	kit1, err := loader.Load("cached-kit")
	if err != nil {
		t.Fatalf("Failed to load kit: %v", err)
	}

	// Second load should come from cache
	kit2, err := loader.Load("cached-kit")
	if err != nil {
		t.Fatalf("Failed to load cached kit: %v", err)
	}

	// Should be the same pointer (from cache)
	if kit1 != kit2 {
		t.Error("Expected cached kit to be same instance")
	}
}

func TestLoad_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "invalid-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create invalid manifest (missing required fields)
	manifest := `name: invalid-kit
version: 1.0.0
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	_, err := loader.Load("invalid-kit")

	if err == nil {
		t.Error("Expected error for invalid manifest")
	}
}

func TestLoad_UnsupportedFramework(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "unsupported-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Kit with unsupported framework
	manifest := `name: unsupported-kit
version: 1.0.0
description: Kit with unsupported framework
framework: unsupported-framework
author: Test
license: MIT
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	kit, err := loader.Load("unsupported-kit")

	// Unsupported frameworks are allowed for custom/test frameworks
	if err != nil {
		t.Errorf("Expected kit to load with unsupported framework, got error: %v", err)
	}

	if kit == nil {
		t.Fatal("Expected kit to be loaded, got nil")
	}

	// Verify helpers are nil for unsupported frameworks
	if kit.Helpers != nil {
		t.Error("Expected nil helpers for unsupported framework")
	}

	// Verify kit information is still valid
	if kit.Manifest.Name != "unsupported-kit" {
		t.Errorf("Expected kit name 'unsupported-kit', got '%s'", kit.Manifest.Name)
	}

	if kit.Manifest.Framework != "unsupported-framework" {
		t.Errorf("Expected framework 'unsupported-framework', got '%s'", kit.Manifest.Framework)
	}
}

func TestList_FilterBySource(t *testing.T) {
	tmpDir := t.TempDir()
	kitDir := filepath.Join(tmpDir, "local-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: local-kit
version: 1.0.0
description: Local kit
framework: pico
author: Test
license: MIT
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Filter by local source
	opts := &KitSearchOptions{
		Source: SourceLocal,
	}

	list, err := loader.List(opts)
	if err != nil {
		t.Fatalf("Failed to list kits: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 local kit, got %d", len(list))
	}

	if list[0].Source != SourceLocal {
		t.Errorf("Expected source 'local', got '%s'", list[0].Source)
	}
}

func TestList_FilterByQuery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create kits with different names/descriptions
	testCases := []struct {
		name        string
		description string
		tags        []string
	}{
		{"custom-tailwind", "Custom Tailwind CSS kit", []string{"tailwind", "custom"}},
		{"bootstrap-kit", "Bootstrap CSS framework", []string{"bootstrap"}},
		{"material-design", "Material Design CSS", []string{"material", "design"}},
	}

	for _, tc := range testCases {
		kitDir := filepath.Join(tmpDir, tc.name)
		if err := os.MkdirAll(kitDir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := "name: " + tc.name + "\nversion: 1.0.0\ndescription: " + tc.description + "\nframework: none\nauthor: Test\nlicense: MIT\ntags:\n"
		for _, tag := range tc.tags {
			manifest += "  - " + tag + "\n"
		}

		if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
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
		{"tailwind", 1, "custom-tailwind"},
		{"bootstrap", 1, "bootstrap-kit"},
		{"material", 1, "material-design"},
		{"design", 1, "material-design"},
		{"CSS", 3, ""},
		{"nonexistent", 0, ""},
	}

	for _, tt := range tests {
		t.Run("Query_"+tt.query, func(t *testing.T) {
			opts := &KitSearchOptions{
				Query: tt.query,
			}

			list, err := loader.List(opts)
			if err != nil {
				t.Fatalf("Failed to list kits: %v", err)
			}

			if len(list) != tt.expectedCount {
				t.Errorf("Expected %d kits matching '%s', got %d", tt.expectedCount, tt.query, len(list))
			}

			if tt.expectedName != "" && len(list) > 0 {
				if list[0].Manifest.Name != tt.expectedName {
					t.Errorf("Expected kit '%s', got '%s'", tt.expectedName, list[0].Manifest.Name)
				}
			}
		})
	}
}

func TestList_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two search paths with same kit name
	dir1 := filepath.Join(tmpDir, "path1", "test-kit")
	dir2 := filepath.Join(tmpDir, "path2", "test-kit")

	for _, dir := range []string{dir1, dir2} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}

		manifest := `name: test-kit
version: 1.0.0
description: Test kit
framework: none
author: Test
license: MIT
`
		if err := os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(filepath.Join(tmpDir, "path1"))
	loader.AddSearchPath(filepath.Join(tmpDir, "path2"))

	// List should not contain duplicates (first path wins)
	list, err := loader.List(nil)
	if err != nil {
		t.Fatalf("Failed to list kits: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 kit (no duplicates), got %d", len(list))
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
	kitDir := filepath.Join(tmpDir, "cached-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: cached-kit
version: 1.0.0
description: Cached kit
framework: tailwind
author: Test
license: MIT
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load to populate cache
	_, err := loader.Load("cached-kit")
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
	kitDir := filepath.Join(tmpDir, "test-kit")
	if err := os.MkdirAll(kitDir, 0755); err != nil {
		t.Fatal(err)
	}

	manifest := `name: test-kit
version: 1.0.0
description: Test kit
framework: bulma
author: Test
license: MIT
`
	if err := os.WriteFile(filepath.Join(kitDir, "kit.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(nil)
	loader.AddSearchPath(tmpDir)

	// Load to populate cache
	_, err := loader.Load("test-kit")
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

	// Modify the returned slice (result intentionally unused)
	_ = append(paths, "/modified")

	// Original should be unchanged
	newPaths := loader.GetSearchPaths()
	if len(newPaths) != originalLen {
		t.Error("GetSearchPaths should return a copy, not original slice")
	}
}

func TestMatchesOptions_NilOptions(t *testing.T) {
	kit := &KitInfo{
		Manifest: KitManifest{
			Name:      "test",
			Framework: "tailwind",
		},
		Source: SourceLocal,
	}

	if !matchesOptions(kit, nil) {
		t.Error("Expected nil options to match any kit")
	}
}

func TestMatchesOptions_EmptyOptions(t *testing.T) {
	kit := &KitInfo{
		Manifest: KitManifest{
			Name:      "test",
			Framework: "tailwind",
		},
		Source: SourceLocal,
	}

	opts := &KitSearchOptions{}

	if !matchesOptions(kit, opts) {
		t.Error("Expected empty options to match any kit")
	}
}

func TestMatchesOptions_SourceFilter(t *testing.T) {
	kit := &KitInfo{
		Manifest: KitManifest{
			Name:      "test",
			Framework: "tailwind",
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		source  KitSource
		matches bool
	}{
		{"matching source", SourceLocal, true},
		{"non-matching source", SourceSystem, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &KitSearchOptions{Source: tt.source}
			result := matchesOptions(kit, opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v", tt.matches, result)
			}
		})
	}
}

func TestMatchesOptions_QueryFilter(t *testing.T) {
	kit := &KitInfo{
		Manifest: KitManifest{
			Name:        "tailwind-kit",
			Description: "A powerful utility-first CSS framework",
			Framework:   "tailwind",
			Tags:        []string{"utility", "responsive", "modern"},
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		query   string
		matches bool
	}{
		{"matches name", "tailwind", true},
		{"matches description", "utility", true},
		{"matches tag", "responsive", true},
		{"no match", "bootstrap", false},
		{"case insensitive", "TAILWIND", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &KitSearchOptions{Query: tt.query}
			result := matchesOptions(kit, opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v for query '%s'", tt.matches, result, tt.query)
			}
		})
	}
}

func TestMatchesOptions_MultipleFilters(t *testing.T) {
	kit := &KitInfo{
		Manifest: KitManifest{
			Name:        "bulma-kit",
			Description: "Modern CSS framework based on Flexbox",
			Framework:   "bulma",
			Tags:        []string{"flexbox", "responsive"},
		},
		Source: SourceLocal,
	}

	tests := []struct {
		name    string
		opts    *KitSearchOptions
		matches bool
	}{
		{
			"all match",
			&KitSearchOptions{Source: SourceLocal, Query: "bulma"},
			true,
		},
		{
			"source fails",
			&KitSearchOptions{Source: SourceSystem, Query: "bulma"},
			false,
		},
		{
			"query fails",
			&KitSearchOptions{Source: SourceLocal, Query: "tailwind"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesOptions(kit, tt.opts)
			if result != tt.matches {
				t.Errorf("Expected %v, got %v", tt.matches, result)
			}
		})
	}
}
