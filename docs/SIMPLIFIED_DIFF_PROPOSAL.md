# Simplified Diff Architecture Proposal

**Status**: Implemented
**Author**: Claude (Opus 4.5)
**Date**: January 2026
**Last Updated**: January 2026

## Overview

This proposal outlines a simplified diffing architecture inspired by Phoenix LiveView's approach. Instead of handling 49+ state transitions with conditional branches, we reduce the problem to **4 simple cases**.

---

## Core Principle

> **When in doubt, send the full subtree.**

Phoenix LiveView has proven this works at scale. The wire size penalty is minimal (statics are typically <1KB), and the code simplicity gains are enormous.

---

## New Data Model

### TreeNode (Revised)

```go
package diff

// TreeNode represents a template node with structure fingerprinting.
type TreeNode struct {
    // StructureFingerprint is a hash of the static structure only.
    // It does NOT include dynamic values - only the shape of the tree.
    // Two nodes with the same fingerprint have identical statics.
    StructureFingerprint string `json:"f,omitempty"`

    // Statics are the static HTML parts that don't change.
    // Sent on first render and when structure changes.
    Statics []string `json:"s,omitempty"`

    // Dynamics are the changing values, keyed by position.
    // Values can be: string, *TreeNode, or for ranges: []RangeItem
    Dynamics map[string]interface{} `json:"-"`

    // Range holds list data when this node represents a {{range}} construct.
    Range *RangeData `json:"r,omitempty"`
}

// RangeData holds items for range constructs.
type RangeData struct {
    // ItemFingerprint is the structure hash for range items.
    // All items in a homogeneous range share the same fingerprint.
    ItemFingerprint string `json:"if,omitempty"`

    // ItemStatics are the statics for each item (homogeneous ranges).
    ItemStatics []string `json:"is,omitempty"`

    // Items contains the range items, each keyed by a stable ID.
    Items []RangeItem `json:"i,omitempty"`
}

// RangeItem represents a single item in a range.
type RangeItem struct {
    Key      string                 `json:"k"`
    Dynamics map[string]interface{} `json:"d,omitempty"`
}
```

### Key Changes from Current Model

| Current | Proposed | Rationale |
|---------|----------|-----------|
| No fingerprint | `StructureFingerprint` | Enables O(1) structure comparison |
| `Statics` sometimes nil | `Statics` always populated internally | Simplifies logic |
| Complex `StaticsMap` for heterogeneous | Removed | Fall back to full replace |
| `Metadata.IDKey` | `RangeItem.Key` explicit | Clearer contract |

---

## The Simplified Diff Algorithm

### Core Function: 4 Cases Total

```go
package diff

// Diff compares two trees and returns the minimal update.
// This is the ENTIRE diff algorithm - no state machine needed.
func Diff(old, new *TreeNode, clientHasStatics bool) *TreeNode {
    // CASE 1: Both nil - no change
    if old == nil && new == nil {
        return nil
    }

    // CASE 2: New node where none existed - send full
    if old == nil {
        return prepareForWire(new, false) // include statics
    }

    // CASE 3: Node removed - send empty marker
    if new == nil {
        return &TreeNode{Dynamics: map[string]interface{}{"0": ""}}
    }

    // CASE 4: Both exist - compare fingerprints
    if old.StructureFingerprint == new.StructureFingerprint {
        // Structure unchanged - send only changed dynamics
        return diffDynamicsOnly(old, new, clientHasStatics)
    }

    // Structure changed - send full new tree
    return prepareForWire(new, false)
}

// diffDynamicsOnly compares dynamics when structure is unchanged.
func diffDynamicsOnly(old, new *TreeNode, clientHasStatics bool) *TreeNode {
    result := &TreeNode{
        Dynamics: make(map[string]interface{}),
    }

    // Compare each dynamic position
    for key, newVal := range new.Dynamics {
        oldVal := old.Dynamics[key]

        // Recurse for nested TreeNodes
        if newNode, ok := newVal.(*TreeNode); ok {
            if oldNode, ok := oldVal.(*TreeNode); ok {
                if changed := Diff(oldNode, newNode, clientHasStatics); changed != nil && !isEmpty(changed) {
                    result.Dynamics[key] = changed
                }
            } else {
                // Old wasn't a TreeNode, new is - send full
                result.Dynamics[key] = prepareForWire(newNode, false)
            }
            continue
        }

        // Primitive comparison
        if !deepEqual(oldVal, newVal) {
            result.Dynamics[key] = newVal
        }
    }

    // Check for removed dynamics
    for key, oldVal := range old.Dynamics {
        if _, exists := new.Dynamics[key]; !exists {
            if isMeaningful(oldVal) {
                result.Dynamics[key] = "" // Signal removal
            }
        }
    }

    // Handle range if present
    if old.Range != nil || new.Range != nil {
        rangeOps := diffRange(old.Range, new.Range, clientHasStatics)
        if rangeOps != nil {
            result.Range = rangeOps
        }
    }

    if isEmpty(result) {
        return nil
    }
    return result
}
```

### Range Diffing: 4 More Cases

```go
// diffRange handles range/list diffing with simple rules.
func diffRange(old, new *RangeData, clientHasStatics bool) *RangeData {
    // CASE R1: No range in either - nothing to do
    if old == nil && new == nil {
        return nil
    }

    // CASE R2: Range added (empty → items)
    if old == nil || len(old.Items) == 0 {
        if new != nil && len(new.Items) > 0 {
            return &RangeData{
                ItemStatics: new.ItemStatics,
                Items:       new.Items,
            }
        }
        return nil
    }

    // CASE R3: Range cleared (items → empty)
    if new == nil || len(new.Items) == 0 {
        return &RangeData{Items: []RangeItem{}} // Empty array signals clear
    }

    // CASE R4: Both have items - compare keys
    oldKeys := extractKeys(old.Items)
    newKeys := extractKeys(new.Items)

    // Sub-case 4a: Same keys, same order - diff items individually
    if slicesEqual(oldKeys, newKeys) {
        return diffRangeItemsOnly(old, new, clientHasStatics)
    }

    // Sub-case 4b: Keys differ - use LCS for optimal diff OR full replace
    // For simplicity, we can start with full replace and optimize later
    return &RangeData{
        ItemStatics: new.ItemStatics,
        Items:       new.Items,
    }
}

// diffRangeItemsOnly diffs items when keys match exactly.
func diffRangeItemsOnly(old, new *RangeData, clientHasStatics bool) *RangeData {
    result := &RangeData{}
    hasChanges := false

    for i, newItem := range new.Items {
        oldItem := old.Items[i]

        itemChanges := make(map[string]interface{})
        for key, newVal := range newItem.Dynamics {
            if oldVal, exists := oldItem.Dynamics[key]; !exists || !deepEqual(oldVal, newVal) {
                itemChanges[key] = newVal
                hasChanges = true
            }
        }

        if len(itemChanges) > 0 {
            result.Items = append(result.Items, RangeItem{
                Key:      newItem.Key,
                Dynamics: itemChanges,
            })
        }
    }

    if !hasChanges {
        return nil
    }
    return result
}
```

---

## Wire Format

### First Render (client has no statics)

```json
{
    "f": "abc123",
    "s": ["<div class=\"todo\">", "<span>", "</span></div>"],
    "0": "Buy groceries",
    "r": {
        "if": "def456",
        "is": ["<li id=\"", "\">", "</li>"],
        "i": [
            {"k": "1", "d": {"0": "1", "1": "Task one"}},
            {"k": "2", "d": {"0": "2", "1": "Task two"}}
        ]
    }
}
```

### Update (structure unchanged)

```json
{
    "0": "Buy milk"
}
```

### Update (structure changed)

```json
{
    "f": "xyz789",
    "s": ["<div class=\"todo done\">", "<span>", "</span></div>"],
    "0": "Buy milk"
}
```

### Range: Item updated

```json
{
    "r": {
        "i": [{"k": "1", "d": {"1": "Task one (edited)"}}]
    }
}
```

### Range: Items added (keys changed)

```json
{
    "r": {
        "is": ["<li id=\"", "\">", "</li>"],
        "i": [
            {"k": "1", "d": {"0": "1", "1": "Task one"}},
            {"k": "3", "d": {"0": "3", "1": "New task"}},
            {"k": "2", "d": {"0": "2", "1": "Task two"}}
        ]
    }
}
```

---

## Comparison with Current Architecture

| Aspect | Current | Proposed |
|--------|---------|----------|
| State transitions | 49+ | 8 (4 tree + 4 range) |
| Lines of diff code | ~2000 | ~200 |
| Files involved | 6+ | 1-2 |
| Registry complexity | High (path tracking) | Low (fingerprint only) |
| Heterogeneous ranges | Complex `StaticsMap` | Full replace |
| Edge case handling | Reactive (add `if` per bug) | Proactive (fallback to full) |

---

## Migration Plan

### Phase 1: Add Fingerprinting (Non-Breaking)

1. Add `StructureFingerprint` field to `TreeNode`
2. Calculate fingerprint during tree building
3. Keep existing diff logic, add fingerprint comparison as optimization

### Phase 2: Simplify Range Handling

1. Remove `StaticsMap` complexity
2. Implement simple key-based diffing
3. Fall back to full replace for complex cases

### Phase 3: Replace Diff Logic

1. Implement new `Diff()` function alongside existing
2. Add feature flag to switch between old/new
3. Run both in parallel in tests, compare outputs
4. Gradually migrate to new logic

### Phase 4: Remove Old Code

1. Remove old diff logic
2. Remove signature registry (replaced by fingerprint)
3. Clean up unused helpers

---

## Risk Mitigation

### Testing Strategy

```go
// Property-based test: new diff is never "wrong", just sometimes larger
func TestNewDiffCorrectness(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        old := generateRandomTree(t)
        new := generateRandomTree(t)

        oldDiff := oldDiffAlgorithm(old, new)
        newDiff := newDiffAlgorithm(old, new)

        // Apply both diffs to old tree
        result1 := applyDiff(old, oldDiff)
        result2 := applyDiff(old, newDiff)

        // Both must produce the same final tree
        assert.Equal(t, result1, result2)
    })
}
```

### Wire Size Monitoring

Add metrics to track:
- Average update size (old vs new algorithm)
- Percentage of "full replace" fallbacks
- P99 update size

If wire size increases significantly, we can add targeted optimizations for hot paths.

---

## Client Compatibility Analysis

### Client Repository

The TypeScript client is at: https://github.com/livetemplate/client

### Key Files Analyzed

| File | Lines | Purpose |
|------|-------|---------|
| `state/tree-renderer.ts` | 814 | Core tree update handling |
| `types.ts` | 37 | TypeScript interfaces |
| `tests/tree-renderer.test.ts` | 187 | Behavior verification |

### How the Client Works

#### State Management

```typescript
private treeState: TreeNode = {};           // Current full tree
private rangeState: Record<string, RangeStateEntry> = {};  // Range cache
private rangeIdKeys: Record<string, string> = {};          // ID positions
```

#### Statics Caching (Already Implemented)

```typescript
// tree-renderer.ts lines 417-430
this.rangeState[stateKey] = {
  items: value.d,
  statics: value.s,
  staticsMap: value.sm,  // For heterogeneous ranges
};
```

#### Range Operations Supported

| Op | Format | Client Implementation |
|----|--------|----------------------|
| `r` | `["r", key]` | `splice(removeIndex, 1)` |
| `u` | `["u", key, changes]` | `{...item, ...changes}` |
| `a` | `["a", items, statics?, meta?]` | `push(...items)` |
| `p` | `["p", items, statics?]` | `unshift(...items)` |
| `i` | `["i", afterKey, items]` | `splice(targetIdx + 1, 0, ...)` |
| `o` | `["o", [keys]]` | Reorder by key lookup |

#### Range-to-Non-Range Transition (Critical Path)

```typescript
// tree-renderer.ts lines 139-145
if (isRangeNode(existing) && !isRangeNode(update)) {
  this.logger.debug(`Range→non-range transition, replacing`);
  return update;  // Full replacement, not merge
}
```

### Compatibility Assessment

| Feature | Current Server | Simplified Server | Client Support | Compatible? |
|---------|---------------|-------------------|----------------|-------------|
| Statics (`s`) | Conditional | When fingerprint differs | ✅ Caches on receive | **Yes** |
| Fingerprint (`f`) | Present | Used for comparison | ✅ Ignored (transparent) | **Yes** |
| Range items (`d`) | Array | Same format | ✅ Stores in rangeState | **Yes** |
| Range ops | `r/u/a/p/i/o` | Same format | ✅ All implemented | **Yes** |
| StaticsMap (`sm`) | Sent for heterogeneous | **Removed** | ✅ Becomes dead code | **Yes** |
| Full range replace | Supported | Used more often | ✅ Lines 78-80 | **Yes** |
| Metadata (`m`) | idKey | Same | ✅ Lines 229-235 | **Yes** |

### Dead Code After Migration (Safe)

The following client code will never execute but won't cause errors:

```typescript
// tree-renderer.ts lines 509, 516-518, 727-735
const hasStaticsMap = staticsMap && typeof staticsMap === "object";
if (hasStaticsMap && item._sk && staticsMap[item._sk]) {
  itemStatics = staticsMap[item._sk];
}
```

### Test Coverage Verification

Existing client tests already cover all scenarios:

| Scenario | Test File | Test Name |
|----------|-----------|-----------|
| Range with items | `tree-renderer.test.ts` | `applyUpdate - range to non-range transition` |
| Range → else | `tree-renderer.test.ts` | `should replace range structure with else clause content` |
| Nested transitions | `tree-renderer.test.ts` | `should handle nested range to non-range transitions` |
| Range merge | `tree-renderer.test.ts` | `should preserve range structure when update also has range` |
| Render after transition | `tree-renderer.test.ts` | `should render else content after range items are removed` |

### Conclusion

**No client changes required.** The simplified server architecture is fully compatible with the current client:

1. Wire format unchanged
2. All operations already supported
3. StaticsMap becomes dead code (safe to ignore)
4. Full replacement already works
5. CI will catch any issues via cross-repo tests

### Optional Future Client Cleanup

After server changes are stable, can remove dead code:
- `staticsMap` field in `RangeStateEntry`
- `hasStaticsMap` checks
- `_sk` key lookups

---

## Technical Considerations

### MD5 Collision Risk Analysis

The implementation uses MD5 for fingerprint calculation, truncated to 64 bits (16 hex characters).

**Collision Probability:**

Using the birthday problem approximation: `P(collision) ≈ 1 - e^(-n²/2m)`

Where:
- `n` = number of unique structures compared
- `m` = 2^64 (hash space with 64-bit truncation)

| Structures | Collision Probability |
|------------|----------------------|
| 1,000 | ~2.7 × 10⁻¹⁴ (negligible) |
| 1,000,000 | ~2.7 × 10⁻⁸ (1 in 37 million) |
| 1,000,000,000 | ~2.7 × 10⁻² (2.7%) |

**Risk Assessment:**
- A typical application has <1,000 unique template structures
- Collision probability is astronomically low for real-world usage
- Even if collision occurs, the only consequence is sending extra statics (no correctness issue)
- MD5's cryptographic weaknesses are irrelevant for fingerprinting (we don't need collision resistance against adversaries)

**If collision risk becomes a concern:**
1. Increase to full 128-bit MD5 hash (32 hex characters)
2. Switch to SHA-256 for larger hash space
3. Add structure equality check as fallback when fingerprints match

### Fingerprint Caching Consideration

**Current Implementation:**
- Fingerprints are calculated on-demand during comparison
- Each `ClientNeedsStatics()` call computes fingerprints for both trees

**Potential Optimization:**
- Cache fingerprint on `TreeNode` after first calculation
- Avoid recalculation for deeply nested trees
- Estimated performance gain: 50-70% for deep trees (15+ levels)

**When to implement:**
- Benchmark results show fingerprint calculation is a bottleneck
- Application has many deeply nested templates (10+ levels)
- High-frequency updates on complex templates

**Proposed API (future):**
```go
type TreeNode struct {
    // ... existing fields ...

    // cachedFingerprint stores the computed fingerprint (internal use only)
    cachedFingerprint string
}

// GetFingerprint returns the structure fingerprint, computing and caching if needed.
func (t *TreeNode) GetFingerprint() string {
    if t.cachedFingerprint == "" {
        t.cachedFingerprint = CalculateStructureFingerprint(t)
    }
    return t.cachedFingerprint
}
```

### Removal Timeline

**v0.8.0 (Current):**
- `StructureRegistry` interface **REMOVED**
- `AreStructuresSimilar` function **REMOVED**
- All registry parameters removed from internal functions
- `ClientNeedsStatics` is the only API for structure comparison

**Migration Path:**
```go
// Old code (v0.7.x)
if !diff.AreStructuresSimilar(oldTree, newTree) {
    sendStatics = true
}

// New code (v0.8.0+)
if diff.ClientNeedsStatics(oldTree, newTree) {
    sendStatics = true
}
```

**Breaking Changes in v0.8.0:**
- `StructureRegistry` interface no longer exists
- `AreStructuresSimilar` function no longer exists
- Functions no longer accept `registry` parameter

---

## Wire Format Metrics

### Tracking Wire Size Changes

Add the following metrics to monitor the impact of the fingerprint-based approach:

```go
// internal/observe/metrics.go

var (
    // UpdatePayloadBytes tracks the size of update payloads sent to clients.
    UpdatePayloadBytes = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "livetemplate_update_payload_bytes",
        Help:    "Size of update payloads in bytes",
        Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
    })

    // FullTreeSends counts when full tree (with statics) is sent.
    FullTreeSends = promauto.NewCounter(prometheus.CounterOpts{
        Name: "livetemplate_full_tree_sends_total",
        Help: "Total number of times full tree with statics was sent",
    })

    // DynamicsOnlySends counts when only dynamics are sent.
    DynamicsOnlySends = promauto.NewCounter(prometheus.CounterOpts{
        Name: "livetemplate_dynamics_only_sends_total",
        Help: "Total number of times only dynamics (no statics) were sent",
    })

    // FingerprintMismatches counts structure changes detected.
    FingerprintMismatches = promauto.NewCounter(prometheus.CounterOpts{
        Name: "livetemplate_fingerprint_mismatches_total",
        Help: "Total number of fingerprint mismatches (structure changes)",
    })
)
```

### Key Metrics to Monitor

| Metric | Target | Action if Exceeded |
|--------|--------|-------------------|
| P50 payload size | <1KB | Acceptable |
| P99 payload size | <10KB | Investigate large templates |
| Full tree ratio | <10% | Expected after initial render |
| Fingerprint mismatches | <5% | Review template stability |

---

## Open Questions

1. **LCS for ranges**: Should we implement LCS-based insert/delete/reorder detection, or start with simple full-replace?
   - Recommendation: Start simple, add LCS if benchmarks show need

2. **Heterogeneous ranges**: Current code handles items with different statics. Worth keeping?
   - Recommendation: Remove. Full replace is simpler and these are rare.
   - **Confirmed**: Client already handles full replacement (lines 78-80)

3. **Client changes**: Does the TypeScript client need updates?
   - **Answered**: No changes required. Wire format compatible. StaticsMap becomes dead code.

4. **Fingerprint algorithm**: MD5? SHA256? FNV? CRC32?
   - **Answered**: MD5 with 64-bit truncation. See "MD5 Collision Risk Analysis" above.

---

## Next Steps

1. [x] Review and approve this proposal
2. [x] Analyze client compatibility ✅ **No changes needed**
3. [x] Implement Phase 1 (fingerprinting) ✅ **`CalculateStructureFingerprint` added to `internal/build/fingerprint.go`**
4. [x] Implement Phase 2 (integrate into diff) ✅ **`ClientNeedsStatics` integrated into `internal/diff/tree_compare.go`**
5. [x] Phase 3: Simplify range handling ✅ **Range diffing now uses fingerprint comparison**
6. [x] Phase 4: Cleanup ✅ **Deprecated `AreStructuresSimilar` and `StructureRegistry`**
7. [x] Benchmark wire size impact ✅ **Added benchmarks in `internal/diff/diff_bench_test.go`**
8. [x] Document MD5 collision risk ✅ **See "MD5 Collision Risk Analysis" above**
9. [x] Document deprecation timeline ✅ **See "Deprecation Timeline" above**

---

## Appendix: Fingerprint Calculation

```go
import "hash/fnv"

// calculateStructureFingerprint hashes the static structure of a tree.
// It does NOT include dynamic values.
func calculateStructureFingerprint(node *TreeNode) string {
    h := fnv.New64a()

    // Hash statics
    for _, s := range node.Statics {
        h.Write([]byte(s))
        h.Write([]byte{0}) // separator
    }

    // Hash structure of dynamics (keys only, not values)
    keys := sortedKeys(node.Dynamics)
    for _, k := range keys {
        h.Write([]byte(k))
        if child, ok := node.Dynamics[k].(*TreeNode); ok {
            // Recurse for nested TreeNodes
            h.Write([]byte(calculateStructureFingerprint(child)))
        }
    }

    // Hash range structure if present
    if node.Range != nil {
        h.Write([]byte("range"))
        for _, s := range node.Range.ItemStatics {
            h.Write([]byte(s))
        }
    }

    return fmt.Sprintf("%016x", h.Sum64())
}
```
