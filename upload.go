package livetemplate

import (
	"io"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// UploadConfig configures file upload behavior for a specific upload field.
// It mirrors Phoenix LiveView's allow_upload/3 configuration pattern.
//
// Fields:
//   - Accept: Allowed MIME types or extensions (e.g., []string{"image/*", ".pdf"})
//   - MaxEntries: Maximum number of concurrent files (0 = unlimited)
//   - MaxFileSize: Maximum file size in bytes (0 = unlimited)
//   - AutoUpload: Whether to start upload automatically on file selection
//   - ChunkSize: Chunk size for WebSocket uploads in bytes (default: 256KB)
//   - Mode: Upload destination (Volume | Direct | Proxied | Preview; default Volume)
//   - External: Presigner for Direct mode (browser → cloud via presigned URL)
//   - Dir: Retain destination for Volume mode (defaults to a temp dir)
type UploadConfig = uploadtypes.UploadConfig

// UploadMode selects where a field's uploaded bytes go. Configured purely
// server-side; the same <input lvt-upload="..."> markup works for every mode.
type UploadMode = uploadtypes.UploadMode

const (
	// UploadModeVolume stages bytes to the server's disk and retains them at Dir.
	UploadModeVolume = uploadtypes.UploadModeVolume
	// UploadModeDirect uploads browser → cloud via a presigned URL (needs External).
	UploadModeDirect = uploadtypes.UploadModeDirect
	// UploadModeProxied streams bytes through the server to remote storage, zero disk
	// (the controller must implement UploadStreamer).
	UploadModeProxied = uploadtypes.UploadModeProxied
	// UploadModePreview keeps the file on the device; the server gets metadata only.
	UploadModePreview = uploadtypes.UploadModePreview
)

// UploadStreamer is optionally implemented by a Controller to receive Proxied
// uploads inline as an io.Reader, with zero local-disk staging. OnUpload is
// invoked once per file part while the bytes are still in flight; the handler
// streams them to remote storage and records the final reference via SetResult.
//
//	func (c *C) OnUpload(part *livetemplate.UploadPart, ctx *livetemplate.Context) error {
//	    ref, err := myBackend.Put(ctx, part.Filename, part) // part is an io.Reader
//	    if err != nil {
//	        return err
//	    }
//	    part.SetResult(ref)
//	    return nil
//	}
type UploadStreamer interface {
	OnUpload(part *UploadPart, ctx *Context) error
}

// UploadPart is a single streaming upload handed to Controller.OnUpload. The
// embedded io.Reader yields the file bytes directly off the HTTP request;
// reading past MaxFileSize returns ErrUploadTooLarge (the read must be aborted,
// not treated as a complete short file). The reader is consumed once, inline
// with the request — no seek or replay.
type UploadPart struct {
	io.Reader
	Field      string // Upload field name (disambiguates a shared OnUpload)
	Filename   string // Original client filename
	ClientType string // Client-reported MIME type
	ClientSize int64  // Client-reported size, or -1 when unknown
	entry      *uploadtypes.UploadEntry
}

// SetResult records the final remote-storage reference for this part (e.g. an
// object URL or key). The follow-on action reads it via
// ctx.GetCompletedUploads(field)[i].ExternalRef.
func (p *UploadPart) SetResult(ref string) {
	if p.entry != nil {
		p.entry.ExternalRef = ref
	}
}

// ErrUploadTooLarge is returned by a UploadPart reader when the streamed bytes
// exceed the field's MaxFileSize. It is a distinct sentinel (not io.EOF) so a
// streaming copy aborts instead of committing a truncated object.
var ErrUploadTooLarge = uploadtypes.ErrUploadTooLarge

// Presigner generates presigned upload URLs for external storage (S3, GCS, etc).
// This enables direct client-to-storage uploads, bypassing the server.
//
// Implementations should return an error if presigning fails due to:
//   - Invalid or expired credentials
//   - Network connectivity issues
//   - Storage service errors
//   - Invalid upload configuration
type Presigner = uploadtypes.Presigner

// UploadMeta contains presigned upload configuration for external storage.
//
// Fields:
//   - Uploader: Client-side uploader identifier (e.g., "s3", "gcs", "azure")
//   - URL: Presigned upload endpoint URL
//   - Fields: Form fields for multipart/form-data POST requests (optional)
//   - Headers: HTTP headers required for the upload request (e.g., Content-Type)
type UploadMeta = uploadtypes.UploadMeta

// UploadEntry represents a single file upload with its state and metadata.
// Exposed to templates via .lvt.Uploads(name).
//
// Fields:
//   - ID: Unique identifier for this upload entry
//   - ClientName: Original filename from the client
//   - ClientType: MIME type reported by the client
//   - ClientSize: File size in bytes
//   - Progress: Upload progress percentage (0-100)
//   - Valid: Whether the upload passed validation
//   - Done: Whether the upload has completed
//   - Error: Error message if validation or upload failed
//   - TempPath: Server-side temporary file path (server uploads only)
//   - BytesRecv: Bytes received so far (for progress tracking)
//   - ExternalRef: Final storage reference (external uploads only, e.g., S3 URL)
//   - CreatedAt: When the upload entry was created
//   - CompletedAt: When the upload completed (zero if not done)
type UploadEntry = uploadtypes.UploadEntry

// Internal upload support (used by mount handler, not part of public API)

// uploadRegistry is an internal interface for upload registries.
// Implemented by internal/upload.Registry.
type uploadRegistry interface {
	CreateUpload(name string, config UploadConfig) error
	GetUpload(name string) interface{}
	// UploadAccessor methods for Context
	HasUploads(name string) bool
	GetCompletedUploads(name string) []*uploadtypes.UploadEntry
}

// uploadTempFileManager is an internal interface for temp file managers.
// Implemented by internal/upload.TempFileManager.
type uploadTempFileManager interface {
	CreateTempFile(sessionID, uploadName, entryID string) (string, error)
	RemoveSession(sessionID string) error
}

// newUploadRegistry creates a new upload registry.
// This is an internal factory function set at runtime to avoid import cycles.
var newUploadRegistry func() uploadRegistry

// newUploadTempFileManager creates a new temp file manager.
// This is an internal factory function set at runtime to avoid import cycles.
var newUploadTempFileManager func(baseDir string) (uploadTempFileManager, error)
