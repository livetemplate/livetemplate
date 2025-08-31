---
id: task-055
title: Add Missing Template Constructs to Tree-Based System
status: To Do
assignee: []
created_date: '2025-08-23 15:55'
labels: []
dependencies: []
priority: low
---

## Description

Implement the remaining template constructs identified in TEMPLATE_LLD.md that are marked as PLANNED for the tree-based strategy

## Acceptance Criteria

- [x] Add support for Context With ({{with .User}}...{{end}}) in tree generation
- [ ] Add support for basic Variable declaration and access
- [ ] Add support for Pipeline operations ({{.Name | upper}})
- [ ] Add support for more utility functions (index slice call)
- [x] Add comprehensive tests for new constructs
- [x] Update TEMPLATE_LLD.md to reflect implemented constructs
- [x] Ensure fallback to legacy system for unsupported constructs
- [x] Maintain performance benchmarks with new constructs

## Implementation Plan

1. Research existing TreeFragmentProcessor patterns for context manipulation
2. Implement buildWithTree function in template_tree_simple.go with proper context evaluation
3. Add support for with/else construct handling
4. Create comprehensive tests validating with construct functionality
5. Update TEMPLATE_LLD.md to mark with construct as fully supported

## Implementation Notes

Successfully implemented the Context With construct ({{with .User}}...{{else}}...{{end}}) in the tree-based optimization system:

**Approach taken:**
- Added buildWithTree() function in template_tree_simple.go to handle with context evaluation
- Implemented proper field evaluation with graceful fallback to else case for missing/nil fields  
- Added buildWithElseCase() function to handle else blocks correctly
- Enhanced error handling to treat evaluation failures as falsy values for proper else case execution

**Features implemented:**
- Full support for {{with .Object}}content{{end}} constructs
- Proper else case handling for {{with .Object}}content{{else}}fallback{{end}}
- Context switching - nested content uses the with field as new data context
- Graceful degradation - missing fields trigger else case instead of errors
- Proper nesting level tracking for complex nested with constructs

**Technical decisions:**
- Uses hierarchical boundary parsing similar to conditionals and ranges
- Maintains consistency with existing tree structure generation patterns
- Evaluates with field once and switches data context for nested content
- Handles both truthy evaluation (field exists and non-nil) and falsy (missing/nil) cases

**Modified files:**
- internal/strategy/template_tree_simple.go - Added with construct processing
- internal/strategy/with_construct_test.go - Comprehensive test coverage
- docs/TEMPLATE_LLD.md - Updated to mark with construct as fully supported

**Future enhancements:**
- Variable declarations and pipeline operations remain planned for future releases
- Current implementation focuses on with construct which was the primary requirement
- Additional utility functions and advanced constructs can be added incrementally
