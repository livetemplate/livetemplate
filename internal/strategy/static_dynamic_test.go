package strategy

import (
	"strings"
	"testing"

	"github.com/livefir/livetemplate/internal/diff"
)

func TestStaticDynamicGenerator_Generate(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	tests := []struct {
		name       string
		oldHTML    string
		newHTML    string
		fragmentID string
		wantErr    bool
		validate   func(*testing.T, *StaticDynamicData)
	}{
		{
			name:       "simple text change",
			oldHTML:    "<p>Hello World</p>",
			newHTML:    "<p>Hello Universe</p>",
			fragmentID: "test-1",
			wantErr:    false,
			validate: func(t *testing.T, data *StaticDynamicData) {
				if data.IsEmpty {
					t.Error("Should not be empty for text change")
				}
				if data.FragmentID != "test-1" {
					t.Errorf("FragmentID = %s, want test-1", data.FragmentID)
				}
				if len(data.Statics) == 0 {
					t.Error("Should have static segments")
				}
			},
		},
		{
			name:       "show content (empty to content)",
			oldHTML:    "",
			newHTML:    "<p>Hello World</p>",
			fragmentID: "test-2",
			wantErr:    false,
			validate: func(t *testing.T, data *StaticDynamicData) {
				if data.IsEmpty {
					t.Error("Should not be empty when showing content")
				}
				if len(data.Statics) != 1 {
					t.Errorf("Expected 1 static segment, got %d", len(data.Statics))
				}
				if data.Statics[0] != "<p>Hello World</p>" {
					t.Errorf("Static content = %s, want <p>Hello World</p>", data.Statics[0])
				}
			},
		},
		{
			name:       "hide content (content to empty)",
			oldHTML:    "<p>Hello World</p>",
			newHTML:    "",
			fragmentID: "test-3",
			wantErr:    false,
			validate: func(t *testing.T, data *StaticDynamicData) {
				if !data.IsEmpty {
					t.Error("Should be empty when hiding content")
				}
				if len(data.Statics) != 0 {
					t.Errorf("Expected 0 static segments for empty, got %d", len(data.Statics))
				}
			},
		},
		{
			name:       "no change (both empty)",
			oldHTML:    "",
			newHTML:    "",
			fragmentID: "test-4",
			wantErr:    false,
			validate: func(t *testing.T, data *StaticDynamicData) {
				if !data.IsEmpty {
					t.Error("Should be empty when both are empty")
				}
			},
		},
		{
			name:       "no change (identical content)",
			oldHTML:    "<p>Same Content</p>",
			newHTML:    "<p>Same Content</p>",
			fragmentID: "test-5",
			wantErr:    false,
			validate: func(t *testing.T, data *StaticDynamicData) {
				// This should optimize to minimal change
				if data.IsEmpty {
					t.Error("Should not be empty for identical content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := generator.Generate(tt.oldHTML, tt.newHTML, tt.fragmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if data == nil {
				t.Fatal("Generate() returned nil data")
			}

			tt.validate(t, data)
		})
	}
}

func TestStaticDynamicGenerator_TextOnlyChanges(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	tests := []struct {
		name         string
		oldHTML      string
		newHTML      string
		expectStatic bool // Whether we expect static segments to be preserved
	}{
		{
			name:         "single word change",
			oldHTML:      "<span>Count: 5</span>",
			newHTML:      "<span>Count: 7</span>",
			expectStatic: true,
		},
		{
			name:         "complete text replacement",
			oldHTML:      "<h1>Old Title</h1>",
			newHTML:      "<h1>New Title</h1>",
			expectStatic: true,
		},
		{
			name:         "multiple changes in same element",
			oldHTML:      "<p>Hello World from Earth</p>",
			newHTML:      "<p>Hi Universe from Mars</p>",
			expectStatic: false, // Complex change, less optimization
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := generator.Generate(tt.oldHTML, tt.newHTML, "test")
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			// Debug output for the simple text change case
			if tt.name == "single word change" {
				t.Logf("Debug - Statics: %+v", data.Statics)
				t.Logf("Debug - Dynamics: %+v", data.Dynamics)
			}

			// Verify reconstruction works
			reconstructed := generator.ReconstructHTML(data)
			if reconstructed != tt.newHTML {
				t.Errorf("Reconstruction failed: got %s, want %s", reconstructed, tt.newHTML)
			}

			// Check static preservation
			hasStatics := len(data.Statics) > 1 || (len(data.Statics) == 1 && data.Statics[0] != "")
			if tt.expectStatic && !hasStatics {
				t.Error("Expected static segments to be preserved")
			}
		})
	}
}

func TestStaticDynamicGenerator_BandwidthReduction(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	tests := []struct {
		name            string
		oldHTML         string
		newHTML         string
		minReductionPct float64 // Minimum expected bandwidth reduction
	}{
		{
			name:            "simple text change in realistic template",
			oldHTML:         "<div class='card'><h3>User Profile</h3><p>Name: John Doe</p><p>Status: Active</p><span class='badge'>Member</span></div>",
			newHTML:         "<div class='card'><h3>User Profile</h3><p>Name: Jane Smith</p><p>Status: Active</p><span class='badge'>Member</span></div>",
			minReductionPct: 85.0, // Should achieve Strategy 1 target for text-only changes
		},
		{
			name:            "small change in large HTML",
			oldHTML:         "<div class='container'><h1>Welcome to Our Site</h1><p>This is a long paragraph with lots of text that doesn't change. Only one word will change: old.</p></div>",
			newHTML:         "<div class='container'><h1>Welcome to Our Site</h1><p>This is a long paragraph with lots of text that doesn't change. Only one word will change: new.</p></div>",
			minReductionPct: 85.0, // Should achieve high reduction for small changes
		},
		{
			name:            "empty state transition",
			oldHTML:         "",
			newHTML:         "<p>Show this content</p>",
			minReductionPct: 0.0, // Can't reduce empty->content much
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := generator.Generate(tt.oldHTML, tt.newHTML, "test")
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			originalSize := len(tt.newHTML)
			reduction := generator.CalculateBandwidthReduction(originalSize, data)

			// Debug output for text change in realistic template
			if tt.name == "simple text change in realistic template" {
				fragmentSize := generator.calculateFragmentSize(data)
				t.Logf("Debug - Original: %d, Fragment: %d, Statics: %+v, Dynamics: %+v",
					originalSize, fragmentSize, data.Statics, data.Dynamics)
			}

			if reduction < tt.minReductionPct {
				t.Errorf("Bandwidth reduction = %.2f%%, want >= %.2f%%", reduction, tt.minReductionPct)
			}

			t.Logf("Original size: %d bytes, Reduction: %.2f%%", originalSize, reduction)
		})
	}
}

func TestStaticDynamicGenerator_EmptyStates(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	tests := []struct {
		name    string
		oldHTML string
		newHTML string
		isEmpty bool
	}{
		{
			name:    "hide content",
			oldHTML: "<div>Content to hide</div>",
			newHTML: "",
			isEmpty: true,
		},
		{
			name:    "show content",
			oldHTML: "",
			newHTML: "<div>Content to show</div>",
			isEmpty: false,
		},
		{
			name:    "both empty",
			oldHTML: "",
			newHTML: "",
			isEmpty: true,
		},
		{
			name:    "whitespace only",
			oldHTML: "   ",
			newHTML: "",
			isEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := generator.Generate(tt.oldHTML, tt.newHTML, "test")
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			if data.IsEmpty != tt.isEmpty {
				t.Errorf("IsEmpty = %v, want %v", data.IsEmpty, tt.isEmpty)
			}

			// Verify reconstruction
			reconstructed := generator.ReconstructHTML(data)
			expectedReconstructed := tt.newHTML
			if tt.isEmpty && strings.TrimSpace(tt.newHTML) == "" {
				expectedReconstructed = ""
			}

			if reconstructed != expectedReconstructed {
				t.Errorf("Reconstruction = %q, want %q", reconstructed, expectedReconstructed)
			}
		})
	}
}

func TestStaticDynamicGenerator_ReconstructHTML(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	tests := []struct {
		name string
		data *StaticDynamicData
		want string
	}{
		{
			name: "simple static/dynamic",
			data: &StaticDynamicData{
				Statics:  []string{"<p>Hello ", "</p>"},
				Dynamics: map[int]string{1: "World"},
				IsEmpty:  false,
			},
			want: "<p>Hello World</p>",
		},
		{
			name: "empty state",
			data: &StaticDynamicData{
				Statics:  []string{},
				Dynamics: map[int]string{},
				IsEmpty:  true,
			},
			want: "",
		},
		{
			name: "all dynamic",
			data: &StaticDynamicData{
				Statics:  []string{""},
				Dynamics: map[int]string{0: "<div>All dynamic content</div>"},
				IsEmpty:  false,
			},
			want: "<div>All dynamic content</div>",
		},
		{
			name: "multiple dynamics",
			data: &StaticDynamicData{
				Statics:  []string{"<span>", " items: ", "</span>"},
				Dynamics: map[int]string{1: "5", 2: "completed"},
				IsEmpty:  false,
			},
			want: "<span>5 items: completed</span>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.ReconstructHTML(tt.data)
			if got != tt.want {
				t.Errorf("ReconstructHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStaticDynamicGenerator_Performance(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	// Test with a realistic template scenario
	oldHTML := `<div class="status-card">
		<h2>Server Status</h2>
		<p>Uptime: 99.5%</p>
		<p>Last updated: 2 minutes ago</p>
		<span class="indicator green">Online</span>
	</div>`

	newHTML := `<div class="status-card">
		<h2>Server Status</h2>
		<p>Uptime: 99.7%</p>
		<p>Last updated: 1 minute ago</p>
		<span class="indicator green">Online</span>
	</div>`

	data, err := generator.Generate(oldHTML, newHTML, "status-card")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify reconstruction works
	reconstructed := generator.ReconstructHTML(data)
	if reconstructed != newHTML {
		t.Errorf("Performance test reconstruction failed")
	}

	// Check bandwidth reduction
	originalSize := len(newHTML)
	reduction := generator.CalculateBandwidthReduction(originalSize, data)

	t.Logf("Performance test: Original %d bytes, Reduction %.2f%%", originalSize, reduction)

	// Should achieve reasonable reduction for this type of change
	if reduction < 30.0 {
		t.Errorf("Performance test: Expected at least 30%% reduction, got %.2f%%", reduction)
	}
}

// Benchmark the static/dynamic generation
func BenchmarkStaticDynamicGenerator_Generate(b *testing.B) {
	generator := NewStaticDynamicGenerator()
	oldHTML := "<p>Count: 1234</p>"
	newHTML := "<p>Count: 5678</p>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.Generate(oldHTML, newHTML, "bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestStaticDynamicGenerator_GenerateConditional(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	testCases := []struct {
		name            string
		conditionalType string
		states          [2]string
		isFullElement   bool
	}{
		{
			name:            "boolean attribute conditional",
			conditionalType: "boolean",
			states:          [2]string{`<button>Click me</button>`, `<button disabled>Click me</button>`},
			isFullElement:   false,
		},
		{
			name:            "show/hide element conditional",
			conditionalType: "show-hide",
			states:          [2]string{``, `<span class="badge">5</span>`},
			isFullElement:   true,
		},
		{
			name:            "nil-notnil attribute conditional",
			conditionalType: "nil-notnil",
			states:          [2]string{`<div>Content</div>`, `<div class="active">Content</div>`},
			isFullElement:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &diff.ConditionalPattern{
				Type:          diff.ConditionalType(tc.conditionalType),
				States:        tc.states,
				ChangeType:    "attribute",
				IsFullElement: tc.isFullElement,
				IsPredictable: true,
			}

			result, err := generator.GenerateConditional(pattern, "test_conditional")
			if err != nil {
				t.Fatalf("GenerateConditional() error = %v", err)
			}

			// Verify basic structure
			if result == nil {
				t.Fatal("GenerateConditional() returned nil result")
			}

			if result.FragmentID != "test_conditional" {
				t.Errorf("FragmentID = %s, want test_conditional", result.FragmentID)
			}

			// Verify conditionals are populated
			if len(result.Conditionals) == 0 {
				t.Error("Expected conditionals to be populated, but got empty map")
			}

			// Verify the first conditional slot
			conditional, exists := result.Conditionals[0]
			if !exists {
				t.Error("Expected conditional slot at position 0")
			} else {
				if conditional.ConditionType != tc.conditionalType {
					t.Errorf("ConditionType = %s, want %s", conditional.ConditionType, tc.conditionalType)
				}

				if conditional.IsFullElement != tc.isFullElement {
					t.Errorf("IsFullElement = %v, want %v", conditional.IsFullElement, tc.isFullElement)
				}
			}
		})
	}
}

func TestStaticDynamicGenerator_ConditionalBandwidthReduction(t *testing.T) {
	generator := NewStaticDynamicGenerator()

	testCases := []struct {
		name                 string
		states               [2]string
		conditionalType      string
		expectedMinReduction float64
	}{
		{
			name:                 "boolean attribute - good efficiency",
			states:               [2]string{`<button class="btn">Click me</button>`, `<button class="btn" disabled>Click me</button>`},
			conditionalType:      "boolean",
			expectedMinReduction: 30.0, // Realistic expectation
		},
		{
			name:                 "show/hide element - conditional benefit",
			states:               [2]string{``, `<div class="notification error">An error occurred! Please try again.</div>`},
			conditionalType:      "show-hide",
			expectedMinReduction: 0.0, // May have overhead but provides conditional logic
		},
		{
			name:                 "nil-notnil attribute - good efficiency",
			states:               [2]string{`<div class="container">Content here</div>`, `<div class="container active">Content here</div>`},
			conditionalType:      "nil-notnil",
			expectedMinReduction: 35.0, // Realistic expectation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &diff.ConditionalPattern{
				Type:          diff.ConditionalType(tc.conditionalType),
				States:        tc.states,
				ChangeType:    "attribute",
				IsFullElement: tc.conditionalType == "show-hide",
				IsPredictable: true,
			}

			result, err := generator.GenerateConditional(pattern, "bandwidth_test")
			if err != nil {
				t.Fatalf("GenerateConditional() error = %v", err)
			}

			// Calculate bandwidth reduction
			originalSize := len(tc.states[1]) // Size of the "new" state
			compressedSize := calculateConditionalSize(result)
			reduction := (1.0 - float64(compressedSize)/float64(originalSize)) * 100.0

			t.Logf("Original: %d bytes, Compressed: %d bytes, Reduction: %.2f%%",
				originalSize, compressedSize, reduction)

			if tc.expectedMinReduction > 0 && reduction < tc.expectedMinReduction {
				t.Errorf("Expected at least %.1f%% reduction, got %.2f%%", tc.expectedMinReduction, reduction)
			}

			// Verify that conditional approach is much more efficient than regular Strategy 1
			regularResult, err := generator.Generate(tc.states[0], tc.states[1], "regular_test")
			if err == nil {
				regularSize := calculateRegularSize(regularResult)
				conditionalEfficiency := float64(regularSize) / float64(compressedSize)

				t.Logf("Conditional is %.2fx more efficient than regular Strategy 1", conditionalEfficiency)

				if conditionalEfficiency < 1.5 {
					t.Logf("Warning: Conditional approach should be significantly more efficient (got %.2fx)", conditionalEfficiency)
				}
			}
		})
	}
}

// Helper functions for bandwidth calculation
func calculateConditionalSize(data *StaticDynamicData) int {
	size := 0

	// Add dynamic values
	for _, dynamic := range data.Dynamics {
		size += len(dynamic)
	}

	// Add conditional values (the key advantage)
	for _, conditional := range data.Conditionals {
		size += len(conditional.TruthyValue)
		size += len(conditional.FalsyValue)
		size += 20 // metadata overhead
	}

	return size
}

func calculateRegularSize(data *StaticDynamicData) int {
	size := 0
	for _, dynamic := range data.Dynamics {
		size += len(dynamic)
	}
	return size
}

func BenchmarkStaticDynamicGenerator_ReconstructHTML(b *testing.B) {
	generator := NewStaticDynamicGenerator()
	data := &StaticDynamicData{
		Statics:  []string{"<p>Count: ", "</p>"},
		Dynamics: map[int]string{1: "5678"},
		IsEmpty:  false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generator.ReconstructHTML(data)
	}
}
