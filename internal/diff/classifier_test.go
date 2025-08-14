package diff

import (
	"testing"
)

func TestPatternClassifier_ClassifyPattern(t *testing.T) {
	classifier := NewPatternClassifier()

	tests := []struct {
		name             string
		oldHTML          string
		newHTML          string
		expectedStrategy int
		expectedPattern  PatternType
	}{
		{
			name:             "no changes",
			oldHTML:          "<p>Hello World</p>",
			newHTML:          "<p>Hello World</p>",
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "simple text change",
			oldHTML:          "<p>Hello World</p>",
			newHTML:          "<p>Hello Universe</p>",
			expectedStrategy: 1, // Simple text changes are optimal for static/dynamic
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "multiple text changes",
			oldHTML:          "<div><p>Hello</p><span>World</span></div>",
			newHTML:          "<div><p>Hi</p><span>Universe</span></div>",
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "attribute change",
			oldHTML:          `<p class="old">Hello</p>`,
			newHTML:          `<p class="new">Hello</p>`,
			expectedStrategy: 2,
			expectedPattern:  PatternMarkerizable,
		},
		{
			name:             "mixed text and attribute",
			oldHTML:          `<p class="old">Hello</p>`,
			newHTML:          `<p class="new">Hi</p>`,
			expectedStrategy: 2,
			expectedPattern:  PatternMarkerizable,
		},
		{
			name:             "structural change",
			oldHTML:          "<div>Hello</div>",
			newHTML:          "<div>Hello<span>World</span></div>",
			expectedStrategy: 3,
			expectedPattern:  PatternGranular,
		},
		{
			name:             "complex changes",
			oldHTML:          `<div class="old"><p>Hello</p></div>`,
			newHTML:          `<section class="new"><h1>Hi</h1><p>World</p></section>`,
			expectedStrategy: 3, // Element tag change is detected as pure structural
			expectedPattern:  PatternGranular,
		},
		{
			name:             "empty state - show content",
			oldHTML:          "<div></div>",
			newHTML:          "<div><p>Hello</p></div>",
			expectedStrategy: 1, // Empty states are great for static/dynamic
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "empty state - hide content",
			oldHTML:          "<div><p>Hello</p></div>",
			newHTML:          "<div></div>",
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec == nil {
				t.Fatal("expected non-nil recommendation")
			}

			if rec.Strategy != tt.expectedStrategy {
				t.Errorf("ClassifyPattern() strategy = %d, want %d", rec.Strategy, tt.expectedStrategy)
			}

			if rec.Pattern != tt.expectedPattern {
				t.Errorf("ClassifyPattern() pattern = %v, want %v", rec.Pattern, tt.expectedPattern)
			}

			if rec.Reason == "" {
				t.Error("ClassifyPattern() should include a reason")
			}
		})
	}
}

func TestPatternClassifier_StaticDynamicDetection(t *testing.T) {
	classifier := NewPatternClassifier()

	// Test cases that should be detected as static/dynamic pattern
	staticDynamicCases := []struct {
		name    string
		oldHTML string
		newHTML string
	}{
		{
			name:    "single text replacement",
			oldHTML: "<h1>Welcome John</h1>",
			newHTML: "<h1>Welcome Jane</h1>",
		},
		{
			name:    "multiple text replacements",
			oldHTML: "<div><span>Count: 5</span><p>Items: 10</p></div>",
			newHTML: "<div><span>Count: 7</span><p>Items: 12</p></div>",
		},
		{
			name:    "conditional show",
			oldHTML: "<div></div>",
			newHTML: "<div><p class=\"message\">Success!</p></div>",
		},
		{
			name:    "conditional hide",
			oldHTML: "<div><p class=\"error\">Failed!</p></div>",
			newHTML: "<div></div>",
		},
	}

	for _, tc := range staticDynamicCases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec.Strategy != 1 {
				t.Errorf("expected strategy 1 for static/dynamic case, got %d", rec.Strategy)
			}

			if rec.Pattern != PatternStaticDynamic {
				t.Errorf("expected PatternStaticDynamic, got %v", rec.Pattern)
			}
		})
	}
}

func TestPatternClassifier_MarkerizableDetection(t *testing.T) {
	classifier := NewPatternClassifier()

	// Test cases that should be detected as markerizable
	markerizableCases := []struct {
		name    string
		oldHTML string
		newHTML string
	}{
		{
			name:    "class change",
			oldHTML: `<div class="inactive">Content</div>`,
			newHTML: `<div class="active">Content</div>`,
		},
		{
			name:    "attribute addition",
			oldHTML: `<button>Click me</button>`,
			newHTML: `<button disabled>Click me</button>`,
		},
		{
			name:    "mixed attribute and text",
			oldHTML: `<span class="count">5</span>`,
			newHTML: `<span class="count updated">7</span>`,
		},
	}

	for _, tc := range markerizableCases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec.Strategy != 2 {
				t.Errorf("expected strategy 2 for markerizable case, got %d", rec.Strategy)
			}

			if rec.Pattern != PatternMarkerizable {
				t.Errorf("expected PatternMarkerizable, got %v", rec.Pattern)
			}
		})
	}
}

func TestPatternClassifier_EdgeCases(t *testing.T) {
	classifier := NewPatternClassifier()

	t.Run("identical HTML", func(t *testing.T) {
		html := "<div><p>Same content</p></div>"
		rec, err := classifier.ClassifyPattern(html, html)
		if err != nil {
			t.Fatalf("ClassifyPattern() error = %v", err)
		}

		if rec.Strategy != 1 {
			t.Errorf("identical HTML should use strategy 1, got %d", rec.Strategy)
		}

		// For identical HTML, we expect a deterministic strategy 1 result
	})

	t.Run("empty HTML", func(t *testing.T) {
		rec, err := classifier.ClassifyPattern("", "<p>Hello</p>")
		if err == nil {
			t.Error("expected error for empty HTML")
		}
		if rec != nil {
			t.Error("expected nil recommendation for invalid input")
		}
	})

	t.Run("invalid HTML", func(t *testing.T) {
		// HTML parser is quite forgiving, so this might not error
		rec, err := classifier.ClassifyPattern("<invalid>", "<p>Valid</p>")
		if err != nil {
			// It's okay if this errors due to parsing issues
			return
		}

		// If it doesn't error, it should at least provide a recommendation
		if rec == nil {
			t.Error("expected a recommendation even for malformed HTML")
		}
	})
}

func TestPatternClassifier_ReasonGeneration(t *testing.T) {
	classifier := NewPatternClassifier()

	testCases := []struct {
		name    string
		oldHTML string
		newHTML string
	}{
		{
			name:    "text change",
			oldHTML: "<p>Hello</p>",
			newHTML: "<p>Hi</p>",
		},
		{
			name:    "attribute change",
			oldHTML: `<div class="old">Content</div>`,
			newHTML: `<div class="new">Content</div>`,
		},
		{
			name:    "structural change",
			oldHTML: "<div>Hello</div>",
			newHTML: "<div>Hello<span>World</span></div>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec.Reason == "" {
				t.Error("recommendation should include a descriptive reason")
			}

			// Reason should be informative
			if len(rec.Reason) < 10 {
				t.Errorf("reason seems too short: %s", rec.Reason)
			}
		})
	}
}
