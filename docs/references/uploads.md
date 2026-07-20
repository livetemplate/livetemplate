# Upload Reference

## Overview

LiveTemplate provides a file upload system with four **modes**, chosen purely by
server config on an otherwise identical `<input lvt-upload>`:

| Mode | Bytes path | Server sees bytes? | Local disk? | Config |
|------|------------|--------------------|-------------|--------|
| **Volume** *(default)* | browser → server → retained directory | yes | yes | `Mode: UploadModeVolume, Dir: "..."` |
| **Direct** | browser → cloud via presigned URL | no | no | `Mode: UploadModeDirect, External: presigner` |
| **Proxied** | browser → server → remote storage, streamed | yes | **no** | `Mode: UploadModeProxied` + `OnUpload` |
| **Preview** | stays on the device | metadata only | no | `Mode: UploadModePreview` |

```go
livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
    Mode:        livetemplate.UploadModeProxied, // stream to remote storage, zero disk
    Accept:      []string{"image/*"},
    MaxFileSize: 5 << 20,
})
```

The mode is delivered to the client per-entry, so the same markup and the same
`ctx.GetCompletedUploads(name)` consumption work across every mode. See
[Upload modes](#upload-modes) below and the `upload-modes` example.

## Upload modes

### Volume — staged to the server's disk

`Mode: UploadModeVolume` (the default) stages bytes to the server's filesystem.
With `Dir` set the file is **retained** there and the app owns its lifecycle; with
no `Dir` it stages to a temp dir that is cleaned up when the connection closes
(the legacy stage-then-move pattern). Read the path from `entry.TempPath`.

### Direct — browser uploads straight to cloud storage

`Mode: UploadModeDirect` with an `External` presigner has the browser PUT bytes
straight to S3/GCS/etc. via a presigned URL — they never touch the server. Read
the reference from `entry.ExternalRef`. (Setting `External` without an explicit
`Mode` is treated as Direct for backward compatibility.)

### Proxied — stream through the server with zero local disk

`Mode: UploadModeProxied` streams the in-flight bytes straight to a handler with
no local-disk staging — ideal for forwarding to remote object storage. The
controller implements `UploadStreamer`:

```go
func (c *Controller) OnUpload(part *livetemplate.UploadPart, ctx *livetemplate.Context) error {
    // "id" must be marked lvt-upload-with and ordered before the file input
    // in the form (see below).
    ref, err := myBackend.Put(ctx, ctx.GetString("id"), part.Filename, part)
    if err != nil {
        return err
    }
    part.SetResult(ref) // surfaced via GetCompletedUploads(...).ExternalRef
    return nil
}
```

Inside `OnUpload`, `ctx` carries the request identity (`ctx.UserID()`,
`ctx.GroupID()`) **and** the form fields the upload was told to carry, read via
`ctx.GetString(...)`.

#### Sending form fields with an upload

A Proxied upload auto-fires the moment a file is selected — there is no submit
for the user to review — so nothing from the enclosing form travels unless you
mark it with `lvt-upload-with`:

```html
<form>
  <input type="hidden" name="id" value="{{.Record.ID}}" lvt-upload-with />
  <input type="hidden" name="csrf" value="{{.CSRFToken}}" />
  <input type="file" lvt-upload="scan" />
</form>
```

Here `id` reaches `OnUpload` and `csrf` does not. Mark only what the handler
needs: unmarked fields — session tokens, co-located credentials, anything else
that happens to share the form — never reach the upload endpoint. Forgetting to
mark a field you *do* need surfaces as a missing value in `OnUpload`, which is
why the default is off.

Three rules apply on top of the marking:

- **Order matters.** Parts stream in body order, so a marked field is readable
  mid-stream only if its input precedes the file input. Fields after the file
  part reach only the follow-on action.
- **Marking is by name, and form-scoped.** A marked field travels with every
  upload fired from its form, and marking any one member of a radio or checkbox
  group opts in the whole group. Which values actually send is still the
  browser's usual successful-control behaviour — an unchecked box sends nothing.
- **The file wins a name collision.** If a marked field shares the upload
  field's name, the file part replaces it.

When one selection carries several files, each is POSTed as its own request and
the marked fields ride along with every one, so each reaches `OnUpload`
self-describing.

#### Marked fields are multipart-only

Marked fields travel on the **multipart** upload path, and only there. Which mode
you use decides whether that path is taken:

| Mode | Transport | Marked fields arrive? |
| --- | --- | --- |
| **Proxied** | always multipart | **Yes**, always — `OnUpload` included |
| **Volume** | chunked over the WebSocket; multipart when the socket is down | Only on the multipart fallback |
| **Direct** | presigned PUT to storage, then a metadata-only completion | **No**, on any path |

The chunked WebSocket transport sends bytes and entry ids, and Direct completes
with a metadata-only message — neither carries form fields, so the server builds
the `upload_<name>_complete` action's context from an empty map.

The client logs a console warning when an upload leaves by one of those
transports from a form that marks fields, so this surfaces as a diagnosable
message rather than a silently empty value.

Note the asymmetry between the two: a Volume upload *does* deliver marked fields
when it falls back to multipart, so a handler can see them intermittently
depending on socket state — don't depend on that. Direct never delivers them at
all. For either, read the context from the state your controller already holds
rather than from the upload's form. Tracked as
[#508](https://github.com/livetemplate/livetemplate/issues/508).

Requires `@livetemplate/client` v0.20.0 or newer.

The reader enforces `MaxFileSize` mid-stream, returning `ErrUploadTooLarge` (a
distinct sentinel, not `io.EOF`) so a truncated stream aborts instead of
committing a partial object. Because nothing stages to disk, a pure-Proxied app
needs no writable working directory and never creates `.uploads`.

> **Note:** Adding a Proxied field routes **every** multipart POST to that
> handler through the streaming path, including requests carrying only Volume
> fields. Those Volume parts are staged to disk as usual (equivalent to the
> default path), so mixing modes on one handler is fine — just be aware the
> coupling exists.

### Preview — file stays on the device

`Mode: UploadModePreview` keeps the file in the browser; only its metadata
(name/type/size) reaches the server. Render the on-device preview with the
template helper:

```html
<input type="file" lvt-upload="draft" accept="image/*" />
{{.lvt.UploadPreview "draft"}}
```

The client fills the placeholder from `URL.createObjectURL` and never uploads the
bytes. The server records a metadata-only entry (`entry.Preview == true`, no
`TempPath`/`ExternalRef`) readable via `GetCompletedUploads`.

## Quick Start

### 1. Configure Uploads on Template

Use `WithUpload()` to declare upload fields when creating a template:

```go
tmpl := livetemplate.New("profile",
    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 5 * 1024 * 1024, // 5MB
        MaxEntries:  1,
        AutoUpload:  true,
    }),
)

handler := tmpl.Handle(&ProfileController{}, livetemplate.AsState(&ProfileState{}))
```

### 2. Add Upload Input to Template

```html
<form method="POST" enctype="multipart/form-data">
    <input type="file" lvt-upload="avatar" accept="image/*" />

    {{range .lvt.Uploads "avatar"}}
        <div class="upload-entry">
            <span>{{.ClientName}}</span>
            <progress value="{{.Progress}}" max="100"></progress>
            {{if .Error}}<span class="error">{{.Error}}</span>{{end}}
        </div>
    {{end}}

    <button name="saveProfile" type="submit">Save Profile</button>
</form>
```

### 3. Process Uploads in Action Handler

Access completed uploads and text fields via the Context. When a form with
`enctype="multipart/form-data"` is submitted, both file uploads and text fields
are available in the same action handler.

> **Note:** The field name `data` is reserved for the LiveTemplate client library's
> JSON encoding. Avoid naming a plain text field `data` in multipart forms.

```go
func (c *ProfileController) SaveProfile(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
    // Text fields from the same form are available via ctx.GetString()
    state.Name = ctx.GetString("name")
    state.Email = ctx.GetString("email")

    // File uploads are available via ctx.GetCompletedUploads()
    for _, entry := range ctx.GetCompletedUploads("avatar") {
        // entry.TempPath: server-side temporary file path
        // entry.ClientName: original filename
        // entry.ClientType: MIME type
        // entry.ClientSize: file size in bytes
        state.AvatarPath = moveToStorage(entry.TempPath)
    }
    return state, nil
}
```

## Server API

### WithUpload Option

Configure upload fields at template creation:

```go
func WithUpload(name string, config UploadConfig) Option
```

Multiple upload fields can be configured on the same template:

```go
tmpl := livetemplate.New("editor",
    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 5 << 20,
        MaxEntries:  1,
    }),
    livetemplate.WithUpload("documents", livetemplate.UploadConfig{
        Accept:      []string{".pdf", ".doc", ".docx"},
        MaxFileSize: 50 << 20,
        MaxEntries:  10,
    }),
)
```

### UploadConfig

Configures upload behavior for a specific field:

```go
type UploadConfig struct {
    Accept      []string  // Allowed MIME types or extensions (e.g., []string{"image/*", ".pdf"})
    MaxEntries  int       // Maximum number of concurrent files (0 = unlimited)
    MaxFileSize int64     // Maximum file size in bytes (0 = unlimited)
    AutoUpload  bool      // Start upload automatically on file selection
    ChunkSize   int       // Chunk size for WebSocket uploads in bytes (default: 256KB)
    External    Presigner // Optional presigner for direct-to-storage uploads
}
```

### UploadEntry

Represents a single uploaded file:

```go
type UploadEntry struct {
    ID          string    // Server-generated unique ID
    ClientName  string    // Original filename from client
    ClientType  string    // MIME type reported by the client
    ClientSize  int64     // File size in bytes
    Progress    int       // Upload progress 0-100
    Valid       bool      // Whether the upload passed validation
    Done        bool      // Whether the upload has completed
    Error       string    // Error message if validation or upload failed
    TempPath    string    // Server-side temporary file path (server uploads only)
    BytesRecv   int64     // Bytes received so far (for progress tracking)
    ExternalRef string    // Presigned URL from Presigner (external uploads only)
    CreatedAt   time.Time
    CompletedAt time.Time
}
```

### Context Upload Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `ctx.HasUploads(name)` | `bool` | Check if any entries exist for a field (including in-progress) |
| `ctx.GetCompletedUploads(name)` | `[]*UploadEntry` | Get all completed upload entries |

## Template Helpers

### `.lvt.Uploads "name"`

Iterate over upload entries for a specific field:

```html
{{range .lvt.Uploads "avatar"}}
    <div class="upload">
        <span>{{.ClientName}} ({{.ClientSize}} bytes)</span>
        <progress value="{{.Progress}}" max="100">{{.Progress}}%</progress>

        {{if .Error}}
            <div class="error">{{.Error}}</div>
        {{end}}

        {{if .Done}}
            <span class="badge">Complete</span>
        {{end}}
    </div>
{{end}}
```

### `.lvt.HasUploadError "name"`

Check if an upload field has errors:

```html
{{if .lvt.HasUploadError "avatar"}}
    <div class="alert alert-error">
        {{.lvt.UploadError "avatar"}}
    </div>
{{end}}
```

### `.lvt.UploadError "name"`

Get the error message for an upload field:

```html
<span class="error">{{.lvt.UploadError "documents"}}</span>
```

## S3 / External Uploads

### Setup S3 Presigner

Import `github.com/livetemplate/lvt/pkg/s3presigner` to use the S3 presigner:

```go
s3Config := s3presigner.S3Config{
    Bucket:    "my-uploads",
    Region:    "us-east-1",
    KeyPrefix: "uploads",        // Optional: organizes S3 keys
    Expiry:    15 * time.Minute, // Presigned URL expiry

    // Option 1: IAM role (recommended for production)
    // Credentials auto-detected from environment

    // Option 2: Static credentials
    AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
    SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),

    // Option 3: Custom endpoint (MinIO, LocalStack)
    Endpoint: "http://localhost:9000",
}

presigner, err := s3presigner.NewS3Presigner(s3Config)
if err != nil {
    log.Fatal(err)
}

tmpl := livetemplate.New("photos",
    livetemplate.WithUpload("photos", livetemplate.UploadConfig{
        Accept:      []string{"image/*"},
        MaxFileSize: 10 << 20,
        External:    presigner, // Client uploads directly to S3
    }),
)
```

### S3 Upload Flow

1. **Client selects file** - Sends file metadata to server
2. **Server generates presigned URL** - Calls `Presigner.Presign()`, stores the URL in `UploadEntry.ExternalRef`, and returns `UploadMeta` to the client
3. **Client uploads directly to S3** - No server bandwidth used
4. **Client sends `upload_complete`** - Server marks entries as done
5. **Action handler processes** - Access via `ctx.GetCompletedUploads()`

### Presigner Interface

For custom external upload providers:

```go
type Presigner interface {
    Presign(entry *UploadEntry) (UploadMeta, error)
}

type UploadMeta struct {
    Uploader string            // Provider name (e.g., "s3", "gcs", "azure")
    URL      string            // Presigned upload URL
    Fields   map[string]string // Form fields for multipart POST providers (nil for PUT-based providers like S3)
    Headers  map[string]string // HTTP headers for the upload request
}
```

### Custom External Uploader

```go
type AzurePresigner struct {
    config AzureConfig
}

func (p *AzurePresigner) Presign(entry *livetemplate.UploadEntry) (livetemplate.UploadMeta, error) {
    sasURL := p.generateSAS(entry.ClientName)
    return livetemplate.UploadMeta{
        Uploader: "azure",
        URL:      sasURL,
        Headers: map[string]string{
            "x-ms-blob-type": "BlockBlob",
        },
    }, nil
}
```

## Client Library

The LiveTemplate client automatically detects `lvt-upload` attributes on file inputs:

```html
<input type="file" lvt-upload="avatar" accept="image/*" />
```

No manual initialization required.

### Upload Events

```javascript
const wrapper = document.querySelector('[data-lvt-id]');

// Progress updates
wrapper.addEventListener('lvt:upload:progress', (e) => {
    const { entry } = e.detail;
    console.log(`${entry.file.name}: ${entry.progress}%`);
});

// Upload complete
wrapper.addEventListener('lvt:upload:complete', (e) => {
    const { uploadName, entries } = e.detail;
    console.log(`Completed: ${uploadName}`, entries);
});

// Upload error
wrapper.addEventListener('lvt:upload:error', (e) => {
    const { entry, error } = e.detail;
    console.error(`Error uploading ${entry.file.name}:`, error);
});
```

## Validation

Uploads are validated against `UploadConfig`:

```go
livetemplate.UploadConfig{
    Accept:      []string{"image/jpeg", "image/png", ".jpg", ".png"},
    MaxFileSize: 5 * 1024 * 1024,  // 5MB
    MaxEntries:  3,                 // Max 3 files
}
```

**Validation checks:**
- File type (MIME type or extension)
- File size
- File count

**Invalid files:**
- Marked as `Valid: false`
- `Error` field set with reason
- NOT included in `GetCompletedUploads()` results
- Temp files cleaned up automatically

## Security

### File Type Validation

```go
Accept: []string{
    "image/*",        // Any image MIME type
    "image/jpeg",     // Specific MIME type
    ".jpg", ".png",   // File extensions
}
```

MIME types can be spoofed. Always validate file content in your action handler.

### File Size Limits

```go
MaxFileSize: 10 * 1024 * 1024, // 10MB limit
```

### Path Traversal Prevention

S3 keys are sanitized using `filepath.Base()` to extract filename only.

### Temporary File Security

- Created in system temp dir with restricted permissions
- Random entry IDs prevent guessing
- Automatic cleanup on connection close

## Performance

### Chunked Upload Sizes

Default: 256KB chunks. Tunable via `ChunkSize`:

```go
ChunkSize: 512 * 1024, // 512KB chunks
```

- Smaller chunks = more overhead, better progress granularity
- Larger chunks = less overhead, coarser progress updates

### Memory Usage

- Chunked uploads: one chunk in memory at a time
- External uploads: no server memory used

## Troubleshooting

### Upload Not Starting

- Verify `lvt-upload` attribute matches the field name in `WithUpload()`
- Check browser console for JavaScript errors
- Ensure the template includes `lvt-upload="fieldName"` on the file input
- **SSR'd input + `AutoUpload` not firing on first select?** Upgrade to
  `@livetemplate/client` >= v0.13.1. Before that fix, a server-rendered
  `lvt-upload` input whose first WebSocket render added no new nodes was left
  without a change listener (see
  [#453](https://github.com/livetemplate/livetemplate/issues/453))

### Progress Not Updating

- Progress events require chunked uploads (default 256KB chunks)
- Very small files may complete in a single chunk with no intermediate progress
- Check WebSocket connection is active

### File Rejected by Validation

- `Accept` validation checks MIME type and extension — ensure both match
- `MaxFileSize` is in bytes — use `5 << 20` for 5MB, not `5000000`
- `MaxEntries` limits concurrent uploads per field

### Temporary Files Not Cleaned Up

- Temp files are cleaned automatically on WebSocket disconnect
- For HTTP-only mode, implement cleanup in your action handler
- Check system temp directory permissions if files persist

### External Upload (S3) Errors

- Verify presigner returns valid URLs with correct expiration
- Check CORS configuration allows PUT from the client origin
- S3 keys are sanitized via `filepath.Base()` — forward-slash paths are stripped (note: on Unix, backslash-separated paths like `..\..\..\etc\passwd` are treated as literal filenames)

### Content Validation

MIME types can be spoofed. For security-critical uploads, validate actual file content:

```go
import (
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"

    "github.com/livetemplate/livetemplate"
)

func (c *Controller) SaveAvatar(state State, ctx *livetemplate.Context) (State, error) {
    for _, entry := range ctx.GetCompletedUploads("avatar") {
        detected, err := detectContentType(entry.TempPath)
        if err != nil {
            return state, fmt.Errorf("reading upload: %w", err)
        }
        if !strings.HasPrefix(detected, "image/") {
            return state, fmt.Errorf("invalid file type: %s", detected)
        }
    }
    return state, nil
}

func detectContentType(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    buf := make([]byte, 512)
    if _, err := f.Read(buf); err != nil && err != io.EOF {
        return "", err
    }
    return http.DetectContentType(buf), nil
}
```

## See Also

- [Controller+State Pattern](controller-pattern.md) - Core architecture pattern
- [Client Attributes Reference](client-attributes.md) - `lvt-upload` attribute details
- [Client Library](https://github.com/livetemplate/client) - TypeScript client
