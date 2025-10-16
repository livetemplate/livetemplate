# Toolbar Component

Toolbar component with integrated search, sort, and add button functionality for LiveTemplate applications.

## Description

Provides a complete toolbar for resource management including:
- Debounced search input
- Dynamic sort dropdown with field-based options
- Add button with modal integration
- Kit-aware wrapper (article or div based on framework)
- Responsive flexbox layout
- Field type filtering for sort options

## Usage

```go
{{template "toolbar" .}}
```

## Inputs

- **ResourceNameLower** (string, required): Lowercase singular name (e.g., "product", "user")
- **ResourceNameSingular** (string, required): Singular display name (e.g., "Product", "User")
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **SearchQuery** (string, optional): Current search query value
- **SortBy** (string, optional): Current sort field and direction
- **Fields** (array, required): Array of field definitions with Name, GoType properties

## Templates

### `toolbar`
Complete toolbar with search, sort, and add functionality:
```go
{{template "toolbar" .}}
```

Features:
- Kit-aware wrapper (article or div)
- Flexbox layout with gap and wrapping
- Three main sections: search, sort, add button
- Responsive design with minimum widths
- Semantic HTML structure

## Components

### Search Input
Debounced search with live updates:
- Search input type for browser integration
- Placeholder text with resource name
- Value preservation from SearchQuery
- LiveTemplate action: `lvt-change="search"`
- Debounce: 300ms via `lvt-debounce="300"`
- Kit-aware styling via `inputClass`

### Sort Dropdown
Dynamic sort options based on field types:
- Default options: "Newest First", "Oldest First"
- String fields: Alphabetical (A-Z, Z-A)
- Selected state preservation from SortBy
- LiveTemplate action: `lvt-change="sort"`
- Kit-aware styling via `selectClass` and `selectWrapperClass`

### Add Button
Modal trigger button:
- Opens add-modal via `lvt-modal-open="add-modal"`
- Display: "+ Add [ResourceName]"
- Kit-aware styling via `buttonClass` with "primary" variant

## Sort Options

The toolbar automatically generates sort options based on field types:

**Default Options:**
- "" (empty): Newest First
- "oldest_first": Oldest First

**String Field Options:**
For each string field, generates:
- "[field]_asc": Field Name (A-Z)
- "[field]_desc": Field Name (Z-A)

**Example:**
If Fields contains:
```go
{Name: "name", GoType: "string"}
{Name: "price", GoType: "float64"}
{Name: "category", GoType: "string"}
```

Sort options will be:
- Newest First
- Name (A-Z) / Name (Z-A)
- Category (A-Z) / Category (Z-A)
- Oldest First

Note: Only string fields generate sort options. Other types (int, float, bool, time) are excluded.

## Kit Integration

The toolbar uses kit helper functions for styling:
- `needsArticle` - Determines if kit needs semantic `<article>` wrapper
- `boxClass` - Container box styling
- `inputClass` - Search input styling
- `selectClass` - Sort dropdown styling
- `selectWrapperClass` - Optional wrapper for select element
- `buttonClass` - Button styling with variants (primary)

## Actions

The toolbar uses LiveTemplate actions:

**Search:**
```go
lvt-change="search" lvt-debounce="300"
```
Triggers search action on input change, debounced by 300ms.

**Sort:**
```go
lvt-change="sort"
```
Triggers sort action on dropdown change.

**Add:**
```go
lvt-modal-open="add-modal"
```
Opens the add-modal for creating new resources.

## Examples

### Basic Toolbar
```go
data := struct {
  ResourceNameLower    string
  ResourceNameSingular string
  CSSFramework         string
  SearchQuery          string
  SortBy               string
  Fields               []Field
}{
  ResourceNameLower:    "product",
  ResourceNameSingular: "Product",
  CSSFramework:         "tailwind",
  SearchQuery:          "",
  SortBy:               "",
  Fields:               productFields,
}

{{template "toolbar" .}}
```

### With Active Search
```go
data := struct {
  ResourceNameLower    string
  ResourceNameSingular string
  SearchQuery          string
  SortBy               string
  Fields               []Field
}{
  ResourceNameLower:    "user",
  ResourceNameSingular: "User",
  SearchQuery:          "john",
  SortBy:               "name_asc",
  Fields:               userFields,
}
```

### Custom Fields
```go
fields := []Field{
  {Name: "title", GoType: "string"},
  {Name: "author", GoType: "string"},
  {Name: "price", GoType: "float64"},
  {Name: "published", GoType: "bool"},
}

// Sort options generated:
// - Newest First
// - Title (A-Z) / Title (Z-A)
// - Author (A-Z) / Author (Z-A)
// - Oldest First
```

### Pico CSS Framework
```go
data := struct {
  ResourceNameLower    string
  ResourceNameSingular string
  CSSFramework         string
  Fields               []Field
}{
  ResourceNameLower:    "order",
  ResourceNameSingular: "Order",
  CSSFramework:         "pico", // Uses <article> wrapper
  Fields:               orderFields,
}
```

## Styling

The toolbar uses:
- Inline styles for critical layout:
  - `display: flex` - Flexbox layout
  - `gap: 1rem` - Spacing between items
  - `align-items: center` - Vertical alignment
  - `flex-wrap: wrap` - Responsive wrapping
  - `flex: 1` - Search input stretches
  - `min-width` - Minimum widths for responsive design
- Kit helper functions for theme-specific styling
- Semantic HTML with proper form elements

## Responsive Design

The toolbar is fully responsive:
- **Desktop**: Single row with search, sort, button
- **Tablet**: Wraps to maintain usability
- **Mobile**: Stacks vertically via flex-wrap

Minimum widths ensure:
- Search: 200px minimum
- Sort: 200px minimum
- Button: Natural size

## Server-Side Handlers

Example action handlers:

```go
case "search":
  query := data.Get("query")
  // Filter resources by query
  // Re-render with filtered results

case "sort":
  sortBy := data.Get("sort_by")
  // Parse sortBy (e.g., "name_asc")
  // Sort resources accordingly
  // Re-render with sorted results
```

## Accessibility

- Semantic HTML form elements
- Proper input types (search)
- Placeholder text for guidance
- Selected state for current sort
- Keyboard navigation support

## Notes

- The template uses `[[` `]]` delimiters for generation-time substitution
- Search debounce prevents excessive server calls
- Only string fields appear in sort options
- Sort field names use underscore separator: "field_direction"
- Wrapper type (article vs div) is kit-dependent
- Add button requires corresponding add-modal component
