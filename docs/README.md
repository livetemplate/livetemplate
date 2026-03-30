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
- **[PubSub](references/pubsub.md)** - Distributed broadcasting, Redis channel schema, subscription lifecycle
- **[Template Support Matrix](references/template-support-matrix.md)** - Supported Go template features
- **[API Reference](references/api-reference.md)** - Kit manifest schemas and API reference
- **[Configuration](references/CONFIGURATION.md)** - Environment variables, connection limits, WebSocket settings
- **[File Uploads](references/uploads.md)** - Phoenix LiveView-inspired upload system
- **[Observability](guides/OBSERVABILITY.md)** - Structured logging (slog) and Prometheus metrics

## Guides

Step-by-step guides and tutorials:

- **[Standard HTML Reactivity](guides/standard-html-reactivity.md)** - How standard HTML works reactively and comparison with htmx, Livewire, and LiveView
- **[New Contributor Walkthrough](guides/new-contributor-walkthrough.md)** - Comprehensive guide to the 5-phase architecture

For `lvt` CLI documentation (code generation, auth, migrations), see the [lvt repository](https://github.com/livetemplate/lvt).

Older guides are available in [`archive/guides/`](archive/guides/) for historical reference.

## Architecture & Design

System architecture and design documents:

- **[Architecture Overview](design/ARCHITECTURE.md)** - 5-phase system flow, package structure, and design decisions
- **[Code Structure](design/CODE_STRUCTURE.md)** - File-by-file codebase organization
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

- **[Scaling Guide](guides/SCALING.md)** - 4 scaling tiers (Hobby to Enterprise), Redis integration
- **[Roadmap](../ROADMAP.md)** - Prioritized roadmap (Now/Next/Later/Blocked)

## Proposals

Active feature proposals and RFCs:

- **[Lifecycle Hooks](proposals/lifecycle-hooks-proposal.md)** - `lvt-hook` for JS library integration
- **[Data Binding](proposals/lvt-bind-proposal.md)** - Two-way form data binding (`lvt-bind`) proposal

## Roadmap Tracking

Active implementation tracking:

- **[Architecture Improvements](roadmap/ARCHITECTURE_IMPROVEMENTS.md)** - Client simplification (Phase 6B/6C)
- **[HTML Fallback Coverage](roadmap/html-fallback-coverage.md)** - HTML fallback implementation coverage

## Design & Planning History

Active design documents and future feature plans:

- **[Fuzz Framework Design](design/fuzz-framework-design.md)** - Fuzzing test framework design

## Archive

Completed, implemented, or superseded documentation preserved for historical reference:

- **[archive/guides/](archive/guides/)** - Archived guides (user-guide, CODE_TOUR, kit-development, serve-guide)
- **[archive/client-structure-registry.md](archive/client-structure-registry.md)** - Previous client registry implementation (replaced by fingerprint-based approach)
- **[archive/plans/](archive/plans/)** - Completed plans: Controller+State, uploads, lvt gen stack, lvt gen auth, performance benchmarks
- **[archive/proposals/](archive/proposals/)** - Implemented/declined proposals: Controller+State RFC, async WebSocket, auth v0.5, value dedup, store redesign, bug fix persistence, reactive attributes, event bindings
- **[archive/implementation-plans/](archive/implementation-plans/)** - Completed tracking: scalability roadmap (M1/M2/M3), v1.0 migration guide, implementation status, refactoring progress, strip statics, validation conditionals, testing framework, implementation readiness
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
| **Understanding internals** | [Architecture](design/ARCHITECTURE.md) → [Code Structure](design/CODE_STRUCTURE.md) |
| **Deploying** | [Configuration](references/CONFIGURATION.md) → [Scaling](guides/SCALING.md) |
