package render

import (
	"log/slog"
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

// MinifyHTML removes unnecessary whitespace from HTML while preserving content.
// If content contains HTML tags, uses full HTML minification.
// For text-only content, normalizes whitespace.
// Returns the original content if minification fails.
func MinifyHTML(htmlContent string) string {
	// If content contains HTML tags, use full HTML minification
	if strings.Contains(htmlContent, "<") {
		minified, err := getMinifier().String("text/html", htmlContent)
		if err != nil {
			// If minification fails, fall back to original content
			slog.Warn("HTML minification failed, using original content",
				slog.Any("error", err),
				slog.Int("content_length", len(htmlContent)))
			return htmlContent
		}
		return minified
	}

	// For text-only content, normalize whitespace
	return normalizeWhitespace(htmlContent)
}

// normalizeWhitespace removes leading/trailing whitespace and normalizes internal whitespace.
// Handles \n, \t, multiple spaces, etc., replacing them with single spaces.
func normalizeWhitespace(text string) string {
	// Trim leading and trailing whitespace
	text = strings.TrimSpace(text)

	// Early return for empty text to avoid unnecessary processing
	if text == "" {
		return ""
	}

	// Replace multiple whitespace characters with single spaces
	// This handles \n, \t, multiple spaces, etc.
	words := strings.Fields(text)
	return strings.Join(words, " ")
}
