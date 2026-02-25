package diff

// prepareAndFilterIfNeeded recursively prepares a value and returns it only if non-empty.
// This helper consolidates the common pattern of prepare + filter logic.
func prepareAndFilterIfNeeded(value any, clientHasStatics bool) (any, bool) {
	prepared := PrepareTreeForClient(value, clientHasStatics)
	if IsEmpty(prepared) {
		return nil, false
	}
	return prepared, true
}

// PrepareTreeForClient prepares a tree for wire transmission to the client.
// This implements a critical optimization from the tree-update-specification:
// - If clientHasStatics is false (first render): everything is sent including statics
// - If clientHasStatics is true (updates): statics are stripped to reduce wire size (~90% reduction)
// Empty values are filtered out to minimize payload size.
func PrepareTreeForClient(node any, clientHasStatics bool) any {
	if !clientHasStatics {
		// Client doesn't have statics - send everything as-is
		return node
	}

	// Client has statics - remove them to reduce wire size
	switch v := node.(type) {
	case *TreeNode:
		// Create new TreeNode without statics or fingerprint
		result := &TreeNode{
			Dynamics: make(map[string]any, len(v.Dynamics)),
		}
		// Recursively prepare dynamics, filtering out empty values while preserving static-only conditional blocks
		for k, val := range v.Dynamics {
			prepared := PrepareTreeForClient(val, clientHasStatics)
			if !IsEmpty(prepared) {
				result.Dynamics[k] = prepared
			} else if nestedNode, ok := val.(*TreeNode); ok && nestedNode.HasStatics() {
				// Special case for conditional blocks ({{if}}/{{else}}) with static-only content.
				// Even though clientHasStatics=true means we normally strip statics, conditional
				// branches are dynamically-rendered structures. When a new item is prepended,
				// the client hasn't seen THIS particular branch's statics yet - only the template's
				// base statics are cached. We must send the full TreeNode so the client can render
				// the branch content (e.g., <span>High</span> inside {{if eq .Priority "high"}}).
				result.Dynamics[k] = val
			}
		}
		// Handle Range: preserve Items array without statics (client has them cached)
		if v.HasRange() {
			result.Range = &RangeData{Items: v.Range.Items}
		}
		// Preserve Metadata (needed for client to extract item keys)
		if v.Metadata != nil {
			result.Metadata = v.Metadata
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint (client has them cached)
			}
			prepared := PrepareTreeForClient(val, clientHasStatics)
			if !IsEmpty(prepared) {
				result[k] = prepared
			} else if nestedNode, ok := val.(*TreeNode); ok && nestedNode.HasStatics() {
				// Mirror the TreeNode case above: if this entry represents a conditional
				// block whose branch content is pure static HTML (no dynamics), then
				// PrepareTreeForClient returns an "empty" value and we would normally
				// drop it. However, the client still needs the static content in order
				// to render that branch, so we keep the original value with its statics.
				result[k] = val
			}
		}
		return result
	case []any:
		// Don't pre-allocate since we're filtering - build slice dynamically
		var result []any
		for _, item := range v {
			if prepared, ok := prepareAndFilterIfNeeded(item, clientHasStatics); ok {
				result = append(result, prepared)
			}
		}
		return result
	default:
		return v
	}
}
