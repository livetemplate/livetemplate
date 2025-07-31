#!/bin/bash

# Git Hook Installation Script
# This script installs the pre-commit hook that runs Go tests before allowing commits

set -e  # Exit on any error

echo "🔧 Installing Git Pre-commit Hook"
echo "================================="

# Check if we're in a Git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a Git repository"
    echo "   Please run this script from within a Git repository"
    exit 1
fi

# Get repository root
REPO_ROOT="$(git rev-parse --show-toplevel)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "📍 Repository root: $REPO_ROOT"
echo "📍 Scripts directory: $SCRIPT_DIR"

# Check if validate-tests.sh exists
VALIDATE_SCRIPT="$SCRIPT_DIR/validate-tests.sh"
if [ ! -f "$VALIDATE_SCRIPT" ]; then
    echo "❌ Error: validate-tests.sh not found at $VALIDATE_SCRIPT"
    echo "   Please ensure the validation script exists"
    exit 1
fi

# Create the pre-commit hook
HOOK_PATH="$REPO_ROOT/.git/hooks/pre-commit"
echo "📝 Creating pre-commit hook at: $HOOK_PATH"

cat > "$HOOK_PATH" << 'EOF'
#!/bin/bash

# Git pre-commit hook - calls the validation script
# This hook runs Go tests before allowing commits

# Get the directory containing this script
HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Look for the validation script in the scripts directory
VALIDATE_SCRIPT="$REPO_ROOT/scripts/validate-tests.sh"

if [ ! -f "$VALIDATE_SCRIPT" ]; then
    echo "❌ Error: Validation script not found at $VALIDATE_SCRIPT"
    echo "   Please ensure scripts/validate-tests.sh exists and is executable"
    exit 1
fi

echo "🔗 Running pre-commit validation..."
echo "   Using script: $VALIDATE_SCRIPT"
echo ""

# Execute the validation script
if "$VALIDATE_SCRIPT"; then
    echo ""
    echo "✅ Pre-commit validation passed! Proceeding with commit."
    exit 0
else
    echo ""
    echo "❌ Pre-commit validation failed! Commit rejected."
    echo ""
    echo "💡 Additional tips:"
    echo "   • Fix the failing tests before trying to commit again"
    echo "   • Use 'git commit --no-verify' to bypass this hook (not recommended)"
    echo "   • Run 'scripts/validate-tests.sh' manually to test your changes"
    exit 1
fi
EOF

# Make the hook executable
chmod +x "$HOOK_PATH"

echo ""
echo "✅ Git pre-commit hook installed successfully!"
echo ""
echo "📋 What was installed:"
echo "   • Pre-commit hook: $HOOK_PATH"
echo "   • Validation script: $VALIDATE_SCRIPT"
echo ""
echo "🎯 How it works:"
echo "   • Every 'git commit' will automatically run 'go test ./...'"
echo "   • Commits are blocked if any tests fail"
echo "   • The hook calls scripts/validate-tests.sh for the actual validation"
echo ""
echo "🧪 Test the installation:"
echo "   • Run: scripts/validate-tests.sh (manual test)"
echo "   • Or make a test commit to see the hook in action"
echo ""
echo "🚀 Ready to go! Your commits are now protected by automated testing."
