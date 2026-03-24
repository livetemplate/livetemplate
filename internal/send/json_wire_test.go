package send

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/jsonutil"
)

func TestJSONIterWireFormat_HTMLNotEscaped(t *testing.T) {
	tree := &build.TreeNode{
		Statics:  []string{"<div class=\"counter\">", "</div>"},
		Dynamics: []interface{}{"<b>bold</b>"},
	}

	data, err := jsonutil.API.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := result["s"]; !ok {
		t.Error("Missing 's' key")
	}

	val, ok := result["0"]
	if !ok {
		t.Fatal("Missing '0' key")
	}
	if str, ok := val.(string); ok && !strings.Contains(str, "<b>") {
		t.Errorf("HTML was escaped: %s", str)
	}
}

func TestJSONIterWireFormat_NoEscapePlainMap(t *testing.T) {
	// NoEscape preserves literal HTML for plain maps (non-TreeNode types).
	// TreeNode types go through MarshalJSON which uses API.Marshal internally.
	data := map[string]string{"html": "<div>test</div>"}

	result, err := jsonutil.NoEscape.Marshal(data)
	if err != nil {
		t.Fatalf("NoEscape marshal failed: %v", err)
	}

	output := string(result)
	if strings.Contains(output, `\u003c`) {
		t.Errorf("NoEscape should not escape HTML entities: %s", output)
	}
	if !strings.Contains(output, "<div>") {
		t.Errorf("NoEscape should preserve literal HTML: %s", output)
	}
}

func TestJSONIterWireFormat_RoundTrip(t *testing.T) {
	original := &build.TreeNode{
		Statics:     []string{"<div>", "</div>"},
		Dynamics:    []interface{}{"value0", "value1"},
		AutoKey:     "key-123",
		Fingerprint: "fp-abc",
	}

	data, err := jsonutil.API.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored build.TreeNode
	if err := jsonutil.API.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.Dynamics[0] != "value0" || restored.Dynamics[1] != "value1" {
		t.Errorf("Dynamics mismatch: %v", restored.Dynamics)
	}
	if restored.AutoKey != "key-123" {
		t.Errorf("AutoKey mismatch: %s", restored.AutoKey)
	}
	if restored.Fingerprint != "fp-abc" {
		t.Errorf("Fingerprint mismatch: %s", restored.Fingerprint)
	}
}
