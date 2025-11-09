package signature_test

import (
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/signature"
)

func TestCalculateSignature(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected signature.StructureSignature
	}{
		{
			name:     "nil value",
			value:    nil,
			expected: signature.SigEmpty,
		},
		{
			name:     "empty string",
			value:    "",
			expected: signature.SigEmpty,
		},
		{
			name:     "non-empty string",
			value:    "hello",
			expected: signature.SigScalar,
		},
		{
			name:     "integer value",
			value:    42,
			expected: signature.SigScalar,
		},
		{
			name:     "boolean value",
			value:    true,
			expected: signature.SigScalar,
		},
		{
			name: "conditional TreeNode (has statics, no range)",
			value: &build.TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: make(map[string]interface{}),
			},
			expected: signature.SigConditional,
		},
		{
			name: "empty range",
			value: &build.TreeNode{
				Statics:  []string{"<tr>", "</tr>"},
				Dynamics: make(map[string]interface{}),
				Range: &build.RangeData{
					Items:   []interface{}{},
					Statics: []string{"<td>", "</td>"},
				},
			},
			expected: signature.SigRangeEmpty,
		},
		{
			name: "range with items",
			value: &build.TreeNode{
				Statics:  []string{"<tr>", "</tr>"},
				Dynamics: make(map[string]interface{}),
				Range: &build.RangeData{
					Items: []interface{}{
						map[string]interface{}{"0": "item1"},
					},
					Statics: []string{"<td>", "</td>"},
				},
			},
			expected: "range:items:", // Will have hash suffix
		},
		{
			name: "TreeNode without statics or range",
			value: &build.TreeNode{
				Dynamics: map[string]interface{}{"0": "value"},
			},
			expected: signature.SigScalar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := signature.CalculateSignature(tt.value)

			// For range:items, just check prefix since hash varies
			if strings.HasPrefix(string(tt.expected), "range:items:") {
				if !strings.HasPrefix(string(sig), "range:items:") {
					t.Errorf("Expected signature starting with 'range:items:', got %s", sig)
				}
			} else {
				if sig != tt.expected {
					t.Errorf("Expected signature %s, got %s", tt.expected, sig)
				}
			}
		})
	}
}

func TestCalculateSignature_RangeStaticsUniqueness(t *testing.T) {
	// Two ranges with different statics should have different signatures
	range1 := &build.TreeNode{
		Statics: []string{"<tr>", "</tr>"},
		Range: &build.RangeData{
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
			Statics: []string{"<td>", "</td>"},
		},
	}

	range2 := &build.TreeNode{
		Statics: []string{"<tr>", "</tr>"},
		Range: &build.RangeData{
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
			Statics: []string{"<th>", "</th>"}, // Different statics
		},
	}

	sig1 := signature.CalculateSignature(range1)
	sig2 := signature.CalculateSignature(range2)

	if sig1 == sig2 {
		t.Error("Different range statics should produce different signatures")
	}

	if !sig1.HasItems() || !sig2.HasItems() {
		t.Error("Range signatures should have items")
	}
}

func TestCalculateSignature_RangeStaticsSame(t *testing.T) {
	// Two ranges with same statics should have same signature
	range1 := &build.TreeNode{
		Statics: []string{"<tr>", "</tr>"},
		Range: &build.RangeData{
			Items:   []interface{}{map[string]interface{}{"0": "item1"}},
			Statics: []string{"<td>", "</td>"},
		},
	}

	range2 := &build.TreeNode{
		Statics: []string{"<tr>", "</tr>"},
		Range: &build.RangeData{
			Items:   []interface{}{map[string]interface{}{"0": "item2"}}, // Different data
			Statics: []string{"<td>", "</td>"},                           // Same statics
		},
	}

	sig1 := signature.CalculateSignature(range1)
	sig2 := signature.CalculateSignature(range2)

	if sig1 != sig2 {
		t.Errorf("Same range statics should produce same signature, got %s and %s", sig1, sig2)
	}
}

func TestCalculateSignature_Deterministic(t *testing.T) {
	// Same value should always produce same signature
	value := &build.TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range: &build.RangeData{
			Items:   []interface{}{map[string]interface{}{"0": "test"}},
			Statics: []string{"<span>", "</span>"},
		},
	}

	sig1 := signature.CalculateSignature(value)
	sig2 := signature.CalculateSignature(value)
	sig3 := signature.CalculateSignature(value)

	if sig1 != sig2 || sig2 != sig3 {
		t.Errorf("Signature calculation should be deterministic, got %s, %s, %s", sig1, sig2, sig3)
	}
}

func TestStructureSignature_IsRange(t *testing.T) {
	tests := []struct {
		sig      signature.StructureSignature
		expected bool
	}{
		{signature.SigEmpty, false},
		{signature.SigScalar, false},
		{signature.SigConditional, false},
		{signature.SigRangeEmpty, true},
		{signature.StructureSignature("range:items:abc123"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.sig), func(t *testing.T) {
			if got := tt.sig.IsRange(); got != tt.expected {
				t.Errorf("IsRange() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStructureSignature_HasItems(t *testing.T) {
	tests := []struct {
		sig      signature.StructureSignature
		expected bool
	}{
		{signature.SigEmpty, false},
		{signature.SigScalar, false},
		{signature.SigConditional, false},
		{signature.SigRangeEmpty, false},
		{signature.StructureSignature("range:items:abc123"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.sig), func(t *testing.T) {
			if got := tt.sig.HasItems(); got != tt.expected {
				t.Errorf("HasItems() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStructureSignature_IsEmpty(t *testing.T) {
	tests := []struct {
		sig      signature.StructureSignature
		expected bool
	}{
		{signature.SigEmpty, true},
		{signature.SigScalar, false},
		{signature.SigConditional, false},
		{signature.SigRangeEmpty, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.sig), func(t *testing.T) {
			if got := tt.sig.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStructureSignature_IsScalar(t *testing.T) {
	tests := []struct {
		sig      signature.StructureSignature
		expected bool
	}{
		{signature.SigEmpty, false},
		{signature.SigScalar, true},
		{signature.SigConditional, false},
		{signature.SigRangeEmpty, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.sig), func(t *testing.T) {
			if got := tt.sig.IsScalar(); got != tt.expected {
				t.Errorf("IsScalar() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStructureSignature_IsConditional(t *testing.T) {
	tests := []struct {
		sig      signature.StructureSignature
		expected bool
	}{
		{signature.SigEmpty, false},
		{signature.SigScalar, false},
		{signature.SigConditional, true},
		{signature.SigRangeEmpty, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.sig), func(t *testing.T) {
			if got := tt.sig.IsConditional(); got != tt.expected {
				t.Errorf("IsConditional() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestHashStatics(t *testing.T) {
	tests := []struct {
		name     string
		statics  []string
		expected string
	}{
		{
			name:     "empty statics",
			statics:  []string{},
			expected: "none",
		},
		{
			name:     "single static",
			statics:  []string{"<div>"},
			expected: "", // Will have hash
		},
		{
			name:     "multiple statics",
			statics:  []string{"<tr>", "<td>", "</td>", "</tr>"},
			expected: "", // Will have hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := signature.HashStatics(tt.statics)

			if tt.expected == "none" {
				if hash != "none" {
					t.Errorf("Empty statics should return 'none', got %s", hash)
				}
			} else {
				// Should be 16 character hex string
				if len(hash) != 16 {
					t.Errorf("Hash should be 16 characters, got %d: %s", len(hash), hash)
				}
			}
		})
	}
}

func TestHashStatics_Deterministic(t *testing.T) {
	statics := []string{"<tr>", "<td>", "</td>", "</tr>"}

	hash1 := signature.HashStatics(statics)
	hash2 := signature.HashStatics(statics)
	hash3 := signature.HashStatics(statics)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash should be deterministic, got %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestHashStatics_Uniqueness(t *testing.T) {
	statics1 := []string{"<tr>", "<td>", "</td>", "</tr>"}
	statics2 := []string{"<tr>", "<th>", "</th>", "</tr>"}

	hash1 := signature.HashStatics(statics1)
	hash2 := signature.HashStatics(statics2)

	if hash1 == hash2 {
		t.Error("Different statics should produce different hashes")
	}
}
