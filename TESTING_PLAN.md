# Internal Package Unit Testing Plan

**Status**: 🚧 In Progress
**Branch**: `feature/internal-unit-tests`
**Created**: 2025-11-04
**Target**: Comprehensive unit test coverage for all internal packages

## Executive Summary

This document tracks the implementation of comprehensive unit tests for LiveTemplate's internal packages. Currently, `internal/parse/`, `internal/build/`, and `internal/diff/` have ZERO unit tests, relying entirely on integration tests in the main package. This plan addresses this gap with ~212 new unit tests across 15 test files.

## Current State Analysis

### Test Coverage (Before)

| Package | Test Files | Test Count | Coverage Level |
|---------|-----------|-----------|----------------|
| Main package | 16 | ~243 | ✅ High (integration/E2E) |
| internal/parse | 0 | 0 | ❌ None |
| internal/build | 0 | 0 | ❌ None |
| internal/diff | 0 | 0 | ❌ None |
| internal/observe | 2 | 20 | ✅ Good |
| **TOTAL** | **18** | **~263** | |

### Issues Identified

1. **Zero Unit Test Coverage**: Core packages have no isolated unit tests
2. **Misplaced Tests**: TreeNode tests in root `tree_test.go` should be in `internal/build/types_test.go`
3. **Integration-Only Testing**: Complex logic only tested through full pipeline
4. **Debugging Difficulty**: Failures require debugging entire stack
5. **Refactoring Risk**: No safety net for internal refactoring

## Implementation Strategy

**Priority Order**: Critical-first (diff → parse → build → observe)

### Why Critical-First?

1. **internal/diff/**: Most critical for correctness
   - `PrepareTreeForClient()` implements wire format specification
   - Range differential operations are algorithmically complex
   - Bugs here = incorrect updates sent to clients

2. **internal/parse/**: Complex parsing logic needs isolation
   - Range parsing with variable context is intricate
   - Conditional and field handling has many edge cases
   - Bugs here = incorrect tree generation

3. **internal/build/**: Infrastructure and optimization
   - Fingerprinting affects caching behavior
   - Key generation must be stable
   - Bugs here = inefficient updates or cache misses

4. **internal/observe/**: Already well-tested
   - Has 20 existing tests
   - Low complexity
   - Can be completed last

## Implementation Phases

### ✅ Phase 0: Setup
- [x] Verify `.gitignore` contains `.worktrees/`
- [x] Create git worktree: `.worktrees/internal-unit-tests`
- [x] Create `TESTING_PLAN.md` tracker document

### 🚧 Phase 1: internal/diff/ Tests (79 tests) - CRITICAL

#### prepare_test.go (~7 tests)
**Priority**: CRITICAL - Wire format specification compliance

- [ ] `TestPrepareTreeForClient_WithStatics` - Client has statics cached
- [ ] `TestPrepareTreeForClient_WithoutStatics` - First render, client needs statics
- [ ] `TestPrepareTreeForClient_Nested` - Nested tree structures
- [ ] `TestPrepareTreeForClient_Range` - Range constructs
- [ ] `TestPrepareTreeForClient_Empty` - Empty tree edge case
- [ ] `TestPrepareTreeForClient_Map` - Map value handling
- [ ] `TestPrepareTreeForClient_Recursion` - Recursive static stripping

**Rationale**: This function implements the critical wire format optimization per `tree-update-specification.md`. First render includes statics, updates strip them. Bugs here violate the spec and break client updates.

#### range_ops_test.go (~18 tests)
**Priority**: CRITICAL - Complex range differential algorithm

- [ ] `TestGenerateRangeDifferentialOperations_NoChange` - No changes
- [ ] `TestGenerateRangeDifferentialOperations_PureReorder` - Pure reordering
- [ ] `TestGenerateRangeDifferentialOperations_Removal` - Item removal
- [ ] `TestGenerateRangeDifferentialOperations_Update` - Item update
- [ ] `TestGenerateRangeDifferentialOperations_Insertion` - Item insertion
- [ ] `TestGenerateRangeDifferentialOperations_Mixed` - Mixed operations
- [ ] `TestGenerateRangeDifferentialOperations_EmptyToItems` - Empty → items
- [ ] `TestGenerateRangeDifferentialOperations_ItemsToEmpty` - Items → empty
- [ ] `TestGenerateRangeDifferentialOperations_StripStatics` - Static stripping
- [ ] `TestExtractRangeData_TreeNode` - TreeNode extraction
- [ ] `TestExtractRangeData_EmptyToItems` - Empty → items static handling
- [ ] `TestGenerateRemovalOperations` - Removal operation generation
- [ ] `TestGenerateUpdateOperations` - Update operation generation
- [ ] `TestGenerateInsertionOperations_Prepend` - Prepend operations
- [ ] `TestGenerateInsertionOperations_Append` - Append operations
- [ ] `TestGenerateInsertionOperations_Complex` - Complex insertion patterns
- [ ] `TestCompareRangeItemsForChanges_NoDiff` - No item changes
- [ ] `TestCompareRangeItemsForChanges_Changed` - Item changes detected

**Rationale**: Range differential operations are the most algorithmically complex part of the codebase. They generate `["u", ...], ["i", ...], ["r", ...], ["o", ...]` operations that must be correct for list updates to work.

#### tree_compare_test.go (~27 tests)
**Priority**: CRITICAL - Main diffing orchestrator

- [ ] `TestCompareTreesAndGetChangesWithPath_NoDiff` - No changes detected
- [ ] `TestCompareTreesAndGetChangesWithPath_SimpleDiff` - Simple field change
- [ ] `TestCompareTreesAndGetChangesWithPath_NestedDiff` - Nested tree changes
- [ ] `TestCompareTreesAndGetChangesWithPath_NilTrees` - Nil handling
- [ ] `TestCompareTreesAndGetChangesWithPath_TopLevelRange` - Top-level range
- [ ] `TestHandleTopLevelRange_BothRanges` - Both trees are ranges
- [ ] `TestHandleTopLevelRange_NewRange` - New range appearing
- [ ] `TestHandleMatchedRanges_WithOps` - Range with operations
- [ ] `TestHandleMatchedRanges_EmptyRanges` - Both ranges empty
- [ ] `TestCompareDynamicSegments_NewField` - New field addition
- [ ] `TestCompareDynamicSegments_ChangedField` - Field value change
- [ ] `TestCompareDynamicSegments_UnchangedField` - No change optimization
- [ ] `TestBuildFieldPath` - Path building for nested fields
- [ ] `TestHandleNewField_Primitive` - New primitive value
- [ ] `TestHandleNewField_TreeNode` - New TreeNode value
- [ ] `TestHandleNewField_Map` - New map value
- [ ] `TestHandleNewTreeNodeField` - TreeNode field handling
- [ ] `TestHandleNewMapField` - Map field handling
- [ ] `TestHandleChangedField_RangeMatch` - Matched range structures
- [ ] `TestHandleChangedField_TreeNodes` - TreeNode comparison
- [ ] `TestHandleChangedField_TypeChange` - Value type changed
- [ ] `TestExtractTreeNodePair` - TreeNode pair extraction
- [ ] `TestHandleNestedTreeNodes_StructureChanged` - Structure mismatch
- [ ] `TestHandleNestedTreeNodes_Similar` - Similar structure
- [ ] `TestHandleStaticOnlyChanges` - Static-only changes
- [ ] `TestHandleNewTreeNodeFromPrimitive` - Primitive → TreeNode
- [ ] `TestIsNilRegistry` - Nil registry detection

**Rationale**: This orchestrates all diff logic. It must correctly detect what changed between two trees and route to appropriate handlers (range ops, nested comparison, etc.).

#### helpers_test.go (~27 tests)
**Priority**: HIGH - Many utility functions

- [ ] `TestIsEmpty_AllTypes` - Empty detection for all types
- [ ] `TestIsRangeConstruct_TreeNode` - TreeNode range detection
- [ ] `TestIsRangeConstruct_Map` - Map range detection
- [ ] `TestIsRangeConstruct_NotRange` - Non-range detection
- [ ] `TestHasRangeItems_WithItems` - Range with items
- [ ] `TestHasRangeItems_Empty` - Empty range
- [ ] `TestContainsRangeConstruct_Direct` - Direct range
- [ ] `TestContainsRangeConstruct_Nested` - Nested range
- [ ] `TestContainsRangeConstruct_None` - No range
- [ ] `TestAreStructuresSimilar_Similar` - Similar structures
- [ ] `TestAreStructuresSimilar_Different` - Different structures
- [ ] `TestDeepEqual_AllTypes` - Deep equality for all types
- [ ] `TestFindKeyPositionFromStatics` - Key position in statics
- [ ] `TestGetItemKey_WithKey` - Explicit key extraction
- [ ] `TestGetItemKey_NoKey` - Hash-based key
- [ ] `TestGenerateItemHash` - Item hashing
- [ ] `TestExtractItemKeys` - Key extraction from items
- [ ] `TestDetectPositionField` - Position field detection
- [ ] `TestIsPureReordering_True` - Reordering detected
- [ ] `TestIsPureReordering_False` - Not reordering
- [ ] `TestFindNewItems` - New item detection
- [ ] `TestAreAllItemsAtStart` - Start position check
- [ ] `TestAreAllItemsAtEnd` - End position check
- [ ] `TestIsComplexInsertionPattern` - Complex pattern check
- [ ] `TestGetRangeSignature` - Range signature calculation
- [ ] `TestFindRangeConstructs` - Find range constructs
- [ ] `TestFindRangeConstructMatches` - Match range constructs

**Rationale**: These helpers are used throughout diff operations. Bugs here cascade to all diff functionality.

**Phase 1 Total**: 79 tests across 4 files

---

### ⏳ Phase 2: internal/parse/ Tests (62 tests) - CRITICAL

#### range_test.go (~20 tests)
**Priority**: CRITICAL - Most complex parsing logic

- [ ] `TestHandleRangeNode_SimpleSlice` - Basic slice iteration
- [ ] `TestHandleRangeNode_EmptySlice` - Empty collection
- [ ] `TestHandleRangeNode_Map` - Map iteration
- [ ] `TestHandleRangeNode_WithElse` - Range with else branch
- [ ] `TestHandleRangeNode_WithVarDecls` - With `$i`, `$v` declarations
- [ ] `TestExtractRangeCollection_Simple` - Simple `{{range .Items}}`
- [ ] `TestExtractRangeCollection_WithDecls` - With variable declarations
- [ ] `TestExtractRangeCollection_Error` - Error cases
- [ ] `TestIsEmpty_AllTypes` - Empty detection for all types
- [ ] `TestHandleEmptyRange_NoElse` - Empty range, no else
- [ ] `TestHandleEmptyRange_WithElse` - Empty range with else
- [ ] `TestHandleSliceRange` - Slice processing
- [ ] `TestHandleMapRange` - Map processing
- [ ] `TestBuildRangeTree` - Range tree construction
- [ ] `TestExecuteRangeBodyWithVars_SingleVar` - Single variable ($v)
- [ ] `TestExecuteRangeBodyWithVars_TwoVars` - Index + value ($i, $v)
- [ ] `TestExecuteRangeBodyWithVarsMap` - Map key/value vars
- [ ] `TestDetectIDKey_AllPatterns` - All key patterns (id, ID, Id, uuid, etc.)
- [ ] `TestDetectIDKey_Priority` - Priority order when multiple keys
- [ ] `TestDetectIDKey_NoKey` - No key found, return ""

**Rationale**: Range parsing with variable context (`$i`, `$v`) is the most complex parsing logic. ID key detection is critical for range differential operations.

#### conditional_test.go (~13 tests)
**Priority**: HIGH - Conditional logic has many branches

- [ ] `TestHandleIfNode_TrueBranch` - If condition true
- [ ] `TestHandleIfNode_FalseBranch` - If condition false
- [ ] `TestHandleIfNode_WithElse` - If/else branches
- [ ] `TestHandleIfNode_NoElse` - If without else
- [ ] `TestHandleIfNode_NestedIf` - Nested conditionals
- [ ] `TestHandleIfNode_ComplexCondition` - Complex expressions
- [ ] `TestHandleIfNodeWithVars_NoVars` - No variable context
- [ ] `TestHandleIfNodeWithVars_WithVars` - With variables
- [ ] `TestHandleIfNodeWithVars_RootVar` - Root variable `$`
- [ ] `TestMergeFieldsIntoMap_Struct` - Struct field merging
- [ ] `TestMergeFieldsIntoMap_Map` - Map merging
- [ ] `TestMergeFieldsIntoMap_Primitive` - Primitive value handling
- [ ] `TestMergeFieldsIntoMap_Nil` - Nil handling

**Rationale**: Conditional handling must correctly select branches and merge context. Bugs cause wrong content to render.

#### parse_test.go (~17 tests)
**Priority**: HIGH - Core parsing infrastructure

- [ ] `TestParse_SimpleTemplate` - Basic template parsing
- [ ] `TestParse_WithFuncMap` - FuncMap integration
- [ ] `TestParse_InvalidSyntax` - Syntax error handling
- [ ] `TestParse_EmptyTemplate` - Empty template edge case
- [ ] `TestBuildTree_SimpleField` - Simple field extraction
- [ ] `TestBuildTree_NestedFields` - Nested data structures
- [ ] `TestBuildTreeFromAST_TextNode` - Static text nodes
- [ ] `TestBuildTreeFromAST_ActionNode` - Dynamic action nodes
- [ ] `TestBuildTreeFromAST_CommentNode` - Comment handling
- [ ] `TestBuildTreeFromList_SingleNode` - Single node in list
- [ ] `TestBuildTreeFromList_MultipleNodes` - Node merging
- [ ] `TestBuildTreeFromList_EmptyList` - Empty list edge case
- [ ] `TestEvaluatePipe_Simple` - Simple dot access `.Field`
- [ ] `TestEvaluatePipe_Complex` - Pipeline evaluation `.Field | func`
- [ ] `TestEvaluatePipe_WithFuncs` - Function calls in pipeline
- [ ] `TestFormatPipe` - Pipe formatting
- [ ] `TestIsZeroValue_AllTypes` - Zero value detection for all types

**Rationale**: Core parsing infrastructure. Must correctly walk AST and build tree structures.

#### field_test.go (~12 tests)
**Priority**: MEDIUM - Field handling is straightforward but critical

- [ ] `TestHandleActionNode_SimpleField` - `{{.Field}}`
- [ ] `TestHandleActionNode_Method` - `{{.Method}}`
- [ ] `TestHandleActionNode_Pipeline` - `{{.Field | func}}`
- [ ] `TestHandleActionNode_Error` - Error cases
- [ ] `TestHandleActionNodeWithVars_NoVars` - No variable context
- [ ] `TestHandleActionNodeWithVars_WithVars` - With `$var`
- [ ] `TestHandleActionNodeWithVars_RootVar` - With `$` root
- [ ] `TestEvaluateActionWithVars_SingleVar` - Single variable
- [ ] `TestEvaluateActionWithVars_MultipleVars` - Multiple variables
- [ ] `TestEvaluateActionWithVars_RootVar` - Root variable
- [ ] `TestDetectsRootVariable` - Root variable detection
- [ ] `TestIsLetter` - Letter character check

**Rationale**: Field extraction is the most common operation. Must handle all field types and variable contexts.

**Phase 2 Total**: 62 tests across 4 files

---

### ⏳ Phase 3: internal/build/ Tests (63 tests) - HIGH

#### types_test.go (~20 tests)
**Priority**: HIGH - Move from root + add new tests

**Tests to Move from tree_test.go** (~15 tests):
- [ ] Move `TestNewTreeNode`
- [ ] Move `TestNewTreeNodeWithStatics`
- [ ] Move `TestTreeNode_SetDynamic`
- [ ] Move `TestTreeNode_GetDynamic`
- [ ] Move `TestTreeNode_HasStatics`
- [ ] Move `TestTreeNode_HasDynamics`
- [ ] Move `TestTreeNode_HasRange`
- [ ] Move `TestTreeNode_MarshalJSON`
- [ ] Move `TestTreeNode_UnmarshalJSON`
- [ ] Move `TestTreeNode_ToMap`
- [ ] Move `TestTreeNode_FromMap`
- [ ] Move `TestTreeNode_Clone`
- [ ] Move `TestTreeNode_NestedClone`
- [ ] Move `TestRangeData_Creation`
- [ ] Move `TestTreeMetadata_Creation`

**New Tests** (~5 tests):
- [ ] `TestContext_NewContext` - Context creation
- [ ] `TestContext_NewUpdateContext` - Update context creation
- [ ] `TestContext_ShouldIncludeStatics` - Static inclusion logic
- [ ] `TestRangeData_NewRangeData` - RangeData constructor
- [ ] `TestTreeMetadata_NewTreeMetadata` - Metadata constructor

**Rationale**: TreeNode is defined in `internal/build/types.go`, so tests should be colocated there. Moving improves organization and makes internal package tests self-contained.

#### fingerprint_test.go (~12 tests)
**Priority**: HIGH - Fingerprinting affects caching

- [ ] `TestCalculateFingerprint_Simple` - Simple tree fingerprint
- [ ] `TestCalculateFingerprint_Nested` - Nested tree structures
- [ ] `TestCalculateFingerprint_Range` - Range constructs
- [ ] `TestCalculateFingerprint_Deterministic` - Same input = same hash
- [ ] `TestCalculateFingerprint_Different` - Different inputs ≠ same hash
- [ ] `TestHashTreeIncremental_Statics` - Static array hashing
- [ ] `TestHashTreeIncremental_Dynamics` - Dynamic value hashing
- [ ] `TestHashTreeIncremental_Nested` - Nested tree hashing
- [ ] `TestHashValueIncremental_AllTypes` - All value types
- [ ] `TestHashValueIncremental_TreeNode` - TreeNode hashing
- [ ] `TestHashValueIncremental_Array` - Array hashing
- [ ] `TestAddFingerprintToTree` - Fingerprint attachment to tree

**Rationale**: Fingerprinting determines if updates are needed. Bugs cause unnecessary updates or missed changes.

#### key_test.go (~9 tests)
**Priority**: HIGH - Key generation must be stable

- [ ] `TestNewKeyGenerator` - KeyGenerator constructor
- [ ] `TestKeyGenerator_NextKey` - Sequential key generation
- [ ] `TestKeyGenerator_Reset` - Reset behavior
- [ ] `TestKeyGenerator_LoadExistingKeys` - Load keys from tree
- [ ] `TestKeyGenerator_Uniqueness` - No duplicate keys
- [ ] `TestGenerateWrapperKey` - Wrapper key generation
- [ ] `TestDetectIDKey_AllPatterns` - All ID patterns
- [ ] `TestDetectIDKey_Priority` - Priority ordering
- [ ] `TestDetectIDKey_NoKey` - Default behavior when no key

**Rationale**: Keys must be stable across renders for updates to target correct elements. Bugs cause updates to wrong elements or full re-renders.

#### render_test.go (~12 tests)
**Priority**: MEDIUM - HTML rendering for initial state

- [ ] `TestRenderNode_TextNode` - Text node rendering
- [ ] `TestRenderNode_ElementNode` - Element node rendering
- [ ] `TestRenderNode_VoidElement` - Void elements (`<br>`, `<img>`, etc.)
- [ ] `TestRenderNode_WithAttributes` - Element with attributes
- [ ] `TestRenderNode_NestedElements` - Nested element structures
- [ ] `TestIsVoidHTMLElement_AllVoid` - All void elements recognized
- [ ] `TestIsVoidHTMLElement_NonVoid` - Non-void elements
- [ ] `TestRenderTreeToHTML_Simple` - Simple tree to HTML
- [ ] `TestRenderTreeToHTML_WithDynamics` - Tree with dynamic values
- [ ] `TestRenderTreeToHTML_Nested` - Nested tree to HTML
- [ ] `TestRenderTreeToHTML_Error` - Error handling
- [ ] `TestRenderRangeComprehensionToHTML` - Range rendering

**Rationale**: HTML rendering creates initial page content. Bugs cause malformed HTML or incorrect initial state.

#### wrapper_test.go (~10 tests)
**Priority**: MEDIUM - Wrapper injection for update targeting

- [ ] `TestGenerateRandomID_Uniqueness` - IDs are unique
- [ ] `TestGenerateRandomID_Format` - ID format `lvt-[random]`
- [ ] `TestInjectWrapperDiv_FullDocument` - Full HTML document wrapping
- [ ] `TestInjectWrapperDiv_Fragment` - Fragment wrapping
- [ ] `TestInjectWrapperDiv_WithLoading` - Loading indicator injection
- [ ] `TestInjectWrapperDiv_LoadingDisabled` - No loading indicator
- [ ] `TestExtractTemplateBodyContent` - Body content extraction
- [ ] `TestExtractTemplateContent` - Content extraction
- [ ] `TestFindElementByDataLvtID` - Element finding by lvt ID
- [ ] `TestNormalizeTemplateSpacing` - Whitespace normalization

**Rationale**: Wrapper div provides target for client-side updates. Bugs cause updates to fail to target correctly.

**Phase 3 Total**: 63 tests across 5 files

---

### ⏳ Phase 4: internal/observe/ Tests (8 tests) - LOW

#### logger_test.go (~4 tests)
**Priority**: LOW - Simple logger functionality

- [ ] `TestNewLogger_Default` - Default logger creation
- [ ] `TestNewLogger_CustomLevel` - Custom log level
- [ ] `TestNewLogger_CustomHandler` - Custom handler
- [ ] `TestLogger_Methods` - Info, Warn, Error methods

**Rationale**: Logger is simple and already indirectly tested. Low priority.

#### metrics_test.go (~4 tests)
**Priority**: LOW - Metrics already well-tested via prometheus_test.go

- [ ] `TestNewMetrics` - Metrics constructor
- [ ] `TestMetrics_AllMethods` - All metric recording methods
- [ ] `TestDurationHistogram_Record` - Histogram recording
- [ ] `TestDurationHistogram_Quantiles` - Quantile calculation

**Rationale**: Metrics are already tested via prometheus_test.go integration tests. Additional unit tests are nice-to-have.

**Phase 4 Total**: 8 tests across 2 files

---

### ⏳ Phase 5: Verification & Completion

- [ ] Run full test suite: `go test -v ./...`
- [ ] Run with race detector: `go test -race ./...`
- [ ] Check test coverage: `go test -cover ./internal/...`
- [ ] Verify all pre-commit hooks pass
- [ ] Clean up any temporary test files
- [ ] Update this tracker with final statistics
- [ ] Commit all changes (no --no-verify)
- [ ] Create pull request with comprehensive description

---

## Test Guidelines

### Test Structure

```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    input := setupInput()
    expected := expectedOutput()

    // Act
    result := FunctionUnderTest(input)

    // Assert
    if !reflect.DeepEqual(result, expected) {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

### Table-Driven Tests

```go
func TestFunction_MultipleCases(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case1", "input1", "output1"},
        {"case2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Function(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Golden Files

For complex HTML/JSON output:
```go
golden := filepath.Join("testdata", "test_name.golden.json")
if *update {
    os.WriteFile(golden, resultJSON, 0644)
}
expected, _ := os.ReadFile(golden)
// Compare result with expected
```

### Best Practices

1. **Clear Test Names**: `TestFunction_Scenario` pattern
2. **Table-Driven**: Use for multiple similar cases
3. **No Test-Only Code**: Don't add test-only methods to production code
4. **Test Error Paths**: Not just happy paths
5. **Isolated Tests**: Each test should be independent
6. **Minimal Setup**: Keep test setup simple and focused
7. **Clear Assertions**: Make failures obvious

---

## Progress Tracking

### Summary Statistics

| Phase | Status | Files | Tests | % Complete |
|-------|--------|-------|-------|-----------|
| Phase 0: Setup | ✅ Done | 1 | 0 | 100% |
| Phase 1: internal/diff/ | 🚧 In Progress | 4 | 79 | 0% |
| Phase 2: internal/parse/ | ⏳ Pending | 4 | 62 | 0% |
| Phase 3: internal/build/ | ⏳ Pending | 5 | 63 | 0% |
| Phase 4: internal/observe/ | ⏳ Pending | 2 | 8 | 0% |
| **TOTAL** | **🚧 In Progress** | **16** | **212** | **0%** |

### Timeline

- **Phase 0 (Setup)**: ✅ Complete
- **Phase 1 (internal/diff/)**: In Progress
- **Phase 2 (internal/parse/)**: Not Started
- **Phase 3 (internal/build/)**: Not Started
- **Phase 4 (internal/observe/)**: Not Started
- **Phase 5 (Completion)**: Not Started

---

## Success Criteria

✅ Checklist for completion:

- [ ] All 15 test files created
- [ ] All ~212 tests implemented
- [ ] TreeNode tests moved from root to internal/build/types_test.go
- [ ] All tests passing (`go test -v ./...`)
- [ ] No race conditions (`go test -race ./...`)
- [ ] Pre-commit hooks passing (format, tests)
- [ ] Test coverage >80% for internal packages
- [ ] Documentation updated
- [ ] Pull request created with comprehensive description

---

## Notes & Learnings

### Key Architectural Insights

1. **Wire Format Optimization**: `PrepareTreeForClient()` strips statics on updates per spec
2. **Range Differential Operations**: Most complex algorithm, requires comprehensive testing
3. **Key Generation**: Must be stable across renders for updates to work
4. **Fingerprinting**: Determines if updates needed, affects performance

### Testing Challenges

_(Document challenges and solutions as work progresses)_

---

## References

- **Main Package Tests**: Root `*_test.go` files (integration/E2E tests)
- **Tree Update Specification**: `tree-update-specification.md`
- **Project Guidelines**: `CLAUDE.md`
- **Git Branch**: `feature/internal-unit-tests`
- **Worktree Location**: `.worktrees/internal-unit-tests`

---

_Last Updated: 2025-11-04_
