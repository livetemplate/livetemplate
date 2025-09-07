package livetemplate

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
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
	pageDataModels map[string][]DataModel        // Data models per page (keyed by page ID)
	dataModelsMu   sync.RWMutex                  // Mutex for pageDataModels map
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
		pageDataModels: make(map[string][]DataModel),        // Initialize data models registry
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

// DataModel represents a registered data model that can handle actions through methods
type DataModel struct {
	Instance      interface{}               // The actual data model instance
	Name          string                    // Model name for namespacing (derived from type name)
	ActionMethods map[string]reflect.Method // Map of action name to reflect.Method
}

// ActionContext provides a clean interface for action methods to interact with request data and responses
type ActionContext struct {
	actionData map[string]interface{} // Raw action data from client
	response   interface{}            // Response data to return
}

// NewActionContext creates a new ActionContext with the provided action data
func NewActionContext(actionData map[string]interface{}) *ActionContext {
	if actionData == nil {
		actionData = make(map[string]interface{})
	}
	return &ActionContext{
		actionData: actionData,
	}
}

// Bind parses action data into the provided struct using JSON-like field matching
func (ctx *ActionContext) Bind(target interface{}) error {
	// Simple field-by-field binding (can be enhanced with reflection)
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("bind target must be a pointer to struct")
	}

	targetStruct := targetValue.Elem()
	targetType := targetStruct.Type()

	for i := 0; i < targetStruct.NumField(); i++ {
		field := targetStruct.Field(i)
		fieldType := targetType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get field name (use json tag if available, otherwise use field name)
		fieldName := fieldType.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = fieldType.Name
		}

		// Get value from action data
		if value, exists := ctx.actionData[fieldName]; exists {
			if err := ctx.setFieldValue(field, value); err != nil {
				return fmt.Errorf("failed to set field %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value with appropriate type conversion
func (ctx *ActionContext) setFieldValue(field reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	valueReflect := reflect.ValueOf(value)
	fieldType := field.Type()

	// Handle type conversions
	switch fieldType.Kind() {
	case reflect.String:
		if str, ok := value.(string); ok {
			field.SetString(str)
		} else {
			field.SetString(fmt.Sprintf("%v", value))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if num, ok := value.(float64); ok { // JSON numbers are float64
			field.SetInt(int64(num))
		} else if valueReflect.CanConvert(fieldType) {
			field.Set(valueReflect.Convert(fieldType))
		}
	case reflect.Float32, reflect.Float64:
		if num, ok := value.(float64); ok {
			field.SetFloat(num)
		} else if valueReflect.CanConvert(fieldType) {
			field.Set(valueReflect.Convert(fieldType))
		}
	case reflect.Bool:
		if b, ok := value.(bool); ok {
			field.SetBool(b)
		}
	default:
		if valueReflect.Type().AssignableTo(fieldType) {
			field.Set(valueReflect)
		}
	}

	return nil
}

// Get retrieves a value from the action data
func (ctx *ActionContext) Get(key string) (interface{}, bool) {
	value, exists := ctx.actionData[key]
	return value, exists
}

// GetString retrieves a string value from action data
func (ctx *ActionContext) GetString(key string) string {
	if value, exists := ctx.actionData[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", value)
	}
	return ""
}

// GetInt retrieves an integer value from action data
func (ctx *ActionContext) GetInt(key string) int {
	if value, exists := ctx.actionData[key]; exists {
		if num, ok := value.(float64); ok { // JSON numbers are float64
			return int(num)
		}
		if num, ok := value.(int); ok {
			return num
		}
	}
	return 0
}

// GetBool retrieves a boolean value from action data
func (ctx *ActionContext) GetBool(key string) bool {
	if value, exists := ctx.actionData[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}

// Data sets the response data that will be returned to the client
func (ctx *ActionContext) Data(data interface{}) error {
	fmt.Printf("DEBUG: ActionContext.Data called with: %v\n", data)
	ctx.response = data
	fmt.Printf("DEBUG: ActionContext.Data set response, returning\n")
	return nil
}

// GetResponse returns the response data (internal use)
func (ctx *ActionContext) GetResponse() interface{} {
	return ctx.response
}

// ActionMethodSignature defines the new expected signature for action methods
// Methods should have signature: func(ctx *ActionContext) error
type ActionMethodSignature func(ctx *ActionContext) error

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
	internal   *app.Page
	cacheInfo  *ClientCacheInfo
	actions    map[string]ActionHandler // Registered action handlers
	dataModels []DataModel              // Registered data models with method-based actions
	sessionID  string                   // Session ID for this page
	app        *Application             // Reference to parent application
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
		internal:   internal,
		actions:    a.actions,            // Use application's action registry
		dataModels: make([]DataModel, 0), // Initialize empty data models slice
		sessionID:  sess.ID,
		app:        a,
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

	// Load data models for this page
	a.dataModelsMu.RLock()
	pageDataModels := make([]DataModel, len(a.pageDataModels[sess.PageID]))
	copy(pageDataModels, a.pageDataModels[sess.PageID])
	a.dataModelsMu.RUnlock()

	// Create ApplicationPage with session context
	page := &ApplicationPage{
		internal:   internalPage,
		actions:    a.actions,
		dataModels: pageDataModels,
		sessionID:  sess.ID,
		app:        a,
		cacheInfo:  cacheInfo,
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

// RegisterDataModel registers a data model that can handle actions through methods
// The data model must be a struct with public methods that match the ActionMethodSignature
// Method signature: func(actionData map[string]interface{}) (interface{}, error)
func (p *ApplicationPage) RegisterDataModel(model interface{}) error {
	if model == nil {
		return fmt.Errorf("data model cannot be nil")
	}

	// Use reflection to analyze the model
	modelType := reflect.TypeOf(model)

	// Store the original type for method lookup (preserve pointer type)
	originalType := modelType

	// Get the underlying type for name extraction
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Ensure it's a struct
	if modelType.Kind() != reflect.Struct {
		return fmt.Errorf("data model must be a struct, got %s", modelType.Kind())
	}

	// Generate model name from type (e.g., "Counter" from type Counter)
	modelName := strings.ToLower(modelType.Name())
	if modelName == "" {
		return fmt.Errorf("data model must have a named type")
	}

	// Extract action methods using the original type (with pointer)
	actionMethods := make(map[string]reflect.Method)
	numMethods := originalType.NumMethod()

	for i := 0; i < numMethods; i++ {
		method := originalType.Method(i)

		// Only consider exported methods
		if !method.IsExported() {
			continue
		}

		// Check method signature: func(receiver, *ActionContext) error
		if p.isValidActionMethod(method.Type) {
			actionName := strings.ToLower(method.Name)
			actionMethods[actionName] = method
		}
	}

	if len(actionMethods) == 0 {
		return fmt.Errorf("data model %s has no valid action methods", modelName)
	}

	// Create and register the data model
	dataModel := DataModel{
		Instance:      model,
		Name:          modelName,
		ActionMethods: actionMethods,
	}

	// Store data model at the application level for persistence across GetPage calls
	pageID := p.internal.GetID()
	p.app.dataModelsMu.Lock()
	p.app.pageDataModels[pageID] = append(p.app.pageDataModels[pageID], dataModel)
	p.app.dataModelsMu.Unlock()

	// Also store locally for immediate access
	p.dataModels = append(p.dataModels, dataModel)
	return nil
}

// isValidActionMethod checks if a method has the correct signature for an action method
// Expected: func(receiver, *ActionContext) error
func (p *ApplicationPage) isValidActionMethod(methodType reflect.Type) bool {
	// Method should have 2 parameters (receiver + actionContext)
	if methodType.NumIn() != 2 {
		return false
	}

	// Method should have 1 return value (error)
	if methodType.NumOut() != 1 {
		return false
	}

	// Check parameter types: first is receiver (skip), second should be *ActionContext
	paramType := methodType.In(1)
	expectedParamType := reflect.TypeOf((*ActionContext)(nil))
	if paramType != expectedParamType {
		return false
	}

	// Check return type: should be error
	returnType := methodType.Out(0)
	expectedReturnType := reflect.TypeOf((*error)(nil)).Elem()

	return returnType == expectedReturnType
}

// resolveAndExecuteAction handles action resolution with conflict detection and namespacing
func (p *ApplicationPage) resolveAndExecuteAction(actionName string, actionData map[string]interface{}) (interface{}, error) {
	// First, check for exact match in registered action handlers
	if handler, exists := p.actions[actionName]; exists {
		currentData := p.GetData()
		return handler(currentData, actionData)
	}

	// Then check if it's a namespaced action (e.g., "counter.increment")
	if strings.Contains(actionName, ".") {
		return p.executeNamespacedAction(actionName, actionData)
	}

	// Finally, check for direct action in data models with conflict detection
	return p.executeDirectAction(actionName, actionData)
}

// executeNamespacedAction handles actions with explicit namespace (e.g., "counter.increment")
func (p *ApplicationPage) executeNamespacedAction(actionName string, actionData map[string]interface{}) (interface{}, error) {
	parts := strings.SplitN(actionName, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid namespaced action format: %q", actionName)
	}

	modelName := strings.ToLower(parts[0])
	methodName := strings.ToLower(parts[1])

	// Find the specific data model by name
	for _, model := range p.dataModels {
		if model.Name == modelName {
			if method, exists := model.ActionMethods[methodName]; exists {
				return p.callDataModelMethod(model.Instance, method, actionData)
			}
			return nil, fmt.Errorf("action method %q not found on model %q", methodName, modelName)
		}
	}

	return nil, fmt.Errorf("data model %q not found for action %q", modelName, actionName)
}

// executeDirectAction handles direct actions with conflict detection
func (p *ApplicationPage) executeDirectAction(actionName string, actionData map[string]interface{}) (interface{}, error) {
	var matchingModels []DataModel
	var matchingMethods []reflect.Method

	// Find all models that have this action method
	for _, model := range p.dataModels {
		if method, exists := model.ActionMethods[actionName]; exists {
			matchingModels = append(matchingModels, model)
			matchingMethods = append(matchingMethods, method)
		}
	}

	if len(matchingModels) == 0 {
		return nil, fmt.Errorf("action %q not found in any registered handlers or data models", actionName)
	}

	if len(matchingModels) == 1 {
		// No conflict, execute the action
		return p.callDataModelMethod(matchingModels[0].Instance, matchingMethods[0], actionData)
	}

	// Conflict detected - provide helpful error message with namespacing suggestions
	var modelNames []string
	for _, model := range matchingModels {
		modelNames = append(modelNames, fmt.Sprintf("%s.%s", model.Name, actionName))
	}

	return nil, fmt.Errorf("action %q conflicts between multiple data models. Use namespaced actions: %s",
		actionName, strings.Join(modelNames, ", "))
}

// callDataModelMethod invokes a data model method using reflection
func (p *ApplicationPage) callDataModelMethod(instance interface{}, method reflect.Method, actionData map[string]interface{}) (interface{}, error) {
	// Create ActionContext with the action data
	ctx := NewActionContext(actionData)

	// Prepare method arguments: receiver + actionContext
	args := []reflect.Value{
		reflect.ValueOf(instance),
		reflect.ValueOf(ctx),
	}

	// Call the method
	results := method.Func.Call(args)

	// Extract error result
	errInterface := results[0].Interface()

	// Handle error result
	if errInterface != nil {
		if err, ok := errInterface.(error); ok {
			return nil, fmt.Errorf("data model action failed: %w", err)
		}
	}

	// Get response data from context (may be nil)
	return ctx.GetResponse(), nil
}

// HasActions returns true if any actions are registered (either handlers or data models)
func (p *ApplicationPage) HasActions() bool {
	if len(p.actions) > 0 {
		return true
	}

	// Check if any data models have action methods
	for _, model := range p.dataModels {
		if len(model.ActionMethods) > 0 {
			return true
		}
	}

	return false
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

	// Try to resolve the action (with conflict detection and namespacing)
	newData, err := p.resolveAndExecuteAction(msg.Action, msg.Data)
	if err != nil {
		return nil, err
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
