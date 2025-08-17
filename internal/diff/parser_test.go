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

	tests := []struct {
		name                   string
		inputNode              *DOMNode
		expectedChildrenCount  int
		expectedAttributes     map[string]string
		shouldFilterFragmentID bool
		expectedFirstChildData string
	}{
		{
			name: "remove whitespace-only text and fragment ID",
			inputNode: &DOMNode{
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
			},
			expectedChildrenCount:  1,
			expectedAttributes:     map[string]string{"class": "test"},
			shouldFilterFragmentID: true,
			expectedFirstChildData: "Hello World",
		},
		{
			name: "preserve non-fragment ID attributes",
			inputNode: &DOMNode{
				Type: html.ElementNode,
				Data: "span",
				Attributes: map[string]string{
					"id":    "user-id-123",
					"class": "highlight",
					"data":  "value",
				},
				Children: []*DOMNode{
					{
						Type: html.TextNode,
						Data: "Content",
					},
				},
			},
			expectedChildrenCount: 1,
			expectedAttributes: map[string]string{
				"id":    "user-id-123",
				"class": "highlight",
				"data":  "value",
			},
			shouldFilterFragmentID: false,
			expectedFirstChildData: "Content",
		},
		{
			name: "empty node normalization",
			inputNode: &DOMNode{
				Type:       html.ElementNode,
				Data:       "p",
				Attributes: map[string]string{},
				Children:   []*DOMNode{},
			},
			expectedChildrenCount:  0,
			expectedAttributes:     map[string]string{},
			shouldFilterFragmentID: false,
			expectedFirstChildData: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := parser.NormalizeNode(tt.inputNode)

			if normalized == nil {
				t.Fatal("expected non-nil normalized node")
			}

			// Check children count
			if len(normalized.Children) != tt.expectedChildrenCount {
				t.Errorf("expected %d children after normalization, got %d", tt.expectedChildrenCount, len(normalized.Children))
			}

			// Check first child data if expected
			if tt.expectedChildrenCount > 0 && tt.expectedFirstChildData != "" {
				if normalized.Children[0].Data != tt.expectedFirstChildData {
					t.Errorf("expected first child data '%s', got '%s'", tt.expectedFirstChildData, normalized.Children[0].Data)
				}
			}

			// Check fragment ID filtering
			if tt.shouldFilterFragmentID {
				if _, exists := normalized.Attributes["id"]; exists {
					t.Error("fragment ID should be filtered out")
				}
			}

			// Check expected attributes
			for key, expectedValue := range tt.expectedAttributes {
				if actualValue, exists := normalized.Attributes[key]; !exists {
					t.Errorf("expected attribute '%s' to be preserved", key)
				} else if actualValue != expectedValue {
					t.Errorf("expected attribute '%s' value '%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
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
	tests := []struct {
		name     string
		node     *DOMNode
		expected string
	}{
		{
			name: "mixed content with nested elements",
			node: &DOMNode{
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
			},
			expected: "Hello World!",
		},
		{
			name: "simple text node",
			node: &DOMNode{
				Type: html.TextNode,
				Data: "Simple text",
			},
			expected: "Simple text",
		},
		{
			name: "element with only text children",
			node: &DOMNode{
				Type: html.ElementNode,
				Data: "p",
				Children: []*DOMNode{
					{
						Type: html.TextNode,
						Data: "First part",
					},
					{
						Type: html.TextNode,
						Data: " second part",
					},
				},
			},
			expected: "First part second part",
		},
		{
			name: "empty element",
			node: &DOMNode{
				Type:     html.ElementNode,
				Data:     "div",
				Children: []*DOMNode{},
			},
			expected: "",
		},
		{
			name: "deeply nested structure",
			node: &DOMNode{
				Type: html.ElementNode,
				Data: "article",
				Children: []*DOMNode{
					{
						Type: html.ElementNode,
						Data: "header",
						Children: []*DOMNode{
							{
								Type: html.ElementNode,
								Data: "h1",
								Children: []*DOMNode{
									{
										Type: html.TextNode,
										Data: "Title",
									},
								},
							},
						},
					},
					{
						Type: html.ElementNode,
						Data: "section",
						Children: []*DOMNode{
							{
								Type: html.TextNode,
								Data: " Content",
							},
						},
					},
				},
			},
			expected: "Title Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textContent := tt.node.GetTextContent()
			if textContent != tt.expected {
				t.Errorf("GetTextContent() = '%s', expected '%s'", textContent, tt.expected)
			}
		})
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

	t.Run("IsElementNode", func(t *testing.T) {
		tests := []struct {
			name     string
			node     *DOMNode
			expected bool
		}{
			{
				name:     "element node should return true",
				node:     elementNode,
				expected: true,
			},
			{
				name:     "text node should return false",
				node:     textNode,
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.node.IsElementNode()
				if result != tt.expected {
					t.Errorf("IsElementNode() = %v, expected %v", result, tt.expected)
				}
			})
		}
	})

	t.Run("IsTextNode", func(t *testing.T) {
		tests := []struct {
			name     string
			node     *DOMNode
			expected bool
		}{
			{
				name:     "element node should return false",
				node:     elementNode,
				expected: false,
			},
			{
				name:     "text node should return true",
				node:     textNode,
				expected: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.node.IsTextNode()
				if result != tt.expected {
					t.Errorf("IsTextNode() = %v, expected %v", result, tt.expected)
				}
			})
		}
	})

	t.Run("HasAttribute", func(t *testing.T) {
		tests := []struct {
			name      string
			node      *DOMNode
			attribute string
			expected  bool
		}{
			{
				name:      "element should have existing class attribute",
				node:      elementNode,
				attribute: "class",
				expected:  true,
			},
			{
				name:      "element should have existing id attribute",
				node:      elementNode,
				attribute: "id",
				expected:  true,
			},
			{
				name:      "element should not have nonexistent attribute",
				node:      elementNode,
				attribute: "nonexistent",
				expected:  false,
			},
			{
				name:      "text node should not have attributes",
				node:      textNode,
				attribute: "class",
				expected:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.node.HasAttribute(tt.attribute)
				if result != tt.expected {
					t.Errorf("HasAttribute(%s) = %v, expected %v", tt.attribute, result, tt.expected)
				}
			})
		}
	})

	t.Run("GetAttribute", func(t *testing.T) {
		tests := []struct {
			name      string
			node      *DOMNode
			attribute string
			expected  string
		}{
			{
				name:      "get existing class attribute",
				node:      elementNode,
				attribute: "class",
				expected:  "test",
			},
			{
				name:      "get existing id attribute",
				node:      elementNode,
				attribute: "id",
				expected:  "myid",
			},
			{
				name:      "get nonexistent attribute returns empty string",
				node:      elementNode,
				attribute: "nonexistent",
				expected:  "",
			},
			{
				name:      "text node attribute returns empty string",
				node:      textNode,
				attribute: "class",
				expected:  "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.node.GetAttribute(tt.attribute)
				if result != tt.expected {
					t.Errorf("GetAttribute(%s) = %s, expected %s", tt.attribute, result, tt.expected)
				}
			})
		}
	})
}
