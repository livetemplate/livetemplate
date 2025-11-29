package livetemplate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestActionContext_IsHTTP tests the IsHTTP method
func TestActionContext_IsHTTP(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *ActionContext
		expected bool
	}{
		{
			name: "HTTP context (both w and r)",
			ctx: &ActionContext{
				w: httptest.NewRecorder(),
				r: httptest.NewRequest("POST", "/", nil),
			},
			expected: true,
		},
		{
			name: "WebSocket context (both nil)",
			ctx: &ActionContext{
				w: nil,
				r: nil,
			},
			expected: false,
		},
		{
			name: "only w set",
			ctx: &ActionContext{
				w: httptest.NewRecorder(),
				r: nil,
			},
			expected: false,
		},
		{
			name: "only r set",
			ctx: &ActionContext{
				w: nil,
				r: httptest.NewRequest("POST", "/", nil),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.IsHTTP(); got != tt.expected {
				t.Errorf("IsHTTP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestActionContext_SetCookie_HTTP tests SetCookie with HTTP context
func TestActionContext_SetCookie_HTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)

	ctx := &ActionContext{
		w:   rec,
		r:   req,
		Ctx: context.Background(),
	}

	cookie := &http.Cookie{
		Name:     "session_token",
		Value:    "abc123",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}

	err := ctx.SetCookie(cookie)
	if err != nil {
		t.Fatalf("SetCookie() error = %v, want nil", err)
	}

	// Verify cookie was set
	setCookie := rec.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("Set-Cookie header not found")
	}

	// Check cookie contains expected values
	if !strings.Contains(setCookie, "session_token=abc123") {
		t.Errorf("Set-Cookie does not contain expected value, got: %s", setCookie)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Errorf("Set-Cookie does not contain HttpOnly, got: %s", setCookie)
	}
	if !strings.Contains(setCookie, "Secure") {
		t.Errorf("Set-Cookie does not contain Secure, got: %s", setCookie)
	}
}

// TestActionContext_SetCookie_NoHTTP tests SetCookie without HTTP context
func TestActionContext_SetCookie_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	cookie := &http.Cookie{
		Name:  "test",
		Value: "value",
	}

	err := ctx.SetCookie(cookie)
	if err != ErrNoHTTPContext {
		t.Errorf("SetCookie() error = %v, want ErrNoHTTPContext", err)
	}
}

// TestActionContext_GetCookie tests GetCookie
func TestActionContext_GetCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: "xyz789",
	})

	ctx := &ActionContext{
		w:   httptest.NewRecorder(),
		r:   req,
		Ctx: context.Background(),
	}

	cookie, err := ctx.GetCookie("session_token")
	if err != nil {
		t.Fatalf("GetCookie() error = %v, want nil", err)
	}

	if cookie.Value != "xyz789" {
		t.Errorf("GetCookie() value = %v, want %v", cookie.Value, "xyz789")
	}
}

// TestActionContext_GetCookie_NoHTTP tests GetCookie without HTTP context
func TestActionContext_GetCookie_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	_, err := ctx.GetCookie("session_token")
	if err != ErrNoHTTPContext {
		t.Errorf("GetCookie() error = %v, want ErrNoHTTPContext", err)
	}
}

// TestActionContext_GetCookie_NotFound tests GetCookie with missing cookie
func TestActionContext_GetCookie_NotFound(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)

	ctx := &ActionContext{
		w:   httptest.NewRecorder(),
		r:   req,
		Ctx: context.Background(),
	}

	_, err := ctx.GetCookie("nonexistent")
	if err != http.ErrNoCookie {
		t.Errorf("GetCookie() error = %v, want http.ErrNoCookie", err)
	}
}

// TestActionContext_DeleteCookie tests DeleteCookie
func TestActionContext_DeleteCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)

	ctx := &ActionContext{
		w:   rec,
		r:   req,
		Ctx: context.Background(),
	}

	err := ctx.DeleteCookie("session_token")
	if err != nil {
		t.Fatalf("DeleteCookie() error = %v, want nil", err)
	}

	// Verify cookie was deleted (MaxAge=-1)
	setCookie := rec.Header().Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("Set-Cookie header not found")
	}

	if !strings.Contains(setCookie, "session_token=") {
		t.Errorf("Set-Cookie does not contain cookie name, got: %s", setCookie)
	}
	if !strings.Contains(setCookie, "Max-Age=0") && !strings.Contains(setCookie, "Max-Age=-1") {
		// Note: Go's http.SetCookie converts MaxAge=-1 to "Max-Age=0" in the header
		t.Errorf("Set-Cookie does not have expired Max-Age, got: %s", setCookie)
	}
}

// TestActionContext_DeleteCookie_NoHTTP tests DeleteCookie without HTTP context
func TestActionContext_DeleteCookie_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	err := ctx.DeleteCookie("session_token")
	if err != ErrNoHTTPContext {
		t.Errorf("DeleteCookie() error = %v, want ErrNoHTTPContext", err)
	}
}

// TestActionContext_Redirect_HTTP tests Redirect with HTTP context
func TestActionContext_Redirect_HTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", nil)

	ctx := &ActionContext{
		w:   rec,
		r:   req,
		Ctx: context.Background(),
	}

	err := ctx.Redirect("/dashboard", http.StatusSeeOther)
	if err != nil {
		t.Fatalf("Redirect() error = %v, want nil", err)
	}

	// Verify redirect response
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Redirect() status = %v, want %v", rec.Code, http.StatusSeeOther)
	}

	location := rec.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("Redirect() location = %v, want %v", location, "/dashboard")
	}
}

// TestActionContext_Redirect_NoHTTP tests Redirect without HTTP context
func TestActionContext_Redirect_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	err := ctx.Redirect("/dashboard", http.StatusSeeOther)
	if err != ErrNoHTTPContext {
		t.Errorf("Redirect() error = %v, want ErrNoHTTPContext", err)
	}
}

// TestActionContext_Redirect_InvalidCode tests Redirect with invalid status code
func TestActionContext_Redirect_InvalidCode(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", nil)

			ctx := &ActionContext{
				w:   rec,
				r:   req,
				Ctx: context.Background(),
			}

			err := ctx.Redirect("/dashboard", tt.code)
			if err != ErrInvalidRedirectCode {
				t.Errorf("Redirect() with code %d error = %v, want ErrInvalidRedirectCode", tt.code, err)
			}
		})
	}
}

// TestActionContext_Redirect_ValidCodes tests Redirect with all valid 3xx codes
func TestActionContext_Redirect_ValidCodes(t *testing.T) {
	validCodes := []int{
		http.StatusMultipleChoices,  // 300
		http.StatusMovedPermanently, // 301
		http.StatusFound,            // 302
		http.StatusSeeOther,         // 303
		http.StatusNotModified,      // 304
		http.StatusTemporaryRedirect, // 307
		http.StatusPermanentRedirect, // 308
	}

	for _, code := range validCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", nil)

			ctx := &ActionContext{
				w:   rec,
				r:   req,
				Ctx: context.Background(),
			}

			err := ctx.Redirect("/dashboard", code)
			if err != nil {
				t.Errorf("Redirect() with code %d error = %v, want nil", code, err)
			}
		})
	}
}

// TestActionContext_Redirect_OpenRedirect tests protection against open redirects
func TestActionContext_Redirect_OpenRedirect(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"protocol-relative URL", "//evil.com"},
		{"absolute URL with http", "http://evil.com"},
		{"absolute URL with https", "https://evil.com"},
		{"absolute URL with ftp", "ftp://evil.com"},
		{"no leading slash", "dashboard"},
		{"relative path", "dashboard/settings"},
		{"protocol-relative with path", "//evil.com/phishing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", nil)

			ctx := &ActionContext{
				w:   rec,
				r:   req,
				Ctx: context.Background(),
			}

			err := ctx.Redirect(tt.url, http.StatusSeeOther)
			if err != ErrInvalidRedirectURL {
				t.Errorf("Redirect(%q) error = %v, want ErrInvalidRedirectURL", tt.url, err)
			}
		})
	}
}

// TestActionContext_Redirect_ValidURLs tests valid redirect URLs
func TestActionContext_Redirect_ValidURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"root path", "/"},
		{"simple path", "/dashboard"},
		{"nested path", "/admin/users/123"},
		{"path with query", "/search?q=test"},
		{"path with fragment", "/page#section"},
		{"path with query and fragment", "/page?id=1#section"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/", nil)

			ctx := &ActionContext{
				w:   rec,
				r:   req,
				Ctx: context.Background(),
			}

			err := ctx.Redirect(tt.url, http.StatusSeeOther)
			if err != nil {
				t.Errorf("Redirect(%q) error = %v, want nil", tt.url, err)
			}

			location := rec.Header().Get("Location")
			if location != tt.url {
				t.Errorf("Redirect() location = %v, want %v", location, tt.url)
			}
		})
	}
}

// TestActionContext_SetHeader tests SetHeader
func TestActionContext_SetHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", nil)

	ctx := &ActionContext{
		w:   rec,
		r:   req,
		Ctx: context.Background(),
	}

	err := ctx.SetHeader("X-Custom-Header", "custom-value")
	if err != nil {
		t.Fatalf("SetHeader() error = %v, want nil", err)
	}

	if got := rec.Header().Get("X-Custom-Header"); got != "custom-value" {
		t.Errorf("SetHeader() got = %v, want %v", got, "custom-value")
	}
}

// TestActionContext_SetHeader_NoHTTP tests SetHeader without HTTP context
func TestActionContext_SetHeader_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	err := ctx.SetHeader("X-Custom-Header", "custom-value")
	if err != ErrNoHTTPContext {
		t.Errorf("SetHeader() error = %v, want ErrNoHTTPContext", err)
	}
}

// TestActionContext_GetHeader tests GetHeader
func TestActionContext_GetHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Request-ID", "req-123")

	ctx := &ActionContext{
		w:   httptest.NewRecorder(),
		r:   req,
		Ctx: context.Background(),
	}

	got := ctx.GetHeader("X-Request-ID")
	if got != "req-123" {
		t.Errorf("GetHeader() = %v, want %v", got, "req-123")
	}
}

// TestActionContext_GetHeader_NoHTTP tests GetHeader without HTTP context
func TestActionContext_GetHeader_NoHTTP(t *testing.T) {
	ctx := &ActionContext{
		w:   nil,
		r:   nil,
		Ctx: context.Background(),
	}

	got := ctx.GetHeader("X-Request-ID")
	if got != "" {
		t.Errorf("GetHeader() = %v, want empty string", got)
	}
}

// TestActionContext_GetHeader_NotFound tests GetHeader with missing header
func TestActionContext_GetHeader_NotFound(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)

	ctx := &ActionContext{
		w:   httptest.NewRecorder(),
		r:   req,
		Ctx: context.Background(),
	}

	got := ctx.GetHeader("X-Nonexistent")
	if got != "" {
		t.Errorf("GetHeader() = %v, want empty string", got)
	}
}

// Test isValidRedirectURL helper function
func TestIsValidRedirectURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		// Valid URLs
		{"/", true},
		{"/dashboard", true},
		{"/admin/users", true},
		{"/search?q=test", true},
		{"/page#section", true},

		// Invalid URLs
		{"//evil.com", false},
		{"http://evil.com", false},
		{"https://evil.com", false},
		{"ftp://evil.com", false},
		{"dashboard", false},
		{"", false},
		{"//", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := isValidRedirectURL(tt.url); got != tt.expected {
				t.Errorf("isValidRedirectURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}

