package send

import "github.com/livetemplate/livetemplate/internal/jsonutil"

// MarshalOrderedJSON marshals a tree or other data structure to JSON with no HTML escaping.
// This is used for wire format transmission where HTML entities should be preserved as-is.
func MarshalOrderedJSON(tree interface{}) ([]byte, error) {
	return jsonutil.NoEscape.Marshal(tree)
}

// MarshalValue marshals a single value to JSON with no HTML escaping.
// Deprecated: Use MarshalOrderedJSON directly.
func MarshalValue(value interface{}) ([]byte, error) {
	return MarshalOrderedJSON(value)
}
