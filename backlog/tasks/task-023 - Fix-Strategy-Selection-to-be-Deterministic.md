---
id: task-023
title: Fix Strategy Selection to be Deterministic
status: Done
assignee: []
created_date: '2025-08-14 04:42'
updated_date: '2025-08-14 17:17'
labels: []
dependencies: []
---

## Description

Remove confidence-based strategy selection and replace with deterministic rule-based approach for predictable library behavior

## Acceptance Criteria

- [ ] Strategy selection uses binary rules based on change types
- [ ] not confidence thresholds
- [ ] Text-only changes always use Strategy 1 (Static/Dynamic)
- [ ] Attribute changes always use Strategy 2 (Markers)
- [ ] Structural changes always use Strategy 3 (Granular)
- [ ] Complex changes always use Strategy 4 (Replacement)
- [ ] Confidence scores only used for quality metrics and debugging info
- [ ] Updated HLD.md with deterministic strategy selection rules
- [ ] Updated LLD.md with rule-based classification logic
- [ ] Updated CLAUDE.md with deterministic guidance
- [ ] Implementation refactored to remove confidence-based selection
- [ ] All tests updated to reflect deterministic behavior
- [ ] Library behavior is completely predictable for same template constructs

## Implementation Notes

Duplicate of task-024 which has already been completed. All acceptance criteria for deterministic strategy selection have been implemented in task-024:

- Strategy selection is now deterministic based on HTML diff patterns
- Confidence-based logic completely removed from codebase
- Deterministic rules implemented: text-only → Strategy 1, attribute → Strategy 2, structural → Strategy 3, mixed → Strategy 4
- All documentation updated (HLD.md, LLD.md, CLAUDE.md)
- All tests updated to reflect deterministic behavior
- Library behavior is completely predictable

This task is marked as complete since all work was done under task-024.
