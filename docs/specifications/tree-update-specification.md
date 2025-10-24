# LiveTemplate Tree Update Specification

Version: 1.0.0
Last Updated: 2025-10-22
Status: Draft

## 1. Introduction

This document provides the formal specification for LiveTemplate's tree-based update generation system. It defines the exact structure, rules, and behaviors that all LiveTemplate implementations must follow to ensure correct and efficient template updates.

## 2. Core Concepts

### 2.1 Tree Node Structure

A tree node is a JSON object that represents the separation of static and dynamic content in templates.

**Go Representation (Server-side):**
```go
type TreeNode map[string]interface{}
// Keys:
// "s": []interface{} - Static content array (template structure)
// "0", "1", etc: Dynamic value positions
// "d": []interface{} - Range data for list items
// "f": string - Fingerprint for change detection
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

  // Fingerprint for change detection (internal only)
  "f"?: string;
}
```

#### Rules:
1. The `"s"` key MUST contain an array of strings representing static HTML/text segments
2. Numeric string keys (`"0"`, `"1"`, etc.) MUST be used for dynamic content slots
3. The `"d"` key MUST be used exclusively for range construct data
4. Keys MUST be ordered sequentially starting from `"0"`
5. Empty dynamics MAY be represented as empty strings `""`

### 2.2 Update Sequence Rules

#### First Render (Initial Load)
**MUST** include:
- Complete static structure (`"s"` array)
- All dynamic values (numeric keys)
- Full tree structure for nested constructs

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
- NO static arrays unless structure is new to client
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
2. Field type changed (e.g., empty state → range construct)
3. Client has never seen this specific structure

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
  "0": "ON"  // or "OFF" depending on condition
}
```

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

When empty, else branch becomes simple dynamic.

### 3.4 Range Operations

#### Insert Operation
Format: `["i", afterId, position, data]`

- `afterId`: Item ID to insert after (null for start)
- `position`: "start", "end", or numeric position
- `data`: New item data (WITHOUT statics)

Example:
```json
["i", null, "start", {"1": "item-4", "3": "New Task"}]
```

#### Remove Operation
Format: `["r", itemId]`

Example:
```json
["r", "item-2"]
```

#### Update Operation
Format: `["u", itemId, updates]`

Example:
```json
["u", "item-1", {"3": "Updated Text"}]
```

#### Reorder Operation
Format: `["o", [itemIds]]`

Example:
```json
["o", ["item-3", "item-1", "item-2"]]
```

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

### 4.1 Fingerprinting
1. Calculate MD5 hash of tree (statics + dynamics)
2. Compare fingerprints for change detection
3. Skip update if fingerprints match

### 4.2 Tree Comparison
```go
func compareTreesAndGetChanges(oldTree, newTree TreeNode) TreeNode {
    changes := make(TreeNode)

    for key, newValue := range newTree {
        // Skip statics and fingerprint
        if key == "s" || key == "f" {
            continue
        }

        oldValue, exists := oldTree[key]
        if !exists {
            // New field - include with statics if client hasn't seen it
            if clientHasStructure(key) {
                changes[key] = stripStatics(newValue)
            } else {
                changes[key] = newValue // Include statics
            }
        } else if !reflect.DeepEqual(oldValue, newValue) {
            // Changed field
            if isRangeConstruct(newValue) {
                changes[key] = generateRangeOps(oldValue, newValue)
            } else {
                changes[key] = stripStatics(newValue)
            }
        }
    }

    return changes
}
```

### 4.3 Range Differential Algorithm
```go
func generateRangeOps(oldItems, newItems []interface{}) []interface{} {
    var ops []interface{}

    // Check for pure reordering
    if isPureReorder(oldItems, newItems) {
        return []interface{}{
            []interface{}{"o", extractKeys(newItems)},
        }
    }

    oldKeys := extractKeys(oldItems)
    newKeys := extractKeys(newItems)

    // Find removed items
    for _, key := range oldKeys {
        if !contains(newKeys, key) {
            ops = append(ops, []interface{}{"r", key})
        }
    }

    // Find added/updated items
    for i, key := range newKeys {
        if !contains(oldKeys, key) {
            // Insert operation
            var afterID interface{}
            if i > 0 {
                afterID = newKeys[i-1]
            }
            position := "start"
            if i > 0 {
                position = fmt.Sprintf("%d", i)
            }
            ops = append(ops, []interface{}{"i", afterID, position, stripStatics(newItems[i])})
        } else if itemChanged(oldItems[key], newItems[key]) {
            // Update operation
            changes := getItemChanges(oldItems[key], newItems[key])
            ops = append(ops, []interface{}{"u", key, changes})
        }
    }

    return ops
}
```

## 5. Validation Rules

### 5.1 First Render Validation
- MUST contain `"s"` key with static array
- MUST contain all dynamic slots referenced in template
- Statics array length MUST equal dynamic count + 1

### 5.2 Update Validation
- MUST NOT contain `"s"` for existing structures
- MUST contain ONLY changed dynamics
- Range operations MUST be granular (not full list)

### 5.3 Structural Invariants
- Numeric keys MUST be sequential from "0"
- No gaps in numeric key sequence
- `"d"` key exclusive to range constructs
- Each range item MUST have unique identifier

## 6. Performance Requirements

### 6.1 Update Size
- Updates SHOULD be < 10% size of full render
- Range operations SHOULD affect only changed items
- Empty updates MUST return `{}`

### 6.2 Processing Time
- Tree generation: O(n) where n = template size
- Diff computation: O(m) where m = changed nodes
- Fingerprint: O(1) for comparison

## 7. Error Handling

### 7.1 Malformed Templates
- Invalid syntax: Return error, no partial tree
- Missing data: Use zero values, continue generation

### 7.2 Update Failures
- Network errors: Client retains last valid state
- Invalid updates: Client rejects, requests full render

## 8. Examples

### 8.1 Complete User Journey

#### Step 1: Initial Visit
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

#### Step 2: Add First Item
```json
{
  "1": {
    "0": [
      ["i", null, "start", {"0": "item-1", "1": "First task"}]
    ]
  }
}
```

#### Step 3: Add Second Item
```json
{
  "1": {
    "0": [
      ["i", "item-1", 1, {"0": "item-2", "1": "Second task"}]
    ]
  }
}
```

#### Step 4: Update First Item
```json
{
  "1": {
    "0": [
      ["u", "item-1", {"1": "Updated first task"}]
    ]
  }
}
```

#### Step 5: Remove Second Item
```json
{
  "1": {
    "0": [
      ["r", "item-2"]
    ]
  }
}
```

## 9. Compliance Testing

### 9.1 Required Tests
1. First render includes statics
2. Updates exclude unchanged content
3. Range operations are granular
4. Empty → Content transition
5. Content → Empty transition
6. Rapid successive updates

### 9.2 Fuzz Testing Requirements
- Minimum 1M iterations without violations
- Random user journey sequences
- All construct types covered
- Edge cases (empty, null, large lists)

## 10. Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-10-22 | Initial specification |

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
      "type": "string"
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
      "description": "Insert operation",
      "type": "array",
      "items": [
        { "const": "i" },
        { "type": ["string", "null"] },
        { "type": ["string", "number"] },
        { "type": "object" }
      ],
      "minItems": 4,
      "maxItems": 4
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
      "description": "Order operation",
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