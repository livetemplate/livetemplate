# Pico CSS Kit

Minimal semantic CSS framework kit for LiveTemplate applications using Pico CSS.

## Overview

The Pico kit provides semantic, classless CSS styling that works with native HTML elements. This kit emphasizes simplicity, accessibility, and minimal markup with beautiful default styling.

## Features

- Minimal and semantic HTML
- Classless by default (native element styling)
- Native form styling
- Dark mode built-in (automatic with system preference)
- Lightweight (~10KB gzipped)
- Accessibility focused
- No build step required

## CSS CDN

```
https://cdn.jsdelivr.net/npm/@picocss/pico@1.5.0/css/pico.min.css
```

## Characteristics

- **needs_wrapper**: true (uses semantic `<main>` wrapper)
- **needs_article**: true (uses `<article>` for content blocks)
- **needs_table_wrapper**: false (tables are responsive by default)

## Philosophy

Pico CSS follows a "write semantic HTML, get beautiful styling" approach. Most elements don't need classes - they're styled automatically.

## Container & Layout

### `containerClass()`
Returns: `""`

Pico uses semantic `<main>` element with `container` class for centering.

### `boxClass()`
Returns: `""`

Pico uses semantic `<article>` elements instead of div boxes.

### `needsWrapper()`
Returns: `true`

Pico requires a `<main>` wrapper element.

### `needsArticle()`
Returns: `true`

Pico uses `<article>` elements for content blocks instead of divs.

## Typography

### `titleClass()`
Returns: `""`

Use native `<h1>`, `<h2>` elements - no classes needed.

### `subtitleClass()`
Returns: `""`

Use native heading elements - no classes needed.

## Buttons

### `buttonClass(variant)`
Variants:
- **primary**: `""` (default button styling)
- **secondary**: `"secondary"`
- **danger**: `"contrast"`
- **default**: `""`

Pico styles native `<button>` elements. Use role attributes for variants.

## Forms

Pico excels at form styling with zero classes needed.

### `fieldClass()`
Returns: `""`

No field wrapper class needed.

### `labelClass()`
Returns: `""`

Use native `<label>` - no classes needed.

### `inputClass()`
Returns: `""`

Use native `<input>` - no classes needed.

### `selectClass()`
Returns: `""`

Use native `<select>` - no classes needed.

### `selectWrapperClass()`
Returns: `""`

No wrapper needed.

### `textareaClass()`
Returns: `""`

Use native `<textarea>` - no classes needed.

### `checkboxClass()`
Returns: `""`

No wrapper class needed.

### `checkboxInputClass()`
Returns: `""`

Use native `<input type="checkbox">` - no classes needed.

### `checkboxLabelClass()`
Returns: `""`

Use native `<label>` - no classes needed.

## Tables

### `tableClass()`
Returns: `""`

Use native `<table>` - no classes needed. Pico provides beautiful default styling.

### `needsTableWrapper()`
Returns: `false`

Tables are responsive by default.

### `tableWrapperClass()`
Returns: `""`

Not needed.

## Pagination

### `paginationClass()`
Returns: `""`

Use semantic `<nav>` with role="navigation".

### `paginationButtonClass()`
Returns: `""`

Use native buttons or links.

### `paginationInfoClass()`
Returns: `""`

No special class needed.

### `paginationCurrentClass()`
Returns: `""`

No special class needed.

### `paginationActiveClass()`
Returns: `""`

Use aria-current="page" attribute instead.

## Loading & Error States

### `loadingClass()`
Returns: `""`

Use semantic HTML with aria-busy attribute.

### `errorClass()`
Returns: `""`

Use native form validation styling.

## Display Field

### `displayField(fields)`
Returns the first field from the fields array.

## CSS CDN Helper

### `csscdn(framework)`
Returns: CDN URL for Pico CSS.

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@1.5.0/css/pico.min.css">
```

## Usage Examples

### Basic Layout
```html
<main class="container">
  <h1>Products</h1>

  <article>
    <p>Content here</p>
  </article>
</main>
```

### Form (Classless!)
```html
<form>
  <label>
    Name
    <input type="text" placeholder="Enter name" required>
  </label>

  <label>
    Description
    <textarea placeholder="Enter description"></textarea>
  </label>

  <label>
    <input type="checkbox" role="switch">
    Enable notifications
  </label>

  <button type="submit">Submit</button>
</form>
```

### Buttons
```html
<button>Primary Button</button>
<button class="secondary">Secondary Button</button>
<button class="contrast">Contrast Button</button>
<button disabled>Disabled Button</button>
```

### Table (Classless!)
```html
<table>
  <thead>
    <tr>
      <th>Name</th>
      <th>Price</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Product A</td>
      <td>$10</td>
    </tr>
  </tbody>
</table>
```

### Grid Layout
```html
<div class="grid">
  <div>Column 1</div>
  <div>Column 2</div>
  <div>Column 3</div>
</div>
```

### Loading State
```html
<article aria-busy="true">
  Loading content...
</article>
```

## Semantic HTML

Pico encourages semantic HTML:

```html
<main class="container">
  <header>
    <h1>Page Title</h1>
    <nav>Navigation</nav>
  </header>

  <section>
    <article>
      <header>
        <h2>Article Title</h2>
      </header>
      <p>Article content</p>
      <footer>
        <small>Article footer</small>
      </footer>
    </article>
  </section>

  <footer>
    <small>Page footer</small>
  </footer>
</main>
```

## Dark Mode

Pico includes automatic dark mode based on system preference:

```html
<!-- Automatic dark mode -->
<html data-theme="auto">

<!-- Force light mode -->
<html data-theme="light">

<!-- Force dark mode -->
<html data-theme="dark">
```

## Form Elements

Pico provides beautiful native form styling:

### Text Input
```html
<label>
  Email
  <input type="email" placeholder="email@example.com">
</label>
```

### Select
```html
<label>
  Category
  <select>
    <option>Option 1</option>
    <option>Option 2</option>
  </select>
</label>
```

### Switch (Checkbox styled as toggle)
```html
<label>
  <input type="checkbox" role="switch">
  Toggle feature
</label>
```

### Radio Buttons
```html
<fieldset>
  <legend>Choose option</legend>
  <label>
    <input type="radio" name="option" value="1">
    Option 1
  </label>
  <label>
    <input type="radio" name="option" value="2">
    Option 2
  </label>
</fieldset>
```

## Responsive Design

Pico is responsive by default:

```html
<!-- Responsive grid (auto-fills) -->
<div class="grid">
  <div>Auto column</div>
  <div>Auto column</div>
  <div>Auto column</div>
</div>

<!-- Responsive container -->
<main class="container">
  <!-- Content automatically centered and padded -->
</main>
```

## Accessibility

Pico is built with accessibility in mind:

- Proper ARIA attributes
- Focus states on all interactive elements
- Keyboard navigation support
- Semantic HTML encouraged
- High contrast ratios
- Screen reader friendly

### Example with ARIA
```html
<button aria-label="Close dialog" aria-busy="true">
  Processing...
</button>

<nav aria-label="Pagination">
  <ul>
    <li><a href="#" aria-current="page">1</a></li>
    <li><a href="#">2</a></li>
  </ul>
</nav>
```

## Color Scheme

Pico uses CSS custom properties:

```css
/* Light mode colors */
--primary: #1095c1;
--secondary: #5f6c7b;
--contrast: #1f2a37;

/* Dark mode colors (automatic) */
/* Pico handles dark mode colors automatically */
```

## Utilities

Pico provides minimal utility classes:

```html
<!-- Container -->
<main class="container"></main>

<!-- Grid -->
<div class="grid"></div>

<!-- Button variants -->
<button class="secondary"></button>
<button class="contrast"></button>
<button class="outline"></button>

<!-- Loading state -->
<div aria-busy="true"></div>
```

## Best Practices

1. **Use semantic HTML**: Let Pico's defaults do the work
2. **Minimal classes**: Only add classes when needed
3. **Native elements**: Prefer native HTML elements
4. **ARIA attributes**: Use aria-* for states and roles
5. **Container usage**: Wrap content in `<main class="container">`
6. **Article blocks**: Use `<article>` for content cards

## Documentation

Full Pico CSS documentation: https://picocss.com/docs

## Version

This kit is based on Pico CSS v1.5.0.

## Notes

- Extremely lightweight (~10KB)
- No JavaScript required
- No build tools needed
- Perfect for minimalist designs
- Great for prototyping
- Excellent accessibility
- Beautiful default styling
- Works great with server-side rendering
