package context_test

import (
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate"
	"github.com/livetemplate/livetemplate/internal/context"
	"github.com/livetemplate/livetemplate/internal/upload"
)

func TestTemplateContext_Uploads(t *testing.T) {
	// Create upload registry
	registry := upload.NewRegistry()
	config := livetemplate.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 5,
	}

	if err := registry.CreateUpload("avatar", config); err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Add some entries
	avatarUploadInterface := registry.GetUpload("avatar")
	avatarUpload, ok := avatarUploadInterface.(*upload.Upload)
	if !ok || avatarUpload == nil {
		t.Fatal("Failed to get avatar upload")
	}
	entry1 := &livetemplate.UploadEntry{
		ID:         "entry-1",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024,
		Valid:      true,
		Done:       true,
	}
	if err := avatarUpload.AddEntry(entry1); err != nil {
		t.Fatalf("Failed to add entry: %v", err)
	}

	// Create template context with upload registry
	ctx := context.NewTemplateContext(nil, false)
	ctx.SetUploadRegistry(registry)

	// Test Uploads method
	entries := ctx.Uploads("avatar")
	if entries == nil {
		t.Fatal("Expected entries, got nil")
	}

	// Verify we can access the entries
	entriesSlice, ok := entries.([]*livetemplate.UploadEntry)
	if !ok {
		t.Fatalf("Expected []*livetemplate.UploadEntry, got %T", entries)
	}

	if len(entriesSlice) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entriesSlice))
	}

	if entriesSlice[0].ClientName != "photo.jpg" {
		t.Errorf("Expected filename 'photo.jpg', got %q", entriesSlice[0].ClientName)
	}
}

func TestTemplateContext_Uploads_NonExistent(t *testing.T) {
	registry := upload.NewRegistry()
	ctx := context.NewTemplateContext(nil, false)
	ctx.SetUploadRegistry(registry)

	// Test non-existent upload
	entries := ctx.Uploads("nonexistent")
	if entries != nil {
		t.Error("Expected nil for non-existent upload")
	}
}

func TestTemplateContext_Uploads_NoRegistry(t *testing.T) {
	ctx := context.NewTemplateContext(nil, false)

	// Test without setting registry
	entries := ctx.Uploads("avatar")
	if entries != nil {
		t.Error("Expected nil when no registry is set")
	}
}

func TestTemplateContext_HasUploadError(t *testing.T) {
	registry := upload.NewRegistry()
	config := livetemplate.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 1,
	}

	if err := registry.CreateUpload("avatar", config); err != nil {
		t.Fatalf("Failed to create upload: %v", err)
	}
	avatarUploadInterface := registry.GetUpload("avatar")
	avatarUpload, ok := avatarUploadInterface.(*upload.Upload)
	if !ok || avatarUpload == nil {
		t.Fatal("Failed to get avatar upload")
	}

	// Add entry with invalid type - this triggers validation error but entry is still added
	entry := &livetemplate.UploadEntry{
		ID:         "entry-1",
		ClientName: "file.txt",
		ClientType: "text/plain",
		ClientSize: 1024,
	}
	// AddEntry returns error for invalid entries, but still adds them for error display
	_ = avatarUpload.AddEntry(entry)

	ctx := context.NewTemplateContext(nil, false)
	ctx.SetUploadRegistry(registry)

	// Test HasUploadError - entry was added with error due to type mismatch
	if !ctx.HasUploadError("avatar") {
		t.Error("Expected HasUploadError to return true")
	}

	// Test non-existent upload
	if ctx.HasUploadError("nonexistent") {
		t.Error("Expected HasUploadError to return false for non-existent upload")
	}
}

func TestTemplateContext_UploadError(t *testing.T) {
	registry := upload.NewRegistry()
	config := livetemplate.UploadConfig{
		Accept:     []string{"image/*"},
		MaxEntries: 1,
	}

	if err := registry.CreateUpload("avatar", config); err != nil {
		t.Fatalf("Failed to create upload: %v", err)
	}
	avatarUploadInterface := registry.GetUpload("avatar")
	avatarUpload, ok := avatarUploadInterface.(*upload.Upload)
	if !ok || avatarUpload == nil {
		t.Fatal("Failed to get avatar upload")
	}

	// Add entry with invalid type - this triggers validation error but entry is still added
	entry := &livetemplate.UploadEntry{
		ID:         "entry-1",
		ClientName: "file.txt",
		ClientType: "text/plain",
		ClientSize: 1024,
	}
	// AddEntry returns error for invalid entries, but still adds them for error display
	_ = avatarUpload.AddEntry(entry)

	ctx := context.NewTemplateContext(nil, false)
	ctx.SetUploadRegistry(registry)

	// Test UploadError - entry was added with error due to type mismatch
	errMsg := ctx.UploadError("avatar")
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}
	// Error message should contain information about the invalid file type
	if !strings.Contains(errMsg, "file type") && !strings.Contains(errMsg, "type:") {
		t.Errorf("Expected error message to mention file type, got %q", errMsg)
	}

	// Test non-existent upload
	errMsg = ctx.UploadError("nonexistent")
	if errMsg != "" {
		t.Errorf("Expected empty error for non-existent upload, got %q", errMsg)
	}

	// Test upload with no errors
	if err := registry.CreateUpload("documents", config); err != nil {
		t.Fatalf("Failed to create documents upload: %v", err)
	}
	errMsg = ctx.UploadError("documents")
	if errMsg != "" {
		t.Errorf("Expected empty error for upload with no errors, got %q", errMsg)
	}
}

func TestTemplateContext_UploadError_NoRegistry(t *testing.T) {
	ctx := context.NewTemplateContext(nil, false)

	// Test without registry
	if ctx.HasUploadError("avatar") {
		t.Error("Expected false when no registry is set")
	}

	errMsg := ctx.UploadError("avatar")
	if errMsg != "" {
		t.Errorf("Expected empty string when no registry is set, got %q", errMsg)
	}
}
