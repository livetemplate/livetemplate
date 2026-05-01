package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// TransitionToStreamMode replaces Range.Items with a RangeStreamState snapshot
// for top-level homogeneous ranges. Caller must hold the lock that protects tree.
//
// Uses CalculateStaticsFingerprint (not GetStructureFingerprint) for the
// homogeneity check — the latter over-captures scalar position-presence,
// falsely classifying [id, "x"] vs [id, nil] as heterogeneous.
func TransitionToStreamMode(tree *build.TreeNode) {
	if tree == nil {
		return
	}
	for _, value := range tree.Dynamics {
		node, ok := value.(*build.TreeNode)
		if !ok || !node.HasRange() || node.Range == nil {
			continue
		}
		rd := node.Range
		if rd.StreamState != nil {
			continue
		}
		if len(rd.Items) == 0 {
			continue
		}

		firstItem, ok := rd.Items[0].(*build.TreeNode)
		if !ok {
			continue
		}
		ref := build.CalculateStaticsFingerprint(firstItem)
		homogeneous := true
		for _, item := range rd.Items[1:] {
			itemNode, ok := item.(*build.TreeNode)
			if !ok || build.CalculateStaticsFingerprint(itemNode) != ref {
				homogeneous = false
				break
			}
		}
		if !homogeneous {
			continue
		}

		keyPos := FindKeyPositionFromStatics(rd.Statics)
		keysList := make([]string, len(rd.Items))
		hashes := make([]uint64, len(rd.Items))
		for i, item := range rd.Items {
			itemNode := item.(*build.TreeNode)
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
}
