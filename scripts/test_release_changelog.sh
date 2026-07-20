#!/usr/bin/env bash
# Tests the changelog handling in release.sh: a curated [Unreleased] section is
# promoted to the release heading, and the commit-subject fallback is used only
# when there is nothing curated to promote.
#
# Sources release.sh with RELEASE_SH_LIB=1 so the functions are defined without
# main() running.

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMPDIR=$(mktemp -d)

failures=0

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; echo "$2" | sed 's/^/      /'; failures=$((failures + 1)); }

# Source before installing the cleanup trap: release.sh registers its own EXIT
# trap at top level, which would otherwise replace ours and leak TMPDIR.
# shellcheck source=/dev/null
RELEASE_SH_LIB=1 source "$PROJECT_ROOT/scripts/release.sh"
trap 'rm -rf "$TMPDIR"' EXIT

header() {
    cat <<'EOF'
# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

EOF
}

echo "🔥 Testing release.sh changelog handling..."
cd "$TMPDIR"

echo ""
echo "1️⃣  Curated [Unreleased] is promoted, content preserved verbatim"
{
    header
    echo "## [Unreleased]"
    echo ""
    echo "### Fixed"
    echo ""
    echo "- **Something a human wrote.** With detail worth keeping."
    echo ""
    echo "## [v0.1.0] - 2026-01-01"
    echo ""
    echo "- older"
} > CHANGELOG.md
generate_changelog "9.9.9" >/dev/null 2>&1

if grep -q '^## \[v9.9.9\] - ' CHANGELOG.md; then
    pass "release heading written"
else
    fail "release heading missing" "$(head -12 CHANGELOG.md)"
fi
if ! grep -q '^## \[Unreleased\]' CHANGELOG.md; then
    pass "[Unreleased] heading consumed, not left stranded"
else
    fail "[Unreleased] survived the promotion" "$(grep -n 'Unreleased' CHANGELOG.md)"
fi
if grep -q 'Something a human wrote' CHANGELOG.md; then
    pass "curated prose preserved"
else
    fail "curated prose lost" "$(cat CHANGELOG.md)"
fi
# The regression that motivated this: a commit-subject dump appearing instead of,
# or above, the curated text.
if ! grep -qE '^- .* \([0-9a-f]{7,}\)$' CHANGELOG.md; then
    pass "no raw commit-subject dump"
else
    fail "raw commit dump present despite curated content" "$(grep -nE '^- .* \([0-9a-f]{7,}\)$' CHANGELOG.md)"
fi
if [ "$(grep -c '^## \[v9.9.9\]' CHANGELOG.md)" = "1" ] && grep -q '^## \[v0.1.0\]' CHANGELOG.md; then
    pass "prior releases still present, exactly one new heading"
else
    fail "history damaged" "$(grep -n '^## \[' CHANGELOG.md)"
fi

# Which branch generate_changelog took. Asserted on the decision rather than the
# resulting file: the fallback's output differs per repo (and with whether any
# tag exists), but the choice between promoting and regenerating does not.
took_fallback() { generate_changelog "9.9.9" 2>&1 | grep -q "falling back"; }

echo ""
echo "2️⃣  Empty [Unreleased] falls through to the commit-subject fallback"
{
    header
    echo "## [Unreleased]"
    echo ""
    echo "## [v0.1.0] - 2026-01-01"
    echo ""
    echo "- older"
} > CHANGELOG.md
if took_fallback; then
    pass "empty section not promoted to a contentless release heading"
else
    fail "promoted an empty [Unreleased]" "$(head -12 CHANGELOG.md)"
fi

echo ""
echo "3️⃣  Whitespace-only [Unreleased] counts as empty"
{
    header
    echo "## [Unreleased]"
    echo ""
    echo "   "
    echo ""
    echo "## [v0.1.0] - 2026-01-01"
} > CHANGELOG.md
if took_fallback; then
    pass "whitespace-only section treated as empty"
else
    fail "whitespace-only section was promoted" "$(head -12 CHANGELOG.md)"
fi

echo ""
echo "3️⃣ b  A curated section does NOT take the fallback"
{
    header
    echo "## [Unreleased]"
    echo ""
    echo "- real content"
    echo ""
    echo "## [v0.1.0] - 2026-01-01"
} > CHANGELOG.md
if ! took_fallback; then
    pass "promotion chosen over regeneration"
else
    fail "regenerated over curated content" "$(head -12 CHANGELOG.md)"
fi

echo ""
echo "4️⃣  unreleased_body reads only its own section"
{
    header
    echo "## [Unreleased]"
    echo ""
    echo "MINE"
    echo ""
    echo "## [v0.1.0] - 2026-01-01"
    echo ""
    echo "NOT-MINE"
} > CHANGELOG.md
body=$(unreleased_body)
if echo "$body" | grep -q 'MINE' && ! echo "$body" | grep -q 'NOT-MINE'; then
    pass "stops at the next version heading"
else
    fail "section boundary wrong" "$body"
fi

echo ""
if [ "$failures" -ne 0 ]; then
    echo "❌ $failures check(s) failed"
    exit 1
fi
echo "✅ All changelog checks passed"
