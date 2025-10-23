# Bug Report: Validation Error Display - Conditional Rendering Issue

## Summary
Template conditional blocks (like `{{if .lvt.HasError "field"}}...{{end}}`) are not rendering correctly in tree updates. Instead of rendering the HTML content inside the conditional, the tree update contains an object that gets rendered as `[object Object]` in the browser.

## Reproduction
1. Create a form with validation error display:
```html
<input name="title" required>
{{if .lvt.HasError "title"}}
  <small>{{.lvt.Error "title"}}</small>
{{end}}
```

2. Submit the form with empty fields (bypassing HTML5 validation)
3. Server-side validation fails and returns errors
4. Client receives tree update and applies it
5. Result: `[object Object]` appears in the DOM where the `<small>` tag should be

## Investigation Findings

### Client-Side Evidence
When inspecting the rendered HTML after a failed validation, the form shows:
```html
<input name="title" required="">
[object Object]
</input>
```

The `[object Object]` text indicates that JavaScript is calling `.toString()` on an object, which typically happens when morphdom tries to insert an object as a text node.

### Execution Flow
1. ✅ Form submission works correctly (verified with debug flags)
2. ✅ Server receives form data
3. ✅ Server-side validation runs and fails (empty strings fail `required` validation)
4. ✅ Validation errors are set in state (`state.setError()`)
5. ✅ Errors are passed to `ExecuteUpdates()`
6. ✅ Template is executed with `.lvt` context containing errors
7. ❌ Tree generation creates object nodes instead of HTML strings for conditional content
8. ❌ Client receives tree with objects
9. ❌ morphdom inserts `[object Object]` as text

### Root Cause
The issue is in how the tree generation code handles conditional template constructs. When a conditional evaluates to true and should render its content:

1. The condition `{{if .lvt.HasError "title"}}` evaluates to true
2. The template engine should render the `<small>` tag HTML
3. Instead, the tree generation code is creating an object representation
4. This object is serialized to JSON and sent to the client
5. The client's morphdom receives this object and calls `.toString()` on it
6. Result: `[object Object]` in the DOM

### Code Locations

**Server-Side (Correct)**:
- `mount.go:575-587` - Error processing and `state.setError()` ✅
- `mount.go:339-362` - Errors passed to `ExecuteUpdates()` ✅
- `template.go:616-644` - `ExecuteUpdates()` calls tree generation ✅
- `template.go:686-733` - `.lvt` context created with errors ✅

**Tree Generation (Bug Location)**:
- `template.go:646-684` - `generateTreeInternalWithErrors()`
- Tree generation for conditional constructs needs investigation
- Likely in full_tree_parser.go or tree_ast.go

**Client-Side (Receives Bad Data)**:
- `livetemplate-client.ts:529-546` - Receives tree update ✅
- `livetemplate-client.ts:545` - Calls `updateDOM()` with tree ✅
- morphdom attempts to render objects as text nodes ❌

## Testing
- Validation logic itself works: `go-playground/validator` correctly rejects empty strings with `required` tag
- HTML5 validation bypass works: Setting `form.noValidate = true` allows form submission
- WebSocket communication works: Messages are sent and received
- Form submission handler works: All debug flags confirm execution

## Impact
- All forms with validation error display are affected
- Validation errors are processed but not displayed to users
- Users cannot see what fields need correction

## Affected Tests
- `TestCompleteWorkflow_BlogApp/Validation_Errors` - FAILING
- `TestTutorialE2E/Validation_Errors` - FAILING
- Any form that uses `.lvt.HasError` and `.lvt.Error` for validation display

## Required Fix
The core library tree generation code needs to be fixed to properly serialize conditional block content as HTML strings instead of objects. This requires changes to how conditional constructs are handled during tree generation and serialization.

Specific areas to investigate:
1. How `ConditionalConstruct` is compiled and hydrated
2. How conditional branches are represented in the tree
3. How tree nodes are serialized to JSON
4. Whether conditional content needs special handling for proper HTML rendering

## Workaround
Until the core library fix is implemented:
1. Skip validation error display tests
2. Or use a different approach for error display that doesn't rely on template conditionals
3. Or inject error HTML directly without using `.lvt.HasError` conditionals

## Date
2025-10-23

## Branch
feat/multi-session-isolation

---

# Related Issue: Range Construct Statics Not Sent on Empty→First Item Transition

## Summary
When transitioning from an empty list to adding the first item in a range construct, the static HTML structure is not being sent to the client, resulting in empty tbody elements even though data is being added.

## Reproduction
1. Render a template with an empty range: `{{range .Items}}...{{end}}`
2. Add first item to the list
3. Client receives update but tbody remains empty
4. Subsequent items also don't appear

## Investigation Findings

### Root Cause
The issue is in `template.go:isRangeConstruct()` and how it determines whether to strip statics from range updates.

When transitioning from empty list `{"d": [], "s": [...]}` to first item:
1. System recognizes both old and new as range constructs
2. Calls `generateRangeDifferentialOperations()` with `stripStatics=true`
3. Generates append operation: `["a", {...item...}]`
4. Strips statics from the operation, assuming client already has them
5. Client receives item data without statics, cannot render

### Initial Fix Attempt
Changed `isRangeConstruct()` to return `false` when `d` array is empty:
```go
if dArray, ok := d.([]interface{}); ok {
    return len(dArray) > 0  // Only true if has items
}
```

**Result**: 10/11 E2E tests passed, but introduced new bug where empty ranges sent as raw objects `[object Object]`

### Secondary Issue
When a range becomes empty (items → no results from search), the range object `{"d": [], "s": [""]}` is sent directly to client, which renders it as `[object Object]`.

### Attempted Fixes
1. **Check oldItems length in stripStatics logic** - Broke all tests
2. **Send empty string for empty ranges in fallback** - Broke all tests
3. **Revert to baseline** - Back to original 5/11 tests passing

### Test Results
- Baseline (no fixes): 5/11 TodosE2E tests pass
- With first fix only: 10/11 tests pass (Search_Functionality fails with `[object Object]`)
- With attempted secondary fixes: 5/11 tests pass (broke existing functionality)

## Code Locations

**Core Functions**:
- `template.go:1429-1449` - `isRangeConstruct()` determines if value is range
- `template.go:1732-1939` - `generateRangeDifferentialOperations()` creates diff ops
- `template.go:1932-1936` - Statics stripping logic
- `template.go:1047-1075` - Range match handling and fallback

**Test File**:
- `examples/todos/todos_e2e_test.go` - E2E tests showing the failures

## Proper Fix Needed
The fix requires a more nuanced approach:
1. Track whether client has seen the statics for a specific range instance
2. Don't assume client has statics just because old value is a range
3. Handle empty→first-item transition specially
4. Ensure empty ranges are rendered correctly (not as objects)

This is a complex state management issue that requires careful consideration of:
- When client receives statics
- How to track what client knows
- How to handle all transition states (empty↔items, items↔different-items)

## Affected Tests
All TodosE2E tests that involve:
- Adding todos (Add_First_Todo, Add_Second_Todo, etc.)
- Searching with no results (Search_Functionality)
- Pagination (Pagination_Functionality)

## Root Cause - CONFIRMED

The bug IS in the core library. The issue occurs in `template.go:isRangeConstruct()`:

```go
shouldStripStatics := isRangeConstruct(oldValue)
```

When `oldValue` is `{"d": [], "s": [""]}` (empty range from initial WebSocket tree), the function returns `true` because both keys exist. This causes statics to be stripped from the append operation:

```json
// What gets sent (WRONG - missing statics):
{"0": [["a", [{"0": "todo-1", "1": "First Todo"}]]]}

// What should be sent (with statics):
{"0": [["a", [{"s": ["<tr data-key=\"", "\"><td>", "</td></tr>"], "0": "todo-1", "1": "First Todo"}]]]}
```

Without statics, the client cannot render the item.

## Why Only Todos Fails

The examples have different template patterns:
- **Counter**: No ranges involved
- **Chat**: Uses conditional wrapper around range that prevents empty range scenario
- **Todos**: Range can be empty (no conditional wrapper), triggers the bug

### Chat Template Pattern (AVOIDS BUG)
```html
{{if eq (len .Messages) 0}}
  <div class="empty-state">No messages yet...</div>
{{else}}
  {{range .Messages}}
    <div class="message">...</div>
  {{end}}
{{end}}
```

When first message is added, this switches from `if` branch to `else` branch - a **conditional branch switch**, NOT a range append operation. The range is never empty!

### Todos Template Pattern (TRIGGERS BUG)
```html
<table>
  <tbody>
    {{range .PaginatedTodos}}
      <tr data-key="{{.ID}}">...</tr>
    {{end}}
  </tbody>
</table>
```

The `<tbody>` always exists, and the range starts empty. When first item is added, it's a **range append operation** on an empty range - this triggers the bug!

### Todos Flow (BUG MANIFESTS)
1. Initial page load: HTML sent with empty tbody
2. WebSocket connects: Sends initial tree with empty range `{"d": [], "s": [""]}`
3. Add first todo: Generates append without statics (BUG!)
4. Client can't render - tbody stays empty

## Status
Root cause definitively identified. Ready for fix implementation.
