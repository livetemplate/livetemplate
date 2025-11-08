package livetemplate

import "github.com/livetemplate/livetemplate/internal/build"

// This file provides the public API for tree-based template data structures.
//
// # Version 0.2.0 API Reduction
//
// As part of v0.2.0, the livetemplate package underwent a significant API reduction,
// moving implementation details to internal packages while maintaining backward compatibility
// through type aliases. The public API was reduced by 33% (from 18 to 12 files) while
// preserving all user-facing functionality.
//
// # Type Re-exports
//
// This file re-exports core types from internal/build for backward compatibility.
// These type aliases allow existing code to continue working while enabling internal
// refactoring and preventing external dependencies on implementation details.
//
// # When to Use Public vs Internal Imports
//
// Use the public package (github.com/livetemplate/livetemplate) for:
//   - All application code
//   - External libraries integrating with livetemplate
//   - Accessing stable, documented APIs
//
// Avoid importing internal packages directly:
//   - Internal packages may change without notice
//   - Type aliases ensure you get the same types with a stable import path
//   - The public API is the only supported interface
//
// # Type Aliases vs New Types
//
// Go type aliases (using =) create true aliases, not new types:
//
//	type TreeNode = build.TreeNode  // Same type, different path
//
// This means:
//   - Zero runtime overhead
//   - Type assertions work as expected
//   - Reflection shows the underlying type name (build.TreeNode)
//   - Full API compatibility with the source type

type (
	// TreeNode represents a node in the template tree structure.
	//
	// TreeNode is the core data structure for representing both static and dynamic
	// template content. It supports nested structures, range operations, and efficient
	// wire format transmission.
	//
	// Structure:
	//   - Statics: Static HTML parts (key "s" in JSON)
	//   - Dynamics: Dynamic values indexed by position (keys "0", "1", etc.)
	//   - Fingerprint: Hash for change detection (key "f")
	//   - Range: Range operation data if this is a list (key "d")
	//   - Metadata: Additional metadata like ID keys (key "m")
	//
	// See examples for common usage patterns.
	TreeNode = build.TreeNode

	// RangeData represents data for range operations in templates.
	//
	// RangeData is used when rendering lists or iterating over collections.
	// It contains the items being rendered and the static HTML template that
	// wraps each item.
	//
	// Structure:
	//   - Items: List of range operations (update, insert, remove, reorder)
	//   - Statics: Static HTML parts for rendering each item
	//
	// Example: Rendering a list of users
	//   items := []interface{}{user1, user2, user3}
	//   statics := []string{"<li>", "</li>"}
	//   rangeData := NewRangeData(items, statics)
	RangeData = build.RangeData

	// TreeMetadata contains metadata about a tree node.
	//
	// TreeMetadata provides additional information about tree nodes, particularly
	// for range operations where items need unique identifiers.
	//
	// Structure:
	//   - IDKey: Field name used as unique identifier for range items
	//
	// Example: Tracking list items by ID
	//   metadata := NewTreeMetadata("id")  // Use "id" field as key
	TreeMetadata = build.TreeMetadata

	// TreeGenerationContext provides context for tree generation.
	//
	// Deprecated: Use build.Context instead. This alias is maintained for
	// backward compatibility but may be removed in a future version.
	// Internal code should migrate to build.Context directly.
	TreeGenerationContext = build.Context
)

// Constructor functions for creating tree data structures.

var (
	// NewTreeNode creates a new empty TreeNode with initialized dynamic map.
	//
	// The returned TreeNode has no statics but is ready to accept dynamic values
	// via SetDynamic. This is useful for programmatically building trees.
	//
	// Returns:
	//   - *TreeNode with empty statics and initialized dynamics map
	//
	// Example:
	//   node := NewTreeNode()
	//   node.SetDynamic("0", "Hello, World!")
	NewTreeNode = build.NewTreeNode

	// NewTreeNodeWithStatics creates a new TreeNode with the given static HTML parts.
	//
	// Static parts are the unchanging HTML structure that surrounds dynamic content.
	// For a template like "<div>{{.Name}}</div>", the statics would be ["<div>", "</div>"].
	//
	// Parameters:
	//   - statics: Slice of static HTML strings
	//
	// Returns:
	//   - *TreeNode with statics set and initialized dynamics map
	//
	// Example:
	//   statics := []string{"<div class='user'>", "</div>"}
	//   node := NewTreeNodeWithStatics(statics)
	NewTreeNodeWithStatics = build.NewTreeNodeWithStatics

	// NewRangeData creates a new RangeData for list rendering operations.
	//
	// RangeData is used when rendering lists or collections. The items can be
	// any JSON-serializable values, and the statics wrap each item.
	//
	// Parameters:
	//   - items: Slice of items to render (can be any type)
	//   - statics: Static HTML parts wrapping each item
	//
	// Returns:
	//   - *RangeData ready for use in a TreeNode
	//
	// Example:
	//   users := []interface{}{user1, user2}
	//   statics := []string{"<li>", "</li>"}
	//   rangeData := NewRangeData(users, statics)
	NewRangeData = build.NewRangeData

	// NewTreeMetadata creates metadata for a tree node with the given ID key.
	//
	// Metadata is particularly useful for range operations where items need
	// to be tracked by a unique identifier for efficient updates.
	//
	// Parameters:
	//   - idKey: Name of the field to use as unique identifier
	//
	// Returns:
	//   - *TreeMetadata with the specified ID key
	//
	// Example:
	//   metadata := NewTreeMetadata("userId")  // Track items by "userId" field
	NewTreeMetadata = build.NewTreeMetadata

	// NewTreeGenerationContext creates a context for first render.
	//
	// Deprecated: Use build.NewContext instead. This alias is maintained for
	// backward compatibility but may be removed in a future version.
	// Internal code should migrate to build.NewContext directly.
	NewTreeGenerationContext = build.NewContext

	// NewUpdateContext creates a context for template updates.
	//
	// Update contexts are used for subsequent renders after the initial render.
	// They control whether static HTML parts should be included in the generated
	// tree based on what the client has already cached.
	//
	// Parameters:
	//   - clientStructures: Map of field paths the client has already received
	//
	// Returns:
	//   - *Context configured for updates (IsFirstRender=false, IncludeStatics=false)
	//
	// Example:
	//   clientStructures := map[string]bool{"0": true, "1.2": true}
	//   ctx := NewUpdateContext(clientStructures)
	NewUpdateContext = build.NewUpdateContext

	// FromMap creates a TreeNode from a map[string]interface{}.
	//
	// This is useful for deserializing tree data from JSON or other sources.
	// The map should follow the TreeNode wire format with keys:
	//   - "s": Static HTML parts ([]interface{})
	//   - "0", "1", etc.: Dynamic values at each position
	//   - "f": Fingerprint hash (string)
	//   - "d": Range data (map)
	//   - "m": Metadata (map)
	//
	// Parameters:
	//   - m: Map containing tree data in wire format
	//
	// Returns:
	//   - *TreeNode reconstructed from the map
	//   - error if the map format is invalid
	//
	// Example:
	//   data := map[string]interface{}{
	//       "s": []interface{}{"<div>", "</div>"},
	//       "0": "content",
	//   }
	//   node, err := FromMap(data)
	FromMap = build.FromMap
)
