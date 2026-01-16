# Architectural Review: LiveTemplate Core Logic

**Date**: January 2026
**Reviewer**: Claude (Opus 4.5)
**Scope**: Template parsing, tree building, diff generation, and registry management

---

## Executive Summary

After analyzing the codebase architecture, I've identified opportunities for improvement in the tree transformation logic. The system handles state transitions through conditional checks that could benefit from a more systematic approach. This document proposes the fingerprint-based architecture that has now been implemented.

> **Note**: This review led to the implementation of the fingerprint-based diff architecture. See the "Implementation Status" section at the end for completed changes.

---

## Observed Patterns

### Pattern of Incremental Additions

The codebase shows a pattern where edge cases are handled by adding conditional branches. The following table illustrates the *types* of issues that arise when state transitions aren't systematically enumerated:

| Issue Type | Example | Root Cause Pattern |
|------------|---------|-------------------|
| Static-only conditionals | Content stripped in updates | Missing case in `PrepareTreeForClient` |
| Load more operations | Statics resent unnecessarily | Pattern detection incomplete |
| Range transitions | range→else handling | Missing transition case |
| Registry invalidation | Stale cache entries | Missing `InvalidatePath` call |
| Checkbox toggles | State change not detected | Statics comparison needed |
| Type transitions | non-TreeNode→TreeNode | Missing type change handler |
| Empty→items | Statics not populated | Missing propagation path |
| Heterogeneous ranges | Different item structures | Homogeneous assumption |

> **Note**: These represent categories of issues, not specific verified commits. The pattern is illustrative of how ad-hoc handling can lead to gaps.

**Key Observation**: Each new case adds a conditional branch. A systematic enumeration of transitions could prevent gaps.

---

## Architectural Observations

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

The **cross-product of these states** creates many transitions. Currently handled via cascading `if` statements:

```go
// From tree_compare.go - handleTopLevelRange
if newTree.HasRange() && newTree.HasStatics() {
    if oldTree.HasRange() && oldTree.HasStatics() {
        if _, isMatched := rangeMatches[currentPath]; isMatched {
            return handleMatchedRanges(...)
        }
    } else {
        *changes = *newTree
        return true
    }
}
```

---

### 2. Dual-Responsibility of Statics (Intentional Design)

Statics serve two purposes, which is an **intentional architectural decision** documented in CLAUDE.md:

1. **Wire Format Optimization**: Strip statics when client has them cached (~90% payload reduction)
2. **Comparison Consistency**: Always generate trees WITH statics for accurate comparison

Per CLAUDE.md (lines 147-165):
> **Key Architectural Points**:
> - Trees are ALWAYS generated WITH statics (ensures consistent comparison)
> - `prepareTreeForClient` removes statics before wire transmission
> - **This is NOT a "reactive fix" - it's the correct implementation of specification**

The `contextWithStatics()` pattern is a deliberate mechanism to ensure statics are available for internal processing while being stripped for wire transmission.

**Potential improvement**: The flag name `IsFirstRender` could be renamed to `ShouldIncludeStatics` for clarity, though the current implementation is functionally correct.

---

### 3. Registry Path Conventions

The system uses special path conventions like `__range_statics__`:

```go
rangeStaticsPath := currentPath + ".__range_statics__"
```

**Observation**: These conventions are scattered across files.

**Proposed improvement**: Centralize in a `paths` package:
```go
// internal/paths/conventions.go
const RangeStaticsSuffix = ".__range_statics__"

func RangeStaticsPath(basePath string) string {
    return basePath + RangeStaticsSuffix
}
```

---

### 4. Signature Granularity

The `CalculateSignature` function uses categories that may be too coarse:

```go
SigConditional = "conditional"  // Doesn't distinguish different structures
```

**Proposed improvement**: Include statics hash for finer granularity:
```go
func CalculateSignature(node *TreeNode) string {
    if node.HasConditional() {
        staticsHash := md5.Sum(node.Statics)
        return fmt.Sprintf("conditional:%x", staticsHash[:8])
    }
    // ...
}
```

---

## Trade-offs and Design Philosophy

The current architecture optimizes for:

1. **Wire efficiency**: ~90% payload reduction through statics caching
2. **Incremental updates**: Only changed dynamics sent after first render
3. **Client simplicity**: Client doesn't need complex reconciliation logic

These optimizations come with trade-offs:
- Server must track what client has seen (registry complexity)
- Edge cases in state transitions can cause bugs
- Testing requires covering many transition combinations

The fingerprint-based approach (now implemented) addresses the state transition complexity while preserving the wire efficiency benefits.

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

### Option 3: Hybrid Approach

1. **Create Formal Transition Model** (document, not code initially)
2. **Validate Current Code Against Model** (find gaps systematically)
3. **Refactor Incrementally** with the model as guide
4. **Add Property-Based Tests** to verify model completeness

### Option 4: Fingerprint-Based Approach ✅ (Implemented)

Inspired by Phoenix LiveView, this approach sidesteps state transition complexity:

1. **Calculate structure fingerprint** - Hash only the static structure (not dynamic values)
2. **Compare fingerprints** - If same, client has statics; if different, send full tree
3. **"When in doubt, send full tree"** - Simplifies edge case handling

```go
// Simple 4-case logic replaces 49+ state transitions
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
    if oldTree == nil { return true }   // First render
    if newTree == nil { return false }  // Removal
    return CalculateStructureFingerprint(oldTree) != CalculateStructureFingerprint(newTree)
}
```

**Pros**: Simple, deterministic, O(1) comparison, no registry tracking needed
**Cons**: May send slightly larger payloads in some edge cases (benchmarks show negligible impact)

**This option was implemented** - see Implementation Status section below.

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
5. **`internal/parse/range.go`**: Range tree building (438 lines)

---

## Conclusion

The core logic has **room for architectural improvement**. Key observations:
- Edge cases are handled by adding conditional branches incrementally
- State transitions aren't systematically enumerated
- Some patterns (like registry path conventions) could be centralized

The code is functional and well-tested, but could benefit from a more systematic approach to state transitions. The fingerprint-based approach (now implemented) addresses this by:
- Reducing state transition complexity from many cases to 4 simple cases
- Using O(1) fingerprint comparison instead of path-based registry tracking
- Adopting the "when in doubt, send full tree" philosophy from Phoenix LiveView

This maintains the wire efficiency benefits while simplifying the codebase.

---

## Implementation Status (Updated January 2026)

The fingerprint-based approach has been implemented to address the architectural concerns raised above. Instead of tracking state transitions explicitly, the system now uses structure fingerprints for O(1) comparison.

### Completed Changes

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | ✅ | `CalculateStructureFingerprint` in `internal/build/fingerprint.go` |
| Phase 2 | ✅ | `ClientNeedsStatics` integrated into `internal/diff/tree_compare.go` |
| Phase 3 | ✅ | Range handling simplified with fingerprint comparison |
| Phase 4 | ✅ | Deprecated `AreStructuresSimilar` and `StructureRegistry` |

### Key API Changes

**New Functions:**
- `build.CalculateStructureFingerprint(tree *TreeNode) string` - Hash of static structure only
- `diff.ClientNeedsStatics(oldTree, newTree *TreeNode) bool` - Fingerprint-based comparison

**Deprecated:**
- `diff.AreStructuresSimilar` - Use `!ClientNeedsStatics()` instead
- `diff.StructureRegistry` interface - No longer needed for statics decisions

### How It Works

```go
// Old approach: Registry-based path tracking (49+ state transitions)
rangeStaticsPath := currentPath + ".__range_statics__"
clientHasStatics := registry.HasSeen(rangeStaticsPath, tree.Statics)

// New approach: Fingerprint comparison (4 cases)
clientHasStatics := !ClientNeedsStatics(oldTree, newTree)
```

The fingerprint approach reduces complexity by:
1. Hashing only the **static structure** (statics arrays, dynamic positions, nested structure)
2. Comparing fingerprints instead of tracking paths
3. Using "when in doubt, send full tree" philosophy (inspired by Phoenix LiveView)

See `docs/SIMPLIFIED_DIFF_PROPOSAL.md` for the full design document.

---

## Next Steps

1. ~~**Immediate**: Create comprehensive test matrix for all transitions~~ ✅ Fingerprint tests added
2. ~~**Short-term**: Add dev-mode assertions for unhandled transition detection~~ ✅ Simplified via fingerprints
3. ~~**Medium-term**: Document formal transition model~~ ✅ See SIMPLIFIED_DIFF_PROPOSAL.md
4. ~~**Long-term**: Consider gradual refactor toward state machine architecture~~ ✅ Replaced with fingerprint approach

### Remaining Work

1. [x] Benchmark wire size impact ✅ See `internal/diff/diff_bench_test.go`
2. [x] Remove deprecated code ✅ `StructureRegistry` and `AreStructuresSimilar` removed in v0.8.0
3. [x] Consider fingerprint caching ✅ Documented in SIMPLIFIED_DIFF_PROPOSAL.md (implement when benchmarks show need)
4. [x] Document MD5 collision risk ✅ See SIMPLIFIED_DIFF_PROPOSAL.md "Technical Considerations"
