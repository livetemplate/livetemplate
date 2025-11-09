package livetemplate

import (
	"context"
	"time"
)

// UploadConfig configures file upload behavior for a specific upload field.
// It mirrors Phoenix LiveView's allow_upload/3 configuration pattern.
type UploadConfig struct {
	// Accept specifies allowed file types as MIME types or extensions.
	// Examples: []string{"image/png", "image/jpeg", ".pdf"}
	Accept []string

	// MaxEntries limits concurrent uploads. Default is 1.
	MaxEntries int

	// MaxFileSize limits individual file size in bytes.
	// Example: 10 * 1024 * 1024 for 10MB
	MaxFileSize int64

	// AutoUpload triggers upload automatically on file selection.
	// Default is false (manual trigger required).
	AutoUpload bool

	// ChunkSize determines chunk size for WebSocket uploads in bytes.
	// Default is 1MB. Only used for server-side uploads.
	ChunkSize int

	// External provides presigned upload configuration for external storage.
	// When set, files upload directly to S3/cloud instead of server.
	External Presigner
}

// Presigner generates presigned upload URLs for external storage (S3, GCS, etc).
// This enables direct client-to-storage uploads, bypassing the server.
type Presigner interface {
	// Presign generates presigned upload configuration for an entry.
	// Returns metadata including URL, form fields, and headers.
	Presign(entry *UploadEntry) (UploadMeta, error)
}

// UploadMeta contains presigned upload configuration for external storage.
type UploadMeta struct {
	// Uploader identifies the client-side uploader to use (e.g., "S3", "GCS").
	Uploader string

	// URL is the presigned upload endpoint.
	URL string

	// Fields contains form fields required for the upload POST request.
	Fields map[string]string

	// Headers contains HTTP headers required for the upload.
	Headers map[string]string
}

// UploadEntry represents a single file upload with its state and metadata.
// Exposed to templates via .lvt.Uploads(name).
type UploadEntry struct {
	// ID uniquely identifies this upload entry within a session.
	ID string

	// ClientName is the original filename from the client.
	ClientName string

	// ClientType is the MIME type reported by the client.
	ClientType string

	// ClientSize is the file size in bytes reported by the client.
	ClientSize int64

	// Progress tracks upload completion percentage (0-100).
	Progress int

	// Valid indicates whether the entry passed validation.
	Valid bool

	// Done indicates whether the upload has completed.
	Done bool

	// Error contains any error message if validation or upload failed.
	Error string

	// TempPath is the server-side temporary file path for server uploads.
	// Only populated for server-side uploads, empty for external uploads.
	TempPath string

	// ExternalRef is the final storage reference for external uploads.
	// Populated after external upload completes (e.g., S3 key or URL).
	ExternalRef string

	// CreatedAt tracks when the entry was created.
	CreatedAt time.Time

	// CompletedAt tracks when the upload completed.
	CompletedAt time.Time
}

// UploadAware is an optional interface that stores can implement to support uploads.
// When implemented, the mount handler automatically manages upload lifecycle.
type UploadAware interface {
	// AllowUploads returns upload configurations keyed by field name.
	// Called once during initialization to configure allowed uploads.
	AllowUploads() map[string]UploadConfig

	// ConsumeUpload processes completed uploads for a specific field.
	// Called after all entries for a field have successfully uploaded.
	// The store should move/process files from TempPath or ExternalRef.
	ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error
}
