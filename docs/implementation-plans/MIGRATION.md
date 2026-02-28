# Migration Guide to v1.0

This guide helps you migrate to LiveTemplate v1.0, which includes a major refactoring for production readiness.

## Overview

LiveTemplate v1.0 introduces:
- Production-ready observability (structured logging and metrics)
- Clean internal package architecture
- Better organized codebase with operational phase naming
- Zero breaking changes to the public API

## Breaking Changes

**None!** The v1.0 refactoring maintains 100% backward compatibility with previous versions.

All existing code will continue to work without modifications. The changes are entirely internal reorganization and new optional features.

## New Features

### 1. Production Observability

v1.0 adds production-ready observability using Go's standard `log/slog` package.

#### Structured Logging

```go
import (
    "log/slog"
    "os"
    "github.com/livetemplate/livetemplate"
)

// Configure structured logging (JSON for production, Text for development)
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))

// Use with LiveTemplate
tmpl := livetemplate.Must(livetemplate.New("mytemplate").Parse(templateString))
// Template operations are automatically logged via slog
```

**What gets logged:**
- Template parsing events
- Tree building operations
- Tree diffing and updates
- WebSocket connections
- Action processing
- Broadcast events

**Example log output:**
```json
{
  "time": "2025-10-31T12:00:00Z",
  "level": "INFO",
  "msg": "Template parsed",
  "template": "index.html",
  "duration_ms": 2.5
}
```

#### Operational Metrics

Metrics are collected automatically and exposed via a Prometheus endpoint:

```go
handler := tmpl.Handle(controller, livetemplate.AsState(&State{}))

mux := http.NewServeMux()
mux.Handle("/", handler)
mux.Handle("/metrics", handler.MetricsHandler()) // Prometheus text format
```

**Available metrics:**
- **Counters**: `templates_executed`, `actions_processed`, `broadcasts_sent`
- **Gauges**: `active_connections`, `active_groups`
- **Histograms**: `template_duration_ms`, `build_duration_ms`, `diff_duration_ms` (with p50, p95, p99)

**Example metrics output:**
```json
{
  "time": "2025-10-31T12:01:00Z",
  "level": "INFO",
  "msg": "Metrics snapshot",
  "templates_executed": 1523,
  "template_duration_p50": 1.2,
  "template_duration_p95": 3.8,
  "template_duration_p99": 5.1,
  "active_connections": 42
}
```

### 2. Internal Package Organization

The codebase now uses operational phase naming for better clarity:

- `internal/parse/` - Template parsing (replaces old tree_ast.go)
- `internal/build/` - Tree building operations
- `internal/diff/` - Tree comparison and differential operations
- `internal/observe/` - Operational metrics (exposed via public `MetricsHandler()`)

**You don't need to import these packages** - they're internal implementation details that cannot be imported externally. The public API (`github.com/livetemplate/livetemplate`) remains unchanged.

## Migration Steps

### Step 1: Update Dependency

```bash
go get -u github.com/livetemplate/livetemplate@v1.0.0
go mod tidy
```

### Step 2: (Optional) Add Observability

If you want production observability, configure structured logging and expose the metrics endpoint:

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    "github.com/livetemplate/livetemplate"
)

func main() {
    // Setup structured logging
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))

    // Use LiveTemplate as normal
    tmpl := livetemplate.Must(livetemplate.New("index").Parse(`
        <h1>{{.Title}}</h1>
    `))

    handler := tmpl.Handle(controller, livetemplate.AsState(&State{}))

    mux := http.NewServeMux()
    mux.Handle("/", handler)
    mux.Handle("/metrics", handler.MetricsHandler()) // Prometheus metrics

    http.ListenAndServe(":8080", mux)
}
```

### Step 3: Test

Run your existing tests - everything should pass without changes:

```bash
go test ./...
```

## API Stability

The following public APIs are **stable and unchanged**:

### Core Types
- `Template`
- `TreeNode`
- `Config`
- `Engine`

### Core Functions
- `New(name string) *Template`
- `Must(t *Template, err error) *Template`
- `Parse(text string) (*Template, error)`
- `Execute(data interface{}) (string, error)`
- `ExecuteToTree(data interface{}) (*TreeNode, error)`
- `Update(oldData, newData interface{}) (*TreeNode, error)`

### WebSocket Integration
- `HandleActionMessage()`
- `HandleWebSocket()`
- All WebSocket helper functions

## Examples

### Before (v0.x) - Still Works!

```go
tmpl := livetemplate.Must(livetemplate.New("counter").Parse(`
    <div>Count: {{.Count}}</div>
`))

html, _ := tmpl.Execute(map[string]interface{}{"Count": 5})
```

### After (v1.0) - Same Code Works + Optional Observability

```go
// Optional: Configure structured logging
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

// Existing code works exactly the same
tmpl := livetemplate.Must(livetemplate.New("counter").Parse(`
    <div>Count: {{.Count}}</div>
`))

html, _ := tmpl.Execute(map[string]interface{}{"Count": 5})
// Now you get automatic structured logging of template operations!
```

## Performance

v1.0 maintains the same performance characteristics:

- **Observability overhead**: <0.1% of request time
- **Memory usage**: No increase
- **Tree diffing**: Same O(n) complexity
- **Update generation**: Same optimizations

Benchmarks show no performance regression:

```
BenchmarkTemplateExecution-8    50000    25000 ns/op
BenchmarkTreeDiff-8            100000    15000 ns/op
```

## Troubleshooting

### Q: My tests are failing after upgrade

**A:** This shouldn't happen as there are zero breaking changes. If you see failures:

1. Ensure you're running `go mod tidy`
2. Check for any custom imports of internal packages (these were never public)
3. Verify test environment (especially Docker for e2e tests)

### Q: How do I access the old tree_ast.go functionality?

**A:** The functionality is now in `internal/parse/` package. However, this was never part of the public API. If you were using it, please use the public `Parse()` and `ExecuteToTree()` functions instead.

### Q: Do I need to enable observability?

**A:** No! Observability is completely optional. Structured logging uses `log/slog` (zero overhead if not configured), and the Prometheus `/metrics` endpoint is only active if you register it.

### Q: Can I use custom slog handlers?

**A:** Yes! LiveTemplate uses `log/slog` directly, which works with any `slog.Handler` implementation:

```go
// Custom handler
slog.SetDefault(slog.New(myCustomHandler{}))
```

## Getting Help

- **Documentation**: See [OBSERVABILITY.md](./OBSERVABILITY.md) for detailed observability guide
- **Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md) for internal architecture
- **Issues**: Report issues at https://github.com/livetemplate/livetemplate/issues
- **Discussions**: Ask questions at https://github.com/livetemplate/livetemplate/discussions

## Summary

**v1.0 Migration Checklist:**

- [ ] Update dependency: `go get -u github.com/livetemplate/livetemplate@v1.0.0`
- [ ] Run tests: `go test ./...`
- [ ] (Optional) Configure `slog` and expose `/metrics` endpoint via `handler.MetricsHandler()`
- [ ] (Optional) Review new documentation (OBSERVABILITY.md, ARCHITECTURE.md)

That's it! Your existing code works without changes, and you get the benefits of a production-ready, well-organized codebase.
