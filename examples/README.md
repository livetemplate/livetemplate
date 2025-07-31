# State Template Examples

This directory contains three examples demonstrating different aspects of the state template system:

## 📁 Examples Overview

### 1. `simple/` - Basic Template Tracking
**File:** `examples/simple/main.go`

Demonstrates the core functionality:
- Creating multiple templates with data dependencies
- Setting up live data update channels
- Automatic change detection and selective re-rendering
- Basic template dependency tracking

**Key Features:**
- Multiple templates (header, sidebar, user-profile)
- Channel-based data updates
- Real-time change notifications
- Dependency-based selective updates

### 2. `files/` - File-Based Template Loading
**File:** `examples/files/main.go`

Shows how to work with template files:
- Loading templates from a directory
- Loading specific template files with custom names
- Template file creation and cleanup
- Directory-based template management

**Key Features:**
- `LoadTemplatesFromDirectory()` for bulk loading
- `LoadTemplateFromFile()` for specific files
- Automatic file extension filtering
- Custom template naming

### 3. `fragments/` - Automatic Fragment Extraction
**File:** `examples/fragments/main.go`

Demonstrates the most advanced feature:
- Automatic extraction of minimal template fragments
- Smart dependency analysis per fragment
- Fragment-level change detection
- Optimized partial re-rendering

**Key Features:**
- `AddTemplateWithFragmentExtraction()` for automatic processing
- Fragment dependency tracking
- Granular update notifications
- Minimal re-rendering optimization

## 🚀 Running the Examples

```bash
# Run each example from the project root:
go run examples/simple/main.go
go run examples/files/main.go
go run examples/fragments/main.go
```

## 📊 What Each Example Shows

| Example | Templates | Data Changes | Key Benefit |
|---------|-----------|--------------|-------------|
| Simple | 3 named templates | 4 different updates | Basic dependency tracking |
| Files | 6 templates (3 from dir, 3 from files) | 4 targeted updates | File-based workflow |
| Fragments | 1 template → 12 auto-extracted fragments | 4 granular updates | Maximum optimization |

## 🎯 Use Cases

- **Simple**: Getting started, understanding the core concepts
- **Files**: Production workflows with template files
- **Fragments**: High-performance applications requiring minimal DOM updates

Each example includes detailed logging to show exactly what's happening during template processing and data updates.
