#!/usr/bin/env bash
# Tests the client-pin guard in release.sh: version ordering, reading the pin
# out of client_assets.go, and the decision the guard makes for each case.
#
# Sources release.sh with RELEASE_SH_LIB=1 so the functions are defined without
# main() running. No network — check_client_pin's lookup is stubbed.

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

echo "🔥 Testing release.sh client-pin guard..."

echo ""
echo "1️⃣  version_order"
check_order() {
    local got
    got=$(version_order "$1" "$2")
    if [ "$got" = "$3" ]; then
        pass "$1 vs $2 → $3"
    else
        fail "$1 vs $2 → $got (want $3)" ""
    fi
}
check_order "0.20.0" "0.20.0" same
check_order "0.18.2" "0.20.0" older
check_order "0.20.0" "0.18.2" newer
# Numeric, not lexicographic — the case a plain string compare gets wrong.
check_order "0.9.0"  "0.10.0" older
check_order "0.20.9" "0.20.10" older
check_order "1.0.0"  "0.99.99" newer

echo ""
echo "2️⃣  read_client_pin"
cd "$TMPDIR"
cat > client_assets.go <<'EOF'
package livetemplate

// ClientVersion is the version of the @livetemplate/client browser bundle.
const ClientVersion = "0.20.0"

const clientCDNBase = "https://cdn.jsdelivr.net/npm/@livetemplate/client@" + ClientVersion
EOF
got=$(read_client_pin)
if [ "$got" = "0.20.0" ]; then
    pass "reads the constant"
else
    fail "read_client_pin → '$got' (want 0.20.0)" ""
fi

# The godoc above the constant mentions other versions in prose in the real
# file; the anchor is the assignment line, not the first version-shaped string.
cat > client_assets.go <<'EOF'
package livetemplate

// Bumped from 0.18.2; see the 0.19.1 release notes for why 9.9.9 is not used.
const ClientVersion = "0.20.0"
EOF
got=$(read_client_pin)
if [ "$got" = "0.20.0" ]; then
    pass "ignores versions mentioned in surrounding comments"
else
    fail "read_client_pin → '$got' (want 0.20.0)" ""
fi

echo ""
echo "3️⃣  latest_client_release selection"
# gh is stubbed to emit tag lines; the flags it would pass to filter drafts and
# pre-releases are bypassed by the stub, so these assert the shell-side safety
# net (end-anchored bare-semver filter + true semver max), which is what runs
# regardless of whether the flags took effect.

# Ordered by creation time, gh could return an older line last; the max must win
# over list order.
gh() { printf 'v0.19.1\nv0.20.0\nv0.18.2\n'; }
got=$(latest_client_release)
if [ "$got" = "0.20.0" ]; then
    pass "picks the semver max, not the list order"
else
    fail "latest_client_release → '$got' (want 0.20.0)" ""
fi

# 0.10.0 must outrank 0.9.0 — the case a lexicographic max gets wrong.
gh() { printf 'v0.9.0\nv0.10.0\n'; }
got=$(latest_client_release)
if [ "$got" = "0.10.0" ]; then
    pass "0.10.0 outranks 0.9.0 (numeric, not lexicographic)"
else
    fail "latest_client_release → '$got' (want 0.10.0)" ""
fi

# A pre-release-suffixed tag that slips past the --exclude flag is dropped by the
# end-anchored filter rather than fed into the comparison.
gh() { printf 'v0.21.0-rc.1\nv0.20.0\n'; }
got=$(latest_client_release)
if [ "$got" = "0.20.0" ]; then
    pass "a -rc pre-release is excluded, not treated as newest"
else
    fail "latest_client_release → '$got' (want 0.20.0)" ""
fi

# Nothing usable → empty, which check_client_pin turns into a SKIP.
gh() { printf ''; }
got=$(latest_client_release)
if [ -z "$got" ]; then
    pass "no usable tags → empty"
else
    fail "latest_client_release → '$got' (want empty)" ""
fi
unset -f gh

echo ""
echo "4️⃣  check_client_pin decisions"
# Stub the lookup rather than hitting the network. Redefining gh keeps
# check_client_pin itself under test, including its ${latest#v} handling.
run_guard() {
    local pinned=$1
    # Deliberately NOT a local named `latest`: check_client_pin declares its own
    # `local latest`, and bash's dynamic scoping means the stub would read that
    # (empty, mid-assignment) instead of ours. A global name cannot be shadowed.
    STUB_LATEST=$2
    cat > client_assets.go <<EOF
package livetemplate

const ClientVersion = "$pinned"
EOF
    gh() { echo "v$STUB_LATEST"; }
    guard_result
}

# Run check_client_pin and emit its output followed by EXIT:<status>. Cannot be
# a subshell ending in `echo "EXIT:$?"`: check_client_pin calls `exit 1`, which
# terminates the subshell before that echo ever runs — the status would be lost
# and `set -e` would kill the harness at the assignment.
guard_result() {
    local out rc
    set +e
    out=$(check_client_pin 2>&1)
    rc=$?
    set -e
    printf '%s\nEXIT:%d\n' "$out" "$rc"
}

out=$(run_guard "0.20.0" "0.20.0")
if [[ "$out" == *"EXIT:0"* && "$out" == *"Client pin is current"* ]]; then
    pass "matching pin passes"
else
    fail "matching pin should pass" "$out"
fi

out=$(run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:1"* && "$out" == *"behind"* ]]; then
    pass "stale pin blocks the release"
else
    fail "stale pin should block" "$out"
fi
# The regression this guard exists for, named explicitly: 0.18.2 pinned while
# 0.20.0 was published is exactly what shipped between #452 and #515.
if [[ "$out" == *"0.18.2"* && "$out" == *"0.20.0"* ]]; then
    pass "message names both versions"
else
    fail "message should name both versions" "$out"
fi

out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=1 run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:0"* && "$out" == *"proceeding"* ]]; then
    pass "override lets a deliberate lag through"
else
    fail "override should allow a stale pin" "$out"
fi

out=$(run_guard "0.21.0" "0.20.0")
if [[ "$out" == *"EXIT:1"* && "$out" == *"ahead"* ]]; then
    pass "unpublished pin blocks the release"
else
    fail "unpublished pin should block" "$out"
fi

out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=1 run_guard "0.21.0" "0.20.0")
if [[ "$out" == *"EXIT:1"* ]]; then
    pass "override does NOT excuse an unpublished pin"
else
    fail "override must not bypass the 'newer' case — that URL 404s for everyone" "$out"
fi

# An empty release list makes `gh ... --jq '.[0].tagName'` emit the literal
# string "null". It is non-empty, survives the "v" strip, and sorts BELOW any
# real version — so a bare -z check would treat it as a valid answer and refuse
# the release citing "Latest: null". Blocking on garbage is worse than skipping.
out=$(run_guard "0.20.0" "null")
if [[ "$out" == *"EXIT:0"* && "$out" == *"SKIPPED"* ]]; then
    pass "a 'null' version is treated as no answer, not as a stale pin"
else
    fail "'null' must skip, not block" "$out"
fi

out=$(run_guard "0.20.0" "not-a-version")
if [[ "$out" == *"EXIT:0"* && "$out" == *"SKIPPED"* ]]; then
    pass "unparseable version skips rather than comparing"
else
    fail "garbage version must skip" "$out"
fi

# Without a working `sort -V`, sort still succeeds and returns a lexicographic
# order — silently wrong, and inside a [ ] test that `set -e` does not trap. The
# probe must notice. Simulated by stripping -V from the arguments.
# Drop -V entirely rather than "${@/-V/}", which substitutes an empty string but
# leaves it as an argument — GNU sort then reads "" as a filename and errors,
# which happens to satisfy the probe for the wrong reason and leaks stderr into
# the captured output. Filtering gives a real lexicographic sort of stdin.
sort() {
    local a args=()
    for a in "$@"; do [ "$a" = "-V" ] || args+=("$a"); done
    command sort "${args[@]}"
}
out=$(run_guard "0.9.0" "0.10.0")
unset -f sort
if [[ "$out" == *"EXIT:0"* && "$out" == *"SKIPPED"* && "$out" == *"-V"* ]]; then
    pass "a sort(1) without -V skips instead of comparing wrongly"
else
    fail "missing sort -V must skip, not silently misorder" "$out"
fi

# An unreadable pin must stop the release outright: without a version there is
# nothing to compare, and proceeding would ship whatever the constant now says.
cat > client_assets.go <<'EOF'
package livetemplate

const (
    ClientVersion = "0.20.0"
)
EOF
out=$(guard_result)
if [[ "$out" == *"EXIT:1"* && "$out" == *"Could not read ClientVersion"* ]]; then
    pass "a reformatted const block stops the release rather than passing"
else
    fail "unreadable pin must exit 1" "$out"
fi

# The dangerous direction: a value meaning "off" must not enable the bypass.
out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=false run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:1"* && "$out" == *"behind"* ]]; then
    pass "LVT_ALLOW_CLIENT_PIN_DRIFT=false does NOT bypass the guard"
else
    fail "a 'false' override must not let a stale pin through" "$out"
fi

out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=0 run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:1"* && "$out" == *"behind"* ]]; then
    pass "LVT_ALLOW_CLIENT_PIN_DRIFT=0 does NOT bypass the guard"
else
    fail "a '0' override must not let a stale pin through" "$out"
fi

out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=true run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:0"* && "$out" == *"proceeding"* ]]; then
    pass "LVT_ALLOW_CLIENT_PIN_DRIFT=true is honoured, not silently ignored"
else
    fail "'true' should work as an override" "$out"
fi

out=$(LVT_ALLOW_CLIENT_PIN_DRIFT=banana run_guard "0.18.2" "0.20.0")
if [[ "$out" == *"EXIT:1"* && "$out" == *"neither true nor false"* ]]; then
    pass "an unparseable override errors rather than guessing"
else
    fail "unparseable override must not be assumed either way" "$out"
fi

# A lookup failure is not a pass. Blocking on a GitHub hiccup would be worse
# than proceeding, but the output has to say the check did not run.
cat > client_assets.go <<'EOF'
package livetemplate

const ClientVersion = "0.20.0"
EOF
gh() { return 1; }
out=$(guard_result)
if [[ "$out" == *"EXIT:0"* && "$out" == *"SKIPPED"* ]]; then
    pass "unreachable lookup skips loudly rather than passing silently"
else
    fail "lookup failure should warn SKIPPED and continue" "$out"
fi

echo ""
if [ "$failures" -ne 0 ]; then
    echo "❌ $failures check(s) failed"
    exit 1
fi
echo "✅ All client-pin checks passed"
