package livetemplate

import (
	"strings"
	"testing"
)

// TestParseTemplateToTree_NestedConditionals tests that nested {{if}} constructs
// are properly recognized and extracted, not treated as static text.
func TestParseTemplateToTree_NestedConditionals(t *testing.T) {
	// This is the bug case from page mode: nested {{if}} conditionals
	templateStr := `<div>
  {{if .HasMore}}
    {{if .IsLoading}}
      <div>Loading more...</div>
    {{end}}
    <div id="sentinel"></div>
  {{end}}
</div>`

	// Sample data with both flags true
	data := map[string]interface{}{
		"HasMore":   true,
		"IsLoading": true,
	}

	keyGen := newKeyGenerator()
	tree, err := parseTemplateToTree(templateStr, data, keyGen)
	if err != nil {
		t.Fatalf("parseTemplateToTree failed: %v", err)
	}

	// Check that tree was generated successfully
	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	// The tree should have statics array
	statics, ok := tree["s"]
	if !ok {
		t.Fatal("Expected 's' key in tree")
	}

	// Convert statics to string for inspection
	staticsJSON := marshalToString(statics)

	// BUG CHECK: The statics should NOT contain raw template expressions
	// This is the bug we're fixing - currently {{if}} blocks appear as literal text
	if strings.Contains(staticsJSON, "{{if") {
		t.Errorf("BUG DETECTED: Raw {{if}} expressions found in statics array: %s", staticsJSON)
	}
	if strings.Contains(staticsJSON, "{{end}}") {
		t.Errorf("BUG DETECTED: Raw {{end}} expressions found in statics array: %s", staticsJSON)
	}

	// The rendered output should contain the actual content (not template expressions)
	// When both flags are true, we expect to see the loading div and sentinel
	t.Logf("Generated tree: %+v", tree)
}

// TestParseTemplateToTree_NestedConditionals_FalseFlags tests with false flags
func TestParseTemplateToTree_NestedConditionals_FalseFlags(t *testing.T) {
	templateStr := `<div>
  {{if .HasMore}}
    {{if .IsLoading}}
      <div>Loading more...</div>
    {{end}}
    <div id="sentinel"></div>
  {{end}}
</div>`

	// Sample data with HasMore false
	data := map[string]interface{}{
		"HasMore":   false,
		"IsLoading": false,
	}

	keyGen := newKeyGenerator()
	tree, err := parseTemplateToTree(templateStr, data, keyGen)
	if err != nil {
		t.Fatalf("parseTemplateToTree failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	// Check statics don't contain raw template expressions
	statics, ok := tree["s"]
	if !ok {
		t.Fatal("Expected 's' key in tree")
	}

	staticsJSON := marshalToString(statics)

	if strings.Contains(staticsJSON, "{{if") {
		t.Errorf("BUG DETECTED: Raw {{if}} expressions found in statics: %s", staticsJSON)
	}

	t.Logf("Generated tree: %+v", tree)
}

// Helper to marshal value to string for inspection
func marshalToString(v interface{}) string {
	bytes, _ := marshalValue(v)
	return string(bytes)
}

// TestExecuteUpdates_NestedConditionals tests the full flow including JSON serialization
// This mimics what happens during WebSocket message generation
func TestExecuteUpdates_NestedConditionals(t *testing.T) {
	// Create a template with nested conditionals similar to page mode
	templateStr := `<!DOCTYPE html>
<html>
<body>
<div class="container">
  {{if .HasMore}}
    {{if .IsLoading}}
      <div>Loading...</div>
    {{end}}
    <div id="sentinel"></div>
  {{end}}
</div>
</body>
</html>`

	tmpl := New("test")
	_, err := tmpl.Parse(templateStr)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Execute with data
	data := map[string]interface{}{
		"HasMore":   true,
		"IsLoading": true,
	}

	// This mimics what happens in WebSocket initial message
	var buf strings.Builder
	err = tmpl.ExecuteUpdates(&buf, data)
	if err != nil {
		t.Fatalf("ExecuteUpdates failed: %v", err)
	}

	treeJSON := buf.String()
	t.Logf("Tree JSON: %s", treeJSON)

	// BUG CHECK: The JSON should NOT contain raw template expressions
	if strings.Contains(treeJSON, "{{if") {
		t.Errorf("BUG DETECTED: Raw {{if}} in JSON: %s", treeJSON)
	}
	if strings.Contains(treeJSON, "{{end}}") {
		t.Errorf("BUG DETECTED: Raw {{end}} in JSON: %s", treeJSON)
	}
}
