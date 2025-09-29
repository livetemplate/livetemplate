package livetemplate

import (
	"regexp"
	"strings"
)

var (
	// Regex patterns for HTML minification
	whitespacePattern = regexp.MustCompile(`\s+`)
	newlinePattern    = regexp.MustCompile(`[\r\n]+`)
	spaceAroundTags   = regexp.MustCompile(`>\s+<`)
)

// minifyHTMLWhitespace removes unnecessary whitespace from HTML content
// while preserving space in important contexts
func minifyHTMLWhitespace(html string) string {
	// Skip minification for very short strings
	if len(html) <= 3 {
		return html
	}

	// Replace newlines and tabs with spaces first
	html = newlinePattern.ReplaceAllString(html, " ")

	// Replace multiple consecutive spaces with single space
	html = whitespacePattern.ReplaceAllString(html, " ")

	// Remove spaces between tags (but preserve attribute trailing spaces)
	html = spaceAroundTags.ReplaceAllString(html, "><")

	// Trim leading and trailing whitespace only if the entire string is whitespace
	if strings.TrimSpace(html) != "" {
		html = strings.TrimSpace(html)
	}

	return html
}

// minifyStatics applies minification to an array of static strings
func minifyStatics(statics []string) []string {
	minified := make([]string, len(statics))
	for i, static := range statics {
		// Only minify strings that contain HTML tags or lots of whitespace
		if strings.Contains(static, "<") || strings.Contains(static, ">") {
			minified[i] = minifyHTMLWhitespace(static)
		} else if strings.Contains(static, "\n") || strings.Contains(static, "\t") {
			// Minify whitespace-heavy strings
			minified[i] = minifyHTMLWhitespace(static)
		} else {
			// Preserve as-is for attribute values and short strings
			minified[i] = static
		}
	}
	return minified
}
