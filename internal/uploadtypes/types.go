package uploadtypes

import (
	"context"
	"time"
)

// Presigner generates presigned upload URLs for external storage.
type Presigner interface {
	Presign(entry *UploadEntry) (UploadMeta, error)
}

// UploadMeta contains presigned upload configuration for external storage.
type UploadMeta struct {
	Uploader string
	URL      string
	Fields   map[string]string
	Headers  map[string]string
}

// UploadConfig configures file upload behavior for a specific upload field.
type UploadConfig struct {
	Accept      []string
	MaxEntries  int
	MaxFileSize int64
	AutoUpload  bool
	ChunkSize   int
	External    Presigner
}

// UploadEntry represents a single file upload with its state and metadata.
type UploadEntry struct {
	ID          string
	ClientName  string
	ClientType  string
	ClientSize  int64
	Progress    int
	Valid       bool
	Done        bool
	Error       string
	TempPath    string
	BytesRecv   int64 // Bytes received so far (for chunked uploads)
	ExternalRef string
	CreatedAt   time.Time
	CompletedAt time.Time
}

// Deprecated: UploadAware is the legacy upload interface. Use WithUpload() option
// combined with Context.HasUploads() and Context.GetCompletedUploads() instead.
// See docs/references/uploads.md for the current API.
type UploadAware interface {
	AllowUploads() map[string]UploadConfig
	ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error
}
