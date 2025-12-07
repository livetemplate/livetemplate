# Controller + State Pattern Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor LiveTemplate to separate Controller (singleton, holds dependencies) from State (pure data, cloned per session), eliminating reflection-based cloning of dependencies.

**Architecture:** Controller + State separation with `AsState[T]()` generic wrapper for zero-boilerplate state marking. Methods receive state by value and return `(StateType, error)` tuple. Unified `*Context` replaces `*ActionContext`.

**Tech Stack:** Go 1.21+, generics, reflection for method dispatch, JSON for default serialization

---

## Overview

This is a breaking change that removes:
- `Handle(store)` single-argument signature
- `lvt:"state"` tag
- `cloneStore()` reflection-based cloning
- `ActionContext` type
- `StoreInitializer.Init()` method

And replaces with:
- `Handle(controller, AsState(state), ...options)`
- `AsState[T]()` generic wrapper
- `*Context` unified context
- `Mount()`, `OnConnect()`, `OnDisconnect()` lifecycle methods
- Tuple return `(StateType, error)` pattern

---

## Task 1: Create State Interface and AsState Wrapper

**Files:**
- Modify: `state.go`
- Test: `state_test.go`

**Step 1: Write failing test for State interface**

```go
// state_test.go
func TestAsState_MarshalUnmarshal(t *testing.T) {
    type TodoState struct {
        Items []string
        Count int
    }

    original := &TodoState{Items: []string{"buy milk"}, Count: 1}
    state := AsState(original)

    // Marshal
    data, err := state.MarshalBinary()
    if err != nil {
        t.Fatalf("MarshalBinary failed: %v", err)
    }

    // Unmarshal into new instance
    restored := &TodoState{}
    restoredState := AsState(restored)
    if err := restoredState.UnmarshalBinary(data); err != nil {
        t.Fatalf("UnmarshalBinary failed: %v", err)
    }

    // Verify
    if restored.Count != original.Count {
        t.Errorf("Count mismatch: got %d, want %d", restored.Count, original.Count)
    }
    if len(restored.Items) != len(original.Items) {
        t.Errorf("Items length mismatch: got %d, want %d", len(restored.Items), len(original.Items))
    }
}

func TestAsState_Inner(t *testing.T) {
    type MyState struct{ Value int }
    original := &MyState{Value: 42}
    state := AsState(original)

    inner := state.Inner()
    if inner != original {
        t.Error("Inner() should return original pointer")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestAsState -v`
Expected: FAIL with "undefined: AsState"

**Step 3: Write State interface and AsState implementation**

Add to `state.go`:

```go
// State is the interface for session state that can be persisted.
// The serialization requirement ensures state contains only pure data.
// Use AsState[T]() for zero-boilerplate implementation.
type State interface {
    encoding.BinaryMarshaler
    encoding.BinaryUnmarshaler
    Inner() any // Returns the underlying value for framework use
}

// AsState wraps a plain struct pointer to satisfy the State interface.
// Uses JSON serialization by default. For custom serialization,
// implement the State interface directly on your type.
//
// Example:
//   state := AsState(&TodoState{})
//   handler := tmpl.Handle(&TodoController{DB: db}, state)
func AsState[T any](s *T) State {
    return &jsonState[T]{value: s}
}

// jsonState is the generic wrapper implementing State with JSON serialization
type jsonState[T any] struct {
    value *T
}

func (s *jsonState[T]) MarshalBinary() ([]byte, error) {
    return json.Marshal(s.value)
}

func (s *jsonState[T]) UnmarshalBinary(data []byte) error {
    return json.Unmarshal(data, s.value)
}

func (s *jsonState[T]) Inner() any {
    return s.value
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestAsState -v`
Expected: PASS

**Step 5: Commit**

```bash
git add state.go state_test.go
git commit -m "feat: add State interface and AsState generic wrapper

Implements the core abstraction for Controller+State pattern.
- State interface requires MarshalBinary/UnmarshalBinary (serialization = purity marker)
- AsState[T]() provides zero-boilerplate JSON serialization wrapper
- Inner() method for framework to access underlying value

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Create Unified Context Type

**Files:**
- Create: `context.go`
- Test: `context_test.go`

**Step 1: Write failing test for Context**

```go
// context_test.go
package livetemplate

import (
    "context"
    "testing"
)

func TestContext_GetString(t *testing.T) {
    data := map[string]interface{}{"name": "Alice"}
    ctx := NewContext(context.Background(), "test_action", data)

    if got := ctx.GetString("name"); got != "Alice" {
        t.Errorf("GetString(name) = %q, want %q", got, "Alice")
    }

    if got := ctx.GetString("missing"); got != "" {
        t.Errorf("GetString(missing) = %q, want empty", got)
    }
}

func TestContext_Action(t *testing.T) {
    ctx := NewContext(context.Background(), "increment", nil)

    if got := ctx.Action(); got != "increment" {
        t.Errorf("Action() = %q, want %q", got, "increment")
    }
}

func TestContext_UserID(t *testing.T) {
    ctx := NewContext(context.Background(), "test", nil)
    ctx = ctx.WithUserID("user-123")

    if got := ctx.UserID(); got != "user-123" {
        t.Errorf("UserID() = %q, want %q", got, "user-123")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestContext_ -v`
Expected: FAIL with "undefined: NewContext"

**Step 3: Write Context implementation**

Create `context.go`:

```go
package livetemplate

import (
    "context"
    "net/http"
)

// Context provides unified context for all controller lifecycle methods.
// It embeds context.Context for cancellation, timeout, and request-scoped values.
type Context struct {
    context.Context
    action  string
    data    *ActionData
    userID  string
    session Session

    // HTTP context (nil for WebSocket actions)
    w http.ResponseWriter
    r *http.Request
}

// NewContext creates a new Context for action handling.
func NewContext(ctx context.Context, action string, data map[string]interface{}) *Context {
    return &Context{
        Context: ctx,
        action:  action,
        data:    newActionData(data),
    }
}

// Action returns the action name that triggered this context.
func (c *Context) Action() string {
    return c.action
}

// UserID returns the authenticated user's ID.
func (c *Context) UserID() string {
    return c.userID
}

// WithUserID returns a new Context with the given user ID.
func (c *Context) WithUserID(userID string) *Context {
    newCtx := *c
    newCtx.userID = userID
    return &newCtx
}

// Session returns the Session for server-initiated actions.
func (c *Context) Session() Session {
    return c.session
}

// WithSession returns a new Context with the given session.
func (c *Context) WithSession(session Session) *Context {
    newCtx := *c
    newCtx.session = session
    return &newCtx
}

// Data extraction methods (delegate to ActionData)

func (c *Context) GetString(key string) string {
    if c.data == nil {
        return ""
    }
    return c.data.GetString(key)
}

func (c *Context) GetInt(key string) int {
    if c.data == nil {
        return 0
    }
    return c.data.GetInt(key)
}

func (c *Context) GetFloat(key string) float64 {
    if c.data == nil {
        return 0
    }
    return c.data.GetFloat(key)
}

func (c *Context) GetBool(key string) bool {
    if c.data == nil {
        return false
    }
    return c.data.GetBool(key)
}

func (c *Context) Has(key string) bool {
    if c.data == nil {
        return false
    }
    return c.data.Has(key)
}

func (c *Context) Get(key string) interface{} {
    if c.data == nil {
        return nil
    }
    return c.data.Get(key)
}

// Bind unmarshals the action data into a struct.
func (c *Context) Bind(v interface{}) error {
    if c.data == nil {
        return nil
    }
    return c.data.Bind(v)
}

// HTTP Methods (same as ActionContext)

func (c *Context) IsHTTP() bool {
    return c.w != nil && c.r != nil
}

func (c *Context) SetCookie(cookie *http.Cookie) error {
    if c.w == nil {
        return ErrNoHTTPContext
    }
    http.SetCookie(c.w, cookie)
    return nil
}

func (c *Context) GetCookie(name string) (*http.Cookie, error) {
    if c.r == nil {
        return nil, ErrNoHTTPContext
    }
    return c.r.Cookie(name)
}

func (c *Context) Redirect(url string, code int) error {
    if c.w == nil || c.r == nil {
        return ErrNoHTTPContext
    }
    if code < 300 || code >= 400 {
        return ErrInvalidRedirectCode
    }
    if !isValidRedirectURL(url) {
        return ErrInvalidRedirectURL
    }
    http.Redirect(c.w, c.r, url, code)
    return nil
}

// WithHTTP returns a new Context with HTTP request/response.
func (c *Context) WithHTTP(w http.ResponseWriter, r *http.Request) *Context {
    newCtx := *c
    newCtx.w = w
    newCtx.r = r
    return &newCtx
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestContext_ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add context.go context_test.go
git commit -m "feat: add unified Context type for lifecycle methods

Replaces ActionContext with a single Context type used across all lifecycle methods.
- Embeds context.Context for cancellation/timeout
- Provides data extraction methods (GetString, GetInt, etc.)
- Supports HTTP methods (SetCookie, Redirect) for HTTP POST actions
- Chainable With* methods for building context

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Update Dispatch for New Method Signatures

**Files:**
- Modify: `dispatch.go`
- Modify: `dispatch_test.go`

**Step 1: Write failing test for new signature**

```go
// dispatch_test.go
func TestDispatch_NewSignature(t *testing.T) {
    type CounterState struct {
        Count int
    }

    type CounterController struct{}

    // New signature: (state, ctx) -> (state, error)
    // Note: We test via reflection since Dispatch needs to handle this

    ctrl := &CounterController{}
    state := CounterState{Count: 5}
    ctx := NewContext(context.Background(), "increment", nil)

    newState, err := DispatchWithState(ctrl, state, ctx)
    if err != nil {
        t.Fatalf("DispatchWithState failed: %v", err)
    }

    result, ok := newState.(CounterState)
    if !ok {
        t.Fatalf("Expected CounterState, got %T", newState)
    }

    if result.Count != 6 {
        t.Errorf("Count = %d, want 6", result.Count)
    }
}

// Method for test
func (c *CounterController) Increment(state CounterState, ctx *Context) (CounterState, error) {
    state.Count++
    return state, nil
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestDispatch_NewSignature -v`
Expected: FAIL with "undefined: DispatchWithState"

**Step 3: Write DispatchWithState implementation**

Add to `dispatch.go`:

```go
// DispatchWithState routes an action to a controller method with new signature.
//
// Method signature: func(state StateType, ctx *Context) (StateType, error)
//
// Returns the modified state and any error from the method.
func DispatchWithState(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
    if ctx == nil || ctx.action == "" {
        return state, ErrMethodNotFound
    }

    controllerValue := reflect.ValueOf(controller)
    controllerType := controllerValue.Type()

    // Get method index using cached lookup
    methodIndex := getMethodIndexNewSignature(controllerType, ctx.action)
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

// methodCacheNewSignature caches method lookups for new signature
var methodCacheNewSignature sync.Map

// getMethodIndexNewSignature returns method index for new signature methods
func getMethodIndexNewSignature(controllerType reflect.Type, action string) int {
    cacheKey := controllerType
    cached, ok := methodCacheNewSignature.Load(cacheKey)
    if ok {
        actionMap := cached.(map[string]int)
        if idx, found := actionMap[action]; found {
            return idx
        }
        return -1
    }

    // Build cache for this type
    actionMap := buildMethodCacheNewSignature(controllerType)
    methodCacheNewSignature.Store(cacheKey, actionMap)

    if idx, found := actionMap[action]; found {
        return idx
    }
    return -1
}

// buildMethodCacheNewSignature builds method cache for new signature
func buildMethodCacheNewSignature(controllerType reflect.Type) map[string]int {
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

        // Third param must be *Context
        if methodType.In(2) != contextType {
            continue
        }

        // Second output must implement error
        if !methodType.Out(1).Implements(errorType) {
            continue
        }

        // First input (after receiver) and first output must be same type (state)
        stateInType := methodType.In(1)
        stateOutType := methodType.Out(0)
        if stateInType != stateOutType {
            continue
        }

        // Map method name to actions
        for _, action := range methodNameToActions(method.Name) {
            actionMap[action] = i
        }
    }

    return actionMap
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestDispatch_NewSignature -v`
Expected: PASS

**Step 5: Commit**

```bash
git add dispatch.go dispatch_test.go
git commit -m "feat: add DispatchWithState for new method signatures

Adds dispatch function supporting (state, ctx) -> (state, error) signature.
- DispatchWithState handles new Controller+State pattern
- Validates method signature: same state type in/out
- Uses cached method lookups for performance
- Original Dispatch remains for backward compat during migration

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Add Mount Lifecycle Method

**Files:**
- Modify: `mount.go`
- Test: `mount_test.go`

**Step 1: Write failing test for Mount**

```go
// mount_test.go
func TestMount_Lifecycle(t *testing.T) {
    type TodoState struct {
        Items []string
        Loaded bool
    }

    type TodoController struct {
        MountCalled bool
    }

    func (c *TodoController) Mount(state TodoState, ctx *Context) (TodoState, error) {
        c.MountCalled = true
        state.Items = []string{"item1", "item2"}
        state.Loaded = true
        return state, nil
    }

    ctrl := &TodoController{}
    initialState := TodoState{}
    ctx := NewContext(context.Background(), "", nil)

    newState, err := callMount(ctrl, initialState, ctx)
    if err != nil {
        t.Fatalf("Mount failed: %v", err)
    }

    if !ctrl.MountCalled {
        t.Error("Mount was not called")
    }

    result := newState.(TodoState)
    if !result.Loaded {
        t.Error("State.Loaded should be true")
    }
    if len(result.Items) != 2 {
        t.Errorf("Items count = %d, want 2", len(result.Items))
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestMount_Lifecycle -v`
Expected: FAIL with "undefined: callMount"

**Step 3: Write callMount implementation**

Add to `mount.go`:

```go
// Mountable is implemented by controllers that need initialization on session creation.
// Mount is called once when a new session is created (not on reconnect).
type Mountable interface{}

// callMount invokes the Mount method on a controller if it exists.
// Mount signature: func(state StateType, ctx *Context) (StateType, error)
func callMount(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
    controllerValue := reflect.ValueOf(controller)
    controllerType := controllerValue.Type()

    // Look for Mount method
    method, ok := controllerType.MethodByName("Mount")
    if !ok {
        // No Mount method - return state unchanged
        return state, nil
    }

    // Validate signature: func(state, *Context) (state, error)
    if !isValidMountSignature(method.Type, reflect.TypeOf(state)) {
        return state, nil // Invalid signature, skip
    }

    // Call Mount
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

// isValidMountSignature checks if method has signature: func(state, *Context) (state, error)
func isValidMountSignature(methodType reflect.Type, stateType reflect.Type) bool {
    contextType := reflect.TypeOf((*Context)(nil))
    errorType := reflect.TypeOf((*error)(nil)).Elem()

    // NumIn = 3 (receiver, state, ctx), NumOut = 2 (state, error)
    if methodType.NumIn() != 3 || methodType.NumOut() != 2 {
        return false
    }

    // Params: state type in, *Context
    if methodType.In(1) != stateType {
        return false
    }
    if methodType.In(2) != contextType {
        return false
    }

    // Returns: state type, error
    if methodType.Out(0) != stateType {
        return false
    }
    if !methodType.Out(1).Implements(errorType) {
        return false
    }

    return true
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestMount_Lifecycle -v`
Expected: PASS

**Step 5: Commit**

```bash
git add mount.go mount_test.go
git commit -m "feat: add Mount lifecycle method support

Adds callMount function for session initialization:
- Mount(state, ctx) -> (state, error) called once per session
- Uses reflection to find and validate Mount method
- Returns unchanged state if no Mount method exists
- Validates signature matches expected pattern

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Add OnConnect Lifecycle Method

**Files:**
- Modify: `mount.go`
- Test: `mount_test.go`

**Step 1: Write failing test for OnConnect**

```go
func TestOnConnect_Lifecycle(t *testing.T) {
    type ChatState struct {
        Connected bool
    }

    type ChatController struct {
        OnConnectCalled bool
    }

    func (c *ChatController) OnConnect(state ChatState, ctx *Context) (ChatState, error) {
        c.OnConnectCalled = true
        state.Connected = true
        return state, nil
    }

    ctrl := &ChatController{}
    state := ChatState{}
    ctx := NewContext(context.Background(), "", nil)

    newState, err := callOnConnect(ctrl, state, ctx)
    if err != nil {
        t.Fatalf("OnConnect failed: %v", err)
    }

    if !ctrl.OnConnectCalled {
        t.Error("OnConnect was not called")
    }

    result := newState.(ChatState)
    if !result.Connected {
        t.Error("State.Connected should be true")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestOnConnect_Lifecycle -v`
Expected: FAIL with "undefined: callOnConnect"

**Step 3: Write callOnConnect implementation**

Add to `mount.go`:

```go
// callOnConnect invokes the OnConnect method on a controller if it exists.
// OnConnect signature: func(state StateType, ctx *Context) (StateType, error)
// Called on every WebSocket connection (including reconnects).
func callOnConnect(controller interface{}, state interface{}, ctx *Context) (interface{}, error) {
    controllerValue := reflect.ValueOf(controller)
    controllerType := controllerValue.Type()

    method, ok := controllerType.MethodByName("OnConnect")
    if !ok {
        return state, nil
    }

    if !isValidMountSignature(method.Type, reflect.TypeOf(state)) {
        return state, nil
    }

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
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestOnConnect_Lifecycle -v`
Expected: PASS

**Step 5: Commit**

```bash
git add mount.go mount_test.go
git commit -m "feat: add OnConnect lifecycle method support

Adds callOnConnect function for WebSocket connection events:
- OnConnect(state, ctx) -> (state, error) called on every WS connect
- Reuses isValidMountSignature for validation
- Returns unchanged state if no OnConnect method exists

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Add OnDisconnect Lifecycle Method

**Files:**
- Modify: `mount.go`
- Test: `mount_test.go`

**Step 1: Write failing test**

```go
func TestOnDisconnect_Lifecycle(t *testing.T) {
    type StreamController struct {
        DisconnectCalled bool
    }

    func (c *StreamController) OnDisconnect() {
        c.DisconnectCalled = true
    }

    ctrl := &StreamController{}

    callOnDisconnect(ctrl)

    if !ctrl.DisconnectCalled {
        t.Error("OnDisconnect was not called")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestOnDisconnect_Lifecycle -v`
Expected: FAIL with "undefined: callOnDisconnect"

**Step 3: Write callOnDisconnect implementation**

```go
// callOnDisconnect invokes OnDisconnect() on a controller if it exists.
// OnDisconnect signature: func()
// Called when WebSocket connection closes.
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
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestOnDisconnect_Lifecycle -v`
Expected: PASS

**Step 5: Commit**

```bash
git add mount.go mount_test.go
git commit -m "feat: add OnDisconnect lifecycle method support

Adds callOnDisconnect function for cleanup on WS close:
- OnDisconnect() called when WebSocket closes
- No state or context (connection already closed)
- For cleanup: unsubscribe pubsub, stop goroutines, etc.

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Update Handle() Signature

**Files:**
- Modify: `template.go`
- Modify: `mount.go`
- Test: `template_test.go`

**Step 1: Write failing test for new Handle signature**

```go
func TestHandle_NewSignature(t *testing.T) {
    type CounterState struct {
        Count int
    }

    type CounterController struct{}

    func (c *CounterController) Mount(state CounterState, ctx *Context) (CounterState, error) {
        return state, nil
    }

    func (c *CounterController) Increment(state CounterState, ctx *Context) (CounterState, error) {
        state.Count++
        return state, nil
    }

    tmpl, err := New("test", WithParseFiles("testdata/fixtures/simple.html"))
    if err != nil {
        t.Skip("Test requires fixture file")
    }

    handler := tmpl.Handle(&CounterController{}, AsState(&CounterState{}))

    if handler == nil {
        t.Fatal("Handle returned nil")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `GOWORK=off go test -run TestHandle_NewSignature -v`
Expected: FAIL (signature mismatch or compilation error)

**Step 3: Update Handle implementation**

This is a larger change. Update `template.go` Handle signature:

```go
// Handle creates an http.Handler for the template with controller and state.
//
// Controller: Singleton that holds dependencies (DB, Logger, etc.). Never cloned.
// State: Pure data that is cloned per session. Must be wrapped with AsState().
//
// Example:
//   handler := tmpl.Handle(
//       &TodoController{DB: db, Logger: logger},
//       AsState(&TodoState{}),
//   )
//   http.Handle("/todos", handler)
func (t *Template) Handle(controller interface{}, state State, opts ...HandleOption) LiveHandler {
    // Validate inputs
    if controller == nil {
        panic("Handle: controller cannot be nil")
    }
    if state == nil {
        panic("Handle: state cannot be nil - use AsState(&YourState{})")
    }

    // Apply options
    config := handleConfig{}
    for _, opt := range opts {
        opt(&config)
    }

    // Create mount config
    mountCfg := mountConfig{
        Template:      t,
        Controller:    controller,
        State:         state,
        // ... rest of config
    }

    return newLiveHandler(mountCfg)
}

// HandleOption configures Handle behavior
type HandleOption func(*handleConfig)

type handleConfig struct {
    sessionStore SessionStore
    // ... other options
}

// WithStore sets the session store for state persistence
func WithStore(store SessionStore) HandleOption {
    return func(c *handleConfig) {
        c.sessionStore = store
    }
}
```

**Step 4: Run test to verify it passes**

Run: `GOWORK=off go test -run TestHandle_NewSignature -v`
Expected: PASS

**Step 5: Commit**

```bash
git add template.go mount.go template_test.go
git commit -m "feat!: update Handle() to accept controller and state separately

BREAKING CHANGE: Handle(store) -> Handle(controller, state, ...opts)

New signature enforces Controller+State separation:
- Controller: singleton with dependencies, never cloned
- State: pure data wrapped with AsState(), cloned per session

This prevents accidental sharing of dependencies across sessions.

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Wire Up Lifecycle in WebSocket Handler

**Files:**
- Modify: `mount.go`

**Step 1: Update handleWebSocket to use new lifecycle**

This task involves integrating the lifecycle methods into the WebSocket handler:

1. On new session: Call `Mount()`, store state
2. On WS connect: Call `OnConnect()`, update state
3. On action: Call action method via `DispatchWithState()`
4. On WS disconnect: Call `OnDisconnect()`

The implementation is complex and involves refactoring `handleWebSocket()`. See the detailed changes below.

**Step 2: Test with integration test**

Create an integration test that validates the full lifecycle.

**Step 3: Commit**

```bash
git add mount.go
git commit -m "feat: wire up lifecycle methods in WebSocket handler

Integrates Controller+State lifecycle into handleWebSocket:
- Mount() called on new session creation
- OnConnect() called on every WS connection
- Actions dispatched via DispatchWithState()
- OnDisconnect() called on WS close
- State cloned via AsState serialization (not reflection)

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 9: Remove Old Store/cloneStore Code

**Files:**
- Modify: `mount.go`
- Modify: `action.go`
- Modify: `state.go`

**Step 1: Remove deprecated code**

Remove:
- `cloneStore()` function
- `copyStruct()` function
- `hydrateStores()` function (replaced by new state serialization)
- `StoreInitializer` interface
- `Stores` type alias
- `ActionContext` type (keep as deprecated alias to Context for one release?)
- `lvt:"state"` tag handling

**Step 2: Run all tests**

Run: `GOWORK=off go test ./... -v`
Expected: PASS (or identify tests that need updating)

**Step 3: Commit**

```bash
git add mount.go action.go state.go
git commit -m "feat!: remove old store/cloneStore code

BREAKING CHANGE: Removes deprecated store mechanisms

Removed:
- cloneStore() - replaced by State serialization
- copyStruct() - no longer needed
- hydrateStores() - replaced by State deserialization
- StoreInitializer.Init() - replaced by Mount()
- Stores type alias - use explicit controller+state
- lvt:'state' tag - replaced by AsState wrapper

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 10: Add AssertPureState Test Helper

**Files:**
- Create: `testing.go`
- Test: `testing_test.go`

**Step 1: Write test for AssertPureState**

```go
// testing_test.go
func TestAssertPureState_DetectsPointers(t *testing.T) {
    type BadState struct {
        DB *sql.DB // Dependency - should fail
    }

    // This should fail because BadState contains pointer
    // We test that it produces the right error
    err := validatePureState[BadState]()
    if err == nil {
        t.Error("Expected error for state with pointer field")
    }
}

func TestAssertPureState_AllowsDataPointers(t *testing.T) {
    type Item struct{ Name string }
    type GoodState struct {
        Items []*Item // Data pointer - OK
        Count int
    }

    err := validatePureState[GoodState]()
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }
}
```

**Step 2: Write AssertPureState implementation**

```go
// testing.go
package livetemplate

import (
    "fmt"
    "reflect"
    "testing"
)

// AssertPureState validates that a state type contains only serializable data.
// Use in tests to catch accidental dependency inclusion:
//
//   func TestMyState_IsPure(t *testing.T) {
//       AssertPureState[MyState](t)
//   }
func AssertPureState[T any](t *testing.T) {
    t.Helper()
    if err := validatePureState[T](); err != nil {
        t.Error(err)
    }
}

func validatePureState[T any]() error {
    var zero T
    typ := reflect.TypeOf(zero)

    return validatePureStateType(typ, "")
}

func validatePureStateType(typ reflect.Type, path string) error {
    if typ.Kind() == reflect.Ptr {
        typ = typ.Elem()
    }

    if typ.Kind() != reflect.Struct {
        return nil // Non-struct types are OK
    }

    for i := 0; i < typ.NumField(); i++ {
        field := typ.Field(i)
        fieldPath := path + "." + field.Name
        if path == "" {
            fieldPath = field.Name
        }

        fieldType := field.Type

        // Check for common dependency patterns
        if isDependencyType(fieldType) {
            return fmt.Errorf("field %s appears to be a dependency (%s) - move to controller",
                fieldPath, fieldType.String())
        }

        // Recursively check embedded structs
        if fieldType.Kind() == reflect.Struct {
            if err := validatePureStateType(fieldType, fieldPath); err != nil {
                return err
            }
        }
    }

    return nil
}

// isDependencyType checks if a type looks like a dependency
func isDependencyType(typ reflect.Type) bool {
    if typ.Kind() != reflect.Ptr && typ.Kind() != reflect.Interface {
        return false
    }

    name := typ.String()
    // Common dependency patterns
    patterns := []string{
        "*sql.DB", "*sql.Tx", "*sql.Conn",
        "*slog.Logger", "*log.Logger",
        "*http.Client",
        "*redis.Client",
        "io.Writer", "io.Reader",
    }

    for _, p := range patterns {
        if name == p {
            return true
        }
    }

    return false
}
```

**Step 3: Run tests**

Run: `GOWORK=off go test -run TestAssertPureState -v`

**Step 4: Commit**

```bash
git add testing.go testing_test.go
git commit -m "feat: add AssertPureState test helper

Provides test-time validation that state types are pure data:
- AssertPureState[T](t) for use in tests
- Detects common dependency patterns (*sql.DB, *slog.Logger, etc.)
- Recursively checks embedded structs
- Produces clear error messages pointing to problematic fields

Part of #67: Controller+State pattern implementation

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 11: Update Existing Tests

**Files:**
- Modify: Various test files

Update all existing tests to use the new API:
- Replace `Handle(store)` with `Handle(controller, AsState(state))`
- Update method signatures
- Replace `ActionContext` with `Context`

This is a sweep through test files to update them to the new patterns.

---

## Task 12: Run Full Test Suite

**Step 1: Run all tests**

```bash
GOWORK=off go test ./... -v -count=1
```

**Step 2: Fix any failures**

Address any test failures found.

**Step 3: Run with race detector**

```bash
GOWORK=off go test ./... -race -v
```

**Step 4: Commit any fixes**

---

## Progress Tracker

- [ ] Task 1: Create State Interface and AsState Wrapper
- [ ] Task 2: Create Unified Context Type
- [ ] Task 3: Update Dispatch for New Method Signatures
- [ ] Task 4: Add Mount Lifecycle Method
- [ ] Task 5: Add OnConnect Lifecycle Method
- [ ] Task 6: Add OnDisconnect Lifecycle Method
- [ ] Task 7: Update Handle() Signature
- [ ] Task 8: Wire Up Lifecycle in WebSocket Handler
- [ ] Task 9: Remove Old Store/cloneStore Code
- [ ] Task 10: Add AssertPureState Test Helper
- [ ] Task 11: Update Existing Tests
- [ ] Task 12: Run Full Test Suite
