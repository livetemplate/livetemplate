#!/bin/bash

# Install Git Hooks for LiveTemplate

set -e

echo "Installing git hooks for LiveTemplate..."

# Create hooks directory if it doesn't exist
mkdir -p .git/hooks

# Install pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
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

# Step 2: Run fast CI validation for pre-commit (which now only checks, doesn't format)
if [ -f "./scripts/validate-ci-fast.sh" ]; then
    echo "📋 Running fast CI validation script..."
    ./scripts/validate-ci-fast.sh
else
    echo "❌ validate-ci-fast.sh script not found at ./scripts/validate-ci-fast.sh"
    echo "💡 Falling back to full CI validation..."
    if [ -f "./scripts/validate-ci.sh" ]; then
        ./scripts/validate-ci.sh
    else
        echo "❌ No validation scripts found"
        exit 1
    fi
fi

echo "✅ Pre-commit validation completed successfully"
EOF

# Make pre-commit hook executable  
chmod +x .git/hooks/pre-commit

echo "✅ Git hooks installed successfully"
echo "Pre-commit hook will now run fast validation (core tests + linting) before each commit"
echo "💡 Full E2E tests will run in CI - this keeps commits fast while ensuring quality"
