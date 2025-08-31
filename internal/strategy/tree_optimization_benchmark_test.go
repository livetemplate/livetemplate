package strategy

import (
	"encoding/json"
	"testing"
)

// BenchmarkTreeOptimization benchmarks tree-based optimization vs traditional approaches
func BenchmarkTreeOptimization(b *testing.B) {
	benchCases := []struct {
		name           string
		templateSource string
		data           map[string]interface{}
	}{
		{
			name:           "SimpleFields",
			templateSource: `<div>Hello {{.Name}}, you have {{.Count}} messages</div>`,
			data:           map[string]interface{}{"Name": "John", "Count": 42},
		},
		{
			name:           "ConditionalBranching",
			templateSource: `{{if .Premium}}<gold>VIP: {{.Name}}</gold>{{else}}<span>User: {{.Name}}</span>{{end}}`,
			data:           map[string]interface{}{"Premium": true, "Name": "Alice"},
		},
		{
			name:           "RangeIteration",
			templateSource: `<ul>{{range .Items}}<li>Item: {{.}}</li>{{end}}</ul>`,
			data:           map[string]interface{}{"Items": []interface{}{"Apple", "Banana", "Cherry", "Date"}},
		},
		{
			name:           "NestedComplexity",
			templateSource: `{{range .Users}}<div>{{if .Active}}✓ {{.Name}} ({{.Role}}){{else}}✗ {{.Name}}{{end}}</div>{{end}}`,
			data: map[string]interface{}{
				"Users": []interface{}{
					map[string]interface{}{"Name": "Alice", "Active": true, "Role": "Admin"},
					map[string]interface{}{"Name": "Bob", "Active": false, "Role": "User"},
					map[string]interface{}{"Name": "Charlie", "Active": true, "Role": "Manager"},
				},
			},
		},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			generator := NewSimpleTreeGenerator()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := generator.GenerateFromTemplateSource(bc.templateSource, nil, bc.data, "benchmark")
				if err != nil {
					b.Fatalf("Failed to generate tree: %v", err)
				}

				// Measure JSON serialization as part of client communication cost
				_, err = json.Marshal(result)
				if err != nil {
					b.Fatalf("Failed to marshal result: %v", err)
				}
			}
		})
	}
}

// BenchmarkIncrementalUpdates benchmarks the incremental update performance
func BenchmarkIncrementalUpdates(b *testing.B) {
	generator := NewSimpleTreeGenerator()
	templateSource := `<div>User: {{.Name}}, Score: {{.Score}}, Level: {{.Level}}</div>`
	fragmentID := "benchmark-incremental"

	// First render to establish cache
	firstData := map[string]interface{}{"Name": "Player1", "Score": 1000, "Level": 5}
	_, err := generator.GenerateFromTemplateSource(templateSource, nil, firstData, fragmentID)
	if err != nil {
		b.Fatalf("First render failed: %v", err)
	}

	// Benchmark incremental updates
	updateData := map[string]interface{}{"Name": "Player1", "Score": 1500, "Level": 6}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := generator.GenerateFromTemplateSource(templateSource, firstData, updateData, fragmentID)
		if err != nil {
			b.Fatalf("Incremental update failed: %v", err)
		}

		// Measure serialization cost
		resultJSON, err := json.Marshal(result)
		if err != nil {
			b.Fatalf("Failed to marshal: %v", err)
		}

		// Simulate bandwidth calculation
		_ = len(resultJSON)
	}
}

// BenchmarkBandwidthSavings measures bandwidth efficiency
func BenchmarkBandwidthSavings(b *testing.B) {
	generator := NewSimpleTreeGenerator()
	templateSource := `<section class="dashboard"><h2>{{.Title}}</h2><p>Welcome {{.User.Name}}, you have {{.User.MessageCount}} new messages</p></section>`
	fragmentID := "bandwidth-test"

	firstData := map[string]interface{}{
		"Title": "Dashboard",
		"User": map[string]interface{}{
			"Name":         "John Doe",
			"MessageCount": 5,
		},
	}

	updateData := map[string]interface{}{
		"Title": "Dashboard",
		"User": map[string]interface{}{
			"Name":         "John Doe",
			"MessageCount": 12,
		},
	}

	// First render
	firstResult, _ := generator.GenerateFromTemplateSource(templateSource, nil, firstData, fragmentID)
	firstJSON, _ := json.Marshal(firstResult)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Incremental update
		updateResult, err := generator.GenerateFromTemplateSource(templateSource, firstData, updateData, fragmentID)
		if err != nil {
			b.Fatalf("Update failed: %v", err)
		}

		updateJSON, _ := json.Marshal(updateResult)

		// Calculate bandwidth savings
		originalSize := len(firstJSON)
		updateSize := len(updateJSON)
		savings := float64(originalSize-updateSize) / float64(originalSize) * 100

		// Report savings in first iteration
		if i == 0 {
			b.ReportMetric(savings, "bandwidth_savings_%")
			b.ReportMetric(float64(originalSize), "original_bytes")
			b.ReportMetric(float64(updateSize), "update_bytes")
		}
	}
}
