package livetemplate

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// These tests pin the standard-library integration surface: a LiveHandler is an
// ordinary net/http handler, so it must work through http.HandleFunc, Go 1.22
// method patterns, http.StripPrefix and middleware — including the WebSocket
// upgrade, which is the only part of the request lifecycle stdlib wrapping can
// break.

type stdlibState struct {
	Count int `lvt:"persist"`
}

type stdlibController struct{}

func (c *stdlibController) Mount(state stdlibState, ctx *Context) (stdlibState, error) {
	return state, nil
}

func (c *stdlibController) Increment(state stdlibState, ctx *Context) (stdlibState, error) {
	state.Count++
	return state, nil
}

func newStdlibHandler(t *testing.T) LiveHandler {
	t.Helper()
	tmpl := Must(Must(New("stdlib")).Parse(
		`<div><span id="count">{{.Count}}</span><form method="POST"><button name="lvt-action" value="Increment">+</button></form></div>`))
	return tmpl.Handle(&stdlibController{}, AsState(&stdlibState{}))
}

// stdlibClient keeps cookies across requests so the session group (and with it
// the persisted Count) survives the POST → GET hop.
func stdlibClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func getBody(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	resp, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	return okBody(t, resp, "GET "+target)
}

// postAction submits a form action the way a browser without JavaScript does,
// so the assertion covers the real POST dispatch path rather than the JSON one.
func postAction(t *testing.T, client *http.Client, target, action string) string {
	t.Helper()
	form := url.Values{"lvt-action": {action}}
	resp, err := client.Post(target, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	return okBody(t, resp, "POST "+target)
}

func okBody(t *testing.T, resp *http.Response, what string) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d, want 200", what, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", what, err)
	}
	return string(body)
}

func assertCount(t *testing.T, body, want string) {
	t.Helper()
	if marker := `<span id="count">` + want + `</span>`; !strings.Contains(body, marker) {
		t.Errorf("rendered body missing %s\ngot: %s", marker, body)
	}
}

// TestLiveHandler_Func_UsableWithStdlibHandleFunc is the issue's literal ask:
// the value Handle() returns reaches http.HandleFunc, which takes a function
// rather than an http.Handler. ServeMux.HandleFunc takes the same argument type
// as the package-level http.HandleFunc, without mutating DefaultServeMux.
func TestLiveHandler_Func_UsableWithStdlibHandleFunc(t *testing.T) {
	handler := newStdlibHandler(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/counter", handler.Func())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	assertCount(t, getBody(t, stdlibClient(t), srv.URL+"/counter"), "0")
}

// TestLiveHandler_Func_SplitAcrossMethodPatterns covers the Go 1.22 routing
// shape that a stdlib user is most likely to reach for. LiveTemplate routes the
// initial render (GET) and form actions (POST) through one ServeHTTP, so
// registering the same Func() under two method patterns has to keep dispatch
// working rather than serving a bare re-render on POST.
func TestLiveHandler_Func_SplitAcrossMethodPatterns(t *testing.T) {
	handler := newStdlibHandler(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /counter", handler.Func())
	mux.HandleFunc("POST /counter", handler.Func())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := stdlibClient(t)
	assertCount(t, getBody(t, client, srv.URL+"/counter"), "0")
	assertCount(t, postAction(t, client, srv.URL+"/counter", "Increment"), "1")
}

// TestLiveHandler_Func_KeepsLifecycleMethods guards the reason Handle() returns
// LiveHandler instead of a bare http.HandlerFunc: Func() is an accessor, not a
// downgrade, so the lifecycle methods stay reachable on the same value while it
// serves through the mux. Shutdown is left to shutdown_test.go, which owns the
// teardown ordering against a live server.
func TestLiveHandler_Func_KeepsLifecycleMethods(t *testing.T) {
	handler := newStdlibHandler(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/counter", handler.Func())
	mux.Handle("/metrics", handler.MetricsHandler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := stdlibClient(t)
	assertCount(t, getBody(t, client, srv.URL+"/counter"), "0")

	if metrics := getBody(t, client, srv.URL+"/metrics"); !strings.Contains(metrics, "livetemplate_templates_executed_total") {
		t.Errorf("metrics endpoint missing livetemplate_templates_executed_total:\n%s", metrics)
	}

	// No subscribers here, so this asserts reachability rather than delivery —
	// fan-out itself is covered by the topic tests.
	if err := handler.Publish("announcements", "Increment", nil); err != nil {
		t.Errorf("Publish after Func(): %v", err)
	}
}

// TestLiveHandler_ServesBehindStripPrefix checks the handler tolerates being
// mounted under a subtree: it tracks the last served path per session to detect
// URL changes, and StripPrefix rewrites exactly that.
func TestLiveHandler_ServesBehindStripPrefix(t *testing.T) {
	handler := newStdlibHandler(t)

	mux := http.NewServeMux()
	mux.Handle("/app/", http.StripPrefix("/app", handler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := stdlibClient(t)
	assertCount(t, getBody(t, client, srv.URL+"/app/"), "0")
	assertCount(t, postAction(t, client, srv.URL+"/app/", "Increment"), "1")
}

// passThroughMiddleware is the well-behaved shape: it observes the request but
// hands the original ResponseWriter down, so http.Hijacker survives.
func passThroughMiddleware(next http.Handler, seen *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen++
		next.ServeHTTP(w, r)
	})
}

// nonHijackableWriter is the shape that breaks upgrades: embedding
// http.ResponseWriter promotes Write/Header/WriteHeader but NOT Hijack, which
// is what a logging or gzip middleware typically ends up doing.
type nonHijackableWriter struct{ http.ResponseWriter }

func wsDialURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

// TestLiveHandler_WebSocketUpgrade_ThroughPassThroughMiddleware is the positive
// composition case: wrapped in stdlib middleware and registered via HandleFunc,
// the handler still upgrades and delivers the initial render.
func TestLiveHandler_WebSocketUpgrade_ThroughPassThroughMiddleware(t *testing.T) {
	handler := newStdlibHandler(t)

	calls := 0
	mux := http.NewServeMux()
	mux.Handle("/counter", passThroughMiddleware(handler.Func(), &calls))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ws, _, err := websocket.DefaultDialer.Dial(wsDialURL(srv, "/counter"), nil)
	if err != nil {
		t.Fatalf("WebSocket dial through middleware failed: %v", err)
	}
	defer func() { _ = ws.Close() }()

	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read initial render: %v", err)
	}
	if !strings.Contains(string(msg), `id=\"count\"`) {
		t.Errorf("initial render over WS missing rendered template: %s", msg)
	}
	if calls == 0 {
		t.Error("middleware was bypassed")
	}
}

// TestLiveHandler_WebSocketUpgrade_WriterWrappingMiddlewareIsDiagnosed pins the
// one stdlib composition that cannot work, and the log line that says why. The
// upgrade is refused cleanly (no panic) and the hint names middleware as the
// cause, because the upgrader's own error mentions only http.Hijacker.
//
// Note: this test mutates the global slog.Default() via SetDefault, so it must
// not run in parallel with tests that assert on log output.
func TestLiveHandler_WebSocketUpgrade_WriterWrappingMiddlewareIsDiagnosed(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	handler := newStdlibHandler(t)
	stripHijacker := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(nonHijackableWriter{w}, r)
		})
	}
	srv := httptest.NewServer(stripHijacker(handler))
	defer srv.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsDialURL(srv, "/"), nil)
	if err == nil {
		t.Fatal("expected the upgrade to fail behind a writer-wrapping middleware")
	}
	if resp == nil {
		t.Fatalf("expected an HTTP response describing the failure, got dial error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("upgrade status = %d, want 500", resp.StatusCode)
	}

	// Assert on the hint's own wording, not on "http.Hijacker" — the upgrader's
	// error already carries that phrase, so matching it would pass even if the
	// hint were dropped.
	got := logs.String()
	if !strings.Contains(got, "WebSocket upgrade failed") {
		t.Errorf("upgrade failure was not logged:\n%s", got)
	}
	if !strings.Contains(got, "middleware in front of this handler is wrapping the writer") {
		t.Errorf("log does not name middleware as the cause:\n%s", got)
	}

	// GET still renders — the failure is scoped to the upgrade, which is what
	// makes it easy to miss without the hint.
	assertCount(t, getBody(t, stdlibClient(t), srv.URL+"/"), "0")
}
