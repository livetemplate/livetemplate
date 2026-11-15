package build

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/livetemplate/livetemplate/internal/render"
	"golang.org/x/net/html"
)

// GenerateRandomID generates a random ID for the wrapper div.
// Panics if crypto/rand fails (extremely rare, indicates serious system issue).
func GenerateRandomID() string {
	const randomIDBytes = 8
	b := make([]byte, randomIDBytes)
	if _, err := cryptorand.Read(b); err != nil {
		// crypto/rand failure is catastrophic - system entropy source is broken
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return "lvt-" + hex.EncodeToString(b)
}

// InjectWrapperDiv injects a wrapper div around body content with the specified ID.
// The trailing run of top-level <script> tags stays after the wrapper, so the
// client bundle a page loads at the end of <body> is not dynamic content. A
// script with content after it is content itself and stays inside, in place:
// moving it out would also move everything after it out of the wrapper.
// Uses proper HTML parsing to handle all script tag variants and malformed HTML gracefully.
func InjectWrapperDiv(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	// If input contains Go template directives, use string-based approach to avoid mangling them
	// html.Render() would escape {{ }} and break template syntax
	if strings.Contains(htmlDoc, "{{") || strings.Contains(htmlDoc, "}}") {
		return injectWrapperDivStringBased(htmlDoc, wrapperID, loadingDisabled)
	}

	// Parse HTML document
	doc, err := html.Parse(strings.NewReader(htmlDoc))
	if err != nil {
		// Parsing failed, fallback to original string-based approach for backward compatibility
		return injectWrapperDivStringBased(htmlDoc, wrapperID, loadingDisabled)
	}

	// Find the body element
	bodyNode := FindBodyNode(doc)
	if bodyNode == nil {
		// No body tag found, return as-is
		return htmlDoc
	}

	// Walk back from the last child to find where the trailing script run starts.
	var trailing *html.Node
	for c := bodyNode.LastChild; c != nil; c = c.PrevSibling {
		isScript := c.Type == html.ElementNode && c.Data == "script"
		if !isScript && !isIgnorableTrailingNode(c) {
			break
		}
		if isScript {
			trailing = c
		}
	}
	var contentNodes, scriptNodes []*html.Node
	for c := bodyNode.FirstChild; c != nil; c = c.NextSibling {
		if c == trailing {
			for ; c != nil; c = c.NextSibling {
				scriptNodes = append(scriptNodes, c)
			}
			break
		}
		contentNodes = append(contentNodes, c)
	}

	// Remove all children from body
	for bodyNode.FirstChild != nil {
		bodyNode.RemoveChild(bodyNode.FirstChild)
	}

	// Create wrapper div
	wrapperDiv := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{Key: "data-lvt-id", Val: wrapperID},
		},
	}

	// Add loading attribute if not disabled
	if !loadingDisabled {
		wrapperDiv.Attr = append(wrapperDiv.Attr, html.Attribute{Key: "data-lvt-loading", Val: "true"})
	}

	// Add content nodes to wrapper
	for _, node := range contentNodes {
		wrapperDiv.AppendChild(node)
	}

	// Add wrapper to body
	bodyNode.AppendChild(wrapperDiv)

	// Add script nodes after wrapper
	for _, node := range scriptNodes {
		bodyNode.AppendChild(node)
	}

	// Render the modified document
	var buf strings.Builder
	if err := html.Render(&buf, doc); err != nil {
		// Rendering failed, return original
		return htmlDoc
	}

	return buf.String()
}

// isIgnorableTrailingNode reports whether n can sit among the trailing scripts
// without making them content: whitespace and comments.
func isIgnorableTrailingNode(n *html.Node) bool {
	switch n.Type {
	case html.CommentNode:
		return true
	case html.TextNode:
		return strings.TrimSpace(n.Data) == ""
	}
	return false
}

// FindBodyNode recursively finds the body element in an HTML document tree.
func FindBodyNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := FindBodyNode(child); found != nil {
			return found
		}
	}
	return nil
}

// locateBodyAndTrailingScripts walks htmlDoc once with html.Tokenizer,
// returning byte offsets for the body open start, body open end, body close
// start, and the start of the trailing run of <script> elements inside body
// content: scripts followed by nothing but whitespace, comments and more
// scripts up to </body>. body close uses LastIndex semantics to match the
// previous string-based implementation on malformed nested-body input. All
// offsets are -1 when not found. Tolerant of {{...}} in text/attribute-value
// positions. Any other tag (including the end tag of an element a script is
// nested in) or non-whitespace text resets the run. A {{...}} directive
// between scripts counts as text, so the scripts before it stay inside.
func locateBodyAndTrailingScripts(htmlDoc string) (int, int, int, int) {
	bodyOpenStart, bodyOpenEnd, bodyCloseStart, trailingStart := -1, -1, -1, -1
	inScript := false
	z := html.NewTokenizer(strings.NewReader(htmlDoc))
	offset := 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		raw := z.Raw()
		inBody := bodyOpenEnd >= 0 && bodyCloseStart < 0
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			switch {
			case string(name) == "body":
				if bodyOpenStart < 0 {
					bodyOpenStart = offset
					bodyOpenEnd = offset + len(raw)
				}
			case string(name) == "script":
				// HTML ignores the slash on <script/>: it opens a script
				// element like <script> does.
				inScript = true
				if inBody && trailingStart < 0 {
					trailingStart = offset
				}
			default:
				trailingStart = -1
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "body":
				bodyCloseStart = offset
			case "script":
				inScript = false
			default:
				if inBody {
					trailingStart = -1
				}
			}
		case html.TextToken:
			if !inScript && len(bytes.TrimSpace(raw)) != 0 {
				trailingStart = -1
			}
		}
		offset += len(raw)
	}
	return bodyOpenStart, bodyOpenEnd, bodyCloseStart, trailingStart
}

// injectWrapperDivStringBased is the string-based implementation used when
// htmlDoc contains Go template directives (which would be mangled by
// html.Parse + html.Render). It uses html.Tokenizer for tag-boundary
// detection so that '<body' / '<script' substrings inside head content
// don't fool it.
func injectWrapperDivStringBased(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	bodyOpenStart, bodyOpenEnd, bodyCloseStart, scriptStart := locateBodyAndTrailingScripts(htmlDoc)
	if bodyOpenStart < 0 || bodyCloseStart < 0 || bodyOpenEnd > bodyCloseStart {
		return htmlDoc
	}

	bodyContent := htmlDoc[bodyOpenEnd:bodyCloseStart]

	var contentToWrap, scriptsSection string
	if scriptStart >= 0 {
		split := scriptStart - bodyOpenEnd
		contentToWrap = bodyContent[:split]
		scriptsSection = bodyContent[split:]
	} else {
		contentToWrap = bodyContent
	}

	loadingAttr := ""
	if !loadingDisabled {
		loadingAttr = ` data-lvt-loading="true"`
	}

	wrappedContent := fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>%s`, wrapperID, loadingAttr, contentToWrap, scriptsSection)
	return htmlDoc[:bodyOpenEnd] + wrappedContent + htmlDoc[bodyCloseStart:]
}

// ExtractTemplateBodyContent extracts only the body content from a full HTML
// template. Handles body tags with or without attributes (e.g., <body>,
// <body class="dark">). Uses html.Tokenizer (see locateBodyAndTrailingScripts)
// to avoid being fooled by '<body' substrings in head content.
//
// This is purely a slice between the body tags and knows nothing about what
// produced the template. Recursive {{template}} support needs FlattenTemplate's
// {{define}} blocks, which sit after </html> and so fall outside this slice —
// the caller appends them, having received them from FlattenTemplate directly.
// This function used to rescan for them, which quietly made an HTML slicer
// depend on FlattenTemplate's output layout (issue #496).
func ExtractTemplateBodyContent(templateStr string) string {
	body, _ := ExtractTemplateBodyContentSliced(templateStr)
	return body
}

// ExtractTemplateBodyContentSliced is ExtractTemplateBodyContent plus whether a
// <body>…</body> region was actually sliced out — meaning anything after the
// body close was dropped from the result.
//
// Callers holding template source that belongs after the document need this to
// know whether to re-attach it. FlattenTemplate's recursion {{define}} blocks
// are the case in point: they sit past </html>, so a real slice drops them and
// they must be appended back, while the paths that return the input unchanged
// still contain them and must not have them appended twice.
//
// Reporting the fact is the point — a caller inferring it by comparing strings
// or rescanning for "{{define" is how this function ended up encoding
// FlattenTemplate's output layout in the first place (issue #496).
func ExtractTemplateBodyContentSliced(templateStr string) (body string, sliced bool) {
	bodyOpenStart, bodyOpenEnd, bodyCloseStart, _ := locateBodyAndTrailingScripts(templateStr)
	if bodyOpenStart < 0 {
		return templateStr, false
	}
	// bodyCloseStart < 0 covers "no </body> at all".
	// bodyOpenEnd > bodyCloseStart covers pathological input like
	// "</body><body>x" where the only </body> precedes the body open
	// tag — slicing [open:close] would panic. Treat both as "no usable
	// close" and fall through to TrimSpace from body open onward. That keeps
	// everything from the body open to the end, tail included, so nothing was
	// dropped and sliced stays false.
	if bodyCloseStart < 0 || bodyOpenEnd > bodyCloseStart {
		return strings.TrimSpace(templateStr[bodyOpenEnd:]), false
	}
	return strings.TrimSpace(templateStr[bodyOpenEnd:bodyCloseStart]), true
}

// ExtractTemplateContent extracts template content using wrapper ID with proper HTML parsing.
func ExtractTemplateContent(input string, wrapperID string) string {
	if wrapperID == "" {
		// For standalone templates without wrapper, return as-is
		return input
	}

	// Parse HTML document
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		// If parsing fails, fallback to returning input as-is
		return input
	}

	// Find the div with the matching data-lvt-id
	wrapperDiv := FindElementByDataLvtID(doc, wrapperID)
	if wrapperDiv == nil {
		// If wrapper not found, return the input as-is (shouldn't happen with proper injection)
		return input
	}

	// Extract content from the wrapper div
	var result strings.Builder
	for child := wrapperDiv.FirstChild; child != nil; child = child.NextSibling {
		render.Node(&result, child)
	}

	return result.String()
}

// FindElementByDataLvtID recursively searches for an element with the given data-lvt-id.
func FindElementByDataLvtID(n *html.Node, targetID string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "data-lvt-id" && attr.Val == targetID {
				return n
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := FindElementByDataLvtID(child, targetID); found != nil {
			return found
		}
	}

	return nil
}

// NormalizeTemplateSpacing normalizes spacing in template tags to prevent formatter issues.
// Converts "{{ if .X }}" to "{{if .X}}" and "{{ range .Y }}" to "{{range .Y}}".
func NormalizeTemplateSpacing(templateStr string) string {
	// Pattern to match template tags: {{ ... }}
	// Captures the content between {{ and }}
	re := regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

	return re.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Defensive check: regex guarantees at least {{ }}, but guard against edge cases
		if len(match) < 4 {
			return match
		}
		// Extract content between {{ and }}
		content := strings.TrimSpace(match[2 : len(match)-2])

		// Reconstruct with no spaces after {{ and before }}
		return "{{" + content + "}}"
	})
}
