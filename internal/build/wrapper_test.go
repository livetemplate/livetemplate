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
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
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

	// Fragment without body tag should return as-is
	if result != fragment {
		t.Errorf("Expected fragment unchanged, got: %q", result)
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
