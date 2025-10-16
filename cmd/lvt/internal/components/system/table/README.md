# Table Component

Table components for displaying resource lists with kit-aware styling for LiveTemplate applications.

## Description

Provides reusable table templates for resource listings including:
- Responsive data tables
- Kit-aware wrapper (article or div based on framework)
- Fixed table layout for consistent column widths
- Edit actions (modal or page-based)
- Empty state handling
- Search result messaging
- Data type-specific rendering (bool, time, string, etc.)

## Usage

```go
{{template "tableBox" .}}
{{template "resourceTable" .}}
```

## Inputs

- **ResourceNamePlural** (string, required): Plural name of the resource (e.g., "Products", "Users")
- **ResourceNameLower** (string, required): Lowercase singular name (e.g., "product", "user")
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **Fields** (array, required): Array of field definitions with Name, GoType properties
- **PaginatedResourceNamePlural** (array, required): Paginated list of resources to display
- **EditMode** (string, optional): Edit mode ("modal" or "page"), defaults to "modal"
- **SearchQuery** (string, optional): Current search query for empty state message

## Templates

### `tableBox`
Semantic wrapper for table content based on kit requirements:
```go
{{define "tableContent"}}
  <!-- Your table content here -->
{{end}}
{{template "tableBox" .}}
```

Features:
- Uses `<article>` for kits that need semantic HTML (like Pico)
- Uses `<div>` with optional box class for other kits
- Kit-aware wrapper selection via `needsArticle` helper
- Automatic wrapper styling via `boxClass` helper

### `resourceTable`
Table for displaying resource data:
```go
{{template "resourceTable" .}}
```

Features:
- Responsive table with fixed layout
- Displays first field as primary column
- Data type-specific rendering:
  - `bool`: ✓ or ✗ symbols
  - `time.Time`: Formatted as "2006-01-02 15:04"
  - Other types: Direct display
- Edit mode support:
  - **modal**: Shows "Edit" button in second column
  - **page**: Makes entire row clickable link to detail page
- Table wrapper for scrolling (kit-dependent)
- Empty state with contextual messages
- Search result feedback

## Kit Integration

The table component uses kit helper functions for styling:
- `needsArticle` - Determines if kit needs semantic `<article>` wrapper
- `boxClass` - Container box styling
- `needsTableWrapper` - Determines if kit needs scrollable wrapper div
- `tableWrapperClass` - Wrapper div styling
- `tableClass` - Table element styling
- `subtitleClass` - Heading style
- `buttonClass` - Button style (for edit buttons)
- `displayField` - Determines which field to display in table

## Edit Modes

### Modal Mode (default)
```go
EditMode: "modal"
```
- Displays two columns: main field + edit button
- Edit button triggers modal dialog
- Button uses `lvt-click="edit"` with `lvt-data-id`

### Page Mode
```go
EditMode: "page"
```
- Single column with clickable row
- Links to detail page: `/resourcename/{id}`
- No inline edit button
- Entire cell is clickable

## Data Type Rendering

The table automatically handles different data types:

**Boolean fields:**
```go
{{if .Enabled}}✓{{else}}✗{{end}}
```

**Time fields:**
```go
{{.CreatedAt.Format "2006-01-02 15:04"}}
```

**Other types:**
```go
{{.Name}}
```

## Empty States

The table provides contextual empty state messages:

**With search query:**
```
No products found matching "search term"
```

**Without search query:**
```
No products yet. Add one above!
```

## Examples

### Basic Table
```go
{{define "tableContent"}}
  {{template "resourceTable" .}}
{{end}}
{{template "tableBox" .}}
```

### Modal Edit Mode
```go
data := struct {
  ResourceNamePlural string
  ResourceNameLower  string
  CSSFramework       string
  Fields             []Field
  PaginatedProducts  []Product
  EditMode           string
}{
  ResourceNamePlural: "Products",
  ResourceNameLower:  "product",
  CSSFramework:       "tailwind",
  Fields:             fields,
  PaginatedProducts:  products,
  EditMode:           "modal",
}
```

### Page Edit Mode
```go
data := struct {
  ResourceNamePlural string
  ResourceNameLower  string
  CSSFramework       string
  Fields             []Field
  PaginatedUsers     []User
  EditMode           string
}{
  ResourceNamePlural: "Users",
  ResourceNameLower:  "user",
  CSSFramework:       "pico",
  Fields:             fields,
  PaginatedUsers:     users,
  EditMode:           "page",
}
```

### With Search
```go
data := struct {
  ResourceNamePlural string
  ResourceNameLower  string
  PaginatedOrders    []Order
  SearchQuery        string
}{
  ResourceNamePlural: "Orders",
  ResourceNameLower:  "order",
  PaginatedOrders:    filteredOrders,
  SearchQuery:        "pending",
}
```

## Styling

The table uses:
- Inline styles for critical layout (fixed table layout, cell padding, word wrapping)
- Kit helper functions for theme-specific styling
- Responsive design considerations
- Proper semantic HTML based on kit requirements

Key inline styles:
- `table-layout: fixed` - Ensures consistent column widths
- `word-wrap: break-word` - Prevents overflow in cells
- `white-space: nowrap` - Keeps edit button on single line
- `width: 70px` - Fixed width for edit button column

## Accessibility

- Proper semantic HTML (table structure)
- Data keys for efficient updates: `data-key="{{.ID}}"`
- Clickable links with proper href in page mode
- Button elements for actions in modal mode

## Notes

- The template uses `[[` `]]` delimiters for generation-time substitution
- Table uses `displayField` helper to determine which field to show
- The `Paginated` prefix is dynamic based on ResourceNamePlural
- Row data is accessed using proper key iteration
- Table wrapper is conditionally rendered based on kit requirements
- Fixed layout ensures predictable column widths across different data
