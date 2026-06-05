# Standard HTML Reactivity

LiveTemplate makes standard HTML reactive by default. A plain `<form method="POST">` with `<button name="add">` is interactive at every transport level — no framework-specific attributes required. This guide explains how it works, how it compares to other frameworks, and the tradeoffs involved.

> **Recent reinforcement:** As of client v0.8.38, the TypeScript client and the generated templates went through a deliberate "attribute reduction" pass that removed `lvt-*` attributes from anything HTML can already express. Tier 1 standard HTML is now the default everywhere; Tier 2 attributes are reserved for behaviors HTML genuinely cannot express (timing, keyboard shortcuts, reactive DOM).

---

## Why Standard HTML?

Every interactive feature in a traditional web app requires the same ceremony: design a REST endpoint, write a serializer, manage client-side state, update the DOM, and wire it all together. That overhead discourages interactivity — teams leave things static not because they should be, but because the plumbing isn't worth it. As Chris McCord [put it](https://fly.io/blog/how-we-got-to-liveview/) when explaining why he built Phoenix LiveView: conventional frameworks make you "fetch the world, munge it into some format, and shoot it over the wire... then throw all that state away" on every request.

LiveView's answer was to keep all state on the server and push rendered updates over a persistent connection. LiveTemplate brings that approach to Go, with one major difference: it works equally well over standard HTTP. And it goes a step further — the HTML itself needs no framework-specific attributes for core interactions.

## How It Works

### Button Name = Action Routing

The `name` attribute on a button routes to a Go method:

```html
<button name="add">Add</button>       <!-- routes to Add() -->
<button name="delete">Delete</button>  <!-- routes to Delete() -->
```

This uses standard HTML semantics — the button `name` is included in form data on submit. LiveTemplate reads it and dispatches to the matching method. No custom attributes needed.

### Form Auto-Interception

All `<form>` elements inside a LiveTemplate handler are automatically intercepted:

- **Without JavaScript**: The form submits as a standard POST. The server uses Post-Redirect-Get (PRG) — redirects on success, re-renders with errors on validation failure.
- **With JavaScript (fetch)**: The JS client intercepts the submit, sends via `fetch()`, and patches the DOM with the response. No page reload.
- **With JavaScript (WebSocket)**: Actions are sent over the WebSocket connection for real-time updates.

The same HTML works identically across all three modes.

### Validation

For production form validation, use `ctx.BindAndValidate()` with Go struct tags:

```go
// validate is a *validator.Validate from github.com/go-playground/validator/v10,
// typically initialized once and stored on the controller.
var input struct {
    Email string `validate:"required,email,min=5"`
}
if err := ctx.BindAndValidate(&input, c.validate); err != nil {
    return state, err // field errors sent to template automatically
}
```

For HTML-attribute-based validation (`required`, `pattern`, `min`, `max`), see the [Error Handling reference](../references/error-handling.md) for the `ValidateForm` + `WithFormSchema` pattern.

---

## Multi-User Peer Fan-Out

When one user's action should be visible to other WebSocket-connected tabs, the pattern is two-step: each connection that wants peer updates opts in via `ctx.Subscribe(ctx.SelfTopic())` in `Mount`, and the action that mutated shared state fans out via `ctx.Publish(ctx.SelfTopic(), "Refresh", nil)`. Peer fan-out is opt-in — a connection that didn't subscribe receives nothing.

`SelfTopic()` resolves to `lvt:session:<groupID>` — the reserved-namespace topic for this session's own connections, ACL-exempt by construction. For app-wide announcements that should cross session boundaries, use a developer-defined topic (e.g. `"announcements"`) and admit it in your `WithTopicACL` ruleset.

Note: `Publish` must be called AFTER all state mutations and `ctx.With*()` calls. `With*()` creates shallow copies, and publishes queued before the copy are stranded on the pre-copy Context and never propagate.

```go
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    _ = ctx.Subscribe(ctx.SelfTopic()) // opt this connection in to peer fan-out
    return state, nil
}

func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Items = append(state.Items, Todo{Title: ctx.GetString("title")})
    // Publish after all state changes — pushes to subscribed peer tabs
    ctx.Publish(ctx.SelfTopic(), "Refresh", nil)
    return state, nil
}

func (c *TodoController) Refresh(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    state.Items = c.loadItems()
    return state, nil
}
```

Peer fan-out is scoped to the session group. For multi-instance deployments, add Redis pub/sub:

```go
tmpl, _ := livetemplate.New("app",
    livetemplate.WithPubSubBroadcaster(redisBroadcaster),
)
```

See [PubSub Reference](../references/pubsub.md) for details.

---

## Comparison with Other Frameworks

Every major reactive framework makes HTML reactive by adding a layer on top of it — custom attributes (`hx-*`, `wire:*`, `phx-*`) or a templating DSL.

LiveTemplate keeps the HTML standard and moves the reactivity to the server. You add an `lvt-*` attribute only when the behavior is something HTML itself cannot define — timing, keyboard shortcuts, reactive DOM — never to make ordinary HTML reactive.

The boundary is *what HTML can express*, not *how common the case is*.

| Framework | Markup for a form action | Attributes to make it reactive |
|-----------|--------------------------|--------------------------------|
| **htmx** | `<form hx-post="/todos">` | `hx-post`, `hx-target`, `hx-swap`, `hx-trigger` |
| **templ + htmx** | a `templ` component (Go DSL) rendering `<form hx-post="/todos">` | `hx-post`, `hx-target`, … (htmx still does the interactivity) |
| **Laravel Livewire** | `<form wire:submit="add">` (Blade) | `wire:submit`, `wire:model`, `wire:click` |
| **Phoenix LiveView** | `<form phx-submit="add">` (HEEx) | `phx-submit`, `phx-click`, `phx-change` |
| **LiveTemplate** | `<form method="POST">` (standard `html/template`) | None for standard interactions; `lvt-*` only for what HTML can't express |

### htmx

htmx extends HTML with `hx-*` attributes for AJAX interactions. A form without `hx-post` submits normally (full page reload). Every interactive element needs explicit `hx-*` attributes.

### templ + htmx

[templ](https://templ.guide) is a Go DSL for authoring and composing HTML as type-safe Go components — a popular alternative to `html/template`. It is a *templating* layer, not an interactivity layer, so it is commonly paired with htmx for reactivity. That means two things to learn and adopt: a new markup language **and** `hx-*` attributes on the rendered HTML.

LiveTemplate takes the opposite trade: it stays on Go's standard `html/template` (no new DSL) and provides the reactivity itself. You compose with what `html/template` already gives you — partials and the `{{template}}` action — plus per-session state and one render-and-diff pipeline, rather than adopting a new language for either authoring or interactivity. If you specifically want compile-time-checked, function-composed markup, templ is the better fit; if you want standard HTML to be reactive without a DSL or `hx-*` wiring, that's LiveTemplate.

### Laravel Livewire

Livewire uses `wire:*` directives in PHP/Blade templates. `wire:submit` captures form submissions, `wire:model` enables two-way binding. State is serialized into HTML attributes.

### Phoenix LiveView

LiveView uses `phx-*` attributes and requires a persistent WebSocket connection. Forms need `phx-submit` to route actions. The initial page renders as static HTML, then upgrades to WebSocket.

### LiveTemplate

Standard HTML forms work reactively without any framework attributes. The button `name` routes to a Go method, form data is available via `ctx.GetString()`, and the response is a minimal tree diff. WebSocket is optional — only needed for server-initiated publishes (peer fan-out).

---

## Feature Gap vs Phoenix LiveView

LiveTemplate is inspired by Phoenix LiveView but does not yet cover its full feature set. Tracked gaps as of v0.8.23:

| Feature | LiveView | LiveTemplate | Notes |
|---------|----------|-------------|-------|
| **Live Navigation** | `push_navigate`, `push_patch` | Partial — `__navigate__` action covers same-handler query-string navigation (no reconnect). Different-handler nav still falls back to fetch or full page load. | See [Navigate Action](../references/navigate.md). |
| **Stateful Components** | `LiveComponent` with own lifecycle | Stateless templates only | `{{template}}` invocations work but have no component-level state or event handling. |
| **Streams** | `stream/3` for large lists | Not yet | LiveView streams handle large/infinite lists without keeping all items in server memory. Streaming-range rendering (PRs #366/#368/#369/#370) is the latest step toward this. |
| **JS Commands** | `JS.push`, `JS.toggle`, `JS.show` | Partial | [`lvt-*` reactive attributes](../references/client-attributes.md) cover common cases (disable, add/remove class, set attribute) but aren't as composable as LiveView's server-defined JS chains. |
| **Client Hooks** | `phx-hook` lifecycle callbacks | Proposed | [`lvt-hook` proposal](../proposals/lifecycle-hooks-proposal.md) covers third-party JS library integration; not yet shipped. |
| **Presence** | `Phoenix.Presence` | Not built-in | Can be built on LiveTemplate's session stores; requires manual implementation. |
| **Testing Helpers** | `live/2`, `render_click/3` | Minimal | `AssertPureState` exists; no view-level test DSL. Browser tests use chromedp. |
| **Form Recovery** | Automatic on reconnect | Partial — `lvt-form:preserve` retains specific fields across re-renders | Full automatic recovery on WS reconnection is not yet built in. |

For day-to-day workarounds, see [Current Limitations](../references/current-limitations.md).

---

## Progressive Complexity

LiveTemplate follows a two-tier model:

| Tier | What you write | When to use |
|------|---------------|-------------|
| **Tier 1: Standard HTML** | `<form>`, `<button name="add">`, `<dialog>`, `<a href>` | Forms, actions, modals, navigation |
| **Tier 2: `lvt-*` attributes** | `lvt-on:`, `lvt-mod:debounce`, `lvt-el:`, `lvt-fx:` | Timing, keyboard shortcuts, reactive DOM |

Tier 2 is only for behaviors standard HTML cannot express. For example, debounced search requires `lvt-mod:debounce` because HTML has no timing mechanism:

```html
<input name="Query" value="{{.Query}}"
    lvt-on:input="search" lvt-mod:debounce="300"
    placeholder="Search...">
```

See the [Progressive Complexity Guide](progressive-complexity.md) for the complete walkthrough.

---

## Tradeoffs

| Approach | Philosophy | Clarity | Flexibility |
|----------|-----------|---------|-------------|
| **Custom attributes** (htmx, Livewire, LiveView) | Explicit is better than implicit | High — clear what's reactive | High — opt-in reactivity |
| **Standard HTML** (LiveTemplate) | Make the common case simple | Lower — everything is reactive | Lower — opt-out via `lvt-form:no-intercept` / `lvt-nav:no-intercept` |

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
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Quick-lookup table
- [Controller+State Pattern](../references/controller-pattern.md) — Core architecture pattern
- [Client Attributes](../references/client-attributes.md) — Complete `lvt-*` reference
- [Examples](https://github.com/livetemplate/examples) — Counter, Todos, Chat, and more
