package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TempFileManager manages temporary upload files with automatic cleanup.
// Files are organized by session/upload/entry for easy tracking and cleanup.
type TempFileManager struct {
	baseDir string
	mu      sync.RWMutex
	files   map[string]*tempFileInfo // entry ID → file info
}

type tempFileInfo struct {
	path      string
	createdAt time.Time
	sessionID string
}

// NewTempFileManager creates a new temp file manager.
// baseDir defaults to ./.uploads/.tmp if empty (relative to current working directory).
// This avoids cross-filesystem rename issues when moving uploaded files.
func NewTempFileManager(baseDir string) (*TempFileManager, error) {
	if baseDir == "" {
		baseDir = filepath.Join(".", ".uploads", ".tmp")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &TempFileManager{
		baseDir: baseDir,
		files:   make(map[string]*tempFileInfo),
	}, nil
}

// CreateTempFile creates a temporary file for an upload entry.
// Path format: {baseDir}/{sessionID}/{uploadName}/{entryID}
func (m *TempFileManager) CreateTempFile(sessionID, uploadName, entryID string) (string, error) {
	if sessionID == "" || uploadName == "" || entryID == "" {
		return "", fmt.Errorf("sessionID, uploadName, and entryID are required")
	}

	// Create directory structure
	dir := filepath.Join(m.baseDir, sessionID, uploadName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Create file path
	path := filepath.Join(dir, entryID)

	// Create the file
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file after creation: %w", err)
	}

	// Track the file
	m.mu.Lock()
	m.files[entryID] = &tempFileInfo{
		path:      path,
		createdAt: time.Now(),
		sessionID: sessionID,
	}
	m.mu.Unlock()

	return path, nil
}

// GetFilePath returns the path for a temp file by entry ID.
// Returns empty string if file doesn't exist.
func (m *TempFileManager) GetFilePath(entryID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.files[entryID]
	if !exists {
		return ""
	}
	return info.path
}

// OpenForWriting opens a temp file for writing.
func (m *TempFileManager) OpenForWriting(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

// RemoveFile removes a temp file by entry ID.
func (m *TempFileManager) RemoveFile(entryID string) error {
	m.mu.Lock()
	info, exists := m.files[entryID]
	if !exists {
		m.mu.Unlock()
		return nil // Already removed
	}
	delete(m.files, entryID)
	m.mu.Unlock()

	// Remove the file
	if err := os.Remove(info.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}

// RemoveSession removes all temp files for a session.
func (m *TempFileManager) RemoveSession(sessionID string) error {
	m.mu.Lock()
	// Find all files for this session
	var toRemove []string
	for entryID, info := range m.files {
		if info.sessionID == sessionID {
			toRemove = append(toRemove, entryID)
		}
	}
	// Remove from tracking
	for _, entryID := range toRemove {
		delete(m.files, entryID)
	}
	m.mu.Unlock()

	// Remove session directory
	sessionDir := filepath.Join(m.baseDir, sessionID)
	if err := os.RemoveAll(sessionDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session directory: %w", err)
	}

	return nil
}

// CleanupStale removes temp files older than the specified TTL.
// Returns the number of files removed.
func (m *TempFileManager) CleanupStale(ttl time.Duration) (int, error) {
	m.mu.Lock()
	now := time.Now()
	var toRemove []*tempFileInfo

	// Find stale files and collect their info
	for entryID, info := range m.files {
		if now.Sub(info.createdAt) > ttl {
			toRemove = append(toRemove, info)
			delete(m.files, entryID)
		}
	}
	m.mu.Unlock()

	// Remove the files
	var removed int
	for _, info := range toRemove {
		if err := os.Remove(info.path); err != nil && !os.IsNotExist(err) {
			// Log error but continue cleanup
			continue
		}
		removed++
	}

	return removed, nil
}

// Count returns the number of tracked temp files.
func (m *TempFileManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.files)
}

// GenerateEntryID generates a unique entry ID for an upload.
// Returns an error if random generation fails (extremely rare).
func GenerateEntryID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate entry ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}
