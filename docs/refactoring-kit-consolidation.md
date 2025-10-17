# Kit Consolidation Refactoring

**Branch**: `feature/components-library`
**Status**: 🔄 In Progress
**Started**: 2025-10-17
**Current Phase**: Phase 1 (Setup)

---

## Overview

### Goal
Consolidate three separate commands (`lvt template`, `lvt kits`, `lvt components`) into a single unified `lvt kits` command that provides complete starter packages.

### Architecture
- **Before**: Kits (CSS only) + Components (separate) + Templates (separate)
- **After**: Kits = CSS adapter + Components + Templates (complete package)

### Strategy
⚠️ **NO BACKWARD COMPATIBILITY** - This is an unreleased library
- Aggressive deletion of redundant code
- Clean slate implementation
- No fallback paths needed

---

## Session Plan

### Session 1: Core Infrastructure (Phases 1-2)
- ✅ Create progress document
- [ ] Update kit types (add Components, Templates)
- [ ] Update kit manifest loading
- [ ] Delete `cmd/lvt/internal/components/` package (~500 lines)
- [ ] Enhance kit loader with component/template methods

**Deliverable**: Core kits infrastructure ready
**Expected**: Tests will fail (normal)

### Session 2: Commands & Generator (Phases 3-4)
- [ ] Delete `cmd/lvt/commands/template.go`
- [ ] Delete `cmd/lvt/commands/components.go`
- [ ] Rewrite `cmd/lvt/commands/kits.go` (add `customize` command)
- [ ] Update `cmd/lvt/internal/generator/` to use kit loader
- [ ] Delete old template loader

**Deliverable**: CLI commands work with new kit system

### Session 3: Serve & System Kits (Phases 5-6)
- [ ] Update serve command integration
- [ ] Create 4 embedded kits (tailwind, bulma, pico, html)
- [ ] Use `//go:embed` for system kits

**Deliverable**: Complete system with embedded kits

### Session 4: Tests & Cleanup (Phases 7-8)
- [ ] Fix all failing tests
- [ ] Update documentation
- [ ] Remove dead code
- [ ] Final verification

**Deliverable**: ✅ All tests pass, ready to merge

---

## Detailed Phase Breakdown

### Phase 1: Setup ✅
- [x] Create this progress document
- [x] Update todo list
- [ ] Initial commit

### Phase 2: Core Infrastructure Refactoring
**Files to Modify**:
- [ ] `cmd/lvt/internal/kits/types.go` - Add Components, Templates to KitManifest
- [ ] `cmd/lvt/internal/kits/manifest.go` - Update parsing and validation
- [ ] `cmd/lvt/internal/kits/loader.go` - Add LoadKitComponent, LoadKitTemplate methods

**Files to Delete**:
- [ ] `cmd/lvt/internal/components/` (entire directory)
  - `errors.go`
  - `manifest.go`
  - `helpers.go`
  - `embed.go`
  - `types.go`
  - `loader.go`
  - `loader_test.go`

**Expected Changes**: ~+150 lines, ~-500 lines

### Phase 3: Command Refactoring
**Files to Delete**:
- [ ] `cmd/lvt/commands/template.go` (~113 lines)
- [ ] `cmd/lvt/commands/components.go` (~379 lines)

**Files to Modify**:
- [ ] `cmd/lvt/commands/kits.go` - Add `customize` subcommand
- [ ] `cmd/lvt/main.go` - Remove template/components command registration

**New Commands**:
- `lvt kits customize <name>` - Copy kit to `.lvt/kits/`
- `lvt kits customize <name> --global` - Copy to `~/.config/lvt/kits/`
- `lvt kits customize <name> --only components` - Copy only components
- `lvt kits customize <name> --only templates` - Copy only templates

### Phase 4: Generator Integration
**Files to Modify**:
- [ ] `cmd/lvt/internal/generator/resource.go` - Use kit loader
- [ ] `cmd/lvt/internal/generator/view.go` - Use kit loader
- [ ] `cmd/lvt/internal/generator/app.go` - Use kit loader

**Files to Delete**:
- [ ] `cmd/lvt/internal/generator/template_loader.go` - No longer needed

**Changes**: Replace direct template loading with kit-based loading

### Phase 5: Serve Command Integration
**Files to Modify**:
- [ ] `cmd/lvt/internal/serve/component_mode.go` - Use kit loader
- [ ] `cmd/lvt/internal/serve/kit_mode.go` - Use kit loader
- [ ] `cmd/lvt/internal/serve/helpers.go` - Simplify

### Phase 6: System Kits Creation
**New Directory Structure**:
```
cmd/lvt/internal/kits/system/
├── tailwind-basic/
│   ├── kit.yaml
│   ├── components/
│   │   ├── form.tmpl
│   │   ├── table.tmpl
│   │   ├── toolbar.tmpl
│   │   ├── pagination.tmpl
│   │   ├── detail.tmpl
│   │   ├── layout.tmpl
│   │   ├── search.tmpl
│   │   ├── stats.tmpl
│   │   └── sort.tmpl
│   └── templates/
│       ├── resource/
│       ├── view/
│       └── app/
├── bulma-basic/
├── pico-basic/
└── html-basic/
```

**Files to Modify**:
- [ ] `cmd/lvt/internal/kits/embed.go` - Add `//go:embed system/*`

### Phase 7: Testing
- [ ] Update `cmd/lvt/internal/kits/loader_test.go`
- [ ] Add tests for component/template loading
- [ ] Add tests for cascade priority
- [ ] Ensure all existing tests pass

### Phase 8: Documentation
- [ ] Update main README.md
- [ ] Remove references to `lvt template` and `lvt components`
- [ ] Add kit customization guide
- [ ] Update CLI help text

---

## Progress Tracking

### Completed
- ✅ Progress document created
- ✅ Todo list updated

### In Progress
- 🔄 Phase 1: Setup

### Next Up
- 📋 Phase 2: Core infrastructure refactoring

---

## File Change Summary

| Status | File | Lines Changed | Description |
|--------|------|---------------|-------------|
| ✅ | `docs/refactoring-kit-consolidation.md` | +0 | This file |
| ⏳ | (In progress) | - | - |

---

## Test Status

| Package | Status | Notes |
|---------|--------|-------|
| `cmd/lvt/internal/kits` | ⏳ Not yet tested | Will test after Phase 2 |
| `cmd/lvt/internal/generator` | ⏳ Not yet tested | - |
| `cmd/lvt` | ⏳ Not yet tested | - |

---

## Handoff Template

### Session End Summary
```
✅ Completed: [Phase X - Description]
📁 Files Modified: X files
📁 Files Deleted: X files (~Y lines removed)
📊 Lines: +X -Y
⚠️ Test Status: [Expected failures / All pass]
📋 Next Session: [Phase X - What to do next]
💬 Notes: [Any important context]
```

### Session Start Command
To continue in next session:
```
"Continue kit consolidation refactoring. Read docs/refactoring-kit-consolidation.md for current progress."
```

---

## Architecture Reference

### New Kit Manifest Structure
```yaml
name: tailwind-basic
version: 1.0.0
description: Basic Tailwind CSS starter kit
framework: tailwind
cdn: https://cdn.tailwindcss.com
author: LiveTemplate Team
tags: [css, utility-first]

components:
  - form
  - table
  - toolbar
  - pagination
  - detail
  - layout
  - search
  - stats
  - sort

templates:
  resource: true
  view: true
  app: true
```

### Cascade Loading Priority
1. **Project**: `.lvt/kits/<name>/` (highest priority)
2. **User**: `~/.config/lvt/kits/<name>/`
3. **System**: Embedded kits (fallback)

### Kit Loader API
```go
// Load complete kit
kit, err := loader.Load("tailwind")

// Load specific component from kit
component, err := loader.LoadKitComponent("tailwind", "form.tmpl")

// Load specific template from kit
template, err := loader.LoadKitTemplate("tailwind", "resource/handler.go.tmpl")
```

---

**Last Updated**: 2025-10-17
**Next Review**: After Session 1 completion
