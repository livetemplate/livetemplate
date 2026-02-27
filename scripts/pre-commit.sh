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
    echo "    IMPORTANT: Actually handle errors, don't just suppress them!"
    echo ""
    echo "    WRONG (just suppressing):"
    echo "      _ = f.Close()           // BAD: hides potential bugs"
    echo "      _, _ = doSomething()    // BAD: ignoring errors"
    echo ""
    echo "    RIGHT (proper handling):"
    echo "      if err := f.Close(); err != nil {"
    echo "          log.Printf(\"failed to close file: %v\", err)"
    echo "      }"
    echo ""
    echo "      err := doSomething()"
    echo "      if err != nil {"
    echo "          return fmt.Errorf(\"operation failed: %w\", err)"
    echo "      }"
    echo ""
    echo "    IN TESTS - use t.Errorf or t.Logf:"
    echo "      if err := cleanup(); err != nil {"
    echo "          t.Errorf(\"cleanup failed: %v\", err)"
    echo "      }"
    echo ""
    echo "    IN DEFERS - log or handle gracefully:"
    echo "      defer func() {"
    echo "          if err := f.Close(); err != nil {"
    echo "              log.Printf(\"close error: %v\", err)"
    echo "          }"
    echo "      }()"
    echo ""
    echo "  unused - Unused code (variables, functions, types, constants)"
    echo "    Fix: REMOVE the unused code entirely. Don't just rename with _."
    echo "    If code is needed for future use, it shouldn't be committed yet."
    echo ""
    echo "  staticcheck - Static analysis (SA*/S*/ST* codes)"
    echo "    Fix: Follow the specific guidance in the error message"
    echo "    Docs: https://staticcheck.dev/docs/checks/"
    echo "    Common: SA1012 (nil context), SA1029 (built-in type as key)"
    echo "    Simplifications: S1009 (nil check before len), QF1012 (use fmt.Fprintf)"
    echo ""
    echo "  govet - Suspicious constructs (go vet checks)"
    echo "    Fix: Follow the specific guidance in the error message"
    echo "    Common: printf format mismatches, struct tag issues, nil pointer"
    echo ""
    echo "  ineffassign - Ineffective assignments"
    echo "    Fix: Remove the unused assignment or use the variable"
    echo "    Example: x := 1; x = 2 -> change to x := 2 (if x=1 never read)"
    echo ""
    echo "CRITICAL RULES:"
    echo "  - NEVER suppress errors with '_ =' just to pass linting"
    echo "  - NEVER use --no-verify to skip this hook"
    echo "  - NEVER add //nolint comments without strong justification"
    echo "  - ALWAYS handle errors properly (log, return, or test assertion)"
    echo "  - ALWAYS remove unused code rather than renaming it"
    echo ""
    echo "═══════════════════════════════════════════════════════════════════════"
    exit 1
fi

# Step 3: Run npm tests (client library)
if [ -d "client" ]; then
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
else
    echo "⏭️  Skipping npm tests (client/ directory not found)"
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
