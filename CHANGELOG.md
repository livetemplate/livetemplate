# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Components & Kits System
- **Component System**: Reusable UI template blocks with YAML manifests
  - System components: layout, form, table, pagination, toolbar, detail
  - Component loader with auto-discovery from configured paths
  - Component validation with template syntax checking
  - Component scaffolding with `lvt components create`
  - Component search and filtering by category, source, tags

- **Kit System**: CSS framework integrations with unified helper interface
  - System kits: Tailwind CSS, Bulma, Pico CSS, and plain HTML (none)
  - CSSHelpers interface with ~70 helper methods
  - Kit loader with auto-discovery
  - Kit validation with Go AST parsing and interface compliance checking
  - Kit scaffolding with `lvt kits create`

- **Configuration System**: User config at `~/.config/lvt/config.yaml`
  - Configurable component and kit search paths
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
  - `lvt components list/create/info/validate` - Component management
  - `lvt kits list/create/info/validate` - Kit management
  - `lvt config list/get/set` - Configuration management
  - `lvt serve` - Development server with hot reload

- **Validation System**:
  - Component validation: structure, manifest, templates, documentation
  - Kit validation: structure, manifest, Go code compilation, interface compliance
  - Three-tier validation: errors, warnings, info
  - Pretty-printed output with emoji indicators (✅/❌/⚠️/ℹ️)

- **Documentation**:
  - `docs/user-guide.md` - Getting started and usage
  - `docs/component-development.md` - Creating custom components
  - `docs/kit-development.md` - Creating custom CSS kits
  - `docs/serve-guide.md` - Development server guide
  - `docs/api-reference.md` - Complete API reference

- **Testing**:
  - Component loader tests (20 tests)
  - Kit loader tests (18 tests)
  - Config management tests (22 tests)
  - Validator tests (26 tests)
  - E2E workflow tests (16 tests)
  - Serve package tests (21 tests)
  - Total: 123+ new tests

### Changed

- **Generator Integration**: All generators now use component and kit loaders
  - `lvt new` uses kits for app generation
  - `lvt gen` uses kits for resource generation
  - Template generation uses components and kit helpers
  - Backward compatibility maintained with `--css` flag

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
