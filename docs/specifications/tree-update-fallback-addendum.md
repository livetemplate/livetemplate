# Tree Update Specification Addendum: HTML Fallback Coverage

Version: 2.0.0
Last Updated: 2026-03-07
Status: Final

## Purpose

This addendum documents the behavioral guarantees for LiveTemplate's HTML fallback path when the AST walker declines to produce a tree. The fallback ensures that unsupported template constructs still produce valid, renderable updates via deterministic HTML segmentation.

## Fallback Strategy Requirements

1. The server MUST route unsupported constructs through `createHTMLStructureBasedTree`, ensuring deterministic block-level segmentation.
2. `analyzeChangeAndCreateTree` MUST delegate to the same segmentation path whenever diff analysis detects a whole-document replacement so that incremental updates reuse the previously established boundaries.
3. Fallback responses MUST avoid synthesizing range metadata; clients treat these payloads as opaque HTML snapshots.
4. Fallback trees MUST still use the standard TreeNode wire format (`"s"`, numeric dynamic keys) so that clients process them identically to AST-generated trees.

## Relationship to Main Specification

The main tree-update-specification (v2.0.0) defines the tree format, fingerprinting, and range operations for AST-generated trees. This addendum covers the subset of behavior that applies when the AST path is not available:

- **Structure fingerprinting** still applies: fallback trees have fingerprints computed from their HTML-segmented statics, enabling the same `ClientNeedsStatics()` optimization.
- **Range operations** do NOT apply: fallback trees never produce differential range operations (`"a"`, `"p"`, `"i"`, `"r"`, `"u"`, `"o"`). Changes are expressed as full tree replacements.
- **Wire format** is identical: fallback trees serialize to the same JSON format as AST trees.

## Current Fallback Triggers

| Scenario | Reason | Guardrail Test |
|----------|--------|----------------|
| Dynamic template indirection (`{{template (printf ...)}}`) | Runtime template selection rejected during parsing | `template_fallback_dynamic_template_test.go` |
| Channel or integer ranges (`{{range .Stream}}`, `{{range 3}}`) | Go templates cannot iterate on channels or literals | `template_fallback_channel_test.go` |
| Go 1.25 range control flow (`{{break}}`, `{{continue}}`) | Control-flow helpers unsupported in the AST phase | `template_fallback_controlflow_test.go` |
| Block delegation (`{{block "region" .}}`) | Block body resolution happens at runtime | `template_fallback_block_test.go` |
| `with` pipelines returning `iter.Seq` | Iterator helpers not yet integrated into tree generation | `template_fallback_with_test.go` |
| Whole-document diff without stable anchors | Ensures diff analysis mirrors fallback segmentation | `template_diff_analysis_test.go` |

## Behavioral Guarantees

1. **Determinism**: Given the same template and data, the fallback MUST produce the same tree structure. This ensures fingerprint stability across renders.
2. **Graceful degradation**: Fallback trees are less granular than AST trees (no range operations, coarser dynamics), but they are functionally correct.
3. **Client transparency**: Clients do not need to distinguish between AST-generated and fallback trees. The wire format is identical.
4. **No data loss**: All dynamic content from the template MUST be captured in the fallback tree, even if segmentation is coarser than AST-based parsing.

## Forward Work

- Track labeled control flow support in upstream Go templates; migrate these scenarios from fallback to native AST coverage when available.
- Coordinate with SDK teams to adopt this addendum before attempting HTML fallback retirement.
- Consider adding telemetry to track fallback frequency in production to prioritize AST coverage gaps.
