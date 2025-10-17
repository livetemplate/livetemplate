# Scripts

This directory contains development and automation scripts for the LiveTemplate project.

## Git Hooks

### install-hooks.sh

Installs Git hooks for the project. Run this once after cloning the repository:

```bash
./scripts/install-hooks.sh
```

This will set up:
- **pre-commit hook**: Runs before each commit to ensure code quality

### pre-commit.sh

The pre-commit validation script that runs automatically before each commit. It performs:

1. **Go code formatting** - Automatically formats Go code using `go fmt`
2. **Client tests** - Runs npm tests for the TypeScript client library
3. **Go tests** - Runs all Go tests with a 30-second timeout

If any step fails, the commit is blocked.

**Bypass (not recommended):**
```bash
git commit --no-verify
```

## Usage

After cloning the repository, run:

```bash
./scripts/install-hooks.sh
```

This ensures all contributors have the same pre-commit validation, maintaining code quality and preventing broken commits.
