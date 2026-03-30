package livetemplate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestUploadConfig_Defaults verifies default behavior of UploadConfig zero values.
func TestUploadConfig_Defaults(t *testing.T) {
	config := UploadConfig{}

	if config.MaxEntries != 0 {
		t.Errorf("Expected MaxEntries default to be 0, got %d", config.MaxEntries)
	}

	if config.MaxFileSize != 0 {
		t.Errorf("Expected MaxFileSize default to be 0, got %d", config.MaxFileSize)
	}

	if config.AutoUpload {
		t.Error("Expected AutoUpload default to be false")
	}

	if config.ChunkSize != 0 {
		t.Errorf("Expected ChunkSize default to be 0, got %d", config.ChunkSize)
	}
}

// TestUploadConfig_ValidConfiguration verifies a valid upload configuration.
func TestUploadConfig_ValidConfiguration(t *testing.T) {
	config := UploadConfig{
		Accept:      []string{"image/jpeg", "image/png", ".pdf"},
		MaxEntries:  3,
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		AutoUpload:  true,
		ChunkSize:   256 * 1024, // 256KB
	}

	if len(config.Accept) != 3 {
		t.Errorf("Expected 3 accepted types, got %d", len(config.Accept))
	}

	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("Expected MaxFileSize 10MB, got %d", config.MaxFileSize)
	}
}

// TestUploadEntry_StateTransitions verifies upload entry state management.
func TestUploadEntry_StateTransitions(t *testing.T) {
	tests := []struct {
		name  string
		entry UploadEntry
		want  string
	}{
		{
			name: "new entry",
			entry: UploadEntry{
				ID:         "entry-123",
				ClientName: "photo.jpg",
				Valid:      true,
				Done:       false,
				Progress:   0,
			},
			want: "pending",
		},
		{
			name: "uploading entry",
			entry: UploadEntry{
				ID:       "entry-123",
				Valid:    true,
				Done:     false,
				Progress: 50,
			},
			want: "uploading",
		},
		{
			name: "completed entry",
			entry: UploadEntry{
				ID:       "entry-123",
				Valid:    true,
				Done:     true,
				Progress: 100,
			},
			want: "done",
		},
		{
			name: "failed entry",
			entry: UploadEntry{
				ID:    "entry-123",
				Valid: false,
				Error: "file too large",
			},
			want: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var state string
			switch {
			case !tt.entry.Valid:
				state = "failed"
			case tt.entry.Done:
				state = "done"
			case tt.entry.Progress > 0:
				state = "uploading"
			default:
				state = "pending"
			}

			if state != tt.want {
				t.Errorf("Expected state %s, got %s", tt.want, state)
			}
		})
	}
}

// TestUploadEntry_ExternalVsServerUpload verifies external vs server upload fields.
func TestUploadEntry_ExternalVsServerUpload(t *testing.T) {
	t.Run("server upload", func(t *testing.T) {
		entry := UploadEntry{
			ID:       "entry-123",
			TempPath: "/tmp/upload-123/file.jpg",
		}

		if entry.TempPath == "" {
			t.Error("Expected TempPath to be set for server upload")
		}

		if entry.ExternalRef != "" {
			t.Error("Expected ExternalRef to be empty for server upload")
		}
	})

	t.Run("external upload", func(t *testing.T) {
		entry := UploadEntry{
			ID:          "entry-456",
			ExternalRef: "s3://bucket/uploads/entry-456/file.jpg",
		}

		if entry.ExternalRef == "" {
			t.Error("Expected ExternalRef to be set for external upload")
		}

		if entry.TempPath != "" {
			t.Error("Expected TempPath to be empty for external upload")
		}
	})
}

// TestUploadEntry_Timestamps verifies timestamp handling.
func TestUploadEntry_Timestamps(t *testing.T) {
	now := time.Now()
	entry := UploadEntry{
		ID:        "entry-123",
		CreatedAt: now,
	}

	if entry.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if !entry.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be zero for incomplete upload")
	}

	// Simulate completion
	entry.Done = true
	entry.CompletedAt = time.Now()

	if entry.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set after completion")
	}

	if entry.CompletedAt.Before(entry.CreatedAt) {
		t.Error("Expected CompletedAt to be after CreatedAt")
	}
}

// mockPresigner implements Presigner for testing.
type mockPresigner struct {
	shouldError bool
	meta        UploadMeta
}

func (m *mockPresigner) Presign(entry *UploadEntry) (UploadMeta, error) {
	if m.shouldError {
		return UploadMeta{}, errors.New("presigning failed")
	}
	return m.meta, nil
}

// TestPresigner_Interface verifies Presigner interface implementation.
func TestPresigner_Interface(t *testing.T) {
	presigner := &mockPresigner{
		meta: UploadMeta{
			Uploader: "s3",
			URL:      "https://bucket.s3.amazonaws.com/upload",
			Headers: map[string]string{
				"Content-Type": "image/jpeg",
			},
		},
	}

	entry := &UploadEntry{
		ID:         "entry-123",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024 * 1024, // 1MB
	}

	meta, err := presigner.Presign(entry)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if meta.Uploader != "s3" {
		t.Errorf("Expected uploader 's3', got %s", meta.Uploader)
	}

	if meta.URL == "" {
		t.Error("Expected URL to be set")
	}

	if meta.Headers["Content-Type"] != "image/jpeg" {
		t.Errorf("Expected Content-Type header, got %v", meta.Headers)
	}
}

// TestPresigner_ErrorHandling verifies error handling.
func TestPresigner_ErrorHandling(t *testing.T) {
	presigner := &mockPresigner{shouldError: true}

	entry := &UploadEntry{
		ID:         "entry-123",
		ClientName: "photo.jpg",
	}

	_, err := presigner.Presign(entry)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "presigning failed" {
		t.Errorf("Expected 'presigning failed' error, got %v", err)
	}
}

// mockStore implements UploadAware for testing.
type mockStore struct {
	uploads      map[string]UploadConfig
	consumeCalls []consumeCall
	shouldError  bool
}

type consumeCall struct {
	name    string
	entries []*UploadEntry
}

func (m *mockStore) AllowUploads() map[string]UploadConfig {
	return m.uploads
}

func (m *mockStore) ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error {
	m.consumeCalls = append(m.consumeCalls, consumeCall{
		name:    name,
		entries: entries,
	})

	if m.shouldError {
		return errors.New("consume failed")
	}

	return nil
}

// TestUploadAware_Interface verifies UploadAware interface implementation.
func TestUploadAware_Interface(t *testing.T) {
	store := &mockStore{
		uploads: map[string]UploadConfig{
			"avatar": {
				Accept:      []string{"image/*"},
				MaxEntries:  1,
				MaxFileSize: 5 * 1024 * 1024, // 5MB
			},
			"documents": {
				Accept:      []string{".pdf", ".doc"},
				MaxEntries:  10,
				MaxFileSize: 20 * 1024 * 1024, // 20MB
			},
		},
	}

	configs := store.AllowUploads()

	if len(configs) != 2 {
		t.Errorf("Expected 2 upload configs, got %d", len(configs))
	}

	avatarConfig, ok := configs["avatar"]
	if !ok {
		t.Fatal("Expected 'avatar' config to exist")
	}

	if avatarConfig.MaxEntries != 1 {
		t.Errorf("Expected MaxEntries 1, got %d", avatarConfig.MaxEntries)
	}

	if avatarConfig.MaxFileSize != 5*1024*1024 {
		t.Errorf("Expected MaxFileSize 5MB, got %d", avatarConfig.MaxFileSize)
	}
}

// TestUploadAware_ConsumeUpload verifies ConsumeUpload implementation.
func TestUploadAware_ConsumeUpload(t *testing.T) {
	store := &mockStore{
		uploads: map[string]UploadConfig{
			"avatar": {Accept: []string{"image/*"}},
		},
	}

	entries := []*UploadEntry{
		{
			ID:         "entry-123",
			ClientName: "photo.jpg",
			TempPath:   "/tmp/upload/photo.jpg",
			Valid:      true,
			Done:       true,
		},
	}

	ctx := context.Background()
	err := store.ConsumeUpload(ctx, "avatar", entries)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(store.consumeCalls) != 1 {
		t.Fatalf("Expected 1 consume call, got %d", len(store.consumeCalls))
	}

	call := store.consumeCalls[0]
	if call.name != "avatar" {
		t.Errorf("Expected name 'avatar', got %s", call.name)
	}

	if len(call.entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(call.entries))
	}

	if call.entries[0].ID != "entry-123" {
		t.Errorf("Expected entry ID 'entry-123', got %s", call.entries[0].ID)
	}
}

// TestUploadAware_ConsumeUpload_Error verifies error handling.
func TestUploadAware_ConsumeUpload_Error(t *testing.T) {
	store := &mockStore{
		uploads:     map[string]UploadConfig{"avatar": {}},
		shouldError: true,
	}

	entries := []*UploadEntry{
		{ID: "entry-123", Valid: true, Done: true},
	}

	err := store.ConsumeUpload(context.Background(), "avatar", entries)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "consume failed" {
		t.Errorf("Expected 'consume failed' error, got %v", err)
	}
}

// TestUploadConfig_WithExternalPresigner verifies external presigner configuration.
func TestUploadConfig_WithExternalPresigner(t *testing.T) {
	presigner := &mockPresigner{
		meta: UploadMeta{
			Uploader: "s3",
			URL:      "https://s3.amazonaws.com/bucket/key",
		},
	}

	config := UploadConfig{
		Accept:   []string{"image/*"},
		External: presigner,
	}

	if config.External == nil {
		t.Fatal("Expected External presigner to be set")
	}

	// Test presigner functionality
	entry := &UploadEntry{
		ID:         "entry-123",
		ClientName: "photo.jpg",
	}

	meta, err := config.External.Presign(entry)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if meta.Uploader != "s3" {
		t.Errorf("Expected uploader 's3', got %s", meta.Uploader)
	}
}

// ExampleUploadAware demonstrates implementing the UploadAware interface.
func ExampleUploadAware() {
	type ProfileStore struct {
		avatarPath string
	}

	// Implement AllowUploads to configure accepted uploads
	_ = func(s *ProfileStore) map[string]UploadConfig {
		return map[string]UploadConfig{
			"avatar": {
				Accept:      []string{"image/jpeg", "image/png"},
				MaxFileSize: 5 * 1024 * 1024, // 5MB
				MaxEntries:  1,
				AutoUpload:  true,
			},
		}
	}

	// Implement ConsumeUpload to process completed uploads
	_ = func(s *ProfileStore) error {
		ctx := context.Background()
		name := "avatar"
		entries := []*UploadEntry{
			{
				ID:         "entry-123",
				ClientName: "profile.jpg",
				TempPath:   "/tmp/uploads/entry-123/profile.jpg",
				Valid:      true,
				Done:       true,
			},
		}

		// Process the upload
		for _, entry := range entries {
			s.avatarPath = entry.TempPath
		}

		_ = ctx
		_ = name
		return nil
	}
}

// ExamplePresigner demonstrates implementing a custom presigner.
func ExamplePresigner() {
	type CustomPresigner struct {
		endpoint string
		apiKey   string
	}

	presign := func(p *CustomPresigner, entry *UploadEntry) (UploadMeta, error) {
		return UploadMeta{
			Uploader: "custom",
			URL:      p.endpoint + "/upload/" + entry.ID,
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
				"Content-Type":  entry.ClientType,
			},
		}, nil
	}

	presigner := &CustomPresigner{
		endpoint: "https://storage.example.com",
		apiKey:   "secret-key",
	}

	entry := &UploadEntry{
		ID:         "entry-123",
		ClientType: "image/jpeg",
	}

	meta, _ := presign(presigner, entry)
	_ = meta.URL // https://storage.example.com/upload/entry-123
}

func TestHandle_NilTempFileManager_WithoutUploadConfig(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{})
	handler := tmpl.Handle(ctrl, state)

	// Without upload config, tempFileManager should be nil (no .uploads dir created)
	lh := handler.(*liveHandler)
	if lh.tempFileManager != nil {
		t.Error("tempFileManager should be nil without upload config")
	}
}

func TestHandle_CreatesUploadsDir_WithUploadConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Failed to restore working directory: %v", err)
		}
	}()

	tmpl, err := New("test", WithUpload("avatar", UploadConfig{
		Accept:     []string{"image/png"},
		MaxEntries: 1,
	}))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{})
	_ = tmpl.Handle(ctrl, state)

	uploadsDir := filepath.Join(dir, ".uploads")
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		t.Errorf(".uploads directory should exist with upload config, but it doesn't")
	}
}

func TestHandleUploadAction_NilTempFileManager(t *testing.T) {
	handler := &liveHandler{
		tempFileManager: nil,
	}

	actions := []string{"upload_start", "upload_chunk", "upload_complete", "cancel_upload"}
	for _, action := range actions {
		msg := message{Action: action}
		handled, err := handler.handleUploadAction(context.Background(), nil, nil, msg, nil, nil, nil)
		if !handled {
			t.Errorf("action %q: expected handled=true, got false", action)
		}
		if err == nil {
			t.Errorf("action %q: expected error for nil tempFileManager, got nil", action)
		}
	}

	// Non-upload actions should return handled=false
	msg := message{Action: "some_other_action"}
	handled, err := handler.handleUploadAction(context.Background(), nil, nil, msg, nil, nil, nil)
	if handled {
		t.Error("non-upload action: expected handled=false, got true")
	}
	if err != nil {
		t.Errorf("non-upload action: expected no error, got %v", err)
	}
}
