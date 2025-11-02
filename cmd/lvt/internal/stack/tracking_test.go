package stack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackingFile_WriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	trackingPath := filepath.Join(tmpDir, ".lvtstack")

	tracking := &TrackingFile{
		Version:          1,
		Provider:         "docker",
		GeneratedAt:      time.Now(),
		GeneratorVersion: "0.1.0",
		Configuration: TrackingConfig{
			Database: "sqlite",
			Backup:   "litestream",
			Redis:    "none",
			Storage:  "s3",
			CI:       "github",
		},
		Files: []TrackedFile{
			{Path: "deploy/docker/docker-compose.yml", Checksum: "abc123", Modified: false},
			{Path: "deploy/docker/Dockerfile", Checksum: "def456", Modified: false},
		},
	}

	// Write
	err := tracking.Write(trackingPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read
	read, err := ReadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("ReadTrackingFile() error = %v", err)
	}

	if read.Provider != tracking.Provider {
		t.Errorf("Provider = %v, want %v", read.Provider, tracking.Provider)
	}
	if len(read.Files) != len(tracking.Files) {
		t.Errorf("Files count = %v, want %v", len(read.Files), len(tracking.Files))
	}
}

func TestTrackingFile_CheckModifications(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("original content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate checksum
	checksum, err := calculateChecksum(testFile)
	if err != nil {
		t.Fatal(err)
	}

	tracking := &TrackingFile{
		Files: []TrackedFile{
			{Path: "test.txt", Checksum: checksum, Modified: false},
		},
	}

	// Check modifications - should be false
	modified, err := tracking.CheckModifications(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Errorf("Expected no modifications, got %v", modified)
	}

	// Modify file
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Check again - should detect modification
	modified, err = tracking.CheckModifications(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 {
		t.Errorf("Expected 1 modification, got %v", len(modified))
	}
}
