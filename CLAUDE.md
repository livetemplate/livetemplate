# LiveTemplate - Development Guidelines

## Project Overview

LiveTemplate is a high-performance Go library for building reactive web applications. It provides an API similar to `html/template` but with the additional capability of generating minimal, tree-based updates that can be efficiently transmitted to clients.

**Related repositories:**
- **CLI Tool (lvt)**: Code generator and development server — maintained at `github.com/livetemplate/lvt`
- **TypeScript Client**: Browser-side update handler — maintained at `github.com/livetemplate/client`

## Controller+State Pattern

Controllers hold dependencies (singleton, never cloned). State holds pure data (cloned per session). Action methods: `func (c *Controller) Action(state State, ctx *Context) (State, error)`. Mount runs on every HTTP request and WebSocket connect.

**Key caveats:**
- **Mount() guard pattern:** Mount runs on POST too, so guard side effects: `if ctx.Action() == "" { trackPageView() }` — only fires on GET, not form submissions.
- **BroadcastAction ordering:** `ctx.With*()` creates shallow copies. Call `ctx.BroadcastAction()` AFTER all `With*()` calls, or broadcasts queued before the copy won't propagate.
- **AssertPureState[T]():** Use in tests to catch dependency types accidentally in state structs.

See `docs/references/controller-pattern.md` for the full pattern guide with examples.

## Progressive Complexity

Standard HTML (forms, buttons, dialogs) handles Tier 1. `lvt-*` attributes (`lvt-on:`, `lvt-el:`, `lvt-fx:`, `lvt-mod:`, `lvt-form:`, `lvt-nav:`) are reserved for Tier 2 behaviors HTML cannot express.

See `docs/guides/progressive-complexity.md` for the full walkthrough.

## Architecture

5 operational phases: **Parse -> Build -> Diff -> Render -> Send**, each in `internal/{phase}/`. Supporting packages: `internal/keys/`, `internal/session/`, `internal/observe/`, `internal/upload/`, `internal/fuzz/`.

See `docs/design/ARCHITECTURE.md` for design decisions and `docs/design/CODE_STRUCTURE.md` for the complete file-by-file map.

## Key Data Structures

### TreeNode
```go
type TreeNode struct {
    Statics     []string                // Static HTML parts (key: "s")
    Dynamics    map[string]interface{}   // Dynamic content (keys: "0", "1", etc.)
    Fingerprint string                   // Structure hash (key: "f")
    Range       *RangeData              // Range operation data (key: "d")
    Metadata    *TreeMetadata           // Additional metadata (key: "m")
}
```
- Core structure for representing static/dynamic content
- Custom JSON marshaling maintains wire format compatibility (numeric keys for dynamics)
- Can be nested for complex templates

### Key Constructs
- `FieldConstruct`: Simple field replacement `{{.Field}}`
- `ConditionalConstruct`: If/else branches `{{if .Cond}}...{{else}}...{{end}}`
- `RangeConstruct`: Iteration `{{range .Items}}...{{end}}`
- `WithConstruct`: Context switching `{{with .Item}}...{{end}}`
- `TemplateInvokeConstruct`: Template invocation `{{template "name" .}}`

## Testing Strategy

### Test Files Structure
- `template_test.go`: Core template functionality tests (includes key injection tests)
- `tree_test.go`: Tree structure invariant validation
- `e2e_update_spec_test.go`: Tree update specification compliance tests
- Internal package tests: `internal/*/`

**Browser-based E2E Tests:**
Browser-based chromedp E2E tests are maintained in the lvt repository:
- Location: `github.com/livetemplate/lvt/e2e/livetemplate_core_test.go`
- These tests validate the library from a black-box perspective using real browser automation
- Tests include: complete rendering sequences, loading indicators, focus preservation, etc.

### Test Data
- `testdata/fixtures/`: Template fixtures for unit tests
- `testdata/golden/`: Golden files for snapshot testing
- `testdata/fuzz/`: Fuzz test corpus

### Running Tests
```bash
# Run all tests
go test -v ./...

# Run specific test categories
go test -run TestTemplate -v          # Template engine tests
go test -run TestTreeInvariant -v     # Tree invariant tests
go test -run TestKeyInjection -v      # Key injection tests
go test -run TestE2EUpdateSpec -v     # Update spec compliance tests

# Run with timeout
go test -v ./... -timeout=30s
```

## Development Conventions

### Release Process
- **Never create git tags manually** - Always use `release.sh` script for releases

### Code Style
1. **No unnecessary comments** - Code should be self-documenting
2. **Follow existing patterns** - Check neighboring code for conventions
3. **Use existing utilities** - Don't reinvent the wheel
4. **Maintain idiomatic Go** - Follow Go best practices

### Template Processing Flow
1. **Parse**: Template string -> Compiled template structure
2. **Execute**: First render generates initial tree with statics and dynamics
3. **Update**: Subsequent renders generate minimal update trees
4. **Diff**: Compare trees to produce update operations

### Key Generation Strategy
- Uses wrapper-based approach with sequential key generation
- Keys are stable within a single render
- Supports any data type without special handling
- Keys reset between renders for consistency

### Best Practices: `data-key` Attributes in Range Templates

When rendering lists with `{{range}}`, the diff engine needs stable keys to track items across renders. By default, LiveTemplate auto-generates hash-based keys from item content. Explicit `data-key` attributes give you control:

```html
{{range .Items}}
<div data-key="{{.ID}}">{{.Name}}</div>
{{end}}
```

**When to use explicit `data-key`:**
- Items have a natural unique identifier (database ID, UUID)
- List items are reordered frequently (drag-and-drop, sorting)
- Items may have identical content but different identity

**When auto-generated keys suffice:**
- Small static lists (<100 items) with no reordering
- Items are always appended/removed, never reordered
- Content is unique across all items

**Key stability matters for performance:** Stable keys enable the diff engine to emit minimal `["u", key, changes]` operations instead of removing and re-inserting items. Unstable keys (e.g., using array index) cause unnecessary DOM thrashing on the client.

## Important Implementation Details

### Wire Format Optimization
The library implements a critical optimization per the tree-update-specification.md:
- **First Render**: Tree includes complete static structure (`"s"` arrays) + all dynamics
- **Updates**: Tree includes ONLY changed dynamics, NO statics (client has them cached)

This is implemented by `PrepareTreeForClient(node, clientHasStatics)` which:
1. Takes a fully-built tree (WITH statics, needed for comparison consistency)
2. Returns a wire-format tree (WITHOUT statics if client has cached them)
3. Implements spec requirement: "Updates MUST include ONLY changed dynamics"

**Key Architectural Points**:
- Trees are ALWAYS generated WITH statics (ensures consistent comparison)
- Comparison logic (`compareTreesAndGetChanges`) determines what changed
- `PrepareTreeForClient` removes statics before wire transmission
- This is NOT a "reactive fix" - it's the correct implementation of specification
- Result: Updates are ~10% the size of full renders (statics are largest part)

### Fingerprint-Based Structure Comparison

The system uses FNV-1a structure fingerprints to decide whether statics need to be resent. This replaced an earlier per-path `ClientStructureRegistry` approach (removed in [PR #86](https://github.com/livetemplate/livetemplate/pull/86)) that was more complex and harder to debug.

**What gets fingerprinted** (`internal/build/fingerprint.go`):
- Statics arrays (the HTML template parts between dynamic slots)
- Dynamic key positions (e.g., "there's a dynamic at position 0, 1, 2")
- Nested TreeNode structure (recursively)
- Range statics (item template structure)
- NOT dynamic values — two trees with identical structure but different content produce the same fingerprint

**Decision flow** (`internal/diff/tree_compare.go`):
```go
func ClientNeedsStatics(oldTree, newTree *TreeNode) bool {
    // First render: no previous tree -> must send statics
    if oldTree == nil { return true }
    // Removal: new tree is gone -> no statics needed
    if newTree == nil { return false }
    // Compare structure fingerprints for non-nil trees
    oldFP := oldTree.GetStructureFingerprint()
    newFP := newTree.GetStructureFingerprint()
    // Same fingerprint -> client has statics cached -> send dynamics only
    // Different fingerprint -> structure changed -> send full tree with statics
    return oldFP != newFP
}
```

The lazy-cached fingerprint enables O(1) structure comparison, avoiding re-computation on subsequent comparisons.

**Key functions**:
- `CalculateStructureFingerprint(tree)` — Computes FNV-1a 128-bit truncated to 64 bits (16 hex chars) of static structure (`internal/build/fingerprint.go`)
- `TreeNode.GetStructureFingerprint()` — Lazy-computes and caches fingerprint on first access (`internal/build/types.go`)
- `ClientNeedsStatics(oldTree, newTree)` — Returns true if fingerprints differ (`internal/diff/tree_compare.go`)
- `PrepareTreeForClient(tree, clientHasStatics)` — Strips statics from wire format when cached (`internal/diff/prepare.go`)

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

After first render, client caches the `"s"` arrays. Subsequent updates omit them:
```json
{
  "0": "Updated content"         // Only changed dynamic, no statics
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
1. Add handling logic in the appropriate `internal/parse/` file (e.g., `field.go`, `conditional.go`, `range.go`, `with.go`)
2. For new construct types, create a new file in `internal/parse/` following existing patterns
3. Add parser logic in `internal/parse/parse.go`
4. Update type definitions in `internal/parse/types.go` if needed
5. Write tests in appropriate test files
6. Ensure backward compatibility if modifying existing constructs

### Debugging Tree Generation
1. Use `TreeNode.GetStructureFingerprint()` to track structural changes
2. Check `lastTree` vs current tree in Template
3. Validate tree structure with `validateTreeStructure()`
4. Use golden files for regression testing

### Updating Client Library
The TypeScript client is maintained in a separate repository at `github.com/livetemplate/client` (locally `../client`).
1. Make changes in the client repository
2. Ensure compatibility with tree format
3. Test with browser test suite
4. Update TypeScript types if needed

## Performance Considerations

1. **Tree Diffing**: O(n) complexity for most operations
2. **Memory**: Trees are kept in memory for diffing
3. **Fingerprinting**: FNV-1a hashing for change detection
4. **Key Generation**: Sequential integers for minimal overhead

## Security Notes

1. **HTML Escaping**: Uses `html/template` for automatic escaping
2. **No Direct HTML**: All content goes through template engine
3. **Wrapper IDs**: Random generation prevents conflicts

## Troubleshooting

### Test Failures
- Check golden files in `testdata/golden/` and `testdata/fixtures/`
- Verify tree structure matches expected format
- Ensure key generation is consistent
- Check for HTML escaping issues
- For browser E2E test failures, see lvt repository

### Tree Generation Issues
- Validate template syntax
- Check construct parsing order
- Verify hydration logic matches compilation
- Test with simpler templates first

### Async WebSocket Issues

**Connection Closes Unexpectedly:**
- Check metrics for `wsSlowClientCloses` counter
- Client may be too slow (buffer full)
- Increase buffer size with `LVT_WS_BUFFER_SIZE` or `WithWebSocketBufferSize()`
- Default buffer: 50 messages

**Goroutine Leaks:**
- Ensure all connections are unregistered via `registry.Unregister(conn)`
- `Unregister()` calls `Close()` which signals writePump to exit
- Check `pumpExited` channel closes (5-second timeout)
- Run tests with `go test -race` to detect race conditions

**High Memory Usage:**
- Each connection: ~980 bytes base + (buffer size x avg message size)
- Default 50-buffer: ~1KB per connection + message overhead
- Reduce buffer size for memory-constrained environments
- Monitor `wsSendBufferSize` gauge metric

**Messages Not Delivered:**
- Check `wsWriteErrors` metric for write failures
- Verify client WebSocket is connected and reading
- Check for `ErrConnectionClosed` or `ErrClientTooSlow` errors
- Review server logs for "WebSocket write failed" warnings

**Performance Tuning:**
- Benchmark results: 165M concurrent sends/sec, 54.7M queued sends/sec
- For high-throughput: Increase buffer size (100-1000)
- For low-latency: Decrease buffer size (10-20)
- Monitor `wsBufferFull` metric to detect backpressure
- Use Prometheus metrics at `/metrics` endpoint for observability

### PubSub / Cross-Instance Broadcasting Issues

**Cross-instance messages not received:**
- Verify `WithPubSubBroadcaster(broadcaster)` is configured
- Check Redis connectivity; all instances must use the same Redis
- Subscriptions are created automatically during WebSocket setup via `DynamicSubscriber` type assertion

**Subscriptions lost after Redis reconnection:**
- Should auto-recover: `reconnect()` replays all tracked channels from `subscribedChannels` map
- Check logs for `"Reconnected successfully"` with `dynamic_channels` count
- If `dynamic_channels=0`, no dynamic subscriptions were active

**See also:** [PubSub Reference](docs/references/pubsub.md)

## Future Improvements
- Consider adding more sophisticated diffing algorithms
- Optimize memory usage for large trees
- Add metrics and profiling hooks
- Enhance client-side caching strategies
