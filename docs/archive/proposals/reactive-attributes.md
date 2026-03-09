# Client Reactive Attributes

**Status:** Proposed
**Version:** 1.0
**Date:** 2025-12-01

---

## TL;DR

**Problem:** Developers need custom JavaScript to react to action lifecycle events (pending, success, error, done) for common UX patterns like form reset, loading states, and button disabling.

**Solution:** Declarative HTML attributes that automatically execute DOM actions in response to LiveTemplate events.

**Pattern:** `lvt-{action}-on:{event}="param"`

**Example:**
```html
<form lvt-submit="create-todo" lvt-reset-on:create-todo:success>
  <button lvt-disable-on:pending lvt-enable-on:done
          lvt-addClass-on:pending="opacity-50"
          lvt-removeClass-on:done="opacity-50">
    Add Todo
  </button>
</form>
```

---

## The Problem

### Common UX Patterns Requiring JavaScript

Currently, implementing these common patterns requires custom JavaScript event listeners:

1. **Form reset on success** - Clear form fields after successful submission
2. **Button loading state** - Disable button and show loading indicator during action
3. **Error message display** - Show/hide error states based on action result
4. **Accessibility** - Set `aria-busy` during pending states

### Current Workaround

```html
<form id="todo-form" lvt-submit="create-todo">
  <input name="title">
  <button id="submit-btn">Add</button>
</form>

<script>
const form = document.getElementById('todo-form');
const btn = document.getElementById('submit-btn');

form.addEventListener('lvt:pending', () => {
  btn.disabled = true;
  btn.classList.add('opacity-50');
});

form.addEventListener('lvt:success', () => {
  form.reset();
});

form.addEventListener('lvt:done', () => {
  btn.disabled = false;
  btn.classList.remove('opacity-50');
});
</script>
```

**Problems:**
- Repetitive boilerplate for every form
- Requires element IDs or complex selectors
- Easy to forget cleanup in `lvt:done`
- Not declarative - logic scattered between HTML and JavaScript

---

## The Solution

### Declarative Attributes

Add HTML attributes that declare reactive behavior:

```html
<form lvt-submit="create-todo" lvt-reset-on:create-todo:success>
  <input name="title">
  <button lvt-disable-on:pending
          lvt-enable-on:done
          lvt-addClass-on:pending="opacity-50"
          lvt-removeClass-on:done="opacity-50">
    Add
  </button>
</form>
```

**No JavaScript required.** The client library handles all event binding automatically.

### Attribute Pattern

```
lvt-{action}-on:{event}="param"
```

Where:
- `{action}` - DOM action to perform (reset, addClass, etc.)
- `{event}` - Lifecycle event to respond to
- `param` - Action-specific parameter (CSS classes, attribute value, etc.)

### Event Syntax

| Syntax | Scope | Example |
|--------|-------|---------|
| `lvt-reset-on:success` | Global (any action) | Resets on ANY action's success |
| `lvt-reset-on:create-todo:success` | Action-specific | Resets only on `create-todo` success |

### Lifecycle Events

| Event | When Fired |
|-------|------------|
| `pending` | Action started, waiting for server response |
| `success` | Action completed successfully (no validation errors) |
| `error` | Action completed with validation errors |
| `done` | Action completed (regardless of success/error) |

---

## Actions Reference

### Parameterless Actions

These actions don't require a value:

| Action | Attribute | Effect |
|--------|-----------|--------|
| `reset` | `lvt-reset-on:success` | Calls `form.reset()` |
| `disable` | `lvt-disable-on:pending` | Sets `element.disabled = true` |
| `enable` | `lvt-enable-on:done` | Sets `element.disabled = false` |

### Parameterized Actions

These actions require a value:

| Action | Attribute | Value Format | Effect |
|--------|-----------|--------------|--------|
| `addClass` | `lvt-addClass-on:pending="loading"` | Space-separated classes | `classList.add(...)` |
| `removeClass` | `lvt-removeClass-on:done="loading"` | Space-separated classes | `classList.remove(...)` |
| `toggleClass` | `lvt-toggleClass-on:pending="active"` | Space-separated classes | `classList.toggle(...)` |
| `setAttr` | `lvt-setAttr-on:pending="aria-busy:true"` | `name:value` | `setAttribute(name, value)` |
| `toggleAttr` | `lvt-toggleAttr-on:pending="readonly"` | Attribute name | `toggleAttribute(name)` |

---

## Use Cases & Examples

### 1. Form Reset on Success

Reset the form only when a specific action succeeds:

```html
<form lvt-submit="create-todo" lvt-reset-on:create-todo:success>
  <input name="title" required placeholder="What needs to be done?">
  <button type="submit">Add Todo</button>
</form>
```

### 2. Submit Button with Loading State

Comprehensive loading state with visual feedback:

```html
<button type="submit"
        lvt-disable-on:pending
        lvt-enable-on:done
        lvt-addClass-on:pending="opacity-50 cursor-wait"
        lvt-removeClass-on:done="opacity-50 cursor-wait">
  <span lvt-addClass-on:pending="hidden">Save Changes</span>
  <span class="hidden" lvt-removeClass-on:pending="hidden" lvt-addClass-on:done="hidden">
    Saving...
  </span>
</button>
```

### 3. Loading Spinner

Show/hide a loading indicator:

```html
<div class="hidden"
     lvt-removeClass-on:pending="hidden"
     lvt-addClass-on:done="hidden">
  <span class="animate-spin">⟳</span> Loading...
</div>
```

### 4. Error Message Container

Show error state, hide on new submission:

```html
<div class="hidden text-red-500"
     lvt-removeClass-on:error="hidden"
     lvt-addClass-on:pending="hidden">
  Something went wrong. Please try again.
</div>
```

### 5. Accessibility with aria-busy

Mark form as busy during submission:

```html
<form lvt-submit="save"
      lvt-setAttr-on:pending="aria-busy:true"
      lvt-setAttr-on:done="aria-busy:false">
  ...
</form>
```

### 6. Read-only Input During Submit

Prevent editing while submitting:

```html
<input type="text" name="email"
       lvt-toggleAttr-on:pending="readonly">
```

### 7. Multiple Actions Same Element

Combine multiple reactive behaviors:

```html
<button type="submit"
        lvt-disable-on:pending
        lvt-enable-on:done
        lvt-addClass-on:pending="bg-gray-400"
        lvt-removeClass-on:done="bg-gray-400"
        lvt-setAttr-on:pending="aria-busy:true"
        lvt-setAttr-on:done="aria-busy:false">
  Submit
</button>
```

### 8. Global vs Action-Specific

React to any action or specific action:

```html
<!-- Global: reacts to ANY action -->
<div class="hidden" lvt-removeClass-on:pending="hidden">
  Processing...
</div>

<!-- Specific: only reacts to 'delete' action -->
<div class="hidden" lvt-removeClass-on:delete:pending="hidden">
  Deleting...
</div>
```

---

## Implementation Notes

### Client-Side Architecture

1. **Event Delegation**: Document-level listeners for `lvt:pending`, `lvt:success`, `lvt:error`, `lvt:done`
2. **Attribute Discovery**: On event fire, query all elements with matching reactive attributes
3. **Event Matching**: Check if fired event matches attribute's event specifier (global or action-specific)
4. **Action Execution**: Execute the appropriate DOM manipulation

### New Module: `reactive-attributes.ts`

```typescript
interface ReactiveBinding {
  action: 'reset' | 'disable' | 'enable' | 'addClass' | 'removeClass' | 'toggleClass' | 'setAttr' | 'toggleAttr';
  lifecycle: 'pending' | 'success' | 'error' | 'done';
  actionName?: string;  // undefined = global
  param?: string;
}

function parseReactiveAttribute(attrName: string, attrValue: string): ReactiveBinding | null;
function executeAction(element: Element, action: string, param?: string): void;
function processReactiveAttributes(lifecycle: string, actionName?: string): void;
```

### Integration Points

1. **form-lifecycle-manager.ts**: Emit events with action name in detail
2. **livetemplate-client.ts**: Initialize document-level event listeners
3. **event-delegation.ts**: Ensure events bubble/capture appropriately

---

## Future Considerations

Potential additions (not in initial scope):

1. **`removeAttr`** - Remove an attribute entirely
2. **`focus`** - Focus an element on event
3. **`scrollIntoView`** - Scroll element into view
4. **`setText`** - Set element text content
5. **Transition support** - CSS transitions for show/hide
6. **Animation triggers** - Start CSS animations on events

---

## Migration & Compatibility

### Backward Compatibility

This feature is purely additive. Existing code continues to work unchanged.

### Relationship to Existing Attributes

| Existing | Purpose | Still Needed? |
|----------|---------|---------------|
| `lvt-disable-with` | Set button text during submit | Yes - different use case (text, not disable) |
| `lvt-preserve` | Don't auto-reset form | Yes - but `lvt-reset-on` gives more control |

### Deprecation Candidates

None. Existing attributes serve different purposes and remain useful.

---

## Acceptance Criteria

1. All 8 actions work: reset, disable, enable, addClass, removeClass, toggleClass, setAttr, toggleAttr
2. Both global and action-specific event matching work
3. Multiple reactive attributes on same element work
4. Events from elements anywhere on page (not just within form) work
5. Unit tests cover attribute parsing and action execution
6. E2E tests cover real-world scenarios with browser automation
