# Tree Update Specification Addendum: HTML Fallback Coverage

**Status**: Draft

## Purpose

This addendum documents the behavioural guarantees for LiveTemplate's HTML fallback path so implementation teams can reason about structural parity when the AST walker declines to produce a tree.

## Fallback Strategy Requirements

1. The server MUST route unsupported constructs through `createHTMLStructureBasedTree`, ensuring deterministic block-level segmentation.
2. `analyzeChangeAndCreateTree` MUST delegate to the same segmentation path whenever diff analysis detects a whole-document replacement so that incremental updates reuse the previously established boundaries.
3. Fallback responses MUST avoid synthesising range metadata; clients treat these payloads as opaque HTML snapshots.

## Current Fallback Triggers

| Scenario                                                       | Reason                                                   | Guardrail                                                                                   |
| -------------------------------------------------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| Dynamic template indirection (`{{template (printf ...)}}`)     | Runtime template selection rejected during parsing       | `template_fallback_dynamic_template_test.go`                                                |
| Channel or integer ranges (`{{range .Stream}}`, `{{range 3}}`) | Go templates cannot iterate on channels or literals      | `template_fallback_channel_test.go`                                                         |
| Go 1.25 range control flow (`{{break}}`, `{{continue}}`)       | Control-flow helpers unsupported in the AST phase        | `template_fallback_controlflow_test.go`                                                     |
| Block delegation (`{{block "region" .}}`)                      | Block body resolution happens at runtime                 | `template_fallback_block_test.go`                                                           |
| `with` pipelines returning `iter.Seq`                          | Iterator helpers not yet integrated into tree generation | `template_fallback_with_test.go`                                                            |
| Whole-document diff without stable anchors                     | Ensures diff analysis mirrors fallback segmentation      | `template_diff_analysis_test.go:TestAnalyzeChangeAndCreateTree_EntireContentFallbackParity` |

## Forward Work

- Track labelled control flow support in upstream Go templates; migrate these scenarios from fallback to native coverage when available.
- Coordinate with SDK teams to adopt this addendum before attempting HTML fallback retirement.
