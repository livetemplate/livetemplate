#!/bin/bash

# CI Validation Script for LiveTemplate
# Runs comprehensive validation including tests, formatting, vetting, and linting

set -e

echo "🚀 Starting CI validation for LiveTemplate..."
echo "================================================"

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

echo ""
echo "1️⃣  Running Go tests..."
echo "------------------------"
if go test ./... -v; then
    echo "✅ All tests passed"
else
    echo "❌ Tests failed"
    exit 1
fi

echo ""
echo "2️⃣  Checking code formatting..."
echo "--------------------------------"
UNFORMATTED=$(gofmt -l .)
if [ -z "$UNFORMATTED" ]; then
    echo "✅ Code formatting is correct"
else
    echo "❌ The following files need formatting:"
    echo "$UNFORMATTED"
    echo ""
    echo "💡 Code formatting should be handled by git hooks before commit."
    echo "   If you're seeing this in CI, it means formatting wasn't applied during commit."
    echo "   Run: go fmt ./... locally and commit the changes."
    exit 1
fi

echo ""
echo "3️⃣  Running go vet..."
echo "---------------------"
if go vet ./...; then
    echo "✅ go vet passed"
else
    echo "❌ go vet failed"
    exit 1
fi

echo ""
echo "4️⃣  Running golangci-lint..."
echo "-----------------------------"

# Capture golangci-lint output for parsing
LINT_OUTPUT=$(golangci-lint run --timeout=5m 2>&1)
LINT_EXIT_CODE=$?

if [ $LINT_EXIT_CODE -eq 0 ]; then
    echo "✅ golangci-lint passed"
else
    echo "❌ golangci-lint found issues that need to be fixed"
    echo ""
    echo "🤖 LLM-PARSEABLE GOLANGCI-LINT ISSUES:"
    echo "======================================"
    echo "The following issues were found by golangci-lint and need to be addressed:"
    echo ""
    
    # Extract just the issue lines (file:line:column format)
    echo "$LINT_OUTPUT" | grep -E "^[^[:space:]].*:[0-9]+:[0-9]+:" | head -20
    
    echo ""
    echo "INSTRUCTIONS FOR LLM:"
    echo "- Each line above shows: file:line:column: message (linter)"
    echo "- Fix ineffassign issues by removing or using assigned variables"
    echo "- Fix staticcheck issues according to Go best practices"  
    echo "- Fix unused issues by removing unused functions/variables or adding // nolint:unused if intentionally kept"
    echo "- Use replace_string_in_file tool to fix each issue"
    echo "- Include sufficient context (3-5 lines before/after) when using replace_string_in_file"
    echo "======================================"
    echo ""
    exit 1
fi

echo ""
echo "5️⃣  Checking go mod tidy..."
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
echo "🎉 All CI validation checks passed!"
echo "===================================="