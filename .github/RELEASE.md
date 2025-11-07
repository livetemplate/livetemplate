# Release Process

This document describes the automated release process for the LiveTemplate Go library.

**Note:** As of v0.1.2, LiveTemplate is distributed across multiple repositories:
- **This repo** (`github.com/livetemplate/livetemplate`) - Go library only
- **Client** (`github.com/livetemplate/client`) - TypeScript client (has its own releases)
- **CLI** (`github.com/livetemplate/lvt`) - CLI tool (has its own releases)
- **Examples** (`github.com/livetemplate/examples`) - Example applications

This release script handles **only the Go library**.

## Prerequisites

### Required Tools

Install the following tools on your machine:

```bash
# macOS
brew install gh

# Or manually:
# - GitHub CLI: https://cli.github.com/manual/installation
```

### Optional Tools

For enhanced changelog generation:

```bash
brew install git-chglog
```

### Authentication

```bash
# Login to GitHub CLI
gh auth login
```

## Quick Start

### Running a Release

```bash
# From repository root
./scripts/release.sh
```

The script will:
1. Check prerequisites
2. Prompt for version bump type (patch/minor/major)
3. Show what will be released
4. Ask for confirmation
5. Execute the full release process

### Version Selection

When prompted, choose:

- **Patch** (0.1.1 → 0.1.2) - Bug fixes, small improvements
- **Minor** (0.1.1 → 0.2.0) - New features, non-breaking changes
- **Major** (0.1.1 → 1.0.0) - Breaking changes
- **Custom** - Specify exact version (e.g., 1.0.0-beta.1)

## Release Process Details

### What Happens During Release

The script performs these steps automatically:

1. **Validation**
   - Checks all required tools are installed
   - Verifies git working directory is clean
   - Validates current version

2. **Version Update**
   - Updates `VERSION` file

3. **Changelog Generation**
   - Parses git commits since last release
   - Groups by type (features, fixes, etc.)
   - Updates `CHANGELOG.md`

4. **Build & Test**
   - Runs all Go tests (`go test ./...`)
   - Verifies Go module builds (`go build ./...`)

5. **Commit & Tag**
   - Creates commit with version bump
   - Creates git tag (`v0.2.0`)

6. **Publish**
   - Pushes to GitHub (commits + tags)
   - Creates GitHub release with release notes

### Release Artifacts

After successful release:

- **GitHub Release**: https://github.com/livetemplate/livetemplate/releases
- **Go Module**: Available via `go get github.com/livetemplate/livetemplate@vX.Y.Z`

For client and CLI releases, see:
- **Client npm Package**: https://github.com/livetemplate/client
- **CLI Binaries**: https://github.com/livetemplate/lvt

## File Structure

The release system consists of:

```
.
├── VERSION                          # Single source of truth
├── scripts/
│   └── release.sh                   # Main release script
├── .chglog/
│   ├── config.yml                   # Changelog config
│   └── CHANGELOG.tpl.md             # Changelog template
└── .github/
    ├── RELEASE.md                   # This file
    └── COMMIT_CONVENTION.md         # Commit message guide
```

## Commit Convention

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation.

### Quick Reference

```bash
# Features (minor version bump)
feat(client): add new tree caching feature

# Bug fixes (patch version bump)
fix(template): resolve nil pointer error

# Breaking changes (major version bump)
feat(api)!: change session interface

BREAKING CHANGE: SessionStore now requires context parameter
```

See [COMMIT_CONVENTION.md](./COMMIT_CONVENTION.md) for detailed guide.

## Advanced Usage

### Dry Run

Test the release process without making changes:

```bash
./scripts/release.sh --dry-run
```

### Manual Version Specification

When prompted, choose option 4 (custom) to specify exact version:

```bash
Enter choice [1-4]: 4
Enter custom version (e.g., 1.2.3): 1.0.0-rc.1
```

### Using git-chglog

For better changelog generation, install git-chglog:

```bash
brew install git-chglog

# Generate changelog for next version
git-chglog --next-tag v0.2.0 -o CHANGELOG.md

# Preview without writing
git-chglog --next-tag v0.2.0
```

## Troubleshooting

### "Working directory is not clean"

Commit or stash your changes:

```bash
git status
git add .
git commit -m "feat: your changes"
```

### "Tests failed"

Fix failing tests before releasing:

```bash
go test ./... -v
```

### Release Failed After Tag Creation

If the release fails after creating the git tag, you can:

1. Fix the issue
2. Delete the tag and retry:
   ```bash
   git tag -d v0.2.0
   git push origin :refs/tags/v0.2.0
   gh release delete v0.2.0  # if release was created
   ```

## Best Practices

### Before Releasing

1. **Update Documentation**: Ensure README and docs are current
2. **Review Changes**: Check `git log` for unreleased commits
3. **Run Tests Locally**: `go test ./...` should pass
4. **Check Branch**: Ensure you're on `main` (or appropriate branch)
5. **Pull Latest**: `git pull origin main`

### After Releasing

1. **Verify GitHub Release**: Check release notes are correct
2. **Test Installation**: Try `go get github.com/livetemplate/livetemplate@vX.Y.Z` in a test project
3. **Consider Client/CLI Releases**: If API changed, may need to release client/CLI
4. **Announce**: Post to relevant channels (Discord, Twitter, etc.)
5. **Update Docs**: If you have a docs site

### Release Cadence

Suggested release schedule:

- **Patch**: As needed for critical fixes (can be daily)
- **Minor**: Weekly or bi-weekly for features
- **Major**: When breaking changes are necessary (rare)

## Security

### GitHub Token

Ensure your GitHub CLI token has necessary permissions:

```bash
gh auth status
gh auth refresh -s write:packages,write:discussion
```

## Rollback

If you need to rollback a release:

### Delete GitHub Release

```bash
gh release delete v0.2.0
git tag -d v0.2.0
git push origin :refs/tags/v0.2.0
```

### Revert Commit

```bash
git revert <commit-hash>
git push origin main
```

**Note:** Go modules are immutable once published. Users can still access old versions via `@vX.Y.Z` tags.

## CI/CD Integration (Future)

While currently manual, the release process can be automated with GitHub Actions:

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags:
      - 'v*'
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - name: Run tests
        run: go test ./... -timeout=120s
      - name: Create release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: ${{ github.ref }}
          release_name: ${{ github.ref }}
          draft: false
          prerelease: false
```

## Questions?

- **Commit Messages**: See [COMMIT_CONVENTION.md](./COMMIT_CONVENTION.md)
- **Issues**: Open an issue on GitHub
- **Client/CLI Releases**: See respective repositories

## Checklist

Before your first release, ensure:

- [ ] GitHub CLI installed and authenticated
- [ ] Git working directory clean
- [ ] On correct branch (main)
- [ ] All tests passing (`go test ./...`)
- [ ] Documentation updated

Ready? Run:

```bash
./scripts/release.sh
```

🚀 Happy releasing!
