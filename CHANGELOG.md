# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
