package diff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Stream-mode INSERTs must carry full nested statics; client renders new rows without a branch-statics cache.
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
		t.Errorf("nested marker statics missing from insert payload; client cannot render new rows\n  payload=%s", payload)
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

// Static-only conditional branches ({{if .Add}}+{{end}}) carry identity in their statics — never strip them.
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

// PrepareTreeForClient must preserve AutoKey when stripping statics so the client can track new rows by key.
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
		t.Errorf("auto-key (_k) missing from stream-mode insert; client cannot track new row\n  payload=%s", payload)
	}
}

// extractDynamics mirrors parse.extractItemDynamics: range items carry statics on parent Range.Statics, not per-item.
func extractDynamics(item *TreeNode) *TreeNode {
	result := &TreeNode{AutoKey: item.AutoKey}
	if len(item.Dynamics) > 0 {
		result.Dynamics = make([]interface{}, len(item.Dynamics))
		copy(result.Dynamics, item.Dynamics)
	}
	return result
}
