// Package mutations provides state mutation types and operations for fuzz testing.
package mutations

// MutationType defines the kind of mutation to apply to state.
type MutationType string

const (
	// Primitive field mutations
	MutSetField     MutationType = "set_field"     // Set a field to a new value
	MutToggleBool   MutationType = "toggle_bool"   // Toggle a boolean field
	MutIncrementInt MutationType = "increment_int" // Increment an integer field
	MutSetNil       MutationType = "set_nil"       // Set a field to nil
	MutSetEmpty     MutationType = "set_empty"     // Set a field to empty value ([], "", 0)

	// Slice mutations
	MutAppendSlice   MutationType = "append_slice"   // Append item to end of slice
	MutPrependSlice  MutationType = "prepend_slice"  // Prepend item to start of slice
	MutInsertSlice   MutationType = "insert_slice"   // Insert item at specific index
	MutRemoveSlice   MutationType = "remove_slice"   // Remove item at specific index
	MutClearSlice    MutationType = "clear_slice"    // Clear all items from slice
	MutReorderSlice  MutationType = "reorder_slice"  // Reorder items using permutation
	MutReverseSlice  MutationType = "reverse_slice"  // Reverse slice order
	MutDuplicateItem MutationType = "duplicate_item" // Duplicate an item in slice
	MutSwapItems     MutationType = "swap_items"     // Swap two items in slice

	// Nested/item mutations
	MutUpdateItem  MutationType = "update_item"  // Update fields within a slice item
	MutReplaceItem MutationType = "replace_item" // Replace entire slice item

	// Edge case mutations (for finding bugs)
	MutUnicodeString MutationType = "unicode_string" // Set field to unicode/emoji string
	MutLargeString   MutationType = "large_string"   // Set field to very large string
	MutSpecialChars  MutationType = "special_chars"  // Set field to HTML-sensitive chars
	MutEmptyString   MutationType = "empty_string"   // Set field to empty string
	MutZeroInt       MutationType = "zero_int"       // Set int field to zero
	MutNegativeInt   MutationType = "negative_int"   // Set int field to negative

	// Type mutations (for testing type handling)
	MutTypeFlip MutationType = "type_flip" // Change field type (string→int, etc.)

	// Key mutations (for testing key stability)
	MutKeyCollision MutationType = "key_collision" // Force duplicate IDs in range

	// Application-level mutations (for testing derived views)
	MutSetFilter       MutationType = "set_filter"        // Change filter: "all" → "active" → "completed"
	MutSetSort         MutationType = "set_sort"          // Change sort: "created" → "priority" → "alpha"
	MutToggleSortOrder MutationType = "toggle_sort_order" // Toggle "asc" ↔ "desc"
	MutSetSearch       MutationType = "set_search"        // Set search query
	MutClearSearch     MutationType = "clear_search"      // Clear search query

	// Nested range mutations (for testing ranges within ranges)
	MutToggleExpand       MutationType = "toggle_expand"        // Expand/collapse category
	MutAddToCategory      MutationType = "add_to_category"      // Add item to specific category
	MutRemoveFromCategory MutationType = "remove_from_category" // Remove item from category
	MutMoveToCategory     MutationType = "move_to_category"     // Move item between categories
	MutAddCategory        MutationType = "add_category"         // Add new category
	MutRemoveCategory     MutationType = "remove_category"      // Remove category
	MutReorderWithin      MutationType = "reorder_within"       // Reorder items within a category
	MutReorderCategories  MutationType = "reorder_categories"   // Reorder categories themselves
	MutUpdateCategory     MutationType = "update_category"      // Update category name

	// Pagination mutations (for testing page transitions)
	MutLoadMore       MutationType = "load_more"        // Load next page (append items)
	MutLoadPrevious   MutationType = "load_previous"    // Load previous page (prepend items)
	MutResetPage      MutationType = "reset_page"       // Reset to first page
	MutJumpToPage     MutationType = "jump_to_page"     // Jump to specific page
	MutPageSizeChange MutationType = "page_size_change" // Change items per page
	MutToggleLoading  MutationType = "toggle_loading"   // Toggle loading indicator

	// Modal/Panel mutations (for testing TreeNode transitions)
	MutOpenModal    MutationType = "open_modal"    // Open a modal (push to stack)
	MutCloseModal   MutationType = "close_modal"   // Close a modal (pop from stack)
	MutCloseAll     MutationType = "close_all"     // Close all modals
	MutUpdateModal  MutationType = "update_modal"  // Update modal content while open
	MutSwitchPanel  MutationType = "switch_panel"  // Switch active panel
	MutTogglePanel  MutationType = "toggle_panel"  // Toggle panel visibility
	MutReorderModal MutationType = "reorder_modal" // Change modal priority/order

	// Form mutations (for testing validation states)
	MutSetFieldValue   MutationType = "set_field_value"   // Set form field value
	MutSetFieldError   MutationType = "set_field_error"   // Set field error message
	MutClearFieldError MutationType = "clear_field_error" // Clear field error
	MutTouchField      MutationType = "touch_field"       // Mark field as touched
	MutResetForm       MutationType = "reset_form"        // Reset entire form
	MutSubmitStart     MutationType = "submit_start"      // Start form submission
	MutSubmitSuccess   MutationType = "submit_success"    // Form submitted successfully
	MutSubmitError     MutationType = "submit_error"      // Form submission failed

	// Async loading mutations (for testing loading states)
	MutStartLoading     MutationType = "start_loading"     // Start global loading
	MutFinishLoading    MutationType = "finish_loading"    // Finish global loading
	MutStartItemLoad    MutationType = "start_item_load"   // Start per-item loading
	MutFinishItemLoad   MutationType = "finish_item_load"  // Finish per-item loading
	MutSetItemError     MutationType = "set_item_error"    // Set per-item error
	MutClearItemError   MutationType = "clear_item_error"  // Clear per-item error
	MutOptimisticUpdate MutationType = "optimistic_update" // Optimistic update
	MutRollbackUpdate   MutationType = "rollback_update"   // Rollback optimistic update

	// Notification queue mutations
	MutAddNotification     MutationType = "add_notification"     // Add notification to queue
	MutDismissNotification MutationType = "dismiss_notification" // Dismiss specific notification
	MutDismissAll          MutationType = "dismiss_all"          // Dismiss all notifications
	MutExpireNotification  MutationType = "expire_notification"  // Auto-dismiss by timer

	// Bulk operations mutations
	MutToggleSelect    MutationType = "toggle_select"    // Toggle single item selection
	MutSelectAll       MutationType = "select_all"       // Select all items
	MutDeselectAll     MutationType = "deselect_all"     // Deselect all items
	MutInvertSelection MutationType = "invert_selection" // Invert current selection
	MutBulkDelete      MutationType = "bulk_delete"      // Delete all selected items
	MutBulkUpdate      MutationType = "bulk_update"      // Update all selected items
)

// Mutation represents a single state change operation.
type Mutation struct {
	Type   MutationType // The type of mutation
	Target string       // Field path (e.g., "Items", "User.Name", "Items.0.Text")
	Index  int          // For slice operations: target index
	Index2 int          // For swap operations: second index
	Value  any          // New value (type depends on mutation type)
}

// MutationWeights controls the probability distribution of mutation types.
// Values should sum to 1.0 for proper weighting.
type MutationWeights struct {
	// Primitive mutations
	SetField     float64
	ToggleBool   float64
	IncrementInt float64
	SetNil       float64
	SetEmpty     float64

	// Slice mutations
	AppendSlice   float64
	PrependSlice  float64
	InsertSlice   float64
	RemoveSlice   float64
	ClearSlice    float64
	ReorderSlice  float64
	ReverseSlice  float64
	DuplicateItem float64
	SwapItems     float64

	// Nested mutations
	UpdateItem  float64
	ReplaceItem float64

	// Edge cases
	EdgeCases float64

	// Type/Key mutations
	TypeFlip     float64
	KeyCollision float64

	// Application-level mutations
	SetFilter       float64
	SetSort         float64
	ToggleSortOrder float64
	SetSearch       float64
	ClearSearch     float64

	// Nested range mutations
	ToggleExpand       float64
	AddToCategory      float64
	RemoveFromCategory float64
	MoveToCategory     float64
	AddCategory        float64
	RemoveCategory     float64
	ReorderWithin      float64
	ReorderCategories  float64
	UpdateCategory     float64

	// Pagination mutations
	LoadMore       float64
	LoadPrevious   float64
	ResetPage      float64
	JumpToPage     float64
	PageSizeChange float64
	ToggleLoading  float64

	// Modal/Panel mutations
	OpenModal    float64
	CloseModal   float64
	CloseAll     float64
	UpdateModal  float64
	SwitchPanel  float64
	TogglePanel  float64
	ReorderModal float64

	// Form mutations
	SetFieldValue   float64
	SetFieldError   float64
	ClearFieldError float64
	TouchField      float64
	ResetForm       float64
	SubmitStart     float64
	SubmitSuccess   float64
	SubmitError     float64

	// Async loading mutations
	StartLoading     float64
	FinishLoading    float64
	StartItemLoad    float64
	FinishItemLoad   float64
	SetItemError     float64
	ClearItemError   float64
	OptimisticUpdate float64
	RollbackUpdate   float64

	// Notification queue mutations
	AddNotification     float64
	DismissNotification float64
	DismissAll          float64
	ExpireNotification  float64

	// Bulk operations mutations
	ToggleSelect    float64
	SelectAll       float64
	DeselectAll     float64
	InvertSelection float64
	BulkDelete      float64
	BulkUpdate      float64
}

// DefaultWeights returns a balanced weight distribution for general fuzzing.
// Emphasizes common operations while still covering edge cases.
var DefaultWeights = MutationWeights{
	// Primitive (25%)
	SetField:     0.10,
	ToggleBool:   0.08,
	IncrementInt: 0.04,
	SetNil:       0.01,
	SetEmpty:     0.02,

	// Slice operations (50% - most bugs are here)
	AppendSlice:   0.12,
	PrependSlice:  0.05,
	InsertSlice:   0.05,
	RemoveSlice:   0.10,
	ClearSlice:    0.05, // ENABLED: tests items→empty transitions (historical bugs a87fc41, db329a5)
	ReorderSlice:  0.06,
	ReverseSlice:  0.02,
	DuplicateItem: 0.02,
	SwapItems:     0.03,

	// Nested mutations (15%)
	UpdateItem:  0.10,
	ReplaceItem: 0.05,

	// Edge cases and special mutations (10%)
	EdgeCases:    0.05,
	TypeFlip:     0.02,
	KeyCollision: 0.00, // DISABLED: diff algorithm requires unique keys (documented constraint)
}

// RangeHeavyWeights emphasizes range/slice operations for testing diff algorithm.
var RangeHeavyWeights = MutationWeights{
	SetField:   0.05,
	ToggleBool: 0.05,

	AppendSlice:   0.12,
	PrependSlice:  0.08,
	InsertSlice:   0.08,
	RemoveSlice:   0.12,
	ClearSlice:    0.08, // ENABLED: tests items→empty transitions
	ReorderSlice:  0.12,
	ReverseSlice:  0.05,
	DuplicateItem: 0.03,
	SwapItems:     0.07,

	UpdateItem:   0.05,
	KeyCollision: 0.00, // DISABLED: diff algorithm requires unique keys (documented constraint)
}

// EdgeCaseWeights emphasizes edge cases for finding subtle bugs.
var EdgeCaseWeights = MutationWeights{
	SetField:   0.05,
	ToggleBool: 0.05,

	AppendSlice:  0.05,
	RemoveSlice:  0.08,
	ClearSlice:   0.10, // ENABLED: tests items→empty transitions (critical edge case)
	ReorderSlice: 0.05,

	EdgeCases:    0.25,
	TypeFlip:     0.15,
	KeyCollision: 0.00, // DISABLED: diff algorithm requires unique keys (documented constraint)
}

// AppOperationsWeights emphasizes application-level operations for testing derived views.
// These mutations change view settings (filter, sort, search) which transform entire lists.
var AppOperationsWeights = MutationWeights{
	// Existing CRUD (35%)
	AppendSlice:  0.08,
	PrependSlice: 0.04,
	RemoveSlice:  0.08,
	ClearSlice:   0.03,
	ReorderSlice: 0.04,
	UpdateItem:   0.08,

	// View controls (45%) - trigger derived view recalculation
	SetFilter:       0.12,
	SetSort:         0.10,
	ToggleSortOrder: 0.05,
	SetSearch:       0.10,
	ClearSearch:     0.08,

	// Conditionals (15%)
	ToggleBool: 0.10, // ShowSearch, ShowFilters
	SetField:   0.05, // SelectedID

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// NestedRangeWeights emphasizes nested range operations for testing ranges within ranges.
// These mutations test key namespace collisions, statics at multiple nesting levels,
// and reorder operations at different hierarchy levels.
var NestedRangeWeights = MutationWeights{
	// Category operations (40%)
	AddCategory:       0.10,
	RemoveCategory:    0.08,
	ReorderCategories: 0.07,
	UpdateCategory:    0.05,
	ToggleExpand:      0.10,

	// Item within category operations (45%)
	AddToCategory:      0.12,
	RemoveFromCategory: 0.10,
	MoveToCategory:     0.08,
	ReorderWithin:      0.08,
	UpdateItem:         0.07,

	// Edge cases (15%)
	EdgeCases:  0.10,
	ClearSlice: 0.05, // Clear all items in a category
}

// PaginationWeights emphasizes pagination operations for testing page transitions.
// These mutations test key stability across pages, loading states, and page jumps.
var PaginationWeights = MutationWeights{
	// Page navigation (50%)
	LoadMore:     0.15,
	LoadPrevious: 0.10,
	ResetPage:    0.08,
	JumpToPage:   0.10,

	// Page configuration (15%)
	PageSizeChange: 0.08,
	ToggleLoading:  0.07,

	// Item CRUD (25%)
	AppendSlice: 0.08,
	RemoveSlice: 0.08,
	UpdateItem:  0.09,

	// Edge cases (10%)
	EdgeCases:  0.05,
	ClearSlice: 0.05,
}

// ModalWeights emphasizes modal/panel operations for testing TreeNode transitions.
// These mutations test conditional statics, modal stacking, and panel visibility.
var ModalWeights = MutationWeights{
	// Modal operations (50%)
	OpenModal:    0.15,
	CloseModal:   0.12,
	CloseAll:     0.05,
	UpdateModal:  0.10,
	ReorderModal: 0.08,

	// Panel operations (30%)
	SwitchPanel: 0.15,
	TogglePanel: 0.15,

	// Conditionals (15%)
	ToggleBool: 0.10,
	SetField:   0.05,

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// FormWeights emphasizes form validation operations for testing error states.
// These mutations test error message transitions, touched states, and submit flows.
// Note: ResetForm is excluded because mass clearing of fields causes oracle divergence
// when conditionals within range items change simultaneously.
var FormWeights = MutationWeights{
	// Field operations (55%)
	SetFieldValue:   0.25,
	SetFieldError:   0.15,
	ClearFieldError: 0.10,
	TouchField:      0.05,

	// Submit flow (30%)
	SubmitStart:   0.12,
	SubmitSuccess: 0.10,
	SubmitError:   0.08,

	// Conditionals (10%)
	ToggleBool: 0.10,

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// AsyncWeights emphasizes async loading operations for testing loading states.
// These mutations test loading indicators, per-item states, and error recovery.
var AsyncWeights = MutationWeights{
	// Global loading (25%)
	StartLoading:  0.12,
	FinishLoading: 0.13,

	// Per-item loading (35%)
	StartItemLoad:  0.12,
	FinishItemLoad: 0.13,
	SetItemError:   0.05,
	ClearItemError: 0.05,

	// Item CRUD (25%)
	AppendSlice: 0.10,
	RemoveSlice: 0.08,
	UpdateItem:  0.07,

	// Optimistic updates (10%)
	OptimisticUpdate: 0.05,
	RollbackUpdate:   0.05,

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// NotificationWeights emphasizes notification queue operations for testing queue management.
// These mutations test add/dismiss, auto-dismiss timers, and overflow handling.
var NotificationWeights = MutationWeights{
	// Add notifications (40%)
	AddNotification: 0.40,

	// Dismiss operations (40%)
	DismissNotification: 0.20,
	DismissAll:          0.10,
	ExpireNotification:  0.10,

	// Toggle operations (15%)
	ToggleBool: 0.15,

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// BulkWeights emphasizes bulk operations for testing selection management.
// These mutations test select all, bulk delete, and batch updates.
var BulkWeights = MutationWeights{
	// Selection operations (45%)
	ToggleSelect:    0.15,
	SelectAll:       0.10,
	DeselectAll:     0.10,
	InvertSelection: 0.10,

	// Bulk actions (25%)
	BulkDelete: 0.12,
	BulkUpdate: 0.13,

	// Item CRUD (25%)
	AppendSlice: 0.10,
	RemoveSlice: 0.08,
	UpdateItem:  0.07,

	// Edge cases (5%)
	EdgeCases: 0.05,
}

// EdgeCaseStrings contains interesting strings for edge case testing.
var EdgeCaseStrings = []string{
	// Unicode and emoji
	"Hello 🎉 World",
	"日本語テスト",
	"Привет мир",
	"مرحبا بالعالم",
	"🎉🎊🎈🎁",

	// Zero-width and special unicode
	"\u200B\u200C\u200D",
	"a\u0308",
	"\uFEFF",

	// HTML-sensitive
	"<script>alert('xss')</script>",
	"<div>test</div>",
	"a & b",
	"\"quoted\"",
	"'single'",
	"<>\"'&",

	// Template-like patterns
	"{{.Field}}",
	"{{ }}",
	"}}{{",
	"${var}",
	"<%=var%>",

	// Special characters
	"foo\x00bar",
	"line1\nline2\rline3\r\nline4",
	"\t\t\ttabs",

	// Empty-ish
	"",
	" ",
	"   ",
}

// String returns a human-readable representation of the mutation.
func (m Mutation) String() string {
	switch m.Type {
	case MutSetField, MutToggleBool, MutIncrementInt, MutSetNil, MutSetEmpty,
		MutUnicodeString, MutLargeString, MutSpecialChars, MutEmptyString,
		MutZeroInt, MutNegativeInt, MutTypeFlip:
		return string(m.Type) + "(" + m.Target + ")"

	case MutAppendSlice, MutPrependSlice, MutClearSlice, MutReverseSlice, MutKeyCollision:
		return string(m.Type) + "(" + m.Target + ")"

	case MutInsertSlice, MutRemoveSlice, MutDuplicateItem, MutUpdateItem, MutReplaceItem:
		return string(m.Type) + "(" + m.Target + "[" + itoa(m.Index) + "])"

	case MutReorderSlice:
		return string(m.Type) + "(" + m.Target + ")"

	case MutSwapItems:
		return string(m.Type) + "(" + m.Target + "[" + itoa(m.Index) + "," + itoa(m.Index2) + "])"

	default:
		return string(m.Type)
	}
}

func itoa(i int) string {
	if i < 0 {
		return "-" + itoa(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}
