package livetemplate

import (
	"os"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/upload"
)

const volumeStartMsg = `{"action":"upload_start","upload_name":"scan","files":[{"name":"x.png","type":"image/png","size":10}]}`

// TestVolumeUpload_RetainsFileAtDir verifies that a Volume field with Dir set
// stages the file under that directory (not the session temp tree) so it
// survives session cleanup — the app owns its lifecycle.
func TestVolumeUpload_RetainsFileAtDir(t *testing.T) {
	dir := t.TempDir()
	reg := upload.NewRegistry()
	if err := reg.CreateUpload("scan", UploadConfig{Mode: UploadModeVolume, Dir: dir}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	// No temp file manager needed: Dir-retained Volume never uses it.
	h := &liveHandler{}
	resp, err := h.buildUploadStartResponse([]byte(volumeStartMsg), "sess-1", reg, false)
	if err != nil {
		t.Fatalf("buildUploadStartResponse: %v", err)
	}
	if len(resp.Entries) != 1 || !resp.Entries[0].Valid {
		t.Fatalf("expected 1 valid entry, got %+v", resp.Entries)
	}
	if resp.Entries[0].Mode != "volume" {
		t.Errorf("expected mode=volume, got %q", resp.Entries[0].Mode)
	}

	u := reg.GetUpload("scan").(*upload.Upload)
	entry := u.GetEntry(resp.Entries[0].EntryID)
	if entry == nil {
		t.Fatal("entry not found in registry")
	}
	if !strings.HasPrefix(entry.TempPath, dir) {
		t.Errorf("expected TempPath under Dir %q, got %q", dir, entry.TempPath)
	}
	if _, err := os.Stat(entry.TempPath); err != nil {
		t.Errorf("retained file should exist: %v", err)
	}
}

// TestDirectUpload_NilExternal_NoPanic verifies the handshake returns an error
// entry (not a panic) when a field is explicitly Mode:Direct without a presigner.
func TestDirectUpload_NilExternal_NoPanic(t *testing.T) {
	reg := upload.NewRegistry()
	if err := reg.CreateUpload("x", UploadConfig{Mode: UploadModeDirect}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	h := &liveHandler{}
	raw := []byte(`{"action":"upload_start","upload_name":"x","files":[{"name":"a.png","type":"image/png","size":10}]}`)
	resp, err := h.buildUploadStartResponse(raw, "sess", reg, false)
	if err != nil {
		t.Fatalf("buildUploadStartResponse: %v", err)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].Valid {
		t.Fatalf("expected 1 invalid entry, got %+v", resp.Entries)
	}
	if resp.Entries[0].Error == "" {
		t.Error("expected an error message for Direct mode without External")
	}
}

// TestVolumeUpload_EphemeralWithoutDir verifies that a Volume field with no Dir
// stages under the session temp tree (.uploads), preserving the legacy
// stage-then-app-moves pattern.
func TestVolumeUpload_EphemeralWithoutDir(t *testing.T) {
	tfm, err := upload.NewTempFileManager("")
	if err != nil {
		t.Fatalf("NewTempFileManager: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(".uploads"); err != nil {
			t.Logf("cleanup .uploads failed: %v", err)
		}
	})

	reg := upload.NewRegistry()
	if err := reg.CreateUpload("scan", UploadConfig{Mode: UploadModeVolume}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	h := &liveHandler{tempFileManager: tfm}
	resp, err := h.buildUploadStartResponse([]byte(volumeStartMsg), "sess-1", reg, false)
	if err != nil {
		t.Fatalf("buildUploadStartResponse: %v", err)
	}
	if len(resp.Entries) != 1 || !resp.Entries[0].Valid {
		t.Fatalf("expected 1 valid entry, got %+v", resp.Entries)
	}

	u := reg.GetUpload("scan").(*upload.Upload)
	entry := u.GetEntry(resp.Entries[0].EntryID)
	if entry == nil || !strings.Contains(entry.TempPath, ".uploads") {
		t.Errorf("expected ephemeral TempPath under .uploads, got %q", entry.TempPath)
	}
}
