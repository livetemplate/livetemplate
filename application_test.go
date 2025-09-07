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

// TestCounter is a test data model for testing data model registration
type TestCounter struct {
	Value int
}

func (c *TestCounter) Increment(ctx *ActionContext) error {
	c.Value++
	return ctx.Data(map[string]interface{}{"Value": c.Value})
}

func (c *TestCounter) Decrement(ctx *ActionContext) error {
	c.Value--
	return ctx.Data(map[string]interface{}{"Value": c.Value})
}

func (c *TestCounter) Reset(ctx *ActionContext) error {
	// Demonstrate ActionContext.GetInt() usage
	newValue := ctx.GetInt("value")
	c.Value = newValue
	return ctx.Data(map[string]interface{}{"Value": c.Value})
}

// TestCounter2 is a second test data model for testing conflict resolution
type TestCounter2 struct {
	Count int
}

func (c *TestCounter2) Increment(ctx *ActionContext) error {
	c.Count++
	return ctx.Data(map[string]interface{}{"Count": c.Count})
}

func TestApplicationPage_RegisterDataModel(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	tmpl := template.Must(template.New("test").Parse("<div>{{.Value}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	page, err := app.NewPage("test", map[string]int{"Value": 0})
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Test successful registration
	counter := &TestCounter{Value: 5}
	err = page.RegisterDataModel(counter)
	if err != nil {
		t.Fatalf("RegisterDataModel failed: %v", err)
	}

	// Verify data model is registered
	if len(page.dataModels) != 1 {
		t.Errorf("expected 1 data model, got %d", len(page.dataModels))
	}

	model := page.dataModels[0]
	if model.Name != "testcounter" {
		t.Errorf("expected model name 'testcounter', got %q", model.Name)
	}

	expectedMethods := []string{"increment", "decrement", "reset"}
	if len(model.ActionMethods) != len(expectedMethods) {
		t.Errorf("expected %d action methods, got %d", len(expectedMethods), len(model.ActionMethods))
	}

	for _, methodName := range expectedMethods {
		if _, exists := model.ActionMethods[methodName]; !exists {
			t.Errorf("expected action method %q not found", methodName)
		}
	}
}

func TestApplicationPage_RegisterDataModel_InvalidInputs(t *testing.T) {
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

	// Test nil model
	err = page.RegisterDataModel(nil)
	if err == nil {
		t.Error("expected error for nil data model")
	}

	// Test non-struct model
	err = page.RegisterDataModel("not a struct")
	if err == nil {
		t.Error("expected error for non-struct data model")
	}

	// Test struct with no valid action methods
	type EmptyModel struct {
		Value int
	}
	err = page.RegisterDataModel(&EmptyModel{})
	if err == nil {
		t.Error("expected error for model with no valid action methods")
	}
}

func TestApplicationPage_DataModelActionExecution(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}
	defer app.Close()

	tmpl := template.Must(template.New("test").Parse("<div>{{.Value}}</div>"))
	err = app.RegisterTemplate("test", tmpl)
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	page, err := app.NewPage("test", map[string]int{"Value": 0})
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	// Register data model
	counter := &TestCounter{Value: 5}
	err = page.RegisterDataModel(counter)
	if err != nil {
		t.Fatalf("RegisterDataModel failed: %v", err)
	}

	// Test increment action
	_, err = page.HandleAction(context.TODO(), NewActionMessage("increment", nil))
	if err != nil {
		t.Fatalf("HandleAction increment failed: %v", err)
	}

	if counter.Value != 6 {
		t.Errorf("expected counter value 6, got %d", counter.Value)
	}

	// Test decrement action
	_, err = page.HandleAction(context.TODO(), NewActionMessage("decrement", nil))
	if err != nil {
		t.Fatalf("HandleAction decrement failed: %v", err)
	}

	if counter.Value != 5 {
		t.Errorf("expected counter value 5, got %d", counter.Value)
	}

	// Test reset action
	_, err = page.HandleAction(context.TODO(), NewActionMessage("reset", nil))
	if err != nil {
		t.Fatalf("HandleAction reset failed: %v", err)
	}

	if counter.Value != 0 {
		t.Errorf("expected counter value 0, got %d", counter.Value)
	}
}

func TestApplicationPage_ActionConflictDetection(t *testing.T) {
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

	// Register two models with conflicting action names
	counter1 := &TestCounter{Value: 1}
	counter2 := &TestCounter2{Count: 2}

	err = page.RegisterDataModel(counter1)
	if err != nil {
		t.Fatalf("RegisterDataModel counter1 failed: %v", err)
	}

	err = page.RegisterDataModel(counter2)
	if err != nil {
		t.Fatalf("RegisterDataModel counter2 failed: %v", err)
	}

	// Test conflict detection - should fail for direct action
	_, err = page.HandleAction(context.TODO(), NewActionMessage("increment", nil))
	if err == nil {
		t.Error("expected error due to action conflict")
	}

	expectedError := "action \"increment\" conflicts between multiple data models. Use namespaced actions: testcounter.increment, testcounter2.increment"
	if err.Error() != expectedError {
		t.Errorf("expected error message: %s\ngot: %s", expectedError, err.Error())
	}
}

func TestApplicationPage_NamespacedActions(t *testing.T) {
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

	// Register two models with conflicting action names
	counter1 := &TestCounter{Value: 1}
	counter2 := &TestCounter2{Count: 2}

	err = page.RegisterDataModel(counter1)
	if err != nil {
		t.Fatalf("RegisterDataModel counter1 failed: %v", err)
	}

	err = page.RegisterDataModel(counter2)
	if err != nil {
		t.Fatalf("RegisterDataModel counter2 failed: %v", err)
	}

	// Test namespaced action for first counter
	_, err = page.HandleAction(context.TODO(), NewActionMessage("testcounter.increment", nil))
	if err != nil {
		t.Fatalf("HandleAction testcounter.increment failed: %v", err)
	}

	if counter1.Value != 2 {
		t.Errorf("expected counter1 value 2, got %d", counter1.Value)
	}

	if counter2.Count != 2 {
		t.Errorf("expected counter2 count unchanged at 2, got %d", counter2.Count)
	}

	// Test namespaced action for second counter
	_, err = page.HandleAction(context.TODO(), NewActionMessage("testcounter2.increment", nil))
	if err != nil {
		t.Fatalf("HandleAction testcounter2.increment failed: %v", err)
	}

	if counter1.Value != 2 {
		t.Errorf("expected counter1 value unchanged at 2, got %d", counter1.Value)
	}

	if counter2.Count != 3 {
		t.Errorf("expected counter2 count 3, got %d", counter2.Count)
	}
}

func TestApplicationPage_ActionPriorityOrder(t *testing.T) {
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

	// Register a regular action handler first
	handlerCalled := false
	page.RegisterAction("increment", func(currentData interface{}, actionData map[string]interface{}) (interface{}, error) {
		handlerCalled = true
		return map[string]interface{}{"handler": "called"}, nil
	})

	// Register data model with same action name
	counter := &TestCounter{Value: 5}
	err = page.RegisterDataModel(counter)
	if err != nil {
		t.Fatalf("RegisterDataModel failed: %v", err)
	}

	// Test that action handler takes priority over data model
	_, err = page.HandleAction(context.TODO(), NewActionMessage("increment", nil))
	if err != nil {
		t.Fatalf("HandleAction failed: %v", err)
	}

	if !handlerCalled {
		t.Error("expected action handler to be called, but it wasn't")
	}

	if counter.Value != 5 {
		t.Errorf("expected counter value unchanged at 5, got %d", counter.Value)
	}
}

func TestActionContext_DataBinding(t *testing.T) {
	// Test data binding functionality
	actionData := map[string]interface{}{
		"name":   "Test Counter",
		"value":  42.0, // JSON numbers are float64
		"active": true,
	}

	ctx := NewActionContext(actionData)

	// Test individual getters
	if name := ctx.GetString("name"); name != "Test Counter" {
		t.Errorf("GetString: expected 'Test Counter', got %q", name)
	}

	if value := ctx.GetInt("value"); value != 42 {
		t.Errorf("GetInt: expected 42, got %d", value)
	}

	if active := ctx.GetBool("active"); !active {
		t.Errorf("GetBool: expected true, got %v", active)
	}

	// Test struct binding
	type TestStruct struct {
		Name   string `json:"name"`
		Value  int    `json:"value"`
		Active bool   `json:"active"`
	}

	var target TestStruct
	err := ctx.Bind(&target)
	if err != nil {
		t.Fatalf("Bind failed: %v", err)
	}

	if target.Name != "Test Counter" {
		t.Errorf("Bind Name: expected 'Test Counter', got %q", target.Name)
	}

	if target.Value != 42 {
		t.Errorf("Bind Value: expected 42, got %d", target.Value)
	}

	if !target.Active {
		t.Errorf("Bind Active: expected true, got %v", target.Active)
	}
}

func TestActionContext_ResponseData(t *testing.T) {
	ctx := NewActionContext(nil)

	// Test setting response data
	responseData := map[string]interface{}{
		"status": "success",
		"value":  123,
	}

	err := ctx.Data(responseData)
	if err != nil {
		t.Errorf("Data() failed: %v", err)
	}

	// Test getting response data
	result := ctx.GetResponse()
	if result == nil {
		t.Fatal("GetResponse() returned nil")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("GetResponse() returned wrong type: %T", result)
	}

	if status := resultMap["status"]; status != "success" {
		t.Errorf("Response status: expected 'success', got %v", status)
	}

	if value := resultMap["value"]; value != 123 {
		t.Errorf("Response value: expected 123, got %v", value)
	}
}
