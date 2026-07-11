# Boilerplate Reduction — Findings & Proposal

Status: **proposal**. Tracks [issue #483](https://github.com/livetemplate/livetemplate/issues/483).

This document catalogs the boilerplate that developers repeatedly write *on top of*
livetemplate, ranks it by cost and universality, and proposes framework changes — both
**additions** and **removals** — to eliminate it. Two low-risk wins (A1, A2) ship alongside
this document; the higher-impact items (Tier B) carry design sketches and await a per-item
greenlight.

## Method

The signal for "a missing framework abstraction" is **the same code shape written
independently across unrelated apps**. Repetition inside one app is a refactor; the same 15
lines re-derived in four apps that never shared code is a framework gap.

Scanned:

- **Canonical `examples/`** (13 demos: counter, chat, todos, login, flash-messages,
  live-preview, shared-notepad, dialog-patterns, avatar-upload, upload-autoupload,
  progressive-enhancement, ws-disabled, landing-demo) — the framework authors' own idea of
  idiomatic minimal usage.
- **Docs-site recipes** (`docs/examples/`) — ~10 small embedded-template handlers.
- **Four independent real apps** consuming livetemplate as a dependency: **devbox-dash**,
  **tinkerdown**, **checklistkit**, **prereview**.

Each finding below cites ≥2 independent apps exhibiting the same shape (paths outside this
repo are given relative to their own repo root).

## Findings

Ranked by cost × universality, in three tiers.

### Tier A — safe wins (shipping with this document)

#### A1. `embed.FS` templates require a temp-file spill — add `WithParseFS`

`WithParseFiles` accepts only OS filepaths (`template.go`, `WithParseFiles` → `ParseFiles` →
`os.ReadFile`). An app that embeds its templates with `//go:embed` (the idiomatic way to ship
a self-contained Go binary) therefore cannot hand them to livetemplate directly — it must
first write the embedded bytes to `os.MkdirTemp` at runtime and pass the temp paths.

Evidence:

- **~10 docs recipes** each carry an identical ~22-line `extractTemplate()` that does
  `embed.FS → os.MkdirTemp → os.WriteFile` (`docs/examples/counter-basic/handler.go:29-50`,
  `docs/examples/greet/handler.go:26-45`, …).
- **prereview** `store.go:80-86` (`stageTemplates`) does the same and its header states the
  reason plainly: *"livetemplate.New requires template files on disk; embedding + staging
  keeps the binary self-contained."* `server.go:84-86` calls it a **"workaround"** and
  cross-references *"Same workaround used by tinkerdown — see
  tinkerdown/internal/server/websocket.go:465."*
- **tinkerdown** — the named cross-reference above.

Three independent apps hand-roll the same spill; two call it a workaround by name. The
machinery to parse from an `fs.FS` **already exists** internally: component templates are
parsed straight from `embed.FS` via `TemplateSet.FS` (`template.go:208`) in
`parseComponentTemplates`. `WithParseFS` just exposes that capability for the main template.

**Design (shipping):**

```go
// Option, in template.go alongside WithParseFiles:
func WithParseFS(fsys fs.FS, patterns ...string) Option

// Method, alongside ParseFiles:
func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error)
```

`ParseFS` mirrors `ParseFiles` but reads via `fs.Glob`/`fs.ReadFile`. Usage collapses the
recipe to:

```go
//go:embed templates
var tmplFS embed.FS

tmpl := livetemplate.Must(livetemplate.New("app", livetemplate.WithParseFS(tmplFS, "templates/*.tmpl")))
```

Precedence (mirrors the existing `WithParseFiles`-overrides-auto-discovery rule):
`WithParseFS` > `WithParseFiles` > auto-discovery. The first matched file is the main
template; the rest parse into the same set for composition.

#### A2. `context.Background()` in actions discards request cancellation — document `ctx`

`*livetemplate.Context` has **embedded `context.Context`** since the Controller+State refactor
(`context.go:92`, PR #69), fed the request/connection context (`r.Context()` on the HTTP and
WS action paths, `mount.go:907,1555`). So `ctx` *already is* a usable `context.Context` — you
can pass it straight to a DB call. But developers don't know that, and reach for
`context.Background()` instead, silently throwing away cancellation, deadlines, and tracing.

Evidence — `context.Background()` in action bodies is **universal**:

- todos 14, tinkerdown 20, checklistkit 19 occurrences.
- checklistkit even carries a **stale comment** — `ui/controller.go:78-80`: *"ctx is the
  livetemplate request context, not a context.Context — use a fresh DB context"* — which
  predates the embed and is now wrong.
- Every sampled site is a within-request DB call
  (`c.Queries.Get…(context.Background(), …)`); **zero** are detached `go func`s. So passing
  `ctx` is strictly safer, and restores request-scoped cancellation.
- (prereview is the clean counter-example: it passes `*livetemplate.Context` through and has
  **no** `context.Background()` in action bodies.)

**Design (shipping):** documentation, not new surface. Add a "Request context" section to
`docs/references/controller-pattern.md` teaching that `ctx` *is* a `context.Context` and
should be passed to DB/HTTP calls; update the canonical examples off `context.Background()`.
An explicit `func (c *Context) Context() context.Context` accessor is **optional** — the embed
already works, so it adds only discoverability; ship it only if it reads better than the bare
embed.

### Tier B — high-impact, design-heavy (design sketches; await greenlight)

#### B1. Live-session fanout is reinvented four ways

"A background event must refresh every live session currently viewing this data" has a
turnkey framework primitive — `ctx.Subscribe(topic)` in `Mount` + out-of-band
`handler.Publish(topic, action, data)` — but it was **undocumented as a pattern**, so every
app that needs server-pushed refresh reached for `Session.TriggerAction` (push to *one*
session group) and hand-rolled a registry of `Session` handles to reach many. devbox-dash
built that registry **twice**:

| App | Where | Shape |
| --- | --- | --- |
| devbox-dash | `session_hub.go:20-60` | `sessionHub`: `register()`/`fanout()` → `TriggerAction("refresh")` + dead-session prune |
| devbox-dash | `main.go:72-90,109-122,224-240` | the **same logic inlined** in `DashController` (second copy) |
| checklistkit | `notifier.go:29-47` | `liveNotifier` over `[]LiveHandler`, fanning out via `h.Publish(topic,"Refresh",nil)` |
| prereview | `internal/review/controller.go:96-109` | single-handle: stashes one `Session`, a watcher goroutine broadcasts `TriggerAction("LLMStatusChanged",nil)` |

Every one carries a paragraph-length comment justifying why it routes around the topic model.
prereview's admits it *"bends the never-store-per-user-data rule"* and **knowingly drops
multi-browser fanout** (`controller.go:103-108`) — precisely the sharp edge a framework
primitive removes.

**Resolution: no new API — the turnkey primitive already exists; the gap was discoverability.**
A design sketch proposed a `LiveGroup`/`handler.Group(name)` abstraction, and its open
question was whether that is anything more than a thin layer over the existing topic model. An
integration proof (`fanout_pattern_test.go`) answered it: the composition **already delivers
the whole capability**, so a wrapper would be pure sugar.

```go
// Join in Mount (reconnect-durable). Then fan out from ANY goroutine, no Context,
// no stashed Session handles, no dead-handle pruning:
ctx.Subscribe(ctx.SelfTopic())                        // per-user (all my tabs), ACL-exempt
handler.Publish(livetemplate.UserTopic("alice"), "Refresh", nil)

ctx.Subscribe("dashboard")                            // shared group (needs WithTopicACL)
handler.Publish("dashboard", "Refresh", nil)
```

Each subscriber re-runs `Refresh` against its own state and re-renders — exactly what
`sessionHub`, `liveNotifier`, and prereview's single-handle hack hand-rolled. The proof also
pinned two facts a `LiveGroup` wrapper would have obscured: (1) developer topics are
**deny-all by default** (a shared group is a real ACL decision, not a name you pick), and (2)
a proposed `Len()` would count only one instance's local subscribers — silently wrong under
multi-instance Redis. checklistkit already used `h.Publish(...)`; the other three simply never
found it.

**Shipped instead (this issue):** task-oriented docs teaching the pattern
([`server-actions.md` § "Fanning out to many sessions"](../references/server-actions.md)),
which also retires the `sync.Map`-of-handles registry the same doc previously recommended; a
regression test locking the two shapes (`fanout_pattern_test.go`); and a runnable example.
The registry-free pattern fixes prereview's dropped multi-browser fanout for free.

#### B2. The client bundle is served from a *test* package in production

livetemplate ships no production static-asset handler for its own client JS/CSS. So apps
import `e2etest "github.com/livetemplate/lvt/testing"` into their **production** `main.go` to
serve the bundle:

- **11/11 canonical examples** + **devbox-dash** (`main.go:485-490`) + **checklistkit**
  (`main.go:261-264`) wire `/livetemplate-client.js` → `e2etest.ServeClientLibrary` and
  `/livetemplate.css` → `e2etest.ServeCSS`.
- Those helpers live in `lvt/testing/chrome.go:559,693`, and their own doc comment says
  *"development/testing purposes only. In production, serve from CDN directly."* The bytes
  come from a runtime **CDN fetch** (`unpkg.com`, `chrome.go:24`) — a network dependency baked
  into request serving.

**prereview resolves the design fork for us.** It refuses the test helper and instead
**vendors + embeds** the bundle: `internal/assets/assets.go:24-28`
(`//go:embed client/livetemplate-client.browser.js` + `livetemplate.css`, synced by
`make sync-client`), served as plain bytes at `server.go:247,251`. It does this because it
must run offline over Tailscale — but that is exactly the production-grade shape every app
wants, and no app should have to hand-vendor it.

**Design sketch.**

```go
// ClientAssetsHandler serves the LiveTemplate client bundle (JS + CSS) from an
// embedded copy — no runtime network fetch. Mount it once:
//   mux.Handle("/livetemplate-client.js", livetemplate.ClientAssetsHandler())
// or a small mux the app mounts under a prefix.
func ClientAssetsHandler() http.Handler
```

The bundle is embedded into the `livetemplate` module via a `go generate` / `make` step that
syncs the built artifact from the separate `client` repo (the module boundary that made it a
test helper in the first place) — the same `make sync-client` mechanism prereview already
proved out.

**Open question (residual):** the vendoring/build mechanism and how the embedded bundle
version is pinned/updated relative to `client` releases. The *approach* (embed, no CDN fetch)
is settled by prereview's precedent.

#### B3. Boot + route ceremony copy-pasted per app and per route

The stand-up sequence `LoadEnvConfig → Validate → ToOptions → Must(New) → Handle(AsState) →
serve client.js/css → PORT → ListenAndServe` recurs in **10+ example mains** as ~15-40 lines
of pure wiring before any business logic. `examples/counter/main.go` is **145 lines** for what
its own README documents as a 10-line `main()`, inflated by a bespoke slog block and a
graceful-shutdown block (with a reflective `liveHandler.(interface{ Shutdown }...)` type
assertion) that no sibling example carries.

Real apps repeat a **per-route handler-builder closure**: checklistkit has **5** builders
(`newLiveHandler`, `newEditorHandler`, `newFillHandler`, `newReviewHandler`,
`newPreviewHandler`) each repeating `opts := []Option{WithParseFiles(…), WithAuthenticator(…),
WithDevMode(…)}` + `if devMode { opts = append(opts, WithPermissiveOriginCheck()) }` +
`Must(New(…))` + `Handle(ctrl, AsState(&X{}))`; devbox-dash's `newPage` closure is the same
shape reused for 6 routes. prereview's `run` and `runExternal` (`server.go`) repeat the
New→controller→mux→listen→shutdown ceremony almost verbatim.

**Design sketch.** Two composable helpers:

```go
// Page builds a LiveHandler in one call (New + Handle), collapsing the three-call
// Must(New(name, opts...)).Handle(controller, AsState(state)) dance.
func Page(name string, controller interface{}, state State, opts ...Option) LiveHandler

// ListenAndServe wires a single-page app end to end: Page + a mux that also serves
// the embedded client assets (B2) + PORT resolution + graceful shutdown on SIGINT/SIGTERM.
func ListenAndServe(name string, controller interface{}, state State, opts ...Option) error
```

Plus a first-class dev-mode toggle folding the recurring `WithPermissiveOriginCheck()` append
into `WithDevMode(true)` semantics (so apps stop hand-appending it per route). The counter
example would return to its documented ~10 lines; checklistkit's 5 builders collapse to 5
`Page(...)` calls.

**Open question:** how opinionated `ListenAndServe` should be (does it own slog setup? read
`PORT`? bundle health/metrics routes?) — keep it thin and composable so multi-route apps
(checklistkit) still drop to `Page` + their own mux.

### Tier C — noted, lower priority / needs a decision

- **C1. `Mount`/`OnConnect`/`Refresh` stub duplication** across ~15 controllers
  (checklistkit `ui/*.go`, devbox-dash `*_controller.go`) — each hand-writes stubs bridging
  the two lifecycle hooks to one load function. **Partly pattern-guidance**: `Mount` already
  runs on WS connect, and `IsInitialMount`/`IsReconnect`/`IsNewConnect` exist to discriminate,
  so much of this is avoidable today. A unified `Load(state, ctx)` hook would formalize it.
  → Propose the hook *and* document the existing pattern.
- **C2. Guard→mutate→error-into-state→reload** action shape — checklistkit prototyped its own
  `mutate()` helper (`ui/editor.go:185-201`); devbox-dash open-codes it
  (`approvals_controller.go`). Hard to generalize without imposing an opinion → propose only.
- **C3. Self-sync idiom** `ctx.Subscribe(ctx.SelfTopic())` + `ctx.Publish(ctx.SelfTopic(),
  "Refresh", nil)` (todos, tinkerdown examples). Common but not universal → propose an opt-in
  `WithSelfSync()` / auto-broadcast flag.
- **C4. Removal candidate — duplicate session-store option.** Two ways to set the store:
  `WithSessionStore` (a `New` Option) and `WithStore` (a `HandleOption`) — both defined in
  `template.go`. Neither is used by any scanned example or app. They operate at different
  levels (construction vs per-handler), so this is a genuine API decision, not a mechanical
  dedup → **pick one to keep** (see Removal pass).
- **C5. e2e harness capture gap** — checklistkit rebuilt a 276-line harness
  (`e2e_harness_test.go`) over the low-level `e2etest` primitives to get WS-frame + browser
  console + server-log capture that the shared `e2etest.Setup` lacks. → Enrich the shared
  harness with capture hooks.

**Surfaced by the prereview scan (new):**

- **C6. Disk-durable store distinct from `lvt:"persist"`.** The default in-memory
  cookie-keyed SessionStore resets on every process relaunch, so a constantly-relaunched CLI
  loses all persisted UI state: prereview `internal/review/uiprefs.go:11-27` writes prefs to
  disk and reloads them every Mount/OnConnect (`controller.go:258`), and `controller.go:240-245`
  does the same reload-from-disk-every-Mount for comments ("the CSV is the source of truth").
  → Propose a disk-backed durable store, separate from session-continuity persistence.
- **C7. Template methods that accept arguments.** livetemplate auto-invokes only zero-arg
  state methods (`internal/parse/eval.go`, `callMethod`), so prereview precomputes predicate
  results into `map[string]…` that the template looks up by key
  (`state.go:806-808,868-873`: `LineDisplay`, `BlockDiffStatus`, `ScrollHeadingBlockKey`).
  → Propose arg-accepting template methods.
- **C8. Recursive `{{template}}` support (cross-app: prereview + tinkerdown).** livetemplate
  flattens `{{template}}` calls at parse time, which overflows on recursion, so a recursive
  file tree drops to a standalone `html/template` injected as `template.HTML`
  (prereview `gitdiff/filetree.go:18-25`, noting *"the same native-`<details>` approach
  tinkerdown uses"*). → Propose recursive-partial support.
- **C9. Static-asset passthrough alongside the `/` LiveHandler.** The SPA owns `/` and
  `ServeMux` routes every unmatched GET to it, so relative asset paths inside rendered content
  (`<img src="mockups/foo.png">`) receive the SPA HTML shell instead of bytes. prereview wraps
  the LiveHandler with an allowlisted-extension, traversal-guarded static file server
  (`server.go:447-524`, `staticFallback`). → Propose a framework static-passthrough wrapper.

## Removal pass

The issue explicitly invites *removing* features, and v0.2.0 was itself an "API Reduction
Release", so subtraction is in-culture. Candidates:

- **C4 — collapse the two session-store options into one.** `WithSessionStore` (New) and
  `WithStore` (Handle) both set the same field at different phases. No scanned code uses
  either (all rely on the in-memory default or `EnvConfig`). Recommend keeping **one**
  (whichever level the store genuinely needs to bind at) and removing the other. Alpha + no
  external users → collapse directly, no deprecation shim.
- **General audit:** the `Context` `With*()` builder family is large and mostly
  internal/test-facing (`WithHTTP`, `WithAction`, `WithConnectKind`, `WithData`,
  `WithFormSchema`, `WithFlashSetter`, `WithUploads`, `WithTopicSubscriber`). Worth a
  follow-up pass to confirm which need to be *exported* vs. package-internal — but that is a
  visibility review, not a behavior change, and is out of scope here.

No other clear-cut removals surfaced: the option set is large but each `With*` option maps to a
real, used capability.

## Recommended sequencing

1. **Ship A1 + A2** (this change) — highest ratio of pain removed to risk taken.
2. **B2 (client-asset handler)** next — the design is settled by prereview's precedent, only
   the vendoring mechanism remains; it removes a test-package-in-production smell from *every*
   app and example.
3. **B3 (Page / ListenAndServe)** — collapses the most raw lines; low conceptual risk.
4. **B1 (fanout)** — **done** (docs + example + regression test, no new API): the proof
   showed the primitive already exists, so this became a discoverability fix. See the B1
   resolution above.
5. **Tier C** individually as they come up; **C4** rides along as a cheap removal.

## Shipped with this document (core `livetemplate` repo)

- **A1** — `WithParseFS(fs.FS, patterns...)` Option + `(*Template).ParseFS`, sharing a
  `parseSources` core with `ParseFiles` (see `template.go`; tests in `parsefs_test.go`,
  including an `embed.FS` golden-parity check against the `ParseFiles` output).
- **A2** — request-context guidance in `docs/references/controller-pattern.md` (the CRUD
  reference example now threads `ctx` into its DB calls). No new API surface: `*Context`
  already embeds `context.Context`, so an explicit `ctx.Context()` accessor was deliberately
  *not* added (redundant).

Sibling-repo companions (land as coordinated follow-up PRs, per the lockstep convention):

- Convert 1-2 `docs/examples` recipes to `embed.FS` + `WithParseFS`, deleting the
  `extractTemplate()` temp-file dance. **Release-gated:** the `docs` repo is outside `go.work`
  and pins a *published* `livetemplate`, so this can only land once a release containing
  `WithParseFS` ships.
- Update the canonical `examples/` apps (todos, chat, …) off `context.Background()` onto
  `ctx`. Doable against the workspace today (the context embed predates this work); needs no
  release.

Everything in Tier B and C is **proposal-only** pending a per-item greenlight.
