# Navigate Action Reference

The `__navigate__` action is a reserved WebSocket-only message that re-runs `Mount` with new query parameters on the **same** WebSocket connection — no reconnect, no full-page reload. It powers in-handler SPA-style navigation (search filters, tab switches, faceted browse) without giving up the LiveTemplate session, the cached statics, or the open WS pipe.

This page is the invariant catalogue (issue #349). It documents what `__navigate__` does, when the client emits it, and the rules a controller must satisfy to behave correctly under it.

---

## What `__navigate__` Does

When the server receives `{action: "__navigate__", data: {<query params>}}` over a session's WebSocket:

1. The event loop bypasses `DispatchWithState` (the normal action router).
2. It treats `msg.Data` as the new query string and re-invokes `Mount` with that data.
3. The render that follows is a tree **update**, not a full render — the client already has statics cached, so only the changed dynamic slots travel over the wire.
4. Flash messages set with `SetFlash` survive the navigate by default; only `ClearFlash(key)` or `ClearAllFlash()` removes them. (See "Flash interaction" below.)

The reserved constant lives at `livetemplate/action.go` — grep `actionNavigate = "__navigate__"`. There is **no controller method named `__navigate__`**, and adding one is not how you customize navigation behavior. Mount is the customization point.

---

## When the Client Emits It

The TypeScript client emits `__navigate__` from `client/dom/link-interceptor.ts`. The cases:

| Trigger | Condition | What gets sent |
|---|---|---|
| `<a href>` click | Same pathname, only query string differs | `{action: "__navigate__", data: <new query params>}` over the open WS |
| `popstate` (back/forward) | Same pathname as before, only query string changed | Same as above |
| Different pathname | New path | Falls back to a fetch-based navigation (or full page load if WS is HTTP-only) |
| External link, `target="_blank"`, `download`, or `lvt-nav:no-intercept` | Any | Not intercepted — browser handles the link normally |

The popstate path matters: when a user hits Back, the browser updates `window.location` first and then fires the event. The link interceptor stores the previous URL on each push so the popstate handler can compare against the right "before" URL.

If the WebSocket is not OPEN, the client falls through to fetch-based navigation. `__navigate__` is strictly the fast path; the slow path stays correct.

---

## What the Controller Must Do

### Mount must be safe to re-run

Mount runs on every HTTP request, every WS connect, AND every `__navigate__`. Crucially, **inside Mount, a `__navigate__` re-run is indistinguishable from a connect-time Mount** — the dispatch loop deliberately rebinds `ctx.Action()` to `""` for navigate so handlers don't have to special-case it. (Grep `mount.go` for `WithAction("") // ctx.Action()=="" matches connect-time Mount`.)

That means the standard `if ctx.Action() == "" { ... }` guard from the controller-pattern docs filters out form POSTs but does **not** filter out navigate re-mounts — it still fires on each `__navigate__`. There are two ways to handle one-time side effects (analytics page-view, audit log, expensive bootstrap):

**Preferred — `ctx.IsInitialMount()`:** Returns true only for the initial HTTP GET, false for WS new-connects, reconnects, *and* `__navigate__` re-mounts (which dispatch through the WS event loop as an action, not as a GET). Side effects fire exactly once per initial page load:

```go
func (c *Controller) Mount(state State, ctx *livetemplate.Context) (State, error) {
    if ctx.IsInitialMount() {
        c.analytics.TrackPageView(ctx.UserID())
    }
    state.Filter = ctx.GetString("filter")
    return state, nil
}
```

**Fallback — `ctx.Action() == ""` + persist flag:** If you still use the older idiom (which is true for GETs, WS connects, *and* navigate re-mounts), gate side effects on per-session state so they don't fire repeatedly:

```go
if ctx.Action() == "" && !state.PageViewTracked {
    c.analytics.TrackPageView(ctx.UserID())
    state.PageViewTracked = true
}
```

### Read query params from `ctx`

Inside Mount, `ctx.GetString("filter")` and friends return whatever was in `msg.Data` for a `__navigate__`, or the URL query for the initial GET. Same call site, same data shape — your Mount code does not need to branch on "am I initial vs. navigate."

### Don't touch the URL yourself

The client owns `pushState`. Mount must not redirect or rewrite paths in response to a `__navigate__` — doing so will desynchronize the browser URL from the server-side state. If you need to deny a navigation, return an error from Mount; the client surfaces it without committing the URL change.

---

## Flash Interaction

PR #344 paired `__navigate__` with a "persist-until-cleared" flash lifecycle. The rules:

- `ctx.SetFlash(key, msg)` — flash persists across renders, including across `__navigate__` re-mounts, until explicitly cleared.
- `ctx.SetFlash(key, msg, livetemplate.FlashExpiry(5*time.Second))` — flash auto-expires after the duration even if not cleared.
- `ctx.ClearFlash(key)` — removes a single keyed flash.
- A bulk `ClearAllFlash()` API is proposed in [issue #345](https://github.com/livetemplate/livetemplate/issues/345) but has not landed — until then, clear keys individually.

The "cleared after one render" semantics from older versions are gone. If you want one-shot flash-after-action, call `ClearFlash` at the top of the handler that consumes it (typically in Mount when the relevant query param disappears).

---

## What Travels Over the Wire

Because the client has the statics cached from the initial render, a `__navigate__` response is a tree **update** — only the changed dynamic slots ship. The shape is identical to any other action's update payload. No special framing, no per-page bundle.

For a counter-template-sized handler (two slots: `Selected`, `MountCount`), the navigate update is on the order of 10-20 bytes. For a search-results handler (item list), it's roughly the bytes of the item list, with the surrounding chrome (header, filter UI, footer) shipped only once at first connect.

---

## Verification

The load-bearing test is `TestNavigateActionReMountsWithNewQueryData` in `navigate_test.go`. It:

1. Connects a WebSocket with `?s=alpha`.
2. Confirms the rendered tree shows `state.Selected == "alpha"` and `MountCount == 1`.
3. Sends `{action: "__navigate__", data: {s: "beta"}}` over the same WS.
4. Confirms the next render flips `Selected` to `"beta"` and bumps `MountCount` to `2` — proving Mount re-ran without any reconnect.

Browser-level chromedp tests live in the lvt repo at `e2e/livetemplate_core_test.go` per the [test strategy](../../CLAUDE.md). Both layers must stay green.

---

## See Also

- [Controller+State Pattern](controller-pattern.md) — Mount-time conventions
- [Client Attributes](client-attributes.md) — `lvt-nav:no-intercept` opt-out
- [Standard HTML Reactivity](../guides/standard-html-reactivity.md) — Why navigation is a Tier 1 concern
- [PR #344](https://github.com/livetemplate/livetemplate/pull/344) — Original implementation
- Follow-up issues: [#345](https://github.com/livetemplate/livetemplate/issues/345) (`ClearAllFlash`), [#346](https://github.com/livetemplate/livetemplate/issues/346) (`BroadcastAction` inside Mount on navigate), [#347](https://github.com/livetemplate/livetemplate/issues/347), [#348](https://github.com/livetemplate/livetemplate/issues/348)
