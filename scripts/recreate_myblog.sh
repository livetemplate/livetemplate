#!/bin/bash

# Script to quickly recreate myblog for testing
# Usage: ./scripts/recreate_myblog.sh

set -e  # Exit on error

MYBLOG_DIR="/Users/adnaan/code/myblog"
LVT_PATH="/Users/adnaan/code/livefir/livetemplate"
LVT_BINARY="$LVT_PATH/lvt"

echo "🔨 Recreating myblog from scratch..."
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
"$LVT_BINARY" gen posts title content published:bool
echo "✅ Posts resource generated"

echo "📝 Generating categories resource..."
"$LVT_BINARY" gen categories name description
echo "✅ Categories resource generated"

echo "📝 Generating comments resource..."
"$LVT_BINARY" gen comments text post_id:int
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
echo "To run the app:"
echo "  cd $MYBLOG_DIR"
echo "  go run cmd/myblog/main.go"
echo ""
echo "Then visit: http://localhost:8080"
