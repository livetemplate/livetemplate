package send

import (
	jsoniter "github.com/json-iterator/go"
)

// jsonAPI is the package-wide json-iterator instance, compatible with encoding/json.
var jsonAPI = jsoniter.ConfigCompatibleWithStandardLibrary

// jsonNoEscape is a json-iterator instance with HTML escaping disabled.
var jsonNoEscape = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            false,
	ValidateJsonRawMessage: false,
}.Froze()

// MarshalOrderedJSON marshals a tree or other data structure to JSON with no HTML escaping.
// This is used for wire format transmission where HTML entities should be preserved as-is.
func MarshalOrderedJSON(tree interface{}) ([]byte, error) {
	return jsonNoEscape.Marshal(tree)
}

// MarshalValue marshals a single value to JSON with no HTML escaping.
// This is an alias for MarshalOrderedJSON for backward compatibility.
func MarshalValue(value interface{}) ([]byte, error) {
	return MarshalOrderedJSON(value)
}
