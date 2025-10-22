# Release Process

This document describes the automated release process for LiveTemplate, which manages three synchronized components:

1. **Go Library** (`github.com/livefir/livetemplate`)
2. **TypeScript Client** (`@livefir/livetemplate-client`)
3. **CLI Tool** (`lvt`)

## Prerequisites

### Required Tools

Install the following tools on your machine:

```bash
# macOS
brew install gh goreleaser npm

# Or manually:
# - GitHub CLI: https://cli.github.com/manual/installation
# - GoReleaser: https://goreleaser.com/install/
# - npm: https://nodejs.org/
```

### Optional Tools

For enhanced changelog generation:

```bash
brew install git-chglog
```

### Authentication

```bash
# Login to npm
npm login

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
   - Updates `client/package.json`
   - Ensures all components have matching version

3. **Changelog Generation**
   - Parses git commits since last release
   - Groups by type (features, fixes, etc.)
   - Updates `CHANGELOG.md`

4. **Build & Test**
   - Runs all Go tests (`go test ./...`)
   - Builds TypeScript client (`npm run build`)
   - Validates CLI builds (`go build ./cmd/lvt`)

5. **Commit & Tag**
   - Creates commit with version bump
   - Creates git tag (`v0.2.0`)

6. **Publish**
   - Publishes to npm (`@livefir/livetemplate-client`)
   - Pushes to GitHub (commits + tags)
   - Runs GoReleaser (creates GitHub release with binaries)

### Release Artifacts

After successful release:

- **npm Package**: https://www.npmjs.com/package/@livefir/livetemplate-client
- **GitHub Release**: https://github.com/livefir/livetemplate/releases
- **Go Module**: Available via `go get github.com/livefir/livetemplate@vX.Y.Z`
- **CLI Binaries**: Attached to GitHub release (macOS, Linux, Windows)

## File Structure

The release system consists of:

```
.
├── VERSION                          # Single source of truth
├── .goreleaser.yml                  # GoReleaser configuration
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

### "Not logged in to npm"

Login to npm:

```bash
npm login
# Follow prompts to authenticate
```

### "Tests failed"

Fix failing tests before releasing:

```bash
go test ./... -v
```

### "GoReleaser failed"

Check GoReleaser configuration:

```bash
goreleaser check
goreleaser release --snapshot --clean
```

### Release Created But npm Publish Failed

The git tag was created but npm publish failed. You can:

1. Fix the npm issue
2. Manually publish:
   ```bash
   cd client
   npm publish
   ```
3. Or delete the tag and retry:
   ```bash
   git tag -d v0.2.0
   git push origin :refs/tags/v0.2.0
   gh release delete v0.2.0
   ```

## Best Practices

### Before Releasing

1. **Update Documentation**: Ensure README and docs are current
2. **Review Changes**: Check `git log` for unreleased commits
3. **Run Tests Locally**: `go test ./...` should pass
4. **Check Branch**: Ensure you're on `main` (or appropriate branch)
5. **Pull Latest**: `git pull origin main`

### After Releasing

1. **Verify npm Package**: Visit npm package page
2. **Test Installation**: Try installing in a test project
3. **Check GitHub Release**: Verify binaries are attached
4. **Announce**: Post to relevant channels (Discord, Twitter, etc.)
5. **Update Docs Site**: If you have one

### Release Cadence

Suggested release schedule:

- **Patch**: As needed for critical fixes (can be daily)
- **Minor**: Weekly or bi-weekly for features
- **Major**: When breaking changes are necessary (rare)

## Security

### npm 2FA

If you have 2FA enabled on npm (recommended):

```bash
npm login --auth-type=web
```

### GitHub Token

Ensure your GitHub CLI token has necessary permissions:

```bash
gh auth status
gh auth refresh -s write:packages,write:discussion
```

## Rollback

If you need to rollback a release:

### Unpublish from npm (within 72 hours)

```bash
npm unpublish @livefir/livetemplate-client@0.2.0
```

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
      - uses: actions/setup-node@v4
      - run: npm ci
        working-directory: client
      - run: npm test
        working-directory: client
      - run: npm publish
        working-directory: client
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
      - uses: goreleaser/goreleaser-action@v5
        with:
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Questions?

- **Commit Messages**: See [COMMIT_CONVENTION.md](./COMMIT_CONVENTION.md)
- **GoReleaser**: Check [.goreleaser.yml](../.goreleaser.yml)
- **Issues**: Open an issue on GitHub

## Checklist

Before your first release, ensure:

- [ ] All prerequisites installed
- [ ] npm login completed
- [ ] GitHub CLI authenticated
- [ ] Git working directory clean
- [ ] On correct branch (main)
- [ ] All tests passing
- [ ] Documentation updated

Ready? Run:

```bash
./scripts/release.sh
```

🚀 Happy releasing!
