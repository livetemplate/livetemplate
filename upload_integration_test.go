package livetemplate

import (
	"bytes"
	"context"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestStore implements UploadAware for integration testing
type TestUploadStore struct {
	uploadsCalled    bool
	consumedEntries  []*UploadEntry
	consumedUpload   string
	consumeError     error
	allowedUploads   map[string]UploadConfig
}

func (s *TestUploadStore) AllowUploads() map[string]UploadConfig {
	if s.allowedUploads == nil {
		s.allowedUploads = map[string]UploadConfig{
			"avatar": {
				Accept:      []string{"image/*"},
				MaxEntries:  1,
				MaxFileSize: 1024 * 1024, // 1MB
			},
			"documents": {
				Accept:      []string{".pdf", ".txt"},
				MaxEntries:  5,
				MaxFileSize: 5 * 1024 * 1024, // 5MB
			},
		}
	}
	return s.allowedUploads
}

func (s *TestUploadStore) ConsumeUpload(ctx context.Context, name string, entries []*UploadEntry) error {
	s.uploadsCalled = true
	s.consumedUpload = name
	s.consumedEntries = entries
	return s.consumeError
}

func (s *TestUploadStore) Change(ctx *ActionContext) error {
	return nil
}

// TestHTTPUploadFlow tests the complete HTTP upload flow
func TestHTTPUploadFlow(t *testing.T) {
	// Create a template with upload helpers
	tmplStr := `
{{range .lvt.Uploads "avatar"}}
<div class="upload">{{.ClientName}}: {{.Progress}}%</div>
{{end}}
{{if .lvt.HasUploadError "avatar"}}
<div class="error">{{.lvt.UploadError "avatar"}}</div>
{{end}}
`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create test store
	store := &TestUploadStore{}

	// Create handler
	handler := tmpl.Handle(store)

	// Create multipart form with file upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add a file
	part, err := writer.CreateFormFile("avatar", "test.jpg")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	testContent := []byte("fake jpeg content")
	if _, err := part.Write(testContent); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}

	// Add action field
	writer.WriteField("action", "submit")

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create response recorder
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ConsumeUpload was called
	if !store.uploadsCalled {
		t.Error("Expected ConsumeUpload to be called")
	}

	// Verify correct upload name
	if store.consumedUpload != "avatar" {
		t.Errorf("Expected upload name 'avatar', got %q", store.consumedUpload)
	}

	// Verify entries were consumed
	if len(store.consumedEntries) != 1 {
		t.Errorf("Expected 1 consumed entry, got %d", len(store.consumedEntries))
	}

	// Verify entry details
	if len(store.consumedEntries) > 0 {
		entry := store.consumedEntries[0]
		if entry.ClientName != "test.jpg" {
			t.Errorf("Expected filename 'test.jpg', got %q", entry.ClientName)
		}
		if !entry.Valid {
			t.Errorf("Expected entry to be valid, got error: %s", entry.Error)
		}
		if !entry.Done {
			t.Error("Expected entry to be marked as done")
		}
		if entry.TempPath == "" {
			t.Error("Expected entry to have temp path")
		}

		// Verify temp file exists and has correct content
		content, err := os.ReadFile(entry.TempPath)
		if err != nil {
			t.Errorf("Failed to read temp file: %v", err)
		}
		if !bytes.Equal(content, testContent) {
			t.Errorf("Expected content %q, got %q", testContent, content)
		}
	}
}

// TestUploadTemplateDisplay tests upload entry display in templates
func TestUploadTemplateDisplay(t *testing.T) {
	// Create template that displays upload entries
	tmplStr := `
{{range .lvt.Uploads "documents"}}
File: {{.ClientName}} ({{.ClientSize}} bytes)
{{if .Valid}}Valid{{else}}Invalid: {{.Error}}{{end}}
Progress: {{.Progress}}%
{{end}}
`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Create test store
	store := &TestUploadStore{}

	// Create handler
	handler := tmpl.Handle(store)

	// Create multipart form with multiple files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add valid PDF file
	part1, _ := writer.CreateFormFile("documents", "doc1.pdf")
	part1.Write([]byte("PDF content"))

	// Add valid text file
	part2, _ := writer.CreateFormFile("documents", "doc2.txt")
	part2.Write([]byte("Text content"))

	writer.WriteField("action", "submit")
	writer.Close()

	// Create and serve request
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check response contains file information
	responseBody := w.Body.String()

	if !strings.Contains(responseBody, "doc1.pdf") {
		t.Error("Expected response to contain 'doc1.pdf'")
	}
	if !strings.Contains(responseBody, "doc2.txt") {
		t.Error("Expected response to contain 'doc2.txt'")
	}
	if !strings.Contains(responseBody, "Valid") {
		t.Error("Expected files to be marked as valid")
	}
	if !strings.Contains(responseBody, "100%") {
		t.Error("Expected progress to show 100%")
	}
}

// TestUploadErrorHandling tests error handling and display
func TestUploadErrorHandling(t *testing.T) {
	// Create template that displays errors
	tmplStr := `
{{if .lvt.HasUploadError "avatar"}}
Error: {{.lvt.UploadError "avatar"}}
{{end}}
{{range .lvt.Uploads "avatar"}}
{{if .Error}}File {{.ClientName}}: {{.Error}}{{end}}
{{end}}
`

	tmpl := Must(New("test"))
	if _, err := tmpl.Parse(tmplStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	store := &TestUploadStore{}
	handler := tmpl.Handle(store)

	// Test 1: Invalid file type
	t.Run("InvalidFileType", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Upload a text file to avatar field (expects images)
		part, _ := writer.CreateFormFile("avatar", "document.txt")
		part.Write([]byte("text content"))

		writer.WriteField("action", "submit")
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		responseBody := w.Body.String()

		// Should contain error about file type
		if !strings.Contains(responseBody, "Error:") {
			t.Error("Expected error to be displayed")
		}
		if !strings.Contains(responseBody, "document.txt") {
			t.Error("Expected filename in error message")
		}
	})

	// Test 2: File too large
	t.Run("FileTooLarge", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("avatar", "huge.jpg")
		// Write more than 1MB (the limit)
		largeContent := make([]byte, 2*1024*1024)
		part.Write(largeContent)

		writer.WriteField("action", "submit")
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		responseBody := w.Body.String()

		// Should contain error about file size
		if !strings.Contains(responseBody, "Error:") {
			t.Error("Expected error to be displayed")
		}
	})

	// Test 3: Too many files
	t.Run("TooManyFiles", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Avatar only allows 1 file, upload 2
		part1, _ := writer.CreateFormFile("avatar", "photo1.jpg")
		part1.Write([]byte("photo1"))
		part2, _ := writer.CreateFormFile("avatar", "photo2.jpg")
		part2.Write([]byte("photo2"))

		writer.WriteField("action", "submit")
		writer.Close()

		req := httptest.NewRequest("POST", "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Request should fail with 400 error (count validation happens before processing)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for too many files, got %d", w.Code)
		}
	})
}

// TestUploadTempFileCleanup tests temp file cleanup
func TestUploadTempFileCleanup(t *testing.T) {
	tmpl := Must(New("test"))
	if _, err := tmpl.Parse("<div>test</div>"); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	store := &TestUploadStore{}
	handler := tmpl.Handle(store)

	// Upload a file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("avatar", "test.jpg")
	part.Write([]byte("content"))
	writer.WriteField("action", "submit")
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Get the temp file path
	var tempPath string
	if len(store.consumedEntries) > 0 {
		tempPath = store.consumedEntries[0].TempPath
	}

	if tempPath == "" {
		t.Fatal("No temp file path found")
	}

	// Verify file exists
	if _, err := os.Stat(tempPath); err != nil {
		t.Errorf("Temp file should exist: %v", err)
	}

	// Note: For HTTP requests, temp files are not auto-cleaned since there's no persistent session.
	// The store's ConsumeUpload should move/copy the file and then the application should clean up.
	// For WebSocket connections, temp files are cleaned up when the connection closes.

	// Clean up manually for test
	os.Remove(tempPath)
}

// TestUploadWithoutFiles tests POST request without file uploads
func TestUploadWithoutFiles(t *testing.T) {
	tmpl := Must(New("test"))
	if _, err := tmpl.Parse("<div>{{.Value}}</div>"); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Use TestUploadStore which implements the Store interface
	simpleStore := &TestUploadStore{}
	handler := tmpl.Handle(simpleStore)

	// Regular form POST without files
	body := strings.NewReader("action=update")
	req := httptest.NewRequest("POST", "/action", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should succeed without trying to process uploads
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestUploadTemplateHelperEdgeCases tests edge cases for template helpers
func TestUploadTemplateHelperEdgeCases(t *testing.T) {
	// Test with no upload registry set
	t.Run("NoUploadRegistry", func(t *testing.T) {
		tmplStr := `
{{range .lvt.Uploads "avatar"}}
  Should not appear
{{end}}
{{if .lvt.HasUploadError "avatar"}}
  Should not appear
{{end}}
Error: "{{.lvt.UploadError "avatar"}}"
`
		goTmpl, err := template.New("test").Parse(tmplStr)
		if err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Execute without upload registry
		data := map[string]interface{}{
			"lvt": struct {
				Uploads        func(string) interface{}
				HasUploadError func(string) bool
				UploadError    func(string) string
			}{
				Uploads:        func(string) interface{} { return nil },
				HasUploadError: func(string) bool { return false },
				UploadError:    func(string) string { return "" },
			},
		}

		var buf bytes.Buffer
		if err := goTmpl.Execute(&buf, data); err != nil {
			t.Fatalf("Template execution failed: %v", err)
		}

		result := buf.String()
		if strings.Contains(result, "Should not appear") {
			t.Error("Template should handle nil upload registry gracefully")
		}
	})

	// Test with non-existent upload name
	t.Run("NonExistentUpload", func(t *testing.T) {
		tmplStr := `
{{range .lvt.Uploads "nonexistent"}}
  Should not appear
{{end}}
`
		tmpl := Must(New("test"))
		if _, err := tmpl.Parse(tmplStr); err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		store := &TestUploadStore{}
		handler := tmpl.Handle(store)

		// Make a GET request (no uploads)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		result := w.Body.String()
		if strings.Contains(result, "Should not appear") {
			t.Error("Template should handle non-existent upload gracefully")
		}
	})
}
