# LiveTemplate Documentation

This directory contains all documentation for LiveTemplate, organized by type.

## Directory Structure

### `/references/`
Complete API references and specifications:
- **[client-attributes.md](references/client-attributes.md)** - `lvt-*` HTML attributes reference
- **[error-handling.md](references/error-handling.md)** - Comprehensive error handling guide
- **[api-reference.md](references/api-reference.md)** - Kit manifest schemas and API reference
- **[template-support-matrix.md](references/template-support-matrix.md)** - Supported Go template features

### `/guides/`
Step-by-step guides and tutorials:
- **[user-guide.md](guides/user-guide.md)** - Getting started with `lvt` CLI
- **[CODE_TOUR.md](guides/CODE_TOUR.md)** - Guided codebase walkthrough
- **[kit-development.md](guides/kit-development.md)** - Creating CSS framework kits (includes components)
- **[serve-guide.md](guides/serve-guide.md)** - Development server usage

### `/design/`
Architecture and design documents:
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture and design decisions
- **[CODE_STRUCTURE.md](CODE_STRUCTURE.md)** - Codebase organization
- **[multi-session-isolation.md](design/multi-session-isolation.md)** - Planned authentication features

### `/proposals/`
Feature proposals and RFCs:
- **[bindings-proposal.md](proposals/bindings-proposal.md)** - Event binding system proposal
- **[lvt-bind-proposal.md](proposals/lvt-bind-proposal.md)** - Data binding proposal
- **[value-deduplication-proposal.md](proposals/value-deduplication-proposal.md)** - Optimization proposal

## Quick Links

### For Users
- Start here: **[User Guide](guides/user-guide.md)**
- API Reference: **[Go API](https://pkg.go.dev/github.com/livefir/livetemplate)** | **[Client Attributes](references/client-attributes.md)**

### For Contributors
- **[CODE_TOUR.md](guides/CODE_TOUR.md)** - Understand the codebase
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System design
- **[CODE_STRUCTURE.md](CODE_STRUCTURE.md)** - File organization
- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute

### For Framework Developers
- **[Design Documents](design/)** - Architecture decisions
- **[Proposals](proposals/)** - Feature proposals
