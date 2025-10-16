# Components Library System - Design Document

**Status:** Planning
**Started:** 2025-10-16
**Target:** 6-week implementation
**Branch:** `feature/components-library`

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture Principles](#architecture-principles)
3. [Component System](#component-system)
4. [Kit System](#kit-system)
5. [Path-Based Discovery](#path-based-discovery)
6. [Scaffolding System](#scaffolding-system)
7. [Validation System](#validation-system)
8. [Unified Development Server (lvt serve)](#unified-development-server)
9. [Implementation Phases](#implementation-phases)
10. [File Structure](#file-structure)
11. [Migration Strategy](#migration-strategy)

---

## Overview

### Goal

Build a comprehensive components library system for lvt CLI that enables:
- **Reusable UI components** independent of CSS frameworks
- **Kits (design systems)** that define look and feel separate from structure
- **Path-based discovery** for automatic component/kit loading
- **Easy contribution workflow** with isolated development
- **Unified development server** for fast iteration

### Core Principles

1. **Separation of Concerns**: Components define structure/behavior, kits define styling
2. **Composition**: Base components combine into complex components
3. **Path-Based**: Auto-discovery from configured paths, no manual registration
4. **Extensibility**: Leverages existing cascading template loader
5. **Backward Compatibility**: Current CSS framework system becomes legacy kits
6. **Developer Experience**: Vite/Next.js-level DX with `lvt serve`

---

## Architecture Principles

### 1. Component Independence

Components must be **CSS framework agnostic**:
- Never hardcode CSS classes
- Use kit helper functions for all styling
- Accept kit as parameter
- Render identically across all kits (structurally)

### 2. Kit Abstraction

Kits provide styling through helper functions:
- Implement standardized interface (~50 helper methods)
- Return appropriate CSS classes for their framework
- Can include custom CSS assets
- Support both CDN and local assets

### 3. Automatic Discovery

No manual add/remove commands:
- Components/kits auto-discovered from configured paths
- Cascading priority: project → user config → system
- Simple config file for path management
- Source tracking (system/local/community)

### 4. Composability

Components can nest and reference each other:
- Dependency resolution via DAG
- Template block composition
- Parent components can override child blocks
- Reusable base components

---

## Component System

### Component Structure

Each component is a directory containing:

```
component-name/
├── component.yaml          # Manifest
├── component-name.tmpl     # Template(s)
├── examples/               # Usage examples
│   ├── basic.yaml
│   └── advanced.yaml
├── test/                   # Tests (optional)
│   └── component_test.go
├── README.md               # Documentation
└── LICENSE                 # License file
```

### Component Manifest Schema

**File:** `component.yaml`

```yaml
name: fancy-card
version: 1.0.0
description: A fancy card component with customizable styling
category: base  # base | form | layout | data | navigation
author: Your Name
license: MIT

# Component inputs (parameters users can pass)
inputs:
  - name: title
    type: string
    required: true
    description: Card title text

  - name: content
    type: string
    required: false
    description: Card content/body text

  - name: variant
    type: string
    enum: [default, highlighted, bordered]
    default: default
    description: Visual variant of the card

# Dependencies on other components (optional)
dependencies: []

# Template files (in order of composition)
templates:
  - fancy-card.tmpl

# Tags for searchability
tags:
  - card
  - container
  - layout
```

### Component Template Pattern

Templates use `[[` `]]` delimiters and kit helpers:

```go
{{define "fancyCard"}}
<div class="[[cardClass .Kit .Variant]]">
  {{if .Title}}
  <div class="[[cardHeaderClass .Kit]]">
    <h3 class="[[cardTitleClass .Kit]]">[[.Title]]</h3>
  </div>
  {{end}}

  {{if .Content}}
  <div class="[[cardBodyClass .Kit]]">
    <p class="[[textClass .Kit]]">[[.Content]]</p>
  </div>
  {{end}}
</div>
{{end}}
```

### System Components

Built-in components (migrated from current templates):

1. **layout** - Base HTML5 layout with kit support
2. **form** - Add/edit forms with validation
3. **table** - Data table with sorting/pagination
4. **pagination** - Multiple pagination modes (infinite/load-more/prev-next/numbers)
5. **toolbar** - Search/sort/add toolbar
6. **detail** - Detail view page

---

## Kit System

### Kit Structure

Each kit is a directory containing:

```
kit-name/
├── kit.yaml                # Manifest
├── helpers.go              # Kit implementation
├── assets/                 # Optional CSS/JS
│   └── kit-name.css
├── examples/               # Preview examples
│   └── preview.html
├── README.md               # Documentation
└── LICENSE                 # License file
```

### Kit Manifest Schema

**File:** `kit.yaml`

```yaml
name: neon
version: 1.0.0
description: A vibrant neon-themed design system with glowing effects
framework: custom  # custom | tailwind | bulma | pico | bootstrap
author: Your Name
license: MIT

# How to load the CSS
cdn_url: https://unpkg.com/@yourorg/neon-kit@1.0.0/dist/neon.css

# Or local assets (relative to this file)
local_assets:
  - assets/neon.css

# Custom CSS variables/theme (optional)
theme:
  colors:
    primary: "#00ff88"
    secondary: "#ff0088"
  fonts:
    body: "Inter, sans-serif"
    heading: "Space Grotesk, sans-serif"

tags:
  - modern
  - colorful
```

### Kit Interface

**File:** `cmd/lvt/internal/kits/interface.go`

```go
type Kit interface {
    Name() string
    Version() string
    GetHelpers() template.FuncMap
}

// Helper functions kits must implement (~50 methods)
type CSSHelpers interface {
    // Layout
    containerClass() string
    sectionClass() string
    boxClass() string

    // Forms
    fieldClass() string
    labelClass() string
    inputClass() string
    selectClass() string
    textareaClass() string
    buttonClass(variant, size string) string

    // Tables
    tableClass() string
    theadClass() string
    thClass() string
    tbodyClass() string
    trClass() string
    tdClass() string

    // Text
    titleClass() string
    subtitleClass() string
    textClass() string

    // Cards
    cardClass(variant string) string
    cardHeaderClass() string
    cardBodyClass() string
    cardFooterClass() string
    cardTitleClass() string

    // Pagination
    paginationClass() string
    paginationButtonClass(active bool) string

    // Utilities
    errorClass() string
    loadingClass() string

    // ... ~30 more methods
}
```

### System Kits

Built-in kits (migrated from current CSS framework logic):

1. **tailwind** - Tailwind CSS v4 utility classes
2. **bulma** - Bulma component framework
3. **pico** - Pico semantic/classless framework
4. **none** - No styling (semantic HTML only)

Future kits:
5. **modern** - Custom CSS with sleek contemporary design
6. **classic** - Bootstrap-inspired traditional design

---

## Path-Based Discovery

### Search Paths

Components and kits are auto-discovered from configured paths:

**Priority order (first match wins):**

1. **Project paths**: `.lvt/components/` or `.lvt/kits/`
2. **User config paths**: From `~/.config/lvt/config.yaml`
3. **System paths**: Embedded in lvt binary

### Configuration File

**File:** `~/.config/lvt/config.yaml`

```yaml
# Local component paths (user's custom components)
component_paths:
  - /Users/you/my-components
  - /Users/you/work/shared-components

# Local kit paths (user's custom kits)
kit_paths:
  - /Users/you/my-kits

# Default kit for new projects
default_kit: tailwind

# Community registry (future)
registry:
  url: https://github.com/livefir/lvt-registry
  enabled: true
```

### Source Tracking

Each component/kit is tagged with its source:

- **system**: Built-in, embedded in lvt binary
- **local**: User's custom components/kits
- **community**: From registry (future)

### Loader Implementation

**File:** `cmd/lvt/internal/components/loader.go`

```go
type ComponentLoader struct {
    searchPaths []string
    cache       map[string]*Component
}

func (l *ComponentLoader) Load(name string) (*Component, error) {
    // Try project paths
    if comp := tryLoadFromProject(name); comp != nil {
        comp.Source = SourceLocal
        return comp, nil
    }

    // Try user config paths
    for _, path := range l.configPaths {
        if comp := tryLoadFromPath(path, name); comp != nil {
            comp.Source = SourceLocal
            return comp, nil
        }
    }

    // Fallback to system
    if comp := loadFromEmbedded(name); comp != nil {
        comp.Source = SourceSystem
        return comp, nil
    }

    return nil, fmt.Errorf("component not found: %s", name)
}
```

---

## Scaffolding System

### Component Scaffolding

**Command:** `lvt components create <name> [--category <cat>]`

Generates complete boilerplate:
- `component.yaml` with pre-filled template
- `.tmpl` file with commented guides
- Example files
- Test scaffold
- README template
- LICENSE file

### Kit Scaffolding

**Command:** `lvt kits create <name> [--framework <framework>]`

Generates complete boilerplate:
- `kit.yaml` with pre-filled template
- `helpers.go` with all methods stubbed
- Starter CSS file
- Preview HTML
- README template
- LICENSE file

### Interactive Mode

**Command:** `lvt components create` (no args)

Launches interactive prompts:
- Component name
- Category selection (dropdown)
- Description
- Author
- License selection
- Options (examples, tests)

---

## Validation System

### Component Validation

**Command:** `lvt components validate <path>`

**Validation checks:**

1. **Structure Validation**
   - `component.yaml` exists and is valid YAML
   - Required fields present
   - Template files exist
   - No unexpected files

2. **Manifest Schema**
   - Name matches directory
   - Version follows semver
   - Category is valid enum
   - Dependencies reference valid components

3. **Template Validation**
   - All `.tmpl` files parse without errors
   - Valid Go template syntax
   - No hardcoded CSS classes
   - Template variables match declared inputs

4. **Examples Validation**
   - `examples/` directory exists
   - At least one example present
   - Examples render without errors

5. **Documentation**
   - `README.md` exists
   - Contains required sections
   - Minimum length check

6. **Testing**
   - Renders with all system kits
   - Output is valid HTML
   - Idempotent rendering

**Output format:**
```
Validating component: fancy-card
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Structure
  ✓ component.yaml found
  ✓ All template files exist

✅ Templates
  ✓ fancy-card.tmpl parses correctly
  ✓ No hardcoded CSS classes

✅ Testing
  ✓ Renders with tailwind kit
  ✓ Renders with bulma kit

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Component is valid and ready for contribution!
```

### Kit Validation

**Command:** `lvt kits validate <path>`

**Validation checks:**

1. **Structure**: kit.yaml, helpers.go exist
2. **Manifest**: Valid YAML, required fields
3. **Helpers**: All ~50 methods implemented, compiles without errors
4. **Assets**: CSS valid, reasonable file sizes
5. **Compatibility**: Renders all system components
6. **Documentation**: README exists with required sections

---

## Unified Development Server

### `lvt serve` Command

**One command for all development scenarios:**

```bash
lvt serve [path] [--port 3000] [--mode auto|component|kit|app]
```

### Auto-Detection

Automatically detects what to serve based on directory structure:

- **component.yaml** present → Component development mode
- **kit.yaml** present → Kit development mode
- **go.mod + cmd/** present → App development mode
- **.lvt/** directory present → App development mode

### Component Development Mode

**When serving a component:**

**Features:**
- Live preview of component
- Kit switcher in UI
- Example selector
- Auto-reload on template/example changes
- Side-by-side code view
- Validation status

**UI Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  Lvt Serve - fancy-card                        [●] Connected │
├─────────────────────────────────────────────────────────────┤
│  Kit: [Tailwind ▼]  Example: [Basic ▼]  [⟳ Reload]          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  [Component Preview - Rendered Output]                       │
│                                                               │
│  [Data from Example]                                          │
│  [Validation Status]                                          │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**File watching:**
- `component.yaml`
- `*.tmpl`
- `examples/*.yaml`

**On change:**
1. Recompile template
2. Revalidate component
3. Send WebSocket message to browser
4. Browser hot-reloads preview

### Kit Development Mode

**When serving a kit:**

**Features:**
- Preview kit with all system components
- Component selector
- Live CSS editing with hot reload
- Helper function tester
- Validation status

**File watching:**
- `kit.yaml`
- `helpers.go`
- `assets/*.css`
- `assets/*.js`

**On CSS change:** Inject new CSS without full reload
**On helpers.go change:** Recompile + full reload

### App Development Mode

**When serving an app:**

**Architecture:**
```
Browser ◄─WS─► lvt serve ◄─HTTP─► Go App
  :3000         (proxy)              :8080
                   │
                   ▼
               [Watcher]
             *.go, *.tmpl
```

**Features:**
- Run Go app server
- Proxy to app server
- Watch Go files + templates + assets
- Auto-restart on changes
- Unified browser console + server logs

**File watching:**
- `cmd/**/*.go`
- `internal/**/*.go`
- `internal/**/*.tmpl`
- `web/assets/**`

**On change:**
1. Stop app server
2. Rebuild Go app
3. Start app server
4. Send reload signal to browser

### WebSocket Protocol

```javascript
// Message types
{
  "type": "reload",          // Full reload
  "data": { "html": "...", "validation": {...} }
}

{
  "type": "css-update",      // Hot CSS injection
  "data": { "css": "..." }
}

{
  "type": "error",           // Show error overlay
  "data": { "message": "...", "file": "...", "line": 12 }
}

{
  "type": "log",             // Server log
  "data": { "level": "info", "message": "..." }
}
```

### Browser Integration

Per CLAUDE.md requirements, `lvt serve` provides access to:
- Browser console logs
- Server logs
- WebSocket messages
- Rendered HTML

This enables deep debugging during development.

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)

**1.1 Component System Core**
- Create `cmd/lvt/internal/components/` package
- Implement types, manifest parser, loader
- Add embedding for system components

**1.2 Kit System Core**
- Create `cmd/lvt/internal/kits/` package
- Implement interface, loader
- Add embedding for system kits

**1.3 Config System**
- Create `cmd/lvt/internal/config/` package
- YAML config file support
- Path management

### Phase 2: Migration (Week 2-3)

**2.1 Extract System Components**
- Migrate templates/components/*.tmpl → component format
- Create component.yaml for each
- Test component loading

**2.2 Extract System Kits**
- Extract css_helpers.go → kit implementations
- Create kit.yaml for each framework
- Implement Kit interface for tailwind, bulma, pico, none

**2.3 Embed Resources**
- Use go:embed for system components/kits
- Ensure all resources accessible

### Phase 3: Integration (Week 3-4)

**3.1 Wire Up Generators**
- Update resource.go to use component loader
- Update types.go to use Kit instead of CSSFramework string
- Add --kit flag to commands

**3.2 Backward Compatibility**
- Map --css flag to kit names
- Ensure old commands still work
- Test all existing examples

**3.3 Testing**
- Run all existing tests (must pass)
- Test recreate_myblog.sh
- Verify golden files match

### Phase 4: Scaffolding & Validation (Week 4-5)

**4.1 Component Scaffolding**
- Implement `lvt components create`
- Generate boilerplate files
- Interactive mode

**4.2 Kit Scaffolding**
- Implement `lvt kits create`
- Generate boilerplate files
- Interactive mode

**4.3 Validation**
- Implement `lvt components validate`
- Implement `lvt kits validate`
- Validation rules registry
- HTML/CSS/template validators

### Phase 5: Development Server (Week 5-6)

**5.1 Serve Command**
- Implement `lvt serve` command
- Auto-detection logic
- File watcher

**5.2 Component/Kit Modes**
- Component dev UI
- Kit dev UI
- WebSocket server
- Hot reload logic

**5.3 App Mode**
- Proxy server
- Go app runner
- Log aggregation
- Browser integration

### Phase 6: Documentation & Polish (Week 6)

**6.1 Documentation**
- User guide
- Component development guide
- Kit development guide
- API reference

**6.2 Polish**
- Error messages
- Help text
- Examples
- Edge cases

---

## File Structure

### New Files

```
cmd/lvt/
├── internal/
│   ├── components/
│   │   ├── types.go              # Component data structures
│   │   ├── manifest.go           # YAML parser
│   │   ├── loader.go             # Component loader
│   │   ├── validator.go          # Component validation
│   │   ├── embed.go              # System components embedding
│   │   └── system/               # System components
│   │       ├── layout/
│   │       ├── form/
│   │       ├── table/
│   │       ├── pagination/
│   │       ├── toolbar/
│   │       └── detail/
│   ├── kits/
│   │   ├── interface.go          # Kit interface
│   │   ├── types.go              # Kit data structures
│   │   ├── loader.go             # Kit loader
│   │   ├── validator.go          # Kit validation
│   │   ├── embed.go              # System kits embedding
│   │   └── system/               # System kits
│   │       ├── tailwind/
│   │       ├── bulma/
│   │       ├── pico/
│   │       └── none/
│   ├── config/
│   │   └── config.go             # Config file management
│   ├── validator/
│   │   ├── component.go          # Component validation logic
│   │   ├── kit.go                # Kit validation logic
│   │   ├── template.go           # Template syntax validation
│   │   ├── html.go               # HTML output validation
│   │   └── rules.go              # Validation rules
│   └── serve/
│       ├── server.go             # Main serve command
│       ├── detector.go           # Mode auto-detection
│       ├── component_mode.go     # Component dev server
│       ├── kit_mode.go           # Kit dev server
│       ├── app_mode.go           # App dev server
│       ├── watcher.go            # File watcher
│       ├── proxy.go              # Proxy to app
│       ├── websocket.go          # WebSocket for hot reload
│       └── ui/                   # Dev UI assets
├── commands/
│   ├── components.go             # lvt components command
│   ├── kits.go                   # lvt kits command
│   ├── config.go                 # lvt config command
│   └── serve.go                  # lvt serve command
```

### Modified Files

```
cmd/lvt/
├── internal/generator/
│   ├── resource.go               # Use component loader
│   ├── types.go                  # Add Kit field
│   └── css_helpers.go            # Extract to kits
├── commands/
│   ├── new.go                    # Add --kit flag
│   └── gen.go                    # Add --kit flag
└── main.go                       # Add new commands
```

### User Config Structure

```
~/.config/lvt/
├── config.yaml                   # Main config file
├── components/
│   ├── custom/                   # User's components
│   └── system/                   # Installed system components
└── kits/
    ├── custom/                   # User's kits
    └── system/                   # Installed system kits
```

### Project Structure

```
.lvt/
├── components/                   # Project-specific components
├── kits/                         # Project-specific kits
└── templates/                    # Existing template overrides
```

---

## Migration Strategy

### Backward Compatibility

**Existing commands must work unchanged:**

```bash
# Old syntax (still works)
lvt gen users name email --css tailwind

# New syntax (equivalent)
lvt gen users name email --kit tailwind
```

**Mapping:**
- `--css tailwind` → loads tailwind kit
- `--css bulma` → loads bulma kit
- `--css pico` → loads pico kit
- `--css none` → loads none kit

### Migration Steps

1. **Phase 1-2**: Build new system alongside existing code
2. **Phase 3**: Wire up, keep old code paths as fallback
3. **Phase 4**: Test everything thoroughly
4. **Phase 5**: Deprecate old paths (warnings only)
5. **Future**: Remove deprecated code (major version bump)

### Testing Strategy

**Must pass:**
- All existing unit tests
- All existing integration tests
- All existing E2E tests (chromedp)
- Golden file tests
- `scripts/recreate_myblog.sh` works identically

**Per CLAUDE.md:**
- Always run all tests before finishing session
- Never skip pre-commit hook
- Use chromedp for UI verification

---

## Success Criteria

- ✅ All existing tests pass without modification
- ✅ All examples work identically
- ✅ Golden files match
- ✅ Old --css flag still works
- ✅ New --kit flag works
- ✅ Custom components auto-discovered from paths
- ✅ Can develop components outside lvt project
- ✅ `lvt serve` provides hot reload
- ✅ Simple config commands for path management
- ✅ Validation catches common errors
- ✅ Scaffolding creates working boilerplate

---

## Next Steps

See `COMPONENTS_TODO.md` for detailed task breakdown and progress tracking.

**To resume work:**
1. Read this design doc for architecture context
2. Check `COMPONENTS_TODO.md` for current progress
3. Pick next uncompleted task
4. Update checkboxes as you work
5. Commit with clear messages

**Branch:** `feature/components-library`

---

## References

- Current template system: `cmd/lvt/internal/generator/template_loader.go`
- Current CSS helpers: `cmd/lvt/internal/generator/css_helpers.go`
- Current components: `cmd/lvt/internal/generator/templates/components/`
- Resource generator: `cmd/lvt/internal/generator/resource.go`
- CLAUDE.md: Project development guidelines
