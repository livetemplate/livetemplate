package livetemplate

import (
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
//   - External: Optional Presigner for direct-to-storage uploads (S3, GCS, etc.)
type UploadConfig = uploadtypes.UploadConfig

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
