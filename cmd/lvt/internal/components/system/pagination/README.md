# Pagination Component

Pagination components with multiple modes for LiveTemplate applications including infinite scroll, load more button, prev/next navigation, and numbered pagination.

## Description

Provides flexible pagination templates with 4 different modes:
- **Infinite Scroll**: Automatic loading with intersection observer
- **Load More**: Manual load button with item count
- **Prev/Next**: Simple previous/next navigation
- **Numbered**: Full numbered page navigation

All modes are kit-aware and support responsive design.

## Usage

```go
{{template "pagination" .}}
```

The pagination template automatically selects the appropriate mode based on `PaginationMode` input.

## Inputs

- **PaginationMode** (string, required): Pagination mode - "infinite", "load-more", "prev-next", or "numbers"
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **HasMore** (bool, optional): Whether there are more items to load (infinite/load-more modes)
- **IsLoading** (bool, optional): Whether currently loading more items
- **TotalPages** (int, optional): Total number of pages (prev-next/numbers modes)
- **CurrentPage** (int, optional): Current page number (prev-next/numbers modes)
- **TotalCount** (int, optional): Total item count for display (load-more mode)
- **ResourceNamePlural** (string, optional): Plural resource name for item count display
- **PaginatedResourceNamePlural** (array, optional): Current paginated items array

## Templates

### `pagination`
Main pagination template that routes to the appropriate mode:
```go
{{template "pagination" .}}
```

### `infiniteScroll`
Infinite scroll with sentinel element:
```go
{{template "infiniteScroll" .}}
```

Features:
- Invisible sentinel div for intersection observer
- Loading indicator while fetching
- Only shown when HasMore is true
- Automatic loading via JavaScript observer

### `loadMoreButton`
Load more button with item count:
```go
{{template "loadMoreButton" .}}
```

Features:
- Manual load button with "Load More" text
- Loading state with spinner/text
- Item count display: "Showing X of Y items"
- Only shown when HasMore is true
- Uses `lvt-click="load_more"` action

### `prevNextPagination`
Simple previous/next navigation:
```go
{{template "prevNextPagination" .}}
```

Features:
- Previous and Next buttons
- Page info display: "Page X of Y"
- Disabled state for first/last pages
- Semantic nav element with ARIA labels
- Uses `lvt-click="prev_page"` and `lvt-click="next_page"` actions

### `numberedPagination`
Full numbered page navigation:
```go
{{template "numberedPagination" .}}
```

Features:
- Previous and Next buttons with arrows (« and »)
- First and last page numbers always visible
- Current page highlighted
- Ellipsis (...) for skipped pages
- Direct page navigation via `lvt-click="goto_page"`
- Semantic nav element with ARIA labels
- Responsive gap spacing

## Pagination Modes

### 1. Infinite Scroll (`infinite`)

Automatically loads more items when user scrolls to bottom.

**Required Inputs:**
- PaginationMode: "infinite"
- HasMore: bool
- IsLoading: bool

**Example:**
```go
data := struct {
  PaginationMode string
  HasMore        bool
  IsLoading      bool
}{
  PaginationMode: "infinite",
  HasMore:        true,
  IsLoading:      false,
}
```

**Client-Side:**
Requires JavaScript intersection observer setup (included in LiveTemplate client).

### 2. Load More (`load-more`)

Manual load button with item count display.

**Required Inputs:**
- PaginationMode: "load-more"
- HasMore: bool
- IsLoading: bool
- TotalCount: int
- ResourceNamePlural: string
- PaginatedResourceNamePlural: array

**Example:**
```go
data := struct {
  PaginationMode      string
  HasMore             bool
  IsLoading           bool
  TotalCount          int
  ResourceNamePlural  string
  PaginatedProducts   []Product
}{
  PaginationMode:      "load-more",
  HasMore:             true,
  IsLoading:           false,
  TotalCount:          150,
  ResourceNamePlural:  "Products",
  PaginatedProducts:   currentProducts, // len = 50
}
```

**Display:** "Showing 50 of 150 items"

### 3. Previous/Next (`prev-next`)

Simple previous and next navigation.

**Required Inputs:**
- PaginationMode: "prev-next"
- TotalPages: int
- CurrentPage: int

**Example:**
```go
data := struct {
  PaginationMode string
  TotalPages     int
  CurrentPage    int
}{
  PaginationMode: "prev-next",
  TotalPages:     10,
  CurrentPage:    3,
}
```

**Display:** "Page 3 of 10" with Previous/Next buttons

### 4. Numbered (`numbers`)

Full numbered page navigation with direct page links.

**Required Inputs:**
- PaginationMode: "numbers"
- TotalPages: int
- CurrentPage: int

**Example:**
```go
data := struct {
  PaginationMode string
  TotalPages     int
  CurrentPage    int
}{
  PaginationMode: "numbers",
  TotalPages:     20,
  CurrentPage:    5,
}
```

**Display:** "« Prev | 1 ... 5 ... 20 | Next »"

## Kit Integration

The pagination component uses kit helper functions for styling:
- `loadingClass` - Loading indicator style
- `buttonClass` - Button style (with variants)
- `paginationClass` - Pagination nav container style
- `paginationButtonClass` - Pagination button style
- `paginationInfoClass` - Page info container style
- `paginationCurrentClass` - Current page indicator style
- `paginationActiveClass` - Active page number style

## Actions

The pagination component uses LiveTemplate actions:

**Load More:**
```go
lvt-click="load_more"
```

**Previous Page:**
```go
lvt-click="prev_page"
```

**Next Page:**
```go
lvt-click="next_page"
```

**Go to Page:**
```go
lvt-click="goto_page" lvt-data-page="5"
```

## Examples

### Infinite Scroll List
```go
{{define "content"}}
  <h1>Products</h1>
  {{template "resourceTable" .}}
  {{template "pagination" .}}
{{end}}
```

### Load More with Button
```go
data := struct {
  PaginationMode      string
  HasMore             bool
  IsLoading           bool
  TotalCount          int
  ResourceNamePlural  string
  PaginatedOrders     []Order
}{
  PaginationMode:      "load-more",
  HasMore:             len(orders) < totalOrders,
  IsLoading:           false,
  TotalCount:          totalOrders,
  ResourceNamePlural:  "Orders",
  PaginatedOrders:     orders,
}
```

### Prev/Next Navigation
```go
data := struct {
  PaginationMode string
  TotalPages     int
  CurrentPage    int
}{
  PaginationMode: "prev-next",
  TotalPages:     calculateTotalPages(totalItems, pageSize),
  CurrentPage:    currentPage,
}
```

### Numbered Pages
```go
data := struct {
  PaginationMode string
  TotalPages     int
  CurrentPage    int
  CSSFramework   string
}{
  PaginationMode: "numbers",
  TotalPages:     50,
  CurrentPage:    10,
  CSSFramework:   "pico",
}
```

## Styling

The pagination component uses:
- Inline styles for critical layout (flexbox, gap, padding)
- Kit helper functions for theme-specific styling
- Semantic HTML with proper ARIA labels
- Disabled state handling for buttons

## Accessibility

- Semantic `<nav>` elements with `role="navigation"`
- ARIA labels: `aria-label="pagination"`
- Proper button disabled states
- Keyboard navigation support (via buttons)
- Screen reader friendly page indicators

## Client-Side Integration

### Infinite Scroll
Requires intersection observer setup:
```javascript
const observer = new IntersectionObserver((entries) => {
  if (entries[0].isIntersecting) {
    loadMore();
  }
});
observer.observe(document.getElementById('scroll-sentinel'));
```

### Action Handlers
Server-side handlers for pagination actions:
```go
case "load_more":
  // Load next batch of items
case "prev_page":
  // Load previous page
case "next_page":
  // Load next page
case "goto_page":
  // Load specific page from data.Page
```

## Notes

- The template uses `[[` `]]` delimiters for generation-time substitution
- Infinite scroll sentinel is invisible (height: 1px)
- Load more shows item count dynamically based on current array length
- Numbered pagination shows ellipsis for large page counts
- All modes check for HasMore/TotalPages before rendering
- Buttons are disabled (not hidden) when at boundaries
- Loading states prevent duplicate requests
