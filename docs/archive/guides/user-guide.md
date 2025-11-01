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
lvt new myapp --kit multi  # Uses Tailwind CSS
cd myapp
```

This creates a new Go web application using the Tailwind CSS kit.

### 2. Generate a resource with CRUD interface

```bash
lvt gen articles title content:text published:bool  # Uses kit's CSS
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
lvt new myapp --kit multi  # Uses Tailwind CSS

# Customize the form component
lvt kits customize tailwind --only components
cd .lvt/kits/tailwind/components
# Edit form.tmpl to add custom fields or styling

# Regenerate with customized component
lvt gen products name price  # Uses kit's CSS
# Uses your customized form.tmpl from .lvt/kits/tailwind/
```

---

## Using Kits in Your Project

### Choosing a Kit

CSS framework is determined by your chosen kit:

- **Multi kit**: Uses Tailwind CSS
- **Single kit**: Uses Tailwind CSS
- **Simple kit**: Uses Pico CSS

```bash
# Create app with Tailwind CSS
lvt new myapp --kit multi
lvt gen users name email  # Will use Tailwind

# Create app with Pico CSS
lvt new myapp --kit simple
lvt gen users name email  # Will use Pico
```
