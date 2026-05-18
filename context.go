package livetemplate

import (
	"context"
	"fmt"
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
// FlashSetter is the interface implemented by connState for flash message
// operations. The methods are intentionally unexported to ensure flash
// messages are only managed through the Context.SetFlash() / ClearFlash()
// public API, maintaining consistent behavior and preventing direct message
// map manipulation.
type FlashSetter interface {
	setFlash(key, message string, expiry time.Duration)
	clearFlashKey(key string)
	clearAllFlash()
}

// ConnectKind classifies the lifecycle path that produced this Context.
// Use Context.IsInitialMount(), Context.IsNewConnect(), and
// Context.IsReconnect() instead of inspecting this value directly — those
// helpers encode the intended guard patterns.
type ConnectKind int

const (
	// ConnectKindAction is the default — a user-triggered action
	// (form submit, button click, server-dispatched broadcast).
	ConnectKindAction ConnectKind = iota

	// ConnectKindInitialMount indicates Mount was invoked on an HTTP GET
	// request (initial page load or page refresh, not an action POST).
	ConnectKindInitialMount

	// ConnectKindNewConnect indicates Mount/OnConnect was invoked on the
	// first WebSocket connection for this session group (no prior
	// persisted state was restored).
	ConnectKindNewConnect

	// ConnectKindReconnect indicates Mount/OnConnect was invoked on a
	// WebSocket connect where persisted state was restored from
	// SessionStore (true reconnect, or a returning user with a surviving
	// session).
	ConnectKindReconnect
)

// String returns a stable human-readable name for the ConnectKind, suitable
// for log lines and slog attributes. Unknown values fall back to
// "ConnectKind(<int>)" rather than panicking so future variants don't break
// existing log scrapers.
func (k ConnectKind) String() string {
	switch k {
	case ConnectKindAction:
		return "action"
	case ConnectKindInitialMount:
		return "initial_mount"
	case ConnectKindNewConnect:
		return "new_connect"
	case ConnectKindReconnect:
		return "reconnect"
	default:
		return fmt.Sprintf("ConnectKind(%d)", int(k))
	}
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
	groupID     string
	session     Session
	uploads     UploadAccessor
	flashSetter FlashSetter
	formSchema  *FormSchema
	broadcasts  []broadcastRequest
	topicSub    topicSubscriber
	topicPubs   []topicPublish
	connectKind ConnectKind

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

// GroupID returns the session-group key for this connection's identity. The
// Authenticator (GetSessionGroup) always sets it for a real connection; it is
// "" only for genuinely empty identity (a misimplemented custom Authenticator).
func (c *Context) GroupID() string {
	return c.groupID
}

// WithGroupID returns a new Context with the given session-group ID. It is a
// framework-internal wiring point (injected at every WithUserID site, and by
// custom-Authenticator extensions) — controller code should not call it; it
// reads identity via ctx.GroupID() / ctx.SelfTopic(). A wrong groupID here
// yields a subtly wrong SelfTopic().
func (c *Context) WithGroupID(groupID string) *Context {
	newCtx := *c
	newCtx.groupID = groupID
	return &newCtx
}

// SelfTopic returns this connection's identity-derived reserved topic, picking
// the most-specific available identity scope (proposal §1):
//
//   - Authenticated (UserID != "") → lvt:user:<UserID>. Spans every device and
//     tab of that user regardless of the Authenticator's groupID strategy —
//     this is what makes phone→desktop self-sync work where BroadcastAction
//     cannot.
//   - Anonymous (UserID == "", GroupID != "") → lvt:session:<GroupID>. Spans
//     the tabs of that one browser session. A custom Authenticator that
//     authenticates a session but leaves UserID empty silently degrades
//     multi-device to multi-tab; that is a valid anonymous-shaped topic, not an
//     error, so it is slog.Debug (not Warn) — legitimate for anonymous traffic.
//
// Invariant: both identity fields empty is a programmer error (a misimplemented
// custom Authenticator). It is fail-closed + loud — slog.Error and return ""
// (Subscribe("")/Publish("") are then rejected) — but logged, never a panic
// (a bad Authenticator must not crash the server). BasicAuthenticator /
// AnonymousAuthenticator never hit this.
//
// SelfTopic() is ACL-exempt and reserved-namespace; the canonical idiom is
// `_ = ctx.Subscribe(ctx.SelfTopic())` so the empty-identity case MUST be loud
// here, independent of whether the caller inspects the returned error.
func (c *Context) SelfTopic() string {
	if c.userID != "" {
		return UserTopic(c.userID)
	}
	if c.groupID != "" {
		slog.Debug("SelfTopic resolved to session scope; UserID empty", slog.String("groupID", c.groupID))
		return sessionTopic(c.groupID)
	}
	slog.Error("SelfTopic called with empty UserID and GroupID")
	return ""
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

// WithConnectKind returns a new Context with the given connect classification.
// The framework sets this at lifecycle-Context construction time; production
// controllers typically read the classification via IsInitialMount,
// IsNewConnect, and IsReconnect rather than calling this builder directly.
// Tests that need to construct a Context with a specific kind may use it.
func (c *Context) WithConnectKind(kind ConnectKind) *Context {
	newCtx := *c
	newCtx.connectKind = kind
	return &newCtx
}

// IsInitialMount reports whether this Mount call is from an initial HTTP GET
// (page load or refresh). Returns false for HTTP POST actions, WebSocket
// new-connects, and WebSocket reconnects. Use this to guard one-time setup
// that should only run on a page load — for example, spawning a background
// goroutine, kicking off lazy data fetches, or recording analytics:
//
//	func (c *MyController) Mount(state State, ctx *livetemplate.Context) (State, error) {
//	    if ctx.IsInitialMount() {
//	        state.Loading = true
//	        state.Data = ""
//	    }
//	    return state, nil
//	}
//
// This replaces the older `ctx.Action() == ""` idiom, which also returned
// true for WebSocket connects/reconnects and internal POST navigations.
func (c *Context) IsInitialMount() bool {
	return c.connectKind == ConnectKindInitialMount
}

// IsReconnect reports whether previously persisted state was restored from
// SessionStore for this Mount/OnConnect call — that is, "state was restored",
// not "a prior WebSocket connection existed." Use this when a pattern needs
// to recover background-pushed state across network blips, for example
// re-announcing presence or skipping a re-fetch the previous connection
// already completed:
//
//	func (c *ChatController) OnConnect(state State, ctx *livetemplate.Context) (State, error) {
//	    if ctx.IsReconnect() {
//	        state.SystemMessages = append(state.SystemMessages, "[reconnected]")
//	    }
//	    return state, nil
//	}
//
// Semantics — IsReconnect is true whenever the framework's SessionStore
// returned previously persisted state for this group, which includes:
//
//   - A true WebSocket reconnect after a network blip.
//   - A returning user with a surviving SessionStore entry (e.g. opened a
//     bookmark after the prior tab was closed).
//   - The very first WebSocket connect that follows an HTTP GET in the same
//     session: the HTTP-path Mount persists state, then the WS connect
//     restores it. This is detectable in user controllers and may be
//     unexpected; pair this helper with [IsNewConnect] when you need to
//     distinguish a brand-new WS session from one that has any persisted
//     history.
//
// IsReconnect does NOT track whether a prior WebSocket connection actually
// existed — distinguishing that would require additional bookkeeping in
// SessionStore.
//
// During a __navigate__ re-mount, this value reflects the underlying WS
// connect-kind, not the navigate event — see docs/references/navigate.md
// for the full preservation rules.
func (c *Context) IsReconnect() bool {
	return c.connectKind == ConnectKindReconnect
}

// IsNewConnect reports whether this Mount/OnConnect call is the first
// WebSocket connection for this session group with no prior persisted state.
// Mutually exclusive with IsReconnect (which fires when state was restored)
// and IsInitialMount (which fires on the HTTP-path Mount).
//
// Use this to gate one-time-per-WebSocket-session setup that must NOT run
// again on a reconnect:
//
//	func (c *ChatController) OnConnect(state State, ctx *livetemplate.Context) (State, error) {
//	    if ctx.IsNewConnect() {
//	        c.metrics.NewSessions.Inc()
//	    }
//	    return state, nil
//	}
//
// During a __navigate__ re-mount, this value reflects the underlying WS
// connect-kind, not the navigate event — see docs/references/navigate.md
// for the full preservation rules.
func (c *Context) IsNewConnect() bool {
	return c.connectKind == ConnectKindNewConnect
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
// the duration elapses, the message is pruned on the next render that walks
// flash state — there is no background timer. The render that the user
// triggers post-expiry is rendered without the expired flash. Use this for
// transient feedback ("Settings saved") that doesn't need explicit
// acknowledgement. Messages without an expiry persist until explicitly
// cleared via ClearFlash.
//
// A duration of 0 or less disables auto-expiry — the message behaves as if
// FlashExpiry were not provided and persists until ClearFlash is called.
//
// Note: FlashExpiry has no observable effect on HTTP connections — HTTP
// flash is inherently one-shot (per-request connSt is GC'd after the
// handler returns) regardless of the expiry duration set here.
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
// ClearFlash will accumulate flash across re-renders. To restore one-shot
// behavior, either call ClearFlash in the next handler, or use FlashExpiry:
//
//	// Option A: persist and clear explicitly in the follow-up handler
//	ctx.SetFlash("success", "Saved!") // call ctx.ClearFlash("success") in the next handler
//	// Option B: auto-expire after a fixed duration (e.g., after 5 seconds)
//	ctx.SetFlash("success", "Saved!", livetemplate.FlashExpiry(5*time.Second))
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

// ClearAllFlash atomically clears every flash message on this connection.
// Use this when navigating to a context where prior flash messages are no
// longer relevant — for example, on logout or before redirecting to a
// section where a stale "Saved!" notification from a different page would
// be confusing.
//
// Mirrors ClearFlash but operates on all keys at once. Field-validation
// errors (set via ctx.SetError) are unaffected.
//
// Example:
//
//	func (c *AuthController) Logout(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
//	    ctx.ClearAllFlash()
//	    state.User = nil
//	    return state, nil
//	}
func (c *Context) ClearAllFlash() {
	if c.flashSetter != nil {
		c.flashSetter.clearAllFlash()
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
