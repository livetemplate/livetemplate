package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// TransitionToStreamMode replaces Range.Items with a RangeStreamState
// snapshot for every homogeneous range reachable through the static tree
// (root plus any TreeNode in Dynamics, including conditional and with
// branches). Never descends into Range.Items, keeping nested-range-in-item
// on the legacy path (spec §5a). Caller must hold the lock that protects
// tree. Uses CalculateStaticsFingerprint (not GetStructureFingerprint) —
// the latter falsely classifies [id, "x"] vs [id, nil] as heterogeneous.
func TransitionToStreamMode(tree *build.TreeNode) {
	if tree == nil {
		return
	}
	if tree.HasRange() {
		transitionRangeIfHomogeneous(tree.Range)
	}
	for _, value := range tree.Dynamics {
		node, ok := value.(*build.TreeNode)
		if !ok {
			continue
		}
		TransitionToStreamMode(node)
	}
}

// transitionRangeIfHomogeneous applies the spec §5a homogeneity check to a
// single RangeData and, if all items share a statics fingerprint, swaps Items
// for a RangeStreamState snapshot. No-op for nil/empty/already-transitioned
// ranges.
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
