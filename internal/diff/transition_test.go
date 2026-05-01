package diff

import (
	"strconv"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/keys"
)

// homoItemRangeTree builds a top-level tree containing one Range with
// `count` homogeneous items. Each item has Dynamics [keyStr, "label-N"]
// and Statics [`<li data-key="`, `">`, `</li>`].
func homoItemRangeTree(count int) *build.TreeNode {
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	items := make([]interface{}, count)
	for i := 0; i < count; i++ {
		items[i] = &build.TreeNode{
			Statics:  itemStatics,
			Dynamics: []interface{}{keyForIdx(i), "label-" + keyForIdx(i)},
		}
	}
	return &build.TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Dynamics: []interface{}{
			&build.TreeNode{
				Statics: []string{"<ul>", "</ul>"},
				Range: &build.RangeData{
					Items:   items,
					Statics: itemStatics,
				},
			},
		},
	}
}

func keyForIdx(i int) string {
	return strconv.Itoa(i)
}

func TestTransitionToStreamMode_HomogeneousFires(t *testing.T) {
	tree := homoItemRangeTree(3)
	rangeNode := tree.Dynamics[0].(*build.TreeNode)
	originalItems := rangeNode.Range.Items

	TransitionToStreamMode(tree)

	if rangeNode.Range.Items != nil {
		t.Errorf("Items should be nil post-transition, got %v", rangeNode.Range.Items)
	}
	if rangeNode.Range.StreamState == nil {
		t.Fatalf("StreamState should be populated post-transition")
	}
	ss := rangeNode.Range.StreamState
	if len(ss.Keys) != 3 || len(ss.Hashes) != 3 {
		t.Errorf("StreamState should have 3 keys and 3 hashes, got Keys=%v Hashes=%v", ss.Keys, ss.Hashes)
	}
	if ss.Fingerprint == "" {
		t.Errorf("StreamState.Fingerprint should be set")
	}
	// Hashes must be deterministic from the original items' dynamics.
	for i, item := range originalItems {
		expected := keys.ItemHashUint64(item.(*build.TreeNode).Dynamics)
		if ss.Hashes[i] != expected {
			t.Errorf("Hashes[%d] = %d, want %d", i, ss.Hashes[i], expected)
		}
	}
}

func TestTransitionToStreamMode_HeterogeneousDefers(t *testing.T) {
	tree := homoItemRangeTree(2)
	rangeNode := tree.Dynamics[0].(*build.TreeNode)
	// Make the second item structurally different by adding a nested *TreeNode
	// at a new dynamic position — this changes its statics fingerprint.
	item2 := rangeNode.Range.Items[1].(*build.TreeNode)
	item2.Statics = []string{`<li data-key="`, `">`, `<span>`, `</span></li>`}
	item2.Dynamics = append(item2.Dynamics, &build.TreeNode{Statics: []string{"badge"}})

	TransitionToStreamMode(tree)

	if rangeNode.Range.StreamState != nil {
		t.Errorf("StreamState should be nil for heterogeneous range (deferred to legacy)")
	}
	if rangeNode.Range.Items == nil {
		t.Errorf("Items should remain populated for het-deferred range")
	}
}

func TestTransitionToStreamMode_EmptyDefers(t *testing.T) {
	tree := homoItemRangeTree(0) // zero items
	rangeNode := tree.Dynamics[0].(*build.TreeNode)

	TransitionToStreamMode(tree)

	if rangeNode.Range.StreamState != nil {
		t.Errorf("StreamState should be nil for empty range (deferred per §5a empty-range edge case)")
	}
	if rangeNode.Range.Items == nil {
		t.Errorf("Items should remain non-nil (empty slice) for empty-deferred range")
	}
}

func TestTransitionToStreamMode_SingleItemFires(t *testing.T) {
	tree := homoItemRangeTree(1)
	rangeNode := tree.Dynamics[0].(*build.TreeNode)

	TransitionToStreamMode(tree)

	if rangeNode.Range.StreamState == nil {
		t.Errorf("Single-item range is trivially homogeneous and should transition")
	}
	if len(rangeNode.Range.StreamState.Keys) != 1 {
		t.Errorf("StreamState should have 1 key, got %d", len(rangeNode.Range.StreamState.Keys))
	}
}

func TestTransitionToStreamMode_Idempotent(t *testing.T) {
	tree := homoItemRangeTree(3)
	rangeNode := tree.Dynamics[0].(*build.TreeNode)

	TransitionToStreamMode(tree)
	firstSS := rangeNode.Range.StreamState
	if firstSS == nil {
		t.Fatalf("Expected first transition to fire")
	}

	// Second invocation should be a no-op (StreamState already set).
	TransitionToStreamMode(tree)
	if rangeNode.Range.StreamState != firstSS {
		t.Errorf("Idempotent: re-invocation should not replace StreamState pointer")
	}
}

func TestTransitionToStreamMode_NestedRangesNotTransitioned(t *testing.T) {
	// Outer range with one item; that item itself contains a nested range.
	innerStatics := []string{`<sub data-key="`, `">`, `</sub>`}
	innerItems := []interface{}{
		&build.TreeNode{Statics: innerStatics, Dynamics: []interface{}{"x", "X"}},
		&build.TreeNode{Statics: innerStatics, Dynamics: []interface{}{"y", "Y"}},
	}
	innerRangeNode := &build.TreeNode{
		Statics: []string{"<div>", "</div>"},
		Range: &build.RangeData{
			Items:   innerItems,
			Statics: innerStatics,
		},
	}

	outerItemStatics := []string{`<li data-key="`, `">`, `</li>`}
	outerItem := &build.TreeNode{
		Statics:  outerItemStatics,
		Dynamics: []interface{}{"a", innerRangeNode},
	}

	tree := &build.TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Dynamics: []interface{}{
			&build.TreeNode{
				Statics: []string{"<ul>", "</ul>"},
				Range: &build.RangeData{
					Items:   []interface{}{outerItem},
					Statics: outerItemStatics,
				},
			},
		},
	}

	TransitionToStreamMode(tree)

	outerRange := tree.Dynamics[0].(*build.TreeNode).Range
	if outerRange.StreamState == nil {
		t.Errorf("Outer top-level range should transition")
	}
	if innerRangeNode.Range.StreamState != nil {
		t.Errorf("Nested range MUST NOT transition (proposal §5a top-level only)")
	}
	if innerRangeNode.Range.Items == nil {
		t.Errorf("Nested range Items must remain populated for legacy serialization")
	}
}

func TestTransitionToStreamMode_NilTreeSafe(t *testing.T) {
	// Must not panic on nil input.
	TransitionToStreamMode(nil)
}

// TestTransitionToStreamMode_RootRangeFires covers the case where the root
// tree itself is a Range (template `{{range .Items}}<x/>{{end}}` with no
// wrapper element). The root range counts as top-level per spec §5a and must
// transition. Without this case, wrapper-less templates silently fall through
// to the legacy diff path even when their range is homogeneous.
func TestTransitionToStreamMode_RootRangeFires(t *testing.T) {
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	tree := &build.TreeNode{
		Statics: itemStatics,
		Range: &build.RangeData{
			Statics: itemStatics,
			Items: []interface{}{
				&build.TreeNode{Statics: itemStatics, Dynamics: []interface{}{"a", "alpha"}},
				&build.TreeNode{Statics: itemStatics, Dynamics: []interface{}{"b", "beta"}},
			},
		},
	}

	TransitionToStreamMode(tree)

	if tree.Range.Items != nil {
		t.Errorf("Items should be nil after root-range transition, got %v", tree.Range.Items)
	}
	if tree.Range.StreamState == nil {
		t.Fatalf("StreamState should be populated for root-range transition")
	}
	ss := tree.Range.StreamState
	if len(ss.Keys) != 2 || ss.Keys[0] != "a" || ss.Keys[1] != "b" {
		t.Errorf("Expected keys [a, b], got %v", ss.Keys)
	}
}
