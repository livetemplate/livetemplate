# Bulma CSS Kit

Modern component-based CSS framework kit for LiveTemplate applications using Bulma.

## Overview

The Bulma kit provides a complete set of CSS helper functions for building clean, modern interfaces using Bulma's component-based approach. This kit emphasizes semantic HTML and readable class names.

## Features

- Component-based CSS classes
- Flexbox-based grid system
- Modular and lightweight
- Responsive by default
- Rich component library
- Easy to customize with SASS
- No JavaScript dependencies

## CSS CDN

```
https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css
```

## Characteristics

- **needs_wrapper**: false (no semantic wrapper needed)
- **needs_article**: false (uses div for containers)
- **needs_table_wrapper**: false (no scrollable wrapper)

## Container & Layout

### `containerClass()`
Returns: `"container"`

Bulma's centered container that adapts to viewport width.

### `boxClass()`
Returns: `"box"`

Bulma box component with default styling (white background, shadow, padding).

### `needsWrapper()`
Returns: `false`

Bulma doesn't require wrapper elements.

### `needsArticle()`
Returns: `false`

Uses div elements instead of semantic article tags.

## Typography

### `titleClass()`
Returns: `"title is-3"`

Large title using Bulma's size modifier.

### `subtitleClass()`
Returns: `"subtitle is-4"`

Subtitle with medium size.

## Buttons

### `buttonClass(variant)`
Variants:
- **primary**: `"button is-primary"`
- **secondary**: `"button"`
- **danger**: `"button is-danger"`
- **default**: `"button"`

Bulma button classes with color modifiers.

## Forms

### `fieldClass()`
Returns: `"field"`

Bulma field component for form element grouping.

### `labelClass()`
Returns: `"label"`

Form label with Bulma styling.

### `inputClass()`
Returns: `"input"`

Text input with Bulma styling.

### `selectClass()`
Returns: `"select"`

Select element base class.

### `selectWrapperClass()`
Returns: `"select"`

Bulma requires a wrapper div with `select` class.

### `textareaClass()`
Returns: `"textarea"`

Textarea with Bulma styling.

### `checkboxClass()`
Returns: `"field"`

Checkbox field wrapper.

### `checkboxInputClass()`
Returns: `""`

No special class for checkbox input.

### `checkboxLabelClass()`
Returns: `"checkbox"`

Checkbox label with Bulma's checkbox class.

## Tables

### `tableClass()`
Returns: `"table is-fullwidth is-hoverable"`

Full-width table with hover effects.

### `needsTableWrapper()`
Returns: `false`

No scrollable wrapper needed.

### `tableWrapperClass()`
Returns: `""`

Not used (no wrapper needed).

## Pagination

### `paginationClass()`
Returns: `"pagination is-centered"`

Bulma pagination component with centered alignment.

### `paginationButtonClass()`
Returns: `"pagination-link"`

Pagination link/button styling.

### `paginationInfoClass()`
Returns: `""`

No special styling for page info.

### `paginationCurrentClass()`
Returns: `""`

No special styling for current page indicator.

### `paginationActiveClass()`
Returns: `"pagination-link is-current"`

Active page link with `is-current` modifier.

## Loading & Error States

### `loadingClass()`
Returns: `"has-text-grey"`

Loading indicator using Bulma's text color helper.

### `errorClass()`
Returns: `"help is-danger"`

Error message using Bulma's help text with danger color.

## Display Field

### `displayField(fields)`
Returns the first field from the fields array.

Used to determine which field to display in tables.

## CSS CDN Helper

### `csscdn(framework)`
Returns: CDN URL for Bulma CSS.

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@0.9.4/css/bulma.min.css">
```

## Usage Examples

### Basic Layout
```html
<div class="container">
  <h1 class="title is-3">Products</h1>

  <div class="box">
    <p>Content here</p>
  </div>
</div>
```

### Form
```html
<form>
  <div class="field">
    <label class="label">Name</label>
    <div class="control">
      <input class="input" type="text" placeholder="Enter name">
    </div>
  </div>

  <div class="field">
    <div class="control">
      <button class="button is-primary">Submit</button>
    </div>
  </div>
</form>
```

### Form with Select
```html
<div class="field">
  <label class="label">Category</label>
  <div class="control">
    <div class="select">
      <select>
        <option>Option 1</option>
        <option>Option 2</option>
      </select>
    </div>
  </div>
</div>
```

### Table
```html
<table class="table is-fullwidth is-hoverable">
  <tbody>
    <tr>
      <td>Data</td>
    </tr>
  </tbody>
</table>
```

### Pagination
```html
<nav class="pagination is-centered" role="navigation">
  <a class="pagination-link">Prev</a>
  <a class="pagination-link is-current">1</a>
  <a class="pagination-link">2</a>
  <a class="pagination-link">Next</a>
</nav>
```

### Buttons
```html
<button class="button is-primary">Primary</button>
<button class="button">Default</button>
<button class="button is-danger">Danger</button>
```

## Bulma Modifiers

Bulma uses modifier classes for variations:

### Size Modifiers
```html
<button class="button is-small">Small</button>
<button class="button">Normal</button>
<button class="button is-medium">Medium</button>
<button class="button is-large">Large</button>
```

### Color Modifiers
```html
<button class="button is-primary">Primary</button>
<button class="button is-link">Link</button>
<button class="button is-info">Info</button>
<button class="button is-success">Success</button>
<button class="button is-warning">Warning</button>
<button class="button is-danger">Danger</button>
```

### State Modifiers
```html
<button class="button is-loading">Loading</button>
<button class="button" disabled>Disabled</button>
<button class="button is-outlined">Outlined</button>
```

## Responsive Design

Bulma includes responsive helpers:

```html
<!-- Responsive columns -->
<div class="columns">
  <div class="column is-half-tablet is-one-third-desktop">
    Column content
  </div>
</div>

<!-- Responsive visibility -->
<div class="is-hidden-mobile">Hidden on mobile</div>
<div class="is-hidden-tablet">Hidden on tablet and up</div>
```

## Grid System

Bulma uses a flexible 12-column grid:

```html
<div class="columns">
  <div class="column is-4">
    <!-- 4/12 width -->
  </div>
  <div class="column is-8">
    <!-- 8/12 width -->
  </div>
</div>

<!-- Auto-width columns -->
<div class="columns">
  <div class="column">Auto</div>
  <div class="column">Auto</div>
  <div class="column">Auto</div>
</div>
```

## Color Palette

Bulma's default color scheme:
- **Primary**: Turquoise (#00d1b2)
- **Link**: Blue (#485fc7)
- **Info**: Cyan (#3e8ed0)
- **Success**: Green (#48c78e)
- **Warning**: Yellow (#ffe08a)
- **Danger**: Red (#f14668)

## Form Controls

Bulma provides comprehensive form control styling:

```html
<!-- Input with icon -->
<div class="field">
  <div class="control has-icons-left">
    <input class="input" type="email" placeholder="Email">
    <span class="icon is-small is-left">
      <i class="fas fa-envelope"></i>
    </span>
  </div>
</div>

<!-- Horizontal form -->
<div class="field is-horizontal">
  <div class="field-label is-normal">
    <label class="label">Name</label>
  </div>
  <div class="field-body">
    <div class="field">
      <input class="input" type="text">
    </div>
  </div>
</div>
```

## Best Practices

1. **Use Bulma components**: Leverage Bulma's pre-built components
2. **Semantic modifiers**: Use is-* classes for state and style variations
3. **Responsive columns**: Use column system for layouts
4. **Form structure**: Always wrap inputs in control divs within field divs
5. **Consistent spacing**: Use Bulma's spacing helpers (m-*, p-*)

## Documentation

Full Bulma documentation: https://bulma.io/documentation/

## Version

This kit is based on Bulma v0.9.4.

## Notes

- Pure CSS framework (no JavaScript)
- Based on Flexbox
- Mobile-first responsive design
- SASS source available for customization
- Icon-friendly (works well with Font Awesome)
- Extensive component library
