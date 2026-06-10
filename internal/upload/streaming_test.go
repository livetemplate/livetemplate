package upload

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

func TestLimitGuard_UnderAndExactLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		max  int64
	}{
		{"under", "hello", 10},
		{"exact", "hello", 5},
		{"unlimited", "hello world", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewLimitGuard(strings.NewReader(tc.data), tc.max)
			got, err := io.ReadAll(g)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.data {
				t.Errorf("got %q, want %q", got, tc.data)
			}
		})
	}
}

// TestLimitGuard_OverLimitDeliversAtMostMax verifies the guard never hands the
// oversize bytes to the consumer — a streaming copy aborts with ErrUploadTooLarge
// having seen at most max bytes, so no partial object is committed downstream.
func TestLimitGuard_OverLimitDeliversAtMostMax(t *testing.T) {
	const max = 4
	src := strings.NewReader("0123456789") // 10 bytes, limit 4

	// Copy through the guard the way OnUpload would, into a sink that records
	// exactly what it received.
	var sink bytes.Buffer
	g := NewLimitGuard(src, max)
	_, err := io.Copy(&sink, g)

	if !errors.Is(err, uploadtypes.ErrUploadTooLarge) {
		t.Fatalf("expected ErrUploadTooLarge, got %v", err)
	}
	if int64(sink.Len()) > max {
		t.Errorf("sink received %d bytes, must be <= max (%d) — oversize bytes leaked", sink.Len(), max)
	}
}
