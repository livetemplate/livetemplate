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

Buttons with a `name` attribute work as actions even outside any `<form>`:

```html
<h1>Counter: {{.Counter}}</h1>
<button name="increment">+</button>
<button name="decrement">-</button>
```

The button's `name` routes to the corresponding Go method. Button `value` and `data-*` attributes are sent as action data:

```html
<button name="delete" value="{{.ID}}">Delete</button>
<button name="edit" data-id="{{.ID}}" data-mode="quick">Quick Edit</button>
```

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

Add a `Change()` method to your controller to enable live updates as the user types — no `lvt-*` attributes needed:

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

**What happens:** The server detects the `Change()` method and sends `capabilities: ["change"]` in the initial render. The client auto-wires debounced input events (300ms default) on form fields with dynamic values. The preview updates live as the user types. If no `Change()` method exists, the form is submit-only.

Override the default debounce per input with `lvt-debounce`:

```html
<input name="Name" value="{{.Name}}" lvt-debounce="500">
```

---

## 11. Complete Tier 1 Example

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

---

## 12. Tier 2: `lvt-*` Attributes

Use `lvt-*` attributes only when standard HTML cannot express the behavior. For the complete attribute reference, see the [Client Attributes Reference](../references/client-attributes.md).

### 12.1 Event Bindings Outside Forms

For interactions outside the form submit lifecycle — hover effects, focus/blur tracking, click-away detection:

```html
<!-- Server-rendered tooltip on hover (use CSS :hover for static tooltips instead) -->
<div lvt-mouseenter="showTooltip" lvt-mouseleave="hideTooltip">
    {{.Label}}
    {{if .TooltipVisible}}<span class="tooltip">{{.TooltipText}}</span>{{end}}
</div>

<!-- Close dropdown when clicking outside -->
<div lvt-click-away="closeDropdown">
    {{if .DropdownOpen}}
        <ul>{{range .Options}}<li>{{.}}</li>{{end}}</ul>
    {{end}}
</div>
```

> **Prefer Tier 1 when possible:** For buttons that trigger actions, use `<form>` + `<button name="action">` instead of `lvt-click`. See [Section 2](#2-multiple-actions-with-button-names).

See [Client Attributes Reference — Event Bindings](../references/client-attributes.md#event-bindings) for the full list: `lvt-click`, `lvt-input`, `lvt-change`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-click-away`.

### 12.2 Rate Limiting

HTML has no mechanism for debounce or throttle. **Debounce** waits until the user stops (ideal for typing). **Throttle** limits frequency (ideal for scroll/resize). Both are essential for search inputs and scroll handlers:

```html
<!-- Wait 300ms after user stops typing -->
<input lvt-input="search" lvt-debounce="300" name="query" placeholder="Search...">

<!-- Fire scroll handler at most once per 100ms -->
<div lvt-window-scroll="loadMore" lvt-throttle="100">...</div>
```

See [Client Attributes Reference — Rate Limiting](../references/client-attributes.md#rate-limiting) for details.

### 12.3 Keyboard Shortcuts

Filter events by key and listen at the window level for global shortcuts:

```html
<!-- Submit search on Enter key only -->
<input lvt-keydown="submitSearch" lvt-key="Enter" name="query">

<!-- Global Escape key to close modal -->
<div lvt-window-keydown="closeModal" lvt-key="Escape">
    Modal content...
</div>
```

See [Client Attributes Reference — Keyboard Events](../references/client-attributes.md#keyboard-events) for valid key values.

### 12.4 Reactive DOM

Declarative DOM mutations tied to the action lifecycle (`pending`, `success`, `error`, `done`):

```html
<!-- Button with loading state -->
<button name="save"
    lvt-disable-on:pending
    lvt-addClass-on:pending="opacity-50"
    lvt-enable-on:done
    lvt-removeClass-on:done="opacity-50">
    Save
</button>

<!-- Reset form after successful submission -->
<form method="POST" lvt-reset-on:success>
    <input name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

Available reactive actions: `lvt-disable-on`, `lvt-enable-on`, `lvt-addClass-on`, `lvt-removeClass-on`, `lvt-toggleClass-on`, `lvt-setAttr-on`, `lvt-toggleAttr-on`, `lvt-reset-on`.

See [Client Attributes Reference — Reactive Attributes](../references/client-attributes.md#reactive-attributes) for the full pattern.

### 12.5 Directives

Declarative UI behaviors for scroll management, visual feedback, and animations:

```html
<!-- Chat messages: auto-scroll to bottom, stick if user is near bottom -->
<div lvt-scroll="bottom-sticky" lvt-scroll-threshold="100" class="chat-messages">
    {{range .Messages}}
        <div>{{.Text}}</div>
    {{end}}
</div>

<!-- Preserve scroll position across updates (e.g., search results) -->
<div lvt-scroll="preserve" class="results">
    {{range .Results}}
        <div>{{.Title}}</div>
    {{end}}
</div>

<!-- Highlight updated items -->
<div lvt-highlight="flash">{{.UpdatedContent}}</div>

<!-- Fade in new content -->
<div lvt-animate="fade">{{.NewContent}}</div>
```

Scroll modes: `bottom` (always scroll to bottom), `bottom-sticky` (scroll only if user is near bottom), `top` (scroll to top), `preserve` (maintain current scroll position across updates).

See [Client Attributes Reference — Directives](../references/client-attributes.md#directives) for all scroll, highlight, and animation options.

### 12.6 Complete Tier 2 Example

A search interface combining debounced input, loading states, keyboard shortcuts, and scroll preservation.

> **`Change()` vs `lvt-input`:** Use `Change()` ([Section 10](#10-live-updates)) when you want generic live-update on all form inputs — no `lvt-*` needed. Use `lvt-input` when you need per-element control: a specific action name, custom debounce, or only some inputs triggering server calls.

```html
<h1>Search</h1>

<!-- Global Escape key clears the search -->
<div lvt-window-keydown="clearSearch" lvt-key="Escape">

    <!-- lvt-input fires directly without a form — Tier 2 event binding -->
    <input name="Query" value="{{.Query}}"
        lvt-input="search" lvt-debounce="300"
        lvt-addClass-on:pending="border-blue-500"
        lvt-removeClass-on:done="border-blue-500"
        placeholder="Type to search...">

    <form method="POST">
        <button name="clearSearch"
            lvt-disable-on:pending>
            Clear
        </button>
    </form>

    <div class="results" lvt-scroll="preserve">
        {{if .Query}}
            <p>{{len .Results}} results for "{{.Query}}"</p>
        {{end}}
        {{range .Results}}
            <div data-key="{{.ID}}" lvt-animate="fade">
                <h3>{{.Title}}</h3>
                <p>{{.Summary}}</p>
            </div>
        {{end}}
    </div>

</div>
```

```go
type SearchController struct {
    DB *sql.DB
}

type SearchState struct {
    Query   string
    Results []Result
}

func (c *SearchController) Search(state SearchState, ctx *livetemplate.Context) (SearchState, error) {
    state.Query = ctx.GetString("Query")
    if state.Query == "" {
        state.Results = nil
        return state, nil
    }
    results, err := c.DB.Search(state.Query)
    if err != nil {
        return state, err
    }
    state.Results = results
    return state, nil
}

func (c *SearchController) ClearSearch(state SearchState, ctx *livetemplate.Context) (SearchState, error) {
    state.Query = ""
    state.Results = nil
    return state, nil
}
```

---

See also: [Progressive Complexity Reference](../references/progressive-complexity-reference.md) for a quick-lookup table of HTML attributes and their framework behaviors, and [Client Attributes Reference](../references/client-attributes.md) for the complete `lvt-*` attribute listing.
