package parse

import "html/template"

// TreeNode represents a node in the tree structure with statics and dynamics.
// This is a placeholder that will be replaced with internal/build types.
type TreeNode struct {
	Statics  []string
	Dynamics map[string]interface{}
	Range    *RangeData
	Metadata *TreeMetadata
}

// NewTreeNode creates an empty tree node.
func NewTreeNode() *TreeNode {
	return &TreeNode{
		Dynamics: make(map[string]interface{}),
	}
}

// NewTreeNodeWithStatics creates a tree node with statics.
func NewTreeNodeWithStatics(statics []string) *TreeNode {
	return &TreeNode{
		Statics:  statics,
		Dynamics: make(map[string]interface{}),
	}
}

// SetDynamic sets a dynamic value in the tree.
func (t *TreeNode) SetDynamic(key string, value interface{}) {
	t.Dynamics[key] = value
}

// HasRange checks if this node represents a range comprehension.
func (t *TreeNode) HasRange() bool {
	return t.Range != nil
}

// RangeData holds range iteration data.
type RangeData struct {
	Items   []interface{}
	Statics []string
}

// NewRangeData creates range data.
func NewRangeData(items []interface{}, statics []string) *RangeData {
	return &RangeData{
		Items:   items,
		Statics: statics,
	}
}

// TreeMetadata holds metadata about tree structure.
type TreeMetadata struct {
	IDKey string
}

// NewTreeMetadata creates tree metadata.
func NewTreeMetadata(idKey string) *TreeMetadata {
	return &TreeMetadata{
		IDKey: idKey,
	}
}

// KeyGenerator generates unique keys for range items.
type KeyGenerator interface {
	Next() string
}

// Context holds parsing context including function map and options.
type Context struct {
	FuncMap        template.FuncMap
	IncludeStatics bool
}

// ShouldIncludeStatics returns whether statics should be included in output.
func (c *Context) ShouldIncludeStatics() bool {
	// Default to true if not explicitly set
	return c == nil || c.IncludeStatics
}
