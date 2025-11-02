# `lvt gen stack` Implementation Plan

## Overview

Adding stack deployment generation to the `lvt` CLI tool. This enables users to generate deployment configurations for various platforms with a single command.

**Branch**: `feature/lvt-gen-stack`
**Started**: 2025-11-02

## Providers (Implementation Order)

1. ⬜ **docker** - Docker Compose (simplest, local/VPS deployment)
2. ⬜ **fly** - Fly.io (single node, managed platform)
3. ⬜ **do** - DigitalOcean App Platform (managed platform)
4. ⬜ **k8s** - Kubernetes (any cluster: GKE, EKS, DO, etc.)

## Command Structure

```bash
# Generation (under lvt gen)
lvt gen stack <provider> [flags]

# Management (under lvt stack)
lvt stack validate [provider]
lvt stack info [provider]
```

### Flags

```bash
# Database options
--db <sqlite|postgres|none>
--skip-db

# Integration options
--redis <upstash|fly|none>
--skip-redis
--storage <s3|do-spaces|b2|none>
--skip-storage
--skip-all-integrations

# CI/CD options (GitHub Actions is default)
--ci <github|gitlab|none>
--skip-ci / --no-ci

# Provider-specific
--multi-region              # fly, k8s only
--namespace <name>          # k8s only
--ingress <nginx|traefik|none>  # k8s only
--registry <ghcr|docker|gcr|ecr>  # k8s only
```

## Implementation Phases

### ⬜ Phase 1: Project Setup

- Create feature branch
- Create this progress tracking file
- Run baseline tests
- Update todos

### ⬜ Phase 2: Core Architecture

- Create cmd/lvt/commands/gen_stack.go
- Create cmd/lvt/commands/stack.go
- Update cmd/lvt/commands/gen.go routing
- Create cmd/lvt/internal/stack/ package:
  - types.go - Common types
  - generator.go - Base interface
  - docker/, fly/, digitalocean/, k8s/ - Provider packages
  - validator/ - Validation engine
  - info/ - Info gathering
  - ci/github/ - CI generation
- Implement flag parsing
- Add E2E test for flags

### ⬜ Phase 3: Docker Provider

- Create internal/stack/docker/generator.go
- Add templates in all kits (multi, single, simple):
  - templates/stack/docker/docker-compose.yml.tmpl
  - templates/stack/docker/docker-compose.postgres.yml.tmpl
  - templates/stack/docker/Dockerfile.tmpl
  - templates/stack/docker/.dockerignore.tmpl
  - templates/stack/docker/README.md.tmpl
  - templates/stack/docker/.env.example.tmpl
- Implement Docker generator
- Generate .lvtstack tracking file
- Add E2E tests
- Manual test: docker compose up

### ⬜ Phase 4: Fly.io Provider

- Create internal/stack/fly/generator.go
- Add templates:
  - fly.toml.tmpl (SQLite)
  - fly.postgres.toml.tmpl
  - Dockerfile.tmpl
  - litestream.yml.tmpl
  - README.md.tmpl
- Implement Fly generator
- Add E2E tests
- Manual test (if Fly account available)

### ⬜ Phase 5: DigitalOcean Provider

- Create internal/stack/digitalocean/generator.go
- Add templates:
  - app-spec.yaml.tmpl
  - Dockerfile.tmpl (reuse)
  - README.md.tmpl
- Implement DO generator
- Add E2E tests

### ⬜ Phase 6: Kubernetes Provider

- Create internal/stack/k8s/generator.go
- Add templates:
  - namespace.yaml.tmpl
  - deployment.yaml.tmpl (SQLite + Postgres variants)
  - service.yaml.tmpl
  - ingress.yaml.tmpl
  - pvc.yaml.tmpl
  - configmap.yaml.tmpl
  - secret.yaml.example.tmpl
  - litestream-config.yaml.tmpl
  - README.md.tmpl
- Implement K8s generator
- Add E2E tests

### ⬜ Phase 7: CI/CD (GitHub Actions)

- Create internal/stack/ci/github/ package
- Add templates:
  - deploy-docker.yml.tmpl
  - deploy-fly.yml.tmpl
  - deploy-do.yml.tmpl
  - deploy-k8s.yml.tmpl
  - test.yml.tmpl
- Integrate into all providers (default)
- Add E2E tests

### ⬜ Phase 8: Integrations (Redis, Storage)

- Redis templates for each provider
- Object storage templates (S3, DO Spaces, B2)
- Update README templates
- Add E2E tests

### ⬜ Phase 9: Validation Command

- Create internal/stack/validator/
  - validator.go - Interface
  - docker.go
  - fly.go
  - digitalocean.go
  - k8s.go
  - common.go - Shared validation
- Implement validation checks:
  - Config syntax (YAML/TOML)
  - Required fields
  - Secret validation
  - Tool availability
  - User modification detection
- User-friendly error formatting
- Add E2E tests

### ⬜ Phase 10: Info Command

- Create internal/stack/info/
  - info.go - Interface
  - Provider-specific info files
  - cost.go - Cost estimation
- Auto-detection of stacks
- .lvtstack parsing
- Required secrets collection
- Tool checking
- Cost estimation
- Formatted output
- Add E2E tests

### ⬜ Phase 11: Multi-region (Optional)

- Implement --multi-region flag
- Fly: primary + replicas
- K8s: multi-cluster configs
- Update READMEs
- Add E2E tests

### ⬜ Phase 12: Final Testing

- Run full E2E suite
- Update golden files
- Manual testing per provider
- Update project README
- Update CHANGELOG
- Code review

## Files to Create

### Commands

- cmd/lvt/commands/gen_stack.go
- cmd/lvt/commands/stack.go

### Internal Packages

- cmd/lvt/internal/stack/types.go
- cmd/lvt/internal/stack/generator.go
- cmd/lvt/internal/stack/docker/generator.go
- cmd/lvt/internal/stack/fly/generator.go
- cmd/lvt/internal/stack/digitalocean/generator.go
- cmd/lvt/internal/stack/k8s/generator.go
- cmd/lvt/internal/stack/validator/*.go
- cmd/lvt/internal/stack/info/*.go
- cmd/lvt/internal/stack/ci/github/*.go

### Templates (per kit: multi, single, simple)

- templates/stack/docker/*.tmpl (6 files)
- templates/stack/fly/*.tmpl (5 files)
- templates/stack/digitalocean/*.tmpl (3 files)
- templates/stack/k8s/*.tmpl (9 files)
- templates/stack/ci/github/*.tmpl (5 files)

### Tests

- cmd/lvt/e2e/gen_stack_test.go
- cmd/lvt/e2e/stack_validate_test.go
- cmd/lvt/e2e/stack_info_test.go

## Generated File Structure (Example)

```
myapp/
├── deploy/
│   ├── docker/
│   │   ├── docker-compose.yml
│   │   ├── Dockerfile
│   │   ├── .dockerignore
│   │   ├── .env.example
│   │   └── README.md
│   ├── fly/
│   │   ├── fly.toml
│   │   ├── Dockerfile
│   │   ├── litestream.yml
│   │   └── README.md
│   ├── digitalocean/
│   │   ├── app-spec.yaml
│   │   └── README.md
│   └── k8s/
│       ├── namespace.yaml
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── ingress.yaml
│       └── README.md
├── .github/workflows/
│   ├── deploy-docker.yml
│   ├── deploy-fly.yml
│   ├── deploy-do.yml
│   └── deploy-k8s.yml
└── .lvtstack  # Tracking file
```

## .lvtstack Tracking File Format

```yaml
version: 1
provider: docker
database: sqlite
backup: litestream
integrations:
  redis: none
  storage: s3
ci: github
generated_at: 2025-11-02T10:00:00Z
generator_version: 0.1.0
files:
  - path: deploy/docker/docker-compose.yml
    checksum: abc123...
    modified: false
  - path: deploy/docker/Dockerfile
    checksum: def456...
    modified: true  # User edited
```

## Success Criteria

- ✅ All 4 providers generate working configs
- ✅ All skip flags work correctly
- ✅ lvt stack validate catches errors
- ✅ lvt stack info shows comprehensive overview
- ✅ GitHub Actions CI generated by default
- ✅ Docker compose works locally
- ✅ All E2E tests pass
- ✅ Generated READMEs are comprehensive

## Notes & Decisions

### 2025-11-02 - Initial Planning

- Decided on 4 providers: docker, fly, do, k8s
- Docker first (simplest to test locally)
- GitHub Actions as default CI
- SQLite with Litestream as default for simple apps
- .lvtstack file for tracking and validation

---

## Current Status

**Phase**: 1 - Project Setup
**Last Updated**: 2025-11-02
**Next Steps**: Create feature branch, run baseline tests, create core architecture
