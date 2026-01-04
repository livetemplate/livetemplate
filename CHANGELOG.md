# Changelog

All notable changes to LiveTemplate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


<a name="v0.7.10"></a>
## [v0.7.10] - 2026-01-04

### Bug Fixes

- handle range→else transitions in top-level range handling


<a name="v0.7.9"></a>
## [v0.7.9] - 2026-01-03

### Bug Fixes

- invalidate registry when conditional becomes empty ([#81](https://github.com/livefir/livetemplate/issues/81))


<a name="v0.7.8"></a>
## [v0.7.8] - 2025-12-27

### Bug Fixes

- **diff:** detect tree node changes when statics differ
- **mount:** enable flash messages on HTTP redirects with query params


<a name="v0.7.7"></a>
## [v0.7.7] - 2025-12-26

### Features

- add per-connection flash messages ([#79](https://github.com/livefir/livetemplate/issues/79))


<a name="v0.7.6"></a>
## [v0.7.6] - 2025-12-25

### Features

- add query parameter support for Mount and action handlers ([#78](https://github.com/livefir/livetemplate/issues/78))


<a name="v0.7.5"></a>
## [v0.7.5] - 2025-12-24

### Bug Fixes

- handle non-TreeNode to TreeNode transitions in range updates ([#77](https://github.com/livefir/livetemplate/issues/77))
- handle non-TreeNode to TreeNode transitions in range updates


<a name="v0.7.4"></a>
## [v0.7.4] - 2025-12-23

### Bug Fixes

- ensure Range.Statics populated for empty→items transitions ([#76](https://github.com/livefir/livetemplate/issues/76))


<a name="v0.7.3"></a>
## [v0.7.3] - 2025-12-22

### Bug Fixes

- support heterogeneous range items with per-item statics ([#75](https://github.com/livefir/livetemplate/issues/75))


<a name="v0.7.2"></a>
## [v0.7.2] - 2025-12-20

### Bug Fixes

- add type guard in SetDynamic to prevent raw structs in tree dynamics ([#74](https://github.com/livefir/livetemplate/issues/74))

### Features

- action.go updates for livepage ([#73](https://github.com/livefir/livetemplate/issues/73))


<a name="v0.7.1"></a>
## [v0.7.1] - 2025-12-14

### Bug Fixes

- mark range statics path in registry for proper caching ([#72](https://github.com/livefir/livetemplate/issues/72))


<a name="v0.7.0"></a>
## [v0.7.0] - 2025-12-10

### Documentation

- update all documentation for Controller+State API (v0.7.0) ([#70](https://github.com/livefir/livetemplate/issues/70))

### Features

- add component template registration support ([#71](https://github.com/livefir/livetemplate/issues/71))


<a name="v0.6.0"></a>
## [v0.6.0] - 2025-12-04


<a name="v0.5.2"></a>
## [v0.5.2] - 2025-12-03

### Documentation

- update client-attributes reference with reactive attributes and more ([#65](https://github.com/livefir/livetemplate/issues/65))
- add reactive attributes proposal ([#64](https://github.com/livefir/livetemplate/issues/64))

### Features

- store pattern redesign with automatic method dispatch ([#66](https://github.com/livefir/livetemplate/issues/66))


<a name="v0.5.1"></a>
## [v0.5.1] - 2025-11-30

### Documentation

- add authentication and session reference documentation ([#63](https://github.com/livefir/livetemplate/issues/63))


<a name="v0.5.0"></a>
## [v0.5.0] - 2025-11-30

### Documentation

- update documentation for Session API ([#62](https://github.com/livefir/livetemplate/issues/62))
- improve README structure and narrative flow ([#59](https://github.com/livefir/livetemplate/issues/59))

### Features

- add Session API for server-initiated actions ([#61](https://github.com/livefir/livetemplate/issues/61))
- add HTTP methods to ActionContext for authentication (v0.5) ([#60](https://github.com/livefir/livetemplate/issues/60))
- add coverage targets to Makefile ([#57](https://github.com/livefir/livetemplate/issues/57))


<a name="v0.4.2-debug.2"></a>
## [v0.4.2-debug.2] - 2025-11-22

### Bug Fixes

- add log package import for debug logging

### Documentation

- update investigation with breakthrough findings from timing instrumentation


<a name="v0.4.2-debug.1"></a>
## [v0.4.2-debug.1] - 2025-11-22


<a name="v0.4.1"></a>
## [v0.4.1] - 2025-11-22

### Bug Fixes

- use async WebSocket Send() instead of blocking WriteMessage() ([#56](https://github.com/livefir/livetemplate/issues/56))


<a name="v0.4.0"></a>
## [v0.4.0] - 2025-11-22

### Code Refactoring

- **registry:** achieve Grade A code quality for async WebSocket ([#55](https://github.com/livefir/livetemplate/issues/55))


<a name="v0.3.2"></a>
## [v0.3.2] - 2025-11-20

### Bug Fixes

- convert validation error field names to lowercase


<a name="v0.3.1"></a>
## [v0.3.1] - 2025-11-19

### Bug Fixes

- send live tree update after upload completion ([#54](https://github.com/livefir/livetemplate/issues/54))
- send live tree update after upload completion ([#53](https://github.com/livefir/livetemplate/issues/53))

### Features

- Phoenix LiveView-inspired file upload system v0.3.0 ([#52](https://github.com/livefir/livetemplate/issues/52))


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


<a name="v0.1.3"></a>
## [v0.1.3] - 2025-11-07


<a name="ls"></a>
## [ls] - 2025-11-07

### Bug Fixes

- update release script for Go-only releases
- use absolute paths for replace directives in cross-repo tests
- resolve race conditions in RedisBroadcaster

### Code Refactoring

- API reduction for v0.2.0 - reduce public API surface area ([#23](https://github.com/livefir/livetemplate/issues/23))

### Documentation

- update RELEASE.md for Go-only releases

### Features

- Code review backlog implementation - Issues [#12](https://github.com/livefir/livetemplate/issues/12)-52 ([#24](https://github.com/livefir/livetemplate/issues/24))
- add comprehensive unit tests for internal packages ([#22](https://github.com/livefir/livetemplate/issues/22))

### BREAKING CHANGE


SessionStore methods now require context.Context parameter

This change adds proper context propagation throughout the session store
layer, enabling timeout control, cancellation, and tracing for all Redis
and session operations.

Changes to SessionStore interface:
- Get(ctx context.Context, groupID string) Stores
- Set(ctx context.Context, groupID string, stores Stores)
- Delete(ctx context.Context, groupID string)
- List(ctx context.Context) []string

Implementation updates:

MemorySessionStore:
- Accepts context parameter for interface compliance
- Operations are in-memory so context not used internally

RedisSessionStore:
- Uses provided context for all Redis operations
- getWithRetry and execPipelineWithRetry now respect context
- Context-aware sleep during retry backoff
- Checks for context cancellation before each retry attempt

Benefits:
- Redis operations can be cancelled mid-flight
- Timeouts are properly respected across retry logic
- Trace IDs and request metadata can be propagated
- Better observability in distributed systems
- Prevents resource leaks from hung operations

Migration guide:
- All SessionStore method calls must now pass context
- Use r.Context() in HTTP handlers for request-scoped context
- Use context.Background() for background operations
- Consider using context.WithTimeout() for bounded operations

### Breaking Change


No - added field to struct, backward compatible.

Note: Only one pre-existing test failure (TestTemplateGenerateTreeWithFuncMap)

🤖 Generated with [Claude Code](https://claude.com/claude-code)


<a name="v0.1.2"></a>
## [v0.1.2] - 2025-11-03

### Bug Fixes

- exclude extracted components from test workflow

### Features

- add cross-repository testing and local development workflows


<a name="v0.1.1"></a>
## [v0.1.1] - 2025-11-03


<a name="v0.1.0"></a>
## v0.1.0 - 2025-11-03

### Bug Fixes

- improve binary build and archive naming in release script
- increase test timeout in release script from 30s to 120s
- remove t.Parallel() from e2e tests to prevent timeout deadlocks
- resolve flaky TestConnectionLimits_ConcurrentAccess test
- add LVT_DEV_MODE to todos e2e test and update hardcoded client paths
- set LVT_DEV_MODE=true in test server startup
- correct observability API usage in example
- prevent accidental .golangci.yml restoration
- resolve all golangci-lint issues and enhance CI validation
- **lvt:** prevent auth tests from generating files in commands/internal ([#19](https://github.com/livefir/livetemplate/issues/19))
- **lvt:** move auth command under lvt gen subcommands ([#17](https://github.com/livefir/livetemplate/issues/17))

### Code Refactoring

- Phase 4 - Extract large functions into internal/diff package
- move remaining build functions to internal/build (Phase 3.2)
- move fingerprinting functions to internal/build
- integrate internal/parse package and remove tree_ast.go
- move tree types to internal/build package
- convert TDD tests to maintainable table-driven format

### Documentation

- Update documentation for repository restructuring
- Complete Milestone 2 - Horizontal Scaling Documentation & Implementation ([#20](https://github.com/livefir/livetemplate/issues/20))
- add first principles document and fix pre-commit hook ([#18](https://github.com/livefir/livetemplate/issues/18))
- update all docs to reflect v1.0 internal package architecture
- Phase 5 - Migration guide, observability example, and test fixtures
- mark refactoring as complete and ready to merge
- update REFACTORING_PROGRESS.md for Phase 3 completion
- update REFACTORING_PROGRESS.md for Phase 3.1 completion
- update REFACTORING_PROGRESS.md - Phase 2 complete
- add comprehensive observability guide
- comprehensive documentation audit and API accuracy fixes ([#4](https://github.com/livefir/livetemplate/issues/4))

### Features

- update release script to use GitHub CLI and publish npm package
- add testcontainers for Redis testing
- Add deployment stack generation (lvt gen stack) ([#21](https://github.com/livefir/livetemplate/issues/21))
- create internal/parse package for template parsing
- observability and architecture documentation
- add comprehensive TDD tests for all Go template actions
- implement comprehensive granular fragment support for all template actions
- implement granular range fragment system with CRUD operations
- **lvt:** add lvt gen auth command - Complete (Phases 1-6) ([#15](https://github.com/livefir/livetemplate/issues/15))


[Unreleased]: https://github.com/livefir/livetemplate/compare/v0.7.10...HEAD
[v0.7.10]: https://github.com/livefir/livetemplate/compare/v0.7.9...v0.7.10
[v0.7.9]: https://github.com/livefir/livetemplate/compare/v0.7.8...v0.7.9
[v0.7.8]: https://github.com/livefir/livetemplate/compare/v0.7.7...v0.7.8
[v0.7.7]: https://github.com/livefir/livetemplate/compare/v0.7.6...v0.7.7
[v0.7.6]: https://github.com/livefir/livetemplate/compare/v0.7.5...v0.7.6
[v0.7.5]: https://github.com/livefir/livetemplate/compare/v0.7.4...v0.7.5
[v0.7.4]: https://github.com/livefir/livetemplate/compare/v0.7.3...v0.7.4
[v0.7.3]: https://github.com/livefir/livetemplate/compare/v0.7.2...v0.7.3
[v0.7.2]: https://github.com/livefir/livetemplate/compare/v0.7.1...v0.7.2
[v0.7.1]: https://github.com/livefir/livetemplate/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/livefir/livetemplate/compare/v0.6.0...v0.7.0
[v0.6.0]: https://github.com/livefir/livetemplate/compare/v0.5.2...v0.6.0
[v0.5.2]: https://github.com/livefir/livetemplate/compare/v0.5.1...v0.5.2
[v0.5.1]: https://github.com/livefir/livetemplate/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/livefir/livetemplate/compare/v0.4.2-debug.2...v0.5.0
[v0.4.2-debug.2]: https://github.com/livefir/livetemplate/compare/v0.4.2-debug.1...v0.4.2-debug.2
[v0.4.2-debug.1]: https://github.com/livefir/livetemplate/compare/v0.4.1...v0.4.2-debug.1
[v0.4.1]: https://github.com/livefir/livetemplate/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/livefir/livetemplate/compare/v0.3.2...v0.4.0
[v0.3.2]: https://github.com/livefir/livetemplate/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/livefir/livetemplate/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/livefir/livetemplate/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/livefir/livetemplate/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/livefir/livetemplate/compare/v0.1.3...v0.2.0
[v0.1.3]: https://github.com/livefir/livetemplate/compare/ls...v0.1.3
[ls]: https://github.com/livefir/livetemplate/compare/v0.1.2...ls
[v0.1.2]: https://github.com/livefir/livetemplate/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/livefir/livetemplate/compare/v0.1.0...v0.1.1
