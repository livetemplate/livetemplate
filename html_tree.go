package livetemplate


// HTMLTreeNode represents a complete HTML tree structure with static and dynamic parts
type HTMLTreeNode struct {
	Type       string                 `json:"type,omitempty"`     // "element", "text", "dynamic"
	Tag        string                 `json:"tag,omitempty"`      // HTML tag name (for elements)
	Attrs      map[string]interface{} `json:"attrs,omitempty"`    // Attributes (can be static or dynamic)
	Children   []interface{}          `json:"children,omitempty"` // Child nodes (HTMLTreeNode or string)
	Content    string                 `json:"content,omitempty"`  // Text content (for text nodes)
	Dynamic    bool                   `json:"dynamic,omitempty"`  // Whether this node is dynamic
	Expression string                 `json:"expr,omitempty"`     // Template expression (for dynamic nodes)
	Value      interface{}            `json:"value,omitempty"`    // Evaluated value (for dynamic nodes)
}







// convertFullTreeToSegmentTree converts a full HTML tree to the segment-based format
// This maintains backward compatibility with the current client expectations





