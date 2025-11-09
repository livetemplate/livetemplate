package context

import (
	"log/slog"
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

			// Warn if field will shadow the "lvt" namespace
			if fieldName == "lvt" || field.Name == "lvt" {
				slog.Warn("struct field shadows LiveTemplate 'lvt' namespace and will be overwritten",
					slog.String("field", field.Name),
					slog.String("type", typ.Name()))
			}

			// Store field under both json tag name (if present) and original struct field name.
			// This dual mapping ensures templates work with both naming conventions:
			// - {{.fieldName}} using json tag (e.g., "user_id")
			// - {{.FieldName}} using struct field name (e.g., "UserID")
			// This maintains backward compatibility with existing templates.
			templateData[fieldName] = val.Field(i).Interface()
			templateData[field.Name] = val.Field(i).Interface()
		}
	} else if val.Kind() == reflect.Map {
		for _, key := range val.MapKeys() {
			keyStr := key.String()
			// Warn if map key will shadow the "lvt" namespace
			if keyStr == "lvt" {
				slog.Warn("map key shadows LiveTemplate 'lvt' namespace and will be overwritten",
					slog.String("key", keyStr))
			}
			templateData[keyStr] = val.MapIndex(key).Interface()
		}
	}

	return templateData
}
