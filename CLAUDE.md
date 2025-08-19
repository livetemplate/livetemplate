# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LiveTemplate is a Go library for ultra-efficient HTML template update generation using HTML diffing-enhanced four-tier strategy selection. It analyzes actual HTML changes to select optimal strategies: Static/Dynamic (85-95% for text-only changes) → Markers (70-85% for position-discoverable) → Granular (60-80% for simple structural) → Replacement (40-60% for complex changes).

**Current Status**: v1.0 implementation completed with Application/Page architecture, JWT security, and production load testing
**Target**: First public release (v1.0) with HTML diffing-enhanced four-tier strategy selection ✅ COMPLETED
**Next**: Complete HTML diffing implementation for optimal strategy selection accuracy
**Future**: Advanced optimization and enhanced features in v2.0+

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

## Architecture (v1.0 Target)

The library is being redesigned with a two-layer architecture focused on security and reliability:

**Security Foundation**:
1. **Application** - Multi-tenant isolation with JWT-based authentication
2. **Page** - Isolated user sessions with stateless design for horizontal scaling  
3. **TokenService** - Standard JWT tokens with replay protection
4. **PageRegistry** - Thread-safe page storage with TTL cleanup

**HTML Diffing-Enhanced Strategy System**:
1. **HTMLDiffer** - Analyzes rendered HTML changes to identify patterns
2. **StrategyAnalyzer** - Selects optimal strategy based on HTML diff analysis
3. **StaticDynamicAnalyzer** - Strategy 1: Text-only changes (60-70% of cases)
4. **MarkerCompiler** - Strategy 2: Position-discoverable changes (15-20% of cases)
5. **GranularAnalyzer** - Strategy 3: Simple structural changes (10-15% of cases)
6. **FragmentExtractor** - Strategy 4: Complex structural changes (5-10% of cases)
7. **UpdateGenerator** - Multi-strategy update generation with diff insights
8. **DiffEngine** - Data change detection enhanced with HTML pattern analysis

**Current Components** (legacy, to be replaced):
- `realtime_renderer.go` - Being replaced by Application/Page architecture
- `template_tracker.go` - Being replaced by DiffEngine with HTML diff analysis
- `fragment_extractor.go` - Being redesigned for HTML diffing-enhanced strategy support
- `advanced_analyzer.go` - Being replaced by StrategyAnalyzer with HTML diffing logic

## API Design (v1.0 Target)

The v1.0 API focuses on secure multi-tenant architecture with zero-configuration:

### Core Types (v1.0)
```go
type Application struct { /* private fields */ }
type Page struct { /* private fields */ }
type Fragment struct {
    ID       string      `json:"id"`
    Strategy string      `json:"strategy"`   // "static_dynamic", "markers", "granular", "replacement"
    Action   string      `json:"action"`     // Strategy-specific action
    Data     interface{} `json:"data"`       // Strategy-specific payload
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

## HTML Diffing-Enhanced Four-Tier Strategy (v1.0)

LiveTemplate v1.0 implements **HTML Diffing-Based Strategy Selection** for maximum efficiency:

**Deterministic HTML Diffing Approach**:
- **Render Both Versions**: Template + oldData → oldHTML, Template + newData → newHTML
- **Analyze Changes**: Compare actual HTML to identify change patterns
- **Rule-Based Classification**: Text-only, attribute, structural, or complex changes
- **Deterministic Strategy Selection**: Same change pattern always chooses same strategy (predictable behavior)

**Critical Design Principle**: Strategy selection is **deterministic** and **rule-based**. This ensures:
- Library users can predict which strategy will be used
- Same template constructs always behave consistently  
- Performance is predictable and debuggable
- Deterministic rule-based thresholds

**Deterministic Strategy Rules**:
1. **Text-only changes** → Always Strategy 1 (Static/Dynamic)
2. **Attribute changes** → Always Strategy 2 (Markers)
3. **Structural changes** → Always Strategy 3 (Granular)
4. **Mixed change types** → Always Strategy 4 (Replacement)

**Strategy 1: Static/Dynamic** (85-95% reduction - 60-70% of cases):
- **When**: Pure text content changes, HTML structure identical
- **Rule**: `hasText && !hasAttribute && !hasStructural`
- **Example**: `<span>John</span>` → `<span>Jane</span>` (text-only change)
- **Benefit**: Extreme bandwidth efficiency, no HTML transmission

**Strategy 2: Marker Compilation** (70-85% reduction - 15-20% of cases):
- **When**: Attribute changes (with or without text changes)
- **Rule**: `hasAttribute && !hasStructural`
- **Example**: `<div class="old">Text</div>` → `<div class="new">Text</div>` (attribute change)
- **Benefit**: Precise value patching despite structural complexity

**Strategy 3: Granular Operations** (60-80% reduction - 10-15% of cases):
- **When**: Pure structural changes (no text/attribute changes)
- **Rule**: `hasStructural && !hasText && !hasAttribute`
- **Example**: `<ul><li>A</li></ul>` → `<ul><li>A</li><li>B</li></ul>` (element addition)
- **Benefit**: Efficient operations without full replacement

**Strategy 4: Fragment Replacement** (40-60% reduction - 5-10% of cases):
- **When**: Complex mixed changes (structural + text/attribute)
- **Rule**: `hasStructural && (hasText || hasAttribute)`
- **Example**: Complete layout changes with mixed content modifications
- **Benefit**: Guaranteed compatibility for any complexity

**HTML Diff Pattern Examples**:
```html
<!-- Text-Only Change → Strategy 1 -->
OldHTML: <div class="alert-info">Server maintenance</div>
NewHTML: <div class="alert-warning">Database issue</div>
Pattern: TEXT_CHANGES_ONLY
Strategy: Static/Dynamic

<!-- Element Addition → Strategy 3 -->
OldHTML: <ul><li>Task 1</li></ul>
NewHTML: <ul><li>Task 1</li><li>Task 2</li></ul>
Pattern: ELEMENT_APPEND
Strategy: Granular Operation

<!-- Complex Rewrite → Strategy 4 -->
OldHTML: <div><span>User: John</span></div>
NewHTML: <table><tr><td>John</td><td>Admin</td></tr></table>
Pattern: STRUCTURAL_REWRITE
Strategy: Fragment Replacement
```

**Data-Driven Selection Benefits**:
- **High Accuracy**: >90% optimal strategy selection vs template guessing
- **Performance Optimization**: Always selects most efficient viable approach
- **Predictable Distribution**: Known percentages for capacity planning
- **Pattern Learning**: Can optimize based on actual usage patterns

## Testing Strategy (v1.0 Focus)

**Test-Driven Development**:
- Red-Green-Refactor cycle for all new features
- Security tests must pass for v1.0 release  
- Performance benchmarks validate 40-60% bandwidth reduction
- Comprehensive error handling and edge case coverage

**Test Categories**:
- **Unit Tests** (60%): Individual component behavior
- **Integration Tests** (30%): Component interaction and workflows
- **Security Tests** (10%): Multi-tenant isolation and JWT authentication

**Critical Test Requirements for v1.0**:
- Zero cross-application data leaks in testing
- JWT implementation passes security audit
- HTML diffing engine >95% accuracy in change pattern recognition
- Strategy 1 (Static/Dynamic) achieves 85-95% size reduction for text-only changes
- Strategy 2 (Markers) achieves 70-85% size reduction for position-discoverable changes
- Strategy 3 (Granular) achieves 60-80% size reduction for simple structural changes
- Strategy 4 (Replacement) achieves 40-60% size reduction for complex changes
- HTML diff-based strategy selection accuracy >90% across all change patterns
- Strategy distribution matches expected percentages (60-70%, 15-20%, 10-15%, 5-10%)
- Memory usage bounded under concurrent load (1000+ pages)
- P95 update generation latency <75ms (includes HTML diffing overhead)

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
**Phase 1 (Immediate Start)**: Security Foundation
- Application isolation with JWT tokens
- Page lifecycle management  
- Fragment replacement updates
- Memory management and cleanup

**Phase 2 (Following Phase 1)**: HTML Diffing-Enhanced Four-Tier System
- HTML diffing engine for change pattern analysis
- Strategy 1: Static/dynamic generation for text-only changes
- Strategy 2: Marker compilation for position-discoverable changes
- Strategy 3: Granular operations for simple structural changes
- Strategy 4: Fragment replacement for complex structural changes
- HTML diff-based strategy selection with pattern recognition
- Update optimization based on HTML diff insights

**Phase 3 (Final Implementation)**: Production Features
- Memory management and cleanup
- Simple built-in metrics collection (no external dependencies)
- Operational readiness (health checks, logging)

**LLM Development Approach**:
- **Immediate implementation**: No waiting periods between phases
- **Focused sessions**: Complete related tasks in single development iterations
- **Test-driven**: Generate comprehensive tests alongside implementation
- **Parallel development**: Work on multiple components simultaneously when dependencies allow

### HTML Diffing-Enhanced Updates (v1.0 Scope)
- **HTML Diff Analysis**: Render old + new HTML, analyze change patterns
- **Pattern-Based Selection**: Strategy based on actual changes, not template prediction
- **Strategy 1**: `"static_dynamic"` - Text-only changes (60-70% of cases)
- **Strategy 2**: `"markers"` - Position-discoverable changes (15-20% of cases)
- **Strategy 3**: `"granular"` - Simple structural changes (10-15% of cases)
- **Strategy 4**: `"replacement"` - Complex structural changes (5-10% of cases)
- **Fragment IDs**: Deterministic generation based on template + diff signature
- **Confidence Scoring**: Track accuracy of change detection (quality metric, not used for strategy selection)
- **Batching**: Multiple fragment updates grouped by compatible diff patterns
- **Error Handling**: Failed strategy triggers automatic fallback based on complexity
- **Guaranteed Compatibility**: HTML diff analysis + four-tier fallback ensures all templates work

### Error Handling Strategy
- Sentinel errors for common cases (`ErrPageNotFound`, `ErrInvalidApplication`)
- Comprehensive error context with structured logging
- Graceful degradation under memory/load pressure
- Security-focused error messages (no information leakage)

## Performance Considerations (v1.0 Targets)

**v1.0 Performance Goals** (HTML Diffing-Enhanced Strategy):
- Strategy 1: 85-95% size reduction for text-only changes (60-70% of templates)
- Strategy 2: 70-85% size reduction for position-discoverable changes (15-20% of templates)
- Strategy 3: 60-80% size reduction for simple structural changes (10-15% of templates)
- Strategy 4: 40-60% size reduction for complex changes (5-10% of templates)
- HTML diff-based strategy selection accuracy: >90% optimal choice
- HTML diffing pattern recognition: >95% accuracy in strategy classification
- P95 update generation latency <75ms (includes HTML diffing overhead)
- Support 1000 concurrent pages per instance (with 8GB RAM)
- Memory usage <12MB per page (HTML diffing + strategy caching overhead)

**Implementation Guidelines**:
- Parse templates once at startup, analyze HTML diff patterns
- HTML diff results cached by template hash + data signature
- Strategy selection results cached for repeated change patterns
- Static segment caching optimized for Strategy 1 efficiency
- Marker position mapping cached for Strategy 2 reuse
- Granular operation patterns cached for Strategy 3 optimization
- Fragment replacement minimized through accurate HTML diff analysis
- JWT token validation optimized for high throughput  
- Update batching with 16ms windows (60fps alignment)
- Memory cleanup with TTL-based page expiration

**Measurement and Monitoring**:
- Simple built-in metrics (no external dependencies)
- Real-world usage data collection for v2.0 optimization decisions
- Performance regression testing in CI pipeline
- Memory leak detection and alerting

**Deferred Optimizations** (v2.0+):
- Advanced value patch optimizations (nested object patching, array diffs)
- Enhanced template analysis for edge cases
- High-frequency update optimizations (>1000 updates/second) 
- Complex memory management strategies
- Client-side update application optimizations

## Code Quality Standards

- All public methods are thread-safe
- No code comments unless absolutely necessary for complex logic
- Follow Go best practices and idioms
- Use golangci-lint for code quality enforcement
- Maintain test coverage for all new functionality

## Project Structure (v1.0 Target)

```
livetemplate/
├── internal/                # Internal implementation (hidden from public API)
│   ├── app/                # Application isolation and lifecycle
│   ├── page/               # Page session management
│   ├── token/              # JWT token service  
│   ├── diff/               # HTML diffing engine and pattern analysis
│   ├── strategy/           # HTML diff-based strategy selection
│   ├── fragment/           # Fragment extraction and update generation
│   ├── metrics/            # Simple built-in metrics (no dependencies)
│   └── memory/             # Memory management and cleanup
├── examples/               # Usage examples and demos
├── docs/                   # Comprehensive documentation  
│   ├── HLD.md             # High-level design (implementation driver)
│   └── LLD.md             # Low-level design (60-task roadmap)
├── testdata/              # Test templates and data
└── scripts/               # Development and validation scripts

# Legacy files (to be replaced in v1.0):
├── realtime_renderer.go    # Being replaced by internal/app + internal/page
├── template_tracker.go     # Being replaced by internal/fragment (UpdateGenerator)  
├── fragment_extractor.go   # Being redesigned for HTML diffing-enhanced strategy support
└── advanced_analyzer.go    # Being replaced by internal/diff (HTMLDiffer) + internal/strategy (StrategyAnalyzer)
```

## Implementation Guidance

**When implementing v1.0 components**:
1. **Follow HLD.md and LLD.md specifications exactly**
2. **Security first**: All cross-application access must be blocked
3. **TDD required**: Write failing tests before implementation
4. **Performance targets**: Measure and validate three-tier strategy performance (70-85% value patch, 60-80% granular ops, 40-60% fragment replace)
5. **Strategy selection**: Implement template analysis for automatic three-tier optimization
6. **Error handling**: Comprehensive error context without information leakage

**Priority order for implementation**:
1. **Internal package structure** with proper encapsulation
2. **JWT token service** with replay protection (security foundation)
3. **Application/Page architecture** with multi-tenant isolation
4. **HTMLDiffer** with pattern analysis and change detection
5. **StrategyAnalyzer** with HTML diff-based selection
6. **Four-tier update system** - data-driven strategy selection based on actual HTML changes
7. **Strategy-specific optimizations** enhanced with HTML diff insights
7. **Memory management** and resource cleanup
8. **Simple metrics collection** for operational readiness (no external dependencies)

**Testing requirements**:
- Security tests must pass (zero cross-application access)
- HTML diffing engine >95% accuracy in change pattern recognition
- Performance benchmarks meet HTML diff-enhanced strategy targets (85-95% Strategy 1, 70-85% Strategy 2, 60-80% Strategy 3, 40-60% Strategy 4)
- HTML diff-based strategy selection accuracy >90% across all change patterns
- Strategy distribution matches expected percentages (60-70%, 15-20%, 10-15%, 5-10%)
- All components >90% unit test coverage
- Integration tests for complete HTML diff → strategy selection → update generation workflows
- Memory leak detection and cleanup validation
- HTML diffing performance and strategy effectiveness validation

## Documentation

**Primary Implementation Drivers**:
- `docs/HLD.md` - High-level design for v1.0 (architectural decisions and strategy)
- `docs/LLD.md` - Low-level design for v1.0 (75-task implementation roadmap)

**Supporting Documentation**:
- `docs/ARCHITECTURE.md` - Legacy architectural overview (reference only)
- `docs/API_DESIGN.md` - Legacy API design (being replaced by HLD.md v1.0 API)
- `docs/EXAMPLES.md` - WebSocket integration examples (will be updated for v1.0)
- `README.md` - Complete usage guide (will be updated for v1.0)

**Implementation Status Tracking**:
- Follow the 60-task breakdown in `docs/LLD.md` for immediate LLM-assisted development
- Phase 1 (Tasks 1-30): Security foundation with JWT and Application/Page architecture
- Phase 2 (Tasks 31-50): HTML diffing-enhanced four-tier strategy system
- Phase 3 (Tasks 51-60): Production features with monitoring and operational readiness

**LLM Implementation Guidelines**:
- **Start immediately** with Phase 1 security foundation
- **Complete related tasks together** in focused development sessions
- **Generate tests first** following TDD methodology  
- **Validate security requirements** before proceeding to next phase
- **Implement internal packages** with proper encapsulation from the start
- **Focus on production readiness** - comprehensive error handling, simple metrics, and monitoring

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
