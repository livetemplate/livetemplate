package livetemplate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/testutil"
	"github.com/livetemplate/livetemplate/pubsub"
)

// ============================================================================
// Task 7: New Handle() Signature Tests
// ============================================================================

type testHandleState struct {
	Count   int `lvt:"persist"`
	Message string
}

// testEphemeralState is like testHandleState but without persist tags.
// Used by ephemeral tests that verify state does NOT survive reconnection.
type testEphemeralState struct {
	Count   int
	Message string
}

type testEphemeralController struct{}

func (c *testEphemeralController) Mount(state testEphemeralState, ctx *Context) (testEphemeralState, error) {
	state.Message = "mounted"
	return state, nil
}

func (c *testEphemeralController) Increment(state testEphemeralState, ctx *Context) (testEphemeralState, error) {
	state.Count++
	return state, nil
}

type testHandleController struct {
	MountCalled bool
}

func (c *testHandleController) Mount(state testHandleState, ctx *Context) (testHandleState, error) {
	c.MountCalled = true
	state.Message = "mounted"
	return state, nil
}

func (c *testHandleController) Increment(state testHandleState, ctx *Context) (testHandleState, error) {
	state.Count++
	return state, nil
}

func TestHandle_ReturnsLiveHandler(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{})

	handler := tmpl.Handle(ctrl, state)

	if handler == nil {
		t.Fatal("Handle returned nil")
	}

	// Should implement http.Handler
	var _ http.Handler = handler
}

func TestHandle_PanicsOnNilController(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>test</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil controller")
		}
	}()

	tmpl.Handle(nil, AsState(&testHandleState{}))
}

func TestHandle_PanicsOnNilState(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>test</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil state")
		}
	}()

	tmpl.Handle(&testHandleController{}, nil)
}

func TestHandle_ServesHTTP(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	state := AsState(&testHandleState{Count: 42})

	handler := tmpl.Handle(ctrl, state)

	// Make HTTP request
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("Response body should not be empty")
	}
}

// Test that state is properly cloned per session (via serialization)
func TestHandle_StateCloning(t *testing.T) {
	// This test verifies that the State interface's serialization
	// is used for cloning, not struct copying
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	originalState := &testHandleState{Count: 100}
	state := AsState(originalState)

	handler := tmpl.Handle(&testHandleController{}, state)

	// Handler should exist
	if handler == nil {
		t.Fatal("Handler is nil")
	}

	// Original state should not be modified by creating handler
	if originalState.Count != 100 {
		t.Errorf("Original state was modified: Count = %d", originalState.Count)
	}
}

// ============================================================================
// wantsJSON Tests
// ============================================================================

func TestWantsJSON(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{"explicit json", "application/json", true},
		{"json with charset", "application/json; charset=utf-8", true},
		{"json subtype", "application/vnd.api+json", true},
		{"text/html only", "text/html", false},
		{"empty accept", "", false},
		{"xml", "application/xml", false},
		{"wildcard only", "*/*", false},
		// Browser-like headers where HTML is preferred
		{"browser html first", "text/html, application/json", false},
		{"browser html with quality", "text/html;q=0.9, application/json;q=0.8", false},
		{"browser complex", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", false},
		// JS client headers where JSON is preferred
		{"js client json first", "application/json, text/html", true},
		{"js client json only", "application/json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if got := wantsJSON(req); got != tt.want {
				t.Errorf("wantsJSON() = %v, want %v for Accept=%q", got, tt.want, tt.accept)
			}
		})
	}
}

// ============================================================================
// Progressive Enhancement Tests
// ============================================================================

type peTestState struct {
	Title string
	Items []string
}

type peTestController struct{}

func (c *peTestController) Mount(state peTestState, ctx *Context) (peTestState, error) {
	return state, nil
}

func (c *peTestController) Add(state peTestState, ctx *Context) (peTestState, error) {
	title := ctx.GetString("title")
	if title == "" {
		// Return a FieldError to trigger validation error handling
		return state, NewFieldError("title", errors.New("Title is required"))
	}
	state.Items = append(state.Items, title)
	ctx.SetFlash("success", "Added: "+title)
	return state, nil
}

func TestProgressiveEnhancement_NonJSFormWithErrors(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>
		{{if .lvt.HasError "title"}}<span class="error">{{.lvt.Error "title"}}</span>{{end}}
		<form method="POST"><input name="title"></form>
	</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&peTestController{}, AsState(&peTestState{}))

	// POST without Accept: application/json (browser behavior)
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "") // Empty title should cause validation error

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 200 with HTML (not redirect)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	// Should contain error message inline
	body := rec.Body.String()
	if !strings.Contains(body, "Title is required") {
		t.Errorf("Expected error message in body, got: %s", body)
	}
}

func TestProgressiveEnhancement_NonJSFormSuccess(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{range .Items}}<p>{{.}}</p>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&peTestController{}, AsState(&peTestState{}))

	// POST without Accept: application/json
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "Test item")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 303 redirect (PRG pattern)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", rec.Code)
	}

	// Location should be clean (no flash params in URL)
	location := rec.Header().Get("Location")
	if strings.Contains(location, "success=") {
		t.Errorf("Flash message should NOT be in redirect URL, got: %s", location)
	}

	// Flash should be in Set-Cookie header
	flashCookie := extractFlashCookie(rec)
	if flashCookie == nil {
		t.Fatal("Expected lvt-flash cookie to be set")
	}
	if !strings.Contains(flashCookie.Value, "success=") {
		t.Errorf("Expected 'success=' in flash cookie value, got: %s", flashCookie.Value)
	}
}

func TestProgressiveEnhancement_JSClientStillGetsJSON(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{range .Items}}<p>{{.}}</p>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&peTestController{}, AsState(&peTestState{}))

	// POST with Accept: application/json (JS client behavior)
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "Test item")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json") // JS client
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 200 with JSON (not redirect)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestProgressiveEnhancement_Disabled(t *testing.T) {
	tmpl, err := New("test", WithProgressiveEnhancement(false))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{range .Items}}<p>{{.}}</p>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&peTestController{}, AsState(&peTestState{}))

	// POST without Accept: application/json
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "Test item")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html") // Non-JS client
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 200 with JSON (progressive enhancement disabled)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Should return JSON even for non-JS client when disabled
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json when PE disabled, got %s", contentType)
	}
}

func TestProgressiveEnhancement_FlashCookieConsumed(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>{{if .lvt.HasFlash "success"}}<p class="flash">{{.lvt.Flash "success"}}</p>{{end}}{{range .Items}}<p>{{.}}</p>{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&peTestController{}, AsState(&peTestState{}))

	// Step 1: POST to trigger action and get flash cookie
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "Cookie test")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 redirect, got %d", rec.Code)
	}

	sessionCookie := extractSessionCookie(rec)
	flashCookie := extractFlashCookie(rec)
	if flashCookie == nil {
		t.Fatal("Expected lvt-flash cookie in POST response")
	}

	// Step 2: Follow redirect GET with flash cookie
	req = httptest.NewRequest("GET", rec.Header().Get("Location"), nil)
	if sessionCookie != nil {
		req.AddCookie(sessionCookie)
	}
	req.AddCookie(flashCookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Added: Cookie test") {
		t.Errorf("Expected flash message in rendered HTML, got: %s", body)
	}

	// Step 3: Verify cookie was cleared (MaxAge=-1)
	clearCookie := extractFlashCookie(rec)
	if clearCookie == nil {
		t.Fatal("Expected lvt-flash cookie to be cleared in GET response")
	}
	if clearCookie.MaxAge != -1 {
		t.Errorf("Expected MaxAge=-1 to clear cookie, got %d", clearCookie.MaxAge)
	}
}

// ============================================================================
// WebSocket Disabled Mode Tests
// ============================================================================

type wsDisabledState struct {
	Count   int `lvt:"persist"`
	Items   []string
	Message string
}

type wsDisabledController struct{}

func (c *wsDisabledController) Mount(state wsDisabledState, ctx *Context) (wsDisabledState, error) {
	state.Message = "mounted"
	return state, nil
}

func (c *wsDisabledController) Add(state wsDisabledState, ctx *Context) (wsDisabledState, error) {
	title := ctx.GetString("title")
	if title == "" {
		return state, NewFieldError("title", errors.New("Title is required"))
	}
	state.Items = append(state.Items, title)
	state.Count++
	ctx.SetFlash("success", "Added: "+title)
	return state, nil
}

func (c *wsDisabledController) Increment(state wsDisabledState, ctx *Context) (wsDisabledState, error) {
	state.Count++
	return state, nil
}

func extractSessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "livetemplate-id" {
			return c
		}
	}
	return nil
}

func extractFlashCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "lvt-flash" {
			return c
		}
	}
	return nil
}

func newWSDisabledHandler(t *testing.T, opts ...Option) LiveHandler {
	t.Helper()
	baseOpts := []Option{WithWebSocketDisabled()}
	baseOpts = append(baseOpts, opts...)
	tmpl, err := New("test", baseOpts...)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>Count: {{.Count}}{{if .Message}} Message: {{.Message}}{{end}}{{range .Items}} Item: {{.}}{{end}}{{if .lvt.HasError "title"}} Error: {{.lvt.Error "title"}}{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{}))
}

func TestWebSocketDisabled_ResponseHeader(t *testing.T) {
	tests := []struct {
		name           string
		wsDisabled     bool
		method         string
		expectedHeader string
	}{
		{"enabled", false, "GET", "enabled"},
		{"disabled GET", true, "GET", "disabled"},
		{"disabled POST", true, "POST", "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.wsDisabled {
				opts = append(opts, WithWebSocketDisabled())
			}
			tmpl, err := New("test", opts...)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}
			tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			handler := tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{}))

			var req *http.Request
			if tt.method == "POST" {
				form := url.Values{}
				form.Set("lvt-action", "Increment")
				req = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("Accept", "application/json")
			} else {
				req = httptest.NewRequest("GET", "/", nil)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			got := rec.Header().Get("X-LiveTemplate-WebSocket")
			if got != tt.expectedHeader {
				t.Errorf("X-LiveTemplate-WebSocket = %q, want %q", got, tt.expectedHeader)
			}
		})
	}
}

func TestWebSocketDisabled_UpgradeRejected(t *testing.T) {
	handler := newWSDisabledHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "WebSocket is disabled") {
		t.Errorf("Expected 'WebSocket is disabled' in body, got: %s", body)
	}
}

func TestWebSocketDisabled_GETRendersHTML(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	handler := tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{Count: 42}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Count: 42") {
		t.Errorf("Expected 'Count: 42' in body, got: %s", body)
	}
	if !strings.Contains(body, "<div") {
		t.Errorf("Expected HTML content, got: %s", body)
	}
}

func TestWebSocketDisabled_GETCallsMount(t *testing.T) {
	handler := newWSDisabledHandler(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Message: mounted") {
		t.Errorf("Expected 'Message: mounted' in body (Mount should be called), got: %s", body)
	}
}

func TestWebSocketDisabled_POSTActionSuccess(t *testing.T) {
	handler := newWSDisabledHandler(t)

	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "Test item")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", rec.Code)
	}

	// Location should be clean (no flash params)
	location := rec.Header().Get("Location")
	if strings.Contains(location, "success=") {
		t.Errorf("Flash message should NOT be in redirect URL, got: %s", location)
	}

	// Flash should be in cookie
	flashCookie := extractFlashCookie(rec)
	if flashCookie == nil {
		t.Fatal("Expected lvt-flash cookie to be set")
	}
	if !strings.Contains(flashCookie.Value, "success=") {
		t.Errorf("Expected 'success=' in flash cookie value, got: %s", flashCookie.Value)
	}
}

func TestWebSocketDisabled_POSTValidationError(t *testing.T) {
	handler := newWSDisabledHandler(t)

	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Title is required") {
		t.Errorf("Expected 'Title is required' in body, got: %s", body)
	}
}

func TestWebSocketDisabled_POSTReturnsJSONForJSClient(t *testing.T) {
	handler := newWSDisabledHandler(t)

	form := url.Values{}
	form.Set("lvt-action", "Increment")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if _, ok := response["tree"]; !ok {
		t.Error("Expected 'tree' key in JSON response")
	}
}

func TestWebSocketDisabled_StatePersistsAcrossRequests(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET to create session
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie to be set")
	}

	// Step 2: POST with cookie (Increment)
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 3: GET with same cookie to verify state persists
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Count: 1") {
		t.Errorf("Expected 'Count: 1' after increment, got: %s", body)
	}
}

func TestWebSocketDisabled_SessionIsolation(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Session A: GET to create session
	reqA := httptest.NewRequest("GET", "/", nil)
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	cookieA := extractSessionCookie(recA)
	if cookieA == nil {
		t.Fatal("Expected session cookie for session A")
	}

	// Session A: POST Increment
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	reqA = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	reqA.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqA.Header.Set("Accept", "text/html")
	reqA.AddCookie(cookieA)
	recA = httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)

	// Session B: GET (no cookie → new session)
	reqB := httptest.NewRequest("GET", "/", nil)
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	bodyB := recB.Body.String()
	if !strings.Contains(bodyB, "Count: 0") {
		t.Errorf("Session B should have Count: 0, got: %s", bodyB)
	}

	// Session A: GET to verify count is still 1
	reqA = httptest.NewRequest("GET", "/", nil)
	reqA.AddCookie(cookieA)
	recA = httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)

	bodyA := recA.Body.String()
	if !strings.Contains(bodyA, "Count: 1") {
		t.Errorf("Session A should have Count: 1, got: %s", bodyA)
	}
}

func TestWebSocketDisabled_WithPEDisabled(t *testing.T) {
	handler := newWSDisabledHandler(t, WithProgressiveEnhancement(false))

	form := url.Values{}
	form.Set("lvt-action", "Increment")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json when PE disabled, got %s", contentType)
	}
}

func TestWebSocketDisabled_DiffOptimization(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET to create session
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie to be set")
	}

	// Step 2: First POST — should include statics ("s" key) since it's the
	// first time the cached HTTP template renders via ExecuteUpdates
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("First POST: expected status 200, got %d", rec.Code)
	}

	var firstResp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&firstResp); err != nil {
		t.Fatalf("First POST: failed to decode JSON: %v", err)
	}

	firstTree, ok := firstResp["tree"].(map[string]interface{})
	if !ok {
		t.Fatal("First POST: expected 'tree' to be an object")
	}

	if !hasStaticsInTree(firstTree) {
		t.Error("First POST: expected statics ('s' key) in tree for initial render")
	}

	// Step 3: Second POST — should NOT include statics (diff only)
	form = url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Second POST: expected status 200, got %d", rec.Code)
	}

	var secondResp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&secondResp); err != nil {
		t.Fatalf("Second POST: failed to decode JSON: %v", err)
	}

	secondTree, ok := secondResp["tree"].(map[string]interface{})
	if !ok {
		t.Fatal("Second POST: expected 'tree' to be an object")
	}

	if hasStaticsInTree(secondTree) {
		t.Error("Second POST: expected NO statics in tree (diff optimization); got statics — this means the HTTP template cache is not working")
	}
}

func TestWebSocketDisabled_GETReturnsJSONForJSClient(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// GET HTML first to create session
	getReq := httptest.NewRequest("GET", "/", nil)
	getReq.Header.Set("Accept", "text/html")
	getW := httptest.NewRecorder()
	handler.ServeHTTP(getW, getReq)
	cookie := extractSessionCookie(getW)

	// GET with Accept: application/json (like the JS client in HTTP mode)
	jsonReq := httptest.NewRequest("GET", "/", nil)
	jsonReq.Header.Set("Accept", "application/json")
	if cookie != nil {
		jsonReq.AddCookie(cookie)
	}
	jsonW := httptest.NewRecorder()
	handler.ServeHTTP(jsonW, jsonReq)

	if jsonW.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", jsonW.Code)
	}

	ct := jsonW.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("Expected Content-Type application/json, got %q", ct)
	}

	var resp struct {
		Tree map[string]any `json:"tree"`
		Meta *struct {
			Success bool `json:"success"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(jsonW.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal JSON response: %v", err)
	}

	if resp.Tree == nil {
		t.Fatal("Expected tree in JSON response")
	}
	if resp.Meta == nil || !resp.Meta.Success {
		t.Fatal("Expected meta.success=true in JSON response")
	}
}

func TestWebSocketDisabled_MultiTabDiffCorrectness(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Create a session via GET
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Tab A: POST Increment (first action — response includes statics)
	postWithCookie := func(action string) *httptest.ResponseRecorder {
		form := url.Values{}
		form.Set("lvt-action", action)
		r := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	recA1 := postWithCookie("Increment")
	if recA1.Code != http.StatusOK {
		t.Fatalf("Tab A first POST: expected 200, got %d", recA1.Code)
	}

	// Tab B: POST Increment (same session, simulating second tab)
	// Should still return a valid tree (diff against Tab A's last state)
	recB1 := postWithCookie("Increment")
	if recB1.Code != http.StatusOK {
		t.Fatalf("Tab B POST: expected 200, got %d", recB1.Code)
	}

	var respB map[string]interface{}
	if err := json.NewDecoder(recB1.Body).Decode(&respB); err != nil {
		t.Fatalf("Tab B: failed to decode JSON: %v", err)
	}

	tree, ok := respB["tree"].(map[string]interface{})
	if !ok {
		t.Fatal("Tab B: expected 'tree' object in response")
	}

	// The response must be valid JSON with a tree — regardless of whether
	// it contains statics or not. In HTTP mode, tabs share diff state via
	// a per-groupID cache. The serialized mutex ensures no data races.
	// The client handles both full trees and diffs correctly.
	if len(tree) == 0 {
		t.Error("Tab B: expected non-empty tree")
	}

	meta, ok := respB["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Tab B: expected 'meta' object in response")
	}
	if success, _ := meta["success"].(bool); !success {
		t.Error("Tab B: expected meta.success=true")
	}

	// Tab A: POST again — should still produce a valid diff
	recA2 := postWithCookie("Increment")
	if recA2.Code != http.StatusOK {
		t.Fatalf("Tab A second POST: expected 200, got %d", recA2.Code)
	}

	var respA2 map[string]interface{}
	if err := json.NewDecoder(recA2.Body).Decode(&respA2); err != nil {
		t.Fatalf("Tab A second POST: failed to decode JSON: %v", err)
	}
	if _, ok := respA2["tree"].(map[string]interface{}); !ok {
		t.Fatal("Tab A second POST: expected 'tree' object")
	}
}

// ============================================================================
// HTTP Template Cache Sweep Tests
// ============================================================================

func TestHTTPTemplateSweep_CleansStaleEntries(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	store := NewMemorySessionStore()
	handler := tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{}), WithStore(store))

	lh := handler.(*liveHandler)

	// Step 1: GET to create session
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Step 2: POST to populate httpTemplates cache
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify cache entry exists
	if _, ok := lh.httpTemplates.Load(cookie.Value); !ok {
		t.Fatal("Expected httpTemplates cache entry after POST")
	}

	// Step 3: Delete session from store (simulates expiry)
	store.Delete(t.Context(), cookie.Value)

	// Step 4: Run sweep
	lh.sweepStaleHTTPTemplates()

	// Step 5: Verify cache entry was cleaned up
	if _, ok := lh.httpTemplates.Load(cookie.Value); ok {
		t.Error("Expected httpTemplates cache entry to be swept after session deletion")
	}
}

func TestHTTPTemplateSweep_PreservesActiveSessions(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	store := NewMemorySessionStore()
	handler := tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{}), WithStore(store))

	lh := handler.(*liveHandler)

	// Create two sessions
	req1 := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	cookie1 := extractSessionCookie(rec1)

	req2 := httptest.NewRequest("GET", "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	cookie2 := extractSessionCookie(rec2)

	// POST on both to populate cache
	for _, cookie := range []*http.Cookie{cookie1, cookie2} {
		form := url.Values{}
		form.Set("lvt-action", "Increment")
		req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Delete only session 1
	store.Delete(t.Context(), cookie1.Value)

	// Run sweep
	lh.sweepStaleHTTPTemplates()

	// Session 1 should be swept
	if _, ok := lh.httpTemplates.Load(cookie1.Value); ok {
		t.Error("Expected session 1 cache entry to be swept")
	}

	// Session 2 should be preserved
	if _, ok := lh.httpTemplates.Load(cookie2.Value); !ok {
		t.Error("Expected session 2 cache entry to be preserved")
	}
}

func TestHTTPTemplateSweep_CleansOrphanedLastPaths(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	store := NewMemorySessionStore()
	handler := tmpl.Handle(&wsDisabledController{}, AsState(&wsDisabledState{}), WithStore(store))

	lh := handler.(*liveHandler)

	// Step 1: GET to create session (populates httpLastPaths but NOT httpTemplates)
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Verify httpLastPaths was populated
	if _, ok := lh.httpLastPaths.Load(cookie.Value); !ok {
		t.Fatal("Expected httpLastPaths entry after GET")
	}

	// Verify httpTemplates was NOT populated (GET-only session)
	if _, ok := lh.httpTemplates.Load(cookie.Value); ok {
		t.Fatal("Did not expect httpTemplates entry for GET-only session")
	}

	// Step 2: Delete session from store (simulates expiry)
	store.Delete(t.Context(), cookie.Value)

	// Step 3: Run sweep
	lh.sweepStaleHTTPTemplates()

	// Step 4: Verify orphaned httpLastPaths entry was cleaned up
	if _, ok := lh.httpLastPaths.Load(cookie.Value); ok {
		t.Error("Expected orphaned httpLastPaths entry to be swept after session deletion")
	}
}

func TestWebSocketDisabled_ConcurrentPOSTsSameSession(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Create session
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Send concurrent POSTs (simulates multiple tabs)
	const concurrency = 10
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			form := url.Values{}
			form.Set("lvt-action", "Increment")
			r := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Accept", "application/json")
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				errs <- errors.New("expected status 200, got " + http.StatusText(w.Code))
				return
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				errs <- err
				return
			}
			if _, ok := resp["tree"]; !ok {
				errs <- errors.New("expected 'tree' in response")
				return
			}
			errs <- nil
		}()
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Concurrent POST failed: %v", err)
		}
	}

	// Verify state advanced (exact count depends on scheduling — concurrent
	// POSTs load the same state, so last-writer-wins is expected for HTTP).
	// The key assertion is: no races, all responses valid, count > 0.
	req = httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "Count: 0") {
		t.Error("Expected count > 0 after concurrent increments, still at 0")
	}
}

func TestWebSocketDisabled_POSTToDifferentPathDoesNotResetState(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET /page-a — creates session
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Step 2: POST /page-a — increment count
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 3: POST to /page-a/action (different path) — should NOT reset state
	form = url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 4: GET /page-a — state should still reflect both increments (count=2)
	req = httptest.NewRequest("GET", "/page-a", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Count: 2") {
		t.Errorf("POST to different path should not reset state: expected 'Count: 2', got: %s", body)
	}
}

func TestWebSocketDisabled_ConcurrentPathChanges(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Create session via GET /page-a
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Increment count on /page-a
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Concurrent GETs to different paths — should not race
	const concurrency = 10
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		path := "/page-a"
		if i%2 == 1 {
			path = "/page-b"
		}
		go func(p string) {
			r := httptest.NewRequest("GET", p, nil)
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				errs <- errors.New("expected status 200 for " + p)
				return
			}
			errs <- nil
		}(path)
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Concurrent path change failed: %v", err)
		}
	}
}

func TestWebSocketDisabled_PathChangeResetsState(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET /page-a — creates session
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}
	if !strings.Contains(rec.Body.String(), "Count: 0") {
		t.Errorf("Expected 'Count: 0' on initial load, got: %s", rec.Body.String())
	}

	// Step 2: POST /page-a — increment count
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 3: GET /page-a — same path, state should persist
	req = httptest.NewRequest("GET", "/page-a", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Count: 1") {
		t.Errorf("Same path: expected 'Count: 1', got: %s", body)
	}

	// Step 4: GET /page-b — different path, state should reset (fresh Mount)
	req = httptest.NewRequest("GET", "/page-b", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body = rec.Body.String()
	if !strings.Contains(body, "Count: 0") {
		t.Errorf("Different path: expected 'Count: 0' (fresh state), got: %s", body)
	}
	if !strings.Contains(body, "Message: mounted") {
		t.Errorf("Different path: expected 'Message: mounted' (Mount called), got: %s", body)
	}
}

func TestWebSocketDisabled_TrailingSlashDoesNotResetState(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET /page-a — creates session
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Step 2: POST /page-a — increment count
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Step 3: GET /page-a/ (trailing slash) — same path after normalization,
	// state should persist and NOT reset.
	req = httptest.NewRequest("GET", "/page-a/", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Count: 1") {
		t.Errorf("Trailing slash: expected 'Count: 1' (same path), got: %s", body)
	}
}

func TestWebSocketDisabled_ErrorsDoNotLeakAcrossRoutes(t *testing.T) {
	handler := newWSDisabledHandler(t)

	// Step 1: GET /page-a — creates session, no errors
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Error:") {
		t.Errorf("Initial GET should have no errors, got: %s", body)
	}

	// Step 2: POST /page-a with empty title — triggers validation error
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "Error: Title is required") {
		t.Errorf("POST with empty title should show error, got: %s", body)
	}

	// Step 3: GET /page-a — same route, errors should be gone (fresh request)
	req = httptest.NewRequest("GET", "/page-a", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	body = rec.Body.String()
	if strings.Contains(body, "Error:") {
		t.Errorf("GET after POST error should have no errors, got: %s", body)
	}
	if !strings.Contains(body, "Count: 0") {
		t.Errorf("State should be intact after failed POST, got: %s", body)
	}

	// Step 4: GET /page-b — different route, no errors should leak
	req = httptest.NewRequest("GET", "/page-b", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	body = rec.Body.String()
	if strings.Contains(body, "Error:") {
		t.Errorf("GET on different route should have no errors, got: %s", body)
	}
	if !strings.Contains(body, "Count: 0") {
		t.Errorf("Different route should have fresh state, got: %s", body)
	}
	if !strings.Contains(body, "Message: mounted") {
		t.Errorf("Different route should call Mount, got: %s", body)
	}
}

func TestWebSocketDisabled_ErrorsDoNotLeakAcrossRoutes_JSONClient(t *testing.T) {
	type jsonResponse struct {
		Tree map[string]any `json:"tree"`
		Meta *struct {
			Success bool              `json:"success"`
			Errors  map[string]string `json:"errors"`
		} `json:"meta"`
	}

	handler := newWSDisabledHandler(t)

	// Step 1: GET /page-a (HTML first to create session before JSON requests)
	req := httptest.NewRequest("GET", "/page-a", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Step 2: POST /page-a with empty title (JSON) — should return errors in meta
	form := url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	var resp jsonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal JSON response: %v", err)
	}
	if resp.Meta == nil {
		t.Fatal("Expected meta in response")
	}
	if resp.Meta.Success {
		t.Error("Expected meta.success=false for validation error")
	}
	if resp.Meta.Errors["title"] != "Title is required" {
		t.Errorf("Expected title error, got errors: %v", resp.Meta.Errors)
	}

	// Step 3: POST /page-a with valid title (JSON) — errors should clear
	form = url.Values{}
	form.Set("lvt-action", "Add")
	form.Set("title", "hello")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	var resp2 jsonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("Failed to unmarshal JSON response: %v", err)
	}
	if resp2.Meta == nil {
		t.Fatal("Expected meta in response")
	}
	if !resp2.Meta.Success {
		t.Error("Expected meta.success=true after valid submission")
	}
	if len(resp2.Meta.Errors) > 0 {
		t.Errorf("Expected no errors after valid submission, got: %v", resp2.Meta.Errors)
	}

	// Step 4: GET /page-b (JSON) — no errors should leak to different route
	req = httptest.NewRequest("GET", "/page-b", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}
	var resp3 jsonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp3); err != nil {
		t.Fatalf("Failed to unmarshal JSON response: %v", err)
	}
	if resp3.Meta == nil {
		t.Fatal("Expected meta in response")
	}
	if !resp3.Meta.Success {
		t.Error("Expected meta.success=true on different route GET")
	}
	if len(resp3.Meta.Errors) > 0 {
		t.Errorf("Expected no errors on different route, got: %v", resp3.Meta.Errors)
	}
}

// failingMountController fails Mount on demand via a channel signal.
type failingMountController struct {
	failNext chan struct{}
}

func (c *failingMountController) Mount(state wsDisabledState, ctx *Context) (wsDisabledState, error) {
	select {
	case <-c.failNext:
		return state, errors.New("mount failed")
	default:
		state.Message = "mounted"
		return state, nil
	}
}

func (c *failingMountController) Increment(state wsDisabledState, ctx *Context) (wsDisabledState, error) {
	state.Count++
	return state, nil
}

func TestWebSocketDisabled_PathChangeMountFailureRetry(t *testing.T) {
	ctrl := &failingMountController{failNext: make(chan struct{}, 1)}

	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>Count: {{.Count}}{{if .Message}} Message: {{.Message}}{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	handler := tmpl.Handle(ctrl, AsState(&wsDisabledState{}))

	// Step 1: GET /page-a — creates session
	req := httptest.NewRequest("GET", "/page-a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	cookie := extractSessionCookie(rec)
	if cookie == nil {
		t.Fatal("Expected session cookie")
	}

	// Step 2: POST /page-a to increment count to 1
	// This ensures stale state differs from fresh state, so the retry
	// assertion can distinguish "Mount was called" from "used stale state".
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req = httptest.NewRequest("POST", "/page-a", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify count is 1
	req = httptest.NewRequest("GET", "/page-a", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Count: 1") {
		t.Fatalf("Expected Count: 1 after increment, got: %s", rec.Body.String())
	}

	// Step 3: Make next Mount fail, then GET /page-b (triggers path change)
	ctrl.failNext <- struct{}{}
	req = httptest.NewRequest("GET", "/page-b", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500 on Mount failure, got %d", rec.Code)
	}

	// Step 4: Retry GET /page-b — Mount should succeed this time because
	// httpLastPaths still holds "/page-a", allowing path-change re-detection.
	// If the rollback is broken, the retry would use stale state (Count: 1).
	req = httptest.NewRequest("GET", "/page-b", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 on retry, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Count: 0") {
		t.Errorf("Retry should have fresh state with Count: 0 (Mount called), got: %s", body)
	}
	if !strings.Contains(body, "Message: mounted") {
		t.Errorf("Retry should call Mount, got: %s", body)
	}
}

// ============================================================================
// Capabilities Detection Tests
// ============================================================================

type capState struct {
	Name string
}

type capControllerWithChange struct{}

func (c *capControllerWithChange) Change(state capState, ctx *Context) (capState, error) {
	return state, nil
}

type capControllerWithoutChange struct{}

func (c *capControllerWithoutChange) Submit(state capState, ctx *Context) (capState, error) {
	return state, nil
}

func TestHandle_CapabilitiesInInitialRender(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &capControllerWithChange{}
	state := AsState(&capState{Name: "test"})

	handler := tmpl.Handle(ctrl, state)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in response")
	}

	caps, ok := meta["capabilities"].([]interface{})
	if !ok {
		t.Fatal("Expected capabilities array in meta")
	}

	if len(caps) != 1 || caps[0] != "change" {
		t.Errorf("Expected capabilities=[\"change\"], got %v", caps)
	}
}

func TestHandle_NoCapabilitiesWithoutChangeMethod(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &capControllerWithoutChange{}
	state := AsState(&capState{Name: "test"})

	handler := tmpl.Handle(ctrl, state)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in response")
	}

	if _, exists := meta["capabilities"]; exists {
		t.Error("Expected capabilities to be omitted when controller has no Change() method")
	}
}

func TestHandle_CapabilitiesInWebSocketInitialRender(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &capControllerWithChange{}
	state := AsState(&capState{Name: "test"})

	handler := tmpl.Handle(ctrl, state)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	}()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse WebSocket response: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in WebSocket initial render")
	}

	caps, ok := meta["capabilities"].([]interface{})
	if !ok {
		t.Fatal("Expected capabilities array in WebSocket initial render meta")
	}

	if len(caps) != 1 || caps[0] != "change" {
		t.Errorf("Expected capabilities=[\"change\"], got %v", caps)
	}
}

// fixedGroupAuthHandle always returns the same groupID.
type fixedGroupAuthHandle struct {
	groupID string
}

func (a *fixedGroupAuthHandle) Identify(_ *http.Request) (string, error) { return "", nil }
func (a *fixedGroupAuthHandle) GetSessionGroup(_ *http.Request, _ string) (string, error) {
	return a.groupID, nil
}

// TestFaviconDoesNotResetState verifies that browser requests for /favicon.ico
// do not trigger pathChanged logic and wipe session state (#276).
func TestFaviconDoesNotResetState(t *testing.T) {
	auth := &fixedGroupAuthHandle{groupID: "favicon-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()

	// 1. GET / — establishes session
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}

	// 2. POST increment — updates state (Count becomes 1)
	form := url.Values{}
	form.Set("lvt-action", "increment")
	resp, err = http.Post(server.URL+"/", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}

	// 3. GET /favicon.ico — must NOT reset state
	resp, err = http.Get(server.URL + "/favicon.ico")
	if err != nil {
		t.Fatalf("GET /favicon.ico failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}

	// 4. GET / — should see incremented state, not fresh state
	resp, err = http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("body close: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	if !strings.Contains(string(body), ">1<") {
		t.Errorf("Expected state to be preserved (Count=1) after /favicon.ico, got body: %s", string(body))
	}
}

func TestHandle_NoCapabilitiesInWebSocketWithoutChangeMethod(t *testing.T) {
	tmpl, err := New("test")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Name}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &capControllerWithoutChange{}
	state := AsState(&capState{Name: "test"})

	handler := tmpl.Handle(ctrl, state)

	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	}()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse WebSocket response: %v", err)
	}

	meta, ok := resp["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in WebSocket initial render")
	}

	if _, exists := meta["capabilities"]; exists {
		t.Error("Expected capabilities to be omitted when controller has no Change() method")
	}
}

// ============================================================================
// Per-Connection State Persistence Tests (#289)
// ============================================================================

// reconnectWSRaw closes an existing connection, waits briefly for server-side
// cleanup, then connects a new one. Returns the new connection and its initial
// render message.
func reconnectWSRaw(t *testing.T, wsURL string, old *websocket.Conn) (*websocket.Conn, []byte) {
	t.Helper()
	if err := old.Close(); err != nil {
		t.Logf("old connection close: %v", err)
	}
	// Brief pause for server-side unregister to complete before reconnecting.
	time.Sleep(50 * time.Millisecond)
	return connectWSRaw(t, wsURL)
}

// connectWSRaw connects and returns the raw initial render message.
func connectWSRaw(t *testing.T, wsURL string) (*websocket.Conn, []byte) {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		if closeErr := ws.Close(); closeErr != nil {
			t.Logf("WebSocket close error: %v", closeErr)
		}
		t.Fatalf("Failed to read initial render: %v", err)
	}
	return ws, msg
}

// treeDynamic extracts a dynamic value by key from a WS response tree.
func treeDynamic(t *testing.T, raw []byte, key string) string {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Failed to parse WS response: %v", err)
	}
	tree, ok := resp["tree"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tree field in response, got: %s", string(raw))
	}
	val, ok := tree[key]
	if !ok {
		t.Fatalf("Expected key %q in tree, got: %v", key, tree)
	}
	return fmt.Sprintf("%v", val)
}

// TestPerConnectionState_WSActionPersistsToStore verifies that WebSocket action
// state changes in per-connection mode are persisted to SessionStore, so a page
// refresh (new WS connection) gets the updated state, not fresh initial state.
func TestPerConnectionState_WSActionPersistsToStore(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "persist-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// 1. Connect WS1 and verify initial render has Count: 0
	ws1, initialMsg := connectWSRaw(t, wsURL)
	if v := treeDynamic(t, initialMsg, "0"); v != "0" {
		t.Fatalf("Expected initial Count=0, got dynamic 0=%q", v)
	}

	// 2. Send increment action
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}

	// 3. Read action response — should reflect Count: 1
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, actionResp, err := ws1.ReadMessage()
	if err != nil {
		t.Fatalf("WS1 read action response failed: %v", err)
	}
	if v := treeDynamic(t, actionResp, "0"); v != "1" {
		t.Fatalf("Expected action response Count=1, got dynamic 0=%q", v)
	}

	// 4. Close WS1 and reconnect (simulates page refresh)
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close error: %v", err)
		}
	}()

	// 5. Verify the reconnected WS sees Count: 1, not Count: 0
	if v := treeDynamic(t, reconnectMsg, "0"); v != "1" {
		t.Errorf("State NOT persisted: expected Count=1 on reconnect, got dynamic 0=%q. Full response: %s", v, string(reconnectMsg))
	}
}

// TestPerConnectionState_ActionPersistsAcrossReconnect verifies that state changes
// from WS actions are persisted, so reconnection sees updated state.
// Uses Count (not Message) because Mount() overwrites Message on every connect.
func TestPerConnectionState_ActionPersistsAcrossReconnect(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "dispatch-persist-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}} - {{.Message}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// 1. Connect and increment
	ws1, _ := connectWSRaw(t, wsURL)
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, actionResp, err := ws1.ReadMessage()
	if err != nil {
		t.Fatalf("WS1 read failed: %v", err)
	}
	if v := treeDynamic(t, actionResp, "0"); v != "1" {
		t.Fatalf("Expected Count=1 in action response, got %q", v)
	}

	// 2. Reconnect — persisted state should survive (Count=1)
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	if v := treeDynamic(t, reconnectMsg, "0"); v != "1" {
		t.Errorf("Action state NOT persisted: expected Count=1, got %q. Full: %s", v, string(reconnectMsg))
	}
}

// TestPerConnectionState_NoAutoBroadcast verifies that without a Sync() method,
// WS2 does NOT receive an automatic update when WS1 performs an action.
func TestPerConnectionState_NoAutoBroadcast(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "no-autobroadcast-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Connect two WS clients
	ws1 := connectWS(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()
	ws2 := connectWS(t, wsURL)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	// WS1 sends increment action
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}

	// WS1 should receive its own action response
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err = ws1.ReadMessage()
	if err != nil {
		t.Fatalf("WS1 should have received action response: %v", err)
	}

	// WS2 should NOT receive any auto-broadcast update
	if err := ws2.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, _, err = ws2.ReadMessage()
	if err == nil {
		t.Error("WS2 should NOT receive auto-broadcast in per-connection mode")
	}
}

// fixedUserAuth returns a fixed groupID and userID for server action tests.
type fixedUserAuth struct {
	groupID string
	userID  string
}

func (a *fixedUserAuth) Identify(_ *http.Request) (string, error) { return a.userID, nil }
func (a *fixedUserAuth) GetSessionGroup(_ *http.Request, _ string) (string, error) {
	return a.groupID, nil
}

// TestPerConnectionState_ServerActionPersists verifies that server-initiated
// actions (via PubSub/TriggerAction) persist state in per-connection mode.
func TestPerConnectionState_ServerActionPersists(t *testing.T) {
	testutil.SkipIfNoDocker(t)

	client := testutil.GetTestRedisClient(t)

	// Two separate broadcasters: one subscribes (handler), one publishes.
	// Same Redis, different instance IDs (RedisBroadcaster skips own-instance messages).
	subscriber := pubsub.NewRedisBroadcaster(client)
	defer func() {
		if err := subscriber.Close(); err != nil {
			t.Logf("subscriber close: %v", err)
		}
	}()
	publisher := pubsub.NewRedisBroadcaster(client)
	defer func() {
		if err := publisher.Close(); err != nil {
			t.Logf("publisher close: %v", err)
		}
	}()

	auth := &fixedUserAuth{groupID: "server-action-test", userID: "test-user"}

	tmpl, err := New("test",
		WithAuthenticator(auth),
		WithPubSubBroadcaster(subscriber),
	)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// 1. Connect WS and read initial render
	ws1, initialMsg := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()

	if v := treeDynamic(t, initialMsg, "0"); v != "0" {
		t.Fatalf("Expected initial Count=0, got dynamic 0=%q", v)
	}

	// 2. Publish a server action that increments the counter
	if err := publisher.PublishServerAction("test-user", "increment", nil); err != nil {
		t.Fatalf("PublishServerAction failed: %v", err)
	}

	// 3. WS1 should receive the server action update
	update := readWSUpdate(t, ws1, 5*time.Second)
	tree, ok := update["tree"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tree in server action update, got: %v", update)
	}
	if v := fmt.Sprintf("%v", tree["0"]); v != "1" {
		t.Fatalf("Expected Count=1 after server action, got dynamic 0=%q", v)
	}

	// 4. Close WS1 and reconnect to verify persisted state
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	if v := treeDynamic(t, reconnectMsg, "0"); v != "1" {
		t.Errorf("Server action state NOT persisted: expected Count=1, got dynamic 0=%q. Full: %s", v, string(reconnectMsg))
	}
}

// TestStatePersistence_WSActionPersisted verifies that state is always persisted
// to SessionStore after actions, so reconnection gets the updated state.
func TestStatePersistence_WSActionPersisted(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "persist-default-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testHandleController{}
	handler := tmpl.Handle(ctrl, AsState(&testHandleState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// 1. Connect and increment
	ws1, _ := connectWSRaw(t, wsURL)
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, actionResp, err := ws1.ReadMessage()
	if err != nil {
		t.Fatalf("WS1 read failed: %v", err)
	}
	if v := treeDynamic(t, actionResp, "0"); v != "1" {
		t.Fatalf("Expected Count=1 in action response, got %q", v)
	}

	// 2. Reconnect — should get persisted state (Count=1) since persistence is default
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	if v := treeDynamic(t, reconnectMsg, "0"); v != "1" {
		t.Errorf("State should be persisted on reconnect: expected Count=1, got %q. Full: %s", v, string(reconnectMsg))
	}
}

// =============================================================================
// Ephemeral State Tests (no lvt:"persist" tags = ephemeral by default)
// =============================================================================

// TestEphemeral_WSReconnectFresh verifies that state without persist tags causes
// WebSocket reconnect to start with fresh state (action changes not persisted).
func TestEphemeral_WSReconnectFresh(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "ephemeral-ws-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// 1. Connect and increment
	ws1, _ := connectWSRaw(t, wsURL)
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("WS1 write failed: %v", err)
	}
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, actionResp, err := ws1.ReadMessage()
	if err != nil {
		t.Fatalf("WS1 read failed: %v", err)
	}
	if v := treeDynamic(t, actionResp, "0"); v != "1" {
		t.Fatalf("Expected Count=1 in action response, got %q", v)
	}

	// 2. Reconnect — ephemeral: should get fresh state (Count=0)
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	if v := treeDynamic(t, reconnectMsg, "0"); v != "0" {
		t.Errorf("Ephemeral: expected Count=0 on reconnect (fresh state), got %q. Full: %s", v, string(reconnectMsg))
	}
}

// TestEphemeral_WSActionInMemory verifies that actions within a single
// WebSocket connection still modify in-memory state normally.
func TestEphemeral_WSActionInMemory(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "ephemeral-ws-inmem-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws, _ := connectWSRaw(t, wsURL)
	defer func() {
		if err := ws.Close(); err != nil {
			t.Logf("ws close: %v", err)
		}
	}()

	// Increment twice within the same connection
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	for i := 0; i < 2; i++ {
		if err := ws.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}
		_, _, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
	}

	// Third action should return Count=3
	if err := ws.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	_, resp, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if v := treeDynamic(t, resp, "0"); v != "3" {
		t.Errorf("Expected Count=3 in action response, got %q", v)
	}
}

// TestEphemeral_HTTPGetAlwaysFresh verifies that every HTTP GET with
// ephemeral state starts with fresh state (Mount called each time).
func TestEphemeral_HTTPGetAlwaysFresh(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	server := httptest.NewServer(handler)
	defer server.Close()

	// First GET
	resp1, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET 1 failed: %v", err)
	}
	body1, _ := io.ReadAll(resp1.Body)
	if err := resp1.Body.Close(); err != nil {
		t.Logf("resp1 body close: %v", err)
	}

	if !strings.Contains(string(body1), "Count: 0") {
		t.Fatalf("GET 1: expected Count: 0, got: %s", string(body1))
	}

	// Extract session cookie
	cookies := resp1.Cookies()

	// POST to increment
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req, _ := http.NewRequest("POST", server.URL+"/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if err := resp2.Body.Close(); err != nil {
		t.Logf("resp2 body close: %v", err)
	}

	// Second GET (with same session cookie) — should still be fresh (Count: 0)
	req2, _ := http.NewRequest("GET", server.URL+"/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	resp3, err := client.Do(req2)
	if err != nil {
		t.Fatalf("GET 2 failed: %v", err)
	}
	body3, _ := io.ReadAll(resp3.Body)
	if err := resp3.Body.Close(); err != nil {
		t.Logf("resp3 body close: %v", err)
	}

	if !strings.Contains(string(body3), "Count: 0") {
		t.Errorf("Ephemeral: GET 2 should have fresh state (Count: 0), got: %s", string(body3))
	}
}

// TestEphemeral_HTTPPostWorks verifies that HTTP POST actions still
// work in ephemeral mode (Mount runs before action).
func TestEphemeral_HTTPPostWorks(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>Count: {{.Count}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	// POST with JSON (JS client mode) to get the response body directly
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"0":"1"`) {
		t.Errorf("Expected Count=1 in POST response, got: %s", body)
	}
}

// TestMount_RunsOnHTTPPost verifies Mount() runs on POST requests (not just GET).
func TestMount_RunsOnHTTPPost(t *testing.T) {
	tmpl, err := New("test", WithWebSocketDisabled())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse(`<div>Count: {{.Count}}{{if .Message}} Message: {{.Message}}{{end}}</div>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	// POST with JSON accept to get structured response
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	// Mount sets Message = "mounted", Increment sets Count++
	// Both should be reflected in the response
	if !strings.Contains(body, "mounted") {
		t.Errorf("Expected Mount() to have run (Message='mounted'), got: %s", body)
	}
}

// TestMount_RunsOnWSReconnect verifies Mount() runs on WebSocket reconnect.
func TestMount_RunsOnWSReconnect(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "mount-ws-reconnect-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}} - {{.Message}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &testEphemeralController{}
	handler := tmpl.Handle(ctrl, AsState(&testEphemeralState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Connect, increment, then reconnect
	ws1, _ := connectWSRaw(t, wsURL)
	actionMsg, _ := json.Marshal(map[string]interface{}{
		"action": "increment",
		"data":   map[string]interface{}{},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, actionMsg); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := ws1.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	if _, _, err := ws1.ReadMessage(); err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	// Reconnect
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	// Mount() should have run on reconnect (Message = "mounted")
	if v := treeDynamic(t, reconnectMsg, "1"); v != "mounted" {
		t.Errorf("Expected Mount() to run on reconnect (Message='mounted'), got %q. Full: %s", v, string(reconnectMsg))
	}
}

// TestEphemeral_DispatchedActionNotPersisted verifies that dispatched
// actions in ephemeral mode modify in-memory state but don't persist to the store.
func TestEphemeral_DispatchedActionNotPersisted(t *testing.T) {
	auth := &fixedGroupAuth{groupID: "ephemeral-dispatch-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{.Count}} - {{.Message}}</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &broadcastTestController{}
	handler := tmpl.Handle(ctrl, AsState(&broadcastTestState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws1 := connectWS(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()

	// HTTP POST triggers Increment, which broadcasts RefreshCount to WS1
	form := url.Values{}
	form.Set("lvt-action", "Increment")
	req, _ := http.NewRequest("POST", server.URL+"/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Logf("resp body close: %v", err)
	}

	// WS1 receives the dispatched RefreshCount
	readWSUpdate(t, ws1, 3*time.Second)

	// Reconnect — ephemeral: Count should be 0 (not persisted)
	ws2, reconnectMsg := reconnectWSRaw(t, wsURL, ws1)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	if v := treeDynamic(t, reconnectMsg, "0"); v != "0" {
		t.Errorf("Ephemeral: expected Count=0 on reconnect (dispatch not persisted), got %q. Full: %s", v, string(reconnectMsg))
	}
}

// TestEphemeral_SyncStillWorks verifies that Sync() auto-dispatches
// to peer connections in ephemeral mode (uses in-memory registry, not SessionStore).
func TestEphemeral_SyncStillWorks(t *testing.T) {
	db := &syncDB{items: make(map[string][]syncDBItem)}
	auth := &fixedGroupAuth{groupID: "ephemeral-sync-test"}

	tmpl, err := New("test", WithAuthenticator(auth))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	tmpl, err = tmpl.Parse("<div>{{len .Items}} items</div>")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	ctrl := &syncController{DB: db}
	handler := tmpl.Handle(ctrl, AsState(&itemsState{}))

	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	ws1 := connectWS(t, wsURL)
	defer func() {
		if err := ws1.Close(); err != nil {
			t.Logf("ws1 close: %v", err)
		}
	}()
	ws2 := connectWS(t, wsURL)
	defer func() {
		if err := ws2.Close(); err != nil {
			t.Logf("ws2 close: %v", err)
		}
	}()

	// WS1 adds an item — Sync should auto-dispatch to WS2
	addMsg, _ := json.Marshal(map[string]interface{}{
		"action": "add",
		"data":   map[string]interface{}{"text": "test item"},
	})
	if err := ws1.WriteMessage(websocket.TextMessage, addMsg); err != nil {
		t.Fatalf("ws1 write failed: %v", err)
	}

	// WS1 gets its own action response
	readWSUpdate(t, ws1, 3*time.Second)

	// WS2 should receive the Sync dispatch (in-memory, not via SessionStore).
	// Dynamic "0" is {{len .Items}} = "1" after Add.
	update2 := readWSUpdate(t, ws2, 3*time.Second)
	if tree, ok := update2["tree"].(map[string]interface{}); ok {
		if v, ok := tree["0"].(string); ok && v != "1" {
			t.Errorf("Ephemeral Sync: expected len(Items)=1 on WS2, got %q", v)
		}
	}
}
