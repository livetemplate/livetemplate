package strategy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/livefir/livetemplate/internal/diff"
)

func TestStrategyAnalyzer_AnalyzeStrategy(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	tests := []struct {
		name             string
		oldHTML          string
		newHTML          string
		expectedStrategy int
		expectedPattern  diff.PatternType
		wantErr          bool
	}{
		{
			name:             "text-only change",
			oldHTML:          `<div>Hello</div>`,
			newHTML:          `<div>Hi there</div>`,
			expectedStrategy: 1,
			expectedPattern:  diff.PatternStaticDynamic,
			wantErr:          false,
		},
		{
			name:             "attribute change",
			oldHTML:          `<div class="old">Content</div>`,
			newHTML:          `<div class="new">Content</div>`,
			expectedStrategy: 2,
			expectedPattern:  diff.PatternMarkerizable,
			wantErr:          false,
		},
		{
			name:             "structural change",
			oldHTML:          `<ul><li>Item 1</li></ul>`,
			newHTML:          `<ul><li>Item 1</li><li>Item 2</li></ul>`,
			expectedStrategy: 3,
			expectedPattern:  diff.PatternGranular,
			wantErr:          false,
		},
		{
			name:             "mixed changes (structural + text)",
			oldHTML:          `<ul><li>Item 1</li></ul>`,
			newHTML:          `<ul><li>Item 1</li><li>Item 2</li></ul>`,
			expectedStrategy: 3, // This is actually pure structural change
			expectedPattern:  diff.PatternGranular,
			wantErr:          false,
		},
		{
			name:             "empty state - show content",
			oldHTML:          ``,
			newHTML:          `<span>New content</span>`,
			expectedStrategy: 0, // Will error due to empty input
			expectedPattern:  "",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.AnalyzeStrategy(tt.oldHTML, tt.newHTML)

			if (err != nil) != tt.wantErr {
				t.Errorf("AnalyzeStrategy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error
			}

			if result == nil {
				t.Fatal("AnalyzeStrategy() returned nil result")
			}

			if result.Strategy != tt.expectedStrategy {
				t.Errorf("Strategy = %d, want %d", result.Strategy, tt.expectedStrategy)
			}

			if result.Recommendation.Pattern != tt.expectedPattern {
				t.Errorf("Pattern = %s, want %s", result.Recommendation.Pattern, tt.expectedPattern)
			}

			if result.Confidence != 1.0 {
				t.Errorf("Confidence = %f, want 1.0 (deterministic)", result.Confidence)
			}

			if result.DiffResult == nil {
				t.Error("DiffResult should not be nil")
			}

			if result.AnalysisTime <= 0 {
				t.Error("AnalysisTime should be positive")
			}
		})
	}
}

func TestStrategyAnalyzer_AnalyzeWithFallback(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	tests := []struct {
		name              string
		oldHTML           string
		newHTML           string
		preferredStrategy int
		expectedStrategy  int
		expectFallback    bool
	}{
		{
			name:              "no fallback needed",
			oldHTML:           `<div>Hello</div>`,
			newHTML:           `<div>Hi</div>`,
			preferredStrategy: 1,
			expectedStrategy:  1,
			expectFallback:    false,
		},
		{
			name:              "upgrade strategy (safe)",
			oldHTML:           `<div>Hello</div>`,
			newHTML:           `<div>Hi</div>`,
			preferredStrategy: 4, // Request strategy 4 for strategy 1 pattern
			expectedStrategy:  4,
			expectFallback:    true,
		},
		{
			name:              "downgrade strategy (compatible)",
			oldHTML:           `<div class="old">Content</div>`,
			newHTML:           `<div class="new">Content</div>`,
			preferredStrategy: 1, // Request strategy 1 for strategy 2 pattern (incompatible)
			expectedStrategy:  2, // Should keep original strategy for safety
			expectFallback:    false,
		},
		{
			name:              "complex to replacement (safe)",
			oldHTML:           `<div><p>Old</p></div>`,
			newHTML:           `<article><h1>New</h1></article>`,
			preferredStrategy: 4,
			expectedStrategy:  4,
			expectFallback:    true, // Will use fallback to upgrade from strategy 3 to 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.AnalyzeWithFallback(tt.oldHTML, tt.newHTML, tt.preferredStrategy)
			if err != nil {
				t.Fatalf("AnalyzeWithFallback() error = %v", err)
			}

			if result.Strategy != tt.expectedStrategy {
				t.Errorf("Strategy = %d, want %d", result.Strategy, tt.expectedStrategy)
			}

			if result.UsesFallback != tt.expectFallback {
				t.Errorf("UsesFallback = %v, want %v", result.UsesFallback, tt.expectFallback)
			}

			if tt.expectFallback && result.FallbackReason == "" {
				t.Error("FallbackReason should not be empty when using fallback")
			}
		})
	}
}

func TestStrategyAnalyzer_QuickAnalyze(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	tests := []struct {
		name             string
		oldHTML          string
		newHTML          string
		expectedStrategy int
		wantErr          bool
	}{
		{
			name:             "quick text analysis",
			oldHTML:          `<span>Old</span>`,
			newHTML:          `<span>New</span>`,
			expectedStrategy: 1,
			wantErr:          false,
		},
		{
			name:             "quick attribute analysis",
			oldHTML:          `<div class="a">Text</div>`,
			newHTML:          `<div class="b">Text</div>`,
			expectedStrategy: 2,
			wantErr:          false,
		},
		{
			name:    "quick analysis with empty input",
			oldHTML: "",
			newHTML: `<div>Content</div>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := analyzer.QuickAnalyze(tt.oldHTML, tt.newHTML)

			if (err != nil) != tt.wantErr {
				t.Errorf("QuickAnalyze() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Expected error
			}

			if strategy != tt.expectedStrategy {
				t.Errorf("Strategy = %d, want %d", strategy, tt.expectedStrategy)
			}
		})
	}
}

func TestStrategyAnalyzer_Caching(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	oldHTML := `<div>Hello</div>`
	newHTML := `<div>Hi there</div>`

	// First analysis
	result1, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("First analysis failed: %v", err)
	}

	if result1.CacheHit {
		t.Error("First analysis should not be a cache hit")
	}

	// Second analysis - should hit cache
	result2, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("Second analysis failed: %v", err)
	}

	if !result2.CacheHit {
		t.Error("Second analysis should be a cache hit")
	}

	// Results should be identical except for cache hit flag
	if result1.Strategy != result2.Strategy {
		t.Errorf("Cached strategy differs: %d vs %d", result1.Strategy, result2.Strategy)
	}

	// Test cache stats
	stats := analyzer.GetCacheStats()
	if stats["cache_size"].(int) == 0 {
		t.Error("Cache should contain entries")
	}

	if !stats["cache_enabled"].(bool) {
		t.Error("Cache should be enabled")
	}
}

func TestStrategyAnalyzer_CacheDisabled(t *testing.T) {
	analyzer := NewStrategyAnalyzer()
	analyzer.SetCacheEnabled(false)

	oldHTML := `<div>Hello</div>`
	newHTML := `<div>Hi there</div>`

	// First analysis
	result1, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("First analysis failed: %v", err)
	}

	// Second analysis - should not hit cache
	result2, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		t.Fatalf("Second analysis failed: %v", err)
	}

	if result1.CacheHit || result2.CacheHit {
		t.Error("With cache disabled, no results should be cache hits")
	}

	stats := analyzer.GetCacheStats()
	if stats["cache_enabled"].(bool) {
		t.Error("Cache should be disabled")
	}
}

func TestStrategyAnalyzer_Metrics(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	// Perform various analyses
	testCases := []struct {
		oldHTML, newHTML string
		expectedStrategy int
	}{
		{`<div>Old</div>`, `<div>New</div>`, 1},                       // Strategy 1
		{`<div class="a">Text</div>`, `<div class="b">Text</div>`, 2}, // Strategy 2
		{`<ul><li>A</li></ul>`, `<ul><li>A</li><li>B</li></ul>`, 3},   // Strategy 3
	}

	for _, tc := range testCases {
		_, err := analyzer.AnalyzeStrategy(tc.oldHTML, tc.newHTML)
		if err != nil {
			t.Fatalf("Analysis failed: %v", err)
		}
	}

	// Check metrics
	metrics := analyzer.GetMetrics()

	if metrics.TotalAnalyses != int64(len(testCases)) {
		t.Errorf("TotalAnalyses = %d, want %d", metrics.TotalAnalyses, len(testCases))
	}

	// Check strategy usage
	for _, tc := range testCases {
		count, exists := metrics.StrategyUsage[tc.expectedStrategy]
		if !exists || count == 0 {
			t.Errorf("Strategy %d should have been used", tc.expectedStrategy)
		}
	}

	// Check pattern distribution
	if len(metrics.PatternDistribution) == 0 {
		t.Error("PatternDistribution should not be empty")
	}

	// Check rule correctness tracking
	if len(metrics.RuleCorrectness) == 0 {
		t.Error("RuleCorrectness should not be empty")
	}

	// Test metrics reset
	analyzer.ResetMetrics()
	resetMetrics := analyzer.GetMetrics()
	if resetMetrics.TotalAnalyses != 0 {
		t.Error("Metrics should be reset")
	}
}

func TestStrategyAnalyzer_FallbackLogic(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	tests := []struct {
		name             string
		pattern          diff.PatternType
		originalStrategy int
		targetStrategy   int
		expectDowngrade  bool
	}{
		{
			name:             "static-dynamic to static-dynamic",
			pattern:          diff.PatternStaticDynamic,
			originalStrategy: 2,
			targetStrategy:   1,
			expectDowngrade:  true, // Safe downgrade
		},
		{
			name:             "markerizable to static-dynamic",
			pattern:          diff.PatternMarkerizable,
			originalStrategy: 2,
			targetStrategy:   1,
			expectDowngrade:  false, // Unsafe downgrade
		},
		{
			name:             "granular to marker",
			pattern:          diff.PatternGranular,
			originalStrategy: 3,
			targetStrategy:   2,
			expectDowngrade:  false, // Unsafe downgrade
		},
		{
			name:             "replacement allows any target",
			pattern:          diff.PatternReplacement,
			originalStrategy: 4,
			targetStrategy:   1,
			expectDowngrade:  false, // Strategy 4 shouldn't downgrade
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock result
			originalResult := &AnalysisResult{
				Strategy: tt.originalStrategy,
				Recommendation: &diff.StrategyRecommendation{
					Strategy: tt.originalStrategy,
					Pattern:  tt.pattern,
					Reason:   "test",
				},
				Confidence: 1.0,
			}

			canDowngrade := analyzer.canDowngradeStrategy(originalResult, tt.targetStrategy)

			if canDowngrade != tt.expectDowngrade {
				t.Errorf("canDowngradeStrategy() = %v, want %v", canDowngrade, tt.expectDowngrade)
			}
		})
	}
}

func TestStrategyAnalyzer_CacheManagement(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	// Set small cache size for testing
	analyzer.maxCacheSize = 2
	analyzer.SetCacheTTL(100 * time.Millisecond)

	// Fill cache beyond capacity
	testInputs := []struct{ old, new string }{
		{"<div>1</div>", "<div>1a</div>"},
		{"<div>2</div>", "<div>2a</div>"},
		{"<div>3</div>", "<div>3a</div>"}, // This should trigger cache cleanup
	}

	for _, input := range testInputs {
		_, err := analyzer.AnalyzeStrategy(input.old, input.new)
		if err != nil {
			t.Fatalf("Analysis failed: %v", err)
		}
	}

	stats := analyzer.GetCacheStats()
	cacheSize := stats["cache_size"].(int)
	if cacheSize > analyzer.maxCacheSize {
		t.Errorf("Cache size %d exceeds max size %d", cacheSize, analyzer.maxCacheSize)
	}

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond) // Wait for TTL to expire

	// Access an expired entry (should not be found)
	result, err := analyzer.AnalyzeStrategy(testInputs[0].old, testInputs[0].new)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if result.CacheHit {
		t.Error("Should not get cache hit for expired entry")
	}

	// Test cache clearing
	analyzer.ClearCache()
	statsAfterClear := analyzer.GetCacheStats()
	if statsAfterClear["cache_size"].(int) != 0 {
		t.Error("Cache should be empty after clearing")
	}
}

func TestStrategyAnalyzer_DeterministicBehavior(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	// Test cases that should always produce the same result
	testCases := []struct {
		name    string
		oldHTML string
		newHTML string
	}{
		{
			name:    "text change",
			oldHTML: `<p>Original text</p>`,
			newHTML: `<p>Modified text</p>`,
		},
		{
			name:    "attribute change",
			oldHTML: `<div class="red">Content</div>`,
			newHTML: `<div class="blue">Content</div>`,
		},
		{
			name:    "structural change",
			oldHTML: `<ul><li>Item</li></ul>`,
			newHTML: `<ul><li>Item</li><li>New item</li></ul>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the same analysis multiple times
			var results []*AnalysisResult

			for i := 0; i < 5; i++ {
				result, err := analyzer.AnalyzeStrategy(tc.oldHTML, tc.newHTML)
				if err != nil {
					t.Fatalf("Analysis %d failed: %v", i, err)
				}
				results = append(results, result)
			}

			// All results should be identical (except cache hit status)
			firstResult := results[0]
			for i, result := range results[1:] {
				if result.Strategy != firstResult.Strategy {
					t.Errorf("Result %d strategy %d != first result strategy %d", i+1, result.Strategy, firstResult.Strategy)
				}
				if result.Recommendation.Pattern != firstResult.Recommendation.Pattern {
					t.Errorf("Result %d pattern %s != first result pattern %s", i+1, result.Recommendation.Pattern, firstResult.Recommendation.Pattern)
				}
				if result.Confidence != firstResult.Confidence {
					t.Errorf("Result %d confidence %f != first result confidence %f", i+1, result.Confidence, firstResult.Confidence)
				}
			}
		})
	}
}

func TestStrategyAnalyzer_RuleCorrectness(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	// Test expected pattern -> strategy mappings
	expectedMappings := []struct {
		description      string
		oldHTML, newHTML string
		expectedStrategy int
		expectedPattern  diff.PatternType
	}{
		{
			description:      "Text-only changes should map to Strategy 1",
			oldHTML:          `<span>Before</span>`,
			newHTML:          `<span>After</span>`,
			expectedStrategy: 1,
			expectedPattern:  diff.PatternStaticDynamic,
		},
		{
			description:      "Attribute changes should map to Strategy 2",
			oldHTML:          `<div class="old">Text</div>`,
			newHTML:          `<div class="new">Text</div>`,
			expectedStrategy: 2,
			expectedPattern:  diff.PatternMarkerizable,
		},
		{
			description:      "Pure structural changes should map to Strategy 3",
			oldHTML:          `<ul><li>A</li></ul>`,
			newHTML:          `<ul><li>A</li><li>B</li></ul>`,
			expectedStrategy: 3,
			expectedPattern:  diff.PatternGranular,
		},
	}

	for _, mapping := range expectedMappings {
		t.Run(mapping.description, func(t *testing.T) {
			result, err := analyzer.AnalyzeStrategy(mapping.oldHTML, mapping.newHTML)
			if err != nil {
				t.Fatalf("Analysis failed: %v", err)
			}

			if result.Strategy != mapping.expectedStrategy {
				t.Errorf("Strategy = %d, want %d", result.Strategy, mapping.expectedStrategy)
			}

			if result.Recommendation.Pattern != mapping.expectedPattern {
				t.Errorf("Pattern = %s, want %s", result.Recommendation.Pattern, mapping.expectedPattern)
			}

			// Verify the mapping is tracked in metrics
			metrics := analyzer.GetMetrics()
			ruleKey := fmt.Sprintf("%s->strategy%d", mapping.expectedPattern, mapping.expectedStrategy)
			if count, exists := metrics.RuleCorrectness[ruleKey]; !exists || count == 0 {
				t.Errorf("Rule correctness not tracked for %s", ruleKey)
			}
		})
	}

	// Check overall rule correctness
	metrics := analyzer.GetMetrics()
	correctMappings := metrics.RuleCorrectness["correct_mapping"]
	totalMappings := correctMappings + metrics.RuleCorrectness["unexpected_mapping"]

	if totalMappings > 0 {
		correctnessRate := float64(correctMappings) / float64(totalMappings)
		if correctnessRate < 1.0 {
			t.Errorf("Rule correctness rate = %f, want 1.0 (100%% correct)", correctnessRate)
		}
	}
}

// Benchmark strategy analysis performance
func BenchmarkStrategyAnalyzer_AnalyzeStrategy(b *testing.B) {
	analyzer := NewStrategyAnalyzer()
	oldHTML := `<div class="container"><h1>Title</h1><p>Some content here</p></div>`
	newHTML := `<div class="container"><h1>Updated Title</h1><p>Some content here</p></div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStrategyAnalyzer_QuickAnalyze(b *testing.B) {
	analyzer := NewStrategyAnalyzer()
	oldHTML := `<div class="container"><p>Content</p></div>`
	newHTML := `<div class="updated"><p>Content</p></div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.QuickAnalyze(oldHTML, newHTML)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStrategyAnalyzer_WithCache(b *testing.B) {
	analyzer := NewStrategyAnalyzer()
	oldHTML := `<span>Original</span>`
	newHTML := `<span>Modified</span>`

	// Warm up cache
	_, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestStrategyAnalyzer_EdgeCases(t *testing.T) {
	analyzer := NewStrategyAnalyzer()

	t.Run("identical HTML", func(t *testing.T) {
		html := `<div>Same content</div>`
		result, err := analyzer.AnalyzeStrategy(html, html)
		if err != nil {
			t.Fatalf("Analysis failed: %v", err)
		}

		// Should default to Strategy 1 for no changes
		if result.Strategy != 1 {
			t.Errorf("Strategy = %d, want 1 for identical HTML", result.Strategy)
		}
	})

	t.Run("very large HTML", func(t *testing.T) {
		// Create large HTML content
		var oldBuilder, newBuilder strings.Builder
		oldBuilder.WriteString("<div>")
		newBuilder.WriteString("<div>")

		for i := 0; i < 1000; i++ {
			oldBuilder.WriteString(fmt.Sprintf("<p>Paragraph %d content</p>", i))
			newBuilder.WriteString(fmt.Sprintf("<p>Paragraph %d updated</p>", i))
		}

		oldBuilder.WriteString("</div>")
		newBuilder.WriteString("</div>")

		start := time.Now()
		result, err := analyzer.AnalyzeStrategy(oldBuilder.String(), newBuilder.String())
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Analysis failed: %v", err)
		}

		if result == nil {
			t.Fatal("Result should not be nil")
		}

		// Analysis should complete in reasonable time
		if duration > 5*time.Second {
			t.Errorf("Analysis took too long: %v", duration)
		}

		t.Logf("Large HTML analysis took: %v", duration)
	})

	t.Run("malformed HTML", func(t *testing.T) {
		oldHTML := `<div><p>Unclosed paragraph`
		newHTML := `<div><p>Fixed paragraph</p></div>`

		// Should handle malformed HTML gracefully
		result, err := analyzer.AnalyzeStrategy(oldHTML, newHTML)
		if err != nil {
			t.Fatalf("Analysis should handle malformed HTML: %v", err)
		}

		if result == nil {
			t.Fatal("Result should not be nil for malformed HTML")
		}
	})
}
