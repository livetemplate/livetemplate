# LiveTemplate

LiveTemplate is a Go library for building real-time web applications with minimal code. It uses tree-based DOM diffing to send only what changed over WebSocket or HTTP, inspired by Phoenix LiveView.

## Quick Start

```bash
go get github.com/livefir/livetemplate
```

## Basic Example

```go
package main

import (
    "log"
    "net/http"
    "github.com/livefir/livetemplate"
)

type CounterState struct {
    Counter int `json:"counter"`
}

func (s *CounterState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "increment":
        s.Counter++
    case "decrement":
        s.Counter--
    }
    return nil
}

func main() {
    state := &CounterState{Counter: 0}
    tmpl := livetemplate.New("counter") // auto-discovers counter.tmpl

    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

**counter.tmpl:**
```html
<!DOCTYPE html>
<html>
<body>
    <h1>Counter: {{.Counter}}</h1>
    <button lvt-click="increment">+</button>
    <button lvt-click="decrement">-</button>

    <script src="livetemplate-client.js"></script>
</body>
</html>
```

## How It Works

1. **Server**: Define state and actions using the `Store` interface
2. **Template**: Use `lvt-*` attributes to bind UI events to actions
3. **Client**: JavaScript library handles WebSocket/HTTP communication and DOM updates
4. **Updates**: Only changed data is sent using tree-based diffing

## Event Bindings

### Basic Events
```html
<!-- Click events -->
<button lvt-click="submit">Submit</button>

<!-- Form submission -->
<form lvt-submit="save">
    <input type="text" name="title">
    <button type="submit">Save</button>
</form>

<!-- Input changes -->
<input lvt-change="validate" name="email">
<input lvt-input="search" name="query">

<!-- Keyboard events -->
<input lvt-keydown="handleKey" lvt-key="Enter">
```

### Extended Events
```html
<!-- Focus/blur -->
<input lvt-focus="onFocus" lvt-blur="onBlur">

<!-- Mouse events -->
<div lvt-mouseenter="showTooltip" lvt-mouseleave="hideTooltip">Hover me</div>

<!-- Click away (detect clicks outside element) -->
<div lvt-click-away="close">Modal content</div>

<!-- Window events -->
<div lvt-window-keydown="globalShortcut" lvt-key="Escape">
<div lvt-window-scroll="handleScroll">
<div lvt-window-resize="handleResize">
```

### Rate Limiting
```html
<!-- Debounce: wait for user to stop typing -->
<input lvt-change="search" lvt-debounce="300">

<!-- Throttle: limit event frequency -->
<div lvt-window-scroll="updatePosition" lvt-throttle="100">
```

### Form Features
```html
<!-- Auto-validate on change -->
<form lvt-change="validate" lvt-submit="save">
    <input type="text" name="email" required>
    <button type="submit" lvt-disable-with="Saving...">Save</button>
</form>

<!-- Preserve form data on errors (default: form resets on success) -->
<form lvt-submit="save" lvt-preserve>
    <input type="text" name="title">
</form>
```

### Passing Data
```html
<!-- Simple data attributes -->
<button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>

<!-- Multiple data attributes -->
<button lvt-click="update"
    lvt-data-id="{{.ID}}"
    lvt-data-status="{{.Status}}">Update</button>
```

## Lifecycle Events

### Form Lifecycle
Forms emit lifecycle events you can listen to:

```javascript
const form = document.querySelector('form');

form.addEventListener('lvt:pending', (e) => {
    console.log('Action started');
});

form.addEventListener('lvt:success', (e) => {
    console.log('Action succeeded', e.detail);
});

form.addEventListener('lvt:error', (e) => {
    console.log('Validation errors', e.detail.errors);
});

form.addEventListener('lvt:done', (e) => {
    console.log('Action completed', e.detail);
});
```

### Element Lifecycle Hooks
```html
<!-- Inline JavaScript hooks -->
<div lvt-mounted="console.log('Element mounted', element)">
<div lvt-updated="console.log('Element updated', element)">
<div lvt-destroyed="console.log('Element removed', element)">
```

### Connection State Hooks
```javascript
const wrapper = document.querySelector('[data-lvt-id]');

wrapper.addEventListener('lvt:connected', () => {
    console.log('WebSocket connected');
});

wrapper.addEventListener('lvt:disconnected', () => {
    console.log('WebSocket disconnected');
});
```

## Validation

Server-side validation using go-playground/validator:

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

type TodoInput struct {
    Text string `json:"text" validate:"required,min=3"`
}

func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "add":
        var input TodoInput
        if err := ctx.BindAndValidate(&input, validate); err != nil {
            return err // Automatically shown in template
        }
        // ... add todo
    }
    return nil
}
```

Show errors in template:
```html
<form lvt-change="validate" lvt-submit="add">
    <input type="text" name="text"
        {{if .lvt.HasError "text"}}aria-invalid="true"{{end}}>

    {{if .lvt.HasError "text"}}
        <small>{{.lvt.Error "text"}}</small>
    {{end}}
</form>
```

## Tree-Based Optimization

LiveTemplate uses tree diffing to minimize data transfer:

**First Render (includes static structure):**
```json
{
    "s": ["<div>Count: ", "</div>"],
    "0": "5"
}
```

**Subsequent Updates (only dynamic values):**
```json
{
    "0": "6"
}
```

Static parts (`s`) are cached client-side. For complex templates with multiple dynamic values, this achieves 90%+ bandwidth savings.

## Examples

See the `examples/` directory:
- **counter/** - Simple counter with increment/decrement
- **todos/** - Todo app with validation and form lifecycle

## Client Library

The TypeScript client is built automatically:

```bash
cd client
npm install
npm run build
```

Include in your HTML:
```html
<script src="/livetemplate-client.js"></script>
```

The client automatically:
- Connects via WebSocket (with HTTP fallback)
- Handles event delegation for `lvt-*` attributes
- Applies DOM updates using morphdom
- Manages form lifecycle and validation errors

## LiveTemplate CLI (`lvt`)

The `lvt` CLI provides rapid application scaffolding with a complete components and kits system.

### Installation

```bash
go install github.com/livefir/livetemplate/cmd/lvt@latest
```

### Quick Start

```bash
# Create a new app
lvt new myapp --css tailwind
cd myapp

# Generate a CRUD resource
lvt gen products name price:float stock:int

# Start development server with hot reload
lvt serve
```

### Components & Kits System

LiveTemplate includes a powerful components and kits system for rapid UI development:

**Components** - Reusable UI template blocks (forms, tables, layouts, pagination)
**Kits** - CSS framework integrations (Tailwind, Bulma, Pico, or plain HTML)

```bash
# List available components
lvt components list

# List available kits
lvt kits list

# Create custom component
lvt components create alert --category feedback

# Create custom kit
lvt kits create mykit
```

### Development Server

The `lvt serve` command provides three development modes with hot reload:

- **Component Mode** - Live preview for component development
- **Kit Mode** - CSS helper showcase and testing
- **App Mode** - Full Go app with auto-rebuild and restart

```bash
# Auto-detect mode and start server
lvt serve

# Component development (in component directory)
cd ~/.lvt/components/mycomponent
lvt serve  # Opens live preview with JSON test data editor

# Kit development (in kit directory)
cd ~/.lvt/kits/mykit
lvt serve  # Shows CSS helper showcase

# App development (in app directory)
cd myapp
lvt serve  # Runs app with hot reload on file changes
```

### Documentation

Full documentation available in the `docs/` directory:

- **[User Guide](docs/user-guide.md)** - Getting started and usage
- **[Component Development](docs/component-development.md)** - Creating custom components
- **[Kit Development](docs/kit-development.md)** - Creating custom CSS kits
- **[Serve Guide](docs/serve-guide.md)** - Development server guide
- **[API Reference](docs/api-reference.md)** - Complete API reference

### CLI Commands

```bash
# App scaffolding
lvt new <name>              # Create new app
lvt gen <resource> [fields] # Generate CRUD resource
lvt gen view <name>         # Generate view-only handler

# Components & Kits
lvt components list         # List components
lvt components create <name>  # Create component
lvt components validate <path>  # Validate component
lvt kits list              # List kits
lvt kits create <name>     # Create kit
lvt kits validate <path>   # Validate kit

# Database
lvt migration up           # Run migrations
lvt migration down         # Rollback migration
lvt migration status       # Migration status

# Development
lvt serve                  # Start dev server with hot reload
lvt parse <file>          # Validate template syntax
```

### Building from Source

```bash
make build       # Build with timestamp
./lvt version    # Check version and build info
```

Build commands:
```bash
make build                    # Build with timestamp
make dev                      # Build with dev version tag
make install                  # Install to $GOPATH/bin
make build VERSION=v1.0.0     # Build with specific version
```

Without Make:
```bash
go build -ldflags "-X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o lvt cmd/lvt/*.go
```

## Testing

```bash
go test -v ./...
```

## License

MIT
