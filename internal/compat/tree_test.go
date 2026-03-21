package compat

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// TestNewKeyGenerator verifies the wrapper creates a valid key generator
func TestNewKeyGenerator(t *testing.T) {
	kg := NewKeyGenerator()
	if kg == nil {
		t.Fatal("NewKeyGenerator returned nil")
	}

	// Verify it can generate keys
	key, err := kg.NextKey()
	if err != nil {
		t.Fatalf("NextKey failed: %v", err)
	}
	if key == "" {
		t.Error("NextKey returned empty string")
	}
}

// TestGenerateRandomID verifies random ID generation
func TestGenerateRandomID(t *testing.T) {
	id1 := GenerateRandomID()
	if id1 == "" {
		t.Error("GenerateRandomID returned empty string")
	}

	id2 := GenerateRandomID()
	if id1 == id2 {
		t.Error("GenerateRandomID returned same ID twice (should be random)")
	}
}

// TestInjectWrapperDiv verifies wrapper div injection
func TestInjectWrapperDiv(t *testing.T) {
	html := "<html><body><div>test</div></body></html>"
	wrapperID := "test-wrapper"

	result := InjectWrapperDiv(html, wrapperID, false)
	if result == html {
		t.Error("InjectWrapperDiv did not modify HTML")
	}
	if result == "" {
		t.Error("InjectWrapperDiv returned empty string")
	}
}

// TestExtractTemplateBodyContent verifies body content extraction
func TestExtractTemplateBodyContent(t *testing.T) {
	tmpl := "<html><body><div>test</div></body></html>"

	result := ExtractTemplateBodyContent(tmpl)
	if result == "" {
		t.Error("ExtractTemplateBodyContent returned empty string")
	}
}

// TestExtractTemplateContent verifies content extraction
func TestExtractTemplateContent(t *testing.T) {
	html := `<div id="wrapper"><span>test</span></div>`
	wrapperID := "wrapper"

	result := ExtractTemplateContent(html, wrapperID)
	if result == "" {
		t.Error("ExtractTemplateContent returned empty string")
	}
}

// TestNormalizeTemplateSpacing verifies spacing normalization
func TestNormalizeTemplateSpacing(t *testing.T) {
	tmpl := "{{ range .Items }}test{{ end }}"

	result := NormalizeTemplateSpacing(tmpl)
	if result == "" {
		t.Error("NormalizeTemplateSpacing returned empty string")
	}
}

// TestRenderTreeToHTML verifies tree to HTML rendering
func TestRenderTreeToHTML(t *testing.T) {
	// Create a proper TreeNode and convert to map
	tree := build.NewTreeNode()
	tree.Statics = []string{"<div>", "</div>"}
	tree.SetDynamic(0, "test")
	treeMap := tree.ToMap()

	html, err := RenderTreeToHTML(treeMap)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}
	if html == "" {
		t.Error("RenderTreeToHTML returned empty string")
	}
	if html != "<div>test</div>" {
		t.Errorf("Expected '<div>test</div>', got %q", html)
	}
}

// TestParseTemplateToTree verifies template parsing
func TestParseTemplateToTree(t *testing.T) {
	tmpl := "<div>{{.Value}}</div>"
	data := map[string]interface{}{"Value": "test"}
	kg := NewKeyGenerator()

	tree, err := ParseTemplateToTree("test", tmpl, data, kg)
	if err != nil {
		t.Fatalf("ParseTemplateToTree failed: %v", err)
	}
	if tree == nil {
		t.Fatal("ParseTemplateToTree returned nil tree")
	}
	if !tree.HasStatics() {
		t.Error("ParseTemplateToTree did not generate statics")
	}
}

// TestDetectIDKey verifies ID key detection
func TestDetectIDKey(t *testing.T) {
	statics := []string{"<div id=\"{{.ID}}\">", "</div>"}

	key := DetectIDKey(statics)
	// Should detect "ID" or return empty if not found
	// This is a smoke test - actual behavior is implementation-dependent
	_ = key // Just verify it doesn't panic
}

// TestGenerateWrapperKey verifies wrapper key generation
func TestGenerateWrapperKey(t *testing.T) {
	kg := NewKeyGenerator()

	key := GenerateWrapperKey(kg)
	if key == "" {
		t.Error("GenerateWrapperKey returned empty string")
	}

	// Verify sequential keys
	key2 := GenerateWrapperKey(kg)
	if key == key2 {
		t.Error("GenerateWrapperKey returned same key twice")
	}
}
