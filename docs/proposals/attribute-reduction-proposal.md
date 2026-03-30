# Attribute Surface Reduction

**Status:** Proposal
**Date:** 2026-03-30
**Issue:** [#288](https://github.com/livetemplate/livetemplate/issues/288)

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

**Impact: 7 attributes eliminated, 2 narrowed in scope.**

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
| `lvt-upload` | File upload identifier — framework-specific protocol |

## Category 4: Needs Discussion

These attributes are technically needed but their real-world usage may be rare enough to not justify the surface area.

| Attribute | Question |
|-----------|----------|
| `lvt-input` | Is it needed when `Change()` convention exists? Difference: `lvt-input` routes to a named action, `Change()` always routes to `Change`. Could `Change()` accept action name routing instead? |
| `lvt-focus`, `lvt-blur` | How common are server-round-trip focus/blur handlers? If usage is rare across real apps, consider removing. |
| `lvt-mouseenter`, `lvt-mouseleave` | Server round-trip on hover has latency concerns. May cause poor UX if network is slow. Consider whether these encourage bad patterns. |
| `lvt-disable-with` | Could be replaced by CSS: `button[aria-busy="true"] { ... }` with `::after` for text. Less ergonomic but zero custom attributes. Worth the tradeoff? |

## Before and After

| Category | Before | After |
|----------|--------|-------|
| **Total `lvt-*` attributes** | ~51 | ~37 |
| Event bindings | 11 | 10 (narrowed scope on 2) |
| Data passing | 2 patterns (`lvt-data-*`, `lvt-value-*`) | 0 (standard HTML) |
| Modals | 2 | 0 (native `<dialog>`) |
| Legacy routing | 2 (`lvt-submit`, `lvt-action`) | 0 (deprecated) |
| Confirmation | 1 | 0 (standard `onsubmit`) |
| Scroll directives | 3 | 1 + CSS custom properties |
| Highlight directives | 3 | 1 + CSS custom properties |
| Animation directives | 2 | 1 + CSS custom properties |
| Enable/disable sugar | 2 | 0 (use `lvt-toggleAttr-on`) |
| Timing, keyboard, reactive, form, upload | 25 | 25 (unchanged — essential) |

## Migration Path

### Phase 1: Documentation (no breaking changes)
- Update client-attributes.md to mark eliminated attributes as **deprecated**
- Add migration examples showing standard HTML alternatives
- Update progressive complexity guide with narrowed `lvt-click`/`lvt-change` guidance

### Phase 2: Deprecation Warnings (minor version)
- Client emits console warnings when deprecated attributes are used
- Warnings include the specific standard HTML replacement

### Phase 3: CSS Consolidation (minor version)
- Client ships `livetemplate.css` with CSS custom property defaults
- Scroll/highlight/animate configuration attributes removed
- Client reads `--lvt-*` custom properties from computed styles

### Phase 4: Removal (major version)
- Remove deprecated attributes from client
- Remove `lvt-data-*`, `lvt-value-*`, `lvt-modal-open/close`, `lvt-confirm`
- Remove `disable-on`/`enable-on` sugar

## References

- [Progressive Complexity Proposal](progressive-complexity-proposal.md) — Foundation for Tier 1/Tier 2 model
- [Client Attributes Reference](../references/client-attributes.md) — Current complete attribute listing
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Quick reference
- [HTML Invoker Commands](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/command) — `command`/`commandfor` spec
