package keys

import (
	"fmt"

	"github.com/livetemplate/livetemplate/internal/build"
)

// LoadExistingKeyMappings loads existing key mappings from the last tree node
// to ensure stable range item IDs across renders.
//
// This function:
// 1. Traverses the tree looking for range data
// 2. Loads existing key mappings from range items
// 3. Ensures continuity of item IDs when rendering updates
//
// Parameters:
//   - gen: The key generator to load mappings into
//   - lastTree: The previous tree node containing range data
//
// Returns:
//   - Error if key loading fails
func LoadExistingKeyMappings(gen *Generator, lastTree *build.TreeNode) error {
	if lastTree == nil {
		return nil
	}

	// Look for range data in the tree dynamics and load existing key mappings
	for _, value := range lastTree.Dynamics {
		// Check if this is a TreeNode with Range data
		if node, ok := value.(*build.TreeNode); ok {
			if node.HasRange() && node.Range != nil {
				if node.Range.IsStreamMode() {
					if err := gen.LoadExistingKeysFromSlice(node.Range.StreamState.Keys); err != nil {
						return fmt.Errorf("loadExistingKeyMappings (stream): %w", err)
					}
				} else if err := gen.LoadExistingKeys(node.Range.Items); err != nil {
					return fmt.Errorf("loadExistingKeyMappings: %w", err)
				}
			}
		}
	}
	return nil
}
