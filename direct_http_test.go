package livetemplate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/upload"
)

// directState is pure data for the Direct HTTP-completion test.
type directState struct {
	Ref string `lvt:"persist"`
}

// directController records the ExternalRef the completion action read, proving
// the WS-disabled HTTP upload_complete reconstructed the entry (#448).
type directController struct {
	savedRef string
}

// UploadAvatarComplete is the completion action for the "avatar" field: the
// framework builds the action name upload_avatar_complete from the field name,
// which snake-case-resolves to this method, so the name must track "avatar".
func (c *directController) UploadAvatarComplete(state directState, ctx *Context) (directState, error) {
	if ups := ctx.GetCompletedUploads("avatar"); len(ups) > 0 {
		state.Ref = ups[0].ExternalRef
		c.savedRef = ups[0].ExternalRef
	}
	return state, nil
}

func newDirectServer(t *testing.T, ctrl *directController) (*httptest.Server, []*http.Cookie) {
	t.Helper()
	tmpl, err := New("test",
		WithWebSocketDisabled(),
		WithUpload("avatar", UploadConfig{
			Mode:        UploadModeDirect,
			External:    &mockPresigner{meta: UploadMeta{Uploader: "s3", URL: "https://cdn.example/avatar.png"}},
			Accept:      []string{"image/png"},
			MaxFileSize: 1 << 20,
			AutoUpload:  true,
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tmpl, err = tmpl.Parse(`<div>{{.Ref}}</div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	server := httptest.NewServer(tmpl.Handle(ctrl, AsState(&directState{})))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	cookies := resp.Cookies()
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}
	return server, cookies
}

// postUpload posts a JSON body with the given X-Lvt-Upload header value.
func postUpload(t *testing.T, url, phase, body string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", url+"/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Lvt-Upload", phase)
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", phase, err)
	}
	return resp
}

// TestDirectUpload_CompleteOverHTTP is the #448 regression: with the WebSocket
// disabled, Direct's upload_start presigns over HTTP and upload_complete
// reconstructs the entry from client-asserted metadata so upload_<field>_complete
// runs and GetCompletedUploads returns the ExternalRef.
func TestDirectUpload_CompleteOverHTTP(t *testing.T) {
	ctrl := &directController{}
	server, cookies := newDirectServer(t, ctrl)

	// 1. upload_start over HTTP → presigned URL.
	startResp := postUpload(t, server.URL, "start",
		`{"action":"upload_start","upload_name":"avatar","files":[{"name":"avatar.png","type":"image/png","size":1024}]}`,
		cookies)
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("upload_start status = %d, want 200", startResp.StatusCode)
	}
	var startBody upload.UploadStartResponse
	if err := json.NewDecoder(startResp.Body).Decode(&startBody); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	_ = startResp.Body.Close()
	if len(startBody.Entries) != 1 || !startBody.Entries[0].Valid {
		t.Fatalf("expected 1 valid entry, got %+v", startBody.Entries)
	}
	if startBody.Entries[0].External == nil || startBody.Entries[0].External.URL == "" {
		t.Fatalf("expected presigned external URL, got %+v", startBody.Entries[0].External)
	}
	ref := startBody.Entries[0].External.URL

	// 2. Browser PUTs out-of-band, then upload_complete echoes the metadata + ref.
	completeResp := postUpload(t, server.URL, "complete",
		`{"action":"upload_complete","upload_name":"avatar","entries":[{"client_name":"avatar.png","type":"image/png","size":1024,"ref":"`+ref+`"}]}`,
		cookies)
	completeBody, _ := io.ReadAll(completeResp.Body)
	_ = completeResp.Body.Close()
	if completeResp.StatusCode != http.StatusOK {
		t.Fatalf("upload_complete status = %d, want 200; body=%s", completeResp.StatusCode, completeBody)
	}

	// The completion action ran and read the reconstructed entry's ExternalRef.
	if ctrl.savedRef != ref {
		t.Errorf("UploadAvatarComplete saw ref %q, want %q (entry not reconstructed for HTTP complete)", ctrl.savedRef, ref)
	}
	// The response carries the rendered tree.
	if !strings.Contains(string(completeBody), "tree") {
		t.Errorf("complete response should carry a tree, got %s", completeBody)
	}

	// State persisted: a fresh GET on the same session (cookie) renders the ref.
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	pageResp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("re-GET with cookie: %v", err)
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	_ = pageResp.Body.Close()
	if !strings.Contains(string(pageBody), ref) {
		t.Errorf("persisted page should contain ref %q, got %s", ref, pageBody)
	}
}

// TestDirectUpload_CompleteOverHTTP_RejectsBadMetadata verifies a client-asserted
// entry that violates the field config (wrong type) is not recorded as completed
// — the ref is not trusted into state.
func TestDirectUpload_CompleteOverHTTP_RejectsBadMetadata(t *testing.T) {
	ctrl := &directController{}
	server, cookies := newDirectServer(t, ctrl)

	resp := postUpload(t, server.URL, "complete",
		`{"action":"upload_complete","upload_name":"avatar","entries":[{"client_name":"evil.pdf","type":"application/pdf","size":1024,"ref":"https://cdn.example/evil.pdf"}]}`,
		cookies)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (field error, not transport error); body=%s", resp.StatusCode, body)
	}
	if ctrl.savedRef != "" {
		t.Errorf("rejected metadata must not be recorded as completed, but ref %q was read", ctrl.savedRef)
	}
}
