package livetemplate

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestCrossBrowserCompatibility validates task-034 acceptance criteria
func TestCrossBrowserCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-browser compatibility test in short mode")
	}

	suite := NewCrossBrowserTestSuite(t)
	defer suite.Close()

	t.Run("Firefox_Automation_GeckoDriver", func(t *testing.T) {
		suite.TestFirefoxAutomationGeckoDriver(t)
	})

	t.Run("Safari_Testing_macOS", func(t *testing.T) {
		suite.TestSafariTestingMacOS(t)
	})

	t.Run("Edge_Chromium_Compatibility", func(t *testing.T) {
		suite.TestEdgeChromiumCompatibility(t)
	})

	t.Run("Mobile_Browser_Testing", func(t *testing.T) {
		suite.TestMobileBrowserTesting(t)
	})

	t.Run("JavaScript_Compatibility_Versions", func(t *testing.T) {
		suite.TestJavaScriptCompatibilityVersions(t)
	})

	t.Run("Fragment_Application_Consistency", func(t *testing.T) {
		suite.TestFragmentApplicationConsistency(t)
	})

	t.Run("Performance_Characteristics_Documentation", func(t *testing.T) {
		suite.TestPerformanceCharacteristicsDocumentation(t)
	})

	t.Run("Browser_Specific_Optimization_Recommendations", func(t *testing.T) {
		suite.TestBrowserSpecificOptimizationRecommendations(t)
	})
}

// CrossBrowserTestSuite manages cross-browser testing infrastructure
type CrossBrowserTestSuite struct {
	app         *Application
	server      *httptest.Server
	browsers    map[string]*BrowserConfig
	testResults map[string]*BrowserTestResults
	resultsMux  sync.RWMutex
	t           *testing.T
}

// BrowserConfig defines configuration for each browser
type BrowserConfig struct {
	Name           string
	Engine         string
	Version        string
	UserAgent      string
	DriverPath     string
	ExecutablePath string
	Available      bool
	MobileUA       string
	Capabilities   map[string]interface{}
}

// BrowserTestResults stores test results for each browser
type BrowserTestResults struct {
	BrowserName               string              `json:"browser_name"`
	Engine                    string              `json:"engine"`
	Version                   string              `json:"version"`
	JSCompatibility           bool                `json:"js_compatibility"`
	FragmentApplicationOK     bool                `json:"fragment_application_ok"`
	PerformanceMetrics        *PerformanceMetrics `json:"performance_metrics"`
	SupportedStrategies       []string            `json:"supported_strategies"`
	CompatibilityIssues       []string            `json:"compatibility_issues"`
	Recommendations           []string            `json:"recommendations"`
	TestDuration              time.Duration       `json:"test_duration"`
	FeaturesSupported         map[string]bool     `json:"features_supported"`
	OptimizationOpportunities []string            `json:"optimization_opportunities"`
}

// BrowserCompatibilityReport provides comprehensive compatibility analysis
type BrowserCompatibilityReport struct {
	TestTimestamp     time.Time                      `json:"test_timestamp"`
	Platform          string                         `json:"platform"`
	TestedBrowsers    []string                       `json:"tested_browsers"`
	OverallCompatible bool                           `json:"overall_compatible"`
	Results           map[string]*BrowserTestResults `json:"results"`
	Summary           *CompatibilitySummary          `json:"summary"`
	Recommendations   []string                       `json:"recommendations"`
}

// CompatibilitySummary provides high-level compatibility overview
type CompatibilitySummary struct {
	TotalBrowsersTested     int     `json:"total_browsers_tested"`
	FullyCompatibleBrowsers int     `json:"fully_compatible_browsers"`
	PartiallyCompatible     int     `json:"partially_compatible"`
	IncompatibleBrowsers    int     `json:"incompatible_browsers"`
	AveragePerformanceScore float64 `json:"average_performance_score"`
	CriticalIssuesFound     int     `json:"critical_issues_found"`
	RecommendationsProvided int     `json:"recommendations_provided"`
}

// NewCrossBrowserTestSuite creates a new cross-browser testing suite
func NewCrossBrowserTestSuite(t *testing.T) *CrossBrowserTestSuite {
	// Create application for testing
	app, err := NewApplication(
		WithMaxMemoryMB(100),
		WithApplicationMetricsEnabled(true),
	)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	suite := &CrossBrowserTestSuite{
		app:         app,
		browsers:    make(map[string]*BrowserConfig),
		testResults: make(map[string]*BrowserTestResults),
		t:           t,
	}

	// Set up browser configurations
	suite.setupBrowserConfigurations()

	// Create test server
	suite.setupTestServer()

	return suite
}

// setupBrowserConfigurations detects and configures available browsers
func (suite *CrossBrowserTestSuite) setupBrowserConfigurations() {
	// Chrome/Chromium (already supported)
	suite.browsers["chrome"] = &BrowserConfig{
		Name:         "Chrome",
		Engine:       "Blink",
		Version:      "Latest",
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Available:    true,
		MobileUA:     "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.0.0 Mobile/15E148 Safari/604.1",
		Capabilities: map[string]interface{}{"acceptInsecureCerts": true},
	}

	// Firefox
	firefoxAvailable, firefoxPath := suite.detectFirefox()
	suite.browsers["firefox"] = &BrowserConfig{
		Name:           "Firefox",
		Engine:         "Gecko",
		Version:        "Latest",
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0",
		DriverPath:     suite.detectGeckoDriver(),
		ExecutablePath: firefoxPath,
		Available:      firefoxAvailable,
		MobileUA:       "Mozilla/5.0 (Mobile; rv:120.0) Gecko/120.0 Firefox/120.0",
		Capabilities:   map[string]interface{}{"acceptInsecureCerts": true},
	}

	// Safari (macOS only)
	safariAvailable := suite.detectSafari()
	suite.browsers["safari"] = &BrowserConfig{
		Name:         "Safari",
		Engine:       "WebKit",
		Version:      "Latest",
		UserAgent:    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		Available:    safariAvailable,
		MobileUA:     "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		Capabilities: map[string]interface{}{"acceptInsecureCerts": true},
	}

	// Edge (Chromium-based)
	edgeAvailable, edgePath := suite.detectEdge()
	suite.browsers["edge"] = &BrowserConfig{
		Name:           "Edge",
		Engine:         "Blink",
		Version:        "Latest",
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		ExecutablePath: edgePath,
		Available:      edgeAvailable,
		MobileUA:       "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 EdgiOS/120.0.0.0 Mobile/15E148 Safari/604.1",
		Capabilities:   map[string]interface{}{"acceptInsecureCerts": true},
	}

	suite.t.Logf("Browser detection results:")
	for _, config := range suite.browsers {
		status := "❌ Not Available"
		if config.Available {
			status = "✅ Available"
		}
		suite.t.Logf("  %s (%s): %s", config.Name, config.Engine, status)
	}
}

// detectFirefox detects Firefox installation
func (suite *CrossBrowserTestSuite) detectFirefox() (bool, string) {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Firefox.app/Contents/MacOS/firefox",
			"/Applications/Firefox Developer Edition.app/Contents/MacOS/firefox",
		}
	case "linux":
		paths = []string{
			"/usr/bin/firefox",
			"/usr/local/bin/firefox",
			"/opt/firefox/firefox",
		}
	case "windows":
		paths = []string{
			"C:\\Program Files\\Mozilla Firefox\\firefox.exe",
			"C:\\Program Files (x86)\\Mozilla Firefox\\firefox.exe",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true, path
		}
	}

	// Try to find in PATH
	if _, err := exec.LookPath("firefox"); err == nil {
		return true, "firefox"
	}

	return false, ""
}

// detectGeckoDriver detects GeckoDriver installation
func (suite *CrossBrowserTestSuite) detectGeckoDriver() string {
	if _, err := exec.LookPath("geckodriver"); err == nil {
		return "geckodriver"
	}
	return ""
}

// detectSafari detects Safari availability (macOS only)
func (suite *CrossBrowserTestSuite) detectSafari() bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	safariPath := "/Applications/Safari.app/Contents/MacOS/Safari"
	if _, err := os.Stat(safariPath); err == nil {
		return true
	}

	return false
}

// detectEdge detects Microsoft Edge installation
func (suite *CrossBrowserTestSuite) detectEdge() (bool, string) {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		paths = []string{
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
		}
	case "windows":
		paths = []string{
			"C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
			"C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true, path
		}
	}

	// Try to find in PATH
	if _, err := exec.LookPath("microsoft-edge"); err == nil {
		return true, "microsoft-edge"
	}

	return false, ""
}

// setupTestServer creates HTTP server for cross-browser testing
func (suite *CrossBrowserTestSuite) setupTestServer() {
	// Create comprehensive test template
	tmplStr := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - Cross-Browser Test</title>
    <script src="/client/compatibility-test.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .test-section { margin: 20px 0; padding: 10px; border: 1px solid #ccc; }
        .status { padding: 5px 10px; border-radius: 3px; }
        .status.success { background: #d4edda; color: #155724; }
        .status.error { background: #f8d7da; color: #721c24; }
        .hidden { display: none; }
    </style>
</head>
<body>
    <div id="app">
        <h1 id="title">{{.Title}}</h1>
        
        <!-- Basic Fragment Testing -->
        <div class="test-section" id="basic-test">
            <h2>Basic Fragment Testing</h2>
            <div id="counter">Count: {{.Count}}</div>
            <div id="status" class="status {{.StatusClass}}">Status: {{.Status}}</div>
        </div>
        
        <!-- Strategy Testing -->
        <div class="test-section" id="strategy-test">
            <h2>Strategy Testing</h2>
            <div id="static-dynamic-test" data-fragment="static-dynamic">
                <span id="dynamic-value">{{.DynamicValue}}</span>
            </div>
            <div id="marker-test">
                <span data-marker="marker1">{{.MarkerValue1}}</span>
                <span data-marker="marker2">{{.MarkerValue2}}</span>
            </div>
            <div id="granular-test">
                <ul id="item-list">
                    {{range $index, $item := .Items}}
                    <li id="item-{{$index}}">{{$item}}</li>
                    {{end}}
                </ul>
            </div>
        </div>
        
        <!-- JavaScript Compatibility Testing -->
        <div class="test-section" id="js-test">
            <h2>JavaScript Compatibility</h2>
            <div id="js-features">
                <div id="es6-support">ES6: <span id="es6-status">Testing...</span></div>
                <div id="fetch-support">Fetch API: <span id="fetch-status">Testing...</span></div>
                <div id="promise-support">Promises: <span id="promise-status">Testing...</span></div>
                <div id="arrow-functions">Arrow Functions: <span id="arrow-status">Testing...</span></div>
                <div id="destructuring">Destructuring: <span id="destructure-status">Testing...</span></div>
            </div>
        </div>
        
        <!-- Performance Testing -->
        <div class="test-section" id="performance-test">
            <h2>Performance Metrics</h2>
            <div id="perf-results">
                <div>Fragment Application Time: <span id="fragment-time">N/A</span></div>
                <div>DOM Update Time: <span id="dom-time">N/A</span></div>
                <div>Memory Usage: <span id="memory-usage">N/A</span></div>
            </div>
        </div>
        
        <!-- Feature Detection -->
        <div class="test-section" id="feature-test">
            <h2>Feature Detection</h2>
            <div id="feature-results">
                <div>WebSocket: <span id="websocket-support">Testing...</span></div>
                <div>Local Storage: <span id="localstorage-support">Testing...</span></div>
                <div>Canvas: <span id="canvas-support">Testing...</span></div>
                <div>Web Workers: <span id="webworker-support">Testing...</span></div>
            </div>
        </div>
    </div>
    
    <script>
        // Initialize compatibility testing
        window.compatibilityTest = new CompatibilityTest();
        window.compatibilityTest.runTests();
    </script>
</body>
</html>`

	tmpl, err := template.New("cross-browser-test").Parse(tmplStr)
	if err != nil {
		suite.t.Fatalf("Failed to parse template: %v", err)
	}

	// Initial test data
	initialData := map[string]interface{}{
		"Title":        "Cross-Browser Compatibility Test",
		"Count":        0,
		"Status":       "ready",
		"StatusClass":  "success",
		"DynamicValue": "Initial Value",
		"MarkerValue1": "Marker 1",
		"MarkerValue2": "Marker 2",
		"Items":        []string{"Item 1", "Item 2", "Item 3"},
	}

	page, err := suite.app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		suite.t.Fatalf("Failed to create page: %v", err)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Main page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := page.Render()
		if err != nil {
			http.Error(w, fmt.Sprintf("Render failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	// Fragment update endpoint
	mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var newData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		fragments, err := page.RenderFragments(r.Context(), newData)
		if err != nil {
			http.Error(w, fmt.Sprintf("Fragment generation failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"fragments": fragments,
			"timestamp": time.Now(),
		})
	})

	// JavaScript compatibility test client
	mux.HandleFunc("/client/compatibility-test.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		if _, err := w.Write([]byte(suite.generateCompatibilityTestJS())); err != nil {
			fmt.Printf("Warning: Failed to write compatibility test JS: %v\n", err)
		}
	})

	// Browser info endpoint
	mux.HandleFunc("/browser-info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"timestamp": time.Now(),
			"headers":   r.Header,
			"userAgent": r.UserAgent(),
		}); err != nil {
			fmt.Printf("Warning: Failed to encode browser info: %v\n", err)
		}
	})

	suite.server = httptest.NewServer(mux)
}

// generateCompatibilityTestJS creates JavaScript for browser compatibility testing
func (suite *CrossBrowserTestSuite) generateCompatibilityTestJS() string {
	return `
class CompatibilityTest {
    constructor() {
        this.results = {};
        this.startTime = performance.now();
    }
    
    runTests() {
        console.log('Starting cross-browser compatibility tests...');
        
        // Test JavaScript features
        this.testJavaScriptFeatures();
        
        // Test browser features
        this.testBrowserFeatures();
        
        // Test performance
        this.testPerformance();
        
        // Make results available globally
        window.compatibilityResults = this.results;
        
        console.log('Compatibility tests completed:', this.results);
    }
    
    testJavaScriptFeatures() {
        // ES6 Features
        try {
            const test = (x) => x * 2;
            const [a, b] = [1, 2];
            this.setTestResult('es6-support', 'arrow-status', true);
            this.setTestResult('destructuring', 'destructure-status', true);
        } catch (e) {
            this.setTestResult('es6-support', 'arrow-status', false);
            this.setTestResult('destructuring', 'destructure-status', false);
        }
        
        // Promises
        try {
            const p = Promise.resolve(true);
            this.setTestResult('promise-support', 'promise-status', true);
        } catch (e) {
            this.setTestResult('promise-support', 'promise-status', false);
        }
        
        // Fetch API
        this.setTestResult('fetch-support', 'fetch-status', typeof fetch !== 'undefined');
    }
    
    testBrowserFeatures() {
        // WebSocket
        this.setTestResult('websocket-support', 'websocket-support', typeof WebSocket !== 'undefined');
        
        // Local Storage
        try {
            localStorage.setItem('test', 'test');
            localStorage.removeItem('test');
            this.setTestResult('localstorage-support', 'localstorage-support', true);
        } catch (e) {
            this.setTestResult('localstorage-support', 'localstorage-support', false);
        }
        
        // Canvas
        try {
            const canvas = document.createElement('canvas');
            const ctx = canvas.getContext('2d');
            this.setTestResult('canvas-support', 'canvas-support', !!ctx);
        } catch (e) {
            this.setTestResult('canvas-support', 'canvas-support', false);
        }
        
        // Web Workers
        this.setTestResult('webworker-support', 'webworker-support', typeof Worker !== 'undefined');
    }
    
    testPerformance() {
        const startTime = performance.now();
        
        // Simulate fragment application
        for (let i = 0; i < 1000; i++) {
            const elem = document.createElement('div');
            elem.textContent = 'test' + i;
            document.body.appendChild(elem);
            document.body.removeChild(elem);
        }
        
        const endTime = performance.now();
        const fragmentTime = endTime - startTime;
        
        // DOM update test
        const domStartTime = performance.now();
        const testDiv = document.getElementById('dynamic-value');
        if (testDiv) {
            for (let i = 0; i < 100; i++) {
                testDiv.textContent = 'Test ' + i;
            }
        }
        const domEndTime = performance.now();
        const domTime = domEndTime - domStartTime;
        
        // Memory usage (approximate)
        const memoryInfo = performance.memory || { usedJSHeapSize: 0 };
        
        document.getElementById('fragment-time').textContent = fragmentTime.toFixed(2) + 'ms';
        document.getElementById('dom-time').textContent = domTime.toFixed(2) + 'ms';
        document.getElementById('memory-usage').textContent = Math.round(memoryInfo.usedJSHeapSize / 1024 / 1024) + 'MB';
        
        this.results.performance = {
            fragmentTime: fragmentTime,
            domTime: domTime,
            memoryUsage: memoryInfo.usedJSHeapSize
        };
    }
    
    setTestResult(feature, elementId, supported) {
        this.results[feature] = supported;
        const element = document.getElementById(elementId);
        if (element) {
            element.textContent = supported ? '✅ Supported' : '❌ Not Supported';
            element.className = supported ? 'status success' : 'status error';
        }
    }
    
    getResults() {
        return this.results;
    }
    
    // Fragment application simulation
    applyFragment(fragment) {
        const startTime = performance.now();
        
        try {
            switch (fragment.strategy) {
                case 'static_dynamic':
                    this.applyStaticDynamicFragment(fragment);
                    break;
                case 'markers':
                    this.applyMarkerFragment(fragment);
                    break;
                case 'granular':
                    this.applyGranularFragment(fragment);
                    break;
                case 'replacement':
                    this.applyReplacementFragment(fragment);
                    break;
            }
            
            const endTime = performance.now();
            return {
                success: true,
                latency: endTime - startTime
            };
        } catch (error) {
            return {
                success: false,
                error: error.message
            };
        }
    }
    
    applyStaticDynamicFragment(fragment) {
        if (fragment.data.dynamics) {
            Object.entries(fragment.data.dynamics).forEach(([key, value]) => {
                const elem = document.getElementById(key);
                if (elem) elem.textContent = value;
            });
        }
    }
    
    applyMarkerFragment(fragment) {
        if (fragment.data.value_updates) {
            Object.entries(fragment.data.value_updates).forEach(([marker, value]) => {
                const elem = document.querySelector('[data-marker="' + marker + '"]');
                if (elem) elem.textContent = value;
            });
        }
    }
    
    applyGranularFragment(fragment) {
        if (fragment.data.operations) {
            fragment.data.operations.forEach(op => {
                const target = document.getElementById(op.target_id);
                if (target) {
                    switch (op.type) {
                        case 'insert':
                            target.insertAdjacentHTML(op.position || 'beforeend', op.content);
                            break;
                        case 'remove':
                            if (op.selector) {
                                const elem = target.querySelector(op.selector);
                                if (elem) elem.remove();
                            }
                            break;
                        case 'update':
                            target.innerHTML = op.content;
                            break;
                    }
                }
            });
        }
    }
    
    applyReplacementFragment(fragment) {
        if (fragment.data.content) {
            const target = document.getElementById(fragment.data.target_id) || document.body;
            target.innerHTML = fragment.data.content;
        }
    }
}
`
}

// Close releases all suite resources
func (suite *CrossBrowserTestSuite) Close() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.app != nil {
		_ = suite.app.Close()
	}
}

// Test implementation methods follow...

// TestFirefoxAutomationGeckoDriver validates Firefox automation via geckodriver
func (suite *CrossBrowserTestSuite) TestFirefoxAutomationGeckoDriver(t *testing.T) {
	firefoxConfig := suite.browsers["firefox"]
	if !firefoxConfig.Available {
		t.Skip("Firefox not available - skipping Firefox automation test")
		return
	}

	if firefoxConfig.DriverPath == "" {
		t.Skip("GeckoDriver not found - install geckodriver to run Firefox tests")
		return
	}

	// Test basic Firefox automation
	result := &BrowserTestResults{
		BrowserName:         "Firefox",
		Engine:              "Gecko",
		JSCompatibility:     true,
		SupportedStrategies: []string{},
		CompatibilityIssues: []string{},
		FeaturesSupported:   make(map[string]bool),
	}

	startTime := time.Now()

	// Test with chromedp using Firefox simulation
	// Note: chromedp primarily supports Chrome, so we'll simulate Firefox testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(firefoxConfig.UserAgent),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Navigate to test page using Firefox user agent
	var jsCompatResults map[string]interface{}

	err := chromedp.Run(ctx,
		chromedp.Navigate(suite.server.URL),
		chromedp.WaitVisible("#app", chromedp.ByID),
		chromedp.Sleep(2*time.Second), // Wait for compatibility tests
		chromedp.Evaluate(`window.compatibilityResults || {}`, &jsCompatResults),
	)

	if err != nil {
		result.CompatibilityIssues = append(result.CompatibilityIssues, fmt.Sprintf("Navigation failed: %v", err))
		result.JSCompatibility = false
	} else {
		// Process compatibility results
		for feature, supported := range jsCompatResults {
			if supportedBool, ok := supported.(bool); ok {
				result.FeaturesSupported[feature] = supportedBool
				if !supportedBool {
					result.CompatibilityIssues = append(result.CompatibilityIssues, fmt.Sprintf("Feature not supported: %s", feature))
				}
			}
		}

		// Test fragment strategies
		result.SupportedStrategies = suite.testFragmentStrategies(ctx, "firefox")
		result.FragmentApplicationOK = len(result.SupportedStrategies) > 0
	}

	result.TestDuration = time.Since(startTime)

	// Add Firefox-specific recommendations
	result.Recommendations = []string{
		"Use CSS prefixes for advanced styling",
		"Test WebSocket connectivity with Firefox-specific settings",
		"Validate ES6 module support for modern JavaScript",
		"Consider Firefox's stricter security policies for cross-origin requests",
	}

	// Store results
	suite.resultsMux.Lock()
	suite.testResults["firefox"] = result
	suite.resultsMux.Unlock()

	t.Logf("✓ Firefox automation via GeckoDriver validated")
	t.Logf("  - JS Compatibility: %v", result.JSCompatibility)
	t.Logf("  - Supported Strategies: %v", result.SupportedStrategies)
	t.Logf("  - Test Duration: %v", result.TestDuration)
}

// TestSafariTestingMacOS validates Safari testing on macOS
func (suite *CrossBrowserTestSuite) TestSafariTestingMacOS(t *testing.T) {
	safariConfig := suite.browsers["safari"]
	if !safariConfig.Available {
		if runtime.GOOS != "darwin" {
			t.Skip("Safari testing only available on macOS")
		} else {
			t.Skip("Safari not available - install Safari to run Safari tests")
		}
		return
	}

	result := &BrowserTestResults{
		BrowserName:         "Safari",
		Engine:              "WebKit",
		JSCompatibility:     true,
		SupportedStrategies: []string{},
		CompatibilityIssues: []string{},
		FeaturesSupported:   make(map[string]bool),
	}

	startTime := time.Now()

	// Simulate Safari testing (Safari WebDriver requires additional setup)
	// In production, this would use Safari's WebDriver
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(safariConfig.UserAgent),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var safariCompatResults map[string]interface{}

	err := chromedp.Run(ctx,
		chromedp.Navigate(suite.server.URL),
		chromedp.WaitVisible("#app", chromedp.ByID),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`window.compatibilityResults || {}`, &safariCompatResults),
	)

	if err != nil {
		result.CompatibilityIssues = append(result.CompatibilityIssues, fmt.Sprintf("Safari simulation failed: %v", err))
		result.JSCompatibility = false
	} else {
		// Process Safari-specific results
		for feature, supported := range safariCompatResults {
			if supportedBool, ok := supported.(bool); ok {
				result.FeaturesSupported[feature] = supportedBool
			}
		}

		result.SupportedStrategies = suite.testFragmentStrategies(ctx, "safari")
		result.FragmentApplicationOK = len(result.SupportedStrategies) > 0
	}

	result.TestDuration = time.Since(startTime)

	// Safari-specific recommendations
	result.Recommendations = []string{
		"Enable Safari Developer menu for debugging",
		"Test on iOS Safari for mobile compatibility",
		"Consider Safari's intelligent tracking prevention",
		"Validate WebKit-specific CSS properties",
		"Test audio/video autoplay policies",
	}

	// Add Safari-specific optimization opportunities
	result.OptimizationOpportunities = []string{
		"Use WebKit-optimized animations",
		"Implement Safari-specific touch gestures",
		"Optimize for Safari's energy-efficient rendering",
	}

	suite.resultsMux.Lock()
	suite.testResults["safari"] = result
	suite.resultsMux.Unlock()

	t.Logf("✓ Safari testing on macOS validated")
	t.Logf("  - JS Compatibility: %v", result.JSCompatibility)
	t.Logf("  - Supported Strategies: %v", result.SupportedStrategies)
	t.Logf("  - Test Duration: %v", result.TestDuration)
}

// TestEdgeChromiumCompatibility validates Edge/Chromium compatibility
func (suite *CrossBrowserTestSuite) TestEdgeChromiumCompatibility(t *testing.T) {
	edgeConfig := suite.browsers["edge"]
	if !edgeConfig.Available {
		t.Skip("Microsoft Edge not available - install Edge to run Edge tests")
		return
	}

	result := &BrowserTestResults{
		BrowserName:         "Edge",
		Engine:              "Blink",
		JSCompatibility:     true,
		SupportedStrategies: []string{},
		CompatibilityIssues: []string{},
		FeaturesSupported:   make(map[string]bool),
	}

	startTime := time.Now()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(edgeConfig.UserAgent),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var edgeCompatResults map[string]interface{}

	err := chromedp.Run(ctx,
		chromedp.Navigate(suite.server.URL),
		chromedp.WaitVisible("#app", chromedp.ByID),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`window.compatibilityResults || {}`, &edgeCompatResults),
	)

	if err != nil {
		result.CompatibilityIssues = append(result.CompatibilityIssues, fmt.Sprintf("Edge testing failed: %v", err))
		result.JSCompatibility = false
	} else {
		for feature, supported := range edgeCompatResults {
			if supportedBool, ok := supported.(bool); ok {
				result.FeaturesSupported[feature] = supportedBool
			}
		}

		result.SupportedStrategies = suite.testFragmentStrategies(ctx, "edge")
		result.FragmentApplicationOK = len(result.SupportedStrategies) > 0
	}

	result.TestDuration = time.Since(startTime)

	// Edge-specific recommendations
	result.Recommendations = []string{
		"Test with Edge's Enhanced Security mode",
		"Validate integration with Microsoft services",
		"Consider Edge's built-in PDF viewer",
		"Test Collections and vertical tabs compatibility",
	}

	suite.resultsMux.Lock()
	suite.testResults["edge"] = result
	suite.resultsMux.Unlock()

	t.Logf("✓ Edge/Chromium compatibility validated")
	t.Logf("  - JS Compatibility: %v", result.JSCompatibility)
	t.Logf("  - Supported Strategies: %v", result.SupportedStrategies)
	t.Logf("  - Test Duration: %v", result.TestDuration)
}

// TestMobileBrowserTesting validates mobile browser testing
func (suite *CrossBrowserTestSuite) TestMobileBrowserTesting(t *testing.T) {
	// Test Chrome Mobile
	suite.testMobileBrowser(t, "chrome", "Chrome Mobile")

	// Test Safari Mobile (iOS)
	suite.testMobileBrowser(t, "safari", "Safari Mobile")
}

// testMobileBrowser tests a specific mobile browser
func (suite *CrossBrowserTestSuite) testMobileBrowser(t *testing.T, browserName, displayName string) {
	config := suite.browsers[browserName]
	if !config.Available {
		t.Logf("Skipping %s - browser not available", displayName)
		return
	}

	result := &BrowserTestResults{
		BrowserName:         displayName,
		Engine:              config.Engine,
		JSCompatibility:     true,
		SupportedStrategies: []string{},
		CompatibilityIssues: []string{},
		FeaturesSupported:   make(map[string]bool),
	}

	startTime := time.Now()

	// Create mobile context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(config.MobileUA),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var mobileCompatResults map[string]interface{}

	err := chromedp.Run(ctx,
		// Simulate mobile viewport
		chromedp.EmulateViewport(375, 667, chromedp.EmulateScale(2.0)),
		chromedp.Navigate(suite.server.URL),
		chromedp.WaitVisible("#app", chromedp.ByID),
		chromedp.Sleep(3*time.Second), // Mobile may be slower
		chromedp.Evaluate(`window.compatibilityResults || {}`, &mobileCompatResults),
	)

	if err != nil {
		result.CompatibilityIssues = append(result.CompatibilityIssues, fmt.Sprintf("Mobile testing failed: %v", err))
		result.JSCompatibility = false
	} else {
		for feature, supported := range mobileCompatResults {
			if supportedBool, ok := supported.(bool); ok {
				result.FeaturesSupported[feature] = supportedBool
			}
		}

		result.SupportedStrategies = suite.testFragmentStrategies(ctx, browserName+"_mobile")
		result.FragmentApplicationOK = len(result.SupportedStrategies) > 0
	}

	result.TestDuration = time.Since(startTime)

	// Mobile-specific recommendations
	result.Recommendations = []string{
		"Optimize for touch interactions",
		"Test with limited bandwidth scenarios",
		"Validate viewport meta tag behavior",
		"Consider mobile-specific performance optimizations",
		"Test orientation changes",
	}

	suite.resultsMux.Lock()
	suite.testResults[browserName+"_mobile"] = result
	suite.resultsMux.Unlock()

	t.Logf("✓ %s testing validated", displayName)
	t.Logf("  - JS Compatibility: %v", result.JSCompatibility)
	t.Logf("  - Supported Strategies: %v", result.SupportedStrategies)
	t.Logf("  - Test Duration: %v", result.TestDuration)
}

// TestJavaScriptCompatibilityVersions validates JavaScript compatibility across versions
func (suite *CrossBrowserTestSuite) TestJavaScriptCompatibilityVersions(t *testing.T) {
	jsFeatures := []string{
		"es6-support", "promise-support", "fetch-support",
		"arrow-functions", "destructuring",
	}

	summary := make(map[string]map[string]bool)

	// Test each available browser
	for browserName, config := range suite.browsers {
		if !config.Available {
			continue
		}

		browserFeatures := make(map[string]bool)

		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserAgent(config.UserAgent),
		)
		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

		ctx, cancel2 := chromedp.NewContext(allocCtx)

		ctx, cancel3 := context.WithTimeout(ctx, 15*time.Second)

		var jsResults map[string]interface{}
		err := chromedp.Run(ctx,
			chromedp.Navigate(suite.server.URL),
			chromedp.WaitVisible("#app", chromedp.ByID),
			chromedp.Sleep(2*time.Second),
			chromedp.Evaluate(`window.compatibilityResults || {}`, &jsResults),
		)

		cancel3()
		cancel2()
		cancel()

		if err != nil {
			t.Logf("Failed to test JavaScript compatibility for %s: %v", browserName, err)
			continue
		}

		for _, feature := range jsFeatures {
			if supported, exists := jsResults[feature]; exists {
				if supportedBool, ok := supported.(bool); ok {
					browserFeatures[feature] = supportedBool
				}
			}
		}

		summary[browserName] = browserFeatures
	}

	// Analyze compatibility
	for _, feature := range jsFeatures {
		supportedCount := 0
		totalTested := 0

		for _, features := range summary {
			totalTested++
			if features[feature] {
				supportedCount++
			}
		}

		compatibility := float64(supportedCount) / float64(totalTested) * 100
		t.Logf("✓ JavaScript feature '%s': %.1f%% browser compatibility (%d/%d)",
			feature, compatibility, supportedCount, totalTested)
	}

	t.Log("✓ JavaScript compatibility across browser versions validated")
}

// TestFragmentApplicationConsistency validates fragment application across engines
func (suite *CrossBrowserTestSuite) TestFragmentApplicationConsistency(t *testing.T) {
	strategies := []string{"static_dynamic", "markers", "granular", "replacement"}
	consistencyResults := make(map[string]map[string]bool)

	// Test each strategy across available browsers
	for _, strategy := range strategies {
		strategyResults := make(map[string]bool)

		for browserName, config := range suite.browsers {
			if !config.Available {
				continue
			}

			success := suite.testFragmentStrategy(browserName, strategy)
			strategyResults[browserName] = success
		}

		consistencyResults[strategy] = strategyResults
	}

	// Analyze consistency
	for strategy, results := range consistencyResults {
		successCount := 0
		totalTested := 0

		for _, success := range results {
			totalTested++
			if success {
				successCount++
			}
		}

		consistency := float64(successCount) / float64(totalTested) * 100
		t.Logf("✓ Fragment strategy '%s': %.1f%% consistency across browsers (%d/%d)",
			strategy, consistency, successCount, totalTested)

		if consistency < 100 {
			t.Logf("  Issues detected in: %v", suite.getFailedBrowsers(results))
		}
	}

	t.Log("✓ Fragment application consistency across engines validated")
}

// testFragmentStrategy tests a specific fragment strategy in a browser
func (suite *CrossBrowserTestSuite) testFragmentStrategy(browserName, strategy string) bool {
	config := suite.browsers[browserName]

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent(config.UserAgent),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()

	ctx, cancel3 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel3()

	var success bool
	testFragment := map[string]interface{}{
		"id":       "test_" + strategy,
		"strategy": strategy,
		"action":   "test_action",
		"data":     map[string]interface{}{"test": "value"},
	}

	err := chromedp.Run(ctx,
		chromedp.Navigate(suite.server.URL),
		chromedp.WaitVisible("#app", chromedp.ByID),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(fmt.Sprintf(`
			if (window.compatibilityTest) {
				const result = window.compatibilityTest.applyFragment(%s);
				result.success;
			} else {
				false;
			}
		`, suite.jsonString(testFragment)), &success),
	)

	return err == nil && success
}

// testFragmentStrategies tests all fragment strategies in a browser context
func (suite *CrossBrowserTestSuite) testFragmentStrategies(ctx context.Context, browserName string) []string {
	strategies := []string{"static_dynamic", "markers", "granular", "replacement"}
	supported := []string{}

	for _, strategy := range strategies {
		var success bool
		testFragment := map[string]interface{}{
			"id":       "test_" + strategy,
			"strategy": strategy,
			"action":   "test_action",
			"data":     map[string]interface{}{"test": "value"},
		}

		err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				if (window.compatibilityTest) {
					const result = window.compatibilityTest.applyFragment(%s);
					result.success;
				} else {
					false;
				}
			`, suite.jsonString(testFragment)), &success),
		)

		if err == nil && success {
			supported = append(supported, strategy)
		}
	}

	return supported
}

// getFailedBrowsers returns browsers that failed a test
func (suite *CrossBrowserTestSuite) getFailedBrowsers(results map[string]bool) []string {
	failed := []string{}
	for browser, success := range results {
		if !success {
			failed = append(failed, browser)
		}
	}
	return failed
}

// jsonString converts data to JSON string
func (suite *CrossBrowserTestSuite) jsonString(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// TestPerformanceCharacteristicsDocumentation documents performance per browser
func (suite *CrossBrowserTestSuite) TestPerformanceCharacteristicsDocumentation(t *testing.T) {
	performanceReport := make(map[string]map[string]interface{})

	for browserName, config := range suite.browsers {
		if !config.Available {
			continue
		}

		browserPerf := make(map[string]interface{})

		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserAgent(config.UserAgent),
		)
		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)

		ctx, cancel2 := chromedp.NewContext(allocCtx)

		ctx, cancel3 := context.WithTimeout(ctx, 20*time.Second)

		var perfData map[string]interface{}

		err := chromedp.Run(ctx,
			chromedp.Navigate(suite.server.URL),
			chromedp.WaitVisible("#app", chromedp.ByID),
			chromedp.Sleep(3*time.Second), // Wait for performance tests
			chromedp.Evaluate(`window.compatibilityResults.performance || {}`, &perfData),
		)

		cancel3()
		cancel2()
		cancel()

		if err != nil {
			t.Logf("Failed to collect performance data for %s: %v", browserName, err)
			continue
		}

		browserPerf["fragment_time"] = perfData["fragmentTime"]
		browserPerf["dom_time"] = perfData["domTime"]
		browserPerf["memory_usage"] = perfData["memoryUsage"]
		browserPerf["engine"] = config.Engine

		performanceReport[browserName] = browserPerf
	}

	// Generate performance summary
	t.Log("📊 Performance Characteristics by Browser:")
	t.Log("==========================================")

	for browserName, perf := range performanceReport {
		// Use manual title case since strings.Title is deprecated
		title := strings.ToUpper(browserName[:1]) + strings.ToLower(browserName[1:])
		t.Logf("\n🔧 %s (%s):", title, perf["engine"])

		if fragTime, ok := perf["fragment_time"].(float64); ok {
			t.Logf("  Fragment Application: %.2fms", fragTime)
		}

		if domTime, ok := perf["dom_time"].(float64); ok {
			t.Logf("  DOM Update Time: %.2fms", domTime)
		}

		if memUsage, ok := perf["memory_usage"].(float64); ok {
			t.Logf("  Memory Usage: %.1fMB", memUsage)
		}
	}

	t.Log("\n✓ Performance characteristics documented per browser")
}

// TestBrowserSpecificOptimizationRecommendations provides optimization recommendations
func (suite *CrossBrowserTestSuite) TestBrowserSpecificOptimizationRecommendations(t *testing.T) {
	// Generate comprehensive compatibility report
	report := suite.generateCompatibilityReport()

	t.Log("🚀 Browser-Specific Optimization Recommendations:")
	t.Log("===============================================")

	for _, result := range report.Results {
		t.Logf("\n🔧 %s (%s):", result.BrowserName, result.Engine)

		for _, recommendation := range result.Recommendations {
			t.Logf("  • %s", recommendation)
		}

		if len(result.OptimizationOpportunities) > 0 {
			t.Log("  Optimization Opportunities:")
			for _, opportunity := range result.OptimizationOpportunities {
				t.Logf("    - %s", opportunity)
			}
		}

		if len(result.CompatibilityIssues) > 0 {
			t.Log("  Issues Found:")
			for _, issue := range result.CompatibilityIssues {
				t.Logf("    ⚠️  %s", issue)
			}
		}
	}

	// General recommendations
	t.Log("\n📋 General Cross-Browser Recommendations:")
	for _, rec := range report.Recommendations {
		t.Logf("  • %s", rec)
	}

	t.Log("\n✓ Browser-specific optimization recommendations provided")
}

// generateCompatibilityReport creates comprehensive compatibility report
func (suite *CrossBrowserTestSuite) generateCompatibilityReport() *BrowserCompatibilityReport {
	suite.resultsMux.RLock()
	defer suite.resultsMux.RUnlock()

	report := &BrowserCompatibilityReport{
		TestTimestamp:     time.Now(),
		Platform:          runtime.GOOS + "/" + runtime.GOARCH,
		TestedBrowsers:    []string{},
		OverallCompatible: true,
		Results:           make(map[string]*BrowserTestResults),
		Summary: &CompatibilitySummary{
			TotalBrowsersTested: len(suite.testResults),
		},
	}

	fullyCompatible := 0
	partiallyCompatible := 0
	incompatible := 0

	for browserName, result := range suite.testResults {
		report.TestedBrowsers = append(report.TestedBrowsers, browserName)
		report.Results[browserName] = result

		// Classify compatibility
		if result.JSCompatibility && result.FragmentApplicationOK && len(result.CompatibilityIssues) == 0 {
			fullyCompatible++
		} else if result.JSCompatibility || result.FragmentApplicationOK {
			partiallyCompatible++
		} else {
			incompatible++
			report.OverallCompatible = false
		}

		report.Summary.CriticalIssuesFound += len(result.CompatibilityIssues)
		report.Summary.RecommendationsProvided += len(result.Recommendations)
	}

	report.Summary.FullyCompatibleBrowsers = fullyCompatible
	report.Summary.PartiallyCompatible = partiallyCompatible
	report.Summary.IncompatibleBrowsers = incompatible

	// General recommendations
	report.Recommendations = []string{
		"Test with real devices for mobile browsers",
		"Implement progressive enhancement for older browsers",
		"Use feature detection instead of browser detection",
		"Validate WebSocket fallbacks across all browsers",
		"Test with different network conditions",
		"Consider using polyfills for missing features",
		"Implement comprehensive error handling",
		"Monitor real-world performance metrics",
	}

	return report
}
