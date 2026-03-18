package diff

import (
	"reflect"
	"slices"
)

// DeepEqual compares two values deeply.
// For TreeNode pointers, it uses TreeNodeEqual to ignore internal cache fields.
// Fast paths for common types avoid reflect.DeepEqual overhead.
func DeepEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case *TreeNode:
		bv, ok := b.(*TreeNode)
		return ok && TreeNodeEqual(av, bv)
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	}
	return reflect.DeepEqual(a, b)
}

// TreeNodeEqual compares two TreeNodes for equality, ignoring internal cache fields.
// Pointer identity is checked first to handle shared references and prevent
// infinite recursion on cyclic graphs.
func TreeNodeEqual(a, b *TreeNode) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if !slices.Equal(a.Statics, b.Statics) {
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
		if !rangeItemsEqual(a.Range.Items, b.Range.Items) {
			return false
		}
		if !slices.Equal(a.Range.Statics, b.Range.Statics) {
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

func rangeItemsEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i, av := range a {
		if !DeepEqual(av, b[i]) {
			return false
		}
	}
	return true
}
