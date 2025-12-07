package livetemplate

import (
	"context"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// UploadAccessor provides access to upload entries during action handling.
type UploadAccessor interface {
	HasUploads(name string) bool
	GetCompletedUploads(name string) []*uploadtypes.UploadEntry
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
	action  string
	data    *ActionData
	userID  string
	session Session
	uploads UploadAccessor

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
