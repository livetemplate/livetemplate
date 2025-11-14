# Upload Feature Documentation

## Overview

LiveTemplate provides a Phoenix LiveView-inspired upload system with support for:
- **HTTP multipart uploads** - Simple, synchronous uploads via POST
- **WebSocket chunked uploads** - Large file uploads with real-time progress
- **External uploads** - Direct uploads to S3/cloud storage via presigned URLs
- **Progress tracking** - Real-time upload progress with validation
- **Template helpers** - Display upload state in your templates

## Quick Start

### 1. Make Your Store Upload-Aware

Implement the `UploadAware` interface on your store:

```go
import "github.com/livetemplate/livetemplate"

type ProfileStore struct {
    avatarPath string
}

// AllowUploads configures which uploads are accepted
func (s *ProfileStore) AllowUploads() map[string]livetemplate.UploadConfig {
    return map[string]livetemplate.UploadConfig{
        "avatar": {
            Accept:      []string{"image/*"},
            MaxFileSize: 5 * 1024 * 1024, // 5MB
            MaxEntries:  1,                // Single file
            AutoUpload:  true,
        },
    }
}

// ConsumeUpload processes uploaded files
func (s *ProfileStore) ConsumeUpload(ctx context.Context, name string, entries []*livetemplate.UploadEntry) error {
    if name != "avatar" {
        return nil
    }

    for _, entry := range entries {
        // Move file from temp location to permanent storage
        s.avatarPath = entry.TempPath
        // Or process the file...
    }

    return nil
}
```

### 2. Add Upload Input to Template

```html
<form lvt-submit="updateProfile">
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

### 3. Initialize Client (Browser)

The LiveTemplate client automatically detects and handles file uploads:

```typescript
import { LiveTemplateClient } from '@livetemplate/client';

const client = new LiveTemplateClient();
await client.connect();

// Listen for upload events
document.querySelector('[data-lvt-id]').addEventListener('lvt:upload:progress', (e) => {
    console.log(`Progress: ${e.detail.entry.progress}%`);
});
```

## Server API

### UploadConfig

Configures upload behavior for a specific field:

```go
type UploadConfig struct {
    // Accept specifies allowed MIME types or extensions
    // Examples: ["image/*"], ["image/jpeg", "image/png"], [".pdf", ".doc"]
    Accept []string

    // MaxEntries limits number of files (0 = unlimited)
    MaxEntries int

    // MaxFileSize limits file size in bytes (0 = unlimited)
    MaxFileSize int64

    // AutoUpload triggers upload on file selection (default: false)
    AutoUpload bool

    // ChunkSize for WebSocket uploads in bytes (default: 256KB)
    ChunkSize int

    // External presigner for S3/cloud uploads (optional)
    External Presigner
}
```

### UploadEntry

Represents a single uploaded file:

```go
type UploadEntry struct {
    ID          string    // Server-generated unique ID
    ClientName  string    // Original filename from client
    ClientType  string    // MIME type
    ClientSize  int64     // File size in bytes
    Progress    int       // Upload progress 0-100
    Valid       bool      // Passed validation
    Done        bool      // Upload complete
    Error       string    // Validation/upload error
    TempPath    string    // Path to temporary file (server-side uploads)
    BytesRecv   int64     // Bytes received (for progress)
    ExternalRef string    // S3 URL or external reference
    CreatedAt   time.Time
    CompletedAt time.Time
}
```

### UploadAware Interface

Stores implement this interface to support uploads:

```go
type UploadAware interface {
    // AllowUploads returns upload configurations by field name
    AllowUploads() map[string]UploadConfig

    // ConsumeUpload processes completed uploads
    // Only called for valid, completed entries
    ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error
}
```

### Presigner Interface

For external upload providers (S3, Azure, etc.):

```go
type Presigner interface {
    // Presign generates a presigned upload URL
    Presign(entry *UploadEntry) (UploadMeta, error)
}

type UploadMeta struct {
    Uploader string            // Provider name (e.g., "s3")
    URL      string            // Presigned upload URL
    Fields   map[string]string // Form fields for POST
    Headers  map[string]string // HTTP headers for PUT
}
```

## Template Helpers

### `.lvt.Uploads "name"`

Iterate over upload entries for a specific field:

```html
{{range .lvt.Uploads "avatar"}}
    <div class="upload">
        <img src="/uploads/{{.ID}}" alt="{{.ClientName}}" />
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
import "github.com/livetemplate/livetemplate"

// Configure S3
s3Config := livetemplate.S3Config{
    Bucket:    "my-uploads",
    Region:    "us-east-1",
    KeyPrefix: "uploads",           // Optional: organizes S3 keys
    Expiry:    15 * time.Minute,    // Presigned URL expiry

    // Option 1: Use IAM role (recommended for production)
    // Credentials are auto-detected from environment

    // Option 2: Use static credentials
    AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
    SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),

    // Option 3: Use custom endpoint (MinIO, LocalStack)
    Endpoint: "http://localhost:9000",
}

presigner, err := livetemplate.NewS3Presigner(s3Config)
if err != nil {
    log.Fatal(err)
}

// Use in UploadConfig
func (s *Store) AllowUploads() map[string]livetemplate.UploadConfig {
    return map[string]livetemplate.UploadConfig{
        "photos": {
            Accept:      []string{"image/*"},
            MaxFileSize: 10 * 1024 * 1024,
            External:    presigner, // Client uploads directly to S3
        },
    }
}
```

### S3 Upload Flow

1. **Client selects file** → Sends file metadata to server
2. **Server generates presigned URL** → Returns to client
3. **Client uploads directly to S3** → No server bandwidth used
4. **Client notifies server** → Server stores S3 key in `ExternalRef`
5. **ConsumeUpload called** → Process S3 reference

### Custom External Uploader

Implement the `Presigner` interface for other providers:

```go
type AzurePresigner struct {
    config AzureConfig
}

func (p *AzurePresigner) Presign(entry *livetemplate.UploadEntry) (livetemplate.UploadMeta, error) {
    // Generate Azure SAS token
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

## Upload Modes

### HTTP Multipart (Simple)

Best for small files (<10MB) and simple forms:

```html
<form action="/profile" method="POST" enctype="multipart/form-data">
    <input type="file" name="avatar" lvt-upload="avatar" />
    <button type="submit">Upload</button>
</form>
```

- ✅ Simple, works everywhere
- ✅ No JavaScript required
- ❌ No progress tracking
- ❌ Blocking (waits for upload)

### WebSocket Chunked (Large Files)

Best for large files (>10MB) with progress tracking:

```html
<form lvt-submit="saveProfile">
    <input type="file" lvt-upload="documents" />
    <button type="submit">Upload</button>
</form>

{{range .lvt.Uploads "documents"}}
    <progress value="{{.Progress}}" max="100"></progress>
{{end}}
```

- ✅ Real-time progress
- ✅ Non-blocking
- ✅ Handles large files
- ✅ Can cancel mid-upload
- ❌ Requires WebSocket

### External (S3/Cloud)

Best for scalability and CDN integration:

```go
func (s *Store) AllowUploads() map[string]livetemplate.UploadConfig {
    return map[string]livetemplate.UploadConfig{
        "files": {
            External: s3Presigner, // Direct to S3
        },
    }
}
```

- ✅ Offloads server bandwidth
- ✅ Scales infinitely
- ✅ CDN-ready
- ❌ Requires cloud account
- ❌ Additional cost

## Client Library (TypeScript)

### Automatic File Input Binding

The client automatically detects `lvt-upload` attributes:

```html
<input type="file" lvt-upload="avatar" accept="image/*" />
```

No manual initialization required!

### Upload Events

Listen for upload lifecycle events:

```typescript
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

### Manual Upload Control

For advanced use cases:

```typescript
import { LiveTemplateClient } from '@livetemplate/client';

const client = new LiveTemplateClient();
await client.connect();

// Access upload handler
const uploadHandler = client.uploadHandler;

// Start upload programmatically
const files = [...fileInput.files];
await uploadHandler.startUpload('documents', files);

// Cancel upload
uploadHandler.cancelUpload(entryId);

// Register custom uploader
uploadHandler.registerUploader('custom', new CustomUploader());
```

## Validation

### Server-Side Validation

Uploads are validated against `UploadConfig`:

```go
livetemplate.UploadConfig{
    Accept:      []string{"image/jpeg", "image/png", ".jpg", ".png"},
    MaxFileSize: 5 * 1024 * 1024,  // 5MB
    MaxEntries:  3,                 // Max 3 files
}
```

**Validation checks:**
- ✅ File type (MIME type or extension)
- ✅ File size
- ✅ File count
- ✅ Custom validation in `ConsumeUpload()`

**Invalid files:**
- Marked as `Valid: false`
- Set `Error` field with reason
- NOT passed to `ConsumeUpload()`
- Temp files cleaned up automatically

### Client-Side Validation (Future)

Coming in Phase 5.3:
- Pre-upload validation
- Instant feedback
- Prevent invalid uploads

## Security

### File Type Validation

Uses both MIME type and extension:

```go
Accept: []string{
    "image/*",        // Any image MIME type
    "image/jpeg",     // Specific MIME type
    ".jpg", ".png",   // File extensions
}
```

**Security notes:**
- MIME types can be spoofed
- Extensions are fallback
- Always validate file content in `ConsumeUpload()`

### File Size Limits

Prevent DOS attacks:

```go
MaxFileSize: 10 * 1024 * 1024, // 10MB limit
```

### Path Traversal Prevention

S3 keys are sanitized:

```go
// Client sends: "../../../etc/passwd"
// Server generates: "uploads/entry-123/passwd"
```

Uses `filepath.Base()` to extract filename only.

### Temporary File Security

- Created in system temp dir with restricted permissions
- Random entry IDs prevent guessing
- Automatic cleanup on connection close
- Configurable cleanup interval

## Performance

### Chunked Upload Sizes

Default: 256KB chunks

**Tuning:**
```go
ChunkSize: 512 * 1024, // 512KB chunks
```

- Smaller chunks = more overhead, better progress granularity
- Larger chunks = less overhead, coarser progress updates

### Memory Usage

- Multipart uploads: Streamed to disk (constant memory)
- Chunked uploads: One chunk in memory at a time
- External uploads: No server memory used

### Concurrent Uploads

- Each WebSocket connection has independent upload registry
- Multiple files can upload simultaneously
- No server-side limits (limited by client)

## Error Handling

### Upload Errors

Errors are captured in `UploadEntry.Error`:

```html
{{range .lvt.Uploads "files"}}
    {{if .Error}}
        <div class="error">
            {{.ClientName}}: {{.Error}}
        </div>
    {{end}}
{{end}}
```

### ConsumeUpload Errors

Return errors from `ConsumeUpload()`:

```go
func (s *Store) ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error {
    for _, entry := range entries {
        if err := s.processFile(entry.TempPath); err != nil {
            return fmt.Errorf("failed to process %s: %w", entry.ClientName, err)
        }
    }
    return nil
}
```

Errors are:
- Logged server-side
- Displayed in template via `.lvt.HasUploadError`
- Sent to client via update response

## Best Practices

### 1. Validate File Content

Don't trust MIME types:

```go
func (s *Store) ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error {
    for _, entry := range entries {
        // Verify it's actually an image
        img, err := image.Decode(openFile(entry.TempPath))
        if err != nil {
            return fmt.Errorf("invalid image: %w", err)
        }
    }
    return nil
}
```

### 2. Move Files Atomically

```go
// Bad: leaves corrupt files
os.Rename(entry.TempPath, finalPath)

// Good: atomic move
if err := os.Rename(entry.TempPath, finalPath); err != nil {
    return fmt.Errorf("failed to save file: %w", err)
}
```

### 3. Clean Up Temporary Files

Temp files are automatically cleaned up on connection close, but you can also:

```go
func (s *Store) ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error {
    for _, entry := range entries {
        // Process file
        if err := s.saveFile(entry.TempPath); err != nil {
            return err
        }

        // Explicitly clean up temp file
        os.Remove(entry.TempPath)
    }
    return nil
}
```

### 4. Use External Uploads for Large Files

For files >10MB, use S3/cloud storage:
- Offloads server bandwidth
- Better user experience
- Enables CDN delivery

### 5. Set Appropriate Limits

```go
livetemplate.UploadConfig{
    Accept:      []string{"image/*"},
    MaxFileSize: 10 * 1024 * 1024, // 10MB - adjust for your needs
    MaxEntries:  5,                 // Limit simultaneous uploads
}
```

## Troubleshooting

### "Upload not configured" Error

Ensure your store implements `UploadAware`:

```go
func (s *Store) AllowUploads() map[string]livetemplate.UploadConfig {
    return map[string]livetemplate.UploadConfig{
        "avatar": {...},
    }
}
```

### Progress Not Updating

- Ensure WebSocket is connected (not HTTP fallback)
- Check browser console for errors
- Verify `ChunkSize` is set appropriately

### Files Not Appearing in ConsumeUpload

Check validation errors:

```html
{{range .lvt.Uploads "files"}}
    {{if not .Valid}}
        Error: {{.Error}}
    {{end}}
{{end}}
```

### S3 Upload Fails

- Verify AWS credentials
- Check S3 bucket CORS configuration
- Ensure presigned URL hasn't expired
- Check S3 bucket permissions

## Migration Guide

**From v0.2.x to v0.3.x:**

No breaking changes! Upload feature is additive.

To adopt uploads:
1. Implement `UploadAware` on stores
2. Add `lvt-upload` attributes to file inputs
3. Update client library to v0.1.0+

## API Reference

See also:
- [upload.go](../upload.go) - Public API types
- [mount.go](../mount.go) - HTTP/WebSocket handlers
- [s3_presigner.go](../s3_presigner.go) - S3 implementation
- [Client Library](https://github.com/livetemplate/client) - TypeScript client
