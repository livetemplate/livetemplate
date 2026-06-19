package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWSIsUpgrade(t *testing.T) {
	// validReq builds a fully-valid RFC 6455 handshake, then applies mutate.
	validReq := func(mutate func(h http.Header)) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Connection", "Upgrade")
		r.Header.Set("Upgrade", "websocket")
		r.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		r.Header.Set("Sec-WebSocket-Version", "13")
		mutate(r.Header)
		return r
	}

	tests := []struct {
		name   string
		mutate func(h http.Header)
		want   bool
	}{
		{"valid handshake", func(http.Header) {}, true},
		{"version list containing 13", func(h http.Header) { h.Set("Sec-WebSocket-Version", "8, 13") }, true},
		{"missing Sec-WebSocket-Key", func(h http.Header) { h.Del("Sec-WebSocket-Key") }, false},
		{"empty Sec-WebSocket-Key", func(h http.Header) { h.Set("Sec-WebSocket-Key", "") }, false},
		{"missing Sec-WebSocket-Version", func(h http.Header) { h.Del("Sec-WebSocket-Version") }, false},
		{"wrong Sec-WebSocket-Version", func(h http.Header) { h.Set("Sec-WebSocket-Version", "8") }, false},
		{"missing Upgrade header", func(h http.Header) { h.Del("Upgrade") }, false},
		{"missing Connection header", func(h http.Header) { h.Del("Connection") }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WSIsUpgrade(validReq(tt.mutate)); got != tt.want {
				t.Errorf("WSIsUpgrade() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("non-GET method", func(t *testing.T) {
		r := validReq(func(http.Header) {})
		r.Method = http.MethodPost
		if WSIsUpgrade(r) {
			t.Error("WSIsUpgrade() = true for POST, want false")
		}
	})
}

func TestGorillaUpgrader_PerInstanceWriteBufferPool(t *testing.T) {
	u1 := NewGorillaUpgrader()
	u2 := NewGorillaUpgrader()

	if u1.inner.WriteBufferPool == nil {
		t.Fatal("expected a non-nil default WriteBufferPool")
	}
	// Each upgrader must own its pool so that upgraders with different
	// WriteBufferSize values never share mismatched buffers.
	if u1.inner.WriteBufferPool == u2.inner.WriteBufferPool {
		t.Error("expected per-upgrader WriteBufferPool, got a shared instance")
	}

	// An explicit pool via WithGorillaWriteBufferPool still overrides the default.
	shared := &sync.Pool{}
	u3 := NewGorillaUpgrader(WithGorillaWriteBufferPool(shared))
	if u3.inner.WriteBufferPool != websocket.BufferPool(shared) {
		t.Error("WithGorillaWriteBufferPool did not override the default pool")
	}
}
