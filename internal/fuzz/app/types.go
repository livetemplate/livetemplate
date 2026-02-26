// Package app provides application-level state and operations for fuzz testing.
// It models realistic user workflows like filtering, sorting, and searching
// which transform entire lists rather than individual items.
package app

// AppState represents a realistic application state with view settings.
// The key insight is that FilteredItems is a derived view computed from
// Items + view settings (Filter, SortBy, SortOrder, SearchQuery).
type AppState struct {
	// Raw data (source of truth)
	Items []Item

	// View settings (control how items are displayed)
	Filter      string // "all", "active", "completed"
	SortBy      string // "created", "priority", "alpha"
	SortOrder   string // "asc", "desc"
	SearchQuery string // Filter by text match

	// Conditionals (tested alongside ranges)
	ShowSearch  bool
	ShowFilters bool
	SelectedID  string // Show detail panel if non-empty

	// Derived view (computed before each render)
	FilteredItems []Item
}

// Item represents a single item in the list.
type Item struct {
	ID        string
	Title     string
	Body      string
	Complete  bool
	Priority  string // "low", "medium", "high"
	CreatedAt int    // For ordering (sequence number)
}

// FilterValues are the valid values for the Filter field.
var FilterValues = []string{"all", "active", "completed"}

// SortByValues are the valid values for the SortBy field.
var SortByValues = []string{"created", "priority", "alpha"}

// SortOrderValues are the valid values for the SortOrder field.
var SortOrderValues = []string{"asc", "desc"}

// PriorityValues are the valid values for item Priority.
var PriorityValues = []string{"low", "medium", "high"}

// PaginatedState represents paginated content with load more functionality.
// This tests page transitions, key stability across pages, and loading states.
type PaginatedState struct {
	// All items (source of truth)
	Items []Item

	// Pagination settings
	Page     int // Current page (0-indexed)
	PageSize int // Items per page

	// State indicators
	HasMore     bool // Whether more items are available
	LoadingMore bool // Loading indicator

	// Derived view (computed before each render)
	VisibleItems []Item
}

// Modal represents a single modal in a stack.
type Modal struct {
	ID       string
	Title    string
	Content  string
	IsOpen   bool
	Priority int // For stacking order
}

// ModalState represents modal/panel UI state.
// This tests TreeNode-to-primitive transitions, conditional statics,
// and multiple conditionals changing simultaneously.
type ModalState struct {
	Modals      []Modal
	ActivePanel string // "none", "left", "right", "bottom"
}

// PanelValues are the valid values for ActivePanel.
var PanelValues = []string{"none", "left", "right", "bottom"}

// NestedRangeState represents state with nested ranges (categories containing items).
// This tests key namespace collisions, statics at multiple nesting levels,
// and reorder operations at different hierarchy levels.
type NestedRangeState struct {
	Categories []Category
}

// Category represents a category that can contain items (nested range).
type Category struct {
	ID     string
	Name   string
	Items  []Item
	IsOpen bool // Controls whether items are visible
}

// DefaultNestedRangeState returns a new NestedRangeState with sensible defaults.
func DefaultNestedRangeState() *NestedRangeState {
	return &NestedRangeState{
		Categories: nil,
	}
}

// Clone creates a deep copy of the NestedRangeState.
func (s *NestedRangeState) Clone() *NestedRangeState {
	clone := &NestedRangeState{}
	if s.Categories != nil {
		clone.Categories = make([]Category, len(s.Categories))
		for i, cat := range s.Categories {
			clone.Categories[i] = Category{
				ID:     cat.ID,
				Name:   cat.Name,
				IsOpen: cat.IsOpen,
			}
			if cat.Items != nil {
				clone.Categories[i].Items = make([]Item, len(cat.Items))
				copy(clone.Categories[i].Items, cat.Items)
			}
		}
	}
	return clone
}

// ToMap converts NestedRangeState to map[string]any for template execution.
func (s *NestedRangeState) ToMap() map[string]any {
	categories := make([]map[string]any, len(s.Categories))
	for i, cat := range s.Categories {
		items := make([]map[string]any, len(cat.Items))
		for j, item := range cat.Items {
			items[j] = map[string]any{
				"ID":        item.ID,
				"Title":     item.Title,
				"Body":      item.Body,
				"Complete":  item.Complete,
				"Priority":  item.Priority,
				"CreatedAt": item.CreatedAt,
			}
		}
		categories[i] = map[string]any{
			"ID":     cat.ID,
			"Name":   cat.Name,
			"Items":  items,
			"IsOpen": cat.IsOpen,
		}
	}
	return map[string]any{
		"Categories": categories,
	}
}

// DefaultAppState returns a new AppState with sensible defaults.
func DefaultAppState() *AppState {
	return &AppState{
		Items:       nil,
		Filter:      "all",
		SortBy:      "created",
		SortOrder:   "asc",
		SearchQuery: "",
		ShowSearch:  true,
		ShowFilters: true,
		SelectedID:  "",
	}
}

// Clone creates a deep copy of the AppState.
func (s *AppState) Clone() *AppState {
	clone := &AppState{
		Filter:      s.Filter,
		SortBy:      s.SortBy,
		SortOrder:   s.SortOrder,
		SearchQuery: s.SearchQuery,
		ShowSearch:  s.ShowSearch,
		ShowFilters: s.ShowFilters,
		SelectedID:  s.SelectedID,
	}

	// Deep copy Items
	if s.Items != nil {
		clone.Items = make([]Item, len(s.Items))
		copy(clone.Items, s.Items)
	}

	// Deep copy FilteredItems
	if s.FilteredItems != nil {
		clone.FilteredItems = make([]Item, len(s.FilteredItems))
		copy(clone.FilteredItems, s.FilteredItems)
	}

	return clone
}

// ToMap converts AppState to map[string]any for template execution.
func (s *AppState) ToMap() map[string]any {
	items := make([]map[string]any, len(s.Items))
	for i, item := range s.Items {
		items[i] = map[string]any{
			"ID":        item.ID,
			"Title":     item.Title,
			"Body":      item.Body,
			"Complete":  item.Complete,
			"Priority":  item.Priority,
			"CreatedAt": item.CreatedAt,
		}
	}

	filteredItems := make([]map[string]any, len(s.FilteredItems))
	for i, item := range s.FilteredItems {
		filteredItems[i] = map[string]any{
			"ID":        item.ID,
			"Title":     item.Title,
			"Body":      item.Body,
			"Complete":  item.Complete,
			"Priority":  item.Priority,
			"CreatedAt": item.CreatedAt,
		}
	}

	return map[string]any{
		"Items":         items,
		"Filter":        s.Filter,
		"SortBy":        s.SortBy,
		"SortOrder":     s.SortOrder,
		"SearchQuery":   s.SearchQuery,
		"ShowSearch":    s.ShowSearch,
		"ShowFilters":   s.ShowFilters,
		"SelectedID":    s.SelectedID,
		"FilteredItems": filteredItems,
	}
}

// DefaultPaginatedState returns a new PaginatedState with sensible defaults.
func DefaultPaginatedState() *PaginatedState {
	return &PaginatedState{
		Items:       nil,
		Page:        0,
		PageSize:    5,
		HasMore:     false,
		LoadingMore: false,
	}
}

// Clone creates a deep copy of the PaginatedState.
func (s *PaginatedState) Clone() *PaginatedState {
	clone := &PaginatedState{
		Page:        s.Page,
		PageSize:    s.PageSize,
		HasMore:     s.HasMore,
		LoadingMore: s.LoadingMore,
	}

	if s.Items != nil {
		clone.Items = make([]Item, len(s.Items))
		copy(clone.Items, s.Items)
	}

	if s.VisibleItems != nil {
		clone.VisibleItems = make([]Item, len(s.VisibleItems))
		copy(clone.VisibleItems, s.VisibleItems)
	}

	return clone
}

// ToMap converts PaginatedState to map[string]any for template execution.
func (s *PaginatedState) ToMap() map[string]any {
	visibleItems := make([]map[string]any, len(s.VisibleItems))
	for i, item := range s.VisibleItems {
		visibleItems[i] = map[string]any{
			"ID":        item.ID,
			"Title":     item.Title,
			"Body":      item.Body,
			"Complete":  item.Complete,
			"Priority":  item.Priority,
			"CreatedAt": item.CreatedAt,
		}
	}

	return map[string]any{
		"Page":         s.Page,
		"PageSize":     s.PageSize,
		"HasMore":      s.HasMore,
		"LoadingMore":  s.LoadingMore,
		"VisibleItems": visibleItems,
	}
}

// DefaultModalState returns a new ModalState with sensible defaults.
func DefaultModalState() *ModalState {
	return &ModalState{
		Modals:      nil,
		ActivePanel: "none",
	}
}

// Clone creates a deep copy of the ModalState.
func (s *ModalState) Clone() *ModalState {
	clone := &ModalState{
		ActivePanel: s.ActivePanel,
	}

	if s.Modals != nil {
		clone.Modals = make([]Modal, len(s.Modals))
		copy(clone.Modals, s.Modals)
	}

	return clone
}

// ToMap converts ModalState to map[string]any for template execution.
func (s *ModalState) ToMap() map[string]any {
	modals := make([]map[string]any, len(s.Modals))
	for i, modal := range s.Modals {
		modals[i] = map[string]any{
			"ID":       modal.ID,
			"Title":    modal.Title,
			"Content":  modal.Content,
			"IsOpen":   modal.IsOpen,
			"Priority": modal.Priority,
		}
	}

	// Compute derived values for template
	hasOpenModal := false
	var topModal map[string]any
	for _, modal := range s.Modals {
		if modal.IsOpen {
			hasOpenModal = true
			topModal = map[string]any{
				"ID":       modal.ID,
				"Title":    modal.Title,
				"Content":  modal.Content,
				"IsOpen":   modal.IsOpen,
				"Priority": modal.Priority,
			}
		}
	}

	return map[string]any{
		"Modals":       modals,
		"ActivePanel":  s.ActivePanel,
		"HasOpenModal": hasOpenModal,
		"TopModal":     topModal,
	}
}

// FormState represents a form with validation state.
// This tests error message appearing/disappearing (TreeNode transitions),
// multiple fields with different error states, and submit state changes.
type FormState struct {
	Fields     map[string]string // Field name -> value
	Errors     map[string]string // Field name -> error message (empty = no error)
	Touched    map[string]bool   // Field name -> whether user has interacted
	Submitting bool              // Form is being submitted
	Submitted  bool              // Form was successfully submitted
}

// FormFieldNames are the available form fields for testing.
var FormFieldNames = []string{"name", "email", "password", "phone", "address"}

// DefaultFormState returns a new FormState with sensible defaults.
func DefaultFormState() *FormState {
	return &FormState{
		Fields:     make(map[string]string),
		Errors:     make(map[string]string),
		Touched:    make(map[string]bool),
		Submitting: false,
		Submitted:  false,
	}
}

// Clone creates a deep copy of the FormState.
func (s *FormState) Clone() *FormState {
	clone := &FormState{
		Fields:     make(map[string]string),
		Errors:     make(map[string]string),
		Touched:    make(map[string]bool),
		Submitting: s.Submitting,
		Submitted:  s.Submitted,
	}

	for k, v := range s.Fields {
		clone.Fields[k] = v
	}
	for k, v := range s.Errors {
		clone.Errors[k] = v
	}
	for k, v := range s.Touched {
		clone.Touched[k] = v
	}

	return clone
}

// ToMap converts FormState to map[string]any for template execution.
func (s *FormState) ToMap() map[string]any {
	// Convert fields to template-friendly format
	fields := make([]map[string]any, 0, len(FormFieldNames))
	for _, name := range FormFieldNames {
		value := s.Fields[name]
		errorMsg := s.Errors[name]
		touched := s.Touched[name]
		fields = append(fields, map[string]any{
			"Name":     name,
			"Value":    value,
			"Error":    errorMsg,
			"Touched":  touched,
			"HasError": errorMsg != "",
		})
	}

	hasErrors := false
	for _, err := range s.Errors {
		if err != "" {
			hasErrors = true
			break
		}
	}

	return map[string]any{
		"Fields":     fields,
		"Submitting": s.Submitting,
		"Submitted":  s.Submitted,
		"HasErrors":  hasErrors,
	}
}

// AsyncState represents state with async loading indicators.
// This tests loading→content→loading cycles, per-item loading states,
// and error state transitions.
type AsyncState struct {
	Items       []Item
	Loading     bool              // Global loading state
	ItemLoading map[string]bool   // Per-item loading by ID
	ItemErrors  map[string]string // Per-item errors by ID
}

// DefaultAsyncState returns a new AsyncState with sensible defaults.
func DefaultAsyncState() *AsyncState {
	return &AsyncState{
		Items:       nil,
		Loading:     false,
		ItemLoading: make(map[string]bool),
		ItemErrors:  make(map[string]string),
	}
}

// Clone creates a deep copy of the AsyncState.
func (s *AsyncState) Clone() *AsyncState {
	clone := &AsyncState{
		Loading:     s.Loading,
		ItemLoading: make(map[string]bool),
		ItemErrors:  make(map[string]string),
	}

	if s.Items != nil {
		clone.Items = make([]Item, len(s.Items))
		copy(clone.Items, s.Items)
	}

	for k, v := range s.ItemLoading {
		clone.ItemLoading[k] = v
	}
	for k, v := range s.ItemErrors {
		clone.ItemErrors[k] = v
	}

	return clone
}

// ToMap converts AsyncState to map[string]any for template execution.
func (s *AsyncState) ToMap() map[string]any {
	items := make([]map[string]any, len(s.Items))
	for i, item := range s.Items {
		items[i] = map[string]any{
			"ID":        item.ID,
			"Title":     item.Title,
			"Body":      item.Body,
			"Complete":  item.Complete,
			"Priority":  item.Priority,
			"CreatedAt": item.CreatedAt,
			"Loading":   s.ItemLoading[item.ID],
			"Error":     s.ItemErrors[item.ID],
			"HasError":  s.ItemErrors[item.ID] != "",
		}
	}

	return map[string]any{
		"Items":   items,
		"Loading": s.Loading,
	}
}

// -----------------------------------------------------------------------------
// Notification Queue State
// -----------------------------------------------------------------------------

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotificationInfo    NotificationType = "info"
	NotificationSuccess NotificationType = "success"
	NotificationWarning NotificationType = "warning"
	NotificationError   NotificationType = "error"
)

// NotificationTypeValues is a list of all notification types.
var NotificationTypeValues = []NotificationType{
	NotificationInfo,
	NotificationSuccess,
	NotificationWarning,
	NotificationError,
}

// Notification represents a single notification in the queue.
type Notification struct {
	ID          string
	Message     string
	Type        NotificationType
	AutoDismiss bool
	Timer       int // Countdown timer for auto-dismiss (0 = no timer)
}

// NotificationState represents the notification queue state.
// Tests notification queue management, auto-dismiss timers, and max visible limits.
type NotificationState struct {
	Notifications []Notification
	MaxVisible    int
}

// DefaultNotificationState returns a new NotificationState with sensible defaults.
func DefaultNotificationState() *NotificationState {
	return &NotificationState{
		Notifications: []Notification{},
		MaxVisible:    5,
	}
}

// Clone creates a deep copy of the NotificationState.
func (s *NotificationState) Clone() *NotificationState {
	clone := &NotificationState{
		Notifications: make([]Notification, len(s.Notifications)),
		MaxVisible:    s.MaxVisible,
	}
	copy(clone.Notifications, s.Notifications)
	return clone
}

// VisibleNotifications returns the notifications that should be visible.
func (s *NotificationState) VisibleNotifications() []Notification {
	if len(s.Notifications) <= s.MaxVisible {
		return s.Notifications
	}
	return s.Notifications[:s.MaxVisible]
}

// HasOverflow returns true if there are more notifications than MaxVisible.
func (s *NotificationState) HasOverflow() bool {
	return len(s.Notifications) > s.MaxVisible
}

// OverflowCount returns the number of hidden notifications.
func (s *NotificationState) OverflowCount() int {
	if len(s.Notifications) <= s.MaxVisible {
		return 0
	}
	return len(s.Notifications) - s.MaxVisible
}

// ToMap converts the NotificationState to a map for template rendering.
func (s *NotificationState) ToMap() map[string]any {
	notifications := make([]map[string]any, len(s.VisibleNotifications()))
	for i, notif := range s.VisibleNotifications() {
		notifications[i] = map[string]any{
			"ID":          notif.ID,
			"Message":     notif.Message,
			"Type":        string(notif.Type),
			"AutoDismiss": notif.AutoDismiss,
			"Timer":       notif.Timer,
			"IsInfo":      notif.Type == NotificationInfo,
			"IsSuccess":   notif.Type == NotificationSuccess,
			"IsWarning":   notif.Type == NotificationWarning,
			"IsError":     notif.Type == NotificationError,
		}
	}

	return map[string]any{
		"Notifications": notifications,
		"MaxVisible":    s.MaxVisible,
		"HasOverflow":   s.HasOverflow(),
		"OverflowCount": s.OverflowCount(),
		"TotalCount":    len(s.Notifications),
	}
}

// -----------------------------------------------------------------------------
// Bulk Operations State
// -----------------------------------------------------------------------------

// BulkState represents the bulk operations state.
// Tests select all, bulk delete, batch updates, and selection management.
type BulkState struct {
	Items       []Item
	SelectedIDs map[string]bool
	SelectAll   bool
}

// DefaultBulkState returns a new BulkState with sensible defaults.
func DefaultBulkState() *BulkState {
	return &BulkState{
		Items:       []Item{},
		SelectedIDs: make(map[string]bool),
		SelectAll:   false,
	}
}

// Clone creates a deep copy of the BulkState.
func (s *BulkState) Clone() *BulkState {
	clone := &BulkState{
		Items:       make([]Item, len(s.Items)),
		SelectedIDs: make(map[string]bool),
		SelectAll:   s.SelectAll,
	}
	copy(clone.Items, s.Items)
	for k, v := range s.SelectedIDs {
		clone.SelectedIDs[k] = v
	}
	return clone
}

// SelectedCount returns the number of selected items.
func (s *BulkState) SelectedCount() int {
	if s.SelectAll {
		return len(s.Items)
	}
	count := 0
	for _, item := range s.Items {
		if s.SelectedIDs[item.ID] {
			count++
		}
	}
	return count
}

// HasSelection returns true if any items are selected.
func (s *BulkState) HasSelection() bool {
	return s.SelectedCount() > 0
}

// AllSelected returns true if all items are selected.
func (s *BulkState) AllSelected() bool {
	if len(s.Items) == 0 {
		return false
	}
	return s.SelectAll || s.SelectedCount() == len(s.Items)
}

// GetSelectedItems returns the list of selected items.
func (s *BulkState) GetSelectedItems() []Item {
	var selected []Item
	for _, item := range s.Items {
		if s.SelectAll || s.SelectedIDs[item.ID] {
			selected = append(selected, item)
		}
	}
	return selected
}

// ToMap converts the BulkState to a map for template rendering.
func (s *BulkState) ToMap() map[string]any {
	items := make([]map[string]any, len(s.Items))
	for i, item := range s.Items {
		isSelected := s.SelectAll || s.SelectedIDs[item.ID]
		items[i] = map[string]any{
			"ID":         item.ID,
			"Title":      item.Title,
			"Body":       item.Body,
			"Complete":   item.Complete,
			"Priority":   item.Priority,
			"CreatedAt":  item.CreatedAt,
			"IsSelected": isSelected,
		}
	}

	return map[string]any{
		"Items":         items,
		"SelectAll":     s.SelectAll,
		"HasSelection":  s.HasSelection(),
		"AllSelected":   s.AllSelected(),
		"SelectedCount": s.SelectedCount(),
		"TotalCount":    len(s.Items),
	}
}
