# Plan: Reducing boilerplate for server-owned state-attribute updates (issue #481)

Status: **investigation complete — design proposal, awaiting greenlight + a demonstrated consumer.**
Tracks [issue #481](https://github.com/livetemplate/livetemplate/issues/481) ("Investigate how
boilerplate can be reduced for updating state attributes"). Sibling of the umbrella
[boilerplate-reduction.md](../docs/design/boilerplate-reduction.md) effort (issue #483); this
item was not in that catalog because its evidence signal is materially weaker — see § Evidence.

> **This is a plan, not an implementation.** Nothing here changes code. The deliverable is: a
> shippable docs decision-guide (ready now), a fully-specified `Async` primitive with its
> concurrency contract, and an execution outline gated on a real consumer appearing.

---

## Problem statement

A developer wants to show a loading indicator while an async operation runs (e.g. a slow API
call triggered by a button click). In LiveTemplate today, there are two ways to do this, and the
issue asks whether the verbose one can be simplified — preferably to "only template variables"
(no `lvt-*` attributes).

**The verbose path — server-owned loading state (~15 lines, 2 methods):**

```go
// Method 1: initiate — set Loading=true, spawn goroutine, trigger second action
func (a *App) Greet(s State, ctx *lvt.Context) (State, error) {
    if s.Loading { return s, nil }          // re-entrancy guard
    session := ctx.Session()                // session handle
    if session == nil { return s, nil }     // nil check
    name := strings.TrimSpace(ctx.GetString("name"))
    s.Loading = true
    go func() {                             // goroutine with lifecycle concern
        time.Sleep(700 * time.Millisecond)  // simulate slow work
        _ = session.TriggerAction("finishGreet", map[string]any{"name": name})
    }()
    return s, nil
}
// Method 2: complete — clear Loading, apply result
func (a *App) FinishGreet(s State, ctx *lvt.Context) (State, error) {
    s.Name = ctx.GetString("name")
    s.Loading = false
    return s, nil
}
```

```html
<!-- Template uses standard Go conditionals — no lvt-* attributes -->
<button name="greet" {{if .Loading}}disabled aria-busy="true"{{end}}>
    {{if .Loading}}Loading...{{else}}Greet{{end}}
</button>
```

This pattern has four easy-to-botch parts: the re-entrancy guard, the session-nil check, the
goroutine lifetime management, and a whole second action method whose only job is to clear a
flag and apply a result.

**The concise path — client-owned pending (2 attrs, 0 Go lines):**

```go
// Just the business logic — one method, no loading scaffolding
func (a *App) Greet(s State, ctx *lvt.Context) (State, error) {
    time.Sleep(700 * time.Millisecond) // simulate slow work (blocks event loop)
    s.Name = strings.TrimSpace(ctx.GetString("name"))
    return s, nil
}
```

```html
<!-- Tier 2 lvt-el:* attributes handle the visual state client-side -->
<button name="greet"
    lvt-el:toggleAttr:on:pending="disabled"
    lvt-el:addClass:on:pending="opacity-50"
    lvt-el:removeClass:on:done="opacity-50">
    Greet
</button>
```

Zero Go boilerplate — but requires `lvt-*` attributes (Tier 2), the pending state is
client-only (doesn't fan out to peers, doesn't survive reconnect), and the action blocks the
event loop for its duration (no other clicks or peer pushes until it returns).

**The gap:** developers who want custom loading UI (spinners, text changes, progress) using
*only template variables* (staying in Tier 1 / no `lvt-*` attributes) are forced into the
verbose 15-line two-method pattern. The issue asks: can this be reduced?

---

## TL;DR — recommendation

1. **For pure button chrome (disable + opacity), Tier 1 already covers it for free.** The
   framework automatically sets `aria-busy="true"` on forms and disables `<fieldset>` elements
   during submission — zero attributes, zero Go code. CSS-only loading states:
   `form[aria-busy="true"] fieldset { opacity: 0.5; pointer-events: none; }`.
2. **For custom loading UX beyond grey-out** (spinners, text changes, progress indicators),
   there are two paths depending on whether the developer accepts `lvt-*` attributes:
   - *With attributes (Tier 2):* `lvt-el:*:on:pending/done` — 2 attrs, 0 Go lines, instant
     first paint (fires before server sees the message). Already minimal.
   - *Without attributes (Tier 1 only):* server-owned `{{if .Loading}}` — currently ~15 lines
     across 2 methods. **This is what needs reduction.**
3. **Ship now (no new API):** a "loading & pending states" decision-guide in the docs that
   makes the three tiers of loading (auto, client-owned, server-owned) and their trade-offs
   explicit. Documents the manual two-action pattern as the current server-owned recipe.
4. **Spec now, build later (gated):** two primitives that reduce template-variable-only
   loading boilerplate:
   - **`livetemplate.Async`** — collapses the 15-line / two-method server-owned pattern to
     ~7 lines / 1 method. For when loading is *real application state* (fans out, persists,
     drives server logic).
   - **Automatic `{{.lvt.Pending}}`** — a framework-provided template variable that is `true`
     while a form submission is in flight, enabling `{{if .lvt.Pending}}Loading...{{end}}` with
     zero Go code and zero attributes. For when the developer wants custom loading UX but has no
     need for server-owned state — the Tier 1 equivalent of `lvt-el:*:on:pending`.
   Both gated on implementation feasibility and ≥1 demonstrated consumer.

---

## LLM session guide (for whoever executes this later)

- **Docs phase (P1) needs no greenlight** — it is the shippable half. Execute it first and
  independently; it closes the "reduce boilerplate" ask for the common case.
- **The `Async` primitive (P2+) is greenlight-gated.** Do not implement it until (a) a maintainer
  approves the spec below and (b) a real consumer exists. If neither has happened, P1 + this spec
  *is* the complete deliverable for #481.
- **The concurrency contract in § Recommended spec is the acceptance bar**, not the ergonomics.
  If P2's implementation cannot honor "apply to current state, never a snapshot" cleanly, stop
  and report that the primitive isn't ready — that is a legitimate outcome.
- **Cite function-name anchors, not line numbers** (per CLAUDE.md); line numbers below are
  as-of-2026-07 orientation aids and will drift.

---

## Root cause — why server-owned loading is verbose

LiveTemplate uses the same action-dispatch model across all transports (HTTP POST, fetch, and
WebSocket). The core constraint is **one action → one response**: an action method receives
state, returns updated state, and the framework renders exactly once. This is true whether the
request arrives as an HTTP POST (response = full page), a fetch request (response = DOM patch),
or a WebSocket message (response = update frame).

On WebSocket connections, this is implemented as a **single-threaded event loop** (`mount.go`,
the `eventLoop:` labelled `for`/`select`) that drains `readChan` and `connection.DispatchChan`
serially. On HTTP, the same constraint holds naturally: one request, one response.

Two consequences fall out of this model regardless of transport:

- **One action → exactly one render.** After `DispatchWithState` (`dispatch.go`) returns, the
  framework renders the template once and sends the result. There is no "render again later" hook
  within a single request/message cycle.
- **A blocking action freezes the response.** `time.Sleep(700ms)` or a slow DB call inside an
  action delays the response (and on WebSocket, stalls all other client messages and peer pushes
  for that connection until it returns).

So to show a spinner *and later* clear it from one logical user action, the slow work **must
return early and re-enter later**. On WebSocket, the re-entry mechanism is a dispatched action
via `Session.TriggerAction` → `connection.DispatchChan`. On HTTP/fetch, the same
`Session.TriggerAction` triggers a re-render that the client picks up on the next poll or
WebSocket upgrade.

```
Greet action ──(return, render #1: Loading=true)──▶ framework renders spinner-on
     │
     └─ go func(){ slow work off-loop; session.TriggerAction("finishGreet") }
                                    │
                                    ▼
        EnqueueDispatch ▶ dispatch channel ▶ handleDispatchedAction
                                                    │
                                    FinishGreet(current state) ─▶ render #2: Loading=false
```

Every line of the manual pattern (§ Problem statement) is scaffolding around that forced
round-trip: re-entrancy guard (`if s.Loading { return }`), `ctx.Session()` handle + nil check,
the goroutine, `Session.TriggerAction`, and a whole second method `FinishGreet`.

**Crucial correctness detail that the eventual primitive must preserve:** `handleDispatchedAction`
runs the second action against the **current** state at re-entry time (`mount.go`, the
`DispatchWithState(h.config.Controller, connSt.state, ctx)` call), *not* a snapshot captured when
`Greet` ran. And `FinishGreet` writes only the two fields it owns (`Name`, `Loading`). That is
what makes the manual pattern safe under concurrent edits during the async window.

---

## The three loading models (this is the shippable decision-guide)

| | **Tier 1 auto** | **Client-owned pending** (Tier 2) | **Server-owned loading** (Tier 1) |
|---|---|---|---|
| Template | `<fieldset>` wrapping + CSS | `lvt-el:addClass:on:pending="is-loading"` etc. | `{{if .Loading}}disabled{{end}}` |
| Go | 0 lines | 1 linear method, no loading code | ~15 lines across **2** methods + goroutine |
| Attributes | **None** | 2 `lvt-el:*` attributes | **None** |
| Custom UX (spinner, text) | No — grey-out only | Yes — any DOM mutation | Yes — full template control |
| First-paint latency | One round-trip | **Instant** — fires on click | **Instant** with optimistic pending; one round-trip without |
| Fans out to peers | No | No | Yes (`TriggerAction`) |
| Survives reconnect | No | No | Yes (it's in state) |
| Correct when… | "Disable the form" suffices | Indicator is **chrome** | Loading is **application state** |

**Guidance to document:** Go + standard HTML (Tier 1) is the preferred developer experience —
it uses familiar tools, keeps logic in one place, and requires no framework-specific template
syntax. Start with Tier 1 auto (wrap inputs in `<fieldset>` + CSS). For custom loading UX
(spinners, text changes), the Tier 1 server-owned path (`{{if .Loading}}`) is the natural
next step — once `Async` and `{{.lvt.Pending}}` land, this becomes low-boilerplate too.

**When `lvt-*` attributes are the better choice:** with client-side optimistic pending (above),
`{{.lvt.Pending}}` can match `lvt-el:*:on:pending` on latency. The remaining reasons to reach
for `lvt-el:*` attributes are: (a) **fine-grained DOM mutations** beyond what template
conditionals express (e.g. `addClass`, `toggleAttr` on specific elements without wrapping them
in `{{if}}`), or (b) **lifecycle states beyond pending** (`success`, `error`, `done` — the
full action lifecycle, not just the binary pending/not-pending). The trade-off remains: the
pending state is client-only, so it doesn't fan out to peers, doesn't survive reconnect, and
can't drive server-side logic.

**The issue's question — "using only template variables":** for developers who want custom
loading UX staying in Tier 1, the current answer is the verbose 15-line manual pattern. The
proposed `{{.lvt.Pending}}` (design option 2) and `Async` (design option 3) would reduce this
to zero Go lines (for chrome-level pending) or ~7 lines (for real app state), respectively.

This table + the manual two-action recipe is the **P1 docs deliverable** and needs no new API.

---

## Design space

### Tier 1 (no `lvt-*` attributes) — the issue's "only template variables" constraint

- **(0) Automatic form loading — existing, zero-boilerplate for grey-out.** The framework
  auto-sets `aria-busy="true"` on forms and disables `<fieldset>` during submission. CSS rules
  like `form[aria-busy="true"] fieldset { opacity: 0.5 }` give visual feedback with zero Go
  code and zero attributes. **Limitation:** covers only the "grey out the whole form" case.
  Custom UX (spinners, text changes, progress indicators) needs something more.

- **(1) Manual two-action pattern — existing, verbose.** Correct but ~15 lines / 2 methods with
  four easy-to-botch parts (guard, session nil-check, goroutine lifetime, second action name).
  Uses `{{if .Loading}}` in the template — pure Tier 1. This is what #481 wants to shrink.

- **(2) Framework-provided `{{.lvt.Pending}}` template variable — RECOMMENDED to evaluate.**
  The Tier 1 equivalent of `lvt-el:*:on:pending`. The framework exposes a boolean (or
  per-action map) in the template data that is `true` while a form submission is processing:

  ```html
  <button name="greet" {{if .lvt.Pending}}disabled{{end}}>
      {{if .lvt.Pending}}Loading...{{else}}Greet{{end}}
  </button>
  ```

  Zero Go code, zero attributes. The developer stays entirely in Tier 1.

  **Instant feedback via client-side optimistic pending (best of both worlds):**

  The one advantage `lvt-el:*:on:pending` has over a server-rendered `{{.lvt.Pending}}` is
  latency — the client fires it instantly on click, before the server even receives the message.
  We can eliminate this gap by making the client aware of pending-state diffs:

  1. On each render where `.lvt.Pending` is `false`, the server also computes the tree with
     `.lvt.Pending = true` and includes the **pending diff** as metadata alongside the normal
     response (a small additional payload — typically just the changed dynamics: button text,
     disabled attr, spinner visibility).
  2. When the client dispatches an action, it **immediately applies the pre-computed pending
     diff** — instant visual feedback, zero round-trip, no `lvt-*` attributes.
  3. When the server responds (with the actual `Async` pending state, or the completed state),
     the client applies that response as usual, which either confirms or supersedes the
     optimistic pending.

  This gives developers Go + standard HTML templates with the same instant-feedback UX that
  `lvt-el:*:on:pending` provides — genuinely best of both worlds. The developer writes
  `{{if .lvt.Pending}}Loading...{{end}}` in their template and gets instant client-side
  feedback without any `lvt-*` attributes or Go boilerplate.

  **Cost:** one additional template render per response cycle (the pending-state variant).
  This is a server-side cost only, and only for templates that use `.lvt.Pending`. The pending
  diff is typically small (a few changed dynamics), so wire overhead is minimal.

  **Mechanics (server side):** after each render, if the template references `.lvt.Pending`,
  the framework re-renders with `.lvt.Pending = true` and diffs the two trees. The resulting
  diff is sent as a `"p"` (pending) key in the response metadata. The client stores it and
  applies it optimistically on the next action dispatch.

  **Mechanics (client side):** the client JS (`livetemplate/client`) stores the latest pending
  diff. On form submit or action dispatch, it applies the diff to the DOM immediately. When
  the server response arrives, normal patch logic takes over (the server's response is
  authoritative).

  **Open design question:** should the optimistic pending apply to all actions dispatched from
  within the pending-diff's subtree, or should it be scoped to the specific form/button that
  triggered it? Scoped is more correct (a page with two forms should only show pending on the
  submitted one) but requires the client to match the dispatch source to the diff's DOM scope.
  Evaluate during P5 implementation.

- **(3) `Async` continuation primitive — RECOMMENDED to spec.** Collapse (1) to one method
  while keeping loading in real server state. This is the primary reduction for developers
  who need server-owned loading (fans out to peers, persists across reconnect, drives server
  logic). Detailed in § Recommended spec. Also enables option (2): when an action calls
  `Async` and returns early, `.lvt.Pending` can reflect the in-flight state.

### Tier 2 (`lvt-*` attributes available)

- **(4) Client-owned `lvt-el:*:on:pending/done` attributes — existing, already minimal.**
  2 template attrs, 0 Go lines. Instant first paint (fires on click, before server sees the
  message). No framework change reduces this further. This is the recommended path when the
  developer is willing to use Tier 2 attributes and the pending state is purely visual chrome.

### Decision matrix — "what's my path?"

| I want… | Accepts `lvt-*` attrs? | Path | Boilerplate |
|---|---|---|---|
| Grey out the form | N/A | **(0)** Auto `aria-busy` + `<fieldset>` | 0 Go, 0 attrs, CSS only |
| Custom loading UX (spinner, text) | Yes | **(4)** `lvt-el:*:on:pending/done` | 0 Go, 2 attrs |
| Custom loading UX (spinner, text) | **No** | **(2)** `{{.lvt.Pending}}` *(new)* | 0 Go, 0 attrs |
| Loading is real app state (fan-out, persist) | Either | **(3)** `Async` *(new)* | ~7 Go lines, 0 attrs |
| Loading is real app state | Either (today) | **(1)** Manual two-action | ~15 Go lines, 0 attrs |

### Rejected alternatives (do not re-propose)

- **Single-closure `ctx.Async(func(s State) (State, error))`.** The nicest-reading shape and the
  wrong one: it must run off-loop on a snapshot of `s`, silently clobbering any state a concurrent
  action mutated during the async window. Rejected on correctness; the `work`/`apply` split
  replaces it. **This is the central finding.**
- **Blocking the event loop + an intermediate `ctx.Flush(s)` render.** Would freeze all other
  client messages and peer pushes for the connection for the whole `work` duration — the exact
  tradeoff `mount.go` calls out. A non-starter under the single-loop model.
- **Auto-detecting "slow" actions and rendering pending before dispatch.** Requires the framework
  to guess which actions are slow and pre-render a pending tree; brittle, and still round-trip
  latency. `Async` makes the async boundary explicit where the developer already knows it.

---

## Recommended spec — `livetemplate.Async`

### Shape (concurrency-correct)

The primitive **must** separate the off-loop work from the on-loop state application, because the
single-closure form that reads best is incorrect (§ The trap). A free generic function is the
natural home (the apply step needs the `State` type, which `*Context` does not carry):

```go
// Async runs work off the connection event loop, then re-enters the loop to
// apply its result to the CURRENT session state and re-render this connection.
//
//   - work runs in a supervised goroutine bound to the connection's context;
//     it must NOT touch session state (it has none) — only its own inputs.
//   - apply runs ON the event loop against the latest state at completion time
//     (exactly like a dispatched action), so it composes safely with any edits
//     that landed during the async window. It should mutate only the fields it owns.
//   - If the connection closes first, work is cancelled and apply never runs.
func Async[S State, R any](
    ctx *Context,
    work  func(context.Context) (R, error),
    apply func(s S, result R, err error) (S, error),
)
```

### Before / after

```go
// BEFORE — 15 body lines across two methods (the manual pattern from § Problem statement)
func (a *App) Greet(s State, ctx *lvt.Context) (State, error) {
    if s.Loading { return s, nil }
    session := ctx.Session()
    if session == nil { return s, nil }
    name := strings.TrimSpace(ctx.GetString("name"))
    s.Loading = true
    go func() {
        time.Sleep(700 * time.Millisecond)
        _ = session.TriggerAction("finishGreet", map[string]any{"name": name})
    }()
    return s, nil
}
func (a *App) FinishGreet(s State, ctx *lvt.Context) (State, error) {
    s.Name = ctx.GetString("name")
    s.Loading = false
    return s, nil
}

// AFTER — one method, ~7 lines; no guard, no session handle, no goroutine, no second action
func (a *App) Greet(s State, ctx *lvt.Context) (State, error) {
    s.Loading = true
    name := strings.TrimSpace(ctx.GetString("name"))
    lvt.Async(ctx,
        func(context.Context) (string, error) { time.Sleep(700 * time.Millisecond); return name, nil },
        func(s State, name string, err error) (State, error) { s.Name = name; s.Loading = false; return s, nil },
    )
    return s, nil // render #1: Loading=true
}
```

Template is unchanged from the server-owned column: `{{if .Loading}}...{{end}}` (and the same
`{{if .Loading}}disabled{{end}}` double-submit guard the developer already writes).

### Concurrency contract — the acceptance bar

1. **Apply-to-current-state, never a snapshot.** `apply` receives the live `connState.state` at
   completion, reusing the existing `handleDispatchedAction` re-entry (which already does exactly
   this). It must not receive state captured when `work` was registered.
2. **`work` has no state access — by construction.** It takes only `context.Context`. This makes
   the snapshot footgun *unrepresentable*, rather than merely discouraged.
3. **Per-connection render scope.** Completion re-renders **only the originating connection**, not
   the whole session group. (Contrast: the manual `TriggerAction` fans out to all tabs. Per-tab is
   the more intuitive default for loading; group fan-out is opt-in via `Session.TriggerAction`
   inside `apply` if the developer wants it — but see Open question O2 re: the dispatched-context
   `Publish` guard.)
4. **Lifetime bound to the connection.** `work` runs under the connection's context; on disconnect
   the goroutine is cancelled and `apply` is skipped (mirrors `TriggerAction`'s
   `ErrSessionDisconnected` and Phoenix's "async ops stop when the LiveView exits").
5. **Re-entrancy is the developer's, at the UI.** The disabled button (`{{if .Loading}}disabled`)
   already prevents double-submit; the primitive need not add an implicit guard. (Optional
   `start_async`-style "same key replaces in-flight task" dedupe is a possible enhancement, not
   required for v1.)

### Mechanics (reuses existing machinery, no new transport)

- On the action's return, the event loop renders state as usual (render #1, spinner on).
- The primitive registers a pending continuation on `ctx` (like `pendingTopicPublishes`), which the
  event loop hands to a supervised goroutine after the render.
- The goroutine runs `work` under `connection.Done()`-tied context; on completion it enqueues a
  synthetic request onto `connection.DispatchChan` carrying the result + the `apply` callback.
- `handleDispatchedAction` (extended, or a sibling) invokes `apply` against current
  `connSt.state`, persists, and sends render #2 (spinner off). All on the event loop — no mutex.

---

## Prior art — Phoenix LiveView (same execution model)

Phoenix LiveView runs the identical one-process-per-connection model and shipped this exact
primitive (~v0.20), which de-risks the design:

- `assign_async(socket, keys, func)` — `func` returns `{:ok, assigns}` / `{:error, reason}`;
  framework tracks an `AsyncResult` with **loading / ok / failed / exit** states the template reads
  (`@x.loading`, `@x.ok?`, `@x.result`). Maps to our optional declarative-status enhancement.
- `start_async(socket, name, func)` + `handle_async/3` — lower-level; the callback applies the
  result to **current** socket assigns before returning. This is our `work`/`apply` split, and it
  independently validates contract point 1.
- Tasks are **linked to the caller and stopped when the LiveView exits** — our contract point 4.
- On reconnect, tasks are **not** auto-restarted (Mount re-runs) — informs Open question O3.

Reference: `Phoenix.LiveView` async operations docs (verify current signatures at build time).

---

## Disposition & sequencing

Consistent with the `boilerplate-reduction.md` culture (resolve to docs when primitives compose;
gate speculative API on demonstrated consumers):

1. **P1 — docs decision-guide. Ship now, no greenlight.** The three-models table + "start with
   Tier 1 auto, reach for client-owned, use server-owned only for real state" guidance + the
   manual server-owned recipe. Closes the common-case ask.
2. **P2..P4 — `Async` primitive. Spec approved here; implementation gated** on maintainer
   greenlight **and** ≥1 real app needing server-owned async state (currently zero). Relaxed from
   the repo's usual ≥2-app bar to ≥1 because the shape is well-established prior art, not a
   guess — but not to zero.
3. **P5 — `{{.lvt.Pending}}` template variable. Design gated** on resolving the open question
   of whether the framework sends an intermediate "pending" render before dispatching actions
   (universally or only for `Async`-using actions). If paired with `Async` (option 3 enables
   option 2), this becomes the zero-boilerplate Tier 1 answer the issue asks for. Evaluate
   feasibility during P2.

---

## Evidence & honest calibration

The `boilerplate-reduction.md` method sets the bar: a real framework gap is *"the same code shape
written independently across ≥2 unrelated apps."* Measured against that bar, this item is **weak**:

- A repo-wide grep for the pattern (`Greet`/`FinishGreet`, `greet-btn`, server-owned `Loading` +
  `TriggerAction`) finds it in **zero** real apps and **zero** examples — only `navigate_test.go`
  and an internal `data_test.go`, both unrelated.
- The `Greet`/`FinishGreet` snippet in the issue is a **hand-authored illustration**, not
  observed repetition. Contrast A1 (10 recipes + 3 apps hand-rolling `extractTemplate`), B1 (four
  apps), B3 (10+ mains) — all had concrete cross-app repetition.

This does not make the boilerplate unreal — it is genuinely ~15 lines with four botchable parts —
but it means the honest disposition is **"design it, prove the concurrency contract, gate the
build on a consumer,"** not "ship the API." Documenting the two models (P1) is the part that
demonstrably reduces friction today.

---

## Implementation phases — progress tracker (conditional on greenlight)

Legend: `[x]` done · `[~]` in progress · `[ ]` todo · `[GATED]` blocked on greenlight + consumer

- [x] **P0 — Investigation** (this document): root cause, two-models guide, `Async` spec +
      concurrency contract, prior art, rejected alternatives, disposition.
- [x] **P1 — Docs decision-guide** *(shippable now, no greenlight)*
  - [x] "Loading & pending states: client-owned vs server-owned" section in
        `docs/guides/progressive-complexity.md` (§7 expanded with three-tier guide + decision table).
  - [x] Cross-link from `docs/references/client-attributes.md` (the `on:pending`/`on:done` block)
        and `docs/references/controller-pattern.md` (TriggerAction section).
  - [x] Document the manual two-action pattern as the current server-owned recipe, with the
        re-entrancy / session-nil / goroutine-lifetime caveats called out.
  - Acceptance: `/simplify` on the diff; a reader can pick the right model from the table alone.
- [ ] **P2 — `Async` primitive core** `[GATED]`
  - [ ] `Async[S State, R any](ctx, work, apply)` free function; register a pending continuation on
        `Context` (parallel to `pendingTopicPublishes`).
  - [ ] Event-loop hook: after render #1, hand the continuation to a connection-context-bound
        goroutine; on completion enqueue a synthetic `DispatchChan` request carrying result+apply.
  - [ ] Extend `handleDispatchedAction` (or a sibling) to run `apply` against current
        `connState.state`, per-connection render scope, persist.
  - Acceptance (the contract): unit tests proving (1) apply sees state mutated by an interleaved
    action, not a snapshot; (2) disconnect during `work` cancels and skips `apply` with no
    goroutine leak (`-race`); (3) render scope is the originating connection only.
- [ ] **P3 — Optional `AsyncResult`-style declarative status** `[GATED]`
  - [ ] Framework-managed loading/ok/failed status readable as a template helper, for the
        "only template variables" ergonomic — only as sugar over P2, only when it's real state.
- [ ] **P4 — Example + docs lockstep** `[GATED]` (per "feature work ships an example + docs")
  - [ ] Runnable `docs/examples` app (a server-owned async greet/job-status) demonstrating `Async`.
  - [ ] Reference docs; browser e2e (chromedp) asserting the two-frame sequence (spinner-on frame,
        then spinner-off frame) with WS-frame capture; reactive-path full-doc e2e per repo rule.
- [ ] **P5 — `{{.lvt.Pending}}` template variable** `[GATED]` (design feasibility TBD in P2)
  - [ ] Resolve open design question: automatic pending render for all actions vs `Async`-only.
  - [ ] Inject `.lvt.Pending` (and optionally `.lvt.PendingAction`) into template context during
        action processing.
  - [ ] Tests: template renders `{{if .lvt.Pending}}` correctly during async window; clears after
        action completes; does not leak across connections.
  - [ ] Runnable `docs/examples` app demonstrating `{{.lvt.Pending}}` for zero-attribute loading UX.
  - [ ] Update docs decision-guide to include this as the zero-boilerplate Tier 1 path.

---

## Open questions

- **O1 — API home & generics. RESOLVED:** Free generic function
  `Async[S State, R any](ctx, work, apply)`. `*Context` can't carry `S`, so a free function is
  the natural home. Confirm at implementation time that the `State` constraint composes with the
  reflection-based dispatch in `dispatch.go`.
- **O2 — Group fan-out from `apply`. RESOLVED:** Use `Session.TriggerAction` inside `apply` for
  fan-out — no `WithBroadcast()` option on `Async`. Keeps `Async` scoped to per-connection
  rendering; group fan-out uses the same `TriggerAction` mechanism developers already know.
- **O3 — Reconnect semantics.** Phoenix does not auto-restart async tasks on reconnect (Mount
  re-runs). Confirm the same for `Async`: an in-flight `work` whose connection drops is abandoned;
  the reconnect's Mount is responsible for re-deriving state. Document explicitly.
- **O4 — Multi-instance.** `work` runs on the instance that received the action; `apply` re-enters
  that instance's connection. If the connection migrated instances mid-flight (Redis-backed), the
  original connection is gone → cancel + skip (contract point 4). Verify no cross-instance apply is
  attempted.
- **O5 — Error surfacing.** `apply` receives `err` from `work`; document the convention (set a
  state error field / `FieldError`) so failed async work renders a visible failed state rather than
  silently clearing the spinner (aligns with Phoenix's `failed`/`exit`).
- **O6 — `{{.lvt.Pending}}` render timing. SUPERSEDED by optimistic pending.** The original
  question (send a server-side pending render before or after dispatch?) is resolved by the
  client-side optimistic pending approach: the server pre-computes the pending diff at render
  time, and the client applies it instantly on action dispatch — no extra server round-trip
  needed. Remaining design question: should the optimistic diff be scoped per-form or per-page?
  See the open question in § Design space option (2).
