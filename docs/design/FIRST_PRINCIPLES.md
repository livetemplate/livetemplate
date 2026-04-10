# LiveTemplate First Principles

## Core Architectural Principles

### 1. Static/Dynamic Separation is Fundamental
**The tree-based update system separates static (unchanging HTML structure) from dynamic (changing data).**

- First render: Send complete structure WITH statics and dynamics
- Updates: Send ONLY changed dynamics, NO statics (client caches them)
- Trees are ALWAYS generated WITH statics internally (ensures consistent comparison)
- Statics are stripped ONLY for wire transmission via `prepareTreeForClient`
- This achieves 50-90% bandwidth reduction compared to sending full HTML
- **Never compromise this separation** - it's the core optimization

### 2. Tree Structure Invariants are Inviolable
**Tree nodes follow strict structural rules that MUST be maintained.**

- Numeric keys (`"0"`, `"1"`, etc.) for dynamic slots - sequential, no gaps
- `"s"` key for static arrays (template structure)
- `"d"` key exclusively for range construct data
- Empty dynamics represented as empty strings `""`
- These invariants enable efficient client-side updates

### 3. Specification Compliance is Non-Negotiable
**The tree-update-specification.md defines exact behavior that implementations MUST follow.**

- Update sequence rules (first render vs. updates)
- Range operation formats (insert, remove, update, reorder)
- Validation rules (structural invariants)
- Performance requirements (updates < 10% of full render)
- **Never deviate from the specification** without updating the spec itself

### 4. html/template Compatibility is Required
**Public API must match Go's standard html/template package.**

- Methods: `Parse()`, `ParseFiles()`, `ParseGlob()`, `Execute()`
- Template composition: `{{define}}`, `{{template}}`, `{{block}}`
- All constructs: `{{if}}`, `{{range}}`, `{{with}}`, `{{$.Field}}`
- Automatic HTML escaping for security
- Users should be able to drop in LiveTemplate with minimal changes

### 5. Progressive Complexity / Standard HTML First
**Start with standard HTML. Add `lvt-*` only when HTML can't express it.**

- Standard HTML forms work without any `lvt-*` attributes (auto-intercept, auto-submit)
- Action routing uses `button name` and `form name` before `lvt-form:action`
- Template expressions (`{{.Field}}`) in `value=` attributes ARE binding declarations
- `lvt-*` attributes reserved for non-HTML behaviors: debounce, reactive DOM, hooks, loading states
- Three transport modes degrade gracefully: WebSocket → fetch → no-JS POST
- **Never require `lvt-*` for something standard HTML already handles**

### 6. Code Organization Follows Single Responsibility
**Each package has one clear purpose with minimal coupling.**

- `internal/parse/`: Template parsing into constructs (parser.go, constructs.go, compile.go, hydrate.go)
- `internal/build/`: Tree construction and operations (builder.go, tree_ops.go, fingerprint.go)
- `internal/diff/`: Tree comparison and update generation (tree_compare.go, range_ops.go)
- `internal/observe/`: Production-ready logging and metrics
- Core library: Orchestrates internal packages, provides public API
- **Never mix concerns** - parsing doesn't diff, building doesn't parse

### 7. Testing Validates Behavior, Not Implementation
**Tests focus on correctness of tree updates and rendering sequences.**

- E2E tests: Complete rendering sequences (first render → updates)
- Golden files: Expected tree structures and HTML output
- Chromedp browser tests: User interfaces validated in real browsers
- Invariant tests: Tree structure rules never violated
- Fuzz tests: Random inputs don't break invariants
- **Tests MUST access: browser console logs, server logs, WebSocket messages, rendered HTML**

### 8. Kit System Enables CSS Framework Flexibility
**CLI tool supports multiple CSS frameworks via pluggable kits.**

- Cascade priority: Project (.lvt/kits) → User (~/.config/lvt/kits) → System (embedded)
- Kit contains: CSS helpers (~60 methods), components (reusable blocks), templates (generators)
- Four system kits: tailwind, bulma, pico, none
- Component templates use `[[ ]]` delimiters (not `{{ }}`)
- **Always load kits following cascade priority** - never skip levels

### 9. Generator Code is Generic and Reusable
**Code generation creates library code applicable to all use cases.**

- Generated code works for: tests, examples, AND production apps
- No test-specific or example-specific logic in generated code
- Templates in kits drive generation behavior
- Validation happens at generation time, not runtime
- **Never generate code that only works for one use case**

### 10. Security by Default
**Template execution and WebSocket communication are secure by default.**

- HTML escaping via `html/template` - automatic and required
- WebSocket origin validation via AllowedOrigins
- No direct HTML injection - all content through template engine
- Git commits never skip hooks (no --no-verify) unless explicitly requested
- **Security features cannot be disabled accidentally**

### 11. Observability is Production-Ready
**Logging and metrics are built-in and structured.**

- Structured logging via slog (not fmt.Printf)
- Operational metrics for monitoring performance
- Context propagation for tracing
- Clear error messages with actionable information
- **Never use ad-hoc logging** - always use observe package

## Code Change Discipline

### Before Making Changes
1. **Read the specification** - Ensure you understand the contract
2. **Check existing patterns** - Follow established conventions
3. **Identify the right package** - Respect separation of concerns
4. **Consider backward compatibility** - Public API stability matters

### During Implementation
1. **Maintain invariants** - Tree structure rules are sacred
2. **Add tests first** - TDD for new features, regression tests for bugs
3. **Use existing utilities** - Don't reinvent the wheel
4. **Write idiomatic Go** - Follow Go best practices

### After Implementation
1. **Run all tests** - Including E2E tests with chromedp
2. **Validate golden files** - Update if behavior intentionally changed
3. **Check pre-commit hooks** - Fix all failures before committing
4. **Clean up temporary files** - Leave no test artifacts

### Red Flags
- "I'll add a quick workaround" - Usually indicates architectural issue
- "This is just for testing" - Generated code must be production-quality
- "I'll skip the tests for now" - Tests prevent regressions
- "Let me bypass the pre-commit hook" - Hooks enforce quality gates
- "I'll just change the spec" - Spec changes require deep consideration

## Forbidden Practices

1. **Never strip statics during tree generation** - Only strip for wire format
2. **Never skip validation** - Kit manifests, tree structures, template syntax
3. **Never use curl for UI testing** - Use chromedp browser tests
4. **Never commit without running tests** - Pre-commit hooks exist for this
5. **Never mix test-only code with library code** - Keep concerns separate
6. **Never bypass security features** - Origin validation, HTML escaping
7. **Never use ad-hoc string manipulation** - Use parse/build/diff packages
8. **Never ignore golden file failures** - They indicate behavior changes

## Performance Expectations

- Tree generation: O(n) where n = template size
- Diff computation: O(m) where m = changed nodes
- Update size: < 10% of full render (50-90% reduction typical)
- Fingerprint comparison: O(1)
- Range operations: Affect only changed items, never full list

## When to Update This Document

Add new principles when:
- Discovering a fundamental architectural decision
- Establishing a new convention that affects multiple components
- Identifying a common mistake that needs prevention

Principles should be:
- **Fundamental** - Not tactical implementation details
- **Actionable** - Clear guidance for developers
- **Testable** - Possible to verify compliance
- **Essential** - Violating them creates serious problems
