# PR #9 Code Review Fixes - Progress Tracker

**Created**: 2025-10-19
**PR**: #9 (cli branch)
**Status**: In Progress
**Worktree**: TBD (will be created when work begins)

---

## Overview

This tracker addresses critical fixes and code quality improvements identified in the PR #9 code review. The PR introduces the complete `lvt` CLI tool with kit system, code generation, and comprehensive E2E testing.

**Total Tasks**: 12
**Completed**: 0
**In Progress**: 0
**Remaining**: 12

---

## Phase 1: Critical Fixes (Blockers)

These issues must be resolved before the PR can be merged.

### 🔴 Task 1.1: Commit Uncommitted Changes

**Status**: ⬜ Not Started
**Priority**: 🔴 Blocker
**Estimated Effort**: 15 minutes

**Description**:
Commit all pending changes that are currently showing in `git status`:
- `cmd/lvt/e2e/complete_workflow_test.go` - Un-skipped flaky test with retry logic
- `cmd/lvt/e2e/url_routing_test.go` - Modified
- `cmd/lvt/internal/config/config.go` - Added globalConfigPath support
- `cmd/lvt/internal/config/config_test.go` - Modified
- `docs/design/multi-session-isolation.md` - New design doc

**Files**:
- `cmd/lvt/e2e/complete_workflow_test.go`
- `cmd/lvt/e2e/url_routing_test.go`
- `cmd/lvt/internal/config/config.go`
- `cmd/lvt/internal/config/config_test.go`
- `docs/design/multi-session-isolation.md`

**Acceptance Criteria**:
- [ ] All uncommitted changes are committed
- [ ] Commit messages follow conventional commits format
- [ ] `git status` shows clean working directory
- [ ] Changes are properly attributed with Co-Authored-By if applicable

**Implementation Notes**:
```bash
# Review changes
git diff cmd/lvt/e2e/complete_workflow_test.go
git diff cmd/lvt/e2e/url_routing_test.go
git diff cmd/lvt/internal/config/config.go
git diff cmd/lvt/internal/config/config_test.go
git status docs/design/multi-session-isolation.md

# Stage and commit
git add <files>
git commit -m "fix: improve E2E test stability with retry logic and better timeouts"
git commit -m "feat: add custom config path support to config manager"
git commit -m "docs: add multi-session isolation design document"
```

---

### 🔴 Task 1.2: Fix Global State in config.go

**Status**: ⬜ Not Started
**Priority**: 🔴 Blocker
**Estimated Effort**: 1 hour

**Description**:
Remove package-level global state from `config.go` which is not thread-safe and makes testing difficult.

Current problematic code:
```go
var globalConfigPath string

func SetConfigPath(path string) {
    globalConfigPath = path
}
```

**Files**:
- `cmd/lvt/internal/config/config.go`
- `cmd/lvt/internal/config/config_test.go`
- Any code that calls `SetConfigPath()`

**Acceptance Criteria**:
- [ ] Remove `globalConfigPath` package variable
- [ ] Create `ConfigManager` struct to hold config state
- [ ] Update all config functions to be methods on `ConfigManager`
- [ ] Update all callers to use `ConfigManager` instance
- [ ] Ensure tests can run in parallel without state leakage
- [ ] All existing tests still pass
- [ ] Add new test for concurrent config access

**Implementation Notes**:
```go
// New approach
type ConfigManager struct {
    customPath string
}

func NewConfigManager() *ConfigManager {
    return &ConfigManager{}
}

func (cm *ConfigManager) SetCustomPath(path string) {
    cm.customPath = path
}

func (cm *ConfigManager) GetConfigPath() (string, error) {
    if cm.customPath != "" {
        return cm.customPath, nil
    }
    // ... default logic
}
```

**Dependencies**: None

---

### 🔴 Task 1.3: Validate Flaky Test Fixes

**Status**: ⬜ Not Started
**Priority**: 🔴 Blocker
**Estimated Effort**: 2 hours

**Description**:
The "Edit Post" test in `complete_workflow_test.go` was previously skipped due to flakiness. It has been un-skipped with retry logic. We need to validate this fix is stable.

**Files**:
- `cmd/lvt/e2e/complete_workflow_test.go` (lines 258-379)

**Acceptance Criteria**:
- [ ] Run "Edit Post" test 20 times in a row successfully
- [ ] No random timeouts or failures
- [ ] Average test duration is acceptable (< 30s)
- [ ] Logs show consistent behavior
- [ ] Consider alternative waiting strategies if still flaky

**Implementation Notes**:
```bash
# Run test multiple times
for i in {1..20}; do
    echo "Run $i of 20..."
    go test -v -run TestCompleteWorkflow_BlogApp/Edit_Post ./cmd/lvt/e2e/ || exit 1
done
```

If test fails:
1. Analyze failure pattern (timeout, element not found, etc.)
2. Consider using chromedp's built-in `WaitFunc` instead of manual retries
3. Add more detailed logging
4. Consider using `chromedp.WaitReady` instead of `WaitVisible`
5. If cannot stabilize: Re-skip with detailed explanation

**Dependencies**: Task 1.1 (commit changes first)

---

### 🔴 Task 1.4: Run Full Test Suite

**Status**: ⬜ Not Started
**Priority**: 🔴 Blocker
**Estimated Effort**: 30 minutes

**Description**:
Run the complete test suite to ensure all tests pass before merge.

**Files**: All test files

**Acceptance Criteria**:
- [ ] `go test -v ./... -timeout=10m` passes with 0 failures
- [ ] No race conditions detected with `go test -race ./...`
- [ ] All E2E tests pass (including Docker Chrome tests)
- [ ] Pre-commit hook passes
- [ ] No linting errors from golangci-lint

**Implementation Notes**:
```bash
# Full test suite
go test -v ./... -timeout=10m

# With race detection
go test -race ./... -timeout=10m

# Linting
golangci-lint run

# Pre-commit hook (if configured)
git commit --dry-run --allow-empty -m "test"
```

**Dependencies**:
- Task 1.1 (commit changes)
- Task 1.2 (fix global state)
- Task 1.3 (validate flaky tests)

---

## Phase 2: Code Quality Improvements

These improvements enhance code quality and maintainability.

### 🟡 Task 2.1: Add Unit Tests for Config Package

**Status**: ⬜ Not Started
**Priority**: 🟡 High
**Estimated Effort**: 2 hours

**Description**:
Add comprehensive unit tests for the config package, especially the new `ConfigManager` struct.

**Files**:
- `cmd/lvt/internal/config/config_test.go` (expand)

**Acceptance Criteria**:
- [ ] Test `DefaultConfig()` returns expected values
- [ ] Test `LoadConfig()` with missing file (should return default)
- [ ] Test `LoadConfig()` with valid YAML file
- [ ] Test `LoadConfig()` with invalid YAML (should error)
- [ ] Test `SaveConfig()` creates directory if needed
- [ ] Test `SaveConfig()` writes correct YAML format
- [ ] Test `AddKitPath()` validates path exists
- [ ] Test `AddKitPath()` prevents duplicates
- [ ] Test `AddKitPath()` converts to absolute path
- [ ] Test `RemoveKitPath()` removes existing path
- [ ] Test `RemoveKitPath()` errors on non-existent path
- [ ] Test `Validate()` catches invalid paths
- [ ] Test concurrent access with `ConfigManager` (after refactor)
- [ ] Achieve >80% code coverage for config package

**Implementation Notes**:
```go
func TestConfigManager_ConcurrentAccess(t *testing.T) {
    cm := NewConfigManager()

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            path := fmt.Sprintf("/tmp/config-%d.yaml", id)
            cm.SetCustomPath(path)
            _, _ = cm.LoadConfig()
        }(i)
    }
    wg.Wait()
}
```

**Dependencies**: Task 1.2 (ConfigManager refactor)

---

### 🟡 Task 2.2: Extract Hardcoded Sleeps to Constants

**Status**: ⬜ Not Started
**Priority**: 🟡 High
**Estimated Effort**: 30 minutes

**Description**:
Replace hardcoded `time.Sleep()` values in E2E tests with named constants for better maintainability.

**Files**:
- `cmd/lvt/e2e/complete_workflow_test.go`
- `cmd/lvt/e2e/url_routing_test.go`
- Other E2E test files

**Acceptance Criteria**:
- [ ] Define constants at package level (e.g., `modalOpenDelay`, `wsReadyTimeout`)
- [ ] Replace all hardcoded sleep durations with constants
- [ ] Add comments explaining why each delay is needed
- [ ] Reduce total sleep time where possible using proper waits
- [ ] Tests still pass after changes

**Implementation Notes**:
```go
const (
    // modalOpenDelay is the time to wait for modal animation to complete
    modalOpenDelay = 500 * time.Millisecond

    // wsReadyTimeout is the maximum time to wait for WebSocket connection
    wsReadyTimeout = 5 * time.Second

    // formSubmitDelay is the time to wait after form submission
    formSubmitDelay = 2 * time.Second
)

// Replace:
chromedp.Sleep(500*time.Millisecond)
// With:
chromedp.Sleep(modalOpenDelay)
```

**Dependencies**: None

---

### 🟡 Task 2.3: Improve Modal Wait Logic in E2E Tests

**Status**: ⬜ Not Started
**Priority**: 🟡 High
**Estimated Effort**: 1.5 hours

**Description**:
Replace manual retry loops with chromedp's built-in waiting mechanisms for more robust E2E tests.

**Files**:
- `cmd/lvt/e2e/complete_workflow_test.go` (lines 304-330)

**Acceptance Criteria**:
- [ ] Replace manual `for` loop retries with `chromedp.WaitFunc`
- [ ] Use `chromedp.WaitReady` or `chromedp.WaitVisible` where appropriate
- [ ] Remove unnecessary `Sleep()` calls after proper waits
- [ ] Add custom wait conditions using `chromedp.ActionFunc`
- [ ] Tests are more reliable and faster
- [ ] All E2E tests still pass

**Implementation Notes**:
```go
// Replace manual retry loop:
maxRetries := 10
for i := 0; i < maxRetries && !inputVisible; i++ {
    time.Sleep(500 * time.Millisecond)
    // ... check condition
}

// With chromedp WaitFunc:
err := chromedp.WaitFunc(func(ctx context.Context, cur *chromedp.Frame) error {
    var modalOpen bool
    if err := chromedp.Evaluate(`
        const modal = document.getElementById('edit-modal');
        const input = document.querySelector('input[name="title"]');
        modal && !modal.hasAttribute('hidden') && input !== null
    `, &modalOpen).Do(ctx); err != nil {
        return err
    }
    if !modalOpen {
        return errors.New("modal not open")
    }
    return nil
}).Do(ctx)
```

**Dependencies**: Task 2.2 (extract constants first)

---

### 🟢 Task 2.4: Add SQL Parameterization in Test Seeding

**Status**: ⬜ Not Started
**Priority**: 🟢 Medium
**Estimated Effort**: 45 minutes

**Description**:
Replace direct SQL string concatenation in test seeding with parameterized queries or use the application's data layer.

**Files**:
- `cmd/lvt/e2e/url_routing_test.go` (lines 86-88)

**Acceptance Criteria**:
- [ ] Remove direct SQL string execution
- [ ] Use application's data models/repositories if available
- [ ] OR use sqlite3 parameterized queries
- [ ] Tests still pass and seed data correctly
- [ ] No SQL injection risk in test code (sets good example)

**Implementation Notes**:
```go
// Current (problematic):
seedCmd := exec.Command("sqlite3", dbPath,
    "INSERT INTO products (id, name, created_at, updated_at) VALUES ('test-prod-1', 'Test Product 1', datetime('now'), datetime('now'));")

// Better approach - use app's data layer:
// Import the app's product repository and create products properly
// This also ensures migrations and models are in sync

// Alternative - use Go's database/sql with parameters:
db, _ := sql.Open("sqlite3", dbPath)
_, _ = db.Exec("INSERT INTO products (id, name, created_at, updated_at) VALUES (?, ?, datetime('now'), datetime('now'))",
    "test-prod-1", "Test Product 1")
```

**Dependencies**: None

---

## Completion Checklist

When all tasks are complete:

### Pre-Merge Validation
- [ ] All Phase 1 tasks completed (Critical Fixes)
- [ ] All Phase 2 tasks completed (Code Quality)
- [ ] Full test suite passes: `go test -v ./... -timeout=10m`
- [ ] No race conditions: `go test -race ./... -timeout=10m`
- [ ] Pre-commit hook passes
- [ ] Linting passes: `golangci-lint run`
- [ ] All changes committed with proper messages
- [ ] Code reviewed by team (if applicable)

### Merge Process
- [ ] Merge worktree changes back to `cli` branch
- [ ] Run full test suite on `cli` branch
- [ ] If tests fail: Fix issues before cleanup
- [ ] If unsure about fixes: Stop and request help
- [ ] Only after tests pass: Cleanup worktree

---

## Notes and Issues

### Known Issues
- E2E tests require Docker Chrome environment
- Some tests are timing-sensitive and may need tuning
- Binary size with embedded kits not yet measured

### Future Work (Not in Scope)
- PR splitting strategy (deferred)
- CI/CD pipeline setup (deferred)
- Code coverage reporting (deferred)
- Binary size optimization (deferred)

---

## Progress Summary

**Phase 1 (Critical)**: 0/4 tasks complete (0%)
**Phase 2 (Quality)**: 0/4 tasks complete (0%)
**Overall**: 0/8 tasks complete (0%)

**Last Updated**: 2025-10-19
