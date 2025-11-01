# LiveTemplate CLI (`lvt`) - Complete Guide

The `lvt` CLI is a Phoenix-inspired code generator for building LiveTemplate applications with CRUD functionality, authentication, and real-time features.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Command Reference](#command-reference)
  - [Creating Applications](#creating-applications)
  - [Generating Resources](#generating-resources)
  - [Generating Views](#generating-views)
  - [Generating Auth](#generating-auth)
  - [Managing Migrations](#managing-migrations)
  - [Kit Management](#kit-management)
- [Kits System](#kits-system)
- [Type System](#type-system)
- [Testing](#testing)
- [Project Structure](#project-structure)

---

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

Verify installation:

```bash
lvt --help
```

---

## Quick Start

```bash
# 1. Create a new application
lvt new myblog
cd myblog

# 2. Generate a CRUD resource
lvt gen posts title content published:bool

# 3. Run migrations (auto-generates database code)
lvt migration up

# 4. Run the app
go run cmd/myblog/main.go

# 5. Visit http://localhost:8080/posts
```

That's it! You have a fully functional CRUD application with:
- ✅ Create, Read, Update, Delete operations
- ✅ Search and filtering
- ✅ Pagination
- ✅ Real-time WebSocket updates
- ✅ Tailwind CSS styling
- ✅ E2E tests with Chromedp

---

## Command Reference

### Creating Applications

#### `lvt new <name>`

Creates a new LiveTemplate application.

**Usage:**

```bash
# Interactive mode (recommended for beginners)
lvt new

# Direct mode
lvt new myapp

# Specify kit (CSS framework)
lvt new myapp --kit multi     # Tailwind CSS (default)
lvt new myapp --kit simple    # Pico CSS
```

**What it generates:**

```
myapp/
├── cmd/myapp/main.go         # Application entry point
├── go.mod                    # Go module with tools directive
├── internal/
│   ├── app/                  # Handlers and templates
│   ├── database/
│   │   ├── db.go            # Database connection
│   │   ├── schema.sql       # Database schema
│   │   ├── queries.sql      # SQL queries (for sqlc)
│   │   ├── sqlc.yaml        # sqlc configuration
│   │   └── models/          # Generated code
│   └── shared/              # Shared utilities
├── web/assets/              # Static assets
└── README.md
```

**Kit Options:**

- `multi` - Multi-page app with Tailwind CSS (default)
- `single` - Single-page app with Tailwind CSS
- `simple` - Simple app with Pico CSS

---

### Generating Resources

#### `lvt gen <resource> <field:type>...`

Generates a full CRUD resource with database integration.

**Usage:**

```bash
# Interactive mode
lvt gen

# With explicit types
lvt gen users name:string email:string age:int

# With type inference (smart defaults)
lvt gen products name price quantity enabled created_at
# → Infers: name:string price:float quantity:int enabled:bool created_at:time
```

**What it generates:**

- `internal/app/{resource}/{resource}.go` - CRUD handler
- `internal/app/{resource}/{resource}.tmpl` - UI template
- `internal/app/{resource}/{resource}_test.go` - E2E tests
- `internal/app/{resource}/{resource}_ws_test.go` - WebSocket tests
- Database migration file (timestamped)
- SQL queries appended to `queries.sql`
- Auto-injected route in `main.go`

**Features:**

- ✅ CRUD operations
- ✅ Search across string fields
- ✅ Sorting by any field
- ✅ Pagination
- ✅ Real-time updates via WebSocket
- ✅ Form validation
- ✅ Statistics/counts
- ✅ CSS styling (from kit)
- ✅ Comprehensive tests

**Type Mappings:**

| CLI Type | Go Type   | SQL Type |
|----------|-----------|----------|
| string   | string    | TEXT     |
| int      | int64     | INTEGER  |
| bool     | bool      | BOOLEAN  |
| float    | float64   | REAL     |
| time     | time.Time | DATETIME |

**Relationships:**

```bash
# Basic foreign key (ON DELETE CASCADE)
lvt gen comments post_id:references:posts author text

# Custom ON DELETE behavior
lvt gen audit_logs user_id:references:users:set_null action

# Multiple references
lvt gen likes user_id:references:users post_id:references:posts

# Restrict deletion
lvt gen invoices customer_id:references:customers:restrict amount:float
```

---

### Generating Views

#### `lvt gen view <name>`

Generates a view-only handler without database integration.

**Usage:**

```bash
# Interactive mode
lvt gen view

# Direct mode
lvt gen view dashboard
```

**What it generates:**

- `internal/app/{view}/{view}.go` - View handler
- `internal/app/{view}/{view}.tmpl` - UI template
- `internal/app/{view}/{view}_test.go` - E2E tests
- `internal/app/{view}/{view}_ws_test.go` - WebSocket tests
- Auto-injected route in `main.go`

**Use cases:**

- Landing pages
- Dashboards
- Static content pages
- Custom UI components
- Counter/calculator apps

---

### Generating Auth

#### `lvt gen auth`

Generates a complete authentication system similar to Phoenix's `mix phx.gen.auth`.

**Usage:**

```bash
# Default: User struct, users table
lvt gen auth

# Custom struct name (table name auto-pluralized)
lvt gen auth Account              # Creates Account struct, accounts table

# Custom struct and table names
lvt gen auth Admin admin_users    # Creates Admin struct, admin_users table

# With feature flags
lvt gen auth --no-magic-link                    # Password only
lvt gen auth Account --no-email-confirm         # Custom names + no confirmation
lvt gen auth Admin admin_users --no-password    # Custom names + magic-link only
```

**Flags:**

- `--no-password` - Disable password authentication
- `--no-magic-link` - Disable magic-link authentication
- `--no-email-confirm` - Disable email confirmation flow
- `--no-password-reset` - Disable password reset functionality
- `--no-sessions-ui` - Disable session management UI
- `--no-csrf` - Disable CSRF protection middleware

**Note:** At least one authentication method (password or magic-link) must be enabled.

**What it generates:**

- `internal/app/auth/auth.go` - Complete auth handler with all flows
- `internal/app/auth/auth.tmpl` - LiveTemplate UI with Tailwind CSS
- `internal/app/auth/middleware.go` - Route protection middleware
- `internal/shared/password/password.go` - Password hashing (bcrypt)
- `internal/shared/email/email.go` - Email sender interface
- `internal/database/migrations/YYYYMMDDHHMMSS_create_auth_tables.sql` - Migration
- Auth queries appended to `internal/database/queries.sql`

**Database Tables:**

Default names (can be customized):
- `users` (or custom table name) - User accounts (email, optional password)
- `users_tokens` (or `{table}_tokens`) - Tokens for magic links, email confirmation, password reset

**Features:**

- ✅ **Customizable struct/table names** - Like Phoenix's `mix phx.gen.auth`
- ✅ **Smart pluralization** - User → users, Account → accounts, etc.
- ✅ **Complete auth handlers** - Registration, login, logout, password reset, email confirmation
- ✅ **LiveTemplate UI** - Tailwind CSS styled forms with error/success messages
- ✅ **Route protection middleware** - RequireAuth, RequireConfirmed, OptionalAuth
- ✅ **Password authentication** (bcrypt)
- ✅ **Magic-link email authentication**
- ✅ **Email confirmation flow**
- ✅ **Password reset functionality**
- ✅ **Session management** with secure cookies
- ✅ **CSRF protection** ready (gorilla/csrf)
- ✅ **Auto-updates `go.mod` dependencies**
- ✅ **EmailSender interface** (console logger + SMTP/Mailgun examples)
- ✅ **Case-insensitive email matching**
- ✅ **Production-ready security** (HTTP-only, secure, SameSite cookies)

**Next Steps:**

```bash
# 1. Run migrations
lvt migration up

# 2. Generate sqlc code
sqlc generate

# 3. Wire routes in main.go (see internal/app/auth/auth.go for examples)

# 4. Configure email sender (see internal/shared/email/email.go)
```

**Example main.go setup:**

```go
import (
	"yourapp/internal/app/auth"
	"yourapp/internal/shared/email"
	"github.com/livefir/livetemplate"
)

// Create auth handler
emailSender := email.NewConsoleEmailSender()
authHandler := auth.NewUserHandler(db, emailSender, "http://localhost:8080")

// Register routes
http.Handle("/auth", livetemplate.NewHandler(authHandler, "internal/app/auth/auth.tmpl"))
http.HandleFunc("/auth/logout", authHandler.HandleLogout)
http.HandleFunc("/auth/magic", authHandler.HandleMagicLinkVerify)      // if magic-link enabled
http.HandleFunc("/auth/reset", authHandler.HandleResetPassword)        // if password-reset enabled
http.HandleFunc("/auth/confirm", authHandler.HandleConfirmEmail)       // if email-confirm enabled

// Protected route example
protectedHandler := authHandler.RequireAuth(http.HandlerFunc(myHandler))
http.Handle("/dashboard", protectedHandler)
```

---

### Managing Migrations

#### `lvt migration <command>`

Manages database migrations using Goose.

**Commands:**

```bash
# Apply all pending migrations (and run sqlc generate)
lvt migration up

# Rollback one migration
lvt migration down

# Show migration status
lvt migration status

# Create a new migration
lvt migration create add_user_roles
```

**Auto-generated Migrations:**

When you run `lvt gen`, migrations are automatically created:

```bash
lvt gen posts title content
# Creates: internal/database/migrations/20240315120000_create_posts.sql
```

**Migration Format:**

```sql
-- +goose Up
CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE posts;
```

**sqlc Integration:**

`lvt migration up` automatically runs `sqlc generate` after applying migrations, ensuring your Go database code stays in sync.

---

### Kit Management

#### `lvt kits <command>`

Manages CSS framework kits.

**Commands:**

```bash
# List all available kits
lvt kits list

# Show kit information
lvt kits info tailwind

# Customize a kit for your project
lvt kits customize tailwind

# Customize globally (all projects)
lvt kits customize tailwind --global

# Validate kit structure
lvt kits validate .lvt/kits/tailwind
```

**Available System Kits:**

| Kit      | Framework   | Description                    |
|----------|-------------|--------------------------------|
| multi    | Tailwind    | Multi-page app (default)       |
| single   | Tailwind    | Single-page app                |
| simple   | Pico CSS    | Minimal semantic HTML          |

---

## Kits System

Kits are complete starter packages that include:

1. **CSS Framework Integration** - ~60 helper methods for CSS classes
2. **Components** - Pre-built UI blocks (form, table, layout, pagination, etc.)
3. **Templates** - Generator templates for resources, views, and apps

### Kit Cascade

Kits are loaded with this priority:

1. **Project**: `.lvt/kits/<name>/` (highest)
2. **User**: `~/.config/lvt/kits/<name>/`
3. **System**: Embedded in binary (fallback)

This allows:
- Per-project customization (`.lvt/kits/`)
- Global customization (`~/.config/lvt/kits/`)
- System defaults as fallback

### Customizing Kits

```bash
# 1. Copy kit to project
lvt kits customize multi

# 2. Edit templates
cd .lvt/kits/multi/templates
# Edit resource.go.tmpl, resource.tmpl, etc.

# 3. Generate with custom templates
lvt gen products name price
# Uses your customized templates
```

---

## Type System

### Type Inference

The CLI automatically infers types from field names:

```bash
lvt gen articles title content published_at author email price
# Infers: title=string, content=string, published_at=time,
#         author=string, email=string, price=float
```

**Inference Rules:**

**String** (default):
- `name`, `title`, `description`, `email`, `username`, `url`, `slug`, `address`

**Integer:**
- `age`, `count`, `quantity`, `views`, `likes`, `score`, `rank`, `year`
- `*_count`, `*_number`, `*_index`

**Float:**
- `price`, `amount`, `rating`, `latitude`, `longitude`
- `*_price`, `*_amount`, `*_rate`

**Boolean:**
- `enabled`, `active`, `published`, `verified`, `approved`, `deleted`
- `is_*`, `has_*`, `can_*`

**Time:**
- `created_at`, `updated_at`, `deleted_at`, `published_at`
- `*_at`, `*_date`, `*_time`

### Type Aliases

- `str`, `text` → `string`
- `integer` → `int`
- `boolean` → `bool`
- `float64`, `decimal` → `float`
- `datetime`, `timestamp` → `time`

---

## Testing

Each generated resource includes comprehensive tests.

### WebSocket Tests (`*_ws_test.go`)

Fast unit tests for WebSocket protocol:

```bash
# Run WebSocket tests
go test ./internal/app/users -run WebSocket

# Features:
# - Dynamic port allocation
# - WebSocket connection testing
# - CRUD action testing
# - Server log capture
# - Fast execution (~2-5 seconds)
```

### E2E Tests (`*_test.go`)

Full browser tests with Chromedp:

```bash
# Run E2E tests
go test ./internal/app/users -run E2E

# Features:
# - Real browser interactions
# - Visual verification
# - Screenshot capture
# - Console log access
# - Comprehensive (~20-60 seconds)
```

### Skip Slow Tests

```bash
go test -short ./...
```

---

## Project Structure

Generated apps follow idiomatic Go conventions:

```
myapp/
├── cmd/myapp/main.go          # Application entry point
├── go.mod                     # Go module
├── internal/
│   ├── app/                   # Handlers and templates (co-located!)
│   │   ├── posts/
│   │   │   ├── posts.go       # Handler
│   │   │   ├── posts.tmpl     # Template
│   │   │   ├── posts_test.go  # E2E tests
│   │   │   └── posts_ws_test.go # WebSocket tests
│   │   └── users/
│   ├── database/
│   │   ├── db.go              # Database connection
│   │   ├── migrations/        # Migration files
│   │   ├── queries.sql        # SQL queries (sqlc)
│   │   ├── sqlc.yaml          # sqlc config
│   │   └── models/            # Generated code
│   └── shared/                # Shared utilities
│       ├── password/          # Password hashing
│       └── email/             # Email sender
└── web/assets/                # Static assets
```

**Key Design Decisions:**

- Templates live next to handlers for easy discovery
- `internal/` prevents external imports
- Database layer uses sqlc for type safety
- Migrations are version-controlled

---

## Development Workflow

### Standard Workflow

```bash
# 1. Create app
lvt new myapp
cd myapp

# 2. Generate resources
lvt gen users name email
lvt gen posts title content user_id:references:users

# 3. Run migrations (auto-generates DB code)
lvt migration up

# 4. Run tests
go test ./...

# 5. Run app
go run cmd/myapp/main.go
```

### Iterative Development

```bash
# Add a field to existing resource
lvt migration create add_users_bio
# Edit migration file to add bio column

# Run migration
lvt migration up

# Update queries.sql to include bio
# Update handler and template

# Test changes
go test ./internal/app/users
```

---

## Best Practices

### 1. Start Simple

Begin with core resources, add features incrementally:

```bash
lvt new blog
cd blog
lvt gen posts title content      # Start here
lvt gen comments post_id:references:posts author text  # Add later
```

### 2. Use Type Inference

Let the CLI infer types for common fields:

```bash
# Instead of:
lvt gen users name:string email:string created_at:time

# Do this:
lvt gen users name email created_at
```

### 3. Test Continuously

Run tests after each change:

```bash
go test ./...
```

### 4. Customize Templates

Copy and modify templates for your needs:

```bash
lvt kits customize multi
cd .lvt/kits/multi/templates
# Edit templates
```

### 5. Use Migrations

Always use migrations for schema changes:

```bash
lvt migration create add_user_roles
# Edit migration file
lvt migration up
```

---

## Troubleshooting

### GOWORK conflicts

If creating an app inside an existing Go workspace:

```bash
GOWORK=off go run cmd/myapp/main.go
```

### Migration errors

Check migration status:

```bash
lvt migration status
```

Rollback if needed:

```bash
lvt migration down
```

### sqlc errors

Manually run sqlc:

```bash
sqlc generate
```

### Test failures

Run with verbose output:

```bash
go test -v ./internal/app/users
```

---

## Examples

### Blog Application

```bash
lvt new myblog
cd myblog

lvt gen posts title content published:bool
lvt gen categories name description
lvt gen comments post_id:references:posts author text

lvt migration up
go test ./...
go run cmd/myblog/main.go
```

### E-commerce Store

```bash
lvt new mystore
cd mystore

lvt gen products name price:float stock:int
lvt gen customers name email
lvt gen orders customer_id:references:customers total:float

lvt migration up
go test ./...
go run cmd/mystore/main.go
```

### Social Network

```bash
lvt new mysocial
cd mysocial

# Generate auth first
lvt gen auth

lvt gen profiles user_id:references:users bio avatar_url
lvt gen posts user_id:references:users content
lvt gen likes user_id:references:users post_id:references:posts

lvt migration up
go test ./...
go run cmd/mysocial/main.go
```

---

## Next Steps

1. **Add Authentication** - Run `lvt gen auth`
2. **Add More Resources** - Generate CRUD for your domain
3. **Customize Templates** - Tailor to your design
4. **Add Business Logic** - Extend handlers
5. **Deploy** - Build and deploy your app

For more information:
- [API Reference](../references/api-reference.md)
- [Template Support Matrix](../references/template-support-matrix.md)
- [LiveTemplate Documentation](../../CLAUDE.md)
