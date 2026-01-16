# Simplified Diff Architecture Proposal

**Status**: Draft
**Author**: Claude (Opus 4.5)
**Date**: January 2026

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

## Open Questions

1. **LCS for ranges**: Should we implement LCS-based insert/delete/reorder detection, or start with simple full-replace?
   - Recommendation: Start simple, add LCS if benchmarks show need

2. **Heterogeneous ranges**: Current code handles items with different statics. Worth keeping?
   - Recommendation: Remove. Full replace is simpler and these are rare.

3. **Client changes**: Does the TypeScript client need updates?
   - Likely minimal changes if wire format stays compatible

4. **Fingerprint algorithm**: MD5? SHA256? FNV? CRC32?
   - Recommendation: FNV-1a (fast, good distribution, used by Go maps)

---

## Next Steps

1. [ ] Review and approve this proposal
2. [ ] Create detailed test matrix for migration
3. [ ] Implement Phase 1 (fingerprinting)
4. [ ] Benchmark wire size impact
5. [ ] Proceed with remaining phases

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
