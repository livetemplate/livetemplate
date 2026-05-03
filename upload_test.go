package livetemplate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
		// Real presigners reject malformed inputs — checking ClientType here
		// also doubles as documentation of the error-return contract.
		if entry.ClientType == "" {
			return UploadMeta{}, errors.New("entry missing client content type")
		}
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

func TestHandle_TempFileManagerInitialized_WithUploadConfig(t *testing.T) {
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
	handler := tmpl.Handle(ctrl, state)

	// With upload config, tempFileManager should be initialized
	lh := handler.(*liveHandler)
	if lh.tempFileManager == nil {
		t.Error("tempFileManager should not be nil with upload config")
	}
}

func TestHandleUploadAction_NilTempFileManager(t *testing.T) {
	handler := &liveHandler{
		tempFileManager: nil,
	}

	actions := []string{"upload_start", "upload_chunk", "upload_complete", "cancel_upload"}
	for _, action := range actions {
		msg := message{Action: action}
		handled, err := handler.handleUploadAction(context.Background(), nil, msg, nil, nil, nil)
		if !handled {
			t.Errorf("action %q: expected handled=true, got false", action)
		}
		if err == nil {
			t.Errorf("action %q: expected error for nil tempFileManager, got nil", action)
		}
	}

	// Non-upload actions should return handled=false
	msg := message{Action: "some_other_action"}
	handled, err := handler.handleUploadAction(context.Background(), nil, msg, nil, nil, nil)
	if handled {
		t.Error("non-upload action: expected handled=false, got true")
	}
	if err != nil {
		t.Errorf("non-upload action: expected no error, got %v", err)
	}
}

// --- Tier 1 HTTP Multipart Upload Tests ---

type multipartUploadState struct {
	AvatarPath string `lvt:"persist"`
}

type multipartUploadController struct {
	lastUploads []*UploadEntry
}

func (c *multipartUploadController) UpdateProfile(state multipartUploadState, ctx *Context) (multipartUploadState, error) {
	uploads := ctx.GetCompletedUploads("avatar")
	c.lastUploads = uploads
	if len(uploads) > 0 {
		state.AvatarPath = uploads[0].TempPath
	}
	return state, nil
}

func TestHTTPMultipartUpload_FilesAvailableInAction(t *testing.T) {
	tmpl, err := New("test", WithUpload("avatar", UploadConfig{
		Accept:      []string{"image/png"},
		MaxFileSize: 5 * 1024 * 1024,
		MaxEntries:  1,
	}))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.AvatarPath}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &multipartUploadController{}
	handler := tmpl.Handle(ctrl, AsState(&multipartUploadState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	// Clean up temp uploads after test
	defer func() {
		if err := os.RemoveAll(".uploads"); err != nil {
			t.Logf("cleanup .uploads failed: %v", err)
		}
	}()

	// GET first to establish session
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("body close failed: %v", err)
	}
	cookies := resp.Cookies()

	// Build multipart form with a file
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add action routing via button name (simulates <button name="updateProfile">)
	if err := writer.WriteField("updateProfile", ""); err != nil {
		t.Fatalf("WriteField failed: %v", err)
	}
	// Also add lvt-action for multipart parser (which uses this field for routing)
	if err := writer.WriteField("lvt-action", "updateProfile"); err != nil {
		t.Fatalf("WriteField lvt-action failed: %v", err)
	}

	// Add file
	part, err := writer.CreateFormFile("avatar", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	// Write a minimal PNG header (8 bytes) as test content
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if _, err := part.Write(pngHeader); err != nil {
		t.Fatalf("Write file content failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close failed: %v", err)
	}

	// POST multipart form
	req, err := http.NewRequest("POST", server.URL+"/", &body)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("body close failed: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	// Verify the controller received the upload entries
	if len(ctrl.lastUploads) == 0 {
		t.Fatal("Expected upload entries in action handler, got none")
	}

	entry := ctrl.lastUploads[0]
	if entry.ClientName != "test.png" {
		t.Errorf("Expected ClientName 'test.png', got %q", entry.ClientName)
	}
	if !entry.Done {
		t.Error("Expected entry.Done=true")
	}
	if !entry.Valid {
		t.Errorf("Expected entry.Valid=true, error: %s", entry.Error)
	}
	if entry.TempPath == "" {
		t.Error("Expected non-empty TempPath")
	}
	// Verify temp file exists
	if _, err := os.Stat(entry.TempPath); os.IsNotExist(err) {
		t.Errorf("Temp file does not exist: %s", entry.TempPath)
	}
}

func TestHTTPMultipartUpload_NoUploadConfig_NoFiles(t *testing.T) {
	// Without WithUpload(), multipart files should be ignored
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>ok</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	// GET to establish session
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("body close failed: %v", err)
	}
	cookies := resp.Cookies()

	// POST multipart with file but no upload config
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte("fake png data")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close failed: %v", err)
	}

	req, _ := http.NewRequest("POST", server.URL+"/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "text/html")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close failed: %v", err)
	}

	// Should not crash — just process as a normal form submission
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 303 or 200, got %d", resp.StatusCode)
	}
}

func TestProgressReader(t *testing.T) {
	data := strings.Repeat("x", 100)
	reader := strings.NewReader(data)

	var progressCalls []int
	pr := newTestProgressReader(io.NopCloser(reader), int64(len(data)), func(bytesRead, total int64) {
		pct := int(bytesRead * 100 / total)
		progressCalls = append(progressCalls, pct)
	})

	buf := make([]byte, 10)
	for {
		_, err := pr.Read(buf)
		if err != nil {
			break
		}
	}

	// Should have received progress callbacks at each 10% increment
	if len(progressCalls) == 0 {
		t.Fatal("Expected progress callbacks, got none")
	}
	// Last callback should be 100%
	if progressCalls[len(progressCalls)-1] != 100 {
		t.Errorf("Expected last progress to be 100%%, got %d%%", progressCalls[len(progressCalls)-1])
	}
}

// newTestProgressReader creates a progress reader for testing (avoids import cycle).
func newTestProgressReader(reader io.ReadCloser, total int64, onProgress func(int64, int64)) io.Reader {
	return &testProgressReader{reader: reader, total: total, onProgress: onProgress}
}

type testProgressReader struct {
	reader     io.ReadCloser
	bytesRead  int64
	total      int64
	onProgress func(bytesRead, total int64)
	lastPct    int
}

func (p *testProgressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.bytesRead += int64(n)
	if p.total > 0 && p.onProgress != nil {
		pct := int(p.bytesRead * 100 / p.total)
		if pct > p.lastPct {
			p.lastPct = pct
			p.onProgress(p.bytesRead, p.total)
		}
	}
	return n, err
}
