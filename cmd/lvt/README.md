# LiveTemplate CLI Generator (`lvt`)

A Phoenix-inspired code generator for LiveTemplate applications with CRUD functionality and interactive TUI wizards.

## Installation

```bash
go install github.com/livefir/livetemplate/cmd/lvt@latest
```

Or build from source:

```bash
git clone https://github.com/livefir/livetemplate
cd livetemplate
go build -o lvt ./cmd/lvt
```

## Quick Start

You can use `lvt` in two modes: **Interactive** (TUI wizards) or **Direct** (CLI arguments).

### Interactive Mode (Recommended for New Users)

```bash
# Launch interactive app creator
lvt new

# Launch interactive resource builder
lvt gen

# Launch interactive view creator
lvt gen view
```

### Direct Mode

### 1. Create a New App

```bash
lvt new myapp
cd myapp
```

This generates:
- Complete Go project structure
- Database layer with sqlc integration
- go.mod with Go 1.24+ tools directive
- README with next steps

### 2. Generate a CRUD Resource

```bash
# With explicit types
lvt gen users name:string email:string age:int

# With inferred types (NEW!)
lvt gen products name price quantity enabled created_at
# → Infers: name:string price:float quantity:int enabled:bool created_at:time
```

This generates:
- `internal/app/users/users.go` - Full CRUD handler
- `internal/app/users/users.tmpl` - Bulma CSS UI
- `internal/app/users/users_ws_test.go` - WebSocket tests
- `internal/app/users/users_test.go` - Chromedp E2E tests
- Database schema and queries (appended)

### 3. Generate Database Code

```bash
cd internal/database
go run github.com/sqlc-dev/sqlc/cmd/sqlc generate
cd ../..
```

### 4. Wire Up Routes

Add to `cmd/myapp/main.go`:

```go
import "myapp/internal/app/users"

// In main():
http.Handle("/users", users.Handler(queries))
```

### 5. Run the App

```bash
go run cmd/myapp/main.go
```

Open http://localhost:8080/users

## Commands

### `lvt new <app-name>`

Creates a new LiveTemplate application with:

```
myapp/
├── cmd/myapp/main.go           # Application entry point
├── go.mod                      # With //go:tool directive
├── internal/
│   ├── app/                    # Handlers and templates
│   ├── database/
│   │   ├── db.go              # Connection & migrations
│   │   ├── schema.sql         # Database schema
│   │   ├── queries.sql        # SQL queries (sqlc)
│   │   ├── sqlc.yaml          # sqlc configuration
│   │   └── models/            # Generated code
│   └── shared/                # Shared utilities
├── web/assets/                # Static assets
└── README.md
```

### `lvt gen <resource> <field:type>...`

Generates a full CRUD resource with database integration.

**Example:**
```bash
lvt gen posts title:string content:string published:bool views:int
```

**Generated Files:**
- Handler with State struct, Change() method, Init() method
- Bulma CSS template with:
  - Create form with validation
  - List view with search, sort, pagination
  - Delete functionality
  - Real-time WebSocket updates
- WebSocket unit tests
- Chromedp E2E tests
- Database schema and queries

**Features:**
- ✅ CRUD operations (Create, Read, Update, Delete)
- ✅ Search across string fields
- ✅ Sorting by fields
- ✅ Pagination
- ✅ Real-time updates via WebSocket
- ✅ Form validation
- ✅ Statistics/counts
- ✅ Bulma CSS styling
- ✅ Comprehensive tests
- ✅ **Auto-injected routes** - Automatically adds route and import to `main.go`

### `lvt gen view <name>`

Generates a view-only handler without database integration (like the counter example).

**Example:**
```bash
lvt gen view dashboard
```

**Generates:**
- `internal/app/dashboard/dashboard.go` - View handler with state management
- `internal/app/dashboard/dashboard.tmpl` - Bulma CSS template
- `internal/app/dashboard/dashboard_ws_test.go` - WebSocket tests
- `internal/app/dashboard/dashboard_test.go` - Chromedp E2E tests

**Features:**
- ✅ State management
- ✅ Real-time updates via WebSocket
- ✅ Bulma CSS styling
- ✅ Comprehensive tests
- ✅ No database dependencies
- ✅ **Auto-injected routes** - Automatically adds route and import to `main.go`

## Router Auto-Update

When you generate a resource or view, `lvt` automatically:

1. **Adds the import** to your `cmd/*/main.go`:
   ```go
   import (
       "yourapp/internal/app/users"  // ← Auto-added
   )
   ```

2. **Injects the route** after the TODO comment:
   ```go
   // TODO: Add routes here
   http.Handle("/users", users.Handler(queries))  // ← Auto-added
   ```

3. **Maintains idempotency** - Running the same command twice won't duplicate routes

This eliminates the manual step of wiring up routes, making the development workflow smoother. Routes are inserted in the order you generate them, right after the TODO marker.

## Type Mappings

| CLI Type | Go Type      | SQL Type   |
|----------|--------------|------------|
| string   | string       | TEXT       |
| int      | int64        | INTEGER    |
| bool     | bool         | BOOLEAN    |
| float    | float64      | REAL       |
| time     | time.Time    | DATETIME   |

**Aliases:**
- `str`, `text` → `string`
- `integer` → `int`
- `boolean` → `bool`
- `float64`, `decimal` → `float`
- `datetime`, `timestamp` → `time`

## Smart Type Inference (🆕 Phase 1)

The CLI includes an intelligent type inference system that automatically suggests types based on field names:

### How It Works

When using the type inference system, you can omit explicit types and let the system infer them:

```go
// In ui.InferType("email") → returns "string"
// In ui.InferType("age") → returns "int"
// In ui.InferType("price") → returns "float"
// In ui.InferType("enabled") → returns "bool"
// In ui.InferType("created_at") → returns "time"
```

### Inference Rules

**String fields** (default for unknown):
- Exact: `name`, `title`, `description`, `email`, `username`, `url`, `slug`, `address`, etc.
- Contains: `*email*`, `*url*`

**Integer fields:**
- Exact: `age`, `count`, `quantity`, `views`, `likes`, `score`, `rank`, `year`
- Suffix: `*_count`, `*_number`, `*_index`

**Float fields:**
- Exact: `price`, `amount`, `rating`, `latitude`, `longitude`
- Suffix/Contains: `*_price`, `*_amount`, `*_rate`, `*price*`, `*amount*`

**Boolean fields:**
- Exact: `enabled`, `active`, `published`, `verified`, `approved`, `deleted`
- Prefix: `is_*`, `has_*`, `can_*`

**Time fields:**
- Exact: `created_at`, `updated_at`, `deleted_at`, `published_at`
- Suffix: `*_at`, `*_date`, `*_time`

### Usage

The inference system is available via the `ui` package:

```go
import "github.com/livefir/livetemplate/cmd/lvt/internal/ui"

// Infer type from field name
fieldType := ui.InferType("email")  // → "string"

// Parse field input (with or without type)
name, typ := ui.ParseFieldInput("email")      // → "email", "string" (inferred)
name, typ := ui.ParseFieldInput("age:float")  // → "age", "float" (explicit override)
```

### Future Enhancement

In upcoming phases, this will power:
- Interactive field builders that suggest types as you type
- Direct mode support: `lvt gen users name email age` (without explicit types)
- Smart defaults that reduce typing

## Project Layout

The generated app follows idiomatic Go conventions:

- **`cmd/`** - Application entry points
- **`internal/app/`** - Handlers and templates (co-located!)
- **`internal/database/`** - Database layer with sqlc
- **`internal/shared/`** - Shared utilities
- **`web/assets/`** - Static assets

**Key Design Decision:** Templates live next to their handlers for easy discovery.

## Generated Handler Structure

```go
package users

type State struct {
    Queries        *models.Queries
    Users          []User
    SearchQuery    string
    SortBy         string
    CurrentPage    int
    PageSize       int
    TotalPages     int
    // ...
}

func (s *State) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "add":
        // Create user
    case "update":
        // Update user
    case "delete":
        // Delete user
    case "search":
        // Search users
    // ...
    }
}

func (s *State) Init() error {
    // Load initial data
}

func Handler(queries *models.Queries) http.Handler {
    tmpl := livetemplate.New("users")
    state := &State{Queries: queries, PageSize: 10}
    return tmpl.Handle(state)
}
```

## Testing

Each generated resource includes comprehensive tests:

### WebSocket Tests (`*_ws_test.go`)

Fast unit tests for WebSocket protocol and state changes:

```bash
go test ./internal/app/users -run WebSocket
```

Features:
- Test server startup with dynamic ports
- WebSocket connection testing
- CRUD action testing
- Server log capture for debugging
- Fast execution (~2-5 seconds)

### E2E Tests (`*_test.go`)

Full browser testing with real user interactions:

```bash
go test ./internal/app/users -run E2E
```

Features:
- Docker Chrome container
- Real browser interactions (clicks, typing, forms)
- Visual verification
- Screenshot capture
- Console log access
- Comprehensive (~20-60 seconds)

**Skip slow tests:**
```bash
go test -short ./...
```

## Go 1.24+ Tools Support

Generated `go.mod` includes:

```go
//go:tool github.com/sqlc-dev/sqlc/cmd/sqlc
```

Run sqlc via:
```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc generate
```

## CSS Framework

All generated templates use [Bulma CSS](https://bulma.io/) by default:

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
```

Components used:
- `.section`, `.container` - Layout
- `.box` - Content containers
- `.table` - Data tables
- `.button`, `.input`, `.select` - Form controls
- `.pagination` - Pagination controls

## Development Workflow

1. **Create app:** `lvt new myapp`
2. **Generate resources:** `lvt gen users name:string email:string`
3. **Generate DB code:** `cd internal/database && go run github.com/sqlc-dev/sqlc/cmd/sqlc generate`
4. **Wire routes** in `main.go`
5. **Run tests:** `go test ./...`
6. **Run app:** `go run cmd/myapp/main.go`

## Examples

### Blog App

```bash
lvt new myblog
cd myblog

# Generate posts resource
lvt gen posts title:string content:string published:bool

# Generate comments resource
lvt gen comments post_id:string author:string text:string

# Generate DB code
cd internal/database
go run github.com/sqlc-dev/sqlc/cmd/sqlc generate
cd ../..

# Run
go run cmd/myblog/main.go
```

### E-commerce

```bash
lvt new mystore
cd mystore

lvt gen products name:string price:float stock:int
lvt gen customers name:string email:string
lvt gen orders customer_id:string total:float

cd internal/database && go run github.com/sqlc-dev/sqlc/cmd/sqlc generate && cd ../..
go run cmd/mystore/main.go
```

## Architecture

### Template System

The generator uses custom delimiters (`[[`, `]]`) to avoid conflicts with Go template syntax:

- **Generator templates:** `[[.ResourceName]]` - Replaced during generation
- **Output templates:** `{{.Title}}` - Used at runtime by LiveTemplate

### Embedded Templates

All templates are embedded using `embed.FS` for easy distribution.

### Code Generation Strategy

1. Parse field definitions (`name:type`)
2. Map types to Go and SQL types
3. Render templates with resource data
4. Generate handler, template, tests
5. Append to database files

## Testing the Generator

### Run All Tests

```bash
go test ./cmd/lvt -v
```

### Test Layers

1. **Parser Tests** (`cmd/lvt/internal/parser/fields_test.go`)
   - Field parsing and validation
   - Type mapping correctness
   - 13 comprehensive tests

2. **Golden File Tests** (`cmd/lvt/golden_test.go`)
   - Regression testing for generated code
   - Validates handler and template output
   - Update with: `UPDATE_GOLDEN=1 go test ./cmd/lvt -run Golden`

3. **Integration Tests** (`cmd/lvt/integration_test.go`)
   - Go syntax validation
   - File structure validation
   - Generation pipeline testing

4. **Smoke Test** (`scripts/test_cli_smoke.sh`)
   - End-to-end CLI workflow
   - App creation and resource generation
   - File structure verification

## Roadmap

- [x] ~~`lvt gen view` - View-only handlers~~ ✅ Complete
- [x] ~~Router auto-update~~ ✅ Complete
- [x] ~~Bubbletea interactive UI~~ ✅ Complete (Phase 1-3)
  - [x] Dependencies & infrastructure
  - [x] Smart type inference system (50+ patterns)
  - [x] UI styling framework (Lipgloss)
  - [x] Interactive app creation wizard
  - [x] Interactive resource builder
  - [x] Interactive view builder
  - [x] Mode detection (auto-switch based on args)
  - [x] Type inference in direct mode
  - [x] ~~Enhanced validation & help system (Phase 4)~~ ✅ Complete
    - [x] Real-time Go identifier validation
    - [x] SQL reserved word warnings (25+ keywords)
    - [x] Help overlay with `?` key in all wizards
    - [x] Color-coded feedback (✓✗⚠)
    - [x] All 3 wizards enhanced
- [x] ~~Migration commands~~ ✅ Complete
  - [x] Goose integration with minimal wrapper (~410 lines)
  - [x] Auto-generate migrations from `lvt gen resource`
  - [x] Commands: `up`, `down`, `status`, `create <name>`
  - [x] Timestamped migration files with Up/Down sections
  - [x] Schema versioning and rollback support
- [x] ~~Custom template support~~ ✅ Complete
  - [x] Cascading template lookup (project → user → embedded)
  - [x] `lvt template copy` command for easy customization
  - [x] Project templates in `.lvt/templates/` (version-controlled)
  - [x] User-wide templates in `~/.config/lvt/templates/`
  - [x] Selective override (only customize what you need)
  - [x] Zero breaking changes (~250 lines total)
- [ ] Multiple CSS frameworks
- [ ] GraphQL support

## Contributing

See the main [LiveTemplate CLAUDE.md](../../CLAUDE.md) for development guidelines.

## License

Same as LiveTemplate project.
