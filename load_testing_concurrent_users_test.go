package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLoadTestingConcurrentUsers implements comprehensive load testing capabilities
func TestLoadTestingConcurrentUsers(t *testing.T) {
	suite := &LoadTestSuite{
		t: t,
	}

	// Run all load testing scenarios
	t.Run("Multiple_Browser_Instance_Management", suite.TestMultipleBrowserInstances)
	t.Run("User_Session_Simulation", suite.TestUserSessionSimulation)
	t.Run("Fragment_Generation_Performance", suite.TestFragmentGenerationPerformance)
	t.Run("Memory_Usage_Scaling", suite.TestMemoryUsageScaling)
	t.Run("State_Management_Stress", suite.TestStateManagementStress)
	t.Run("Performance_Degradation_Thresholds", suite.TestPerformanceDegradationThresholds)
	t.Run("Horizontal_Scaling_Behavior", suite.TestHorizontalScalingBehavior)
	t.Run("Resource_Bottleneck_Identification", suite.TestResourceBottleneckIdentification)
}

// LoadTestSuite provides comprehensive load testing capabilities
type LoadTestSuite struct {
	t *testing.T
}

// LoadTestMetrics captures comprehensive load testing metrics
type LoadTestMetrics struct {
	// Overall test metrics
	TotalUsers   int           `json:"total_users"`
	TestDuration time.Duration `json:"test_duration"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`

	// Performance metrics
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	P50ResponseTime float64 `json:"p50_response_time_ms"`
	P95ResponseTime float64 `json:"p95_response_time_ms"`
	P99ResponseTime float64 `json:"p99_response_time_ms"`
	MaxResponseTime float64 `json:"max_response_time_ms"`
	MinResponseTime float64 `json:"min_response_time_ms"`

	// Throughput metrics
	RequestsPerSecond  float64 `json:"requests_per_second"`
	FragmentsPerSecond float64 `json:"fragments_per_second"`
	TotalRequests      int64   `json:"total_requests"`
	TotalFragments     int64   `json:"total_fragments"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`

	// Memory metrics
	InitialMemoryMB float64 `json:"initial_memory_mb"`
	PeakMemoryMB    float64 `json:"peak_memory_mb"`
	FinalMemoryMB   float64 `json:"final_memory_mb"`
	MemoryGrowthMB  float64 `json:"memory_growth_mb"`
	MemoryPerUser   float64 `json:"memory_per_user_mb"`

	// Resource utilization
	CPUUtilization float64 `json:"cpu_utilization_percent"`
	GoroutineCount int     `json:"goroutine_count"`
	HeapObjects    uint64  `json:"heap_objects"`
	GCCycles       uint32  `json:"gc_cycles"`

	// Error metrics
	ErrorRate         float64 `json:"error_rate_percent"`
	TimeoutErrors     int64   `json:"timeout_errors"`
	ConnectionErrors  int64   `json:"connection_errors"`
	ApplicationErrors int64   `json:"application_errors"`

	// Scaling metrics
	LinearScaling     bool    `json:"linear_scaling"`
	ScalingEfficiency float64 `json:"scaling_efficiency"`
	BottleneckType    string  `json:"bottleneck_type"`

	// Detailed breakdowns
	UserMetrics []UserMetrics `json:"user_metrics"`
}

// UserMetrics captures individual user session metrics
type UserMetrics struct {
	UserID             int           `json:"user_id"`
	SessionDuration    time.Duration `json:"session_duration"`
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	AvgResponseTime    float64       `json:"avg_response_time_ms"`
	FragmentsGenerated int           `json:"fragments_generated"`
	MemoryUsage        float64       `json:"memory_usage_mb"`
	ErrorCount         int           `json:"error_count"`
	LastActivity       time.Time     `json:"last_activity"`
}

// BrowserSession represents a simulated browser session
type BrowserSession struct {
	ID              string
	Application     *Application
	Page            *ApplicationPage
	HttpClient      *http.Client
	Server          *httptest.Server
	SessionData     map[string]interface{}
	RequestCount    int64
	LastRequestTime time.Time
	TotalLatency    time.Duration
	ErrorCount      int64
	mu              sync.RWMutex
}

// TestMultipleBrowserInstances tests concurrent browser instance management
func (suite *LoadTestSuite) TestMultipleBrowserInstances(t *testing.T) {
	t.Run("Concurrent_Browser_Sessions", func(t *testing.T) {
		const numSessions = 25
		sessions := make([]*BrowserSession, numSessions)

		// Create multiple browser sessions concurrently
		var wg sync.WaitGroup
		for i := 0; i < numSessions; i++ {
			wg.Add(1)
			go func(sessionID int) {
				defer wg.Done()
				session, err := suite.createBrowserSession(sessionID)
				if err != nil {
					t.Errorf("Failed to create browser session %d: %v", sessionID, err)
					return
				}
				sessions[sessionID] = session
			}(i)
		}
		wg.Wait()

		// Verify all sessions are independent and functional
		activeCount := 0
		for i, session := range sessions {
			if session != nil {
				activeCount++

				// Test session independence
				err := suite.testSessionIndependence(session, i)
				if err != nil {
					t.Errorf("Session %d failed independence test: %v", i, err)
				}
			}
		}

		// Cleanup sessions
		for _, session := range sessions {
			if session != nil {
				suite.cleanupBrowserSession(session)
			}
		}

		t.Logf("✓ Multiple browser instances: %d/%d sessions successful", activeCount, numSessions)
	})

	t.Run("Session_Isolation_Validation", func(t *testing.T) {
		const numSessions = 10
		sessions := make([]*BrowserSession, numSessions)

		// Create sessions with different data
		for i := 0; i < numSessions; i++ {
			session, err := suite.createBrowserSession(i)
			if err != nil {
				t.Fatalf("Failed to create session %d: %v", i, err)
			}
			sessions[i] = session

			// Update each session with unique data
			uniqueData := map[string]interface{}{
				"SessionID": i,
				"UserName":  fmt.Sprintf("User_%d", i),
				"Counter":   i * 100,
				"Timestamp": time.Now().Unix(),
			}

			err = suite.updateSessionData(session, uniqueData)
			if err != nil {
				t.Errorf("Failed to update session %d: %v", i, err)
			}
		}

		// Verify data isolation
		for i, session := range sessions {
			html, err := suite.getSessionHTML(session)
			if err != nil {
				t.Errorf("Failed to get HTML for session %d: %v", i, err)
				continue
			}

			// Check for data leakage between sessions
			for j, otherSession := range sessions {
				if i != j && otherSession != nil {
					otherHTML, err := suite.getSessionHTML(otherSession)
					if err == nil && html == otherHTML && i != j {
						t.Errorf("Data leakage detected between sessions %d and %d", i, j)
					}
				}
			}
		}

		// Cleanup
		for _, session := range sessions {
			if session != nil {
				suite.cleanupBrowserSession(session)
			}
		}

		t.Log("✓ Session isolation validated successfully")
	})
}

// TestUserSessionSimulation tests realistic user session behavior
func (suite *LoadTestSuite) TestUserSessionSimulation(t *testing.T) {
	t.Run("Realistic_User_Behavior", func(t *testing.T) {
		const numUsers = 15
		const sessionDuration = 10 * time.Second
		const maxRequestsPerUser = 20

		var wg sync.WaitGroup
		userMetrics := make([]UserMetrics, numUsers)

		for userID := 0; userID < numUsers; userID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				metrics := suite.simulateUserSession(id, sessionDuration, maxRequestsPerUser)
				userMetrics[id] = metrics
			}(userID)
		}

		wg.Wait()

		// Analyze user session results
		totalRequests := 0
		totalErrors := 0
		totalLatency := time.Duration(0)

		for _, metrics := range userMetrics {
			totalRequests += metrics.TotalRequests
			totalErrors += metrics.ErrorCount
			totalLatency += time.Duration(metrics.AvgResponseTime * float64(time.Millisecond))
		}

		avgLatency := float64(totalLatency.Nanoseconds()) / float64(totalRequests) / 1000000 // Convert to ms
		errorRate := float64(totalErrors) / float64(totalRequests) * 100

		t.Logf("✓ User session simulation completed:")
		t.Logf("  - %d users simulated", numUsers)
		t.Logf("  - %d total requests", totalRequests)
		t.Logf("  - %.2f ms average latency", avgLatency)
		t.Logf("  - %.2f%% error rate", errorRate)

		if errorRate > 5.0 {
			t.Errorf("Error rate too high: %.2f%% (threshold: 5%%)", errorRate)
		}
	})

	t.Run("Independent_Data_States", func(t *testing.T) {
		const numUsers = 12

		// Create individual applications for each user to ensure complete isolation
		apps := make([]*Application, numUsers)
		pages := make([]*ApplicationPage, numUsers)
		dataStates := make([]map[string]interface{}, numUsers)

		// Create template for user data
		tmpl := template.Must(template.New("user-data").Parse(`
		<div id="user-{{.UserID}}">
			<h2>{{.UserName}}</h2>
			<p>Role: {{.UserRole}}</p>
			<p>Counter: {{.Counter}}</p>
			<div>Activities: {{range .Activities}}{{.}}, {{end}}</div>
		</div>
		`))

		// Create sessions with independent data states
		for i := 0; i < numUsers; i++ {
			app, err := NewApplication()
			if err != nil {
				t.Fatalf("Failed to create application for user %d: %v", i, err)
			}
			apps[i] = app

			// Create unique data state for each user
			dataStates[i] = map[string]interface{}{
				"UserID":     i,
				"UserName":   fmt.Sprintf("TestUser_%d", i),
				"UserRole":   []string{"user", "admin", "guest"}[i%3],
				"Settings":   map[string]interface{}{"theme": []string{"light", "dark"}[i%2]},
				"Activities": make([]string, 0),
				"Counter":    i * 10,
			}

			page, err := app.NewApplicationPage(tmpl, dataStates[i])
			if err != nil {
				t.Fatalf("Failed to create page for user %d: %v", i, err)
			}
			pages[i] = page
		}

		// Simulate concurrent user activities
		var wg sync.WaitGroup
		for i := 0; i < numUsers; i++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()

				// Perform user-specific activities
				for activity := 0; activity < 5; activity++ {
					// Update user data
					dataStates[userID]["Counter"] = dataStates[userID]["Counter"].(int) + 1
					dataStates[userID]["Activities"] = append(
						dataStates[userID]["Activities"].([]string),
						fmt.Sprintf("Activity_%d_%d", userID, activity),
					)

					_, err := pages[userID].RenderFragments(context.Background(), dataStates[userID])
					if err != nil {
						t.Errorf("User %d activity %d failed: %v", userID, activity, err)
					}

					time.Sleep(50 * time.Millisecond) // Simulate user thinking time
				}
			}(i)
		}
		wg.Wait()

		// Verify data state independence
		for i := 0; i < numUsers; i++ {
			html, err := pages[i].Render()
			if err != nil {
				t.Errorf("Failed to get final HTML for user %d: %v", i, err)
				continue
			}

			// Verify user-specific data is present
			expectedUserID := fmt.Sprintf("TestUser_%d", i)
			expectedDivID := fmt.Sprintf("user-%d", i)

			if !hasSubstring(html, expectedUserID) {
				t.Logf("User %d data not found in HTML: %s", i, html)
				t.Errorf("User %d data not found in HTML", i)
			}
			if !hasSubstring(html, expectedDivID) {
				t.Errorf("User %d div ID not found in HTML", i)
			}

			// Verify no cross-contamination by checking for other user IDs
			for j := 0; j < numUsers; j++ {
				if i != j {
					otherUserID := fmt.Sprintf("TestUser_%d", j)
					otherDivID := fmt.Sprintf("user-%d", j)
					// Only report cross-contamination if we find the exact other user ID or div ID
					// This avoids false positives from substring matches like "TestUser_1" in "TestUser_10"
					if (hasSubstring(html, otherUserID) && !strings.Contains(expectedUserID, otherUserID)) ||
						(hasSubstring(html, otherDivID) && !strings.Contains(expectedDivID, otherDivID)) {
						t.Errorf("Cross-contamination detected: User %d HTML contains User %d data (%s or %s)", i, j, otherUserID, otherDivID)
					}
				}
			}
		}

		// Cleanup
		for i := 0; i < numUsers; i++ {
			if pages[i] != nil {
				if err := pages[i].Close(); err != nil {
					t.Logf("Warning: Failed to close page %d: %v", i, err)
				}
			}
			if apps[i] != nil {
				if err := apps[i].Close(); err != nil {
					t.Logf("Warning: Failed to close app %d: %v", i, err)
				}
			}
		}

		t.Log("✓ Independent data states validated successfully")
	})
}

// TestFragmentGenerationPerformance tests performance under concurrent load
func (suite *LoadTestSuite) TestFragmentGenerationPerformance(t *testing.T) {
	t.Run("High_Concurrency_Fragment_Generation", func(t *testing.T) {
		const numWorkers = 20
		const requestsPerWorker = 50
		const totalRequests = numWorkers * requestsPerWorker

		// Setup shared application and page
		app, err := NewApplication(WithMaxMemoryMB(100))
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("load-test").Parse(`
		<div id="header">{{.Title}}</div>
		<div id="content">{{.Message}}</div>
		<div id="counter">{{.Counter}}</div>
		<ul id="items">
		{{range .Items}}
		<li>{{.}}</li>
		{{end}}
		</ul>
		`))

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"Title":   "Load Test",
			"Message": "Initial",
			"Counter": 0,
			"Items":   []string{},
		})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Metrics collection
		var totalLatency int64 // in nanoseconds
		var successCount int64
		var errorCount int64
		var fragmentCount int64

		startTime := time.Now()

		// Launch concurrent workers
		var wg sync.WaitGroup
		for worker := 0; worker < numWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for req := 0; req < requestsPerWorker; req++ {
					// Generate unique request data
					requestData := map[string]interface{}{
						"Title":   fmt.Sprintf("Load Test - Worker %d", workerID),
						"Message": fmt.Sprintf("Request %d from Worker %d", req, workerID),
						"Counter": workerID*requestsPerWorker + req,
						"Items":   []string{fmt.Sprintf("Item_%d_%d", workerID, req)},
					}

					// Measure fragment generation latency
					reqStart := time.Now()
					fragments, err := page.RenderFragments(context.Background(), requestData)
					reqDuration := time.Since(reqStart)

					atomic.AddInt64(&totalLatency, reqDuration.Nanoseconds())

					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						t.Logf("Fragment generation failed for worker %d request %d: %v", workerID, req, err)
					} else {
						atomic.AddInt64(&successCount, 1)
						atomic.AddInt64(&fragmentCount, int64(len(fragments)))
					}
				}
			}(worker)
		}

		wg.Wait()
		endTime := time.Now()
		totalDuration := endTime.Sub(startTime)

		// Calculate metrics
		avgLatencyMs := float64(totalLatency) / float64(successCount) / 1000000
		requestsPerSecond := float64(totalRequests) / totalDuration.Seconds()
		fragmentsPerSecond := float64(fragmentCount) / totalDuration.Seconds()
		errorRate := float64(errorCount) / float64(totalRequests) * 100

		t.Logf("✓ High concurrency fragment generation results:")
		t.Logf("  - %d total requests (%d workers × %d requests)", totalRequests, numWorkers, requestsPerWorker)
		t.Logf("  - %d successful, %d failed (%.2f%% error rate)", successCount, errorCount, errorRate)
		t.Logf("  - %.2f ms average latency", avgLatencyMs)
		t.Logf("  - %.2f requests/second", requestsPerSecond)
		t.Logf("  - %.2f fragments/second", fragmentsPerSecond)
		t.Logf("  - %d total fragments generated", fragmentCount)

		// Performance assertions
		if errorRate > 1.0 {
			t.Errorf("Error rate too high: %.2f%% (threshold: 1%%)", errorRate)
		}
		if avgLatencyMs > 100.0 {
			t.Errorf("Average latency too high: %.2f ms (threshold: 100ms)", avgLatencyMs)
		}
		if requestsPerSecond < 100.0 {
			t.Errorf("Throughput too low: %.2f RPS (minimum: 100 RPS)", requestsPerSecond)
		}
	})

	t.Run("Sustained_Load_Testing", func(t *testing.T) {
		const numUsers = 10
		const testDuration = 5 * time.Second
		const requestInterval = 100 * time.Millisecond

		app, err := NewApplication(WithMaxMemoryMB(50))
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("sustained").Parse(`<div>{{.Counter}}: {{.Message}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"Counter": 0,
			"Message": "Sustained Load Test",
		})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		var requestCount int64
		var totalLatency int64
		var errorCount int64

		// Start sustained load
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()

		for user := 0; user < numUsers; user++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()

				counter := 0
				ticker := time.NewTicker(requestInterval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						counter++
						data := map[string]interface{}{
							"Counter": userID*1000 + counter,
							"Message": fmt.Sprintf("User %d - Message %d", userID, counter),
						}

						start := time.Now()
						_, err := page.RenderFragments(context.Background(), data)
						latency := time.Since(start)

						atomic.AddInt64(&requestCount, 1)
						atomic.AddInt64(&totalLatency, latency.Nanoseconds())

						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						}
					}
				}
			}(user)
		}

		wg.Wait()

		// Calculate sustained load metrics
		avgLatencyMs := float64(totalLatency) / float64(requestCount) / 1000000
		requestsPerSecond := float64(requestCount) / testDuration.Seconds()
		errorRate := float64(errorCount) / float64(requestCount) * 100

		t.Logf("✓ Sustained load testing results:")
		t.Logf("  - %d users for %v", numUsers, testDuration)
		t.Logf("  - %d total requests", requestCount)
		t.Logf("  - %.2f requests/second", requestsPerSecond)
		t.Logf("  - %.2f ms average latency", avgLatencyMs)
		t.Logf("  - %.2f%% error rate", errorRate)

		if errorRate > 0.5 {
			t.Errorf("Sustained load error rate too high: %.2f%%", errorRate)
		}
	})
}

// TestMemoryUsageScaling validates memory usage with increased users
func (suite *LoadTestSuite) TestMemoryUsageScaling(t *testing.T) {
	userCounts := []int{5, 10, 20, 50}
	memoryUsage := make([]float64, len(userCounts))

	for i, numUsers := range userCounts {
		t.Run(fmt.Sprintf("Memory_Scaling_%d_Users", numUsers), func(t *testing.T) {
			// Force GC and get baseline
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)
			baseline := float64(m1.Alloc) / 1024 / 1024 // Convert to MB

			// Create applications for users
			apps := make([]*Application, numUsers)
			pages := make([]*ApplicationPage, numUsers)

			for user := 0; user < numUsers; user++ {
				app, err := NewApplication(WithMaxMemoryMB(10))
				if err != nil {
					t.Fatalf("Failed to create application for user %d: %v", user, err)
				}
				apps[user] = app

				tmpl := template.Must(template.New("mem-test").Parse(`
				<div>User {{.UserID}}: {{.Data}}</div>
				<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>
				`))

				// Create data that will consume memory
				initialData := map[string]interface{}{
					"UserID": user,
					"Data":   fmt.Sprintf("Data for user %d with some content", user),
					"Items":  make([]string, 100), // Create some memory usage
				}

				// Fill items with data
				items := initialData["Items"].([]string)
				for j := range items {
					items[j] = fmt.Sprintf("Item_%d_%d_with_content", user, j)
				}

				page, err := app.NewApplicationPage(tmpl, initialData)
				if err != nil {
					t.Fatalf("Failed to create page for user %d: %v", user, err)
				}
				pages[user] = page

				// Generate some fragments to exercise memory
				updateData := map[string]interface{}{
					"UserID": user,
					"Data":   fmt.Sprintf("Updated data for user %d", user),
					"Items":  items,
				}

				_, err = page.RenderFragments(context.Background(), updateData)
				if err != nil {
					t.Logf("Fragment generation failed for user %d: %v", user, err)
				}
			}

			// Force GC and measure memory usage
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)
			currentMemory := float64(m2.Alloc) / 1024 / 1024 // Convert to MB

			memoryGrowth := currentMemory - baseline
			memoryPerUser := memoryGrowth / float64(numUsers)
			memoryUsage[i] = memoryPerUser

			t.Logf("✓ Memory scaling for %d users:", numUsers)
			t.Logf("  - Baseline memory: %.2f MB", baseline)
			t.Logf("  - Current memory: %.2f MB", currentMemory)
			t.Logf("  - Memory growth: %.2f MB", memoryGrowth)
			t.Logf("  - Memory per user: %.2f MB", memoryPerUser)

			// Cleanup
			for user := 0; user < numUsers; user++ {
				if pages[user] != nil {
					if err := pages[user].Close(); err != nil {
						t.Logf("Warning: Failed to close page %d: %v", user, err)
					}
				}
				if apps[user] != nil {
					if err := apps[user].Close(); err != nil {
						t.Logf("Warning: Failed to close app %d: %v", user, err)
					}
				}
			}

			// Check memory usage per user is reasonable
			if memoryPerUser > 5.0 {
				t.Errorf("Memory per user too high: %.2f MB (threshold: 5MB)", memoryPerUser)
			}
		})
	}

	// Analyze scaling pattern
	t.Run("Memory_Scaling_Analysis", func(t *testing.T) {
		t.Log("✓ Memory scaling analysis:")
		linearScaling := true
		expectedGrowth := memoryUsage[0] // Expected linear growth based on first measurement

		for i, usage := range memoryUsage {
			expectedUsage := expectedGrowth
			deviation := abs(usage - expectedUsage)
			deviationPercent := deviation / expectedUsage * 100

			t.Logf("  - %d users: %.2f MB/user (expected: %.2f, deviation: %.1f%%)",
				userCounts[i], usage, expectedUsage, deviationPercent)

			if deviationPercent > 50.0 { // Allow 50% deviation for non-linear scaling
				linearScaling = false
			}
		}

		if linearScaling {
			t.Log("✓ Memory usage scales linearly with user count")
		} else {
			t.Log("⚠ Memory usage shows non-linear scaling pattern")
		}
	})
}

// TestStateManagementStress tests state management under stress
func (suite *LoadTestSuite) TestStateManagementStress(t *testing.T) {
	t.Run("Concurrent_State_Updates", func(t *testing.T) {
		const numUsers = 15
		const updatesPerUser = 30

		app, err := NewApplication(WithMaxPages(100))
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		// Create shared state structure
		pages := make([]*ApplicationPage, numUsers)
		tmpl := template.Must(template.New("state-stress").Parse(`
		<div id="user-{{.UserID}}">
			<h3>{{.UserName}}</h3>
			<p>Counter: {{.Counter}}</p>
			<p>Status: {{.Status}}</p>
			<ul>{{range .History}}<li>{{.}}</li>{{end}}</ul>
		</div>
		`))

		// Initialize user pages
		for i := 0; i < numUsers; i++ {
			initialData := map[string]interface{}{
				"UserID":   i,
				"UserName": fmt.Sprintf("StressUser_%d", i),
				"Counter":  0,
				"Status":   "initialized",
				"History":  []string{},
			}

			page, err := app.NewApplicationPage(tmpl, initialData)
			if err != nil {
				t.Fatalf("Failed to create page for user %d: %v", i, err)
			}
			pages[i] = page
		}

		// Perform concurrent state updates
		var wg sync.WaitGroup
		var totalUpdates int64
		var successfulUpdates int64
		var updateErrors int64

		startTime := time.Now()

		for userID := 0; userID < numUsers; userID++ {
			wg.Add(1)
			go func(uid int) {
				defer wg.Done()

				history := make([]string, 0, updatesPerUser)

				for update := 0; update < updatesPerUser; update++ {
					atomic.AddInt64(&totalUpdates, 1)

					// Create update data
					history = append(history, fmt.Sprintf("Update_%d_%d", uid, update))
					updateData := map[string]interface{}{
						"UserID":   uid,
						"UserName": fmt.Sprintf("StressUser_%d", uid),
						"Counter":  update + 1,
						"Status":   []string{"processing", "completed", "pending"}[update%3],
						"History":  history,
					}

					_, err := pages[uid].RenderFragments(context.Background(), updateData)
					if err != nil {
						atomic.AddInt64(&updateErrors, 1)
						t.Logf("Update failed for user %d update %d: %v", uid, update, err)
					} else {
						atomic.AddInt64(&successfulUpdates, 1)
					}

					// Small delay to simulate realistic update patterns
					time.Sleep(10 * time.Millisecond)
				}
			}(userID)
		}

		wg.Wait()
		duration := time.Since(startTime)

		// Cleanup
		for _, page := range pages {
			if page != nil {
				if err := page.Close(); err != nil {
					t.Logf("Warning: Failed to close page: %v", err)
				}
			}
		}

		// Calculate metrics
		updatesPerSecond := float64(totalUpdates) / duration.Seconds()
		errorRate := float64(updateErrors) / float64(totalUpdates) * 100

		t.Logf("✓ Concurrent state updates completed:")
		t.Logf("  - %d total updates (%d users × %d updates)", totalUpdates, numUsers, updatesPerUser)
		t.Logf("  - %d successful, %d failed", successfulUpdates, updateErrors)
		t.Logf("  - %.2f updates/second", updatesPerSecond)
		t.Logf("  - %.2f%% error rate", errorRate)
		t.Logf("  - Test duration: %v", duration)

		if errorRate > 2.0 {
			t.Errorf("State update error rate too high: %.2f%%", errorRate)
		}
	})

	t.Run("State_Consistency_Under_Load", func(t *testing.T) {
		const numWriters = 8
		const numReaders = 12
		const testDuration = 3 * time.Second

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("consistency").Parse(`
		<div id="shared-state">
			<p>Counter: {{.Counter}}</p>
			<p>LastWriter: {{.LastWriter}}</p>
			<p>Timestamp: {{.Timestamp}}</p>
		</div>
		`))

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"Counter":    0,
			"LastWriter": "none",
			"Timestamp":  time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("Failed to create shared page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		var writeOperations int64
		var readOperations int64
		var writeErrors int64
		var readErrors int64

		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()

		var wg sync.WaitGroup

		// Start writers
		for writerID := 0; writerID < numWriters; writerID++ {
			wg.Add(1)
			go func(wid int) {
				defer wg.Done()

				counter := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
						counter++
						atomic.AddInt64(&writeOperations, 1)

						data := map[string]interface{}{
							"Counter":    wid*1000 + counter,
							"LastWriter": fmt.Sprintf("Writer_%d", wid),
							"Timestamp":  time.Now().Unix(),
						}

						_, err := page.RenderFragments(context.Background(), data)
						if err != nil {
							atomic.AddInt64(&writeErrors, 1)
						}

						time.Sleep(50 * time.Millisecond)
					}
				}
			}(writerID)
		}

		// Start readers
		for readerID := 0; readerID < numReaders; readerID++ {
			wg.Add(1)
			go func(rid int) {
				defer wg.Done()

				for {
					select {
					case <-ctx.Done():
						return
					default:
						atomic.AddInt64(&readOperations, 1)

						_, err := page.Render()
						if err != nil {
							atomic.AddInt64(&readErrors, 1)
						}

						time.Sleep(25 * time.Millisecond)
					}
				}
			}(readerID)
		}

		wg.Wait()

		writeErrorRate := float64(writeErrors) / float64(writeOperations) * 100
		readErrorRate := float64(readErrors) / float64(readOperations) * 100

		t.Logf("✓ State consistency under load:")
		t.Logf("  - %d write operations, %.2f%% error rate", writeOperations, writeErrorRate)
		t.Logf("  - %d read operations, %.2f%% error rate", readOperations, readErrorRate)
		t.Logf("  - %d writers, %d readers for %v", numWriters, numReaders, testDuration)

		if writeErrorRate > 1.0 {
			t.Errorf("Write error rate too high: %.2f%%", writeErrorRate)
		}
		if readErrorRate > 0.1 {
			t.Errorf("Read error rate too high: %.2f%%", readErrorRate)
		}
	})
}

// TestPerformanceDegradationThresholds identifies performance limits
func (suite *LoadTestSuite) TestPerformanceDegradationThresholds(t *testing.T) {
	userCounts := []int{10, 25, 50, 100, 150}
	performanceMetrics := make([]LoadTestMetrics, len(userCounts))

	for i, numUsers := range userCounts {
		t.Run(fmt.Sprintf("Performance_Threshold_%d_Users", numUsers), func(t *testing.T) {
			metrics := suite.measurePerformanceAtScale(numUsers, 2*time.Second)
			performanceMetrics[i] = metrics

			t.Logf("✓ Performance at %d users:", numUsers)
			t.Logf("  - Avg response time: %.2f ms", metrics.AvgResponseTime)
			t.Logf("  - P95 response time: %.2f ms", metrics.P95ResponseTime)
			t.Logf("  - Requests/sec: %.2f", metrics.RequestsPerSecond)
			t.Logf("  - Error rate: %.2f%%", metrics.ErrorRate)
			t.Logf("  - Memory per user: %.2f MB", metrics.MemoryPerUser)
		})
	}

	// Analyze degradation patterns
	t.Run("Degradation_Analysis", func(t *testing.T) {
		t.Log("✓ Performance degradation analysis:")

		baselineRPS := performanceMetrics[0].RequestsPerSecond
		baselineLatency := performanceMetrics[0].AvgResponseTime

		for i, metrics := range performanceMetrics {
			rpsRatio := metrics.RequestsPerSecond / baselineRPS
			latencyRatio := metrics.AvgResponseTime / baselineLatency

			t.Logf("  - %d users: RPS ratio %.2f, Latency ratio %.2f",
				userCounts[i], rpsRatio, latencyRatio)

			// Identify degradation threshold
			if metrics.AvgResponseTime > 200.0 || metrics.ErrorRate > 5.0 {
				t.Logf("  ⚠ Performance degradation detected at %d users", userCounts[i])
			}
		}
	})
}

// TestHorizontalScalingBehavior documents scaling behavior
func (suite *LoadTestSuite) TestHorizontalScalingBehavior(t *testing.T) {
	t.Run("Application_Instance_Scaling", func(t *testing.T) {
		instanceCounts := []int{1, 2, 4}
		usersPerInstance := 25

		for _, numInstances := range instanceCounts {
			totalUsers := numInstances * usersPerInstance

			t.Run(fmt.Sprintf("%d_Instances", numInstances), func(t *testing.T) {
				apps := make([]*Application, numInstances)

				// Create multiple application instances
				for i := 0; i < numInstances; i++ {
					app, err := NewApplication(WithMaxMemoryMB(50))
					if err != nil {
						t.Fatalf("Failed to create application instance %d: %v", i, err)
					}
					apps[i] = app
				}

				// Distribute users across instances
				var wg sync.WaitGroup
				startTime := time.Now()
				var totalRequests int64
				var totalLatency int64

				for instance := 0; instance < numInstances; instance++ {
					wg.Add(1)
					go func(instanceID int) {
						defer wg.Done()

						// Create page for this instance
						tmpl := template.Must(template.New("scaling").Parse(
							`<div>Instance {{.InstanceID}}: {{.Data}}</div>`,
						))

						page, err := apps[instanceID].NewApplicationPage(tmpl, map[string]interface{}{
							"InstanceID": instanceID,
							"Data":       "Initial data",
						})
						if err != nil {
							t.Errorf("Failed to create page for instance %d: %v", instanceID, err)
							return
						}
						defer func() {
							if err := page.Close(); err != nil {
								t.Logf("Warning: Failed to close page: %v", err)
							}
						}()

						// Simulate users for this instance
						for user := 0; user < usersPerInstance; user++ {
							wg.Add(1)
							go func(userID int) {
								defer wg.Done()

								for req := 0; req < 10; req++ {
									reqStart := time.Now()
									data := map[string]interface{}{
										"InstanceID": instanceID,
										"Data":       fmt.Sprintf("User %d Request %d", userID, req),
									}

									_, err := page.RenderFragments(context.Background(), data)
									reqDuration := time.Since(reqStart)

									atomic.AddInt64(&totalRequests, 1)
									atomic.AddInt64(&totalLatency, reqDuration.Nanoseconds())

									if err != nil {
										t.Logf("Request failed for instance %d user %d: %v", instanceID, userID, err)
									}
								}
							}(user)
						}
					}(instance)
				}

				wg.Wait()
				duration := time.Since(startTime)

				// Cleanup
				for _, app := range apps {
					if app != nil {
						if err := app.Close(); err != nil {
							fmt.Printf("Warning: Failed to close application: %v\n", err)
						}
					}
				}

				// Calculate scaling metrics
				avgLatency := float64(totalLatency) / float64(totalRequests) / 1000000
				requestsPerSecond := float64(totalRequests) / duration.Seconds()

				t.Logf("✓ %d instances with %d users each:", numInstances, usersPerInstance)
				t.Logf("  - Total users: %d", totalUsers)
				t.Logf("  - Total requests: %d", totalRequests)
				t.Logf("  - Avg latency: %.2f ms", avgLatency)
				t.Logf("  - Requests/sec: %.2f", requestsPerSecond)
				t.Logf("  - Duration: %v", duration)
			})
		}
	})
}

// TestResourceBottleneckIdentification identifies and resolves bottlenecks
func (suite *LoadTestSuite) TestResourceBottleneckIdentification(t *testing.T) {
	t.Run("CPU_Bottleneck_Detection", func(t *testing.T) {
		const numUsers = 30
		const cpuIntensiveOps = 20

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		// CPU-intensive template
		tmpl := template.Must(template.New("cpu-intensive").Parse(`
		<div>
		{{range .Items}}
			<p>{{.}}</p>
			{{range .SubItems}}
				<span>{{.}}</span>
			{{end}}
		{{end}}
		</div>
		`))

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"Items":    make([]string, 0),
			"SubItems": make([]string, 0),
		})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		startTime := time.Now()
		var wg sync.WaitGroup
		var totalLatency int64
		var requests int64

		// Generate CPU-intensive workload
		for user := 0; user < numUsers; user++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()

				for op := 0; op < cpuIntensiveOps; op++ {
					// Create computationally expensive data
					items := make([]string, 100)
					subItems := make([]string, 50)

					for i := range items {
						items[i] = fmt.Sprintf("CPU_Intensive_Item_%d_%d_%d", userID, op, i)
					}
					for i := range subItems {
						subItems[i] = fmt.Sprintf("SubItem_%d_%d_%d", userID, op, i)
					}

					data := map[string]interface{}{
						"Items":    items,
						"SubItems": subItems,
					}

					reqStart := time.Now()
					_, err := page.RenderFragments(context.Background(), data)
					reqDuration := time.Since(reqStart)

					atomic.AddInt64(&requests, 1)
					atomic.AddInt64(&totalLatency, reqDuration.Nanoseconds())

					if err != nil {
						t.Logf("CPU-intensive operation failed: %v", err)
					}
				}
			}(user)
		}

		wg.Wait()
		duration := time.Since(startTime)

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		avgLatency := float64(totalLatency) / float64(requests) / 1000000
		requestsPerSecond := float64(requests) / duration.Seconds()
		memoryDelta := float64(m2.Alloc-m1.Alloc) / 1024 / 1024

		t.Logf("✓ CPU bottleneck detection:")
		t.Logf("  - %d users × %d operations = %d total requests", numUsers, cpuIntensiveOps, requests)
		t.Logf("  - Avg latency: %.2f ms", avgLatency)
		t.Logf("  - Requests/sec: %.2f", requestsPerSecond)
		t.Logf("  - Memory delta: %.2f MB", memoryDelta)
		t.Logf("  - GC cycles: %d", m2.NumGC-m1.NumGC)

		// Identify CPU bottleneck
		if avgLatency > 100.0 {
			t.Logf("⚠ CPU bottleneck detected: high latency %.2f ms", avgLatency)
		}
		if requestsPerSecond < 50.0 {
			t.Logf("⚠ CPU bottleneck detected: low throughput %.2f RPS", requestsPerSecond)
		}
	})

	t.Run("Memory_Bottleneck_Detection", func(t *testing.T) {
		const numUsers = 20
		const memoryIntensiveOps = 15

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		app, err := NewApplication(WithMaxMemoryMB(100))
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("memory-intensive").Parse(`
		<div>{{range .LargeData}}<p>{{.}}</p>{{end}}</div>
		`))

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
			"LargeData": make([]string, 0),
		})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		var wg sync.WaitGroup
		var memoryErrors int64
		var successfulOps int64

		// Generate memory-intensive workload
		for user := 0; user < numUsers; user++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()

				for op := 0; op < memoryIntensiveOps; op++ {
					// Create memory-intensive data
					largeData := make([]string, 1000)
					for i := range largeData {
						largeData[i] = fmt.Sprintf("Large_Memory_Item_%d_%d_%d_%s",
							userID, op, i, strings.Repeat("X", 100))
					}

					data := map[string]interface{}{
						"LargeData": largeData,
					}

					_, err := page.RenderFragments(context.Background(), data)
					if err != nil {
						atomic.AddInt64(&memoryErrors, 1)
						if hasSubstring(err.Error(), "memory") || hasSubstring(err.Error(), "limit") {
							t.Logf("Memory bottleneck detected for user %d: %v", userID, err)
						}
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}
				}
			}(user)
		}

		wg.Wait()

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		memoryGrowth := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
		totalOps := successfulOps + memoryErrors
		memoryErrorRate := float64(memoryErrors) / float64(totalOps) * 100

		t.Logf("✓ Memory bottleneck detection:")
		t.Logf("  - %d users × %d operations", numUsers, memoryIntensiveOps)
		t.Logf("  - %d successful, %d memory errors", successfulOps, memoryErrors)
		t.Logf("  - Memory error rate: %.2f%%", memoryErrorRate)
		t.Logf("  - Memory growth: %.2f MB", memoryGrowth)
		t.Logf("  - GC cycles: %d", m2.NumGC-m1.NumGC)

		if memoryErrorRate > 10.0 {
			t.Logf("⚠ Memory bottleneck detected: %.2f%% memory errors", memoryErrorRate)
		}
		if memoryGrowth > 200.0 {
			t.Logf("⚠ Memory bottleneck detected: excessive growth %.2f MB", memoryGrowth)
		}
	})
}

// Helper methods for load testing

func (suite *LoadTestSuite) createBrowserSession(sessionID int) (*BrowserSession, error) {
	app, err := NewApplication(WithMaxMemoryMB(20))
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	tmpl := template.Must(template.New("browser-session").Parse(`
	<div id="session-{{.SessionID}}">
		<h2>{{.Title}}</h2>
		<p>Session: {{.SessionID}}</p>
		<div>{{.Content}}</div>
	</div>
	`))

	initialData := map[string]interface{}{
		"SessionID": sessionID,
		"Title":     fmt.Sprintf("Browser Session %d", sessionID),
		"Content":   fmt.Sprintf("Initial content for session %d", sessionID),
	}

	page, err := app.NewApplicationPage(tmpl, initialData)
	if err != nil {
		if err := app.Close(); err != nil {
			fmt.Printf("Warning: Failed to close application: %v\n", err)
		}
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	// Create HTTP server for this session
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html, err := page.Render()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			fmt.Printf("Warning: Failed to write HTML response: %v\n", err)
		}
	})

	server := httptest.NewServer(mux)

	session := &BrowserSession{
		ID:          fmt.Sprintf("session_%d", sessionID),
		Application: app,
		Page:        page,
		HttpClient:  &http.Client{Timeout: 5 * time.Second},
		Server:      server,
		SessionData: initialData,
	}

	return session, nil
}

func (suite *LoadTestSuite) testSessionIndependence(session *BrowserSession, sessionID int) error {
	// Test that session responds correctly
	resp, err := session.HttpClient.Get(session.Server.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to session: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: Failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (suite *LoadTestSuite) cleanupBrowserSession(session *BrowserSession) {
	if session.Server != nil {
		session.Server.Close()
	}
	if session.Page != nil {
		if err := session.Page.Close(); err != nil {
			suite.t.Logf("Warning: Failed to close session page: %v", err)
		}
	}
	if session.Application != nil {
		if err := session.Application.Close(); err != nil {
			suite.t.Logf("Warning: Failed to close session application: %v", err)
		}
	}
}

func (suite *LoadTestSuite) updateSessionData(session *BrowserSession, data map[string]interface{}) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	_, err := session.Page.RenderFragments(context.Background(), data)
	if err != nil {
		return err
	}

	session.SessionData = data
	session.RequestCount++
	session.LastRequestTime = time.Now()

	return nil
}

func (suite *LoadTestSuite) getSessionHTML(session *BrowserSession) (string, error) {
	session.mu.RLock()
	defer session.mu.RUnlock()

	return session.Page.Render()
}

func (suite *LoadTestSuite) simulateUserSession(userID int, duration time.Duration, maxRequests int) UserMetrics {
	startTime := time.Now()
	endTime := startTime.Add(duration)

	session, err := suite.createBrowserSession(userID)
	if err != nil {
		return UserMetrics{
			UserID:       userID,
			ErrorCount:   1,
			LastActivity: startTime,
		}
	}
	defer suite.cleanupBrowserSession(session)

	var requestCount int
	var successCount int
	var errorCount int
	var totalLatency time.Duration

	requestInterval := duration / time.Duration(maxRequests)
	if requestInterval < 10*time.Millisecond {
		requestInterval = 10 * time.Millisecond
	}

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	counter := 0
	for time.Now().Before(endTime) && requestCount < maxRequests {
		select {
		case <-ticker.C:
			counter++
			requestCount++

			// Generate realistic request data
			data := map[string]interface{}{
				"SessionID": userID,
				"Title":     fmt.Sprintf("User %d Activity %d", userID, counter),
				"Content":   fmt.Sprintf("User activity at %v", time.Now()),
				"Counter":   counter,
			}

			start := time.Now()
			err := suite.updateSessionData(session, data)
			latency := time.Since(start)

			totalLatency += latency

			if err != nil {
				errorCount++
			} else {
				successCount++
			}

		default:
			time.Sleep(requestInterval / 10)
		}
	}

	sessionDuration := time.Since(startTime)
	avgResponseTime := float64(totalLatency.Nanoseconds()) / float64(requestCount) / 1000000 // Convert to ms

	return UserMetrics{
		UserID:             userID,
		SessionDuration:    sessionDuration,
		TotalRequests:      requestCount,
		SuccessfulRequests: successCount,
		AvgResponseTime:    avgResponseTime,
		FragmentsGenerated: successCount, // Assume one fragment per successful request
		ErrorCount:         errorCount,
		LastActivity:       time.Now(),
	}
}

func (suite *LoadTestSuite) measurePerformanceAtScale(numUsers int, duration time.Duration) LoadTestMetrics {
	startTime := time.Now()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	initialMemory := float64(m1.Alloc) / 1024 / 1024

	app, err := NewApplication(WithMaxPages(numUsers + 10))
	if err != nil {
		return LoadTestMetrics{
			TotalUsers:   numUsers,
			TestDuration: duration,
			StartTime:    startTime,
			EndTime:      time.Now(),
		}
	}
	defer func() {
		if err := app.Close(); err != nil {
			fmt.Printf("Warning: Failed to close application: %v\n", err)
		}
	}()

	tmpl := template.Must(template.New("scale-test").Parse(`<div>{{.Data}}</div>`))
	page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
		"Data": "Performance test",
	})
	if err != nil {
		return LoadTestMetrics{
			TotalUsers:   numUsers,
			TestDuration: duration,
			StartTime:    startTime,
			EndTime:      time.Now(),
		}
	}
	defer func() {
		if err := page.Close(); err != nil {
			fmt.Printf("Warning: Failed to close page: %v\n", err)
		}
	}()

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var totalLatency int64
	var maxLatency int64
	var minLatency int64 = 999999999         // Start with high value
	responseTimes := make([]int64, 0, 10000) // Collect response times for percentiles
	var responseTimesMu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	// Launch user goroutines
	for user := 0; user < numUsers; user++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			counter := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					counter++
					atomic.AddInt64(&totalRequests, 1)

					data := map[string]interface{}{
						"Data": fmt.Sprintf("User %d Request %d", userID, counter),
					}

					start := time.Now()
					_, err := page.RenderFragments(context.Background(), data)
					latency := time.Since(start).Nanoseconds()

					// Update latency statistics
					atomic.AddInt64(&totalLatency, latency)
					for {
						currentMax := atomic.LoadInt64(&maxLatency)
						if latency <= currentMax || atomic.CompareAndSwapInt64(&maxLatency, currentMax, latency) {
							break
						}
					}
					for {
						currentMin := atomic.LoadInt64(&minLatency)
						if latency >= currentMin || atomic.CompareAndSwapInt64(&minLatency, currentMin, latency) {
							break
						}
					}

					// Collect response time for percentile calculation
					responseTimesMu.Lock()
					if len(responseTimes) < cap(responseTimes) {
						responseTimes = append(responseTimes, latency)
					}
					responseTimesMu.Unlock()

					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}

					time.Sleep(50 * time.Millisecond) // Simulate user think time
				}
			}
		}(user)
	}

	wg.Wait()
	endTime := time.Now()
	actualDuration := endTime.Sub(startTime)

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	finalMemory := float64(m2.Alloc) / 1024 / 1024

	// Calculate metrics
	avgResponseTime := float64(totalLatency) / float64(totalRequests) / 1000000 // Convert to ms
	requestsPerSecond := float64(totalRequests) / actualDuration.Seconds()
	errorRate := float64(failedRequests) / float64(totalRequests) * 100
	memoryGrowth := finalMemory - initialMemory
	memoryPerUser := memoryGrowth / float64(numUsers)

	// Calculate percentiles
	responseTimesMu.Lock()
	p50, p95, p99 := calculatePercentiles(responseTimes)
	responseTimesMu.Unlock()

	return LoadTestMetrics{
		TotalUsers:         numUsers,
		TestDuration:       actualDuration,
		StartTime:          startTime,
		EndTime:            endTime,
		AvgResponseTime:    avgResponseTime,
		P50ResponseTime:    float64(p50) / 1000000, // Convert to ms
		P95ResponseTime:    float64(p95) / 1000000,
		P99ResponseTime:    float64(p99) / 1000000,
		MaxResponseTime:    float64(maxLatency) / 1000000,
		MinResponseTime:    float64(minLatency) / 1000000,
		RequestsPerSecond:  requestsPerSecond,
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		InitialMemoryMB:    initialMemory,
		FinalMemoryMB:      finalMemory,
		MemoryGrowthMB:     memoryGrowth,
		MemoryPerUser:      memoryPerUser,
		ErrorRate:          errorRate,
		GoroutineCount:     runtime.NumGoroutine(),
		HeapObjects:        m2.HeapObjects,
		GCCycles:           m2.NumGC - m1.NumGC,
	}
}

// Helper functions

func hasSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func calculatePercentiles(values []int64) (p50, p95, p99 int64) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	// Simple sort for percentile calculation
	sorted := make([]int64, len(values))
	copy(sorted, values)

	// Simple bubble sort (good enough for tests)
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	p50 = sorted[int(float64(len(sorted))*0.5)]
	p95 = sorted[int(float64(len(sorted))*0.95)]
	p99 = sorted[int(float64(len(sorted))*0.99)]

	return p50, p95, p99
}
