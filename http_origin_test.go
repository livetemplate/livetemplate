package livetemplate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type originCountController struct{ bumps atomic.Int32 }

type originCountState struct{ N int }

func (c *originCountController) Mount(s originCountState, ctx *Context) (originCountState, error) {
	return s, nil
}

func (c *originCountController) Bump(s originCountState, ctx *Context) (originCountState, error) {
	c.bumps.Add(1)
	s.N++
	return s, nil
}

// TestHTTPActionOriginCheck: an action sent over plain HTTP gets the same
// origin check as the WebSocket handshake. The request is the shape a page on
// another origin can send without a CORS preflight: a no-cors POST with a
// text/plain JSON body.
func TestHTTPActionOriginCheck(t *testing.T) {
	onlyProxy := func(r *http.Request) bool { return r.Header.Get("Origin") == "https://proxy.example" }
	cases := []struct {
		name    string
		opts    []Option
		origin  string // "" sends no Origin header
		allowed bool
	}{
		{name: "another origin is refused", origin: "http://evil.example"},
		{name: "same origin runs", origin: "http://example.com", allowed: true},
		{name: "no Origin (a non-browser client) runs", origin: "", allowed: true},
		{name: "a listed origin runs", opts: []Option{WithAllowedOrigins([]string{"https://proxy.example"})}, origin: "https://proxy.example", allowed: true},
		{name: "an unlisted origin is refused when a list is set", opts: []Option{WithAllowedOrigins([]string{"https://proxy.example"})}, origin: "http://evil.example"},
		{name: "DevMode allows any origin", opts: []Option{WithDevMode(true)}, origin: "http://evil.example", allowed: true},
		{name: "DevMode allows any origin with a list set too", opts: []Option{WithDevMode(true), WithAllowedOrigins([]string{"https://proxy.example"})}, origin: "http://evil.example", allowed: true},
		{name: "WithPermissiveOriginCheck allows any origin", opts: []Option{WithPermissiveOriginCheck()}, origin: "http://evil.example", allowed: true},
		{name: "WithPermissiveOriginCheck wins over a list", opts: []Option{WithPermissiveOriginCheck(), WithAllowedOrigins([]string{"https://proxy.example"})}, origin: "http://evil.example", allowed: true},
		{name: "the app's own Gorilla check governs HTTP too", opts: []Option{WithUpgrader(NewGorillaUpgrader(WithGorillaCheckOrigin(onlyProxy)))}, origin: "http://example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl, err := New("origin-check", c.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if tmpl, err = tmpl.Parse(`<div>{{.N}}</div>`); err != nil {
				t.Fatal(err)
			}
			ctrl := &originCountController{}
			handler := tmpl.Handle(ctrl, AsState(&originCountState{}))

			req := httptest.NewRequest(http.MethodPost, "http://example.com/", strings.NewReader(`{"action":"bump"}`))
			req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if ran := ctrl.bumps.Load() > 0; ran != c.allowed {
				t.Errorf("action ran = %v, want %v (status %d, body %q)", ran, c.allowed, rec.Code, rec.Body.String())
			}
			if !c.allowed && rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403", rec.Code)
			}
		})
	}
}

// TestSharedUpgraderKeepsPolicyPerTemplate: two templates built on one upgrader
// with different allow-lists each keep their own policy, on the socket and HTTP.
func TestSharedUpgraderKeepsPolicyPerTemplate(t *testing.T) {
	shared := NewGorillaUpgrader()
	build := func(origin string) (http.Handler, *originCountController) {
		tmpl, err := New("shared-"+origin, WithUpgrader(shared), WithAllowedOrigins([]string{origin}))
		if err != nil {
			t.Fatal(err)
		}
		if tmpl, err = tmpl.Parse(`<div>{{.N}}</div>`); err != nil {
			t.Fatal(err)
		}
		ctrl := &originCountController{}
		return tmpl.Handle(ctrl, AsState(&originCountState{})), ctrl
	}
	a, actrl := build("https://a.example")
	b, bctrl := build("https://b.example")
	post := func(h http.Handler, origin string) {
		req := httptest.NewRequest(http.MethodPost, "http://example.com/", strings.NewReader(`{"action":"bump"}`))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Origin", origin)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	post(a, "https://a.example")
	post(b, "https://a.example")
	if actrl.bumps.Load() != 1 || bctrl.bumps.Load() != 0 {
		t.Errorf("a ran %d (want 1), b ran %d (want 0): templates on one upgrader share a policy", actrl.bumps.Load(), bctrl.bumps.Load())
	}
}

// TestHTTPGetSkipsOriginCheck: a GET renders the page whatever its Origin, as a
// cross-site link or image may; only requests that can dispatch an action are
// checked.
func TestHTTPGetSkipsOriginCheck(t *testing.T) {
	tmpl, err := New("origin-get")
	if err != nil {
		t.Fatal(err)
	}
	if tmpl, err = tmpl.Parse(`<div>{{.N}}</div>`); err != nil {
		t.Fatal(err)
	}
	handler := tmpl.Handle(&originCountController{}, AsState(&originCountState{}))
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET status = %d, want 200", rec.Code)
	}
}
