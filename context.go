package livetemplate

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// UploadAccessor provides access to upload entries during action handling.
type UploadAccessor interface {
	HasUploads(name string) bool
	GetCompletedUploads(name string) []*uploadtypes.UploadEntry
}

// FlashSetter allows setting flash messages from action handlers.
// Flash messages are page-level notifications (success, info, warning, error)
// that don't affect ResponseMetadata.Success (unlike field validation errors).
//
// FlashSetter is the internal interface for flash message operations.
// The methods are intentionally unexported to ensure flash messages
// are only managed through the Context.SetFlash() / ClearFlash() public
// API, maintaining consistent behavior and preventing direct message
// map manipulation.
type FlashSetter interface {
	setFlash(key, message string, expiry time.Duration)
	clearFlashKey(key string)
}

// broadcastRequest represents a deferred broadcast action to be dispatched
// to other connections in the same session group after the current action completes.
type broadcastRequest struct {
	Action string
	Data   map[string]interface{}
}

// Context provides unified context for all controller lifecycle methods.
// It embeds context.Context for cancellation, timeout, and request-scoped values.
//
// Context replaces ActionContext with a single type used across:
// - Mount(state, ctx) - session initialization
// - OnConnect(state, ctx) - WebSocket connect
// - Action methods(state, ctx) - user interactions
type Context struct {
	context.Context
	action      string
	data        *ActionData
	userID      string
	session     Session
	uploads     UploadAccessor
	flashSetter FlashSetter
	formSchema  *FormSchema
	broadcasts  []broadcastRequest

	// HTTP context (nil for WebSocket actions)
	w          http.ResponseWriter
	r          *http.Request
	redirected *bool // shared across With*() copies so mount.go sees the flag
}

// NewContext creates a new Context for action handling.
func NewContext(ctx context.Context, action string, data map[string]interface{}) *Context {
	redirected := false
	return &Context{
		Context:    ctx,
		action:     action,
		data:       newActionData(data),
		redirected: &redirected,
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

// BindAndValidate binds data to struct and validates it in one step.
// Uses the provided go-playground/validator instance for validation.
func (c *Context) BindAndValidate(v interface{}, validate *validator.Validate) error {
	if c.data == nil {
		return nil
	}
	return c.data.BindAndValidate(v, validate)
}

// HTTP Methods (same as ActionContext)

func (c *Context) IsHTTP() bool {
	return c.w != nil && c.r != nil
}

// SetCookie sets an HTTP cookie on the response.
// Returns ErrNoHTTPContext if called from a WebSocket action.
func (c *Context) SetCookie(cookie *http.Cookie) error {
	if c.w == nil {
		return ErrNoHTTPContext
	}
	http.SetCookie(c.w, cookie)
	return nil
}

// DeleteCookie removes an HTTP cookie by setting MaxAge to -1.
// Returns ErrNoHTTPContext if called from a WebSocket action.
func (c *Context) DeleteCookie(name string) error {
	if c.w == nil {
		return ErrNoHTTPContext
	}
	http.SetCookie(c.w, &http.Cookie{
		Name:   name,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	return nil
}

// GetCookie retrieves an HTTP cookie from the request.
// Returns ErrNoHTTPContext if called from a WebSocket action.
func (c *Context) GetCookie(name string) (*http.Cookie, error) {
	if c.r == nil {
		return nil, ErrNoHTTPContext
	}
	return c.r.Cookie(name)
}

// Redirect sends an HTTP redirect response.
// Returns ErrNoHTTPContext if called from a WebSocket action.
// Returns ErrInvalidRedirectCode if code is not 3xx.
// Returns ErrInvalidRedirectURL if URL is not a valid relative path.
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
	if c.redirected != nil {
		*c.redirected = true
	}
	return nil
}

// WithHTTP returns a new Context with HTTP request/response.
func (c *Context) WithHTTP(w http.ResponseWriter, r *http.Request) *Context {
	newCtx := *c
	newCtx.w = w
	newCtx.r = r
	return &newCtx
}

// WithAction returns a new Context with the given action name.
func (c *Context) WithAction(action string) *Context {
	newCtx := *c
	newCtx.action = action
	return &newCtx
}

// WithData returns a new Context with the given data.
func (c *Context) WithData(data map[string]interface{}) *Context {
	newCtx := *c
	newCtx.data = newActionData(data)
	return &newCtx
}

// WithUploads returns a new Context with the given upload accessor.
func (c *Context) WithUploads(uploads UploadAccessor) *Context {
	newCtx := *c
	newCtx.uploads = uploads
	return &newCtx
}

// WithFormSchema returns a new Context with the given form validation schema.
func (c *Context) WithFormSchema(schema *FormSchema) *Context {
	newCtx := *c
	newCtx.formSchema = schema
	return &newCtx
}

// ValidateForm validates form data against HTML attributes inferred from the template.
// Uses validation rules extracted from HTML attributes like required, pattern, min, max,
// minlength, maxlength, and input type (email, url, number).
// Returns MultiError with field-level errors, or nil if all fields are valid.
//
// Note: the schema must be set via WithFormSchema(ExtractFormSchema(statics)).
// If no schema is set, returns nil (no validation). For production validation
// with complex rules, use BindAndValidate() with go-playground/validator tags.
//
// Known limitation: ExtractFormSchema merges all forms in a template into one
// schema. If your template has multiple forms, use BindAndValidate() instead.
func (c *Context) ValidateForm() error {
	if c.formSchema == nil {
		return nil
	}
	return c.formSchema.Validate(c.data.Raw())
}

// HasUploads checks if there are any uploads for the given field name.
func (c *Context) HasUploads(name string) bool {
	if c.uploads == nil {
		return false
	}
	return c.uploads.HasUploads(name)
}

// GetCompletedUploads returns all completed upload entries for the given field name.
func (c *Context) GetCompletedUploads(name string) []*uploadtypes.UploadEntry {
	if c.uploads == nil {
		return nil
	}
	return c.uploads.GetCompletedUploads(name)
}

// WithFlashSetter returns a new Context with the given flash setter.
func (c *Context) WithFlashSetter(setter FlashSetter) *Context {
	newCtx := *c
	newCtx.flashSetter = setter
	return &newCtx
}

// FlashOption configures optional behavior for SetFlash.
type FlashOption func(*flashConfig)

type flashConfig struct {
	expiry time.Duration // 0 = no auto-expiry, persist until ClearFlash
}

// FlashExpiry sets an auto-expiry duration for the flash message. After
// the duration elapses, the message is pruned from the flash store on
// the next render. Use this for transient feedback ("Settings saved")
// that doesn't need explicit acknowledgement. Messages without an
// expiry persist until explicitly cleared via ClearFlash.
//
// Example:
//
//	ctx.SetFlash("success", "Saved!", livetemplate.FlashExpiry(5*time.Second))
func FlashExpiry(d time.Duration) FlashOption {
	return func(c *flashConfig) { c.expiry = d }
}

// SetFlash sets a flash message that will be available in templates via .lvt.Flash(key).
// Flash messages are page-level notifications (success, info, warning, error).
// Unlike field errors, flash messages don't affect ResponseMetadata.Success.
//
// Flash messages persist until explicitly cleared via ClearFlash or until
// their optional expiry duration elapses. This matches the Phoenix LiveView
// model where flash is a separate namespace from assigns — background
// updates (TriggerAction / scan-loop Refresh) that modify state fields
// do not touch flash messages. Use ClearFlash in your action handlers
// when the user has acknowledged the message (e.g., after a navigation
// or a follow-up action).
//
// Migration note (v0.8 → v0.9): In earlier releases, flash was automatically
// cleared after each render (one-shot). Flash now persists on WebSocket
// connections until ClearFlash is explicitly called or FlashExpiry elapses.
// Existing handlers that relied on the auto-clear behavior and do not call
// ClearFlash will accumulate flash across re-renders. Add an explicit
// ClearFlash call (or FlashExpiry) to preserve one-shot behavior:
//
//	ctx.SetFlash("success", "Saved!", livetemplate.FlashExpiry(0)) // one-shot: use ClearFlash in next handler
//	ctx.SetFlash("success", "Saved!", livetemplate.FlashExpiry(5*time.Second)) // auto-expire after 5s
//
// Transport lifetime note: On WebSocket connections the flash store survives
// across renders — messages persist until ClearFlash is called. On HTTP
// connections (form submissions with progressive enhancement) the flash
// store is per-request, so flash is inherently one-shot regardless of
// whether ClearFlash is called.
//
// Redirect note: Flash set in a handler that also calls ctx.Redirect()
// does not survive the redirect — no flash cookie is written before the
// redirect response, so the message is lost. Use session-backed flash
// (or a query param) if you need flash to survive an HTTP redirect.
//
// Common keys: "success", "error", "info", "warning"
//
// Key conventions:
//   - Use simple, lowercase keys (e.g., "success", "error")
//   - Avoid keys containing colons or special characters
//   - Do not use keys starting with "_flash:" (reserved for internal use)
//
// Example:
//
//	ctx.SetFlash("success", "Changes saved successfully!")
//	ctx.SetFlash("error", "Failed to process your request.")
//	ctx.SetFlash("info", "Uploading...", livetemplate.FlashExpiry(3*time.Second))
func (c *Context) SetFlash(key, message string, opts ...FlashOption) {
	if c.flashSetter == nil {
		return
	}
	var cfg flashConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	c.flashSetter.setFlash(key, message, cfg.expiry)
}

// ClearFlash explicitly removes a flash message by key. Use this when
// the user has acknowledged the message (e.g., after navigating away
// or completing a follow-up action). Flash messages without an expiry
// persist until ClearFlash is called — the framework does not auto-clear
// them after render.
//
// Example:
//
//	func (c *MyController) Acknowledge(state MyState, ctx *livetemplate.Context) (MyState, error) {
//	    ctx.ClearFlash("error")
//	    return state, nil
//	}
func (c *Context) ClearFlash(key string) {
	if c.flashSetter != nil {
		c.flashSetter.clearFlashKey(key)
	}
}

// BroadcastAction queues a broadcast to all other connections in the same
// session group. The named action is dispatched on each receiving connection
// after the current action completes successfully.
//
// Each receiving connection runs the named action with its own per-connection
// state via DispatchWithState, preserving per-connection fields (e.g., CurrentUser).
//
// Broadcasts are deferred: they execute only after the triggering action returns
// without error. If the action returns an error, queued broadcasts are discarded.
//
// Constraints:
//   - Dispatched actions run with context.Background() — middleware-injected
//     request values (auth tokens, tracing spans) are not available.
//   - BroadcastAction calls inside a dispatched action are ignored to prevent
//     infinite broadcast storms.
//   - Context.With*() methods create shallow copies. Broadcasts queued after the
//     copy diverge (append allocates a new backing array once capacity is exceeded).
//
// Example:
//
//	func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
//	    c.mu.Lock()
//	    c.messages = append(c.messages, msg)
//	    c.mu.Unlock()
//	    state.Messages = c.copyMessages()
//	    ctx.BroadcastAction("RefreshMessages", nil)
//	    return state, nil
//	}
//
// MaxBroadcastsPerAction is the maximum number of BroadcastAction calls
// allowed per action invocation. Excess calls are dropped with an error log.
const MaxBroadcastsPerAction = 100

func (c *Context) BroadcastAction(action string, data map[string]interface{}) {
	if action == "" {
		return
	}
	if len(c.broadcasts) >= MaxBroadcastsPerAction {
		slog.Error("BroadcastAction cap reached, dropping",
			slog.String("action", action),
			slog.Int("limit", MaxBroadcastsPerAction))
		return
	}
	c.broadcasts = append(c.broadcasts, broadcastRequest{Action: action, Data: data})
}

// pendingBroadcasts returns and clears pending broadcast requests.
// Called by the mount handler after action dispatch to process deferred broadcasts.
func (c *Context) pendingBroadcasts() []broadcastRequest {
	b := c.broadcasts
	c.broadcasts = nil
	return b
}
