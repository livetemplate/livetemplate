# Ephemeral Components Guide

Ephemeral components are UI elements that appear briefly, deliver information, and then disappear — toasts, banners, alerts, and confirmation flashes. They have no meaningful persistent state from the server's perspective.

They belong **entirely on the client**. Below is why, and how to build them that way.

---

## Why Not Put Them in the Diff Tree?

When a toast or alert is rendered in a LiveTemplate server template, it becomes part of the diff tree. That creates several problems:

- **Unnecessary diff traffic**: the toast HTML appears in every update cycle, even when it hasn't changed, because the server must track it in the tree
- **Server-driven dismissal**: to close a toast, the client must round-trip to the server
- **Stale DOM elements**: even with non-persistent fields (no `lvt:"persist"` tag), the DOM still carries stale toast elements between updates until the server explicitly clears them

The right model: the server **signals** the client; the client **creates and manages** the DOM.

---

## The Trigger-Attribute Pattern

The server renders a single hidden `<span>` with a `data-pending` attribute containing JSON:

```html
<span
  data-toast-trigger="notifications"
  data-pending='[{"id":"1","title":"Saved","body":"Item saved.","type":"success","dismissible":true,"dismissMS":5000}]'
  hidden
  aria-hidden="true"
></span>
```

After each DOM update, a client directive reads `data-pending`, creates the toast DOM, and handles auto-dismiss and click-outside — **no server round-trip needed**.

> **HTML escaping safety**: `html/template` automatically escapes the JSON inside the `data-pending` attribute. Combined with the single-quote wrapping of the attribute value, entity-escaped characters in the JSON (e.g., `&amp;`, `&lt;`) are decoded correctly by the browser's HTML parser before `JSON.parse` sees the string. No manual escaping is needed.

---

## Server Side: The Component

The `github.com/livetemplate/lvt/components/toast` package provides a `Container` that queues messages and serializes them on demand.

### State

Add a `*toast.Container` to your state struct. Do **not** add `lvt:"persist"` — the container is non-serializable and must be re-initialized from `initComponents`:

```go
type AppState struct {
    // persistent fields ...

    // Component state (non-persistent, re-initialized each connection)
    Toasts *toast.Container
}
```

> **Note on `AssertPureState`**: If your tests use `lvt/testing.AssertPureState[T](t)` to verify state contains no dependency types, `*toast.Container` will need to be excluded. Component containers are not external dependencies — they hold transient UI data, not connections or handles. Use `AssertPureState` with the `IgnoreFields` option, or structure your state so component fields live in a separate struct that is not checked.

### Initialization

Initialize the container wherever non-persistent fields may be nil — `Mount` (first connection), `OnConnect` (reconnection), and `Sync` (cross-connection state sync):

```go
func initComponents(state AppState) AppState {
    if state.Toasts == nil {
        state.Toasts = toast.New("notifications",
            toast.WithPosition(toast.TopRight),
            toast.WithMaxVisible(3),
        )
        state.Toasts.SetStyled(false)
    }
    return state
}

func (c *Controller) Mount(state AppState, ctx *livetemplate.Context) (AppState, error) {
    state = initComponents(state)
    return state, nil
}

func (c *Controller) OnConnect(state AppState, ctx *livetemplate.Context) (AppState, error) {
    state = initComponents(state)
    return state, nil
}

func (c *Controller) Refresh(state AppState, ctx *livetemplate.Context) (AppState, error) {
    state = initComponents(state)
    return state, nil
}
```

Any action that can run after state has been deserialized or independently dispatched must call `initComponents` because non-persistent fields (like `*toast.Container`) may be nil on that code path.

### Adding Messages

Call the convenience helpers from any action handler:

```go
func (c *Controller) Save(state AppState, ctx *livetemplate.Context) (AppState, error) {
    // ... business logic ...
    state.Toasts.AddSuccess("Saved", "Your changes have been saved.")
    return state, nil
}
```

Available helpers: `AddInfo`, `AddSuccess`, `AddWarning`, `AddError`.

### Template

Use the provided component template to render the trigger span:

```html
{{ template "lvt:toast:container:v1" .Toasts }}
```

This renders a hidden `<span data-toast-trigger="..." data-pending='...'>` when messages are queued. The pending JSON is drained during rendering. Because LiveTemplate evaluates dynamic template expressions twice per action (once for HTML output, once for the diff tree), `TakePendingJSON()` must be explicitly idempotent — the first call drains and caches; the second returns the cached value.

---

## Client Side: The Directive

The `handleToastDirectives` function in `client/dom/directives.ts` is called by the framework after every DOM update. It reads `data-pending`, creates toast DOM elements, and schedules auto-dismiss.

A per-element property (`__lvtPendingProcessed`) prevents the same batch from being shown twice if the directive fires multiple times before the DOM is patched again:

```typescript
// Already handled by handleToastDirectives in directives.ts
// No custom JS needed in your app.
```

**Click-outside dismissal** is set up once at connect time via `setupToastClickOutside()`.

Both functions are wired automatically — no action needed in application code.

---

## CSS for Client-Managed DOM

The client directive creates DOM elements (the toast stack, toast items) that are **not in the server-rendered HTML**. This matters because LiveTemplate uses a morphdom-style diff that removes DOM nodes not present in the server tree on every update.

Two consequences:

1. **The toast stack (`[data-lvt-toast-stack]`) is removed on each server update.** The directive re-creates it every time there are pending messages — no problem.

2. **In LiveTemplate's DOM update strategy, CSS dynamically injected into `<head>` via JS is also removed on each server update**, because the injected `<style>` element is not in the server-rendered `<head>` and the diffing algorithm removes it.

**The solution**: CSS for client-managed elements belongs in the **component template**, not in the consuming app. The `container.tmpl` template already renders a `<style>` block alongside the trigger span:

```html
{{define "lvt:toast:container:v1"}}
{{- $c := . -}}
{{- $pending := $c.TakePendingJSON -}}
<style>
  [data-lvt-toast-stack] { position: fixed; top: 1rem; right: 1rem; ... }
  [data-lvt-toast-item] { ... }
  [data-lvt-toast-item] > button { width: auto; background: transparent; ... }
</style>
<span
  data-toast-trigger="{{$c.ID}}"
  {{- if $pending}} data-pending='{{$pending}}'{{end}}
  hidden aria-hidden="true"
></span>
{{end}}
```

Because `container.tmpl` is included in every server render (it's called from the page template), the diffing algorithm sees the `<style>` on every response and keeps it. The consuming app template needs no CSS for the component.

> **Source**: [`github.com/livetemplate/lvt/components/toast`](https://github.com/livetemplate/lvt/tree/main/components/toast) — the full `Container` API, message helpers, and template.

---

## Adding a New Ephemeral Component

Follow the same pattern for any short-lived UI element (alert banners, confirmation flashes, etc.):

### 1. Server: queue data in state, drain on render

Add the component to state as a non-persistent field. Provide `TakePendingJSON()`-style drain method that is idempotent across LiveTemplate's double-evaluation:

```go
// In your component:
func (c *MyComponent) TakePendingJSON() string {
    if c.hasNewData {
        b, err := json.Marshal(c.data)
        if err != nil {
            // Log the error; return empty so the client directive is a no-op.
            log.Printf("mycomponent: failed to marshal pending data: %v", err)
            c.data = nil
            c.hasNewData = false
            return ""
        }
        c.renderedJSON = string(b)
        c.data = nil
        c.hasNewData = false
        return c.renderedJSON
    }
    result := c.renderedJSON
    c.renderedJSON = ""
    return result
}
```

> **Note**: Always handle the `json.Marshal` error. Silently discarding it (e.g., `b, _ := json.Marshal(...)`) can hide bugs — for example, a field with an unsupported type will produce empty output with no indication of failure.

The three-call contract:
1. **First call** (HTML render pass): marshals data, caches the JSON, clears the queue, returns the JSON.
2. **Second call** (diff-tree pass): returns the cached JSON and clears the cache. Both passes see the same value, so the diff is correct.
3. **Any further call** (next action, no new data): returns `""` — the data has been consumed.

### 2. Template: emit CSS + trigger span

Include a `<style>` block for the client-managed DOM **in the component template** — not in the consuming app. Since the template is called on every server render, morphdom keeps the `<style>` element and the CSS is always in the page.

```html
{{define "myapp:alert:v1"}}
{{- $c := . -}}
{{- $pending := $c.TakePendingJSON -}}
<style>
  [data-lvt-alert-stack] { position: fixed; bottom: 1rem; left: 1rem; ... }
  [data-lvt-alert-item]  { ... }
</style>
<span
  data-alert-trigger="{{$c.ID}}"
  {{- if $pending}} data-pending='{{$pending}}'{{end}}
  hidden aria-hidden="true"
></span>
{{end}}
```

### 3. Client: add a directive in `dom/directives.ts`

```typescript
export function handleAlertDirectives(rootElement: Element): void {
  rootElement.querySelectorAll<HTMLElement>("[data-alert-trigger]").forEach((trigger) => {
    const pending = trigger.getAttribute("data-pending");
    if (!pending) return;
    if ((trigger as any).__lvtAlertProcessed === pending) return;
    (trigger as any).__lvtAlertProcessed = pending;

    let messages: AlertMessage[];
    try { messages = JSON.parse(pending); } catch { return; }
    messages.forEach((msg) => {
      // Create and insert alert DOM
    });
  });
}
```

### 4. Wire the directive in `livetemplate-client.ts`

Import and call from `updateDOM()`:

```typescript
import { handleAlertDirectives } from "./dom/directives";

// In updateDOM():
handleAlertDirectives(element);
```

---

## What NOT to Do

| Anti-pattern | Why it fails |
| --- | --- |
| Render full toast HTML in the template | Unnecessary diff traffic; server must be involved in dismissal |
| Call `TakePendingJSON()` only once | LiveTemplate double-evaluates; the diff tree sees empty string |
| Store toast messages with `lvt:"persist"` | Toasts re-appear after page reload; stale state in session store |
| Write custom JS in the app template | Breaks the framework's progressive-complexity contract |

---

See also: [Progressive Complexity Guide](progressive-complexity.md) for the broader Tier 1/Tier 2 model.
