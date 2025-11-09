package send

import (
	"bytes"
	"encoding/json"
)

// MarshalOrderedJSON marshals a tree or other data structure to JSON with no HTML escaping.
// This is used for wire format transmission where HTML entities should be preserved as-is.
// The encoder removes the trailing newline that json.Encoder.Encode() adds.
func MarshalOrderedJSON(tree interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(tree)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}

// MarshalValue marshals a single value to JSON with no HTML escaping.
// This is a specialized version of MarshalOrderedJSON for individual values.
// The encoder removes the trailing newline that json.Encoder.Encode() adds.
func MarshalValue(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(value)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}
