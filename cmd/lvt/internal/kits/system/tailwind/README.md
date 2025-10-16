# Tailwind CSS Kit

Utility-first CSS framework kit for LiveTemplate applications using Tailwind CSS.

## Overview

The Tailwind kit provides a complete set of CSS helper functions for building modern, responsive interfaces using Tailwind's utility-first approach. This kit emphasizes flexibility and customization through utility classes.

## Features

- Utility-first CSS classes
- Highly customizable styling
- Responsive design built-in
- Modern component styling
- Flexbox and Grid utilities
- No wrapper elements needed
- Clean, minimal markup

## CSS CDN

```
https://cdn.jsdelivr.net/npm/tailwindcss@3.4.0/dist/tailwind.min.css
```

## Characteristics

- **needs_wrapper**: false (no semantic wrapper needed)
- **needs_article**: false (uses div for containers)
- **needs_table_wrapper**: false (no scrollable wrapper)

## Container & Layout

### `containerClass()`
Returns: `"container mx-auto px-4"`

Provides a centered container with automatic margins and padding.

### `boxClass()`
Returns: `"bg-white rounded-lg shadow-md p-6 mb-6"`

Card-style box with white background, rounded corners, shadow, and spacing.

### `needsWrapper()`
Returns: `false`

Tailwind doesn't require wrapper elements.

### `needsArticle()`
Returns: `false`

Uses div elements instead of semantic article tags.

## Typography

### `titleClass()`
Returns: `"text-3xl font-bold mb-6"`

Large, bold title with bottom margin.

### `subtitleClass()`
Returns: `"text-2xl font-semibold mb-4"`

Medium-sized, semi-bold subtitle with bottom margin.

## Buttons

### `buttonClass(variant)`
Variants:
- **primary**: `"bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"`
- **secondary**: `"bg-gray-500 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded"`
- **danger**: `"bg-red-500 hover:bg-red-700 text-white font-bold py-2 px-4 rounded"`
- **default**: same as primary

Provides colored buttons with hover states and proper padding.

## Forms

### `fieldClass()`
Returns: `"mb-4"`

Form field wrapper with bottom margin.

### `labelClass()`
Returns: `"block text-gray-700 text-sm font-bold mb-2"`

Form label with proper typography and spacing.

### `inputClass()`
Returns: `"shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"`

Text input with shadow, border, focus states, and proper sizing.

### `selectClass()`
Returns: `"shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"`

Select dropdown with consistent styling to inputs.

### `selectWrapperClass()`
Returns: `""`

No wrapper needed for Tailwind selects.

### `textareaClass()`
Returns: `"shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline"`

Textarea with consistent styling to inputs.

### `checkboxClass()`
Returns: `"flex items-center mb-4"`

Checkbox wrapper using flexbox for alignment.

### `checkboxInputClass()`
Returns: `"mr-2"`

Checkbox input with right margin.

### `checkboxLabelClass()`
Returns: `"text-gray-700"`

Checkbox label with proper color.

## Tables

### `tableClass()`
Returns: `"min-w-full divide-y divide-gray-200"`

Full-width table with row dividers.

### `needsTableWrapper()`
Returns: `false`

No scrollable wrapper needed.

### `tableWrapperClass()`
Returns: `""`

Not used (no wrapper needed).

## Pagination

### `paginationClass()`
Returns: `"flex items-center justify-center space-x-2"`

Flexbox container for pagination controls with centered alignment and spacing.

### `paginationButtonClass()`
Returns: `"px-3 py-1 border rounded hover:bg-gray-100"`

Pagination button with border, hover state, and padding.

### `paginationInfoClass()`
Returns: `""`

No special styling for page info.

### `paginationCurrentClass()`
Returns: `""`

No special styling for current page indicator.

### `paginationActiveClass()`
Returns: `"px-3 py-1 bg-blue-500 text-white rounded"`

Active page number with blue background and white text.

## Loading & Error States

### `loadingClass()`
Returns: `"text-gray-600"`

Loading indicator text color.

### `errorClass()`
Returns: `"text-red-500 text-sm mt-1"`

Error message styling with red color and small text.

## Display Field

### `displayField(fields)`
Returns the first field from the fields array.

Used to determine which field to display in tables.

## CSS CDN Helper

### `csscdn(framework)`
Returns: CDN URL for Tailwind CSS.

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@3.4.0/dist/tailwind.min.css">
```

## Usage Examples

### Basic Layout
```html
<div class="container mx-auto px-4">
  <h1 class="text-3xl font-bold mb-6">Products</h1>

  <div class="bg-white rounded-lg shadow-md p-6 mb-6">
    <p>Content here</p>
  </div>
</div>
```

### Form
```html
<form>
  <div class="mb-4">
    <label class="block text-gray-700 text-sm font-bold mb-2">Name</label>
    <input class="shadow appearance-none border rounded w-full py-2 px-3 text-gray-700 leading-tight focus:outline-none focus:shadow-outline" type="text">
  </div>

  <button class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded">
    Submit
  </button>
</form>
```

### Table
```html
<table class="min-w-full divide-y divide-gray-200">
  <tbody>
    <tr>
      <td class="px-6 py-4">Data</td>
    </tr>
  </tbody>
</table>
```

### Pagination
```html
<nav class="flex items-center justify-center space-x-2">
  <button class="px-3 py-1 border rounded hover:bg-gray-100">Prev</button>
  <span class="px-3 py-1 bg-blue-500 text-white rounded">1</span>
  <button class="px-3 py-1 border rounded hover:bg-gray-100">Next</button>
</nav>
```

## Customization

Tailwind is highly customizable through utility classes. The kit provides sensible defaults that can be overridden:

```html
<!-- Override button color -->
<button class="bg-green-500 hover:bg-green-700 text-white font-bold py-2 px-4 rounded">
  Custom Button
</button>

<!-- Add additional utilities -->
<div class="container mx-auto px-4 max-w-4xl">
  <!-- Content with max width -->
</div>
```

## Responsive Design

Tailwind includes responsive utilities out of the box:

```html
<!-- Responsive padding -->
<div class="px-4 md:px-8 lg:px-12">
  Content
</div>

<!-- Responsive grid -->
<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
  <!-- Items -->
</div>
```

## Color Palette

The kit uses Tailwind's default color palette:
- **Primary**: Blue (blue-500, blue-700)
- **Secondary**: Gray (gray-500, gray-700)
- **Danger**: Red (red-500, red-700)
- **Text**: Gray shades (gray-700, gray-600)
- **Borders**: Gray (gray-200)

## Best Practices

1. **Use utility classes**: Leverage Tailwind's utilities instead of custom CSS
2. **Responsive first**: Use responsive modifiers (sm:, md:, lg:, xl:)
3. **Consistent spacing**: Use Tailwind's spacing scale (p-4, mb-6, etc.)
4. **Focus states**: Always include focus states for accessibility
5. **Hover effects**: Add hover states for interactive elements

## Documentation

Full Tailwind CSS documentation: https://tailwindcss.com/docs

## Version

This kit is based on Tailwind CSS v3.4.0.

## Notes

- No JavaScript required for basic functionality
- Works with Tailwind's JIT (Just-In-Time) compiler
- Compatible with Tailwind plugins
- Can be extended with custom configuration
- All classes are responsive by default
