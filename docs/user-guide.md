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

LiveTemplate provides a unified **Kits** system for rapid web application development. Each kit is a complete package that includes:

1. **CSS Framework Integration** - Helpers for your chosen CSS framework
2. **Components** - Reusable UI template blocks (forms, tables, layouts, pagination, etc.)
3. **Templates** - Generator templates for resources, views, and apps

Kits are complete starter packages that include everything you need to build applications with a consistent design system.

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

## Understanding Kits

Kits are complete starter packages that include:

1. **CSS Framework Integration** - ~60 helper methods for generating CSS classes
2. **Components** - Pre-built UI template blocks (form, table, layout, pagination, etc.)
3. **Templates** - Generator templates for resources, views, and apps

This unified approach ensures consistency across your application and makes it easy to switch or customize frameworks.

### System Kits

| Kit | Framework | Includes | Description |
|-----|-----------|----------|-------------|
| **tailwind** | Tailwind CSS | CSS + Components + Templates | Utility-first CSS framework |
| **bulma** | Bulma | CSS + Components + Templates | Modern CSS framework based on Flexbox |
| **pico** | Pico CSS | CSS + Components + Templates | Minimal CSS framework for semantic HTML |
| **none** | Plain HTML | CSS + Components + Templates | No CSS framework, semantic HTML only |

Each system kit includes 9 components (form, table, layout, pagination, toolbar, detail, search, sort, stats) and complete generator templates.

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

## Customizing Kits

You can customize kits to match your project's specific needs. When you customize a kit, you copy it to your project or user directory where it can be modified.

### Customization Command

```bash
# Copy entire kit to project directory (.lvt/kits/tailwind/)
lvt kits customize tailwind

# Copy to global config for all projects (~/.config/lvt/kits/tailwind/)
lvt kits customize tailwind --global

# Copy only components
lvt kits customize tailwind --only components

# Copy only templates
lvt kits customize tailwind --only templates
```

### Customization Cascade

LiveTemplate searches for kits in this order:

1. **Project**: `.lvt/kits/<name>/` (highest priority)
2. **User**: `~/.config/lvt/kits/<name>/`
3. **System**: Embedded kits (fallback)

This allows you to:
- Override kits per-project (`.lvt/kits/`)
- Override kits globally (`~/.config/lvt/kits/`)
- Fall back to system defaults

### Example Workflow

```bash
# Start with Tailwind
lvt new myapp --css tailwind

# Customize the form component
lvt kits customize tailwind --only components
cd .lvt/kits/tailwind/components
# Edit form.tmpl to add custom fields or styling

# Regenerate with customized component
lvt gen products name price --css tailwind
# Uses your customized form.tmpl from .lvt/kits/tailwind/
```

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

### Kit Commands

```bash
# List kits
lvt kits list [--filter system|local|all] [--format table|json]

# Get kit info
lvt kits info <name>

# Create new kit
lvt kits create <name>

# Customize existing kit
lvt kits customize <name> [--global] [--only components|templates]

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
