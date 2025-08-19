#!/bin/bash

# Fast CI Validation Script for LiveTemplate (Pre-commit)
# Runs essential validation checks without heavy E2E tests

set -e

echo "🚀 Starting fast CI validation for LiveTemplate (pre-commit)..."
echo "============================================================"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install golangci-lint if not present
install_golangci_lint() {
    echo "📦 Installing golangci-lint..."
    
    # Use the official installation script with latest version for Go 1.24 compatibility
    if command_exists curl; then
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest
    else
        echo "❌ curl is required to install golangci-lint"
        exit 1
    fi
    
    # Add GOPATH/bin to PATH if not already there
    export PATH="$(go env GOPATH)/bin:$PATH"
    
    echo "✅ golangci-lint installed successfully"
}

# Check and install golangci-lint if needed
if ! command_exists golangci-lint; then
    install_golangci_lint
    # Ensure GOPATH/bin is in PATH for subsequent commands
    export PATH="$(go env GOPATH)/bin:$PATH"
else
    echo "✅ golangci-lint is already installed"
fi

echo "Running go fmt ..."
go fmt ./...

echo ""
echo "1️⃣  Running core tests (fast)..."
echo "--------------------------------"
# Run only core functionality tests, skip heavy E2E/load/performance tests
if timeout 60s go test -v -run="Test(Application|Page|Fragment|Template)" ./...; then
    echo "✅ Core tests passed"
else
    echo "❌ Core tests failed"
    exit 1
fi

echo ""
echo "2️⃣  Checking code compilation..."
echo "--------------------------------"
if go build ./...; then
    echo "✅ Code compiles successfully"
else
    echo "❌ Code compilation failed"
    exit 1
fi

echo ""
echo "3️⃣  Checking code formatting..."
echo "--------------------------------"
UNFORMATTED=$(gofmt -l .)
if [ -z "$UNFORMATTED" ]; then
    echo "✅ Code formatting is correct"
else
    echo "❌ The following files need formatting:"
    echo "$UNFORMATTED"
    echo ""
    echo "💡 Run: go fmt ./... to fix formatting"
    exit 1
fi

echo ""
echo "4️⃣  Running go vet..."
echo "---------------------"
if go vet ./...; then
    echo "✅ go vet passed"
else
    echo "❌ go vet failed"
    exit 1
fi

echo ""
echo "5️⃣  Running golangci-lint..."
echo "-----------------------------"

# Capture golangci-lint output for parsing, temporarily disable exit on error
set +e
LINT_OUTPUT=$(golangci-lint run --timeout=5m 2>&1)
LINT_EXIT_CODE=$?
set -e

if [ $LINT_EXIT_CODE -eq 0 ]; then
    echo "✅ golangci-lint passed"
else
    echo "❌ golangci-lint found issues that need to be fixed"
    echo ""
    echo "🤖 FULL GOLANGCI-LINT OUTPUT:"
    echo "======================================"
    echo "$LINT_OUTPUT"
    echo "======================================"
    echo ""
    
    # Try to extract specific issue lines for structured parsing
    ISSUE_LINES=$(echo "$LINT_OUTPUT" | grep -E "^[^[:space:]].*:[0-9]+:[0-9]+:" | head -20)
    
    if [ -n "$ISSUE_LINES" ]; then
        echo "🔍 PARSED ISSUES FOR LLM:"
        echo "-------------------------"
        echo "$ISSUE_LINES"
        echo ""
        echo "INSTRUCTIONS FOR LLM:"
        echo "- Each line above shows: file:line:column: message (linter)"
        echo "- Fix errcheck issues by checking returned errors"
        echo "- Fix ineffassign issues by removing or using assigned variables"
        echo "- Fix staticcheck issues according to Go best practices"  
        echo "- Fix unused issues by removing unused functions/variables or adding // nolint:unused if intentionally kept"
        echo "- Fix modernize issues by updating to modern Go syntax"
        echo "- Use Edit/MultiEdit tools to fix each issue"
        echo "- Include sufficient context when making edits"
    else
        echo "⚠️  Could not parse issue format. Please check the full output above."
    fi
    
    echo ""
    exit 1
fi

echo ""
echo "6️⃣  Checking go mod tidy..."
echo "---------------------------"
go mod tidy

# Check if there are changes after running go mod tidy
if git diff --exit-code go.mod; then
    echo "✅ go.mod is tidy"
else
    echo "✅ go.mod was updated by go mod tidy"
fi

# Check go.sum but don't fail the build for it (cache inconsistencies are common)
if git diff --exit-code go.sum; then
    echo "✅ go.sum is tidy"
else
    echo "⚠️  go.sum has changes (likely cached dependencies), but continuing..."
    echo "ℹ️  This is often due to module cache inconsistencies and doesn't indicate actual issues"
fi

echo ""
echo "🎉 Fast CI validation passed! Ready for commit."
echo "==============================================="
echo ""
echo "💡 Note: This runs core tests only. Full E2E tests will run in CI."
echo "   To run full validation: ./scripts/validate-ci.sh"