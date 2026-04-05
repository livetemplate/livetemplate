# Attribute Surface Reduction

**Status:** In Progress — Phases 1–2 Complete
**Date:** 2026-03-30 (last updated: 2026-04-05)
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

### `lvt-modal-open` / `lvt-modal-close` — Eliminate

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

**Rationale:** The HTML Invoker Commands API (`command`/`commandfor`) handles modal open/close natively when used with `<dialog>`. Focus trapping, backdrop, and Escape key are all browser-native with `<dialog>` + `showModal()`.

> **Implementation Update (2026-04-05):** The `command`/`commandfor` approach was initially implemented in lvt PR #292 but caused 14 e2e test failures because the Invoker Commands API requires Chrome 135+, which CI Docker containers don't support. The actual implementation uses `data-lvt-target` cross-element targeting (client PR [#53](https://github.com/livetemplate/client/pull/53)):
>
> ```html
> <button lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#edit-modal">Edit</button>
> <div id="edit-modal" hidden>
>     <button lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#edit-modal">Cancel</button>
> </div>
> ```
>
> `data-lvt-target` is a general mechanism: any `lvt-el:` method can target another element via `#id` (by ID) or `closest:selector` (walk up ancestors). Falls back to self when absent. This solves both modal open/close and dropdown/popover toggle buttons with zero new methods.
>
> **Note:** The `<div hidden>` approach does **not** provide the browser-native features that `<dialog>` + `showModal()` offers: no automatic backdrop, focus trapping, Escape key handling, or top-layer stacking context. Do not use `role="dialog"` on a `<div>` toggle — without focus trapping and `aria-modal`, it is ARIA misuse. Applications requiring proper modal dialog behavior should use `<dialog>` elements with a JS `lvt-hook` to call `.showModal()`, or wait for broader Invoker Commands API support. The `data-lvt-target` + `hidden` toggle is suitable for simple show/hide panels and the lvt component library's modal component (which provides its own CSS backdrop and JS focus management).

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
| `lvt-confirm` | Eliminate | `onsubmit="return confirm('...')"` (**CSP note:** inline handlers require `'unsafe-inline'` in `script-src`. For strict-CSP apps, a CSP-safe alternative is not yet available — tracked in [#298](https://github.com/livetemplate/lvt/issues/298). Potential future approach: a framework directive like `lvt-form:confirm="message"` processed by the client script.) |
| `lvt-modal-open` | Eliminate | `lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#id"` (actual; `command`/`commandfor` requires Chrome 135+) |
| `lvt-modal-close` | Eliminate | `lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#id"` (actual) |
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

`lvt-disable-on:{lifecycle}` and `lvt-enable-on:{lifecycle}` are syntactic sugar for `lvt-el:toggleAttr:on:[{action}:]{state}="disabled"`, which already exists:

**Before:**
```html
<button lvt-disable-on:pending lvt-enable-on:done>Save</button>
```

**After:**
```html
<button lvt-el:toggleAttr:on:pending="disabled" lvt-el:toggleAttr:on:done="disabled">Save</button>
```

No CSS dependency needed — this is pure attribute consolidation.

> **Migration note:** `lvt-disable-on:*` and `lvt-enable-on:*` (and their renamed forms `lvt-el:disable:on:*` / `lvt-el:enable:on:*`) are removed with no alias. Use `lvt-el:toggleAttr:on:pending="disabled"` and `lvt-el:toggleAttr:on:done="disabled"` instead. The `toggleAttr` approach assumes the `disabled` attribute is absent initially — if the button starts disabled (server-rendered), the toggle fires in the wrong direction. For deterministic behavior on pre-disabled elements, use `lvt-el:setAttr:on:pending="disabled"` and `lvt-el:removeAttr:on:done="disabled"` (when available) instead of `toggleAttr`.

### Summary of Consolidations

| Before | After | Reduction |
|--------|-------|-----------|
| `lvt-scroll`, `lvt-scroll-behavior`, `lvt-scroll-threshold` | `lvt-fx:scroll` + CSS custom properties | 3 → 1 |
| `lvt-highlight`, `lvt-highlight-color`, `lvt-highlight-duration` | `lvt-fx:highlight` + CSS custom properties | 3 → 1 |
| `lvt-animate`, `lvt-animate-duration` | `lvt-fx:animate` + CSS custom properties | 2 → 1 |
| `lvt-disable-on:{lifecycle}`, `lvt-enable-on:{lifecycle}` | `lvt-el:toggleAttr:on:[{action}:]{state}="disabled"` | 2 → 0 |

**Impact: 7 attributes removed via consolidation.**

> **Why not eliminate the main directive attributes too?** The main attributes (`lvt-fx:scroll`, `lvt-fx:highlight`, `lvt-fx:animate`) serve as **discovery markers** — the client finds elements via `querySelectorAll('[lvt-fx\\:scroll]')`. CSS custom properties cannot be queried this way; there is no selector for "elements with `--lvt-scroll` set." Scanning every DOM element with `getComputedStyle()` would be prohibitively expensive. The current split — HTML attribute for behavior declaration (discoverable), CSS custom properties for configuration (readable via `getComputedStyle`) — is the optimal abstraction boundary.

## Category 3: Behaviors That Require Tier 2

These behaviors cannot be expressed with standard HTML. They are the **essential `lvt-*` surface**.

> **Note:** This section identifies the *behaviors* that must remain in Tier 2. Category 5 below consolidates many of these individual attribute names into the generic `lvt-on[:{scope}]:{event}` pattern. Category 6 groups the remaining flat attributes under `lvt-fx:`, `lvt-mod:`, and `lvt-form:` prefixes. Category 7 renames reactive DOM attributes under `lvt-el:` and moves `click-away` to the `lvt-el:` trigger system. The behaviors remain, but the attribute names change.

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
| `lvt-click-away` | Click outside element — no HTML native. *Moved to `lvt-el:` family as a client-side interaction trigger (Category 7). See [`lvt-el:` click-away](#click-away-as-lvt-el-trigger).* |

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
| `lvt-addClass-on:{lifecycle}` → `lvt-el:addClass:on:[{action}:]{state\|interaction}` | Add CSS class on pending/success/error/done — optionally scoped to a specific action |
| `lvt-removeClass-on:{lifecycle}` → `lvt-el:removeClass:on:[{action}:]{state\|interaction}` | Remove CSS class on action lifecycle state |
| `lvt-toggleClass-on:{lifecycle}` → `lvt-el:toggleClass:on:[{action}:]{state\|interaction}` | Toggle CSS class on action lifecycle state |
| `lvt-setAttr-on:{lifecycle}` → `lvt-el:setAttr:on:[{action}:]{state\|interaction}` | Set attribute on action lifecycle state |
| `lvt-toggleAttr-on:{lifecycle}` → `lvt-el:toggleAttr:on:[{action}:]{state\|interaction}` | Toggle boolean attribute (subsumes `disable-on`/`enable-on`) |
| `lvt-reset-on:{lifecycle}` → `lvt-el:reset:on:[{action}:]{state\|interaction}` | Reset form on action lifecycle state |

### Form Behavior
| Attribute | Why |
|-----------|-----|
| `lvt-preserve` → `lvt-form:preserve` | Prevent form auto-reset — opposite of default framework behavior |
| `lvt-disable-with` → `lvt-form:disable-with` | Button text swap + disable during pending — no HTML pattern |
| `lvt-no-intercept` → `lvt-form:no-intercept` (forms) / `lvt-nav:no-intercept` (links) | Opt-out of SPA interception — `lvt-form:` for forms, `lvt-nav:` for links |
| (new) `lvt-form:action` | Explicit action routing attribute on `<form>` — replaces reserved `action` field |

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

### Grammar: `lvt-on[:{scope}]:{event}`

The full attribute syntax is `lvt-on[:{scope}]:{event}="action"`. `scope` is an optional prefix; omitting it selects the default (element-scoped). All events routed through `lvt-on:` are native browser events.

**Scope** — the `EventTarget` to attach the listener to (DOM Living Standard names):

| Scope keyword | `EventTarget` | Use cases |
|---|---|---|
| (omitted) | `Element` (default) | click, input, keydown, focus, blur — the common case |
| `window` | `Window` | resize, scroll, global keyboard shortcuts |
| `document` | `Document` | visibilitychange, selectionchange, fullscreenchange |

**Type** — how the event is dispatched (DOM Living Standard event interfaces):

All events routed through `lvt-on:` are native browser-dispatched events (`isTrusted: true`). Non-native interactions like `click-away` are handled by `lvt-el:` instead (see [Category 7](#click-away-as-lvt-el-trigger)).

**Why no explicit `native` keyword:** "Native" is not a DOM specification term. Browser-dispatched events are simply "events" in the DOM Living Standard — omitting the type prefix is the web-standard default. Adding a `native` keyword would be a non-standard invention.

**Reserved keywords:** `window`, `document` must not be used as bare event names within the `lvt-on:` namespace.

**Parser algorithm** (ordered greedy consumption):

```
// Reserved scope keywords — no ambiguity
scopeKeywords = { 'window', 'document' }

segments = rest.split(':')   // rest = everything after "lvt-on:"
scope = 'element'            // default

if segments[0] in scopeKeywords: scope = segments.shift()
event = segments.join(':')       // remaining; join preserves hyphenated names
```

Examples: `lvt-on:click` → (element, click) · `lvt-on:window:keydown` → (window, keydown) · `lvt-on:document:visibilitychange` → (document, visibilitychange)

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

<!-- After: lvt-on: routes events to server, lvt-el: handles client-side click-away -->
<div class="dropdown {{if .DropdownOpen}}open{{end}}"
     lvt-el:removeClass:on:click-away="open">
  <input lvt-on:input="search" lvt-on:focus="open" lvt-on:blur="close" lvt-mod:debounce="300">
  <ul>
    {{range .Results}}
      <li lvt-on:click="select" data-id="{{.ID}}">{{.Name}}</li>
    {{end}}
  </ul>
</div>
```

**Pattern:** The dropdown is always rendered in the DOM. Visibility is controlled by the `open` CSS class (e.g., `.dropdown ul { display: none; } .dropdown.open ul { display: block; }`). Server actions `Open()` and `Search()` set `DropdownOpen=true` which adds the `open` class. `Close()` sets `DropdownOpen=false` which removes the `open` class. `lvt-el:removeClass:on:click-away="open"` removes the class client-side without a server round-trip — the next server response will reconcile the state.

**Note:** `lvt-on:click` is only for non-button elements. Buttons use standard HTML `<button name="action">`.

**Note on namespace distinction:** `lvt-el:{method}:on:[{action}:]{state|interaction}` (reactive DOM) and `lvt-on:{event}` (event routing) both use `on` with colon separators but are semantically distinct. `lvt-el:addClass:on:pending` (or `lvt-el:addClass:on:save:pending`) reacts to the server action request-response lifecycle and manipulates the DOM client-side. `lvt-on:click` routes native DOM events to server actions. The `lvt-el:` prefix disambiguates them.

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
| Element-scoped events → `lvt-on:{event}` | 9 (`lvt-click`, `lvt-input`, `lvt-keydown`, `lvt-keyup`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-mouseover`) |
| Window-scoped events → `lvt-on:window:{event}` | 6 (`lvt-window-keydown`, `lvt-window-keyup`, `lvt-window-scroll`, `lvt-window-resize`, `lvt-window-focus`, `lvt-window-blur`) |
| `lvt-change` deprecated entirely | 1 |
| `lvt-click-away` → `lvt-el:*:on:click-away` | 1 (moved to reactive DOM family) |
| **Total** | **17 attribute names removed, replaced by 1 generic event pattern (`lvt-on[:{scope}]:{event}`) + 1 click-away trigger on `lvt-el:`** |

### Modifiers Unchanged

`lvt-mod:debounce`, `lvt-mod:throttle`, and `lvt-key` continue to work alongside the generic router:

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

**`lvt-form:` — Form Behavior (4 attributes)**

| Before | After |
|--------|-------|
| `lvt-preserve` | `lvt-form:preserve` |
| `lvt-disable-with="Saving..."` | `lvt-form:disable-with="Saving..."` |
| `lvt-no-intercept` (on forms) | `lvt-form:no-intercept` |
| (new) | `lvt-form:action="X"` — explicit action routing on `<form>`, highest priority |

**`lvt-nav:` — Navigation Behavior (1 attribute)**

| Before | After |
|--------|-------|
| `lvt-no-intercept` (on links) | `lvt-nav:no-intercept` |

Rationale: `lvt-no-intercept` was used on both `<form>` and `<a>` tags, but these are semantically different: form interception vs. link interception. The `lvt-form:` prefix signals form processing, while `lvt-nav:` signals navigation behavior. This prevents the confusion of seeing `lvt-form:no-intercept` on an anchor tag.

**Not grouped:**
- `lvt-key` — event filter, not a timing modifier. Unlike `lvt-mod:debounce`/`lvt-mod:throttle` (which modify *when/how often* an action fires), `lvt-key` determines *whether* the event fires at all (key filter). Semantically different from modifiers.
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
| `lvt-on:` | `lvt-on[:{scope}]:{event}` | 1 generic | Event routing to server actions |
| `lvt-el:` | `lvt-el:addClass:on:pending`, `lvt-el:addClass:on:save:pending`, etc. | 6 named | Reactive DOM — lifecycle states + interactions |
| `lvt-fx:` | `lvt-fx:scroll`, `lvt-fx:highlight`, `lvt-fx:animate` | 3 named | Visual effects and DOM behaviors |
| `lvt-mod:` | `lvt-mod:debounce`, `lvt-mod:throttle` | 2 named | Event timing modifiers |
| `lvt-form:` | `lvt-form:action`, `lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept` | 4 named | Form behavior overrides + routing |
| `lvt-nav:` | `lvt-nav:no-intercept` | 1 named | Navigation interception opt-out |
| (flat) | `lvt-key` | 1 named | Key filter |
| (flat) | `lvt-upload` | 1 named | Custom drop zones |

**Total: 6 prefix families + 2 standalone = 8 attribute concepts (down from 12)**

### Complete Example After All Consolidations

```html
<!-- Event routing (lvt-on:) -->
<input lvt-on:input="search" lvt-mod:debounce="300">
<div lvt-on:window:keydown="shortcut" lvt-key="/">

<!-- Client-side click-away (lvt-el:) -->
<div class="dropdown open" lvt-el:removeClass:on:click-away="open">

<!-- Visual effects (lvt-fx:) -->
<div lvt-fx:scroll="bottom-sticky"
     style="--lvt-scroll-behavior: smooth; --lvt-scroll-threshold: 100px;">
<div lvt-fx:highlight="flash"
     style="--lvt-highlight-color: #ffc107; --lvt-highlight-duration: 500ms;">
<li lvt-fx:animate="fade"
    style="--lvt-animate-duration: 300ms;">

<!-- Form behavior (lvt-form:) -->
<form lvt-form:action="checkout" lvt-form:no-intercept>
<input lvt-form:preserve>
<button lvt-form:disable-with="Saving...">Save</button>

<!-- Navigation behavior (lvt-nav:) -->
<a href="/legacy-page" lvt-nav:no-intercept>Legacy Page</a>

<!-- Reactive DOM (lvt-el:) — unscoped triggers (any action) -->
<button lvt-on:click="save"
        lvt-el:toggleAttr:on:pending="disabled"
        lvt-el:addClass:on:pending="opacity-50">
  Save
</button>

<!-- Reactive DOM (lvt-el:) — action-scoped triggers -->
<div>
  <button lvt-on:click="save">Save</button>
  <button lvt-on:click="delete">Delete</button>
  <span lvt-el:addClass:on:save:pending="saving">Saving...</span>
  <span lvt-el:addClass:on:delete:pending="deleting">Deleting...</span>
</div>

<!-- Reactive DOM (lvt-el:) — multi-action shorthand (server-expanded) -->
<div lvt-el:addClass:on:[save,delete]:pending="opacity-50">
  <button lvt-on:click="save">Save</button>
  <button lvt-on:click="delete">Delete</button>
</div>

<!-- Upload (standalone) -->
<div lvt-upload="documents" class="drop-zone">Drop files here</div>
```

| Category | Before | After |
|----------|--------|-------|
| **Total `lvt-*` surface** | ~51 | ~16 named attributes across 5 prefix families + 2 standalone + 1 generic event pattern |
| Event bindings (element + window) | 16 individual names + 1 (`lvt-change`) | `lvt-on[:{scope}]:{event}` (1 pattern, 3 scope variants) |
| Event bindings (document) | 0 | `lvt-on:document:{event}` (new capability) |
| Data passing | 2 patterns (`lvt-data-*`, `lvt-value-*`) | 0 (standard HTML) |
| Modals | 2 | 0 (`data-lvt-target` + `lvt-el:toggleAttr:on:click="hidden"`) |
| Legacy routing | 2 (`lvt-submit`, `lvt-action`) | 0 (standard HTML) |
| Confirmation | 1 | 0 (standard `onsubmit`/`onclick`) |
| Scroll config | 3 (`lvt-scroll` + 2 config attrs) | 1 (`lvt-fx:scroll`) + CSS custom properties |
| Highlight config | 3 (`lvt-highlight` + 2 config attrs) | 1 (`lvt-fx:highlight`) + CSS custom properties |
| Animation config | 2 (`lvt-animate` + 1 config attr) | 1 (`lvt-fx:animate`) + CSS custom properties |
| Enable/disable sugar | 2 | 0 (use `lvt-el:toggleAttr:on`) |
| Upload | 1 | 1 (`lvt-upload`, narrowed: Tier 1 for basic uploads via standard HTML, Tier 2 for custom drop zones) |
| Timing modifiers | 2 (`lvt-debounce`, `lvt-throttle`) | 2 (`lvt-mod:debounce`, `lvt-mod:throttle`) |
| Key filter | 1 (`lvt-key`) | 1 (unchanged) |
| Reactive DOM | 6 (`lvt-*-on:{lifecycle}`) | 6 methods × triggers (`lvt-el:{method}:on:[{action}:]{state\|interaction}`) — states: pending/success/error/done; interactions: click-away |
| Form behavior | 3 (`lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept`) | 4 (`lvt-form:action`, `lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept`) + 1 (`lvt-nav:no-intercept`) |

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
| `lvt-addClass-on:{lifecycle}` | `lvt-el:addClass:on:[{action}:]{state\|interaction}` |
| `lvt-removeClass-on:{lifecycle}` | `lvt-el:removeClass:on:[{action}:]{state\|interaction}` |
| `lvt-toggleClass-on:{lifecycle}` | `lvt-el:toggleClass:on:[{action}:]{state\|interaction}` |
| `lvt-setAttr-on:{lifecycle}` | `lvt-el:setAttr:on:[{action}:]{state\|interaction}` |
| `lvt-toggleAttr-on:{lifecycle}` | `lvt-el:toggleAttr:on:[{action}:]{state\|interaction}` |
| `lvt-reset-on:{lifecycle}` | `lvt-el:reset:on:[{action}:]{state\|interaction}` |

**Why `lvt-el:`?** "el" = Element. All 6 methods are DOM Element operations (`classList.add()`, `setAttribute()`, `toggleAttribute()`, `HTMLFormElement.reset()`). Short, unambiguous, maps directly to DOM vocabulary.

**camelCase note:** The method segment uses camelCase (`addClass`, not `add-class`) to match the DOM API naming convention. This is consistent with the existing attribute names — they already use camelCase.

**Trigger events** — three categories:

| Category | Pattern | Example | Scope |
|----------|---------|---------|-------|
| Unscoped lifecycle | `:on:{state}` | `lvt-el:addClass:on:pending` | Any `lvt-on:` action on this element |
| Action-scoped lifecycle | `:on:{action}:{state}` | `lvt-el:addClass:on:save:pending` | Only the named action |
| Multi-action lifecycle | `:on:[{a},{b}]:{state}` | `lvt-el:addClass:on:[save,delete]:pending` | Multiple named actions (server-expanded) |
| Interaction | `:on:{interaction}` | `lvt-el:removeClass:on:click-away` | Client-side detection |

**State keywords** (closed set): `pending`, `success`, `error`, `done`
**Interaction keywords** (closed set, extensible): `click-away`

### Lifecycle Trigger Binding: What Is "Pending"?

The lifecycle states (`pending`, `success`, `error`, `done`) represent the **server action request-response cycle**. When an `lvt-on:` event fires an action, the request goes through these phases:

```
User clicks button         →  "pending"   (request in flight)
Server responds OK         →  "success"   (action completed successfully)
Server responds with error →  "error"     (action failed)
After success OR error     →  "done"      (request settled, regardless of outcome)
```

**Binding rule:** Lifecycle triggers observe the `lvt-on:` action on the **same element** (or the nearest ancestor with an `lvt-on:` action). The binding is implicit — no wiring needed:

```html
<!-- "pending" fires when the "save" action (from lvt-on:click) is in flight -->
<button lvt-on:click="save"
        lvt-el:addClass:on:pending="opacity-50"
        lvt-el:toggleAttr:on:pending="disabled">
  Save
</button>
```

**Action-scoped triggers** make the binding explicit by naming the action:

```html
<!-- Only react to the "save" action, not other actions on this element -->
<button lvt-on:click="save"
        lvt-el:addClass:on:save:pending="opacity-50">

<!-- Different indicators for different actions -->
<div>
  <button lvt-on:click="save">Save</button>
  <button lvt-on:click="delete">Delete</button>
  <span lvt-el:addClass:on:save:pending="saving-spinner">Saving...</span>
  <span lvt-el:addClass:on:delete:pending="deleting-spinner">Deleting...</span>
</div>
```

**Multi-action shorthand** — when the same DOM manipulation applies to multiple actions:

```html
<!-- Shorthand: applies to both save and delete -->
<div lvt-el:addClass:on:[save,delete]:pending="opacity-50">

<!-- Server expands to individual attributes before sending to browser: -->
<div lvt-el:addClass:on:save:pending="opacity-50"
     lvt-el:addClass:on:delete:pending="opacity-50">
```

The `[action1,action2]` bracket syntax is expanded **server-side** during template rendering. The TypeScript client only sees single-action attributes — no bracket parsing needed on the client. This keeps the wire format clean and the client simple.

**Unscoped = wildcard:** Omitting the action name (`on:pending`) means "match any action on this element." This is the default and most common pattern — explicit action scoping is only needed when multiple actions share an element and need different reactive behavior.

### Unified Grammar

With `lvt-el:` adopted, every attribute family follows the same structural pattern:

```
lvt-{family}:{member}[:on:[{action}:]{trigger}]="value"
```

| Family | Grammar | `:on:` trigger | Status |
|--------|---------|---------------|--------|
| `lvt-on:` | `lvt-on[:{scope}]:{event}="action"` | IS the event (not a suffix) | v1 |
| `lvt-el:` | `lvt-el:{method}:on:[{action}:]{state\|interaction}="value"` | **Required** — specifies when to manipulate the element | v1 |
| `lvt-fx:` | `lvt-fx:{effect}[:on:[{action}:]{state}][="config"]` | **Optional** — without `:on:`, activates on every DOM content change (implicit trigger) | v1 |
| `lvt-mod:` | `lvt-mod:{modifier}="value"` | Not applicable — modifies sibling `lvt-on:*` | v1 |
| `lvt-form:` | `lvt-form:{behavior}[:on:[{action}:]{state}][="value"]` | **Optional** — without `:on:`, always active (implicit trigger) | v1 |
| `lvt-nav:` | `lvt-nav:{behavior}` | Not applicable — opt-out flag | v1 |

The `:on:` suffix is a universal **trigger mechanism**. It answers the question: "When should this behavior activate?" For `lvt-el:`, the trigger is always explicit and required — it can be a lifecycle state (`pending`/`success`/`error`/`done`), optionally scoped to a specific action name, or an interaction event (`click-away`). For `lvt-fx:` and `lvt-form:`, the trigger is optional — omitting it preserves backward-compatible implicit activation (effects on DOM update, form behaviors always on). When specified, the behavior becomes conditional on the named lifecycle state.

### Click-Away as `lvt-el:` Trigger

`click-away` is handled entirely by `lvt-el:` — not `lvt-on:`. This keeps `lvt-on:` clean as a "native DOM events → server" pipeline.

**Why click-away belongs in `lvt-el:`:**
1. Click-away is NOT a native DOM event — it's synthesized by the client (document-level click listener checking `!element.contains(event.target)`)
2. Most click-away use cases (dropdowns, popovers, toasts) are ephemeral UI where CSS-class-based visibility avoids unnecessary server round-trips
3. `lvt-on:` becomes purely about routing browser events to server action methods — no `custom` type complexity
4. The detection logic (inverted containment) runs client-side regardless — `lvt-el:` just keeps the response client-side too

**Usage:**
```html
<!-- Toggle CSS class on click-away (no server round-trip) -->
<div class="dropdown open" lvt-el:removeClass:on:click-away="open">
  <ul>...</ul>
</div>

<!-- Multiple methods on click-away -->
<div class="dropdown open"
     lvt-el:removeClass:on:click-away="open"
     lvt-el:setAttr:on:click-away="aria-expanded" data-value="false">
```

**CSS visibility pattern:** The dropdown is always rendered in the DOM. Visibility is controlled by the `open` CSS class:
```css
.dropdown ul { display: none; }
.dropdown.open ul { display: block; }
```

**Server state reconciliation:** When a server action later runs (e.g., the user types in the search field), the server re-renders the template. If the server's `DropdownOpen` state differs from the client's class state, the server's rendered HTML takes precedence — the diff engine reconciles automatically.

**If you need server-routed click-away:** Use `lvt-on:document:click` with client-side containment checking in the server action. This is an intentional escape hatch, not the default pattern.

### Parser: `lvt-el:{method}:on:[{action}:]{state|interaction}`

```
methodKeywords      = { 'addClass', 'removeClass', 'toggleClass', 'setAttr', 'toggleAttr', 'reset' }
stateKeywords       = { 'pending', 'success', 'error', 'done' }
interactionKeywords = { 'click-away' }

// After matching "lvt-el:" prefix:
segments = rest.split(':')                          // e.g. "addClass:on:save:pending"
method = segments[0]                                // "addClass"
assert segments[1] == 'on'                          // literal ":on:" separator
assert method in methodKeywords

remaining = segments[2:]                            // ["save", "pending"] or ["pending"] or ["click-away"]

if remaining[0] in stateKeywords:
    // Unscoped lifecycle — matches any action
    action = '*'
    state  = remaining[0]                           // "pending"
elif remaining[0] in interactionKeywords:
    // Interaction trigger — client-side only
    interaction = remaining[0]                      // "click-away"
else:
    // Action-scoped lifecycle
    action = remaining[0]                           // "save"
    state  = remaining[1]                           // "pending"
    assert state in stateKeywords
```

**No ambiguity:** State keywords (`pending`, `success`, `error`, `done`), interaction keywords (`click-away`), and action names are all disjoint sets. Action names are developer-chosen Go method names — they cannot collide with reserved keywords.

**Server-side bracket expansion:** Before the template reaches the client, the Go library expands multi-action attributes:
```
lvt-el:addClass:on:[save,delete]:pending="X"
→ lvt-el:addClass:on:save:pending="X"  +  lvt-el:addClass:on:delete:pending="X"
```
This happens during HTML rendering, so the client parser never sees bracket syntax.

### Optional `:on:` Triggers for `lvt-fx:` and `lvt-form:`

The `:on:` trigger suffix works across `lvt-fx:` and `lvt-form:` families, making them reactive to action lifecycle states. Without `:on:`, the behavior uses its implicit trigger (backward compatible). With `:on:`, it becomes conditional.

**`lvt-fx:` with `:on:` — conditional effects:**

```html
<!-- Default: highlight on any content change (implicit trigger) -->
<div lvt-fx:highlight="flash">

<!-- Highlight only on success lifecycle -->
<div lvt-fx:highlight:on:success="flash">

<!-- Action-scoped: highlight only when "save" succeeds -->
<div lvt-fx:highlight:on:save:success="flash">

<!-- Different effects for different lifecycle states -->
<div lvt-fx:highlight:on:success="flash" lvt-fx:highlight:on:error="shake">
```

**`lvt-form:` with `:on:` — conditional form behavior:**

```html
<!-- Default: always preserve form state (implicit trigger) -->
<input lvt-form:preserve>

<!-- Preserve only on error (form resets on success) -->
<input lvt-form:preserve:on:error>

<!-- Action-scoped: preserve only when "update" errors -->
<input lvt-form:preserve:on:update:error>
```

**Server-expanded bracket syntax also works:**

```html
<!-- Highlight on success of either save or update -->
<div lvt-fx:highlight:on:[save,update]:success="flash">
<!-- Server expands to: -->
<!-- lvt-fx:highlight:on:save:success="flash" lvt-fx:highlight:on:update:success="flash" -->
```

**Why NOT `lvt-mod:` with `:on:`?** Event modifiers (debounce, throttle) are **adverbs** — they modify how an event handler dispatches, not when they activate. Debounce doesn't "fire on pending" — it alters the timing of event delivery. The modifier's target is the sibling `lvt-on:*` attribute on the same element, which is already implicit and unambiguous for the common case.

### Final Attribute Surface

After all reductions (Categories 1–7), the complete `lvt-*` surface is:

**Named attributes (18) in 6 prefix families + 2 standalone:**

| # | Attribute | Family |
|---|-----------|--------|
| 1 | `lvt-mod:debounce` | `lvt-mod:` (event modifier) |
| 2 | `lvt-mod:throttle` | `lvt-mod:` (event modifier) |
| 3 | `lvt-key` | standalone (key filter) |
| 4 | `lvt-el:addClass:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 5 | `lvt-el:removeClass:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 6 | `lvt-el:toggleClass:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 7 | `lvt-el:setAttr:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 8 | `lvt-el:toggleAttr:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 9 | `lvt-el:reset:on:[{action}:]{state\|interaction}` | `lvt-el:` (reactive DOM) |
| 10 | `lvt-form:action` | `lvt-form:` (form behavior) |
| 11 | `lvt-form:preserve[:on:[{action}:]{state}]` | `lvt-form:` (form behavior) |
| 12 | `lvt-form:disable-with[:on:[{action}:]{state}]` | `lvt-form:` (form behavior) |
| 13 | `lvt-form:no-intercept[:on:[{action}:]{state}]` | `lvt-form:` (form behavior) |
| 14 | `lvt-nav:no-intercept` | `lvt-nav:` (navigation) |
| 15 | `lvt-fx:scroll[:on:[{action}:]{state}]` | `lvt-fx:` (visual effect) |
| 16 | `lvt-fx:highlight[:on:[{action}:]{state}]` | `lvt-fx:` (visual effect) |
| 17 | `lvt-fx:animate[:on:[{action}:]{state}]` | `lvt-fx:` (visual effect) |
| 18 | `lvt-upload` | standalone (upload) |

**Generic event pattern (1):**

`lvt-on[:{scope}]:{event}` — replaces 16 individual event attributes with 1 pattern supporting 3 scope variants (element, window, document). All events are native browser events; `click-away` is handled by `lvt-el:` instead.

---

## Implementation Plan

> **Pre-requisite added during Phase 2A/2B:** The component open/close client-side refactor (replacing server-side `lvt-click-away` with `lvt-el:*:on:click` / `lvt-el:*:on:click-away` interaction triggers) was completed as part of lvt PR #292. This refactor was a pre-requisite for Phase 2A/2B because lvt components (dropdowns, comboboxes, etc.) relied on server-dispatched click-away events that needed to become client-side `lvt-el:` interactions. The client-side `lvt-el:` click-away interaction trigger was implemented in client PR #44 (Phase 1A). DOM event triggers (`on:click`, `on:mouseenter`, `on:focusin`, `on:focusout`, etc.) were added in client PR #49 (Phase 1A.1).
>
> **Additional client work during Phase 2A/2B:**
> - Client PR [#49](https://github.com/livetemplate/client/pull/49): DOM event triggers for `lvt-el:` and `lvt-fx:` — extended client to support `on:click`, `on:mouseenter`, etc. as `lvt-el:` triggers
> - Client PR [#53](https://github.com/livetemplate/client/pull/53): `data-lvt-target` cross-element targeting — general mechanism allowing any `lvt-el:` method to operate on a different element via `#id` or `closest:selector`. This replaced `command`/`commandfor` for modal open/close (Chrome 135+ not available in CI) and also eliminated all remaining `onclick` toggle handlers in dropdown/popover/datepicker/timepicker/menu components.
>
> **Design deviation — modals:** The proposal specified `command="show-modal" commandfor="id"` for modal open/close, but CI Docker containers don't support the Invoker Commands API (Chrome 135+). The actual implementation uses `lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#modal-id"` — a framework-level solution that works on all browsers. See the [modal section above](#lvt-modal-open--lvt-modal-close--replace-with-native-dialog) for details.
>
> **Design deviation — delete confirm modals:** Server-managed 3-step delete flows (RequestDelete → ConfirmDelete → CancelDelete) were replaced entirely with browser-native `onclick="return confirm('...')"`. This eliminated ~760 lines of server state management code (PendingDeleteID, DeleteConfirm template rendering, confirm modal components).

### Design Summary (Quick Reference for Implementors)

This section recaps the key design decisions from Categories 1-7 so each implementation phase is self-contained.

**Grammar:** `lvt-on[:{scope}]:{event}="action"` (event routing) · `lvt-el:{method}:on:[{action}:]{state|interaction}="value"` (reactive DOM) · `lvt-fx:{effect}[:on:[{action}:]{state}][="config"]` (visual effects) · `lvt-form:{behavior}[:on:[{action}:]{state}][="value"]` (form behavior) · `lvt-nav:{behavior}` (navigation)

| Segment | Values | Default |
|---------|--------|---------|
| `scope` | (omitted) = element, `window`, `document` | element |
| `event` | Any native DOM event name | — |
| `action` | (omitted) = any action, or specific action name | any |
| `state` | `pending`, `success`, `error`, `done` | — |
| `interaction` | `click-away` | — |

**`lvt-el:` trigger binding:** Lifecycle states (`pending`/`success`/`error`/`done`) observe the server action request-response cycle of the `lvt-on:` action on the same element (or nearest ancestor). Unscoped (`:on:pending`) matches any action; action-scoped (`:on:save:pending`) matches only the named action. Multi-action shorthand (`:on:[save,delete]:pending`) is expanded server-side to individual attributes.

**Parser algorithm** (ordered greedy consumption):
```
scopeKeywords = { 'window', 'document' }
segments = rest.split(':')           // rest = everything after "lvt-on:"
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
| `lvt-modal-open="id"` | `lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#id"` (actual impl; `command`/`commandfor` requires Chrome 135+) |
| `lvt-modal-close="id"` | `lvt-el:toggleAttr:on:click="hidden" data-lvt-target="#id"` (actual impl; see implementation note above) |
| `lvt-change="X"` | `lvt-on:change="X"` (select/checkbox) or `lvt-on:input="X"` (text) |
| `lvt-input="X"` | `lvt-on:input="X"` |
| `lvt-keydown="X"` | `lvt-on:keydown="X"` |
| `lvt-keyup="X"` | `lvt-on:keyup="X"` |
| `lvt-focus="X"` | `lvt-on:focus="X"` |
| `lvt-blur="X"` | `lvt-on:blur="X"` |
| `lvt-mouseenter="X"` | `lvt-on:mouseenter="X"` |
| `lvt-mouseleave="X"` | `lvt-on:mouseleave="X"` |
| `lvt-mouseover="X"` | `lvt-on:mouseover="X"` |
| `lvt-click-away="X"` | `lvt-el:removeClass:on:click-away="X"` (or other `lvt-el:` method) |
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
| `lvt-no-intercept` (on forms) | `lvt-form:no-intercept` |
| `lvt-no-intercept` (on links) | `lvt-nav:no-intercept` |
| `lvt-addClass-on:{lifecycle}` | `lvt-el:addClass:on:[{action}:]{state}` |
| `lvt-removeClass-on:{lifecycle}` | `lvt-el:removeClass:on:[{action}:]{state}` |
| `lvt-toggleClass-on:{lifecycle}` | `lvt-el:toggleClass:on:[{action}:]{state}` |
| `lvt-setAttr-on:{lifecycle}` | `lvt-el:setAttr:on:[{action}:]{state}` |
| `lvt-toggleAttr-on:{lifecycle}` | `lvt-el:toggleAttr:on:[{action}:]{state}` |
| `lvt-reset-on:{lifecycle}` | `lvt-el:reset:on:[{action}:]{state}` |
| `lvt-disable-on:{lifecycle}` | Removed — use `lvt-el:toggleAttr:on:[{action}:]{state}="disabled"` |
| `lvt-enable-on:{lifecycle}` | Removed — use `lvt-el:toggleAttr:on:[{action}:]{state}="disabled"` (inverse lifecycle state) |
| `lvt-scroll-behavior` | Removed — use CSS custom property `--lvt-scroll-behavior` |
| `lvt-scroll-threshold` | Removed — use CSS custom property `--lvt-scroll-threshold` |
| `lvt-highlight-color` | Removed — use CSS custom property `--lvt-highlight-color` |
| `lvt-highlight-duration` | Removed — use CSS custom property `--lvt-highlight-duration` |
| `lvt-animate-duration` | Removed — use CSS custom property `--lvt-animate-duration` |

**CSS selector escaping:** Colons in `lvt-on:*`, `lvt-el:*`, `lvt-fx:*`, `lvt-mod:*`, `lvt-form:*`, and `lvt-nav:*` must be escaped in CSS selectors. Use the `lvtSelector(attr, value?)` utility (Phase 1 Step 3) for all `querySelectorAll` calls.

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

This plan skips the deprecation-warning phase from the original migration path. The library and TypeScript client are consumed only by the `lvt`, `tinkerdown`, and `examples` repositories — all maintained in the same organization. All consuming repos are updated atomically in Phases 2-4. This eliminates the runtime warning phase. Since `@livetemplate/client` is **unreleased with no external consumers**, this is a **minor/patch version bump** — not a major semver break. A changelog entry documenting all attribute changes is still recommended for internal tracking. If the client gains significant external adoption before this executes, reinstate a deprecation phase before the next breaking change.

### CSS Selector Escaping Convention

The `lvt-on:{event}`, `lvt-el:{method}:on:[{action}:]{state|interaction}`, `lvt-fx:{effect}`, `lvt-mod:{modifier}`, and `lvt-form:{behavior}` syntax introduces colons in HTML attribute names. Colons must be escaped in CSS selectors:

```ts
// Wrong — unescaped colons
document.querySelectorAll('[lvt-on:click="X"]')
document.querySelectorAll('[lvt-el:addClass:on:pending]')
document.querySelectorAll('[lvt-el:removeClass:on:click-away]')
document.querySelectorAll('[lvt-fx:scroll]')

// Correct — escaped colons
document.querySelectorAll('[lvt-on\\:click="X"]')
document.querySelectorAll('[lvt-on\\:window\\:keydown="X"]')
document.querySelectorAll('[lvt-el\\:addClass\\:on\\:pending]')
document.querySelectorAll('[lvt-el\\:addClass\\:on\\:save\\:pending]')  // action-scoped
document.querySelectorAll('[lvt-el\\:toggleAttr\\:on\\:pending="disabled"]')
document.querySelectorAll('[lvt-el\\:removeClass\\:on\\:click-away="open"]')
document.querySelectorAll('[lvt-fx\\:scroll]')
document.querySelectorAll('[lvt-mod\\:debounce]')
document.querySelectorAll('[lvt-form\\:preserve]')
```

**Required:** Phase 1 Step 3 creates a shared `lvtSelector(attr, value?)` utility in `utils/lvt-selector.ts`. All `querySelectorAll` calls and chromedp selectors using colon-delimited `lvt-*` attributes must go through this utility — see implementation in Phase 1 Step 3 below. The Phase 1 audit inventories all query sites (audit item 10).

### Progress Tracker

> **Execution order:** The rows below are listed in the required execution sequence — **not** alphanumeric order. Each row depends on all rows above it being complete (unless the dependency graph below explicitly allows parallelism). See [Cross-Phase Dependency Graph](#cross-phase-dependency-graph) for which phases can run in parallel.

| # | Sub-phase | Description | Repo | Status | PR |
|---|-----------|-------------|------|--------|----|
| 1 | 1A | Client: generic event router + removals | `client` | COMPLETE | [#44](https://github.com/livetemplate/client/pull/44) |
| 1.5 | 1A.1 | Client: DOM event triggers for `lvt-el:` and `lvt-fx:` (unplanned — needed for Phase 2 component migration) | `client` | COMPLETE | [#49](https://github.com/livetemplate/client/pull/49) |
| 1.6 | 1A.2 | Client: `data-lvt-target` cross-element targeting (unplanned — replaced `command`/`commandfor` for modal open/close) | `client` | COMPLETE | [client#53](https://github.com/livetemplate/client/pull/53) |
| 2 | 1B | Server: remove `lvt-action` + update docs | `livetemplate` | COMPLETE | [#322](https://github.com/livetemplate/livetemplate/pull/322) |
| 3 | 2E | Examples: early migration + manual review | `examples` | COMPLETE | [examples#53](https://github.com/livetemplate/examples/pull/53) |
| 4 | 2A | lvt: audit + template/Go migration | `lvt` | COMPLETE | [#292](https://github.com/livetemplate/lvt/pull/292) |
| 5 | 2B | lvt: golden files + e2e tests + PR | `lvt` | COMPLETE | [#292](https://github.com/livetemplate/lvt/pull/292) |
| 6 | 3A | tinkerdown: audit + Go/TS migration | `tinkerdown` | NOT STARTED | — |
| 7 | 3B | tinkerdown: templates + docs + e2e + PR | `tinkerdown` | NOT STARTED | — |
| 8 | 4 | Final cross-repo verification + dep alignment | `examples` | NOT STARTED | — |

**After completing each sub-phase:** Update Status to COMPLETE, fill in PR numbers, and commit this file.

**PR merge order:** `client` → `livetemplate` → `examples (2E)` → `lvt` → `tinkerdown` → `examples (4)`. See [PR Merge Order](#pr-merge-order) for details. The client must be published first because `lvt` and `tinkerdown` e2e tests load the client library.

**Known regressions (introduced in Phase 2, must fix before Phase 3):** These functional regressions were shipped in lvt PR #292 (Phase 2) and identified during bot review. They are already in `main` and must be fixed in follow-up PRs before proceeding with the tinkerdown migration (Phase 3):

| Issue | Description | Severity |
|-------|-------------|----------|
| [#293](https://github.com/livetemplate/lvt/issues/293) | `hidden` attribute on datepicker/timepicker panels conflicts with CSS `.open` toggle — panels never become visible | Bug |
| [#294](https://github.com/livetemplate/lvt/issues/294) | Autocomplete panel doesn't close after suggestion selection | Bug |
| [#295](https://github.com/livetemplate/lvt/issues/295) | `ContextMenu.ShowAt()` no longer opens the menu — silent breaking change | Bug |
| [#296](https://github.com/livetemplate/lvt/issues/296) | `NewInline()` datepicker renders hidden by default | Bug |
| [#298](https://github.com/livetemplate/lvt/issues/298) | Inline `onclick` handlers require `'unsafe-inline'` in CSP — regression for strict-CSP apps | Breaking |
| [#299](https://github.com/livetemplate/lvt/issues/299) | `WithOpen(true)` is a no-op with no migration path — silent breaking change | Breaking |

**Additional follow-up issues (lower priority):** 11 more issues ([#297](https://github.com/livetemplate/lvt/issues/297), [#300](https://github.com/livetemplate/lvt/issues/300)–[#309](https://github.com/livetemplate/lvt/issues/309)) were created covering accessibility (`aria-expanded` reset), code quality (stale docstrings, duplicate styles, godoc markers), and minor UX items (chevron indicators, tab-away behavior).

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
9. **Critical:** Map the `setupClickAwayDelegation()` logic — `click-away` is NOT a native DOM event. The delegation detects outside-clicks via inverted containment. This logic is preserved but moved to `lvt-el:` — the new attribute is `lvt-el:removeClass:on:click-away` (or other `lvt-el:` methods). The detection mechanism stays the same; only the response changes from server action dispatch to client-side DOM manipulation.
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

1. **Implement scope/event parser.** Add a helper function that extracts `scope` and `event` from an `lvt-on:*` attribute name using the reserved keyword algorithm (see [Grammar](#grammar-lvt-onscopeevent)):

   ```ts
   const SCOPE_KEYWORDS = new Set(['window', 'document'])

   function parseLvtOn(attr: string): { scope: string; event: string } {
     const segs = attr.replace('lvt-on:', '').split(':')
     let scope = 'element'
     if (SCOPE_KEYWORDS.has(segs[0])) scope = segs.shift()!
     return { scope, event: segs.join(':') }
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

4. **Implement click-away in `lvt-el:` directive handler.** Move click-away from `lvt-on:` (where it was `custom` type) to `lvt-el:` (where it becomes an interaction trigger alongside lifecycle triggers):
   - In `setupClickAwayDelegation()`, change `[lvt-click-away]` selector to `lvtSelector('lvt-el:removeClass:on:click-away')` (or scan for any `lvt-el:*:on:click-away` attributes)
   - The delegation callback no longer dispatches a `CustomEvent` — instead, it directly manipulates the DOM (e.g., `element.classList.remove(value)`)
   - The trigger `click-away` in the `lvt-el:` parser is handled alongside lifecycle triggers (`pending`, `success`, `error`, `done`)

5. **Remove `lvt-submit` handling.** Remove the code that checks for `lvt-submit` on forms. Forms route via button `name`, form `name`, or default `"submit"`.

6. **Remove `lvt-data-*` and `lvt-value-*` extraction.** Remove the loops that scan for `lvt-data-*` and `lvt-value-*` attributes on action elements. Data should come from `data-*` attributes or hidden inputs.

7. **Remove `lvt-change` handling.** Remove the special case for `lvt-change` on forms and inputs. The `Change()` auto-wiring (in `state/change-auto-wirer.ts`) is orthogonal and untouched.

#### Step 4: Remove Deprecated Modules

1. **Delete `dom/modal-manager.ts`.** Remove the entire file. Update `livetemplate-client.ts` to remove the import, instantiation, and any `setupModalDelegation()` calls.

2. **Update `utils/confirm.ts`.** Remove `checkLvtConfirm()`. If `extractLvtData()` is only used for `lvt-data-*` extraction, remove it too. Check all imports first.

3. **Update `dom/reactive-attributes.ts`.** Remove `"disable"` and `"enable"` from the reactive action types. Rename all reactive attribute patterns from `lvt-{method}-on:{lifecycle}` → `lvt-el:{method}:on:[{action}:]{state}`. Implement the action-scoped trigger parser:
   - Parse `:on:pending` as unscoped (any action)
   - Parse `:on:save:pending` as action-scoped (only "save" action)
   - State keywords: `pending`, `success`, `error`, `done`
   - Interaction keywords: `click-away`
   - Users must use `lvt-el:toggleAttr:on:[{action}:]{state}="disabled"` instead of disable/enable sugar.

4. **Update `dom/directives.ts`.** For each directive (scroll, highlight, animate):
   - Rename attribute selectors from `lvt-scroll` → `lvt-fx:scroll`, `lvt-highlight` → `lvt-fx:highlight`, `lvt-animate` → `lvt-fx:animate`
   - Remove reads of `lvt-scroll-behavior`, `lvt-scroll-threshold` attributes
   - Remove reads of `lvt-highlight-color`, `lvt-highlight-duration` attributes
   - Remove reads of `lvt-animate-duration` attribute
   - Instead, read from CSS custom properties via `getComputedStyle(element).getPropertyValue('--lvt-*')`
   - Fall back to hardcoded defaults if CSS property is empty
   - Use `lvtSelector()` for all `querySelectorAll` calls (colons need escaping)
   - **Implement optional `:on:` trigger parsing** for `lvt-fx:` attributes. The parser should:
     - Match `lvt-fx:{effect}` (no `:on:`) → implicit trigger (activate on DOM content change, as before)
     - Match `lvt-fx:{effect}:on:{state}` → activate only when lifecycle reaches that state
     - Match `lvt-fx:{effect}:on:{action}:{state}` → activate only when named action reaches that state
     - Reuse the same trigger state keywords as `lvt-el:`: `pending`, `success`, `error`, `done`

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
   - `lvt-preserve` → `lvt-form:preserve`, `lvt-disable-with` → `lvt-form:disable-with`, `lvt-no-intercept` → `lvt-form:no-intercept` (form behavior on forms), `lvt-no-intercept` → `lvt-nav:no-intercept` (link interception opt-out)
   - `lvt-addClass-on:*` → `lvt-el:addClass:on:*`, and similarly for all 6 reactive DOM attrs (Category 7 renames)
   - Update all `querySelectorAll` calls, attribute reads, and constants to use the new prefixed names
   - Use `lvtSelector()` for all CSS selector queries involving colons
   - **Implement optional `:on:` trigger parsing** for `lvt-form:` attributes. The parser should:
     - Match `lvt-form:{behavior}` (no `:on:`) → always active (implicit trigger, as before)
     - Match `lvt-form:{behavior}:on:{state}` → active only in that lifecycle state
     - Match `lvt-form:{behavior}:on:{action}:{state}` → active only for named action in that state
     - Reuse the same trigger infrastructure as `lvt-el:` and `lvt-fx:`

#### Step 5: Update Tests

Update all test files to use new attribute syntax:

- `tests/event-delegation.test.ts` — update from `lvt-click`, `lvt-keydown` → `lvt-on:{event}` syntax
- `tests/modal-manager.test.ts` — delete this file
- `tests/reactive-attributes.test.ts` — remove `disable`/`enable` test cases
- `tests/directives.test.ts` — update to test `lvt-fx:*` attribute names + CSS custom property reading + optional `:on:` trigger parsing (both implicit and explicit triggers)
- Remove any test that depends on removed `lvt-submit`, `lvt-data-*`, `lvt-value-*`, `lvt-confirm`, `lvt-modal-*` handling

Add new tests:
- `lvt-on:click` routes to named action
- `lvt-on:window:keydown` with `lvt-key` filter works
- `lvt-el:removeClass:on:click-away` works (inverted containment, client-side class removal)
- `lvt-el:addClass:on:pending` unscoped trigger fires for any action
- `lvt-el:addClass:on:save:pending` action-scoped trigger fires only for "save" action
- CSS custom property `--lvt-scroll-behavior` is read by `lvt-fx:scroll` directive
- CSS custom property `--lvt-highlight-duration` is read by `lvt-fx:highlight` directive
- `lvt-mod:debounce` modifier applies to `lvt-on:*` events
- `lvt-form:preserve` prevents form auto-reset

```bash
cd $REPO_ROOT/client/.worktrees/attr-reduction
npm test
```

#### Step 6: Acceptance Criteria (Phase 1A)

- [x] Client: `lvt-on:click`, `lvt-on:input`, `lvt-on:change`, `lvt-on:keydown`, etc. all route to server actions correctly
- [x] Client: `lvt-on:window:keydown` with `lvt-key` filter works
- [x] Client: `lvt-el:*:on:click-away` inverted containment works (client-side DOM manipulation, no server round-trip)
- [x] Client: `modal-manager.ts` deleted, no `lvt-modal-open/close` handling
- [x] Client: No `lvt-data-*`, `lvt-value-*`, `lvt-submit`, `lvt-confirm`, `lvt-change` handling
- [x] Client: `lvt-disable-on`/`lvt-enable-on` reactive actions removed
- [x] Client: Reactive DOM uses `lvt-el:*:on:*` prefix with unscoped (`lvt-el:addClass:on:pending`) and action-scoped (`lvt-el:addClass:on:save:pending`) triggers
- [x] Client: Directives use `lvt-fx:*` prefix, read from CSS custom properties, `livetemplate.css` ships with defaults
- [x] Client: `lvt-fx:` supports optional `:on:` triggers — implicit (no `:on:`, activates on content change) and explicit (`lvt-fx:highlight:on:success`, `lvt-fx:highlight:on:save:success`)
- [x] Client: Timing modifiers use `lvt-mod:*` prefix (`lvt-mod:debounce`, `lvt-mod:throttle`)
- [x] Client: Form behavior uses `lvt-form:*` prefix (`lvt-form:action`, `lvt-form:preserve`, `lvt-form:disable-with`, `lvt-form:no-intercept`)
- [x] Client: Link interception opt-out uses `lvt-nav:no-intercept` (semantic separation from `lvt-form:no-intercept`)
- [x] Client: `action` field is no longer reserved — flows through to ActionData as normal form data
- [x] Client: `lvt-form:` supports optional `:on:` triggers — implicit (no `:on:`, always active) and explicit (`lvt-form:preserve:on:error`, `lvt-form:preserve:on:update:error`)
- [x] Client: All tests pass: `npm test`

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

#### Step 4: Implement Multi-Action Bracket Expansion (Server-Side)

The Go server library must expand bracket syntax in `lvt-el:*`, `lvt-fx:*`, and `lvt-form:*` attributes during HTML rendering so the client only sees single-action attributes.

**Where:** In the template rendering pipeline, after HTML is generated but before it's sent to the client. This could be a post-processing step in `template.go` or `internal/render/html.go`.

**Algorithm:**
```go
// For any attribute matching lvt-{el|fx|form}:*:on:[*]:*
// 1. Extract the bracket content: "save,delete"
// 2. Split by comma: ["save", "delete"]
// 3. For each action, emit a separate attribute:
//    lvt-el:addClass:on:[save,delete]:pending="opacity-50"
//    → lvt-el:addClass:on:save:pending="opacity-50"
//    + lvt-el:addClass:on:delete:pending="opacity-50"
```

**Tests:**
- `lvt-el:addClass:on:[save,delete]:pending="X"` expands to two attributes
- `lvt-el:addClass:on:[a,b,c]:error="X"` expands to three attributes
- `lvt-el:addClass:on:save:pending="X"` passes through unchanged (no brackets)
- `lvt-el:addClass:on:pending="X"` passes through unchanged (unscoped)
- `lvt-fx:highlight:on:[save,update]:success="flash"` expands to two attributes
- `lvt-form:preserve:on:[create,edit]:error` expands to two attributes

#### Step 5: Update Documentation

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

#### Step 6: Acceptance Criteria (Phase 1B)

- [x] Server: `lvt-action` form field no longer parsed
- [x] Server: Multi-action bracket expansion works for `lvt-el:*`, `lvt-fx:*`, and `lvt-form:*` (`on:[a,b]:state` → individual attributes)
- [x] Server: All tests pass: `GOWORK=off go test ./... -timeout=300s`
- [x] Server: `docs/references/client-attributes.md` updated with new syntax, deprecated entries removed
- [x] Server: `docs/guides/progressive-complexity.md` uses `lvt-on:{event}` syntax throughout
- [x] Server: `docs/references/progressive-complexity-reference.md` updated

#### Step 7: Commit and Create PR

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
   # Category 1 eliminations
   grep -r "lvt-click" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-submit" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-data-" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-change" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-modal-" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-confirm" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   # Category 5 event router
   grep -r "lvt-input" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-focus" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-blur" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-keydown" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-keyup" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-mouseenter\|lvt-mouseleave\|lvt-mouseover" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-click-away" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   # Window events (most caught by element-scoped greps above; lvt-window-resize has no element counterpart)
   grep -r "lvt-window-resize" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   # Category 2 consolidations
   grep -r "lvt-scroll-behavior\|lvt-scroll-threshold" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-highlight-color\|lvt-highlight-duration" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-animate-duration" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-disable-on\|lvt-enable-on" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   # Category 6 prefix renames (flat → prefixed)
   grep -rw "lvt-scroll\b\|lvt-highlight\b\|lvt-animate\b" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-debounce\|lvt-throttle" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   grep -r "lvt-preserve\|lvt-disable-with\|lvt-no-intercept" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
   # Category 7 reactive DOM renames
   grep -r "lvt-addClass-on\|lvt-removeClass-on\|lvt-toggleClass-on\|lvt-setAttr-on\|lvt-toggleAttr-on\|lvt-reset-on" --include="*.tmpl" --include="*.go" --include="*.golden" -l | wc -l
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

8. **Identify components that could benefit from action-scoped triggers:**
   - Which components have elements with multiple `lvt-on:` actions that need distinct reactive DOM behavior?
   - Which templates use `lvt-reset-on:success` or similar reactive attrs that should become `lvt-el:reset:on:success`?
   - Note which components could adopt multi-action bracket syntax (`lvt-el:*:on:[action1,action2]:state`) — the server-side Go library (Phase 1B) expands these to individual attributes before sending to the client

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
| `lvt-mouseenter/leave/over` | _TBD_ |
| `lvt-click-away` | _TBD_ |
| `lvt-scroll-behavior/threshold` | _TBD_ |
| `lvt-highlight-color/duration` | _TBD_ |
| `lvt-animate-duration` | _TBD_ |
| `lvt-disable-on/enable-on` | _TBD_ |
| `lvt-scroll` (flat, → `lvt-fx:scroll`) | _TBD_ |
| `lvt-highlight` (flat, → `lvt-fx:highlight`) | _TBD_ |
| `lvt-animate` (flat, → `lvt-fx:animate`) | _TBD_ |
| `lvt-debounce/throttle` (→ `lvt-mod:*`) | _TBD_ |
| `lvt-preserve/disable-with/no-intercept` (→ `lvt-form:*`) | _TBD_ |
| `lvt-addClass-on:*/removeClass-on:*` etc. (→ `lvt-el:*`) | _TBD_ |

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

**Category 1 eliminations:**
- [x] Zero occurrences of `lvt-click` in any `.tmpl` file (replaced by `name=` or `lvt-on:click`)
- [x] Zero occurrences of `lvt-submit` in any `.tmpl` file
- [x] Zero occurrences of `lvt-data-*` in any `.tmpl` file (replaced by `data-*`)
- [x] Zero occurrences of `lvt-confirm` in any `.tmpl` file
- [x] Zero occurrences of `lvt-modal-open` or `lvt-modal-close` in any `.tmpl` file
- [x] Zero occurrences of `lvt-change` in any `.tmpl` file (replaced by `lvt-on:change` or `lvt-on:input`)

**Category 2 consolidations:**
- [x] Zero occurrences of `lvt-scroll-behavior`, `lvt-scroll-threshold` (replaced by CSS `--lvt-scroll-*`)
- [x] Zero occurrences of `lvt-highlight-color`, `lvt-highlight-duration` (replaced by CSS `--lvt-highlight-*`)
- [x] Zero occurrences of `lvt-animate-duration` (replaced by CSS `--lvt-animate-duration`)
- [x] Zero occurrences of `lvt-disable-on:*`, `lvt-enable-on:*` (replaced by `lvt-el:toggleAttr:on:*`)

**Category 5 event router:**
- [x] All `lvt-input`, `lvt-keydown`, `lvt-keyup`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-mouseover` replaced by `lvt-on:{event}` equivalents; `lvt-click-away` replaced by `lvt-el:*:on:click-away`
- [x] All `lvt-window-keydown`, `lvt-window-keyup`, `lvt-window-scroll`, `lvt-window-resize`, `lvt-window-focus`, `lvt-window-blur` replaced by `lvt-on:window:{event}` equivalents

**Category 6 prefix consolidation:**
- [x] Zero occurrences of flat `lvt-scroll`, `lvt-highlight`, `lvt-animate` (replaced by `lvt-fx:*`)
- [x] Zero occurrences of flat `lvt-debounce`, `lvt-throttle` (replaced by `lvt-mod:*`)
- [x] Zero occurrences of flat `lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept` (replaced by `lvt-form:*`)

**Category 7 reactive DOM renames + action-scoped triggers:**
- [x] Zero occurrences of `lvt-addClass-on:*`, `lvt-removeClass-on:*`, etc. (replaced by `lvt-el:*:on:*`)
- [x] Server-expanded multi-action bracket syntax (`lvt-el:*:on:[action1,action2]:state`) works if adopted in templates

**Build verification:**
- [x] Golden files regenerated and matching
- [x] All Go tests pass: `GOWORK=off go test ./... -timeout=300s`
- [x] E2E tests pass: `GOWORK=off go test ./e2e/... -timeout=600s`
- [x] `go.mod` updated to latest livetemplate version

**Additional work completed in Phase 2 (beyond original scope):**
- [x] `command`/`commandfor` replaced with `data-lvt-target` + `lvt-el:toggleAttr:on:click="hidden"` for modals (Chrome 135+ not available in CI)
- [x] All `onclick` toggle handlers in dropdown/popover/datepicker/timepicker/menu replaced with `lvt-el:toggleClass:on:click` + `data-lvt-target="closest:..."` 
- [x] Server-managed delete confirm modals (RequestDelete/ConfirmDelete/CancelDelete) replaced with browser-native `confirm()` — ~760 lines removed
- [x] 17 follow-up issues created from bot reviews (lvt [#293](https://github.com/livetemplate/lvt/issues/293)–[#309](https://github.com/livetemplate/lvt/issues/309))

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
   - **Also search for:** `lvt-debounce`, `lvt-throttle` (→ `lvt-mod:*`); `lvt-scroll`, `lvt-highlight`, `lvt-animate` (→ `lvt-fx:*`); `lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept` (→ `lvt-form:*`); `lvt-addClass-on:*` etc. (→ `lvt-el:*`); config attrs (`lvt-scroll-behavior`, etc.); `lvt-disable-on`, `lvt-enable-on`
   - Identify any Go code generating HTML with reactive DOM attrs that could benefit from **action-scoped triggers** (e.g., `lvt-el:reset:on:add:success` to reset form only on successful Add, not Delete) or **multi-action bracket syntax** (e.g., `lvt-el:addClass:on:[add,update]:pending`). Server-side bracket expansion (Phase 1B) makes these available in templates.

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

**Category 1 eliminations:**
- [ ] Zero `lvt-click` in Go code string literals (replaced by `name=` on buttons)
- [ ] Zero `lvt-submit` in Go code or templates (replaced by button/form `name`)
- [ ] Zero `lvt-data-*` in Go code or templates (replaced by `data-*`)
- [ ] Zero `lvt-confirm` in Go code or templates (replaced by `onclick`)
- [ ] Zero `lvt-change` in templates (replaced by `lvt-on:change` or `lvt-on:input`)

**Category 2 consolidations:**
- [ ] Zero `lvt-scroll-behavior`, `lvt-scroll-threshold`, `lvt-highlight-color`, `lvt-highlight-duration`, `lvt-animate-duration` in Go code or templates
- [ ] Zero `lvt-disable-on:*`, `lvt-enable-on:*` (replaced by `lvt-el:toggleAttr:on:*`)

**Category 5 event router:**
- [ ] All `lvt-input`, `lvt-keydown`, `lvt-keyup`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-mouseover` replaced by `lvt-on:{event}` equivalents
- [ ] All `lvt-window-keydown`, `lvt-window-keyup`, `lvt-window-scroll`, `lvt-window-resize`, `lvt-window-focus`, `lvt-window-blur` replaced by `lvt-on:window:{event}` equivalents

**Category 6 prefix consolidation:**
- [ ] Zero flat `lvt-scroll`, `lvt-highlight`, `lvt-animate` (replaced by `lvt-fx:*`)
- [ ] Zero flat `lvt-debounce`, `lvt-throttle` (replaced by `lvt-mod:*`)
- [ ] Zero flat `lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept` (replaced by `lvt-form:*`)

**Category 7 reactive DOM renames + action-scoped triggers:**
- [ ] Zero `lvt-addClass-on:*`, `lvt-reset-on:*`, etc. (replaced by `lvt-el:*:on:*`) — e.g., `lvt-reset-on:success` → `lvt-el:reset:on:success`
- [ ] Server-expanded multi-action bracket syntax works if adopted in Go-generated HTML

**Scope guard + build verification:**
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

### Phase 2E: Examples Repo — Early Migration + Manual Review

> **Session Context**
>
> - **Prerequisites:** Phase 1B must be **merged** and tagged. Phase 1A must be **published** to npm.
> - **Pre-flight checks:**
>   ```bash
>   npm view @livetemplate/client version  # Phase 1A version
>   go list -m github.com/livetemplate/livetemplate@latest  # Phase 1B version
>   ```
> - **Starting point:** Create worktree `git worktree add .worktrees/attr-reduction -b attr-reduction` in `$REPO_ROOT/examples`
> - **Scope:** Bump dependencies, migrate any deprecated attrs, update docs — provides **early migration feedback** before lvt/tinkerdown phases
> - **Key insight:** The `examples` repo depends only on `livetemplate` (Go library) — NOT on `lvt` or `tinkerdown`. It can be migrated right after Phase 1B, in parallel with Phases 2A/2B and 3A/3B.
> - **Skip:** Any example that imports `github.com/livetemplate/lvt/components` — that dependency won't be updated until Phase 2B. Migrate it in Phase 4 instead.
> - **Outputs:** Working examples on new attribute syntax. Manual review by maintainer before proceeding with lvt/tinkerdown.

**Goal:** Migrate the examples repository early to get hands-on feedback on the new attribute syntax before tackling the larger lvt and tinkerdown codebases.

**Repo:** `livetemplate/examples`

**Dependency:** Phase 1A (client published) + Phase 1B (server tagged). Does NOT depend on Phase 2 or 3.

#### Step 1: Audit (MANDATORY — do this first)

```
cd $REPO_ROOT/examples
```

1. Confirm which `lvt-*` attributes are in actual template files (not just docs):
   - Expected: `lvt-fx:scroll`, `lvt-upload`, `lvt-form:preserve`, `lvt-form:no-intercept`, `lvt-el:*:on:*` (all Tier 2)
   - Verify zero deprecated attributes in templates

2. **Identify which examples import `lvt/components`:**
   ```
   grep -r "livetemplate/lvt/components" --include="*.go" -l
   ```
   These examples depend on the `lvt` component library and must be **skipped** in this phase — migrate them in Phase 4 after Phase 2B merges.

3. Check `go.mod` — dependency version

4. Check docs/README for references to deprecated attributes

5. Run baseline tests:
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
go mod tidy
```

Note: `lvt` dependency bump (if any) happens later in Phase 4 after Phase 2B merges.

#### Step 4: Migrate Templates + Update Documentation

- Replace any deprecated `lvt-*` attributes using the [attribute replacement rules](#design-summary-quick-reference-for-implementors)
- Update `README.md` and any `CLAUDE.md` references to use the new attribute syntax in code examples

#### Step 5: Run Tests + Manual Review

```bash
GOWORK=off go test ./... -timeout=300s
```

Then **run each example application manually** and verify:
- All actions still work (button clicks, form submissions)
- Reactive DOM behavior (loading states, class toggling) functions correctly
- No console errors in the browser

⚠️ **Pause here for maintainer review.** This is the first real-world test of the new syntax. Report any ergonomic issues, confusing patterns, or migration friction before proceeding with Phase 2/3.

#### Step 6: Acceptance Criteria (Phase 2E)

- [x] `examples` `go.mod` points to Phase 1B livetemplate version
- [x] Zero deprecated attribute references in templates
- [x] All Go tests pass
- [x] Manual review completed — each example runs correctly in browser
- [x] Migration friction documented (if any) — feed back into Phase 2/3 approach

#### Step 7: PR and Merge

```bash
cd $REPO_ROOT/examples/.worktrees/attr-reduction
git add -u
git commit -m "feat!: migrate to new lvt-on/lvt-el attribute syntax

BREAKING CHANGE: Templates use lvt-on:{event} and lvt-el:* syntax."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction — early migration" \
  --body "Phase 2E. Early examples migration for manual review feedback."
```

After merge:
```bash
cd $REPO_ROOT/examples && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 2E to COMPLETE, fill in PR number.

---

### Phase 4: Final Cross-Repo Verification

> **Session Context**
>
> - **Prerequisites:** Phases 1A, 1B, 2B, 2E, and 3B must all be **merged** and published.
> - **Pre-flight checks:**
>   ```bash
>   npm view @livetemplate/client version  # Phase 1A version
>   go list -m github.com/livetemplate/livetemplate@latest  # Phase 1B version
>   go list -m github.com/livetemplate/lvt@latest  # Phase 2B version
>   ```
> - **Scope:** Bump `lvt` dependency in examples (if needed), run cross-repo verification
> - **Outputs:** All 5 repos passing tests. Migration complete.

**Goal:** Final dependency alignment and cross-repo verification after all phases are merged.

**Repo:** `livetemplate/examples` (final bump) + cross-repo

**Dependency:** All previous phases merged.

#### Step 1: Final Dependency Bump (if needed)

If examples `go.mod` still references an old `lvt` version:

```bash
cd $REPO_ROOT/examples
go get github.com/livetemplate/lvt@latest
go get github.com/livetemplate/lvt/components@latest
go mod tidy
```

#### Step 2: Cross-Repo Verification

With ALL changes merged, verify end-to-end:

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

#### Step 3: Acceptance Criteria (Final)

- [ ] All 5 repos pass their test suites
- [ ] `examples` `go.mod` points to latest livetemplate AND lvt versions
- [ ] No remaining references to deprecated attributes across any repo (verify with grep)
- [ ] Migration friction from Phase 2E has been addressed

#### Step 4: PR and Merge (if deps were bumped)

```bash
cd $REPO_ROOT/examples
git add -u
git commit -m "chore: bump lvt dependency after attribute reduction"
git push origin attr-reduction
gh pr create --head attr-reduction --title "chore: final dep bump for attribute reduction" \
  --body "Phase 4. Final lvt dependency bump + cross-repo verification."
```

**Update this progress tracker:** Set Phase 4 to COMPLETE, fill in PR number.

---

### Cross-Phase Dependency Graph

```
Phase 1A: client ──→ Phase 1B: livetemplate (server + docs)
    |                     |
    ├─────────────────────┤
    |                     |
    ├──→ Phase 2E: examples (early migration + manual review) ─────────────────────────┐
    |                                                                                   |
    ├──→ Phase 2A: lvt (audit + templates + Go) ──→ Phase 2B: lvt (golden + e2e + PR) ─┤
    |                                                                                   ├──→ Phase 4
    └──→ Phase 3A: tinkerdown (audit + Go + TS) ──→ Phase 3B: tinkerdown (docs + e2e) ─┘
```

**Parallelism:**
- 1A → 1B is sequential (server needs client to be ready, but PRs can be prepared in parallel)
- 2E, 2A/2B, and 3A/3B can all proceed in parallel after 1B merges
- 2E runs early to provide migration feedback before the larger lvt/tinkerdown efforts
- Within each repo, A → B is sequential (B depends on A's changes)
- Phase 4 depends on 2B + 2E + 3B being merged

**Total: 8 sub-phases across ~8 LLM sessions** (1A, 1B, 2E, 2A, 2B, 3A, 3B, 4).

### PR Merge Order

1. **`client`** (Phase 1A) — merge and publish new npm version
2. **`livetemplate`** (Phase 1B) — merge and tag new Go version
3. **`examples`** (Phase 2E) — early migration for manual review (can merge immediately after 1B)
4. **`lvt`** (Phase 2B) — depends on new client + server versions
5. **`tinkerdown`** (Phase 3B) — depends on new client + server versions
6. **`examples`** (Phase 4, if needed) — final lvt dep bump + cross-repo verification

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
