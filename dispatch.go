package livetemplate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// ErrMethodNotFound is returned when Dispatch cannot find a method matching the action.
var ErrMethodNotFound = errors.New("method not found for action")

// DispatchError provides context about a failed dispatch.
type DispatchError struct {
	Action    string
	StoreType string
	Err       error
}

func (e *DispatchError) Error() string {
	return fmt.Sprintf("%v: action %q not found on type %s", e.Err, e.Action, e.StoreType)
}

func (e *DispatchError) Unwrap() error {
	return e.Err
}

// methodCache caches method lookups by type to avoid repeated reflection.
// Key: reflect.Type, Value: map[action]methodIndex
var methodCache sync.Map

// Dispatch routes an action to a method on the store.
//
// It maps action names to methods using naming conventions:
//   - "increment" → Increment(ctx *ActionContext) error
//   - "add_item" → AddItem(ctx *ActionContext) error
//   - "updateProfile" → UpdateProfile(ctx *ActionContext) error
//
// Methods must have the signature: func(ctx *ActionContext) error
//
// Dispatch caches method lookups per type for zero reflection overhead
// on the hot path after the first call.
//
// Example:
//
//	type Counter struct {
//	    Count int
//	}
//
//	func (c *Counter) Increment(ctx *ActionContext) error {
//	    c.Count++
//	    return nil
//	}
//
//	// In template: lvt-click="increment"
//	// Framework calls: Dispatch(counter, ctx) → counter.Increment(ctx)
func Dispatch(store interface{}, ctx *ActionContext) error {
	if ctx == nil || ctx.Action == "" {
		return ErrMethodNotFound
	}

	storeValue := reflect.ValueOf(store)
	storeType := storeValue.Type()

	// Get or create method index cache for this type
	methodIndex := getMethodIndex(storeType, ctx.Action)
	if methodIndex < 0 {
		return &DispatchError{
			Action:    ctx.Action,
			StoreType: storeType.String(),
			Err:       ErrMethodNotFound,
		}
	}

	// Call the method
	method := storeValue.Method(methodIndex)
	results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})

	// Check error return
	if len(results) > 0 && !results[0].IsNil() {
		return results[0].Interface().(error)
	}

	return nil
}

// getMethodIndex returns the method index for the given action, using cache.
// Returns -1 if no matching method is found.
func getMethodIndex(storeType reflect.Type, action string) int {
	// Check cache first
	cacheKey := storeType
	cached, ok := methodCache.Load(cacheKey)
	if ok {
		actionMap := cached.(map[string]int)
		if idx, found := actionMap[action]; found {
			return idx
		}
		// Action not in cache, method doesn't exist
		return -1
	}

	// Build method cache for this type
	actionMap := buildMethodCache(storeType)
	methodCache.Store(cacheKey, actionMap)

	if idx, found := actionMap[action]; found {
		return idx
	}
	return -1
}

// buildMethodCache builds a map of action names to method indices for a type.
func buildMethodCache(storeType reflect.Type) map[string]int {
	actionMap := make(map[string]int)

	for i := 0; i < storeType.NumMethod(); i++ {
		method := storeType.Method(i)

		// Check method signature: func(ctx *ActionContext) error
		if !isValidActionMethod(method.Type) {
			continue
		}

		// Map method name to possible action names
		methodName := method.Name
		actions := methodNameToActions(methodName)
		for _, action := range actions {
			actionMap[action] = i
		}
	}

	return actionMap
}

// isValidActionMethod checks if a method has the signature: func(ctx *ActionContext) error
func isValidActionMethod(methodType reflect.Type) bool {
	// Method must have 2 inputs: receiver and *ActionContext
	if methodType.NumIn() != 2 {
		return false
	}

	// Second input must be *ActionContext
	ctxType := reflect.TypeOf((*ActionContext)(nil))
	if methodType.In(1) != ctxType {
		return false
	}

	// Must have exactly 1 output: error
	if methodType.NumOut() != 1 {
		return false
	}

	// Output must be error
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if !methodType.Out(0).Implements(errorType) {
		return false
	}

	return true
}

// methodNameToActions converts a method name to possible action names.
// Returns multiple variations to handle different naming conventions.
//
// Examples:
//   - "Increment" → ["increment", "Increment"]
//   - "AddItem" → ["add_item", "addItem", "AddItem"]
//   - "UpdateUserProfile" → ["update_user_profile", "updateUserProfile", "UpdateUserProfile"]
func methodNameToActions(methodName string) []string {
	actions := make([]string, 0, 3)

	// 1. lowercase first letter (camelCase)
	if len(methodName) > 0 {
		camelCase := strings.ToLower(methodName[:1]) + methodName[1:]
		actions = append(actions, camelCase)
	}

	// 2. snake_case
	snakeCase := toSnakeCase(methodName)
	if snakeCase != "" && snakeCase != actions[0] {
		actions = append(actions, snakeCase)
	}

	// 3. exact match (PascalCase)
	if methodName != actions[0] {
		actions = append(actions, methodName)
	}

	return actions
}

// toSnakeCase converts PascalCase/camelCase to snake_case.
// Example: "AddItem" → "add_item", "UpdateUserProfile" → "update_user_profile"
func toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 5) // Estimate with room for underscores

	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}
