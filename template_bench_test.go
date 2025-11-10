package livetemplate

import (
	"bytes"
	"testing"
)

// Core template operations

func BenchmarkTemplateExecute(b *testing.B) {
	b.Run("initial-render", func(b *testing.B) {
		data := map[string]interface{}{"Name": "Test"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tmpl := New("test")
			tmpl.Parse(`<div>{{.Name}}</div>`)
			var buf bytes.Buffer
			err := tmpl.Execute(&buf, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("subsequent-render", func(b *testing.B) {
		tmpl := New("test")
		tmpl.Parse(`<div>{{.Name}}</div>`)
		data := map[string]interface{}{"Name": "Test"}
		var buf bytes.Buffer
		tmpl.Execute(&buf, data) // Prime the cache

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := tmpl.Execute(&buf, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTemplateExecuteUpdates(b *testing.B) {
	tmpl := New("test")
	tmpl.Parse(`<div>{{.Name}}</div>`)

	initialData := map[string]interface{}{"Name": "Initial"}
	var buf bytes.Buffer
	tmpl.ExecuteUpdates(&buf, initialData) // Prime

	b.Run("no-changes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := tmpl.ExecuteUpdates(&buf, initialData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("small-update", func(b *testing.B) {
		data := map[string]interface{}{"Name": "Updated"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := tmpl.ExecuteUpdates(&buf, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	largeTemplate := `<div>
		<h1>{{.Title}}</h1>
		<p>{{.Description}}</p>
		<span>{{.Author}}</span>
		<time>{{.Date}}</time>
		<div>{{.Content}}</div>
	</div>`

	tmplLarge := New("large")
	tmplLarge.Parse(largeTemplate)
	largeData := map[string]interface{}{
		"Title":       "Title",
		"Description": "Description",
		"Author":      "Author",
		"Date":        "2025-01-01",
		"Content":     "Content",
	}
	buf.Reset()
	tmplLarge.ExecuteUpdates(&buf, largeData)

	b.Run("large-update", func(b *testing.B) {
		updatedData := map[string]interface{}{
			"Title":       "New Title",
			"Description": "New Description",
			"Author":      "New Author",
			"Date":        "2025-01-02",
			"Content":     "New Content",
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := tmplLarge.ExecuteUpdates(&buf, updatedData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Template complexity variations

func BenchmarkTemplateComplexity(b *testing.B) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{
			"simple-fields",
			`<div>{{.A}} {{.B}} {{.C}}</div>`,
			map[string]interface{}{"A": "a", "B": "b", "C": "c"},
		},
		{
			"with-conditionals",
			`<div>{{if .Show}}<span>{{.Name}}</span>{{else}}<span>Hidden</span>{{end}}</div>`,
			map[string]interface{}{"Show": true, "Name": "Test"},
		},
		{
			"with-ranges",
			`<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			"deeply-nested",
			`<div>{{range .L1}}{{range .L2}}{{range .L3}}<span>{{.}}</span>{{end}}{{end}}{{end}}</div>`,
			map[string]interface{}{
				"L1": []map[string]interface{}{
					{"L2": []map[string]interface{}{
						{"L3": []string{"a", "b"}},
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tmpl := New("test")
			_, err := tmpl.Parse(tt.template)
			if err != nil {
				b.Fatal(err)
			}

			var buf bytes.Buffer
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				err := tmpl.Execute(&buf, tt.data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Concurrent operations

func BenchmarkTemplateConcurrent(b *testing.B) {
	tmpl := New("test")
	_, err := tmpl.Parse(`<div>{{.Name}}</div>`)
	if err != nil {
		b.Fatal(err)
	}

	data := map[string]interface{}{"Name": "Test"}

	concurrency := []int{1, 10, 100}

	for _, n := range concurrency {
		b.Run(string(rune('0'+n)), func(b *testing.B) {
			b.SetParallelism(n)
			b.RunParallel(func(pb *testing.PB) {
				var buf bytes.Buffer
				for pb.Next() {
					buf.Reset()
					err := tmpl.Execute(&buf, data)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
