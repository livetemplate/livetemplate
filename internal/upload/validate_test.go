package upload

import (
	"testing"

	"github.com/livetemplate/livetemplate"
)

func TestValidateEntry(t *testing.T) {
	tests := []struct {
		name      string
		entry     *livetemplate.UploadEntry
		config    livetemplate.UploadConfig
		wantError bool
	}{
		{
			name: "valid image upload",
			entry: &livetemplate.UploadEntry{
				ClientName: "photo.jpg",
				ClientType: "image/jpeg",
				ClientSize: 1024 * 1024, // 1MB
			},
			config: livetemplate.UploadConfig{
				Accept:      []string{"image/*"},
				MaxFileSize: 5 * 1024 * 1024, // 5MB
			},
			wantError: false,
		},
		{
			name: "valid PDF by extension",
			entry: &livetemplate.UploadEntry{
				ClientName: "document.pdf",
				ClientType: "application/pdf",
				ClientSize: 2 * 1024 * 1024,
			},
			config: livetemplate.UploadConfig{
				Accept:      []string{".pdf"},
				MaxFileSize: 10 * 1024 * 1024,
			},
			wantError: false,
		},
		{
			name: "invalid file type",
			entry: &livetemplate.UploadEntry{
				ClientName: "script.js",
				ClientType: "application/javascript",
				ClientSize: 1024,
			},
			config: livetemplate.UploadConfig{
				Accept: []string{"image/*", ".pdf"},
			},
			wantError: true,
		},
		{
			name: "file too large",
			entry: &livetemplate.UploadEntry{
				ClientName: "large.jpg",
				ClientType: "image/jpeg",
				ClientSize: 10 * 1024 * 1024,
			},
			config: livetemplate.UploadConfig{
				Accept:      []string{"image/*"},
				MaxFileSize: 5 * 1024 * 1024,
			},
			wantError: true,
		},
		{
			name: "no restrictions",
			entry: &livetemplate.UploadEntry{
				ClientName: "anything.xyz",
				ClientType: "application/octet-stream",
				ClientSize: 1024,
			},
			config:    livetemplate.UploadConfig{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEntry(tt.entry, tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateEntry() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateFileType(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		mimeType  string
		accept    []string
		wantError bool
	}{
		{
			name:      "exact MIME match",
			filename:  "image.png",
			mimeType:  "image/png",
			accept:    []string{"image/png", "image/jpeg"},
			wantError: false,
		},
		{
			name:      "wildcard MIME match",
			filename:  "photo.jpg",
			mimeType:  "image/jpeg",
			accept:    []string{"image/*"},
			wantError: false,
		},
		{
			name:      "extension match",
			filename:  "document.pdf",
			mimeType:  "application/pdf",
			accept:    []string{".pdf", ".doc"},
			wantError: false,
		},
		{
			name:      "case insensitive",
			filename:  "Photo.PNG",
			mimeType:  "IMAGE/PNG",
			accept:    []string{"image/*"},
			wantError: false,
		},
		{
			name:      "no match",
			filename:  "script.js",
			mimeType:  "application/javascript",
			accept:    []string{"image/*", ".pdf"},
			wantError: true,
		},
		{
			name:      "empty accept list",
			filename:  "anything.xyz",
			mimeType:  "application/octet-stream",
			accept:    []string{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileType(tt.filename, tt.mimeType, tt.accept)
			if (err != nil) != tt.wantError {
				t.Errorf("validateFileType() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestMatchesMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		pattern  string
		want     bool
	}{
		{
			name:     "exact match",
			mimeType: "image/png",
			pattern:  "image/png",
			want:     true,
		},
		{
			name:     "wildcard match",
			mimeType: "image/jpeg",
			pattern:  "image/*",
			want:     true,
		},
		{
			name:     "no match different type",
			mimeType: "text/plain",
			pattern:  "image/*",
			want:     false,
		},
		{
			name:     "no match different subtype",
			mimeType: "image/png",
			pattern:  "image/jpeg",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesMIMEType(tt.mimeType, tt.pattern); got != tt.want {
				t.Errorf("matchesMIMEType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateCount(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		config    livetemplate.UploadConfig
		wantError bool
	}{
		{
			name:  "within limit",
			count: 3,
			config: livetemplate.UploadConfig{
				MaxEntries: 5,
			},
			wantError: false,
		},
		{
			name:  "at limit",
			count: 5,
			config: livetemplate.UploadConfig{
				MaxEntries: 5,
			},
			wantError: false,
		},
		{
			name:  "exceeds limit",
			count: 6,
			config: livetemplate.UploadConfig{
				MaxEntries: 5,
			},
			wantError: true,
		},
		{
			name:  "no limit",
			count: 100,
			config: livetemplate.UploadConfig{
				MaxEntries: 0,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCount(tt.count, tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCount() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
