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

The finding is really **two smells**: (1) a *test* package imported into production, and
(2) a runtime CDN fetch pinned to the unversioned `@latest` tag — and there is **no runtime
version handshake** between server and client (grepped both; none), so `@latest` can serve a
browser a client newer than the server it talks to. The `@livetemplate/client` CHANGELOG
documents exactly this footgun: the `__navigate__` in-band action is *"a no-op on server
versions before livetemplate#344 — deploy the server update before or simultaneously with this
client version."* These are 0.x releases, where minors are allowed to break the wire.

**Resolution: framework-owned version pin, no bundling.** The embed-the-bytes route was
considered and rejected. prereview's `//go:embed` (`internal/assets/assets.go:24-28`, synced by
`make sync-client`) is right *for an app* that must run offline over Tailscale, but wrong *for a
library*: `go:embed` resolves against the module zip the proxy serves at a tag, so a gitignored
bundle breaks every downstream `go get` build, and *committing* a ~140KB minified blob makes
core stop being the zero-asset leaf it deliberately is (its `go.mod` depends on neither `lvt`
nor `client`). Embedding solves a minority (offline) need at a cost every consumer pays.

Instead, core owns the one thing it uniquely knows — **which client version is wire-compatible
with it** — and nothing more (`client_assets.go`):

```go
// Moves in lockstep with each livetemplate release, so apps never hand-maintain a
// client version: `go get -u` bumps the server and its matching client together.
const ClientVersion   = "0.16.5"
const ClientScriptURL = "https://cdn.jsdelivr.net/npm/@livetemplate/client@" + ClientVersion + "/dist/livetemplate-client.browser.js"
const ClientStyleURL  = "https://cdn.jsdelivr.net/npm/@livetemplate/client@" + ClientVersion + "/livetemplate.css"
```

So templates can reference the pinned URL with **zero per-app wiring**, the framework seeds two
template functions into every `Template`'s `FuncMap` at `New` (before parse), returning those
constants:

```html
<link rel="stylesheet" href="{{ lvtClientStyleURL }}">
<script src="{{ lvtClientScriptURL }}" defer></script>
```

This kills both smells: examples use those funcs and drop the `e2etest` import + the two
serving routes entirely — pinned, no test-package-in-prod, no bytes anywhere, no per-app State
field, zero per-app version maintenance. It also *removes* the toil `@latest` appeared to save:
the version moves only when you deliberately upgrade the Go dependency, in lockstep with the
compatible server. Offline / air-gapped / CSP-strict deployments self-host (vendor
`@livetemplate/client@<ClientVersion>`, serve same-origin, write their own tag instead of
calling the funcs) — exactly what prereview keeps doing. Pinned to `0.16.5` because that is what
`@latest` resolves to today, so the switch is behavior-preserving.

The `lvt/testing` helpers (`ServeClientLibrary`/`ServeCSS`) **stay** — real E2E tests keep
using them; we only stop shipping them in production examples.

**Follow-ups (separate, greenlightable on their own):** the framework already owns full-HTML
wrapper injection (`compat.InjectWrapperDiv` in `template.go`), so it *could* later auto-inject
the pinned
`<script>`/`<link>` (opt-out) so apps drop even the tag — a bigger win with real design surface
(CSP, opt-out, dev/prod placement), left out of this minimal change. SRI-hash pinning is another
optional hardening.

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

- **C1. `Mount`/`OnConnect`/`Refresh` stub duplication.** ✅ **RESOLVED (docs, no core
  change).** Original claim was ~15 controllers across checklistkit + devbox-dash hand-writing
  a lifecycle triple. On re-audit the signal had **collapsed to one un-migrated app**:
  checklistkit uses a different notifier (0 triples), tinkerdown/prereview are `Mount`-only or
  use a distinct `OnConnect`, and devbox-dash's own triple is the **pre-B1 pattern** — half of
  it (`OnConnect{ hub.register(ctx.Session()) }` + the hub) is exactly the sync.Map handle
  registry that [B1 / livetemplate#485](https://github.com/livetemplate/livetemplate/pull/485)
  already retired. The residual boilerplate is just `Refresh(){ return c.Mount() }`, and that
  turned out to be an **anti-pattern, not a primitive to bless**: `Mount` does connect-time work
  (`Subscribe`, `IsInitialMount`-guarded setup) that must not repeat on every fan-out tick, so a
  push-refresh action must reload data *only* — as the shipped `live-dashboard` example already
  demonstrates (`Mount` subscribes+snapshots; `Refresh` snapshots only). A `Load(state, ctx)`
  hook was considered and **rejected**: merging `Mount` (load) with `OnConnect` (subscribe)
  conflates two genuinely different jobs. → Shipped as a "the published action is not a
  re-Mount" callout in `controller-pattern.md`; no new API.
- **C2. Guard→mutate→error-into-state→reload** action shape. ✅ **RESOLVED (no change).** The
  underlying mechanism — return a `FieldError`/`MultiError` and the framework surfaces it into
  state for the re-render — is already documented in `error-handling.md`. checklistkit's
  `mutate()` helper (the cited evidence) has since been **removed** from the app, so even the
  one prototype is gone. Generalizing the shape into a framework primitive would bake in an
  opinion about *when* to reload and *how* to structure the guard; the primitives it composes
  (`FieldError`, `Mount`-reload) are already first-class. → No primitive; mechanism already
  documented.
- **C3. Self-sync idiom** `ctx.Subscribe(ctx.SelfTopic())` + `ctx.Publish(ctx.SelfTopic(),
  "Refresh", nil)`. ✅ **RESOLVED (no change).** The idiom is already taught in **three**
  reference docs (`controller-pattern.md` fan-out section, `session.md` "Explicit Peer
  Refresh", `error-handling.md`). A `WithSelfSync()` auto-broadcast flag was considered and
  **rejected**: the explicit two-line idiom is discoverable and self-documenting, whereas an
  implicit "everything fans out" flag hides the opt-in boundary the framework deliberately
  makes explicit (see CLAUDE.md "peer fan-out is opt-in"). → No flag; idiom already documented.
- **C4. Removal candidate — duplicate session-store option.** ✅ **RESOLVED (shipped, #488).**
  Removed the `WithStore` `HandleOption`; the store now binds only at construction via
  `WithSessionStore`. See the Removal pass and "Shipped" list below.
- **C5. e2e harness capture gap** — checklistkit rebuilt a 276-line harness
  (`e2e_harness_test.go`) over the low-level `e2etest` primitives to get WS-frame + browser
  console + server-log capture that the shared `e2etest.Setup` lacks. ⏭️ **DEFERRED to the
  `lvt` repo.** The shared harness lives in `github.com/livetemplate/lvt` (`lvt/testing/`), not
  in core livetemplate — enriching it with capture hooks is an `lvt`-repo change, out of scope
  for this core-repo pass. → Track as an `lvt` follow-up.

**Surfaced by the prereview scan (new):**

- **C6. Disk-durable store distinct from `lvt:"persist"`.** ✅ **RESOLVED (docs, no new API).**
  The pitch was a disk-backed `SessionStore`. On audit the prereview evidence points the other
  way: prereview *keeps* its session-continuity state (Base, SelectedFile, DraftBody) on
  `lvt:"persist"` and moves **only** durable, per-user-global view prefs to a disk file
  (`internal/review/uiprefs.go`) — by its own design an **app-domain preference store, not a
  session-store gap**. And the `SessionStore` is keyed by cookie-derived `groupID`, so a disk
  `SessionStore` couldn't serve per-user prefs (different device → different cookie → different
  group) without re-keying. No app has hand-rolled a disk `SessionStore` — a speculative third
  impl with zero demonstrated consumers. The real gap was that the *boundary* (session
  continuity vs. durable app data) wasn't documented. → Shipped a "What the session store is
  not for" section in `session.md`; no new API.
- **C7. Template methods that accept arguments. ✅ RESOLVED (pattern, no core change).**
  The finding: prereview precomputes predicate results into `map[string]…` that the template
  looks up by key (`state.go:806-808,868-873`: `LineDisplay`, `BlockDiffStatus`,
  `ScrollHeadingBlockKey`), because it assumed a top-level `{{.LineDisplay $key}}` would not
  work. Investigation showed **arg-accepting methods already work — just not on the top-level
  State struct.** Hang them off a struct FIELD (a "view helper") and `{{.Views.LineDisplay $key}}`
  renders correctly in both render phases, deleting the precompute-map boilerplate.

  **Why top-level is unsupported, and why there is no small core fix.** LiveTemplate flattens
  State into a `map` (so it can inject the `{{.lvt}}` namespace and precompute zero-arg methods).
  That map is consumed by *two* renderers: `html/template` for the initial HTTP response
  (`renderHTML` → `context.ExecuteTemplateWithContext`, where the error is fatal) and the tree
  evaluator for WebSocket updates (`buildTree`). `html/template` **cannot call an
  argument-accepting method once its receiver is a map key** — it errors *"X is not a method but
  has arguments"* — and nothing stored in the map changes that (a stored `func` value is not
  called with args either). The obvious "wrap State in a synthetic struct so its methods survive"
  fix does not work: `reflect.StructOf` explicitly *"does not generate wrapper methods for
  embedded fields"*, so a dynamically-built wrapper would not expose the user's methods to
  `html/template`'s `MethodByName`. The remaining options (template-rewriting `.M arg` into a
  FuncMap call, or rendering the initial HTML from the tree instead of `html/template`) are
  render-engine surgery — disproportionate for a boilerplate item.

  **The working boundary** (locked by `template_arg_methods_test.go`): a struct-field / nested
  arg-method works in **both** the initial render and the update tree; a top-level State
  arg-method is unsupported and errors. This mirrors the framework's own idiom —
  `{{.lvt.AriaInvalid "field"}}` is exactly this pattern (`.lvt` is a struct value in the map).
  → **Resolution: document the view-helper pattern (guide + example), keep a regression anchor.
  No core API change.**
- **C8. Recursive `{{template}}` support (direct consumer: prereview; latent: tinkerdown).**
  📝 **DESIGN APPROVED (2026-07-15) — Phase 1 next.** livetemplate flattens `{{template}}` calls at
  parse time with no cycle guard, so a self-referential template stack-overflows during `Parse`; a
  recursive file tree drops to a standalone `html/template` injected as opaque `template.HTML`
  (prereview `internal/review/filetree.go`), which removes the whole subtree from reactive diffing.
  tinkerdown corroborates the recursive-tree UI pattern (recursive-Go `writeNavNode` →
  native-`<details>`) but serves static HTML, so it's a latent case, not a second bug victim. The
  one Tier C item with a genuine capability gap and a documented direct consumer → **full design
  written**:
  [`docs/proposals/recursive-templates-proposal.md`](../proposals/recursive-templates-proposal.md).
  Recommended approach: route recursive invocations through the runtime nested-`TreeNode` path
  `{{range}}` already uses (bounded by data depth, not parse-time expansion). Approach A
  (selective), a max-depth guard erroring uniformly, and content-hash `data-key` fallback are all
  decided (see the proposal's § Decisions); Phase 1 (crash → `ParseError`) is independently
  shippable and up next.
- **C9. Static-asset passthrough alongside the `/` LiveHandler.** ✅ **RESOLVED (docs, no new
  API).** The original pitch was a framework static-passthrough wrapper. On audit the common
  case — assets under a **known prefix** — is already handled by plain `net/http`:
  `http.Handle("/assets/", http.StripPrefix(…, http.FileServer(http.Dir(…))))` +
  `http.Handle("/", handler)`, and `ServeMux` longest-prefix-match routes them (shipped in the
  `avatar-upload` example; `http.Dir` already supplies traversal safety). Two independent apps
  confirm the prefix shape is the norm — tinkerdown routes everything under `/assets/` to its
  own handler (`internal/server/server.go:504`), and each app hand-rolls its file handler only
  for app-specific **policy** (embedded-first, extension allowlist, symlink eval), not for a
  missing framework primitive. Only **prereview** needs the harder *arbitrary-path* fall-through
  (`server.go:447-524`, `staticFallback`) because it serves user-authored content whose asset
  paths collide with SPA routes — a single-consumer, code-review-tool-specific case. A generic
  core wrapper would have to bake in one security policy that the two apps deliberately chose
  differently, so the fall-through stays an app-`main` concern. → Shipped as a "Serving static
  assets alongside the app" section in `controller-pattern.md`; no new API.

## Removal pass

The issue explicitly invites *removing* features, and v0.2.0 was itself an "API Reduction
Release", so subtraction is in-culture. Candidates:

- **C4 — collapse the two session-store options into one. ✅ RESOLVED.** `WithSessionStore`
  (a `New` `Option`) and `WithStore` (a `Handle` `HandleOption`) both set the same
  `SessionStore` field at different phases; `SessionStore` was the *only* dependency
  configurable at both levels (`Authenticator`, `PubSubBroadcaster`, etc. are `New`-only).
  No app or example used `WithStore` (only three sweep tests did). Removed `WithStore` and
  kept `WithSessionStore` — the store now binds at construction, consistent with every other
  dependency. Alpha + no external users → collapsed directly, no deprecation shim.
- **General audit:** the `Context` `With*()` builder family is large and mostly
  internal/test-facing (`WithHTTP`, `WithAction`, `WithConnectKind`, `WithData`,
  `WithFormSchema`, `WithFlashSetter`, `WithUploads`, `WithTopicSubscriber`). Worth a
  follow-up pass to confirm which need to be *exported* vs. package-internal — but that is a
  visibility review, not a behavior change, and is out of scope here.

No other clear-cut removals surfaced: the option set is large but each `With*` option maps to a
real, used capability.

## Recommended sequencing

1. **A1 + A2** — **done** (`WithParseFS`; `ctx` request-context guidance). Highest ratio of
   pain removed to risk taken.
2. **B1 (fanout)** — **done** (docs + example + regression test, **no new API**): the fanout
   capability already existed (`ctx.Subscribe` in Mount + out-of-band `handler.Publish`); the
   gap was discoverability. Shipped as livetemplate#485 + docs#103.
3. **B2 (client version pin)** — **in progress** (`client_assets.go`: `ClientVersion` +
   pinned `ClientScriptURL`/`ClientStyleURL`, **no bundling**). Removes a test-package-in-
   production smell + the unpinned-`@latest` wire-incompat risk from *every* app and example.
4. **B3 (Page / ListenAndServe)** — collapses the most raw lines; low conceptual risk.
5. **Tier C** individually as they come up. Most resolved to docs/no-core-change: **C4** a
   cheap removal, **C7** a documented view-helper pattern with a regression anchor, **C1** a
   lifecycle callout, **C9** a static-assets composition note, **C6** a session-durability
   boundary note; **C2** and **C3** needed nothing (mechanism/idiom already documented); **C5**
   is an `lvt`-repo follow-up. **C8** (recursive `{{template}}`) is the one item with a genuine
   cross-app capability gap — got its own full design, now **approved** with Phase 1 ready to
   implement.

## Shipped with this document (core `livetemplate` repo)

- **A1** — `WithParseFS(fs.FS, patterns...)` Option + `(*Template).ParseFS`, sharing a
  `parseSources` core with `ParseFiles` (see `template.go`; tests in `parsefs_test.go`,
  including an `embed.FS` golden-parity check against the `ParseFiles` output).
- **A2** — request-context guidance in `docs/references/controller-pattern.md` (the CRUD
  reference example now threads `ctx` into its DB calls). No new API surface: `*Context`
  already embeds `context.Context`, so an explicit `ctx.Context()` accessor was deliberately
  *not* added (redundant).
- **C1** — the "published action is not a re-Mount" lifecycle callout in
  `docs/references/controller-pattern.md` (fan-out section): `Mount` = subscribe + load; the
  published action = load only. No API change (the `Load` hook was considered and rejected).
- **C4** — removed the duplicate `WithStore` `HandleOption`; the session store now binds only
  at construction via `WithSessionStore` (`template.go`; sweep tests in `handle_test.go` updated).
- **C7** — the view-helper arg-method boundary, documented in
  `docs/references/controller-pattern.md` and locked by `template_arg_methods_test.go` (nested
  works in both render phases; top-level errors). No API change.
- **C9** — a "Serving static assets alongside the app" section in
  `docs/references/controller-pattern.md`: the stdlib `ServeMux` prefix composition for the
  common case, and why the arbitrary-path fall-through stays an app-`main` concern. No API
  change (the static-passthrough wrapper was considered and rejected — one policy can't fit the
  two apps' divergent choices).
- **C6** — a "What the session store is not for" section in `docs/references/session.md`: the
  session-continuity vs. durable-app-data boundary (the store resets on relaunch and is
  cookie-keyed, so durable/per-user data belongs in your own store). No API change (a disk
  `SessionStore` was considered and rejected — zero demonstrated consumers, and it wouldn't
  serve prereview's actual per-user need anyway).

Sibling-repo companions (land as coordinated follow-up PRs, per the lockstep convention):

- Convert 1-2 `docs/examples` recipes to `embed.FS` + `WithParseFS`, deleting the
  `extractTemplate()` temp-file dance. **Release-gated:** the `docs` repo is outside `go.work`
  and pins a *published* `livetemplate`, so this can only land once a release containing
  `WithParseFS` ships.
- Update the canonical `examples/` apps (todos, chat, …) off `context.Background()` onto
  `ctx`. Doable against the workspace today (the context embed predates this work); needs no
  release.

Everything in Tier B and C is **proposal-only** pending a per-item greenlight.
