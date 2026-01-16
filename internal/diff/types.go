// Package diff handles tree comparison and differential update generation.
// It provides functionality to compare tree structures and generate minimal
// update operations for efficient client-side rendering.
package diff

import (
	"github.com/livetemplate/livetemplate/internal/build"
)

// TreeNode is an alias for build.TreeNode for convenience.
type TreeNode = build.TreeNode

// RangeData is an alias for build.RangeData for convenience.
type RangeData = build.RangeData

// TreeMetadata is an alias for build.TreeMetadata for convenience.
type TreeMetadata = build.TreeMetadata
