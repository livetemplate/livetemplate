package parse

import (
	"testing"
)

// Benchmark helpers

func createSimpleTemplate() string {
	return `<div>{{.Name}}</div>`
}

func createConditionalTemplate() string {
	return `<div>{{if .Show}}<span>{{.Name}}</span>{{else}}<span>Hidden</span>{{end}}</div>`
}

func createRangeTemplate() string {
	return `<ul>{{range .Items}}<li>{{.Name}}</li>{{end}}</ul>`
}

func createNestedTemplate() string {
	return `<div>{{range .Items}}{{if .Active}}<span>{{.Name}}</span>{{end}}{{end}}</div>`
}

func createComplexTemplate() string {
	return `<div>
		<h1>{{.Title}}</h1>
		{{if .ShowItems}}
		<ul>
		{{range .Items}}
			<li>
				<span>{{.Name}}</span>
				{{if .Tags}}
				<div>{{range .Tags}}<span>{{.}}</span>{{end}}</div>
				{{end}}
			</li>
		{{end}}
		</ul>
		{{end}}
	</div>`
}

// Entry point benchmarks

func BenchmarkParse(b *testing.B) {
	tests := []struct {
		name     string
		template string
	}{
		{"simple", createSimpleTemplate()},
		{"conditional", createConditionalTemplate()},
		{"range", createRangeTemplate()},
		{"nested", createNestedTemplate()},
		{"complex", createComplexTemplate()},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := Parse(tt.template, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBuildTree(b *testing.B) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{"simple", createSimpleTemplate(), map[string]interface{}{"Name": "Test"}},
		{"conditional-true", createConditionalTemplate(), map[string]interface{}{"Show": true, "Name": "Test"}},
		{"conditional-false", createConditionalTemplate(), map[string]interface{}{"Show": false, "Name": "Test"}},
		{"range-small", createRangeTemplate(), map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Name": "Item 1"},
				{"Name": "Item 2"},
				{"Name": "Item 3"},
			},
		}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tmpl, err := Parse(tt.template, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := &Context{IncludeStatics: true}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := BuildTree(tmpl, tt.data, newMockKeyGen(), ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Scale variation benchmarks

func BenchmarkBuildTreeScale(b *testing.B) {
	templateStr := createRangeTemplate()

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
			items := make([]map[string]interface{}, scale.size)
			for i := 0; i < scale.size; i++ {
				items[i] = map[string]interface{}{"Name": "Item"}
			}
			data := map[string]interface{}{"Items": items}

			tmpl, err := Parse(templateStr, nil)
			if err != nil {
				b.Fatal(err)
			}
			ctx := &Context{IncludeStatics: true}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := BuildTree(tmpl, data, newMockKeyGen(), ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
