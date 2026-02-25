package livetemplate

import (
	"bytes"
	"fmt"
	"testing"
)

type Activity struct {
	Action string
	Data   map[string]any
}

func simulateUserJourney(tmpl *Template, activities []Activity) error {
	var buf bytes.Buffer
	for _, activity := range activities {
		buf.Reset()
		err := tmpl.ExecuteUpdates(&buf, activity.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func BenchmarkE2EUserJourney(b *testing.B) {
	tmpl := Must(New("counter"))
	_, err := tmpl.Parse(`<div><button>{{.Count}}</button></div>`)
	if err != nil {
		b.Fatal(err)
	}

	activities := make([]Activity, 100)
	for i := range 100 {
		activities[i] = Activity{
			Action: "increment",
			Data:   map[string]any{"Count": i},
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{"Count": 0}); err != nil {
		b.Fatalf("Execute failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := simulateUserJourney(tmpl, activities)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkE2ETodoApp(b *testing.B) {
	tmpl := Must(New("todos"))
	_, err := tmpl.Parse(`<ul>{{range .Items}}<li>{{.Text}}</li>{{end}}</ul>`)
	if err != nil {
		b.Fatal(err)
	}

	initialData := map[string]any{
		"Items": []map[string]any{},
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, initialData); err != nil {
		b.Fatalf("Execute failed: %v", err)
	}

	activities := []Activity{
		{Action: "add", Data: map[string]any{
			"Items": []map[string]any{
				{"Text": "Todo 1"},
			},
		}},
		{Action: "add", Data: map[string]any{
			"Items": []map[string]any{
				{"Text": "Todo 1"},
				{"Text": "Todo 2"},
			},
		}},
		{Action: "add", Data: map[string]any{
			"Items": []map[string]any{
				{"Text": "Todo 1"},
				{"Text": "Todo 2"},
				{"Text": "Todo 3"},
			},
		}},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := simulateUserJourney(tmpl, activities)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkE2ERangeOperations(b *testing.B) {
	tmpl := Must(New("list"))
	_, err := tmpl.Parse(`<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`)
	if err != nil {
		b.Fatal(err)
	}

	baseItems := []string{"a", "b", "c"}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{"Items": baseItems}); err != nil {
		b.Fatalf("Execute failed: %v", err)
	}

	b.Run("add-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			newItems := append(baseItems, "d", "e")
			err := tmpl.ExecuteUpdates(&buf, map[string]any{"Items": newItems})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("remove-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			newItems := baseItems[:2]
			err := tmpl.ExecuteUpdates(&buf, map[string]any{"Items": newItems})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("reorder-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			reordered := []string{"c", "a", "b"}
			err := tmpl.ExecuteUpdates(&buf, map[string]any{"Items": reordered})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("update-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			updated := []string{"x", "y", "z"}
			err := tmpl.ExecuteUpdates(&buf, map[string]any{"Items": updated})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkE2EMultipleSessions(b *testing.B) {
	template := `<div>{{.Count}}</div>`

	sessions := []int{1, 10, 100}

	for _, sessionCount := range sessions {
		b.Run(fmt.Sprintf("sessions-%d", sessionCount), func(b *testing.B) {
			templates := make([]*Template, sessionCount)
			buffers := make([]bytes.Buffer, sessionCount)
			for i := range sessionCount {
				tmpl := Must(New("session"))
				if _, err := tmpl.Parse(template); err != nil {
					b.Fatal(err)
				}
				if err := tmpl.Execute(&buffers[i], map[string]any{"Count": 0}); err != nil {
					b.Fatal(err)
				}
				templates[i] = tmpl
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for j, tmpl := range templates {
					buffers[j].Reset()
					err := tmpl.ExecuteUpdates(&buffers[j], map[string]any{"Count": i})
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
