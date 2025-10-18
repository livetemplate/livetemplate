# Get GitHub PR Comments for Current Branch

Fetch and display GitHub Pull Request review comments for the current branch.

This skill will:
1. Get the current branch name
2. Find the PR number for that branch
3. Fetch all review comments from the PR
4. Display them in a readable format with file paths, line numbers, and comment text

## Usage

Invoke this skill when you need to:
- Review feedback from code reviews
- Address PR comments
- Understand what changes are requested

## Implementation

```bash
# Get current branch
BRANCH=$(git branch --show-current)
echo "Current branch: $BRANCH"

# Get PR number for current branch
PR_NUMBER=$(gh pr list --state all --head "$BRANCH" --json number --jq '.[0].number')

if [ -z "$PR_NUMBER" ]; then
    echo "No PR found for branch $BRANCH"
    exit 1
fi

echo "Found PR #$PR_NUMBER for branch $BRANCH"
echo ""
echo "Fetching review comments..."
echo ""

# Fetch PR review comments using GitHub API
gh api "/repos/{owner}/{repo}/pulls/$PR_NUMBER/comments" \
    --jq '.[] | "File: \(.path):\(.line // .original_line)\nComment: \(.body)\nAuthor: \(.user.login)\n---"'
```

## Output Format

The skill outputs comments in this format:

```
File: path/to/file.go:123
Comment: This function should handle edge cases better
Author: reviewer-username
---
```
