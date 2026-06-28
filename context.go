package livetemplate

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"path"
	"strings"
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
	// (form submit, button click, peer-fan-out via ctx.Publish).
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
	submitter   string // SubmitEvent.submitter.name; distinct from action under lvt-on:submit routing
	data        *ActionData
	userID      string
	groupID     string
	session     Session
	uploads     UploadAccessor
	flashSetter FlashSetter
	formSchema  *FormSchema
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

// withSubmitter returns a copy with the form submitter set. Unexported: the
// submitter is framework-populated from the client's SubmitEvent at dispatch
// time, not a value applications set when building a Context (read it via
// Submitter()).
func (c *Context) withSubmitter(name string) *Context {
	newCtx := *c
	newCtx.submitter = name
	return &newCtx
}

// Submitter returns the name of the control that submitted the form — the
// clicked submit button — across all tiers: the client sends it explicitly on
// the WebSocket and HTTP-fetch paths, and the no-JS path resolves it from the
// submit button's form field. It is "" when no form submitter applies (e.g. a
// non-form action, or a no-JS submit button that carried a value). Under
// lvt-on:submit routing Action is the handler while Submitter is the button. Use
// it to branch custom validation in BindAndValidate flows, mirroring how
// ValidateForm honors formnovalidate.
func (c *Context) Submitter() string {
	return c.submitter
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
//     this is what makes phone→desktop self-sync work across devices, not just
//     same-session tabs.
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
//
// The target may be an absolute-path reference ("/dashboard") or a relative
// reference — including a bare segment ("dashboard"), dot forms (".", "./settings",
// "../list"), and the empty string. Relative references are emitted
// as-is in the Location header so the browser resolves them against its own
// (un-stripped) request URL — this lets a recipe mounted behind
// http.StripPrefix redirect back to its own mount without knowing the prefix.
// The empty string means "reload self": Redirect("", http.StatusSeeOther) is
// the canonical POST-Redirect-GET target for a recipe's own mount. This assumes
// a trailing-slash mount — the canonical http.StripPrefix("/apps/login/", …)
// pattern — because "" resolves to "./", the current directory. An exact-match
// mount without a trailing slash (http.StripPrefix("/apps/login", …) serving
// /apps/login) would resolve "./" to the parent path; mount with a trailing
// slash to use the reload-self form.
//
// Returns ErrNoHTTPContext if called from a WebSocket action.
// Returns ErrInvalidRedirectCode if code is not 3xx.
// Returns ErrInvalidRedirectURL if the target carries a scheme or host, or is a
// protocol-relative URL (open-redirect guards — see isValidRedirectURL).
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

	if strings.HasPrefix(url, "/") {
		// Absolute-path target: unchanged behaviour. http.Redirect does not
		// resolve absolute paths, so the stripped request path is irrelevant.
		http.Redirect(c.w, c.r, url, code)
	} else {
		// Relative target: emit the reference RAW so the client resolves it
		// against its effective (un-stripped) request URI. http.Redirect would
		// resolve it server-side against the http.StripPrefix-stripped path —
		// the bug we're avoiding. "" means "reload self": translate to
		// "./<last-segment>" so the Location is non-empty (an empty Location
		// breaks clients) and resolves back to the current URL.
		target := url
		if target == "" {
			target = relativeSelfReference(c.r)
		}
		writeRelativeRedirect(c.w, c.r, target, code)
	}

	if c.redirected != nil {
		*c.redirected = true
	}
	return nil
}

// relativeSelfReference returns "./<last-segment>", a relative reference a
// browser resolves back to the request's own URL under a trailing-slash mount —
// the StripPrefix-safe "reload self" target for a handler that cannot see the
// prefix http.StripPrefix removed from r.URL.Path. Under an exact-match mount
// without a trailing slash a browser resolves "./" to the parent path; see
// Context.Redirect's doc for that caveat.
func relativeSelfReference(r *http.Request) string {
	_, last := path.Split(r.URL.EscapedPath())
	return "./" + last
}

// writeRelativeRedirect emits target RAW in the Location header so the client
// resolves it against its effective (un-stripped) request URI — http.Redirect
// would instead resolve it server-side against the http.StripPrefix-stripped
// path, the bug this avoids. It mirrors http.Redirect's method-aware tail.
func writeRelativeRedirect(w http.ResponseWriter, r *http.Request, target string, code int) {
	h := w.Header()
	_, hadCT := h["Content-Type"]
	h.Set("Location", hexEscapeNonASCII(target))
	if !hadCT && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		h.Set("Content-Type", "text/html; charset=utf-8")
	}
	w.WriteHeader(code)
	if !hadCT && r.Method == http.MethodGet {
		if _, err := fmt.Fprintf(w, "<a href=\"%s\">%s</a>.\n", html.EscapeString(target), http.StatusText(code)); err != nil {
			slog.Warn("Failed to write redirect body",
				slog.String("component", "context"),
				slog.Any("error", err))
		}
	}
}

// hexEscapeNonASCII percent-encodes bytes >= 0x80 so a relative Location value
// stays within HTTP's ASCII header constraint. It mirrors the unexported
// net/http helper http.Redirect applies to the absolute-path branch, and only
// touches non-ASCII bytes — ASCII path delimiters ("/", ".", "?") are preserved,
// unlike url.PathEscape which would mangle them.
func hexEscapeNonASCII(s string) string {
	const upperhex = "0123456789ABCDEF"
	hasNonASCII := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			hasNonASCII = true
			break
		}
	}
	if !hasNonASCII {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 0x80 {
			b = append(b, '%', upperhex[c>>4], upperhex[c&0x0f])
		} else {
			b = append(b, c)
		}
	}
	return string(b)
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
// Validation is skipped when the form was submitted by a control carrying the
// formnovalidate attribute (e.g. <button name="save-draft" formnovalidate>) —
// matched by the submitter's name. This is a client-controlled convenience for
// draft/save-without-validation flows, NOT a security boundary: enforce
// server-authoritative rules unconditionally where it matters.
//
// Known limitation: ExtractFormSchema merges all forms in a template into one
// schema. If your template has multiple forms, use BindAndValidate() instead.
func (c *Context) ValidateForm() error {
	if c.formSchema == nil {
		return nil
	}
	// c.submitter is the clicked button's name on every form-submit tier — the
	// client sends it explicitly (WS / HTTP-fetch), and the no-JS button-name
	// path sets it from the empty-value submit field (parseURLEncodedForm). So a
	// single lookup suffices; "" (no submitter) is simply not in the set.
	if c.formSchema.NoValidateSubmitters[c.submitter] {
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
