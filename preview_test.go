package livetemplate

import (
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/upload"
)

// TestPreviewUpload_SynthesizesMetadataEntry verifies that a Preview field's
// handshake records a metadata-only entry (no bytes, no disk) that the app reads
// via GetCompletedUploads to render a placeholder.
func TestPreviewUpload_SynthesizesMetadataEntry(t *testing.T) {
	reg := upload.NewRegistry()
	if err := reg.CreateUpload("draft", UploadConfig{Mode: UploadModePreview}); err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	h := &liveHandler{}
	raw := []byte(`{"action":"upload_start","upload_name":"draft","files":[{"name":"p.png","type":"image/png","size":10}]}`)
	resp, err := h.buildUploadStartResponse(raw, "sess", reg, false)
	if err != nil {
		t.Fatalf("buildUploadStartResponse: %v", err)
	}
	if len(resp.Entries) != 1 || !resp.Entries[0].Valid {
		t.Fatalf("expected 1 valid entry, got %+v", resp.Entries)
	}
	if resp.Entries[0].Mode != "preview" {
		t.Errorf("expected mode=preview, got %q", resp.Entries[0].Mode)
	}

	u := reg.GetUpload("draft").(*upload.Upload)
	completed := u.GetCompletedEntries()
	if len(completed) != 1 {
		t.Fatalf("expected 1 completed (preview) entry, got %d", len(completed))
	}
	e := completed[0]
	if !e.Preview {
		t.Error("expected entry.Preview=true")
	}
	if e.TempPath != "" || e.ExternalRef != "" {
		t.Errorf("preview entry must carry no bytes ref: TempPath=%q ExternalRef=%q", e.TempPath, e.ExternalRef)
	}
	if e.ClientName != "p.png" || e.ClientSize != 10 {
		t.Errorf("expected metadata p.png/10, got %s/%d", e.ClientName, e.ClientSize)
	}
}

// TestUploadPreviewHelper verifies the template helper emits the placeholder the
// client fills with a local object URL.
func TestUploadPreviewHelper(t *testing.T) {
	tmpl, err := New("test", WithUpload("draft", UploadConfig{Mode: UploadModePreview}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := tmpl.Parse(`<div>{{.lvt.UploadPreview "draft"}}</div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, AsState(&testHandleState{}), nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, `data-lvt-upload-preview="draft"`) {
		t.Errorf("expected preview placeholder in output, got: %s", out)
	}
}
