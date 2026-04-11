# README Rewrite Proposal

**Status:** Proposed
**Issue:** [#268](https://github.com/livetemplate/livetemplate/issues/268)

## TL;DR

**Problem:** The README is 353 lines, prose-heavy, and buries LiveTemplate's unique differentiator under a "Pure Go" framing. A first-time reader scrolls past ~80 lines of text before seeing any code.

**Solution:** Rewrite to ~160 lines. Lead with the reactive demo SVG + a standard HTML code example. Move detailed content (limitations table, benchmarks, verbose feature sections) to existing docs. Create a new `docs/guides/standard-html-reactivity.md` guide to absorb the removed philosophical content.

**Key reframing:** "Reactive UIs in Pure Go" → "Reactive web UIs in standard HTML and Go." Standard HTML is the unique hook — no other reactive framework makes plain `<form>` and `<button name="action">` interactive without custom attributes.

---

## Problem Statement

### Current README issues

1. **Too dense for first-time readers.** 353 lines with five verbose "Why LiveTemplate?" subsections, each containing multi-paragraph prose and code examples. The reader encounters ~80 lines of text before any code.

2. **"A Better Way" and "Why LiveTemplate" overlap.** Both sections pitch the same value proposition with different framing, creating a wall of text.

3. **Wrong framing.** "Reactive UIs in Pure Go" targets Go developers specifically. But the unique selling point — writing reactive UIs in *standard HTML* — appeals to anyone who knows HTML. A PHP developer, a Rails developer, or a designer can look at `<button name="add">Add</button>` and immediately understand it.

4. **Too much "tell," not enough "show."** Comparison tables, benchmark numbers, and feature explanations dominate. The user's prior feedback: "lead with GIFs/code, not tables/prose; move comparisons to docs."

5. **Content duplication.** The limitations table, performance benchmarks, controller pattern, and error handling examples all have comprehensive coverage in `docs/references/` and `docs/performance/`. The README repeats them in abbreviated form.

6. **SVG contradicts the pitch.** `assets/demo.svg` (the first visual a visitor sees) shows `lvt-click="increment"` — a Tier 2 custom attribute. If the pitch is "reactive UIs in standard HTML, no custom attributes needed," the hero visual can't contradict that.

### What competitor READMEs do well

| Project | Length | Hook strategy | First code at |
|---------|--------|--------------|--------------|
| **htmx** | Medium | Prose-first: "access AJAX directly in HTML" | ~20 lines |
| **Livewire** | Short | One sentence + immediate docs link | N/A (links out) |
| **LiveView** | Medium | Value prop + video demo | Video at ~5 lines |
| **Templ** | Short | Headline + GIF demo + docs link | GIF at ~5 lines |
| **Gomponents** | Medium | Problem-oriented: "Tired of complex template languages?" | ~15 lines |

Common patterns: prose-first hook establishing the problem, visual or code within the first 15 lines, link to external docs quickly, keep under ~200 lines.

---

## The "Standard HTML" Differentiator

Research confirms that **no other reactive framework makes standard HTML interactive without custom attributes**:

| Framework | Form markup | Custom attributes required |
|-----------|------------|--------------------------|
| **htmx** | `<form hx-post="/todos" hx-target="#list">` | `hx-post`, `hx-target`, `hx-swap`, `hx-trigger` |
| **Laravel Livewire** | `<form wire:submit="add">` | `wire:submit`, `wire:model`, `wire:click` |
| **Phoenix LiveView** | `<form phx-submit="add">` | `phx-submit`, `phx-click`, `phx-change` |
| **LiveTemplate** | `<form method="POST">` | None for core interactions |

LiveTemplate's `<button name="add">` routing to `Add()` uses standard HTML form semantics. No framework vocabulary needed for forms, buttons, dialogs, or links. Only Tier 2 behaviors (debounce, keyboard shortcuts, reactive DOM) require `lvt-*` attributes.

### Tradeoffs of the standard HTML approach

**Advantages:**
- Standard HTML works at all transport levels (no-JS POST, fetch DOM-patch, WebSocket real-time)
- No framework vocabulary to learn for common interactions
- Progressive enhancement works out of the box
- Less markup — no attribute ceremony per element

**Disadvantages:**
- Less visual distinction between reactive and static elements in templates
- Action routing via button `name` is less explicit than URL-based routing (htmx) or named events (LiveView)
- Server round-trip for every interaction (no client-side optimistic updates)

---

## Proposed New README Structure

Target: ~160 lines (down from 353).

### Section breakdown

| # | Section | ~Lines | What it contains |
|---|---------|--------|-----------------|
| 1 | Header + nav + alpha | 10 | New tagline, nav links, one-line alpha notice |
| 2 | Hero SVG + code + paragraph | 35 | `demo.svg`, todo form HTML + Go `Add()` method, one explanatory paragraph |
| 3 | How It Works (mermaid) | 15 | Existing mermaid sequence diagram (moved up), one sentence |
| 4 | Quick Start (counter) | 40 | Existing counter example, slightly trimmed |
| 5 | Features (bullet summaries) | 25 | Five features, 2-3 lines each, no code blocks |
| 6 | Learn More | 20 | Three groups: Guides, References, Related Projects |
| 7 | Contributing + License | 10 | Unchanged |

### Narrative arc

1. **What is it** (tagline) — Reactive web UIs in standard HTML and Go
2. **See it** (SVG) — Visual demo of the reactive flow
3. **See the code** (HTML + Go) — That's just a plain HTML form and a Go method
4. **How it works** (mermaid) — Button click → server method → tree diff → minimal JSON
5. **Build one** (Quick Start) — Copy-paste counter in 3 steps
6. **What else** (Features) — Five one-liners with doc links
7. **Go deeper** (Learn More) — Organized doc links

The reader sees code within ~20 lines of scrolling, before any prose explanation.

---

## Draft: New README.md

```markdown
# LiveTemplate

Reactive web UIs in standard HTML and Go. No custom template language. No client-side framework. No persistent connection required.

**[Quick Start](#quick-start)** | **[Docs](docs/)** | **[Examples](https://github.com/livetemplate/examples)** | **[API Reference](https://pkg.go.dev/github.com/livetemplate/livetemplate)**

> **Alpha** — Core features work and are tested, but the API may change before v1.0.

<p align="center">
  <img src="assets/demo.svg" alt="LiveTemplate reactive update flow — click a button, server updates state, only changed value sent to browser" width="720">
</p>

The HTML and Go behind a reactive todo list:

​```html
<form method="POST">
    <input type="text" name="title" required placeholder="What needs to be done?">
    <button name="add">Add Todo</button>
</form>
<ul>
{{range .Items}}<li>{{.Title}}</li>{{end}}
</ul>
​```

​```go
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
    if err := ctx.ValidateForm(); err != nil {
        return state, err
    }
    state.Items = append(state.Items, Todo{Title: ctx.GetString("title")})
    ctx.BroadcastAction("Refresh", nil) // other tabs see the change
    return state, nil
}
​```

The button's `name` IS the action — `<button name="add">` routes to `Add()`. No custom attributes, no JavaScript wiring. Without JS, the form POSTs normally. With the JS client, the DOM patches in place. Add a WebSocket and other tabs sync automatically. See [Standard HTML Reactivity](docs/guides/standard-html-reactivity.md) for how this compares to htmx, Livewire, and LiveView.

## How It Works

​```mermaid
sequenceDiagram
    participant Browser
    participant Server

    Browser->>Server: User clicks button<br/>{action: "increment"}
    Note over Server: s.Counter++<br/>(Counter: 5 → 6)
    Note over Server: Tree diff calculated<br/>Only Counter changed → {"0": "6"}
    Server->>Browser: {"0": "6"}
    Note over Browser: DOM updated<br/>Counter: 6
​```

When a user clicks a button, LiveTemplate calls a method on your Go struct, diffs the template output, and sends only what changed.

## Quick Start

​```bash
go get github.com/livetemplate/livetemplate
​```

**1. Define controller and state** ([full example](https://github.com/livetemplate/examples/blob/main/counter/main.go))

​```go
type CounterState struct {
    Counter int
}

type CounterController struct{}

func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Counter++
    return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
    state.Counter--
    return state, nil
}

func main() {
    controller := &CounterController{}
    state := &CounterState{Counter: 0}
    tmpl := livetemplate.New("counter")
    http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(state)))
    http.ListenAndServe(":8080", nil)
}
​```

**2. Write the template** ([counter.tmpl](https://github.com/livetemplate/examples/blob/main/counter/counter.tmpl))

​```html
<h1>Counter: {{.Counter}}</h1>
<form method="POST" style="display:inline">
    <button name="increment">+</button>
    <button name="decrement">-</button>
</form>

<script src="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/dist/livetemplate-client.browser.js"></script>
​```

**3. Run it**

​```bash
go run main.go  # Open http://localhost:8080
​```

## Features

**Standard HTML first** — Forms, buttons, dialogs, and links work reactively without custom attributes. `lvt-*` attributes are available for behaviors HTML can't express (debounce, keyboard shortcuts, reactive DOM). [Guide →](docs/guides/progressive-complexity.md)

**Safe state management** — Controllers (singleton, holds dependencies) are separated from state (pure data, cloned per session). No accidental data leakage between users. [Reference →](docs/references/controller-pattern.md)

**Efficient updates** — Templates split into static structure (cached) and dynamic values. Updates send only changed values — typically 85%+ bandwidth savings. [Details →](docs/performance/performance-characteristics.md)

**Idiomatic Go errors** — Actions return `(State, error)`. Validation errors flow to templates automatically. No error serialization code. [Error handling →](docs/references/error-handling.md)

**Code generation** — `lvt new myapp && lvt gen products name price:float` generates complete CRUD apps with reactive UIs. [CLI →](https://github.com/livetemplate/lvt)

## Learn More

**Guides:**

- [Progressive Complexity](docs/guides/progressive-complexity.md) — Standard HTML → `lvt-*` attributes
- [Standard HTML Reactivity](docs/guides/standard-html-reactivity.md) — How LiveTemplate compares to htmx, Livewire, LiveView
- [Scaling](docs/guides/SCALING.md) — Redis-backed sessions, horizontal scaling

**References:**

- [Controller+State Pattern](docs/references/controller-pattern.md) — Core architecture
- [Client Attributes](docs/references/client-attributes.md) — `lvt-*` reference
- [Error Handling](docs/references/error-handling.md) — Validation and errors
- [Configuration](docs/references/CONFIGURATION.md) — Options and environment variables
- [Current Limitations](docs/references/current-limitations.md) — Known gaps and workarounds

**Related Projects:**

- [CLI Tool (lvt)](https://github.com/livetemplate/lvt) — Code generator and dev server
- [Client Library](https://github.com/livetemplate/client) — TypeScript client (npm: `@livetemplate/client`)
- [Examples](https://github.com/livetemplate/examples) — Counter, Todos, Chat, and more

## Contributing

**New to the codebase?** Start with the [Contributor Walkthrough](docs/guides/new-contributor-walkthrough.md).

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License — see [LICENSE](LICENSE) file for details.
```

---

## Draft: New `docs/guides/standard-html-reactivity.md`

This new guide absorbs the philosophical content removed from the README.

```markdown
# Standard HTML Reactivity

LiveTemplate makes standard HTML reactive by default. A plain `<form method="POST">`
with `<button name="add">` is interactive at every transport level — no framework-specific
attributes required. This guide explains how it works, how it compares to other frameworks,
and the tradeoffs involved.

---

## Why Standard HTML?

Every interactive feature in a traditional web app requires the same ceremony: design a REST
endpoint, write a serializer, manage client-side state, update the DOM, and wire it all together.
That overhead discourages interactivity — teams leave things static not because they should be,
but because the plumbing isn't worth it. As Chris McCord [put it](https://fly.io/blog/how-we-got-to-liveview/)
when explaining why he built Phoenix LiveView: conventional frameworks make you "fetch the world,
munge it into some format, and shoot it over the wire... then throw all that state away" on every request.

LiveView's answer was to keep all state on the server and push rendered updates over a persistent
connection. LiveTemplate brings that approach to Go, with one major difference: it works equally well
over standard HTTP. And it goes a step further — the HTML itself needs no framework-specific attributes
for core interactions.

## How It Works

### Button Name = Action Routing

The `name` attribute on a button routes to a Go method:

    <button name="add">Add</button>       <!-- routes to Add() -->
    <button name="delete">Delete</button>  <!-- routes to Delete() -->

This uses standard HTML semantics — the button `name` is included in form data on submit.
LiveTemplate reads it and dispatches to the matching method. No custom attributes needed.

### Form Auto-Interception

All `<form>` elements inside a LiveTemplate handler are automatically intercepted:

- **Without JavaScript**: The form submits as a standard POST. The server uses Post-Redirect-Get
  (PRG) — redirects on success, re-renders with errors on validation failure.
- **With JavaScript (fetch)**: The JS client intercepts the submit, sends via `fetch()`, and
  patches the DOM with the response. No page reload.
- **With JavaScript (WebSocket)**: Actions are sent over the WebSocket connection for real-time updates.

The same HTML works identically across all three modes.

### Validation Inference

HTML validation attributes become server-side rules:

    <input type="email" name="Email" required minlength="5">

`ctx.ValidateForm()` checks these constraints server-side. For production use,
`ctx.BindAndValidate()` with Go struct tags is the recommended approach.

---

## Multi-User Broadcast

When one user's action should be visible to others, use `BroadcastAction`:

    func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
        state.Items = append(state.Items, Todo{Title: ctx.GetString("title")})
        ctx.BroadcastAction("Refresh", nil)
        return state, nil
    }

    func (c *TodoController) Refresh(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
        state.Items = c.loadItems()
        return state, nil
    }

Broadcast is scoped to the session group. For multi-instance deployments, add Redis pub/sub:

    tmpl, _ := livetemplate.New("app",
        livetemplate.WithPubSubBroadcaster(redisBroadcaster),
    )

See [PubSub Reference](../references/pubsub.md) for details.

---

## Comparison with Other Frameworks

Every major reactive framework requires custom attributes on HTML elements. LiveTemplate is
unique in making standard HTML reactive without modification.

| Framework | Form markup required | Custom attributes |
|-----------|---------------------|-------------------|
| **htmx** | `<form hx-post="/todos" hx-target="#list">` | `hx-post`, `hx-target`, `hx-swap`, `hx-trigger` |
| **Laravel Livewire** | `<form wire:submit="add">` | `wire:submit`, `wire:model`, `wire:click` |
| **Phoenix LiveView** | `<form phx-submit="add">` | `phx-submit`, `phx-click`, `phx-change` |
| **LiveTemplate** | `<form method="POST">` | None for standard interactions |

### htmx

htmx extends HTML with `hx-*` attributes for AJAX interactions. A form without `hx-post`
submits normally (full page reload). Every interactive element needs explicit `hx-*` attributes.

### Laravel Livewire

Livewire uses `wire:*` directives in PHP/Blade templates. `wire:submit` captures form
submissions, `wire:model` enables two-way binding. State is serialized into HTML attributes.

### Phoenix LiveView

LiveView uses `phx-*` attributes and requires a persistent WebSocket connection. Forms need
`phx-submit` to route actions. The initial page renders as static HTML, then upgrades to WebSocket.

### LiveTemplate

Standard HTML forms work reactively without any framework attributes. The button `name` routes
to a Go method, form data is available via `ctx.GetString()`, and the response is a minimal
tree diff. WebSocket is optional — only needed for server-initiated broadcasts.

---

## Progressive Complexity

LiveTemplate follows a two-tier model:

| Tier | What you write | When to use |
|------|---------------|-------------|
| **Tier 1: Standard HTML** | `<form>`, `<button name="add">`, `<dialog>`, `<a href>` | Forms, actions, modals, navigation |
| **Tier 2: `lvt-*` attributes** | `lvt-debounce`, `lvt-key`, `lvt-addClass-on:pending` | Timing, keyboard shortcuts, reactive DOM |

Tier 2 is only for behaviors standard HTML cannot express. For example, debounced search
requires `lvt-debounce` because HTML has no timing mechanism:

    <input name="Query" value="{{.Query}}"
        lvt-input="search" lvt-debounce="300"
        placeholder="Search...">

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
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Quick-lookup table
- [Controller+State Pattern](../references/controller-pattern.md) — Core architecture pattern
- [Examples](https://github.com/livetemplate/examples) — Counter, Todos, Chat, and more
```

---

## Content Migration Map

Content removed from the README doesn't disappear — it moves to existing docs where it has better context.

| README content | Current location | Destination | Action |
|---------------|-----------------|-------------|--------|
| "A Better Way" philosophy (Chris McCord quote, LiveView comparison) | Lines 19-35 | `docs/guides/standard-html-reactivity.md` § "Why Standard HTML?" | New section in new file |
| Tier table + debounce example | Lines 43-89 | Already in `docs/guides/progressive-complexity.md` | No action (already covered) |
| Controller/State code example | Lines 92-118 | Already in `docs/references/controller-pattern.md` | No action |
| Static/dynamic JSON examples | Lines 122-134 | Already in `docs/performance/performance-characteristics.md` | No action |
| Error handling code example | Lines 136-163 | Already in `docs/references/error-handling.md` | No action |
| Phoenix LiveView comparison table | Lines 178-193 | `docs/references/current-limitations.md` | Add new "Phoenix LiveView Feature Comparison" section at top |
| Benchmark metrics table | Lines 276-284 | `docs/performance/performance-characteristics.md` | Add "Summary" section at top |
| Benchmark commands (`make bench`, etc.) | Lines 294-304 | `docs/performance/performance-characteristics.md` | Add to summary section |
| "How It Works" numbered list | Lines 260-269 | `docs/guides/standard-html-reactivity.md` | Absorbed into guide |

---

## SVG Fix: `assets/demo.svg`

The demo SVG currently shows Tier 2 syntax that contradicts the "standard HTML" pitch:

| Line | Current | Proposed |
|------|---------|----------|
| 241 | `<button lvt-click="increment">+</button>` | `<button name="increment">+</button>` |
| 242 | `<button lvt-click="decrement">−</button>` | `<button name="decrement">−</button>` |
| 292 | `"<button lvt-click=..."` | `"<button name=..."` |

These are text edits inside SVG `<text>` elements. No layout change.

---

## Changes to `docs/README.md`

Add link to the new guide in the Guides section:

```markdown
- **[Standard HTML Reactivity](guides/standard-html-reactivity.md)** — Why standard HTML is LiveTemplate's key differentiator
```

---

## Implementation Checklist

When this proposal is approved and ready for implementation:

1. [ ] Fix `assets/demo.svg` — 3 text edits
2. [ ] Create `docs/guides/standard-html-reactivity.md`
3. [ ] Add LiveView comparison table to `docs/references/current-limitations.md`
4. [ ] Add benchmark summary to `docs/performance/performance-characteristics.md`
5. [ ] Update `docs/README.md` with new guide link
6. [ ] Rewrite `README.md`
7. [ ] Verify all doc links resolve
8. [ ] Run `go test -v ./... -timeout=30s`
