package diff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestStreamInsert_StripsNestedDynamicBranchStatics — issue #413.
//
// The stream-mode update path (`dynamicsToUpdatePayload`) already strips
// nested statics via `PrepareTreeForClient(v, true)` because the §5b
// homogeneity check guarantees the client has them. Insertions were
// inconsistent: they kept all nested statics via `PrepareTreeForClient(item,
// false)`, re-emitting per-row scaffolding the client had cached.
//
// After the fix in #413, stream-mode insertions also pass the
// homogeneity-guaranteed flag, so nested branches containing dynamics drop
// their statics on the wire. Static-only branches (whose entire content is
// HTML, no dynamics) still retain statics — the special case in
// `PrepareTreeForClient` preserves branch identity for those, since the
// client otherwise has no way to distinguish two static-only branches at the
// same item position.
func TestStreamInsert_StripsNestedDynamicBranchStatics(t *testing.T) {
	// Build a homogeneous range "old side" with items shaped like:
	//   {data-key, marker-branch-with-dynamic}
	// where the marker is `{{if .HasMarker}}<m>{{.MarkerText}}</m>{{end}}`.
	// All items share the same statics-fingerprint shape, so
	// TransitionToStreamMode succeeds.
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	makeItem := func(key, markerText string) *TreeNode {
		marker := &TreeNode{
			Statics:  []string{"<m>", "</m>"},
			Dynamics: []interface{}{markerText},
		}
		return &TreeNode{
			Statics:  itemStatics, // before extractItemDynamics, item carries statics
			Dynamics: []interface{}{key, marker},
		}
	}

	oldItems := []interface{}{
		extractDynamics(makeItem("k1", "old-marker-1")),
		extractDynamics(makeItem("k2", "old-marker-2")),
	}
	oldTree := &TreeNode{
		Statics: itemStatics,
		Range:   &build.RangeData{Items: oldItems, Statics: itemStatics},
	}
	TransitionToStreamMode(oldTree)
	if oldTree.Range.StreamState == nil {
		t.Fatalf("TransitionToStreamMode left StreamState nil — fixture lost homogeneity?")
	}

	// New side: all items replaced with new keys (full content swap).
	// Items have the SAME structure (marker with dynamic), but different
	// dynamic values.
	newItems := []interface{}{
		extractDynamics(makeItem("n1", "new-marker-1")),
		extractDynamics(makeItem("n2", "new-marker-2")),
	}
	newTree := &TreeNode{
		Statics: itemStatics,
		Range:   &build.RangeData{Items: newItems, Statics: itemStatics},
	}

	// Drive the dispatch path so stripStatics == true (fingerprint match).
	changes := &TreeNode{}
	if !handleStreamModeRange(oldTree, newTree, changes) {
		t.Fatalf("handleStreamModeRange did not dispatch to stream mode")
	}

	if changes.Range == nil {
		t.Fatalf("expected changes.Range to be populated; got: %+v", changes)
	}

	payload, err := json.Marshal(changes.Range.Items)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// The marker's `<m>` static SHOULD be absent — the client has it cached
	// from the homogeneity guarantee.
	if strings.Contains(string(payload), `<m>`) {
		t.Errorf("nested marker statics leaked into stream-mode insert payload (issue #413)\n  payload=%s", payload)
	}

	// The new dynamic content MUST still arrive.
	if !strings.Contains(string(payload), "new-marker-1") || !strings.Contains(string(payload), "new-marker-2") {
		t.Errorf("new-side dynamic content missing from insert payload\n  payload=%s", payload)
	}
}

// TestStreamInsert_PreservesStaticOnlyBranchIdentity — companion to the
// previous test. For static-only conditional branches (e.g.
// `{{if .Add}}+{{end}}`) the branch's statics ARE the branch identity, and
// stripping them would make `{s:["+"]}` and `{s:["-"]}` indistinguishable on
// the wire. The special case in `PrepareTreeForClient` retains those, so the
// fix in #413 must not regress this case.
func TestStreamInsert_PreservesStaticOnlyBranchIdentity(t *testing.T) {
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	// Build items where position 1 is a static-only nested *TreeNode (e.g.
	// the {{else}} branch of a multi-branch conditional rendered with no
	// dynamics).
	makeItem := func(key, branchStatic string) *TreeNode {
		branch := &TreeNode{Statics: []string{branchStatic}}
		return &TreeNode{
			Statics:  itemStatics,
			Dynamics: []interface{}{key, branch},
		}
	}

	// Need homogeneous old side to enter stream mode. All items use the same
	// branch shape (static-only), but they may differ in what static text
	// fills the branch — that's a STATICS difference, not a structure
	// difference, so the staticsFingerprint matches.
	oldItems := []interface{}{
		extractDynamics(makeItem("k1", "+")),
		extractDynamics(makeItem("k2", "+")),
	}
	oldTree := &TreeNode{
		Statics: itemStatics,
		Range:   &build.RangeData{Items: oldItems, Statics: itemStatics},
	}
	TransitionToStreamMode(oldTree)
	if oldTree.Range.StreamState == nil {
		t.Fatalf("TransitionToStreamMode left StreamState nil — fixture lost homogeneity?")
	}

	newItems := []interface{}{
		extractDynamics(makeItem("n1", "+")),
		extractDynamics(makeItem("n2", "+")),
	}
	newTree := &TreeNode{
		Statics: itemStatics,
		Range:   &build.RangeData{Items: newItems, Statics: itemStatics},
	}

	changes := &TreeNode{}
	if !handleStreamModeRange(oldTree, newTree, changes) {
		t.Fatalf("handleStreamModeRange did not dispatch to stream mode")
	}

	if changes.Range == nil {
		t.Fatalf("expected changes.Range; got: %+v", changes)
	}

	payload, err := json.Marshal(changes.Range.Items)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// The static-only branch's content MUST remain — it carries the branch
	// identity. Stripping it would make two static-only branches at the same
	// position indistinguishable.
	if !strings.Contains(string(payload), `"+"`) {
		t.Errorf("static-only branch statics were stripped, losing branch identity (regression vs PrepareTreeForClient special case)\n  payload=%s", payload)
	}
}

// extractDynamics mirrors parse.extractItemDynamics for test fixtures: items
// in a range carry their statics on the parent Range.Statics, not on each
// item TreeNode. Without this, the homogeneity check would compare two items
// with embedded Statics arrays as if they were the wire shape, which the
// diff path never sees in production.
func extractDynamics(item *TreeNode) *TreeNode {
	result := &TreeNode{AutoKey: item.AutoKey}
	if len(item.Dynamics) > 0 {
		result.Dynamics = make([]interface{}, len(item.Dynamics))
		copy(result.Dynamics, item.Dynamics)
	}
	return result
}
