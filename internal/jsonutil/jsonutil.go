// Package jsonutil provides shared json-iterator configuration used across internal packages.
package jsonutil

import jsoniter "github.com/json-iterator/go"

// API is a json-iterator instance configured for encoding/json compatibility.
var API = jsoniter.ConfigCompatibleWithStandardLibrary

// NoEscape is a json-iterator instance with HTML escaping disabled.
// Use this for wire format transmission where HTML entities should be preserved as-is.
var NoEscape = jsoniter.Config{
	EscapeHTML:             false,
	SortMapKeys:            false,
	ValidateJsonRawMessage: false,
}.Froze()
