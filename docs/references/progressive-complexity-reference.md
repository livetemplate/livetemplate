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
| `<button form="lvt-xxx-form" name="increment">` | Standalone button references auto-injected hidden form | `Increment()` |

## Action Resolution Order

When a form is submitted, the action is resolved in this order (first match wins):

1. `lvt-submit="X"` on the form (prefer button `name` or form `name` unless its needed for routing)
2. Clicked button's `name` attribute
3. Form's `name` attribute
4. Default: `"submit"` → `Submit()`

## Live Updates (Change Convention)

| Pattern | Behavior |
|---|---|
| Controller has `Change()` method | Server sends `capabilities: ["change"]`; client auto-wires debounced input events (300ms) |
| `<input name="X" value="{{.X}}">` | Auto-bound when `Change()` exists (dynamic value detected) |
| `<input name="X">` (no template expression) | NOT auto-bound (static only) |
| `lvt-debounce="500"` on input | Overrides default 300ms debounce |
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
| `command="show-modal" commandfor="id"` | Opens `<dialog>` (native browser) |
| `command="close" commandfor="id"` | Closes `<dialog>` (native browser) |
| `<form method="dialog">` inside `<dialog>` | Closes dialog AND routes action to server |

## Navigation Interception

All `<a href>` links inside the LiveTemplate wrapper are auto-intercepted for SPA navigation.

| Pattern | Intercepted? |
|---|---|
| `<a href="/path">` | Yes — fetched via `fetch()`, DOM patched, `pushState` updated |
| `<a href="/path" download>` | No — `download` attribute skips interception |
| `<a href="https://external.com">` | No — different origin skips interception |
| `<a href="/path" lvt-no-intercept>` | No — explicit opt-out |

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
| `form name` routing | N/A (use button name) | Client reads `form.name` | Client reads `form.name` |
| Hidden inputs | Native POST | In FormData | In FormData |
| `Change()` auto-binding | N/A | Works | Works |
| `lvt-*` attributes | N/A | Works | Works |
| Server push (broadcast) | N/A | N/A | Works |

## Tier 2: `lvt-*` Attributes

For behaviors that standard HTML cannot express — timing control, reactive DOM, keyboard shortcuts, scroll management — use `lvt-*` attributes. See the [Client Attributes Reference](client-attributes.md) for the complete listing.
