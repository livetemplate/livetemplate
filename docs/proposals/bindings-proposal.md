# LiveTemplate Bindings Proposal

## Overview

This proposal outlines a comprehensive, cohesive system for HTML bindings in LiveTemplate, inspired by Phoenix LiveView. The goal is to provide a complete set of `lvt-*` attributes that work together logically to handle common UI patterns like form validation, loading states, real-time updates, and lifecycle management.

## Motivation

Current limitations:

1. **No form lifecycle**: Forms don't auto-reset on success, no validation feedback during typing
2. **No loading states**: No way to disable buttons or show "Loading..." during actions
3. **Limited events**: Only basic click/submit/change, missing focus/blur/keydown filtering
4. **No rate limiting**: Can't debounce search inputs or throttle scroll handlers
5. **Fragmented patterns**: Each feature requires manual implementation in user code

Phoenix LiveView has solved these problems with a cohesive binding system. We can adapt their proven patterns while maintaining LiveTemplate's tree-based efficiency.

**Key insight:** Since LiveTemplate already tracks errors via the `Change(...) error` API, we can automatically detect success (no errors) and reset forms by default, with an opt-out attribute for special cases.

## Current State

**What exists:**

- `lvt-click="action"` - Click events
- `lvt-submit="action"` - Form submission
- `lvt-change="action"` - Input change events
- `lvt-input="action"` - Input events
- `lvt-keydown="action"` - Keydown events
- `lvt-keyup="action"` - Keyup events
- `lvt-data-*="value"` - Pass data to actions
- `lvt-value-*="value"` - Pass values to actions

**What's missing:**

- Form lifecycle (validation, reset, disable-with)
- Rate limiting (debounce, throttle)
- Event modifiers (key filtering)
- Extended events (focus, blur, click-away, window events, scroll)
- Lifecycle hooks (mounted, updated, destroyed, connected, disconnected)
- Auto-recovery (preserve state across reconnections)

## Complete Bindings Specification

### 1. Event Handlers

#### Basic Events

```html
lvt-click="action"
<!-- Click events -->
lvt-submit="action"
<!-- Form submission -->
lvt-change="action"
<!-- Change events (select, checkbox, radio) -->
lvt-input="action"
<!-- Input events (text inputs) -->
lvt-focus="action"
<!-- Focus events -->
lvt-blur="action"
<!-- Blur events -->
lvt-keydown="action"
<!-- Keydown events -->
lvt-keyup="action"
<!-- Keyup events -->
lvt-mouseenter="action"
<!-- Mouse enter -->
lvt-mouseleave="action"
<!-- Mouse leave -->
```

#### Window Events

For document-level events (useful for keyboard shortcuts, scroll tracking):

```html
lvt-window-keydown="action"
<!-- Global keydown -->
lvt-window-keyup="action"
<!-- Global keyup -->
lvt-window-scroll="action"
<!-- Window scroll -->
lvt-window-resize="action"
<!-- Window resize -->
lvt-window-focus="action"
<!-- Window focus -->
lvt-window-blur="action"
<!-- Window blur -->
```

#### Special Events

```html
lvt-click-away="action"
<!-- Click outside element -->
```

### 2. Event Modifiers

#### Rate Limiting

```html
lvt-debounce="300"
<!-- Wait 300ms after last event -->
lvt-throttle="1000"
<!-- Fire at most once per 1000ms -->
```

**Note:** If both `lvt-debounce` and `lvt-throttle` are present on the same element, `lvt-throttle` takes precedence.

Examples:

```html
<!-- Search input: wait for user to stop typing -->
<input lvt-input="search" lvt-debounce="300" />

<!-- Infinite scroll: limit scroll event processing -->
<div lvt-window-scroll="load_more" lvt-throttle="500"></div>
```

#### Key Filtering

```html
lvt-key="Enter"
<!-- Only trigger on Enter key -->
lvt-key="Escape"
<!-- Only trigger on Escape key -->
lvt-key="ArrowUp"
<!-- Only trigger on ArrowUp -->
```

Examples:

```html
<!-- Submit on Enter -->
<input lvt-keydown="submit_form" lvt-key="Enter" />

<!-- Close modal on Escape -->
<div lvt-window-keydown="close_modal" lvt-key="Escape"></div>

<!-- Navigate with arrows -->
<div lvt-keydown="next_item" lvt-key="ArrowDown"></div>
```

### 3. Form Bindings

#### Form-Level Attributes

```html
lvt-change="action"
<!-- Trigger on any form input change (for validation) -->
lvt-submit="action"
<!-- Trigger on form submit (auto-resets on success by default) -->
lvt-preserve
<!-- Opt-out: Don't reset form even on successful submission -->
lvt-auto-recover
<!-- Preserve form state across reconnections -->
```

#### Input-Level Attributes

```html
lvt-disable-with="text"
<!-- Replace button text and disable during action -->
```

#### Complete Form Example

```html
<form
  lvt-change="validate_todo"
  lvt-submit="add_todo"
  lvt-debounce="300">

  <input
    type="text"
    name="text"
    placeholder="What needs to be done?"
    {{if .lvt.HasError "text"}}aria-invalid="true"{{end}}>

  {{if .lvt.HasError "text"}}
    <small class="error">{{.lvt.Error "text"}}</small>
  {{end}}

  <button type="submit" lvt-disable-with="Adding...">
    Add Todo
  </button>
</form>
```

**Form Lifecycle Flow:**

1. User types → `lvt-change` triggers validation (debounced 300ms)
2. Server validates → Returns errors → `aria-invalid` set, errors shown
3. User clicks submit → Button shows "Adding...", becomes disabled
4. Server processes → Success (no errors) or failure (errors returned)
5. On success → Form auto-resets (default behavior), button re-enabled
6. On error → Errors shown, button re-enabled, form state preserved

### 4. Lifecycle Hooks

For integrating with JavaScript libraries (charts, maps, rich text editors):

```html
lvt-hook="hookName"
<!-- Connect element to JS hook -->
```

JavaScript side:

```typescript
const hooks = {
  chart: {
    mounted() {
      // Initialize chart when element first appears
      this.chart = new Chart(this.el, {...});
    },
    updated() {
      // Update chart when data changes
      this.chart.update(this.data);
    },
    destroyed() {
      // Cleanup when element removed
      this.chart.destroy();
    },
    connected() {
      // WebSocket connection established
      console.log('Connected to server');
    },
    disconnected() {
      // WebSocket connection lost
      console.log('Disconnected from server');
    }
  }
};

new LiveTemplate('/todos', { hooks });
```

**Lifecycle callbacks:**
- `mounted()` - Element added to DOM
- `updated()` - Element's data changed
- `destroyed()` - Element removed from DOM
- `connected()` - WebSocket connection established
- `disconnected()` - WebSocket connection lost

Template usage:

```html
<canvas lvt-hook="chart" data-chart-data="{{.ChartData}}"></canvas>
```

## Action Lifecycle States

Every action goes through states that can be used for UI feedback:

```
PENDING  → Action sent to server
   ↓
SUCCESS  → Server responded with no errors
   ↓
DONE     → Cleanup complete (form reset, re-enable buttons)

PENDING  → Action sent to server
   ↓
ERROR    → Server responded with errors
   ↓
DONE     → Cleanup complete (re-enable buttons, preserve form)
```

**Client-side events:**

```typescript
element.addEventListener("lvt:pending", () => {
  // Action sent to server
});

element.addEventListener("lvt:success", () => {
  // Action succeeded (no errors)
});

element.addEventListener("lvt:error", (e) => {
  // Action failed (errors present)
  console.log(e.detail.errors);
});

element.addEventListener("lvt:done", () => {
  // Action complete (success or error)
});
```

## Breaking Changes

### Removed Attributes

None - all existing `lvt-*` attributes remain compatible.

### Changed Behavior

1. **Form submission:**

   - **Before:** Forms with `lvt-submit` just send action, never reset
   - **After:** Forms with `lvt-submit` auto-reset on success (use `lvt-preserve` to disable)

2. **Error handling:**

   - **Before:** Errors stored, no automatic UI feedback
   - **After:** Forms automatically set `aria-invalid` on fields with errors

3. **Button states:**
   - **Before:** No automatic disabling
   - **After:** Buttons with `lvt-disable-with` auto-disable during action

## Implementation Phases

### Phase 1: Form Lifecycle (Highest Priority)

**Why first:** Solves the original problem (form clearing) and most common use case.

- `lvt-submit` - Auto-resets forms on success by default
- `lvt-preserve` - Opt-out attribute to prevent auto-reset
- `lvt-disable-with` - Disable buttons during submission
- Form-level `lvt-change` - Real-time validation
- Success/error state detection based on server errors
- Lifecycle events: `lvt:pending`, `lvt:success`, `lvt:error`, `lvt:done`

**Deliverable:** Todos example with validation + auto-reset

### Phase 2: Rate Limiting

**Why second:** Required for good UX on search/filter inputs.

- `lvt-debounce` - Wait for pause before triggering
- `lvt-throttle` - Rate limit event firing
- Apply to all event handlers

**Deliverable:** Search example with debounced input

### Phase 3: Extended Events

**Why third:** Expands use cases (keyboard shortcuts, modals, infinite scroll).

- `lvt-focus`, `lvt-blur`
- `lvt-window-*` events
- `lvt-click-away`
- `lvt-key` filtering
- `lvt-mouseenter`, `lvt-mouseleave`

**Deliverable:** Modal example with Escape to close, keyboard navigation example

### Phase 4: Lifecycle Hooks

**Why last:** Most advanced, requires stable foundation.

- `lvt-hook` attribute
- Hook registration API
- `mounted()`, `updated()`, `destroyed()` callbacks
- `connected()`, `disconnected()` for WebSocket state
- Data passing from template to hooks

**Deliverable:** Chart integration example with connection state handling

## Example Migrations

### Todos Example

**Before:**

```html
<form lvt-submit="add">
  <fieldset role="group">
    <input type="text" name="text" placeholder="What needs to be done?" required
    {{if .lvt.HasError "text"}}aria-invalid="true"{{end}}>
    <button type="submit">Add</button>
  </fieldset>
  {{if .lvt.HasError "text"}}
  <small style="color: var(--pico-del-color);">{{.lvt.Error "text"}}</small>
  {{end}}
</form>
```

**After:**

```html
<form lvt-change="validate" lvt-submit="add" lvt-debounce="300">
  <fieldset role="group">
    <input type="text" name="text" placeholder="What needs to be done?" required
    {{if .lvt.HasError "text"}}aria-invalid="true"{{end}}>
    <button type="submit" lvt-disable-with="Adding...">Add</button>
  </fieldset>
  {{if .lvt.HasError "text"}}
  <small style="color: var(--pico-del-color);">{{.lvt.Error "text"}}</small>
  {{end}}
</form>
```

**Backend:**

```go
// Add new "validate" action
func (s *TodoStore) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "validate":
        // Validate without saving
        var input struct {
            Text string `json:"text" validate:"required,min=3,max=100"`
        }
        return ctx.BindAndValidate(&input, s.validate)

    case "add":
        // Validate and save
        var input struct {
            Text string `json:"text" validate:"required,min=3,max=100"`
        }
        if err := ctx.BindAndValidate(&input, s.validate); err != nil {
            return err
        }

        s.mu.Lock()
        defer s.mu.Unlock()

        s.Todos = append(s.Todos, Todo{
            ID:        generateID(),
            Text:      input.Text,
            Completed: false,
        })
        s.updateStats()
        s.LastUpdated = time.Now().Format(time.RFC1123)
        return nil

    // ... other actions
    }
}
```

**Benefits:**

- Form auto-clears on success ✅ (original request, enabled by default)
- Real-time validation as user types
- Button disabled during submission
- Loading state shown to user
- No manual form handling needed
- Use `lvt-preserve` if you need to keep form state after success

### Counter Example

**Before:**

```html
<button lvt-click="increment">+</button>
<button lvt-click="decrement">-</button>
```

**After:**

```html
<button lvt-click="increment" lvt-disable-with="+...">+</button>
<button lvt-click="decrement" lvt-disable-with="-...">-</button>

<!-- Add keyboard shortcuts -->
<div
  lvt-window-keydown="increment"
  lvt-key="ArrowUp"
  lvt-window-keydown="decrement"
  lvt-key="ArrowDown"
></div>
```

**Benefits:**

- Buttons show loading state
- Keyboard navigation (↑/↓)
- Prevents double-clicks

## Additional Examples

### Search with Debounce

```html
<input
  type="search"
  name="query"
  lvt-input="search"
  lvt-debounce="300"
  placeholder="Search..."
/>

<div id="results">
  {{range .Results}}
  <div data-key="{{.ID}}">{{.Title}}</div>
  {{end}}
</div>
```

### Modal with Keyboard Close

```html
<div
  class="modal"
  lvt-window-keydown="close_modal"
  lvt-key="Escape"
  lvt-click-away="close_modal"
>
  <div class="modal-content">
    <h2>{{.ModalTitle}}</h2>
    <button lvt-click="close_modal">Close</button>
  </div>
</div>
```

### Infinite Scroll

```html
<div class="scrollable-feed" lvt-window-scroll="load_more" lvt-throttle="500">
  {{range .Items}}
  <div data-key="{{.ID}}">{{.Content}}</div>
  {{end}} {{if .HasMore}}
  <div class="loading">Loading more...</div>
  {{end}}
</div>
```

### Chart Integration

```html
<canvas
  lvt-hook="chart"
  data-chart-type="line"
  data-chart-data="{{.ChartData}}"
  width="800"
  height="400"
>
</canvas>
```

```typescript
const hooks = {
  chart: {
    mounted() {
      const type = this.el.dataset.chartType;
      const data = JSON.parse(this.el.dataset.chartData);
      this.chart = new Chart(this.el, { type, data });
    },
    updated() {
      const data = JSON.parse(this.el.dataset.chartData);
      this.chart.data = data;
      this.chart.update();
    },
    destroyed() {
      this.chart.destroy();
    },
  },
};
```

## Server Response Format

To support form lifecycle, server responses need metadata:

```json
{
  "tree": { ... },
  "meta": {
    "formId": "form-123",      // Which form triggered this
    "action": "add_todo",       // Which action was called
    "success": true,            // No validation errors
    "errors": {}                // Field errors (if any)
  }
}
```

This allows the client to:

1. Detect success vs error
2. Apply form-specific behavior (reset, preserve state)
3. Emit lifecycle events with context

## Implementation Notes

### Client-Side Changes

1. **Event delegation system** - Extend to support all new event types
2. **Rate limiting** - Add debounce/throttle wrappers
3. **Form tracking** - Track submitted forms, detect success/error
4. **Lifecycle events** - Emit `lvt:pending`, `lvt:success`, `lvt:error`, `lvt:done`
5. **Hook system** - Registry for lifecycle hooks, call mounted/updated/destroyed/connected/disconnected

### Server-Side Changes

1. **Form detection** - Identify when action came from form submission
2. **Response metadata** - Include form ID, success flag, errors in response
3. **No breaking changes** - All existing code continues to work

### Testing Strategy

1. **Unit tests** - Test each attribute independently
2. **Integration tests** - Test form lifecycle end-to-end
3. **Browser tests** - Use chromedp for E2E validation
4. **Example apps** - Migrate todos, counter; add search, modal, infinite scroll, chart

## Timeline

- **Phase 1** (Form Lifecycle): 1-2 days
- **Phase 2** (Rate Limiting): 1 day
- **Phase 3** (Extended Events): 1-2 days
- **Phase 4** (Lifecycle Hooks): 2-3 days

**Total: ~1 week** for complete implementation

## Design Decisions

1. **Event naming:** Using `lvt-window-*` for clarity (e.g., `lvt-window-keydown`)
2. **Rate limiting precedence:** `lvt-throttle` takes precedence over `lvt-debounce` when both present
3. **Form validation:** Opt-in via `lvt-change` attribute on forms
4. **DOM updates:** Tree diffing with `data-key` attributes already provides efficient streaming updates (no need for `lvt-update` attribute)
5. **Hook lifecycle:** Including `connected`/`disconnected` for WebSocket connection state management

## Conclusion

This proposal provides a complete, cohesive binding system for LiveTemplate that:

- ✅ Solves the original problem (form clearing on success)
- ✅ Provides comprehensive form lifecycle management
- ✅ Matches Phoenix LiveView's proven patterns
- ✅ Maintains LiveTemplate's tree-based efficiency
- ✅ Allows breaking changes (library unreleased)
- ✅ Phases implementation for incremental progress

The system is designed to work together logically - form bindings build on event handlers, rate limiting works with all events, lifecycle hooks integrate with DOM updates. Users learn one coherent system rather than disconnected attributes.
