package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// TransitionToStreamMode replaces Range.Items with a RangeStreamState
// snapshot for every homogeneous range reachable through the static tree
// (root, plus any range inside a TreeNode reachable via Dynamics —
// including conditional and with branches). Caller must hold the lock
// that protects tree.
//
// The walk descends through Dynamics children but never enters a Range's
// Items: a range nested as items inside another range stays on the legacy
// per-item path (spec §5a "nested ranges always take legacy serialization"),
// because the inner range has no retained lastTree slot of its own. So the
// recursion preserves that wire-format invariant while fixing the silent
// fallback that occurred when a homogeneous range was wrapped in {{if}}
// or {{with}} — the conditional branch is a TreeNode in Dynamics, and
// without recursion the wrapped range never reached the homogeneity check.
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
