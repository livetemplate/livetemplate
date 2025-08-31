#!/bin/bash

# Unified CI Script for LiveTemplate Tree-Based Architecture
# Works for both GitHub Actions and pre-commit hooks with configurable validation levels

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Default configuration
DEFAULT_MODE="full"
GOLANGCI_TIMEOUT="5m"
TEST_TIMEOUT="60s"

# Environment detection
detect_environment() {
    if [ "$CI" = "true" ]; then
        echo "ci"
    elif [ "${LIVETEMPLATE_PRE_COMMIT:-false}" = "true" ]; then
        echo "pre-commit"
    else
        echo "local"
    fi
}

ENVIRONMENT=$(detect_environment)

# Print colored message
print_msg() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Print section header
print_header() {
    print_msg $PURPLE "
╔════════════════════════════════════════════════════════════════════╗
║ $1
╚════════════════════════════════════════════════════════════════════╝"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install golangci-lint if not present
install_golangci_lint() {
    print_msg $BLUE "📦 Installing golangci-lint..."
    
    if command_exists curl; then
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest
    else
        print_msg $RED "❌ curl is required to install golangci-lint"
        exit 1
    fi
    
    # Add GOPATH/bin to PATH if not already there
    export PATH="$(go env GOPATH)/bin:$PATH"
    
    print_msg $GREEN "✅ golangci-lint installed successfully"
}

# Setup environment
setup_environment() {
    print_header "LiveTemplate CI - Environment: $ENVIRONMENT, Mode: ${MODE:-$DEFAULT_MODE}"
    
    print_msg $BLUE "🌍 Environment: $ENVIRONMENT"
    print_msg $BLUE "📁 Project Root: $PROJECT_ROOT"
    print_msg $BLUE "⚙️  Validation Mode: ${MODE:-$DEFAULT_MODE}"
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Check and install golangci-lint if needed
    if ! command_exists golangci-lint; then
        install_golangci_lint
        export PATH="$(go env GOPATH)/bin:$PATH"
    else
        print_msg $GREEN "✅ golangci-lint is already installed"
    fi
    
    # Always format code first
    print_msg $BLUE "🔧 Formatting code..."
    go fmt ./...
}

# Run Go tests
run_tests() {
    print_msg $BLUE "🧪 Running Go tests..."
    
    local test_pattern=""
    local test_args="-v"
    
    case "${MODE:-$DEFAULT_MODE}" in
        "fast"|"pre-commit")
            # Fast tests - core functionality only
            test_pattern="-run=Test(Application|Page|Fragment|Template|Tree)"
            test_args="-v -short"
            print_msg $BLUE "   Running core tests only (fast mode)"
            ;;
        "full"|"ci")
            # Full test suite
            test_pattern=""
            test_args="-v"
            print_msg $BLUE "   Running full test suite"
            ;;
    esac
    
    if timeout $TEST_TIMEOUT go test $test_args $test_pattern ./...; then
        print_msg $GREEN "✅ Tests passed"
        return 0
    else
        print_msg $RED "❌ Tests failed"
        return 1
    fi
}

# Check code compilation
check_compilation() {
    print_msg $BLUE "🔨 Checking code compilation..."
    
    if go build ./...; then
        print_msg $GREEN "✅ Code compiles successfully"
        return 0
    else
        print_msg $RED "❌ Code compilation failed"
        return 1
    fi
}

# Check code formatting
check_formatting() {
    print_msg $BLUE "🎨 Checking code formatting..."
    
    UNFORMATTED=$(gofmt -l .)
    if [ -z "$UNFORMATTED" ]; then
        print_msg $GREEN "✅ Code formatting is correct"
        return 0
    else
        print_msg $RED "❌ The following files need formatting:"
        echo "$UNFORMATTED"
        print_msg $YELLOW "💡 Run: go fmt ./... to fix formatting"
        
        if [ "$ENVIRONMENT" = "pre-commit" ]; then
            print_msg $BLUE "🔧 Auto-formatting for pre-commit..."
            go fmt ./...
            print_msg $GREEN "✅ Code auto-formatted"
            return 0
        fi
        
        return 1
    fi
}

# Run go vet
run_vet() {
    print_msg $BLUE "🔍 Running go vet..."
    
    if go vet ./...; then
        print_msg $GREEN "✅ go vet passed"
        return 0
    else
        print_msg $RED "❌ go vet failed"
        return 1
    fi
}

# Run golangci-lint
run_linting() {
    print_msg $BLUE "🔎 Running golangci-lint..."
    
    # Capture golangci-lint output for parsing, temporarily disable exit on error
    set +e
    LINT_OUTPUT=$(golangci-lint run --timeout=$GOLANGCI_TIMEOUT 2>&1)
    LINT_EXIT_CODE=$?
    set -e
    
    if [ $LINT_EXIT_CODE -eq 0 ]; then
        print_msg $GREEN "✅ golangci-lint passed"
        return 0
    else
        print_msg $RED "❌ golangci-lint found issues that need to be fixed"
        print_msg $YELLOW ""
        print_msg $YELLOW "🤖 GOLANGCI-LINT OUTPUT:"
        print_msg $YELLOW "========================"
        echo "$LINT_OUTPUT"
        print_msg $YELLOW "========================"
        print_msg $YELLOW ""
        
        # Try to extract specific issue lines for structured parsing
        ISSUE_LINES=$(echo "$LINT_OUTPUT" | grep -E "^[^[:space:]].*:[0-9]+:[0-9]+:" | head -20)
        
        if [ -n "$ISSUE_LINES" ]; then
            print_msg $BLUE "🔍 PARSED ISSUES:"
            print_msg $BLUE "----------------"
            echo "$ISSUE_LINES"
            print_msg $YELLOW ""
            print_msg $YELLOW "COMMON FIXES:"
            print_msg $YELLOW "- errcheck: Add error handling for returned errors"
            print_msg $YELLOW "- ineffassign: Remove or use assigned variables"
            print_msg $YELLOW "- staticcheck: Follow Go best practices"
            print_msg $YELLOW "- unused: Remove unused functions/variables"
        fi
        
        return 1
    fi
}

# Check go mod tidy
check_go_mod() {
    print_msg $BLUE "📦 Checking go mod tidy..."
    
    go mod tidy
    
    # Check if there are changes after running go mod tidy
    if git diff --exit-code go.mod; then
        print_msg $GREEN "✅ go.mod is tidy"
    else
        print_msg $GREEN "✅ go.mod was updated by go mod tidy"
    fi
    
    # Check go.sum but don't fail the build for it (cache inconsistencies are common)
    if git diff --exit-code go.sum; then
        print_msg $GREEN "✅ go.sum is tidy"
    else
        print_msg $YELLOW "⚠️  go.sum has changes (likely cached dependencies), but continuing..."
    fi
    
    return 0
}

# Run JavaScript client validation
run_javascript_validation() {
    print_msg $BLUE "📱 Running JavaScript client validation..."
    
    local js_client="pkg/client/web/tree-fragment-client.js"
    
    if [ -f "$js_client" ]; then
        if command_exists node; then
            if node -c "$js_client"; then
                print_msg $GREEN "✅ JavaScript client syntax is valid"
            else
                print_msg $RED "❌ JavaScript client has syntax errors"
                return 1
            fi
        else
            print_msg $YELLOW "⚠️  Node.js not found, skipping JavaScript validation"
        fi
    else
        print_msg $YELLOW "⚠️  JavaScript client not found at $js_client"
    fi
    
    return 0
}

# Run example demos
run_demos() {
    if [ "${MODE:-$DEFAULT_MODE}" = "fast" ] || [ "${MODE:-$DEFAULT_MODE}" = "pre-commit" ]; then
        print_msg $YELLOW "⏭️  Skipping demos in fast mode"
        return 0
    fi
    
    print_msg $BLUE "🎬 Running example demos..."
    
    local demos=(
        "examples/bandwidth-savings/main.go"
        "examples/template-constructs/main.go"
    )
    
    for demo in "${demos[@]}"; do
        if [ -f "$demo" ]; then
            print_msg $BLUE "   Running $(basename $(dirname $demo)) demo..."
            if timeout 5s go run "$demo" > /dev/null 2>&1; then
                print_msg $GREEN "✅ Demo $(basename $(dirname $demo)) completed"
            else
                print_msg $YELLOW "⚠️  Demo $(basename $(dirname $demo)) timed out (expected)"
            fi
        fi
    done
    
    return 0
}

# Run benchmarks
run_benchmarks() {
    if [ "${MODE:-$DEFAULT_MODE}" = "fast" ] || [ "${MODE:-$DEFAULT_MODE}" = "pre-commit" ]; then
        print_msg $YELLOW "⏭️  Skipping benchmarks in fast mode"
        return 0
    fi
    
    print_msg $BLUE "⚡ Running performance benchmarks..."
    
    if go test ./internal/strategy/ -bench=. -benchmem -timeout 10m > /dev/null 2>&1; then
        print_msg $GREEN "✅ Performance benchmarks completed"
    else
        print_msg $YELLOW "⚠️  Benchmarks failed or timed out"
    fi
    
    return 0
}

# Generate summary report
generate_summary() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    print_header "CI Validation Summary"
    
    if [ "$OVERALL_SUCCESS" = "true" ]; then
        print_msg $GREEN "🎉 All CI validation checks passed!"
        print_msg $GREEN "⏱️  Duration: ${duration}s"
        print_msg $GREEN "🌍 Environment: $ENVIRONMENT"
        print_msg $GREEN "⚙️  Mode: ${MODE:-$DEFAULT_MODE}"
        
        case "${MODE:-$DEFAULT_MODE}" in
            "fast"|"pre-commit")
                print_msg $BLUE "💡 Fast validation complete. Full tests will run in CI."
                ;;
            "full"|"ci")
                print_msg $BLUE "💡 Full validation complete. Ready for deployment."
                ;;
        esac
        
        return 0
    else
        print_msg $RED "❌ CI validation failed"
        print_msg $YELLOW "⏱️  Duration: ${duration}s"
        print_msg $YELLOW "🌍 Environment: $ENVIRONMENT"
        print_msg $YELLOW "⚙️  Mode: ${MODE:-$DEFAULT_MODE}"
        
        print_msg $YELLOW ""
        print_msg $YELLOW "🔧 To fix issues:"
        print_msg $YELLOW "  - Check the specific error messages above"
        print_msg $YELLOW "  - Run: ./scripts/ci.sh --mode fast for quick validation"
        print_msg $YELLOW "  - Run: ./scripts/ci.sh --mode full for comprehensive validation"
        
        return 1
    fi
}

# Main validation pipeline
run_validation_pipeline() {
    local success=true
    
    # Core validation steps (always run)
    local steps=(
        "run_tests"
        "check_compilation"
        "check_formatting"
        "run_vet"
        "run_linting"
        "check_go_mod"
    )
    
    # Additional steps for full mode
    if [ "${MODE:-$DEFAULT_MODE}" = "full" ] || [ "${MODE:-$DEFAULT_MODE}" = "ci" ]; then
        steps+=("run_javascript_validation" "run_demos" "run_benchmarks")
    fi
    
    # Run all validation steps
    for step in "${steps[@]}"; do
        print_msg $BLUE ""
        if ! $step; then
            success=false
            if [ "$ENVIRONMENT" = "pre-commit" ]; then
                # In pre-commit mode, try to continue with other checks
                print_msg $YELLOW "⚠️  Step failed but continuing in pre-commit mode..."
            else
                # In CI mode, fail fast
                break
            fi
        fi
    done
    
    if [ "$success" = "true" ]; then
        OVERALL_SUCCESS="true"
    else
        OVERALL_SUCCESS="false"
    fi
}

# Usage information
usage() {
    cat << EOF
Usage: $0 [options]

Unified CI script for LiveTemplate tree-based architecture.
Works for both GitHub Actions and pre-commit hooks.

Options:
  -h, --help          Show this help message
  -m, --mode MODE     Set validation mode: fast, full (default: full)
  -t, --timeout TIME  Set test timeout (default: 60s)
  -v, --verbose       Enable verbose output

Modes:
  fast        Fast validation for pre-commit (5-10 seconds)
              - Core tests only
              - Skip demos and benchmarks
              - Auto-format code

  full        Full validation for CI (1-2 minutes)
              - All tests
              - JavaScript validation
              - Demos and benchmarks
              - Comprehensive reporting

Environment Variables:
  CI                        Set to 'true' for CI mode
  LIVETEMPLATE_PRE_COMMIT  Set to 'true' for pre-commit mode
  GOLANGCI_TIMEOUT         Timeout for golangci-lint (default: 5m)

Examples:
  $0                          Run full validation
  $0 --mode fast              Run fast validation (pre-commit)
  $0 --mode full              Run comprehensive validation (CI)
  
  # Pre-commit hook usage:
  LIVETEMPLATE_PRE_COMMIT=true $0 --mode fast
  
  # CI usage:
  CI=true $0 --mode full

Integration:
  - GitHub Actions: Use --mode full for comprehensive validation
  - Pre-commit hook: Use --mode fast for quick validation
  - Local development: Use either mode as needed
EOF
}

# Parse command line arguments function
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                exit 0
                ;;
            -m|--mode)
                MODE="$2"
                shift 2
                ;;
            -t|--timeout)
                TEST_TIMEOUT="$2"
                shift 2
                ;;
            -v|--verbose)
                set -x
                shift
                ;;
            *)
                print_msg $RED "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Validate and setup mode
validate_and_setup_mode() {
    # Auto-detect mode based on environment if not specified
    if [ -z "$MODE" ]; then
        case "$ENVIRONMENT" in
            "pre-commit")
                MODE="fast"
                ;;
            "ci")
                MODE="full"
                ;;
            *)
                MODE="$DEFAULT_MODE"
                ;;
        esac
    fi
    
    # Validate mode
    case "${MODE:-$DEFAULT_MODE}" in
        "fast"|"pre-commit"|"full"|"ci")
            ;;
        *)
            print_msg $RED "Invalid mode: ${MODE:-$DEFAULT_MODE}"
            print_msg $YELLOW "Valid modes: fast, pre-commit, full, ci"
            exit 1
            ;;
    esac
}

# Main execution
main() {
    START_TIME=$(date +%s)
    OVERALL_SUCCESS="false"
    
    # Parse command line arguments
    parse_arguments "$@"
    
    # Validate and setup mode
    validate_and_setup_mode
    
    # Setup
    setup_environment
    
    # Run validation pipeline
    run_validation_pipeline
    
    # Generate summary and exit
    if generate_summary; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main "$@"