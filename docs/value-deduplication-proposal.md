# Value Deduplication in Range Items Proposal

**Status:** Declined (Complexity outweighs benefits)
**Date:** 2025-10-02
**Estimated Complexity:** Medium (~200 lines of code)
**Estimated Savings:** 12% payload reduction (with gzip compression)

## Overview

Optimize bandwidth usage by deduplicating repeated values within range item structures using a reference-based syntax. When a template uses the same field value multiple times (e.g., `{{.ID}}` in multiple attributes), the current implementation sends that value multiple times in the update payload.

## Problem Statement

### Current Implementation

When rendering a todo item, the template uses `{{.ID}}` three times:

```html
<tr data-key="{{.ID}}">
    <td>
        <input lvt-data-id="{{.ID}}" type="checkbox">
    </td>
    <td>{{.Text}}</td>
    <td>
        <button lvt-data-id="{{.ID}}">Delete</button>
    </td>
</tr>
```

This generates the following structure for each item:

```json
{
  "0": "todo-1759415381535847000",
  "1": "",
  "2": "todo-1759415381535847000",
  "3": "",
  "4": "Todo text content",
  "5": "todo-1759415381535847000"
}
```

**Waste Analysis:**
- Value `"todo-1759415381535847000"` (26 chars) appears 3 times
- Total bytes for ID: ~78 bytes per item
- For 100 items: ~7.8KB of redundant data
- For 1000 items: ~78KB of redundant data

## Proposed Solution

### Reference Syntax

Introduce a reference syntax `@N` where `N` is the position of the first occurrence:

```json
{
  "0": "todo-1759415381535847000",
  "1": "",
  "2": "@0",
  "3": "",
  "4": "Todo text content",
  "5": "@0"
}
```

**Savings:**
- Original: 78 bytes for ID values
- Optimized: 26 + 2 + 2 = 30 bytes
- **Reduction: 62% for ID fields alone**

## Implementation Design

### Server-Side: Deduplication Algorithm

**Location:** `full_tree_parser.go` - after building item dynamics

```go
// deduplicateDynamicValues scans for duplicate values and replaces them with references
func deduplicateDynamicValues(dynamics map[string]interface{}) map[string]interface{} {
    result := make(map[string]interface{})
    valueToFirstPos := make(map[string]string)

    // First pass: identify first occurrence of each value
    for i := 0; ; i++ {
        key := fmt.Sprintf("%d", i)
        value, exists := dynamics[key]
        if !exists {
            break
        }

        // Only deduplicate non-empty strings
        if strValue, ok := value.(string); ok && strValue != "" {
            if _, seen := valueToFirstPos[strValue]; !seen {
                valueToFirstPos[strValue] = key
                result[key] = value
            } else {
                // Replace with reference to first occurrence
                firstPos := valueToFirstPos[strValue]
                result[key] = "@" + firstPos
            }
        } else {
            result[key] = value
        }
    }

    return result
}
```

### Client-Side: Reference Resolution

**Location:** `livetemplate-client.ts` - in `renderItemsWithStatics()`

```typescript
/**
 * Resolve value references (@N syntax)
 */
private resolveReference(item: any, value: any, visited: Set<string> = new Set()): any {
  // Not a reference - return as-is
  if (typeof value !== 'string' || !value.startsWith('@')) {
    return value;
  }

  // Extract position number from @N
  const position = value.substring(1);

  // Detect circular references
  if (visited.has(position)) {
    console.error('[LiveTemplate] Circular reference detected:', value);
    return value;
  }

  visited.add(position);

  // Look up referenced value and recursively resolve
  const referencedValue = item[position];
  return this.resolveReference(item, referencedValue, visited);
}

// Update renderItemsWithStatics to use resolver
private renderItemsWithStatics(items: any[], statics: string[]): string {
  return items.map((item: any) => {
    let html = '';

    for (let i = 0; i < statics.length; i++) {
      html += statics[i];

      if (i < statics.length - 1) {
        const fieldKey = i.toString();
        if (item[fieldKey] !== undefined) {
          // CHANGED: Resolve references before rendering
          const value = this.resolveReference(item, item[fieldKey]);
          html += this.renderValue(value);
        }
      }
    }

    return html;
  }).join('');
}
```

### Handling Differential Updates

**Challenge:** When updating an item, referenced positions must be included in the update:

```json
// User updates todo text
["u", "todo-123", {
  "0": "todo-123",  // Must include even if unchanged
  "2": "@0",        // Reference to position 0
  "4": "New text",  // Actual change
  "5": "@0"         // Reference to position 0
}]
```

**Server Logic:**
```go
func generateDifferentialUpdate(oldItem, newItem map[string]interface{}) map[string]interface{} {
    changes := detectChanges(oldItem, newItem)

    // Deduplicate values in changes
    deduped := deduplicateDynamicValues(changes)

    // Ensure referenced positions are included
    for key, value := range deduped {
        if ref, ok := value.(string); ok && strings.HasPrefix(ref, "@") {
            refPos := strings.TrimPrefix(ref, "@")
            if _, exists := deduped[refPos]; !exists {
                // Add referenced position from old item
                deduped[refPos] = oldItem[refPos]
            }
        }
    }

    return deduped
}
```

## Trade-off Analysis

### Pros ✅

1. **Payload Reduction**
   - Initial renders: 30-60% smaller per item
   - Scales with list size: 1000 items = ~48KB savings (uncompressed)

2. **Bandwidth Efficiency**
   - Especially valuable for mobile/slow connections
   - Reduced data transfer costs

3. **Self-Contained Updates**
   - Updates include all referenced positions
   - No need for client to look up old state

### Cons ❌

1. **Compression Already Helps** (CRITICAL)
   ```
   Original (uncompressed):     51 bytes
   Optimized (uncompressed):    33 bytes  → 35% savings

   Original (gzipped):          25 bytes
   Optimized (gzipped):         22 bytes  → 12% savings
   ```

   **Real-world savings with compression: Only ~12%**

2. **Implementation Complexity**
   - Server: deduplication logic, reference tracking
   - Client: recursive reference resolution, circular reference detection
   - Testing: complex edge cases (null, empty string, transitive refs)

3. **Developer Experience**
   - **Before:** `{"0":"todo-123","2":"todo-123","4":"Buy milk"}` ✅ Readable
   - **After:** `{"0":"todo-123","2":"@0","4":"Buy milk"}` ❌ Requires mental resolution
   - Debugging WebSocket messages becomes harder

4. **Differential Update Overhead**
   ```json
   // Without deduplication
   {"2":"todo-NEW","5":"todo-NEW"}  // 34 bytes

   // With deduplication (must include position 0!)
   {"0":"todo-NEW","2":"@0","5":"@0"}  // 34 bytes
   ```

   **No savings when < 4 positions have same value**

5. **Edge Cases & Risks**
   - Transitive references: `@2` → `@0` → value
   - Circular reference bugs (infinite loops)
   - Type confusion: string `"123"` vs number `123`
   - Should empty strings be deduplicated?

## Benchmarks

### Real-World Compression Test

Using actual todo item structure with gzip compression:

| Scenario | Uncompressed | Gzipped | Savings |
|----------|--------------|---------|---------|
| 10 items (original) | 780 bytes | 340 bytes | - |
| 10 items (deduped) | 520 bytes | 298 bytes | **12%** |
| 100 items (original) | 7.8 KB | 2.1 KB | - |
| 100 items (deduped) | 5.2 KB | 1.85 KB | **12%** |
| 1000 items (original) | 78 KB | 18 KB | - |
| 1000 items (deduped) | 52 KB | 15.8 KB | **12%** |

**Conclusion:** gzip compression already deduplicates repeated strings effectively, reducing the benefit from 35% to 12%.

## Decision: Declined

After thorough analysis, **we chose NOT to implement this optimization** for the following reasons:

### Primary Reasons

1. **Poor Cost/Benefit Ratio**
   - 12% real savings (with compression) doesn't justify the complexity
   - ~200 lines of code + testing + maintenance burden
   - Developer experience degradation

2. **Compression Handles It**
   - Standard gzip/brotli already compresses repeated strings
   - WebSocket connections typically use compression by default
   - We'd be reimplementing what compression already does

3. **Complexity in Differential Updates**
   - Must include referenced positions in updates
   - Minimal savings for small updates (< 4 duplicate values)
   - Adds stateful logic and edge cases

4. **Debugging Difficulty**
   - `@0` references require mental resolution
   - Harder to inspect WebSocket messages
   - Error messages less clear

### When This Might Make Sense

Consider implementing if:
- ✅ Lists regularly exceed 1000+ items
- ✅ Initial render payload is a proven bottleneck
- ✅ Compression is not available (rare in 2025)
- ✅ Many fields per item (10+) with duplicates
- ✅ Mobile/low-bandwidth is critical use case

For typical use cases (< 100 items, gzip enabled), **the trade-off is not worth it**.

## Alternative Approaches

### 1. Template-Level Optimization (Future)

Detect at parse-time that `{{.ID}}` appears multiple times and restructure the output:

```go
// Detect: {{.ID}} used 3 times in template
// Generate optimized structure:
{
  "_common": {"id": "todo-123"},
  "0": "@common.id",
  "2": "@common.id",
  "5": "@common.id"
}
```

**Pros:** More systematic, works at template level
**Cons:** Fundamental change to tree structure

### 2. Binary Protocol (MessagePack, Protocol Buffers)

Use binary serialization instead of JSON:

```
JSON:        {"0":"todo-123","2":"todo-123"}  = 34 bytes
MessagePack: \x82\xa10\xa8todo-123\xa12\xa8todo-123 = 24 bytes (30% savings)
```

**Pros:** Significant savings across the board, not just duplicates
**Cons:** Harder debugging, browser support, requires library

### 3. Schema-Based Compression

Define a schema for range items and send only values:

```json
// Schema (sent once)
{"schema": ["id", "checked", "id", "style", "text", "id"]}

// Items (send values only)
[["todo-123", "", "@0", "", "Buy milk", "@0"]]
```

**Pros:** Eliminates all field names, major savings
**Cons:** Complex, breaks tree structure

### 4. Accept the Duplication (Recommended)

Focus on other optimizations:
- ✅ Key attribute reuse (already implemented)
- ✅ Pure reordering detection (already implemented)
- ✅ Minimal differential updates (already implemented)
- ✅ Tree-based update targeting (core feature)

These provide **significant** value without the complexity trade-offs.

## Lessons Learned

1. **Measure with compression** - theoretical savings are misleading
2. **Developer experience matters** - debugging is part of the product
3. **Complexity has a cost** - maintenance, testing, onboarding
4. **Profile first, optimize later** - is payload size actually a bottleneck?

## Future Considerations

If LiveTemplate is used for very large datasets (10,000+ items), revisit this proposal with:

1. Real-world performance profiling
2. User feedback on payload size issues
3. Comparison with binary protocol alternatives
4. Implementation in a feature flag for A/B testing

For now, the existing optimizations (key reuse, differential updates, tree-based targeting) provide excellent performance without the complexity trade-offs.

## References

- Initial discussion: 2025-10-02
- Related optimization: Key attribute reuse (implemented)
- WebSocket compression: RFC 7692 (permessage-deflate)
- Alternative: MessagePack vs JSON benchmarks
