# Client Attributes Reference

Complete reference for LiveTemplate form handling and `lvt-*` HTML attributes.

**For server-side Go API:** See [pkg.go.dev/github.com/livetemplate/livetemplate](https://pkg.go.dev/github.com/livetemplate/livetemplate)

## Table of Contents

- [Standard HTML Form Routing](#standard-html-form-routing)
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

1. `lvt-form:action="X"` on the form → action is `X` (explicit routing, highest precedence)
2. `lvt-submit="X"` on the form → action is `X` (backward compatible)
3. Clicked button's `name` attribute → action is the button name
4. `form name="X"` → action is `X`
5. None of the above → defaults to `"submit"` → routes to `Submit()`

> **Note:** The form field name `action` is **not** reserved. A form field `<input name="action" value="approve">` flows through to `ActionData` as normal data. Use `lvt-form:action` on the `<form>` element for routing.

### Opt-Out

Forms that should NOT be auto-intercepted (external URLs, downloads):

```html
<form action="/api/export" method="POST" lvt-form:no-intercept>
    <button type="submit">Export CSV</button>
</form>
```

Links that should NOT be auto-intercepted (external pages, legacy routes):

```html
<a href="/legacy-page" lvt-nav:no-intercept>Legacy Page</a>
```

> **Note:** Use `lvt-form:no-intercept` on `<form>` elements and `lvt-nav:no-intercept` on `<a>` elements. These are semantically distinct: form interception vs. link/navigation interception.

### Transport Compatibility

| Mechanism | No JS | JS + HTTP | JS + WebSocket |
|-----------|-------|-----------|----------------|
| `button name="action"` | Native POST | Client extracts | Client extracts |
| `form name` | N/A (use button) | Client reads | Client reads |
| Hidden inputs | Native POST | In FormData | In FormData |

---

## Event Bindings

LiveTemplate uses `lvt-*` attributes to bind DOM events to server-side actions. These are for interactions that standard HTML forms cannot express.

### Basic Events

```html
<!-- Click events -->
<button lvt-on:click="submit">Submit</button>
<button lvt-on:click="delete" lvt-data-id="{{.ID}}">Delete</button>

<!-- Form submission -->
<form lvt-form:action="save">
    <input type="text" name="title" required>
    <button type="submit">Save</button>
</form>

<!-- Input events -->
<input lvt-on:change="validate" name="email">
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

---

## Data Passing

Pass data to Tier 2 event handlers using `lvt-data-*` attributes. For Tier 1 forms, use standard HTML instead: hidden inputs (`<input type="hidden" name="id" value="{{.ID}}">`), button `value`, or `data-*` attributes on buttons. See [Standard HTML — Data Passing](#data-passing-1) above.

### Simple Data

```html
<button lvt-on:click="delete" lvt-data-id="{{.ID}}">
    Delete
</button>
```

### Multiple Data Attributes

```html
<button lvt-on:click="update"
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

Reactive attributes allow declarative DOM manipulation in response to action lifecycle events or native DOM events, without writing JavaScript.

### Pattern

```
lvt-el:{method}:on:{trigger}="param"
lvt-el:{method}:on:{action}:{trigger}="param"
```

Where `{trigger}` is a lifecycle state **or** any native DOM event (see below).

### Lifecycle Events

| Event | Description |
|-------|-------------|
| `pending` | Action started, waiting for server response |
| `success` | Action completed successfully (no validation errors) |
| `error` | Action completed with validation errors |
| `done` | Action completed (regardless of success/error) |

### Interaction Triggers

In addition to lifecycle states, `lvt-el:` supports native DOM events as triggers.
These execute client-side with no server round-trip.

| Trigger | DOM Event | Use case |
|---------|-----------|----------|
| `click` | `click` | Toggle visibility on click |
| `focusin` | `focusin` | Open panel when focus enters (bubbles) |
| `focusout` | `focusout` | Close panel when focus leaves (bubbles) |
| `mouseenter` | `mouseenter` | Show on hover |
| `mouseleave` | `mouseleave` | Hide on hover end |
| `click-away` | (synthetic) | Close when clicking outside element |
| Any other | Corresponding DOM event | Custom behavior |

### Available Methods

| Method | Description | Param |
|--------|-------------|-------|
| `reset` | Calls `form.reset()` | None |
| `addClass` | Adds CSS class(es) | Space-separated classes |
| `removeClass` | Removes CSS class(es) | Space-separated classes |
| `toggleClass` | Toggles CSS class(es) | Space-separated classes |
| `setAttr` | Sets an attribute | `name:value` format |
| `toggleAttr` | Toggles a boolean attribute | Attribute name |

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
<button name="save"
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
<button name="submit"
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
<button name="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:toggleAttr:on:done="disabled"
    lvt-el:addClass:on:pending="loading"
    lvt-el:removeClass:on:done="loading"
    lvt-el:addClass:on:success="success"
    lvt-el:addClass:on:error="error">
    Save
</button>
```

**Note:** When multiple reactive attributes target the same lifecycle event, all matching methods execute in DOM order. For example, `lvt-el:addClass:on:pending="loading"` and `lvt-el:addClass:on:pending="disabled"` will both add their respective classes.

### DOM Event Trigger Examples

```html
<!-- Toggle dropdown visibility on click -->
<div lvt-el:toggleClass:on:click="open"
     lvt-el:removeClass:on:click-away="open">
  ...
</div>

<!-- Show tooltip on hover -->
<div lvt-el:addClass:on:mouseenter="visible"
     lvt-el:removeClass:on:mouseleave="visible">
  ...
</div>

<!-- Open suggestions on focus, close on blur -->
<div lvt-el:addClass:on:focusin="open"
     lvt-el:removeClass:on:focusout="open"
     lvt-el:removeClass:on:click-away="open">
  <input type="text" ...>
  <ul data-suggestions>...</ul>
</div>
```

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

> **Note:** Attribute values must be quoted (`="..."` or `='...'`). Unquoted values like `lvt-el:addClass:on:[a,b]:pending=loading` will produce incorrect output. Bracket expansion operates on raw template source, so patterns inside `<script>` or `<style>` blocks would also be expanded if they match — though the `lvt-el:`/`lvt-fx:`/`lvt-form:` prefixes make false matches unlikely in practice.

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
| `--lvt-scroll-behavior` | CSS custom property: `auto` (default), `smooth` |
| `--lvt-scroll-threshold` | CSS custom property: pixel threshold for sticky scroll (default: 100px) |

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
| `--lvt-highlight-color` | CSS custom property: background color (default: `#ffc107`) |
| `--lvt-highlight-duration` | CSS custom property: duration (default: 500ms) |

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
| `--lvt-animate-duration` | CSS custom property: duration (default: 300ms) |

### Trigger Types

`lvt-fx:` attributes support three trigger modes:

**Implicit (no `:on:`)** -- fires on every DOM content update:

```html
<div lvt-fx:scroll="bottom-sticky">...</div>
<div lvt-fx:highlight="flash">...</div>
```

**Lifecycle (`:on:{state}`)** -- fires on action lifecycle state:

```html
<div lvt-fx:highlight:on:success="flash">Saved!</div>
<div lvt-fx:highlight:on:save:success="flash">Save confirmed</div>
```

**DOM Event (`:on:{event}`)** -- fires on any native DOM event:

```html
<div lvt-fx:highlight:on:click="flash">Click to highlight</div>
<div lvt-fx:highlight:on:mouseenter="flash">Hover to highlight</div>
<div lvt-fx:animate:on:click="fade">Click to animate</div>
```

---

## Modals

Use the native `<dialog>` element with `command`/`commandfor` for modal dialogs. No `lvt-*` attributes needed — this is a Tier 1 pattern.

The client polyfills the [Invoker Commands API](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/command) for browsers that don't support it natively (Firefox, Safari as of April 2026). The polyfill calls `.showModal()` / `.close()` on the target `<dialog>`, providing backdrop, focus trapping, and Escape key handling across all browsers. Feature detection via `commandForElement` makes the polyfill a no-op when native support lands.

### Supported commands

| Button Attribute | Target | Effect |
|---|---|---|
| `command="show-modal" commandfor="dialog-id"` | `<dialog id="dialog-id">` | Calls `.showModal()` |
| `command="close" commandfor="dialog-id"` | `<dialog id="dialog-id">` | Calls `.close()` |

### Auto-close on success

Any form inside a `<dialog>` that completes successfully will have its parent dialog closed automatically. This means the dialog stays open for validation errors but closes on success — no extra attributes needed.

A `<form method="dialog">` inside a `<dialog>` closes the dialog immediately on submit (before the server responds). Use this only when you don't need server-side validation feedback inside the dialog.

See [Progressive Complexity Guide — Dialogs](../guides/progressive-complexity.md#5-dialogs) for the full walkthrough.

### Server-managed modals

For modals whose visibility is controlled by server state (e.g., confirmation dialogs triggered by a server action), use the `lvt/components/modal` package. See the [todos example](https://github.com/livetemplate/examples/tree/main/todos).

---

## Automatic Client-Side State Preservation

The client automatically preserves certain client-side state across server-pushed DOM updates. These behaviors require no attributes — they are built into the morphdom diffing pass.

### Checkbox and Radio Buttons

User-toggled `checked` state on `<input type="checkbox">` and `<input type="radio">` survives DOM updates. The client copies the live DOM's `.checked` property onto the incoming virtual element before morphdom compares them, so morphdom sees no diff and leaves the element alone.

```html
<!-- User checks this box; a server refresh won't uncheck it -->
<label><input type="checkbox" name="select" value="item-1"> Item 1</label>
```

**Radio group caveat:** Browser mutual exclusion fires synchronously during the morphdom pass. If you need to force-reset a radio group from the server, add `data-lvt-force-update` to *all* radios in the group, not just the one being checked.

### Dialog Open State

When a `<dialog>` is opened via `showModal()`, the browser adds it to the top layer — a special rendering context above all other content. The `open` attribute alone doesn't preserve this state; morphdom's attribute sync and child reconciliation can disrupt the top-layer positioning even when `open` is retained. The client prevents this by skipping the entire dialog element and its subtree while `open` is present. The server continues sending updates while the dialog is open, but the client discards them. After the dialog closes, the next server update applies normally, reconciling the client DOM with current server state.

Adding `data-lvt-force-update` to the `<dialog>` overrides this skip: the client applies morphdom to the dialog's content while it remains open, allowing the server to update dialog contents in real time (e.g., live validation feedback inside a modal form).

```html
<!-- Dialog stays open across server refreshes -->
<dialog id="settings">
  <form method="POST" name="SaveSettings">
    <input name="theme">
    <button type="submit">Save</button>
  </form>
</dialog>
```

### Datalist Dropdown

Native `<datalist>` dropdowns are fragile — ANY DOM mutation on the page (not just to the datalist itself) dismisses the popup, and unlike checkbox state, dropdown-open has no DOM representation. The client defers the entire morphdom pass while `document.activeElement` is an `<input>` connected to a `<datalist>` via the `list` attribute.

```html
<input type="text" list="suggestions" name="query">
<datalist id="suggestions">
  <option value="alpha">
  <option value="beta">
</datalist>
```

When the user blurs the input, the deferred morphdom pass runs, applying all pending changes (not just to the datalist, but to the entire page). Adding `data-lvt-force-update` to the connected `<input>` overrides this deferral, allowing the morphdom pass to proceed immediately even while the datalist dropdown is open.

### Focused Input Elements

Any form element that currently has focus is skipped during morphdom updates, preserving in-progress user input.

### Overriding with `data-lvt-force-update`

All automatic preservation behaviors can be overridden by adding `data-lvt-force-update` to the element in the server template. When present, the server's value wins over the client-side state. The client strips the attribute from the live DOM after applying the update; because it lives in the server template, the server re-sends it on every render, so it continuously forces the server value.

```html
<!-- Server always controls this checkbox -->
<input type="checkbox" name="locked" data-lvt-force-update {{if .Locked}}checked{{end}}>
```

| Preserved State | Mechanism | Override |
|---|---|---|
| Checkbox/radio `checked` | Property copied to virtual DOM | `data-lvt-force-update` on the input |
| Dialog `open` | morphdom update skipped while dialog is open | `data-lvt-force-update` on the dialog |
| Datalist dropdown | Entire morphdom pass deferred while input focused | `data-lvt-force-update` on the connected `<input>` (overrides deferral for the entire pass) |
| Focused input elements | morphdom update skipped | `data-lvt-force-update` on the input |

### Manual Preservation with `lvt-ignore`

For cases where automatic preservation doesn't cover your needs, two attributes provide explicit control:

- **`lvt-ignore`** — Skips the element and its entire subtree during morphdom diff. Use this for third-party widgets (maps, rich-text editors) whose DOM is managed by external JavaScript. Checked on the live DOM element, so it can be set from templates or client JS. Equivalent to Phoenix LiveView's `phx-update="ignore"`.

- **`lvt-ignore-attrs`** — Skips attribute diffing but still diffs children. Use this when client-set attributes (e.g., `open` on `<details>`) need to survive server updates while child content remains server-managed.

Both can be overridden by `data-lvt-force-update` when the server needs to take control.

---

## File Uploads

Handle file uploads with progress tracking.

### Basic Upload

```html
<form method="POST">
    <input type="file" lvt-upload="avatar" name="avatar">
    <button name="save-profile" type="submit">Save</button>
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
<form method="POST">
    <input name="title">
    <button name="save" type="submit" lvt-form:disable-with="Saving...">Save</button>
</form>
```

### Confirm Delete

Use standard `onsubmit` for confirmation dialogs:

```html
<form method="POST" onsubmit="return confirm('Are you sure?')">
    <button name="delete">Delete</button>
</form>
```

---

## Attribute Reference

Complete reference of all `lvt-*` and `data-*` template attributes.

### Event Attributes (`lvt-on:`)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-on:click` | Click event | `<button lvt-on:click="save">` |
| `lvt-on:change` | Input change event | `<select lvt-on:change="sort">` |
| `lvt-on:input` | Input event (every keystroke) | `<input lvt-on:input="search">` |
| `lvt-on:keydown` | Keydown event | `<input lvt-on:keydown="submit">` |
| `lvt-on:keyup` | Keyup event | `<input lvt-on:keyup="handle">` |
| `lvt-on:focus` | Focus event | `<input lvt-on:focus="highlight">` |
| `lvt-on:blur` | Blur event | `<input lvt-on:blur="validate">` |
| `lvt-on:mouseenter` | Mouse enter event | `<div lvt-on:mouseenter="show">` |
| `lvt-on:mouseleave` | Mouse leave event | `<div lvt-on:mouseleave="hide">` |
| `lvt-on:click-away` | Click outside element | `<div lvt-on:click-away="close">` |
| `lvt-on:window:keydown` | Global keydown | `<div lvt-on:window:keydown="close">` |
| `lvt-on:window:keyup` | Global keyup | `<div lvt-on:window:keyup="handle">` |
| `lvt-on:window:scroll` | Window scroll | `<div lvt-on:window:scroll="load">` |
| `lvt-on:window:resize` | Window resize | `<div lvt-on:window:resize="adjust">` |
| `lvt-on:window:focus` | Window focus | `<div lvt-on:window:focus="refresh">` |
| `lvt-on:window:blur` | Window blur | `<div lvt-on:window:blur="pause">` |

### Data Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-data-<key>` | Pass data to action | `lvt-data-id="{{.ID}}"` |
| `lvt-value-<key>` | Pass value to action | `lvt-value-count="{{.Count}}"` |

**Note:** Both `lvt-data-*` and `lvt-value-*` attributes are accessible via `ctx.GetString()`, `ctx.GetInt()`, etc.

### Reactive Attributes (`lvt-el:`)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-el:reset:on:{trigger}` | Reset form on trigger | `lvt-el:reset:on:success` |
| `lvt-el:addClass:on:{trigger}` | Add class(es) on trigger | `lvt-el:addClass:on:pending="loading"` |
| `lvt-el:removeClass:on:{trigger}` | Remove class(es) on trigger | `lvt-el:removeClass:on:done="loading"` |
| `lvt-el:toggleClass:on:{trigger}` | Toggle class(es) on trigger | `lvt-el:toggleClass:on:click="active"` |
| `lvt-el:setAttr:on:{trigger}` | Set attribute on trigger | `lvt-el:setAttr:on:pending="aria-busy:true"` |
| `lvt-el:toggleAttr:on:{trigger}` | Toggle boolean attr on trigger | `lvt-el:toggleAttr:on:pending="disabled"` |

**Note:** `{trigger}` can be a lifecycle state (`pending`, `success`, `error`, `done`), any native DOM event (`click`, `focusin`, `focusout`, `mouseenter`, `mouseleave`, etc.), or the synthetic `click-away`. For action-specific: `lvt-el:reset:on:create-todo:success`.

### Modifier Attributes (`lvt-mod:`)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-key` | Filter keyboard events by key | `lvt-key="Enter"` |
| `lvt-mod:debounce` | Debounce delay in milliseconds | `lvt-mod:debounce="300"` |
| `lvt-mod:throttle` | Throttle interval in milliseconds | `lvt-mod:throttle="100"` |

### Form Attributes (`lvt-form:`, `lvt-nav:`)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-form:action` | Explicit action routing on form | `<form lvt-form:action="checkout">` |
| `lvt-form:preserve` | Keep form values after submit | `<form lvt-form:preserve>` |
| `lvt-form:disable-with` | Button text during submit | `lvt-form:disable-with="Saving..."` |
| `lvt-form:no-intercept` | Opt-out of form interception | `<form lvt-form:no-intercept>` |
| `lvt-nav:no-intercept` | Opt-out of link interception | `<a lvt-nav:no-intercept>` |

### Directive Attributes (`lvt-fx:`)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-fx:scroll` | Scroll behavior | `lvt-fx:scroll="bottom"` |
| `lvt-fx:highlight` | Highlight effect | `lvt-fx:highlight="flash"` |
| `lvt-fx:animate` | Entrance animation | `lvt-fx:animate="fade"` |

Directives use CSS custom properties for configuration: `--lvt-scroll-behavior`, `--lvt-scroll-threshold`, `--lvt-highlight-color`, `--lvt-highlight-duration`, `--lvt-animate-duration`.

### Upload Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-upload` | File upload identifier | `lvt-upload="avatar"` |

### Preservation Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-ignore` | Skip this element and its entire subtree during morphdom diff. Checked on the live DOM (`fromEl`), usable from both templates and client JS. Equivalent to Phoenix LiveView's `phx-update="ignore"` | `<div lvt-ignore class="map-widget">` |
| `lvt-ignore-attrs` | Skip attribute diffing but still diff children. Preserves client-set attributes (e.g. `open` on `<details>`) while keeping child content server-managed | `<details lvt-ignore-attrs>` |
| `data-lvt-force-update` | Override all preservation (automatic and `lvt-ignore`); server value wins. Client strips it after processing; server re-sends it each render | `<input type="checkbox" data-lvt-force-update>` |

### Identity Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `data-key` | Stable element identity for the diff engine and morphdom matching. In `{{range}}` templates, controls which items are updated in-place vs. removed/inserted. On singleton elements, helps morphdom match nodes across updates. Hardcoded keys are valid for singletons; use template expressions (`{{.ID}}`) in lists | `<dialog data-key="settings-dialog">` |

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
<button name="save"
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
<button name="save"
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
