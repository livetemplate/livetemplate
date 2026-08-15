# Plan: Redesign lvt attributes to be naturally extensible

**Issue:** livetemplate/livetemplate#473

---

## At a glance

*Read this section alone to review the plan's shape. Everything below it is the supporting detail, and every row here links to where the argument lives.*

Today the client hardcodes ~20 `handleXDirectives()` calls, so nobody can add an `lvt-*` attribute without forking the library — and 10 handlers that belong to exactly one app (prereview) ship to every LiveTemplate user. **This plan replaces the hardcoded pipeline with a public handler registry, and moves the single-app handlers out of the library to the app that owns them.**

### High-level decisions

| # | Decision | Why | Detail |
|---|----------|-----|--------|
| H1 | Replace the hardcoded call sequence with a **registry of handler objects**, scanned per render | One interface instead of 20 ad-hoc conventions; makes external registration possible at all | § Design 2 |
| H2 | Handlers are **per-attribute singletons that scan**, not per-element instances like LiveView hooks | Matches existing architecture; no per-row allocation on large lists. Per-element hooks stay deferred | § Reference design, Phase 6 |
| H3 | **A usage census decides core vs leaving**, not judgement | "Universally useful" is unfalsifiable; consumer counts aren't | § Core vs extras |
| H4 | **Single-app handlers leave the repo entirely** — prereview owns its 10. There is no first-party "extras" tier | A blessed extras bucket has no membership rule and becomes the next kitchen sink | § Server-side 3, § OQ5 |
| H5 | **Registration is static + late-registration catch-up**, never instance-scoped | `autoInit()` connects during the core bundle's own evaluation — no instance exists for a second script to call | § Design 2 (Bootstrap timing) |
| H6 | The handler interface is **public API from day one**, semver'd | Publishing `registerAttribute` freezes the contract; there is no public-but-not-really grace period | § Server-side 3 (req. 2) |
| H7 | **Diagnostics ship with the registry, not after it** | The registry's failure mode is *silence*; shipping the API first and the warnings later is backwards | § Server-side 1 |
| H8 | The server **still never interprets attributes** | The census reads attribute *names* for a dev warning; nothing branches on it | § Non-goals |
| H9 | **Custom morphdom hooks: deferred, not foreclosed.** Every `AttributeHandler` extension point is an optional member | Hooks are per-element *predicates* (`return false` skips a subtree) — needs a conflict rule and hot-path budget. `lvt-ignore` already covers the common case | § Scope boundaries |
| H10 | **No custom event delegators** — routing stays single-implementation. Instead a handler may declare `delegatedEvents` | `lvt-el:` already accepts arbitrary event names but `lvt-on:` is a closed 17-entry list, so a custom element's event can't reach the server. That asymmetry is the actual gap | § Scope boundaries, Phase 3 |
| H11 | **Authors declare an attribute name, not a CSS selector** | Hand-escaped `'[lvt-x\\:copy]'` is silent-failure-prone and was written twice per handler; the framework can own escaping, scanning, wiring idempotence and sweep | § Writing your own attribute |

### Low-level decisions

| # | Decision | Detail |
|---|----------|--------|
| L1 | New `meta.attributes []string` **beside** `capabilities`, not a struct array replacing it | § Server-side 1 |
| L2 | Census computed at parse (sibling of `extractWiredActionNames`), set at **both** initial-render sites — WS and HTTP | § Server-side 1 |
| L3 | Keep the three run categories: `always` / `fire-on-change` / `wire-idempotent`. **Derived** for declarative handlers from which callbacks are supplied; required for low-level ones | § Choosing a category |
| L4 | **Two-layer registration**: declare `attribute: "lvt-x:copy"` + per-element callbacks (most authors), or `selectors` + `setup()` (cross-element state). The declarative layer also *resolves* the old "`selectors` has no consumer" problem — the framework owns the scan, so the declared name is authoritative for both warnings by construction | § Design 2, § Writing your own attribute |
| L5 | Bracket expansion generalized to **any** `lvt-` prefix, so custom attributes get syntax parity | § Server-side 2 |
| L6 | Built-ins register first; duplicate-selector warning is **unconditional**; third parties advised off `lvt-fx:`/`lvt-el:`/`lvt-form:` | § OQ3 |
| L7 | Census corrections: **toast stays core** (lvt + 11 docs examples use it), **`iframe-autoheight` is deleted** (zero consumers), **shadow-root hydration stays core** (platform bridge) | § Core vs extras |
| L8 | Morphdom hooks, event delegation, and reactive listeners stay **outside** the registry | § Scope boundaries |
| L9 | Relocation gate: must cut **≥ 25 KB minified** from the core bundle, or don't do it | Phase 4 |
| L10 | Tier-2 community bundles documented with SRI pinning + an explicit trust note | § Server-side 3 (req. 6) |
| L11 | `ElementContext.value` / `.send` are **live accessors, not fields** — capturing `ctx` in a listener stays correct across a re-render and a reconnect | § Design 2 |

### What changes for users

**If your app uses only core attributes — nothing changes, and your bundle gets smaller.** No template edits, no attribute renames, no new tags.

| Change | Kind | Who it affects |
|--------|------|----------------|
| `LiveTemplateClient.registerAttribute(handler)` | **New API** | anyone wanting a custom attribute — the headline feature (§ Writing your own attribute) |
| `AttributeHandler` / `SetupContext` / `SendFn` / `ElementContext` exported as public TS types | **New API** | custom-attribute authors |
| Declarative registration: `attribute` + `onElementAdded` / `onElement` / `onElementRemoved` | **New API** | custom-attribute authors — no CSS-selector escaping, no manual scan |
| Any `lvt-`-prefixed attribute may use bracket syntax (`lvt-x:foo:on:[a,b]:pending`) | **New capability** | custom-attribute authors |
| `delegatedEvents` — `lvt-on:` can route a custom element's own DOM event to a server action | **New capability** | anyone using custom elements / web components |
| `meta.attributes` on the initial-render response | **Additive wire field** | nobody directly — `omitempty`, ignored by older clients |
| Console warning: attribute in template with no registered handler | **New diagnostic** | anyone who typos an attribute or forgets a bundle |
| Console warning: two handlers claim the same selector | **New diagnostic** | apps registering custom handlers |
| **10 attributes leave the library** — `area-select`, `region-select`, `text-select`, `viewport-report`, `proxy-bridge`, `preview-bridge`, `url-hash`, `resize`, `spy`, `auto-click` | **Breaking** | prereview (the only app using them). Any other app would now have to supply its own handler — the new warning tells it so on first render |
| `lvt-fx:iframe-autoheight` removed | **Breaking in principle** | nobody — zero consumers found |
| Attribute syntax, semantics, and server-side treatment | **Unchanged** | everyone |

**Not user-visible but worth knowing:** no new WebSocket message types, no server-side attribute interpretation, and no second version pin to hand-maintain (§ Non-goals).

### How to read the rest

The body is long because a dozen review comments each added the evidence behind a decision. Three passages are pure **decision history** — how an earlier draft was wrong and what changed it — and are tagged *[decision history — skippable]* inline. Skip them on a first read; they exist so a decision isn't silently re-litigated later.

### Shape of the work

Six phases in three milestones — **M1 Registry** (1 registry + core migration, 2 census + diagnostics, 3 public API + docs), **M2 Extraction** (4 relocate to prereview, 5 downstream release), **M3 Hooks** (6, deferred). Full blocks with audits and acceptance criteria: § Implementation phases.

**The largest single risk** is Phase 4: prereview has no JavaScript toolchain today, so owning its handlers means acquiring one (~a day). That cost is also the plan's best validation — if prereview can't adopt its handlers through the public API, the API isn't finished. A documented fallback (option B) exists if it runs long.

---

## Problem statement

The client-side attribute system has an **opaque, hardcoded pipeline** that makes it impossible for external contributors to add custom `lvt-*` attributes without modifying the core client library. This has two consequences:

1. **Users can't build their own attributes.** There is no way to add one without modifying the core `livetemplate-client.ts` — so an app that wants `lvt-x:copy-to-clipboard` has to fork the client, or give up and write bespoke JS outside the framework. This is the headline goal of the issue, and § Writing your own attribute shows exactly what it looks like once the registry exists.
2. **The core library becomes a kitchen sink.** Every new attribute — including domain-specific ones that only make sense for a particular application (e.g., `handleAreaSelectDirectives`, `handlePreviewBridgeDirectives`, `handleRegionSelectDirectives` are prereview-specific) — gets baked into the core library because there's no mechanism for downstream consumers to register their own. This bloats the client bundle for every user, even those who never use these attributes.

   **The magnitude, measured (client v0.20.0):** `dom/directives.ts` is 3,429 lines. Everything from `handleAreaSelectDirectives` to end-of-file — area-select, region-select, text-select, viewport-report, proxy-bridge, preview-bridge, url-hash and their geometry/hit-testing helpers — is single-app domain code, ~2,400 lines, ~70% of the file. It is statically imported and unconditionally called by the post-render pipeline, and the browser build is a single `esbuild --bundle --format=iife` entry (`package.json` `build:browser`), so there is no tree-shaking escape hatch: all of it ships in `dist/livetemplate-client.browser.js`, which measures 141 KB minified at client 0.20.0. That single artifact is what every LiveTemplate app downloads — `client_assets.go` pins one CDN URL for it (currently `ClientVersion = "0.18.2"`; the pin trails the client's own version by design, see § Server-side changes 3).

**Where the pain is:** `livetemplate-client.ts:2190-2306` — a hardcoded sequence of ~20 `handleXDirectives(element)` calls, each independently scanning the DOM via `querySelectorAll`, each with its own ad-hoc setup/teardown convention, each requiring a new import + a new call site. Adding a new attribute means:

1. Writing a new `dom/*.ts` module with a `handleX` + `teardownX` function pair
2. Importing both into `livetemplate-client.ts`
3. Adding the call to the right position in the post-render pipeline
4. Knowing which lifecycle category it falls into (fire-on-change vs wire-idempotent)
5. Optionally wiring the `send()` callback if it sends server actions

There is **no shared interface**, no registry, no plugin API. The contract between the server and client is implicit — the server passes attributes through as opaque HTML in the Statics array, and the client hardcodes which attribute selectors to look for after each render.

**The extensibility goal is twofold:** (a) let downstream consumers register app-specific attribute handlers without forking the library, and (b) let the domain-specific handlers already baked into the core be moved *out* of it — to the apps that own them — shrinking the core to the universally-useful primitives.

(b) is the harder half and the one that shapes the design. A plugin API alone doesn't fix the kitchen sink — it only stops it growing. The core stays bloated until the existing domain handlers actually leave, which means the plan needs three things a pure plugin API doesn't: a **criterion** for what leaves (§ Core vs extras), a **registration mechanism that works from a second bundle** (§ 2, Bootstrap timing), and an answer to **where the departing handlers go** (§ Server-side changes 3 — to their owning app, not to a blessed first-party "extras" tier). Which handlers are core and which are extras is not a judgement call in this plan — it's decided by a usage census, below.

**The server side is mostly fine.** The server already treats `lvt-*` attributes as opaque static HTML. It doesn't parse them (except for `lvt-on:` action extraction in `topic_wired_actions.go` and bracket expansion in `internal/render/bracket_expand.go`). The redesign is primarily a client-side architecture change with minimal server-side additions.

## Design

### Core vs extras — the extraction criterion

"Universally-useful primitive" is an assertion unless it's grounded in something checkable. The rule this plan uses:

> **A handler stays in core if it is referenced by code the framework itself ships (lvt's generators, kits, and components) or by more than one downstream app. A handler referenced only by the single app that motivated it leaves — to that app (§ Server-side changes 3).**

Applying it — census of attribute references across the workspace's checkouts as of client v0.20.0 (2026-07-25), counting `*.go` / `*.tmpl` / `*.html` sources only, excluding vendored `dist/` bundles:

| Attribute | Handler | Referenced by | Destination |
|-----------|---------|---------------|-------------|
| `lvt-fx:scroll` | `handleScrollDirectives` | docs=7, prereview=21 | **core** |
| `lvt-fx:highlight` | `handleHighlightDirectives` | docs=9, prereview=1 | **core** |
| `lvt-fx:animate` | `handleAnimateDirectives` | docs=12, prereview=8 | **core** |
| `lvt-toast-stack` | `handleToastDirectives` | docs=11, **lvt=2** (`components/toast`) | **core** |
| `lvt-scroll-away` | `setupScrollAway` | docs=3 | **core** |
| `lvt-scroll-sentinel` | (scroll-away module) | docs=7, **lvt=13** (pagination + resource templates) | **core** |
| `lvt-fx:*` event triggers | `setupFxDOMEventTriggers` | framework primitive | **core** |
| `lvt-el:*` delegation | `setupDOMEventTriggerDelegation` | framework primitive | **core** |
| file inputs | `uploadHandler.initializeFileInputs` | framework primitive | **core** |
| `lvt-fx:area-select` | `handleAreaSelectDirectives` | prereview=7 | extras |
| `lvt-fx:region-select` | `handleRegionSelectDirectives` | prereview=5 | extras |
| `lvt-fx:text-select` | `handleTextSelectDirectives` | prereview=20 | extras |
| `lvt-fx:viewport-report` | `handleViewportReportDirectives` | prereview=28 | extras |
| `lvt-fx:proxy-bridge` | `handleProxyBridgeDirectives` | prereview=4 | extras |
| `lvt-fx:preview-bridge` | `handlePreviewBridgeDirectives` | prereview=5 | extras |
| `lvt-url-hash` | `handleURLHashDirective` | prereview=28 | extras |
| `lvt-fx:resize` | `handleResizeDirectives` | prereview=27 | extras |
| `lvt-spy` | `setupSpy` | prereview=10 | extras |
| `lvt-fx:auto-click` | `handleAutoClickDirectives` | prereview=3 | extras |
| `lvt-fx:iframe-autoheight` | `handleIframeAutoHeightDirectives` | **none** | audit → delete |
| `<template shadowrootmode>` | `handleShadowRootHydration` | prereview=2 | see note |

**Three corrections this census forces on the earlier draft of this plan:**

1. **Toast is core, not extras.** An earlier draft listed it for extraction. `lvt/components/toast` and 11 docs examples depend on it — extracting it would break the framework's own generated code.
2. **`iframe-autoheight` has no consumers at all.** Its own source comment says preview-bridge is its opaque-origin successor. Phase 4 audits it and **deletes** rather than extracts — the cheapest possible win, and the single cleanest piece of evidence for the reviewer's point that the core accretes attributes nothing removes.
3. **Shadow-root hydration is a platform bridge, not a domain handler.** It activates Declarative Shadow DOM that morphdom inserted via DOM APIs — a gap in the platform's own hydration, not an app feature. It's ~30 lines and applies to any app emitting DSD. Keep in core; the prereview-only reference count reflects who happens to emit DSD today, not who could.

**tinkerdown is a vocabulary tracker, not a consumer.** Its hits are in `vocabulary.go` (a sanitizer allowlist of attributes authored markdown may pass through) plus its attribute-reference docs and tests. It uses none of them in its own templates — but it *does* enumerate them, so extraction requires updating that allowlist in lockstep. It's the second entry in the blast radius (see § Non-goals).

### Three layers

#### 1. Event spec — formalize the lifecycle contract

The client already emits lifecycle events (`pending`, `success`, `error`, `done`) and content-update signals. These are the **contract** custom attributes build on. Today they're implicit — scattered across `FormLifecycleManager`, morphdom callbacks, and individual directive functions.

**Formalize into an explicit event spec:**

```typescript
interface AttributeLifecycle {
  // Fired after every morphdom pass (content changed)
  contentUpdated(root: Element): void;

  // Fired on action lifecycle state transitions
  actionPending(root: Element, action: string): void;
  actionSuccess(root: Element, action: string): void;
  actionError(root: Element, action: string, errors: Record<string, string>): void;
  actionDone(root: Element, action: string): void;

  // DOM event subscriptions are NOT part of this interface — a handler
  // declares them via `delegatedEvents` on AttributeHandler (§ 2).
}
```

This is the **stable contract** external attribute authors program against. It already exists informally; this step makes it explicit and documented.

Note it is a *documented event contract*, not a second interface to implement: handlers observe these events through the DOM (`lvt:pending` et al. are real events on the wrapper), which is why `AttributeLifecycle` isn't in the published-types list — `AttributeHandler` is the only type an author implements.

#### 2. Attribute handler registry — the plugin interface

Replace the hardcoded import/call-site pattern with a registry of attribute handlers:

```typescript
// Two variants. Both are `AttributeHandler`; they differ only in how elements
// are found and delivered. Declarative is what most authors should use.

interface BaseHandler {
  // Defaults to `attribute` for declarative handlers; required for low-level.
  name?: string;

  // Omit to derive: declaring onElement => "fire-on-change";
  // only onElementAdded/onElementRemoved => "wire-idempotent".
  // An explicit value always wins. See § Choosing a category.
  category?: "always" | "fire-on-change" | "wire-idempotent";

  // Whether this handler dispatches server actions. Default false.
  needsServerChannel?: boolean;

  // Extra DOM event types the delegator should listen for at the document
  // level, so a custom element's event can reach a server action via lvt-on:.
  // See § Scope boundaries, event delegation.
  delegatedEvents?: string[];

  // Reserved: per-element morphdom predicates (onBeforeElUpdated?, ...).
  // Deferred, not designed — the slot exists so adding them stays additive.
  // Every extension point is optional by construction, so adding one is a
  // minor release (H6: this interface is public and semver'd from day one).
}

// A. DECLARATIVE — declare an attribute NAME, not a selector. The framework
//    escapes it, scans for it, tracks which elements match, reads the value,
//    and sweeps elements that lost it. An author never writes '[lvt-x\:copy]'.
interface DeclarativeHandler extends BaseHandler {
  attribute: string;

  onElementAdded?(el: Element, ctx: ElementContext): void;   // once per element
  onElement?(el: Element, ctx: ElementContext): void;        // every run
  onElementRemoved?(el: Element): void;                      // detached, or lost the attr
}

// B. LOW-LEVEL — the escape hatch, used by built-ins with cross-element state.
//    See § Writing your own attribute, Example 3, for when this is warranted.
interface LowLevelHandler extends BaseHandler {
  selectors: string[];                    // e.g. '[lvt-fx\:scroll]'
  setup(ctx: SetupContext): void;
  teardown(root: Element): void;
}

type AttributeHandler = DeclarativeHandler | LowLevelHandler;

type SendFn = (message: { action: string; data: Record<string, unknown> }) => void;

// Both members are LIVE ACCESSORS, deliberately. Capturing ctx inside a
// listener is the normal thing to do, so reading it later must be correct:
//   - `value` re-reads the attribute, so a server re-render is picked up
//     rather than the callback holding the value it saw at wiring time;
//   - `send` resolves the current transport, so a captured ctx survives a
//     WebSocket reconnect instead of dispatching into a dead socket.
// Those are two of the three failure modes in § Writing your own attribute,
// closed by the shape of this type rather than by documentation.
interface ElementContext {
  readonly value: string;
  readonly send?: SendFn;        // present when needsServerChannel is true
  readonly wrapperRoot: Element;
}
```

The `setup` method receives a context object rather than bare positional args, because some handlers need both a scan root and a separate registry root (e.g., `setupFxDOMEventTriggers(scanRoot, wrapperRoot)`):

```typescript
interface SetupContext {
  scanRoot: Element;       // Element subtree to scan
  wrapperRoot: Element;    // Top-level wrapper (for event listener storage)
  send?: SendFn;           // Only provided when needsServerChannel is true
}
```

**Registration API — static, not per-instance:**

```typescript
// Built-in handlers registered at module load
LiveTemplateClient.registerAttribute(scrollHandler);
LiveTemplateClient.registerAttribute(highlightHandler);

// User-defined custom attribute (also how a tier-2/3 bundle registers).
// Declarative form — no selector, no escaping, no manual scan.
LiveTemplateClient.registerAttribute({
  attribute: "lvt-x:tooltip",        // lvt-x:, not lvt-fx: — see § Open questions 3
  onElementAdded(el, ctx) {
    // ctx.value is the attribute's current value, re-read on each access
  },
});
```

**Bootstrap timing — why registration is static, and why late registration must be a supported path.** The obvious API is an instance method, `client.registerAttribute(...)`. It doesn't work, and the reason is load-bearing for the whole extraction plan:

- `livetemplate-client.ts` calls `LiveTemplateClient.autoInit()` at **module top level** (bottom of file, guarded by `typeof window !== "undefined"`).
- `autoInit()` only waits for `DOMContentLoaded` when `document.readyState === "loading"`. Otherwise it calls `init()` immediately.
- The documented load pattern is `<script src="{{ lvtClientScriptURL }}" defer></script>`. Deferred scripts execute *after* parsing completes, when readyState is already `"interactive"` — so `init()` runs **synchronously during the core bundle's own evaluation**: it constructs the client, calls `connect()`, and only then assigns `window.liveTemplateClient`.

So under the documented `defer` pattern, by the time any second `<script>` evaluates the client already exists and has connected — and before that moment, no instance exists to call a method on. Other load styles differ, which makes the case stronger rather than weaker: a classic non-`defer` `<script>` in `<head>` *does* execute while readyState is `"loading"`, so `autoInit` installs a DOMContentLoaded listener and a later script would get a pre-init window; an `async` or dynamically-injected load lands nondeterministically on either side. **An API whose viability depends on which of three load styles the app chose is not an API.** Worse, a separate IIFE bundle can't share module scope with core either; it reaches core only through the `LiveTemplateClient` global, which requires core to have evaluated (and therefore auto-initialized) first.

Two consequences for the design:

1. **The registry is module-level, exposed via a static `LiveTemplateClient.registerAttribute()`.** Instances read from it rather than owning it. This is what lets a bundle that loads *after* core still register.
2. **Late registration runs a one-shot catch-up.** `registerAttribute()` called after a client exists immediately runs the handler against each live wrapper — `setup()` for a low-level handler, a scan + `onElementAdded` pass for a declarative one — instead of waiting for the next render. Cheap to implement — the post-render loop already calls exactly this — and it turns the load-order problem into a non-issue rather than a documented footgun. Without it, an extras attribute silently does nothing until the user happens to trigger a render.

Optional refinement (not required for correctness once (2) lands): make `autoInit` also defer when readyState is `"interactive"`, so a `defer`-ordered second bundle registers before the first connect and skips the catch-up scan. ESM consumers who `import` both entries get pre-init registration for free via import ordering; the IIFE/CDN path is the one that needs (2).

**The post-render loop becomes:**

```typescript
// Replace 20 hardcoded handleXDirectives calls with:
const ctx: SetupContext = {
  scanRoot: element,
  wrapperRoot: this.wrapperElement || element,
  // send is set per-handler below
};

// Reads the module-level registry live on every render — NOT a snapshot taken
// at construction, so a handler registered late (a second bundle, user code after
// connect) participates from its very next render.
for (const handler of attributeRegistry) {
  if (!shouldRun(handler, this.nodesAddedThisRender, this.directiveTouchedThisRender))
    continue;

  if (isDeclarative(handler)) {
    // Framework owns the scan: escape the name once at registration, match,
    // diff against the previous match set, then dispatch per element.
    dispatchDeclarative(handler, ctx, (msg) => this.send(msg));
  } else {
    handler.setup({
      ...ctx,
      send: handler.needsServerChannel ? (msg) => this.send(msg) : undefined,
    });
  }
}
```

**What the registry knows it handles.** Phase 2's two diagnostics — the duplicate-registration warning (§ Open questions 3) and the unregistered-attribute warning that diffs the server's census against the registry (§ Server-side changes 1) — both need an authoritative answer to "which attributes does this registry claim?".

For **declarative** handlers the answer is free and exact: `attribute` is a plain name, the framework owns the scan, and the census comparison is a string match. That is a second reason to prefer the declarative layer over the ergonomic one.

For **low-level** handlers `selectors` is the only source, and it is author-supplied prose the loop never reads — so a handler whose `selectors` disagree with what its `setup` actually queries will produce a wrong warning in either direction. Phase 1's audit decides how far to close that: either the loop consults `selectors` to dispatch (which also removes N redundant `querySelectorAll` passes), or low-level handlers are simply excluded from the census comparison and the plan says so. Nine built-ins is a small enough population to keep honest by review; a public API is not.

### Scope boundaries — what the registry covers and what it doesn't

The registry covers **post-render DOM scans** — the `handleXDirectives` / `setupX` / `teardownX` functions that run after morphdom completes. Several attribute-handling mechanisms are fundamentally different and stay **outside** the registry:

- **Morphdom hooks** (`lvt-ignore`, `lvt-ignore-attrs`, `data-lvt-force-update`, client-owned-state preservation for checkboxes/dialogs/focused inputs). These run *during* the morphdom pass in `onBeforeElUpdated` / `onBeforeNodeDiscarded`, not after it. They cannot be post-render handlers.

  **Should custom handlers get morphdom hooks? Yes in principle — deferred, but Phase 1 must not foreclose it.** The common case is already served declaratively: `lvt-ignore` covers "morphdom must not touch my subtree", which is what a third-party editor or map widget needs. The uncovered case is narrower — a stateful custom element that isn't a checkbox, dialog, or focused input (say a `<my-video>` whose `currentTime` must survive a morph). Real, but no consumer has asked yet.

  Deferred because these hooks are **predicates, not actions**, and that changes the contract: `onBeforeElUpdated` returning `false` short-circuits an entire subtree, so two registrants disagreeing needs a defined rule (any-false-wins, presumably), and a user callback that throws mid-patch leaves the DOM half-applied. They also run per element on the morph hot path, where the registry's cost model (one attribute-selector `querySelectorAll`) doesn't apply.

  **The cheap thing Phase 1 owes this:** `AttributeHandler` must be an open shape with optional members, so `onBeforeElUpdated?` can be added later as a minor. Under H6 the interface is public and semver'd from day one — sealing it now would make this a major-version event for no benefit.

- **Event delegation** (`lvt-on:*`, `lvt-mod:*`, `lvt-form:*` action routing, `lvt-key` filtering). Set up once at connect time as document-level listeners. Events are discovered during bubbling, not via DOM scans.

  **Should custom event delegators be allowed? No — but the real gap is narrower than that, and it should be closed.** A second delegator implementation would fork the routing protocol (action extraction, modifiers, `lvt-key`), which must stay single-implementation.

  What's actually missing is checkable: **`DELEGATED_EVENT_TYPES` is a closed 17-entry list** (`click`, `submit`, `change`, … `dragleave`), so `lvt-on:my-widget-change="Save"` on a custom element never fires — its event reaches no server action. Meanwhile `lvt-el:*:on:{event}` gates on `isDOMEventTrigger()`, which is a *denylist* (not a lifecycle event, not synthetic) — so **the same custom event already drives client-side effects but cannot reach the server.** That asymmetry is arbitrary, and it bites exactly the custom-element authors this plan is for.

  **Fix: a handler may declare `delegatedEvents: string[]`**, which the delegator adds to its document-level listener set at registration. No new routing, no second delegator — the existing pipeline just listens for more event types. Lands in Phase 3 with the rest of the public API surface.

- **Reactive attribute listeners** (`setupReactiveAttributeListeners`). Document-level lifecycle event listeners (`lvt:pending`, `lvt:success`, etc.) set up once at connect time (line 675). Not a per-render scan.

- **Pre-render hydration** (`hydrateRedactedTokens`, `uploadHandler.hydratePreviews`). These run *before* the directive sweep in the post-render pipeline and stay in-place — they're order-sensitive and serve the morphdom-to-directive bridge, not attribute behavior.

- **Change auto-wirer** (`changeAutoWirer.wireElements()`). Runs unconditionally after every render (needs to process evictions for removed elements). This is a special case that either becomes an `"always"` category handler or stays in-place.

#### 3. Migration — existing directives become built-in registrations

Every existing `handleXDirectives` / `teardownX` pair becomes a handler object using the same interface. This is the proof the interface works. The handlers themselves don't change — only how they're called.

**Migration order.** `Destination` comes from § Core vs extras and decides *which phase* touches the handler: core handlers become registry handlers in Phase 1; **`extras` means "leaves this repo"** — those handlers move to their owning app in Phase 4 (§ Server-side changes 3, tier 1 withdrawn) and never enter the core registry at all.

| Handler | Category | Needs send() | Current file | Destination |
|---------|----------|-------------|--------------|-------------|
| `handleScrollDirectives` | fire-on-change | no | `dom/directives.ts` | core |
| `handleHighlightDirectives` | fire-on-change | no | `dom/directives.ts` | core |
| `handleAnimateDirectives` | fire-on-change | no | `dom/directives.ts` | core |
| `handleToastDirectives` | fire-on-change | no | `dom/directives.ts` | core |
| `handleShadowRootHydration` | fire-on-change | no | `dom/directives.ts` | core |
| `setupScrollAway` | fire-on-change | no | `dom/scroll-away.ts` | core |
| `setupFxDOMEventTriggers` | wire-idempotent | no | `dom/directives.ts` | core |
| `setupDOMEventTriggerDelegation` | wire-idempotent | no | `dom/event-delegation.ts` | core |
| `uploadHandler.initializeFileInputs` | wire-idempotent | no | `upload/upload-handler.ts` | core |
| `handleAutoClickDirectives` | fire-on-change | no | `dom/directives.ts` | extras |
| `handleResizeDirectives` | fire-on-change | no | `dom/resize.ts` | extras |
| `handlePreviewBridgeDirectives` | fire-on-change | no | `dom/directives.ts` | extras |
| `setupSpy` | fire-on-change | no | `dom/spy.ts` | extras |
| `handleAreaSelectDirectives` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleRegionSelectDirectives` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleTextSelectDirectives` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleViewportReportDirectives` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleProxyBridgeDirectives` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleURLHashDirective` | fire-on-change | **yes** | `dom/directives.ts` | extras |
| `handleIframeAutoHeightDirectives` | fire-on-change | no | `dom/directives.ts` | audit → delete |

Note that `Needs send()` and `Destination` are **orthogonal** — that's why the phasing is axed on destination and not on `needsServerChannel` (see § Implementation phases).

### Reference design: Phoenix LiveView Hooks

The closest prior art is [Phoenix LiveView's Hooks system](https://hexdocs.pm/phoenix_live_view/js-interop.html#client-hooks-via-phx-hook). In LiveView:

- Custom JS behavior is attached via `phx-hook="MyHook"` attribute
- Hooks register by name and receive `mounted`, `updated`, `destroyed`, `disconnected`, `reconnected` callbacks
- Each hook instance has access to `this.el`, `this.pushEvent()` (≈ our `send()`), and `this.handleEvent()` (server → client push)

**Key differences for our design:**

1. LiveView hooks are per-element instances; our handlers are per-attribute-type singletons that scan the DOM. Both models work — the singleton scan model matches our existing architecture better and avoids instance allocation overhead for large lists.

2. LiveView hooks use a single `phx-hook` attribute for the entry point; our system uses the attribute name itself (`lvt-fx:scroll`, `lvt-el:addClass`, etc.) as the entry point, which is more declarative and doesn't require naming the hook separately.

3. The two models could coexist — attribute handlers for the declarative case, `lvt-hook="name"` for complex stateful widgets. But that is **not** a commitment this plan makes: per-element hooks are demand-gated in Phase 6 (§ Open questions 2), and nothing in M1 or M2 depends on them.

## Writing your own attribute

This is the issue's headline goal, so it gets worked examples rather than a description. All three are **tier 3** (§ Server-side changes 3): the app's own code, no fork, no npm package, no build step.

The API has **two layers**. Examples 1 and 2 use the declarative one — declare an attribute *name*, get called per element — which is what most authors should ever touch. Example 3 shows the low-level `selectors` + `setup()` layer the built-ins use, and says when you'd actually need it. Both are the same public `AttributeHandler`, so an author is never on a lesser API than the framework itself.

These examples are the spec for the authoring guide and the `docs/examples/` app that Phase 3 ships.

### Example 1 — visual only, no server involvement

A copy-to-clipboard button. No `category` — it's derived (`onElementAdded` only ⇒ `wire-idempotent`), and no `name` — it defaults to the attribute.

```html
<button lvt-x:copy="{{.ShareURL}}">Copy link</button>
```

```html
<script src="{{ lvtClientScriptURL }}" defer></script>
<script>
  LiveTemplateClient.registerAttribute({
    attribute: "lvt-x:copy",          // plain name — the framework builds the selector
    onElementAdded(el, ctx) {         // once per element, not once per render
      el.addEventListener("click", () => navigator.clipboard.writeText(ctx.value));
    },
  });
</script>
```

That is the whole handler. Compare what the low-level form (Example 3) would have required: an escaped selector string `'[lvt-x\\:copy]'` written twice, a manual `querySelectorAll` loop, an `el.__copyWired` flag to stop listeners stacking on every render, and an `el.getAttribute()` to read back a value the framework already had.

**Why `ctx.value` and not a destructured `value`:** the listener is attached once but fires later, possibly many renders later, and `{{.ShareURL}}` can change in between. `ctx.value` re-reads the attribute at access time, so the click copies the *current* URL. A destructured `const { value }` would capture the URL as it was at wiring time and quietly copy a stale link — which is exactly why the context exposes accessors rather than plain fields.

The server never learns this attribute exists. It renders as opaque HTML in the Statics array, exactly like any other attribute (§ Non-goals).

### Example 2 — dispatching a server action

A star-rating widget. `needsServerChannel: true` puts `send` on the element context; the message routes through the same WS/HTTP path a normal action uses.

```html
<div lvt-x:rating="SetRating" data-value="{{.Rating}}"></div>
```

```js
LiveTemplateClient.registerAttribute({
  attribute: "lvt-x:rating",
  needsServerChannel: true,
  onElementAdded(el, ctx) {
    el.addEventListener("click", (e) => {
      // ctx.value = the action name; ctx.send = the live transport, so this
      // keeps working across a reconnect and across an action-name change.
      ctx.send({ action: ctx.value, data: { stars: Number(e.target.dataset.star) } });
    });
  },
});
```

An empty attribute value (`lvt-x:rating=""`) is rejected by the framework with a warning before `onElementAdded` runs — every handler needed that check, so it stopped being the author's job.

Server side is an ordinary Controller action — nothing attribute-specific:

```go
func (c *Controller) SetRating(state State, ctx *Context) (State, error) {
    state.Rating = ctx.GetInt("stars")     // or ctx.Bind(&payload) for structured data
    return state, nil
}
```

That symmetry is the point: a custom attribute's action is indistinguishable from an `lvt-on:click` action once it reaches the server. No new wire message type, no server-side registration (§ Non-goals).

### Example 3 — the low-level form, and when you still need it

Examples 1 and 2 use the **declarative layer**: declare an attribute name, get called per element. The **low-level layer** (`selectors` + `setup(ctx)`) is still there, and the built-ins use it — but it is now an escape hatch rather than the entry price, and an author reaches for it only when one of these is true:

- the handler owns **cross-element state** (a drag in progress, a shared overlay) rather than per-element state;
- it must match **more than one attribute**, or a selector that isn't a plain attribute presence;
- it needs to run **before or after** the per-element pass to batch work.

Everything else — selector escaping, the scan, wiring idempotence, sweeping elements that lost the attribute or left the DOM — is the framework's job in the declarative layer. What follows is what the low level looks like, and it is also a fair picture of what the declarative layer does *for* you. `handleAreaSelectDirectives` is the reference implementation:

```js
const armed = new WeakMap();   // element → { action, cleanup(), updateSend() }

setup({ scanRoot, send }) {
  // 1. Sweep first. An element can lose the attribute via a server diff, or be
  //    detached entirely — without this it keeps its listeners and silently
  //    dispatches the stale action forever.
  for (const [el, entry] of Array.from(armed)) {
    if (!el.isConnected || !el.hasAttribute("lvt-x:mything")) entry.cleanup();
  }

  for (const el of scanRoot.querySelectorAll('[lvt-x\\:mything]')) {
    const action = el.getAttribute("lvt-x:mything");
    const existing = armed.get(el);

    // 2. Idempotent re-arm: same action → keep listeners, refresh the captured
    //    send (it changes after a WebSocket reconnect rebuilds the transport).
    if (existing && existing.action === action) { existing.updateSend(send); continue; }

    // 3. Action changed → tear the old one down before re-attaching.
    if (existing) existing.cleanup();
    armed.set(el, attachMyThing(el, action, send));
  }
}
```

**Three mistakes this prevents** — each has a real analogue in the built-ins, and **the declarative layer prevents all three by construction**, which is the main argument for having it:

| Mistake | Consequence | Declarative layer |
|---------|-------------|-------------------|
| No sweep | an element whose attribute the server removed keeps firing | framework diffs the match set and calls `onElementRemoved` |
| Non-idempotent setup | `setup` runs after *every* render, so a naive `addEventListener` stacks one listener per render | `onElementAdded` fires once per element |
| Captured `send` never refreshed | dispatches through a dead transport after a reconnect | `ctx.send` resolves the live transport on access |

An author who never leaves the declarative layer cannot make any of them.

### Choosing a category

| Category | Runs | Use when |
|----------|------|----------|
| `fire-on-change` | every render | the handler reacts to a *value* changing on an existing element (highlight flash, scroll-to, a re-read of the attribute's value) |
| `wire-idempotent` | only when nodes were added or a directive attribute was touched | the handler walks many descendants to attach listeners — skipping saves ~150–200 ms per render at 80k nodes |
| `always` | every render, unconditionally | the handler must process *removals* too, not just current matches |

**Declarative handlers normally don't set this** — it's derived from which callbacks you supply:

| You declare | Derived category | Because |
|-------------|------------------|---------|
| `onElement` (with or without the others) | `fire-on-change` | you're reacting to a value that can change on an existing element |
| only `onElementAdded` / `onElementRemoved` | `wire-idempotent` | you're wiring new elements; re-running on an unchanged DOM would do nothing |

An explicit `category` always wins — set it when the derivation is wrong for you, most often `always` when the handler must see removals on every render regardless.

For a **low-level** handler, `category` is required and there is nothing to derive from. When unsure, start with `fire-on-change` and a cheap attribute selector — that's what most built-ins do, and an empty `querySelectorAll` on a 10k-row table costs 1–3 ms.

### Testing a custom attribute

Unit-test the handler directly against jsdom. A declarative handler needs no framework harness at all — call `onElementAdded(el, ctx)` with a stub context (`{ get value() {…}, send })`) and assert on the DOM and dispatched messages. A low-level handler is called the same way the framework calls it: `setup({ scanRoot, wrapperRoot, send })`. That is how the built-ins' tests work today (`tests/directives.test.ts`, `tests/text-select.test.ts`). For anything touching real browser behaviour (pointer capture, clipboard, selection), a chromedp E2E is the honest test — see the delivery protocol.

### What you never have to do

No forking the client. No server-side registration or capability declaration — the server keeps treating the attribute as opaque HTML. No new wire message type. No build step, unless you choose to publish (tier 2). No coordination with a livetemplate release.

## Server-side changes (minimal)

### 1. Attribute census advertisement — ships with the registry, not optional

The server tells the client which `lvt-*` attributes appear in its templates. The client diffs that against its registry and console-warns for each attribute nothing handles.

**This lands in Phase 2 — immediately after the registry and before any extraction — not later as an optional extra.** It is small enough that deferring it costs more than doing it, and the registry's two safety nets — the unregistered-attribute warning and the duplicate-selector warning (§ Open questions 3) — are the only things standing between a developer and an attribute that silently does nothing. Shipping a plugin API whose failure mode is *silence* and adding the diagnostics in a later phase gets the ordering exactly backwards.

**An earlier draft of this section proposed the wrong shape.** *[decision history — skippable, but the three reasons say useful things about the wire contract]* It sketched a struct:

```go
type AttributeCapability struct {
    Name    string `json:"name"`
    Version string `json:"version,omitempty"`
}
```

Three problems, all visible in the existing code:

1. **It changes the wire shape of an existing field.** `ResponseMetadata.Capabilities` is `[]string` (`internal/send/response.go`), sent only on initial render. Swapping in a struct array is a wire-format change, which means cross-repo lockstep with the client and lvt's goldens — the opposite of minimal.
2. **It doesn't meet the bar the field documents for itself.** `dispatch.go` states the rule plainly: *"A capability should only be in the list if the client changes its behavior based on it."* The existing four (`change`, `validate`, `upload`, `progressive_enhancement`) each flip a client-side switch. A list of attribute names doesn't — the client's registry already decides behavior. An attribute census is a *template fact*, not a server capability, and belongs beside `capabilities`, not inside it.
3. **`Version` has no consumer.** Nothing in this plan reads it. Drop it; the existing capabilities are unversioned, and no artifact this plan ships needs a second version pin (§ 3).

**The minimal shape** is an additive sibling field, `omitempty`, initial render only — no change to any existing field:

```go
type ResponseMetadata struct {
    // …existing fields…
    Capabilities []string `json:"capabilities,omitempty"` // server features (unchanged)
    Attributes   []string `json:"attributes,omitempty"`   // lvt-* attribute names found in the template
}
```

**The extraction machinery already exists.** `topic_wired_actions.go` runs `extractWiredActionNames` — a regex scan over the template string with a `{{…}}`-strip discipline, called once at parse time (`template.go`, alongside `t.wiredActions = …`) and cached on the `Template`. The attribute census is the same function shape over the same input: one sibling regex, one sibling field. That is the whole server-side cost.

**Why the server does this rather than the client scanning the DOM.** A client-side scan would need no server involvement at all — but it can only see attributes that have *already rendered*. An `lvt-fx:text-select` inside a `{{if}}` branch stays invisible until a user reaches that branch, at which point the attribute silently no-ops and the warning arrives (if at all) long after the mistake. The server sees the entire template including unrendered branches, so the warning fires on the first render. That is precisely the failure mode extraction introduces — an app on the core bundle only, with an extras attribute in a conditional branch — so the diagnostic has to be the one that catches it, not the one that happens to be free.

A DOM scan is also per-render work at every scale (the same cost concern that produced the wire-idempotent category); the census is computed once at parse.

**Client side:** on initial render, diff `meta.attributes` against the registry's known selectors and `console.warn` once per unhandled attribute, naming the likely cause (`lvt-fx:text-select` has no registered handler — load the bundle that provides it).

**Limitations — measured in Phase 2's audit:**

- ~~**Associated templates.**~~ **Not a limitation — measured in both shapes, then deleted.** The concern was that attributes inside separately-parsed associated templates (`{{template "x" .}}`) would be outside the scanned string, so the census would under-report exactly the large template sets most likely to need the warning. It doesn't, and the two shapes need separate evidence because they take different paths: a `{{define}}` in the *same* string is inlined by `parse.FlattenTemplate`, which `parseInternal` runs before the extraction site; a `{{define}}` in a *separately parsed file* first joins one template set via `parseSources`, and is then flattened by the same step. Both pinned — `TestAttributeCensus_Covers{AssociatedTemplates,SeparatelyParsedSources}` — so a change to the flatten or parse order fails a test rather than silently hollowing out the diagnostic. **Review note: the first version of this deletion rested on the same-string probe alone, which is not the shape the plan was worried about.**
- **Dynamic attribute names.** `lvt-fx:{{.Kind}}` collapses to the bare namespace `lvt-fx:` under the `{{…}}`-strip discipline and is discarded — the same behavior wired-action extraction already has for `name="{{.X}}"`. Discarded rather than reported, because `lvt-fx:` would name a handler that cannot exist and produce a warning the developer can do nothing about.
- **Attributes only ever added client-side.** The census reads template source, so an attribute a script sets via `setAttribute` is invisible to it. Right side to err on: a handler registered for one is not a mistake.

None is worth engineering around. All are written down — in the code and in `docs/references/client-attributes.md` — so the warning's absence is never read as proof of correctness.

**Scope, settled in Phase 2:** the census reports `lvt-*` attributes in *attribute position*, reduced to the part before `:on:`, and reports them **all** — including `lvt-on:`/`lvt-mod:`/`lvt-nav:`. See Phase 2's "Two design decisions the plan left open" for why, and for the consequence Phase 1 has to absorb.

### 2. Bracket expansion generalization

`internal/render/bracket_expand.go` currently hardcodes `lvt-(?:el|fx|form)` in the regex. Generalize to match any `lvt-` prefix so custom attributes can also use bracket syntax:

```go
// Before: (lvt-(?:el|fx|form):[^=\s]+?:on:)
// Shipped: (lvt-[a-zA-Z][a-zA-Z0-9-]*:[^=\s]+?:on:)
```

The shipped class is wider than the drafted `[a-z]+` so a hyphenated custom namespace (`lvt-x-acme:orgchart:…`) works too; a bare `lvt-` attribute with no namespace segment (`lvt-toast-stack`) still does not carry bracket syntax.

**Correction — the note that was here was wrong, and Phase 2's audit measured it.** It claimed the state-suffix constraint (`:(?:pending|success|error|done)`) would still reject `lvt-on:`/`lvt-mod:`/`lvt-nav:` under the widened prefix "since those prefixes don't use the `:on:[actions]:state` pattern". That is a claim about how those namespaces are *used*, not about what the regex *accepts*: `lvt-on:click:on:[a,b]:pending="x"` matches the widened pattern structurally, and Go's RE2 has no lookahead to write `(?!on:|mod:|nav:)`.

**Decision: allow it, don't special-case it.** Such an attribute is already malformed — those three namespaces have no `:on:[actions]:state` form at all — so the client has no handler for either spelling and expansion just turns one inert attribute into two. An exclusion list would buy nothing and would be one more place every future namespace has to be added to. Real-world `lvt-on:click="Save"` / `lvt-mod:debounce="300"` / `lvt-nav:no-intercept` are untouched: the match is anchored by the `:on:[…]:state` tail, which they don't carry. Both behaviors are pinned by tests in `bracket_expand_test.go` so this is never re-derived from the regex.

### 3. Distribution — three tiers of extras, not one

Extraction only pays off if apps can load the extracted handlers, and today the server owns the client's `<script>` URL. `client_assets.go` pins a single bundle:

```go
const ClientVersion = "0.18.2"                    // moves in lockstep with each livetemplate release
const clientCDNBase = "https://cdn.jsdelivr.net/npm/@livetemplate/client@" + ClientVersion
const ClientScriptURL = clientCDNBase + "/dist/livetemplate-client.browser.js"
```

…exposed to every template as `{{ lvtClientScriptURL }}` via `frameworkTemplateFuncs()`.

**There are three kinds of extras and they need three different answers.** *[the sentence that follows is decision history — skippable]* An earlier draft of this section answered only the first and read as if it answered all of them, which would have left the issue's headline goal ("anyone can write their own custom `lvt` attributes") unaddressed by the distribution design.

| Tier | Who writes it | Example | Distribution | Versioning |
|------|---------------|---------|--------------|------------|
| ~~**1. First-party**~~ | ~~us — handlers extracted from core~~ | — | **withdrawn — see below** | — |
| **2. Community** | third parties, published | `@acme/lvt-tooltip` on npm | app writes its own `<script>` tag, or imports the package | independent; **cannot** track core releases |
| **3. Project-specific** | the app itself, never published | prereview's area-select; one `lvt-x:orgchart` handler in an internal app | inline `<script>` or the app's own bundle | none — it's just app code |

#### Tier 1 is withdrawn — a first-party "extras" bundle shouldn't exist

An earlier draft made first-party extras their own tier: a second esbuild entry in `@livetemplate/client`, blessed with a server-side `{{ lvtClientExtrasScriptURL }}` template func. **Review objection: that locks in the decision that "extras" is a category worth promoting at all, when the extras handlers aren't a general concept — they're one app's code.** The objection holds, and three pieces of evidence say so.

**1. The server-side API would ship with no consumer.** `lvtClientScriptURL` is well-used — 30 templates across `docs/examples/` reference it. But `lvtClientExtrasScriptURL`'s only intended consumer was prereview, and **prereview doesn't use the CDN path at all**: it `go:embed`s the bundle (`internal/assets/assets.go`) and its template hardcodes `<script defer src="/livetemplate-client.js">`, because it's reviewed over Tailscale/devbox and often offline. Meanwhile zero of the 30 docs examples uses any extras-destination attribute. So the template func would ship with no consumer *today* — adding permanent server surface for a hypothetical.

**2. "First-party extras" has no membership rule.** Core has one (§ Core vs extras: framework-shipped code or more than one app uses it). Extras would have only "ours, but not core" — which is exactly the accretion dynamic that produced the kitchen sink in the first place. The next borderline handler goes to extras because that's where borderline handlers go, and in three releases the second bucket needs its own extraction plan.

**3. Every extras-destination handler is prereview-only.** Not "mostly" — all ten, per the census (an eleventh, `iframe-autoheight`, has no consumer anywhere and is deleted rather than moved). So "first-party extras bundle" decodes to "prereview's code that we keep maintaining, in a different file." Moving it out of the core *bundle* while keeping it in the core *repo* fixes the download size and none of the ownership problem.

**Options, with the cost priced:**

- **(A) Second entry point + server template func** — the withdrawn draft. Rejected: adds a consumer-less server API and blesses the category permanently. Removing a template func later is itself a breaking change.
- **(B) Second entry point, no server promotion.** The extras bundle builds in the client repo; whoever wants it vendors it exactly as prereview already vendors the core bundle (`make sync-client`). Drops the blessed API, keeps the cheap build path. Still leaves "extras" as a maintained category, so problems 2 and 3 survive.
- **(C) No extras tier — prereview owns its handlers.** They move to prereview and become tier 3, indistinguishable from any other app's custom attributes.

**Recommendation: (C), with (B) as the fallback.**

The cost of (C) is real and worth stating plainly: prereview has **no JavaScript toolchain at all** — no `package.json`, no `tsconfig.json`, not one `.ts` or `.js` source. It only vendors the built bundle. So (C) means prereview acquires package.json + tsconfig + esbuild + jest, and ~2,400 lines plus their tests move into it. The `make sync-client` dependency on `../client/dist/` already exists, but it would have to invert: prereview builds its own bundle instead of copying ours. Call it a day of work.

**That cost is the argument for (C), not against it.** prereview owning its handlers through the public API is the only real test of whether the plugin API works for an app that isn't us. If prereview *can't* do it — if it needs an internal import, a build hook we don't document, or a lifecycle callback the interface doesn't expose — then the API isn't finished, and that is a finding we want in Phase 3 rather than after we've told external users the API is ready. Every other validation in this plan is us testing ourselves.

Take (B) if the toolchain turns out to cost materially more than a day. It still removes the promotion the objection targets; it just leaves the ownership question for later.

**Note what this removes rather than adds:** no `ClientExtrasScriptURL`, no `lvtClientExtrasScriptURL`, no second wire-pinned artifact. The server-side surface *shrinks*, which is the direction § Non-goals already points.

Tiers 2 and 3 are unaffected — they were the answer to "what about community and project-specific extras", and they stay exactly as they were. Tier 1 was the odd one out; prereview simply becomes an instance of tier 3, which is what the census said it was all along.

#### Tiers 2 and 3 — everything the server should stay out of

Neither gets a server-side template func. The server has no business knowing a third party's CDN URL, and a project-specific handler has no URL at all. Both load exactly the way any other script does:

```html
<script src="{{ lvtClientScriptURL }}" defer></script>

<!-- tier 2: third-party code from a CDN we don't control — pin with SRI -->
<script src="https://cdn.example.com/@acme/lvt-tooltip@1.2.0.js"
        integrity="sha384-…" crossorigin="anonymous"></script>

<!-- tier 3: the app's own handler, no packaging at all -->
<script>
  LiveTemplateClient.registerAttribute({ name: "orgchart", /* … */ });
</script>
```

This works **only** because of the two decisions in § 2 Bootstrap timing — static registration and late-registration catch-up. Those were derived from the first-party extras case, but they are what makes tiers 2 and 3 possible at all: any script that runs after the client has connected can still register and have its handler take effect immediately. Without the catch-up, every tier-3 inline script would silently do nothing until the next render.

**What tiers 2 and 3 require:**

1. **The handler interface is public API from day one.** `AttributeHandler` (both variants), `SetupContext`, `ElementContext`, and `SendFn` ship as exported TypeScript types; `registerAttribute` is reachable both as a named export (ESM) and on the `LiveTemplateClient` global (IIFE). The global's *name* becomes API too.
2. **A stated compatibility policy for the registry contract.** A built-in that breaks on an interface change gets fixed in the same commit. A tier-2 handler doesn't — it's in someone else's repo, pinned to a version we don't control. The interface needs a semver commitment and a documented deprecation path before the first third party depends on it.

   This corrects the wait-and-see framing in § Open questions 5 ("once the handler API has survived a few releases unchanged"). It won't get that grace period: publishing a plugin API *is* freezing it. The stabilization has to be a deliberate policy from the phase that ships the registry, not an emergent property of nobody having used it yet.
3. **Namespace guidance** — see § Open questions 3, which stops being theoretical the moment third parties can register.
4. **Bracket-expansion generalization (§ 2) is what gives tiers 2 and 3 syntax parity.** Without it, `lvt-x:tooltip:on:[save,delete]:pending` doesn't expand and custom attributes are second-class next to `lvt-fx:*`. That change is small, but it's load-bearing for this comment's concern rather than a nice-to-have.
5. **tinkerdown's allowlist is a real constraint for markdown-sourced content.** `vocabulary.go` gates which `lvt-*` attributes survive sanitization. A tier-2 or tier-3 attribute is invisible in tinkerdown-rendered content until that allowlist admits it — so the allowlist needs an extension mechanism, not just a core/extras split.
6. **A security posture worth stating in the authoring docs.** A tier-2 handler is arbitrary third-party JS with full DOM access and — if it declares `needsServerChannel` — the ability to dispatch server actions through the same `send()` the framework uses. That's not a reason to avoid tier 2, but the docs should say plainly that registering a community handler is trusting its author with the session, and show the `integrity` / `crossorigin` pinning above rather than a bare `<script src>`. There is no exempt tier: with tier 1 withdrawn, every handler that isn't core is somebody's third-party or first-party app code, and the trust note applies to all of it.

**One property that carries across both tiers for free:** the attribute census warning (§ 1) compares template attributes against the registry without caring who registered what. A missing tier-2 script and a missing tier-3 inline script produce the same first-render warning.

### 4. Wired-action extraction

`topic_wired_actions.go` extracts `lvt-on:` action names for the publish collision warning. Custom attributes that generate server actions (via `send()`) should be opt-in to this extraction. Every `needsServerChannel` handler is an extras handler (§ Migration table), so this lands in Phase 5 with the extraction release rather than in its own phase.

## Affected files

### Client (`github.com/livetemplate/client`)

| File | Change | Phase |
|------|--------|-------|
| `livetemplate-client.ts` | Replace hardcoded directive calls with registry loop; add static `registerAttribute()`; late-registration catch-up | 1 |
| NEW: `attribute-registry.ts` | `AttributeHandler` interface, module-level registry implementation | 1 |
| NEW: `event-spec.ts` | Formalized lifecycle event types | 1 |
| `dom/scroll-away.ts` | Convert to handler object (stays core) | 1 |
| `dom/event-delegation.ts` | Convert DOM event trigger delegation to handler object (stays core) | 1 |
| `upload/upload-handler.ts` | Convert `initializeFileInputs` to handler object (stays core) | 1 |
| `dom/reactive-attributes.ts` | May stay as-is (lifecycle-driven, not post-render scan) | 1 |
| `livetemplate-client.ts` | Diff `meta.attributes` against the registry; `console.warn` per unhandled attribute | 2 |
| `dom/event-delegation.ts` | Feed handler-declared `delegatedEvents` into the document-level listener set (`DELEGATED_EVENT_TYPES` becomes a base list, not the whole list) | 3 |
| `index`/`types` exports | Publish `AttributeHandler` (both variants) / `SetupContext` / `ElementContext` / `SendFn`; `registerAttribute` as named export + documented global | 3 |
| `dom/directives.ts` | Core handlers → handler objects (P1); **~2,400 lines of prereview-owned handlers deleted** once relocated (P4) | 1, 4 |
| `dom/resize.ts`, `dom/spy.ts` | Deleted from this repo (move to prereview) | 4 |
| `tests/text-select.test.ts`, `tests/resize.test.ts`, `tests/viewport-report.test.ts`, … | Move with their handlers | 4 |

### Downstream (blast radius of extraction)

| Repo | Change | Phase |
|------|--------|-------|
| `docs` | Runnable `docs/examples/` app registering a tier-3 custom attribute + authoring guide | 3 |
| `tinkerdown` | Allowlist extension mechanism for app-defined attributes (tiers 2–3) | 3 |
| `prereview` | **Takes ownership of its 10 handlers**: package.json + tsconfig + esbuild + jest, handlers + tests moved in, own bundle built and `go:embed`ed, `<script>` tag added to `page.tmpl` | 4–5 |
| `tinkerdown` | `vocabulary.go` sanitizer allowlist + `docs/reference/lvt-attributes.md` split core vs extras vocabulary | 5 |

### Server (`github.com/livetemplate/livetemplate`)

| File | Change | Phase |
|------|--------|-------|
| `internal/render/bracket_expand.go` | Generalize regex prefix | 2 |
| `internal/render/bracket_expand_test.go` | Add tests for custom prefixes | 2 |
| NEW: `attribute_census.go` + `_test.go` | Scan statics for `lvt-*` attribute names. *(Drift: planned as a sibling inside `topic_wired_actions.go`; split out because that file's header is scoped to wired actions and publish collisions.)* | 2 |
| `template.go` | Cache the census beside `t.wiredActions = extractWiredActionNames(text)`; copy it in `Clone()` beside `wiredActions`; copy it into `mountCfg` beside `mountCfg.Capabilities = detectCapabilities(…)` | 2 |
| `internal/send/response.go` | Add `Attributes []string` to `ResponseMetadata` (`omitempty`, initial render only) | 2 |
| `mount.go` | Set `Attributes: h.config.Attributes` at **both** initial-render `ResponseMetadata` sites — the WS initial response and the HTTP JSON one. They are the only two `Capabilities:` assignments in the tree, and they are different *transports*: setting one and not the other ships a warning that fires over WebSocket and stays silent over HTTP | 2 |
| `topic_wired_actions.go` | Extend wired-action extraction to custom `send()` action names | 5 |
| Docs | Document the event spec and the handler API | 3 |

The client side of the census (diff `meta.attributes` against the registry, `console.warn` per unhandled attribute) lands in `livetemplate-client.ts` beside the existing `response.meta?.capabilities` handling.

## Open questions

### 1. Public plugin API vs internal refactor

The issue says "anyone can write their own custom lvt attributes." This means a **public plugin API**, not just internal cleanup. The plan assumes this — `registerAttribute()` is a public API on `LiveTemplateClient`.

**Decision needed:** Should custom handlers be registered:
- (a) In JS, statically: `LiveTemplateClient.registerAttribute(myHandler)` — simplest, most flexible
- (b) Declaratively via a server-advertised capability — cleaner but more complex
- (c) Both (a) for user-defined, server capability for validation

Recommendation: **(c), both, within M1 (Phases 1–2).** (a) is how handlers are registered; the server side of (c) is the attribute census that makes a *missing* registration visible (§ Server-side changes 1). They are two halves of one shippable unit, so the earlier "(a) now, (c) later" split is gone.

Note (a) is also no longer fully open: § 2 Bootstrap timing establishes that it *must* be static and *must* support late registration, because a second bundle can only run after the core bundle has already auto-initialized. An instance-scoped `client.registerAttribute()` is off the table regardless of which option wins.

(b) alone stays rejected — the server can enumerate what a template *uses*, but it has no way to know what JS the browser registered, so it can never be the registration mechanism.

### 2. Per-element hooks (`lvt-hook`)

Should we also support a per-element hook model (like LiveView's `phx-hook`) in addition to the singleton scan model? Per-element hooks are better for complex, stateful behaviors (code editors, map widgets, third-party library integrations).

**Trade-off:** Per-element hooks require instance management (create on mount, call `updated` on morphdom, call `destroyed` on removal). The singleton scan model is simpler but less powerful.

Recommendation: Phase 1 ships the singleton scan registry. Phase 6 adds `lvt-hook="name"` if demand exists.

### 3. Namespace collisions

If external contributors can register `lvt-fx:*` handlers, they could collide with built-in ones. Options:
- (a) First-registered wins (built-ins always win)
- (b) Custom attributes use a different prefix (`lvt-x-*` or `lvt-ext-*`)
- (c) No restriction; user takes responsibility

Recommendation: **(a) plus a documented convention for tier 2/3** — built-ins registered first; duplicate selector registration logs a warning unconditionally (not dev-mode-only).

Two things sharpen this once community and project-specific extras exist (§ Server-side changes 3):

- **Relocation removes the built-ins' head start for the departing handlers.** Once prereview's handlers register from prereview's own bundle they load *after* core, in the same position any app's handler would — correct (they get no more privilege than user code), but it means the duplicate-selector warning is the only thing between an app and a silently shadowed attribute.
- **(b) stops being a rejected option and becomes advice.** We can't *enforce* a prefix on third parties without rejecting registrations, which would be a worse failure than a collision. But the docs should recommend third-party attributes avoid `lvt-fx:` / `lvt-el:` / `lvt-form:` — those namespaces are ours and will keep growing, so a community `lvt-fx:tooltip` is a collision waiting for the release that adds one. A distinguishable namespace (`lvt-x:tooltip`, vendor-prefixed) costs the author nothing and is why § 2's bracket-expansion generalization matches any `lvt-` prefix rather than an enumerated set.

### 4. Server-to-client push for custom attributes

Some advanced use cases need the server to push data to a specific custom attribute handler (e.g., "scroll to message ID X"). LiveView solves this with `push_event`. Should we support this?

Recommendation: Defer to Phase 6. Document the workaround: use `data-*` attributes in the template to pass server state to the handler on each render.

### 5. Bundle-level extraction vs repo-level relocation

Getting the domain handlers out of every app's *download* and out of the maintainer's *tree* are two different moves. Should prereview's handlers relocate into prereview's own repository, or just out of the shipped bundle?

- (a) **Bundle-level only.** Extras stay in the client repo as a second entry point. One build, one version pin, one test suite.
- (b) **Repo-level relocation.** Each app owns its handlers. Truest reading of "not the core library's problem", but prereview then owns TypeScript build infrastructure.

**Answered: (b).** Full reasoning and the priced alternatives are in § Server-side changes 3 ("Tier 1 is withdrawn"); Phase 4 implements it.

**This answer moved twice, so here is what actually decided it** *[decision history — skippable]* — the churn is worth recording, because two plausible-sounding arguments were wrong in different ways.

- **v1 recommended (a) now, (b) later**, justified as "once the handler API has survived a few releases unchanged, relocation is a pure move against a stable interface."
- **v2 kept (a) but withdrew that justification.** Tiers 2 and 3 mean the contract is frozen the moment `registerAttribute` is published — there is no grace period in which it's public-but-not-really. So (a)-first was never buying interface flexibility; the argument collapsed to pure cost.
- **v3 (this) flips to (b),** on a review objection: a first-party extras bundle promotes "extras" to a permanent category with no membership rule, and its server-side API would have shipped with no consumer. What decided it was noticing that the remaining argument for (a) — cost — is a *sequencing* argument, and this project's standing answer to sequencing is to skip the intermediate state: LiveTemplate is alpha with no external users, so we collapse to the end state directly rather than landing a staging post we intend to dismantle. (a) *is* a staging post.

The cost of (b) is real and unchanged (prereview has no JS toolchain). What changed is that it stopped being weighed against a benefit — once interface stability wasn't on offer, (a)'s only remaining virtue was being cheaper *this month*.

## Implementation phases — progress tracker

**Why the phase axis is core-vs-extras, not `needsServerChannel`.** An earlier draft split the phases on whether a handler needs `send()`. The census shows that axis cuts straight across the extraction: the `needsServerChannel: false` group (preview-bridge, resize, spy, auto-click) and the `true` group (area-select, region-select, text-select, viewport-report, proxy-bridge, url-hash) are *both* almost entirely prereview-only. Phasing on it would migrate prereview's ~2,400 lines **into** the core registry in M1 and then move them **out** in M2 — churn that the second motivation of this plan argues directly against. `needsServerChannel` remains a per-handler property (it decides whether `send` is available on the setup/element context); it is not a phase boundary.

**Milestones.** M1 *Registry* (Phases 1–3) makes the interface exist, loud, and public. M2 *Extraction* (Phases 4–5) empties the kitchen sink. M3 *Hooks* (Phase 6) is demand-gated and stays outline-only until M2 lands.

### State

Done: Phase 2's **livetemplate half only** (census extractor, `meta.attributes` on both initial-render transports, bracket-namespace generalization, docs). Next: **Phase 1** (client repo) — it is the prerequisite for Phase 2's client half and for the E2E bar Phase 2 could not meet. Phases 1, 3 and 4 are client-repo work and cannot be delivered from a livetemplate branch.

### Progress tracker

| # | Phase | Milestone | Repos touched | Status |
|---|-------|-----------|---------------|--------|
| 1 | Registry + core handler migration | M1 | client | ☐ not started |
| 2 | Attribute census + diagnostics | M1 | livetemplate, client | ◐ **server half done** — client half blocked on Phase 1 |
| 3 | Public API surface + authoring docs | M1 | client, docs, tinkerdown | ☐ not started |
| 4 | Extraction — relocate prereview's handlers | M2 | client, prereview | ☐ not started |
| 5 | Downstream lockstep release | M2 | livetemplate, prereview, tinkerdown | ☐ not started |
| 6 | Per-element hooks | M3 | client, livetemplate | ☐ deferred — outline only |

### LLM session guide

1. **Read only what this phase needs.** Each phase's **Design refs** lists the sections that govern it. Skip the rest.
2. **Audit first, code second.** Complete the Audit sub-section before any implementation; enrich the Implementation list inline with what it turns up.
3. **Acceptance criteria are the success bar**, not the Implementation checkboxes. The first is always `/simplify`.
4. **Persist drift to the plan** in the same commit as the code.
5. **Learn is for surprises only.** "No surprises" is a valid entry.
6. **Stay in scope.** Phase implements its checklist + audit-derived additions. Ask before adding anything else.
7. **One coherent commit per phase**, and one PR per phase — never bundle two phases into one review.
8. **"Work on the next phase" → auto-position:** scan the tracker top-to-bottom for the first phase with unchecked boxes; read its Goal + Design refs + the prior phase's Learn; begin with Audit.
9. **Cross-repo phases land as one coordinated release.** Phases 2, 3, 4 and 5 each touch more than one repo — see the delivery protocol.

### Per-phase delivery protocol

Applies to every phase; defined once here rather than repeated per block. Precondition: `prereview` CLI + `gh` authenticated.

1. **Manual browser verification** at the devbox URL (`http://devbox:PORT/…`, never `curl`, never `localhost` — the URL has to work from the reviewer's machine).
2. **`/prereview` hand-off** → iterate until sign-off.
3. **Open the PR against `main`.** A PR not based on `main` runs only `claude-review`, not the full check set — a green tick on a non-`main` base is not a green build.
4. **`/prcommentsfix` convergence loop** against the review bot. Stop when successive rounds raise no new *functional* issue; decline the rest by reply, never silently.
5. **Final signoff → merge.** Never self-merge without explicit signoff.

**Cross-repo release order** (Phases 2–5): livetemplate core first, then client — the version coupling runs that way (`ClientVersion` in `client_assets.go` is bumped by the core release that adopts the new client). Use `release.sh`; never tag by hand.

---

#### Phase 1 (M1) — Registry + core handler migration (~1 session)

**Goal at end:** the ~20 hardcoded `handleXDirectives` calls are gone, replaced by a registry loop over handler objects; every core-destination handler is registered through the same public interface an external author would use; no behavior change.

**Design refs:** § Design 2 (Attribute handler registry, incl. Bootstrap timing), § Scope boundaries, § Migration table (core rows only), § Design 1 (Event spec).

**Audit** *(do first)*
- [ ] Read the current post-render block in `livetemplate-client.ts` end-to-end; confirm the fire-on-change / wire-idempotent split matches the comment block that documents it, and that nothing else has crept in since.
- [ ] Confirm the ordering constraints: `hydrateRedactedTokens` and `uploadHandler.hydratePreviews` run *before* the sweep and are order-sensitive; `changeAutoWirer.wireElements()` runs unconditionally after. Decide `always`-category vs stays-in-place for the auto-wirer and record which.
- [ ] **Decide how low-level handlers report what they claim** (§ Design 2, "What the registry knows it handles"). Declarative handlers answer this for free via `attribute`; low-level `selectors` is author-supplied and unverified. Either the loop consults `selectors` to dispatch, or low-level handlers are excluded from the census comparison and the plan says so. Phase 2's diagnostics depend on the answer, so it cannot be deferred past this phase.
- [ ] Verify `autoInit`'s call site and readyState branch still match § 2's description before building the static-registration design on it.
- [ ] Confirm `CSS.escape` produces a working attribute selector for every `lvt-` name shape in use (the colon is the case that matters: `lvt-x:copy` → `[lvt-x\:copy]`). If any built-in's selector isn't a plain attribute-presence match, it stays on the low-level layer — that's expected, not a failure.
- [ ] **Enumerate the 9 core handlers' selectors and confirm no two overlap.** The duplicate-selector warning ships in this phase and fires at registration — if e.g. `setupFxDOMEventTriggers` and `setupDOMEventTriggerDelegation` both claim `lvt-el:`/`lvt-fx:` territory, the warning needs a scope narrower than "same selector string" before it ships, or this phase's own E2E bar (zero new console warnings) fails against its own new feature.

**Implementation**
- [ ] `attribute-registry.ts`: `AttributeHandler` (declarative + low-level variants), `SetupContext`, `ElementContext`, `SendFn`, module-level registry. **Every extension point is an optional member** — the interface is public and semver'd from day one (H6), so future hooks (morphdom predicates, `delegatedEvents`) must be addable as a minor. Sealing it now buys nothing and costs a major bump later
- [ ] `event-spec.ts`: formalize the lifecycle contract (§ Design 1)
- [ ] Static `LiveTemplateClient.registerAttribute()` reading/writing the module registry
- [ ] Late-registration catch-up: registering after a client exists immediately runs the handler against each live wrapper — `setup()` for a low-level handler, a full scan + `onElementAdded` pass for a declarative one
- [ ] **Declarative layer**: `attribute` + `onElementAdded` / `onElement` / `onElementRemoved`, with the framework owning selector escaping (`CSS.escape`), the scan, per-element match tracking, empty-value rejection, and sweep-on-removal. Reject a handler supplying both `attribute` and `selectors`
- [ ] Replace the hardcoded sequence with the registry loop; loop reads the registry live, not a construction-time snapshot
- [ ] Migrate the 9 core-destination handlers (§ Migration table) to handler objects
- [ ] Wire and unit-test the `needsServerChannel` / `send()` path with a test-only handler — no core handler exercises it, and Phase 4 must not be debugging the extraction and an untried contract simultaneously
- [ ] Duplicate-selector warning on registration (§ Open questions 3), unconditional

**Acceptance criteria**
- **Simplify:** `/simplify` over the diff before tests are assessed.
- **Unit:** existing `tests/directives.test.ts`, `event-delegation.test.ts`, `resize.test.ts`, `text-select.test.ts` pass unchanged — they are the regression net proving "only how they're called" changed. Plus: late registration runs setup immediately; duplicate selector warns; loop skips wire-idempotent handlers when nothing was added. **Declarative layer:** `onElementAdded` fires exactly once across N renders; `onElementRemoved` fires when the server diff drops the attribute *and* when the element detaches; an attribute name containing `:` matches without the author escaping anything; empty attribute value warns and does not invoke the callback.
- **Integration:** full `npm test` in the client repo green.
- **E2E:** chromedp in `lvt/e2e/` — a page using `lvt-fx:scroll` + `lvt-fx:highlight` + `lvt-scroll-sentinel` behaves identically pre/post refactor. Capture browser console, server stderr, WS frames, rendered HTML. **Zero new console warnings** is part of the bar.

**Learn** — surprises; plan drift fixed in this commit; feed-forward to Phase 2's audit; new/changed risks.

---

#### Phase 2 (M1) — Attribute census + diagnostics (~1 session)

**Goal at end:** a template using an attribute nothing handles produces a console warning on first render, over both transports.

**Design refs:** § Server-side changes 1 (whole section, incl. the two limitations), § Server-side changes 2, § Affected files (server table).

**Audit** *(done — findings below)*
- [x] **Measure the associated-template coverage** of `extractWiredActionNames`'s scan (§ Server-side changes 1, limitation 1). **Result: not a limitation, in either shape.** `parseInternal` runs `parse.FlattenTemplate` (template.go, `if parse.HasTemplateComposition(baseTemplate)`) *before* the extraction site, so `{{define}}`/`{{template}}`/`{{block}}` bodies are already inlined into `text` — and `ParseFiles`/`ParseFS` parse every source into one set before that flatten, so a `{{define}}` in a different *file* is covered too. Both measured by probe, then pinned as `TestAttributeCensus_Covers{AssociatedTemplates,SeparatelyParsedSources}`. **The union-across-associated-set work budgeted here is not needed and was not done.**
- [x] Confirm `Capabilities:` still has exactly two assignment sites and both are initial-render. **Confirmed.** Six `ResponseMetadata{` sites exist in `mount.go`; only `mount.go:762` (WS initial) and `mount.go:1273` (HTTP JSON initial) carry `Capabilities`. The other four are an action response (WS), an action response (HTTP POST), a rate-limit error, and a broadcast push — all correctly left alone.
- [x] **Wire-format check** against lvt's goldens. **Green, no regen needed.** Ran lvt's suite against this branch via a scratch `GOWORK` pointing at this worktree. (The three `*FullFlow` integration tests fail under that override because they shell out to `go build` in a temp dir that inherits the env var — verified they pass with stock `GOWORK=off`. Env artifact, not a regression.)
- [x] Confirm the bracket-expansion state-suffix constraint really does reject `lvt-on:`/`lvt-mod:`/`lvt-nav:` under the widened prefix regex. **It does NOT — this plan's § Server-side changes 2 was wrong.** `lvt-on:click:on:[a,b]:pending` matches the widened pattern structurally; the plan was reasoning about how those namespaces are *used*, not what the regex *accepts*, and Go's RE2 has no lookahead to exclude them cheaply. Decision recorded in § Server-side changes 2 and pinned by two tests.

**Implementation**
- [x] Census extractor — **drift: new file `attribute_census.go`, not `topic_wired_actions.go`.** That file's header comment is scoped entirely to wired actions and publish collisions; an unrelated census inside it would make the filename wrong. Same `{{…}}`-strip discipline as planned.
- [x] Cache on the `Template` beside `t.wiredActions`; copy into `mountCfg` beside `mountCfg.Capabilities = detectCapabilities(…)`. **Plus, not in the plan: the `Clone()` field copy** — production renders through per-session clones, so a master-only field is invisible in the real path.
- [x] `Attributes []string` on `ResponseMetadata` (`omitempty`)
- [x] Set it at **both** initial-render sites in `mount.go`
- [ ] Client: diff `meta.attributes` against the registry beside the existing `response.meta?.capabilities` handling; `console.warn` once per unhandled attribute, naming the likely cause — **blocked on Phase 1: there is no registry to diff against yet.**
- [x] Generalize the bracket-expansion regex + tests for custom prefixes
- [x] Document both census limitations where the warning is documented — `docs/references/client-attributes.md` § Attribute Census

**Two design decisions the plan left open, settled here**
- **What string goes in the array.** Names in *attribute position only*, reduced to the part before `:on:`. Attribute position matters more than it looks: a naive `lvt-[\w:-]+` scan over real docs/lvt templates returns `lvt-modal__overlay` and `lvt-fade-in` (CSS **class values**), `lvt-06af684d6b9c7483` (a wrapper **ID value**), and `lvt-id` matched inside `data-lvt-id` — a census built on it would warn about things that were never attributes. Reducing at `:on:` is what stops one authored `lvt-fx:animate:on:[a,b,c]:pending` reporting three times after bracket expansion.
- **The census is complete, not filtered.** `lvt-on:`/`lvt-mod:`/`lvt-nav:` are included. The server has no registry and no business deciding what "handled" means (H8); the client can ignore a name it knows, but cannot recover one the server dropped. **Consequence for Phase 1/3:** the client's diff must claim the core routing namespaces, or every app warns on first render.
- **Case is the author's, so the client must compare case-insensitively.** Caught in review, where the first version of this decision had it backwards. HTML parsers ASCII-lowercase attribute names, so an authored `lvt-el:addClass` reaches JavaScript only as `lvt-el:addclass` — the client already does `attrName.toLowerCase()` (`dom/reactive-attributes.ts`) and keys `METHOD_MAP` lowercase. The census reports the authored spelling anyway, because a warning naming `lvt-el:addclass` sends the developer grepping for a string that is nowhere in their template. **This is a wire contract that currently exists only as a Go comment, with no test spanning the two halves, and the server half ships first — Phase 1 must fold case before comparing.**

**Acceptance criteria**
- **Simplify:** `/simplify` over the diff. ✅
- **Unit:** Go — ✅ `TestExtractAttributeNames` (18 cases incl. `{{…}}` strip, nil when empty, CSS-class/`data-lvt-*`/ID-value exclusion, `:on:` collapse, `lvt-on:click` *not* truncated, case preservation), `TestExtractAttributeNames_SortedIndependentOfSourceOrder`, `TestAttributeCensus_{CoversAssociatedTemplates,SurvivesClone,CollapsesExpandedBracketSyntax}`, `TestResponseMetadata_Attributes{OmittedWhenNil,IncludedWhenSet}`. Bracket expansion accepts custom prefixes ✅ — **but does not reject `lvt-on:`**, see the audit; `TestExpandBracketAttributes/routing_namespace_in_bracket_context_expands` pins the measured behavior instead. TS — ⬚ **not met, blocked on Phase 1.**
- **Integration:** ✅ `go test ./...` green; lvt goldens green against this branch with no regen. ⬚ client `npm test` — no client change in this phase.
- **E2E:** ⬚ **not met, blocked on Phase 1.** The chromedp bar asserts a *console warning*, which cannot exist until the registry does. In-repo substitute, run and green: `TestHandle_AttributesIn{HTTP,WebSocket}InitialRender` assert the field's presence, contents and order in **both** transports' actual payload bytes; `TestHandle_AttributesOmittedWhenTemplateHasNone` and `TestHandle_AttributesNotRepeatedOnActionResponse` bound it. **Carry the real E2E into Phase 1's or Phase 3's acceptance — it is not satisfied.**

**Learn**
- The plan's § Server-side changes 2 asserted a regex property that measurement contradicted. Both surviving `{{…}}`-adjacent claims in this phase were checked by probe rather than reading; one of two was wrong. Worth repeating for Phase 3's `DELEGATED_EVENT_TYPES` audit item, which has exactly the same shape ("confirm the asymmetry still holds").
- The associated-template limitation the plan budgeted for does not exist, because flattening already ran. § Server-side changes 1 has been corrected — it would otherwise have been re-litigated in Phase 5.
- **Feed-forward to Phase 1:** the census reports the core routing namespaces, so the client's diff needs handlers or an allowlist for `lvt-on:`/`lvt-mod:`/`lvt-nav:`/`lvt-form:`/`lvt-key`/`lvt-ignore`/`lvt-upload`/`lvt-scroll-sentinel`/`lvt-autofocus`/`lvt-focus-trap` before the warning is switched on, or first render warns for every app. A census of the real docs+lvt+prereview template corpus found **46 distinct names** — that is the size of the set Phase 1 has to cover.
- **Risk raised:** Phase 2's diagnostic is the safety net for Phase 4's breaking extraction, and half of it (the warning itself) is now sequenced *after* the registry rather than before it. Phase 1 or 3 must land the client half, or M2 ships the break without the net.

---

#### Phase 3 (M1) — Public API surface + authoring docs (~1 session)

**Goal at end:** an app author can write a custom attribute handler by copying a worked example, and a third party can publish one — both using only published types and documented entry points, with no forking and no internal imports.

**Design refs:** § Writing your own attribute (all three examples — they are this phase's acceptance spec), § Server-side changes 3 (tiers 2 and 3, and their six requirements), § Open questions 3, § Open questions 5 (the correction).

**Audit** *(do first)*
- [ ] Confirm the package's `main` / `types` / `browser` entries actually expose the registry to both an ESM `import` and an IIFE global consumer. If `dist/livetemplate-client.js` doesn't re-export it, that's added work.
- [ ] Check whether the IIFE `--global-name=LiveTemplateClient` shape lets a third-party script call `registerAttribute` without reaching into internals.
- [ ] Confirm the delegation asymmetry still holds: `DELEGATED_EVENT_TYPES` closed (17 entries) vs `isDOMEventTrigger()` being a denylist. If `lvt-on:` has since gained dynamic event types, `delegatedEvents` is unnecessary — drop it rather than adding a redundant knob.
- [ ] Enumerate what tinkerdown's `vocabulary.go` would need to admit an app-defined attribute — is an extension hook viable, or does it need a config surface?

**Implementation**
- [ ] Export `AttributeHandler` (both variants), `SetupContext`, `ElementContext`, `SendFn` as public TS types
- [ ] `registerAttribute` reachable as a named export *and* on the global; the global's name documented as API
- [ ] **`delegatedEvents`**: a handler may declare extra DOM event types; the delegator adds them to its document-level listener set at registration. Closes the `lvt-on:` / `lvt-el:` asymmetry (§ Scope boundaries) — no new routing, no second delegator
- [ ] **Semver commitment** for the registry contract, written down: what may change in a minor, what forces a major, how a deprecation is announced
- [ ] Authoring guide: the three tiers, the load-order rules from § 2, category selection, the sweep-and-rearm discipline and its three failure modes, namespace guidance (avoid `lvt-fx:`/`lvt-el:`/`lvt-form:`), and the SRI + trust note for tier 2
- [ ] **A runnable example app in `../docs/examples/`** covering all three shapes from § Writing your own attribute: visual-only (`lvt-x:copy`), server-dispatching (`lvt-x:rating` + its Controller action), and the stateful sweep-and-rearm pattern. Feature work ships an example, not just an interface
- [ ] Promote § Writing your own attribute into the docs authoring guide — the worked examples in this plan are its spec, so they must compile against the shipped types rather than drift into pseudocode
- [ ] tinkerdown allowlist extension mechanism (or a written decision to defer it, with the consequence stated)

**Acceptance criteria**
- **Simplify:** `/simplify` over the diff.
- **Unit:** each of the three worked examples, copied verbatim from § Writing your own attribute, compiles against the built `.d.ts` and passes a jsdom test — not against internal source paths. If an example doesn't compile, the interface is wrong, not the example.
- **Integration:** the docs example builds and runs against the published client (GOWORK=off in worktrees).
- **E2E:** chromedp against the docs example — a tier-3 handler registered from an inline `<script>` after the client tag takes effect on the *current* render (proving late-registration catch-up), and again on the next render. Plus: a custom element emitting `my-widget-change` reaches a server action via `lvt-on:my-widget-change` once its handler declares `delegatedEvents`. Plus the `lvt-x:rating` round trip: click dispatches, the Controller action runs, the re-render lands. Capture console + server stderr + WS frames.

**Learn** — as above. **Also expand Phase 6's outline into full detail if M3 is still wanted.**

---

#### Phase 4 (M2) — Extraction: relocate prereview's handlers (~1 session)

**Goal at end:** the 10 prereview-only handlers live in prereview, built and served by prereview; the core bundle is measurably smaller; and prereview's own experience of doing it is the verdict on whether the plugin API is finished.

**Design refs:** § Core vs extras (census + criterion), § Migration table (extras rows — `extras` means *leaves this repo*), § Server-side changes 3 (why tier 1 is withdrawn), § Writing your own attribute (prereview is now an instance of this).

**Audit** *(do first)*
- [ ] **Re-run the usage census** — it was taken 2026-07-25 against one workspace's checkouts and is the whole basis for what moves. Anything that gained a consumer since then stays in core.
- [ ] **Confirm `handleIframeAutoHeightDirectives` is dead** and delete rather than relocate. If it has acquired a consumer, it moves with the others.
- [ ] Map the shared helpers in `dom/directives.ts` (geometry, hit-testing, `regionRectFromBox`, `lineRangeFrom*`). Anything used by *both* a core and a relocating handler is the real decision in this phase: duplicate it into prereview, or keep a copy in core as a genuinely general primitive. Do not export it from core purely so prereview can import it — that recreates the coupling this phase exists to remove.
- [ ] **Cost check on prereview's toolchain.** It has no `package.json`, `tsconfig.json`, or any `.ts`/`.js` source today — only a vendored bundle synced by `make sync-client`. Scope the setup before moving code. **If it materially exceeds a day, stop and take option (B)** (second entry point in the client repo, no server promotion) rather than half-relocating.

**Implementation**
- [ ] Delete `handleIframeAutoHeightDirectives` + its teardown + tests
- [ ] prereview: `package.json` + `tsconfig.json` + esbuild + jest; `make build-handlers` producing its own bundle; invert `sync-client` so prereview builds rather than only copies
- [ ] Move the 10 handlers **and their tests** into prereview, rewritten as `AttributeHandler` objects registering through the public API — no internal imports from `@livetemplate/client`
- [ ] prereview: `go:embed` the handler bundle beside the client bundle; add its `<script>` tag to `page.tmpl` after the existing `/livetemplate-client.js` tag
- [ ] Delete the relocated handlers from the client repo
- [ ] **Measure** core bundle before vs after; record the number in this plan
- [ ] **Gate:** if the saving doesn't justify the disruption, stop and keep the handlers in core — that outcome is a success for the phase, not a failure.
      **Threshold, set now rather than at the moment of maximum sunk cost:** relocation has to remove **≥ 25 KB minified** (~18% of the 141 KB baseline) from the core bundle. ~2,400 of 3,429 lines in `dom/directives.ts` should clear this comfortably; if it doesn't, that itself is the surprising finding and belongs in Learn.

**Acceptance criteria**
- **Simplify:** `/simplify` over the diff.
- **Unit:** every moved handler's tests pass from prereview; the client repo has no remaining reference to any relocated handler; **prereview's handlers compile against the published `.d.ts` only** — an internal import is a Phase 3 bug, not a prereview workaround.
- **Integration:** client builds smaller; prereview builds and embeds both bundles; `go test ./...` green in both repos.
- **E2E:** prereview's chromedp suite — area-select, text-select, region-select, viewport-report all still work through the public registration path. Plus: a build with prereview's handler bundle omitted produces the Phase 2 warning and no crash.

**Learn** — as above, **plus the API verdict**: list every place prereview needed something the public interface didn't offer. Each entry is a Phase 3 defect to fix before external users hit it. An empty list is the strongest possible evidence the registry is done.

---

#### Phase 5 (M2) — Downstream lockstep release (~1 session)

**Goal at end:** the vocabulary split and the wired-action extension land, and every downstream repo is consistent in one coordinated release.

**Design refs:** § Server-side changes 4, § Non-goals (blast radius), § Affected files (downstream table).

**Audit** *(do first)*
- [ ] Confirm the blast radius is still exactly prereview + tinkerdown — re-grep rather than trusting this plan's census.
- [ ] Confirm no docs example acquired an extras attribute since the census (zero did as of 2026-07-26).

**Implementation**
- [ ] Extend wired-action extraction to custom `send()` action names (§ Server-side changes 4)
- [ ] tinkerdown: split `vocabulary.go` into core vocabulary + app-extensible vocabulary; update `docs/reference/lvt-attributes.md` to describe the relocated attributes as prereview's, not the framework's
- [ ] Release core → client → downstreams, in that order, via `release.sh`

**Acceptance criteria**
- **Simplify:** `/simplify` over the diff.
- **Unit:** wired-action extraction picks up a custom action name.
- **Integration:** prereview's suite green against the released client; tinkerdown's vocabulary tests green.
- **E2E:** prereview's chromedp suite on the released artifacts, not local builds.

**Learn** — as above.

---

#### Phase 6 (M3) — Per-element hooks *(deferred — outline only)*

Demand-gated; expand into a full phase block only if M2 lands and per-element hooks are actually wanted (§ Open questions 2).

Sections to design at kickoff: instance lifecycle (mounted / updated / destroyed) against morphdom's callbacks; `lvt-hook="name"` resolution; `pushEvent` / `handleEvent` for server ↔ hook communication (§ Open questions 4); and how instance-per-element interacts with the singleton registry rather than replacing it.

Open questions to resolve then: does a hook instance survive a morphdom update of its element, or is it destroyed and recreated? What happens to a hook whose element is inside a `lvt-ignore` subtree?

## Non-goals

- **Server-side attribute *interpretation*.** `lvt-*` attributes stay opaque HTML in the Statics array. The server never acts on one, never changes rendering because of one, and never needs to understand what a custom attribute *does*.

  The Phase 2 attribute census (§ Server-side changes 1) is not a violation of this and the distinction is worth pinning: it collects attribute **names** for a developer warning and throws away everything else. Nothing downstream branches on the result — the attributes render byte-identically whether or not the census ran. This is the same posture as the existing `extractWiredActionNames`, which already regex-scans statics for `lvt-on:` names to power a dev-time collision warning without the server ever interpreting the attribute. Reading names for diagnostics ≠ parsing attributes for behavior.
- **Custom wire protocol messages.** Custom attributes communicate through the existing tree update + action message protocol. No custom WebSocket message types.
- **Breaking existing attribute *syntax*.** Every `lvt-*` attribute keeps its spelling, its semantics, and its server-side treatment. No template anywhere needs an edit to the attribute itself.

**Not a non-goal — extraction is a breaking change, and pretending otherwise would hide the cost.** "The registry is additive" is true of M1 and false of M2: once a handler leaves the core bundle, an app that uses its attribute without loading whatever now provides it gets a silently inert attribute. Phase 2's attribute-census warning (§ Server-side changes 1) is what makes that break loud instead of quiet — which is the main reason it ships in M1 rather than after the extraction it exists to cover.

The cost is bounded and enumerable, which is the argument for paying it now rather than deferring:

- **prereview** — the only app with template usage of the departing attributes, and now their owner. Cost: a JS toolchain it has never had (~a day), plus the handlers and tests moving in. Priced and justified in § Server-side changes 3; this is the largest single cost in the plan and the one most worth re-checking before Phase 4 commits.
- **tinkerdown** — `vocabulary.go` sanitizer allowlist and the attribute reference docs enumerate the vocabulary without using it. Fix: split into core vocabulary + app-extensible vocabulary.
- **Everyone else** — unaffected, and gets a smaller bundle.

LiveTemplate is alpha with no external users, so this lands as a coordinated same-release change across the three repos, not as a deprecation cycle or a compatibility shim.
