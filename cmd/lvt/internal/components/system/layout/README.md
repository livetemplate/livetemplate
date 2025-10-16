# Layout Component

Base HTML5 layout component for LiveTemplate applications with full kit support.

## Description

Provides the foundation HTML document structure including:
- Proper HTML5 doctype and head section
- Meta tags for charset and viewport
- Dynamic title and CSS loading via kit
- Semantic container/wrapper based on kit requirements
- LiveTemplate client script injection
- Page routing support for multi-page apps

## Usage

```go
{{template "layout" .}}
```

## Inputs

- **Title** (string, optional): Page title, defaults to "LiveTemplate App"
- **CSSFramework** (string, optional): CSS framework/kit name, defaults to "tailwind"
- **EditMode** (string, optional): Edit mode ("modal" or "page"), defaults to "modal"

## Blocks

### `head`
Customize the head section including title and CSS:
```go
{{define "head"}}
  <title>My Custom Title</title>
  [[csscdn .CSSFramework]]
  <link rel="stylesheet" href="/custom.css">
{{end}}
```

### `content`
Main page content:
```go
{{define "content"}}
  <h1>Hello World</h1>
  <p>Your content here</p>
{{end}}
```

### `scripts`
Additional scripts:
```go
{{define "scripts"}}
  {{/* LiveTemplate client automatically included */}}
  <script src="/app.js"></script>
{{end}}
```

## Kit Integration

The layout component automatically adapts to different kits:
- Uses `csscdn` to load kit CSS
- Uses `containerClass` for wrapper styling
- Checks `needsWrapper` to determine if semantic <main> is needed

## Features

- Responsive viewport meta tag
- Automatic LiveTemplate client loading (dev vs production)
- Page routing support for delete/update actions
- Kit-aware container wrapping
- Extensible block system

## Examples

### Basic Page
```go
{{define "content"}}
  <h1>Welcome</h1>
{{end}}
{{template "layout" .}}
```

### Custom Head
```go
{{define "head"}}
  <title>Dashboard</title>
  [[csscdn .CSSFramework]]
  <link rel="icon" href="/favicon.ico">
{{end}}

{{define "content"}}
  <h1>Dashboard</h1>
{{end}}

{{template "layout" .}}
```

## Notes

- The layout uses `[[` `]]` delimiters for generation-time substitution
- LiveTemplate client script is automatically injected based on DevMode
- Page routing JavaScript is included when EditMode is "page"
