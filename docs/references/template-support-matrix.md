# LiveTemplate Go Template Support Matrix

## Overview

This document provides a comprehensive matrix of Go template patterns and their support status in LiveTemplate's AST-based template parser. All patterns have been systematically tested through 6 phases of fuzz testing with 60+ explicit test seeds and 2150+ baseline coverage patterns.

**Test Coverage**: ~95% of Go template features
**Fuzz Testing**: Multi-layer infrastructure with TypeScript oracle validation, mutation testing, and invariant checking
**Last Updated**: 2026-03-07

---

## Support Status Legend

- ✅ **Fully Supported**: Pattern works correctly, extensively tested
- ⚠️ **Limited Support**: Works with caveats or edge cases
- ❌ **Not Supported**: Pattern does not work or is not applicable

---

## Template Pattern Support Matrix

### 1. Actions & Basic Syntax

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Simple field output | ✅ | Baseline | `{{.Name}}` | Core feature |
| Nested field access | ✅ | Baseline | `{{.User.Name}}` | Tested in multiple contexts |
| Template comments | ✅ | Phase 5 | `{{/* comment */}}` | Properly ignored during parsing |
| HTML comments | ✅ | Phase 1 | `<!-- comment -->` | Stripped to match `html/template` (see note below) |
| Empty templates | ✅ | Phase 5 | `` (empty string) | Handles gracefully |
| Comment-only templates | ✅ | Phase 5 | `{{/* only comment */}}` | Valid template |

#### HTML comment stripping

`html/template` removes HTML comments (`<!-- ... -->`) during its escape pass.
LiveTemplate builds its static segments by walking the raw parse tree, which
never triggers that pass, so comments are stripped explicitly at parse time
(`render.StripHTMLComments`, applied in `parseInternal`) to keep output
consistent with `html/template` and to avoid leaking developer/internal
comments to the client. A comment that wraps an action (`<!-- {{.X}} -->`) is
removed in full, action included — matching `html/template`.

Stripping uses the HTML tokenizer (not a regex), so it is context-aware:

- A literal `<!--` inside a `{{...}}` action (e.g. `{{"<!--"}}`, a
  `{{/* <!-- */}}` template comment, or a quoted argument) is **preserved** and
  never mistaken for an HTML comment — action spans are masked out before
  stripping, matching `html/template`, which never strips inside an action.
- Comment-like text inside an attribute value (`title="<!-- x -->"`) is real
  content and is **preserved**.
- Comment markup inside `<script>`, `<style>`, and `<textarea>` (RAWTEXT /
  RCDATA) is **left verbatim**, since the tokenizer does not treat it as a
  comment there. Note two residual divergences from `html/template` in these
  contexts (both pre-existing, unchanged by the stripping): in `<script>`,
  `html/template` rejects `<!--` outright; in `<textarea>`/RCDATA it escapes
  `<!--` to `&lt;!--`. LiveTemplate preserves both verbatim.

### 2. Whitespace Handling

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Leading trim | ✅ | Phase 5 | `{{- .Field}}` | Removes preceding whitespace |
| Trailing trim | ✅ | Phase 5 | `{{.Field -}}` | Removes following whitespace |
| Both sides trim | ✅ | Phase 5 | `{{- .Field -}}` | Trims both sides |
| Trim in ranges | ✅ | Phase 5 | `{{range .Items -}}\n{{end}}` | Works correctly |
| Negative number | ✅ | Phase 5 | `{{-3}}` | Parsed as number, not trim |
| Trim with space | ✅ | Phase 5 | `{{- 3}}` | Correctly identified as trim |

### 3. Control Flow: Conditionals

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Simple if | ✅ | Baseline | `{{if .Show}}yes{{end}}` | Basic conditional |
| If-else | ✅ | Baseline | `{{if .Show}}yes{{else}}no{{end}}` | Both branches |
| Else-if chains | ✅ | Phase 2 | `{{if .A}}a{{else if .B}}b{{else}}c{{end}}` | Multiple conditions |
| Nested if (2 levels) | ✅ | Baseline | `{{if .A}}{{if .B}}nested{{end}}{{end}}` | Tested extensively |
| Nested if (10 levels) | ✅ | Deep nesting | Tested up to 10 levels | AST parser handles any depth |
| If with complex condition | ✅ | Phase 6 | `{{if gt (len .Items) 0}}...{{end}}` | Function calls in condition |

### 4. Control Flow: Ranges

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Simple range | ✅ | Baseline | `{{range .Items}}{{.}}{{end}}` | Core iteration |
| Range with else | ✅ | Phase 1 | `{{range .Items}}item{{else}}empty{{end}}` | Empty state handling |
| Range with index | ✅ | Phase 3 | `{{range $i, $v := .Items}}...{{end}}` | Variable declarations |
| Range with value only | ✅ | Phase 3 | `{{range $v := .Items}}...{{end}}` | Single variable |
| Map range | ✅ | Phase 1 | `{{range $k, $v := .Map}}...{{end}}` | Key-value iteration |
| Nested ranges | ✅ | Phase 2 | `{{range .Outer}}{{range .Inner}}...{{end}}{{end}}` | 2+ levels |
| Range + nested if | ✅ | Phase 2 | `{{range .Items}}{{if .Active}}...{{end}}{{end}}` | Combined patterns |
| Empty slice | ✅ | Phase 1 | Data: `[]string{}` | Properly handles empty |
| Nil slice | ✅ | Phase 1 | Data: `([]string)(nil)` | Correctly treats as empty |
| Break statement | ✅ | Phase 2 | `{{range .Items}}{{if ...}}{{break}}{{end}}{{end}}` | Go 1.18+ |
| Continue statement | ✅ | Phase 2 | `{{range .Items}}{{if ...}}{{continue}}{{end}}{{end}}` | Go 1.18+ |

### 5. Control Flow: With

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Simple with | ✅ | Baseline | `{{with .User}}Hello {{.Name}}{{end}}` | Context switching |
| With-else | ✅ | Phase 2 | `{{with .User}}user{{else}}no user{{end}}` | Nil handling |
| Nil value with | ✅ | Phase 1 | Data: `nil` | Else branch triggered |
| Empty string with | ✅ | Phase 2 | Data: `""` | Falsy value |
| With + nested if | ✅ | Phase 2 | `{{with .User}}{{if .Active}}...{{end}}{{end}}` | Complex nesting |

### 6. Variables

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Variable declaration | ✅ | Phase 3 | `{{$x := .Value}}{{$x}}` | Basic assignment |
| Context alias | ✅ | Phase 3 | `{{$c := .}}{{$c.Field}}` | Alias for dot context; see `internal/parse/var_decl_test.go` |
| Variable reassignment | ✅ | Phase 3 | `{{$x := ""}}{{$x = "new"}}{{$x}}` | In if/else blocks |
| Multiple variables | ✅ | Phase 3 | `{{$a := .A}}{{$b := .B}}{{$a}}{{$b}}` | Multiple declarations |
| Range variables | ✅ | Phase 3 | `{{range $i, $v := .Items}}{{$i}}: {{$v}}{{end}}` | Index and value |
| Variable shadowing | ✅ | Phase 3 | `{{$v := .Name}}{{range .Items}}{{$v := .}}{{end}}` | Scoped correctly |
| Parent context access | ✅ | Phase 1, 3 | `{{range .Items}}{{$.Title}}{{end}}` | `$` for root |
| Nested context access | ✅ | Phase 3 | `{{range}}{{range}}{{$$. Root}}{{end}}{{end}}` | Double `$` |

### 7. Pipelines & Functions

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Simple pipeline | ✅ | Baseline | `{{.Name \| printf "User: %s"}}` | Function call |
| Function with format | ✅ | Phase 6 | `{{.Value \| printf "%d"}}` | Type-specific formatting |
| Comparison: eq | ✅ | Phase 6 | `{{if eq .A .B}}equal{{end}}` | Equality |
| Comparison: ne | ✅ | Phase 6 | `{{if ne .A .B}}not equal{{end}}` | Inequality |
| Comparison: lt | ✅ | Phase 6 | `{{if lt .Count 10}}small{{end}}` | Less than |
| Comparison: gt | ✅ | Phase 6 | `{{if gt .Count 10}}large{{end}}` | Greater than |
| Comparison: le | ✅ | Phase 6 | `{{if le .Count 5}}...{{end}}` | Less or equal |
| Comparison: ge | ✅ | Phase 6 | `{{if ge .Count 5}}...{{end}}` | Greater or equal |
| Logical: and | ✅ | Phase 6 | `{{if and .A .B}}both{{end}}` | Boolean AND |
| Logical: or | ✅ | Phase 6 | `{{if or .A .B}}either{{end}}` | Boolean OR |
| Logical: not | ✅ | Phase 6 | `{{if not .Empty}}has value{{end}}` | Boolean NOT |
| Function: len | ✅ | Phase 6 | `{{len .Items}}`, `{{len .Name}}` | Length/count |
| Function: index | ✅ | Phase 6 | `{{index .Items 0}}` | Array access |
| Complex expression | ✅ | Phase 6 | `{{if and (gt .Count 0) (lt .Count 10)}}...{{end}}` | Nested functions |

### 8. Data Types

| Data Type | Status | Phase | Example | Notes |
|-----------|--------|-------|---------|-------|
| String | ✅ | All phases | `"TestName"` | Primary type |
| String slices | ✅ | All phases | `[]string{"a", "b", "c"}` | Iteration support |
| Integer | ✅ | Phase 4 | `5`, `42` | Numbers |
| Integer slices | ✅ | Phase 4 | `[]int{1, 2, 3}` | Numeric iteration |
| Boolean | ✅ | All phases | `true`, `false` | Conditionals |
| Boolean slices | ✅ | Phase 4 | `[]bool{true, false}` | Boolean iteration |
| Maps (string→string) | ✅ | Phase 1, 4 | `map[string]string` | Key-value |
| Maps (generic) | ✅ | Phase 4 | `map[string]interface{}` | Any value type |
| Structs | ✅ | All phases | Custom struct types | Field access |
| Nested structs | ✅ | Phase 4 | `User.Profile.Bio` | Deep nesting |
| Interfaces | ✅ | Phase 4 | `[]interface{}{"str", 42, true}` | Mixed types |
| Nil values | ✅ | Phase 1 | `nil` | Proper handling |
| Nil slices | ✅ | Phase 1 | `([]string)(nil)` | Distinct from empty |
| Nil pointers | ✅ | Phase 4 | `(*string)(nil)` | Safe dereference |
| Empty collections | ✅ | Phase 1 | `[]string{}`, `map[string]string{}` | Edge cases |

### 9. Mixed Template Patterns

| Pattern | Status | Phase | Example | Notes |
|---------|--------|-------|---------|-------|
| Range + other dynamics | ✅ | Phase 1 | `{{.Title}}{{range .Items}}...{{end}}{{.Footer}}` | **Critical fix** |
| Multiple ranges | ✅ | Phase 1 | Multiple `{{range}}` blocks in template | Independent ranges |
| If wrapping range | ✅ | Baseline | `{{if .Show}}{{range .Items}}...{{end}}{{end}}` | Tested extensively |
| Range + if + with | ✅ | Phase 2 | Complex 3-way nesting | All combinations work |

---

## Template Composition

| Pattern | Status | Notes |
|---------|--------|-------|
| `{{define}}` / `{{template}}` | ✅ | Automatically flattened via `FlattenTemplate()` in `internal/parse/flatten.go` |
| `{{block}}` | ✅ | Resolved during flattening; equivalent to `{{define}}` + `{{template}}` |
| Recursive / circular template references | ✅ | Supported since v0.19.0 — direct self-recursion (`node -> node`), mutual recursion (`a -> b -> a`), longer cycles, and a self-referential entry point all render. `{{template}}` calls are normally inlined during flattening, which a cycle cannot survive, so `FlattenTemplate` detects templates reachable from themselves and leaves those invocations un-inlined, evaluating them at build time as nested `TreeNode`s. The recursive region stays inside the reactive tree, so updates diff per-leaf rather than re-sending the branch. Depth is capped — see [Recursion depth](#recursion-depth). |
| Undefined template invocation | ❌ | Returns error from Go template engine |

Template composition is fully supported through automatic AST flattening. `FlattenTemplate()` walks the parsed template tree, identifies the entry point, and inlines all `{{template}}` invocations into a single flat template before tree generation. Templates that are reachable from themselves are the exception: they are left un-inlined and invoked at build time instead.

### Recursion depth

Recursive invocation is capped so that self-referential *data* (a node whose child list contains itself) surfaces an error instead of overflowing the stack. The cap defaults to **128** and is configurable:

```go
lt.New("tree", lt.WithMaxTemplateDepth(256))
```

```bash
export LVT_MAX_TEMPLATE_DEPTH=256   # must be a positive integer
```

Exceeding the cap on a **reactive update** returns a build-phase `ParseError` naming the template and the limit. Raise the cap only if your data is legitimately deeper — the default exists to catch cycles in data, not to constrain real nesting.

Two behaviours differ from that and are worth knowing:

- **First render with finite data deeper than the cap does not error.** The build degrades and still produces a usable, keyed page, so a too-low cap can go unnoticed until the first update fails.
- **If the initial tree build errors outright** — genuinely self-referential data, where a node's children contain the node itself — the initial render falls back to an HTML-structure-based tree and logs `Template parsing failed, falling back to HTML structure-based tree`. That region then stays on HTML-string diffing for the life of the template rather than the reactive path. A recursive region that renders but never updates reactively is the symptom; that warning in the server log is the confirmation.

## Custom Functions

| Pattern | Status | Notes |
|---------|--------|-------|
| Built-in Go functions | ✅ | All standard functions supported |
| User-defined functions | ⚠️ | Must be registered with Go template engine |
| Zero-arg method calls on data | ✅ | Public methods; precomputed from State (`{{.Count}}`) |
| Arg-accepting method calls | ⚠️ | Only on a struct **field**, not top-level State — `{{.Views.Class .ID}}`, not `{{.Class .ID}}` (see [Controller Pattern — view helpers](controller-pattern.md#methods-that-take-arguments-view-helpers)) |

## Performance Considerations

| Scenario | Performance | Notes |
|----------|-------------|-------|
| Deep nesting (10+ levels) | ✅ Good | AST parser handles any depth |
| Large lists (1000+ items) | ✅ Good | Efficient tree generation |
| Complex mixed patterns | ✅ Good | Optimized for real-world templates |
| Whitespace-heavy templates | ✅ Good | Trimming works efficiently |

---

## Testing Coverage by Phase

| Phase | Patterns Added | Cumulative Total | Key Features |
|-------|----------------|------------------|--------------|
| Baseline | 10 seeds | 10 | Core field access, basic ranges/conditionals |
| Phase 1 | 10 seeds | 20 | Mixed templates, empty states, map ranges |
| Phase 2 | 10 seeds | 30 | Break/continue, else-if, nested ranges, with-else |
| Phase 3 | 6 seeds | 36 | Variable scope, parent context, shadowing |
| Phase 4 | 6 seeds | 42 | Maps, int/bool slices, mixed types, pointers |
| Phase 5 | 8 seeds | 50 | Whitespace trimming, edge cases, empty templates |
| Phase 6 | 11 seeds | 61 | Pipelines, comparison/logical functions, index/len |

**Total Baseline Coverage**: 2150 interesting inputs (after fuzzing)
**Fuzz Executions**: Ongoing via multi-layer infrastructure (diff fuzzing, TypeScript oracle, mutation testing)
**Failures**: 0 crashes, 0 structural errors

---

## Validation Levels

LiveTemplate employs multi-level validation for fuzz testing:

### Level 1: Structure Validation ✅
- Verifies tree has required `"s"` (statics) key
- Checks basic tree structure validity

### Level 2: Render Validation ✅
- Attempts to reconstruct HTML from tree
- Validates semantic correctness
- Ensures tree is not just syntactically valid but also renderable

### Level 3: Round-Trip Validation ✅
- Parse → Render → Parse → Compare
- Ensures bidirectional consistency
- Uses deterministic `orderedVars` structure (`internal/parse/vars.go`) and sorted TreeNode iteration

### Level 4: Transition Validation ✅
- Tests empty→non-empty state changes
- Validates dynamic updates work correctly
- Both empty and non-empty trees must be valid and renderable

---

## Common Patterns & Best Practices

### ✅ Recommended Patterns

**1. Mixed Templates (ranges + other dynamics)**
```html
<h1>{{.Title}}</h1>
{{range .Items}}
  <div>{{.Name}}</div>
{{end}}
<footer>{{.Footer}}</footer>
```
**Works perfectly** after Phase 1 fix.

**2. Empty State Handling**
```go
{{range .Items}}
  <div>Item: {{.}}</div>
{{else}}
  <p>No items to display</p>
{{end}}
```
**Best practice** for user-friendly empty states.

**3. Variable Scoping**
```go
{{$title := .Title}}
{{range .Items}}
  <div>{{$title}}: {{.}}</div>
{{end}}
```
**Efficient** for accessing outer context.

**4. Parent Context Access**
```go
{{range .Items}}
  <div>{{$.SiteTitle}} - {{.Name}}</div>
{{end}}
```
**Alternative** to variable declarations.

**5. Template Composition**
```go
{{define "user"}}
  <div>{{.Name}} - {{.Email}}</div>
{{end}}
{{template "user" .CurrentUser}}
```
**Fully supported** — automatically flattened before tree generation.

### ⚠️ Patterns to Avoid

**1. Deeply Nested Variables**
```go
{{range}}{{range}}{{range}}{{$$$.Field}}{{end}}{{end}}{{end}}
```
Use intermediate variables instead.

---

## References

- **Fuzz Framework Design**: `docs/fuzz-framework-design.md`
- **Go text/template**: https://pkg.go.dev/text/template
- **Go html/template**: https://pkg.go.dev/html/template
- **Fuzz Tests**: `fuzz_diff_test.go`, `fuzz_ts_oracle_test.go`
- **Fuzz Infrastructure**: `internal/fuzz/` (app, invariants, mutations, generators)
- **Parser Implementation**: `internal/parse/` package
- **Variable Context**: `internal/parse/vars.go`
- **Template Flattening**: `internal/parse/flatten.go`

---

## Maintenance

**Document Owner**: LiveTemplate Core Team
**Last Updated**: 2026-03-07
**Next Review**: When new Go template features are added
**Feedback**: Report issues or unsupported patterns via GitHub issues

---

## Summary

LiveTemplate's parser provides **comprehensive support** for Go template patterns with:

- ✅ **60+ explicitly tested patterns** across all Go template constructs
- ✅ **Full template composition** via automatic AST flattening
- ✅ **Variable declarations** including `$c := .` context aliasing
- ✅ **Multi-layer fuzz testing** with TypeScript oracle validation
- ✅ **Production-ready** with extensive testing and 0 crashes

All standard Go template patterns are supported, including template composition (`{{define}}`/`{{template}}`/`{{block}}`). Performance is excellent across all tested scenarios.
