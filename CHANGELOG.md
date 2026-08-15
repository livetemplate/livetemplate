# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.25.0] - 2026-08-15

### Changes

- docs: rewrite the guide and reference openers (#533) (c922f83a)
- test(census): pin associated-template coverage for separately-parsed sources (#473) (be26cab3)
- feat(census): advertise template lvt-* attributes so unhandled ones can warn (#473) (6f1bb1c1)



## [v0.24.0] - 2026-08-03

### Fixed

- **`ctx.GetBool()` now reads a checkbox on both transports.** It accepted
  `bool` and the strings `"true"`/`"false"` — and neither is what a checkbox
  actually sends. Over the WebSocket the client serializes a lone checkbox as
  `input.checked` (a bool; the `value` attribute is discarded), which worked.
  With the client not running, the browser posts the box's `value` attribute —
  `"1"`, or `"on"` when it has none — and `GetBool` read `false` for a ticked
  box. Since `GetString` does not accept a bool either, no accessor read a
  checkbox correctly on both paths, so a handler could not be written once and
  stay correct across them — the opposite of what the `progressive_enhancement`
  capability promises. `GetBoolOk` now accepts `"1"`/`"on"`/`"true"` as true and
  `"0"`/`"off"`/`"false"` as false (case-insensitive), plus numbers in every
  width `GetFloatOk` accepts — a numeric-looking hidden input is what the
  client's `parseValue()` turns into a number before sending it. `NaN` and
  `±Inf` are rejected rather than read as true, and a string that is neither
  boolean-shaped nor `1`/`0` (say `"2"`) stays unrecognized rather than being
  guessed at. An absent key still reads `(false, false)` — that is how an
  unchecked box arrives on the POST path, where it is not submitted at all.
  `docs/proposals/patterns.md` has documented `ctx.GetBool()` as the way to read
  checkbox state all along.

## [v0.23.0] - 2026-08-02

### Added

- **`LiveHandler.Func()` returns `ServeHTTP` as an `http.HandlerFunc`.** The
  value `Template.Handle()` returns already satisfied `http.Handler`, so
  `http.Handle`/`mux.Handle` worked, but the stdlib entry points that take a
  function — `http.HandleFunc`, and `ServeMux.HandleFunc` with Go 1.22 method
  patterns — required spelling out `handler.ServeHTTP`. `Func()` is that method
  value, so `http.HandleFunc("/counter", handler.Func())` and
  `mux.HandleFunc("GET /counter", handler.Func())` read naturally. It is an
  accessor, not a downgrade: `Shutdown`, `Publish` and `MetricsHandler` stay
  available on the `LiveHandler` it came from.

### Changed

- **A failed WebSocket upgrade now logs a `hint` when the `http.ResponseWriter`
  does not implement `http.Hijacker`.** An upgrade takes over the raw
  connection, so middleware that wraps the writer (logging, gzip, status
  capture) without forwarding `Hijack` breaks it — while GET and POST keep
  rendering, making the symptom "the page renders but never goes live". The
  underlying upgrader error names `http.Hijacker` but not the middleware that
  caused it; the hint does, and points at forwarding `Hijack` or leaving the
  writer unwrapped when `livetemplate.WSIsUpgrade(r)` is true. It is attached on
  the writer's own defect, which need not be what the accompanying error reports
  — an upgrader can reject a handshake earlier (a disallowed `Origin`) and never
  reach the hijack — so it is worded as a second failure the upgrade would have
  hit regardless, rather than as the reported cause.

## [v0.22.0] - 2026-07-26

### Added

- **`Async[S, R](ctx, work, apply)` — run expensive work off the event loop
  with type-safe on-loop state application.** The generic function spawns a
  goroutine for `work`, then dispatches `apply` back on the session's event
  loop when it completes. This replaces the manual two-action pattern
  (action triggers goroutine → goroutine calls `DispatchChan` → second action
  applies result) with a single call that handles goroutine lifecycle,
  panic recovery, and error propagation.

- **`{{.lvt.Pending}}` — framework-provided template variable for
  zero-boilerplate loading indicators.** On the render that registers Async
  work, `.lvt.Pending` is `true`; on all other renders it is `false`. This
  eliminates the need for a manual `Loading bool` field in state and the
  `s.Loading = true / s.Loading = false` bookkeeping across two actions.

## [v0.21.0] - 2026-07-23

### Added

- **`Validate(templateText)` reports template problems as structured
  diagnostics — including the parse and composition errors the live-render path
  silently swallows.** `Execute`/`ExecuteUpdates` catch a first-render failure
  and fall back to an HTML-structure tree, so a malformed template (an unclosed
  `{{range}}`, an unknown function, an unresolved `{{template}}`) renders
  degraded with no error returned — a tool that wants to reject a bad template
  *before* serving had nothing to call. `Validate(templateText string, opts
  ...ValidateOption) ([]Diagnostic, error)` parses the text through the real
  framework function set and any component templates supplied via
  `WithValidateComponents` — the same `ParseFS` path serve uses, so component
  definitions resolve rather than false-positive — and returns a
  `Diagnostic{Line, Severity, Message}` per problem (at most one today — the
  parser stops at the first error). The returned
  `error` is reserved for infrastructure failures (a component set that itself
  fails to parse); a template that does not parse is always a diagnostic, not an
  error, mirroring the shape of a linter. Because unknown functions are checked
  against the framework's own builtins — which a downstream consumer cannot
  enumerate — this check cannot be reproduced outside the module. Data-dependent
  checks (render behaviour against a sample value) are out of scope for now, and
  `SeverityWarning` is reserved for them.

## [v0.20.1] - 2026-07-20

### Fixed

- **The pinned browser client is no longer three releases behind, so the upload
  field-serialization fix actually reaches applications.** `ClientVersion` — the
  constant `ClientScriptURL` is built from, and therefore what every app using
  the documented `<script src="{{ .ClientScriptURL }}">` integration loads —
  still pointed at client 0.18.2 while the client had shipped 0.19.1 and 0.20.0.
  0.18.2 predates the opt-in change to upload form fields, so an application on
  the documented path was still serializing its entire enclosing form into every
  Proxied upload (everything except `type="password"`), including CSRF tokens and
  hidden secrets. Pinned to 0.20.0. Applications that self-host or pin the client
  themselves were unaffected; those following the default were not.
  ([client#150](https://github.com/livetemplate/client/pull/150), #452)

### Documentation

- **`lvt-upload-with` is documented where the docs site actually reads from.**
  The upload reference and the client-attribute tables live here and are mirrored
  into the docs site on release; the opt-in marking contract had been written
  into the mirror instead, where the next sync would have deleted it. The
  `UploadStreamer` godoc also still described the pre-opt-in behaviour, which is
  the copy an `OnUpload` implementer is likeliest to read. (#452, #508)

## [v0.20.0] - 2026-07-20

### Fixed

- **Template auto-discovery no longer fails when a directory disappears while it
  is being searched.** `livetemplate.New()` walks the template directory, and any
  error from that walk aborted it — including a transient directory being removed
  by something else at that moment, which surfaced as
  `template auto-discovery failed: readdirent …: no such file or directory` from
  an unrelated part of the app. A path vanishing underneath the walk is now
  skipped rather than fatal. Deliberately narrow: a missing *base* directory
  still errors, since that means the caller pointed at somewhere that does not
  exist, and non-ENOENT failures such as permissions still surface. `.uploads` is
  also skipped outright, being uploaded files rather than templates. (#502)
- **Range items whose keyed element sits under more than one wrapper keep their
  stable identity.** Nested constructs stack wrappers, so
  `{{if}}{{if}}<li data-key="…">` puts the keyed element two levels below the
  range item; key lookup only looked one level down and fell back to hashing the
  item's content. Because a content hash changes when the content does, editing
  such an item changed its key and the client removed and re-inserted the row
  instead of patching it — the churn `data-key` exists to avoid. Lookup now
  descends through nested wrappers, bounded at four levels. Items with no key at
  any depth still use content hashes, as before. (#505)

### Internal

No behaviour change from these, but they are where the two fixes above came from:
`FlattenTemplate` now hands recursion defines back to its caller rather than
appending them itself (#503), and wrappers carry an explicit kind instead of
being inferred from tree shape (#504) — the inference was whitespace-sensitive,
which is what let #505 through. Release tooling also refuses to bump on top of an
unpublished release (#501) and restores its files when a run aborts (#499).

## [v0.19.1] - 2026-07-18

Documentation-only. No library behavior changes; released so the docs site,
which mirrors this repo's markdown per release tag, stops describing v0.19.0's
headline feature as unsupported.

### Fixed

- **The template support matrix said recursive templates were rejected**, in the
  release that shipped them. The row had been updated to the parse-time-rejection
  wording during C8's first phase and never revised once runtime invocation
  landed. It now records what is actually supported — direct self-recursion,
  mutual recursion, longer cycles, and a self-referential entry point — along
  with a **Recursion depth** section covering the cap and two behaviors that
  differ from a plain error: a finite tree deeper than the cap degrades silently
  on first render, and a build error on first render drops that region to
  HTML-string diffing for the life of the template. (#498)
- `LVT_MAX_TEMPLATE_DEPTH` / `WithMaxTemplateDepth` had no entry in the
  configuration reference at all. (#498)
- `checkFlattenCycle` reported "recursive template invocation is not supported"
  and its comment claimed livetemplate "does not yet evaluate recursive
  invocations at runtime". Neither holds since v0.19.0, and the check is no
  longer reachable through `FlattenTemplate` — `detectRecursiveTemplates` runs
  first over the same invocation edges, so a cycle member is emitted verbatim
  rather than pushed onto the inlining stack. Reworded as the internal backstop
  it now is. (#498)



## [v0.19.0] - 2026-07-18

### Added

- **Recursive `{{template}}` invocation** — self-referential templates (file trees, comment threads, nested navigation) now render. Previously they were rejected at parse time, because `{{template}}` calls are inlined during flattening and a self-referential template cannot be inlined. The parser now detects self-referential invocation cycles, leaves them un-inlined, and evaluates them at build time as nested `TreeNode`s, so the recursive region stays inside the reactive tree rather than degrading to opaque HTML. Dot-rebinding matches `html/template` semantics. (Tier C · C8)
- `WithMaxTemplateDepth(n)` `Option` and the `LVT_MAX_TEMPLATE_DEPTH` environment variable cap how deep recursive `{{template}}` invocations may nest while building the tree, so unbounded recursion (e.g. self-referential data) surfaces a clear error instead of overflowing the stack. Defaults to 128; raise it only if your data is legitimately deeper.

### Changed

- **The differential range path now emits a per-item `["u", key, <recursive diff>]` for kept-but-changed items** instead of replacing the item's whole subtree. This is framework-wide and benefits *every* nested or heterogeneous data-keyed range, not just recursive templates: a deep edit now scopes to a nested `["u"]` chain down to the changed leaf (~24KB → ~200B on a depth-5 tree, ~100× smaller than the previous opaque re-send). No application change is required. The format is backward-compatible with published clients — `@livetemplate/client` has merged nested `"u"` payloads recursively via `deepMergeTreeNodes` since v0.8.2 — and is validated by the TypeScript-oracle fuzz suite running against the real client.
- Recursive range items are keyed by their real `data-key` read through the invocation wrapper, giving stable per-item identity across renders instead of falling back to positional keys.
- `ClientVersion` is bumped `0.16.5` → `0.18.2`, adopting the current published `@livetemplate/client`. Wire-compatible in both directions for this release's format; the bump additionally picks up two client fixes — `lvt-el` preserving client-applied class/attr state across morphs, and bare-key `lvt-on:keydown` shortcuts no longer firing while Ctrl/Meta/Alt is held.

### Fixed

- A **full-HTML-document template using a recursive `{{template}}` silently fell back to HTML-string diffing** instead of taking the reactive tree path. Body extraction dropped `FlattenTemplate`'s trailing `{{define}}` blocks, so the recursive invocation had no definition to resolve against; the trailing blocks are now re-attached during extraction.

### Removed

- `WithStore(store)` `HandleOption` — removed in favor of the existing `WithSessionStore(store)` `New` `Option`. Both set the same `SessionStore` field, just at different phases (per-`Handle` vs per-`New`), and `SessionStore` was the only dependency configurable at both levels — every other dependency (`Authenticator`, `PubSubBroadcaster`, allowed origins, …) binds only at `New`. No app or example used `WithStore`; the session store now binds at construction like everything else. Migrate `tmpl.Handle(ctrl, state, WithStore(s))` to `New(name, WithSessionStore(s))`. (#483)

### Documentation

- Recursive-template design proposal (#493), Tier C dispositions for C2/C3/C5/C6 (#492), serving static assets alongside the app (#491), the arg-method view-helper pattern (#488), and a clarification that a published action is not a re-`Mount` (#490).

## [v0.18.1] - 2026-07-13

### Fixed

- An **unmatched range whose item statics changed** had its update silently dropped: the client kept rendering stale items until a full page load, with no error anywhere. `handleNestedTreeNodes` exempted range-bearing subtrees from the "statics changed → send the full tree" branch, on the assumption that a range's item changes always travel through the range-diff operations. They only do for a **matched** range: `FindRangeConstructMatches` pairs ranges by signature (= the item statics), so a range whose item statics change shape does not match — and the fall-through recursion then finds nothing to send, because a range's content lives in `Range.Items`, not `Dynamics`. An item's statics are data-dependent whenever its body has a slot that can vanish: a nested `{{template}}` that renders nothing for an empty argument contributes no dynamic, so the item's statics differ between the render where it is empty and the one where it is not (the reported case: a comment card whose conversation thread renders only once a reply exists — the reply never reached the open page). The guard is now simply `if structureChanged`, which is what it always meant: if the statics differ, the client cannot render the subtree from its cache, so it needs the whole thing. Matched ranges are unaffected — they never reach this branch — and keep their item-diff operations. This fixes the statics-shape-change case; signature matching itself is unchanged. (#489)
- A **nil `lvt:"persist"` field no longer discards the entire restored state** on reconnect. A field that is nil when the state is saved is written as JSON `null`, and json-iterator decodes `null` into a **zero-length** `json.RawMessage` where `encoding/json` yields the literal `"null"`. `InjectPersistFields` fed those empty bytes to `Unmarshal` and errored (`ReadMapCB: expect { or n, but found \x00`); because it returns on the first error and `restorePersistedState` drops the state when it does, **one** nil field silently reverted **every other** persist field to its zero value — with only a server-log line to show for it. The trigger is ordinary: any persist-tagged map or slice nobody has written to yet. `null` carries nothing to apply, so it is now skipped and the field is left at its zero value, which is exactly what it round-trips to. This regressed in v0.17.0 with the migration of the remaining `encoding/json` callsites to `jsonutil.API` (#231), whose "wire output is unchanged" note held for encoding but not for this decode path. (#489)

### Changed

- Clarified that `WithDevMode(true)` already relaxes the WebSocket origin check to allow **all** origins — so pairing it with `WithPermissiveOriginCheck()` for local development is redundant. This was always the behavior (`createSecureOriginChecker` short-circuits to allow-all when dev mode is on, and `New` installs it on the default upgrader), but the `WithDevMode` godoc, the `DevMode` field/config comments, and the `LVT_DEV_MODE` reference docs previously described it as "uses local client library instead of CDN" — a stale claim that never reflected core behavior and led apps to hand-append `WithPermissiveOriginCheck()`. The godoc now leads with the security semantics ("allows all WebSocket origins, disabling the same-origin/CSRF check — never in production"), notes it also exposes `{{.lvt.DevMode}}` to templates, and `WithPermissiveOriginCheck`'s godoc points local-dev users to `WithDevMode` instead, reserving itself for the disable-origin-check-*without*-dev-mode case. A new `TestWithDevModeWiresPermissiveOrigin` locks the wiring through `New()` (dev mode alone allows cross-origin; no options rejects it). No behavior change. (#483) (#487)

## [v0.18.0] - 2026-07-12

### Added

- `ClientVersion`, `ClientScriptURL`, and `ClientStyleURL` exported constants pin the `@livetemplate/client` browser bundle this LiveTemplate release is wire-compatible with, plus two framework-seeded template functions — `lvtClientScriptURL` and `lvtClientStyleURL` — that render those URLs. Templates can now reference `{{lvtClientScriptURL}}` / `{{lvtClientStyleURL}}` with no per-app wiring (the funcs are seeded into every template's FuncMap in `New`, before any parse path runs, so they resolve in full-HTML documents, fragments, and component templates alike; a user `Funcs` call still overrides them by key). This replaces the previous pattern of hardcoding an unpinned `@latest` CDN URL — which was unsafe because there is no runtime server↔client version handshake, so a client-only release could ship a wire-protocol change to browsers still talking to an older server. Pinning moves the client version only on a deliberate `go get -u`, in lockstep with the compatible server. `ClientVersion` is pinned to the release `@latest` resolves to today, so behavior is preserved. Self-hosters (offline / air-gapped / CSP-strict) vendor `@livetemplate/client@<ClientVersion>` and serve it from their own origin instead. (#483)

## [v0.17.0] - 2026-07-10

### Added

- `WithParseFS(fsys fs.FS, patterns ...string)` `Option` and `(*Template).ParseFS(fsys, patterns...)` parse templates directly from an `fs.FS` (e.g. an `embed.FS`), so an app can ship templates embedded in its binary without staging them to a temp directory first. `WithParseFS` takes precedence over `WithParseFiles` and auto-discovery; the first resolved file is the main template and the rest compose into the same set (same semantics as `ParseFiles`). `ParseFiles` and `ParseFS` now share an internal `parseSources` core, so `ParseFiles` behavior is unchanged. (#483)

### Changed

- Zero-arg State methods are now precomputed only when a template actually references them, instead of on every render regardless. Previously `BuildDataMap` eagerly evaluated **every** exported zero-arg method of the State struct on every render — because the struct is converted to a map, and `html/template` cannot auto-call methods on a map — so an expensive or side-effecting helper method that no template rendered still ran. The render path now computes, once at parse time, the set of identifiers referenced across the templates (a deliberate over-approximation: it unions every field/chain/variable identifier plus every string literal, so a method reached via `{{index . "Name"}}` is still included) and skips precomputing any method whose name is absent. Rendered output is byte-identical for templates that reference the methods they use. Two limits worth knowing: a method referenced only under a false branch (`{{if .Show}}{{.Expensive}}{{end}}`) is still evaluated because its name appears in the text; and a method name that collides with an unrelated field or string literal is still (harmlessly) precomputed. Direct callers of `BuildDataMap`/`ExecuteTemplateWithContext` pass `nil` for the new allow-set to keep the original precompute-all behavior. (#462, #255)
- Migrated the remaining production JSON callsites (`action.go`, `state.go`, `mount.go`, `health.go`, `session_stores.go`, `topic_runtime.go`, `pubsub/redis.go`, `internal/upload/protocol.go`) from `encoding/json` to the shared `jsonutil.API` (json-iterator, `ConfigCompatibleWithStandardLibrary`), so the production paths now go through one JSON library. Wire output is unchanged (same HTML-escaping and `Encoder.Encode` trailing newline). Some callsites deliberately keep `encoding/json` and are **not** migrated: `internal/keys/hash.go` uses it as a canonical byte-exact reference for hash-key stability (its `Wire-stability invariant` forbids any encoder whose bytes could differ), `internal/fuzz/*` (three files) uses stdlib as the oracle it validates json-iterator against — swapping either would defeat its purpose — and `e2e/docker/app/main.go` is a demo/e2e fixture app rather than library code. One behavioral nuance: `state.go`'s `MarshalBinary`/`ExtractPersistFields` serialize arbitrary user application state, and json-iterator does not detect reference cycles the way `encoding/json` does — a cyclic user-state struct that previously returned a marshal *error* now overflows the stack. Such state is already non-persistable under either library; the change is graceful-error → crash for that already-broken edge case. (#231)

### Fixed

- A bare template reference to an argument-requiring method (e.g. `{{.Get}}` where `Get(key string) string`) now errors like `text/template` (`wrong number of args for Get: want 1 got 0`) instead of silently stringifying the uncalled method as a Go func value. A bare reference to a *variadic* method (`{{.Tags}}` for `Tags(...string)`) is now called with no arguments, also matching `text/template`. The with-args method path (`{{.ctx.Get "key"}}`) is unchanged. The fix lives at the eval-node altitude in `internal/parse/eval.go`, where the presence of trailing args is known — `callMethod`/`resolveFieldChain` run before that and so cannot distinguish "awaiting args" from "bare reference". (#203, #459)



## [v0.16.0] - 2026-07-05

### Added

- `__ping__` reserved WebSocket action for a client-driven liveness heartbeat: the event loop replies with a fixed `{"pong":true}` and skips the action/render pipeline (short-circuited after the message rate limiter). Browsers can't send WebSocket ping control frames from JS, so this app-level round-trip lets a client detect a dead — or zombie (reports OPEN while the TCP is gone, fires no close event) — socket by the absence of a pong and reconnect. Non-heartbeat clients see no behavior change. Documented in `docs/references/navigate.md`. (#477)
- Same-origin WebSocket origin checks now recognize the standard RFC 7239 `Forwarded` header (its `proto` parameter) as a fallback for scheme detection when the de-facto `X-Forwarded-Proto` is absent or invalid. `X-Forwarded-Proto` keeps precedence, and both headers remain behind the existing `WithTrustForwardedHeaders` gate, so a direct (unproxied) client cannot forge either to upgrade the detected scheme. Parsing takes the first (client-most) element's `proto`, is case-insensitive on the parameter name, and accepts quoted values. (#198)

### Changed

- Cap the capacity of pooled render buffers so a single large render no longer keeps an oversized buffer retained in the pool for reuse across later (typically smaller) renders. (#226)

### Fixed

- `livetemplate_publishes_sent_total` now actually increments. From v0.11.0 through v0.15.0 the counter (and its predecessor `livetemplate_broadcasts_sent_total`) was defined but never wired to a production call site, so it always reported 0 — a footgun for operators with dashboards or alerts on peer-fan-out rate. It is now incremented once per receiving connection whenever a peer-fan-out dispatch is enqueued, covering `ctx.Publish`, `Session.TriggerAction`, and cross-instance group/topic re-fan-out. The count is per instance (each instance counts only its own local deliveries, so a clustered publish is not double-counted) and is recorded at enqueue time, so a downstream slow-client close can still drop the resulting WebSocket send. (#432)



## [v0.15.0] - 2026-06-29

### Added

- `ctx.Submitter()` returns the name of the control that submitted the form (`SubmitEvent.submitter.name`), mirroring `ctx.Action()`. Under button-name routing it equals the action; under `lvt-on:submit` routing the action is the handler while the submitter is the clicked button. Lets `BindAndValidate` flows branch on the submitter the same way `ValidateForm` honors `formnovalidate`. (#239)
- Action routing now accepts **kebab-case** action names, so a method `SaveDraft` routes from `save-draft` (in addition to the existing `saveDraft`/`save_draft`/`SaveDraft`). This makes the documented progressive-enhancement button-name pattern — `<button name="save-draft">` — route on the no-JS tier, where the button `name` is the action verbatim. (#239)
- `ctx.ValidateForm()` now honors `formnovalidate` on submit controls: when a form is submitted by a button/input carrying `formnovalidate` (e.g. `<button name="save-draft" formnovalidate>`), validation is skipped. The framework records the control's `name` from the template into the form schema (`FormSchema.NoValidateSubmitters`) and matches it against the submission's submitter, so the skip works on every tier — WebSocket, HTTP-fetch, and no-JS native POST — with no client change. Only named submitters are detected; on the no-JS tier the button must not carry a `value` (the submitter is identified by its empty-value field). The skip is client-controlled convenience, not a security boundary. (#239)
- `WithEphemeralSweepTTL(ttl)` `HandleOption` to tune how long idle HTTP template cache entries survive in ephemeral mode (state with no `lvt:"persist"` fields) before the sweep loop evicts them. Defaults to 30 minutes; no effect in persistent mode (eviction follows the SessionStore there). A short TTL also tightens the sweep interval (floored at 1 minute) so the value is actually honored. (#304, #305)

### Changed

- **Breaking (`FormRule`):** unified the optional-bound sentinels. `MinLength`/`MaxLength` no longer use `-1` to mean "not set"; all four numeric bounds are now gated by paired `Has*` booleans — new `HasMinLength`/`HasMaxLength` join the existing `HasMin`/`HasMax`. Code that constructed a `FormRule` with `MinLength: -1` (or read it expecting the `-1` sentinel) must switch to the `Has*` flags. Callers using `ExtractFormSchema`/`ValidateForm` normally are unaffected. (#241)
- **Breaking (pubsub):** renamed broadcaster handler-registration methods to use the `Register*Handler` convention, disambiguating one-time handler registration from the per-entity, reference-counted `SubscribeTo*` subscription methods. Implementers and callers of these `pubsub` interfaces must update:
  - `Broadcaster.SubscribeServerActions` → `Broadcaster.RegisterServerActionHandler`
  - `GroupActionBroadcaster.SubscribeGroupActions` → `GroupActionBroadcaster.RegisterGroupActionHandler`
  - `TopicActionBroadcaster.SubscribeToTopicActions` → `TopicActionBroadcaster.RegisterTopicActionHandler`
- `WSIsUpgrade` now fully validates the RFC 6455 handshake: in addition to the GET method and `Connection: upgrade` / `Upgrade: websocket` headers, it requires a non-empty `Sec-WebSocket-Key` and `Sec-WebSocket-Version: 13`. Requests missing these are routed as plain HTTP (they would have failed at `Upgrade()` anyway); well-behaved WebSocket clients always send them. (#243, #244, #245, #247)

### Fixed

- HTML comments (`<!-- ... -->`) are now stripped from template output, matching `html/template` (which removes them during its escape pass). LiveTemplate builds statics by walking the raw parse tree, which never triggers that pass, so developer/internal comments previously shipped verbatim to the client (visible in view-source) and over WebSocket tree updates. Stripping uses the HTML tokenizer, so comment-like text in attribute values is preserved and comments inside `<script>`/`<style>`/`<textarea>` are left verbatim. See `docs/references/template-support-matrix.md` for residual divergences. (#468)
- Dynamic content (HTML **and** plain text) is no longer minified or whitespace-normalized before being placed in the tree. The minifier (`tdewolff/minify`) is HTML-tag-aware but CSS-blind, so it silently collapsed whitespace inside elements made whitespace-significant by a CSS class (e.g. `white-space: pre-wrap` on `<span class="chroma">`), corrupting syntax-highlighted code, diffs, and ASCII art rendered through the structure-based/diff tree paths; text-only dynamics were likewise trimmed and space-collapsed. Dynamic content now passes through verbatim, matching how statics are already handled. The `internal/render.MinifyHTML` helper and the `github.com/tdewolff/minify/v2` dependency were removed. (#467)
- `NewGorillaUpgrader` now gives each upgrader its own write-buffer `sync.Pool` instead of sharing a package-level global, so upgraders configured with different `WriteBufferSize` values can no longer draw mismatched buffers from a shared pool. (#243)
- `ExtractFormSchema` no longer emits a validation rule for submit controls (`<input type="submit|image|button|reset">`). Such controls carry no payload field, so a `required` (or other validation attribute) on a named submit input previously produced a `FormRule` that could raise a spurious "field is required" error whenever that control was not the submitter. This is a behavior change to a public function, independent of the `formnovalidate` skip. (#239)

## [v0.14.0] - 2026-06-13

### Changes

- feat(upload): WS-disabled fallbacks for Volume(Dir) and Direct completion (#448, #449) (#455) (a238fdfb)
- docs(uploads): note SSR'd lvt-upload inputs bind on connect (#453) (#454) (08a84bad)



## [v0.13.0] - 2026-06-11

### Changes

- chore(changelog): drop Unreleased block ahead of release (615c112e)
- feat(upload): expose preceding form fields to OnUpload (Proxied streaming) (#451) (e5ecb1a6)



## [v0.12.0] - 2026-06-10

### Changes

- feat(upload): four upload modes — Direct/Proxied/Volume/Preview (#447) (#450) (2c6ceef4)
- docs: compare against templ + htmx, tighten the standard-HTML claim (#446) (66a58b9d)



## [v0.11.2] - 2026-06-01

### Changes

- chore(changelog): drop Unreleased block ahead of release (77518d80)
- feat(context): lvt.Redact template helper for Preview-mode field redaction (#445) (baddc874)
- feat(context): support relative redirects so recipes can redirect-to-self under StripPrefix (#443) (b1ff8591)
- docs: TriggerAction reconnect-gap contract — proposal + Option C+ adoption (#441) (5a9f23c3)
- docs(proposals): wasm-target — supersedes #440 (Draft) (#442) (9ef95f78)
- fix(build): tokenizer-based block-tag boundary detection (closes #436) (#439) (3796f69e)
- ci: drop test-examples job (livetemplate/examples repo archived) (#438) (ec88e4bb)
- ci: collapse 5 per-docs-recipe jobs into one test-docs-examples job (#437) (b96a5a27)



## [v0.11.1] - 2026-05-23

### Changes

- fix(build): tokenizer-based body/script detection (closes #414) (#435) (b28a8b08)
- fix(diff): preserve AutoKey in PrepareTreeForClient strip path (5efb576f)



## [v0.11.0] - 2026-05-22

### Changes

- chore(changelog): restructure v0.11.0 entry as subsections for release.sh prepend (86c75e1e)
- feat(observe)!: rename broadcasts_* metric family to publishes_* (v0.11.0) (#431) (e025ab6c)



### Breaking Changes

- **Prometheus metric family renamed**: `livetemplate_broadcasts_sent_total` is now
  `livetemplate_publishes_sent_total`. The rename reflects the post-v0.10.0 `ctx.Publish` /
  `ctx.Subscribe` API and removes the lingering ambiguity between the old `BroadcastAction`
  vocabulary and the new pub/sub vocabulary. The metric's help text now reads
  *"Total number of peer-fan-out publishes sent (ctx.Publish)"*.
- **Alert renamed**: the example alert `BroadcastFailures` in `docs/guides/OBSERVABILITY.md`
  is now `PublishFailures`; the condition expression uses `publish_errors_per_minute`.
- **Internal (unexported) metric API renamed**: callers of `internal/observe` —
  `(*observe.Metrics).BroadcastSent()` → `PublishSent()`, and
  `MetricsSnapshot.BroadcastsSent` → `PublishesSent`. External users importing only the
  public `MetricsHandler()` API are unaffected.

There is no dual-emit period. Operators must update dashboards, recording rules, and alert
configurations in lockstep with the deploy. See the
[Metric Migration section in OBSERVABILITY.md](docs/guides/OBSERVABILITY.md#metric-migration-v010x--v0110)
for the sed one-liners.

### Terminology cleanup

Scrub residual "broadcast" vocabulary from non-history docs/comments where it was
ambiguous next to the new pub/sub API: `WithPubSubBroadcaster` doc comment (which
referenced now-removed `BroadcastTo*` methods), `WithDispatchBufferSize` doc, package
doc comment, `Context.ConnectKind` comment, `CONTRIBUTING.md` directory map,
`docs/guides/SCALING.md`, `docs/guides/OBSERVABILITY.md`, `docs/guides/standard-html-reactivity.md`,
`docs/references/pubsub.md` (the "Broadcast Scopes" heading is preserved because it documents
the literal `livetemplate:broadcast:*` channel namespace), `docs/references/error-handling.md`,
`docs/references/api-reference.md`, `docs/guides/new-contributor-walkthrough.md`, and
`CLAUDE.md`. Public API type/identifier names (`Broadcaster`, `RedisBroadcaster`,
`WithPubSubBroadcaster`, `pubsub.BroadcastMessage`) are unchanged; wire-format Redis
channel names (`livetemplate:broadcast:*`) are unchanged.

## [v0.10.1] - 2026-05-21

### Changes

- Phase 6: scrub BroadcastAction substrings + structured slog key (v0.10.1-prep) (#430) (84c85a36)



## [v0.10.0] - 2026-05-20

### Changes

- feat(broadcast)!: Phase 5 — remove ctx.BroadcastAction; migrate call sites to Subscribe/Publish self-topic (#429) (ad7ff04e)



## [v0.9.2] - 2026-05-20

### Changes

- chore(scripts): bump release.sh test timeout to 300s to match pre-commit (#428) (1718bf5f)
- feat(topics): Phase 4 — keep-open on ACL-denied Subscribe in WS-connect Mount + V14 Tier-1 (#415) (#427) (3986586b)
- feat(topics): Phase 3 — multi-segment wildcards (PSUBSCRIBE + seen-ring dedup) (#426) (6b8d0f2d)
- feat(topics): Phase 2 — cross-instance Redis topic Pub/Sub (#424) (b7b1ab08)
- fix(pubsub): synchronize the subscribeHook test seam (data race, #422) (#423) (cec0d3ea)
- feat(topics): Phase 1 — Context API + topic ACL (single-instance) (#419) (9836d116)
- fix(pubsub): synchronize the reconnectHook test seam (data race, #420) (#421) (1bca9aef)
- test: skip TestRangeBuildLatency_PostPhase7 under -race (pre-existing flake) (#418) (61a44c32)
- feat(topics): Phase 0 — pub/sub registry foundations (#417) (fcaa92b2)
- docs: add per-phase audit + learnings loop to the BroadcastAction redesign plan (#416) (19774be3)
- docs: pub/sub topic model — converged design & implementation specification (#415) (c01d4a23)
- docs: redesign BroadcastAction as a single Publish/Subscribe topic model (#412) (eb0de7e7)
- docs: propose BroadcastAction redesign with implicit peer sync and topics (#411) (124f1d99)



## [v0.9.1] - 2026-05-14

### Changes

- feat: lifecycle/Context/Session ergonomics helpers (closes #339 #340 #341 #345) (#408) (1e812e76)
- ci: add test-docs-login + test-docs-shared-notepad cross-repo jobs (#407) (12509a52)



## [v0.9.0] - 2026-05-12

### Changes

- refactor: remove reserved Sync action (#406) (8fc9467b)
- ci: add test-docs-progressive-enhancement cross-repo job (#403) (f62a44f7)
- ci: add test-docs-todos cross-repo job for todos recipe coverage (#402) (13d5fe66)
- docs(proposals): collapse explicit-submitter to two-phase rollout (#237) (4a22c178)
- ci: add test-docs cross-repo job for patterns e2e coverage (#400) (a65cc9de)
- test(pubsub): migrate time.Sleep to require.Eventually polling (#216) (#398) (9abe2feb)
- refactor(session): typed flashKey to prevent key-space confusion (#347) (#397) (4df7ca6e)
- feat(send): explicit submitter field on the wire (#237 Phase 1) (#396) (d060ead5)
- feat(session): observability log on chained TriggerAction (#337 Option A) (#393) (4ccc3af6)
- feat(capabilities): advertise validate, upload, progressive_enhancement (#252) (#395) (5bbcff52)
- fix(pubsub): refcount channel subscriptions, unsubscribe on disconnect (#214) (#394) (24b22513)
- fix(pubsub): gate handler registrations on Subscribe() success (#357) (#391) (0c999a10)
- docs: P2 cluster — TreeNode breaking change, lvt-submit deprecation, CSRF note (#390) (904a3bbb)
- fix(mount): use r.Context() for WS lifecycleCtx (#303) (#392) (384e043e)
- fix(mount): auto-wire FormSchema into Context (#236) (#388) (4126d285)
- docs(flash): update lifecycle to persist-until-cleared (#343) (#387) (15a9ba5c)
- fix(pubsub): release mutex before Redis network calls (#215) (#389) (e7ce1c9c)
- docs(proposals): livetemplate.css semantic-tag coverage scope (#317) (#384) (b1d2d7fc)
- docs(proposals): lvt-hook scope and elevation assessment (#294, #43) (#383) (03529aa1)
- docs(proposals): explicit submitter on the wire (#237) (#382) (4f6b01ba)
- docs(README): cross-link to https://livetemplate.fly.dev docs site (#381) (b3f7e86f)
- refactor: remove deprecated generateTreeInternalWithErrors wrapper (#373) (#380) (1d96cd9f)
- test: concurrent invalidate+read fingerprint stress + bench helper pool (#206, #374) (#379) (ad82432e)
- test: BroadcastAction inside Mount on __navigate__ path (#346) (#378) (6dbee442)
- chore: Phase 1A cleanup — extract button-name helper, migrate goldens (#327, #323) (#377) (86a33c67)
- ci: dispatch livetemplate/docs sync on release tag (#376) (b0274219)
- chore: enable unparam in pre-commit gate (Phase 2+3 of #367) (#375) (5635ad1e)
- refactor: clean 32 unparam findings (Phase 1 of #367) (#372) (344f7630)
- docs: rewrite README + add standard-HTML guide and navigate reference (#268, #349) (#371) (9465400d)



## [v0.8.23] - 2026-05-02

### Changes

- refactor: streaming range Phase 8.5 — remove dead keyGen plumbing (#370) (2ef24d11)
- perf: streaming range Phase 7 — type-direct hash + parallel build (#369) (24950bf9)
- feat: streaming range Phase 6 — recursive transition + LargeTable demo (#368) (900d1da8)
- feat: streaming range Phase 5 — benchmark gate + measured §7 numbers (#366) (73cff639)
- feat: streaming range Phase 4 — cleanup + spec update (#365) (45756b72)
- feat: streaming range Phase 3 — caller integration (cutover) (#364) (6d46c35e)
- feat: streaming range Phase 2 — diff entry point (callable, unwired) (#363) (c07977d9)
- feat: streaming range Phase 1 — foundational types (no-op) (#362) (2a28b70d)
- docs(proposals): Phase 0 audit for streaming range rendering (#361) (f075d7b5)
- docs(proposals): streaming range rendering (#360) (89688276)
- docs(proposals): record lvt-scroll-away top edge ship + Pattern #10 status (a5a5b4bc)
- docs(proposals): tick Session 7 boxes + Implementation Notes (895865f1)
- docs(proposals): tick Session 6 boxes + Session 6 implementation notes (83226ab2)
- docs(proposals): patterns Session 5 complete + 3 implementation notes (d6efdacc)



## [v0.8.22] - 2026-04-25

### Changes

- chore: ignore .claude/scheduled_tasks.lock (Claude Code transient state) (b6cb4f52)
- fix: prune expired flash before render (not after sendUpdate) (#359) (ad5f1071)
- docs(proposals): patterns Session 4 complete + 6 implementation notes (747aedce)




<a name="v0.8.21"></a>
## [v0.8.21] - 2026-04-22

### Bug Fixes

- eliminate race in Redis pub/sub init and add subscription retry ([#355](https://github.com/livefir/livetemplate/issues/355))

### Documentation

- scroll effect targeting, lvt-scroll-away, and chat recipe ([#356](https://github.com/livefir/livetemplate/issues/356))
- **proposals:** update patterns for v0.8.19 + v0.8.33 ([#358](https://github.com/livefir/livetemplate/issues/358))


<a name="v0.8.20"></a>
## [v0.8.20] - 2026-04-21

### Documentation

- update scroll-sentinel to lvt-scroll-sentinel attribute ([#352](https://github.com/livefir/livetemplate/issues/352))
- automatic client-side state preservation ([#351](https://github.com/livefir/livetemplate/issues/351))


<a name="v0.8.19"></a>
## [v0.8.19] - 2026-04-18

### Documentation

- **proposals:** Session 3 complete + server-push pattern lessons ([#338](https://github.com/livefir/livetemplate/issues/338))

### Features

- __navigate__ action + flash persist-until-cleared lifecycle ([#344](https://github.com/livefir/livetemplate/issues/344))


<a name="v0.8.18"></a>
## [v0.8.18] - 2026-04-14

### Bug Fixes

- wire Session.TriggerAction into lifecycle contexts ([#336](https://github.com/livefir/livetemplate/issues/336))
- **ci:** update [@livetemplate](https://github.com/livetemplate)/client to latest in cross-repo tests ([#328](https://github.com/livefir/livetemplate/issues/328))

### Documentation

- patterns example proposal ([#333](https://github.com/livefir/livetemplate/issues/333))
- update dialog routing with polyfill context ([#331](https://github.com/livefir/livetemplate/issues/331))
- README rewrite proposal ([#268](https://github.com/livefir/livetemplate/issues/268)) ([#332](https://github.com/livefir/livetemplate/issues/332))
- comprehensive documentation overhaul ([#329](https://github.com/livefir/livetemplate/issues/329))
- **proposals:** patterns session 2 tracker ([#335](https://github.com/livefir/livetemplate/issues/335))
- **proposals:** patterns session 1 tracker + implementation notes ([#334](https://github.com/livefir/livetemplate/issues/334))


<a name="v0.8.17"></a>
## [v0.8.17] - 2026-04-10

### Bug Fixes

- parse individual form fields in multipart submissions ([#326](https://github.com/livefir/livetemplate/issues/326))

### Documentation

- update attribute-reduction proposal with Phase 2 completion status ([#324](https://github.com/livefir/livetemplate/issues/324))


<a name="v0.8.16"></a>
## [v0.8.16] - 2026-04-04

### Documentation

- mark Phase 2E complete in attribute-reduction proposal

### Features

- Tier 1 file uploads — HTTP multipart with progress tracking


<a name="v0.8.15"></a>
## [v0.8.15] - 2026-04-04

### Bug Fixes

- unreserve action field, update tests to use lvt-action ([#321](https://github.com/livefir/livetemplate/issues/321))

### Documentation

- mark Phase 1B as complete in progress tracker ([#322](https://github.com/livefir/livetemplate/issues/322))
- update client-attributes reference for action-fix changes
- add lvt-form:action, lvt-nav: group, lvt-on:change to proposal
- mark Phase 1A complete in attribute-reduction proposal
- attribute reduction proposal — design + implementation plan ([#288](https://github.com/livefir/livetemplate/issues/288))


<a name="v0.8.14"></a>
## [v0.8.14] - 2026-04-02

### Features

- add AriaDisabled and FlashTag template helpers ([#318](https://github.com/livefir/livetemplate/issues/318))


<a name="v0.8.13"></a>
## [v0.8.13] - 2026-04-02

### Documentation

- add ephemeral-components guide ([#316](https://github.com/livefir/livetemplate/issues/316))


<a name="v0.8.12"></a>
## [v0.8.12] - 2026-04-01

### Documentation

- add attribute reduction proposal ([#288](https://github.com/livefir/livetemplate/issues/288)) ([#292](https://github.com/livefir/livetemplate/issues/292))

### Features

- selective state persistence via lvt:"persist" tag ([#308](https://github.com/livefir/livetemplate/issues/308))
- simplify error rendering with ErrorTag and AriaInvalid helpers ([#307](https://github.com/livefir/livetemplate/issues/307))


<a name="v0.8.11"></a>
## [v0.8.11] - 2026-04-01

### Features

- add WithEphemeralState() to opt out of state persistence ([#301](https://github.com/livefir/livetemplate/issues/301))


<a name="v0.8.10"></a>
## [v0.8.10] - 2026-03-31

### Bug Fixes

- skip HTTP POST persistence on action error + add multi-tab dedup logging ([#296](https://github.com/livefir/livetemplate/issues/296))
- only create .uploads directory when uploads are configured ([#287](https://github.com/livefir/livetemplate/issues/287))

### Documentation

- add Tier 1 file uploads proposal ([#271](https://github.com/livefir/livetemplate/issues/271)) ([#291](https://github.com/livefir/livetemplate/issues/291))
- state safety, current limitations, and progressive enhancement ([#284](https://github.com/livefir/livetemplate/issues/284))

### Features

- simplify state management and persistence defaults ([#298](https://github.com/livefir/livetemplate/issues/298))
- make state persistence opt-in via WithStatePersistence() ([#295](https://github.com/livefir/livetemplate/issues/295))
- per-connection state persists to session store for page refresh ([#290](https://github.com/livefir/livetemplate/issues/290))


<a name="v0.8.9"></a>
## [v0.8.9] - 2026-03-30

### Bug Fixes

- flash messages not rendered in WebSocket tree-diff mode ([#283](https://github.com/livefir/livetemplate/issues/283))
- pull latest from remote before starting release ([#281](https://github.com/livefir/livetemplate/issues/281))


<a name="v0.8.8"></a>
## [v0.8.8] - 2026-03-29

### Bug Fixes

- AsState panics if state contains dependency types ([#273](https://github.com/livefir/livetemplate/issues/273))

### Features

- per-connection state scoping (LiveView-style socket assigns) ([#275](https://github.com/livefir/livetemplate/issues/275))

### Breaking change


actions no longer auto-broadcast state or persist to SessionStore.

Key changes:
- Remove auto-broadcast and SessionStore persist from WebSocket action loop
- Add ctx.BroadcastAction() API for explicit cross-connection dispatch
- Restructure WS message loop to select-based event loop (readPump + DispatchChan)
- Add GroupActionMessage type and Redis PubSub support for cross-instance broadcast
- Handle BroadcastAction from both WebSocket and HTTP POST paths


<a name="v0.8.7"></a>
## [v0.8.7] - 2026-03-27

### Features

- formless standalone buttons — remove hidden form ([#263](https://github.com/livefir/livetemplate/issues/263))


<a name="v0.8.6"></a>
## [v0.8.6] - 2026-03-26

### Bug Fixes

- use current branch name in release script instead of hardcoded main/master
- preserve struct methods in template data map ([#254](https://github.com/livefir/livetemplate/issues/254))

### Features

- communicate Change() capability to client via initial render metadata ([#253](https://github.com/livefir/livetemplate/issues/253))


<a name="v0.8.5"></a>
## [v0.8.5] - 2026-03-25

### Breaking changes

> Added retroactively — this change shipped in v0.8.5 but was not recorded at the time.

- **`TreeNode` internal API changed in [#220](https://github.com/livefir/livetemplate/pull/220)** (commit `3fe784ca`). The `Dynamics` field type and the helper signatures changed when the internal map was replaced with a slice for ~20% speedup:
  - `Dynamics` field: `map[string]interface{}` → `[]interface{}`
  - `SetDynamic(position string, value interface{})` → `SetDynamic(index int, value interface{})`
  - `GetDynamic(position string)` → `GetDynamic(index int)`
  - New `AutoKey string` Go field replaces the previous `"_k"` map key. The `"_k"` *wire-format* key is unchanged — only the in-memory Go field name moved.

  `TreeNode` lives in `internal/build` and is not part of the public `livetemplate` API surface; the breaking surface is therefore limited to **library forks and downstream modules that vendor or replace `internal/build`**, plus internal test fixtures. Application code that consumes `livetemplate` through its exported API is unaffected. The on-the-wire tree format (numeric string keys: `"0"`, `"1"`, ...) is unchanged; only the in-memory Go API moved.

### Bug Fixes

- session benchmarks fail with 'client too slow' ([#209](https://github.com/livefir/livetemplate/issues/209))
- track dynamic pubsub subscriptions for reconnect and wire into mount ([#213](https://github.com/livefir/livetemplate/issues/213))
- check X-Forwarded-Proto in WebSocket origin checker ([#190](https://github.com/livefir/livetemplate/issues/190))

### Code Refactoring

- move progressive complexity examples to examples repo ([#248](https://github.com/livefir/livetemplate/issues/248))
- deduplicate generateItemHash into shared keys package ([#208](https://github.com/livefir/livetemplate/issues/208))
- rewrite parse package with custom AST evaluator ([#199](https://github.com/livefir/livetemplate/issues/199))

### Documentation

- update perf docs with TreeNode pooling investigation results ([#228](https://github.com/livefir/livetemplate/issues/228))
- update performance docs and baseline for recent optimizations ([#227](https://github.com/livefir/livetemplate/issues/227))
- update performance docs and baseline for recent optimizations ([#217](https://github.com/livefir/livetemplate/issues/217))

### Features

- progressive complexity model for form handling ([#233](https://github.com/livefir/livetemplate/issues/233))
- enhance ValidationToMultiError with friendly names and new tags ([#218](https://github.com/livefir/livetemplate/issues/218))
- add WithTrustForwardedHeaders config option ([#211](https://github.com/livefir/livetemplate/issues/211))

### Performance Improvements

- system card benchmark and per-session memory optimization ([#235](https://github.com/livefir/livetemplate/issues/235))
- replace encoding/json with json-iterator in hot paths ([#229](https://github.com/livefir/livetemplate/issues/229))
- reduce allocations with shared statics, buffer pool, and reflection dedup ([#224](https://github.com/livefir/livetemplate/issues/224))
- replace TreeNode Dynamics map with slice for ~20% speedup ([#220](https://github.com/livefir/livetemplate/issues/220))
- reduce template parsing allocations by 50-57% per render ([#219](https://github.com/livefir/livetemplate/issues/219))
- optimize range diffing with pre-computed context ([#212](https://github.com/livefir/livetemplate/issues/212))
- switch fingerprint hash to FNV-1a; add stress tests ([#205](https://github.com/livefir/livetemplate/issues/205))


<a name="v0.8.4"></a>
## [v0.8.4] - 2026-03-14

### Bug Fixes

- unify divergent expression evaluation paths ([#176](https://github.com/livefir/livetemplate/issues/176)) ([#179](https://github.com/livefir/livetemplate/issues/179))
- use cookie-based flash messages instead of URL query params ([#136](https://github.com/livefir/livetemplate/issues/136))

### Documentation

- refresh benchmark baseline and remove stale references ([#185](https://github.com/livefir/livetemplate/issues/185))
- batch address 9 documentation follow-up issues ([#178](https://github.com/livefir/livetemplate/issues/178))
- update performance docs to reflect current codebase ([#175](https://github.com/livefir/livetemplate/issues/175))
- audit and reorganize proposals directory ([#173](https://github.com/livefir/livetemplate/issues/173))
- replace api-reference.md with Go library API reference ([#164](https://github.com/livefir/livetemplate/issues/164))
- rewrite uploads.md for Controller+State pattern ([#163](https://github.com/livefir/livetemplate/issues/163))
- fix broken links in CONFIGURATION.md and client-attributes.md ([#162](https://github.com/livefir/livetemplate/issues/162))
- fix session.md interface signatures and add missing features ([#161](https://github.com/livefir/livetemplate/issues/161))
- expand server-actions.md with pubsub package details ([#160](https://github.com/livefir/livetemplate/issues/160))
- fix controller-pattern.md phantom methods, add missing APIs ([#159](https://github.com/livefir/livetemplate/issues/159))
- fix authentication.md phantom methods and broken link ([#158](https://github.com/livefir/livetemplate/issues/158))
- update template-support-matrix.md with current codebase state ([#157](https://github.com/livefir/livetemplate/issues/157))
- fix spec inaccuracies found during implementation verification ([#156](https://github.com/livefir/livetemplate/issues/156))
- move lvt-specific guides to lvt repo ([#153](https://github.com/livefir/livetemplate/issues/153))
- improve new contributor walkthrough guide ([#152](https://github.com/livefir/livetemplate/issues/152))
- audit specs, design, performance, and CLAUDE.md (Batch 5) ([#149](https://github.com/livefir/livetemplate/issues/149))
- update configuration and reference docs (Batch 4) ([#147](https://github.com/livefir/livetemplate/issues/147))
- audit and fix guide documentation (Batch 3) ([#146](https://github.com/livefir/livetemplate/issues/146))
- regenerate core architecture docs (Batch 2) ([#145](https://github.com/livefir/livetemplate/issues/145))
- update component import paths in doc comments
- archive 22 completed planning artifacts (Batch 1) ([#144](https://github.com/livefir/livetemplate/issues/144))
- Add comprehensive documentation overhaul plan and update README index ([#138](https://github.com/livefir/livetemplate/issues/138))
- fix metric names to match prometheus.go output
- fix internal/observe imports and document TraceMiddleware removal ([#137](https://github.com/livefir/livetemplate/issues/137))

### Features

- integrate LVT_WS_BUFFER_SIZE into EnvConfig system ([#151](https://github.com/livefir/livetemplate/issues/151))
- support template variable declarations ($c := .) in parser ([#150](https://github.com/livefir/livetemplate/issues/150))
- support template variable declarations ($c := .) in parser


<a name="v0.8.3"></a>
## [v0.8.3] - 2026-02-27

### Bug Fixes

- skip npm tests in pre-commit when client/ directory is absent
- cache HTTP templates per session to enable diff optimization ([#134](https://github.com/livefir/livetemplate/issues/134))
- add component attribute to all remaining slog calls ([#132](https://github.com/livefir/livetemplate/issues/132))
- slog cleanup — error handling, formatting, and component attributes ([#130](https://github.com/livefir/livetemplate/issues/130))
- enable burst mutation fuzz tests and fix KeyStability invariant ([#118](https://github.com/livefir/livetemplate/issues/118))
- handle complex insertion patterns in range differential operations ([#113](https://github.com/livefir/livetemplate/issues/113))

### Code Refactoring

- migrate log.Printf to structured slog logging ([#100](https://github.com/livefir/livetemplate/issues/100)) ([#123](https://github.com/livefir/livetemplate/issues/123))

### Documentation

- document auto-key behavioral change in release notes ([#121](https://github.com/livefir/livetemplate/issues/121))
- document fingerprint-based diff architecture ([#120](https://github.com/livefir/livetemplate/issues/120))


<a name="v0.8.2"></a>
## [v0.8.2] - 2026-02-02

### Features

- comprehensive fuzz testing framework with TypeScript oracle ([#110](https://github.com/livefir/livetemplate/issues/110))


<a name="v0.8.1"></a>
## [v0.8.1] - 2026-01-26

### Bug Fixes

- skip Redis tests gracefully when Docker is unavailable ([#109](https://github.com/livefir/livetemplate/issues/109))
- address Copilot review comments on API accuracy
- correct API references and range operation format in walkthrough

### Features

- auto-generated keys for range items without explicit key attribute ([#108](https://github.com/livefir/livetemplate/issues/108))
- progressive enhancement support for non-JS form submissions ([#102](https://github.com/livefir/livetemplate/issues/102))


<a name="v0.8.0"></a>
## [v0.8.0] - 2026-01-18


<a name="v0.7.12"></a>
## [v0.7.12] - 2026-01-10

### Bug Fixes

- preserve statics for conditional blocks in tree updates ([#84](https://github.com/livefir/livetemplate/issues/84))


<a name="v0.7.11"></a>
## [v0.7.11] - 2026-01-06

### Bug Fixes

- recognize append/prepend patterns to prevent statics resend on load_more ([#83](https://github.com/livefir/livetemplate/issues/83))


<a name="v0.7.10"></a>
## [v0.7.10] - 2026-01-04

### Bug Fixes

- handle range→else transitions in top-level range handling


<a name="v0.7.9"></a>
## [v0.7.9] - 2026-01-03

### Bug Fixes

- invalidate registry when conditional becomes empty ([#81](https://github.com/livefir/livetemplate/issues/81))


<a name="v0.7.8"></a>
## [v0.7.8] - 2025-12-27

### Bug Fixes

- **diff:** detect tree node changes when statics differ
- **mount:** enable flash messages on HTTP redirects with query params


<a name="v0.7.7"></a>
## [v0.7.7] - 2025-12-26

### Features

- add per-connection flash messages ([#79](https://github.com/livefir/livetemplate/issues/79))


<a name="v0.7.6"></a>
## [v0.7.6] - 2025-12-25

### Features

- add query parameter support for Mount and action handlers ([#78](https://github.com/livefir/livetemplate/issues/78))


<a name="v0.7.5"></a>
## [v0.7.5] - 2025-12-24

### Bug Fixes

- handle non-TreeNode to TreeNode transitions in range updates ([#77](https://github.com/livefir/livetemplate/issues/77))
- handle non-TreeNode to TreeNode transitions in range updates


<a name="v0.7.4"></a>
## [v0.7.4] - 2025-12-23

### Bug Fixes

- ensure Range.Statics populated for empty→items transitions ([#76](https://github.com/livefir/livetemplate/issues/76))


<a name="v0.7.3"></a>
## [v0.7.3] - 2025-12-22

### Bug Fixes

- support heterogeneous range items with per-item statics ([#75](https://github.com/livefir/livetemplate/issues/75))


<a name="v0.7.2"></a>
## [v0.7.2] - 2025-12-20

### Bug Fixes

- add type guard in SetDynamic to prevent raw structs in tree dynamics ([#74](https://github.com/livefir/livetemplate/issues/74))

### Features

- action.go updates for livepage ([#73](https://github.com/livefir/livetemplate/issues/73))


<a name="v0.7.1"></a>
## [v0.7.1] - 2025-12-14

### Bug Fixes

- mark range statics path in registry for proper caching ([#72](https://github.com/livefir/livetemplate/issues/72))


<a name="v0.7.0"></a>
## [v0.7.0] - 2025-12-10

### Documentation

- update all documentation for Controller+State API (v0.7.0) ([#70](https://github.com/livefir/livetemplate/issues/70))

### Features

- add component template registration support ([#71](https://github.com/livefir/livetemplate/issues/71))


<a name="v0.6.0"></a>
## [v0.6.0] - 2025-12-04


<a name="v0.5.2"></a>
## [v0.5.2] - 2025-12-03

### Documentation

- update client-attributes reference with reactive attributes and more ([#65](https://github.com/livefir/livetemplate/issues/65))
- add reactive attributes proposal ([#64](https://github.com/livefir/livetemplate/issues/64))

### Features

- store pattern redesign with automatic method dispatch ([#66](https://github.com/livefir/livetemplate/issues/66))


<a name="v0.5.1"></a>
## [v0.5.1] - 2025-11-30

### Documentation

- add authentication and session reference documentation ([#63](https://github.com/livefir/livetemplate/issues/63))


<a name="v0.5.0"></a>
## [v0.5.0] - 2025-11-30

### Documentation

- update documentation for Session API ([#62](https://github.com/livefir/livetemplate/issues/62))
- improve README structure and narrative flow ([#59](https://github.com/livefir/livetemplate/issues/59))

### Features

- add Session API for server-initiated actions ([#61](https://github.com/livefir/livetemplate/issues/61))
- add HTTP methods to ActionContext for authentication (v0.5) ([#60](https://github.com/livefir/livetemplate/issues/60))
- add coverage targets to Makefile ([#57](https://github.com/livefir/livetemplate/issues/57))


<a name="v0.4.2-debug.2"></a>
## [v0.4.2-debug.2] - 2025-11-22

### Bug Fixes

- add log package import for debug logging

### Documentation

- update investigation with breakthrough findings from timing instrumentation


<a name="v0.4.2-debug.1"></a>
## [v0.4.2-debug.1] - 2025-11-22


<a name="v0.4.1"></a>
## [v0.4.1] - 2025-11-22

### Bug Fixes

- use async WebSocket Send() instead of blocking WriteMessage() ([#56](https://github.com/livefir/livetemplate/issues/56))


<a name="v0.4.0"></a>
## [v0.4.0] - 2025-11-22

### Code Refactoring

- **registry:** achieve Grade A code quality for async WebSocket ([#55](https://github.com/livefir/livetemplate/issues/55))


<a name="v0.3.2"></a>
## [v0.3.2] - 2025-11-20

### Bug Fixes

- convert validation error field names to lowercase


<a name="v0.3.1"></a>
## [v0.3.1] - 2025-11-19

### Bug Fixes

- send live tree update after upload completion ([#54](https://github.com/livefir/livetemplate/issues/54))
- send live tree update after upload completion ([#53](https://github.com/livefir/livetemplate/issues/53))

### Features

- Phoenix LiveView-inspired file upload system v0.3.0 ([#52](https://github.com/livefir/livetemplate/issues/52))


<a name="v0.3.0"></a>
## [v0.3.0] - 2025-11-12

### Bug Fixes

- use GOWORK=off in release script to avoid workspace issues
- address minor code review issues
- address code review feedback

### Code Refactoring

- make New() fail-fast on template parsing errors ([#51](https://github.com/livefir/livetemplate/issues/51))

### Documentation

- add optimization task list to performance bottlenecks
- add performance section to README
- add performance characteristics analysis
- add comprehensive benchmarking guide
- document performance bottlenecks from profiling
- add design and implementation plan

### Performance Improvements

- address code review recommendations
- establish performance baseline
- add end-to-end user journey benchmarks
- add end-to-end template benchmarks
- add Phase 4 (Render) and Phase 5 (Send) benchmarks
- add Phase 3 (Diff) benchmarks
- add Phase 2 (Build) benchmarks
- add Phase 1 (Parse) benchmarks


<a name="v0.2.1"></a>
## [v0.2.1] - 2025-11-11

### Bug Fixes

- allow template discovery in internal directories for multi kit support
- template auto-discovery for go run and lvt serve ([#49](https://github.com/livefir/livetemplate/issues/49))
- improve template auto-discovery robustness ([#47](https://github.com/livefir/livetemplate/issues/47))

### Documentation

- remove version-specific references from contributor walkthrough
- create comprehensive contributor walkthrough for 5-phase architecture
- simplify README to focus on core value proposition ([#48](https://github.com/livefir/livetemplate/issues/48))


<a name="v0.2.0"></a>
## [v0.2.0] - 2025-11-09

### Code Refactoring

- improve key generation and fingerprinting robustness
- complete Phase 2 - move 4 functions to internal packages ([#44](https://github.com/livefir/livetemplate/issues/44))
- align template.go with 5-phase architecture ([#43](https://github.com/livefir/livetemplate/issues/43))
- reduce public API surface area from 11 to 7 files ([#46](https://github.com/livefir/livetemplate/issues/46))
- **conditional:** eliminate duplication and improve error handling ([#40](https://github.com/livefir/livetemplate/issues/40))
- **context:** achieve Grade A code quality ([#31](https://github.com/livefir/livetemplate/issues/31))
- **field:** achieve Grade A code quality ([#36](https://github.com/livefir/livetemplate/issues/36))
- **fingerprint:** fix circular detection and improve robustness
- **helpers:** achieve Grade A code quality ([#35](https://github.com/livefir/livetemplate/issues/35))
- **parse:** achieve Grade A code quality ([#38](https://github.com/livefir/livetemplate/issues/38))
- **parse:** achieve Grade A code quality ([#41](https://github.com/livefir/livetemplate/issues/41))
- **prepare:** achieve Grade A code quality ([#34](https://github.com/livefir/livetemplate/issues/34))
- **range:** achieve Grade A code quality ([#37](https://github.com/livefir/livetemplate/issues/37))
- **range_ops:** achieve Grade A code quality ([#33](https://github.com/livefir/livetemplate/issues/33))
- **render:** achieve Grade A code quality ([#42](https://github.com/livefir/livetemplate/issues/42))
- **render:** performance, security, and quality improvements ([#27](https://github.com/livefir/livetemplate/issues/27))
- **template:** achieve Grade A- code quality with 5-phase architecture ([#45](https://github.com/livefir/livetemplate/issues/45))
- **tree_compare:** achieve Grade A code quality ([#32](https://github.com/livefir/livetemplate/issues/32))
- **types:** achieve Grade A quality with comprehensive tests and documentation
- **var_context:** achieve Grade A code quality ([#39](https://github.com/livefir/livetemplate/issues/39))
- **wrapper:** improve security, correctness, and robustness - Grade A ([#29](https://github.com/livefir/livetemplate/issues/29))


<a name="v0.1.3"></a>
## [v0.1.3] - 2025-11-07


<a name="ls"></a>
## [ls] - 2025-11-07

### Bug Fixes

- update release script for Go-only releases
- use absolute paths for replace directives in cross-repo tests
- resolve race conditions in RedisBroadcaster

### Code Refactoring

- API reduction for v0.2.0 - reduce public API surface area ([#23](https://github.com/livefir/livetemplate/issues/23))

### Documentation

- update RELEASE.md for Go-only releases

### Features

- Code review backlog implementation - Issues [#12](https://github.com/livefir/livetemplate/issues/12)-52 ([#24](https://github.com/livefir/livetemplate/issues/24))
- add comprehensive unit tests for internal packages ([#22](https://github.com/livefir/livetemplate/issues/22))

### BREAKING CHANGE


SessionStore methods now require context.Context parameter

This change adds proper context propagation throughout the session store
layer, enabling timeout control, cancellation, and tracing for all Redis
and session operations.

Changes to SessionStore interface:
- Get(ctx context.Context, groupID string) Stores
- Set(ctx context.Context, groupID string, stores Stores)
- Delete(ctx context.Context, groupID string)
- List(ctx context.Context) []string

Implementation updates:

MemorySessionStore:
- Accepts context parameter for interface compliance
- Operations are in-memory so context not used internally

RedisSessionStore:
- Uses provided context for all Redis operations
- getWithRetry and execPipelineWithRetry now respect context
- Context-aware sleep during retry backoff
- Checks for context cancellation before each retry attempt

Benefits:
- Redis operations can be cancelled mid-flight
- Timeouts are properly respected across retry logic
- Trace IDs and request metadata can be propagated
- Better observability in distributed systems
- Prevents resource leaks from hung operations

Migration guide:
- All SessionStore method calls must now pass context
- Use r.Context() in HTTP handlers for request-scoped context
- Use context.Background() for background operations
- Consider using context.WithTimeout() for bounded operations

### Breaking Change


No - added field to struct, backward compatible.

Note: Only one pre-existing test failure (TestTemplateGenerateTreeWithFuncMap)

🤖 Generated with [Claude Code](https://claude.com/claude-code)


<a name="v0.1.2"></a>
## [v0.1.2] - 2025-11-03

### Bug Fixes

- exclude extracted components from test workflow

### Features

- add cross-repository testing and local development workflows


<a name="v0.1.1"></a>
## [v0.1.1] - 2025-11-03


<a name="v0.1.0"></a>
## v0.1.0 - 2025-11-03

### Bug Fixes

- improve binary build and archive naming in release script
- increase test timeout in release script from 30s to 120s
- remove t.Parallel() from e2e tests to prevent timeout deadlocks
- resolve flaky TestConnectionLimits_ConcurrentAccess test
- add LVT_DEV_MODE to todos e2e test and update hardcoded client paths
- set LVT_DEV_MODE=true in test server startup
- correct observability API usage in example
- prevent accidental .golangci.yml restoration
- resolve all golangci-lint issues and enhance CI validation
- **lvt:** prevent auth tests from generating files in commands/internal ([#19](https://github.com/livefir/livetemplate/issues/19))
- **lvt:** move auth command under lvt gen subcommands ([#17](https://github.com/livefir/livetemplate/issues/17))

### Code Refactoring

- Phase 4 - Extract large functions into internal/diff package
- move remaining build functions to internal/build (Phase 3.2)
- move fingerprinting functions to internal/build
- integrate internal/parse package and remove tree_ast.go
- move tree types to internal/build package
- convert TDD tests to maintainable table-driven format

### Documentation

- Update documentation for repository restructuring
- Complete Milestone 2 - Horizontal Scaling Documentation & Implementation ([#20](https://github.com/livefir/livetemplate/issues/20))
- add first principles document and fix pre-commit hook ([#18](https://github.com/livefir/livetemplate/issues/18))
- update all docs to reflect v1.0 internal package architecture
- Phase 5 - Migration guide, observability example, and test fixtures
- mark refactoring as complete and ready to merge
- update REFACTORING_PROGRESS.md for Phase 3 completion
- update REFACTORING_PROGRESS.md for Phase 3.1 completion
- update REFACTORING_PROGRESS.md - Phase 2 complete
- add comprehensive observability guide
- comprehensive documentation audit and API accuracy fixes ([#4](https://github.com/livefir/livetemplate/issues/4))

### Features

- update release script to use GitHub CLI and publish npm package
- add testcontainers for Redis testing
- Add deployment stack generation (lvt gen stack) ([#21](https://github.com/livefir/livetemplate/issues/21))
- create internal/parse package for template parsing
- observability and architecture documentation
- add comprehensive TDD tests for all Go template actions
- implement comprehensive granular fragment support for all template actions
- implement granular range fragment system with CRUD operations
- **lvt:** add lvt gen auth command - Complete (Phases 1-6) ([#15](https://github.com/livefir/livetemplate/issues/15))


[Unreleased]: https://github.com/livefir/livetemplate/compare/v0.8.23...HEAD
[v0.8.23]: https://github.com/livefir/livetemplate/compare/v0.8.22...v0.8.23
[v0.8.22]: https://github.com/livefir/livetemplate/compare/v0.8.21...v0.8.22
[v0.8.21]: https://github.com/livefir/livetemplate/compare/v0.8.20...v0.8.21
[v0.8.20]: https://github.com/livefir/livetemplate/compare/v0.8.19...v0.8.20
[v0.8.19]: https://github.com/livefir/livetemplate/compare/v0.8.18...v0.8.19
[v0.8.18]: https://github.com/livefir/livetemplate/compare/v0.8.17...v0.8.18
[v0.8.17]: https://github.com/livefir/livetemplate/compare/v0.8.16...v0.8.17
[v0.8.16]: https://github.com/livefir/livetemplate/compare/v0.8.15...v0.8.16
[v0.8.15]: https://github.com/livefir/livetemplate/compare/v0.8.14...v0.8.15
[v0.8.14]: https://github.com/livefir/livetemplate/compare/v0.8.13...v0.8.14
[v0.8.13]: https://github.com/livefir/livetemplate/compare/v0.8.12...v0.8.13
[v0.8.12]: https://github.com/livefir/livetemplate/compare/v0.8.11...v0.8.12
[v0.8.11]: https://github.com/livefir/livetemplate/compare/v0.8.10...v0.8.11
[v0.8.10]: https://github.com/livefir/livetemplate/compare/v0.8.9...v0.8.10
[v0.8.9]: https://github.com/livefir/livetemplate/compare/v0.8.8...v0.8.9
[v0.8.8]: https://github.com/livefir/livetemplate/compare/v0.8.7...v0.8.8
[v0.8.7]: https://github.com/livefir/livetemplate/compare/v0.8.6...v0.8.7
[v0.8.6]: https://github.com/livefir/livetemplate/compare/v0.8.5...v0.8.6
[v0.8.5]: https://github.com/livefir/livetemplate/compare/v0.8.4...v0.8.5
[v0.8.4]: https://github.com/livefir/livetemplate/compare/v0.8.3...v0.8.4
[v0.8.3]: https://github.com/livefir/livetemplate/compare/v0.8.2...v0.8.3
[v0.8.2]: https://github.com/livefir/livetemplate/compare/v0.8.1...v0.8.2
[v0.8.1]: https://github.com/livefir/livetemplate/compare/v0.8.0...v0.8.1
[v0.8.0]: https://github.com/livefir/livetemplate/compare/v0.7.12...v0.8.0
[v0.7.12]: https://github.com/livefir/livetemplate/compare/v0.7.11...v0.7.12
[v0.7.11]: https://github.com/livefir/livetemplate/compare/v0.7.10...v0.7.11
[v0.7.10]: https://github.com/livefir/livetemplate/compare/v0.7.9...v0.7.10
[v0.7.9]: https://github.com/livefir/livetemplate/compare/v0.7.8...v0.7.9
[v0.7.8]: https://github.com/livefir/livetemplate/compare/v0.7.7...v0.7.8
[v0.7.7]: https://github.com/livefir/livetemplate/compare/v0.7.6...v0.7.7
[v0.7.6]: https://github.com/livefir/livetemplate/compare/v0.7.5...v0.7.6
[v0.7.5]: https://github.com/livefir/livetemplate/compare/v0.7.4...v0.7.5
[v0.7.4]: https://github.com/livefir/livetemplate/compare/v0.7.3...v0.7.4
[v0.7.3]: https://github.com/livefir/livetemplate/compare/v0.7.2...v0.7.3
[v0.7.2]: https://github.com/livefir/livetemplate/compare/v0.7.1...v0.7.2
[v0.7.1]: https://github.com/livefir/livetemplate/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/livefir/livetemplate/compare/v0.6.0...v0.7.0
[v0.6.0]: https://github.com/livefir/livetemplate/compare/v0.5.2...v0.6.0
[v0.5.2]: https://github.com/livefir/livetemplate/compare/v0.5.1...v0.5.2
[v0.5.1]: https://github.com/livefir/livetemplate/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/livefir/livetemplate/compare/v0.4.2-debug.2...v0.5.0
[v0.4.2-debug.2]: https://github.com/livefir/livetemplate/compare/v0.4.2-debug.1...v0.4.2-debug.2
[v0.4.2-debug.1]: https://github.com/livefir/livetemplate/compare/v0.4.1...v0.4.2-debug.1
[v0.4.1]: https://github.com/livefir/livetemplate/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/livefir/livetemplate/compare/v0.3.2...v0.4.0
[v0.3.2]: https://github.com/livefir/livetemplate/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/livefir/livetemplate/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/livefir/livetemplate/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/livefir/livetemplate/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/livefir/livetemplate/compare/v0.1.3...v0.2.0
[v0.1.3]: https://github.com/livefir/livetemplate/compare/ls...v0.1.3
[ls]: https://github.com/livefir/livetemplate/compare/v0.1.2...ls
[v0.1.2]: https://github.com/livefir/livetemplate/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/livefir/livetemplate/compare/v0.1.0...v0.1.1
