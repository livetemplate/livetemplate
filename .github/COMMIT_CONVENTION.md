# Commit Message Convention

This project follows [Conventional Commits](https://www.conventionalcommits.org/) specification for commit messages. This enables automated changelog generation and semantic versioning.

## Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Components

- **type**: The type of change (required)
- **scope**: The area of the codebase affected (optional)
- **subject**: A brief description of the change (required)
- **body**: A detailed description (optional)
- **footer**: Breaking changes, issue references (optional)

## Types

| Type | Description | Changelog Section | Version Bump |
|------|-------------|-------------------|--------------|
| `feat` | New feature | Features | minor |
| `fix` | Bug fix | Bug Fixes | patch |
| `perf` | Performance improvement | Performance | patch |
| `refactor` | Code refactoring (no functionality change) | Code Refactoring | patch |
| `docs` | Documentation only | Documentation | - |
| `style` | Code style (formatting, whitespace) | - | - |
| `test` | Adding or updating tests | - | - |
| `chore` | Maintenance tasks, dependencies | - | - |
| `ci` | CI/CD changes | - | - |
| `build` | Build system changes | - | - |

## Breaking Changes

Breaking changes trigger a **major** version bump. Indicate them with:
- `BREAKING CHANGE:` in the footer, or
- `!` after the type/scope: `feat!:` or `feat(api)!:`

## Examples

### Feature

```
feat(client): add tree reconstruction from cache

Implement client-side tree reconstruction to reduce initial payload size.
Trees can now be rebuilt from cached static content and dynamic updates.
```

### Bug Fix

```
fix(template): handle nil pointer in range construct

Fixed panic when range construct receives nil slice.
Added nil checks and appropriate error handling.

Fixes #123
```

### Breaking Change

```
feat(api)!: change session store interface

BREAKING CHANGE: SessionStore interface now requires context.Context
parameter for all methods. Update your implementations:

Before:
  Get(sessionID string) (*Session, error)

After:
  Get(ctx context.Context, sessionID string) (*Session, error)
```

### Performance

```
perf(tree): optimize fingerprint calculation

Use xxhash instead of md5 for 3x faster fingerprinting.
```

### Documentation

```
docs(readme): update installation instructions

Added section on installing lvt CLI via various methods.
```

### Refactoring

```
refactor(generator): extract template helpers to separate package

No functionality changes, improved code organization.
```

## Scopes

Common scopes in this project:

- `client` - TypeScript client library
- `template` - Go template engine
- `tree` - Tree generation and diffing
- `cli` - CLI tool (lvt)
- `generator` - Code generator
- `kits` - Kit system
- `components` - Component system
- `serve` - Development server
- `api` - Public API changes

## Best Practices

1. **Keep it short**: Subject line should be ≤ 72 characters
2. **Use imperative mood**: "add feature" not "added feature"
3. **Don't end with period**: Subject line doesn't need punctuation
4. **Capitalize subject**: Start with capital letter
5. **Reference issues**: Add `Fixes #123` or `Closes #456` in footer
6. **Explain why**: Body should explain why the change was needed
7. **One change per commit**: Keep commits atomic and focused

## Commit Message Template

Create `.gitmessage` in your home directory:

```
# <type>(<scope>): <subject>
#
# <body>
#
# <footer>

# Types: feat, fix, perf, refactor, docs, style, test, chore, ci, build
# Scope: client, template, tree, cli, generator, kits, components, serve, api
# Subject: imperative mood, no period, ≤72 chars
# Body: explain what and why (optional)
# Footer: breaking changes, issue refs (optional)
```

Then configure git:

```bash
git config --global commit.template ~/.gitmessage
```

## Automated Release Process

Our release script uses these conventions to:

1. **Generate Changelog**: Groups commits by type
2. **Determine Version**: Calculates next version from commit types
3. **Create Release Notes**: Formats commits for GitHub releases

Example changelog output:

```markdown
## [v0.2.0] - 2025-01-15

### Features
- **client:** add tree reconstruction from cache
- **cli:** add interactive mode for resource generation

### Bug Fixes
- **template:** handle nil pointer in range construct
- **tree:** fix fingerprint collision for nested structures

### Performance
- **tree:** optimize fingerprint calculation using xxhash
```

## Tools

### Commitizen (Optional)

For interactive commit message creation:

```bash
npm install -g commitizen cz-conventional-changelog
echo '{ "path": "cz-conventional-changelog" }' > ~/.czrc
```

Then use `git cz` instead of `git commit`.

### Commitlint (Optional)

For enforcing commit message format:

```bash
npm install -g @commitlint/cli @commitlint/config-conventional
```

## References

- [Conventional Commits Specification](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)

## Questions?

If you're unsure about commit message format, check recent commits for examples:

```bash
git log --oneline -20
```

Or ask in pull request reviews - we're happy to help!
