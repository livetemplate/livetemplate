// Package livetemplate provides a library for building real-time, reactive web applications
// in Go with minimal code. It uses tree-based DOM diffing to send only what changed over
// WebSocket or HTTP, inspired by Phoenix LiveView.
//
// # Quick Start
//
// Define your application state as a Go struct that implements the Store interface:
//
//	type CounterState struct {
//	    Counter int `json:"counter"`
//	}
//
//	func (s *CounterState) Change(ctx *livetemplate.ActionContext) error {
//	    switch ctx.Action {
//	    case "increment":
//	        s.Counter++
//	    case "decrement":
//	        s.Counter--
//	    }
//	    return nil
//	}
//
// Create a template with `lvt-*` attributes for event binding:
//
//	<!-- counter.tmpl -->
//	<h1>Counter: {{.Counter}}</h1>
//	<button lvt-click="increment">+</button>
//	<button lvt-click="decrement">-</button>
//
// Wire it up in your main function:
//
//	func main() {
//	    state := &CounterState{Counter: 0}
//	    tmpl := livetemplate.New("counter")
//	    http.Handle("/", tmpl.Handle(state))
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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate/internal/context"
	"github.com/livetemplate/livetemplate/internal/diff"
	"github.com/livetemplate/livetemplate/internal/observe"
	"github.com/livetemplate/livetemplate/internal/session"
	"github.com/livetemplate/livetemplate/internal/signature"
	"github.com/livetemplate/livetemplate/pubsub"
)

// Config holds template configuration options
type Config struct {
	Upgrader               *websocket.Upgrader
	SessionStore           SessionStore
	Authenticator          Authenticator      // User authentication and session grouping
	PubSubBroadcaster      pubsub.Broadcaster // Optional: for distributed broadcasting across instances
	AllowedOrigins         []string           // Allowed WebSocket origins (empty = allow all in dev, restrict in prod)
	WebSocketDisabled      bool
	LoadingDisabled        bool          // Disables automatic loading indicator on page load
	TemplateFiles          []string      // If set, overrides auto-discovery
	IgnoreTemplateDirs     []string      // Additional directories to ignore during auto-discovery
	DevMode                bool          // Development mode - use local client library instead of CDN
	MaxConnections         int64         // Maximum total connections (0 = unlimited)
	MaxConnectionsPerGroup int64         // Maximum connections per group (0 = unlimited)
	MessageRateLimit       float64       // Messages per second per connection (0 = unlimited, default 10)
	MessageRateBurst       int           // Burst capacity for rate limiting (default 20)
	CookieMaxAge           time.Duration // Session cookie max age (default: 1 year)
}

// Template represents a live template with caching and tree-based optimization capabilities.
// It provides an API similar to html/template.Template but with additional ExecuteUpdates method
// for generating tree-based updates that can be efficiently transmitted to clients.
type Template struct {
	name            string
	templateStr     string
	tmpl            *template.Template
	wrapperID       string
	funcs           template.FuncMap
	mu              sync.RWMutex // Protects mutable state fields below
	lastData        interface{}
	lastHTML        string
	lastTree        *TreeNode // Store previous tree segments for comparison
	initialTree     *TreeNode
	hasInitialTree  bool
	lastFingerprint string                   // Fingerprint of the last generated tree for change detection
	keyGen          *keyGenerator                     // Per-template key generation for wrapper approach
	config          Config                            // Template configuration
	registry        *signature.ClientStructureRegistry // Track structure signatures sent to client (Phase 2)
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
type UpdateResponse struct {
	Tree interface{}       `json:"tree"` // Opaque tree update (internal format)
	Meta *ResponseMetadata `json:"meta,omitempty"`
}

// ResponseMetadata contains information about the action that generated the update
type ResponseMetadata struct {
	Success bool              `json:"success"` // true if no validation errors
	Errors  map[string]string `json:"errors"`  // field errors
	Action  string            `json:"action,omitempty"`
}

// Option is a functional option for configuring a Template
type Option func(*Config)

// WithParseFiles specifies template files to parse, overriding auto-discovery
func WithParseFiles(files ...string) Option {
	return func(c *Config) {
		c.TemplateFiles = files
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

func New(name string, opts ...Option) *Template {
	// Default configuration
	config := Config{
		Upgrader: &websocket.Upgrader{
			// Secure default: same-origin only
			// This will be replaced with origin-aware check after options are applied
			CheckOrigin: nil, // Will be set after applying options
		},
		SessionStore:     NewMemorySessionStore(),
		Authenticator:    &AnonymousAuthenticator{}, // Default: browser-based session grouping
		MessageRateLimit: 10.0,                      // Default: 10 messages/sec
		MessageRateBurst: 20,                        // Default: burst of 20
		CookieMaxAge:     365 * 24 * time.Hour,      // Default: 1 year
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
	log.Printf("livetemplate.New(%q): DevMode=%v", name, config.DevMode)

	tmpl := &Template{
		name:     name,
		keyGen:   newKeyGenerator(),
		config:   config,
		registry: signature.NewClientStructureRegistry(),
	}

	// Auto-discover and parse templates if not explicitly provided
	if len(config.TemplateFiles) == 0 {
		files, err := discoverTemplateFiles(config.IgnoreTemplateDirs)
		if err == nil && len(files) > 0 {
			if _, err := tmpl.ParseFiles(files...); err != nil {
				log.Printf("Warning: failed to parse template files: %v", err)
			}
		}
	} else {
		if _, err := tmpl.ParseFiles(config.TemplateFiles...); err != nil {
			log.Printf("Warning: failed to parse template files: %v", err)
		}
	}

	return tmpl
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
		keyGen:      newKeyGenerator(),
		config:      config,                       // Preserve configuration
		registry:    signature.NewClientStructureRegistry(), // Fresh registry for new session
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

// Parse parses text as a template body for the template t.
// This matches the signature of html/template.Template.Parse().
func (t *Template) Parse(text string) (*Template, error) {
	// Normalize template spacing to handle formatter-added spaces
	// This prevents issues when formatters add spaces like "{{ range" instead of "{{range"
	text = normalizeTemplateSpacing(text)

	// Determine if this is a full HTML document
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = generateRandomID()

	// First, parse WITHOUT wrapper to check if flattening is needed
	baseTemplate := template.New(t.name)
	if len(t.funcs) > 0 {
		baseTemplate = baseTemplate.Funcs(t.funcs)
	}
	tmpl, err := baseTemplate.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	// Check if template uses composition features and flatten if needed
	if hasTemplateComposition(tmpl) {
		// Flatten the template to resolve all {{define}}/{{template}}/{{block}}
		flattenedStr, err := flattenTemplate(tmpl)
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
		templateContent = injectWrapperDiv(text, t.wrapperID, t.config.LoadingDisabled)
	} else {
		// For standalone templates, wrap the entire content
		loadingAttr := ""
		if !t.config.LoadingDisabled {
			loadingAttr = ` data-lvt-loading="true"`
		}
		templateContent = fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>`, t.wrapperID, loadingAttr, text)
	}

	// Parse the template with wrapper for execution
	wrappedTemplate := template.New(t.name)
	if len(t.funcs) > 0 {
		wrappedTemplate = wrappedTemplate.Funcs(t.funcs)
	}
	tmpl, err = wrappedTemplate.Parse(templateContent)
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
	text := normalizeTemplateSpacing(string(content))

	// Determine if this is a full HTML document
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = generateRandomID()

	// First, parse WITHOUT wrapper to check if flattening is needed
	baseTemplate := template.New(t.name)
	if len(t.funcs) > 0 {
		baseTemplate = baseTemplate.Funcs(t.funcs)
	}
	tmpl, err := baseTemplate.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
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

	// Now that all files are parsed, check if we need to flatten
	if hasTemplateComposition(tmpl) {
		// Flatten the complete template set to resolve all {{define}}/{{template}}/{{block}}
		flattenedStr, err := flattenTemplate(tmpl)
		if err != nil {
			return nil, fmt.Errorf("template flattening failed: %w", err)
		}

		// Store flattened version for tree generation (WITHOUT wrapper)
		text = flattenedStr
	}

	// Now add wrapper to the (possibly flattened) template for execution
	var templateContent string
	if isFullHTML {
		// Inject wrapper div around body content
		templateContent = injectWrapperDiv(text, t.wrapperID, t.config.LoadingDisabled)
	} else {
		// For standalone templates, wrap the entire content
		loadingAttr := ""
		if !t.config.LoadingDisabled {
			loadingAttr = ` data-lvt-loading="true"`
		}
		templateContent = fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>`, t.wrapperID, loadingAttr, text)
	}

	// Parse the template with wrapper for execution
	wrappedTemplate := template.New(t.name)
	if len(t.funcs) > 0 {
		wrappedTemplate = wrappedTemplate.Funcs(t.funcs)
	}
	tmpl, err = wrappedTemplate.Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template with wrapper: %w", err)
	}

	// Store the template text for tree generation (flattened if it had composition)
	t.templateStr = text
	t.tmpl = tmpl

	// Validate that tree generation works with this template
	if err := t.validateTreeGeneration(); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	return t, nil
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

// Execute applies a parsed template to the specified data object,
// writing the output to wr. The template is rendered as a complete HTML page
// with wrapper injection for full HTML documents.
//
// Phase 1: For full HTML documents (containing <!DOCTYPE html> or <html>),
// the body content is automatically wrapped in a div with a randomly generated data-lvt-id.
// Phase 2: The complete HTML (with wrapper) is rendered and written to wr.
//
// Optional errors parameter provides error context for template via lvt namespace.
func (t *Template) Execute(wr io.Writer, data interface{}, errors ...map[string]string) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	var errMap map[string]string
	if len(errors) > 0 {
		errMap = errors[0]
	}
	if errMap == nil {
		errMap = make(map[string]string)
	}

	// Execute the template once and reuse the result for both output and caching
	// This eliminates 2x performance cost from double execution
	htmlBytes, err := context.ExecuteTemplateWithContext(t.tmpl, data, errMap, t.config.DevMode)
	if err != nil {
		return err
	}
	_, err = wr.Write(htmlBytes)
	if err != nil {
		return err
	}

	// Initialize caching state for future ExecuteUpdates calls
	// Reuse htmlBytes from first execution (no need to execute again)
	currentHTML := string(htmlBytes)

	// Extract content from wrapper for consistent caching
	var contentToCache string
	if t.wrapperID != "" {
		contentToCache = extractTemplateContent(currentHTML, t.wrapperID)
	} else {
		contentToCache = currentHTML
	}

	// Set up caching state and generate initial tree (protected by mutex)
	t.mu.Lock()
	t.lastData = data
	t.lastHTML = contentToCache

	// Generate and cache initial tree structure
	_, treeErr := t.generateInitialTree(currentHTML, data)
	t.mu.Unlock()

	if treeErr != nil {
		// Don't fail if tree generation fails, just skip caching
		return nil
	}

	return nil
}

// ExecuteUpdates generates a tree structure of static and dynamic content
// that can be used by JavaScript clients to update changed parts efficiently.
//
// Caching behavior:
// - First call: Returns complete tree with static structure ("s" key) and dynamic values
// - Subsequent calls: Returns only dynamic values that have changed (cache-aware)
//
// Tree generation phases:
// 1. Compile time: Template is analyzed to separate static/dynamic parts
// 2. Runtime: Dynamic parts are hydrated with data and compared with previous state
//
// Optional errors parameter provides error context for template via lvt namespace.
func (t *Template) ExecuteUpdates(wr io.Writer, data interface{}, errors ...map[string]string) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	var errMap map[string]string
	if len(errors) > 0 {
		errMap = errors[0]
	}

	tree, err := t.generateTreeInternalWithErrors(data, errMap)
	if err != nil {
		return fmt.Errorf("tree generation failed: %w", err)
	}

	// Convert tree to ordered JSON with readable HTML (no escape sequences)
	jsonBytes, err := marshalOrderedJSON(tree)
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	_, err = wr.Write(jsonBytes)
	return err
}

// generateTreeInternalWithErrors is the internal implementation that returns TreeNode with error context
func (t *Template) generateTreeInternalWithErrors(data interface{}, errors map[string]string) (*TreeNode, error) {
	// Acquire write lock to protect mutable state
	t.mu.Lock()
	defer t.mu.Unlock()

	// Initialize key generator if needed (but don't reset - keys should increment globally)
	if t.keyGen == nil {
		t.keyGen = newKeyGenerator()
	}

	// Convert data to include lvt context for consistent template execution
	dataWithLvt := t.addLvtToData(data, errors)

	// Load existing key mappings from previous render if available
	if t.lastTree != nil {
		t.loadExistingKeyMappings(t.lastTree)
	}

	// Execute template with current data and errors
	currentHTML, err := t.executeTemplateWithErrors(data, errors)
	if err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}

	// First render - no previous state
	if t.lastData == nil {
		// Extract content from wrapper for consistent caching
		var contentToCache string
		if t.wrapperID != "" {
			contentToCache = extractTemplateContent(currentHTML, t.wrapperID)
		} else {
			contentToCache = currentHTML
		}

		t.lastData = dataWithLvt
		t.lastHTML = contentToCache
		return t.generateInitialTree(currentHTML, dataWithLvt)
	}

	// Subsequent renders - use diffing approach
	return t.generateDiffBasedTree(t.lastHTML, currentHTML, t.lastData, dataWithLvt)
}

// addLvtToData converts data to include lvt context
func (t *Template) addLvtToData(data interface{}, errors map[string]string) interface{} {
	if errors == nil {
		errors = make(map[string]string)
	}

	// Use the same logic as executeTemplateWithContext to convert data
	lvtContext := context.NewTemplateContext(errors, t.config.DevMode)

	templateData := make(map[string]interface{})
	templateData["lvt"] = lvtContext

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)

			if !field.IsExported() {
				continue
			}

			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
					fieldName = jsonTag[:commaIdx]
				} else if jsonTag != "-" {
					fieldName = jsonTag
				}
			}
			templateData[fieldName] = val.Field(i).Interface()
			templateData[field.Name] = val.Field(i).Interface()
		}
	} else if val.Kind() == reflect.Map {
		for _, key := range val.MapKeys() {
			templateData[key.String()] = val.MapIndex(key).Interface()
		}
	}

	return templateData
}

// executeTemplateWithErrors executes the template with given data and errors for lvt context
func (t *Template) executeTemplateWithErrors(data interface{}, errors map[string]string) (string, error) {
	// Always use executeTemplateWithContext to ensure lvt namespace is available
	if errors == nil {
		errors = make(map[string]string)
	}

	// Execute with lvt context
	htmlBytes, err := context.ExecuteTemplateWithContext(t.tmpl, data, errors, t.config.DevMode)
	if err != nil {
		return "", err
	}
	return string(htmlBytes), nil
}

// markAllStructuresAsSeen recursively traverses a tree and marks all structures in the registry.
// This should be called after generating the initial tree to record what the client has received.
func (t *Template) markAllStructuresAsSeen(node *TreeNode, basePath string) {
	if node == nil || t.registry == nil {
		return
	}

	// Mark the current node's structure
	// This is critical: we mark the NODE itself (which includes range info if present)
	// The signature system will detect if it's a range and hash the statics
	t.registry.MarkSeen(basePath, node)

	// Recursively mark all dynamics
	for key, value := range node.Dynamics {
		fieldPath := basePath
		if fieldPath != "" {
			fieldPath += "."
		}
		fieldPath += key

		// If dynamic value is a TreeNode, recursively mark it
		if childNode, ok := value.(*TreeNode); ok {
			t.markAllStructuresAsSeen(childNode, fieldPath)
		} else {
			// Mark scalar values
			t.registry.MarkSeen(fieldPath, value)
		}
	}

	// NOTE: We do NOT iterate over range items here.
	// The range construct itself is marked above (node has Range info).
	// Individual items are dynamic data, not structure.
}

// generateInitialTree creates tree with statics and dynamics for first render
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
func (t *Template) generateInitialTree(html string, data interface{}) (*TreeNode, error) {
	// Extract content from wrapper if we have one
	var contentToAnalyze string
	if t.wrapperID != "" {
		contentToAnalyze = extractTemplateContent(html, t.wrapperID)
	} else {
		contentToAnalyze = html
	}

	// Get the template source (with {{}} placeholders)
	// We need the template source, not rendered HTML, so parseTemplateToTree can identify dynamics
	var templateContent string
	if t.wrapperID != "" {
		// For templates with <body> tags, extract body content
		// For templates without <body> tags (including flattened templates), use template as-is
		bodyContent := extractTemplateBodyContent(t.templateStr)
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
	ctx := NewTreeGenerationContext()
	ctx.FuncMap = t.funcs
	tree, err := parseTemplateToTree(templateContent, data, t.keyGen, ctx)
	if err != nil {
		// parseTemplateToTree failed, falling back to HTML structure
		tree = t.createHTMLStructureBasedTree(contentToAnalyze)
	}

	// Cache the initial structure for future dynamics-only updates
	t.initialTree = tree
	t.hasInitialTree = true

	// Store complete tree as the baseline for comparison
	t.lastTree = tree

	// Calculate and store initial fingerprint for change detection
	t.lastFingerprint = calculateFingerprint(tree)

	// Mark all structures in the registry (Phase 2: Client Structure Registry)
	// This records what structures the client has seen from the initial render
	t.markAllStructuresAsSeen(tree, "")

	// Add fingerprint to tree for client-side tracking
	return addFingerprintToTree(tree), nil
}

// generateDiffBasedTree creates tree based on diff analysis
// NOTE: This method modifies template state. Caller must hold t.mu write lock.
func (t *Template) generateDiffBasedTree(oldHTML, newHTML string, oldData, newData interface{}) (*TreeNode, error) {
	// Extract content from wrapper if we have one for proper comparison
	var oldContent, newContent string
	if t.wrapperID != "" {
		oldContent = extractTemplateContent(oldHTML, t.wrapperID)
		newContent = extractTemplateContent(newHTML, t.wrapperID)
	} else {
		oldContent = oldHTML
		newContent = newHTML
	}

	// Generate new complete tree for comparison
	if t.hasInitialTree {
		// Generate complete tree with current data using the template instance's keyGen
		// to ensure consistent key mapping across renders
		// Don't strip scripts - they may contain template logic
		bodyContent := extractTemplateBodyContent(t.templateStr)
		templateContent := bodyContent

		// IMPORTANT: Always generate trees WITH statics for comparison purposes
		// The stripping happens in compareTreesAndGetChanges, not here
		// Using nil context defaults to including statics
		ctx := NewTreeGenerationContext()
		ctx.FuncMap = t.funcs
		newTree, err := parseTemplateToTree(templateContent, newData, t.keyGen, ctx)
		if err != nil {
			return nil, fmt.Errorf("tree generation failed: %w", err)
		}

		// Compare trees and get only changed dynamics
		// This function will strip statics appropriately based on client state
		changedTree := t.compareTreesAndGetChanges(t.lastTree, newTree)

		// If no changes, return empty TreeNode
		if !changedTree.HasStatics() && !changedTree.HasDynamics() && !changedTree.HasRange() {
			return NewTreeNode(), nil
		}

		// Update cached state for next comparison
		t.lastData = newData
		t.lastHTML = newContent
		t.lastTree = newTree

		return changedTree, nil
	}

	// Fallback to analyzing the change (shouldn't happen after first render)
	tree, err := t.analyzeChangeAndCreateTree(oldContent, newContent, oldData, newData)
	if err != nil {
		return nil, err
	}

	// Calculate and store fingerprint for the new tree
	newFingerprint := calculateFingerprint(tree)
	t.lastFingerprint = newFingerprint

	// Update cached state AFTER successful tree generation (use extracted content)
	t.lastData = newData
	t.lastHTML = newContent

	// Add fingerprint to tree for client-side tracking
	return addFingerprintToTree(tree), nil
}

// deepEqual delegates to diff.DeepEqual for test backward compatibility.
func deepEqual(a, b interface{}) bool {
	return diff.DeepEqual(a, b)
}

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree *TreeNode) *TreeNode {
	return t.compareTreesAndGetChangesWithContext(oldTree, newTree, false)
}

// compareTreesAndGetChangesWithContext compares trees with context about whether we're in a new structure
// insideNewStructure: true if we're inside a structure the client has never seen
func (t *Template) compareTreesAndGetChangesWithContext(oldTree, newTree *TreeNode, insideNewStructure bool) *TreeNode {
	// Calculate range matches once at the top level for the entire tree
	rangeMatches := diff.FindRangeConstructMatches(oldTree, newTree)
	return diff.CompareTreesAndGetChangesWithPath(oldTree, newTree, insideNewStructure, "", rangeMatches, t.registry)
}

// analyzeChangeAndCreateTree determines the best tree structure based on the type of change
func (t *Template) analyzeChangeAndCreateTree(oldHTML, newHTML string, _, _ interface{}) (*TreeNode, error) {
	// Find common prefix and suffix to understand change patterns
	commonPrefix := findCommonPrefix(oldHTML, newHTML)
	commonSuffix := findCommonSuffix(oldHTML, newHTML)

	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)

	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		// Use the same segmentation strategy as the HTML fallback to ensure
		// updates remain structurally consistent with initial renders.
		return t.createHTMLStructureBasedTree(newHTML), nil
	}

	staticOverlap := len(commonPrefix) + len(commonSuffix)
	if staticOverlap <= 2 {
		hasMarkupFragment := strings.Contains(commonPrefix, "<") || strings.Contains(commonPrefix, ">") ||
			strings.Contains(commonSuffix, "<") || strings.Contains(commonSuffix, ">")

		if hasMarkupFragment {
			return t.createHTMLStructureBasedTree(newHTML), nil
		}
	}

	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		dynamicPart := newHTML[changeStart:changeEnd]
		tree := NewTreeNodeWithStatics([]string{commonPrefix, commonSuffix})
		tree.SetDynamic("0", minifyHTML(dynamicPart))
		return tree, nil
	}

	// Default to full dynamic content
	return t.createHTMLStructureBasedTree(newHTML), nil
}

// createHTMLStructureBasedTree implements deterministic segmentation strategies for HTML content
func (t *Template) createHTMLStructureBasedTree(html string) *TreeNode {
	// Define block-level elements that create natural segment boundaries
	blockTags := []string{"<div", "<article", "<section", "<main", "<aside", "<nav", "<ul", "<ol", "<table"}

	// Find the positions of block elements
	var boundaries []int
	for _, tag := range blockTags {
		idx := 0
		for {
			pos := strings.Index(html[idx:], tag)
			if pos == -1 {
				break
			}
			boundaries = append(boundaries, idx+pos)
			idx = idx + pos + len(tag)
		}
	}

	// Sort boundaries
	if len(boundaries) > 0 {
		// Simple sort
		for i := 0; i < len(boundaries)-1; i++ {
			for j := i + 1; j < len(boundaries); j++ {
				if boundaries[i] > boundaries[j] {
					boundaries[i], boundaries[j] = boundaries[j], boundaries[i]
				}
			}
		}

		// Create segments based on boundaries
		const maxSegments = 8
		segmentSize := len(html) / maxSegments

		var statics []string
		var dynamics []interface{}
		lastPos := 0
		dynamicIndex := 0

		for i, boundary := range boundaries {
			// Only create a segment if it's large enough
			if boundary-lastPos > segmentSize || i == len(boundaries)-1 {
				if lastPos == 0 {
					// First segment is typically more static (head, nav, etc)
					statics = append(statics, html[lastPos:boundary])
				} else {
					// Create a dynamic segment
					statics = append(statics, "")
					dynamics = append(dynamics, html[lastPos:boundary])
					dynamicIndex++
				}
				lastPos = boundary
			}
		}

		// Add the final segment
		if lastPos < len(html) {
			statics = append(statics, "")
			dynamics = append(dynamics, html[lastPos:])
		}

		// Build the tree
		tree := NewTreeNodeWithStatics(statics)
		for i, dyn := range dynamics {
			// Minify HTML content if it's a string containing HTML
			if strDyn, ok := dyn.(string); ok && strings.Contains(strDyn, "<") {
				dyn = minifyHTML(strDyn)
			}
			tree.SetDynamic(fmt.Sprintf("%d", i), dyn)
		}

		// If we got reasonable segmentation, use it
		if len(statics) > 2 && len(dynamics) > 0 {
			return tree
		}
	}

	// Fallback to single segment strategy
	fallback := NewTreeNodeWithStatics([]string{"", ""})
	fallback.SetDynamic("0", minifyHTML(html))
	return fallback
}

// Helper functions for string analysis

// findCommonPrefix finds the longest common prefix between two strings
func findCommonPrefix(s1, s2 string) string {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}
	return s1[:minLen]
}

// findCommonSuffix finds the longest common suffix between two strings
func findCommonSuffix(s1, s2 string) string {
	len1, len2 := len(s1), len(s2)
	minLen := len1
	if len2 < minLen {
		minLen = len2
	}

	for i := 0; i < minLen; i++ {
		if s1[len1-1-i] != s2[len2-1-i] {
			return s1[len1-i:]
		}
	}
	return s1[len1-minLen:]
}

// marshalOrderedJSON marshals a tree to JSON with no HTML escaping
func marshalOrderedJSON(tree interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(tree)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}

// marshalValue marshals a value to JSON with no HTML escaping
func marshalValue(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(value)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}

// loadExistingKeyMappings loads existing key mappings from the last tree node
func (t *Template) loadExistingKeyMappings(lastTree *TreeNode) {
	if lastTree == nil {
		return
	}

	// Look for range data in the tree dynamics and load existing key mappings
	for _, value := range lastTree.Dynamics {
		// Check if this is a TreeNode with Range data
		if node, ok := value.(*TreeNode); ok {
			if node.HasRange() && node.Range != nil {
				t.keyGen.LoadExistingKeys(node.Range.Items)
			}
		}
	}
}

// Handle creates an http.Handler for the template with the given stores.
// For single store: actions like "increment", "decrement"
// For multiple stores: actions like "counterstate.increment", "userstate.logout"
// Store names are automatically derived from struct type names (case-insensitive matching).
func (t *Template) Handle(stores ...Store) LiveHandler {
	if len(stores) == 0 {
		panic("Handle requires at least one store")
	}

	// Build stores map with auto-derived names
	storesMap := make(Stores)
	isSingleStore := len(stores) == 1

	if isSingleStore {
		// Single store mode - use empty key
		storesMap[""] = stores[0]
	} else {
		// Multi-store mode - derive names from struct types
		for _, store := range stores {
			name := getStoreName(store)
			storesMap[name] = store
		}
	}

	// Create WebSocket upgrader with origin validation
	upgrader := t.config.Upgrader
	if len(t.config.AllowedOrigins) > 0 {
		// Custom origin validation when AllowedOrigins is set
		upgrader = &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					// Same-origin requests (no Origin header) are allowed
					return true
				}

				// Check if origin is in allowed list
				for _, allowed := range t.config.AllowedOrigins {
					if origin == allowed {
						return true
					}
				}

				log.Printf("WebSocket origin rejected: %s (not in allowed origins)", origin)
				return false
			},
		}
	}

	config := MountConfig{
		Template:               t,
		Stores:                 storesMap,
		IsSingleStore:          isSingleStore,
		Upgrader:               upgrader,
		SessionStore:           t.config.SessionStore,
		Authenticator:          t.config.Authenticator,
		PubSubBroadcaster:      t.config.PubSubBroadcaster,
		AllowedOrigins:         t.config.AllowedOrigins,
		WebSocketDisabled:      t.config.WebSocketDisabled,
		MaxConnections:         t.config.MaxConnections,
		MaxConnectionsPerGroup: t.config.MaxConnectionsPerGroup,
		CookieMaxAge:           t.config.CookieMaxAge,
	}

	limits := session.NewConnectionLimits(config.MaxConnections, config.MaxConnectionsPerGroup)
	metrics := observe.NewMetrics(slog.Default())
	metricsExporter := observe.NewPrometheusExporter(metrics, limits)

	handler := &liveHandler{
		config:          config,
		registry:        session.NewConnectionRegistry(),
		limits:          limits,
		metricsExporter: metricsExporter,
		shutdownChan:    make(chan struct{}),
	}

	// Start pub/sub subscriber if broadcaster is configured
	if config.PubSubBroadcaster != nil {
		go func() {
			log.Printf("LiveHandler: Starting pub/sub subscriber...")
			if err := config.PubSubBroadcaster.Subscribe(handler.handlePubSubMessage); err != nil {
				log.Printf("LiveHandler: Pub/sub subscriber error: %v", err)
			}
		}()
	}

	return handler
}

// validateTreeGeneration validates that tree generation works with this template
// Templates with {{define}}/{{block}}/{{template}} are now supported via automatic flattening
func (t *Template) validateTreeGeneration() error {
	// Template composition ({{define}}/{{block}}/{{template}}) is now supported
	// The tree generation process automatically flattens composite templates
	// No validation needed here - errors will be caught during flattening if they occur
	return nil
}

// getStoreName derives the store name from the struct type
func getStoreName(store Store) string {
	t := reflect.TypeOf(store)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name() // e.g., "CounterState", "UserState"
}
