# LiveTemplate Tree Update Specification

Version: 1.0.0
Last Updated: 2026-03-07
Status: Final

## 1. Introduction

This document provides the formal specification for LiveTemplate's tree-based update generation system. It defines the exact structure, rules, and behaviors that all LiveTemplate implementations must follow to ensure correct and efficient template updates.

## 2. Core Concepts

### 2.1 Tree Node Structure

A tree node represents the separation of static and dynamic content in templates.

**Go Representation (Server-side):**
```go
type TreeNode struct {
    Statics     []string                // Static HTML parts (key: "s")
    Dynamics    map[string]interface{}  // Dynamic content (keys: "0", "1", etc.)
    Fingerprint string                  // Full value fingerprint (key: "f")
    Range       *RangeData             // Range operation data (key: "d")
    Metadata    *TreeMetadata          // Additional metadata (key: "m")
}

type RangeData struct {
    Items   []interface{}  // Range items or operations
    Statics []string       // Shared static HTML for rendering items
}

type TreeMetadata struct {
    IDKey string  // Dynamic position used as unique item identifier
}
```

**TypeScript Representation (Client-side):**
```typescript
interface TreeNode {
  // Static content array (template structure)
  "s"?: string[];

  // Dynamic content slots (indexed by numeric strings)
  [key: `${number}`]: any;

  // Range data array (for iteration constructs)
  "d"?: any[];

  // Structure fingerprint (for change detection)
  "f"?: string;

  // Metadata (for item key identification)
  "m"?: { idKey: string };
}
```

**Wire Format (JSON):**
```json
{
  "s": ["<div>", "</div>"],
  "0": "Dynamic content",
  "1": { "s": ["<span>", "</span>"], "0": "Nested" },
  "d": [{ "0": "id-1", "1": "Item 1" }],
  "f": "abc123def456ab78",
  "m": { "idKey": "0" }
}
```

The TreeNode serializes to a flat JSON object. Custom JSON marshaling ensures that `Dynamics` entries merge into the top-level object alongside reserved keys (`"s"`, `"d"`, `"f"`, `"m"`).

#### Rules:
1. The `"s"` key MUST contain an array of strings representing static HTML/text segments
2. Numeric string keys (`"0"`, `"1"`, etc.) MUST be used for dynamic content slots
3. The `"d"` key MUST be used exclusively for range construct data
4. The `"f"` key contains the structure fingerprint (16 hex characters, MD5-based)
5. The `"m"` key contains metadata such as the `idKey` for item identification
6. Numeric keys MUST be ordered sequentially starting from `"0"`
7. Empty dynamics MAY be represented as empty strings `""`

### 2.2 Structure Fingerprinting

Structure fingerprints enable O(1) comparison to determine whether the client needs statics re-sent.

**What gets fingerprinted:**
- Statics arrays (the HTML template parts between dynamic slots)
- Dynamic key positions (that a dynamic exists at position 0, 1, 2, etc.)
- Nested TreeNode structure (recursively)
- Range statics (the item template structure)

**What is NOT fingerprinted:**
- Dynamic values (two trees with identical structure but different content produce the same fingerprint)

**Algorithm:**
1. MD5 hash of the static structure, truncated to 64 bits (16 hex characters)
2. Lazy-computed and cached on first access via `GetStructureFingerprint()`
3. Subsequent comparisons are O(1) using cached values

**Decision flow:**
```go
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
    if oldTree == nil { return true }   // First render
    if newTree == nil { return false }  // Tree removed
    return oldTree.GetStructureFingerprint() != newTree.GetStructureFingerprint()
}
```

### 2.3 Update Sequence Rules

#### First Render (Initial Load)
**MUST** include:
- Complete static structure (`"s"` array)
- All dynamic values (numeric keys)
- Full tree structure for nested constructs
- Range statics (shared item template)
- Metadata (if range has identifiable items)

Example:
```json
{
  "s": ["<div>Hello ", "</div>"],
  "0": "World"
}
```

#### Subsequent Renders (Updates)
**MUST** include:
- ONLY changed dynamic values
- NO static arrays unless structure fingerprint differs from previous render
- Empty object `{}` when no changes detected

Example (only field "0" changed):
```json
{
  "0": "Universe"
}
```

#### New Structure Detection
Include statics when:
1. Field didn't exist in previous render
2. Structure fingerprint changed (statics layout differs)
3. Transition from non-TreeNode to TreeNode (e.g., empty string to nested structure)

### 2.4 Wire Format Optimization

The `PrepareTreeForClient(node, clientHasStatics)` function implements wire format optimization:
- First render (`clientHasStatics=false`): send everything as-is
- Updates (`clientHasStatics=true`): strip statics and fingerprints to reduce payload

Result: updates are typically ~10% the size of full renders since statics are the largest part.

## 3. Template Construct Specifications

### 3.1 Field Access

#### Simple Field: `{{.FieldName}}`
```go
Template: <div>{{.Name}}</div>
Data: {Name: "Alice"}
```

Tree (first render):
```json
{
  "s": ["<div>", "</div>"],
  "0": "Alice"
}
```

Update (name changed):
```json
{
  "0": "Bob"
}
```

#### Nested Field: `{{.User.Name}}`
```go
Template: <div>{{.User.Name}}</div>
Data: {User: {Name: "Alice"}}
```

Tree structure identical to simple field (single dynamic slot).

### 3.2 Conditional Constructs

#### Simple If: `{{if .Show}}...{{end}}`
```go
Template: {{if .Show}}Visible{{end}}
```

Tree (when true):
```json
{
  "s": ["", ""],
  "0": "Visible"
}
```

Tree (when false):
```json
{
  "s": ["", ""],
  "0": ""
}
```

#### If-Else: `{{if .Active}}...{{else}}...{{end}}`
```go
Template: {{if .Active}}ON{{else}}OFF{{end}}
```

Tree (wrapped in dynamic slot):
```json
{
  "s": ["", ""],
  "0": "ON"
}
```

When the condition changes, the implementation sends the branch content. For static-only branches (no dynamics within the branch), the value is wrapped as a TreeNode with statics:
```json
{
  "0": { "s": ["OFF"] }
}
```

This ensures the client receives the new static content for the branch.

#### Else-If Chains
```go
Template: {{if .A}}a{{else if .B}}b{{else}}c{{end}}
```

Evaluated to single result, wrapped as single dynamic.

### 3.3 Range Constructs

#### Basic Range: `{{range .Items}}...{{end}}`
```go
Template: {{range .Items}}<li>{{.}}</li>{{end}}
Data: {Items: ["A", "B", "C"]}
```

Tree (first render):
```json
{
  "s": ["", ""],
  "0": {
    "s": ["<li>", "</li>"],
    "d": [
      {"0": "A"},
      {"0": "B"},
      {"0": "C"}
    ]
  }
}
```

The `"s"` array on the range node contains the **item template statics** shared by all items. This is stored in `RangeData.Statics` on the server and merged into the node-level `"s"` key during JSON marshaling.

#### Range with Keyed Items
```go
Template: {{range .Items}}<div data-key="{{.ID}}">{{.ID}}: {{.Text}}</div>{{end}}
Data: {Items: [{ID: "1", Text: "Hello"}, {ID: "2", Text: "World"}]}
```

Tree (first render):
```json
{
  "s": ["", ""],
  "0": {
    "s": ["<div data-key=\"", "\">", ": ", "</div>"],
    "d": [
      {"0": "1", "1": "1", "2": "Hello"},
      {"0": "2", "1": "2", "2": "World"}
    ],
    "m": {"idKey": "0"}
  }
}
```

The metadata `"m"` field tells the client which dynamic position contains the item's unique identifier (used for matching items across renders).

#### Empty Range
```go
Data: {Items: []}
```

Tree:
```json
{
  "s": ["", ""],
  "0": {
    "s": [""],
    "d": []
  }
}
```

#### Range with Else
```go
Template: {{range .Items}}{{.}}{{else}}No items{{end}}
```

When empty, the else branch becomes simple dynamic content.

### 3.4 Range Operations

Range operations are generated by comparing old and new range data. The system generates the minimal set of operations to transform the old list into the new one.

#### Key Detection

Item keys are detected by scanning the range statics for key attributes. The following attributes are checked in priority order (first match wins):

1. `id="`
2. `data-key="`
3. `key="`
4. `data-lvt-key="`
5. `lvt-key="`
6. `data-id="`
7. `x-key="` (Alpine.js compatibility)
8. `v-key="` (Vue.js compatibility)

If no key attribute is found, position `"0"` is used as the default key.

#### Append Operation
Format: `["a", items, statics]` or `["a", items, statics, metadata]`

Appends items to the end of the list. O(1) on the client. The extended format with `metadata` is used for empty-to-items transitions where the client needs to initialize range state.

Example (empty to items):
```json
["a", [{"0": "1", "1": "First"}], ["<li data-key=\"", "\">", "</li>"], {"idKey": "0"}]
```

Example (append to existing):
```json
["a", [{"0": "3", "1": "Third"}], ["<li data-key=\"", "\">", "</li>"]]
```

#### Prepend Operation
Format: `["p", items, statics]`

Prepends items to the start of the list. O(1) on the client.

Example:
```json
["p", [{"0": "0", "1": "Zeroth"}], ["<li data-key=\"", "\">", "</li>"]]
```

#### Insert Operation
Format: `["i", afterId, data]`

Inserts a single item after the specified item.

- `afterId`: Item ID to insert after

Example:
```json
["i", "item-1", {"0": "item-1.5", "1": "Between"}]
```

#### Remove Operation
Format: `["r", itemId]`

Example:
```json
["r", "item-2"]
```

#### Update Operation
Format: `["u", itemId, changes]`

Only changed fields are included in the changes object.

Example:
```json
["u", "item-1", {"2": "Updated Text"}]
```

#### Reorder Operation
Format: `["o", [itemIds]]`

Specifies the new order of all items by their IDs.

Example:
```json
["o", ["item-3", "item-1", "item-2"]]
```

#### Operation Selection Logic

The system selects operations based on the detected pattern:

1. **Pure reorder** (same items, different order): Single `["o", ...]` operation
2. **All new items at start**: Single `["p", ...]` prepend operation
3. **All new items at end**: Single `["a", ...]` append operation
4. **Empty to items**: Append with statics and metadata `["a", items, statics, metadata]`
5. **Items at specific positions**: Individual `["i", ...]` insert operations
6. **Content + order change**: Update operations followed by `["o", ...]`
7. **Complex patterns** (many keys changing simultaneously): Falls back to full tree replacement

#### Statics in Operations

Append, prepend, and insert operations include statics when the client hasn't seen the range structure before. When the client has the statics cached (structure fingerprint unchanged), statics are stripped from operations.

### 3.5 Variable Context

#### Variable Declaration: `{{$var := .Value}}`
Variables are resolved at compile time and don't create separate dynamics.

#### Root Context: `{{$.Field}}`
Within ranges, `$` accesses root context.

```go
Template: {{range .Items}}{{$.Title}}: {{.}}{{end}}
```

Each item gets title from root context.

## 4. Update Generation Algorithm

### 4.1 Structure Fingerprinting

1. Calculate MD5 hash of static structure only (not dynamic values)
2. Truncate to 64 bits (16 hex characters) for compact representation
3. Cache fingerprint on first access (lazy computation)
4. Compare fingerprints for O(1) structure change detection

### 4.2 Tree Comparison

The main orchestrator `CompareTreesAndGetChangesWithPath()` coordinates the comparison:

```
1. Handle top-level range constructs:
   - Both ranges (matched): Generate differential operations
   - else→range: Return full new tree (structure changed)
   - range→else: Return full new tree (structure changed)

2. Detect structural changes:
   - Compare structure fingerprints via ClientNeedsStatics()
   - If structure changed AND dynamic field keys differ: return full new tree

3. Compare dynamic segments:
   For each dynamic field in newTree:
     - New field: Include with statics (client hasn't seen it)
     - Changed field: Recursively compare (primitives, TreeNodes, ranges)
     - Unchanged field: Skip (not included in update)
```

### 4.3 Range Differential Algorithm

The `GenerateRangeDifferentialOperations()` function generates minimal operations:

```
1. Extract range data (items, statics, metadata) from old and new values
2. Extract item keys using statics-based key detection

3. Check for pure reordering:
   - Same items, different order → return ["o", newKeys]

4. Check for complex insertion patterns:
   - If too many keys changed simultaneously → return empty (triggers full replacement)

5. Generate operations in order:
   a. Removals: ["r", itemId] for items in old but not new
   b. Updates: ["u", itemId, changes] for items with changed fields
   c. Insertions: Pattern-detected (append/prepend/individual insert)

6. Check for combined content + order changes:
   - If same key set but different order → append ["o", newKeys]

7. Strip statics if client has them cached (fingerprint unchanged)
```

## 5. Validation Rules

### 5.1 First Render Validation
- MUST contain `"s"` key with static array
- MUST contain all dynamic slots referenced in template
- Statics array length MUST equal dynamic count + 1
- Range nodes MUST include item statics in `"s"`
- Range nodes with identifiable items MUST include `"m"` metadata

### 5.2 Update Validation
- MUST NOT contain `"s"` for structures with unchanged fingerprints
- MUST contain ONLY changed dynamics
- Range operations MUST be granular (not full list replacement) when possible
- MUST include statics when structure fingerprint differs from previous render

### 5.3 Structural Invariants
- Numeric keys MUST be sequential from "0"
- No gaps in numeric key sequence
- `"d"` key exclusive to range constructs
- Each range item MUST have a unique identifier (detected from statics or at position "0")
- Structure fingerprints MUST be deterministic (same structure produces same fingerprint)

## 6. Performance Requirements

### 6.1 Update Size
- Updates SHOULD be < 10% size of full render
- Range operations SHOULD affect only changed items
- Empty updates MUST return `{}`

### 6.2 Processing Time
- Tree generation: O(n) where n = template size
- Diff computation: O(m) where m = changed nodes
- Fingerprint comparison: O(1) after initial computation
- Fingerprint computation: O(n) where n = static structure size (computed once, cached)

## 7. Error Handling

### 7.1 Malformed Templates
- Invalid syntax: Return error, no partial tree
- Missing data: Use zero values, continue generation
- Unsupported constructs: Fall back to HTML-based tree (see fallback addendum)

### 7.2 Update Failures
- Network errors: Client retains last valid state
- Invalid updates: Client rejects, requests full render

### 7.3 Range Operation Fallback
- Complex insertion patterns (many keys changing simultaneously): Return empty operations, triggering full tree replacement by the caller
- This avoids generating partial operations (e.g., removals without matching insertions)

## 8. Examples

### 8.1 Complete User Journey

#### Step 1: Initial Visit (First Render)
```json
{
  "s": ["<div>", "<ul>", "</ul>", "</div>"],
  "0": "Todo App",
  "1": {
    "s": [""],
    "d": []
  }
}
```

#### Step 2: Add First Item (Empty → Items Transition)
Uses append with statics and metadata since client needs to initialize range state:
```json
{
  "1": {
    "d": [
      ["a", [{"0": "item-1", "1": "First task"}], ["<li data-key=\"", "\">", "</li>"], {"idKey": "0"}]
    ]
  }
}
```

#### Step 3: Add Second Item (Append)
```json
{
  "1": {
    "d": [
      ["a", [{"0": "item-2", "1": "Second task"}], ["<li data-key=\"", "\">", "</li>"]]
    ]
  }
}
```

#### Step 4: Update First Item
```json
{
  "1": {
    "d": [
      ["u", "item-1", {"1": "Updated first task"}]
    ]
  }
}
```

#### Step 5: Remove Second Item
```json
{
  "1": {
    "d": [
      ["r", "item-2"]
    ]
  }
}
```

## 9. Compliance Testing

### 9.1 Required Tests
1. First render includes statics and metadata
2. Updates exclude unchanged content
3. Range operations are granular
4. Empty-to-content transition uses append with statics
5. Content-to-empty transition clears range
6. Structure fingerprint changes trigger statics re-send
7. Conditional branch changes send new branch content

### 9.2 Fuzz Testing Requirements
- Minimum 1M iterations without violations
- Random user journey sequences
- All construct types covered
- Edge cases (empty, null, large lists)

## 10. Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-03-07 | Initial release (Final). TreeNode as typed struct. Structure fingerprinting. Append/prepend/insert operations. Metadata field. Wire format optimization. |

## Appendix A: JSON Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "s": {
      "type": "array",
      "items": { "type": "string" }
    },
    "d": {
      "type": "array"
    },
    "f": {
      "type": "string",
      "description": "Structure fingerprint (16 hex chars)"
    },
    "m": {
      "type": "object",
      "properties": {
        "idKey": { "type": "string" }
      },
      "description": "Metadata for item key identification"
    }
  },
  "patternProperties": {
    "^[0-9]+$": {}
  },
  "additionalProperties": false
}
```

## Appendix B: Range Operation Schema

```json
{
  "oneOf": [
    {
      "description": "Append operation (items at end or empty→items transition)",
      "type": "array",
      "items": [
        { "const": "a" },
        { "type": "array", "description": "Items to append" },
        { "description": "Statics (item template)" },
        { "type": "object", "description": "Metadata (optional, for empty→items)" }
      ],
      "minItems": 3,
      "maxItems": 4
    },
    {
      "description": "Prepend operation (items at start)",
      "type": "array",
      "items": [
        { "const": "p" },
        { "type": "array", "description": "Items to prepend" },
        { "description": "Statics (item template)" }
      ],
      "minItems": 3,
      "maxItems": 3
    },
    {
      "description": "Insert operation (item at specific position)",
      "type": "array",
      "items": [
        { "const": "i" },
        { "type": "string", "description": "ID of item to insert after" },
        { "type": "object", "description": "Item data" }
      ],
      "minItems": 3,
      "maxItems": 3
    },
    {
      "description": "Remove operation",
      "type": "array",
      "items": [
        { "const": "r" },
        { "type": "string" }
      ],
      "minItems": 2,
      "maxItems": 2
    },
    {
      "description": "Update operation",
      "type": "array",
      "items": [
        { "const": "u" },
        { "type": "string" },
        { "type": "object" }
      ],
      "minItems": 3,
      "maxItems": 3
    },
    {
      "description": "Reorder operation",
      "type": "array",
      "items": [
        { "const": "o" },
        { "type": "array", "items": { "type": "string" } }
      ],
      "minItems": 2,
      "maxItems": 2
    }
  ]
}
```
