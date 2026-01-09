package diff

// prepareAndFilterIfNeeded recursively prepares a value and returns it only if non-empty.
// This helper consolidates the common pattern of prepare + filter logic.
func prepareAndFilterIfNeeded(value interface{}, clientHasStatics bool) (interface{}, bool) {
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
func PrepareTreeForClient(node interface{}, clientHasStatics bool) interface{} {
	if !clientHasStatics {
		// Client doesn't have statics - send everything as-is
		return node
	}

	// Client has statics - remove them to reduce wire size
	switch v := node.(type) {
	case *TreeNode:
		// Create new TreeNode without statics or fingerprint
		result := &TreeNode{
			Dynamics: make(map[string]interface{}, len(v.Dynamics)),
		}
		// Recursively prepare dynamics
		for k, val := range v.Dynamics {
			prepared := PrepareTreeForClient(val, clientHasStatics)
			if !IsEmpty(prepared) {
				result.Dynamics[k] = prepared
			} else if nestedNode, ok := val.(*TreeNode); ok && nestedNode.HasStatics() {
				// For conditional blocks with statics but no dynamics, we must preserve
				// the statics because the client needs them to render the content.
				// This happens for {{if}} blocks where the branch content is pure static HTML.
				result.Dynamics[k] = val // Keep original with statics
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
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint (client has them cached)
			}
			prepared := PrepareTreeForClient(val, clientHasStatics)
			if !IsEmpty(prepared) {
				result[k] = prepared
			} else if nestedNode, ok := val.(*TreeNode); ok && nestedNode.HasStatics() {
				// For conditional blocks with statics but no dynamics, preserve the statics
				result[k] = val
			}
		}
		return result
	case []interface{}:
		// Don't pre-allocate since we're filtering - build slice dynamically
		var result []interface{}
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
