# WASM Build Target — Design & Implementation Specification

**Status:** Draft — work in progress; supersedes [#440](https://github.com/livetemplate/livetemplate/issues/440). Devbox-side validation (Phase V) complete; iPhone measurement and reviewer signoff pending. Continued iteration expected before this moves to Proposed.

This specification is self-contained: it describes the target design and the implementation plan against the current codebase. It requires no knowledge of prior design iterations. The earlier proposal it supersedes and why is summarized in [§8 Why not (alternatives rejected)](#8-why-not-alternatives-rejected); the validation prototype that justified this direction is in [§5 Validation results (Phase V)](#5-validation-results-phase-v).

## 1. Context

LiveTemplate today is a Go web framework: the app developer writes a controller + state + templates, compiles to a server binary, and serves HTTP. The framework's value proposition — small reactive trees, automatic diffing, transport-blind dispatch — is tied to a server that holds session state in memory and runs Mount/actions/render on every request. Three problems motivate a second build target:

1. **Server-side processing is a hard limit for "your data, your device" apps.** Apps where customer data is the product (personal checklists, private notes, scratch-pad calculators, family-coordination tools) want a credible privacy claim: "we do not run your data through our servers." Under GDPR Art. 4(2), transient in-memory holding counts as processing — so any server-rendered app is, by construction, a data processor for the duration of every session. The framework cannot reduce that surface to zero without ceasing to run on a server. The [browser-resident-session-state proposal (#440)](https://github.com/livetemplate/livetemplate/issues/440) attempted to address this by routing the persist-tag pipeline through IndexedDB on the client; it was withdrawn after analysis showed it still leaves state transiting and being processed in server memory during every action, so the marketing claim outruns the mechanism (see [§8.1](#81-why-not-the-440-clientsessionstore-design)).

2. **Server-side dev experience is a tax for apps that don't need a server.** Today, even an app whose entire model is a single user's local data needs a Fly machine, a hosting bill, a domain, and operational ownership — because livetemplate's `Handle()` returns an `http.Handler`. Apps that are "single-user, single-device" by design (the dominant Personal-mode shape — see checklistkit M1) pay the SaaS cost for none of the SaaS benefit.

3. **Offline support is currently impossible.** Any livetemplate app stops working when the connection drops. For some app shapes (collaboration, real-time data) that is correct; for others (offline-first PWA-style use) it is a hard limit the framework cannot ease without an architectural change.

**The design:** a second build target — `lvt build wasm` — that compiles the dev's existing controller + state + templates to `GOOS=js GOARCH=wasm` and produces a static directory. The "server" runs in the user's browser. The same Go codebase ships to either target — server mode (today) or WASM mode (new) — without a rewrite.

**The non-goals** (load-bearing — these define the boundary):

- **A new programming model.** The dev writes the same controller, state, templates, and persist tags as today. The framework's public surface gains nothing user-facing.
- **A drop-in replacement for hosted/collab apps.** WASM mode is for Personal-mode shapes: single user, single device, no real-time peers, no MCP, no REST API. Hosted apps (auth, multi-user, Drive/S3 backends) continue to use server mode unchanged. Both targets ship from one codebase.
- **An offline-first SPA framework.** While true offline falls out of WASM as a side-effect, that is a beneficial consequence, not a design driver. Apps that need collaboration use server mode; apps that need WASM mode get offline for free.

## 2. Design philosophy

**Same controller, same state, same templates — different build target.** The dev's existing Go code is the source of truth for both modes. The difference is who runs it: Fly machine (server mode) or the user's browser (WASM mode). No new public APIs on the server. No new public APIs on the client. The framework's `Template`, `Context`, `AsState`, `WithUpload`, `lvt:"persist"` etc. all work identically.

This is the **Phoenix LiveView meets Elm in WASM** model — but unlike Elm, you don't rewrite in a new language. Unlike LiveView, you don't always need a server. Closer precedent: **htmx + hyperscript** (HTML-driven, but with a fork point for client-side execution); **Blazor WebAssembly** (the .NET equivalent of "compile the framework to the browser"); and **Phoenix's eventual WASM target experiments**, none of which have shipped in production. This proposal is the version livetemplate can ship today because Go's `GOOS=js GOARCH=wasm` is mature stdlib (`html/template` compiles cleanly — verified in Phase V) and the framework's transport layer is already abstracted enough to gain a third dispatch path without disturbing the existing two.

**The build pipeline does the work, not the dev.** This proposal does NOT introduce a `livetemplate.RunWASM(controller, state)` API or require the dev to write a `cmd/wasm/main.go` boilerplate. Instead, `lvt build wasm` parses the dev's existing `main.go` AST to find the `tmpl.Handle(controller, AsState(state))` call, extracts the types, and synthesizes a build-tagged `main_wasm.go` that wires the same types into a framework-internal entrypoint. The dev's surface is unchanged.

**Build-tag separation, not runtime branching.** Where the framework needs different behavior in WASM mode (persist tags → IndexedDB instead of `SessionStore`; uploads → OPFS instead of multipart-to-server), the divergence happens at the **build-tag level**: paired files like `state_persist.go` (`//go:build !js`) and `state_persist_wasm.go` (`//go:build js && wasm`). The WASM binary literally does not contain the `SessionStore`/HTTP-upload code; the server binary literally does not contain the `syscall/js`/IndexedDB code. This is the same pattern the Go stdlib uses for `os` and `net` on different platforms — well-trodden, mechanically inspectable.

**Scope: controller + state + templates apps.** This proposal targets the dominant livetemplate shape — apps with a `tmpl.Handle(controller, AsState(state))` call where business logic lives in controller methods. Document-driven apps (notably **tinkerdown**, whose page is markdown + YAML frontmatter + nine declarative source types — SQLite, PostgreSQL, REST, JSON, CSV, exec/shell, markdown, WASM modules, computed — and where the framework owns all execution via a `Store` interface, not a controller) are **out of scope**: their value proposition is the server-side I/O the WASM sandbox cannot host (subprocess execution for `exec` sources, PostgreSQL drivers for `pg` sources, credentialed outbound REST). A separate design suited to that shape is sketched in [§9 Future companion: tinkerdown-static-export](#9-future-companion-tinkerdown-static-export).

## 3. At a glance

The dev's app — unchanged regardless of build target:

```go
// app/controller.go — same code for server mode and WASM mode.
type CounterState struct {
    Count int `json:"count" lvt:"persist"`
}

type CounterController struct{}

func (c *CounterController) Mount(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
    return s, nil
}

func (c *CounterController) Increment(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
    s.Count++
    return s, nil
}
```

The dev's `main.go` — unchanged regardless of build target:

```go
// main.go — produces a server binary today; left alone by lvt build wasm.
package main

func main() {
    tmpl := livetemplate.Must(livetemplate.New("counter"))
    http.Handle("/", tmpl.Handle(&app.CounterController{}, livetemplate.AsState(&app.CounterState{})))
    http.ListenAndServe(":8080", nil)
}
```

The two build invocations:

```bash
# Server mode (today) — produces a Fly-deployable binary.
go build -o myapp .

# WASM mode (new) — produces a CDN-deployable static directory.
lvt build wasm .
# →  dist/index.html         (the static shell with prerendered initial HTML)
# →  dist/myapp.wasm         (the dev's code + livetemplate + Go runtime, ~16 MiB raw, ~3 MiB brotli)
# →  dist/wasm_exec.js       (Go's standard WASM runtime support)
# →  dist/livetemplate-client-wasm.browser.js  (the WASM-aware client lib variant)
# →  dist/lvt-opfs-sw.js     (service worker that serves OPFS-stored files at /lvt/opfs-blob/<id>)
```

The browser experience (WASM mode):

1. User loads `index.html`. The shell contains a server-prerendered initial render — the wrapper div with zero-value state already rendered inside, no flash of blank.
2. The WASM-aware client lib boots (existing `autoInit()` from `livetemplate-client.ts` in the [@livetemplate/client](https://github.com/livetemplate/client) repo — unchanged). The existing 3px shimmer bar (`dom/loading-indicator.ts` in the same repo) shows progress as the WASM blob streams in (new `setProgress(fraction)` mode).
3. Once instantiated, WASM hydrates state from IndexedDB and runs the dev's `Mount`. The diff against the prerendered initial HTML is usually empty on **first visit** (zero-value state ≈ prerendered HTML) — no flicker. On **return visit** (persisted state diverges from zero-value), the diff is non-trivial: the user sees the zero-value HTML briefly before WASM patches it to the persisted state. This is a known trade-off — prerender is a first-visit win, return-visit-neutral-to-negative. Apps where return visits dominate can opt out via `lvt build wasm --no-prerender` (the loading bar covers the gap; the dist `index.html` is just the empty wrapper). See [§4.1 step 6](#41-the-lvt-build-wasm-pipeline) for the toggle.
4. User clicks `Increment`. The click handler dispatches `wasm.__lvtDispatch("Increment", null)` instead of `fetch(POST)` or `ws.send(...)`. WASM runs the dev's `Increment` method, returns a tree diff. The client applies the diff via the existing `client/state/tree-renderer.ts` morphdom path — unchanged.
5. The mutated state is written through to IndexedDB (the `lvt:"persist"` field). Persists across reloads, across tabs (within the origin), across days. The "server" never saw it because there is no server.

## 4. Detailed design

> **Tentative decisions in this section, surfaced for reviewer pushback before Phase 1 starts:** the AST-parse approach to entrypoint discovery (§4.1 step 1), the `<meta name="lvt-target">` boot-detection mechanism (§4.2), the upload-prefix flag default and service-worker scope (§4.5). The five locked-in design decisions are §4.1 step 2 (prerender), §4.2.1 (progress fill on the existing shimmer bar), §4.3 (build-tag auto-swap), §4.4 (hard-error vs auto-swap policy), and Phase 1 → server-mode dev iteration. If any tentative call feels wrong, flag in review and we'll reopen.

### 4.1 The `lvt build wasm` pipeline

`lvt build wasm <pkg>` runs seven steps:

1. **Parse `main.go` AST** to discover the single `tmpl.Handle(controllerExpr, asStateExpr)` call. Fail with a helpful error if there are zero or more than one (the latter is a multi-page app — see [§7.1](#71-multi-page-apps)). Provide a `--entry=<pkg.Var>` flag for the multi-Handle case as a v0 escape hatch. The expressions extracted (e.g. `&app.CounterController{}` and `livetemplate.AsState(&app.CounterState{})`) are emitted verbatim into the generated wrapper, so the dev's existing types and constructors are honored.

   **Brittle by design (acknowledged):** the AST scan walks the `main()` function body and finds `Handle()` calls *that appear there directly*. Common patterns it does NOT find: a factory like `mux.Handle("/", buildHandler())` where `buildHandler()` calls `tmpl.Handle(...)` internally; a slice of route registrations built dynamically; conditional registration behind an `if`. For these, the error message is actionable: `"no tmpl.Handle(...) call found in main(). If your handler is built in a helper, pass --entry=<pkg.Var> pointing at the controller + state."` Devs with helper-built handlers add a single flag; the dev's source stays unchanged.

2. **Synthesize `main_wasm.go`** in a build-cache scratch dir (not in the dev's source tree) with build tag `//go:build js && wasm`:

   ```go
   //go:build js && wasm
   package main

   import (
       lvtwasm "github.com/livetemplate/livetemplate/internal/wasm"
       livetemplate "github.com/livetemplate/livetemplate"
       app "myapp/app"  // resolved from the dev's main.go import block
   )

   func main() {
       lvtwasm.Run(&app.CounterController{}, livetemplate.AsState(&app.CounterState{}))
   }
   ```

   **Import-path resolution.** The AST extractor reads the dev's `main.go` import block (via `golang.org/x/tools/go/packages`, which wraps `go/ast` + `go/types` and handles workspace mode, build constraints, and import resolution the way `gopls` does — raw `go/types` requires manual configuration of all of these and is brittle in practice) and re-emits the resolved import path backing each referenced package alias (e.g., `app` → `"myapp/app"`) in `main_wasm.go` — not just the expression text. Failure to resolve produces a build error with the unresolved package name pointed at, before any Go compilation runs.

   **Build-tag rewrite.** `lvt build wasm` also rewrites the scratch-dir copy of the dev's `main.go` to prepend `//go:build !js` if it's missing — without that constraint, two `main()` functions in the same package would be a compile error. The dev's source tree is not modified; both files coexist only in the scratch dir, and the toolchain picks one based on `GOOS=js`.

3. **Synthesize an adjacent prerender binary** at the same scratch dir. This is a small server-mode Go program that imports the same package, instantiates the same controller + state, calls `tmpl.Execute(&buf, state)` once, and writes the resulting HTML to stdout. Builds with the default Go toolchain (no `GOOS=js`).

4. **`go build` the WASM binary.** Output: `dist/<appname>.wasm`. With `-trimpath -ldflags="-s -w"` for size.

5. **`go build` the prerender binary.** Output: build-cache scratch dir.

6. **Run the prerender binary.** Capture stdout into the static `index.html` shell. The shell wraps the prerendered HTML in the standard `<div data-lvt-id="lvt-XXX" data-lvt-loading="true">...</div>` pattern produced today by `InjectWrapperDiv` (grep anchor, from the livetemplate repo root: `grep -n "func InjectWrapperDiv" internal/build/wrapper.go`), then injects:
   - `<meta name="lvt-target" content="wasm" data-wasm-url="<appname>.wasm" data-wasm-size="<byteSize>">` — signals the client lib that it's in WASM mode and tells it where the blob is.
   - `<script src="livetemplate-client-wasm.browser.js"></script>` — the WASM-aware client bundle (vs. the regular `livetemplate-client.browser.js` in server mode).
   - `<script src="lvt-opfs-sw.js" type="module">` — registers the OPFS service worker (see [§4.5](#45-service-worker-for-opfs)).

7. **Pre-compress and assemble `dist/`.** Run `gzip -9` and (if available) `brotli -q 11` on `*.wasm` and `*.js`. Copy `wasm_exec.js` from `$(go env GOROOT)/lib/wasm/` (16.6 KiB raw / 4.3 KiB gzipped — see §5's size table for measurement context). The output directory is a complete static site — drop into Cloudflare Pages, S3+CloudFront, GitHub Pages, or any CDN.

### 4.2 The client variant

A second build target in `client/package.json`:

```json
{
  "scripts": {
    "build:browser": "esbuild livetemplate-client.ts --bundle --format=iife --outfile=dist/livetemplate-client.browser.js …",
    "build:wasm":    "esbuild livetemplate-client.ts --bundle --format=iife --outfile=dist/livetemplate-client-wasm.browser.js --define:LVT_WASM_BUILD=true …"
  }
}
```

`LVT_WASM_BUILD` is an esbuild define that the bundler folds into a constant — dead-code elimination removes the server-mode dispatch paths from the wasm bundle and vice versa.

**Boot detection.** The existing `autoInit()` static method on `LiveTemplateClient` (grep anchor, from the [@livetemplate/client](https://github.com/livetemplate/client) repo root: `grep -n "static autoInit" livetemplate-client.ts`) is unchanged. It queries `document.querySelector("[data-lvt-id]")` exactly as today. The WASM bundle, on boot, additionally reads `<meta name="lvt-target" content="wasm">`. If present, it routes to a new `bootWASM()` method instead of the existing `connect()` (which negotiates WS vs HTTP):

```typescript
// Pseudocode — actual implementation will mirror existing connect() shape.
// Use WebAssembly.instantiateStreaming so compile overlaps with download
// (saves ~1-2s on the typical 16 MiB blob over 4G vs. fetch-then-instantiate).
// A ReadableStream tee keeps the byte-progress callback feeding the bar.
async bootWASM(): Promise<void> {
    const meta = document.querySelector('meta[name="lvt-target"][content="wasm"]');
    const wrapperID = this.wrapperElement.getAttribute("data-lvt-id");  // e.g., "lvt-a1b2c3d4..."
    const wasmUrl = meta.getAttribute("data-wasm-url");

    // The service worker must be active before any OPFS-blob URL can be
    // served (§4.5). Wait here so first-paint hydration can resolve them.
    await navigator.serviceWorker.ready;

    this.loadingIndicator.show();
    this.loadingIndicator.setProgress(0);  // NEW mode — see §4.2.1

    // Progress is tracked against the COMPRESSED response stream — Content-Length
    // is the on-wire byte count, which is what "bytes arrived" actually
    // corresponds to. Many CDNs (notably Cloudflare) strip Content-Length for
    // brotli/gzip responses, which means determinate progress is the exception
    // in practice. The fallback (indeterminate shimmer) is what server-mode
    // shows today — no regression, just no improvement over baseline.
    const response = await fetch(wasmUrl);
    const contentLength = parseInt(response.headers.get("Content-Length") || "", 10);
    let wasmResponse: Response;
    if (Number.isFinite(contentLength) && contentLength > 0) {
        // teeWithProgress MUST use response.body.tee() to split into two
        // ReadableStream branches: one for the progress counter, one wrapped
        // back into a new Response for instantiateStreaming. The original
        // `response` is consumed by tee() and can't be passed on directly.
        wasmResponse = teeWithProgress(response, contentLength, (fraction) => {
            this.loadingIndicator.setProgress(fraction);
        });
    } else {
        this.loadingIndicator.transitionToIndeterminate();  // unknown size → shimmer
        wasmResponse = response;
    }

    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(wasmResponse, go.importObject);
    this.loadingIndicator.setProgress(1.0);
    this.loadingIndicator.transitionToIndeterminate();  // shimmer during first render

    // go.run() returns a Promise that resolves only when main() returns; we
    // do NOT await it here because WASM main() blocks forever (select{}).
    // We DO need to wait for the WASM-side init to expose __lvtDispatch_<id>
    // on window — go.run() returning doesn't mean main()'s init code has run
    // yet. The clean signal: Go's main() dispatches a CustomEvent
    // `lvt-wasm-ready-${wrapperID}` on window AFTER registering the dispatch
    // global. JS subscribes via a Promise; the listener pre-checks the global
    // in case the event already fired (fast machine / pre-warm cache).
    const runPromise = go.run(result.instance);  // do not await
    await new Promise<void>((resolve) => {
        if ((window as any)[`__lvtDispatch_${wrapperID}`]) { resolve(); return; }
        window.addEventListener(`lvt-wasm-ready-${wrapperID}`, () => resolve(), { once: true });
    });

    // WASM exposes window.__lvtDispatch_<wrapperID>(messageJSON: string) →
    // diffJSON, set during its main() init. messageJSON is the same wire
    // format the WS/HTTP paths emit today (e.g. {"action":"Increment",
    // "data":{...}}), so the WASM side parses out action+data once.
    // Namespacing by wrapperID prevents collisions when multiple WASM
    // bundles run on the same page (test harnesses, the multi-page bundle
    // case sketched in §7.1, per-route bundles via --entry).
    this.wasmDispatch = (window as any)[`__lvtDispatch_${wrapperID}`];
    this.loadingIndicator.hide();

    // runPromise rejecting (a Go panic in main()) is the §4.6 "Mount/init
    // panic" failure mode — surface it to the dev console; can't recover.
    runPromise.catch(err => console.error("[livetemplate-wasm] crashed:", err));
}
```

**On `data-wasm-size`:** the emitted meta attr carries the uncompressed blob size for §5's size table and developer observability only — *not* for progress tracking. The pseudocode above never reads it. Progress is always derived from `response.headers.get("Content-Length")` (the on-wire byte count); if that's missing, the bar falls back to the existing indeterminate shimmer rather than to a wrong denominator.

**The third dispatch path.** Today, the `send()` method on `LiveTemplateClient` (grep anchor, from the [@livetemplate/client](https://github.com/livetemplate/client) repo root: `grep -n "  send(message:" livetemplate-client.ts`) branches on `useHTTP` and `webSocketManager.getReadyState()`. The WASM bundle adds a branch (`this.wasmDispatch` was bound to `window.__lvtDispatch_<wrapperID>` during `bootWASM`):

```typescript
send(message: any): void {
    if (this.wasmDispatch) {
        const diffJSON = this.wasmDispatch(JSON.stringify(message));
        this.applyDiff(JSON.parse(diffJSON));
        return;
    }
    // ... existing WS / HTTP branches unchanged ...
}
```

The diff JSON returned by `__lvtDispatch` is the same wire format the WS and HTTP paths produce today. The existing `state/tree-renderer.ts` morphdom logic (in the [@livetemplate/client](https://github.com/livetemplate/client) repo) applies it — unchanged.

**Main-thread constraint (load-bearing for app authors).** Go WASM runs on the JS main thread; `wasm.__lvtDispatch` is synchronous from JS's perspective. Action handlers should return well under one frame (~16 ms) to avoid UI jank. **Compute-heavy work — large list sorts, complex regex over big bodies, image processing — is not suited to WASM-mode action handlers**; it would freeze the UI for the handler's duration. The framework does not move dispatch to a Worker in v0 because the worker bridge has its own cost and complexity; surfaced as a future option in [§7.8](#78-worker-isolation-for-cpu-heavy-action-handlers). Server mode is unaffected (handlers run in a goroutine, not on the UI thread).

#### 4.2.1 Loading-indicator extension

The existing `LoadingIndicator` class (`client/dom/loading-indicator.ts`) provides:

```typescript
class LoadingIndicator {
    show(): void
    hide(): void
}
```

The WASM-aware variant extends it with one method + one internal mode flag:

```typescript
class LoadingIndicator {
    show(): void
    hide(): void
    setProgress(fraction: number): void    // NEW: 0.0 to 1.0, switches the bar to determinate-fill mode
    transitionToIndeterminate(): void      // NEW: switches back to existing shimmer animation
}
```

The DOM element stays the same — same 3px `position:fixed; top:0; z-index:9999` bar. In determinate mode the inline CSS swaps `background: linear-gradient(...) [shimmer keyframes]` for `background: #3b82f6; transform: scaleX(${fraction}); transform-origin: left;`. Both modes share the bar element; no extra DOM. Server mode (today) doesn't call `setProgress` — behavior unchanged.

### 4.3 Persist + uploads (build-tag auto-swap)

The dev's `lvt:"persist"` tag and `WithUpload(Accept:..., MaxFileSize:...)` calls work identically in both modes. The divergence is entirely behind build tags inside the framework, never in user code.

**Persist pipeline.** The right architectural seam is *below* mount.go — the `SessionStore` interface, not the persist methods. `mount.go`'s `persistState(ctx, groupID, state)` (line 1568) and `restorePersistedState(ctx, groupID)` (line 1585) stay unchanged: they always call `ExtractPersistFields`/`InjectPersistFields` (methods on the internal `jsonState[T]` type in `state.go` — grep anchor from the livetemplate repo root: `grep -n "func.*ExtractPersistFields\|func.*InjectPersistFields" state.go`) and then delegate to the configured `SessionStore`. Only the **default `SessionStore` selection** and the **set of compiled-in stores** differ by build target:

```
session_stores.go               //go:build !js          (existing: Memory + Redis)
session_stores_wasm.go          //go:build js && wasm   (NEW: WASMSessionStore + IndexedDB)
template_default_store.go       //go:build !js          (existing: New() default = MemorySessionStore)
template_default_store_wasm.go  //go:build js && wasm   (NEW: New() default = WASMSessionStore)
```

The dev's `livetemplate.New("counter")` with no explicit `WithSessionStore` gets the right backend automatically per build target. Dev code that passes `WithSessionStore(NewMemorySessionStore(...))` etc. is rejected by [§4.4](#44-what-lvt-build-wasm-rejects-hard-errors)'s AST scan before any Go compilation runs. **No mount.go refactor needed** — the `SessionStore` interface is already the seam.

The IndexedDB schema: one database named `livetemplate`, one object store named `sessions`, key = `groupID` (always `"wasm"` in WASM mode since there's only one user), value = `[]byte` JSON.

**Sync-Go ↔ async-IndexedDB bridge — the load-bearing implementation detail.** `restoreFields` must complete synchronously within `Mount` (the framework's lifecycle is synchronous), but IndexedDB is entirely asynchronous. Two viable techniques exist:

- **(a) Channel-blocked goroutine.** Register a `js.FuncOf` callback for the IndexedDB promise resolution; block a Go channel until the callback fires. The parked goroutine allows the JS event loop to run and fire the callback. This is the pattern `wasm_exec.js` uses internally for `fetch`/`setTimeout` — battle-tested, adds one `js.FuncOf` allocation + one syscall round-trip per persist op.
- **(b) Native promise-await helper in `syscall/js`.** Hypothetically, a stdlib `Value.Await()` (or `Promise.Await`) would eliminate the manual channel wiring. **No such helper exists** as of the toolchain measured for this proposal (verified via `GOOS=js GOARCH=wasm go doc syscall/js` on 2026-05-25) — `syscall/js` exposes `FuncOf`, `Get`, `Set`, `Call`, `Invoke`, etc., but no `Await`. The package is also marked EXPERIMENTAL and exempt from the Go compatibility promise, so even if (b) lands in a future release the proposal can't depend on it.

**Phase 1 commits to (a).** This was the round-1 commitment; the proposal briefly flipped to (b) during review iteration based on a third-party suggestion that the helper existed, which turned out to be fictional. (a) is the actual mechanism livetemplate must use because it's the one the stdlib supports. The wrapper lives in a small helper `internal/wasm/promise.go` shared by `WASMSessionStore` and the `WASMFileStore` of [§4.3 uploads](#43-persist--uploads-build-tag-auto-swap); per-op cost is dominated by IndexedDB's own latency (5–50 ms), so the channel hop is in the noise.

**Upload pipeline.** Paired files inside `livetemplate/internal/upload/`:

```
handler.go         //go:build !js          (today's multipart-to-server flow)
handler_wasm.go    //go:build js && wasm   (NEW — file picker → OPFS write → metadata-only signal)
```

In WASM mode, the dev's `WithUpload("avatar", UploadConfig{Accept:..., MaxFileSize:...})` registers the same field, but the `<input type=file>` handler is wired to write bytes directly to OPFS at `opfs:/lvt-uploads/<file_id>` instead of streaming chunks to a server endpoint. The `UploadEntry` returned to the dev's handler has `TempPath = "opfs:<file_id>"` (sentinel). The dev's existing `ctx.GetCompletedUploads("avatar")` code works unchanged; new helpers (Phase 1):

```go
// Shape illustrative; Phase 1 picks methods-on-type vs package-level helpers
// based on the existing convention in internal/upload/. Today UploadEntry has
// zero methods (it's a pure data struct in internal/uploadtypes/types.go) and
// utilities live as package-level funcs like ValidateEntry; methods are an
// equally valid choice for a public type — either works at the call site.
func (e *UploadEntry) IsOPFS() bool      { return strings.HasPrefix(e.TempPath, "opfs:") }
func (e *UploadEntry) OPFSFileID() string { return strings.TrimPrefix(e.TempPath, "opfs:") }
```

Handlers that move bytes to permanent storage need a small guard — `if !entry.IsOPFS() { ... os.Open(entry.TempPath) ... }` — to skip filesystem ops in WASM mode. Handlers that only record metadata (file_id, size, mime) need no change.

### 4.4 What `lvt build wasm` rejects (hard errors)

The framework features that depend on multi-user, network, or server-only primitives have no clean WASM equivalent. `lvt build wasm` scans the dev's `main.go` AST and refuses to build if any of these are configured. The errors name the offending option and suggest the fix.

| Option | Why it fails in WASM mode | Suggested fix |
|---|---|---|
| `WithSessionStore(NewRedisSessionStore(...))` (or any non-default `SessionStore`) | Redis/Postgres/etc. unreachable from browser | Remove — IndexedDB is automatic in WASM mode |
| `WithAuthenticator(...)` | Single-user single-device — there's no auth concept | Remove — Personal mode has no auth |
| `WithUpload(..., External: <Presigner>)` | S3/GCS presigning requires server-side keys | Remove `External:` — OPFS is automatic in WASM mode |
| `WithPubSubBroadcaster(...)` | No cross-instance fan-out possible (no peers) | Remove — Subscribe/Publish are no-ops in WASM (see [§7.7](#77-broadcastchannel-for-multi-tab-sync)) |
| `WithMaxConnections(...)` / `WithMaxConnectionsPerGroup(...)` | Single-process | Remove |
| `WithMessageRateLimit(...)` / `WithMessageRateBurst(...)` | No untrusted clients | Remove |
| `WithAllowedOrigins(...)` / `WithCookieMaxAge(...)` | No HTTP semantics | Remove |
| `WithUpgrader(...)` / `WithWebSocketBufferSize(...)` | No WebSocket | Remove |

The build errors are emitted at the AST-scan step (step 1 in [§4.1](#41-the-lvt-build-wasm-pipeline)) — before any Go compilation runs — so they're fast and have full file:line context. Devs see them in their editor's build output the same way they'd see a Go compile error.

**Silent no-ops** (work as expected, but their effect is null in WASM mode): `WithLoadingDisabled` (honored — controls the existing loading indicator), `WithDevMode`, `WithTopicACL` / `WithOpenTopics` (no peers exist), `WithTrustForwardedHeaders` (no proxy), `WithProgressiveEnhancement` (no non-JS path applies).

### 4.5 Service worker for OPFS

The dev's templates today reference uploaded files like:

```html
<img src="/files/{{.AvatarID}}" alt="avatar">
<a href="/files/{{.DocID}}" download="passport.pdf">passport.pdf</a>
```

In WASM mode the bytes live in OPFS, not on a server. To keep the dev's templates working unchanged, `lvt build wasm` generates a service worker (`dist/lvt-opfs-sw.js`) and the static shell registers it. The service worker intercepts requests matching `/lvt/opfs-blob/<id>` (or the configurable equivalent — discussed below) and serves bytes from OPFS via the standard Fetch API.

The dev's templates can use either:

- **Direct OPFS URL** (zero changes if they use this convention): `<img src="/lvt/opfs-blob/{{.AvatarID}}">`.
- **Custom upload URL prefix** (configurable via build flag): `lvt build wasm --upload-prefix=/files` — then `/files/<id>` is intercepted instead.

Alternative considered and rejected: dev plumbs `URL.createObjectURL(blob)` in their templates. This forces the templates to diverge between server mode (where `/files/<id>` is a real HTTP endpoint) and WASM mode (where it's a blob URL). The whole point of this proposal is that the dev's source is unchanged across modes — so the service worker, with its slightly heavier lifecycle, is the right cost to pay.

Service worker lifecycle gotchas the proposal owns:

- **Scope.** The SW is registered at the site root (`scope: "/"`) so it can intercept any path the dev chose. The SW only handles `/lvt/opfs-blob/*` (or `--upload-prefix=...`); other requests pass through.
- **Update.** When `lvt build wasm` regenerates the SW (e.g., bumped upload prefix), the new bundle has a versioned filename; the static shell references the new name; the browser fetches and activates the new SW on next navigation.
- **First-load race.** If the user reloads immediately after first install, the SW may not be active yet for the OPFS-blob requests on that page. The shell defers OPFS-referencing renders until `navigator.serviceWorker.ready` resolves (added to the bootstrap sequence in [§4.2](#42-the-client-variant)).
- **Secure-context requirement.** Service workers register only over HTTPS or on `localhost`. Production CDN deploys satisfy this trivially; **local testing of the built `dist/` via a plain-HTTP server silently fails to register the SW**, which then breaks OPFS-blob URLs in a confusing way. `lvt build wasm` will print a one-line warning to the dev when it detects no `dist/` has been served over HTTPS, suggesting `caddy file-server` or similar.
- **File-ID unguessability.** OPFS-stored file IDs (the `<id>` in `/lvt/opfs-blob/<id>`) are UUID v4 generated via `crypto/rand` — opaque, cryptographically unpredictable. Any third-party script the page loads (analytics, embeds) cannot enumerate uploaded files by probing the namespace because the IDs are not derivable without already knowing them. This matches the existing server-mode `UploadEntry.ID` scheme, which is also `crypto/rand`-derived; the WASM path uses the same generator (no new format, no new randomness source).

### 4.6 Failure modes

WASM mode has two failure modes worth documenting up front; both differ from server mode and shape what the dev needs to defend against.

- **Panics in action handlers.** Server mode wraps action dispatch with `recover()` and surfaces panics as validation errors; the request fails but the connection survives. WASM mode wraps action dispatch the same way (the same dispatch code runs, with build-tag swaps only on persistence/upload), so action-handler panics also recover and surface to the user. **No regression** vs. server mode on this axis.
- **Panics in `Mount` or framework init.** A panic before the first action handler runs has nowhere to recover to — it crashes the WASM runtime, which from the JS side looks like a tab crash. The loading indicator stays visible; the page never hydrates. There is no automatic restart. The dev mitigation is the same as for any panic-on-init bug: surface in development (the `lvt build wasm --dev` build target prints stack traces to the JS console; production builds with `-ldflags="-s -w"` strip them) and fix at the source. **Document this as a known limitation; no framework workaround.**
- **OOM under memory pressure.** iOS Safari kills tabs above ~200 MB of total memory. The WASM heap counts toward this, and Go's GC is conservative about returning memory. Apps that allocate large per-frame data (image processing, big intermediate slices) can OOM the tab in a way that server-mode equivalents never would. The Phase V validation explicitly tests for this; production apps should keep state size in the single-digit MB range and avoid per-action allocations that don't get GC'd before the next user interaction.
- **IndexedDB unavailable (private browsing).** Safari in Private mode and some older browser configurations block or severely limit IndexedDB. A WASM app that depends on persisted state for initial render will silently start from zero-value on every load in those contexts — the app works, but persists nothing; state resets on reload. This is not a crash; it's surprising UX. Phase 1's `WASMSessionStore` surfaces the IndexedDB-unavailable case to the dev's `Mount` (e.g., via a context flag) so apps can show a "private browsing — state won't persist" banner if they care. Default behavior: silent zero-value, which matches the WASM-mode worst case of "no prior session".
- **Concurrent-tab silent data loss.** A WASM app open in two browser tabs is two independent state copies writing under the same `groupID = "wasm"` key. IndexedDB writes are last-write-wins with no coordination: if the user mutates in tab A and tab B at the same moment, one mutation is silently lost. This is true of any browser-storage-backed app (not unique to this design) but `lvt:"persist"` fields make it easy to depend on browser storage as if it were transactional. **v0 mitigation options** (app-level, not framework-provided): (1) use `navigator.locks.request("lvt-session")` in Mount to acquire an exclusive lock and refuse hydration in the second tab; (2) show a "this app supports one tab at a time" UI if the lock is busy; (3) accept the risk for apps where concurrent-tab usage is rare. **Framework fix** is BroadcastChannel-based sync in Phase 2.5 — see [§7.7](#77-broadcastchannel-for-multi-tab-sync) for the design sketch.

## 5. Validation results (Phase V)

A throwaway prototype built 2026-05-24 at `livetemplate/.worktrees/wasm-validation/` measures the deployment-cost shape. **All devbox thresholds met; iPhone measurement in flight as of writing.** Full results in `validation/README.md` in that worktree; this section will be updated with iPhone numbers before the proposal moves from Proposed → Accepted.

| Metric | Value | Threshold | Status |
|---|---|---|---|
| WASM uncompressed | 16.48 MiB | — | informational |
| WASM gzip -9 | 3.79 MiB | ≤ 4.0 MiB | ✅ (5% under) |
| WASM brotli -q11 (est.) | ~2.7 MiB | ≤ 4.0 MiB | ✅ (well under) |
| `wasm_exec.js` (Go stdlib) | 16.6 KiB raw / 4.3 KiB gzipped | — | measured 2026-05-25 against `$(go env GOROOT)/lib/wasm/wasm_exec.js` on toolchain 1.26.1 (go.mod floor: `go 1.26.0`) |
| `livetemplate-client-wasm.browser.js` (est.) | ~25 KiB minified, gzipped | — | similar to existing `livetemplate-client.browser.js`, +WASM-bridge code |
| `dist/index.html` (server-prerendered shell) | <10 KiB typical | — | depends on user template; counter ~3 KiB, 50-item dashboard ~30 KiB |
| Headless smoke (devbox localhost TTI) | 539 ms | informational | correctness check only — localhost has no network latency; the iPhone-over-4G row below is the actual UX benchmark |
| iPhone over 4G (TTI / idle mem / bg survival) | *in flight* | ≤ 5 s / ≤ 80 MB / survives backgrounding | user testing; results gate Proposed → Accepted, not Phase 0 sign-off |

The prototype uses livetemplate's **real** Build/Render pipeline via a small new `WithRawTemplate(text string)` option (added to `template.go` in the worktree, ~15 lines, all existing tests pass). **`WithRawTemplate` is a Phase 1 prerequisite** — the WASM build needs it to bypass `discovery.DiscoverTemplateFiles`, which reads from the filesystem at runtime (no filesystem in WASM). Whether it lands as a standalone preliminary PR or inside Phase 1's combined PR is a sequencing detail; the dependency is mandatory either way. **Apps using `//go:embed` (which bakes files into the binary at build time) do NOT need to migrate** — they can pass the embedded content via `WithRawTemplate(string(myEmbeddedFile))` (or even keep `WithParseFiles` if the embed presents a real-looking path through an `embed.FS`). Only apps relying on runtime filesystem discovery need the new option. The option has standalone value beyond WASM (any caller wanting to pass a template string without scaffolding), which makes the standalone-PR sequencing attractive but not required.

Key non-finding from validation: **the framework code compiles cleanly under `GOOS=js GOARCH=wasm` with no build-tag refactoring.** `net/http` is fully supported on `js/wasm` (uses `fetch` as the transport), so `mount.go` and `ws.go` compile fine — they just can't *listen*. The build-tag work in [§4.3](#43-persist--uploads-build-tag-auto-swap) is therefore minimal and targeted (paired files for persist + upload), not a sweeping refactor.

### 5.1 Browser support matrix

The "any CDN" claim is honestly qualified by the floor set by the most-recent browser feature in the stack. As of 2026-05-25:

| Feature | Chrome | Safari | Firefox | Used by |
|---|---|---|---|---|
| WebAssembly (basic) | 57+ (2017) | 11+ (2017) | 52+ (2017) | core |
| `WebAssembly.instantiateStreaming` | 61+ (2017) | 15+ (2021) | 58+ (2018) | [§4.2 boot](#42-the-client-variant) |
| Service Workers (over HTTPS) | 40+ (2015) | 11.1+ (2018) | 44+ (2016) | [§4.5 OPFS interception](#45-service-worker-for-opfs) |
| IndexedDB | 24+ (2012) | 10+ (2016) | 16+ (2012) | [§4.3 persistence](#43-persist--uploads-build-tag-auto-swap) |
| **OPFS** (Origin Private FS, async API) | **102+ (2022)** | **15.2+ (2021)** | **111+ (2023)** | [§4.3 uploads](#43-persist--uploads-build-tag-auto-swap) |
| `BroadcastChannel` (future, [§7.7](#77-broadcastchannel-for-multi-tab-sync)) | 54+ (2016) | 15.4+ (2022) | 38+ (2015) | future |

**Floor (v0):** Chrome 102+, Safari 15.2+, Firefox 111+ (driven by OPFS). All three are >3 years old at proposal time; >95% of global active browsers per [caniuse.com](https://caniuse.com/native-filesystem-api) are well past these versions. The remaining minority sees the page-load fail at OPFS init with a clear error (Phase 1 will emit a "your browser is too old, please update" fallback render rather than a blank page).

## 6. Implementation phases

| Phase | Duration | Deliverable |
|---|---|---|
| **V — Validation prototype** | done 2026-05-24 | iPhone smoke pending; sizes pass thresholds; framework compiles to WASM with no refactor needed; `WithRawTemplate` option verified |
| **0 — This proposal** | done with reviewer signoff | This document; close [#440](https://github.com/livetemplate/livetemplate/issues/440); open Phase 1 tracking issue |
| **1 — Framework implementation** | ~4 weeks | `livetemplate/internal/wasm/` package (`Run`, `syscall/js` dispatch shim, IndexedDB `WASMSessionStore`, OPFS `WASMFileStore`); paired build-tag files for store-default + upload; `LoadingIndicator.setProgress`; new client esbuild target for `livetemplate-client-wasm.browser.js`; **WASM E2E test harness** — existing chromedp infra hits a Go HTTP server, but service workers (§4.5) require a secure context, so this needs a local TLS server (caddy + mkcert or equivalent) loading a built `dist/` over HTTPS. Non-trivial scaffolding lift; budgeted in this phase |
| **2 — `lvt build wasm` CLI** | ~2 weeks | `lvt/commands/build_wasm.go`; AST parse of `main.go`; synthesize `main_wasm.go` + prerender binary; assemble `dist/`; service-worker generation; pre-compression |
| **3 — Docs + examples + deploy guide** | ~1 week | `examples/wasm-counter/` deployed to a CDN; `docs/guides/wasm-target.md`; caching/CDN guidance (Cache-Control, content-hashed filenames, service-worker patterns); **CSP guidance** (instantiating WASM requires `script-src 'wasm-unsafe-eval'` if a strict CSP is set — a late discovery on CSP-enforcing CDNs); READMEs for both repos updated |
| **3.5 — checklistkit M1 dogfood** | (consumer) | checklistkit Personal mode ships on WASM-livetemplate as the canonical real-app example |

Total framework + CLI work: ~7 weeks. Phases 1 and 2 are sequenceable in parallel by different contributors if available; Phase 2 only depends on Phase 1's stable internal `Run` signature, which can be agreed up-front.

## 7. Open questions

### 7.1 Multi-page apps

v0 supports a single `tmpl.Handle(...)` call discovered by AST. Apps with multiple Handle calls (router pattern: `mux.Handle("/", tmpl1.Handle(...))` + `mux.Handle("/admin", tmpl2.Handle(...))`) need a routing story. Sketch: bundle all pages into one WASM blob; client-side `window.location.pathname` selects which controller to instantiate. `lvt build wasm --entry=...` flag is the v0 escape hatch (build per-route bundles separately). Not blocking Phase 1; surface in Phase 2 design.

### 7.2 Hot-reload in WASM dev mode

`lvt build wasm` takes ~10–30 seconds on a non-trivial app. That cycle is too slow for save-and-reload iteration. The Phase 1 + 2 stance: **WASM only for production builds; dev iteration stays on `go run .` (server mode)**. This creates a parity gap — bugs that only appear in WASM mode (memory pressure, OPFS edge cases, `syscall/js` bridge quirks) only surface on production builds. Mitigations: a `lvt build wasm --dev` mode that skips minification/compression (rebuild ~3–5s); a "smoke before deploy" nudge.

The stretch fix is **two-tier WASM build**: the framework + Go runtime compiled to a stable `.wasm` blob (cached forever); the user's controller + state + templates compiled separately and linked at runtime via a `syscall/js` bridge. Rebuild on save = user-code-only, sub-second. Significant framework complexity (stable ABI for the bridge, linker integration). Defer to Phase 2 stretch goal; only build if Path 1's parity gap actually bites in practice.

### 7.3 SEO for non-Personal-mode use cases

Personal-mode apps are SEO-irrelevant (they're behind a sign-in or user-owned anyway). For other shapes (a marketing page that wants the small-payload SPA model), WASM mode is SEO-hostile — Googlebot doesn't execute 16 MiB of WASM. The build-time prerender ([§4.1](#41-the-lvt-build-wasm-pipeline) step 6) only renders the *initial* state once; per-route prerendering ("static prerender per route") is the SEO mode and is a separate proposal. Out of scope here.

### 7.4 TinyGo revisit

TinyGo's `reflect` package can't compile `html/template` as of TinyGo 0.41 (verified 2026-05-24). If a future TinyGo release closes the gap, the WASM blob shrinks dramatically (10–15 MiB → 1–2 MiB). Worth a periodic re-check; not a blocker now. The framework code doesn't need to change to benefit when TinyGo catches up.

### 7.5 IndexedDB quota policy

iOS Safari caps IndexedDB at ~50% of free disk space and aggressively evicts inactive sites after 7 days. The framework provides:
- 5 MiB **soft warning** by default (configurable at build time via `LVT_WASM_PERSIST_WARN_MB=...`); console warning when state size crosses the cap.
- Hard cap is browser-determined; the framework surfaces the quota error to the dev's handler so they can decide UX response.
- The 7-day eviction is an app-level concern (backup/restore is the app's responsibility); the framework documents the constraint but does not solve it. Apps that need persistent state across long inactivity should ship a backup-export flow.

### 7.6 Tinkerdown's path forward

Tinkerdown is explicitly out of scope (see [§2 Scope](#2-design-philosophy) and the sketch in [§9](#9-future-companion-tinkerdown-static-export)). A `tinkerdown-static-export` design — freeze data sources at build time, ship SQLite as an IndexedDB blob, run WASM-module sources client-side, gate mutations on the IndexedDB+WASM subset — is a separate future deliverable owned by the tinkerdown team, not framework work. This proposal explicitly does *not* extend `lvt build wasm` with tinkerdown-shaped features (source-snapshot pipeline, multi-block `__lvtDispatch(blockID, ...)`, fallback-source proxying) because those concerns belong to a consumer, not the framework.

### 7.7 BroadcastChannel for multi-tab sync

A WASM app open in two tabs of the same browser is currently two independent state copies. **This has a load-bearing data-loss risk that motivates the work beyond "sync is nice":** both tabs write to IndexedDB under the same `groupID = "wasm"` key, so concurrent mutations are last-write-wins with no coordination — if the user clicks "increment" in tab A and "decrement" in tab B at the same moment, one of those mutations is silently lost. This is true of any browser-storage-backed app (not unique to this design), but `lvt:"persist"` fields make it easy to accidentally rely on browser storage as if it were transactional. The `BroadcastChannel` API (Chrome 54+, Safari 15.4+, Firefox 38+) is the standard mitigation. Sketch for a future Phase 2.5: `ctx.Publish(topic, action, data)` in WASM mode posts to a same-origin BroadcastChannel; other tabs' subscribers (`ctx.Subscribe(topic)`) receive and re-render against the same IndexedDB read. The dev's Subscribe/Publish code works identically across server-mode peers and WASM-mode tab peers. Out of scope for v0; documented loudly here so v0 adopters know to either constrain users to one tab (`navigator.locks` API or a UI prompt) or accept the data-loss risk until Phase 2.5 lands.

### 7.8 Worker isolation for CPU-heavy action handlers

[§4.2](#42-the-client-variant)'s main-thread constraint means action handlers freezing for >16 ms cause UI jank. The escape hatch is moving Go WASM into a Web Worker: the main thread keeps the DOM responsive while WASM runs in parallel, with `__lvtDispatch` becoming a `postMessage` round-trip instead of a synchronous call.

Sketched but **not in v0** because (a) the postMessage round-trip adds its own latency (typically 1–5 ms per dispatch, much worse on Safari mobile), so it's a regression for the common case where handlers ARE fast; (b) the bridge needs to serialize state across the worker boundary on every render (DOM stays in the main thread, tree-diff produced in the worker), adding implementation complexity; (c) the constraint is acceptable in practice for the proposal's target use cases (Personal mode = small state, simple handlers).

Phase 2.5 candidate if real-world dogfooding turns up handlers that legitimately need >16 ms — at which point the design becomes a build-flag `lvt build wasm --worker` rather than a default, so apps that don't need it don't pay the cost.

## 8. Why not (alternatives considered)

This section covers the three *architectural* alternatives the design team weighed before choosing WASM ([§8.1](#81-why-not-the-440-clientsessionstore-design), [§8.2](#82-why-not-a-self-hosted-binary-personal-mode-as-a-checklistkit-binary-the-user-runs-locally), [§8.3](#83-why-not-skip-livetemplate-for-personal-mode-entirely-plain-spa-with-vanilla-js-or-reactsvelte)) plus one *timing* alternative ([§8.4](#84-why-not-wait-for-tinygo-to-support-htmltemplate) — when to ship vs. waiting on TinyGo).


### 8.1 Why not the #440 ClientSessionStore design

The #440 proposal kept livetemplate's server architecture intact and routed only the `SessionStore` writes/reads through the client's IndexedDB. State still transited server memory on every request (the proposal itself acknowledged this — see the issue body's design section on caching: "the server holds the deserialized State value in memory for the duration of one session — same lifetime as `MemorySessionStore` holds it today"). Mount, action handlers, and template rendering all ran server-side with plaintext state. Under GDPR Art. 4(2), that is processing — so the proposal's "data must not live on server infrastructure" claim outran its mechanism. The honest framing for that design is "data minimization + storage limitation, but not zero-processing" — valuable, but not the claim the driving use cases (checklistkit Personal mode) wanted to make. This proposal delivers the stronger claim because there is no server.

The #440 design also had a load-bearing structural concern: it claimed a `clientChannel` abstraction "already exists for the diff protocol" — verification showed no such abstraction; transport is direct `WSConn` / `http.ResponseWriter` use today. The wire-protocol work that proposal hand-waved would have required building that abstraction anyway. This proposal sidesteps that work because the WASM client variant adds a single in-process dispatch path next to the existing two, with no new transport machinery.

### 8.2 Why not a self-hosted binary (Personal mode as a `checklistkit` binary the user runs locally)

Distribution friction. The Plex/Jellyfin/Nextcloud pattern works for users who already operate a home server or are willing to run Docker on a NAS — a tiny audience compared to "open this URL in your browser." Self-hosted binary also requires per-platform builds (Mac/Win/Linux/Docker), update channels, install instructions, and a support burden that doesn't exist for a static-site WASM deploy. The privacy claim is the same as WASM mode ("zero data on shared infra"), but the addressable user base is much smaller. Worth offering as a *third* deployment mode for power users at some point; not a replacement for the SaaS-distributable WASM mode.

### 8.3 Why not skip livetemplate for Personal mode entirely (plain SPA with vanilla JS or React/Svelte)

This is what the original #440 proposal explicitly considered and called "the rejected alternative" — but for the wrong reasons. It rejected "templates execute in the browser" on the grounds of needing a Go-compatible JS template engine (~6–10 weeks). That objection is correct for *that path*. A plain SPA doesn't need it — you just use a different framework for Personal mode (React, Svelte, lit-html, etc.) talking directly to IndexedDB/OPFS. Zero framework dependency, zero server.

The cost: two UIs to maintain (Personal SPA + hosted livetemplate). For checklistkit specifically, Personal mode hard-disables every collaboration feature (share, invites, real-time, MCP, comments — see [checklistkit/PLAN.md:20](https://github.com/adnaan/checklistkit/blob/main/PLAN.md)) — so the modes are *already* different products. The "share one codebase" argument for WASM-livetemplate is therefore weaker than it looks for checklistkit, but stronger for apps where Personal mode and hosted mode are the *same* product on different deployment targets. The WASM target supports that broader class.

### 8.4 Why not wait for TinyGo to support `html/template`

TinyGo would shrink the WASM blob 5–10×. But its `reflect` package gap on `html/template` is wide ([§7.4](#74-tinygo-revisit)); no public roadmap commits to closing it on a known timeline. Waiting blocks the use cases for an indefinite period. Better to ship today's full-Go WASM (3–4 MiB compressed — within budget) and reap a free improvement if/when TinyGo catches up.

## 9. Future companion: `tinkerdown-static-export`

This section sketches — not specifies — a future tinkerdown-side feature that *uses* this proposal's primitives but lives in [tinkerdown](https://github.com/livetemplate/tinkerdown)'s repo, not livetemplate's. Included here only to make the boundary in [§2 Scope](#2-design-philosophy) concrete: the line between "framework primitive" and "consumer-shaped feature" can be drawn cleanly, and the consumer-shaped feature is a viable product, not a dead end.

**The user-facing shape.** A new command — `tinkerdown build static <page.md>` — produces a static directory deployable to any CDN. The recipient opens the URL and gets a working interactive dashboard: tables render, filters work, mutations to a SQLite-backed source persist across reloads (via IndexedDB), but mutations to REST/exec/PostgreSQL sources are rejected with a clear read-only message. Use cases: shareable triage board snapshots, a financial-report dashboard your accountant can fork, a personal scratch-pad that doesn't need a Fly machine.

**The build pipeline (sketch).** At build time, `tinkerdown build static`:
1. Parses the markdown and YAML frontmatter exactly as `tinkerdown serve` does today.
2. **Freezes external sources.** REST sources: fetch once, bake the JSON response into the static bundle. Exec sources: run once, bake stdout. PostgreSQL: query once, bake the result set. The bundle ships with snapshot data that the WASM-side runtime serves as if it were a live source — but the runtime refuses writes (`HandleAction` errors with "read-only source").
3. **Translates SQLite sources to IndexedDB.** The `.db` file is parsed at build time, schema + rows emitted as an IndexedDB initialization script. The browser-side SQLite runtime (sql.js or absurd-sql) operates on the IndexedDB-backed copy; writes persist locally.
4. **Bundles WASM-module sources as-is.** Tinkerdown already supports WASM modules running server-side via wazero; the same modules run client-side via [wazero compiled to WASM itself](https://github.com/tetratelabs/wazero) (it's pure Go). The module's `fetch()` and `write()` exports are called via `syscall/js` instead of in-process Go calls — same contract, different bridge.
5. **Emits the static shell + livetemplate WASM bundle** using *this proposal's* primitives: the `lvt-target=wasm` meta tag, the WASM-aware client bundle, the OPFS service worker. The tinkerdown layer rides on top.

**What's borrowed vs. what's added.** Borrowed from this proposal: the WASM build pipeline, the client bundle with the third dispatch path, the IndexedDB persistence layer, the OPFS service worker for uploaded files, the build-tag separation pattern. Added on top in tinkerdown's repo: the source-freeze pipeline, SQLite→IndexedDB translation, the multi-block dispatch convention (each interactive block in a tinkerdown page is its own livetemplate instance, so dispatch must select by `blockID`), the read-only/writable source gating, and the snapshot-staleness UX (e.g., a banner: "this data was last refreshed at 2026-05-24T08:30:00Z").

**Why this stays out of *this* proposal.** Putting source-freeze, multi-block dispatch, or fallback-proxy semantics into `lvt build wasm` would (a) bloat the framework with tinkerdown-specific concerns no other consumer needs, (b) violate the "no new APIs" guardrail by introducing source-snapshot flags + read-only gating into the framework's public surface, and (c) couple the framework's release cadence to tinkerdown's. The clean seam is: this proposal ships `lvt build wasm` as a primitive; tinkerdown ships `tinkerdown build static` as a feature that consumes the primitive on its own timeline. Both can iterate independently.

**Ownership and sequencing.** Owner: tinkerdown team. Prerequisite: this proposal lands and Phase 1 of `lvt build wasm` ships (the framework primitive must exist before tinkerdown can build on it). Estimated effort: a separate ~3–5 week design + implementation cycle in tinkerdown's repo, no framework changes required. Not committed work; surfaced here only so this proposal's scope boundary is concrete and reviewable.
