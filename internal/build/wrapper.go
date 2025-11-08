package build

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

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
// Excludes <script> tags from the wrapper to prevent them from being part of the dynamic content.
// Uses proper HTML parsing to handle all script tag variants and malformed HTML gracefully.
func InjectWrapperDiv(htmlDoc string, wrapperID string, loadingDisabled bool) string {
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

	// Separate script tags from other content
	var contentNodes, scriptNodes []*html.Node
	for child := bodyNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "script" {
			scriptNodes = append(scriptNodes, child)
		} else {
			contentNodes = append(contentNodes, child)
		}
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

// injectWrapperDivStringBased is the original string-based implementation used as fallback.
func injectWrapperDivStringBased(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	// Find the body opening tag and extract the content between <body> and </body>
	bodyStart := strings.Index(htmlDoc, "<body")
	if bodyStart == -1 {
		// No body tag found, return as-is
		return htmlDoc
	}

	// Find the end of the body opening tag
	bodyTagEnd := strings.Index(htmlDoc[bodyStart:], ">")
	if bodyTagEnd == -1 {
		return htmlDoc
	}
	bodyTagEnd += bodyStart + 1

	// Find the closing body tag
	bodyEnd := strings.LastIndex(htmlDoc, "</body>")
	if bodyEnd == -1 {
		return htmlDoc
	}

	// Extract the body content
	bodyContent := htmlDoc[bodyTagEnd:bodyEnd]

	// Find the first <script tag to exclude scripts from the wrapper
	scriptStart := strings.Index(bodyContent, "<script")
	var contentToWrap, scriptsSection string
	if scriptStart != -1 {
		// Split content: wrap everything before first script, leave scripts outside
		contentToWrap = bodyContent[:scriptStart]
		scriptsSection = bodyContent[scriptStart:]
	} else {
		// No scripts found, wrap entire body content
		contentToWrap = bodyContent
		scriptsSection = ""
	}

	// Add loading attribute if not disabled
	loadingAttr := ""
	if !loadingDisabled {
		loadingAttr = ` data-lvt-loading="true"`
	}

	// Create the wrapper div with the specified ID and optional loading attribute
	wrappedContent := fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>%s`, wrapperID, loadingAttr, contentToWrap, scriptsSection)

	// Reconstruct the HTML with the wrapper
	result := htmlDoc[:bodyTagEnd] + wrappedContent + htmlDoc[bodyEnd:]

	return result
}

// ExtractTemplateBodyContent extracts only the body content from a full HTML template.
// Handles body tags with or without attributes (e.g., <body>, <body class="dark">).
func ExtractTemplateBodyContent(templateStr string) string {
	// Find the body opening tag (with or without attributes)
	bodyStart := strings.Index(templateStr, "<body")
	if bodyStart == -1 {
		// No body tag found, return the template as-is
		return templateStr
	}

	// Find the end of the body opening tag
	bodyTagEnd := strings.Index(templateStr[bodyStart:], ">")
	if bodyTagEnd == -1 {
		// Malformed body tag, return as-is
		return templateStr
	}
	bodyTagEnd += bodyStart + 1 // Position after >

	// Find the closing body tag
	bodyEnd := strings.LastIndex(templateStr, "</body>")
	if bodyEnd == -1 {
		// No closing body tag found, return from body start to end
		return strings.TrimSpace(templateStr[bodyTagEnd:])
	}

	return strings.TrimSpace(templateStr[bodyTagEnd:bodyEnd])
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
		RenderNode(&result, child)
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
