package upload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseUploadStartMessage(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *UploadStartMessage)
	}{
		{
			name: "valid single file",
			json: `{
				"action": "upload_start",
				"upload_name": "avatar",
				"files": [
					{"name": "photo.jpg", "type": "image/jpeg", "size": 1024}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *UploadStartMessage) {
				if msg.Action != "upload_start" {
					t.Errorf("Expected action 'upload_start', got %q", msg.Action)
				}
				if msg.UploadName != "avatar" {
					t.Errorf("Expected upload_name 'avatar', got %q", msg.UploadName)
				}
				if len(msg.Files) != 1 {
					t.Errorf("Expected 1 file, got %d", len(msg.Files))
				}
				if msg.Files[0].Name != "photo.jpg" {
					t.Errorf("Expected filename 'photo.jpg', got %q", msg.Files[0].Name)
				}
				if msg.Files[0].Size != 1024 {
					t.Errorf("Expected size 1024, got %d", msg.Files[0].Size)
				}
			},
		},
		{
			name: "valid multiple files",
			json: `{
				"action": "upload_start",
				"upload_name": "documents",
				"files": [
					{"name": "doc1.pdf", "type": "application/pdf", "size": 5000},
					{"name": "doc2.txt", "type": "text/plain", "size": 2000}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *UploadStartMessage) {
				if len(msg.Files) != 2 {
					t.Errorf("Expected 2 files, got %d", len(msg.Files))
				}
			},
		},
		{
			name: "missing action",
			json: `{
				"upload_name": "avatar",
				"files": [{"name": "photo.jpg", "type": "image/jpeg", "size": 1024}]
			}`,
			wantErr: true,
			errMsg:  "expected action 'upload_start'",
		},
		{
			name: "wrong action",
			json: `{
				"action": "upload_chunk",
				"upload_name": "avatar",
				"files": [{"name": "photo.jpg", "type": "image/jpeg", "size": 1024}]
			}`,
			wantErr: true,
			errMsg:  "expected action 'upload_start'",
		},
		{
			name: "missing upload_name",
			json: `{
				"action": "upload_start",
				"files": [{"name": "photo.jpg", "type": "image/jpeg", "size": 1024}]
			}`,
			wantErr: true,
			errMsg:  "upload_name is required",
		},
		{
			name: "empty files array",
			json: `{
				"action": "upload_start",
				"upload_name": "avatar",
				"files": []
			}`,
			wantErr: true,
			errMsg:  "files array is empty",
		},
		{
			name: "file missing name",
			json: `{
				"action": "upload_start",
				"upload_name": "avatar",
				"files": [{"type": "image/jpeg", "size": 1024}]
			}`,
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "file with zero size",
			json: `{
				"action": "upload_start",
				"upload_name": "avatar",
				"files": [{"name": "photo.jpg", "type": "image/jpeg", "size": 0}]
			}`,
			wantErr: true,
			errMsg:  "size must be positive",
		},
		{
			name: "file with negative size",
			json: `{
				"action": "upload_start",
				"upload_name": "avatar",
				"files": [{"name": "photo.jpg", "type": "image/jpeg", "size": -100}]
			}`,
			wantErr: true,
			errMsg:  "size must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseUploadStartMessage([]byte(tt.json))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func TestParseUploadChunkMessage(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *UploadChunkMessage)
	}{
		{
			name: "valid chunk",
			json: `{
				"action": "upload_chunk",
				"entry_id": "entry-123",
				"chunk_base64": "SGVsbG8gV29ybGQ=",
				"offset": 0,
				"total": 11
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *UploadChunkMessage) {
				if msg.Action != "upload_chunk" {
					t.Errorf("Expected action 'upload_chunk', got %q", msg.Action)
				}
				if msg.EntryID != "entry-123" {
					t.Errorf("Expected entry_id 'entry-123', got %q", msg.EntryID)
				}
				if msg.ChunkBase64 != "SGVsbG8gV29ybGQ=" {
					t.Errorf("Expected chunk_base64 'SGVsbG8gV29ybGQ=', got %q", msg.ChunkBase64)
				}
				if msg.Offset != 0 {
					t.Errorf("Expected offset 0, got %d", msg.Offset)
				}
				if msg.Total != 11 {
					t.Errorf("Expected total 11, got %d", msg.Total)
				}
			},
		},
		{
			name: "valid chunk with offset",
			json: `{
				"action": "upload_chunk",
				"entry_id": "entry-456",
				"chunk_base64": "Y2h1bmsy",
				"offset": 1024,
				"total": 2048
			}`,
			wantErr: false,
		},
		{
			name: "wrong action",
			json: `{
				"action": "upload_start",
				"entry_id": "entry-123",
				"chunk_base64": "data",
				"offset": 0,
				"total": 100
			}`,
			wantErr: true,
			errMsg:  "expected action 'upload_chunk'",
		},
		{
			name: "missing entry_id",
			json: `{
				"action": "upload_chunk",
				"chunk_base64": "data",
				"offset": 0,
				"total": 100
			}`,
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "missing chunk_base64",
			json: `{
				"action": "upload_chunk",
				"entry_id": "entry-123",
				"offset": 0,
				"total": 100
			}`,
			wantErr: true,
			errMsg:  "chunk_base64 is required",
		},
		{
			name: "negative offset",
			json: `{
				"action": "upload_chunk",
				"entry_id": "entry-123",
				"chunk_base64": "data",
				"offset": -1,
				"total": 100
			}`,
			wantErr: true,
			errMsg:  "offset must be non-negative",
		},
		{
			name: "zero total",
			json: `{
				"action": "upload_chunk",
				"entry_id": "entry-123",
				"chunk_base64": "data",
				"offset": 0,
				"total": 0
			}`,
			wantErr: true,
			errMsg:  "total must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseUploadChunkMessage([]byte(tt.json))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func TestParseUploadCompleteMessage(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *UploadCompleteMessage)
	}{
		{
			name: "valid single entry",
			json: `{
				"action": "upload_complete",
				"upload_name": "avatar",
				"entry_ids": ["entry-123"]
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *UploadCompleteMessage) {
				if msg.Action != "upload_complete" {
					t.Errorf("Expected action 'upload_complete', got %q", msg.Action)
				}
				if msg.UploadName != "avatar" {
					t.Errorf("Expected upload_name 'avatar', got %q", msg.UploadName)
				}
				if len(msg.EntryIDs) != 1 {
					t.Errorf("Expected 1 entry, got %d", len(msg.EntryIDs))
				}
				if msg.EntryIDs[0] != "entry-123" {
					t.Errorf("Expected entry_id 'entry-123', got %q", msg.EntryIDs[0])
				}
			},
		},
		{
			name: "valid multiple entries",
			json: `{
				"action": "upload_complete",
				"upload_name": "documents",
				"entry_ids": ["entry-1", "entry-2", "entry-3"]
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *UploadCompleteMessage) {
				if len(msg.EntryIDs) != 3 {
					t.Errorf("Expected 3 entries, got %d", len(msg.EntryIDs))
				}
			},
		},
		{
			name: "wrong action",
			json: `{
				"action": "upload_chunk",
				"upload_name": "avatar",
				"entry_ids": ["entry-123"]
			}`,
			wantErr: true,
			errMsg:  "expected action 'upload_complete'",
		},
		{
			name: "missing upload_name",
			json: `{
				"action": "upload_complete",
				"entry_ids": ["entry-123"]
			}`,
			wantErr: true,
			errMsg:  "upload_name is required",
		},
		{
			name: "empty entry_ids",
			json: `{
				"action": "upload_complete",
				"upload_name": "avatar",
				"entry_ids": []
			}`,
			wantErr: true,
			errMsg:  "entry_ids array is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseUploadCompleteMessage([]byte(tt.json))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func TestParseCancelUploadMessage(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *CancelUploadMessage)
	}{
		{
			name: "valid cancel",
			json: `{
				"action": "cancel_upload",
				"entry_id": "entry-123"
			}`,
			wantErr: false,
			validate: func(t *testing.T, msg *CancelUploadMessage) {
				if msg.Action != "cancel_upload" {
					t.Errorf("Expected action 'cancel_upload', got %q", msg.Action)
				}
				if msg.EntryID != "entry-123" {
					t.Errorf("Expected entry_id 'entry-123', got %q", msg.EntryID)
				}
			},
		},
		{
			name: "wrong action",
			json: `{
				"action": "upload_start",
				"entry_id": "entry-123"
			}`,
			wantErr: true,
			errMsg:  "expected action 'cancel_upload'",
		},
		{
			name: "missing entry_id",
			json: `{
				"action": "cancel_upload"
			}`,
			wantErr: true,
			errMsg:  "entry_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseCancelUploadMessage([]byte(tt.json))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, msg)
			}
		})
	}
}

func TestSerializeUploadStartResponse(t *testing.T) {
	resp := &UploadStartResponse{
		UploadName: "avatar",
		Entries: []UploadEntryInfo{
			{
				EntryID:    "entry-123",
				ClientName: "photo.jpg",
				Valid:      true,
				Error:      "",
			},
		},
	}

	data, err := SerializeUploadStartResponse(resp)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Verify it's valid JSON
	var parsed UploadStartResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized JSON: %v", err)
	}

	if parsed.UploadName != "avatar" {
		t.Errorf("Expected upload_name 'avatar', got %q", parsed.UploadName)
	}
	if len(parsed.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(parsed.Entries))
	}
}

func TestSerializeUploadProgressMessage(t *testing.T) {
	msg := &UploadProgressMessage{
		Type:       "upload_progress",
		UploadName: "avatar",
		EntryID:    "entry-123",
		ClientName: "photo.jpg",
		Progress:   50,
		BytesRecv:  512,
		BytesTotal: 1024,
	}

	data, err := SerializeUploadProgressMessage(msg)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Verify it's valid JSON
	var parsed UploadProgressMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized JSON: %v", err)
	}

	if parsed.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", parsed.Progress)
	}
	if parsed.BytesRecv != 512 {
		t.Errorf("Expected bytes_recv 512, got %d", parsed.BytesRecv)
	}
}

func TestSerializeUploadCompleteResponse(t *testing.T) {
	resp := &UploadCompleteResponse{
		UploadName: "avatar",
		Success:    true,
		Error:      "",
	}

	data, err := SerializeUploadCompleteResponse(resp)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Verify it's valid JSON
	var parsed UploadCompleteResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized JSON: %v", err)
	}

	if !parsed.Success {
		t.Error("Expected success true")
	}
}

func TestSerializeCancelUploadResponse(t *testing.T) {
	resp := &CancelUploadResponse{
		EntryID: "entry-123",
		Success: true,
	}

	data, err := SerializeCancelUploadResponse(resp)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Verify it's valid JSON
	var parsed CancelUploadResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse serialized JSON: %v", err)
	}

	if parsed.EntryID != "entry-123" {
		t.Errorf("Expected entry_id 'entry-123', got %q", parsed.EntryID)
	}
	if !parsed.Success {
		t.Error("Expected success true")
	}
}
