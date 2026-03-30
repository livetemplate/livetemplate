# Tier 1 File Uploads

**Status:** Proposed
**Date:** 2026-03-30
**Issue:** [#271](https://github.com/livetemplate/livetemplate/issues/271)

## Summary

Move file uploads from Tier 2 (`lvt-upload`) to Tier 1 (standard HTML). A plain `<input type="file">` inside a `<form>` should work like any other form field — no `lvt-*` attributes needed. Progress tracking uses reactive state variables pushed over the existing WebSocket connection (or HTTP polling when WS is unavailable).

This aligns with the progressive complexity model: simple uploads work with zero attributes, and `lvt-upload` is reserved for behaviors HTML genuinely cannot express.

## Current State

File uploads currently require both a Tier 2 attribute and explicit server configuration:

```html
<input type="file" lvt-upload="avatar" accept="image/*" />
```

```go
tmpl := livetemplate.New("profile",
    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 5 << 20,
        MaxEntries:  1,
        AutoUpload:  true,
    }),
)
```

The `lvt-upload` attribute tells the JS client to intercept the file input and route it through a custom WebSocket chunked upload protocol (`upload_start` → `upload_chunk` → `upload_complete`). This is unnecessarily complex for basic file uploads.

## Proposed Design

### Tier 1 Template

```html
<!-- No lvt-* attributes anywhere -->
<form method="POST" enctype="multipart/form-data">
    <input type="file" name="avatar" accept="image/*">
    <button type="submit">Upload</button>
</form>

{{if .Uploading}}
    <progress value="{{.UploadProgress}}" max="100"></progress>
{{end}}
```

```go
func (c *Controller) Submit(state State, ctx *livetemplate.Context) (State, error) {
    for _, entry := range ctx.GetCompletedUploads("avatar") {
        state.AvatarPath = moveToStorage(entry.TempPath)
    }
    state.Uploading = false
    return state, nil
}
```

### Transport

File uploads use HTTP fetch (`multipart/form-data`), not WebSocket. This is the correct transport for binary data — even when a WebSocket connection is active.

### Progress Tracking via Reactive State

Instead of a custom upload protocol, progress tracking uses the existing reactive update infrastructure:

1. Client submits form via HTTP fetch (`multipart/form-data`)
2. Server reads the multipart body incrementally (wrap `multipart.Reader` with byte-counting `io.Reader`)
3. As bytes arrive, server updates session state (e.g., `state.UploadProgress = 45`) and triggers re-render
4. Diff pushed to client over the existing transport
5. Client patches DOM — progress bar updates
6. Broadcast throttled (every 2-5% or every 100ms) to avoid flooding

Progress is just a normal Go template variable. No special protocol.

### Progress Across Transports

The HTTP upload and WebSocket are separate connections, but share the same session. How progress reaches the client depends on the available transport:

| Transport | How progress arrives | UX |
|---|---|---|
| **WebSocket** | Server pushes state diffs reactively over existing WS connection | Real-time progress bar |
| **HTTP fetch + polling** | Client auto-polls for current state at interval (e.g. every 500ms) | Near-real-time progress bar |
| **No JS (plain POST)** | No mid-request updates possible | Browser-native upload indicator, page refreshes on completion |

When WebSocket is unavailable, the client automatically starts polling when it detects a pending upload. The poll endpoint is just a normal "give me the current state" request — no upload-aware logic needed. Polling stops when the upload completes.

### Auto-Upload on File Selection

LiveTemplate already infers intent from HTML structure (form with no action → `Submit()`, standalone button → triggers action). The same pattern applies to file inputs:

| HTML Structure | Behavior |
|---|---|
| File input in form **with** submit button | Standard: submit on button click |
| File input in form **without** submit button | Auto-submit on file selection |
| Standalone file input outside a form | Auto-submit on selection (mirrors standalone buttons) |

```html
<!-- Manual submit: has a submit button, waits for click -->
<form method="POST" enctype="multipart/form-data">
    <input type="file" name="avatar">
    <button type="submit">Upload</button>
</form>

<!-- Auto-upload: no submit button → auto-submit on file selection -->
<form method="POST" enctype="multipart/form-data">
    <input type="file" name="avatar" accept="image/*">
</form>

<!-- Auto-upload: standalone input, like standalone buttons -->
<input type="file" name="avatar" accept="image/*">
```

### Drag-and-Drop

Dragging a file onto `<input type="file">` already works natively in browsers — it populates the input, then the user submits the form (or auto-upload triggers). This is Tier 1 with no changes needed.

### Validation Inference from HTML Attributes

Following the same pattern as `ExtractFormSchema()` for form validation:

| HTML Attribute | Inferred Constraint |
|---|---|
| `accept="image/*"` | Allowed MIME types/extensions |
| `multiple` present | `MaxEntries` unlimited |
| `multiple` absent | `MaxEntries` = 1 |
| No explicit config | Sensible defaults (e.g., 10MB max) |

`WithUpload()` remains available as an optional override for explicit size limits and other constraints.

### `enctype` Inference

The client auto-infers `enctype="multipart/form-data"` when file inputs are present. Browsers already do this with `FormData`, so this is consistent with native behavior.

## What Stays Tier 2

Only behaviors that genuinely cannot be expressed with standard HTML:

| Capability | Why Tier 2 |
|---|---|
| **Custom drop zone** (`<div>` as drop target) | Requires JS event wiring to a non-input element |
| **Direct-to-S3 presigned uploads** | Requires server round-trip for presigned URL before upload starts; client uploads directly to cloud storage |

## Compatibility Matrix

| Feature | No JS | JS + HTTP | JS + WebSocket |
|---------|-------|-----------|----------------|
| File upload via form submit | Native POST | fetch multipart | fetch multipart |
| `accept` / `multiple` validation | Browser-native | Browser + server | Browser + server |
| Progress tracking | Browser-native indicator | HTTP polling | WS reactive push |
| Auto-upload (no submit button) | N/A | fetch on change | fetch on change |
| Drag-and-drop onto file input | Native browser | Native browser | Native browser |
| Custom drop zone (Tier 2) | N/A | Works | Works |
| Direct-to-S3 (Tier 2) | N/A | Works | Works |

## Implementation Phases

### Phase 1: Basic Tier 1 File Upload

**Client (separate `client` repo, ~100 lines):**
- Detect `<input type="file">` in intercepted forms
- Send as `multipart/form-data` HTTP fetch (not WebSocket), even when WS is active
- Auto-infer `enctype` when file inputs are present

**Server (~50 lines):**
- Auto-create upload registry entries for multipart files even without explicit `WithUpload()` config
- Default config fallback: 10MB max, no type restriction, `MaxEntries` inferred from `multiple`
- Files in `internal/upload/multipart.go`, `mount.go`, `upload.go`

### Phase 2: Progress Tracking via Reactive State

**Server (~150 lines):**
- Wrap `multipart.Reader` with byte-counting `io.Reader` in `internal/upload/multipart.go`
- Update session state with progress during streaming
- Trigger re-render/broadcast at throttled intervals
- HTTP upload handler needs access to session's broadcast mechanism (via `mount.go`)

**Client (~80 lines):**
- Auto-wire HTTP polling fallback when WS is unavailable and upload is in-flight
- Poll interval: 500ms, stops on upload completion

### Phase 3: Auto-Upload Inference

**Client (~60 lines):**
- Detect file inputs in forms without submit buttons → auto-submit on `change` event
- Detect standalone file inputs outside forms → auto-submit on `change` event
- Same HTTP fetch transport as Phase 1

### Phase 4: Validation Inference

**Server (~80 lines):**
- Scan template statics for `<input type="file">` elements (extend `ExtractFormSchema()` pattern)
- Infer `accept` and `multiple` constraints from HTML attributes
- Apply inferred constraints during multipart parsing

### Phase 5: Documentation

- Update `docs/guides/progressive-complexity.md` with file uploads as Tier 1 example
- Update `docs/references/uploads.md` to document both Tier 1 (standard HTML) and Tier 2 (`lvt-upload`) paths
- Update `docs/references/client-attributes.md` to reflect reduced scope of `lvt-upload`

## Key Files

| File | Role |
|---|---|
| `upload.go` | Public API, `WithUpload()` option, default config fallback |
| `mount.go` | Handler integration, upload registry auto-creation, progress broadcast |
| `internal/upload/multipart.go` | Multipart parsing, byte-counting reader, progress tracking |
| `internal/upload/registry.go` | Per-connection upload state |
| `internal/upload/validate.go` | Validation against config (inferred or explicit) |
| `internal/upload/protocol.go` | WebSocket protocol messages (unchanged, used by Tier 2) |
| `internal/send/message.go` | Multipart form detection (already exists) |
| `docs/guides/progressive-complexity.md` | Guide update |
| `docs/references/uploads.md` | Reference update |
| `docs/references/client-attributes.md` | Attribute reference update |

## Design Decisions

1. **HTTP fetch for file uploads, not WebSocket.** Binary data over WS requires base64 encoding (33% overhead) and a custom chunked protocol. HTTP multipart is the native, efficient transport for files. Even Phoenix LiveView uses HTTP for uploads.
2. **Progress via reactive state, not custom protocol.** Reuses existing infrastructure. No new message types, no new client-side parsing. Progress is just a template variable.
3. **Auto-polling when WS unavailable.** Ensures progress tracking works across all JS-enabled transports without user configuration.
4. **`WithUpload()` optional, not removed.** Keeps explicit configuration available for users who need specific constraints. Sensible defaults make it unnecessary for simple cases.
5. **Auto-upload inferred from HTML structure.** Mirrors existing pattern (standalone buttons, forms without explicit action). No new attributes needed.

## What Moves OUT of `lvt-upload`

| Old Pattern | Standard HTML Replacement |
|---|---|
| `<input type="file" lvt-upload="avatar">` | `<input type="file" name="avatar">` |
| `lvt-upload` + `AutoUpload: true` | File input in form without submit button |
| `lvt-upload` + progress template helpers | Standard template variable `{{.UploadProgress}}` |
| `lvt-upload` + drag-and-drop onto file input | Native browser behavior (already works) |

`lvt-upload` continues to work for backward compatibility and for Tier 2 features (custom drop zones, presigned S3 uploads).

## References

- **HTML Living Standard**: [`<input type="file">`](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/input/file)
- **Progressive Complexity Proposal**: `docs/proposals/progressive-complexity-proposal.md`
- **Phoenix LiveView Uploads**: Uses HTTP for file transport, not LiveView's WebSocket — same insight
