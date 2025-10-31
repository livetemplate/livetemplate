// Package util provides generic utility functions for internal use.
package util

// Map applies a function to each element of a slice and returns a new slice.
// This is a generic helper to avoid repetitive transformation code.
//
// Example:
//
//	numbers := []int{1, 2, 3}
//	strings := util.Map(numbers, func(n int) string { return fmt.Sprintf("%d", n) })
//	// strings == []string{"1", "2", "3"}
func Map[T, U any](slice []T, fn func(T) U) []U {
	if slice == nil {
		return nil
	}

	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// Filter returns a new slice containing only elements that match the predicate.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	evens := util.Filter(numbers, func(n int) bool { return n%2 == 0 })
//	// evens == []int{2, 4}
func Filter[T any](slice []T, fn func(T) bool) []T {
	if slice == nil {
		return nil
	}

	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Keys returns all keys from a map as a slice.
// Order is not guaranteed (maps are unordered in Go).
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	keys := util.Keys(m)
//	// keys == []string{"a", "b"} or []string{"b", "a"}
func Keys[K comparable, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}

	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values from a map as a slice.
// Order is not guaranteed (maps are unordered in Go).
//
// Example:
//
//	m := map[string]int{"a": 1, "b": 2}
//	values := util.Values(m)
//	// values == []int{1, 2} or []int{2, 1}
func Values[K comparable, V any](m map[K]V) []V {
	if m == nil {
		return nil
	}

	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// FindIndex returns the index of the first element matching the predicate.
// Returns -1 if no element matches.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4}
//	index := util.FindIndex(numbers, func(n int) bool { return n > 2 })
//	// index == 2 (element 3 is at index 2)
func FindIndex[T any](slice []T, fn func(T) bool) int {
	for i, v := range slice {
		if fn(v) {
			return i
		}
	}
	return -1
}

// Contains checks if a slice contains an element.
//
// Example:
//
//	numbers := []int{1, 2, 3}
//	found := util.Contains(numbers, 2) // true
//	notFound := util.Contains(numbers, 5) // false
func Contains[T comparable](slice []T, element T) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}
