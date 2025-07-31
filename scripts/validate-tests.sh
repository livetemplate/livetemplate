#!/bin/bash

# Pre-commit validation script for Go projects
# This script runs 'go test ./...' and can be used as a Git pre-commit hook
# or run manually for validation

set -e  # Exit on any error

echo "🧪 Running Go tests..."
echo "====================="

# Get the repository root (works both in Git hook context and manual execution)
if git rev-parse --git-dir > /dev/null 2>&1; then
    REPO_ROOT="$(git rev-parse --show-toplevel)"
else
    REPO_ROOT="$(pwd)"
fi

cd "$REPO_ROOT"

# Check if this is a Go project
if [ ! -f "go.mod" ]; then
    echo "⚠️  No go.mod found - skipping Go tests"
    exit 0
fi

# Run build first to catch compilation errors
echo "📍 Running from: $REPO_ROOT"
echo "� Building: go build ./..."
echo ""

# Capture both stdout and stderr, and preserve exit code
if BUILD_OUTPUT=$(go build ./... 2>&1); then
    if [ -n "$BUILD_OUTPUT" ]; then
        echo "$BUILD_OUTPUT"
    fi
    echo "✅ Build successful!"
else
    echo "$BUILD_OUTPUT"
    echo ""
    echo "❌ Build failed!"
    echo ""
    echo "💡 Tips:"
    echo "   • Run 'go build ./...' to see detailed build errors"
    echo "   • Fix compilation errors before proceeding"
    echo "   • Check for missing imports or syntax errors"
    exit 1
fi

echo ""
echo "🔍 Running: go test ./..."
echo ""

# Run all tests with verbose output for better debugging
if TEST_OUTPUT=$(go test ./... -short 2>&1); then
    echo "$TEST_OUTPUT"
    echo ""
    echo "✅ Unit tests passed!"
else
    echo "$TEST_OUTPUT"
    echo ""
    echo "❌ Unit tests failed!"
    echo ""
    echo "💡 Tips:"
    echo "   • Run 'go test ./...' to see detailed test failures"
    echo "   • Fix failing tests before proceeding"
    echo "   • Run 'go test -v ./...' for verbose test output"
    exit 1
fi

echo ""
echo "🎯 Running end-to-end tests..."
echo ""

# Run E2E tests to ensure examples still work
if E2E_OUTPUT=$(go test ./examples/e2e -v 2>&1); then
    echo "$E2E_OUTPUT"
    echo ""
    echo "✅ E2E tests passed!"
    echo ""
    echo "🎉 All validation checks passed!"
    exit 0
else
    echo "$E2E_OUTPUT"
    echo ""
    echo "❌ E2E tests failed!"
    echo ""
    echo "💡 Tips:"
    echo "   • Run 'go test ./examples/e2e -v' to see detailed E2E failures"
    echo "   • Examples may be broken - check template files and paths"
    echo "   • Ensure all example dependencies are available"
    exit 1
fi
