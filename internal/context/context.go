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
	errors        map[string]string
	DevMode       bool        // Development mode - use local client library instead of CDN
	uploadEntries interface{} // *upload.Registry for accessing upload state
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

// SetUploadRegistry sets the upload registry for this template context.
// This allows templates to access upload state via .lvt.Uploads(name).
func (t *TemplateContext) SetUploadRegistry(registry interface{}) {
	t.uploadEntries = registry
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

// Uploads returns upload entries for a given upload name.
// Returns nil if no upload registry is set or upload doesn't exist.
// The upload registry is expected to have a method: GetUpload(name string) interface{}
// which returns an object with GetEntries() []interface{} method.
func (t *TemplateContext) Uploads(name string) interface{} {
	if t.uploadEntries == nil {
		return nil
	}

	// Use reflection to call GetUpload method
	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return nil
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return nil
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 {
		return nil
	}

	upload := results[0].Interface()
	if upload == nil {
		return nil
	}

	// Call GetEntries on the upload object
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return nil
	}

	getEntriesMethod := uploadVal.MethodByName("GetEntries")
	if !getEntriesMethod.IsValid() {
		return nil
	}

	entriesResults := getEntriesMethod.Call(nil)
	if len(entriesResults) == 0 {
		return nil
	}

	return entriesResults[0].Interface()
}

// HasUploadError checks if any upload entry has an error for the given upload name.
func (t *TemplateContext) HasUploadError(name string) bool {
	if t.uploadEntries == nil {
		return false
	}

	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return false
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return false
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 || results[0].IsNil() {
		return false
	}

	upload := results[0].Interface()
	if upload == nil {
		return false
	}
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return false
	}

	hasErrorMethod := uploadVal.MethodByName("HasError")
	if !hasErrorMethod.IsValid() {
		return false
	}

	errorResults := hasErrorMethod.Call(nil)
	if len(errorResults) == 0 {
		return false
	}

	return errorResults[0].Bool()
}

// UploadError returns the first error message for the given upload name.
// Returns empty string if no errors exist.
func (t *TemplateContext) UploadError(name string) string {
	if t.uploadEntries == nil {
		return ""
	}

	registryVal := reflect.ValueOf(t.uploadEntries)
	if !registryVal.IsValid() || registryVal.IsNil() {
		return ""
	}

	getUploadMethod := registryVal.MethodByName("GetUpload")
	if !getUploadMethod.IsValid() {
		return ""
	}

	results := getUploadMethod.Call([]reflect.Value{reflect.ValueOf(name)})
	if len(results) == 0 || results[0].IsNil() {
		return ""
	}

	upload := results[0].Interface()
	if upload == nil {
		return ""
	}
	uploadVal := reflect.ValueOf(upload)
	if !uploadVal.IsValid() || uploadVal.IsNil() {
		return ""
	}

	getErrorMethod := uploadVal.MethodByName("GetError")
	if !getErrorMethod.IsValid() {
		return ""
	}

	errorResults := getErrorMethod.Call(nil)
	if len(errorResults) == 0 {
		return ""
	}

	return errorResults[0].String()
}

const (
	// TemplateContextKey is the key used to access lvt context in templates
	TemplateContextKey = "lvt"
)

// ExecuteTemplateWithContext adds lvt context to template execution by augmenting the data.
//
// This function handles different types of input data:
//   - Structs: Fields are copied to a map, using json tags if present
//   - Maps: Keys are copied to template data
//   - Nil pointers: Only lvt context is provided
//   - Primitives: Passed directly to the template as-is (lvt not available)
//
// The lvt context is available in templates via {{.lvt}} for structs, maps, and nil pointers.
// For primitive types (string, int, bool, etc.), lvt context is not available to maintain
// compatibility with standard Go templates.
//
// Note: If struct fields or map keys conflict with the reserved "lvt" key, they will be
// skipped to ensure the lvt context remains accessible in templates.
func ExecuteTemplateWithContext(tmpl *template.Template, data interface{}, errors map[string]string, devMode bool) ([]byte, error) {
	lvtContext := NewTemplateContext(errors, devMode)

	var templateData interface{}

	val := reflect.ValueOf(data)

	// Handle nil pointer case explicitly
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			// Provide only lvt context for nil pointers
			dataMap := make(map[string]interface{})
			dataMap[TemplateContextKey] = lvtContext
			templateData = dataMap
			var buf bytes.Buffer
			err := tmpl.Execute(&buf, templateData)
			return buf.Bytes(), err
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		dataMap := make(map[string]interface{})

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

			// Handle JSON tags properly
			if jsonTag != "" {
				// Handle ",omitempty" and similar cases
				if commaIdx := strings.Index(jsonTag, ","); commaIdx >= 0 {
					if commaIdx == 0 {
						// Tag is just ",omitempty" - use field name
						jsonTag = ""
					} else {
						jsonTag = jsonTag[:commaIdx]
					}
				}
				// Skip if tag results in reserved key
				if jsonTag != "" && jsonTag != TemplateContextKey {
					dataMap[jsonTag] = fieldValue
				}
			}

			// Skip field name if it conflicts with reserved key
			if field.Name != TemplateContextKey {
				dataMap[field.Name] = fieldValue
			}
		}

		// Add lvt context last to prevent field collision
		dataMap[TemplateContextKey] = lvtContext
		templateData = dataMap

	case reflect.Map:
		dataMap := make(map[string]interface{})

		for _, key := range val.MapKeys() {
			keyStr := key.String()
			// Skip map keys that conflict with reserved key
			if keyStr != TemplateContextKey {
				dataMap[keyStr] = val.MapIndex(key).Interface()
			}
		}

		// Add lvt context last to ensure it's always available
		dataMap[TemplateContextKey] = lvtContext
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
