# Documentation Overhaul — GitHub Issues

Copy each issue below into GitHub. Work through them in order (Batch 1 first).

Each issue is designed to be completed in a single Claude Code session. Start each session with:

> Execute the documentation overhaul batch described in issue #N. Reference `docs/DOCUMENTATION_OVERHAUL_PLAN.md` for full details. Do a thorough line-by-line audit — don't trust the accuracy estimates, verify everything against source code. Commit and push when done.

---

## Issue 1

**Title:** docs: Batch 1 — Archive 23 completed planning artifacts

**Labels:** `documentation`

**Body:**

```
## Summary

Move 23 completed/implemented/superseded docs into `docs/archive/` to declutter active documentation.

## Context

See `docs/DOCUMENTATION_OVERHAUL_PLAN.md` → Batch 1 for the full file list and reasons.

## Files to move

### → `docs/archive/plans/`
- `plans/2024-12-06-controller-state-pattern.md` — Implemented in v0.7.0
- `plans/upload-feature-implementation.md` — All 9 phases complete
- `plans/2025-11-02-lvt-gen-stack-design.md` — Feature complete
- `plans/2025-11-02-lvt-gen-stack-implementation.md` — Duplicate, feature complete
- `plans/2025-11-01-lvt-gen-auth.md` — Implemented in livetemplate/lvt repo
- `plans/2025-11-10-performance-benchmarks.md` — Fully implemented (benchmarks, CI, baseline all exist)

### → `docs/archive/proposals/`
- `proposals/002-controller-state-pattern.md` — Implemented in v0.7.0
- `proposals/async-websocket-sends.md` — Implemented
- `proposals/authentication-v0.5.md` — Implemented in v0.4.1
- `proposals/value-deduplication-proposal.md` — Declined
- `proposals/store-pattern-redesign.md` — Superseded by Controller+State
- `proposals/bug-fix-persistence-and-design-improvements.md` — Superseded by v0.7.0

### → `docs/archive/implementation-plans/`
- `implementation-plans/IMPLEMENTATION_STATUS.md` — Phases 1-7 complete
- `implementation-plans/REFACTORING_PROGRESS.md` — All 5 phases complete
- `implementation-plans/REFACTOR_STRIP_STATICS.md` — Investigation complete
- `implementation-plans/BUG-VALIDATION-CONDITIONALS.md` — Bugs fixed
- `implementation-plans/TESTING_FRAMEWORK_PROGRESS.md` — 15/15 sessions complete
- `implementation-plans/implementation-readiness.md` — Pre-implementation, feature underway

### → `docs/archive/`
- `IMPLEMENTATION_COMPLETE.md` — Historical completion report
- `lvt-gen-stack-implementation.md` — Duplicate plan
- `SIMPLIFIED_DIFF_PROPOSAL.md` — Implemented in v0.8.0
- `ARCHITECTURAL_REVIEW.md` — All recommendations implemented

## After moving

- Update `docs/README.md` to remove archived files from active sections
- Ensure Archive section in README lists the newly archived docs
- Mark Batch 1 as done in `docs/DOCUMENTATION_OVERHAUL_PLAN.md`

## Acceptance criteria

- [ ] All 23 files moved to appropriate `docs/archive/` subdirectories
- [ ] `docs/README.md` updated (no broken links to archived files)
- [ ] No other docs have broken links to the moved files
- [ ] Batch 1 marked complete in overhaul plan
```

---

## Issue 2

**Title:** docs: Batch 2 — Regenerate core architecture docs from codebase

**Labels:** `documentation`

**Body:**

```
## Summary

Thoroughly audit and regenerate the 3 core architecture docs by cross-referencing against actual source code. These are the highest-value docs and have the most significant inaccuracies.

## Context

See `docs/DOCUMENTATION_OVERHAUL_PLAN.md` → Batch 2 for full details.

## Files to update

### `docs/ARCHITECTURE.md`
- Regenerate all `internal/*/` file listings from `find internal/ -name '*.go' ! -name '*_test.go'`
- Fix `internal/send/` section (references wrong filenames)
- Add missing packages: `upload/`, `uploadtypes/`, `discovery/`, `compat/`, `fuzz/`, `testutil/`, `util/`
- Add `internal/session/limits.go`

### `docs/CODE_STRUCTURE.md`
- Regenerate line counts from `wc -l` (currently off by 3-5x)
- Remove references to non-existent files
- Add all missing internal packages
- Update `mount.go` description for current API

### `docs/ROADMAP.md`
- Update M1 and M2 tasks to ✅ DONE (currently shown as in-progress)
- Update progress bars and percentages (currently 66%, actually ~80%+)

## Instructions for the session

Do NOT trust the "known issues" list above — it was generated with token limits. Read every section of each file line-by-line and verify against actual source. There are likely additional issues not listed here.

## Acceptance criteria

- [ ] Every file/package referenced in ARCHITECTURE.md exists in the codebase
- [ ] Line counts in CODE_STRUCTURE.md are within 10% of actual `wc -l` output
- [ ] No references to non-existent files in any of the 3 docs
- [ ] ROADMAP.md percentages reflect actual completion status
- [ ] Batch 2 marked complete in overhaul plan
```

---

## Issue 3

**Title:** docs: Batch 3 — Audit and fix all guides line-by-line

**Labels:** `documentation`

**Body:**

```
## Summary

Thorough line-by-line audit of all 3 guides. The initial audit rated these too generously — expect to find issues beyond what's listed below.

## Context

See `docs/DOCUMENTATION_OVERHAUL_PLAN.md` → Batch 3 for full details.

## Files to update

### `docs/guides/new-contributor-walkthrough.md` (~1300 lines)

Known issues (likely incomplete):
- Broken file refs: `internal/session/connection.go`, `internal/observe/logger.go` don't exist
- Repo structure diagram (lines 83-100): lists `client/`, `examples/` that don't exist; missing 7 internal packages
- Go version (line 58): claims "Go 1.22+" but `go.mod` requires Go 1.26.0
- `render.Node()` signature (line 635): wrong
- Missing references to: `parse/flatten.go`, `parse/types.go`, `build/html_diff.go`, `build/html_segmentation.go`

### `docs/guides/auth-customization.md`
- Assumes `lvt gen auth` exists here (implemented in lvt repo)
- Reframe as manual auth customization guide

### `docs/guides/lvt-cli-guide.md`
- Documents CLI features from separate repo
- Add status indicators for what's implemented vs planned
- Note CLI source is in livetemplate/lvt

## Instructions for the session

Read EVERY line of the walkthrough. Verify EVERY file path resolves. Check EVERY code example is conceptually correct. The initial audit only spot-checked and missed issues.

## Acceptance criteria

- [ ] Every file reference in the walkthrough resolves to an actual file
- [ ] Every code example uses correct function signatures
- [ ] Go version matches `go.mod`
- [ ] Repo structure diagram matches reality
- [ ] Auth guide doesn't assume `lvt gen auth` exists in this repo
- [ ] CLI guide clearly indicates which features are in the lvt repo
- [ ] Batch 3 marked complete in overhaul plan
```

---

## Issue 4

**Title:** docs: Batch 4 — Update configuration and reference docs

**Labels:** `documentation`

**Body:**

```
## Summary

Verify 11 documentation files against actual source code. Update configuration docs with missing env vars, mark unimplemented client attributes, and clean up CLI command references.

## Context

See `docs/DOCUMENTATION_OVERHAUL_PLAN.md` → Batch 4 for full details.

## Files to update

### `docs/CONFIGURATION.md`
- Add missing `LVT_WS_BUFFER_SIZE` env var (WebSocket buffer, default 50)
- Add missing `LVT_PROGRESSIVE_ENHANCEMENT` env var
- Add Redis configuration documentation
- Cross-reference all env vars against actual config loading code

### `docs/SCALING.md`
- Update milestone completion status
- Review for content that can be trimmed (currently 65.3KB)

### `docs/references/client-attributes.md`
- Audit against actual TypeScript client
- Mark unimplemented attributes as "Planned"

### `docs/references/api-reference.md`
- Remove or mark unimplemented CLI commands (e.g., `lvt auth`)

### `docs/references/authentication.md`
- Verify `BasicAuthenticator` against code

### Files to verify need no changes:
- `docs/OBSERVABILITY.md`
- `docs/references/controller-pattern.md`
- `docs/references/error-handling.md`
- `docs/references/server-actions.md`
- `docs/references/session.md`
- `docs/references/template-support-matrix.md`

## Acceptance criteria

- [ ] Every env var in CONFIGURATION.md exists in config loading code
- [ ] No missing env vars that exist in code but not in docs
- [ ] Client attributes accurately reflect implementation status
- [ ] No references to unimplemented CLI commands without "Planned" labels
- [ ] All 6 "no changes" files verified as actually accurate
- [ ] Batch 4 marked complete in overhaul plan
```

---

## Issue 5

**Title:** docs: Batch 5 — Specs, design, performance, and CLAUDE.md

**Labels:** `documentation`

**Body:**

```
## Summary

Final batch: verify 12 files covering specifications, design docs, performance docs, and CLAUDE.md. Archive the unimplemented fuzz framework design. Update CLAUDE.md to remove stale CLI references.

## Context

See `docs/DOCUMENTATION_OVERHAUL_PLAN.md` → Batch 5 for full details.

## Files to update

### `docs/specifications/test-scenarios.md`
- Add header clarifying this is a test specification, not implementation status
- Cross-reference with actual test files

### `docs/fuzz-framework-design.md`
- Archive to `docs/archive/` (framework not implemented, `internal/fuzz/` has only empty dirs)
- Actual property testing is in `internal/diff/diff_property_test.go`

### `CLAUDE.md`
- Remove CLI Tool section that references `cmd/lvt/` (doesn't exist in this repo)
- Update internal package descriptions to match reality
- Verify all referenced types and function signatures still exist

### Files to verify need no changes:
- `docs/specifications/tree-update-specification.md`
- `docs/specifications/tree-update-fallback-addendum.md`
- `docs/performance/performance-characteristics.md`
- `docs/performance/benchmarking-guide.md`
- `docs/performance/known-bottlenecks.md`
- `docs/performance/2025-11-10-benchmarking-system-design.md`
- `docs/design/FIRST_PRINCIPLES.md`
- `docs/design/multi-session-isolation.md`
- `docs/uploads.md`

## Acceptance criteria

- [ ] Fuzz framework design archived
- [ ] CLAUDE.md has no references to files that don't exist in this repo
- [ ] Test scenarios doc clearly labeled as specification
- [ ] All 9 "no changes" files verified as actually accurate
- [ ] Batch 5 marked complete in overhaul plan
- [ ] `docs/DOCUMENTATION_OVERHAUL_PLAN.md` shows all batches complete
```
