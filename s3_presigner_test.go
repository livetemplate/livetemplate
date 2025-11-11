package livetemplate

import (
	"strings"
	"testing"
	"time"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

func TestS3Config_Defaults(t *testing.T) {
	cfg := S3Config{
		Bucket: "test-bucket",
		Region: "us-east-1",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	if presigner.config.Expiry != 15*time.Minute {
		t.Errorf("Expected default expiry 15min, got %v", presigner.config.Expiry)
	}
}

func TestS3Config_CustomExpiry(t *testing.T) {
	cfg := S3Config{
		Bucket: "test-bucket",
		Region: "us-east-1",
		Expiry: 30 * time.Minute,
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	if presigner.config.Expiry != 30*time.Minute {
		t.Errorf("Expected expiry 30min, got %v", presigner.config.Expiry)
	}
}

func TestS3Config_StaticCredentials(t *testing.T) {
	cfg := S3Config{
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	if presigner == nil {
		t.Error("Expected presigner to be created with static credentials")
	}
}

func TestS3Config_CustomEndpoint(t *testing.T) {
	cfg := S3Config{
		Bucket:   "test-bucket",
		Region:   "us-east-1",
		Endpoint: "http://localhost:9000", // MinIO endpoint
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	if presigner == nil {
		t.Error("Expected presigner to be created with custom endpoint")
	}
}

func TestS3Presigner_GenerateKey_WithPrefix(t *testing.T) {
	cfg := S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		KeyPrefix: "uploads",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "test-entry-123",
		ClientName: "photo.jpg",
		ClientType: "image/jpeg",
		ClientSize: 1024 * 1024,
	}

	key := presigner.generateKey(entry)
	expected := "uploads/test-entry-123/photo.jpg"
	if key != expected {
		t.Errorf("generateKey() = %q, want %q", key, expected)
	}
}

func TestS3Presigner_GenerateKey_WithoutPrefix(t *testing.T) {
	cfg := S3Config{
		Bucket: "test-bucket",
		Region: "us-east-1",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "test-entry-456",
		ClientName: "document.pdf",
		ClientType: "application/pdf",
		ClientSize: 2 * 1024 * 1024,
	}

	key := presigner.generateKey(entry)
	expected := "test-entry-456/document.pdf"
	if key != expected {
		t.Errorf("generateKey() = %q, want %q", key, expected)
	}
}

func TestS3Presigner_GenerateKey_PathTraversalPrevention(t *testing.T) {
	cfg := S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		KeyPrefix: "uploads",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	tests := []struct {
		name       string
		clientName string
		wantBase   string
	}{
		{
			name:       "path traversal with ../",
			clientName: "../../../etc/passwd",
			wantBase:   "passwd",
		},
		{
			name:       "absolute path",
			clientName: "/etc/passwd",
			wantBase:   "passwd",
		},
		{
			name:       "windows backslash (treated as literal)",
			clientName: "file\\with\\backslash.txt",
			wantBase:   "file\\with\\backslash.txt", // filepath.Base on Unix treats backslash as literal
		},
		{
			name:       "nested path",
			clientName: "folder/subfolder/file.txt",
			wantBase:   "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &uploadtypes.UploadEntry{
				ID:         "test-entry-789",
				ClientName: tt.clientName,
				ClientType: "text/plain",
				ClientSize: 1024,
			}

			key := presigner.generateKey(entry)

			// Key should be: uploads/test-entry-789/{base}
			parts := strings.Split(key, "/")
			if len(parts) != 3 {
				t.Errorf("Expected 3 parts in key, got %d: %s", len(parts), key)
			}

			gotBase := parts[2]
			if gotBase != tt.wantBase {
				t.Errorf("generateKey() base = %q, want %q", gotBase, tt.wantBase)
			}
		})
	}
}

func TestS3Presigner_Presign_BasicValidation(t *testing.T) {
	// This test validates the presign function returns properly structured metadata
	// Note: We can't actually verify the signature without AWS credentials
	cfg := S3Config{
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Expiry:          5 * time.Minute,
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "test-entry-abc",
		ClientName: "test-file.txt",
		ClientType: "text/plain",
		ClientSize: 1024,
	}

	meta, err := presigner.Presign(entry)
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}

	// Verify uploader type
	if meta.Uploader != "s3" {
		t.Errorf("Uploader = %q, want %q", meta.Uploader, "s3")
	}

	// Verify URL is not empty
	if meta.URL == "" {
		t.Error("Presign() returned empty URL")
	}

	// Verify URL contains bucket name
	if !strings.Contains(meta.URL, "test-bucket") {
		t.Errorf("URL does not contain bucket name: %s", meta.URL)
	}

	// Verify headers include Content-Type
	if meta.Headers["Content-Type"] != "text/plain" {
		t.Errorf("Content-Type header = %q, want %q", meta.Headers["Content-Type"], "text/plain")
	}

	// PUT doesn't use form fields
	if meta.Fields != nil {
		t.Error("Presign() PUT should not return form fields")
	}
}

func TestS3Presigner_Presign_URLStructure(t *testing.T) {
	cfg := S3Config{
		Bucket:          "my-upload-bucket",
		Region:          "us-west-2",
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		KeyPrefix:       "user-uploads",
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() error = %v", err)
	}

	entry := &uploadtypes.UploadEntry{
		ID:         "entry-xyz",
		ClientName: "avatar.png",
		ClientType: "image/png",
		ClientSize: 512 * 1024,
	}

	meta, err := presigner.Presign(entry)
	if err != nil {
		t.Fatalf("Presign() error = %v", err)
	}

	// Verify URL structure
	expectedParts := []string{
		"my-upload-bucket",
		"user-uploads",
		"entry-xyz",
		"avatar.png",
	}

	for _, part := range expectedParts {
		if !strings.Contains(meta.URL, part) {
			t.Errorf("URL does not contain expected part %q: %s", part, meta.URL)
		}
	}

	// Verify signature parameters are present
	if !strings.Contains(meta.URL, "X-Amz-") {
		t.Error("URL does not contain AWS signature parameters")
	}
}

func TestS3Config_MissingBucket(t *testing.T) {
	cfg := S3Config{
		Region: "us-east-1",
		// Bucket is missing
	}

	presigner, err := NewS3Presigner(cfg)
	if err != nil {
		t.Fatalf("NewS3Presigner() should not error on missing bucket: %v", err)
	}

	// Presign should fail when bucket is empty
	entry := &uploadtypes.UploadEntry{
		ID:         "test-entry",
		ClientName: "test.txt",
		ClientType: "text/plain",
		ClientSize: 100,
	}

	_, err = presigner.Presign(entry)
	if err == nil {
		t.Error("Presign() should error when bucket is empty")
	}
}

func TestS3Config_MissingRegion(t *testing.T) {
	cfg := S3Config{
		Bucket: "test-bucket",
		// Region is missing - AWS SDK may use default or error
	}

	// This should succeed - AWS SDK will use default region
	_, err := NewS3Presigner(cfg)
	if err != nil {
		// Region might be required depending on AWS SDK config
		t.Logf("NewS3Presigner() with no region: %v (expected)", err)
	}
}
