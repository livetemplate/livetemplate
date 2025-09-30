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

	// Preserve leading/trailing space flags before any processing
	hasLeadingSpace := len(html) > 0 && (html[0] == ' ' || html[0] == '\t' || html[0] == '\n' || html[0] == '\r')
	hasTrailingSpace := len(html) > 0 && (html[len(html)-1] == ' ' || html[len(html)-1] == '\t' || html[len(html)-1] == '\n' || html[len(html)-1] == '\r')

	// Replace newlines and tabs with spaces first
	html = newlinePattern.ReplaceAllString(html, " ")

	// Replace multiple consecutive spaces with single space
	html = whitespacePattern.ReplaceAllString(html, " ")

	// Remove spaces between tags (but preserve attribute trailing spaces)
	html = spaceAroundTags.ReplaceAllString(html, "><")

	// Trim interior whitespace
	trimmed := strings.TrimSpace(html)

	// If after trimming we have content and there was originally leading/trailing whitespace,
	// ensure we have exactly one space (not more)
	if hasLeadingSpace && len(trimmed) > 0 {
		trimmed = " " + trimmed
	}
	if hasTrailingSpace && len(trimmed) > 0 {
		trimmed = trimmed + " "
	}

	return trimmed
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
