# Scripts Directory

This directory contains utility scripts for development workflow automation and Git hooks.

## 📁 Scripts Overview

### 🧪 `validate-tests.sh` - Test Validation Script
**Purpose:** Runs Go tests and validates code quality

**Usage:**
```bash
# Run manually
./scripts/validate-tests.sh

# Or from anywhere in the project
scripts/validate-tests.sh
```

**Features:**
- ✅ **Automatic Go project detection** - Checks for `go.mod`
- 📍 **Smart path handling** - Works from any directory in the project
- 🔍 **Comprehensive testing** - Runs `go test ./...` for all packages
- 💡 **Helpful error messages** - Provides tips when tests fail
- 🛡️ **Error handling** - Proper exit codes for CI/CD integration

### 🔧 `install-git-hooks.sh` - Git Hook Installer
**Purpose:** Installs Git pre-commit hooks for automated testing

**Usage:**
```bash
# Install Git hooks (run once per repository)
./scripts/install-git-hooks.sh
```

**What it does:**
- 🎯 **Creates pre-commit hook** - Automatically runs tests before commits
- 🔗 **Links to validation script** - Uses `validate-tests.sh` for actual testing
- 🛡️ **Error checking** - Validates Git repository and script existence
- 📋 **Clear feedback** - Shows exactly what was installed

## 🚀 Quick Setup for New Contributors

```bash
# 1. Clone the repository
git clone <repository-url>
cd <repository-name>

# 2. Install Git hooks (one-time setup)
./scripts/install-git-hooks.sh

# 3. Test the setup
./scripts/validate-tests.sh
```

## 🎯 How It Works

### Manual Testing
```bash
# Test your changes manually before committing
./scripts/validate-tests.sh
```

### Automatic Testing (via Git Hook)
```bash
# Git will automatically run tests when you commit
git commit -m "your changes"
# → Tests run automatically
# → Commit proceeds only if tests pass
```

## 📊 Integration Benefits

| Script | Use Case | When It Runs | Benefits |
|--------|----------|--------------|----------|
| `validate-tests.sh` | Manual testing | On demand | Quick feedback during development |
| Git pre-commit hook | Automatic testing | Every `git commit` | Prevents broken code from being committed |

## 🔧 Customization

### Adding More Validations
Edit `validate-tests.sh` to add additional checks:
```bash
# Example: Add linting
if ! golangci-lint run; then
    echo "❌ Linting failed!"
    exit 1
fi

# Example: Add formatting check
if ! gofmt -l . | grep -q .; then
    echo "❌ Code formatting issues found!"
    exit 1
fi
```

### Modifying Hook Behavior
The Git hook automatically calls `validate-tests.sh`, so any changes to the validation script will be reflected in the Git hook behavior.

## 🛠️ Troubleshooting

### Hook Not Running
```bash
# Check if hook is installed and executable
ls -la .git/hooks/pre-commit

# Reinstall if needed
./scripts/install-git-hooks.sh
```

### Tests Failing
```bash
# Run tests manually to see detailed output
./scripts/validate-tests.sh

# Run with verbose output
go test -v ./...
```

### Bypass Hook (Emergency Only)
```bash
# Skip pre-commit hook (not recommended)
git commit --no-verify -m "emergency fix"
```

## 🎉 Benefits Summary

- ✅ **Quality Assurance** - Prevents broken code from being committed
- 🤝 **Team Consistency** - Everyone uses the same validation process
- ⚡ **Developer Efficiency** - Automated testing without manual steps
- 🔄 **CI/CD Integration** - Scripts work both locally and in CI pipelines
- 📈 **Maintainable** - Centralized validation logic that's easy to update
