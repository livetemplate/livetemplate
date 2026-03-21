package build

import (
	"encoding/hex"
	"hash"
	"hash/fnv"
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
// This enables the "fingerprint + full replace" optimization (inspired by Phoenix LiveView):
// - If structure fingerprint is the same, client already has statics cached
// - If structure fingerprint differs, send full tree with statics
//
// Two trees with the same StructureFingerprint have identical static HTML structure
// and can be diffed by comparing only dynamic values.
//
// Invariant: always returns a non-empty string for non-nil input.
// The empty string is reserved as an invalidation sentinel in GetStructureFingerprint.
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

	var intBuf [20]byte

	if tree.HasStatics() {
		hasher.Write(fpStatPfx)
		hasher.Write(strconv.AppendInt(intBuf[:0], int64(len(tree.Statics)), 10))
		hasher.Write(fpColon)
		for _, s := range tree.Statics {
			hasher.Write([]byte(s))
			hasher.Write(fpSep)
		}
	}

	for i, v := range tree.Dynamics {
		if v == nil {
			continue
		}
		hasher.Write(fpDynPfx)
		hasher.Write(strconv.AppendInt(intBuf[:0], int64(i), 10))
		hasher.Write(fpColon)

		if nestedTree, ok := v.(*TreeNode); ok {
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
			hasher.Write(strconv.AppendInt(intBuf[:0], int64(len(tree.Range.Statics)), 10))
			hasher.Write(fpColon)
			for _, s := range tree.Range.Statics {
				hasher.Write([]byte(s))
				hasher.Write(fpSep)
			}
		}
		hasher.Write(fpClose)
	}
}
