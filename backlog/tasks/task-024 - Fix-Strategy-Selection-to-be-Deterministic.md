---
id: task-024
title: Fix Strategy Selection to be Deterministic
status: Done
assignee: []
created_date: '2025-08-14 13:54'
updated_date: '2025-08-14 15:53'
labels:
  - architecture
  - strategy-selection
  - deterministic
dependencies: []
priority: high
---

## Description

The current strategy selection design uses confidence scores which makes it non-deterministic and unpredictable. This violates the core design principle that library users should be able to predict which strategy will be used for a given template construct. Replace confidence-based selection with deterministic rule-based selection where same template constructs always behave consistently.

## Acceptance Criteria

- [x] Strategy selection is deterministic based on HTML diff patterns
- [x] Same template construct with same change pattern always chooses same strategy
- [x] No confidence thresholds or probabilistic decision making
- [x] Clear rules: text-only → Strategy 1 attribute changes → Strategy 2 structural → Strategy 3 mixed → Strategy 4
- [x] Update HLD.md and LLD.md to reflect deterministic approach
- [x] Remove all confidence-based logic from design documents
- [x] Strategy selection rules are documented and predictable

## Implementation Plan

1. Remove confidence fields from all data structures in internal/diff/
2. Update strategy selection logic to use deterministic rules based on change types
3. Remove all confidence-based tests and validation logic
4. Update documentation (HLD.md, LLD.md, CLAUDE.md) to remove confidence references
5. Ensure strategy selection is purely rule-based: text-only → Strategy 1, attribute → Strategy 2, structural → Strategy 3, mixed → Strategy 4
6. Run tests to validate deterministic behavior

## Implementation Notes

Successfully removed all confidence-based logic from strategy selection system and replaced with deterministic rule-based approach.

**Core Changes Made:**
- Removed Confidence field from StrategyRecommendation and DOMChange structs
- Updated strategy selection logic to use pure rule-based decisions based on HTML change patterns
- Replaced confidence-based tests with deterministic behavior tests
- Updated all documentation (HLD.md, LLD.md, CLAUDE.md) to remove confidence references

**Deterministic Rules Implemented:**
- Text-only changes → Always Strategy 1 (Static/Dynamic)
- Attribute changes → Always Strategy 2 (Markers)
- Structural changes → Always Strategy 3 (Granular)
- Mixed changes → Always Strategy 4 (Replacement)

**Key Benefits:**
- Same template constructs now always behave consistently
- Performance is predictable and debuggable
- Library behavior is deterministic and reliable
- No unpredictable confidence thresholds

**Files Modified:**
- internal/diff/classifier.go - Removed confidence fields and logic
- internal/diff/comparator.go - Removed confidence from DOMChange struct
- internal/diff/differ.go - Updated validation and strategy selection
- All test files - Updated to test deterministic behavior
- docs/HLD.md, docs/LLD.md, CLAUDE.md - Removed confidence references

All tests now pass and strategy selection is completely deterministic.
