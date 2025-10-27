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
// For complete documentation, see https://github.com/livefir/livetemplate
package livetemplate

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Config holds template configuration options
type Config struct {
	Upgrader          *websocket.Upgrader
	SessionStore      SessionStore
	Authenticator     Authenticator // User authentication and session grouping
	AllowedOrigins    []string      // Allowed WebSocket origins (empty = allow all in dev, restrict in prod)
	WebSocketDisabled bool
	LoadingDisabled   bool     // Disables automatic loading indicator on page load
	TemplateFiles     []string // If set, overrides auto-discovery
	DevMode           bool     // Development mode - use local client library instead of CDN
}

// Template represents a live template with caching and tree-based optimization capabilities.
// It provides an API similar to html/template.Template but with additional ExecuteUpdates method
// for generating tree-based updates that can be efficiently transmitted to clients.
type Template struct {
	name            string
	templateStr     string
	tmpl            *template.Template
	wrapperID       string
	mu              sync.RWMutex             // Protects mutable state fields below
	lastData        interface{}
	lastHTML        string
	lastTree        *TreeNode // Store previous tree segments for comparison
	initialTree     *TreeNode
	hasInitialTree  bool
	lastFingerprint string                   // Fingerprint of the last generated tree for change detection
	keyGen          *keyGenerator            // Per-template key generation for wrapper approach
	config          Config                   // Template configuration
	registry        *ClientStructureRegistry // Track structure signatures sent to client (Phase 2)
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
//   - WebSocket upgrader with permissive CheckOrigin
//   - In-memory session store
//   - Anonymous authenticator (browser-based session grouping)
//   - Auto-discovery enabled
//   - Loading indicator enabled
//   - Production mode (CDN client library)
//
// See the With* functions for available options.
func New(name string, opts ...Option) *Template {
	// Default configuration
	config := Config{
		Upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		SessionStore:  NewMemorySessionStore(),
		Authenticator: &AnonymousAuthenticator{}, // Default: browser-based session grouping
	}

	// Apply options
	for _, opt := range opts {
		opt(&config)
	}

	// Log DevMode configuration for debugging
	log.Printf("livetemplate.New(%q): DevMode=%v", name, config.DevMode)

	tmpl := &Template{
		name:     name,
		keyGen:   newKeyGenerator(),
		config:   config,
		registry: NewClientStructureRegistry(),
	}

	// Auto-discover and parse templates if not explicitly provided
	if len(config.TemplateFiles) == 0 {
		files, err := discoverTemplateFiles()
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
		keyGen:      newKeyGenerator(),
		config:      config,                       // Preserve configuration
		registry:    NewClientStructureRegistry(), // Fresh registry for new session
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
	tmpl, err := template.New(t.name).Parse(text)
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
	tmpl, err = template.New(t.name).Parse(templateContent)
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
	tmpl, err := template.New(t.name).Parse(text)
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
	tmpl, err = template.New(t.name).Parse(templateContent)
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

	// Execute the template with wrapper injection and lvt context
	htmlBytes, err := executeTemplateWithContext(t.tmpl, data, errMap, t.config.DevMode)
	if err != nil {
		return err
	}
	_, err = wr.Write(htmlBytes)
	if err != nil {
		return err
	}

	// Initialize caching state for future ExecuteUpdates calls
	// Execute template again to get HTML for caching
	currentHTML, execErr := t.executeTemplateWithErrors(data, errMap)
	if execErr != nil {
		// Don't fail the main Execute call if caching setup fails
		return nil
	}

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
	lvtContext := &TemplateContext{
		errors:  errors,
		DevMode: t.config.DevMode,
	}

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
	htmlBytes, err := executeTemplateWithContext(t.tmpl, data, errors, t.config.DevMode)
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
		newTree, err := parseTemplateToTree(templateContent, newData, t.keyGen)
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

// prepareTreeForClient prepares a tree node for transmission to client
// If clientHasStatics is true, removes statics/fingerprints to reduce wire size
// Also removes fields that become empty after stripping (empty strings or empty maps)
// This implements the specification requirement: "Updates MUST include ONLY changed dynamics, NO statics unless structure is new"
func prepareTreeForClient(node interface{}, clientHasStatics bool) interface{} {
	if !clientHasStatics {
		// Client doesn't have statics - send everything as-is
		return node
	}

	// Client has statics - remove them to reduce wire size
	switch v := node.(type) {
	case *TreeNode:
		// Create new TreeNode without statics or fingerprint
		result := &TreeNode{
			Dynamics: make(map[string]interface{}),
		}
		// Recursively prepare dynamics
		for k, val := range v.Dynamics {
			prepared := prepareTreeForClient(val, clientHasStatics)
			// Only include non-empty values
			if !isEmpty(prepared) {
				result.Dynamics[k] = prepared
			}
		}
		// Handle Range but without statics (client has them cached)
		if v.HasRange() {
			result.Range = &RangeData{Items: v.Range.Items}
		}
		// Preserve Metadata (needed for client to extract item keys)
		if v.Metadata != nil {
			result.Metadata = v.Metadata
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint (client has them cached)
			}
			prepared := prepareTreeForClient(val, clientHasStatics)
			// Only include non-empty values
			if !isEmpty(prepared) {
				result[k] = prepared
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			prepared := prepareTreeForClient(item, clientHasStatics)
			// Only include non-empty values
			if !isEmpty(prepared) {
				result = append(result, prepared)
			}
		}
		return result
	default:
		return v
	}
}

// isEmpty checks if a value is considered empty (empty string, empty map, empty slice)
func isEmpty(v interface{}) bool {
	switch val := v.(type) {
	case *TreeNode:
		return !val.HasStatics() && !val.HasDynamics() && !val.HasRange()
	case string:
		return val == ""
	case map[string]interface{}:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree *TreeNode) *TreeNode {
	return t.compareTreesAndGetChangesWithContext(oldTree, newTree, false)
}

// compareTreesAndGetChangesWithContext compares trees with context about whether we're in a new structure
// insideNewStructure: true if we're inside a structure the client has never seen
func (t *Template) compareTreesAndGetChangesWithContext(oldTree, newTree *TreeNode, insideNewStructure bool) *TreeNode {
	// Calculate range matches once at the top level for the entire tree
	rangeMatches := findRangeConstructMatches(oldTree, newTree)
	return t.compareTreesAndGetChangesWithPath(oldTree, newTree, insideNewStructure, "", rangeMatches)
}

// compareTreesAndGetChangesWithPath compares trees with path tracking for nested range matching
func (t *Template) compareTreesAndGetChangesWithPath(oldTree, newTree *TreeNode, insideNewStructure bool, currentPath string, rangeMatches map[string]string) *TreeNode {
	changes := &TreeNode{
		Dynamics: make(map[string]interface{}),
	}

	// CRITICAL FIX: Check if both trees ARE range constructs (top-level range template)
	// Example: {{range .Items}}<div>...</div>{{end}} where the entire tree is a range
	// OR if newTree is a range but oldTree isn't (range appearing for first time, e.g., from {{else}} clause)
	if oldTree != nil && newTree != nil && newTree.HasRange() && newTree.HasStatics() {
		// Case 1: Both are ranges and matched
		if oldTree.HasRange() && oldTree.HasStatics() {
			if _, isMatched := rangeMatches[currentPath]; isMatched {
				// Generate differential operations for the entire range
				// Never strip statics - they're needed for rendering new items
				diffOps := generateRangeDifferentialOperations(oldTree, newTree, false)

				if len(diffOps) > 0 {
					// Return the operations directly - the entire tree is the range
					// Always include statics - they're needed for prepend/append rendering
					result := &TreeNode{Dynamics: make(map[string]interface{})}
					result.Dynamics["d"] = diffOps
					result.Statics = newTree.Statics
					return result
				} else {
					// No operations generated - check for empty range cases
					if (newTree.Range == nil || len(newTree.Range.Items) == 0) && (oldTree.Range == nil || len(oldTree.Range.Items) == 0) {
						// Both empty, no change
						return &TreeNode{}
					}
					// Fallback: return the new tree
					return newTree
				}
			}
		} else {
			// Case 2: newTree is a range but oldTree isn't (range appearing for first time)
			// This happens when going from {{else}} clause to range content
			// Return the full new tree so client can replace the else content with range items
			return newTree
		}
	}

	// Handle nil trees
	if newTree == nil {
		return &TreeNode{}
	}

	// Compare dynamic segments
	for k, newValue := range newTree.Dynamics {
		// Build full path for this field
		fieldPath := k
		if currentPath != "" {
			fieldPath = currentPath + "." + k
		}

		var oldValue interface{}
		var exists bool
		if oldTree != nil {
			oldValue, exists = oldTree.GetDynamic(k)
		}
		if !exists {
			// Field is NEW compared to last update
			// If we're inside a new structure, client has never seen this, so include statics
			if insideNewStructure {
				changes.SetDynamic(k, newValue)
				continue
			}

			// Check if client has seen THIS EXACT structure at this path (Phase 2: Registry)
			// The registry tracks structure signatures, so it knows if the client has seen:
			// - This specific scalar type
			// - This specific conditional structure
			// - This specific range structure (with same statics hash)
			clientHasStructure := false
			if t.registry != nil {
				clientHasStructure = t.registry.HasSeen(fieldPath, newValue)
			}

			if clientHasStructure {
				// Client already has this structure's statics from initial render
				// Strip statics when sending
				// Need to handle both map[string]interface{} type and map[string]interface{}
				var newTreeNode map[string]interface{}
				var newIsTree bool

				if tn, ok := newValue.(map[string]interface{}); ok {
					newTreeNode = tn
					newIsTree = true
				} else if m, ok := newValue.(map[string]interface{}); ok {
					newTreeNode = m
					newIsTree = true
				}

				if newIsTree {
					stripped := prepareTreeForClient(newTreeNode, true)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes.SetDynamic(k, "")
					} else {
						changes.SetDynamic(k, stripped)
					}
				} else {
					changes.SetDynamic(k, newValue)
				}
			} else {
				// Client doesn't have this structure - send WITH statics
				// However, normalize empty tree nodes to empty strings for cleaner output
				if tn, ok := newValue.(map[string]interface{}); ok {
					stripped := prepareTreeForClient(tn, true)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes.SetDynamic(k, "")
					} else {
						changes.SetDynamic(k, newValue)
						// Track that we've now sent this structure (Phase 2: Registry)
						if t.registry != nil {
							t.registry.MarkSeen(fieldPath, newValue)
						}
					}
				} else if m, ok := newValue.(map[string]interface{}); ok {
					stripped := prepareTreeForClient(m, true)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes.SetDynamic(k, "")
					} else {
						changes.SetDynamic(k, newValue)
						// Track that we've now sent this structure (Phase 2: Registry)
						if t.registry != nil {
							t.registry.MarkSeen(fieldPath, newValue)
						}
					}
				} else {
					changes.SetDynamic(k, newValue)
				}
			}
		} else if !deepEqual(oldValue, newValue) {
			// Field exists but changed - need to determine what to send

			// Check if this field has a range construct match using full path
			if _, isRangeMatch := rangeMatches[fieldPath]; isRangeMatch {
				// The oldValue is already the old range construct we need!
				// No need to traverse the tree - we're already at the right position

				// Check if old value is ALSO a range construct
				// If oldValue is NOT a range (e.g., was empty-state div), this is first appearance
				// Never strip statics - they're needed for rendering new items in prepend/append operations
				// Generate differential operations for matched range constructs
				diffOps := generateRangeDifferentialOperations(oldValue, newValue, false)
				if len(diffOps) > 0 {
					// For nested ranges, set operations directly (not wrapped in TreeNode)
					// This matches the golden file expectations
					changes.SetDynamic(k, diffOps)
				} else {
					// No diff operations generated - use fallback
					// Check if both are empty ranges (no change needed)
					if isRangeConstruct(newValue) && !hasRangeItems(newValue) &&
						isRangeConstruct(oldValue) && !hasRangeItems(oldValue) {
						// Both empty ranges, no update needed
						continue
					}

					// Check if new value is an empty range (items→empty transition)
					// Send the empty range structure so client knows to clear items
					if isRangeConstruct(newValue) && !hasRangeItems(newValue) {
						// Send empty range with statics (client will clear items and keep structure)
						changes.SetDynamic(k, newValue)
					} else {
						// Regular fallback with statics included
						changes.SetDynamic(k, newValue)
					}
				}
			} else {
				// Check if both old and new values are TreeNodes (nested structures)
				var oldTreeNodePtr, newTreeNodePtr *TreeNode
				var oldIsTree, newIsTree bool

				// Check for TreeNode first
				if tn, ok := oldValue.(*TreeNode); ok {
					oldTreeNodePtr = tn
					oldIsTree = true
				}

				if tn, ok := newValue.(*TreeNode); ok {
					newTreeNodePtr = tn
					newIsTree = true
				}

				if oldIsTree && newIsTree {
					// Both are tree nodes - recursively compare them

					// Check if this is a fundamental structure change (not part of a range match)
					// If the structures are completely different, treat nested content as new
					_, isRangeMatch := rangeMatches[fieldPath]
					structureChanged := !isRangeMatch && !areStructuresSimilar(oldTreeNodePtr, newTreeNodePtr)

					// If structure fundamentally changed, send the full new tree with statics
					// This ensures client gets all the HTML needed for the new structure
					// EXCEPT when both old and new contain ranges - in that case use incremental operations
					oldHasRange := containsRangeConstruct(oldValue)
					newHasRange := containsRangeConstruct(newValue)

					if structureChanged && !(oldHasRange && newHasRange) {
						// Structure changed and this isn't just range item updates
						// This includes: non-range → non-range, non-range → range, range → non-range
						changes.SetDynamic(k, newValue)
					} else {
						// Structure similar, do normal diff
						nestedChanges := t.compareTreesAndGetChangesWithPath(oldTreeNodePtr, newTreeNodePtr, insideNewStructure || structureChanged, fieldPath, rangeMatches)
						if nestedChanges.HasDynamics() {
							// Use nested changes as-is - the recursive call already handled statics correctly
							// Don't strip again or we'll lose statics for NEW structures like ranges
							changes.SetDynamic(k, nestedChanges)
						} else {
							// No dynamic changes detected, but check if both are static-only and not equal
							// This handles the case where static content changed (e.g., conditional rendering)
							oldStripped := prepareTreeForClient(oldTreeNodePtr, true)
							newStripped := prepareTreeForClient(newTreeNodePtr, true)
							oldIsEmpty := isEmpty(oldStripped)
							newIsEmpty := isEmpty(newStripped)

							// If both strip to empty (both static-only) but the originals aren't equal,
							// the statics changed - send empty string to indicate change
							if oldIsEmpty && newIsEmpty && !deepEqual(oldTreeNodePtr, newTreeNodePtr) {
								changes.SetDynamic(k, "")
							}
						}
					}
				} else if newIsTree {
					// New value is a tree node but old wasn't (Phase 2: Registry)
					// Use registry to check if client has seen THIS EXACT structure at this path
					clientHasStructure := false
					if t.registry != nil {
						clientHasStructure = t.registry.HasSeen(fieldPath, newValue)
					}

					if clientHasStructure {
						// Strip statics since client has them cached
						stripped := prepareTreeForClient(newTreeNodePtr, true)
						// If stripping results in empty, send empty string
						if isEmpty(stripped) {
							changes.SetDynamic(k, "")
						} else {
							changes.SetDynamic(k, stripped)
						}
					} else {
						// Client doesn't have structure - send WITH statics
						changes.SetDynamic(k, newValue)
						// Track that we've now sent this structure (Phase 2: Registry)
						if t.registry != nil {
							t.registry.MarkSeen(fieldPath, newValue)
						}
					}
				} else {
					// At least one is a primitive value or type changed - send new value as-is
					changes.SetDynamic(k, newValue)
				}
			}
		}
	}

	return changes
}

// areStructuresSimilar checks if two tree structures are fundamentally similar
// Returns true if they have similar structure (same static keys), false if completely different
func areStructuresSimilar(oldTree, newTree *TreeNode) bool {
	if oldTree == nil || newTree == nil {
		return false
	}
	return areStructuresSimilarTreeNode(oldTree, newTree)
}

func areStructuresSimilarTreeNode(oldTree, newTree *TreeNode) bool {
	// Check if both have statics - if statics differ, structures are different
	oldHasS := oldTree.HasStatics()
	newHasS := newTree.HasStatics()

	if oldHasS != newHasS {
		return false // One has statics, other doesn't
	}

	if oldHasS && newHasS {
		oldS := oldTree.Statics
		newS := newTree.Statics

		if len(oldS) != len(newS) {
			return false
		}

		// If statics are different, it's a different structure
		for i := range oldS {
			if oldS[i] != newS[i] {
				return false
			}
		}

		// Special case: Check if this is a conditional wrapper with empty statics
		// Conditionals are wrapped as {"s": ["", ""], "0": branchTree}
		// If both have empty statics and a single "0" child, compare the child structures
		if len(oldS) == 2 && oldS[0] == "" && oldS[1] == "" &&
			len(newS) == 2 && newS[0] == "" && newS[1] == "" {
			// Check if both have exactly one dynamic child "0"
			oldChild, oldHasChild := oldTree.GetDynamic("0")
			newChild, newHasChild := newTree.GetDynamic("0")

			if oldHasChild && newHasChild {
				// This looks like conditional wrappers - recursively compare children
				if oldChildNode, ok := oldChild.(*TreeNode); ok {
					if newChildNode, ok := newChild.(*TreeNode); ok {
						// Recursively check if the child structures are similar
						return areStructuresSimilarTreeNode(oldChildNode, newChildNode)
					}
				}
			}
		}
	}

	// Check if both are range constructs
	oldIsRange := oldTree.HasRange()
	newIsRange := newTree.HasRange()

	if oldIsRange != newIsRange {
		return false // One is range, other isn't
	}

	return true
}

// findRangeConstructMatches finds range constructs in both trees and matches them by content signature
// Returns a map of newField -> oldField for range constructs that represent the same template construct
func findRangeConstructMatches(oldTree, newTree *TreeNode) map[string]string {
	matches := make(map[string]string)

	// Handle nil trees
	if oldTree == nil || newTree == nil {
		return matches
	}

	// Find all range constructs in both trees
	oldRanges := findRangeConstructs(oldTree)
	newRanges := findRangeConstructs(newTree)

	// Match range constructs by their static template signature
	for newField, newRange := range newRanges {
		newSignature := getRangeSignature(newRange)

		matched := false
		for oldField, oldRange := range oldRanges {
			oldSignature := getRangeSignature(oldRange)

			// If signatures match, this is the same template construct
			if newSignature == oldSignature && newSignature != "" {
				matches[newField] = oldField
				matched = true
				break // Each new range should match at most one old range
			}
		}

		// FALLBACK: If no match found and one side has empty signature (empty range),
		// AND there's only one range in each tree at the same position, match by position
		if !matched && len(newRanges) == 1 && len(oldRanges) == 1 {
			// Single range in both trees at same position - must be the same construct
			for oldField := range oldRanges {
				if newField == oldField {
					matches[newField] = oldField
					break
				}
			}
		}
	}

	return matches
}

// findRangeConstructs finds all range constructs in a tree, recursively searching nested structures
func findRangeConstructs(tree *TreeNode) map[string]interface{} {
	if tree == nil {
		return make(map[string]interface{})
	}
	return findRangeConstructsRecursive(tree, "")
}

// findRangeConstructsRecursive finds range constructs with path tracking
func findRangeConstructsRecursive(tree *TreeNode, path string) map[string]interface{} {
	ranges := make(map[string]interface{})

	if tree == nil {
		return ranges
	}

	// CRITICAL FIX: Check if the tree ITSELF is a range construct
	// This handles top-level ranges like: {{range .Items}}...{{end}}
	// where the entire tree has Range field set
	if tree.HasRange() && tree.HasStatics() {
		ranges[path] = tree
		// Don't recurse into range internals - treat the range as an atomic unit
		return ranges
	}

	// Tree is not a range, search for ranges as field values in dynamics
	for field, value := range tree.Dynamics {
		// Build the full path to this field
		fieldPath := field
		if path != "" {
			fieldPath = path + "." + field
		}

		if isRangeConstruct(value) {
			ranges[fieldPath] = value
		} else {
			// Recursively search nested tree nodes
			if nestedTree, ok := value.(*TreeNode); ok {
				// Merge nested ranges into our map
				nestedRanges := findRangeConstructsRecursive(nestedTree, fieldPath)
				for k, v := range nestedRanges {
					ranges[k] = v
				}
			}
		}
	}

	return ranges
}

// getRangeSignature creates a signature for a range construct based on its static template structure
// This signature should be the same for the same template construct regardless of data
func getRangeSignature(rangeValue interface{}) string {
	// Check if value is a TreeNode with statics
	if node, ok := rangeValue.(*TreeNode); ok {
		if node.HasStatics() {
			return fmt.Sprintf("%v", node.Statics)
		}
		return ""
	}

	// Fallback: check for map representation (for compatibility during migration)
	rangeMap, ok := rangeValue.(map[string]interface{})
	if !ok {
		return ""
	}

	// Use the static parts ("s") as the signature since they represent the template structure
	staticParts, exists := rangeMap["s"]
	if !exists {
		return ""
	}

	// Convert static parts to a string signature
	return fmt.Sprintf("%v", staticParts)
}

// deepEqual compares two values deeply
func deepEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// isRangeConstruct checks if a value is a range construct (has "d" and "s" keys)
func isRangeConstruct(value interface{}) bool {
	// Check if value is a TreeNode with Range field
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && node.HasStatics()
	}

	// Fallback: check for map representation (for compatibility during migration)
	if valueMap, ok := value.(map[string]interface{}); ok {
		_, hasD := valueMap["d"]
		_, hasS := valueMap["s"]
		// Both "d" (data array) and "s" (statics array) must be present
		return hasD && hasS
	}

	return false
}

// isConditionalWrapper checks if a value is a conditional branch wrapper
// Conditional wrappers have format: {"s": ["value"]} or {"s": ["value"], "f": "fingerprint"}
// They represent the content of an if/else branch
// func isConditionalWrapper(value interface{}) bool {
// 	var valueMap map[string]interface{}
// 	var ok bool
//
// 	if tn, isTN := value.(map[string]interface{}); isTN {
// 		valueMap = tn
// 		ok = true
// 	} else if vm, isVM := value.(map[string]interface{}); isVM {
// 		valueMap = vm
// 		ok = true
// 	}
//
// 	if !ok {
// 		return false
// 	}
//
// 	// Must have "s" key
// 	sValue, hasS := valueMap["s"]
// 	if !hasS {
// 		return false
// 	}
//
// 	// Must NOT have "d" key (that would be a range, not a conditional)
// 	if _, hasD := valueMap["d"]; hasD {
// 		return false
// 	}
//
// 	// Check if "s" is an array with exactly 1 element
// 	if sArray, ok := sValue.([]interface{}); ok {
// 		if len(sArray) != 1 {
// 			return false
// 		}
// 		// The single element should be a string (the branch content)
// 		if _, isString := sArray[0].(string); !isString {
// 			return false
// 		}
// 	} else if sStringArray, ok := sValue.([]string); ok {
// 		if len(sStringArray) != 1 {
// 			return false
// 		}
// 	} else {
// 		return false
// 	}
//
// 	// Only "s" and optionally "f" (fingerprint) should be present
// 	for key := range valueMap {
// 		if key != "s" && key != "f" {
// 			return false
// 		}
// 	}
//
// 	return true
// }

// unwrapConditionalValue extracts the value from a conditional wrapper
// Returns the unwrapped value and true if successful, or original value and false if not a wrapper
// func unwrapConditionalValue(value interface{}) (interface{}, bool) {
// 	if !isConditionalWrapper(value) {
// 		return value, false
// 	}
//
// 	var valueMap map[string]interface{}
// 	if tn, ok := value.(map[string]interface{}); ok {
// 		valueMap = tn
// 	} else if vm, ok := value.(map[string]interface{}); ok {
// 		valueMap = vm
// 	} else {
// 		return value, false
// 	}
//
// 	sValue := valueMap["s"]
// 	if sArray, ok := sValue.([]interface{}); ok {
// 		return sArray[0], true
// 	} else if sStringArray, ok := sValue.([]string); ok {
// 		return sStringArray[0], true
// 	}
//
// 	return value, false
// }

// hasRangeItems checks if a range construct has any items in its data array
// Returns true only if value is a range AND has at least one item
// This is used to determine if the client has seen item rendering templates
func hasRangeItems(value interface{}) bool {
	// Check if value is a TreeNode with Range and items
	if node, ok := value.(*TreeNode); ok {
		return node.HasRange() && len(node.Range.Items) > 0
	}

	// Fallback: check for map representation
	if valueMap, ok := value.(map[string]interface{}); ok {
		if d, hasD := valueMap["d"]; hasD {
			if dArray, ok := d.([]interface{}); ok {
				return len(dArray) > 0
			}
		}
	}

	return false
}

// containsRangeConstruct recursively checks if a tree node or any of its children contains a range construct
// This is used to detect when conditional wrappers contain ranges, to avoid sending full range arrays
func containsRangeConstruct(value interface{}) bool {
	// Check if this value itself is a range
	if isRangeConstruct(value) {
		return true
	}

	// Check TreeNode dynamics recursively
	if node, ok := value.(*TreeNode); ok {
		for _, v := range node.Dynamics {
			if containsRangeConstruct(v) {
				return true
			}
		}
		return false
	}

	// Fallback: check for map representation
	if valueMap, ok := value.(map[string]interface{}); ok {
		// Recursively check all children (skip "s" and "f" keys)
		for k, v := range valueMap {
			if k == "s" || k == "f" {
				continue
			}
			if containsRangeConstruct(v) {
				return true
			}
		}
	}

	return false
}

// findKeyPositionFromStatics parses the statics array to find which position contains the key
func findKeyPositionFromStatics(statics interface{}) int {
	// Priority order for key attributes (same as server-side)
	keyAttrs := []string{`data-lvt-key="`, `data-key="`, `key="`, `id="`}

	// Try []interface{} first
	if staticsArr, ok := statics.([]interface{}); ok {
		for i, static := range staticsArr {
			if staticStr, ok := static.(string); ok {
				// Check for any of the key attributes in priority order
				for _, keyAttr := range keyAttrs {
					if strings.Contains(staticStr, keyAttr) {
						// The next position after this static contains the key value
						return i
					}
				}
			}
		}
		return 0 // Not found, default to 0
	}

	// Try []string
	if staticsArr, ok := statics.([]string); ok {
		for i, staticStr := range staticsArr {
			// Check for any of the key attributes in priority order
			for _, keyAttr := range keyAttrs {
				if strings.Contains(staticStr, keyAttr) {
					// The next position after this static contains the key value
					return i
				}
			}
		}
		return 0 // Not found, default to 0
	}

	return 0 // Unknown type, default to position 0 for backwards compatibility
}

// getItemKey extracts the key from a range item using the statics structure
func getItemKey(item interface{}, statics interface{}) (string, bool) {
	// Handle TreeNode items
	if itemNode, ok := item.(*TreeNode); ok {
		// First, check for reserved auto-generated key field
		if autoKey, exists := itemNode.GetDynamic("_k"); exists {
			if keyStr, ok := autoKey.(string); ok {
				return keyStr, true
			}
		}

		keyPos := findKeyPositionFromStatics(statics)
		keyPosStr := fmt.Sprintf("%d", keyPos)

		if key, exists := itemNode.GetDynamic(keyPosStr); exists {
			if keyStr, ok := key.(string); ok {
				return keyStr, true
			}
		}

		// If no explicit key found, generate a content-based hash
		// This ensures items have stable keys even without template key attributes
		return generateItemHash(itemNode), true
	}

	return "", false
}

// generateItemHash creates a stable hash for a range item based on its content
// This is used when no explicit key attribute is provided in the template
func generateItemHash(item interface{}) string {
	// Handle TreeNode
	if itemNode, ok := item.(*TreeNode); ok {
		// Create a canonical JSON representation for hashing
		// Sort keys to ensure deterministic ordering
		keys := make([]string, 0, len(itemNode.Dynamics))
		for k := range itemNode.Dynamics {
			// Skip internal/reserved fields
			if k != "_k" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		// Build canonical representation
		var parts []string
		for _, k := range keys {
			val, _ := itemNode.GetDynamic(k)
			valJSON, _ := json.Marshal(val)
			parts = append(parts, fmt.Sprintf("%s:%s", k, string(valJSON)))
		}

		// Hash the canonical representation
		content := strings.Join(parts, "|")
		hasher := md5.New()
		hasher.Write([]byte(content))
		hash := hex.EncodeToString(hasher.Sum(nil))

		// Return first 12 characters for compactness
		if len(hash) >= 12 {
			return hash[:12]
		}
		return hash
	}

	return ""
}

// extractItemKeys extracts the keys from a slice of range items using the statics structure
func extractItemKeys(items []interface{}, statics interface{}) []string {
	var keys []string
	for _, item := range items {
		// Items are now *TreeNode
		if itemNode, ok := item.(*TreeNode); ok {
			if key, ok := getItemKey(itemNode, statics); ok {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// detectPositionField finds the field containing positional display like "#0", "#1", etc.
func detectPositionField(itemsByKey map[string]interface{}) string {
	positionPattern := regexp.MustCompile(`^#\d+`)

	for _, item := range itemsByKey {
		if itemNode, ok := item.(*TreeNode); ok {
			for field, value := range itemNode.Dynamics {
				if strValue, ok := value.(string); ok {
					if positionPattern.MatchString(strValue) {
						return field
					}
				}
			}
		}
		break
	}
	return ""
}

// isPureReordering checks if the items are the same but just in different order
func isPureReordering(oldItems, newItems []interface{}, oldKeys, newKeys []string, statics interface{}) bool {
	// Must have same number of items
	if len(oldKeys) != len(newKeys) {
		return false
	}

	// Check if keys are the same (just different order)
	oldKeySet := make(map[string]bool)
	newKeySet := make(map[string]bool)

	for _, k := range oldKeys {
		oldKeySet[k] = true
	}
	for _, k := range newKeys {
		newKeySet[k] = true
	}

	// If key sets don't match, it's not pure reordering
	if len(oldKeySet) != len(newKeySet) {
		return false
	}
	for k := range oldKeySet {
		if !newKeySet[k] {
			return false
		}
	}

	// Now check if the items with same keys have identical content
	oldItemsByKey := make(map[string]interface{})
	newItemsByKey := make(map[string]interface{})

	for _, item := range oldItems {
		if key, ok := getItemKey(item, statics); ok {
			oldItemsByKey[key] = item
		}
	}

	for _, item := range newItems {
		if key, ok := getItemKey(item, statics); ok {
			newItemsByKey[key] = item
		}
	}

	// Detect position field by finding field with pattern like "#0", "#1", etc.
	positionField := detectPositionField(oldItemsByKey)

	// Compare each item's content (excluding position-dependent fields)
	for key, oldItem := range oldItemsByKey {
		newItem, exists := newItemsByKey[key]
		if !exists {
			return false
		}

		// Compare items excluding position field (field contains "#0:", "#1:", etc.)
		oldItemNode, ok1 := oldItem.(*TreeNode)
		newItemNode, ok2 := newItem.(*TreeNode)

		if !ok1 || !ok2 {
			// If we can't compare as TreeNodes, fall back to full comparison
			if !deepEqual(oldItem, newItem) {
				return false
			}
			continue
		}

		// Find key position to skip it in comparison
		keyPos := findKeyPositionFromStatics(statics)
		keyPosStr := fmt.Sprintf("%d", keyPos)

		// Compare all fields except position field and key field
		for field, oldValue := range oldItemNode.Dynamics {
			// Skip position field (contains positional display like "#0:")
			// Skip key field (determined from statics)
			if field == positionField || field == keyPosStr {
				continue
			}

			newValue, exists := newItemNode.GetDynamic(field)
			if !exists || !deepEqual(oldValue, newValue) {
				return false
			}
		}

		// Also check that new item doesn't have extra fields (except position and key)
		for field := range newItemNode.Dynamics {
			if field == positionField || field == keyPosStr {
				continue
			}
			if _, exists := oldItemNode.GetDynamic(field); !exists {
				return false
			}
		}
	}

	// Check if order actually changed
	for i := range oldKeys {
		if oldKeys[i] != newKeys[i] {
			return true // Same items, different order = pure reordering
		}
	}

	// Same items, same order = no change
	return false
}

// generateRangeDifferentialOperations generates differential operations for range constructs
// stripStatics: if true, removes "s" keys from operations (client has cached them)
// if false, keeps "s" keys (client hasn't seen this structure yet)
func generateRangeDifferentialOperations(oldValue, newValue interface{}, stripStatics bool) []interface{} {
	var operations []interface{}
	var oldItems, newItems []interface{}
	var statics interface{}
	var metadata map[string]interface{}

	// Try to extract TreeNode first
	if oldNode, ok := oldValue.(*TreeNode); ok {
		if oldNode.HasRange() && oldNode.Range != nil {
			oldItems = oldNode.Range.Items
			statics = oldNode.Statics
		} else {
			return operations
		}
	} else {
		return operations
	}

	if newNode, ok := newValue.(*TreeNode); ok {
		if newNode.HasRange() && newNode.Range != nil {
			newItems = newNode.Range.Items
			// IMPORTANT: For empty→items transition, use newNode.Statics (the item template)
			// oldNode.Statics will be empty/nil for empty ranges
			if len(oldItems) == 0 && len(newItems) > 0 {
				statics = newNode.Statics // Use new statics for first items
			} else if staticsSlice, ok := statics.([]string); ok && len(staticsSlice) == 0 {
				statics = newNode.Statics // Fallback if old statics empty
			}
			// Extract metadata for empty→items transitions
			if newNode.Metadata != nil {
				metadata = map[string]interface{}{
					"idKey": newNode.Metadata.IDKey,
				}
			}
		} else {
			return operations
		}
	} else {
		return operations
	}

	// First, check if this is a pure reordering (same items, different order)
	oldKeys := extractItemKeys(oldItems, statics)
	newKeys := extractItemKeys(newItems, statics)

	if isPureReordering(oldItems, newItems, oldKeys, newKeys, statics) {
		// Generate ordering operation
		return []interface{}{[]interface{}{"o", newKeys}}
	}

	// Create maps for easy lookup by keys
	oldItemsByKey := make(map[string]interface{})
	newItemsByKey := make(map[string]interface{})

	// Map old items by their auto-generated keys
	for _, item := range oldItems {
		if key, ok := getItemKey(item, statics); ok {
			oldItemsByKey[key] = item
		}
	}

	// Map new items by their auto-generated keys
	for _, item := range newItems {
		if key, ok := getItemKey(item, statics); ok {
			newItemsByKey[key] = item
		}
	}

	// Find removed items (in old but not in new)
	// Sort keys to ensure deterministic order
	sortedOldKeys := make([]string, 0, len(oldItemsByKey))
	for key := range oldItemsByKey {
		sortedOldKeys = append(sortedOldKeys, key)
	}
	sort.Strings(sortedOldKeys)

	for _, key := range sortedOldKeys {
		if _, exists := newItemsByKey[key]; !exists {
			operations = append(operations, []interface{}{"r", key})
		}
	}

	// Find updated items (in both, but changed)
	// Sort keys to ensure deterministic order
	sortedNewKeys := make([]string, 0, len(newItemsByKey))
	for key := range newItemsByKey {
		sortedNewKeys = append(sortedNewKeys, key)
	}
	sort.Strings(sortedNewKeys)

	for _, key := range sortedNewKeys {
		newItem := newItemsByKey[key]
		if oldItem, exists := oldItemsByKey[key]; exists {
			// Compare items and generate update operation if different
			changes := compareRangeItemsForChanges(oldItem, newItem, statics)
			if len(changes) > 0 {
				// Debug: log what key we're using
				if key == "" {
					// Empty key suggests an issue - log the item
					_ = newItem // Placeholder to inspect in debugger
				}
				// Check if changes only contains empty values - if so, don't include changes
				hasNonEmptyChanges := false
				for _, v := range changes {
					if s, ok := v.(string); !ok || s != "" {
						hasNonEmptyChanges = true
						break
					}
				}
				if hasNonEmptyChanges {
					operations = append(operations, []interface{}{"u", key, changes})
				} else {
					// All changes are empty strings - use simple format without changes
					operations = append(operations, []interface{}{"u", key})
				}
			}
		}
	}

	// Smart insertion pattern detection for added items
	addedKeys := findNewItems(oldItems, newItems, statics)
	if len(addedKeys) > 0 {
		// Check if it's a complex pattern that should fall back to full state
		if isComplexInsertionPattern(addedKeys, oldItems, newItems, statics) {
			// Fall back to full state replacement - return empty operations to trigger fallback
			return operations
		}

		// SPECIAL CASE: If old range was empty, use 'a' (append) with statics and metadata
		// This is needed because client can't apply differential operations without range state
		if len(oldItems) == 0 {
			// Build array of items to append, stripping nested statics
			itemsToAppend := make([]interface{}, 0, len(newItems))
			for _, item := range newItems {
				itemsToAppend = append(itemsToAppend, prepareTreeForClient(item, true))
			}
			// Use 'a' operation with statics and metadata so client can initialize range state
			// Format: ['a', items, statics, metadata]
			// statics now correctly contains newNode.Statics (the item template) from above
			// metadata is extracted from newNode above
			if metadata != nil {
				operations = append(operations, []interface{}{"a", itemsToAppend, statics, metadata})
			} else {
				operations = append(operations, []interface{}{"a", itemsToAppend, statics})
			}
		} else {
			// Range has existing items - detect append/prepend/insert patterns

			// Check if all new items are at the start (prepend)
			if areAllItemsAtStart(addedKeys, newItems, statics) {
				itemsToPrepend := make([]interface{}, 0, len(addedKeys))
				for _, key := range addedKeys {
					if item, exists := newItemsByKey[key]; exists {
						// Strip nested statics from items (client has cached them)
						itemsToPrepend = append(itemsToPrepend, prepareTreeForClient(item, true))
					}
				}
				// Use 'p' operation for prepending (O(1) on client)
				// Format: ['p', items, statics] - statics describe how to render items
				operations = append(operations, []interface{}{"p", itemsToPrepend, statics})
			} else if areAllItemsAtEnd(addedKeys, oldItems, newItems, statics) {
				// Check if all new items are at the end (append)
				itemsToAppend := make([]interface{}, 0, len(addedKeys))
				for _, key := range addedKeys {
					if item, exists := newItemsByKey[key]; exists {
						// Strip nested statics from items (client has cached them)
						itemsToAppend = append(itemsToAppend, prepareTreeForClient(item, true))
					}
				}
				// Use 'a' operation for appending (O(1) on client)
				// Format: ['a', items, statics] - statics describe how to render items
				operations = append(operations, []interface{}{"a", itemsToAppend, statics})
			} else {
				// Individual insertions at specific positions
				for _, key := range addedKeys {
					if newItem, exists := newItemsByKey[key]; exists {
						// Find position for this specific item
						for i, item := range newItems {
							if itemKey, ok := getItemKey(item, statics); ok && itemKey == key {
								// Determine insertion position
								if i == 0 {
									// Item at start - use prepend for single item
									// Strip nested statics and include top-level statics
									strippedItem := prepareTreeForClient(newItem, true)
									operations = append(operations, []interface{}{"p", []interface{}{strippedItem}, statics})
								} else {
									// Find the item before this one and use simplified insert
									if prevKey, ok := getItemKey(newItems[i-1], statics); ok {
										// Simplified insert: ['i', afterId, data] (no position param)
										// Strip nested statics from item
										strippedItem := prepareTreeForClient(newItem, true)
										operations = append(operations, []interface{}{"i", prevKey, strippedItem})
									}
								}
								break
							}
						}
					}
				}
			}
		}
	}

	// Strip statics from all operations if requested
	// Only strip if client already has the structure cached from initial tree
	if stripStatics {
		for i, op := range operations {
			operations[i] = prepareTreeForClient(op, true)
		}
	}

	return operations
}

// compareRangeItemsForChanges compares two range items and returns a map of field changes
func compareRangeItemsForChanges(oldItem, newItem interface{}, statics interface{}) map[string]interface{} {
	changes := make(map[string]interface{})

	oldItemNode, ok1 := oldItem.(*TreeNode)
	newItemNode, ok2 := newItem.(*TreeNode)

	if !ok1 || !ok2 {
		return changes
	}

	// Find key position to skip it
	keyPos := findKeyPositionFromStatics(statics)
	keyPosStr := fmt.Sprintf("%d", keyPos)

	// Compare each field (except the key field)
	for fieldKey, newValue := range newItemNode.Dynamics {
		if fieldKey == keyPosStr {
			continue // Skip the key field
		}

		oldValue, exists := oldItemNode.GetDynamic(fieldKey)
		if !exists || !deepEqual(oldValue, newValue) {
			// Strip statics from nested tree nodes since client already has them cached
			if newTreeNode, ok := newValue.(*TreeNode); ok {
				stripped := prepareTreeForClient(newTreeNode, true)
				// If stripping results in empty, check if this is a meaningful change
				if isEmpty(stripped) {
					// Check if old value would also strip to empty
					// If both old and new are static-only (strip to empty), don't send the change
					if exists {
						if oldTreeNode, ok := oldValue.(*TreeNode); ok {
							oldStripped := prepareTreeForClient(oldTreeNode, true)
							if isEmpty(oldStripped) {
								// Both old and new strip to empty - no meaningful change, skip it
								continue
							}
						}
					}
					// Old doesn't exist or had dynamics, send empty string to indicate removal of dynamics
					changes[fieldKey] = ""
				} else {
					changes[fieldKey] = stripped
				}
			} else {
				changes[fieldKey] = newValue
			}
		}
	}

	return changes
}

// Smart pattern detection functions for enhanced insertion operations

// findNewItems returns keys of items that exist in new but not in old
func findNewItems(oldItems, newItems []interface{}, statics interface{}) []string {
	oldKeys := make(map[string]bool)
	for _, item := range oldItems {
		if key, ok := getItemKey(item, statics); ok {
			oldKeys[key] = true
		}
	}

	var newKeys []string
	for _, item := range newItems {
		if key, ok := getItemKey(item, statics); ok {
			if !oldKeys[key] {
				newKeys = append(newKeys, key)
			}
		}
	}

	return newKeys
}

// areAllItemsAtEnd checks if all new items are appended at the end
// func areAllItemsAtEnd(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
// 	if len(newKeys) == 0 {
// 		return false
// 	}
//
// 	oldCount := len(oldItems)
// 	newCount := len(newItems)
//
// 	// Check if new items are exactly at the end positions
// 	for i, key := range newKeys {
// 		expectedIndex := oldCount + i
// 		if expectedIndex >= newCount {
// 			return false
// 		}
//
// 		// Get the item at this position in newItems
// 		if itemMap, ok := newItems[expectedIndex].(map[string]interface{}); ok {
// 			if keyStr, ok := getItemKey(itemMap, statics); ok {
// 				if keyStr != key {
// 					return false
// 				}
// 			} else {
// 				return false
// 			}
// 		} else {
// 			return false
// 		}
// 	}
//
// 	return true
// }

// areAllItemsAtSamePosition checks if all new items are inserted at the same position
// areAllItemsAtStart checks if all new items are at the beginning of the list (prepend)
func areAllItemsAtStart(newKeys []string, newItems []interface{}, statics interface{}) bool {
	if len(newKeys) == 0 {
		return false
	}

	// Check if all new keys are at the beginning of newItems
	for i, key := range newKeys {
		if i >= len(newItems) {
			return false
		}
		if itemMap, ok := newItems[i].(map[string]interface{}); ok {
			if itemKey, ok := getItemKey(itemMap, statics); ok {
				if itemKey != key {
					return false
				}
			} else {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// areAllItemsAtEnd checks if all new items are at the end of the list (append)
func areAllItemsAtEnd(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	if len(newKeys) == 0 || len(oldItems) == 0 {
		return false
	}

	// New items should be after all old items
	// Start index for new items should be len(oldItems)
	startIndex := len(newItems) - len(newKeys)

	// Verify that items before startIndex are all old items
	oldKeys := extractItemKeys(oldItems, statics)
	for i := 0; i < startIndex; i++ {
		if i >= len(newItems) {
			return false
		}
		if itemKey, ok := getItemKey(newItems[i], statics); ok {
			// Check if this key exists in oldKeys
			found := false
			for _, oldKey := range oldKeys {
				if oldKey == itemKey {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else {
			return false
		}
	}

	// Check if all new keys are contiguous at the end
	for i, key := range newKeys {
		index := startIndex + i
		if index >= len(newItems) {
			return false
		}
		if itemKey, ok := getItemKey(newItems[index], statics); ok {
			if itemKey != key {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// isComplexInsertionPattern checks if the insertion pattern is too complex for simple operations
func isComplexInsertionPattern(newKeys []string, oldItems, newItems []interface{}, statics interface{}) bool {
	// Consider it complex if there are more than 3 separate insertion points
	const maxInsertionPoints = 3

	if len(newKeys) == 0 {
		return false
	}

	insertionPoints := make(map[string]bool)

	for i, item := range newItems {
		if keyStr, ok := getItemKey(item, statics); ok {
			// Check if this is a new key
			for _, newKey := range newKeys {
				if newKey == keyStr {
					// Determine insertion point
					var insertionPoint string
					if i > 0 {
						if prevKeyStr, ok := getItemKey(newItems[i-1], statics); ok {
							insertionPoint = prevKeyStr + ":after"
						}
					} else {
						insertionPoint = "start"
					}
					insertionPoints[insertionPoint] = true
					break
				}
			}
		}
	}

	return len(insertionPoints) > maxInsertionPoints
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
		tree := NewTreeNodeWithStatics([]string{"", ""})
		tree.SetDynamic("0", minifyHTML(newHTML))
		return tree, nil
	}

	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		dynamicPart := newHTML[changeStart:changeEnd]
		tree := NewTreeNodeWithStatics([]string{commonPrefix, commonSuffix})
		tree.SetDynamic("0", minifyHTML(dynamicPart))
		return tree, nil
	}

	// Default to full dynamic content
	tree := NewTreeNodeWithStatics([]string{"", ""})
	tree.SetDynamic("0", minifyHTML(newHTML))
	return tree, nil
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
				t.keyGen.loadExistingKeys(node.Range.Items)
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
		Template:          t,
		Stores:            storesMap,
		IsSingleStore:     isSingleStore,
		Upgrader:          upgrader,
		SessionStore:      t.config.SessionStore,
		Authenticator:     t.config.Authenticator,
		AllowedOrigins:    t.config.AllowedOrigins,
		WebSocketDisabled: t.config.WebSocketDisabled,
	}

	return &liveHandler{
		config:   config,
		registry: NewConnectionRegistry(),
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

// getStoreName derives the store name from the struct type
func getStoreName(store Store) string {
	t := reflect.TypeOf(store)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name() // e.g., "CounterState", "UserState"
}
