package metrics

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector()

	if collector == nil {
		t.Fatal("NewCollector() returned nil")
	}

	if collector.applicationMetrics == nil {
		t.Fatal("applicationMetrics not initialized")
	}

	if collector.operationCounters == nil {
		t.Fatal("operationCounters not initialized")
	}

	// Check initial values
	metrics := collector.GetMetrics()
	if metrics.HTMLDiffAccuracyScore != 0.0 {
		t.Errorf("Expected initial accuracy score 0.0, got %f", metrics.HTMLDiffAccuracyScore)
	}

	if metrics.AverageCompressionRatio != 1.0 {
		t.Errorf("Expected initial compression ratio 1.0, got %f", metrics.AverageCompressionRatio)
	}
}

func TestPageManagementMetrics(t *testing.T) {
	collector := NewCollector()

	// Test page creation
	collector.IncrementPageCreated()
	collector.IncrementPageCreated()
	collector.IncrementPageCreated()

	metrics := collector.GetMetrics()
	if metrics.PagesCreated != 3 {
		t.Errorf("Expected 3 pages created, got %d", metrics.PagesCreated)
	}

	if metrics.ActivePages != 3 {
		t.Errorf("Expected 3 active pages, got %d", metrics.ActivePages)
	}

	if metrics.MaxConcurrentPages != 3 {
		t.Errorf("Expected max concurrent pages 3, got %d", metrics.MaxConcurrentPages)
	}

	// Test page destruction
	collector.IncrementPageDestroyed()
	metrics = collector.GetMetrics()

	if metrics.PagesDestroyed != 1 {
		t.Errorf("Expected 1 page destroyed, got %d", metrics.PagesDestroyed)
	}

	if metrics.ActivePages != 2 {
		t.Errorf("Expected 2 active pages after destruction, got %d", metrics.ActivePages)
	}

	// Max concurrent should remain the same
	if metrics.MaxConcurrentPages != 3 {
		t.Errorf("Expected max concurrent pages to remain 3, got %d", metrics.MaxConcurrentPages)
	}
}

func TestTokenMetrics(t *testing.T) {
	collector := NewCollector()

	// Test token generation
	collector.IncrementTokenGenerated()
	collector.IncrementTokenGenerated()

	// Test token verification
	collector.IncrementTokenVerified()

	// Test token failure
	collector.IncrementTokenFailure()

	metrics := collector.GetMetrics()

	if metrics.TokensGenerated != 2 {
		t.Errorf("Expected 2 tokens generated, got %d", metrics.TokensGenerated)
	}

	if metrics.TokensVerified != 1 {
		t.Errorf("Expected 1 token verified, got %d", metrics.TokensVerified)
	}

	if metrics.TokenFailures != 1 {
		t.Errorf("Expected 1 token failure, got %d", metrics.TokenFailures)
	}

	// Test success rate calculation
	successRate := collector.GetTokenSuccessRate()
	expectedRate := 50.0 // 1 success out of 2 total (1 verified + 1 failed)
	if successRate != expectedRate {
		t.Errorf("Expected token success rate %.1f%%, got %.1f%%", expectedRate, successRate)
	}
}

func TestHTMLDiffingMetrics(t *testing.T) {
	collector := NewCollector()

	// Test HTML diff operations
	collector.RecordHTMLDiffPerformed(10*time.Millisecond, 0.95)
	collector.RecordHTMLDiffPerformed(15*time.Millisecond, 0.90)
	collector.RecordHTMLDiffPerformed(12*time.Millisecond, 0.98)

	// Test HTML diff error
	collector.RecordHTMLDiffError()

	// Test change pattern detection
	collector.RecordChangePatternDetection()
	collector.RecordChangePatternDetection()

	metrics := collector.GetMetrics()

	if metrics.HTMLDiffsPerformed != 3 {
		t.Errorf("Expected 3 HTML diffs performed, got %d", metrics.HTMLDiffsPerformed)
	}

	if metrics.HTMLDiffErrors != 1 {
		t.Errorf("Expected 1 HTML diff error, got %d", metrics.HTMLDiffErrors)
	}

	if metrics.ChangePatternDetections != 2 {
		t.Errorf("Expected 2 change pattern detections, got %d", metrics.ChangePatternDetections)
	}

	// Check average time calculation
	expectedAvg := (10 + 15 + 12) * time.Millisecond / 3
	if metrics.HTMLDiffAverageTime != expectedAvg {
		t.Errorf("Expected average diff time %v, got %v", expectedAvg, metrics.HTMLDiffAverageTime)
	}

	// Check accuracy score (simple moving average)
	expectedAccuracy := (0.95 + 0.90 + 0.98) / 3
	if metrics.HTMLDiffAccuracyScore != expectedAccuracy {
		t.Errorf("Expected accuracy score %.3f, got %.3f", expectedAccuracy, metrics.HTMLDiffAccuracyScore)
	}

	// Test success rate
	successRate := collector.GetHTMLDiffSuccessRate()
	expectedRate := 75.0 // 3 successes out of 4 total operations
	if successRate != expectedRate {
		t.Errorf("Expected HTML diff success rate %.1f%%, got %.1f%%", expectedRate, successRate)
	}
}

func TestStrategyUsageMetrics(t *testing.T) {
	collector := NewCollector()

	// Test strategy usage recording
	collector.RecordStrategyUsage("static_dynamic", 5*time.Millisecond)
	collector.RecordStrategyUsage("static_dynamic", 3*time.Millisecond)
	collector.RecordStrategyUsage("markers", 8*time.Millisecond)
	collector.RecordStrategyUsage("granular", 12*time.Millisecond)
	collector.RecordStrategyUsage("replacement", 20*time.Millisecond)

	metrics := collector.GetMetrics()

	if metrics.StaticDynamicUsage != 2 {
		t.Errorf("Expected 2 static_dynamic usage, got %d", metrics.StaticDynamicUsage)
	}

	if metrics.MarkerUsage != 1 {
		t.Errorf("Expected 1 marker usage, got %d", metrics.MarkerUsage)
	}

	if metrics.GranularUsage != 1 {
		t.Errorf("Expected 1 granular usage, got %d", metrics.GranularUsage)
	}

	if metrics.ReplacementUsage != 1 {
		t.Errorf("Expected 1 replacement usage, got %d", metrics.ReplacementUsage)
	}

	// Test strategy distribution
	distribution := collector.GetStrategyDistribution()
	expectedStaticPct := 40.0 // 2 out of 5 total
	if distribution["static_dynamic"] != expectedStaticPct {
		t.Errorf("Expected static_dynamic distribution %.1f%%, got %.1f%%",
			expectedStaticPct, distribution["static_dynamic"])
	}

	expectedMarkerPct := 20.0 // 1 out of 5 total
	if distribution["markers"] != expectedMarkerPct {
		t.Errorf("Expected markers distribution %.1f%%, got %.1f%%",
			expectedMarkerPct, distribution["markers"])
	}

	// Test average selection time
	avgTime := collector.GetAverageStrategySelectionTime()
	expectedAvg := (5 + 3 + 8 + 12 + 20) * time.Millisecond / 5
	if avgTime != expectedAvg {
		t.Errorf("Expected average selection time %v, got %v", expectedAvg, avgTime)
	}
}

func TestBandwidthSavingsMetrics(t *testing.T) {
	collector := NewCollector()

	// Record strategy usage first (required for efficiency calculation)
	collector.RecordStrategyUsage("static_dynamic", 5*time.Millisecond)
	collector.RecordStrategyUsage("markers", 8*time.Millisecond)
	collector.RecordStrategyUsage("granular", 12*time.Millisecond)
	collector.RecordStrategyUsage("replacement", 20*time.Millisecond)

	// Test bandwidth savings recording
	collector.RecordBandwidthSavings(1000, 200, "static_dynamic") // 80% savings
	collector.RecordBandwidthSavings(800, 400, "markers")         // 50% savings
	collector.RecordBandwidthSavings(500, 350, "granular")        // 30% savings
	collector.RecordBandwidthSavings(1200, 1000, "replacement")   // 16.7% savings

	metrics := collector.GetMetrics()

	// Check totals
	expectedOriginal := int64(1000 + 800 + 500 + 1200)
	if metrics.OriginalBytes != expectedOriginal {
		t.Errorf("Expected original bytes %d, got %d", expectedOriginal, metrics.OriginalBytes)
	}

	expectedCompressed := int64(200 + 400 + 350 + 1000)
	if metrics.CompressedBytes != expectedCompressed {
		t.Errorf("Expected compressed bytes %d, got %d", expectedCompressed, metrics.CompressedBytes)
	}

	expectedSaved := expectedOriginal - expectedCompressed
	if metrics.TotalBytesSaved != expectedSaved {
		t.Errorf("Expected bytes saved %d, got %d", expectedSaved, metrics.TotalBytesSaved)
	}

	// Check strategy-specific savings
	if metrics.StaticDynamicSavings != 800 {
		t.Errorf("Expected static_dynamic savings 800, got %d", metrics.StaticDynamicSavings)
	}

	if metrics.MarkerSavings != 400 {
		t.Errorf("Expected marker savings 400, got %d", metrics.MarkerSavings)
	}

	if metrics.GranularSavings != 150 {
		t.Errorf("Expected granular savings 150, got %d", metrics.GranularSavings)
	}

	if metrics.ReplacementSavings != 200 {
		t.Errorf("Expected replacement savings 200, got %d", metrics.ReplacementSavings)
	}

	// Check percentage calculation
	expectedPct := float64(expectedSaved) / float64(expectedOriginal) * 100.0
	if metrics.BandwidthSavingsPct != expectedPct {
		t.Errorf("Expected bandwidth savings %.2f%%, got %.2f%%",
			expectedPct, metrics.BandwidthSavingsPct)
	}

	// Check compression ratio
	expectedRatio := float64(expectedCompressed) / float64(expectedOriginal)
	if metrics.AverageCompressionRatio != expectedRatio {
		t.Errorf("Expected compression ratio %.3f, got %.3f",
			expectedRatio, metrics.AverageCompressionRatio)
	}

	// Test strategy efficiency ratios
	efficiencyRatios := collector.GetStrategyEfficiencyRatios()
	expectedStaticEff := 800.0 / 1.0 // 800 bytes saved per usage
	if efficiencyRatios["static_dynamic"] != expectedStaticEff {
		t.Errorf("Expected static_dynamic efficiency %.1f, got %.1f",
			expectedStaticEff, efficiencyRatios["static_dynamic"])
	}
}

func TestMemoryMetrics(t *testing.T) {
	collector := NewCollector()

	// Test memory updates
	collector.UpdateMemoryUsage(1024*1024, 512*1024) // 1MB total, 512KB average
	collector.IncrementPageCreated()
	collector.IncrementPageCreated()

	metrics := collector.GetMetrics()

	if metrics.TotalMemoryUsage != 1024*1024 {
		t.Errorf("Expected total memory 1MB, got %d", metrics.TotalMemoryUsage)
	}

	if metrics.AveragePageMemory != 512*1024 {
		t.Errorf("Expected average page memory 512KB, got %d", metrics.AveragePageMemory)
	}

	// Test memory efficiency calculation
	efficiency := collector.GetMemoryEfficiency()
	expectedEff := float64(1024*1024) / float64(2) // Total memory / active pages
	if efficiency != expectedEff {
		t.Errorf("Expected memory efficiency %.1f, got %.1f", expectedEff, efficiency)
	}
}

func TestCleanupMetrics(t *testing.T) {
	collector := NewCollector()

	// Test cleanup operations
	collector.IncrementCleanupOperation(5)
	collector.IncrementCleanupOperation(3)
	collector.IncrementCleanupOperation(2)

	metrics := collector.GetMetrics()

	if metrics.CleanupOperations != 3 {
		t.Errorf("Expected 3 cleanup operations, got %d", metrics.CleanupOperations)
	}

	if metrics.ExpiredPagesRemoved != 10 {
		t.Errorf("Expected 10 expired pages removed, got %d", metrics.ExpiredPagesRemoved)
	}
}

func TestCustomCounters(t *testing.T) {
	collector := NewCollector()

	// Test custom counters
	collector.IncrementCustomCounter("custom_operation")
	collector.IncrementCustomCounter("custom_operation")
	collector.IncrementCustomCounter("another_operation")

	counters := collector.GetCustomCounters()

	if counters["custom_operation"] != 2 {
		t.Errorf("Expected custom_operation count 2, got %d", counters["custom_operation"])
	}

	if counters["another_operation"] != 1 {
		t.Errorf("Expected another_operation count 1, got %d", counters["another_operation"])
	}
}

func TestPrometheusExport(t *testing.T) {
	collector := NewCollector()

	// Add some sample data
	collector.IncrementPageCreated()
	collector.IncrementTokenGenerated()
	collector.RecordHTMLDiffPerformed(10*time.Millisecond, 0.95)
	collector.RecordStrategyUsage("static_dynamic", 5*time.Millisecond)
	collector.RecordBandwidthSavings(1000, 200, "static_dynamic")

	// Test Prometheus JSON export
	jsonExport, err := collector.ExportPrometheusJSON()
	if err != nil {
		t.Fatalf("Failed to export Prometheus JSON: %v", err)
	}

	var promMetrics PrometheusMetrics
	if err := json.Unmarshal([]byte(jsonExport), &promMetrics); err != nil {
		t.Fatalf("Failed to parse Prometheus JSON: %v", err)
	}

	if len(promMetrics.Metrics) == 0 {
		t.Error("Expected Prometheus metrics, got empty list")
	}

	// Check for specific metrics
	foundPageMetric := false
	foundStrategyMetric := false
	for _, metric := range promMetrics.Metrics {
		if metric.Name == "livetemplate_pages_created_total" {
			foundPageMetric = true
			// Accept both int64 and float64 types (JSON marshaling may convert)
			switch v := metric.Value.(type) {
			case int64:
				if v != 1 {
					t.Errorf("Expected pages created metric value 1, got %d", v)
				}
			case float64:
				if v != 1.0 {
					t.Errorf("Expected pages created metric value 1, got %f", v)
				}
			default:
				t.Errorf("Expected pages created metric value 1, got %v (type %T)", metric.Value, metric.Value)
			}
		}
		if metric.Name == "livetemplate_strategy_usage_total" &&
			metric.Labels != nil && metric.Labels["strategy"] == "static_dynamic" {
			foundStrategyMetric = true
			// Accept both int64 and float64 types (JSON marshaling may convert)
			switch v := metric.Value.(type) {
			case int64:
				if v != 1 {
					t.Errorf("Expected strategy usage metric value 1, got %d", v)
				}
			case float64:
				if v != 1.0 {
					t.Errorf("Expected strategy usage metric value 1, got %f", v)
				}
			default:
				t.Errorf("Expected strategy usage metric value 1, got %v (type %T)", metric.Value, metric.Value)
			}
		}
	}

	if !foundPageMetric {
		t.Error("Page metric not found in Prometheus export")
	}

	if !foundStrategyMetric {
		t.Error("Strategy metric not found in Prometheus export")
	}

	// Test Prometheus text export
	textExport := collector.ExportPrometheusText()
	if textExport == "" {
		t.Error("Prometheus text export is empty")
	}

	// Check for required format elements
	if !strings.Contains(textExport, "# HELP") {
		t.Error("Prometheus text export missing HELP comments")
	}

	if !strings.Contains(textExport, "# TYPE") {
		t.Error("Prometheus text export missing TYPE comments")
	}

	if !strings.Contains(textExport, "livetemplate_pages_created_total") {
		t.Error("Prometheus text export missing pages created metric")
	}
}

func TestMetricsReset(t *testing.T) {
	collector := NewCollector()

	// Add some data
	collector.IncrementPageCreated()
	collector.IncrementTokenGenerated()
	collector.RecordHTMLDiffPerformed(10*time.Millisecond, 0.95)
	collector.RecordStrategyUsage("static_dynamic", 5*time.Millisecond)
	collector.RecordBandwidthSavings(1000, 200, "static_dynamic")
	collector.IncrementCustomCounter("test_counter")

	// Verify data exists
	metrics := collector.GetMetrics()
	if metrics.PagesCreated == 0 {
		t.Error("Expected non-zero pages created before reset")
	}

	// Reset all metrics
	collector.Reset()

	// Verify reset
	metrics = collector.GetMetrics()
	if metrics.PagesCreated != 0 {
		t.Errorf("Expected pages created to be 0 after reset, got %d", metrics.PagesCreated)
	}

	if metrics.HTMLDiffsPerformed != 0 {
		t.Errorf("Expected HTML diffs to be 0 after reset, got %d", metrics.HTMLDiffsPerformed)
	}

	if metrics.StaticDynamicUsage != 0 {
		t.Errorf("Expected strategy usage to be 0 after reset, got %d", metrics.StaticDynamicUsage)
	}

	if metrics.TotalBytesSaved != 0 {
		t.Errorf("Expected bytes saved to be 0 after reset, got %d", metrics.TotalBytesSaved)
	}

	// Check custom counters reset
	counters := collector.GetCustomCounters()
	if len(counters) != 0 {
		t.Errorf("Expected custom counters to be empty after reset, got %d", len(counters))
	}

	// Check accuracy score and compression ratio reset to defaults
	if metrics.HTMLDiffAccuracyScore != 0.0 {
		t.Errorf("Expected accuracy score to be 0.0 after reset, got %f", metrics.HTMLDiffAccuracyScore)
	}

	if metrics.AverageCompressionRatio != 1.0 {
		t.Errorf("Expected compression ratio to be 1.0 after reset, got %f", metrics.AverageCompressionRatio)
	}
}

func TestErrorRateCalculations(t *testing.T) {
	collector := NewCollector()

	// Test with no operations (should return 0% error rate)
	errorRate := collector.GetErrorRate()
	if errorRate != 0.0 {
		t.Errorf("Expected 0%% error rate with no operations, got %.1f%%", errorRate)
	}

	// Add some successful operations
	collector.IncrementFragmentGenerated()
	collector.IncrementFragmentGenerated()
	collector.IncrementFragmentGenerated()

	// Add some errors
	collector.IncrementGenerationError()

	// Calculate error rate: 1 error / (3 successful + 1 error) = 25%
	errorRate = collector.GetErrorRate()
	expectedErrorRate := 25.0
	if errorRate != expectedErrorRate {
		t.Errorf("Expected %.1f%% error rate, got %.1f%%", expectedErrorRate, errorRate)
	}
}

func TestConcurrentAccess(t *testing.T) {
	collector := NewCollector()

	// Test concurrent access to metrics
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			collector.IncrementPageCreated()
			collector.RecordHTMLDiffPerformed(time.Millisecond, 0.9)
			collector.RecordStrategyUsage("static_dynamic", time.Microsecond)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = collector.GetMetrics()
			_ = collector.GetStrategyDistribution()
			_ = collector.GetCustomCounters()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	metrics := collector.GetMetrics()
	if metrics.PagesCreated != 100 {
		t.Errorf("Expected 100 pages created, got %d", metrics.PagesCreated)
	}

	if metrics.HTMLDiffsPerformed != 100 {
		t.Errorf("Expected 100 HTML diffs, got %d", metrics.HTMLDiffsPerformed)
	}
}
