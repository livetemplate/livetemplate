# Detail Component

Detail page component for viewing and editing individual resources in page mode for LiveTemplate applications.

## Description

Provides a complete detail page for single resource view/edit with:
- View mode with back, edit, and delete buttons
- Edit mode with back button and edit form
- Field-level detail display with type-specific formatting
- Navigation controls with proper routing
- Kit-aware styling
- Maximum width constraint for readability
- Integration with editForm component

## Usage

```go
{{template "detailPage" .}}
```

## Inputs

- **ResourceName** (string, required): Display name of the resource (e.g., "Product", "User")
- **ResourceNameSingular** (string, required): Singular display name (e.g., "Product", "User")
- **ResourceNameLower** (string, required): Lowercase singular name (e.g., "product", "user")
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **EditingResourceName** (object, required): The resource object (dynamic key based on ResourceName)
- **EditingID** (string, required): ID of the resource being viewed/edited
- **IsEditingMode** (bool, required): Whether in edit mode (true) or view mode (false)
- **Fields** (array, required): Array of field definitions with Name, GoType, IsTextarea properties

## Templates

### `detailPage`
Complete detail page with view/edit modes:
```go
{{template "detailPage" .}}
```

Features:
- Conditional rendering based on IsEditingMode
- Navigation bar with action buttons
- Field display with type-specific formatting
- Edit form integration
- Maximum width container for readability

## Modes

### View Mode (`IsEditingMode: false`)

Navigation bar contains:
- **Back button**: Links to list page (`/resource-name`)
- **Edit button**: Links to edit page (`/resource-name/{id}/edit`)
- **Delete button**: Triggers delete action with confirmation

Detail section displays:
- Section heading: "[ResourceName] Details"
- All fields in read-only view
- Type-specific formatting:
  - **Textarea**: Preserves whitespace with `white-space: pre-wrap`
  - **Bool**: Shows ✓ Yes or ✗ No
  - **Time**: Formatted as "2006-01-02 15:04"
  - **Other**: Direct value display

### Edit Mode (`IsEditingMode: true`)

Navigation bar contains:
- **Back button**: Links to detail page (`/resource-name/{id}`)

Content section:
- Renders the `editForm` template
- Full form with update and delete actions
- Form handles its own navigation after submit

## Field Display

The detail view displays all fields with different formatting based on type:

**String/Int/Float:**
```html
<div>Value</div>
```

**Textarea:**
```html
<div style="white-space: pre-wrap;">
  Multiline text
  with preserved
  formatting
</div>
```

**Boolean:**
```html
✓ Yes  (if true)
✗ No   (if false)
```

**Time:**
```html
2025-10-16 18:39  (formatted)
```

## Kit Integration

The detail component uses kit helper functions for styling:
- `buttonClass` - Button styling with variants (secondary, primary, danger)
- `subtitleClass` - Section heading style
- `fieldClass` - Field wrapper style
- `labelClass` - Label style

## Actions

The detail component uses LiveTemplate actions:

**Delete (View Mode):**
```go
lvt-click="delete"
lvt-data-id="{{.EditingID}}"
lvt-confirm="Are you sure you want to delete this [resource]?"
```

## Examples

### View Mode
```go
data := struct {
  ResourceName         string
  ResourceNameSingular string
  ResourceNameLower    string
  CSSFramework         string
  EditingProduct       Product
  EditingID            string
  IsEditingMode        bool
  Fields               []Field
}{
  ResourceName:         "Product",
  ResourceNameSingular: "Product",
  ResourceNameLower:    "product",
  CSSFramework:         "tailwind",
  EditingProduct:       product,
  EditingID:            "prod-123",
  IsEditingMode:        false,
  Fields:               productFields,
}

{{template "detailPage" .}}
```

### Edit Mode
```go
data := struct {
  ResourceName         string
  ResourceNameLower    string
  EditingUser          User
  EditingID            string
  IsEditingMode        bool
  Fields               []Field
}{
  ResourceName:      "User",
  ResourceNameLower: "user",
  EditingUser:       user,
  EditingID:         "user-456",
  IsEditingMode:     true,
  Fields:            userFields,
}

{{template "detailPage" .}}
```

### With Complex Fields
```go
fields := []Field{
  {Name: "title", GoType: "string", IsTextarea: false},
  {Name: "description", GoType: "string", IsTextarea: true},
  {Name: "price", GoType: "float64", IsTextarea: false},
  {Name: "enabled", GoType: "bool", IsTextarea: false},
  {Name: "created_at", GoType: "time.Time", IsTextarea: false},
}

// View will display:
// - Title: text value
// - Description: multiline with preserved formatting
// - Price: float value
// - Enabled: ✓ Yes or ✗ No
// - Created At: 2025-10-16 18:39
```

## Navigation Flow

**Typical page flow:**

1. **List Page** (`/products`)
   - Click on product → Navigate to detail view

2. **Detail View** (`/products/prod-123`)
   - Back → List page
   - Edit → Edit mode
   - Delete → Confirmation → List page

3. **Edit Mode** (`/products/prod-123/edit`)
   - Back → Detail view
   - Save → Detail view (handled by editForm)
   - Delete → Confirmation → List page (handled by editForm)

## Styling

The detail component uses:
- Inline styles for critical layout:
  - Flexbox for navigation bar
  - Border-bottom for visual separation
  - Max-width (600px) for readability
  - Padding and gaps for spacing
  - Font-weight for labels
  - White-space preservation for textarea
- Kit helper functions for theme-specific styling
- Responsive design considerations

## Integration Requirements

The detail component requires:
- **editForm component**: Must be available for edit mode rendering
- **Page routing**: URL structure `/resource/{id}` and `/resource/{id}/edit`
- **Server-side handlers**:
  - GET `/resource/{id}` - Load resource and render with IsEditingMode=false
  - GET `/resource/{id}/edit` - Load resource and render with IsEditingMode=true

## Accessibility

- Semantic HTML structure
- Proper button types
- Link elements for navigation
- Confirmation dialogs for destructive actions
- Descriptive labels
- Keyboard navigation support

## Server-Side Example

```go
// View mode
func detailHandler(w http.ResponseWriter, r *http.Request) {
  id := getIDFromURL(r)
  product := loadProduct(id)

  data := DetailData{
    ResourceName:         "Product",
    ResourceNameSingular: "Product",
    ResourceNameLower:    "product",
    EditingProduct:       product,
    EditingID:            id,
    IsEditingMode:        false,
    Fields:               productFields,
  }

  render(w, "detailPage", data)
}

// Edit mode
func editHandler(w http.ResponseWriter, r *http.Request) {
  id := getIDFromURL(r)
  product := loadProduct(id)

  data := DetailData{
    ResourceName:      "Product",
    ResourceNameLower: "product",
    EditingProduct:    product,
    EditingID:         id,
    IsEditingMode:     true,
    Fields:            productFields,
  }

  render(w, "detailPage", data)
}
```

## Notes

- The template uses `[[` `]]` delimiters for generation-time substitution
- EditingResourceName key is dynamic: `Editing{{.ResourceName}}`
- Resource accessors use camelCase: `.Editing{{.ResourceName}}.{{.Name | camelCase}}`
- Maximum width (600px) ensures readability for detail view
- White-space pre-wrap preserves textarea formatting
- Confirmation required for delete action
- Edit mode delegates to editForm component
- Navigation URLs follow RESTful conventions
