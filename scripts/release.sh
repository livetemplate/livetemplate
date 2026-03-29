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

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        echo ""
        echo "Install with:"
        echo "  macOS:   brew install gh"
        echo "  Linux:   see https://cli.github.com/manual/installation"
        exit 1
    fi

    # Check GitHub CLI auth
    if ! gh auth status >/dev/null 2>&1; then
        log_error "GitHub CLI not authenticated. Run 'gh auth login' first"
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

    log_info "VERSION file updated to $new_version"
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
    git add VERSION CHANGELOG.md
    git commit -m "chore(release): v$new_version

Release v$new_version

Go library: github.com/livetemplate/livetemplate@v$new_version

Related repositories:
- CLI Tool: https://github.com/livetemplate/lvt
- Client Library: https://github.com/livetemplate/client
- Examples: https://github.com/livetemplate/examples

🤖 Generated with automated release script"

    log_step "Creating git tag v$new_version"
    git tag -a "v$new_version" -m "Release v$new_version"

    log_info "Committed and tagged v$new_version"
}

# Build and test
build_and_test() {
    log_step "Running Go tests..."
    GOWORK=off go test ./... -timeout=120s || {
        log_error "Tests failed, aborting release"
        exit 1
    }
    log_info "Go tests passed"

    log_step "Verifying Go module..."
    GOWORK=off go build ./... || {
        log_error "Build failed, aborting release"
        exit 1
    }
    log_info "Go module builds successfully"
}

# Extract release notes from CHANGELOG
extract_release_notes() {
    local new_version=$1
    local notes_file="/tmp/release-notes-$new_version.md"

    if [ ! -f CHANGELOG.md ]; then
        log_warn "CHANGELOG.md not found, using default release notes"
        echo "Release v$new_version" > "$notes_file"
        echo "" >> "$notes_file"
        echo "LiveTemplate Go library release." >> "$notes_file"
        echo "$notes_file"
        return
    fi

    # Extract notes for this version from CHANGELOG
    # Look for the version header and extract until next version or end
    awk -v ver="$new_version" '
        /^## \[v/ {
            if (found) exit
            if ($0 ~ "\\[v"ver"\\]") {
                found=1
                next
            }
        }
        found && /^## \[v/ { exit }
        found { print }
    ' CHANGELOG.md > "$notes_file"

    # If empty, add default content
    if [ ! -s "$notes_file" ]; then
        log_warn "No changelog entries found for v$new_version, using default notes"
        echo "Release v$new_version" > "$notes_file"
        echo "" >> "$notes_file"
        echo "LiveTemplate Go library release." >> "$notes_file"
    fi

    # Add installation instructions
    {
        echo ""
        echo "## Installation"
        echo ""
        echo "\`\`\`bash"
        echo "go get github.com/livetemplate/livetemplate@v$new_version"
        echo "\`\`\`"
        echo ""
        echo "## Related Repositories"
        echo ""
        echo "- **CLI Tool**: [livetemplate/lvt](https://github.com/livetemplate/lvt)"
        echo "- **Client Library**: [livetemplate/client](https://github.com/livetemplate/client)"
        echo "- **Examples**: [livetemplate/examples](https://github.com/livetemplate/examples)"
    } >> "$notes_file"

    echo "$notes_file"
}

# Push and create GitHub release
publish_github() {
    local new_version=$1

    local branch
    branch=$(git rev-parse --abbrev-ref HEAD)

    log_step "Pushing commits and tags to GitHub (branch: $branch)"
    git push origin "$branch" || {
        log_error "Failed to push to origin. You may need to 'git pull --rebase origin $branch' first."
        exit 1
    }
    git push origin "v$new_version"
    log_info "Pushed to GitHub"

    # Extract release notes
    log_step "Extracting release notes from CHANGELOG"
    local notes_file=$(extract_release_notes "$new_version")
    log_info "Release notes prepared"

    # Create GitHub release with gh CLI
    log_step "Creating GitHub release v$new_version"
    gh release create "v$new_version" \
        --title "v$new_version" \
        --notes-file "$notes_file" || {
        log_error "Failed to create GitHub release"
        exit 1
    }

    # Cleanup
    rm -f "$notes_file"

    log_info "GitHub release created: https://github.com/livetemplate/livetemplate/releases/tag/v$new_version"
}

# Dry run mode
dry_run() {
    local new_version=$1

    echo ""
    echo "🔍 DRY RUN MODE - No changes will be made"
    echo "========================================"
    echo ""

    log_info "Would update VERSION to: $new_version"
    log_info "Would generate CHANGELOG.md"
    log_info "Would run Go tests"
    log_info "Would verify Go module builds"
    log_info "Would commit with message: chore(release): v$new_version"
    log_info "Would create tag: v$new_version"
    log_info "Would push to GitHub and create release"

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

    # Pull latest from remote
    local branch
    if ! branch=$(git symbolic-ref --short HEAD 2>/dev/null); then
        log_error "Repository is in a detached HEAD state. Please check out a branch before running this script."
        exit 1
    fi
    if [ "$dry_run_mode" = true ]; then
        log_info "[dry-run] Would pull latest from origin/$branch"
    else
        log_step "Pulling latest changes from origin/$branch"
        git pull --rebase origin "$branch" || {
            log_error "Failed to pull latest changes. Resolve conflicts and try again."
            exit 1
        }
        log_info "Up to date with origin/$branch"
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
    echo "  • Update VERSION file"
    echo "  • Generate/update CHANGELOG.md"
    echo "  • Run Go tests and verify build"
    echo "  • Commit and tag v$new_version"
    echo "  • Push to GitHub and create release"
    echo ""
    echo "Note: This repo only releases the Go library."
    echo "For client/CLI releases, see:"
    echo "  • https://github.com/livetemplate/client"
    echo "  • https://github.com/livetemplate/lvt"
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
    publish_github "$new_version"

    echo ""
    echo "================================================"
    log_info "✨ Release v$new_version completed successfully!"
    echo "================================================"
    echo ""
    echo "📦 Released:"
    echo "  • GitHub: https://github.com/livetemplate/livetemplate/releases/tag/v$new_version"
    echo "  • Go:     go get github.com/livetemplate/livetemplate@v$new_version"
    echo ""
    echo "📝 Next steps:"
    echo "  • Verify the release on GitHub"
    echo "  • Test installation: go get github.com/livetemplate/livetemplate@v$new_version"
    echo "  • Consider releasing client/CLI if needed"
    echo "  • Announce the release"
}

main "$@"
