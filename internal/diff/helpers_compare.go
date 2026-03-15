package diff

import "reflect"

// DeepEqual compares two values deeply.
// For TreeNode pointers, it uses TreeNodeEqual to ignore internal cache fields.
func DeepEqual(a, b interface{}) bool {
	if aNode, ok := a.(*TreeNode); ok {
		if bNode, ok := b.(*TreeNode); ok {
			return TreeNodeEqual(aNode, bNode)
		}
		return false
	}
	return reflect.DeepEqual(a, b)
}

// TreeNodeEqual compares two TreeNodes for equality, ignoring internal cache fields.
func TreeNodeEqual(a, b *TreeNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if !reflect.DeepEqual(a.Statics, b.Statics) {
		return false
	}

	if len(a.Dynamics) != len(b.Dynamics) {
		return false
	}

	for key, aVal := range a.Dynamics {
		bVal, exists := b.Dynamics[key]
		if !exists {
			return false
		}
		if !DeepEqual(aVal, bVal) {
			return false
		}
	}

	if a.Fingerprint != b.Fingerprint {
		return false
	}

	if (a.Range == nil) != (b.Range == nil) {
		return false
	}
	if a.Range != nil {
		if !reflect.DeepEqual(a.Range.Items, b.Range.Items) {
			return false
		}
		if !reflect.DeepEqual(a.Range.Statics, b.Range.Statics) {
			return false
		}
	}

	if (a.Metadata == nil) != (b.Metadata == nil) {
		return false
	}
	if a.Metadata != nil && a.Metadata.IDKey != b.Metadata.IDKey {
		return false
	}

	return true
}
