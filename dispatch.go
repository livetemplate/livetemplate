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

// CapabilityChange is the capability name for controllers with a Change() method.
const CapabilityChange = "change"

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

// =============================================================================
// Controller+State Pattern Dispatch (New Signature)
// =============================================================================

// DispatchWithState routes an action to a controller method with new signature.
//
// Method signature: func(state StateType, ctx *Context) (StateType, error)
//
// Returns the modified state and any error from the method.
// The controller is a singleton that holds dependencies.
// State is passed by value and a new state is returned.
//
// Example:
//
//	type CounterController struct { DB *sql.DB }
//	func (c *CounterController) Increment(state CounterState, ctx *Context) (CounterState, error) {
//	    state.Count++
//	    return state, nil
//	}
func DispatchWithState(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
	if ctx == nil || ctx.action == "" {
		return state, ErrMethodNotFound
	}

	controllerValue := reflect.ValueOf(controller)
	controllerType := controllerValue.Type()
	stateType := reflect.TypeOf(state)

	// Get method index using cached lookup
	methodIndex := getMethodIndexNewSignature(controllerType, stateType, ctx.action)
	if methodIndex < 0 {
		return state, &DispatchError{
			Action:    ctx.action,
			StoreType: controllerType.String(),
			Err:       ErrMethodNotFound,
		}
	}

	// Call the method with state and context
	method := controllerValue.Method(methodIndex)
	results := method.Call([]reflect.Value{
		reflect.ValueOf(state),
		reflect.ValueOf(ctx),
	})

	// Extract results: (state, error)
	newState := results[0].Interface()
	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return newState, err
}

// HasActionMethod checks if a controller has a method that can handle the given action.
// Uses the same signature validation as DispatchWithState: func(state, *Context) (state, error).
// Automatically dereferences pointer state types to match the value type used by dispatch.
func HasActionMethod(controller interface{}, state interface{}, action string) bool {
	controllerType := reflect.TypeOf(controller)
	stateType := reflect.TypeOf(state)
	// DispatchWithState receives dereferenced value types (e.g., TodoState not *TodoState).
	// State.Inner() returns a pointer, so dereference to match the dispatch path.
	if stateType.Kind() == reflect.Ptr {
		stateType = stateType.Elem()
	}
	return getMethodIndexNewSignature(controllerType, stateType, action) >= 0
}

// methodCacheNewSignature caches method lookups for new signature
// Key: controllerType + stateType hash, Value: map[action]methodIndex
var methodCacheNewSignature sync.Map

// cacheKeyNewSig creates a cache key from controller and state types
type cacheKeyNewSig struct {
	controllerType reflect.Type
	stateType      reflect.Type
}

// getMethodIndexNewSignature returns method index for new signature methods
func getMethodIndexNewSignature(controllerType reflect.Type, stateType reflect.Type, action string) int {
	cacheKey := cacheKeyNewSig{controllerType, stateType}
	cached, ok := methodCacheNewSignature.Load(cacheKey)
	if ok {
		actionMap := cached.(map[string]int)
		if idx, found := actionMap[action]; found {
			return idx
		}
		return -1
	}

	// Build cache for this type combination
	actionMap := buildMethodCacheNewSignature(controllerType, stateType)
	methodCacheNewSignature.Store(cacheKey, actionMap)

	if idx, found := actionMap[action]; found {
		return idx
	}
	return -1
}

// buildMethodCacheNewSignature builds method cache for new signature
func buildMethodCacheNewSignature(controllerType reflect.Type, stateType reflect.Type) map[string]int {
	actionMap := make(map[string]int)
	contextType := reflect.TypeOf((*Context)(nil))
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	for i := 0; i < controllerType.NumMethod(); i++ {
		method := controllerType.Method(i)
		methodType := method.Type

		// Check: func(receiver, state, *Context) (state, error)
		// NumIn = 3 (receiver, state, ctx), NumOut = 2 (state, error)
		if methodType.NumIn() != 3 || methodType.NumOut() != 2 {
			continue
		}

		// First param (after receiver) must match state type
		if methodType.In(1) != stateType {
			continue
		}

		// Second param must be *Context
		if methodType.In(2) != contextType {
			continue
		}

		// First output must match state type
		if methodType.Out(0) != stateType {
			continue
		}

		// Second output must implement error
		if !methodType.Out(1).Implements(errorType) {
			continue
		}

		// Map method name to actions
		for _, actionName := range methodNameToActions(method.Name) {
			actionMap[actionName] = i
		}
	}

	return actionMap
}
