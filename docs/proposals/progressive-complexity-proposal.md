# Progressive Complexity Model

**Status:** Accepted (supersedes lvt-bind-proposal.md)
**Date:** 2026-03-24

## Summary

LiveTemplate should offer a path from simplicity of implicit inference to explicit declarations. A standard Go HTML template without any `lvt-*` attributes should be sufficient to build simple to moderate complexity. Users add explicit `lvt-*` declarations only for behaviors that HTML cannot express.

Two key insights drive this proposal:

1. **Template expressions ARE binding declarations.** `<input name="Title" value="{{.Title}}">` already declares that input "Title" is bound to state field `.Title`. No `lvt-bind` attribute needed.
2. **Standard HTML already routes actions.** `<button name="action" value="delete">` works at all three transport levels (no-JS POST, fetch, WebSocket) without duplication. No `lvt-submit` needed.

This supersedes the `lvt-bind` proposal which introduced a new message type, new method signature, and new reflection code (~1150 lines) while still requiring an explicit attribute.

## Two Dimensions of Progressive Complexity

### Dimension 1: Template Complexity

| Tier | What You Write | Examples |
|------|---------------|---------|
| **Tier 1: Standard HTML** | Forms, buttons, hidden inputs — no `lvt-*` | Auto-submit → `Submit()`, `button name="action"`, `form name`, `Change()` |
| **Tier 2: `lvt-*` Attributes** | Custom attributes for non-HTML behaviors | `lvt-debounce`, `lvt-disable-with`, `lvt-key`, reactive DOM, hooks, window events |

### Dimension 2: Transport Capability

| Mode | Transport | Response |
|------|-----------|----------|
| **No JS** | HTML POST | Full page reload (PRG pattern) |
| **JS + HTTP** | `fetch()` POST | JSON tree update → DOM patch |
| **JS + WebSocket** | WebSocket | JSON tree update + server push |

Standard HTML (`button name="action"`) is the only approach that works at all three levels without duplication. The current `lvt-submit` + hidden `lvt-action` approach requires duplicated routing declarations.

## Tier 1: Standard HTML

A plain HTML form within a LiveTemplate-managed region just works.

### Template

```html
<h1>User Profile</h1>
<form method="POST">
    <label>Name</label>
    <input type="text" name="Name" value="{{.Name}}" required>
    {{if .lvt.HasError "name"}}
        <span class="error">{{.lvt.Error "name"}}</span>
    {{end}}

    <label>Email</label>
    <input type="email" name="Email" value="{{.Email}}" required>
    {{if .lvt.HasError "email"}}
        <span class="error">{{.lvt.Error "email"}}</span>
    {{end}}

    <label>Bio</label>
    <textarea name="Bio">{{.Bio}}</textarea>

    <!-- Live preview updates as user types (inferred from {{.Name}}, {{.Email}}) -->
    <div class="preview">
        <p>Hello, {{.Name}}!</p>
        <p>Email: {{.Email}}</p>
    </div>

    <button type="submit">Save</button>
</form>
```

### Server

```go
type ProfileState struct {
    Name  string
    Email string
    Bio   string
}

// Submit: auto-routed when form has no lvt-submit attribute
func (c *ProfileController) Submit(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
    var input struct {
        Name  string `validate:"required,min=2"`
        Email string `validate:"required,email"`
        Bio   string
    }
    if err := ctx.BindAndValidate(&input, validate); err != nil {
        return state, err
    }
    c.DB.SaveProfile(input.Name, input.Email, input.Bio)
    return state, nil
}

// Change: auto-routed for inferred bindings (optional)
// If absent, live change tracking is disabled (submit-only mode)
func (c *ProfileController) Change(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
    if ctx.Has("Name") { state.Name = ctx.GetString("Name") }
    if ctx.Has("Email") { state.Email = ctx.GetString("Email") }
    if ctx.Has("Bio") { state.Bio = ctx.GetString("Bio") }
    return state, nil
}
```

### How It Works

| Transport | Behavior |
|-----------|----------|
| **No JS** | POST → server defaults empty action to `"submit"` → `Submit()` → PRG redirect |
| **JS + HTTP** | Client intercepts form → fetch POST → JSON tree update → DOM patch |
| **JS + WebSocket** | Client intercepts form → WS message → DOM patch + live change tracking |

The client auto-intercepts all `<form>` elements in the LiveTemplate wrapper div. Opt out with `lvt-no-intercept` on a form.

When you need multiple forms or specific action routing, still use standard HTML.

### Action Routing via Button

```html
<form method="POST">
    <input type="text" name="Title" value="{{.Title}}">
    <button type="submit" name="action" value="save">Save</button>
    <button type="submit" name="action" value="save-draft">Save Draft</button>
</form>
```

The clicked button's `name="action"` value becomes the action name. Routes to `Save()` or `SaveDraft()`.

### Action Routing via Form Name

```html
<form name="search" method="POST">
    <input name="Query" value="{{.Query}}">
    <button type="submit">Search</button>
</form>
```

Form `name` attribute becomes the action. Routes to `Search()`.

### Data Passing via Hidden Inputs

```html
{{range .Items}}
<li data-key="{{.ID}}">
    {{.Title}}
    <form method="POST">
        <input type="hidden" name="id" value="{{.ID}}">
        <button type="submit" name="action" value="toggle">
            {{if .Done}}Undo{{else}}Done{{end}}
        </button>
        <button type="submit" name="action" value="delete">Delete</button>
    </form>
</li>
{{end}}
```

Hidden inputs and `data-*` attributes on the submit button are included in action data.

### Action Resolution Priority

1. Button `name="action"` + `value="X"` → action is X
2. Form `name="X"` → action is X
3. None → default `""` → server defaults to `"submit"`
4. `lvt-submit="X"` → backward compat, overrides all above

### What Already Works Today

`parseURLEncodedForm()` in `internal/send/message.go` already checks `r.FormValue("action")` as a fallback. `<button name="action" value="delete">` already routes to `Delete()` on the HTTP POST path with zero code changes.

## Tier 2: `lvt-*` Attributes

Reserved for behaviors standard HTML cannot express:

| Behavior | Attribute | Why HTML Can't Do It |
|----------|-----------|---------------------|
| Debounce/throttle | `lvt-debounce`, `lvt-throttle` | Timing control |
| Loading states | `lvt-disable-with` | Text swap + disable combo |
| Key filtering | `lvt-key` | Action-specific key matching |
| Reactive DOM | `lvt-addClass-on:pending` | Lifecycle-driven mutations |
| Lifecycle hooks | `lvt-hook` | JS library integration |
| Click outside | `lvt-click-away` | Non-standard event |
| Window events | `lvt-window-*` | Global event → action routing |
| Scroll directives | `lvt-scroll` | Scroll position management |
| Confirmation | `lvt-confirm` | Pre-action dialog |

### What Moves OUT of `lvt-*`

| Old Pattern | Standard HTML Replacement |
|-------------|--------------------------|
| `lvt-submit="save"` | `<form name="save">` or `<button name="action" value="save">` |
| `lvt-click="delete" lvt-data-id="X"` | `<form><input type="hidden" name="id" value="X"><button name="action" value="delete">` |
| `lvt-data-*="X"` | `data-*="X"` on submit button, or hidden `<input>` |

Existing `lvt-*` attributes continue to work (backward compatible).

Standard HTML routing and `lvt-*` reactive behavior can be combined freely:

```html
<button type="submit" name="action" value="save"
    lvt-disable-on:pending
    lvt-addClass-on:pending="opacity-50"
    lvt-enable-on:done>
    Save
</button>
```

## Binding Inference

The tree's statics contain the information needed for inference:

```
Template: <input name="Title" value="{{.Title}}">
Statics:  ["<input name=\"Title\" value=\"", "\">"]
Dynamic:  [0] = evaluated value of .Title
```

The client scans statics for `name="X"` adjacent to dynamic value slots. If detected, the input is "bound" — change events are debounced (300ms default) and sent to the server as `{action: "change", data: {Title: "new value"}}`.

The server dispatches `"change"` to `Change()` via existing `methodNameToActions()`. If no `Change()` method exists, `ErrMethodNotFound` is silently ignored.

### Edge Cases

- `<textarea name="Bio">{{.Bio}}</textarea>` → bound (dynamic inside element)
- `<input type="checkbox" name="Active" {{if .Active}}checked{{end}}>` → bound (conditional attribute)
- `<input name="csrf">` (no template expression) → NOT bound
- Explicit `lvt-change` on an input → overrides inferred binding

## Compatibility Matrix

| Feature | No JS | JS + HTTP | JS + WebSocket |
|---------|-------|-----------|----------------|
| Form submit (Tier 1) | POST + PRG | fetch + DOM patch | WS + DOM patch |
| `button name="action"` (Tier 1) | Native POST | Client extracts | Client extracts |
| `form name` (Tier 1) | N/A (use button) | Client reads | Client reads |
| Hidden inputs (Tier 1) | Native POST | In FormData | In FormData |
| Inferred bindings (Tier 1) | N/A | fetch per change | WS per change |
| `lvt-*` attributes (Tier 2) | N/A | Works | Works |
| Server push / broadcast | N/A | N/A | Works |

Tier 1 degrades gracefully to no-JS. Tier 2 requires JS but is additive.

## Implementation Phases

### Phase 1: Form Auto-Submit + Standard HTML Routing

**Server (~6 lines in `mount.go`):** Default empty action to `"submit"` in both WebSocket and HTTP POST handlers.

**Client (~120 lines):** Auto-intercept forms in wrapper scope. Resolve action from submitter button `name="action"` or `form.name`. Collect `data-*` from submitter. Send via existing transport.

### Phase 2: Inferred Bindings

**Client (~150 lines):** Analyze statics for `name="X" value="` + dynamic slot patterns. Attach debounced change listeners. Send to `Change()` method.

**Server: zero changes.** `"change"` already maps to `Change()` via `methodNameToActions()`.

### Phase 3: Auto-Apply (Future)

**Server (~100 lines):** If no `Submit()`/`Change()` method, auto-apply form fields to State struct via reflection. Opt-in via `livetemplate.WithAutoApply()`.

## Design Decisions

1. **Default action `"submit"`**: `methodNameToActions("Submit")` → `["submit", "Submit"]`. Natural convention.
2. **`Change()` is optional**: No `Change()` = submit-only mode. Adding it enables live updates with zero template changes.
3. **Opt-out via `lvt-no-intercept`**: Forms with external `action` URLs or `lvt-no-intercept` are not auto-intercepted.
4. **Explicit overrides inference**: `lvt-change="validate"` on an input prevents inferred binding from firing.
5. **300ms debounce default**: Matches Phoenix LiveView. Override with `lvt-debounce="500"`.

## References

- **Turbo Drive (Hotwire)**: Intercepts all forms — fully implicit, inspiration for Tier 1 auto-intercept
- **Remix**: Progressive enhancement — forms work without JS
- **Phoenix LiveView**: `phx-submit`/`phx-change` — explicit only, contrast point
- **HTMX**: `hx-boost` for implicit interception — similar opt-in concept
