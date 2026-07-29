package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/benchharness"
)

// This file is the honest composite-pipeline harness: it drives the REAL
// action cycle every app runs on every interaction —
//
//	action dispatch → controller method → render → diff → serialize →
//	Connection.Send → writePump → WriteMessage
//
// — through the production handler (mount, event loop, registry, pump),
// faking only the syscall via benchharness.Conn. Contrast with
// e2e_bench_test.go's *_ExecuteUpdatesOnly benchmarks, which measure the
// render+diff primitive in isolation.

type compositeBenchState struct {
	Count int
}

type compositeBenchController struct{}

func (c *compositeBenchController) Increment(state compositeBenchState, _ *Context) (compositeBenchState, error) {
	state.Count++
	return state, nil
}

// benchUpgrader hands a pre-built scripted conn to the handler instead of
// performing a real HTTP 101 upgrade; everything downstream of Upgrade is
// the production path.
type benchUpgrader struct {
	conn *benchharness.Conn
}

func (u *benchUpgrader) Upgrade(http.ResponseWriter, *http.Request, http.Header) (WSConn, error) {
	return u.conn, nil
}

// compositeSession is a live WebSocket session driven end-to-end through the
// real handler with a benchharness.Conn at the syscall boundary.
type compositeSession struct {
	conn *benchharness.Conn
	done chan struct{} // closed when the handler's event loop exits
}

// benchCounterTemplate mirrors the counter shape used by the old E2E journey
// bench and the docs counter example.
const benchCounterTemplate = `<div><button>{{.Count}}</button></div>`

// startCompositeSession builds a Template+Handle app around a scripted conn
// and connects one WebSocket session through the real handleWebSocket path.
// It returns after the initial render has reached the write boundary.
func startCompositeSession(tb testing.TB, templateSrc string, controller interface{}, state State) *compositeSession {
	tb.Helper()

	conn := benchharness.NewConn()
	tmpl := Must(New("composite-bench",
		WithUpgrader(&benchUpgrader{conn: conn}),
		// The default rate limit (10 msg/s) would throttle the benchmark and
		// turn measured ops into rate-limit error responses.
		WithMessageRateLimit(0, 0),
	))
	if _, err := tmpl.Parse(templateSrc); err != nil {
		tb.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(controller, state)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "composite-bench")
	req.Header.Set("Sec-WebSocket-Version", "13")

	s := &compositeSession{conn: conn, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}()

	select {
	case <-conn.Writes(): // initial tree render reached the write boundary
	case <-s.done:
		tb.Fatal("handler exited before sending the initial render")
	}
	tb.Cleanup(s.close)
	return s
}

// dispatch feeds one action frame into the real event loop and blocks until
// the resulting update has been serialized and handed to WriteMessage.
func (s *compositeSession) dispatch(tb testing.TB, frame []byte) {
	if err := s.conn.FeedRead(frame); err != nil {
		tb.Fatalf("FeedRead failed: %v", err)
	}
	select {
	case <-s.conn.Writes():
	case <-s.done:
		tb.Fatal("handler exited mid-dispatch")
	}
}

func (s *compositeSession) close() {
	_ = s.conn.Close()
	<-s.done
}

// startCounterSession is the common counter-app setup shared by the
// composite benchmarks and the harness guard test.
func startCounterSession(tb testing.TB) *compositeSession {
	return startCompositeSession(tb, benchCounterTemplate,
		&compositeBenchController{}, AsState(&compositeBenchState{}))
}

var incrementFrame = []byte(`{"action":"Increment"}`)

// BenchmarkCompositeUpdate measures one full interaction — dispatch through
// controller method, render, diff, serialize, and the real write pump — on a
// single connection. This is the unit of work behind every button click in
// consumer apps (counter example, muster /ports actions).
func BenchmarkCompositeUpdate(b *testing.B) {
	s := startCounterSession(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.dispatch(b, incrementFrame)
	}
}

// BenchmarkE2EUserJourney measures a 100-action user journey through the
// full composite pipeline. It replaces the old bench of the same name, which
// looped tmpl.ExecuteUpdates in isolation (kept as
// BenchmarkUserJourney_ExecuteUpdatesOnly for contrast).
func BenchmarkE2EUserJourney(b *testing.B) {
	s := startCounterSession(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			s.dispatch(b, incrementFrame)
		}
	}
}

// BenchmarkCompositeUpdate_LoopbackWS is the fidelity spot-check for the
// benchharness harness: the same app and action driven over a real loopback
// WebSocket (httptest server + gorilla dial). Its per-op cost should be the
// composite number plus real socket I/O; a large divergence means the
// harness fake got too cheap to be meaningful. Not part of the CI gate.
func BenchmarkCompositeUpdate_LoopbackWS(b *testing.B) {
	tmpl := Must(New("composite-loopback", WithMessageRateLimit(0, 0)))
	if _, err := tmpl.Parse(benchCounterTemplate); err != nil {
		b.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(&compositeBenchController{}, AsState(&compositeBenchState{}))
	server := httptest.NewServer(handler)
	defer server.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		b.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = ws.Close() }()
	if err := ws.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		b.Fatalf("SetReadDeadline failed: %v", err)
	}
	if _, _, err := ws.ReadMessage(); err != nil {
		b.Fatalf("initial render read failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := ws.WriteMessage(websocket.TextMessage, incrementFrame); err != nil {
			b.Fatalf("write failed: %v", err)
		}
		if _, _, err := ws.ReadMessage(); err != nil {
			b.Fatalf("read failed: %v", err)
		}
	}
}

// TestCompositeHarnessSerializes is the guard that keeps the harness honest:
// it proves benchharness.Conn receives fully serialized frames from the real
// write pump (bytes actually flow to the syscall boundary) and that each
// dispatched action produces exactly one update frame.
func TestCompositeHarnessSerializes(t *testing.T) {
	s := startCounterSession(t)

	if got := s.conn.MsgsWritten(); got != 1 {
		t.Fatalf("expected exactly the initial render frame, got %d frames", got)
	}
	initialBytes := s.conn.BytesWritten()
	if initialBytes == 0 {
		t.Fatal("initial render reached WriteMessage with 0 bytes — harness is not serializing")
	}

	s.dispatch(t, incrementFrame)

	if got := s.conn.MsgsWritten(); got != 2 {
		t.Fatalf("expected 1 update frame after dispatch, got %d total frames", got)
	}
	if got := s.conn.BytesWritten(); got <= initialBytes {
		t.Fatalf("update frame added no bytes (total %d, initial %d)", got, initialBytes)
	}
}
