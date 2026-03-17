package build

import (
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
)

// CalculateStructureFingerprint calculates a fingerprint based ONLY on the static structure.
// It only considers:
// - Statics arrays (the HTML template parts)
// - The positions where dynamics exist (keys only, not values)
// - Nested TreeNode structures (recursively)
// - Range statics
//
// This enables the "fingerprint + full replace" optimization from Phoenix LiveView:
// - If structure fingerprint is the same, client already has statics cached
// - If structure fingerprint differs, send full tree with statics
//
// Two trees with the same StructureFingerprint have identical static HTML structure
// and can be diffed by comparing only dynamic values.
func CalculateStructureFingerprint(tree *TreeNode) string {
	if tree == nil {
		return ""
	}

	hasher := fnv.New128a()
	visitPath := make(map[*TreeNode]struct{})
	hashStructureWithCircularDetection(tree, hasher, visitPath)

	// Return 16 hex chars (64 bits) for compact representation.
	// FNV-1a 128-bit provides excellent distribution; truncating to 64 bits
	// gives <0.01% collision probability at 10,000 unique structures.
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// hashStructureWithCircularDetection hashes only the static structure of a tree.
// It includes statics and the shape of dynamics, but NOT dynamic values.
func hashStructureWithCircularDetection(tree *TreeNode, hasher hash.Hash, visitPath map[*TreeNode]struct{}) {
	// Check for circular reference in current path
	if _, found := visitPath[tree]; found {
		hasher.Write([]byte("<circular>\x00"))
		return
	}

	// Mark this node as part of current path
	visitPath[tree] = struct{}{}
	defer delete(visitPath, tree)

	// Hash statics (the core static structure)
	if tree.HasStatics() {
		_, _ = fmt.Fprintf(hasher, "s:%d:", len(tree.Statics))
		for _, s := range tree.Statics {
			hasher.Write([]byte(s))
			hasher.Write([]byte("\x00"))
		}
	}

	// Hash dynamic keys (positions only, not values)
	// This captures the structure: "there's a dynamic at position 0, 1, 2"
	keys := make([]string, 0, len(tree.Dynamics))
	for k := range tree.Dynamics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// For each dynamic position, hash the key and recurse into nested TreeNodes
	for _, k := range keys {
		_, _ = fmt.Fprintf(hasher, "d:%s:", k)

		value := tree.Dynamics[k]
		// Only recurse into nested TreeNodes to capture nested structure
		if nestedTree, ok := value.(*TreeNode); ok {
			hasher.Write([]byte("tree{"))
			hashStructureWithCircularDetection(nestedTree, hasher, visitPath)
			hasher.Write([]byte("}\x00"))
		} else {
			// For primitive values, just mark that a dynamic exists here
			// We don't hash the value itself - that's the key difference
			hasher.Write([]byte("val\x00"))
		}
	}

	// Hash range structure if present
	if tree.Range != nil {
		hasher.Write([]byte("range{"))
		// Hash range statics (item template structure)
		if len(tree.Range.Statics) > 0 {
			_, _ = fmt.Fprintf(hasher, "rs:%d:", len(tree.Range.Statics))
			for _, s := range tree.Range.Statics {
				hasher.Write([]byte(s))
				hasher.Write([]byte("\x00"))
			}
		}
		hasher.Write([]byte("}\x00"))
	}
}
