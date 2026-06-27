package livetemplate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/livetemplate/livetemplate/internal/util"
)

// ErrMethodNotFound is returned when Dispatch cannot find a method matching the action.
var ErrMethodNotFound = errors.New("method not found for action")

// Capability names advertised in ResponseMetadata.Capabilities so the client
// can adapt its behavior to features the server supports.
//
// A capability should only be in the list if the client changes its behavior
// based on it. Detection happens once at Handle() time.
const (
	// CapabilityChange is set when the controller has a Change() method,
	// detected via reflection (see HasActionMethod).
	CapabilityChange = "change"
	// CapabilityValidate is set when the controller has a Validate() method,
	// detected via reflection (see HasActionMethod).
	CapabilityValidate = "validate"
	// CapabilityUpload is set when the template has at least one upload
	// configuration registered via WithUpload.
	CapabilityUpload = "upload"
	// CapabilityProgressiveEnhancement is set when non-JS form fallback is
	// enabled (default: enabled). Clients can use it to know the server
	// handles plain POSTs end-to-end.
	CapabilityProgressiveEnhancement = "progressive_enhancement"
)

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
// Returns multiple variations to handle different naming conventions, so a
// method routes from camelCase, snake_case, kebab-case, or exact-PascalCase
// action strings. kebab-case matters for progressive enhancement: HTML button
// names are conventionally kebab (<button name="save-draft">), so a no-JS POST
// must reach SaveDraft via "save-draft".
//
// Examples:
//   - "Increment" → ["increment", "Increment"]
//   - "AddItem" → ["addItem", "add_item", "add-item", "AddItem"]
//   - "UpdateUserProfile" → ["updateUserProfile", "update_user_profile", "update-user-profile", "UpdateUserProfile"]
func methodNameToActions(methodName string) []string {
	if methodName == "" {
		return nil
	}
	actions := make([]string, 0, 4)
	add := func(s string) {
		if s == "" {
			return
		}
		for _, existing := range actions {
			if existing == s {
				return
			}
		}
		actions = append(actions, s)
	}

	add(strings.ToLower(methodName[:1]) + methodName[1:]) // camelCase
	snake := toSnakeCase(methodName)
	add(snake)                               // snake_case
	add(strings.ReplaceAll(snake, "_", "-")) // kebab-case (HTML button names)
	add(methodName)                          // exact PascalCase

	return actions
}

func toSnakeCase(s string) string {
	return util.ToSnakeCase(s)
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
	if stateType.Kind() == reflect.Pointer {
		stateType = stateType.Elem()
	}
	return getMethodIndexNewSignature(controllerType, stateType, action) >= 0
}

// detectCapabilities builds the capability list advertised to the client.
// Order is fixed (change, validate, upload, progressive_enhancement) so wire
// output is deterministic.
//
// Returns nil when no capabilities apply so JSON marshaling can omit the
// field via `omitempty`.
func detectCapabilities(controller interface{}, state interface{}, cfg *mountConfig) []string {
	caps := make([]string, 0, 4)
	if HasActionMethod(controller, state, CapabilityChange) {
		caps = append(caps, CapabilityChange)
	}
	if HasActionMethod(controller, state, CapabilityValidate) {
		caps = append(caps, CapabilityValidate)
	}
	if len(cfg.UploadConfigs) > 0 {
		caps = append(caps, CapabilityUpload)
	}
	if cfg.ProgressiveEnhancement {
		caps = append(caps, CapabilityProgressiveEnhancement)
	}
	if len(caps) == 0 {
		return nil
	}
	return caps
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
