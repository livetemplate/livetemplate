package livetemplate

import (
	"strings"
	"sync"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
)

var (
	minifier *minify.M
	once     sync.Once
)

// getMinifier returns a configured HTML minifier (singleton)
func getMinifier() *minify.M {
	once.Do(func() {
		minifier = minify.New()
		minifier.AddFunc("text/html", html.Minify)
	})
	return minifier
}

// minifyHTML removes unnecessary whitespace from HTML while preserving content
func minifyHTML(htmlContent string) string {
	// If content contains HTML tags, use full HTML minification
	if strings.Contains(htmlContent, "<") {
		minified, err := getMinifier().String("text/html", htmlContent)
		if err != nil {
			// If minification fails, fall back to original content
			return htmlContent
		}
		return minified
	}

	// For text-only content, normalize whitespace
	return normalizeWhitespace(htmlContent)
}

// normalizeWhitespace removes leading/trailing whitespace and normalizes internal whitespace
func normalizeWhitespace(text string) string {
	// Trim leading and trailing whitespace
	text = strings.TrimSpace(text)

	// Replace multiple whitespace characters with single spaces
	// This handles \n, \t, multiple spaces, etc.
	words := strings.Fields(text)
	return strings.Join(words, " ")
}
