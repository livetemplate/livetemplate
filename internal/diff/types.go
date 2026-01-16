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
//
// Deprecated: This registry-based approach is being replaced by fingerprint comparison.
// The diff logic now uses ClientNeedsStatics() which compares structure fingerprints
// to determine if the client needs statics, eliminating the need for path-based tracking.
//
// This interface is kept for backward compatibility but its methods are no longer called
// for statics decisions. It may be removed in a future version.
//
// Migration: Instead of implementing StructureRegistry, use the fingerprint-based approach:
//   - ClientNeedsStatics(oldTree, newTree) returns true if client needs statics
//   - CalculateStructureFingerprint(tree) returns a fingerprint for structure comparison
type StructureRegistry interface {
	// HasSeen returns true if the client has seen the given structure at the specified path.
	HasSeen(path string, value interface{}) bool

	// MarkSeen records that the client has now seen the given structure at the specified path.
	MarkSeen(path string, value interface{})

	// InvalidatePath removes all registry entries for the given path and its children.
	// This should be called when a TreeNode is replaced with a primitive value.
	InvalidatePath(path string)
}
