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
# Calls the comprehensive CI validation script

set -e

echo "🔄 Running pre-commit validation..."

# Call the comprehensive CI validation script
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
