# LiveTemplate Components & Kits - User Guide

Welcome to the LiveTemplate components and kits system! This guide will help you get started with using pre-built components and CSS kits in your Go web applications.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Understanding Components](#understanding-components)
- [Understanding Kits](#understanding-kits)
- [Using Components in Your Project](#using-components-in-your-project)
- [Using Kits in Your Project](#using-kits-in-your-project)
- [Configuration](#configuration)
- [CLI Commands Reference](#cli-commands-reference)

---

## Overview

LiveTemplate provides two main building blocks for rapid web application development:

1. **Components** - Reusable UI template blocks (forms, tables, layouts, pagination, etc.)
2. **Kits** - CSS framework integrations (Tailwind, Bulma, Pico, or plain HTML)

Components are CSS-independent, while kits provide the styling layer. This separation allows you to use any component with any kit, giving you maximum flexibility.

### Key Features

- **Pre-built Components**: Layout, forms, tables, pagination, toolbars, detail views
- **Multiple CSS Frameworks**: Tailwind, Bulma, Pico, or no framework
- **Auto-discovery**: Components and kits are automatically discovered from configured paths
- **Validation**: Built-in validation for component and kit structure
- **Development Server**: Live preview server for component and kit development
- **Extensible**: Create your own custom components and kits

---

## Installation

LiveTemplate CLI is installed as a Go binary:

```bash
go install github.com/livefir/livetemplate/cmd/lvt@latest
```

Verify installation:

```bash
lvt --help
```

---

## Quick Start

### 1. Create a new app

```bash
lvt new myapp --css tailwind
cd myapp
```

This creates a new Go web application using the Tailwind CSS kit.

### 2. Generate a resource with CRUD interface

```bash
lvt gen articles title content:text published:bool --css tailwind
```

This generates:
- Database migration
- CRUD handlers
- Template files using the form, table, and layout components
- All styled with Tailwind CSS

### 3. Run your app

```bash
lvt serve
```

The development server will:
- Auto-detect you're in an app directory
- Build and run your Go application
- Watch for file changes and auto-restart
- Proxy requests to your app with hot reload

---

## Understanding Components

Components are reusable template blocks that define UI structures. They are CSS-independent and work with any kit.

### System Components

LiveTemplate ships with these built-in components:

| Component | Description | Inputs |
|-----------|-------------|--------|
| **layout** | Page layout with head, content, scripts | Title, EditMode |
| **form** | Form with fields and validation | Resource, Fields, SubmitURL, Method |
| **table** | Data table with pagination | Resource, Rows, Fields, Actions |
| **pagination** | Page navigation controls | TotalPages, CurrentPage, BaseURL |
| **toolbar** | Action toolbar with buttons | Title, Actions, SearchURL |
| **detail** | Detail view for single record | Resource, Fields, Item |

### Listing Components

```bash
# List all components
lvt components list

# Filter by source
lvt components list --filter=system
lvt components list --filter=local

# Search by name
lvt components list --search=form

# Output as JSON
lvt components list --format=json
```

### Component Information

```bash
# Get detailed information about a component
lvt components info form

# Output shows:
# - Component name and version
# - Description
# - Inputs (data structure)
# - Templates included
# - Dependencies
# - README content
```

---

## Understanding Kits

Kits provide CSS framework integration through a unified helper interface. Each kit implements ~60 helper methods that return appropriate CSS classes for the selected framework.

### System Kits

| Kit | Framework | CDN | Description |
|-----|-----------|-----|-------------|
| **tailwind** | Tailwind CSS | ✅ | Utility-first CSS framework |
| **bulma** | Bulma | ✅ | Modern CSS framework based on Flexbox |
| **pico** | Pico CSS | ✅ | Minimal CSS framework for semantic HTML |
| **none** | Plain HTML | ❌ | No CSS framework, semantic HTML only |

### Listing Kits

```bash
# List all kits
lvt kits list

# Filter by source
lvt kits list --filter=system
lvt kits list --filter=local

# Search by name
lvt kits list --search=tailwind

# Output as JSON
lvt kits list --format=json
```

### Kit Information

```bash
# Get detailed information about a kit
lvt kits info tailwind

# Output shows:
# - Kit name and version
# - Framework name and version
# - CDN link (if available)
# - Tags
# - README content
```

---

## Using Components in Your Project

### Method 1: Using `lvt gen` (Recommended)

The easiest way to use components is through the `lvt gen` command:

```bash
# Generate a resource with CRUD UI
lvt gen products name price:float stock:int --css bulma
```

This automatically:
1. Loads the specified kit (bulma)
2. Uses system components (form, table, detail, layout)
3. Generates handler files with proper data structures
4. Generates template files using components
5. Injects kit helper functions into templates

### Method 2: Manual Template Usage

You can also use components directly in your templates:

```go
// In your handler
import "github.com/livefir/livetemplate/cmd/lvt/internal/kits"

// Load kit
kitLoader := kits.DefaultLoader()
kit, err := kitLoader.Load("tailwind")
if err != nil {
    log.Fatal(err)
}

// Load component
componentLoader := components.DefaultLoader()
component, err := componentLoader.Load("form")
if err != nil {
    log.Fatal(err)
}

// Create template with kit helpers
tmpl := template.New("page").Funcs(kit.Helpers.TemplateFuncs())
tmpl, err = tmpl.ParseFiles(component.GetTemplate("form"))
if err != nil {
    log.Fatal(err)
}

// Execute template
data := map[string]interface{}{
    "Title": "Create Product",
    "Resource": "products",
    "Fields": []Field{
        {Name: "name", Type: "text", Label: "Product Name"},
        {Name: "price", Type: "number", Label: "Price"},
    },
}
tmpl.Execute(w, data)
```

### Available Helper Functions in Templates

All kits provide these helper functions in templates:

```html
<!-- CDN link for CSS framework -->
<link rel="stylesheet" href="[[csscdn]]">

<!-- Layout helpers -->
<div class="[[containerClass]]">
  <section class="[[sectionClass]]">
    <!-- content -->
  </section>
</div>

<!-- Form helpers -->
<form>
  <div class="[[fieldClass]]">
    <label class="[[labelClass]]">Name</label>
    <input class="[[inputClass]]" type="text">
  </div>
  <button class="[[buttonClass "primary"]]">Submit</button>
</form>

<!-- Table helpers -->
<table class="[[tableClass]]">
  <thead class="[[tableHeaderClass]]">
    <tr class="[[tableRowClass]]">
      <th class="[[tableCellClass]]">Header</th>
    </tr>
  </thead>
</table>

<!-- Card helpers -->
<div class="[[cardClass]]">
  <div class="[[cardHeaderClass]]">Header</div>
  <div class="[[cardBodyClass]]">Body</div>
</div>
```

See [API Reference](api-reference.md) for complete list of helper functions.

---

## Using Kits in Your Project

### Choosing a Kit

When creating a new project or generating resources, use the `--css` flag:

```bash
# Tailwind CSS (utility-first, highly customizable)
lvt new myapp --css tailwind
lvt gen users name email --css tailwind

# Bulma (component-based, easy to learn)
lvt new myapp --css bulma
lvt gen users name email --css bulma

# Pico CSS (minimal, semantic HTML)
lvt new myapp --css pico
lvt gen users name email --css pico

# No framework (plain HTML)
lvt new myapp --css none
lvt gen users name email --css none
```

### Switching Kits

To switch kits in an existing project:

1. Update your layout template to use the new kit's CDN:
```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
```

2. Regenerate your resource templates:
```bash
lvt gen articles title content --css bulma
```

Note: This will overwrite existing templates, so back up any customizations first.

---

## Configuration

LiveTemplate stores its configuration in `~/.config/lvt/config.yaml`.

### View Configuration

```bash
# List all configuration
lvt config list

# Get specific value
lvt config get components_paths
lvt config get kits_paths
```

### Add Custom Paths

You can add paths where LiveTemplate will search for custom components and kits:

```bash
# Add component search path
lvt config set components_paths ~/.lvt/components

# Add kit search path
lvt config set kits_paths ~/.lvt/kits
```

Components and kits in these paths will be auto-discovered and available alongside system components/kits.

### Path Priority

When loading components/kits, LiveTemplate searches in this order:
1. System (embedded in lvt binary)
2. Local paths (from config)
3. Community paths (from config)

If multiple components/kits have the same name, the first one found is used.

---

## CLI Commands Reference

### App Commands

```bash
# Create new app
lvt new <name> [--css framework]

# Generate resource
lvt gen <resource> [fields...] [--css framework]

# Run development server
lvt serve [--port 3000] [--mode app|component|kit]
```

### Component Commands

```bash
# List components
lvt components list [--filter system|local|all] [--format table|json]

# Get component info
lvt components info <name>

# Create new component
lvt components create <name> [--category category]

# Validate component
lvt components validate <path>
```

### Kit Commands

```bash
# List kits
lvt kits list [--filter system|local|all] [--format table|json]

# Get kit info
lvt kits info <name>

# Create new kit
lvt kits create <name>

# Validate kit
lvt kits validate <path>
```

### Config Commands

```bash
# List all config
lvt config list

# Get config value
lvt config get <key>

# Set config value
lvt config set <key> <value>
```

---

## Next Steps

- **Component Development**: Learn to create your own components in [Component Development Guide](component-development.md)
- **Kit Development**: Learn to create your own kits in [Kit Development Guide](kit-development.md)
- **Development Server**: Deep dive into `lvt serve` in [Serve Guide](serve-guide.md)
- **API Reference**: Complete reference of all APIs in [API Reference](api-reference.md)

---

## Troubleshooting

### Component not found

```bash
# Check if component is available
lvt components list --search=mycomponent

# Check search paths
lvt config get components_paths

# Validate component
lvt components validate /path/to/component
```

### Kit not found

```bash
# Check if kit is available
lvt kits list --search=mykit

# Check search paths
lvt config get kits_paths

# Validate kit
lvt kits validate /path/to/kit
```

### Template errors

```bash
# Validate component templates
lvt components validate /path/to/component

# Check template syntax in your editor
# Templates use [[ ]] delimiters instead of {{ }}
```

---

## Support

- **Documentation**: Check the docs/ directory for detailed guides
- **Issues**: Report bugs at https://github.com/livefir/livetemplate/issues
- **Examples**: See examples/ directory for working code samples

---

Last updated: 2025-10-17
