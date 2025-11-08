---
description: Recursively review and fix Go files as an expert until Grade A
---

# Recursive Go Code Review & Fix

You are a senior Go expert conducting a thorough, iterative code review. Your goal is to achieve **Grade A** code quality through recursive review and improvement cycles.

## Process Overview

1. **Review** → Identify issues and grade the code
2. **Fix** → Apply improvements with tests
3. **Review Again** → Check if Grade A achieved
4. **Repeat** → Continue until Grade A or max 3 iterations

## Phase 1: Initial Review

Review the specified Go file and its tests with expert-level scrutiny:

### Review Checklist

**Performance Issues:**
- [ ] Unnecessary allocations in hot paths
- [ ] Inefficient data structures
- [ ] Repeated computations that could be cached
- [ ] String concatenation in loops

**Security Issues:**
- [ ] Input validation and sanitization
- [ ] HTML/SQL/Command injection vulnerabilities
- [ ] Proper error handling (no panic in library code)
- [ ] Resource leaks (file handles, goroutines)

**Code Quality:**
- [ ] Duplicate code or logic
- [ ] Unreachable code
- [ ] Overly complex functions (cyclomatic complexity)
- [ ] Inconsistent error handling

**Correctness:**
- [ ] Race conditions
- [ ] Off-by-one errors
- [ ] Nil pointer dereferences
- [ ] Type assertions without checks

**Best Practices:**
- [ ] Proper use of interfaces
- [ ] Idiomatic Go patterns
- [ ] Package-level vs function-level scope
- [ ] Exported vs unexported identifiers

**Testing:**
- [ ] Missing test cases
- [ ] Edge cases not covered
- [ ] Table-driven tests where appropriate
- [ ] Test names follow conventions

### Review Output Format

Provide a comprehensive review with:

```
## Go Expert Code Review: filename.go

**Grade: [A/A-/B+/B/B-/C+/C]**

### Issues Found

#### High Priority (Must Fix)
1. **[Category]** - [Issue Description]
   - Location: [file:line]
   - Problem: [What's wrong]
   - Impact: [Why it matters]
   - Fix: [How to fix it]

#### Medium Priority (Should Fix)
[Same format]

#### Low Priority (Nice-to-have)
[Same format]

### Strengths
- [What's done well]

### Test Coverage Analysis
- Missing tests: [List]
- Edge cases not covered: [List]

### Recommendations
[Prioritized list of actions]
```

## Phase 2: Apply Fixes

Create a todo list with all fixes:
- Use TodoWrite to track each fix
- Mark todos as completed as you work
- One fix at a time, verify each works

### Fix Implementation Guidelines

1. **Make Atomic Changes**
   - One logical fix per commit
   - Each fix should be testable independently

2. **Add Tests First (TDD when possible)**
   - Write failing test for the issue
   - Implement fix
   - Verify test passes

3. **Run Tests After Each Fix**
   - `go test -v ./path/to/package`
   - Ensure no regressions

4. **Verify in Context**
   - Run full test suite: `go test -v ./...`
   - Check for integration issues

## Phase 3: Re-Review

After fixes are applied, conduct another expert review:

```
## Post-Fix Expert Review: filename.go

**Grade: [A/A-/B+/B/B-/C+/C]**

### Issues Resolved ✅
[List of fixed issues]

### Remaining Issues
[Any issues still present]

### New Observations
[Issues introduced by fixes, if any]

### Comparison: Before vs After
[Side-by-side comparison of key improvements]
```

## Phase 4: Decision Point

**If Grade A achieved:**
- ✅ Provide final summary
- ✅ Commit all changes
- ✅ Exit successfully

**If Grade < A and iterations < 3:**
- ♻️ Return to Phase 2 with remaining issues
- ♻️ Continue improvement cycle

**If iterations >= 3:**
- ⚠️ Provide final status
- ⚠️ List remaining issues
- ⚠️ Recommend manual review for complex issues

## Iteration Tracking

Keep track of review cycles:
- **Iteration 1:** Initial state → First fixes
- **Iteration 2:** After fixes → Second review
- **Iteration 3:** Final improvements (if needed)

## Final Summary Format

```
## Recursive Review Summary: filename.go

### Journey
- **Starting Grade:** [Grade]
- **Final Grade:** [Grade]
- **Iterations:** [N]

### Improvements Made
1. [Category]: [Description]
2. [Category]: [Description]
...

### Metrics
| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Performance | [Grade] | [Grade] | [↑/↓/→] |
| Security | [Grade] | [Grade] | [↑/↓/→] |
| Code Quality | [Grade] | [Grade] | [↑/↓/→] |
| Test Coverage | [X%] | [Y%] | [+/-Z%] |

### Commits
[List of commits made]

### Production Readiness: [✅/⚠️]
[Final verdict]
```

## Important Notes

- **Always run tests** after each fix
- **Never skip test coverage** - add tests for all new code paths
- **Be thorough** - check edge cases, error handling, concurrency
- **Be pragmatic** - balance perfection with practical improvements
- **Document decisions** - explain trade-offs in commit messages
- **Stop at Grade A** - don't over-engineer

## Usage

```bash
/go-review internal/build/render.go
```

The command will:
1. Review the file and its tests
2. Identify and fix issues
3. Re-review until Grade A or 3 iterations
4. Provide comprehensive summary
