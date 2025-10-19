package livetemplate

import (
	"bytes"
	"html/template"
	"reflect"
	"strings"
)

// TemplateContext provides utility functions for templates via the lvt namespace
type TemplateContext struct {
	errors  map[string]string
	DevMode bool // Development mode - use local client library instead of CDN
}

// Error returns the error message for a field
func (t *TemplateContext) Error(field string) string {
	if t.errors == nil {
		return ""
	}
	return t.errors[field]
}

// HasError checks if a field has an error
func (t *TemplateContext) HasError(field string) bool {
	if t.errors == nil {
		return false
	}
	_, exists := t.errors[field]
	return exists
}

// HasAnyError checks if any errors exist
func (t *TemplateContext) HasAnyError() bool {
	return len(t.errors) > 0
}

// AllErrors returns all errors (useful for debugging or displaying all)
func (t *TemplateContext) AllErrors() map[string]string {
	if t.errors == nil {
		return make(map[string]string)
	}
	return t.errors
}

// executeTemplateWithContext adds lvt context to template execution by augmenting the data
func executeTemplateWithContext(tmpl *template.Template, data interface{}, errors map[string]string, devMode bool) ([]byte, error) {
	// Create context object
	lvtContext := &TemplateContext{
		errors:  errors,
		DevMode: devMode,
	}

	// Create a map that includes both the original data fields and lvt
	templateData := make(map[string]interface{})
	templateData["lvt"] = lvtContext

	// Use reflection to copy fields from data to the map
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Use the json tag name if available, otherwise use field name
			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				// Extract just the field name from json tag (ignore options like omitempty)
				if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
					fieldName = jsonTag[:commaIdx]
				} else if jsonTag != "-" {
					fieldName = jsonTag
				}
			}
			templateData[fieldName] = val.Field(i).Interface()
			// Also add with original field name for templates that use {{.FieldName}}
			templateData[field.Name] = val.Field(i).Interface()
		}
	} else if val.Kind() == reflect.Map {
		// If data is already a map, just add lvt to it
		for _, key := range val.MapKeys() {
			templateData[key.String()] = val.MapIndex(key).Interface()
		}
	}

	var buf bytes.Buffer
	err := tmpl.Execute(&buf, templateData)
	return buf.Bytes(), err
}
