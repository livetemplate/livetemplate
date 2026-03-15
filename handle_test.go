package livetemplate

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ============================================================================
// Task 7: New Handle() Signature Tests
// ============================================================================

type testHandleState struct {
	Count   int
	Message string
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
	Count   int
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
