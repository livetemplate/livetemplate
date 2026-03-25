# Progressive Complexity Guide

LiveTemplate follows a two-tier progressive complexity model:

- **Tier 1: Standard HTML** — forms, buttons, links, dialogs, validation. No custom attributes.
- **Tier 2: `lvt-*` Attributes** — debounce, reactive DOM, lifecycle hooks. Only when HTML can't express it.

This guide walks through Tier 1 from the simplest case to full-featured applications.

---

## 1. Your First Form

A form inside a LiveTemplate handler just works. No `lvt-submit`, no hidden fields, no special attributes:

```html
<form method="POST">
    <input type="text" name="Title" placeholder="New todo...">
    <button type="submit">Add</button>
</form>
```

```go
func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    title := ctx.GetString("Title")
    state.Items = append(state.Items, Todo{Title: title})
    return state, nil
}
```

**What happens:** The framework auto-intercepts all forms. When no action is specified, it routes to `Submit()`. This works at all three transport levels: no-JS (POST + page reload), fetch (DOM patch), and WebSocket.

---

## 2. Multiple Actions with Button Names

When a form needs multiple actions, the button `name` IS the action:

```html
<form method="POST">
    <input type="text" name="Title" value="{{.Title}}">
    <button name="save">Save</button>
    <button name="save-draft" formnovalidate>Save Draft</button>
</form>
```

```go
func (c *Controller) Save(state State, ctx *livetemplate.Context) (State, error) {
    // Validated save
    return state, nil
}

func (c *Controller) SaveDraft(state State, ctx *livetemplate.Context) (State, error) {
    // Save without validation (formnovalidate skips HTML validation)
    return state, nil
}
```

The clicked button's `name` determines which method is called. Button `value` becomes data:

```html
{{range .Items}}
<form method="POST">
    <input type="hidden" name="id" value="{{.ID}}">
    <span>{{.Title}}</span>
    <button name="toggle">{{if .Done}}Undo{{else}}Done{{end}}</button>
    <button name="delete" value="{{.ID}}">Delete</button>
</form>
{{end}}
```

```go
func (c *Controller) Toggle(state State, ctx *livetemplate.Context) (State, error) {
    id := ctx.GetString("id")  // from hidden input
    // toggle item...
    return state, nil
}

func (c *Controller) Delete(state State, ctx *livetemplate.Context) (State, error) {
    id := ctx.GetString("value")  // from button value
    // delete item...
    return state, nil
}
```

---

## 3. Standalone Buttons

Buttons outside a form can reference the auto-injected hidden form using the standard HTML `form` attribute. The framework injects `<form id="{wrapperID}-form" method="POST" hidden>` at the top of every page.

```html
<h1>Counter: {{.Counter}}</h1>
<button form="lvt-abc123-form" name="increment">+</button>
<button form="lvt-abc123-form" name="decrement">-</button>
```

The wrapper ID is available in the rendered HTML's `data-lvt-id` attribute.

---

## 4. Validation from HTML Attributes

> **Note:** Auto-wiring the form schema from template statics is not yet implemented. Currently you must call `ctx.WithFormSchema(ExtractFormSchema(statics))` manually. For production validation, use `ctx.BindAndValidate()` with struct tags. `formnovalidate` on buttons is not yet respected server-side.

HTML validation attributes (`required`, `pattern`, `min`, `max`, `minlength`, `maxlength`, `type`) can be extracted by the framework. Use `ctx.ValidateForm()` instead of writing Go struct tags:

```html
<form method="POST">
    <input type="email" name="Email" required minlength="5" maxlength="100">
    {{if .lvt.HasError "email"}}
        <span class="error">{{.lvt.Error "email"}}</span>
    {{end}}

    <input type="number" name="Age" min="18" max="120">
    {{if .lvt.HasError "age"}}
        <span class="error">{{.lvt.Error "age"}}</span>
    {{end}}

    <input type="text" name="Code" pattern="[A-Z]{3}">

    <button type="submit">Submit</button>
</form>
```

```go
func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    if err := ctx.ValidateForm(); err != nil {
        return state, err  // Errors auto-displayed via .lvt.HasError/.lvt.Error
    }
    // All fields valid
    state.Email = ctx.GetString("Email")
    state.Age = ctx.GetInt("Age")
    return state, nil
}
```

No Go struct tags needed. The `required`, `type="email"`, `minlength="5"`, `min="18"` attributes are the validation rules.

Use `formnovalidate` on buttons that should skip validation:

```html
<button type="submit">Save</button>
<button name="save-draft" formnovalidate>Save Draft</button>
```

---

## 5. Dialogs

Use the standard `<dialog>` element with `command`/`commandfor` for native modal dialogs:

```html
<!-- Open button -->
<button command="show-modal" commandfor="edit-dialog">Edit</button>

<!-- Dialog with form -->
<dialog id="edit-dialog">
    <form method="dialog">
        <h2>Edit Item</h2>
        <input name="title" value="{{.Title}}">

        <button name="save" value="{{.ID}}">Save</button>
        <button command="close" commandfor="edit-dialog">Cancel</button>
    </form>
</dialog>
```

- `command="show-modal"` opens the dialog (native browser behavior)
- `command="close"` closes it (native)
- `method="dialog"` on the form closes the dialog AND routes the action to the server
- Focus trapping, backdrop, and Escape key handling are all native

---

## 6. Navigation

Links inside the LiveTemplate wrapper are auto-intercepted for SPA navigation:

```html
<nav>
    <a href="/todos">Todos</a>
    <a href="/profile">Profile</a>
    <a href="/settings">Settings</a>
</nav>
```

The framework fetches the page via `fetch()`, extracts the wrapper content, and replaces the DOM. No full page reload. Browser history (`pushState`) is updated automatically.

**Opt-out** for links that should navigate normally:

```html
<a href="/api/export.csv" download>Export</a>            <!-- download attr: skipped -->
<a href="https://external.com">External</a>              <!-- different origin: skipped -->
<a href="/legacy-page" lvt-no-intercept>Old Page</a>      <!-- explicit opt-out -->
```

---

## 7. Loading States

During form submission, the framework automatically:

1. Sets `aria-busy="true"` on the form
2. Disables `<fieldset>` elements inside the form (if present)
3. Clears both when the server responds

```html
<form method="POST">
    <fieldset>
        <input name="title">
        <button type="submit">Save</button>
    </fieldset>
</form>

<style>
    form[aria-busy="true"] fieldset {
        opacity: 0.5;
        pointer-events: none;
    }
</style>
```

No `lvt-*` attributes needed. The `<fieldset>` wrapping is the signal.

---

## 8. Confirmation

Use standard `onsubmit` for confirmation dialogs:

```html
<form method="POST" onsubmit="return confirm('Delete this item?')">
    <input type="hidden" name="id" value="{{.ID}}">
    <button name="delete">Delete</button>
</form>
```

---

## 9. Expand/Collapse

Use native `<details>` and `<summary>`:

```html
<details>
    <summary>Advanced Options</summary>
    <div>
        <input name="advanced_setting" value="{{.AdvancedSetting}}">
    </div>
</details>
```

Works without JavaScript. Keyboard accessible by default.

---

## 10. Live Updates

> **Coming soon:** This feature requires inferred bindings (Phase 2), which is not yet implemented. The `Change()` convention is designed but the client-side binding inference is pending..

Add a `Change()` method to your controller to enable live updates as the user types:

```html
<form method="POST">
    <input name="Name" value="{{.Name}}">
    <div class="preview">Hello, {{.Name}}!</div>
    <button type="submit">Save</button>
</form>
```

```go
func (c *Controller) Change(state State, ctx *livetemplate.Context) (State, error) {
    if ctx.Has("Name") { state.Name = ctx.GetString("Name") }
    return state, nil
}
```

The preview updates live as the user types. If no `Change()` method exists, the form is submit-only.

---

## 11. When to Use Tier 2 (`lvt-*`)

Use `lvt-*` attributes only for behaviors standard HTML cannot express:

| Need | Tier 2 Attribute | Why HTML Can't |
|------|-----------------|----------------|
| Wait for typing pause | `lvt-debounce="300"` | Timing control |
| Limit event rate | `lvt-throttle="100"` | Rate limiting |
| Filter by key | `lvt-key="Enter"` | Key-specific routing |
| Lifecycle DOM changes | `lvt-addClass-on:pending="loading"` | State-driven mutations |
| Global keyboard shortcuts | `lvt-window-keydown="shortcut"` | Window-level events |
| JS library integration | `lvt-hook="chart"` | Lifecycle callbacks |
| Sticky scroll | `lvt-scroll="bottom-sticky"` | Threshold logic |

```html
<!-- Tier 2: debounced live search (timing not expressible in HTML) -->
<input name="Query" value="{{.Query}}" lvt-input="search" lvt-debounce="300">

<!-- Tier 2: loading indicator with class toggle -->
<button name="save"
    lvt-disable-on:pending
    lvt-addClass-on:pending="opacity-50"
    lvt-enable-on:done>
    Save
</button>
```

---

## 12. Complete Example

A todo app using Tier 1 only (zero `lvt-*` attributes):

```html
<h1>Todos ({{.ActiveCount}} remaining)</h1>

<form method="POST">
    <input type="text" name="Title" required minlength="1" placeholder="New todo...">
    {{if .lvt.HasError "title"}}
        <span class="error">{{.lvt.Error "title"}}</span>
    {{end}}
    <button type="submit">Add</button>
</form>

<ul>
{{range .FilteredItems}}
    <li data-key="{{.ID}}">
        <form method="POST">
            <input type="hidden" name="id" value="{{.ID}}">
            <span>{{.Title}}</span>
            <button name="toggle">{{if .Done}}Undo{{else}}Done{{end}}</button>
            <button name="delete">Delete</button>
        </form>
    </li>
{{end}}
</ul>

<form name="filter" method="POST">
    <button name="filter" value="all">All</button>
    <button name="filter" value="active">Active</button>
    <button name="filter" value="done">Done</button>
</form>
```

```go
func (c *TodoController) Submit(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    if err := ctx.ValidateForm(); err != nil {
        return state, err
    }
    state.Items = append(state.Items, Todo{ID: uuid.New(), Title: ctx.GetString("Title")})
    return state, nil
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // toggle by ctx.GetString("id")
    return state, nil
}

func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    // delete by ctx.GetString("id")
    return state, nil
}

func (c *TodoController) Filter(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.ActiveFilter = ctx.GetString("filter")
    return state, nil
}
```
