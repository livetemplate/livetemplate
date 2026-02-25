package upload

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// ValidationError represents an upload validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateEntry validates an upload entry against its configuration.
// Returns an error if validation fails.
func ValidateEntry(entry *uploadtypes.UploadEntry, config uploadtypes.UploadConfig) error {
	// Validate file type
	if err := validateFileType(entry.ClientName, entry.ClientType, config.Accept); err != nil {
		return err
	}

	// Validate file size
	if config.MaxFileSize > 0 && entry.ClientSize > config.MaxFileSize {
		return &ValidationError{
			Field:   "size",
			Message: fmt.Sprintf("file size %d bytes exceeds maximum %d bytes", entry.ClientSize, config.MaxFileSize),
		}
	}

	return nil
}

// validateFileType checks if the file type is allowed based on config.
// Accept entries can be MIME types ("image/png") or extensions (".pdf").
func validateFileType(filename, mimeType string, accept []string) error {
	if len(accept) == 0 {
		return nil // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(filename))
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))

	for _, allowed := range accept {
		allowed = strings.ToLower(strings.TrimSpace(allowed))

		// Check if it's an extension pattern
		if strings.HasPrefix(allowed, ".") {
			if ext == allowed {
				return nil
			}
			continue
		}

		// Check if it's a MIME type pattern
		if matchesMIMEType(mimeType, allowed) {
			return nil
		}

		// Also check MIME type derived from extension
		if extMime := mime.TypeByExtension(ext); extMime != "" {
			if matchesMIMEType(extMime, allowed) {
				return nil
			}
		}
	}

	return &ValidationError{
		Field:   "type",
		Message: fmt.Sprintf("file type not accepted (file: %s, type: %s, allowed: %v)", filename, mimeType, accept),
	}
}

// matchesMIMEType checks if a MIME type matches an allowed pattern.
// Supports wildcards like "image/*".
func matchesMIMEType(mimeType, pattern string) bool {
	// Exact match
	if mimeType == pattern {
		return true
	}

	// Wildcard match (e.g., "image/*")
	if before, ok := strings.CutSuffix(pattern, "/*"); ok {
		prefix := before
		if strings.HasPrefix(mimeType, prefix+"/") {
			return true
		}
	}

	return false
}

// ValidateCount checks if the number of entries exceeds MaxEntries.
func ValidateCount(count int, config uploadtypes.UploadConfig) error {
	if config.MaxEntries > 0 && count > config.MaxEntries {
		return &ValidationError{
			Field:   "count",
			Message: fmt.Sprintf("too many files: %d exceeds maximum %d", count, config.MaxEntries),
		}
	}
	return nil
}
