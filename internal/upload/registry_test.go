package upload

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

func TestRegistry_CreateUpload(t *testing.T) {
	reg := NewRegistry()

	config := uploadtypes.UploadConfig{
		Accept:      []string{"image/*"},
		MaxEntries:  5,
		MaxFileSize: 1024 * 1024,
	}

	// First creation should succeed
	err := reg.CreateUpload("avatar", config)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Duplicate creation should fail
	err = reg.CreateUpload("avatar", config)
	if err == nil {
		t.Fatal("Expected error for duplicate upload, got nil")
	}
}

func TestRegistry_GetUpload(t *testing.T) {
	reg := NewRegistry()

	// Non-existent upload should return nil
	nonexistent := reg.GetUpload("nonexistent")
	if nonexistent != nil {
		t.Fatal("Expected nil for non-existent upload")
	}

	// Create and retrieve upload
	config := uploadtypes.UploadConfig{Accept: []string{"image/*"}}
	if err := reg.CreateUpload("avatar", config); err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	uploadInterface := reg.GetUpload("avatar")
	if uploadInterface == nil {
		t.Fatal("Expected upload, got nil")
	}
	upload, ok := uploadInterface.(*Upload)
	if !ok {
		t.Fatal("Expected *Upload type")
	}
	if upload.Name != "avatar" {
		t.Errorf("Expected name 'avatar', got %q", upload.Name)
	}
}

func TestUpload_AddEntry(t *testing.T) {
	upload := &Upload{
		Name: "avatar",
		Config: uploadtypes.UploadConfig{
			Accept:      []string{"image/*"},
			MaxEntries:  2,
			MaxFileSize: 1024 * 1024,
		},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	// Add valid entry
	entry1 := &uploadtypes.UploadEntry{
		ID:         "entry-1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 512 * 1024,
	}
	err := upload.AddEntry(entry1)
	if err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}
	if !entry1.Valid {
		t.Error("Expected entry to be valid")
	}
	if entry1.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Add second entry (at limit)
	entry2 := &uploadtypes.UploadEntry{
		ID:         "entry-2",
		ClientName: "photo2.jpg",
		ClientType: "image/jpeg",
		ClientSize: 512 * 1024,
	}
	err = upload.AddEntry(entry2)
	if err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Add third entry (exceeds limit)
	entry3 := &uploadtypes.UploadEntry{
		ID:         "entry-3",
		ClientName: "photo3.jpg",
		ClientType: "image/jpeg",
		ClientSize: 512 * 1024,
	}
	err = upload.AddEntry(entry3)
	if err == nil {
		t.Fatal("Expected error for exceeding max entries")
	}

	// Invalid entry should still be added but marked invalid
	if entry3.Valid {
		t.Error("Expected entry to be marked invalid")
	}
	if entry3.Error == "" {
		t.Error("Expected error message to be set")
	}
}

func TestUpload_GetEntries(t *testing.T) {
	upload := &Upload{
		Name: "avatar",
		Config: uploadtypes.UploadConfig{
			Accept: []string{"image/*"},
		},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	// Add some entries
	for i := range 3 {
		entry := &uploadtypes.UploadEntry{
			ID:         string(rune('a' + i)),
			ClientName: "photo.jpg",
			ClientType: "image/jpeg",
			ClientSize: 1024,
		}
		if err := upload.AddEntry(entry); err != nil {
			t.Fatalf("AddEntry failed: %v", err)
		}
	}

	entries := upload.GetEntries()
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}
}

func TestUpload_GetValidEntries(t *testing.T) {
	upload := &Upload{
		Name: "avatar",
		Config: uploadtypes.UploadConfig{
			Accept:     []string{"image/*"},
			MaxEntries: 2,
		},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	// Add first valid entry
	validEntry1 := &uploadtypes.UploadEntry{
		ID:         "valid1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
	}
	if err := upload.AddEntry(validEntry1); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Add second valid entry
	validEntry2 := &uploadtypes.UploadEntry{
		ID:         "valid2",
		ClientName: "photo2.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
	}
	if err := upload.AddEntry(validEntry2); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Add third entry (exceeds max entries)
	invalidEntry := &uploadtypes.UploadEntry{
		ID:         "invalid",
		ClientName: "photo3.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
	}
	// This will fail validation but should still succeed in adding the entry
	_ = upload.AddEntry(invalidEntry)

	// Get only valid entries
	validEntries := upload.GetValidEntries()
	if len(validEntries) != 2 {
		t.Errorf("Expected 2 valid entries, got %d", len(validEntries))
	}

	// Verify invalid entry is present but not in valid entries
	allEntries := upload.GetEntries()
	if len(allEntries) != 3 {
		t.Errorf("Expected 3 total entries, got %d", len(allEntries))
	}
}

func TestUpload_GetCompletedEntries(t *testing.T) {
	upload := &Upload{
		Name:    "avatar",
		Config:  uploadtypes.UploadConfig{Accept: []string{"image/*"}},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	// Add completed entry
	completedEntry := &uploadtypes.UploadEntry{
		ID:         "completed",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
		Done:       true,
	}
	if err := upload.AddEntry(completedEntry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Add incomplete entry
	incompleteEntry := &uploadtypes.UploadEntry{
		ID:         "incomplete",
		ClientName: "photo2.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
		Done:       false,
	}
	if err := upload.AddEntry(incompleteEntry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Get only completed entries
	completedEntries := upload.GetCompletedEntries()
	if len(completedEntries) != 1 {
		t.Errorf("Expected 1 completed entry, got %d", len(completedEntries))
	}
	if completedEntries[0].ID != "completed" {
		t.Errorf("Expected completed entry ID, got %q", completedEntries[0].ID)
	}
}

func TestUpload_UpdateEntry(t *testing.T) {
	upload := &Upload{
		Name:    "avatar",
		Config:  uploadtypes.UploadConfig{Accept: []string{"image/*"}},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "entry-1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
	}
	if err := upload.AddEntry(entry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Update progress
	err := upload.UpdateEntry("entry-1", func(e *uploadtypes.UploadEntry) {
		e.Progress = 50
	})
	if err != nil {
		t.Fatalf("UpdateEntry failed: %v", err)
	}

	updated := upload.GetEntry("entry-1")
	if updated.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", updated.Progress)
	}

	// Update non-existent entry should fail
	err = upload.UpdateEntry("nonexistent", func(e *uploadtypes.UploadEntry) {})
	if err == nil {
		t.Fatal("Expected error for non-existent entry")
	}
}

func TestUpload_RemoveEntry(t *testing.T) {
	upload := &Upload{
		Name:    "avatar",
		Config:  uploadtypes.UploadConfig{Accept: []string{"image/*"}},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "entry-1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
	}
	if err := upload.AddEntry(entry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	// Remove entry
	upload.RemoveEntry("entry-1")

	if upload.GetEntry("entry-1") != nil {
		t.Error("Expected entry to be removed")
	}
}

func TestUpload_HasError(t *testing.T) {
	upload := &Upload{
		Name:    "avatar",
		Config:  uploadtypes.UploadConfig{Accept: []string{"image/*"}},
		Entries: make(map[string]*uploadtypes.UploadEntry),
	}

	// No errors initially
	if upload.HasError() {
		t.Error("Expected no errors initially")
	}

	// Add entry with error
	entry := &uploadtypes.UploadEntry{
		ID:         "entry-1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
		Error:      "upload failed",
	}
	if err := upload.AddEntry(entry); err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	if !upload.HasError() {
		t.Error("Expected HasError to return true")
	}

	errorMsg := upload.GetError()
	if errorMsg != "upload failed" {
		t.Errorf("Expected error message 'upload failed', got %q", errorMsg)
	}
}
