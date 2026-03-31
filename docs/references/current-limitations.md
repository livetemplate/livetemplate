# Current Limitations

Known limitations of LiveTemplate, organized by category. Each entry includes the impact, workaround, and current status. For planned improvements, see the [Roadmap](../../ROADMAP.md).

All limitations verified against the current codebase.

---

## Template Features

These Go template constructs trigger a fallback to HTML segmentation, which produces coarser diffs (no range operations, larger update payloads). Templates still render correctly — only the diff granularity is affected.

| Construct | Workaround | Status |
|-----------|-----------|--------|
| Dynamic template indirection (`{{template (printf ...)}}`) | Use static template names | By design (fallback) |
| Channel ranges (`{{range .Stream}}`) | Collect channel to slice before passing to template | Blocked on Go templates |
| Integer literal ranges (`{{range 3}}`) | Range over a pre-built slice | Blocked on Go templates |
| `{{break}}` / `{{continue}}` (Go 1.23+) | Restructure template logic to avoid control flow | Planned — LiveTemplate parser doesn't yet handle BreakNode/ContinueNode |
| `{{block}}` with dynamic template names | Use `{{template "name" .}}` with static names | By design (fallback) |
| `iter.Seq` ranges | Collect iterator to slice before passing to template | Blocked on Go templates |

See [HTML Fallback Coverage](../roadmap/html-fallback-coverage.md) for test coverage details and [Template Support Matrix](template-support-matrix.md) for full Go template feature support.

---

## JavaScript Requirements

These features require the JavaScript client (fetch or WebSocket transport). Standard HTML form submission (no-JS) does not support them.

| Feature | No-JS Alternative | Why |
|---------|-------------------|-----|
| Standalone buttons outside `<form>` | Wrap in `<form method="POST">` | Button click events require JS to intercept |
| `Change()` live input binding | N/A — form is submit-only | Requires client to detect input changes and send to server |
| `form.name` routing | Use `button name` instead | JS client reads `form.name` as an action router — standard HTML POST ignores it as a routing signal |
| `lvt-*` attributes | Use standard HTML equivalents (see [Progressive Complexity Guide](../guides/progressive-complexity.md)) | Custom attributes require JS to interpret |
| Server push / broadcast | N/A — poll or page reload | Requires WebSocket connection |
| SPA navigation (link interception) | Standard full-page navigation | Requires JS to intercept clicks and use `fetch()` |

See the [Transport Compatibility table](progressive-complexity-reference.md#transport-compatibility) for a complete feature-by-transport breakdown.

---

## State & Serialization

| Limitation | Detail | Workaround |
|-----------|--------|-----------|
| JSON serialization overhead | State is cloned via JSON marshal/unmarshal per session | Keep state structs small; avoid large nested structures |
| State must be JSON-serializable | Functions, channels, and unexported fields cannot be in state | Put non-serializable dependencies in the controller |
| Dependency detection is heuristic | `AsState[T]()` only catches 9 known dependency patterns (stdlib + `*redis.Client`) | Add `AssertPureState[T](t)` to test files for stricter validation |

See [State Safety Reference](state-safety.md) for the full enforcement architecture.

---

## Session Behavior

| Limitation | Detail | Workaround |
|-----------|--------|-----------|
| Tabs don't auto-sync by default | Each connection owns its state independently | Implement `Sync()` on the controller for automatic cross-tab sync, or use `ctx.BroadcastAction()` for custom sync |
| Concurrent HTTP requests serialized | Per-group mutex in HTTP mode processes one action at a time | By design — prevents data races on shared state |

See [Session Reference](session.md) for session stores and connection management.

---

## HTTP vs WebSocket Feature Split

Some features are only available in one transport mode. This is by design — each transport has different capabilities.

| HTTP-Only | WebSocket-Only |
|-----------|---------------|
| `ctx.SetCookie()` / `ctx.GetCookie()` / `ctx.DeleteCookie()` | `ctx.BroadcastAction()` |
| `ctx.Redirect()` | Server push via `Session.TriggerAction()` |
| Query params merged with form data | Real-time bidirectional communication |

Use `ctx.IsHTTP()` to check which transport is active in an action method.

---

## Validation

| Limitation | Detail | Workaround |
|-----------|--------|-----------|
| Form schema not auto-wired from statics | `ExtractFormSchema()` exists but must be called manually via `ctx.WithFormSchema()` | Use `ctx.BindAndValidate()` with struct tags for production validation |
| `formnovalidate` not respected server-side | `ctx.ValidateForm()` validates all fields regardless of the submitting button's `formnovalidate` attribute | Skip validation manually in the action method for draft/save-without-validation flows |

---

## Performance

| Limitation | Detail | Status |
|-----------|--------|--------|
| TreeNode allocations (22.7% of memory) | Inherent cost of tree-based diffing architecture | Investigated — `sync.Pool` yielded only 2.7% allocation reduction, not worth the complexity |
| State cloning JSON round-trip | Per-session cost on first request | Keep state small; subsequent renders are fast (~3 KB, 61 allocs) |
| HTML fallback parsing (3.05% of allocations) | Triggered by unsupported template constructs (see Template Features above) | Improve template construct coverage to reduce fallback frequency |

See [Known Bottlenecks](../performance/known-bottlenecks.md) for detailed profiling data and optimization history.

---

## See Also

- [Roadmap](../../ROADMAP.md) — Planned improvements and feature timeline
- [State Safety Reference](state-safety.md) — Enforcement layers for state purity and session isolation
- [Template Support Matrix](template-support-matrix.md) — Supported Go template features
- [HTML Fallback Coverage](../roadmap/html-fallback-coverage.md) — Fallback trigger test coverage
- [Known Bottlenecks](../performance/known-bottlenecks.md) — Performance profiling and optimization
