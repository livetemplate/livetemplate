package context

import (
	"log/slog"
	"reflect"
	"strings"
)

// BuildDataMap creates a template data map with lvt context from the given data.
// This is the single source of truth for data→map conversion, used by both
// template execution and tree building to avoid duplicate reflection.
func BuildDataMap(data interface{}, messages map[string]string, devMode bool, uploadRegistry interface{}) interface{} {
	if messages == nil {
		messages = make(map[string]string)
	}

	lvtContext := NewTemplateContext(messages, devMode)
	if uploadRegistry != nil {
		lvtContext.SetUploadRegistry(uploadRegistry)
	}

	return buildDataMapWithContext(data, lvtContext)
}

// buildDataMapWithContext does the actual reflection and map construction.
func buildDataMapWithContext(data interface{}, lvtContext *TemplateContext) interface{} {
	val := reflect.ValueOf(data)

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			dataMap := make(map[string]interface{})
			dataMap[TemplateContextKey] = lvtContext
			return dataMap
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

			if jsonTag != "" {
				if commaIdx := strings.Index(jsonTag, ","); commaIdx >= 0 {
					if commaIdx == 0 {
						jsonTag = ""
					} else {
						jsonTag = jsonTag[:commaIdx]
					}
				}
				if jsonTag != "" && jsonTag != TemplateContextKey {
					dataMap[jsonTag] = fieldValue
				}
			}

			if field.Name == TemplateContextKey {
				slog.Warn("struct field shadows LiveTemplate 'lvt' namespace and will be skipped",
					slog.String("field", field.Name),
					slog.String("type", typ.Name()))
				continue
			}

			dataMap[field.Name] = fieldValue
		}

		dataMap[TemplateContextKey] = lvtContext
		return dataMap

	case reflect.Map:
		dataMap := make(map[string]interface{})
		for _, key := range val.MapKeys() {
			keyStr := key.String()
			if keyStr == TemplateContextKey {
				slog.Warn("map key shadows LiveTemplate 'lvt' namespace and will be skipped",
					slog.String("key", keyStr))
			}
			if keyStr != TemplateContextKey {
				dataMap[keyStr] = val.MapIndex(key).Interface()
			}
		}
		dataMap[TemplateContextKey] = lvtContext
		return dataMap

	default:
		return data
	}
}

// AddLvtToData converts data to include lvt context for template execution.
// This is a convenience wrapper around BuildDataMap for backward compatibility.
func AddLvtToData(data interface{}, messages map[string]string, devMode bool, uploadRegistry ...interface{}) interface{} {
	var registry interface{}
	if len(uploadRegistry) > 0 {
		registry = uploadRegistry[0]
	}
	return BuildDataMap(data, messages, devMode, registry)
}
