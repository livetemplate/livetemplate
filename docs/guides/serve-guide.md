# LiveTemplate Serve Guide

The `lvt serve` command provides a unified development server with hot reload for three different development scenarios: components, kits, and full Go applications.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Component Development Mode](#component-development-mode)
- [Kit Development Mode](#kit-development-mode)
- [App Development Mode](#app-development-mode)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [Advanced Features](#advanced-features)
- [Troubleshooting](#troubleshooting)

---

## Overview

`lvt serve` automatically detects what you're working on and provides the appropriate development environment:

| Mode | Detection | Purpose |
|------|-----------|---------|
| **Component** | `component.yaml` or `*.tmpl` files | Live preview component templates |
| **Kit** | `kit.yaml` file | Showcase CSS helper methods |
| **App** | `go.mod` or `main.go` | Run full Go app with hot reload |

### Key Features

- **Auto-detection**: Automatically determines the correct mode
- **Hot Reload**: WebSocket-based browser reload on file changes
- **Live Preview**: Real-time template rendering
- **Error Display**: Friendly error messages in browser
- **Port Management**: Automatic port availability checking

---

## Quick Start

### Automatic Mode Detection

```bash
# Navigate to your project directory
cd myproject

# Start the server (auto-detects mode)
lvt serve
```

The server will:
1. Detect the mode based on directory contents
2. Start on port 3000 (or use `--port` to specify)
3. Open your browser automatically
4. Watch for file changes and reload

### Manual Mode Selection

```bash
# Force specific mode
lvt serve --mode component
lvt serve --mode kit
lvt serve --mode app
```

### Common Options

```bash
# Custom port
lvt serve --port 8080

# Custom host
lvt serve --host 0.0.0.0

# Don't open browser
lvt serve --no-browser

# Disable hot reload
lvt serve --no-reload

# Specify directory
lvt serve --dir /path/to/project
```

---

## Component Development Mode

Component mode provides a live preview environment for developing component templates.

### When It's Used

Component mode activates when the current directory contains:
- `component.yaml` file, OR
- `*.tmpl` template files

### Features

1. **Split-Pane UI**: Editor on left, preview on right
2. **JSON Test Data Editor**: Edit component inputs in real-time
3. **Live Preview**: Renders template with test data
4. **Kit Selection**: Test with different CSS frameworks
5. **Hot Reload**: Auto-reload on template changes
6. **Error Display**: Template errors shown in preview pane

### Workflow Example

```bash
# Create a new component
lvt components create card --category data
cd ~/.lvt/components/card

# Start development server
lvt serve
```

**In the browser:**

1. Edit JSON data in left pane:
```json
{
  "Title": "Product Card",
  "Description": "A beautiful product",
  "Price": "$99.99",
  "Image": "/images/product.jpg"
}
```

2. Edit template file (changes reload automatically):
```html
[[define "card"]]
<div class="[[cardClass]]">
  [[if .Image]]
  <img src="[[.Image]]" alt="[[.Title]]">
  [[end]]
  <div class="[[cardBodyClass]]">
    <h3>[[.Title]]</h3>
    <p>[[.Description]]</p>
    <span class="price">[[.Price]]</span>
  </div>
</div>
[[end]]
```

3. See live preview update instantly

### Kit Integration

Components automatically load the kit specified in `component.yaml`:

```yaml
# component.yaml
kit: tailwind  # Use Tailwind CSS for preview
```

Or test with different kits using the kit selector in the UI.

### Files Watched

Component mode watches:
- `component.yaml` - Component manifest
- `*.tmpl` - All template files
- `kit.yaml` - If kit is local

### URLs

| URL | Description |
|-----|-------------|
| `http://localhost:3000/` | Main component preview UI |
| `http://localhost:3000/preview` | Preview pane only |
| `http://localhost:3000/render` | POST endpoint to render with data |
| `http://localhost:3000/reload` | Trigger manual reload |
| `http://localhost:3000/ws` | WebSocket for hot reload |

---

## Kit Development Mode

Kit mode showcases CSS helper methods and provides live examples of all CSS classes.

### When It's Used

Kit mode activates when the current directory contains:
- `kit.yaml` file

### Features

1. **Kit Information Display**: Name, version, framework details
2. **Helper Methods List**: All ~60 helper methods with examples
3. **Live CSS Examples**: See actual rendered output of each class
4. **Component Previews**: Common components styled with your kit
5. **Hot Reload**: Auto-reload on helper code changes

### Workflow Example

```bash
# Create a new kit
lvt kits create mykit
cd ~/.lvt/kits/mykit

# Edit helpers.go - implement your CSS classes
# Edit kit.yaml - update metadata

# Start development server
lvt serve
```

**In the browser:**

See live showcase of your kit:
- Container examples
- Button variants (primary, secondary, danger, etc.)
- Form elements (inputs, labels, selects)
- Table styles
- Card styles
- Typography examples

### Files Watched

Kit mode watches:
- `kit.yaml` - Kit manifest
- `helpers.go` - Helper implementation
- `*.go` - Any Go source files

When files change:
- Go code is recompiled
- Browser reloads
- Errors shown in browser if compilation fails

### URLs

| URL | Description |
|-----|-------------|
| `http://localhost:3000/` | Kit showcase UI |
| `http://localhost:3000/test` | Test specific helper method |
| `http://localhost:3000/helpers` | List all helper methods (JSON) |
| `http://localhost:3000/ws` | WebSocket for hot reload |

### Helper Testing

Test individual helper methods:

```bash
# In browser console
fetch('/test?method=ButtonClass&args=primary')
  .then(r => r.text())
  .then(console.log)
// Output: "btn btn-primary"
```

---

## App Development Mode

App mode runs your full Go application with automatic rebuild and restart on file changes.

### When It's Used

App mode activates when the current directory contains:
- `go.mod` file, OR
- `main.go` file

### Features

1. **Automatic Build**: Compiles your Go app on start and file changes
2. **Process Management**: Manages app lifecycle (start, stop, restart)
3. **Hot Reload**: Rebuilds and restarts on .go, .tmpl, .sql file changes
4. **Reverse Proxy**: Proxies requests from port 3000 to your app on port 8080
5. **Error Handling**: Shows build errors in browser
6. **Graceful Shutdown**: Clean shutdown on Ctrl+C

### Workflow Example

```bash
# Create a new app
lvt new myapp --css tailwind
cd myapp

# Generate a resource
lvt gen products name price:float stock:int

# Start development server
lvt serve
```

**What happens:**

1. Server detects app mode (sees `go.mod`)
2. Builds your Go application
3. Starts your app on port 8080
4. Proxies port 3000 → port 8080
5. Opens browser at `http://localhost:3000`
6. Watches for file changes

**When you edit code:**

1. Detects file change
2. Stops running app
3. Rebuilds app
4. Starts new app
5. Browser auto-reloads

### Port Configuration

Your app must listen on the port specified by the `PORT` environment variable:

```go
// main.go
func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    http.ListenAndServe(":"+port, nil)
}
```

The development server sets `PORT=8080` automatically.

### Files Watched

App mode watches:
- `**/*.go` - All Go source files
- `**/*.tmpl` - All template files
- `**/*.sql` - SQL migration files

Ignored patterns:
- `.git/`
- `node_modules/`
- `*.swp`, `*.tmp`
- `_test.go` files (rebuild triggered manually with tests)

### Build Process

On each change:

```bash
# Stop running app (if any)
kill <pid>

# Build app
go build -o /tmp/lvt-app-<random> .

# Start app
PORT=8080 /tmp/lvt-app-<random>
```

### URLs

| URL | Description |
|-----|-------------|
| `http://localhost:3000/*` | Proxied to your app on port 8080 |
| `http://localhost:3000/ws` | WebSocket for hot reload |

### Error Handling

If build fails:
- Browser shows "Building..." message
- Build errors logged to console
- Browser auto-refreshes when build succeeds

If app crashes:
- Browser shows "Starting..." message
- App automatically restarted
- Errors logged to console

---

## Command Reference

### Basic Usage

```bash
lvt serve [options]
```

### Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--port` | `-p` | Server port | 3000 |
| `--host` | `-h` | Server host | localhost |
| `--dir` | `-d` | Project directory | . (current) |
| `--mode` | `-m` | Force mode (component\|kit\|app) | auto-detect |
| `--no-browser` | | Don't open browser automatically | false |
| `--no-reload` | | Disable hot reload | false |

### Examples

```bash
# Basic usage (auto-detect mode)
lvt serve

# Custom port
lvt serve --port 8080

# Custom host (allow external connections)
lvt serve --host 0.0.0.0 --port 8080

# Force component mode
lvt serve --mode component

# Don't open browser, don't reload
lvt serve --no-browser --no-reload

# Serve from different directory
lvt serve --dir /path/to/project
```

---

## Configuration

### Global Configuration

Server settings can be configured in `~/.config/lvt/config.yaml`:

```yaml
# Server defaults
serve:
  port: 3000
  host: localhost
  open_browser: true
  live_reload: true
  debounce: 100ms
```

(Note: Server configuration is not yet implemented - this is for future reference)

### Per-Project Configuration

Component mode reads from `component.yaml`:

```yaml
name: mycomponent
kit: tailwind  # Default kit for preview

# Development settings
dev:
  test_data:
    Title: "Default Title"
    Content: "Default content"
```

Kit mode reads from `kit.yaml`:

```yaml
name: mykit
framework:
  name: MyFramework
  version: 1.0.0
```

---

## Advanced Features

### WebSocket Protocol

The hot reload uses WebSocket messages:

```javascript
// Client code (automatically injected)
const ws = new WebSocket('ws://localhost:3000/ws');

ws.onmessage = (event) => {
    const data = JSON.parse(event.data);

    if (data.type === 'reload') {
        console.log('Reloading:', data.path);
        window.location.reload();
    }
};
```

### Custom WebSocket Path

```bash
# Default: /ws
# Custom: /live-reload
export WEBSOCKET_PATH=/live-reload
lvt serve
```

(Note: Custom WebSocket path is not yet implemented)

### Debouncing

File changes are debounced to prevent multiple rapid reloads:

- Default debounce: 100ms
- Configurable via server code (not yet exposed to config)

When you save a file multiple times rapidly, only one reload occurs after 100ms of inactivity.

### Ignore Patterns

Some files/directories are automatically ignored:

```
.git/
node_modules/
*.swp
*.tmp
*~
.DS_Store
```

Additional patterns can be added via watcher:

```go
// In code (not yet exposed to config)
watcher.AddIgnorePattern("*.log")
watcher.AddIgnorePattern("dist/")
```

### Multiple Instances

Run multiple serve instances on different ports:

```bash
# Terminal 1: Component development
cd ~/.lvt/components/card
lvt serve --port 3000

# Terminal 2: Kit development
cd ~/.lvt/kits/mykit
lvt serve --port 3001

# Terminal 3: App development
cd ~/myapp
lvt serve --port 3002
```

### Proxy Configuration

App mode proxies requests from dev server to your app:

```
Browser (port 3000)
    ↓
lvt serve (reverse proxy)
    ↓
Your app (port 8080)
```

This allows:
- Hot reload injection
- Error page display
- Build status messages

---

## Troubleshooting

### Port Already in Use

```bash
# Error: port 3000 is already in use

# Solution 1: Use different port
lvt serve --port 3001

# Solution 2: Kill process using port
lsof -ti:3000 | xargs kill
```

### Mode Not Detected Correctly

```bash
# Server detected wrong mode

# Solution: Force correct mode
lvt serve --mode component
lvt serve --mode kit
lvt serve --mode app
```

### WebSocket Connection Failed

```bash
# Check browser console:
# WebSocket connection to 'ws://localhost:3000/ws' failed

# Possible causes:
# 1. Server not running
# 2. Firewall blocking WebSocket
# 3. Reverse proxy stripping WebSocket headers

# Solution: Restart server
lvt serve --port 3000
```

### Hot Reload Not Working

```bash
# Browser not reloading on file changes

# Check:
# 1. Is --no-reload flag set?
lvt serve  # Remove --no-reload

# 2. Are you editing watched files?
# Component mode: *.tmpl, component.yaml
# Kit mode: *.go, kit.yaml
# App mode: *.go, *.tmpl, *.sql

# 3. Check browser console for WebSocket errors
```

### Template Errors in Component Mode

```bash
# Error shown in preview pane:
# template: component.tmpl:5: unexpected "}"

# Solution:
# 1. Check line 5 of template
# 2. Verify [[ ]] delimiters (not {{ }})
# 3. Validate template:
lvt components validate .
```

### Build Errors in App Mode

```bash
# Browser shows "Building..." but never completes

# Check terminal for build errors:
# # command-line-arguments
# ./main.go:10:2: undefined: fmt

# Solution: Fix build errors, save file, server rebuilds
```

### App Crashes on Startup

```bash
# Browser shows "Starting..." repeatedly

# Check terminal for runtime errors:
# panic: database connection failed

# Solution:
# 1. Fix runtime errors
# 2. Check environment variables
# 3. Verify database is running
# 4. Check port 8080 is available
```

### Browser Not Opening

```bash
# Server starts but browser doesn't open

# Solutions:
# 1. Open manually: http://localhost:3000
# 2. Check if --no-browser flag is set
# 3. Check default browser is configured
```

### Slow Rebuild in App Mode

```bash
# App takes long time to rebuild

# Optimization:
# 1. Use Go modules (go.mod)
# 2. Enable Go build cache
# 3. Reduce number of dependencies
# 4. Use go build -i for faster incremental builds
```

---

## Performance Tips

### Component Mode

- Keep test data small (large JSON slows preview)
- Use simple template structures for faster rendering
- Disable hot reload if editing documentation only

### Kit Mode

- Helpers.go should compile quickly (no heavy dependencies)
- Keep helper methods simple (no complex logic)
- Use inline classes instead of computed classes when possible

### App Mode

- Use Go build cache (automatic with Go 1.10+)
- Minimize dependencies in frequently-edited files
- Use `//go:embed` for static assets (faster than reading files)
- Keep main.go small (move logic to packages)

---

## Next Steps

- Create components: [Component Development Guide](component-development.md)
- Create kits: [Kit Development Guide](kit-development.md)
- Explore API: [API Reference](api-reference.md)

---

Last updated: 2025-10-17
