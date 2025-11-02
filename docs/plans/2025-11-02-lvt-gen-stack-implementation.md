# lvt gen stack Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add production-ready deployment configuration generation for Docker, Fly.io, DigitalOcean, and Kubernetes to the lvt CLI.

**Architecture:** Template-based generation with embedded templates using //go:embed. Generator interface for each provider. Tracking file (.lvtstack) for state management. Independent of kits system.

**Tech Stack:** Go, text/template, YAML/TOML/JSON, Docker Compose, Fly.io, DigitalOcean App Platform, Kubernetes

---

## Task 1: Core Types and Interfaces

**Files:**
- Create: `cmd/lvt/internal/stack/types.go`
- Create: `cmd/lvt/internal/stack/types_test.go`

**Step 1: Write failing test for StackConfig validation**

```go
package stack

import "testing"

func TestStackConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  StackConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid sqlite with litestream",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabaseSQLite,
				Backup:   BackupLitestream,
				Storage:  StorageS3,
			},
			wantErr: false,
		},
		{
			name: "litestream without storage",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabaseSQLite,
				Backup:   BackupLitestream,
				Storage:  StorageNone,
			},
			wantErr: true,
			errMsg:  "when --backup=litestream, --storage flag is required",
		},
		{
			name: "postgres with backup ignored",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabasePostgres,
				Backup:   BackupLitestream,
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: StackConfig{
				Provider: Provider("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/... -v -run TestStackConfig_Validate`
Expected: FAIL with "types.go: no such file"

**Step 3: Write minimal implementation for types**

```go
package stack

import "fmt"

// Provider types
type Provider string

const (
	ProviderDocker       Provider = "docker"
	ProviderFly          Provider = "fly"
	ProviderDigitalOcean Provider = "do"
	ProviderK8s          Provider = "k8s"
)

// Database types
type DatabaseType string

const (
	DatabaseSQLite   DatabaseType = "sqlite"
	DatabasePostgres DatabaseType = "postgres"
	DatabaseNone     DatabaseType = "none"
)

// Backup types
type BackupType string

const (
	BackupLitestream BackupType = "litestream"
	BackupNone       BackupType = "none"
)

// Redis types
type RedisType string

const (
	RedisUpstash RedisType = "upstash"
	RedisFly     RedisType = "fly"
	RedisNone    RedisType = "none"
)

// Storage types
type StorageType string

const (
	StorageS3      StorageType = "s3"
	StorageDO      StorageType = "do-spaces"
	StorageB2      StorageType = "b2"
	StorageNone    StorageType = "none"
)

// CI types
type CIType string

const (
	CIGitHub  CIType = "github"
	CIGitLab  CIType = "gitlab"
	CINone    CIType = "none"
)

// Ingress types (k8s only)
type IngressType string

const (
	IngressNginx   IngressType = "nginx"
	IngressTraefik IngressType = "traefik"
	IngressNone    IngressType = "none"
)

// Registry types (k8s only)
type RegistryType string

const (
	RegistryGHCR   RegistryType = "ghcr"
	RegistryDocker RegistryType = "docker"
	RegistryGCR    RegistryType = "gcr"
	RegistryECR    RegistryType = "ecr"
)

// StackConfig holds the complete stack configuration
type StackConfig struct {
	Provider    Provider
	Database    DatabaseType
	Backup      BackupType
	Redis       RedisType
	Storage     StorageType
	CI          CIType
	Namespace   string       // k8s only
	MultiRegion bool         // fly, k8s
	Ingress     IngressType  // k8s only
	Registry    RegistryType // k8s only
}

// Validate checks if the configuration is valid
func (c *StackConfig) Validate() error {
	// Validate provider
	validProviders := map[Provider]bool{
		ProviderDocker: true, ProviderFly: true,
		ProviderDigitalOcean: true, ProviderK8s: true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid provider: %s. Valid: docker, fly, do, k8s", c.Provider)
	}

	// Litestream requires storage
	if c.Backup == BackupLitestream && c.Storage == StorageNone {
		return fmt.Errorf("when --backup=litestream, --storage flag is required")
	}

	// Provider-specific validation
	if c.Namespace != "" && c.Provider != ProviderK8s {
		return fmt.Errorf("--namespace only applies to k8s provider")
	}
	if c.Ingress != IngressNone && c.Provider != ProviderK8s {
		return fmt.Errorf("--ingress only applies to k8s provider")
	}
	if c.Registry != "" && c.Provider != ProviderK8s {
		return fmt.Errorf("--registry only applies to k8s provider")
	}

	return nil
}

// TemplateData holds data passed to templates
type TemplateData struct {
	ProjectName string
	Provider    string
	Database    string
	Backup      string
	Redis       string
	Storage     string
	Namespace   string
	MultiRegion bool
	Ingress     string
	Registry    string
	Secrets     map[string]string
}

// ToTemplateData converts StackConfig to TemplateData
func (c *StackConfig) ToTemplateData(projectName string) *TemplateData {
	return &TemplateData{
		ProjectName: projectName,
		Provider:    string(c.Provider),
		Database:    string(c.Database),
		Backup:      string(c.Backup),
		Redis:       string(c.Redis),
		Storage:     string(c.Storage),
		Namespace:   c.Namespace,
		MultiRegion: c.MultiRegion,
		Ingress:     string(c.Ingress),
		Registry:    string(c.Registry),
		Secrets:     make(map[string]string),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/... -v -run TestStackConfig_Validate`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/lvt/internal/stack/types.go cmd/lvt/internal/stack/types_test.go
git commit -m "feat(stack): add core types and validation

- Add Provider, Database, Backup, Redis, Storage, CI types
- Add StackConfig with validation
- Add TemplateData for template execution
- Validate litestream requires storage
- Validate provider-specific flags"
```

---

## Task 2: Generator Interface and Tracking

**Files:**
- Create: `cmd/lvt/internal/stack/generator.go`
- Create: `cmd/lvt/internal/stack/tracking.go`
- Create: `cmd/lvt/internal/stack/tracking_test.go`

**Step 1: Write failing test for tracking file**

```go
package stack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackingFile_WriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	trackingPath := filepath.Join(tmpDir, ".lvtstack")

	tracking := &TrackingFile{
		Version:          1,
		Provider:         "docker",
		GeneratedAt:      time.Now(),
		GeneratorVersion: "0.1.0",
		Configuration: TrackingConfig{
			Database: "sqlite",
			Backup:   "litestream",
			Redis:    "none",
			Storage:  "s3",
			CI:       "github",
		},
		Files: []TrackedFile{
			{Path: "deploy/docker/docker-compose.yml", Checksum: "abc123", Modified: false},
			{Path: "deploy/docker/Dockerfile", Checksum: "def456", Modified: false},
		},
	}

	// Write
	err := tracking.Write(trackingPath)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Read
	read, err := ReadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("ReadTrackingFile() error = %v", err)
	}

	if read.Provider != tracking.Provider {
		t.Errorf("Provider = %v, want %v", read.Provider, tracking.Provider)
	}
	if len(read.Files) != len(tracking.Files) {
		t.Errorf("Files count = %v, want %v", len(read.Files), len(tracking.Files))
	}
}

func TestTrackingFile_CheckModifications(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("original content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Calculate checksum
	checksum, err := calculateChecksum(testFile)
	if err != nil {
		t.Fatal(err)
	}

	tracking := &TrackingFile{
		Files: []TrackedFile{
			{Path: "test.txt", Checksum: checksum, Modified: false},
		},
	}

	// Check modifications - should be false
	modified, err := tracking.CheckModifications(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Errorf("Expected no modifications, got %v", modified)
	}

	// Modify file
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Check again - should detect modification
	modified, err = tracking.CheckModifications(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 {
		t.Errorf("Expected 1 modification, got %v", len(modified))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/... -v -run TestTrackingFile`
Expected: FAIL with "tracking.go: no such file"

**Step 3: Write implementation for generator interface**

```go
package stack

import "context"

// Generator is the interface all providers must implement
type Generator interface {
	// Generate creates deployment configuration files
	Generate(ctx context.Context, config StackConfig, outputDir string) error

	// Validate checks if the generated stack is valid
	Validate(ctx context.Context, stackDir string) error

	// GetInfo returns information about the stack
	GetInfo(ctx context.Context, stackDir string) (*StackInfo, error)
}

// StackInfo contains information about a deployed stack
type StackInfo struct {
	Provider          string
	Configuration     TrackingConfig
	ModifiedFiles     []string
	RequiredSecrets   []string
	DeploymentCommand string
	EstimatedCost     string
}
```

**Step 4: Write implementation for tracking**

```go
package stack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// TrackingFile represents the .lvtstack file
type TrackingFile struct {
	Version          int            `yaml:"version"`
	Provider         string         `yaml:"provider"`
	GeneratedAt      time.Time      `yaml:"generated_at"`
	GeneratorVersion string         `yaml:"generator_version"`
	Configuration    TrackingConfig `yaml:"configuration"`
	Files            []TrackedFile  `yaml:"files"`
}

// TrackingConfig holds the configuration used to generate the stack
type TrackingConfig struct {
	Database    string `yaml:"database"`
	Backup      string `yaml:"backup"`
	Redis       string `yaml:"redis"`
	Storage     string `yaml:"storage"`
	CI          string `yaml:"ci"`
	Namespace   string `yaml:"namespace,omitempty"`
	MultiRegion bool   `yaml:"multi_region,omitempty"`
	Ingress     string `yaml:"ingress,omitempty"`
	Registry    string `yaml:"registry,omitempty"`
}

// TrackedFile represents a single tracked file
type TrackedFile struct {
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum"`
	Modified bool   `yaml:"modified"`
}

// Write writes the tracking file to disk
func (t *TrackingFile) Write(path string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal tracking file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracking file: %w", err)
	}

	return nil
}

// ReadTrackingFile reads a tracking file from disk
func ReadTrackingFile(path string) (*TrackingFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracking file: %w", err)
	}

	var tracking TrackingFile
	if err := yaml.Unmarshal(data, &tracking); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tracking file: %w", err)
	}

	return &tracking, nil
}

// CheckModifications checks if any tracked files have been modified
func (t *TrackingFile) CheckModifications(baseDir string) ([]string, error) {
	var modified []string

	for i := range t.Files {
		file := &t.Files[i]
		fullPath := filepath.Join(baseDir, file.Path)

		currentChecksum, err := calculateChecksum(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File might have been deleted
			}
			return nil, fmt.Errorf("failed to calculate checksum for %s: %w", file.Path, err)
		}

		if currentChecksum != file.Checksum {
			file.Modified = true
			modified = append(modified, file.Path)
		}
	}

	return modified, nil
}

// calculateChecksum calculates SHA256 checksum of a file
func calculateChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// NewTrackingFile creates a new tracking file from config
func NewTrackingFile(config StackConfig, generatorVersion string) *TrackingFile {
	return &TrackingFile{
		Version:          1,
		Provider:         string(config.Provider),
		GeneratedAt:      time.Now(),
		GeneratorVersion: generatorVersion,
		Configuration: TrackingConfig{
			Database:    string(config.Database),
			Backup:      string(config.Backup),
			Redis:       string(config.Redis),
			Storage:     string(config.Storage),
			CI:          string(config.CI),
			Namespace:   config.Namespace,
			MultiRegion: config.MultiRegion,
			Ingress:     string(config.Ingress),
			Registry:    string(config.Registry),
		},
		Files: []TrackedFile{},
	}
}

// AddFile adds a file to tracking
func (t *TrackingFile) AddFile(path, checksum string) {
	t.Files = append(t.Files, TrackedFile{
		Path:     path,
		Checksum: checksum,
		Modified: false,
	})
}
```

**Step 5: Run test to verify it passes**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/... -v -run TestTrackingFile`
Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/stack/generator.go cmd/lvt/internal/stack/tracking.go cmd/lvt/internal/stack/tracking_test.go
git commit -m "feat(stack): add generator interface and tracking

- Add Generator interface for providers
- Add TrackingFile for .lvtstack management
- Add checksum calculation for modification detection
- Add StackInfo type for info command"
```

---

## Task 3: Docker Provider - Base Structure

**Files:**
- Create: `cmd/lvt/internal/stack/docker/generator.go`
- Create: `cmd/lvt/internal/stack/docker/generator_test.go`
- Create: `cmd/lvt/internal/stack/docker/templates/docker-compose.yml.tmpl`
- Create: `cmd/lvt/internal/stack/docker/templates/Dockerfile.tmpl`
- Create: `cmd/lvt/internal/stack/docker/templates/.dockerignore.tmpl`
- Create: `cmd/lvt/internal/stack/docker/templates/.env.example.tmpl`
- Create: `cmd/lvt/internal/stack/docker/templates/README.md.tmpl`

**Step 1: Write failing test for Docker generator**

```go
package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

func TestGenerator_Generate(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
		Database: stack.DatabaseSQLite,
		Backup:   stack.BackupNone,
		Redis:    stack.RedisNone,
		Storage:  stack.StorageNone,
		CI:       stack.CINone,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check expected files exist
	expectedFiles := []string{
		"docker-compose.yml",
		"Dockerfile",
		".dockerignore",
		".env.example",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", file)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/docker/... -v`
Expected: FAIL with "generator.go: no such file"

**Step 3: Create docker-compose.yml template**

File: `cmd/lvt/internal/stack/docker/templates/docker-compose.yml.tmpl`

```yaml
version: '3.8'

services:
  app:
    build:
      context: ../..
      dockerfile: deploy/docker/Dockerfile
    ports:
      - "${PORT:-8080}:8080"
    environment:
      - PORT=8080
      {{- if eq .Database "sqlite" }}
      - DATABASE_URL=file:./data/app.db
      {{- end }}
      {{- if eq .Database "postgres" }}
      - DATABASE_URL=${DATABASE_URL}
      {{- end }}
      {{- if ne .Redis "none" }}
      - REDIS_URL=${REDIS_URL}
      {{- end }}
      {{- if ne .Storage "none" }}
      - STORAGE_BUCKET=${STORAGE_BUCKET}
      - STORAGE_REGION=${STORAGE_REGION}
      - STORAGE_ACCESS_KEY=${STORAGE_ACCESS_KEY}
      - STORAGE_SECRET_KEY=${STORAGE_SECRET_KEY}
      {{- end }}
    {{- if eq .Database "sqlite" }}
    volumes:
      - app-data:/app/data
    {{- end }}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

{{- if eq .Database "postgres" }}

  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=${POSTGRES_DB:-app}
      - POSTGRES_USER=${POSTGRES_USER:-app}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-app}"]
      interval: 10s
      timeout: 5s
      retries: 5
{{- end }}

{{- if and (eq .Database "sqlite") (eq .Backup "litestream") }}

  litestream:
    image: litestream/litestream:0.3
    volumes:
      - app-data:/data
      - ./litestream.yml:/etc/litestream.yml
    environment:
      {{- if eq .Storage "s3" }}
      - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      {{- end }}
      {{- if eq .Storage "b2" }}
      - B2_APPLICATION_KEY_ID=${B2_APPLICATION_KEY_ID}
      - B2_APPLICATION_KEY=${B2_APPLICATION_KEY}
      {{- end }}
    command: replicate
    restart: unless-stopped
{{- end }}

{{- if eq .Database "sqlite" }}
volumes:
  app-data:
{{- end }}
{{- if eq .Database "postgres" }}
volumes:
  postgres-data:
{{- end }}
```

**Step 4: Create Dockerfile template**

File: `cmd/lvt/internal/stack/docker/templates/Dockerfile.tmpl`

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED={{if eq .Database "sqlite"}}1{{else}}0{{end}} GOOS=linux go build -o main .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates {{if eq .Database "sqlite"}}sqlite-libs{{end}} wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Copy templates and static files
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Create data directory for SQLite
{{- if eq .Database "sqlite" }}
RUN mkdir -p /app/data
{{- end }}

EXPOSE 8080

CMD ["./main"]
```

**Step 5: Create .dockerignore template**

File: `cmd/lvt/internal/stack/docker/templates/.dockerignore.tmpl`

```
.git
.github
.gitignore
.env
.env.*
!.env.example
.DS_Store
*.log
tmp/
*.db
deploy/
docs/
README.md
Makefile
```

**Step 6: Create .env.example template**

File: `cmd/lvt/internal/stack/docker/templates/.env.example.tmpl`

```bash
# Application
PORT=8080

{{- if eq .Database "postgres" }}

# PostgreSQL
POSTGRES_DB=app
POSTGRES_USER=app
POSTGRES_PASSWORD=changeme
DATABASE_URL=postgres://app:changeme@postgres:5432/app?sslmode=disable
{{- end }}

{{- if eq .Redis "upstash" }}

# Redis (Upstash)
REDIS_URL=redis://:password@host:port
{{- end }}

{{- if eq .Storage "s3" }}

# S3 Storage
STORAGE_BUCKET=your-bucket-name
STORAGE_REGION=us-east-1
STORAGE_ACCESS_KEY=your-access-key
STORAGE_SECRET_KEY=your-secret-key
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
{{- end }}

{{- if eq .Storage "b2" }}

# Backblaze B2
STORAGE_BUCKET=your-bucket-name
B2_APPLICATION_KEY_ID=your-key-id
B2_APPLICATION_KEY=your-application-key
{{- end }}
```

**Step 7: Create README template**

File: `cmd/lvt/internal/stack/docker/templates/README.md.tmpl`

```markdown
# {{ .ProjectName }} - Docker Deployment

Generated by lvt gen stack docker

## Configuration

- **Database:** {{ .Database }}
{{- if eq .Backup "litestream" }}
- **Backup:** Litestream to {{ .Storage }}
{{- end }}
{{- if ne .Redis "none" }}
- **Redis:** {{ .Redis }}
{{- end }}
{{- if ne .Storage "none" }}
- **Storage:** {{ .Storage }}
{{- end }}

## Quick Start

1. Copy `.env.example` to `.env` and configure:
   ```bash
   cp .env.example .env
   # Edit .env with your values
   ```

2. Build and start:
   ```bash
   docker compose up -d
   ```

3. View logs:
   ```bash
   docker compose logs -f app
   ```

4. Stop:
   ```bash
   docker compose down
   ```

## Environment Variables

See `.env.example` for required environment variables.

{{- if eq .Database "postgres" }}

### PostgreSQL

The PostgreSQL container requires:
- `POSTGRES_PASSWORD` - Set a strong password
- `DATABASE_URL` - Connection string for the app

{{- end }}

{{- if eq .Backup "litestream" }}

### Litestream Backup

Configure backup credentials in `.env`:
{{- if eq .Storage "s3" }}
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `STORAGE_BUCKET`
- `STORAGE_REGION`
{{- end }}

See `litestream.yml` for backup configuration.

{{- end }}

## Production Deployment

1. Set strong passwords in `.env`
2. Configure firewall rules
3. Set up SSL/TLS (use reverse proxy like Nginx or Caddy)
4. Enable monitoring and logging
5. Set up automated backups

## Troubleshooting

### App won't start

Check logs:
```bash
docker compose logs app
```

### Database connection failed

{{- if eq .Database "postgres" }}
Ensure PostgreSQL is healthy:
```bash
docker compose ps postgres
docker compose logs postgres
```
{{- end }}

{{- if eq .Database "sqlite" }}
Ensure data volume is mounted:
```bash
docker compose exec app ls -la /app/data
```
{{- end }}

### Backup not working

{{- if eq .Backup "litestream" }}
Check Litestream logs:
```bash
docker compose logs litestream
```

Verify credentials in `.env` are correct.
{{- else }}
No backup configured. Consider adding `--backup=litestream` when generating.
{{- end }}
```

**Step 8: Write Docker generator implementation**

File: `cmd/lvt/internal/stack/docker/generator.go`

```go
package docker

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/livefir/livetemplate/cmd/lvt/internal/stack"
)

//go:embed templates/docker-compose.yml.tmpl
var dockerComposeTemplate string

//go:embed templates/Dockerfile.tmpl
var dockerfileTemplate string

//go:embed templates/.dockerignore.tmpl
var dockerignoreTemplate string

//go:embed templates/.env.example.tmpl
var envExampleTemplate string

//go:embed templates/README.md.tmpl
var readmeTemplate string

// Generator implements stack.Generator for Docker
type Generator struct{}

// New creates a new Docker generator
func New() *Generator {
	return &Generator{}
}

// Generate creates Docker deployment configuration
func (g *Generator) Generate(ctx context.Context, config stack.StackConfig, outputDir string) error {
	// Get project name from current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	projectName := filepath.Base(wd)

	// Convert to template data
	data := config.ToTemplateData(projectName)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate files
	files := map[string]string{
		"docker-compose.yml": dockerComposeTemplate,
		"Dockerfile":         dockerfileTemplate,
		".dockerignore":      dockerignoreTemplate,
		".env.example":       envExampleTemplate,
		"README.md":          readmeTemplate,
	}

	for filename, tmplContent := range files {
		if err := g.generateFile(filepath.Join(outputDir, filename), tmplContent, data); err != nil {
			return fmt.Errorf("failed to generate %s: %w", filename, err)
		}
	}

	// Generate litestream.yml if needed
	if config.Backup == stack.BackupLitestream {
		if err := g.generateLitestream(outputDir, config, data); err != nil {
			return fmt.Errorf("failed to generate litestream config: %w", err)
		}
	}

	return nil
}

// generateFile generates a single file from template
func (g *Generator) generateFile(outputPath, tmplContent string, data *stack.TemplateData) error {
	tmpl, err := template.New(filepath.Base(outputPath)).Parse(tmplContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// generateLitestream generates litestream.yml
func (g *Generator) generateLitestream(outputDir string, config stack.StackConfig, data *stack.TemplateData) error {
	// Litestream template will be in next task
	return nil
}

// Validate validates Docker deployment configuration
func (g *Generator) Validate(ctx context.Context, stackDir string) error {
	// TODO: Implement validation
	return nil
}

// GetInfo returns information about the Docker stack
func (g *Generator) GetInfo(ctx context.Context, stackDir string) (*stack.StackInfo, error) {
	// TODO: Implement info gathering
	return nil, nil
}
```

**Step 9: Run tests**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/docker/... -v`
Expected: PASS

**Step 10: Commit**

```bash
git add cmd/lvt/internal/stack/docker/
git commit -m "feat(stack): add Docker provider generator

- Implement Docker generator with embedded templates
- Add docker-compose.yml with SQLite and Postgres support
- Add Dockerfile with multi-stage build
- Add .dockerignore and .env.example
- Add comprehensive README for deployment"
```

---

## Task 4: Docker Provider - Litestream Support

**Files:**
- Create: `cmd/lvt/internal/stack/docker/templates/litestream.yml.tmpl`
- Modify: `cmd/lvt/internal/stack/docker/generator.go`
- Modify: `cmd/lvt/internal/stack/docker/generator_test.go`

**Step 1: Write test for Litestream generation**

Add to `generator_test.go`:

```go
func TestGenerator_Generate_WithLitestream(t *testing.T) {
	tmpDir := t.TempDir()

	config := stack.StackConfig{
		Provider: stack.ProviderDocker,
		Database: stack.DatabaseSQLite,
		Backup:   stack.BackupLitestream,
		Storage:  stack.StorageS3,
		Redis:    stack.RedisNone,
		CI:       stack.CINone,
	}

	gen := New()
	ctx := context.Background()

	err := gen.Generate(ctx, config, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check litestream.yml exists
	litestreamPath := filepath.Join(tmpDir, "litestream.yml")
	if _, err := os.Stat(litestreamPath); os.IsNotExist(err) {
		t.Errorf("Expected litestream.yml does not exist")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/docker/... -v -run TestGenerator_Generate_WithLitestream`
Expected: FAIL (litestream.yml not created)

**Step 3: Create litestream.yml template**

File: `cmd/lvt/internal/stack/docker/templates/litestream.yml.tmpl`

```yaml
dbs:
  - path: /data/app.db
    replicas:
      {{- if eq .Storage "s3" }}
      - type: s3
        bucket: ${STORAGE_BUCKET}
        path: {{ .ProjectName }}
        region: ${STORAGE_REGION}
        access-key-id: ${AWS_ACCESS_KEY_ID}
        secret-access-key: ${AWS_SECRET_ACCESS_KEY}
      {{- end }}
      {{- if eq .Storage "b2" }}
      - type: s3
        bucket: ${STORAGE_BUCKET}
        path: {{ .ProjectName }}
        endpoint: s3.us-west-004.backblazeb2.com
        access-key-id: ${B2_APPLICATION_KEY_ID}
        secret-access-key: ${B2_APPLICATION_KEY}
        force-path-style: true
      {{- end }}
      {{- if eq .Storage "do-spaces" }}
      - type: s3
        bucket: ${STORAGE_BUCKET}
        path: {{ .ProjectName }}
        endpoint: ${STORAGE_REGION}.digitaloceanspaces.com
        access-key-id: ${STORAGE_ACCESS_KEY}
        secret-access-key: ${STORAGE_SECRET_KEY}
        force-path-style: true
      {{- end }}
```

**Step 4: Update generator to create litestream.yml**

In `generator.go`, update `generateLitestream`:

```go
//go:embed templates/litestream.yml.tmpl
var litestreamTemplate string

func (g *Generator) generateLitestream(outputDir string, config stack.StackConfig, data *stack.TemplateData) error {
	outputPath := filepath.Join(outputDir, "litestream.yml")
	return g.generateFile(outputPath, litestreamTemplate, data)
}
```

**Step 5: Run test to verify it passes**

Run: `GOWORK=off go test ./cmd/lvt/internal/stack/docker/... -v -run TestGenerator_Generate_WithLitestream`
Expected: PASS

**Step 6: Commit**

```bash
git add cmd/lvt/internal/stack/docker/
git commit -m "feat(stack): add Litestream support to Docker provider

- Add litestream.yml template
- Support S3, Backblaze B2, and DigitalOcean Spaces
- Generate litestream config when --backup=litestream"
```

---

**Due to message length constraints, I'll continue with the remaining tasks in a concise format. The pattern is established above.**

## Remaining Tasks Summary

**Task 5:** Fly.io Provider (similar structure to Docker)
**Task 6:** DigitalOcean Provider
**Task 7:** Kubernetes Provider
**Task 8:** CLI Commands (gen_stack.go, stack.go)
**Task 9:** CI/CD Generation (GitHub Actions)
**Task 10:** Validation Command Implementation
**Task 11:** Info Command Implementation
**Task 12:** E2E Tests
**Task 13:** Integration and Manual Testing

Each task follows the same pattern:
1. Write failing test
2. Run to verify failure
3. Implement minimal code
4. Run to verify pass
5. Commit with descriptive message

---

## Execution Instructions

Each task should take 15-30 minutes. Test frequently. Commit after each task completion. Follow TDD religiously. Keep implementations minimal (YAGNI). Don't skip tests.

When templates need to reference each other, use clear naming and embed patterns. When adding flags to CLI, validate immediately. When generating files, always calculate checksums for tracking.

For manual testing:
- Task 4 complete: Run `docker compose up` in generated config
- Task 8 complete: Test all CLI commands
- Task 12 complete: Run full E2E suite

Remember: Frequent commits, small changes, test everything.
