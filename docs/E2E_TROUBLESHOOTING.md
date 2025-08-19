# E2E Testing Troubleshooting Guide for LiveTemplate

## Overview

This comprehensive troubleshooting guide helps diagnose and resolve common issues encountered during LiveTemplate E2E testing, covering browser automation, fragment generation, performance problems, and CI/CD integration challenges.

## Table of Contents

- [Quick Diagnostic Checklist](#quick-diagnostic-checklist)
- [Browser Automation Issues](#browser-automation-issues)
- [Fragment Generation Problems](#fragment-generation-problems)
- [Performance Issues](#performance-issues)
- [Template and Data Problems](#template-and-data-problems)
- [CI/CD Integration Issues](#cicd-integration-issues)
- [Network and Connectivity Problems](#network-and-connectivity-problems)
- [Memory and Resource Issues](#memory-and-resource-issues)
- [Debug Tools and Commands](#debug-tools-and-commands)

## Quick Diagnostic Checklist

### Pre-Flight Checklist

Before diving into specific troubleshooting, run through this checklist:

```bash
# 1. Verify Go installation
go version
# Expected: go1.23 or higher

# 2. Check Chrome/Chromium installation
google-chrome-stable --version
# or
chromium-browser --version

# 3. Verify dependencies
go mod verify
go mod download

# 4. Check environment variables
echo $CHROME_BIN
echo $LIVETEMPLATE_E2E_SCREENSHOTS
echo $LIVETEMPLATE_E2E_ARTIFACTS

# 5. Run basic validation
./scripts/validate-ci.sh

# 6. Test browser automation
go run -c "
package main
import (
    \"context\"
    \"github.com/chromedp/chromedp\"
    \"log\"
    \"time\"
)
func main() {
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()
    ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    err := chromedp.Run(ctx, chromedp.Navigate(\"https://example.com\"))
    if err != nil {
        log.Fatalf(\"Browser test failed: %v\", err)
    }
    log.Println(\"✅ Browser automation working\")
}
"
```

### Quick Status Check

```bash
# Check system resources
free -h                    # Memory usage
df -h                     # Disk space
ps aux | grep chrome      # Chrome processes
lsof -i :8080            # Port usage (if using test servers)

# Check recent test artifacts
ls -la test-artifacts/
ls -la screenshots/

# View recent test logs
tail -n 50 test-artifacts/test-report.md
```

## Browser Automation Issues

### Issue: Chrome Binary Not Found

**Symptoms:**
```
exec: "google-chrome-stable": executable file not found in $PATH
context deadline exceeded
chrome_launcher ERROR: Failed to launch chrome!
```

**Root Causes:**
- Chrome/Chromium not installed
- Chrome binary not in PATH
- Wrong binary name for OS

**Solutions:**

1. **Install Chrome (Ubuntu/Debian):**
   ```bash
   wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add -
   sudo sh -c 'echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" >> /etc/apt/sources.list.d/google-chrome.list'
   sudo apt update
   sudo apt install -y google-chrome-stable
   ```

2. **Install Chrome (macOS):**
   ```bash
   brew install --cask google-chrome
   ```

3. **Set explicit Chrome path:**
   ```bash
   # Find Chrome installation
   which google-chrome-stable google-chrome chromium-browser chromium
   
   # Set environment variable
   export CHROME_BIN=/usr/bin/google-chrome-stable
   # or
   export CHROME_BIN=/opt/google/chrome/chrome
   # or (macOS)
   export CHROME_BIN="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
   ```

4. **Auto-detect Chrome binary:**
   ```bash
   # Add to your shell profile
   detect_chrome() {
       local chrome_paths=(
           "/usr/bin/google-chrome-stable"
           "/usr/bin/google-chrome"
           "/usr/bin/chromium-browser"
           "/usr/bin/chromium"
           "/snap/bin/chromium"
           "/opt/google/chrome/chrome"
           "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
           "C:/Program Files/Google/Chrome/Application/chrome.exe"
           "C:/Program Files (x86)/Google/Chrome/Application/chrome.exe"
       )
       
       for path in "${chrome_paths[@]}"; do
           if [[ -x "$path" ]]; then
               export CHROME_BIN="$path"
               echo "Found Chrome at: $CHROME_BIN"
               return 0
           fi
       done
       
       echo "Chrome not found in standard locations"
       return 1
   }
   
   detect_chrome
   ```

### Issue: Chrome Crashes or Hangs

**Symptoms:**
```
chrome crashed: exit status 1
context deadline exceeded
Test hangs indefinitely
```

**Diagnosis Commands:**
```bash
# Check Chrome process status
ps aux | grep chrome

# Test Chrome headless mode directly
google-chrome-stable --headless --dump-dom https://example.com

# Check system limits
ulimit -a

# Check available memory
free -h
```

**Solutions:**

1. **Add CI-specific Chrome flags:**
   ```go
   // In test helper
   opts := append(chromedp.DefaultExecAllocatorOptions[:],
       chromedp.NoSandbox,                    // Required for CI
       chromedp.DisableGPU,                   // Disable GPU acceleration
       chromedp.DisableDevShmUsage,          // Disable /dev/shm usage
       chromedp.DisableSoftwareRasterizer,   // Disable software rasterizer
       chromedp.NoFirstRun,                  // Skip first run experience
       chromedp.Flag("disable-background-timer-throttling", true),
       chromedp.Flag("disable-backgrounding-occluded-windows", true),
       chromedp.Flag("disable-renderer-backgrounding", true),
   )
   ```

2. **Use virtual display (Linux):**
   ```bash
   # Install Xvfb
   sudo apt-get install xvfb
   
   # Start virtual display
   export DISPLAY=:99
   Xvfb :99 -ac -screen 0 1920x1080x24 &
   
   # Run tests
   go test -v -run "TestE2E" ./...
   ```

3. **Increase system limits:**
   ```bash
   # Increase file descriptor limit
   ulimit -n 4096
   
   # Increase process limit
   ulimit -u 4096
   
   # Check memory limits
   ulimit -v
   ```

### Issue: Screenshot Capture Fails

**Symptoms:**
```
Failed to capture screenshot: chrome not running
Screenshot directory not created
Empty screenshot files
```

**Solutions:**

1. **Enable screenshots and create directory:**
   ```bash
   export LIVETEMPLATE_E2E_SCREENSHOTS=true
   mkdir -p screenshots
   chmod 755 screenshots
   ```

2. **Test screenshot capture manually:**
   ```go
   // Test script
   ctx, cancel := chromedp.NewContext(context.Background())
   defer cancel()
   
   var buf []byte
   err := chromedp.Run(ctx,
       chromedp.Navigate("https://example.com"),
       chromedp.WaitVisible("body"),
       chromedp.CaptureScreenshot(&buf),
   )
   
   if err == nil {
       os.WriteFile("test-screenshot.png", buf, 0644)
       fmt.Println("✅ Screenshot capture working")
   } else {
       fmt.Printf("❌ Screenshot failed: %v\n", err)
   }
   ```

3. **Check display configuration (Linux):**
   ```bash
   # Verify X11 display
   echo $DISPLAY
   xdpyinfo | head
   
   # Start Xvfb if needed
   Xvfb :99 -ac -screen 0 1920x1080x24 &
   export DISPLAY=:99
   ```

## Fragment Generation Problems

### Issue: No Fragments Generated

**Symptoms:**
```
no fragments generated
empty fragment array returned
fragments array is null
```

**Diagnosis:**

1. **Check template syntax:**
   ```go
   // Validate template compilation
   tmpl, err := template.New("test").Parse(yourTemplateString)
   if err != nil {
       fmt.Printf("Template error: %v\n", err)
   }
   
   // Test template execution
   var buf bytes.Buffer
   err = tmpl.Execute(&buf, yourTestData)
   if err != nil {
       fmt.Printf("Template execution error: %v\n", err)
   }
   fmt.Printf("Template output: %s\n", buf.String())
   ```

2. **Verify fragment annotations:**
   ```bash
   # Search for fragment annotations in template
   grep -n "data-lt-fragment" your_template_file
   
   # Expected output:
   # <div data-lt-fragment="header">
   # <section data-lt-fragment="content">
   ```

3. **Check data changes:**
   ```go
   // Compare old and new data
   oldDataJSON, _ := json.MarshalIndent(oldData, "", "  ")
   newDataJSON, _ := json.MarshalIndent(newData, "", "  ")
   
   fmt.Printf("Old data:\n%s\n", oldDataJSON)
   fmt.Printf("New data:\n%s\n", newDataJSON)
   
   // Check if data actually changed
   if reflect.DeepEqual(oldData, newData) {
       fmt.Println("❌ No data changes detected")
   } else {
       fmt.Println("✅ Data changes detected")
   }
   ```

**Solutions:**

1. **Fix template fragments:**
   ```html
   <!-- Wrong: Missing data-lt-fragment attribute -->
   <div class="header">{{.Title}}</div>
   
   <!-- Correct: With proper fragment annotation -->
   <div data-lt-fragment="header" class="header">{{.Title}}</div>
   ```

2. **Ensure data changes:**
   ```go
   // Ensure meaningful data changes
   oldData := map[string]interface{}{
       "Title": "Old Title",
       "Count": 1,
   }
   
   newData := map[string]interface{}{
       "Title": "New Title",  // Changed
       "Count": 2,           // Changed
   }
   ```

3. **Debug fragment extraction:**
   ```go
   // Enable debug logging
   fragments, err := page.RenderFragments(ctx, newData)
   
   fmt.Printf("Generated %d fragments\n", len(fragments))
   for i, fragment := range fragments {
       fmt.Printf("Fragment %d:\n", i)
       fmt.Printf("  ID: %s\n", fragment.ID)
       fmt.Printf("  Strategy: %s\n", fragment.Strategy)
       fmt.Printf("  Data: %+v\n", fragment.Data)
   }
   ```

### Issue: Wrong Strategy Selected

**Symptoms:**
```
Expected static_dynamic strategy, got replacement
Unexpected markers strategy for text-only changes
Fragment strategy doesn't match change pattern
```

**Diagnosis:**

1. **Analyze HTML diff:**
   ```go
   // Compare rendered HTML
   oldHTML, _ := page.RenderWithData(oldData)
   newHTML, _ := page.RenderWithData(newData)
   
   fmt.Printf("Old HTML:\n%s\n", oldHTML)
   fmt.Printf("New HTML:\n%s\n", newHTML)
   
   // Identify change type
   hasTextChanges := detectTextChanges(oldHTML, newHTML)
   hasAttributeChanges := detectAttributeChanges(oldHTML, newHTML)
   hasStructuralChanges := detectStructuralChanges(oldHTML, newHTML)
   
   fmt.Printf("Change analysis:\n")
   fmt.Printf("  Text changes: %t\n", hasTextChanges)
   fmt.Printf("  Attribute changes: %t\n", hasAttributeChanges)  
   fmt.Printf("  Structural changes: %t\n", hasStructuralChanges)
   ```

2. **Expected strategy mapping:**
   ```
   Text-only changes → static_dynamic strategy
   Attribute changes → markers strategy
   Structural changes → granular strategy
   Complex mixed changes → replacement strategy
   ```

**Solutions:**

1. **Adjust test data for target strategy:**
   ```go
   // For static_dynamic strategy (text-only changes)
   updateData := map[string]interface{}{
       "Title": "Updated Title",    // Text change
       "Content": "New content",    // Text change
       "CSSClass": "unchanged",     // No attribute change
       "Items": []string{"A", "B"}, // No structural change
   }
   
   // For markers strategy (attribute changes)
   updateData := map[string]interface{}{
       "Title": "Same Title",           // No text change
       "Content": "Same content",       // No text change
       "CSSClass": "new-class",         // Attribute change
       "DataState": "active",           // Attribute change
   }
   ```

2. **Verify strategy selection logic:**
   ```go
   // Custom strategy validation
   func validateExpectedStrategy(t *testing.T, fragments []Fragment, expected string) {
       found := false
       for _, fragment := range fragments {
           if fragment.Strategy == expected {
               found = true
               break
           }
       }
       
       if !found {
           availableStrategies := make([]string, len(fragments))
           for i, f := range fragments {
               availableStrategies[i] = f.Strategy
           }
           t.Errorf("Expected strategy %s not found. Available: %v", 
               expected, availableStrategies)
       }
   }
   ```

## Performance Issues

### Issue: Tests Running Slowly

**Symptoms:**
```
Test execution time > 5 minutes
Individual fragment generation > 100ms
Browser actions timing out
```

**Diagnosis:**

1. **Profile test execution:**
   ```bash
   # Run with CPU profiling
   go test -v -cpuprofile=cpu.prof -memprofile=mem.prof -run "TestE2E"
   
   # Analyze profiles
   go tool pprof cpu.prof
   go tool pprof mem.prof
   ```

2. **Check system resources:**
   ```bash
   # Monitor during test execution
   htop           # CPU and memory usage
   iotop          # I/O usage
   vmstat 1       # System stats
   ```

3. **Measure test phases:**
   ```go
   // Add timing measurements
   start := time.Now()
   
   // Browser startup
   ctx, cancel := helper.CreateBrowserContext()
   defer cancel()
   browserStartup := time.Since(start)
   
   // Page navigation
   start = time.Now()
   err := chromedp.Run(ctx, chromedp.Navigate(server.URL))
   navigationTime := time.Since(start)
   
   // Fragment generation
   start = time.Now()
   fragments, err := page.RenderFragments(ctx, updateData)
   fragmentTime := time.Since(start)
   
   fmt.Printf("Timing breakdown:\n")
   fmt.Printf("  Browser startup: %v\n", browserStartup)
   fmt.Printf("  Navigation: %v\n", navigationTime)
   fmt.Printf("  Fragment generation: %v\n", fragmentTime)
   ```

**Solutions:**

1. **Optimize Chrome configuration:**
   ```go
   // Performance-optimized Chrome flags
   opts := []chromedp.ExecAllocatorOption{
       chromedp.NoSandbox,
       chromedp.DisableGPU,
       chromedp.Headless,
       chromedp.Flag("disable-background-timer-throttling", true),
       chromedp.Flag("disable-backgrounding-occluded-windows", true),
       chromedp.Flag("disable-renderer-backgrounding", true),
       chromedp.Flag("disable-extensions", true),
       chromedp.Flag("disable-plugins", true),
       chromedp.Flag("disable-images", true), // Skip image loading
       chromedp.Flag("disable-javascript", false), // Keep JS for testing
       chromedp.WindowSize(1280, 720), // Smaller window
   }
   ```

2. **Reduce test data size:**
   ```go
   // Use smaller datasets for performance tests
   smallDataset := generateTestData(10)   // Instead of 1000
   mediumDataset := generateTestData(50)  // For integration tests
   largeDataset := generateTestData(500)  // Only for stress tests
   ```

3. **Parallel test execution:**
   ```bash
   # Run tests in parallel
   go test -v -parallel 4 -run "TestE2E"
   
   # Split test groups
   go test -v -run "TestE2EInfrastructure" &
   go test -v -run "TestE2EBrowser" &
   wait
   ```

### Issue: Memory Usage Too High

**Symptoms:**
```
Test process using >4GB RAM
Out of memory errors
System becomes unresponsive during tests
```

**Solutions:**

1. **Monitor memory usage:**
   ```go
   // Add memory monitoring to tests
   func (h *E2ETestHelper) MonitorMemory(t *testing.T) {
       var m runtime.MemStats
       runtime.ReadMemStats(&m)
       
       t.Logf("Memory usage:")
       t.Logf("  Alloc: %d KB", m.Alloc/1024)
       t.Logf("  Sys: %d KB", m.Sys/1024)
       t.Logf("  NumGC: %d", m.NumGC)
       
       if m.Alloc > 100*1024*1024 { // 100MB
           t.Logf("⚠️ High memory usage detected")
       }
   }
   ```

2. **Force garbage collection:**
   ```go
   // Add cleanup between test iterations
   if i%100 == 0 {
       runtime.GC()
       runtime.ReadMemStats(&mem)
       t.Logf("GC performed at iteration %d, mem: %d KB", i, mem.Alloc/1024)
   }
   ```

3. **Limit concurrent operations:**
   ```go
   // Use semaphore to limit concurrency
   semaphore := make(chan struct{}, 10) // Max 10 concurrent operations
   
   for _, testCase := range testCases {
       semaphore <- struct{}{}
       go func(tc TestCase) {
           defer func() { <-semaphore }()
           // Test execution
       }(testCase)
   }
   ```

## Template and Data Problems

### Issue: Template Parsing Errors

**Symptoms:**
```
template: parse error at line X
unexpected token in template
template execution failed
```

**Solutions:**

1. **Validate template syntax:**
   ```bash
   # Use Go template validator
   go run -c '
   package main
   import (
       "html/template"
       "log"
   )
   func main() {
       tmplStr := `your template here`
       _, err := template.New("test").Parse(tmplStr)
       if err != nil {
           log.Fatalf("Template error: %v", err)
       }
       log.Println("✅ Template syntax valid")
   }
   '
   ```

2. **Common template fixes:**
   ```html
   <!-- Wrong: Missing closing tags -->
   <div data-lt-fragment="header">
   <h1>{{.Title}}</h1>
   
   <!-- Correct: Properly closed -->
   <div data-lt-fragment="header">
   <h1>{{.Title}}</h1>
   </div>
   
   <!-- Wrong: Invalid Go template syntax -->
   {{if .Items > 0}}
   
   <!-- Correct: Valid condition -->
   {{if gt (len .Items) 0}}
   
   <!-- Wrong: Unescaped quotes in attributes -->
   <div class="{{.Class}}-"special"">
   
   <!-- Correct: Properly escaped -->
   <div class="{{.Class}}-special">
   ```

3. **Debug template execution:**
   ```go
   // Test template with debug data
   debugData := map[string]interface{}{
       "Title": "Debug Title",
       "Items": []string{"Item1", "Item2"},
       "Class": "debug-class",
   }
   
   var buf bytes.Buffer
   err := tmpl.Execute(&buf, debugData)
   if err != nil {
       t.Fatalf("Template execution failed: %v", err)
   }
   
   t.Logf("Template output:\n%s", buf.String())
   ```

### Issue: Data Type Mismatches

**Symptoms:**
```
interface conversion: interface {} is string, not int
cannot range over non-slice value
nil pointer dereference in template
```

**Solutions:**

1. **Validate data types:**
   ```go
   // Type assertion helpers
   func getString(data map[string]interface{}, key string) string {
       if val, ok := data[key]; ok {
           if str, ok := val.(string); ok {
               return str
           }
       }
       return ""
   }
   
   func getInt(data map[string]interface{}, key string) int {
       if val, ok := data[key]; ok {
           switch v := val.(type) {
           case int:
               return v
           case float64:
               return int(v)
           case string:
               if i, err := strconv.Atoi(v); err == nil {
                   return i
               }
           }
       }
       return 0
   }
   
   func getSlice(data map[string]interface{}, key string) []interface{} {
       if val, ok := data[key]; ok {
           if slice, ok := val.([]interface{}); ok {
               return slice
           }
       }
       return []interface{}{}
   }
   ```

2. **Safe template patterns:**
   ```html
   <!-- Safe nil checking -->
   {{if .Items}}
   {{range .Items}}
   <li>{{.}}</li>
   {{end}}
   {{else}}
   <li>No items</li>
   {{end}}
   
   <!-- Safe type conversion -->
   {{if .Count}}
   Count: {{.Count}}
   {{else}}
   Count: 0
   {{end}}
   
   <!-- Safe string formatting -->
   {{printf "Value: %v" .Value}}
   ```

## CI/CD Integration Issues

### Issue: Tests Pass Locally but Fail in CI

**Symptoms:**
```
Local: ✅ All tests pass
CI: ❌ Tests fail with browser errors
Different behavior in CI environment
```

**Diagnosis:**

1. **Compare environments:**
   ```bash
   # Local environment info
   echo "=== Local Environment ==="
   go version
   google-chrome --version
   echo $DISPLAY
   free -h
   
   # CI environment info (add to CI script)
   echo "=== CI Environment ==="
   go version
   google-chrome --version || chromium-browser --version
   echo $DISPLAY
   free -h
   whoami
   pwd
   env | sort
   ```

2. **Test CI-specific conditions:**
   ```go
   // Detect CI environment
   func isCI() bool {
       return os.Getenv("CI") == "true" || 
              os.Getenv("CONTINUOUS_INTEGRATION") == "true" ||
              os.Getenv("GITHUB_ACTIONS") == "true"
   }
   
   // Adjust test behavior for CI
   if isCI() {
       // Use more conservative timeouts
       testTimeout = 5 * time.Minute
       // Disable interactive features
       enableScreenshots = false
   }
   ```

**Solutions:**

1. **CI-specific Chrome configuration:**
   ```yaml
   # GitHub Actions
   - name: Setup Chrome
     run: |
       sudo apt-get update
       sudo apt-get install -y google-chrome-stable xvfb
       
   - name: Run E2E Tests
     env:
       DISPLAY: :99
       CHROME_BIN: /usr/bin/google-chrome-stable
     run: |
       Xvfb :99 -ac -screen 0 1920x1080x24 &
       sleep 2
       go test -v -timeout=30m -run "TestE2E" ./...
   ```

2. **Reproduce CI environment locally:**
   ```bash
   # Docker simulation of CI
   docker run -it --rm \
     -v $(pwd):/app \
     -w /app \
     -e CI=true \
     -e DISPLAY=:99 \
     ubuntu:latest bash
     
   # Install dependencies
   apt-get update
   apt-get install -y golang-go google-chrome-stable xvfb
   
   # Run tests
   Xvfb :99 -ac -screen 0 1920x1080x24 &
   go test -v ./...
   ```

### Issue: Artifacts Not Preserved

**Symptoms:**
```
Screenshots not uploaded
Test reports missing
Artifacts directory empty
```

**Solutions:**

1. **Verify artifact paths:**
   ```yaml
   # GitHub Actions
   - name: Upload artifacts
     if: always()
     uses: actions/upload-artifact@v3
     with:
       name: e2e-artifacts
       path: |
         test-artifacts/
         screenshots/
         *.log
       retention-days: 30
   ```

2. **Ensure artifacts are created:**
   ```bash
   # Check artifacts before upload
   ls -la test-artifacts/
   ls -la screenshots/
   find . -name "*.log" -type f
   ```

3. **Debug artifact creation:**
   ```go
   // Ensure directories exist
   os.MkdirAll("test-artifacts", 0755)
   os.MkdirAll("screenshots", 0755)
   
   // Write test artifacts explicitly
   artifactData := map[string]interface{}{
       "test_name": "debug-test",
       "timestamp": time.Now(),
       "success": false,
   }
   
   data, _ := json.MarshalIndent(artifactData, "", "  ")
   os.WriteFile("test-artifacts/debug-artifact.json", data, 0644)
   ```

## Debug Tools and Commands

### Essential Debug Commands

```bash
# System diagnostics
uname -a                    # System information
go version                  # Go version
google-chrome --version     # Chrome version
ps aux | grep chrome        # Chrome processes
netstat -tlnp              # Network ports
df -h                      # Disk space
free -h                    # Memory usage

# Test environment
go env                      # Go environment
go list -m all             # Module dependencies
go mod verify              # Verify modules

# Chrome debugging
google-chrome --headless --remote-debugging-port=9222 &
curl http://localhost:9222/json  # Chrome debug info

# Process debugging
strace -p <chrome_pid>      # System call tracing
gdb --pid=<chrome_pid>      # Debugger attach
```

### Debug Test Helper

```go
// DebugHelper provides comprehensive debugging utilities
type DebugHelper struct {
    t      *testing.T
    prefix string
}

func NewDebugHelper(t *testing.T, prefix string) *DebugHelper {
    return &DebugHelper{t: t, prefix: prefix}
}

func (dh *DebugHelper) LogSystemInfo() {
    dh.t.Logf("%s System Information:", dh.prefix)
    dh.t.Logf("  OS: %s", runtime.GOOS)
    dh.t.Logf("  Arch: %s", runtime.GOARCH)
    dh.t.Logf("  CPUs: %d", runtime.NumCPU())
    
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    dh.t.Logf("  Memory: %d KB", m.Alloc/1024)
    
    dh.t.Logf("  Chrome Path: %s", os.Getenv("CHROME_BIN"))
    dh.t.Logf("  Display: %s", os.Getenv("DISPLAY"))
}

func (dh *DebugHelper) LogChromeProcesses() {
    out, err := exec.Command("ps", "aux").Output()
    if err != nil {
        dh.t.Logf("%s Failed to list processes: %v", dh.prefix, err)
        return
    }
    
    lines := strings.Split(string(out), "\n")
    chromeLines := []string{}
    for _, line := range lines {
        if strings.Contains(line, "chrome") || strings.Contains(line, "chromium") {
            chromeLines = append(chromeLines, line)
        }
    }
    
    if len(chromeLines) > 0 {
        dh.t.Logf("%s Chrome processes:", dh.prefix)
        for _, line := range chromeLines {
            dh.t.Logf("  %s", line)
        }
    } else {
        dh.t.Logf("%s No Chrome processes found", dh.prefix)
    }
}

func (dh *DebugHelper) CaptureDebugScreenshot(ctx context.Context, name string) {
    var buf []byte
    err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
    if err != nil {
        dh.t.Logf("%s Failed to capture debug screenshot: %v", dh.prefix, err)
        return
    }
    
    filename := fmt.Sprintf("debug-%s-%s.png", dh.prefix, name)
    err = os.WriteFile(filename, buf, 0644)
    if err != nil {
        dh.t.Logf("%s Failed to save debug screenshot: %v", dh.prefix, err)
    } else {
        dh.t.Logf("%s Debug screenshot saved: %s", dh.prefix, filename)
    }
}
```

### Automated Diagnostic Script

```bash
#!/bin/bash
# e2e-diagnostics.sh - Comprehensive E2E test diagnostics

echo "=== LiveTemplate E2E Diagnostics ==="
echo "Timestamp: $(date)"
echo ""

echo "=== System Information ==="
uname -a
echo "Go version: $(go version)"
echo "Chrome: $(google-chrome --version 2>/dev/null || chromium-browser --version 2>/dev/null || echo 'Not found')"
echo ""

echo "=== Environment Variables ==="
env | grep -E "(CHROME|DISPLAY|LIVETEMPLATE)" | sort
echo ""

echo "=== System Resources ==="
echo "Memory:"
free -h
echo ""
echo "Disk:"
df -h /tmp /var/tmp
echo ""

echo "=== Running Processes ==="
echo "Chrome processes:"
ps aux | grep -E "(chrome|chromium)" | grep -v grep || echo "None"
echo ""
echo "Go test processes:"
ps aux | grep "go test" | grep -v grep || echo "None"
echo ""

echo "=== Network ==="
echo "Listening ports:"
netstat -tlnp 2>/dev/null | grep -E "(LISTEN|Active)" || echo "netstat not available"
echo ""

echo "=== Test Artifacts ==="
echo "Screenshots directory:"
ls -la screenshots/ 2>/dev/null || echo "Directory not found"
echo ""
echo "Test artifacts directory:"
ls -la test-artifacts/ 2>/dev/null || echo "Directory not found"
echo ""

echo "=== Recent Logs ==="
echo "Recent Go test logs:"
find . -name "*.log" -mtime -1 -exec echo "File: {}" \; -exec tail -n 5 {} \; 2>/dev/null || echo "No recent logs"
echo ""

echo "=== Chrome Installation Check ==="
CHROME_PATHS=(
    "/usr/bin/google-chrome-stable"
    "/usr/bin/google-chrome"
    "/usr/bin/chromium-browser"
    "/usr/bin/chromium"
    "/snap/bin/chromium"
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
)

for path in "${CHROME_PATHS[@]}"; do
    if [[ -x "$path" ]]; then
        echo "✅ Found: $path"
        "$path" --version
    else
        echo "❌ Not found: $path"
    fi
done
echo ""

echo "=== Go Module Status ==="
go mod verify
echo "Dependencies:"
go list -m all | head -10
echo ""

echo "=== Quick Browser Test ==="
if command -v google-chrome-stable >/dev/null 2>&1; then
    timeout 10s google-chrome-stable --headless --dump-dom https://example.com >/dev/null 2>&1
    if [[ $? -eq 0 ]]; then
        echo "✅ Basic browser test passed"
    else
        echo "❌ Basic browser test failed"
    fi
else
    echo "⚠️ Chrome not available for testing"
fi

echo ""
echo "=== Diagnostics Complete ==="
```

This comprehensive troubleshooting guide covers the most common issues encountered in LiveTemplate E2E testing, providing systematic diagnosis and resolution approaches for robust test execution across all environments.