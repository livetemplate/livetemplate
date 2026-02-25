package mutations

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
)

// Apply applies a mutation to state and returns the new state.
// The original state is not modified; a deep copy is made first.
func Apply(state map[string]any, m Mutation) (map[string]any, error) {
	result := DeepCopy(state)

	switch m.Type {
	case MutSetField:
		return setAtPath(result, m.Target, m.Value)

	case MutToggleBool:
		current, err := getAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("toggle_bool: %w", err)
		}
		b, ok := current.(bool)
		if !ok {
			return nil, fmt.Errorf("toggle_bool: %s is not bool, got %T", m.Target, current)
		}
		return setAtPath(result, m.Target, !b)

	case MutIncrementInt:
		current, err := getAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("increment_int: %w", err)
		}
		switch v := current.(type) {
		case int:
			return setAtPath(result, m.Target, v+1)
		case float64:
			return setAtPath(result, m.Target, v+1)
		default:
			return nil, fmt.Errorf("increment_int: %s is not numeric, got %T", m.Target, current)
		}

	case MutSetNil:
		return setAtPath(result, m.Target, nil)

	case MutSetEmpty:
		current, err := getAtPath(result, m.Target)
		if err != nil {
			// Field doesn't exist, set to empty string
			return setAtPath(result, m.Target, "")
		}
		switch current.(type) {
		case string:
			return setAtPath(result, m.Target, "")
		case []any:
			return setAtPath(result, m.Target, []any{})
		case map[string]any:
			return setAtPath(result, m.Target, map[string]any{})
		case int, float64:
			return setAtPath(result, m.Target, 0)
		case bool:
			return setAtPath(result, m.Target, false)
		default:
			return setAtPath(result, m.Target, nil)
		}

	case MutAppendSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("append_slice: %w", err)
		}
		item := ensureUniqueID(m.Value)
		return setAtPath(result, m.Target, append(slice, item))

	case MutPrependSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("prepend_slice: %w", err)
		}
		item := ensureUniqueID(m.Value)
		return setAtPath(result, m.Target, append([]any{item}, slice...))

	case MutInsertSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("insert_slice: %w", err)
		}
		if m.Index < 0 || m.Index > len(slice) {
			return nil, fmt.Errorf("insert_slice: index %d out of bounds (len=%d)", m.Index, len(slice))
		}
		item := ensureUniqueID(m.Value)
		newSlice := make([]any, 0, len(slice)+1)
		newSlice = append(newSlice, slice[:m.Index]...)
		newSlice = append(newSlice, item)
		newSlice = append(newSlice, slice[m.Index:]...)
		return setAtPath(result, m.Target, newSlice)

	case MutRemoveSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("remove_slice: %w", err)
		}
		if len(slice) == 0 {
			return result, nil // Nothing to remove
		}
		idx := max(
			// Wrap around for safety
			m.Index%len(slice), 0)
		newSlice := append(slice[:idx], slice[idx+1:]...)
		return setAtPath(result, m.Target, newSlice)

	case MutClearSlice:
		return setAtPath(result, m.Target, []any{})

	case MutReorderSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("reorder_slice: %w", err)
		}
		perm, ok := m.Value.([]int)
		if !ok {
			return nil, fmt.Errorf("reorder_slice: permutation must be []int, got %T", m.Value)
		}
		reordered := applyPermutation(slice, perm)
		return setAtPath(result, m.Target, reordered)

	case MutReverseSlice:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("reverse_slice: %w", err)
		}
		reversed := make([]any, len(slice))
		for i, v := range slice {
			reversed[len(slice)-1-i] = v
		}
		return setAtPath(result, m.Target, reversed)

	case MutDuplicateItem:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("duplicate_item: %w", err)
		}
		if len(slice) == 0 {
			return result, nil
		}
		idx := m.Index % len(slice)
		item := DeepCopyValue(slice[idx])
		// Give duplicate a new ID
		item = ensureUniqueID(item)
		newSlice := make([]any, 0, len(slice)+1)
		newSlice = append(newSlice, slice[:idx+1]...)
		newSlice = append(newSlice, item)
		newSlice = append(newSlice, slice[idx+1:]...)
		return setAtPath(result, m.Target, newSlice)

	case MutSwapItems:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("swap_items: %w", err)
		}
		if len(slice) < 2 {
			return result, nil
		}
		i := m.Index % len(slice)
		j := m.Index2 % len(slice)
		if i < 0 {
			i = 0
		}
		if j < 0 {
			j = 0
		}
		slice[i], slice[j] = slice[j], slice[i]
		return setAtPath(result, m.Target, slice)

	case MutUpdateItem:
		updates, ok := m.Value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("update_item: value must be map[string]any, got %T", m.Value)
		}
		itemPath := fmt.Sprintf("%s.%d", m.Target, m.Index)
		for k, v := range updates {
			var err error
			result, err = setAtPath(result, itemPath+"."+k, v)
			if err != nil {
				return nil, fmt.Errorf("update_item: %w", err)
			}
		}
		return result, nil

	case MutReplaceItem:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("replace_item: %w", err)
		}
		if len(slice) == 0 {
			return result, nil
		}
		idx := m.Index % len(slice)
		slice[idx] = ensureUniqueID(m.Value)
		return setAtPath(result, m.Target, slice)

	case MutUnicodeString, MutLargeString, MutSpecialChars, MutEmptyString:
		str, ok := m.Value.(string)
		if !ok {
			return nil, fmt.Errorf("%s: value must be string, got %T", m.Type, m.Value)
		}
		return setAtPath(result, m.Target, str)

	case MutZeroInt:
		return setAtPath(result, m.Target, 0)

	case MutNegativeInt:
		n, ok := m.Value.(int)
		if !ok {
			n = -1
		}
		if n >= 0 {
			n = -n - 1
		}
		return setAtPath(result, m.Target, n)

	case MutTypeFlip:
		current, err := getAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("type_flip: %w", err)
		}
		flipped := flipType(current)
		return setAtPath(result, m.Target, flipped)

	case MutKeyCollision:
		slice, err := getSliceAtPath(result, m.Target)
		if err != nil {
			return nil, fmt.Errorf("key_collision: %w", err)
		}
		if len(slice) < 2 {
			return result, nil
		}
		// Copy ID from first item to second item
		first, ok := slice[0].(map[string]any)
		if !ok {
			return result, nil
		}
		second, ok := slice[1].(map[string]any)
		if !ok {
			return result, nil
		}
		if id, exists := first["ID"]; exists {
			second["ID"] = id
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown mutation type: %s", m.Type)
	}
}

// DeepCopy creates a deep copy of a map using JSON serialization.
func DeepCopy(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		// Fallback to shallow copy
		result := make(map[string]any, len(m))
		maps.Copy(result, m)
		return result
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return m
	}
	return result
}

// DeepCopyValue creates a deep copy of any value using JSON serialization.
func DeepCopyValue(v any) any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return v
	}
	return result
}

// getAtPath gets a value at a dot-separated path (e.g., "User.Name", "Items.0.Text").
func getAtPath(obj map[string]any, path string) (any, error) {
	if path == "" {
		return obj, nil
	}

	parts := strings.Split(path, ".")
	var current any = obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, exists := v[part]
			if !exists {
				return nil, fmt.Errorf("path %q: key %q not found", path, part)
			}
			current = val

		case []any:
			idx, err := parseIndex(part)
			if err != nil {
				return nil, fmt.Errorf("path %q: %w", path, err)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("path %q: index %d out of bounds (len=%d)", path, idx, len(v))
			}
			current = v[idx]

		default:
			return nil, fmt.Errorf("path %q: cannot traverse %T", path, current)
		}
	}

	return current, nil
}

// setAtPath sets a value at a dot-separated path, creating intermediate maps as needed.
func setAtPath(obj map[string]any, path string, value any) (map[string]any, error) {
	if path == "" {
		return obj, nil
	}

	parts := strings.Split(path, ".")
	current := obj

	// Navigate to parent of target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		nextPart := parts[i+1]

		// Check if next part is an array index
		_, isNextIdx := parseIndexSafe(nextPart)

		switch v := current[part].(type) {
		case map[string]any:
			current = v
		case []any:
			idx, err := parseIndex(parts[i+1])
			if err != nil {
				return nil, fmt.Errorf("path %q: %w", path, err)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("path %q: index %d out of bounds", path, idx)
			}
			if m, ok := v[idx].(map[string]any); ok {
				current = m
				i++ // Skip the index part
			} else {
				return nil, fmt.Errorf("path %q: element at index %d is not a map", path, idx)
			}
		case nil:
			// Create intermediate structure
			if isNextIdx {
				current[part] = []any{}
			} else {
				newMap := make(map[string]any)
				current[part] = newMap
				current = newMap
			}
		default:
			return nil, fmt.Errorf("path %q: cannot set through %T", path, v)
		}
	}

	// Set the final value
	lastPart := parts[len(parts)-1]
	current[lastPart] = value

	return obj, nil
}

// getSliceAtPath gets a slice at the given path.
func getSliceAtPath(obj map[string]any, path string) ([]any, error) {
	val, err := getAtPath(obj, path)
	if err != nil {
		return nil, err
	}
	slice, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("path %q: expected slice, got %T", path, val)
	}
	return slice, nil
}

// parseIndex parses a string as an array index.
func parseIndex(s string) (int, error) {
	var idx int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid index: %q", s)
		}
		idx = idx*10 + int(c-'0')
	}
	return idx, nil
}

// parseIndexSafe parses a string as an array index, returning -1 if not a valid index.
func parseIndexSafe(s string) (int, bool) {
	idx, err := parseIndex(s)
	return idx, err == nil
}

// applyPermutation reorders a slice according to a permutation.
// perm[i] indicates the new position of element i.
func applyPermutation(slice []any, perm []int) []any {
	if len(perm) == 0 || len(slice) == 0 {
		return slice
	}

	// Build index mapping: perm[i] = j means "element currently at i goes to position j"
	// But we want the inverse: result[j] = slice[i] where perm[i] = j
	result := make([]any, len(slice))
	used := make([]bool, len(slice))

	for i, newPos := range perm {
		if i >= len(slice) {
			break
		}
		if newPos < 0 || newPos >= len(slice) {
			continue
		}
		if used[newPos] {
			continue // Skip duplicates in permutation
		}
		result[newPos] = slice[i]
		used[newPos] = true
	}

	// Fill any unused positions with remaining elements
	j := 0
	for i := range result {
		if !used[i] {
			for j < len(slice) && used[j] {
				j++
			}
			if j < len(slice) {
				result[i] = slice[j]
				j++
			}
		}
	}

	return result
}

// ensureUniqueID ensures an item has a unique ID field.
// If the item is a map and has an ID field, appends a random suffix.
func ensureUniqueID(item any) any {
	m, ok := item.(map[string]any)
	if !ok {
		return item
	}

	result := DeepCopyValue(m).(map[string]any)

	if id, exists := result["ID"]; exists {
		if idStr, ok := id.(string); ok {
			result["ID"] = idStr + fmt.Sprintf("_%d", uniqueCounter)
			uniqueCounter++
		}
	}

	return result
}

var uniqueCounter int

// flipType changes a value's type while preserving some meaning.
func flipType(v any) any {
	switch val := v.(type) {
	case string:
		// String → try to parse as int, else return length
		if len(val) > 0 {
			return len(val)
		}
		return 0
	case int:
		// Int → string representation
		return fmt.Sprintf("%d", val)
	case float64:
		// Float → int (truncate)
		return int(val)
	case bool:
		// Bool → int (0/1)
		if val {
			return 1
		}
		return 0
	case nil:
		return ""
	default:
		return nil
	}
}
