package context

import (
	"bytes"
	"testing"
)

// TestBufferReusable verifies the capacity gate that keeps oversized buffers out
// of bufferPool: buffers up to maxPooledBufferBytes are reusable, anything larger
// is dropped so a single huge render cannot pin memory in the pool.
func TestBufferReusable(t *testing.T) {
	cases := []struct {
		name string
		cap  int
		want bool
	}{
		{"empty", 0, true},
		{"typical", 8 << 10, true},
		{"at limit", maxPooledBufferBytes, true},
		{"over limit", maxPooledBufferBytes + 1, false},
		{"huge", 10 << 20, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			buf.Grow(tc.cap)
			if got := bufferReusable(buf); got != tc.want {
				t.Errorf("bufferReusable(cap=%d) = %v, want %v", buf.Cap(), got, tc.want)
			}
		})
	}
}

// TestPutBuffer_DropsOversized confirms putBuffer never returns an oversized
// buffer to the pool. This assertion is race-detector safe: a buffer that is
// never Put can never be handed back by Get, regardless of scheduling.
func TestPutBuffer_DropsOversized(t *testing.T) {
	big := new(bytes.Buffer)
	big.Grow(maxPooledBufferBytes + 1)
	putBuffer(big)
	if got := bufferPool.Get().(*bytes.Buffer); got == big {
		t.Fatalf("oversized buffer (cap %d) was retained in the pool", big.Cap())
	}
}
