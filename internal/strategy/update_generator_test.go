package strategy

import (
	"html/template"
	"testing"
	"time"
)

func TestUpdateGenerator_GenerateUpdate(t *testing.T) {
	generator := NewUpdateGenerator()

	tests := []struct {
		name             string
		templateContent  string
		oldData          interface{}
		newData          interface{}
		expectedStrategy string
		wantErr          bool
	}{
		{
			name:             "text-only change - Strategy 1",
			templateContent:  `<div>Hello {{.Name}}</div>`,
			oldData:          map[string]interface{}{"Name": "Alice"},
			newData:          map[string]interface{}{"Name": "Bob"},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "attribute value change - Strategy 1",
			templateContent:  `<div class="{{.Class}}">Content</div>`,
			oldData:          map[string]interface{}{"Class": "old"},
			newData:          map[string]interface{}{"Class": "new"},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "attribute structure change - now enhanced Strategy 1",
			templateContent:  `<div{{if .HasClass}} class="active"{{end}}>Content</div>`,
			oldData:          map[string]interface{}{"HasClass": false},
			newData:          map[string]interface{}{"HasClass": true},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "structural change - Strategy 3",
			templateContent:  `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			oldData:          map[string]interface{}{"Items": []string{"A"}},
			newData:          map[string]interface{}{"Items": []string{"A", "B"}},
			expectedStrategy: "granular",
			wantErr:          false,
		},
		{
			name:             "if-else structural conditional - now Strategy 1",
			templateContent:  `{{if .ShowTable}}<table><tr><td>{{.Value}}</td></tr></table>{{else}}<div>{{.Value}}</div>{{end}}`,
			oldData:          map[string]interface{}{"ShowTable": false, "Value": "Data"},
			newData:          map[string]interface{}{"ShowTable": true, "Value": "Updated"},
			expectedStrategy: "static_dynamic", // Now detected as enhanced Strategy 1 with if-else conditionals
			wantErr:          false,
		},
		{
			name:             "simple empty to content",
			templateContent:  `<div>{{.Content}}</div>`,
			oldData:          map[string]interface{}{"Content": ""},
			newData:          map[string]interface{}{"Content": "Hello"},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "simple content change",
			templateContent:  `<div>{{.Content}}</div>`,
			oldData:          map[string]interface{}{"Content": "Hello"},
			newData:          map[string]interface{}{"Content": "World"},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "boolean conditional - enhanced Strategy 1",
			templateContent:  `<button{{if .Disabled}} disabled{{end}}>Click me</button>`,
			oldData:          map[string]interface{}{"Disabled": false},
			newData:          map[string]interface{}{"Disabled": true},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "show/hide conditional - enhanced Strategy 1",
			templateContent:  `<div>{{if .ShowBadge}}<span class="badge">{{.Count}}</span>{{end}}</div>`,
			oldData:          map[string]interface{}{"ShowBadge": false, "Count": 5},
			newData:          map[string]interface{}{"ShowBadge": true, "Count": 5},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
		{
			name:             "nil-notnil conditional - enhanced Strategy 1",
			templateContent:  `<div{{if .Class}} class="{{.Class}}"{{end}}>Content</div>`,
			oldData:          map[string]interface{}{"Class": ""},
			newData:          map[string]interface{}{"Class": "active"},
			expectedStrategy: "static_dynamic",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateContent)
			if err != nil {
				t.Fatalf("Template parsing failed: %v", err)
			}

			fragments, err := generator.GenerateUpdate(tmpl, tt.oldData, tt.newData)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error
			}

			if len(fragments) == 0 {
				t.Error("Expected at least one fragment")
				return
			}

			fragment := fragments[0]
			if fragment.Strategy != tt.expectedStrategy {
				t.Errorf("Strategy = %s, want %s", fragment.Strategy, tt.expectedStrategy)
			}

			if fragment.ID == "" {
				t.Error("Fragment ID should not be empty")
			}

			if fragment.Data == nil {
				t.Error("Fragment data should not be nil")
			}

			if fragment.Metadata == nil {
				t.Error("Fragment metadata should not be nil")
			}

			if fragment.Metadata.GenerationTime <= 0 {
				t.Error("Generation time should be positive")
			}

			if fragment.Metadata.OriginalSize <= 0 {
				t.Error("Original size should be positive")
			}
		})
	}
}

func TestUpdateGenerator_TemplateRendering(t *testing.T) {
	generator := NewUpdateGenerator()

	tests := []struct {
		name          string
		templateText  string
		oldData       interface{}
		newData       interface{}
		expectOldHTML string
		expectNewHTML string
		wantErr       bool
	}{
		{
			name:          "simple text substitution",
			templateText:  `<span>{{.Text}}</span>`,
			oldData:       map[string]interface{}{"Text": "Before"},
			newData:       map[string]interface{}{"Text": "After"},
			expectOldHTML: `<span>Before</span>`,
			expectNewHTML: `<span>After</span>`,
			wantErr:       false,
		},
		{
			name:          "nil old data",
			templateText:  `<span>{{.Text}}</span>`,
			oldData:       nil,
			newData:       map[string]interface{}{"Text": "New"},
			expectOldHTML: "",
			expectNewHTML: `<span>New</span>`,
			wantErr:       false,
		},
		{
			name:          "conditional rendering",
			templateText:  `{{if .Show}}<div>{{.Content}}</div>{{end}}`,
			oldData:       map[string]interface{}{"Show": false, "Content": "Hidden"},
			newData:       map[string]interface{}{"Show": true, "Content": "Visible"},
			expectOldHTML: "",
			expectNewHTML: `<div>Visible</div>`,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Template parsing failed: %v", err)
			}

			oldHTML, newHTML, err := generator.renderTemplates(tmpl, tt.oldData, tt.newData)

			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error
			}

			if oldHTML != tt.expectOldHTML {
				t.Errorf("Old HTML = %q, want %q", oldHTML, tt.expectOldHTML)
			}

			if newHTML != tt.expectNewHTML {
				t.Errorf("New HTML = %q, want %q", newHTML, tt.expectNewHTML)
			}
		})
	}
}

func TestUpdateGenerator_FallbackHandling(t *testing.T) {
	generator := NewUpdateGenerator()

	// Create a template that might cause strategy failures in some edge cases
	tmpl, err := template.New("test").Parse(`<div class="{{.Class}}">{{.Text}}</div>`)
	if err != nil {
		t.Fatalf("Template parsing failed: %v", err)
	}

	oldData := map[string]interface{}{"Class": "old", "Text": "Before"}
	newData := map[string]interface{}{"Class": "new", "Text": "After"}

	// Test with fallback enabled
	generator.SetFallbackEnabled(true)
	fragments, err := generator.GenerateUpdate(tmpl, oldData, newData)
	if err != nil {
		t.Errorf("Update generation with fallback failed: %v", err)
	}
	if len(fragments) == 0 {
		t.Error("Expected at least one fragment with fallback enabled")
	}

	// Test with fallback disabled
	generator.SetFallbackEnabled(false)
	fragments2, err2 := generator.GenerateUpdate(tmpl, oldData, newData)
	if err2 != nil {
		t.Errorf("Update generation without fallback failed: %v", err2)
	}
	if len(fragments2) == 0 {
		t.Error("Expected at least one fragment with fallback disabled")
	}
}

func TestUpdateGenerator_Metrics(t *testing.T) {
	generator := NewUpdateGenerator()
	generator.SetMetricsEnabled(true)

	tmpl, err := template.New("test").Parse(`<span>{{.Text}}</span>`)
	if err != nil {
		t.Fatalf("Template parsing failed: %v", err)
	}

	// Generate several updates
	testData := []struct {
		oldData interface{}
		newData interface{}
	}{
		{
			map[string]interface{}{"Text": "A"},
			map[string]interface{}{"Text": "B"},
		},
		{
			map[string]interface{}{"Text": "C"},
			map[string]interface{}{"Text": "D"},
		},
		{
			map[string]interface{}{"Text": "E"},
			map[string]interface{}{"Text": "F"},
		},
	}

	for _, data := range testData {
		_, err := generator.GenerateUpdate(tmpl, data.oldData, data.newData)
		if err != nil {
			t.Errorf("Update generation failed: %v", err)
		}
	}

	// Check metrics
	metrics := generator.GetMetrics()

	if metrics.TotalGenerations != int64(len(testData)) {
		t.Errorf("TotalGenerations = %d, want %d", metrics.TotalGenerations, len(testData))
	}

	if metrics.SuccessfulGenerations != int64(len(testData)) {
		t.Errorf("SuccessfulGenerations = %d, want %d", metrics.SuccessfulGenerations, len(testData))
	}

	if metrics.FailedGenerations != 0 {
		t.Errorf("FailedGenerations = %d, want 0", metrics.FailedGenerations)
	}

	if len(metrics.StrategyUsage) == 0 {
		t.Error("StrategyUsage should not be empty")
	}

	if metrics.AverageGenerationTime <= 0 {
		t.Error("AverageGenerationTime should be positive")
	}

	// Test metrics reset
	generator.ResetMetrics()
	resetMetrics := generator.GetMetrics()

	if resetMetrics.TotalGenerations != 0 {
		t.Error("Metrics should be reset")
	}
}

func TestUpdateGenerator_PerformanceOptimization(t *testing.T) {
	generator := NewUpdateGenerator()

	// Test that repeated identical updates are efficiently handled
	tmpl, err := template.New("test").Parse(`<div>{{.Value}}</div>`)
	if err != nil {
		t.Fatalf("Template parsing failed: %v", err)
	}

	oldData := map[string]interface{}{"Value": "Same"}
	newData := map[string]interface{}{"Value": "Same"}

	start := time.Now()

	// Run multiple identical updates
	for i := 0; i < 10; i++ {
		fragments, err := generator.GenerateUpdate(tmpl, oldData, newData)
		if err != nil {
			t.Errorf("Update generation failed: %v", err)
		}

		if len(fragments) == 0 {
			t.Error("Expected at least one fragment")
		}
	}

	duration := time.Since(start)

	// Should complete quickly due to caching and optimization
	if duration > 100*time.Millisecond {
		t.Errorf("Performance test took too long: %v", duration)
	}

	// Test bandwidth optimization metrics
	metrics := generator.GetMetrics()
	if metrics.TotalBandwidthSaved < 0 {
		t.Error("TotalBandwidthSaved should not be negative")
	}
}

func TestUpdateGenerator_ErrorHandling(t *testing.T) {
	generator := NewUpdateGenerator()

	tests := []struct {
		name         string
		templateText string
		oldData      interface{}
		newData      interface{}
		expectError  bool
	}{
		{
			name:         "template with missing required option",
			templateText: `<div>{{.Text | printf "Value: %s"}}</div>`,
			oldData:      map[string]interface{}{"Text": "Valid"},
			newData:      map[string]interface{}{"Text": "Valid"},
			expectError:  false, // This actually works in Go templates
		},
		{
			name:         "valid template",
			templateText: `<div>{{.Text}}</div>`,
			oldData:      map[string]interface{}{"Text": "Valid"},
			newData:      map[string]interface{}{"Text": "Updated"},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Template parsing failed: %v", err)
			}

			_, err = generator.GenerateUpdate(tmpl, tt.oldData, tt.newData)

			if (err != nil) != tt.expectError {
				t.Errorf("GenerateUpdate() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}

	// Test error metrics
	metrics := generator.GetMetrics()
	if metrics.ErrorRate < 0 || metrics.ErrorRate > 1 {
		t.Errorf("ErrorRate should be between 0 and 1, got %f", metrics.ErrorRate)
	}
}

func TestUpdateGenerator_FragmentGeneration(t *testing.T) {
	generator := NewUpdateGenerator()

	// Test each strategy type generates correct fragment structure
	tests := []struct {
		name           string
		templateText   string
		oldData        interface{}
		newData        interface{}
		expectedFields []string
	}{
		{
			name:           "static dynamic fragment",
			templateText:   `<span>{{.Text}}</span>`,
			oldData:        map[string]interface{}{"Text": "Old"},
			newData:        map[string]interface{}{"Text": "New"},
			expectedFields: []string{"ID", "Strategy", "Action", "Data", "Metadata"},
		},
		{
			name:           "marker fragment",
			templateText:   `<div class="{{.Class}}">Content</div>`,
			oldData:        map[string]interface{}{"Class": "old"},
			newData:        map[string]interface{}{"Class": "new"},
			expectedFields: []string{"ID", "Strategy", "Action", "Data", "Metadata"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Template parsing failed: %v", err)
			}

			fragments, err := generator.GenerateUpdate(tmpl, tt.oldData, tt.newData)
			if err != nil {
				t.Fatalf("Update generation failed: %v", err)
			}

			if len(fragments) == 0 {
				t.Fatal("Expected at least one fragment")
			}

			fragment := fragments[0]

			// Test required fields are present
			if fragment.ID == "" {
				t.Error("Fragment ID should not be empty")
			}

			if fragment.Strategy == "" {
				t.Error("Fragment Strategy should not be empty")
			}

			if fragment.Action == "" {
				t.Error("Fragment Action should not be empty")
			}

			if fragment.Data == nil {
				t.Error("Fragment Data should not be nil")
			}

			if fragment.Metadata == nil {
				t.Error("Fragment Metadata should not be nil")
			}

			// Test metadata fields
			metadata := fragment.Metadata
			if metadata.GenerationTime <= 0 {
				t.Error("GenerationTime should be positive")
			}

			if metadata.OriginalSize <= 0 {
				t.Error("OriginalSize should be positive")
			}

			if metadata.CompressedSize < 0 {
				t.Error("CompressedSize should not be negative")
			}

			if metadata.Strategy < 1 || metadata.Strategy > 4 {
				t.Errorf("Strategy number should be 1-4, got %d", metadata.Strategy)
			}

			if metadata.Confidence <= 0 || metadata.Confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", metadata.Confidence)
			}
		})
	}
}

func TestUpdateGenerator_StrategySizes(t *testing.T) {
	generator := NewUpdateGenerator()

	// Test that different strategies produce reasonable size calculations
	tests := []struct {
		name                string
		templateText        string
		oldData             interface{}
		newData             interface{}
		expectSizeReduction bool
	}{
		{
			name:                "text change should reduce size",
			templateText:        `<div class="large-container"><h1>Title</h1><p>{{.Text}}</p><footer>Footer content</footer></div>`,
			oldData:             map[string]interface{}{"Text": "Short"},
			newData:             map[string]interface{}{"Text": "Longer text content"},
			expectSizeReduction: false, // New content is larger
		},
		{
			name:                "small change in large template",
			templateText:        `<div class="large-container"><h1>Static Title</h1><p>Large amount of static content that doesn't change</p><span>{{.Value}}</span><footer>More static content</footer></div>`,
			oldData:             map[string]interface{}{"Value": "A"},
			newData:             map[string]interface{}{"Value": "B"},
			expectSizeReduction: true, // Small change should reduce bandwidth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateText)
			if err != nil {
				t.Fatalf("Template parsing failed: %v", err)
			}

			fragments, err := generator.GenerateUpdate(tmpl, tt.oldData, tt.newData)
			if err != nil {
				t.Fatalf("Update generation failed: %v", err)
			}

			if len(fragments) == 0 {
				t.Fatal("Expected at least one fragment")
			}

			fragment := fragments[0]
			if fragment.Metadata == nil {
				t.Fatal("Fragment metadata should not be nil")
			}

			metadata := fragment.Metadata

			t.Logf("Strategy: %s, Original: %d bytes, Compressed: %d bytes, Ratio: %.2f",
				fragment.Strategy, metadata.OriginalSize, metadata.CompressedSize, metadata.CompressionRatio)

			if tt.expectSizeReduction {
				if metadata.CompressionRatio >= 1.0 {
					t.Errorf("Expected size reduction, but ratio is %f (>= 1.0)", metadata.CompressionRatio)
				}
			}

			// Ensure reasonable bounds
			if metadata.CompressionRatio < 0 {
				t.Errorf("Compression ratio should not be negative: %f", metadata.CompressionRatio)
			}
		})
	}
}

// Benchmark update generation performance
func BenchmarkUpdateGenerator_GenerateUpdate(b *testing.B) {
	generator := NewUpdateGenerator()

	tmpl, err := template.New("test").Parse(`<div class="{{.Class}}"><h1>{{.Title}}</h1><p>{{.Content}}</p></div>`)
	if err != nil {
		b.Fatal(err)
	}

	oldData := map[string]interface{}{
		"Class":   "old",
		"Title":   "Old Title",
		"Content": "Old content here",
	}
	newData := map[string]interface{}{
		"Class":   "new",
		"Title":   "New Title",
		"Content": "New content here",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateUpdate(tmpl, oldData, newData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpdateGenerator_TemplateRendering(b *testing.B) {
	generator := NewUpdateGenerator()

	tmpl, err := template.New("test").Parse(`<div><h1>{{.Title}}</h1><p>{{.Content}}</p></div>`)
	if err != nil {
		b.Fatal(err)
	}

	oldData := map[string]interface{}{"Title": "Title", "Content": "Content"}
	newData := map[string]interface{}{"Title": "New Title", "Content": "New Content"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := generator.renderTemplates(tmpl, oldData, newData)
		if err != nil {
			b.Fatal(err)
		}
	}
}
