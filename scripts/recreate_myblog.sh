#!/bin/bash

# Script to quickly recreate myblog for testing
# Usage: ./scripts/recreate_myblog.sh [--edit-mode modal|page]

set -e  # Exit on error

MYBLOG_DIR="/Users/adnaan/code/myblog"
LVT_PATH="/Users/adnaan/code/livefir/livetemplate"
LVT_BINARY="$LVT_PATH/lvt"

# Parse command line arguments
EDIT_MODE="modal"  # default
while [[ $# -gt 0 ]]; do
    case $1 in
        --edit-mode)
            EDIT_MODE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--edit-mode modal|page]"
            exit 1
            ;;
    esac
done

# Validate edit mode
if [[ "$EDIT_MODE" != "modal" && "$EDIT_MODE" != "page" ]]; then
    echo "Error: Invalid edit mode '$EDIT_MODE'. Must be 'modal' or 'page'"
    exit 1
fi

echo "🔨 Recreating myblog from scratch..."
echo "📋 Edit mode: $EDIT_MODE"
echo ""

# Step 1: Remove old myblog
if [ -d "$MYBLOG_DIR" ]; then
    echo "📁 Removing old myblog directory..."
    rm -rf "$MYBLOG_DIR"
fi

# Step 2: Build lvt (always rebuild to ensure templates are embedded)
echo "🔧 Building lvt..."
cd "$LVT_PATH"
make build > /dev/null 2>&1
echo "✅ lvt built"

# Step 3: Create new myblog app
echo "📦 Creating new myblog app..."
cd "$(dirname $MYBLOG_DIR)"
"$LVT_BINARY" new myblog --module myblog --dev
echo "✅ App created"

# Step 4: Generate resources
echo "📝 Generating posts resource..."
cd "$MYBLOG_DIR"
"$LVT_BINARY" gen posts title content published:bool --edit-mode "$EDIT_MODE"
echo "✅ Posts resource generated"

echo "📝 Generating categories resource..."
"$LVT_BINARY" gen categories name description --edit-mode "$EDIT_MODE"
echo "✅ Categories resource generated"

echo "📝 Generating comments resource..."
"$LVT_BINARY" gen comments text post_id:int --edit-mode "$EDIT_MODE"
echo "✅ Comments resource generated"

# Step 5: Add replace directive to use local livetemplate
echo "🔗 Linking to local livetemplate..."
go mod edit -replace=github.com/livefir/livetemplate="$LVT_PATH"
go mod tidy > /dev/null 2>&1
echo "✅ Using local livetemplate"

# Step 6: Copy latest client library
echo "📦 Copying latest client library..."
cp "$LVT_PATH/client/dist/livetemplate-client.browser.js" livetemplate-client.js
echo "✅ Client library updated"

# Step 7: Run migrations
echo "🗄️  Running database migrations..."
"$LVT_BINARY" migration up
echo "✅ Migrations complete"

echo ""
echo "✨ myblog recreated successfully!"
echo ""

# Step 8: Kill any previous server on port 8080
echo "🔪 Checking for existing server on port 8080..."
if lsof -ti:8080 > /dev/null 2>&1; then
    echo "   Killing existing server..."
    lsof -ti:8080 | xargs kill -9 2>/dev/null || true
    sleep 1
    echo "✅ Previous server stopped"
else
    echo "✅ No existing server found"
fi

# Step 9: Start the new server
echo ""
echo "🚀 Starting myblog server..."
echo "   Visit: http://localhost:8080"
echo ""
cd "$MYBLOG_DIR"
go run cmd/myblog/main.go
