# LiveTemplate API Reference

Complete reference for LiveTemplate manifests, interfaces, and CLI commands.

## Table of Contents

- [Component Manifest Schema](#component-manifest-schema)
- [Kit Manifest Schema](#kit-manifest-schema)
- [CSSHelpers Interface](#csshelpers-interface)
- [Config File Reference](#config-file-reference)
- [CLI Commands](#cli-commands)

---

## Component Manifest Schema

**Note:** Components are part of kits in LiveTemplate. Component manifests are located in the `components/` directory within a kit.

Component manifests are defined in `component.yaml` files inside a kit's `components/<name>/` directory.

### Schema

```yaml
# Required fields
name: string                     # Component name (lowercase, alphanumeric, hyphens)
version: string                  # Semantic version (e.g., "1.0.0")
description: string              # Brief description of component

# Categorization
category: string                 # Component category (see categories below)
tags: []string                   # Search tags

# Inputs definition
inputs:
  - name: string                 # Input name (PascalCase recommended)
    type: string                 # Input type (string, int, float, bool, array, object)
    description: string          # Input description
    required: bool               # Whether input is required
    default: any                 # Default value (optional)

# Templates
templates: []string              # List of template files (e.g., ["form.tmpl", "header.tmpl"])

# Dependencies
dependencies: []string           # Component dependencies (e.g., ["layout", "button"])

# Kit preference (optional)
kit: string                      # Preferred kit for preview (e.g., "tailwind")
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique component name (lowercase, alphanumeric, hyphens only) |
| `version` | string | Yes | Semantic version following semver spec |
| `description` | string | Yes | Brief description of what the component does |
| `category` | string | No | Component category for organization |
| `tags` | []string | No | Searchable tags |
| `inputs` | []Input | No | Input specifications (see Input Schema below) |
| `templates` | []string | Yes | List of template files relative to component directory |
| `dependencies` | []string | No | Names of other components this component depends on |
| `kit` | string | No | Preferred CSS kit for preview |

### Input Schema

```yaml
name: string              # Input name (e.g., "Title", "Items")
type: string              # One of: string, int, float, bool, array, object
description: string       # What this input is for
required: bool            # Whether this input must be provided
default: any              # Default value if not provided (type must match)
```

### Valid Input Types

| Type | Go Type | Example Value |
|------|---------|---------------|
| `string` | string | "Hello World" |
| `int` | int | 42 |
| `float` | float64 | 3.14 |
| `bool` | bool | true |
| `array` | []interface{} | ["a", "b", "c"] |
| `object` | map[string]interface{} | {"key": "value"} |

### Standard Categories

| Category | Description | Examples |
|----------|-------------|----------|
| `forms` | Form components | Input fields, selects, checkboxes, form groups |
| `layout` | Layout components | Containers, grids, sections, wrappers |
| `data` | Data display | Tables, lists, cards, data grids |
| `feedback` | User feedback | Alerts, modals, toasts, notifications |
| `navigation` | Navigation | Menus, breadcrumbs, tabs, pagination |
| `buttons` | Buttons | Button variants, button groups |
| `typography` | Text elements | Headings, paragraphs, text styles |
| `media` | Media elements | Images, videos, galleries |

### Example: Form Component

```yaml
name: form
version: 1.0.0
description: Form with fields and validation support
category: forms
tags:
  - form
  - input
  - validation

inputs:
  - name: Title
    type: string
    description: Form title
    required: false
    default: ""

  - name: Fields
    type: array
    description: Array of field objects (Name, Type, Label, Value, Placeholder)
    required: true

  - name: SubmitURL
    type: string
    description: Form submission URL
    required: true

  - name: Method
    type: string
    description: HTTP method (GET or POST)
    required: false
    default: "POST"

  - name: SubmitText
    type: string
    description: Submit button text
    required: false
    default: "Submit"

templates:
  - form.tmpl

dependencies:
  - layout

kit: tailwind
```

---

## Kit Manifest Schema

Kit manifests are defined in `kit.yaml` files.

### Schema

```yaml
# Required fields
name: string                     # Kit name (lowercase, alphanumeric, hyphens)
version: string                  # Semantic version (e.g., "1.0.0")
description: string              # Brief description of CSS framework

# Framework information
framework: string                # Framework name (e.g., "tailwind", "bulma")
author: string                   # Kit author (optional)
license: string                  # License (e.g., "MIT") (optional)

# CDN link (optional but recommended)
cdn: string                      # CSS CDN URL

# Components included in this kit
components: []string             # List of component template files

# Templates included in this kit
templates:
  resource: bool                 # Includes resource templates
  view: bool                     # Includes view templates
  app: bool                      # Includes app templates

# Categorization
tags: []string                   # Searchable tags
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique kit name (lowercase, alphanumeric, hyphens only) |
| `version` | string | Yes | Semantic version following semver spec |
| `description` | string | Yes | Brief description of the CSS framework |
| `framework` | string | Yes | Framework identifier (e.g., "tailwind", "bulma", "pico") |
| `author` | string | No | Kit author name or organization |
| `license` | string | No | License type (e.g., "MIT", "Apache 2.0") |
| `cdn` | string | No | CSS CDN URL (recommended if framework provides CDN) |
| `components` | []string | No | List of component template files included in this kit |
| `templates.resource` | bool | No | Whether kit includes resource templates |
| `templates.view` | bool | No | Whether kit includes view templates |
| `templates.app` | bool | No | Whether kit includes app templates |
| `tags` | []string | No | Searchable tags |

### Example: Tailwind Kit

```yaml
name: tailwind
version: 1.0.0
description: Tailwind CSS utility-first framework integration
framework: tailwind
author: LiveTemplate Team
license: MIT
cdn: https://cdn.tailwindcss.com
components:
  - detail.tmpl
  - form.tmpl
  - layout.tmpl
  - pagination.tmpl
  - search.tmpl
  - sort.tmpl
  - stats.tmpl
  - table.tmpl
  - toolbar.tmpl
templates:
  resource: true
  view: true
  app: true
tags:
  - css
  - utility
  - tailwind
  - responsive
```

### Example: Pico Kit

```yaml
name: pico
version: 1.0.0
description: Pico CSS semantic/classless framework starter kit
framework: pico
author: LiveTemplate Team
license: MIT
cdn: <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
components:
  - detail.tmpl
  - form.tmpl
  - layout.tmpl
  - pagination.tmpl
  - search.tmpl
  - sort.tmpl
  - stats.tmpl
  - table.tmpl
  - toolbar.tmpl
templates:
  resource: true
  view: true
  app: true
tags:
  - css
  - semantic
  - classless
```

---

## CSSHelpers Interface

The CSSHelpers interface defines ~70 methods that all kits must implement.

### Framework Information

```go
CSSCDN() string  // Returns CDN URL for CSS framework
```

**Usage in templates:**
```html
<link rel="stylesheet" href="[[csscdn]]">
```

---

### Layout Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `ContainerClass()` | - | string | Main container class |
| `SectionClass()` | - | string | Section wrapper class |
| `BoxClass()` | - | string | Box/panel class |
| `ColumnClass()` | - | string | Single column class |
| `ColumnsClass()` | - | string | Column container class |

**Usage in templates:**
```html
<div class="[[containerClass]]">
  <section class="[[sectionClass]]">
    <div class="[[boxClass]]">
      Content
    </div>
  </section>
</div>
```

---

### Form Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `FormClass()` | - | string | Form element class |
| `FieldClass()` | - | string | Form field wrapper class |
| `LabelClass()` | - | string | Label element class |
| `InputClass()` | - | string | Text input class |
| `TextareaClass()` | - | string | Textarea class |
| `SelectClass()` | - | string | Select dropdown class |
| `CheckboxClass()` | - | string | Checkbox input class |
| `RadioClass()` | - | string | Radio input class |
| `ButtonClass(variant)` | variant: string | string | Button class with variant |
| `ButtonGroupClass()` | - | string | Button group container |

**Variants for ButtonClass:**
- `primary` - Primary action button
- `secondary` - Secondary action button
- `success` - Success/confirmation button
- `danger` - Destructive action button
- `warning` - Warning button
- `info` - Informational button

**Usage in templates:**
```html
<form class="[[formClass]]">
  <div class="[[fieldClass]]">
    <label class="[[labelClass]]">Name</label>
    <input class="[[inputClass]]" type="text">
  </div>

  <div class="[[fieldClass]]">
    <label class="[[labelClass]]">Description</label>
    <textarea class="[[textareaClass]]"></textarea>
  </div>

  <div class="[[buttonGroupClass]]">
    <button class="[[buttonClass "primary"]]">Save</button>
    <button class="[[buttonClass "secondary"]]">Cancel</button>
  </div>
</form>
```

---

### Table Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `TableClass()` | - | string | Table element class |
| `TableContainerClass()` | - | string | Table wrapper/container class |
| `TheadClass()` | - | string | Table header section class |
| `TbodyClass()` | - | string | Table body section class |
| `ThClass()` | - | string | Table header cell class |
| `TdClass()` | - | string | Table data cell class |
| `TrClass()` | - | string | Table row class |

**Usage in templates:**
```html
<div class="[[tableContainerClass]]">
  <table class="[[tableClass]]">
    <thead class="[[theadClass]]">
      <tr class="[[trClass]]">
        <th class="[[thClass]]">Name</th>
        <th class="[[thClass]]">Value</th>
      </tr>
    </thead>
    <tbody class="[[tbodyClass]]">
      <tr class="[[trClass]]">
        <td class="[[tdClass]]">Data</td>
        <td class="[[tdClass]]">123</td>
      </tr>
    </tbody>
  </table>
</div>
```

---

### Navigation Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `NavbarClass()` | - | string | Navigation bar class |
| `NavbarBrandClass()` | - | string | Brand/logo section class |
| `NavbarMenuClass()` | - | string | Menu container class |
| `NavbarItemClass()` | - | string | Individual nav item class |
| `NavbarStartClass()` | - | string | Left-aligned nav section |
| `NavbarEndClass()` | - | string | Right-aligned nav section |

**Usage in templates:**
```html
<nav class="[[navbarClass]]">
  <div class="[[navbarBrandClass]]">
    <a href="/">Logo</a>
  </div>
  <div class="[[navbarMenuClass]]">
    <div class="[[navbarStartClass]]">
      <a class="[[navbarItemClass]]">Home</a>
      <a class="[[navbarItemClass]]">About</a>
    </div>
    <div class="[[navbarEndClass]]">
      <a class="[[navbarItemClass]]">Login</a>
    </div>
  </div>
</nav>
```

---

### Typography Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `TitleClass(level)` | level: int | string | Heading class (level 1-6) |
| `SubtitleClass()` | - | string | Subtitle class |
| `TextClass(size)` | size: string | string | Text size class |
| `TextMutedClass()` | - | string | Muted/secondary text |
| `TextPrimaryClass()` | - | string | Primary color text |
| `TextDangerClass()` | - | string | Danger/error text |
| `TextSuccessClass()` | - | string | Success text |
| `TextWarningClass()` | - | string | Warning text |

**Size options for TextClass:**
- `xs` - Extra small
- `sm` - Small
- `md` - Medium (base)
- `lg` - Large
- `xl` - Extra large

**Usage in templates:**
```html
<h1 class="[[titleClass 1]]">Main Title</h1>
<h2 class="[[titleClass 2]]">Section Title</h2>
<p class="[[subtitleClass]]">Subtitle text</p>
<p class="[[textClass "lg"]]">Large text</p>
<p class="[[textMutedClass]]">Secondary information</p>
<p class="[[textDangerClass]]">Error message</p>
<p class="[[textSuccessClass]]">Success message</p>
```

---

### Card Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `CardClass()` | - | string | Card container class |
| `CardHeaderClass()` | - | string | Card header class |
| `CardBodyClass()` | - | string | Card body class |
| `CardFooterClass()` | - | string | Card footer class |

**Usage in templates:**
```html
<div class="[[cardClass]]">
  <div class="[[cardHeaderClass]]">
    <h3>Card Title</h3>
  </div>
  <div class="[[cardBodyClass]]">
    <p>Card content goes here</p>
  </div>
  <div class="[[cardFooterClass]]">
    <button class="[[buttonClass "primary"]]">Action</button>
  </div>
</div>
```

---

### Pagination Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `PaginationClass()` | - | string | Pagination container class |
| `PaginationListClass()` | - | string | Pagination list class |
| `PaginationItemClass()` | - | string | Pagination item class |
| `PaginationButtonClass(state)` | state: string | string | Pagination button with state |

**States for PaginationButtonClass:**
- `active` - Current page
- `disabled` - Disabled page link
- `normal` - Regular page link

**Usage in templates:**
```html
<nav class="[[paginationClass]]">
  <ul class="[[paginationListClass]]">
    <li class="[[paginationItemClass]]">
      <a class="[[paginationButtonClass "normal"]]">1</a>
    </li>
    <li class="[[paginationItemClass]]">
      <a class="[[paginationButtonClass "active"]]">2</a>
    </li>
    <li class="[[paginationItemClass]]">
      <a class="[[paginationButtonClass "disabled"]]">3</a>
    </li>
  </ul>
</nav>
```

---

### Alert/Notification Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `AlertClass(variant)` | variant: string | string | Alert box class with variant |
| `NotificationClass(variant)` | variant: string | string | Notification class with variant |

**Variants:**
- `info` - Informational message
- `success` - Success message
- `warning` - Warning message
- `error` / `danger` - Error message

**Usage in templates:**
```html
<div class="[[alertClass "success"]]">
  Operation completed successfully!
</div>

<div class="[[alertClass "error"]]">
  An error occurred
</div>
```

---

### Badge/Tag Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `BadgeClass(variant)` | variant: string | string | Badge class with variant |
| `TagClass(variant)` | variant: string | string | Tag class with variant |

**Variants:** Same as alert variants (info, success, warning, danger)

**Usage in templates:**
```html
<span class="[[badgeClass "primary"]]">New</span>
<span class="[[tagClass "info"]]">Category</span>
```

---

### Modal Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `ModalClass()` | - | string | Modal container class |
| `ModalBackgroundClass()` | - | string | Modal background/overlay class |
| `ModalContentClass()` | - | string | Modal content area class |
| `ModalCloseClass()` | - | string | Modal close button class |

**Usage in templates:**
```html
<div class="[[modalClass]]">
  <div class="[[modalBackgroundClass]]"></div>
  <div class="[[modalContentClass]]">
    <button class="[[modalCloseClass]]">×</button>
    <p>Modal content</p>
  </div>
</div>
```

---

### Loading Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `SpinnerClass()` | - | string | Spinner/loading indicator class |
| `LoadingClass()` | - | string | Loading state class |

---

### Grid Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `GridClass()` | - | string | CSS Grid container class |
| `GridItemClass()` | - | string | CSS Grid item class |

---

### Flex Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `FlexClass()` | - | string | Flexbox container class |
| `FlexItemClass()` | - | string | Flexbox item class |

---

### Spacing Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `MarginClass(size)` | size: string | string | Margin utility class |
| `PaddingClass(size)` | size: string | string | Padding utility class |

**Size options:** `0`, `1`, `2`, `3`, `4`, `5`, `auto` (framework-dependent)

---

### Display Helpers

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `HiddenClass()` | - | string | Hide element class |
| `VisibleClass()` | - | string | Show element class |

---

### Framework Checks

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `NeedsWrapper()` | - | bool | Whether framework needs wrapper div |
| `NeedsArticle()` | - | bool | Whether framework uses article tags |

---

### Template Utility Functions

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `Dict(values...)` | variadic interface{} | map[string]interface{} | Create map from key-value pairs |
| `Until(count)` | count: int | []int | Generate range from 1 to count |
| `Add(a, b)` | a, b: int | int | Add two integers |

**Usage in templates:**
```html
<!-- Dict: Create map for template invocation -->
[[template "alert" dict "Message" "Hello" "Type" "info"]]

<!-- Until: Generate range -->
[[range until 5]]
  <span>Item [[.]]</span>
[[end]]
<!-- Outputs: Item 1, Item 2, Item 3, Item 4, Item 5 -->

<!-- Add: Arithmetic -->
<p>Next page: [[add .CurrentPage 1]]</p>
```

---

## Config File Reference

Configuration is stored in `~/.config/lvt/config.yaml`.

### Schema

```yaml
# Kit search paths
kits_paths:
  - /path/to/kits/dir1
  - /path/to/kits/dir2

# Default preferences
defaults:
  kit: tailwind              # Default kit for new projects
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `kits_paths` | []string | Directories to search for custom kits |
| `defaults.kit` | string | Default CSS kit for new projects (not yet implemented) |

### Example

```yaml
kits_paths:
  - /Users/you/.lvt/kits
  - /Users/you/projects/custom-kits

defaults:
  kit: tailwind
```

---

## CLI Commands

Complete reference of all lvt CLI commands.

### App Commands

#### lvt new

Create a new Go web application.

```bash
lvt new <name> [options]
```

**Arguments:**
- `<name>` - Application name (required)

**Options:**
- CSS framework is determined by the chosen kit
- `--dir <path>` - Directory to create app in (default: ./<name>)

**Examples:**
```bash
lvt new myapp
lvt new myapp --kit multi  # Uses Tailwind CSS
lvt new blogapp --kit simple --dir ~/projects/blogapp  # Uses Pico CSS
```

---

#### lvt gen

Generate a CRUD resource.

```bash
lvt gen <resource> [fields...] [options]
```

**Arguments:**
- `<resource>` - Resource name (singular, e.g., "product")
- `[fields...]` - Field definitions (e.g., "name", "price:float", "stock:int")

**Field Types:**
- `name` (no type) - Default to string
- `name:string` - Text field
- `name:text` - Textarea field
- `name:int` - Integer field
- `name:float` - Float field
- `name:bool` - Boolean checkbox
- `name:date` - Date field
- `name:datetime` - Datetime field

**Options:**
- CSS framework is determined by the kit

**Examples:**
```bash
lvt gen products name price:float stock:int
lvt gen articles title content:text published:bool  # Uses kit's CSS
lvt gen users name email password:string created_at:datetime
```

---

### Kit Commands

**Note:** Components are developed as part of kits. To work on components, customize a kit using `lvt kits customize <name> --only components`.

#### lvt kits list

List available kits.

```bash
lvt kits list [options]
```

**Options:**
- `--filter <source>` - Filter by source (system, local, community, all)
- `--format <format>` - Output format (table, json, simple)
- `--search <query>` - Search by name or description

**Examples:**
```bash
lvt kits list
lvt kits list --filter=system
lvt kits list --format=json
lvt kits list --search=tailwind
```

---

#### lvt kits info

Show kit information.

```bash
lvt kits info <name>
```

**Arguments:**
- `<name>` - Kit name

**Examples:**
```bash
lvt kits info tailwind
lvt kits info bulma
```

---

#### lvt kits create

Create a new kit.

```bash
lvt kits create <name>
```

**Arguments:**
- `<name>` - Kit name

**Examples:**
```bash
lvt kits create bootstrap
lvt kits create myframework
```

---

#### lvt kits validate

Validate a kit.

```bash
lvt kits validate <path>
```

**Arguments:**
- `<path>` - Path to kit directory

**Examples:**
```bash
lvt kits validate ~/.lvt/kits/mykit
lvt kits validate .
```

---

#### lvt kits customize

Copy a kit for customization.

```bash
lvt kits customize <name> [options]
```

**Arguments:**
- `<name>` - Kit name to customize

**Options:**
- `--global` - Copy to user config directory (`~/.config/lvt/kits/`) instead of project directory (`.lvt/kits/`)
- `--only <type>` - Copy only specific parts (components or templates)

**Examples:**
```bash
# Copy entire kit to project directory
lvt kits customize tailwind

# Copy to global config for all projects
lvt kits customize tailwind --global

# Copy only components
lvt kits customize tailwind --only components

# Copy only templates
lvt kits customize tailwind --only templates
```

**Kit Customization Cascade:**

When you customize a kit, LiveTemplate searches in this order:
1. **Project**: `.lvt/kits/<name>/` (highest priority)
2. **User**: `~/.config/lvt/kits/<name>/`
3. **System**: Embedded kits (fallback)

This allows project-specific and user-specific overrides.

---

### Config Commands

#### lvt config list

List all configuration.

```bash
lvt config list
```

---

#### lvt config get

Get a configuration value.

```bash
lvt config get <key>
```

**Arguments:**
- `<key>` - Configuration key (kits_paths)

**Examples:**
```bash
lvt config get kits_paths
```

---

#### lvt config set

Set a configuration value.

```bash
lvt config set <key> <value>
```

**Arguments:**
- `<key>` - Configuration key
- `<value>` - Configuration value

**Examples:**
```bash
lvt config set kits_paths ~/.lvt/kits
```

---

### Serve Command

#### lvt serve

Start development server.

```bash
lvt serve [options]
```

**Options:**
- `--port <port>` / `-p <port>` - Server port (default: 3000)
- `--host <host>` / `-h <host>` - Server host (default: localhost)
- `--dir <path>` / `-d <path>` - Project directory (default: .)
- `--mode <mode>` / `-m <mode>` - Force mode (component, kit, app)
- `--no-browser` - Don't open browser automatically
- `--no-reload` - Disable hot reload

**Examples:**
```bash
lvt serve
lvt serve --port 8080
lvt serve --host 0.0.0.0 --port 3000
lvt serve --mode component
lvt serve --no-browser --no-reload
```

---

Last updated: 2025-10-17
