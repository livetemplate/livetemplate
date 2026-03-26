package context

import (
	"log/slog"
	"reflect"
	"strings"
	"sync"
)

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

// methodMeta holds cached metadata about which methods on a type
// qualify for template precomputation (zero-arg, 1 or 2 returns).
type methodMeta struct {
	Name  string
	Index int
	// HasError is true when the method returns (value, error).
	HasError bool
}

// methodMetaCache caches per-type method metadata to avoid repeated
// signature scanning on every render. Follows the sync.Map pattern
// used by dispatch.go (methodCacheNewSignature) and state.go (stateFieldCache).
var methodMetaCache sync.Map // reflect.Type → []methodMeta

func getMethodMeta(ptrType reflect.Type) []methodMeta {
	if cached, ok := methodMetaCache.Load(ptrType); ok {
		return cached.([]methodMeta)
	}
	var meta []methodMeta
	for i := 0; i < ptrType.NumMethod(); i++ {
		method := ptrType.Method(i)
		if method.Name == TemplateContextKey {
			continue
		}
		mt := method.Type
		numIn := mt.NumIn() // includes receiver
		if numIn != 1 {
			continue
		}
		switch mt.NumOut() {
		case 1:
			meta = append(meta, methodMeta{Name: method.Name, Index: i})
		case 2:
			if mt.Out(1).Implements(errorInterface) {
				meta = append(meta, methodMeta{Name: method.Name, Index: i, HasError: true})
			}
		}
	}
	methodMetaCache.Store(ptrType, meta)
	return meta
}

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

	// Track whether input was a pointer so we can reuse it for method calls
	// instead of allocating a new one via reflect.New.
	var ptrVal reflect.Value
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			dataMap := make(map[string]interface{})
			dataMap[TemplateContextKey] = lvtContext
			return dataMap
		}
		ptrVal = val
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

		// Precompute exported methods so {{.MethodName}} works in templates.
		// Go templates auto-call methods on structs, but since we convert to a map,
		// we need to call zero-arg methods and store their return values.
		// Fields take precedence over methods (matching Go's resolution order).
		if !ptrVal.IsValid() {
			ptrVal = reflect.New(typ)
			ptrVal.Elem().Set(val)
		}
		for _, m := range getMethodMeta(ptrVal.Type()) {
			if _, exists := dataMap[m.Name]; exists {
				continue
			}
			results := ptrVal.Method(m.Index).Call(nil)
			if m.HasError && !results[1].IsNil() {
				continue
			}
			dataMap[m.Name] = results[0].Interface()
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
