# 5-Phase Architecture Refactoring - v0.3.0

**Branch:** `refactor-5-phase`
**Goal:** Refactor into proper 5-phase architecture (Parse → Build → Diff → Render → Send)
**Previous:** v0.2.0 API Reduction (18 → 12 files)
**Current:** v0.3.0 5-Phase Separation

**Status:** ✅ IMPLEMENTATION COMPLETE - All Tests Passing

---

## v0.3.0 Completion Summary

### ✅ Completed Work

**Phase 1: internal/keys/ - Key Generation**
- ✅ Created internal/keys/generator.go (201 lines)
- ✅ Created internal/keys/generator_test.go (17 test functions)
- ✅ Renamed KeyGenerator → Generator for cleaner API
- ✅ Added DynamicsGetter interface to avoid circular dependencies
- ✅ Updated imports in tree.go, internal/parse/

**Phase 2: internal/render/ - HTML Rendering**
- ✅ Created internal/render/html.go (148 lines)
- ✅ Created internal/render/html_test.go (21 test functions)
- ✅ Renamed functions: RenderNode → Node, RenderTreeToHTML → TreeToHTML, IsVoidHTMLElement → IsVoidElement
- ✅ Updated imports in tree.go, internal/build/wrapper.go

**Phase 3: internal/send/ - Message Formatting & Serialization**
- ✅ Created internal/send/message.go (50 lines)
- ✅ Created internal/send/response.go (55 lines)
- ✅ Moved ActionMessage, UpdateResponse, ResponseMetadata types
- ✅ Functions: ParseActionFromHTTP(), ParseActionFromWebSocket(), PrepareUpdate(), SerializeUpdate()
- ✅ Updated imports in action.go, template.go
- ✅ Maintained backward compatibility with type aliases

**Previous Work (v0.2.0):**
- ✅ internal/context/ - Template execution context
- ✅ internal/signature/ - Structure tracking and registry
- ✅ internal/session/ - Connection registry and limits

**Phase 2: File Consolidation**
- ✅ Merged `session.go` + `session_redis.go` → `session_stores.go` (533 lines)
- ✅ Merged `health_redis.go` content into `health.go`
- ✅ Created `types.go` from `build_types.go` (clean re-exports)

**Phase 3: Deleted Obsolete Files**
- ✅ Deleted: errors.go, structure_signature.go, client_structure_registry.go
- ✅ Deleted: session.go, session_redis.go, health_redis.go
- ✅ Deleted: build_types.go, registry.go, limits.go

**Phase 4: Updated Imports & Fixed Tests**
- ✅ Updated mount.go to use `internal/session` types
- ✅ Fixed Connection.Mu export for shutdown handling
- ✅ Moved test files to internal packages (signature, session)
- ✅ Fixed import cycles by using `package signature_test`
- ✅ Exported HashStatics for testing
- ✅ All tests passing (main + internal packages)

**Phase 5: Results**
- Files reduced: 18 → 12 (33% reduction)
- Key types moved to internal: ✅ Connection, ConnectionRegistry, ConnectionLimits, StructureSignature, ClientStructureRegistry
- API surface reduced: Implementation details now internal
- Test coverage: 100% passing ✅

---

## Phase 1: Setup ✅

- [x] Check .gitignore for .worktrees/
- [x] Create git worktree at .worktrees/api-reduction-v0.2.0
- [x] Create PROGRESS.md tracker file
- [ ] Run baseline tests to ensure clean start

---

## Phase 2: Move Implementation to Internal Packages

### 2.1 internal/context/ (Template Execution Context)
- [ ] Create internal/context/context.go
- [ ] Move TemplateContext type from errors.go
- [ ] Move executeTemplateWithContext function from errors.go
- [ ] Add package-level documentation
- [ ] Update template.go imports

### 2.2 internal/keys/ (Key Generation)
- [ ] Create internal/keys/generator.go
- [ ] Move keyGenerator type from tree.go
- [ ] Move newKeyGenerator function from tree.go
- [ ] Move generateWrapperKey function from tree.go
- [ ] Move detectIDKey function from tree.go
- [ ] Add package-level documentation
- [ ] Update template.go imports

### 2.3 internal/session/ (Connection & Session Management)
- [ ] Create internal/session/registry.go
- [ ] Move Connection type from registry.go
- [ ] Move ConnectionRegistry type from registry.go
- [ ] Move all ConnectionRegistry methods
- [ ] Create internal/session/limits.go
- [ ] Move ConnectionLimits type from limits.go
- [ ] Move ConnectionLimitStats type from limits.go
- [ ] Move all ConnectionLimits methods
- [ ] Add package-level documentation
- [ ] Update mount.go imports

### 2.4 internal/send/ (Broadcasting & Messaging)
- [ ] Create internal/send/broadcaster.go
- [ ] Move broadcaster type from mount.go
- [ ] Move Broadcaster interface coordination logic
- [ ] Move BroadcastAware coordination logic
- [ ] Move message sending utilities
- [ ] Add package-level documentation
- [ ] Update mount.go imports

### 2.5 internal/render/ (HTML Rendering Utilities)
- [ ] Create internal/render/html.go
- [ ] Move renderTreeToHTML from tree.go (if still used)
- [ ] Move any HTML rendering helper functions
- [ ] Add package-level documentation
- [ ] Update relevant imports

### 2.6 internal/mount/ (Handler Implementation)
- [ ] Create internal/mount/handler.go
- [ ] Move liveHandler type from mount.go
- [ ] Move all liveHandler methods
- [ ] Move MountConfig type from mount.go
- [ ] Move MountOption type from mount.go
- [ ] Move connState type from mount.go
- [ ] Move WebSocket handling logic
- [ ] Add package-level documentation
- [ ] Update template.go imports

### 2.7 internal/signature/ (Structure Tracking)
- [ ] Create internal/signature/signature.go
- [ ] Move entire structure_signature.go content
- [ ] Move StructureSignature type
- [ ] Move CalculateSignature function
- [ ] Create internal/signature/registry.go
- [ ] Move entire client_structure_registry.go content
- [ ] Move ClientStructureRegistry type
- [ ] Move all ClientStructureRegistry methods
- [ ] Add package-level documentation
- [ ] Update template.go imports

### 2.8 internal/build/ (Tree Building Utilities)
- [ ] Create internal/build/wrappers.go
- [ ] Move calculateFingerprint from tree.go
- [ ] Move addFingerprintToTree from tree.go
- [ ] Move generateRandomID from tree.go
- [ ] Move injectWrapperDiv from tree.go
- [ ] Move extractTemplateBodyContent from tree.go
- [ ] Move extractTemplateContent from tree.go
- [ ] Move normalizeTemplateSpacing from tree.go
- [ ] Move parseTemplateToTree from tree.go
- [ ] Add package-level documentation
- [ ] Update template.go imports

---

## Phase 3: Consolidate Main Package Files

### 3.1 Create session_stores.go
- [ ] Create session_stores.go
- [ ] Copy SessionStore interface from session.go
- [ ] Copy MemorySessionStore from session.go
- [ ] Copy SessionStoreOption from session.go
- [ ] Copy all session.go functions
- [ ] Copy RedisSessionStore from session_redis.go
- [ ] Copy RedisSessionStoreOption from session_redis.go
- [ ] Copy all session_redis.go functions
- [ ] Verify all exports preserved
- [ ] Delete session.go (after verification)
- [ ] Delete session_redis.go (after verification)

### 3.2 Merge health.go
- [ ] Copy all content from health_redis.go to health.go
- [ ] Verify RedisHealthChecker moved
- [ ] Verify NewRedisHealthChecker function moved
- [ ] Verify no duplicate code
- [ ] Delete health_redis.go

### 3.3 Create types.go
- [ ] Create types.go with package documentation
- [ ] Copy TreeNode type from build_types.go
- [ ] Copy RangeData type from build_types.go
- [ ] Copy TreeMetadata type from build_types.go
- [ ] Copy all constructor functions (NewTreeNode, etc.)
- [ ] Add clear documentation for each type
- [ ] Verify backward compatibility
- [ ] Delete build_types.go (after verification)

### 3.4 Update mount.go (Interface Only)
- [ ] Keep only LiveHandler interface definition
- [ ] Keep only Broadcaster interface definition
- [ ] Keep only BroadcastAware interface definition
- [ ] Add factory function: func (t *Template) Handle(stores ...Store) LiveHandler
- [ ] Factory should delegate to internal/mount
- [ ] Remove all implementation details
- [ ] Verify interface contracts unchanged

### 3.5 Delete Obsolete Files
- [ ] Verify tree.go content fully migrated
- [ ] Delete tree.go
- [ ] Verify registry.go content fully migrated
- [ ] Delete registry.go
- [ ] Verify limits.go content fully migrated
- [ ] Delete limits.go
- [ ] Verify errors.go content fully migrated
- [ ] Delete errors.go
- [ ] Verify structure_signature.go content fully migrated
- [ ] Delete structure_signature.go
- [ ] Verify client_structure_registry.go content fully migrated
- [ ] Delete client_structure_registry.go

---

## Phase 4: Update Code & Tests

### 4.1 Update Main Package Imports
- [ ] Update template.go to import internal/context
- [ ] Update template.go to import internal/signature
- [ ] Update template.go to import internal/build
- [ ] Update template.go to import internal/keys
- [ ] Update template.go to import internal/mount
- [ ] Update mount.go (new interface-only) to import internal/mount
- [ ] Update mount.go to import internal/session
- [ ] Update mount.go to import internal/send
- [ ] Update action.go if needed
- [ ] Update config.go if needed

### 4.2 Update Test Files
- [ ] Find all test files: `find . -name "*_test.go"`
- [ ] Update template_test.go imports
- [ ] Update mount_test.go imports
- [ ] Update session_test.go imports (if exists)
- [ ] Update registry_test.go imports (if exists)
- [ ] Update all other *_test.go files
- [ ] Verify no broken test imports

### 4.3 Run Tests & Fix Issues
- [ ] Run: go test -v ./...
- [ ] Document any test failures
- [ ] Fix compilation errors
- [ ] Fix broken references
- [ ] Fix import cycles if any
- [ ] Re-run tests until 100% pass
- [ ] Verify no regression in functionality

---

## Phase 5: Update Core Documentation

### 5.1 CLAUDE.md
- [ ] Update "Key Components" section with new structure
- [ ] Document 8 main package files
- [ ] Document internal package structure
- [ ] Document empty packages now in use
- [ ] Update "Common Tasks" section
- [ ] Update file paths in examples

### 5.2 README.md
- [ ] Add v0.2.0 breaking changes section
- [ ] Document reduced API surface (18→8 files, ~80→~40-50 exports)
- [ ] Create migration guide from v0.1.x
- [ ] List new internal packages
- [ ] Update any outdated code examples
- [ ] Update quickstart if needed

### 5.3 Package Documentation
- [ ] Add package doc for internal/context
- [ ] Add package doc for internal/keys
- [ ] Add package doc for internal/session
- [ ] Add package doc for internal/send
- [ ] Add package doc for internal/render
- [ ] Add package doc for internal/mount
- [ ] Add package doc for internal/signature
- [ ] Update package doc for internal/build

---

## Phase 6: Update docs/ Folder

### 6.1 Update Current Documentation
- [ ] docs/ARCHITECTURE.md - reflect new internal packages
- [ ] docs/ARCHITECTURE.md - document 18→8 consolidation
- [ ] docs/CODE_STRUCTURE.md - update file listing
- [ ] docs/CODE_STRUCTURE.md - document package organization
- [ ] docs/references/api-reference.md - document minimal public API
- [ ] docs/references/api-reference.md - list ~40-50 essential exports
- [ ] docs/ROADMAP.md - add v0.2.0 milestone
- [ ] docs/ROADMAP.md - mark API reduction as complete

### 6.2 Verify Accuracy
- [ ] docs/specifications/ - verify tree specs unchanged
- [ ] docs/BROADCASTING.md - verify API still accurate
- [ ] docs/CONFIGURATION.md - verify still accurate
- [ ] docs/OBSERVABILITY.md - verify still accurate
- [ ] docs/SCALING.md - verify still accurate
- [ ] docs/guides/auth-customization.md - update if needed
- [ ] docs/guides/lvt-cli-guide.md - verify still accurate

### 6.3 Archive Completed Work
- [ ] Create docs/archive/implementation-plans/ if not exists
- [ ] Move docs/implementation-plans/* to archive
- [ ] Create docs/archive/plans/ if not exists
- [ ] Move docs/plans/* to archive
- [ ] Review docs/proposals/ - identify implemented proposals
- [ ] Archive implemented proposals
- [ ] Keep active proposals in docs/proposals/

### 6.4 Update Index
- [ ] Update docs/README.md with current structure
- [ ] Remove references to archived docs
- [ ] Add clear navigation to active docs
- [ ] Organize by category (Architecture, Guides, References, Specifications)
- [ ] Add quick links to essential docs

---

## Phase 7: Version & Release

### 7.1 Version Updates
- [ ] Update go.mod module version comment
- [ ] Check if any internal version constants exist
- [ ] Update version references in code if any

### 7.2 Changelog
- [ ] Create CHANGELOG.md if doesn't exist
- [ ] Add v0.2.0 section
- [ ] Document breaking changes (file consolidation)
- [ ] Document breaking changes (API reduction)
- [ ] Document breaking changes (internal package moves)
- [ ] Add migration guide reference
- [ ] Document benefits (cleaner API, better organization)

### 7.3 Final Validation
- [ ] Run full test suite: go test -v ./...
- [ ] Verify 100% test pass rate
- [ ] Check for any TODO or FIXME comments added
- [ ] Verify no debug/temporary code left
- [ ] Run go fmt ./...
- [ ] Run go vet ./...
- [ ] Check examples/ directory if exists

### 7.4 Commit
- [ ] Stage all changes: git add .
- [ ] Review staged changes: git status
- [ ] Commit: "refactor: reduce API surface area for v0.2.0"
- [ ] Add detailed commit message body explaining changes
- [ ] Verify commit created successfully

---

## Summary Stats

### Before (v0.1.x)
- Main package files: **18**
- Public exports: **~80+**
- Empty internal packages: **5** (context, keys, render, send, session)
- Implementation exposure: **High** (many internals public)

### After (v0.2.0)
- Main package files: **8**
- Public exports: **~40-50** (essential only)
- Empty internal packages: **0** (all utilized)
- New internal packages: **2** (mount, signature)
- Implementation exposure: **Low** (clean public API)

### File Mapping
| Old Files | New File | Status |
|-----------|----------|--------|
| session.go + session_redis.go | session_stores.go | Merged |
| health.go + health_redis.go | health.go | Merged |
| tree.go + build_types.go | types.go | Merged/Moved |
| registry.go | (deleted) | → internal/session |
| limits.go | (deleted) | → internal/session |
| errors.go | (deleted) | → internal/context |
| structure_signature.go | (deleted) | → internal/signature |
| client_structure_registry.go | (deleted) | → internal/signature |
| tree.go (impl) | (deleted) | → internal/build |

### Retained Files
- template.go (updated imports)
- action.go (unchanged)
- auth.go (unchanged)
- config.go (unchanged)
- mount.go (interface only)
- session_stores.go (new, merged)
- health.go (updated, merged)
- types.go (new, clean re-exports)

---

## Notes
- All changes maintain backward compatibility at the API level
- Internal package imports are breaking changes (expected for v0.2.0)
- Test coverage must remain at 100% passing
- Documentation must be updated to reflect new structure
- Migration guide is essential for users upgrading from v0.1.x
