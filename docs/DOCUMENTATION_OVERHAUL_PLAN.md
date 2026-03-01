# Documentation Overhaul Plan

**Date:** 2026-02-28
**Branch:** `claude/overhaul-documentation-1TcqZ`
**Scope:** All 65 documentation files across the LiveTemplate repository

---

## How to Use This Plan

This plan is designed to be **executed across multiple sessions**. Each session should pick up one batch, do thorough line-by-line verification against the actual codebase, make fixes, and check off the batch. Do not trust the accuracy ratings below blindly — they are estimates from initial audits that had token limitations and have already been found to contain errors.

**Per-session workflow:**
1. Pick the next unchecked batch
2. Read every line of each file in the batch
3. Cross-reference every claim against actual source code
4. Make fixes and commit
5. Mark the batch as done in this file

---

## Motivation

The documentation has not been systematically updated since ~v0.7.0. The codebase is now at v0.8.3 with significant changes: the 5-phase refactoring is complete, all M1/M2 roadmap milestones are done, many proposals have been implemented, and the repository was split (CLI moved to `livetemplate/lvt`). As a result:

- **6 broken links** in docs/README.md (now fixed in prior commit)
- **43+ files** were completely unindexed (now fixed in prior commit)
- **ROADMAP.md** says 66% complete when M1/M2 are actually 100% done
- **CODE_STRUCTURE.md** has line counts off by 3-5x and references files that don't exist
- **Many completed planning artifacts** still live alongside active docs
- **Implemented proposals** still read as if they're proposals, not completed work
- **CLI-specific docs** remain in this repo despite CLI moving to `livetemplate/lvt`

---

## Batch 1: Archive Completed Planning Artifacts

**Effort:** Small (file moves only)
**Status:** [x] Complete

Move completed plans, implemented proposals, and finished implementation tracking docs into `docs/archive/` so they don't clutter active documentation.

### Move to `docs/archive/plans/`:

| File | Reason |
|------|--------|
| `plans/2024-12-06-controller-state-pattern.md` | Implemented in v0.7.0 |
| `plans/upload-feature-implementation.md` | All 9 phases complete (2025-11-11) |
| `plans/2025-11-02-lvt-gen-stack-design.md` | Feature complete per IMPLEMENTATION_COMPLETE.md |
| `plans/2025-11-02-lvt-gen-stack-implementation.md` | Duplicate of above; feature complete |
| `plans/2025-11-01-lvt-gen-auth.md` | Implemented in livetemplate/lvt repo |
| `plans/2025-11-10-performance-benchmarks.md` | Fully implemented: benchmarks exist for all 5 phases, CI workflow, baseline tracking |

### Move to `docs/archive/proposals/`:

| File | Reason |
|------|--------|
| `proposals/002-controller-state-pattern.md` | Implemented in v0.7.0 |
| `proposals/async-websocket-sends.md` | Implemented (async WebSocket in session package) |
| `proposals/authentication-v0.5.md` | Implemented in v0.4.1 (marked "Implemented") |
| `proposals/value-deduplication-proposal.md` | Explicitly declined |
| `proposals/store-pattern-redesign.md` | Superseded by Controller+State (v0.7.0) |
| `proposals/bug-fix-persistence-and-design-improvements.md` | Partially implemented; design portion superseded by v0.7.0 |

### Move to `docs/archive/implementation-plans/`:

| File | Reason |
|------|--------|
| `implementation-plans/IMPLEMENTATION_STATUS.md` | Phases 1-7 complete (2025-10-20) |
| `implementation-plans/REFACTORING_PROGRESS.md` | All 5 phases complete, merged to main |
| `implementation-plans/REFACTOR_STRIP_STATICS.md` | Investigation complete, function renamed |
| `implementation-plans/BUG-VALIDATION-CONDITIONALS.md` | Both bugs fixed, all E2E tests passing |
| `implementation-plans/TESTING_FRAMEWORK_PROGRESS.md` | 15/15 sessions complete, production-ready |
| `implementation-plans/implementation-readiness.md` | Pre-implementation validation, feature underway |

### Move to `docs/archive/`:

| File | Reason |
|------|--------|
| `IMPLEMENTATION_COMPLETE.md` | Completion report for lvt gen stack (historical) |
| `lvt-gen-stack-implementation.md` | Duplicate plan for same feature, shows TODO markers for completed work |
| `SIMPLIFIED_DIFF_PROPOSAL.md` | Fully implemented in v0.8.0 |
| `ARCHITECTURAL_REVIEW.md` | Historical review, all recommendations implemented |

### Keep in place (still active):

| File | Reason |
|------|--------|
| `proposals/bindings-proposal.md` | Not implemented, 4-phase future feature |
| `proposals/lvt-bind-proposal.md` | Not implemented, future feature |
| `proposals/reactive-attributes.md` | Not implemented, future feature |
| `implementation-plans/MIGRATION.md` | Active reference for v1.0 migration |
| `implementation-plans/ARCHITECTURE_IMPROVEMENTS.md` | Phase 6 still deferred |
| `implementation-plans/html-fallback-coverage.md` | Ongoing safety net documentation |

### Post-archiving: Update docs/README.md index

After moving files, update docs/README.md to remove archived files from active sections and ensure the Archive section is accurate.

---

## Batch 2: Update Core Architecture Docs

**Effort:** Large
**Status:** [x] Complete

These are the highest-value docs and need line-by-line regeneration from the actual codebase.

### 2a. `docs/ARCHITECTURE.md`

**Known issues (needs thorough re-audit during this batch):**
- `internal/parse/` file listing incomplete
- `internal/build/` file listing incomplete
- `internal/send/` section references wrong filenames
- `internal/session/` missing `limits.go`
- Supporting packages section incomplete — missing `upload/`, `uploadtypes/`, `discovery/`, `compat/`, `fuzz/`, `testutil/`, `util/`

**Action:** Read every section. Run `find internal/ -name '*.go' ! -name '*_test.go'` and regenerate all file listings.

### 2b. `docs/CODE_STRUCTURE.md`

**Known issues (needs thorough re-audit during this batch):**
- Line counts severely outdated (off by 3-5x)
- References non-existent files
- Missing 7+ internal packages
- `mount.go` description outdated

**Action:** Regenerate file listing and line counts from `wc -l`. Remove references to non-existent files. Add all missing internal packages.

### 2c. `docs/ROADMAP.md`

**Known issues:**
- Shows 66% when actually ~80%+ complete
- M1 and M2 items marked in-progress that are done

**Action:** Update all M1 and M2 tasks to done. Update progress bars and percentages.

---

## Batch 3: Update Guides (thorough re-audit required)

**Effort:** Medium
**Status:** [ ] Not started

**IMPORTANT:** The initial audit rated these too generously. Each guide must be read line-by-line with every file reference and code example verified.

### 3a. `docs/guides/new-contributor-walkthrough.md`

**Confirmed issues:**
- 2 broken file references: `internal/session/connection.go`, `internal/observe/logger.go` don't exist
- Repository structure (lines 83-100): lists `client/`, `examples/` dirs that don't exist; missing 7 internal packages
- Go version (line 58): claims "Go 1.22+" but `go.mod` requires Go 1.26.0
- `render.Node()` signature (line 635): shown as `Node(node)` but actual is `Node(w *strings.Builder, n *html.Node)`
- Missing file references: `parse/flatten.go`, `parse/types.go`, `build/html_diff.go`, `build/html_segmentation.go`

**Likely additional issues not yet caught:** This is a 1300-line file. The above was from spot-checking. A full audit will likely find more stale references.

**Action:** Read every line. Verify every `[link](../../path)` resolves. Check every code example compiles conceptually. Fix all issues found.

### 3b. `docs/guides/auth-customization.md`

**Known issues:**
- Assumes `lvt gen auth` command exists (implemented in lvt repo, not this one)
- References generated files that don't exist in this codebase

**Action:** Reframe as manual auth customization guide. Remove references to `lvt gen auth` generated files.

### 3c. `docs/guides/lvt-cli-guide.md`

**Known issues:**
- Documents CLI features that live in separate repo
- Some commands may be aspirational

**Action:** Either move to lvt repo or add clear status indicators. Note that CLI source is in separate repo.

---

## Batch 4: Update Configuration, Operations, and Reference Docs

**Effort:** Medium
**Status:** [ ] Not started

### 4a. `docs/CONFIGURATION.md`

**Known issues:**
- Missing `LVT_WS_BUFFER_SIZE` env var
- Missing `LVT_PROGRESSIVE_ENHANCEMENT` env var
- Missing Redis configuration

**Action:** Cross-reference against actual config loading code. Add missing env vars.

### 4b. `docs/SCALING.md`

**Known issues:**
- Milestone completion status outdated
- Very large file (65.3KB) — review for content that can be trimmed

**Action:** Update milestone references. Review for unnecessary bulk.

### 4c. `docs/references/client-attributes.md`

**Known issues:**
- Some attributes may not be implemented in current client
- Needs audit against actual TypeScript client

**Action:** Mark unimplemented attributes as "Planned".

### 4d. `docs/references/api-reference.md`

**Known issues:**
- References `lvt auth` command (implemented in lvt repo)
- Some CLI examples assume features that may not exist here

**Action:** Remove or mark unimplemented CLI commands.

### 4e. `docs/references/authentication.md`

**Known issues:**
- `BasicAuthenticator` may not be fully implemented

**Action:** Verify against code. Minor updates if needed.

### Files expected to need no changes (verify during batch):
- `docs/OBSERVABILITY.md`
- `docs/references/controller-pattern.md`
- `docs/references/error-handling.md`
- `docs/references/server-actions.md`
- `docs/references/session.md`
- `docs/references/template-support-matrix.md`

---

## Batch 5: Specifications, Design, Performance, and CLAUDE.md

**Effort:** Medium
**Status:** [ ] Not started

### 5a. `docs/specifications/test-scenarios.md`

**Action:** Add header clarifying this is a test specification, not implementation status. Cross-reference with actual test files.

### 5b. `docs/fuzz-framework-design.md`

**Known issues:**
- Framework described is not implemented
- `internal/fuzz/` has only empty subdirectories
- Actual fuzz/property testing is in `internal/diff/diff_property_test.go`

**Action:** Archive as unimplemented design doc.

### 5c. `CLAUDE.md`

**Known issues:**
- CLI Tool section references `cmd/lvt/` directory structure that doesn't exist in this repo
- Internal package descriptions may be outdated

**Action:** Update internal package descriptions. Remove or relocate CLI section. Verify all referenced types and functions.

### Files expected to need no changes (verify during batch):
- `docs/specifications/tree-update-specification.md`
- `docs/specifications/tree-update-fallback-addendum.md`
- `docs/performance/performance-characteristics.md`
- `docs/performance/benchmarking-guide.md`
- `docs/performance/known-bottlenecks.md`
- `docs/performance/2025-11-10-benchmarking-system-design.md`
- `docs/design/FIRST_PRINCIPLES.md`
- `docs/design/multi-session-isolation.md`
- `docs/uploads.md`

---

## Summary

### Files to Archive (24 files)

| Category | Count |
|----------|-------|
| Completed plans | 6 |
| Implemented/declined proposals | 6 |
| Completed implementation tracking | 6 |
| Completed top-level docs | 4 |
| Fuzz framework design (Batch 5) | 1 |
| Already archived (no change) | 6 |
| **Total to move (Batch 1)** | **22** |

### Files to Update (13 files)

| File | Severity |
|------|----------|
| `docs/CODE_STRUCTURE.md` | High — line counts 3-5x off, missing packages |
| `docs/ROADMAP.md` | High — shows 66% when actually ~80% |
| `docs/ARCHITECTURE.md` | Medium — file listings incomplete |
| `docs/CONFIGURATION.md` | Medium — missing env vars |
| `docs/guides/new-contributor-walkthrough.md` | Medium — broken file refs, stale repo structure, wrong Go version |
| `docs/guides/lvt-cli-guide.md` | Medium — aspirational features, wrong repo |
| `docs/guides/auth-customization.md` | Medium — assumes unimplemented command |
| `docs/references/client-attributes.md` | Medium — unimplemented attributes |
| `docs/references/api-reference.md` | Medium — unimplemented CLI commands |
| `docs/SCALING.md` | Low — milestone status |
| `docs/specifications/test-scenarios.md` | Low — clarify as spec vs status |
| `CLAUDE.md` | Low — CLI section, internal package descriptions |
| `docs/README.md` | Low — update index after archiving |

### Files Expected to Need No Changes (25 files)

**Caveat:** These ratings are from initial audits with known token limitations. Each batch session should re-verify the "no changes needed" files in its scope.

### Batch Summary

| Batch | Scope | Effort | Key Work |
|-------|-------|--------|----------|
| **1** | Archiving | Small | `git mv` 22 files, update index |
| **2** | Core architecture docs | Large | Regenerate ARCHITECTURE.md, CODE_STRUCTURE.md, ROADMAP.md from codebase |
| **3** | Guides | Medium | Line-by-line audit of 3 guides (1300+ lines total) |
| **4** | Config + references | Medium | Verify 11 files against source code |
| **5** | Specs + design + CLAUDE.md | Medium | Verify 12 files, update CLAUDE.md |
