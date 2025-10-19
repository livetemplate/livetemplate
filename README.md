# LiveTemplate

Build real-time, reactive web applications in Go with minimal code. LiveTemplate uses tree-based DOM diffing to send only what changed over WebSocket or HTTP, inspired by Phoenix LiveView.

**[Quick Start](#quick-start)** • **[Documentation](#documentation)** • **[Examples](#examples)** • **[CLI Tool](#cli-tool)** • **[Contributing](CONTRIBUTING.md)**

---

## Why LiveTemplate?

LiveTemplate brings Phoenix LiveView's developer experience to Go:

- **Server-side state** - Your state lives in Go, not scattered across client and server
- **Automatic updates** - Change state, UI updates automatically. No manual DOM manipulation
- **Ultra-efficient** - Tree-based diffing sends 50-90% less data than full HTML
- **Type-safe** - Leverage Go's type system for your entire application
- **Zero frontend build** - No webpack, no npm dependencies for your app
- **Production-ready** - Built-in session management, validation, and error handling

### When to Use LiveTemplate

**Perfect for:**
- Admin dashboards and internal tools
- Real-time collaborative features
- Forms with complex validation
- Server-side state is your source of truth
- Teams that prefer Go over JavaScript frameworks

**Consider alternatives if:**
- You need offline-first capabilities
- Heavy client-side interactions (games, drawing apps)
- Mobile app with native feel
- SEO is critical and SSR isn't enough

## Quick Start

### Installation

```bash
go get github.com/livefir/livetemplate
```

### Your First App (5 minutes)

**1. Create your state**

```go
// main.go
package main

import (
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
    case "reset":
        s.Counter = 0
    }
    return nil
}

func main() {
    state := &CounterState{Counter: 0}
    tmpl := livetemplate.New("counter")

    http.Handle("/", tmpl.Handle(state))
    http.ListenAndServe(":8080", nil)
}
```

**2. Create your template**

```html
<!-- counter.tmpl -->
<!DOCTYPE html>
<html>
<head>
    <title>Counter</title>
</head>
<body>
    <h1>Counter: {{.Counter}}</h1>
    <button lvt-click="increment">+</button>
    <button lvt-click="decrement">-</button>
    <button lvt-click="reset">Reset</button>

    <script src="https://cdn.jsdelivr.net/npm/@livefir/livetemplate-client@latest/dist/livetemplate-client.min.js"></script>
</body>
</html>
```

**3. Run it**

```bash
go run main.go
# Open http://localhost:8080
```

That's it! Click buttons and watch the counter update in real-time.

## How It Works

```
User clicks button → Client sends action → Server updates state →
Server renders template → Tree diff → Minimal update → Client applies patch
```

1. **Define state**: Your Go struct holds application state
2. **Handle actions**: Implement `Change(ctx)` to handle user interactions
3. **Render template**: Use standard Go templates with `lvt-*` attributes
4. **Automatic updates**: LiveTemplate handles the rest

## Comparison with Other Frameworks

| Feature | LiveTemplate | Phoenix LiveView | Datastar | HTMX | Hotwire Turbo | Alpine.js |
|---------|-------------|------------------|----------|------|---------------|-----------|
| **Language** | Go | Elixir | Any | Any | Any | JavaScript |
| **Approach** | Server-side state | Server-side state | Hypermedia + Signals | HTML over wire | HTML over wire | Client-side only |
| **State Location** | Server (Go) | Server (Elixir) | Client + Server | Server | Server | Client (JS) |
| **Transport** | WebSocket/HTTP | WebSocket | SSE/WebSocket | HTTP | HTTP | N/A |
| **Update Mechanism** | Tree diffing | Minimal DOM patches | Signal updates | Full/partial HTML | Full page/frame | Reactive JS |
| **Bandwidth Efficiency** | 50-90% reduction | Very high | Medium-High | Medium | Medium-Low | N/A |
| **Learning Curve** | Low (Go + templates) | Medium (Elixir) | Low-Medium | Very low | Very low | Low |
| **Type Safety** | Full (Go) | Full (Elixir) | Depends on backend | Depends on backend | Depends on backend | None |
| **Real-time Support** | Built-in | Built-in | Built-in (SSE) | Polling only | Polling/Streams | Requires backend |
| **Client JS Required** | Minimal (~15KB) | Minimal | Minimal (~10KB) | Minimal (~14KB) | Minimal (~30KB) | Full framework |
| **Form Validation** | Server-side | Server-side | Server or client | Server-side | Server-side | Client-side |
| **Best For** | Go apps, dashboards | Elixir ecosystem | Hypermedia apps | Simple interactions | Rails/Django apps | Enhancing static sites |
| **Maturity** | Alpha | Production | Alpha | Production | Production | Production |

### Key Differentiators

**vs Phoenix LiveView**
- LiveTemplate brings the LiveView pattern to Go
- Simpler deployment (single binary vs Elixir/Erlang VM)
- Go's performance and ecosystem
- Tree-based approach vs LiveView's DOM patch commands

**vs HTMX**
- LiveTemplate maintains WebSocket connections for instant updates
- More efficient updates (tree diff vs full HTML)
- Built-in state management
- HTMX is simpler but less efficient for frequent updates

**vs Datastar**
- Both are inspired by LiveView, but different approaches
- LiveTemplate uses WebSocket/tree diff, Datastar uses SSE/signals
- LiveTemplate is Go-focused, Datastar is language-agnostic
- Similar goals, different implementation strategies

**vs Hotwire Turbo**
- LiveTemplate is for real-time apps, Turbo for enhanced navigation
- WebSocket vs HTTP
- Turbo focuses on full-page updates, LiveTemplate on surgical patches
- Complementary: Turbo for navigation, LiveTemplate for dynamic UIs

**vs Alpine.js**
- LiveTemplate is server-centric, Alpine is client-centric
- Use together: Alpine for local UI state, LiveTemplate for server state
- Alpine requires no backend integration, LiveTemplate requires server

## Features

### Event Bindings

```html
<!-- Click events -->
<button lvt-click="submit">Submit</button>

<!-- Form submission with validation -->
<form lvt-submit="save">
    <input type="text" name="title" required>
    <button type="submit">Save</button>
</form>

<!-- Input events -->
<input lvt-change="validate" name="email">
<input lvt-input="search" lvt-debounce="300" name="query">

<!-- Keyboard events -->
<input lvt-keydown="handleKey" lvt-key="Enter">

<!-- Mouse events -->
<div lvt-mouseenter="show" lvt-mouseleave="hide">Hover me</div>

<!-- Window events -->
<div lvt-window-keydown="closeModal" lvt-key="Escape">
<div lvt-window-scroll="loadMore" lvt-throttle="100">
```

### Passing Data

```html
<!-- Simple data -->
<button lvt-click="delete" lvt-data-id="{{.ID}}">Delete</button>

<!-- Multiple data attributes -->
<button lvt-click="update"
    lvt-data-id="{{.ID}}"
    lvt-data-status="{{.Status}}"
    lvt-data-priority="{{.Priority}}">
    Update
</button>
```

Access in Go:

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

### Validation

Server-side validation with automatic error display:

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
            return err // Errors automatically available in template
        }
        // Add todo
    }
    return nil
}
```

Show errors in template:

```html
<form lvt-submit="add">
    <div>
        <input type="text" name="title"
            {{if .lvt.HasError "title"}}aria-invalid="true"{{end}}>
        {{if .lvt.HasError "title"}}
            <small>{{.lvt.Error "title"}}</small>
        {{end}}
    </div>
    <button type="submit">Add Todo</button>
</form>
```

### Form Lifecycle

```javascript
const form = document.querySelector('form');

// Action started
form.addEventListener('lvt:pending', (e) => {
    console.log('Submitting...');
});

// Validation errors
form.addEventListener('lvt:error', (e) => {
    console.log('Errors:', e.detail.errors);
});

// Success
form.addEventListener('lvt:success', (e) => {
    console.log('Saved!');
});

// Always fired
form.addEventListener('lvt:done', (e) => {
    console.log('Completed');
});
```

### Multi-Store Pattern

For complex apps, use multiple stores:

```go
stores := livetemplate.Stores{
    "counter": &CounterState{},
    "todos":   &TodosState{},
    "user":    &UserState{},
}

handler := livetemplate.HandleStores(tmpl, stores)
http.Handle("/", handler)
```

```html
<!-- Namespaced actions -->
<button lvt-click="counter.increment">+</button>
<button lvt-click="todos.add">Add Todo</button>
<button lvt-click="user.logout">Logout</button>
```

### Broadcasting

Share state updates across all connected clients:

```go
type ChatState struct {
    Messages []Message
}

func (s *ChatState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "send":
        msg := Message{
            Text: ctx.GetString("text"),
            Time: time.Now(),
        }
        s.Messages = append(s.Messages, msg)

        // Broadcast to all connected clients
        if b, ok := ctx.Broadcaster(); ok {
            b.Broadcast(s)
        }
    }
    return nil
}
```

### Tree-Based Optimization

LiveTemplate achieves 50-90% bandwidth savings through tree diffing:

**First render (full tree with statics):**
```json
{
    "s": ["<div>Counter: ", "</div>"],
    "0": "5"
}
```

**Subsequent updates (only changed dynamics):**
```json
{
    "0": "6"
}
```

Static parts (`s`) are cached client-side and referenced by ID. For templates with lots of static HTML and few dynamic values, this is extremely efficient.

## Examples

### Counter
Simple increment/decrement counter demonstrating basic state management.

```bash
cd examples/counter
go run main.go
# Open http://localhost:8080
```

### Todos
Full CRUD application with validation, forms, and lifecycle events.

```bash
cd examples/todos
go run main.go
# Open http://localhost:8080
```

### Source Code
Both examples are ~100 lines of Go + template. See `examples/` directory for complete code.

## CLI Tool

The `lvt` CLI provides rapid application scaffolding with components and CSS framework kits.

### Installation

```bash
go install github.com/livefir/livetemplate/cmd/lvt@latest
```

### Quick Start

```bash
# Create new app with Tailwind CSS
lvt new myapp --css tailwind
cd myapp

# Generate CRUD resource
lvt gen products name price:float stock:int

# Start dev server with hot reload
lvt serve
```

### Features

- **App Scaffolding**: Generate complete apps with routing and database
- **CRUD Generation**: Instant CRUD with forms, validation, tables
- **CSS Kits**: Tailwind, Bulma, Pico, or plain HTML
- **Components**: Reusable UI blocks (forms, tables, layouts, pagination)
- **Hot Reload**: Auto-rebuild and restart on file changes
- **Database Migrations**: Built-in migration management

### Commands

```bash
# App commands
lvt new <name>                 # Create new app
lvt gen <resource> [fields]    # Generate CRUD resource
lvt gen view <name>            # Generate view-only handler

# Development
lvt serve                      # Start dev server with hot reload

# Components & Kits
lvt kits list                  # List available CSS kits
lvt kits create <name>         # Create custom kit
lvt components list            # List available components
lvt components create <name>   # Create custom component

# Database
lvt migration up               # Run migrations
lvt migration down             # Rollback migrations
lvt migration status           # Show migration status
```

### CLI Documentation

Full CLI documentation:
- **[User Guide](docs/user-guide.md)** - Getting started with CLI
- **[Component Development](docs/component-development.md)** - Creating components
- **[Kit Development](docs/kit-development.md)** - Creating CSS kits
- **[Serve Guide](docs/serve-guide.md)** - Development server

## Client Library

The TypeScript client handles WebSocket connections, event delegation, and DOM updates.

### CDN

```html
<script src="https://cdn.jsdelivr.net/npm/@livefir/livetemplate-client@latest/dist/livetemplate-client.min.js"></script>
```

### Build from Source

```bash
cd client
npm install
npm run build
```

The client (~15KB minified):
- Connects via WebSocket with automatic reconnection
- Falls back to HTTP for browsers without WebSocket support
- Handles event delegation for `lvt-*` attributes
- Applies DOM updates efficiently using morphdom
- Manages form lifecycle and validation errors
- Preserves input focus and scroll position

## Documentation

### Core Documentation
- **[Contributing Guide](CONTRIBUTING.md)** - How to contribute
- **[Architecture](docs/ARCHITECTURE.md)** - System architecture and design
- **[Code Tour](docs/CODE_TOUR.md)** - Guided walkthrough for newcomers

### API Documentation
- **[API Reference](docs/api-reference.md)** - Complete API reference
- **[Template Support Matrix](docs/template-support-matrix.md)** - Supported Go template features

### CLI Documentation
- **[User Guide](docs/user-guide.md)** - CLI getting started
- **[Component Development](docs/component-development.md)** - Creating components
- **[Kit Development](docs/kit-development.md)** - Creating CSS kits
- **[Serve Guide](docs/serve-guide.md)** - Development server

## Testing

```bash
# Run all tests
go test -v ./...

# Run specific test suite
go test -run TestTemplate_E2E -v

# Run client tests
cd client && npm test

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development setup
- Testing guidelines
- Code style conventions
- PR process

## Roadmap

- [ ] Stable v1.0 release
- [ ] Performance benchmarks vs alternatives
- [ ] Deployment guides (Docker, Kubernetes, serverless)
- [ ] Advanced examples (real-time chat, collaborative editing)
- [ ] Streaming updates for large datasets
- [ ] Client-side caching improvements
- [ ] Developer tools (browser extension)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by [Phoenix LiveView](https://hexdocs.pm/phoenix_live_view) - bringing that developer experience to Go.

## Community

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and community discussion
- **Examples**: Check `examples/` directory for working code

---

**Built with LiveTemplate?** We'd love to hear about it! Share your project in GitHub Discussions.
