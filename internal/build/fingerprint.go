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

	// MD5 always produces 16 bytes (32 hex chars), so truncate to 16 chars (64 bits)
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
		hasher.Write([]byte(fmt.Sprintf("s:%d:", len(tree.Statics))))
		for _, s := range tree.Statics {
			hasher.Write([]byte(s))
			hasher.Write([]byte("\x00")) // Null byte separator
		}
	}

	// Collect and sort dynamic keys for consistent hashing
	// Uses lexicographic sorting (sort.Strings) which is sufficient because:
	// 1. Dynamic keys are string-formatted indices ("0", "1", "2", etc.) from template positions
	// 2. Lexicographic order provides consistent ordering across runs (deterministic hashing)
	// 3. The actual numeric values don't matter for change detection - only consistency matters
	// 4. Simpler and faster than numeric-aware sorting
	keys := make([]string, 0, len(tree.Dynamics))
	for k := range tree.Dynamics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Hash each dynamic value incrementally
	for _, k := range keys {
		value := tree.Dynamics[k]
		// Write key with length prefix to prevent collisions
		hasher.Write([]byte(fmt.Sprintf("k%d:", len(k))))
		hasher.Write([]byte(k))
		hasher.Write([]byte(":"))
		hashValueWithCircularDetection(value, hasher, visitPath)
		hasher.Write([]byte("\x00")) // Null byte separator
	}
}

// HashTreeIncremental incrementally hashes a tree node without full JSON marshaling.
// This is much faster for nested trees as it avoids marshaling entire subtrees.
// Deprecated: Use CalculateFingerprint which includes circular reference detection.
func HashTreeIncremental(tree *TreeNode, hasher hash.Hash) {
	visitPath := make(map[*TreeNode]struct{})
	hashTreeWithCircularDetection(tree, hasher, visitPath)
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
			hasher.Write([]byte(fmt.Sprintf("<type:%T>", v)))
		} else {
			hasher.Write(mapJSON)
		}
		hasher.Write([]byte("}\x00"))

	case []interface{}:
		// Array - hash each element with length prefix
		hasher.Write([]byte(fmt.Sprintf("arr[%d]:", len(v))))
		for _, item := range v {
			hashValueWithCircularDetection(item, hasher, visitPath)
		}
		hasher.Write([]byte("]\x00"))

	case string:
		// Write string with length prefix to prevent collisions
		hasher.Write([]byte(fmt.Sprintf("str%d:", len(v))))
		hasher.Write([]byte(v))
		hasher.Write([]byte("\x00"))

	case int:
		hasher.Write([]byte(fmt.Sprintf("int:%d\x00", v)))

	case int64:
		hasher.Write([]byte(fmt.Sprintf("i64:%d\x00", v)))

	case float64:
		// Use binary representation for exact equality
		bits := math.Float64bits(v)
		hasher.Write([]byte(fmt.Sprintf("f64:%016x\x00", bits)))

	case bool:
		hasher.Write([]byte(fmt.Sprintf("bool:%t\x00", v)))

	case nil:
		hasher.Write([]byte("nil\x00"))

	default:
		// Fallback to JSON marshal for unknown types
		hasher.Write([]byte("json:"))
		valueJSON, err := json.Marshal(v)
		if err != nil {
			// For unmarshalable types, use type information instead
			hasher.Write([]byte(fmt.Sprintf("<type:%T>", v)))
		} else {
			hasher.Write(valueJSON)
		}
		hasher.Write([]byte("\x00"))
	}
}

// HashValueIncremental hashes a value incrementally based on its type.
// For nested trees, it recursively hashes instead of marshaling.
// Deprecated: Use CalculateFingerprint which includes circular reference detection.
func HashValueIncremental(value interface{}, hasher hash.Hash) {
	visitPath := make(map[*TreeNode]struct{})
	hashValueWithCircularDetection(value, hasher, visitPath)
}

// AddFingerprintToTree is deprecated and does nothing.
// Fingerprinting is handled internally by CalculateFingerprint.
// This function is kept for backward compatibility and will be removed in a future version.
//
// Deprecated: Use CalculateFingerprint directly instead.
func AddFingerprintToTree(tree *TreeNode) *TreeNode {
	return tree
}
