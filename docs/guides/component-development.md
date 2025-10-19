# Component Development Guide

This guide teaches you how to create custom components for LiveTemplate. Components are reusable UI template blocks that work with any CSS kit.

## Table of Contents

- [Overview](#overview)
- [Component Structure](#component-structure)
- [Creating Your First Component](#creating-your-first-component)
- [Component Manifest Reference](#component-manifest-reference)
- [Template Guidelines](#template-guidelines)
- [Testing Components](#testing-components)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Publishing Components](#publishing-components)

---

## Overview

Components are self-contained UI blocks consisting of:

1. **component.yaml** - Manifest describing the component
2. **\*.tmpl** - Template files with the actual HTML/template code
3. **README.md** - Documentation for users
4. **examples/** - (Optional) Usage examples
5. **LICENSE** - (Optional) License file

Components are CSS-independent and should work with any kit (Tailwind, Bulma, Pico, or none).

### Key Principles

- **Kit-agnostic**: Use kit helper functions, don't hardcode CSS classes
- **Data-driven**: Accept inputs via the manifest, don't assume data structure
- **Composable**: Components can include other components
- **Documented**: Provide clear examples and input specifications

---

## Component Structure

```
mycomponent/
├── component.yaml          # Manifest (required)
├── mycomponent.tmpl        # Main template (required)
├── part1.tmpl             # Additional templates (optional)
├── part2.tmpl             # Additional templates (optional)
├── README.md              # Documentation (recommended)
├── examples/              # Usage examples (optional)
│   └── basic.html
└── LICENSE                # License (optional)
```

### Required Files

- **component.yaml**: Component metadata and input specification
- **\*.tmpl**: At least one template file

### Recommended Files

- **README.md**: User-facing documentation
- **examples/**: Usage examples

---

## Creating Your First Component

### Step 1: Generate Boilerplate

```bash
lvt components create alert --category feedback
```

This creates:
```
~/.lvt/components/alert/
├── component.yaml
├── alert.tmpl
└── README.md
```

### Step 2: Edit component.yaml

```yaml
name: alert
version: 1.0.0
description: Alert box with different severity levels
category: feedback
tags:
  - notification
  - message
  - alert

inputs:
  - name: Message
    type: string
    description: The alert message to display
    required: true

  - name: Type
    type: string
    description: Alert type (info, success, warning, error)
    default: info

  - name: Dismissible
    type: bool
    description: Whether the alert can be dismissed
    default: false

templates:
  - alert.tmpl

dependencies: []
```

### Step 3: Write Template

Edit `alert.tmpl`:

```html
[[- /*
Alert Component
Shows a colored alert box with icon and message
*/ -]]

[[- define "alert" -]]
<div class="[[cardClass]] [[if eq .Type "error"]]bg-red-100[[else if eq .Type "success"]]bg-green-100[[else if eq .Type "warning"]]bg-yellow-100[[else]]bg-blue-100[[end]]" role="alert">
  <div class="[[cardBodyClass]] [[if .Dismissible]]flex justify-between items-center[[end]]">
    <div>
      [[- if eq .Type "error" -]]
      <span class="text-red-600">❌</span>
      [[- else if eq .Type "success" -]]
      <span class="text-green-600">✅</span>
      [[- else if eq .Type "warning" -]]
      <span class="text-yellow-600">⚠️</span>
      [[- else -]]
      <span class="text-blue-600">ℹ️</span>
      [[- end -]]
      <strong>[[.Message]]</strong>
    </div>
    [[- if .Dismissible -]]
    <button class="[[buttonClass "secondary"]]" onclick="this.parentElement.parentElement.remove()">
      ×
    </button>
    [[- end -]]
  </div>
</div>
[[- end -]]
```

### Step 4: Update README

Edit `README.md`:

```markdown
# Alert Component

Displays a colored alert box with icon and optional dismiss button.

## Usage

\`\`\`go
data := map[string]interface{}{
    "Message": "Operation completed successfully!",
    "Type": "success",
    "Dismissible": true,
}
\`\`\`

## Inputs

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| Message | string | Yes | - | The alert message to display |
| Type | string | No | info | Alert type (info, success, warning, error) |
| Dismissible | bool | No | false | Whether the alert can be dismissed |

## Examples

### Success Alert
\`\`\`html
[[template "alert" dict "Message" "Saved!" "Type" "success"]]
\`\`\`

### Error Alert
\`\`\`html
[[template "alert" dict "Message" "Failed to save" "Type" "error"]]
\`\`\`
```

### Step 5: Validate Component

```bash
lvt components validate ~/.lvt/components/alert
```

Expected output:
```
Validating component at: /Users/you/.lvt/components/alert

✅ Component structure is valid
✅ Manifest is valid
✅ Template syntax is valid
✅ Documentation is present

Component validation passed!
```

### Step 6: Test with Development Server

```bash
cd ~/.lvt/components/alert
lvt serve
```

This opens a live preview where you can:
- Edit test data in JSON editor
- See real-time preview
- Test with different kits

---

## Component Manifest Reference

### Basic Structure

```yaml
# Required fields
name: component-name          # Lowercase, alphanumeric + hyphens
version: 1.0.0               # Semantic versioning
description: Brief description of what this component does

# Categorization
category: forms              # forms, layout, data, feedback, navigation, etc.
tags:                        # Array of searchable tags
  - form
  - input
  - validation

# Inputs definition
inputs:
  - name: Title             # Input name (PascalCase recommended)
    type: string            # string, int, bool, array, object
    description: Input description
    required: true          # Whether input is required
    default: ""             # Default value if not provided

# Templates list
templates:
  - main.tmpl              # List of template files
  - parts/header.tmpl

# Dependencies (optional)
dependencies:               # Other components this depends on
  - layout
  - button
```

### Input Types

| Type | Go Type | Example |
|------|---------|---------|
| string | string | "Hello World" |
| int | int | 42 |
| float | float64 | 3.14 |
| bool | bool | true |
| array | []interface{} | ["a", "b", "c"] |
| object | map[string]interface{} | {"key": "value"} |

### Categories

Standard categories (for consistency):

- **forms**: Form components (inputs, selects, checkboxes, etc.)
- **layout**: Layout components (containers, grids, sections)
- **data**: Data display (tables, lists, cards)
- **feedback**: User feedback (alerts, modals, toasts)
- **navigation**: Navigation (menus, breadcrumbs, tabs)
- **buttons**: Button variants
- **typography**: Text and heading components

---

## Template Guidelines

### Use [[ ]] Delimiters

LiveTemplate uses `[[ ]]` instead of `{{ }}`:

```html
<!-- Correct -->
<h1>[[.Title]]</h1>
[[if .ShowContent]]
  <p>[[.Content]]</p>
[[end]]

<!-- Incorrect -->
<h1>{{.Title}}</h1>
{{if .ShowContent}}
  <p>{{.Content}}</p>
{{end}}
```

### Use Kit Helper Functions

Always use kit helpers instead of hardcoded CSS classes:

```html
<!-- Correct (works with all kits) -->
<button class="[[buttonClass "primary"]]">Submit</button>
<div class="[[containerClass]]">
  <input class="[[inputClass]]" type="text">
</div>

<!-- Incorrect (hardcoded Tailwind classes) -->
<button class="bg-blue-500 text-white px-4 py-2">Submit</button>
```

### Available Helper Functions

See [API Reference](api-reference.md#kit-helper-functions) for complete list. Common ones:

```html
[[csscdn]]                          # CSS CDN link
[[containerClass]]                  # Container class
[[buttonClass "primary"]]           # Button with variant
[[inputClass]]                      # Input field
[[tableClass]]                      # Table
[[cardClass]]                       # Card
[[cardHeaderClass]]                 # Card header
[[cardBodyClass]]                   # Card body
```

### Define Named Templates

Use `define` for reusable blocks:

```html
[[define "mycomponent"]]
  <div class="[[cardClass]]">
    [[.Content]]
  </div>
[[end]]
```

### Include Other Templates

```html
[[template "header" .]]
<main>
  [[.Content]]
</main>
[[template "footer" .]]
```

### Conditional Rendering

```html
[[if .IsAdmin]]
  <button class="[[buttonClass "danger"]]">Delete</button>
[[else]]
  <span>Read-only mode</span>
[[end]]
```

### Range Over Arrays

```html
<ul>
[[range .Items]]
  <li>[[.Name]]: [[.Value]]</li>
[[end]]
</ul>
```

### With Context

```html
[[with .User]]
  <p>Welcome, [[.Name]]!</p>
[[end]]
```

### Template Functions

```html
<!-- dict: Create a map -->
[[template "alert" dict "Message" "Hello" "Type" "info"]]

<!-- add: Add numbers -->
<p>Total: [[add .Price .Tax]]</p>

<!-- until: Generate range -->
[[range until 5]]
  <span>[[.]]</span>
[[end]]
```

---

## Testing Components

### Manual Testing with Serve

```bash
cd ~/.lvt/components/mycomponent
lvt serve
```

Features:
- JSON editor for test data
- Live preview with any kit
- Auto-reload on template changes
- Error display

### Test Data Examples

Create test data in the JSON editor:

```json
{
  "Title": "User Profile",
  "Fields": [
    {"Name": "name", "Label": "Name", "Type": "text", "Value": "John Doe"},
    {"Name": "email", "Label": "Email", "Type": "email", "Value": "john@example.com"}
  ],
  "Actions": [
    {"Label": "Save", "URL": "/save", "Class": "primary"},
    {"Label": "Cancel", "URL": "/cancel", "Class": "secondary"}
  ]
}
```

### Test with Different Kits

The development server allows switching kits to verify your component works with:
- Tailwind CSS
- Bulma
- Pico CSS
- None (plain HTML)

### Integration Testing

Test your component in a real app:

```bash
# Create test app
lvt new testapp --css tailwind
cd testapp

# Generate resource using your component
# (Requires modifying generator or manual template editing)

# Run app
lvt serve
```

---

## Validation

Validate your component before publishing:

```bash
lvt components validate ~/.lvt/components/mycomponent
```

### Validation Checks

The validator checks:

1. **Structure**
   - component.yaml exists
   - At least one .tmpl file exists
   - README.md exists (warning if missing)

2. **Manifest**
   - Valid YAML syntax
   - Required fields present (name, version, description)
   - Valid version format (semver)
   - Inputs have required fields
   - Templates list matches actual files

3. **Templates**
   - Valid Go template syntax
   - Uses [[ ]] delimiters
   - No syntax errors

4. **Documentation**
   - README.md exists
   - Contains usage examples (recommended)

### Fix Common Issues

**Invalid template syntax:**
```bash
# Error: template: component.tmpl:5: unexpected "}"
# Fix: Check line 5 for mismatched brackets
```

**Missing required field:**
```bash
# Error: manifest missing required field "version"
# Fix: Add version field to component.yaml
```

**Template file not found:**
```bash
# Error: template file "missing.tmpl" listed but not found
# Fix: Remove from templates list or create the file
```

---

## Best Practices

### 1. Keep Components Focused

Each component should do one thing well:

```
Good: form-field (single field with label and validation)
Bad:  admin-dashboard (entire page layout)
```

### 2. Document Inputs Clearly

```yaml
inputs:
  - name: Items
    type: array
    description: Array of items, each with {Name: string, URL: string, Icon: string}
    required: true
```

### 3. Provide Default Values

```yaml
inputs:
  - name: ShowHeader
    type: bool
    description: Whether to show the header
    default: true
```

### 4. Use Semantic HTML

```html
<!-- Good -->
<nav class="[[navClass]]" aria-label="Main navigation">
  <ul>
    <li><a href="/">Home</a></li>
  </ul>
</nav>

<!-- Bad -->
<div class="[[navClass]]">
  <div><a href="/">Home</a></div>
</div>
```

### 5. Add ARIA Labels

```html
<button class="[[buttonClass "danger"]]" aria-label="Delete item">
  🗑️
</button>
```

### 6. Handle Empty States

```html
[[if .Items]]
  <ul>
  [[range .Items]]
    <li>[[.Name]]</li>
  [[end]]
  </ul>
[[else]]
  <p class="text-muted">No items to display</p>
[[end]]
```

### 7. Include Examples in README

```markdown
## Examples

### Basic Usage
\`\`\`go
data := map[string]interface{}{
    "Title": "Products",
    "Items": []map[string]string{
        {"Name": "Item 1"},
        {"Name": "Item 2"},
    },
}
\`\`\`

### Advanced Usage
\`\`\`go
// Complex example with all options
\`\`\`
```

### 8. Version Appropriately

Use semantic versioning:
- `1.0.0` → `1.0.1`: Bug fixes
- `1.0.0` → `1.1.0`: New features (backward compatible)
- `1.0.0` → `2.0.0`: Breaking changes

---

## Publishing Components

### Option 1: Local Directory

Share components by placing them in a directory:

```bash
# Users add your directory to their config
lvt config set components_paths /path/to/your/components
```

### Option 2: Git Repository

Host components in a Git repository:

```
your-components-repo/
├── alert/
│   ├── component.yaml
│   ├── alert.tmpl
│   └── README.md
├── modal/
│   ├── component.yaml
│   ├── modal.tmpl
│   └── README.md
└── README.md
```

Users can clone and configure:

```bash
git clone https://github.com/you/lvt-components.git
lvt config set components_paths ~/lvt-components
```

### Option 3: Community Registry (Future)

A central component registry is planned for future releases.

### Licensing

Include a LICENSE file:

```
mycomponent/
├── component.yaml
├── mycomponent.tmpl
├── README.md
└── LICENSE          # MIT, Apache 2.0, etc.
```

---

## Advanced Topics

### Multi-Template Components

Components can have multiple template files:

```yaml
templates:
  - card.tmpl
  - card-header.tmpl
  - card-body.tmpl
  - card-footer.tmpl
```

```html
<!-- card.tmpl -->
[[define "card"]]
<div class="[[cardClass]]">
  [[template "card-header" .]]
  [[template "card-body" .]]
  [[template "card-footer" .]]
</div>
[[end]]

<!-- card-header.tmpl -->
[[define "card-header"]]
[[if .Title]]
<div class="[[cardHeaderClass]]">
  <h3>[[.Title]]</h3>
</div>
[[end]]
[[end]]
```

### Component Dependencies

If your component requires another component:

```yaml
dependencies:
  - layout
  - button
```

Users must have these components available.

### Complex Input Types

```yaml
inputs:
  - name: Columns
    type: array
    description: |
      Array of column definitions, each object has:
      - Field: string (field name)
      - Label: string (display label)
      - Type: string (text, number, date, bool)
      - Sortable: bool (optional, default false)
```

---

## Examples

### Pagination Component

```yaml
name: pagination
version: 1.0.0
description: Page navigation with prev/next and page numbers
category: navigation

inputs:
  - name: CurrentPage
    type: int
    required: true
  - name: TotalPages
    type: int
    required: true
  - name: BaseURL
    type: string
    required: true

templates:
  - pagination.tmpl
```

```html
[[define "pagination"]]
<nav class="[[paginationClass]]" aria-label="Pagination">
  [[if gt .CurrentPage 1]]
  <a href="[[.BaseURL]]?page=[[add .CurrentPage -1]]" class="[[buttonClass "secondary"]]">
    ← Previous
  </a>
  [[end]]

  <span>Page [[.CurrentPage]] of [[.TotalPages]]</span>

  [[if lt .CurrentPage .TotalPages]]
  <a href="[[.BaseURL]]?page=[[add .CurrentPage 1]]" class="[[buttonClass "secondary"]]">
    Next →
  </a>
  [[end]]
</nav>
[[end]]
```

---

## Troubleshooting

### Template not rendering

Check if you've defined the template:
```html
[[define "mycomponent"]]
  <!-- content -->
[[end]]
```

### Kit helpers not working

Ensure kit is loaded and helpers are injected:
```go
tmpl := template.New("page").Funcs(kit.Helpers.TemplateFuncs())
```

### Validation errors

Run validation to see specific issues:
```bash
lvt components validate /path/to/component
```

---

## Next Steps

- Create a kit for custom styling: [Kit Development Guide](kit-development.md)
- Learn about `lvt serve`: [Serve Guide](serve-guide.md)
- See complete API reference: [API Reference](api-reference.md)

---

Last updated: 2025-10-17
