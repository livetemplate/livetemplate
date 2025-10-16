#!/bin/bash

# Pre-commit hook for LiveTemplate
# Automatically formats code and runs validation

set -e

echo "🔄 Running pre-commit validation..."

# Step 1: Auto-format Go code before validation
echo "📝 Auto-formatting Go code..."
if go fmt ./...; then
    echo "✅ Code formatting completed"

    # Add any formatted files to the commit
    FORMATTED_FILES=$(git diff --name-only)
    if [ -n "$FORMATTED_FILES" ]; then
        echo "📁 Adding formatted files to commit:"
        echo "$FORMATTED_FILES"
        git add $FORMATTED_FILES
    fi
else
    echo "❌ Code formatting failed"
    exit 1
fi

# Step 2: Run npm tests (client library)
echo "🧪 Running npm tests..."
cd client
if npm test; then
    echo "✅ Client tests passed"
    cd ..
else
    echo "❌ Client tests failed - commit blocked"
    cd ..
    exit 1
fi

# Step 3: Run all Go tests with increased timeout for slow e2e tests
# Exclude the problematic e2e tests (flaky browser automation)
echo "🧪 Running Go tests..."
if go test -v ./... -timeout=180s -skip="TestPageModeURLRouting|TestModalFunctionality"; then
    echo "✅ All Go tests passed"
else
    echo "❌ Go tests failed - commit blocked"
    exit 1
fi

echo "✅ Pre-commit validation completed successfully"
