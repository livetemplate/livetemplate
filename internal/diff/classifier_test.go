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
			name:             "attribute value change",
			oldHTML:          `<p class="old">Hello</p>`,
			newHTML:          `<p class="new">Hello</p>`,
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "mixed text and attribute value",
			oldHTML:          `<p class="old">Hello</p>`,
			newHTML:          `<p class="new">Hi</p>`,
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "attribute structure change - now conditional",
			oldHTML:          `<p>Hello</p>`,
			newHTML:          `<p class="new">Hello</p>`,
			expectedStrategy: 1,
			expectedPattern:  PatternConditionalStatic,
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
			expectedStrategy: 1, // Empty states are detected as static/dynamic (container based)
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
			name:    "text replacement only",
			oldHTML: "<div>Hello World</div>",
			newHTML: "<div>Hello Universe</div>",
		},
		{
			name:    "number change",
			oldHTML: "<span>5</span>",
			newHTML: "<span>7</span>",
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

	// Test cases that should be detected as markerizable (Strategy 2)
	markerizableCases := []struct {
		name             string
		oldHTML          string
		newHTML          string
		expectedStrategy int
		expectedPattern  PatternType
	}{
		{
			name:             "class value change (now Strategy 1)",
			oldHTML:          `<div class="inactive">Content</div>`,
			newHTML:          `<div class="active">Content</div>`,
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
		{
			name:             "attribute addition (now enhanced Strategy 1)",
			oldHTML:          `<button>Click me</button>`,
			newHTML:          `<button disabled>Click me</button>`,
			expectedStrategy: 1,
			expectedPattern:  PatternConditionalStatic,
		},
		{
			name:             "mixed attribute value and text (now Strategy 1)",
			oldHTML:          `<span class="count">5</span>`,
			newHTML:          `<span class="count updated">7</span>`,
			expectedStrategy: 1,
			expectedPattern:  PatternStaticDynamic,
		},
	}

	for _, tc := range markerizableCases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec.Strategy != tc.expectedStrategy {
				t.Errorf("expected strategy %d, got %d", tc.expectedStrategy, rec.Strategy)
			}

			if rec.Pattern != tc.expectedPattern {
				t.Errorf("expected %v, got %v", tc.expectedPattern, rec.Pattern)
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

	t.Run("empty HTML - now allowed for show/hide", func(t *testing.T) {
		rec, err := classifier.ClassifyPattern("", "<p>Hello</p>")
		if err != nil {
			t.Errorf("unexpected error for empty HTML: %v", err)
		}
		if rec == nil {
			t.Error("expected recommendation for show pattern")
		} else {
			if rec.Strategy != 1 {
				t.Errorf("expected strategy 1 for show pattern, got %d", rec.Strategy)
			}
			if rec.Pattern != PatternConditionalStatic {
				t.Errorf("expected conditional-static pattern, got %v", rec.Pattern)
			}
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

func TestPatternClassifier_ConditionalPatterns(t *testing.T) {
	classifier := NewPatternClassifier()

	testCases := []struct {
		name                     string
		oldHTML                  string
		newHTML                  string
		expectedStrategy         int
		expectedPattern          PatternType
		expectConditionalPattern bool
		expectedConditionalType  string
	}{
		{
			name:                     "boolean attribute addition",
			oldHTML:                  `<button>Click me</button>`,
			newHTML:                  `<button disabled>Click me</button>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "boolean",
		},
		{
			name:                     "boolean attribute removal",
			oldHTML:                  `<button disabled>Click me</button>`,
			newHTML:                  `<button>Click me</button>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "boolean",
		},
		{
			name:                     "show/hide element",
			oldHTML:                  ``,
			newHTML:                  `<span class="badge">5</span>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "show-hide",
		},
		{
			name:                     "hide/show element",
			oldHTML:                  `<span class="badge">5</span>`,
			newHTML:                  ``,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "show-hide",
		},
		{
			name:                     "nil to value attribute",
			oldHTML:                  `<div>Content</div>`,
			newHTML:                  `<div class="active">Content</div>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "nil-notnil",
		},
		{
			name:                     "value to nil attribute",
			oldHTML:                  `<div class="active">Content</div>`,
			newHTML:                  `<div>Content</div>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "nil-notnil",
		},
		{
			name:                     "boolean class conditional",
			oldHTML:                  `<div>Inactive</div>`,
			newHTML:                  `<div class="active">Inactive</div>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "nil-notnil",
		},
		{
			name:                     "complex changes should not be conditional",
			oldHTML:                  `<div class="old">Old text</div>`,
			newHTML:                  `<span class="new">New text</span>`,
			expectedStrategy:         3, // This is actually Strategy 3 (granular) since it's primarily structural
			expectedPattern:          PatternGranular,
			expectConditionalPattern: false,
		},
		{
			name:                     "multiple attribute changes should not be conditional",
			oldHTML:                  `<div class="old" id="test">Content</div>`,
			newHTML:                  `<div class="new" id="changed" data-value="added">Content</div>`,
			expectedStrategy:         2,
			expectedPattern:          PatternMarkerizable,
			expectConditionalPattern: false,
		},
		{
			name:                     "if-else structural conditional",
			oldHTML:                  `<div>Data</div>`,
			newHTML:                  `<table><tr><td>Updated</td></tr></table>`,
			expectedStrategy:         1,
			expectedPattern:          PatternConditionalStatic,
			expectConditionalPattern: true,
			expectedConditionalType:  "if-else",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
			if err != nil {
				t.Fatalf("ClassifyPattern() error = %v", err)
			}

			if rec.Strategy != tc.expectedStrategy {
				t.Errorf("Strategy = %d, want %d", rec.Strategy, tc.expectedStrategy)
			}

			if rec.Pattern != tc.expectedPattern {
				t.Errorf("Pattern = %v, want %v", rec.Pattern, tc.expectedPattern)
			}

			if tc.expectConditionalPattern {
				if rec.ConditionalPattern == nil {
					t.Error("Expected conditional pattern to be detected, but got nil")
				} else {
					actualType := string(rec.ConditionalPattern.Type)
					if actualType != tc.expectedConditionalType {
						t.Errorf("ConditionalType = %s, want %s", actualType, tc.expectedConditionalType)
					}

					if !rec.ConditionalPattern.IsPredictable {
						t.Error("Conditional pattern should be marked as predictable")
					}
				}
			} else {
				if rec.ConditionalPattern != nil {
					t.Errorf("Expected no conditional pattern, but got %v", rec.ConditionalPattern.Type)
				}
			}
		})
	}
}

func TestConditionalPatternGeneration(t *testing.T) {
	classifier := NewPatternClassifier()

	// Test boolean conditionals specifically
	t.Run("boolean conditionals", func(t *testing.T) {
		testCases := []struct {
			name     string
			oldHTML  string
			newHTML  string
			expected ConditionalType
		}{
			{
				name:     "disabled attribute",
				oldHTML:  `<input type="text">`,
				newHTML:  `<input type="text" disabled>`,
				expected: ConditionalBoolean,
			},
			{
				name:     "checked attribute",
				oldHTML:  `<input type="checkbox">`,
				newHTML:  `<input type="checkbox" checked>`,
				expected: ConditionalBoolean,
			},
			{
				name:     "hidden attribute",
				oldHTML:  `<div>Content</div>`,
				newHTML:  `<div hidden>Content</div>`,
				expected: ConditionalBoolean,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
				if err != nil {
					t.Fatalf("ClassifyPattern() error = %v", err)
				}

				if rec.ConditionalPattern == nil {
					t.Fatal("Expected conditional pattern to be detected")
				}

				if rec.ConditionalPattern.Type != tc.expected {
					t.Errorf("ConditionalType = %v, want %v", rec.ConditionalPattern.Type, tc.expected)
				}

				// Verify states are stored correctly
				if len(rec.ConditionalPattern.States) != 2 {
					t.Error("ConditionalPattern should have exactly 2 states")
				}
			})
		}
	})

	// Test show/hide conditionals
	t.Run("show/hide conditionals", func(t *testing.T) {
		testCases := []struct {
			name    string
			oldHTML string
			newHTML string
		}{
			{
				name:    "show notification",
				oldHTML: ``,
				newHTML: `<div class="notification">Success!</div>`,
			},
			{
				name:    "hide badge",
				oldHTML: `<span class="badge">3</span>`,
				newHTML: ``,
			},
			{
				name:    "show error message",
				oldHTML: `   `,
				newHTML: `<p class="error">Something went wrong</p>`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
				if err != nil {
					t.Fatalf("ClassifyPattern() error = %v", err)
				}

				if rec.ConditionalPattern == nil {
					t.Fatal("Expected conditional pattern to be detected")
				}

				if rec.ConditionalPattern.Type != ConditionalShowHide {
					t.Errorf("ConditionalType = %v, want %v", rec.ConditionalPattern.Type, ConditionalShowHide)
				}

				if !rec.ConditionalPattern.IsFullElement {
					t.Error("Show/hide conditionals should be marked as full element")
				}
			})
		}
	})

	// Test if-else structural conditionals
	t.Run("if-else structural conditionals", func(t *testing.T) {
		testCases := []struct {
			name    string
			oldHTML string
			newHTML string
		}{
			{
				name:    "div to table switch",
				oldHTML: `<div>Data</div>`,
				newHTML: `<table><tr><td>Updated</td></tr></table>`,
			},
			{
				name:    "list to grid switch",
				oldHTML: `<ul><li>Item</li></ul>`,
				newHTML: `<div class="grid"><div class="item">Item</div></div>`,
			},
			{
				name:    "card to table switch",
				oldHTML: `<div class="card">Content</div>`,
				newHTML: `<table><tr><td>Content</td></tr></table>`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rec, err := classifier.ClassifyPattern(tc.oldHTML, tc.newHTML)
				if err != nil {
					t.Fatalf("ClassifyPattern() error = %v", err)
				}

				if rec.ConditionalPattern == nil {
					t.Fatal("Expected conditional pattern to be detected")
				}

				if rec.ConditionalPattern.Type != ConditionalIfElse {
					t.Errorf("ConditionalType = %v, want %v", rec.ConditionalPattern.Type, ConditionalIfElse)
				}

				if !rec.ConditionalPattern.IsFullElement {
					t.Error("If-else conditionals should be marked as full element")
				}

				if !rec.ConditionalPattern.IsPredictable {
					t.Error("If-else conditionals should be marked as predictable")
				}

				// Verify states are stored correctly
				if len(rec.ConditionalPattern.States) != 2 {
					t.Error("ConditionalPattern should have exactly 2 states")
				}
			})
		}
	})
}
