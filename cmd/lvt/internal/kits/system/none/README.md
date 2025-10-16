# None Kit (Vanilla HTML)

Plain HTML kit for LiveTemplate applications with no CSS framework dependencies.

## Overview

The "none" kit provides semantic HTML without any CSS framework. This kit is perfect for projects that want complete control over styling, use custom CSS, or prefer browser defaults.

## Features

- Pure semantic HTML
- Zero CSS dependencies
- Maximum control over styling
- Lightweight (no external CSS)
- Custom CSS ready
- Browser default styling
- Progressive enhancement friendly

## CSS CDN

```
(none - no external CSS loaded)
```

## Characteristics

- **needs_wrapper**: false (no wrapper required)
- **needs_article**: false (uses div elements)
- **needs_table_wrapper**: false (no wrapper needed)

## Philosophy

This kit embraces the philosophy of "HTML first, style later". It provides clean, semantic HTML that can be styled with custom CSS, inline styles, or left with browser defaults.

## Container & Layout

### `containerClass()`
Returns: `""`

No container class. Use your own custom CSS or inline styles.

### `boxClass()`
Returns: `""`

No box class. Style with custom CSS as needed.

### `needsWrapper()`
Returns: `false`

No semantic wrapper required.

### `needsArticle()`
Returns: `false`

Uses standard `<div>` elements.

## Typography

### `titleClass()`
Returns: `""`

Use native `<h1>`, `<h2>` elements with browser default styling.

### `subtitleClass()`
Returns: `""`

Use native heading elements with browser default styling.

## Buttons

### `buttonClass(variant)`
Returns: `""` for all variants

Use native `<button>` elements. Style with custom CSS or inline styles.

Variants:
- **primary**: `""`
- **secondary**: `""`
- **danger**: `""`
- **default**: `""`

## Forms

All form helpers return empty strings. Use native HTML form elements.

### `fieldClass()`
Returns: `""`

No field wrapper class.

### `labelClass()`
Returns: `""`

Use native `<label>` element.

### `inputClass()`
Returns: `""`

Use native `<input>` element.

### `selectClass()`
Returns: `""`

Use native `<select>` element.

### `selectWrapperClass()`
Returns: `""`

No wrapper needed.

### `textareaClass()`
Returns: `""`

Use native `<textarea>` element.

### `checkboxClass()`
Returns: `""`

No wrapper class.

### `checkboxInputClass()`
Returns: `""`

Use native `<input type="checkbox">`.

### `checkboxLabelClass()`
Returns: `""`

Use native `<label>` element.

## Tables

### `tableClass()`
Returns: `""`

Use native `<table>` element with browser default styling.

### `needsTableWrapper()`
Returns: `false`

No wrapper needed.

### `tableWrapperClass()`
Returns: `""`

Not used.

## Pagination

### `paginationClass()`
Returns: `""`

Use semantic `<nav>` element.

### `paginationButtonClass()`
Returns: `""`

Use native buttons or links.

### `paginationInfoClass()`
Returns: `""`

No special class.

### `paginationCurrentClass()`
Returns: `""`

No special class.

### `paginationActiveClass()`
Returns: `""`

No special class. Consider using aria-current attribute.

## Loading & Error States

### `loadingClass()`
Returns: `""`

Use custom CSS or inline styles.

### `errorClass()`
Returns: `""`

Use custom CSS or inline styles.

## Display Field

### `displayField(fields)`
Returns the first field from the fields array.

## CSS CDN Helper

### `csscdn(framework)`
Returns: `""` (no CSS CDN)

No external CSS is loaded.

## Usage Examples

### Basic Layout
```html
<div>
  <h1>Products</h1>

  <div>
    <p>Content here</p>
  </div>
</div>
```

### Form
```html
<form>
  <div>
    <label for="name">Name</label>
    <input type="text" id="name" name="name">
  </div>

  <div>
    <label for="description">Description</label>
    <textarea id="description" name="description"></textarea>
  </div>

  <div>
    <label>
      <input type="checkbox" name="enabled">
      Enable
    </label>
  </div>

  <button type="submit">Submit</button>
</form>
```

### Table
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

### Buttons
```html
<button>Default Button</button>
<button type="submit">Submit Button</button>
<button disabled>Disabled Button</button>
```

### Pagination
```html
<nav>
  <a href="#">Previous</a>
  <span aria-current="page">1</span>
  <a href="#">2</a>
  <a href="#">3</a>
  <a href="#">Next</a>
</nav>
```

## Styling Options

With the none kit, you have several options for styling:

### 1. Custom CSS File
```html
<head>
  <link rel="stylesheet" href="/custom.css">
</head>
```

### 2. Inline Styles
```html
<div style="max-width: 800px; margin: 0 auto; padding: 2rem;">
  <h1 style="color: #333; font-size: 2rem;">Title</h1>
</div>
```

### 3. Custom Classes
```html
<!-- Define your own classes -->
<style>
.container { max-width: 1200px; margin: 0 auto; }
.btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; }
.btn-primary { background: #007bff; color: white; }
</style>

<div class="container">
  <button class="btn btn-primary">Submit</button>
</div>
```

### 4. CSS-in-JS
```html
<script>
// Use your preferred CSS-in-JS solution
</script>
```

### 5. Browser Defaults
```html
<!-- Clean HTML, styled by browser -->
<form>
  <label>Email <input type="email"></label>
  <button>Submit</button>
</form>
```

## Semantic HTML Best Practices

Without framework constraints, focus on semantic HTML:

```html
<main>
  <header>
    <h1>Page Title</h1>
    <nav>
      <ul>
        <li><a href="/">Home</a></li>
        <li><a href="/about">About</a></li>
      </ul>
    </nav>
  </header>

  <article>
    <header>
      <h2>Article Title</h2>
      <time datetime="2025-10-16">October 16, 2025</time>
    </header>

    <section>
      <p>Article content</p>
    </section>

    <footer>
      <p>Author information</p>
    </footer>
  </article>

  <aside>
    <h3>Related Content</h3>
  </aside>

  <footer>
    <p>&copy; 2025 Your Company</p>
  </footer>
</main>
```

## Accessibility

Without framework classes, use ARIA attributes and semantic HTML:

```html
<!-- Buttons with ARIA -->
<button aria-label="Close dialog">×</button>
<button aria-pressed="true">Toggle</button>

<!-- Forms with proper associations -->
<label for="email">Email</label>
<input type="email" id="email" aria-describedby="email-help">
<small id="email-help">We'll never share your email</small>

<!-- Navigation with ARIA -->
<nav aria-label="Main navigation">
  <ul>
    <li><a href="/" aria-current="page">Home</a></li>
  </ul>
</nav>

<!-- Loading states -->
<div aria-busy="true" aria-live="polite">
  Loading content...
</div>
```

## Progressive Enhancement

The none kit is perfect for progressive enhancement:

```html
<!-- Basic HTML that works without CSS/JS -->
<form method="POST" action="/submit">
  <label>Name <input type="text" name="name" required></label>
  <button type="submit">Submit</button>
</form>

<!-- Enhanced with CSS -->
<style>
form { /* your styles */ }
</style>

<!-- Enhanced with JavaScript -->
<script>
// Add interactive features
</script>
```

## Use Cases

The none kit is ideal for:

1. **Custom Design Systems**: Build your own styling from scratch
2. **Minimal Applications**: Keep it simple with browser defaults
3. **Progressive Enhancement**: Start with HTML, add styling later
4. **Learning Projects**: Understand HTML structure without framework overhead
5. **Performance Critical**: Eliminate CSS framework overhead
6. **Legacy Integration**: Match existing custom styles
7. **Print Stylesheets**: Control print output completely
8. **Email Templates**: HTML without CSS frameworks

## Browser Support

All browsers support native HTML elements:
- Modern browsers: Full support
- Older browsers: Graceful degradation
- Screen readers: Excellent (semantic HTML)
- Mobile browsers: Full support

## Migration Path

Starting with none kit allows easy migration to other kits later:

```go
// Start with none
data.CSSFramework = "none"

// Switch to Tailwind later
data.CSSFramework = "tailwind"

// Or Bulma
data.CSSFramework = "bulma"

// Or Pico
data.CSSFramework = "pico"
```

## Examples with Inline Styles

Since there are no framework classes, inline styles are common:

```html
<!-- Container -->
<div style="max-width: 1200px; margin: 0 auto; padding: 1rem;">
  <h1>Products</h1>
</div>

<!-- Card -->
<div style="border: 1px solid #ddd; border-radius: 4px; padding: 1rem; margin-bottom: 1rem;">
  <h2>Product Name</h2>
  <p>Description</p>
</div>

<!-- Button -->
<button style="background: #007bff; color: white; border: none; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer;">
  Submit
</button>

<!-- Form input -->
<input type="text" style="width: 100%; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px;">
```

## Best Practices

1. **Use semantic HTML**: Proper element choice for accessibility
2. **ID and class attributes**: Add for custom styling hooks
3. **ARIA attributes**: Enhance accessibility where needed
4. **Progressive enhancement**: HTML first, styling second
5. **Consistent structure**: Maintain consistent HTML patterns
6. **Comments**: Document structure in complex layouts
7. **Validation**: Use HTML5 validation attributes

## Documentation

No framework documentation needed. Refer to:
- MDN Web Docs: https://developer.mozilla.org/
- HTML Living Standard: https://html.spec.whatwg.org/
- ARIA Authoring Practices: https://www.w3.org/WAI/ARIA/apg/

## Version

This kit is version 1.0.0 (semantic HTML standard).

## Notes

- No external dependencies
- No CSS file loading
- Maximum flexibility
- Perfect for custom designs
- Ideal for learning HTML
- Great for prototyping
- Minimal performance overhead
- Complete control over markup and styling
