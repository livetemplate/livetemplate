# Progressive Complexity Guide

LiveTemplate follows a two-tier progressive complexity model:

- **Tier 1: Standard HTML** — forms, buttons, links, dialogs, validation. No custom attributes.
- **Tier 2: `lvt-*` Attributes** — debounce, reactive DOM, lifecycle hooks. Only when HTML can't express it.

This guide walks through Tier 1 from the simplest case to full-featured applications.

---

## 1. Your First Form

A form inside a LiveTemplate handler just works. No `lvt-*` attributes, no hidden fields, no special setup:

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

Buttons with a `name` attribute work as actions even outside any `<form>` (requires JS client — fetch or WebSocket):

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

> **No-JS fallback:** For progressive enhancement without JavaScript, wrap buttons in a `<form method="POST">` instead (see [Section 2](#2-multiple-actions-with-button-names)).

---

## 4. Validation from HTML Attributes

> **Note:** The form schema is auto-wired from your template's HTML attributes, so `ctx.ValidateForm()` works without calling `WithFormSchema` manually. For production validation with custom rules, use `ctx.BindAndValidate()` with struct tags.

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

Use `formnovalidate` on a submit button to skip `ctx.ValidateForm()` for that
submit path — the canonical "save draft" flow:

```html
<button name="save">Save</button>
<button name="save-draft" formnovalidate>Save Draft</button>
```

The framework reads the `formnovalidate` button's `name` from your template and
skips validation when that button is the submitter — on every tier (WebSocket,
HTTP-fetch, and no-JS native POST). Three things to know:

- **Name it.** The skip is matched by the submitter's `name`; a dynamic
  (`{{...}}`) name can't be detected.
- **No-JS: no `value`.** On the pure no-JS tier the server identifies the
  submitter by its empty-value form field, so a `formnovalidate` button that
  carries a `value` won't be recognized as the submitter there (JS tiers send an
  explicit submitter and are unaffected).
- **Not a security boundary.** Skipping is client-controlled convenience for
  draft flows — enforce server-authoritative rules unconditionally where it
  matters.

---

## 5. Dialogs

Use the standard `<dialog>` element with `command`/`commandfor` for native modal dialogs:

```html
<!-- Open button -->
<button command="show-modal" commandfor="edit-dialog">Edit</button>

<!-- Dialog with form -->
<dialog id="edit-dialog">
    <form name="save">
        <h2>Edit Item</h2>
        <input name="title" value="{{.Title}}">
        <input type="hidden" name="id" value="{{.ID}}">

        <button type="submit">Save</button>
        <button type="button" command="close" commandfor="edit-dialog">Cancel</button>
    </form>
</dialog>
```

- `command="show-modal"` opens the dialog via `.showModal()` — backdrop, focus trapping, and Escape key handling are all native to `<dialog>`
- `command="close"` closes it via `.close()`
- The client automatically closes any parent `<dialog>` when a form submission succeeds — so the dialog stays open for validation errors but closes on success
- The LiveTemplate client polyfills `command`/`commandfor` for browsers that don't yet support the [Invoker Commands API](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/command) natively (Firefox, Safari). The polyfill uses feature detection (`commandForElement`) and becomes a no-op when browsers add native support

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
<a href="/legacy-page" lvt-nav:no-intercept>Old Page</a>       <!-- explicit opt-out -->
```

---

## 7. Loading States

LiveTemplate offers three loading models. Pick the simplest one that fits your use case:

| I want… | Accepts `lvt-*` attrs? | Path | Go boilerplate |
|---|---|---|---|
| Grey out the form | N/A | **7.1 — Auto** (`<fieldset>` + CSS) | 0 lines, 0 attrs |
| Custom loading UX (spinner, text) | Yes | **7.2 — Client-owned pending** (`lvt-el:*:on:pending`) | 0 lines, 2 attrs |
| Custom loading UX (spinner, text) | **No** | **7.3 — Server-owned loading** with `Async` + `{{.lvt.Pending}}` | ~5 lines, 0 attrs |
| Loading fans out to peers / survives reconnect | Either | **7.3 — Server-owned loading** with `Async` | ~7 lines, 0 attrs |

**Rule of thumb:** start with 7.1. Move to 7.2 or 7.3 only when you need custom UX (spinners, text changes, progress indicators) or loading that is real application state.

### 7.1 Automatic Form Loading (Tier 1)

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

No `lvt-*` attributes needed, no Go code. The `<fieldset>` wrapping is the signal. This covers the "grey out the whole form" case. For custom loading UX beyond grey-out, see 7.2 or 7.3.

### 7.2 Client-Owned Pending (Tier 2)

When you need custom loading UX (spinners, text changes, visual feedback) and are willing to use `lvt-*` attributes, `lvt-el:*:on:pending` gives you instant feedback with zero Go code:

```html
<button name="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:addClass:on:pending="opacity-50"
    lvt-el:removeClass:on:done="opacity-50">
    Save
</button>
```

```go
// Just the business logic — no loading scaffolding
func (c *Controller) Save(state State, ctx *livetemplate.Context) (State, error) {
    time.Sleep(700 * time.Millisecond) // simulate slow work
    state.Name = ctx.GetString("Name")
    return state, nil
}
```

The `pending` state fires instantly on click (before the server even receives the message) and clears on `done`. This is the most concise option for custom loading UX.

**Trade-offs:** the pending state is client-only — it does not fan out to peer tabs, does not survive reconnect, and cannot drive server-side logic. The action blocks the event loop for its duration (no other clicks or peer pushes until it returns). For loading that is real application state, use 7.3.

See the [Client Attributes Reference — Reactive Attributes](../references/client-attributes.md#reactive-attributes) for the full `lvt-el:*` pattern.

### 7.3 Server-Owned Loading (Tier 1)

When loading is real application state — it needs to fan out to peers, survive reconnect, or drive server-side logic — use a server-owned `Loading` field in your state with `{{if .Loading}}` in the template. This keeps everything in Tier 1 (no `lvt-*` attributes), but requires the manual two-action pattern:

```go
type State struct {
    Name    string
    Loading bool
}

// Action 1: set Loading=true, spawn the slow work off the event loop
func (c *Controller) Greet(state State, ctx *livetemplate.Context) (State, error) {
    if state.Loading {
        return state, nil // re-entrancy guard: ignore clicks while loading
    }
    session := ctx.Session()
    if session == nil {
        return state, nil // nil check: no session on initial HTTP render
    }
    name := strings.TrimSpace(ctx.GetString("name"))
    state.Loading = true
    go func() {
        time.Sleep(700 * time.Millisecond) // simulate slow work
        _ = session.TriggerAction("finishGreet", map[string]any{"name": name})
    }()
    return state, nil // render #1: spinner on
}

// Action 2: clear Loading, apply the result
func (c *Controller) FinishGreet(state State, ctx *livetemplate.Context) (State, error) {
    state.Name = ctx.GetString("name")
    state.Loading = false
    return state, nil // render #2: spinner off
}
```

```html
<button name="greet" {{if .Loading}}disabled{{end}}>
    {{if .Loading}}Loading...{{else}}Greet{{end}}
</button>
```

**Why two methods?** LiveTemplate's event loop processes one action per render cycle. To show a spinner *and later* clear it, the slow work must return early (render #1: spinner on) and re-enter the event loop via `Session.TriggerAction` (render #2: spinner off).

**Four things to get right:**

1. **Re-entrancy guard** (`if state.Loading { return }`): prevents double-clicks from spawning duplicate goroutines. The `{{if .Loading}}disabled{{end}}` on the button is the UI-level guard; the Go check is the server-level backup.
2. **Session nil-check** (`if session == nil`): `ctx.Session()` returns nil during initial HTTP renders (before WebSocket connects). The goroutine + `TriggerAction` pattern requires a live session.
3. **Goroutine lifetime**: the goroutine should be short-lived. If the connection drops, `TriggerAction` returns `ErrSessionDisconnected` — the goroutine exits cleanly.
4. **Second action name**: the `FinishGreet` method name must match the string passed to `TriggerAction`. A typo silently fails (the action dispatches but no method handles it).

**Trade-offs vs 7.2:** more verbose (~15 lines across 2 methods), but the loading state is real server state — it fans out to peers via `ctx.Publish()`, survives reconnect (it's in the state struct), and can drive server-side logic (e.g., preventing concurrent operations).

#### Simplified with `Async`

`livetemplate.Async` collapses the two-method pattern to one method (~7 lines) by handling the goroutine, dispatch channel, and re-entry automatically:

```go
func (c *Controller) Greet(state State, ctx *livetemplate.Context) (State, error) {
    state.Loading = true
    name := strings.TrimSpace(ctx.GetString("name"))
    livetemplate.Async(ctx,
        func(ctx context.Context) (string, error) {
            time.Sleep(700 * time.Millisecond) // simulate slow work
            return name, nil
        },
        func(s State, name string, err error) (State, error) {
            s.Name = name
            s.Loading = false
            return s, nil
        },
    )
    return state, nil // render #1: Loading=true
    // render #2 happens when work completes: Loading=false
}
```

The template is identical — `{{if .Loading}}` works the same way. The key guarantees:
- **`apply` sees the current state**, not a snapshot — any actions that ran during the async window are visible
- **Connection-scoped** — only the originating connection gets the completion render
- **Lifetime-bound** — if the connection closes, the goroutine is cancelled and `apply` is skipped

See the [Async API reference](../references/api-reference.md#async) for the full contract.

#### Zero-boilerplate with `{{.lvt.Pending}}`

When loading is purely visual (no need for a `Loading` field in state), combine `Async` with the framework-provided `{{.lvt.Pending}}` template variable. It is `true` on the render that registered async work and `false` on all other renders (including the async completion render):

```go
func (c *Controller) Greet(state State, ctx *livetemplate.Context) (State, error) {
    name := strings.TrimSpace(ctx.GetString("name"))
    livetemplate.Async(ctx,
        func(ctx context.Context) (string, error) {
            time.Sleep(700 * time.Millisecond)
            return name, nil
        },
        func(s State, name string, err error) (State, error) {
            s.Name = name
            return s, nil
        },
    )
    return state, nil
}
```

```html
<button name="greet" {{if .lvt.Pending}}disabled{{end}}>
    {{if .lvt.Pending}}Loading...{{else}}Greet{{end}}
</button>
```

No `Loading` field, no `lvt-*` attributes — pure Go + standard HTML templates. Use `{{.lvt.Pending}}` when the loading indicator is chrome (visual feedback). Use an explicit `Loading` state field (with `Async`) when loading is real application state that needs to fan out to peers or survive reconnect.

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

Override the default debounce per input with `lvt-mod:debounce`:

```html
<input name="Name" value="{{.Name}}" lvt-mod:debounce="500">
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

## 12. Progressive Enhancement

LiveTemplate supports three transport layers that degrade gracefully: **WebSocket** → **fetch (HTTP)** → **no-JS (POST + page reload)**. All Tier 1 features (Sections 1-11) work across all three layers. This is controlled by the `ProgressiveEnhancement` config flag (default: `true`).

### No JavaScript (POST + Page Reload)

When JavaScript is unavailable, forms submit as standard HTML POST requests. The server uses the **Post-Redirect-Get (PRG)** pattern:

1. User submits `<form method="POST">`
2. Server processes the action, updates state
3. **On success**: 303 redirect back to the same URL — prevents duplicate submissions on browser refresh. Flash messages are passed via a short-lived `lvt-flash` cookie (10-second max-age, consumed immediately on the next GET)
4. **On validation error**: Re-renders the page inline with errors — no redirect, so error messages appear next to the relevant fields

This is why all Tier 1 examples use `<form method="POST">` — they work without JavaScript by design.

> **Security note:** When using no-JS POST mode, implement CSRF protection (e.g., [`gorilla/csrf`](https://github.com/gorilla/csrf) or equivalent CSRF middleware). The JS transport modes send the `Origin` header that the server validates on WebSocket upgrade and fetch; plain HTML form POST does not carry the same protection.

### JavaScript + HTTP (fetch)

When JavaScript is available but WebSocket is not connected, the JS client intercepts form submissions and sends them via `fetch()`. The server responds with a JSON tree update, and the client patches the DOM. No page reload occurs.

This transport is also used as the automatic fallback when a WebSocket connection disconnects.

### JavaScript + WebSocket

Full bidirectional communication. Actions are sent as WebSocket messages, and the server can push updates at any time. Server push (`Session.TriggerAction()`) and peer fan-out (`ctx.Publish()` on a subscribed topic) reach peer tabs only in this mode.

### How the Server Detects Transport

The server determines the client's transport from the HTTP request:

| Signal | Transport | Response |
|--------|-----------|----------|
| WebSocket upgrade header | WebSocket | Upgrade to WebSocket, send JSON trees |
| `Accept: application/json` | fetch (JS client) | JSON tree update |
| Standard browser `Accept: text/html` | No JS | Full HTML page (PRG pattern for POST) |

### What Works at Each Level

For a complete feature-by-transport breakdown, see the [Transport Compatibility table](../references/progressive-complexity-reference.md#transport-compatibility) in the reference doc.

### Disabling Progressive Enhancement

```go
tmpl := livetemplate.New("app",
    livetemplate.WithProgressiveEnhancement(false),
)
```

When disabled, POST requests from non-JS browsers return JSON instead of HTML. Only disable this if all clients have JavaScript.

---

## 13. Tier 2: `lvt-*` Attributes

Use `lvt-*` attributes only when standard HTML cannot express the behavior. For the complete attribute reference, see the [Client Attributes Reference](../references/client-attributes.md).

### 13.1 Event Bindings Outside Forms

For interactions outside the form submit lifecycle — hover effects, focus/blur tracking:

```html
<!-- Server-rendered tooltip on hover (use CSS :hover for static tooltips instead) -->
<div lvt-on:mouseenter="showTooltip" lvt-on:mouseleave="hideTooltip">
    {{.Label}}
    {{if .TooltipVisible}}<span class="tooltip">{{.TooltipText}}</span>{{end}}
</div>
```

> **Prefer Tier 1 when possible:** For buttons that trigger actions, use `<form>` + `<button name="action" value="save">` instead of `lvt-on:click`. See [Section 2](#2-multiple-actions-with-button-names).

See [Client Attributes Reference — Event Bindings](../references/client-attributes.md#event-bindings) for the full list of `lvt-on:{event}` bindings.

### 13.2 Rate Limiting

HTML has no mechanism for debounce or throttle. **Debounce** waits until the user stops (ideal for typing). **Throttle** limits frequency (ideal for scroll/resize). Both are essential for search inputs and scroll handlers:

```html
<!-- Wait 300ms after user stops typing -->
<input lvt-on:input="search" lvt-mod:debounce="300" name="query" placeholder="Search...">

<!-- Fire scroll handler at most once per 100ms -->
<div lvt-on:window:scroll="loadMore" lvt-mod:throttle="100">...</div>
```

See [Client Attributes Reference — Rate Limiting](../references/client-attributes.md#rate-limiting) for details.

### 13.3 Keyboard Shortcuts

Filter events by key and listen at the window level for global shortcuts:

```html
<!-- Submit search on Enter key only -->
<input lvt-on:keydown="submitSearch" lvt-key="Enter" name="query">

<!-- Global Escape key to close modal -->
<div lvt-on:window:keydown="closeModal" lvt-key="Escape">
    Modal content...
</div>
```

See [Client Attributes Reference — Keyboard Events](../references/client-attributes.md#keyboard-events) for valid key values.

### 13.4 Reactive DOM

Declarative DOM mutations tied to the action lifecycle (`pending`, `success`, `error`, `done`):

```html
<!-- Button with loading state -->
<button name="save"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:addClass:on:pending="opacity-50"
    lvt-el:removeClass:on:done="opacity-50">
    Save
</button>

<!-- Reset form after successful submission -->
<form method="POST" lvt-el:reset:on:success>
    <input name="title" placeholder="New todo">
    <button type="submit">Add</button>
</form>
```

Available reactive actions: `lvt-el:addClass:on:*`, `lvt-el:removeClass:on:*`, `lvt-el:toggleClass:on:*`, `lvt-el:setAttr:on:*`, `lvt-el:toggleAttr:on:*`, `lvt-el:reset:on:*`.

See [Client Attributes Reference — Reactive Attributes](../references/client-attributes.md#reactive-attributes) for the full pattern.

### 13.5 Directives

Declarative UI behaviors for scroll management, visual feedback, and animations. Configuration uses CSS custom properties (defaults provided by `livetemplate.css`):

```html
<!-- Chat messages: auto-scroll to bottom, stick if user is near bottom -->
<div lvt-fx:scroll="bottom-sticky" style="--lvt-scroll-threshold: 100px" class="chat-messages">
    {{range .Messages}}
        <div>{{.Text}}</div>
    {{end}}
</div>

<!-- Preserve scroll position across updates (e.g., search results) -->
<div lvt-fx:scroll="preserve" class="results">
    {{range .Results}}
        <div>{{.Title}}</div>
    {{end}}
</div>

<!-- Highlight updated items -->
<div lvt-fx:highlight="flash">{{.UpdatedContent}}</div>

<!-- Fade in new content -->
<div lvt-fx:animate="fade">{{.NewContent}}</div>
```

Scroll modes: `bottom` (always scroll to bottom), `bottom-sticky` (scroll only if user is near bottom), `top` (scroll to top), `preserve` (maintain current scroll position across updates).

CSS custom properties: `--lvt-scroll-behavior`, `--lvt-scroll-threshold`, `--lvt-highlight-color`, `--lvt-highlight-duration`, `--lvt-animate-duration`.

Directives also support lifecycle and DOM event triggers via `:on:` syntax. Without `:on:`, the directive fires on every DOM content update. With `:on:{state}`, it fires on a lifecycle state. With `:on:{event}`, it fires on a native DOM event:

```html
<!-- Highlight on successful save action -->
<div lvt-fx:highlight:on:save:success="flash">Save confirmed</div>

<!-- Highlight on click (DOM event trigger, no server round-trip) -->
<div lvt-fx:highlight:on:click="flash">Click to highlight</div>
```

See [Client Attributes Reference — Directives](../references/client-attributes.md#directives) for all scroll, highlight, and animation options.

### 13.6 Complete Tier 2 Example

A search interface combining debounced input, loading states, keyboard shortcuts, and scroll preservation.

> **`Change()` vs `lvt-input`:** Use `Change()` ([Section 10](#10-live-updates)) when you want generic live-update on all form inputs — no `lvt-*` needed. Use `lvt-input` when you need per-element control: a specific action name, custom debounce, or only some inputs triggering server calls.

```html
<h1>Search</h1>

<!-- Global Escape key clears the search -->
<div lvt-on:window:keydown="clearSearch" lvt-key="Escape">

    <!-- lvt-on:input fires directly without a form — Tier 2 event binding -->
    <input name="Query" value="{{.Query}}"
        lvt-on:input="search" lvt-mod:debounce="300"
        lvt-el:addClass:on:pending="border-blue-500"
        lvt-el:removeClass:on:done="border-blue-500"
        placeholder="Type to search...">

    <form method="POST">
        <button name="clearSearch"
            lvt-el:toggleAttr:on:pending="disabled">
            Clear
        </button>
    </form>

    <div class="results" lvt-fx:scroll="preserve">
        {{if .Query}}
            <p>{{len .Results}} results for "{{.Query}}"</p>
        {{end}}
        {{range .Results}}
            <div data-key="{{.ID}}" lvt-fx:animate="fade">
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

See also: [Progressive Complexity Reference](../references/progressive-complexity-reference.md) for a quick-lookup table of HTML attributes and their framework behaviors, [Client Attributes Reference](../references/client-attributes.md) for the complete `lvt-*` attribute listing, and [Ephemeral Components Guide](ephemeral-components.md) for implementing client-side toast/alert patterns.
