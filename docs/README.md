# StateTemplate Documentation

This directory contains comprehensive documentation for the StateTemplate library, optimized for both developers and AI assistants.

## 🗺️ Documentation Overview

### [ARCHITECTURE.md](ARCHITECTURE.md) - Technical Deep Dive

**Purpose**: Complete technical architecture and implementation details
**Best for**: Understanding internals, debugging, contributing to core

**Contents:**

- Core component architecture (Renderer, TemplateTracker, FragmentExtractor, TemplateAnalyzer)
- Data flow diagrams and sequence charts (mermaid visualizations)
- Fragment types and processing pipeline
- Update system architecture with real-time WebSocket integration
- Performance characteristics and scalability considerations
- Technical debt analysis and improvement roadmap
- Test coverage status and gap analysis
- Security model and considerations

### [API_DESIGN.md](API_DESIGN.md) - Developer Reference

**Purpose**: Complete public API documentation and usage patterns
**Best for**: Integration, development, API usage

**Contents:**

- All public types (`Renderer`, `Update`, `RangeInfo`) with examples
- Constructor and configuration options
- Template parsing methods (Parse, ParseFiles, ParseGlob, ParseFS)
- Real-time update lifecycle (SetInitialData, GetUpdateChannel, SendUpdate, Start, Stop)
- Comprehensive usage examples and integration patterns
- Error handling patterns and troubleshooting
- Performance best practices and optimization guidelines
- Thread safety documentation and concurrency patterns

### [EXAMPLES.md](EXAMPLES.md) - Practical Usage Guide

**Purpose**: Real-world examples and integration patterns
**Best for**: Getting started, learning patterns, copy-paste solutions

**Contents:**

- Quick start guide with basic template rendering
- Real-time update examples with WebSocket integration
- File-based and embedded template usage
- Complete web application example with Gin framework
- Range fragment examples for dynamic list operations
- Advanced template features (blocks, conditionals, variables)
- Integration with popular frameworks (Gorilla WebSocket, standard library)
- Migration guide from earlier versions

### [.github/instructions/llm-instructions.md](../.github/instructions/llm-instructions.md) - AI Assistant Guide

**Purpose**: Comprehensive guidance for Language Learning Models working with StateTemplate
**Best for**: AI assistants, automated analysis, code generation

**Contents:**

- Project overview and mental models for LLM understanding
- Repository structure with priority guidance for code analysis
- Fragment types and template action reference
- Development workflow and validation requirements
- Testing patterns and debugging approaches
- Code organization rules and best practices
- Architecture understanding with critical concepts highlighted

## 🚀 Quick Reference

### Basic Usage Pattern

```go
// 1. Create renderer
renderer := statetemplate.NewRenderer()

// 2. Parse template
err := renderer.Parse(`<h1>{{.Title}}</h1>`)

// 3. Set initial data and render
html, err := renderer.SetInitialData(data)

// 4. Get update channel for real-time changes
updateChan := renderer.GetUpdateChannel()

// 5. Send updates
renderer.SendUpdate(newData)
```

### Template Fragment Types

| Type            | Example                          | Use Case             |
| --------------- | -------------------------------- | -------------------- |
| **Simple**      | `{{.Title}}`                     | Direct field display |
| **Conditional** | `{{if .Show}}...{{end}}`         | Dynamic visibility   |
| **Range**       | `{{range .Items}}...{{end}}`     | List rendering       |
| **Block**       | `{{block "header" .}}...{{end}}` | Template composition |

### Common Integration Pattern

```go
// WebSocket integration example
go func() {
    for update := range renderer.GetUpdateChannel() {
        // Send update to WebSocket clients
        websocketBroadcast(update)
    }
}()

// Start real-time processing
renderer.Start()
defer renderer.Stop()
```

## 📚 Learning Path

### For New Developers

1. **Start Here**: [EXAMPLES.md](EXAMPLES.md) - Basic usage and patterns
2. **Deep Dive**: [API_DESIGN.md](API_DESIGN.md) - Complete API reference
3. **Advanced**: [ARCHITECTURE.md](ARCHITECTURE.md) - Internal architecture

### For AI Assistants

1. **Essential**: [llm-instructions.md](../.github/instructions/llm-instructions.md) - LLM-specific guidance
2. **Context**: [ARCHITECTURE.md](ARCHITECTURE.md) - Technical understanding
3. **Patterns**: [EXAMPLES.md](EXAMPLES.md) - Usage patterns

### For Contributors

1. **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md) - Understanding internals
2. **Testing**: Component-specific instructions in `.github/instructions/`
3. **API**: [API_DESIGN.md](API_DESIGN.md) - Public interface stability

## 🔧 Development Workflow

### Documentation Standards

- **All documentation** (except root README.md) MUST be in `docs/` directory
- **Mermaid diagrams** preferred for architecture visualization
- **Code examples** should be runnable and tested
- **Version compatibility** noted for breaking changes

### Validation Requirements

```bash
# MANDATORY before any commit
./scripts/validate-ci.sh
```

This ensures:

- All tests pass
- Code formatting consistency
- No linting errors
- Documentation links are valid

## 🎯 Documentation Maintenance

### Update Triggers

| Change Type              | Update Required             |
| ------------------------ | --------------------------- |
| **Architecture changes** | ARCHITECTURE.md             |
| **Public API changes**   | API_DESIGN.md               |
| **New features**         | EXAMPLES.md + API_DESIGN.md |
| **LLM workflow**         | llm-instructions.md         |

### Quality Standards

- **Examples must work**: All code examples should be copy-pasteable and functional
- **Diagrams stay current**: Mermaid diagrams updated with architecture changes
- **Cross-references**: Maintain links between related documentation sections
- **Version notes**: Document breaking changes and migration paths

---

_This documentation structure supports StateTemplate v1.x. See individual files for version-specific details and migration guidance._
