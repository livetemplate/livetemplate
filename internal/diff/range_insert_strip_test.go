package diff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestStreamInsert_PreservesNestedDynamicBranchStatics — issue #413.
//
// The original symmetry attempt (stripping nested item statics in stream-mode
// INSERTs to mirror the UPDATE path) broke client rendering. Updates patch
// existing DOM in place, so the client only needs new dynamic values. Inserts
// have to RENDER new DOM, and the client has no per-position branch-statics
// cache — when it sees an item like {"0": {"0": "marker text"}} with no "s"
// on the nested object, it joins values with empty string and loses the
// HTML wrapper (`<span class="marker">…</span>`). Downstream regressions:
// examples/flash-messages and tinkerdown auto-tables.
//
// This test locks in the contract that stream-mode INSERT payloads carry
// the full per-item nested branch statics so the client can reconstruct
// every new row's HTML structure.
func TestStreamInsert_PreservesNestedDynamicBranchStatics(t *testing.T) {
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	makeItem := func(key, markerText string) *TreeNode {
		marker := &TreeNode{
			Statics:  []string{"<m>", "</m>"},
			Dynamics: []interface{}{markerText},
		}
		return &TreeNode{
			Statics:  itemStatics,
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

	newItems := []interface{}{
		extractDynamics(makeItem("n1", "new-marker-1")),
		extractDynamics(makeItem("n2", "new-marker-2")),
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
		t.Fatalf("expected changes.Range to be populated; got: %+v", changes)
	}

	payload, err := json.Marshal(changes.Range.Items)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// json.Marshal HTML-escapes `<` to `<` and `>` to `>` by
	// default. Check the "s":[...] envelope (proves the nested statics
	// were not stripped) and the actual on-wire marker content.
	if !strings.Contains(string(payload), `"s":[`) {
		t.Errorf("nested marker statics missing from insert payload; client cannot render new rows (issue #413)\n  payload=%s", payload)
	}
	if !strings.Contains(string(payload), `\u003cm\u003e`) {
		t.Errorf("nested marker static content (<m>) missing from insert payload\n  payload=%s", payload)
	}

	// The new dynamic content MUST still arrive.
	if !strings.Contains(string(payload), "new-marker-1") || !strings.Contains(string(payload), "new-marker-2") {
		t.Errorf("new-side dynamic content missing from insert payload\n  payload=%s", payload)
	}

	// Item keys must arrive so the client can track the new rows.
	if !strings.Contains(string(payload), `"n1"`) || !strings.Contains(string(payload), `"n2"`) {
		t.Errorf("new-side item keys missing from insert payload\n  payload=%s", payload)
	}
}

// TestStreamInsert_PreservesStaticOnlyBranchIdentity — companion test. For
// static-only conditional branches (e.g. `{{if .Add}}+{{end}}`) the branch's
// statics ARE the branch identity. The PrepareTreeForClient special case for
// static-only branches preserved this even when the (now-reverted) strip-on-
// insert path was active. Keeping the test guards against future regressions
// in that special case.
func TestStreamInsert_PreservesStaticOnlyBranchIdentity(t *testing.T) {
	itemStatics := []string{`<li data-key="`, `">`, `</li>`}
	makeItem := func(key, branchStatic string) *TreeNode {
		branch := &TreeNode{Statics: []string{branchStatic}}
		return &TreeNode{
			Statics:  itemStatics,
			Dynamics: []interface{}{key, branch},
		}
	}

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

	if !strings.Contains(string(payload), `"+"`) {
		t.Errorf("static-only branch statics were stripped, losing branch identity (regression vs PrepareTreeForClient special case)\n  payload=%s", payload)
	}
}

// TestStreamInsert_PreservesAutoKey — regression test for the
// examples/flash-messages bug. Items in a `{{range}}` without an explicit
// data-key get an auto-generated `_k` field via build.TreeNode.AutoKey.
// PrepareTreeForClient was dropping AutoKey when stripping statics, so
// stream-mode inserts arrived at the client without `_k` and the client
// could not track the new row by key.
func TestStreamInsert_PreservesAutoKey(t *testing.T) {
	makeItem := func(autoKey, val string) *TreeNode {
		return &TreeNode{
			AutoKey:  autoKey,
			Dynamics: []interface{}{val},
		}
	}

	itemStatics := []string{"<li>", "</li>"}
	oldItems := []interface{}{
		makeItem("hash-a", "Apple"),
		makeItem("hash-b", "Banana"),
	}
	oldTree := &TreeNode{
		Statics:  itemStatics,
		Range:    &build.RangeData{Items: oldItems, Statics: itemStatics},
		Metadata: &build.TreeMetadata{IDKey: "_k"},
	}
	TransitionToStreamMode(oldTree)
	if oldTree.Range.StreamState == nil {
		t.Fatalf("TransitionToStreamMode left StreamState nil — fixture lost homogeneity?")
	}

	newItems := []interface{}{
		makeItem("hash-a", "Apple"),
		makeItem("hash-b", "Banana"),
		makeItem("hash-c", "Cherry"),
	}
	newTree := &TreeNode{
		Statics:  itemStatics,
		Range:    &build.RangeData{Items: newItems, Statics: itemStatics},
		Metadata: &build.TreeMetadata{IDKey: "_k"},
	}

	changes := &TreeNode{}
	if !handleStreamModeRange(oldTree, newTree, changes) {
		t.Fatalf("handleStreamModeRange did not dispatch to stream mode")
	}

	payload, err := json.Marshal(changes.Range.Items)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	if !strings.Contains(string(payload), `"_k":"hash-c"`) {
		t.Errorf("auto-key (_k) missing from stream-mode insert; client cannot track new row (issue #413)\n  payload=%s", payload)
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
