// Package build handles tree building from parsed templates and data.
// It contains the core tree data structures and tree construction logic.
package build

import (
	"encoding/json"
	"fmt"
	"html/template"
)

// TreeNode represents a node in the template tree structure with type safety.
// It replaces the old map[string]interface{} representation while maintaining
// wire format compatibility through custom JSON marshaling.
type TreeNode struct {
	// Statics are the static HTML parts of the template (key: "s")
	Statics []string

	// Dynamics maps position indices to dynamic content (keys: "0", "1", "2", etc.)
	// Values can be: string, TreeNode, RangeData, or any JSON-serializable type
	Dynamics map[string]interface{}

	// Fingerprint is the hash of the tree structure for change detection (key: "f")
	Fingerprint string

	// Range contains range operation data if this node represents a range (key: "d")
	Range *RangeData

	// Metadata contains additional information like ID key mappings (key: "m")
	Metadata *TreeMetadata
}

// RangeData represents data for range operations in templates.
// It contains the items being iterated and the static HTML template parts.
type RangeData struct {
	// Items is the list of range operations
	// Can contain: update, remove, append, prepend, insert, reorder operations
	Items []interface{}

	// Statics are the static HTML parts for rendering range items
	Statics []string
}

// TreeMetadata contains metadata about the tree node.
type TreeMetadata struct {
	// IDKey is the field name used as the unique identifier for range items
	IDKey string
}

// Context provides context for tree generation to determine
// whether static content should be included in the generated tree.
// This enables context-aware generation instead of reactive stripping.
type Context struct {
	// IsFirstRender indicates this is the initial render where all statics must be included
	IsFirstRender bool

	// IncludeStatics controls whether static HTML parts are built into the tree.
	// First render: always true
	// Updates: false for structures client already has, true for new structures
	IncludeStatics bool

	// ClientStructures maps field paths to whether the client has seen them.
	// Used to determine if statics should be included for a specific path.
	ClientStructures map[string]bool

	// CurrentPath tracks the current field path during recursive tree building.
	// Format: "0" or "1.2" for nested structures
	CurrentPath string

	// FuncMap provides the template functions available during tree generation.
	FuncMap template.FuncMap

	// DevMode indicates whether development mode is enabled.
	// In DevMode, panics are not caught to aid debugging.
	DevMode bool

	// TemplateName is the name of the template being built.
	// Used for expression caching to avoid redundant template executions.
	TemplateName string
}

// NewContext creates a context for first render (includes all statics).
func NewContext() *Context {
	return &Context{
		IsFirstRender:    true,
		IncludeStatics:   true,
		ClientStructures: make(map[string]bool),
		CurrentPath:      "",
	}
}

// NewUpdateContext creates a context for updates (excludes statics by default).
func NewUpdateContext(clientStructures map[string]bool) *Context {
	if clientStructures == nil {
		clientStructures = make(map[string]bool)
	}
	return &Context{
		IsFirstRender:    false,
		IncludeStatics:   false,
		ClientStructures: clientStructures,
		CurrentPath:      "",
	}
}

// ShouldIncludeStatics determines if statics should be included for current path.
func (ctx *Context) ShouldIncludeStatics() bool {
	if ctx == nil {
		// Backward compatibility: default to including statics
		return true
	}

	// First render always includes statics
	if ctx.IsFirstRender {
		return true
	}

	// For updates, include statics only if client doesn't have this structure
	if ctx.CurrentPath != "" {
		return !ctx.ClientStructures[ctx.CurrentPath]
	}

	return ctx.IncludeStatics
}

// WithPath returns a new context with updated CurrentPath.
// Used for tracking path during recursive tree building.
func (ctx *Context) WithPath(path string) *Context {
	if ctx == nil {
		return &Context{
			IsFirstRender:  true,
			IncludeStatics: true,
			CurrentPath:    path,
		}
	}

	newCtx := *ctx
	if ctx.CurrentPath == "" {
		newCtx.CurrentPath = path
	} else {
		newCtx.CurrentPath = ctx.CurrentPath + "." + path
	}
	return &newCtx
}

// NewTreeNode creates a new TreeNode with initialized maps.
func NewTreeNode() *TreeNode {
	return &TreeNode{
		Dynamics: make(map[string]interface{}),
	}
}

// NewTreeNodeWithStatics creates a new TreeNode with the given static parts.
func NewTreeNodeWithStatics(statics []string) *TreeNode {
	return &TreeNode{
		Statics:  statics,
		Dynamics: make(map[string]interface{}),
	}
}

// NewRangeData creates a new RangeData with the given items and statics.
func NewRangeData(items []interface{}, statics []string) *RangeData {
	return &RangeData{
		Items:   items,
		Statics: statics,
	}
}

// NewTreeMetadata creates a new TreeMetadata with the given ID key.
func NewTreeMetadata(idKey string) *TreeMetadata {
	return &TreeMetadata{
		IDKey: idKey,
	}
}

// SetDynamic sets a dynamic value at the given position.
func (tn *TreeNode) SetDynamic(position string, value interface{}) {
	if tn.Dynamics == nil {
		tn.Dynamics = make(map[string]interface{})
	}
	tn.Dynamics[position] = value
}

// GetDynamic retrieves a dynamic value at the given position.
func (tn *TreeNode) GetDynamic(position string) (interface{}, bool) {
	if tn.Dynamics == nil {
		return nil, false
	}
	val, ok := tn.Dynamics[position]
	return val, ok
}

// GetDynamics returns the entire Dynamics map.
// This implements the DynamicsGetter interface from internal/keys package,
// allowing TreeNode to be used with key generation utilities.
func (tn *TreeNode) GetDynamics() map[string]interface{} {
	return tn.Dynamics
}

// HasStatics returns true if the node has static parts.
func (tn *TreeNode) HasStatics() bool {
	return len(tn.Statics) > 0
}

// HasDynamics returns true if the node has dynamic content.
func (tn *TreeNode) HasDynamics() bool {
	return len(tn.Dynamics) > 0
}

// HasRange returns true if the node represents a range.
func (tn *TreeNode) HasRange() bool {
	return tn.Range != nil
}

// MarshalJSON implements custom JSON marshaling to maintain wire format compatibility.
// The TreeNode is marshaled as a map[string]interface{} with keys:
//   - "s": statics array
//   - "0", "1", "2", etc.: dynamic values at positions
//   - "f": fingerprint
//   - "d": range data
//   - "m": metadata
//
// This ensures that the typed TreeNode serializes to the same JSON format as the
// original map[string]interface{} representation.
func (tn *TreeNode) MarshalJSON() ([]byte, error) {
	result := make(map[string]interface{})

	// Add statics if present
	if len(tn.Statics) > 0 {
		result["s"] = tn.Statics
	}

	// Add dynamics
	for key, value := range tn.Dynamics {
		result[key] = value
	}

	// Add fingerprint if present
	if tn.Fingerprint != "" {
		result["f"] = tn.Fingerprint
	}

	// Add range data if present
	if tn.Range != nil {
		result["d"] = tn.Range.Items
	}

	// Add metadata if present
	if tn.Metadata != nil {
		result["m"] = map[string]interface{}{
			"idKey": tn.Metadata.IDKey,
		}
	}

	return json.Marshal(result)
}

// UnmarshalJSON implements custom JSON unmarshaling from wire format.
// This allows reading the old map[string]interface{} format into typed TreeNode.
func (tn *TreeNode) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Initialize dynamics map
	tn.Dynamics = make(map[string]interface{})

	for key, value := range raw {
		switch key {
		case "s":
			// Parse statics
			if statics, ok := value.([]interface{}); ok {
				tn.Statics = make([]string, len(statics))
				for i, s := range statics {
					if str, ok := s.(string); ok {
						tn.Statics[i] = str
					} else {
						return fmt.Errorf("invalid static at index %d: expected string, got %T", i, s)
					}
				}
			} else {
				return fmt.Errorf("invalid statics: expected array, got %T", value)
			}

		case "f":
			// Parse fingerprint
			if fp, ok := value.(string); ok {
				tn.Fingerprint = fp
			} else {
				return fmt.Errorf("invalid fingerprint: expected string, got %T", value)
			}

		case "d":
			// Parse range data
			if items, ok := value.([]interface{}); ok {
				tn.Range = &RangeData{
					Items: items,
				}
			} else {
				return fmt.Errorf("invalid range data: expected array, got %T", value)
			}

		case "m":
			// Parse metadata
			if meta, ok := value.(map[string]interface{}); ok {
				if idKey, ok := meta["idKey"].(string); ok {
					tn.Metadata = &TreeMetadata{
						IDKey: idKey,
					}
				}
			} else {
				return fmt.Errorf("invalid metadata: expected object, got %T", value)
			}

		default:
			// Numeric keys are dynamics
			tn.Dynamics[key] = value
		}
	}

	return nil
}

// ToMap converts the TreeNode back to map[string]interface{} format.
// This is useful for gradual migration and interop with existing code.
func (tn *TreeNode) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	// Add statics
	if len(tn.Statics) > 0 {
		result["s"] = tn.Statics
	}

	// Add dynamics
	for key, value := range tn.Dynamics {
		// Recursively convert nested TreeNodes
		if nestedNode, ok := value.(*TreeNode); ok {
			result[key] = nestedNode.ToMap()
		} else {
			result[key] = value
		}
	}

	// Add fingerprint
	if tn.Fingerprint != "" {
		result["f"] = tn.Fingerprint
	}

	// Add range data
	if tn.Range != nil {
		// Recursively convert any nested TreeNodes in range items
		convertedItems := make([]interface{}, len(tn.Range.Items))
		for i, item := range tn.Range.Items {
			convertedItems[i] = convertValueToMap(item)
		}
		result["d"] = convertedItems
	}

	// Add metadata
	if tn.Metadata != nil {
		result["m"] = map[string]interface{}{
			"idKey": tn.Metadata.IDKey,
		}
	}

	return result
}

// FromMap creates a TreeNode from a map[string]interface{}.
// This is useful for converting existing code to use typed TreeNode.
func FromMap(m map[string]interface{}) (*TreeNode, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var tn TreeNode
	if err := json.Unmarshal(data, &tn); err != nil {
		return nil, err
	}

	return &tn, nil
}

// Clone creates a deep copy of the TreeNode.
func (tn *TreeNode) Clone() *TreeNode {
	clone := &TreeNode{
		Fingerprint: tn.Fingerprint,
	}

	// Clone statics
	if len(tn.Statics) > 0 {
		clone.Statics = make([]string, len(tn.Statics))
		copy(clone.Statics, tn.Statics)
	}

	// Clone dynamics
	if len(tn.Dynamics) > 0 {
		clone.Dynamics = make(map[string]interface{}, len(tn.Dynamics))
		for k, v := range tn.Dynamics {
			// Deep clone nested TreeNodes
			if nestedNode, ok := v.(*TreeNode); ok {
				clone.Dynamics[k] = nestedNode.Clone()
			} else {
				clone.Dynamics[k] = v
			}
		}
	}

	// Clone range data
	if tn.Range != nil {
		clone.Range = &RangeData{
			Items: make([]interface{}, len(tn.Range.Items)),
		}
		copy(clone.Range.Items, tn.Range.Items)
		if len(tn.Range.Statics) > 0 {
			clone.Range.Statics = make([]string, len(tn.Range.Statics))
			copy(clone.Range.Statics, tn.Range.Statics)
		}
	}

	// Clone metadata
	if tn.Metadata != nil {
		clone.Metadata = &TreeMetadata{
			IDKey: tn.Metadata.IDKey,
		}
	}

	return clone
}

// convertValueToMap recursively converts any *TreeNode pointers in a value to maps.
// This is used to ensure complete conversion in ToMap() for nested structures.
func convertValueToMap(value interface{}) interface{} {
	switch v := value.(type) {
	case *TreeNode:
		// Convert TreeNode to map
		return v.ToMap()
	case map[string]interface{}:
		// Recursively convert any TreeNodes in the map
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = convertValueToMap(val)
		}
		return result
	case []interface{}:
		// Recursively convert any TreeNodes in slices
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = convertValueToMap(val)
		}
		return result
	default:
		// Return other types as-is
		return value
	}
}
