package build

import (
	"fmt"
	"math/rand"
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

// trickyHeadDecoyCases enumerates <head> contents whose literal text contains
// substrings ("<body", "<script", "</body>") that a naive strings.Index scan
// would mis-match as real tags. Each case is mirrored across InjectWrapperDiv
// and ExtractTemplateBodyContent tests below.
var trickyHeadDecoyCases = []struct {
	name string
	head string
}{
	{"css comment with <body>", `<style>/* <body> is bad */ .x { color: red; }</style>`},
	{"prereview reproducer comment", `<style>/* the wrapper sits between <body> and our layout */</style>`},
	{"inline JS string with <body", `<script>var x = "<body";</script>`},
	{"inline JS with </body>", `<script>var x = "</body>";</script>`},
	{"meta og:description with <body", `<meta property="og:description" content="contains <body literally">`},
	{"meta content with full <body> tag", `<meta name="x" content="<body></body>">`},
	{"HTML comment with <body>", `<!-- <body> note -->`},
	{"HTML comment with </body>", `<!-- </body> -->`},
	{"title text with <body>", `<title>About the <body> tag</title>`},
	{"style containing fake <script>", `<style>/* <script>alert(1)</script> */</style>`},
	{"multiple decoy contexts", `<style>/* <body> */</style><meta content="<body"><script>var s="<body";</script><!-- <body> -->`},
}

// TestInjectWrapperDiv_HeadDecoys exercises the string-fallback path
// (triggered by {{...}} in htmlDoc) with <head> contents that contain
// literal "<body" / "</body>" substrings. The wrapper must end up around
// the real <body> content, not at the first text match in <head>.
func TestInjectWrapperDiv_HeadDecoys(t *testing.T) {
	const sentinel = `<button data-real="x" lvt-on:click="bump">click</button>`
	const wrapperID = "lvt-test"
	for _, tc := range trickyHeadDecoyCases {
		t.Run(tc.name, func(t *testing.T) {
			// {{.X}} forces the string-fallback path (the one with the bug).
			doc := `<!DOCTYPE html><html><head>` + tc.head + `</head><body>` + sentinel + `{{.X}}</body></html>`

			result := InjectWrapperDiv(doc, wrapperID, false)

			wrapperOpen := strings.Index(result, `<div data-lvt-id="`+wrapperID+`"`)
			sentinelPos := strings.Index(result, sentinel)
			if wrapperOpen < 0 {
				t.Fatalf("wrapper div absent from result:\n%s", result)
			}
			if sentinelPos < 0 {
				t.Fatalf("sentinel absent from result:\n%s", result)
			}
			if sentinelPos < wrapperOpen {
				t.Fatalf("sentinel ended up BEFORE wrapper open — wrapper mis-positioned\nsentinelPos=%d wrapperOpen=%d\n%s",
					sentinelPos, wrapperOpen, result)
			}
			// The wrapper's open tag ends at the first '>' after wrapperOpen.
			relClose := strings.Index(result[wrapperOpen:], "</div>")
			if relClose < 0 {
				t.Fatalf("wrapper has no closing </div>:\n%s", result)
			}
			wrapperClose := wrapperOpen + relClose
			if sentinelPos >= wrapperClose {
				t.Fatalf("sentinel ended up AFTER wrapper close\nsentinelPos=%d wrapperClose=%d\n%s",
					sentinelPos, wrapperClose, result)
			}
		})
	}
}

// TestExtractTemplateBodyContent_HeadDecoys mirrors the wrapper test for the
// source-template body-extraction path (called from tree generation). The
// extracted content must contain the real body content and must NOT include
// head markup.
func TestExtractTemplateBodyContent_HeadDecoys(t *testing.T) {
	const sentinel = `<button data-real="x">click</button>`
	for _, tc := range trickyHeadDecoyCases {
		t.Run(tc.name, func(t *testing.T) {
			doc := `<!DOCTYPE html><html><head>` + tc.head + `</head><body>` + sentinel + `</body></html>`

			content := ExtractTemplateBodyContent(doc)

			if !strings.Contains(content, sentinel) {
				t.Errorf("extracted body content missing sentinel\nhead=%q\ngot=%q", tc.head, content)
			}
			// Head markup must not leak into the extracted body content.
			if strings.Contains(content, "<head>") || strings.Contains(content, "<meta") || strings.Contains(content, "<title>") {
				t.Errorf("extracted body content contains <head> markup\nhead=%q\ngot=%q", tc.head, content)
			}
		})
	}
}

// TestInjectWrapperDiv_ScriptSubstringInStyle confirms a literal "<script>"
// appearing inside <style> text in <head> is NOT mis-detected as a script
// tag when splitting body content from scripts.
func TestInjectWrapperDiv_ScriptSubstringInStyle(t *testing.T) {
	doc := `<!DOCTYPE html><html><head><style>/* fake <script>alert(1)</script> */</style></head><body><div>content</div>{{.X}}</body></html>`

	result := InjectWrapperDiv(doc, "lvt-test", false)

	wrapperOpen := strings.Index(result, `<div data-lvt-id="lvt-test"`)
	contentPos := strings.Index(result, `<div>content</div>`)
	if wrapperOpen < 0 || contentPos < 0 || contentPos < wrapperOpen {
		t.Fatalf("real body content was not wrapped properly:\n%s", result)
	}
}

// TestInjectWrapperDiv_PropertyDecoys is a randomized regression test:
// no combination of decoy <body / <script substrings in <head> should ever
// fool the wrapper into mis-positioning. Catches future regressions toward
// naive strings.Index by asserting both that the sentinel is wrapped AND
// that the wrapper appears strictly after the real <body> open tag.
func TestInjectWrapperDiv_PropertyDecoys(t *testing.T) {
	const sentinel = `<button data-real="x">go</button>`
	const wrapperID = "lvt-test"
	const realBodyMarker = `</head><body>`

	decoys := []string{
		`<style>/* <body> */</style>`,
		`<style>/* <body */</style>`,
		`<script>var s="<body>";</script>`,
		`<script>// </body></script>`,
		`<meta property="og:title" content="<body">`,
		`<meta content="<body>">`,
		`<!-- <body> -->`,
		`<!-- </body> -->`,
		`<title>About <body></title>`,
		`<noscript><body></noscript>`,
	}

	// Deterministic seed so failures reproduce; bump to flush out new patterns.
	const seed = int64(0xc0ffee10)
	rng := rand.New(rand.NewSource(seed))

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		n := rng.Intn(len(decoys) + 1)
		perm := rng.Perm(len(decoys))
		var head strings.Builder
		for _, k := range perm[:n] {
			head.WriteString(decoys[k])
		}
		doc := `<!DOCTYPE html><html><head>` + head.String() + `</head><body>` + sentinel + `{{.X}}</body></html>`

		result := InjectWrapperDiv(doc, wrapperID, false)
		wrapperOpen := strings.Index(result, `<div data-lvt-id="`+wrapperID+`"`)
		sentinelPos := strings.Index(result, sentinel)
		realBodyPos := strings.Index(result, realBodyMarker)
		if wrapperOpen < 0 || sentinelPos < 0 || realBodyPos < 0 {
			t.Fatalf("iter %d (seed=%#x): missing landmark — wrapperOpen=%d sentinelPos=%d realBodyPos=%d\nhead=%q\nresult=%s",
				i, seed, wrapperOpen, sentinelPos, realBodyPos, head.String(), result)
		}
		realBodyOpenEnd := realBodyPos + len(realBodyMarker)
		// The wrapper must appear strictly after the real <body...> open
		// tag, not inside <head> even if the sentinel happens to also fall
		// after the wrapper. This is the regression-catching assertion.
		if wrapperOpen < realBodyOpenEnd {
			t.Fatalf("iter %d (seed=%#x): wrapper at %d is BEFORE real <body> at %d (mis-positioned inside <head>)\nhead=%q\nresult=%s",
				i, seed, wrapperOpen, realBodyOpenEnd, head.String(), result)
		}
		if sentinelPos < wrapperOpen {
			t.Fatalf("iter %d (seed=%#x): sentinel at %d is BEFORE wrapper at %d\nhead=%q\nresult=%s",
				i, seed, sentinelPos, wrapperOpen, head.String(), result)
		}
	}
}

// TestLocateBodyAndFirstScript_OffsetCoverage proves the offset accounting in
// locateBodyAndFirstScript is exact: every byte of input is consumed by
// exactly one token's Raw(). If this ever drifts, every byte offset returned
// by the helper is wrong.
func TestLocateBodyAndFirstScript_OffsetCoverage(t *testing.T) {
	inputs := []string{
		`<!DOCTYPE html><html><head><title>X</title></head><body><div>Hello</div></body></html>`,
		`<html><body><div>{{.X}}</div><script>console.log("ok")</script></body></html>`,
		`<html><head><style>/* <body> */</style></head><body><p>x</p></body></html>`,
		``,
		`not html at all`,
		`<html><head><script src="a.js"></script></head><body class="dark">{{range .Items}}<li>{{.}}</li>{{end}}</body></html>`,
		`<!-- comment --><html><body>x</body></html>`,
	}
	for i, input := range inputs {
		t.Run(fmt.Sprintf("input_%d", i), func(t *testing.T) {
			z := html.NewTokenizer(strings.NewReader(input))
			sum := 0
			for {
				tt := z.Next()
				if tt == html.ErrorToken {
					break
				}
				sum += len(z.Raw())
			}
			if sum != len(input) {
				t.Errorf("offset coverage drift: sum=%d want=%d input=%q", sum, len(input), input)
			}
		})
	}
}

// TestLocateBodyAndFirstScript_NestedBody_BehaviorPin pins the strings.LastIndex
// semantics for </body> in malformed nested-body input, preserving backward
// compatibility with the previous string-based implementation.
func TestLocateBodyAndFirstScript_NestedBody_BehaviorPin(t *testing.T) {
	input := `<html><body><body>Content</body></body></html>`
	bodyOpen, _, bodyClose, _ := locateBodyAndFirstScript(input)

	if want := strings.Index(input, "<body>"); bodyOpen != want {
		t.Errorf("bodyOpen = %d, want %d (first <body> wins)", bodyOpen, want)
	}
	if want := strings.LastIndex(input, "</body>"); bodyClose != want {
		t.Errorf("bodyClose = %d, want %d (last </body> wins, matching strings.LastIndex)", bodyClose, want)
	}
}

// TestLocateBodyAndFirstScript_TemplateDirectiveInAttrValue_Safe pins the
// safe template-directive case: a {{...}} inside a quoted attribute value
// is opaque to html.Tokenizer, and the body open tag is located correctly.
func TestLocateBodyAndFirstScript_TemplateDirectiveInAttrValue_Safe(t *testing.T) {
	input := `<html><head></head><body class="{{.Theme}}"><div>x</div></body></html>`

	bodyOpenStart, bodyOpenEnd, bodyCloseStart, _ := locateBodyAndFirstScript(input)

	if got, want := bodyOpenStart, strings.Index(input, "<body"); got != want {
		t.Errorf("bodyOpenStart = %d, want %d", got, want)
	}
	wantOpenEnd := strings.Index(input, `<div>`)
	if bodyOpenEnd != wantOpenEnd {
		t.Errorf("bodyOpenEnd = %d, want %d (start of <div> after body open tag)", bodyOpenEnd, wantOpenEnd)
	}
	if got, want := bodyCloseStart, strings.Index(input, "</body>"); got != want {
		t.Errorf("bodyCloseStart = %d, want %d", got, want)
	}
}

// TestLocateBodyAndFirstScript_TemplateDirectiveInBodyTag_LimitationPin pins
// today's behavior on a pathological template directive that emits attribute-
// like text in the body open tag itself. Behavior matches the previous
// string-based implementation; a robust fix would require Go-template-aware
// preprocessing, intentionally out of scope here.
func TestLocateBodyAndFirstScript_TemplateDirectiveInBodyTag_LimitationPin(t *testing.T) {
	input := `<html><body {{if .Dark}}class="dark"{{end}}><div>x</div></body></html>`

	bodyOpenStart, bodyOpenEnd, _, _ := locateBodyAndFirstScript(input)

	if got, want := bodyOpenStart, strings.Index(input, "<body"); got != want {
		t.Errorf("bodyOpenStart drift: got %d, want %d", got, want)
	}
	// {{...}} contains no '>' character, so the tokenizer's start-tag end
	// scan correctly finds the body tag's closing '>'. If a future change
	// alters template-directive handling, update this pin deliberately.
	wantOpenEnd := strings.Index(input, "<div>")
	if bodyOpenEnd != wantOpenEnd {
		t.Errorf("limitation drift: bodyOpenEnd = %d, want %d (one past first '>' after <body)",
			bodyOpenEnd, wantOpenEnd)
	}
}

// TestLocateBodyAndFirstScript_GoTemplateComment_LimitationPin pins the
// documented residual limitation: html.Tokenizer is unaware of Go template
// comments {{/* ... */}}, so a literal "<body>" inside one still matches as
// a tag. Fixing this would require Go-template-syntax stripping before
// tokenization, intentionally out of scope here.
func TestLocateBodyAndFirstScript_GoTemplateComment_LimitationPin(t *testing.T) {
	input := `<html><head>{{/* <body> */}}</head><body>real</body></html>`

	bodyOpen, _, _, _ := locateBodyAndFirstScript(input)

	// Tokenizer sees the inner literal "<body>" as a start tag — this PINS
	// the known limitation. If a future change adds Go-template-comment
	// stripping, update this test deliberately.
	want := strings.Index(input, "<body>")
	if bodyOpen != want {
		t.Errorf("limitation drift: bodyOpen = %d, want %d (first textual <body>, including inside {{/* */}})",
			bodyOpen, want)
	}
}
