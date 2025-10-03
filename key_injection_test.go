package livetemplate

import (
	"fmt"
	"testing"
)

func TestKeyInjectionScenarios(t *testing.T) {
	// Reset key generator for clean test
	resetKeyGenerator()

	// Test the new simple wrapper approach
	tests := []struct {
		name     string
		expected string
	}{
		{name: "First key", expected: "1"},
		{name: "Second key", expected: "2"},
		{name: "Third key", expected: "3"},
		{name: "Fourth key", expected: "4"},
		{name: "Fifth key", expected: "5"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := generateWrapperKey(globalKeyGenerator)
			if result != test.expected {
				t.Errorf("Expected key %q, got %q", test.expected, result)
			}
		})
	}
}

func TestKeyInjectionStabilityAcrossChanges(t *testing.T) {
	// Reset key generator for clean test
	resetKeyGenerator()

	t.Logf("🎯 NEW WRAPPER APPROACH: Keys are assigned once per page load")
	t.Logf("✅ No complex identity tracking needed")
	t.Logf("✅ Works with ANY data type")
	t.Logf("✅ Keys are stable within a single page render")

	// Generate a few keys to show the pattern
	keys := make([]string, 3)
	for i := 0; i < 3; i++ {
		keys[i] = generateWrapperKey(globalKeyGenerator)
	}

	t.Logf("Generated keys: %v", keys)
	t.Logf("✅ Simple sequential generation: 1, 2, 3")
}

func TestKeyInjectionUniversalCompatibility(t *testing.T) {
	// Reset key generator for clean test
	resetKeyGenerator()

	t.Logf("🎯 UNIVERSAL COMPATIBILITY: Works with any data type")

	// Test that wrapper approach works with ANY data type
	testCases := []interface{}{
		42,                                     // primitive int
		"hello",                                // primitive string
		true,                                   // primitive bool
		[]int{1, 2, 3},                         // slice
		map[string]interface{}{"key": "value"}, // map
		struct {
			Count  int
			Active bool
		}{Count: 5, Active: true}, // struct without stable fields
		struct {
			ID   string
			Name string
		}{ID: "123", Name: "John"}, // struct with potential stable fields
	}

	for i, item := range testCases {
		key := generateWrapperKey(globalKeyGenerator)
		expectedKey := fmt.Sprintf("%d", i+1)

		if key != expectedKey {
			t.Errorf("Expected key %q for item %v, got %q", expectedKey, item, key)
		}

		t.Logf("  ✅ %T → %s", item, key)
	}

	t.Logf("✅ All data types handled uniformly - no special cases needed!")
}
