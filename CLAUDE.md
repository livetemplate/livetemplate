# LiveTemplate - Development Guidelines

## Project Overview

LiveTemplate is a high-performance Go library and CLI tool for building reactive web applications. The project consists of two main parts:

1. **Core Library** - Go library for generating ultra-efficient HTML template updates using tree-based optimization
2. **CLI Tool (lvt)** - Code generator and development server for rapid application development

The core library provides an API similar to `html/template` but with the additional capability of generating minimal, tree-based updates that can be efficiently transmitted to clients.

## Core Architecture

### Key Components

1. **Template Engine (`template.go`)**:
   - Main entry point providing `html/template` compatible API
   - Manages template parsing, execution, and update generation
   - Handles wrapper ID injection for update targeting

2. **Tree Structure (`tree.go`)**:
   - Implements tree-based static/dynamic separation
   - Manages fingerprinting for change detection
   - Provides tree diffing and update generation

3. **Full Tree Parser (`full_tree_parser.go`)**:
   - Parses Go templates into tree structures
   - Handles template constructs (fields, conditionals, ranges, with, template invokes)
   - Manages construct compilation and hydration

4. **HTML Tree (`html_tree.go`)**:
   - Manages HTML node tree structures
   - Used for HTML-aware operations

5. **Client Library (`client/livetemplate-client.ts`)**:
   - TypeScript client for browser integration
   - Handles tree-based updates efficiently
   - Manages static content caching

## Key Data Structures

### TreeNode
```go
type TreeNode map[string]interface{}
```
- Core structure for representing static/dynamic content
- Keys: "s" for statics array, numeric strings for dynamic values
- Can be nested for complex templates

### Template
```go
type Template struct {
    name            string
    templateStr     string
    tmpl            *template.Template
    wrapperID       string
    lastData        interface{}
    lastHTML        string
    lastTree        TreeNode
    initialTree     TreeNode
    hasInitialTree  bool
    lastFingerprint string
    keyGen          *KeyGenerator
}
```

### Key Constructs
- `FieldConstruct`: Simple field replacement `{{.Field}}`
- `ConditionalConstruct`: If/else branches `{{if .Cond}}...{{else}}...{{end}}`
- `RangeConstruct`: Iteration `{{range .Items}}...{{end}}`
- `WithConstruct`: Context switching `{{with .Item}}...{{end}}`
- `TemplateInvokeConstruct`: Template invocation `{{template "name" .}}`

## Testing Strategy

### Test Files Structure
- `e2e_test.go`: End-to-end tests with complete rendering sequences
- `template_test.go`: Core template functionality tests
- `tree_invariant_test.go`: Tree structure invariant validation
- `key_injection_test.go`: Key generation and stability tests

### Test Data
- `testdata/e2e/`: Contains golden files for E2E tests
  - `*.html`: Expected rendered HTML output
  - `*.json`: Expected tree updates
  - `*.golden.json`: Golden files for update validation

### Running Tests
```bash
# Run all tests
go test -v ./...

# Run specific test categories
go test -run TestTemplate_E2E -v      # E2E tests
go test -run TestTreeInvariant -v     # Tree invariant tests
go test -run TestKeyInjection -v      # Key injection tests

# Run with timeout
go test -v ./... -timeout=30s
```

## Development Conventions

### Code Style
1. **No unnecessary comments** - Code should be self-documenting
2. **Follow existing patterns** - Check neighboring code for conventions
3. **Use existing utilities** - Don't reinvent the wheel
4. **Maintain idiomatic Go** - Follow Go best practices

### Template Processing Flow
1. **Parse**: Template string → Compiled template structure
2. **Execute**: First render generates initial tree with statics and dynamics
3. **Update**: Subsequent renders generate minimal update trees
4. **Diff**: Compare trees to produce update operations

### Key Generation Strategy
- Uses wrapper-based approach with sequential key generation
- Keys are stable within a single render
- Supports any data type without special handling
- Keys reset between renders for consistency

## Important Implementation Details

### Wrapper ID Injection
- All templates get a wrapper div with unique ID (`lvt-[random]`)
- Full HTML documents: Wrapper injected around body content
- Fragments: Entire content wrapped
- Used for targeting updates on client side

### Tree Update Format
```json
{
  "s": ["<div>", "</div>"],     // Static parts (cached client-side)
  "0": "Dynamic content",        // Dynamic value at position 0
  "1": {                         // Nested tree for complex structures
    "s": ["<span>", "</span>"],
    "0": "Nested dynamic"
  }
}
```

### Range Operations
For list updates, special operations are used:
- `["u", "item-id", updates]`: Update existing item
- `["i", "after-id", "position", data]`: Insert new item
- `["r", "item-id"]`: Remove item
- `["o", ["id1", "id2", ...]]`: Reorder items

## Pre-commit Hook
The repository has a pre-commit hook that:
1. Auto-formats Go code using `go fmt`
2. Runs all tests with 30-second timeout
3. Blocks commits if tests fail
4. Automatically stages formatted files

## Common Tasks

### Adding New Template Construct
1. Define construct type in `tree.go`
2. Implement `Construct` interface
3. Add parser in `full_tree_parser.go`
4. Add compilation logic
5. Add hydration logic
6. Write tests

### Debugging Tree Generation
1. Use `calculateFingerprint()` to track tree changes
2. Check `lastTree` vs current tree in Template
3. Validate tree structure with `validateTreeStructure()`
4. Use golden files for regression testing

### Updating Client Library
1. Edit `client/livetemplate-client.ts`
2. Ensure compatibility with tree format
3. Test with browser test suite
4. Update TypeScript types if needed

## Performance Considerations

1. **Tree Diffing**: O(n) complexity for most operations
2. **Memory**: Trees are kept in memory for diffing
3. **Fingerprinting**: MD5 hashing for change detection
4. **Key Generation**: Sequential integers for minimal overhead

## Security Notes

1. **HTML Escaping**: Uses `html/template` for automatic escaping
2. **No Direct HTML**: All content goes through template engine
3. **Wrapper IDs**: Random generation prevents conflicts

## Troubleshooting

### Test Failures
- Check golden files in `testdata/e2e/`
- Verify tree structure matches expected format
- Ensure key generation is consistent
- Check for HTML escaping issues

### Tree Generation Issues
- Validate template syntax
- Check construct parsing order
- Verify hydration logic matches compilation
- Test with simpler templates first

## Future Improvements
- Consider adding more sophisticated diffing algorithms
- Optimize memory usage for large trees
- Add metrics and profiling hooks
- Enhance client-side caching strategies

---

## CLI Tool (lvt)

The `lvt` CLI tool provides code generation and development server capabilities for rapid application development.

### Tool Structure

```
cmd/lvt/
├── main.go                     # CLI entry point
├── commands/                   # CLI commands
│   ├── new.go                  # Create new apps
│   ├── gen.go                  # Generate resources
│   ├── kits.go                 # Kit management
│   ├── config.go               # Configuration
│   └── serve.go                # Development server
├── internal/
│   ├── generator/              # Code generation
│   ├── kits/                   # Kit system
│   │   ├── loader.go           # Kit loading
│   │   ├── types.go            # Kit types
│   │   ├── manifest.go         # Manifest parsing
│   │   └── system/             # System kits
│   │       ├── tailwind/
│   │       ├── bulma/
│   │       ├── pico/
│   │       └── none/
│   ├── config/                 # Configuration management
│   ├── validator/              # Validation
│   └── serve/                  # Development server
```

### Kits System

Kits are complete starter packages that include:
- **CSS Helpers**: ~60 methods for generating CSS classes
- **Components**: Reusable UI template blocks (form, table, layout, etc.)
- **Templates**: Generator templates for resources, views, and apps

#### System Kits

Four built-in kits are embedded in the `lvt` binary:
1. **Tailwind** - Utility-first CSS framework
2. **Bulma** - Component-based CSS framework
3. **Pico** - Minimal semantic CSS framework
4. **None** - Plain HTML with no framework

#### Kit Cascade

Kits are loaded with cascade priority:
1. Project: `.lvt/kits/<name>/` (highest priority)
2. User: `~/.config/lvt/kits/<name>/`
3. System: Embedded in binary (fallback)

### CLI Commands

#### Application Commands
- `lvt new <name> --css <framework>` - Create new app
- `lvt gen <resource> [fields...] --css <framework>` - Generate CRUD resource

#### Kit Commands
- `lvt kits list` - List available kits
- `lvt kits info <name>` - Show kit information
- `lvt kits create <name>` - Create new kit
- `lvt kits customize <name>` - Copy kit for customization
- `lvt kits validate <path>` - Validate kit structure

#### Development Server
- `lvt serve` - Start development server with hot reload

### Development Conventions (CLI)

1. **Kit Manifests**: All kits have a `kit.yaml` manifest
2. **Component Templates**: Use `[[ ]]` delimiters (not `{{ }}`)
3. **Embedded Resources**: System kits are embedded via `//go:embed`
4. **Cascade Loading**: Project > User > System priority

### Key Implementation Details (CLI)

- **Kit Loader**: Automatically discovers and loads kits from configured paths
- **Generator**: Uses templates from kits to generate code
- **Hot Reload**: WebSocket-based reload for development server
- **Validation**: Validates kit structure, manifest, and templates before use