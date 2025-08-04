#!/bin/bash

# Install Git Hooks for StateTemplate

set -e

echo "Installing git hooks for StateTemplate..."

# Create hooks directory if it doesn't exist
mkdir -p .git/hooks

# Install pre-commit hook
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash

# Pre-commit hook for StateTemplate
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

# Step 2: Run CI validation (which now only checks, doesn't format)
if [ -f "./scripts/validate-ci.sh" ]; then
    echo "📋 Running CI validation script..."
    ./scripts/validate-ci.sh
else
    echo "❌ validate-ci.sh script not found at ./scripts/validate-ci.sh"
    exit 1
fi

echo "✅ Pre-commit validation completed successfully"
EOF

# Make pre-commit hook executable  
chmod +x .git/hooks/pre-commit

echo "✅ Git hooks installed successfully"
echo "Pre-commit hook will now run tests and validation before each commit"
