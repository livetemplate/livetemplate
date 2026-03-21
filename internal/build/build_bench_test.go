package build

import (
	"testing"
)

// Benchmark helpers

func createTestTree(depth, breadth int) *TreeNode {
	if depth == 0 {
		return NewTreeNodeWithStatics([]string{"<span>", "</span>"})
	}

	statics := make([]string, breadth+1)
	statics[0] = "<div>"
	for i := 1; i < breadth; i++ {
		statics[i] = ""
	}
	statics[breadth] = "</div>"

	node := NewTreeNodeWithStatics(statics)
	for i := 0; i < breadth; i++ {
		node.SetDynamic(i, createTestTree(depth-1, breadth))
	}

	return node
}

// TreeNode operations

func BenchmarkTreeNodeCreation(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
		{"nested-large", 5, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = createTestTree(tt.depth, tt.breadth)
			}
		})
	}
}

func BenchmarkTreeNodeMarshalJSON(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tree := createTestTree(tt.depth, tt.breadth)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := tree.MarshalJSON()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Wrapper operations

func BenchmarkWrapperInjection(b *testing.B) {
	fullHTML := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<div><h1>Hello</h1><p>World</p></div>
</body>
</html>`

	fragment := `<div><h1>Hello</h1><p>World</p></div>`

	b.Run("full-html", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = InjectWrapperDiv(fullHTML, "test-wrapper", false)
		}
	})

	b.Run("fragment", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = InjectWrapperDiv(fragment, "test-wrapper", false)
		}
	})
}

func BenchmarkExtractWrapperContent(b *testing.B) {
	html := `<div data-lvt-id="test-wrapper"><div><h1>Hello</h1><p>World</p></div></div>`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ExtractTemplateContent(html, "test-wrapper")
	}
}

// Context operations

func BenchmarkContextOperations(b *testing.B) {
	tree := createTestTree(3, 3)

	b.Run("with-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := NewContext()
			_ = ctx.ShouldIncludeStatics()
			// Process tree through context
			_ = tree.Clone()
		}
	})

	b.Run("without-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := NewUpdateContext(nil)
			_ = ctx.ShouldIncludeStatics()
			// Process tree through context
			_ = tree.Clone()
		}
	})
}

// Additional TreeNode operations

func BenchmarkTreeNodeClone(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tree := createTestTree(tt.depth, tt.breadth)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = tree.Clone()
			}
		})
	}
}

func BenchmarkTreeNodeToMap(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tree := createTestTree(tt.depth, tt.breadth)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = tree.ToMap()
			}
		})
	}
}

func BenchmarkGenerateRandomID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateRandomID()
	}
}
