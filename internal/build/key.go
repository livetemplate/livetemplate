// Package build provides tree building and key generation for LiveTemplate.
//
// Key Generation Strategy:
// KeyGenerator uses sequential integers as keys, starting from 1.
// Keys are stable within a single render and can be reset between renders.
// LoadExistingKeys allows continuing from previous state for range updates.
//
// KeyGenerator is safe for concurrent use by multiple goroutines.
package build

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

// KeyGenerator provides counter-based key generation for wrapper approach.
// It is safe for concurrent use by multiple goroutines.
type KeyGenerator struct {
	mu       sync.Mutex
	counter  int
	usedKeys map[string]bool // Track used keys to prevent duplicates
}

// NewKeyGenerator creates a new key generator for a template instance.
func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{
		usedKeys: make(map[string]bool),
	}
}

// NextKey generates the next sequential key.
// It is safe to call from multiple goroutines.
func (kg *KeyGenerator) NextKey() string {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if kg.counter == math.MaxInt {
		panic("KeyGenerator: counter overflow - maximum keys generated")
	}

	kg.counter++
	return strconv.Itoa(kg.counter)
}

// Reset resets the counter and used keys tracking.
// It is safe to call from multiple goroutines.
func (kg *KeyGenerator) Reset() {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	kg.counter = 0
	clear(kg.usedKeys)
}

// LoadExistingKeys loads previous range data and updates the counter.
// It tracks used keys and sets counter to the maximum found key value.
// Accepts both map[string]interface{} and *TreeNode items for flexibility.
// Returns an error if the data structure is invalid.
// It is safe to call from multiple goroutines.
func (kg *KeyGenerator) LoadExistingKeys(oldRangeData []interface{}) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Reset used keys tracking
	clear(kg.usedKeys)

	// Extract max key to update counter
	for i, item := range oldRangeData {
		var keyStr string

		// Handle both map and TreeNode formats
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

		case *TreeNode:
			// For TreeNode, extract key from dynamics
			if v.Dynamics != nil {
				keyValue, exists := v.Dynamics[KeyDynamicPosition]
				if exists {
					if str, ok := keyValue.(string); ok {
						keyStr = str
					}
				}
			}
			// If no key found in TreeNode, skip it (it may be a different type of node)
			if keyStr == "" {
				continue
			}

		default:
			return fmt.Errorf("LoadExistingKeys: item %d is not a map or TreeNode, got %T", i, item)
		}

		kg.usedKeys[keyStr] = true

		// Update counter if it's a numeric key
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
// Searches for key attributes in priority order:
// id, data-key, key, data-lvt-key, lvt-key, data-id, x-key (Alpine.js), v-key (Vue.js)
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
		"x-key=\"",  // Alpine.js compatibility
		"v-key=\"",  // Vue.js compatibility
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
				return strconv.Itoa(i)
			}
		}
	}

	// Default to position 0 if no key attribute found
	return "0"
}
