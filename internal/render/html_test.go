package render

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestNode_TextNode tests text node rendering.
func TestNode_TextNode(t *testing.T) {
	node := &html.Node{
		Type: html.TextNode,
		Data: "Hello World",
	}

	var w strings.Builder
	Node(&w, node)

	expected := "Hello World"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestNode_ElementNode tests element node rendering.
func TestNode_ElementNode(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Data: "div",
	}

	// Add text child
	textChild := &html.Node{
		Type: html.TextNode,
		Data: "content",
	}
	node.AppendChild(textChild)

	var w strings.Builder
	Node(&w, node)

	expected := "<div>content</div>"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestNode_VoidElement tests void element rendering.
func TestNode_VoidElement(t *testing.T) {
	tests := []struct {
		tag      string
		expected string
	}{
		{"br", "<br>"},
		{"img", "<img>"},
		{"hr", "<hr>"},
		{"input", "<input>"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			node := &html.Node{
				Type: html.ElementNode,
				Data: tt.tag,
			}

			var w strings.Builder
			Node(&w, node)

			if w.String() != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, w.String())
			}
		})
	}
}

// TestNode_WithAttributes tests element with attributes.
func TestNode_WithAttributes(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{Key: "class", Val: "container"},
			{Key: "id", Val: "main"},
		},
	}

	var w strings.Builder
	Node(&w, node)

	result := w.String()
	if !strings.Contains(result, `class="container"`) {
		t.Error("Expected class attribute in output")
	}
	if !strings.Contains(result, `id="main"`) {
		t.Error("Expected id attribute in output")
	}
	if !strings.HasPrefix(result, "<div") {
		t.Error("Expected output to start with <div")
	}
	if !strings.HasSuffix(result, "</div>") {
		t.Error("Expected output to end with </div>")
	}
}

// TestNode_AttributeEscaping tests HTML escaping in attribute values.
func TestNode_AttributeEscaping(t *testing.T) {
	tests := []struct {
		name     string
		attrVal  string
		expected string
	}{
		{
			name:     "quote injection",
			attrVal:  `foo" onclick="alert('xss')`,
			expected: `<div class="foo&#34; onclick=&#34;alert(&#39;xss&#39;)"></div>`,
		},
		{
			name:     "ampersand",
			attrVal:  "Tom & Jerry",
			expected: `<div class="Tom &amp; Jerry"></div>`,
		},
		{
			name:     "less than",
			attrVal:  "a < b",
			expected: `<div class="a &lt; b"></div>`,
		},
		{
			name:     "greater than",
			attrVal:  "a > b",
			expected: `<div class="a &gt; b"></div>`,
		},
		{
			name:     "single quote",
			attrVal:  "it's working",
			expected: `<div class="it&#39;s working"></div>`,
		},
		{
			name:     "multiple special chars",
			attrVal:  `"Tom & Jerry" <script>`,
			expected: `<div class="&#34;Tom &amp; Jerry&#34; &lt;script&gt;"></div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &html.Node{
				Type: html.ElementNode,
				Data: "div",
				Attr: []html.Attribute{
					{Key: "class", Val: tt.attrVal},
				},
			}

			var w strings.Builder
			Node(&w, node)

			if w.String() != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, w.String())
			}
		})
	}
}

// TestNode_NestedElements tests nested element structures.
func TestNode_NestedElements(t *testing.T) {
	// Create: <div><span>text</span></div>
	root := &html.Node{
		Type: html.ElementNode,
		Data: "div",
	}

	span := &html.Node{
		Type: html.ElementNode,
		Data: "span",
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: "text",
	}

	span.AppendChild(text)
	root.AppendChild(span)

	var w strings.Builder
	Node(&w, root)

	expected := "<div><span>text</span></div>"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestIsVoidElement_AllVoid tests all void elements are recognized.
func TestIsVoidElement_AllVoid(t *testing.T) {
	voidElements := []string{
		"area", "base", "br", "col", "embed", "hr", "img",
		"input", "link", "meta", "param", "source", "track", "wbr",
	}

	for _, elem := range voidElements {
		t.Run(elem, func(t *testing.T) {
			if !IsVoidElement(elem) {
				t.Errorf("Expected %q to be recognized as void element", elem)
			}
		})
	}
}

// TestIsVoidElement_NonVoid tests non-void elements.
func TestIsVoidElement_NonVoid(t *testing.T) {
	nonVoidElements := []string{
		"div", "span", "p", "a", "button", "section", "article",
		"header", "footer", "nav", "main", "h1", "ul", "li",
	}

	for _, elem := range nonVoidElements {
		t.Run(elem, func(t *testing.T) {
			if IsVoidElement(elem) {
				t.Errorf("Expected %q to NOT be recognized as void element", elem)
			}
		})
	}
}

// TestTreeToHTML_Simple tests simple tree to HTML rendering.
func TestTreeToHTML_Simple(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := "<div>content</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestTreeToHTML_WithDynamics tests tree with multiple dynamic values.
func TestTreeToHTML_WithDynamics(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", " - ", "</div>"},
		"0": "Hello",
		"1": "World",
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := "<div>Hello - World</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestTreeToHTML_Nested tests nested tree structures.
func TestTreeToHTML_Nested(t *testing.T) {
	nestedTree := map[string]interface{}{
		"s": []string{"<span>", "</span>"},
		"0": "nested",
	}

	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": nestedTree,
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := "<div><span>nested</span></div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestTreeToHTML_Error tests error handling.
func TestTreeToHTML_Error(t *testing.T) {
	tests := []struct {
		name string
		tree map[string]interface{}
	}{
		{
			"no statics",
			map[string]interface{}{
				"0": "value",
			},
		},
		{
			"empty statics",
			map[string]interface{}{
				"s": []string{},
				"0": "value",
			},
		},
		{
			"invalid statics type",
			map[string]interface{}{
				"s": "not-a-slice",
				"0": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TreeToHTML(tt.tree)
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

// TestRangeComprehensionToHTML tests range rendering.
func TestRangeComprehensionToHTML(t *testing.T) {
	// Range tree structure
	tree := map[string]interface{}{
		"s": []string{"<div id=\"", "\">", "</div>"},
		"d": []interface{}{
			map[string]interface{}{
				"0": "id1",
				"1": "Item 1",
			},
			map[string]interface{}{
				"0": "id2",
				"1": "Item 2",
			},
		},
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := `<div id="id1">Item 1</div><div id="id2">Item 2</div>`
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRangeComprehensionToHTML_Empty tests empty range rendering.
func TestRangeComprehensionToHTML_Empty(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"d": []interface{}{},
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	// Empty range should produce empty output
	if html != "" {
		t.Errorf("Expected empty string, got: %q", html)
	}
}

// TestRangeComprehensionToHTML_NestedTrees tests range with nested trees.
func TestRangeComprehensionToHTML_NestedTrees(t *testing.T) {
	nestedTree := map[string]interface{}{
		"s": []string{"<span>", "</span>"},
		"0": "nested",
	}

	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"d": []interface{}{
			map[string]interface{}{
				"0": nestedTree,
			},
		},
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := "<div><span>nested</span></div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestIsVoidElement_CaseInsensitive tests case-insensitive void element recognition.
func TestIsVoidElement_CaseInsensitive(t *testing.T) {
	tests := []struct {
		tag      string
		expected bool
	}{
		{"br", true},
		{"BR", true},
		{"Br", true},
		{"bR", true},
		{"img", true},
		{"IMG", true},
		{"Img", true},
		{"input", true},
		{"INPUT", true},
		{"Input", true},
		{"div", false},
		{"DIV", false},
		{"Div", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			result := IsVoidElement(tt.tag)
			if result != tt.expected {
				t.Errorf("IsVoidElement(%q) = %v, expected %v", tt.tag, result, tt.expected)
			}
		})
	}
}

// TestTreeToHTML_HTMLEscaping tests HTML escaping in dynamic values.
func TestTreeToHTML_HTMLEscaping(t *testing.T) {
	tests := []struct {
		name     string
		tree     map[string]interface{}
		expected string
	}{
		{
			name: "script tag",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": "<script>alert('xss')</script>",
			},
			expected: "<div>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</div>",
		},
		{
			name: "ampersand",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": "Tom & Jerry",
			},
			expected: "<div>Tom &amp; Jerry</div>",
		},
		{
			name: "angle brackets",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": "5 < 10 > 3",
			},
			expected: "<div>5 &lt; 10 &gt; 3</div>",
		},
		{
			name: "quotes",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"0": `He said "hello"`,
			},
			expected: `<div>He said &#34;hello&#34;</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := TreeToHTML(tt.tree)
			if err != nil {
				t.Fatalf("TreeToHTML failed: %v", err)
			}

			if html != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, html)
			}
		})
	}
}

// TestRangeComprehensionToHTML_HTMLEscaping tests HTML escaping in range items.
func TestRangeComprehensionToHTML_HTMLEscaping(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"d": []interface{}{
			map[string]interface{}{
				"0": "<script>alert('xss')</script>",
			},
			map[string]interface{}{
				"0": "Tom & Jerry",
			},
		},
	}

	html, err := TreeToHTML(tree)
	if err != nil {
		t.Fatalf("TreeToHTML failed: %v", err)
	}

	expected := "<div>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</div><div>Tom &amp; Jerry</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRangeComprehensionToHTML_InvalidItem tests error handling for invalid range items.
func TestRangeComprehensionToHTML_InvalidItem(t *testing.T) {
	tests := []struct {
		name string
		tree map[string]interface{}
	}{
		{
			name: "item is not a map",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"d": []interface{}{
					"not a map",
				},
			},
		},
		{
			name: "item is number",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"d": []interface{}{
					42,
				},
			},
		},
		{
			name: "item is nil",
			tree: map[string]interface{}{
				"s": []string{"<div>", "</div>"},
				"d": []interface{}{
					nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TreeToHTML(tt.tree)
			if err == nil {
				t.Error("Expected error for invalid range item, got nil")
			}
			if err != nil && !contains(err.Error(), "item") {
				t.Errorf("Expected error message to mention 'item', got: %v", err)
			}
		})
	}
}

// TestTreeToHTML_NestedTreeError tests error handling in nested tree rendering.
func TestTreeToHTML_NestedTreeError(t *testing.T) {
	// Nested tree with invalid structure (no statics)
	invalidNestedTree := map[string]interface{}{
		"0": "value",
	}

	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": invalidNestedTree,
	}

	_, err := TreeToHTML(tree)
	if err == nil {
		t.Error("Expected error for invalid nested tree, got nil")
	}
	if err != nil && !contains(err.Error(), "position 0") {
		t.Errorf("Expected error message to mention position, got: %v", err)
	}
}

// TestRangeComprehensionToHTML_MissingStatics tests error when statics are missing.
func TestRangeComprehensionToHTML_MissingStatics(t *testing.T) {
	tree := map[string]interface{}{
		"d": []interface{}{
			map[string]interface{}{
				"0": "value",
			},
		},
	}

	_, err := TreeToHTML(tree)
	if err == nil {
		t.Error("Expected error for missing statics, got nil")
	}
	if err != nil && !contains(err.Error(), "missing statics") {
		t.Errorf("Expected error message to mention 'missing statics', got: %v", err)
	}
}

// contains checks if a string contains a substring (case-insensitive helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
