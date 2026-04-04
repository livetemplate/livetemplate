# Client Attributes Reference

Complete reference for LiveTemplate form handling and `lvt-*` HTML attributes.

**For server-side Go API:** See [pkg.go.dev/github.com/livetemplate/livetemplate](https://pkg.go.dev/github.com/livetemplate/livetemplate)

## Table of Contents

- [Standard HTML Form Routing](#standard-html-form-routing)
- [Event Bindings](#event-bindings)
- [Form Lifecycle Events](#form-lifecycle-events)
- [Reactive Attributes](#reactive-attributes)
- [Validation](#validation)
- [Rate Limiting](#rate-limiting)
- [Directives](#directives)
- [File Uploads](#file-uploads)
- [Form Behavior](#form-behavior)
- [Attribute Reference](#attribute-reference)

---

## Standard HTML Form Routing

LiveTemplate follows a **progressive complexity** model. Standard HTML forms work without any `lvt-*` attributes. Use `lvt-*` only for behaviors HTML cannot express (debounce, loading states, reactive DOM, etc.).

### Auto-Submit (Zero Attributes)

All `<form>` elements within a LiveTemplate-managed region are automatically intercepted. Forms without explicit action routing default to the `Submit()` method on the controller:

```html
<!-- No lvt-* needed — auto-routes to Submit() -->
<form method="POST">
    <input type="text" name="title" placeholder="New todo...">
    <button type="submit">Add</button>
</form>
```

```go
func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    title := ctx.GetString("title")
    // ...
    return state, nil
}
```

### Action Routing via Button Name

The button's `name` IS the action. Button `value` carries optional data:

```html
<form method="POST">
    <input type="text" name="Title" value="{{.Title}}">
    <button name="save">Save</button>
    <button name="save-draft" formnovalidate>Save Draft</button>
</form>
```

`<button name="save">` routes to `Save()`. `<button name="save-draft">` routes to `SaveDraft()`.

### Action Routing via Form Name

Use the `name` attribute on the form itself:

```html
<form name="search" method="POST">
    <input name="query" value="{{.Query}}">
    <button type="submit">Search</button>
</form>
```

Routes to `Search()` on the controller **when using the JS client**, which reads `form.name`. A plain HTML POST does not include the form's `name` attribute, so for no-JS compatibility use `<button name="search">` instead.

### Data Passing

Data can be passed via hidden inputs, button `value`, or `data-*` attributes:

```html
{{range .Items}}
<form method="POST">
    <input type="hidden" name="id" value="{{.ID}}">
    <button name="toggle">{{if .Done}}Undo{{else}}Done{{end}}</button>
    <button name="delete" value="{{.ID}}">Delete</button>
</form>
{{end}}
```

- Hidden inputs: `ctx.GetString("id")`
- Button value: `ctx.GetString("value")`
- `data-*` on button: `ctx.GetString("key")`

### Action Resolution Order

The client resolves the action name in this order (first match wins):

1. Explicit `action` field → `<button name="action" value="save">` routes to `Save()`
2. Clicked button's `name` attribute → `<button name="save">` routes to `Save()` (uses empty-value heuristic; works when only the clicked button submits an empty value)
3. `form name="X"` → action is `X`
4. None of the above → defaults to `"submit"` → routes to `Submit()`

### Opt-Out

Forms that should NOT be auto-intercepted (external URLs, downloads):

```html
<form action="/api/export" method="POST" lvt-form:no-intercept>
    <button type="submit">Export CSV</button>
</form>
```

### Transport Compatibility

| Mechanism | No JS | JS + HTTP | JS + WebSocket |
|-----------|-------|-----------|----------------|
| `button name="action"` | Native POST | Client extracts | Client extracts |
| `form name` | N/A (use button) | Client reads | Client reads |
| Hidden inputs | Native POST | In FormData | In FormData |

---

## Event Bindings

LiveTemplate uses `lvt-on:{event}` attributes to bind DOM events to server-side actions. These are for interactions that standard HTML forms cannot express.

### Pattern

```
lvt-on:{event}="action"           (element scope)
lvt-on:window:{event}="action"    (window scope)
```

### Basic Events

```html
<!-- Click events -->
<button lvt-on:click="submit">Submit</button>
<button lvt-on:click="delete" data-id="{{.ID}}">Delete</button>

<!-- Input events -->
<input lvt-on:input="search" name="query">
```

### Mouse Events

```html
<!-- Hover events -->
<div lvt-on:mouseenter="showTooltip" lvt-on:mouseleave="hideTooltip">
    Hover for tooltip
</div>

<!-- Click events -->
<button lvt-on:click="handleClick">Click me</button>
```

### Keyboard Events

```html
<!-- Keydown events -->
<input lvt-on:keydown="handleKey" name="search">

<!-- With key filtering -->
<input lvt-on:keydown="submit" lvt-key="Enter" name="query">
<div lvt-on:window:keydown="closeModal" lvt-key="Escape">
    Modal content
</div>
```

### Window Events

```html
<!-- Global keyboard events -->
<div lvt-on:window:keydown="handleShortcut" lvt-key="Escape">

<!-- Scroll events -->
<div lvt-on:window:scroll="loadMore" lvt-mod:throttle="100">
```

### Focus Events

```html
<input lvt-on:focus="highlight" name="email">
<input lvt-on:blur="validate" name="email">

<!-- Window focus/blur -->
<div lvt-on:window:focus="refresh">
<div lvt-on:window:blur="pause">
```

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
lvt-el:{method}:on:{lifecycle}="param"
lvt-el:{method}:on:{actionName}:{lifecycle}="param"
```

### Lifecycle Events

| Event | Description |
|-------|-------------|
| `pending` | Action started, waiting for server response |
| `success` | Action completed successfully (no validation errors) |
| `error` | Action completed with validation errors |
| `done` | Action completed (regardless of success/error) |

### Available Methods

| Method | Description | Param |
|--------|-------------|-------|
| `reset` | Calls `form.reset()` | None |
| `addClass` | Adds CSS class(es) | Space-separated classes |
| `removeClass` | Removes CSS class(es) | Space-separated classes |
| `toggleClass` | Toggles CSS class(es) | Space-separated classes |
| `setAttr` | Sets an attribute | `name:value` format |
| `toggleAttr` | Toggles a boolean attribute | Attribute name |

**Note:** To disable/enable elements, use `lvt-el:toggleAttr:on:pending="disabled"` and `lvt-el:toggleAttr:on:done="disabled"`.

### Event Scope

**Global** - Reacts to any action:

```html
<!-- Reset form on any successful action -->
<form name="save" method="POST" lvt-el:reset:on:success>
    <input name="title">
    <button type="submit">Save</button>
</form>
```

**Action-Specific** - Reacts only to a specific action:

```html
<!-- Reset form only when 'create-todo' succeeds -->
<form name="create-todo" method="POST" lvt-el:reset:on:create-todo:success>
    <input name="title">
    <button type="submit">Add Todo</button>
</form>
```

### Examples

**Loading States:**

```html
<button
    lvt-on:click="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:addClass:on:pending="opacity-50 cursor-wait"
    lvt-el:toggleAttr:on:done="disabled"
    lvt-el:removeClass:on:done="opacity-50 cursor-wait">
    Save
</button>
```

**Form Reset on Success:**

```html
<form name="create-todo" method="POST" lvt-el:reset:on:success>
    <input type="text" name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

**Accessibility States:**

```html
<button
    lvt-on:click="submit"
    lvt-el:setAttr:on:pending="aria-busy:true"
    lvt-el:setAttr:on:done="aria-busy:false">
    Submit
</button>
```

**Error Indicators:**

```html
<!-- Visual feedback on form-level errors -->
<!-- Note: For field-specific validation errors, use .lvt.HasError and .lvt.Error helpers -->
<div
    lvt-el:addClass:on:error="border-red-500"
    lvt-el:removeClass:on:success="border-red-500">
    <form name="save" method="POST">
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
    lvt-el:setAttr:on:error="aria-invalid:true"
    lvt-el:setAttr:on:success="aria-invalid:false">
```

**Multiple Actions on Same Element:**

```html
<button
    lvt-on:click="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:toggleAttr:on:done="disabled"
    lvt-el:addClass:on:pending="loading"
    lvt-el:removeClass:on:done="loading"
    lvt-el:addClass:on:success="success"
    lvt-el:addClass:on:error="error">
    Save
</button>
```

**Note:** When multiple reactive attributes target the same lifecycle event, all matching actions execute in DOM order. For example, `lvt-el:addClass:on:pending="loading"` and `lvt-el:addClass:on:pending="disabled"` will both add their respective classes.

### Bracket Expansion (Multi-Action Shorthand)

When the same reactive attribute applies to multiple actions, use bracket syntax to avoid repetition:

```html
<!-- Shorthand: bracket syntax -->
<button
    lvt-on:click="save"
    lvt-el:addClass:on:[save,delete]:pending="opacity-50"
    lvt-el:toggleAttr:on:[save,delete]:pending="disabled">
    Save
</button>

<!-- Equivalent expanded form -->
<button
    lvt-on:click="save"
    lvt-el:addClass:on:save:pending="opacity-50"
    lvt-el:addClass:on:delete:pending="opacity-50"
    lvt-el:toggleAttr:on:save:pending="disabled"
    lvt-el:toggleAttr:on:delete:pending="disabled">
    Save
</button>
```

Bracket expansion works for `lvt-el:*`, `lvt-fx:*`, and `lvt-form:*` prefixes, including boolean attributes (no `="value"`). Bracket syntax works everywhere in templates, including inside `{{range}}` and `{{if}}` blocks.

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
<form name="add" method="POST">
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
    lvt-on:input="search"
    lvt-mod:debounce="300"
    name="query"
    placeholder="Search...">
```

**Use for:** Search inputs, auto-save, validation

### Throttle

Limit event frequency to at most once per interval.

```html
<!-- Fire at most once every 100ms -->
<div lvt-on:window:scroll="loadMore" lvt-mod:throttle="100">
```

**Use for:** Scroll events, resize events, mouse tracking

---

## Directives

Directives provide declarative behavior for common UI patterns.

### Scroll Directives

Control scroll behavior after DOM updates.

```html
<!-- Scroll to bottom -->
<div lvt-fx:scroll="bottom" class="chat-messages">
    {{range .Messages}}
        <div>{{.Text}}</div>
    {{end}}
</div>

<!-- Sticky scroll (only if user is near bottom) -->
<div lvt-fx:scroll="bottom-sticky" style="--lvt-scroll-threshold: 100px">
    {{range .Logs}}
        <div>{{.}}</div>
    {{end}}
</div>

<!-- Scroll to top -->
<div lvt-fx:scroll="top">...</div>

<!-- Preserve scroll position -->
<div lvt-fx:scroll="preserve">...</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-fx:scroll` | Scroll mode: `bottom`, `bottom-sticky`, `top`, `preserve` |

Scroll behavior and threshold are configured via CSS custom properties:

| CSS Custom Property | Description | Default |
|---------------------|-------------|---------|
| `--lvt-scroll-behavior` | Scroll behavior: `auto`, `smooth` | `auto` |
| `--lvt-scroll-threshold` | Pixel threshold for sticky scroll | `100px` |

Defaults are provided by importing `livetemplate.css`.

### Highlight Directives

Temporarily highlight elements after updates.

```html
<!-- Highlight updated item -->
<div lvt-fx:highlight="flash" style="--lvt-highlight-color: #ffc107; --lvt-highlight-duration: 500ms">
    {{.UpdatedContent}}
</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-fx:highlight` | Highlight mode: `flash` |

Highlight appearance is configured via CSS custom properties:

| CSS Custom Property | Description | Default |
|---------------------|-------------|---------|
| `--lvt-highlight-color` | Background color | `#ffc107` |
| `--lvt-highlight-duration` | Duration | `500ms` |

### Animation Directives

Apply entrance animations to elements.

```html
<!-- Fade in -->
<div lvt-fx:animate="fade">New content</div>

<!-- Slide in -->
<div lvt-fx:animate="slide" style="--lvt-animate-duration: 300ms">Slide content</div>

<!-- Scale in -->
<div lvt-fx:animate="scale">Pop content</div>
```

| Attribute | Description |
|-----------|-------------|
| `lvt-fx:animate` | Animation type: `fade`, `slide`, `scale` |

Animation duration is configured via CSS custom properties:

| CSS Custom Property | Description | Default |
|---------------------|-------------|---------|
| `--lvt-animate-duration` | Duration | `300ms` |

---

## File Uploads

Handle file uploads with progress tracking.

### Basic Upload

```html
<form name="save-profile" method="POST">
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

By default, forms reset after successful submission. Use `lvt-form:preserve` to keep form values:

```html
<form name="search" method="POST" lvt-form:preserve>
    <input name="query">
    <button type="submit">Search</button>
</form>
```

### Disable Button During Submit

Show loading state on submit buttons:

```html
<form name="save" method="POST">
    <input name="title">
    <button type="submit" lvt-form:disable-with="Saving...">Save</button>
</form>
```

---

## Attribute Reference

Complete reference of all `lvt-*` attributes.

### Event Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-on:click` | Click event on element | `<button lvt-on:click="save">` |
| `lvt-on:input` | Input event (every keystroke) | `<input lvt-on:input="search">` |
| `lvt-on:change` | Change event (on blur for text, immediate for select/checkbox/radio) | `<select lvt-on:change="filter">` |
| `lvt-on:keydown` | Keydown event | `<input lvt-on:keydown="submit">` |
| `lvt-on:keyup` | Keyup event | `<input lvt-on:keyup="handle">` |
| `lvt-on:focus` | Focus event | `<input lvt-on:focus="highlight">` |
| `lvt-on:blur` | Blur event | `<input lvt-on:blur="validate">` |
| `lvt-on:mouseenter` | Mouse enter event | `<div lvt-on:mouseenter="show">` |
| `lvt-on:mouseleave` | Mouse leave event | `<div lvt-on:mouseleave="hide">` |
| `lvt-on:window:keydown` | Global keydown | `<div lvt-on:window:keydown="close">` |
| `lvt-on:window:keyup` | Global keyup | `<div lvt-on:window:keyup="handle">` |
| `lvt-on:window:scroll` | Window scroll | `<div lvt-on:window:scroll="load">` |
| `lvt-on:window:resize` | Window resize | `<div lvt-on:window:resize="adjust">` |
| `lvt-on:window:focus` | Window focus | `<div lvt-on:window:focus="refresh">` |
| `lvt-on:window:blur` | Window blur | `<div lvt-on:window:blur="pause">` |

### Reactive Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-el:reset:on:{event}` | Reset form on lifecycle event | `lvt-el:reset:on:success` |
| `lvt-el:addClass:on:{event}` | Add class(es) on event | `lvt-el:addClass:on:pending="loading"` |
| `lvt-el:removeClass:on:{event}` | Remove class(es) on event | `lvt-el:removeClass:on:done="loading"` |
| `lvt-el:toggleClass:on:{event}` | Toggle class(es) on event | `lvt-el:toggleClass:on:success="active"` |
| `lvt-el:setAttr:on:{event}` | Set attribute on event | `lvt-el:setAttr:on:pending="aria-busy:true"` |
| `lvt-el:toggleAttr:on:{event}` | Toggle boolean attr on event | `lvt-el:toggleAttr:on:pending="disabled"` |

**Note:** `{event}` can be `pending`, `success`, `error`, or `done`. For action-specific: `lvt-el:reset:on:create-todo:success`.

### Modifier Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-key` | Filter keyboard events by key | `lvt-key="Enter"` |
| `lvt-mod:debounce` | Debounce delay in milliseconds | `lvt-mod:debounce="300"` |
| `lvt-mod:throttle` | Throttle interval in milliseconds | `lvt-mod:throttle="100"` |

### Form Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-form:preserve` | Keep form values after submit | `<form lvt-form:preserve>` |
| `lvt-form:disable-with` | Button text during submit | `lvt-form:disable-with="Saving..."` |
| `lvt-form:no-intercept` | Opt out of auto-interception | `<form lvt-form:no-intercept>` |

### Directive Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-fx:scroll` | Scroll behavior | `lvt-fx:scroll="bottom"` |
| `lvt-fx:highlight` | Highlight effect | `lvt-fx:highlight="flash"` |
| `lvt-fx:animate` | Entrance animation | `lvt-fx:animate="fade"` |

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
    lvt-on:click="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:addClass:on:pending="opacity-50"
    lvt-el:toggleAttr:on:done="disabled"
    lvt-el:removeClass:on:done="opacity-50">
    Save
</button>

<!-- Avoid: JavaScript for simple loading state -->
```

### 2. Use Debounce for Search

```html
<input
    lvt-on:input="search"
    lvt-mod:debounce="300"
    name="query">
```

### 3. Use Throttle for Scroll

```html
<div lvt-on:window:scroll="loadMore" lvt-mod:throttle="100">
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
<form name="create-todo" method="POST" lvt-el:reset:on:success>
    <input name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

### 6. Accessibility with Reactive Attributes

```html
<button
    lvt-on:click="save"
    lvt-el:setAttr:on:pending="aria-busy:true"
    lvt-el:setAttr:on:done="aria-busy:false"
    lvt-el:setAttr:on:error="aria-invalid:true">
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
