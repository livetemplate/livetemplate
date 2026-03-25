package livetemplate

import (
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// defaultWriteBufferPool is a shared pool for gorilla write buffers.
// Using a pool avoids per-connection write buffer allocation (~1KB saved per connection).
// Intentionally has no New func — gorilla allocates the buffer on nil Get() and returns
// it to the pool after write, so the pool self-populates after the first write per connection.
var defaultWriteBufferPool = &sync.Pool{}

// GorillaUpgrader wraps gorilla/websocket.Upgrader as a WSUpgrader.
type GorillaUpgrader struct {
	inner *websocket.Upgrader
}

// GorillaOption configures the gorilla WebSocket upgrader.
type GorillaOption func(*websocket.Upgrader)

// NewGorillaUpgrader creates a WSUpgrader backed by gorilla/websocket.
// Default buffer sizes are 1024 bytes (optimized for LiveTemplate's small payloads).
// Write buffers are pooled via sync.Pool to avoid per-connection allocation.
func NewGorillaUpgrader(opts ...GorillaOption) *GorillaUpgrader {
	u := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		WriteBufferPool: defaultWriteBufferPool,
	}
	for _, opt := range opts {
		opt(u)
	}
	return &GorillaUpgrader{inner: u}
}

// WithGorillaReadBufferSize sets the read buffer size for the gorilla upgrader.
func WithGorillaReadBufferSize(size int) GorillaOption {
	return func(u *websocket.Upgrader) { u.ReadBufferSize = size }
}

// WithGorillaWriteBufferSize sets the write buffer size for the gorilla upgrader.
func WithGorillaWriteBufferSize(size int) GorillaOption {
	return func(u *websocket.Upgrader) { u.WriteBufferSize = size }
}

// WithGorillaCheckOrigin sets the origin check function for the gorilla upgrader.
func WithGorillaCheckOrigin(fn func(*http.Request) bool) GorillaOption {
	return func(u *websocket.Upgrader) { u.CheckOrigin = fn }
}

// WithGorillaWriteBufferPool sets a custom write buffer pool.
// Pass nil to disable pooling (each connection allocates its own buffer).
func WithGorillaWriteBufferPool(pool websocket.BufferPool) GorillaOption {
	return func(u *websocket.Upgrader) { u.WriteBufferPool = pool }
}

// WithGorillaCompression enables permessage-deflate compression.
// Reduces bandwidth for larger payloads at the cost of CPU.
func WithGorillaCompression() GorillaOption {
	return func(u *websocket.Upgrader) { u.EnableCompression = true }
}

// Copy creates a shallow copy of the upgrader to avoid mutating shared state.
func (g *GorillaUpgrader) Copy() *GorillaUpgrader {
	innerCopy := *g.inner
	return &GorillaUpgrader{inner: &innerCopy}
}

// SetCompression enables or disables permessage-deflate compression.
func (g *GorillaUpgrader) SetCompression(enabled bool) {
	g.inner.EnableCompression = enabled
}

// Upgrade upgrades an HTTP connection to a WebSocket connection.
func (g *GorillaUpgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (WSConn, error) {
	conn, err := g.inner.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	return &gorillaConn{conn: conn}, nil
}

// SetCheckOrigin sets the origin check function on the underlying gorilla upgrader.
func (g *GorillaUpgrader) SetCheckOrigin(fn func(*http.Request) bool) {
	g.inner.CheckOrigin = fn
}

// gorillaConn wraps gorilla's *websocket.Conn to implement WSConn.
type gorillaConn struct {
	conn *websocket.Conn
}

func (c *gorillaConn) ReadMessage() (int, []byte, error) {
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		return messageType, data, convertGorillaError(err)
	}
	return messageType, data, nil
}

func (c *gorillaConn) WriteMessage(messageType int, data []byte) error {
	return convertGorillaError(c.conn.WriteMessage(messageType, data))
}

func (c *gorillaConn) Close() error {
	return convertGorillaError(c.conn.Close())
}

// convertGorillaError converts gorilla-specific error types to LiveTemplate's WSCloseError.
func convertGorillaError(err error) error {
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		return &WSCloseError{Code: closeErr.Code, Text: closeErr.Text}
	}
	return err
}
