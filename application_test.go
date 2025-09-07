package livetemplate

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestApplication_SessionBasedAuthentication(t *testing.T) {
	// Create application
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Create a template
	tmpl := template.Must(template.New("test").Parse("<div>{{.Message}}</div>"))

	// Register the template
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// Create a page
	data := map[string]string{"Message": "Hello"}
	page, err := app.NewPage("test", data)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Test ServeHTTP sets session cookie
	w := httptest.NewRecorder()
	err = page.ServeHTTP(w, data)
	if err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}

	// Check session cookie is set
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "livetemplate_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("session cookie not set")
	}

	if sessionCookie.Value == "" {
		t.Error("session cookie has empty value")
	}

	if !sessionCookie.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}

	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Error("session cookie should have SameSite=Lax")
	}

	// Test GetPage with session cookie
	req := httptest.NewRequest("GET", "/ws", nil)
	req.AddCookie(sessionCookie)

	retrievedPage, err := app.GetPage(req)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if retrievedPage == nil {
		t.Fatal("GetPage returned nil page")
	}

	// Session ID should match
	if retrievedPage.GetSessionID() != sessionCookie.Value {
		t.Errorf("session ID mismatch: got %s, want %s",
			retrievedPage.GetSessionID(), sessionCookie.Value)
	}
}

func TestApplication_GetPageWithoutSession(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Request without session cookie
	req := httptest.NewRequest("GET", "/ws", nil)

	_, err = app.GetPage(req)
	if err == nil {
		t.Error("GetPage should fail without session cookie")
	}

	if !strings.Contains(err.Error(), "no session cookie found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApplication_GetPageWithInvalidSession(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Request with invalid session cookie
	req := httptest.NewRequest("GET", "/ws", nil)
	req.AddCookie(&http.Cookie{
		Name:  "livetemplate_session",
		Value: "invalid-session-id",
	})

	_, err = app.GetPage(req)
	if err == nil {
		t.Error("GetPage should fail with invalid session")
	}

	if !strings.Contains(err.Error(), "invalid or expired session") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApplication_GetPageWithCacheInfo(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Create a page
	tmpl := template.Must(template.New("test").Parse("<div>{{.Message}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	data := map[string]string{"Message": "Hello"}
	page, err := app.NewPage("test", data)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Get session cookie
	w := httptest.NewRecorder()
	err = page.ServeHTTP(w, data)
	if err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	sessionCookie := w.Result().Cookies()[0]

	// Test with cache info in query params
	req := httptest.NewRequest("GET", "/ws?has_cache=true&cached_fragments=frag1,frag2", nil)
	req.AddCookie(sessionCookie)

	retrievedPage, err := app.GetPage(req)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	cacheInfo := retrievedPage.GetCacheInfo()
	if cacheInfo == nil {
		t.Fatal("cache info should not be nil")
	}

	if !cacheInfo.HasCache {
		t.Error("HasCache should be true")
	}

	if len(cacheInfo.CachedFragments) != 2 {
		t.Errorf("expected 2 cached fragments, got %d", len(cacheInfo.CachedFragments))
	}
}

func TestApplication_SessionPersistence(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Create a page
	tmpl := template.Must(template.New("test").Parse("<div>{{.Count}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	data := map[string]int{"Count": 0}
	page, err := app.NewPage("test", data)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Get session cookie
	w := httptest.NewRecorder()
	err = page.ServeHTTP(w, data)
	if err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	sessionCookie := w.Result().Cookies()[0]

	// Make multiple requests with same session
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/ws", nil)
		req.AddCookie(sessionCookie)

		retrievedPage, err := app.GetPage(req)
		if err != nil {
			t.Fatalf("GetPage failed on request %d: %v", i, err)
		}

		if retrievedPage.GetSessionID() != sessionCookie.Value {
			t.Errorf("session ID changed on request %d", i)
		}
	}
}

func TestApplication_ActionHandling(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Register actions
	incrementCalled := false
	decrementCalled := false

	app.RegisterAction("increment", func(currentData interface{}, actionData map[string]interface{}) (interface{}, error) {
		incrementCalled = true
		data := currentData.(map[string]int)
		data["Count"]++
		return data, nil
	})

	app.RegisterAction("decrement", func(currentData interface{}, actionData map[string]interface{}) (interface{}, error) {
		decrementCalled = true
		data := currentData.(map[string]int)
		data["Count"]--
		return data, nil
	})

	// Create a page
	tmpl := template.Must(template.New("test").Parse("<div>{{.Count}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	data := map[string]int{"Count": 10}
	page, err := app.NewPage("test", data)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Get session
	w := httptest.NewRecorder()
	err = page.ServeHTTP(w, data)
	if err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	sessionCookie := w.Result().Cookies()[0]

	// Get page with session
	req := httptest.NewRequest("GET", "/ws", nil)
	req.AddCookie(sessionCookie)

	retrievedPage, err := app.GetPage(req)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	// Test increment action
	_, err = retrievedPage.HandleAction(context.TODO(), NewActionMessage("increment", nil))
	if err != nil {
		t.Fatalf("HandleAction increment failed: %v", err)
	}

	if !incrementCalled {
		t.Error("increment action not called")
	}

	// Test decrement action
	_, err = retrievedPage.HandleAction(context.TODO(), NewActionMessage("decrement", nil))
	if err != nil {
		t.Fatalf("HandleAction decrement failed: %v", err)
	}

	if !decrementCalled {
		t.Error("decrement action not called")
	}
}

func TestParseCacheInfoFromURL(t *testing.T) {
	tests := []struct {
		name              string
		queryString       string
		expectedHasCache  bool
		expectedFragments int
	}{
		{
			name:              "no cache",
			queryString:       "has_cache=false",
			expectedHasCache:  false,
			expectedFragments: 0,
		},
		{
			name:              "has cache without fragments",
			queryString:       "has_cache=true",
			expectedHasCache:  true,
			expectedFragments: 0,
		},
		{
			name:              "has cache with fragments",
			queryString:       "has_cache=true&cached_fragments=frag1,frag2,frag3",
			expectedHasCache:  true,
			expectedFragments: 3,
		},
		{
			name:              "empty query",
			queryString:       "",
			expectedHasCache:  false,
			expectedFragments: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _ := url.ParseQuery(tt.queryString)
			cacheInfo := ParseCacheInfoFromURL(values)

			if cacheInfo.HasCache != tt.expectedHasCache {
				t.Errorf("HasCache = %v, want %v", cacheInfo.HasCache, tt.expectedHasCache)
			}

			if len(cacheInfo.CachedFragments) != tt.expectedFragments {
				t.Errorf("CachedFragments length = %d, want %d",
					len(cacheInfo.CachedFragments), tt.expectedFragments)
			}
		})
	}
}

func TestApplicationPage_GetMethods(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	// Create a page
	tmpl := template.Must(template.New("test").Parse("<div>Test</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	page, err := app.NewPage("test", nil)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Test GetSessionID
	sessionID := page.GetSessionID()
	if sessionID == "" {
		t.Error("GetSessionID returned empty string")
	}

	// Test GetCacheToken
	cacheToken := page.GetCacheToken()
	if cacheToken == "" {
		t.Error("GetCacheToken returned empty string")
	}

	// Test GetToken (legacy method)
	token := page.GetToken()
	if token != sessionID {
		t.Errorf("GetToken should return session ID for compatibility")
	}
}

func TestApplication_SessionExpiration(t *testing.T) {
	// Create app with short session TTL for testing
	// Note: This would require exposing session TTL configuration
	// For now, we'll test that sessions work over reasonable time
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	tmpl := template.Must(template.New("test").Parse("<div>Test</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	page, err := app.NewPage("test", nil)
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Get session cookie
	w := httptest.NewRecorder()
	err = page.ServeHTTP(w, nil)
	if err != nil {
		t.Fatalf("ServeHTTP failed: %v", err)
	}
	sessionCookie := w.Result().Cookies()[0]

	// Immediate request should work
	req := httptest.NewRequest("GET", "/ws", nil)
	req.AddCookie(sessionCookie)

	_, err = app.GetPage(req)
	if err != nil {
		t.Errorf("GetPage should work immediately after session creation: %v", err)
	}

	// Request after a short delay should still work
	time.Sleep(100 * time.Millisecond)

	req = httptest.NewRequest("GET", "/ws", nil)
	req.AddCookie(sessionCookie)

	_, err = app.GetPage(req)
	if err != nil {
		t.Errorf("GetPage should work after short delay: %v", err)
	}
}

func TestApplication_ConcurrentSessions(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	tmpl := template.Must(template.New("test").Parse("<div>{{.ID}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// Create multiple pages concurrently
	done := make(chan bool)
	cookies := make([]*http.Cookie, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			data := map[string]int{"ID": idx}
			page, err := app.NewPage("test", data)
			if err != nil {
				t.Errorf("NewPage failed for session %d: %v", idx, err)
				done <- false
				return
			}

			w := httptest.NewRecorder()
			err = page.ServeHTTP(w, data)
			if err != nil {
				t.Errorf("ServeHTTP failed for session %d: %v", idx, err)
				done <- false
				return
			}

			cookies[idx] = w.Result().Cookies()[0]
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		success := <-done
		if !success {
			t.Fatal("concurrent session creation failed")
		}
	}

	// Verify all sessions are valid and different
	sessionIDs := make(map[string]bool)
	for i, cookie := range cookies {
		if cookie == nil {
			t.Errorf("cookie %d is nil", i)
			continue
		}

		// Check for duplicate session IDs
		if sessionIDs[cookie.Value] {
			t.Errorf("duplicate session ID: %s", cookie.Value)
		}
		sessionIDs[cookie.Value] = true

		// Verify session works
		req := httptest.NewRequest("GET", "/ws", nil)
		req.AddCookie(cookie)

		_, err := app.GetPage(req)
		if err != nil {
			t.Errorf("GetPage failed for session %d: %v", i, err)
		}
	}
}
