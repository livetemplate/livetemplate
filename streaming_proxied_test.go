package livetemplate

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proxiedState is pure data for the Proxied streaming tests.
type proxiedState struct {
	Ref string `lvt:"persist"`
}

// proxiedController implements UploadStreamer, streaming each part into an
// in-memory "remote" backend instead of local disk.
type proxiedController struct {
	received map[string][]byte
	refs     []string
}

func (c *proxiedController) OnUpload(part *UploadPart, ctx *Context) error {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, part); err != nil {
		return err // e.g. ErrUploadTooLarge — aborts before SetResult
	}
	if c.received == nil {
		c.received = make(map[string][]byte)
	}
	c.received[part.Filename] = buf.Bytes()
	part.SetResult("memory://" + part.Filename)
	return nil
}

func (c *proxiedController) SaveDoc(state proxiedState, ctx *Context) (proxiedState, error) {
	for _, e := range ctx.GetCompletedUploads("doc") {
		state.Ref = e.ExternalRef
		c.refs = append(c.refs, e.ExternalRef)
	}
	return state, nil
}

// postProxiedFile POSTs a multipart form with one "doc" file part and the
// SaveDoc action, returning the response status.
func postProxiedFile(t *testing.T, serverURL string, cookies []*http.Cookie, filename string, content []byte) int {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("lvt-action", "SaveDoc"); err != nil {
		t.Fatalf("WriteField failed: %v", err)
	}
	part, err := writer.CreateFormFile("doc", filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer close failed: %v", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/", &body)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close failed: %v", err)
	}
	return resp.StatusCode
}

func newProxiedServer(t *testing.T, cfg UploadConfig, ctrl *proxiedController) (*httptest.Server, []*http.Cookie) {
	t.Helper()
	tmpl, err := New("test", WithUpload("doc", cfg))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{.Ref}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	server := httptest.NewServer(tmpl.Handle(ctrl, AsState(&proxiedState{})))
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		if err := os.RemoveAll(".uploads"); err != nil {
			t.Logf("cleanup .uploads failed: %v", err)
		}
	})

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	cookies := resp.Cookies()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("body close failed: %v", err)
	}
	return server, cookies
}

// uploadsTempFileCount counts regular files staged under ./.uploads (the temp
// staging tree). Proxied streaming must never write here.
func uploadsTempFileCount(t *testing.T) int {
	t.Helper()
	count := 0
	_ = filepath.Walk(".uploads", func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info != nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func TestProxiedUpload_StreamsToBackend_ZeroDisk(t *testing.T) {
	ctrl := &proxiedController{}
	server, cookies := newProxiedServer(t, UploadConfig{
		Mode:        UploadModeProxied,
		Accept:      []string{"image/png"},
		MaxFileSize: 5 << 20,
	}, ctrl)

	payload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x01, 0x02}
	if status := postProxiedFile(t, server.URL, cookies, "scan.png", payload); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	// The handler streamed the exact bytes to the in-memory backend.
	got, ok := ctrl.received["scan.png"]
	if !ok {
		t.Fatal("backend did not receive the file")
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("backend bytes mismatch: got %x want %x", got, payload)
	}

	// The follow-on action read the streamed result via ExternalRef.
	if len(ctrl.refs) != 1 || ctrl.refs[0] != "memory://scan.png" {
		t.Errorf("expected ExternalRef from GetCompletedUploads, got %v", ctrl.refs)
	}

	// Zero-disk: nothing was staged under .uploads.
	if n := uploadsTempFileCount(t); n != 0 {
		t.Errorf("expected zero staged temp files, found %d", n)
	}
}

func TestProxiedUpload_OversizeAborts(t *testing.T) {
	ctrl := &proxiedController{}
	server, cookies := newProxiedServer(t, UploadConfig{
		Mode:        UploadModeProxied,
		MaxFileSize: 4, // smaller than the payload
	}, ctrl)

	if status := postProxiedFile(t, server.URL, cookies, "big.bin", []byte("0123456789")); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	// Oversize stream aborts: no completed upload, no ExternalRef recorded.
	if len(ctrl.refs) != 0 {
		t.Errorf("expected no completed uploads on oversize, got %v", ctrl.refs)
	}
	if n := uploadsTempFileCount(t); n != 0 {
		t.Errorf("expected zero staged temp files, found %d", n)
	}
}

type mixedState struct{}

type mixedController struct {
	proxied map[string][]byte
}

func (c *mixedController) OnUpload(part *UploadPart, ctx *Context) error {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, part); err != nil {
		return err
	}
	if c.proxied == nil {
		c.proxied = make(map[string][]byte)
	}
	c.proxied[part.Filename] = buf.Bytes()
	part.SetResult("mem://" + part.Filename)
	return nil
}

func (c *mixedController) Submit(s mixedState, ctx *Context) (mixedState, error) { return s, nil }

// TestMixedProxiedVolume_StagesBothInOneRequest verifies that a Volume file part
// sharing a streaming (Proxied) multipart request is staged to disk rather than
// silently dropped.
func TestMixedProxiedVolume_StagesBothInOneRequest(t *testing.T) {
	volumeDir := t.TempDir()
	ctrl := &mixedController{}
	tmpl := Must(New("test",
		WithUpload("doc", UploadConfig{Mode: UploadModeProxied}),
		WithUpload("scan", UploadConfig{Mode: UploadModeVolume, Dir: volumeDir}),
	))
	tmpl = Must(tmpl.Parse(`<div>ok</div>`))
	server := httptest.NewServer(tmpl.Handle(ctrl, AsState(&mixedState{})))
	t.Cleanup(server.Close)
	t.Cleanup(func() { _ = os.RemoveAll(".uploads") })

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	cookies := resp.Cookies()
	_ = resp.Body.Close()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("lvt-action", "Submit")
	docPart, _ := w.CreateFormFile("doc", "d.png")
	_, _ = docPart.Write([]byte("proxied-bytes"))
	scanPart, _ := w.CreateFormFile("scan", "s.png")
	_, _ = scanPart.Write([]byte("volume-bytes"))
	_ = w.Close()

	req, _ := http.NewRequest("POST", server.URL+"/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp2, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	// Proxied part streamed to OnUpload.
	if got := ctrl.proxied["d.png"]; string(got) != "proxied-bytes" {
		t.Errorf("proxied part = %q, want %q", got, "proxied-bytes")
	}
	// Volume part staged to disk (not dropped).
	found := false
	_ = filepath.Walk(volumeDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			if b, _ := os.ReadFile(p); string(b) == "volume-bytes" {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Error("volume part was dropped — expected it staged under Dir")
	}
}

func TestProxiedUpload_HTTPHandshake_WSDisabled(t *testing.T) {
	ctrl := &proxiedController{}
	server, cookies := newProxiedServer(t, UploadConfig{
		Mode:   UploadModeProxied,
		Accept: []string{"image/*"},
	}, ctrl)

	body := `{"action":"upload_start","upload_name":"doc","files":[{"name":"scan.png","type":"image/png","size":10}]}`
	req, err := http.NewRequest("POST", server.URL+"/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Lvt-Upload", "start") // marks the WS-disabled handshake
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		UploadName string `json:"upload_name"`
		Entries    []struct {
			Valid bool   `json:"valid"`
			Mode  string `json:"mode"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode handshake response: %v", err)
	}
	if out.UploadName != "doc" || len(out.Entries) != 1 {
		t.Fatalf("unexpected handshake response: %+v", out)
	}
	if !out.Entries[0].Valid || out.Entries[0].Mode != "proxied" {
		t.Errorf("expected valid proxied entry, got %+v", out.Entries[0])
	}
}

func TestProxiedUpload_RejectsDisallowedType(t *testing.T) {
	ctrl := &proxiedController{}
	server, cookies := newProxiedServer(t, UploadConfig{
		Mode:   UploadModeProxied,
		Accept: []string{"image/png"},
	}, ctrl)

	if status := postProxiedFile(t, server.URL, cookies, "notes.txt", []byte("hello")); status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	// Accept rejection happens from the header before OnUpload runs.
	if _, ok := ctrl.received["notes.txt"]; ok {
		t.Error("OnUpload should not run for a disallowed type")
	}
	if len(ctrl.refs) != 0 {
		t.Errorf("expected no completed uploads, got %v", ctrl.refs)
	}
}
