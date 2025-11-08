// Package context provides template execution context utilities for the LiveTemplate library.
// It handles the "lvt" namespace in templates, providing access to validation errors
// and development mode flags.
package context

import (
	"bytes"
	"html/template"
	"reflect"
	"strings"
)

// TemplateContext provides utility functions for templates via the lvt namespace.
//
// Thread-safety: TemplateContext is safe for concurrent reads but not for concurrent writes.
// If you need to share a TemplateContext across goroutines that modify it, external
// synchronization is required. In typical usage, each template execution creates a new
// TemplateContext, so concurrent access is not an issue.
type TemplateContext struct {
	errors  map[string]string
	DevMode bool // Development mode - use local client library instead of CDN
}

// NewTemplateContext creates a new TemplateContext with the given errors and devMode flag.
//
// The errors map is stored by reference, not copied. Callers should not modify the errors map
// after passing it to NewTemplateContext. If you need to modify errors after construction,
// pass a copy of the map or use AllErrors() to get a defensive copy.
func NewTemplateContext(errors map[string]string, devMode bool) *TemplateContext {
	return &TemplateContext{
		errors:  errors,
		DevMode: devMode,
	}
}

// Error returns the error message for a field.
// Returns empty string if the field has no error or if errors map is nil.
func (t *TemplateContext) Error(field string) string {
	return t.errors[field]
}

// HasError checks if a field has an error.
// Returns false if errors map is nil.
func (t *TemplateContext) HasError(field string) bool {
	_, exists := t.errors[field]
	return exists
}

// HasAnyError checks if any errors exist.
// Returns false if errors map is nil or empty.
func (t *TemplateContext) HasAnyError() bool {
	return len(t.errors) > 0
}

// AllErrors returns a copy of all errors (useful for debugging or displaying all).
// The returned map is a defensive copy and mutations will not affect internal state.
func (t *TemplateContext) AllErrors() map[string]string {
	result := make(map[string]string)
	if t.errors == nil {
		return result
	}
	for k, v := range t.errors {
		result[k] = v
	}
	return result
}

const (
	// TemplateContextKey is the key used to access lvt context in templates
	TemplateContextKey = "lvt"
)

// ExecuteTemplateWithContext adds lvt context to template execution by augmenting the data.
//
// This function handles three types of input data:
//   - Structs: Fields are copied to a map, using json tags if present
//   - Maps: Keys are copied to template data
//   - Primitives: Passed directly to the template as-is
//
// The lvt context is always available in templates via {{.lvt}}.
func ExecuteTemplateWithContext(tmpl *template.Template, data interface{}, errors map[string]string, devMode bool) ([]byte, error) {
	lvtContext := NewTemplateContext(errors, devMode)

	var templateData interface{}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		dataMap := make(map[string]interface{})
		dataMap[TemplateContextKey] = lvtContext

		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}

			fieldValue := val.Field(i).Interface()
			jsonTag := field.Tag.Get("json")

			if jsonTag == "-" {
				continue
			}

			if jsonTag != "" {
				if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
					jsonTag = jsonTag[:commaIdx]
				}
				dataMap[jsonTag] = fieldValue
			}

			dataMap[field.Name] = fieldValue
		}
		templateData = dataMap

	case reflect.Map:
		dataMap := make(map[string]interface{})
		dataMap[TemplateContextKey] = lvtContext

		for _, key := range val.MapKeys() {
			dataMap[key.String()] = val.MapIndex(key).Interface()
		}
		templateData = dataMap

	default:
		// For primitive types (string, int, bool, etc.), we can't inject lvt
		// Pass the data as-is to maintain compatibility with standard templates
		templateData = data
	}

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, templateData)
	return buf.Bytes(), err
}
