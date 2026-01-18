#!/bin/bash

# Pre-commit hook for LiveTemplate
# Automatically formats code and runs validation

set -e

echo "🔄 Running pre-commit validation..."

# Step 0: Clean up generated test artifacts
if [ -d "cmd/lvt/commands/internal" ]; then
    echo "🧹 Cleaning up generated test artifacts..."
    rm -rf cmd/lvt/commands/internal/
fi

# Step 1: Auto-format Go code before validation
echo "📝 Auto-formatting Go code..."
if GOWORK=off go fmt ./...; then
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

# Step 2: Run golangci-lint
echo "🔍 Running golangci-lint..."
LINT_OUTPUT=$(GOWORK=off golangci-lint run --enable-only=errcheck,govet,ineffassign,staticcheck,unused 2>&1) || LINT_EXIT_CODE=$?

if [ -z "$LINT_EXIT_CODE" ] || [ "$LINT_EXIT_CODE" -eq 0 ]; then
    echo "✅ Linting passed"
else
    echo "❌ Linting failed - commit blocked"
    echo ""
    echo "$LINT_OUTPUT"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════════"
    echo "                    INSTRUCTIONS FOR LLM ASSISTANTS                    "
    echo "═══════════════════════════════════════════════════════════════════════"
    echo ""
    echo "If you are an LLM (Claude, GPT, etc.) helping with this commit:"
    echo ""
    echo "1. Parse the lint errors above carefully"
    echo "2. For each error, identify: file path, line number, linter name"
    echo "3. Fix each issue using the guidance below"
    echo "4. After fixing, run: golangci-lint run --enable-only=errcheck,govet,ineffassign,staticcheck,unused"
    echo "5. Stage fixed files: git add <files>"
    echo "6. Retry commit: git commit -m 'message'"
    echo ""
    echo "LINTER-SPECIFIC FIX GUIDANCE:"
    echo ""
    echo "  errcheck - Unchecked error return values"
    echo "    Fix: Handle the error or use _ = fn() if intentionally ignoring"
    echo "    Example: err := doSomething(); if err != nil { return err }"
    echo ""
    echo "  unused - Unused code (variables, functions, types, constants)"
    echo "    Fix: Remove the unused code or add a usage"
    echo "    If intentionally unused, prefix with _ (e.g., _unusedVar)"
    echo ""
    echo "  staticcheck - Static analysis (SA* codes)"
    echo "    Fix: Follow the specific guidance in the error message"
    echo "    Docs: https://staticcheck.dev/docs/checks/"
    echo "    Common: SA1019 (deprecated), SA4006 (unused value), SA9003 (empty branch)"
    echo ""
    echo "  govet - Suspicious constructs (go vet checks)"
    echo "    Fix: Follow the specific guidance in the error message"
    echo "    Common: printf format mismatches, struct tag issues, unreachable code"
    echo ""
    echo "  ineffassign - Ineffective assignments"
    echo "    Fix: Remove the unused assignment or use the variable"
    echo "    Example: x := 1; x = 2 -> change to x := 2 (if x=1 never read)"
    echo ""
    echo "CRITICAL RULES:"
    echo "  - DO NOT use --no-verify to skip this hook"
    echo "  - DO NOT add //nolint comments without strong justification"
    echo "  - DO NOT ignore errors that could indicate bugs"
    echo "  - ALWAYS fix all lint errors before committing"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════════"
    exit 1
fi

# Step 3: Run npm tests (client library)
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

# Step 4: Run all Go tests with increased timeout for slow e2e tests
echo "🧪 Running Go tests..."
if GOWORK=off go test -v ./... -timeout=300s; then
    echo "✅ All Go tests passed"
else
    echo "❌ Go tests failed - commit blocked"
    exit 1
fi

echo "✅ Pre-commit validation completed successfully"
