package strategy

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/diff"
)

// Fragment represents a generated update fragment with strategy-specific data
type Fragment struct {
	ID       string            `json:"id"`
	Strategy string            `json:"strategy"` // "static_dynamic", "markers", "granular", "replacement"
	Action   string            `json:"action"`   // Strategy-specific action
	Data     interface{}       `json:"data"`     // Strategy-specific payload
	Metadata *FragmentMetadata `json:"metadata,omitempty"`
}

// FragmentMetadata contains performance and optimization information
type FragmentMetadata struct {
	GenerationTime   time.Duration    `json:"generation_time"`
	OriginalSize     int              `json:"original_size"`
	CompressedSize   int              `json:"compressed_size"`
	CompressionRatio float64          `json:"compression_ratio"`
	Strategy         int              `json:"strategy_number"`
	PatternType      diff.PatternType `json:"pattern_type"`
	Confidence       float64          `json:"confidence"`
	FallbackUsed     bool             `json:"fallback_used"`
}

// UpdateGeneratorMetrics tracks performance of the update generation pipeline
type UpdateGeneratorMetrics struct {
	TotalGenerations      int64            `json:"total_generations"`
	SuccessfulGenerations int64            `json:"successful_generations"`
	FailedGenerations     int64            `json:"failed_generations"`
	StrategyUsage         map[string]int64 `json:"strategy_usage"`
	AverageGenerationTime time.Duration    `json:"average_generation_time"`
	TotalBandwidthSaved   int64            `json:"total_bandwidth_saved"`
	FallbackRate          float64          `json:"fallback_rate"`
	ErrorRate             float64          `json:"error_rate"`
	LastReset             time.Time        `json:"last_reset"`
}

// UpdateGenerator orchestrates the complete update generation pipeline
type UpdateGenerator struct {
	// Core components
	strategyAnalyzer *StrategyAnalyzer
	staticDynamicGen *StaticDynamicGenerator
	markerCompiler   *MarkerCompiler
	granularOperator *GranularOperator
	fragmentReplacer *FragmentReplacer

	// Metrics and performance tracking
	metrics      *UpdateGeneratorMetrics
	metricsMutex sync.RWMutex

	// Configuration
	enableMetrics     bool
	enableFallback    bool
	maxGenerationTime time.Duration
}

// NewUpdateGenerator creates a new update generator pipeline
func NewUpdateGenerator() *UpdateGenerator {
	return &UpdateGenerator{
		strategyAnalyzer: NewStrategyAnalyzer(),
		staticDynamicGen: NewStaticDynamicGenerator(),
		markerCompiler:   NewMarkerCompiler(),
		granularOperator: NewGranularOperator(),
		fragmentReplacer: NewFragmentReplacer(),

		metrics: &UpdateGeneratorMetrics{
			StrategyUsage: make(map[string]int64),
			LastReset:     time.Now(),
		},

		enableMetrics:     true,
		enableFallback:    true,
		maxGenerationTime: 5 * time.Second,
	}
}

// GenerateUpdate orchestrates the complete update generation workflow
func (ug *UpdateGenerator) GenerateUpdate(tmpl *template.Template, oldData, newData interface{}) ([]*Fragment, error) {
	startTime := time.Now()

	// Render templates with old and new data for HTML diffing
	oldHTML, newHTML, err := ug.renderTemplates(tmpl, oldData, newData)
	if err != nil {
		ug.updateErrorMetrics()
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	// Perform HTML diff analysis and strategy selection
	analysisResult, err := ug.strategyAnalyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		ug.updateErrorMetrics()
		return nil, fmt.Errorf("strategy analysis failed: %w", err)
	}

	// Generate fragments using selected strategy
	fragments, err := ug.generateFragments(oldHTML, newHTML, analysisResult, tmpl, oldData, newData)
	if err != nil {
		// Try fallback if enabled
		if ug.enableFallback {
			fallbackFragments, fallbackErr := ug.generateFallbackFragments(oldHTML, newHTML, tmpl, oldData, newData)
			if fallbackErr == nil {
				ug.updateSuccessMetrics(startTime, fallbackFragments, true)
				return fallbackFragments, nil
			}
		}

		ug.updateErrorMetrics()
		return nil, fmt.Errorf("fragment generation failed: %w", err)
	}

	// Update metrics and return results
	ug.updateSuccessMetrics(startTime, fragments, false)
	return fragments, nil
}

// renderTemplates renders the template with both old and new data for comparison
func (ug *UpdateGenerator) renderTemplates(tmpl *template.Template, oldData, newData interface{}) (string, string, error) {
	var oldHTML, newHTML string
	var err error

	// Render with old data
	if oldData != nil {
		oldHTML, err = ug.executeTemplate(tmpl, oldData)
		if err != nil {
			return "", "", fmt.Errorf("failed to render template with old data: %w", err)
		}
	}

	// Render with new data
	newHTML, err = ug.executeTemplate(tmpl, newData)
	if err != nil {
		return "", "", fmt.Errorf("failed to render template with new data: %w", err)
	}

	return oldHTML, newHTML, nil
}

// executeTemplate safely executes a template with data
func (ug *UpdateGenerator) executeTemplate(tmpl *template.Template, data interface{}) (string, error) {
	var buf strings.Builder
	err := tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generateFragments delegates to the appropriate strategy-specific generator
func (ug *UpdateGenerator) generateFragments(oldHTML, newHTML string, analysis *AnalysisResult, tmpl *template.Template, oldData, newData interface{}) ([]*Fragment, error) {
	switch analysis.Strategy {
	case 1: // Static/Dynamic
		return ug.generateStaticDynamicFragments(oldHTML, newHTML, analysis)
	case 2: // Markers
		return ug.generateMarkerFragments(oldHTML, newHTML, analysis)
	case 3: // Granular
		return ug.generateGranularFragments(oldHTML, newHTML, analysis)
	case 4: // Replacement
		return ug.generateReplacementFragments(oldHTML, newHTML, analysis)
	default:
		return nil, fmt.Errorf("unsupported strategy: %d", analysis.Strategy)
	}
}

// generateStaticDynamicFragments generates Strategy 1 fragments
func (ug *UpdateGenerator) generateStaticDynamicFragments(oldHTML, newHTML string, analysis *AnalysisResult) ([]*Fragment, error) {
	startTime := time.Now()

	var result *StaticDynamicData
	var err error
	var action string

	// Check if this is a conditional pattern (enhanced Strategy 1)
	if analysis.Recommendation.ConditionalPattern != nil {
		result, err = ug.staticDynamicGen.GenerateConditional(analysis.Recommendation.ConditionalPattern, ug.generateFragmentID("conditional_static", oldHTML, newHTML))
		action = "update_conditional"
	} else {
		result, err = ug.staticDynamicGen.Generate(oldHTML, newHTML, ug.generateFragmentID("static_dynamic", oldHTML, newHTML))
		action = "update_values"
	}

	if err != nil {
		return nil, fmt.Errorf("static/dynamic generation failed: %w", err)
	}

	fragmentID := ug.generateFragmentID("static_dynamic", oldHTML, newHTML)
	if analysis.Recommendation.ConditionalPattern != nil {
		fragmentID = ug.generateFragmentID("conditional_static", oldHTML, newHTML)
	}

	fragment := &Fragment{
		ID:       fragmentID,
		Strategy: "static_dynamic",
		Action:   action,
		Data:     result,
		Metadata: &FragmentMetadata{
			GenerationTime: time.Since(startTime),
			OriginalSize:   len(newHTML),
			CompressedSize: ug.calculateStaticDynamicSize(result),
			Strategy:       1,
			PatternType:    analysis.Recommendation.Pattern,
			Confidence:     analysis.Confidence,
			FallbackUsed:   analysis.UsesFallback,
		},
	}

	// Calculate compression ratio
	if fragment.Metadata.OriginalSize > 0 {
		fragment.Metadata.CompressionRatio = float64(fragment.Metadata.CompressedSize) / float64(fragment.Metadata.OriginalSize)
	}

	return []*Fragment{fragment}, nil
}

// generateMarkerFragments generates Strategy 2 fragments
func (ug *UpdateGenerator) generateMarkerFragments(oldHTML, newHTML string, analysis *AnalysisResult) ([]*Fragment, error) {
	startTime := time.Now()

	result, err := ug.markerCompiler.Compile(oldHTML, newHTML, ug.generateFragmentID("markers", oldHTML, newHTML))
	if err != nil {
		return nil, fmt.Errorf("marker compilation failed: %w", err)
	}

	fragment := &Fragment{
		ID:       ug.generateFragmentID("markers", oldHTML, newHTML),
		Strategy: "markers",
		Action:   "apply_patches",
		Data:     result,
		Metadata: &FragmentMetadata{
			GenerationTime: time.Since(startTime),
			OriginalSize:   len(newHTML),
			CompressedSize: ug.calculateMarkerSize(result),
			Strategy:       2,
			PatternType:    analysis.Recommendation.Pattern,
			Confidence:     analysis.Confidence,
			FallbackUsed:   analysis.UsesFallback,
		},
	}

	// Calculate compression ratio
	if fragment.Metadata.OriginalSize > 0 {
		fragment.Metadata.CompressionRatio = float64(fragment.Metadata.CompressedSize) / float64(fragment.Metadata.OriginalSize)
	}

	return []*Fragment{fragment}, nil
}

// generateGranularFragments generates Strategy 3 fragments
func (ug *UpdateGenerator) generateGranularFragments(oldHTML, newHTML string, analysis *AnalysisResult) ([]*Fragment, error) {
	startTime := time.Now()

	result, err := ug.granularOperator.Compile(oldHTML, newHTML, ug.generateFragmentID("granular", oldHTML, newHTML))
	if err != nil {
		return nil, fmt.Errorf("granular operation failed: %w", err)
	}

	fragment := &Fragment{
		ID:       ug.generateFragmentID("granular", oldHTML, newHTML),
		Strategy: "granular",
		Action:   "apply_operations",
		Data:     result,
		Metadata: &FragmentMetadata{
			GenerationTime: time.Since(startTime),
			OriginalSize:   len(newHTML),
			CompressedSize: ug.calculateGranularSize(result),
			Strategy:       3,
			PatternType:    analysis.Recommendation.Pattern,
			Confidence:     analysis.Confidence,
			FallbackUsed:   analysis.UsesFallback,
		},
	}

	// Calculate compression ratio
	if fragment.Metadata.OriginalSize > 0 {
		fragment.Metadata.CompressionRatio = float64(fragment.Metadata.CompressedSize) / float64(fragment.Metadata.OriginalSize)
	}

	return []*Fragment{fragment}, nil
}

// generateReplacementFragments generates Strategy 4 fragments
func (ug *UpdateGenerator) generateReplacementFragments(oldHTML, newHTML string, analysis *AnalysisResult) ([]*Fragment, error) {
	startTime := time.Now()

	result, err := ug.fragmentReplacer.Compile(oldHTML, newHTML, ug.generateFragmentID("replacement", oldHTML, newHTML))
	if err != nil {
		return nil, fmt.Errorf("fragment replacement failed: %w", err)
	}

	fragment := &Fragment{
		ID:       ug.generateFragmentID("replacement", oldHTML, newHTML),
		Strategy: "replacement",
		Action:   "replace_content",
		Data:     result,
		Metadata: &FragmentMetadata{
			GenerationTime: time.Since(startTime),
			OriginalSize:   len(newHTML),
			CompressedSize: ug.calculateReplacementSize(result),
			Strategy:       4,
			PatternType:    analysis.Recommendation.Pattern,
			Confidence:     analysis.Confidence,
			FallbackUsed:   analysis.UsesFallback,
		},
	}

	// Calculate compression ratio
	if fragment.Metadata.OriginalSize > 0 {
		fragment.Metadata.CompressionRatio = float64(fragment.Metadata.CompressedSize) / float64(fragment.Metadata.OriginalSize)
	}

	return []*Fragment{fragment}, nil
}

// generateFallbackFragments generates fallback fragments when primary strategy fails
func (ug *UpdateGenerator) generateFallbackFragments(oldHTML, newHTML string, tmpl *template.Template, oldData, newData interface{}) ([]*Fragment, error) {
	// Always fall back to Strategy 4 (replacement) as it handles any change
	fallbackAnalysis := &AnalysisResult{
		Strategy: 4,
		Recommendation: &diff.StrategyRecommendation{
			Strategy: 4,
			Pattern:  diff.PatternReplacement,
			Reason:   "Fallback to replacement strategy",
		},
		Confidence:   1.0,
		UsesFallback: true,
	}

	return ug.generateReplacementFragments(oldHTML, newHTML, fallbackAnalysis)
}

// generateFragmentID creates a deterministic ID for fragments
func (ug *UpdateGenerator) generateFragmentID(strategy, oldHTML, newHTML string) string {
	combined := strategy + "|" + oldHTML + "|" + newHTML
	hash := md5.Sum([]byte(combined))
	return fmt.Sprintf("frag_%s_%x", strategy, hash[:8])
}

// Size calculation methods for different strategies
func (ug *UpdateGenerator) calculateStaticDynamicSize(data interface{}) int {
	if sdData, ok := data.(*StaticDynamicData); ok {
		// For static/dynamic, the compressed size includes:
		// 1. Dynamic values (traditional Strategy 1)
		// 2. Conditional metadata (enhanced Strategy 1)
		size := 0

		// Add dynamic values
		for _, dynamic := range sdData.Dynamics {
			size += len(dynamic)
		}

		// Add conditional values (much smaller than full HTML)
		for _, conditional := range sdData.Conditionals {
			size += len(conditional.TruthyValue)
			size += len(conditional.FalsyValue)
			// Add small overhead for conditional metadata
			size += 20 // rough estimate for condition type, position, etc.
		}

		return size
	}
	return 0
}

func (ug *UpdateGenerator) calculateMarkerSize(data interface{}) int {
	if mcData, ok := data.(*MarkerPatchData); ok {
		size := 0
		for _, value := range mcData.ValueUpdates {
			size += len(value)
		}
		return size
	}
	return 0
}

func (ug *UpdateGenerator) calculateGranularSize(data interface{}) int {
	if goData, ok := data.(*GranularOpData); ok {
		size := 0
		for _, op := range goData.Operations {
			size += len(op.Content)
		}
		return size
	}
	return 0
}

func (ug *UpdateGenerator) calculateReplacementSize(data interface{}) int {
	if frData, ok := data.(*ReplacementData); ok {
		return len(frData.Content)
	}
	return 0
}

// Metrics update methods
func (ug *UpdateGenerator) updateSuccessMetrics(startTime time.Time, fragments []*Fragment, usedFallback bool) {
	if !ug.enableMetrics {
		return
	}

	ug.metricsMutex.Lock()
	defer ug.metricsMutex.Unlock()

	ug.metrics.TotalGenerations++
	ug.metrics.SuccessfulGenerations++

	generationTime := time.Since(startTime)
	ug.updateAverageTime(generationTime)

	// Update strategy usage and bandwidth savings
	for _, fragment := range fragments {
		ug.metrics.StrategyUsage[fragment.Strategy]++

		if fragment.Metadata != nil {
			saved := fragment.Metadata.OriginalSize - fragment.Metadata.CompressedSize
			if saved > 0 {
				ug.metrics.TotalBandwidthSaved += int64(saved)
			}
		}
	}

	// Update fallback rate
	if ug.metrics.TotalGenerations > 0 {
		fallbackCount := int64(0)
		if usedFallback {
			fallbackCount = 1
		}
		ug.metrics.FallbackRate = float64(fallbackCount) / float64(ug.metrics.TotalGenerations)
	}

	// Update error rate
	if ug.metrics.TotalGenerations > 0 {
		ug.metrics.ErrorRate = float64(ug.metrics.FailedGenerations) / float64(ug.metrics.TotalGenerations)
	}
}

func (ug *UpdateGenerator) updateErrorMetrics() {
	if !ug.enableMetrics {
		return
	}

	ug.metricsMutex.Lock()
	defer ug.metricsMutex.Unlock()

	ug.metrics.TotalGenerations++
	ug.metrics.FailedGenerations++

	// Update error rate
	if ug.metrics.TotalGenerations > 0 {
		ug.metrics.ErrorRate = float64(ug.metrics.FailedGenerations) / float64(ug.metrics.TotalGenerations)
	}
}

func (ug *UpdateGenerator) updateAverageTime(newTime time.Duration) {
	if ug.metrics.SuccessfulGenerations <= 1 {
		ug.metrics.AverageGenerationTime = newTime
	} else {
		// Weighted average
		totalTime := time.Duration(ug.metrics.SuccessfulGenerations-1)*ug.metrics.AverageGenerationTime + newTime
		ug.metrics.AverageGenerationTime = totalTime / time.Duration(ug.metrics.SuccessfulGenerations)
	}
}

// GetMetrics returns current pipeline metrics
func (ug *UpdateGenerator) GetMetrics() *UpdateGeneratorMetrics {
	ug.metricsMutex.RLock()
	defer ug.metricsMutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	metrics := &UpdateGeneratorMetrics{
		TotalGenerations:      ug.metrics.TotalGenerations,
		SuccessfulGenerations: ug.metrics.SuccessfulGenerations,
		FailedGenerations:     ug.metrics.FailedGenerations,
		StrategyUsage:         make(map[string]int64),
		AverageGenerationTime: ug.metrics.AverageGenerationTime,
		TotalBandwidthSaved:   ug.metrics.TotalBandwidthSaved,
		FallbackRate:          ug.metrics.FallbackRate,
		ErrorRate:             ug.metrics.ErrorRate,
		LastReset:             ug.metrics.LastReset,
	}

	for k, v := range ug.metrics.StrategyUsage {
		metrics.StrategyUsage[k] = v
	}

	return metrics
}

// ResetMetrics resets all pipeline metrics
func (ug *UpdateGenerator) ResetMetrics() {
	ug.metricsMutex.Lock()
	defer ug.metricsMutex.Unlock()

	ug.metrics = &UpdateGeneratorMetrics{
		StrategyUsage: make(map[string]int64),
		LastReset:     time.Now(),
	}
}

// SetFallbackEnabled enables or disables fallback strategies
func (ug *UpdateGenerator) SetFallbackEnabled(enabled bool) {
	ug.enableFallback = enabled
}

// SetMetricsEnabled enables or disables metrics collection
func (ug *UpdateGenerator) SetMetricsEnabled(enabled bool) {
	ug.enableMetrics = enabled
}

// SetMaxGenerationTime sets the maximum time allowed for update generation
func (ug *UpdateGenerator) SetMaxGenerationTime(duration time.Duration) {
	ug.maxGenerationTime = duration
}
