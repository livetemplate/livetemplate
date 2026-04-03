# Attribute Surface Reduction

**Status:** Proposal
**Date:** 2026-03-30
**Issue:** [#288](https://github.com/livetemplate/livetemplate/issues/288), [#271](https://github.com/livetemplate/livetemplate/issues/271)

## Summary

LiveTemplate currently has ~51 active `lvt-*` attributes. A large custom attribute surface increases maintenance burden, API instability risk, and learning curve. This proposal identifies attributes that can be **eliminated** (replaced by standard HTML5 or Go template patterns), **consolidated** (merged into fewer attributes), or that **must remain** because no HTML equivalent exists.

The goal is to reduce `lvt-*` to the minimum set of attributes that express behaviors standard HTML cannot.

## Guiding Principle

From [FIRST_PRINCIPLES.md](../design/FIRST_PRINCIPLES.md):

> Start with standard HTML. Add `lvt-*` only when HTML can't express it.

An `lvt-*` attribute is justified **only** when all of the following are true:
1. No standard HTML attribute achieves the same behavior
2. No Go template pattern achieves the same behavior
3. The behavior cannot be expressed by combining existing Tier 1 conventions

## Category 1: Can Be Eliminated

These attributes have standard HTML or existing convention replacements.

### `lvt-submit` — Deprecate

**Current:**
```html
<form lvt-submit="create">
    <input name="title">
    <button type="submit">Add</button>
</form>
```

**Replacement (already works today):**
```html
<form method="POST">
    <input name="title">
    <button name="create">Add</button>
</form>
```

Or via form name:
```html
<form name="create" method="POST">
    <input name="title">
    <button type="submit">Add</button>
</form>
```

**Rationale:** `lvt-submit` is already the lowest-priority routing mechanism (action resolution order: button `name` > form `name` > default `"submit"`). Standard HTML covers all use cases. Eliminate immediately — no deprecation period (see [rationale](#no-deprecation-window-deliberate-decision)).

### `lvt-action` (hidden input) — Deprecate

**Current:**
```html
<form method="POST">
    <input type="hidden" name="lvt-action" value="add">
    ...
</form>
```

**Replacement:**
```html
<form method="POST">
    ...
    <button name="add">Add</button>
</form>
```

**Rationale:** Legacy routing mechanism. Button `name` routing supersedes it entirely.

### `lvt-confirm` — Replace with Standard HTML

**Current:**
```html
<button lvt-click="delete" lvt-confirm="Are you sure?">Delete</button>
```

**Replacement:**
```html
<form method="POST" onsubmit="return confirm('Are you sure?')">
    <button name="delete">Delete</button>
</form>
```

Or on a standalone button:
```html
<button name="delete" onclick="return confirm('Are you sure?')">Delete</button>
```

**Rationale:** `confirm()` is a standard browser API. `onsubmit`/`onclick` with `return confirm(...)` is an established HTML pattern that works without any framework. The LiveTemplate client already respects the return value of inline event handlers — if `false`, the action is not sent.

### `lvt-modal-open` / `lvt-modal-close` — Replace with Native `<dialog>`

**Current:**
```html
<button lvt-modal-open="edit-modal">Edit</button>
<dialog id="edit-modal">
    <button lvt-modal-close="edit-modal">Cancel</button>
</dialog>
```

**Replacement (already documented as Tier 1):**
```html
<button command="show-modal" commandfor="edit-modal">Edit</button>
<dialog id="edit-modal">
    <form method="dialog">
        <button command="close" commandfor="edit-modal">Cancel</button>
    </form>
</dialog>
```

**Rationale:** The HTML Invoker Commands API (`command`/`commandfor`) handles modal open/close natively. Focus trapping, backdrop, and Escape key are all browser-native. Already documented in the progressive complexity guide as the preferred approach.

### `lvt-data-*` — Replace with Standard `data-*` or Hidden Inputs

**Current:**
```html
<button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>
```

**Replacement:**
```html
<form method="POST">
    <input type="hidden" name="id" value="{{.ID}}">
    <button name="delete">Delete</button>
</form>
```

Or with `data-*` on the button (already supported by the client):
```html
<button name="delete" data-id="{{.ID}}">Delete</button>
```

**Rationale:** Standard HTML data passing via hidden inputs or `data-*` attributes is already documented as Tier 1. `lvt-data-*` is a duplicate mechanism.

### `lvt-value-*` — Replace with Hidden Inputs or Button `value`

**Current:**
```html
<button lvt-click="update" lvt-value-count="{{.Count}}">Update</button>
```

**Replacement:**
```html
<form method="POST">
    <input type="hidden" name="count" value="{{.Count}}">
    <button name="update">Update</button>
</form>
```

**Rationale:** Same as `lvt-data-*` — hidden inputs cover this with standard HTML.

### `lvt-click` — Narrow Scope

`lvt-click` should NOT be fully eliminated, but its documented scope should narrow:

**Eliminate for buttons:**
```html
<!-- Before -->
<button lvt-click="save">Save</button>

<!-- After: use button name routing -->
<button name="save">Save</button>
```

**Keep for non-button elements only:**
```html
<!-- No standard HTML equivalent for this -->
<tr lvt-click="select" lvt-data-id="{{.ID}}">...</tr>
<div lvt-click="expand">...</div>
```

**Rationale:** `<button name="action">` covers the common case (standalone buttons, v0.8.7+). `lvt-click` remains necessary for non-button elements like table rows, divs, or spans where wrapping in a button would break layout or semantics.

### `lvt-change` — Narrow Scope

**Eliminate for common case:**
```html
<!-- Before: explicit lvt-change -->
<input lvt-change="validate" name="email">

<!-- After: Change() method convention handles it -->
<input name="email" value="{{.Email}}">
```

**Keep when routing to a named action other than `Change()`:**
```html
<!-- Routes to ValidateEmail(), not Change() -->
<input lvt-change="validateEmail" name="email" value="{{.Email}}">
```

**Rationale:** The `Change()` convention (v0.8.6+) auto-wires input events when the controller has a `Change()` method. Explicit `lvt-change` is only needed to route to a different method name.

### `lvt-upload` — Narrow Scope (Tier 1 for Basic Uploads)

Basic file uploads can use standard HTML with no `lvt-*` attributes. See [Tier 1 File Uploads Proposal](tier1-file-uploads-proposal.md) for full design.

**Current:**
```html
<input type="file" lvt-upload="avatar" accept="image/*">
```

```go
tmpl := livetemplate.New("profile",
    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 5 << 20,
        MaxEntries:  1,
        AutoUpload:  true,
    }),
)
```

**Replacement (standard HTML):**
```html
<form method="POST" enctype="multipart/form-data">
    <input type="file" name="avatar" accept="image/*">
    <button type="submit">Upload</button>
</form>

{{if .Uploading}}
    <progress value="{{.UploadProgress}}" max="100"></progress>
{{end}}
```

```go
func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    for _, entry := range ctx.GetCompletedUploads("avatar") {
        state.AvatarPath = moveToStorage(entry.TempPath)
    }
    return state, nil
}
```

**Key design decisions:**
- **HTTP fetch for file transport, not WebSocket.** Binary data over WS requires base64 encoding (33% overhead) and a custom chunked protocol. HTTP multipart is native and efficient.
- **Progress via reactive state.** Server wraps `multipart.Reader` with a byte-counting `io.Reader`, updates session state (e.g., `state.UploadProgress = 45`), and triggers re-render. Progress is just a normal Go template variable — no special protocol.
- **Auto-upload inferred from HTML structure.** File input in a form without a submit button → auto-submit on file selection. Mirrors the standalone button convention.
- **Validation inferred from HTML attributes.** `accept="image/*"` → allowed MIME types. `multiple` present → unlimited entries. No explicit `WithUpload()` config needed for basic cases.

**Keep `lvt-upload` for Tier 2-only features:**
```html
<!-- Custom drop zone (non-input element) — still needs lvt-upload -->
<div lvt-upload="documents" class="drop-zone">Drop files here</div>
```

### Summary of Eliminations

| Attribute | Action | Replacement |
|-----------|--------|-------------|
| `lvt-submit` | Deprecate | `<button name>` or `<form name>` |
| `lvt-action` | Deprecate | `<button name>` |
| `lvt-confirm` | Eliminate | `onsubmit="return confirm('...')"` |
| `lvt-modal-open` | Eliminate | `command="show-modal" commandfor="id"` |
| `lvt-modal-close` | Eliminate | `command="close" commandfor="id"` |
| `lvt-data-*` | Eliminate | `data-*` on button or hidden `<input>` |
| `lvt-value-*` | Eliminate | Hidden `<input>` or button `value` |
| `lvt-click` | Narrow | Keep only for non-button elements |
| `lvt-change` | Narrow | Keep only for non-`Change()` routing |
| `lvt-upload` | Narrow | Keep only for Tier 2 features (custom drop zones) |

**Impact: 7 attributes eliminated, 3 narrowed in scope.**

## Category 2: Can Be Consolidated

Multiple attributes that configure a single feature can be merged into one attribute plus CSS custom properties.

**Prerequisite:** These consolidations require the LiveTemplate TypeScript client (`@livetemplate/client`) to ship a CSS file or auto-inject default styles that read CSS custom properties. The client would expose `livetemplate.css` (or inject a `<style>` block) with sensible defaults.

**Client CSS example:**
```css
/* livetemplate.css — default directive styles */
:root {
  --lvt-scroll-behavior: auto;
  --lvt-scroll-threshold: 100px;
  --lvt-highlight-color: #ffc107;
  --lvt-highlight-duration: 500ms;
  --lvt-animate-duration: 300ms;
}
```

Users override per-element or globally via their own CSS.

### Scroll: 3 → 1

**Before:**
```html
<div lvt-scroll="bottom-sticky"
     lvt-scroll-behavior="smooth"
     lvt-scroll-threshold="100">
```

**After:**
```html
<div lvt-fx:scroll="bottom-sticky"
     style="--lvt-scroll-behavior: smooth; --lvt-scroll-threshold: 100px;">
```

The client reads `--lvt-scroll-behavior` and `--lvt-scroll-threshold` from computed styles. Defaults come from `livetemplate.css`.

### Highlight: 3 → 1

**Before:**
```html
<div lvt-highlight="flash"
     lvt-highlight-color="#ffc107"
     lvt-highlight-duration="500">
```

**After:**
```html
<div lvt-fx:highlight="flash"
     style="--lvt-highlight-color: #ffc107; --lvt-highlight-duration: 500ms;">
```

### Animate: 2 → 1

**Before:**
```html
<div lvt-animate="fade" lvt-animate-duration="300">
```

**After:**
```html
<div lvt-fx:animate="fade" style="--lvt-animate-duration: 300ms;">
```

### Disable/Enable: 2 → 0

`lvt-disable-on:{event}` and `lvt-enable-on:{event}` are syntactic sugar for `lvt-el:toggleAttr:on:{event}="disabled"`, which already exists:

**Before:**
```html
<button lvt-disable-on:pending lvt-enable-on:done>Save</button>
```

**After:**
```html
<button lvt-el:toggleAttr:on:pending="disabled" lvt-el:toggleAttr:on:done="disabled">Save</button>
```

No CSS dependency needed — this is pure attribute consolidation.

### Summary of Consolidations

| Before | After | Reduction |
|--------|-------|-----------|
| `lvt-scroll`, `lvt-scroll-behavior`, `lvt-scroll-threshold` | `lvt-fx:scroll` + CSS custom properties | 3 → 1 |
| `lvt-highlight`, `lvt-highlight-color`, `lvt-highlight-duration` | `lvt-fx:highlight` + CSS custom properties | 3 → 1 |
| `lvt-animate`, `lvt-animate-duration` | `lvt-fx:animate` + CSS custom properties | 2 → 1 |
| `lvt-disable-on:{event}`, `lvt-enable-on:{event}` | `lvt-el:toggleAttr:on:{event}="disabled"` | 2 → 0 |

**Impact: 7 attributes removed via consolidation.**

> **Why not eliminate the main directive attributes too?** The main attributes (`lvt-fx:scroll`, `lvt-fx:highlight`, `lvt-fx:animate`) serve as **discovery markers** — the client finds elements via `querySelectorAll('[lvt-fx\\:scroll]')`. CSS custom properties cannot be queried this way; there is no selector for "elements with `--lvt-scroll` set." Scanning every DOM element with `getComputedStyle()` would be prohibitively expensive. The current split — HTML attribute for behavior declaration (discoverable), CSS custom properties for configuration (readable via `getComputedStyle`) — is the optimal abstraction boundary.

## Category 3: Behaviors That Require Tier 2

These behaviors cannot be expressed with standard HTML. They are the **essential `lvt-*` surface**.

> **Note:** This section identifies the *behaviors* that must remain in Tier 2. Category 5 below consolidates many of these individual attribute names into the generic `lvt-on[:{type}][:{scope}]:{event}` pattern. Category 6 groups the remaining flat attributes under `lvt-fx:`, `lvt-mod:`, and `lvt-form:` prefixes. Category 7 renames reactive DOM attributes under `lvt-el:`. The behaviors remain, but the attribute names change.

### Timing Control
| Attribute | Why |
|-----------|-----|
| `lvt-debounce` → `lvt-mod:debounce` | Server-side debounce timing — no HTML equivalent |
| `lvt-throttle` → `lvt-mod:throttle` | Rate limiting — no HTML equivalent |

### Keyboard Filtering
| Attribute | Why |
|-----------|-----|
| `lvt-key` | Filter by specific key (Enter, Escape, etc.) — no HTML attribute for this |

### Event Routing (element-scoped)
| Attribute | Why |
|-----------|-----|
| `lvt-click` (narrowed) | Non-button click → server action. Buttons use `name` instead. *Consolidated to `lvt-on:click` in Category 5.* |
| `lvt-change` (narrowed) | Named action other than `Change()`. Default case uses convention. *Deprecated entirely in Category 5; use `lvt-on:change` or `lvt-on:input`.* |
| `lvt-input` | Per-keystroke server action (distinct from debounced `Change()`) |
| `lvt-keydown` | Element keydown → server action |
| `lvt-keyup` | Element keyup → server action |
| `lvt-focus` | Focus → server action |
| `lvt-blur` | Blur → server action |
| `lvt-mouseenter` | Mouse enter → server action |
| `lvt-mouseleave` | Mouse leave → server action |
| `lvt-mouseover` | Mouse over → server action (distinct from `mouseenter`: fires on child elements too) |
| `lvt-click-away` | Click outside element — no HTML native. *Consolidated to `lvt-on:custom:click-away` in Category 5.* |

### Event Routing (window-scoped)
| Attribute | Why |
|-----------|-----|
| `lvt-window-keydown` | Global keyboard shortcuts → server |
| `lvt-window-keyup` | Global key release → server |
| `lvt-window-scroll` | Window scroll → server |
| `lvt-window-resize` | Window resize → server |
| `lvt-window-focus` | Window focus → server |
| `lvt-window-blur` | Window blur → server |

### Reactive DOM (lifecycle-driven, not DOM events)
| Attribute | Why |
|-----------|-----|
| `lvt-addClass-on:{lifecycle}` → `lvt-el:addClass:on:{lifecycle}` | Add CSS class on pending/success/error/done |
| `lvt-removeClass-on:{lifecycle}` → `lvt-el:removeClass:on:{lifecycle}` | Remove CSS class on lifecycle event |
| `lvt-toggleClass-on:{lifecycle}` → `lvt-el:toggleClass:on:{lifecycle}` | Toggle CSS class on lifecycle event |
| `lvt-setAttr-on:{lifecycle}` → `lvt-el:setAttr:on:{lifecycle}` | Set attribute on lifecycle event |
| `lvt-toggleAttr-on:{lifecycle}` → `lvt-el:toggleAttr:on:{lifecycle}` | Toggle boolean attribute (subsumes `disable-on`/`enable-on`) |
| `lvt-reset-on:{lifecycle}` → `lvt-el:reset:on:{lifecycle}` | Reset form on lifecycle event |

### Form Behavior
| Attribute | Why |
|-----------|-----|
| `lvt-preserve` → `lvt-form:preserve` | Prevent form auto-reset — opposite of default framework behavior |
| `lvt-disable-with` → `lvt-form:disable-with` | Button text swap + disable during pending — no HTML pattern |
| `lvt-no-intercept` → `lvt-form:no-intercept` | Opt-out of SPA form interception — framework-specific concept |

### Directives
| Attribute | Why |
|-----------|-----|
| `lvt-scroll` → `lvt-fx:scroll` | Scroll position management (bottom, bottom-sticky, preserve) |
| `lvt-highlight` → `lvt-fx:highlight` | Temporary highlight effect on update |
| `lvt-animate` → `lvt-fx:animate` | Entrance animation on insert |

### Upload
| Attribute | Why |
|-----------|-----|
| `lvt-upload` (narrowed) | Tier 2-only: custom drop zones (`<div>` as drop target). Basic file uploads moved to Tier 1 via standard `<input type="file" name="...">` in a `<form>`. See [Tier 1 File Uploads Proposal](tier1-file-uploads-proposal.md). |

## Category 5: Generic Event Router Consolidation

Individual event-binding attributes (`lvt-click`, `lvt-keydown`, `lvt-mouseenter`, etc.) can be replaced by a single generic pattern. The client already uses a parameterized loop internally (`attrName = lvt-${eventType}`) — this change surfaces that generic infrastructure to users.

### Grammar: `lvt-on[:{type}][:{scope}]:{event}`

The full attribute syntax is `lvt-on[:{type}][:{scope}]:{event}="action"`. Both `type` and `scope` are optional prefixes; omitting them selects the defaults.

**Scope** — the `EventTarget` to attach the listener to (DOM Living Standard names):

| Scope keyword | `EventTarget` | Use cases |
|---|---|---|
| (omitted) | `Element` (default) | click, input, keydown, focus, blur — the common case |
| `window` | `Window` | resize, scroll, global keyboard shortcuts |
| `document` | `Document` | visibilitychange, selectionchange, fullscreenchange |

**Type** — how the event is dispatched (DOM Living Standard event interfaces):

| Type keyword | DOM interface | `isTrusted` | Use cases |
|---|---|---|---|
| (omitted) | `Event`/`MouseEvent`/`KeyboardEvent` etc. | `true` (browser) | All standard browser-dispatched events |
| `custom` | `CustomEvent` | `false` (script) | Developer-defined events requiring special client-side delegation logic |

**Why no explicit `native` keyword:** "Native" is not a DOM specification term. Browser-dispatched events are simply "events" in the DOM Living Standard — omitting the type prefix is the web-standard default. Adding a `native` keyword would be a non-standard invention.

**Why `custom` maps to `CustomEvent`:** The DOM Living Standard's `CustomEvent` interface is specifically for developer-defined events. For `lvt-on:custom:click-away`, the client:
1. Sets up its own detection logic (e.g. a document-level click listener checking `!element.contains(event.target)`)
2. Fires `element.dispatchEvent(new CustomEvent('click-away'))` when detected
3. The router's `addEventListener('click-away', handler)` on the element receives it normally

This decouples detection logic from routing logic — cleaner than an embedded `setupClickAwayDelegation()` monolith.

**Reserved keywords:** `custom`, `window`, `document` must not be used as bare event names within the `lvt-on:` namespace.

**Parser algorithm** (ordered greedy consumption):

```
// Disjoint reserved sets — no ambiguity
typeKeywords  = { 'custom' }
scopeKeywords = { 'window', 'document' }

segments = rest.split(':')   // rest = everything after "lvt-on:"
type  = 'browser'            // default
scope = 'element'            // default

if segments[0] in typeKeywords:  type  = segments.shift()
if segments[0] in scopeKeywords: scope = segments.shift()
event = segments.join(':')       // remaining; join preserves hyphenated names
```

Examples: `lvt-on:click` → (browser, element, click) · `lvt-on:window:keydown` → (browser, window, keydown) · `lvt-on:custom:click-away` → (CustomEvent, element, click-away) · `lvt-on:document:visibilitychange` → (browser, document, visibilitychange)

---

### `lvt-on:{event}` — Element-Scoped Events

Replaces all individual element-scoped event attributes:

| Before | After |
|--------|-------|
| `lvt-click="select"` | `lvt-on:click="select"` |
| `lvt-input="search"` | `lvt-on:input="search"` |
| `lvt-keydown="navigate"` | `lvt-on:keydown="navigate"` |
| `lvt-keyup="release"` | `lvt-on:keyup="release"` |
| `lvt-focus="show"` | `lvt-on:focus="show"` |
| `lvt-blur="hide"` | `lvt-on:blur="hide"` |
| `lvt-mouseenter="preview"` | `lvt-on:mouseenter="preview"` |
| `lvt-mouseleave="unpreview"` | `lvt-on:mouseleave="unpreview"` |
| `lvt-mouseover="highlight"` | `lvt-on:mouseover="highlight"` |
| `lvt-click-away="close"` | `lvt-on:custom:click-away="close"` |

**Example — autocomplete component:**
```html
<!-- Before: 5 different attribute names -->
<div lvt-click-away="close">
  <input lvt-input="search" lvt-focus="open" lvt-blur="close" lvt-debounce="300">
  {{if .DropdownOpen}}
  <ul>
    {{range .Results}}
      <li lvt-click="select" data-id="{{.ID}}">{{.Name}}</li>
    {{end}}
  </ul>
  {{end}}
</div>

<!-- After: 1 attribute pattern, Close() sets DropdownOpen=false and the dropdown disappears -->
<div lvt-on:custom:click-away="close">
  <input lvt-on:input="search" lvt-on:focus="open" lvt-on:blur="close" lvt-mod:debounce="300">
  {{if .DropdownOpen}}
  <ul>
    {{range .Results}}
      <li lvt-on:click="select" data-id="{{.ID}}">{{.Name}}</li>
    {{end}}
  </ul>
  {{end}}
</div>
```

**Note:** `lvt-on:click` is only for non-button elements. Buttons use standard HTML `<button name="action">`.

**Note:** `lvt-on:custom:click-away` uses the `custom` type prefix because `click-away` is not a native DOM event — it is dispatched as `new CustomEvent('click-away')` by the client's inverted-containment delegation logic. See [Grammar](#grammar-lvt-ontypescopeevent) above for the full dispatch pattern.

**Note on namespace distinction:** `lvt-el:{method}:on:{lifecycle}` (reactive DOM) and `lvt-on:{event}` (event routing) both use `on` with colon separators but are semantically distinct. `lvt-el:addClass:on:pending` reacts to framework lifecycle states (pending/success/error/done) and manipulates the DOM client-side. `lvt-on:click` routes DOM events to server actions. The `lvt-el:` prefix disambiguates them.

### `lvt-on:window:{event}` — Window-Scoped Events (`Window` EventTarget)

Routes events on the `Window` object — global events that are not scoped to a specific element:

| Before | After |
|--------|-------|
| `lvt-window-keydown="shortcut"` | `lvt-on:window:keydown="shortcut"` |
| `lvt-window-keyup="release"` | `lvt-on:window:keyup="release"` |
| `lvt-window-scroll="loadMore"` | `lvt-on:window:scroll="loadMore"` |
| `lvt-window-resize="layout"` | `lvt-on:window:resize="layout"` |
| `lvt-window-focus="resume"` | `lvt-on:window:focus="resume"` |
| `lvt-window-blur="pause"` | `lvt-on:window:blur="pause"` |

**Example — keyboard shortcut:**
```html
<div lvt-on:window:keydown="shortcut" lvt-key="/">
  <!-- Pressing "/" anywhere on the page routes to Shortcut() -->
</div>
```

### `lvt-on:document:{event}` — Document-Scoped Events (`Document` EventTarget)

Routes events on the `Document` object. This is a new capability enabled by the grammar (no `lvt-window-document-*` equivalents exist today):

| Attribute | Use case |
|-----------|----------|
| `lvt-on:document:visibilitychange="pause"` | Pause activity when tab is hidden |
| `lvt-on:document:fullscreenchange="resize"` | React to fullscreen state changes |
| `lvt-on:document:selectionchange="annotate"` | React to text selection changes |

**Example — pause on tab hidden:**
```html
<div lvt-on:document:visibilitychange="pause">
  <!-- Routes to Pause() whenever the tab becomes hidden or visible -->
</div>
```

### `lvt-change` — Deprecate Entirely

`lvt-change` is removed with no direct replacement:

- **Default case (route to `Change()`):** Already handled by the `Change()` auto-wiring convention — no attribute needed. If a controller has a `Change()` method, inputs auto-wire to it.
- **Named action case (route to a specific method):** Use the appropriate DOM event via the generic router. For text inputs, prefer `lvt-on:input="validateEmail"` (fires on every keystroke, more responsive). For select elements, checkboxes, and file inputs, use `lvt-on:change="sort"` (the `change` event is the correct semantic for these element types).

| Before | After |
|--------|-------|
| No attribute (auto-wired `Change()`) | No attribute (unchanged) |
| `lvt-change="validateEmail"` | `lvt-on:input="validateEmail"` |
| `<select lvt-change="sort">` | `<select lvt-on:change="sort">` (select elements use DOM `change` event) |

**Rationale:** `lvt-change` created ambiguity — users couldn't tell whether it was needed alongside the `Change()` convention. Removing it forces a clear choice: convention for the default, explicit `lvt-on:{event}` for named routing.

### `lvt-click` — Fully Superseded

With the generic router, `lvt-click` is fully superseded:

- **On buttons:** Use `<button name="action">` (standard HTML, Tier 1)
- **On non-button elements:** Use `lvt-on:click="action"` (generic router, Tier 2)

> **Why not `onclick` instead?** The `onclick` attribute executes JavaScript in the browser. LiveTemplate's action routing sends a message to a server-side Go method — fundamentally different semantics. Convention-based `onclick` (e.g., `onclick="lvt('delete')"`) would require exposing a global JS function, mixing declarative templates with imperative JS, and breaking the no-JS fallback model. The correct Tier 1 alternative is `<button name="action">` — semantic HTML that works across all three transport levels (no-JS POST, fetch, WebSocket) with zero JavaScript.

### Impact Summary

| Change | Attributes Removed |
|--------|-------------------|
| Element-scoped events → `lvt-on:{event}` or `lvt-on:custom:{event}` | 10 (`lvt-click`, `lvt-input`, `lvt-keydown`, `lvt-keyup`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-mouseover`, `lvt-click-away`) |
| Window-scoped events → `lvt-on:window:{event}` | 6 (`lvt-window-keydown`, `lvt-window-keyup`, `lvt-window-scroll`, `lvt-window-resize`, `lvt-window-focus`, `lvt-window-blur`) |
| `lvt-change` deprecated entirely | 1 |
| **Total** | **17 attribute names removed, replaced by 1 generic pattern (`lvt-on[:{type}][:{scope}]:{event}`)** |

### Modifiers Unchanged

`lvt-debounce`, `lvt-throttle`, and `lvt-key` continue to work alongside the generic router:

```html
<input lvt-on:input="search" lvt-mod:debounce="300">
<div lvt-on:window:keydown="shortcut" lvt-key="/">
```

## Category 6: Prefix Consolidation

Categories 1–5 reduced the surface from ~51 to ~16 named attributes + 1 generic event pattern. This section explores whether the remaining 10 flat-named attributes can be further organized under colon-delimited prefixes, following the same pattern as `lvt-on:{event}`.

### Current Flat Attributes (10)

After Categories 1–5, these attributes remain as flat names with no structural prefix:

| Attribute | Semantic role |
|-----------|--------------|
| `lvt-debounce` | Timing modifier for event routing |
| `lvt-throttle` | Timing modifier for event routing |
| `lvt-key` | Key filter for keyboard events |
| `lvt-scroll` | Visual effect: scroll position management |
| `lvt-highlight` | Visual effect: temporary highlight on update |
| `lvt-animate` | Visual effect: entrance animation on insert |
| `lvt-preserve` | Form behavior: prevent form auto-reset |
| `lvt-disable-with` | Form behavior: button text swap + disable during pending |
| `lvt-no-intercept` | Form behavior: opt-out of SPA form interception |
| `lvt-upload` | Upload: custom drop zones for non-input elements |

### Candidate Prefixes

Three natural groups emerge, each with a candidate prefix:

**`lvt-fx:` — Visual Effects (3 attributes)**

| Before | After |
|--------|-------|
| `lvt-scroll="bottom-sticky"` | `lvt-fx:scroll="bottom-sticky"` |
| `lvt-highlight="flash"` | `lvt-fx:highlight="flash"` |
| `lvt-animate="fade"` | `lvt-fx:animate="fade"` |

Rationale: All three apply visual/behavioral effects to DOM elements. The `fx` prefix (short for "effects") signals "this attribute triggers a client-side visual behavior." Future effects (e.g., `lvt-fx:parallax`, `lvt-fx:transition`) would naturally extend this prefix.

**`lvt-mod:` — Event Modifiers (2 attributes)**

| Before | After |
|--------|-------|
| `lvt-debounce="300"` | `lvt-mod:debounce="300"` |
| `lvt-throttle="500"` | `lvt-mod:throttle="500"` |

Rationale: Both modify how `lvt-on:*` events are dispatched — they don't trigger actions themselves but alter the timing of event delivery. The `mod` prefix (short for "modifier") signals "this attribute modifies a sibling event attribute." `lvt-key` is excluded: it filters which key triggers the event rather than modifying event timing, and is already concise.

**`lvt-form:` — Form Behavior (3 attributes)**

| Before | After |
|--------|-------|
| `lvt-preserve` | `lvt-form:preserve` |
| `lvt-disable-with="Saving..."` | `lvt-form:disable-with="Saving..."` |
| `lvt-no-intercept` | `lvt-form:no-intercept` |

Rationale: All three modify how the framework handles HTML forms — they are form-scoped behavioral overrides. The `form` prefix signals "this attribute affects form processing." Future form behaviors would extend this prefix.

**Not grouped:**
- `lvt-key` — already concise, semantically distinct from timing modifiers
- `lvt-upload` — standalone concern (custom drop zones only); doesn't fit cleanly into effects or form groups

### Evaluation Matrix

| Criterion | Without prefixes (status quo) | With prefixes (`lvt-fx:`, `lvt-mod:`, `lvt-form:`) |
|-----------|------|------|
| **Things to learn** | 2 patterns + 10 flat names = 12 concepts | 5 prefix groups + 2 flat = 7 concepts |
| **Discoverability** | Developer must know exact name | Type `lvt-fx:` → IDE autocomplete shows all effects |
| **Consistency** | `lvt-on:*` uses colons, others don't | All behavioral attributes use colon-delimited prefixes |
| **CSS selector escaping** | No escaping for flat attrs | `[lvt-fx\:scroll]` needs escaping (mitigated by `lvtSelector()` utility) |
| **Verbosity** | `lvt-scroll` (10 chars) | `lvt-fx:scroll` (14 chars) — +4 chars |
| **Future extensibility** | Each new attr needs a globally unique name | New attrs slot into existing prefix families |
| **Breaking change cost** | None (already decided) | Zero marginal cost — attributes are already being renamed in this proposal |
| **Framework precedent** | HTMX (`hx-*` flat) | Alpine.js (`x-on:`, `x-bind:`, `x-transition:` — colon-delimited) |

### Comparison with Other Frameworks

- **Alpine.js** uses colon-delimited namespaces extensively: `x-on:click`, `x-bind:class`, `x-transition:enter`. This is the closest precedent for LiveTemplate's approach.
- **HTMX** uses flat names (`hx-get`, `hx-trigger`, `hx-target`) — simpler but less organized as the attribute count grows.
- **Stimulus** uses `data-*` with controller namespacing — different pattern but similar goal of grouping.

LiveTemplate already adopted the colon-delimited pattern for `lvt-on:*` and `lvt-el:*:on:*`. Extending it to effects, modifiers, and form behavior creates a consistent system.

### Recommendation: Adopt Prefixes

**Adopt `lvt-fx:`, `lvt-mod:`, and `lvt-form:` prefixes.** The cognitive load reduction (12 → 7 concepts) and consistency with `lvt-on:*` outweigh the minor verbosity increase. The CSS escaping concern is already addressed by the `lvtSelector()` utility introduced for `lvt-on:*`.

**Updated attribute taxonomy after all consolidations (Categories 6–7):**

| Prefix family | Pattern | Count | Purpose |
|---------------|---------|-------|---------|
| `lvt-on:` | `lvt-on[:{type}][:{scope}]:{event}` | 1 generic | Event routing to server actions |
| `lvt-el:` | `lvt-el:addClass:on:pending`, etc. | 6 named | Reactive DOM based on lifecycle states |
| `lvt-fx:` | `lvt-fx:scroll`, `lvt-fx:highlight`, `lvt-fx:animate` | 3 named | Visual effects and DOM behaviors |
| `lvt-mod:` | `lvt-mod:debounce`, `lvt-mod:throttle` | 2 named | Event timing modifiers |
| `lvt-form:` | `lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept` | 3 named | Form behavior overrides |
| (flat) | `lvt-key` | 1 named | Key filter |
| (flat) | `lvt-upload` | 1 named | Custom drop zones |

**Total: 5 prefix families + 2 standalone = 7 attribute concepts (down from 12)**

### Complete Example After All Consolidations

```html
<!-- Event routing (lvt-on:) -->
<input lvt-on:input="search" lvt-mod:debounce="300">
<div lvt-on:window:keydown="shortcut" lvt-key="/">
<div lvt-on:custom:click-away="close">

<!-- Visual effects (lvt-fx:) -->
<div lvt-fx:scroll="bottom-sticky"
     style="--lvt-scroll-behavior: smooth; --lvt-scroll-threshold: 100px;">
<div lvt-fx:highlight="flash"
     style="--lvt-highlight-color: #ffc107; --lvt-highlight-duration: 500ms;">
<li lvt-fx:animate="fade"
    style="--lvt-animate-duration: 300ms;">

<!-- Form behavior (lvt-form:) -->
<form lvt-form:no-intercept>
<input lvt-form:preserve>
<button lvt-form:disable-with="Saving...">Save</button>

<!-- Reactive DOM (lvt-el:) -->
<button lvt-el:toggleAttr:on:pending="disabled"
        lvt-el:addClass:on:pending="opacity-50">
  Save
</button>

<!-- Upload (standalone) -->
<div lvt-upload="documents" class="drop-zone">Drop files here</div>
```

| Category | Before | After |
|----------|--------|-------|
| **Total `lvt-*` surface** | ~51 | ~16 named attributes across 5 prefix families + 2 standalone + 1 generic event pattern |
| Event bindings (element + window) | 16 individual names + 1 (`lvt-change`) | `lvt-on[:{type}][:{scope}]:{event}` (1 pattern, 3 scope variants) |
| Event bindings (document) | 0 | `lvt-on:document:{event}` (new capability) |
| Data passing | 2 patterns (`lvt-data-*`, `lvt-value-*`) | 0 (standard HTML) |
| Modals | 2 | 0 (native `<dialog>`) |
| Legacy routing | 2 (`lvt-submit`, `lvt-action`) | 0 (standard HTML) |
| Confirmation | 1 | 0 (standard `onsubmit`/`onclick`) |
| Scroll config | 3 (`lvt-scroll` + 2 config attrs) | 1 (`lvt-fx:scroll`) + CSS custom properties |
| Highlight config | 3 (`lvt-highlight` + 2 config attrs) | 1 (`lvt-fx:highlight`) + CSS custom properties |
| Animation config | 2 (`lvt-animate` + 1 config attr) | 1 (`lvt-fx:animate`) + CSS custom properties |
| Enable/disable sugar | 2 | 0 (use `lvt-el:toggleAttr:on`) |
| Upload | 1 | 1 (`lvt-upload`, narrowed: Tier 1 for basic uploads via standard HTML, Tier 2 for custom drop zones) |
| Timing modifiers | 2 (`lvt-debounce`, `lvt-throttle`) | 2 (`lvt-mod:debounce`, `lvt-mod:throttle`) |
| Key filter | 1 (`lvt-key`) | 1 (unchanged) |
| Reactive DOM | 6 (`lvt-*-on:{lifecycle}`) | 6 (`lvt-el:{method}:on:{lifecycle}`) |
| Form behavior | 3 (`lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept`) | 3 (`lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept`) |

## Category 7: `lvt-el:` Prefix and Unified Grammar

### Problem

After Categories 1–6, the reactive DOM family is the only family using a wildcard-in-the-middle naming pattern: `lvt-{method}-on:{lifecycle}` (e.g., `lvt-addClass-on:pending`). Every other family uses the consistent `lvt-{family}:{member}` convention. This irregularity means:
- The parser must pattern-match `lvt-*-on:*` wildcards instead of checking a prefix
- The family is not enumerable by prefix (`querySelectorAll` cannot target all reactive DOM attrs at once)
- The `-on:` hybrid (dash-colon) separator is inconsistent with the `:on:` colon-colon separator implied by the rest of the system

### Solution: `lvt-el:` Prefix

Rename all 6 reactive DOM attributes under a new `lvt-el:` family prefix:

| Before | After |
|--------|-------|
| `lvt-addClass-on:{lifecycle}` | `lvt-el:addClass:on:{lifecycle}` |
| `lvt-removeClass-on:{lifecycle}` | `lvt-el:removeClass:on:{lifecycle}` |
| `lvt-toggleClass-on:{lifecycle}` | `lvt-el:toggleClass:on:{lifecycle}` |
| `lvt-setAttr-on:{lifecycle}` | `lvt-el:setAttr:on:{lifecycle}` |
| `lvt-toggleAttr-on:{lifecycle}` | `lvt-el:toggleAttr:on:{lifecycle}` |
| `lvt-reset-on:{lifecycle}` | `lvt-el:reset:on:{lifecycle}` |

**Why `lvt-el:`?** "el" = Element. All 6 methods are DOM Element operations (`classList.add()`, `setAttribute()`, `toggleAttribute()`, `HTMLFormElement.reset()`). Short, unambiguous, maps directly to DOM vocabulary.

**camelCase note:** The method segment uses camelCase (`addClass`, not `add-class`) to match the DOM API naming convention. This is consistent with the existing attribute names — they already use camelCase.

**Lifecycle events** (closed set): `pending`, `success`, `error`, `done`. These are LiveTemplate framework states representing the server action request-response lifecycle.

### Unified Grammar

With `lvt-el:` adopted, every attribute family follows the same structural pattern:

```
lvt-{family}:{member}[:on:{lifecycle}]="value"
```

| Family | Grammar | `:on:` trigger | Status |
|--------|---------|---------------|--------|
| `lvt-on:` | `lvt-on[:{type}][:{scope}]:{event}="action"` | IS the event (not a suffix) | v1 |
| `lvt-el:` | `lvt-el:{method}:on:{lifecycle}="value"` | **Required** — specifies when to manipulate the element | v1 |
| `lvt-fx:` | `lvt-fx:{effect}[="config"]` | Optional (see Future Extension) | v1 (without `:on:`) |
| `lvt-mod:` | `lvt-mod:{modifier}="value"` | Not applicable — modifies sibling `lvt-on:*` | v1 |
| `lvt-form:` | `lvt-form:{behavior}[="value"]` | Optional (see Future Extension) | v1 (without `:on:`) |

The `:on:{lifecycle}` suffix is a universal **trigger mechanism**. It answers the question: "When should this behavior activate?" For `lvt-el:`, the trigger is always explicit and required. For other families, the trigger is currently implicit (effects activate on DOM update, form behaviors on submission).

### Parser: `lvt-el:{method}:on:{lifecycle}`

```
methodKeywords = { 'addClass', 'removeClass', 'toggleClass', 'setAttr', 'toggleAttr', 'reset' }
lifecycleKeywords = { 'pending', 'success', 'error', 'done' }

// After matching "lvt-el:" prefix:
segments = rest.split(':')                          // e.g. "addClass:on:pending" → ["addClass", "on", "pending"]
method = segments[0]                                // "addClass"
assert segments[1] == 'on'                          // literal ":on:" separator
lifecycle = segments[2]                             // "pending"
assert method in methodKeywords
assert lifecycle in lifecycleKeywords
```

### Future Extension: `:on:` for `lvt-fx:` and `lvt-form:`

The unified grammar is designed to be **forward-compatible** with optional `:on:{lifecycle}` suffixes on other families. This is NOT part of v1 but does not require future breaking changes to enable.

**`lvt-fx:` with optional `:on:` (future):**

Currently `lvt-fx:highlight="flash"` activates on every DOM content change. A future `:on:` suffix could make the trigger conditional:

```html
<!-- v1: highlight on any content change (implicit trigger) -->
<div lvt-fx:highlight="flash">

<!-- Future: highlight only on success lifecycle -->
<div lvt-fx:highlight:on:success="flash">

<!-- Future: different effects for different lifecycle states -->
<div lvt-fx:highlight:on:success="flash" lvt-fx:highlight:on:error="shake">
```

**`lvt-form:` with optional `:on:` (future):**

```html
<!-- v1: always preserve form state (implicit trigger) -->
<input lvt-form:preserve>

<!-- Future: preserve only on error (reset on success) -->
<input lvt-form:preserve:on:error>
```

**Why NOT `lvt-mod:` with `:on:`?** Event modifiers (debounce, throttle) are **adverbs** — they modify how an event handler dispatches, not when they activate. Debounce doesn't "fire on pending" — it alters the timing of event delivery. The modifier's target is the sibling `lvt-on:*` attribute on the same element, which is already implicit and unambiguous for the common case.

### Final Attribute Surface

After all reductions (Categories 1–7), the complete `lvt-*` surface is:

**Named attributes (16) in 5 prefix families + 2 standalone:**

| # | Attribute | Family |
|---|-----------|--------|
| 1 | `lvt-mod:debounce` | `lvt-mod:` (event modifier) |
| 2 | `lvt-mod:throttle` | `lvt-mod:` (event modifier) |
| 3 | `lvt-key` | standalone (key filter) |
| 4 | `lvt-el:addClass:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 5 | `lvt-el:removeClass:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 6 | `lvt-el:toggleClass:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 7 | `lvt-el:setAttr:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 8 | `lvt-el:toggleAttr:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 9 | `lvt-el:reset:on:{lifecycle}` | `lvt-el:` (reactive DOM) |
| 10 | `lvt-form:preserve` | `lvt-form:` (form behavior) |
| 11 | `lvt-form:disable-with` | `lvt-form:` (form behavior) |
| 12 | `lvt-form:no-intercept` | `lvt-form:` (form behavior) |
| 13 | `lvt-fx:scroll` | `lvt-fx:` (visual effect) |
| 14 | `lvt-fx:highlight` | `lvt-fx:` (visual effect) |
| 15 | `lvt-fx:animate` | `lvt-fx:` (visual effect) |
| 16 | `lvt-upload` | standalone (upload) |

**Generic event pattern (1):**

`lvt-on[:{type}][:{scope}]:{event}` — replaces 17 individual event attributes with 1 pattern supporting 3 scope variants (element, window, document) and custom event type.

---

## Implementation Plan

### Design Summary (Quick Reference for Implementors)

This section recaps the key design decisions from Categories 1-7 so each implementation phase is self-contained.

**Grammar:** `lvt-on[:{type}][:{scope}]:{event}="action"` (event routing) · `lvt-el:{method}:on:{lifecycle}="value"` (reactive DOM)

| Segment | Values | Default |
|---------|--------|---------|
| `type` | (omitted) = browser-dispatched, `custom` = `CustomEvent` | browser |
| `scope` | (omitted) = element, `window`, `document` | element |
| `event` | Any DOM event name or custom event name | — |

**Parser algorithm** (ordered greedy consumption):
```
typeKeywords  = { 'custom' }
scopeKeywords = { 'window', 'document' }
segments = rest.split(':')           // rest = everything after "lvt-on:"
if segments[0] in typeKeywords:  type  = segments.shift()
if segments[0] in scopeKeywords: scope = segments.shift()
event = segments.join(':')           // remainder is the event name
```

**Attribute replacement rules** (used in all phases):

| Old attribute | Replacement |
|---------------|-------------|
| `lvt-click="X"` on `<button>` | `name="X"` (Tier 1 button routing) |
| `lvt-click="X"` on non-button | `lvt-on:click="X"` |
| `lvt-submit="X"` | Remove; use `<button name="X">` or `<form name="X">` |
| `lvt-data-{key}="V"` | `data-{key}="V"` |
| `lvt-value-{key}="V"` | `<input type="hidden" name="{key}" value="V">` |
| `lvt-confirm="msg"` | `onsubmit="return confirm('msg')"` or `onclick="return confirm('msg')"` |
| `lvt-modal-open="id"` | `command="show-modal" commandfor="id"` |
| `lvt-modal-close="id"` | `command="close" commandfor="id"` (inside `<form method="dialog">`) |
| `lvt-change="X"` | `lvt-on:change="X"` (select/checkbox) or `lvt-on:input="X"` (text) |
| `lvt-input="X"` | `lvt-on:input="X"` |
| `lvt-keydown="X"` | `lvt-on:keydown="X"` |
| `lvt-keyup="X"` | `lvt-on:keyup="X"` |
| `lvt-focus="X"` | `lvt-on:focus="X"` |
| `lvt-blur="X"` | `lvt-on:blur="X"` |
| `lvt-mouseenter="X"` | `lvt-on:mouseenter="X"` |
| `lvt-mouseleave="X"` | `lvt-on:mouseleave="X"` |
| `lvt-mouseover="X"` | `lvt-on:mouseover="X"` |
| `lvt-click-away="X"` | `lvt-on:custom:click-away="X"` |
| `lvt-window-keydown="X"` | `lvt-on:window:keydown="X"` |
| `lvt-window-keyup="X"` | `lvt-on:window:keyup="X"` |
| `lvt-window-scroll="X"` | `lvt-on:window:scroll="X"` |
| `lvt-window-resize="X"` | `lvt-on:window:resize="X"` |
| `lvt-window-focus="X"` | `lvt-on:window:focus="X"` |
| `lvt-window-blur="X"` | `lvt-on:window:blur="X"` |
| `lvt-scroll="X"` | `lvt-fx:scroll="X"` |
| `lvt-highlight="X"` | `lvt-fx:highlight="X"` |
| `lvt-animate="X"` | `lvt-fx:animate="X"` |
| `lvt-debounce="X"` | `lvt-mod:debounce="X"` |
| `lvt-throttle="X"` | `lvt-mod:throttle="X"` |
| `lvt-preserve` | `lvt-form:preserve` |
| `lvt-disable-with="X"` | `lvt-form:disable-with="X"` |
| `lvt-no-intercept` | `lvt-form:no-intercept` |
| `lvt-addClass-on:{lifecycle}` | `lvt-el:addClass:on:{lifecycle}` |
| `lvt-removeClass-on:{lifecycle}` | `lvt-el:removeClass:on:{lifecycle}` |
| `lvt-toggleClass-on:{lifecycle}` | `lvt-el:toggleClass:on:{lifecycle}` |
| `lvt-setAttr-on:{lifecycle}` | `lvt-el:setAttr:on:{lifecycle}` |
| `lvt-toggleAttr-on:{lifecycle}` | `lvt-el:toggleAttr:on:{lifecycle}` |
| `lvt-reset-on:{lifecycle}` | `lvt-el:reset:on:{lifecycle}` |

**CSS selector escaping:** Colons in `lvt-on:*`, `lvt-el:*`, `lvt-fx:*`, `lvt-mod:*`, and `lvt-form:*` must be escaped in CSS selectors. Use the `lvtSelector(attr, value?)` utility (Phase 1 Step 3) for all `querySelectorAll` calls.

### Path Convention

All paths in this plan use the `$REPO_ROOT` prefix, which refers to the parent directory containing all LiveTemplate repositories:

```bash
export REPO_ROOT=/path/to/livetemplate  # e.g., ~/code/livetemplate
# Expected layout:
# $REPO_ROOT/livetemplate/  — Go server library (this repo)
# $REPO_ROOT/client/        — TypeScript client
# $REPO_ROOT/lvt/           — CLI tool + component library
# $REPO_ROOT/tinkerdown/    — Tinkerdown
# $REPO_ROOT/examples/      — Example applications
```

### No Deprecation Window (Deliberate Decision)

This plan skips the deprecation-warning phase from the original migration path. The library and TypeScript client are consumed only by the `lvt`, `tinkerdown`, and `examples` repositories — all maintained in the same organization. All consuming repos are updated atomically in Phases 2-4. This eliminates the runtime warning phase, but does **not** eliminate semver and changelog obligations: the npm package (`@livetemplate/client`) requires a **major version bump** (e.g. `2.0.0`) and a **changelog entry** documenting every removed attribute. External npm dependents can pin to the previous version and migrate at their own pace. If the client gains significant external adoption before this executes, reinstate a deprecation phase before the next breaking change.

### CSS Selector Escaping Convention

The `lvt-on:{event}`, `lvt-el:{method}:on:{lifecycle}`, `lvt-fx:{effect}`, `lvt-mod:{modifier}`, and `lvt-form:{behavior}` syntax introduces colons in HTML attribute names. Colons must be escaped in CSS selectors:

```ts
// Wrong — unescaped colons
document.querySelectorAll('[lvt-on:click="X"]')
document.querySelectorAll('[lvt-on:custom:click-away]')
document.querySelectorAll('[lvt-el:addClass:on:pending]')
document.querySelectorAll('[lvt-fx:scroll]')

// Correct — escaped colons
document.querySelectorAll('[lvt-on\\:click="X"]')
document.querySelectorAll('[lvt-on\\:custom\\:click-away]')
document.querySelectorAll('[lvt-on\\:window\\:keydown="X"]')
document.querySelectorAll('[lvt-el\\:addClass\\:on\\:pending]')
document.querySelectorAll('[lvt-el\\:toggleAttr\\:on\\:pending="disabled"]')
document.querySelectorAll('[lvt-fx\\:scroll]')
document.querySelectorAll('[lvt-mod\\:debounce]')
document.querySelectorAll('[lvt-form\\:preserve]')
```

**Required:** Phase 1 Step 3 creates a shared `lvtSelector(attr, value?)` utility in `utils/lvt-selector.ts`. All `querySelectorAll` calls and chromedp selectors using colon-delimited `lvt-*` attributes must go through this utility — see implementation in Phase 1 Step 3 below. The Phase 1 audit inventories all query sites (audit item 10).

### Progress Tracker

| Sub-phase | Description | Repo | Status | PR |
|-----------|-------------|------|--------|----|
| 1A | Client: generic event router + removals | `client` | NOT STARTED | — |
| 1B | Server: remove `lvt-action` + update docs | `livetemplate` | NOT STARTED | — |
| 2A | lvt: audit + template/Go migration | `lvt` | NOT STARTED | — |
| 2B | lvt: golden files + e2e tests + PR | `lvt` | NOT STARTED | — |
| 3A | tinkerdown: audit + Go/TS migration | `tinkerdown` | NOT STARTED | — |
| 3B | tinkerdown: templates + docs + e2e + PR | `tinkerdown` | NOT STARTED | — |
| 4 | examples: deps + final cross-repo verification | `examples` | NOT STARTED | — |

**After completing each sub-phase:** Update Status to COMPLETE, fill in PR numbers, and commit this file.

**PR merge order:** `client` → `livetemplate` → `lvt` → `tinkerdown` → `examples`. The client must be published first because `lvt` and `tinkerdown` e2e tests load the client library.

---

### Phase 1A: Client — Generic Event Router + Removals

> **Session Context**
>
> - **Prerequisites:** None (this is the first sub-phase)
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/client`
> - **Scope:** Audit client codebase, implement `lvt-on:{event}` router, remove deprecated modules, create `livetemplate.css`, update tests
> - **Key constraints:** Do NOT change `state/change-auto-wirer.ts` (Change() auto-wiring is orthogonal). The `lvt-on:{event}` parser uses ordered greedy consumption — see [Design Summary](#design-summary-quick-reference-for-implementors) for grammar and algorithm.
> - **Outputs:** Committed branch `attr-reduction` on `client` repo with all client changes. PR created. **Do NOT merge yet** — Phase 1B server changes must also be ready.

**Goal:** Implement the generic event router in the TypeScript client, remove all deprecated attribute handling, and ship CSS custom property support.

**Repo:** `livetemplate/client`

#### Step 1: Audit (MANDATORY — do this first)

Before making any changes, deep dive into the client codebase. Update the **Audit Findings** section below with specific findings.

```
cd $REPO_ROOT/client
```

1. Read `dom/event-delegation.ts` end-to-end. Map:
   - Exact line numbers for the `eventTypes` array and attribute lookup pattern
   - How `lvt-data-*` and `lvt-value-*` are extracted (line numbers)
   - Where `lvt-submit` is checked (line numbers)
   - Where `lvt-confirm` is checked (file + line)
   - Where `handleAction()` sends messages — this is the funnel point
2. Read `dom/modal-manager.ts` — understand all exports and imports
3. Read `dom/directives.ts` — map each directive's attribute reads
4. Read `dom/reactive-attributes.ts` — find `disable`/`enable` action handling
5. Read `utils/confirm.ts` — understand `checkLvtConfirm()` and `extractLvtData()`
6. Read `livetemplate-client.ts` — find all imports of modules being modified/removed
7. Run `npm test` to get baseline test count and ensure all pass
8. Search for ALL imports of `modal-manager`, `confirm`, `extractLvtData` across the codebase
9. **Critical:** Map the `setupClickAwayDelegation()` logic — `click-away` is a `CustomEvent` (not a native DOM event). The delegation detects outside-clicks and fires `element.dispatchEvent(new CustomEvent('click-away'))`. This logic must be preserved and adapted; the new attribute is `lvt-on:custom:click-away`.
10. Inventory ALL `querySelectorAll` calls that use `lvt-*` attribute selectors — every one needs colon escaping for the new `lvt-on:` syntax
11. List ALL test files and what they cover

#### Audit Findings (Phase 1A)

<!-- Fill this section during the audit. A future LLM session will read this. -->

**Event delegation (`dom/event-delegation.ts`):**
- `eventTypes` array location: _line TBD_
- `lvt-data-*` extraction: _line TBD_
- `lvt-value-*` extraction: _line TBD_
- `lvt-submit` check: _line TBD_
- `lvt-confirm` check: _file:line TBD_
- `handleAction()` funnel: _line TBD_

**Modal manager:** _exports and import sites TBD_

**Directives (`dom/directives.ts`):**
- Scroll attribute reads: _lines TBD_
- Highlight attribute reads: _lines TBD_
- Animate attribute reads: _lines TBD_

**Reactive attributes:** `disable`/`enable` handling at _line TBD_

**Click-away delegation:** `setupClickAwayDelegation()` at _file:line TBD_

**querySelectorAll sites needing colon escaping:** _count TBD, list TBD_

**Baseline test count:** _TBD_

#### Step 2: Worktree Setup

```bash
cd $REPO_ROOT/client
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Implement Generic Event Router

**File:** `dom/event-delegation.ts`

1. **Implement type/scope/event parser.** Add a helper function that extracts `type`, `scope`, and `event` from an `lvt-on:*` attribute name using the reserved keyword algorithm (see [Grammar](#grammar-lvt-ontypescopeevent)):

   ```ts
   const TYPE_KEYWORDS  = new Set(['custom'])
   const SCOPE_KEYWORDS = new Set(['window', 'document'])

   function parseLvtOn(attr: string): { type: string; scope: string; event: string } {
     const segs = attr.replace('lvt-on:', '').split(':')
     let type  = 'browser', scope = 'element'
     if (TYPE_KEYWORDS.has(segs[0]))  type  = segs.shift()!
     if (SCOPE_KEYWORDS.has(segs[0])) scope = segs.shift()!
     return { type, scope, event: segs.join(':') }
   }
   ```

2. **Create shared `lvtSelector` utility.** Add `utils/lvt-selector.ts`:

   ```ts
   /** Escapes colons in attribute names for use in CSS attribute selectors. */
   export function lvtSelector(attr: string, value?: string): string {
     const escaped = attr.replace(/:/g, '\\:')
     return value !== undefined ? `[${escaped}="${value}"]` : `[${escaped}]`
   }
   ```

   All `querySelectorAll` calls and chromedp selectors using `lvt-on:*` attributes must go through this utility. The audit inventories all query sites (audit item 10).

3. **Route by scope.** In the event loop, after parsing the attribute, determine the `EventTarget`:
   - `scope === 'window'` → `window.addEventListener(event, handler)`
   - `scope === 'document'` → `document.addEventListener(event, handler)`
   - `scope === 'element'` (default) → `element.addEventListener(event, handler)`

4. **Route by type.** For `type === 'custom'`, the client dispatches a `CustomEvent` on the element after running its detection logic. For `click-away`:
   - In `setupClickAwayDelegation()`, change `[lvt-click-away]` selector to `lvtSelector('lvt-on:custom:click-away')` (yields `[lvt-on\\:custom\\:click-away]`)
   - The delegation callback fires `element.dispatchEvent(new CustomEvent('click-away'))`
   - The router then listens via `element.addEventListener('click-away', handler)` normally

5. **Remove `lvt-submit` handling.** Remove the code that checks for `lvt-submit` on forms. Forms route via button `name`, form `name`, or default `"submit"`.

6. **Remove `lvt-data-*` and `lvt-value-*` extraction.** Remove the loops that scan for `lvt-data-*` and `lvt-value-*` attributes on action elements. Data should come from `data-*` attributes or hidden inputs.

7. **Remove `lvt-change` handling.** Remove the special case for `lvt-change` on forms and inputs. The `Change()` auto-wiring (in `state/change-auto-wirer.ts`) is orthogonal and untouched.

#### Step 4: Remove Deprecated Modules

1. **Delete `dom/modal-manager.ts`.** Remove the entire file. Update `livetemplate-client.ts` to remove the import, instantiation, and any `setupModalDelegation()` calls.

2. **Update `utils/confirm.ts`.** Remove `checkLvtConfirm()`. If `extractLvtData()` is only used for `lvt-data-*` extraction, remove it too. Check all imports first.

3. **Update `dom/reactive-attributes.ts`.** Remove `"disable"` and `"enable"` from the reactive action types. Rename all reactive attribute patterns from `lvt-{method}-on:{lifecycle}` → `lvt-el:{method}:on:{lifecycle}`. Users must use `lvt-el:toggleAttr:on:{event}="disabled"` instead of disable/enable sugar.

4. **Update `dom/directives.ts`.** For each directive (scroll, highlight, animate):
   - Rename attribute selectors from `lvt-scroll` → `lvt-fx:scroll`, `lvt-highlight` → `lvt-fx:highlight`, `lvt-animate` → `lvt-fx:animate`
   - Remove reads of `lvt-scroll-behavior`, `lvt-scroll-threshold` attributes
   - Remove reads of `lvt-highlight-color`, `lvt-highlight-duration` attributes
   - Remove reads of `lvt-animate-duration` attribute
   - Instead, read from CSS custom properties via `getComputedStyle(element).getPropertyValue('--lvt-*')`
   - Fall back to hardcoded defaults if CSS property is empty
   - Use `lvtSelector()` for all `querySelectorAll` calls (colons need escaping)

5. **Create `livetemplate.css`.** New file with `:root` defaults for all CSS custom properties:
   ```css
   :root {
     --lvt-scroll-behavior: auto;
     --lvt-scroll-threshold: 100px;
     --lvt-highlight-color: #ffc107;
     --lvt-highlight-duration: 500ms;
     --lvt-animate-duration: 300ms;
   }
   ```
   Add to `package.json` `files` array. Document the import in the client README:
   ```js
   import '@livetemplate/client/livetemplate.css'  // CSS custom property defaults
   ```
   All examples and e2e tests that use scroll, highlight, or animate directives must import this file (or provide equivalent `:root` overrides).

Add a step to rename the prefix-consolidated attributes:

6. **Rename prefix-consolidated attributes.** Throughout the client codebase:
   - `lvt-debounce` → `lvt-mod:debounce`, `lvt-throttle` → `lvt-mod:throttle` (timing modifiers)
   - `lvt-preserve` → `lvt-form:preserve`, `lvt-disable-with` → `lvt-form:disable-with`, `lvt-no-intercept` → `lvt-form:no-intercept` (form behavior)
   - `lvt-addClass-on:*` → `lvt-el:addClass:on:*`, and similarly for all 6 reactive DOM attrs (Category 7 renames)
   - Update all `querySelectorAll` calls, attribute reads, and constants to use the new prefixed names
   - Use `lvtSelector()` for all CSS selector queries involving colons

#### Step 5: Update Tests

Update all test files to use new attribute syntax:

- `tests/event-delegation.test.ts` — update from `lvt-click`, `lvt-keydown` → `lvt-on:{event}` syntax
- `tests/modal-manager.test.ts` — delete this file
- `tests/reactive-attributes.test.ts` — remove `disable`/`enable` test cases
- `tests/directives.test.ts` — update to test `lvt-fx:*` attribute names + CSS custom property reading
- Remove any test that depends on removed `lvt-submit`, `lvt-data-*`, `lvt-value-*`, `lvt-confirm`, `lvt-modal-*` handling

Add new tests:
- `lvt-on:click` routes to named action
- `lvt-on:window:keydown` with `lvt-key` filter works
- `lvt-on:custom:click-away` works (inverted containment, dispatches `CustomEvent`)
- CSS custom property `--lvt-scroll-behavior` is read by `lvt-fx:scroll` directive
- CSS custom property `--lvt-highlight-duration` is read by `lvt-fx:highlight` directive
- `lvt-mod:debounce` modifier applies to `lvt-on:*` events
- `lvt-form:preserve` prevents form auto-reset

```bash
cd $REPO_ROOT/client/.worktrees/attr-reduction
npm test
```

#### Step 6: Acceptance Criteria (Phase 1A)

- [ ] Client: `lvt-on:click`, `lvt-on:input`, `lvt-on:keydown`, etc. all route to server actions correctly
- [ ] Client: `lvt-on:window:keydown` with `lvt-key` filter works
- [ ] Client: `lvt-on:custom:click-away` inverted containment works (dispatches `CustomEvent`)
- [ ] Client: `modal-manager.ts` deleted, no `lvt-modal-open/close` handling
- [ ] Client: No `lvt-data-*`, `lvt-value-*`, `lvt-submit`, `lvt-confirm`, `lvt-change` handling
- [ ] Client: `lvt-disable-on`/`lvt-enable-on` reactive actions removed
- [ ] Client: Reactive DOM uses `lvt-el:*:on:*` prefix (`lvt-el:addClass:on:pending`, `lvt-el:toggleAttr:on:pending`, etc.)
- [ ] Client: Directives use `lvt-fx:*` prefix, read from CSS custom properties, `livetemplate.css` ships with defaults
- [ ] Client: Timing modifiers use `lvt-mod:*` prefix (`lvt-mod:debounce`, `lvt-mod:throttle`)
- [ ] Client: Form behavior uses `lvt-form:*` prefix (`lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept`)
- [ ] Client: All tests pass: `npm test`

#### Step 7: Commit and Create PR

```bash
cd $REPO_ROOT/client/.worktrees/attr-reduction
git add -u && git add livetemplate.css
git commit -m "feat!: generic event router + prefix consolidation + remove deprecated attributes

BREAKING CHANGE: Replaces lvt-click, lvt-keydown, etc. with lvt-on:{event}.
Renames lvt-scroll → lvt-fx:scroll, lvt-highlight → lvt-fx:highlight, lvt-animate → lvt-fx:animate.
Renames lvt-debounce → lvt-mod:debounce, lvt-throttle → lvt-mod:throttle.
Renames lvt-preserve → lvt-form:preserve, lvt-disable-with → lvt-form:disable-with, lvt-no-intercept → lvt-form:no-intercept.
Renames lvt-addClass-on → lvt-el:addClass:on, etc. for all reactive DOM attrs.
Removes lvt-submit, lvt-confirm, lvt-modal-*, lvt-data-*, lvt-value-*,
lvt-change, lvt-disable-on, lvt-enable-on, and directive config attributes.
Adds livetemplate.css with CSS custom property defaults."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: generic event router + prefix consolidation + attribute reduction" \
  --body "Phase 1A of attribute reduction. See livetemplate/livetemplate docs/proposals/attribute-reduction-proposal.md"
```

**⚠️ Do NOT merge this PR yet.** Phase 1B must also be ready. After both PRs are reviewed: merge client PR first → publish new npm version → then merge server PR.

**Update this progress tracker:** Set Phase 1A to COMPLETE, fill in PR number. Record the published npm version: `@livetemplate/client@___`.

---

### Phase 1B: Server — Remove `lvt-action` + Update Docs

> **Session Context**
>
> - **Prerequisites:** Phase 1A PR must exist (but not necessarily merged). Read the Phase 1A Audit Findings above for context on what changed in the client.
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/livetemplate`
> - **Scope:** Audit server `lvt-action` usage, remove it, update all documentation files to new syntax
> - **Key constraints:** Only `internal/send/message.go` and test files reference `lvt-action`. Documentation changes span 3 files. Do NOT modify template parsing — the server doesn't parse `lvt-on:*` attributes (that's client-side).
> - **Outputs:** Committed branch `attr-reduction` on `livetemplate` repo with all server changes. PR created.

**Goal:** Remove `lvt-action` form field parsing from the Go server and update all documentation to the new attribute syntax.

**Repo:** `livetemplate/livetemplate`

#### Step 1: Audit (MANDATORY — do this first)

```
cd $REPO_ROOT/livetemplate
```

1. Grep for `lvt-action` in `internal/send/message.go` — exact line numbers
2. Grep for `lvt-action` in `handle_test.go` — count and list all occurrences
3. Grep for `lvt-action` in `internal/send/message_test.go` — exact test case names
4. Grep for `lvt-` in `action.go`, `template.go` — list all comment references
5. Run `GOWORK=off go test ./... -timeout=300s` to get baseline

#### Audit Findings (Phase 1B)

<!-- Fill this section during the audit. -->

**`internal/send/message.go`:**
- `lvt-action` in `parseURLEncodedForm()`: _line TBD_
- `lvt-action` in `parseMultipartForm()`: _line TBD_
- `actionFields` set: _line TBD_

**`handle_test.go`:** _N occurrences of `form.Set("lvt-action", ...)` at lines TBD_

**`internal/send/message_test.go`:** _test case names TBD_

**Doc comment references in `action.go`, `template.go`:** _lines TBD_

**Baseline test count:** _TBD (all passing: yes/no)_

#### Step 2: Worktree Setup

```bash
cd $REPO_ROOT/livetemplate
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Remove `lvt-action` Parsing

**File:** `internal/send/message.go`

In `parseURLEncodedForm()`: remove the `lvt-action` check from action resolution. Remove `"lvt-action"` from the `actionFields` set. Resolution becomes: `action` field → button name → default `"submit"`.

In `parseMultipartForm()`: remove the `r.FormValue("lvt-action")` check.

**File:** `handle_test.go`

Update all 31 occurrences of `form.Set("lvt-action", "X")` to `form.Set("action", "X")`.

**File:** `internal/send/message_test.go`

Update test cases:
- Change `lvt-action` form field tests to use `action` field
- Remove the "lvt-action takes precedence" test
- Update all remaining `lvt-action` references

**Files:** `action.go`, `template.go`

Update doc comments to remove `lvt-submit`, `lvt-action`, `lvt-data-*` references. Replace with standard HTML patterns.

```bash
cd $REPO_ROOT/livetemplate/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

#### Step 4: Update Documentation

**File:** `docs/references/client-attributes.md`

- Remove the `lvt-submit` entry from Event Bindings
- Remove the `lvt-data-*` / `lvt-value-*` Data Passing section
- Remove the `lvt-confirm` entry from Form Behavior
- Remove the Modals section (`lvt-modal-open`, `lvt-modal-close`)
- Remove `lvt-disable-on` and `lvt-enable-on` from Reactive Attributes
- Remove `lvt-scroll-behavior`, `lvt-scroll-threshold`, `lvt-highlight-color`, `lvt-highlight-duration`, `lvt-animate-duration` from Directives
- Rename directive attributes: `lvt-scroll` → `lvt-fx:scroll`, `lvt-highlight` → `lvt-fx:highlight`, `lvt-animate` → `lvt-fx:animate`
- Rename timing modifiers: `lvt-debounce` → `lvt-mod:debounce`, `lvt-throttle` → `lvt-mod:throttle`
- Rename form behavior: `lvt-preserve` → `lvt-form:preserve`, `lvt-disable-with` → `lvt-form:disable-with`, `lvt-no-intercept` → `lvt-form:no-intercept`
- Rename reactive DOM: `lvt-addClass-on:*` → `lvt-el:addClass:on:*`, and similarly for all 6 reactive DOM attrs (Category 7)
- Replace individual event attribute entries with a single `lvt-on:{event}` section
- Replace individual window event entries with a single `lvt-on:window:{event}` section
- Remove `lvt-change` entry; add note that `Change()` convention handles this automatically
- Update Table of Contents

**File:** `docs/guides/progressive-complexity.md`

- Update Section 13.1 (Event Bindings) to use `lvt-on:{event}` syntax
- Update Section 13.3 (Keyboard Shortcuts) to use `lvt-on:window:keydown`
- Update Section 13.4 (Reactive DOM) to remove `lvt-disable-on`/`lvt-enable-on` examples; rename remaining attrs from `lvt-*-on:` to `lvt-el:*:on:` syntax
- Update Section 13.5 (Directives) to show CSS custom properties instead of `lvt-*-behavior/color/duration` attributes

**File:** `docs/references/progressive-complexity-reference.md`

- Remove `lvt-submit` from Action Resolution Order
- Remove `lvt-modal-open`/`lvt-modal-close` from Dialog Routing
- Update event attribute examples to `lvt-on:{event}` syntax

#### Step 5: Acceptance Criteria (Phase 1B)

- [ ] Server: `lvt-action` form field no longer parsed
- [ ] Server: All tests pass: `GOWORK=off go test ./... -timeout=300s`
- [ ] Server: `docs/references/client-attributes.md` updated with new syntax, deprecated entries removed
- [ ] Server: `docs/guides/progressive-complexity.md` uses `lvt-on:{event}` syntax throughout
- [ ] Server: `docs/references/progressive-complexity-reference.md` updated

#### Step 6: Commit and Create PR

```bash
cd $REPO_ROOT/livetemplate/.worktrees/attr-reduction
git add -u
git commit -m "feat!: remove lvt-action parsing, update docs for attribute reduction (#288)

BREAKING CHANGE: lvt-action form field no longer parsed. Use button name or action field."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction — server + docs (#288)" \
  --body "Phase 1B server-side. Removes lvt-action parsing, updates all documentation."
```

**Merge order:** Client PR first → publish new client version → then server PR.

After both merge, clean up worktrees:
```bash
cd $REPO_ROOT/client && git worktree remove .worktrees/attr-reduction
cd $REPO_ROOT/livetemplate && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 1B to COMPLETE, fill in PR number. Record the published versions:
- `@livetemplate/client@___` (npm)
- `github.com/livetemplate/livetemplate@v___` (Go module)

---

### Phase 2A: lvt — Audit + Template/Go Migration

> **Session Context**
>
> - **Prerequisites:** Phase 1A and 1B PRs must be **merged** and new versions published.
> - **Pre-flight checks:** Run these before starting:
>   ```bash
>   # Verify client is published
>   npm view @livetemplate/client version  # should be >= the Phase 1A version
>   # Verify server module is tagged
>   go list -m github.com/livetemplate/livetemplate@latest  # should be >= the Phase 1B version
>   ```
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/lvt`
> - **Scope:** Audit the lvt codebase, update all component/generator/kit templates and Go code. Do NOT update golden files or tests — that's Phase 2B.
> - **Key constraints:** Apply the [attribute replacement rules](#design-summary-quick-reference-for-implementors) from the Design Summary. Determine the golden file update command during the audit (see audit item 4) and record it in Audit Findings for Phase 2B.
> - **Outputs:** All template and Go code changes committed to the `attr-reduction` branch. Audit findings filled in below.

**Goal:** Audit the lvt codebase and update all component templates, generator templates, kit templates, and Go code to use the new attribute syntax.

**Repo:** `livetemplate/lvt`

**Dependency:** Phase 1A and 1B must be merged and new client version published.

#### Step 1: Audit (MANDATORY — do this first)

Before making any changes, deep dive into the `lvt` codebase to capture the full migration impact.

```
cd $REPO_ROOT/lvt
```

1. **Count and list all occurrences** of each deprecated attribute across the repo:
   ```
   grep -r "lvt-click" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-submit" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-data-" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-change" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-modal-" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-confirm" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-input" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-focus" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-blur" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-keydown" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-mouseenter\|lvt-mouseleave" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-click-away" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   ```

2. **Map the component template structure:**
   - List all `components/*/templates/*.tmpl` files
   - For each component, note which deprecated attributes it uses
   - Identify any Go code that GENERATES HTML with `lvt-*` attributes (e.g., `components/base/action.go`)

3. **Map the generator templates:**
   - List all files in `internal/generator/templates/`
   - List all files in `internal/kits/system/*/`
   - Note which attributes each uses

4. **Map golden files and determine update command:**
   - List all `testdata/golden/*.golden` and `e2e/testdata/golden/*.golden`
   - Determine the golden file update mechanism: check for `-update` flag in test code, or `TestMain` with update logic
   - Record the exact command in the Audit Findings section below

5. **Check `components/base/action.go`:**
   - Does it extract `lvt-data-*` attributes?
   - What's the replacement (standard `data-*`)?
   - Does the Go server's `ActionData` already handle `data-*` attributes from the client?

6. **Check go.mod:**
   - Current livetemplate dependency version
   - Will need bumping to the Phase 1 server version

7. **Run baseline tests:**
   ```
   GOWORK=off go test ./... -timeout=300s
   ```

#### Audit Findings (Phase 2)

<!-- Fill this section during the audit. Phase 2B will read this. -->

**Deprecated attribute counts:**
| Attribute | Files affected |
|-----------|---------------|
| `lvt-click` | _TBD_ |
| `lvt-submit` | _TBD_ |
| `lvt-data-*` | _TBD_ |
| `lvt-change` | _TBD_ |
| `lvt-modal-*` | _TBD_ |
| `lvt-confirm` | _TBD_ |
| `lvt-input` | _TBD_ |
| `lvt-focus` | _TBD_ |
| `lvt-blur` | _TBD_ |
| `lvt-keydown` | _TBD_ |
| `lvt-mouseenter/leave` | _TBD_ |
| `lvt-click-away` | _TBD_ |

**Component template map:** _TBD_

**Go code generating HTML:** _files TBD_

**Golden file update command:** _TBD_

**Baseline test count:** _TBD_

#### Step 2: Worktree Setup

```bash
cd $REPO_ROOT/lvt
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Component Templates

For each component in `components/*/templates/*.tmpl`, apply the **attribute replacement rules** from the [Design Summary](#design-summary-quick-reference-for-implementors) above. This includes event router changes (`lvt-click` → `lvt-on:click`), prefix consolidation (`lvt-debounce` → `lvt-mod:debounce`, `lvt-scroll` → `lvt-fx:scroll`, `lvt-preserve` → `lvt-form:preserve`, etc.), and reactive DOM renames (`lvt-addClass-on:*` → `lvt-el:addClass:on:*`, etc.).

**High-impact components** (from initial exploration — verify during Phase 2 audit):
- `components/modal/templates/default.tmpl` — `lvt-modal-close`, `lvt-keydown`
- `components/dropdown/templates/*.tmpl` — `lvt-click-away`, `lvt-focus`, `lvt-change`
- `components/autocomplete/templates/*.tmpl` — `lvt-input`, `lvt-focus`, `lvt-blur`, `lvt-click-away`
- `components/tooltip/templates/*.tmpl` — `lvt-mouseenter`, `lvt-mouseleave`, `lvt-focus`, `lvt-blur`
- `components/popover/templates/*.tmpl` — `lvt-mouseenter`, `lvt-mouseleave`, `lvt-click-away`
- `components/tagsinput/templates/*.tmpl` — `lvt-keydown`, `lvt-blur`
- `components/toggle/templates/*.tmpl` — `lvt-change`
- `components/datatable/templates/*.tmpl` — `lvt-input`, `lvt-click`
- `components/rating/templates/*.tmpl` — `lvt-mouseleave`, `lvt-mouseover`

#### Step 4: Update Generator Templates and Kit Templates

- `internal/generator/templates/resource/template.tmpl.tmpl` — resource scaffolding
- `internal/generator/templates/components/toolbar.tmpl` — toolbar with `lvt-submit`, `lvt-modal-open`
- `internal/generator/templates/components/form.tmpl` — form with `lvt-modal-close`
- `internal/kits/system/multi/components/form.tmpl` — `lvt-submit`, `lvt-upload`
- `internal/kits/system/multi/templates/resource/template.tmpl.tmpl` — `lvt-confirm`
- `internal/kits/system/single/` — similar patterns

Apply the same replacement rules from Step 3.

#### Step 5: Update Go Code

**File:** `components/base/action.go`

If this file extracts `lvt-data-*` attributes, update to extract `data-*` attributes instead. The server's `ActionData` already handles `data-*` attributes sent by the client.

**File:** `components/styles/unstyled/modal.go` (and other style files)

If Go code generates `lvt-confirm`, `lvt-modal-open`, or `lvt-modal-close` attributes in HTML strings, update to the replacement syntax.

#### Step 6: Update go.mod

```bash
cd $REPO_ROOT/lvt/.worktrees/attr-reduction
go get github.com/livetemplate/livetemplate@latest
go mod tidy
```

#### Step 7: Commit Progress

Commit the template and Go code changes. Do NOT create a PR yet — golden files and tests are Phase 2B.

```bash
cd $REPO_ROOT/lvt/.worktrees/attr-reduction
git add -u
git commit -m "wip: migrate templates and Go code to new attribute syntax"
```

**Update this progress tracker:** Set Phase 2A to COMPLETE.

---

### Phase 2B: lvt — Golden Files + E2E Tests + PR

> **Session Context**
>
> - **Prerequisites:** Phase 2A must be complete (template and Go changes committed to `attr-reduction` branch).
> - **Starting point:** `cd $REPO_ROOT/lvt/.worktrees/attr-reduction` (worktree already exists from 2A)
> - **Scope:** Regenerate golden files, update all e2e tests, verify all tests pass, create PR
> - **Key constraints:** Read the **Audit Findings (Phase 2)** section above for the golden file update command and component map. Chromedp selectors with colon-delimited attributes (`lvt-on:*`, `lvt-fx:*`, `lvt-mod:*`, `lvt-form:*`) require **double backslash escaping** in Go strings: `[lvt-on\\:click="X"]`.
> - **Outputs:** All tests passing, PR created on `lvt` repo.

**Goal:** Regenerate golden files, update e2e tests, verify everything passes, and create the PR.

**Repo:** `livetemplate/lvt`

#### Step 1: Regenerate Golden Files

Golden files in `testdata/golden/` and `e2e/testdata/golden/` will no longer match after template changes. Use the golden file update command recorded in the Phase 2 Audit Findings:

```bash
# Use the command determined during Phase 2A audit, e.g.:
GOWORK=off go test ./... -update  # if tests have an -update flag
# OR manually run the generator and compare
```

If the project doesn't have auto-update for golden files, manually update each golden file to reflect the new attribute syntax.

#### Step 2: Update E2E Tests

E2E test files that assert on HTML content or query elements by `lvt-*` attributes need updating:

- `e2e/complete_workflow_test.go` — update attribute selectors
- `e2e/tutorial_test.go` — update attribute references
- `e2e/delete_multi_post_test.go` — update form submission patterns
- `e2e/modal_test.go` — update modal open/close to `command`/`commandfor`
- `e2e/rendering_test.go` — update attribute expectations

For chromedp selectors:
| Find | Replace |
|------|---------|
| `[lvt-click="X"]` | `[name="X"]` (buttons) or `[lvt-on\\:click="X"]` (non-buttons) |
| `[lvt-submit="X"]` | `[name="X"]` on button, or `form[name="X"]` |
| `[lvt-modal-open="id"]` | `[commandfor="id"]` |

**Note:** Colon in attribute selectors requires escaping in CSS: `[lvt-on\:click="X"]`. In Go strings, this becomes `[lvt-on\\:click="X"]`. This applies to all colon-delimited families: `lvt-on:*`, `lvt-el:*`, `lvt-fx:*`, `lvt-mod:*`, `lvt-form:*`. For reactive DOM attrs: `[lvt-el\\:addClass\\:on\\:pending="loading"]`.

#### Step 3: Run Tests

```bash
cd $REPO_ROOT/lvt/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

For e2e tests (if they require the new client):
```bash
# Ensure the new client version is available (published from Phase 1A)
GOWORK=off go test ./e2e/... -timeout=600s
```

#### Step 4: Acceptance Criteria (Phase 2)

- [ ] Zero occurrences of `lvt-click` in any `.tmpl` file (replaced by `name=` or `lvt-on:click`)
- [ ] Zero occurrences of `lvt-submit` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-data-*` in any `.tmpl` file (replaced by `data-*`)
- [ ] Zero occurrences of `lvt-confirm` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-modal-open` or `lvt-modal-close` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-change` in any `.tmpl` file (replaced by `lvt-on:change` or `lvt-on:input`)
- [ ] All `lvt-input`, `lvt-keydown`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave` replaced by `lvt-on:{event}` equivalents; `lvt-click-away` replaced by `lvt-on:custom:click-away`
- [ ] Golden files regenerated and matching
- [ ] All Go tests pass: `GOWORK=off go test ./... -timeout=300s`
- [ ] E2E tests pass: `GOWORK=off go test ./e2e/... -timeout=600s`
- [ ] `go.mod` updated to latest livetemplate version

#### Step 5: PR and Merge

```bash
cd $REPO_ROOT/lvt/.worktrees/attr-reduction
git add -u
git commit -m "feat!: migrate to generic event router (lvt-on:{event}), remove deprecated attributes

BREAKING CHANGE: All component templates updated to use lvt-on:{event} syntax.
lvt-submit, lvt-confirm, lvt-modal-*, lvt-data-*, lvt-change removed."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction — migrate all templates" \
  --body "Phase 2. Updates all component/generator/kit templates to new syntax."
```

After merge:
```bash
cd $REPO_ROOT/lvt && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 2B to COMPLETE, fill in PR number.

---

### Phase 3A: tinkerdown — Audit + Go/TS Migration

> **Session Context**
>
> - **Prerequisites:** Phase 1A and 1B must be **merged** and published.
> - **Pre-flight checks:**
>   ```bash
>   npm view @livetemplate/client version  # should be >= the Phase 1A version
>   go list -m github.com/livetemplate/livetemplate@latest  # should be >= the Phase 1B version
>   ```
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/tinkerdown`
> - **Scope:** Audit tinkerdown, update Go code that generates HTML, update TypeScript client
> - **Key constraints:** Apply the [attribute replacement rules](#design-summary-quick-reference-for-implementors).
>
> ⚠️ **DO NOT CHANGE these tinkerdown-specific attributes** — they are parsed by tinkerdown's own Go code, not the LiveTemplate client library:
> `lvt-source`, `lvt-columns`, `lvt-field`, `lvt-actions`, `lvt-empty`, `lvt-datatable`

**Goal:** Audit the tinkerdown codebase and update Go code that generates HTML and the TypeScript client.

**Repo:** `livetemplate/tinkerdown`

**Dependency:** Phase 1A and 1B must be merged. Phase 2 is independent (tinkerdown doesn't depend on lvt).

#### Step 1: Audit (MANDATORY — do this first)

Deep dive into the tinkerdown codebase to capture the full migration impact.

```
cd $REPO_ROOT/tinkerdown
```

1. **Go code that GENERATES lvt-* attributes:**
   - Read `auto_tables.go` — find all `lvt-click`, `lvt-submit`, `lvt-confirm`, `lvt-data-*`, `lvt-reset-on:success` (now `lvt-el:reset:on:success`) in string literals
   - Read `auto_tasks.go` — same analysis
   - Read `page.go` — find all generated `lvt-click`, `lvt-data-*` in table action buttons
   - These are the most critical files — they programmatically build HTML

2. **TypeScript client:**
   - Read `client/src/blocks/interactive-block.ts` — how does it handle `lvt-click`, `lvt-submit`, `lvt-change`, `lvt-confirm`, `lvt-data-*`?
   - Does it import from `@livetemplate/client` or implement its own handling?
   - What needs to change for `lvt-on:{event}` syntax?

3. **Templates (scaffold + examples):**
   - List all `cmd/tinkerdown/commands/templates/*/index.md` files
   - List all `examples/*/index.md` files
   - Count `lvt-*` attribute usage in each

4. **Documentation:**
   - Read `docs/reference/lvt-attributes.md` — this needs a full rewrite
   - Read `skills/tinkerdown/SKILL.md` and `skills/tinkerdown/reference.md`
   - Read `README.md` — attribute references

5. **E2E tests:**
   - List all `*_e2e_test.go` files
   - Count `lvt-*` references in each
   - Note: these tests likely use chromedp with attribute selectors

6. **Tinkerdown-specific attributes (NOT affected):**
   - Confirm `lvt-source`, `lvt-columns`, `lvt-field`, `lvt-actions`, `lvt-empty`, `lvt-datatable` are parsed by tinkerdown's own Go code, not the client library
   - These must NOT be changed

7. **Check go.mod and client/package.json:**
   - Current livetemplate and client dependency versions
   - What versions to bump to

8. **Run baseline tests:**
   ```
   GOWORK=off go test ./... -timeout=300s
   ```

#### Audit Findings (Phase 3)

<!-- Fill this section during the audit. Phase 3B will read this. -->

**Go code generating HTML:**
- `auto_tables.go`: _attributes found TBD_
- `auto_tasks.go`: _attributes found TBD_
- `page.go`: _attributes found TBD_

**TypeScript client (`interactive-block.ts`):**
- Imports from `@livetemplate/client`: _TBD_
- Own handling of `lvt-*`: _TBD_

**Template/example count:** _N files with deprecated attrs TBD_

**E2E test count:** _N files with `lvt-*` refs TBD_

**Tinkerdown-specific attrs confirmed separate:** _yes/no_

**Baseline test count:** _TBD_

#### Step 2: Worktree Setup

```bash
cd $REPO_ROOT/tinkerdown
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Go Code That Generates HTML

**File:** `auto_tables.go`

This file generates CRUD table HTML with `lvt-click`, `lvt-submit`, `lvt-confirm`, `lvt-data-id`, `lvt-reset-on:success` (→ `lvt-el:reset:on:success`). Update all string literals:

| Find in string literal | Replace |
|------------------------|---------|
| `lvt-click="Refresh"` | `name="Refresh"` (if on button) or `lvt-on:click="Refresh"` |
| `lvt-click="Edit"` | `name="Edit"` (on button) |
| `lvt-click="Delete"` | `name="Delete"` (on button) |
| `lvt-click="CancelEdit"` | `name="CancelEdit"` (on button) |
| `lvt-submit="Add"` | `name="Add"` (on button) or `<form name="Add">` |
| `lvt-submit="Update"` | `name="Update"` (on button) |
| `lvt-confirm="Delete this item?"` | `onclick="return confirm('Delete this item?')"` |
| `lvt-data-id="{{.Id}}"` | `data-id="{{.Id}}"` |
| `lvt-reset-on:success` | `lvt-el:reset:on:success` (renamed per Category 7) |

**File:** `auto_tasks.go`

Same pattern as `auto_tables.go`:
| Find | Replace |
|------|---------|
| `lvt-click="Toggle"` | `name="Toggle"` |
| `lvt-submit="Add"` | `name="Add"` |
| `lvt-data-id="{{.Id}}"` | `data-id="{{.Id}}"` |

**File:** `page.go`

This file has regex parsing for `lvt-actions` (tinkerdown-specific, keep) and generates action buttons. Update the button generation to use `name=` instead of `lvt-click=`, and `data-id=` instead of `lvt-data-id=`.

#### Step 4: Update TypeScript Client

**File:** `client/src/blocks/interactive-block.ts`

Update event attribute detection:
- Change `lvt-click` lookups to `lvt-on:click` or `name` on buttons
- Change `lvt-submit` lookups to form `name` or button `name`
- Change `lvt-change` lookups to `lvt-on:change`
- Remove `lvt-confirm` handling (use inline `onclick` pattern instead)
- Change `lvt-data-*` extraction to `data-*` extraction

If this file imports `checkLvtConfirm` or `extractLvtData` from `@livetemplate/client`, those imports need updating or removing.

#### Step 5: Update Dependencies

```bash
cd $REPO_ROOT/tinkerdown/.worktrees/attr-reduction
go get github.com/livetemplate/livetemplate@latest
cd client && npm install @livetemplate/client@latest
```

#### Step 6: Commit Progress

Commit the Go and TS changes. Templates, docs, and tests are Phase 3B.

```bash
cd $REPO_ROOT/tinkerdown/.worktrees/attr-reduction
git add -u
git commit -m "wip: migrate Go generators and TS client to new attribute syntax"
```

**Update this progress tracker:** Set Phase 3A to COMPLETE.

---

### Phase 3B: tinkerdown — Templates + Docs + E2E + PR

> **Session Context**
>
> - **Prerequisites:** Phase 3A must be complete (Go and TS changes committed to `attr-reduction` branch).
> - **Starting point:** `cd $REPO_ROOT/tinkerdown/.worktrees/attr-reduction` (worktree already exists from 3A)
> - **Scope:** Update scaffold/example templates, rewrite docs, update e2e tests, create PR
> - **Key constraints:** Read the **Audit Findings (Phase 3)** section above. Apply the [attribute replacement rules](#design-summary-quick-reference-for-implementors).
>
> ⚠️ **DO NOT CHANGE:** `lvt-source`, `lvt-columns`, `lvt-field`, `lvt-actions`, `lvt-empty`, `lvt-datatable` — these are tinkerdown-specific.

**Goal:** Update all templates, documentation, and e2e tests, then create the PR.

**Repo:** `livetemplate/tinkerdown`

#### Step 1: Update Templates and Examples

Apply the replacement rules to all:
- `cmd/tinkerdown/commands/templates/*/index.md` — scaffold templates
- `examples/*/index.md` — example applications

Key files:
- `templates/todo/index.md` — `lvt-submit`, `lvt-click`, `lvt-data-id`
- `templates/form/index.md` — `lvt-submit`, `lvt-click`, `lvt-data-id`
- `templates/tutorial/index.md` — `lvt-click`, `lvt-change`
- `examples/action-buttons/index.md` — `lvt-click`, `lvt-submit`
- `examples/expense-tracker/index.md` — full CRUD
- All `lvt-source`, `lvt-columns` references remain UNCHANGED

#### Step 2: Update Documentation

**File:** `docs/reference/lvt-attributes.md`

Rewrite to reflect:
- `lvt-on:{event}` replaces individual event attributes
- `lvt-on:window:{event}` replaces window event attributes
- Standard HTML replaces `lvt-submit`, `lvt-data-*`, `lvt-confirm`, `lvt-modal-*`
- Tinkerdown-specific attributes (`lvt-source`, `lvt-columns`, etc.) documented separately

**Files:** `skills/tinkerdown/SKILL.md`, `skills/tinkerdown/reference.md`, `README.md`

Update attribute references and examples throughout.

#### Step 3: Update E2E Tests

All `*_e2e_test.go` files need attribute selector updates:
- `auto_tables_e2e_test.go`
- `auto_tasks_test.go`
- `action_buttons_e2e_test.go`
- `component_library_e2e_test.go`
- `execargs_e2e_test.go`
- `lvtsource_*_e2e_test.go` files

Selector replacement patterns:
| Find | Replace |
|------|---------|
| `[lvt-click="X"]` | `button[name="X"]` or `[lvt-on\\:click="X"]` |
| `[lvt-submit="X"]` | `button[name="X"]` or `form[name="X"]` |

#### Step 4: Run Tests

```bash
cd $REPO_ROOT/tinkerdown/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

#### Step 5: Acceptance Criteria (Phase 3)

- [ ] Zero `lvt-click` in Go code string literals (replaced by `name=` on buttons)
- [ ] Zero `lvt-submit` in Go code or templates (replaced by button/form `name`)
- [ ] Zero `lvt-data-*` in Go code or templates (replaced by `data-*`)
- [ ] Zero `lvt-confirm` in Go code or templates (replaced by `onclick`)
- [ ] Zero `lvt-change` in templates (replaced by `lvt-on:change` or `lvt-on:input`)
- [ ] Tinkerdown-specific attributes (`lvt-source`, `lvt-columns`, etc.) UNCHANGED
- [ ] `docs/reference/lvt-attributes.md` fully updated
- [ ] TypeScript client handles new attribute syntax
- [ ] All Go tests pass: `GOWORK=off go test ./... -timeout=300s`
- [ ] All e2e tests pass
- [ ] Dependencies bumped to Phase 1 versions

#### Step 6: PR and Merge

```bash
cd $REPO_ROOT/tinkerdown/.worktrees/attr-reduction
git add -u
git commit -m "feat!: migrate to generic event router, remove deprecated lvt-* attributes

BREAKING CHANGE: Auto-generated HTML uses standard button name routing and
lvt-on:{event} syntax. lvt-submit, lvt-data-*, lvt-confirm removed."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction migration" \
  --body "Phase 3. Updates Go generators, templates, client, docs, and tests."
```

After merge:
```bash
cd $REPO_ROOT/tinkerdown && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 3B to COMPLETE, fill in PR number.

---

### Phase 4: Examples Repo + Final Verification

> **Session Context**
>
> - **Prerequisites:** Phases 1A, 1B, 2B, and 3B must all be **merged** and published.
> - **Pre-flight checks:**
>   ```bash
>   npm view @livetemplate/client version  # Phase 1A version
>   go list -m github.com/livetemplate/livetemplate@latest  # Phase 1B version
>   go list -m github.com/livetemplate/lvt@latest  # Phase 2B version
>   ```
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/examples`
> - **Scope:** Bump dependencies, update docs, run cross-repo verification
> - **Key constraints:** The `examples` repo likely has zero deprecated attributes in templates (already uses Tier 1 patterns). Main work is dependency bumping and documentation updates.
> - **Outputs:** All 5 repos passing tests. Final PR on `examples` repo.

**Goal:** Update the examples repository (minimal changes) and verify all repos work together end-to-end.

**Repo:** `livetemplate/examples`

**Dependency:** Phases 1A, 1B, 2B, and 3B must be merged.

#### Step 1: Audit (MANDATORY — do this first)

```
cd $REPO_ROOT/examples
```

1. Confirm which `lvt-*` attributes are in actual template files (not just docs):
   - Expected: `lvt-fx:scroll`, `lvt-upload`, `lvt-form:preserve`, `lvt-form:no-intercept`, `lvt-el:*:on:*` (all Tier 2)
   - Verify zero deprecated attributes in templates

2. Check `go.mod` — dependency version

3. Check docs/README for references to deprecated attributes

4. Run baseline tests:
   ```
   GOWORK=off go test ./... -timeout=300s
   ```

#### Step 2: Worktree Setup

```bash
cd $REPO_ROOT/examples
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Dependencies

```bash
cd $REPO_ROOT/examples/.worktrees/attr-reduction
go get github.com/livetemplate/livetemplate@latest
go get github.com/livetemplate/lvt@latest
go get github.com/livetemplate/lvt/components@latest
go mod tidy
```

#### Step 4: Update Documentation

Update `README.md` and any `CLAUDE.md` references to use the new attribute syntax in code examples.

#### Step 5: Run Tests

```bash
GOWORK=off go test ./... -timeout=300s
```

#### Step 6: Final Cross-Repo Verification

With ALL worktrees (or merged changes), verify end-to-end:

```bash
# 1. Client tests
cd $REPO_ROOT/client && npm test

# 2. Core library tests
cd $REPO_ROOT/livetemplate && GOWORK=off go test ./... -timeout=300s

# 3. lvt tests (including e2e)
cd $REPO_ROOT/lvt && GOWORK=off go test ./... -timeout=300s

# 4. tinkerdown tests (including e2e)
cd $REPO_ROOT/tinkerdown && GOWORK=off go test ./... -timeout=300s

# 5. examples tests
cd $REPO_ROOT/examples && GOWORK=off go test ./... -timeout=300s
```

All 5 repos must have passing tests.

#### Step 7: Acceptance Criteria

- [ ] `examples` repo has zero deprecated attribute references in templates
- [ ] `examples` `go.mod` points to latest livetemplate version
- [ ] All 5 repos pass their test suites
- [ ] No remaining references to deprecated attributes across any repo (verify with grep)

#### Step 8: PR and Merge

```bash
cd $REPO_ROOT/examples/.worktrees/attr-reduction
git add -u
git commit -m "chore: bump dependencies for attribute reduction"
git push origin attr-reduction
gh pr create --head attr-reduction --title "chore: bump deps for attribute reduction" \
  --body "Phase 4. Updates dependencies. No template changes needed (examples already use Tier 1 patterns)."
```

After merge:
```bash
cd $REPO_ROOT/examples && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 4 to COMPLETE, fill in PR number.

---

### Cross-Phase Dependency Graph

```
Phase 1A: client ──→ Phase 1B: livetemplate (server + docs)
    |                     |
    ├─────────────────────┤
    |                     |
    ├──→ Phase 2A: lvt (audit + templates + Go) ──→ Phase 2B: lvt (golden + e2e + PR) ──┐
    |                                                                                     ├──→ Phase 4
    └──→ Phase 3A: tinkerdown (audit + Go + TS) ──→ Phase 3B: tinkerdown (docs + e2e) ──┘
```

**Parallelism:**
- 1A → 1B is sequential (server needs client to be ready, but PRs can be prepared in parallel)
- 2A/2B and 3A/3B can proceed in parallel (lvt and tinkerdown don't depend on each other)
- Within each repo, A → B is sequential (B depends on A's changes)
- Phase 4 depends on 2B + 3B being merged

**Total: 7 sub-phases across ~7 LLM sessions** (1A, 1B, 2A, 2B, 3A, 3B, 4).

### PR Merge Order

1. **`client`** (Phase 1A) — merge and publish new npm version
2. **`livetemplate`** (Phase 1B) — merge and tag new Go version
3. **`lvt`** (Phase 2B) — depends on new client + server versions
4. **`tinkerdown`** (Phase 3B) — depends on new client + server versions
5. **`examples`** (Phase 4) — depends on all above

### CI Considerations

- `lvt` CI runs e2e tests with chromedp that load the client library. The new client version must be published to npm/CDN BEFORE the lvt PR can pass CI.
- `tinkerdown` CI similarly needs the published client.
- If repos use `go.work` locally, worktrees must use `GOWORK=off` for testing.
- Golden file updates in `lvt` will show large diffs (every template changed) — this is expected.

## References

- [Progressive Complexity Proposal](progressive-complexity-proposal.md) — Foundation for Tier 1/Tier 2 model
- [Client Attributes Reference](../references/client-attributes.md) — Current complete attribute listing
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Quick reference
- [Tier 1 File Uploads Proposal](tier1-file-uploads-proposal.md) — Moving basic file uploads to standard HTML
- [HTML Invoker Commands](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/command) — `command`/`commandfor` spec
