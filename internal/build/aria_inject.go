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
func InjectAriaInvalid(htmlStr string, fieldErrors map[string]string) string {
	if len(fieldErrors) == 0 {
		return htmlStr
	}

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}

	injectAriaInvalidWalk(doc, fieldErrors)

	var buf strings.Builder

	// html.Parse always produces a full document tree (<html><head><body>).
	// For fragment input, render only body children to avoid adding a wrapper.
	trimmed := strings.TrimSpace(htmlStr)
	isFullDoc := strings.HasPrefix(trimmed, "<!") ||
		strings.HasPrefix(strings.ToLower(trimmed), "<html")

	if isFullDoc {
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

func injectAriaInvalidWalk(n *html.Node, fieldErrors map[string]string) {
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
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		injectAriaInvalidWalk(c, fieldErrors)
	}
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
