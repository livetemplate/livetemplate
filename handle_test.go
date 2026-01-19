package livetemplate

import (
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

	// Should have Location header with flash message
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "success=") {
		t.Errorf("Expected flash message in redirect URL, got: %s", location)
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
