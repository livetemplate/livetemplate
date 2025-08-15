package strategy

import (
	"crypto/md5"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/diff"
)

// AnalysisResult represents the complete analysis result with strategy recommendation
type AnalysisResult struct {
	Strategy       int                          `json:"strategy"`
	Recommendation *diff.StrategyRecommendation `json:"recommendation"`
	DiffResult     *diff.DiffResult             `json:"diff_result"`
	Confidence     float64                      `json:"confidence"` // Always 1.0 for deterministic rules
	FallbackReason string                       `json:"fallback_reason,omitempty"`
	CacheHit       bool                         `json:"cache_hit"`
	AnalysisTime   time.Duration                `json:"analysis_time"`
	UsesFallback   bool                         `json:"uses_fallback"`
}

// StrategyMetrics tracks the effectiveness of deterministic rules
type StrategyMetrics struct {
	TotalAnalyses       int64                      `json:"total_analyses"`
	StrategyUsage       map[int]int64              `json:"strategy_usage"`
	PatternDistribution map[diff.PatternType]int64 `json:"pattern_distribution"`
	FallbackCount       int64                      `json:"fallback_count"`
	CacheHitRate        float64                    `json:"cache_hit_rate"`
	AverageAnalysisTime time.Duration              `json:"average_analysis_time"`
	RuleCorrectness     map[string]int64           `json:"rule_correctness"`
	LastReset           time.Time                  `json:"last_reset"`
}

// CacheEntry represents a cached analysis result
type CacheEntry struct {
	Result    *AnalysisResult
	Timestamp time.Time
	Hits      int64
}

// StrategyAnalyzer provides HTML diff-based deterministic strategy analysis
type StrategyAnalyzer struct {
	differ       *diff.HTMLDiffer
	cache        map[string]*CacheEntry
	cacheMutex   sync.RWMutex
	metrics      *StrategyMetrics
	metricsMutex sync.RWMutex

	// Configuration
	cacheEnabled bool
	cacheTTL     time.Duration
	maxCacheSize int
}

// NewStrategyAnalyzer creates a new strategy analyzer with HTML diff integration
func NewStrategyAnalyzer() *StrategyAnalyzer {
	return &StrategyAnalyzer{
		differ: diff.NewHTMLDiffer(),
		cache:  make(map[string]*CacheEntry),
		metrics: &StrategyMetrics{
			StrategyUsage:       make(map[int]int64),
			PatternDistribution: make(map[diff.PatternType]int64),
			RuleCorrectness:     make(map[string]int64),
			LastReset:           time.Now(),
		},
		cacheEnabled: true,
		cacheTTL:     5 * time.Minute, // Cache analysis results for 5 minutes
		maxCacheSize: 1000,            // Maximum cache entries
	}
}

// AnalyzeStrategy performs deterministic strategy analysis based on HTML diffing
func (sa *StrategyAnalyzer) AnalyzeStrategy(oldHTML, newHTML string) (*AnalysisResult, error) {
	startTime := time.Now()

	// Generate cache key
	cacheKey := sa.generateCacheKey(oldHTML, newHTML)

	// Check cache first
	if sa.cacheEnabled {
		if cached := sa.getCachedResult(cacheKey); cached != nil {
			sa.updateMetrics(cached, true)
			return cached, nil
		}
	}

	// Perform HTML diffing analysis
	diffResult, err := sa.differ.Diff(oldHTML, newHTML)
	if err != nil {
		return nil, fmt.Errorf("HTML diffing failed: %w", err)
	}

	// Validate the strategy recommendation
	if err := sa.differ.ValidateStrategy(diffResult.Strategy); err != nil {
		return nil, fmt.Errorf("invalid strategy recommendation: %w", err)
	}

	// Create analysis result
	result := &AnalysisResult{
		Strategy:       diffResult.Strategy.Strategy,
		Recommendation: diffResult.Strategy,
		DiffResult:     diffResult,
		Confidence:     1.0, // Deterministic rules always have 100% confidence
		CacheHit:       false,
		AnalysisTime:   time.Since(startTime),
		UsesFallback:   false,
	}

	// Track rule correctness
	sa.trackRuleCorrectness(diffResult)

	// Cache the result
	if sa.cacheEnabled {
		sa.cacheResult(cacheKey, result)
	}

	// Update metrics
	sa.updateMetrics(result, false)

	return result, nil
}

// AnalyzeWithFallback performs strategy analysis with automatic fallback logic
func (sa *StrategyAnalyzer) AnalyzeWithFallback(oldHTML, newHTML string, preferredStrategy int) (*AnalysisResult, error) {
	// Get the recommended strategy
	result, err := sa.AnalyzeStrategy(oldHTML, newHTML)
	if err != nil {
		return nil, err
	}

	// Check if the recommended strategy matches preference
	if preferredStrategy > 0 && result.Strategy != preferredStrategy {
		// Apply fallback logic
		fallbackResult := sa.applyFallbackLogic(result, preferredStrategy, oldHTML, newHTML)
		sa.updateFallbackMetrics()
		return fallbackResult, nil
	}

	return result, nil
}

// QuickAnalyze performs fast strategy analysis for performance-critical scenarios
func (sa *StrategyAnalyzer) QuickAnalyze(oldHTML, newHTML string) (int, error) {
	cacheKey := sa.generateCacheKey(oldHTML, newHTML)

	// Check cache first
	if sa.cacheEnabled {
		if cached := sa.getCachedResult(cacheKey); cached != nil {
			return cached.Strategy, nil
		}
	}

	// Use quick diff for fast analysis
	recommendation, err := sa.differ.QuickDiff(oldHTML, newHTML)
	if err != nil {
		return 0, fmt.Errorf("quick diff failed: %w", err)
	}

	return recommendation.Strategy, nil
}

// applyFallbackLogic implements strategy fallback when preferred strategy doesn't match
func (sa *StrategyAnalyzer) applyFallbackLogic(originalResult *AnalysisResult, preferredStrategy int, _, _ string) *AnalysisResult {
	// Fallback logic: prefer higher-numbered strategies (more general)
	finalStrategy := originalResult.Strategy
	fallbackReason := ""

	if preferredStrategy > originalResult.Strategy {
		// User wants a more general strategy - allow it
		finalStrategy = preferredStrategy
		fallbackReason = fmt.Sprintf("Upgraded from strategy %d to %d per preference", originalResult.Strategy, preferredStrategy)
	} else {
		// User wants a more specific strategy - analyze if it's safe
		if sa.canDowngradeStrategy(originalResult, preferredStrategy) {
			finalStrategy = preferredStrategy
			fallbackReason = fmt.Sprintf("Downgraded from strategy %d to %d (compatible)", originalResult.Strategy, preferredStrategy)
		} else {
			// Keep original strategy for safety
			fallbackReason = fmt.Sprintf("Kept strategy %d (downgrade to %d unsafe)", originalResult.Strategy, preferredStrategy)
		}
	}

	// Create fallback result
	fallbackResult := &AnalysisResult{
		Strategy:       finalStrategy,
		Recommendation: originalResult.Recommendation,
		DiffResult:     originalResult.DiffResult,
		Confidence:     1.0, // Deterministic rules
		FallbackReason: fallbackReason,
		CacheHit:       originalResult.CacheHit,
		AnalysisTime:   originalResult.AnalysisTime,
		UsesFallback:   finalStrategy != originalResult.Strategy,
	}

	return fallbackResult
}

// canDowngradeStrategy checks if it's safe to downgrade to a more specific strategy
func (sa *StrategyAnalyzer) canDowngradeStrategy(result *AnalysisResult, targetStrategy int) bool {
	// Conservative approach: only allow downgrades for simple patterns
	switch targetStrategy {
	case 1: // Static/Dynamic
		return result.Recommendation.Pattern == diff.PatternStaticDynamic
	case 2: // Marker Compilation
		return result.Recommendation.Pattern == diff.PatternMarkerizable || result.Recommendation.Pattern == diff.PatternStaticDynamic
	case 3: // Granular Operations
		return result.Recommendation.Pattern != diff.PatternReplacement
	case 4: // Fragment Replacement
		return true // Strategy 4 handles everything
	default:
		return false
	}
}

// generateCacheKey creates a cache key from HTML content
func (sa *StrategyAnalyzer) generateCacheKey(oldHTML, newHTML string) string {
	combined := oldHTML + "|" + newHTML
	hash := md5.Sum([]byte(combined))
	return fmt.Sprintf("%x", hash)
}

// getCachedResult retrieves a cached analysis result if valid
func (sa *StrategyAnalyzer) getCachedResult(key string) *AnalysisResult {
	sa.cacheMutex.RLock()
	defer sa.cacheMutex.RUnlock()

	entry, exists := sa.cache[key]
	if !exists {
		return nil
	}

	// Check TTL
	if time.Since(entry.Timestamp) > sa.cacheTTL {
		// Expired - will be cleaned up later
		return nil
	}

	// Update hit counter
	entry.Hits++

	// Mark as cache hit
	result := *entry.Result
	result.CacheHit = true

	return &result
}

// cacheResult stores an analysis result in cache
func (sa *StrategyAnalyzer) cacheResult(key string, result *AnalysisResult) {
	sa.cacheMutex.Lock()
	defer sa.cacheMutex.Unlock()

	// Clean cache if needed
	if len(sa.cache) >= sa.maxCacheSize {
		sa.cleanCache()
	}

	sa.cache[key] = &CacheEntry{
		Result:    result,
		Timestamp: time.Now(),
		Hits:      1,
	}
}

// cleanCache removes expired entries and old entries if cache is full
func (sa *StrategyAnalyzer) cleanCache() {
	now := time.Now()

	// Remove expired entries
	for key, entry := range sa.cache {
		if now.Sub(entry.Timestamp) > sa.cacheTTL {
			delete(sa.cache, key)
		}
	}

	// If still too full, remove least recently used entries
	if len(sa.cache) >= sa.maxCacheSize {
		// Find oldest entries by timestamp
		oldestKey := ""
		oldestTime := now

		for key, entry := range sa.cache {
			if entry.Timestamp.Before(oldestTime) {
				oldestTime = entry.Timestamp
				oldestKey = key
			}
		}

		if oldestKey != "" {
			delete(sa.cache, oldestKey)
		}
	}
}

// trackRuleCorrectness tracks how well deterministic rules are working
func (sa *StrategyAnalyzer) trackRuleCorrectness(diffResult *diff.DiffResult) {
	sa.metricsMutex.Lock()
	defer sa.metricsMutex.Unlock()

	// Track pattern to strategy mapping correctness
	pattern := diffResult.Strategy.Pattern
	strategy := diffResult.Strategy.Strategy

	ruleKey := fmt.Sprintf("%s->strategy%d", pattern, strategy)
	sa.metrics.RuleCorrectness[ruleKey]++

	// Track expected mappings
	expectedMappings := map[diff.PatternType]int{
		diff.PatternStaticDynamic: 1,
		diff.PatternMarkerizable:  2,
		diff.PatternGranular:      3,
		diff.PatternReplacement:   4,
	}

	expectedStrategy, exists := expectedMappings[pattern]
	if exists && expectedStrategy == strategy {
		sa.metrics.RuleCorrectness["correct_mapping"]++
	} else {
		sa.metrics.RuleCorrectness["unexpected_mapping"]++
	}
}

// updateMetrics updates analysis metrics
func (sa *StrategyAnalyzer) updateMetrics(result *AnalysisResult, fromCache bool) {
	sa.metricsMutex.Lock()
	defer sa.metricsMutex.Unlock()

	sa.metrics.TotalAnalyses++
	sa.metrics.StrategyUsage[result.Strategy]++

	if result.Recommendation != nil {
		sa.metrics.PatternDistribution[result.Recommendation.Pattern]++
	}

	// Update cache hit rate
	cacheHits := int64(0)
	for _, entry := range sa.cache {
		cacheHits += entry.Hits
	}

	if sa.metrics.TotalAnalyses > 0 {
		sa.metrics.CacheHitRate = float64(cacheHits) / float64(sa.metrics.TotalAnalyses)
	}

	// Update average analysis time (only for non-cached results)
	if !fromCache {
		totalTime := time.Duration(sa.metrics.TotalAnalyses-1)*sa.metrics.AverageAnalysisTime + result.AnalysisTime
		sa.metrics.AverageAnalysisTime = totalTime / time.Duration(sa.metrics.TotalAnalyses)
	}
}

// updateFallbackMetrics updates fallback-related metrics
func (sa *StrategyAnalyzer) updateFallbackMetrics() {
	sa.metricsMutex.Lock()
	defer sa.metricsMutex.Unlock()

	sa.metrics.FallbackCount++
}

// GetMetrics returns current strategy analysis metrics
func (sa *StrategyAnalyzer) GetMetrics() *StrategyMetrics {
	sa.metricsMutex.RLock()
	defer sa.metricsMutex.RUnlock()

	// Create a copy to avoid concurrent access issues
	metrics := &StrategyMetrics{
		TotalAnalyses:       sa.metrics.TotalAnalyses,
		StrategyUsage:       make(map[int]int64),
		PatternDistribution: make(map[diff.PatternType]int64),
		FallbackCount:       sa.metrics.FallbackCount,
		CacheHitRate:        sa.metrics.CacheHitRate,
		AverageAnalysisTime: sa.metrics.AverageAnalysisTime,
		RuleCorrectness:     make(map[string]int64),
		LastReset:           sa.metrics.LastReset,
	}

	maps.Copy(metrics.StrategyUsage, sa.metrics.StrategyUsage)

	maps.Copy(metrics.PatternDistribution, sa.metrics.PatternDistribution)

	maps.Copy(metrics.RuleCorrectness, sa.metrics.RuleCorrectness)

	return metrics
}

// ResetMetrics resets all metrics
func (sa *StrategyAnalyzer) ResetMetrics() {
	sa.metricsMutex.Lock()
	defer sa.metricsMutex.Unlock()

	sa.metrics = &StrategyMetrics{
		StrategyUsage:       make(map[int]int64),
		PatternDistribution: make(map[diff.PatternType]int64),
		RuleCorrectness:     make(map[string]int64),
		LastReset:           time.Now(),
	}
}

// ClearCache clears the analysis cache
func (sa *StrategyAnalyzer) ClearCache() {
	sa.cacheMutex.Lock()
	defer sa.cacheMutex.Unlock()

	sa.cache = make(map[string]*CacheEntry)
}

// SetCacheEnabled enables or disables caching
func (sa *StrategyAnalyzer) SetCacheEnabled(enabled bool) {
	sa.cacheEnabled = enabled
}

// SetCacheTTL sets the cache time-to-live duration
func (sa *StrategyAnalyzer) SetCacheTTL(ttl time.Duration) {
	sa.cacheTTL = ttl
}

// GetCacheStats returns cache statistics
func (sa *StrategyAnalyzer) GetCacheStats() map[string]any {
	sa.cacheMutex.RLock()
	defer sa.cacheMutex.RUnlock()

	totalHits := int64(0)
	for _, entry := range sa.cache {
		totalHits += entry.Hits
	}

	return map[string]any{
		"cache_size":    len(sa.cache),
		"max_size":      sa.maxCacheSize,
		"total_hits":    totalHits,
		"cache_enabled": sa.cacheEnabled,
		"ttl_seconds":   sa.cacheTTL.Seconds(),
	}
}
