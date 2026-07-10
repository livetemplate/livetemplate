// Package livetemplate provides a library for building real-time, reactive web applications
// in Go with minimal code. It uses tree-based DOM diffing to send only what changed over
// WebSocket or HTTP, inspired by Phoenix LiveView.
//
// # Quick Start
//
// Define your application state as a Go struct with methods for each action:
//
//	type Counter struct {
//	    Count int
//	}
//
//	func (c *Counter) Increment(ctx *livetemplate.ActionContext) error {
//	    c.Count++
//	    return nil
//	}
//
//	func (c *Counter) Decrement(ctx *livetemplate.ActionContext) error {
//	    c.Count--
//	    return nil
//	}
//
// Actions are automatically dispatched to methods matching the action name
// (e.g., "increment" → Increment, "add_item" → AddItem).
//
// Create a template with `lvt-on:{event}` attributes for event binding:
//
//	<!-- counter.tmpl -->
//	<h1>Counter: {{.Count}}</h1>
//	<button lvt-on:click="increment">+</button>
//	<button lvt-on:click="decrement">-</button>
//
// Wire it up in your main function:
//
//	func main() {
//	    counter := &Counter{Count: 0}
//	    tmpl := livetemplate.New("counter")
//	    http.Handle("/", tmpl.Handle(counter))
//	    http.ListenAndServe(":8080", nil)
//	}
//
// # How It Works
//
// LiveTemplate separates static and dynamic content in templates:
//
//   - Static content (HTML structure, unchanging text) is sent once and cached client-side
//   - Dynamic content (data values) is sent on every update as a minimal tree diff
//   - This achieves 50-90% bandwidth reduction compared to sending full HTML
//
// The client library (TypeScript) handles WebSocket communication, event delegation,
// and applying DOM updates efficiently.
//
// # Tree-Based Updates
//
// Templates are parsed into a tree structure that separates statics and dynamics:
//
//	{
//	    "s": ["<div>Count: ", "</div>"],  // Statics (cached)
//	    "0": "42"                          // Dynamic value
//	}
//
// Subsequent updates only send changed dynamic values:
//
//	{
//	    "0": "43"  // Only the changed value
//	}
//
// # Key Types
//
//   - Template: Manages template parsing, execution, and update generation
//   - Store: Interface for application state and action handlers
//   - ActionContext: Provides action data and utilities in Change() method
//   - ActionData: Type-safe data extraction and validation
//   - Broadcaster: Share state updates across instances via Redis Pub/Sub
//   - SessionStore: Per-session state management
//
// # Advanced Features
//
//   - Multi-store pattern: Namespace multiple stores in one template
//   - Peer fan-out: Real-time updates to all connected clients via ctx.Publish
//   - Server-side validation: Automatic error handling with go-playground/validator
//   - Form lifecycle events: Client-side hooks for pending, success, error, done
//   - Focus preservation: Maintains input focus and scroll position during updates
//
// For complete documentation, see https://github.com/livetemplate/livetemplate
package livetemplate

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/context"
	"github.com/livetemplate/livetemplate/internal/diff"
	"github.com/livetemplate/livetemplate/internal/discovery"
	"github.com/livetemplate/livetemplate/internal/observe"
	"github.com/livetemplate/livetemplate/internal/parse"
	"github.com/livetemplate/livetemplate/internal/render"
	"github.com/livetemplate/livetemplate/internal/send"
	"github.com/livetemplate/livetemplate/internal/session"
	uploadtypes "github.com/livetemplate/livetemplate/internal/uploadtypes"
	"github.com/livetemplate/livetemplate/pubsub"
)

// =============================================================================
// Internal Type Aliases (Not Part of Public API)
// =============================================================================
//
// IMPORTANT: These type aliases exist in the main package (not internal/) to support
// Template's implementation and same-package test files, but are NOT part of the stable
// public API. External users should NOT depend on these types - they may change without notice.
//
// Why lowercase (unexported)?
// - These are true internal implementation details
// - Only Template methods and same-package tests need access
// - External packages cannot and should not use them
//
// These aliases exist to:
// - Provide convenient access to internal types within this package
// - Maintain clean imports without exposing internal packages publicly
// - Support backward compatibility for internal test code

// treeNode is an internal alias for build.TreeNode.
// Used internally by Template for tree caching and comparison.
type treeNode = build.TreeNode

// =============================================================================
// Configuration
// =============================================================================

// Config holds template configuration options
type Config struct {
	Upgrader               WSUpgrader
	SessionStore           SessionStore
	Authenticator          Authenticator      // User authentication and session grouping
	PubSubBroadcaster      pubsub.Broadcaster // Optional: for distributed broadcasting across instances
	AllowedOrigins         []string           // Allowed WebSocket origins (empty = allow all in dev, restrict in prod)
	WebSocketDisabled      bool
	LoadingDisabled        bool                                // Disables automatic loading indicator on page load
	TemplateFiles          []string                            // If set, overrides auto-discovery
	TemplateFS             fs.FS                               // If set (WithParseFS), templates are parsed from this fs.FS; takes precedence over TemplateFiles and auto-discovery
	TemplateFSPatterns     []string                            // Glob patterns matched against TemplateFS
	TemplateBaseDir        string                              // Base directory for template auto-discovery (default: directory of calling code via runtime.Caller)
	IgnoreTemplateDirs     []string                            // Additional directories to ignore during auto-discovery
	DevMode                bool                                // Development mode - use local client library instead of CDN
	MaxConnections         int64                               // Maximum total connections (0 = unlimited)
	MaxConnectionsPerGroup int64                               // Maximum connections per group (0 = unlimited)
	MessageRateLimit       float64                             // Messages per second per connection (0 = unlimited, default 10)
	MessageRateBurst       int                                 // Burst capacity for rate limiting (default 20)
	CookieMaxAge           time.Duration                       // Session cookie max age (default: 1 year)
	UploadConfigs          map[string]uploadtypes.UploadConfig // Upload field configurations
	WebSocketBufferSize    int                                 // WebSocket send buffer size per connection (default: 50)
	ComponentTemplates     []*TemplateSet                      // Component library templates (parsed before project templates)
	ProgressiveEnhancement bool                                // Enable non-JS form submission support with PRG pattern (default: true)
	TrustForwardedHeaders  bool                                // Trust forwarded scheme headers (X-Forwarded-Proto, RFC 7239 Forwarded) for scheme detection (default: true)
	DispatchBufferSize     int                                 // Broadcast dispatch channel buffer per connection (default: 16)

	// TopicACL gates every developer (non-lvt:) topic Subscribe. nil unless
	// WithTopicACL is set. Mutually exclusive with OpenTopics (enforced at New()).
	TopicACL TopicACLFunc
	// OpenTopics (WithOpenTopics()) permits every topic Subscribe. Mutually
	// exclusive with TopicACL (enforced at New()). Neither set = deny-all:
	// every developer-topic Subscribe returns ErrTopicForbidden; only
	// ctx.SelfTopic() is ACL-exempt.
	OpenTopics bool
}

// =============================================================================
// Component Template Registration
// =============================================================================

// TemplateSet represents a collection of embedded templates from a component library.
// Components create TemplateSet instances to expose their templates for registration.
//
// Example usage in a component library:
//
//	package dropdown
//
//	import "embed"
//
//	//go:embed templates/*.tmpl
//	var templateFS embed.FS
//
//	func Templates() *livetemplate.TemplateSet {
//	    return &livetemplate.TemplateSet{
//	        FS:        templateFS,
//	        Pattern:   "templates/*.tmpl",
//	        Namespace: "dropdown",
//	    }
//	}
//
// Example usage in main.go:
//
//	tmpl, err := livetemplate.New("app",
//	    livetemplate.WithComponentTemplates(dropdown.Templates(), tabs.Templates()),
//	)
type TemplateSet struct {
	// FS is the embedded filesystem containing the template files.
	FS embed.FS

	// Pattern is the glob pattern for matching template files within FS.
	// Examples: "templates/*.tmpl", "*.tmpl"
	Pattern string

	// Namespace identifies the component type for this template set.
	// Used for documentation and debugging purposes.
	// Example: "dropdown" for templates like "lvt:dropdown:searchable:v1"
	Namespace string

	// Funcs provides additional template functions for this component.
	// These are merged with the base template functions when parsing.
	Funcs template.FuncMap
}

// Template represents a live template with caching and tree-based optimization capabilities.
// It provides an API similar to html/template.Template but with additional ExecuteUpdates method
// for generating tree-based updates that can be efficiently transmitted to clients.
type Template struct {
	name                   string
	templateStr            string
	tmpl                   *template.Template
	wrapperID              string
	funcs                  template.FuncMap
	mu                     sync.RWMutex // Protects mutable state fields below
	lastHTML               string
	lastTree               *treeNode // Store previous tree segments for comparison
	hasInitialTree         bool
	config                 Config              // Template configuration
	uploadRegistry         interface{}         // Upload registry for this connection (*upload.Registry)
	cachedParseTemplate    *parse.Template     // Cached AST to avoid re-parsing on every render
	cachedBodyContent      string              // Cached result of ExtractTemplateBodyContent(t.templateStr)
	cachedBodyContentValid bool                // Whether cachedBodyContent has been computed (empty string is valid)
	formSchema             *FormSchema         // Cached schema extracted from templateStr; nil if no rules
	wiredActions           map[string]struct{} // Cached set of client-wired action names (form/button name=, lvt-on:) extracted from templateStr; immutable after parse; drives the Publish symmetry-collision warning
	wiredCollisionWarned   *sync.Map           // action -> struct{}: dedups the Publish symmetry-collision slog.Warn to once per action name; shared by pointer across per-session clones so the warning is app-global, not per-connection
}

// Funcs registers a template.FuncMap that will be applied to all template parsing and execution.
func (t *Template) Funcs(funcMap template.FuncMap) *Template {
	if len(funcMap) == 0 {
		return t
	}

	if t.funcs == nil {
		t.funcs = make(template.FuncMap, len(funcMap))
	}

	for name, fn := range funcMap {
		t.funcs[name] = fn
	}

	// Update the existing parsed template if one is available.
	t.mu.Lock()
	if t.tmpl != nil {
		t.tmpl = t.tmpl.Funcs(t.funcs)
	}
	t.cachedParseTemplate = nil // Invalidate cached AST since funcMap changed
	t.mu.Unlock()

	return t
}

// UpdateResponse wraps a tree update with metadata for form lifecycle.
// Tree is an opaque type representing the update payload - the client library handles this automatically.
// This is an alias for internal/send.UpdateResponse for backward compatibility.
type UpdateResponse = send.UpdateResponse

// ResponseMetadata contains information about the action that generated the update.
// This is an alias for internal/send.ResponseMetadata for backward compatibility.
type ResponseMetadata = send.ResponseMetadata

// Option is a functional option for configuring a Template
type Option func(*Config)

// TopicACLFunc decides whether a connection may subscribe to a topic. It is
// called once per ctx.Subscribe with the literal subscribed name — the
// wildcard pattern "room/*", never a concrete match like "room/42" (proposal
// §2/§3) — the resolved userID ("" for anonymous), and the request that
// established the connection.
//
// The request is the WS upgrade request on the WebSocket path and a plain GET
// on the HTTP page-render path; because ctx.Subscribe runs the ACL eagerly
// even on a plain GET, distinguish the two by the Upgrade header, NOT r.Method
// (a WebSocket handshake is itself an HTTP GET — an r.Method=="GET" early
// return would silently disable the ACL on the real WS connection).
//
// Returning (false, _) rejects the subscription with ErrTopicForbidden.
//
// Because Subscribe runs the ACL eagerly even on a plain HTTP GET, returning
// false for a topic a controller subscribes in Mount causes that GET to
// surface the error as an HTTP 500 to the browser (Mount errors map to 500),
// not a WS-time rejection. If that is undesirable, gate the Mount Subscribe
// with ctx.IsInitialMount() / defer to WS connect via the Upgrade-header
// check above.
type TopicACLFunc func(topic string, userID string, r *http.Request) (allowed bool, err error)

// WithTopicACL sets the topic-subscription ACL hook (proposal §3). Called once
// per ctx.Subscribe with the literal subscribed name. ctx.SelfTopic() is the
// only ACL-exempt topic. Mutually exclusive with WithOpenTopics — setting both
// is a hard error returned from New(), order-independent.
//
// Footgun: the ACL runs eagerly even on a plain HTTP GET, so if a controller
// calls ctx.Subscribe in Mount and this hook can deny it, the GET surfaces an
// HTTP 500 (Mount errors map to 500), not a 403. If your ACL may deny during
// Mount, gate the Subscribe with ctx.IsInitialMount() / defer the real check
// to WS connect via the Upgrade-header pattern (see TopicACLFunc).
func WithTopicACL(fn TopicACLFunc) Option {
	return func(c *Config) {
		c.TopicACL = fn
	}
}

// WithOpenTopics opts every topic into being publicly subscribable, disabling
// the deny-all default. It is deliberately explicit and self-documenting: the
// docs-site scaffolds are copy-pasted into user projects, so an allow-all
// default would teach an insecure idiom. Mutually exclusive with WithTopicACL
// — setting both is a hard error returned from New(), order-independent.
func WithOpenTopics() Option {
	return func(c *Config) {
		c.OpenTopics = true
	}
}

// WithParseFiles specifies template files to parse, overriding auto-discovery
func WithParseFiles(files ...string) Option {
	return func(c *Config) {
		c.TemplateFiles = files
	}
}

// WithParseFS parses templates from an fs.FS (e.g. an embed.FS) matching the
// given glob patterns, instead of reading files from disk. This lets an app ship
// its templates embedded in the binary without staging them to a temp directory
// first.
//
// It takes precedence over WithParseFiles and auto-discovery. The first matched
// file is the main template; the rest are parsed into the same set for
// composition (same semantics as WithParseFiles).
//
// Example:
//
//	//go:embed templates
//	var tmplFS embed.FS
//
//	tmpl := livetemplate.Must(livetemplate.New("app",
//	    livetemplate.WithParseFS(tmplFS, "templates/*.tmpl")))
func WithParseFS(fsys fs.FS, patterns ...string) Option {
	return func(c *Config) {
		c.TemplateFS = fsys
		c.TemplateFSPatterns = patterns
	}
}

// WithTemplateBaseDir sets the base directory for template auto-discovery.
// This overrides the default runtime.Caller detection. Useful when running
// via 'go run' or when templates are in a non-standard location.
func WithTemplateBaseDir(dir string) Option {
	return func(c *Config) {
		c.TemplateBaseDir = dir
	}
}

// WithUpgrader sets a custom WebSocket upgrader.
func WithUpgrader(upgrader WSUpgrader) Option {
	return func(c *Config) {
		c.Upgrader = upgrader
	}
}

// WithSessionStore sets a custom session store for HTTP requests
func WithSessionStore(store SessionStore) Option {
	return func(c *Config) {
		c.SessionStore = store
	}
}

// WithWebSocketDisabled disables WebSocket support, forcing HTTP-only mode
func WithWebSocketDisabled() Option {
	return func(c *Config) {
		c.WebSocketDisabled = true
	}
}

// WithLoadingDisabled disables the automatic loading indicator shown during page initialization
func WithLoadingDisabled() Option {
	return func(c *Config) {
		c.LoadingDisabled = true
	}
}

// WithDevMode enables development mode - uses local client library instead of CDN
func WithDevMode(enabled bool) Option {
	return func(c *Config) {
		c.DevMode = enabled
	}
}

// WithAuthenticator sets a custom authenticator for user identification and session grouping.
//
// The authenticator determines:
//   - Who is the user? (userID via Identify)
//   - Which session group should they join? (groupID via GetSessionGroup)
//
// Default: AnonymousAuthenticator (browser-based session grouping)
//
// Example with BasicAuthenticator:
//
//	auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
//	    return db.ValidateUser(username, password)
//	})
//	tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
//
// Example with custom JWT authenticator:
//
//	tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(myJWTAuth))
func WithAuthenticator(auth Authenticator) Option {
	return func(c *Config) {
		c.Authenticator = auth
	}
}

// WithAllowedOrigins sets the allowed WebSocket origins for CORS protection.
//
// When set, WebSocket upgrade requests will be validated against this list.
// Requests from origins not in the list will be rejected with 403 Forbidden.
//
// If empty (default):
//   - Development: All origins allowed (permissive for local dev)
//   - Production: Consider setting explicitly for security
//
// Example for production:
//
//	tmpl := livetemplate.New("app",
//	    livetemplate.WithAllowedOrigins([]string{
//	        "https://yourdomain.com",
//	        "https://www.yourdomain.com",
//	    }))
//
// Security note: Always set this in production to prevent CSRF attacks via WebSocket.
//
// When AllowedOrigins is not set, same-origin detection derives the request
// scheme from proxy forwarding headers (X-Forwarded-Proto, falling back to the
// RFC 7239 Forwarded header) if present and trusted, or from r.TLS otherwise. By
// default these forwarded headers are trusted (see [WithTrustForwardedHeaders]).
// If the server is directly reachable by clients without a proxy, either set
// WithTrustForwardedHeaders(false) to ignore forwarded headers, or use
// WithAllowedOrigins to explicitly list trusted origins.
func WithAllowedOrigins(origins []string) Option {
	return func(c *Config) {
		c.AllowedOrigins = origins
	}
}

// WithTrustForwardedHeaders controls whether proxy forwarding headers are
// trusted for scheme detection in same-origin WebSocket checks.
//
// Default: true (backward compatible). When true, the origin checker reads
// X-Forwarded-Proto (falling back to the RFC 7239 Forwarded header) to determine
// whether the original client connection used HTTP or HTTPS. This is safe when
// the server is behind a reverse proxy that sets/overwrites these headers.
//
// Set to false if the server is directly reachable by clients (no proxy) to
// prevent clients from forging the headers. In this case, scheme detection falls
// back to r.TLS (non-nil = HTTPS, nil = HTTP).
//
// This option only affects the default same-origin check. It has no effect when
// WithAllowedOrigins is set (explicit origins take priority).
func WithTrustForwardedHeaders(trust bool) Option {
	return func(c *Config) {
		c.TrustForwardedHeaders = trust
	}
}

// WithPermissiveOriginCheck disables origin checking for WebSocket connections.
//
// WARNING: This allows connections from any origin and should ONLY be used in:
//   - Local development environments
//   - Testing scenarios
//   - Specific use cases where CSRF protection is handled externally
//
// In production, use WithAllowedOrigins() instead to specify trusted origins.
//
// Example:
//
//	// Development only - DO NOT use in production
//	tmpl := livetemplate.New("app",
//	    livetemplate.WithDevMode(true),
//	    livetemplate.WithPermissiveOriginCheck(),
//	)
func WithPermissiveOriginCheck() Option {
	return func(c *Config) {
		if gu, ok := c.Upgrader.(*GorillaUpgrader); ok {
			gu.SetCheckOrigin(func(r *http.Request) bool {
				return true
			})
		} else {
			slog.Warn("WithPermissiveOriginCheck has no effect on non-Gorilla WSUpgrader implementations")
		}
	}
}

// WithIgnoreTemplateDirs adds directories to ignore during template auto-discovery.
// This is useful to skip directories containing generator templates or other non-runtime templates.
//
// Example:
//
//	tmpl := livetemplate.New("app", livetemplate.WithIgnoreTemplateDirs("generators", "scaffolds"))
func WithIgnoreTemplateDirs(dirs ...string) Option {
	return func(c *Config) {
		c.IgnoreTemplateDirs = append(c.IgnoreTemplateDirs, dirs...)
	}
}

// WithMaxConnections sets the maximum number of concurrent connections.
// 0 (default) means unlimited.
func WithMaxConnections(max int64) Option {
	return func(c *Config) {
		c.MaxConnections = max
	}
}

// WithMaxConnectionsPerGroup sets the maximum number of connections per session group.
// 0 (default) means unlimited. Prevents single users from exhausting connection limits.
func WithMaxConnectionsPerGroup(max int64) Option {
	return func(c *Config) {
		c.MaxConnectionsPerGroup = max
	}
}

// WithWebSocketBufferSize sets the send buffer size per WebSocket connection.
//
// The buffer queues messages for async delivery. Larger buffers handle burst traffic better
// but use more memory. Smaller buffers use less memory but may close slow clients more aggressively.
//
// Default: 50 messages per connection
//   - Memory per connection: ~50KB (assuming 1KB avg message size)
//   - Memory for 100 connections: ~5MB
//
// Recommended values:
//   - Low traffic / memory constrained: 10-25
//   - Normal traffic: 50 (default)
//   - High traffic / burst heavy: 100-1000
//
// Environment variable override: LVT_WS_BUFFER_SIZE
//
// Example:
//
//	// High-throughput application
//	tmpl := livetemplate.New("app", livetemplate.WithWebSocketBufferSize(100))
//
//	// Memory-constrained environment
//	tmpl := livetemplate.New("app", livetemplate.WithWebSocketBufferSize(10))
func WithWebSocketBufferSize(size int) Option {
	return func(c *Config) {
		if size <= 0 {
			slog.Warn("Invalid WebSocketBufferSize, using default", slog.Int("value", size), slog.Int("default", 50))
			c.WebSocketBufferSize = 50
		} else {
			c.WebSocketBufferSize = size
		}
	}
}

// WithDispatchBufferSize sets the buffer size for the peer-fan-out dispatch channel
// per WebSocket connection. This is separate from the WebSocket send buffer
// (WithWebSocketBufferSize) because dispatch requests are less frequent.
// Default: 16. Increase for apps with high ctx.Publish fan-out.
func WithDispatchBufferSize(size int) Option {
	return func(c *Config) {
		c.DispatchBufferSize = size
	}
}

// WithWebSocketCompression enables permessage-deflate WebSocket compression.
// Reduces bandwidth for larger payloads at the cost of CPU.
// Only effective when using the default GorillaUpgrader.
func WithWebSocketCompression() Option {
	return func(c *Config) {
		if gu, ok := c.Upgrader.(*GorillaUpgrader); ok {
			gu.SetCompression(true)
		} else {
			slog.Warn("WithWebSocketCompression has no effect on non-Gorilla WSUpgrader implementations")
		}
	}
}

// WithMessageRateLimit sets the rate limit for WebSocket messages per connection.
//
// Uses token bucket algorithm: messagesPerSecond determines the rate,
// burstCapacity allows short bursts above the rate.
//
// Default: 10 messages/sec with burst of 20.
// Set messagesPerSecond = 0 to disable rate limiting (not recommended for production).
//
// Example:
//
//	tmpl := livetemplate.New("app",
//	    livetemplate.WithMessageRateLimit(20, 50), // 20 msg/sec, burst of 50
//	)
func WithMessageRateLimit(messagesPerSecond float64, burstCapacity int) Option {
	return func(c *Config) {
		c.MessageRateLimit = messagesPerSecond
		c.MessageRateBurst = burstCapacity
	}
}

// WithCookieMaxAge sets the maximum age for session cookies.
//
// The cookie is used to maintain anonymous user sessions across page reloads.
// Default: 365 days (1 year)
//
// Example:
//
//	tmpl := livetemplate.New("app",
//	    livetemplate.WithCookieMaxAge(30*24*time.Hour), // 30 days
//	)
func WithCookieMaxAge(maxAge time.Duration) Option {
	return func(c *Config) {
		c.CookieMaxAge = maxAge
	}
}

// WithUpload configures file upload support for a specific form field.
//
// Upload configuration specifies validation rules, size limits, and storage options.
// Once configured, uploads are accessible via Context during action handling.
//
// Example:
//
//	tmpl := livetemplate.New("profile",
//	    livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
//	        Accept:      []string{"image/png", "image/jpeg"},
//	        MaxFileSize: 5 << 20, // 5 MB
//	        MaxFiles:    1,
//	    }),
//	)
//
// In your controller's action method, access uploads via Context:
//
//	func (c *ProfileController) SaveProfile(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
//	    if ctx.HasUploads("avatar") {
//	        for _, entry := range ctx.GetCompletedUploads("avatar") {
//	            state.AvatarURL = moveToStorage(entry.TempPath)
//	        }
//	    }
//	    return state, nil
//	}
func WithUpload(name string, config uploadtypes.UploadConfig) Option {
	return func(c *Config) {
		if c.UploadConfigs == nil {
			c.UploadConfigs = make(map[string]uploadtypes.UploadConfig)
		}
		// Back-compat: a config that sets External without an explicit Mode is a
		// Direct (presigned) upload. UploadModeVolume is the zero value, so this
		// only promotes the legacy "External implies direct-to-storage" shape.
		if config.External != nil && config.Mode == uploadtypes.UploadModeVolume {
			config.Mode = uploadtypes.UploadModeDirect
		}
		c.UploadConfigs[name] = config
	}
}

// uploadConfigsNeedDisk reports whether any configured upload field stages bytes
// to the local filesystem (Volume mode). Direct/Proxied/Preview never do, so a
// template using only those needs no temp file manager.
func uploadConfigsNeedDisk(configs map[string]uploadtypes.UploadConfig) bool {
	for _, c := range configs {
		if c.Mode == uploadtypes.UploadModeVolume {
			return true
		}
	}
	return false
}

// uploadConfigsNeedStreaming reports whether any field needs the zero-buffer
// MultipartReader path on an HTTP POST: Proxied (streamed to OnUpload) OR
// Volume-with-Dir (retained staging via stageVolumePart, which the Tier-1
// ParseMultipartForm path ignores — it stages ephemerally to the session temp
// dir). This is the WebSocket-disabled fallback for Volume-with-Dir fields
// (issue #449); ephemeral Volume (no Dir) stays on the Tier-1 path.
func uploadConfigsNeedStreaming(configs map[string]uploadtypes.UploadConfig) bool {
	for _, c := range configs {
		if c.Mode == uploadtypes.UploadModeProxied {
			return true
		}
		if c.Mode == uploadtypes.UploadModeVolume && c.Dir != "" {
			return true
		}
	}
	return false
}

// WithPubSubBroadcaster enables distributed peer-fan-out across multiple application instances.
//
// When set, ctx.Publish calls (and the framework-internal group/user/global scopes) are
// republished to Redis Pub/Sub via the configured pubsub.Broadcaster. Each instance subscribes
// to these channels and fans the messages out to its local WebSocket connections.
//
// This is essential for horizontal scaling — without it, ctx.Publish only reaches connections
// on the same instance.
//
// Example:
//
//	import (
//	    "github.com/livetemplate/livetemplate"
//	    "github.com/livetemplate/livetemplate/pubsub"
//	    "github.com/redis/go-redis/v9"
//	)
//
//	redisClient := redis.NewClient(&redis.Options{
//	    Addr: "localhost:6379",
//	})
//
//	broadcaster := pubsub.NewRedisBroadcaster(redisClient)
//
//	tmpl := livetemplate.New("app",
//	    livetemplate.WithPubSubBroadcaster(broadcaster),
//	)
//
// Redis must be reachable when Handle() is called; configure a DialTimeout
// on the Redis client to bound startup blocking.
func WithPubSubBroadcaster(broadcaster pubsub.Broadcaster) Option {
	return func(c *Config) {
		c.PubSubBroadcaster = broadcaster
	}
}

// WithComponentTemplates registers component library templates to be parsed before project templates.
// This enables using pre-built UI components from the livetemplate/components library or custom
// component libraries.
//
// Component templates are parsed first, then project templates are parsed on top, allowing
// project templates to override component templates with the same name.
//
// Example:
//
//	import "github.com/livetemplate/lvt/components"
//
//	tmpl, err := livetemplate.New("app",
//	    livetemplate.WithComponentTemplates(components.All()...),
//	)
//
// Or with specific components:
//
//	import (
//	    "github.com/livetemplate/lvt/components/dropdown"
//	    "github.com/livetemplate/lvt/components/tabs"
//	)
//
//	tmpl, err := livetemplate.New("app",
//	    livetemplate.WithComponentTemplates(
//	        dropdown.Templates(),
//	        tabs.Templates(),
//	    ),
//	)
//
// Templates are parsed in the order provided. Official component templates use the naming
// convention "lvt:<category>:<name>:v<version>" (e.g., "lvt:dropdown:searchable:v1").
// Third-party components may use their own prefix (e.g., "myorg:widget:default:v1").
func WithComponentTemplates(sets ...*TemplateSet) Option {
	return func(c *Config) {
		c.ComponentTemplates = append(c.ComponentTemplates, sets...)
	}
}

// WithProgressiveEnhancement enables or disables progressive enhancement support.
//
// When enabled (default: true), HTTP form submissions from non-JavaScript clients
// receive full HTML page responses instead of JSON. This allows applications to
// work without JavaScript using standard HTML form submissions.
//
// The feature uses the POST-Redirect-GET (PRG) pattern:
//   - Successful actions: 303 redirect to prevent duplicate submissions on refresh
//   - Validation errors: Re-render page with errors inline (no redirect)
//
// Detection uses the Accept header: clients sending "application/json" receive JSON,
// while browsers sending "text/html" receive full HTML pages.
//
// Example form structure for progressive enhancement:
//
//	<form method="POST">
//	    <input type="text" name="title">
//	    <button name="action" value="add" type="submit">Add</button>
//	</form>
//
// Using an explicit action value avoids ambiguous POST parsing when other form
// fields submit empty strings.
//
// The form works with both JavaScript (via WebSocket or fetch/JSON) and without
// JavaScript (via method="POST").
func WithProgressiveEnhancement(enabled bool) Option {
	return func(c *Config) {
		c.ProgressiveEnhancement = enabled
	}
}

// New creates a new template with the given name and options.
//
// By default, New auto-discovers template files in the current directory and common
// template directories (templates/, views/, etc.), looking for files with extensions:
// .tmpl, .html, .gotmpl
//
// # Template Discovery
//
// The template name is used to find the template file. For example:
//
//	livetemplate.New("counter")
//
// Will look for counter.tmpl, counter.html, or counter.gotmpl in:
//   - Current directory
//   - ./templates/
//   - ./views/
//
// # Options
//
// Use functional options to configure the template:
//
//	// Override auto-discovery with specific files
//	tmpl := livetemplate.New("app", livetemplate.WithParseFiles("app.tmpl", "partials.tmpl"))
//
//	// Disable WebSocket, use HTTP only
//	tmpl := livetemplate.New("app", livetemplate.WithWebSocketDisabled())
//
//	// Use custom session store
//	tmpl := livetemplate.New("app", livetemplate.WithSessionStore(myStore))
//
//	// Use custom authentication
//	auth := livetemplate.NewBasicAuthenticator(validateUser)
//	tmpl := livetemplate.New("app", livetemplate.WithAuthenticator(auth))
//
//	// Restrict WebSocket origins (production security)
//	tmpl := livetemplate.New("app", livetemplate.WithAllowedOrigins([]string{
//	    "https://yourdomain.com",
//	}))
//
// # Configuration
//
// The template is configured with sensible defaults:
//   - Secure WebSocket origin checking (same-origin only, configurable via WithAllowedOrigins)
//   - In-memory session store
//   - Anonymous authenticator (browser-based session grouping)
//   - Auto-discovery enabled
//   - Loading indicator enabled
//   - Production mode (CDN client library)
//
// # Environment Variables
//
// New does not read environment variables directly. To apply environment-based
// configuration (e.g., LVT_WS_BUFFER_SIZE, LVT_MAX_CONNECTIONS), use
// [LoadEnvConfig] and pass the resulting options:
//
//	envConfig, err := livetemplate.LoadEnvConfig()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	tmpl := livetemplate.New("app", envConfig.ToOptions()...)
//
// See the With* functions for available options.

// Must is a helper that wraps a call to New and panics if the error is non-nil.
// It is intended for use in variable initializations and startup code where
// template initialization failures should be fatal, such as:
//
//	var t = livetemplate.Must(livetemplate.New("app"))
//
// This follows the same pattern as html/template.Must and text/template.Must.
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return t
}

// createSecureOriginChecker creates a CheckOrigin function that enforces origin restrictions.
//
// Security behavior:
//   - DevMode=true: Allows all origins (for local development)
//   - DevMode=false with AllowedOrigins empty: Same-origin only (secure default)
//   - DevMode=false with AllowedOrigins set: Only allows listed origins
//
// This prevents CSRF attacks by rejecting WebSocket upgrade requests from unauthorized origins.
func createSecureOriginChecker(allowedOrigins []string, devMode bool, trustForwardedHeaders bool) func(*http.Request) bool {
	return func(r *http.Request) bool {
		// Development mode: allow all origins for convenience
		if devMode {
			return true
		}

		origin := r.Header.Get("Origin")

		// No origin header: allow (same-origin requests may not include Origin)
		if origin == "" {
			return true
		}

		// If AllowedOrigins is specified, check against the list
		if len(allowedOrigins) > 0 {
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			// Origin not in allowed list
			return false
		}

		// Default: same-origin only
		// Compare origin against the request's Host header
		host := r.Host
		if host == "" {
			return false
		}

		// Derive scheme from forwarded headers (if trusted) or r.TLS (direct).
		// See WithTrustForwardedHeaders godoc for security trade-offs.
		scheme := ""
		if trustForwardedHeaders {
			scheme = forwardedScheme(r)
		}
		if scheme == "" {
			scheme = "https"
			if r.TLS == nil {
				scheme = "http"
			}
		}

		// Check if origin matches scheme://host
		expectedOrigin := scheme + "://" + host
		return origin == expectedOrigin
	}
}

// forwardedScheme derives the original client scheme ("http" or "https") from
// proxy forwarding headers. It prefers the de-facto X-Forwarded-Proto (set by
// every major proxy) and falls back to the standard RFC 7239 Forwarded header.
// Returns "" when neither header yields a valid scheme, letting the caller fall
// back to r.TLS.
//
// Callers MUST only invoke this when forwarded headers are trusted, since a
// direct client can forge either header.
func forwardedScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		first, _, _ := strings.Cut(proto, ",")
		if s := normalizeScheme(first); s != "" {
			return s
		}
	}
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		if s := forwardedHeaderProto(fwd); s != "" {
			return s
		}
	}
	return ""
}

// forwardedHeaderProto extracts the proto parameter from the first (client-most)
// element of an RFC 7239 Forwarded header, e.g. `for=192.0.2.1;proto=https`.
// Elements are comma-separated and parameters semicolon-separated with
// case-insensitive names; the value may be quoted. Returns "" if no valid
// http/https proto is present.
func forwardedHeaderProto(header string) string {
	first, _, _ := strings.Cut(header, ",")
	for _, param := range strings.Split(first, ";") {
		key, val, ok := strings.Cut(param, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "proto") {
			continue
		}
		return normalizeScheme(strings.Trim(strings.TrimSpace(val), `"`))
	}
	return ""
}

// normalizeScheme trims and lowercases a scheme token, returning it only if it
// is a recognized "http" or "https"; otherwise it returns "".
func normalizeScheme(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "http" || v == "https" {
		return v
	}
	return ""
}

func New(name string, opts ...Option) (*Template, error) {
	// Default configuration
	config := Config{
		Upgrader:               NewGorillaUpgrader(), // Default: gorilla with 1KB buffers
		SessionStore:           NewMemorySessionStore(),
		Authenticator:          &AnonymousAuthenticator{},  // Default: browser-based session grouping
		MessageRateLimit:       10.0,                       // Default: 10 messages/sec
		MessageRateBurst:       20,                         // Default: burst of 20
		CookieMaxAge:           365 * 24 * time.Hour,       // Default: 1 year
		WebSocketBufferSize:    defaultWebSocketBufferSize, // Override via WithWebSocketBufferSize or EnvConfig
		ProgressiveEnhancement: true,                       // Default: enabled for non-JS form support
		TrustForwardedHeaders:  true,                       // Default: trust forwarded scheme headers (safe behind proxy)
	}

	// Apply options
	for _, opt := range opts {
		opt(&config)
	}

	// WithTopicACL and WithOpenTopics are mutually exclusive. Detected here at
	// New() — not inside the With*() calls — so the result is order-independent:
	// New("app", WithOpenTopics(), WithTopicACL(fn)) and the reverse both fail
	// identically. Returned as an error (not a panic): option values may come
	// from runtime config and New() already returns error.
	if config.TopicACL != nil && config.OpenTopics {
		return nil, fmt.Errorf("livetemplate: WithTopicACL and WithOpenTopics are mutually exclusive (set exactly one, or neither for deny-all)")
	}

	// Set secure CheckOrigin on the default gorilla upgrader (after options are applied)
	if gu, ok := config.Upgrader.(*GorillaUpgrader); ok {
		if gu.inner.CheckOrigin == nil {
			gu.SetCheckOrigin(createSecureOriginChecker(config.AllowedOrigins, config.DevMode, config.TrustForwardedHeaders))
		}
	}

	// Log DevMode configuration for debugging
	slog.Debug("Template created",
		slog.String("name", name),
		slog.Bool("dev_mode", config.DevMode))

	tmpl := &Template{
		name:   name,
		config: config,
	}

	// Parse component templates first (before project templates)
	// This establishes a base layer of templates that project templates can override
	if len(config.ComponentTemplates) > 0 {
		if err := tmpl.parseComponentTemplates(config.ComponentTemplates); err != nil {
			return nil, fmt.Errorf("livetemplate.New(%q): failed to parse component templates: %w", name, err)
		}
		if config.DevMode {
			slog.Debug("Parsed component template sets",
				slog.Int("count", len(config.ComponentTemplates)))
		}
	}

	// Parse from an explicit fs.FS (WithParseFS) first — it takes precedence over
	// file paths and auto-discovery.
	if config.TemplateFS != nil {
		if _, err := tmpl.ParseFS(config.TemplateFS, config.TemplateFSPatterns...); err != nil {
			return nil, fmt.Errorf("livetemplate.New(%q): failed to parse templates from fs.FS with patterns %v: %w", name, config.TemplateFSPatterns, err)
		}
	} else if len(config.TemplateFiles) == 0 {
		// Auto-discover and parse templates if not explicitly provided
		// Use TemplateBaseDir from config if provided, otherwise fall back to runtime.Caller
		files, err := discovery.DiscoverTemplateFiles(config.TemplateBaseDir, config.IgnoreTemplateDirs)
		if err != nil {
			return nil, fmt.Errorf("livetemplate.New(%q): template auto-discovery failed in %q: %w", name, config.TemplateBaseDir, err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("livetemplate.New(%q): no template files found in %q (ignored: %v)", name, config.TemplateBaseDir, config.IgnoreTemplateDirs)
		}
		if config.DevMode {
			slog.Debug("Auto-discovered template files",
				slog.Int("count", len(files)))
		}
		if _, err := tmpl.ParseFiles(files...); err != nil {
			return nil, fmt.Errorf("livetemplate.New(%q): failed to parse discovered files %v: %w", name, files, err)
		}
	} else {
		if _, err := tmpl.ParseFiles(config.TemplateFiles...); err != nil {
			return nil, fmt.Errorf("livetemplate.New(%q): failed to parse template files %v: %w", name, config.TemplateFiles, err)
		}
	}

	return tmpl, nil
}

// Clone creates a deep copy of the template with fresh state.
// This is useful for creating per-connection template instances that don't interfere with each other.
func (t *Template) Clone() (*Template, error) {
	// Acquire read lock to safely read template fields
	t.mu.RLock()
	name := t.name
	templateStr := t.templateStr
	wrapperID := t.wrapperID
	config := t.config
	tmpl := t.tmpl
	funcs := t.funcs
	cachedParse := t.cachedParseTemplate
	bodyContent := t.cachedBodyContent
	bodyContentValid := t.cachedBodyContentValid
	formSchema := t.formSchema
	wiredActions := t.wiredActions
	wiredCollisionWarned := t.wiredCollisionWarned
	t.mu.RUnlock()

	// Share immutable data from master instead of re-creating per clone.
	// Go's html/template.Execute() is safe for concurrent use after parsing
	// (see go.dev/src/html/template/template.go line 118). The template is
	// never modified after Parse(), so sharing is safe.
	clone := &Template{
		name:                   name,
		templateStr:            templateStr,
		tmpl:                   tmpl, // Share parsed template (concurrent Execute is safe)
		wrapperID:              wrapperID,
		funcs:                  funcs, // Share FuncMap (read-only after Parse)
		config:                 config,
		cachedParseTemplate:    cachedParse, // Share parsed AST + builtins
		cachedBodyContent:      bodyContent, // Share extracted body content
		cachedBodyContentValid: bodyContentValid,
		formSchema:             formSchema,           // Share extracted form schema (immutable)
		wiredActions:           wiredActions,         // Share extracted wired-action set (immutable)
		wiredCollisionWarned:   wiredCollisionWarned, // Share dedup store by pointer (app-global once-per-action warn)
		// Don't copy lastData, lastHTML, lastTree, etc. - start fresh per session
	}

	return clone, nil
}

// SetUploadRegistry sets the upload registry for this template instance.
// This should be called after cloning a template for a specific connection.
func (t *Template) SetUploadRegistry(registry interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.uploadRegistry = registry
}

// newUploadRegistry creates a new upload registry instance.
// This is used internally by the mount handler.
func (t *Template) newUploadRegistry() uploadRegistry {
	return newUploadRegistry()
}

// =============================================================================
// Phase 1: Parse - Template Parsing
// =============================================================================

// Parse parses text as a template body for the template t.
// This matches the signature of html/template.Template.Parse().
func (t *Template) Parse(text string) (*Template, error) {
	// Normalize template spacing to handle formatter-added spaces
	// This prevents issues when formatters add spaces like "{{ range" instead of "{{range"
	text = compat.NormalizeTemplateSpacing(text)

	// Strip HTML comments before parsing so they match html/template (which
	// drops them in its escape pass) and so a {{define}}/{{block}} living inside
	// a comment is removed rather than resolved during composition. See #468.
	text = compat.StripHTMLComments(text)

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = compat.GenerateRandomID()

	// First, parse WITHOUT wrapper to check if flattening is needed
	baseTemplate := template.New(t.name)
	if len(t.funcs) > 0 {
		baseTemplate = baseTemplate.Funcs(t.funcs)
	}
	tmpl, err := baseTemplate.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("template '%s' parse error: %w", t.name, err)
	}

	return t.parseInternal(text, tmpl)
}

// parseInternal handles the common logic for parsing templates:
// flattening, wrapper injection, final parsing, and validation.
func (t *Template) parseInternal(text string, baseTemplate *template.Template) (*Template, error) {
	// Check if template uses composition features and flatten if needed
	if parse.HasTemplateComposition(baseTemplate) {
		// Flatten the template to resolve all {{define}}/{{template}}/{{block}}
		flattenedStr, err := parse.FlattenTemplate(baseTemplate)
		if err != nil {
			return nil, fmt.Errorf("template flattening failed: %w", err)
		}

		// Store flattened version for tree generation (WITHOUT wrapper)
		// This ensures updates use the flattened template
		text = flattenedStr
	}

	// Determine if this is a full HTML document. Computed here (after any
	// flattening) on the already comment-stripped text — callers strip before
	// parsing (see #468) — so a `<!DOCTYPE`/`<html` marker that existed only
	// inside a removed comment cannot route a bare fragment through the
	// full-document wrapper path.
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

	// Expand multi-action bracket syntax before parsing.
	// e.g. lvt-el:addClass:on:[save,delete]:pending="X" → individual attributes.
	// Done at source level so both HTTP rendering (t.tmpl) and WebSocket tree
	// generation (t.templateStr → buildTreeWithCache) see expanded attributes.
	text = render.ExpandBracketAttributes(text)

	// Now add wrapper to the (possibly flattened) template for execution
	var templateContent string
	if isFullHTML {
		// Inject wrapper div around body content
		templateContent = compat.InjectWrapperDiv(text, t.wrapperID, t.config.LoadingDisabled)
	} else {
		// For standalone templates, wrap the entire content
		loadingAttr := ""
		if !t.config.LoadingDisabled {
			loadingAttr = ` data-lvt-loading="true"`
		}
		templateContent = fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>`, t.wrapperID, loadingAttr, text)
	}

	// Parse the template with wrapper for execution
	// Clone from baseTemplate to preserve component template definitions and funcs
	wrappedTemplate, err := baseTemplate.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone template for wrapper: %w", err)
	}
	// Create/update the named template within the cloned set
	wrappedTemplate = wrappedTemplate.New(t.name)
	if len(t.funcs) > 0 {
		wrappedTemplate = wrappedTemplate.Funcs(t.funcs)
	}
	tmpl, err := wrappedTemplate.Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template with wrapper: %w", err)
	}

	// Store the template text for tree generation (flattened if it had composition)
	t.templateStr = text
	t.tmpl = tmpl
	t.cachedParseTemplate = nil // Invalidate cached AST when template source changes
	t.cachedBodyContent = ""    // Invalidate cached body content
	t.cachedBodyContentValid = false
	t.formSchema = extractFormSchemaFromTemplateStr(text)
	t.wiredActions = extractWiredActionNames(text)
	if t.wiredActions != nil {
		t.wiredCollisionWarned = &sync.Map{}
	} else {
		t.wiredCollisionWarned = nil
	}

	// Validate that tree generation works with this template
	// This ensures templates with {{define}}/{{block}} are caught during initialization
	if err := t.validateTreeGeneration(); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	return t, nil
}

// namedTemplateSource is one template file resolved to its bytes plus a base
// name, decoupling how sources are read (OS files vs. an fs.FS) from how they are
// parsed. Both ParseFiles and ParseFS resolve their inputs to these and share
// parseSources.
type namedTemplateSource struct {
	name    string
	content []byte
}

// ParseFiles parses the named files and associates the resulting templates with t.
// This matches the signature of html/template.Template.ParseFiles().
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	sources := make([]namedTemplateSource, len(filenames))
	for i, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}
		sources[i] = namedTemplateSource{name: filepath.Base(filename), content: content}
	}

	return t.parseSources(sources)
}

// ParseFS parses templates from an fs.FS (e.g. an embed.FS), resolving the given
// glob patterns and associating the results with t. It mirrors ParseFiles but
// reads via fs.ReadFile, so embedded templates need not be staged to disk first.
//
// Like ParseFiles, the first resolved file is the main template and the rest are
// parsed into the same set for composition — unlike html/template.ParseFS, which
// has no first-match-is-main concept. fs.Glob returns matches in lexical order, so
// a single wildcard pattern (e.g. "templates/*.tmpl") makes the lexically-first
// match the main template; pass an explicit ordered pattern list when that matters.
//
// Patterns are resolved in order with no dedup, so a file matched by two overlapping
// patterns is parsed twice; pass non-overlapping patterns to avoid that.
func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no patterns specified")
	}

	var sources []namedTemplateSource
	for _, pattern := range patterns {
		matches, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, fmt.Errorf("glob pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			content, err := fs.ReadFile(fsys, match)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", match, err)
			}
			sources = append(sources, namedTemplateSource{name: path.Base(match), content: content})
		}
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no files match patterns: %v", patterns)
	}

	return t.parseSources(sources)
}

// parseSources parses one or more resolved template sources into t. The first
// source is the main template; the rest are parsed into the same set for
// composition. Shared by ParseFiles and ParseFS.
func (t *Template) parseSources(sources []namedTemplateSource) (*Template, error) {
	main := sources[0]

	// Use the first source's base name as template name if not already set
	if t.name == "" {
		t.name = main.name
	}

	// Normalize template spacing
	text := compat.NormalizeTemplateSpacing(string(main.content))

	// Strip HTML comments before parsing (see #468).
	text = compat.StripHTMLComments(text)

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = compat.GenerateRandomID()

	// First, parse WITHOUT wrapper to check if flattening is needed
	// If component templates were already parsed, use that as the base
	// This allows component templates to be available during parsing and flattening
	var baseTemplate *template.Template
	if t.tmpl != nil {
		// Clone the existing template to preserve component definitions
		cloned, err := t.tmpl.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone template with components: %w", err)
		}
		// Create a new named template within the cloned set
		baseTemplate = cloned.New(t.name)
	} else {
		baseTemplate = template.New(t.name)
	}
	if len(t.funcs) > 0 {
		baseTemplate = baseTemplate.Funcs(t.funcs)
	}
	tmpl, err := baseTemplate.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("template '%s' parse error: %w", t.name, err)
	}

	// Parse additional sources if provided (for template composition)
	for _, src := range sources[1:] {
		// Parse additional templates into the same template set
		// (comments stripped first — see #468).
		if _, err := tmpl.Parse(compat.StripHTMLComments(string(src.content))); err != nil {
			return nil, fmt.Errorf("failed to parse file %s: %w", src.name, err)
		}
	}

	return t.parseInternal(text, tmpl)
}

// ParseGlob parses the template definitions from the files identified by the pattern.
// This matches the signature of html/template.Template.ParseGlob().
func (t *Template) ParseGlob(pattern string) (*Template, error) {
	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern error: %w", err)
	}

	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files match pattern: %s", pattern)
	}

	return t.ParseFiles(filenames...)
}

// parseComponentTemplates parses templates from embedded component libraries.
// Component templates are parsed first to establish a base layer that project
// templates can override.
func (t *Template) parseComponentTemplates(sets []*TemplateSet) error {
	for _, set := range sets {
		if set == nil {
			continue
		}

		// Find all files matching the pattern in the embedded filesystem
		matches, err := fs.Glob(set.FS, set.Pattern)
		if err != nil {
			return fmt.Errorf("component %q: invalid glob pattern %q: %w", set.Namespace, set.Pattern, err)
		}

		if len(matches) == 0 {
			return fmt.Errorf("component %q: no files match pattern %q", set.Namespace, set.Pattern)
		}

		// Parse each matching file
		for _, match := range matches {
			content, err := fs.ReadFile(set.FS, match)
			if err != nil {
				return fmt.Errorf("component %q: failed to read %q: %w", set.Namespace, match, err)
			}

			// Create or get the base template
			if t.tmpl == nil {
				baseTmpl := template.New(t.name)
				if len(t.funcs) > 0 {
					baseTmpl = baseTmpl.Funcs(t.funcs)
				}
				if set.Funcs != nil {
					baseTmpl = baseTmpl.Funcs(set.Funcs)
				}
				t.tmpl = baseTmpl
			} else if set.Funcs != nil {
				// Add component-specific funcs to existing template
				t.tmpl = t.tmpl.Funcs(set.Funcs)
			}

			// Parse the template content - this adds any {{define}} blocks to the template set
			// (comments stripped first to match html/template — see #468).
			_, err = t.tmpl.Parse(compat.StripHTMLComments(string(content)))
			if err != nil {
				return fmt.Errorf("component %q: failed to parse %q: %w", set.Namespace, match, err)
			}
		}
	}

	return nil
}

// =============================================================================
// Public API - Execution (Orchestrates All 5 Phases)
// =============================================================================

// Execute applies a parsed template to the specified data object,
// writing the output to wr. It orchestrates all 5 phases:
//
//	Phase 1: Parse (already done via Parse/ParseFiles/ParseGlob)
//	Phase 2: Build - Generate tree structure
//	Phase 3: Diff - Compare with cached state (no-op for first render)
//	Phase 4: Render - Execute template to HTML
//	Phase 5: Send - Write HTML response
//
// Note: Phases execute in order 1→4→5→2 (Render before Build) to minimize
// response latency. Tree building for caching happens after sending the response.
//
// The optional messages parameter provides context for templates via the lvt namespace.
// It contains both field validation errors and flash messages (prefixed with "_flash:").
// Field errors affect ResponseMetadata.Success; flash messages don't.
func (t *Template) Execute(wr io.Writer, data interface{}, messages ...map[string]string) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	var msgMap map[string]string
	if len(messages) > 0 {
		msgMap = messages[0]
	}
	if msgMap == nil {
		msgMap = make(map[string]string)
	}

	// Phase 1: Parse (already completed during New/Parse/ParseFiles/ParseGlob)

	// Phase 4: Render HTML (done first to get the HTML for output)
	html, err := t.renderHTML(data, msgMap)
	if err != nil {
		return err
	}

	// Phase 5: Send HTML response
	err = t.sendResponse(wr, html)
	if err != nil {
		return err
	}

	// Phase 2: Build tree structure for caching (includes Phase 3: Diff internally)
	// This is done after sending the response for performance
	_, treeErr := t.buildTree(data, msgMap)
	if treeErr != nil {
		// Don't fail if tree generation fails, just skip caching
		// Log for observability so operators can detect degraded performance
		slog.Warn("Tree building failed, skipping cache update",
			slog.String("template", t.name),
			slog.Any("error", treeErr))
		return nil
	}

	return nil
}

// ExecuteUpdates generates a tree structure of static and dynamic content
// that can be used by JavaScript clients to update changed parts efficiently.
// It orchestrates all 5 phases:
//
//	Phase 1: Parse (already done via Parse/ParseFiles/ParseGlob)
//	Phase 2: Build - Generate tree structure (includes Phase 3: Diff internally)
//	Phase 3: Diff - Compare with cached tree, return only changes (integrated in Build)
//	Phase 4: Render - Execute template (integrated in Build)
//	Phase 5: Send - Write JSON tree response
//
// Caching behavior:
// - First call: Returns complete tree with static structure ("s" key) and dynamic values
// - Subsequent calls: Returns only dynamic values that have changed (cache-aware)
//
// The optional messages parameter provides context for templates via the lvt namespace.
// It contains both field validation errors and flash messages (prefixed with "_flash:").
// Field errors affect ResponseMetadata.Success; flash messages don't.
func (t *Template) ExecuteUpdates(wr io.Writer, data interface{}, messages ...map[string]string) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	var msgMap map[string]string
	if len(messages) > 0 {
		msgMap = messages[0]
	}
	if msgMap == nil {
		msgMap = make(map[string]string)
	}

	// Phase 1: Parse (already completed during New/Parse/ParseFiles/ParseGlob)

	// Phase 2: Build tree structure (includes Phase 3: Diff and Phase 4: Render internally)
	tree, err := t.buildTree(data, msgMap)
	if err != nil {
		return fmt.Errorf("tree generation failed: %w", err)
	}

	// Phase 5: Send JSON tree response
	return t.sendResponse(wr, tree)
}

// =============================================================================
// Internal Helper Methods (Phase 2: Build)
// =============================================================================

// getOrComputeBodyContent returns the cached body content from t.templateStr (the template
// source with {{}} action syntax), NOT from rendered HTML. The parser needs template actions
// to identify dynamic slots, so this intentionally uses the source text.
// Caller must hold t.mu write lock.
func (t *Template) getOrComputeBodyContent() string {
	if !t.cachedBodyContentValid {
		if t.wrapperID != "" {
			t.cachedBodyContent = compat.ExtractTemplateBodyContent(t.templateStr)
		} else {
			t.cachedBodyContent = t.templateStr
		}
		t.cachedBodyContentValid = true
	}
	return t.cachedBodyContent
}

// buildTreeWithCache builds a tree using the cached parse template, parsing on first call.
// Caller must hold t.mu write lock.
func (t *Template) buildTreeWithCache(data interface{}, ctx *build.Context) (*treeNode, error) {
	if t.cachedParseTemplate == nil {
		templateContent := t.getOrComputeBodyContent()
		parsedTmpl, err := compat.ParseAndCacheTemplate(templateContent, t.funcs)
		if err != nil {
			return nil, err
		}
		t.cachedParseTemplate = parsedTmpl
	}
	// ctx.FuncMap is set by callers but not used here — builtins were pre-computed
	// at parse time and stored on cachedParseTemplate (see PrecomputeBuiltins).
	return compat.BuildTreeFromCached(t.cachedParseTemplate, data, ctx)
}

// generateInitialTreeWithoutRegistry creates tree with statics and dynamics for first render.
// extractedContent is the pre-extracted HTML content (from wrapper extraction in buildTree).
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
// Errors from buildTreeWithCache are absorbed via the HTML structure-based fallback,
// so this method never propagates failure to the caller.
func (t *Template) generateInitialTreeWithoutRegistry(data interface{}, extractedContent string) *treeNode {
	ctx := build.NewContext()
	ctx.DevMode = t.config.DevMode

	tree, err := t.buildTreeWithCache(data, ctx)
	if err != nil {
		slog.Warn("Template parsing failed, falling back to HTML structure-based tree",
			slog.String("template", t.name),
			slog.Any("error", err))
		tree = build.CreateHTMLStructureBasedTree(extractedContent)
		// Don't set hasInitialTree — subsequent renders must use the HTML fallback
		// path (AnalyzeChangeAndCreateTree) since the AST path will fail again.
	} else {
		t.hasInitialTree = true
		t.lastHTML = "" // Free initial HTML — no longer needed once AST path is active
	}

	// Store complete tree as the baseline for comparison
	t.lastTree = tree

	// NOTE: Caller is responsible for calling markAllStructuresAsSeen outside the lock
	return tree
}

// generateDiffBasedTree creates tree based on diff analysis
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
func (t *Template) generateDiffBasedTree(oldHTML, newHTML string, newData interface{}) (*treeNode, error) {
	// Generate new complete tree for comparison
	if t.hasInitialTree {
		// MAIN PATH: tree generation uses t.templateStr (template source), not extracted
		// rendered HTML. No html.Parse() extraction needed on this path.
		// Note: t.lastHTML is intentionally not updated here — it holds stale data from
		// the first render. This is safe because lastHTML is only consumed by the fallback
		// path below, which is unreachable once hasInitialTree is true.
		ctx := build.NewContext()
		ctx.DevMode = t.config.DevMode

		newTree, err := t.buildTreeWithCache(newData, ctx)
		if err != nil {
			return nil, fmt.Errorf("tree generation failed: %w", err)
		}

		changedTree := t.compareTreesAndGetChanges(t.lastTree, newTree)

		if !changedTree.HasStatics() && !changedTree.HasDynamics() && !changedTree.HasRange() {
			return build.NewTreeNode(), nil
		}

		t.lastTree = newTree

		return changedTree, nil
	}

	// FALLBACK PATH: extract lazily (rare - only when hasInitialTree is false)
	var oldContent, newContent string
	if t.wrapperID != "" {
		oldContent = compat.ExtractTemplateContent(oldHTML, t.wrapperID)
		newContent = compat.ExtractTemplateContent(newHTML, t.wrapperID)
	} else {
		oldContent = oldHTML
		newContent = newHTML
	}

	tree, err := build.AnalyzeChangeAndCreateTree(oldContent, newContent)
	if err != nil {
		return nil, err
	}

	t.lastHTML = newContent

	return tree, nil
}

// =============================================================================
// Internal Helper Methods (Phase 3: Diff)
// =============================================================================

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree *treeNode) *treeNode {
	return t.compareTreesAndGetChangesWithContext(oldTree, newTree, false)
}

// compareTreesAndGetChangesWithContext compares trees with context about whether we're in a new structure
// insideNewStructure: true if we're inside a structure the client has never seen
func (t *Template) compareTreesAndGetChangesWithContext(oldTree, newTree *treeNode, insideNewStructure bool) *treeNode {
	// Calculate range matches once at the top level for the entire tree
	rangeMatches := diff.FindRangeConstructMatches(oldTree, newTree)
	return diff.CompareTreesAndGetChangesWithPath(oldTree, newTree, insideNewStructure, "", rangeMatches)
}

// =============================================================================
// Controller+State Pattern Handle
// =============================================================================

// Handle creates an http.Handler using the Controller+State pattern.
//
// Controller: Singleton that holds dependencies (DB, Logger, etc.). Never cloned.
// State: Pure data that is cloned per session via serialization. Must be wrapped with AsState().
//
// The Controller+State separation ensures dependencies are never accidentally
// shared across sessions while pure state data is cloned via serialization.
//
// Example:
//
//	handler := tmpl.Handle(
//	    &TodoController{DB: db, Logger: logger},
//	    AsState(&TodoState{}),
//	)
//	http.Handle("/todos", handler)
//
// Lifecycle methods (all optional):
//   - Mount(state, ctx) - Called once when session is created
//   - OnConnect(state, ctx) - Called on WebSocket connect/reconnect
//   - OnDisconnect() - Called on WebSocket disconnect
//
// Action methods have signature: func(state StateType, ctx *Context) (StateType, error)
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

	// Apply origin validation to a copy of the upgrader (avoid mutating shared state)
	upgrader := t.config.Upgrader
	if len(t.config.AllowedOrigins) > 0 {
		if gu, ok := upgrader.(*GorillaUpgrader); ok {
			upgCopy := gu.Copy()
			allowedOrigins := t.config.AllowedOrigins
			upgCopy.SetCheckOrigin(func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, allowed := range allowedOrigins {
					if origin == allowed {
						return true
					}
				}
				slog.Warn("WebSocket origin rejected",
					slog.String("origin", origin))
				return false
			})
			upgrader = upgCopy
		}
	}

	// Determine session store - use option, then template config, then default
	sessionStore := config.sessionStore
	if sessionStore == nil {
		sessionStore = t.config.SessionStore
	}
	if sessionStore == nil {
		sessionStore = NewMemorySessionStore()
	}

	mountCfg := mountConfig{
		Template:               t,
		Controller:             controller,
		State:                  state,
		Upgrader:               upgrader,
		SessionStore:           sessionStore,
		Authenticator:          t.config.Authenticator,
		PubSubBroadcaster:      t.config.PubSubBroadcaster,
		AllowedOrigins:         t.config.AllowedOrigins,
		WebSocketDisabled:      t.config.WebSocketDisabled,
		MaxConnections:         t.config.MaxConnections,
		MaxConnectionsPerGroup: t.config.MaxConnectionsPerGroup,
		CookieMaxAge:           t.config.CookieMaxAge,
		UploadConfigs:          t.config.UploadConfigs,
		wsBufferSize:           t.config.WebSocketBufferSize,
		ProgressiveEnhancement: t.config.ProgressiveEnhancement,
		TopicACL:               t.config.TopicACL,
		OpenTopics:             t.config.OpenTopics,
	}

	mountCfg.Capabilities = detectCapabilities(controller, state.Inner(), &mountCfg)
	validateLifecycleSignatures(controller, state.Inner())

	limits := session.NewConnectionLimits(mountCfg.MaxConnections, mountCfg.MaxConnectionsPerGroup)
	metrics := observe.NewMetrics(slog.Default())
	metricsExporter := observe.NewPrometheusExporter(metrics, limits)

	// Initialize upload factories (lazy, done once)
	initUploadFactories()

	// Only stage to local disk for Volume-mode uploads. Direct, Proxied, and
	// Preview never touch the server's filesystem, so a pure-remote-storage app
	// needs no writable working directory and must not fail at startup creating
	// .uploads — the deploy-time failure that motivated streaming uploads (#447).
	var tempFileManager uploadTempFileManager
	if uploadConfigsNeedDisk(t.config.UploadConfigs) {
		var err error
		tempFileManager, err = newUploadTempFileManager("")
		if err != nil {
			slog.Error("Failed to create temp file manager - Volume-mode uploads will not work",
				slog.Any("error", err))
		}
	}

	// Detect persist fields for selective state persistence
	var ps persistableState
	if p, ok := state.(persistableState); ok && p.HasPersistFields() {
		ps = p
	}

	sweepTTL := config.ephemeralSweepTTL
	if sweepTTL <= 0 {
		sweepTTL = defaultEphemeralSweepTTL
	}

	handler := &liveHandler{
		config:            mountCfg,
		persistable:       ps,
		registry:          session.NewConnectionRegistry(),
		limits:            limits,
		metricsExporter:   metricsExporter,
		tempFileManager:   tempFileManager,
		needsStreaming:    uploadConfigsNeedStreaming(t.config.UploadConfigs),
		ephemeralSweepTTL: sweepTTL,
		shutdownChan:      make(chan struct{}),
	}

	// Wire up metrics to registry for WebSocket observability
	handler.registry.SetMetrics(metrics)
	if t.config.DispatchBufferSize > 0 {
		handler.registry.SetDispatchBufferSize(t.config.DispatchBufferSize)
	}

	// Start periodic sweep of stale HTTP template cache entries
	go handler.httpTemplateSweepLoop()

	// Start pub/sub subscriber if broadcaster is configured.
	// Subscribe() must return only after the broadcaster is ready to accept
	// dynamic per-connection SubscribeTo* calls, so calling it synchronously
	// here avoids races before WebSocket connections begin using pub/sub.
	// If the backing store (e.g. Redis) is unreachable, this blocks until
	// the client's configured dial timeout fires.
	if mountCfg.PubSubBroadcaster != nil {
		if err := mountCfg.PubSubBroadcaster.Subscribe(handler.handlePubSubMessage); err != nil {
			slog.Warn("Pub/sub subscriber failed - cross-instance dispatch disabled",
				slog.Any("error", err))
		} else {
			slog.Info("Pub/sub subscriber started")
			if err := mountCfg.PubSubBroadcaster.RegisterServerActionHandler(handler.handleServerActionMessage); err != nil {
				slog.Error("Failed to subscribe to server actions",
					slog.Any("error", err))
			}

			if gab, ok := mountCfg.PubSubBroadcaster.(pubsub.GroupActionBroadcaster); ok {
				if err := gab.RegisterGroupActionHandler(handler.handleGroupActionMessage); err != nil {
					slog.Error("Failed to subscribe to group actions",
						slog.Any("error", err))
				}
			}

			// One topicActionHandler per broadcaster: a second Handle() sharing
			// this *RedisBroadcaster gets "already subscribed to topic actions"
			// here, which is logged and swallowed (consistent with the
			// RegisterGroupActionHandler/RegisterServerActionHandler pattern above) — that
			// second handler then silently receives NO cross-instance topic
			// actions for its whole lifetime. Operators must give each Handle()
			// its own broadcaster. Contract recorded for Phase 6 docs in
			// learnings/phase-2.md "Open questions".
			if tab, ok := mountCfg.PubSubBroadcaster.(pubsub.TopicActionBroadcaster); ok {
				if err := tab.RegisterTopicActionHandler(handler.handleTopicActionMessage); err != nil {
					slog.Error("Failed to subscribe to topic actions",
						slog.String("component", "live_handler"),
						slog.String("event", "topic_action_subscribe_failed"),
						slog.Any("error", err))
				}
			}
		}
	}

	return handler
}

// HandleOption configures Handle behavior
type HandleOption func(*handleConfig)

type handleConfig struct {
	sessionStore      SessionStore
	ephemeralSweepTTL time.Duration
}

// WithStore sets the session store for state persistence.
// Use this to configure Redis or other distributed stores.
func WithStore(store SessionStore) HandleOption {
	return func(c *handleConfig) {
		c.sessionStore = store
	}
}

// WithEphemeralSweepTTL sets how long an idle HTTP template cache entry survives
// in ephemeral mode (state with no lvt:"persist" fields) before the sweep loop
// evicts it. Defaults to 30 minutes when unset or non-positive. Has no effect in
// persistent mode, where eviction follows the SessionStore instead.
//
// Eviction runs on a periodic sweep, so an entry may persist up to one sweep
// interval beyond the TTL; very short TTLs are floored to keep the sweep cheap.
func WithEphemeralSweepTTL(ttl time.Duration) HandleOption {
	return func(c *handleConfig) {
		c.ephemeralSweepTTL = ttl
	}
}

// validateTreeGeneration validates that tree generation works with this template
// Templates with {{define}}/{{block}}/{{template}} are now supported via automatic flattening
func (t *Template) validateTreeGeneration() error {
	// Template composition ({{define}}/{{block}}/{{template}}) is now supported
	// The tree generation process automatically flattens composite templates
	// No validation needed here - errors will be caught during flattening if they occur
	return nil
}

// =============================================================================
// Phase 2: Build - Tree Construction
// =============================================================================

// buildTree orchestrates tree building for the current render.
// This is the main entry point for Phase 2 (Build).
// It handles both initial renders and subsequent updates internally.
// Thread-safe: uses single lock acquisition to prevent race conditions.
func (t *Template) buildTree(data interface{}, messages map[string]string) (*treeNode, error) {
	// Build data map ONCE — this performs reflection over the data struct/map.
	// The result is reused for both HTML rendering and tree building,
	// eliminating the duplicate reflection that previously occurred in
	// renderHTML (via ExecuteTemplateWithContext) and AddLvtToData.
	dataWithLvt := context.BuildDataMap(data, messages, t.config.DevMode, t.uploadRegistry)

	// Phase 4: Render HTML using pre-built data map (no reflection)
	currentHTML, err := t.renderHTMLWithData(dataWithLvt)
	if err != nil {
		return nil, err
	}

	// Note: We don't invalidate the expression cache here because:
	// 1. Cache keys include dataHash, so changed data naturally misses the cache
	// 2. Cache is intra-render optimization (expressions within a single template execution)
	// 3. Invalidating on every render would defeat the purpose of caching
	// The cache will naturally expire as data changes across renders.

	// Acquire lock once for all state reads/writes
	t.mu.Lock()
	defer t.mu.Unlock()

	// Transition the previous render's retained tree at the START of this
	// render. Doing it at the lastTree assignment sites would alias the wire
	// output on the first render.
	diff.TransitionToStreamMode(t.lastTree)

	isFirstRender := !t.hasInitialTree

	// Build tree based on render type
	var tree *treeNode
	var treeErr error

	if isFirstRender {
		// Extract content from wrapper for consistent caching
		var contentToCache string
		if t.wrapperID != "" {
			contentToCache = compat.ExtractTemplateContent(currentHTML, t.wrapperID)
		} else {
			contentToCache = currentHTML
		}

		t.lastHTML = contentToCache
		tree = t.generateInitialTreeWithoutRegistry(dataWithLvt, contentToCache)
	} else {
		// Subsequent renders - use diffing approach
		tree, treeErr = t.generateDiffBasedTree(t.lastHTML, currentHTML, dataWithLvt)
	}

	if treeErr != nil {
		return nil, treeErr
	}

	return tree, nil
}

// =============================================================================
// Phase 4: Render - HTML Rendering
// =============================================================================

// renderHTML executes the template and returns the rendered HTML.
// This is the main entry point for Phase 4 (Render).
// It handles both full renders and update renders internally.
func (t *Template) renderHTML(data interface{}, messages map[string]string) (string, error) {
	if t.tmpl == nil {
		return "", fmt.Errorf("template not parsed")
	}

	if messages == nil {
		messages = make(map[string]string)
	}

	// Execute template with lvt context
	htmlBytes, err := context.ExecuteTemplateWithContext(t.tmpl, data, messages, t.config.DevMode, t.uploadRegistry)
	if err != nil {
		return "", err
	}

	result := string(htmlBytes)

	// Auto-inject aria-invalid="true" on form elements with validation errors.
	// Only applied in the HTTP response path (renderHTML). The WebSocket tree path
	// (buildTree → buildTreeWithCache) builds trees from the template AST, not rendered
	// HTML, so templates should use {{.lvt.AriaInvalid "field"}} for WebSocket updates.
	if fieldErrors := filterFieldErrors(messages); len(fieldErrors) > 0 {
		result = build.InjectAriaInvalid(result, fieldErrors)
	}

	return result, nil
}

// renderHTMLWithData executes the template with a pre-built data map.
// Used by buildTree to avoid duplicate reflection.
func (t *Template) renderHTMLWithData(dataWithLvt interface{}) (string, error) {
	if t.tmpl == nil {
		return "", fmt.Errorf("template not parsed")
	}

	htmlBytes, err := context.ExecuteTemplateWithDataMap(t.tmpl, dataWithLvt)
	if err != nil {
		return "", err
	}

	return string(htmlBytes), nil
}

// filterFieldErrors returns a map of only field errors (excludes flash messages).
// Returns nil if there are no field errors.
func filterFieldErrors(messages map[string]string) map[string]string {
	if messages == nil {
		return nil
	}
	var result map[string]string
	for k, v := range messages {
		if !strings.HasPrefix(k, context.FlashPrefix) {
			if result == nil {
				result = make(map[string]string)
			}
			result[k] = v
		}
	}
	return result
}

// =============================================================================
// Phase 5: Send - Response Writing
// =============================================================================

// sendResponse writes the response to the output writer.
// This is the main entry point for Phase 5 (Send).
// For Execute(): sends HTML
// For ExecuteUpdates(): sends JSON tree
func (t *Template) sendResponse(wr io.Writer, data interface{}) error {
	// Check if data is a TreeNode (JSON response) or HTML string
	switch v := data.(type) {
	case *treeNode:
		// Send JSON tree updates
		jsonBytes, err := send.MarshalOrderedJSON(v)
		if err != nil {
			return fmt.Errorf("JSON encoding failed: %w", err)
		}
		_, err = wr.Write(jsonBytes)
		return err
	case string:
		// Send HTML
		_, err := wr.Write([]byte(v))
		return err
	case []byte:
		// Send HTML bytes
		_, err := wr.Write(v)
		return err
	default:
		return fmt.Errorf("unsupported response type: %T", data)
	}
}
