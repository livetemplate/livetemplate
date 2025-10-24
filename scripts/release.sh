#!/usr/bin/env bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() { echo -e "${GREEN}✓${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }
log_step() { echo -e "${BLUE}▸${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    local missing=()

    command -v gh >/dev/null 2>&1 || missing+=("gh (GitHub CLI)")
    command -v goreleaser >/dev/null 2>&1 || missing+=("goreleaser")
    command -v npm >/dev/null 2>&1 || missing+=("npm")

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        echo ""
        echo "Install with:"
        echo "  macOS:   brew install gh goreleaser npm"
        echo "  Linux:   see https://goreleaser.com/install and https://cli.github.com/manual/installation"
        exit 1
    fi

    # Check optional tools
    if ! command -v git-chglog >/dev/null 2>&1; then
        log_warn "git-chglog not installed (optional). Install with: brew install git-chglog"
    fi
}

# Get current version
get_current_version() {
    if [ ! -f VERSION ]; then
        log_error "VERSION file not found"
        exit 1
    fi
    cat VERSION | tr -d '\n'
}

# Bump version
bump_version() {
    local current_version=$1
    local bump_type=$2

    IFS='.' read -r major minor patch <<< "$current_version"

    case $bump_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            echo "$bump_type"  # Allow custom version
            return
            ;;
    esac

    echo "${major}.${minor}.${patch}"
}

# Update all version files
update_versions() {
    local new_version=$1

    log_step "Updating VERSION file to $new_version"
    echo "$new_version" > VERSION

    log_step "Updating client/package.json to $new_version"
    # Use npm version but don't create git tag
    cd client
    npm version "$new_version" --no-git-tag-version --allow-same-version > /dev/null 2>&1
    cd ..

    log_info "All version files updated to $new_version"
}

# Generate changelog
generate_changelog() {
    local new_version=$1
    local prev_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

    log_step "Generating changelog for v$new_version"

    if command -v git-chglog >/dev/null 2>&1; then
        # Use git-chglog if available
        log_info "Using git-chglog for changelog generation"
        git-chglog --next-tag "v$new_version" -o CHANGELOG.md 2>/dev/null || {
            log_warn "git-chglog failed, keeping existing CHANGELOG.md"
        }
    else
        # Simple changelog generation
        log_warn "git-chglog not found, using simple changelog generation"

        if [ -n "$prev_tag" ]; then
            {
                echo "# Changelog"
                echo ""
                echo "All notable changes to LiveTemplate will be documented in this file."
                echo ""
                echo "The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),"
                echo "and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)."
                echo ""
                echo "## [v$new_version] - $(date +%Y-%m-%d)"
                echo ""
                echo "### Changes"
                echo ""
                git log "$prev_tag"..HEAD --pretty=format:"- %s (%h)" --no-merges | grep -v "^- Merge " || true
                echo ""
                echo ""
                tail -n +7 CHANGELOG.md 2>/dev/null || true
            } > CHANGELOG.md.tmp
            mv CHANGELOG.md.tmp CHANGELOG.md
        else
            log_warn "No previous tag found, skipping changelog generation"
        fi
    fi
}

# Commit and tag
commit_and_tag() {
    local new_version=$1

    log_step "Committing version bump"
    git add VERSION client/package.json client/package-lock.json CHANGELOG.md
    git commit -m "chore(release): v$new_version

Release v$new_version

This release includes:
- Go library (github.com/livefir/livetemplate)
- TypeScript client (@livefir/livetemplate-client)
- lvt CLI

All components versioned at v$new_version

🤖 Generated with automated release script"

    log_step "Creating git tag v$new_version"
    git tag -a "v$new_version" -m "Release v$new_version"

    log_info "Committed and tagged v$new_version"
}

# Build and test
build_and_test() {
    log_step "Running Go tests..."
    go test ./... -timeout=30s || {
        log_error "Tests failed, aborting release"
        exit 1
    }
    log_info "Go tests passed"

    log_step "Building TypeScript client..."
    cd client
    npm run build || {
        log_error "Client build failed, aborting release"
        exit 1
    }
    cd ..
    log_info "Client built successfully"

    log_step "Building CLI..."
    go build -o /tmp/lvt ./cmd/lvt || {
        log_error "CLI build failed, aborting release"
        exit 1
    }
    log_info "CLI built successfully"
}

# Publish to npm
publish_npm() {
    local new_version=$1

    log_step "Publishing @livefir/livetemplate-client@$new_version to npm"
    cd client

    # Check if logged in
    if ! npm whoami >/dev/null 2>&1; then
        log_error "Not logged in to npm. Run 'npm login' first"
        cd ..
        exit 1
    fi

    # Publish
    npm publish || {
        log_error "npm publish failed"
        cd ..
        exit 1
    }
    cd ..

    log_info "Published to npm: https://www.npmjs.com/package/@livefir/livetemplate-client/v/$new_version"
}

# Push and create GitHub release
publish_github() {
    local new_version=$1

    log_step "Pushing commits and tags to GitHub"
    git push origin main || git push origin master || {
        log_error "Failed to push to origin. Check your branch name."
        exit 1
    }
    git push origin "v$new_version"
    log_info "Pushed to GitHub"

    log_step "Creating GitHub release with GoReleaser"
    goreleaser release --clean || {
        log_error "GoReleaser failed"
        exit 1
    }

    log_info "GitHub release created: https://github.com/livefir/livetemplate/releases/tag/v$new_version"
}

# Dry run mode
dry_run() {
    local new_version=$1

    echo ""
    echo "🔍 DRY RUN MODE - No changes will be made"
    echo "========================================"
    echo ""

    log_info "Would update VERSION to: $new_version"
    log_info "Would update client/package.json to: $new_version"
    log_info "Would generate CHANGELOG.md"
    log_info "Would run tests and builds"
    log_info "Would commit with message: chore(release): v$new_version"
    log_info "Would create tag: v$new_version"
    log_info "Would publish to npm"
    log_info "Would push to GitHub and create release with GoReleaser"

    echo ""
    log_info "Dry run completed successfully"
    exit 0
}

# Main release function
main() {
    local dry_run_mode=false

    # Parse flags
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                dry_run_mode=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    echo "🚀 LiveTemplate Release Automation"
    echo "===================================="
    echo ""

    check_prerequisites

    # Check git status
    if [ -n "$(git status --porcelain)" ]; then
        log_error "Working directory is not clean. Commit or stash changes first."
        echo ""
        git status --short
        exit 1
    fi

    # Get current version
    current_version=$(get_current_version)
    log_info "Current version: $current_version"

    # Ask for version bump type
    echo ""
    echo "Select version bump type:"
    echo "  1) patch (bug fixes)        → $(bump_version "$current_version" patch)"
    echo "  2) minor (new features)     → $(bump_version "$current_version" minor)"
    echo "  3) major (breaking changes) → $(bump_version "$current_version" major)"
    echo "  4) custom version"
    echo ""
    read -rp "Enter choice [1-4]: " choice

    case $choice in
        1) new_version=$(bump_version "$current_version" patch) ;;
        2) new_version=$(bump_version "$current_version" minor) ;;
        3) new_version=$(bump_version "$current_version" major) ;;
        4)
            read -rp "Enter custom version (e.g., 1.2.3): " new_version
            if ! [[ $new_version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                log_error "Invalid version format. Must be X.Y.Z"
                exit 1
            fi
            ;;
        *)
            log_error "Invalid choice"
            exit 1
            ;;
    esac

    echo ""
    log_info "New version will be: $new_version"
    echo ""
    echo "This will:"
    echo "  • Update VERSION, client/package.json, go.mod"
    echo "  • Generate/update CHANGELOG.md"
    echo "  • Run all tests and builds"
    echo "  • Commit and tag v$new_version"
    echo "  • Publish to npm (@livefir/livetemplate-client)"
    echo "  • Create GitHub release (Go library + lvt CLI binaries)"
    echo ""

    if [ "$dry_run_mode" = true ]; then
        dry_run "$new_version"
    fi

    read -rp "Continue? [y/N]: " confirm

    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        log_warn "Release cancelled"
        exit 0
    fi

    echo ""
    log_info "Starting release process..."
    echo ""

    # Execute release steps
    update_versions "$new_version"
    generate_changelog "$new_version"
    build_and_test
    commit_and_tag "$new_version"
    publish_npm "$new_version"
    publish_github "$new_version"

    echo ""
    echo "================================================"
    log_info "✨ Release v$new_version completed successfully!"
    echo "================================================"
    echo ""
    echo "📦 Published artifacts:"
    echo "  • npm:    https://www.npmjs.com/package/@livefir/livetemplate-client/v/$new_version"
    echo "  • GitHub: https://github.com/livefir/livetemplate/releases/tag/v$new_version"
    echo "  • Go:     go get github.com/livefir/livetemplate@v$new_version"
    echo ""
    echo "📝 Next steps:"
    echo "  • Verify the npm package"
    echo "  • Test the GitHub release binaries"
    echo "  • Announce the release"
}

main "$@"
