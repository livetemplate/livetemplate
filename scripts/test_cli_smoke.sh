#!/bin/bash
# Smoke test for the LiveTemplate CLI generator

set -e

echo "🔥 Running CLI generator smoke test..."
echo ""

# Get the project root (assuming script is in scripts/)
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Create temp directory
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo "📁 Working in: $TMPDIR"

# Build the CLI from project root
echo ""
echo "1️⃣  Building CLI..."
cd "$PROJECT_ROOT"
go build -o "$TMPDIR/lvt" ./cmd/lvt

cd $TMPDIR

# Test: lvt new
echo ""
echo "2️⃣  Testing: lvt new testapp..."
./lvt new testapp

# Verify app structure
if [ ! -f "testapp/go.mod" ]; then
    echo "❌ go.mod not created"
    exit 1
fi

if [ ! -f "testapp/cmd/testapp/main.go" ]; then
    echo "❌ main.go not created"
    exit 1
fi

if [ ! -d "testapp/internal/database" ]; then
    echo "❌ database directory not created"
    exit 1
fi

echo "✅ App structure created successfully"

# Test: lvt gen (CRUD resource)
echo ""
echo "3️⃣  Testing: lvt gen users name:string email:string..."
cd testapp
../lvt gen users name:string email:string

# Verify resource files
if [ ! -f "internal/app/users/users.go" ]; then
    echo "❌ users.go not created"
    exit 1
fi

if [ ! -f "internal/app/users/users.tmpl" ]; then
    echo "❌ users.tmpl not created"
    exit 1
fi

if [ ! -f "internal/app/users/users_ws_test.go" ]; then
    echo "❌ users_ws_test.go not created"
    exit 1
fi

if [ ! -f "internal/app/users/users_test.go" ]; then
    echo "❌ users_test.go (E2E) not created"
    exit 1
fi

# Check if schema was appended
if ! grep -q "CREATE TABLE" internal/database/schema.sql; then
    echo "❌ schema.sql not updated"
    exit 1
fi

# Check if queries were appended
if ! grep -q "GetAllUsers" internal/database/queries.sql; then
    echo "❌ queries.sql not updated"
    exit 1
fi

echo "✅ Resource files generated successfully (including tests)"

# Test: compile generated code (without sqlc for now)
echo ""
echo "4️⃣  Testing: Code compilation..."

# Add replace directive for local development
echo "Adding replace directive for local livetemplate..."
echo "" >> go.mod
echo "replace github.com/livefir/livetemplate => $PROJECT_ROOT" >> go.mod

# Run go mod tidy to download dependencies
echo "Running go mod tidy..."
go mod tidy 2>&1 | head -5

# Create a minimal models package for testing compilation
mkdir -p internal/database/models
cat > internal/database/models/models.go <<EOF
package models

import "time"

type User struct {
    ID        string
    Name      string
    Email     string
    CreatedAt time.Time
}

type Queries struct {
}

func New(db interface{}) *Queries {
    return &Queries{}
}

type CreateUserParams struct {
    ID        string
    Name      string
    Email     string
    CreatedAt time.Time
}

type UpdateUserParams struct {
    ID    string
    Name  string
    Email string
}

func (q *Queries) CreateUser(ctx interface{}, arg CreateUserParams) (User, error) {
    return User{}, nil
}

func (q *Queries) GetAllUsers(ctx interface{}) ([]User, error) {
    return nil, nil
}

func (q *Queries) GetUserByID(ctx interface{}, id string) (User, error) {
    return User{}, nil
}

func (q *Queries) UpdateUser(ctx interface{}, arg UpdateUserParams) error {
    return nil
}

func (q *Queries) DeleteUser(ctx interface{}, id string) error {
    return nil
}
EOF

# Try to build the users package
if ! go build ./internal/app/users/... 2>&1; then
    echo "❌ Generated code failed to compile"
    exit 1
fi

echo "✅ Generated code compiles successfully"

echo ""
echo "=============================================="
echo "🎉 Smoke test passed!"
echo "=============================================="
echo ""
echo "Summary:"
echo "  ✅ CLI built successfully"
echo "  ✅ 'lvt new' creates app structure"
echo "  ✅ 'lvt gen' creates resource files"
echo "  ✅ Generated code compiles"
echo ""
