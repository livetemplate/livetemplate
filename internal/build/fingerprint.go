package build

import (
	"encoding/hex"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
)

// Pre-allocated byte slices to avoid repeated string→[]byte conversions in the hot path.
var (
	fpSep      = []byte("\x00")
	fpCircular = []byte("<circular>\x00")
	fpStatPfx  = []byte("s:")
	fpDynPfx   = []byte("d:")
	fpColon    = []byte(":")
	fpTreeOpen = []byte("tree{")
	fpClose    = []byte("}\x00")
	fpVal      = []byte("val\x00")
	fpRngOpen  = []byte("range{")
	fpRngPfx   = []byte("rs:")
)

// CalculateStructureFingerprint calculates a fingerprint based ONLY on the static structure.
// It only considers:
// - Statics arrays (the HTML template parts)
// - The positions where dynamics exist (keys only, not values)
// - Nested TreeNode structures (recursively)
// - Range statics
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
	// Collision is only relevant across distinct template structures within
	// a single application; the failure mode is a redundant statics resend.
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// hashStructureWithCircularDetection hashes only the static structure of a tree.
// It includes statics and the shape of dynamics, but NOT dynamic values.
func hashStructureWithCircularDetection(tree *TreeNode, hasher hash.Hash, visitPath map[*TreeNode]struct{}) {
	if _, found := visitPath[tree]; found {
		hasher.Write(fpCircular)
		return
	}

	visitPath[tree] = struct{}{}
	defer delete(visitPath, tree)

	if tree.HasStatics() {
		hasher.Write(fpStatPfx)
		hasher.Write([]byte(strconv.Itoa(len(tree.Statics))))
		hasher.Write(fpColon)
		for _, s := range tree.Statics {
			hasher.Write([]byte(s))
			hasher.Write(fpSep)
		}
	}

	keys := make([]string, 0, len(tree.Dynamics))
	for k := range tree.Dynamics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		hasher.Write(fpDynPfx)
		hasher.Write([]byte(k))
		hasher.Write(fpColon)

		value := tree.Dynamics[k]
		if nestedTree, ok := value.(*TreeNode); ok {
			hasher.Write(fpTreeOpen)
			hashStructureWithCircularDetection(nestedTree, hasher, visitPath)
			hasher.Write(fpClose)
		} else {
			hasher.Write(fpVal)
		}
	}

	if tree.Range != nil {
		hasher.Write(fpRngOpen)
		if len(tree.Range.Statics) > 0 {
			hasher.Write(fpRngPfx)
			hasher.Write([]byte(strconv.Itoa(len(tree.Range.Statics))))
			hasher.Write(fpColon)
			for _, s := range tree.Range.Statics {
				hasher.Write([]byte(s))
				hasher.Write(fpSep)
			}
		}
		hasher.Write(fpClose)
	}
}
