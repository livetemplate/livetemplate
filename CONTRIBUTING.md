# Contributing to LiveTemplate

Thank you for your interest in contributing to LiveTemplate! This guide will help you get started.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Setup](#setup)
- [Development Workflow](#development-workflow)
- [Pre-commit Hook](#pre-commit-hook)
- [Testing](#testing)
- [Code Style](#code-style)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Where to Start](#where-to-start)
- [Getting Help](#getting-help)

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.21+** - Required for building and testing
- **Node.js 18+** - Required for client library development and testing
- **golangci-lint** - Required for linting (pre-commit hook)
  ```bash
  # macOS
  brew install golangci-lint

  # Linux/WSL
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
  ```
- **Chrome/Chromium** - Required for E2E browser tests (chromedp)

## Setup

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/yourusername/livetemplate.git
   cd livetemplate
   ```

2. **Install dependencies**
   ```bash
   # Go dependencies (automatically handled by Go modules)
   go mod download

   # Client library dependencies
   cd client
   npm install
   cd ..
   ```

3. **Install pre-commit hook** (automatically validates before each commit)
   ```bash
   cp scripts/pre-commit.sh .git/hooks/pre-commit
   chmod +x .git/hooks/pre-commit
   ```

4. **Verify setup**
   ```bash
   # Run all tests
   go test -v ./... -timeout=30s

   # Run client tests
   cd client && npm test && cd ..

   # Run linter
   golangci-lint run
   ```

## Development Workflow

### Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

2. **Make your changes**
   - Follow existing code patterns and conventions
   - Add tests for new functionality
   - Update documentation if needed

3. **Run tests frequently**
   ```bash
   # Quick feedback loop
   go test -v ./...

   # Or test specific packages
   go test -v -run TestYourSpecificTest
   ```

4. **Commit your changes**
   ```bash
   git add .
   git commit -m "your commit message"
   # Pre-commit hook will automatically run validation
   ```

### Directory Structure

```
livetemplate/
├── template.go          # Main API - Template type and public interface
├── tree.go              # Tree operations (private implementation)
├── tree_ast.go          # AST-based template parser
├── action.go            # Action protocol and data binding
├── mount.go             # Store pattern and HTTP/WebSocket handlers
├── session.go           # Session management
├── broadcast.go         # Broadcasting for multi-user apps
├── client/              # TypeScript client library
│   ├── livetemplate-client.ts
│   └── livetemplate-client.test.ts
├── cmd/lvt/             # CLI tool for code generation
├── examples/            # Example applications
│   ├── counter/
│   └── todos/
├── testdata/            # Test fixtures and golden files
│   └── e2e/
├── docs/                # Documentation
└── scripts/             # Development scripts
```

## Pre-commit Hook

The pre-commit hook is **CRITICAL** for maintaining code quality. It automatically:

1. **Auto-formats Go code** using `go fmt`
2. **Runs golangci-lint** to catch common issues
3. **Runs client tests** (npm test)
4. **Runs all Go tests** with 300-second timeout

### Important Rules

1. **NEVER skip the pre-commit hook** using `--no-verify`
   - The hook is there to catch issues early
   - Skipping it will break CI and block your PR

2. **Fix failures before committing**
   - Linting errors: Fix the code issues
   - Test failures: Ensure all tests pass
   - If stuck, ask for help (see [Getting Help](#getting-help))

3. **Formatted files are auto-added**
   - The hook runs `go fmt` and stages formatted files automatically
   - No need to manually format before committing

### Example Hook Output

```
🔄 Running pre-commit validation...
📝 Auto-formatting Go code...
✅ Code formatting completed
🔍 Running golangci-lint...
✅ Linting passed
🧪 Running npm tests...
✅ Client tests passed
🧪 Running Go tests...
✅ All Go tests passed
✅ Pre-commit validation completed successfully
```

## Testing

### Test Categories

1. **Unit Tests** - Fast tests for individual functions
   ```bash
   go test -v ./... -short
   ```

2. **E2E Tests** - End-to-end tests with template rendering
   ```bash
   go test -run TestTemplate_E2E -v
   ```

3. **Browser Tests** - Chromedp tests for real browser interactions
   ```bash
   go test -run TestE2E -v
   cd cmd/lvt/e2e && go test -v
   ```

4. **Client Tests** - TypeScript/Jest tests for client library
   ```bash
   cd client && npm test
   ```

5. **Fuzz Tests** - Randomized input testing
   ```bash
   go test -fuzz=FuzzTree -fuzztime=30s
   ```

### Golden Files

Many E2E tests use golden files in `testdata/e2e/`:
- `*.html` - Expected rendered HTML output
- `*.json` - Expected tree updates

To update golden files after intentional changes:
```bash
UPDATE_GOLDEN=1 go test -run TestTemplate_E2E -v
```

### Writing Tests

Follow these patterns:

**Unit test example:**
```go
func TestNewFeature(t *testing.T) {
    t.Run("description of test case", func(t *testing.T) {
        // Arrange
        input := "test input"

        // Act
        result := YourFunction(input)

        // Assert
        if result != expected {
            t.Errorf("expected %v, got %v", expected, result)
        }
    })
}
```

**E2E browser test example:**
```go
func TestFeature(t *testing.T) {
    // Setup server and browser context
    ctx, cancel := chromedp.NewContext(context.Background())
    defer cancel()

    // Run test actions
    err := chromedp.Run(ctx,
        chromedp.Navigate("http://localhost:8080"),
        chromedp.Click("button#submit"),
        chromedp.WaitVisible("#result"),
    )

    if err != nil {
        t.Fatal(err)
    }
}
```

## Code Style

### General Principles

1. **No unnecessary comments** - Code should be self-documenting
2. **Follow existing patterns** - Check neighboring code for conventions
3. **Use existing utilities** - Don't reinvent the wheel
4. **Maintain idiomatic Go** - Follow Go best practices

### Naming Conventions

- **Public API** (exported): PascalCase
  - `Template`, `Store`, `ActionContext`, `Broadcaster`
- **Internal implementation** (unexported): camelCase
  - `treeNode`, `keyGenerator`, `parseAction`
- **Test functions**: `TestFeatureName`
- **Benchmark functions**: `BenchmarkFeatureName`

### Public API Guidelines

The public API surface is minimal by design. Only export:
- Types that users directly interact with
- Functions that users must call
- Interfaces that users implement

Keep implementation details private.

### Documentation

- Add godoc comments for all public types and functions
- Document non-obvious behavior and edge cases
- Include examples in godoc when helpful

```go
// Template represents a parsed template that can generate updates.
// It maintains state between renders to produce minimal diffs.
type Template struct {
    // ...
}

// ExecuteToUpdate renders the template and returns a JSON update.
// This is more efficient than ExecuteToHTML for subsequent renders.
func (t *Template) ExecuteToUpdate(data interface{}) (*UpdateResponse, error) {
    // ...
}
```

## Commit Messages

Use conventional commit format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code restructuring without behavior change
- `test`: Adding or updating tests
- `docs`: Documentation changes
- `perf`: Performance improvements
- `chore`: Build process or tooling changes

### Examples

```
feat(template): add support for nested template invokes

Implements recursive template invocation to support complex
component hierarchies. Updates tree parser to handle nested
{{template}} calls correctly.

Closes #123
```

```
fix(client): prevent duplicate WebSocket connections

Adds connection state tracking to prevent race condition where
multiple connections could be established during reconnection.

Fixes #456
```

```
refactor: minimize public API surface

BREAKING CHANGE: Internal types like TreeNode and KeyGenerator
are now private. Users should only interact with Template,
Store, and ActionContext interfaces.
```

## Pull Requests

### Before Submitting

1. Ensure all tests pass locally
2. Update documentation if needed
3. Add tests for new features
4. Rebase on latest main branch
5. Run the pre-commit hook manually if needed:
   ```bash
   .git/hooks/pre-commit
   ```

### PR Description Template

```markdown
## Description
Brief description of changes

## Motivation
Why is this change needed?

## Changes
- List of specific changes
- One per line

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Pre-commit hook passes
- [ ] No breaking changes (or documented if necessary)
```

### Review Process

1. PRs require at least one approval
2. CI must pass (tests, linting, formatting)
3. Address reviewer feedback
4. Maintainer will merge when ready

## Where to Start

### Good First Issues

Look for issues labeled `good first issue` - these are:
- Well-defined and scoped
- Don't require deep system knowledge
- Good for getting familiar with the codebase

### Areas to Explore

1. **Client library features** (`client/livetemplate-client.ts`)
   - Add new event bindings
   - Improve error handling
   - Performance optimizations

2. **Documentation** (`docs/`)
   - Improve existing docs
   - Add examples
   - Fix typos or unclear sections

3. **Testing** (various `*_test.go` files)
   - Add test coverage
   - Improve E2E tests
   - Add edge case tests

4. **CLI tool** (`cmd/lvt/`)
   - New generators
   - Kit improvements
   - Development server features

### Learning the Codebase

1. **Read the docs**
   - `CLAUDE.md` - Development guidelines
   - `docs/ARCHITECTURE.md` - System architecture
   - `docs/CODE_TOUR.md` - Guided code walkthrough

2. **Run the examples**
   ```bash
   cd examples/counter
   go run main.go
   # Open http://localhost:8080
   ```

3. **Read the tests**
   - Tests are excellent documentation
   - Start with `e2e_test.go` for high-level flow
   - Check `template_test.go` for core functionality

4. **Experiment**
   - Make small changes
   - Run tests to see what breaks
   - Use debugger to step through code

## Getting Help

- **Questions**: Open a discussion on GitHub
- **Bugs**: Open an issue with reproduction steps
- **Features**: Open an issue to discuss before implementing
- **Real-time help**: Check if there's a Discord/Slack (if available)

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (check LICENSE file).

---

Thank you for contributing to LiveTemplate!
