# Standard HTML Reactivity

LiveTemplate makes standard HTML reactive by default. A plain `<form method="POST">` with `<button name="add">` is interactive at every transport level — no framework-specific attributes required. This guide explains how it works, how it compares to other frameworks, and the tradeoffs involved.

---

## How It Works

### Button Name = Action Routing

The `name` attribute on a button routes to a Go method:

```html
<button name="add">Add</button>       <!-- routes to Add() -->
<button name="delete">Delete</button> <!-- routes to Delete() -->
```

This uses standard HTML semantics — the button `name` is included in form data on submit. LiveTemplate reads it and dispatches to the matching method. No custom attributes needed.

### Form Auto-Interception

All `<form>` elements inside a LiveTemplate handler are automatically intercepted:

- **Without JavaScript**: The form submits as a standard POST. The server uses Post-Redirect-Get (PRG) — redirects on success, re-renders with errors on validation failure.
- **With JavaScript (fetch)**: The JS client intercepts the submit, sends via `fetch()`, and patches the DOM with the response. No page reload.
- **With JavaScript (WebSocket)**: Actions are sent over the WebSocket connection for real-time updates.

The same HTML works identically across all three modes.

### Validation Inference

HTML validation attributes become server-side rules:

```html
<input type="email" name="Email" required minlength="5">
```

When wired up with `ctx.WithFormSchema(ExtractFormSchema(statics))`, calling `ctx.ValidateForm()` checks these constraints server-side. For production use, `ctx.BindAndValidate()` with Go struct tags is recommended.

---

## Multi-User Broadcast

When one user's action should be visible to others, use `BroadcastAction`:

```go
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Items = append(state.Items, Todo{Title: ctx.GetString("title")})
    ctx.BroadcastAction("Refresh", nil)  // pushes to all other tabs in the session group
    return state, nil
}

func (c *TodoController) Refresh(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Items = c.loadItems()  // each tab refreshes its own state
    return state, nil
}
```

Broadcast is scoped to the session group — users in other groups don't receive it. For multi-instance deployments, add Redis pub/sub with one config option:

```go
tmpl, _ := livetemplate.New("app",
    livetemplate.WithPubSubBroadcaster(redisBroadcaster),
)
```

See [PubSub Reference](../references/pubsub.md) for details.

---

## Comparison with Other Frameworks

Every major reactive framework requires custom attributes on HTML elements. LiveTemplate is unique in making standard HTML reactive without modification.

| Framework | Form markup required | Transport | Custom attributes |
|-----------|---------------------|-----------|-------------------|
| **htmx** | `<form hx-post="/todos" hx-target="#list" hx-swap="beforeend">` | HTTP (AJAX) | `hx-post`, `hx-target`, `hx-swap`, `hx-trigger`, ... |
| **Laravel Livewire** | `<form wire:submit="add">` | HTTP (stateless re-hydration) | `wire:submit`, `wire:model`, `wire:click`, ... |
| **Phoenix LiveView** | `<form phx-submit="add">` | WebSocket (persistent) | `phx-submit`, `phx-click`, `phx-change`, ... |
| **LiveTemplate** | `<form method="POST">` | HTTP, fetch, or WebSocket | None for standard interactions |

### htmx

htmx extends HTML with `hx-*` attributes that enable AJAX interactions. A form without `hx-post` submits normally (full page reload). Every interactive element needs explicit `hx-*` attributes to opt in to reactivity.

### Laravel Livewire

Livewire uses `wire:*` directives in PHP/Blade templates. `wire:submit` captures form submissions, `wire:model` enables two-way data binding. State is serialized into HTML attributes and re-hydrated on each request.

### Phoenix LiveView

LiveView uses `phx-*` attributes and requires a persistent WebSocket connection for all interactions. Forms need `phx-submit` to route actions. The initial page renders as static HTML, then LiveView upgrades to WebSocket for subsequent interactions.

### LiveTemplate

Standard HTML forms work reactively without any framework attributes. The button `name` routes to a Go method, form data is available via `ctx.GetString()`, and the response is a minimal tree diff. WebSocket is optional — only needed for server-initiated broadcasts.

---

## Progressive Complexity

LiveTemplate follows a two-tier model:

| Tier | What you write | When to use |
|------|---------------|-------------|
| **Tier 1: Standard HTML** | `<form>`, `<button name="add">`, `<dialog>`, `<a href>` | Forms, actions, modals, navigation |
| **Tier 2: `lvt-*` attributes** | `lvt-debounce`, `lvt-key`, `lvt-addClass-on:pending` | Timing, keyboard shortcuts, reactive DOM |

Tier 2 is only for behaviors standard HTML cannot express. For example, debounced search requires `lvt-debounce` because HTML has no timing mechanism:

```html
<input name="Query" value="{{.Query}}"
    lvt-input="search" lvt-debounce="300"
    placeholder="Search...">
```

See the [Progressive Complexity Guide](progressive-complexity.md) for the complete walkthrough.

---

## Tradeoffs

| Approach | Philosophy | Clarity | Flexibility |
|----------|-----------|---------|-------------|
| **Custom attributes** (htmx, Livewire, LiveView) | Explicit is better than implicit | High — clear what's reactive | High — opt-in reactivity |
| **Standard HTML** (LiveTemplate) | Make the common case simple | Lower — everything is reactive | Lower — opt-out via `lvt-no-intercept` |

**Advantages of LiveTemplate's approach:**
- Standard HTML works at all transport levels (no-JS, fetch, WebSocket)
- No framework vocabulary to learn for common interactions
- Progressive enhancement works out of the box
- Less markup to write

**Disadvantages:**
- Less visual distinction between reactive and static elements
- Harder to tell at a glance which elements trigger server actions
- Action routing via button `name` is less explicit than URL-based routing

---

## See Also

- [Progressive Complexity Guide](progressive-complexity.md) — Full walkthrough from standard HTML to `lvt-*` attributes
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Quick-lookup table for HTML → framework behavior
- [Controller+State Pattern](../references/controller-pattern.md) — Core architecture pattern
- [Examples](https://github.com/livetemplate/examples) — Counter, Todos, Chat, and more
