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
)

// Mutation represents a single state change operation.
type Mutation struct {
	Type   MutationType // The type of mutation
	Target string       // Field path (e.g., "Items", "User.Name", "Items.0.Text")
	Index  int          // For slice operations: target index
	Index2 int          // For swap operations: second index
	Value  interface{}  // New value (type depends on mutation type)
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
	ClearSlice:    0.00, // Disabled: clearing to empty not yet handled by oracle
	ReorderSlice:  0.08,
	ReverseSlice:  0.02,
	DuplicateItem: 0.02,
	SwapItems:     0.06, // Increased to compensate for disabled mutations

	// Nested mutations (15%)
	UpdateItem:  0.10,
	ReplaceItem: 0.05,

	// Edge cases and special mutations (10%)
	EdgeCases:    0.05,
	TypeFlip:     0.02,
	KeyCollision: 0.00, // Disabled: diff algorithm requires unique keys per item
}

// RangeHeavyWeights emphasizes range/slice operations for testing diff algorithm.
var RangeHeavyWeights = MutationWeights{
	SetField:   0.05,
	ToggleBool: 0.05,

	AppendSlice:   0.15,
	PrependSlice:  0.10,
	InsertSlice:   0.10,
	RemoveSlice:   0.15,
	ClearSlice:    0.00, // Disabled: clearing to empty not yet handled by oracle
	ReorderSlice:  0.15,
	ReverseSlice:  0.05,
	DuplicateItem: 0.03,
	SwapItems:     0.10, // Increased to compensate for disabled mutations

	UpdateItem:   0.05,
	KeyCollision: 0.00, // Disabled: diff algorithm requires unique keys per item
}

// EdgeCaseWeights emphasizes edge cases for finding subtle bugs.
var EdgeCaseWeights = MutationWeights{
	SetField:   0.05,
	ToggleBool: 0.05,

	AppendSlice:  0.05,
	RemoveSlice:  0.10, // Increased to compensate for disabled mutations
	ClearSlice:   0.00, // Disabled: clearing to empty not yet handled by oracle
	ReorderSlice: 0.05,

	EdgeCases:    0.30,
	TypeFlip:     0.20, // Increased to compensate for disabled mutations
	KeyCollision: 0.00, // Disabled: diff algorithm requires unique keys per item
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
