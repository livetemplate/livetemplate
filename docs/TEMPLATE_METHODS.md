# Template Parsing Methods

LiveTemplate provides simplified template parsing methods that automatically register templates for immediate use.

## Auto-Registering Parse Methods

These methods parse templates AND automatically register them with sensible default names:

### ParseFiles
```go
func (a *Application) ParseFiles(filenames ...string) (*template.Template, error)
```
Parses template definitions from files and automatically registers the template using the first file's base name (without extension).

**Example:**
```go
// Parses and registers as "index"
tmpl, err := app.ParseFiles("templates/index.html")
page, _ := app.NewPage("index", data) // Use the auto-registered name
```

### MustParseFiles  
```go
func (a *Application) MustParseFiles(filenames ...string) *template.Template
```
Like `ParseFiles` but panics if parsing fails. Follows the `template.Must()` pattern.

### ParseGlob
```go
func (a *Application) ParseGlob(pattern string) (*template.Template, error)
```
Parses template definitions from files matching pattern and automatically registers using a name derived from the pattern (directory name if pattern contains path, otherwise "templates").

**Example:**
```go
// Parses and registers as "components" 
tmpl, err := app.ParseGlob("components/*.html")
page, _ := app.NewPage("components", data)
```

### MustParseGlob
```go
func (a *Application) MustParseGlob(pattern string) *template.Template
```
Like `ParseGlob` but panics if parsing fails.

## Manual Registration (when you need custom names)

For cases where you need specific template names:

### RegisterTemplate
```go
func (a *Application) RegisterTemplate(name string, tmpl *template.Template) error
```
Manually register a pre-parsed template with a custom name.

### RegisterTemplateFromFile
```go
func (a *Application) RegisterTemplateFromFile(name string, filepath string) error
```
Parse a single file and register with a custom name.

## Usage Examples

```go
app, _ := livetemplate.NewApplication()

// Simple case - auto-registration
_, err := app.ParseFiles("templates/index.html")
page, _ := app.NewPage("index", data) // Uses filename without extension

// Multiple files - still uses first file's name
_, err = app.ParseFiles("layout.html", "content.html", "footer.html") 
page, _ = app.NewPage("layout", data) // Uses "layout"

// Pattern-based parsing
_, err = app.ParseGlob("components/*.html")
page, _ = app.NewPage("components", data) // Uses directory name

// Custom name when needed
err = app.RegisterTemplateFromFile("mypage", "templates/special.html")
page, _ = app.NewPage("mypage", data)

// Manual registration
tmpl := template.Must(template.New("custom").Parse("{{.Name}}"))
app.RegisterTemplate("greeting", tmpl)
page, _ = app.NewPage("greeting", data)
```

## Key Benefits

1. **One-step parsing**: No separate registration step needed
2. **Sensible defaults**: Template names derived from filenames/paths  
3. **Standard patterns**: Follows familiar `html/template` method signatures
4. **Flexibility**: Manual registration still available when needed

## Auto-Registration Rules

- `ParseFiles("path/to/file.html")` → registers as `"file"`
- `ParseGlob("components/*.html")` → registers as `"components"` 
- `ParseGlob("*.html")` → registers as `"templates"`