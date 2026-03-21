package diff

// IsEmpty checks if a value is considered empty (empty string, empty map, empty slice).
func IsEmpty(v interface{}) bool {
	switch val := v.(type) {
	case *TreeNode:
		return !val.HasStatics() && !val.HasDynamics() && !val.HasRange()
	case string:
		return val == ""
	case map[string]interface{}:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// isMeaningfulValue checks if a value is meaningful (not empty/nil) and should
// be reported when removed. This is used in CompareRangeItemsForChanges to
// determine if removing a field should generate a change notification.
func isMeaningfulValue(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case string:
		return val != ""
	case *TreeNode:
		return true
	case map[string]interface{}:
		return len(val) > 0
	case []interface{}:
		return len(val) > 0
	default:
		return true
	}
}

// hasDynamicsChanged checks if the set of dynamic positions differs between oldTree and newTree.
func hasDynamicsChanged(oldTree, newTree *TreeNode) bool {
	oldHasDynamics := oldTree != nil && oldTree.HasDynamics()
	newHasDynamics := newTree != nil && newTree.HasDynamics()

	if oldHasDynamics != newHasDynamics {
		return true
	}

	if !oldHasDynamics && !newHasDynamics {
		return false
	}

	if oldTree.DynamicLen() != newTree.DynamicLen() {
		return true
	}

	// Check that non-nil positions match
	maxLen := len(oldTree.Dynamics)
	if len(newTree.Dynamics) > maxLen {
		maxLen = len(newTree.Dynamics)
	}
	for i := 0; i < maxLen; i++ {
		var oldNil, newNil bool
		if i >= len(oldTree.Dynamics) {
			oldNil = true
		} else {
			oldNil = oldTree.Dynamics[i] == nil
		}
		if i >= len(newTree.Dynamics) {
			newNil = true
		} else {
			newNil = newTree.Dynamics[i] == nil
		}
		if oldNil != newNil {
			return true
		}
	}
	return false
}

// IsRangeConstruct checks if a value is a range construct (has Range and Statics).
func IsRangeConstruct(value interface{}) bool {
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && node.HasStatics()
	}
	if valueMap, ok := value.(map[string]interface{}); ok {
		_, hasD := valueMap["d"]
		_, hasS := valueMap["s"]
		return hasD && hasS
	}
	return false
}

// HasRangeItems checks if a range construct has any items in its data array.
func HasRangeItems(value interface{}) bool {
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && len(node.Range.Items) > 0
	}
	if valueMap, ok := value.(map[string]interface{}); ok {
		if d, hasD := valueMap["d"]; hasD {
			if dArray, ok := d.([]interface{}); ok {
				return len(dArray) > 0
			}
		}
	}
	return false
}

// ContainsRangeConstruct recursively checks if a tree node or any of its children contains a range construct.
func ContainsRangeConstruct(value interface{}) bool {
	if IsRangeConstruct(value) {
		return true
	}
	if node, ok := value.(*TreeNode); ok {
		for _, v := range node.Dynamics {
			if ContainsRangeConstruct(v) {
				return true
			}
		}
		return false
	}
	if valueMap, ok := value.(map[string]interface{}); ok {
		for k, v := range valueMap {
			if k == "s" || k == "f" {
				continue
			}
			if ContainsRangeConstruct(v) {
				return true
			}
		}
	}
	return false
}
