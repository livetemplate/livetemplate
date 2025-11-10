package render

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func BenchmarkNodeRender(b *testing.B) {
	// Create a simple HTML node
	node := &html.Node{
		Type: html.ElementNode,
		Data: "div",
	}
	textNode := &html.Node{
		Type: html.TextNode,
		Data: "content",
	}
	node.AppendChild(textNode)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var w strings.Builder
		Node(&w, node)
	}
}

func BenchmarkTreeToHTML(b *testing.B) {
	simpleTree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}

	nestedTree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": map[string]interface{}{
			"s": []string{"<span>", "</span>"},
			"0": "nested",
		},
	}

	rangeTree := map[string]interface{}{
		"s": []string{"<li>", "</li>"},
		"d": []interface{}{
			map[string]interface{}{"0": "item1"},
			map[string]interface{}{"0": "item2"},
		},
	}

	b.Run("simple", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(simpleTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nested", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(nestedTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with-ranges", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(rangeTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTreeToHTMLScale(b *testing.B) {
	scales := []struct {
		name string
		size int
	}{
		{"small-10", 10},
		{"medium-100", 100},
		{"large-1000", 1000},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			// Create a range tree with many items
			items := make([]interface{}, scale.size)
			for i := 0; i < scale.size; i++ {
				items[i] = map[string]interface{}{"0": "item"}
			}

			rangeTree := map[string]interface{}{
				"s": []string{"<li>", "</li>"},
				"d": items,
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := TreeToHTML(rangeTree)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkIsVoidElement(b *testing.B) {
	tags := []string{"div", "br", "img", "span", "input", "hr", "p"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, tag := range tags {
			_ = IsVoidElement(tag)
		}
	}
}

func BenchmarkNodeRenderComplex(b *testing.B) {
	// Create a more complex HTML structure
	root := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{Key: "class", Val: "container"},
			{Key: "id", Val: "main"},
		},
	}

	h1 := &html.Node{
		Type: html.ElementNode,
		Data: "h1",
	}
	h1.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "Title",
	})
	root.AppendChild(h1)

	p := &html.Node{
		Type: html.ElementNode,
		Data: "p",
	}
	p.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: "Paragraph text",
	})
	root.AppendChild(p)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var w strings.Builder
		Node(&w, root)
	}
}

func BenchmarkMinifyHTML(b *testing.B) {
	htmlWithWhitespace := `
		<div>
			<h1>Title</h1>
			<p>
				Paragraph with    extra spaces
			</p>
		</div>
	`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = MinifyHTML(htmlWithWhitespace)
	}
}
