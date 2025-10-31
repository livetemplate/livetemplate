package livetemplate

import "github.com/livefir/livetemplate/internal/build"

// Re-export build types for backward compatibility
// These types are now in internal/build but remain accessible via the main package

type (
	// TreeNode represents a node in the template tree structure.
	TreeNode = build.TreeNode

	// RangeData represents data for range operations in templates.
	RangeData = build.RangeData

	// TreeMetadata contains metadata about the tree node.
	TreeMetadata = build.TreeMetadata

	// TreeGenerationContext provides context for tree generation.
	// Deprecated: Use build.Context instead. This alias maintains backward compatibility.
	TreeGenerationContext = build.Context
)

// Re-export build functions for backward compatibility

var (
	// NewTreeNode creates a new TreeNode with initialized maps.
	NewTreeNode = build.NewTreeNode

	// NewTreeNodeWithStatics creates a new TreeNode with the given static parts.
	NewTreeNodeWithStatics = build.NewTreeNodeWithStatics

	// NewRangeData creates a new RangeData with the given items and statics.
	NewRangeData = build.NewRangeData

	// NewTreeMetadata creates a new TreeMetadata with the given ID key.
	NewTreeMetadata = build.NewTreeMetadata

	// NewTreeGenerationContext creates a context for first render.
	// Deprecated: Use build.NewContext instead. This alias maintains backward compatibility.
	NewTreeGenerationContext = build.NewContext

	// NewUpdateContext creates a context for updates.
	NewUpdateContext = build.NewUpdateContext

	// FromMap creates a TreeNode from a map[string]interface{}.
	FromMap = build.FromMap
)
