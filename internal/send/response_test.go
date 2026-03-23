package send

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPrepareUpdate_WithErrors tests preparing updates with validation errors.
func TestPrepareUpdate_WithErrors(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "test",
	}
	errors := map[string]string{
		"name":  "Name is required",
		"email": "Invalid email format",
	}
	action := "submit"

	resp := PrepareUpdate(tree, errors, action)

	if resp.Tree == nil {
		t.Error("Expected tree to be set")
	}

	if resp.Meta == nil {
		t.Fatal("Expected metadata to be set when errors present")
	}

	if resp.Meta.Success {
		t.Error("Expected success=false when errors present")
	}

	if len(resp.Meta.Errors) != 2 {
		t.Errorf("Expected 2 errors, got: %d", len(resp.Meta.Errors))
	}

	if resp.Meta.Errors["name"] != "Name is required" {
		t.Errorf("Expected name error, got: %v", resp.Meta.Errors["name"])
	}

	if resp.Meta.Action != "submit" {
		t.Errorf("Expected action 'submit', got: %q", resp.Meta.Action)
	}
}

// TestPrepareUpdate_NoErrors tests preparing updates without errors.
func TestPrepareUpdate_NoErrors(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "success",
	}
	action := "increment"

	resp := PrepareUpdate(tree, nil, action)

	if resp.Tree == nil {
		t.Error("Expected tree to be set")
	}

	if resp.Meta == nil {
		t.Fatal("Expected metadata to be set when action present")
	}

	if !resp.Meta.Success {
		t.Error("Expected success=true when no errors")
	}

	if len(resp.Meta.Errors) != 0 {
		t.Errorf("Expected 0 errors, got: %d", len(resp.Meta.Errors))
	}

	if resp.Meta.Action != "increment" {
		t.Errorf("Expected action 'increment', got: %q", resp.Meta.Action)
	}
}

// TestPrepareUpdate_EmptyErrors tests that empty error map doesn't add metadata.
func TestPrepareUpdate_EmptyErrors(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
	}
	errors := map[string]string{} // Empty but not nil

	resp := PrepareUpdate(tree, errors, "")

	if resp.Meta != nil {
		t.Error("Expected no metadata when errors map is empty and no action")
	}
}

// TestPrepareUpdate_OnlyAction tests metadata with only action, no errors.
func TestPrepareUpdate_OnlyAction(t *testing.T) {
	tree := map[string]interface{}{"0": "data"}
	action := "test-action"

	resp := PrepareUpdate(tree, nil, action)

	if resp.Meta == nil {
		t.Fatal("Expected metadata when action is present")
	}

	if resp.Meta.Action != action {
		t.Errorf("Expected action %q, got: %q", action, resp.Meta.Action)
	}

	if !resp.Meta.Success {
		t.Error("Expected success=true when no errors")
	}
}

// TestPrepareUpdate_NoMetadata tests that no metadata is added when neither errors nor action.
func TestPrepareUpdate_NoMetadata(t *testing.T) {
	tree := map[string]interface{}{"0": "simple"}

	resp := PrepareUpdate(tree, nil, "")

	if resp.Tree == nil {
		t.Error("Expected tree to be set")
	}

	if resp.Meta != nil {
		t.Error("Expected no metadata when no errors and no action")
	}
}

// TestSerializeUpdate_ValidTree tests serializing valid update responses.
func TestSerializeUpdate_ValidTree(t *testing.T) {
	resp := &UpdateResponse{
		Tree: map[string]interface{}{
			"s": []string{"<div>", "</div>"},
			"0": "content",
		},
		Meta: &ResponseMetadata{
			Success: true,
			Action:  "update",
		},
	}

	bytes, err := SerializeUpdate(resp)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(bytes) == 0 {
		t.Error("Expected non-empty serialized data")
	}

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Errorf("Expected valid JSON, got error: %v", err)
	}

	// Verify structure
	if decoded["tree"] == nil {
		t.Error("Expected 'tree' field in JSON")
	}

	if decoded["meta"] == nil {
		t.Error("Expected 'meta' field in JSON")
	}
}

// TestSerializeUpdate_NilMetadata tests serializing without metadata.
func TestSerializeUpdate_NilMetadata(t *testing.T) {
	resp := &UpdateResponse{
		Tree: map[string]interface{}{"0": "test"},
		Meta: nil,
	}

	bytes, err := SerializeUpdate(resp)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	// Meta should not be present in JSON when nil (omitempty tag)
	if decoded["meta"] != nil {
		t.Error("Expected 'meta' to be omitted when nil")
	}
}

// TestSerializeUpdate_UnmarshalableTree tests handling of unmarshalable data.
func TestSerializeUpdate_UnmarshalableTree(t *testing.T) {
	// Test with a channel (unmarshalable type) instead of cyclic map.
	// json-iterator doesn't detect cycles (stack overflow), so we use
	// a type that all JSON libraries reject cleanly.
	resp := &UpdateResponse{
		Tree: make(chan int),
	}

	_, err := SerializeUpdate(resp)
	if err == nil {
		t.Error("Expected error for unmarshalable tree")
	}

	if !strings.Contains(err.Error(), "failed to marshal") {
		t.Errorf("Expected 'failed to marshal' error, got: %v", err)
	}
}

// TestSerializeUpdate_ComplexTree tests serializing complex tree structures.
func TestSerializeUpdate_ComplexTree(t *testing.T) {
	resp := &UpdateResponse{
		Tree: map[string]interface{}{
			"s": []string{"<div>", "<span>", "</span>", "</div>"},
			"0": "title",
			"1": map[string]interface{}{
				"s": []string{"<p>", "</p>"},
				"0": "nested content",
			},
		},
		Meta: &ResponseMetadata{
			Success: true,
			Errors:  map[string]string{},
			Action:  "complex",
		},
	}

	bytes, err := SerializeUpdate(resp)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var decoded UpdateResponse
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	// Verify the nested structure survived serialization
	tree, ok := decoded.Tree.(map[string]interface{})
	if !ok {
		t.Fatal("Expected tree to be a map")
	}

	statics, ok := tree["s"].([]interface{})
	if !ok {
		t.Fatal("Expected statics to be an array")
	}

	if len(statics) != 4 {
		t.Errorf("Expected 4 static parts, got: %d", len(statics))
	}
}

// TestPrepareAndSerialize_Integration tests the convenience function.
func TestPrepareAndSerialize_Integration(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "test content",
	}
	errors := map[string]string{"field": "error message"}
	action := "validate"

	bytes, err := PrepareAndSerialize(tree, errors, action)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Deserialize and verify
	var resp UpdateResponse
	if err := json.Unmarshal(bytes, &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Tree == nil {
		t.Error("Expected tree to be set")
	}

	if resp.Meta == nil {
		t.Fatal("Expected metadata to be set")
	}

	if resp.Meta.Success {
		t.Error("Expected success=false when errors present")
	}

	if resp.Meta.Errors["field"] != "error message" {
		t.Errorf("Expected error message, got: %v", resp.Meta.Errors["field"])
	}

	if resp.Meta.Action != "validate" {
		t.Errorf("Expected action 'validate', got: %q", resp.Meta.Action)
	}
}

// TestPrepareAndSerialize_NoErrorsNoAction tests convenience function with minimal data.
func TestPrepareAndSerialize_NoErrorsNoAction(t *testing.T) {
	tree := map[string]interface{}{"0": "simple"}

	bytes, err := PrepareAndSerialize(tree, nil, "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var resp UpdateResponse
	if err := json.Unmarshal(bytes, &resp); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if resp.Tree == nil {
		t.Error("Expected tree to be set")
	}

	// Meta should be omitted when nil
	if resp.Meta != nil {
		t.Error("Expected metadata to be nil")
	}
}

// TestResponseMetadata_JSONMarshaling tests metadata JSON structure.
func TestResponseMetadata_JSONMarshaling(t *testing.T) {
	meta := &ResponseMetadata{
		Success: true,
		Errors: map[string]string{
			"email": "Invalid format",
		},
		Action: "submit",
	}

	bytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if decoded["success"] != true {
		t.Error("Expected success field")
	}

	if decoded["errors"] == nil {
		t.Error("Expected errors field")
	}

	if decoded["action"] != "submit" {
		t.Error("Expected action field")
	}
}

// TestResponseMetadata_OmitEmpty tests that empty action is omitted.
func TestResponseMetadata_OmitEmpty(t *testing.T) {
	meta := &ResponseMetadata{
		Success: false,
		Errors:  map[string]string{"field": "error"},
		Action:  "", // Empty action
	}

	bytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	jsonStr := string(bytes)
	if strings.Contains(jsonStr, `"action"`) {
		t.Error("Expected empty action to be omitted from JSON")
	}
}

// TestUpdateResponse_JSONStructure tests the complete response structure.
func TestUpdateResponse_JSONStructure(t *testing.T) {
	resp := &UpdateResponse{
		Tree: map[string]interface{}{
			"s": []string{"<div>", "</div>"},
			"0": "value",
		},
		Meta: &ResponseMetadata{
			Success: true,
			Errors:  map[string]string{},
			Action:  "test",
		},
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify JSON structure has both tree and meta at top level
	var decoded map[string]interface{}
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}

	if decoded["tree"] == nil {
		t.Error("Expected 'tree' at top level")
	}

	if decoded["meta"] == nil {
		t.Error("Expected 'meta' at top level")
	}

	// Should only have these two keys
	if len(decoded) != 2 {
		t.Errorf("Expected 2 top-level keys, got: %d", len(decoded))
	}
}
