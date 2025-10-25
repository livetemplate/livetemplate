# LiveTemplate Architecture Improvements

**Branch**: `feat/architecture-improvements`
**Start Date**: 2025-10-25
**Status**: In Progress
**Overall Progress**: Phases 1-2 COMPLETE ✅ (2 of 6 phases done)

---

## Executive Summary

This document tracks the implementation of comprehensive architecture improvements to LiveTemplate, addressing 5 verified server-side issues and significant client complexity (2,252 lines). The improvements focus on fixing critical bugs, adding type safety, optimizing performance, and dramatically simplifying both server and client implementations.

### Verified Issues

1. **Stateful Structure Bug** - Server only checks `initialTree`, not tracking all sent structures
2. **O(N) Fingerprinting** - Full tree marshaling on every render
3. **Complex Range Matching** - Two-pass brittle static signature matching
4. **Ambiguous Range Operations** - Redundant positioning parameters
5. **Non-Type-Safe TreeNode** - Magic strings ("s", "d", "f") throughout codebase

### Client Complexity
- 2,252 lines with complex state tracking
- `findKeyPositionFromStatics` - 50+ lines of HTML parsing
- `applyDifferentialOperations` - 100+ lines of operation handling
- Deep merge logic and path tracking

---

## Simplified Operation Set (Phase 3 Deliverable)

```typescript
['u', id, changes]   // Update item by ID (O(n))
['r', id]            // Remove item by ID (O(n))
['a', data]          // Append to end (O(1) - common: infinite scroll, logs)
['p', data]          // Prepend to start (O(1) - common: newest-first feeds)
['i', afterId, data] // Insert after ID (O(n) - specific positioning)
['o', [ids]]         // Reorder list (O(n))
```

---

## Phase 1: Fix Stateful Structure Bug ✅

**Priority**: CRITICAL
**Status**: ✅ COMPLETED
**Duration**: 2-3 days
**Completed**: 2025-10-25

### Problem Statement

Server only checks `initialTree` to determine if client has cached statics. Dynamically-appearing structures (modals, conditional branches) not tracked in `seenStructures`, causing statics to be resent on every appearance.

**Bug Scenario**:
```
Render 1: Modal hidden → not in initialTree
Render 2: Modal shown → sends WITH statics ✅
Render 3: Modal hidden
Render 4: Modal shown → sends WITH statics again ❌ (BUG!)
```

### Solution

Add `seenStructures map[string]bool` to Template struct to track ALL structures sent across renders.

### Implementation Checklist

#### Code Changes
- [x] 1.1: Add `seenStructures map[string]bool` field to Template struct (template.go:~135)
- [x] 1.2: Initialize `seenStructures` in `New()` function
- [x] 1.3: Initialize `seenStructures` in `Parse()` function
- [x] 1.4: Initialize `seenStructures` in `Must()` function (N/A - no Must() in implementation)
- [x] 1.5: Update `clientHasStructure` logic at line ~984 (new field appears)
- [x] 1.6: Update `clientHasStructure` logic at line ~1161 (field changed)
- [x] 1.7: Add structure tracking after sending (both code paths)
- [ ] 1.8: Add `Reset()` method to clear `seenStructures` if needed (OPTIONAL - deferred)

#### Testing
- [x] 1.9: Create `template_dynamic_structure_test.go`
- [x] 1.10: Test: Modal toggle (hide → show → hide → show) - 82% reduction achieved!
- [x] 1.11: Test: Conditional branch switch (A → B → A) - statics cached correctly!
- [x] 1.12: Test: Nested dynamic structures - 65% reduction achieved!
- [x] 1.13: Verify all existing tests pass - core library tests passing
- [ ] 1.14: Add benchmark for structure tracking overhead (OPTIONAL - deferred)

#### Documentation
- [ ] 1.15: Update `docs/specifications/tree-update-specification.md` (TODO Phase 2+)
- [ ] 1.16: Add section on structure caching behavior (TODO Phase 2+)
- [ ] 1.17: Document `seenStructures` tracking in CLAUDE.md (TODO Phase 2+)

### Code References

**template.go:121** - Template struct definition
**template.go:984** - First `clientHasStructure` check (new fields)
**template.go:1161** - Second `clientHasStructure` check (changed fields)
**template.go:859** - `stripStaticsRecursively` function

### Success Metrics
- ✅ Modal/conditional toggle bug fixed - ACHIEVED
- ✅ 30-80% reduction in update payload for toggling UIs - ACHIEVED (82% for modals, 65% for nested)
- ✅ No breaking changes to wire format - ACHIEVED
- ✅ All tests pass - ACHIEVED (core library + new tests)

---

## Phase 2: Add Server-Side ID Metadata ✅

**Priority**: HIGH
**Status**: ✅ COMPLETED (Server-Side)
**Duration**: 2 days
**Completed**: 2025-10-25

### Problem Statement

Client must search through HTML statics array to find where item IDs are located. This requires complex `findKeyPositionFromStatics` logic (~50 lines) that parses HTML strings looking for `id="`, `data-key="`, etc.

### Solution

Server detects ID position during template compilation and sends `_idKey` metadata telling client where IDs are located.

**Before**:
```typescript
// Client must search through: ["<li id=\"", "\">", "</li>"]
const keyPos = this.findKeyPositionFromStatics(statics); // Complex!
```

**After**:
```json
{
  "s": ["<li id=\"", "\">", "</li>"],
  "d": [/* items */],
  "_idKey": "1"  // Server tells client: IDs are at position "1"
}
```

```typescript
// Client just uses it directly
const id = item[this.idKey]; // Simple!
```

### Implementation Checklist

#### Server Changes
- [x] 2.1: Add ID detection in range compilation (tree_ast.go:470-478)
- [x] 2.2: Implement `detectIDKey(statics []string) string` (tree.go:399-436)
- [x] 2.3: Include `_idKey` in range tree node output (tree_ast.go:474-478)
- [x] 2.4: Test ID detection for all attribute types (tree_id_detection_test.go:13-52)
- [x] 2.5: Handle missing ID gracefully (defaults to "0")

#### Client Changes (🔜 TODO: Requires client-side implementation)
- [ ] 2.6: Add `rangeIdKeys: { [path: string]: string }` field
- [ ] 2.7: Store `_idKey` when processing range structures
- [ ] 2.8: Update `getItemKey` to use stored `_idKey`
- [ ] 2.9: Remove `findKeyPositionFromStatics` function (~50 lines)
- [ ] 2.10: Update all callsites
- [ ] 2.11: Add backward compatibility (default to "0")

#### Testing & Documentation
- [x] 2.12: Test ID detection with various HTML attributes (tree_id_detection_test.go)
- [x] 2.13: Test backward compatibility (detectIDKey defaults to "0")
- [ ] 2.14: Update TypeScript interfaces (TODO Phase 6)
- [ ] 2.15: Update protocol documentation (TODO Phase 6)

### Success Metrics
- ✅ Server sends `_idKey` metadata in all range nodes - ACHIEVED
- ✅ ID detection works for all attribute types (id, data-key, key, etc.) - ACHIEVED
- ✅ Backward compatible (defaults to "0") - ACHIEVED
- ✅ All tests pass with updated golden files - ACHIEVED
- 🔜 Client-side implementation pending (Phase 6)

---

## Phase 3: Unify Range Operations

**Priority**: MEDIUM
**Status**: 🔜 Not Started
**Duration**: 2 days

### Problem Statement

1. Insert operation has redundant parameters: `["i", afterId, position, data]`
2. Optimizes append but not prepend (inconsistent)
3. Protocol has ambiguity

### Solution

Full symmetry with 6 clear operations:

```typescript
['u', id, changes]   // Update
['r', id]            // Remove
['a', data]          // Append (O(1))
['p', data]          // Prepend (O(1))  ← NEW
['i', afterId, data] // Insert (simplified, no position param)
['o', [ids]]         // Reorder
```

### Implementation Checklist

#### Server Changes
- [ ] 3.1: Add prepend detection in `generateRangeDifferentialOperations`
- [ ] 3.2: Remove position parameter from insert operation
- [ ] 3.3: Update insert operation generation
- [ ] 3.4: Update all tests using insert operations

#### Client Changes
- [ ] 3.5: Add prepend operation handler (`case 'p'`)
- [ ] 3.6: Simplify insert operation handler (remove position logic)
- [ ] 3.7: Update append to handle batch items
- [ ] 3.8: Remove 10+ lines of position logic
- [ ] 3.9: Update all client tests

#### Documentation
- [ ] 3.10: Update operation specification
- [ ] 3.11: Document when each operation is used
- [ ] 3.12: Add performance notes (O(1) vs O(n))

### Success Metrics
- ✅ 6 clear, symmetric operations
- ✅ Removed ambiguity
- ✅ Optimized both append and prepend

---

## Phase 4: Type-Safe TreeNode Structure

**Priority**: MEDIUM
**Status**: 🔜 Not Started
**Duration**: 4-5 days

### Problem Statement

Using `map[string]interface{}` with magic strings throughout:
- `tree["s"]` - statics
- `tree["d"]` - dynamics
- `tree["f"]` - fingerprint
- `tree["_idKey"]` - ID metadata

Makes code hard to understand, maintain, and refactor.

### Solution

```go
type TreeNode struct {
    Statics     []string               `json:"s,omitempty"`
    Dynamics    map[string]interface{} `json:"-"`
    Fingerprint string                 `json:"f,omitempty"`
    Range       *RangeData             `json:"-"`
    Metadata    *TreeMetadata          `json:"-"`
}

type RangeData struct {
    Items   []interface{} `json:"d"`
    Statics []string      `json:"s,omitempty"`
}

type TreeMetadata struct {
    IDKey string `json:"idKey,omitempty"`
}

// Custom JSON marshaling maintains wire format compatibility
func (t TreeNode) MarshalJSON() ([]byte, error) { /* ... */ }
func (t *TreeNode) UnmarshalJSON(data []byte) error { /* ... */ }
```

### Implementation Checklist

#### Core Types
- [ ] 4.1: Define TreeNode struct with typed fields
- [ ] 4.2: Define RangeData struct
- [ ] 4.3: Define TreeMetadata struct
- [ ] 4.4: Implement custom MarshalJSON for wire compatibility
- [ ] 4.5: Implement custom UnmarshalJSON
- [ ] 4.6: Add helper methods (GetDynamic, SetDynamic, IsRange)

#### Migration
- [ ] 4.7: Create type alias `type treeNode = TreeNode` for compatibility
- [ ] 4.8: Update `calculateFingerprint` to use struct fields
- [ ] 4.9: Update `compareTreesAndGetChanges` to use typed access
- [ ] 4.10: Update all tree construction in `full_tree_parser.go`
- [ ] 4.11: Update template.go to use typed fields
- [ ] 4.12: Run all tests (should pass with compatibility layer)
- [ ] 4.13: Performance benchmark (struct vs map)
- [ ] 4.14: Gradually remove `treeNode` alias usage
- [ ] 4.15: Update CLAUDE.md documentation

### Success Metrics
- ✅ Full type safety on server
- ✅ No wire format changes
- ✅ Clearer, more maintainable code

---

## Phase 5: Optimize Fingerprinting

**Priority**: MEDIUM
**Status**: 🔜 Not Started
**Duration**: 3-4 days

### Problem Statement

`calculateFingerprint` marshals entire tree on every render:
```go
valueJSON, _ := json.Marshal(value)  // O(N) for entire subtree!
```

For 1000-node tree with 1 change, hashes all 1000 nodes.

### Solution

Hierarchical caching with dirty flagging:

```go
type TreeNode struct {
    // ... existing fields ...
    cachedFingerprint *string
    isDirty          bool
}

func (t *TreeNode) Fingerprint() string {
    if !t.isDirty && t.cachedFingerprint != nil {
        return *t.cachedFingerprint  // Cache hit!
    }
    fp := t.calculateFingerprintIncremental()
    t.cachedFingerprint = &fp
    t.isDirty = false
    return fp
}
```

### Implementation Checklist

#### Core Implementation
- [ ] 5.1: Add caching fields to TreeNode
- [ ] 5.2: Implement `MarkDirty()` method
- [ ] 5.3: Implement incremental fingerprint calculation
- [ ] 5.4: Update tree construction to mark new nodes dirty
- [ ] 5.5: Add cache invalidation on updates
- [ ] 5.6: Test cache invalidation correctness

#### Benchmarking
- [ ] 5.7: Create benchmark suite
  - [ ] 10 nodes
  - [ ] 100 nodes
  - [ ] 1,000 nodes
  - [ ] 10,000 nodes
- [ ] 5.8: Measure before/after performance
- [ ] 5.9: Profile memory usage
- [ ] 5.10: Add metrics for cache hit rate

### Success Metrics
- ✅ 50%+ faster for trees >100 nodes
- ✅ Cache hit rate >90%
- ✅ Memory overhead <5%

---

## Phase 6: Simplify Client Implementation

**Priority**: MEDIUM
**Status**: 🔜 Not Started
**Duration**: 3-4 days

### Problem Statement

Client is 2,252 lines with:
- Complex `rangeState` tracking
- Deep merge logic
- Path-based state tracking
- Complex key extraction (fixed in Phase 2)
- Operation parsing complexity

### Target

Reduce to ~700 lines by removing unnecessary complexity.

### Simplification Plan

```typescript
class LiveTemplateClient {
    private state: TreeNode = {};
    private rangeIdKeys: { [path: string]: string } = {};

    applyUpdate(update: TreeNode): UpdateResult {
        // Store metadata
        if (update._idKey) {
            this.rangeIdKeys[path] = update._idKey;
        }

        // Simple state merge
        this.state = { ...this.state, ...update };

        // Render
        return { html: this.render(this.state), changed: true };
    }

    private applyOperations(ops: any[], items: any[], path: string) {
        for (const op of ops) {
            switch(op[0]) {
                case 'u': this.updateItem(items, op[1], op[2], path); break;
                case 'r': this.removeItem(items, op[1], path); break;
                case 'a': items.push(...this.toArray(op[1])); break;
                case 'p': items.unshift(...this.toArray(op[1])); break;
                case 'i': this.insertItem(items, op[1], op[2], path); break;
                case 'o': return this.reorderItems(items, op[1], path);
            }
        }
        return items;
    }
}
```

### Implementation Checklist

- [ ] 6.1: Remove `findKeyPositionFromStatics` (done in Phase 2)
- [ ] 6.2: Simplify `applyDifferentialOperations` with new ops
- [ ] 6.3: Use `_idKey` metadata directly
- [ ] 6.4: Reduce `deepMergeTreeNodes` complexity
- [ ] 6.5: Clean up operation handlers
- [ ] 6.6: Remove redundant type checking
- [ ] 6.7: Consolidate state tracking
- [ ] 6.8: Add clear code comments
- [ ] 6.9: Extract helper functions
- [ ] 6.10: Update all client tests
- [ ] 6.11: Measure line count reduction

### Success Metrics
- ✅ 70% reduction (2,252 → ~700 lines)
- ✅ Clearer code structure
- ✅ All tests pass

---

## Overall Success Metrics

### Complexity Reduction
- **Server**: ~500 lines improved, full type safety
- **Client**: 70% reduction (2,252 → ~700 lines)
- **Protocol**: 6 clear operations (u, r, a, p, i, o)

### Performance
- **Fingerprinting**: 50%+ faster for large trees
- **Network**: Same efficiency (differential maintained)
- **Client CPU**: 40% less processing

### Quality
- **Bug Fixes**: Modal/dynamic structure bug eliminated
- **Type Safety**: Full typing on server
- **Maintainability**: Clearer, documented code

---

## Session Log

### Session 2025-10-25 (Initial)
- Created `ARCHITECTURE_IMPROVEMENTS.md`
- Created branch `feat/architecture-improvements`
- Verified all issues still present after merge
- Starting Phase 1 implementation

---

## Testing Strategy

### Unit Tests
- Each phase must maintain 80%+ coverage
- New functionality fully tested
- Edge cases covered

### Integration Tests
- End-to-end scenarios for each feature
- Backward compatibility verified
- No regression in existing functionality

### Performance Tests
- Benchmarks before/after each optimization
- Memory profiling
- Network payload analysis

---

## Migration Notes

### Backward Compatibility

All changes maintain wire format compatibility:
- Phase 1: No wire format changes
- Phase 2: Adds `_idKey` (optional, client defaults)
- Phase 3: Operation format changes (versioned)
- Phase 4: Wire format unchanged (custom JSON)
- Phase 5: Internal optimization only
- Phase 6: Client implementation only

### Rollout Strategy

1. Feature flags for each phase
2. Gradual rollout with monitoring
3. Rollback capability
4. Performance tracking

---

## References

- Original feedback from LLM review
- Codebase analysis session 2025-10-25
- LiveTemplate specification docs
- Phoenix LiveView optimization approach
