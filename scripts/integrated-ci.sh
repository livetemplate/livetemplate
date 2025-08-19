#!/bin/bash

# Integrated CI Script for LiveTemplate
# Combines existing Go test pipeline with enhanced E2E testing capabilities

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/.github/e2e-config.yml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Default configuration
DEFAULT_CONFIG='{
  "execution": {
    "timeout_minutes": 30,
    "retry_attempts": 3,
    "parallel_execution": true
  },
  "test_groups": {
    "infrastructure": {"required": true},
    "browser-lifecycle": {"required": true},
    "performance": {"required": false},
    "error-scenarios": {"required": false},
    "concurrent-users": {"required": false},
    "cross-browser": {"required": false}
  },
  "screenshots": {"enabled": true},
  "artifacts": {"enabled": true, "retention_days": 30},
  "performance": {"enabled": true},
  "flakiness": {"enabled": true, "auto_retry": true}
}'

# Environment detection
CI_ENVIRONMENT="local"
if [ "$CI" = "true" ]; then
    CI_ENVIRONMENT="ci"
fi

if [ "$GITHUB_EVENT_NAME" = "pull_request" ]; then
    CI_ENVIRONMENT="pr"
fi

# Print colored message
print_msg() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Print section header
print_header() {
    print_msg $PURPLE "
╔══════════════════════════════════════════════════════════════════════════════╗
║ $1
╚══════════════════════════════════════════════════════════════════════════════╝"
}

# Load configuration from YAML (simplified parsing)
load_config() {
    local config_key=$1
    local default_value=$2
    
    if [ -f "$CONFIG_FILE" ]; then
        # Simple grep-based YAML parsing (for basic values)
        local value=$(grep "^[[:space:]]*${config_key}:" "$CONFIG_FILE" | sed 's/.*: *\(.*\)/\1/' | tr -d '"' | head -1)
        echo "${value:-$default_value}"
    else
        echo "$default_value"
    fi
}

# Check if test group is required
is_test_group_required() {
    local group=$1
    
    case "$CI_ENVIRONMENT" in
        "ci"|"pr")
            case "$group" in
                "infrastructure"|"browser-lifecycle")
                    echo "true"
                    ;;
                *)
                    echo "false"
                    ;;
            esac
            ;;
        *)
            echo "false"  # In local environment, no tests are strictly required
            ;;
    esac
}

# Setup integrated test environment
setup_integrated_environment() {
    print_header "Setting up Integrated CI Environment"
    
    print_msg $BLUE "🌍 Environment: $CI_ENVIRONMENT"
    print_msg $BLUE "📁 Project Root: $PROJECT_ROOT"
    print_msg $BLUE "⚙️  Config File: $CONFIG_FILE"
    
    # Create necessary directories
    mkdir -p "$PROJECT_ROOT/test-artifacts" "$PROJECT_ROOT/screenshots" "$PROJECT_ROOT/reports"
    
    # Set environment variables for consistent behavior
    export LIVETEMPLATE_CI_ENVIRONMENT="$CI_ENVIRONMENT"
    export LIVETEMPLATE_E2E_SCREENSHOTS="$(load_config 'screenshots.enabled' 'true')"
    export LIVETEMPLATE_E2E_ARTIFACTS="$PROJECT_ROOT/test-artifacts"
    export E2E_RETRY_ATTEMPTS="$(load_config 'execution.retry_attempts' '3')"
    
    # Detect Chrome binary
    if [ -z "$CHROME_BIN" ]; then
        if command -v google-chrome-stable > /dev/null 2>&1; then
            export CHROME_BIN="google-chrome-stable"
        elif command -v google-chrome > /dev/null 2>&1; then
            export CHROME_BIN="google-chrome"
        elif command -v chromium-browser > /dev/null 2>&1; then
            export CHROME_BIN="chromium-browser"
        elif command -v chromium > /dev/null 2>&1; then
            export CHROME_BIN="chromium"
        else
            print_msg $YELLOW "⚠️  Chrome binary not found - some E2E tests may fail"
        fi
    fi
    
    if [ -n "$CHROME_BIN" ]; then
        print_msg $GREEN "✅ Chrome binary: $CHROME_BIN"
        if command -v "$CHROME_BIN" > /dev/null 2>&1; then
            "$CHROME_BIN" --version || print_msg $YELLOW "⚠️  Could not get Chrome version"
        fi
    fi
    
    print_msg $GREEN "✅ Integrated environment setup complete"
}

# Run the original CI validation
run_original_ci_validation() {
    print_header "Running Original CI Validation"
    
    local validation_script="$SCRIPT_DIR/validate-ci.sh"
    
    if [ -f "$validation_script" ]; then
        print_msg $BLUE "🔧 Running existing CI validation..."
        chmod +x "$validation_script"
        
        # Run with timeout
        if timeout 15m "$validation_script"; then
            print_msg $GREEN "✅ Original CI validation passed"
            return 0
        else
            print_msg $RED "❌ Original CI validation failed"
            return 1
        fi
    else
        print_msg $YELLOW "⚠️  Original CI validation script not found: $validation_script"
        print_msg $BLUE "📋 Running basic Go tests instead..."
        
        # Fallback to basic tests
        if go test -short -v ./...; then
            print_msg $GREEN "✅ Basic Go tests passed"
            return 0
        else
            print_msg $RED "❌ Basic Go tests failed"
            return 1
        fi
    fi
}

# Run enhanced E2E tests
run_enhanced_e2e_tests() {
    print_header "Running Enhanced E2E Tests"
    
    local e2e_script="$SCRIPT_DIR/run-e2e-tests.sh"
    local overall_success=true
    local required_failures=0
    
    # Test groups to run
    local test_groups=("infrastructure" "browser-lifecycle" "performance" "error-scenarios" "concurrent-users" "cross-browser")
    
    if [ "$CI_ENVIRONMENT" = "pr" ]; then
        # Reduced test set for pull requests
        test_groups=("infrastructure" "browser-lifecycle" "error-scenarios")
    fi
    
    for group in "${test_groups[@]}"; do
        local is_required=$(is_test_group_required "$group")
        local group_start_time=$(date +%s)
        
        print_msg $BLUE "🧪 Running test group: $group (required: $is_required)"
        
        if [ -f "$e2e_script" ]; then
            chmod +x "$e2e_script"
            
            if "$e2e_script" "$group"; then
                local duration=$(($(date +%s) - group_start_time))
                print_msg $GREEN "✅ Test group '$group' passed (${duration}s)"
            else
                local duration=$(($(date +%s) - group_start_time))
                print_msg $RED "❌ Test group '$group' failed (${duration}s)"
                
                if [ "$is_required" = "true" ]; then
                    required_failures=$((required_failures + 1))
                    overall_success=false
                else
                    print_msg $YELLOW "⚠️  Optional test group '$group' failed - continuing..."
                fi
            fi
        else
            # Fallback to direct Go test execution
            print_msg $BLUE "📋 Running Go tests for pattern: $group"
            
            local test_pattern=""
            case "$group" in
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
            esac
            
            if [ -n "$test_pattern" ]; then
                if timeout 20m go test -v -run "$test_pattern" ./...; then
                    print_msg $GREEN "✅ Test group '$group' passed (direct execution)"
                else
                    print_msg $RED "❌ Test group '$group' failed (direct execution)"
                    
                    if [ "$is_required" = "true" ]; then
                        required_failures=$((required_failures + 1))
                        overall_success=false
                    fi
                fi
            else
                print_msg $YELLOW "⚠️  No test pattern defined for group: $group"
            fi
        fi
        
        # Brief pause between test groups
        sleep 2
    done
    
    # Summary
    if [ "$overall_success" = "true" ]; then
        print_msg $GREEN "✅ All required E2E tests passed"
        return 0
    else
        print_msg $RED "❌ $required_failures required test group(s) failed"
        return 1
    fi
}

# Generate comprehensive test report
generate_integrated_report() {
    print_header "Generating Integrated Test Report"
    
    local report_file="$PROJECT_ROOT/reports/integrated-ci-report.md"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    
    cat > "$report_file" << EOF
# LiveTemplate Integrated CI Report

**Generated:** $timestamp  
**Environment:** $CI_ENVIRONMENT  
**Chrome Binary:** ${CHROME_BIN:-"Not detected"}  
**Retry Attempts:** $E2E_RETRY_ATTEMPTS  
**Screenshots:** $LIVETEMPLATE_E2E_SCREENSHOTS  

## Test Execution Summary

| Phase | Status | Duration | Notes |
|-------|---------|----------|-------|
| Environment Setup | ✅ | - | Chrome detected and configured |
| Original CI Validation | $([[ -f "$PROJECT_ROOT/test-artifacts/ci-validation-success" ]] && echo "✅" || echo "❌") | - | Go tests, formatting, linting |
| Enhanced E2E Tests | $([[ -f "$PROJECT_ROOT/test-artifacts/e2e-success" ]] && echo "✅" || echo "❌") | - | Browser automation tests |
| Report Generation | ✅ | - | This report |

## Test Artifacts

EOF

    # List available artifacts
    if [ -d "$PROJECT_ROOT/test-artifacts" ]; then
        local artifact_count=$(find "$PROJECT_ROOT/test-artifacts" -type f | wc -l)
        echo "- **Total Artifacts:** $artifact_count files" >> "$report_file"
        
        local screenshot_count=$(find "$PROJECT_ROOT/screenshots" -name "*.png" 2>/dev/null | wc -l)
        echo "- **Screenshots:** $screenshot_count images" >> "$report_file"
        
        local log_count=$(find "$PROJECT_ROOT/test-artifacts" -name "*.log" 2>/dev/null | wc -l)
        echo "- **Log Files:** $log_count logs" >> "$report_file"
        
        local json_count=$(find "$PROJECT_ROOT/test-artifacts" -name "*.json" 2>/dev/null | wc -l)
        echo "- **JSON Reports:** $json_count reports" >> "$report_file"
    fi
    
    # Add performance metrics if available
    if [ -f "$PROJECT_ROOT/test-artifacts/performance-trends.json" ]; then
        cat >> "$report_file" << 'EOF'

## Performance Trends

EOF
        # Extract key metrics from JSON (simplified)
        if command -v jq > /dev/null 2>&1; then
            local avg_duration=$(jq '.test_groups[].duration_seconds' "$PROJECT_ROOT/test-artifacts/performance-trends.json" 2>/dev/null | awk '{sum+=$1} END {print sum/NR "s"}' || echo "N/A")
            echo "- **Average Test Duration:** $avg_duration" >> "$report_file"
        fi
    fi
    
    cat >> "$report_file" << EOF

## Environment Details

- **Operating System:** $(uname -s)
- **Architecture:** $(uname -m)
- **Go Version:** $(go version | awk '{print $3}')
- **Chrome Version:** $($CHROME_BIN --version 2>/dev/null || echo "Not available")
- **Node.js Version:** $(node --version 2>/dev/null || echo "Not installed")

## Configuration

- **Environment:** $CI_ENVIRONMENT
- **Screenshots Enabled:** $LIVETEMPLATE_E2E_SCREENSHOTS
- **Retry Attempts:** $E2E_RETRY_ATTEMPTS
- **Artifacts Directory:** $LIVETEMPLATE_E2E_ARTIFACTS

---
*Generated by LiveTemplate Integrated CI Pipeline*
EOF

    print_msg $GREEN "📄 Integrated report generated: $report_file"
    
    # Display summary
    if [ -f "$report_file" ]; then
        print_msg $BLUE "📋 Report Summary:"
        head -20 "$report_file" | tail -n +2
    fi
}

# Cleanup function
cleanup_integrated_ci() {
    print_msg $BLUE "🧹 Cleaning up integrated CI environment..."
    
    # Kill any remaining Chrome processes
    pkill -f "chrome.*headless" 2>/dev/null || true
    pkill -f "chromium.*headless" 2>/dev/null || true
    
    # Compress large artifacts
    if [ -d "$PROJECT_ROOT/test-artifacts" ]; then
        find "$PROJECT_ROOT/test-artifacts" -name "*.log" -size +1M -exec gzip {} \; 2>/dev/null || true
    fi
    
    # Clean up temporary files
    find /tmp -name "*livetemplate*" -mmin +60 -delete 2>/dev/null || true
    
    print_msg $GREEN "✅ Cleanup completed"
}

# Main execution function
main() {
    local start_time=$(date +%s)
    local overall_success=true
    
    # Setup signal handlers
    trap cleanup_integrated_ci EXIT
    
    print_header "LiveTemplate Integrated CI Pipeline"
    print_msg $BLUE "🚀 Starting integrated CI pipeline in $CI_ENVIRONMENT environment"
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Phase 1: Setup
    if ! setup_integrated_environment; then
        print_msg $RED "❌ Environment setup failed"
        exit 1
    fi
    
    # Phase 2: Original CI validation
    if run_original_ci_validation; then
        touch "$PROJECT_ROOT/test-artifacts/ci-validation-success"
    else
        overall_success=false
        print_msg $RED "❌ Original CI validation failed"
        
        # Continue with E2E tests even if original CI fails (for debugging)
        print_msg $YELLOW "⚠️  Continuing with E2E tests for debugging purposes..."
    fi
    
    # Phase 3: Enhanced E2E tests
    if run_enhanced_e2e_tests; then
        touch "$PROJECT_ROOT/test-artifacts/e2e-success"
    else
        overall_success=false
        print_msg $RED "❌ Enhanced E2E tests failed"
    fi
    
    # Phase 4: Report generation
    generate_integrated_report
    
    # Final summary
    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))
    
    print_header "Integrated CI Pipeline Summary"
    
    if [ "$overall_success" = "true" ]; then
        print_msg $GREEN "✅ Integrated CI pipeline completed successfully!"
        print_msg $GREEN "⏱️  Total duration: ${total_duration}s"
        print_msg $GREEN "📁 Artifacts available in: $PROJECT_ROOT/test-artifacts"
        print_msg $GREEN "📊 Report available at: $PROJECT_ROOT/reports/integrated-ci-report.md"
        
        exit 0
    else
        print_msg $RED "❌ Integrated CI pipeline failed"
        print_msg $YELLOW "⏱️  Total duration: ${total_duration}s"
        print_msg $YELLOW "📁 Debug artifacts available in: $PROJECT_ROOT/test-artifacts"
        print_msg $YELLOW "📊 Debug report available at: $PROJECT_ROOT/reports/integrated-ci-report.md"
        
        exit 1
    fi
}

# Usage information
usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Runs the integrated LiveTemplate CI pipeline including:"
    echo "  - Original CI validation (tests, formatting, linting)"
    echo "  - Enhanced E2E tests with browser automation"
    echo "  - Comprehensive reporting and artifact collection"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --verbose  Enable verbose output"
    echo ""
    echo "Environment Variables:"
    echo "  CI                              Set to 'true' to enable CI mode"
    echo "  CHROME_BIN                      Path to Chrome binary"
    echo "  LIVETEMPLATE_E2E_SCREENSHOTS    Enable screenshots (true/false)"
    echo "  E2E_RETRY_ATTEMPTS              Number of retry attempts (default: 3)"
    echo ""
    echo "Examples:"
    echo "  $0                              Run full integrated pipeline"
    echo "  CI=true $0                      Run in CI mode"
    echo "  CHROME_BIN=/usr/bin/chromium $0 Run with specific Chrome binary"
}

# Handle command line arguments
case "${1:-}" in
    -h|--help)
        usage
        exit 0
        ;;
    -v|--verbose)
        set -x
        main
        ;;
    "")
        main
        ;;
    *)
        echo "Unknown option: $1"
        usage
        exit 1
        ;;
esac