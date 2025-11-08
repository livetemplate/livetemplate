package build

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestRenderNode_TextNode tests text node rendering.
func TestRenderNode_TextNode(t *testing.T) {
	node := &html.Node{
		Type: html.TextNode,
		Data: "Hello World",
	}

	var w strings.Builder
	RenderNode(&w, node)

	expected := "Hello World"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestRenderNode_ElementNode tests element node rendering.
func TestRenderNode_ElementNode(t *testing.T) {
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
	RenderNode(&w, node)

	expected := "<div>content</div>"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestRenderNode_VoidElement tests void element rendering.
func TestRenderNode_VoidElement(t *testing.T) {
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
			RenderNode(&w, node)

			if w.String() != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, w.String())
			}
		})
	}
}

// TestRenderNode_WithAttributes tests element with attributes.
func TestRenderNode_WithAttributes(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{Key: "class", Val: "container"},
			{Key: "id", Val: "main"},
		},
	}

	var w strings.Builder
	RenderNode(&w, node)

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

// TestRenderNode_AttributeEscaping tests HTML escaping in attribute values.
func TestRenderNode_AttributeEscaping(t *testing.T) {
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
			RenderNode(&w, node)

			if w.String() != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, w.String())
			}
		})
	}
}

// TestRenderNode_NestedElements tests nested element structures.
func TestRenderNode_NestedElements(t *testing.T) {
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
	RenderNode(&w, root)

	expected := "<div><span>text</span></div>"
	if w.String() != expected {
		t.Errorf("Expected %q, got: %q", expected, w.String())
	}
}

// TestIsVoidHTMLElement_AllVoid tests all void elements are recognized.
func TestIsVoidHTMLElement_AllVoid(t *testing.T) {
	voidElements := []string{
		"area", "base", "br", "col", "embed", "hr", "img",
		"input", "link", "meta", "param", "source", "track", "wbr",
	}

	for _, elem := range voidElements {
		t.Run(elem, func(t *testing.T) {
			if !IsVoidHTMLElement(elem) {
				t.Errorf("Expected %q to be recognized as void element", elem)
			}
		})
	}
}

// TestIsVoidHTMLElement_NonVoid tests non-void elements.
func TestIsVoidHTMLElement_NonVoid(t *testing.T) {
	nonVoidElements := []string{
		"div", "span", "p", "a", "button", "section", "article",
		"header", "footer", "nav", "main", "h1", "ul", "li",
	}

	for _, elem := range nonVoidElements {
		t.Run(elem, func(t *testing.T) {
			if IsVoidHTMLElement(elem) {
				t.Errorf("Expected %q to NOT be recognized as void element", elem)
			}
		})
	}
}

// TestRenderTreeToHTML_Simple tests simple tree to HTML rendering.
func TestRenderTreeToHTML_Simple(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := "<div>content</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRenderTreeToHTML_WithDynamics tests tree with multiple dynamic values.
func TestRenderTreeToHTML_WithDynamics(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", " - ", "</div>"},
		"0": "Hello",
		"1": "World",
	}

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := "<div>Hello - World</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRenderTreeToHTML_Nested tests nested tree structures.
func TestRenderTreeToHTML_Nested(t *testing.T) {
	nestedTree := map[string]interface{}{
		"s": []string{"<span>", "</span>"},
		"0": "nested",
	}

	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": nestedTree,
	}

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := "<div><span>nested</span></div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRenderTreeToHTML_Error tests error handling.
func TestRenderTreeToHTML_Error(t *testing.T) {
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
			_, err := RenderTreeToHTML(tt.tree)
			if err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

// TestRenderRangeComprehensionToHTML tests range rendering.
func TestRenderRangeComprehensionToHTML(t *testing.T) {
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

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := `<div id="id1">Item 1</div><div id="id2">Item 2</div>`
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestRenderRangeComprehensionToHTML_Empty tests empty range rendering.
func TestRenderRangeComprehensionToHTML_Empty(t *testing.T) {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"d": []interface{}{},
	}

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	// Empty range should produce empty output
	if html != "" {
		t.Errorf("Expected empty string, got: %q", html)
	}
}

// TestRenderRangeComprehensionToHTML_NestedTrees tests range with nested trees.
func TestRenderRangeComprehensionToHTML_NestedTrees(t *testing.T) {
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

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := "<div><span>nested</span></div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}

// TestIsVoidHTMLElement_CaseInsensitive tests case-insensitive void element recognition.
func TestIsVoidHTMLElement_CaseInsensitive(t *testing.T) {
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
			result := IsVoidHTMLElement(tt.tag)
			if result != tt.expected {
				t.Errorf("IsVoidHTMLElement(%q) = %v, expected %v", tt.tag, result, tt.expected)
			}
		})
	}
}

// TestRenderTreeToHTML_HTMLEscaping tests HTML escaping in dynamic values.
func TestRenderTreeToHTML_HTMLEscaping(t *testing.T) {
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
			html, err := RenderTreeToHTML(tt.tree)
			if err != nil {
				t.Fatalf("RenderTreeToHTML failed: %v", err)
			}

			if html != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, html)
			}
		})
	}
}

// TestRenderRangeComprehensionToHTML_HTMLEscaping tests HTML escaping in range items.
func TestRenderRangeComprehensionToHTML_HTMLEscaping(t *testing.T) {
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

	html, err := RenderTreeToHTML(tree)
	if err != nil {
		t.Fatalf("RenderTreeToHTML failed: %v", err)
	}

	expected := "<div>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</div><div>Tom &amp; Jerry</div>"
	if html != expected {
		t.Errorf("Expected %q, got: %q", expected, html)
	}
}
