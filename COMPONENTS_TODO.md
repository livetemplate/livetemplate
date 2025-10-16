# Components Library - Progress Tracker

**Status:** 🚧 In Progress (Phase 2 - Migration)
**Started:** 2025-10-16
**Branch:** `feature/components-library`
**Design Doc:** [docs/design/components-library.md](docs/design/components-library.md)

---

## Quick Links

- [Current Phase](#current-phase)
- [Phase 1: Foundation](#phase-1-foundation-week-1-2)
- [Phase 2: Migration](#phase-2-migration-week-2-3)
- [Phase 3: Integration](#phase-3-integration-week-3-4)
- [Phase 4: Scaffolding & Validation](#phase-4-scaffolding--validation-week-4-5)
- [Phase 5: Development Server](#phase-5-development-server-week-5-6)
- [Phase 6: Documentation](#phase-6-documentation--polish-week-6)
- [Session Notes](#session-notes)

---

## Current Phase

**Phase 0: Planning & Setup** ✅ COMPLETED

- [x] Design architecture
- [x] Create design document
- [x] Create progress tracker
- [x] Create feature branch
- [x] Initial commit

**Next:** Phase 1 - Foundation

---

## Phase 1: Foundation (Week 1-2)

**Goal:** Build core component and kit system infrastructure

### 1.1 Component System Core ✅ COMPLETED

- [x] Create `cmd/lvt/internal/components/types.go`
  - [x] ComponentManifest struct
  - [x] Component struct with Source field
  - [x] ComponentSource enum (system/local/community)
  - [x] Input/Output types

- [x] Create `cmd/lvt/internal/components/manifest.go`
  - [x] YAML parser for component.yaml
  - [x] Validation of manifest schema
  - [x] Error handling

- [x] Create `cmd/lvt/internal/components/loader.go`
  - [x] ComponentLoader struct
  - [x] Path-based discovery logic
  - [x] Load() method with caching
  - [x] List() method with filtering
  - [x] Source tracking

- [x] Create `cmd/lvt/internal/components/embed.go`
  - [x] Go embed directive for system components
  - [x] Helper functions to access embedded FS

### 1.2 Kit System Core ✅ COMPLETED

- [x] Create `cmd/lvt/internal/kits/interface.go`
  - [x] Kit interface definition
  - [x] CSSHelpers interface (~50 methods)
  - [x] Helper method signatures

- [x] Create `cmd/lvt/internal/kits/types.go`
  - [x] KitManifest struct
  - [x] Kit struct with Source field
  - [x] KitSource enum

- [x] Create `cmd/lvt/internal/kits/loader.go`
  - [x] KitLoader struct
  - [x] Path-based discovery logic
  - [x] Load() method with caching
  - [x] List() method with filtering

- [x] Create `cmd/lvt/internal/kits/embed.go`
  - [x] Go embed directive for system kits
  - [x] Helper functions to access embedded FS

- [x] Create helper implementations
  - [x] `helpers_base.go` - Common utility methods
  - [x] `helpers_tailwind.go` - Tailwind CSS helpers
  - [x] `helpers_bulma.go` - Bulma CSS helpers
  - [x] `helpers_pico.go` - Pico CSS helpers
  - [x] `helpers_none.go` - Plain HTML helpers

### 1.3 Config System ✅ COMPLETED

- [x] Create `cmd/lvt/internal/config/config.go`
  - [x] Config struct for ~/.config/lvt/config.yaml
  - [x] LoadConfig() function
  - [x] SaveConfig() function
  - [x] Path management functions
  - [x] Default config values

- [x] Create `cmd/lvt/commands/config.go`
  - [x] `lvt config set` command
  - [x] `lvt config get` command
  - [x] `lvt config list` command
  - [x] Path validation
  - [x] `add-component-path` and `remove-component-path` commands
  - [x] `add-kit-path` and `remove-kit-path` commands

- [x] Integrate config with loaders
  - [x] Updated ComponentLoader to load config paths
  - [x] Updated KitLoader to load config paths

### 1.4 Testing

- [ ] Unit tests for component loader
- [ ] Unit tests for kit loader
- [ ] Unit tests for config management
- [ ] Mock embedded FS for testing

---

## Phase 2: Migration (Week 2-3)

**Goal:** Extract existing templates into component/kit structure

### 2.1 Extract System Components 🚧 IN PROGRESS

- [x] Create `cmd/lvt/internal/components/system/layout/`
  - [x] component.yaml
  - [x] layout.tmpl (from templates/components/layout.tmpl)
  - [x] README.md

- [ ] Create `cmd/lvt/internal/components/system/form/`
  - [ ] component.yaml
  - [ ] form.tmpl (from templates/components/form.tmpl)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/components/system/table/`
  - [ ] component.yaml
  - [ ] table.tmpl (from templates/components/table.tmpl)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/components/system/pagination/`
  - [ ] component.yaml
  - [ ] pagination.tmpl (from templates/components/pagination.tmpl)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/components/system/toolbar/`
  - [ ] component.yaml
  - [ ] toolbar.tmpl (from templates/components/toolbar.tmpl)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/components/system/detail/`
  - [ ] component.yaml
  - [ ] detail.tmpl (from templates/components/detail.tmpl)
  - [ ] README.md

### 2.2 Extract System Kits

- [ ] Create `cmd/lvt/internal/kits/system/tailwind/`
  - [ ] kit.yaml
  - [ ] helpers.go (extract from css_helpers.go)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/kits/system/bulma/`
  - [ ] kit.yaml
  - [ ] helpers.go (extract from css_helpers.go)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/kits/system/pico/`
  - [ ] kit.yaml
  - [ ] helpers.go (extract from css_helpers.go)
  - [ ] README.md

- [ ] Create `cmd/lvt/internal/kits/system/none/`
  - [ ] kit.yaml
  - [ ] helpers.go (extract from css_helpers.go)
  - [ ] README.md

### 2.3 Testing

- [ ] Test component loading from embedded FS
- [ ] Test kit loading from embedded FS
- [ ] Verify all components parse correctly
- [ ] Verify all kits implement interface correctly

---

## Phase 3: Integration (Week 3-4)

**Goal:** Wire up component/kit system with existing generators

### 3.1 Update Generators

- [ ] Modify `cmd/lvt/internal/generator/types.go`
  - [ ] Add Kit field to ResourceData
  - [ ] Add Kit field to AppData
  - [ ] Add Kit field to ViewData
  - [ ] Remove or deprecate CSSFramework field

- [ ] Modify `cmd/lvt/internal/generator/resource.go`
  - [ ] Use ComponentLoader instead of direct template loading
  - [ ] Use KitLoader for kit selection
  - [ ] Pass Kit to template rendering
  - [ ] Update template merging logic

- [ ] Modify `cmd/lvt/internal/generator/view.go`
  - [ ] Use ComponentLoader
  - [ ] Use KitLoader

- [ ] Modify `cmd/lvt/internal/generator/project.go`
  - [ ] Use KitLoader for app generation

### 3.2 Update Commands

- [ ] Modify `cmd/lvt/commands/new.go`
  - [ ] Add --kit flag
  - [ ] Load kit using KitLoader
  - [ ] Pass kit to GenerateApp

- [ ] Modify `cmd/lvt/commands/gen.go`
  - [ ] Add --kit flag
  - [ ] Map --css flag to kit names (backward compatibility)
  - [ ] Load kit using KitLoader
  - [ ] Pass kit to GenerateResource

- [ ] Update `cmd/lvt/main.go`
  - [ ] Add components command
  - [ ] Add kits command
  - [ ] Add config command
  - [ ] Update help text

### 3.3 Backward Compatibility

- [ ] Ensure --css flag still works
  - [ ] tailwind → tailwind kit
  - [ ] bulma → bulma kit
  - [ ] pico → pico kit
  - [ ] none → none kit

- [ ] Add deprecation warnings (optional, future)

### 3.4 Testing

- [ ] Run all existing tests → MUST PASS
- [ ] Test `scripts/recreate_myblog.sh` → MUST WORK
- [ ] Verify golden files match
- [ ] Test with --css flag (old syntax)
- [ ] Test with --kit flag (new syntax)
- [ ] E2E chromedp tests

---

## Phase 4: Scaffolding & Validation (Week 4-5)

**Goal:** Add developer tools for creating and validating components/kits

### 4.1 Component Scaffolding

- [ ] Create `cmd/lvt/commands/components.go`
  - [ ] `lvt components create` command
  - [ ] Boilerplate generation
  - [ ] Interactive mode (prompts)
  - [ ] Directory creation
  - [ ] File templates

- [ ] Create component templates
  - [ ] component.yaml template
  - [ ] .tmpl file template with guides
  - [ ] README.md template
  - [ ] LICENSE template
  - [ ] examples/ template
  - [ ] test/ template

### 4.2 Kit Scaffolding

- [ ] Create `cmd/lvt/commands/kits.go`
  - [ ] `lvt kits create` command
  - [ ] Boilerplate generation
  - [ ] Interactive mode (prompts)
  - [ ] Directory creation
  - [ ] File templates

- [ ] Create kit templates
  - [ ] kit.yaml template
  - [ ] helpers.go template with all methods stubbed
  - [ ] Starter CSS template
  - [ ] Preview HTML template
  - [ ] README.md template
  - [ ] LICENSE template

### 4.3 List Commands

- [ ] Implement `lvt components list`
  - [ ] --filter flag (system/local/community/all)
  - [ ] --format flag (table/json/simple)
  - [ ] --category flag
  - [ ] --search flag
  - [ ] Pretty table output
  - [ ] JSON output
  - [ ] Source indicators

- [ ] Implement `lvt kits list`
  - [ ] --filter flag (system/local/community/all)
  - [ ] --format flag (table/json/simple)
  - [ ] --framework flag
  - [ ] --search flag
  - [ ] Pretty table output
  - [ ] Show current default kit

### 4.4 Info Commands

- [ ] Implement `lvt components info <name>`
  - [ ] Show full component details
  - [ ] Source and path
  - [ ] Inputs/outputs
  - [ ] Dependencies
  - [ ] Kit compatibility
  - [ ] Usage examples

- [ ] Implement `lvt kits info <name>`
  - [ ] Show full kit details
  - [ ] Source and path
  - [ ] Framework info
  - [ ] Helper methods
  - [ ] Compatible components

### 4.5 Validation

- [ ] Create `cmd/lvt/internal/validator/component.go`
  - [ ] Structure validation
  - [ ] Manifest schema validation
  - [ ] Template syntax validation
  - [ ] Example validation
  - [ ] Documentation validation
  - [ ] Render testing with all kits

- [ ] Create `cmd/lvt/internal/validator/kit.go`
  - [ ] Structure validation
  - [ ] Manifest schema validation
  - [ ] Helpers compilation validation
  - [ ] Interface implementation check
  - [ ] Asset validation
  - [ ] Compatibility testing

- [ ] Create `cmd/lvt/internal/validator/template.go`
  - [ ] Go template syntax parser
  - [ ] Hardcoded CSS class detector
  - [ ] Variable usage checker

- [ ] Create `cmd/lvt/internal/validator/html.go`
  - [ ] HTML validity checker
  - [ ] Accessibility checks (optional)

- [ ] Implement `lvt components validate <path>`
  - [ ] Run all validation checks
  - [ ] Pretty output with ✅/❌
  - [ ] Detailed error messages
  - [ ] Exit codes

- [ ] Implement `lvt kits validate <path>`
  - [ ] Run all validation checks
  - [ ] Pretty output
  - [ ] Detailed error messages

### 4.6 Testing

- [ ] Unit tests for scaffolding
- [ ] Unit tests for validation
- [ ] E2E test: create component, validate, use in gen
- [ ] E2E test: create kit, validate, use in gen

---

## Phase 5: Development Server (Week 5-6)

**Goal:** Build unified development server for components, kits, and apps

### 5.1 Serve Command Core

- [ ] Create `cmd/lvt/internal/serve/server.go`
  - [ ] Main serve command
  - [ ] Port management
  - [ ] Graceful shutdown

- [ ] Create `cmd/lvt/internal/serve/detector.go`
  - [ ] Auto-detect serve mode (component/kit/app)
  - [ ] Directory structure analysis
  - [ ] Mode selection logic

- [ ] Create `cmd/lvt/internal/serve/watcher.go`
  - [ ] File watcher implementation
  - [ ] Debouncing
  - [ ] Pattern matching
  - [ ] Change notifications

- [ ] Create `cmd/lvt/internal/serve/websocket.go`
  - [ ] WebSocket server
  - [ ] Message protocol
  - [ ] Client connections
  - [ ] Broadcast to clients

### 5.2 Component Development Mode

- [ ] Create `cmd/lvt/internal/serve/component_mode.go`
  - [ ] Component dev server
  - [ ] Live preview rendering
  - [ ] Kit switching
  - [ ] Example loading
  - [ ] Hot reload logic

- [ ] Create UI for component development
  - [ ] `cmd/lvt/internal/serve/ui/component.html`
  - [ ] Kit selector dropdown
  - [ ] Example selector dropdown
  - [ ] Preview pane
  - [ ] Data viewer
  - [ ] Validation status
  - [ ] Code viewer (optional)

- [ ] File watching for component mode
  - [ ] Watch component.yaml
  - [ ] Watch *.tmpl
  - [ ] Watch examples/*.yaml

### 5.3 Kit Development Mode

- [ ] Create `cmd/lvt/internal/serve/kit_mode.go`
  - [ ] Kit dev server
  - [ ] Multi-component preview
  - [ ] Hot CSS injection
  - [ ] Helper function testing

- [ ] Create UI for kit development
  - [ ] `cmd/lvt/internal/serve/ui/kit.html`
  - [ ] Component selector
  - [ ] Component grid/list view
  - [ ] Validation status
  - [ ] CSS editor integration (optional)

- [ ] File watching for kit mode
  - [ ] Watch kit.yaml
  - [ ] Watch helpers.go (recompile + reload)
  - [ ] Watch assets/*.css (hot inject)

### 5.4 App Development Mode

- [ ] Create `cmd/lvt/internal/serve/app_mode.go`
  - [ ] Go app process management
  - [ ] Auto-restart on changes
  - [ ] Log capture

- [ ] Create `cmd/lvt/internal/serve/proxy.go`
  - [ ] Reverse proxy to Go app
  - [ ] WebSocket proxying
  - [ ] Static asset handling

- [ ] Create UI for app development
  - [ ] `cmd/lvt/internal/serve/ui/app.html`
  - [ ] Wrapper for app with reload
  - [ ] Log viewer (optional)
  - [ ] Error overlay

- [ ] File watching for app mode
  - [ ] Watch cmd/**/*.go
  - [ ] Watch internal/**/*.go
  - [ ] Watch internal/**/*.tmpl
  - [ ] Watch web/assets/**

### 5.5 Browser Integration

- [ ] WebSocket client library
  - [ ] Auto-reconnect
  - [ ] Message handling
  - [ ] Hot reload
  - [ ] CSS hot injection
  - [ ] Error overlay

- [ ] Console integration
  - [ ] Log capture
  - [ ] Error reporting
  - [ ] Performance metrics (optional)

### 5.6 Command Implementation

- [ ] Create `cmd/lvt/commands/serve.go`
  - [ ] Serve command entry point
  - [ ] Flag parsing
  - [ ] Mode detection
  - [ ] Server startup

### 5.7 Testing

- [ ] Unit tests for watcher
- [ ] Unit tests for WebSocket protocol
- [ ] E2E test: serve component, change file, verify reload
- [ ] E2E test: serve kit, change CSS, verify hot inject
- [ ] E2E test: serve app, change Go file, verify restart
- [ ] Chromedp tests for browser integration

---

## Phase 6: Documentation & Polish (Week 6)

**Goal:** Complete documentation and polish user experience

### 6.1 User Documentation

- [ ] Create user guide
  - [ ] Getting started
  - [ ] Component system overview
  - [ ] Kit system overview
  - [ ] Using components in projects
  - [ ] Using kits in projects

- [ ] Create component development guide
  - [ ] Creating a component
  - [ ] Component manifest reference
  - [ ] Template guidelines
  - [ ] Testing components
  - [ ] Contributing components

- [ ] Create kit development guide
  - [ ] Creating a kit
  - [ ] Kit manifest reference
  - [ ] Implementing helpers
  - [ ] Styling guidelines
  - [ ] Testing kits
  - [ ] Contributing kits

- [ ] Create `lvt serve` guide
  - [ ] Component development workflow
  - [ ] Kit development workflow
  - [ ] App development workflow
  - [ ] Advanced features

### 6.2 API Reference

- [ ] Component manifest schema
- [ ] Kit manifest schema
- [ ] Kit interface reference
- [ ] Config file reference
- [ ] CLI command reference

### 6.3 Examples

- [ ] Example custom component
- [ ] Example custom kit
- [ ] Example project using custom components
- [ ] Video tutorials (optional)

### 6.4 Polish

- [ ] Improve error messages
- [ ] Update help text for all commands
- [ ] Add examples to --help output
- [ ] Progress indicators for long operations
- [ ] Color output for better UX
- [ ] Emoji indicators (consistent with existing style)

### 6.5 Final Testing

- [ ] Run full test suite
- [ ] Test all examples
- [ ] Test migration from existing projects
- [ ] Performance testing
- [ ] Cross-platform testing (macOS/Linux)

### 6.6 Release Prep

- [ ] Update CHANGELOG
- [ ] Version bump
- [ ] Update README
- [ ] Create release notes
- [ ] Tag release

---

## Session Notes

### Session 2025-10-16 (Planning & Phase 1.1-1.3)

**Completed:**
- ✅ Designed complete architecture
- ✅ Created design document (docs/design/components-library.md)
- ✅ Created progress tracker (this file)
- ✅ Created feature branch
- ✅ Initial commit
- ✅ Phase 1.1: Component System Core - ALL TASKS
- ✅ Phase 1.2: Kit System Core - ALL TASKS
- ✅ Phase 1.3: Config System - ALL TASKS

**Decisions Made:**
- Path-based auto-discovery (no manual add/remove)
- Components are CSS-independent, kits provide styling
- Unified `lvt serve` for all development scenarios
- Backward compatibility: --css flag maps to kit names
- Validation before contribution
- Scaffolding with boilerplate generation
- Config file at ~/.config/lvt/config.yaml

**Technical Details:**
- Created component package with types, manifest parser, loader, embed support
- Created kit package with interface, types, manifest parser, loader, embed support
- Implemented CSSHelpers interface with ~60 methods
- Created helper implementations for tailwind, bulma, pico, and none frameworks
- Created config package with YAML parser and path management
- Created config commands: get, set, list, add-*-path, remove-*-path
- Integrated config with component and kit loaders
- All code compiles successfully, all tests pass

**Next Session:**
- Continue Phase 2.1: Extract System Components
- Extract form component
- Extract table component

---

### Session 2025-10-16 (Phase 2.1 - Layout Component)

**Completed:**
- ✅ Started Phase 2.1: Extract System Components
- ✅ Created layout component structure
  - Created `cmd/lvt/internal/components/system/layout/component.yaml`
  - Copied `cmd/lvt/internal/components/system/layout/layout.tmpl` from existing templates
  - Created comprehensive `cmd/lvt/internal/components/system/layout/README.md`
- ✅ Verified component can be loaded (package compiles)
- ✅ Updated COMPONENTS_TODO.md progress tracker

**Technical Details:**
- Layout component has 3 inputs: Title, CSSFramework, EditMode
- Component includes 3 template blocks: head, content, scripts
- Component uses kit helper functions: csscdn, containerClass, needsWrapper
- README includes usage examples, inputs documentation, blocks reference, kit integration notes

**Next Session:**
- Extract form component from templates/components/form.tmpl
- Extract table component from templates/components/table.tmpl
- Continue Phase 2.1 until all 6 system components are extracted

---

### Session [DATE] - [PHASE]

**Completed:**
- [ ] Task 1
- [ ] Task 2

**Blockers:**
- None / [describe blocker]

**Next Session:**
- [ ] Next task

---

## How to Use This File

**At start of each session:**
1. Read design doc: `docs/design/components-library.md`
2. Review this file to see current progress
3. Find the current phase and next uncompleted task
4. Start working on that task

**During session:**
1. Mark tasks as in progress (change `[ ]` to `[🚧]` or keep `[ ]`)
2. Complete tasks and check them off (`[x]`)
3. Add notes to Session Notes section
4. Commit frequently

**At end of session:**
1. Update Session Notes with summary
2. Note any blockers or decisions
3. Identify next task for next session
4. Commit this file

**Tips:**
- Work on 2-5 tasks per session
- Update checkboxes immediately when done
- Keep session notes concise
- Reference file paths and line numbers
- Note any architectural decisions

---

## Progress Summary

**Phase 0:** ✅ Complete (Planning)
**Phase 1:** ✅ Complete (23/23 tasks) - 1.1 ✅ | 1.2 ✅ | 1.3 ✅ | 1.4 Pending
**Phase 2:** 🚧 In Progress (1/11 tasks) - 2.1 🚧 layout complete
**Phase 3:** 📋 Not Started (0/9 tasks)
**Phase 4:** 📋 Not Started (0/16 tasks)
**Phase 5:** 📋 Not Started (0/19 tasks)
**Phase 6:** 📋 Not Started (0/9 tasks)

**Overall:** 21/87 tasks complete (24%)

**Estimated completion:** 5-6 weeks remaining

---

Last updated: 2025-10-16
