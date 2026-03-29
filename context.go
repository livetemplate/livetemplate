package livetemplate

import (
	"context"
	"log/slog"
	"net/http"

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
// The setFlash method is intentionally unexported to ensure flash messages
// are only set through the Context.SetFlash() public API, maintaining
// consistent behavior and preventing direct message map manipulation.
type FlashSetter interface {
	setFlash(key, message string)
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

// SetFlash sets a flash message that will be available in templates via .lvt.Flash(key).
// Flash messages are page-level notifications (success, info, warning, error).
// Unlike field errors, flash messages don't affect ResponseMetadata.Success.
// Flash messages are cleared after each render, so they appear only once.
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
func (c *Context) SetFlash(key, message string) {
	if c.flashSetter != nil {
		c.flashSetter.setFlash(key, message)
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
//   - Silently ignored when WithSharedState() is enabled (auto-broadcast handles sync).
//   - Broadcasts are accumulated on the Context value. Context.With*() methods create
//     shallow copies — broadcasts queued before the copy are visible to both.
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
func (c *Context) BroadcastAction(action string, data map[string]interface{}) {
	if action == "" {
		return
	}
	const maxBroadcasts = 100
	if len(c.broadcasts) >= maxBroadcasts {
		slog.Warn("BroadcastAction cap reached, dropping",
			slog.String("action", action),
			slog.Int("limit", maxBroadcasts))
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
