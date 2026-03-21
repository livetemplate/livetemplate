package invariants

import (
	"encoding/json"
	"fmt"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/diff"
	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
)

// Violation records a failed invariant check with context for debugging.
type Violation struct {
	Invariant   string               // Name of the violated invariant
	Description string               // Human-readable description
	OldState    any                  // State before mutation
	NewState    any                  // State after mutation
	OldTree     *build.TreeNode      // Tree before render
	NewTree     *build.TreeNode      // Tree after render
	Diff        *build.TreeNode      // Computed diff
	Mutations   []mutations.Mutation // Mutation sequence that led to failure
	Seed        int64                // Random seed for reproduction
}

func (v *Violation) Error() string {
	return fmt.Sprintf("Invariant %q violated: %s (seed=%d, mutations=%d)",
		v.Invariant, v.Description, v.Seed, len(v.Mutations))
}

// Verifier checks LiveTemplate's core invariants.
//
// Note: Diff correctness is validated by TypeScript oracle tests (fuzz_ts_oracle_test.go).
// This verifier focuses on structural invariants that don't require diff application.
type Verifier struct {
	seed       int64
	mutations  []mutations.Mutation
	violations []Violation
}

// NewVerifier creates a new invariant verifier.
func NewVerifier(seed int64) *Verifier {
	return &Verifier{seed: seed}
}

// RecordMutation tracks a mutation for debugging context.
func (v *Verifier) RecordMutation(m mutations.Mutation) {
	v.mutations = append(v.mutations, m)
}

// ClearMutations resets the mutation history.
func (v *Verifier) ClearMutations() {
	v.mutations = nil
}

// GetViolations returns all recorded violations.
func (v *Verifier) GetViolations() []Violation {
	return v.violations
}

// VerifyAll checks all P0 invariants for a render transition.
//
// Parameters:
//   - oldState: State before mutation (nil for first render)
//   - newState: State after mutation
//   - oldTree: Tree from previous render (nil for first render)
//   - newTree: Tree from current render
//   - diffTree: Computed diff between oldTree and newTree
//   - isFirstRender: True if this is the first render
//
// Returns an error if any invariant is violated.
func (v *Verifier) VerifyAll(
	oldState, newState any,
	oldTree, newTree *build.TreeNode,
	diffTree *build.TreeNode,
	isFirstRender bool,
) error {
	// NOTE: Diff correctness is validated by TypeScript oracle tests (fuzz_ts_oracle_test.go).
	// The TypeScript oracle uses the production client code as the source of truth.

	// P0: Update Minimality
	if err := v.VerifyUpdateMinimality(diffTree, isFirstRender); err != nil {
		return err
	}

	// P0: Key Stability
	if err := v.VerifyKeyStability(oldTree, newTree); err != nil {
		return err
	}

	// P1: Tree Structure
	if err := v.VerifyTreeStructure(newTree); err != nil {
		return err
	}

	// P1: No Data Loss (JSON roundtrip)
	if err := v.VerifyNoDataLoss(newState); err != nil {
		return err
	}

	return nil
}

// VerifyUpdateMinimality checks that updates contain only changed dynamics, not statics.
//
// Invariant:
//   - First render: MUST include statics (client needs them)
//   - Subsequent updates: MUST NOT include statics (client has them cached)
func (v *Verifier) VerifyUpdateMinimality(diffTree *build.TreeNode, isFirstRender bool) error {
	if isFirstRender {
		// First render must have statics
		if !treeHasStatics(diffTree) && !isEmptyTree(diffTree) {
			violation := &Violation{
				Invariant:   "UpdateMinimality",
				Description: "First render missing statics",
				Diff:        diffTree,
				Mutations:   copyMutations(v.mutations),
				Seed:        v.seed,
			}
			v.violations = append(v.violations, *violation)
			return violation
		}
		return nil
	}

	// Subsequent updates must NOT have statics (client cached them)
	if hasUnexpectedStatics(diffTree) {
		violation := &Violation{
			Invariant:   "UpdateMinimality",
			Description: "Update includes unchanged statics (should be stripped)",
			Diff:        diffTree,
			Mutations:   copyMutations(v.mutations),
			Seed:        v.seed,
		}
		v.violations = append(v.violations, *violation)
		return violation
	}

	return nil
}

// VerifyKeyStability checks that the same list item keeps the same key across renders.
//
// Invariant: If an item's ID is unchanged, its key must be unchanged.
func (v *Verifier) VerifyKeyStability(oldTree, newTree *build.TreeNode) error {
	if oldTree == nil || newTree == nil {
		return nil
	}

	// Extract all ranges from both trees
	oldRanges := extractAllRanges(oldTree, "")
	newRanges := extractAllRanges(newTree, "")

	for path, oldRange := range oldRanges {
		newRange, exists := newRanges[path]
		if !exists {
			continue // Range no longer exists
		}

		// Build ID-to-key maps
		oldIDKeys := buildIDKeyMap(oldRange)
		newIDKeys := buildIDKeyMap(newRange)

		// For items that exist in both, keys must match
		for id, oldKey := range oldIDKeys {
			if newKey, exists := newIDKeys[id]; exists {
				if oldKey != newKey {
					violation := &Violation{
						Invariant:   "KeyStability",
						Description: fmt.Sprintf("Item %q at %s changed key from %q to %q", id, path, oldKey, newKey),
						OldTree:     oldTree,
						NewTree:     newTree,
						Mutations:   copyMutations(v.mutations),
						Seed:        v.seed,
					}
					v.violations = append(v.violations, *violation)
					return violation
				}
			}
		}
	}

	return nil
}

// VerifyTreeStructure checks that the tree maintains structural invariants.
//
// Invariant: len(statics) == len(dynamics) + 1 for non-range nodes
func (v *Verifier) VerifyTreeStructure(tree *build.TreeNode) error {
	return verifyTreeStructureRecursive(tree, "root", v)
}

func verifyTreeStructureRecursive(tree *build.TreeNode, path string, v *Verifier) error {
	if tree == nil {
		return nil
	}

	// Only check if this is a full tree (not a diff-only update)
	if !tree.HasStatics() {
		return nil // Diff-only, no structure check needed
	}

	// Check statics/dynamics ratio for non-range nodes
	if !tree.HasRange() {
		staticsLen := len(tree.Statics)
		dynamicsLen := len(tree.GetDynamics())

		if staticsLen != dynamicsLen+1 {
			violation := &Violation{
				Invariant: "TreeStructure",
				Description: fmt.Sprintf("at %s: len(statics)=%d, len(dynamics)=%d, expected len(statics)=len(dynamics)+1",
					path, staticsLen, dynamicsLen),
				NewTree:   tree,
				Mutations: copyMutations(v.mutations),
				Seed:      v.seed,
			}
			v.violations = append(v.violations, *violation)
			return violation
		}
	}

	// Check nested structures
	for k, value := range tree.GetDynamics() {
		if nested, ok := value.(*build.TreeNode); ok {
			if err := verifyTreeStructureRecursive(nested, path+"."+k, v); err != nil {
				return err
			}
		}
	}

	// Check range items
	if tree.HasRange() && tree.Range != nil {
		for i, item := range tree.Range.Items {
			if itemTree, ok := item.(*build.TreeNode); ok {
				itemPath := fmt.Sprintf("%s.d[%d]", path, i)
				if err := verifyTreeStructureRecursive(itemTree, itemPath, v); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// VerifyNoDataLoss checks that state survives JSON roundtrip.
func (v *Verifier) VerifyNoDataLoss(state any) error {
	if state == nil {
		return nil
	}

	// Serialize
	data, err := json.Marshal(state)
	if err != nil {
		violation := &Violation{
			Invariant:   "NoDataLoss",
			Description: fmt.Sprintf("JSON marshal failed: %v", err),
			NewState:    state,
			Mutations:   copyMutations(v.mutations),
			Seed:        v.seed,
		}
		v.violations = append(v.violations, *violation)
		return violation
	}

	// Deserialize
	var roundtripped any
	if err := json.Unmarshal(data, &roundtripped); err != nil {
		violation := &Violation{
			Invariant:   "NoDataLoss",
			Description: fmt.Sprintf("JSON unmarshal failed: %v", err),
			NewState:    state,
			Mutations:   copyMutations(v.mutations),
			Seed:        v.seed,
		}
		v.violations = append(v.violations, *violation)
		return violation
	}

	// Note: We can't do deep equality check here because JSON normalizes types
	// (int becomes float64, etc.). This is acceptable for state verification.

	return nil
}

// Helper functions

func treeHasStatics(tree *build.TreeNode) bool {
	if tree == nil {
		return false
	}
	return tree.HasStatics()
}

func isEmptyTree(tree *build.TreeNode) bool {
	if tree == nil {
		return true
	}
	return !tree.HasStatics() && !tree.HasDynamics() && !tree.HasRange()
}

// hasUnexpectedStatics checks if a diff tree contains statics that shouldn't be there.
// Some statics are expected in updates:
//   - Range statics when structure changes (e.g., empty→items transition)
//   - Statics in newly appearing branches (conditional becoming true)
//
// But statics should NOT appear for unchanged structures.
//
// Note: This check is intentionally lenient. Without access to the old tree,
// we can't distinguish between:
//  1. Static-only node for new structure (VALID - needs statics)
//  2. Static-only node for unchanged structure (INVALID - should be stripped)
//
// We err on the side of allowing statics to avoid false positives.
func hasUnexpectedStatics(tree *build.TreeNode) bool {
	if tree == nil {
		return false
	}

	// Check dynamics for nested statics
	for _, value := range tree.GetDynamics() {
		if nested, ok := value.(*build.TreeNode); ok {
			// Static-only nodes ARE valid in diffs when a conditional becomes true.
			// The client needs to receive the full structure including statics.
			// Only flag recursively unexpected statics in nested structures.
			if hasUnexpectedStatics(nested) {
				return true
			}
		}

		// Check range operations
		if ops, ok := value.([]any); ok {
			for _, op := range ops {
				if opArray, ok := op.([]any); ok && len(opArray) >= 2 {
					opType, _ := opArray[0].(string)
					// Insert, Append, Prepend MAY have statics (for new items)
					// Update should NOT have statics
					if opType == "u" && len(opArray) >= 3 {
						if changes, ok := opArray[2].(map[string]any); ok {
							if hasStaticsInMap(changes) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

func hasStaticsInMap(m map[string]any) bool {
	if _, has := m["s"]; has {
		return true
	}
	for _, v := range m {
		if nested, ok := v.(map[string]any); ok {
			if hasStaticsInMap(nested) {
				return true
			}
		}
	}
	return false
}

// extractAllRanges extracts all range constructs from a tree.
func extractAllRanges(tree *build.TreeNode, path string) map[string]*build.RangeData {
	ranges := make(map[string]*build.RangeData)

	if tree == nil {
		return ranges
	}

	if tree.HasRange() && tree.Range != nil {
		ranges[path] = tree.Range
	}

	for k, value := range tree.GetDynamics() {
		childPath := path
		if path != "" {
			childPath += "."
		}
		childPath += k

		if nested, ok := value.(*build.TreeNode); ok {
			for p, r := range extractAllRanges(nested, childPath) {
				ranges[p] = r
			}
		}
	}

	return ranges
}

// buildIDKeyMap builds a map from item IDs to their keys.
func buildIDKeyMap(rangeData *build.RangeData) map[string]string {
	idToKey := make(map[string]string)

	if rangeData == nil {
		return idToKey
	}

	for _, item := range rangeData.Items {
		id := extractItemID(item)
		key := extractKey(item, rangeData.Statics)
		if id != "" && key != "" {
			idToKey[id] = key
		}
	}

	return idToKey
}

// extractItemID extracts the semantic ID from an item (typically the "ID" field).
// For TreeNode items, the ID is usually at dynamic position "0".
// For map items, look for the "ID" key.
func extractItemID(item any) string {
	switch v := item.(type) {
	case *build.TreeNode:
		// ID is typically at position "0" (first dynamic in the template)
		if val, exists := v.GetDynamic(0); exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
	case map[string]any:
		// Try explicit "ID" field first
		if id, exists := v["ID"]; exists {
			if s, ok := id.(string); ok {
				return s
			}
		}
		// Fall back to "0" for tree-format maps
		if id, exists := v["0"]; exists {
			if s, ok := id.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractKey extracts the range key from an item using the canonical key
// extraction logic from the diff package. This checks for auto-generated keys
// (_k field), key attributes found via statics (id=, data-key=), and falls
// back to content-based hashing.
func extractKey(item any, statics interface{}) string {
	key, ok := diff.GetItemKey(item, statics)
	if ok {
		return key
	}
	return ""
}

func copyMutations(ms []mutations.Mutation) []mutations.Mutation {
	if ms == nil {
		return nil
	}
	result := make([]mutations.Mutation, len(ms))
	copy(result, ms)
	return result
}
