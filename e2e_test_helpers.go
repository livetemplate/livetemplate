package livetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// E2ETestConfig holds configuration for E2E tests in CI/CD
type E2ETestConfig struct {
	ScreenshotsEnabled bool
	ArtifactsDir       string
	ScreenshotsDir     string
	ChromePath         string
	TestTimeout        time.Duration
	RetryAttempts      int
}

// E2ETestMetrics tracks performance metrics during test execution
type E2ETestMetrics struct {
	TestName        string                 `json:"test_name"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         time.Time              `json:"end_time"`
	Duration        time.Duration          `json:"duration"`
	MemoryUsage     runtime.MemStats       `json:"memory_usage"`
	FragmentMetrics []FragmentMetric       `json:"fragment_metrics"`
	ScreenshotCount int                    `json:"screenshot_count"`
	ErrorCount      int                    `json:"error_count"`
	BrowserActions  []BrowserActionMetric  `json:"browser_actions"`
	CustomMetrics   map[string]interface{} `json:"custom_metrics"`
	Success         bool                   `json:"success"`
	FailureReason   string                 `json:"failure_reason,omitempty"`
}

// FragmentMetric tracks individual fragment performance
type FragmentMetric struct {
	FragmentID       string        `json:"fragment_id"`
	Strategy         string        `json:"strategy"`
	GenerationTime   time.Duration `json:"generation_time"`
	Size             int           `json:"size"`
	CompressionRatio float64       `json:"compression_ratio"`
	CacheHit         bool          `json:"cache_hit"`
}

// BrowserActionMetric tracks browser interaction performance
type BrowserActionMetric struct {
	Action    string        `json:"action"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// E2ETestHelper provides utilities for enhanced E2E testing in CI/CD
type E2ETestHelper struct {
	config  E2ETestConfig
	metrics E2ETestMetrics
	mu      sync.RWMutex
}

// NewE2ETestHelper creates a new test helper with CI/CD configuration
func NewE2ETestHelper(testName string) *E2ETestHelper {
	config := E2ETestConfig{
		ScreenshotsEnabled: os.Getenv("LIVETEMPLATE_E2E_SCREENSHOTS") == "true",
		ArtifactsDir:       os.Getenv("LIVETEMPLATE_E2E_ARTIFACTS"),
		TestTimeout:        10 * time.Minute,
		RetryAttempts:      3,
	}

	// Set default artifacts directory if not specified
	if config.ArtifactsDir == "" {
		config.ArtifactsDir = "./test-artifacts"
	}

	config.ScreenshotsDir = filepath.Join(config.ArtifactsDir, "screenshots")
	config.ChromePath = os.Getenv("CHROME_BIN")

	// Create directories
	if err := os.MkdirAll(config.ArtifactsDir, 0755); err != nil {
		// Log error but continue - artifacts directory is not critical for core functionality
		fmt.Printf("Warning: Failed to create artifacts directory %s: %v\n", config.ArtifactsDir, err)
	}
	if config.ScreenshotsEnabled {
		if err := os.MkdirAll(config.ScreenshotsDir, 0755); err != nil {
			fmt.Printf("Warning: Failed to create screenshots directory %s: %v\n", config.ScreenshotsDir, err)
			// Disable screenshots if directory can't be created
			config.ScreenshotsEnabled = false
		}
	}

	return &E2ETestHelper{
		config: config,
		metrics: E2ETestMetrics{
			TestName:        testName,
			StartTime:       time.Now(),
			FragmentMetrics: make([]FragmentMetric, 0),
			BrowserActions:  make([]BrowserActionMetric, 0),
			CustomMetrics:   make(map[string]interface{}),
		},
	}
}

// StartTest initializes test metrics collection
func (h *E2ETestHelper) StartTest(t *testing.T) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.metrics.StartTime = time.Now()
	runtime.ReadMemStats(&h.metrics.MemoryUsage)

	t.Logf("🚀 Starting E2E test: %s", h.metrics.TestName)
	if h.config.ScreenshotsEnabled {
		t.Logf("📸 Screenshots enabled - will capture on failures")
	}
	t.Logf("📁 Artifacts directory: %s", h.config.ArtifactsDir)
}

// FinishTest completes test metrics collection and saves results
func (h *E2ETestHelper) FinishTest(t *testing.T, success bool, failureReason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.metrics.EndTime = time.Now()
	h.metrics.Duration = h.metrics.EndTime.Sub(h.metrics.StartTime)
	h.metrics.Success = success
	h.metrics.FailureReason = failureReason

	// Save metrics to file
	metricsFile := filepath.Join(h.config.ArtifactsDir, fmt.Sprintf("test-metrics-%s.json", h.metrics.TestName))
	if data, err := json.MarshalIndent(h.metrics, "", "  "); err == nil {
		if err := os.WriteFile(metricsFile, data, 0644); err != nil {
			fmt.Printf("Warning: Failed to write metrics file %s: %v\n", metricsFile, err)
		}
	} else {
		fmt.Printf("Warning: Failed to marshal test metrics: %v\n", err)
	}

	// Log summary
	statusIcon := "✅"
	if !success {
		statusIcon = "❌"
	}

	t.Logf("%s Test completed: %s (duration: %v)", statusIcon, h.metrics.TestName, h.metrics.Duration)
	t.Logf("📊 Fragments processed: %d", len(h.metrics.FragmentMetrics))
	t.Logf("🖱️ Browser actions: %d", len(h.metrics.BrowserActions))
	t.Logf("📸 Screenshots captured: %d", h.metrics.ScreenshotCount)
	t.Logf("❌ Errors encountered: %d", h.metrics.ErrorCount)

	if !success && failureReason != "" {
		t.Logf("💥 Failure reason: %s", failureReason)
	}
}

// CaptureScreenshot takes a screenshot with the given name
func (h *E2ETestHelper) CaptureScreenshot(ctx context.Context, name string) error {
	if !h.config.ScreenshotsEnabled {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s-%s.png", h.metrics.TestName, name, timestamp)
	screenshotPath := filepath.Join(h.config.ScreenshotsDir, filename)

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	if err := os.WriteFile(screenshotPath, buf, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot: %w", err)
	}

	h.metrics.ScreenshotCount++
	return nil
}

// CaptureFailureScreenshot captures a screenshot on test failure
func (h *E2ETestHelper) CaptureFailureScreenshot(ctx context.Context, t *testing.T, reason string) {
	if err := h.CaptureScreenshot(ctx, "failure"); err != nil {
		t.Logf("⚠️ Failed to capture failure screenshot: %v", err)
	} else {
		t.Logf("📸 Failure screenshot captured for: %s", reason)
	}
}

// RecordFragmentMetric records performance metrics for a fragment operation
func (h *E2ETestHelper) RecordFragmentMetric(fragmentID, strategy string, generationTime time.Duration, size int, compressionRatio float64, cacheHit bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	metric := FragmentMetric{
		FragmentID:       fragmentID,
		Strategy:         strategy,
		GenerationTime:   generationTime,
		Size:             size,
		CompressionRatio: compressionRatio,
		CacheHit:         cacheHit,
	}

	h.metrics.FragmentMetrics = append(h.metrics.FragmentMetrics, metric)
}

// RecordBrowserAction records timing for browser actions
func (h *E2ETestHelper) RecordBrowserAction(action string, duration time.Duration, success bool, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	metric := BrowserActionMetric{
		Action:    action,
		Timestamp: time.Now(),
		Duration:  duration,
		Success:   success,
	}

	if err != nil {
		metric.Error = err.Error()
		h.metrics.ErrorCount++
	}

	h.metrics.BrowserActions = append(h.metrics.BrowserActions, metric)
}

// SetCustomMetric sets a custom metric value
func (h *E2ETestHelper) SetCustomMetric(key string, value interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.metrics.CustomMetrics[key] = value
}

// CreateBrowserContext creates a Chrome context with optimized settings for CI/CD
func (h *E2ETestHelper) CreateBrowserContext() (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.NoSandbox, // Required for CI environments
		chromedp.Headless,  // Always headless in CI
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent("LiveTemplate-E2E-Test/1.0"),
	}

	// Add Chrome binary path if specified
	if h.config.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(h.config.ChromePath))
	}

	// Add additional CI-specific options
	opts = append(opts,
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-web-security", true), // For testing only
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-background-networking", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	// Create context with timeout
	ctx, cancel := chromedp.NewContext(allocCtx)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, h.config.TestTimeout)

	// Combine cancellation functions
	combinedCancel := func() {
		timeoutCancel()
		cancel()
		allocCancel()
	}

	return timeoutCtx, combinedCancel
}

// RunWithRetry executes a test function with retry logic for flakiness handling
func (h *E2ETestHelper) RunWithRetry(t *testing.T, testFunc func() error) error {
	var lastErr error

	for attempt := 1; attempt <= h.config.RetryAttempts; attempt++ {
		t.Logf("🔄 Test attempt %d/%d", attempt, h.config.RetryAttempts)

		if err := testFunc(); err != nil {
			lastErr = err
			h.metrics.ErrorCount++

			t.Logf("❌ Attempt %d failed: %v", attempt, err)

			// Capture failure screenshot on each failed attempt
			if ctx := context.Background(); h.config.ScreenshotsEnabled {
				h.CaptureFailureScreenshot(ctx, t, fmt.Sprintf("attempt-%d", attempt))
			}

			// Wait before retry (exponential backoff)
			if attempt < h.config.RetryAttempts {
				waitTime := time.Duration(attempt*attempt) * time.Second
				t.Logf("⏰ Waiting %v before retry...", waitTime)
				time.Sleep(waitTime)
			}
			continue
		}

		// Success
		if attempt > 1 {
			t.Logf("✅ Test passed on attempt %d (flaky test detected)", attempt)
			h.SetCustomMetric("flaky_test", true)
			h.SetCustomMetric("successful_attempt", attempt)
		}
		return nil
	}

	return fmt.Errorf("test failed after %d attempts, last error: %w", h.config.RetryAttempts, lastErr)
}

// GeneratePerformanceReport creates a detailed performance report
func (h *E2ETestHelper) GeneratePerformanceReport() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var report strings.Builder

	report.WriteString("# E2E Test Performance Report\n\n")
	report.WriteString(fmt.Sprintf("**Test:** %s\n", h.metrics.TestName))
	report.WriteString(fmt.Sprintf("**Duration:** %v\n", h.metrics.Duration))
	report.WriteString(fmt.Sprintf("**Success:** %t\n", h.metrics.Success))
	report.WriteString(fmt.Sprintf("**Screenshots:** %d\n", h.metrics.ScreenshotCount))
	report.WriteString(fmt.Sprintf("**Errors:** %d\n\n", h.metrics.ErrorCount))

	// Fragment performance
	if len(h.metrics.FragmentMetrics) > 0 {
		report.WriteString("## Fragment Performance\n\n")
		report.WriteString("| Fragment | Strategy | Generation Time | Size | Compression | Cache Hit |\n")
		report.WriteString("|----------|----------|-----------------|------|-------------|----------|\n")

		for _, fm := range h.metrics.FragmentMetrics {
			cacheStatus := "❌"
			if fm.CacheHit {
				cacheStatus = "✅"
			}
			report.WriteString(fmt.Sprintf("| %s | %s | %v | %d bytes | %.2f%% | %s |\n",
				fm.FragmentID, fm.Strategy, fm.GenerationTime,
				fm.Size, fm.CompressionRatio*100, cacheStatus))
		}
		report.WriteString("\n")
	}

	// Browser actions
	if len(h.metrics.BrowserActions) > 0 {
		report.WriteString("## Browser Actions\n\n")
		report.WriteString("| Action | Duration | Status | Error |\n")
		report.WriteString("|--------|----------|-----------|-------|\n")

		for _, ba := range h.metrics.BrowserActions {
			status := "✅"
			if !ba.Success {
				status = "❌"
			}
			errorMsg := ba.Error
			if errorMsg == "" {
				errorMsg = "-"
			}
			report.WriteString(fmt.Sprintf("| %s | %v | %s | %s |\n",
				ba.Action, ba.Duration, status, errorMsg))
		}
		report.WriteString("\n")
	}

	// Custom metrics
	if len(h.metrics.CustomMetrics) > 0 {
		report.WriteString("## Custom Metrics\n\n")
		for key, value := range h.metrics.CustomMetrics {
			report.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
		}
		report.WriteString("\n")
	}

	return report.String()
}

// SavePerformanceReport saves the performance report to file
func (h *E2ETestHelper) SavePerformanceReport(t *testing.T) {
	report := h.GeneratePerformanceReport()
	reportFile := filepath.Join(h.config.ArtifactsDir, fmt.Sprintf("performance-report-%s.md", h.metrics.TestName))

	if err := os.WriteFile(reportFile, []byte(report), 0644); err != nil {
		t.Logf("⚠️ Failed to save performance report: %v", err)
	} else {
		t.Logf("📊 Performance report saved: %s", reportFile)
	}
}

// E2ETestWithHelper is a convenience function that wraps test execution with full helper functionality
func E2ETestWithHelper(t *testing.T, testName string, testFunc func(*E2ETestHelper) error) {
	helper := NewE2ETestHelper(testName)
	helper.StartTest(t)

	success := false
	failureReason := ""

	defer func() {
		helper.FinishTest(t, success, failureReason)
		helper.SavePerformanceReport(t)
	}()

	// Run test with retry logic
	if err := helper.RunWithRetry(t, func() error {
		return testFunc(helper)
	}); err != nil {
		failureReason = err.Error()
		t.Fatalf("E2E test failed: %v", err)
		return
	}

	success = true
}
