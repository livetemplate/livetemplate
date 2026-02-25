package livetemplate

import (
	"bytes"
	"testing"
)

// Error path benchmarks - testing performance of error handling

func BenchmarkErrorPaths(b *testing.B) {
	b.Run("invalid-template-syntax", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tmpl := Must(New("error"))
			_, err := tmpl.Parse(`<div>{{.Invalid syntax here`)
			if err == nil {
				b.Fatal("expected parse error")
			}
		}
	})

	b.Run("missing-field", func(b *testing.B) {
		tmpl := Must(New("test"))
		_, err := tmpl.Parse(`<div>{{.NonExistent}}</div>`)
		if err != nil {
			b.Fatal(err)
		}

		data := map[string]any{"Other": "value"}
		var buf bytes.Buffer

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			// Execute with missing field - should use zero value
			err := tmpl.Execute(&buf, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nil-data", func(b *testing.B) {
		tmpl := Must(New("test"))
		_, err := tmpl.Parse(`<div>{{.Name}}</div>`)
		if err != nil {
			b.Fatal(err)
		}

		var buf bytes.Buffer

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			err := tmpl.Execute(&buf, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("empty-template", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tmpl := Must(New("empty"))
			_, err := tmpl.Parse("")
			if err != nil {
				b.Fatal(err)
			}

			var buf bytes.Buffer
			data := map[string]any{"Name": "Test"}
			err = tmpl.Execute(&buf, data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
