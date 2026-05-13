package livetemplate

import (
	"fmt"
	"log/slog"
	"reflect"
)

// =============================================================================
// Controller Lifecycle Methods
// =============================================================================
//
// Lifecycle methods are optional methods on controllers that the framework
// calls at specific points in the session/connection lifecycle:
//
// - Mount(state, ctx) -> (state, error)
//   Called on every HTTP request (GET and POST) and every WebSocket connect
//   (new and reconnect). Use for initial data loading from database.
//
// - OnConnect(state, ctx) -> (state, error)
//   Called every time a WebSocket connects (including reconnects).
//   Use for subscriptions or refresh logic.
//
// - OnDisconnect()
//   Called when WebSocket closes. Use for cleanup (unsubscribe, stop goroutines).
//

// callMount invokes the Mount method on a controller if it exists.
//
// Mount signature: func(state StateType, ctx *Context) (StateType, error)
//
// Mount is called when a new session is created and on every HTTP GET request.
// It receives the current state (from SessionStore) and returns refreshed state.
// Use it to load/refresh data from the database while preserving UI state.
// Keep Mount cheap — it runs on every page load, not just session creation.
//
// If the controller doesn't have a Mount method, state is returned unchanged.
func callMount(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
	return callLifecycleMethod(controller, state, ctx, "Mount")
}

// callOnConnect invokes the OnConnect method on a controller if it exists.
//
// OnConnect signature: func(state StateType, ctx *Context) (StateType, error)
//
// OnConnect is called every time a WebSocket connection is established,
// including reconnects. It's the place to:
// - Subscribe to PubSub channels
// - Refresh stale data
// - Set up connection-specific state
//
// If the controller doesn't have an OnConnect method, state is returned unchanged.
func callOnConnect(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
	return callLifecycleMethod(controller, state, ctx, "OnConnect")
}

// callLifecycleMethod is a helper that calls a lifecycle method with the standard
// signature: func(state StateType, ctx *Context) (StateType, error)
func callLifecycleMethod(controller interface{}, state interface{}, ctx *Context, methodName string) (interface{}, error) {
	controllerValue := reflect.ValueOf(controller)
	controllerType := controllerValue.Type()

	// Look for method by name
	method, ok := controllerType.MethodByName(methodName)
	if !ok {
		// No method - return state unchanged
		return state, nil
	}

	// Validate signature: func(state, *Context) (state, error)
	stateType := reflect.TypeOf(state)
	if !isValidLifecycleSignature(method.Type, stateType) {
		// Invalid signature - skip
		return state, nil
	}

	// Call the method
	results := controllerValue.Method(method.Index).Call([]reflect.Value{
		reflect.ValueOf(state),
		reflect.ValueOf(ctx),
	})

	newState := results[0].Interface()
	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return newState, err
}

// isValidLifecycleSignature checks if method has signature:
// func(state StateType, ctx *Context) (StateType, error)
func isValidLifecycleSignature(methodType reflect.Type, stateType reflect.Type) bool {
	contextType := reflect.TypeOf((*Context)(nil))
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	// NumIn = 3 (receiver, state, ctx), NumOut = 2 (state, error)
	if methodType.NumIn() != 3 || methodType.NumOut() != 2 {
		return false
	}

	// First param (after receiver) must match state type
	if methodType.In(1) != stateType {
		return false
	}

	// Second param must be *Context
	if methodType.In(2) != contextType {
		return false
	}

	// First return must match state type
	if methodType.Out(0) != stateType {
		return false
	}

	// Second return must implement error
	if !methodType.Out(1).Implements(errorType) {
		return false
	}

	return true
}

// validateLifecycleSignatures walks the controller's methods and emits an
// slog.Warn for any method whose name matches a known lifecycle name but
// whose signature doesn't validate. Called once at handler construction
// (Template.Handle) so the mismatch surfaces at server boot, before users
// notice the "method never runs" symptom in development.
//
// Lifecycle methods recognized:
//   - Mount, OnConnect: func(state StateType, ctx *Context) (StateType, error)
//   - OnDisconnect:     func()
//
// Methods with these names but a wrong signature are silently skipped at
// dispatch time by callLifecycleMethod / callOnDisconnect. This warning is
// non-fatal — controllers that happen to define a helper method with one
// of these names continue to function exactly as before.
func validateLifecycleSignatures(controller interface{}, state interface{}) {
	controllerType := reflect.TypeOf(controller)
	stateType := reflect.TypeOf(state)
	// In production state.Inner() is the result of AsState and never nil;
	// guard anyway so a misconfigured caller doesn't get a spurious "all
	// lifecycle methods invalid" warning. callLifecycleMethod will surface
	// the underlying issue at dispatch time.
	if stateType == nil {
		return
	}
	// callLifecycleMethod sees dereferenced value types (e.g. TodoState not
	// *TodoState) — mirror HasActionMethod's dereference so the validator
	// agrees with the dispatch path. Without this, valid Mount(value, *Context)
	// methods are flagged as mismatches against a pointer expected type.
	if stateType.Kind() == reflect.Pointer {
		stateType = stateType.Elem()
	}

	for _, name := range []string{"Mount", "OnConnect"} {
		m, ok := controllerType.MethodByName(name)
		if !ok {
			continue
		}
		if isValidLifecycleSignature(m.Type, stateType) {
			continue
		}
		slog.Warn("lifecycle method has invalid signature — will be skipped at runtime",
			slog.String("component", "live_handler"),
			slog.String("method", name),
			slog.String("controller", controllerType.String()),
			slog.String("expected",
				fmt.Sprintf("func(%s, *livetemplate.Context) (%s, error)",
					stateType, stateType)),
			slog.String("got", m.Type.String()))
	}

	if m, ok := controllerType.MethodByName("OnDisconnect"); ok {
		// OnDisconnect signature mirror of callOnDisconnect's check:
		// NumIn = 1 (receiver only), NumOut = 0.
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 0 {
			slog.Warn("lifecycle method has invalid signature — will be skipped at runtime",
				slog.String("component", "live_handler"),
				slog.String("method", "OnDisconnect"),
				slog.String("controller", controllerType.String()),
				slog.String("expected", "func()"),
				slog.String("got", m.Type.String()))
		}
	}
}

// callOnDisconnect invokes OnDisconnect() on a controller if it exists.
//
// OnDisconnect signature: func()
//
// OnDisconnect is called when a WebSocket connection closes. It's the place to:
// - Unsubscribe from PubSub channels
// - Stop background goroutines
// - Clean up any connection-specific resources
//
// Note: No state or context is passed because the connection is already closed.
// Any cleanup that needs state should be done before this point.
func callOnDisconnect(controller interface{}) {
	controllerValue := reflect.ValueOf(controller)
	controllerType := controllerValue.Type()

	method, ok := controllerType.MethodByName("OnDisconnect")
	if !ok {
		return
	}

	// Validate signature: func() with no params (besides receiver) and no returns
	if method.Type.NumIn() != 1 || method.Type.NumOut() != 0 {
		return
	}

	controllerValue.Method(method.Index).Call(nil)
}
