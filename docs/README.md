# LiveTemplate Documentation

This directory contains all documentation for the LiveTemplate core library. For a quick overview and getting started guide, see the [main README](../README.md).

## Getting Started

- **[Main README](../README.md)** - Quick start, examples, and project overview
- **[New Contributor Walkthrough](guides/new-contributor-walkthrough.md)** - 5-phase architecture walkthrough with links to code
- **[Contributing Guide](../CONTRIBUTING.md)** - Development setup, testing, and PR process

## References

Complete API references, configuration, and specifications:

- **[Controller+State Pattern](references/controller-pattern.md)** - Core architecture pattern (v0.7.0+)
- **[Client Attributes](references/client-attributes.md)** - `lvt-*` HTML event binding reference
- **[Error Handling](references/error-handling.md)** - Validation errors, field errors, and error display
- **[Authentication](references/authentication.md)** - Authentication system and custom authenticators
- **[Server Actions](references/server-actions.md)** - Server-initiated actions (TriggerAction API)
- **[Session Management](references/session.md)** - Session stores and connection management
- **[Template Support Matrix](references/template-support-matrix.md)** - Supported Go template features
- **[API Reference](references/api-reference.md)** - Kit manifest schemas and API reference
- **[Configuration](CONFIGURATION.md)** - Environment variables, connection limits, WebSocket settings
- **[File Uploads](uploads.md)** - Phoenix LiveView-inspired upload system
- **[Observability](OBSERVABILITY.md)** - Structured logging (slog) and Prometheus metrics

## Guides

Step-by-step guides and tutorials:

- **[New Contributor Walkthrough](guides/new-contributor-walkthrough.md)** - Comprehensive guide to the 5-phase architecture
- **[Auth Customization](guides/auth-customization.md)** - Custom authentication implementation
- **[lvt CLI Guide](guides/lvt-cli-guide.md)** - Using the `lvt` CLI tool for code generation

Older guides are available in [`archive/guides/`](archive/guides/) for historical reference.

## Architecture & Design

System architecture and design documents:

- **[Architecture Overview](ARCHITECTURE.md)** - 5-phase system flow, package structure, and design decisions
- **[Code Structure](CODE_STRUCTURE.md)** - File-by-file codebase organization
- **[First Principles](design/FIRST_PRINCIPLES.md)** - Core design principles and philosophy
- **[Multi-Session Isolation](design/multi-session-isolation.md)** - Session isolation and multi-user design
- **[Architectural Review](ARCHITECTURAL_REVIEW.md)** - Fingerprint-based diff architecture analysis

## Specifications

Formal specifications for the tree-update protocol:

- **[Tree Update Specification](specifications/tree-update-specification.md)** - Wire format, update sequences, and client caching rules
- **[Tree Update Fallback Addendum](specifications/tree-update-fallback-addendum.md)** - HTML fallback behavior
- **[Test Scenarios](specifications/test-scenarios.md)** - Specification test cases

## Performance

Benchmarking and performance analysis:

- **[Performance Characteristics](performance/performance-characteristics.md)** - Phase-by-phase performance analysis
- **[Benchmarking Guide](performance/benchmarking-guide.md)** - How to run and interpret benchmarks
- **[Known Bottlenecks](performance/known-bottlenecks.md)** - Identified bottlenecks and optimization opportunities
- **[Benchmarking System Design](performance/2025-11-10-benchmarking-system-design.md)** - Benchmark infrastructure architecture

## Operations & Scaling

- **[Scaling Guide](SCALING.md)** - 4 scaling tiers (Hobby to Enterprise), Redis integration
- **[Roadmap](ROADMAP.md)** - Production readiness milestones and progress

## Proposals

Feature proposals and RFCs:

- **[Controller+State Pattern](proposals/002-controller-state-pattern.md)** - Controller+State separation RFC
- **[Async WebSocket Sends](proposals/async-websocket-sends.md)** - Channel-based async WebSocket architecture
- **[Reactive Attributes](proposals/reactive-attributes.md)** - Reactive `lvt-*` attributes proposal
- **[Authentication v0.5](proposals/authentication-v0.5.md)** - Authentication system design
- **[Event Bindings](proposals/bindings-proposal.md)** - Event binding system proposal
- **[Data Binding](proposals/lvt-bind-proposal.md)** - Data binding proposal
- **[Value Deduplication](proposals/value-deduplication-proposal.md)** - Wire format optimization proposal
- **[Store Pattern Redesign](proposals/store-pattern-redesign.md)** - Store pattern redesign (preceded Controller+State)
- **[Bug Fix Persistence](proposals/bug-fix-persistence-and-design-improvements.md)** - Bug fixes and design improvements

## Implementation Plans

Internal tracking for implementation work:

- **[Implementation Status](implementation-plans/IMPLEMENTATION_STATUS.md)** - Current status tracking
- **[Migration Guide](implementation-plans/MIGRATION.md)** - API migration guide
- **[Refactoring Progress](implementation-plans/REFACTORING_PROGRESS.md)** - 5-phase architecture refactoring
- **[Architecture Improvements](implementation-plans/ARCHITECTURE_IMPROVEMENTS.md)** - Proposed improvements
- **[Testing Framework Progress](implementation-plans/TESTING_FRAMEWORK_PROGRESS.md)** - Testing framework development
- **[Strip Statics Refactor](implementation-plans/REFACTOR_STRIP_STATICS.md)** - Static stripping optimization
- **[Validation Conditionals](implementation-plans/BUG-VALIDATION-CONDITIONALS.md)** - Validation and conditional handling
- **[HTML Fallback Coverage](implementation-plans/html-fallback-coverage.md)** - HTML fallback implementation coverage
- **[Implementation Readiness](implementation-plans/implementation-readiness.md)** - Readiness checklist

## Design & Planning History

Historical design documents and feature plans:

- **[Controller+State Pattern Plan](plans/2024-12-06-controller-state-pattern.md)** - Original design plan
- **[lvt gen auth](plans/2025-11-01-lvt-gen-auth.md)** - Auth generation plan
- **[lvt gen stack Design](plans/2025-11-02-lvt-gen-stack-design.md)** - Stack generation design
- **[lvt gen stack Implementation](plans/2025-11-02-lvt-gen-stack-implementation.md)** - Stack generation implementation
- **[Performance Benchmarks Plan](plans/2025-11-10-performance-benchmarks.md)** - Benchmarking plan
- **[Upload Feature Plan](plans/upload-feature-implementation.md)** - File upload implementation plan
- **[lvt gen stack Complete](IMPLEMENTATION_COMPLETE.md)** - Stack generation completion report
- **[lvt gen stack Implementation (alt)](lvt-gen-stack-implementation.md)** - Stack generation details
- **[Simplified Diff Proposal](SIMPLIFIED_DIFF_PROPOSAL.md)** - Simplified diffing algorithm proposal
- **[Fuzz Framework Design](fuzz-framework-design.md)** - Fuzzing test framework design

## Archive

Outdated documentation preserved for historical reference:

- **[archive/guides/](archive/guides/)** - Archived guides (user-guide, CODE_TOUR, kit-development, serve-guide)
- **[archive/client-structure-registry.md](archive/client-structure-registry.md)** - Previous client registry implementation (replaced by fingerprint-based approach)

## Quick Links

| Audience | Start Here |
|----------|------------|
| **New users** | [Main README](../README.md) → [Quick Start](../README.md#quick-start) |
| **New contributors** | [Contributor Walkthrough](guides/new-contributor-walkthrough.md) → [Contributing Guide](../CONTRIBUTING.md) |
| **Building features** | [Controller+State Pattern](references/controller-pattern.md) → [Client Attributes](references/client-attributes.md) |
| **Understanding internals** | [Architecture](ARCHITECTURE.md) → [Code Structure](CODE_STRUCTURE.md) |
| **Deploying** | [Configuration](CONFIGURATION.md) → [Scaling](SCALING.md) |
