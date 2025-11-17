package upload

import (
	uploadtypes "github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// RegistryAccessor adapts Registry to provide upload access for ActionContext
type RegistryAccessor struct {
	registry *Registry
}

// NewAccessor creates a new RegistryAccessor
func NewAccessor(registry *Registry) *RegistryAccessor {
	return &RegistryAccessor{registry: registry}
}

// HasUploads checks if there are any uploads for the given field name
func (a *RegistryAccessor) HasUploads(name string) bool {
	upload := a.registry.GetUpload(name)
	if upload == nil {
		return false
	}
	u := upload.(*Upload)
	return len(u.GetEntries()) > 0
}

// GetUploads returns all upload entries for the given field name
func (a *RegistryAccessor) GetUploads(name string) []*uploadtypes.UploadEntry {
	upload := a.registry.GetUpload(name)
	if upload == nil {
		return nil
	}
	u := upload.(*Upload)
	return u.GetEntries()
}

// GetUpload returns a specific upload entry by field name and entry ID
func (a *RegistryAccessor) GetUpload(name string, entryID string) *uploadtypes.UploadEntry {
	upload := a.registry.GetUpload(name)
	if upload == nil {
		return nil
	}
	u := upload.(*Upload)
	return u.GetEntry(entryID)
}

// GetValidUploads returns all valid (non-error) upload entries for the given field name
func (a *RegistryAccessor) GetValidUploads(name string) []*uploadtypes.UploadEntry {
	upload := a.registry.GetUpload(name)
	if upload == nil {
		return nil
	}
	u := upload.(*Upload)

	var valid []*uploadtypes.UploadEntry
	for _, entry := range u.GetEntries() {
		if entry.Valid && entry.Error == "" {
			valid = append(valid, entry)
		}
	}
	return valid
}

// GetCompletedUploads returns all completed upload entries for the given field name
func (a *RegistryAccessor) GetCompletedUploads(name string) []*uploadtypes.UploadEntry {
	upload := a.registry.GetUpload(name)
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
