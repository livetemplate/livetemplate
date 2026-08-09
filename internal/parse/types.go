package parse

import (
	"github.com/livetemplate/livetemplate/internal/build"
)

// Re-export build types for convenience within parse package
type (
	TreeNode     = build.TreeNode
	RangeData    = build.RangeData
	TreeMetadata = build.TreeMetadata
	Context      = build.Context
	WrapperKind  = build.WrapperKind
)

// Re-export the wrapper kinds so parse can tag nodes without importing build
// at every wrapper site.
const (
	WrapperNone        = build.WrapperNone
	WrapperConditional = build.WrapperConditional
	WrapperInvocation  = build.WrapperInvocation
)

// Re-export build functions for convenience within parse package
var (
	NewTreeNode            = build.NewTreeNode
	NewTreeNodeWithStatics = build.NewTreeNodeWithStatics
	NewRangeData           = build.NewRangeData
	NewTreeMetadata        = build.NewTreeMetadata
)
