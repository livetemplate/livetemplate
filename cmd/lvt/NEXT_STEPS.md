# CSS Framework Support - Next Steps

## Overview
This document outlines the remaining work to complete the CSS framework selection feature. The foundation has been laid with data structures and helper functions.

## ✅ Completed

1. **Data Structures** (`types.go`, `view.go`)
   - Added `CSSFramework` field to `ResourceData` and `ViewData`

2. **CSS Helper Functions** (`css_helpers.go`)
   - Comprehensive framework abstraction with ~310 lines
   - Functions for CDN links, class names, and conditional rendering
   - Support for 4 frameworks: Tailwind, Bulma, Pico, None

## 🚧 Remaining Work

### 1. Update `generateFile()` Function (~15 lines)

**File:** `internal/generator/resource.go` (lines ~134-151)

**Current code:**
```go
func generateFile(tmplStr string, data interface{}, outPath string) error {
    tmpl, err := template.New("template").Delims("[[", "]]").Funcs(funcMap).Parse(tmplStr)
    // ... rest
}
```

**Replace with:**
```go
func generateFile(tmplStr string, data interface{}, outPath string) error {
    // Merge base funcMap with CSS helpers
    funcs := make(template.FuncMap)
    for k, v := range funcMap {
        funcs[k] = v
    }
    for k, v := range CSSHelpers() {
        funcs[k] = v
    }

    tmpl, err := template.New("template").Delims("[[", "]]").Funcs(funcs).Parse(tmplStr)
    // ... rest stays the same
}
```

### 2. Update `GenerateResource()` Signature (~25 lines)

**File:** `internal/generator/resource.go` (line ~14)

**Current signature:**
```go
func GenerateResource(basePath, moduleName, resourceName string, fields []parser.Field) error {
```

**Change to:**
```go
func GenerateResource(basePath, moduleName, resourceName string, fields []parser.Field, cssFramework string) error {
    // Default to tailwind if not specified
    if cssFramework == "" {
        cssFramework = "tailwind"
    }

    // Add to data struct (around line 36)
    data := ResourceData{
        PackageName:          resourceNameLower,
        ModuleName:           moduleName,
        ResourceName:         resourceName,
        ResourceNameLower:    resourceNameLower,
        ResourceNameSingular: resourceNameSingularCap,
        ResourceNamePlural:   resourceNamePluralCap,
        TableName:            tableName,
        Fields:               fieldData,
        CSSFramework:         cssFramework,  // ADD THIS LINE
    }

    // Rest remains unchanged
}
```

### 3. Update `GenerateView()` Signature (~20 lines)

**File:** `internal/generator/view.go` (line ~17)

**Current signature:**
```go
func GenerateView(basePath, moduleName, viewName string) error {
```

**Change to:**
```go
func GenerateView(basePath, moduleName, viewName string, cssFramework string) error {
    // Default to tailwind
    if cssFramework == "" {
        cssFramework = "tailwind"
    }

    // Add to data struct (around line 22)
    data := ViewData{
        PackageName:   viewNameLower,
        ModuleName:    moduleName,
        ViewName:      viewName,
        ViewNameLower: viewNameLower,
        CSSFramework:  cssFramework,  // ADD THIS LINE
    }

    // Rest remains unchanged
}
```

### 4. Update CLI Command (`commands/gen.go`) (~45 lines)

**File:** `commands/gen.go`

**Add flag parsing at the top of `Gen()` function:**
```go
func Gen(args []string) error {
    if len(args) < 1 {
        return fmt.Errorf("resource name required")
    }

    // Parse --css flag
    cssFramework := "tailwind" // default
    filteredArgs := []string{}

    for _, arg := range args {
        if strings.HasPrefix(arg, "--css=") {
            cssFramework = strings.TrimPrefix(arg, "--css=")
        } else {
            filteredArgs = append(filteredArgs, arg)
        }
    }

    // Validate framework
    validFrameworks := map[string]bool{
        "tailwind": true,
        "bulma":    true,
        "pico":     true,
        "none":     true,
    }
    if !validFrameworks[cssFramework] {
        return fmt.Errorf("invalid CSS framework: %s (valid: tailwind, bulma, pico, none)", cssFramework)
    }

    // Check if "view" subcommand
    if filteredArgs[0] == "view" {
        return GenView(filteredArgs[1:], cssFramework)
    }

    resourceName := filteredArgs[0]
    fieldArgs := filteredArgs[1:]

    // ... rest of code ...

    // Update generator call (around line 57):
    if err := generator.GenerateResource(basePath, moduleName, resourceName, fields, cssFramework); err != nil {
        return err
    }

    // ... rest unchanged
}
```

**Update `GenView()` signature:**
```go
func GenView(args []string, cssFramework string) error {
    // ... existing code ...

    // Update generator call:
    if err := generator.GenerateView(basePath, moduleName, viewName, cssFramework); err != nil {
        return err
    }

    // ... rest unchanged
}
```

### 5. Update Interactive UI (~100 lines total)

#### 5a. Update `ui/gen_resource.go` (~80 lines)

**Add CSS framework fields to model:**
```go
type genResourceModel struct {
    textInput       textinput.Model
    stage           int // 0: resource, 1: CSS framework, 2: fields, 3: confirm, 4: generating, 5: success
    resourceName    string
    fields          []fieldEntry
    cssFramework    string // NEW
    cssFrameworkIdx int    // NEW: 0=tailwind, 1=bulma, 2=pico, 3=none
    moduleName      string
    basePath        string
    err             error
    validationError string
    validationWarn  string
    showHelp        bool
    termWidth       int
    termHeight      int
}
```

**Update stage numbers:**
- Stage 0: Resource name
- Stage 1: **CSS framework selection (NEW)**
- Stage 2: Add fields
- Stage 3: Confirm
- Stage 4: Generating
- Stage 5: Success

**Add CSS framework selection in `Update()` method:**
```go
// After stage 0 (resource name), before field entry
case tea.KeyEnter:
    if m.stage == 0 {
        // Resource name validation...
        m.stage = 1  // Go to CSS framework selection
        return m, nil

    } else if m.stage == 1 {
        // CSS framework selected
        frameworks := []string{"tailwind", "bulma", "pico", "none"}
        m.cssFramework = frameworks[m.cssFrameworkIdx]
        m.stage = 2  // Go to field entry
        m.textInput.Reset()
        m.textInput.Placeholder = "name, email, age:int, ..."
        return m, nil

    } else if m.stage == 2 {
        // Field entry...
    }

// Handle up/down for CSS framework selection
if m.stage == 1 {
    switch msg.Type {
    case tea.KeyUp:
        if m.cssFrameworkIdx > 0 {
            m.cssFrameworkIdx--
        }
        return m, nil
    case tea.KeyDown:
        if m.cssFrameworkIdx < 3 {
            m.cssFrameworkIdx++
        }
        return m, nil
    }
}
```

**Add CSS framework UI in `View()` method:**
```go
if m.stage == 1 {
    b.WriteString(TitleStyle.Render("Select CSS Framework"))
    b.WriteString("\n\n")

    frameworks := []struct {
        name string
        desc string
    }{
        {"Tailwind CSS", "Utility-first, modern (default)"},
        {"Bulma", "Component-based, clean"},
        {"Pico CSS", "Semantic, minimal, classless"},
        {"None", "Pure HTML only"},
    }

    for i, fw := range frameworks {
        if i == m.cssFrameworkIdx {
            b.WriteString(SelectedStyle.Render(fmt.Sprintf("→ %s - %s", fw.name, fw.desc)))
        } else {
            b.WriteString(fmt.Sprintf("  %s - %s", fw.name, fw.desc))
        }
        b.WriteString("\n")
    }

    b.WriteString("\n")
    b.WriteString(HelpStyle.Render("↑/↓: Navigate  Enter: Select  Esc: Cancel  ?: Help"))
    return b.String()
}
```

**Update generator call in stage 4:**
```go
if m.stage == 4 {
    // Generate the resource
    err := generator.GenerateResource(m.basePath, m.moduleName, m.resourceName, fields, m.cssFramework)
    // ... rest unchanged
}
```

#### 5b. Update `ui/gen_view.go` (~40 lines)

Similar changes as gen_resource.go but simpler since it's view-only.

### 6. Update Help Text (~20 lines)

**File:** `main.go`

**Update `printUsage()` function:**
```go
fmt.Println("Direct Mode Examples:")
fmt.Println("  lvt new myapp")
fmt.Println("  lvt gen users name:string email:string")
fmt.Println("  lvt gen users name email --css=tailwind     (default)")
fmt.Println("  lvt gen users name email --css=bulma")
fmt.Println("  lvt gen users name email --css=pico")
fmt.Println("  lvt gen users name email --css=none")
fmt.Println("  lvt gen view counter --css=pico")
```

### 7. Rewrite Templates with Conditionals

This is the largest task. The templates need to be rewritten to use the CSS helper functions.

#### 7a. Resource Template (`templates/resource/template.tmpl.tmpl`) (~150 lines)

See the detailed template in the implementation plan. Key patterns:

```html
<!-- CDN link -->
[[csscdn .CSSFramework]]

<!-- Container -->
<div class="[[containerClass .CSSFramework]]">

<!-- Box/Card -->
[[if needsArticle .CSSFramework]]<article>[[else]]<div class="[[boxClass .CSSFramework]]">[[end]]

<!-- Title -->
<h1 [[if ne (titleClass .CSSFramework) ""]]class="[[titleClass .CSSFramework]]"[[end]]>

<!-- Input -->
<input class="[[inputClass .CSSFramework]]" type="text">

<!-- Button -->
<button class="[[buttonClass .CSSFramework "primary"]]">

<!-- Table -->
<table class="[[tableClass .CSSFramework]]">
  <thead [[if ne (theadClass .CSSFramework) ""]]class="[[theadClass .CSSFramework]]"[[end]]>
    <tr>
      <th [[if ne (thClass .CSSFramework) ""]]class="[[thClass .CSSFramework]]"[[end]]>

<!-- Close wrapper -->
[[if needsArticle .CSSFramework]]</article>[[else]]</div>[[end]]
```

#### 7b. View Template (`templates/view/template.tmpl.tmpl`) (~30 lines)

Simpler version of resource template with same patterns.

### 8. Update Template Copy Command (~30 lines)

**File:** `commands/template.go`

The template copy command works as-is since we're using one unified template. No changes needed unless you want to copy framework-specific versions.

### 9. Update README (~20 lines)

**File:** `cmd/lvt/README.md`

- Change "Bulma CSS UI" → "Tailwind CSS UI (or Bulma, Pico, None)"
- Add CSS framework selection documentation
- Update examples with `--css` flag

### 10. Testing (~50 lines of test updates)

**Update golden files:**
- `testdata/golden/resource_template.tmpl.golden` - Use Tailwind classes

**Test all frameworks:**
```bash
lvt gen products name price:float --css=tailwind
lvt gen products name price:float --css=bulma
lvt gen products name price:float --css=pico
lvt gen products name price:float --css=none
```

## Estimated Effort

- **generateFile() update**: 15 minutes
- **Generator function updates**: 30 minutes
- **CLI command updates**: 45 minutes
- **Interactive UI updates**: 2 hours
- **Template rewrites**: 2-3 hours (most complex)
- **Testing**: 1 hour

**Total**: ~6-7 hours of focused work

## Testing Strategy

1. **Unit tests**: Ensure CSS helpers return correct values
2. **Integration tests**: Generate resources with each framework
3. **Visual tests**: Verify generated HTML renders correctly
4. **Golden file tests**: Update goldens with Tailwind as default

## Notes

- The conditional template approach is more maintainable than 8 separate files
- Users can still override templates using custom template system
- Default is Tailwind (most popular, modern)
- Pico CSS requires semantic HTML (`<article>`, `<main>`)
- None option is for users who want full CSS control
