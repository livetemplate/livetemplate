package diff

// PrepareTreeForClient prepares a tree for transmission to the client.
// If clientHasStatics is true, statics are stripped to reduce wire size.
// If clientHasStatics is false, everything is sent as-is.
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
			Dynamics: make(map[string]interface{}),
		}
		// Recursively prepare dynamics
		for k, val := range v.Dynamics {
			prepared := PrepareTreeForClient(val, clientHasStatics)
			// Only include non-empty values
			if !IsEmpty(prepared) {
				result.Dynamics[k] = prepared
			}
		}
		// Handle Range but without statics (client has them cached)
		if v.HasRange() {
			result.Range = &RangeData{Items: v.Range.Items}
		}
		// Preserve Metadata (needed for client to extract item keys)
		if v.Metadata != nil {
			result.Metadata = v.Metadata
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint (client has them cached)
			}
			prepared := PrepareTreeForClient(val, clientHasStatics)
			// Only include non-empty values
			if !IsEmpty(prepared) {
				result[k] = prepared
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			prepared := PrepareTreeForClient(item, clientHasStatics)
			// Only include non-empty values
			if !IsEmpty(prepared) {
				result = append(result, prepared)
			}
		}
		return result
	default:
		return v
	}
}
