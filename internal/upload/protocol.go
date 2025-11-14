package upload

import (
	"encoding/json"
	"fmt"
)

// FileMetadata represents metadata for a file to be uploaded.
// Sent by client in UploadStartMessage before chunks are transmitted.
type FileMetadata struct {
	Name string `json:"name"` // Client-side filename
	Type string `json:"type"` // MIME type (e.g., "image/jpeg")
	Size int64  `json:"size"` // Total file size in bytes
}

// UploadStartMessage is sent by client to initiate upload(s).
// Server responds with entry IDs for tracking chunks.
type UploadStartMessage struct {
	Action     string         `json:"action"`      // Always "upload_start"
	UploadName string         `json:"upload_name"` // Field name (e.g., "avatar", "documents")
	Files      []FileMetadata `json:"files"`       // Metadata for each file to upload
}

// UploadStartResponse is sent by server after upload_start.
// Maps client file indices to server-generated entry IDs.
type UploadStartResponse struct {
	UploadName string            `json:"upload_name"` // Field name
	Entries    []UploadEntryInfo `json:"entries"`     // Entry info for each file
}

// UploadEntryInfo contains entry ID and validation status.
type UploadEntryInfo struct {
	EntryID    string            `json:"entry_id"`    // Server-generated ID for tracking chunks
	ClientName string            `json:"client_name"` // Original filename
	Valid      bool              `json:"valid"`       // Whether entry passed initial validation
	Error      string            `json:"error"`       // Error message if validation failed
	External   *ExternalUploadMeta `json:"external,omitempty"` // Presigned upload metadata (if External configured)
}

// ExternalUploadMeta contains presigned upload configuration for external storage.
type ExternalUploadMeta struct {
	Uploader string            `json:"uploader"` // Uploader type (e.g., "s3")
	URL      string            `json:"url"`      // Presigned URL for upload
	Fields   map[string]string `json:"fields"`   // Form fields for multipart POST
	Headers  map[string]string `json:"headers"`  // HTTP headers for request
}

// UploadChunkMessage is sent by client to transmit file data.
// Chunks are sent sequentially until file is complete.
type UploadChunkMessage struct {
	Action      string `json:"action"`       // Always "upload_chunk"
	EntryID     string `json:"entry_id"`     // Server-provided entry ID
	ChunkBase64 string `json:"chunk_base64"` // Base64-encoded chunk data
	Offset      int64  `json:"offset"`       // Byte offset in file (for verification)
	Total       int64  `json:"total"`        // Total file size (for progress calculation)
}

// UploadProgressMessage is broadcast by server to client(s).
// Allows displaying upload progress in UI.
type UploadProgressMessage struct {
	Type       string `json:"type"`        // Always "upload_progress"
	UploadName string `json:"upload_name"` // Field name
	EntryID    string `json:"entry_id"`    // Entry being uploaded
	ClientName string `json:"client_name"` // Filename for display
	Progress   int    `json:"progress"`    // Percentage complete (0-100)
	BytesRecv  int64  `json:"bytes_recv"`  // Bytes received so far
	BytesTotal int64  `json:"bytes_total"` // Total file size
}

// UploadCompleteMessage is sent by client after all chunks transmitted.
// Server marks entries as done and calls ConsumeUpload.
type UploadCompleteMessage struct {
	Action     string   `json:"action"`      // Always "upload_complete"
	UploadName string   `json:"upload_name"` // Field name
	EntryIDs   []string `json:"entry_ids"`   // Entry IDs to mark complete
}

// UploadCompleteResponse is sent by server after ConsumeUpload.
type UploadCompleteResponse struct {
	UploadName string `json:"upload_name"` // Field name
	Success    bool   `json:"success"`     // Whether ConsumeUpload succeeded
	Error      string `json:"error"`       // Error message if failed
}

// CancelUploadMessage is sent by client to cancel an in-progress upload.
// Server cleans up temp files and removes entry from registry.
type CancelUploadMessage struct {
	Action  string `json:"action"`   // Always "cancel_upload"
	EntryID string `json:"entry_id"` // Entry to cancel
}

// CancelUploadResponse is sent by server after cancellation.
type CancelUploadResponse struct {
	EntryID string `json:"entry_id"` // Entry that was cancelled
	Success bool   `json:"success"`  // Whether cancellation succeeded
}

// ParseUploadStartMessage parses an upload_start action from JSON.
func ParseUploadStartMessage(data []byte) (*UploadStartMessage, error) {
	var msg UploadStartMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse upload_start message: %w", err)
	}

	if msg.Action != "upload_start" {
		return nil, fmt.Errorf("expected action 'upload_start', got %q", msg.Action)
	}

	if msg.UploadName == "" {
		return nil, fmt.Errorf("upload_name is required")
	}

	if len(msg.Files) == 0 {
		return nil, fmt.Errorf("files array is empty")
	}

	// Validate file metadata
	for i, file := range msg.Files {
		if file.Name == "" {
			return nil, fmt.Errorf("file[%d]: name is required", i)
		}
		if file.Size <= 0 {
			return nil, fmt.Errorf("file[%d]: size must be positive", i)
		}
	}

	return &msg, nil
}

// ParseUploadChunkMessage parses an upload_chunk action from JSON.
func ParseUploadChunkMessage(data []byte) (*UploadChunkMessage, error) {
	var msg UploadChunkMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse upload_chunk message: %w", err)
	}

	if msg.Action != "upload_chunk" {
		return nil, fmt.Errorf("expected action 'upload_chunk', got %q", msg.Action)
	}

	if msg.EntryID == "" {
		return nil, fmt.Errorf("entry_id is required")
	}

	if msg.ChunkBase64 == "" {
		return nil, fmt.Errorf("chunk_base64 is required")
	}

	if msg.Offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}

	if msg.Total <= 0 {
		return nil, fmt.Errorf("total must be positive")
	}

	return &msg, nil
}

// ParseUploadCompleteMessage parses an upload_complete action from JSON.
func ParseUploadCompleteMessage(data []byte) (*UploadCompleteMessage, error) {
	var msg UploadCompleteMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse upload_complete message: %w", err)
	}

	if msg.Action != "upload_complete" {
		return nil, fmt.Errorf("expected action 'upload_complete', got %q", msg.Action)
	}

	if msg.UploadName == "" {
		return nil, fmt.Errorf("upload_name is required")
	}

	if len(msg.EntryIDs) == 0 {
		return nil, fmt.Errorf("entry_ids array is empty")
	}

	return &msg, nil
}

// ParseCancelUploadMessage parses a cancel_upload action from JSON.
func ParseCancelUploadMessage(data []byte) (*CancelUploadMessage, error) {
	var msg CancelUploadMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse cancel_upload message: %w", err)
	}

	if msg.Action != "cancel_upload" {
		return nil, fmt.Errorf("expected action 'cancel_upload', got %q", msg.Action)
	}

	if msg.EntryID == "" {
		return nil, fmt.Errorf("entry_id is required")
	}

	return &msg, nil
}

// SerializeUploadStartResponse serializes UploadStartResponse to JSON.
func SerializeUploadStartResponse(resp *UploadStartResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// SerializeUploadProgressMessage serializes UploadProgressMessage to JSON.
func SerializeUploadProgressMessage(msg *UploadProgressMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// SerializeUploadCompleteResponse serializes UploadCompleteResponse to JSON.
func SerializeUploadCompleteResponse(resp *UploadCompleteResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// SerializeCancelUploadResponse serializes CancelUploadResponse to JSON.
func SerializeCancelUploadResponse(resp *CancelUploadResponse) ([]byte, error) {
	return json.Marshal(resp)
}
