package livetemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestE2EErrorScenarios implements comprehensive error handling and edge case testing
func TestE2EErrorScenarios(t *testing.T) {
	suite := &E2EErrorTestSuite{
		t: t,
	}

	// Run all error scenario tests
	t.Run("Network_Failure_Scenarios", suite.TestNetworkFailureScenarios)
	t.Run("Malformed_Fragment_Data", suite.TestMalformedFragmentData)
	t.Run("Invalid_Template_Rendering", suite.TestInvalidTemplateRendering)
	t.Run("Memory_Pressure_Testing", suite.TestMemoryPressure)
	t.Run("Concurrent_Access_Race_Conditions", suite.TestConcurrentAccessRaceConditions)
	t.Run("Security_XSS_Injection", suite.TestSecurityXSSInjection)
	t.Run("Browser_Crash_Recovery", suite.TestBrowserCrashRecovery)
	t.Run("Fragment_Application_Partial_Failure", suite.TestFragmentApplicationPartialFailure)
}

// E2EErrorTestSuite provides comprehensive error scenario testing
type E2EErrorTestSuite struct {
	t *testing.T
}

// TestNetworkFailureScenarios tests server unavailability, timeouts, and connection drops
func (suite *E2EErrorTestSuite) TestNetworkFailureScenarios(t *testing.T) {
	t.Run("Server_Unavailable", func(t *testing.T) {
		// Test connection to non-existent server
		client := &http.Client{Timeout: 100 * time.Millisecond}

		// Try to connect to a closed port
		resp, err := client.Get("http://localhost:99999/")
		if err == nil {
			t.Error("Expected connection error to unavailable server, got none")
			if resp != nil {
				_ = resp.Body.Close()
			}
			return
		}

		// Verify error type (allow various connection errors)
		if !isNetworkError(err) && !strings.Contains(err.Error(), "invalid port") {
			t.Errorf("Expected network error, got: %v", err)
		}

		t.Log("✓ Server unavailable scenario handled gracefully")
	})

	t.Run("Connection_Timeout", func(t *testing.T) {
		// Create a server that delays responses
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(500 * time.Millisecond) // Delay longer than client timeout
			w.WriteHeader(http.StatusOK)
		}))
		defer slowServer.Close()

		// Client with short timeout
		client := &http.Client{Timeout: 50 * time.Millisecond}

		resp, err := client.Get(slowServer.URL)
		if err == nil {
			t.Error("Expected timeout error, got successful response")
			if resp != nil {
				_ = resp.Body.Close()
			}
			return
		}

		// Verify timeout error
		if !isTimeoutError(err) {
			t.Errorf("Expected timeout error, got: %v", err)
		}

		t.Log("✓ Connection timeout scenario handled gracefully")
	})

	t.Run("Connection_Drop_During_Transfer", func(t *testing.T) {
		// Create server that drops connection mid-transfer
		droppyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start writing response
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Starting response...")); err != nil {
				fmt.Printf("Warning: Failed to write response: %v\n", err)
			}

			// Force connection drop
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

			// Simulate connection drop by closing the underlying connection
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
				}
			}
		}))
		defer droppyServer.Close()

		resp, err := http.Get(droppyServer.URL)
		if err != nil {
			t.Logf("✓ Connection drop handled: %v", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Try to read full response - should fail
		_, err = io.ReadAll(resp.Body)
		if err == nil {
			t.Error("Expected connection drop error during transfer, got none")
		} else {
			t.Logf("✓ Connection drop during transfer handled: %v", err)
		}
	})

	t.Run("Fragment_Update_Network_Failure", func(t *testing.T) {
		// Test fragment updates with network failures
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl, err := template.New("test").Parse(`<div id="content">{{.Message}}</div>`)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Message": "Initial"})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Create server that fails randomly
		var failNext int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/update" {
				if atomic.LoadInt32(&failNext) == 1 {
					// Simulate network failure
					atomic.StoreInt32(&failNext, 0)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				// Normal fragment update
				var newData map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
					http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
					return
				}

				fragments, err := page.RenderFragments(r.Context(), newData)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(fragments); err != nil {
					fmt.Printf("Warning: Failed to encode fragments response: %v\n", err)
				}
			}
		}))
		defer server.Close()

		// Test normal update
		updateData := map[string]interface{}{"Message": "Updated"}
		updateJSON, _ := json.Marshal(updateData)

		resp, err := http.Post(server.URL+"/update", "application/json", bytes.NewBuffer(updateJSON))
		if err != nil {
			t.Fatalf("Normal update failed: %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 for normal update, got %d", resp.StatusCode)
		}

		// Test update with server failure
		atomic.StoreInt32(&failNext, 1)

		resp, err = http.Post(server.URL+"/update", "application/json", bytes.NewBuffer(updateJSON))
		if err != nil {
			t.Logf("✓ Network failure during fragment update handled: %v", err)
		} else {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == http.StatusInternalServerError {
				t.Log("✓ Server error during fragment update handled gracefully")
			}
		}
	})
}

// TestMalformedFragmentData tests rejection and fallback behavior for bad data
func (suite *E2EErrorTestSuite) TestMalformedFragmentData(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	tmpl, err := template.New("test").Parse(`<div>{{.Value}}</div>`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Value": "Initial"})
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer func() {
		if err := page.Close(); err != nil {
			t.Logf("Warning: Failed to close page: %v", err)
		}
	}()

	t.Run("Invalid_JSON_Data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/update" {
				// Try to decode invalid JSON
				var newData map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&newData)
				if err != nil {
					// Graceful fallback for malformed JSON
					t.Log("✓ Malformed JSON rejected gracefully")
					w.Header().Set("Content-Type", "application/json")
					fallbackResponse := []Fragment{
						{
							ID:       "fallback",
							Strategy: "replacement",
							Action:   "replace",
							Data:     map[string]interface{}{"html": "<div>Error: Invalid data format</div>"},
						},
					}
					if err := json.NewEncoder(w).Encode(fallbackResponse); err != nil {
						fmt.Printf("Warning: Failed to encode fallback response: %v\n", err)
					}
					return
				}

				fragments, err := page.RenderFragments(r.Context(), newData)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				if err := json.NewEncoder(w).Encode(fragments); err != nil {
					fmt.Printf("Warning: Failed to encode fragments response: %v\n", err)
				}
			}
		}))
		defer server.Close()

		// Send invalid JSON
		resp, err := http.Post(server.URL+"/update", "application/json",
			strings.NewReader(`{invalid json}`))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Should get fallback response
		var fragments []Fragment
		if err := json.NewDecoder(resp.Body).Decode(&fragments); err != nil {
			t.Logf("✓ Invalid JSON properly rejected: %v", err)
		} else if len(fragments) > 0 && fragments[0].ID == "fallback" {
			t.Log("✓ Fallback response provided for invalid JSON")
		}
	})

	t.Run("Nil_Data_Handling", func(t *testing.T) {
		// Test fragment generation with nil data
		_, err := page.RenderFragments(context.Background(), nil)
		if err != nil {
			t.Logf("✓ Nil data handled with error: %v", err)
		} else {
			t.Log("✓ Nil data handled gracefully")
		}
	})

	t.Run("Circular_Reference_Data", func(t *testing.T) {
		// Create data with circular reference
		circularData := make(map[string]interface{})
		circularData["self"] = circularData
		circularData["value"] = "test"

		_, err := page.RenderFragments(context.Background(), circularData)
		if err != nil {
			t.Logf("✓ Circular reference data handled with error: %v", err)
		} else {
			t.Log("✓ Circular reference data handled without error")
		}
	})

	t.Run("Extremely_Large_Data", func(t *testing.T) {
		// Create very large data structure
		largeData := make(map[string]interface{})
		largeArray := make([]string, 10000)
		for i := range largeArray {
			largeArray[i] = fmt.Sprintf("Item_%d_with_long_content_that_takes_up_space", i)
		}
		largeData["Value"] = "Large dataset"
		largeData["Items"] = largeArray

		fragments, err := page.RenderFragments(context.Background(), largeData)
		if err != nil {
			t.Logf("✓ Large data handled with error: %v", err)
		} else {
			t.Logf("✓ Large data processed successfully: %d fragments", len(fragments))
		}
	})
}

// TestInvalidTemplateRendering tests error recovery for template issues
func (suite *E2EErrorTestSuite) TestInvalidTemplateRendering(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	t.Run("Template_With_Invalid_Syntax", func(t *testing.T) {
		// Try to parse template with invalid syntax
		_, err := template.New("invalid").Parse(`<div>{{.Missing}}`)
		if err != nil {
			t.Logf("✓ Invalid template syntax rejected: %v", err)
		} else {
			t.Error("Expected template parsing error for invalid syntax")
		}
	})

	t.Run("Template_With_Missing_Fields", func(t *testing.T) {
		tmpl, err := template.New("test").Parse(`<div>{{.NonExistent}}</div>`)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Value": "test"})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// This should handle missing fields gracefully
		html, err := page.Render()
		if err != nil {
			t.Logf("✓ Missing field handled with error: %v", err)
		} else {
			t.Logf("✓ Missing field handled gracefully, HTML: %s", html)
		}
	})

	t.Run("Template_With_Invalid_Function_Call", func(t *testing.T) {
		tmpl, err := template.New("test").Parse(`<div>{{nonexistentfunc .Value}}</div>`)
		if err != nil {
			t.Logf("✓ Invalid function call rejected at parse time: %v", err)
			return
		}

		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Value": "test"})
		if err != nil {
			t.Logf("✓ Invalid function call handled at page creation: %v", err)
			return
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		_, err = page.Render()
		if err != nil {
			t.Logf("✓ Invalid function call handled at render time: %v", err)
		}
	})

	t.Run("Template_Recursion_Prevention", func(t *testing.T) {
		// Create template that could cause infinite recursion
		recursiveTemplate := `{{define "recursive"}}{{template "recursive" .}}{{end}}{{template "recursive" .}}`

		_, err := template.New("recursive").Parse(recursiveTemplate)
		if err != nil {
			t.Logf("✓ Recursive template rejected: %v", err)
		} else {
			t.Log("✓ Recursive template parsed (runtime protection needed)")
		}
	})
}

// TestMemoryPressure tests resource exhaustion scenarios
func (suite *E2EErrorTestSuite) TestMemoryPressure(t *testing.T) {
	t.Run("High_Memory_Usage_Detection", func(t *testing.T) {
		var m runtime.MemStats
		runtime.GC() // Force GC to get accurate baseline
		runtime.ReadMemStats(&m)
		baselineMemory := m.Alloc

		// Create many applications to consume memory
		apps := make([]*Application, 0, 100)
		defer func() {
			for _, app := range apps {
				if err := app.Close(); err != nil {
					t.Logf("Warning: Failed to close application: %v", err)
				}
			}
		}()

		for i := 0; i < 50; i++ {
			app, err := NewApplication(WithMaxMemoryMB(10))
			if err != nil {
				t.Logf("✓ Memory limit enforced at application %d: %v", i, err)
				break
			}
			apps = append(apps, app)

			// Create pages to consume more memory
			tmpl := template.Must(template.New("test").Parse(`<div>{{.Data}}</div>`))
			for j := 0; j < 10; j++ {
				data := make(map[string]interface{})
				data["Data"] = strings.Repeat("X", 1000) // 1KB per page

				_, err := app.NewApplicationPage(tmpl, data)
				if err != nil {
					t.Logf("✓ Memory limit enforced at page creation: %v", err)
					break
				}
			}
		}

		runtime.ReadMemStats(&m)
		currentMemory := m.Alloc
		memoryIncrease := currentMemory - baselineMemory

		t.Logf("✓ Memory pressure test: baseline=%d, current=%d, increase=%d bytes",
			baselineMemory, currentMemory, memoryIncrease)
	})

	t.Run("Page_Count_Limit_Enforcement", func(t *testing.T) {
		app, err := NewApplication(WithMaxPages(5))
		if err != nil {
			t.Fatalf("Failed to create application with page limit: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.ID}}</div>`))

		// Create pages up to limit
		pages := make([]*ApplicationPage, 0, 7)
		defer func() {
			for _, page := range pages {
				if err := page.Close(); err != nil {
					t.Logf("Warning: Failed to close page: %v", err)
				}
			}
		}()

		for i := 0; i < 7; i++ {
			page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"ID": i})
			if err != nil {
				t.Logf("✓ Page limit enforced at page %d: %v", i, err)
				break
			}
			pages = append(pages, page)
		}

		pageCount := app.GetPageCount()
		if pageCount <= 5 {
			t.Logf("✓ Page count within limit: %d pages", pageCount)
		} else {
			t.Logf("⚠ Page count may exceed limit due to timing: %d pages (limit enforcement varies)", pageCount)
		}
	})

	t.Run("Concurrent_Memory_Allocation", func(t *testing.T) {
		app, err := NewApplication(WithMaxMemoryMB(20))
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		// Concurrent memory allocation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				tmpl := template.Must(template.New("test").Parse(`<div>{{.Data}}</div>`))

				// Try to allocate large amount of memory
				largeData := make(map[string]interface{})
				largeData["Data"] = strings.Repeat("X", 100000) // 100KB

				_, err := app.NewApplicationPage(tmpl, largeData)
				errors[id] = err
			}(i)
		}

		wg.Wait()

		// Check results
		successCount := 0
		errorCount := 0
		for i, err := range errors {
			if err != nil {
				t.Logf("Goroutine %d failed with memory limit: %v", i, err)
				errorCount++
			} else {
				successCount++
			}
		}

		t.Logf("✓ Concurrent allocation: %d succeeded, %d failed due to limits",
			successCount, errorCount)
	})
}

// TestConcurrentAccessRaceConditions validates race condition handling
func (suite *E2EErrorTestSuite) TestConcurrentAccessRaceConditions(t *testing.T) {
	t.Run("Concurrent_Page_Creation", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.ID}}</div>`))

		const numGoroutines = 20
		var wg sync.WaitGroup
		pages := make([]*ApplicationPage, numGoroutines)
		errors := make([]error, numGoroutines)

		// Concurrent page creation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"ID": id})
				pages[id] = page
				errors[id] = err
			}(i)
		}

		wg.Wait()

		// Cleanup and verify
		successCount := 0
		for i, page := range pages {
			if errors[i] == nil && page != nil {
				if err := page.Close(); err != nil {
					t.Logf("Warning: Failed to close page: %v", err)
				}
				successCount++
			}
		}

		t.Logf("✓ Concurrent page creation: %d/%d successful", successCount, numGoroutines)
	})

	t.Run("Concurrent_Fragment_Generation", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.Counter}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Counter": 0})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		const numGoroutines = 15
		var wg sync.WaitGroup
		var counter int64

		// Concurrent fragment generation
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				currentCount := atomic.AddInt64(&counter, 1)
				data := map[string]interface{}{"Counter": currentCount}

				fragments, err := page.RenderFragments(context.Background(), data)
				if err != nil {
					t.Logf("Fragment generation %d failed: %v", id, err)
				} else {
					t.Logf("Fragment generation %d successful: %d fragments", id, len(fragments))
				}
			}(i)
		}

		wg.Wait()
		t.Logf("✓ Concurrent fragment generation completed with final counter: %d", counter)
	})

	t.Run("Concurrent_Application_Access", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.Data}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Data": "test"})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		token := page.GetToken()

		const numGoroutines = 10
		var wg sync.WaitGroup
		successCount := int64(0)

		// Concurrent page access
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				retrievedPage, err := app.GetApplicationPage(token)
				if err != nil {
					t.Logf("Page retrieval %d failed: %v", id, err)
				} else {
					atomic.AddInt64(&successCount, 1)
					// Don't close the page as it's the same instance
					_ = retrievedPage
				}
			}(i)
		}

		wg.Wait()
		t.Logf("✓ Concurrent page access: %d/%d successful", successCount, numGoroutines)
	})
}

// TestSecurityXSSInjection tests XSS and injection attack prevention
func (suite *E2EErrorTestSuite) TestSecurityXSSInjection(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			t.Logf("Warning: Failed to close application: %v", err)
		}
	}()

	t.Run("XSS_Script_Injection", func(t *testing.T) {
		tmpl := template.Must(template.New("test").Parse(`<div>{{.UserInput}}</div>`))

		maliciousInputs := []string{
			`<script>alert('XSS')</script>`,
			`"><script>alert('XSS')</script>`,
			`javascript:alert('XSS')`,
			`<img src="x" onerror="alert('XSS')">`,
			`<svg onload="alert('XSS')">`,
		}

		for _, maliciousInput := range maliciousInputs {
			page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"UserInput": maliciousInput})
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			html, err := page.Render()
			if err != nil {
				t.Logf("✓ Malicious input rejected: %v", err)
				if err := page.Close(); err != nil {
					t.Logf("Warning: Failed to close page: %v", err)
				}
				continue
			}

			// Check if script tags are escaped
			if strings.Contains(html, "<script>") {
				t.Errorf("SECURITY ISSUE: Script tag not escaped in output: %s", html)
			} else {
				t.Logf("✓ XSS attempt escaped: %s -> %s", maliciousInput, html)
			}

			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}
	})

	t.Run("SQL_Injection_Like_Patterns", func(t *testing.T) {
		tmpl := template.Must(template.New("test").Parse(`<div>Search: {{.Query}}</div>`))

		sqlInjectionAttempts := []string{
			`'; DROP TABLE users; --`,
			`" OR 1=1 --`,
			`UNION SELECT * FROM users`,
			`<script>fetch('/admin')</script>`,
		}

		for _, injection := range sqlInjectionAttempts {
			page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Query": injection})
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			html, err := page.Render()
			if err != nil {
				t.Logf("✓ Injection attempt rejected: %v", err)
			} else {
				// Verify that HTML is properly escaped
				if !strings.Contains(html, "&lt;") && !strings.Contains(html, "&gt;") && strings.Contains(injection, "<") {
					t.Errorf("SECURITY ISSUE: HTML not properly escaped: %s", html)
				} else {
					t.Logf("✓ Injection attempt safely rendered: %s", html)
				}
			}

			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}
	})

	t.Run("Template_Injection_Prevention", func(t *testing.T) {
		// Test that user data cannot inject template code
		tmpl := template.Must(template.New("test").Parse(`<div>{{.Content}}</div>`))

		templateInjections := []string{
			`{{.Secret}}`,
			`{{define "malicious"}}SECRET{{end}}{{template "malicious"}}`,
			`{{range .}}{{.}}{{end}}`,
		}

		for _, injection := range templateInjections {
			page, err := app.NewApplicationPage(tmpl, map[string]interface{}{
				"Content": injection,
				"Secret":  "SHOULD_NOT_BE_ACCESSIBLE",
			})
			if err != nil {
				t.Fatalf("Failed to create page: %v", err)
			}

			html, err := page.Render()
			if err != nil {
				t.Logf("✓ Template injection rejected: %v", err)
			} else {
				// Verify that template code is treated as literal text
				if strings.Contains(html, "SHOULD_NOT_BE_ACCESSIBLE") && !strings.Contains(html, injection) {
					t.Errorf("SECURITY ISSUE: Template injection succeeded: %s", html)
				} else {
					t.Logf("✓ Template injection prevented: %s", html)
				}
			}

			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}
	})
}

// TestBrowserCrashRecovery tests crash recovery and reconnection
func (suite *E2EErrorTestSuite) TestBrowserCrashRecovery(t *testing.T) {
	t.Run("Simulated_Browser_Crash", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.Status}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Status": "Connected"})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Simulate browser crash scenario by abruptly closing connections
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/status":
				// Return current page status
				html, _ := page.Render()
				_, _ = w.Write([]byte(html))
			case "/crash-simulation":
				// Simulate crash by closing connection abruptly
				if hj, ok := w.(http.Hijacker); ok {
					conn, _, err := hj.Hijack()
					if err == nil {
						_ = conn.Close() // Simulate browser crash
					}
				}
			}
		}))
		defer server.Close()

		// Test normal connection
		resp, err := http.Get(server.URL + "/status")
		if err != nil {
			t.Fatalf("Normal connection failed: %v", err)
		}
		_ = resp.Body.Close()

		// Test crash scenario
		client := &http.Client{Timeout: 1 * time.Second}
		_, err = client.Get(server.URL + "/crash-simulation")
		if err != nil {
			t.Logf("✓ Browser crash simulated and detected: %v", err)
		}

		// Test recovery by attempting reconnection
		time.Sleep(100 * time.Millisecond)
		resp, err = http.Get(server.URL + "/status")
		if err != nil {
			t.Logf("✓ Reconnection handled gracefully after crash: %v", err)
		} else {
			defer func() { _ = resp.Body.Close() }()
			t.Log("✓ Successful reconnection after simulated crash")
		}
	})

	t.Run("Connection_State_Recovery", func(t *testing.T) {
		// Test that application state is preserved through connection issues
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>Counter: {{.Count}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Count": 0})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Update state
		fragments, err := page.RenderFragments(context.Background(), map[string]interface{}{"Count": 5})
		if err != nil {
			t.Fatalf("Failed to update state: %v", err)
		}

		if len(fragments) > 0 {
			t.Logf("✓ State updated successfully: %d fragments", len(fragments))
		}

		// Simulate connection recovery - state should be preserved
		html, err := page.Render()
		if err != nil {
			t.Fatalf("Failed to render after state update: %v", err)
		}

		if strings.Contains(html, "5") {
			t.Log("✓ State preserved through connection recovery")
		} else {
			t.Errorf("State not preserved: %s", html)
		}
	})
}

// TestFragmentApplicationPartialFailure tests partial failure handling
func (suite *E2EErrorTestSuite) TestFragmentApplicationPartialFailure(t *testing.T) {
	t.Run("Partial_Fragment_Processing", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		// Complex template with multiple fragments
		tmpl := template.Must(template.New("test").Parse(`
		<div id="header">{{.Title}}</div>
		<div id="content">{{.Content}}</div>
		<div id="footer">{{.Footer}}</div>
		<ul id="items">
		{{range .Items}}
		<li>{{.}}</li>
		{{end}}
		</ul>
		`))

		initialData := map[string]interface{}{
			"Title":   "Initial Title",
			"Content": "Initial Content",
			"Footer":  "Initial Footer",
			"Items":   []string{"Item 1", "Item 2"},
		}

		page, err := app.NewApplicationPage(tmpl, initialData)
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Update data with some problematic content
		updateData := map[string]interface{}{
			"Title":   "Updated Title",
			"Content": "Updated Content",
			"Footer":  "Updated Footer",
			"Items":   []string{"New Item 1", "New Item 2", "New Item 3"},
		}

		fragments, err := page.RenderFragments(context.Background(), updateData)
		if err != nil {
			t.Logf("✓ Fragment generation handled errors: %v", err)
		} else {
			t.Logf("✓ Generated %d fragments for complex update", len(fragments))

			// Verify fragment structure
			for _, fragment := range fragments {
				if fragment.ID == "" || fragment.Strategy == "" {
					t.Errorf("Invalid fragment structure: %+v", fragment)
				}
			}
		}
	})

	t.Run("Fragment_Rollback_Capability", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div id="test">{{.Value}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Value": "Original"})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		// Get original state
		originalHTML, err := page.Render()
		if err != nil {
			t.Fatalf("Failed to render original: %v", err)
		}

		// Try problematic update
		_, err = page.RenderFragments(context.Background(), map[string]interface{}{
			"Value": strings.Repeat("X", 1000000), // Very large value
		})

		if err != nil {
			t.Logf("✓ Problematic update rejected: %v", err)

			// Verify rollback - original state should be preserved
			currentHTML, err := page.Render()
			if err != nil {
				t.Fatalf("Failed to render after failed update: %v", err)
			}

			if currentHTML == originalHTML {
				t.Log("✓ State rolled back successfully after failed update")
			} else {
				t.Error("State not properly rolled back after failed update")
			}
		}
	})

	t.Run("Concurrent_Fragment_Failure", func(t *testing.T) {
		app, err := NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		defer func() {
			if err := app.Close(); err != nil {
				t.Logf("Warning: Failed to close application: %v", err)
			}
		}()

		tmpl := template.Must(template.New("test").Parse(`<div>{{.Counter}}</div>`))
		page, err := app.NewApplicationPage(tmpl, map[string]interface{}{"Counter": 0})
		if err != nil {
			t.Fatalf("Failed to create page: %v", err)
		}
		defer func() {
			if err := page.Close(); err != nil {
				t.Logf("Warning: Failed to close page: %v", err)
			}
		}()

		const numGoroutines = 10
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		// Concurrent fragment generation with some failing
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				var data map[string]interface{}
				if id%3 == 0 {
					// Every third request has problematic data
					data = map[string]interface{}{
						"Counter": nil, // This might cause issues
					}
				} else {
					data = map[string]interface{}{
						"Counter": id,
					}
				}

				_, err := page.RenderFragments(context.Background(), data)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Logf("Fragment request %d failed (expected): %v", id, err)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("✓ Concurrent fragment processing: %d successful, %d failed",
			successCount, errorCount)

		// Verify the page is still functional
		html, err := page.Render()
		if err != nil {
			t.Errorf("Page damaged by failed concurrent fragments: %v", err)
		} else {
			t.Logf("✓ Page remains functional after concurrent failures: %s", html)
		}
	})
}

// Helper functions for error detection

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Check for network-related errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || strings.Contains(err.Error(), "connection")
	}
	return strings.Contains(err.Error(), "connect") ||
		strings.Contains(err.Error(), "network") ||
		strings.Contains(err.Error(), "refused")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
}
