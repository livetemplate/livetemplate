# Enhanced Range Operations with insertafter/insertbefore

## Overview

StateTemplate now supports enhanced range operations with `insertafter` and `insertbefore` actions, making it perfect for sorting, reordering, and dynamic list management use cases. The enhancements maintain backward compatibility while adding powerful new capabilities.

## Key Enhancements

### 1. New RealtimeUpdate Fields

```go
type RealtimeUpdate struct {
    FragmentID   string `json:"fragment_id"`             // The ID of the div/element to update
    HTML         string `json:"html"`                    // The new HTML content for that fragment
    Action       string `json:"action"`                  // "replace", "append", "prepend", "remove", "insertafter", "insertbefore"
    ReferenceID  string `json:"reference_id,omitempty"`  // For insertafter/insertbefore: ID of the reference element
    ItemKey      string `json:"item_key,omitempty"`      // For range items: unique key for stable identification
}
```

### 2. Stable Fragment IDs

Range items now use **stable, key-based fragment IDs** instead of index-based ones:

- **Before**: `container-item-0`, `container-item-1`, `container-item-2` (breaks on reorder)
- **After**: `container-item-/task/1`, `container-item-task-abc`, `container-item-home` (stable across reorders)

### 3. New Actions Supported

| Action         | Description                             | Use Case                     |
| -------------- | --------------------------------------- | ---------------------------- |
| `insertafter`  | Insert element after reference element  | Precise positioning, sorting |
| `insertbefore` | Insert element before reference element | Precise positioning, sorting |
| `prepend`      | Insert at beginning of container        | Move to first position       |
| `append`       | Insert at end of container              | Move to last position        |
| `remove`       | Remove element                          | Delete items                 |
| `replace`      | Replace element content                 | Update existing items        |

## Use Cases

### ✅ Sorting Lists

```go
// Before: [Task A (Priority 3), Task B (Priority 1), Task C (Priority 2)]
// After:  [Task B (Priority 1), Task C (Priority 2), Task A (Priority 3)]

// Generated updates:
// 1. insertafter: Move Task B after position reference
// 2. insertafter: Move Task C after Task B
// 3. insertafter: Move Task A after Task C
```

### ✅ Dynamic Insertion

```go
// Insert new item at specific position
// Result: insertafter action with ReferenceID pointing to previous item
```

### ✅ Reordering/Drag & Drop

```go
// Move item from position 3 to position 1
// Result: prepend action to move to beginning, or insertafter with precise positioning
```

### ✅ Live Updates with Minimal DOM Changes

Instead of replacing entire lists, only specific items are moved/updated, resulting in:

- Better performance
- Preserved scroll positions
- Smoother animations
- Maintained focus states

## Example Usage

### Template Setup

```go
template := `
<ul class="task-list">
    {{range .Tasks}}
    <li data-id="{{.ID}}" class="task-item">
        <a href="{{.URL}}">{{.Title}}</a>
        <span class="priority">Priority: {{.Priority}}</span>
    </li>
    {{end}}
</ul>`
```

### Generated HTML with Stable IDs

```html
<ul id="abc123" class="task-list">
  <li id="abc123-item-task-1" data-id="task-1" class="task-item">
    <a href="/task/1">Fix critical bug</a>
    <span class="priority">Priority: 1</span>
  </li>
  <li id="abc123-item-task-2" data-id="task-2" class="task-item">
    <a href="/task/2">Write docs</a>
    <span class="priority">Priority: 3</span>
  </li>
</ul>
```

### Sorting Updates

When sorting by priority (ascending), you get:

```json
[
  {
    "fragment_id": "abc123-item-task-2",
    "action": "insertafter",
    "reference_id": "abc123-item-task-1",
    "item_key": "task-2",
    "html": ""
  }
]
```

## Benefits

### 🚀 Performance

- **Granular updates**: Only changed items are updated, not entire lists
- **Minimal DOM manipulation**: Precise insertafter/insertbefore operations
- **Efficient diffing**: Smart comparison of old vs new item arrays

### 🎯 Precision

- **Stable IDs**: Fragment IDs remain consistent across reorders
- **Reference-based positioning**: Exact placement using ReferenceID
- **Key-based identification**: Items tracked by unique keys, not indices

### 🔄 Flexibility

- **Multiple update types**: Supports all common list operations
- **Backwards compatible**: Existing code continues to work
- **Smart fallbacks**: Automatic fallback to full replacement when needed

### 🛠️ Developer Experience

- **Unified API**: Same RealtimeUpdate structure for all operations
- **Rich metadata**: ItemKey and ReferenceID provide context
- **Easy debugging**: Clear action types and reference relationships

## Migration Guide

### Existing Code (No Changes Required)

```go
// This continues to work unchanged
renderer.SendUpdate(newData)
```

### Enhanced Functionality (Automatic)

```go
// Now automatically gets:
// ✅ Stable fragment IDs
// ✅ Granular range updates
// ✅ insertafter/insertbefore actions
// ✅ Smart performance optimizations
```

## Testing

Run the enhanced range demo:

```bash
go run cmd/enhanced-range-demo/main.go
```

Run the sorting tests:

```bash
go test -v -run TestRangeSortingWithInsertActions
go test -v -run TestRangeInsertOperations
go test -v -run TestStableFragmentIDs
```

## Summary

The enhanced range operations provide a powerful foundation for building dynamic, responsive web applications with smooth list manipulations. Whether you're building:

- 📋 **Todo lists** with drag & drop reordering
- 📊 **Data tables** with column sorting
- 🗂️ **File managers** with dynamic folder contents
- 📱 **Social feeds** with live updates
- 🛒 **Shopping carts** with item management

StateTemplate now provides the granular control and performance you need, while maintaining the simplicity of the original fragment-based approach.
