# Patterns Example

**Status:** Proposed
**Date:** 2026-04-11
**Issue:** [#330](https://github.com/livetemplate/livetemplate/issues/330)

## Summary

Create a single multi-page LiveTemplate example app in `./examples/patterns/` that implements 31 UI patterns across 7 categories. The patterns are sourced from [htmx.org/examples](https://htmx.org/examples/) (22 patterns) and [Phoenix LiveView](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html) (9 additional patterns unique to server-rendered reactive frameworks).

Each pattern is a focused, isolated demonstration — one page, one handler, one template. A main index page categorizes all patterns with short descriptions. The app uses in-memory data (no database) to keep the focus on interaction patterns rather than persistence.

All 31 patterns are implemented fresh in this app, even where existing examples cover similar ground. Cross-references to existing examples are provided for context.

## Design Principles

From [FIRST_PRINCIPLES.md](../design/FIRST_PRINCIPLES.md):

> Start with standard HTML. Add `lvt-*` only when HTML can't express it.

Every pattern follows the progressive complexity model:

1. **Tier 1 first** — standard HTML forms, buttons, inputs, `<dialog>`, links
2. **Tier 2 only when needed** — `lvt-*` attributes for debounce, reactive DOM, scroll, animations, keyboard shortcuts, file uploads

The app also demonstrates a key architectural advantage of LiveTemplate over htmx: **the entire template re-renders server-side and only changed dynamics are sent via tree diff**. This means htmx patterns like "Update Other Content" (out-of-band swaps) and explicit target selectors are unnecessary — changing any state field automatically updates all template expressions that reference it.

## Example App Architecture

```
examples/patterns/
  main.go              # Router, all handlers registered
  handlers.go          # Pattern handler functions (organized by category)
  state.go             # State structs for each pattern
  data.go              # In-memory sample data
  templates/
    layout.tmpl        # Shared HTML layout (head, nav, footer)
    index.tmpl         # Main index page with categorized pattern grid
    forms/
      click-to-edit.tmpl
      edit-row.tmpl
      inline-validation.tmpl
      bulk-update.tmpl
      reset-input.tmpl
      file-upload.tmpl
      preserve-inputs.tmpl
    lists/
      delete-row.tmpl
      click-to-load.tmpl
      infinite-scroll.tmpl
      value-select.tmpl
    search/
      active-search.tmpl
      url-filters.tmpl
    loading/
      lazy-loading.tmpl
      progress-bar.tmpl
      async-operations.tmpl
    navigation/
      modal-dialog.tmpl
      confirm-dialog.tmpl
      tabs.tmpl
      spa-navigation.tmpl
      keyboard-shortcuts.tmpl
    feedback/
      animations.tmpl
      loading-states.tmpl
      highlight.tmpl
      flash-messages.tmpl
    realtime/
      multi-user-sync.tmpl
      broadcasting.tmpl
      presence.tmpl
      reconnection.tmpl
      live-preview.tmpl
      server-push.tmpl
  main_test.go         # Chromedp E2E tests for all patterns
```

### Routing

Each pattern is mounted at `/patterns/{category}/{name}`:

```go
// main.go
func main() {
    mux := http.NewServeMux()

    // Index page
    mux.Handle("/", indexHandler())

    // Category: Forms & Editing
    mux.Handle("/patterns/forms/click-to-edit", clickToEditHandler())
    mux.Handle("/patterns/forms/edit-row", editRowHandler())
    // ... etc
}
```

Each handler function returns an `http.Handler` by creating a `livetemplate.Template` and calling `.Handle(controller, state)`.

### Shared Layout

All templates extend a shared layout:

```html
{{define "layout"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="color-scheme" content="light dark">
    <title>{{.Title}} — LiveTemplate Patterns</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
    {{if .lvt.DevMode}}
    <link rel="stylesheet" href="/livetemplate.css">
    {{else}}
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@livetemplate/client@latest/livetemplate.css">
    {{end}}
</head>
<body>
    <main class="container">
        <nav>
            <ul><li><a href="/"><strong>Patterns</strong></a></li></ul>
            <ul><li><small>{{.Category}}</small></li></ul>
        </nav>
        {{template "content" .}}
    </main>
</body>
</html>
{{end}}
```

## Cross-Reference: Existing Examples

These existing examples cover similar patterns. The patterns app reimplements each as a focused, isolated demonstration.

| Example | Relevant Patterns |
|---|---|
| `todos/` | CRUD, search, sort, pagination, modal, toasts, validation, animate, highlight |
| `chat/` | Multi-user sync, presence, real-time, scroll |
| `flash-messages/` | Flash/toast notifications |
| `live-preview/` | Live input binding (Change()) |
| `avatar-upload/` | File upload with progress |
| `shared-notepad/` | Shared state, form preservation |
| `login/` | Auth, server push (TriggerAction) |
| `dialog-patterns/` | Dialog interactions |

---

## Pattern Catalog

### Category 1: Forms & Editing

#### 1. Click To Edit

**htmx:** Uses `hx-get`/`hx-put` with `hx-target="this"` and `hx-swap="outerHTML"` to toggle between view and edit mode by swapping HTML fragments.

**LiveTemplate (Tier 1):** Server state tracks editing mode. Template uses `{{if}}` to render view or edit branch. Button `name` routes to `Edit()`/`Save()`/`Cancel()` methods. The tree diff sends only the changed dynamic — no explicit targeting needed.

**Also implemented in:** —

```go
type ClickToEditState struct {
    Title     string
    FirstName string
    LastName  string
    Email     string
    Editing   bool
}

func (c *Controller) Edit(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
    state.Editing = true
    return state, nil
}

func (c *Controller) Save(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
    state.FirstName = ctx.GetString("firstName")
    state.LastName = ctx.GetString("lastName")
    state.Email = ctx.GetString("email")
    state.Editing = false
    return state, nil
}

func (c *Controller) Cancel(state ClickToEditState, ctx *livetemplate.Context) (ClickToEditState, error) {
    state.Editing = false
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Click To Edit</h3>
    {{if .Editing}}
    <form method="POST">
        <label>First Name
            <input name="firstName" value="{{.FirstName}}" required>
        </label>
        <label>Last Name
            <input name="lastName" value="{{.LastName}}" required>
        </label>
        <label>Email
            <input name="email" type="email" value="{{.Email}}" required>
        </label>
        <fieldset role="group">
            <button name="save">Save</button>
            <button name="cancel" class="secondary">Cancel</button>
        </fieldset>
    </form>
    {{else}}
    <dl>
        <dt>First Name</dt><dd>{{.FirstName}}</dd>
        <dt>Last Name</dt><dd>{{.LastName}}</dd>
        <dt>Email</dt><dd>{{.Email}}</dd>
    </dl>
    <button name="edit">Edit</button>
    {{end}}
</article>
{{end}}
```

**Key features:** Tier 1 button name routing, `{{if}}` conditional rendering, auto tree diff

---

#### 2. Edit Row

**htmx:** Uses `hx-get`/`hx-put` with `hx-target="closest tr"` to swap individual table rows between view and edit mode.

**LiveTemplate (Tier 1):** Server state tracks which row ID is being edited. Template iterates with `{{range}}` and uses `{{if}}` per row. `data-key` provides stable row identity for efficient diffing.

**Also implemented in:** —

```go
type EditRowState struct {
    Title     string
    Contacts  []Contact
    EditingID string
}

type Contact struct {
    ID    string
    Name  string
    Email string
}

func (c *Controller) Edit(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
    state.EditingID = ctx.GetString("id")
    return state, nil
}

func (c *Controller) Save(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
    id := ctx.GetString("id")
    for i, contact := range state.Contacts {
        if contact.ID == id {
            state.Contacts[i].Name = ctx.GetString("name")
            state.Contacts[i].Email = ctx.GetString("email")
            break
        }
    }
    state.EditingID = ""
    return state, nil
}

func (c *Controller) Cancel(state EditRowState, ctx *livetemplate.Context) (EditRowState, error) {
    state.EditingID = ""
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Edit Row</h3>
    <table>
        <thead><tr><th>Name</th><th>Email</th><th></th></tr></thead>
        <tbody>
        {{range .Contacts}}
        <tr data-key="{{.ID}}">
            {{if eq $.EditingID .ID}}
            <td>
                <form method="POST" class="inline">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <input name="name" value="{{.Name}}">
            </td>
            <td><input name="email" value="{{.Email}}"></td>
            <td>
                    <fieldset role="group">
                        <button name="save" class="compact">Save</button>
                        <button name="cancel" class="compact secondary">Cancel</button>
                    </fieldset>
                </form>
            </td>
            {{else}}
            <td>{{.Name}}</td>
            <td>{{.Email}}</td>
            <td>
                <form method="POST" class="inline">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <button name="edit" class="compact secondary outline">Edit</button>
                </form>
            </td>
            {{end}}
        </tr>
        {{end}}
        </tbody>
    </table>
</article>
{{end}}
```

**Key features:** `data-key` for stable row identity, per-row `{{if}}` branching, range diffing

---

#### 3. Inline Validation

**htmx:** Uses `hx-post` with `hx-trigger="changed"` to validate individual fields server-side as the user types.

**LiveTemplate (Tier 1):** The `Change()` method auto-wires debounced input events (300ms) on fields with dynamic `value` attributes. Validation errors display via `.lvt.HasError`/`.lvt.Error` helpers.

**Also implemented in:** `todos/`

```go
type InlineValidationState struct {
    Title    string
    Email    string
    Username string
    Saved    bool
}

func (c *Controller) Change(state InlineValidationState, ctx *livetemplate.Context) (InlineValidationState, error) {
    if ctx.Has("email") {
        state.Email = ctx.GetString("email")
    }
    if ctx.Has("username") {
        state.Username = ctx.GetString("username")
    }
    // Validate on every change
    _ = ctx.ValidateForm()
    return state, nil
}

func (c *Controller) Submit(state InlineValidationState, ctx *livetemplate.Context) (InlineValidationState, error) {
    if err := ctx.ValidateForm(); err != nil {
        return state, err
    }
    state.Saved = true
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Inline Validation</h3>
    <form method="POST">
        <label>Email
            <input type="email" name="email" value="{{.Email}}" required
                {{if .lvt.HasError "email"}}aria-invalid="true"{{end}}>
            {{if .lvt.HasError "email"}}<small>{{.lvt.Error "email"}}</small>{{end}}
        </label>
        <label>Username
            <input name="username" value="{{.Username}}" required minlength="3" maxlength="20"
                {{if .lvt.HasError "username"}}aria-invalid="true"{{end}}>
            {{if .lvt.HasError "username"}}<small>{{.lvt.Error "username"}}</small>{{end}}
        </label>
        <button type="submit">Submit</button>
    </form>
    {{if .Saved}}<ins style="display:block;text-decoration:none">Saved successfully!</ins>{{end}}
</article>
{{end}}
```

**Key features:** `Change()` auto-binding, `ctx.ValidateForm()`, `.lvt.HasError`/`.lvt.Error`, HTML validation attributes

---

#### 4. Bulk Update

**htmx:** Uses `hx-post` with checkboxes in a single form. Custom name format `active:email` groups selections.

**LiveTemplate (Tier 1):** Standard form with checkboxes. Button name routes to `BulkUpdate()`. Checkbox state arrives via `ctx.GetBool()`.

**Also implemented in:** —

```go
type BulkUpdateState struct {
    Title string
    Users []UserRow
}

type UserRow struct {
    ID     string
    Name   string
    Email  string
    Active bool
}

func (c *Controller) BulkUpdate(state BulkUpdateState, ctx *livetemplate.Context) (BulkUpdateState, error) {
    for i, user := range state.Users {
        state.Users[i].Active = ctx.GetBool("active-" + user.ID)
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Bulk Update</h3>
    <form method="POST">
        <table>
            <thead><tr><th>Name</th><th>Email</th><th>Active</th></tr></thead>
            <tbody>
            {{range .Users}}
            <tr data-key="{{.ID}}">
                <td>{{.Name}}</td>
                <td>{{.Email}}</td>
                <td><input type="checkbox" name="active-{{.ID}}" {{if .Active}}checked{{end}}></td>
            </tr>
            {{end}}
            </tbody>
        </table>
        <button name="bulkUpdate">Update</button>
    </form>
</article>
{{end}}
```

**Key features:** Tier 1 checkbox handling, `ctx.GetBool()`, batch operations

---

#### 5. Reset User Input

**htmx:** Requires `hx-on::after-request="if(event.detail.successful) this.reset()"` (inline JS) to clear forms after submission.

**LiveTemplate (Tier 1):** Forms auto-reset after successful submission by default — no attributes needed. For explicit control, use `lvt-el:reset:on:success` (Tier 2).

**Also implemented in:** `todos/`

```go
type ResetInputState struct {
    Title    string
    Messages []string
}

func (c *Controller) Submit(state ResetInputState, ctx *livetemplate.Context) (ResetInputState, error) {
    msg := ctx.GetString("message")
    if msg != "" {
        state.Messages = append(state.Messages, msg)
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Reset User Input</h3>
    <p><small>Form clears automatically after successful submission — no extra attributes needed.</small></p>
    <form method="POST">
        <fieldset role="group">
            <input name="message" placeholder="Type a message..." required>
            <button type="submit">Send</button>
        </fieldset>
    </form>
    {{range .Messages}}
    <p>{{.}}</p>
    {{end}}
</article>
{{end}}
```

**Key features:** Auto-reset on success (default), explicit `lvt-el:reset:on:success`

---

#### 6. File Upload

**htmx:** Uses `hx-post` with `hx-encoding='multipart/form-data'` and tracks progress via `htmx:xhr:progress` events.

**LiveTemplate (Tier 1+2):** Tier 1 uses standard `<input type="file">` with multipart form. Tier 2 uses `lvt-upload` for chunked uploads with progress tracking via WebSocket.

**Also implemented in:** `avatar-upload/`

```go
type FileUploadState struct {
    Title      string
    UploadName string
    Uploaded   bool
}

func (c *Controller) Submit(state FileUploadState, ctx *livetemplate.Context) (FileUploadState, error) {
    if ctx.HasUploads("document") {
        entries := ctx.GetCompletedUploads("document")
        if len(entries) > 0 {
            state.UploadName = entries[0].ClientName
            state.Uploaded = true
        }
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>File Upload</h3>

    <h4>Tier 1: Standard HTML</h4>
    <form method="POST" enctype="multipart/form-data">
        <input type="file" name="document">
        <button type="submit">Upload</button>
    </form>

    <h4>Tier 2: Chunked with Progress</h4>
    <form method="POST">
        <input type="file" lvt-upload="document" name="document">
        <button type="submit">Upload</button>
    </form>

    {{if .Uploaded}}
    <ins style="display:block;text-decoration:none">Uploaded: {{.UploadName}}</ins>
    {{end}}
</article>
{{end}}
```

**Key features:** Tier 1 multipart, Tier 2 `lvt-upload`, `ctx.HasUploads()`/`ctx.GetCompletedUploads()`

---

#### 7. Preserving File Inputs

**htmx:** Requires custom JavaScript to preserve file input state after form errors.

**LiveTemplate (Tier 2):** `lvt-form:preserve` on the form retains all non-file field values across re-renders.

**Also implemented in:** `shared-notepad/`

```html
{{define "content"}}
<article>
    <h3>Preserving Form Inputs</h3>
    <form method="POST" lvt-form:preserve>
        <label>Name
            <input name="name" value="{{.Name}}" required>
        </label>
        <label>Description
            <textarea name="description">{{.Description}}</textarea>
        </label>
        <label>Attachment
            <input type="file" name="attachment">
        </label>
        <button type="submit">Submit</button>
    </form>
    {{if .lvt.HasError "name"}}
    <del style="display:block;text-decoration:none">{{.lvt.Error "name"}}</del>
    {{end}}
</article>
{{end}}
```

**Key features:** `lvt-form:preserve`, form values survive validation errors

---

### Category 2: Lists & Data

#### 8. Delete Row

**htmx:** Uses `hx-delete` with `hx-confirm` and `hx-swap="outerHTML swap:1s"` for animated row removal.

**LiveTemplate (Tier 1+2):** Button name="delete" with hidden input for ID. `lvt-fx:animate="fade"` provides entry animation. The tree diff removes the row from the range automatically.

**Also implemented in:** `todos/`

```go
type DeleteRowState struct {
    Title string
    Items []Item
}

type Item struct {
    ID   string
    Name string
}

func (c *Controller) Delete(state DeleteRowState, ctx *livetemplate.Context) (DeleteRowState, error) {
    id := ctx.GetString("id")
    filtered := state.Items[:0]
    for _, item := range state.Items {
        if item.ID != id {
            filtered = append(filtered, item)
        }
    }
    state.Items = filtered
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Delete Row</h3>
    <table>
        <thead><tr><th>Name</th><th></th></tr></thead>
        <tbody>
        {{range .Items}}
        <tr data-key="{{.ID}}" lvt-fx:animate="fade">
            <td>{{.Name}}</td>
            <td>
                <form method="POST" class="inline">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <button name="delete" class="compact contrast outline">Delete</button>
                </form>
            </td>
        </tr>
        {{end}}
        </tbody>
    </table>
</article>
{{end}}
```

**Key features:** Range diffing with `data-key`, `lvt-fx:animate="fade"` entry animation

---

#### 9. Click To Load

**htmx:** Uses `hx-get` with pagination URL and `hx-swap="outerHTML"` on a "Load More" button that replaces itself with next page content.

**LiveTemplate (Tier 1):** Server state tracks current page. Button name="loadMore" increments page and appends items. No explicit swap targeting needed — tree diff handles it.

**Also implemented in:** —

```go
const pageSize = 10

type ClickToLoadState struct {
    Title       string
    Items       []Item
    CurrentPage int
    HasMore     bool
}

func (c *Controller) LoadMore(state ClickToLoadState, ctx *livetemplate.Context) (ClickToLoadState, error) {
    state.CurrentPage++
    newItems := c.getPage(state.CurrentPage, pageSize)
    state.Items = append(state.Items, newItems...)
    state.HasMore = len(newItems) == pageSize
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Click To Load</h3>
    <table>
        <thead><tr><th>#</th><th>Name</th><th>Email</th></tr></thead>
        <tbody>
        {{range .Items}}
        <tr data-key="{{.ID}}">
            <td>{{.ID}}</td><td>{{.Name}}</td><td>{{.Email}}</td>
        </tr>
        {{end}}
        </tbody>
    </table>
    {{if .HasMore}}
    <button name="loadMore">Load More...</button>
    {{end}}
</article>
{{end}}
```

**Key features:** Append-only pagination, conditional "Load More" button

---

#### 10. Infinite Scroll

**htmx:** Uses `hx-get` with `hx-trigger="revealed"` and `hx-swap="afterend"` on the last row to auto-load on scroll.

**LiveTemplate (Tier 1):** The client has built-in IntersectionObserver support. A `<div id="scroll-sentinel">` at the end of the list triggers the `load_more` action automatically when it becomes visible.

**Also implemented in:** —

```go
type InfiniteScrollState struct {
    Title       string
    Items       []Item
    CurrentPage int
    HasMore     bool
}

func (c *Controller) LoadMore(state InfiniteScrollState, ctx *livetemplate.Context) (InfiniteScrollState, error) {
    state.CurrentPage++
    newItems := c.getPage(state.CurrentPage, pageSize)
    state.Items = append(state.Items, newItems...)
    state.HasMore = len(newItems) == pageSize
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Infinite Scroll</h3>
    <table>
        <thead><tr><th>#</th><th>Name</th><th>Email</th></tr></thead>
        <tbody>
        {{range .Items}}
        <tr data-key="{{.ID}}">
            <td>{{.ID}}</td><td>{{.Name}}</td><td>{{.Email}}</td>
        </tr>
        {{end}}
        </tbody>
    </table>
    {{if .HasMore}}
    <div id="scroll-sentinel"><small>Loading more...</small></div>
    {{end}}
</article>
{{end}}
```

**Key features:** `scroll-sentinel` auto-triggers `load_more`, IntersectionObserver built into client

---

#### 11. Value Select (Cascading Selects)

**htmx:** Uses `hx-get` with `hx-target` to load dependent options when a parent select changes.

**LiveTemplate (Tier 1):** The `Change()` method detects which select changed and updates the dependent data in state. Template re-renders with updated options automatically.

**Also implemented in:** `todos/` (sort dropdown)

```go
type ValueSelectState struct {
    Title    string
    Makes    []string
    Models   []string
    Make     string
    Model    string
}

func (c *Controller) Change(state ValueSelectState, ctx *livetemplate.Context) (ValueSelectState, error) {
    if ctx.Has("make") {
        state.Make = ctx.GetString("make")
        state.Models = c.getModels(state.Make)
        state.Model = ""
    }
    if ctx.Has("model") {
        state.Model = ctx.GetString("model")
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Value Select</h3>
    <form method="POST">
        <label>Make
            <select name="make">
                <option value="">Select Make</option>
                {{range .Makes}}
                <option value="{{.}}" {{if eq $.Make .}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
        </label>
        <label>Model
            <select name="model">
                <option value="">Select Model</option>
                {{range .Models}}
                <option value="{{.}}" {{if eq $.Model .}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
        </label>
    </form>
    {{if .Model}}<p>Selected: {{.Make}} {{.Model}}</p>{{end}}
</article>
{{end}}
```

**Key features:** `Change()` auto-binding on select elements, cascading state updates

---

### Category 3: Search & Filtering

#### 12. Active Search

**htmx:** Uses `hx-post` with `hx-trigger="input changed delay:500ms, keyup[key=='Enter']"` for debounced search.

**LiveTemplate (Tier 1):** The `Change()` method auto-wires debounced input events (300ms). `input type="search"` provides a native clear button. No `lvt-*` attributes needed.

**Also implemented in:** `todos/`

```go
type ActiveSearchState struct {
    Title   string
    Query   string
    Results []Contact
}

func (c *Controller) Change(state ActiveSearchState, ctx *livetemplate.Context) (ActiveSearchState, error) {
    if ctx.Has("query") {
        state.Query = ctx.GetString("query")
        state.Results = c.search(state.Query)
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Active Search</h3>
    <input type="search" name="query" value="{{.Query}}" placeholder="Search contacts...">
    <table>
        <thead><tr><th>Name</th><th>Email</th></tr></thead>
        <tbody>
        {{range .Results}}
        <tr data-key="{{.ID}}"><td>{{.Name}}</td><td>{{.Email}}</td></tr>
        {{end}}
        </tbody>
    </table>
    {{if not .Results}}<p><small>No results found.</small></p>{{end}}
</article>
{{end}}
```

**Key features:** `Change()` with auto-debounce, `input type="search"` native clear, zero `lvt-*` attributes

---

#### 13. URL-Preserved Filters

**Source:** Phoenix LiveView (`handle_params` + `push_patch`)

**LiveTemplate (Tier 1):** SPA navigation via link interception preserves filters in the URL. `Mount()` reads query parameters to restore state. Users can bookmark and share filtered views.

**Also implemented in:** —

```go
type URLFiltersState struct {
    Title    string
    Status   string
    Sort     string
    Items    []Item
}

func (c *Controller) Mount(state URLFiltersState, ctx *livetemplate.Context) (URLFiltersState, error) {
    // Read filters from URL query params
    if status := ctx.GetString("status"); status != "" {
        state.Status = status
    }
    if sort := ctx.GetString("sort"); sort != "" {
        state.Sort = sort
    }
    state.Items = c.filter(state.Status, state.Sort)
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>URL-Preserved Filters</h3>
    <nav>
        <ul>
            <li><a href="?status=all&sort={{.Sort}}">All</a></li>
            <li><a href="?status=active&sort={{.Sort}}">Active</a></li>
            <li><a href="?status=completed&sort={{.Sort}}">Completed</a></li>
        </ul>
        <ul>
            <li><a href="?status={{.Status}}&sort=name">By Name</a></li>
            <li><a href="?status={{.Status}}&sort=date">By Date</a></li>
        </ul>
    </nav>
    <table>
        <thead><tr><th>Name</th><th>Status</th><th>Date</th></tr></thead>
        <tbody>
        {{range .Items}}
        <tr data-key="{{.ID}}"><td>{{.Name}}</td><td>{{.Status}}</td><td>{{.Date}}</td></tr>
        {{end}}
        </tbody>
    </table>
</article>
{{end}}
```

**Key features:** Link interception + pushState, `Mount()` reads query params, bookmarkable URLs

---

### Category 4: Loading & Progress

#### 14. Lazy Loading

**htmx:** Uses `hx-get` with `hx-trigger="load"` to fetch content after the page renders, showing a spinner placeholder.

**LiveTemplate (Tier 1):** `Mount()` sets a loading state. A background goroutine simulates a slow data fetch, then calls `session.TriggerAction()` to push the loaded data to the client.

**Also implemented in:** `login/` (server push)

```go
type LazyLoadState struct {
    Title   string
    Loading bool
    Data    string
}

func (c *Controller) Mount(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
    state.Loading = true
    return state, nil
}

func (c *Controller) OnConnect(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
    session := ctx.Session()
    go func() {
        time.Sleep(2 * time.Second) // Simulate slow API call
        _ = session.TriggerAction("dataLoaded", map[string]interface{}{
            "data": "This content was loaded lazily from the server!",
        })
    }()
    return state, nil
}

func (c *Controller) DataLoaded(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
    state.Data = ctx.GetString("data")
    state.Loading = false
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Lazy Loading</h3>
    {{if .Loading}}
    <p aria-busy="true">Loading content...</p>
    {{else}}
    <p>{{.Data}}</p>
    {{end}}
</article>
{{end}}
```

**Key features:** `session.TriggerAction()` from goroutine, server push, loading → loaded state machine

---

#### 15. Progress Bar

**htmx:** Uses polling (`hx-trigger="every 600ms"`) with a `HX-Trigger: done` response header to track job progress.

**LiveTemplate (Tier 1):** A background goroutine updates progress via `session.TriggerAction()` at intervals. The native `<progress>` element renders the current value. No polling — updates are pushed via WebSocket.

**Also implemented in:** —

```go
type ProgressBarState struct {
    Title    string
    Progress int
    Running  bool
    Done     bool
}

func (c *Controller) Start(state ProgressBarState, ctx *livetemplate.Context) (ProgressBarState, error) {
    state.Running = true
    state.Progress = 0
    state.Done = false
    session := ctx.Session()
    go func() {
        for i := 1; i <= 100; i += 10 {
            time.Sleep(500 * time.Millisecond)
            _ = session.TriggerAction("updateProgress", map[string]interface{}{
                "progress": i,
            })
        }
    }()
    return state, nil
}

func (c *Controller) UpdateProgress(state ProgressBarState, ctx *livetemplate.Context) (ProgressBarState, error) {
    state.Progress = ctx.GetInt("progress")
    if state.Progress >= 100 {
        state.Running = false
        state.Done = true
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Progress Bar</h3>
    {{if .Running}}
    <progress value="{{.Progress}}" max="100"></progress>
    <p><small>{{.Progress}}% complete</small></p>
    {{else if .Done}}
    <ins style="display:block;text-decoration:none">Job complete!</ins>
    <button name="start">Run Again</button>
    {{else}}
    <button name="start">Start Job</button>
    {{end}}
</article>
{{end}}
```

**Key features:** Server push via goroutine + `TriggerAction()`, no polling, `<progress>` element

---

#### 16. Async Operations

**Source:** Phoenix LiveView (`assign_async` / `AsyncResult`)

**LiveTemplate (Tier 1):** Demonstrates the loading/success/error state machine pattern using goroutines and `TriggerAction()`. Multiple async operations can run concurrently with independent state.

**Also implemented in:** —

```go
type AsyncState struct {
    Title   string
    Status  string // "idle", "loading", "success", "error"
    Result  string
    Error   string
}

func (c *Controller) Fetch(state AsyncState, ctx *livetemplate.Context) (AsyncState, error) {
    state.Status = "loading"
    state.Result = ""
    state.Error = ""
    session := ctx.Session()
    go func() {
        time.Sleep(2 * time.Second)
        // Simulate success or failure
        _ = session.TriggerAction("fetchResult", map[string]interface{}{
            "result":  "Data fetched successfully at " + time.Now().Format("15:04:05"),
            "success": true,
        })
    }()
    return state, nil
}

func (c *Controller) FetchResult(state AsyncState, ctx *livetemplate.Context) (AsyncState, error) {
    if ctx.GetBool("success") {
        state.Status = "success"
        state.Result = ctx.GetString("result")
    } else {
        state.Status = "error"
        state.Error = ctx.GetString("error")
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Async Operations</h3>
    <button name="fetch" {{if eq .Status "loading"}}aria-busy="true" disabled{{end}}>
        {{if eq .Status "loading"}}Fetching...{{else}}Fetch Data{{end}}
    </button>
    {{if eq .Status "success"}}
    <ins style="display:block;text-decoration:none">{{.Result}}</ins>
    {{end}}
    {{if eq .Status "error"}}
    <del style="display:block;text-decoration:none">{{.Error}}</del>
    {{end}}
</article>
{{end}}
```

**Key features:** State machine (idle → loading → success/error), concurrent goroutines, no polling

---

### Category 5: Dialogs, Tabs & Navigation

#### 17. Modal Dialog

**htmx:** Uses `hx-get` to fetch modal HTML into a target container, then opens it with Bootstrap/UIKit JS.

**LiveTemplate (Tier 1):** Native `<dialog>` element with `command`/`commandfor` attributes (polyfilled for Firefox/Safari). Focus trapping, backdrop, and Escape key are all browser-native with `showModal()`.

**Also implemented in:** `todos/`

```html
{{define "content"}}
<article>
    <h3>Modal Dialog</h3>
    <button command="show-modal" commandfor="edit-dialog">Open Modal</button>

    <dialog id="edit-dialog">
        <article>
            <header>
                <button command="close" commandfor="edit-dialog" aria-label="Close" rel="prev"></button>
                <h4>Edit Item</h4>
            </header>
            <form method="POST">
                <label>Name
                    <input name="name" value="{{.Name}}" required>
                </label>
                <button name="save" type="submit">Save</button>
            </form>
        </article>
    </dialog>
</article>
{{end}}
```

**Key features:** Native `<dialog>`, `command`/`commandfor` (polyfilled), auto-close on form success

---

#### 18. Confirm Dialog

**htmx:** Uses `hx-confirm` or custom SweetAlert2 integration (inline JS) for confirmation before destructive actions.

**LiveTemplate (Tier 1):** Uses `<dialog>` with `command`/`commandfor` for a CSP-compliant confirmation flow. The destructive action form lives inside the dialog. No inline JavaScript.

**Also implemented in:** `todos/`

```html
{{define "content"}}
<article>
    <h3>Confirm Dialog</h3>
    {{range .Items}}
    <div data-key="{{.ID}}">
        <span>{{.Name}}</span>
        <button command="show-modal" commandfor="confirm-{{.ID}}" class="compact contrast outline">Delete</button>

        <dialog id="confirm-{{.ID}}">
            <article>
                <h4>Confirm Delete</h4>
                <p>Are you sure you want to delete "{{.Name}}"?</p>
                <footer>
                    <form method="POST" class="inline">
                        <input type="hidden" name="id" value="{{.ID}}">
                        <fieldset role="group">
                            <button command="close" commandfor="confirm-{{.ID}}" class="secondary" type="button">Cancel</button>
                            <button name="delete">Delete</button>
                        </fieldset>
                    </form>
                </footer>
            </article>
        </dialog>
    </div>
    {{end}}
</article>
{{end}}
```

**Key features:** `command`/`commandfor` (no inline JS), `<dialog>` auto-close on success, CSP-compliant

---

#### 19. Tabs (HATEOAS)

**htmx:** Uses `hx-get` with `hx-trigger="load"` and `hx-swap="innerHTML"` to load tab content server-side. Server returns full tab markup with selected state.

**LiveTemplate (Tier 1):** SPA navigation via `<a href>` link interception. The server renders the active tab based on URL path. No JavaScript tab management — the server decides what's active.

**Also implemented in:** —

```go
type TabsState struct {
    Title     string
    ActiveTab string
    Content   string
}

func (c *Controller) Mount(state TabsState, ctx *livetemplate.Context) (TabsState, error) {
    tab := ctx.GetString("tab")
    if tab == "" {
        tab = "tab1"
    }
    state.ActiveTab = tab
    state.Content = c.getTabContent(tab)
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Tabs (Server-Driven)</h3>
    <nav>
        <ul>
            <li><a href="?tab=tab1" {{if eq .ActiveTab "tab1"}}aria-current="page"{{end}}>Tab 1</a></li>
            <li><a href="?tab=tab2" {{if eq .ActiveTab "tab2"}}aria-current="page"{{end}}>Tab 2</a></li>
            <li><a href="?tab=tab3" {{if eq .ActiveTab "tab3"}}aria-current="page"{{end}}>Tab 3</a></li>
        </ul>
    </nav>
    <div>{{.Content}}</div>
</article>
{{end}}
```

**Key features:** SPA navigation, server-driven tab state via query params, `aria-current` for active tab

---

#### 20. SPA Navigation

**Source:** Phoenix LiveView (`push_patch` / `push_navigate`)

**LiveTemplate (Tier 1):** All `<a href>` links inside the LiveTemplate wrapper are auto-intercepted for SPA navigation. The page is fetched via `fetch()`, DOM is patched, and `pushState` updates the URL — no full page reload.

**Also implemented in:** —

This pattern is demonstrated by the index page itself — clicking between patterns uses SPA navigation. Each pattern page is a separate LiveTemplate handler, and navigation between them is seamless.

```html
<!-- Navigation between patterns uses standard links — intercepted automatically -->
<nav>
    <a href="/patterns/forms/click-to-edit">Click To Edit</a>
    <a href="/patterns/forms/edit-row">Edit Row</a>
    <a href="/patterns/lists/delete-row">Delete Row</a>
</nav>

<!-- Opt-out for external links -->
<a href="https://htmx.org/examples/" lvt-nav:no-intercept>htmx.org</a>
```

**Key features:** Auto link interception, `pushState`, `lvt-nav:no-intercept` for opt-out

---

#### 21. Keyboard Shortcuts

**htmx:** Uses `hx-trigger="keyup[altKey&&shiftKey&&key=='D'] from:body"` for global keyboard shortcuts.

**LiveTemplate (Tier 2):** `lvt-on:window:keydown` binds global keyboard events. `lvt-key` filters by specific key. No custom JavaScript.

**Also implemented in:** —

```go
func (c *Controller) Close(state ShortcutsState, ctx *livetemplate.Context) (ShortcutsState, error) {
    state.PanelOpen = false
    return state, nil
}

func (c *Controller) Open(state ShortcutsState, ctx *livetemplate.Context) (ShortcutsState, error) {
    state.PanelOpen = true
    return state, nil
}
```

```html
{{define "content"}}
<article lvt-on:window:keydown="open" lvt-key="/">
    <h3>Keyboard Shortcuts</h3>
    <p><small>Press <kbd>/</kbd> to open panel, <kbd>Escape</kbd> to close</small></p>

    {{if .PanelOpen}}
    <div lvt-on:window:keydown="close" lvt-key="Escape">
        <article>
            <h4>Command Panel</h4>
            <input type="search" placeholder="Type a command..." autofocus>
        </article>
    </div>
    {{end}}
</article>
{{end}}
```

**Key features:** `lvt-on:window:keydown`, `lvt-key` filter, global event binding

---

### Category 6: Visual Feedback

#### 22. Animations

**htmx:** Uses CSS classes (`htmx-added`, `htmx-swapping`, `htmx-request`) and the `swap`/`settle` timing parameters for transitions.

**LiveTemplate (Tier 2):** `lvt-fx:animate` applies entry animations (`fade`, `slide`, `scale`). Duration configurable via CSS custom property `--lvt-animate-duration`.

**Also implemented in:** `todos/`

```html
{{define "content"}}
<article>
    <h3>Animations</h3>
    <button name="addItem">Add Item</button>

    {{range .Items}}
    <div data-key="{{.ID}}" lvt-fx:animate="fade">
        <p>{{.Name}} — added at {{.Time}}</p>
    </div>
    {{end}}

    <h4>Animation Types</h4>
    <div lvt-fx:animate="fade">Fade In (default)</div>
    <div lvt-fx:animate="slide">Slide In</div>
    <div lvt-fx:animate="scale" style="--lvt-animate-duration: 500ms">Scale In (500ms)</div>
</article>
{{end}}
```

**Key features:** `lvt-fx:animate="fade|slide|scale"`, `--lvt-animate-duration` CSS custom property

---

#### 23. Loading States

**htmx:** Uses the `htmx-request` CSS class to show loading indicators during requests.

**LiveTemplate (Tier 1+2):** Automatic behavior: forms get `aria-busy="true"` and fieldsets get `disabled` during submission. For custom UX, `lvt-form:disable-with` changes button text, and `lvt-el:*:on:pending` adds reactive DOM changes.

**Also implemented in:** —

```html
{{define "content"}}
<article>
    <h3>Loading States</h3>

    <h4>Tier 1: Automatic</h4>
    <p><small>Form gets aria-busy="true", fieldset gets disabled during submit.</small></p>
    <form method="POST">
        <fieldset>
            <input name="data" placeholder="Type something..." required>
            <button name="slowSave" type="submit">Save (2s delay)</button>
        </fieldset>
    </form>

    <h4>Tier 2: Custom Loading Text</h4>
    <form method="POST">
        <input name="data" placeholder="Type something..." required>
        <button name="slowSave" type="submit" lvt-form:disable-with="Saving...">Save</button>
    </form>

    <h4>Tier 2: Reactive Attributes</h4>
    <form method="POST">
        <input name="data" placeholder="Type something..." required>
        <button name="slowSave" type="submit"
            lvt-el:setAttr:on:pending="aria-busy:true"
            lvt-el:setAttr:on:done="aria-busy:false">
            Save
        </button>
    </form>
</article>
{{end}}
```

**Key features:** Auto `aria-busy`/`disabled`, `lvt-form:disable-with`, `lvt-el:setAttr:on:pending`

---

#### 24. Highlight on Change

**Source:** Phoenix LiveView (visual feedback on state changes)

**LiveTemplate (Tier 2):** `lvt-fx:highlight="flash"` temporarily highlights elements after DOM updates. Color and duration configurable via CSS custom properties.

**Also implemented in:** `todos/`

```html
{{define "content"}}
<article>
    <h3>Highlight on Change</h3>
    <button name="increment">Increment</button>
    <div lvt-fx:highlight="flash">
        <p>Counter: {{.Counter}}</p>
    </div>

    <h4>Custom Highlight</h4>
    <div lvt-fx:highlight="flash"
         style="--lvt-highlight-color: #4caf50; --lvt-highlight-duration: 1000ms">
        <p>Custom color + duration: {{.Counter}}</p>
    </div>
</article>
{{end}}
```

**Key features:** `lvt-fx:highlight="flash"`, `--lvt-highlight-color`, `--lvt-highlight-duration`

---

#### 25. Flash Messages / Toasts

**Source:** Phoenix LiveView (`put_flash` / temporary assigns)

**LiveTemplate (Tier 1):** `ctx.SetFlash()` stores messages that render as toasts. The toast component handles auto-dismiss and positioning.

**Also implemented in:** `flash-messages/`, `todos/`

```go
func (c *Controller) Save(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
    name := ctx.GetString("name")
    if name == "" {
        ctx.SetFlash("error", "Name is required")
        return state, nil
    }
    ctx.SetFlash("success", "Saved: "+name)
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Flash Messages</h3>
    <form method="POST">
        <input name="name" placeholder="Enter name...">
        <button name="save">Save</button>
    </form>
    <p><small>Try submitting empty for error, or with a name for success.</small></p>
</article>
{{end}}
```

**Key features:** `ctx.SetFlash()`, toast component, auto-dismiss

---

### Category 7: Real-Time & Multi-User

#### 26. Multi-User Sync

**Source:** Phoenix LiveView (PubSub broadcast + handle_info)

**LiveTemplate (Tier 1):** The `Sync()` handler is auto-dispatched to peer connections in the same session group. Multiple tabs or users see the same state changes in real-time.

**Also implemented in:** `chat/`, `shared-notepad/`

```go
type SyncState struct {
    Title   string
    Counter int
}

// Sync is called automatically when another connection in the same session group
// triggers an action. The state parameter contains the updated state.
func (c *Controller) Sync(state SyncState, ctx *livetemplate.Context) (SyncState, error) {
    return state, nil
}

func (c *Controller) Increment(state SyncState, ctx *livetemplate.Context) (SyncState, error) {
    state.Counter++
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Multi-User Sync</h3>
    <p><small>Open this page in two tabs. Changes sync automatically.</small></p>
    <p>Counter: {{.Counter}}</p>
    <button name="increment">Increment</button>
</article>
{{end}}
```

**Key features:** `Sync()` handler, session group auto-sync, multi-tab real-time updates

---

#### 27. Broadcasting

**Source:** Phoenix LiveView (`Phoenix.PubSub.broadcast`)

**LiveTemplate (Tier 1):** `ctx.BroadcastAction()` sends updates to all connections in a group, to a specific user, or globally.

**Also implemented in:** `chat/`

```go
func (c *Controller) SendMessage(state BroadcastState, ctx *livetemplate.Context) (BroadcastState, error) {
    msg := ctx.GetString("message")
    c.mu.Lock()
    c.messages = append(c.messages, Message{Text: msg, User: state.Username})
    c.mu.Unlock()
    // Notify all other connections
    ctx.BroadcastAction("NewMessage", nil)
    return state, nil
}

func (c *Controller) NewMessage(state BroadcastState, ctx *livetemplate.Context) (BroadcastState, error) {
    c.mu.RLock()
    state.Messages = c.copyMessages()
    c.mu.RUnlock()
    return state, nil
}
```

**Key features:** `ctx.BroadcastAction()`, shared controller state with mutex, cross-connection updates

---

#### 28. Presence Tracking

**Source:** Phoenix Presence (CRDTs for distributed presence)

**LiveTemplate (Tier 1):** `OnConnect()` and `OnDisconnect()` lifecycle hooks track who's online. Broadcast updates to show live user counts.

**Also implemented in:** `chat/`

```go
func (c *Controller) OnConnect(state PresenceState, ctx *livetemplate.Context) (PresenceState, error) {
    c.mu.Lock()
    c.onlineUsers[state.Username] = true
    state.OnlineCount = len(c.onlineUsers)
    c.mu.Unlock()
    ctx.BroadcastAction("PresenceChanged", nil)
    return state, nil
}

func (c *Controller) OnDisconnect() {
    // Clean up on disconnect
}

func (c *Controller) PresenceChanged(state PresenceState, ctx *livetemplate.Context) (PresenceState, error) {
    c.mu.RLock()
    state.OnlineCount = len(c.onlineUsers)
    c.mu.RUnlock()
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Presence Tracking</h3>
    <p><mark>{{.OnlineCount}} user(s) online</mark></p>
</article>
{{end}}
```

**Key features:** `OnConnect()`/`OnDisconnect()`, `BroadcastAction()` for presence updates

---

#### 29. Reconnection Recovery

**Source:** Phoenix LiveView (automatic reconnection with state recovery)

**LiveTemplate (Tier 1):** State fields tagged with `lvt:"persist"` survive disconnection and reconnection. The client auto-reconnects and the framework restores persisted state.

**Also implemented in:** —

```go
type ReconnectState struct {
    Title   string
    Counter int    `lvt:"persist"`
    Notes   string `lvt:"persist"`
}

func (c *Controller) Increment(state ReconnectState, ctx *livetemplate.Context) (ReconnectState, error) {
    state.Counter++
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Reconnection Recovery</h3>
    <p><small>Try disconnecting your network briefly. State persists across reconnections.</small></p>
    <p>Counter: {{.Counter}} <small>(persisted)</small></p>
    <button name="increment">Increment</button>
    <form method="POST" lvt-form:preserve>
        <label>Notes <small>(persisted)</small>
            <textarea name="notes">{{.Notes}}</textarea>
        </label>
    </form>
</article>
{{end}}
```

**Key features:** `lvt:"persist"` state tag, auto-reconnect, state recovery

---

#### 30. Live Preview

**Source:** Phoenix LiveView (live input binding)

**LiveTemplate (Tier 1):** The `Change()` method auto-wires debounced input events. As the user types, the server updates state and the preview renders in real-time.

**Also implemented in:** `live-preview/`

```go
type LivePreviewState struct {
    Title   string
    Input   string
    Preview string
}

func (c *Controller) Change(state LivePreviewState, ctx *livetemplate.Context) (LivePreviewState, error) {
    if ctx.Has("input") {
        state.Input = ctx.GetString("input")
        state.Preview = c.renderPreview(state.Input) // e.g., markdown → HTML
    }
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Live Preview</h3>
    <div class="grid">
        <div>
            <label>Input
                <textarea name="input">{{.Input}}</textarea>
            </label>
        </div>
        <div>
            <label>Preview</label>
            <blockquote>{{.Preview}}</blockquote>
        </div>
    </div>
</article>
{{end}}
```

**Key features:** `Change()` auto-binding, 300ms debounce, split-pane preview

---

#### 31. Server Push

**Source:** Phoenix LiveView (`send` / `handle_info`)

**LiveTemplate (Tier 1):** `session.TriggerAction()` pushes updates from background goroutines. The controller method handles the pushed action and updates state.

**Also implemented in:** `login/`

```go
func (c *Controller) StartTimer(state PushState, ctx *livetemplate.Context) (PushState, error) {
    state.Running = true
    session := ctx.Session()
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        for i := 0; i < 10; i++ {
            <-ticker.C
            _ = session.TriggerAction("tick", map[string]interface{}{
                "elapsed": i + 1,
            })
        }
        _ = session.TriggerAction("timerDone", nil)
    }()
    return state, nil
}

func (c *Controller) Tick(state PushState, ctx *livetemplate.Context) (PushState, error) {
    state.Elapsed = ctx.GetInt("elapsed")
    return state, nil
}

func (c *Controller) TimerDone(state PushState, ctx *livetemplate.Context) (PushState, error) {
    state.Running = false
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Server Push</h3>
    {{if .Running}}
    <p aria-busy="true">Timer: {{.Elapsed}}s</p>
    {{else}}
    <button name="startTimer">Start 10s Timer</button>
    {{if .Elapsed}}<p>Timer completed: {{.Elapsed}}s</p>{{end}}
    {{end}}
</article>
{{end}}
```

**Key features:** `session.TriggerAction()`, goroutine lifecycle, server-to-client push

---

## Skipped Patterns

| htmx Pattern | Reason |
|---|---|
| Sortable / Drag-and-Drop | Requires Sortable.js (custom JS). Violates "no custom JavaScript" principle. See [Future Features](#future-features) |
| Web Components | LiveTemplate doesn't use Shadow DOM or web components |
| moveBefore() API | DOM-level optimization handled by LiveTemplate's morphdom internally |
| Bootstrap / UIKit Modals | LiveTemplate uses Pico CSS + native `<dialog>` |
| JS Tabs | Server-driven tabs (HATEOAS) is the LiveTemplate way |
| Async Authentication | Authentication is server-side middleware in LiveTemplate. No client-side token handling needed |

### Why "Update Other Content" (OOB Swaps) Is Unnecessary

htmx's "Update Other Content" pattern exists because htmx patches individual HTML fragments — updating element B when element A triggers the request requires explicit out-of-band swap markup.

LiveTemplate re-renders the **entire template** server-side and diffs the tree. Only changed dynamics are sent. Changing any state field automatically updates every template expression that references it. This means every pattern above implicitly demonstrates multi-area updates — no OOB mechanism needed.

---

## Future Features

### Drag-and-Drop / Sortable

**Scope:** Client repo (`github.com/livetemplate/client`) + possible core repo changes

**Requirements:**
- Client: Add `lvt-on:dragstart`, `lvt-on:dragover`, `lvt-on:drop` event support
- Client: Serialize drag source/target data in action message
- Core: Reorder protocol for range items (extend existing range operations)

**Not blocking for the patterns example.** This is documented here for future implementation.

---

## Implementation Plan

### Session 1: Scaffold + Forms & Editing

**Scope:** App skeleton, shared layout, patterns #1–7

- [ ] Create `examples/patterns/` directory structure
- [ ] `main.go` — router with all handler registrations
- [ ] `templates/layout.tmpl` — shared HTML layout
- [ ] `data.go` — in-memory sample data (contacts, users, items)
- [ ] `state.go` — state structs for all patterns
- [ ] Implement Click To Edit (#1)
- [ ] Implement Edit Row (#2)
- [ ] Implement Inline Validation (#3)
- [ ] Implement Bulk Update (#4)
- [ ] Implement Reset User Input (#5)
- [ ] Implement File Upload (#6)
- [ ] Implement Preserving File Inputs (#7)

### Session 2: Lists & Data + Search & Filtering

**Scope:** Patterns #8–13

- [ ] Implement Delete Row (#8)
- [ ] Implement Click To Load (#9)
- [ ] Implement Infinite Scroll (#10)
- [ ] Implement Value Select (#11)
- [ ] Implement Active Search (#12)
- [ ] Implement URL-Preserved Filters (#13)

### Session 3: Loading & Progress

**Scope:** Patterns #14–16

- [ ] Implement Lazy Loading (#14)
- [ ] Implement Progress Bar (#15)
- [ ] Implement Async Operations (#16)
- [ ] Verify goroutine cleanup on disconnect

### Session 4: Dialogs, Tabs & Navigation

**Scope:** Patterns #17–21

- [ ] Implement Modal Dialog (#17)
- [ ] Implement Confirm Dialog (#18)
- [ ] Implement Tabs (HATEOAS) (#19)
- [ ] Implement SPA Navigation (#20)
- [ ] Implement Keyboard Shortcuts (#21)

### Session 5: Visual Feedback

**Scope:** Patterns #22–25

- [ ] Implement Animations (#22)
- [ ] Implement Loading States (#23)
- [ ] Implement Highlight on Change (#24)
- [ ] Implement Flash Messages (#25)

### Session 6: Real-Time & Multi-User

**Scope:** Patterns #26–31

- [ ] Implement Multi-User Sync (#26)
- [ ] Implement Broadcasting (#27)
- [ ] Implement Presence Tracking (#28)
- [ ] Implement Reconnection Recovery (#29)
- [ ] Implement Live Preview (#30)
- [ ] Implement Server Push (#31)

### Session 7: Index Page + E2E Tests + Polish

**Scope:** Main index page, chromedp test suite, final review

- [ ] `templates/index.tmpl` — categorized grid of all patterns with descriptions
- [ ] Index handler with pattern metadata
- [ ] E2E chromedp tests for all 31 patterns
  - Exercise every controller method
  - Assert full page state after mutations
  - Use real browser interactions (not WebSocket bypass)
  - Verify form field state after mutations
  - Condition-based waits (not Sleep)
  - Test error/validation paths
- [ ] `README.md` for the patterns example
- [ ] Run `./test-all.sh` to verify all examples still pass
- [ ] Final review and polish

---

## Testing Strategy

All patterns must have chromedp E2E tests following the [Examples CLAUDE.md](https://github.com/livetemplate/examples/blob/main/CLAUDE.md) testing requirements:

1. **Exercise every controller method** — each action must be triggered in tests
2. **Assert full page state after mutations** — not just the changed element
3. **Use real browser interactions** — `chromedp.SendKeys` + `chromedp.Click`, not WebSocket API bypass
4. **Verify form field state** — check input cleared/retained after mutations
5. **Condition-based waits** — `e2etest.WaitFor()`/`e2etest.WaitForText()`, not `chromedp.Sleep`
6. **Test error paths** — validation failures, empty inputs

E2E tests must have access to: browser console logs, server logs, WebSocket messages, rendered HTML.

---

## References

- [htmx.org/examples](https://htmx.org/examples/) — Source patterns
- [Phoenix LiveView docs](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html) — Additional patterns
- [FIRST_PRINCIPLES.md](../design/FIRST_PRINCIPLES.md) — Core design philosophy
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Tier 1 vs Tier 2
- [Client Attributes Reference](../references/client-attributes.md) — Complete `lvt-*` attribute listing
- [Examples CLAUDE.md](https://github.com/livetemplate/examples/blob/main/CLAUDE.md) — Example guidelines
