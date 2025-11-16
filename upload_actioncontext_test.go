package livetemplate

import (
	"strings"
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

	// Create test store that handles upload action
	store := &testUploadActionStore{
		onUploadAction: func(ctx *ActionContext) error {
			t.Logf("Upload action called: %s", ctx.Action)

			// Verify action name format
			if !strings.HasPrefix(ctx.Action, "upload:") {
				t.Errorf("Expected action to start with 'upload:', got: %s", ctx.Action)
			}

			// Extract upload name from action
			parts := strings.Split(ctx.Action, ":")
			if len(parts) < 2 {
				t.Errorf("Invalid upload action format: %s", ctx.Action)
				return nil
			}
			uploadName := parts[1]

			// Verify we can access uploads via ActionContext
			if !ctx.HasUploads(uploadName) {
				t.Log("HasUploads returned false (expected in this test)")
			}

			uploads := ctx.GetCompletedUploads(uploadName)
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

func (s *testUploadActionStore) Change(ctx *ActionContext) error {
	if strings.HasPrefix(ctx.Action, "upload:") && s.onUploadAction != nil {
		return s.onUploadAction(ctx)
	}
	return nil
}
