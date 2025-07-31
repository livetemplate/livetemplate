# 🧪 Testing Strategy Overview

This document outlines the comprehensive testing approach for the StateTemplate Go library, ensuring quality and preventing regressions as the codebase evolves.

## 📋 Testing Layers

### 1. **Unit Tests** (`*_test.go` files alongside source)
- **Location**: `template_tracker_test.go`, `advanced_analyzer_test.go`, `fragment_extractor_test.go`
- **Purpose**: Test individual functions and methods in isolation
- **Coverage**: Core functionality, edge cases, error handling
- **Run with**: `go test ./... -short`

### 2. **End-to-End Tests** (`examples/e2e/`)
- **Location**: `examples/e2e/{simple,files,fragments,main}_test.go`
- **Purpose**: Validate that all examples work correctly with real-world usage
- **Coverage**: Full user workflows, integration scenarios, example functionality
- **Run with**: `go test ./examples/e2e -v`

### 3. **Build Validation** (via scripts)
- **Location**: `scripts/validate-tests.sh`
- **Purpose**: Ensure code compiles correctly across all packages
- **Coverage**: Compilation errors, missing imports, syntax issues
- **Run with**: `go build ./...`

## 🎯 What Each Test Layer Validates

| Layer | Focus | Benefits | When to Use |
|-------|--------|----------|-------------|
| **Unit Tests** | Individual functions, edge cases | Fast feedback, precise error location | During development, debugging specific functions |
| **E2E Tests** | Complete workflows, real usage | Confidence in user experience | Before releases, validating examples |
| **Build Validation** | Compilation, syntax | Catch basic errors early | Every commit, CI/CD pipelines |

## 🚀 Running Tests

### Quick Validation (recommended for development)
```bash
# Run all tests including build validation
bash scripts/validate-tests.sh
```

### Individual Test Suites
```bash
# Unit tests only
go test ./... -short

# E2E tests only  
go test ./examples/e2e -v

# Build validation only
go build ./...
```

### Comprehensive Testing
```bash
# All tests with coverage
go test ./... -cover

# Performance benchmarks
go test ./examples/e2e -bench=.

# Verbose output for debugging
go test ./... -v
```

## 📊 Test Coverage Details

### **Unit Tests Coverage**
- ✅ **TemplateTracker** - Adding templates, dependency detection, live updates
- ✅ **AdvancedAnalyzer** - Complex data structure analysis, nested dependencies
- ✅ **FragmentExtractor** - HTML parsing, fragment extraction, ID generation
- ✅ **Error Handling** - Invalid templates, missing files, malformed data
- ✅ **Edge Cases** - Empty data, circular references, malformed HTML

### **E2E Tests Coverage**
- ✅ **Simple Example** - Basic template tracking and live updates
- ✅ **Files Example** - Loading templates from files and directories
- ✅ **Fragments Example** - Automatic fragment extraction and granular updates
- ✅ **Path Resolution** - Working from different directories
- ✅ **Real Templates** - Using actual HTML template files
- ✅ **Integration** - All components working together

### **Build Validation Coverage**
- ✅ **Package Compilation** - All packages build successfully
- ✅ **Import Resolution** - All dependencies available
- ✅ **Syntax Validation** - No Go syntax errors
- ✅ **Cross-Package Dependencies** - Internal package imports work

## 🔧 Git Integration

### **Pre-Commit Hook**
The Git pre-commit hook automatically runs the full validation suite:

```bash
# Install the hook
bash scripts/install-git-hooks.sh

# Manual validation (same as what the hook runs)
bash scripts/validate-tests.sh
```

### **Validation Steps**
1. **Build Check** - `go build ./...`
2. **Unit Tests** - `go test ./... -short`  
3. **E2E Tests** - `go test ./examples/e2e -v`

## 📈 Benefits of This Testing Strategy

### **Development Benefits**
- 🚀 **Fast Feedback** - Unit tests run quickly during development
- 🎯 **Precise Debugging** - Know exactly which component has issues
- 🔒 **Regression Prevention** - E2E tests catch breaking changes
- 📚 **Living Documentation** - Tests serve as usage examples

### **Quality Assurance Benefits**
- ✅ **Comprehensive Coverage** - Multiple testing layers catch different issues
- 🛡️ **Production Confidence** - Real-world scenarios tested
- 🔄 **Continuous Validation** - Git hooks prevent broken commits
- 📊 **Performance Monitoring** - Benchmarks track performance over time

### **Maintenance Benefits**
- 🔧 **Safe Refactoring** - Comprehensive tests enable confident changes
- 📖 **Clear Examples** - E2E tests show how to use the library
- 🎨 **Code Quality** - Testing requirements encourage good design
- 🚀 **Easy Onboarding** - New developers can understand usage from tests

## 🎯 Testing Best Practices

### **Writing Tests**
- **Be Specific** - Test one thing at a time with clear assertions
- **Use Real Data** - Test with realistic data structures and templates
- **Handle Timeouts** - Use timeouts for async operations (channels, goroutines)
- **Clean Setup/Teardown** - Each test should be independent

### **Running Tests**
- **Run Before Committing** - Use Git hooks or manual validation
- **Test Different Scenarios** - Run from different directories
- **Monitor Performance** - Use benchmarks to catch performance regressions
- **Check Coverage** - Ensure new code is adequately tested

### **Debugging Failed Tests**
- **Read Error Messages** - Tests provide detailed failure context
- **Run Individual Tests** - Isolate failing tests with `-run` flag
- **Use Verbose Output** - `-v` flag shows detailed test execution
- **Check Examples** - E2E tests failing often means examples are broken

## 🔄 Continuous Improvement

### **Adding New Tests**
1. **New Features** - Add unit tests for new functionality
2. **New Examples** - Add E2E tests for new example scenarios
3. **Bug Fixes** - Add regression tests for fixed bugs
4. **Performance** - Add benchmarks for performance-critical code

### **Maintaining Tests**
- **Keep Tests Updated** - Update tests when APIs change
- **Review Test Failures** - Understand why tests fail before fixing them
- **Optimize Performance** - Keep test execution time reasonable
- **Document Changes** - Update this document when testing strategy evolves

---

## 🏃‍♂️ Quick Start

For new developers or contributors:

```bash
# 1. Run full validation to ensure everything works
bash scripts/validate-tests.sh

# 2. Make your changes

# 3. Run tests again to ensure nothing is broken
bash scripts/validate-tests.sh

# 4. Install Git hooks for automatic validation
bash scripts/install-git-hooks.sh

# 5. Commit with confidence!
git commit -m "Your awesome changes"
```

The validation will run automatically on commit, preventing broken code from entering the repository.
