package livetemplate_test

import (
	"encoding/json"
	"testing"

	"github.com/livetemplate/livetemplate"
)

// TestPublicAPI_NewTreeNode verifies the public API for creating empty TreeNodes.
func TestPublicAPI_NewTreeNode(t *testing.T) {
	node := livetemplate.NewTreeNode()
	if node == nil {
		t.Fatal("NewTreeNode() returned nil")
	}

	// Verify node is properly initialized
	if node.Statics != nil {
		t.Error("Expected Statics to be nil for empty node")
	}
	if node.Dynamics == nil {
		t.Error("Expected Dynamics to be initialized (non-nil)")
	}
	if len(node.Dynamics) != 0 {
		t.Errorf("Expected empty Dynamics, got %d entries", len(node.Dynamics))
	}
}

// TestPublicAPI_NewTreeNodeWithStatics verifies the public API for creating TreeNodes with statics.
func TestPublicAPI_NewTreeNodeWithStatics(t *testing.T) {
	statics := []string{"<div>", "</div>"}
	node := livetemplate.NewTreeNodeWithStatics(statics)

	if node == nil {
		t.Fatal("NewTreeNodeWithStatics() returned nil")
	}
	if node.Statics == nil {
		t.Fatal("Expected Statics to be set")
	}
	if len(node.Statics) != 2 {
		t.Errorf("Expected 2 statics, got %d", len(node.Statics))
	}
	if node.Statics[0] != "<div>" || node.Statics[1] != "</div>" {
		t.Errorf("Statics content mismatch: got %v", node.Statics)
	}
}

// TestPublicAPI_NewRangeData verifies the public API for creating RangeData.
func TestPublicAPI_NewRangeData(t *testing.T) {
	items := []interface{}{
		livetemplate.NewTreeNode(),
		livetemplate.NewTreeNode(),
	}
	statics := []string{"<li>", "</li>"}

	rangeData := livetemplate.NewRangeData(items, statics)

	if rangeData == nil {
		t.Fatal("NewRangeData() returned nil")
	}
	if len(rangeData.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(rangeData.Items))
	}
	if len(rangeData.Statics) != 2 {
		t.Errorf("Expected 2 statics, got %d", len(rangeData.Statics))
	}
}

// TestPublicAPI_NewTreeMetadata verifies the public API for creating TreeMetadata.
func TestPublicAPI_NewTreeMetadata(t *testing.T) {
	idKey := "item-123"
	metadata := livetemplate.NewTreeMetadata(idKey)

	if metadata == nil {
		t.Fatal("NewTreeMetadata() returned nil")
	}
	if metadata.IDKey != idKey {
		t.Errorf("Expected IDKey %q, got %q", idKey, metadata.IDKey)
	}
}

// TestPublicAPI_NewUpdateContext verifies the public API for creating update contexts.
func TestPublicAPI_NewUpdateContext(t *testing.T) {
	clientStructures := make(map[string]bool)
	ctx := livetemplate.NewUpdateContext(clientStructures)

	if ctx == nil {
		t.Fatal("NewUpdateContext() returned nil")
	}
	if ctx.IsFirstRender {
		t.Error("Expected IsFirstRender to be false for update context")
	}
	if ctx.IncludeStatics {
		t.Error("Expected IncludeStatics to be false for update context")
	}
}

// TestPublicAPI_FromMap verifies the public API for creating TreeNodes from maps.
func TestPublicAPI_FromMap(t *testing.T) {
	testMap := map[string]interface{}{
		"s": []interface{}{"<div>", "</div>"},
		"0": "dynamic content",
	}

	node, err := livetemplate.FromMap(testMap)
	if err != nil {
		t.Fatalf("FromMap() returned error: %v", err)
	}

	if node == nil {
		t.Fatal("FromMap() returned nil")
	}
	if len(node.Statics) != 2 {
		t.Errorf("Expected 2 statics, got %d", len(node.Statics))
	}
	if node.Dynamics["0"] != "dynamic content" {
		t.Errorf("Expected dynamic value %q, got %v", "dynamic content", node.Dynamics["0"])
	}
}

// TestPublicAPI_TreeNodeJSONRoundtrip verifies JSON marshaling/unmarshaling through public API.
func TestPublicAPI_TreeNodeJSONRoundtrip(t *testing.T) {
	// Create a node with all fields populated
	original := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	original.SetDynamic("0", "test content")
	original.SetDynamic("1", 42)
	original.Fingerprint = "test-fingerprint"

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var restored livetemplate.TreeNode
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify roundtrip
	if len(restored.Statics) != len(original.Statics) {
		t.Errorf("Statics length mismatch: got %d, want %d", len(restored.Statics), len(original.Statics))
	}
	restoredVal, _ := restored.GetDynamic("0")
	originalVal, _ := original.GetDynamic("0")
	if restoredVal != originalVal {
		t.Errorf("Dynamic[0] mismatch: got %v, want %v", restoredVal, originalVal)
	}
	if restored.Fingerprint != original.Fingerprint {
		t.Errorf("Fingerprint mismatch: got %q, want %q", restored.Fingerprint, original.Fingerprint)
	}
}

// TestPublicAPI_TreeNodeSetGetDynamic verifies dynamic value operations.
func TestPublicAPI_TreeNodeSetGetDynamic(t *testing.T) {
	node := livetemplate.NewTreeNode()

	// Set various types
	node.SetDynamic("0", "string value")
	node.SetDynamic("1", 123)
	node.SetDynamic("2", true)

	// Get and verify
	if got, ok := node.GetDynamic("0"); !ok || got != "string value" {
		t.Errorf("GetDynamic(0) = %v, %v, want %q, true", got, ok, "string value")
	}
	if got, ok := node.GetDynamic("1"); !ok || got != 123 {
		t.Errorf("GetDynamic(1) = %v, %v, want %d, true", got, ok, 123)
	}
	if got, ok := node.GetDynamic("2"); !ok || got != true {
		t.Errorf("GetDynamic(2) = %v, %v, want %v, true", got, ok, true)
	}

	// Non-existent key
	if got, ok := node.GetDynamic("999"); ok || got != nil {
		t.Errorf("GetDynamic(999) = %v, %v, want nil, false", got, ok)
	}
}

// TestPublicAPI_TreeNodeHasStatics verifies the HasStatics predicate.
func TestPublicAPI_TreeNodeHasStatics(t *testing.T) {
	nodeWithStatics := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	nodeWithoutStatics := livetemplate.NewTreeNode()

	if !nodeWithStatics.HasStatics() {
		t.Error("Expected HasStatics() = true for node with statics")
	}
	if nodeWithoutStatics.HasStatics() {
		t.Error("Expected HasStatics() = false for node without statics")
	}
}

// TestPublicAPI_TreeNodeHasDynamics verifies the HasDynamics predicate.
func TestPublicAPI_TreeNodeHasDynamics(t *testing.T) {
	node := livetemplate.NewTreeNode()

	if node.HasDynamics() {
		t.Error("Expected HasDynamics() = false for empty node")
	}

	node.SetDynamic("0", "value")

	if !node.HasDynamics() {
		t.Error("Expected HasDynamics() = true after setting dynamic")
	}
}

// TestPublicAPI_TreeNodeClone verifies deep cloning functionality.
func TestPublicAPI_TreeNodeClone(t *testing.T) {
	original := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	original.SetDynamic("0", "test")
	original.Fingerprint = "original-fingerprint"

	clone := original.Clone()

	// Verify deep copy
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if len(clone.Statics) != len(original.Statics) {
		t.Error("Statics not cloned correctly")
	}
	cloneVal, _ := clone.GetDynamic("0")
	originalVal, _ := original.GetDynamic("0")
	if cloneVal != originalVal {
		t.Error("Dynamics not cloned correctly")
	}

	// Verify independence
	clone.SetDynamic("0", "modified")
	originalVal2, _ := original.GetDynamic("0")
	if originalVal2 == "modified" {
		t.Error("Clone is not independent - modifying clone affected original")
	}
}

// TestPublicAPI_TreeNodeToMap verifies conversion to map.
func TestPublicAPI_TreeNodeToMap(t *testing.T) {
	node := livetemplate.NewTreeNodeWithStatics([]string{"<div>", "</div>"})
	node.SetDynamic("0", "content")

	m := node.ToMap()

	if m == nil {
		t.Fatal("ToMap() returned nil")
	}

	// Verify statics
	statics, ok := m["s"].([]string)
	if !ok {
		t.Fatal("Statics not found or wrong type in map")
	}
	if len(statics) != 2 {
		t.Errorf("Expected 2 statics in map, got %d", len(statics))
	}

	// Verify dynamics
	if m["0"] != "content" {
		t.Errorf("Dynamic value mismatch in map: got %v", m["0"])
	}
}

// TestPublicAPI_TypeAliasConsistency verifies that type aliases behave identically to source types.
func TestPublicAPI_TypeAliasConsistency(t *testing.T) {
	// This test ensures the public API types are true aliases, not separate types
	node := livetemplate.NewTreeNode()

	// Should be able to assign to interface{} and back
	var i interface{} = node
	_, ok := i.(*livetemplate.TreeNode)
	if !ok {
		t.Error("TreeNode type alias does not work correctly")
	}
}

// TestPublicAPI_NilHandling verifies proper nil handling in public API.
func TestPublicAPI_NilHandling(t *testing.T) {
	// FromMap with nil should not panic
	node, err := livetemplate.FromMap(nil)
	if err != nil {
		t.Errorf("FromMap(nil) returned error: %v", err)
	}
	if node == nil {
		t.Error("FromMap(nil) should return empty node, not nil")
	}

	// NewRangeData with nil items should not panic
	rangeData := livetemplate.NewRangeData(nil, nil)
	if rangeData == nil {
		t.Error("NewRangeData(nil, nil) should return empty RangeData, not nil")
	}
}
