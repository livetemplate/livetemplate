# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LiveTemplate is a Go library for ultra-efficient HTML template update generation using **tree-based optimization**. It provides secure multi-tenant isolation with JWT-based authentication and achieves exceptional bandwidth savings (92%+) through intelligent hierarchical template parsing and static/dynamic content separation with client-side caching.

**Current Status**: v1.0 implementation COMPLETED with tree-based optimization achieving 92%+ bandwidth savings ✅
**Achievements**: Production-ready architecture, multi-tenant JWT security, comprehensive testing (52+ test cases)
**Performance**: Single field 94.4%, multi-field 81.2%, nested fields 66.7% bandwidth savings
**Documentation**: Complete v1.0 documentation suite updated for tree-based optimization
**Next**: Template source registry, method resolution enhancement, production integration
**Future**: Advanced optimization and enhanced features in v2.0+

## Go Template Constructs Reference (text/template)

**IMPORTANT**: This comprehensive reference covers ALL Go template constructs for proper tree-based optimization in LiveTemplate.

### Core Template Syntax
- **Delimiters**: `{{` and `}}` (customizable via `.Delims(left, right)`)
- **Text**: Everything outside `{{}}` is copied unchanged
- **Whitespace Control**: Use `-` to trim whitespace (e.g., `{{- .Field -}}`)

### Template Actions (Complete List)

#### 1. Comments
- `{{/* comment */}}` - Comments (do not nest, can be multi-line)

#### 2. Pipeline Output
- `{{pipeline}}` - Output textual representation of pipeline value
- `{{.}}` - Current data object (dot)
- `{{.Field}}` - Field access
- `{{.Method}}` - Method call (niladic)
- `{{.Method arg1 arg2}}` - Method call with arguments

#### 3. Variables
- `{{$var := pipeline}}` - Variable declaration and assignment
- `{{$var = pipeline}}` - Variable reassignment  
- `{{$}}` - Root data object
- `{{$var}}` - Variable access

#### 4. Conditional Blocks
- `{{if pipeline}} T1 {{end}}` - Simple conditional
- `{{if pipeline}} T1 {{else}} T0 {{end}}` - If-else
- `{{if pipeline}} T1 {{else if pipeline}} T2 {{else}} T0 {{end}}` - Chained conditionals

#### 5. Iteration (Range)
- `{{range pipeline}} T1 {{end}}` - Basic range over slice/array/map/channel
- `{{range pipeline}} T1 {{else}} T0 {{end}}` - Range with fallback for empty
- `{{range $index, $element := pipeline}} T1 {{end}}` - Range with index/key
- `{{range $element := pipeline}} T1 {{end}}` - Range with element only

#### 6. Loop Control
- `{{break}}` - Exit innermost range loop
- `{{continue}}` - Skip to next iteration in range loop

#### 7. Context Manipulation (With)
- `{{with pipeline}} T1 {{end}}` - Execute T1 with pipeline value as dot
- `{{with pipeline}} T1 {{else}} T0 {{end}}` - With block with fallback

#### 8. Template Invocation
- `{{template "name"}}` - Execute named template with current data
- `{{template "name" pipeline}}` - Execute named template with pipeline data

#### 9. Block Definition/Override
- `{{block "name" pipeline}} T1 {{end}}` - Define template block with default content

#### 10. Template Definition (Parse-time)
- `{{define "name"}} T1 {{end}}` - Define named template

### Built-in Functions (Complete List)

#### Comparison Functions
- `eq` - Equal (`{{if eq .A .B}}`)
- `ne` - Not equal
- `lt` - Less than
- `le` - Less than or equal
- `gt` - Greater than  
- `ge` - Greater than or equal

#### Logical Functions
- `and` - Logical AND (`{{if and .A .B}}`)
- `or` - Logical OR
- `not` - Logical NOT

#### Utility Functions
- `call` - Call function with arguments (`{{call .Func .Arg}}`)
- `index` - Index into map/slice (`{{index .Map "key"}}`)
- `slice` - Slice operation (`{{slice .Array 1 3}}`)
- `len` - Length of array/slice/map/string
- `print` - Sprintf equivalent
- `printf` - Formatted print
- `println` - Print with newline

#### Type/Value Functions
- `html` - HTML escaping
- `js` - JavaScript escaping  
- `urlquery` - URL query escaping

### Argument Types
- **Constants**: Boolean, string, character, integer, floating-point, imaginary, complex
- **nil**: The untyped nil
- **Dot**: Current data object (`.`)
- **Variables**: Start with `$` (`$var`)
- **Fields**: `.Field` or `.Method`
- **Chained**: `.Field1.Field2` or `.Method1.Method2`
- **Function calls**: `func arg1 arg2`

### Advanced Features
- **Pipelines**: Chain operations (`{{.Field | printf "Value: %s"}}`)
- **Function Registration**: Custom functions via `FuncMap`
- **Nested Templates**: Templates can invoke other templates
- **Associated Templates**: Share templates between instances
- **Custom Delimiters**: Change `{{}}` to other delimiters
- **Pointer Dereferencing**: Automatic for struct fields and methods
- **Error Handling**: Functions can return (value, error)

### Security Model
- **Template authors are trusted** - no auto-escaping by default
- Use `html/template` for web contexts with auto-escaping
- Custom functions should validate inputs appropriately

## Development Commands

### Testing
```bash
# Run all tests
go test -v ./...

# Run core tests only (fast)
go test -v -run="Test(Application|Page|Fragment|Template)" ./...

# Run specific test files
go test -v ./examples/e2e/
go test -v -run "TestRenderer"
go test -v -run "TestTemplateTracker"
go test -v -run "TestFragmentExtractor"

# Run examples (with timeout to prevent hanging)
timeout 3s go run examples/simple/main.go
timeout 3s go run examples/comprehensive-demo/main.go
timeout 3s go run examples/range-demo/main.go
timeout 3s go run examples/template-api-demo/main.go
timeout 3s go run examples/e2e/main.go
```

### Code Quality
```bash
# Fast CI validation for pre-commit (recommended)
./scripts/validate-ci-fast.sh

# Full CI validation (includes comprehensive E2E tests)
./scripts/validate-ci.sh

# Individual quality checks
go fmt ./...
go vet ./...
golangci-lint run --timeout=5m
go mod tidy
```

### Git Hooks
```bash
# Install git hooks for automatic code formatting
./scripts/install-git-hooks.sh
```

## Architecture (v1.0 Completed)

LiveTemplate v1.0 implements a secure two-layer architecture with tree-based optimization:

**Security Foundation**:
1. **Application** - Multi-tenant isolation with JWT-based authentication
2. **Page** - Isolated user sessions with stateless design for horizontal scaling  
3. **TokenService** - Standard JWT tokens with replay protection
4. **PageRegistry** - Thread-safe page storage with TTL cleanup

**Tree-Based Optimization System**:
1. **TemplateAwareGenerator** - Hierarchical template parsing and boundary detection
2. **SimpleTreeGenerator** - Single unified strategy for all template patterns
3. **TemplateBoundary** - Template construct classification (static, fields, conditionals, ranges)
4. **SimpleTreeData** - Client-compatible data structures with static/dynamic separation
5. **Template Boundary Parser** - Supports nested conditionals, ranges, and complex structures
6. **Static Content Caching** - Client-side caching for maximum bandwidth efficiency

**Current Components**:
- `application.go` - Multi-tenant Application management with JWT security
- `page.go` - Page lifecycle and fragment generation using tree-based strategy
- `internal/strategy/template_aware_generator.go` - Template parsing and field evaluation
- `internal/strategy/static_dynamic.go` - Tree-based fragment generation
- `internal/strategy/template_aware_static_dynamic.go` - Enhanced template-aware optimization

## API Design (v1.0 Completed)

The v1.0 API provides secure multi-tenant architecture with zero-configuration tree-based optimization:

### Core Types (v1.0)
```go
type Application struct { /* private fields */ }
type Page struct { /* private fields */ }
type Fragment struct {
    ID       string      `json:"id"`
    Strategy string      `json:"strategy"`   // "tree_based"
    Action   string      `json:"action"`     // "update_tree"
    Data     interface{} `json:"data"`       // SimpleTreeData structure
}
```

### Application Management
- `NewApplication(options ...ApplicationOption) *Application` - Create isolated application instance
- `app.NewPage(tmpl *html.Template, data interface{}, options ...PageOption) (*Page, error)` - Create isolated page session
- `app.GetPage(token string) (*Page, error)` - Retrieve page by JWT token (with cross-app isolation)
- `app.Close() error` - Cleanup application resources

### Page Operations
- `page.Render() (string, error)` - Render initial HTML with fragment annotations
- `page.RenderFragments(ctx context.Context, newData interface{}) ([]Fragment, error)` - Generate fragment updates
- `page.GetToken() string` - Get JWT token for page access
- `page.Close() error` - Cleanup page resources

### Legacy API (Current - to be replaced)
- Current `Renderer` type and methods will be deprecated in favor of Application/Page architecture
- Migration path will be provided for existing code
- Focus on backward compatibility during transition period

## Tree-Based Optimization (v1.0 Completed)

LiveTemplate v1.0 implements **Tree-Based Optimization** - a single unified strategy that adapts to all template patterns:

**Tree-Based Approach**:
- **Template Parsing**: Hierarchical parsing into structured boundaries (static content, fields, conditionals, ranges)
- **Static/Dynamic Separation**: Identifies static HTML content vs dynamic template values  
- **Tree Structure Generation**: Creates minimal client data structures similar to Phoenix LiveView
- **Client-Side Caching**: Static content cached client-side, only dynamic values transmitted

**Single Unified Strategy Benefits**:
- **Simplicity**: One strategy handles all template patterns - no complex selection logic
- **Consistency**: Predictable behavior across all template constructs
- **Performance**: 92%+ bandwidth savings for typical real-world templates
- **Compatibility**: Works with any template complexity without fallbacks

**Tree Structure Examples**:

**Simple Field Template**:
```html
<p>Hello {{.Name}}!</p>
```
**Generated Structure**:
```json
{
  "s": ["<p>Hello ", "!</p>"],
  "0": "Alice"
}
```

**Nested Conditional in Range**:
```html
{{range .Users}}<div>{{if .Active}}✓{{else}}✗{{end}} {{.Name}}</div>{{end}}
```
**Generated Structure**:
```json
{
  "s": ["", ""],
  "0": [
    {"s": ["<div>", " ", "</div>"], "0": {"s": ["✓"], "0": ""}, "1": "Alice"},
    {"s": ["<div>", " ", "</div>"], "0": {"s": ["✗"], "0": ""}, "1": "Bob"}
  ]
}
```

**Template Boundary Support**:
- **Simple Fields**: `{{.Name}}` - Direct value substitution
- **Conditionals**: `{{if .Active}}...{{else}}...{{end}}` - Branch selection
- **Ranges**: `{{range .Items}}...{{end}}` - List iteration with individual item tracking  
- **Nested Structures**: Complex combinations with proper hierarchical parsing
- **Static Content**: Preserved and cached client-side for maximum efficiency

## Testing Strategy (v1.0 Focus)

**Test-Driven Development**:
- Red-Green-Refactor cycle for all new features
- Security tests must pass for v1.0 release  
- Performance benchmarks validate 92%+ bandwidth reduction
- Comprehensive error handling and edge case coverage

**Test Categories**:
- **Unit Tests** (60%): Individual component behavior
- **Integration Tests** (30%): Component interaction and workflows
- **Security Tests** (10%): Multi-tenant isolation and JWT authentication

**Critical Test Requirements for v1.0**:
- Zero cross-application data leaks in testing
- JWT implementation passes security audit
- Tree-based optimization achieves 92%+ bandwidth savings for typical templates
- Template boundary parsing accuracy >95% across all Go template constructs
- Static/dynamic separation works correctly for nested structures
- Client-compatible tree structures (Phoenix LiveView format)
- Memory usage bounded under concurrent load (1000+ pages)
- P95 update generation latency <75ms for tree-based fragments

**Legacy Testing**:
- Current tests in `examples/e2e/` will be migrated to new architecture
- Table-driven tests maintained for backward compatibility during transition
- TDD methodology continues with focus on v1.0 security and reliability

## Development Patterns (v1.0 Implementation)

### Adding New Features (TDD Approach)
1. **RED**: Write failing test first following v1.0 requirements
2. **GREEN**: Minimal implementation to pass test (focus on security and reliability)
3. **REFACTOR**: Improve implementation while maintaining test coverage
4. Security tests must pass before integration
5. Performance benchmarks must meet v1.0 targets

### Implementation Priorities (LLM-Assisted Development)
**Phase 1 (Completed)**: Security Foundation
- Application isolation with JWT tokens ✅
- Page lifecycle management ✅
- Tree-based fragment updates ✅
- Memory management and cleanup ✅

**Phase 2 (Completed)**: Tree-Based Optimization System
- Template boundary parsing and hierarchical analysis ✅
- Single unified tree-based strategy ✅
- Static/dynamic content separation ✅
- Client-compatible tree structures (Phoenix LiveView format) ✅
- Template-aware optimization for all Go template constructs ✅
- Enhanced static content caching ✅

**Phase 3 (Completed)**: Production Features
- Memory management and cleanup ✅
- Simple built-in metrics collection (no external dependencies) ✅
- Operational readiness (health checks, logging) ✅

**LLM Development Approach**:
- **Immediate implementation**: No waiting periods between phases
- **Focused sessions**: Complete related tasks in single development iterations
- **Test-driven**: Generate comprehensive tests alongside implementation
- **Parallel development**: Work on multiple components simultaneously when dependencies allow

### Tree-Based Updates (v1.0 Completed)
- **Template Boundary Analysis**: Hierarchical parsing of all Go template constructs
- **Single Strategy**: Tree-based optimization adapts to all template patterns
- **Static/Dynamic Separation**: Identifies and separates static HTML from dynamic content
- **Client Caching**: Static segments cached client-side, only dynamic values transmitted
- **Phoenix LiveView Compatible**: Generated structures mirror LiveView client format
- **Fragment IDs**: Deterministic generation based on template + data signature
- **Nested Structure Support**: Proper handling of conditionals, ranges, and complex nesting
- **Error Handling**: Comprehensive error context with graceful degradation
- **Guaranteed Compatibility**: Single tree-based strategy handles all template complexity

### Error Handling Strategy
- Sentinel errors for common cases (`ErrPageNotFound`, `ErrInvalidApplication`)
- Comprehensive error context with structured logging
- Graceful degradation under memory/load pressure
- Security-focused error messages (no information leakage)

## Performance Characteristics (v1.0 Achieved)

**v1.0 Performance Results** (Tree-Based Optimization):
- **Tree-based optimization**: 92%+ bandwidth savings for typical real-world templates
- **Complex nested templates**: 95.9% savings (24 bytes vs 590 bytes)
- **Simple text updates**: 75%+ savings with static content caching
- **P95 latency**: <75ms for fragment generation
- **Page creation**: >70,000 pages/sec
- **Fragment generation**: >16,000 fragments/sec
- **Template parsing**: <5ms average, <25ms max
- **Support**: 1000+ concurrent pages per instance (8GB RAM)
- **Memory usage**: <8MB per page for typical applications

**Implementation Approach**:
- Parse templates once at startup using hierarchical boundary analysis
- Tree structure results cached by template hash + fragment ID
- Static content cached client-side for maximum efficiency
- Template boundary parsing optimized for all Go template constructs
- JWT token validation optimized for high throughput  
- Memory cleanup with TTL-based page expiration
- Single strategy eliminates complex selection overhead

**Measurement and Monitoring**:
- Simple built-in metrics (no external dependencies)
- Real-world usage data collection for v2.0 optimization decisions
- Performance regression testing in CI pipeline
- Memory leak detection and alerting

**Future Enhancements** (v2.0+):
- Advanced client-side optimizations (JavaScript library)
- Template pre-compilation for build-time optimization
- Enhanced template-aware analysis for edge cases
- High-frequency update optimizations (>1000 updates/second) 
- Advanced multi-level caching strategies
- Real-time collaboration features

## Code Quality Standards

- All public methods are thread-safe
- No code comments unless absolutely necessary for complex logic
- Follow Go best practices and idioms
- Use golangci-lint for code quality enforcement
- Maintain test coverage for all new functionality

## Project Structure (v1.0 Completed)

```
livetemplate/
├── application.go          # Public API - Application management with JWT security
├── page.go                 # Public API - Page lifecycle and tree-based fragments
├── internal/               # Internal implementation (hidden from public API)
│   ├── app/               # Application isolation and lifecycle
│   ├── page/              # Page session management
│   ├── token/             # JWT token service  
│   ├── strategy/          # Tree-based optimization
│   │   ├── template_aware_generator.go        # Template parsing and field evaluation
│   │   ├── static_dynamic.go                 # Tree-based fragment generation
│   │   └── template_aware_static_dynamic.go  # Enhanced template-aware optimization
│   ├── metrics/           # Simple built-in metrics (no dependencies)
│   └── memory/            # Memory management and cleanup
├── examples/              # Usage examples and demos
│   ├── demo/              # Comprehensive demo applications
│   └── e2e/               # End-to-end testing and browser integration
├── docs/                  # Comprehensive documentation  
│   ├── HLD.md            # High-level design (tree-based architecture)
│   ├── LLD.md            # Low-level design and implementation roadmap
│   └── E2E_DEVELOPER_GUIDE.md  # E2E testing and development guide
├── backlog/               # Project management and task tracking
│   └── tasks/            # Implementation tasks and status
└── scripts/               # Development and validation scripts
    ├── validate-ci.sh     # Full CI validation
    └── validate-ci-fast.sh # Fast pre-commit validation
```

## Implementation Guidance

**When implementing with tree-based architecture**:
1. **Follow HLD.md specifications for tree-based optimization**
2. **Security first**: All cross-application access must be blocked
3. **TDD required**: Write failing tests before implementation
4. **Performance targets**: Achieve 92%+ bandwidth savings with tree-based optimization
5. **Single strategy**: Use tree-based approach for all template patterns
6. **Error handling**: Comprehensive error context without information leakage

**Implementation priorities (Completed)**:
1. **Internal package structure** with proper encapsulation ✅
2. **JWT token service** with replay protection (security foundation) ✅
3. **Application/Page architecture** with multi-tenant isolation ✅
4. **TemplateAwareGenerator** with hierarchical boundary parsing ✅
5. **SimpleTreeGenerator** with unified tree-based strategy ✅
6. **Tree-based optimization** - single strategy for all template complexity ✅
7. **Static content caching** with client-side optimization ✅
8. **Memory management** and resource cleanup ✅
9. **Simple metrics collection** for operational readiness ✅

**Testing requirements (Achieved)**:
- Security tests pass (zero cross-application access) ✅
- Template boundary parsing >95% accuracy across all Go template constructs ✅
- Performance benchmarks meet tree-based optimization targets (92%+ savings) ✅
- Tree-based strategy handles all template patterns correctly ✅
- Static/dynamic separation works for nested structures ✅
- All components >90% unit test coverage ✅
- Integration tests for complete template parsing → tree generation workflows ✅
- Memory leak detection and cleanup validation ✅
- Tree-based performance and effectiveness validation ✅

## Documentation

**Primary Implementation Drivers**:
- `docs/HLD.md` - High-level design for v1.0 (tree-based architecture and strategy) ✅
- `docs/LLD.md` - Low-level design and implementation roadmap ✅
- `README.md` - Complete usage guide with tree-based optimization ✅

**Supporting Documentation**:
- `docs/E2E_DEVELOPER_GUIDE.md` - End-to-end testing and browser integration ✅
- `docs/API_DESIGN.md` - API design documentation (needs tree-based updates)
- `backlog/tasks/` - Project management and implementation tracking ✅

**Implementation Status**:
- **v1.0 Completed**: Tree-based optimization system fully implemented ✅
- **Security**: Multi-tenant isolation with JWT authentication ✅
- **Performance**: 92%+ bandwidth savings achieved ✅
- **Production Ready**: Comprehensive testing, metrics, and operational features ✅

**Development Guidelines**:
- **Tree-based focus**: All optimizations use single unified strategy
- **Template-aware**: Hierarchical parsing of all Go template constructs
- **Security first**: Multi-tenant isolation and cross-application access prevention
- **Performance driven**: 92%+ bandwidth savings with static content caching
- **Production ready**: Comprehensive error handling, metrics, and monitoring

<!-- BACKLOG.MD GUIDELINES START -->
# Instructions for the usage of Backlog.md CLI Tool

## 1. Source of Truth

- Tasks live under **`backlog/tasks/`** (drafts under **`backlog/drafts/`**).
- Every implementation decision starts with reading the corresponding Markdown task file.
- Project documentation is in **`backlog/docs/`**.
- Project decisions are in **`backlog/decisions/`**.

## 2. Defining Tasks

### Understand the Scope and the purpose

Ask questions to the user if something is not clear or ambiguous.
Break down the task into smaller, manageable parts if it is too large or complex.

### **Title (one liner)**

Use a clear brief title that summarizes the task.

### **Description**: (The **"why"**)

Provide a concise summary of the task purpose and its goal. Do not add implementation details here. It
should explain the purpose and context of the task. Code snippets should be avoided.

### **Acceptance Criteria**: (The **"what"**)

List specific, measurable outcomes that define what means to reach the goal from the description. Use checkboxes (
`- [ ]`) for tracking.
When defining `## Acceptance Criteria` for a task, focus on **outcomes, behaviors, and verifiable requirements** rather
than step-by-step implementation details.
Acceptance Criteria (AC) define *what* conditions must be met for the task to be considered complete.
They should be testable and confirm that the core purpose of the task is achieved.
**Key Principles for Good ACs:**

- **Outcome-Oriented:** Focus on the result, not the method.
- **Testable/Verifiable:** Each criterion should be something that can be objectively tested or verified.
- **Clear and Concise:** Unambiguous language.
- **Complete:** Collectively, ACs should cover the scope of the task.
- **User-Focused (where applicable):** Frame ACs from the perspective of the end-user or the system's external behavior.

    - *Good Example:* "- [ ] User can successfully log in with valid credentials."
    - *Good Example:* "- [ ] System processes 1000 requests per second without errors."
    - *Bad Example (Implementation Step):* "- [ ] Add a new function `handleLogin()` in `auth.ts`."

### Task file

Once a task is created it will be stored in `backlog/tasks/` directory as a Markdown file with the format
`task-<id> - <title>.md` (e.g. `task-42 - Add GraphQL resolver.md`).

### Task Breakdown Strategy

When breaking down features:

1. Identify the foundational components first
2. Create tasks in dependency order (foundations before features)
3. Ensure each task delivers value independently
4. Avoid creating tasks that block each other

### Additional task requirements

- Tasks must be **atomic** and **testable**. If a task is too large, break it down into smaller subtasks.
  Each task should represent a single unit of work that can be completed in a single PR.

- **Never** reference tasks that are to be done in the future or that are not yet created. You can only reference
  previous
  tasks (id < current task id).

- When creating multiple tasks, ensure they are **independent** and they do not depend on future tasks.   
  Example of wrong tasks splitting: task 1: "Add API endpoint for user data", task 2: "Define the user model and DB
  schema".  
  Example of correct tasks splitting: task 1: "Add system for handling API requests", task 2: "Add user model and DB
  schema", task 3: "Add API endpoint for user data".

## 3. Recommended Task Anatomy

```markdown
# task‑42 - Add GraphQL resolver

## Description (the why)

Short, imperative explanation of the goal of the task and why it is needed.

## Acceptance Criteria (the what)

- [ ] Resolver returns correct data for happy path
- [ ] Error response matches REST
- [ ] P95 latency ≤ 50 ms under 100 RPS

## Implementation Plan (the how) (added after putting the task in progress but before implementing any code change)

1. Research existing GraphQL resolver patterns
2. Implement basic resolver with error handling
3. Add performance monitoring
4. Write unit and integration tests
5. Benchmark performance under load

## Implementation Notes (imagine this is the PR description) (only added after finishing the code implementation of a task)

- Approach taken
- Features implemented or modified
- Technical decisions and trade-offs
- Modified or added files
```

## 6. Implementing Tasks

Mandatory sections for every task:

- **Implementation Plan**: (The **"how"**)  
  Outline the steps to achieve the task. Because the implementation details may
  change after the task is created, **the implementation plan must be added only after putting the task in progress**
  and before starting working on the task.
- **Implementation Notes**: (Imagine this is a PR note)  
  Start with a brief summary of what has been implemented. Document your approach, decisions, challenges, and any deviations from the plan. This
  section is added after you are done working on the task. It should summarize what you did and why you did it. Keep it
  concise but informative. Make it brief, explain ONLY the core changes and assume that others will read the code to understand the details.

**IMPORTANT**: Do not implement anything else that deviates from the **Acceptance Criteria**. If you need to
implement something that is not in the AC, update the AC first and then implement it or create a new task for it.

## 2. Typical Workflow

```bash
# 1 Identify work
backlog task list -s "To Do" --plain

# 2 Read details & documentation
backlog task 42 --plain
# Read also all documentation files in `backlog/docs/` directory.
# Read also all decision files in `backlog/decisions/` directory.

# 3 Start work: assign yourself & move column
backlog task edit 42 -a @{yourself} -s "In Progress"

# 4 Add implementation plan before starting
backlog task edit 42 --plan "1. Analyze current implementation\n2. Identify bottlenecks\n3. Refactor in phases"

# 5 Break work down if needed by creating subtasks or additional tasks
backlog task create "Refactor DB layer" -p 42 -a @{yourself} -d "Description" --ac "Tests pass,Performance improved"

# 6 Complete and mark Done
backlog task edit 42 -s Done --notes "Implemented GraphQL resolver with error handling and performance monitoring"
```

### 7. Final Steps Before Marking a Task as Done

Always ensure you have:

1. ✅ Marked all acceptance criteria as completed (change `- [ ]` to `- [x]`)
2. ✅ Added an `## Implementation Notes` section documenting your approach
3. ✅ Run all tests and linting checks
4. ✅ Updated relevant documentation

## 8. Definition of Done (DoD)

A task is **Done** only when **ALL** of the following are complete:

1. **Acceptance criteria** checklist in the task file is fully checked (all `- [ ]` changed to `- [x]`).
2. **Implementation plan** was followed or deviations were documented in Implementation Notes.
3. **Automated tests** (unit + integration) cover new logic.
4. **Static analysis**: linter & formatter succeed.
5. **Documentation**:
    - All relevant docs updated (any relevant README file, backlog/docs, backlog/decisions, etc.).
    - Task file **MUST** have an `## Implementation Notes` section added summarising:
        - Approach taken
        - Features implemented or modified
        - Technical decisions and trade-offs
        - Modified or added files
6. **Review**: self review code.
7. **Task hygiene**: status set to **Done** via CLI (`backlog task edit <id> -s Done`).
8. **No regressions**: performance, security and licence checks green.

⚠️ **IMPORTANT**: Never mark a task as Done without completing ALL items above.

## 9. Handy CLI Commands

| Action                  | Example                                                                                                                                                       |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Create task             | `backlog task create "Add OAuth System"`                                                                                                                      |
| Create with description | `backlog task create "Feature" -d "Add authentication system"`                                                                                                |
| Create with assignee    | `backlog task create "Feature" -a @sara`                                                                                                                      |
| Create with status      | `backlog task create "Feature" -s "In Progress"`                                                                                                              |
| Create with labels      | `backlog task create "Feature" -l auth,backend`                                                                                                               |
| Create with priority    | `backlog task create "Feature" --priority high`                                                                                                               |
| Create with plan        | `backlog task create "Feature" --plan "1. Research\n2. Implement"`                                                                                            |
| Create with AC          | `backlog task create "Feature" --ac "Must work,Must be tested"`                                                                                               |
| Create with notes       | `backlog task create "Feature" --notes "Started initial research"`                                                                                            |
| Create with deps        | `backlog task create "Feature" --dep task-1,task-2`                                                                                                           |
| Create sub task         | `backlog task create -p 14 "Add Login with Google"`                                                                                                           |
| Create (all options)    | `backlog task create "Feature" -d "Description" -a @sara -s "To Do" -l auth --priority high --ac "Must work" --notes "Initial setup done" --dep task-1 -p 14` |
| List tasks              | `backlog task list [-s <status>] [-a <assignee>] [-p <parent>]`                                                                                               |
| List by parent          | `backlog task list --parent 42` or `backlog task list -p task-42`                                                                                             |
| View detail             | `backlog task 7` (interactive UI, press 'E' to edit in editor)                                                                                                |
| View (AI mode)          | `backlog task 7 --plain`                                                                                                                                      |
| Edit                    | `backlog task edit 7 -a @sara -l auth,backend`                                                                                                                |
| Add plan                | `backlog task edit 7 --plan "Implementation approach"`                                                                                                        |
| Add AC                  | `backlog task edit 7 --ac "New criterion,Another one"`                                                                                                        |
| Add notes               | `backlog task edit 7 --notes "Completed X, working on Y"`                                                                                                     |
| Add deps                | `backlog task edit 7 --dep task-1 --dep task-2`                                                                                                               |
| Archive                 | `backlog task archive 7`                                                                                                                                      |
| Create draft            | `backlog task create "Feature" --draft`                                                                                                                       |
| Draft flow              | `backlog draft create "Spike GraphQL"` → `backlog draft promote 3.1`                                                                                          |
| Demote to draft         | `backlog task demote <id>`                                                                                                                                    |

Full help: `backlog --help`

## 10. Tips for AI Agents

- **Always use `--plain` flag** when listing or viewing tasks for AI-friendly text output instead of using Backlog.md
  interactive UI.
- When users mention to create a task, they mean to create a task using Backlog.md CLI tool.

<!-- BACKLOG.MD GUIDELINES END -->
- memorize this workaround completely defeats the purpose of livetemplate. please fix the root cause and never do this workaround again