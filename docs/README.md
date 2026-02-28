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

Active feature proposals and RFCs:

- **[Reactive Attributes](proposals/reactive-attributes.md)** - Reactive `lvt-*` attributes proposal
- **[Event Bindings](proposals/bindings-proposal.md)** - Event binding system proposal
- **[Data Binding](proposals/lvt-bind-proposal.md)** - Data binding proposal

## Implementation Plans

Active implementation tracking:

- **[Migration Guide](implementation-plans/MIGRATION.md)** - API migration guide
- **[Architecture Improvements](implementation-plans/ARCHITECTURE_IMPROVEMENTS.md)** - Proposed improvements
- **[HTML Fallback Coverage](implementation-plans/html-fallback-coverage.md)** - HTML fallback implementation coverage

## Design & Planning History

- **[Fuzz Framework Design](fuzz-framework-design.md)** - Fuzzing test framework design

## Archive

Completed, implemented, or superseded documentation preserved for historical reference:

- **[archive/guides/](archive/guides/)** - Archived guides (user-guide, CODE_TOUR, kit-development, serve-guide)
- **[archive/client-structure-registry.md](archive/client-structure-registry.md)** - Previous client registry implementation (replaced by fingerprint-based approach)
- **[archive/plans/](archive/plans/)** - Completed plans: Controller+State, uploads, lvt gen stack, lvt gen auth, performance benchmarks
- **[archive/proposals/](archive/proposals/)** - Implemented/declined proposals: Controller+State RFC, async WebSocket, auth v0.5, value dedup, store redesign, bug fix persistence
- **[archive/implementation-plans/](archive/implementation-plans/)** - Completed tracking: implementation status, refactoring progress, strip statics, validation conditionals, testing framework, implementation readiness
- **[archive/ARCHITECTURAL_REVIEW.md](archive/ARCHITECTURAL_REVIEW.md)** - Fingerprint-based diff architecture analysis (all recommendations implemented)
- **[archive/SIMPLIFIED_DIFF_PROPOSAL.md](archive/SIMPLIFIED_DIFF_PROPOSAL.md)** - Simplified diffing algorithm (implemented in v0.8.0)
- **[archive/IMPLEMENTATION_COMPLETE.md](archive/IMPLEMENTATION_COMPLETE.md)** - lvt gen stack completion report
- **[archive/lvt-gen-stack-implementation.md](archive/lvt-gen-stack-implementation.md)** - Stack generation implementation details

## Quick Links

| Audience | Start Here |
|----------|------------|
| **New users** | [Main README](../README.md) → [Quick Start](../README.md#quick-start) |
| **New contributors** | [Contributor Walkthrough](guides/new-contributor-walkthrough.md) → [Contributing Guide](../CONTRIBUTING.md) |
| **Building features** | [Controller+State Pattern](references/controller-pattern.md) → [Client Attributes](references/client-attributes.md) |
| **Understanding internals** | [Architecture](ARCHITECTURE.md) → [Code Structure](CODE_STRUCTURE.md) |
| **Deploying** | [Configuration](CONFIGURATION.md) → [Scaling](SCALING.md) |
