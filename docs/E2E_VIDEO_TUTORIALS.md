# E2E Video Tutorials Guide for LiveTemplate

## Overview

This guide provides complete scripts, outlines, and production guidance for creating video tutorials covering complex LiveTemplate E2E testing scenarios. Each tutorial is designed to be 10-20 minutes long and covers specific aspects of the testing framework.

## Table of Contents

- [Tutorial Production Guidelines](#tutorial-production-guidelines)
- [Tutorial 1: Getting Started with E2E Testing](#tutorial-1-getting-started-with-e2e-testing)
- [Tutorial 2: Fragment Strategy Testing Deep Dive](#tutorial-2-fragment-strategy-testing-deep-dive)
- [Tutorial 3: Performance Testing and Optimization](#tutorial-3-performance-testing-and-optimization)
- [Tutorial 4: CI/CD Integration and Automation](#tutorial-4-cicd-integration-and-automation)
- [Tutorial 5: Troubleshooting Common Issues](#tutorial-5-troubleshooting-common-issues)
- [Tutorial 6: Advanced Testing Patterns](#tutorial-6-advanced-testing-patterns)
- [Tutorial 7: Real-World Application Testing](#tutorial-7-real-world-application-testing)

## Tutorial Production Guidelines

### Technical Requirements

**Recording Setup:**
- Screen resolution: 1920x1080 minimum
- Frame rate: 30fps
- Audio: Clear narration with noise-free background
- Code editor: Use high contrast theme with large fonts (14-16pt)
- Terminal: Use larger fonts and high contrast colors

**Software Recommendations:**
- Screen recording: OBS Studio, Camtasia, or ScreenFlow
- Video editing: DaVinci Resolve (free), Adobe Premiere, or Final Cut Pro
- Audio editing: Audacity (free) or Adobe Audition

**File Formats:**
- Master recording: 1920x1080, H.264, 30fps
- Distribution formats: MP4 (H.264), WebM for web compatibility
- Audio: AAC, 44.1kHz, stereo

### Content Structure

**Standard Tutorial Format:**
1. **Introduction (1-2 minutes)**: What will be covered, prerequisites
2. **Setup (2-3 minutes)**: Environment preparation, dependencies
3. **Main Content (10-15 minutes)**: Step-by-step implementation
4. **Troubleshooting (2-3 minutes)**: Common issues and solutions
5. **Summary (1-2 minutes)**: Key takeaways, next steps

**Visual Guidelines:**
- Use zoom/highlight for code sections
- Include captions for accessibility
- Show terminal commands and output clearly
- Use annotations to explain complex concepts
- Include chapter markers for easy navigation

## Tutorial 1: Getting Started with E2E Testing

### Duration: 15 minutes

### Script Outline

**[0:00 - 1:30] Introduction**
```
Narrator: "Welcome to LiveTemplate E2E Testing. I'm [Name], and in this tutorial, 
you'll learn how to set up and run your first E2E test using LiveTemplate's 
fragment-based testing framework.

By the end of this video, you'll be able to:
- Set up the E2E testing environment
- Write a basic E2E test
- Understand fragment strategies
- Run tests and interpret results

Prerequisites: You should have Go 1.23+, basic template knowledge, and 
Chrome/Chromium installed."
```

**[1:30 - 4:00] Environment Setup**
```
# Terminal commands to show on screen:

# 1. Verify prerequisites
go version
google-chrome --version

# 2. Clone project and setup
git clone [repository-url]
cd livetemplate
go mod download

# 3. Install dependencies and verify setup
./scripts/validate-ci.sh

# 4. Set environment variables
export LIVETEMPLATE_E2E_SCREENSHOTS=true
export LIVETEMPLATE_E2E_ARTIFACTS=./test-artifacts

# 5. Create test directories
mkdir -p test-artifacts screenshots
```

**[4:00 - 12:00] Writing Your First E2E Test**

*Show code editor with split screen: template on left, test code on right*

```go
// Show this code being typed step by step
package main

import (
    "testing"
    "html/template"
    "context"
    "time"
    "github.com/chromedp/chromedp"
)

func TestMyFirstE2ETest(t *testing.T) {
    E2ETestWithHelper(t, "first-e2e-test", func(helper *E2ETestHelper) error {
        // Step 1: Create template
        tmpl, err := template.New("hello").Parse(`
            <!DOCTYPE html>
            <html>
            <head><title>{{.Title}}</title></head>
            <body>
                <div data-lt-fragment="greeting">
                    <h1>Hello, {{.Name}}!</h1>
                    <p>Counter: {{.Count}}</p>
                </div>
            </body>
            </html>
        `)
        if err != nil {
            return err
        }
        
        // Step 2: Create application and page
        app, err := NewApplication()
        if err != nil {
            return err
        }
        defer app.Close()
        
        initialData := map[string]interface{}{
            "Title": "My First Test",
            "Name":  "World", 
            "Count": 1,
        }
        
        page, err := app.NewApplicationPage(tmpl, initialData)
        if err != nil {
            return err
        }
        defer page.Close()
        
        // Step 3: Create test server
        server := createBasicServer(app, page)
        defer server.Close()
        
        // Step 4: Browser automation
        ctx, cancel := helper.CreateBrowserContext()
        defer cancel()
        
        // Navigate to page
        err = chromedp.Run(ctx,
            chromedp.Navigate(server.URL),
            chromedp.WaitVisible("h1"),
        )
        if err != nil {
            helper.CaptureFailureScreenshot(ctx, t, "navigation failed")
            return err
        }
        
        helper.CaptureScreenshot(ctx, "initial-load")
        
        // Step 5: Test fragment update
        updateData := map[string]interface{}{
            "Title": "Updated Test",
            "Name":  "LiveTemplate",
            "Count": 42,
        }
        
        fragments, err := page.RenderFragments(context.Background(), updateData)
        if err != nil {
            return err
        }
        
        // Step 6: Validate results
        if len(fragments) == 0 {
            return fmt.Errorf("no fragments generated")
        }
        
        // Record metrics
        for _, fragment := range fragments {
            helper.RecordFragmentMetric(
                fragment.ID,
                fragment.Strategy, 
                5*time.Millisecond,
                200,
                0.85,
                false,
            )
        }
        
        helper.CaptureScreenshot(ctx, "test-complete")
        return nil
    })
}
```

**[12:00 - 13:30] Running the Test**
```
Narrator: "Now let's run our test and see what happens."

# Terminal commands:
go test -v -run "TestMyFirstE2ETest"

# Show test output, explain:
# - Test execution flow
# - Screenshot capture
# - Performance metrics
# - Success/failure indicators

# Show generated artifacts:
ls -la test-artifacts/
ls -la screenshots/
```

**[13:30 - 15:00] Understanding Results and Next Steps**
```
Narrator: "Let's examine what our test generated."

# Show and explain:
1. Screenshots captured
2. Performance metrics in JSON
3. Test report
4. Fragment strategy selection

# Explain fragment strategies briefly:
"Notice the fragment used 'static_dynamic' strategy because we only changed 
text content. LiveTemplate automatically selects the most efficient strategy 
based on what actually changed in the HTML."

# Next steps:
"In the next tutorial, we'll dive deeper into the four fragment strategies 
and learn how to test each one specifically."
```

### Recording Notes
- Use zoom to highlight code sections being explained
- Show browser window when demonstrating navigation
- Display screenshot files when explaining artifacts
- Use terminal split-screen to show commands and output simultaneously

## Tutorial 2: Fragment Strategy Testing Deep Dive

### Duration: 20 minutes

### Script Outline

**[0:00 - 2:00] Introduction and Strategy Overview**
```
Narrator: "Welcome back! In this tutorial, we'll master LiveTemplate's four 
fragment strategies and learn how to test each one effectively.

LiveTemplate uses HTML diffing to automatically select the optimal strategy:
- Static/Dynamic: 85-95% reduction for text-only changes (60-70% of cases)
- Markers: 70-85% reduction for attribute changes (15-20% of cases) 
- Granular: 60-80% reduction for structural changes (10-15% of cases)
- Replacement: 40-60% reduction for complex changes (5-10% of cases)

We'll write tests that specifically trigger each strategy and validate 
the performance characteristics."
```

**[2:00 - 6:00] Strategy 1: Static/Dynamic Testing**

*Show template and test code side-by-side*

```go
func TestStaticDynamicStrategy(t *testing.T) {
    E2ETestWithHelper(t, "static-dynamic", func(helper *E2ETestHelper) error {
        // Template with text-only content
        tmpl, _ := template.New("static-dynamic").Parse(`
            <div data-lt-fragment="content">
                <h1>{{.Title}}</h1>
                <p>Welcome {{.UserName}}</p>
                <span>Count: {{.Count}}</span>
            </div>
        `)
        
        // ... setup code ...
        
        // Initial data
        initialData := map[string]interface{}{
            "Title": "Original Title",
            "UserName": "John",
            "Count": 1,
        }
        
        // Update with ONLY text changes - no attributes, no structure
        updateData := map[string]interface{}{
            "Title": "Updated Title",      // Changed
            "UserName": "Jane",           // Changed  
            "Count": 42,                  // Changed
        }
        
        fragments, err := page.RenderFragments(ctx, updateData)
        
        // Validate static/dynamic strategy was used
        for _, fragment := range fragments {
            if fragment.Strategy == "static_dynamic" {
                t.Log("✅ Static/Dynamic strategy selected")
                // Should achieve 85-95% compression
                helper.RecordFragmentMetric(fragment.ID, "static_dynamic", 
                    8*time.Millisecond, 150, 0.90, false)
                
                // Validate data structure
                data := fragment.Data.(map[string]interface{})
                if _, hasStatics := data["statics"]; !hasStatics {
                    return fmt.Errorf("missing statics array")
                }
                if _, hasDynamics := data["dynamics"]; !hasDynamics {
                    return fmt.Errorf("missing dynamics map")
                }
            }
        }
        
        return nil
    })
}
```

**[6:00 - 10:00] Strategy 2: Markers Testing**

```go
func TestMarkersStrategy(t *testing.T) {
    E2ETestWithHelper(t, "markers", func(helper *E2ETestHelper) error {
        tmpl, _ := template.New("markers").Parse(`
            <div data-lt-fragment="styled-content">
                <div class="{{.CSSClass}}" 
                     style="{{.InlineStyle}}"
                     data-state="{{.State}}">
                    Same Text Content  <!-- Text unchanged -->
                </div>
                <input type="{{.InputType}}" 
                       value="{{.InputValue}}"
                       placeholder="{{.Placeholder}}">
            </div>
        `)
        
        // Initial data
        initialData := map[string]interface{}{
            "CSSClass": "default-style",
            "InlineStyle": "color: blue;",
            "State": "normal",
            "InputType": "text",
            "InputValue": "initial",
            "Placeholder": "Enter text",
        }
        
        // Update ONLY attributes - same text content
        updateData := map[string]interface{}{
            "CSSClass": "highlighted-style",     // Attribute change
            "InlineStyle": "color: red; font-weight: bold;", // Attribute change
            "State": "active",                   // Attribute change
            "InputType": "email",                // Attribute change
            "InputValue": "test@example.com",    // Attribute change
            "Placeholder": "Enter email",        // Attribute change
        }
        
        fragments, err := page.RenderFragments(ctx, updateData)
        
        // Validate markers strategy
        for _, fragment := range fragments {
            if fragment.Strategy == "markers" {
                t.Log("✅ Markers strategy selected")
                // Should achieve 70-85% compression
                helper.RecordFragmentMetric(fragment.ID, "markers",
                    12*time.Millisecond, 250, 0.78, false)
                
                // Validate markers data structure
                data := fragment.Data.(map[string]interface{})
                if _, hasPositions := data["positions"]; !hasPositions {
                    return fmt.Errorf("missing positions data")
                }
            }
        }
        
        return nil
    })
}
```

**[10:00 - 14:00] Strategy 3: Granular Testing**

```go
func TestGranularStrategy(t *testing.T) {
    E2ETestWithHelper(t, "granular", func(helper *E2ETestHelper) error {
        tmpl, _ := template.New("granular").Parse(`
            <div data-lt-fragment="list-content">
                <ul>
                    {{range .Items}}
                    <li data-id="{{.ID}}">{{.Name}}</li>
                    {{end}}
                </ul>
            </div>
        `)
        
        // Initial data
        initialData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "1", "Name": "Item 1"},
                {"ID": "2", "Name": "Item 2"},
            },
        }
        
        // Add new item - structural change
        updateData := map[string]interface{}{
            "Items": []map[string]interface{}{
                {"ID": "1", "Name": "Item 1"},  // Same
                {"ID": "2", "Name": "Item 2"},  // Same
                {"ID": "3", "Name": "Item 3"},  // Added
            },
        }
        
        fragments, err := page.RenderFragments(ctx, updateData)
        
        // Validate granular strategy
        for _, fragment := range fragments {
            if fragment.Strategy == "granular" {
                t.Log("✅ Granular strategy selected")
                // Should achieve 60-80% compression
                helper.RecordFragmentMetric(fragment.ID, "granular",
                    18*time.Millisecond, 400, 0.70, false)
                
                // Validate granular operations
                data := fragment.Data.(map[string]interface{})
                if _, hasOperations := data["operations"]; !hasOperations {
                    return fmt.Errorf("missing operations data")
                }
            }
        }
        
        return nil
    })
}
```

**[14:00 - 18:00] Strategy 4: Replacement Testing**

```go
func TestReplacementStrategy(t *testing.T) {
    E2ETestWithHelper(t, "replacement", func(helper *E2ETestHelper) error {
        tmpl, _ := template.New("replacement").Parse(`
            <div data-lt-fragment="complex-content">
                {{if eq .Layout "list"}}
                <ul class="{{.CSSClass}}">
                    {{range .Items}}
                    <li class="{{.ItemClass}}">{{.Name}}: {{.Value}}</li>
                    {{end}}
                </ul>
                {{else if eq .Layout "table"}}
                <table class="{{.CSSClass}}">
                    {{range .Items}}
                    <tr class="{{.ItemClass}}">
                        <td>{{.Name}}</td>
                        <td>{{.Value}}</td>
                    </tr>
                    {{end}}
                </table>
                {{end}}
            </div>
        `)
        
        // Initial data
        initialData := map[string]interface{}{
            "Layout": "list",
            "CSSClass": "default-list",
            "ItemClass": "item-normal",
            "Items": []map[string]interface{}{
                {"Name": "Item A", "Value": "Value 1"},
                {"Name": "Item B", "Value": "Value 2"},
            },
        }
        
        // Complex mixed changes: structure + attributes + text
        updateData := map[string]interface{}{
            "Layout": "table",               // Structure change
            "CSSClass": "highlighted-table", // Attribute change
            "ItemClass": "item-featured",    // Attribute change
            "Items": []map[string]interface{}{
                {"Name": "Updated A", "Value": "New Value 1"}, // Text change
                {"Name": "Updated B", "Value": "New Value 2"}, // Text change
                {"Name": "Item C", "Value": "Value 3"},        // Structure change
            },
        }
        
        fragments, err := page.RenderFragments(ctx, updateData)
        
        // Validate replacement strategy
        for _, fragment := range fragments {
            if fragment.Strategy == "replacement" {
                t.Log("✅ Replacement strategy selected")
                // Should achieve 40-60% compression
                helper.RecordFragmentMetric(fragment.ID, "replacement",
                    28*time.Millisecond, 800, 0.50, false)
                
                // Validate replacement data
                data := fragment.Data.(map[string]interface{})
                if _, hasHTML := data["html"]; !hasHTML {
                    return fmt.Errorf("missing HTML data")
                }
            }
        }
        
        return nil
    })
}
```

**[18:00 - 20:00] Running All Strategy Tests**

```
# Terminal demonstration:
go test -v -run "TestStaticDynamicStrategy"
go test -v -run "TestMarkersStrategy" 
go test -v -run "TestGranularStrategy"
go test -v -run "TestReplacementStrategy"

# Show and explain results:
- Performance metrics for each strategy
- Compression ratios achieved
- Strategy distribution patterns
- Screenshot artifacts

# Explain strategy selection logic:
"Notice how LiveTemplate automatically selected the right strategy based on 
what actually changed in the HTML. This is the power of HTML diffing-based 
strategy selection."
```

### Recording Notes
- Show side-by-side comparison of HTML before/after for each strategy
- Highlight the specific changes that trigger each strategy
- Demonstrate performance differences between strategies
- Show the actual fragment data structures generated

## Tutorial 3: Performance Testing and Optimization

### Duration: 18 minutes

### Script Outline

**[0:00 - 2:00] Introduction to Performance Testing**
```
Narrator: "Performance is critical in real-world applications. In this tutorial, 
you'll learn how to performance test your LiveTemplate fragments, set up 
benchmarks, detect regressions, and optimize for production workloads.

We'll cover:
- Fragment generation benchmarks
- Load testing with concurrent users
- Memory leak detection
- Performance regression tracking
- CI/CD integration for automated monitoring"
```

**[2:00 - 6:00] Fragment Generation Benchmarks**

*Show benchmark test being written*

```go
func BenchmarkFragmentGeneration(b *testing.B) {
    // Setup template and data
    tmpl := setupPerformanceTemplate()
    app, _ := NewApplication()
    defer app.Close()
    
    initialData := generateLargeDataset(1000) // 1000 items
    page, _ := app.NewApplicationPage(tmpl, initialData)
    defer page.Close()
    
    // Benchmark different change types
    testCases := []struct {
        name     string
        dataGen  func() map[string]interface{}
        maxTime  time.Duration
    }{
        {"text-only-changes", generateTextChanges, 10 * time.Millisecond},
        {"attribute-changes", generateAttributeChanges, 15 * time.Millisecond},
        {"structural-changes", generateStructuralChanges, 20 * time.Millisecond},
        {"complex-changes", generateComplexChanges, 30 * time.Millisecond},
    }
    
    for _, tc := range testCases {
        b.Run(tc.name, func(b *testing.B) {
            updateData := tc.dataGen()
            
            b.ResetTimer()
            start := time.Now()
            
            for i := 0; i < b.N; i++ {
                fragments, err := page.RenderFragments(context.Background(), updateData)
                if err != nil || len(fragments) == 0 {
                    b.Fatalf("Fragment generation failed: %v", err)
                }
            }
            
            avgTime := time.Since(start) / time.Duration(b.N)
            
            // Validate performance target
            if avgTime > tc.maxTime {
                b.Errorf("Average time %v exceeds target %v", avgTime, tc.maxTime)
            }
            
            b.Logf("Strategy performance: avg %v, target %v", avgTime, tc.maxTime)
        })
    }
}
```

**[6:00 - 10:00] Load Testing with Concurrent Users**

```go
func TestConcurrentUserLoad(t *testing.T) {
    E2ETestWithHelper(t, "concurrent-load", func(helper *E2ETestHelper) error {
        // Test with increasing concurrent users
        userCounts := []int{1, 5, 10, 25, 50}
        
        for _, userCount := range userCounts {
            t.Logf("Testing %d concurrent users", userCount)
            
            // Generate load test scenario
            scenario := generateLoadTestScenario(userCount, 10) // 10 updates per user
            
            // Execute concurrent load test
            results := make(chan LoadTestResult, userCount*10)
            var wg sync.WaitGroup
            
            start := time.Now()
            
            for i := 0; i < userCount; i++ {
                wg.Add(1)
                go func(userID int) {
                    defer wg.Done()
                    
                    // Create page for this user
                    page, _ := app.NewApplicationPage(tmpl, scenario.InitialData[userID])
                    defer page.Close()
                    
                    // Execute updates
                    for j, updateData := range scenario.Updates[userID] {
                        updateStart := time.Now()
                        fragments, err := page.RenderFragments(context.Background(), updateData)
                        updateTime := time.Since(updateStart)
                        
                        results <- LoadTestResult{
                            UserID:     userID,
                            UpdateID:   j,
                            Duration:   updateTime,
                            Success:    err == nil,
                            Fragments:  len(fragments),
                        }
                    }
                }(i)
            }
            
            wg.Wait()
            close(results)
            totalTime := time.Since(start)
            
            // Analyze results
            var successCount, totalOps int
            var totalDuration time.Duration
            
            for result := range results {
                totalOps++
                if result.Success {
                    successCount++
                }
                totalDuration += result.Duration
            }
            
            successRate := float64(successCount) / float64(totalOps)
            avgResponseTime := totalDuration / time.Duration(totalOps)
            throughput := float64(totalOps) / totalTime.Seconds()
            
            // Record metrics
            helper.SetCustomMetric(fmt.Sprintf("load_test_%d_users_success_rate", userCount), successRate)
            helper.SetCustomMetric(fmt.Sprintf("load_test_%d_users_avg_response", userCount), avgResponseTime)
            helper.SetCustomMetric(fmt.Sprintf("load_test_%d_users_throughput", userCount), throughput)
            
            t.Logf("Results for %d users:", userCount)
            t.Logf("  Success rate: %.2f%%", successRate*100)
            t.Logf("  Avg response: %v", avgResponseTime)
            t.Logf("  Throughput: %.2f ops/sec", throughput)
            
            // Validate performance doesn't degrade significantly
            if successRate < 0.95 {
                return fmt.Errorf("success rate %.2f%% below threshold", successRate*100)
            }
        }
        
        return nil
    })
}
```

**[10:00 - 13:00] Memory Leak Detection**

```go
func TestMemoryLeakDetection(t *testing.T) {
    E2ETestWithHelper(t, "memory-leak", func(helper *E2ETestHelper) error {
        const iterations = 1000
        
        var initialMem, currentMem runtime.MemStats
        runtime.GC()
        runtime.ReadMemStats(&initialMem)
        
        // Setup template and page
        tmpl := setupMemoryTestTemplate()
        app, _ := NewApplication()
        defer app.Close()
        
        page, _ := app.NewApplicationPage(tmpl, generateTestData(100))
        defer page.Close()
        
        // Perform many updates to detect memory leaks
        for i := 0; i < iterations; i++ {
            // Vary data size to stress memory allocation
            updateData := generateTestData(100 + i%50)
            
            fragments, err := page.RenderFragments(context.Background(), updateData)
            if err != nil {
                return fmt.Errorf("iteration %d failed: %w", i, err)
            }
            
            if len(fragments) == 0 {
                return fmt.Errorf("no fragments at iteration %d", i)
            }
            
            // Check memory every 100 iterations
            if i%100 == 0 {
                runtime.GC()
                runtime.ReadMemStats(&currentMem)
                
                memIncrease := int64(currentMem.Alloc - initialMem.Alloc)
                helper.SetCustomMetric(fmt.Sprintf("memory_usage_iter_%d", i), memIncrease)
                
                t.Logf("Memory at iteration %d: %d bytes (increase: %d bytes)", 
                    i, currentMem.Alloc, memIncrease)
                
                // Alert if memory growth is excessive
                expectedGrowth := int64(i * 1024) // 1KB per iteration expected
                if memIncrease > expectedGrowth*10 { // 10x threshold
                    t.Logf("⚠️  High memory growth detected: %d bytes", memIncrease)
                }
            }
        }
        
        // Final memory check
        runtime.GC()
        runtime.ReadMemStats(&currentMem)
        finalIncrease := int64(currentMem.Alloc - initialMem.Alloc)
        
        helper.SetCustomMetric("final_memory_increase", finalIncrease)
        helper.SetCustomMetric("memory_per_iteration", finalIncrease/iterations)
        
        t.Logf("Final memory analysis:")
        t.Logf("  Total increase: %d bytes", finalIncrease)
        t.Logf("  Per iteration: %d bytes", finalIncrease/iterations)
        
        // Fail if memory leak is significant
        const memoryThreshold = 50 * 1024 * 1024 // 50MB
        if finalIncrease > memoryThreshold {
            return fmt.Errorf("memory leak detected: %d bytes", finalIncrease)
        }
        
        t.Log("✅ No significant memory leaks detected")
        return nil
    })
}
```

**[13:00 - 16:00] Performance Regression Detection**

*Show setting up baseline and detection system*

```bash
# Terminal commands to show:

# 1. Run performance baseline
go test -bench=BenchmarkFragmentGeneration -benchmem > baseline.txt

# 2. Show baseline results
cat baseline.txt

# 3. Make performance change (simulate regression)
# Edit code to add artificial delay...

# 4. Run benchmarks again
go test -bench=BenchmarkFragmentGeneration -benchmem > current.txt

# 5. Compare results
benchcmp baseline.txt current.txt

# 6. Automated regression detection
go run performance-analysis.go baseline.txt current.txt
```

**[16:00 - 18:00] CI/CD Integration and Monitoring**

*Show GitHub Actions configuration*

```yaml
name: Performance Testing

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  performance-tests:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    - name: Run Performance Benchmarks
      run: |
        go test -bench=. -benchmem -count=5 > benchmark-results.txt
        
    - name: Load Testing
      run: |
        go test -v -run "TestConcurrentUserLoad"
        
    - name: Memory Leak Detection
      run: |
        go test -v -run "TestMemoryLeakDetection"
        
    - name: Performance Regression Analysis
      run: |
        if [ -f previous-benchmark.txt ]; then
          benchcmp previous-benchmark.txt benchmark-results.txt
        fi
        
    - name: Upload Performance Artifacts
      uses: actions/upload-artifact@v3
      with:
        name: performance-results
        path: |
          benchmark-results.txt
          test-artifacts/
        retention-days: 30
```

### Recording Notes
- Show live benchmark execution with timing results
- Demonstrate memory usage graphs during leak testing
- Display performance regression detection in action
- Show CI/CD pipeline running performance tests

## Tutorial 4: CI/CD Integration and Automation

### Duration: 16 minutes

### Script Outline

**[0:00 - 2:00] Introduction to CI/CD Integration**
```
Narrator: "Automated testing is crucial for maintaining quality in production. 
In this tutorial, you'll learn how to integrate LiveTemplate E2E tests into 
your CI/CD pipeline with GitHub Actions, including parallel execution, 
artifact collection, and comprehensive reporting."
```

**[2:00 - 5:00] GitHub Actions Workflow Setup**

*Show creating .github/workflows/e2e-tests.yml*

```yaml
name: E2E Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        test-group: [infrastructure, browser-lifecycle, performance, error-scenarios]
        
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
        
    - name: Install Chrome
      run: |
        wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
        sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
        sudo apt update
        sudo apt install -y google-chrome-stable
        
    - name: Install dependencies
      run: go mod download
      
    - name: Run E2E Tests
      env:
        CHROME_BIN: /usr/bin/google-chrome-stable
        LIVETEMPLATE_E2E_SCREENSHOTS: true
        LIVETEMPLATE_E2E_ARTIFACTS: ./test-artifacts
        CI: true
      run: |
        ./scripts/run-e2e-tests.sh ${{ matrix.test-group }}
        
    - name: Upload test artifacts
      if: always()
      uses: actions/upload-artifact@v3
      with:
        name: e2e-artifacts-${{ matrix.test-group }}
        path: |
          test-artifacts/
          screenshots/
        retention-days: 30
        
    - name: Generate test report
      if: always()
      run: |
        ./scripts/generate-test-report.sh ${{ matrix.test-group }}
        
    - name: Comment PR with results
      if: github.event_name == 'pull_request'
      uses: actions/github-script@v6
      with:
        script: |
          const fs = require('fs');
          const report = fs.readFileSync('./test-artifacts/test-report.md', 'utf8');
          
          github.rest.issues.createComment({
            issue_number: context.issue.number,
            owner: context.repo.owner,
            repo: context.repo.repo,
            body: `## E2E Test Results - ${{ matrix.test-group }}\n\n${report}`
          });
```

**[5:00 - 8:00] Advanced E2E Test Runner Script**

*Show creating scripts/run-e2e-tests.sh*

```bash
#!/bin/bash
set -e

# E2E Test Runner with retry logic and comprehensive reporting
TEST_GROUP=${1:-"all"}
MAX_RETRIES=3
RETRY_DELAY=30

echo "🚀 Starting E2E tests for group: $TEST_GROUP"

# Setup test environment
export CHROME_BIN=${CHROME_BIN:-$(which google-chrome-stable || which chromium-browser)}
export LIVETEMPLATE_E2E_SCREENSHOTS=${LIVETEMPLATE_E2E_SCREENSHOTS:-"true"}
export LIVETEMPLATE_E2E_ARTIFACTS=${LIVETEMPLATE_E2E_ARTIFACTS:-"./test-artifacts"}

# Create directories
mkdir -p "$LIVETEMPLATE_E2E_ARTIFACTS/logs"
mkdir -p "screenshots"

# Function to run tests with retry
run_test_with_retry() {
    local test_pattern=$1
    local retry_count=0
    
    while [ $retry_count -lt $MAX_RETRIES ]; do
        echo "🔄 Attempt $((retry_count + 1))/$MAX_RETRIES for $test_pattern"
        
        if go test -v -timeout=30m -run "$test_pattern" ./... 2>&1 | tee "$LIVETEMPLATE_E2E_ARTIFACTS/logs/$test_pattern.log"; then
            echo "✅ $test_pattern passed"
            return 0
        else
            echo "❌ $test_pattern failed on attempt $((retry_count + 1))"
            retry_count=$((retry_count + 1))
            
            if [ $retry_count -lt $MAX_RETRIES ]; then
                echo "⏰ Waiting ${RETRY_DELAY}s before retry..."
                sleep $RETRY_DELAY
            fi
        fi
    done
    
    echo "💥 $test_pattern failed after $MAX_RETRIES attempts"
    return 1
}

# Define test groups
case $TEST_GROUP in
    "infrastructure")
        echo "🏗️ Running infrastructure tests..."
        run_test_with_retry "TestE2EInfrastructure" || exit 1
        ;;
        
    "browser-lifecycle")
        echo "🌐 Running browser lifecycle tests..."
        run_test_with_retry "TestE2EBrowserLifecycle" || exit 1
        ;;
        
    "performance")
        echo "⚡ Running performance tests..."
        run_test_with_retry "TestE2EPerformance" || exit 1
        run_test_with_retry "TestConcurrentUserLoad" || exit 1
        ;;
        
    "error-scenarios")
        echo "🚨 Running error scenario tests..."
        run_test_with_retry "TestE2EErrorHandling" || exit 1
        ;;
        
    "all")
        echo "🎯 Running all test groups..."
        run_test_with_retry "TestE2EInfrastructure" || exit 1
        run_test_with_retry "TestE2EBrowserLifecycle" || exit 1
        run_test_with_retry "TestE2EPerformance" || exit 1
        run_test_with_retry "TestE2EErrorHandling" || exit 1
        ;;
        
    *)
        echo "❓ Unknown test group: $TEST_GROUP"
        echo "Available groups: infrastructure, browser-lifecycle, performance, error-scenarios, all"
        exit 1
        ;;
esac

echo "🎉 All tests in group '$TEST_GROUP' completed successfully!"

# Generate summary report
echo "📊 Generating test summary..."
{
    echo "# E2E Test Summary - $TEST_GROUP"
    echo ""
    echo "**Date:** $(date)"
    echo "**Environment:** $CI"
    echo "**Chrome:** $($CHROME_BIN --version 2>/dev/null || echo 'Not found')"
    echo ""
    
    echo "## Test Results"
    for log_file in "$LIVETEMPLATE_E2E_ARTIFACTS"/logs/*.log; do
        if [[ -f "$log_file" ]]; then
            test_name=$(basename "$log_file" .log)
            if grep -q "PASS" "$log_file"; then
                echo "- ✅ $test_name: PASSED"
            else
                echo "- ❌ $test_name: FAILED"
            fi
        fi
    done
    
    echo ""
    echo "## Artifacts"
    echo "- Screenshots: $(find screenshots -name "*.png" | wc -l) files"
    echo "- Test artifacts: $(find "$LIVETEMPLATE_E2E_ARTIFACTS" -type f | wc -l) files"
    
} > "$LIVETEMPLATE_E2E_ARTIFACTS/summary.md"

echo "📋 Summary report generated: $LIVETEMPLATE_E2E_ARTIFACTS/summary.md"
```

**[8:00 - 11:00] Cross-Platform Testing**

*Show matrix strategy for multiple OS*

```yaml
# Extended workflow for cross-platform testing
name: Cross-Platform E2E Tests

on:
  push:
    branches: [ main ]

jobs:
  cross-platform-tests:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        test-group: [infrastructure, browser-lifecycle]
        
    runs-on: ${{ matrix.os }}
    
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    # OS-specific Chrome installation
    - name: Install Chrome (Linux)
      if: runner.os == 'Linux'
      run: |
        wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
        sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
        sudo apt update
        sudo apt install -y google-chrome-stable xvfb
        
    - name: Install Chrome (macOS)
      if: runner.os == 'macOS'
      run: |
        brew install --cask google-chrome
        
    - name: Install Chrome (Windows)
      if: runner.os == 'Windows'
      run: |
        choco install googlechrome
    
    # OS-specific environment setup
    - name: Setup Environment (Linux)
      if: runner.os == 'Linux'
      run: |
        export DISPLAY=:99
        Xvfb :99 -ac -screen 0 1920x1080x24 &
        echo "DISPLAY=:99" >> $GITHUB_ENV
        echo "CHROME_BIN=/usr/bin/google-chrome-stable" >> $GITHUB_ENV
        
    - name: Setup Environment (macOS)
      if: runner.os == 'macOS'
      run: |
        echo "CHROME_BIN=/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" >> $GITHUB_ENV
        
    - name: Setup Environment (Windows)
      if: runner.os == 'Windows'
      run: |
        echo "CHROME_BIN=C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe" >> $env:GITHUB_ENV
    
    - name: Run E2E Tests
      env:
        LIVETEMPLATE_E2E_SCREENSHOTS: true
        CI: true
      run: |
        go test -v -timeout=20m -run "TestE2E${{ matrix.test-group }}" ./...
```

**[11:00 - 14:00] Docker Integration for Consistent Testing**

*Show Dockerfile and docker-compose setup*

```dockerfile
# Dockerfile.e2e - Consistent testing environment
FROM golang:1.23-bullseye

# Install Chrome and dependencies
RUN apt-get update && apt-get install -y \
    wget gnupg ca-certificates apt-transport-https \
    && wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update \
    && apt-get install -y google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

# Install Xvfb for headless testing
RUN apt-get update && apt-get install -y xvfb && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Set environment variables
ENV CHROME_BIN=/usr/bin/google-chrome-stable
ENV LIVETEMPLATE_E2E_SCREENSHOTS=true
ENV DISPLAY=:99

# Create entrypoint script
RUN echo '#!/bin/bash\nset -e\nXvfb :99 -ac -screen 0 1920x1080x24 &\nsleep 2\nexec "$@"' > /entrypoint.sh \
    && chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["go", "test", "-v", "-run", "TestE2E", "./..."]
```

```yaml
# docker-compose.e2e.yml
version: '3.8'

services:
  e2e-tests:
    build:
      context: .
      dockerfile: Dockerfile.e2e
    volumes:
      - ./test-artifacts:/app/test-artifacts
      - ./screenshots:/app/screenshots
    environment:
      - LIVETEMPLATE_E2E_SCREENSHOTS=true
      - LIVETEMPLATE_E2E_ARTIFACTS=/app/test-artifacts
    command: ["./scripts/run-e2e-tests.sh", "all"]
    
  e2e-performance:
    build:
      context: .
      dockerfile: Dockerfile.e2e
    volumes:
      - ./test-artifacts:/app/test-artifacts
    environment:
      - LIVETEMPLATE_E2E_SCREENSHOTS=false
    command: ["go", "test", "-v", "-run", "TestE2EPerformance"]
```

**[14:00 - 16:00] Advanced Reporting and Analytics**

*Show comprehensive reporting system*

```go
// scripts/generate-test-report.go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

type TestReport struct {
    TestGroup      string                 `json:"test_group"`
    Timestamp      time.Time             `json:"timestamp"`
    Environment    map[string]string     `json:"environment"`
    TestResults    []TestResult          `json:"test_results"`
    Performance    PerformanceMetrics    `json:"performance"`
    Screenshots    []string              `json:"screenshots"`
    Success        bool                  `json:"success"`
}

func main() {
    testGroup := os.Args[1]
    
    report := TestReport{
        TestGroup:   testGroup,
        Timestamp:   time.Now(),
        Environment: collectEnvironmentInfo(),
        TestResults: parseTestLogs("./test-artifacts/logs/"),
        Performance: collectPerformanceMetrics("./test-artifacts/"),
        Screenshots: collectScreenshots("./screenshots/"),
    }
    
    // Determine overall success
    report.Success = true
    for _, result := range report.TestResults {
        if !result.Passed {
            report.Success = false
            break
        }
    }
    
    // Generate JSON report
    jsonData, _ := json.MarshalIndent(report, "", "  ")
    os.WriteFile("./test-artifacts/test-report.json", jsonData, 0644)
    
    // Generate Markdown report
    markdownReport := generateMarkdownReport(report)
    os.WriteFile("./test-artifacts/test-report.md", []byte(markdownReport), 0644)
    
    // Generate HTML report with charts
    htmlReport := generateHTMLReport(report)
    os.WriteFile("./test-artifacts/test-report.html", []byte(htmlReport), 0644)
    
    fmt.Printf("📊 Generated comprehensive test report for %s\n", testGroup)
    if !report.Success {
        os.Exit(1)
    }
}

func generateMarkdownReport(report TestReport) string {
    var md strings.Builder
    
    md.WriteString(fmt.Sprintf("# E2E Test Report - %s\n\n", report.TestGroup))
    md.WriteString(fmt.Sprintf("**Generated:** %s\n", report.Timestamp.Format(time.RFC3339)))
    md.WriteString(fmt.Sprintf("**Status:** %s\n\n", func() string {
        if report.Success { return "✅ PASSED" }
        return "❌ FAILED"
    }()))
    
    // Environment info
    md.WriteString("## Environment\n\n")
    for key, value := range report.Environment {
        md.WriteString(fmt.Sprintf("- **%s:** %s\n", key, value))
    }
    md.WriteString("\n")
    
    // Test results
    md.WriteString("## Test Results\n\n")
    for _, result := range report.TestResults {
        status := "✅"
        if !result.Passed {
            status = "❌"
        }
        md.WriteString(fmt.Sprintf("- %s **%s** (%v)\n", status, result.Name, result.Duration))
    }
    md.WriteString("\n")
    
    // Performance summary
    md.WriteString("## Performance Summary\n\n")
    md.WriteString(fmt.Sprintf("- **Total Fragments Generated:** %d\n", report.Performance.TotalFragments))
    md.WriteString(fmt.Sprintf("- **Average Generation Time:** %v\n", report.Performance.AvgGenerationTime))
    md.WriteString(fmt.Sprintf("- **Strategy Distribution:**\n"))
    for strategy, count := range report.Performance.StrategyDistribution {
        md.WriteString(fmt.Sprintf("  - %s: %d\n", strategy, count))
    }
    md.WriteString("\n")
    
    // Screenshots
    if len(report.Screenshots) > 0 {
        md.WriteString("## Screenshots\n\n")
        md.WriteString(fmt.Sprintf("%d screenshots captured:\n\n", len(report.Screenshots)))
        for _, screenshot := range report.Screenshots {
            md.WriteString(fmt.Sprintf("- `%s`\n", filepath.Base(screenshot)))
        }
    }
    
    return md.String()
}
```

### Recording Notes
- Show live GitHub Actions workflow execution
- Demonstrate parallel test execution across matrix
- Display comprehensive test reports and artifacts
- Show Docker container testing for consistency

This video tutorial guide provides complete production-ready scripts and configurations for creating comprehensive video tutorials covering all aspects of LiveTemplate E2E testing. Each tutorial is designed to be practical, showing real code and demonstrating actual functionality.