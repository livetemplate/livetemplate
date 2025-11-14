package livetemplate

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// S3Config configures AWS S3 presigned upload behavior.
type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
	Expiry          time.Duration
	KeyPrefix       string
}

// S3Presigner generates presigned POST URLs for S3 uploads.
type S3Presigner struct {
	client *s3.PresignClient
	config S3Config
}

// NewS3Presigner creates a new S3 presigner with the given configuration.
func NewS3Presigner(cfg S3Config) (*S3Presigner, error) {
	ctx := context.Background()

	// Set default expiry if not specified
	if cfg.Expiry == 0 {
		cfg.Expiry = 15 * time.Minute
	}

	// Build AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use static credentials
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (IAM role, env vars, etc.)
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override endpoint if custom endpoint specified (for MinIO, LocalStack)
	if cfg.Endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	// Create S3 client and presign client
	s3Client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(s3Client)

	return &S3Presigner{
		client: presignClient,
		config: cfg,
	}, nil
}

// Presign generates a presigned PUT URL for uploading directly to S3.
func (p *S3Presigner) Presign(entry *uploadtypes.UploadEntry) (uploadtypes.UploadMeta, error) {
	// Generate S3 key from entry metadata
	key := p.generateKey(entry)

	// Create presigned PUT request
	req, err := p.client.PresignPutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(p.config.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(entry.ClientType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = p.config.Expiry
	})

	if err != nil {
		return uploadtypes.UploadMeta{}, fmt.Errorf("failed to presign S3 URL: %w", err)
	}

	// Return presigned metadata
	return uploadtypes.UploadMeta{
		Uploader: "s3",
		URL:      req.URL,
		Fields:   nil, // PUT doesn't use form fields
		Headers: map[string]string{
			"Content-Type": entry.ClientType,
		},
	}, nil
}

// generateKey creates S3 object key from upload entry.
// Format: {KeyPrefix}/{entryID}/{sanitized_filename}
func (p *S3Presigner) generateKey(entry *uploadtypes.UploadEntry) string {
	// Sanitize filename to avoid path traversal
	filename := filepath.Base(entry.ClientName)

	if p.config.KeyPrefix != "" {
		return fmt.Sprintf("%s/%s/%s", p.config.KeyPrefix, entry.ID, filename)
	}
	return fmt.Sprintf("%s/%s", entry.ID, filename)
}

