// Package keys provides key generation for LiveTemplate.
//
// Key Generation Strategy:
// Generator uses sequential integers as keys, starting from 1.
// Keys are stable within a single render and can be reset between renders.
// LoadExistingKeys allows continuing from previous state for range updates.
//
// Generator is safe for concurrent use by multiple goroutines.
package keys

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

const (
	// KeyDynamicPosition is the map key where item IDs are stored
	// in the dynamics structure of range item maps.
	KeyDynamicPosition = "0"
)

// DefaultKeyAttributes are the default HTML attributes searched for item keys.
// Searched in priority order - first match wins.
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

// Generator provides counter-based key generation for wrapper approach.
// It is safe for concurrent use by multiple goroutines.
type Generator struct {
	mu            sync.Mutex
	counter       int
	keyAttributes []string // Custom key attributes (nil uses DefaultKeyAttributes)
}

// NewGenerator creates a new key generator for a template instance.
// Optionally accepts custom key attributes - if none provided, uses DefaultKeyAttributes.
// The attributes are searched in the order provided - first match wins.
// Each attribute should include the = and opening quote, e.g., "data-id=\""
func NewGenerator(keyAttributes ...string) *Generator {
	if len(keyAttributes) == 0 {
		return &Generator{}
	}
	return &Generator{
		keyAttributes: keyAttributes,
	}
}

// NextKey generates the next sequential key.
// Returns an error only if counter overflow would occur (extremely unlikely in practice).
// It is safe to call from multiple goroutines.
func (kg *Generator) NextKey() (string, error) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if kg.counter == math.MaxInt {
		return "", fmt.Errorf("key generator: counter overflow - maximum keys generated")
	}

	kg.counter++
	return strconv.Itoa(kg.counter), nil
}

// Reset resets the counter to zero.
// It is safe to call from multiple goroutines.
func (kg *Generator) Reset() {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	kg.counter = 0
}

// DynamicsGetter is an interface for types that have a Dynamics field.
// This allows LoadExistingKeys to work with TreeNode without importing it.
type DynamicsGetter interface {
	GetDynamics() map[string]interface{}
}

// LoadExistingKeys loads previous range data and updates the counter.
// Sets counter to the maximum numeric key value found in the range data.
// Accepts items that are either:
//   - map[string]interface{} (JSON serializable format)
//   - Any type implementing DynamicsGetter (e.g., TreeNode)
//
// Non-numeric keys (UUIDs, content hashes, custom keys) are tracked but don't affect the counter.
// Returns an error if the data structure is invalid.
// It is safe to call from multiple goroutines.
func (kg *Generator) LoadExistingKeys(oldRangeData []interface{}) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Find the maximum numeric key to update counter
	for i, item := range oldRangeData {
		var keyStr string

		// Handle both map and DynamicsGetter formats
		switch v := item.(type) {
		case map[string]interface{}:
			keyValue, exists := v[KeyDynamicPosition]
			if !exists {
				return fmt.Errorf("LoadExistingKeys: item %d missing key at position %q", i, KeyDynamicPosition)
			}

			var ok bool
			keyStr, ok = keyValue.(string)
			if !ok {
				return fmt.Errorf("LoadExistingKeys: item %d key at position %q is not a string, got %T", i, KeyDynamicPosition, keyValue)
			}

		case DynamicsGetter:
			// For types with GetDynamics() method (e.g., TreeNode)
			// Items without keys or dynamics are silently skipped (not an error)
			// since they may be structural nodes not representing range items
			dynamics := v.GetDynamics()
			if dynamics != nil {
				keyValue, exists := dynamics[KeyDynamicPosition]
				if exists {
					if str, ok := keyValue.(string); ok {
						keyStr = str
					}
				}
			}
			// Skip items without keys - they may be structural nodes
			if keyStr == "" {
				continue
			}

		default:
			return fmt.Errorf("LoadExistingKeys: item %d is not a map or DynamicsGetter, got %T", i, item)
		}

		// Update counter only for numeric keys (from NextKey())
		// Non-numeric keys (UUIDs, hashes, custom keys) are valid but don't affect counter
		if keyInt, err := strconv.Atoi(keyStr); err == nil && keyInt > kg.counter {
			kg.counter = keyInt
		}
	}

	return nil
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
// Searches for key attributes in the order provided - first match wins.
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
