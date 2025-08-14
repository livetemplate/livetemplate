package diff

import (
	"testing"
	"time"
)

func TestHTMLDiffer_Diff(t *testing.T) {
	differ := NewHTMLDiffer()

	tests := []struct {
		name    string
		oldHTML string
		newHTML string
		wantErr bool
	}{
		{
			name:    "simple text change",
			oldHTML: "<p>Hello World</p>",
			newHTML: "<p>Hello Universe</p>",
			wantErr: false,
		},
		{
			name:    "no changes",
			oldHTML: "<div><p>Same content</p></div>",
			newHTML: "<div><p>Same content</p></div>",
			wantErr: false,
		},
		{
			name:    "complex changes",
			oldHTML: `<div class="old"><p>Hello</p></div>`,
			newHTML: `<section class="new"><h1>Hi</h1><p>World</p></section>`,
			wantErr: false,
		},
		{
			name:    "empty to content",
			oldHTML: "<div></div>",
			newHTML: "<div><p>Hello</p></div>",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := differ.Diff(tt.oldHTML, tt.newHTML)
			if (err != nil) != tt.wantErr {
				t.Errorf("Diff() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Validate result structure
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Strategy == nil {
				t.Fatal("expected non-nil strategy")
			}

			// Validate metadata
			if result.Metadata.Timestamp.IsZero() {
				t.Error("expected non-zero timestamp")
			}

			if result.Metadata.OldHTMLSize != len(tt.oldHTML) {
				t.Errorf("metadata old HTML size = %d, want %d",
					result.Metadata.OldHTMLSize, len(tt.oldHTML))
			}

			if result.Metadata.NewHTMLSize != len(tt.newHTML) {
				t.Errorf("metadata new HTML size = %d, want %d",
					result.Metadata.NewHTMLSize, len(tt.newHTML))
			}

			if result.Metadata.ChangeCount != len(result.Changes) {
				t.Errorf("metadata change count = %d, want %d",
					result.Metadata.ChangeCount, len(result.Changes))
			}

			// Validate performance metrics
			if result.Performance.TotalTime <= 0 {
				t.Error("expected positive total time")
			}

			// Validate complexity classification
			if result.Metadata.Complexity == "" {
				t.Error("expected non-empty complexity classification")
			}
		})
	}
}

func TestHTMLDiffer_QuickDiff(t *testing.T) {
	differ := NewHTMLDiffer()

	oldHTML := "<p>Hello World</p>"
	newHTML := "<p>Hello Universe</p>"

	rec, err := differ.QuickDiff(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("QuickDiff() error = %v", err)
	}

	if rec == nil {
		t.Fatal("expected non-nil recommendation")
	}

	if rec.Strategy < 1 || rec.Strategy > 4 {
		t.Errorf("QuickDiff() strategy = %d, want 1-4", rec.Strategy)
	}

	// Strategy should be deterministic
	if rec.Reason == "" {
		t.Error("QuickDiff() should include a reason")
	}
}

func TestHTMLDiffer_AnalyzeChanges(t *testing.T) {
	differ := NewHTMLDiffer()

	oldHTML := "<p>Hello World</p>"
	newHTML := "<p>Hello Universe</p>"

	changes, err := differ.AnalyzeChanges(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("AnalyzeChanges() error = %v", err)
	}

	if len(changes) == 0 {
		t.Error("expected at least one change for different HTML")
	}

	// Check that changes have required fields
	for i, change := range changes {
		if change.Type == "" {
			t.Errorf("change %d missing type", i)
		}

		if change.Path == "" {
			t.Errorf("change %d missing path", i)
		}

		if change.Description == "" {
			t.Errorf("change %d has empty description", i)
		}
	}
}

func TestHTMLDiffer_ValidateStrategy(t *testing.T) {
	differ := NewHTMLDiffer()

	tests := []struct {
		name    string
		rec     *StrategyRecommendation
		wantErr bool
	}{
		{
			name: "valid recommendation",
			rec: &StrategyRecommendation{
				Strategy: 1,
				Pattern:  PatternStaticDynamic,
				Reason:   "Text only changes",
			},
			wantErr: false,
		},
		{
			name:    "nil recommendation",
			rec:     nil,
			wantErr: true,
		},
		{
			name: "invalid strategy number",
			rec: &StrategyRecommendation{
				Strategy: 5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := differ.ValidateStrategy(tt.rec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStrategy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTMLDiffer_ShouldUseStrategy(t *testing.T) {
	differ := NewHTMLDiffer()

	tests := []struct {
		name           string
		result         *DiffResult
		targetStrategy int
		want           bool
	}{
		{
			name: "exact match",
			result: &DiffResult{
				Strategy: &StrategyRecommendation{
					Strategy: 1,
				},
			},
			targetStrategy: 1,
			want:           true,
		},
		{
			name: "different strategy",
			result: &DiffResult{
				Strategy: &StrategyRecommendation{
					Strategy: 2,
				},
			},
			targetStrategy: 2,
			want:           true,
		},
		{
			name: "no match",
			result: &DiffResult{
				Strategy: &StrategyRecommendation{
					Strategy: 3,
				},
			},
			targetStrategy: 2,
			want:           false,
		},
		{
			name: "another no match",
			result: &DiffResult{
				Strategy: &StrategyRecommendation{
					Strategy: 1,
				},
			},
			targetStrategy: 2,
			want:           false,
		},
		{
			name:           "nil result",
			result:         nil,
			targetStrategy: 1,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := differ.ShouldUseStrategy(tt.result, tt.targetStrategy)
			if got != tt.want {
				t.Errorf("ShouldUseStrategy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStrategyName(t *testing.T) {
	tests := []struct {
		strategy int
		want     string
	}{
		{1, "Static/Dynamic Fragments"},
		{2, "Marker Compilation"},
		{3, "Granular Operations"},
		{4, "Fragment Replacement"},
		{0, "Unknown Strategy"},
		{5, "Unknown Strategy"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GetStrategyName(tt.strategy)
			if got != tt.want {
				t.Errorf("GetStrategyName(%d) = %s, want %s", tt.strategy, got, tt.want)
			}
		})
	}
}

func TestGetPatternName(t *testing.T) {
	tests := []struct {
		pattern PatternType
		want    string
	}{
		{PatternStaticDynamic, "Static/Dynamic Pattern"},
		{PatternMarkerizable, "Markerizable Pattern"},
		{PatternGranular, "Granular Operations Pattern"},
		{PatternReplacement, "Full Replacement Pattern"},
		{PatternUnknown, "Unknown Pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GetPatternName(tt.pattern)
			if got != tt.want {
				t.Errorf("GetPatternName(%s) = %s, want %s", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestHTMLDiffer_PerformanceMetrics(t *testing.T) {
	differ := NewHTMLDiffer()

	// Test with moderately complex HTML to ensure measurable timing
	oldHTML := `
	<div class="container">
		<h1>Title</h1>
		<p>Paragraph 1</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
		</ul>
	</div>`

	newHTML := `
	<div class="container updated">
		<h1>New Title</h1>
		<p>Paragraph 1</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
			<li>Item 3</li>
		</ul>
	</div>`

	result, err := differ.Diff(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	// Check that performance metrics are reasonable
	perf := result.Performance

	if perf.TotalTime <= 0 {
		t.Error("TotalTime should be positive")
	}

	if perf.CompareTime <= 0 {
		t.Error("CompareTime should be positive")
	}

	if perf.ClassifyTime <= 0 {
		t.Error("ClassifyTime should be positive")
	}

	// Total time should be sum of components (approximately)
	componentSum := perf.ParseTime + perf.CompareTime + perf.ClassifyTime
	if perf.TotalTime < componentSum {
		t.Error("TotalTime should be >= sum of component times")
	}

	// Performance should be reasonable (less than 100ms for simple cases)
	if perf.TotalTime > 100*time.Millisecond {
		t.Errorf("Performance seems slow: %v", perf.TotalTime)
	}
}

func TestHTMLDiffer_ComplexityClassification(t *testing.T) {
	differ := NewHTMLDiffer()

	tests := []struct {
		name               string
		oldHTML            string
		newHTML            string
		expectedComplexity string
	}{
		{
			name:               "no changes",
			oldHTML:            "<p>Same</p>",
			newHTML:            "<p>Same</p>",
			expectedComplexity: "none",
		},
		{
			name:               "simple change",
			oldHTML:            "<p>Hello</p>",
			newHTML:            "<p>Hi</p>",
			expectedComplexity: "simple",
		},
		{
			name:               "moderate changes",
			oldHTML:            `<div><p>Hello</p><span>World</span></div>`,
			newHTML:            `<div class="new"><p>Hi</p><span>Universe</span></div>`,
			expectedComplexity: "moderate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := differ.Diff(tt.oldHTML, tt.newHTML)
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}

			if result.Metadata.Complexity != tt.expectedComplexity {
				t.Errorf("complexity = %s, want %s",
					result.Metadata.Complexity, tt.expectedComplexity)
			}
		})
	}
}
