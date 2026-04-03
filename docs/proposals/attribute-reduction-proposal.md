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

## Why Not `onclick` for Server Routing?

The issue asks: "can't we use `onclick` attribute for custom routing?" The answer is **no**:

1. **`onclick` executes JavaScript.** It runs arbitrary JS in the browser. LiveTemplate's action routing sends a message to a server-side Go method. These are fundamentally different semantics.
2. **Convention-based `onclick`** (e.g., `onclick="lvt('delete')"`) would require exposing a global JS function, mixing declarative templates with imperative JS, and breaking the no-JS fallback model.
3. **The right Tier 1 alternative is `<button name="action">`.** It's semantic HTML, works across all three transport levels (no-JS POST, fetch, WebSocket), and requires zero JavaScript.
4. **For non-button elements**, `lvt-click` remains the only option. Standard HTML has no attribute that means "route this click to the server."

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

**Rationale:** `lvt-submit` is already the lowest-priority routing mechanism (action resolution order: button `name` > form `name` > default `"submit"`). Standard HTML covers all use cases. Keep for backward compatibility during deprecation period.

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

<!-- Direct-to-S3 presigned uploads — still needs lvt-upload -->
<input type="file" lvt-upload="large-file" data-strategy="presigned">
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
| `lvt-upload` | Narrow | Keep only for Tier 2 features (custom drop zones, presigned S3) |

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
<div lvt-scroll="bottom-sticky"
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
<div lvt-highlight="flash"
     style="--lvt-highlight-color: #ffc107; --lvt-highlight-duration: 500ms;">
```

### Animate: 2 → 1

**Before:**
```html
<div lvt-animate="fade" lvt-animate-duration="300">
```

**After:**
```html
<div lvt-animate="fade" style="--lvt-animate-duration: 300ms;">
```

### Disable/Enable: 2 → 0

`lvt-disable-on:{event}` and `lvt-enable-on:{event}` are syntactic sugar for `lvt-toggleAttr-on:{event}="disabled"`, which already exists:

**Before:**
```html
<button lvt-disable-on:pending lvt-enable-on:done>Save</button>
```

**After:**
```html
<button lvt-toggleAttr-on:pending="disabled" lvt-toggleAttr-on:done="disabled">Save</button>
```

No CSS dependency needed — this is pure attribute consolidation.

### Summary of Consolidations

| Before | After | Reduction |
|--------|-------|-----------|
| `lvt-scroll`, `lvt-scroll-behavior`, `lvt-scroll-threshold` | `lvt-scroll` + CSS custom properties | 3 → 1 |
| `lvt-highlight`, `lvt-highlight-color`, `lvt-highlight-duration` | `lvt-highlight` + CSS custom properties | 3 → 1 |
| `lvt-animate`, `lvt-animate-duration` | `lvt-animate` + CSS custom properties | 2 → 1 |
| `lvt-disable-on:{event}`, `lvt-enable-on:{event}` | `lvt-toggleAttr-on:{event}="disabled"` | 2 → 0 |

**Impact: 7 attributes removed via consolidation.**

## Category 3: Must Remain as Tier 2

These attributes express behaviors that standard HTML cannot. They are the **essential `lvt-*` surface**.

### Timing Control
| Attribute | Why |
|-----------|-----|
| `lvt-debounce` | Server-side debounce timing — no HTML equivalent |
| `lvt-throttle` | Rate limiting — no HTML equivalent |

### Keyboard Filtering
| Attribute | Why |
|-----------|-----|
| `lvt-key` | Filter by specific key (Enter, Escape, etc.) — no HTML attribute for this |

### Event Routing (element-scoped)
| Attribute | Why |
|-----------|-----|
| `lvt-click` (narrowed) | Non-button click → server action. Buttons use `name` instead. |
| `lvt-change` (narrowed) | Named action other than `Change()`. Default case uses convention. |
| `lvt-input` | Per-keystroke server action (distinct from debounced `Change()`) |
| `lvt-keydown` | Element keydown → server action |
| `lvt-keyup` | Element keyup → server action |
| `lvt-focus` | Focus → server action |
| `lvt-blur` | Blur → server action |
| `lvt-mouseenter` | Mouse enter → server action |
| `lvt-mouseleave` | Mouse leave → server action |
| `lvt-click-away` | Click outside element — no HTML native |

### Event Routing (window-scoped)
| Attribute | Why |
|-----------|-----|
| `lvt-window-keydown` | Global keyboard shortcuts → server |
| `lvt-window-keyup` | Global key release → server |
| `lvt-window-scroll` | Window scroll → server |
| `lvt-window-resize` | Window resize → server |
| `lvt-window-focus` | Window focus → server |
| `lvt-window-blur` | Window blur → server |

### Reactive DOM (lifecycle-driven)
| Attribute | Why |
|-----------|-----|
| `lvt-addClass-on:{event}` | Add CSS class on pending/success/error/done |
| `lvt-removeClass-on:{event}` | Remove CSS class on lifecycle event |
| `lvt-toggleClass-on:{event}` | Toggle CSS class on lifecycle event |
| `lvt-setAttr-on:{event}` | Set attribute on lifecycle event |
| `lvt-toggleAttr-on:{event}` | Toggle boolean attribute (subsumes `disable-on`/`enable-on`) |
| `lvt-reset-on:{event}` | Reset form on lifecycle event |

### Form Behavior
| Attribute | Why |
|-----------|-----|
| `lvt-preserve` | Prevent form auto-reset — opposite of default framework behavior |
| `lvt-disable-with` | Button text swap + disable during pending — no HTML pattern |
| `lvt-no-intercept` | Opt-out of SPA form interception — framework-specific concept |

### Directives
| Attribute | Why |
|-----------|-----|
| `lvt-scroll` | Scroll position management (bottom, bottom-sticky, preserve) |
| `lvt-highlight` | Temporary highlight effect on update |
| `lvt-animate` | Entrance animation on insert |

### Upload
| Attribute | Why |
|-----------|-----|
| `lvt-upload` (narrowed) | Tier 2-only: custom drop zones (`<div>` as drop target) and direct-to-S3 presigned uploads. Basic file uploads moved to Tier 1 via standard `<input type="file" name="...">` in a `<form>`. See [Tier 1 File Uploads Proposal](tier1-file-uploads-proposal.md). |

## Category 4: Needs Discussion

These attributes are technically needed but their real-world usage may be rare enough to not justify the surface area.

| Attribute | Question |
|-----------|----------|
| `lvt-input` | Is it needed when `Change()` convention exists? Difference: `lvt-input` routes to a named action, `Change()` always routes to `Change`. Could `Change()` accept action name routing instead? |
| `lvt-focus`, `lvt-blur` | How common are server-round-trip focus/blur handlers? If usage is rare across real apps, consider removing. |
| `lvt-mouseenter`, `lvt-mouseleave` | Server round-trip on hover has latency concerns. May cause poor UX if network is slow. Consider whether these encourage bad patterns. |
| `lvt-disable-with` | Could be replaced by CSS: `button[aria-busy="true"] { ... }` with `::after` for text. Less ergonomic but zero custom attributes. Worth the tradeoff? |

## Category 5: Generic Event Router Consolidation

Individual event-binding attributes (`lvt-click`, `lvt-keydown`, `lvt-mouseenter`, etc.) can be replaced by a single generic pattern. The client already uses a parameterized loop internally (`attrName = lvt-${eventType}`) — this change surfaces that generic infrastructure to users.

### `lvt-on:{event}` — Unified Element-Scoped Event Router

**Pattern:** `lvt-on[:{scope}]:{event}="action"`

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
| `lvt-click-away="close"` | `lvt-on:click-away="close"` |

**Example — autocomplete component:**
```html
<!-- Before: 5 different attribute names -->
<div lvt-click-away="close">
  <input lvt-input="search" lvt-focus="open" lvt-blur="close" lvt-debounce="300">
  <ul>
    {{range .Results}}
      <li lvt-click="select" data-id="{{.ID}}">{{.Name}}</li>
    {{end}}
  </ul>
</div>

<!-- After: 1 attribute pattern -->
<div lvt-on:click-away="close">
  <input lvt-on:input="search" lvt-on:focus="open" lvt-on:blur="close" lvt-debounce="300">
  <ul>
    {{range .Results}}
      <li lvt-on:click="select" data-id="{{.ID}}">{{.Name}}</li>
    {{end}}
  </ul>
</div>
```

**Note:** `lvt-on:click` is only for non-button elements. Buttons use standard HTML `<button name="action">`.

### `lvt-on:window:{event}` — Unified Window-Scoped Event Router

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

The colon-separated namespace is consistent with the existing reactive attribute pattern (`lvt-addClass-on:pending`).

### `lvt-change` — Deprecate Entirely

`lvt-change` is removed with no direct replacement:

- **Default case (route to `Change()`):** Already handled by the `Change()` auto-wiring convention — no attribute needed. If a controller has a `Change()` method, inputs auto-wire to it.
- **Named action case (route to a specific method):** Use `lvt-on:input="validateEmail"` instead of `lvt-change="validateEmail"`. The `input` event fires on every keystroke (more responsive); `change` only fires on blur.

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

### Impact Summary

| Change | Attributes Removed |
|--------|-------------------|
| Element-scoped events → `lvt-on:{event}` | 9 (`lvt-click`, `lvt-input`, `lvt-keydown`, `lvt-keyup`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-click-away`) |
| Window-scoped events → `lvt-on:window:{event}` | 6 (`lvt-window-keydown`, `lvt-window-keyup`, `lvt-window-scroll`, `lvt-window-resize`, `lvt-window-focus`, `lvt-window-blur`) |
| `lvt-change` deprecated entirely | 1 |
| **Total** | **16 attribute names removed, replaced by 2 patterns** |

### Modifiers Unchanged

`lvt-debounce`, `lvt-throttle`, and `lvt-key` continue to work alongside the generic router:

```html
<input lvt-on:input="search" lvt-debounce="300">
<div lvt-on:window:keydown="shortcut" lvt-key="/">
```

## Before and After

| Category | Before | After |
|----------|--------|-------|
| **Total `lvt-*` attributes** | ~51 | ~19 attribute names + 2 generic patterns |
| Event bindings (element) | 10 individual names | `lvt-on:{event}` (1 pattern) |
| Event bindings (window) | 6 individual names | `lvt-on:window:{event}` (1 pattern) |
| `lvt-change` | 1 | 0 (convention + `lvt-on:input`) |
| Data passing | 2 patterns (`lvt-data-*`, `lvt-value-*`) | 0 (standard HTML) |
| Modals | 2 | 0 (native `<dialog>`) |
| Legacy routing | 2 (`lvt-submit`, `lvt-action`) | 0 (standard HTML) |
| Confirmation | 1 | 0 (standard `onsubmit`) |
| Scroll directives | 3 | 1 + CSS custom properties |
| Highlight directives | 3 | 1 + CSS custom properties |
| Animation directives | 2 | 1 + CSS custom properties |
| Enable/disable sugar | 2 | 0 (use `lvt-toggleAttr-on`) |
| Upload | 1 | Narrowed (Tier 1 for basic, Tier 2 for drop zones/S3) |
| Timing modifiers | 2 (`lvt-debounce`, `lvt-throttle`) | 2 (unchanged) |
| Key filter | 1 (`lvt-key`) | 1 (unchanged) |
| Reactive DOM | 6 | 6 (unchanged — `addClass-on`, `removeClass-on`, `toggleClass-on`, `setAttr-on`, `toggleAttr-on`, `reset-on`) |
| Form behavior | 3 (`lvt-preserve`, `lvt-disable-with`, `lvt-no-intercept`) | 3 (unchanged) |
| Directives | 3 (`lvt-scroll`, `lvt-highlight`, `lvt-animate`) | 3 (unchanged) |
| Upload (Tier 2) | 1 | 1 (narrowed scope) |

---

## Implementation Plan

### Progress Tracker

| Phase | Description | Repos | Status | PR(s) |
|-------|-------------|-------|--------|-------|
| 1 | Core library + client | `livetemplate`, `client` | NOT STARTED | — |
| 2 | lvt repo migration | `lvt` | NOT STARTED | — |
| 3 | tinkerdown repo migration | `tinkerdown` | NOT STARTED | — |
| 4 | examples repo + final verification | `examples` | NOT STARTED | — |

**After completing each phase:** Update Status to COMPLETE, fill in PR numbers, and commit this file.

**PR merge order:** `client` → `livetemplate` → `lvt` → `tinkerdown` → `examples`. The client must be published first because `lvt` and `tinkerdown` e2e tests load the client library.

---

### Phase 1: Core Library + Client

**Goal:** Implement the generic event router in the TypeScript client, remove all deprecated attribute handling, update the Go server to remove `lvt-action` parsing, update all documentation, and ship CSS custom property support.

**Repos:** `livetemplate/livetemplate` (server), `livetemplate/client` (TypeScript client)

**Estimated effort:** 2 LLM sessions (1 client, 1 server).

#### Step 1: Audit (MANDATORY — do this first)

Before making any changes, deep dive into the codebase to capture the full migration impact for this phase. Update this plan section with specific findings.

**Client audit:**
```
cd /Users/adnaan/code/livetemplate/client
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
9. List ALL test files and what they cover

**Server audit:**
```
cd /Users/adnaan/code/livetemplate/livetemplate
```

1. Grep for `lvt-action` in `internal/send/message.go` — exact line numbers
2. Grep for `lvt-action` in `handle_test.go` — count and list all occurrences
3. Grep for `lvt-action` in `internal/send/message_test.go` — exact test case names
4. Grep for `lvt-` in `action.go`, `template.go` — list all comment references
5. Run `GOWORK=off go test ./... -timeout=300s` to get baseline

**Update this plan with:** Exact file:line mappings, import dependency graph for removed modules, baseline test counts.

#### Step 2: Worktree Setup

```bash
cd /Users/adnaan/code/livetemplate/client
git worktree add .worktrees/attr-reduction -b attr-reduction

cd /Users/adnaan/code/livetemplate/livetemplate
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Client — Implement Generic Event Router

**File:** `dom/event-delegation.ts`

1. **Change attribute lookup.** In the event loop where `attrName = lvt-${eventType}` is constructed, change to look for `lvt-on:${eventType}` instead. The attribute detection walks up the DOM tree from the event target — update the `element.getAttribute()` call.

2. **Window events.** In `setupWindowEventDelegation()`, change `attrName = lvt-window-${eventType}` to `attrName = lvt-on:window:${eventType}`.

3. **Click-away.** In `setupClickAwayDelegation()`, change `[lvt-click-away]` selector to `[lvt-on\\:click-away]`. Note: colon in CSS selectors needs escaping with backslash.

4. **Remove `lvt-submit` handling.** Remove the code that checks for `lvt-submit` on forms. Forms route via button `name`, form `name`, or default `"submit"`.

5. **Remove `lvt-data-*` and `lvt-value-*` extraction.** Remove the loops that scan for `lvt-data-*` and `lvt-value-*` attributes on action elements. Data should come from `data-*` attributes or hidden inputs.

6. **Remove `lvt-change` handling.** Remove the special case for `lvt-change` on forms and inputs. The `Change()` auto-wiring (in `state/change-auto-wirer.ts`) is orthogonal and untouched.

#### Step 4: Client — Remove Deprecated Modules

1. **Delete `dom/modal-manager.ts`.** Remove the entire file. Update `livetemplate-client.ts` to remove the import, instantiation, and any `setupModalDelegation()` calls.

2. **Update `utils/confirm.ts`.** Remove `checkLvtConfirm()`. If `extractLvtData()` is only used for `lvt-data-*` extraction, remove it too. Check all imports first.

3. **Update `dom/reactive-attributes.ts`.** Remove `"disable"` and `"enable"` from the reactive action types. Users must use `lvt-toggleAttr-on:{event}="disabled"` instead.

4. **Update `dom/directives.ts`.** For each directive (scroll, highlight, animate):
   - Remove reads of `lvt-scroll-behavior`, `lvt-scroll-threshold` attributes
   - Remove reads of `lvt-highlight-color`, `lvt-highlight-duration` attributes
   - Remove reads of `lvt-animate-duration` attribute
   - Instead, read from CSS custom properties via `getComputedStyle(element).getPropertyValue('--lvt-*')`
   - Fall back to hardcoded defaults if CSS property is empty

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
   Add to `package.json` `files` array.

#### Step 5: Client — Update Tests

Update all test files to use `lvt-on:{event}` syntax instead of `lvt-click`, `lvt-keydown`, etc.:

- `tests/event-delegation.test.ts` — update attribute names in test fixtures
- `tests/modal-manager.test.ts` — delete this file
- `tests/reactive-attributes.test.ts` — remove `disable`/`enable` test cases
- `tests/directives.test.ts` — update to test CSS custom property reading, remove attribute-based config tests
- Remove any test that depends on removed `lvt-submit`, `lvt-data-*`, `lvt-value-*`, `lvt-confirm`, `lvt-modal-*` handling

Add new tests:
- `lvt-on:click` routes to named action
- `lvt-on:window:keydown` with `lvt-key` filter works
- `lvt-on:click-away` works (inverted containment)
- CSS custom property `--lvt-scroll-behavior` is read by scroll directive
- CSS custom property `--lvt-highlight-duration` is read by highlight directive

```bash
cd /Users/adnaan/code/livetemplate/client/.worktrees/attr-reduction
npm test
```

#### Step 6: Server — Remove `lvt-action` Parsing

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
cd /Users/adnaan/code/livetemplate/livetemplate/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

#### Step 7: Server — Update Documentation

**File:** `docs/references/client-attributes.md`

- Remove the `lvt-submit` entry from Event Bindings
- Remove the `lvt-data-*` / `lvt-value-*` Data Passing section
- Remove the `lvt-confirm` entry from Form Behavior
- Remove the Modals section (`lvt-modal-open`, `lvt-modal-close`)
- Remove `lvt-disable-on` and `lvt-enable-on` from Reactive Attributes
- Remove `lvt-scroll-behavior`, `lvt-scroll-threshold`, `lvt-highlight-color`, `lvt-highlight-duration`, `lvt-animate-duration` from Directives
- Replace individual event attribute entries with a single `lvt-on:{event}` section
- Replace individual window event entries with a single `lvt-on:window:{event}` section
- Remove `lvt-change` entry; add note that `Change()` convention handles this automatically
- Update Table of Contents

**File:** `docs/guides/progressive-complexity.md`

- Update Section 13.1 (Event Bindings) to use `lvt-on:{event}` syntax
- Update Section 13.3 (Keyboard Shortcuts) to use `lvt-on:window:keydown`
- Update Section 13.4 (Reactive DOM) to remove `lvt-disable-on`/`lvt-enable-on` examples
- Update Section 13.5 (Directives) to show CSS custom properties instead of `lvt-*-behavior/color/duration` attributes

**File:** `docs/references/progressive-complexity-reference.md`

- Remove `lvt-submit` from Action Resolution Order
- Remove `lvt-modal-open`/`lvt-modal-close` from Dialog Routing
- Update event attribute examples to `lvt-on:{event}` syntax

#### Step 8: Acceptance Criteria

- [ ] Client: `lvt-on:click`, `lvt-on:input`, `lvt-on:keydown`, etc. all route to server actions correctly
- [ ] Client: `lvt-on:window:keydown` with `lvt-key` filter works
- [ ] Client: `lvt-on:click-away` inverted containment works
- [ ] Client: `modal-manager.ts` deleted, no `lvt-modal-open/close` handling
- [ ] Client: No `lvt-data-*`, `lvt-value-*`, `lvt-submit`, `lvt-confirm`, `lvt-change` handling
- [ ] Client: `lvt-disable-on`/`lvt-enable-on` reactive actions removed
- [ ] Client: Directives read from CSS custom properties, `livetemplate.css` ships with defaults
- [ ] Client: All tests pass: `npm test`
- [ ] Server: `lvt-action` form field no longer parsed
- [ ] Server: All tests pass: `GOWORK=off go test ./... -timeout=300s`
- [ ] Server: `docs/references/client-attributes.md` updated with new syntax, deprecated entries removed
- [ ] Server: `docs/guides/progressive-complexity.md` uses `lvt-on:{event}` syntax throughout

#### Step 9: PR and Merge

Client first:
```bash
cd /Users/adnaan/code/livetemplate/client/.worktrees/attr-reduction
git add -A
git commit -m "feat!: generic event router (lvt-on:{event}), remove deprecated attributes

BREAKING CHANGE: Replaces lvt-click, lvt-keydown, etc. with lvt-on:{event}.
Removes lvt-submit, lvt-confirm, lvt-modal-*, lvt-data-*, lvt-value-*,
lvt-change, lvt-disable-on, lvt-enable-on, and directive config attributes.
Adds livetemplate.css with CSS custom property defaults."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: generic event router + attribute reduction" \
  --body "Phase 1 of attribute reduction. See livetemplate/livetemplate docs/proposals/attribute-reduction-proposal.md"
```

Then server:
```bash
cd /Users/adnaan/code/livetemplate/livetemplate/.worktrees/attr-reduction
git add -A
git commit -m "feat!: remove lvt-action parsing, update docs for attribute reduction (#288)

BREAKING CHANGE: lvt-action form field no longer parsed. Use button name or action field."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction — server + docs (#288)" \
  --body "Phase 1 server-side. Removes lvt-action parsing, updates all documentation."
```

**Merge order:** Client PR first → publish new client version → then server PR.

After both merge, clean up worktrees:
```bash
cd /Users/adnaan/code/livetemplate/client && git worktree remove .worktrees/attr-reduction
cd /Users/adnaan/code/livetemplate/livetemplate && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 1 to COMPLETE, fill in PR numbers.

---

### Phase 2: lvt Repo Migration

**Goal:** Update all component templates, generator templates, kit templates, golden files, and e2e tests in the `lvt` repository to use the new attribute syntax.

**Repo:** `livetemplate/lvt`

**Estimated effort:** 2 LLM sessions (1 for templates, 1 for tests).

**Dependency:** Phase 1 must be merged and new client version published.

#### Step 1: Audit (MANDATORY — do this first)

Before making any changes, deep dive into the `lvt` codebase to capture the full migration impact.

```
cd /Users/adnaan/code/livetemplate/lvt
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

4. **Map golden files:**
   - List all `testdata/golden/*.golden` and `e2e/testdata/golden/*.golden`
   - These will need regeneration after template changes

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

**Update this plan with:** File-by-file migration map, golden file regeneration strategy, Go code changes needed.

#### Step 2: Worktree Setup

```bash
cd /Users/adnaan/code/livetemplate/lvt
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Component Templates

For each component in `components/*/templates/*.tmpl`:

**Attribute replacement rules:**
| Find | Replace |
|------|---------|
| `lvt-click="X"` on `<button>` | `name="X"` (Tier 1 button routing) |
| `lvt-click="X"` on non-button | `lvt-on:click="X"` |
| `lvt-submit="X"` | Remove; use `<button name="X">` or `<form name="X">` |
| `lvt-data-{key}="V"` | `data-{key}="V"` (standard HTML data attribute) |
| `lvt-value-{key}="V"` | `<input type="hidden" name="{key}" value="V">` |
| `lvt-confirm="msg"` | `onclick="return confirm('msg')"` on button |
| `lvt-modal-open="id"` | `command="show-modal" commandfor="id"` |
| `lvt-modal-close="id"` | `command="close" commandfor="id"` (inside `<form method="dialog">`) |
| `lvt-change="X"` | `lvt-on:change="X"` (for select elements) or `lvt-on:input="X"` (for text inputs) |
| `lvt-input="X"` | `lvt-on:input="X"` |
| `lvt-keydown="X"` | `lvt-on:keydown="X"` |
| `lvt-focus="X"` | `lvt-on:focus="X"` |
| `lvt-blur="X"` | `lvt-on:blur="X"` |
| `lvt-mouseenter="X"` | `lvt-on:mouseenter="X"` |
| `lvt-mouseleave="X"` | `lvt-on:mouseleave="X"` |
| `lvt-click-away="X"` | `lvt-on:click-away="X"` |

**High-impact components** (from audit):
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

#### Step 6: Regenerate Golden Files

Golden files in `testdata/golden/` and `e2e/testdata/golden/` will no longer match after template changes. Regenerate them:

```bash
# The exact command depends on the project's golden file update mechanism
# Common patterns:
GOWORK=off go test ./... -update  # if tests have an -update flag
# OR manually run the generator and compare
```

If the project doesn't have auto-update for golden files, manually update each golden file to reflect the new attribute syntax.

#### Step 7: Update E2E Tests

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

**Note:** Colon in attribute selectors requires escaping in CSS: `[lvt-on\:click="X"]`. In Go strings, this becomes `[lvt-on\\:click="X"]`.

#### Step 8: Update go.mod

```bash
cd /Users/adnaan/code/livetemplate/lvt/.worktrees/attr-reduction
go get github.com/livetemplate/livetemplate@latest
go mod tidy
```

#### Step 9: Run Tests

```bash
cd /Users/adnaan/code/livetemplate/lvt/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

For e2e tests (if they require the new client):
```bash
# Ensure the new client version is available (published from Phase 1)
GOWORK=off go test ./e2e/... -timeout=600s
```

#### Step 10: Acceptance Criteria

- [ ] Zero occurrences of `lvt-click` in any `.tmpl` file (replaced by `name=` or `lvt-on:click`)
- [ ] Zero occurrences of `lvt-submit` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-data-*` in any `.tmpl` file (replaced by `data-*`)
- [ ] Zero occurrences of `lvt-confirm` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-modal-open` or `lvt-modal-close` in any `.tmpl` file
- [ ] Zero occurrences of `lvt-change` in any `.tmpl` file (replaced by `lvt-on:change` or `lvt-on:input`)
- [ ] All `lvt-input`, `lvt-keydown`, `lvt-focus`, `lvt-blur`, `lvt-mouseenter`, `lvt-mouseleave`, `lvt-click-away` replaced by `lvt-on:{event}` equivalents
- [ ] Golden files regenerated and matching
- [ ] All Go tests pass: `GOWORK=off go test ./... -timeout=300s`
- [ ] E2E tests pass: `GOWORK=off go test ./e2e/... -timeout=600s`
- [ ] `go.mod` updated to latest livetemplate version

#### Step 11: PR and Merge

```bash
cd /Users/adnaan/code/livetemplate/lvt/.worktrees/attr-reduction
git add -A
git commit -m "feat!: migrate to generic event router (lvt-on:{event}), remove deprecated attributes

BREAKING CHANGE: All component templates updated to use lvt-on:{event} syntax.
lvt-submit, lvt-confirm, lvt-modal-*, lvt-data-*, lvt-change removed."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction — migrate all templates" \
  --body "Phase 2. Updates all component/generator/kit templates to new syntax."
```

After merge:
```bash
cd /Users/adnaan/code/livetemplate/lvt && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 2 to COMPLETE, fill in PR number.

---

### Phase 3: tinkerdown Repo Migration

**Goal:** Update tinkerdown's Go code that generates HTML with `lvt-*` attributes, its TypeScript client, all example/scaffold templates, documentation, and e2e tests.

**Repo:** `livetemplate/tinkerdown`

**Estimated effort:** 2 LLM sessions.

**Dependency:** Phase 1 must be merged. Phase 2 is independent (tinkerdown doesn't depend on lvt).

#### Step 1: Audit (MANDATORY — do this first)

Deep dive into the tinkerdown codebase to capture the full migration impact.

```
cd /Users/adnaan/code/livetemplate/tinkerdown
```

1. **Go code that GENERATES lvt-* attributes:**
   - Read `auto_tables.go` — find all `lvt-click`, `lvt-submit`, `lvt-confirm`, `lvt-data-*`, `lvt-reset-on:success` in string literals
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

**Update this plan with:** Complete file map, Go code change details, client migration strategy.

#### Step 2: Worktree Setup

```bash
cd /Users/adnaan/code/livetemplate/tinkerdown
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Go Code That Generates HTML

**File:** `auto_tables.go`

This file generates CRUD table HTML with `lvt-click`, `lvt-submit`, `lvt-confirm`, `lvt-data-id`, `lvt-reset-on:success`. Update all string literals:

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
| `lvt-reset-on:success` | `lvt-reset-on:success` (unchanged — this is a reactive attribute, not deprecated) |

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

#### Step 5: Update Templates and Examples

Apply the same replacement rules to all:
- `cmd/tinkerdown/commands/templates/*/index.md` — scaffold templates
- `examples/*/index.md` — example applications

Key files:
- `templates/todo/index.md` — `lvt-submit`, `lvt-click`, `lvt-data-id`
- `templates/form/index.md` — `lvt-submit`, `lvt-click`, `lvt-data-id`
- `templates/tutorial/index.md` — `lvt-click`, `lvt-change`
- `examples/action-buttons/index.md` — `lvt-click`, `lvt-submit`
- `examples/expense-tracker/index.md` — full CRUD
- All `lvt-source`, `lvt-columns` references remain UNCHANGED

#### Step 6: Update Documentation

**File:** `docs/reference/lvt-attributes.md`

Rewrite to reflect:
- `lvt-on:{event}` replaces individual event attributes
- `lvt-on:window:{event}` replaces window event attributes
- Standard HTML replaces `lvt-submit`, `lvt-data-*`, `lvt-confirm`, `lvt-modal-*`
- Tinkerdown-specific attributes (`lvt-source`, `lvt-columns`, etc.) documented separately

**Files:** `skills/tinkerdown/SKILL.md`, `skills/tinkerdown/reference.md`, `README.md`

Update attribute references and examples throughout.

#### Step 7: Update E2E Tests

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

#### Step 8: Update Dependencies

```bash
cd /Users/adnaan/code/livetemplate/tinkerdown/.worktrees/attr-reduction
go get github.com/livetemplate/livetemplate@latest
cd client && npm install @livetemplate/client@latest
```

#### Step 9: Run Tests

```bash
cd /Users/adnaan/code/livetemplate/tinkerdown/.worktrees/attr-reduction
GOWORK=off go test ./... -timeout=300s
```

#### Step 10: Acceptance Criteria

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

#### Step 11: PR and Merge

```bash
cd /Users/adnaan/code/livetemplate/tinkerdown/.worktrees/attr-reduction
git add -A
git commit -m "feat!: migrate to generic event router, remove deprecated lvt-* attributes

BREAKING CHANGE: Auto-generated HTML uses standard button name routing and
lvt-on:{event} syntax. lvt-submit, lvt-data-*, lvt-confirm removed."
git push origin attr-reduction
gh pr create --head attr-reduction --title "feat!: attribute reduction migration" \
  --body "Phase 3. Updates Go generators, templates, client, docs, and tests."
```

After merge:
```bash
cd /Users/adnaan/code/livetemplate/tinkerdown && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 3 to COMPLETE, fill in PR number.

---

### Phase 4: Examples Repo + Final Verification

**Goal:** Update the examples repository (minimal changes) and verify all repos work together end-to-end.

**Repo:** `livetemplate/examples`

**Estimated effort:** 1 LLM session.

**Dependency:** Phases 1-3 must be merged.

#### Step 1: Audit (MANDATORY — do this first)

```
cd /Users/adnaan/code/livetemplate/examples
```

1. Confirm which `lvt-*` attributes are in actual template files (not just docs):
   - Expected: `lvt-scroll`, `lvt-upload`, `lvt-preserve`, `lvt-no-intercept` (all Tier 2/unchanged)
   - Verify zero deprecated attributes in templates

2. Check `go.mod` — dependency version

3. Check docs/README for references to deprecated attributes

4. Run baseline tests:
   ```
   GOWORK=off go test ./... -timeout=300s
   ```

**Update this plan with:** Specific findings.

#### Step 2: Worktree Setup

```bash
cd /Users/adnaan/code/livetemplate/examples
git worktree add .worktrees/attr-reduction -b attr-reduction
```

#### Step 3: Update Dependencies

```bash
cd /Users/adnaan/code/livetemplate/examples/.worktrees/attr-reduction
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
cd /Users/adnaan/code/livetemplate/client && npm test

# 2. Core library tests
cd /Users/adnaan/code/livetemplate/livetemplate && GOWORK=off go test ./... -timeout=300s

# 3. lvt tests (including e2e)
cd /Users/adnaan/code/livetemplate/lvt && GOWORK=off go test ./... -timeout=300s

# 4. tinkerdown tests (including e2e)
cd /Users/adnaan/code/livetemplate/tinkerdown && GOWORK=off go test ./... -timeout=300s

# 5. examples tests
cd /Users/adnaan/code/livetemplate/examples && GOWORK=off go test ./... -timeout=300s
```

All 5 repos must have passing tests.

#### Step 7: Acceptance Criteria

- [ ] `examples` repo has zero deprecated attribute references in templates
- [ ] `examples` `go.mod` points to latest livetemplate version
- [ ] All 5 repos pass their test suites
- [ ] No remaining references to deprecated attributes across any repo (verify with grep)

#### Step 8: PR and Merge

```bash
cd /Users/adnaan/code/livetemplate/examples/.worktrees/attr-reduction
git add -A
git commit -m "chore: bump dependencies for attribute reduction"
git push origin attr-reduction
gh pr create --head attr-reduction --title "chore: bump deps for attribute reduction" \
  --body "Phase 4. Updates dependencies. No template changes needed (examples already use Tier 1 patterns)."
```

After merge:
```bash
cd /Users/adnaan/code/livetemplate/examples && git worktree remove .worktrees/attr-reduction
```

**Update this progress tracker:** Set Phase 4 to COMPLETE, fill in PR number.

---

### Cross-Phase Dependency Graph

```
Phase 1: client + livetemplate (core changes)
    |
    ├──→ Phase 2: lvt (template migration)
    |
    └──→ Phase 3: tinkerdown (code + template migration)
              |
              v
         Phase 4: examples (deps + final verification)
```

Phases 2 and 3 can proceed in parallel (lvt and tinkerdown don't depend on each other). Phase 4 depends on all prior phases being merged.

### PR Merge Order

1. **`client`** — publish new npm version (e.g., 1.0.0)
2. **`livetemplate`** — publish new Go version (e.g., v1.0.0)
3. **`lvt`** — depends on new client + server versions
4. **`tinkerdown`** — depends on new client + server versions
5. **`examples`** — depends on all above

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
