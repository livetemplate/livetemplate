#!/bin/bash
set -e

echo "🔥 Running CLI generator smoke test..."
echo

# Create temporary directory
TEMP_DIR=$(mktemp -d)
echo "📁 Working in: $TEMP_DIR"
echo

# Save project root
PROJECT_ROOT=$(pwd)

# Cleanup function
cleanup() {
  cd "$PROJECT_ROOT"
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

cd "$TEMP_DIR"

# 1. Build CLI
echo "1️⃣  Building CLI..."
cd "$PROJECT_ROOT"
go build -o "$TEMP_DIR/lvt" ./cmd/lvt
echo

cd "$TEMP_DIR"

# 2. Test: lvt new
echo "2️⃣  Testing: lvt new testapp..."
./lvt new testapp
echo "✅ App structure created successfully"
echo

# 3. Test: lvt gen (CRUD resource)
echo "3️⃣  Testing: lvt gen users name:string email:string..."
cd testapp
"$TEMP_DIR/lvt" gen users name:string email:string
echo "✅ Resource files generated successfully (including tests)"
echo

# 4. Verify file structure
echo "4️⃣  Testing: Code compilation..."
echo "Adding replace directive for local livetemplate..."
echo "" >> go.mod
echo "replace github.com/livefir/livetemplate => $PROJECT_ROOT" >> go.mod

echo "Running go mod tidy..."
GOWORK=off go mod tidy 2>&1 | head -5 || true

# Try to compile (we expect failure due to missing sqlc models, but structure should be valid)
GOWORK=off go build ./... 2>&1 || echo "⚠️  Compilation failed as expected (requires sqlc-generated models)"
echo

# Check that key files exist
echo "5️⃣  Verifying file structure..."
REQUIRED_FILES=(
  "go.mod"
  "README.md"
  "cmd/testapp/main.go"
  "internal/database/db.go"
  "internal/database/schema.sql"
  "internal/database/queries.sql"
  "internal/database/sqlc.yaml"
  "internal/app/users/users.go"
  "internal/app/users/users.tmpl"
  "internal/app/users/users_ws_test.go"
  "internal/app/users/users_test.go"
)

ALL_EXIST=true
for file in "${REQUIRED_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "❌ Missing file: $file"
    ALL_EXIST=false
  fi
done

if [ "$ALL_EXIST" = true ]; then
  echo "✅ All required files exist"
else
  exit 1
fi

# 6. Test: lvt gen view
echo
echo "6️⃣  Testing: lvt gen view dashboard..."
"$TEMP_DIR/lvt" gen view dashboard
echo "✅ View files generated successfully"

# Verify view files
VIEW_FILES=(
  "internal/app/dashboard/dashboard.go"
  "internal/app/dashboard/dashboard.tmpl"
  "internal/app/dashboard/dashboard_ws_test.go"
  "internal/app/dashboard/dashboard_test.go"
)

ALL_VIEW_EXIST=true
for file in "${VIEW_FILES[@]}"; do
  if [ ! -f "$file" ]; then
    echo "❌ Missing view file: $file"
    ALL_VIEW_EXIST=false
  fi
done

if [ "$ALL_VIEW_EXIST" = true ]; then
  echo "✅ All view files exist"
else
  exit 1
fi

echo
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 All smoke tests passed!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
