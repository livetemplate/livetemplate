package livetemplate

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// Upload-mode benchmarks (docs-repo examples upload-modes and avatar-upload
// as workload provenance). The four modes have radically different SERVER
// byte paths, so the benchmarks measure what the server actually does per
// mode — not a uniform fiction:
//
//   - Proxied: multipart POST streamed through StreamMultipart straight to
//     the controller's OnUpload reader (zero disk) — full byte path, swept
//     over file sizes.
//   - Volume: multipart POST staged to a retained file on disk — full byte
//     path + disk write, swept over file sizes.
//   - Direct:  bytes go browser→cloud storage; the server only handles the
//     upload_start protocol frame and presigns. No server byte path — file
//     sizes are a JSON number, so there is no size sweep.
//   - Preview: bytes never leave the device; the server records a metadata
//     entry. No server byte path — no size sweep.
//
// Proxied/Volume drive the REAL production entry point (handler.ServeHTTP
// with a multipart POST — the WS-disabled/progressive-enhancement flow);
// Direct/Preview drive the real WS upload_start protocol through the
// composite harness.

type uploadBenchState struct {
	Count int
}

type uploadBenchController struct {
	bytesSeen atomic.Int64
}

// OnUpload consumes the proxied stream the way storage-bound apps do,
// discarding at the destination boundary (the app's storage write).
func (c *uploadBenchController) OnUpload(part *UploadPart, _ *Context) error {
	n, err := io.Copy(io.Discard, part.Reader)
	c.bytesSeen.Add(n)
	return err
}

func (c *uploadBenchController) Submit(state uploadBenchState, _ *Context) (uploadBenchState, error) {
	return state, nil
}

const uploadBenchTemplate = `<div>{{.Count}}</div>`

// buildMultipartUpload returns a reusable multipart/form-data body carrying
// one file part of the given size under the given field name.
func buildMultipartUpload(tb testing.TB, field string, size int) (body []byte, contentType string) {
	tb.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, "bench.bin")
	if err != nil {
		tb.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(bytes.Repeat([]byte{0xA5}, size)); err != nil {
		tb.Fatalf("write payload: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("close writer: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

var uploadSizeSweep = []int{64 << 10, 1 << 20, 16 << 20}

func uploadSizeName(size int) string {
	if size >= 1<<20 {
		return fmt.Sprintf("%dMB", size>>20)
	}
	return fmt.Sprintf("%dKB", size>>10)
}

// runUploadPOSTBench drives one full multipart POST through the production
// handler per op. b.SetBytes makes Go report MB/s throughput.
func runUploadPOSTBench(b *testing.B, handler LiveHandler, body []byte, contentType string, size int) {
	b.Helper()
	b.SetBytes(int64(size))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		// The progressive-enhancement browser form flow answers a successful
		// POST with exactly 303 (POST-redirect-GET); an upload validation
		// failure re-renders with an implicit 200, so anything but 303 means
		// the bench measured an error path, not an upload.
		if rec.Code != http.StatusSeeOther {
			b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	}
}

func BenchmarkUpload_Proxied(b *testing.B) {
	discardLogs(b)
	for _, size := range uploadSizeSweep {
		b.Run(uploadSizeName(size), func(b *testing.B) {
			ctrl := &uploadBenchController{}
			tmpl := Must(New("upload-bench",
				WithMessageRateLimit(0, 0),
				WithProgressiveEnhancement(true), // pin the 303 PRG response path
				WithUpload("file", UploadConfig{Mode: UploadModeProxied, MaxFileSize: 64 << 20}),
			))
			if _, err := tmpl.Parse(uploadBenchTemplate); err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
			handler := tmpl.Handle(ctrl, AsState(&uploadBenchState{}))
			body, contentType := buildMultipartUpload(b, "file", size)

			runUploadPOSTBench(b, handler, body, contentType, size)

			b.StopTimer()
			if got := ctrl.bytesSeen.Load(); got != int64(b.N)*int64(size) {
				b.Fatalf("OnUpload saw %d bytes, want %d — the stream did not reach the controller", got, int64(b.N)*int64(size))
			}
		})
	}
}

func BenchmarkUpload_Volume(b *testing.B) {
	discardLogs(b)
	for _, size := range uploadSizeSweep {
		b.Run(uploadSizeName(size), func(b *testing.B) {
			// Dir selects the retained-staging streaming path (the
			// WS-disabled Volume flow); b.TempDir removes the staged files
			// after the sub-benchmark.
			dir := b.TempDir()
			tmpl := Must(New("upload-bench",
				WithMessageRateLimit(0, 0),
				WithProgressiveEnhancement(true), // pin the 303 PRG response path
				WithUpload("file", UploadConfig{Mode: UploadModeVolume, MaxFileSize: 64 << 20, Dir: dir}),
			))
			if _, err := tmpl.Parse(uploadBenchTemplate); err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
			handler := tmpl.Handle(&uploadBenchController{}, AsState(&uploadBenchState{}))
			body, contentType := buildMultipartUpload(b, "file", size)

			runUploadPOSTBench(b, handler, body, contentType, size)

			b.StopTimer()
			// Volume's counterpart to Proxied's bytesSeen guard: prove the
			// bytes were staged to the retained Dir, not silently dropped or
			// diverted to the ephemeral path.
			staged, err := filepath.Glob(filepath.Join(dir, "file", "*"))
			if err != nil || len(staged) != b.N {
				b.Fatalf("expected %d staged files in %s, got %d (err=%v)", b.N, dir, len(staged), err)
			}
			if fi, err := os.Stat(staged[0]); err != nil || fi.Size() != int64(size) {
				b.Fatalf("staged file size = %v (err=%v), want %d", fi, err, size)
			}
		})
	}
}

// stubPresigner stands in for the app's cloud presigner (S3 etc.). The
// framework cost under measurement is the upload_start protocol handling
// around it; real signing cost belongs to the app's SDK, not LiveTemplate.
type stubPresigner struct{}

func (stubPresigner) Presign(entry *UploadEntry) (UploadMeta, error) {
	return UploadMeta{
		Uploader: "s3",
		URL:      "https://bucket.invalid/" + entry.ID,
		Fields:   map[string]string{"key": entry.ID},
	}, nil
}

// uploadStartFrame is the client's upload_start protocol message for one
// 1MB file. For Direct/Preview the size is metadata only — no bytes follow.
var uploadStartFrame = []byte(`{"action":"upload_start","upload_name":"file","files":[{"name":"bench.bin","type":"application/octet-stream","size":1048576}]}`)

// runUploadStartBench drives one upload_start frame through the real WS
// event loop per op and awaits the protocol response frame.
func runUploadStartBench(b *testing.B, cfg UploadConfig) {
	b.Helper()
	app := newCompositeApp(b, uploadBenchTemplate, &uploadBenchController{},
		AsState(&uploadBenchState{}), WithUpload("file", cfg))
	s := app.connect(b, "")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.dispatch(b, uploadStartFrame)
	}
}

// BenchmarkUpload_Direct measures the server's ENTIRE work for a Direct-mode
// upload: the upload_start frame → validate → presign → entry + response.
// Bytes go browser→cloud, so there is no size axis on the server.
func BenchmarkUpload_Direct(b *testing.B) {
	discardLogs(b)
	runUploadStartBench(b, UploadConfig{Mode: UploadModeDirect, MaxFileSize: 64 << 20, External: stubPresigner{}})
}

// BenchmarkUpload_Preview measures the server's ENTIRE work for a Preview-
// mode upload: the upload_start frame → validate → metadata entry +
// response. Bytes never leave the device, so there is no size axis.
func BenchmarkUpload_Preview(b *testing.B) {
	discardLogs(b)
	runUploadStartBench(b, UploadConfig{Mode: UploadModePreview, MaxFileSize: 64 << 20})
}
