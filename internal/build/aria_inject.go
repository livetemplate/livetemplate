package build

import (
	"strings"

	"github.com/livetemplate/livetemplate/internal/util"
	"golang.org/x/net/html"
)

var formElements = map[string]bool{
	"input":    true,
	"textarea": true,
	"select":   true,
}

// InjectAriaInvalid adds aria-invalid="true" to form elements whose name matches a field error.
// This is a safety net for non-JS HTTP responses — templates should also use the
// {{.lvt.AriaInvalid "field"}} helper which works for both HTTP and WebSocket paths.
//
// Note: when injection occurs, golang.org/x/net/html re-serializes the entire HTML,
// which may normalize attribute quoting, void element syntax, and whitespace.
// This only affects the HTTP response path when field errors are present.
func InjectAriaInvalid(htmlStr string, fieldErrors map[string]string) string {
	if len(fieldErrors) == 0 {
		return htmlStr
	}

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}

	if !injectAriaInvalidWalk(doc, fieldErrors) {
		return htmlStr
	}

	var buf strings.Builder

	// html.Parse always produces a full document tree (<html><head><body>).
	// For fragment input, render only body children to avoid adding a wrapper.
	// Detect full documents by checking for an <html> element in the parsed tree.
	if hasDoctype(doc) {
		if err := html.Render(&buf, doc); err != nil {
			return htmlStr
		}
	} else {
		body := FindBodyNode(doc)
		if body == nil {
			return htmlStr
		}
		for c := body.FirstChild; c != nil; c = c.NextSibling {
			if err := html.Render(&buf, c); err != nil {
				return htmlStr
			}
		}
	}

	return buf.String()
}

// injectAriaInvalidWalk walks the DOM tree and injects aria-invalid on matching
// form elements. Returns true if any injection was performed.
func injectAriaInvalidWalk(n *html.Node, fieldErrors map[string]string) bool {
	injected := false
	if n.Type == html.ElementNode && formElements[n.Data] {
		var name string
		hasAriaInvalid := false
		for _, attr := range n.Attr {
			if attr.Key == "name" {
				name = attr.Val
			}
			if attr.Key == "aria-invalid" {
				hasAriaInvalid = true
			}
		}
		if name != "" && !hasAriaInvalid {
			if hasFieldError(name, fieldErrors) {
				n.Attr = append(n.Attr, html.Attribute{Key: "aria-invalid", Val: "true"})
				injected = true
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if injectAriaInvalidWalk(c, fieldErrors) {
			injected = true
		}
	}
	return injected
}

// hasDoctype checks if the parsed tree has a DOCTYPE node, indicating the input
// was a full HTML document rather than a fragment.
func hasDoctype(doc *html.Node) bool {
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.DoctypeNode {
			return true
		}
	}
	return false
}

func hasFieldError(name string, fieldErrors map[string]string) bool {
	if _, ok := fieldErrors[name]; ok {
		return true
	}
	snake := util.ToSnakeCase(name)
	if snake != name {
		if _, ok := fieldErrors[snake]; ok {
			return true
		}
	}
	return false
}
