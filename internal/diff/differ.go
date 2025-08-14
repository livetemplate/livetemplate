package diff

import (
	"fmt"
	"time"
)

// DiffResult represents the complete result of HTML diffing analysis
type DiffResult struct {
	Changes     []DOMChange             `json:"changes"`
	Strategy    *StrategyRecommendation `json:"strategy"`
	Metadata    DiffMetadata            `json:"metadata"`
	Performance PerformanceMetrics      `json:"performance"`
}

// DiffMetadata contains metadata about the diff operation
type DiffMetadata struct {
	Timestamp   time.Time `json:"timestamp"`
	OldHTMLSize int       `json:"old_html_size"`
	NewHTMLSize int       `json:"new_html_size"`
	ChangeCount int       `json:"change_count"`
	Complexity  string    `json:"complexity"`
}

// PerformanceMetrics tracks performance of the diff operation
type PerformanceMetrics struct {
	ParseTime    time.Duration `json:"parse_time"`
	CompareTime  time.Duration `json:"compare_time"`
	ClassifyTime time.Duration `json:"classify_time"`
	TotalTime    time.Duration `json:"total_time"`
}

// HTMLDiffer is the main entry point for HTML diffing functionality
type HTMLDiffer struct {
	parser     *DOMParser
	comparator *DOMComparator
	classifier *PatternClassifier
}

// NewHTMLDiffer creates a new HTML differ with all components
func NewHTMLDiffer() *HTMLDiffer {
	return &HTMLDiffer{
		parser:     NewDOMParser(),
		comparator: NewDOMComparator(),
		classifier: NewPatternClassifier(),
	}
}

// Diff performs complete HTML diffing analysis
func (hd *HTMLDiffer) Diff(oldHTML, newHTML string) (*DiffResult, error) {
	startTime := time.Now()

	// Initialize performance tracking
	var perf PerformanceMetrics

	// Parse phase
	parseStart := time.Now()
	// Parsing is done within the Compare method, so we'll track it there
	perf.ParseTime = time.Since(parseStart)

	// Compare phase
	compareStart := time.Now()
	changes, err := hd.comparator.Compare(oldHTML, newHTML)
	if err != nil {
		return nil, fmt.Errorf("comparison failed: %w", err)
	}
	perf.CompareTime = time.Since(compareStart)

	// Classify phase
	classifyStart := time.Now()
	strategy, err := hd.classifier.ClassifyPattern(oldHTML, newHTML)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}
	perf.ClassifyTime = time.Since(classifyStart)

	perf.TotalTime = time.Since(startTime)

	// Build metadata
	metadata := DiffMetadata{
		Timestamp:   startTime,
		OldHTMLSize: len(oldHTML),
		NewHTMLSize: len(newHTML),
		ChangeCount: len(changes),
		Complexity:  hd.determineComplexity(changes, strategy),
	}

	return &DiffResult{
		Changes:     changes,
		Strategy:    strategy,
		Metadata:    metadata,
		Performance: perf,
	}, nil
}

// QuickDiff performs a fast diff analysis for performance-critical scenarios
func (hd *HTMLDiffer) QuickDiff(oldHTML, newHTML string) (*StrategyRecommendation, error) {
	// Quick classification without full change analysis
	return hd.classifier.ClassifyPattern(oldHTML, newHTML)
}

// AnalyzeChanges provides detailed analysis of specific changes
func (hd *HTMLDiffer) AnalyzeChanges(oldHTML, newHTML string) ([]DOMChange, error) {
	return hd.comparator.Compare(oldHTML, newHTML)
}

// determineComplexity analyzes the overall complexity of changes
func (hd *HTMLDiffer) determineComplexity(changes []DOMChange, strategy *StrategyRecommendation) string {
	if len(changes) == 0 {
		return "none"
	}

	if len(changes) <= 2 && strategy.Strategy <= 2 {
		return "simple"
	}

	if len(changes) <= 5 && strategy.Strategy <= 3 {
		return "moderate"
	}

	return "complex"
}

// GetStrategyName returns a human-readable name for a strategy number
func GetStrategyName(strategy int) string {
	switch strategy {
	case 1:
		return "Static/Dynamic Fragments"
	case 2:
		return "Marker Compilation"
	case 3:
		return "Granular Operations"
	case 4:
		return "Fragment Replacement"
	default:
		return "Unknown Strategy"
	}
}

// GetPatternName returns a human-readable name for a pattern type
func GetPatternName(pattern PatternType) string {
	switch pattern {
	case PatternStaticDynamic:
		return "Static/Dynamic Pattern"
	case PatternMarkerizable:
		return "Markerizable Pattern"
	case PatternGranular:
		return "Granular Operations Pattern"
	case PatternReplacement:
		return "Full Replacement Pattern"
	default:
		return "Unknown Pattern"
	}
}

// ValidateStrategy checks if a strategy recommendation is valid and actionable
func (hd *HTMLDiffer) ValidateStrategy(rec *StrategyRecommendation) error {
	if rec == nil {
		return fmt.Errorf("strategy recommendation is nil")
	}

	if rec.Strategy < 1 || rec.Strategy > 4 {
		return fmt.Errorf("invalid strategy number: %d", rec.Strategy)
	}

	return nil
}

// ShouldUseStrategy determines if a specific strategy should be used based on diff results
func (hd *HTMLDiffer) ShouldUseStrategy(result *DiffResult, targetStrategy int) bool {
	if result == nil || result.Strategy == nil {
		return false
	}

	// Deterministic: use exact strategy match
	return result.Strategy.Strategy == targetStrategy
}
