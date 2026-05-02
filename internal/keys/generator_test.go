package keys

import (
	"testing"
)

// TestDetectIDKey_AllPatterns tests detection of all default key attributes.
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

	// Empty-statics branch: distinct from "non-empty statics with no key
	// attribute" — exercises the early-return at the top of
	// detectIDKeyInternal.
	gotEmpty := DetectIDKey([]string{})
	if gotEmpty != "0" {
		t.Errorf("Expected '0' (default for empty statics), got: %v", gotEmpty)
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
