package upload

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

func TestParseMultipartUpload_Success(t *testing.T) {
	// Create temp file manager
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form with file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("avatar", "test.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}

	testContent := []byte("fake image content")
	if _, err := part.Write(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Create request
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Parse upload
	config := uploadtypes.UploadConfig{
		Accept:      []string{"image/*"},
		MaxFileSize: 1024 * 1024,
		MaxEntries:  1,
	}

	entries, err := ParseMultipartUpload(req, "avatar", config, "session-123", tempMgr)
	if err != nil {
		t.Fatalf("ParseMultipartUpload failed: %v", err)
	}

	// Verify results
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ClientName != "test.jpg" {
		t.Errorf("Expected filename 'test.jpg', got %q", entry.ClientName)
	}

	if !entry.Valid {
		t.Errorf("Expected entry to be valid, got error: %s", entry.Error)
	}

	if !entry.Done {
		t.Error("Expected entry to be marked as done")
	}

	if entry.Progress != 100 {
		t.Errorf("Expected progress 100, got %d", entry.Progress)
	}

	if entry.TempPath == "" {
		t.Error("Expected temp path to be set")
	}

	// Verify file was written
	data, err := os.ReadFile(entry.TempPath)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	if !bytes.Equal(data, testContent) {
		t.Errorf("File content mismatch")
	}
}

func TestParseMultipartUpload_ValidationError(t *testing.T) {
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form with invalid file type
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("avatar", "test.txt")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte("text content")); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Config only allows images
	config := uploadtypes.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 1,
	}

	entries, err := ParseMultipartUpload(req, "avatar", config, "session-123", tempMgr)
	if err != nil {
		t.Fatalf("ParseMultipartUpload should not error on validation failure: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Valid {
		t.Error("Expected entry to be invalid")
	}

	if entry.Error == "" {
		t.Error("Expected error message to be set")
	}

	if entry.TempPath != "" {
		t.Error("Expected no temp path for invalid entry")
	}
}

func TestParseMultipartUpload_FileTooLarge(t *testing.T) {
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form with large file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("avatar", "large.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	// Write more than the limit
	largeContent := make([]byte, 2*1024)
	if _, err := part.Write(largeContent); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Config with small size limit
	config := uploadtypes.UploadConfig{
		Accept:      []string{"image/*"},
		MaxFileSize: 1024, // 1KB limit
		MaxEntries:  1,
	}

	entries, err := ParseMultipartUpload(req, "avatar", config, "session-123", tempMgr)
	if err != nil {
		t.Fatalf("ParseMultipartUpload failed: %v", err)
	}

	entry := entries[0]
	if entry.Valid {
		t.Error("Expected entry to be invalid due to size")
	}

	if entry.Error == "" {
		t.Error("Expected error message about file size")
	}
}

func TestParseMultipartUpload_TooManyFiles(t *testing.T) {
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form with multiple files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for range 3 {
		part, err := writer.CreateFormFile("avatar", "test.jpg")
		if err != nil {
			t.Fatalf("CreateFormFile failed: %v", err)
		}
		if _, err := part.Write([]byte("content")); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Config allows only 2 files
	config := uploadtypes.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 2,
	}

	_, err = ParseMultipartUpload(req, "avatar", config, "session-123", tempMgr)
	if err == nil {
		t.Fatal("Expected error for too many files")
	}
}

func TestParseMultipartUpload_NoFiles(t *testing.T) {
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form without files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	config := uploadtypes.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 1,
	}

	_, err = ParseMultipartUpload(req, "avatar", config, "session-123", tempMgr)
	if err == nil {
		t.Fatal("Expected error for no files")
	}
}

func TestParseMultipartUpload_MultipleFiles(t *testing.T) {
	tempMgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempMgr.baseDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create multipart form with multiple valid files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for i := 1; i <= 3; i++ {
		part, err := writer.CreateFormFile("documents", "doc"+string(rune('0'+i))+".pdf")
		if err != nil {
			t.Fatalf("CreateFormFile failed: %v", err)
		}
		if _, err := part.Write([]byte("pdf content " + string(rune('0'+i)))); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	config := uploadtypes.UploadConfig{
		Accept:      []string{".pdf"},
		MaxFileSize: 1024 * 1024,
		MaxEntries:  5,
	}

	entries, err := ParseMultipartUpload(req, "documents", config, "session-123", tempMgr)
	if err != nil {
		t.Fatalf("ParseMultipartUpload failed: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Verify all entries are valid
	for i, entry := range entries {
		if !entry.Valid {
			t.Errorf("Entry %d should be valid, got error: %s", i, entry.Error)
		}
		if !entry.Done {
			t.Errorf("Entry %d should be done", i)
		}
		if entry.TempPath == "" {
			t.Errorf("Entry %d missing temp path", i)
		}
	}
}
