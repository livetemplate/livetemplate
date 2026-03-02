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
// Create a template with `lvt-*` attributes for event binding:
//
//	<!-- counter.tmpl -->
//	<h1>Counter: {{.Count}}</h1>
//	<button lvt-click="increment">+</button>
//	<button lvt-click="decrement">-</button>
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
//   - Broadcaster: Share state updates across all connected clients
//   - SessionStore: Per-session state management
//
// # Advanced Features
//
//   - Multi-store pattern: Namespace multiple stores in one template
//   - Broadcasting: Real-time updates to all connected clients
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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/context"
	"github.com/livetemplate/livetemplate/internal/diff"
	"github.com/livetemplate/livetemplate/internal/discovery"
	"github.com/livetemplate/livetemplate/internal/keys"
	"github.com/livetemplate/livetemplate/internal/observe"
	"github.com/livetemplate/livetemplate/internal/parse"
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

// keyGenerator is an internal alias for keys.Generator.
// Used internally by Template for sequential key generation.
type keyGenerator = keys.Generator

// =============================================================================
// Configuration
// =============================================================================

// Config holds template configuration options
type Config struct {
	Upgrader               *websocket.Upgrader
	SessionStore           SessionStore
	Authenticator          Authenticator      // User authentication and session grouping
	PubSubBroadcaster      pubsub.Broadcaster // Optional: for distributed broadcasting across instances
	AllowedOrigins         []string           // Allowed WebSocket origins (empty = allow all in dev, restrict in prod)
	WebSocketDisabled      bool
	LoadingDisabled        bool                                // Disables automatic loading indicator on page load
	TemplateFiles          []string                            // If set, overrides auto-discovery
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
	name           string
	templateStr    string
	tmpl           *template.Template
	wrapperID      string
	funcs          template.FuncMap
	mu             sync.RWMutex // Protects mutable state fields below
	lastData       interface{}
	lastHTML       string
	lastTree       *treeNode // Store previous tree segments for comparison
	initialTree    *treeNode
	hasInitialTree bool
	keyGen         *keyGenerator // Per-template key generation for wrapper approach
	config         Config        // Template configuration
	uploadRegistry interface{}   // Upload registry for this connection (*upload.Registry)
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
	t.mu.Unlock()

	return t
}

// copyFuncMap creates a shallow copy of a FuncMap to prevent caller mutation.
func copyFuncMap(src template.FuncMap) template.FuncMap {
	if len(src) == 0 {
		return nil
	}

	clone := make(template.FuncMap, len(src))
	for name, fn := range src {
		clone[name] = fn
	}
	return clone
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

// WithParseFiles specifies template files to parse, overriding auto-discovery
func WithParseFiles(files ...string) Option {
	return func(c *Config) {
		c.TemplateFiles = files
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

// WithUpgrader sets a custom WebSocket upgrader
func WithUpgrader(upgrader *websocket.Upgrader) Option {
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
func WithAllowedOrigins(origins []string) Option {
	return func(c *Config) {
		c.AllowedOrigins = origins
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
		c.Upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
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
// Once configured, uploads are accessible via ActionContext during action handling.
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
// In your store's Change method, access uploads via ActionContext:
//
//	func (s *ProfileStore) Change(ctx *ActionContext) error {
//	    switch ctx.Action {
//	    case "save_profile":
//	        if ctx.HasUploads("avatar") {
//	            for _, entry := range ctx.GetCompletedUploads("avatar") {
//	                s.AvatarURL = moveToStorage(entry.TempPath)
//	            }
//	        }
//	    }
//	    return nil
//	}
func WithUpload(name string, config uploadtypes.UploadConfig) Option {
	return func(c *Config) {
		if c.UploadConfigs == nil {
			c.UploadConfigs = make(map[string]uploadtypes.UploadConfig)
		}
		c.UploadConfigs[name] = config
	}
}

// WithPubSubBroadcaster enables distributed broadcasting across multiple application instances.
//
// When set, Broadcast*, BroadcastToUsers, and BroadcastToGroup methods will publish messages
// to Redis Pub/Sub for distribution to all instances. Each instance subscribes to these messages
// and fans them out to its local connections.
//
// This is essential for horizontal scaling - without it, broadcasts only reach connections
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
//	<form method="POST" lvt-submit="add">
//	    <input type="hidden" name="lvt-action" value="add">
//	    <input type="text" name="title">
//	    <button type="submit">Add</button>
//	</form>
//
// The form works with both JavaScript (via lvt-submit) and without (via method="POST").
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
func createSecureOriginChecker(allowedOrigins []string, devMode bool) func(*http.Request) bool {
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

		// Extract scheme from request
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}

		// Check if origin matches scheme://host
		expectedOrigin := scheme + "://" + host
		return origin == expectedOrigin
	}
}

func New(name string, opts ...Option) (*Template, error) {
	// Get WebSocket buffer size default (50, or from LVT_WS_BUFFER_SIZE env var)
	wsBufferSize := 50
	if envSize := os.Getenv("LVT_WS_BUFFER_SIZE"); envSize != "" {
		var parsed int
		if n, err := fmt.Sscanf(envSize, "%d", &parsed); err == nil && n == 1 && parsed > 0 {
			wsBufferSize = parsed
		} else {
			slog.Warn("Invalid LVT_WS_BUFFER_SIZE environment variable, using default",
				slog.String("value", envSize), slog.Int("default", 50))
		}
	}

	// Default configuration
	config := Config{
		Upgrader: &websocket.Upgrader{
			// Secure default: same-origin only
			// This will be replaced with origin-aware check after options are applied
			CheckOrigin: nil, // Will be set after applying options
		},
		SessionStore:           NewMemorySessionStore(),
		Authenticator:          &AnonymousAuthenticator{}, // Default: browser-based session grouping
		MessageRateLimit:       10.0,                      // Default: 10 messages/sec
		MessageRateBurst:       20,                        // Default: burst of 20
		CookieMaxAge:           365 * 24 * time.Hour,      // Default: 1 year
		WebSocketBufferSize:    wsBufferSize,              // Default: 50 (or LVT_WS_BUFFER_SIZE env)
		ProgressiveEnhancement: true,                      // Default: enabled for non-JS form support
	}

	// Apply options
	for _, opt := range opts {
		opt(&config)
	}

	// Set secure CheckOrigin after options are applied
	if config.Upgrader.CheckOrigin == nil {
		config.Upgrader.CheckOrigin = createSecureOriginChecker(config.AllowedOrigins, config.DevMode)
	}

	// Log DevMode configuration for debugging
	slog.Debug("Template created",
		slog.String("name", name),
		slog.Bool("dev_mode", config.DevMode))

	tmpl := &Template{
		name:   name,
		keyGen: compat.NewKeyGenerator(),
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

	// Auto-discover and parse templates if not explicitly provided
	if len(config.TemplateFiles) == 0 {
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
	t.mu.RUnlock()

	// Cannot clone an executed html/template, must re-parse from source
	// Create a fresh template instance with the same configuration
	clone := &Template{
		name:        name,
		templateStr: templateStr,
		wrapperID:   wrapperID, // Share wrapper ID
		funcs:       copyFuncMap(t.funcs),
		keyGen:      compat.NewKeyGenerator(),
		config:      config, // Preserve configuration
		// Don't copy lastData, lastHTML, lastTree, etc. - start fresh
	}

	// Re-parse the template from source
	if templateStr != "" {
		_, err := clone.Parse(templateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to re-parse template: %w", err)
		}
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

	// Determine if this is a full HTML document
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

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

	return t.parseInternal(text, tmpl, isFullHTML)
}

// parseInternal handles the common logic for parsing templates:
// flattening, wrapper injection, final parsing, and validation.
func (t *Template) parseInternal(text string, baseTemplate *template.Template, isFullHTML bool) (*Template, error) {
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

	// Validate that tree generation works with this template
	// This ensures templates with {{define}}/{{block}} are caught during initialization
	if err := t.validateTreeGeneration(); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	return t, nil
}

// ParseFiles parses the named files and associates the resulting templates with t.
// This matches the signature of html/template.Template.ParseFiles().
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	// Read the first file as the main template
	content, err := os.ReadFile(filenames[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filenames[0], err)
	}

	// Use the first file's base name as template name if not already set
	if t.name == "" {
		t.name = filepath.Base(filenames[0])
	}

	// Normalize template spacing
	text := compat.NormalizeTemplateSpacing(string(content))

	// Determine if this is a full HTML document
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = compat.GenerateRandomID()

	// First, parse WITHOUT wrapper to check if flattening is needed
	// If component templates were already parsed, use that as the base
	// This allows component templates to be available during parsing and flattening
	var baseTemplate *template.Template
	if t.tmpl != nil {
		// Clone the existing template to preserve component definitions
		baseTemplate, err = t.tmpl.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone template with components: %w", err)
		}
		// Create a new named template within the cloned set
		baseTemplate = baseTemplate.New(t.name)
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

	// Parse additional files if provided (for template composition)
	if len(filenames) > 1 {
		for _, filename := range filenames[1:] {
			content, err := os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
			}

			// Parse additional templates into the same template set
			_, err = tmpl.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
			}
		}
	}

	return t.parseInternal(text, tmpl, isFullHTML)
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
			_, err = t.tmpl.Parse(string(content))
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

// generateTreeInternalWithErrors delegates to buildTree() for backward compatibility.
// DEPRECATED: Tests should use buildTree() directly. This wrapper exists only for
// existing test code and will be removed in a future version.
func (t *Template) generateTreeInternalWithErrors(data interface{}, messages map[string]string) (*treeNode, error) {
	return t.buildTree(data, messages)
}

// generateInitialTreeWithoutRegistry creates tree with statics and dynamics for first render.
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
func (t *Template) generateInitialTreeWithoutRegistry(html string, data interface{}) (*treeNode, error) {
	// Extract content from wrapper if we have one
	var contentToAnalyze string
	if t.wrapperID != "" {
		contentToAnalyze = compat.ExtractTemplateContent(html, t.wrapperID)
	} else {
		contentToAnalyze = html
	}

	// Get the template source (with {{}} placeholders)
	// We need the template source, not rendered HTML, so parseTemplateToTree can identify dynamics
	var templateContent string
	if t.wrapperID != "" {
		// For templates with <body> tags, extract body content
		// For templates without <body> tags (including flattened templates), use template as-is
		bodyContent := compat.ExtractTemplateBodyContent(t.templateStr)
		// extractTemplateBodyContent returns the full template if no <body> tag found
		// So we can use it directly - it will be the flattened template content without wrapper

		// Don't strip scripts - they may contain template logic like {{if .DevMode}}
		// that needs to be parsed correctly
		templateContent = bodyContent
	} else {
		templateContent = t.templateStr
	}

	// Use the original parser - it maintains the correct invariant and handles dynamics properly
	// First render: create context that includes all statics
	ctx := build.NewContext()
	ctx.FuncMap = t.funcs
	ctx.DevMode = t.config.DevMode
	tree, err := compat.ParseTemplateToTree(t.name, templateContent, data, t.keyGen, ctx)
	if err != nil {
		// parseTemplateToTree failed, falling back to HTML structure
		slog.Warn("Template parsing failed, falling back to HTML structure-based tree",
			slog.String("template", t.name),
			slog.Any("error", err))
		tree = build.CreateHTMLStructureBasedTree(contentToAnalyze)
	}

	// Cache the initial structure for future dynamics-only updates
	t.initialTree = tree
	t.hasInitialTree = true

	// Store complete tree as the baseline for comparison
	t.lastTree = tree

	// NOTE: Caller is responsible for calling markAllStructuresAsSeen outside the lock
	return tree, nil
}

// generateDiffBasedTree creates tree based on diff analysis
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
func (t *Template) generateDiffBasedTree(oldHTML, newHTML string, oldData, newData interface{}) (*treeNode, error) {
	// Extract content from wrapper if we have one for proper comparison
	var oldContent, newContent string
	if t.wrapperID != "" {
		oldContent = compat.ExtractTemplateContent(oldHTML, t.wrapperID)
		newContent = compat.ExtractTemplateContent(newHTML, t.wrapperID)
	} else {
		oldContent = oldHTML
		newContent = newHTML
	}

	// Generate new complete tree for comparison
	if t.hasInitialTree {
		// Generate complete tree with current data using the template instance's keyGen
		// to ensure consistent key mapping across renders
		// Don't strip scripts - they may contain template logic
		bodyContent := compat.ExtractTemplateBodyContent(t.templateStr)
		templateContent := bodyContent

		// IMPORTANT: Always generate trees WITH statics for comparison purposes
		// The stripping happens in compareTreesAndGetChanges, not here
		// Using nil context defaults to including statics
		ctx := build.NewContext()
		ctx.FuncMap = t.funcs
		ctx.DevMode = t.config.DevMode
		newTree, err := compat.ParseTemplateToTree(t.name, templateContent, newData, t.keyGen, ctx)
		if err != nil {
			return nil, fmt.Errorf("tree generation failed: %w", err)
		}

		// Compare trees and get only changed dynamics
		// This function will strip statics appropriately based on client state
		changedTree := t.compareTreesAndGetChanges(t.lastTree, newTree)

		// If no changes, return empty TreeNode
		if !changedTree.HasStatics() && !changedTree.HasDynamics() && !changedTree.HasRange() {
			return build.NewTreeNode(), nil
		}

		// Update cached state for next comparison
		t.lastData = newData
		t.lastHTML = newContent
		t.lastTree = newTree

		return changedTree, nil
	}

	// Fallback to analyzing the change (shouldn't happen after first render)
	tree, err := build.AnalyzeChangeAndCreateTree(oldContent, newContent)
	if err != nil {
		return nil, err
	}

	// Update cached state AFTER successful tree generation (use extracted content)
	t.lastData = newData
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

	// Create WebSocket upgrader with origin validation
	upgrader := t.config.Upgrader
	if len(t.config.AllowedOrigins) > 0 {
		upgrader = &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, allowed := range t.config.AllowedOrigins {
					if origin == allowed {
						return true
					}
				}
				slog.Warn("WebSocket origin rejected",
					slog.String("origin", origin))
				return false
			},
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
	}

	limits := session.NewConnectionLimits(mountCfg.MaxConnections, mountCfg.MaxConnectionsPerGroup)
	metrics := observe.NewMetrics(slog.Default())
	metricsExporter := observe.NewPrometheusExporter(metrics, limits)

	// Initialize upload factories (lazy, done once)
	initUploadFactories()

	// Create temp file manager for uploads
	tempFileManager, err := newUploadTempFileManager("")
	if err != nil {
		slog.Error("Failed to create temp file manager - uploads will not work",
			slog.Any("error", err))
	}

	handler := &liveHandler{
		config:          mountCfg,
		registry:        session.NewConnectionRegistry(),
		limits:          limits,
		metricsExporter: metricsExporter,
		tempFileManager: tempFileManager,
		shutdownChan:    make(chan struct{}),
	}

	// Wire up metrics to registry for WebSocket observability
	handler.registry.SetMetrics(metrics)

	// Start periodic sweep of stale HTTP template cache entries
	go handler.httpTemplateSweepLoop()

	// Start pub/sub subscriber if broadcaster is configured
	if mountCfg.PubSubBroadcaster != nil {
		go func() {
			slog.Info("Starting pub/sub subscriber")
			if err := mountCfg.PubSubBroadcaster.Subscribe(handler.handlePubSubMessage); err != nil {
				slog.Error("Pub/sub subscriber error",
					slog.Any("error", err))
			}
		}()

		if err := mountCfg.PubSubBroadcaster.SubscribeServerActions(handler.handleServerActionMessage); err != nil {
			slog.Error("Failed to subscribe to server actions",
				slog.Any("error", err))
		}
	}

	return handler
}

// HandleOption configures Handle behavior
type HandleOption func(*handleConfig)

type handleConfig struct {
	sessionStore SessionStore
}

// WithStore sets the session store for state persistence.
// Use this to configure Redis or other distributed stores.
func WithStore(store SessionStore) HandleOption {
	return func(c *handleConfig) {
		c.sessionStore = store
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
	// Phase 4: Render HTML (needed for tree building)
	// Do this outside the lock as it's CPU-intensive and doesn't modify shared state
	currentHTML, err := t.renderHTML(data, messages)
	if err != nil {
		return nil, err
	}

	// Convert data to include lvt context (with upload registry if set)
	dataWithLvt := context.AddLvtToData(data, messages, t.config.DevMode, t.uploadRegistry)

	// Note: We don't invalidate the expression cache here because:
	// 1. Cache keys include dataHash, so changed data naturally misses the cache
	// 2. Cache is intra-render optimization (expressions within a single template execution)
	// 3. Invalidating on every render would defeat the purpose of caching
	// The cache will naturally expire as data changes across renders.

	// Acquire lock once for all state reads/writes
	t.mu.Lock()
	defer t.mu.Unlock()

	// Initialize key generator if needed (defensive check)
	if t.keyGen == nil {
		t.keyGen = compat.NewKeyGenerator()
	}

	// Load existing key mappings if available
	if t.lastTree != nil {
		if err := keys.LoadExistingKeyMappings(t.keyGen, t.lastTree); err != nil {
			return nil, fmt.Errorf("failed to load existing key mappings: %w", err)
		}
	}

	// Determine if this is first render
	isFirstRender := t.lastData == nil

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

		t.lastData = dataWithLvt
		t.lastHTML = contentToCache
		tree, treeErr = t.generateInitialTreeWithoutRegistry(currentHTML, dataWithLvt)
	} else {
		// Subsequent renders - use diffing approach
		tree, treeErr = t.generateDiffBasedTree(t.lastHTML, currentHTML, t.lastData, dataWithLvt)
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

	return string(htmlBytes), nil
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
