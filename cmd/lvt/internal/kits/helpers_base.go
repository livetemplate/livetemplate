package kits

// BaseHelpers provides common utility methods for all CSS helpers
type BaseHelpers struct{}

// Dict creates a map for passing multiple values to nested templates
func (h *BaseHelpers) Dict(values ...interface{}) map[string]interface{} {
	if len(values)%2 != 0 {
		return nil // Invalid arguments
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil // Keys must be strings
		}
		m[key] = values[i+1]
	}
	return m
}

// Until generates a slice of integers from 0 to count-1
func (h *BaseHelpers) Until(count int) []int {
	result := make([]int, count)
	for i := 0; i < count; i++ {
		result[i] = i
	}
	return result
}

// Add adds two integers
func (h *BaseHelpers) Add(a, b int) int {
	return a + b
}
