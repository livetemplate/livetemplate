# Critical Architectural Review: LiveTemplate Core Logic

**Date**: January 2026
**Reviewer**: Claude (Opus 4.5)
**Scope**: Template parsing, tree building, diff generation, and registry management

---

## Executive Summary

After analyzing the git history (15+ fix commits since v0.7.0) and the current codebase architecture, I've identified a **fundamental architectural gap**: the system lacks a unified state machine model for tree transformations. Instead of a deterministic algorithm, the code relies on **case-by-case condition detection and patching** to handle state transitions.

---

## Evidence from Git History

### Pattern of Fixes

| Commit | Issue | Root Cause Pattern |
|--------|-------|-------------------|
| `b223ed8` | Static-only conditionals stripped in updates | Missing case in `PrepareTreeForClient` |
| `3695f27` | Statics resent on load_more | `IsComplexInsertionPattern` didn't recognize append/prepend |
| `a87fc41` | range→else transition broken | Missing case in `handleTopLevelRange` |
| `db329a5` | Registry not invalidated when conditional becomes empty | Missing `InvalidatePath` call |
| `dc842a1` | Checkbox toggle not detected | Statics comparison missing in diff |
| `8b9fe28` | non-TreeNode→TreeNode transitions | Missing case in `handleNestedTreeNodeChange` |
| `7b675b7` | Range.Statics not populated for empty→items | Missing statics propagation path |
| `b0d3545` | Heterogeneous range items broken | Homogeneous assumption in range handling |
| `f1c42b8` | Range statics not marked in registry | Missing `MarkSeen` call for range path |

**Key Observation**: Each fix adds a new conditional branch to handle a specific state transition that was not covered. This is symptomatic of an **ad-hoc approach rather than a systematic algorithm**.

---

## Architectural Issues

### 1. Implicit State Machine with Explicit Transitions

The system implicitly deals with multiple state dimensions but handles them through explicit conditional checks scattered across the codebase:

**Tree Value States:**
- `nil` / empty string
- `string` (primitive)
- `*TreeNode` with statics, no dynamics
- `*TreeNode` with dynamics, no statics
- `*TreeNode` with both
- `*TreeNode` with Range (items present)
- `*TreeNode` with Range (empty, else clause)

**Registry States (per path):**
- Never seen
- Seen with signature X
- Invalidated

**Client Cache States:**
- Has statics cached
- Doesn't have statics cached
- Has range statics cached
- Has conditional branch statics cached

The **cross-product of these states** creates a large number of transitions, each requiring correct handling. Currently, these are handled via cascading `if` statements:

```go
// From tree_compare.go:55-97 - handleTopLevelRange
// Case 1: newTree is a range (with or without items)
if newTree.HasRange() && newTree.HasStatics() {
    if oldTree.HasRange() && oldTree.HasStatics() {
        // Both are ranges - use differential operations if matched
        if _, isMatched := rangeMatches[currentPath]; isMatched {
            return handleMatchedRanges(...)
        }
    } else {
        // else→range: newTree is a range but oldTree isn't
        *changes = *newTree
        return true
    }
}

// Case 2: range→else transition
if oldTree.HasRange() && oldTree.HasStatics() && !newTree.HasRange() {
    // Return full new tree so client can replace range items with else content
    if registryUsable {
        registry.InvalidatePath(rangeStaticsPath)
    }
    *changes = *newTree
    return true
}
```

**Each `if` statement represents a discovered edge case**, not a systematic enumeration.

---

### 2. Dual-Responsibility of Statics

Statics serve two conflicting purposes:

1. **Wire Format Optimization**: Strip statics when client has them cached
2. **Comparison Consistency**: Always need statics for accurate tree comparison

This leads to confusing patterns like `contextWithStatics()` in `range.go:28-48`:

```go
// contextWithStatics returns a context that always includes statics for internal use.
// Setting IsFirstRender=true here ensures ShouldIncludeStatics() returns true
// regardless of CurrentPath checks - it's not semantically a "first render" but
// rather a way to force statics inclusion for internal processing.
```

The comment itself reveals the architectural tension: we're **misusing a semantic flag** (`IsFirstRender`) to achieve a technical goal.

---

### 3. Registry Path Conventions Are Implicit

The system uses special path conventions like `__range_statics__` without formal definition:

```go
// From tree_compare.go:104
rangeStaticsPath := currentPath + ".__range_statics__"
```

These conventions are scattered across files:
- `tree_compare.go` creates them
- `template.go:1226` marks them
- `signature/registry.go` stores them

There's no central definition of valid path patterns or their semantics.

---

### 4. Signature System Has Semantic Gaps

The `CalculateSignature` function in `signature.go` creates different signatures for different structures:

```go
SigEmpty      = "empty"
SigScalar     = "scalar"
SigConditional = "conditional"
SigRangeEmpty = "range:empty"
// SigRangeItems = "range:items:<hash>"
```

But this categorization is too coarse:
- `SigConditional` doesn't distinguish between different conditional structures
- Two conditionals with different statics get the same signature
- This caused issues like the edit modal bug (db329a5)

---

### 5. PrepareTreeForClient Has Growing Complexity

The `prepare.go` file shows the accumulation of special cases:

```go
// From prepare.go:32-44
for k, val := range v.Dynamics {
    prepared := PrepareTreeForClient(val, clientHasStatics)
    if !IsEmpty(prepared) {
        result.Dynamics[k] = prepared
    } else if nestedNode, ok := val.(*TreeNode); ok && nestedNode.HasStatics() {
        // Special case for conditional blocks ({{if}}/{{else}}) with static-only content.
        // Even though clientHasStatics=true means we normally strip statics, conditional
        // branches are dynamically-rendered structures. When a new item is prepended,
        // the client hasn't seen THIS particular branch's statics yet...
        result.Dynamics[k] = val
    }
}
```

This is a **reactive fix** (b223ed8) added because the original algorithm didn't account for this case.

---

## Root Cause Analysis

### The Core Problem: No Formal Transition Model

The system evolved organically by adding handlers for observed bugs rather than being designed around a formal model of:

1. **What states can a tree node be in?**
2. **What transitions are valid between old→new states?**
3. **What should be sent to the client for each transition?**
4. **What registry operations are needed for each transition?**

### Evidence of Missing Model

Consider the range handling across files:

| Location | Handles | Missing When Added |
|----------|---------|-------------------|
| `tree_compare.go:68-79` | range→range, else→range | ✓ Original |
| `tree_compare.go:85-95` | range→else | Added in a87fc41 |
| `range_ops.go:81-103` | empty→items statics | Added in 7b675b7 |
| `range_ops.go:385-432` | TreeNode→TreeNode statics change | Added in dc842a1 |

A proper transition model would enumerate all 4 transitions upfront rather than discovering them via bugs.

---

## Recommendations

### Option 1: Full Rewrite with State Machine

Define an explicit state machine:

```go
type TreeValueKind int
const (
    KindNil TreeValueKind = iota
    KindPrimitive
    KindStaticOnly
    KindDynamicOnly
    KindMixed
    KindRangeWithItems
    KindRangeEmpty
)

type Transition struct {
    Old, New TreeValueKind
}

type TransitionHandler struct {
    ShouldSendStatics     bool
    ShouldSendDynamics    bool
    ShouldInvalidateCache bool
    ShouldMarkSeen        bool
}

var transitionTable = map[Transition]TransitionHandler{
    {KindNil, KindStaticOnly}: {ShouldSendStatics: true, ShouldMarkSeen: true},
    {KindPrimitive, KindMixed}: {ShouldSendStatics: true, ShouldSendDynamics: true, ShouldMarkSeen: true},
    {KindRangeWithItems, KindRangeEmpty}: {ShouldSendDynamics: true, ShouldInvalidateCache: true},
    // ... enumerate all valid transitions
}
```

**Pros**: Deterministic, complete, testable
**Cons**: Significant rewrite, potential regression risk

### Option 2: Incremental Hardening

1. **Add Exhaustive Tests**: Create a test matrix covering all state transitions
2. **Add Assertions**: Detect unhandled transitions with explicit panics in dev mode
3. **Centralize Path Conventions**: Create a `paths` package with formal definitions
4. **Improve Signature Granularity**: Include statics hash in all signatures

**Pros**: Lower risk, incremental progress
**Cons**: Doesn't solve the fundamental architecture issue

### Option 3: Hybrid Approach (Recommended)

1. **Create Formal Transition Model** (document, not code initially)
2. **Validate Current Code Against Model** (find gaps systematically)
3. **Refactor Incrementally** with the model as guide
4. **Add Property-Based Tests** to verify model completeness

---

## Detailed State Transition Matrix

For reference, here's a partial enumeration of the transitions that should be handled:

### Basic Value Transitions

| Old State | New State | Statics | Dynamics | Registry |
|-----------|-----------|---------|----------|----------|
| nil | primitive | - | send | mark |
| nil | static-only | send | - | mark |
| nil | mixed | send | send | mark |
| primitive | nil | - | send "" | - |
| primitive | static-only | send | - | mark |
| primitive | mixed | send | send | mark |
| static-only | nil | - | send "" | invalidate |
| static-only | primitive | - | send | invalidate |
| static-only | static-only (same) | - | - | - |
| static-only | static-only (diff) | send | - | mark |
| mixed | nil | - | - | invalidate |
| mixed | primitive | - | send | invalidate |
| mixed | static-only | * | - | * |
| mixed | mixed (same statics) | - | send diff | - |
| mixed | mixed (diff statics) | send | send | mark |

### Range-Specific Transitions

| Old State | New State | Operation |
|-----------|-----------|-----------|
| range-empty | range-items | send 'a' with statics + metadata |
| range-items | range-empty | send else content, invalidate |
| range-items | range-items (same) | send differential ops |
| range-items | range-items (diff statics) | full replace |
| else-content | range-items | full replace with new tree |
| range-items | else-content | full replace, invalidate |

---

## Files Requiring Attention

In order of impact:

1. **`internal/diff/tree_compare.go`**: Main comparison orchestrator (547 lines)
2. **`internal/diff/range_ops.go`**: Range differential operations (489 lines)
3. **`internal/diff/helpers.go`**: Utility functions with implicit assumptions (780 lines)
4. **`internal/diff/prepare.go`**: Wire format preparation (87 lines)
5. **`internal/signature/signature.go`**: Signature calculation (148 lines)
6. **`internal/signature/registry.go`**: Client state tracking (227 lines)
7. **`internal/parse/range.go`**: Range tree building (438 lines)

---

## Conclusion

The core logic **does have an architectural gap**. The evidence is clear:
- 15+ fixes in ~6 months for edge cases
- Each fix adds conditional branches rather than completing a model
- Comments acknowledging workarounds ("it's not semantically a first render but...")
- Special-case handling scattered across files

The code works, but it's **fragile by construction**. Each new Go template feature or DOM pattern may expose another unhandled transition. A more systematic approach—whether full rewrite or incremental hardening—would reduce ongoing maintenance burden and increase confidence in correctness.

---

## Next Steps

1. **Immediate**: Create comprehensive test matrix for all transitions
2. **Short-term**: Add dev-mode assertions for unhandled transition detection
3. **Medium-term**: Document formal transition model
4. **Long-term**: Consider gradual refactor toward state machine architecture
