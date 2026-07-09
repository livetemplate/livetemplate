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
	// ValueIndex is the method's index in the value type's method set, or -1 when
	// the method has a pointer receiver (so it appears only in the pointer type's
	// set). This lets value-input structs invoke value-receiver methods without
	// allocating a pointer via reflect.New.
	ValueIndex int
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
	valType := ptrType.Elem()
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
		// A value-receiver method also appears in the value type's method set;
		// a pointer-receiver method does not (ValueIndex stays -1).
		valueIndex := -1
		if vm, ok := valType.MethodByName(method.Name); ok {
			valueIndex = vm.Index
		}
		switch mt.NumOut() {
		case 1:
			meta = append(meta, methodMeta{Name: method.Name, Index: i, ValueIndex: valueIndex})
		case 2:
			if mt.Out(1) == errorInterface {
				meta = append(meta, methodMeta{Name: method.Name, Index: i, ValueIndex: valueIndex, HasError: true})
			}
		}
	}
	actual, _ := methodMetaCache.LoadOrStore(ptrType, meta)
	return actual.([]methodMeta)
}

// safeMethodCall invokes a zero-arg method with panic recovery, matching the
// safety behavior of Go's html/template which recovers panics during execution.
func safeMethodCall(method reflect.Value) (results []reflect.Value, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			slog.Debug("method panicked during template data precomputation",
				slog.Any("panic", r))
		}
	}()
	return method.Call(nil), false
}

// BuildDataMap creates a template data map with lvt context from the given data.
// This is the single source of truth for data→map conversion, used by both
// template execution and tree building to avoid duplicate reflection.
//
// For struct data, exported zero-arg methods are eagerly evaluated and their
// return values stored in the map, because html/template cannot auto-call methods
// once the struct has been converted to a map.
//
// precomputeAllow scopes which methods are evaluated: when non-nil, only methods
// whose name is present in the set are called (the set is a superset of the
// identifiers referenced by the templates — see parse.CollectReferencedIdents), so a
// method the templates never reference is skipped entirely. A nil set means evaluate
// all qualifying methods, preserving the original eager behavior for direct callers.
func BuildDataMap(data interface{}, messages map[string]string, devMode bool, uploadRegistry interface{}, precomputeAllow map[string]struct{}) interface{} {
	if messages == nil {
		messages = make(map[string]string)
	}

	lvtContext := NewTemplateContext(messages, devMode)
	if uploadRegistry != nil {
		lvtContext.SetUploadRegistry(uploadRegistry)
	}

	return buildDataMapWithContext(data, lvtContext, precomputeAllow)
}

// methodAllowed reports whether a method should be precomputed. A nil allow-set
// means "precompute everything" (the pre-scoping default).
func methodAllowed(allow map[string]struct{}, name string) bool {
	if allow == nil {
		return true
	}
	_, ok := allow[name]
	return ok
}

// buildDataMapWithContext does the actual reflection and map construction.
func buildDataMapWithContext(data interface{}, lvtContext *TemplateContext, precomputeAllow map[string]struct{}) interface{} {
	val := reflect.ValueOf(data)

	// Track whether input was a pointer so we can reuse it for method calls
	// instead of allocating a new one via reflect.New.
	var ptrVal reflect.Value
	if val.Kind() == reflect.Pointer {
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
		//
		// Semantic note: methods in precomputeAllow are eagerly evaluated on every
		// render (a nil set means all qualifying methods). Methods that return
		// (T, error) are omitted with a warning when error is non-nil (unlike
		// html/template which would stop execution with the error).
		methods := getMethodMeta(reflect.PointerTo(typ))

		// For value-type input, only allocate a pointer (reflect.New) if some
		// qualifying method has a pointer receiver. When every method is a value
		// receiver, call them directly on val and skip the per-render allocation.
		// Methods excluded by the allow-set are ignored here too, so an unreferenced
		// pointer-receiver method never forces the allocation.
		if !ptrVal.IsValid() {
			for _, m := range methods {
				if !methodAllowed(precomputeAllow, m.Name) {
					continue
				}
				if m.ValueIndex < 0 {
					ptrVal = reflect.New(typ)
					ptrVal.Elem().Set(val)
					break
				}
			}
		}

		for _, m := range methods {
			if !methodAllowed(precomputeAllow, m.Name) {
				continue
			}
			if _, exists := dataMap[m.Name]; exists {
				continue
			}
			var method reflect.Value
			if ptrVal.IsValid() {
				method = ptrVal.Method(m.Index)
			} else {
				method = val.Method(m.ValueIndex)
			}
			results, panicked := safeMethodCall(method)
			if panicked {
				continue
			}
			if m.HasError && !results[1].IsNil() {
				slog.Warn("method returned error during template data precomputation, omitting",
					slog.String("method", m.Name),
					slog.String("type", typ.Name()),
					slog.Any("error", results[1].Interface()))
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
	return BuildDataMap(data, messages, devMode, registry, nil)
}
