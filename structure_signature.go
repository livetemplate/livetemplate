package livetemplate

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

// StructureSignature uniquely identifies a template structure type.
// It is used to determine if the client has seen a particular structure before,
// which is critical for deciding whether to include statics in updates.
//
// The signature system ensures specification compliance by definitively answering:
// "Has the client seen THIS EXACT structure at THIS EXACT path?"
type StructureSignature string

// Signature constants define the types of structures that can appear in templates.
const (
	// SigEmpty represents a nil or empty string value
	SigEmpty StructureSignature = "empty"

	// SigScalar represents a simple value (string, number, bool)
	SigScalar StructureSignature = "scalar"

	// SigConditional represents a TreeNode with statics but no range (conditional block)
	SigConditional StructureSignature = "conditional"

	// SigRangeEmpty represents a range construct with zero items
	SigRangeEmpty StructureSignature = "range:empty"

	// SigRangeItems represents a range construct with items.
	// The signature includes a hash of the statics to ensure uniqueness.
	// Format: "range:items:<statics-hash>"
	// This allows detecting when range template structure changes.
)

// CalculateSignature computes a unique signature for any template value.
// The signature identifies the structure type and, for ranges, includes a hash
// of the statics to detect template changes.
//
// Signature determination:
//   - nil or "" → SigEmpty
//   - Non-TreeNode value → SigScalar
//   - TreeNode with range and no items → SigRangeEmpty
//   - TreeNode with range and items → SigRangeItems:<hash>
//   - TreeNode with statics but no range → SigConditional
//   - Otherwise → SigScalar
func CalculateSignature(value interface{}) StructureSignature {
	// Handle nil and empty values
	if value == nil {
		return SigEmpty
	}

	// Handle empty string
	if str, ok := value.(string); ok && str == "" {
		return SigEmpty
	}

	// Check if value is a TreeNode
	node, ok := value.(*TreeNode)
	if !ok {
		// Not a TreeNode - treat as scalar value
		return SigScalar
	}

	// Check if TreeNode is a range construct
	if node.HasRange() {
		if len(node.Range.Items) == 0 {
			// Empty range - client has never seen item templates
			return SigRangeEmpty
		}

		// Range with items - include hash of Range.Statics for uniqueness
		// This ensures we detect when the range item template structure changes
		staticsHash := hashStatics(node.Range.Statics)
		return StructureSignature(fmt.Sprintf("range:items:%s", staticsHash))
	}

	// Check if TreeNode has statics (conditional block)
	if node.HasStatics() {
		return SigConditional
	}

	// Default to scalar
	return SigScalar
}

// hashStatics creates a short hash of statics array for signature uniqueness.
// Uses first 8 bytes of MD5 hash for compact representation.
func hashStatics(statics []string) string {
	if len(statics) == 0 {
		return "none"
	}

	// Join statics with delimiter and hash
	data := strings.Join(statics, "|")
	hash := md5.Sum([]byte(data))

	// Return first 8 bytes as hex string (16 characters)
	return hex.EncodeToString(hash[:8])
}

// IsRange returns true if this signature represents a range construct.
func (s StructureSignature) IsRange() bool {
	return strings.HasPrefix(string(s), "range:")
}

// HasItems returns true if this signature represents a range with items.
func (s StructureSignature) HasItems() bool {
	return strings.HasPrefix(string(s), "range:items:")
}

// IsEmpty returns true if this signature represents an empty/nil value.
func (s StructureSignature) IsEmpty() bool {
	return s == SigEmpty
}

// IsScalar returns true if this signature represents a simple scalar value.
func (s StructureSignature) IsScalar() bool {
	return s == SigScalar
}

// IsConditional returns true if this signature represents a conditional block.
func (s StructureSignature) IsConditional() bool {
	return s == SigConditional
}

// String returns the string representation of the signature.
func (s StructureSignature) String() string {
	return string(s)
}
