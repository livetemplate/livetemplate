# Progressive Complexity Reference

Quick-reference for how standard HTML maps to LiveTemplate behavior. For the learning walkthrough, see the [Progressive Complexity Guide](../guides/progressive-complexity.md). For `lvt-*` attributes, see the [Client Attributes Reference](client-attributes.md).

---

## Form Routing

| HTML Pattern | Framework Behavior | Go Method |
|---|---|---|
| `<form method="POST">` | Auto-intercepted, routes to default action | `Submit()` |
| `<button name="save">` | Button name becomes action | `Save()` |
| `<button name="saveDraft">` | camelCase names converted to PascalCase | `SaveDraft()` |
| `<button name="delete" value="{{.ID}}">` | Value passed as data | `ctx.GetString("value")` |
| `<form name="search">` | Form name becomes action (JS client only) | `Search()` |
| `<input type="hidden" name="id" value="{{.ID}}">` | Included in form data | `ctx.GetString("id")` |
| `<button name="increment">` | Standalone button outside any form (JS client only) | `Increment()` |

## Action Resolution Order (Tier 1)

When a standard HTML form is submitted, the action is resolved in this order (first match wins):

1. Clicked button's `name` attribute
2. Form's `name` attribute
3. Default: `"submit"` → `Submit()`

For the full resolution order including `lvt-form:action` and other Tier 2 attributes, see [Client Attributes Reference — Action Resolution Order](client-attributes.md#action-resolution-order).

## Live Updates (Change Convention)

| Pattern | Behavior |
|---|---|
| Controller has `Change()` method | Server sends `capabilities: ["change"]`; client auto-wires debounced input events (300ms) |
| `<input name="X" value="{{.X}}">` | Auto-bound when `Change()` exists (dynamic value detected) |
| `<input name="X">` (no template expression) | NOT auto-bound (static only) |
| `lvt-mod:debounce="500"` on input | Overrides default 300ms debounce |
| No `Change()` method | Form is submit-only, no auto-binding |

## Validation Inference

HTML validation attributes are extracted by `ctx.ValidateForm()`:

| HTML Attribute | Server Validation |
|---|---|
| `required` | Field must not be empty |
| `type="email"` | Must be valid email format |
| `type="url"` | Must be valid URL |
| `type="number"` | Must be numeric |
| `minlength="N"` | Minimum N characters |
| `maxlength="N"` | Maximum N characters |
| `min="N"` | Numeric minimum |
| `max="N"` | Numeric maximum |
| `pattern="regex"` | Must match regex |
| `formnovalidate` on button | Skips validation for that action |

## Dialog Routing

| HTML Pattern | Framework Behavior |
|---|---|
| `command="show-modal" commandfor="id"` | Opens `<dialog>` via `.showModal()` (polyfilled by client for cross-browser support) |
| `command="close" commandfor="id"` | Closes `<dialog>` via `.close()` (polyfilled by client) |
| `<form>` inside `<dialog>` | Routes action to server; dialog auto-closes on success |

> **Browser support:** The [Invoker Commands API](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/command) (`command`/`commandfor`) is natively supported in Chrome 135+. The LiveTemplate client includes a lightweight polyfill for Firefox and Safari, using feature detection (`commandForElement`) to become a no-op when native support lands.

## Navigation Interception

All `<a href>` links inside the LiveTemplate wrapper are auto-intercepted for SPA navigation.

| Pattern | Intercepted? |
|---|---|
| `<a href="/path">` | Yes — fetched via `fetch()`, DOM patched, `pushState` updated |
| `<a href="/path" download>` | No — `download` attribute skips interception |
| `<a href="https://external.com">` | No — different origin skips interception |
| `<a href="/path" lvt-nav:no-intercept>` | No — explicit opt-out for links (`lvt-form:no-intercept` for forms) |

## Loading States (Automatic)

During form submission, the framework automatically manages loading indicators:

| HTML Pattern | Automatic Behavior |
|---|---|
| `<form>` during submit | `aria-busy="true"` set on form |
| `<fieldset>` inside submitting form | `disabled` attribute set |
| Server responds | Both `aria-busy` and `disabled` cleared |

## Transport Compatibility

| Feature | No JS | JS + HTTP | JS + WebSocket |
|---|---|---|---|
| Form submit | POST + page reload | `fetch()` + DOM patch | WS message + DOM patch |
| `button name` routing | Native POST | Client extracts | Client extracts |
| Standalone button (no form) | N/A (use form) | Client detects | Client detects |
| `form name` routing | N/A (use button name) | Client reads `form.name` | Client reads `form.name` |
| Hidden inputs | Native POST | In FormData | In FormData |
| `Change()` auto-binding | N/A | Works | Works |
| `lvt-*` attributes | N/A | Works | Works |
| Server push (broadcast) | N/A | N/A | Works |

## Tier 2: `lvt-*` Attributes

For behaviors that standard HTML cannot express — timing control, reactive DOM, keyboard shortcuts, scroll management — use `lvt-*` attributes. The attribute prefixes are:

| Prefix | Purpose | Example |
|---|---|---|
| `lvt-on:` | Event bindings | `lvt-on:click="save"`, `lvt-on:window:keydown="close"` |
| `lvt-el:` | Reactive DOM manipulation | `lvt-el:addClass:on:pending="loading"` |
| `lvt-fx:` | Visual directives | `lvt-fx:scroll="bottom"`, `lvt-fx:highlight="flash"` |
| `lvt-mod:` | Event modifiers | `lvt-mod:debounce="300"`, `lvt-mod:throttle="100"` |
| `lvt-form:` | Form behavior | `lvt-form:preserve`, `lvt-form:disable-with="Saving..."` |

See the [Client Attributes Reference](client-attributes.md) for the complete listing.

### Recipe: Chat with Infinite Scroll

Compose multiple `lvt-*` attributes for a chat UI with upward infinite scroll, auto-scroll on new messages, and a scroll-to-bottom button — all without custom JavaScript:

```html
<div class="chat-scroll-wrap">
  <div class="chat-log" id="chat-log" lvt-fx:scroll="bottom-sticky"
       style="--lvt-scroll-threshold: 80;" data-key="chat-{{.SessionID}}">
    {{if .HasMore}}
    <div lvt-scroll-sentinel data-key="sentinel">
      <small aria-busy="true">Loading older messages…</small>
    </div>
    {{end}}
    {{range .Messages}}
    <div class="chat-row {{.Role}}" data-key="{{.Key}}">
      <div class="chat-bubble">{{.Text}}</div>
    </div>
    {{end}}
  </div>
  <button type="button" class="scroll-bottom-btn"
          lvt-fx:scroll:on:click="bottom"
          lvt-scroll-away="bottom"
          data-lvt-target="#chat-log"
          aria-label="Scroll to bottom">↓</button>
</div>
```

How the attributes compose:

| Attribute | Role |
|-----------|------|
| `lvt-fx:scroll="bottom-sticky"` | Auto-scrolls on new messages if user is near bottom; scrolls to bottom on first load |
| `lvt-scroll-sentinel` | IntersectionObserver triggers `LoadMore()` when sentinel enters viewport |
| `data-key` on rows | Differential DOM updates — only new/changed rows are sent over the wire |
| `lvt-fx:scroll:on:click="bottom"` | Scrolls target to bottom on button click |
| `lvt-scroll-away="bottom"` | Shows button only when scrolled away from bottom |
| `data-key` on chat-log | Session switches replace the element, resetting the sticky scroll marker |

Backend: `LoadMore()` increments a page counter; the handler slices messages from the tail of the full list. `lvt-scroll-sentinel` is conditionally rendered with `{{if .HasMore}}` to stop loading when all messages are shown.
