package build

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestGenerateRandomID_Uniqueness tests that generated IDs are unique.
func TestGenerateRandomID_Uniqueness(t *testing.T) {
	// Generate many IDs and check for duplicates
	generated := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := GenerateRandomID()
		if generated[id] {
			t.Fatalf("Duplicate ID generated: %q at iteration %d", id, i)
		}
		generated[id] = true
	}

	if len(generated) != count {
		t.Errorf("Expected %d unique IDs, got: %d", count, len(generated))
	}
}

// TestGenerateRandomID_Format tests ID format is lvt-[random].
func TestGenerateRandomID_Format(t *testing.T) {
	id := GenerateRandomID()

	if !strings.HasPrefix(id, "lvt-") {
		t.Errorf("Expected ID to start with 'lvt-', got: %q", id)
	}

	// Check length (lvt- is 4 chars, 8 bytes hex = 16 chars)
	expectedLength := 4 + 16
	if len(id) != expectedLength {
		t.Errorf("Expected ID length %d, got: %d", expectedLength, len(id))
	}

	// Check hex portion
	hexPart := id[4:]
	for _, c := range hexPart {
		isDigit := c >= '0' && c <= '9'
		isHexLetter := c >= 'a' && c <= 'f'
		if !isDigit && !isHexLetter {
			t.Errorf("Expected hex characters, found: %c in %q", c, id)
			break
		}
	}
}

// TestInjectWrapperDiv_FullDocument tests full HTML document wrapping.
func TestInjectWrapperDiv_FullDocument(t *testing.T) {
	htmlDoc := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<div>Content</div>
</body>
</html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, false)

	// Check wrapper div is present
	if !strings.Contains(result, `data-lvt-id="test-id"`) {
		t.Error("Expected wrapper div with data-lvt-id attribute")
	}

	// Check loading attribute is present
	if !strings.Contains(result, `data-lvt-loading="true"`) {
		t.Error("Expected loading attribute")
	}

	// Check content is inside wrapper
	if !strings.Contains(result, "<div>Content</div>") {
		t.Error("Expected content inside wrapper")
	}

	// Check structure: body should contain wrapper div
	if !strings.Contains(result, "<body>") {
		t.Error("Expected body tag preserved")
	}
}

// TestInjectWrapperDiv_Fragment tests fragment wrapping (no body tag).
func TestInjectWrapperDiv_Fragment(t *testing.T) {
	fragment := `<div>Just a fragment</div>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(fragment, wrapperID, false)

	// With HTML parsing, fragments get auto-wrapped in html/head/body by parser
	// The new implementation will add the wrapper div around the fragment content
	if !strings.Contains(result, `data-lvt-id="test-id"`) {
		t.Error("Expected wrapper div with data-lvt-id attribute")
	}

	// Original fragment content should be preserved
	if !strings.Contains(result, `<div>Just a fragment</div>`) {
		t.Error("Expected original fragment content preserved")
	}
}

// TestInjectWrapperDiv_WithLoading tests loading indicator injection.
func TestInjectWrapperDiv_WithLoading(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div></body></html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, false)

	// Check loading attribute is present
	if !strings.Contains(result, `data-lvt-loading="true"`) {
		t.Error("Expected loading attribute when not disabled")
	}
}

// TestInjectWrapperDiv_LoadingDisabled tests no loading indicator when disabled.
func TestInjectWrapperDiv_LoadingDisabled(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div></body></html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, true)

	// Check loading attribute is NOT present
	if strings.Contains(result, `data-lvt-loading`) {
		t.Error("Expected no loading attribute when disabled")
	}

	// But wrapper div should still be present
	if !strings.Contains(result, `data-lvt-id="test-id"`) {
		t.Error("Expected wrapper div even with loading disabled")
	}
}

// TestInjectWrapperDiv_WithScripts tests script exclusion from wrapper.
func TestInjectWrapperDiv_WithScripts(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div><script>console.log('test');</script></body></html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, false)

	// Check wrapper exists
	if !strings.Contains(result, `data-lvt-id="test-id"`) {
		t.Error("Expected wrapper div")
	}

	// Check script is preserved
	if !strings.Contains(result, "<script>") {
		t.Error("Expected script tag preserved")
	}

	// Script should be outside wrapper (after the wrapper div closing tag)
	wrapperEnd := strings.Index(result, `</div>`)
	scriptStart := strings.Index(result, `<script>`)
	if wrapperEnd == -1 || scriptStart == -1 {
		t.Fatal("Could not find wrapper end or script start")
	}

	if scriptStart < wrapperEnd {
		t.Error("Expected script to be outside (after) wrapper div")
	}
}

// TestExtractTemplateBodyContent tests body content extraction.
func TestExtractTemplateBodyContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"full template",
			`<html><body><div>Content</div></body></html>`,
			`<div>Content</div>`,
		},
		{
			"no body tag",
			`<div>Fragment</div>`,
			`<div>Fragment</div>`,
		},
		{
			"body with whitespace",
			`<html><body>  <div>Content</div>  </body></html>`,
			`<div>Content</div>`,
		},
		{
			"empty body",
			`<html><body></body></html>`,
			``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTemplateBodyContent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestExtractTemplateContent tests content extraction using wrapper ID.
func TestExtractTemplateContent(t *testing.T) {
	// Create HTML with wrapper div
	htmlDoc := `<html><body><div data-lvt-id="wrapper-123"><div>Content</div></div></body></html>`

	result := ExtractTemplateContent(htmlDoc, "wrapper-123")

	expected := "<div>Content</div>"
	if result != expected {
		t.Errorf("Expected %q, got: %q", expected, result)
	}
}

// TestExtractTemplateContent_NoWrapper tests extraction with no wrapper ID.
func TestExtractTemplateContent_NoWrapper(t *testing.T) {
	input := `<div>Fragment</div>`

	// Empty wrapper ID should return as-is
	result := ExtractTemplateContent(input, "")

	if result != input {
		t.Errorf("Expected input unchanged, got: %q", result)
	}
}

// TestExtractTemplateContent_WrapperNotFound tests extraction when wrapper not found.
func TestExtractTemplateContent_WrapperNotFound(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div></body></html>`

	// Non-existent wrapper ID
	result := ExtractTemplateContent(htmlDoc, "non-existent")

	// Should return as-is when wrapper not found
	if result != htmlDoc {
		t.Errorf("Expected input unchanged when wrapper not found")
	}
}

// TestFindElementByDataLvtID tests element finding by lvt ID.
func TestFindElementByDataLvtID(t *testing.T) {
	// Parse HTML
	htmlDoc := `<html><body><div data-lvt-id="target">Content</div></body></html>`
	doc, err := html.Parse(strings.NewReader(htmlDoc))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	// Find element
	element := FindElementByDataLvtID(doc, "target")

	if element == nil {
		t.Fatal("Expected to find element with data-lvt-id='target'")
	}

	if element.Data != "div" {
		t.Errorf("Expected element tag 'div', got: %q", element.Data)
	}

	// Check attribute
	found := false
	for _, attr := range element.Attr {
		if attr.Key == "data-lvt-id" && attr.Val == "target" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected data-lvt-id attribute on found element")
	}
}

// TestFindElementByDataLvtID_NotFound tests element not found.
func TestFindElementByDataLvtID_NotFound(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div></body></html>`
	doc, err := html.Parse(strings.NewReader(htmlDoc))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	// Try to find non-existent element
	element := FindElementByDataLvtID(doc, "non-existent")

	if element != nil {
		t.Error("Expected nil for non-existent element")
	}
}

// TestFindElementByDataLvtID_Nested tests finding nested element.
func TestFindElementByDataLvtID_Nested(t *testing.T) {
	htmlDoc := `<html><body><div><span><div data-lvt-id="nested">Content</div></span></div></body></html>`
	doc, err := html.Parse(strings.NewReader(htmlDoc))
	if err != nil {
		t.Fatalf("Failed to parse HTML: %v", err)
	}

	element := FindElementByDataLvtID(doc, "nested")

	if element == nil {
		t.Fatal("Expected to find nested element")
	}

	if element.Data != "div" {
		t.Errorf("Expected element tag 'div', got: %q", element.Data)
	}
}

// TestNormalizeTemplateSpacing tests whitespace normalization.
func TestNormalizeTemplateSpacing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"if with spaces",
			`{{ if .X }}content{{ end }}`,
			`{{if .X}}content{{end}}`,
		},
		{
			"range with spaces",
			`{{ range .Items }}{{ . }}{{ end }}`,
			`{{range .Items}}{{.}}{{end}}`,
		},
		{
			"field with spaces",
			`{{ .Name }}`,
			`{{.Name}}`,
		},
		{
			"multiple templates",
			`{{ .A }} and {{ .B }}`,
			`{{.A}} and {{.B}}`,
		},
		{
			"already normalized",
			`{{.Name}}`,
			`{{.Name}}`,
		},
		{
			"complex expression",
			`{{ if and .X .Y }}yes{{ end }}`,
			`{{if and .X .Y}}yes{{end}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeTemplateSpacing(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestExtractTemplateBodyContent_WithAttributes tests body tags with attributes.
func TestExtractTemplateBodyContent_WithAttributes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"body with class attribute",
			`<html><body class="dark"><div>Content</div></body></html>`,
			`<div>Content</div>`,
		},
		{
			"body with multiple attributes",
			`<html><body class="dark" id="main"><div>Content</div></body></html>`,
			`<div>Content</div>`,
		},
		{
			"body with data attribute",
			`<html><body data-theme="light"><div>Content</div></body></html>`,
			`<div>Content</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTemplateBodyContent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got: %q", tt.expected, result)
			}
		})
	}
}

// TestInjectWrapperDiv_MultipleScripts tests handling of multiple script tags.
func TestInjectWrapperDiv_MultipleScripts(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div><script>console.log('1');</script><script src="app.js"></script></body></html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, false)

	// Check wrapper exists
	if !strings.Contains(result, `data-lvt-id="test-id"`) {
		t.Error("Expected wrapper div")
	}

	// Check both scripts are preserved
	if !strings.Contains(result, `console.log('1')`) {
		t.Error("Expected first script preserved")
	}
	if !strings.Contains(result, `src="app.js"`) {
		t.Error("Expected second script preserved")
	}

	// Parse result to verify structure
	doc, err := html.Parse(strings.NewReader(result))
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Find wrapper and count scripts
	wrapper := FindElementByDataLvtID(doc, "test-id")
	if wrapper == nil {
		t.Fatal("Could not find wrapper div in result")
	}

	// Scripts should be outside wrapper (siblings, not children)
	scriptCount := 0
	for child := wrapper.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "script" {
			t.Error("Script tag found inside wrapper - should be outside")
		}
	}

	// Count scripts in body (should be siblings of wrapper)
	body := FindBodyNode(doc)
	if body != nil {
		for child := body.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "script" {
				scriptCount++
			}
		}
	}

	if scriptCount != 2 {
		t.Errorf("Expected 2 script tags as siblings of wrapper, got: %d", scriptCount)
	}
}

// TestInjectWrapperDiv_ScriptWithAttributes tests script tag with attributes.
func TestInjectWrapperDiv_ScriptWithAttributes(t *testing.T) {
	htmlDoc := `<html><body><div>Content</div><script type="module" defer src="app.js"></script></body></html>`

	wrapperID := "test-id"
	result := InjectWrapperDiv(htmlDoc, wrapperID, false)

	// Check script with attributes is preserved
	if !strings.Contains(result, `type="module"`) {
		t.Error("Expected type attribute preserved")
	}
	if !strings.Contains(result, `defer`) {
		t.Error("Expected defer attribute preserved")
	}
	if !strings.Contains(result, `src="app.js"`) {
		t.Error("Expected src attribute preserved")
	}
}

// TestNormalizeTemplateSpacing_EdgeCase tests edge case with short match.
func TestNormalizeTemplateSpacing_EdgeCase(t *testing.T) {
	// This shouldn't happen with the regex, but test the guard
	input := `{{}}` // Empty template tag
	result := NormalizeTemplateSpacing(input)

	// Should handle gracefully, not panic
	if result != `{{}}` {
		t.Errorf("Expected %q, got: %q", `{{}}`, result)
	}
}

// TestGenerateRandomID_NoPanic tests that GenerateRandomID doesn't panic under normal conditions.
func TestGenerateRandomID_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GenerateRandomID panicked: %v", r)
		}
	}()

	// Should not panic under normal conditions
	for i := 0; i < 100; i++ {
		id := GenerateRandomID()
		if len(id) != 20 { // lvt- (4) + 16 hex chars
			t.Errorf("Unexpected ID length: %d", len(id))
		}
	}
}

// TestInjectWrapperDiv_MalformedHTML tests graceful handling of malformed HTML.
func TestInjectWrapperDiv_MalformedHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			"unclosed div",
			`<html><body><div>Content</body></html>`,
		},
		{
			"missing closing body",
			`<html><body><div>Content</div>`,
		},
		{
			"nested body tags",
			`<html><body><body>Content</body></body></html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapperID := "test-id"
			result := InjectWrapperDiv(tt.input, wrapperID, false)

			// Should not panic and should produce some result
			if result == "" {
				t.Error("Expected non-empty result for malformed HTML")
			}

			// Should contain wrapper ID in some form
			if !strings.Contains(result, wrapperID) {
				t.Error("Expected wrapper ID in result")
			}
		})
	}
}
