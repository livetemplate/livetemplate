package diff

import (
	"testing"

	"golang.org/x/net/html"
)

func TestDOMParser_Parse(t *testing.T) {
	parser := NewDOMParser()

	tests := []struct {
		name     string
		html     string
		wantErr  bool
		validate func(t *testing.T, node *DOMNode)
	}{
		{
			name:    "simple HTML",
			html:    "<html><body><p>Hello</p></body></html>",
			wantErr: false,
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
				// Should be the document node
				if node.Type != html.DocumentNode {
					t.Errorf("expected DocumentNode, got %v", node.Type)
				}
			},
		},
		{
			name:    "invalid HTML",
			html:    "<html><body><p>Unclosed tag",
			wantErr: false, // html.Parse is forgiving
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
			},
		},
		{
			name:    "empty HTML",
			html:    "",
			wantErr: false,
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parser.Parse(tt.html)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil {
				tt.validate(t, node)
			}
		})
	}
}

func TestDOMParser_ParseFragment(t *testing.T) {
	parser := NewDOMParser()

	tests := []struct {
		name     string
		html     string
		wantErr  bool
		validate func(t *testing.T, node *DOMNode)
	}{
		{
			name:    "single element",
			html:    "<p>Hello World</p>",
			wantErr: false,
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
				if node.Data != "p" {
					t.Errorf("expected 'p', got '%s'", node.Data)
				}
				if len(node.Children) != 1 {
					t.Errorf("expected 1 child, got %d", len(node.Children))
				}
			},
		},
		{
			name:    "multiple elements",
			html:    "<p>Hello</p><p>World</p>",
			wantErr: false,
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
				// Should be wrapped in a container
				if node.Data != "fragment-container" {
					t.Errorf("expected fragment-container, got '%s'", node.Data)
				}
				if len(node.Children) != 2 {
					t.Errorf("expected 2 children, got %d", len(node.Children))
				}
			},
		},
		{
			name:    "text only",
			html:    "Just text",
			wantErr: false,
			validate: func(t *testing.T, node *DOMNode) {
				if node == nil {
					t.Fatal("expected non-nil node")
				}
				if node.Type != html.TextNode {
					t.Errorf("expected TextNode, got %v", node.Type)
				}
			},
		},
		{
			name:    "empty fragment",
			html:    "",
			wantErr: true, // Should error on empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parser.ParseFragment(tt.html)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFragment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.validate != nil {
				tt.validate(t, node)
			}
		})
	}
}

func TestDOMParser_NormalizeNode(t *testing.T) {
	parser := NewDOMParser()

	// Create a test node with whitespace text
	node := &DOMNode{
		Type: html.ElementNode,
		Data: "div",
		Attributes: map[string]string{
			"id":    "fragment-abc123",
			"class": "test",
		},
		Children: []*DOMNode{
			{
				Type: html.TextNode,
				Data: "  \n\t  ", // Whitespace only
			},
			{
				Type: html.TextNode,
				Data: "Hello World",
			},
		},
	}

	normalized := parser.NormalizeNode(node)

	if normalized == nil {
		t.Fatal("expected non-nil normalized node")
	}

	// Should skip fragment ID
	if _, exists := normalized.Attributes["id"]; exists {
		t.Error("fragment ID should be filtered out")
	}

	// Should keep other attributes
	if normalized.Attributes["class"] != "test" {
		t.Error("class attribute should be preserved")
	}

	// Should have only one child (whitespace-only text removed)
	if len(normalized.Children) != 1 {
		t.Errorf("expected 1 child after normalization, got %d", len(normalized.Children))
	}

	if normalized.Children[0].Data != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", normalized.Children[0].Data)
	}
}

func TestDOMNode_GetPath(t *testing.T) {
	// Create a simple DOM structure
	root := &DOMNode{
		Type: html.ElementNode,
		Data: "html",
	}

	body := &DOMNode{
		Type:   html.ElementNode,
		Data:   "body",
		Parent: root,
	}
	root.Children = []*DOMNode{body}

	div := &DOMNode{
		Type:   html.ElementNode,
		Data:   "div",
		Parent: body,
	}
	body.Children = []*DOMNode{div}

	text := &DOMNode{
		Type:   html.TextNode,
		Data:   "Hello",
		Parent: div,
	}
	div.Children = []*DOMNode{text}

	tests := []struct {
		name     string
		node     *DOMNode
		expected string
	}{
		{
			name:     "root node",
			node:     root,
			expected: "html",
		},
		{
			name:     "body node",
			node:     body,
			expected: "html/body",
		},
		{
			name:     "div node",
			node:     div,
			expected: "html/body/div",
		},
		{
			name:     "text node",
			node:     text,
			expected: "html/body/div/text()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.node.GetPath()
			if path != tt.expected {
				t.Errorf("GetPath() = %s, expected %s", path, tt.expected)
			}
		})
	}
}

func TestDOMNode_GetTextContent(t *testing.T) {
	// Create a node with mixed content
	div := &DOMNode{
		Type: html.ElementNode,
		Data: "div",
		Children: []*DOMNode{
			{
				Type: html.TextNode,
				Data: "Hello ",
			},
			{
				Type: html.ElementNode,
				Data: "strong",
				Children: []*DOMNode{
					{
						Type: html.TextNode,
						Data: "World",
					},
				},
			},
			{
				Type: html.TextNode,
				Data: "!",
			},
		},
	}

	textContent := div.GetTextContent()
	expected := "Hello World!"

	if textContent != expected {
		t.Errorf("GetTextContent() = '%s', expected '%s'", textContent, expected)
	}
}

func TestDOMNode_HelperMethods(t *testing.T) {
	elementNode := &DOMNode{
		Type: html.ElementNode,
		Data: "div",
		Attributes: map[string]string{
			"class": "test",
			"id":    "myid",
		},
	}

	textNode := &DOMNode{
		Type: html.TextNode,
		Data: "Hello",
	}

	// Test IsElementNode
	if !elementNode.IsElementNode() {
		t.Error("element node should return true for IsElementNode()")
	}
	if textNode.IsElementNode() {
		t.Error("text node should return false for IsElementNode()")
	}

	// Test IsTextNode
	if elementNode.IsTextNode() {
		t.Error("element node should return false for IsTextNode()")
	}
	if !textNode.IsTextNode() {
		t.Error("text node should return true for IsTextNode()")
	}

	// Test HasAttribute
	if !elementNode.HasAttribute("class") {
		t.Error("element should have class attribute")
	}
	if elementNode.HasAttribute("nonexistent") {
		t.Error("element should not have nonexistent attribute")
	}

	// Test GetAttribute
	if elementNode.GetAttribute("class") != "test" {
		t.Error("class attribute should be 'test'")
	}
	if elementNode.GetAttribute("nonexistent") != "" {
		t.Error("nonexistent attribute should return empty string")
	}
}
