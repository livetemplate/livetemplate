// Package keys provides key-attribute detection for LiveTemplate range items.
//
// Auto-keys for range items are content hashes computed from item dynamics
// (see hash.go); explicit keys come from `data-key`, `id`, etc. attributes
// in the range body's static HTML — DetectIDKey scans the statics array to
// find which dynamic position holds the key attribute's value.
package keys

import (
	"strconv"
	"strings"
)

// DefaultKeyAttributes are the default HTML attributes searched for item keys.
// Searched in priority order — first match wins.
var DefaultKeyAttributes = []string{
	"id=\"",
	"data-key=\"",
	"key=\"",
	"data-lvt-key=\"",
	"lvt-key=\"",
	"data-id=\"",
	"x-key=\"", // Alpine.js compatibility
	"v-key=\"", // Vue.js compatibility
}

// DetectIDKey detects which position in the dynamics contains the item ID
// by scanning the statics array for key attribute patterns.
// Returns the position as a string-formatted index (e.g., "0", "1", "2")
// matching the key format used in dynamics maps.
// Returns "0" as default if no key attribute is found.
//
// Uses DefaultKeyAttributes for key detection.
// For custom attributes, use DetectIDKeyWithAttributes.
func DetectIDKey(statics []string) string {
	return detectIDKeyInternal(statics, nil)
}

// DetectIDKeyWithAttributes detects which position contains the item ID using custom attributes.
// Searches for key attributes in the order provided — first match wins.
// Returns the position as a string-formatted index (e.g., "0", "1", "2")
// matching the key format used in dynamics maps.
// Returns "0" as default if no key attribute is found.
func DetectIDKeyWithAttributes(statics []string, keyAttributes ...string) string {
	return detectIDKeyInternal(statics, keyAttributes)
}

// detectIDKeyInternal is the internal implementation for ID key detection.
// If keyAttributes is nil or empty, uses DefaultKeyAttributes.
func detectIDKeyInternal(statics []string, keyAttributes []string) string {
	if len(statics) == 0 {
		return "0"
	}

	// Use provided attributes or fall back to defaults
	attrs := keyAttributes
	if len(attrs) == 0 {
		attrs = DefaultKeyAttributes
	}

	// Scan through statics array
	for i, staticHTML := range statics {
		// Check if this static contains a key attribute
		for _, keyAttr := range attrs {
			if strings.Contains(staticHTML, keyAttr) {
				// The dynamic value after this static is the ID
				// Position i in statics means dynamic at position i
				// Return the dynamic index (0-indexed)
				return strconv.Itoa(i)
			}
		}
	}

	// Default to position 0 if no key attribute found
	return "0"
}
