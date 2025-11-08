---
description: Recursively review and fix Go files as an expert until Grade A
---

# Recursive Go Code Review & Fix

You are a senior Go expert conducting a thorough, iterative code review. Your goal is to achieve **Grade A** code quality through recursive review and improvement cycles.

## Process Overview

0. **Setup Git Worktree** → Create isolated environment for changes
1. **Review** → Identify issues and grade the code
2. **Fix** → Apply improvements with tests
3. **Review Again** → Check if Grade A achieved
4. **Repeat** → Continue until Grade A or max 3 iterations
5. **Create PR** → Submit changes via pull request

## Phase 0: Git Worktree Setup (MANDATORY)

Before starting any review work, create an isolated git worktree:

### Worktree Setup Steps

1. **Determine the file to review** from command arguments
2. **Create branch name** from filename:
   - Format: `review/{filename}-grade-a`
   - Example: `types.go` → `review/types-grade-a`
   - Example: `internal/build/render.go` → `review/render-grade-a`

3. **Create worktree**:
   ```bash
   # Try to create from existing branch first, otherwise create new
   git worktree add ../{repo}-review-{filename} review/{filename}-grade-a 2>&1 || \
   git worktree add -b review/{filename}-grade-a ../{repo}-review-{filename} 2>&1
   ```

4. **Verify worktree created successfully**
5. **Switch to worktree for all subsequent work**

### Important Notes
- All file modifications MUST happen in the worktree
- All commits MUST be made in the worktree
- Never modify files in the main working directory

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
- ✅ Proceed to Phase 5: Create Pull Request

**If Grade < A and iterations < 3:**
- ♻️ Return to Phase 2 with remaining issues
- ♻️ Continue improvement cycle

**If iterations >= 3:**
- ⚠️ Provide final status
- ⚠️ List remaining issues
- ⚠️ Commit work-in-progress
- ⚠️ Proceed to Phase 5: Create Pull Request (mark as draft)

## Phase 5: Create Pull Request (MANDATORY)

After all commits are made and tests pass, create a pull request:

### PR Creation Steps

1. **Push branch to remote**:
   ```bash
   git push -u origin review/{filename}-grade-a
   ```

2. **Create PR using gh CLI**:
   ```bash
   gh pr create \
     --title "refactor({filename}): achieve Grade A code quality" \
     --body "$(cat <<'EOF'
   ## Summary

   Recursive code review of {filename} to achieve Grade A production quality.

   ### Changes Made
   - [List key improvements from review]

   ### Metrics
   | Metric | Before | After | Change |
   |--------|--------|-------|--------|
   | Grade | [Before] | [After] | [↑/→] |
   | Test Coverage | [X%] | [Y%] | [+/-Z%] |
   | Documentation | [Grade] | [Grade] | [↑/→] |

   ### Iteration Summary
   - **Starting Grade:** [Grade]
   - **Final Grade:** [Grade]
   - **Iterations:** [N]
   - **Test Status:** [All passing / X failures]

   ### Files Modified
   - {filename} - [description]
   - {filename}_test.go - [description]
   - [additional files if any]

   ### Production Readiness
   [✅ Production Ready / ⚠️ Needs Review]

   [Detailed explanation of final state]

   ---

   🤖 Generated with [Claude Code](https://claude.com/claude-code)

   Co-Authored-By: Claude <noreply@anthropic.com>
   EOF
   )"
   ```

3. **If Grade < A (iterations >= 3), mark as draft**:
   ```bash
   gh pr ready --undo  # Mark as draft
   ```

4. **Display PR URL to user**

### PR Cleanup After Merge

When user indicates PR has been merged:
```bash
# Switch back to main
cd {original-directory}

# Update main branch
git checkout main
git pull

# Remove worktree
git worktree remove {worktree-path}

# Delete local branch
git branch -d review/{filename}-grade-a

# Clean up go.work if needed
# Remove worktree entry from go.work file
```

### Important Notes
- Always create PR, even if Grade A not achieved (mark as draft)
- PR description should be comprehensive and standalone
- Include all metrics and test status
- Link to any related issues if applicable

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
- **Worktree:** {worktree-path}
- **Branch:** review/{filename}-grade-a

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
[List of commits made in worktree]

### Pull Request
- **Status:** [Created / Draft]
- **URL:** [PR URL]
- **Title:** refactor({filename}): achieve Grade A code quality

### Production Readiness: [✅/⚠️]
[Final verdict]

### Next Steps
1. Review the PR at [URL]
2. Merge when ready
3. Worktree will be cleaned up automatically
```

## Important Notes

- **MANDATORY: Use git worktree** - All work must be done in isolated worktree
- **MANDATORY: Create PR** - Always create pull request, even for WIP
- **Always run tests** after each fix
- **Never skip test coverage** - add tests for all new code paths
- **Be thorough** - check edge cases, error handling, concurrency
- **Be pragmatic** - balance perfection with practical improvements
- **Document decisions** - explain trade-offs in commit messages
- **Stop at Grade A** - don't over-engineer

## Usage

```bash
/go-review types.go
/go-review internal/build/render.go
```

The command will:
0. Create git worktree in isolated branch (e.g., `review/types-grade-a`)
1. Review the file and its tests
2. Identify and fix issues (in worktree)
3. Re-review until Grade A or 3 iterations
4. Commit all changes (in worktree)
5. Push to remote and create pull request
6. Provide comprehensive summary with PR URL

## Complete Workflow Example

```bash
# User runs command
/go-review types.go

# Assistant:
# 1. Creates worktree: ../livetemplate-review-types
# 2. Creates branch: review/types-grade-a
# 3. Reviews types.go
# 4. Makes improvements in worktree
# 5. Commits changes
# 6. Pushes to origin
# 7. Creates PR with gh CLI
# 8. Returns PR URL to user

# User reviews PR, merges when ready
# Worktree cleanup happens on next /go-review or manually
```
