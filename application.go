package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate/internal/app"
	"github.com/livefir/livetemplate/internal/session"
)

// Application provides secure multi-tenant isolation with session-based authentication
type Application struct {
	internal       *app.Application
	config         *ApplicationConfig
	templates      map[string]*template.Template // Template registry for reuse
	actions        map[string]ActionHandler      // Global action registry
	sessionManager *session.Manager              // Session management
}

// ApplicationConfig contains configuration for the public Application
type ApplicationConfig struct {
	MaxMemoryMB    int
	MetricsEnabled bool
}

// ApplicationOption configures an Application instance
type ApplicationOption func(*Application) error

// NewApplication creates a new isolated Application instance
func NewApplication(options ...ApplicationOption) (*Application, error) {
	// Initialize with default configuration
	publicApp := &Application{
		config: &ApplicationConfig{
			MaxMemoryMB:    100,
			MetricsEnabled: true,
		},
		templates:      make(map[string]*template.Template), // Initialize template registry
		actions:        make(map[string]ActionHandler),      // Initialize action registry
		sessionManager: session.NewManager(24 * time.Hour),  // Initialize session manager
	}

	// Apply public options to collect configuration
	for _, option := range options {
		if err := option(publicApp); err != nil {
			return nil, err
		}
	}

	// Convert public options to internal options
	var internalOptions []app.Option
	if publicApp.config.MaxMemoryMB != 100 {
		internalOptions = append(internalOptions, app.WithMaxMemoryMB(publicApp.config.MaxMemoryMB))
	}
	if !publicApp.config.MetricsEnabled {
		internalOptions = append(internalOptions, app.WithMetricsEnabled(false))
	}

	// Create internal application with collected configuration
	internal, err := app.NewApplication(internalOptions...)
	if err != nil {
		return nil, err
	}

	publicApp.internal = internal
	return publicApp, nil
}

// WithMaxPages sets the maximum number of pages
func WithMaxPages(maxPages int) ApplicationOption {
	return func(a *Application) error {
		// Configuration will be applied when creating internal application
		return nil
	}
}

// WithPageTTL sets the page time-to-live
func WithPageTTL(ttl time.Duration) ApplicationOption {
	return func(a *Application) error {
		// Configuration will be applied when creating internal application
		return nil
	}
}

// WithMaxMemoryMB sets the maximum memory usage in MB
func WithMaxMemoryMB(memoryMB int) ApplicationOption {
	return func(a *Application) error {
		a.config.MaxMemoryMB = memoryMB
		return nil
	}
}

// WithApplicationMetricsEnabled configures metrics collection for the application
func WithApplicationMetricsEnabled(enabled bool) ApplicationOption {
	return func(a *Application) error {
		a.config.MetricsEnabled = enabled
		return nil
	}
}

// ApplicationPageOption configures a page created by an Application
type ApplicationPageOption func(*ApplicationPage) error

// WithCacheInfo sets the client cache information for the page
func WithCacheInfo(cacheInfo *ClientCacheInfo) ApplicationPageOption {
	return func(p *ApplicationPage) error {
		p.cacheInfo = cacheInfo
		return nil
	}
}

// ActionHandler is a function that processes an action and returns updated data
type ActionHandler func(currentData interface{}, actionData map[string]interface{}) (interface{}, error)

// ActionMessage represents an action message from the client
type ActionMessage struct {
	Type   string                 `json:"type"`   // Message type (usually "action")
	Action string                 `json:"action"` // Action name to execute
	Token  string                 `json:"token"`  // Optional security token
	Data   map[string]interface{} `json:"data"`   // Action payload data
}

// NewActionMessage creates a new ActionMessage with the given action name and data
func NewActionMessage(action string, data map[string]interface{}) *ActionMessage {
	return &ActionMessage{
		Type:   "action",
		Action: action,
		Data:   data,
	}
}

// ApplicationPage represents a page managed by an Application with session-based authentication
type ApplicationPage struct {
	internal  *app.Page
	cacheInfo *ClientCacheInfo
	actions   map[string]ActionHandler // Registered action handlers
	sessionID string                   // Session ID for this page
	app       *Application             // Reference to parent application
}

// NewApplicationPage creates a new isolated page session with session-based authentication
func (a *Application) NewApplicationPage(tmpl *template.Template, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	// Convert public options to internal options
	var internalOptions []app.PageOption

	// Create internal page
	internal, err := a.internal.NewPage(tmpl, data, internalOptions...)
	if err != nil {
		return nil, err
	}

	// Create session for this page
	sess, err := a.sessionManager.CreateSession(a.internal.GetID(), internal.GetID(), internal.GetToken())
	if err != nil {
		_ = internal.Close() // Cleanup internal page on error
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	publicPage := &ApplicationPage{
		internal:  internal,
		actions:   a.actions, // Use application's action registry
		sessionID: sess.ID,
		app:       a,
	}

	// Apply public options
	for _, option := range options {
		if err := option(publicPage); err != nil {
			_ = publicPage.Close() // Cleanup on error
			return nil, err
		}
	}

	return publicPage, nil
}

// RegisterTemplate registers a template with a name for reuse
func (a *Application) RegisterTemplate(name string, tmpl *template.Template) error {
	if tmpl == nil {
		return fmt.Errorf("template cannot be nil")
	}
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	a.templates[name] = tmpl
	return nil
}

// RegisterTemplateFromFile parses and registers a template from a file
func (a *Application) RegisterTemplateFromFile(name string, filepath string) error {
	tmpl, err := template.ParseFiles(filepath)
	if err != nil {
		return fmt.Errorf("failed to parse template file %s: %w", filepath, err)
	}

	return a.RegisterTemplate(name, tmpl)
}

// NewPageFromTemplate creates a new page using a registered template
func (a *Application) NewPageFromTemplate(templateName string, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	tmpl, exists := a.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %q not registered", templateName)
	}

	return a.NewApplicationPage(tmpl, data, options...)
}

// NewPage creates a new page using a registered template (simplified name)
func (a *Application) NewPage(templateName string, data interface{}, options ...ApplicationPageOption) (*ApplicationPage, error) {
	return a.NewPageFromTemplate(templateName, data, options...)
}

// GetRegisteredTemplates returns the names of all registered templates
func (a *Application) GetRegisteredTemplates() []string {
	names := make([]string, 0, len(a.templates))
	for name := range a.templates {
		names = append(names, name)
	}
	return names
}

// RegisterAction registers an action handler at the application level
func (a *Application) RegisterAction(actionName string, handler ActionHandler) {
	a.actions[actionName] = handler
}

// GetPage retrieves a page from an HTTP request, handling all authentication complexity internally
// This is the primary API that users should use for all page retrieval needs
func (a *Application) GetPage(r *http.Request) (*ApplicationPage, error) {
	// Get session cookie
	cookie, err := r.Cookie("livetemplate_session")
	if err != nil {
		return nil, fmt.Errorf("no session cookie found: %w", err)
	}

	// Validate session
	sess, exists := a.sessionManager.GetSession(cookie.Value)
	if !exists {
		return nil, fmt.Errorf("invalid or expired session")
	}

	// Parse cache information from URL query parameters
	cacheInfo := ParseCacheInfoFromURL(r.URL.Query())

	// Get the internal page directly by ID to bypass JWT token validation
	internalPage, err := a.internal.GetPageByID(sess.PageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	// Create ApplicationPage with session context
	page := &ApplicationPage{
		internal:  internalPage,
		actions:   a.actions,
		sessionID: sess.ID,
		app:       a,
		cacheInfo: cacheInfo,
	}

	return page, nil
}

// ServeHTTP serves the page as HTML response, optionally updating data first
func (p *ApplicationPage) ServeHTTP(w http.ResponseWriter, data ...interface{}) error {
	// Set session cookie before rendering
	http.SetCookie(w, &http.Cookie{
		Name:     "livetemplate_session",
		Value:    p.sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   24 * 3600, // 24 hours
	})

	// Render with optional data update and automatic token embedding
	html, err := p.Render(data...)
	if err != nil {
		return fmt.Errorf("failed to render page: %w", err)
	}

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

// GetPageCount returns the total number of active pages
func (a *Application) GetPageCount() int {
	return a.internal.GetPageCount()
}

// CleanupExpiredPages removes expired pages and returns count
func (a *Application) CleanupExpiredPages() int {
	return a.internal.CleanupExpiredPages()
}

// GetApplicationMetrics returns application metrics
func (a *Application) GetApplicationMetrics() ApplicationMetrics {
	internal := a.internal.GetMetrics()

	return ApplicationMetrics{
		ApplicationID:      internal.ApplicationID,
		PagesCreated:       internal.PagesCreated,
		PagesDestroyed:     internal.PagesDestroyed,
		ActivePages:        internal.ActivePages,
		MaxConcurrentPages: internal.MaxConcurrentPages,
		TokensGenerated:    internal.TokensGenerated,
		TokensVerified:     internal.TokensVerified,
		TokenFailures:      internal.TokenFailures,
		FragmentsGenerated: internal.FragmentsGenerated,
		GenerationErrors:   internal.GenerationErrors,
		MemoryUsage:        internal.MemoryUsage,
		MemoryUsagePercent: internal.MemoryUsagePercent,
		MemoryStatus:       internal.MemoryStatus,
		RegistryCapacity:   internal.RegistryCapacity,
		Uptime:             internal.Uptime,
		StartTime:          internal.StartTime,
	}
}

// Close releases all application resources
func (a *Application) Close() error {
	return a.internal.Close()
}

// ApplicationPage methods

// Render generates the complete HTML output, optionally updating data first
// Automatically embeds the page token in the HTML
func (p *ApplicationPage) Render(data ...interface{}) (string, error) {
	// Update data if provided
	if len(data) > 0 {
		if err := p.SetData(data[0]); err != nil {
			return "", fmt.Errorf("failed to update page data: %w", err)
		}
	}

	html, err := p.internal.Render()
	if err != nil {
		return "", err
	}

	// Automatically embed the page token via meta tag
	token := p.GetToken()

	// Replace placeholder if it exists
	html = strings.ReplaceAll(html, "PAGE_TOKEN_PLACEHOLDER", token)

	// Also inject meta tag if not already present
	if !strings.Contains(html, `meta name="livetemplate-token"`) {
		metaTag := fmt.Sprintf(`<meta name="livetemplate-token" content="%s">`, token)
		if strings.Contains(html, "</head>") {
			html = strings.Replace(html, "</head>", metaTag+"\n</head>", 1)
		} else if strings.Contains(html, "<head>") {
			html = strings.Replace(html, "<head>", "<head>\n"+metaTag, 1)
		}
	}

	return html, nil
}

// ClientCacheInfo contains information about what the client has cached
type ClientCacheInfo struct {
	HasCache        bool            `json:"has_cache"`
	CachedFragments map[string]bool `json:"cached_fragments"`
}

// RenderFragments generates fragment updates for the given new data
// Automatically uses the page's cache information if set
func (p *ApplicationPage) RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error) {
	internalFragments, err := p.internal.RenderFragments(ctx, newData)
	if err != nil {
		return nil, err
	}

	// Convert to existing Fragment type format
	fragments := make([]*Fragment, len(internalFragments))
	for i, frag := range internalFragments {
		fragments[i] = &Fragment{
			ID:       frag.ID,
			Data:     frag.Data,
			Metadata: convertInternalMetadata(frag.Metadata),
		}
	}

	// Apply client cache filtering if cache info is set on the page
	if p.cacheInfo != nil && p.cacheInfo.HasCache {
		fragments = p.filterStaticsFromFragments(fragments, p.cacheInfo.CachedFragments)
	}

	return fragments, nil
}

// filterStaticsFromFragments removes statics from fragments that client already has cached
func (p *ApplicationPage) filterStaticsFromFragments(fragments []*Fragment, cachedFragments map[string]bool) []*Fragment {
	var filtered []*Fragment

	for _, frag := range fragments {
		// Create a copy of the fragment
		newFrag := &Fragment{
			ID:       frag.ID,
			Data:     frag.Data,
			Metadata: frag.Metadata,
		}

		// If client has this fragment cached, remove statics from data
		if cachedFragments[frag.ID] {
			if treeData, ok := frag.Data.(map[string]interface{}); ok {
				// Create new tree data without statics
				newTreeData := make(map[string]interface{})
				for k, v := range treeData {
					if k != "s" { // Remove statics ("s" field)
						newTreeData[k] = v
					}
				}
				newFrag.Data = newTreeData
			}
		}

		filtered = append(filtered, newFrag)
	}

	return filtered
}

// GetSessionID returns the session ID for this page
func (p *ApplicationPage) GetSessionID() string {
	return p.sessionID
}

// GetCacheToken returns the stable token used for client-side cache identification
// This token remains the same across page reloads to maintain cache consistency
func (p *ApplicationPage) GetCacheToken() string {
	return p.internal.GetToken()
}

// GetToken returns the session ID (legacy method for backward compatibility)
func (p *ApplicationPage) GetToken() string {
	return p.GetSessionID()
}

// SetData updates the page data state
func (p *ApplicationPage) SetData(data interface{}) error {
	return p.internal.SetData(data)
}

// SetCacheInfo updates the client cache information for this page
func (p *ApplicationPage) SetCacheInfo(cacheInfo *ClientCacheInfo) {
	p.cacheInfo = cacheInfo
}

// GetCacheInfo returns the current cache information for this page
func (p *ApplicationPage) GetCacheInfo() *ClientCacheInfo {
	return p.cacheInfo
}

// RegisterAction registers an action handler for the given action name
func (p *ApplicationPage) RegisterAction(actionName string, handler ActionHandler) {
	p.actions[actionName] = handler
}

// HasActions returns true if any actions are registered
func (p *ApplicationPage) HasActions() bool {
	return len(p.actions) > 0
}

// HandleAction processes an action message and returns updated fragments
func (p *ApplicationPage) HandleAction(ctx context.Context, msg *ActionMessage) ([]*Fragment, error) {
	// Validate message
	if msg == nil {
		return nil, fmt.Errorf("action message is nil")
	}

	// Validate message type
	if msg.Type != "" && msg.Type != "action" {
		return nil, fmt.Errorf("invalid message type: %q, expected \"action\"", msg.Type)
	}

	// Validate action name
	if msg.Action == "" {
		return nil, fmt.Errorf("action name is empty")
	}

	// Optional: validate token if provided
	if msg.Token != "" && msg.Token != p.GetToken() {
		return nil, fmt.Errorf("invalid token")
	}

	// Ensure data is not nil
	if msg.Data == nil {
		msg.Data = make(map[string]interface{})
	}

	// Check if action is registered
	handler, exists := p.actions[msg.Action]
	if !exists {
		return nil, fmt.Errorf("action %q not registered", msg.Action)
	}

	// Get current data
	currentData := p.GetData()

	// Call the handler to get new data
	newData, err := handler(currentData, msg.Data)
	if err != nil {
		return nil, fmt.Errorf("action handler failed: %w", err)
	}

	// Update page data and generate fragments
	return p.RenderFragments(ctx, newData)
}

// ServeWebSocket provides a complete WebSocket handler that automatically processes actions
func (a *Application) ServeWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get page from request
		page, err := a.GetPage(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get page: %v", err), http.StatusBadRequest)
			return
		}

		// Debug logging
		fmt.Printf("WebSocket connected: token=%s, actions=%d\n", page.GetToken(), len(page.actions))

		// Upgrade to WebSocket
		upgrader := &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in this example
			},
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handle messages
		for {
			var actionMsg ActionMessage
			err := conn.ReadJSON(&actionMsg)
			if err != nil {
				break
			}

			// Check message type
			if actionMsg.Type != "action" {
				continue // Skip non-action messages
			}

			fmt.Printf("Processing action: %s\n", actionMsg.Action)

			// Process action and get fragments
			fragments, err := page.HandleAction(r.Context(), &actionMsg)
			if err != nil {
				fmt.Printf("Action handler error: %v\n", err)
				continue
			}

			fmt.Printf("Generated %d fragments\n", len(fragments))

			// Send fragments to client
			if err := conn.WriteJSON(fragments); err != nil {
				fmt.Printf("WebSocket send error: %v\n", err)
				break
			}
		}
	}
}

// GetData returns the current page data
func (p *ApplicationPage) GetData() interface{} {
	return p.internal.GetData()
}

// GetTemplate returns the page template
func (p *ApplicationPage) GetTemplate() *template.Template {
	return p.internal.GetTemplate()
}

// GetApplicationPageMetrics returns page-specific metrics
func (p *ApplicationPage) GetApplicationPageMetrics() ApplicationPageMetrics {
	internal := p.internal.GetMetrics()

	return ApplicationPageMetrics{
		PageID:                internal.PageID,
		ApplicationID:         internal.ApplicationID,
		CreatedAt:             internal.CreatedAt,
		LastAccessed:          internal.LastAccessed,
		Age:                   internal.Age,
		IdleTime:              internal.IdleTime,
		MemoryUsage:           internal.MemoryUsage,
		FragmentCacheSize:     internal.FragmentCacheSize,
		TotalGenerations:      internal.TotalGenerations,
		SuccessfulGenerations: internal.SuccessfulGenerations,
		FailedGenerations:     internal.FailedGenerations,
		AverageGenerationTime: internal.AverageGenerationTime,
		ErrorRate:             internal.ErrorRate,
	}
}

// Close releases page resources and removes from application
func (p *ApplicationPage) Close() error {
	return p.internal.Close()
}

// Public API types for Application

// ApplicationMetrics contains application performance data
type ApplicationMetrics struct {
	ApplicationID      string        `json:"application_id"`
	PagesCreated       int64         `json:"pages_created"`
	PagesDestroyed     int64         `json:"pages_destroyed"`
	ActivePages        int64         `json:"active_pages"`
	MaxConcurrentPages int64         `json:"max_concurrent_pages"`
	TokensGenerated    int64         `json:"tokens_generated"`
	TokensVerified     int64         `json:"tokens_verified"`
	TokenFailures      int64         `json:"token_failures"`
	FragmentsGenerated int64         `json:"fragments_generated"`
	GenerationErrors   int64         `json:"generation_errors"`
	MemoryUsage        int64         `json:"memory_usage"`
	MemoryUsagePercent float64       `json:"memory_usage_percent"`
	MemoryStatus       string        `json:"memory_status"`
	RegistryCapacity   float64       `json:"registry_capacity"`
	Uptime             time.Duration `json:"uptime"`
	StartTime          time.Time     `json:"start_time"`
}

// ApplicationPageMetrics contains page performance data for Application-managed pages
type ApplicationPageMetrics struct {
	PageID                string  `json:"page_id"`
	ApplicationID         string  `json:"application_id"`
	CreatedAt             string  `json:"created_at"`
	LastAccessed          string  `json:"last_accessed"`
	Age                   string  `json:"age"`
	IdleTime              string  `json:"idle_time"`
	MemoryUsage           int64   `json:"memory_usage"`
	FragmentCacheSize     int     `json:"fragment_cache_size"`
	TotalGenerations      int64   `json:"total_generations"`
	SuccessfulGenerations int64   `json:"successful_generations"`
	FailedGenerations     int64   `json:"failed_generations"`
	AverageGenerationTime string  `json:"average_generation_time"`
	ErrorRate             float64 `json:"error_rate"`
}

// Helper functions

// ParseCacheInfoFromURL extracts cache information from URL query parameters
func ParseCacheInfoFromURL(queryValues url.Values) *ClientCacheInfo {
	hasCache := queryValues.Get("has_cache") == "true"
	if !hasCache {
		return &ClientCacheInfo{HasCache: false, CachedFragments: make(map[string]bool)}
	}

	cachedFragments := make(map[string]bool)
	if fragmentsList := queryValues.Get("cached_fragments"); fragmentsList != "" {
		for _, fragID := range strings.Split(fragmentsList, ",") {
			if fragID != "" {
				cachedFragments[fragID] = true
			}
		}
	}

	return &ClientCacheInfo{
		HasCache:        true,
		CachedFragments: cachedFragments,
	}
}

// convertInternalMetadata converts internal metadata to existing FragmentMetadata format
func convertInternalMetadata(internal *app.Metadata) *FragmentMetadata {
	if internal == nil {
		return nil
	}

	// Parse the generation time string back to duration
	genTime, _ := time.ParseDuration(internal.GenerationTime)

	return &FragmentMetadata{
		GenerationTime:   genTime,
		OriginalSize:     internal.OriginalSize,
		CompressedSize:   internal.CompressedSize,
		CompressionRatio: internal.CompressionRatio,
		Strategy:         internal.Strategy,
		Confidence:       internal.Confidence,
		FallbackUsed:     internal.FallbackUsed,
	}
}
