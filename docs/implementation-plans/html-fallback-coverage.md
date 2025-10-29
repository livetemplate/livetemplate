# HTML Fallback Coverage Tracker

**Branch**: `feat/testing-framework`
**Owner**: LiveTemplate Parser Team
**Status**: Phase 2 (Fallback Hardening) – IN PROGRESS
**Last Updated**: 2025-10-29

---

## Overview

`createHTMLStructureBasedTree` remains the execution safety net when the AST walk cannot faithfully interpret template constructs. Phase 2 focuses on documenting and backstopping every known fallback trigger so we can eventually retire or minimize this path confidently.

---

## Completed Coverage

| Area                        | Scenario                                                | Guardrail                                                                                                         |
| --------------------------- | ------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Comment Handling            | Comment-only template triggers fallback (Phase 1)       | `tree_ast_phase_test.go` – ensures comments do not crash parser and fallback result is consistent                 |
| FuncMap Support             | FuncMap propagation keeps AST parsing intact            | `template_funcmap_test.go` – baseline to ensure fallback is not invoked unnecessarily                             |
| Channel Ranges              | `{{range .Stream}}` over `<-chan` value (no decls)      | `template_fallback_channel_test.go:TestTemplateGenerateInitialTreeFallsBackForChannelRange`                       |
| Channel Ranges w/ Decls     | `{{range $i, $v := .Stream}}` over `<-chan` value       | `template_fallback_channel_test.go:TestTemplateGenerateInitialTreeFallsBackForChannelRangeWithDecls`              |
| HTML Segmentation           | Fallback produces deterministic block segmentation      | `template_fallback_channel_test.go:TestCreateHTMLStructureBasedTreeSegmentsBlockBoundaries`                       |
| Control Flow (Go 1.25)      | `{{break}}` inside `range`                              | `template_fallback_controlflow_test.go:TestTemplateGenerateInitialTreeFallsBackForRangeBreak`                     |
| Control Flow (Go 1.25)      | `{{continue}}` inside `range` with variable decls       | `template_fallback_controlflow_test.go:TestTemplateGenerateInitialTreeFallsBackForRangeContinue`                  |
| Dynamic Template Invocation | Runtime `{{template}}` name resolution (post-flatten)   | `template_fallback_dynamic_template_test.go:TestTemplateGenerateInitialTreeFallsBackForDynamicTemplateInvocation` |
| Block Invocation            | `{{block}}` default defers to dynamic template name     | `template_fallback_block_test.go:TestTemplateGenerateInitialTreeFallsBackForBlockWithDynamicTemplate`             |
| Integer Range Support       | `{{range 3}}` literal loop (Go 1.25)                    | `template_fallback_channel_test.go:TestTemplateGenerateInitialTreeFallsBackForIntegerRange`                       |
| `with` Unsupported Branch   | Pipeline returns `iter.Seq`, forcing fallback           | `template_fallback_with_test.go:TestTemplateGenerateInitialTreeFallsBackForWithIterSeq`                           |
| Diff Analyzer Parity        | Full-content diff analysis reuses fallback segmentation | `template_diff_analysis_test.go:TestAnalyzeChangeAndCreateTree_EntireContentFallbackParity`                       |

Supporting documentation: `docs/specifications/tree-update-fallback-addendum.md` captures the formal fallback guarantees for the tree update specification.

---

## Remaining Targets

1. **Labelled Control Flow** – `{{break label}}` / `{{continue label}}` once Go templates add support; confirm graceful fallback.
2. **`range` over Future Iterables** – extend coverage to Go's forthcoming iterator helpers (e.g., `iter.Seq`, generator funcs) when templates integrate them.
3. **Spec Documentation** – update `docs/specifications/tree-update-specification.md` to call out fallback guarantees once test suite is exhaustive.

---

## Next Steps

1. Monitor Go template releases for labelled control flow support so regressions can migrate from fallback to native coverage when available.
2. Prepare retirement plan for HTML fallback once Remaining Targets are closed and client teams adopt the specification addendum.
