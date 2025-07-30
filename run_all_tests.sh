#!/bin/bash

echo "🧪 Running Enhanced State Template Tests"
echo "========================================"
echo ""

# Run the custom test runner
echo "📋 Running Custom Test Suite..."
go run cmd/test-runner/main.go

echo ""
echo "📋 Running Standard Go Tests..."
go test -v

echo ""
echo "✨ All testing completed!"
