# LiveTemplate Fuzz Testing Framework Design

> **Implementation Status:** This framework is implemented in `internal/fuzz/` with four subpackages.
> See the directory for the full file listing.

## Executive Summary

This document describes the systematic fuzz testing framework for LiveTemplate's core components. The framework validates the library's invariants under random state mutations, finding bugs that humans wouldn't think to test for.

### Key Findings from Codebase Analysis

The existing infrastructure provides a solid foundation:
- **2 Go fuzz tests**: `FuzzParseTemplateToTree` (template parsing) and `FuzzUserJourneys` (state transitions)
- **Property-based tests** using `pgregory.net/rapid` for diff correctness
- **Tree invariant validation** via `checkTreeInvariant()`
- **Test helpers**: `UpdateValidator`, `StateSimulator`, `ActivityGenerator`

However, the existing tests have gaps:
1. **No differential correctness verification** - we don't verify that `apply(oldTree, diff) == newTree`
2. **Limited mutation coverage** - ActivityGenerator uses fixed mutation types
3. **No shrinking** - failing sequences aren't minimized
4. **Template structure is static** - we don't fuzz template constructs themselves
5. **No concurrent rendering tests** - race conditions untested

---

## Framework Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          FUZZ TESTING FRAMEWORK                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │  Template   │    │    State    │    │  Mutation   │    │  Invariant  │  │
│  │  Generator  │    │  Generator  │    │  Generator  │    │  Verifier   │  │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘  │
│         │                  │                  │                  │          │
│         ▼                  ▼                  ▼                  ▼          │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                        FUZZ ORCHESTRATOR                             │  │
│  │  - Seed management and replay                                        │  │
│  │  - Mutation sequence execution                                       │  │
│  │  - Invariant checking after each mutation                            │  │
│  │  - Failure recording and shrinking                                   │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         SHRINK ENGINE                                │  │
│  │  - Remove mutations from sequence                                    │  │
│  │  - Simplify mutation parameters                                      │  │
│  │  - Find minimal reproduction                                         │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Design

### 1. Template Generator

Generate templates covering all constructs with varying complexity:

```go
// internal/fuzz/generators/template.go
package generators

import "pgregory.net/rapid"

// TemplateComplexity controls generation parameters
type TemplateComplexity struct {
    MaxDepth      int  // Maximum nesting depth
    MaxDynamics   int  // Maximum dynamic fields per level
    AllowNested   bool // Allow nested ranges/conditionals
    AllowTemplate bool // Allow {{template}} invokes
}

// GenTemplate generates a random valid template string
func GenTemplate(complexity TemplateComplexity) *rapid.Generator[string] {
    return rapid.Custom(func(t *rapid.T) string {
        return genTemplateNode(t, complexity, 0)
    })
}

// Template construct generators
func genFieldConstruct(t *rapid.T) string {
    fieldName := genFieldName(t)
    // 30% chance of pipeline
    if rapid.Float32().Draw(t, "pipeline_chance") < 0.3 {
        return fmt.Sprintf("{{.%s | %s}}", fieldName, genPipeline(t))
    }
    return fmt.Sprintf("{{.%s}}", fieldName)
}

func genConditionalConstruct(t *rapid.T, comp TemplateComplexity, depth int) string {
    cond := genCondition(t)
    thenBranch := genTemplateNode(t, comp, depth+1)

    // 50% chance of else branch
    if rapid.Bool().Draw(t, "has_else") {
        elseBranch := genTemplateNode(t, comp, depth+1)
        return fmt.Sprintf("{{if %s}}%s{{else}}%s{{end}}", cond, thenBranch, elseBranch)
    }
    return fmt.Sprintf("{{if %s}}%s{{end}}", cond, thenBranch)
}

func genRangeConstruct(t *rapid.T, comp TemplateComplexity, depth int) string {
    sliceName := genSliceName(t)
    body := genTemplateNode(t, comp, depth+1)

    // Generate key attribute with 80% probability
    keyAttr := ""
    if rapid.Float32().Draw(t, "has_key") < 0.8 {
        keyType := rapid.SampledFrom([]string{"id", "data-key", "key", "data-id"}).Draw(t, "key_type")
        keyAttr = fmt.Sprintf(` %s="{{.ID}}"`, keyType)
    }

    // 20% chance of else branch
    hasElse := rapid.Float32().Draw(t, "range_else") < 0.2

    if hasElse {
        return fmt.Sprintf(`{{range .%s}}<div%s>%s</div>{{else}}<div>empty</div>{{end}}`,
            sliceName, keyAttr, body)
    }
    return fmt.Sprintf(`{{range .%s}}<div%s>%s</div>{{end}}`, sliceName, keyAttr, body)
}

func genWithConstruct(t *rapid.T, comp TemplateComplexity, depth int) string {
    fieldName := genFieldName(t)
    body := genTemplateNode(t, comp, depth+1)

    if rapid.Bool().Draw(t, "with_else") {
        return fmt.Sprintf("{{with .%s}}%s{{else}}no value{{end}}", fieldName, body)
    }
    return fmt.Sprintf("{{with .%s}}%s{{end}}", fieldName, body)
}

// genTemplateNode generates a template node at given depth
func genTemplateNode(t *rapid.T, comp TemplateComplexity, depth int) string {
    if depth >= comp.MaxDepth {
        return genFieldConstruct(t)
    }

    numDynamics := rapid.IntRange(1, comp.MaxDynamics).Draw(t, "num_dynamics")
    var parts []string

    for i := 0; i < numDynamics; i++ {
        // Add static HTML
        parts = append(parts, genStaticHTML(t))

        // Add dynamic construct
        constructType := rapid.IntRange(0, 10).Draw(t, "construct_type")
        switch {
        case constructType < 5: // 50% field
            parts = append(parts, genFieldConstruct(t))
        case constructType < 7 && comp.AllowNested: // 20% conditional
            parts = append(parts, genConditionalConstruct(t, comp, depth))
        case constructType < 9 && comp.AllowNested: // 20% range
            parts = append(parts, genRangeConstruct(t, comp, depth))
        default: // 10% with
            parts = append(parts, genWithConstruct(t, comp, depth))
        }
    }

    // Add final static
    parts = append(parts, genStaticHTML(t))
    return strings.Join(parts, "")
}
```

### 2. State Generator

Generate state that matches template expectations:

```go
// internal/fuzz/generators/state.go
package generators

import "pgregory.net/rapid"

// StateShape describes the expected shape of state data
type StateShape struct {
    Fields    map[string]FieldType
    Slices    map[string]SliceShape
    Nested    map[string]*StateShape
}

type FieldType int
const (
    FieldString FieldType = iota
    FieldInt
    FieldBool
    FieldPointer
)

type SliceShape struct {
    ItemShape *StateShape
    MinLen    int
    MaxLen    int
}

// GenState generates state matching a shape
func GenState(shape *StateShape) *rapid.Generator[map[string]interface{}] {
    return rapid.Custom(func(t *rapid.T) map[string]interface{} {
        state := make(map[string]interface{})

        // Generate fields
        for name, ftype := range shape.Fields {
            state[name] = genFieldValue(t, ftype)
        }

        // Generate slices
        for name, sshape := range shape.Slices {
            length := rapid.IntRange(sshape.MinLen, sshape.MaxLen).Draw(t, name+"_len")
            items := make([]map[string]interface{}, length)
            for i := 0; i < length; i++ {
                items[i] = GenState(sshape.ItemShape).Draw(t, fmt.Sprintf("%s[%d]", name, i))
                // Ensure unique ID for range tracking
                items[i]["ID"] = fmt.Sprintf("item-%d", i)
            }
            state[name] = items
        }

        // Generate nested shapes
        for name, nested := range shape.Nested {
            state[name] = GenState(nested).Draw(t, name)
        }

        return state
    })
}

// InferShape infers state shape from a template
func InferShape(templateStr string) (*StateShape, error) {
    // Parse template and extract field references
    tmpl, err := template.New("infer").Parse(templateStr)
    if err != nil {
        return nil, err
    }

    shape := &StateShape{
        Fields: make(map[string]FieldType),
        Slices: make(map[string]SliceShape),
        Nested: make(map[string]*StateShape),
    }

    // Walk template AST to discover required fields
    walkTemplate(tmpl.Tree, shape)

    return shape, nil
}
```

### 3. Mutation Generator

Generate state mutations with weighted distribution:

```go
// internal/fuzz/mutations/mutations.go
package mutations

import "pgregory.net/rapid"

// MutationType defines the kind of mutation
type MutationType string

const (
    // Primitive mutations
    MutSetField      MutationType = "set_field"
    MutToggleBool    MutationType = "toggle_bool"
    MutIncrementInt  MutationType = "increment_int"
    MutSetNil        MutationType = "set_nil"
    MutSetEmpty      MutationType = "set_empty"

    // Slice mutations
    MutAppendSlice   MutationType = "append_slice"
    MutPrependSlice  MutationType = "prepend_slice"
    MutInsertSlice   MutationType = "insert_slice"
    MutRemoveSlice   MutationType = "remove_slice"
    MutClearSlice    MutationType = "clear_slice"
    MutReorderSlice  MutationType = "reorder_slice"
    MutReverseSlice  MutationType = "reverse_slice"
    MutDuplicateItem MutationType = "duplicate_item"
    MutSwapItems     MutationType = "swap_items"

    // Nested mutations
    MutUpdateItem    MutationType = "update_item"
    MutReplaceItem   MutationType = "replace_item"

    // Edge case mutations
    MutUnicodeString MutationType = "unicode_string"
    MutLargeString   MutationType = "large_string"
    MutSpecialChars  MutationType = "special_chars"
    MutEmptyString   MutationType = "empty_string"
    MutZeroInt       MutationType = "zero_int"
    MutNegativeInt   MutationType = "negative_int"
)

// Mutation represents a single state change
type Mutation struct {
    Type    MutationType
    Target  string      // Field path (e.g., "Items", "User.Name")
    Index   int         // For slice operations
    Value   interface{} // New value
}

// MutationWeights controls mutation frequency
type MutationWeights struct {
    SetField      float64
    ToggleBool    float64
    AppendSlice   float64
    RemoveSlice   float64
    ClearSlice    float64
    ReorderSlice  float64
    UpdateItem    float64
    EdgeCases     float64
}

var DefaultWeights = MutationWeights{
    SetField:      0.15,
    ToggleBool:    0.15,
    AppendSlice:   0.20,
    RemoveSlice:   0.15,
    ClearSlice:    0.05,
    ReorderSlice:  0.10,
    UpdateItem:    0.15,
    EdgeCases:     0.05,
}

// GenMutation generates a random mutation for given state shape
func GenMutation(shape *StateShape, weights MutationWeights) *rapid.Generator[Mutation] {
    return rapid.Custom(func(t *rapid.T) Mutation {
        choice := rapid.Float64().Draw(t, "mutation_choice")

        cumulative := 0.0

        // Set field
        cumulative += weights.SetField
        if choice < cumulative && len(shape.Fields) > 0 {
            fieldName := rapid.SampledFrom(keys(shape.Fields)).Draw(t, "field")
            return Mutation{
                Type:   MutSetField,
                Target: fieldName,
                Value:  genFieldValue(t, shape.Fields[fieldName]),
            }
        }

        // Toggle bool
        cumulative += weights.ToggleBool
        if choice < cumulative {
            boolFields := filterByType(shape.Fields, FieldBool)
            if len(boolFields) > 0 {
                return Mutation{
                    Type:   MutToggleBool,
                    Target: rapid.SampledFrom(boolFields).Draw(t, "bool_field"),
                }
            }
        }

        // Slice mutations
        if len(shape.Slices) > 0 {
            sliceName := rapid.SampledFrom(keys(shape.Slices)).Draw(t, "slice")
            sliceShape := shape.Slices[sliceName]

            cumulative += weights.AppendSlice
            if choice < cumulative {
                return Mutation{
                    Type:   MutAppendSlice,
                    Target: sliceName,
                    Value:  GenState(sliceShape.ItemShape).Draw(t, "new_item"),
                }
            }

            cumulative += weights.RemoveSlice
            if choice < cumulative {
                return Mutation{
                    Type:   MutRemoveSlice,
                    Target: sliceName,
                    Index:  rapid.IntRange(0, sliceShape.MaxLen-1).Draw(t, "remove_idx"),
                }
            }

            cumulative += weights.ClearSlice
            if choice < cumulative {
                return Mutation{
                    Type:   MutClearSlice,
                    Target: sliceName,
                }
            }

            cumulative += weights.ReorderSlice
            if choice < cumulative {
                return Mutation{
                    Type:   MutReorderSlice,
                    Target: sliceName,
                    Value:  genPermutation(t, sliceShape.MaxLen),
                }
            }
        }

        // Edge cases
        cumulative += weights.EdgeCases
        if choice < cumulative {
            return genEdgeCaseMutation(t, shape)
        }

        // Default: update item
        if len(shape.Slices) > 0 {
            sliceName := rapid.SampledFrom(keys(shape.Slices)).Draw(t, "slice")
            return Mutation{
                Type:   MutUpdateItem,
                Target: sliceName,
                Index:  rapid.IntRange(0, shape.Slices[sliceName].MaxLen-1).Draw(t, "update_idx"),
                Value:  genItemUpdate(t, shape.Slices[sliceName].ItemShape),
            }
        }

        // Fallback
        return Mutation{
            Type:   MutSetField,
            Target: rapid.SampledFrom(keys(shape.Fields)).Draw(t, "field"),
            Value:  "fallback",
        }
    })
}

// genEdgeCaseMutation generates mutations that target edge cases
func genEdgeCaseMutation(t *rapid.T, shape *StateShape) Mutation {
    edgeType := rapid.SampledFrom([]MutationType{
        MutUnicodeString,
        MutLargeString,
        MutSpecialChars,
        MutEmptyString,
        MutZeroInt,
        MutNegativeInt,
        MutSetNil,
    }).Draw(t, "edge_type")

    switch edgeType {
    case MutUnicodeString:
        return Mutation{
            Type:   MutSetField,
            Target: findStringField(shape),
            Value:  genUnicodeString(t),
        }
    case MutLargeString:
        size := rapid.IntRange(1000, 10000).Draw(t, "large_size")
        return Mutation{
            Type:   MutSetField,
            Target: findStringField(shape),
            Value:  strings.Repeat("x", size),
        }
    case MutSpecialChars:
        return Mutation{
            Type:   MutSetField,
            Target: findStringField(shape),
            Value:  genSpecialCharsString(t),
        }
    case MutEmptyString:
        return Mutation{
            Type:   MutSetField,
            Target: findStringField(shape),
            Value:  "",
        }
    default:
        return Mutation{Type: MutSetEmpty, Target: findSliceField(shape)}
    }
}

// genUnicodeString generates strings with interesting unicode
func genUnicodeString(t *rapid.T) string {
    unicodeRanges := []string{
        "Hello \U0001F600 World",           // Emoji
        "日本語テスト",                        // Japanese
        "Привет мир",                       // Cyrillic
        "مرحبا بالعالم",                     // Arabic (RTL)
        "🎉🎊🎈🎁",                           // Multiple emoji
        "\u200B\u200C\u200D",               // Zero-width chars
        "a\u0308",                          // Combining diacritical
        "\uFEFF",                           // BOM
        "<script>alert('xss')</script>",   // HTML/XSS
        "foo\x00bar",                       // Null byte
        "line1\nline2\rline3\r\nline4",    // Line endings
    }
    return rapid.SampledFrom(unicodeRanges).Draw(t, "unicode")
}

// genSpecialCharsString generates HTML-sensitive strings
func genSpecialCharsString(t *rapid.T) string {
    specialStrings := []string{
        "<div>test</div>",
        "a & b",
        "\"quoted\"",
        "'single'",
        "<>\"'&",
        "{{.Field}}",     // Template injection attempt
        "{{ }}",
        "}}{{",
        "${var}",
        "<%=var%>",
    }
    return rapid.SampledFrom(specialStrings).Draw(t, "special")
}
```

### 4. Invariant Verifier

Verify all core invariants after each mutation:

```go
// internal/fuzz/invariants/verifier.go
package invariants

import (
    "github.com/livetemplate/livetemplate/internal/build"
    "github.com/livetemplate/livetemplate/internal/diff"
)

// InvariantViolation records a failed invariant check
type InvariantViolation struct {
    Invariant   string
    Description string
    OldState    interface{}
    NewState    interface{}
    OldTree     *build.TreeNode
    NewTree     *build.TreeNode
    Diff        *build.TreeNode
    Mutations   []Mutation
    Seed        int64
}

// Verifier checks all invariants
type Verifier struct {
    violations []InvariantViolation
    seed       int64
    mutations  []Mutation
}

// NewVerifier creates a new invariant verifier
func NewVerifier(seed int64) *Verifier {
    return &Verifier{seed: seed}
}

// RecordMutation tracks mutation history for debugging
func (v *Verifier) RecordMutation(m Mutation) {
    v.mutations = append(v.mutations, m)
}

// VerifyAll checks all invariants for a render transition
func (v *Verifier) VerifyAll(
    oldState, newState interface{},
    oldTree, newTree *build.TreeNode,
    diffTree *build.TreeNode,
    isFirstRender bool,
) error {
    // Check each invariant
    if err := v.verifyDiffCorrectness(oldTree, newTree, diffTree); err != nil {
        return err
    }

    if err := v.verifyUpdateMinimality(diffTree, isFirstRender); err != nil {
        return err
    }

    if err := v.verifyKeyStability(oldTree, newTree); err != nil {
        return err
    }

    if err := v.verifyIdempotence(newState, newTree); err != nil {
        return err
    }

    if err := v.verifyNoDataLoss(oldState, newState); err != nil {
        return err
    }

    if err := v.verifyTreeStructure(newTree); err != nil {
        return err
    }

    return nil
}

// Invariant 1: Diff Correctness
// Applying the diff to oldTree must produce newTree
func (v *Verifier) verifyDiffCorrectness(oldTree, newTree, diffTree *build.TreeNode) error {
    if oldTree == nil || newTree == nil {
        return nil // First render or tree removal
    }

    // Apply diff to old tree
    reconstructed := applyDiff(oldTree, diffTree)

    // Compare with new tree (dynamics only, statics are cached)
    if !treeDynamicsEqual(reconstructed, newTree) {
        return &InvariantViolation{
            Invariant:   "DiffCorrectness",
            Description: "Applying diff to old tree does not produce new tree",
            OldTree:     oldTree,
            NewTree:     newTree,
            Diff:        diffTree,
            Mutations:   v.mutations,
            Seed:        v.seed,
        }
    }

    return nil
}

// applyDiff reconstructs a tree by applying a diff to an old tree
func applyDiff(oldTree, diffTree *build.TreeNode) *build.TreeNode {
    if diffTree == nil {
        return oldTree
    }

    result := oldTree.Clone()

    // Apply dynamic changes
    for k, newValue := range diffTree.Dynamics {
        // Handle range operations
        if k == "d" {
            if ops, ok := newValue.([]interface{}); ok {
                result = applyRangeOps(result, ops)
                continue
            }
        }

        // Handle nested tree changes
        if newTreeNode, ok := newValue.(*build.TreeNode); ok {
            if oldValue, exists := result.GetDynamic(k); exists {
                if oldTreeNode, ok := oldValue.(*build.TreeNode); ok {
                    result.SetDynamic(k, applyDiff(oldTreeNode, newTreeNode))
                    continue
                }
            }
        }

        // Direct value replacement
        result.SetDynamic(k, newValue)
    }

    return result
}

// applyRangeOps applies differential range operations
func applyRangeOps(tree *build.TreeNode, ops []interface{}) *build.TreeNode {
    if !tree.HasRange() || tree.Range == nil {
        return tree
    }

    items := make([]interface{}, len(tree.Range.Items))
    copy(items, tree.Range.Items)

    // Build key-to-index map
    keyIndex := make(map[string]int)
    for i, item := range items {
        if key := extractKey(item); key != "" {
            keyIndex[key] = i
        }
    }

    for _, op := range ops {
        opArray, ok := op.([]interface{})
        if !ok || len(opArray) < 2 {
            continue
        }

        opType := opArray[0].(string)
        switch opType {
        case "r": // Remove
            key := opArray[1].(string)
            if idx, exists := keyIndex[key]; exists {
                items = append(items[:idx], items[idx+1:]...)
                // Update keyIndex
                for k, i := range keyIndex {
                    if i > idx {
                        keyIndex[k] = i - 1
                    }
                }
                delete(keyIndex, key)
            }

        case "i": // Insert
            afterKey := opArray[1].(string)
            // position := opArray[2].(string)
            data := opArray[3]

            insertIdx := 0
            if afterKey != "" {
                if idx, exists := keyIndex[afterKey]; exists {
                    insertIdx = idx + 1
                }
            }

            // Insert at position
            items = append(items[:insertIdx], append([]interface{}{data}, items[insertIdx:]...)...)

        case "u": // Update
            key := opArray[1].(string)
            changes := opArray[2].(map[string]interface{})
            if idx, exists := keyIndex[key]; exists {
                items[idx] = applyItemChanges(items[idx], changes)
            }

        case "o": // Reorder
            newOrder := opArray[1].([]interface{})
            reordered := make([]interface{}, len(newOrder))
            for i, key := range newOrder {
                if idx, exists := keyIndex[key.(string)]; exists {
                    reordered[i] = items[idx]
                }
            }
            items = reordered

        case "a": // Append (empty→items)
            if len(opArray) > 2 {
                newItems := opArray[2].([]interface{})
                items = append(items, newItems...)
            }
        }
    }

    result := tree.Clone()
    result.Range.Items = items
    return result
}

// Invariant 2: Update Minimality
// Updates contain only changed dynamics, never unchanged statics
func (v *Verifier) verifyUpdateMinimality(diffTree *build.TreeNode, isFirstRender bool) error {
    if isFirstRender {
        // First render MUST have statics
        if !treeHasStatics(diffTree) {
            return &InvariantViolation{
                Invariant:   "UpdateMinimality",
                Description: "First render missing statics",
                Diff:        diffTree,
                Mutations:   v.mutations,
                Seed:        v.seed,
            }
        }
        return nil
    }

    // Subsequent updates MUST NOT have statics (client cached them)
    if treeHasStaticsDeep(diffTree) {
        return &InvariantViolation{
            Invariant:   "UpdateMinimality",
            Description: "Update includes unchanged statics (should be stripped)",
            Diff:        diffTree,
            Mutations:   v.mutations,
            Seed:        v.seed,
        }
    }

    return nil
}

// treeHasStaticsDeep checks if tree or nested trees contain "s" keys
func treeHasStaticsDeep(tree *build.TreeNode) bool {
    if tree == nil {
        return false
    }

    if tree.HasStatics() {
        return true
    }

    // Check nested dynamics
    for _, value := range tree.Dynamics {
        if nested, ok := value.(*build.TreeNode); ok {
            if treeHasStaticsDeep(nested) {
                return true
            }
        }
        // Check range operations for statics
        if ops, ok := value.([]interface{}); ok {
            for _, op := range ops {
                if opArray, ok := op.([]interface{}); ok && len(opArray) > 0 {
                    // Insert and append ops may have data with statics
                    if opArray[0] == "i" || opArray[0] == "a" {
                        if hasStaticsInData(opArray) {
                            return true
                        }
                    }
                }
            }
        }
    }

    return false
}

// Invariant 3: Key Stability
// Same list item keeps same key across renders
func (v *Verifier) verifyKeyStability(oldTree, newTree *build.TreeNode) error {
    if oldTree == nil || newTree == nil {
        return nil
    }

    // Extract ranges and compare keys
    oldRanges := extractAllRanges(oldTree)
    newRanges := extractAllRanges(newTree)

    for path, oldRange := range oldRanges {
        newRange, exists := newRanges[path]
        if !exists {
            continue
        }

        // Build ID-to-key maps
        oldIDKeys := buildIDKeyMap(oldRange)
        newIDKeys := buildIDKeyMap(newRange)

        // For items that exist in both, keys must match
        for id, oldKey := range oldIDKeys {
            if newKey, exists := newIDKeys[id]; exists {
                if oldKey != newKey {
                    return &InvariantViolation{
                        Invariant:   "KeyStability",
                        Description: fmt.Sprintf("Item %q changed key from %q to %q", id, oldKey, newKey),
                        OldTree:     oldTree,
                        NewTree:     newTree,
                        Mutations:   v.mutations,
                        Seed:        v.seed,
                    }
                }
            }
        }
    }

    return nil
}

// Invariant 4: Idempotence
// Same state always produces same tree
func (v *Verifier) verifyIdempotence(state interface{}, tree *build.TreeNode) error {
    // This requires re-rendering with same state
    // Will be implemented in orchestrator where we have template access
    return nil
}

// Invariant 5: No Data Loss
// State survives JSON roundtrip
func (v *Verifier) verifyNoDataLoss(oldState, newState interface{}) error {
    // Serialize and deserialize
    data, err := json.Marshal(newState)
    if err != nil {
        return &InvariantViolation{
            Invariant:   "NoDataLoss",
            Description: fmt.Sprintf("JSON marshal failed: %v", err),
            NewState:    newState,
            Mutations:   v.mutations,
            Seed:        v.seed,
        }
    }

    var roundtripped interface{}
    if err := json.Unmarshal(data, &roundtripped); err != nil {
        return &InvariantViolation{
            Invariant:   "NoDataLoss",
            Description: fmt.Sprintf("JSON unmarshal failed: %v", err),
            NewState:    newState,
            Mutations:   v.mutations,
            Seed:        v.seed,
        }
    }

    // Deep compare (with type normalization for JSON)
    if !jsonEqual(newState, roundtripped) {
        return &InvariantViolation{
            Invariant:   "NoDataLoss",
            Description: "State changed after JSON roundtrip",
            OldState:    newState,
            NewState:    roundtripped,
            Mutations:   v.mutations,
            Seed:        v.seed,
        }
    }

    return nil
}

// Invariant 6: Tree Structure
// len(statics) == len(dynamics) + 1
func (v *Verifier) verifyTreeStructure(tree *build.TreeNode) error {
    return verifyTreeStructureRecursive(tree, "root")
}

func verifyTreeStructureRecursive(tree *build.TreeNode, path string) error {
    if tree == nil {
        return nil
    }

    // Only check if this is a full tree (not a diff-only update)
    if !tree.HasStatics() {
        // Diff-only, no structure check needed
        return nil
    }

    // Check statics/dynamics ratio
    if tree.HasRange() {
        // Range: check item structure
        if tree.Range != nil && len(tree.Range.Items) > 0 {
            for i, item := range tree.Range.Items {
                if itemTree, ok := item.(*build.TreeNode); ok {
                    if err := verifyTreeStructureRecursive(itemTree, fmt.Sprintf("%s.d[%d]", path, i)); err != nil {
                        return err
                    }
                }
            }
        }
    } else {
        // Regular tree: len(statics) == len(dynamics) + 1
        staticsLen := len(tree.Statics)
        dynamicsLen := len(tree.Dynamics)

        if staticsLen != dynamicsLen+1 {
            return fmt.Errorf("tree structure violation at %s: len(statics)=%d, len(dynamics)=%d, expected len(statics)=len(dynamics)+1",
                path, staticsLen, dynamicsLen)
        }
    }

    // Check nested structures
    for k, value := range tree.Dynamics {
        if nested, ok := value.(*build.TreeNode); ok {
            if err := verifyTreeStructureRecursive(nested, path+"."+k); err != nil {
                return err
            }
        }
    }

    return nil
}
```

### 5. Shrink Engine

Minimize failing sequences:

```go
// internal/fuzz/shrink/shrink.go
package shrink

// ShrinkerConfig controls shrinking behavior
type ShrinkerConfig struct {
    MaxIterations int  // Maximum shrinking iterations
    MinSequence   int  // Don't shrink below this length
}

// Shrinker reduces failing sequences to minimal reproduction
type Shrinker struct {
    config   ShrinkerConfig
    verifier *invariants.Verifier
}

// Shrink reduces a failing mutation sequence to minimal form
func (s *Shrinker) Shrink(
    template string,
    initialState interface{},
    mutations []Mutation,
    failingInvariant string,
) []Mutation {
    current := mutations

    for iteration := 0; iteration < s.config.MaxIterations; iteration++ {
        improved := false

        // Strategy 1: Try removing each mutation
        for i := 0; i < len(current); i++ {
            candidate := removeAt(current, i)
            if len(candidate) < s.config.MinSequence {
                continue
            }

            if s.stillFails(template, initialState, candidate, failingInvariant) {
                current = candidate
                improved = true
                break
            }
        }

        if improved {
            continue
        }

        // Strategy 2: Try simplifying mutation values
        for i := 0; i < len(current); i++ {
            simplified := s.simplifyMutation(current[i])
            if simplified == nil {
                continue
            }

            candidate := replaceAt(current, i, *simplified)
            if s.stillFails(template, initialState, candidate, failingInvariant) {
                current = candidate
                improved = true
                break
            }
        }

        if !improved {
            break // No more improvements possible
        }
    }

    return current
}

// simplifyMutation attempts to make a mutation simpler
func (s *Shrinker) simplifyMutation(m Mutation) *Mutation {
    switch m.Type {
    case MutSetField:
        if str, ok := m.Value.(string); ok && len(str) > 1 {
            return &Mutation{
                Type:   m.Type,
                Target: m.Target,
                Value:  str[:len(str)/2], // Half the string
            }
        }

    case MutAppendSlice:
        // Try with simpler item
        if item, ok := m.Value.(map[string]interface{}); ok {
            simplified := simplifyMap(item)
            if len(simplified) < len(item) {
                return &Mutation{
                    Type:   m.Type,
                    Target: m.Target,
                    Value:  simplified,
                }
            }
        }

    case MutReorderSlice:
        if order, ok := m.Value.([]int); ok && len(order) > 2 {
            // Try just swapping first two
            return &Mutation{
                Type:   m.Type,
                Target: m.Target,
                Value:  []int{1, 0},
            }
        }
    }

    return nil
}
```

### 6. Fuzz Orchestrator

Coordinate all components:

```go
// internal/fuzz/orchestrator.go
package fuzz

import (
    "testing"
    "pgregory.net/rapid"
)

// FuzzConfig configures the fuzzing session
type FuzzConfig struct {
    // Sequence parameters
    MinMutations    int
    MaxMutations    int

    // Template parameters
    TemplateComplexity generators.TemplateComplexity

    // Mutation parameters
    MutationWeights mutations.MutationWeights

    // Shrinking
    ShrinkConfig    shrink.ShrinkerConfig

    // Output
    VerboseLog      bool
    RecordFailures  bool
    FailureDir      string
}

var DefaultFuzzConfig = FuzzConfig{
    MinMutations: 5,
    MaxMutations: 100,
    TemplateComplexity: generators.TemplateComplexity{
        MaxDepth:    3,
        MaxDynamics: 5,
        AllowNested: true,
    },
    MutationWeights: mutations.DefaultWeights,
    ShrinkConfig: shrink.ShrinkerConfig{
        MaxIterations: 100,
        MinSequence:   1,
    },
    RecordFailures: true,
    FailureDir:     "testdata/fuzz_failures",
}

// FuzzSession represents a single fuzzing run
type FuzzSession struct {
    config      FuzzConfig
    template    *livetemplate.Template
    templateStr string
    stateShape  *generators.StateShape
    state       map[string]interface{}
    mutations   []Mutation
    trees       []*build.TreeNode
    verifier    *invariants.Verifier
    seed        int64
}

// RunFuzz executes the fuzz test with rapid
func RunFuzz(t *testing.T, config FuzzConfig) {
    rapid.Check(t, func(rt *rapid.T) {
        // Generate or use fixed template
        templateStr := generators.GenTemplate(config.TemplateComplexity).Draw(rt, "template")

        // Infer state shape from template
        shape, err := generators.InferShape(templateStr)
        if err != nil {
            rt.Skip("Invalid template generated")
        }

        // Create session
        session := &FuzzSession{
            config:      config,
            templateStr: templateStr,
            stateShape:  shape,
            seed:        rt.Seed(),
            verifier:    invariants.NewVerifier(rt.Seed()),
        }

        // Initialize template
        session.template = livetemplate.New("fuzz")
        if _, err := session.template.Parse(templateStr); err != nil {
            rt.Skip("Template parse error")
        }

        // Generate initial state
        session.state = generators.GenState(shape).Draw(rt, "initial_state")

        // First render
        initialTree, err := session.template.Execute(session.state)
        if err != nil {
            rt.Skip("Initial render failed")
        }
        session.trees = append(session.trees, initialTree)

        // Verify first render invariants
        if err := session.verifier.VerifyAll(
            nil, session.state,
            nil, initialTree,
            initialTree,
            true, // isFirstRender
        ); err != nil {
            t.Fatalf("First render invariant violation: %v", err)
        }

        // Generate mutation sequence
        numMutations := rapid.IntRange(config.MinMutations, config.MaxMutations).Draw(rt, "num_mutations")

        for i := 0; i < numMutations; i++ {
            // Generate mutation
            mutation := mutations.GenMutation(shape, config.MutationWeights).Draw(rt, fmt.Sprintf("mutation_%d", i))
            session.mutations = append(session.mutations, mutation)
            session.verifier.RecordMutation(mutation)

            // Apply mutation to state
            oldState := deepCopy(session.state)
            session.state = mutations.Apply(session.state, mutation)

            // Render new tree
            prevTree := session.trees[len(session.trees)-1]
            newTree, err := session.template.Execute(session.state)
            if err != nil {
                // Record failure context
                session.recordFailure("render_error", err)
                rt.Fatalf("Render failed after mutation %d: %v\nMutation: %+v\nState: %+v",
                    i, err, mutation, session.state)
            }

            // Compute diff
            diffTree := session.template.GetChanges()
            session.trees = append(session.trees, newTree)

            // Verify all invariants
            if err := session.verifier.VerifyAll(
                oldState, session.state,
                prevTree, newTree,
                diffTree,
                false, // not first render
            ); err != nil {
                // Shrink the sequence
                shrunk := session.shrinkFailure(err.(*invariants.InvariantViolation))
                session.recordFailure(err.(*invariants.InvariantViolation).Invariant, shrunk)
                rt.Fatalf("Invariant violation after mutation %d:\n%v\nShrunk to %d mutations",
                    i, err, len(shrunk))
            }
        }
    })
}

// shrinkFailure reduces a failing sequence to minimal reproduction
func (s *FuzzSession) shrinkFailure(violation *invariants.InvariantViolation) []Mutation {
    shrinker := &shrink.Shrinker{
        config:   s.config.ShrinkConfig,
        verifier: invariants.NewVerifier(s.seed),
    }

    return shrinker.Shrink(
        s.templateStr,
        generators.GenState(s.stateShape).Example(int(s.seed)),
        s.mutations,
        violation.Invariant,
    )
}

// recordFailure saves failure details for later analysis
func (s *FuzzSession) recordFailure(name string, data interface{}) {
    if !s.config.RecordFailures {
        return
    }

    filename := filepath.Join(s.config.FailureDir,
        fmt.Sprintf("%s_%d.json", name, s.seed))

    failure := map[string]interface{}{
        "seed":        s.seed,
        "template":    s.templateStr,
        "mutations":   s.mutations,
        "violation":   data,
    }

    jsonData, _ := json.MarshalIndent(failure, "", "  ")
    os.WriteFile(filename, jsonData, 0644)
}

// FuzzWithFixedTemplate runs fuzz testing with a specific template
func FuzzWithFixedTemplate(t *testing.T, templateStr string, config FuzzConfig) {
    rapid.Check(t, func(rt *rapid.T) {
        shape, err := generators.InferShape(templateStr)
        if err != nil {
            rt.Fatal(err)
        }

        // ... same as RunFuzz but with fixed template
    })
}
```

---

## Integration with Existing Tests

### Reusing Existing Infrastructure

The framework builds on existing test helpers:

```go
// Reuse existing UpdateValidator
validator := NewUpdateValidator()

// Reuse existing StateSimulator (adapt to new mutation format)
type MutationAdapter struct {
    simulator *StateSimulator
}

func (a *MutationAdapter) Apply(m Mutation) {
    activity := convertMutationToActivity(m)
    a.simulator.ApplyActivity(activity)
}

// Reuse existing ActivityGenerator for baseline tests
func TestExistingActivityGeneratorWithInvariants(t *testing.T) {
    gen := NewActivityGenerator(time.Now().UnixNano())
    verifier := invariants.NewVerifier(gen.Rand.Int63())

    journey := gen.GenerateJourney(50)
    // ... verify invariants for each step
}
```

### New Fuzz Test Functions

```go
// tree_fuzz_test.go

// Existing: FuzzParseTemplateToTree (keep as-is, add invariant checks)

// New: Comprehensive diff fuzzing
func FuzzDiffCorrectness(f *testing.F) {
    // Add seed corpus
    f.Add(`<div>{{.Name}}</div>`, `{"Name":"test"}`, `[{"type":"set_field","target":"Name","value":"changed"}]`)

    f.Fuzz(func(t *testing.T, templateStr, initialStateJSON, mutationsJSON string) {
        // Parse inputs
        var state map[string]interface{}
        if err := json.Unmarshal([]byte(initialStateJSON), &state); err != nil {
            t.Skip()
        }

        var mutations []Mutation
        if err := json.Unmarshal([]byte(mutationsJSON), &mutations); err != nil {
            t.Skip()
        }

        // Run with invariant verification
        session := NewFuzzSession(templateStr, state, mutations)
        if err := session.RunAndVerify(); err != nil {
            t.Errorf("Invariant violation: %v", err)
        }
    })
}

// New: Range operations fuzzing
func FuzzRangeOperations(f *testing.F) {
    rangeTemplate := `{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}`

    f.Add(rangeTemplate, `{"Items":[]}`, `[{"type":"append_slice","target":"Items"}]`)

    f.Fuzz(func(t *testing.T, templateStr, stateJSON, mutationsJSON string) {
        // Focus on range mutation patterns
        // ...
    })
}

// New: Key stability fuzzing
func FuzzKeyStability(f *testing.F) {
    f.Fuzz(func(t *testing.T, seed []byte) {
        rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed))))

        // Generate sequence that stresses key generation
        session := NewKeyStabilitySession(rng)
        session.RunPermutationTests()

        if violations := session.GetViolations(); len(violations) > 0 {
            t.Errorf("Key stability violations: %v", violations)
        }
    })
}

// New: Concurrent rendering fuzzing
func FuzzConcurrentRendering(f *testing.F) {
    f.Fuzz(func(t *testing.T, seed []byte) {
        rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seed))))

        template := livetemplate.New("concurrent")
        template.Parse(`<div>{{.Counter}}</div>`)

        var wg sync.WaitGroup
        errors := make(chan error, 100)

        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(id int) {
                defer wg.Done()

                for j := 0; j < 100; j++ {
                    state := map[string]interface{}{
                        "Counter": rng.Intn(1000),
                    }

                    _, err := template.Execute(state)
                    if err != nil {
                        errors <- fmt.Errorf("goroutine %d iteration %d: %v", id, j, err)
                    }
                }
            }(i)
        }

        wg.Wait()
        close(errors)

        for err := range errors {
            t.Error(err)
        }
    })
}
```

---

## CI Integration

### Running Fuzz Tests

```yaml
# .github/workflows/fuzz.yml
name: Fuzz Testing

on:
  push:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  fuzz:
    runs-on: ubuntu-latest
    timeout-minutes: 60

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run fuzz tests
        run: |
          go test -fuzz=FuzzDiffCorrectness -fuzztime=10m ./...
          go test -fuzz=FuzzRangeOperations -fuzztime=10m ./...
          go test -fuzz=FuzzKeyStability -fuzztime=5m ./...
          go test -fuzz=FuzzConcurrentRendering -fuzztime=5m ./...

      - name: Upload crash artifacts
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: fuzz-crashes
          path: testdata/fuzz/*/
```

### Corpus Management

```bash
# Run fuzz locally and save interesting inputs
go test -fuzz=FuzzDiffCorrectness -fuzztime=1h -test.v

# Minimize corpus
go test -fuzz=FuzzDiffCorrectness -fuzzminimizetime=10m

# Verify corpus still triggers all code paths
go test -run=FuzzDiffCorrectness -cover
```

---

## Bug Patterns to Detect

Based on the codebase analysis, these are likely bug patterns:

### 1. Range Operation Edge Cases

```go
// Pattern: empty→items→empty→items
mutations := []Mutation{
    {Type: MutClearSlice, Target: "Items"},     // empty
    {Type: MutAppendSlice, Target: "Items"},    // items
    {Type: MutClearSlice, Target: "Items"},     // empty again
    {Type: MutAppendSlice, Target: "Items"},    // items again
}
// Bug: Keys may not be properly reset, or statics may leak
```

### 2. Conditional/Range Interaction

```go
// Pattern: toggle visibility of range
// Template: {{if .ShowList}}{{range .Items}}...{{end}}{{end}}
mutations := []Mutation{
    {Type: MutToggleBool, Target: "ShowList"},  // hide
    {Type: MutAppendSlice, Target: "Items"},    // add while hidden
    {Type: MutToggleBool, Target: "ShowList"},  // show
}
// Bug: Items added while hidden may not render correctly
```

### 3. Nested Range Reorder

```go
// Pattern: reorder parent while modifying children
// Template: {{range .Groups}}{{range .Items}}...{{end}}{{end}}
mutations := []Mutation{
    {Type: MutReorderSlice, Target: "Groups"},
    {Type: MutAppendSlice, Target: "Groups.0.Items"},
}
// Bug: Keys may become misaligned after parent reorder
```

### 4. Unicode and Special Characters

```go
// Pattern: special characters in keys and values
mutations := []Mutation{
    {Type: MutSetField, Target: "Name", Value: "Hello 🎉 World"},
    {Type: MutAppendSlice, Target: "Items", Value: map[string]interface{}{
        "ID": "item-日本語",
        "Text": "<script>xss</script>",
    }},
}
// Bug: JSON roundtrip may corrupt unicode, HTML escaping may fail
```

### 5. Fingerprint Collisions

```go
// Pattern: different structures with same fingerprint
// (Requires careful crafting or high iteration count)
mutations := []Mutation{
    // Modify structure in way that keeps same fingerprint
    // but changes actual content
}
// Bug: ClientNeedsStatics may return false when it should return true
```

---

## Success Metrics

1. **Bug Discovery**: Framework finds at least 1 real bug in first 1000 runs
2. **Shrinking Quality**: Failing sequences reduced to ≤10 mutations
3. **Coverage**: Property tests cover >80% of internal/diff/ code paths
4. **Performance**: Can run 1000 mutation sequences per minute
5. **CI Integration**: Runs daily, alerts on new crashes
6. **Reproducibility**: All failures can be replayed with recorded seed

---

## Implementation Priority

### Phase 1: Core Framework (Week 1)
1. Mutation generator with all mutation types
2. Basic invariant verifier (diff correctness, update minimality)
3. Integration with existing rapid tests
4. Recording/replay infrastructure

### Phase 2: Complete Invariants (Week 2)
1. Key stability verifier
2. Tree structure verifier
3. JSON roundtrip verifier
4. Concurrent rendering tests

### Phase 3: Shrinking & Polish (Week 3)
1. Full shrink engine
2. Template generator (optional)
3. CI integration
4. Documentation and examples

---

## Appendix: Comparison with Existing Tests

| Feature | Existing | Proposed |
|---------|----------|----------|
| Template coverage | 50+ seed templates | Unlimited generation |
| State mutations | 6 activity types | 20+ mutation types |
| Invariant checks | Tree structure only | 6 core invariants |
| Shrinking | None | Full sequence shrinking |
| Edge cases | Manual | Weighted generation |
| Concurrency | None | Race condition detection |
| CI integration | Basic `go test` | Daily fuzz campaign |

The proposed framework complements rather than replaces existing tests:
- Keep `FuzzParseTemplateToTree` for parsing coverage
- Keep `FuzzUserJourneys` for realistic user simulation
- Add new fuzz tests for algorithmic invariants
- Add property tests for mathematical properties
