package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// TransitionToStreamMode replaces Range.Items with a RangeStreamState snapshot
// for top-level homogeneous ranges. Caller must hold the lock that protects tree.
//
// "Top-level" covers two cases: the root tree may itself be a Range (when the
// template is just `{{range}}` with no wrapper element), or a Range may sit
// directly inside the root tree's Dynamics. Both are at depth ≤ 1 and qualify
// per proposal §5a — nested ranges deeper than that stay legacy.
//
// Uses CalculateStaticsFingerprint (not GetStructureFingerprint) for the
// homogeneity check — the latter over-captures scalar position-presence,
// falsely classifying [id, "x"] vs [id, nil] as heterogeneous.
func TransitionToStreamMode(tree *build.TreeNode) {
	if tree == nil {
		return
	}
	if tree.HasRange() {
		transitionRangeIfHomogeneous(tree.Range)
	}
	for _, value := range tree.Dynamics {
		node, ok := value.(*build.TreeNode)
		if !ok || !node.HasRange() {
			continue
		}
		transitionRangeIfHomogeneous(node.Range)
	}
}

// transitionRangeIfHomogeneous applies the §5a homogeneity check to a single
// RangeData and, if all items share a statics fingerprint, swaps Items for a
// RangeStreamState snapshot. No-op for nil/empty/already-transitioned ranges.
func transitionRangeIfHomogeneous(rd *build.RangeData) {
	if rd == nil || rd.StreamState != nil || len(rd.Items) == 0 {
		return
	}
	firstItem, ok := rd.Items[0].(*build.TreeNode)
	if !ok {
		return
	}
	ref := build.CalculateStaticsFingerprint(firstItem)
	for _, item := range rd.Items[1:] {
		itemNode, ok := item.(*build.TreeNode)
		if !ok || build.CalculateStaticsFingerprint(itemNode) != ref {
			return
		}
	}

	keyPos := FindKeyPositionFromStatics(rd.Statics)
	keysList := make([]string, len(rd.Items))
	hashes := make([]uint64, len(rd.Items))
	for i, item := range rd.Items {
		itemNode, ok := item.(*build.TreeNode)
		if !ok {
			continue
		}
		if keyStr, ok := getItemKeyWithPos(itemNode, keyPos); ok {
			keysList[i] = keyStr
		}
		hashes[i] = keys.ItemHashUint64(itemNode.Dynamics)
	}
	rd.StreamState = &build.RangeStreamState{
		Keys:        keysList,
		Hashes:      hashes,
		Fingerprint: ref,
	}
	rd.Items = nil
}
