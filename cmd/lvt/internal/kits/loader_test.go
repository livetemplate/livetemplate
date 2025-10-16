package kits

import (
	"testing"
)

// TestLoadSystemKits tests loading all system kits from embedded FS
func TestLoadSystemKits(t *testing.T) {
	loader := DefaultLoader()

	// List of all system kits that should be available
	expectedKits := []string{
		"tailwind",
		"bulma",
		"pico",
		"none",
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

			// Verify helpers are loaded
			if kit.Helpers == nil {
				t.Errorf("Kit %q helpers are nil", name)
			}
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

	// We should have exactly 4 system kits
	if len(kits) != 4 {
		t.Errorf("Expected 4 system kits, got %d", len(kits))
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
			if kit.Manifest.Framework == "" {
				t.Error("Kit framework is empty")
			}
			if kit.Manifest.Author == "" {
				t.Error("Kit author is empty")
			}
			if kit.Manifest.License == "" {
				t.Error("Kit license is empty")
			}

			// CDN can be empty for "none" kit
			if kit.Manifest.Name != "none" && kit.Manifest.CDN == "" {
				t.Logf("Warning: Kit %q has no CSS CDN", kit.Manifest.Name)
			}
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

			if helpers == nil {
				t.Fatal("Helpers are nil")
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
	kit1, err := loader.Load("tailwind")
	if err != nil {
		t.Fatalf("Failed to load kit: %v", err)
	}

	// Load the same kit again
	kit2, err := loader.Load("tailwind")
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
	kit3, err := loader.Load("tailwind")
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
			// Verify framework field matches kit name
			if kit.Manifest.Framework != kit.Manifest.Name {
				t.Errorf("Framework mismatch: manifest.framework = %q, manifest.name = %q",
					kit.Manifest.Framework, kit.Manifest.Name)
			}
		})
	}
}

// TestKitCDN tests that CSS CDN URLs are properly configured
func TestKitCDN(t *testing.T) {
	loader := DefaultLoader()

	testCases := []struct {
		kitName   string
		expectCDN bool
	}{
		{
			kitName:   "tailwind",
			expectCDN: true,
		},
		{
			kitName:   "bulma",
			expectCDN: true,
		},
		{
			kitName:   "pico",
			expectCDN: true,
		},
		{
			kitName:   "none",
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
			helperCDN := kit.Helpers.CSSCDN()

			if tc.expectCDN {
				if cdn == "" {
					t.Errorf("Expected non-empty CDN for %q", tc.kitName)
				}
				// CSSCDN() should return HTML containing the CDN URL
				if helperCDN == "" {
					t.Errorf("Expected non-empty CSSCDN() for %q", tc.kitName)
				}
			} else {
				if cdn != "" {
					t.Errorf("Expected empty CDN for %q, got %q", tc.kitName, cdn)
				}
				// CSSCDN() should also be empty for none kit
				if helperCDN != "" {
					t.Errorf("Expected empty CSSCDN() for %q, got %q", tc.kitName, helperCDN)
				}
			}
		})
	}
}
