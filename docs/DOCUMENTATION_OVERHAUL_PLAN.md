# Documentation Overhaul Plan

**Date:** 2026-02-28
**Branch:** `claude/overhaul-documentation-1TcqZ`
**Scope:** All 65 documentation files across the LiveTemplate repository

---

## Motivation

The documentation has not been systematically updated since ~v0.7.0. The codebase is now at v0.8.3 with significant changes: the 5-phase refactoring is complete, all M1/M2 roadmap milestones are done, many proposals have been implemented, and the repository was split (CLI moved to `livetemplate/lvt`). As a result:

- **6 broken links** in docs/README.md (now fixed)
- **43+ files** were completely unindexed (now fixed)
- **ROADMAP.md** says 66% complete when M1/M2 are actually 100% done
- **CODE_STRUCTURE.md** has line counts off by 3-5x and references files that don't exist
- **9 completed planning artifacts** still live alongside active docs
- **3 implemented proposals** still read as if they're proposals, not completed work
- **CLI-specific docs** remain in this repo despite CLI moving to `livetemplate/lvt`

---

## Phase 1: Archive Completed Planning Artifacts

**Goal:** Move completed plans, implemented proposals, and finished implementation tracking docs into `docs/archive/` so they don't clutter active documentation.

### Move to `docs/archive/plans/`:

| File | Reason |
|------|--------|
| `plans/2024-12-06-controller-state-pattern.md` | Implemented in v0.7.0 |
| `plans/upload-feature-implementation.md` | All 9 phases complete (2025-11-11) |
| `plans/2025-11-02-lvt-gen-stack-design.md` | Feature complete per IMPLEMENTATION_COMPLETE.md |
| `plans/2025-11-02-lvt-gen-stack-implementation.md` | Duplicate of above; feature complete |

### Move to `docs/archive/proposals/`:

| File | Reason |
|------|--------|
| `proposals/002-controller-state-pattern.md` | Implemented in v0.7.0 |
| `proposals/async-websocket-sends.md` | Implemented (async WebSocket in session package) |
| `proposals/authentication-v0.5.md` | Implemented in v0.4.1 (marked "✅ Implemented") |
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
| `plans/2025-11-01-lvt-gen-auth.md` | Future feature, not yet implemented |
| `plans/2025-11-10-performance-benchmarks.md` | Infrastructure planning, still relevant |
| `proposals/bindings-proposal.md` | Not implemented, 4-phase future feature |
| `proposals/lvt-bind-proposal.md` | Not implemented, future feature |
| `proposals/reactive-attributes.md` | Not implemented, future feature |
| `implementation-plans/MIGRATION.md` | Active reference for v1.0 migration |
| `implementation-plans/ARCHITECTURE_IMPROVEMENTS.md` | Phase 6 still deferred |
| `implementation-plans/html-fallback-coverage.md` | Ongoing safety net documentation |

---

## Phase 2: Update Core Architecture Docs

### 2a. `docs/ARCHITECTURE.md` (Accuracy: ~90%)

**Issues:**
- `internal/parse/` file listing incomplete: docs list 5 files, actual has 14 (missing `flatten.go`, `with.go`, `var_context.go`, `types.go`, etc.)
- `internal/build/` file listing incomplete: missing `wrapper.go`, `html_diff.go`, `html_segmentation.go`
- `internal/send/` section outdated: docs mention `handler.go`, `websocket.go`, `broadcast.go` but actual files are `message.go`, `response.go`, `json.go`
- `internal/session/` missing `limits.go`
- Supporting packages section incomplete

**Action:** Update all file listings to match actual codebase. Regenerate from `find internal/ -name '*.go' ! -name '*_test.go'`.

### 2b. `docs/CODE_STRUCTURE.md` (Accuracy: ~70%)

**Issues:**
- Line counts severely outdated (off by 3-5x):
  - `internal/parse/`: claims ~1,320 lines, actual ~5,105
  - `internal/build/`: claims ~570 lines, actual ~2,770
  - `internal/diff/`: claims ~1,570 lines, actual ~8,774
- References `tree.go` as a standalone file, but tree operations are split across internal packages
- `mount.go` description outdated - current API is `Handle(controller, stateWrapper, ...HandleOption)`
- Mentions non-existent global `Config` type
- `internal/observe/` missing `prometheus.go`
- Missing packages: `internal/upload/`, `internal/uploadtypes/`, `internal/discovery/`, `internal/compat/`, `internal/fuzz/`, `internal/util/`

**Action:** Regenerate file listing and line counts from actual codebase. Remove references to non-existent files. Add missing internal packages.

### 2c. `docs/ROADMAP.md` (Accuracy: ~60%)

**Issues:**
- Shows overall progress as "66% (46/70 tasks)" but code review shows:
  - M1 (Health & Resilience): All tasks complete (health.go, limits.go, shutdown exists)
  - M2 (Observability & Operations): All tasks complete (slog, prometheus, RedisSessionStore)
  - M3 (Advanced Features): Not started
- Actual progress: ~80% (56/70), not 66%
- Multiple items marked "🟡 IN PROGRESS" that are actually done

**Action:** Update all M1 and M2 tasks to ✅ DONE. Update progress bars and percentages. Keep M3 as pending.

---

## Phase 3: Update Configuration & Operations Docs

### 3a. `docs/CONFIGURATION.md` (Accuracy: ~95%)

**Issues:**
- Missing `LVT_WS_BUFFER_SIZE` environment variable (WebSocket buffer, default 50)
- Missing `LVT_PROGRESSIVE_ENHANCEMENT` environment variable
- Missing Redis configuration documentation

**Action:** Add missing environment variables with descriptions and defaults.

### 3b. `docs/OBSERVABILITY.md` (Accuracy: ~100%)

**No changes needed.** Metrics list matches `internal/observe/prometheus.go`.

### 3c. `docs/SCALING.md` (Accuracy: ~85%)

**Issues:**
- Milestone completion status outdated (same issue as ROADMAP.md)
- File is very large (65.3KB) - consider whether all content is still needed

**Action:** Update milestone references to reflect actual completion. Review for content that can be trimmed.

---

## Phase 4: Update Guides

### 4a. `docs/guides/new-contributor-walkthrough.md` (Accuracy: ~100%)

**No changes needed.** Excellent, current documentation with correct Controller+State pattern.

### 4b. `docs/guides/auth-customization.md` (Accuracy: ~70%)

**Issues:**
- Assumes `lvt gen auth` command exists (not yet implemented)
- References generated files that don't exist in current codebase
- Kit customization workflows may not be fully integrated

**Action:** Add note that `lvt gen auth` is a planned feature. Reframe as manual auth customization guide until the command exists.

### 4c. `docs/guides/lvt-cli-guide.md` (Accuracy: ~60%)

**Issues:**
- Documents aspirational CLI features not fully implemented
- Commands like `lvt gen auth`, `lvt gen view`, `lvt migration` may not exist
- Quick start assumes more complete tooling than currently available
- CLI is in separate repository (`livetemplate/lvt`)

**Action:** Either (a) move to lvt repository, or (b) add clear status indicators for which commands are implemented vs planned. Add note that CLI source lives in separate repo.

---

## Phase 5: Update Reference Docs

### 5a. `docs/references/controller-pattern.md` (Accuracy: ~100%)

**No changes needed.**

### 5b. `docs/references/client-attributes.md` (Accuracy: ~85%)

**Issues:**
- Some attributes (`lvt-scroll`, `lvt-highlight`, `lvt-animate`, `lvt-modal-*`) may not be implemented in current client
- Multi-store pattern (`lvt-click="counter.increment"`) not clearly tied to server implementation

**Action:** Audit against actual TypeScript client. Mark unimplemented attributes as "Planned".

### 5c. `docs/references/error-handling.md` (Accuracy: ~100%)

**No changes needed.**

### 5d. `docs/references/authentication.md` (Accuracy: ~95%)

**Issues:**
- `BasicAuthenticator` may not be fully implemented
- Custom authenticator examples are idealized

**Action:** Verify BasicAuthenticator against code. Minor updates if needed.

### 5e. `docs/references/server-actions.md` (Accuracy: ~100%)

**No changes needed.**

### 5f. `docs/references/session.md` (Accuracy: ~100%)

**No changes needed.**

### 5g. `docs/references/template-support-matrix.md` (Accuracy: ~100%)

**No changes needed.**

### 5h. `docs/references/api-reference.md` (Accuracy: ~85%)

**Issues:**
- References `lvt auth` command (not yet implemented)
- Some CLI command examples assume features that may not exist

**Action:** Remove or mark unimplemented CLI commands as planned.

---

## Phase 6: Update Specifications & Performance Docs

### 6a. `docs/specifications/tree-update-specification.md` (Accuracy: ~100%)

**No changes needed.** Formal spec is correct.

### 6b. `docs/specifications/tree-update-fallback-addendum.md` (Accuracy: ~100%)

**No changes needed.**

### 6c. `docs/specifications/test-scenarios.md` (Accuracy: ~80%)

**Issues:**
- Reads as specification for how tests SHOULD be written, not what currently exists
- References `testdata/scenarios/` which may not have all golden files

**Action:** Add header clarifying this is a test specification, not implementation status. Cross-reference with actual test files.

### 6d. `docs/performance/` (all 4 files) (Accuracy: ~100%)

**No changes needed.** All performance docs are data-driven and current.

---

## Phase 7: Update Design Docs

### 7a. `docs/design/FIRST_PRINCIPLES.md` (Accuracy: ~100%)

**No changes needed.** Foundational principles remain valid.

### 7b. `docs/design/multi-session-isolation.md` (Accuracy: ~100%)

**No changes needed.** All 6 phases implemented and correctly documented.

---

## Phase 8: Clean Up Fuzz Framework

### 8a. `docs/fuzz-framework-design.md` (Accuracy: ~50%)

**Issues:**
- Describes a framework with `UpdateValidator`, `StateSimulator`, etc.
- `internal/fuzz/` directory exists but contains only empty subdirectories (`app/`, `generators/`, `invariants/`, `mutations/`) - no `.go` files
- Actual fuzz/property testing lives elsewhere: `internal/diff/diff_property_test.go`

**Action:** Either (a) archive as unimplemented design, or (b) update to reflect actual property-based testing approach in `diff_property_test.go`.

---

## Phase 9: Update docs/README.md Index

**Already completed** in prior commit (3aee0f9). After Phase 1 archiving, update the index to:
- Remove links to archived files from active sections
- Add an "Archive" section referencing newly archived docs
- Ensure all remaining active docs are properly listed

---

## Phase 10: Update CLAUDE.md

### Issues:
- CLI Tool section references `cmd/lvt/` directory structure that doesn't exist in this repo
- Some internal package descriptions may be outdated (same issues as ARCHITECTURE.md)

**Action:** Update internal package descriptions to match reality. Add note that CLI is in separate repository. Verify all referenced types and functions still exist.

---

## Summary

### Files to Archive (22 files)

| Category | Count |
|----------|-------|
| Completed plans | 4 |
| Implemented/declined proposals | 6 |
| Completed implementation tracking | 6 |
| Completed top-level docs | 4 |
| Already archived (no change) | 6 |
| **Total** | **22** (+4 new archive subdirectories to create) |

### Files to Update (12 files)

| File | Severity |
|------|----------|
| `docs/CODE_STRUCTURE.md` | High - line counts 3-5x off, missing packages |
| `docs/ROADMAP.md` | High - shows 66% when actually ~80% |
| `docs/ARCHITECTURE.md` | Medium - file listings incomplete |
| `docs/CONFIGURATION.md` | Medium - missing env vars |
| `docs/guides/lvt-cli-guide.md` | Medium - aspirational features |
| `docs/guides/auth-customization.md` | Medium - assumes unimplemented command |
| `docs/references/client-attributes.md` | Medium - unimplemented attributes |
| `docs/references/api-reference.md` | Medium - unimplemented CLI commands |
| `docs/SCALING.md` | Low - milestone status |
| `docs/fuzz-framework-design.md` | Low - framework not implemented |
| `docs/specifications/test-scenarios.md` | Low - clarify as spec vs status |
| `CLAUDE.md` | Low - CLI section references |

### Files Needing No Changes (27 files)

All reference docs, specifications, performance docs, design docs, and contributor guides that are already accurate and current.

### Estimated Effort

| Phase | Effort | Description |
|-------|--------|-------------|
| Phase 1 | Small | File moves only (git mv) |
| Phase 2 | Large | Regenerate architecture docs from codebase |
| Phase 3 | Small | Add missing config entries |
| Phase 4 | Medium | Reframe CLI/auth guides |
| Phase 5 | Small | Mark unimplemented attributes |
| Phase 6 | Small | Add clarifying headers |
| Phase 7 | None | No changes needed |
| Phase 8 | Small | Archive or update fuzz doc |
| Phase 9 | Small | Update index after archiving |
| Phase 10 | Medium | Update CLAUDE.md internals |
