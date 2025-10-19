# Progress Tracker Generator

Generate structured progress trackers for large tasks with git worktree isolation and automated testing.

This skill creates comprehensive progress trackers that:
- Break down large tasks into manageable phases
- Set up isolated git worktrees for work
- Track progress with checkboxes and priorities
- Provide merge-back workflow with test validation
- Handle cleanup automatically

## Usage

Invoke this skill when you need to:
- Start work on a large refactoring or feature
- Track progress across multiple work sessions
- Isolate work in a separate git worktree
- Ensure tests pass before merging back

## Input Required

When invoking this skill, Claude will ask for:
1. **Task Name**: Brief description (e.g., "fix-pr-9-issues")
2. **Branch Name**: Git branch to create (e.g., "fix/pr-9-code-review")
3. **Base Branch**: Branch to work from (e.g., "cli" or "main")
4. **Task Description**: Detailed description of what needs to be done
5. **Context**: Any relevant code review comments, issues, or requirements

## How It Works

### Phase 1: Setup Git Worktree

```bash
# 1. Generate worktree directory name from task
WORKTREE_DIR="../$(basename $PWD)-${TASK_NAME}"
BRANCH_NAME="${BRANCH_NAME}"
BASE_BRANCH="${BASE_BRANCH}"

# 2. Create worktree with new branch
git worktree add "$WORKTREE_DIR" -b "$BRANCH_NAME" "$BASE_BRANCH"

# 3. Record worktree metadata
echo "Worktree created at: $WORKTREE_DIR"
echo "Branch: $BRANCH_NAME"
echo "Base: $BASE_BRANCH"
```

### Phase 2: Generate Progress Tracker

Claude analyzes the task description and creates a structured progress tracker:

1. **Break down into phases**:
   - Phase 1: Critical/Blocker tasks (must do)
   - Phase 2: High priority tasks (should do)
   - Phase 3+: Medium/Low priority tasks (nice to have)

2. **For each task, include**:
   - ✅ Checkbox for tracking completion
   - Priority indicator (🔴 Blocker, 🟡 High, 🟢 Medium, ⚪ Low)
   - Status (Not Started, In Progress, Completed)
   - Estimated effort
   - Description and context
   - Files affected
   - Acceptance criteria (specific, testable)
   - Implementation notes with code examples
   - Dependencies on other tasks

3. **Generate tracker file**:
   - Location: `docs/progress/<task-name>-tracker.md`
   - Format: Markdown with checkboxes
   - Metadata: Creation date, worktree path, branch info

### Phase 3: Work in Worktree

```bash
# Change to worktree directory
cd "$WORKTREE_DIR"

# Verify you're on the correct branch
git branch --show-current

# Make changes, commit regularly
git add <files>
git commit -m "fix: <description>"

# Track progress by updating tracker
# Mark tasks as complete using checkboxes
```

### Phase 4: Merge Back and Cleanup

When all tasks are complete:

```bash
# 1. Ensure all work is committed
git status  # Should be clean

# 2. Switch back to base branch
cd <original-repo-path>
git checkout "$BASE_BRANCH"

# 3. Merge worktree branch
git merge "$BRANCH_NAME" --no-ff -m "merge: ${TASK_NAME} fixes"

# 4. CRITICAL: Run full test suite
echo "Running tests after merge..."
go test -v ./... -timeout=10m

# 5. Check test results
if [ $? -ne 0 ]; then
    echo "❌ Tests failed after merge!"
    echo "Fix the failing tests before continuing cleanup."
    echo ""
    echo "To debug:"
    echo "  - Review test output above"
    echo "  - Check for merge conflicts or integration issues"
    echo "  - Fix issues and re-run: go test -v ./... -timeout=10m"
    echo ""
    echo "If you're unsure how to fix, STOP and ask for help."
    exit 1
fi

echo "✅ All tests passed!"

# 6. Only after tests pass: Remove worktree
git worktree remove "$WORKTREE_DIR"

# 7. Optionally delete branch (if fully merged)
git branch -d "$BRANCH_NAME"

echo "✅ Cleanup complete!"
```

## Progress Tracker Template

The generated tracker follows this structure:

```markdown
# <Task Name> - Progress Tracker

**Created**: <date>
**Branch**: <branch-name>
**Base Branch**: <base-branch>
**Worktree Path**: <path>
**Status**: In Progress

---

## Overview

<Task description and context>

**Total Tasks**: X
**Completed**: 0
**In Progress**: 0
**Remaining**: X

---

## Phase 1: Critical Tasks

### 🔴 Task 1.1: <Task Name>

**Status**: ⬜ Not Started
**Priority**: 🔴 Blocker
**Estimated Effort**: <time>

**Description**:
<What needs to be done>

**Files**:
- `path/to/file.go`

**Acceptance Criteria**:
- [ ] Specific testable criteria
- [ ] Another criteria

**Implementation Notes**:
\```language
// Code examples or commands
\```

**Dependencies**: Task X.Y

---

## Completion Checklist

### Pre-Merge Validation
- [ ] All critical tasks completed
- [ ] All tests pass
- [ ] No linting errors
- [ ] Changes committed

### Merge Process
- [ ] Merge worktree branch to base
- [ ] Run full test suite: `go test -v ./... -timeout=10m`
- [ ] **If tests fail**: Fix before cleanup
- [ ] **If unsure**: Stop and request help
- [ ] Only after tests pass: Cleanup worktree

---

## Progress Summary

**Phase 1**: 0/X tasks complete (0%)
**Phase 2**: 0/Y tasks complete (0%)
**Overall**: 0/Z tasks complete (0%)

**Last Updated**: <date>
```

## Safety Features

1. **Test Validation**: Always runs tests after merge, blocks cleanup if tests fail
2. **Manual Override**: Requires user to fix test failures before continuing
3. **Ask for Help**: Explicitly instructs to stop if unsure how to fix
4. **Clean State**: Verifies worktree is clean before merge
5. **No Force**: Uses safe git operations (no force push, no hard reset)

## Example Invocation

```
User: "I need to fix the issues from PR #9 code review. Can you create a progress tracker?"

Claude uses this skill to:
1. Ask for task details (name, branch, base, description)
2. Create git worktree: `../livetemplate-fix-pr-9`
3. Generate progress tracker: `docs/progress/fix-pr-9-tracker.md`
4. Break down review comments into tracked tasks
5. Provide worktree path and next steps
```

## Implementation

When Claude invokes this skill, it will:

1. **Prompt for information**:
   ```
   Please provide:
   - Task name (slug format): _______
   - Branch name: _______
   - Base branch: _______
   - Task description: _______
   ```

2. **Create worktree**:
   - Generate worktree path from repo name and task name
   - Create branch from base branch
   - Set up worktree directory

3. **Analyze task**:
   - Parse task description
   - Identify subtasks and phases
   - Determine priorities
   - Estimate effort

4. **Generate tracker**:
   - Create markdown file in `docs/progress/`
   - Include all metadata
   - Format with checkboxes and priorities
   - Add acceptance criteria for each task

5. **Provide instructions**:
   - Show how to switch to worktree
   - Explain workflow
   - Show merge-back commands
   - Emphasize test validation

## Error Handling

If errors occur during:

- **Worktree creation**: Check if branch already exists, if worktree path conflicts
- **Merge**: Handle merge conflicts, provide resolution steps
- **Test failures**: Show failing tests, suggest debugging steps, block cleanup
- **Cleanup**: Verify worktree can be removed, check for uncommitted changes

## Best Practices

1. **Keep tasks atomic**: Each task should be independently testable
2. **Use checkboxes**: Track progress by checking off completed tasks
3. **Update estimates**: Adjust time estimates as you learn more
4. **Commit frequently**: Small commits are easier to review and revert
5. **Run tests often**: Don't wait until merge to discover test failures
6. **Document blockers**: Note any issues that prevent progress
7. **Ask for help**: Use the tracker to communicate status and blockers

## Related Skills

- `/review` - Review pull requests and generate fix tasks
- `pr-comments` - Fetch PR review comments for context

## Configuration

The skill uses these defaults (configurable):

- **Worktree location**: `../<repo-name>-<task-name>/`
- **Tracker location**: `docs/progress/<task-name>-tracker.md`
- **Test timeout**: `10m` (for E2E tests)
- **Test command**: `go test -v ./... -timeout=10m`

## Notes

- Worktrees share the same `.git` directory (efficient)
- Each worktree has independent working directory (isolated)
- Branches are visible across all worktrees
- Cleanup removes worktree but keeps branch (unless deleted)
- Always verify tests pass before merging back

---

**Version**: 1.0
**Last Updated**: 2025-10-19
