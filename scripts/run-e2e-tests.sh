#!/bin/bash

# Enhanced E2E Test Runner for LiveTemplate
# Supports screenshots, performance metrics, and artifact collection

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SCREENSHOTS_DIR="${PROJECT_ROOT}/screenshots"
ARTIFACTS_DIR="${PROJECT_ROOT}/test-artifacts"
RESULTS_FILE="${ARTIFACTS_DIR}/test-results.json"
METRICS_FILE="${ARTIFACTS_DIR}/performance-metrics.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration from environment
CHROME_BIN="${CHROME_BIN:-google-chrome}"
TIMEOUT="${E2E_TIMEOUT:-10m}"
SCREENSHOTS_ENABLED="${LIVETEMPLATE_E2E_SCREENSHOTS:-false}"
ARTIFACTS_ENABLED="${LIVETEMPLATE_E2E_ARTIFACTS:-true}"
PARALLEL_JOBS="${E2E_PARALLEL_JOBS:-1}"
RETRY_ATTEMPTS="${E2E_RETRY_ATTEMPTS:-3}"

# Print colored message
print_msg() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Print section header
print_header() {
    print_msg $BLUE "📋 $1"
    echo "$(printf '=%.0s' {1..50})"
}

# Setup test environment
setup_environment() {
    print_header "Setting up E2E test environment"
    
    # Create directories
    mkdir -p "$SCREENSHOTS_DIR" "$ARTIFACTS_DIR"
    
    # Verify Chrome installation
    if command -v "$CHROME_BIN" > /dev/null 2>&1; then
        CHROME_VERSION=$($CHROME_BIN --version 2>/dev/null || echo "unknown")
        print_msg $GREEN "✅ Chrome found: $CHROME_VERSION"
    else
        print_msg $RED "❌ Chrome not found. Please install Chrome or set CHROME_BIN environment variable"
        exit 1
    fi
    
    # Check Go installation
    if command -v go > /dev/null 2>&1; then
        GO_VERSION=$(go version | awk '{print $3}')
        print_msg $GREEN "✅ Go found: $GO_VERSION"
    else
        print_msg $RED "❌ Go not found"
        exit 1
    fi
    
    # Set environment variables for tests
    export CHROME_BIN
    export LIVETEMPLATE_E2E_SCREENSHOTS="$SCREENSHOTS_ENABLED"
    export LIVETEMPLATE_E2E_ARTIFACTS="$ARTIFACTS_DIR"
    
    print_msg $GREEN "✅ Environment setup complete"
}

# Capture system metrics
capture_system_metrics() {
    local start_time=$1
    local end_time=$2
    
    # Calculate duration
    local duration=$((end_time - start_time))
    
    # Get system information
    local memory_usage=0
    local cpu_cores=1
    local load_average="0.0"
    
    # Linux/macOS specific metrics
    if command -v free > /dev/null 2>&1; then
        memory_usage=$(free -m | grep '^Mem:' | awk '{print $3}' || echo 0)
    elif command -v vm_stat > /dev/null 2>&1; then
        # macOS memory calculation
        memory_usage=$(vm_stat | grep "Pages active:" | awk '{print $3}' | sed 's/\.//' | xargs -I {} echo "scale=0; {} * 4096 / 1024 / 1024" | bc -l 2>/dev/null || echo 0)
    fi
    
    if command -v nproc > /dev/null 2>&1; then
        cpu_cores=$(nproc)
    elif command -v sysctl > /dev/null 2>&1; then
        cpu_cores=$(sysctl -n hw.ncpu 2>/dev/null || echo 1)
    fi
    
    if command -v uptime > /dev/null 2>&1; then
        load_average=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',' || echo "0.0")
    fi
    
    # Calculate artifacts size
    local artifacts_size=0
    if [ -d "$ARTIFACTS_DIR" ]; then
        if command -v du > /dev/null 2>&1; then
            artifacts_size=$(du -sk "$ARTIFACTS_DIR" 2>/dev/null | cut -f1 || echo 0)
        fi
    fi
    
    # Create metrics JSON
    cat > "$METRICS_FILE" << EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)",
  "duration_seconds": $duration,
  "memory_usage_mb": $memory_usage,
  "cpu_cores": $cpu_cores,
  "load_average": "$load_average",
  "artifacts_size_kb": $artifacts_size,
  "chrome_version": "$CHROME_VERSION",
  "go_version": "$GO_VERSION",
  "parallel_jobs": $PARALLEL_JOBS,
  "retry_attempts": $RETRY_ATTEMPTS,
  "screenshots_enabled": "$SCREENSHOTS_ENABLED",
  "artifacts_enabled": "$ARTIFACTS_ENABLED"
}
EOF

    print_msg $GREEN "📊 System metrics captured"
}

# Take screenshot on test failure
capture_failure_screenshot() {
    local test_name=$1
    local screenshot_file="${SCREENSHOTS_DIR}/failure-${test_name}-$(date +%s).png"
    
    if [ "$SCREENSHOTS_ENABLED" = "true" ]; then
        print_msg $YELLOW "📸 Attempting to capture failure screenshot for $test_name"
        
        # This would be implemented by the test itself using chromedp
        # For now, we create a placeholder to indicate the feature is active
        echo "Screenshot capture requested for test: $test_name at $(date)" > "$screenshot_file.log"
        
        print_msg $BLUE "Screenshot log created: $screenshot_file.log"
    fi
}

# Run specific test group with retry logic
run_test_group() {
    local group=$1
    local attempt=1
    local success=false
    
    while [ $attempt -le $RETRY_ATTEMPTS ] && [ "$success" = "false" ]; do
        print_msg $BLUE "🔄 Running test group '$group' (attempt $attempt/$RETRY_ATTEMPTS)"
        
        local test_start_time=$(date +%s)
        local test_pattern=""
        
        # Define test patterns for each group
        case $group in
            "infrastructure")
                test_pattern="TestE2EInfrastructure"
                ;;
            "browser-lifecycle")
                test_pattern="TestE2EBrowserLifecycle"
                ;;
            "performance")
                test_pattern="TestE2EPerformance|BenchmarkE2E"
                ;;
            "error-scenarios")
                test_pattern="TestE2EError"
                ;;
            "concurrent-users")
                test_pattern="TestE2EConcurrent|TestLoadTesting"
                ;;
            "cross-browser")
                test_pattern="TestCrossBrowser"
                ;;
            "all")
                test_pattern="TestE2E"
                ;;
            *)
                print_msg $RED "❌ Unknown test group: $group"
                return 1
                ;;
        esac
        
        # Run the test
        local test_output_file="${ARTIFACTS_DIR}/test-output-${group}-${attempt}.log"
        local test_json_file="${ARTIFACTS_DIR}/test-results-${group}-${attempt}.json"
        
        print_msg $BLUE "Running: go test -v -run '$test_pattern' -timeout $TIMEOUT"
        
        if timeout $TIMEOUT go test -v -run "$test_pattern" ./... -json > "$test_json_file" 2> "$test_output_file"; then
            local test_end_time=$(date +%s)
            local test_duration=$((test_end_time - test_start_time))
            
            print_msg $GREEN "✅ Test group '$group' passed in ${test_duration}s (attempt $attempt)"
            success=true
            
            # Copy successful results to main results file
            cp "$test_json_file" "${ARTIFACTS_DIR}/test-results-${group}.json"
            
        else
            local test_end_time=$(date +%s)
            local test_duration=$((test_end_time - test_start_time))
            
            print_msg $RED "❌ Test group '$group' failed in ${test_duration}s (attempt $attempt)"
            
            # Capture failure information
            capture_failure_screenshot "$group-$attempt"
            
            # Log failure details
            echo "Test group: $group" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            echo "Attempt: $attempt" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            echo "Duration: ${test_duration}s" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            echo "Timestamp: $(date)" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            echo "--- Test Output ---" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            cat "$test_output_file" >> "${ARTIFACTS_DIR}/failures-${group}.log" 2>/dev/null || true
            echo "--- End Test Output ---" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            echo "" >> "${ARTIFACTS_DIR}/failures-${group}.log"
            
            if [ $attempt -lt $RETRY_ATTEMPTS ]; then
                local wait_time=$((attempt * 10))
                print_msg $YELLOW "⏰ Waiting ${wait_time}s before retry..."
                sleep $wait_time
            fi
        fi
        
        attempt=$((attempt + 1))
    done
    
    if [ "$success" = "true" ]; then
        return 0
    else
        print_msg $RED "💥 Test group '$group' failed after $RETRY_ATTEMPTS attempts"
        return 1
    fi
}

# Analyze test results for flakiness
analyze_flakiness() {
    print_header "Analyzing test results for flakiness"
    
    local flakiness_report="${ARTIFACTS_DIR}/flakiness-report.json"
    
    cat > "$flakiness_report" << 'EOF'
{
  "timestamp": "",
  "total_groups": 0,
  "flaky_groups": [],
  "retry_summary": {},
  "recommendations": []
}
EOF

    # Update timestamp
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%S.%3NZ)
    jq --arg ts "$timestamp" '.timestamp = $ts' "$flakiness_report" > "$flakiness_report.tmp" && mv "$flakiness_report.tmp" "$flakiness_report"
    
    # Analyze each test group
    local total_groups=0
    for group in infrastructure browser-lifecycle performance error-scenarios concurrent-users cross-browser; do
        if [ -f "${ARTIFACTS_DIR}/test-results-${group}.json" ]; then
            total_groups=$((total_groups + 1))
            
            # Check for retry patterns in test output
            local retry_count=0
            for attempt in {1..3}; do
                if [ -f "${ARTIFACTS_DIR}/test-results-${group}-${attempt}.json" ]; then
                    retry_count=$((retry_count + 1))
                fi
            done
            
            if [ $retry_count -gt 1 ]; then
                print_msg $YELLOW "⚠️ Test group '$group' required $retry_count attempts"
                
                # Add to flaky groups
                jq --arg group "$group" --argjson retries $retry_count '.flaky_groups += [{"group": $group, "retry_count": $retries}]' "$flakiness_report" > "$flakiness_report.tmp" && mv "$flakiness_report.tmp" "$flakiness_report"
            fi
        fi
    done
    
    # Update total groups count
    jq --argjson total $total_groups '.total_groups = $total' "$flakiness_report" > "$flakiness_report.tmp" && mv "$flakiness_report.tmp" "$flakiness_report"
    
    # Generate recommendations
    local flaky_count=$(jq '.flaky_groups | length' "$flakiness_report")
    if [ "$flaky_count" -gt 0 ]; then
        print_msg $YELLOW "⚠️ Found $flaky_count flaky test groups"
        jq '.recommendations += ["Consider increasing timeouts for flaky tests", "Review test isolation and cleanup", "Check for race conditions in test setup"]' "$flakiness_report" > "$flakiness_report.tmp" && mv "$flakiness_report.tmp" "$flakiness_report"
    else
        print_msg $GREEN "✅ No flaky tests detected"
        jq '.recommendations += ["All tests are stable"]' "$flakiness_report" > "$flakiness_report.tmp" && mv "$flakiness_report.tmp" "$flakiness_report"
    fi
    
    print_msg $GREEN "📋 Flakiness analysis complete"
}

# Generate comprehensive test report
generate_test_report() {
    print_header "Generating test report"
    
    local report_file="${ARTIFACTS_DIR}/test-report.md"
    
    cat > "$report_file" << EOF
# LiveTemplate E2E Test Report

**Run Date:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**Chrome Version:** $CHROME_VERSION  
**Go Version:** $GO_VERSION
**Parallel Jobs:** $PARALLEL_JOBS
**Retry Attempts:** $RETRY_ATTEMPTS

## Test Results Summary

| Test Group | Status | Attempts | Duration | Notes |
|------------|--------|----------|----------|-------|
EOF

    # Process each test group
    local total_passed=0
    local total_failed=0
    
    for group in infrastructure browser-lifecycle performance error-scenarios concurrent-users cross-browser; do
        local status="❌ FAILED"
        local attempts=0
        local duration="unknown"
        local notes=""
        
        if [ -f "${ARTIFACTS_DIR}/test-results-${group}.json" ]; then
            status="✅ PASSED"
            total_passed=$((total_passed + 1))
        else
            total_failed=$((total_failed + 1))
        fi
        
        # Count attempts
        for attempt in {1..3}; do
            if [ -f "${ARTIFACTS_DIR}/test-results-${group}-${attempt}.json" ]; then
                attempts=$((attempts + 1))
            fi
        done
        
        # Add notes for flaky tests
        if [ $attempts -gt 1 ]; then
            notes="Flaky (required $attempts attempts)"
        fi
        
        echo "| $group | $status | $attempts | $duration | $notes |" >> "$report_file"
    done
    
    # Add summary statistics
    cat >> "$report_file" << EOF

## Summary Statistics

- **Total Test Groups:** $((total_passed + total_failed))
- **Passed:** $total_passed
- **Failed:** $total_failed
- **Success Rate:** $(echo "scale=1; $total_passed * 100 / ($total_passed + $total_failed)" | bc -l 2>/dev/null || echo "0.0")%

## Performance Metrics

EOF

    # Add performance metrics if available
    if [ -f "$METRICS_FILE" ]; then
        echo "- **Duration:** $(jq -r '.duration_seconds' "$METRICS_FILE")s" >> "$report_file"
        echo "- **Memory Usage:** $(jq -r '.memory_usage_mb' "$METRICS_FILE")MB" >> "$report_file"
        echo "- **CPU Cores:** $(jq -r '.cpu_cores' "$METRICS_FILE")" >> "$report_file"
        echo "- **Load Average:** $(jq -r '.load_average' "$METRICS_FILE")" >> "$report_file"
        echo "- **Artifacts Size:** $(jq -r '.artifacts_size_kb' "$METRICS_FILE")KB" >> "$report_file"
    fi
    
    # Add artifacts section
    cat >> "$report_file" << EOF

## Available Artifacts

- Test results JSON files
- Performance metrics
- Failure logs
- Screenshots (if enabled)
- Flakiness report

EOF

    # Add flakiness section if report exists
    if [ -f "${ARTIFACTS_DIR}/flakiness-report.json" ]; then
        local flaky_count=$(jq '.flaky_groups | length' "${ARTIFACTS_DIR}/flakiness-report.json")
        if [ "$flaky_count" -gt 0 ]; then
            echo "## Flaky Tests Detected" >> "$report_file"
            echo "" >> "$report_file"
            jq -r '.flaky_groups[] | "- **\(.group)**: \(.retry_count) attempts required"' "${ARTIFACTS_DIR}/flakiness-report.json" >> "$report_file"
            echo "" >> "$report_file"
        fi
    fi
    
    echo "---" >> "$report_file"
    echo "*Report generated by LiveTemplate E2E test runner*" >> "$report_file"
    
    print_msg $GREEN "📄 Test report generated: $report_file"
}

# Cleanup function
cleanup() {
    print_msg $BLUE "🧹 Cleaning up..."
    
    # Kill any remaining Chrome processes
    pkill -f "chrome.*headless" 2>/dev/null || true
    pkill -f "chromium.*headless" 2>/dev/null || true
    
    # Remove temporary files older than 1 hour
    find /tmp -name "*livetemplate*" -mmin +60 -delete 2>/dev/null || true
}

# Main execution function
main() {
    local test_groups="${1:-all}"
    local start_time=$(date +%s)
    
    # Setup signal handlers
    trap cleanup EXIT
    
    print_header "LiveTemplate E2E Test Runner"
    echo "Test Groups: $test_groups"
    echo "Chrome Binary: $CHROME_BIN"
    echo "Timeout: $TIMEOUT"
    echo "Screenshots: $SCREENSHOTS_ENABLED"
    echo "Artifacts: $ARTIFACTS_ENABLED"
    echo "Parallel Jobs: $PARALLEL_JOBS"
    echo "Retry Attempts: $RETRY_ATTEMPTS"
    echo ""
    
    # Setup environment
    setup_environment
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    local overall_success=true
    
    if [ "$test_groups" = "all" ]; then
        # Run all test groups
        for group in infrastructure browser-lifecycle performance error-scenarios concurrent-users cross-browser; do
            if ! run_test_group "$group"; then
                overall_success=false
            fi
        done
    else
        # Run specific test group
        if ! run_test_group "$test_groups"; then
            overall_success=false
        fi
    fi
    
    local end_time=$(date +%s)
    
    # Capture final metrics
    capture_system_metrics $start_time $end_time
    
    # Analyze flakiness
    analyze_flakiness
    
    # Generate report
    generate_test_report
    
    # Print final summary
    print_header "Test Execution Summary"
    
    if [ "$overall_success" = "true" ]; then
        print_msg $GREEN "✅ All tests passed successfully!"
        print_msg $GREEN "📁 Artifacts available in: $ARTIFACTS_DIR"
        
        if [ "$SCREENSHOTS_ENABLED" = "true" ] && [ -d "$SCREENSHOTS_DIR" ] && [ "$(ls -A $SCREENSHOTS_DIR 2>/dev/null)" ]; then
            print_msg $BLUE "📸 Screenshots available in: $SCREENSHOTS_DIR"
        fi
        
        exit 0
    else
        print_msg $RED "❌ Some tests failed"
        print_msg $YELLOW "📁 Debug artifacts available in: $ARTIFACTS_DIR"
        print_msg $YELLOW "📋 Check test-report.md for detailed results"
        
        exit 1
    fi
}

# Script usage
usage() {
    echo "Usage: $0 [test-group]"
    echo ""
    echo "Available test groups:"
    echo "  all                - Run all test groups (default)"
    echo "  infrastructure     - Infrastructure setup tests"
    echo "  browser-lifecycle  - Browser lifecycle tests"
    echo "  performance        - Performance benchmarks"
    echo "  error-scenarios    - Error handling tests"
    echo "  concurrent-users   - Concurrent user tests"
    echo "  cross-browser      - Cross-browser compatibility"
    echo ""
    echo "Environment variables:"
    echo "  CHROME_BIN                     - Path to Chrome binary"
    echo "  E2E_TIMEOUT                    - Test timeout (default: 10m)"
    echo "  LIVETEMPLATE_E2E_SCREENSHOTS   - Enable screenshots (default: false)"
    echo "  LIVETEMPLATE_E2E_ARTIFACTS     - Artifacts directory"
    echo "  E2E_PARALLEL_JOBS              - Number of parallel jobs (default: 1)"
    echo "  E2E_RETRY_ATTEMPTS             - Number of retry attempts (default: 3)"
}

# Handle command line arguments
case "${1:-}" in
    -h|--help)
        usage
        exit 0
        ;;
    "")
        main "all"
        ;;
    *)
        main "$1"
        ;;
esac