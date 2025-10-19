# Client Attributes Reference

Complete reference for LiveTemplate client-side `lvt-*` HTML attributes.

**For server-side Go API:** See [pkg.go.dev/github.com/livefir/livetemplate](https://pkg.go.dev/github.com/livefir/livetemplate)

## Table of Contents

- [Event Bindings](#event-bindings)
- [Data Passing](#data-passing)
- [Form Lifecycle Events](#form-lifecycle-events)
- [Validation](#validation)
- [Rate Limiting](#rate-limiting)
- [Multi-Store Pattern](#multi-store-pattern)
- [Attribute Reference](#attribute-reference)

---

## Event Bindings

LiveTemplate uses `lvt-*` attributes to bind DOM events to server-side actions.

### Basic Events

```html
<!-- Click events -->
<button lvt-click="submit">Submit</button>
<button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>

<!-- Form submission -->
<form lvt-submit="save">
    <input type="text" name="title" required>
    <button type="submit">Save</button>
</form>

<!-- Input events -->
<input lvt-change="validate" name="email">
<input lvt-input="search" name="query">
```

### Mouse Events

```html
<!-- Hover events -->
<div lvt-mouseenter="showTooltip" lvt-mouseleave="hideTooltip">
    Hover for tooltip
</div>

<!-- Click events -->
<button lvt-click="handleClick">Click me</button>
```

### Keyboard Events

```html
<!-- Keydown events -->
<input lvt-keydown="handleKey" name="search">

<!-- With key filtering -->
<input lvt-keydown="submit" lvt-key="Enter" name="query">
<div lvt-window-keydown="closeModal" lvt-key="Escape">
    Modal content
</div>
```

### Window Events

```html
<!-- Global keyboard events -->
<div lvt-window-keydown="handleShortcut" lvt-key="Escape">

<!-- Scroll events -->
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

---

## Data Passing

Pass data from the DOM to your server-side action handlers using `lvt-data-*` attributes.

### Simple Data

```html
<button lvt-click="delete" lvt-data-id="{{.ID}}">
    Delete
</button>
```

### Multiple Data Attributes

```html
<button lvt-click="update"
    lvt-data-id="{{.ID}}"
    lvt-data-status="{{.Status}}"
    lvt-data-priority="{{.Priority}}">
    Update Item
</button>
```

### Accessing Data in Go

```go
func (s *State) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "delete":
        id := ctx.GetString("id")
        // Delete item with id

    case "update":
        id := ctx.GetString("id")
        status := ctx.GetString("status")
        priority := ctx.GetInt("priority")
        // Update item
    }
    return nil
}
```

**Available methods:**
- `ctx.GetString(key string) string`
- `ctx.GetInt(key string) int`
- `ctx.GetFloat(key string) float64`
- `ctx.GetBool(key string) bool`
- `ctx.Has(key string) bool`

---

## Form Lifecycle Events

Forms emit JavaScript events during the action lifecycle that you can listen to.

### Event Types

```javascript
const form = document.querySelector('form');

// Fires when action starts
form.addEventListener('lvt:pending', (e) => {
    console.log('Submitting...');
    // Show loading spinner
});

// Fires on validation errors
form.addEventListener('lvt:error', (e) => {
    console.log('Errors:', e.detail.errors);
    // Display error messages
});

// Fires on successful action (no errors)
form.addEventListener('lvt:success', (e) => {
    console.log('Saved!');
    // Show success message, redirect, etc.
});

// Always fires when action completes (success or error)
form.addEventListener('lvt:done', (e) => {
    console.log('Completed');
    // Hide loading spinner
});
```

### Event Detail

```javascript
form.addEventListener('lvt:success', (e) => {
    console.log(e.detail);
    // {
    //   action: "save",
    //   data: {...},
    //   meta: {
    //     success: true,
    //     errors: {}
    //   }
    // }
});
```

---

## Validation

LiveTemplate provides server-side validation with automatic error display.

### Server-Side Validation

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

type TodoInput struct {
    Title string `json:"title" validate:"required,min=3,max=100"`
    Tags  string `json:"tags" validate:"required"`
}

func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "add":
        var input TodoInput
        if err := ctx.BindAndValidate(&input, validate); err != nil {
            return err // Errors automatically sent to client
        }
        // Input is valid, proceed
        s.Todos = append(s.Todos, Todo{Title: input.Title})
    }
    return nil
}
```

### Template Error Display

```html
<form lvt-submit="add">
    <div>
        <label for="title">Title</label>
        <input
            type="text"
            name="title"
            id="title"
            {{if .lvt.HasError "title"}}aria-invalid="true"{{end}}>

        {{if .lvt.HasError "title"}}
            <small class="error">{{.lvt.Error "title"}}</small>
        {{end}}
    </div>

    <button type="submit">Add Todo</button>
</form>
```

### Error Helpers

**In templates:**
- `{{.lvt.HasError "field"}}` - Check if field has error
- `{{.lvt.Error "field"}}` - Get error message for field
- `{{.lvt.Errors}}` - Get all errors map

---

## Rate Limiting

Control how often events are processed using debounce and throttle.

### Debounce

Wait for user to stop typing before triggering action.

```html
<!-- Wait 300ms after user stops typing -->
<input
    lvt-input="search"
    lvt-debounce="300"
    name="query"
    placeholder="Search...">
```

**Use for:** Search inputs, auto-save, validation

### Throttle

Limit event frequency to at most once per interval.

```html
<!-- Fire at most once every 100ms -->
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

**Use for:** Scroll events, resize events, mouse tracking

---

## Multi-Store Pattern

Use namespaced actions for applications with multiple state stores.

### Server Setup

```go
stores := livetemplate.Stores{
    "counter": &CounterState{},
    "todos":   &TodosState{},
    "user":    &UserState{},
}

handler := livetemplate.HandleStores(tmpl, stores)
http.Handle("/", handler)
```

### Template Usage

```html
<!-- Namespaced actions: store.action -->
<button lvt-click="counter.increment">+</button>
<button lvt-click="counter.decrement">-</button>

<form lvt-submit="todos.add">
    <input type="text" name="text">
    <button type="submit">Add Todo</button>
</form>

<button lvt-click="user.logout">Logout</button>
```

Each action is routed to the corresponding store's `Change()` method.

---

## Attribute Reference

Complete reference of all `lvt-*` attributes.

### Event Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-click` | Click event on element | `<button lvt-click="save">` |
| `lvt-submit` | Form submission | `<form lvt-submit="create">` |
| `lvt-change` | Input change event | `<input lvt-change="validate">` |
| `lvt-input` | Input event (every keystroke) | `<input lvt-input="search">` |
| `lvt-keydown` | Keydown event | `<input lvt-keydown="submit">` |
| `lvt-keyup` | Keyup event | `<input lvt-keyup="handle">` |
| `lvt-mouseenter` | Mouse enter event | `<div lvt-mouseenter="show">` |
| `lvt-mouseleave` | Mouse leave event | `<div lvt-mouseleave="hide">` |
| `lvt-window-keydown` | Global keydown | `<div lvt-window-keydown="close">` |
| `lvt-window-scroll` | Window scroll | `<div lvt-window-scroll="load">` |

### Data Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-data-<key>` | Pass string data | `lvt-data-id="{{.ID}}"` |
| `lvt-data-<key>` | Pass any data type | `lvt-data-count="{{.Count}}"` |

**Note:** All `lvt-data-*` attributes are passed to `ActionContext.Data` with the key being the part after `lvt-data-`.

### Modifier Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `lvt-key` | Filter keyboard events by key | `lvt-key="Enter"` |
| `lvt-debounce` | Debounce delay in milliseconds | `lvt-debounce="300"` |
| `lvt-throttle` | Throttle interval in milliseconds | `lvt-throttle="100"` |

### Valid Key Values

For `lvt-key` attribute:

- Letter keys: `"a"`, `"b"`, `"c"`, etc.
- Special keys: `"Enter"`, `"Escape"`, `"Space"`, `"Tab"`, `"Backspace"`, `"Delete"`
- Arrow keys: `"ArrowUp"`, `"ArrowDown"`, `"ArrowLeft"`, `"ArrowRight"`
- Function keys: `"F1"`, `"F2"`, etc.
- Modifiers: Check `e.ctrlKey`, `e.shiftKey`, `e.altKey`, `e.metaKey` in event listeners

---

## Best Practices

### 1. Use Debounce for Search

```html
<input
    lvt-input="search"
    lvt-debounce="300"
    name="query">
```

### 2. Use Throttle for Scroll

```html
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

### 3. Namespace Multi-Store Actions

```html
<button lvt-click="todos.add">Add</button>
<button lvt-click="user.logout">Logout</button>
```

### 4. Show Validation Errors

```html
<input
    type="email"
    name="email"
    {{if .lvt.HasError "email"}}aria-invalid="true"{{end}}>
{{if .lvt.HasError "email"}}
    <span class="error">{{.lvt.Error "email"}}</span>
{{end}}
```

### 5. Handle Form Lifecycle

```javascript
form.addEventListener('lvt:pending', () => {
    submitButton.disabled = true;
});

form.addEventListener('lvt:done', () => {
    submitButton.disabled = false;
});
```

---

## Advanced Usage

### Custom Event Handling

```javascript
document.addEventListener('lvt:connected', () => {
    console.log('WebSocket connected');
});

document.addEventListener('lvt:disconnected', () => {
    console.log('WebSocket disconnected');
});
```

### Accessing Form Data

```javascript
form.addEventListener('lvt:pending', (e) => {
    const formData = new FormData(e.target);
    console.log('Submitting:', Object.fromEntries(formData));
});
```

---

## Related Documentation

- **[Go API Reference](https://pkg.go.dev/github.com/livefir/livetemplate)** - Server-side API
- **[Error Handling Reference](error-handling.md)** - Validation, error display, client-side handling
- **[Template Support Matrix](template-support-matrix.md)** - Supported Go template features
- **[Architecture](../ARCHITECTURE.md)** - System architecture
- **[User Guide](../guides/user-guide.md)** - Getting started with CLI
- **[Contributing Guide](../../CONTRIBUTING.md)** - How to contribute
