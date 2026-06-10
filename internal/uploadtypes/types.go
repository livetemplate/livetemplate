package uploadtypes

import (
	"context"
	"errors"
	"time"
)

// ErrUploadTooLarge is returned by a streaming upload reader when the bytes
// exceed MaxFileSize. It is a distinct sentinel (not io.EOF) so a streaming
// copy aborts instead of committing a truncated object.
var ErrUploadTooLarge = errors.New("livetemplate: upload exceeds MaxFileSize")

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

// UploadMode selects where a field's uploaded bytes go. It is chosen purely by
// server config; the same <input lvt-upload="..."> markup works for every mode.
type UploadMode int

const (
	// UploadModeVolume stages bytes to the server's disk and retains them at the
	// configured Dir (or a temp dir). The app reads the path via TempPath.
	UploadModeVolume UploadMode = iota
	// UploadModeDirect has the browser upload straight to cloud storage via a
	// presigned URL (requires External). Bytes never touch the server.
	UploadModeDirect
	// UploadModeProxied streams bytes through the server to remote storage with
	// zero local-disk staging (requires the controller to implement UploadStreamer).
	UploadModeProxied
	// UploadModePreview keeps the file on the device; the server receives only
	// metadata (name/type/size). Visual-only — bytes are never uploaded.
	UploadModePreview
)

// String renders the mode as the lowercase token sent to the client.
func (m UploadMode) String() string {
	switch m {
	case UploadModeDirect:
		return "direct"
	case UploadModeProxied:
		return "proxied"
	case UploadModePreview:
		return "preview"
	default:
		return "volume"
	}
}

// UploadConfig configures file upload behavior for a specific upload field.
type UploadConfig struct {
	Accept      []string
	MaxEntries  int
	MaxFileSize int64 // Max bytes per file; 0 = no limit (also the only size cap for Proxied)
	AutoUpload  bool
	ChunkSize   int
	Mode        UploadMode // Selects the upload destination (default: Volume)
	External    Presigner  // Required when Mode == UploadModeDirect
	Dir         string     // Retain destination when Mode == UploadModeVolume (temp if empty)
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
	Preview     bool // True when the file stays on-device (UploadModePreview); no bytes received
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
