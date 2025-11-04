package build

import (
	"testing"
)

// TestNewKeyGenerator tests KeyGenerator constructor.
func TestNewKeyGenerator(t *testing.T) {
	kg := NewKeyGenerator()

	if kg == nil {
		t.Fatal("Expected non-nil KeyGenerator")
	}

	if kg.counter != 0 {
		t.Errorf("Expected counter=0, got: %d", kg.counter)
	}

	if kg.usedKeys == nil {
		t.Error("Expected non-nil usedKeys map")
	}

	if kg.fallbackKeys == nil {
		t.Error("Expected non-nil fallbackKeys slice")
	}
}

// TestKeyGenerator_NextKey tests sequential key generation.
func TestKeyGenerator_NextKey(t *testing.T) {
	kg := NewKeyGenerator()

	// Test sequential generation
	tests := []string{"1", "2", "3", "4", "5"}
	for i, expected := range tests {
		got := kg.NextKey()
		if got != expected {
			t.Errorf("NextKey() call %d: expected %q, got %q", i+1, expected, got)
		}
	}

	// Verify counter advanced
	if kg.counter != 5 {
		t.Errorf("Expected counter=5, got: %d", kg.counter)
	}
}

// TestKeyGenerator_Reset tests reset behavior.
func TestKeyGenerator_Reset(t *testing.T) {
	kg := NewKeyGenerator()

	// Generate some keys
	kg.NextKey()
	kg.NextKey()
	kg.NextKey()

	if kg.counter != 3 {
		t.Errorf("Before reset: expected counter=3, got: %d", kg.counter)
	}

	// Reset
	kg.Reset()

	if kg.counter != 0 {
		t.Errorf("After reset: expected counter=0, got: %d", kg.counter)
	}

	if len(kg.usedKeys) != 0 {
		t.Errorf("After reset: expected empty usedKeys, got: %v", kg.usedKeys)
	}

	if len(kg.fallbackKeys) != 0 {
		t.Errorf("After reset: expected empty fallbackKeys, got: %v", kg.fallbackKeys)
	}

	// Next key should start from 1 again
	if got := kg.NextKey(); got != "1" {
		t.Errorf("After reset, NextKey() expected '1', got: %q", got)
	}
}

// TestKeyGenerator_LoadExistingKeys tests loading keys from existing range data.
func TestKeyGenerator_LoadExistingKeys(t *testing.T) {
	kg := NewKeyGenerator()

	// Simulate old range data with keys
	oldData := []interface{}{
		map[string]interface{}{"0": "1", "1": "item1"},
		map[string]interface{}{"0": "3", "1": "item2"},
		map[string]interface{}{"0": "5", "1": "item3"},
	}

	kg.LoadExistingKeys(oldData)

	// Counter should be set to max key value (5)
	if kg.counter != 5 {
		t.Errorf("Expected counter=5 (max key), got: %d", kg.counter)
	}

	// UsedKeys should track all keys
	expectedUsedKeys := []string{"1", "3", "5"}
	for _, key := range expectedUsedKeys {
		if !kg.usedKeys[key] {
			t.Errorf("Expected key %q to be tracked in usedKeys", key)
		}
	}

	// Next key should be 6
	if got := kg.NextKey(); got != "6" {
		t.Errorf("After loading keys, NextKey() expected '6', got: %q", got)
	}
}

// TestKeyGenerator_LoadExistingKeys_NonNumeric tests loading non-numeric keys.
func TestKeyGenerator_LoadExistingKeys_NonNumeric(t *testing.T) {
	kg := NewKeyGenerator()

	// Mix of numeric and non-numeric keys
	oldData := []interface{}{
		map[string]interface{}{"0": "uuid-123", "1": "item1"},
		map[string]interface{}{"0": "2", "1": "item2"},
		map[string]interface{}{"0": "abc", "1": "item3"},
	}

	kg.LoadExistingKeys(oldData)

	// Counter should be 2 (only numeric key)
	if kg.counter != 2 {
		t.Errorf("Expected counter=2 (max numeric key), got: %d", kg.counter)
	}

	// All keys should be tracked
	if !kg.usedKeys["uuid-123"] {
		t.Error("Expected non-numeric key 'uuid-123' to be tracked")
	}
	if !kg.usedKeys["2"] {
		t.Error("Expected numeric key '2' to be tracked")
	}
	if !kg.usedKeys["abc"] {
		t.Error("Expected non-numeric key 'abc' to be tracked")
	}
}

// TestKeyGenerator_Uniqueness tests that generated keys are unique.
func TestKeyGenerator_Uniqueness(t *testing.T) {
	kg := NewKeyGenerator()

	// Generate many keys and check for duplicates
	generated := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		key := kg.NextKey()
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
func TestGenerateWrapperKey(t *testing.T) {
	kg := NewKeyGenerator()

	// First wrapper key should be "1"
	key1 := GenerateWrapperKey(kg)
	if key1 != "1" {
		t.Errorf("Expected first wrapper key '1', got: %q", key1)
	}

	// Second wrapper key should be "2"
	key2 := GenerateWrapperKey(kg)
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
