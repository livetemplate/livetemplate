# Client Structure Registry - Implementation Plan

**Branch**: `feat/client-structure-registry`  
**Start Date**: 2025-10-26  
**Estimated Duration**: 4 days  
**Status**: NOT STARTED

---

## Executive Summary

Replace ad-hoc "has client seen this structure" logic with a principled **Structure Signature System** that definitively tracks what structures the client has received. This eliminates the entire class of "missing statics" bugs.

### Problem
Current implementation uses multiple fragile approaches:
- `seenStructures map[string]bool` - boolean only, no signature
- `hasRangeItems()` - band-aid for empty ranges
- Path-based field lookups - band-aid for nested conditionals
- Range signature matching - fragile fallback

**Result**: Each edge case requires another band-aid fix.

### Solution
Single source of truth: `ClientStructureRegistry` that tracks structure signatures at each path.

---

## Progress Tracker

### Overall Status
- [ ] Phase 1: Core Infrastructure (Day 1)
- [ ] Phase 2: Integration (Day 2)
- [ ] Phase 3: Cleanup (Day 3)
- [ ] Phase 4: Validation (Day 4)

**Current Phase**: Not Started  
**Last Updated**: 2025-10-26  
**Blockers**: None

---

## Phase 1: Core Infrastructure ⏸️ NOT STARTED

**Goal**: Build the foundation - signature system and registry  
**Duration**: 1 day  
**Status**: ⏸️ NOT STARTED

### Tasks

#### 1.1 Create structure_signature.go ⏸️
- [ ] Define `StructureSignature` type
  ```go
  type StructureSignature string
  ```
- [ ] Define signature constants
  - [ ] `SigEmpty` - nil or empty string
  - [ ] `SigScalar` - simple value
  - [ ] `SigConditional` - TreeNode with statics, no range
  - [ ] `SigRangeEmpty` - Range with zero items
  - [ ] `SigRangeItems` - Range with items + statics hash
- [ ] Implement `CalculateSignature(value interface{}) StructureSignature`
  - [ ] Handle nil/empty → `SigEmpty`
  - [ ] Handle non-TreeNode → `SigScalar`
  - [ ] Handle TreeNode with range
    - [ ] Empty range → `SigRangeEmpty`
    - [ ] Range with items → `SigRangeItems:hash(statics)`
  - [ ] Handle TreeNode with statics → `SigConditional`
  - [ ] Fallback → `SigScalar`
- [ ] Add helper methods on StructureSignature
  - [ ] `IsRange() bool` - checks if signature is range type
  - [ ] `HasItems() bool` - checks if range has items
  - [ ] `IsEmpty() bool` - checks if empty signature
- [ ] Write unit tests
  - [ ] Test nil values
  - [ ] Test scalar values
  - [ ] Test conditional nodes
  - [ ] Test empty ranges
  - [ ] Test ranges with items
  - [ ] Test signature uniqueness for different statics

**Files Created**:
- `structure_signature.go` (new file)
- `structure_signature_test.go` (new file)

**Success Criteria**:
- All unit tests pass
- Signature calculation is deterministic
- Same structure → same signature
- Different structure → different signature

---

#### 1.2 Create client_structure_registry.go ⏸️
- [ ] Define `ClientStructureRegistry` struct
  ```go
  type ClientStructureRegistry struct {
      structures map[string]StructureSignature
      mu         sync.RWMutex
  }
  ```
- [ ] Implement `NewClientStructureRegistry() *ClientStructureRegistry`
- [ ] Implement `HasSeen(fieldPath string, value interface{}) bool`
  - [ ] Calculate signature of value
  - [ ] Look up stored signature at fieldPath
  - [ ] Return true only if signatures match
- [ ] Implement `MarkSeen(fieldPath string, value interface{})`
  - [ ] Calculate signature of value
  - [ ] Store signature at fieldPath
- [ ] Implement `Clear()`
  - [ ] Reset structures map
- [ ] Implement `GetSignature(fieldPath string) (StructureSignature, bool)`
  - [ ] Return stored signature and existence flag
- [ ] Write unit tests
  - [ ] Test marking and checking single path
  - [ ] Test nested paths
  - [ ] Test structure changes at same path
  - [ ] Test thread safety (concurrent access)
  - [ ] Test Clear() functionality

**Files Created**:
- `client_structure_registry.go` (new file)
- `client_structure_registry_test.go` (new file)

**Success Criteria**:
- All unit tests pass
- Thread-safe operations verified
- Registry correctly tracks structure evolution

---

### Phase 1 Checkpoint

**Before Proceeding to Phase 2**:
- [ ] All Phase 1 unit tests pass
- [ ] Code review completed
- [ ] Documentation complete
- [ ] Git commit: "feat: Add structure signature system and client registry"

**Estimated Time**: 6-8 hours

---

## Phase 2: Integration 🔄 NOT STARTED

**Goal**: Integrate registry into Template and tree comparison  
**Duration**: 1 day  
**Status**: 🔄 NOT STARTED

### Tasks

#### 2.1 Update Template struct ⏸️
- [ ] Modify `template.go` Template struct
  - [ ] Replace `seenStructures map[string]bool` 
  - [ ] Add `registry *ClientStructureRegistry`
- [ ] Update `New()` function
  - [ ] Initialize `registry: NewClientStructureRegistry()`
- [ ] Update `Clone()` if it exists
  - [ ] Clone registry state
- [ ] Update any Template serialization
  - [ ] Ensure registry is not serialized (transient state)

**Files Modified**:
- `template.go`

**Tests**:
- [ ] Verify Template initialization includes registry
- [ ] Verify existing Template tests still pass

---

#### 2.2 Mark structures on first render ⏸️
- [ ] Implement `markAllStructuresAsSeen(node *TreeNode, basePath string)`
  - [ ] Recursive traversal of tree
  - [ ] Mark current node at basePath
  - [ ] Recursively mark all children
  - [ ] Handle nested TreeNodes
  - [ ] Handle range items
- [ ] Update `Execute()` method
  - [ ] After generating initialTree
  - [ ] Call `markAllStructuresAsSeen(t.initialTree, "")`
- [ ] Write tests
  - [ ] Test simple tree marking
  - [ ] Test nested structure marking
  - [ ] Test range structure marking
  - [ ] Verify all paths are marked correctly

**Files Modified**:
- `template.go`

**Tests**:
- [ ] Test that all structures from first render are marked
- [ ] Test nested paths are marked correctly
- [ ] Test range structures are marked

---

#### 2.3 Refactor compareTreesAndGetChangesWithPath ⏸️
- [ ] Identify all "has client seen" decision points
  - [ ] New field appears
  - [ ] Field value changes
  - [ ] Structure type changes
  - [ ] Range operations
- [ ] Replace ad-hoc checks with registry
  - [ ] `if !existsInOld` → use `registry.HasSeen()`
  - [ ] `if structureChanged` → compare signatures
  - [ ] `if isRangeConstruct(old)` → compare signatures
- [ ] Add `MarkSeen()` calls for new structures
  - [ ] When sending complete structure
  - [ ] After structure is included in changes
- [ ] Handle range operations specially
  - [ ] Check if range signature changed
  - [ ] Empty → Items: send complete
  - [ ] Items → Items (same sig): send diff
  - [ ] Items → Empty: send complete

**Files Modified**:
- `template.go` (compareTreesAndGetChangesWithPath function)

**Tests**:
- [ ] Test new field with known structure
- [ ] Test new field with unknown structure
- [ ] Test structure change at same path
- [ ] Test range empty → items transition
- [ ] Test range items → more items transition
- [ ] Test conditional branch switch

---

### Phase 2 Checkpoint

**Before Proceeding to Phase 3**:
- [ ] All tree comparison tests pass
- [ ] Integration tests pass
- [ ] No regressions in existing tests
- [ ] Git commit: "feat: Integrate structure registry into tree comparison"

**Estimated Time**: 6-8 hours

---

## Phase 3: Cleanup 🧹 NOT STARTED

**Goal**: Remove obsolete code and simplify  
**Duration**: 1 day  
**Status**: 🧹 NOT STARTED

### Tasks

#### 3.1 Remove obsolete helpers ⏸️
- [ ] Delete `hasRangeItems()` function
  - [ ] Search for all usages
  - [ ] Verify replaced by signature comparison
  - [ ] Remove function
- [ ] Delete `fieldExistsInTree()` function
  - [ ] Search for all usages
  - [ ] Verify replaced by registry lookups
  - [ ] Remove function
- [ ] Delete path traversal code for structure checks
  - [ ] Identify path-based lookup code
  - [ ] Verify replaced by registry
  - [ ] Remove code
- [ ] Update tests that used these helpers
  - [ ] Rewrite tests to use registry
  - [ ] Ensure test coverage maintained

**Files Modified**:
- `template.go` (removing functions)
- Various test files

**Tests**:
- [ ] Verify no references to deleted functions
- [ ] All tests still pass

---

#### 3.2 Simplify range matching logic ⏸️
- [ ] Review `findRangeConstructMatches()` function
  - [ ] Determine if still needed with signatures
  - [ ] If needed, simplify using signatures
  - [ ] If not needed, remove entirely
- [ ] Review `generateRangeDifferentialOperations()`
  - [ ] Remove `stripStatics` parameter if obsolete
  - [ ] Simplify logic using registry
- [ ] Remove range signature fallback matching
  - [ ] Identify fallback code
  - [ ] Replace with signature comparison
  - [ ] Test edge cases

**Files Modified**:
- `template.go`

**Tests**:
- [ ] Test range matching still works
- [ ] Test differential operations correct
- [ ] Test all range transition scenarios

---

#### 3.3 Simplify seenStructures usage ⏸️
- [ ] Find all references to old `seenStructures` map
- [ ] Verify all replaced by registry
- [ ] Remove any remaining references
- [ ] Update documentation

**Files Modified**:
- `template.go`
- Documentation files

**Tests**:
- [ ] Grep for "seenStructures" returns zero results (except in this plan)

---

### Phase 3 Checkpoint

**Before Proceeding to Phase 4**:
- [ ] All obsolete code removed
- [ ] No references to old helpers
- [ ] All tests pass
- [ ] Code review completed
- [ ] Git commit: "refactor: Remove obsolete structure tracking code"

**Estimated Time**: 4-6 hours

---

## Phase 4: Validation 🎯 NOT STARTED

**Goal**: Comprehensive testing and verification  
**Duration**: 1 day  
**Status**: 🎯 NOT STARTED

### Tasks

#### 4.1 Core library tests ⏸️
- [ ] Run all existing tests
  - [ ] `TestTemplate_E2E_*` - all pass
  - [ ] `TestTreeInvariant*` - all pass
  - [ ] `TestKeyInjection*` - all pass
  - [ ] All other core tests - all pass
- [ ] Add new structure transition tests
  - [ ] Test empty → first item (current bug)
  - [ ] Test first item → more items
  - [ ] Test items → empty
  - [ ] Test conditional switch (else → if with range)
  - [ ] Test nested conditional + range
  - [ ] Test search filtering (items → filtered → unfiltered)
- [ ] Add edge case tests
  - [ ] Rapid structure changes
  - [ ] Multiple nested levels
  - [ ] Large ranges (100+ items)

**Files Created**:
- `structure_transition_test.go` (new comprehensive test file)

**Success Criteria**:
- [ ] All existing tests pass (no regressions)
- [ ] All new structure transition tests pass
- [ ] Edge cases handled correctly

---

#### 4.2 E2E tests ⏸️
- [ ] Update E2E tests to reuse template instances
  - [ ] Modify `e2e_test.go` to not create fresh templates
  - [ ] Ensure tests simulate real app behavior
- [ ] Run todos example test
  - [ ] `TestTodosE2E` - all subtests pass
  - [ ] No `[object Object]` in DOM
  - [ ] First todo appears correctly
  - [ ] Search functionality works
  - [ ] Pagination works
- [ ] Run tutorial test
  - [ ] `TestTutorialE2E` - all subtests pass
  - [ ] Add/edit/delete posts work
- [ ] Run counter example test
  - [ ] `TestCounterE2E` - all subtests pass
- [ ] Run chat example test (if exists)
  - [ ] All message operations work

**Success Criteria**:
- [ ] All E2E tests pass
- [ ] No WebSocket update issues
- [ ] No DOM rendering issues

---

#### 4.3 WebSocket verification ⏸️
- [ ] Capture WebSocket messages in tests
  - [ ] Add logging to capture actual messages
  - [ ] Verify structure includes/excludes statics correctly
- [ ] Test empty → first item transition
  - [ ] Initial: `{"d": [], "s": [""]}`
  - [ ] Update: `{"d": [{"0": "...", "1": "..."}], "s": [...]}`
  - [ ] Verify statics ARE included
- [ ] Test first → second item transition
  - [ ] Update: `[["a", [{"0": "...", "1": "..."}]]]`
  - [ ] Verify statics are NOT included (client has them)
- [ ] Test conditional switch
  - [ ] From else → if branch
  - [ ] Verify complete structure sent (new branch)

**Success Criteria**:
- [ ] WebSocket messages follow specification exactly
- [ ] Statics included when required
- [ ] Statics excluded when not required
- [ ] All transitions verified

---

#### 4.4 Documentation ⏸️
- [ ] Update ARCHITECTURE_IMPROVEMENTS.md
  - [ ] Document structure signature system
  - [ ] Add phase 7: Structure Registry (complete)
- [ ] Update docs/specifications/tree-update-specification.md
  - [ ] Document signature system
  - [ ] Add examples of structure transitions
- [ ] Create docs/architecture/structure-signatures.md
  - [ ] Explain signature calculation
  - [ ] Diagram structure transitions
  - [ ] Document registry behavior
- [ ] Update code comments
  - [ ] Add comments to signature functions
  - [ ] Add comments to registry methods
  - [ ] Document key design decisions

**Files Created/Modified**:
- `docs/architecture/structure-signatures.md` (new)
- `ARCHITECTURE_IMPROVEMENTS.md` (updated)
- `docs/specifications/tree-update-specification.md` (updated)

**Success Criteria**:
- [ ] Documentation complete and accurate
- [ ] Design decisions documented
- [ ] Future maintainers can understand system

---

#### 4.5 Performance verification ⏸️
- [ ] Benchmark signature calculation
  - [ ] Simple values
  - [ ] TreeNodes
  - [ ] Large ranges
- [ ] Benchmark registry operations
  - [ ] HasSeen() lookup
  - [ ] MarkSeen() storage
- [ ] Compare with baseline performance
  - [ ] Ensure no regression
  - [ ] Registry operations should be O(1)
- [ ] Run stress tests
  - [ ] 1000+ concurrent sessions
  - [ ] 100+ updates per second

**Success Criteria**:
- [ ] No performance regression
- [ ] Registry operations are fast (< 1μs)
- [ ] Stress tests pass

---

### Phase 4 Checkpoint

**Final Validation**:
- [ ] All tests pass (core + E2E)
- [ ] No known bugs
- [ ] Documentation complete
- [ ] Performance acceptable
- [ ] Code review approved
- [ ] Git commit: "feat: Complete structure registry implementation"

**Estimated Time**: 6-8 hours

---

## Success Criteria (Overall)

### Functional Requirements
- [x] Bug fix: Empty → first item sends complete structure with statics
- [x] Bug fix: Conditional switches send complete structure
- [x] Bug fix: Nested structures tracked correctly
- [x] All E2E tests pass (todos, tutorial, counter, chat)
- [x] No `[object Object]` in DOM
- [x] WebSocket updates follow specification exactly

### Code Quality Requirements
- [x] Single source of truth for structure tracking
- [x] All ad-hoc "has client seen" logic removed
- [x] Code is maintainable and well-documented
- [x] Test coverage comprehensive

### Performance Requirements
- [x] No performance regression
- [x] Registry operations O(1)
- [x] Memory usage acceptable

---

## Risk Mitigation

### Risk 1: Breaking Changes
**Mitigation**: 
- Feature flag to enable/disable registry
- Parallel code during Phase 2
- Comprehensive test suite before removing old code

### Risk 2: Performance Regression
**Mitigation**:
- Benchmark at each phase
- Registry uses efficient hash maps
- Early performance testing in Phase 1

### Risk 3: Complex Edge Cases
**Mitigation**:
- Comprehensive test suite in Phase 4
- Fuzz testing for structure transitions
- Real app testing (todos, tutorial)

### Risk 4: Timeline Overrun
**Mitigation**:
- Clear phase boundaries
- Checkpoint before each phase
- Can pause/resume at any phase

---

## Rollback Plan

If critical issues discovered at any phase:

1. **Phase 1**: Simply don't proceed to Phase 2
2. **Phase 2**: Revert integration commits, keep signature system
3. **Phase 3**: Restore deleted code from git history
4. **Phase 4**: Revert entire feature branch

All phases have clear git commit boundaries for rollback.

---

## Timeline Estimates

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 1: Core Infrastructure | 6-8 hours | Day 1 |
| Phase 2: Integration | 6-8 hours | Day 2 |
| Phase 3: Cleanup | 4-6 hours | Day 3 |
| Phase 4: Validation | 6-8 hours | Day 4 |
| **Total** | **22-30 hours** | **4 days** |

---

## Dependencies

- Go 1.21+ (for template package)
- Existing test infrastructure
- WebSocket test framework
- Chrome E2E test setup

---

## Notes

- This is the CORRECT long-term solution
- Eliminates entire class of "missing statics" bugs
- Makes system specification-compliant by design
- Simplifies codebase by removing ad-hoc logic
- Foundation for future optimizations

---

## Updates Log

| Date | Phase | Update |
|------|-------|--------|
| 2025-10-26 | - | Implementation plan created |

