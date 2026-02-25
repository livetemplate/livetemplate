package send

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

func BenchmarkParseActionFromHTTP(b *testing.B) {
	jsonData := `{"action":"increment","data":{"value":5}}`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		body := io.NopCloser(bytes.NewBufferString(jsonData))
		req := httptest.NewRequest("POST", "/action", body)
		req.Header.Set("Content-Type", "application/json")

		_, err := ParseActionFromHTTP(req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseActionFromWebSocket(b *testing.B) {
	jsonData := []byte(`{"action":"increment","data":{"value":5}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ParseActionFromWebSocket(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrepareUpdate(b *testing.B) {
	tree := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]any{"0": "updated value"},
	}

	b.Run("without-errors", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = PrepareUpdate(tree, nil, "")
		}
	})

	b.Run("with-errors", func(b *testing.B) {
		errors := map[string]string{
			"field1": "error1",
			"field2": "error2",
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = PrepareUpdate(tree, errors, "increment")
		}
	})
}

func BenchmarkSerializeUpdate(b *testing.B) {
	tree := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]any{"0": "value"},
	}

	response := PrepareUpdate(tree, nil, "")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := SerializeUpdate(response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrepareAndSerialize(b *testing.B) {
	tree := &build.TreeNode{
		Statics:  []string{"<div>", "</div>"},
		Dynamics: map[string]any{"0": "value"},
	}

	b.Run("simple-update", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := PrepareAndSerialize(tree, nil, "")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with-metadata", func(b *testing.B) {
		errors := map[string]string{"field": "error"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := PrepareAndSerialize(tree, errors, "update")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParseActionScale(b *testing.B) {
	tests := []struct {
		name string
		data string
	}{
		{"small", `{"action":"click","data":{"id":1}}`},
		{"medium", `{"action":"update","data":{"id":1,"name":"test","email":"test@example.com","active":true}}`},
		{"large", `{"action":"update","data":{"id":1,"name":"test","email":"test@example.com","active":true,"field1":"value1","field2":"value2","field3":"value3","field4":"value4","field5":"value5"}}`},
	}

	for _, tt := range tests {
		b.Run(tt.name+"-http", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				body := io.NopCloser(bytes.NewBufferString(tt.data))
				req := httptest.NewRequest("POST", "/action", body)
				req.Header.Set("Content-Type", "application/json")

				_, err := ParseActionFromHTTP(req)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(tt.name+"-ws", func(b *testing.B) {
			jsonData := []byte(tt.data)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := ParseActionFromWebSocket(jsonData)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSerializeUpdateScale(b *testing.B) {
	tests := []struct {
		name string
		tree *build.TreeNode
	}{
		{
			"simple",
			&build.TreeNode{
				Statics:  []string{"<div>", "</div>"},
				Dynamics: map[string]any{"0": "value"},
			},
		},
		{
			"nested",
			&build.TreeNode{
				Statics: []string{"<div>", "</div>"},
				Dynamics: map[string]any{
					"0": &build.TreeNode{
						Statics:  []string{"<span>", "</span>"},
						Dynamics: map[string]any{"0": "nested"},
					},
				},
			},
		},
		{
			"multiple-fields",
			&build.TreeNode{
				Statics: []string{"<div>", "", "", "", "</div>"},
				Dynamics: map[string]any{
					"0": "value1",
					"1": "value2",
					"2": "value3",
					"3": "value4",
				},
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			response := PrepareUpdate(tt.tree, nil, "")
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := SerializeUpdate(response)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
