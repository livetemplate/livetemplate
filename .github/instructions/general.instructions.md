---
applyTo: "**"
---

# Important Notes

- We shouldnt need bash scripts to run tests. Everything should be runnable via `go test`.
- **MANDATORY - NO cmd/ DIRECTORY**: Never create a `cmd/` directory for examples or debug code. All code must go in designated locations:
  - **Examples/Demos**: MUST be in `examples/` directory with descriptive subdirectory names
  - **Debug code**: MUST be in test files as debug test functions, not separate executables
  - **Transient debug code**: Delete immediately after debugging - do not commit
- **MANDATORY - Examples Organization**:
  - All examples and demo code MUST be in `examples/` directory only
  - Use descriptive subdirectory names: `examples/simple/`, `examples/realtime/`, etc.
  - E2E tests for examples MUST be in `examples/e2e/` directory
  - No example or demo code in any other directories (not in `cmd/`, not in root, not in test files)
  - Convert any debug executables to either examples or test cases
- **MANDATORY - Debug Code Policy**:
  - Debug code should be temporary test functions in `*_test.go` files
  - Use names like `TestDebug_SpecificIssue` for temporary debugging
  - Delete debug test functions after debugging is complete
  - Never create separate debug executables or main.go files for debugging
- **MANDATORY**: All documentation and markdown files (except README.md) MUST be created in or moved to the `docs/` directory. This includes:
  - Architecture documentation
  - API documentation
  - Design documents
  - Technical specifications
  - Implementation guides
  - Any .md files that are not README.md in the root
  - When creating new documentation, ALWAYS use the `docs/` directory as the target location
  - When editing existing documentation outside docs/, move it to docs/ first
- **MANDATORY - MILESTONE SUCCESS CRITERIA**: 
  - No milestone can be marked as successful unless `./scripts/validate-ci.sh` passes completely without any issues
  - This includes: all tests passing, code formatting, go vet, golangci-lint, and go mod tidy
  - Any code changes MUST be validated by running `./scripts/validate-ci.sh` before considering work complete
  - Git pre-commit hooks automatically enforce this, but manual validation is also required
- Public API is frozen until the next major release. Do not change public API without a major version bump or an explicit approval from the author.
