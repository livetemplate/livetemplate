# Client Attributes Reference

Complete reference for LiveTemplate client-side `lvt-*` HTML attributes.

**For server-side Go API:** See [pkg.go.dev/github.com/livetemplate/livetemplate](https://pkg.go.dev/github.com/livetemplate/livetemplate)

## Table of Contents

- [Event Bindings](#event-bindings)
- [Data Passing](#data-passing)
- [Form Lifecycle Events](#form-lifecycle-events)
- [Reactive Attributes](#reactive-attributes)
- [Validation](#validation)
- [Rate Limiting](#rate-limiting)
- [Directives](#directives)
- [Modals](#modals)
- [File Uploads](#file-uploads)
- [Form Behavior](#form-behavior)
- [Attribute Reference](#attribute-reference)

---

## Event Bindings

LiveTemplate uses `lvt-*` attributes to bind DOM events to server-side actions.

### Basic Events

```html
<!-- Click events -->
<button lvt-click="submit">Submit</button>
<button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>

<!-- Form submission -->
<form lvt-submit="save">
    <input type="text" name="title" required>
    <button type="submit">Save</button>
</form>

<!-- Input events -->
<input lvt-change="validate" name="email">
<input lvt-input="search" name="query">
```

### Mouse Events

```html
<!-- Hover events -->
<div lvt-mouseenter="showTooltip" lvt-mouseleave="hideTooltip">
    Hover for tooltip
</div>

<!-- Click events -->
<button lvt-click="handleClick">Click me</button>
```

### Keyboard Events

```html
<!-- Keydown events -->
<input lvt-keydown="handleKey" name="search">

<!-- With key filtering -->
<input lvt-keydown="submit" lvt-key="Enter" name="query">
<div lvt-window-keydown="closeModal" lvt-key="Escape">
    Modal content
</div>
```

### Window Events

```html
<!-- Global keyboard events -->
<div lvt-window-keydown="handleShortcut" lvt-key="Escape">

<!-- Scroll events -->
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

---

## Data Passing

Pass data from the DOM to your server-side action handlers using `lvt-data-*` attributes.

### Simple Data

```html
<button lvt-click="delete" lvt-data-id="{{.ID}}">
    Delete
</button>
```

### Multiple Data Attributes

```html
<button lvt-click="update"
    lvt-data-id="{{.ID}}"
    lvt-data-status="{{.Status}}"
    lvt-data-priority="{{.Priority}}">
    Update Item
</button>
```

### Accessing Data in Go

```go
// Action "delete" with lvt-data-id
func (c *Controller) Delete(state State, ctx *livetemplate.Context) (State, error) {
    id := ctx.GetString("id")
    // Delete item with id
    return state, nil
}

// Action "update" with multiple lvt-data-* attributes
func (c *Controller) Update(state State, ctx *livetemplate.Context) (State, error) {
    id := ctx.GetString("id")
    status := ctx.GetString("status")
    priority := ctx.GetInt("priority")
    // Update item
    return state, nil
}
```

**Available methods:**
- `ctx.GetString(key string) string`
- `ctx.GetInt(key string) int`
- `ctx.GetFloat(key string) float64`
- `ctx.GetBool(key string) bool`
- `ctx.Has(key string) bool`

---

## Form Lifecycle Events

Forms emit JavaScript events during the action lifecycle that you can listen to.

### Event Types

```javascript
const form = document.querySelector('form');

// Fires when action starts
form.addEventListener('lvt:pending', (e) => {
    console.log('Submitting...');
    // Show loading spinner
});

// Fires on validation errors
form.addEventListener('lvt:error', (e) => {
    console.log('Errors:', e.detail.errors);
    // Display error messages
});

// Fires on successful action (no errors)
form.addEventListener('lvt:success', (e) => {
    console.log('Saved!');
    // Show success message, redirect, etc.
});

// Always fires when action completes (success or error)
form.addEventListener('lvt:done', (e) => {
    console.log('Completed');
    // Hide loading spinner
});
```

### Document-Level Events

Lifecycle events also bubble to the document level:

```javascript
// Listen for any action lifecycle events
document.addEventListener('lvt:pending', (e) => {
    console.log('Action starting:', e.detail.action);
});

document.addEventListener('lvt:success', (e) => {
    console.log('Action succeeded:', e.detail.action);
});
```

### Event Detail

```javascript
form.addEventListener('lvt:success', (e) => {
    console.log(e.detail);
    // {
    //   action: "save",
    //   data: {...},
    //   meta: {
    //     success: true,
    //     errors: {}
    //   }
    // }
});
```

---

## Reactive Attributes

Reactive attributes allow declarative DOM manipulation in response to action lifecycle events, without writing JavaScript.

### Pattern

```
lvt-{action}-on:{lifecycle}="param"
lvt-{action}-on:{actionName}:{lifecycle}="param"
```

### Lifecycle Events

| Event | Description |
|-------|-------------|
| `pending` | Action started, waiting for server response |
| `success` | Action completed successfully (no validation errors) |
| `error` | Action completed with validation errors |
| `done` | Action completed (regardless of success/error) |

### Available Actions

| Action | Description | Param |
|--------|-------------|-------|
| `reset` | Calls `form.reset()` | None |
| `disable` | Sets `element.disabled = true` | None |
| `enable` | Sets `element.disabled = false` | None |
| `addClass` | Adds CSS class(es) | Space-separated classes |
| `removeClass` | Removes CSS class(es) | Space-separated classes |
| `toggleClass` | Toggles CSS class(es) | Space-separated classes |
| `setAttr` | Sets an attribute | `name:value` format |
| `toggleAttr` | Toggles a boolean attribute | Attribute name |

### Event Scope

**Global** - Reacts to any action:

```html
<!-- Reset form on any successful action -->
<form lvt-submit="save" lvt-reset-on:success>
    <input name="title">
    <button type="submit">Save</button>
</form>
```

**Action-Specific** - Reacts only to a specific action:

```html
<!-- Reset form only when 'create-todo' succeeds -->
<form lvt-submit="create-todo" lvt-reset-on:create-todo:success>
    <input name="title">
    <button type="submit">Add Todo</button>
</form>
```

### Examples

**Loading States:**

```html
<button
    lvt-click="save"
    lvt-disable-on:pending
    lvt-addClass-on:pending="opacity-50 cursor-wait"
    lvt-enable-on:done
    lvt-removeClass-on:done="opacity-50 cursor-wait">
    Save
</button>
```

**Form Reset on Success:**

```html
<form lvt-submit="create-todo" lvt-reset-on:success>
    <input type="text" name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

**Accessibility States:**

```html
<button
    lvt-click="submit"
    lvt-setAttr-on:pending="aria-busy:true"
    lvt-setAttr-on:done="aria-busy:false">
    Submit
</button>
```

**Error Indicators:**

```html
<!-- Visual feedback on form-level errors -->
<!-- Note: For field-specific validation errors, use .lvt.HasError and .lvt.Error helpers -->
<div
    lvt-addClass-on:error="border-red-500"
    lvt-removeClass-on:success="border-red-500">
    <form lvt-submit="save">
        <input name="email">
        <button type="submit">Save</button>
    </form>
</div>
```

**Input Validation State:**

```html
<!-- For form inputs with validation errors -->
<input
    type="email"
    name="email"
    lvt-setAttr-on:error="aria-invalid:true"
    lvt-setAttr-on:success="aria-invalid:false">
```

**Multiple Actions on Same Element:**

```html
<button
    lvt-click="save"
    lvt-disable-on:pending
    lvt-enable-on:done
    lvt-addClass-on:pending="loading"
    lvt-removeClass-on:done="loading"
    lvt-addClass-on:success="success"
    lvt-addClass-on:error="error">
    Save
</button>
```

**Note:** When multiple reactive attributes target the same lifecycle event, all matching actions execute in DOM order. For example, `lvt-addClass-on:pending="loading"` and `lvt-addClass-on:pending="disabled"` will both add their respective classes.

---

## Validation

LiveTemplate provides server-side validation with automatic error display.

### Server-Side Validation

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

type TodoInput struct {
    Title string `json:"title" validate:"required,min=3,max=100"`
    Tags  string `json:"tags" validate:"required"`
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    var input TodoInput
    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return state, err // Errors automatically sent to client
    }
    // Input is valid, proceed
    state.Todos = append(state.Todos, Todo{Title: input.Title})
    return state, nil
}
```

### Template Error Display

```html
<form lvt-submit="add">
    <div>
        <label for="title">Title</label>
        <input
            type="text"
            name="title"
            id="title"
            {{if .lvt.HasError "title"}}aria-invalid="true"{{end}}>

        {{if .lvt.HasError "title"}}
            <small class="error">{{.lvt.Error "title"}}</small>
        {{end}}
    </div>

    <button type="submit">Add Todo</button>
</form>
```

### Error Helpers

**In templates:**
- `{{.lvt.HasError "field"}}` - Check if field has error
- `{{.lvt.Error "field"}}` - Get error message for field
- `{{.lvt.Errors}}` - Get all errors map

---

## Rate Limiting

Control how often events are processed using debounce and throttle.

### Debounce

Wait for user to stop typing before triggering action.

```html
<!-- Wait 300ms after user stops typing -->
<input
    lvt-input="search"
    lvt-debounce="300"
    name="query"
    placeholder="Search...">
```

**Use for:** Search inputs, auto-save, validation

### Throttle

Limit event frequency to at most once per interval.

```html
<!-- Fire at most once every 100ms -->
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

**Use for:** Scroll events, resize events, mouse tracking

---

## Directives

Directives provide declarative behavior for common UI patterns.

### Scroll Directives

Control scroll behavior after DOM updates.

```html
<!-- Scroll to bottom -->
<div lvt-scroll="bottom" class="chat-messages">
    {{range .Messages}}
        <div>{{.Text}}</div>
    {{end}}
</div>

<!-- Sticky scroll (only if user is near bottom) -->
<div lvt-scroll="bottom-sticky" lvt-scroll-threshold="100">
    {{range .Logs}}
        <div>{{.}}</div>
    {{end}}
</div>

<!-- Scroll to top -->
<div lvt-scroll="top">...</div>

<!-- Preserve scroll position -->
<div lvt-scroll="preserve">...</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-scroll` | Scroll mode: `bottom`, `bottom-sticky`, `top`, `preserve` |
| `lvt-scroll-behavior` | Scroll behavior: `auto` (default), `smooth` |
| `lvt-scroll-threshold` | Pixel threshold for sticky scroll (default: 100) |

### Highlight Directives

Temporarily highlight elements after updates.

```html
<!-- Highlight updated item -->
<div lvt-highlight="flash" lvt-highlight-color="#ffc107" lvt-highlight-duration="500">
    {{.UpdatedContent}}
</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-highlight` | Highlight mode: `flash` |
| `lvt-highlight-color` | Background color (default: `#ffc107`) |
| `lvt-highlight-duration` | Duration in ms (default: 500) |

### Animation Directives

Apply entrance animations to elements.

```html
<!-- Fade in -->
<div lvt-animate="fade">New content</div>

<!-- Slide in -->
<div lvt-animate="slide" lvt-animate-duration="300">Slide content</div>

<!-- Scale in -->
<div lvt-animate="scale">Pop content</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-animate` | Animation type: `fade`, `slide`, `scale` |
| `lvt-animate-duration` | Duration in ms (default: 300) |

---

## Modals

Open and close modals declaratively.

### Opening Modals

```html
<button lvt-modal-open="edit-modal">Edit</button>

<div id="edit-modal" role="dialog" hidden>
    <form lvt-submit="save">
        <input name="title">
        <button type="submit">Save</button>
        <button type="button" lvt-modal-close="edit-modal">Cancel</button>
    </form>
</div>
```

### Modal Attributes

| Attribute | Description |
|-----------|-------------|
| `lvt-modal-open` | Opens modal with specified ID on click |
| `lvt-modal-close` | Closes modal with specified ID on click |

### Modal Behavior

- Modals use `role="dialog"` for accessibility
- Press `Escape` to close the topmost modal
- Click backdrop to close (when using modal backdrop)
- Focus is trapped within open modals

---

## File Uploads

Handle file uploads with progress tracking.

### Basic Upload

```html
<form lvt-submit="save-profile">
    <input type="file" lvt-upload="avatar" name="avatar">
    <button type="submit">Save</button>
</form>
```

### Multiple Files

```html
<input type="file" lvt-upload="documents" name="docs" multiple>
```

### Upload Attributes

| Attribute | Description |
|-----------|-------------|
| `lvt-upload` | Upload identifier for tracking |

Files are automatically uploaded when the form is submitted, with progress events emitted.

---

## Form Behavior

### Preserve Form Data

By default, forms reset after successful submission. Use `lvt-preserve` to keep form values:

```html
<form lvt-submit="search" lvt-preserve>
    <input name="query">
    <button type="submit">Search</button>
</form>
```

### Disable Button During Submit

Show loading state on submit buttons:

```html
<form lvt-submit="save">
    <input name="title">
    <button type="submit" lvt-disable-with="Saving...">Save</button>
</form>
```

### Confirm Delete

Require confirmation for destructive actions:

```html
<button lvt-click="delete" lvt-confirm="Are you sure?">Delete</button>
```

---

## Attribute Reference

Complete reference of all `lvt-*` attributes.

### Event Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-click` | Click event on element | `<button lvt-click="save">` |
| `lvt-submit` | Form submission | `<form lvt-submit="create">` |
| `lvt-change` | Input change event | `<input lvt-change="validate">` |
| `lvt-input` | Input event (every keystroke) | `<input lvt-input="search">` |
| `lvt-keydown` | Keydown event | `<input lvt-keydown="submit">` |
| `lvt-keyup` | Keyup event | `<input lvt-keyup="handle">` |
| `lvt-focus` | Focus event | `<input lvt-focus="highlight">` |
| `lvt-blur` | Blur event | `<input lvt-blur="validate">` |
| `lvt-mouseenter` | Mouse enter event | `<div lvt-mouseenter="show">` |
| `lvt-mouseleave` | Mouse leave event | `<div lvt-mouseleave="hide">` |
| `lvt-click-away` | Click outside element | `<div lvt-click-away="close">` |
| `lvt-window-keydown` | Global keydown | `<div lvt-window-keydown="close">` |
| `lvt-window-keyup` | Global keyup | `<div lvt-window-keyup="handle">` |
| `lvt-window-scroll` | Window scroll | `<div lvt-window-scroll="load">` |
| `lvt-window-resize` | Window resize | `<div lvt-window-resize="adjust">` |
| `lvt-window-focus` | Window focus | `<div lvt-window-focus="refresh">` |
| `lvt-window-blur` | Window blur | `<div lvt-window-blur="pause">` |

### Data Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-data-<key>` | Pass data to action | `lvt-data-id="{{.ID}}"` |
| `lvt-value-<key>` | Pass value to action | `lvt-value-count="{{.Count}}"` |

**Note:** Both `lvt-data-*` and `lvt-value-*` attributes are accessible via `ctx.GetString()`, `ctx.GetInt()`, etc.

### Reactive Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-reset-on:{event}` | Reset form on lifecycle event | `lvt-reset-on:success` |
| `lvt-disable-on:{event}` | Disable element on event | `lvt-disable-on:pending` |
| `lvt-enable-on:{event}` | Enable element on event | `lvt-enable-on:done` |
| `lvt-addClass-on:{event}` | Add class(es) on event | `lvt-addClass-on:pending="loading"` |
| `lvt-removeClass-on:{event}` | Remove class(es) on event | `lvt-removeClass-on:done="loading"` |
| `lvt-toggleClass-on:{event}` | Toggle class(es) on event | `lvt-toggleClass-on:success="active"` |
| `lvt-setAttr-on:{event}` | Set attribute on event | `lvt-setAttr-on:pending="aria-busy:true"` |
| `lvt-toggleAttr-on:{event}` | Toggle boolean attr on event | `lvt-toggleAttr-on:pending="disabled"` |

**Note:** `{event}` can be `pending`, `success`, `error`, or `done`. For action-specific: `lvt-reset-on:create-todo:success`.

### Modifier Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-key` | Filter keyboard events by key | `lvt-key="Enter"` |
| `lvt-debounce` | Debounce delay in milliseconds | `lvt-debounce="300"` |
| `lvt-throttle` | Throttle interval in milliseconds | `lvt-throttle="100"` |

### Form Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-preserve` | Keep form values after submit | `<form lvt-preserve>` |
| `lvt-disable-with` | Button text during submit | `lvt-disable-with="Saving..."` |
| `lvt-confirm` | Confirmation dialog | `lvt-confirm="Are you sure?"` |

### Modal Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-modal-open` | Open modal by ID | `lvt-modal-open="edit-modal"` |
| `lvt-modal-close` | Close modal by ID | `lvt-modal-close="edit-modal"` |

### Directive Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-scroll` | Scroll behavior | `lvt-scroll="bottom"` |
| `lvt-scroll-behavior` | Scroll animation | `lvt-scroll-behavior="smooth"` |
| `lvt-scroll-threshold` | Sticky scroll threshold (px) | `lvt-scroll-threshold="100"` |
| `lvt-highlight` | Highlight effect | `lvt-highlight="flash"` |
| `lvt-highlight-color` | Highlight background color | `lvt-highlight-color="#ffc107"` |
| `lvt-highlight-duration` | Highlight duration (ms) | `lvt-highlight-duration="500"` |
| `lvt-animate` | Entrance animation | `lvt-animate="fade"` |
| `lvt-animate-duration` | Animation duration (ms) | `lvt-animate-duration="300"` |

### Upload Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-upload` | File upload identifier | `lvt-upload="avatar"` |

### Valid Key Values

For `lvt-key` attribute:

- Letter keys: `"a"`, `"b"`, `"c"`, etc.
- Special keys: `"Enter"`, `"Escape"`, `"Space"`, `"Tab"`, `"Backspace"`, `"Delete"`
- Arrow keys: `"ArrowUp"`, `"ArrowDown"`, `"ArrowLeft"`, `"ArrowRight"`
- Function keys: `"F1"`, `"F2"`, etc.
- Modifiers: Check `e.ctrlKey`, `e.shiftKey`, `e.altKey`, `e.metaKey` in event listeners

---

## Best Practices

### 1. Use Reactive Attributes for Loading States

Prefer declarative reactive attributes over JavaScript for common UI patterns:

```html
<!-- Good: Declarative loading state -->
<button
    lvt-click="save"
    lvt-disable-on:pending
    lvt-addClass-on:pending="opacity-50"
    lvt-enable-on:done
    lvt-removeClass-on:done="opacity-50">
    Save
</button>

<!-- Avoid: JavaScript for simple loading state -->
```

### 2. Use Debounce for Search

```html
<input
    lvt-input="search"
    lvt-debounce="300"
    name="query">
```

### 3. Use Throttle for Scroll

```html
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

### 4. Show Validation Errors

```html
<input
    type="email"
    name="email"
    {{if .lvt.HasError "email"}}aria-invalid="true"{{end}}>
{{if .lvt.HasError "email"}}
    <span class="error">{{.lvt.Error "email"}}</span>
{{end}}
```

### 5. Reset Forms on Success

Use reactive attributes for automatic form reset:

```html
<form lvt-submit="create-todo" lvt-reset-on:success>
    <input name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

### 6. Accessibility with Reactive Attributes

```html
<button
    lvt-click="save"
    lvt-setAttr-on:pending="aria-busy:true"
    lvt-setAttr-on:done="aria-busy:false"
    lvt-setAttr-on:error="aria-invalid:true">
    Save
</button>
```

---

## Advanced Usage

### Custom Event Handling

```javascript
document.addEventListener('lvt:connected', () => {
    console.log('WebSocket connected');
});

document.addEventListener('lvt:disconnected', () => {
    console.log('WebSocket disconnected');
});
```

### Accessing Form Data

```javascript
form.addEventListener('lvt:pending', (e) => {
    const formData = new FormData(e.target);
    console.log('Submitting:', Object.fromEntries(formData));
});
```

---

## Related Documentation

- **[Go API Reference](https://pkg.go.dev/github.com/livetemplate/livetemplate)** - Server-side API
- **[Error Handling Reference](error-handling.md)** - Validation, error display, client-side handling
- **[Template Support Matrix](template-support-matrix.md)** - Supported Go template features
- **[Architecture](../design/ARCHITECTURE.md)** - System architecture
- **[Contributing Guide](../../CONTRIBUTING.md)** - How to contribute
