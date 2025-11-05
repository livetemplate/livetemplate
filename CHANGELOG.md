# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2025-11-05

**API Reduction & Cleanup** - Significant reduction in public API surface area and improved code organization.

### Changed

- **File Consolidation**: Reduced main package files from 18 to 12 (33% reduction)
  - Merged `session.go` + `session_redis.go` → `session_stores.go` (533 lines)
  - Merged `health_redis.go` into `health.go`
  - Created `types.go` from `build_types.go` with clean re-exports

- **Internal Package Restructure**: Moved implementation details to internal packages
  - **`internal/session/`**: Connection, ConnectionRegistry, ConnectionLimits
    - WebSocket connection tracking and indexing
    - Connection limit enforcement
    - Thread-safe concurrent access

  - **`internal/signature/`**: StructureSignature, ClientStructureRegistry
    - Structure tracking for optimization
    - Client-side structure registry
    - Reduces update payload by detecting structure changes

  - **`internal/context/`**: TemplateContext
    - Template execution context
    - Error handling and dev mode support

- **API Surface Reduction**: Implementation types no longer exported in main package
  - Cleaner public API focused on essential user-facing types
  - Internal implementation details properly hidden
  - Improved package organization

### Removed

- **Deleted Files**: Obsolete files after consolidation
  - `errors.go` (moved to internal/context)
  - `structure_signature.go` (moved to internal/signature)
  - `client_structure_registry.go` (moved to internal/signature)
  - `session.go` (merged into session_stores.go)
  - `session_redis.go` (merged into session_stores.go)
  - `health_redis.go` (merged into health.go)
  - `build_types.go` (replaced by types.go)
  - `registry.go` (moved to internal/session)
  - `limits.go` (moved to internal/session)

### Fixed

- **Test Organization**: Moved test files to internal packages
  - `internal/signature/*_test.go` - Structure tracking tests
  - `internal/session/*_test.go` - Connection management tests
  - Fixed import cycles by using separate test packages
  - 100% test coverage maintained

### Migration Guide

**For Most Users:**
- ✅ No changes required - public API for templates, stores, and actions remains stable
- ✅ All existing examples and applications continue to work

**If Directly Using Internal Types:**
- Update imports for moved types:
  ```go
  // Before
  import "github.com/livetemplate/livetemplate"
  registry := livetemplate.NewConnectionRegistry()

  // After
  import "github.com/livetemplate/livetemplate/internal/session"
  registry := session.NewConnectionRegistry()
  ```

- Affected types (now in internal packages):
  - `Connection`, `ConnectionRegistry`, `ConnectionLimits` → `internal/session/`
  - `StructureSignature`, `ClientStructureRegistry` → `internal/signature/`
  - `TemplateContext` → `internal/context/`

**Note**: If you were using these internal types, consider whether your use case requires direct access. The public API provides all functionality needed for typical applications.

### Technical Details

**Package Organization:**
- Main package now contains 12 files (down from 18)
- 3 new internal packages for implementation details
- Cleaner separation between public API and implementation
- Improved test organization with internal package tests

**Benefits:**
- Smaller public API surface (easier to understand and maintain)
- Better encapsulation of implementation details
- Clear distinction between user-facing and internal APIs
- Reduced chance of accidental breaking changes
- Improved code organization and maintainability

## [0.1.2] - 2025-11-03

**Cleanup** - Removed extracted components from core library and added cross-repository testing.

### Removed

- **Extracted Directories**: Cleaned up repository after extraction
  - Removed `cmd/lvt/` directory (moved to [livetemplate/lvt](https://github.com/livetemplate/lvt))
  - Removed `client/` directory (moved to [livetemplate/client](https://github.com/livetemplate/client))
  - Removed `examples/` directory (moved to [livetemplate/examples](https://github.com/livetemplate/examples))

- **Build Configuration**: Removed files specific to extracted components
  - Removed `Makefile` (lvt-specific, now in lvt repo)
  - Removed `.goreleaser.yml` (lvt binary builds, now in lvt repo)

### Added

- **Cross-Repository Testing**: Automated CI workflows to test dependent repositories
  - `.github/workflows/test.yml` - Core library tests
  - `.github/workflows/cross-repo-test.yml` - Tests lvt and examples against PRs
  - Catches breaking changes before merge
  - Runs in parallel for faster feedback

- **Local Development Workflows**: Tools for multi-repository development
  - Go workspace support (recommended approach)
  - Local development scripts in lvt and examples repositories
  - Documentation in CONTRIBUTING.md
  - See [workspace repository](https://github.com/livetemplate/workspace) for setup

### Fixed

- **Test Workflow**: Updated to exclude extracted directories
  - Filter out `/cmd/lvt`, `/examples`, `/client` from test runs
  - Only test core library packages

### Notes

- This is a cleanup release following the repository extraction in v0.1.1
- Core library is now purely a Go library with no build artifacts
- For CLI tool, examples, or client library, see respective repositories

## [0.1.1] - 2025-11-03

**Repository Restructuring** - LiveTemplate components extracted into separate repositories for independent development and versioning.

### Changed

- **Repository Structure**: Extracted components into separate repositories
  - **Client Library**: Moved to [livetemplate/client](https://github.com/livetemplate/client)
    - Published as npm package: `@livetemplate/client`
    - Independent TypeScript development and testing
    - Dedicated CI/CD for client releases

  - **CLI Tool (lvt)**: Moved to [livetemplate/lvt](https://github.com/livetemplate/lvt)
    - Go module: `github.com/livetemplate/lvt`
    - Exportable `testing` package for E2E tests
    - Independent CLI tool development

  - **Examples**: Moved to [livetemplate/examples](https://github.com/livetemplate/examples)
    - 8 complete example applications
    - Each example self-contained with own `go.mod`
    - Separate E2E testing and CI

- **Version Synchronization**: All repositories follow major.minor version alignment
  - Core v0.1.x → Client v0.1.x, LVT v0.1.x, Examples v0.1.x
  - Patch versions independent across repositories
  - Release scripts validate version compatibility

- **Documentation**: Updated all documentation to reference new repository structure
  - README.md includes "Related Repositories" section
  - CONTRIBUTING.md clarified for core library only
  - Links updated to point to separate repositories

### Migration Guide

**For Users:**
- No breaking changes to core library API
- Client library now available via npm: `npm install @livetemplate/client`
- CLI tool installation: `go install github.com/livetemplate/lvt@latest`
- Examples available at: https://github.com/livetemplate/examples

**For Contributors:**
- Core library contributions: https://github.com/livetemplate/livetemplate
- Client contributions: https://github.com/livetemplate/client
- CLI tool contributions: https://github.com/livetemplate/lvt
- Example contributions: https://github.com/livetemplate/examples

### Technical Details

- Core library remains at `github.com/livetemplate/livetemplate` (note: module path is `github.com/livefir/livetemplate`)
- All extracted repositories maintain full git history
- CI/CD configured independently for each repository
- Cross-repository version synchronization via release scripts

## [0.1.0] - 2025-11-02

**First official release of LiveTemplate** - A Go library for building reactive web applications with tree-based DOM diffing.

### Package Information

- **Go Module**: `github.com/livetemplate/livetemplate`
- **TypeScript Client**: `@livetemplate/client`
- **CLI Tool**: `lvt`

### Added

#### Kits System
- **Kit System**: Complete starter packages combining CSS frameworks with components and templates
  - System kits: Tailwind CSS, Bulma, Pico CSS, and plain HTML (none)
  - Each kit includes CSS helpers (~70 methods), reusable UI components, and generator templates
  - Components are part of kits (located in `kits/<name>/components/` directory)
  - System components included: layout, form, table, pagination, toolbar, detail, search, sort, stats
  - CSSHelpers interface for unified CSS class generation
  - Kit loader with auto-discovery from configured paths
  - Kit validation with Go AST parsing and interface compliance checking
  - Kit scaffolding and customization with `lvt kits create` and `lvt kits customize`

- **Configuration System**: User config at `~/.config/lvt/config.yaml`
  - Configurable kit search paths
  - `lvt config` commands: list, get, set

- **Development Server**: Unified `lvt serve` command with three modes
  - **Component Mode**: Live preview with JSON test data editor and hot reload
  - **Kit Mode**: CSS helper showcase with live examples
  - **App Mode**: Go app process management with auto-rebuild on file changes
  - WebSocket-based hot reload for all modes
  - Auto-detection of mode based on directory structure
  - Reverse proxy for app mode
  - File watching with debouncing

- **CLI Commands**:
  - `lvt kits list/create/info/validate/customize` - Kit management (includes components)
  - `lvt config list/get/set` - Configuration management
  - `lvt serve` - Development server with hot reload

- **Validation System**:
  - Kit validation: structure, manifest, templates, Go code compilation, interface compliance
  - Component validation within kits: structure, manifest, templates
  - Three-tier validation: errors, warnings, info
  - Pretty-printed output with emoji indicators (✅/❌/⚠️/ℹ️)

- **Documentation**:
  - `docs/guides/user-guide.md` - Getting started and usage
  - `docs/guides/kit-development.md` - Creating custom CSS kits (includes components)
  - `docs/guides/serve-guide.md` - Development server guide
  - `docs/references/api-reference.md` - Complete API reference

- **Testing**:
  - Kit loader tests (including component loading within kits)
  - Config management tests
  - Validator tests
  - E2E workflow tests
  - Serve package tests
  - Total: 123+ tests

### Changed

- **Generator Integration**: All generators now use component and kit loaders
  - `lvt new` uses kits for app generation
  - `lvt gen` uses kits for resource generation
  - Template generation uses components and kit helpers
  - CSS framework now part of kit manifest (multi/single: Tailwind, simple: Pico)

- **Help Text**: Enhanced with complete command reference and documentation links

### Technical Details

**New Packages**:
- `cmd/lvt/internal/components` - Component loading and management
- `cmd/lvt/internal/kits` - Kit loading and CSS helper interface
- `cmd/lvt/internal/config` - Configuration management
- `cmd/lvt/internal/validator` - Component and kit validation
- `cmd/lvt/internal/serve` - Development server implementation

**Architecture**:
- Path-based auto-discovery for components and kits
- Embedded system components and kits via Go embed
- Kit helper functions bridge to Go template.FuncMap
- Component templates use `[[ ]]` delimiters
- WebSocket protocol for hot reload communication
- Polling-based file watching for cross-platform compatibility

**Key Features**:
- CSS-independent components work with any kit
- Unified CSSHelpers interface (~70 methods)
- Components have manifest with inputs, templates, dependencies
- Kits provide CSSCDN() and styling helper methods
- Development server auto-detects component/kit/app mode
- Validation catches errors before deployment
- Comprehensive documentation for all features

---

## [Previous Versions]

For changes prior to the components library feature, see git history.
