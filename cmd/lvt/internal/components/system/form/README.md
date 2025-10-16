# Form Component

Form components for adding and editing resources with modal support for LiveTemplate applications.

## Description

Provides reusable form templates for CRUD operations including:
- Add form with modal wrapper
- Edit form with update/delete actions
- Field-level error handling
- Kit-aware styling
- Built-in form validation
- Modal dialog support

## Usage

```go
{{template "addModal" .}}
{{template "addForm" .}}
{{template "editForm" .}}
```

## Inputs

- **ResourceName** (string, required): Display name of the resource (e.g., "Product", "User")
- **ResourceNameLower** (string, required): Lowercase name of the resource (e.g., "product", "user")
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **Fields** (array, required): Array of field definitions with properties:
  - Name (string): Field name
  - GoType (string): Go type ("string", "int64", "bool", "float64")
  - IsTextarea (bool): Whether to render as textarea
- **EditingID** (string, optional): ID of the resource being edited (for editForm only)

## Templates

### `addModal`
Modal wrapper for the add form with backdrop:
```go
{{template "addModal" .}}
```

Features:
- Fixed position overlay with semi-transparent backdrop
- Centered modal dialog
- Close button
- Responsive width (90% max 600px)
- Scrollable content area

### `addForm`
Form for adding new resources:
```go
{{template "addForm" .}}
```

Features:
- Dynamic field rendering based on Fields array
- Field type detection (text, number, textarea, checkbox)
- Form validation with required attribute
- Error display per field
- General error message support
- Kit-aware styling via helper functions
- Submit button with loading state ("Adding...")
- Cancel button with modal close

### `editForm`
Form for editing existing resources:
```go
{{template "editForm" .}}
```

Features:
- Pre-populated fields with existing data
- Hidden ID field for resource identification
- Update action via lvt-submit="update"
- Delete button with confirmation
- Cancel link to detail view
- Field-level error handling
- Kit-aware styling

## Kit Integration

The form component uses kit helper functions for styling:
- `subtitleClass` - Heading style
- `fieldClass` - Form field wrapper style
- `labelClass` - Label style
- `inputClass` - Input/textarea style
- `checkboxClass` - Checkbox wrapper style
- `buttonClass` - Button style with variants (primary, secondary, danger)

## Field Types

The component automatically renders appropriate inputs based on GoType:

- **string**: `<input type="text">`
- **int64**: `<input type="number">`
- **float64**: `<input type="number" step="0.01">`
- **bool**: `<input type="checkbox">`
- **IsTextarea=true**: `<textarea>` with rows="5"

## Error Handling

Built-in error display support via LiveTemplate error API:
- General errors: `.lvt.HasError "_general"` and `.lvt.Error "_general"`
- Field errors: `.lvt.HasError "fieldName"` and `.lvt.Error "fieldName"`
- Visual indicators: `aria-invalid="true"` attribute
- Error messages: Displayed below each field in red

## Examples

### Basic Add Form
```go
{{define "content"}}
  <h1>Products</h1>
  {{template "addModal" .}}
  <button lvt-modal-open="add-modal">Add Product</button>
{{end}}
```

### Edit Form
```go
{{define "content"}}
  {{if .EditingID}}
    {{template "editForm" .}}
  {{else}}
    <p>Select a product to edit</p>
  {{end}}
{{end}}
```

### Custom Fields
```go
fields := []Field{
  {Name: "title", GoType: "string", IsTextarea: false},
  {Name: "description", GoType: "string", IsTextarea: true},
  {Name: "price", GoType: "float64", IsTextarea: false},
  {Name: "quantity", GoType: "int64", IsTextarea: false},
  {Name: "enabled", GoType: "bool", IsTextarea: false},
}
```

## Modal Integration

The addModal template includes modal attributes for LiveTemplate modal system:
- `data-modal-backdrop` - Marks as modal backdrop
- `data-modal-id="add-modal"` - Modal identifier
- `lvt-modal-close="add-modal"` - Close button action
- `lvt-modal-open="add-modal"` - Open button action (used in parent template)

## Form Actions

The forms use LiveTemplate form attributes:
- `lvt-submit="add"` - Triggers add action on submit
- `lvt-submit="update"` - Triggers update action on submit
- `lvt-click="delete"` - Triggers delete action on click
- `lvt-data-id="{{.EditingID}}"` - Passes resource ID to action
- `lvt-confirm="message"` - Shows confirmation dialog
- `lvt-disable-with="text"` - Shows loading state during submission

## Styling

The component uses inline styles for critical layout (modal positioning, backdrop) and kit helper functions for theme-specific styling. This ensures:
- Consistent modal behavior across all kits
- Theme-specific form styling
- Responsive design
- Accessibility features

## Notes

- The template uses `[[` `]]` delimiters for generation-time substitution
- Field values in editForm use camelCase accessor: `.Editing{{.ResourceName}}.{{.Name | camelCase}}`
- All form inputs include `required` attribute for basic validation
- Edit form includes both update and delete actions
- Modal uses `hidden` attribute for initial hidden state
