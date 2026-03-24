// Package jsonutil provides shared json-iterator configuration used across internal packages.
//
// Note: json-iterator does not detect cyclic data structures (unlike encoding/json which
// returns an error). All types passed to Marshal must be acyclic. In practice this is
// guaranteed because TreeNode trees are built bottom-up and never contain cycles.
package jsonutil

import jsoniter "github.com/json-iterator/go"

// API is a json-iterator instance configured for encoding/json compatibility.
var API = jsoniter.ConfigCompatibleWithStandardLibrary

// NoEscape is a json-iterator instance with HTML escaping disabled and map keys sorted.
// Use this for wire format transmission where HTML entities should be preserved as-is.
// SortMapKeys must remain true — required by MarshalOrderedJSON's deterministic output contract.
var NoEscape = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()
