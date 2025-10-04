# CSS Framework Support - Implementation Progress

## Session Summary

This session successfully laid the foundation for CSS framework selection in the LiveTemplate CLI generator.

## What Was Accomplished

### ✅ Phase 1: Foundation (Complete)

1. **Data Structure Updates**
   - Added `CSSFramework` field to `ResourceData` (types.go)
   - Added `CSSFramework` field to `ViewData` (view.go)
   - Fields support: "tailwind", "bulma", "pico", "none"

2. **CSS Helper Functions** (`css_helpers.go` - 310 lines)
   - Complete abstraction layer for all 4 CSS frameworks
   - Helper functions include:
     - `csscdn()` - CDN links for each framework
     - `containerClass()` - Container/wrapper classes
     - `boxClass()` - Card/box components
     - `titleClass()`, `subtitleClass()` - Typography
     - `fieldClass()`, `labelClass()` - Form elements
     - `inputClass()`, `inputErrorClass()` - Input styling
     - `buttonClass()` - Buttons with variants (primary/danger)
     - `tableClass()`, `theadClass()`, `thClass()`, etc - Tables
     - `needsWrapper()`, `needsArticle()` - Semantic HTML checks
     - Plus 10+ more helper functions

3. **Architecture Decision**
   - ✅ Chose conditional templates over separate files
   - ✅ One template adapts to framework choice (DRY principle)
   - ✅ ~390 lines total vs ~765 with separate files (49% reduction)

## Framework Support

| Framework | Version | Type | CDN | Status |
|-----------|---------|------|-----|--------|
| **Tailwind CSS** | v4.0 | Utility-first | `@tailwindcss/browser@4` | ✅ Default |
| **Bulma** | 1.0.4 | Component | `bulma@1.0.4` | ✅ Supported |
| **Pico CSS** | v2.0 | Semantic/Classless | `@picocss/pico@2` | ✅ Supported |
| **None** | - | Plain HTML | None | ✅ Supported |

## Remaining Work

See [NEXT_STEPS.md](./NEXT_STEPS.md) for detailed implementation guide.

**High-level checklist:**
- [ ] Update `generateFile()` to merge CSS helpers (~15 min)
- [ ] Update `GenerateResource()` signature (~25 min)
- [ ] Update `GenerateView()` signature (~20 min)
- [ ] Update CLI commands to parse `--css` flag (~45 min)
- [ ] Update interactive UI for framework selection (~2 hrs)
- [ ] Rewrite templates with conditional CSS (~2-3 hrs)
- [ ] Update help text and documentation (~20 min)
- [ ] Testing and validation (~1 hr)

**Estimated remaining effort**: 6-7 hours

## Design Principles

1. **Default to Modern**: Tailwind CSS v4 as default (most popular, 2025)
2. **Flexible**: Easy to switch frameworks via flag
3. **Maintainable**: One template, multiple outputs
4. **Backward Compatible**: Existing projects continue working
5. **User-Friendly**: Interactive TUI for framework selection

## Usage (When Complete)

### Direct Mode
```bash
# Default (Tailwind)
lvt gen users name email

# Explicit framework
lvt gen users name email --css=tailwind
lvt gen users name email --css=bulma
lvt gen users name email --css=pico
lvt gen users name email --css=none

# View generation
lvt gen view counter --css=pico
```

### Interactive Mode
```bash
lvt gen

# Will show CSS framework selection screen:
# → Tailwind CSS - Utility-first, modern (default)
#   Bulma - Component-based, clean
#   Pico CSS - Semantic, minimal, classless
#   None - Pure HTML only
```

## Technical Notes

### Helper Function Pattern
```go
// In template
[[csscdn .CSSFramework]]
<div class="[[containerClass .CSSFramework]]">
<button class="[[buttonClass .CSSFramework "primary"]]">

// Generates (Tailwind)
<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
<div class="max-w-7xl mx-auto px-4 py-8">
<button class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700">

// Generates (Bulma)
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
<div class="container">
<button class="button is-primary">

// Generates (Pico)
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
<div class="container">
<button>  <!-- Pico styles automatically -->

// Generates (None)

<div>
<button>
```

### Pico CSS Special Cases
Pico CSS uses semantic HTML and automatic styling:
- Needs `<main class="container">` wrapper
- Needs `<article>` instead of div boxes
- No classes on inputs/buttons (styles automatically)
- Detected via `needsWrapper()` and `needsArticle()` helpers

## Files Created/Modified

### New Files (1)
- `cmd/lvt/internal/generator/css_helpers.go` (310 lines)

### Modified Files (2)
- `cmd/lvt/internal/generator/types.go` (+1 field)
- `cmd/lvt/internal/generator/view.go` (+1 field to ViewData)

### Documentation
- `cmd/lvt/NEXT_STEPS.md` - Detailed implementation guide
- `cmd/lvt/CSS_FRAMEWORK_PROGRESS.md` - This file

## Commit

```
feat: add CSS framework support foundation (WIP)

commit: 29530c4
```

## Next Session

When resuming work:
1. Read [NEXT_STEPS.md](./NEXT_STEPS.md)
2. Start with Step 1 (update generateFile function)
3. Work through steps 2-10 sequentially
4. Test each framework as you go
5. Update golden files with Tailwind output

## Questions/Decisions for Future

- Should we add more frameworks (Bootstrap, Foundation)?
- Should `--css` be persisted in project config?
- Should we support mixing frameworks in one project?
- Add framework-specific optimizations?

---

**Status**: Foundation complete, ready for template implementation
**Branch**: `cli`
**Next**: See NEXT_STEPS.md for continuation
