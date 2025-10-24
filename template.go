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
	"strconv"
	"strings"

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
	lastData        interface{}
	lastHTML        string
	lastTree        treeNode // Store previous tree segments for comparison
	initialTree     treeNode
	hasInitialTree  bool
	lastFingerprint string              // Fingerprint of the last generated tree for change detection
	keyGen          *keyGenerator       // Per-template key generation for wrapper approach
	config          Config              // Template configuration
	analyzer        *TreeUpdateAnalyzer // Tree efficiency analyzer (enabled in DevMode)
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

	// Initialize tree analyzer (only enabled in DevMode)
	analyzer := NewTreeUpdateAnalyzer()
	analyzer.Enabled = config.DevMode

	tmpl := &Template{
		name:     name,
		keyGen:   newKeyGenerator(),
		config:   config,
		analyzer: analyzer,
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
	// Cannot clone an executed html/template, must re-parse from source
	// Create a fresh template instance with the same configuration
	analyzer := NewTreeUpdateAnalyzer()
	analyzer.Enabled = t.config.DevMode

	clone := &Template{
		name:        t.name,
		templateStr: t.templateStr,
		wrapperID:   t.wrapperID, // Share wrapper ID
		keyGen:      newKeyGenerator(),
		config:      t.config, // Preserve configuration
		analyzer:    analyzer,
		// Don't copy lastData, lastHTML, lastTree, etc. - start fresh
	}

	// Re-parse the template from source
	if t.templateStr != "" {
		_, err := clone.Parse(t.templateStr)
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

	// Set up caching state
	t.lastData = data
	t.lastHTML = contentToCache

	// Generate and cache initial tree structure
	_, treeErr := t.generateInitialTree(currentHTML, data)
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

	// Analyze tree for efficiency issues (only in DevMode)
	if t.analyzer != nil && t.analyzer.Enabled {
		t.analyzer.AnalyzeUpdate(tree, t.name, t.templateStr)
	}

	// Convert tree to ordered JSON with readable HTML (no escape sequences)
	jsonBytes, err := marshalOrderedJSON(tree)
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	_, err = wr.Write(jsonBytes)
	return err
}

// generateTreeInternalWithErrors is the internal implementation that returns treeNode with error context
func (t *Template) generateTreeInternalWithErrors(data interface{}, errors map[string]string) (treeNode, error) {
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

// generateInitialTree creates tree with statics and dynamics for first render
func (t *Template) generateInitialTree(html string, data interface{}) (treeNode, error) {
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
	tree, err := parseTemplateToTree(templateContent, data, t.keyGen)
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

	// Add fingerprint to tree for client-side tracking
	return addFingerprintToTree(tree), nil
}

// generateDiffBasedTree creates tree based on diff analysis
func (t *Template) generateDiffBasedTree(oldHTML, newHTML string, oldData, newData interface{}) (treeNode, error) {
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

		newTree, err := parseTemplateToTree(templateContent, newData, t.keyGen)
		if err != nil {
			return treeNode{}, fmt.Errorf("tree generation failed: %w", err)
		}

		// Compare trees and get only changed dynamics
		changedTree := t.compareTreesAndGetChanges(t.lastTree, newTree)

		// If no changes, return empty
		if len(changedTree) == 0 {
			return treeNode{}, nil
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

// stripStaticsRecursively removes all "s" and "f" keys from a tree node recursively
// Also removes fields that become empty after stripping (empty strings or empty maps)
func stripStaticsRecursively(node interface{}) interface{} {
	switch v := node.(type) {
	case treeNode:
		result := make(map[string]interface{})
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint
			}
			stripped := stripStaticsRecursively(val)
			// Only include non-empty values
			if !isEmpty(stripped) {
				result[k] = stripped
			}
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			if k == "s" || k == "f" {
				continue // Skip statics and fingerprint
			}
			stripped := stripStaticsRecursively(val)
			// Only include non-empty values
			if !isEmpty(stripped) {
				result[k] = stripped
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			stripped := stripStaticsRecursively(item)
			// Only include non-empty values
			if !isEmpty(stripped) {
				result = append(result, stripped)
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
	case string:
		return val == ""
	case treeNode:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree treeNode) treeNode {
	return t.compareTreesAndGetChangesWithContext(oldTree, newTree, false)
}

// compareTreesAndGetChangesWithContext compares trees with context about whether we're in a new structure
// insideNewStructure: true if we're inside a structure the client has never seen
func (t *Template) compareTreesAndGetChangesWithContext(oldTree, newTree treeNode, insideNewStructure bool) treeNode {
	// Calculate range matches once at the top level for the entire tree
	rangeMatches := findRangeConstructMatches(oldTree, newTree)
	return t.compareTreesAndGetChangesWithPath(oldTree, newTree, insideNewStructure, "", rangeMatches)
}

// compareTreesAndGetChangesWithPath compares trees with path tracking for nested range matching
func (t *Template) compareTreesAndGetChangesWithPath(oldTree, newTree treeNode, insideNewStructure bool, currentPath string, rangeMatches map[string]string) treeNode {
	changes := make(treeNode)

	// CRITICAL FIX: Check if both trees ARE range constructs (top-level range template)
	// Example: {{range .Items}}<div>...</div>{{end}} produces {"d": [...], "s": [...]}
	// In this case, the ENTIRE tree is the range, not a field within it
	if isRangeConstruct(oldTree) && isRangeConstruct(newTree) {
		// Check if this range is matched in rangeMatches at the current path
		if _, isMatched := rangeMatches[currentPath]; isMatched {
			// Generate differential operations for the entire range
			shouldStripStatics := hasRangeItems(oldTree)
			diffOps := generateRangeDifferentialOperations(oldTree, newTree, shouldStripStatics)

			if len(diffOps) > 0 {
				// Return the operations directly - the entire tree is the range
				// Wrap in a treeNode with "d" key to maintain expected format
				return treeNode{"d": diffOps}
			} else {
				// No operations generated - check for empty range cases
				if !hasRangeItems(newTree) && !hasRangeItems(oldTree) {
					// Both empty, no change
					return treeNode{}
				}
				// Fallback: return the new tree
				return newTree
			}
		}
	}

	// Compare dynamic segments (skip statics "s" and fingerprint "f")
	for k, newValue := range newTree {
		if k == "s" || k == "f" {
			continue // Skip static segments and fingerprint
		}

		// Build full path for this field
		fieldPath := k
		if currentPath != "" {
			fieldPath = currentPath + "." + k
		}

		oldValue, exists := oldTree[k]
		if !exists {
			// Field is NEW compared to last update
			// If we're inside a new structure, client has never seen this, so include statics
			if insideNewStructure {
				changes[k] = newValue
				continue
			}

			// Check if client has this EXACT structure from initial render
			// For range constructs, only strip statics if initial tree also had a range at this location
			clientHasStructure := false
			if t.hasInitialTree && t.fieldExistsInTree(k, t.initialTree) {
				if isRangeConstruct(newValue) {
					// For range constructs, check if initial tree ALSO has a range at this field
					// If initial tree had something else (like empty-state), client doesn't have range statics
					initialValue := t.getFieldValueFromTree(k, t.initialTree)
					clientHasStructure = isRangeConstruct(initialValue)
				} else {
					// For non-range structures, field existence is enough
					clientHasStructure = true
				}
			}

			if clientHasStructure {
				// Client already has this structure's statics from initial render
				// Strip statics when sending
				// Need to handle both treeNode type and map[string]interface{}
				var newTreeNode treeNode
				var newIsTree bool

				if tn, ok := newValue.(treeNode); ok {
					newTreeNode = tn
					newIsTree = true
				} else if m, ok := newValue.(map[string]interface{}); ok {
					newTreeNode = m
					newIsTree = true
				}

				if newIsTree {
					stripped := stripStaticsRecursively(newTreeNode)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes[k] = ""
					} else {
						changes[k] = stripped
					}
				} else {
					changes[k] = newValue
				}
			} else {
				// Client doesn't have this structure - send WITH statics
				// However, normalize empty tree nodes to empty strings for cleaner output
				if tn, ok := newValue.(treeNode); ok {
					stripped := stripStaticsRecursively(tn)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes[k] = ""
					} else {
						changes[k] = newValue
					}
				} else if m, ok := newValue.(map[string]interface{}); ok {
					stripped := stripStaticsRecursively(m)
					if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
						changes[k] = ""
					} else {
						changes[k] = newValue
					}
				} else {
					changes[k] = newValue
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
				// Only strip statics if BOTH old and new are range constructs AND old range has items
				// Empty ranges {"d": [], "s": [""]} have never shown item templates to client
				shouldStripStatics := isRangeConstruct(oldValue) && hasRangeItems(oldValue)

				// Generate differential operations for matched range constructs
				diffOps := generateRangeDifferentialOperations(oldValue, newValue, shouldStripStatics)
				if len(diffOps) > 0 {
					changes[k] = diffOps
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
						changes[k] = newValue
					} else if shouldStripStatics {
						// Regular fallback with statics stripped
						changes[k] = stripStaticsRecursively(newValue)
					} else {
						// Regular fallback with statics included
						changes[k] = newValue
					}
				}
			} else {
				// Check if both old and new values are tree nodes (nested structures)
				// Need to handle both treeNode type and map[string]interface{}
				var oldTreeNode, newTreeNode treeNode
				var oldIsTree, newIsTree bool

				// Try treeNode type first
				if tn, ok := oldValue.(treeNode); ok {
					oldTreeNode = tn
					oldIsTree = true
				} else if m, ok := oldValue.(map[string]interface{}); ok {
					oldTreeNode = m
					oldIsTree = true
				}

				if tn, ok := newValue.(treeNode); ok {
					newTreeNode = tn
					newIsTree = true
				} else if m, ok := newValue.(map[string]interface{}); ok {
					newTreeNode = m
					newIsTree = true
				}

				if oldIsTree && newIsTree {
					// Both are tree nodes - recursively compare them

					// Check if this is a fundamental structure change (not part of a range match)
					// If the structures are completely different, treat nested content as new
					_, isRangeMatch := rangeMatches[fieldPath]
					structureChanged := !isRangeMatch && !areStructuresSimilar(oldTreeNode, newTreeNode)

					// If structure fundamentally changed, send the full new tree with statics
					// This ensures client gets all the HTML needed for the new structure
					// EXCEPT when both old and new contain ranges - in that case use incremental operations
					oldHasRange := containsRangeConstruct(oldValue)
					newHasRange := containsRangeConstruct(newValue)

					if structureChanged && !(oldHasRange && newHasRange) {
						// Structure changed and this isn't just range item updates
						// This includes: non-range → non-range, non-range → range, range → non-range
						changes[k] = newValue
					} else {
						// Structure similar, do normal diff
						nestedChanges := t.compareTreesAndGetChangesWithPath(oldTreeNode, newTreeNode, insideNewStructure || structureChanged, fieldPath, rangeMatches)
						if len(nestedChanges) > 0 {
							// Use nested changes as-is - the recursive call already handled statics correctly
							// Don't strip again or we'll lose statics for NEW structures like ranges
							changes[k] = nestedChanges
						} else {
							// No dynamic changes detected, but check if both are static-only and not equal
							// This handles the case where static content changed (e.g., conditional rendering)
							oldStripped := stripStaticsRecursively(oldTreeNode)
							newStripped := stripStaticsRecursively(newTreeNode)
							oldIsEmpty := false
							newIsEmpty := false
							if m, ok := oldStripped.(map[string]interface{}); ok && len(m) == 0 {
								oldIsEmpty = true
							}
							if m, ok := newStripped.(map[string]interface{}); ok && len(m) == 0 {
								newIsEmpty = true
							}

							// If both strip to empty (both static-only) but the originals aren't equal,
							// the statics changed - send empty string to indicate change
							if oldIsEmpty && newIsEmpty && !deepEqual(oldTreeNode, newTreeNode) {
								changes[k] = ""
							}
						}
					}
				} else if newIsTree {
					// New value is a tree node but old wasn't
					// Check if client has this structure from initial render
					// IMPORTANT: Must check if initial value was ALSO a tree node, not just any value
					// (e.g., conditionals can go from "" to tree node - client doesn't have the tree statics)
					//
					// BUG FIX: For nested trees, we must check at the CURRENT field path,
					// not globally. A key "0" in a nested conditional is different from
					// key "0" at the top level. Use fieldPath to get the right initial value.
					clientHasStructure := false
					if t.hasInitialTree {
						// Get initial value at the CURRENT path, not just by key
						var initialValue interface{}
						if fieldPath == "" {
							// Top level - use key directly
							if t.fieldExistsInTree(k, t.initialTree) {
								initialValue = t.getFieldValueFromTree(k, t.initialTree)
							}
						} else {
							// Nested - check at the specific path
							// fieldPath is like "1" for nested, so check if initial tree has that path
							// and then check if that path's value has key k
							pathParts := strings.Split(fieldPath, ".")
							current := interface{}(t.initialTree)
							found := true
							for _, part := range pathParts {
								if tn, ok := current.(treeNode); ok {
									current = tn[part]
								} else if m, ok := current.(map[string]interface{}); ok {
									current = m[part]
								} else {
									found = false
									break
								}
							}
							if found {
								// Now check if current (which is the tree at fieldPath) has key k
								if tn, ok := current.(treeNode); ok {
									if val, exists := tn[k]; exists {
										initialValue = val
									}
								} else if m, ok := current.(map[string]interface{}); ok {
									if val, exists := m[k]; exists {
										initialValue = val
									}
								}
							}
						}

						// Check if initial value is also a tree node (not empty string or other primitive)
						if initialValue != nil {
							if tn, ok := initialValue.(treeNode); ok && len(tn) > 0 {
								clientHasStructure = true
							} else if m, ok := initialValue.(map[string]interface{}); ok && len(m) > 0 {
								clientHasStructure = true
							}
						}
					}

					if clientHasStructure {
						// Strip statics since client has them cached
						stripped := stripStaticsRecursively(newTreeNode)
						// If stripping statics results in an empty map, send empty string to match old behavior
						if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
							changes[k] = ""
						} else {
							changes[k] = stripped
						}
					} else {
						// Client doesn't have structure - send WITH statics
						changes[k] = newValue
					}
				} else {
					// At least one is a primitive value or type changed - send new value as-is
					changes[k] = newValue
				}
			}
		}
	}

	// Strip only the top-level "s" and "f" from the changes object
	delete(changes, "s")
	delete(changes, "f")

	return changes
}

// fieldExistsInTree checks if a field key exists at any level in the tree
func (t *Template) fieldExistsInTree(key string, tree treeNode) bool {
	if tree == nil {
		return false
	}

	// Direct check
	if _, exists := tree[key]; exists {
		return true
	}

	// Recursive check in nested structures
	for k, v := range tree {
		if k == "s" || k == "f" {
			continue
		}
		if nestedTree, ok := v.(map[string]interface{}); ok {
			if t.fieldExistsInTree(key, nestedTree) {
				return true
			}
		}
	}

	return false
}

// areStructuresSimilar checks if two tree structures are fundamentally similar
// Returns true if they have similar structure (same static keys), false if completely different
func areStructuresSimilar(oldTree, newTree treeNode) bool {
	// Check if both have statics - if statics differ, structures are different
	oldStatics, oldHasS := oldTree["s"]
	newStatics, newHasS := newTree["s"]

	if oldHasS != newHasS {
		return false // One has statics, other doesn't
	}

	if oldHasS && newHasS {
		// Try to get statics as either []string or []interface{}
		var oldS, newS []string
		var oldOK, newOK bool

		// Try []string first (most common case from tree_ast.go)
		if s, ok := oldStatics.([]string); ok {
			oldS = s
			oldOK = true
		} else if s, ok := oldStatics.([]interface{}); ok {
			// Convert []interface{} to []string
			oldS = make([]string, len(s))
			for i, v := range s {
				if str, ok := v.(string); ok {
					oldS[i] = str
				}
			}
			oldOK = true
		}

		if s, ok := newStatics.([]string); ok {
			newS = s
			newOK = true
		} else if s, ok := newStatics.([]interface{}); ok {
			// Convert []interface{} to []string
			newS = make([]string, len(s))
			for i, v := range s {
				if str, ok := v.(string); ok {
					newS[i] = str
				}
			}
			newOK = true
		}

		if !oldOK || !newOK || len(oldS) != len(newS) {
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
			oldChild, oldHasChild := oldTree["0"]
			newChild, newHasChild := newTree["0"]

			if oldHasChild && newHasChild {
				// This looks like conditional wrappers - recursively compare children
				oldChildTree, oldIsTree := oldChild.(treeNode)
				newChildTree, newIsTree := newChild.(treeNode)

				if !oldIsTree {
					if m, ok := oldChild.(map[string]interface{}); ok {
						oldChildTree = treeNode(m)
						oldIsTree = true
					}
				}

				if !newIsTree {
					if m, ok := newChild.(map[string]interface{}); ok {
						newChildTree = treeNode(m)
						newIsTree = true
					}
				}

				if oldIsTree && newIsTree {
					// Recursively check if the child structures are similar
					return areStructuresSimilar(oldChildTree, newChildTree)
				}
			}
		}
	}

	// Check if both are range constructs
	oldIsRange := isRangeConstruct(oldTree)
	newIsRange := isRangeConstruct(newTree)

	if oldIsRange != newIsRange {
		return false // One is range, other isn't
	}

	return true
}

// getFieldValueFromTree gets the value for a field key at any level in the tree
func (t *Template) getFieldValueFromTree(key string, tree treeNode) interface{} {
	if tree == nil {
		return nil
	}

	// Direct check
	if value, exists := tree[key]; exists {
		return value
	}

	// Recursive check in nested structures
	for k, v := range tree {
		if k == "s" || k == "f" {
			continue
		}
		if nestedTree, ok := v.(map[string]interface{}); ok {
			if value := t.getFieldValueFromTree(key, nestedTree); value != nil {
				return value
			}
		}
	}

	return nil
}

// findRangeConstructMatches finds range constructs in both trees and matches them by content signature
// Returns a map of newField -> oldField for range constructs that represent the same template construct
func findRangeConstructMatches(oldTree, newTree treeNode) map[string]string {
	matches := make(map[string]string)

	// Find all range constructs in both trees
	oldRanges := findRangeConstructs(oldTree)
	newRanges := findRangeConstructs(newTree)

	// Match range constructs by their static template signature
	for newField, newRange := range newRanges {
		newSignature := getRangeSignature(newRange)

		for oldField, oldRange := range oldRanges {
			oldSignature := getRangeSignature(oldRange)

			// If signatures match, this is the same template construct
			if newSignature == oldSignature {
				matches[newField] = oldField
				break // Each new range should match at most one old range
			}
		}
	}

	return matches
}

// findRangeConstructs finds all range constructs in a tree, recursively searching nested structures
func findRangeConstructs(tree treeNode) map[string]interface{} {
	return findRangeConstructsRecursive(tree, "")
}

// findRangeConstructsRecursive finds range constructs with path tracking
func findRangeConstructsRecursive(tree treeNode, path string) map[string]interface{} {
	ranges := make(map[string]interface{})

	// CRITICAL FIX: Check if the tree ITSELF is a range construct
	// This handles top-level ranges like: {{range .Items}}...{{end}}
	// where the entire tree is {"d": [...], "s": [...]}
	if isRangeConstruct(tree) {
		ranges[path] = tree
		// Don't recurse into range internals - treat the range as an atomic unit
		return ranges
	}

	// Tree is not a range, search for ranges as field values
	for field, value := range tree {
		if field == "s" || field == "f" {
			continue // Skip static segments and fingerprint
		}

		// Build the full path to this field
		fieldPath := field
		if path != "" {
			fieldPath = path + "." + field
		}

		if isRangeConstruct(value) {
			ranges[fieldPath] = value
		} else {
			// Recursively search nested tree nodes
			var nestedTree treeNode
			if tn, ok := value.(treeNode); ok {
				nestedTree = tn
			} else if m, ok := value.(map[string]interface{}); ok {
				nestedTree = m
			}

			if nestedTree != nil {
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
	// Try both treeNode and map[string]interface{} type assertions
	var valueMap map[string]interface{}
	var ok bool

	if tn, isTN := value.(treeNode); isTN {
		valueMap = tn
		ok = true
	} else if vm, isVM := value.(map[string]interface{}); isVM {
		valueMap = vm
		ok = true
	}

	if ok {
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
// 	if tn, isTN := value.(treeNode); isTN {
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
// 	if tn, ok := value.(treeNode); ok {
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
	var valueMap map[string]interface{}

	if tn, ok := value.(treeNode); ok {
		valueMap = tn
	} else if m, ok := value.(map[string]interface{}); ok {
		valueMap = m
	} else {
		return false
	}

	if d, hasD := valueMap["d"]; hasD {
		if dArray, ok := d.([]interface{}); ok {
			return len(dArray) > 0
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

	// Try to get as a map to check children
	var valueMap map[string]interface{}
	if tn, ok := value.(treeNode); ok {
		valueMap = tn
	} else if m, ok := value.(map[string]interface{}); ok {
		valueMap = m
	} else {
		return false
	}

	// Recursively check all children (skip "s" and "f" keys)
	for k, v := range valueMap {
		if k == "s" || k == "f" {
			continue
		}
		if containsRangeConstruct(v) {
			return true
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
func getItemKey(itemMap map[string]interface{}, statics interface{}) (string, bool) {
	// First, check for reserved auto-generated key field
	if autoKey, exists := itemMap["_k"]; exists {
		if keyStr, ok := autoKey.(string); ok {
			return keyStr, true
		}
	}

	keyPos := findKeyPositionFromStatics(statics)
	keyPosStr := fmt.Sprintf("%d", keyPos)

	if key, exists := itemMap[keyPosStr]; exists {
		if keyStr, ok := key.(string); ok {
			return keyStr, true
		}
	}

	// If no explicit key found, generate a content-based hash
	// This ensures items have stable keys even without template key attributes
	return generateItemHash(itemMap), true
}

// generateItemHash creates a stable hash for a range item based on its content
// This is used when no explicit key attribute is provided in the template
func generateItemHash(itemMap map[string]interface{}) string {
	// Create a canonical JSON representation for hashing
	// Sort keys to ensure deterministic ordering
	keys := make([]string, 0, len(itemMap))
	for k := range itemMap {
		// Skip internal/reserved fields
		if k != "_k" && k != "s" && k != "f" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// Build canonical representation
	var parts []string
	for _, k := range keys {
		val := itemMap[k]
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

// extractItemKeys extracts the keys from a slice of range items using the statics structure
func extractItemKeys(items []interface{}, statics interface{}) []string {
	var keys []string
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
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
		if itemMap, ok := item.(map[string]interface{}); ok {
			for field, value := range itemMap {
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
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				oldItemsByKey[key] = item
			}
		}
	}

	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				newItemsByKey[key] = item
			}
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
		oldItemMap, ok1 := oldItem.(map[string]interface{})
		newItemMap, ok2 := newItem.(map[string]interface{})

		if !ok1 || !ok2 {
			// If we can't compare as maps, fall back to full comparison
			if !deepEqual(oldItem, newItem) {
				return false
			}
			continue
		}

		// Find key position to skip it in comparison
		keyPos := findKeyPositionFromStatics(statics)
		keyPosStr := fmt.Sprintf("%d", keyPos)

		// Compare all fields except position field and key field
		for field, oldValue := range oldItemMap {
			// Skip position field (contains positional display like "#0:")
			// Skip key field (determined from statics)
			if field == positionField || field == keyPosStr {
				continue
			}

			newValue, exists := newItemMap[field]
			if !exists || !deepEqual(oldValue, newValue) {
				return false
			}
		}

		// Also check that new item doesn't have extra fields (except position and key)
		for field := range newItemMap {
			if field == positionField || field == keyPosStr {
				continue
			}
			if _, exists := oldItemMap[field]; !exists {
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

	// Try to extract map[string]interface{} from both treeNode and map[string]interface{} types
	var oldRange, newRange map[string]interface{}
	var ok1, ok2 bool

	// Handle oldValue - try treeNode first, then map[string]interface{}
	if tn, isTN := oldValue.(treeNode); isTN {
		oldRange = tn
		ok1 = true
	} else if m, isM := oldValue.(map[string]interface{}); isM {
		oldRange = m
		ok1 = true
	}

	// Handle newValue - try treeNode first, then map[string]interface{}
	if tn, isTN := newValue.(treeNode); isTN {
		newRange = tn
		ok2 = true
	} else if m, isM := newValue.(map[string]interface{}); isM {
		newRange = m
		ok2 = true
	}

	if !ok1 || !ok2 {
		// Type conversion failed
		return operations
	}

	// Extract old and new item arrays
	oldItems, ok1 := oldRange["d"].([]interface{})
	newItems, ok2 := newRange["d"].([]interface{})

	// Try alternative type assertion if the first one fails
	if !ok1 {
		if oldMaps, ok := oldRange["d"].([]map[string]interface{}); ok {
			oldItems = make([]interface{}, len(oldMaps))
			for i, m := range oldMaps {
				oldItems[i] = m
			}
			ok1 = true
		}
	}

	if !ok2 {
		if newMaps, ok := newRange["d"].([]map[string]interface{}); ok {
			newItems = make([]interface{}, len(newMaps))
			for i, m := range newMaps {
				newItems[i] = m
			}
			ok2 = true
		}
	}

	if !ok1 || !ok2 {
		// Item extraction failed
		// Debug: could examine keys and types here if needed
		return operations
	}

	// Comparing old items vs new items

	// Extract statics for key position lookup (both ranges have the same structure)
	statics := newRange["s"]

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
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				oldItemsByKey[key] = item
			}
		}
	}

	// Map new items by their auto-generated keys
	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				newItemsByKey[key] = item
			}
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
				operations = append(operations, []interface{}{"u", key, changes})
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

		// SPECIAL CASE: If old range was empty, use 'a' (append) with statics
		// This is needed because client can't apply differential operations without range state
		if len(oldItems) == 0 {
			// Build array of items to append
			itemsToAppend := append([]interface{}{}, newItems...)
			// Use 'a' operation with statics so client can initialize range state
			if !stripStatics {
				operations = append(operations, []interface{}{"a", itemsToAppend, statics})
			} else {
				operations = append(operations, []interface{}{"a", itemsToAppend})
			}
		} else {
			// Range has existing items, use 'i' (insert) operations
			// Check if all items are at the same position (single-point insertion)
			if isSamePosition, targetKey, position := areAllItemsAtSamePosition(addedKeys, oldItems, newItems, statics); isSamePosition {
				// Generate individual insert operations for each item
				for _, key := range addedKeys {
					if item, exists := newItemsByKey[key]; exists {
						if targetKey == "" {
							operations = append(operations, []interface{}{"i", nil, position, item})
						} else {
							operations = append(operations, []interface{}{"i", targetKey, position, item})
						}
					}
				}
			} else {
				// Multiple individual insertions at different positions
				for _, key := range addedKeys {
					if newItem, exists := newItemsByKey[key]; exists {
						// Find position for this specific item
						for i, item := range newItems {
							if itemMap, ok := item.(map[string]interface{}); ok {
								if itemKey, ok := getItemKey(itemMap, statics); ok && itemKey == key {
									// Determine insertion position using 'i' operation (spec-compliant)
									if i == 0 {
										operations = append(operations, []interface{}{"i", nil, "start", newItem})
									} else {
										// Find the item before this one
										if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
											if prevKey, ok := getItemKey(prevItem, statics); ok {
												operations = append(operations, []interface{}{"i", prevKey, "after", newItem})
											}
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
	}

	// Strip statics from all operations if requested
	// Only strip if client already has the structure cached from initial tree
	if stripStatics {
		for i, op := range operations {
			operations[i] = stripStaticsRecursively(op)
		}
	}

	return operations
}

// compareRangeItemsForChanges compares two range items and returns a map of field changes
func compareRangeItemsForChanges(oldItem, newItem interface{}, statics interface{}) map[string]interface{} {
	changes := make(map[string]interface{})

	oldItemMap, ok1 := oldItem.(map[string]interface{})
	newItemMap, ok2 := newItem.(map[string]interface{})

	if !ok1 || !ok2 {
		return changes
	}

	// Find key position to skip it
	keyPos := findKeyPositionFromStatics(statics)
	keyPosStr := fmt.Sprintf("%d", keyPos)

	// Compare each field (except the key field)
	for fieldKey, newValue := range newItemMap {
		if fieldKey == keyPosStr {
			continue // Skip the key field
		}

		oldValue, exists := oldItemMap[fieldKey]
		if !exists || !deepEqual(oldValue, newValue) {
			// Strip statics from nested tree nodes since client already has them cached
			// Need to handle both treeNode type and map[string]interface{}
			var newTreeNode treeNode
			var isTree bool

			if tn, ok := newValue.(treeNode); ok {
				newTreeNode = tn
				isTree = true
			} else if m, ok := newValue.(map[string]interface{}); ok {
				newTreeNode = m
				isTree = true
			}

			if isTree {
				stripped := stripStaticsRecursively(newTreeNode)
				// If stripping results in empty map, check if this is a meaningful change
				if strippedMap, ok := stripped.(map[string]interface{}); ok && len(strippedMap) == 0 {
					// Check if old value would also strip to empty
					// If both old and new are static-only (strip to empty), don't send the change
					if exists {
						var oldTreeNode treeNode
						var oldIsTree bool
						if tn, ok := oldValue.(treeNode); ok {
							oldTreeNode = tn
							oldIsTree = true
						} else if m, ok := oldValue.(map[string]interface{}); ok {
							oldTreeNode = m
							oldIsTree = true
						}

						if oldIsTree {
							oldStripped := stripStaticsRecursively(oldTreeNode)
							if oldStrippedMap, ok := oldStripped.(map[string]interface{}); ok && len(oldStrippedMap) == 0 {
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
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				oldKeys[key] = true
			}
		}
	}

	var newKeys []string
	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, ok := getItemKey(itemMap, statics); ok {
				if !oldKeys[key] {
					newKeys = append(newKeys, key)
				}
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
func areAllItemsAtSamePosition(newKeys []string, oldItems, newItems []interface{}, statics interface{}) (bool, string, string) {
	if len(newKeys) <= 1 {
		return false, "", "" // Single items don't need this optimization
	}

	// Find the first new item's position
	var firstNewIndex = -1
	var targetKey = ""
	var position = ""

	for i, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if keyStr, ok := getItemKey(itemMap, statics); ok {
				// Check if this is a new key
				for _, newKey := range newKeys {
					if newKey == keyStr {
						if firstNewIndex == -1 {
							firstNewIndex = i
							// Determine the target and position
							if i > 0 {
								// Check the item before
								if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
									if prevKeyStr, ok := getItemKey(prevItem, statics); ok {
										targetKey = prevKeyStr
										position = "after"
									}
								}
							} else {
								// At the beginning
								targetKey = ""
								position = "start"
							}
						}
						break
					}
				}
			}
		}
	}

	if firstNewIndex == -1 {
		return false, "", ""
	}

	// Verify all new items are contiguous starting from firstNewIndex
	for i, newKey := range newKeys {
		expectedIndex := firstNewIndex + i
		if expectedIndex >= len(newItems) {
			return false, "", ""
		}

		if itemMap, ok := newItems[expectedIndex].(map[string]interface{}); ok {
			if keyStr, ok := getItemKey(itemMap, statics); ok {
				if keyStr != newKey {
					return false, "", ""
				}
			} else {
				return false, "", ""
			}
		} else {
			return false, "", ""
		}
	}

	return true, targetKey, position
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
		if itemMap, ok := item.(map[string]interface{}); ok {
			if keyStr, ok := getItemKey(itemMap, statics); ok {
				// Check if this is a new key
				for _, newKey := range newKeys {
					if newKey == keyStr {
						// Determine insertion point
						var insertionPoint string
						if i > 0 {
							if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
								if prevKeyStr, ok := getItemKey(prevItem, statics); ok {
									insertionPoint = prevKeyStr + ":after"
								}
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
	}

	return len(insertionPoints) > maxInsertionPoints
}

// analyzeChangeAndCreateTree determines the best tree structure based on the type of change
func (t *Template) analyzeChangeAndCreateTree(oldHTML, newHTML string, _, _ interface{}) (treeNode, error) {
	// Find common prefix and suffix to understand change patterns
	commonPrefix := findCommonPrefix(oldHTML, newHTML)
	commonSuffix := findCommonSuffix(oldHTML, newHTML)

	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)

	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		return treeNode{
			"s": []string{"", ""},
			"0": minifyHTML(newHTML),
		}, nil
	}

	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		dynamicPart := newHTML[changeStart:changeEnd]
		return treeNode{
			"s": []string{commonPrefix, commonSuffix},
			"0": minifyHTML(dynamicPart),
		}, nil
	}

	// Default to full dynamic content
	return treeNode{
		"s": []string{"", ""},
		"0": minifyHTML(newHTML),
	}, nil
}

// createHTMLStructureBasedTree implements deterministic segmentation strategies for HTML content
func (t *Template) createHTMLStructureBasedTree(html string) treeNode {
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
		tree := treeNode{"s": statics}
		for i, dyn := range dynamics {
			// Minify HTML content if it's a string containing HTML
			if strDyn, ok := dyn.(string); ok && strings.Contains(strDyn, "<") {
				dyn = minifyHTML(strDyn)
			}
			tree[fmt.Sprintf("%d", i)] = dyn
		}

		// If we got reasonable segmentation, use it
		if len(statics) > 2 && len(dynamics) > 0 {
			return tree
		}
	}

	// Fallback to single segment strategy
	return treeNode{
		"s": []string{"", ""},
		"0": minifyHTML(html),
	}
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

// marshalOrderedJSON marshals a treeNode to JSON with keys in sorted order
func marshalOrderedJSON(tree treeNode) ([]byte, error) {
	if len(tree) == 0 {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteByte('{')

	// Sort keys numerically for proper ordering
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}

	// Custom sort to handle numeric keys properly
	sort.Slice(keys, func(i, j int) bool {
		// Try to parse as numbers first
		num1, err1 := strconv.Atoi(keys[i])
		num2, err2 := strconv.Atoi(keys[j])

		if err1 == nil && err2 == nil {
			// Both are numbers, sort numerically
			return num1 < num2
		}

		// If one or both are not numbers, sort lexicographically
		// But put "s" (static) first
		if keys[i] == "s" {
			return true
		}
		if keys[j] == "s" {
			return false
		}

		return keys[i] < keys[j]
	})

	first := true
	for _, key := range keys {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Write key
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')

		// Write value with no HTML escaping
		valueBytes, err := marshalValue(tree[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
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
func (t *Template) loadExistingKeyMappings(lastTree treeNode) {
	// Look for range data in the tree and load existing key mappings
	for _, value := range lastTree {
		if rangeData, ok := value.(map[string]interface{}); ok {
			// Check if this looks like range data with "d" field
			if dynData, exists := rangeData["d"]; exists {
				if dynSlice, ok := dynData.([]interface{}); ok {
					t.keyGen.loadExistingKeys(dynSlice)
				}
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
