# lvt gen stack - Design Document

**Date:** 2025-11-02
**Status:** Approved
**Branch:** feature/lvt-gen-stack

## Overview

Add stack deployment generation to the `lvt` CLI tool, enabling users to generate production-ready deployment configurations for Docker, Fly.io, DigitalOcean, and Kubernetes with a single command.

**Key Design Decisions:**
- Production deployment focus (not just local dev)
- Kit-independent (CSS framework doesn't affect deployment)
- Single provider per project (one .lvtstack file)
- Full integration support (database, backup, Redis, storage, CI/CD)
- Template-based generation (embedded in binary)

## Command Structure

### Commands

```bash
lvt gen stack <provider>     # Generate deployment config
lvt stack validate           # Validate existing config
lvt stack info              # Show stack information
```

**Providers:** `docker`, `fly`, `do`, `k8s`

### Flags

```bash
# Database
--db <sqlite|postgres|none>        # Default: sqlite

# Backup (SQLite only)
--backup <litestream|none>         # Default: litestream (if --db=sqlite)

# Integrations
--redis <upstash|fly|none>         # Default: none
--storage <s3|do-spaces|b2|none>   # Default: none (required if --backup=litestream)

# CI/CD
--ci <github|gitlab|none>          # Default: github

# Provider-specific
--multi-region                     # fly, k8s only
--namespace <name>                 # k8s only
--ingress <nginx|traefik|none>     # k8s only
--registry <ghcr|docker|gcr|ecr>   # k8s only
```

### Validation Rules

- If `--backup=litestream`, require `--storage` to be set
- If `--db=postgres`, ignore `--backup` flag
- If `--db=sqlite` and `--backup=litestream` but no `--storage`, prompt user to select storage
- Provider-specific flags only apply to their respective providers

## Architecture

### Package Structure

```
cmd/lvt/
├── commands/
│   ├── gen_stack.go           # lvt gen stack <provider>
│   └── stack.go               # lvt stack {validate|info}
├── internal/
│   └── stack/
│       ├── types.go           # Common types
│       ├── generator.go       # Generator interface
│       ├── validation.go      # Validation logic
│       ├── info.go           # Info gathering
│       ├── tracking.go       # .lvtstack management
│       ├── docker/
│       │   ├── generator.go
│       │   └── templates/    # Embedded with //go:embed
│       ├── fly/
│       │   ├── generator.go
│       │   └── templates/
│       ├── digitalocean/
│       │   ├── generator.go
│       │   └── templates/
│       └── k8s/
│           ├── generator.go
│           └── templates/
```

### Core Types

```go
type StackConfig struct {
    Provider    Provider
    Database    DatabaseType
    Backup      BackupType
    Redis       RedisType
    Storage     StorageType
    CI          CIType
    Namespace   string        // k8s only
    MultiRegion bool          // fly, k8s
    Ingress     IngressType   // k8s only
    Registry    RegistryType  // k8s only
}

type Generator interface {
    Generate(config StackConfig, outputDir string) error
    Validate(stackDir string) error
    GetInfo(stackDir string) (*StackInfo, error)
}

type TemplateData struct {
    ProjectName  string
    Provider     string
    Database     string
    Backup       string
    Redis        string
    Storage      string
    Namespace    string
    MultiRegion  bool
    Ingress      string
    Registry     string
    Secrets      map[string]string
}
```

## Template Structure

### Organization (per provider)

```
cmd/lvt/internal/stack/<provider>/templates/
├── base/
│   ├── config.tmpl           # Main config (compose/toml/yaml)
│   └── README.md.tmpl
├── database/
│   ├── sqlite.tmpl           # SQLite-specific config
│   └── postgres.tmpl         # Postgres-specific config
├── backup/
│   └── litestream.yml.tmpl   # Litestream configuration
├── integrations/
│   ├── redis-upstash.tmpl
│   ├── redis-fly.tmpl
│   ├── storage-s3.tmpl
│   ├── storage-do.tmpl
│   └── storage-b2.tmpl
└── ci/
    ├── github.yml.tmpl
    └── gitlab.yml.tmpl
```

### Generation Process

1. Parse CLI flags → Build StackConfig
2. Select provider generator (docker/fly/do/k8s)
3. Generator loads embedded templates
4. Load base template for provider
5. Conditionally include database template based on --db
6. If --backup=litestream, include litestream.yml
7. Add integration templates (redis, storage)
8. Generate CI/CD workflow if --ci set
9. Write files to deploy/<provider>/
10. Generate .lvtstack tracking file

### Generated Structure

```
project/
├── deploy/
│   ├── <provider>/              # docker, fly, digitalocean, or k8s
│   │   ├── (provider configs)
│   │   ├── litestream.yml       # If --backup=litestream
│   │   ├── .env.example         # Secret placeholders
│   │   └── README.md
│   └── .lvtstack                # Tracking metadata
└── .github/workflows/           # If --ci=github
    └── deploy-<provider>.yml
```

Example Docker output:
```
deploy/docker/
├── docker-compose.yml       # Base + database + integrations
├── Dockerfile
├── .dockerignore
├── .env.example             # All secret placeholders
├── litestream.yml           # If --backup=litestream
└── README.md                # Setup instructions
```

## Tracking & Validation

### .lvtstack Format

```yaml
version: 1
provider: docker
generated_at: 2025-11-02T10:30:00Z
generator_version: 0.1.0

configuration:
  database: sqlite
  backup: litestream
  redis: none
  storage: s3
  ci: github

files:
  - path: deploy/docker/docker-compose.yml
    checksum: abc123def456...
    modified: false
  - path: deploy/docker/litestream.yml
    checksum: mno345pqr678...
    modified: true
```

### Validation (lvt stack validate)

Checks performed:
- Verify all tracked files exist
- Check checksums to detect user modifications
- Validate config syntax (YAML/TOML/JSON)
- Check for required environment variables
- Verify tool availability (docker, kubectl, flyctl)
- Output: List of issues with severity (error/warning)

### Info Command (lvt stack info)

Displays:
- Provider and configuration summary
- Required secrets/environment variables
- Which files were modified by user
- Estimated monthly cost (if possible)
- Deployment command examples

### Re-generation Behavior

When running `lvt gen stack <provider>` on existing stack:
1. Check if .lvtstack exists
2. If different provider → Error: "Stack already exists for <old-provider>. Delete deploy/ dir first."
3. If same provider → Warn about overwriting, show modified files, require --force flag

## Testing Strategy

### Unit Tests

Location: `*_test.go` files alongside implementation

Coverage:
- Template parsing and data injection
- StackConfig validation
- Flag parsing logic
- Checksum calculation
- Error message formatting

### E2E Tests

Location: `cmd/lvt/e2e/gen_stack_test.go`

Tests:
- Generate stack for each provider
- Verify all expected files created
- Validate generated config syntax
- Test flag combinations
- Verify .lvtstack tracking file
- Test validation command
- Test info command
- Test --force flag behavior

### Golden File Tests

Location: `testdata/stack/<provider>/`

Process:
- Store expected output as golden files
- Compare generated files against golden files
- Use `UPDATE_GOLDEN=1` env var for updates
- Version control golden files

### Manual Verification

- **Docker:** `docker compose up` in generated config
- **Fly.io:** `flyctl launch` with generated config (if account available)
- **K8s:** `kubectl apply --dry-run` to verify syntax
- **DO:** Validate app-spec.yaml against DO schema

## Error Handling

### User-Facing Errors

Return helpful, actionable messages:
- Missing required flags: "When --backup=litestream, --storage flag is required"
- Invalid provider: "Unknown provider 'aws'. Valid: docker, fly, do, k8s"
- Conflicting flags: "--namespace only applies to k8s provider"
- File conflicts: "deploy/docker already exists. Use --force to overwrite."

### Template Errors

Should never happen in production:
- Log template parse errors with file path
- Include template data in error for debugging
- Fail fast with clear error message

### Validation Errors

From `lvt stack validate`, structured output:
- File path, line number, issue description, severity
- Example: "deploy/docker/docker-compose.yml:15: Missing required env var DATABASE_URL (error)"

### Recovery Guidance

- Each error includes suggested fix
- README.md in generated stack has troubleshooting section
- Common issues documented with solutions

## Provider-Specific Details

### Docker

Files generated:
- docker-compose.yml
- Dockerfile
- .dockerignore
- .env.example
- litestream.yml (if --backup=litestream)
- README.md

Key features:
- Volume mounts for SQLite persistence
- Health checks for services
- Network configuration
- Environment variable management

### Fly.io

Files generated:
- fly.toml
- Dockerfile
- litestream.yml (if --backup=litestream)
- README.md

Key features:
- Volume configuration for SQLite
- Region selection (--multi-region)
- Secrets management via flyctl
- Health check endpoints

### DigitalOcean

Files generated:
- app-spec.yaml
- Dockerfile
- README.md

Key features:
- Managed database integration
- Environment variables
- Build configuration
- Health checks

### Kubernetes

Files generated:
- namespace.yaml
- deployment.yaml
- service.yaml
- ingress.yaml (if --ingress set)
- pvc.yaml (for SQLite)
- configmap.yaml
- secret.yaml.example
- litestream-configmap.yaml (if --backup=litestream)
- README.md

Key features:
- Namespace isolation
- Resource limits
- Liveness/readiness probes
- Ingress configuration
- Storage class selection

## Implementation Phases

1. **Phase 1:** Core architecture (types, generator interface)
2. **Phase 2:** Docker provider (simplest, testable locally)
3. **Phase 3:** Fly.io provider
4. **Phase 4:** DigitalOcean provider
5. **Phase 5:** Kubernetes provider
6. **Phase 6:** CI/CD generation (GitHub Actions)
7. **Phase 7:** Validation command
8. **Phase 8:** Info command
9. **Phase 9:** Integration testing
10. **Phase 10:** Documentation and examples

## Success Criteria

- All 4 providers generate working configs
- All flags work correctly with validation
- lvt stack validate catches common errors
- lvt stack info shows comprehensive overview
- GitHub Actions CI generated by default
- Docker compose works locally (verified)
- All E2E tests pass
- Generated READMEs are comprehensive
- User can deploy to production without manual config editing

## Future Enhancements

- GitLab CI/CD support
- Additional storage providers (Cloudflare R2, Wasabi)
- Additional Redis providers (Redis Cloud)
- Cost estimation improvements
- Multi-region configuration for all providers
- Terraform generation option
- Environment-specific configs (staging, production)
- Auto-detection of existing infrastructure
