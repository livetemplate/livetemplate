package diff

import (
	"testing"
)

// TestHTMLDiffingEngine_BasicFunctionality demonstrates the core acceptance criteria
func TestHTMLDiffingEngine_BasicFunctionality(t *testing.T) {
	// Test acceptance criteria:
	// ✓ DOM parser can parse HTML into comparable tree structure
	// ✓ DOM comparator can identify differences between two HTML trees
	// ✓ Pattern classifier can categorize changes as text-only vs structural vs complex
	// ✓ Deterministic strategy selection based on change patterns
	// ✓ Basic HTML diff analysis returns structured diff results

	t.Run("DOM Parser - parse HTML into tree structure", func(t *testing.T) {
		parser := NewDOMParser()

		html := "<div><p>Hello World</p></div>"
		node, err := parser.ParseFragment(html)

		if err != nil {
			t.Fatalf("DOM parser should parse valid HTML: %v", err)
		}

		if node == nil {
			t.Fatal("DOM parser should return non-nil node")
		}

		// Verify tree structure exists
		if len(node.Children) == 0 {
			t.Error("DOM parser should create tree structure with children")
		}
	})

	t.Run("DOM Comparator - identify differences", func(t *testing.T) {
		comparator := NewDOMComparator()

		oldHTML := "<p>Hello World</p>"
		newHTML := "<p>Hello Universe</p>"

		changes, err := comparator.Compare(oldHTML, newHTML)

		if err != nil {
			t.Fatalf("DOM comparator should handle valid HTML: %v", err)
		}

		if len(changes) == 0 {
			t.Error("DOM comparator should detect differences between different HTML")
		}

		// Verify change structure
		for _, change := range changes {
			if change.Type == "" {
				t.Error("DOM change should have a type")
			}
			if change.Path == "" {
				t.Error("DOM change should have a path")
			}
		}
	})

	t.Run("Pattern Classifier - categorize changes", func(t *testing.T) {
		classifier := NewPatternClassifier()

		testCases := []struct {
			name    string
			oldHTML string
			newHTML string
		}{
			{"text change", "<p>Hello</p>", "<p>Hi</p>"},
			{"attribute change", `<div class="old">Hello</div>`, `<div class="new">Hello</div>`},
			{"structural change", "<div>Hello</div>", "<div>Hello<span>World</span></div>"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)

				if err != nil {
					t.Fatalf("Pattern classifier should handle valid HTML: %v", err)
				}

				if rec == nil {
					t.Fatal("Pattern classifier should return recommendation")
				}

				// Verify recommendation structure
				if rec.Strategy < 1 || rec.Strategy > 4 {
					t.Errorf("Strategy should be 1-4, got %d", rec.Strategy)
				}

				if rec.Pattern == "" {
					t.Error("Recommendation should have pattern type")
				}

				if rec.Reason == "" {
					t.Error("Recommendation should have reason")
				}
			})
		}
	})

	t.Run("Deterministic Strategy Selection", func(t *testing.T) {
		classifier := NewPatternClassifier()

		rec1, err := classifier.ClassifyPattern("<p>Hello</p>", "<p>Hi</p>")
		if err != nil {
			t.Fatalf("Classifier error: %v", err)
		}

		// Same input should produce same result (deterministic)
		rec2, err := classifier.ClassifyPattern("<p>Hello</p>", "<p>Hi</p>")
		if err != nil {
			t.Fatalf("Classifier error: %v", err)
		}

		if rec1.Strategy != rec2.Strategy {
			t.Error("Same input should produce same strategy (deterministic)")
		}

		if rec1.Pattern != rec2.Pattern {
			t.Error("Same input should produce same pattern (deterministic)")
		}

		// Text-only changes should always select Strategy 1
		if rec1.Strategy != 1 {
			t.Errorf("Text-only changes should always select Strategy 1, got %d", rec1.Strategy)
		}
	})

	t.Run("Structured Diff Results", func(t *testing.T) {
		differ := NewHTMLDiffer()

		oldHTML := "<div><p>Hello World</p></div>"
		newHTML := "<div><p>Hello Universe</p></div>"

		result, err := differ.Diff(oldHTML, newHTML)

		if err != nil {
			t.Fatalf("HTML differ should handle valid HTML: %v", err)
		}

		if result == nil {
			t.Fatal("HTML differ should return structured result")
		}

		// Verify result structure
		if result.Changes == nil {
			t.Error("Result should include changes array")
		}

		if result.Strategy == nil {
			t.Error("Result should include strategy recommendation")
		}

		if result.Metadata.Timestamp.IsZero() {
			t.Error("Result should include metadata with timestamp")
		}

		if result.Performance.TotalTime <= 0 {
			t.Error("Result should include performance metrics")
		}

		// Verify metadata accuracy
		if result.Metadata.OldHTMLSize != len(oldHTML) {
			t.Error("Metadata should track old HTML size correctly")
		}

		if result.Metadata.NewHTMLSize != len(newHTML) {
			t.Error("Metadata should track new HTML size correctly")
		}

		if result.Metadata.ChangeCount != len(result.Changes) {
			t.Error("Metadata should track change count correctly")
		}
	})

	t.Run("Major HTML Patterns Coverage", func(t *testing.T) {
		differ := NewHTMLDiffer()

		patterns := []struct {
			name    string
			oldHTML string
			newHTML string
		}{
			{
				"Simple text substitution",
				"<p>Count: 5</p>",
				"<p>Count: 7</p>",
			},
			{
				"Class toggle",
				`<button class="inactive">Click</button>`,
				`<button class="active">Click</button>`,
			},
			{
				"Content show/hide",
				"<div></div>",
				"<div><p>Message</p></div>",
			},
			{
				"Element addition",
				"<ul><li>Item 1</li></ul>",
				"<ul><li>Item 1</li><li>Item 2</li></ul>",
			},
			{
				"Complex restructure",
				`<div class="old"><p>Hello</p></div>`,
				`<section class="new"><h1>Hi</h1><p>World</p></section>`,
			},
		}

		for _, pattern := range patterns {
			t.Run(pattern.name, func(t *testing.T) {
				result, err := differ.Diff(pattern.oldHTML, pattern.newHTML)

				if err != nil {
					t.Errorf("Should handle pattern %s: %v", pattern.name, err)
					return
				}

				if result == nil {
					t.Errorf("Should return result for pattern %s", pattern.name)
					return
				}

				// Verify a strategy was selected
				if result.Strategy.Strategy < 1 || result.Strategy.Strategy > 4 {
					t.Errorf("Should select valid strategy for pattern %s", pattern.name)
				}
			})
		}
	})
}
