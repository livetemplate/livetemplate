package build

import (
	"fmt"
	"strconv"
	"strings"
)

// KeyAttributeConfig defines which attributes to check for explicit keys.
type KeyAttributeConfig struct {
	AttributeNames []string
}

// DefaultKeyAttributes provides sensible defaults for key attribute names.
var DefaultKeyAttributes = KeyAttributeConfig{
	AttributeNames: []string{
		"key",
		"lvt-key",
		"data-key",
		"data-lvt-key",
		"data-id",
		"id",
		"x-key", // Alpine.js compatibility
		"v-key", // Vue.js compatibility
	},
}

// KeyGenerator provides counter-based key generation for wrapper approach.
type KeyGenerator struct {
	counter      int
	usedKeys     map[string]bool    // Track used keys to prevent duplicates
	fallbackKeys []string           // Position-based fallback keys
	keyConfig    KeyAttributeConfig // Configuration for key attribute names
}

// NewKeyGenerator creates a new key generator for a template instance.
func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{
		counter:      0,
		usedKeys:     make(map[string]bool),
		fallbackKeys: []string{},
		keyConfig:    DefaultKeyAttributes,
	}
}

// NextKey generates the next sequential key.
func (kg *KeyGenerator) NextKey() string {
	kg.counter++
	return fmt.Sprintf("%d", kg.counter)
}

// Reset resets the counter (useful for testing).
func (kg *KeyGenerator) Reset() {
	kg.counter = 0
	kg.usedKeys = make(map[string]bool)
	kg.fallbackKeys = []string{}
}

// LoadExistingKeys stores previous data and updates counter.
func (kg *KeyGenerator) LoadExistingKeys(oldRangeData []interface{}) {
	// Reset used keys tracking
	kg.usedKeys = make(map[string]bool)

	// Extract max key to update counter
	for _, item := range oldRangeData {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Track this key as used
			if keyValue, exists := itemMap["0"]; exists {
				if keyStr, ok := keyValue.(string); ok {
					kg.usedKeys[keyStr] = true

					// Update counter if it's a numeric key
					if keyInt, err := strconv.Atoi(keyStr); err == nil && keyInt > kg.counter {
						kg.counter = keyInt
					}
				}
			}
		}
	}
}

// GenerateWrapperKey generates a simple wrapper key using provided generator.
func GenerateWrapperKey(keyGen *KeyGenerator) string {
	return keyGen.NextKey()
}

// DetectIDKey detects which position in the dynamics contains the item ID
// by scanning the statics array for key attribute patterns.
// Returns the position as a string (e.g., "1" for the second dynamic position).
// Returns "0" as default if no key attribute is found.
func DetectIDKey(statics []string) string {
	if len(statics) == 0 {
		return "0"
	}

	// Key attributes to search for (in priority order)
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"",
		"v-key=\"",
	}

	// Scan through statics array
	for i, static := range statics {
		// Check if this static contains a key attribute
		for _, attr := range keyAttrs {
			if strings.Contains(static, attr) {
				// The dynamic value after this static is the ID
				// Position i in statics means dynamic at position i+1
				// But we need to return the dynamic index, which starts at 0
				// So dynamic position is i (0-indexed in the dynamics)
				return fmt.Sprintf("%d", i)
			}
		}
	}

	// Default to position 0 if no key attribute found
	return "0"
}
