package upload

import (
	"fmt"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// Registry tracks active uploads for a single connection.
// Each connection has its own registry to isolate upload state.
type Registry struct {
	uploads map[string]*Upload // upload name → Upload
	mu      sync.RWMutex
}

// Upload tracks entries for a specific upload field.
type Upload struct {
	Name    string                              // Field name (e.g., "avatar")
	Config  uploadtypes.UploadConfig            // Upload configuration
	Entries map[string]*uploadtypes.UploadEntry // entry ID → entry
	mu      sync.RWMutex
}

// NewRegistry creates a new upload registry.
func NewRegistry() *Registry {
	return &Registry{
		uploads: make(map[string]*Upload),
	}
}

// CreateUpload creates a new upload field with the given configuration.
// Returns error if upload with this name already exists.
func (r *Registry) CreateUpload(name string, config uploadtypes.UploadConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.uploads[name]; exists {
		return fmt.Errorf("upload %q already exists", name)
	}

	r.uploads[name] = &Upload{
		Name:    name,
		Config:  config,
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	return nil
}

// GetUpload returns the upload for a given name.
// Returns nil if upload doesn't exist.
// Returns interface{} to satisfy the uploadRegistry interface in main package.
func (r *Registry) GetUpload(name string) interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	upload := r.uploads[name]
	if upload == nil {
		return nil
	}
	return upload
}

// GetAllUploads returns all uploads.
func (r *Registry) GetAllUploads() map[string]*Upload {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Upload, len(r.uploads))
	for name, upload := range r.uploads {
		result[name] = upload
	}
	return result
}

// DeleteUpload removes an upload and all its entries.
func (r *Registry) DeleteUpload(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.uploads, name)
}

// AddEntry adds an upload entry to the specified upload.
// Returns error if entry validation fails.
// Invalid entries are marked but still added for error display purposes.
func (u *Upload) AddEntry(entry *uploadtypes.UploadEntry) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	entry.CreatedAt = time.Now()

	// Validate count limit
	if err := ValidateCount(len(u.Entries)+1, u.Config); err != nil {
		entry.Valid = false
		entry.Error = err.Error()
		u.Entries[entry.ID] = entry
		return err
	}

	// Validate entry
	if err := ValidateEntry(entry, u.Config); err != nil {
		entry.Valid = false
		entry.Error = err.Error()
		u.Entries[entry.ID] = entry
		return err
	}

	entry.Valid = true
	u.Entries[entry.ID] = entry
	return nil
}

// GetEntry returns an entry by ID.
// Returns nil if entry doesn't exist.
func (u *Upload) GetEntry(entryID string) *uploadtypes.UploadEntry {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Entries[entryID]
}

// GetEntries returns all entries for this upload.
func (u *Upload) GetEntries() []*uploadtypes.UploadEntry {
	u.mu.RLock()
	defer u.mu.RUnlock()

	entries := make([]*uploadtypes.UploadEntry, 0, len(u.Entries))
	for _, entry := range u.Entries {
		entries = append(entries, entry)
	}
	return entries
}

// GetValidEntries returns only valid entries.
func (u *Upload) GetValidEntries() []*uploadtypes.UploadEntry {
	u.mu.RLock()
	defer u.mu.RUnlock()

	entries := make([]*uploadtypes.UploadEntry, 0, len(u.Entries))
	for _, entry := range u.Entries {
		if entry.Valid {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetCompletedEntries returns only completed valid entries.
func (u *Upload) GetCompletedEntries() []*uploadtypes.UploadEntry {
	u.mu.RLock()
	defer u.mu.RUnlock()

	entries := make([]*uploadtypes.UploadEntry, 0, len(u.Entries))
	for _, entry := range u.Entries {
		if entry.Valid && entry.Done {
			entries = append(entries, entry)
		}
	}
	return entries
}

// UpdateEntry updates an existing entry.
// Returns error if entry doesn't exist.
func (u *Upload) UpdateEntry(entryID string, updateFn func(*uploadtypes.UploadEntry)) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	entry, exists := u.Entries[entryID]
	if !exists {
		return fmt.Errorf("entry %q not found", entryID)
	}

	updateFn(entry)
	return nil
}

// RemoveEntry removes an entry by ID.
func (u *Upload) RemoveEntry(entryID string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.Entries, entryID)
}

// ClearEntries removes all entries.
func (u *Upload) ClearEntries() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Entries = make(map[string]*uploadtypes.UploadEntry)
}

// HasError returns true if any entry has an error.
func (u *Upload) HasError() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, entry := range u.Entries {
		if entry.Error != "" {
			return true
		}
	}
	return false
}

// GetError returns the first error message found, or empty string.
func (u *Upload) GetError() string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, entry := range u.Entries {
		if entry.Error != "" {
			return entry.Error
		}
	}
	return ""
}

// HasUploads checks if there are any uploads for the given field name.
// Implements UploadAccessor interface for use with Context.
func (r *Registry) HasUploads(name string) bool {
	upload := r.GetUpload(name)
	if upload == nil {
		return false
	}
	u := upload.(*Upload)
	return len(u.GetEntries()) > 0
}

// GetCompletedUploads returns all completed upload entries for the given field name.
// Implements UploadAccessor interface for use with Context.
func (r *Registry) GetCompletedUploads(name string) []*uploadtypes.UploadEntry {
	upload := r.GetUpload(name)
	if upload == nil {
		return nil
	}
	u := upload.(*Upload)

	var completed []*uploadtypes.UploadEntry
	for _, entry := range u.GetEntries() {
		if entry.Done && entry.Valid && entry.Error == "" {
			completed = append(completed, entry)
		}
	}
	return completed
}
