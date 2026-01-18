package upload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTempFileManager(t *testing.T) {
	// Test with custom base dir
	baseDir := filepath.Join(os.TempDir(), "test-uploads")
	defer func() {
		if err := os.RemoveAll(baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	mgr, err := NewTempFileManager(baseDir)
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		t.Error("Expected base directory to be created")
	}

	if mgr.baseDir != baseDir {
		t.Errorf("Expected baseDir %q, got %q", baseDir, mgr.baseDir)
	}
}

func TestCreateTempFile(t *testing.T) {
	baseDir := filepath.Join(os.TempDir(), "test-uploads-create")
	defer func() {
		if err := os.RemoveAll(baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	mgr, err := NewTempFileManager(baseDir)
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}

	// Create temp file
	path, err := mgr.CreateTempFile("session-123", "avatar", "entry-1")
	if err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Expected temp file to exist")
	}

	// Verify path structure
	expectedPath := filepath.Join(baseDir, "session-123", "avatar", "entry-1")
	if path != expectedPath {
		t.Errorf("Expected path %q, got %q", expectedPath, path)
	}

	// Verify tracking
	if mgr.Count() != 1 {
		t.Errorf("Expected 1 tracked file, got %d", mgr.Count())
	}
}

func TestCreateTempFile_ValidationErrors(t *testing.T) {
	mgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(mgr.baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	tests := []struct {
		name       string
		sessionID  string
		uploadName string
		entryID    string
	}{
		{"empty sessionID", "", "avatar", "entry-1"},
		{"empty uploadName", "session-123", "", "entry-1"},
		{"empty entryID", "session-123", "avatar", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mgr.CreateTempFile(tt.sessionID, tt.uploadName, tt.entryID)
			if err == nil {
				t.Error("Expected validation error")
			}
		})
	}
}

func TestGetFilePath(t *testing.T) {
	mgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(mgr.baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	// Create a file
	path, err := mgr.CreateTempFile("session-123", "avatar", "entry-1")
	if err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	// Get the path
	retrievedPath := mgr.GetFilePath("entry-1")
	if retrievedPath != path {
		t.Errorf("Expected path %q, got %q", path, retrievedPath)
	}

	// Non-existent entry should return empty string
	retrievedPath = mgr.GetFilePath("nonexistent")
	if retrievedPath != "" {
		t.Errorf("Expected empty path for nonexistent entry, got %q", retrievedPath)
	}
}

func TestRemoveFile(t *testing.T) {
	mgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(mgr.baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	// Create a file
	path, err := mgr.CreateTempFile("session-123", "avatar", "entry-1")
	if err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	// Remove the file
	if err := mgr.RemoveFile("entry-1"); err != nil {
		t.Fatalf("RemoveFile failed: %v", err)
	}

	// Verify file is removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Expected file to be removed")
	}

	// Verify tracking
	if mgr.Count() != 0 {
		t.Errorf("Expected 0 tracked files, got %d", mgr.Count())
	}

	// Removing again should be idempotent
	if err := mgr.RemoveFile("entry-1"); err != nil {
		t.Errorf("Second RemoveFile should not error: %v", err)
	}
}

func TestRemoveSession(t *testing.T) {
	mgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(mgr.baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	// Create multiple files for same session
	if _, err := mgr.CreateTempFile("session-123", "avatar", "entry-1"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}
	if _, err := mgr.CreateTempFile("session-123", "avatar", "entry-2"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}
	if _, err := mgr.CreateTempFile("session-123", "documents", "entry-3"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	// Create file for different session
	if _, err := mgr.CreateTempFile("session-456", "avatar", "entry-4"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	if mgr.Count() != 4 {
		t.Fatalf("Expected 4 tracked files, got %d", mgr.Count())
	}

	// Remove session-123
	if err := mgr.RemoveSession("session-123"); err != nil {
		t.Fatalf("RemoveSession failed: %v", err)
	}

	// Verify only session-456 file remains
	if mgr.Count() != 1 {
		t.Errorf("Expected 1 tracked file, got %d", mgr.Count())
	}

	// Verify session directory is removed
	sessionDir := filepath.Join(mgr.baseDir, "session-123")
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("Expected session directory to be removed")
	}

	// Verify session-456 still exists
	path := mgr.GetFilePath("entry-4")
	if path == "" {
		t.Error("Expected session-456 file to still exist")
	}
}

func TestCleanupStale(t *testing.T) {
	mgr, err := NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager failed: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(mgr.baseDir); err != nil {
			t.Errorf("Failed to remove test dir: %v", err)
		}
	}()

	// Create files
	if _, err := mgr.CreateTempFile("session-123", "avatar", "entry-1"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}
	if _, err := mgr.CreateTempFile("session-123", "avatar", "entry-2"); err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}

	// Manually set creation time to be old
	mgr.mu.Lock()
	mgr.files["entry-1"].createdAt = time.Now().Add(-2 * time.Hour)
	mgr.mu.Unlock()

	// Cleanup stale files (older than 1 hour)
	removed, err := mgr.CleanupStale(1 * time.Hour)
	if err != nil {
		t.Fatalf("CleanupStale failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("Expected 1 file removed, got %d", removed)
	}

	// Verify only entry-2 remains
	if mgr.Count() != 1 {
		t.Errorf("Expected 1 tracked file, got %d", mgr.Count())
	}

	if mgr.GetFilePath("entry-1") != "" {
		t.Error("Expected entry-1 to be removed")
	}

	if mgr.GetFilePath("entry-2") == "" {
		t.Error("Expected entry-2 to remain")
	}
}

func TestGenerateEntryID(t *testing.T) {
	// Generate multiple IDs
	id1 := GenerateEntryID()
	id2 := GenerateEntryID()

	// Verify IDs are non-empty
	if id1 == "" || id2 == "" {
		t.Error("Expected non-empty entry IDs")
	}

	// Verify IDs are unique
	if id1 == id2 {
		t.Error("Expected unique entry IDs")
	}

	// Verify length (32 hex chars = 16 bytes)
	if len(id1) != 32 {
		t.Errorf("Expected ID length 32, got %d", len(id1))
	}
}
