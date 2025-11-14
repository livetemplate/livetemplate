# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


<a name="v0.3.1"></a>
## [v0.3.1] - 2025-11-12

### Features

- **uploads:** Phoenix LiveView-inspired file upload system
  - HTTP multipart uploads for simple forms
  - WebSocket chunked uploads with real-time progress tracking
  - External uploads via presigned URLs (S3, GCS, Azure)
  - Automatic file validation (type, size, count)
  - Template helpers for upload UI (`.lvt.Uploads`, `.lvt.HasUploadError`)
  - Per-session temp file management with automatic cleanup

### API Additions

- **UploadAware interface:** Stores can implement `AllowUploads()` and `ConsumeUpload()` to support uploads
- **UploadConfig type:** Configures upload behavior (accept types, size limits, auto-upload, etc.)
- **UploadEntry type:** Represents upload state (progress, validation, errors)
- **Presigner interface:** Enables custom external upload providers
- **UploadMeta type:** Contains presigned upload configuration
- **S3Presigner:** Built-in AWS S3 presigner implementation

### Documentation

- Comprehensive upload documentation in [docs/uploads.md](docs/uploads.md)
- API reference for all upload types
- Quick start guide with examples
- S3/external upload setup instructions
- Security best practices
- Performance tuning guide
- Troubleshooting section

### Tests

- **upload_test.go:** 10 comprehensive unit tests for public API
- **internal/upload tests:** Full coverage of validation, registry, multipart, protocol
- **s3_presigner_test.go:** 11 tests including path traversal prevention
- **upload_integration_test.go:** End-to-end upload flow testing

### Internal Implementation

- **internal/upload:** Upload registry, validation, multipart parsing, temp file management
- **internal/upload/protocol.go:** WebSocket protocol messages for chunked uploads
- **internal/uploadtypes:** Core upload types (isolated to avoid import cycles)
- **upload_init.go:** Factory function initialization for upload subsystem

### Breaking Changes

None - Upload feature is fully additive and backward compatible.

### Migration Guide

To adopt uploads in existing applications:
1. Implement `UploadAware` interface on stores that need uploads
2. Add `lvt-upload` attributes to file inputs in templates
3. Update client library to v0.1.0+ (if using TypeScript client)

See [docs/uploads.md](docs/uploads.md) for complete migration guide.


<a name="v0.3.0"></a>
## [v0.3.0] - 2025-11-12

### Bug Fixes

- use GOWORK=off in release script to avoid workspace issues
- address minor code review issues
- address code review feedback

### Code Refactoring

- make New() fail-fast on template parsing errors ([#51](https://github.com/livefir/livetemplate/issues/51))

### Documentation

- add optimization task list to performance bottlenecks
- add performance section to README
- add performance characteristics analysis
- add comprehensive benchmarking guide
- document performance bottlenecks from profiling
- add design and implementation plan

### Performance Improvements

- address code review recommendations
- establish performance baseline
- add end-to-end user journey benchmarks
- add end-to-end template benchmarks
- add Phase 4 (Render) and Phase 5 (Send) benchmarks
- add Phase 3 (Diff) benchmarks
- add Phase 2 (Build) benchmarks
- add Phase 1 (Parse) benchmarks


<a name="v0.2.1"></a>
## [v0.2.1] - 2025-11-11

### Bug Fixes

- allow template discovery in internal directories for multi kit support
- template auto-discovery for go run and lvt serve ([#49](https://github.com/livefir/livetemplate/issues/49))
- improve template auto-discovery robustness ([#47](https://github.com/livefir/livetemplate/issues/47))

### Documentation

- remove version-specific references from contributor walkthrough
- create comprehensive contributor walkthrough for 5-phase architecture
- simplify README to focus on core value proposition ([#48](https://github.com/livefir/livetemplate/issues/48))


<a name="v0.2.0"></a>
## [v0.2.0] - 2025-11-09

### Code Refactoring

- improve key generation and fingerprinting robustness
- complete Phase 2 - move 4 functions to internal packages ([#44](https://github.com/livefir/livetemplate/issues/44))
- align template.go with 5-phase architecture ([#43](https://github.com/livefir/livetemplate/issues/43))
- reduce public API surface area from 11 to 7 files ([#46](https://github.com/livefir/livetemplate/issues/46))
- **conditional:** eliminate duplication and improve error handling ([#40](https://github.com/livefir/livetemplate/issues/40))
- **context:** achieve Grade A code quality ([#31](https://github.com/livefir/livetemplate/issues/31))
- **field:** achieve Grade A code quality ([#36](https://github.com/livefir/livetemplate/issues/36))
- **fingerprint:** fix circular detection and improve robustness
- **helpers:** achieve Grade A code quality ([#35](https://github.com/livefir/livetemplate/issues/35))
- **parse:** achieve Grade A code quality ([#38](https://github.com/livefir/livetemplate/issues/38))
- **parse:** achieve Grade A code quality ([#41](https://github.com/livefir/livetemplate/issues/41))
- **prepare:** achieve Grade A code quality ([#34](https://github.com/livefir/livetemplate/issues/34))
- **range:** achieve Grade A code quality ([#37](https://github.com/livefir/livetemplate/issues/37))
- **range_ops:** achieve Grade A code quality ([#33](https://github.com/livefir/livetemplate/issues/33))
- **render:** achieve Grade A code quality ([#42](https://github.com/livefir/livetemplate/issues/42))
- **render:** performance, security, and quality improvements ([#27](https://github.com/livefir/livetemplate/issues/27))
- **template:** achieve Grade A- code quality with 5-phase architecture ([#45](https://github.com/livefir/livetemplate/issues/45))
- **tree_compare:** achieve Grade A code quality ([#32](https://github.com/livefir/livetemplate/issues/32))
- **types:** achieve Grade A quality with comprehensive tests and documentation
- **var_context:** achieve Grade A code quality ([#39](https://github.com/livefir/livetemplate/issues/39))
- **wrapper:** improve security, correctness, and robustness - Grade A ([#29](https://github.com/livefir/livetemplate/issues/29))
