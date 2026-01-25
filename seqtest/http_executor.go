package seqtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPExecutor tests HTTP handlers by simulating form submissions
// This provides a more realistic test than DirectExecutor without browser overhead
type HTTPExecutor struct {
	handler    http.Handler
	server     *httptest.Server
	client     *http.Client
	history    *StateHistory
	invariants []Invariant

	// Request tracking
	lastResponse *http.Response
	lastBody     string
	lastTree     map[string]interface{}
	cookies      []*http.Cookie

	mu sync.Mutex
}

// NewHTTPExecutor creates a new HTTPExecutor
// The handler should be a LiveTemplate handler (from tmpl.Handle())
func NewHTTPExecutor(handler http.Handler) *HTTPExecutor {
	jar, _ := cookiejar.New(nil)

	executor := &HTTPExecutor{
		handler:    handler,
		history:    NewStateHistory(nil),
		invariants: DefaultInvariants,
		client: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
	}

	return executor
}

// WithServer starts an httptest.Server for the handler
// This allows testing with actual HTTP instead of httptest.ResponseRecorder
func (e *HTTPExecutor) WithServer() *HTTPExecutor {
	e.server = httptest.NewServer(e.handler)
	return e
}

// Close shuts down the test server if started
func (e *HTTPExecutor) Close() {
	if e.server != nil {
		e.server.Close()
	}
}

// WithInvariants adds invariants to check after each action
func (e *HTTPExecutor) WithInvariants(invariants ...Invariant) Executor {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.invariants = append(e.invariants, invariants...)
	return e
}

// CurrentState returns nil for HTTPExecutor (state is server-side)
func (e *HTTPExecutor) CurrentState() interface{} {
	return nil
}

// History returns the request/response history
func (e *HTTPExecutor) History() *StateHistory {
	return e.history
}

// LastResponse returns the last HTTP response
func (e *HTTPExecutor) LastResponse() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastBody
}

// LastTree returns the last parsed tree from response
func (e *HTTPExecutor) LastTree() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastTree
}

// Reset clears the executor state
func (e *HTTPExecutor) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = NewStateHistory(nil)
	e.lastResponse = nil
	e.lastBody = ""
	e.lastTree = nil
	e.cookies = nil

	// Reset cookie jar
	jar, _ := cookiejar.New(nil)
	e.client.Jar = jar
}

// Run executes a complete scenario
func (e *HTTPExecutor) Run(scenario Scenario) error {
	e.Reset()

	// Initial GET request to establish session
	if err := e.doInitialRequest(); err != nil {
		return fmt.Errorf("initial request failed: %w", err)
	}

	// Execute actions
	for i, action := range scenario.Actions {
		if err := e.executeAction(i, action); err != nil {
			return fmt.Errorf("action %d (%s) failed: %w", i, action.Name, err)
		}
	}

	return nil
}

// RunSequence executes a slice of actions
func (e *HTTPExecutor) RunSequence(actions []Action) error {
	return e.Run(Scenario{Actions: actions})
}

// ExecuteOne executes a single action via HTTP POST
func (e *HTTPExecutor) ExecuteOne(action Action) (interface{}, error) {
	err := e.executeAction(-1, action)
	return nil, err
}

// doInitialRequest performs GET to establish session
func (e *HTTPExecutor) doInitialRequest() error {
	return e.doRequest("GET", "", nil)
}

// executeAction performs a POST with the action
func (e *HTTPExecutor) executeAction(index int, action Action) error {
	// Build form data
	form := url.Values{}
	form.Set("lvt-action", action.Name)

	for key, value := range action.Data {
		switch v := value.(type) {
		case string:
			form.Set(key, v)
		case int:
			form.Set(key, fmt.Sprintf("%d", v))
		case float64:
			form.Set(key, fmt.Sprintf("%v", v))
		case bool:
			if v {
				form.Set(key, "true")
			} else {
				form.Set(key, "false")
			}
		default:
			// Try JSON encoding for complex types
			jsonBytes, err := json.Marshal(v)
			if err == nil {
				form.Set(key, string(jsonBytes))
			}
		}
	}

	return e.doRequest("POST", "", strings.NewReader(form.Encode()))
}

// doRequest performs an HTTP request
func (e *HTTPExecutor) doRequest(method, path string, body io.Reader) error {
	var resp *http.Response
	var err error

	if e.server != nil {
		// Use actual HTTP
		reqURL := e.server.URL
		if path != "" {
			reqURL = e.server.URL + path
		}

		var req *http.Request
		req, err = http.NewRequest(method, reqURL, body)
		if err != nil {
			return err
		}

		if method == "POST" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err = e.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	} else {
		// Use httptest.ResponseRecorder
		var req *http.Request
		if path == "" {
			path = "/"
		}
		req = httptest.NewRequest(method, path, body)

		if method == "POST" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		// Restore cookies from previous requests
		for _, cookie := range e.cookies {
			req.AddCookie(cookie)
		}

		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)

		resp = rec.Result()
		defer resp.Body.Close()

		// Save cookies for next request
		e.cookies = append(e.cookies, resp.Cookies()...)
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.lastResponse = resp
	e.lastBody = string(bodyBytes)

	// Try to parse as JSON tree
	if len(bodyBytes) > 0 && bodyBytes[0] == '{' {
		var tree map[string]interface{}
		if json.Unmarshal(bodyBytes, &tree) == nil {
			e.lastTree = tree
		}
	}
	e.mu.Unlock()

	// Check response status
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ResponseContains checks if the last response contains the given string
func (e *HTTPExecutor) ResponseContains(s string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Contains(e.lastBody, s)
}

// AssertResponseContains fails the test if response doesn't contain s
func (e *HTTPExecutor) AssertResponseContains(t TestingT, s string) {
	t.Helper()
	if !e.ResponseContains(s) {
		t.Errorf("response does not contain %q\nResponse: %s", s, e.lastBody)
	}
}

// AssertResponseNotContains fails the test if response contains s
func (e *HTTPExecutor) AssertResponseNotContains(t TestingT, s string) {
	t.Helper()
	if e.ResponseContains(s) {
		t.Errorf("response should not contain %q\nResponse: %s", s, e.lastBody)
	}
}

// TestingT is the testing interface
type TestingT interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// HTTPAction represents an action with HTTP-specific options
type HTTPAction struct {
	Action
	Headers map[string]string // Custom headers
	Cookies []*http.Cookie    // Cookies to send
	Path    string            // Custom path (default "/")
}

// ExecuteHTTP executes an HTTP action with full control
func (e *HTTPExecutor) ExecuteHTTP(action HTTPAction) error {
	// Build form data
	form := url.Values{}
	form.Set("lvt-action", action.Name)

	for key, value := range action.Data {
		form.Set(key, fmt.Sprintf("%v", value))
	}

	var resp *http.Response
	var err error

	path := action.Path
	if path == "" {
		path = "/"
	}

	if e.server != nil {
		req, err := http.NewRequest("POST", e.server.URL+path, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range action.Headers {
			req.Header.Set(k, v)
		}
		for _, c := range action.Cookies {
			req.AddCookie(c)
		}

		resp, err = e.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	} else {
		req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range action.Headers {
			req.Header.Set(k, v)
		}
		for _, c := range action.Cookies {
			req.AddCookie(c)
		}
		for _, c := range e.cookies {
			req.AddCookie(c)
		}

		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)

		resp = rec.Result()
		defer resp.Body.Close()

		e.cookies = append(e.cookies, resp.Cookies()...)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.lastResponse = resp
	e.lastBody = string(bodyBytes)
	e.mu.Unlock()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// JSONAction sends an action as JSON instead of form data
func (e *HTTPExecutor) JSONAction(action Action) error {
	payload := map[string]interface{}{
		"action": action.Name,
		"data":   action.Data,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var resp *http.Response

	if e.server != nil {
		req, err := http.NewRequest("POST", e.server.URL, bytes.NewReader(jsonBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = e.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	} else {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		for _, c := range e.cookies {
			req.AddCookie(c)
		}

		rec := httptest.NewRecorder()
		e.handler.ServeHTTP(rec, req)

		resp = rec.Result()
		defer resp.Body.Close()

		e.cookies = append(e.cookies, resp.Cookies()...)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.lastResponse = resp
	e.lastBody = string(bodyBytes)
	e.mu.Unlock()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
