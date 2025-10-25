package livetemplate

import (
	"encoding/json"
	"fmt"
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
		result["d"] = tn.Range.Items
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
