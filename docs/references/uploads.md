# Upload Reference

## Overview

LiveTemplate provides a file upload system with support for:
- **WebSocket chunked uploads** - Large file uploads with real-time progress
- **External uploads** - Direct uploads to S3/cloud storage via presigned URLs
- **Progress tracking** - Real-time upload progress with validation
- **Template helpers** - Display upload state in your templates

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
<form lvt-submit="saveProfile">
    <input type="file" lvt-upload="avatar" accept="image/*" />

    {{range .lvt.Uploads "avatar"}}
        <div class="upload-entry">
            <span>{{.ClientName}}</span>
            <progress value="{{.Progress}}" max="100"></progress>
            {{if .Error}}<span class="error">{{.Error}}</span>{{end}}
        </div>
    {{end}}

    <button type="submit">Save Profile</button>
</form>
```

### 3. Process Uploads in Action Handler

Access completed uploads via the Context:

```go
func (c *ProfileController) SaveProfile(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
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

```go
s3Config := livetemplate.S3Config{
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

presigner, err := livetemplate.NewS3Presigner(s3Config)
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
