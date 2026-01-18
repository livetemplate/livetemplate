package keys

import (
	"math"
	"strings"
	"testing"
)

// TestNewGenerator tests Generator constructor.
func TestNewGenerator(t *testing.T) {
	kg := NewGenerator()

	if kg == nil {
		t.Fatal("Expected non-nil Generator")
	}

	if kg.counter != 0 {
		t.Errorf("Expected counter=0, got: %d", kg.counter)
	}
}

// TestGenerator_NextKey tests sequential key generation.
func TestGenerator_NextKey(t *testing.T) {
	kg := NewGenerator()

	// Test sequential generation
	tests := []string{"1", "2", "3", "4", "5"}
	for i, expected := range tests {
		got, err := kg.NextKey()
		if err != nil {
			t.Fatalf("NextKey() call %d: unexpected error: %v", i+1, err)
		}
		if got != expected {
			t.Errorf("NextKey() call %d: expected %q, got %q", i+1, expected, got)
		}
	}

	// Verify counter advanced
	if kg.counter != 5 {
		t.Errorf("Expected counter=5, got: %d", kg.counter)
	}
}

// TestGenerator_Reset tests reset behavior.
func TestGenerator_Reset(t *testing.T) {
	kg := NewGenerator()

	// Generate some keys
	if _, err := kg.NextKey(); err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}
	if _, err := kg.NextKey(); err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}
	if _, err := kg.NextKey(); err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}

	if kg.counter != 3 {
		t.Errorf("Before reset: expected counter=3, got: %d", kg.counter)
	}

	// Reset
	kg.Reset()

	if kg.counter != 0 {
		t.Errorf("After reset: expected counter=0, got: %d", kg.counter)
	}

	// Next key should start from 1 again
	got, err := kg.NextKey()
	if err != nil {
		t.Fatalf("After reset, NextKey() error: %v", err)
	}
	if got != "1" {
		t.Errorf("After reset, NextKey() expected '1', got: %q", got)
	}
}

// TestGenerator_LoadExistingKeys tests loading keys from existing range data.
func TestGenerator_LoadExistingKeys(t *testing.T) {
	kg := NewGenerator()

	// Simulate old range data with keys
	oldData := []interface{}{
		map[string]interface{}{"0": "1", "1": "item1"},
		map[string]interface{}{"0": "3", "1": "item2"},
		map[string]interface{}{"0": "5", "1": "item3"},
	}

	err := kg.LoadExistingKeys(oldData)
	if err != nil {
		t.Fatalf("LoadExistingKeys failed: %v", err)
	}

	// Counter should be set to max key value (5)
	if kg.counter != 5 {
		t.Errorf("Expected counter=5 (max key), got: %d", kg.counter)
	}

	// Next key should be 6
	got, err := kg.NextKey()
	if err != nil {
		t.Fatalf("After loading keys, NextKey() error: %v", err)
	}
	if got != "6" {
		t.Errorf("After loading keys, NextKey() expected '6', got: %q", got)
	}
}

// TestGenerator_LoadExistingKeys_NonNumeric tests loading non-numeric keys.
func TestGenerator_LoadExistingKeys_NonNumeric(t *testing.T) {
	kg := NewGenerator()

	// Mix of numeric and non-numeric keys
	// Non-numeric keys (UUIDs, content hashes) are common in practice
	oldData := []interface{}{
		map[string]interface{}{"0": "uuid-123", "1": "item1"},
		map[string]interface{}{"0": "2", "1": "item2"},
		map[string]interface{}{"0": "abc", "1": "item3"},
	}

	err := kg.LoadExistingKeys(oldData)
	if err != nil {
		t.Fatalf("LoadExistingKeys failed: %v", err)
	}

	// Counter should be 2 (only numeric key)
	// Non-numeric keys don't affect the counter since they're from other sources
	// (content hashes, user-provided IDs, etc.)
	if kg.counter != 2 {
		t.Errorf("Expected counter=2 (max numeric key), got: %d", kg.counter)
	}

	// Next numeric key should be 3
	got, err := kg.NextKey()
	if err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}
	if got != "3" {
		t.Errorf("Expected next key to be '3', got: %q", got)
	}
}

// TestGenerator_Uniqueness tests that generated keys are unique.
func TestGenerator_Uniqueness(t *testing.T) {
	kg := NewGenerator()

	// Generate many keys and check for duplicates
	generated := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		key, err := kg.NextKey()
		if err != nil {
			t.Fatalf("NextKey() at iteration %d: %v", i, err)
		}
		if generated[key] {
			t.Fatalf("Duplicate key generated: %q at iteration %d", key, i)
		}
		generated[key] = true
	}

	if len(generated) != count {
		t.Errorf("Expected %d unique keys, got: %d", count, len(generated))
	}
}

// TestGenerateWrapperKey tests wrapper key generation.
// Note: GenerateWrapperKey was removed, now using NextKey directly.
func TestGenerateWrapperKey(t *testing.T) {
	kg := NewGenerator()

	// First wrapper key should be "1"
	key1, err := kg.NextKey()
	if err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}
	if key1 != "1" {
		t.Errorf("Expected first wrapper key '1', got: %q", key1)
	}

	// Second wrapper key should be "2"
	key2, err := kg.NextKey()
	if err != nil {
		t.Fatalf("NextKey() error: %v", err)
	}
	if key2 != "2" {
		t.Errorf("Expected second wrapper key '2', got: %q", key2)
	}

	// Keys should be different
	if key1 == key2 {
		t.Error("Wrapper keys should be unique")
	}
}

// TestDetectIDKey_AllPatterns tests ID key detection for all attribute patterns.
func TestDetectIDKey_AllPatterns(t *testing.T) {
	tests := []struct {
		name     string
		statics  []string
		expected string
	}{
		{"id attribute", []string{"<div id=\"", "\">test</div>"}, "0"},
		{"data-key attribute", []string{"<div data-key=\"", "\">test</div>"}, "0"},
		{"key attribute", []string{"<div key=\"", "\">test</div>"}, "0"},
		{"data-lvt-key attribute", []string{"<div data-lvt-key=\"", "\">test</div>"}, "0"},
		{"lvt-key attribute", []string{"<div lvt-key=\"", "\">test</div>"}, "0"},
		{"data-id attribute", []string{"<div data-id=\"", "\">test</div>"}, "0"},
		{"x-key attribute", []string{"<div x-key=\"", "\">test</div>"}, "0"},
		{"v-key attribute", []string{"<div v-key=\"", "\">test</div>"}, "0"},
		{"no key", []string{"<div>", "</div>"}, "0"},
		{"key at position 1", []string{"<div class=\"", "\" id=\"", "\">test</div>"}, "1"},
		{"key at position 2", []string{"<div class=\"", "\" name=\"", "\" key=\"", "\">test</div>"}, "2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectIDKey(tt.statics)
			if got != tt.expected {
				t.Errorf("DetectIDKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDetectIDKey_Priority tests priority order of key detection.
func TestDetectIDKey_Priority(t *testing.T) {
	// id= should take priority over data-key=
	statics := []string{"<div data-key=\"x\" id=\"", "\">test</div>"}
	got := DetectIDKey(statics)
	if got != "0" {
		t.Errorf("Expected '0' (id takes priority), got: %v", got)
	}

	// First occurrence takes priority
	statics2 := []string{"<div id=\"", "\" data-id=\"", "\">test</div>"}
	got2 := DetectIDKey(statics2)
	if got2 != "0" {
		t.Errorf("Expected '0' (first id), got: %v", got2)
	}
}

// TestDetectIDKey_NoKey tests default behavior with no key attribute.
func TestDetectIDKey_NoKey(t *testing.T) {
	statics := []string{"<div>", "<span>", "</span>", "</div>"}
	got := DetectIDKey(statics)
	if got != "0" {
		t.Errorf("Expected '0' (default), got: %v", got)
	}

	// Empty statics
	got2 := DetectIDKey([]string{})
	if got2 != "0" {
		t.Errorf("Expected '0' (default for empty), got: %v", got2)
	}
}

// TestNewGeneratorWithAttributes tests custom key attributes.
func TestNewGeneratorWithAttributes(t *testing.T) {
	customAttrs := []string{"custom-id=\"", "my-key=\""}
	kg := NewGenerator(customAttrs...)

	if kg == nil {
		t.Fatal("Expected non-nil Generator")
	}

	if len(kg.keyAttributes) != 2 {
		t.Errorf("Expected 2 custom attributes, got: %d", len(kg.keyAttributes))
	}
}

// TestDetectIDKeyWithAttributes_CustomAttributes tests custom attribute detection.
func TestDetectIDKeyWithAttributes_CustomAttributes(t *testing.T) {
	tests := []struct {
		name       string
		statics    []string
		attributes []string
		expected   string
	}{
		{
			name:       "custom attribute at position 0",
			statics:    []string{"<div custom-id=\"", "\">test</div>"},
			attributes: []string{"custom-id=\""},
			expected:   "0",
		},
		{
			name:       "custom attribute at position 1",
			statics:    []string{"<div class=\"", "\" my-key=\"", "\">test</div>"},
			attributes: []string{"my-key=\""},
			expected:   "1",
		},
		{
			name:       "fallback to default when custom not found",
			statics:    []string{"<div id=\"", "\">test</div>"},
			attributes: []string{"custom-id=\""},
			expected:   "0", // Falls back to position 0 since custom attr not found
		},
		{
			name:       "priority order matters",
			statics:    []string{"<div second=\"x\" first=\"", "\">test</div>"},
			attributes: []string{"first=\"", "second=\""},
			expected:   "0", // First attr wins even though second appears earlier in HTML
		},
		{
			name:       "empty attributes uses defaults",
			statics:    []string{"<div id=\"", "\">test</div>"},
			attributes: []string{},
			expected:   "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectIDKeyWithAttributes(tt.statics, tt.attributes...)
			if got != tt.expected {
				t.Errorf("DetectIDKeyWithAttributes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDefaultKeyAttributes tests that defaults are sensible.
func TestDefaultKeyAttributes(t *testing.T) {
	if len(DefaultKeyAttributes) == 0 {
		t.Error("DefaultKeyAttributes should not be empty")
	}

	// Check some expected defaults exist
	expectedDefaults := []string{"id=\"", "data-key=\"", "key=\""}
	for _, expected := range expectedDefaults {
		found := false
		for _, attr := range DefaultKeyAttributes {
			if attr == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected default attribute %q not found in DefaultKeyAttributes", expected)
		}
	}
}

// TestGenerator_Concurrent tests concurrent access to Generator.
func TestGenerator_Concurrent(t *testing.T) {
	kg := NewGenerator()
	numGoroutines := 100
	keysPerGoroutine := 100

	// Channel to collect all generated keys
	keysChan := make(chan string, numGoroutines*keysPerGoroutine)
	errChan := make(chan error, numGoroutines*keysPerGoroutine)

	// Launch goroutines to generate keys concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < keysPerGoroutine; j++ {
				key, err := kg.NextKey()
				if err != nil {
					errChan <- err
					return
				}
				keysChan <- key
			}
		}()
	}

	// Collect all keys
	keys := make(map[string]bool)
	for i := 0; i < numGoroutines*keysPerGoroutine; i++ {
		select {
		case key := <-keysChan:
			if keys[key] {
				t.Errorf("Duplicate key generated: %s", key)
			}
			keys[key] = true
		case err := <-errChan:
			t.Fatalf("NextKey() error in concurrent test: %v", err)
		}
	}

	// Verify we got the expected number of unique keys
	if len(keys) != numGoroutines*keysPerGoroutine {
		t.Errorf("Expected %d unique keys, got %d", numGoroutines*keysPerGoroutine, len(keys))
	}
}

// TestLoadExistingKeys_InvalidData tests error handling for invalid data.
func TestLoadExistingKeys_InvalidData(t *testing.T) {
	tests := []struct {
		name     string
		data     []interface{}
		wantErr  bool
		errMatch string
	}{
		{
			name:     "not a map",
			data:     []interface{}{"not a map"},
			wantErr:  true,
			errMatch: "is not a map",
		},
		{
			name:     "missing key position",
			data:     []interface{}{map[string]interface{}{"1": "value"}},
			wantErr:  true,
			errMatch: "missing key at position",
		},
		{
			name:     "key not a string",
			data:     []interface{}{map[string]interface{}{"0": 123}},
			wantErr:  true,
			errMatch: "is not a string",
		},
		{
			name:    "valid data",
			data:    []interface{}{map[string]interface{}{"0": "1", "1": "item"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kg := NewGenerator()
			err := kg.LoadExistingKeys(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errMatch)
				} else if !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("Expected error containing %q, got: %v", tt.errMatch, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestGenerator_OverflowProtection tests overflow protection.
func TestGenerator_OverflowProtection(t *testing.T) {
	kg := NewGenerator()
	kg.counter = math.MaxInt - 1

	// This should work
	key1, err := kg.NextKey()
	if err != nil {
		t.Errorf("Expected no error before overflow, got: %v", err)
	}
	if key1 == "" {
		t.Error("Expected valid key before overflow")
	}

	// This should return an error
	_, err = kg.NextKey()
	if err == nil {
		t.Error("Expected error on counter overflow")
	} else {
		if !strings.Contains(err.Error(), "overflow") {
			t.Errorf("Expected error message containing 'overflow', got: %v", err)
		}
	}
}
