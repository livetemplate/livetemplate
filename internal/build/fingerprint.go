package build

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"sort"
)

// CalculateFingerprint calculates a 64-bit fingerprint (MD5 hash) for a tree's statics and dynamics.
// This allows detecting when a subtree has changed, similar to LiveView's optimization #2.
// Optimized to use incremental hashing instead of full JSON marshaling.
//
// MD5 Usage Note:
// MD5 is used here for change detection only, not for security purposes.
// It is sufficient for detecting accidental changes in template trees and provides
// good performance characteristics. For this use case, the collision probability
// is acceptable and cryptographic strength is not required.
//
// Nil Input: Returns empty string for nil tree.
func CalculateFingerprint(tree *TreeNode) string {
	if tree == nil {
		return ""
	}

	hasher := md5.New()
	// visitPath tracks the current path for circular detection, not all seen nodes
	visitPath := make(map[*TreeNode]struct{})
	hashTreeWithCircularDetection(tree, hasher, visitPath)

	// Truncated to 64 bits (16 hex chars) for compact fingerprints.
	// Collision probability: negligible for <1M unique structures (~1 in 10^9).
	// Even if collision occurs, worst case is sending extra statics (no correctness issue).
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// hashTreeWithCircularDetection hashes a tree with circular reference detection.
// visitPath tracks the current recursion path to detect cycles, allowing the same
// node to appear in different branches (structural sharing) while preventing infinite loops.
func hashTreeWithCircularDetection(tree *TreeNode, hasher hash.Hash, visitPath map[*TreeNode]struct{}) {
	// Check for circular reference in current path
	if _, found := visitPath[tree]; found {
		hasher.Write([]byte("<circular>\x00"))
		return
	}

	// Mark this node as part of current path
	visitPath[tree] = struct{}{}
	// Defer removal so node can appear in other branches
	defer delete(visitPath, tree)

	// Add statics to hash (template structure)
	if tree.HasStatics() {
		// Write statics count first for disambiguation
		// Note: hash.Write never returns an error, so we ignore the return value
		_, _ = fmt.Fprintf(hasher, "s:%d:", len(tree.Statics))
		for _, s := range tree.Statics {
			hasher.Write([]byte(s))
			hasher.Write([]byte("\x00")) // Null byte separator
		}
	}

	// Collect and sort dynamic keys for consistent hashing.
	// Uses lexicographic sorting (sort.Strings) which is sufficient because:
	// 1. Dynamic keys are string-formatted indices ("0", "1", "2", etc.) from template positions
	// 2. Lexicographic order provides consistent ordering across runs (deterministic hashing)
	// 3. The actual numeric values don't matter for change detection - only consistency matters
	// 4. Simpler and faster than numeric-aware sorting
	//
	// Note on lexicographic vs numeric order:
	// Lexicographic: "0", "1", "10", "2", "3"... (sort.Strings)
	// Numeric:       "0", "1", "2", "3"... "10"  (parseInt sort)
	//
	// This does NOT affect the client because:
	// - Fingerprints are server-side only (client never sees them)
	// - Client iterates dynamics by sequential index from statics array length
	// - When client needs key sorting, it uses parseInt() for numeric order
	// - JSON has unordered keys by specification anyway
	keys := make([]string, 0, len(tree.Dynamics))
	for k := range tree.Dynamics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Hash each dynamic value incrementally
	for _, k := range keys {
		value := tree.Dynamics[k]
		// Write key with length prefix to prevent collisions
		_, _ = fmt.Fprintf(hasher, "k%d:", len(k))
		hasher.Write([]byte(k))
		hasher.Write([]byte(":"))
		hashValueWithCircularDetection(value, hasher, visitPath)
		hasher.Write([]byte("\x00")) // Null byte separator
	}
}

// hashValueWithCircularDetection hashes a value with circular reference detection.
// All values are terminated with null byte for consistent delimiter usage.
func hashValueWithCircularDetection(value interface{}, hasher hash.Hash, visitPath map[*TreeNode]struct{}) {
	switch v := value.(type) {
	case *TreeNode:
		// Nested tree node - recursively hash it with circular detection
		hasher.Write([]byte("tree{"))
		hashTreeWithCircularDetection(v, hasher, visitPath)
		hasher.Write([]byte("}\x00"))

	case map[string]interface{}:
		// Plain map - hash as JSON (rare case for raw data)
		hasher.Write([]byte("map{"))
		mapJSON, err := json.Marshal(v)
		if err != nil {
			// For unmarshalable types, use type information instead
			// This prevents different errors from producing different hashes for same data
			_, _ = fmt.Fprintf(hasher, "<type:%T>", v)
		} else {
			hasher.Write(mapJSON)
		}
		hasher.Write([]byte("}\x00"))

	case []interface{}:
		// Array - hash each element with length prefix
		_, _ = fmt.Fprintf(hasher, "arr[%d]:", len(v))
		for _, item := range v {
			hashValueWithCircularDetection(item, hasher, visitPath)
		}
		hasher.Write([]byte("]\x00"))

	case string:
		// Write string with length prefix to prevent collisions
		_, _ = fmt.Fprintf(hasher, "str%d:", len(v))
		hasher.Write([]byte(v))
		hasher.Write([]byte("\x00"))

	case int:
		_, _ = fmt.Fprintf(hasher, "int:%d\x00", v)

	case int64:
		_, _ = fmt.Fprintf(hasher, "i64:%d\x00", v)

	case float64:
		// Use binary representation for exact equality
		bits := math.Float64bits(v)
		_, _ = fmt.Fprintf(hasher, "f64:%016x\x00", bits)

	case bool:
		_, _ = fmt.Fprintf(hasher, "bool:%t\x00", v)

	case nil:
		hasher.Write([]byte("nil\x00"))

	default:
		// Fallback to JSON marshal for unknown types
		hasher.Write([]byte("json:"))
		valueJSON, err := json.Marshal(v)
		if err != nil {
			// For unmarshalable types, use type information instead
			_, _ = fmt.Fprintf(hasher, "<type:%T>", v)
		} else {
			hasher.Write(valueJSON)
		}
		hasher.Write([]byte("\x00"))
	}
}

// CalculateStructureFingerprint calculates a fingerprint based ONLY on the static structure.
// Unlike CalculateFingerprint which includes dynamic values, this only considers:
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

	hasher := md5.New()
	visitPath := make(map[*TreeNode]struct{})
	hashStructureWithCircularDetection(tree, hasher, visitPath)

	// Return 16 hex chars (64 bits) for compact representation
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
