package context

import (
	"reflect"
	"strings"
)

// AddLvtToData converts data to include lvt context for template execution.
//
// This function:
// 1. Creates a LiveTemplate context with errors and dev mode settings
// 2. Wraps the original data in a map with "lvt" namespace
// 3. Handles both struct and map data types
// 4. For structs, respects json tags for field naming
//
// The resulting map allows templates to access both the original data fields
// and LiveTemplate-specific functionality through the "lvt" namespace.
//
// Parameters:
//   - data: The original template data (struct, map, or other type)
//   - errors: Map of field names to error messages for validation
//   - devMode: Whether to enable development mode features
//
// Returns:
//   - A map containing the original data fields plus the "lvt" namespace
func AddLvtToData(data interface{}, errors map[string]string, devMode bool) interface{} {
	if errors == nil {
		errors = make(map[string]string)
	}

	// Create LiveTemplate context
	lvtContext := NewTemplateContext(errors, devMode)

	templateData := make(map[string]interface{})
	templateData["lvt"] = lvtContext

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)

			if !field.IsExported() {
				continue
			}

			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
					fieldName = jsonTag[:commaIdx]
				} else if jsonTag != "-" {
					fieldName = jsonTag
				}
			}
			templateData[fieldName] = val.Field(i).Interface()
			templateData[field.Name] = val.Field(i).Interface()
		}
	} else if val.Kind() == reflect.Map {
		for _, key := range val.MapKeys() {
			templateData[key.String()] = val.MapIndex(key).Interface()
		}
	}

	return templateData
}
