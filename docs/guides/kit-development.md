# Kit Development Guide

This guide teaches you how to create custom CSS kits for LiveTemplate. Kits provide the styling layer that makes components look beautiful with different CSS frameworks.

## Table of Contents

- [Overview](#overview)
- [Kit Structure](#kit-structure)
- [Creating Your First Kit](#creating-your-first-kit)
- [Kit Manifest Reference](#kit-manifest-reference)
- [Implementing Helpers](#implementing-helpers)
- [Styling Guidelines](#styling-guidelines)
- [Testing Kits](#testing-kits)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Publishing Kits](#publishing-kits)

---

## Overview

Kits are CSS framework integrations that provide a unified helper interface for LiveTemplate components. Each kit implements the `CSSHelpers` interface with ~60 methods that return appropriate CSS classes.

### Kit Components

1. **kit.yaml** - Manifest describing the kit
2. **helpers.go** - Go code implementing the CSSHelpers interface (for system kits)
3. **components/** - (Optional) Component templates specific to this kit
4. **templates/** - (Optional) Resource, view, and app templates
5. **README.md** - Documentation for users
6. **assets/** - (Optional) Custom CSS, fonts, JavaScript
7. **LICENSE** - (Optional) License file

**Note**: Kits can be CSS-only (just helpers.go) or complete packages including components and templates. System kits (Tailwind, Bulma, Pico, None) include both.

### Supported Frameworks

You can create kits for any CSS framework:
- Utility-first (Tailwind, UnoCSS, Windi CSS)
- Component-based (Bootstrap, Bulma, Foundation)
- Minimal (Pico, Simple.css, MVP.css)
- Custom CSS

---

## Kit Structure

```
mykit/
├── kit.yaml               # Manifest (required)
├── helpers.go             # Helper implementation (optional, for Go-based helpers)
├── components/            # Component templates (optional)
│   ├── form.tmpl
│   ├── table.tmpl
│   ├── layout.tmpl
│   └── ...
├── templates/             # Generator templates (optional)
│   ├── resource/
│   │   ├── handler.go.tmpl
│   │   └── template.tmpl
│   ├── view/
│   │   └── view.tmpl
│   └── app/
│       └── main.go.tmpl
├── README.md              # Documentation (recommended)
├── assets/                # Optional assets
│   ├── custom.css
│   └── icons.woff2
└── LICENSE                # License (optional)
```

### Required Files

- **kit.yaml**: Kit metadata (always required)

### Optional Files

- **helpers.go**: CSSHelpers interface implementation (for system kits with Go helpers)
- **components/**: Component template files (.tmpl) specific to this kit
- **templates/**: Resource, view, and app generator templates
- **README.md**: Usage documentation (recommended)
- **assets/**: Framework assets (if not using CDN)

### Kit Types

1. **CSS-Only Kit**: Just helpers.go with CSS class mappings
2. **Components Kit**: Includes helpers.go + components/
3. **Complete Kit**: Includes helpers.go + components/ + templates/ (like system kits)

---

## Creating Your First Kit

### Step 1: Generate Boilerplate

```bash
lvt kits create bootstrap
```

This creates:
```
~/.lvt/kits/bootstrap/
├── kit.yaml
├── helpers.go
└── README.md
```

### Step 2: Edit kit.yaml

```yaml
name: bootstrap
version: 1.0.0
description: Bootstrap 5 CSS framework integration
framework: bootstrap
author: Your Name
license: MIT

cdn: https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css

components:
  - form.tmpl
  - table.tmpl
  - layout.tmpl

templates:
  resource: true
  view: true
  app: true

tags:
  - css
  - framework
  - bootstrap
  - responsive
```

### Step 3: Implement Helpers

Edit `helpers.go`:

```go
package bootstrap

import (
    "fmt"
    "github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

type Helpers struct{}

// Ensure Helpers implements CSSHelpers interface
var _ kits.CSSHelpers = (*Helpers)(nil)

// CSSCDN returns the CDN link for Bootstrap
func (h *Helpers) CSSCDN() string {
    return "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css"
}

// ContainerClass returns Bootstrap container class
func (h *Helpers) ContainerClass() string {
    return "container"
}

// SectionClass returns Bootstrap section class
func (h *Helpers) SectionClass() string {
    return "my-5"
}

// ButtonClass returns Bootstrap button classes
func (h *Helpers) ButtonClass(variant string) string {
    switch variant {
    case "primary":
        return "btn btn-primary"
    case "secondary":
        return "btn btn-secondary"
    case "success":
        return "btn btn-success"
    case "danger":
        return "btn btn-danger"
    case "warning":
        return "btn btn-warning"
    case "info":
        return "btn btn-info"
    default:
        return "btn btn-primary"
    }
}

// InputClass returns Bootstrap input class
func (h *Helpers) InputClass() string {
    return "form-control"
}

// FieldClass returns Bootstrap form field wrapper class
func (h *Helpers) FieldClass() string {
    return "mb-3"
}

// LabelClass returns Bootstrap label class
func (h *Helpers) LabelClass() string {
    return "form-label"
}

// TableClass returns Bootstrap table classes
func (h *Helpers) TableClass() string {
    return "table table-striped table-hover"
}

// CardClass returns Bootstrap card class
func (h *Helpers) CardClass() string {
    return "card"
}

// CardHeaderClass returns Bootstrap card header class
func (h *Helpers) CardHeaderClass() string {
    return "card-header"
}

// CardBodyClass returns Bootstrap card body class
func (h *Helpers) CardBodyClass() string {
    return "card-body"
}

// ... implement remaining ~50 methods
// See cmd/lvt/internal/kits/interface.go for complete list
```

### Step 4: Complete All Required Methods

Your helpers.go must implement all methods from the CSSHelpers interface. Use the generated boilerplate as a starting point - it includes stubs for all required methods.

### Step 5: Update README

```markdown
# Bootstrap Kit

Bootstrap 5 CSS framework integration for LiveTemplate.

## Features

- Bootstrap 5.3.0 classes
- Responsive grid system
- Modern components
- Utility classes
- Icons support (optional)

## Installation

\`\`\`bash
lvt config set kits_paths ~/.lvt/kits
\`\`\`

## Usage

\`\`\`bash
# Create app with Bootstrap
lvt new myapp --kit mykit  # Where mykit defines bootstrap as css_framework

# Generate resource with Bootstrap
lvt gen products name price  # Uses kit's CSS framework
\`\`\`

## CDN

Uses Bootstrap CDN by default:
\`\`\`
https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css
\`\`\`

## Customization

To use custom Bootstrap build, modify the CSSCDN() method in helpers.go.
```

### Step 6: Validate Kit

```bash
lvt kits validate ~/.lvt/kits/bootstrap
```

Expected output:
```
Validating kit at: /Users/you/.lvt/kits/bootstrap

✅ Kit structure is valid
✅ Manifest is valid
✅ Helpers code compiles
✅ Interface implementation is complete
✅ Documentation is present

Kit validation passed!
```

### Step 7: Test with Development Server

```bash
cd ~/.lvt/kits/bootstrap
lvt serve
```

This opens a showcase displaying:
- All helper methods and their output
- Live examples of each CSS class
- Component previews styled with your kit

---

## Kit Manifest Reference

### Basic Structure

```yaml
# Required fields
name: kit-name                  # Lowercase, alphanumeric + hyphens
version: 1.0.0                  # Semantic versioning
description: Brief description of the CSS framework

# Framework information
framework: framework-name       # Framework identifier (e.g., "tailwind", "bulma")
author: Author Name             # Kit author (optional)
license: MIT                    # License (optional)

# CDN link (optional but recommended)
cdn: https://cdn.example.com/framework.min.css

# Components included in this kit (optional)
components:
  - form.tmpl
  - table.tmpl
  - layout.tmpl

# Templates included in this kit (optional)
templates:
  resource: true                # Includes resource templates
  view: true                    # Includes view templates
  app: true                     # Includes app templates

# Categorization
tags:
  - css
  - framework
  - responsive
  - modern
```

### Example: Tailwind Kit

```yaml
name: tailwind
version: 1.0.0
description: Tailwind CSS utility-first framework starter kit
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
  - responsive
```

---

## Implementing Helpers

### CSSHelpers Interface

All kits must implement this interface (see `cmd/lvt/internal/kits/interface.go`):

```go
type CSSHelpers interface {
    // Core
    CSSCDN() string

    // Layout
    ContainerClass() string
    SectionClass() string
    ColumnClass() string
    ColumnsClass() string

    // Forms
    FieldClass() string
    LabelClass() string
    InputClass() string
    TextareaClass() string
    SelectClass() string
    CheckboxClass() string
    CheckboxLabelClass() string

    // Buttons
    ButtonClass(variant string) string
    ButtonGroupClass() string

    // Tables
    TableClass() string
    TableHeaderClass() string
    TableBodyClass() string
    TableRowClass() string
    TableCellClass() string

    // Cards
    CardClass() string
    CardHeaderClass() string
    CardBodyClass() string
    CardFooterClass() string

    // Navigation
    NavClass() string
    NavItemClass() string
    NavLinkClass() string

    // Utilities
    TextClass(size string) string
    ColorClass(color string) string
    BackgroundClass(color string) string
    SpacingClass(type, size string) string

    // Template Functions
    Dict(values ...interface{}) map[string]interface{}
    Until(count int) []int
    Add(a, b int) int

    // ... ~60 methods total
}
```

### Method Categories

**Layout Helpers** (10 methods)
- Container, section, columns, grids, boxes, wrappers

**Form Helpers** (15 methods)
- Fields, labels, inputs, selects, checkboxes, radios, validation

**Button Helpers** (5 methods)
- Button variants, groups, sizes, states

**Table Helpers** (10 methods)
- Table, rows, cells, headers, footers, striping

**Card Helpers** (6 methods)
- Card containers, headers, bodies, footers

**Navigation Helpers** (8 methods)
- Nav bars, items, links, tabs, breadcrumbs

**Typography Helpers** (6 methods)
- Headings, text sizes, colors, weights

**Utility Helpers** (10 methods)
- Spacing, colors, backgrounds, borders, shadows

**Template Functions** (10 methods)
- dict, add, until, and other template utilities

### Helper Implementation Patterns

#### Simple Class Return

```go
func (h *Helpers) ContainerClass() string {
    return "container"
}
```

#### Variant-Based

```go
func (h *Helpers) ButtonClass(variant string) string {
    switch variant {
    case "primary":
        return "btn btn-primary"
    case "secondary":
        return "btn btn-secondary"
    case "danger":
        return "btn btn-danger"
    default:
        return "btn"
    }
}
```

#### Size-Based

```go
func (h *Helpers) TextClass(size string) string {
    switch size {
    case "xs":
        return "text-xs"
    case "sm":
        return "text-sm"
    case "lg":
        return "text-lg"
    case "xl":
        return "text-xl"
    default:
        return "text-base"
    }
}
```

#### Multiple Parameters

```go
func (h *Helpers) SpacingClass(type, size string) string {
    prefix := map[string]string{
        "margin":  "m",
        "padding": "p",
    }[type]

    return fmt.Sprintf("%s-%s", prefix, size)
}
```

#### Template Utilities

```go
func (h *Helpers) Dict(values ...interface{}) map[string]interface{} {
    dict := make(map[string]interface{})
    for i := 0; i < len(values); i += 2 {
        key := values[i].(string)
        dict[key] = values[i+1]
    }
    return dict
}

func (h *Helpers) Until(count int) []int {
    result := make([]int, count)
    for i := 0; i < count; i++ {
        result[i] = i + 1
    }
    return result
}

func (h *Helpers) Add(a, b int) int {
    return a + b
}
```

---

## Styling Guidelines

### 1. Follow Framework Conventions

Use the framework's official class naming:

```go
// Bootstrap - correct
func (h *Helpers) ButtonClass(variant string) string {
    return "btn btn-" + variant
}

// Bootstrap - incorrect (not following conventions)
func (h *Helpers) ButtonClass(variant string) string {
    return "button button-" + variant
}
```

### 2. Support Common Variants

```go
func (h *Helpers) ButtonClass(variant string) string {
    switch variant {
    case "primary", "secondary", "success", "danger", "warning", "info":
        return "btn btn-" + variant
    default:
        return "btn btn-primary"
    }
}
```

### 3. Provide Sensible Defaults

```go
func (h *Helpers) TableClass() string {
    // Include commonly needed classes
    return "table table-striped table-hover table-bordered"
}
```

### 4. Consider Accessibility

```go
func (h *Helpers) ButtonClass(variant string) string {
    // Include focus states, ARIA-friendly classes
    return "btn btn-" + variant + " focus:outline focus:ring"
}
```

### 5. Mobile-First Responsive

```go
func (h *Helpers) ContainerClass() string {
    // Responsive container
    return "container mx-auto px-4 sm:px-6 lg:px-8"
}
```

---

## Testing Kits

### Manual Testing with Serve

```bash
cd ~/.lvt/kits/mykit
lvt serve
```

The kit development server shows:
- Kit information
- All helper methods and their outputs
- Live CSS class examples
- Component previews using your kit

### Test with Components

```bash
# Create test component
lvt components create test-card --category data

# Edit test-card.tmpl to use kit helpers
# Start component dev server
cd ~/.lvt/components/test-card
lvt serve

# Select your kit from dropdown
# Verify component renders correctly
```

### Integration Testing

```bash
# Create test app with your kit
lvt new testapp --kit mykit  # CSS framework defined in mykit/kit.yaml
cd testapp

# Generate resource
lvt gen products name price  # Uses mykit's CSS framework

# Run app
lvt serve

# Test in browser
```

### Validation

```bash
lvt kits validate ~/.lvt/kits/mykit
```

Checks:
- Go code compiles
- All interface methods implemented
- Method signatures match interface
- Package structure is correct

---

## Validation

### Run Validation

```bash
lvt kits validate ~/.lvt/kits/mykit
```

### Validation Checks

1. **Structure**
   - kit.yaml exists
   - helpers.go exists
   - README.md exists (warning if missing)

2. **Manifest**
   - Valid YAML syntax
   - Required fields (name, version, description)
   - Valid semver version

3. **Helpers Code**
   - Valid Go syntax (compiles)
   - Package declaration correct
   - Imports valid

4. **Interface Implementation**
   - All CSSHelpers methods present
   - Correct method signatures
   - Implements kits.CSSHelpers interface

### Common Validation Errors

**Missing method:**
```bash
# Error: helpers.go missing method "CardClass"
# Fix: Add the method to helpers.go
```

**Wrong signature:**
```bash
# Error: ButtonClass has wrong signature, expected ButtonClass(variant string) string
# Fix: Update method signature to match interface
```

**Compilation error:**
```bash
# Error: helpers.go:45:2: undefined: fmt
# Fix: Add missing import "fmt"
```

---

## Best Practices

### 1. Study the Framework First

Before creating a kit, thoroughly understand the framework's:
- Class naming conventions
- Component structure
- Utility patterns
- Responsive approach

### 2. Match Framework Patterns

```go
// Tailwind - utility-based
func (h *Helpers) ButtonClass(variant string) string {
    base := "px-4 py-2 rounded font-medium"
    colors := map[string]string{
        "primary":   "bg-blue-500 text-white hover:bg-blue-600",
        "secondary": "bg-gray-500 text-white hover:bg-gray-600",
    }
    return base + " " + colors[variant]
}

// Bootstrap - component-based
func (h *Helpers) ButtonClass(variant string) string {
    return "btn btn-" + variant
}
```

### 3. Test All Helper Methods

```go
// Create test table
func TestHelpers(t *testing.T) {
    h := &Helpers{}

    tests := []struct{
        name     string
        method   string
        args     []interface{}
        expected string
    }{
        {"Container", "ContainerClass", nil, "container"},
        {"Button Primary", "ButtonClass", []interface{}{"primary"}, "btn btn-primary"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 4. Document Special Cases

```markdown
## Special Notes

### Dark Mode
This kit supports dark mode via the `dark:` prefix:
\`\`\`html
<div class="[[backgroundClass "white"]] dark:bg-gray-800">
\`\`\`

### Icons
For icons, use Bootstrap Icons:
\`\`\`html
<i class="bi bi-heart"></i>
\`\`\`
```

### 5. Provide CDN and Local Options

```go
func (h *Helpers) CSSCDN() string {
    // Allow override via environment variable
    if custom := os.Getenv("BOOTSTRAP_CDN"); custom != "" {
        return custom
    }
    return "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css"
}
```

### 6. Version Appropriately

- `1.0.0` → `1.0.1`: CSS class fixes
- `1.0.0` → `1.1.0`: New helper methods (backward compatible)
- `1.0.0` → `2.0.0`: Breaking class changes

---

## Publishing Kits

### Option 1: Local Directory

```bash
# Users add your directory
lvt config set kits_paths /path/to/your/kits
```

### Option 2: Git Repository

```
your-kits-repo/
├── bootstrap/
│   ├── kit.yaml
│   ├── helpers.go
│   └── README.md
├── foundation/
│   ├── kit.yaml
│   ├── helpers.go
│   └── README.md
└── README.md
```

```bash
git clone https://github.com/you/lvt-kits.git
lvt config set kits_paths ~/lvt-kits
```

### Licensing

```
mykit/
├── kit.yaml
├── helpers.go
├── README.md
└── LICENSE
```

---

## Advanced Topics

### Custom CSS

Include custom CSS in assets/:

```
mykit/
├── kit.yaml
├── helpers.go
├── assets/
│   ├── custom.css
│   └── overrides.css
└── README.md
```

```go
func (h *Helpers) CSSCDN() string {
    // Return path to local CSS
    return "/static/mykit/custom.css"
}
```

### JavaScript Dependencies

Some frameworks need JavaScript:

```yaml
# kit.yaml
framework:
  name: Bootstrap
  version: 5.3.0
  js: https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js
```

### Icon Support

```go
func (h *Helpers) IconClass(name string) string {
    return "bi bi-" + name  // Bootstrap Icons
}
```

### Dark Mode Support

```go
func (h *Helpers) CardClass() string {
    return "card bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
}
```

---

## Examples

### Minimal Kit (Plain HTML)

```go
package none

type Helpers struct{}

func (h *Helpers) CSSCDN() string {
    return "" // No CDN
}

func (h *Helpers) ContainerClass() string {
    return "" // No classes
}

func (h *Helpers) ButtonClass(variant string) string {
    return "" // Plain button
}

// ... all methods return ""
```

### Utility-First Kit (Tailwind)

```go
func (h *Helpers) CardClass() string {
    return "bg-white rounded-lg shadow-md border border-gray-200"
}

func (h *Helpers) CardHeaderClass() string {
    return "px-6 py-4 border-b border-gray-200 font-semibold"
}

func (h *Helpers) CardBodyClass() string {
    return "px-6 py-4"
}
```

---

## Troubleshooting

### Helpers not compiling

```bash
# Check Go syntax
go build -C ~/.lvt/kits/mykit helpers.go

# Run validation
lvt kits validate ~/.lvt/kits/mykit
```

### Interface not satisfied

```bash
# Error: *Helpers does not implement kits.CSSHelpers (missing method CardClass)
# Fix: Add all required methods from interface
```

### Kit not found

```bash
# Check search paths
lvt config get kits_paths

# List available kits
lvt kits list

# Validate kit
lvt kits validate /path/to/kit
```

---

## Next Steps

- Create components using your kit: [Component Development Guide](component-development.md)
- Learn about `lvt serve`: [Serve Guide](serve-guide.md)
- See complete API reference: [API Reference](api-reference.md)

---

Last updated: 2025-10-17
