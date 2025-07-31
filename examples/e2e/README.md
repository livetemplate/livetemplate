# End-to-End Tests

This directory contains comprehensive end-to-end tests for all examples to ensure they continue working as we make changes to the codebase.

## 📁 Test Files

### `simple_test.go` - Simple Example Tests
Tests the basic template tracking functionality from `examples/simple/main.go`:

- ✅ **Template Registration** - Adding templates to tracker
- ✅ **Dependency Detection** - Automatic field dependency analysis  
- ✅ **Live Updates** - Real-time change detection and notifications
- ✅ **Template Rendering** - Actual HTML template execution
- ✅ **Targeted Updates** - Only affected templates are notified

### `files_test.go` - File-Based Template Tests
Tests the file loading functionality from `examples/files/main.go`:

- ✅ **Directory Loading** - Loading all templates from a directory
- ✅ **Specific File Loading** - Loading individual files with custom names
- ✅ **Path Detection** - Robust path resolution from different working directories
- ✅ **Template File Validation** - Ensuring all required template files exist
- ✅ **Dependency Analysis** - Verifying dependencies from actual template files
- ✅ **Live Updates** - File-based template change notifications

### `fragments_test.go` - Fragment Extraction Tests
Tests the automatic fragment extraction from `examples/fragments/main.go`:

- ✅ **Fragment Extraction** - Automatic minimal template fragment creation
- ✅ **Fragment Properties** - ID generation, content extraction, position tracking
- ✅ **Dependency Mapping** - Fragment-level dependency detection
- ✅ **Granular Updates** - Fragment-specific change notifications
- ✅ **Complex Templates** - Multi-section template processing
- ✅ **Targeted Fragment Updates** - Only affected fragments are updated

### `main_test.go` - Test Orchestration
Provides test runners and benchmarks:

- ✅ **Comprehensive Test Runner** - Runs all example tests together
- ✅ **Performance Benchmarks** - Measures example execution performance
- ✅ **Test Setup/Teardown** - Manages test environment

## 🚀 Running the Tests

### Run All E2E Tests
```bash
# From project root
go test ./examples/e2e -v

# Run specific test file
go test ./examples/e2e -run TestSimpleExample -v
go test ./examples/e2e -run TestFilesExample -v
go test ./examples/e2e -run TestFragmentsExample -v
```

### Run All Examples Tests Together
```bash
go test ./examples/e2e -run TestAllExamples -v
```

### Run Performance Benchmarks
```bash
go test ./examples/e2e -bench=. -v
```

### Run with Coverage
```bash
go test ./examples/e2e -cover -v
```

## 🎯 What These Tests Validate

| Test Category | Validation Focus | Benefits |
|---------------|------------------|----------|
| **Functionality** | All example features work correctly | Prevents regressions |
| **Integration** | Examples work with current API | Catches breaking changes |
| **Performance** | Examples execute within reasonable time | Detects performance issues |
| **Robustness** | Examples handle edge cases properly | Improves reliability |

## 📊 Test Coverage

The tests cover:

- ✅ **Core API Usage** - All major TemplateTracker methods
- ✅ **Template Processing** - Parsing, dependency analysis, fragment extraction
- ✅ **Live Update System** - Data change detection and notifications
- ✅ **File Operations** - Template loading from files and directories
- ✅ **Path Resolution** - Working from different directories
- ✅ **Error Handling** - Proper error responses and recovery
- ✅ **Data Structures** - Complex nested data handling
- ✅ **Real-world Scenarios** - Actual use cases from examples

## 🔧 Maintenance

### Adding New Tests
When adding new examples or features:

1. Create a new test file: `{example_name}_test.go`
2. Follow the existing test patterns
3. Add to `TestAllExamples` in `main_test.go`
4. Update this README

### Test Guidelines
- **Mirror Examples** - Tests should closely match the actual examples
- **Comprehensive Coverage** - Test all major functionality paths
- **Clear Assertions** - Descriptive error messages with context
- **Isolated Tests** - Each test should be independent
- **Performance Aware** - Use timeouts for async operations

## 🛡️ Integration with CI/CD

These tests are designed to:
- ✅ **Run in CI/CD pipelines** - No external dependencies
- ✅ **Provide clear feedback** - Detailed error messages
- ✅ **Execute quickly** - Reasonable test execution time
- ✅ **Handle different environments** - Robust path detection

## 📈 Benefits

1. **Regression Prevention** - Catch breaking changes before they reach production
2. **Documentation** - Tests serve as executable documentation
3. **Confidence** - Safe refactoring with comprehensive test coverage
4. **Quality Assurance** - Ensure examples always work for users
5. **Performance Monitoring** - Track performance characteristics over time
