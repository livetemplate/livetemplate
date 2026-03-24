package send

import (
	"bytes"
	stdjson "encoding/json"
	"strings"
	"testing"

	jsonv2 "github.com/go-json-experiment/json"
	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"github.com/livetemplate/livetemplate/internal/build"
)

var jsoniterAPI = jsoniter.ConfigCompatibleWithStandardLibrary

// Test data builders

func simpleTree() *build.TreeNode {
	tn := &build.TreeNode{
		Statics:  []string{"<div class=\"counter\">", "</div>"},
		Dynamics: []interface{}{"42"},
	}
	return tn
}

func nestedTree() *build.TreeNode {
	child := &build.TreeNode{
		Statics:  []string{"<span>", "</span>"},
		Dynamics: []interface{}{"nested value"},
	}
	return &build.TreeNode{
		Statics:  []string{"<div>", "<p>", "</p>", "</div>"},
		Dynamics: []interface{}{child, "static text", "another value"},
	}
}

func rangeTree() *build.TreeNode {
	items := make([]interface{}, 10)
	for i := range items {
		items[i] = &build.TreeNode{
			Dynamics: []interface{}{"item-" + string(rune('0'+i)), "Item Name"},
			AutoKey:  "key-" + string(rune('0'+i)),
		}
	}
	return &build.TreeNode{
		Statics: []string{"<ul>", "</ul>"},
		Range: &build.RangeData{
			Items:   items,
			Statics: []string{`<li data-key="`, `">`, `</li>`},
		},
		Metadata: &build.TreeMetadata{IDKey: "0"},
	}
}

func updateResponse() *UpdateResponse {
	return PrepareUpdate(nestedTree(), nil, "")
}

func actionJSON() []byte {
	return []byte(`{"action":"addTodo","data":{"title":"Buy groceries","priority":"high","tags":["food","urgent"]}}`)
}

// =============================================================================
// Marshal benchmarks
// =============================================================================

func BenchmarkJSONMarshal_TreeNode(b *testing.B) {
	trees := map[string]*build.TreeNode{
		"simple": simpleTree(),
		"nested": nestedTree(),
		"range":  rangeTree(),
	}

	for name, tree := range trees {
		b.Run(name+"/stdlib", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := stdjson.Marshal(tree)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/jsoniter", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := jsoniterAPI.Marshal(tree)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/gojson", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := gojson.Marshal(tree)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/jsonv2", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := jsonv2.Marshal(tree)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// =============================================================================
// Unmarshal benchmarks
// =============================================================================

func BenchmarkJSONUnmarshal_TreeNode(b *testing.B) {
	trees := map[string]*build.TreeNode{
		"simple": simpleTree(),
		"nested": nestedTree(),
	}

	for name, tree := range trees {
		data, _ := stdjson.Marshal(tree)

		b.Run(name+"/stdlib", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var tn build.TreeNode
				if err := stdjson.Unmarshal(data, &tn); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/jsoniter", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var tn build.TreeNode
				if err := jsoniterAPI.Unmarshal(data, &tn); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/gojson", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var tn build.TreeNode
				if err := gojson.Unmarshal(data, &tn); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(name+"/jsonv2", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var tn build.TreeNode
				if err := jsonv2.Unmarshal(data, &tn); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// =============================================================================
// UpdateResponse marshal (the actual hot path)
// =============================================================================

func BenchmarkJSONMarshal_UpdateResponse(b *testing.B) {
	resp := updateResponse()

	b.Run("stdlib", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := stdjson.Marshal(resp)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsoniter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := jsoniterAPI.Marshal(resp)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("gojson", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := gojson.Marshal(resp)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsonv2", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := jsonv2.Marshal(resp)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// =============================================================================
// ActionMessage unmarshal (client → server)
// =============================================================================

func BenchmarkJSONUnmarshal_ActionMessage(b *testing.B) {
	data := actionJSON()

	b.Run("stdlib", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var msg ActionMessage
			if err := stdjson.Unmarshal(data, &msg); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsoniter", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var msg ActionMessage
			if err := jsoniterAPI.Unmarshal(data, &msg); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("gojson", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var msg ActionMessage
			if err := gojson.Unmarshal(data, &msg); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsonv2", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var msg ActionMessage
			if err := jsonv2.Unmarshal(data, &msg); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// =============================================================================
// Encoder with HTML escape disabled (MarshalOrderedJSON replacement)
// =============================================================================

func BenchmarkJSONEncoder_NoHTMLEscape(b *testing.B) {
	tree := nestedTree()

	b.Run("stdlib", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			enc := stdjson.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(tree); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsoniter", func(b *testing.B) {
		api := jsoniter.Config{
			EscapeHTML: false,
		}.Froze()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := api.Marshal(tree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("gojson", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := gojson.MarshalNoEscape(tree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// =============================================================================
// Wire format correctness verification (not a benchmark, but important)
// =============================================================================

func TestJSONLibrary_WireFormatCompatibility(t *testing.T) {
	tree := simpleTree()
	tree.SetDynamic(0, "<b>bold</b>") // HTML content that must not be escaped

	stdBytes, err := stdjson.Marshal(tree)
	if err != nil {
		t.Fatalf("stdlib marshal failed: %v", err)
	}
	iterBytes, err := jsoniterAPI.Marshal(tree)
	if err != nil {
		t.Fatalf("jsoniter marshal failed: %v", err)
	}
	goBytes, err := gojson.Marshal(tree)
	if err != nil {
		t.Fatalf("gojson marshal failed: %v", err)
	}
	v2Bytes, err := jsonv2.Marshal(tree)
	if err != nil {
		t.Fatalf("jsonv2 marshal failed: %v", err)
	}

	// All libraries must produce valid JSON that round-trips correctly
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"stdlib", stdBytes},
		{"jsoniter", iterBytes},
		{"gojson", goBytes},
		{"jsonv2", v2Bytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result map[string]interface{}
			if err := stdjson.Unmarshal(tc.data, &result); err != nil {
				t.Fatalf("Failed to unmarshal %s output: %v", tc.name, err)
			}
			// Check statics are present
			if _, ok := result["s"]; !ok {
				t.Errorf("%s: missing 's' key", tc.name)
			}
			// Check dynamic at position 0 contains unescaped HTML
			val, ok := result["0"]
			if !ok {
				t.Errorf("%s: missing '0' key", tc.name)
			} else if str, ok := val.(string); ok {
				if !strings.Contains(str, "<b>") {
					t.Errorf("%s: HTML was escaped in dynamic value: %s", tc.name, str)
				}
			}
		})
	}
}
