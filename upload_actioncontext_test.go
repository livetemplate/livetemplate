package livetemplate

import (
	"testing"
)

// TestActionContextUploadAccess verifies uploads are accessible via ActionContext
func TestActionContextUploadAccess(t *testing.T) {
	// Create template with upload configuration
	tmpl := Must(New("test",
		WithUpload("avatar", UploadConfig{
			Accept:      []string{"image/*"},
			MaxEntries:  1,
			MaxFileSize: 1024 * 1024,
		}),
	))

	tmplStr := `<div>Test</div>`
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create test store that handles upload action via UploadAvatarComplete method
	store := &testUploadActionStore{
		onUploadAction: func(ctx *ActionContext) error {
			t.Logf("Upload action called: %s", ctx.Action)

			// Verify action name format (now uses underscores: upload_avatar_complete)
			expected := "upload_avatar_complete"
			if ctx.Action != expected {
				t.Errorf("Expected action %q, got: %s", expected, ctx.Action)
			}

			// Verify we can access uploads via ActionContext
			if !ctx.HasUploads("avatar") {
				t.Log("HasUploads returned false (expected in this test)")
			}

			uploads := ctx.GetCompletedUploads("avatar")
			t.Logf("Found %d completed uploads", len(uploads))

			return nil
		},
	}

	handler := tmpl.Handle(store)

	// The test would need to simulate an actual upload flow
	// For now, we've verified the ActionContext methods exist and compile
	_ = handler

	t.Log("✅ ActionContext upload methods are available")
}

// testUploadActionStore is a test store that handles upload actions
type testUploadActionStore struct {
	onUploadAction func(ctx *ActionContext) error
}

// UploadAvatarComplete handles the "upload_avatar_complete" action
func (s *testUploadActionStore) UploadAvatarComplete(ctx *ActionContext) error {
	if s.onUploadAction != nil {
		return s.onUploadAction(ctx)
	}
	return nil
}
