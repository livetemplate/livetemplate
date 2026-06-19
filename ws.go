package livetemplate

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/livetemplate/livetemplate/internal/session"
)

// WSConn is the interface for a WebSocket connection.
// Implementations can wrap gorilla/websocket, gws, gobwas/ws, coder/websocket,
// or any other WebSocket library.
type WSConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
}

// Compile-time assertions that this WSConn and the session package's mirror
// (session.WSConn, defined separately to avoid a circular import) stay in sync.
// Both directions are checked, so adding or removing a method on either
// interface without matching the other breaks the build here rather than
// silently at the connection-handoff call site.
var (
	_ session.WSConn = (WSConn)(nil)
	_ WSConn         = (session.WSConn)(nil)
)

// WSUpgrader upgrades an HTTP connection to a WebSocket connection.
type WSUpgrader interface {
	Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (WSConn, error)
}

// WebSocket message types (RFC 6455, Section 11.8)
const (
	WSTextMessage   = 1
	WSBinaryMessage = 2
	WSCloseMessage  = 8
	WSPingMessage   = 9
	WSPongMessage   = 10
)

// WebSocket close codes (RFC 6455, Section 7.4)
const (
	WSCloseNormalClosure   = 1000
	WSCloseGoingAway       = 1001
	WSCloseProtocolError   = 1002
	WSCloseAbnormalClosure = 1006
	WSCloseServiceRestart  = 1012
)

// WSCloseError represents a WebSocket close message.
type WSCloseError struct {
	Code int
	Text string
}

func (e *WSCloseError) Error() string {
	return fmt.Sprintf("websocket: close %d %s", e.Code, e.Text)
}

// WSCloseStatusText returns a text description for the close code.
func WSCloseStatusText(code int) string {
	switch code {
	case WSCloseNormalClosure:
		return "normal closure"
	case WSCloseGoingAway:
		return "going away"
	case WSCloseProtocolError:
		return "protocol error"
	case WSCloseAbnormalClosure:
		return "abnormal closure"
	case WSCloseServiceRestart:
		return "service restart"
	default:
		return "unknown"
	}
}

// WSFormatCloseMessage builds a WebSocket close frame payload.
// Per RFC 6455 §5.5, the close reason must be <= 123 bytes (125 - 2 for code).
func WSFormatCloseMessage(closeCode int, text string) []byte {
	if len(text) > 123 {
		text = text[:123]
		// Ensure we don't split a multi-byte UTF-8 character (RFC 6455 §8.1)
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	buf := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(buf, uint16(closeCode))
	copy(buf[2:], text)
	return buf
}

// WSIsUnexpectedCloseError reports whether err is a WebSocket close error
// with a code not in the list of expected codes.
func WSIsUnexpectedCloseError(err error, expectedCodes ...int) bool {
	var closeErr *WSCloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	for _, code := range expectedCodes {
		if closeErr.Code == code {
			return false
		}
	}
	return true
}

// WSIsUpgrade reports whether the HTTP request is a WebSocket upgrade request
// per RFC 6455 §4.1: a GET carrying Connection: upgrade, Upgrade: websocket,
// a Sec-WebSocket-Key, and Sec-WebSocket-Version: 13. Requests missing the
// Sec-WebSocket-* headers are not valid handshakes and are routed as plain HTTP
// (where the upgrade would otherwise fail at Upgrade() anyway).
func WSIsUpgrade(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		headerContainsValue(r.Header, "Connection", "upgrade") &&
		headerContainsValue(r.Header, "Upgrade", "websocket") &&
		r.Header.Get("Sec-WebSocket-Key") != "" &&
		headerContainsValue(r.Header, "Sec-WebSocket-Version", "13")
}

func headerContainsValue(h http.Header, key, val string) bool {
	for _, v := range h[http.CanonicalHeaderKey(key)] {
		for _, s := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(s), val) {
				return true
			}
		}
	}
	return false
}
