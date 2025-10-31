package build

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sort"
	"strconv"
)

// CalculateFingerprint calculates a 64-bit fingerprint (MD5 hash) for a tree's statics and dynamics.
// This allows detecting when a subtree has changed, similar to LiveView's optimization #2.
// Optimized to use incremental hashing instead of full JSON marshaling.
func CalculateFingerprint(tree *TreeNode) string {
	hasher := md5.New()
	HashTreeIncremental(tree, hasher)

	// Return first 16 characters of hex (64 bits)
	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) >= 16 {
		return fullHash[:16]
	}
	return fullHash
}

// HashTreeIncremental incrementally hashes a tree node without full JSON marshaling.
// This is much faster for nested trees as it avoids marshaling entire subtrees.
func HashTreeIncremental(tree *TreeNode, hasher hash.Hash) {
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
	var keys []string
	for k := range tree.Dynamics {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		num1, err1 := strconv.Atoi(keys[i])
		num2, err2 := strconv.Atoi(keys[j])
		if err1 == nil && err2 == nil {
			return num1 < num2
		}
		return keys[i] < keys[j]
	})

	// Hash each dynamic value incrementally
	for _, k := range keys {
		value := tree.Dynamics[k]
		hasher.Write([]byte(k))
		hasher.Write([]byte(":"))
		HashValueIncremental(value, hasher)
		hasher.Write([]byte("\x00")) // Null byte separator
	}
}

// HashValueIncremental hashes a value incrementally based on its type.
// For nested trees, it recursively hashes instead of marshaling.
func HashValueIncremental(value interface{}, hasher hash.Hash) {
	switch v := value.(type) {
	case *TreeNode:
		// Nested tree node - recursively hash it
		hasher.Write([]byte("tree{"))
		HashTreeIncremental(v, hasher)
		hasher.Write([]byte("}"))

	case map[string]interface{}:
		// Plain map - hash as JSON (rare case for raw data)
		hasher.Write([]byte("map{"))
		mapJSON, _ := json.Marshal(v)
		hasher.Write(mapJSON)
		hasher.Write([]byte("}"))

	case []interface{}:
		// Array - hash each element
		hasher.Write([]byte(fmt.Sprintf("arr[%d]:", len(v))))
		for i, item := range v {
			hasher.Write([]byte(fmt.Sprintf("%d:", i)))
			HashValueIncremental(item, hasher)
		}
		hasher.Write([]byte("]"))

	case string:
		hasher.Write([]byte("str:"))
		hasher.Write([]byte(v))

	case int:
		hasher.Write([]byte(fmt.Sprintf("int:%d", v)))

	case int64:
		hasher.Write([]byte(fmt.Sprintf("i64:%d", v)))

	case float64:
		hasher.Write([]byte(fmt.Sprintf("f64:%f", v)))

	case bool:
		hasher.Write([]byte(fmt.Sprintf("bool:%t", v)))

	case nil:
		hasher.Write([]byte("nil"))

	default:
		// Fallback to JSON marshal for unknown types
		// This is rare and only happens for custom types
		valueJSON, _ := json.Marshal(v)
		hasher.Write([]byte("json:"))
		hasher.Write(valueJSON)
	}
}

// AddFingerprintToTree adds the fingerprint to the tree for client-side tracking.
// NOTE: This should be internal-only for conditional branch detection.
func AddFingerprintToTree(tree *TreeNode) *TreeNode {
	if !tree.HasStatics() && !tree.HasDynamics() {
		return tree // Don't add fingerprint to empty trees
	}

	// For now, don't expose fingerprint to clients - keep it internal
	// fingerprint := CalculateFingerprint(tree)
	// tree.Fingerprint = fingerprint
	return tree
}
