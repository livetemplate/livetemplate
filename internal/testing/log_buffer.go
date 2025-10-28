package testing

import (
	"bytes"
	"sync"
)

// SafeBuffer is a concurrency-safe buffer for capturing logs from processes or goroutines.
type SafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// NewSafeBuffer returns an empty SafeBuffer.
func NewSafeBuffer() *SafeBuffer {
	return &SafeBuffer{}
}

// Write appends the provided bytes to the buffer in a thread-safe manner.
func (b *SafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the buffered contents as a string.
func (b *SafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Bytes returns a copy of the buffered bytes.
func (b *SafeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := b.buf.Bytes()
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

// Reset clears the buffer contents.
func (b *SafeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}
