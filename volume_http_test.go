package livetemplate

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// volumeState is pure data for the Volume HTTP-fallback test.
type volumeState struct {
	Path string `lvt:"persist"`
}

// volumeController records the retained on-disk path the Volume upload staged,
// proving the multipart POST reached stageVolumePart (not the ephemeral Tier-1
// path). savedPath is captured for direct assertion, like proxiedController.refs.
type volumeController struct {
	savedPath string
}

func (c *volumeController) UploadVolumeComplete(state volumeState, ctx *Context) (volumeState, error) {
	if ups := ctx.GetCompletedUploads("volume"); len(ups) > 0 {
		state.Path = ups[0].TempPath
		c.savedPath = ups[0].TempPath
	}
	return state, nil
}

// TestVolumeUpload_HTTPMultipart_StagesToDir is the #449 regression: with the
// WebSocket disabled, a Volume-with-Dir field must accept a single multipart
// POST and stage the bytes under cfg.Dir (retained), matching the WS-chunk path.
//
// Before the gate broadening, a Volume-only app has needsStreaming=false, so the
// POST falls into the ephemeral Tier-1 path and stages under .uploads, ignoring
// Dir — this test fails on that code.
func TestVolumeUpload_HTTPMultipart_StagesToDir(t *testing.T) {
	dir := t.TempDir()
	ctrl := &volumeController{}

	tmpl, err := New("test",
		WithWebSocketDisabled(),
		WithUpload("volume", UploadConfig{Mode: UploadModeVolume, Dir: dir, AutoUpload: true}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tmpl, err = tmpl.Parse(`<div>{{.Path}}</div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	server := httptest.NewServer(tmpl.Handle(ctrl, AsState(&volumeState{})))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		if err := os.RemoveAll(".uploads"); err != nil {
			t.Logf("cleanup .uploads: %v", err)
		}
	})

	getResp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	cookies := getResp.Cookies()
	if err := getResp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}

	// The disconnected Volume client posts a single multipart body: the
	// completion action (upload_<field>_complete, resolving to UploadVolumeComplete
	// for the "volume" field) plus the file part.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("lvt-action", "upload_volume_complete"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	part, err := writer.CreateFormFile("volume", "x-ray.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	req, err := http.NewRequest("POST", server.URL+"/", &body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", resp.StatusCode)
	}

	// The action must have seen the completed upload, with its retained path
	// under Dir — not the ephemeral .uploads tree.
	if c := ctrl.savedPath; c == "" {
		t.Fatal("UploadVolumeComplete saw no completed upload — Volume multipart fallback did not record an entry")
	}
	if !strings.HasPrefix(ctrl.savedPath, dir) {
		t.Errorf("staged path = %q, want under Dir %q (ephemeral Tier-1 path was used instead)", ctrl.savedPath, dir)
	}
	if _, err := os.Stat(ctrl.savedPath); err != nil {
		t.Errorf("retained file should exist under Dir: %v", err)
	}

	// Retention semantics: the file is under Dir, and nothing was staged under
	// the session temp tree.
	matches, _ := filepath.Glob(filepath.Join(dir, "volume", "*"))
	if len(matches) != 1 {
		t.Errorf("expected exactly 1 retained file under %s/volume, got %d", dir, len(matches))
	}
	if n := uploadsTempFileCount(t); n != 0 {
		t.Errorf("expected 0 files under .uploads (Dir-retained), got %d", n)
	}
}

// TestVolumeUpload_HTTPStart_NoOrphanFile verifies the WS-disabled Volume
// upload_start handshake does NOT pre-create a staging file at Dir: over HTTP the
// bytes arrive via a multipart POST (stageVolumePart), so a handshake-created
// file would be an orphaned empty file the app never cleans up.
func TestVolumeUpload_HTTPStart_NoOrphanFile(t *testing.T) {
	dir := t.TempDir()
	tmpl, err := New("test",
		WithWebSocketDisabled(),
		WithUpload("volume", UploadConfig{Mode: UploadModeVolume, Dir: dir, AutoUpload: true}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tmpl, err = tmpl.Parse(`<div></div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	server := httptest.NewServer(tmpl.Handle(&volumeController{}, AsState(&volumeState{})))
	t.Cleanup(server.Close)

	req, _ := http.NewRequest("POST", server.URL+"/",
		strings.NewReader(`{"action":"upload_start","upload_name":"volume","files":[{"name":"x.png","type":"image/png","size":10}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Lvt-Upload", "start")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST start: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload_start status = %d, want 200", resp.StatusCode)
	}

	// No staging file should exist under Dir yet — only the later multipart POST
	// creates the retained file.
	matches, _ := filepath.Glob(filepath.Join(dir, "volume", "*"))
	if len(matches) != 0 {
		t.Errorf("upload_start must not pre-create a Volume staging file over HTTP, found %d under %s/volume", len(matches), dir)
	}
}
