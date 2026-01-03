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

// StructureRegistry defines the interface for tracking client-side structure visibility.
// It tracks which structures have been sent to the client to optimize subsequent updates.
type StructureRegistry interface {
	// HasSeen returns true if the client has seen the given structure at the specified path.
	HasSeen(path string, value interface{}) bool

	// MarkSeen records that the client has now seen the given structure at the specified path.
	MarkSeen(path string, value interface{})

	// InvalidatePath removes all registry entries for the given path and its children.
	// This should be called when a TreeNode is replaced with a primitive value.
	InvalidatePath(path string)
}
