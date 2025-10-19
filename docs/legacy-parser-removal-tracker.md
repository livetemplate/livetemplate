# Legacy Parser Removal - Multi-Session Tracker

**Project:** LiveTemplate - Remove legacy regex-based parser, keep only AST parser
**Created:** 2025-10-19
**Status:** ✅ COMPLETED (Option A - Single Commit Approach)
**Completed:** 2025-10-19

---

## Overview

**Goal:** Remove the legacy regex-based template parser and make the AST-based parser (`parseTemplateToTreeAST`) the only implementation.

**Current State:**
- File: `tree.go`
- Total lines: 2,509
- Total functions: 113
- UseASTParser: true (line 24)
- All legacy code isolated to tree.go only

**Expected Outcome:**
- File: `tree.go`
- Expected lines: ~1,000-1,100
- Reduction: ~1,500 lines (60%)
- Functions removed: 32
- Types removed: 2

---

## Session Progress Checklist

- [x] **Session 1:** Switch to AST-only & Validate
- [x] **Session 2:** Remove Legacy Types
- [x] **Session 3:** Remove Expression Extraction Functions
- [x] **Session 4:** Remove Tree Building & Field Helper Functions
- [x] **Session 5:** Remove Expression Evaluation Functions
- [x] **Final Validation:** Full test suite & verification

**Note:** All sessions completed together in a single commit (Option A approach) due to pre-commit hook requirements.

---

## Session 1: Switch to AST-only & Validate

**Status:** Not Started
**Goal:** Make AST parser the only path, verify everything works

### Changes to tree.go

1. **Remove UseASTParser variable** (lines 20-24)
   ```go
   // DELETE THESE LINES:
   // UseASTParser is a feature flag to switch between regex and AST-based parsing
   // Set to true to use the new AST-based parser (tree_ast.go)
   // Set to false to use the legacy regex-based parser
   // Default: true (AST parser is now the default as of October 2025)
   var UseASTParser = true
   ```

2. **Simplify parseTemplateToTree** (lines 412-448)
   - Remove the `if UseASTParser` conditional (line 421)
   - Remove legacy implementation (lines 425-447)
   - Keep panic recovery wrapper
   - Call `parseTemplateToTreeAST` directly

   ```go
   // REPLACE lines 412-448 with:
   func parseTemplateToTree(templateStr string, data interface{}, keyGen *KeyGenerator) (tree TreeNode, err error) {
       // Recover from panics in template execution (can happen with fuzz-generated templates)
       defer func() {
           if r := recover(); r != nil {
               err = fmt.Errorf("template execution panic: %v", r)
           }
       }()

       return parseTemplateToTreeAST(templateStr, data, keyGen)
   }
   ```

### Testing
```bash
# Run full test suite
go test -v ./... -timeout=30s

# Run E2E tests specifically
go test -run TestE2E -v

# Check for any test failures
echo $?  # Should be 0
```

### Git Commit
```bash
git add tree.go
git commit -m "refactor: make AST parser the only implementation

- Remove UseASTParser feature flag
- Simplify parseTemplateToTree to only call parseTemplateToTreeAST
- Remove legacy regex-based implementation path
- Keep panic recovery wrapper and normalizeTemplateSpacing function"
```

### Expected Impact
- Lines removed: ~35
- Test failures: 0 expected

### Completion Checklist
- [ ] Code changes made
- [ ] Tests pass
- [ ] Committed to git
- [ ] No pre-commit hook failures

---

## Session 2: Remove Legacy Types

**Status:** Not Started
**Goal:** Remove types only used by regex parser

### Changes to tree.go

1. **Delete ConditionalRange struct** (lines 548-552)
   ```go
   // DELETE:
   type ConditionalRange struct {
       Text  string
       Start int
       End   int
   }
   ```

2. **Delete TemplateExpression struct** (lines 723-728)
   ```go
   // DELETE:
   type TemplateExpression struct {
       Text  string
       Type  string // "field", "conditional", "range"
       Start int
       End   int
   }
   ```

### Testing
```bash
go test -v ./... -timeout=30s
```

### Git Commit
```bash
git add tree.go
git commit -m "refactor: remove legacy regex parser types

- Remove ConditionalRange struct
- Remove TemplateExpression struct
- These types were only used by the removed regex parser"
```

### Expected Impact
- Lines removed: ~10
- Test failures: 0 expected

### Completion Checklist
- [ ] Code changes made
- [ ] Tests pass
- [ ] Committed to git

---

## Session 3: Remove Expression Extraction Functions

**Status:** Not Started
**Goal:** Remove pattern detection and expression extraction functions

### Functions to Delete (10 functions)

| Line | Function Name | Description |
|------|--------------|-------------|
| 451 | `extractFlattenedExpressions` | Extract all template expressions |
| 555 | `detectSimpleRanges` | Find simple range patterns |
| 595 | `isInsideSimpleRange` | Check if position inside simple range |
| 605 | `detectConditionalRanges` | Find conditional range patterns |
| 658 | `extractIfCondition` | Extract if condition text |
| 672 | `isRangeAtTopLevel` | Check if range at top level |
| 713 | `isInsideConditionalRange` | Check if position inside conditional |
| 1301 | `extractWithBlock` | Extract with block |
| 1328 | `extractRangeBlock` | Extract range block |
| 1370 | `extractConditionalBlock` | Extract conditional block |

### Testing
```bash
go test -v ./... -timeout=30s
```

### Git Commit
```bash
git add tree.go
git commit -m "refactor: remove legacy expression extraction functions

Removed 10 functions used only by regex parser:
- extractFlattenedExpressions
- detectSimpleRanges, detectConditionalRanges
- isInsideSimpleRange, isInsideConditionalRange
- extractIfCondition, isRangeAtTopLevel
- extractWithBlock, extractRangeBlock, extractConditionalBlock"
```

### Expected Impact
- Lines removed: ~450
- Test failures: 0 expected

### Completion Checklist
- [ ] All 10 functions deleted
- [ ] Tests pass
- [ ] Committed to git

---

## Session 4: Remove Tree Building & Field Helper Functions

**Status:** Not Started
**Goal:** Remove tree construction and field extraction functions

### Functions to Delete (13 functions)

| Line | Function Name | Description |
|------|--------------|-------------|
| 1288 | `extractWithVariable` | Extract with variable name |
| 1412 | `buildTreeFromExpressions` | Build tree from expressions |
| 1482 | `buildRangeComprehension` | Build range comprehension |
| 1493 | `buildConditionalRange` | Build conditional range |
| 1632 | `buildRegularRangeComprehension` | Build regular range |
| 1672 | `extractRangeField` | Extract range field name |
| 1691 | `extractRangeContent` | Extract range content |
| 1740 | `extractRangeContentWithWrappers` | Extract with wrappers |
| 1851 | `extractRangeFieldName` | Extract range field |
| 1862 | `getFieldValue` | Get field value via reflection |
| 1897 | `generateDynamicDataForItems` | Generate dynamic data |
| 1935 | `evaluateConditionalBlock` | Evaluate conditional |
| 2298 | `extractFieldFromCondition` | Extract field from condition |

### Testing
```bash
go test -v ./... -timeout=30s
```

### Git Commit
```bash
git add tree.go
git commit -m "refactor: remove legacy tree building functions

Removed 13 functions used only by regex parser:
- buildTreeFromExpressions, buildRangeComprehension
- buildConditionalRange, buildRegularRangeComprehension
- extractRangeField, extractRangeContent, extractRangeContentWithWrappers
- extractRangeFieldName, extractWithVariable, extractFieldFromCondition
- getFieldValue, generateDynamicDataForItems
- evaluateConditionalBlock"
```

### Expected Impact
- Lines removed: ~650
- Test failures: 0 expected

### Completion Checklist
- [ ] All 13 functions deleted
- [ ] Tests pass
- [ ] Committed to git

---

## Session 5: Remove Expression Evaluation Functions

**Status:** Not Started
**Goal:** Remove final legacy evaluation functions

### Functions to Delete (9 functions)

| Line | Function Name | Description |
|------|--------------|-------------|
| 2209 | `evaluateConditionalExpression` | Evaluate conditional expr |
| 2227 | `evaluateTemplateExpression` | Evaluate template expr |
| 2248 | `evaluateRangeBlock` | Evaluate range block |
| 2265 | `evaluateRangeExpression` | Evaluate range expr |
| 2324 | `evaluateCondition` | Evaluate condition |
| 2357 | `evaluateEmbeddedFields` | Evaluate embedded fields |
| 2386 | `evaluateConditionalInRangeContext` | Evaluate conditional in range |
| 2458 | `evaluateFieldExpression` | Evaluate field expr |
| 2475 | `findMatchingEndForExpression` | Find matching end tag |

### Testing
```bash
# Run full test suite
go test -v ./... -timeout=30s

# Run linter to check for unused code
golangci-lint run
```

### Git Commit
```bash
git add tree.go
git commit -m "refactor: remove legacy expression evaluation functions

Removed final 9 functions used only by regex parser:
- evaluateConditionalExpression, evaluateTemplateExpression
- evaluateRangeBlock, evaluateRangeExpression
- evaluateCondition, evaluateEmbeddedFields
- evaluateConditionalInRangeContext, evaluateFieldExpression
- findMatchingEndForExpression

This completes the removal of the legacy regex-based parser."
```

### Expected Impact
- Lines removed: ~350
- Test failures: 0 expected

### Completion Checklist
- [ ] All 9 functions deleted
- [ ] Tests pass
- [ ] Linter passes
- [ ] Committed to git

---

## Final Validation

**Status:** Not Started

### Verification Steps

1. **Run complete test suite**
   ```bash
   go test -v ./... -timeout=30s
   ```

2. **Run E2E tests**
   ```bash
   go test -run TestE2E -v
   cd cmd/lvt/e2e && go test -v
   cd ../../..
   ```

3. **Verify line count reduction**
   ```bash
   wc -l tree.go
   # Expected: ~1000-1100 lines (down from 2509)
   ```

4. **Check for unused code**
   ```bash
   golangci-lint run
   ```

5. **Verify function count**
   ```bash
   grep -c "^func " tree.go
   # Expected: ~81 functions (down from 113)
   ```

6. **Check for orphaned imports**
   ```bash
   # Review tree.go imports, remove any unused
   ```

### Final Commit (Optional)
```bash
git add tree.go
git commit -m "docs: update after legacy parser removal

- Removed 32 functions and 2 types
- Reduced tree.go from 2509 to ~1100 lines
- AST parser is now the only implementation"
```

### Completion Checklist
- [ ] All tests pass
- [ ] E2E tests pass
- [ ] Line count verified (~1000-1100)
- [ ] Linter clean
- [ ] No orphaned imports
- [ ] All 5 sessions committed

---

## Functions to KEEP (Not Legacy)

These functions are used by the AST parser or other parts of the system:

- `normalizeTemplateSpacing` - Used by AST parser
- `ParseTemplateToTreeForTesting` - Test export
- All `KeyGenerator` methods
- All `calculateFingerprint` functions
- All `Construct` types and interfaces
- All HTML parsing utilities (`injectWrapperDiv`, etc.)
- All compiled construct types

---

## Rollback Instructions

If any session causes issues:

```bash
# Rollback the last commit
git revert HEAD

# Or reset to before the problematic commit
git log --oneline  # Find the commit before the issue
git reset --hard <commit-hash>

# Re-run tests to verify
go test -v ./... -timeout=30s
```

---

## Progress Log

### All Sessions (Combined - Option A)
- **Date:** 2025-10-19
- **Status:** ✅ COMPLETED
- **Commit:** 8037cef "refactor: remove legacy regex parser - complete cleanup"
- **Notes:**
  - Pre-commit hook prevented incremental commits with unused code
  - Chose Option A: remove all legacy code in single commit
  - Used Task agent with general-purpose subagent to perform systematic removal
  - All 37+ legacy functions removed
  - Both legacy types (ConditionalRange, TemplateExpression) removed
  - minify.go file deleted (functions unused)
  - Additional cleanup: removed 6 more unused helper functions
  - Final line count: 776 lines (down from 2,509)
  - Total reduction: 1,733 lines (69%)
  - All core tests passing
  - Pre-commit hook passed (formatting + linting + tests)

### Final Validation
- **Date:** 2025-10-19
- **Status:** ✅ PASSED
- **Notes:**
  - Build: ✅ SUCCESS
  - Core tests: ✅ ALL PASSING
  - Client tests: ✅ ALL PASSING
  - Pre-commit hook: ✅ PASSED
  - E2E tests: Known issue (was failing before changes)

---

## Summary Statistics

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| Total Lines | 2,509 | 776 | -1,733 (-69%) |
| Total Functions | 113 | ~75 | -38+ (-34%) |
| Legacy Functions | 37+ | 0 | -37+ (-100%) |
| Legacy Types | 2 | 0 | -2 (-100%) |
| Files | tree.go, minify.go | tree.go | -1 file |

---

## Notes

- Each session is independent and can be done separately
- Always run tests after each session before committing
- The pre-commit hook will run tests automatically - do NOT skip it
- If any session fails, rollback and investigate before proceeding
- Update this tracker as you complete each session
