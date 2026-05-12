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
  handlers_forms.go    # Category 1: Forms & Editing handlers
  handlers_lists.go    # Category 2: Lists & Data handlers
  handlers_search.go   # Category 3: Search & Filtering handlers
  handlers_loading.go  # Category 4: Loading & Progress handlers
  handlers_nav.go      # Category 5: Dialogs, Tabs & Navigation handlers
  handlers_feedback.go # Category 6: Visual Feedback handlers
  handlers_realtime.go # Category 7: Real-Time & Multi-User handlers
  state_forms.go       # State structs: Forms & Editing
  state_lists.go       # State structs: Lists & Data
  state_search.go      # State structs: Search & Filtering
  state_loading.go     # State structs: Loading & Progress
  state_nav.go         # State structs: Dialogs, Tabs & Navigation
  state_feedback.go    # State structs: Visual Feedback
  state_realtime.go    # State structs: Real-Time & Multi-User
  data.go              # In-memory sample data + shared domain types (Contact, Item)
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
  patterns_test.go     # Chromedp E2E tests (in examples repo, not lvt repo)
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

Each handler function returns an `http.Handler` by creating a `livetemplate.Template` and calling `.Handle(controller, state)`. Each handler creates its own controller instance — there is no shared `Controller` singleton across patterns. Patterns that need shared state (e.g., real-time patterns #26–#28) define their own controller types with mutexes and shared data:

```go
func broadcastingHandler() http.Handler {
    ctrl := &BroadcastController{messages: []Message{}}
    state := &BroadcastState{Title: "Broadcasting"}
    tmpl := livetemplate.Must(livetemplate.New("broadcasting", opts...))
    return tmpl.Handle(ctrl, livetemplate.AsState(state))
}
```

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
            <td colspan="3">
                <form method="POST" class="inline">
                    <input type="hidden" name="id" value="{{.ID}}">
                    <fieldset role="group">
                        <input name="name" value="{{.Name}}">
                        <input name="email" value="{{.Email}}">
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
    // Populates .lvt errors for template rendering; error intentionally
    // discarded — Change() should not fail on inline validation.
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

**Key features:** Auto-reset on success (source: `client/state/form-lifecycle-manager.ts:67-69` — calls `form.reset()` unless `lvt-form:preserve` is set), explicit `lvt-el:reset:on:success`

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
    // Check both Tier 1 (standard multipart) and Tier 2 (chunked) uploads
    for _, name := range []string{"document", "chunked-doc"} {
        if ctx.HasUploads(name) {
            entries := ctx.GetCompletedUploads(name)
            if len(entries) > 0 {
                state.UploadName = entries[0].ClientName
                state.Uploaded = true
                return state, nil
            }
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
    <!-- lvt-upload handles chunked transfer via WebSocket — no enctype needed -->
    <form method="POST">
        <input type="file" lvt-upload="chunked-doc" name="chunked-doc">
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

```go
type PreserveInputsState struct {
    Title       string
    Name        string
    Description string
}

func (c *Controller) Submit(state PreserveInputsState, ctx *livetemplate.Context) (PreserveInputsState, error) {
    state.Name = ctx.GetString("name")
    state.Description = ctx.GetString("description")
    if err := ctx.ValidateForm(); err != nil {
        return state, err
    }
    return state, nil
}
```

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
    <small>{{.lvt.Error "name"}}</small>
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
    filtered := make([]Item, 0, len(state.Items))
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

**LiveTemplate (Tier 1):** The client has built-in IntersectionObserver support. A `<div lvt-scroll-sentinel>` at the end of the list triggers the `load_more` action automatically when it becomes visible. The framework routes `load_more` (snake_case) to the `LoadMore()` method.

**Also implemented in:** —

```go
type InfiniteScrollState struct {
    Title       string
    Items       []Item
    CurrentPage int
    HasMore     bool
}

// LoadMore handles the "load_more" action triggered by the scroll sentinel.
// The framework auto-routes snake_case → PascalCase (see dispatch.go:50-54,
// methodNameToActions converts LoadMore → ["loadMore", "load_more", "LoadMore"]).
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
    <div lvt-scroll-sentinel><small>Loading more...</small></div>
    {{end}}
</article>
{{end}}
```

**Key features:** `lvt-scroll-sentinel` attribute auto-triggers `load_more`; IntersectionObserver support is built into the client

> **Limitation:** The client detects the sentinel by `[lvt-scroll-sentinel]` attribute (`querySelector` selects the first match), so only one infinite scroll list can exist per wrapper. This is fine for the patterns app (one list per page).

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

func (c *Controller) Mount(state ValueSelectState, ctx *livetemplate.Context) (ValueSelectState, error) {
    state.Makes = c.getAllMakes() // e.g., ["Audi", "BMW", "Toyota"]
    return state, nil
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
    {{if and .Query (eq (len .Results) 0)}}<p><small>No results found.</small></p>{{end}}
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
    if ctx.Action() == "" { // Guard: only read query params on GET, not POST
        if status := ctx.GetString("status"); status != "" {
            state.Status = status
        }
        if sort := ctx.GetString("sort"); sort != "" {
            state.Sort = sort
        }
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
            <li><a href="?status=all&amp;sort={{.Sort}}">All</a></li>
            <li><a href="?status=active&amp;sort={{.Sort}}">Active</a></li>
            <li><a href="?status=completed&amp;sort={{.Sort}}">Completed</a></li>
        </ul>
        <ul>
            <li><a href="?status={{.Status}}&amp;sort=name">By Name</a></li>
            <li><a href="?status={{.Status}}&amp;sort=date">By Date</a></li>
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

**Key features:** `__navigate__` in-band navigation (same-pathname filter links use WebSocket, not HTTP fetch — v0.8.26+), `Mount()` reads query params, bookmarkable URLs

> **`__navigate__` transport:** Since all filter links are same-pathname query-param changes (`?status=active&sort=name`), client v0.8.26+ routes them through the WebSocket `__navigate__` action. The server re-runs `Mount()` with the new query params on the existing connection — the Go code works unchanged (same `ctx.GetString()`/`ctx.GetInt()` calls in `Mount()`), the only difference is the transport.

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
    if ctx.Action() == "" { // Guard: only on initial GET, not form POSTs
        state.Loading = true
    }
    return state, nil
}

// Note: TriggerAction errors are ignored here for simplicity.
// See Pattern #31 (Server Push) for the recommended cancellation
// pattern using TriggerAction error checking.
func (c *Controller) OnConnect(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
    if !state.Loading {
        return state, nil // Guard: skip on reconnect if data already loaded
    }
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

> **Note:** `Mount()` sets loading state on the initial HTTP response (pre-WebSocket). The goroutine in `OnConnect()` only fires after WebSocket connects. If the client has JS disabled, the loading spinner renders but the data never loads. This is expected — `TriggerAction()` requires an active WebSocket.

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

// Note: The Running guard is per-session state (prevents double-clicks in
// one tab). Multi-tab concurrent starts would require a controller-level
// mutex — omitted here for simplicity. See Pattern #31 for cancellation.
func (c *Controller) Start(state ProgressBarState, ctx *livetemplate.Context) (ProgressBarState, error) {
    if state.Running {
        return state, nil // already running in this session
    }
    state.Running = true
    state.Progress = 0
    state.Done = false
    session := ctx.Session()
    go func() {
        for i := 10; i <= 100; i += 10 {
            time.Sleep(500 * time.Millisecond)
            if err := session.TriggerAction("updateProgress", map[string]interface{}{
                "progress": i,
            }); err != nil {
                return // Session disconnected — stop the goroutine
            }
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

// Note: TriggerAction errors are ignored here for simplicity.
// See Pattern #31 (Server Push) for the recommended cancellation
// pattern using TriggerAction error checking.
func (c *Controller) Fetch(state AsyncState, ctx *livetemplate.Context) (AsyncState, error) {
    state.Status = "loading"
    state.Result = ""
    state.Error = ""
    session := ctx.Session()
    go func() {
        time.Sleep(2 * time.Second)
        // Simulate success or failure (~33% failure rate for demonstration)
        if rand.Intn(3) == 0 {
            _ = session.TriggerAction("fetchResult", map[string]interface{}{
                "error":   "Connection timed out",
                "success": false,
            })
        } else {
            _ = session.TriggerAction("fetchResult", map[string]interface{}{
                "result":  "Data fetched successfully at " + time.Now().Format("15:04:05"),
                "success": true,
            })
        }
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

**LiveTemplate (Tier 1):** Native `<dialog>` element with `command`/`commandfor` attributes (polyfilled for Firefox/Safari). Hash-driven deep linking (v0.8.30+) also allows opening the dialog via `<a href="#edit-dialog">`, browser Back/Forward, and direct URL sharing. Focus trapping, backdrop, and Escape key are all browser-native with `showModal()`.

**Also implemented in:** `todos/`

```go
type ModalState struct {
    Title string
    Name  string
}

func (c *Controller) Save(state ModalState, ctx *livetemplate.Context) (ModalState, error) {
    state.Name = ctx.GetString("name")
    return state, nil
}
```

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

**Key features:** Native `<dialog>`, `command`/`commandfor` (polyfilled), hash-driven deep linking (`<a href="#edit-dialog">` — v0.8.30+), auto-close on form success, dialog child updates survive morphdom (v0.8.33+)

> **Polyfill:** The Invoker Commands API polyfill is bundled in the LiveTemplate client library (source: `client/dom/invoker-polyfill.ts`). No additional `<script>` tag is needed — the client detects `commandForElement` support and activates the polyfill automatically for Firefox and Safari.

> **Hash-driven deep linking (v0.8.30+):** The dialog can also be opened via `<a href="#edit-dialog">` or by navigating directly to `/patterns/navigation/modal-dialog#edit-dialog`. The client activates `<dialog>`, `[popover]`, and `<details>` elements matching the URL hash, using `history.pushState` (not `location.hash`). Browser Back/Forward buttons close/reopen the dialog. This works alongside `command`/`commandfor` — both approaches are valid. Source: `client/dom/hash-link.ts`.

> **Dialog child updates (v0.8.33+):** Prior to v0.8.33, morphdom skipped the entire subtree of an open `<dialog>`. v0.8.33 allows child updates inside open dialogs, so form values and validation errors update in real-time while the dialog remains open. This means inline validation (Pattern #3 style) works correctly inside modal forms.

---

#### 18. Confirm Dialog

**htmx:** Uses `hx-confirm` or custom SweetAlert2 integration (inline JS) for confirmation before destructive actions.

**LiveTemplate (Tier 1):** Uses `<dialog>` with `command`/`commandfor` for a CSP-compliant confirmation flow. The destructive action form lives inside the dialog. No inline JavaScript.

**Also implemented in:** `todos/`

```go
type ConfirmDialogState struct {
    Title string
    Items []Item
}

func (c *Controller) Delete(state ConfirmDialogState, ctx *livetemplate.Context) (ConfirmDialogState, error) {
    id := ctx.GetString("id")
    filtered := make([]Item, 0, len(state.Items))
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

**Key features:** `command`/`commandfor` (no inline JS), `<dialog>` auto-close on success, CSP-compliant, hash-driven deep linking (v0.8.30+), dialog stays open during re-renders (v0.8.33+)

> **Per-item deep linking:** Since confirm dialogs are per-item (`id="confirm-{{.ID}}"`), the URL hash can directly link to a specific item's confirmation dialog (e.g., `#confirm-item3`). The dialog stays open during re-renders triggered by other actions (e.g., another user modifying the list via `BroadcastAction`), because v0.8.33 allows child updates inside open dialogs while preserving the dialog's open/top-layer state.

---

#### 19. Tabs (HATEOAS)

**htmx:** Uses `hx-get` with `hx-trigger="load"` and `hx-swap="innerHTML"` to load tab content server-side. Server returns full tab markup with selected state.

**LiveTemplate (Tier 1):** In-band `__navigate__` navigation via `<a href>` links. Since all tab links are same-pathname query-param changes (`?tab=tab1`), client v0.8.26+ routes them through the WebSocket `__navigate__` action instead of an HTTP fetch. The server re-runs `Mount()` with the new query params on the existing connection — zero HTTP round-trips for tab switches. No JavaScript tab management — the server decides what's active.

**Also implemented in:** —

```go
type TabsState struct {
    Title     string
    ActiveTab string
    Content   string
}

func (c *Controller) Mount(state TabsState, ctx *livetemplate.Context) (TabsState, error) {
    if ctx.Action() == "" { // Tab switches are GET navigations (SPA link interception),
        // so they correctly enter this branch. POST actions skip it.
        tab := ctx.GetString("tab")
        if tab == "" {
            tab = "tab1"
        }
        state.ActiveTab = tab
    }
    state.Content = c.getTabContent(state.ActiveTab) // returns plain text
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

**Key features:** `__navigate__` in-band navigation (zero HTTP round-trips), server-driven tab state via query params, `aria-current` for active tab

> **`__navigate__` mechanism:** The `__navigate__` action is a reserved framework action (defined in `action.go`). The client sends `{action: "__navigate__", data: {tab: "tab2"}}` over WebSocket. The server re-runs `Mount()` with `{tab: "tab2"}` as query parameters, and `ctx.Action()` returns `""` so the existing `if ctx.Action() == ""` guard works correctly. No controller method named `__navigate__` is required — the framework handles it directly in the event loop (`mount.go`). HTTP requests reject `__navigate__` with 400 (since HTTP inherently does a full request). The Go code above works unchanged — the only difference is the transport: WebSocket instead of HTTP fetch.

---

#### 20. SPA Navigation

**Source:** Phoenix LiveView (`push_patch` / `push_navigate`)

**LiveTemplate (Tier 1):** All `<a href>` links inside the LiveTemplate wrapper are auto-intercepted for SPA navigation. **Same-pathname links** (query-param-only changes) use the WebSocket `__navigate__` action for zero-HTTP-round-trip navigation (v0.8.26+). **Cross-pathname links** trigger a full WebSocket reconnect — the page is fetched via `fetch()`, DOM is patched, `pushState` updates the URL, and the WebSocket reconnects to the new handler. This cross-pathname reconnect is a v0.8.26 breaking change — previously, routes sharing the same `data-lvt-id` could do in-place swaps without reconnecting.

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

**Key features:** Auto link interception, `pushState`, cross-pathname reconnect (v0.8.26+), same-pathname `__navigate__` (v0.8.26+), `lvt-nav:no-intercept` for opt-out

---

#### 21. Keyboard Shortcuts

**htmx:** Uses `hx-trigger="keyup[altKey&&shiftKey&&key=='D'] from:body"` for global keyboard shortcuts.

**LiveTemplate (Tier 2):** `lvt-on:window:keydown` binds global keyboard events. `lvt-key` filters by specific key. No custom JavaScript.

**Also implemented in:** —

```go
type ShortcutsState struct {
    Title     string
    PanelOpen bool
}

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

```go
type AnimationItem struct {
    ID   string
    Name string
    Time string
}

type AnimationsState struct {
    Title string
    Items []AnimationItem
}

func (c *Controller) AddItem(state AnimationsState, ctx *livetemplate.Context) (AnimationsState, error) {
    state.Items = append(state.Items, AnimationItem{
        ID:   fmt.Sprintf("item-%d", len(state.Items)+1),
        Name: fmt.Sprintf("Item %d", len(state.Items)+1),
        Time: time.Now().Format("15:04:05"),
    })
    return state, nil
}
```

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
    <div lvt-fx:animate="fade">Fade In (default 300ms)</div>
    <div lvt-fx:animate="slide">Slide In</div>
    <div lvt-fx:animate="scale">Scale In</div>
</article>
{{end}}
```

**Key features:** `lvt-fx:animate="fade|slide|scale"`, `--lvt-animate-duration` CSS custom property

---

#### 23. Loading States

**htmx:** Uses the `htmx-request` CSS class to show loading indicators during requests.

**LiveTemplate (Tier 1+2):** Automatic behavior: forms get `aria-busy="true"` and fieldsets get `disabled` during submission. For custom UX, `lvt-form:disable-with` changes button text, and `lvt-el:*:on:pending` adds reactive DOM changes.

**Also implemented in:** —

```go
type LoadingStatesState struct {
    Title string
    Saved bool
}

func (c *Controller) SlowSave(state LoadingStatesState, ctx *livetemplate.Context) (LoadingStatesState, error) {
    time.Sleep(2 * time.Second) // Simulate slow operation to demo loading UI
    state.Saved = true
    return state, nil
}
```

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

```go
type HighlightState struct {
    Title   string
    Counter int
}

func (c *Controller) Increment(state HighlightState, ctx *livetemplate.Context) (HighlightState, error) {
    state.Counter++
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Highlight on Change</h3>
    <button name="increment">Increment</button>
    <div lvt-fx:highlight="flash">
        <p>Counter: {{.Counter}}</p>
    </div>

    <h4>Custom Highlight</h4>
    <div lvt-fx:highlight="flash">
        <p>Another counter display: {{.Counter}}</p>
    </div>
</article>
{{end}}
```

**Key features:** `lvt-fx:highlight="flash"`, `--lvt-highlight-color`, `--lvt-highlight-duration`

---

#### 25. Flash Messages / Toasts

**Source:** Phoenix LiveView (`put_flash` / temporary assigns)

**LiveTemplate (Tier 1):** `ctx.SetFlash()` stores messages that persist on WebSocket connections until explicitly cleared via `ctx.ClearFlash(key)` or auto-expired via `FlashExpiry(duration)`. On HTTP connections, flash is inherently one-shot (per-request). The `{{.lvt.FlashTag}}` template helper renders flash as `<output>` elements — `role="alert"` for the `"error"` key, `role="status"` for all others.

**Also implemented in:** `flash-messages/`, `todos/`

```go
type FlashState struct {
    Title string
}

func (c *Controller) Save(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
    name := ctx.GetString("name")
    if name == "" {
        ctx.ClearFlash("success") // clear opposing flash before setting new one
        ctx.SetFlash("error", "Name is required")
        return state, nil
    }
    ctx.ClearFlash("error") // clear opposing flash; auto-expire success after 5s
    ctx.SetFlash("success", "Saved: "+name, livetemplate.FlashExpiry(5*time.Second))
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Flash Messages</h3>
    <form method="POST">
        <label>Name
            <input name="name" placeholder="Enter name...">
        </label>
        <button name="save">Save</button>
    </form>
    {{.lvt.FlashTag "success"}}
    {{.lvt.FlashTag "error"}}
    <p><small>Try submitting empty for error, or with a name for success.
    Success auto-expires after 5s. Error persists until a successful save clears it.</small></p>
</article>
{{end}}
```

**Key features:** `ctx.SetFlash()` with `FlashExpiry` for auto-dismiss, `ctx.ClearFlash()` for explicit clearing, `{{.lvt.FlashTag}}` helper, persistent flash on WebSocket / one-shot on HTTP

> **Flash lifecycle (v0.8.19+):** This pattern demonstrates the three core flash operations: `SetFlash` (create), `ClearFlash` (remove), and `FlashExpiry` (auto-remove after duration). On WebSocket connections, flash persists across re-renders — if the handler doesn't clear the opposing category, both success and error flash can render simultaneously. On HTTP connections (no-JS fallback), flash is inherently one-shot because the per-request `connState` is GC'd after the response — `ClearFlash` still executes (it can affect the current response), but is typically unnecessary since flash doesn't persist across requests. The `FlashTag` helper renders `<output role="alert" data-flash="error">` for the `"error"` key and `<output role="status" data-flash="...">` for all other keys (including `"warning"`). E2E test selector is `output[data-flash]`.

---

### Category 7: Real-Time & Multi-User

#### 26. Multi-User Refresh

**Source:** Phoenix LiveView (PubSub broadcast + handle_info)

**LiveTemplate (Tier 1):** The mutating action explicitly broadcasts a peer refresh action to connections in the same session group. Multiple tabs or users see the same state changes in real time.

**Also implemented in:** `chat/`, `shared-notepad/`

```go
type RefreshController struct {
    mu      sync.Mutex
    counter int
}

type RefreshState struct {
    Title   string
    Counter int
}

// RefreshCounter is explicitly broadcast to peers after Increment. The state
// parameter is the peer's LOCAL state. The handler reads the shared counter
// from the controller (singleton) so all peers see the same value.
func (c *RefreshController) RefreshCounter(state RefreshState, ctx *livetemplate.Context) (RefreshState, error) {
    c.mu.Lock()
    state.Counter = c.counter
    c.mu.Unlock()
    return state, nil
}

func (c *RefreshController) Increment(state RefreshState, ctx *livetemplate.Context) (RefreshState, error) {
    c.mu.Lock()
    c.counter++
    state.Counter = c.counter
    c.mu.Unlock()
    ctx.BroadcastAction("RefreshCounter", nil)
    return state, nil
}
```

```html
{{define "content"}}
<article>
    <h3>Multi-User Refresh</h3>
    <p><small>Open this page in two tabs. Changes refresh peers explicitly.</small></p>
    <p>Counter: {{.Counter}}</p>
    <button name="increment">Increment</button>
</article>
{{end}}
```

**Key features:** `BroadcastAction`, session group peer refresh, multi-tab real-time updates

---

#### 27. Broadcasting

**Source:** Phoenix LiveView (`Phoenix.PubSub.broadcast`)

**LiveTemplate (Tier 1):** `ctx.BroadcastAction()` sends updates to all connections in a group, to a specific user, or globally.

**Also implemented in:** `chat/`

```go
func (c *Controller) SendMessage(state BroadcastState, ctx *livetemplate.Context) (BroadcastState, error) {
    msg := ctx.GetString("message")
    c.mu.Lock()
    c.msgID++
    c.messages = append(c.messages, Message{ID: c.msgID, Text: msg, User: state.Username})
    state.Messages = c.copyMessages() // Update sender's view immediately
    c.mu.Unlock()
    // Notify all other connections.
    // IMPORTANT: BroadcastAction must be called AFTER any ctx.With*() calls,
    // because With*() creates shallow copies and broadcasts queued before
    // the copy won't propagate. (See CLAUDE.md BroadcastAction ordering caveat.)
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

```html
{{define "content"}}
<article>
    <h3>Broadcasting</h3>
    <div class="messages">
        {{range .Messages}}
        <div class="message" data-key="{{.ID}}">
            <strong>{{.User}}:</strong> {{.Text}}
        </div>
        {{end}}
    </div>
    <form method="POST">
        <fieldset role="group">
            <input name="message" placeholder="Type a message..." required>
            <button name="sendMessage">Send</button>
        </fieldset>
    </form>
</article>
{{end}}
```

**Key features:** `ctx.BroadcastAction()`, shared controller state with mutex, cross-connection updates

---

#### 28. Presence Tracking

**Source:** Phoenix Presence (CRDTs for distributed presence)

**LiveTemplate (Tier 1):** Users explicitly join/leave. `OnConnect()` tracks connection state. The controller maintains a user map with mutex protection. `BroadcastAction()` notifies all connections of presence changes.

**Also implemented in:** `chat/`

```go
type PresenceController struct {
    mu          sync.RWMutex
    onlineUsers map[string]bool
}

type PresenceState struct {
    Title       string
    Username    string
    OnlineCount int
    Joined      bool
}

func (c *PresenceController) Join(state PresenceState, ctx *livetemplate.Context) (PresenceState, error) {
    username := ctx.GetString("username")
    if username == "" {
        return state, nil
    }
    c.mu.Lock()
    state.Username = username
    state.Joined = true
    c.onlineUsers[username] = true
    state.OnlineCount = len(c.onlineUsers)
    c.mu.Unlock()
    ctx.BroadcastAction("PresenceChanged", nil)
    return state, nil
}

func (c *PresenceController) Leave(state PresenceState, ctx *livetemplate.Context) (PresenceState, error) {
    if state.Username == "" {
        return state, nil
    }
    c.mu.Lock()
    delete(c.onlineUsers, state.Username)
    state.Username = ""
    state.Joined = false
    state.OnlineCount = len(c.onlineUsers)
    c.mu.Unlock()
    ctx.BroadcastAction("PresenceChanged", nil)
    return state, nil
}

func (c *PresenceController) PresenceChanged(state PresenceState, ctx *livetemplate.Context) (PresenceState, error) {
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
    {{if .Joined}}
    <p>Logged in as: {{.Username}}</p>
    <button name="leave" class="secondary">Leave</button>
    {{else}}
    <form method="POST">
        <fieldset role="group">
            <input name="username" placeholder="Enter username..." required>
            <button name="join">Join</button>
        </fieldset>
    </form>
    {{end}}
</article>
{{end}}
```

**Key features:** Explicit join/leave actions, `BroadcastAction()` for presence updates, shared controller state with mutex

> **Disconnect cleanup limitation:** `OnDisconnect()` receives only the controller receiver (no state or context), so it cannot identify which user disconnected. Users who close tabs without clicking Leave remain in `onlineUsers`. This is the same pattern used in the `chat/` example. To implement automatic cleanup, a connection→username map would be needed, but the framework does not currently expose a connection ID in `OnConnect()`. Two workarounds:
>
> 1. **Application-level heartbeat:** A periodic goroutine that prunes users who haven't sent an action within N seconds.
> 2. **Framework enhancement:** A future `ctx.ConnectionID()` API in `OnConnect()` would allow tracking connections, with `OnDisconnect()` receiving the ID for cleanup.
>
> For this example, explicit Join/Leave is sufficient. The limitation should be noted in the implementation.

---

#### 29. Reconnection Recovery

**Source:** Phoenix LiveView (automatic reconnection with state recovery)

**LiveTemplate (Tier 1):** State fields tagged with `lvt:"persist"` survive disconnection and reconnection. The client auto-reconnects and the framework restores persisted state. This is an existing framework feature used across 13 examples (`counter/`, `todos/`, `chat/`, `login/`, `shared-notepad/`, `flash-messages/`, `progressive-enhancement/`, `ws-disabled/`, `avatar-upload/`, `live-preview/`).

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

> **Why both `lvt:"persist"` and `lvt-form:preserve`?** They serve different purposes: `lvt:"persist"` (server-side) survives WebSocket reconnects by restoring state fields from the session store. `lvt-form:preserve` (client-side) retains unsaved input values in the DOM across re-renders triggered by other state changes. Using both on the Notes field means: the last-saved value survives reconnects, AND in-progress edits survive re-renders from other actions.

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
        state.Preview = "Hello, " + state.Input + "!" // plain text preview
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
// Goroutine cancellation pattern: exit early if TriggerAction returns an
// error (session disconnected). The framework does not currently expose a
// done channel, so checking TriggerAction errors is the recommended approach.
func (c *Controller) StartTimer(state PushState, ctx *livetemplate.Context) (PushState, error) {
    if state.Running {
        return state, nil // Guard: prevent concurrent goroutines
    }
    state.Running = true
    session := ctx.Session()
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        for i := 0; i < 10; i++ {
            <-ticker.C
            if err := session.TriggerAction("tick", map[string]interface{}{
                "elapsed": i + 1,
            }); err != nil {
                return // Session disconnected — stop the goroutine
            }
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

Future features identified during this proposal should be filed as GitHub issues for tracking. Create the following issue when implementation begins:

### Drag-and-Drop / Sortable — File as GitHub issue

**Title:** `feat: add drag-and-drop event support (lvt-on:drag*)`
**Repo:** `livetemplate/client` (primary), `livetemplate/livetemplate` (secondary)
**Labels:** `enhancement`

**Requirements:**
- Client: Add `lvt-on:dragstart`, `lvt-on:dragover`, `lvt-on:drop` event support
- Client: Serialize drag source/target data in action message
- Core: Reorder protocol for range items (extend existing range operations)

**Not blocking for the patterns example.** The Sortable pattern is skipped until this issue is implemented.

---

## Implementation Plan

**Session workflow:** Each session ends by: (1) updating the index page with links to the newly implemented patterns, (2) running the app locally for manual review — wait for signoff on UI and code before proceeding, (3) creating a PR for the work done, (4) updating this tracker (check off completed items), and (5) pushing the updated proposal. The next session picks up from the tracker state.

### Implementation Notes (accumulated from completed sessions)

Concrete, non-obvious patterns validated during earlier sessions. Apply these directly — they are the bits that took iterations to discover. Items already covered in [`examples/CLAUDE.md`](https://github.com/livetemplate/examples/blob/main/CLAUDE.md) (Tier 1/2 table, E2E test rules, Pico/CSP boilerplate) and [`livetemplate/CLAUDE.md`](../../CLAUDE.md) (controller pattern, `data-key`) are **not** repeated here.

**Template helpers (always prefer over manual patterns):**

- `{{.lvt.FlashTag "success"}}` / `{{.lvt.FlashTag "error"}}` — renders `<output role="status" data-flash="success">…</output>` (errors use `role="alert"`). **E2E selector is `output[data-flash]`, NOT `<ins>`/`<del>`.**
- `{{.lvt.ErrorTag "fieldname"}}` — renders the field-scoped error message (uses `<small>`).
- `{{.lvt.AriaInvalid "fieldname"}}` — emits `aria-invalid="true"` when the field has an error. Use with `required` / `pattern` inputs.
- `<ins>`/`<del>` with inline `style="display:block;text-decoration:none"` is a **separate** manual block-alert pattern used only when you need a non-flash inline status (e.g., the `inline-validation.tmpl` "Saved!" indicator) — don't confuse it with `FlashTag`.
- `ctx.ValidateForm()` is the validation primitive — call it in `Change()` for live feedback and in `Submit()` for gate-keeping. Returned errors populate `ErrorTag` / `AriaInvalid` automatically.

**State struct shape (layout depends on these):**

- Every pattern's state struct MUST have `Title string` and `Category string` as the first two fields. `layout.tmpl` renders `<title>{{.Title}} — LiveTemplate Patterns</title>` and shows `{{.Category}}` in the breadcrumb.
- Set `Title` to the bare pattern name ("Delete Row"), NOT "Delete Row — Patterns" and NOT "LiveTemplate Patterns" — the layout already appends ` — LiveTemplate Patterns`, so a redundant suffix produces `"Delete Row — LiveTemplate Patterns — LiveTemplate Patterns"` (filed as [`livetemplate/examples#62`](https://github.com/livetemplate/examples/issues/62)).

**File layout convention:**

- One `state_{category}.go` + `handlers_{category}.go` file per category. Session 2 should create `state_lists.go` / `handlers_lists.go` / `state_search.go` / `handlers_search.go` — NOT a single monolithic file.
- Shared sample data goes in `data.go`. Add `sampleItems()` etc. alongside existing `sampleContacts()` / `sampleUsers()`.

**Index page is data-driven:** `data.go :: allPatterns()` already declares all 31 patterns with `Implemented: false` for unimplemented entries. To "add" a pattern to the index, just flip `Implemented: true` on its `PatternLink` — the index template iterates `allPatterns()` automatically. **No `index.tmpl` edits needed per pattern.**

**Accessibility non-negotiables (Copilot will flag these in review):**

- Every input needs a visible `<label>` or explicit `aria-label`. Placeholder-only is a failure (see [`livetemplate/examples#63`](https://github.com/livetemplate/examples/issues/63), [`#64`](https://github.com/livetemplate/examples/issues/64), [`#65`](https://github.com/livetemplate/examples/issues/65)).
- Every action button must be inside a `<form method="POST">` so Tier 1 no-JS fallback can route to the controller method. A bare `<button>` outside a form will be flagged (see [`livetemplate/examples#66`](https://github.com/livetemplate/examples/issues/66) for the Session 1 `click-to-edit.tmpl` miss).
- Every range item in list patterns should use explicit `data-key="{{.ID}}"` for stable identity across delete/reorder operations.

**E2E test helpers already built in `patterns_test.go` (reuse them):**

- `setupTest(t)` — shared Chrome + server fixture returning `(ctx, cancel, serverPort)`. Use this for all new subtests; do NOT duplicate Chrome startup boilerplate.
- `TestMain` is already wired with `e2etest.CleanupChromeContainers()` before and after. **Do NOT create a second `TestMain`.**
- `attachFileViaDataTransfer(selector, filename, content, mimeType)` — the only reliable file-upload helper for Docker Chrome (`chromedp.SetUploadFiles` doesn't work there).
- `runUIStandards` vs `runUIStandardsWithPico` — use `WithPico` for pages with `<fieldset role="group">` inline forms; plain for vertical labeled forms. The choice is non-obvious and is made per-pattern.
- `cross_handler_nav_test.go` — smoke suite for cross-handler SPA navigation. Run it whenever navigation-adjacent code changes.

**E2E waits:**

- `e2etest.WaitFor(jsPredicate, timeout)` is legitimate when `jsPredicate` is a real DOM/JS check. **Never pass a constant-true expression** like `` e2etest.WaitFor(`true`, …) `` — that is a disguised sleep and will be flagged (see [`livetemplate/examples#67`](https://github.com/livetemplate/examples/issues/67), [`#68`](https://github.com/livetemplate/examples/issues/68)).
- Prefer `WaitForText(selector, text, timeout)` / `WaitForCount(selector, n, timeout)` / `WaitForWebSocketReady(timeout)` when applicable — they encode the intent clearly.
- `chromedp.Sleep` is always wrong.

**Visual_Check subtests:**

- Each pattern includes a `Visual_Check` subtest using `e2etest.ValidateScreenshotWithLLM(t, ctx, "description of expected layout")`. These are **skipped unless `LVT_VISUAL_CHECK=true`** is set.
- The description string is load-bearing — the LLM uses it to judge layout drift. Write it at pattern-implementation time, not after.

**File uploads (Session 1 scope, but worth noting for later sessions):**

- Tier 1 multipart: must register with `livetemplate.WithUpload("fieldname", livetemplate.UploadConfig{MaxFileSize: 10 << 20, MaxEntries: 1})` in the handler — without this, multipart parsing silently fails and `ctx.HasUploads` always returns false. (Always use named fields: `UploadConfig` has `Accept []string` as its first field, so positional init would not compile.)
- Tier 2 chunked (`lvt-upload` attribute): use a small `ChunkSize: 1024` to make progress visible in demos (the default chunk size completes before the progress bar renders for small files).
- Read entries: `if ctx.HasUploads(name) { entries := ctx.GetCompletedUploads(name); ... }`.

**Cross-handler SPA navigation:**

- Each pattern is its own handler with its own `data-lvt-id`. Client **v0.8.23+** handles WebSocket disconnect/reconnect AND per-handler state reset transparently — no workarounds needed. Earlier versions left `treeRenderer` state from the previous handler, causing visible cross-contamination ("index page content bleeds into the next pattern").
- Use `@latest` CDN in templates (per project convention); do not pin a specific client version. This is an intentional tradeoff — the examples always demonstrate the current client, accepting the risk that a client release could break a demo until fixed. If a demo breaks after a client release, fix the demo (or the client), don't pin the version.
- `TestCrossHandlerNavigation` must assert **absence of stale content**, not just presence of new content. Positive assertions alone miss the cross-contamination class of bugs. Add row-count / `<article>` count / `<h4>` count assertions after each cross-handler navigation.

**Framework version requirements (both repos):**

- **Client v0.8.23+** is required for Session 2+. Prior versions contain three bugs that manifest in patterns: (1) `WebSocketManager.connect()` doesn't await `onopen`, so observers fire during CONNECTING and fall back to HTTP — producing duplicate rows on Infinite Scroll; (2) `loadMorePending` throttle was missing entirely — rapid observer re-fires stacked concurrent actions; (3) cross-handler navigation didn't reset `treeRenderer` — stale state bled between handlers. All three are fixed in v0.8.23. If tests fail with duplicate `data-key` entries after pagination, the client version is too old.
- **Library v0.8.18+** is required for Session 3+ (any pattern using `session.TriggerAction` from a goroutine). Prior versions had NO concrete `Session` implementation wired into `Mount`/lifecycle contexts — `ctx.Session()` returned `nil` because `WithSession(...)` was never called at any lifecycle call site, leaving the context's session field as a nil interface value. Goroutine pushes invoking the nil session silently no-op'd or panicked at the interface-dispatch site, depending on the call path. The fix (`livetemplate/livetemplate#336`) added a concrete `localSession` type and wired it through `WithSession(newLocalSession(...))` at every relevant Mount / OnConnect / action-dispatch / broadcast-dispatch call site in `mount.go` — search for `WithSession(newLocalSession` to see the current set. If you're writing a goroutine-push pattern and it doesn't appear to push anything, verify `go.mod` is at v0.8.18+ BEFORE debugging the controller. Same release also fixed `ctx.GetInt`/`ctx.GetFloat` to handle native Go numeric types (int, int8-64, uint8-64, float32/64) with NaN/Inf/overflow checks — prior versions silently returned 0 for a goroutine passing a Go `int` via `TriggerAction`. The delegation chain is `ctx.GetInt()` (in `context.go`, a thin wrapper) → `ActionData.GetIntOk()` (in `action.go`, the actual type switch); verify the numeric-type switch is present in your version with `grep -n "int8\|int16\|int32\|int64" action.go` (it should match the type-switch branches in `GetIntOk`/`GetFloatOk`).
- **Client v0.8.33+** is required for Session 4+. Key features accumulated across v0.8.23→v0.8.33: (1) Invoker Commands polyfill for `command`/`commandfor` on Firefox/Safari (v0.8.21); (2) `__navigate__` in-band SPA navigation — same-pathname link clicks use WebSocket instead of HTTP fetch (v0.8.26); (3) `lvt-ignore`/`lvt-ignore-attrs` morphdom escape hatches (v0.8.26); (4) **cross-pathname navigation always reconnects** — BREAKING change, previously shared `data-lvt-id` allowed in-place swap (v0.8.26); (5) multiple same-name checkboxes send `[value1, value2]` array instead of boolean (v0.8.28); (6) checkbox/radio checked state preserved across morphdom updates (v0.8.29); (7) hash-driven deep linking — URL fragments activate `<dialog>`, `[popover]`, `<details>` elements (v0.8.30); (8) datalist preservation while input focused (v0.8.31); (9) WebSocket resilience — proper promise settlement, handler detach on disconnect (v0.8.32); (10) `lvt-scroll-sentinel` attribute replaces `id="scroll-sentinel"` (v0.8.33); (11) dialog child updates inside open dialogs — morphdom no longer skips entire subtree (v0.8.33); (12) `lvt-scroll-away` for scroll-to-top buttons (v0.8.33). If Session 4 tests fail with dialogs not updating while open, same-pathname links falling back to HTTP fetch, or `command`/`commandfor` not working in Firefox, the client version is too old.
- **Library v0.8.19+** is required for Session 5+ (any pattern using `ctx.SetFlash` with `FlashExpiry` or `ClearFlash`). The flash message lifecycle was overhauled: flash on WebSocket connections now **persists across re-renders** until explicitly cleared via `ctx.ClearFlash(key)` or auto-expired via `ctx.SetFlash(key, msg, livetemplate.FlashExpiry(duration))`. HTTP connections remain one-shot (per-request). The `__navigate__` reserved action server-side handling is also in this release but is transparent to controllers — `Mount()` receives the data as query params without any controller code changes. If flash messages persist unexpectedly or `ClearFlash` is not recognized, verify `go.mod` is at v0.8.19+ BEFORE debugging the controller. Verify with: `grep -n "FlashExpiry\|ClearFlash" context.go` (should show both public methods).

**Server-push patterns (goroutine → `session.TriggerAction`):** Session 3 established the canonical shape for patterns that run a background goroutine and push updates via the WebSocket. Apply these rules verbatim when writing any new goroutine-pushing pattern.

- **Action handler shape: re-entrancy guard → session nil-check → mutate → spawn.** The ordering is load-bearing. This example matches Session 3's `ProgressBarController.Start` — intentionally bounded by a **hard iteration count** (`i < maxTicks`) so it is safe in both single-instance and multi-instance deployments. Why the hard count matters: in multi-instance mode `TriggerAction` never returns an error (see the caveat below), so the `if err != nil { return }` inside the loop body cannot be relied on as the exit condition. The `i < maxTicks` ceiling is what actually guarantees termination. A done channel (closed on `OnDisconnect`) is the other acceptable pattern for indefinite-work loops. Either way: the exit condition must be something the loop body controls directly, not something that depends on `TriggerAction`'s return value.
  ```go
  func (c *Controller) Start(state State, ctx *Context) (State, error) {
      // 1. Re-entrancy guard: direct WS messages can bypass a template-disabled button.
      if state.Running { return state, nil }
      // 2. Session check BEFORE mutation: if session is nil, mutation would leave
      //    Running=true with no goroutine to clear it, and the guard above would
      //    permanently block recovery.
      session := ctx.Session()
      if session == nil { return state, nil }
      // 3. Mutate.
      state.Running = true
      // 4. Spawn with a BOUNDED loop (critical — see multi-instance caveat below).
      //    Unbounded `for { ... }` loops using only `err != nil` as the exit
      //    condition will silently run forever under PubSub, because
      //    TriggerAction returns nil when a broadcaster is configured. Always
      //    bound the iteration count, OR use a done channel — never rely on
      //    the error return as the sole stop signal in an unbounded loop.
      go func() {
          for i := 0; i < maxTicks; i++ {
              time.Sleep(tickInterval)
              if err := session.TriggerAction("update", data); err != nil {
                  return // Session disconnected — stop cleanly (single-instance).
              }
          }
      }()
      return state, nil
  }
  ```
  The re-entrancy guard is NOT optional even when the template hides the action button — `liveTemplateClient.send({action:"start"})` via a direct WebSocket message bypasses the rendered UI, and the `Concurrent_Fetch_Reaches_Single_Result` test in `patterns_test.go` proves this matters.

- **Canonical goroutine cancellation (single-instance mode):** `if err := session.TriggerAction(...); err != nil { return }` in every loop. In single-instance mode (no PubSub broadcaster configured), `TriggerAction` returns an error with the prefix `livetemplate: no connected sessions for group` when `registry.GetByGroup(groupID)` finds no connections and `!hasRemote` — that's how the goroutine learns the WebSocket is gone and exits cleanly.

- **Multi-instance caveat — indefinite-work loops MUST use a hard bound or done channel, not the error-return pattern.** `TriggerAction` only returns the no-connections error when there is no `GroupActionBroadcaster` configured (trace with `grep -n "hasRemote" session_impl.go`). If a broadcaster is configured (e.g., Redis PubSub), `TriggerAction` returns `nil` even with zero local connections, because the user may be connected to a peer instance. An **unbounded** `err != nil` loop will therefore loop forever in any multi-instance deployment — and worse, under a persistent PubSub outage it will loop forever while emitting "pubsub publish failed" warnings on every call. The full lifecycle contract (including the bounded-loop workaround) lives in the doc comment preceding the function definition; find it with `grep -n "Disconnect semantics" session_impl.go`.
  
  **Session 3's three patterns are all bounded by design** and do not exhibit this bug: LazyLoad uses a single 2s sleep, ProgressBar's loop iterates from `progressStep` to `100` in `progressStep`-sized increments (the constants live in the **examples repo** at `patterns/handlers_loading.go` — grep `progressStep` there to verify the current bound, which is not visible from this repo), AsyncOps uses a single 2s sleep. The warning below is for **future** patterns that need to work in both single-instance and multi-instance modes and would otherwise copy the `err != nil` pattern into an unbounded loop without realizing it. Future unbounded-loop patterns MUST use one of:
  - A **hard iteration bound**: `for i := 0; i < 100; i++ { ... }` — matches the ProgressBar shape. Safe for finite work.
  - A **done channel** or **external context**: track a `stopCh chan struct{}` in the controller struct, close it on `OnDisconnect`, and `select { case <-stopCh: return; case <-time.After(tick): }` in the loop. Safe for indefinite work.
  
  Do NOT reach for `context.Context` from `r.Context()` (the HTTP request context) — as of `livetemplate v0.8.18`, the framework does not thread a cancellable request context through `Session` (see `livetemplate/livetemplate#303` for the open tracking issue). A **controller-stored** `context.Context` or `done chan struct{}`, created in the action handler and closed in `OnDisconnect`, IS the correct pattern for indefinite-work goroutines. The doc comment preceding `localSession.TriggerAction` (`grep -n "Disconnect semantics" session_impl.go`) shows the canonical shape with a `select { case <-ctx.Done(): ... }` loop — note that `ctx` in that example is controller-stored, not `r.Context()`. Copy the loop structure, create your own context in the action handler.

- **Ephemeral state = natural self-healing on reconnect.** State structs WITHOUT `lvt:"persist"` tags are freshly cloned on every WebSocket connect. Trace the path from the `livetemplate/` repo root with `grep -n "restorePersistedState\|cloneStateTyped" mount.go`: `restorePersistedState` returns `(nil, false)` when `persistable == nil`, and the call site falls through to `cloneStateTyped()` which returns zero-value state. Concretely:
  1. WebSocket disconnects mid-goroutine → `registry.Unregister` removes the connection.
  2. Goroutine wakes → `TriggerAction` → `GetByGroup` returns empty → error → goroutine exits cleanly.
  3. WebSocket reconnects → `Mount` fires → state is zero-valued (`Running=false`, `Status=""`) → user sees the clickable button.
  
  **Consequence:** patterns like `ProgressBar` and `AsyncOps` do NOT need an `OnConnect` recovery path — the "stuck state after reconnect" scenario cannot manifest in ephemeral mode. Document this with an in-code comment so review bots don't flag the absence of `OnConnect` as a bug. The comment should explicitly cite the `cloneStateTyped()` path so the reasoning is legible.

- **Patterns that DO need `OnConnect`:** Only patterns where the initial render is the "pre-goroutine" state (like `LazyLoad`'s spinner) need an `OnConnect` recovery path — otherwise the user reconnects to a spinner with nothing driving it. The lifecycle shape is: the framework calls `Mount` first on every WebSocket connect/reconnect, then passes the returned state to `OnConnect`.

  **`Mount`:**
  ```go
  func (c *LazyLoadController) Mount(state State, ctx *Context) (State, error) {
      if ctx.Action() == "" { // Only reset on non-action calls (initial GET or WS connect);
                              // skip when a POST action is being dispatched, so the action's
                              // own state changes aren't clobbered mid-flight.
          state.Loading = true
          state.Data = ""
      }
      return state, nil
  }
  ```

  **`OnConnect`:**
  ```go
  func (c *LazyLoadController) OnConnect(state State, ctx *Context) (State, error) {
      // Load-bearing safety net. Today, Mount ALWAYS sets Loading=true on
      // a WS (re)connect before OnConnect runs, so this check doesn't fire
      // with the current Mount shape. But if Mount ever gains a condition
      // that preserves already-completed state (e.g., `if state.Data == ""`
      // added to Mount's reset condition — a reasonable future change to
      // avoid re-fetching already-loaded data on reconnect), this guard
      // becomes the active safety net: reconnecting after completion would
      // arrive here with Loading=false, and without the check, OnConnect
      // would re-spawn the goroutine and overwrite already-loaded data.
      // Do NOT remove when refactoring Mount.
      if !state.Loading { return state, nil }
      session := ctx.Session()
      if session == nil { return state, nil }
      // Reconnect race: if the client disconnects and reconnects WHILE a
      // goroutine from the previous connect is still sleeping, OnConnect
      // will spawn a second goroutine and both will eventually race to
      // dispatch `dataLoaded`. That's safe ONLY because DataLoaded is
      // idempotent (it just overwrites state.Data). Non-idempotent
      // patterns — anything that increments a counter, appends to a
      // list, or triggers a side effect — must use an in-flight request
      // ID guard instead. See the "Reconnect-during-loading double-fire"
      // section below for the full analysis.
      go func() {
          time.Sleep(lazyLoadDelay)
          if err := session.TriggerAction("dataLoaded", map[string]any{
              "data": "Content loaded lazily at " + time.Now().Format("15:04:05"),
          }); err != nil {
              return // Session disconnected — stop cleanly (single-instance).
          }
      }()
      return state, nil
  }
  ```

- **Reconnect-during-loading double-fire is a real race, and the framework does NOT invalidate old sessions.** If the client disconnects + reconnects within a goroutine's sleep window, `OnConnect` spawns a second goroutine while the first is still asleep. Both goroutines look up the session via `registry.GetByGroup(groupID)`, and **`groupID` is stable across reconnects** (it's cookie-bound). Outcome depends on timing:
  - (a) Goroutine wakes during the dead-connection gap → `GetByGroup` empty → error → exits cleanly.
  - (b) Goroutine wakes after reconnect → both goroutines dispatch successfully to the new connection. For `LazyLoad`'s `DataLoaded`, the second call overwrites `state.Data` with a slightly different timestamp — harmless.
  
  **Every goroutine-push pattern in the examples repo MUST be idempotent by construction** — result handlers must produce the same final state regardless of whether the dispatch arrives once or twice. For Session 3, this is true by structure: `DataLoaded` just sets a string, `UpdateProgress` writes a monotonic value, `FetchResult` is a terminal transition. A future non-idempotent pattern (e.g., "increment a counter on each tick") would need an explicit in-flight request ID tracked in state — but do not write such a pattern without first filing an issue against the library's `TriggerAction` reconnect-gap work (see `livetemplate/livetemplate#342`). **Do NOT claim "framework session invalidation" semantics in comments** — that is not a thing. The groupID lookup is deterministic, and an earlier version of this proposal's example comments made that false claim before a review caught it.

- **Flash scoping in branched templates is load-bearing.** When a controller emits a flash only on a state transition (e.g., `UpdateProgress` calls `ctx.SetFlash("success", "Job complete")` only when `Progress >= 100`), the `{{.lvt.FlashTag "success"}}` MUST live inside the branch that renders after that transition — not at the always-rendered top of the article. Under the persistent flash model (v0.8.19+), flash on WebSocket connections persists until explicitly cleared via `ClearFlash` or auto-expired via `FlashExpiry`; placing the tag in an always-rendered location causes the flash to appear prematurely and **persist visibly** across subsequent renders of the wrong branch. For transient feedback inside branched templates, use `FlashExpiry` to auto-dismiss or `ClearFlash` for mutually exclusive categories, and place the tag in the correct branch so it only renders when relevant. Comment the placement inline so a future maintainer doesn't "simplify" the tag out of its scoped position.

**In-memory shared DB for mutable state:** Patterns like Delete Row that need server-side persistence across sessions (but NOT across process restarts) should use a `sync.Mutex + []T` in the **controller struct** — NOT `lvt:"persist"` tags. Controllers are singletons, so this is safe. Pattern:
```go
type DeleteRowController struct {
    mu    sync.Mutex
    items []Item
}
func (c *DeleteRowController) snapshot() []Item {
    c.mu.Lock(); defer c.mu.Unlock()
    return slices.Clone(c.items)
}
func (c *DeleteRowController) Mount(state DeleteRowState, ctx *Context) (DeleteRowState, error) {
    c.mu.Lock(); defer c.mu.Unlock()
    if c.items == nil { c.items = sampleListItems(5) }
    state.Items = slices.Clone(c.items)
    return state, nil
}
```
State is pure data (`AssertPureState` still passes) and gets repopulated on every Mount from the controller's DB. This avoids the complexity of selective persistence and is the right pattern for demo-scale state.

**Tier 1 row-scoped actions — button `value` attribute, not hidden inputs:** For actions like Delete/Edit/Save that need an ID, use `<button name="delete" value="{{.ID}}">` and read `ctx.GetString("value")`. Don't use `<input type="hidden" name="id">` + `<button name="delete">`. Shorter template, idiomatic HTML, no hidden inputs to forget. This is the documented Tier 1 pattern in `docs/references/progressive-complexity-reference.md`. Session 1's Edit Row used hidden inputs because Edit needs multiple fields (name + email + ID), which is the only legit use case.

**Range-removal animation:** Use `lvt-fx:animate="slide"` (not `"fade"`) for row delete animations — slide is visibly distinctive. Default duration is 500ms in client v0.8.23+; override with `style="--lvt-animate-duration: 800ms"` if needed. The client treats `animatedElements` as a once-per-lifetime WeakSet, so re-renders of the same DOM node skip the animation (morphdom creates fresh nodes for new range items, which do animate).

**Infinite Scroll dataset sizing:** With `listPageSize = 10`, use **25+ items** in the dataset. 25 produces 3 pages (10 + 10 + 5), which is enough to demonstrate auto-advance through multiple pages. Anything less than ~25 and the infinite-scroll effect isn't visible.

**Sentinel must render only when `.HasMore`:** Always-rendering `<div lvt-scroll-sentinel>` causes an **infinite empty-load loop** after the last page — the sentinel stays visible, the observer fires, the server returns an empty page, rinse/repeat. Wrap it in `{{if .HasMore}}...{{end}}`.

**Headless Chrome test limitations (and their fixes):**

- `IntersectionObserver` doesn't fire in headless mode (no compositing). **Fix:** use `chromedp.Evaluate("window.liveTemplateClient.send({action:'load_more'})", nil)` as the trigger for `TestInfiniteScroll`. Add a comment explaining why. `examples/CLAUDE.md` carves out this case explicitly.
- `chromedp.Click` on a `<select>` doesn't open the native dropdown. **Fix:** `chromedp.SetValue(selector, value)` + dispatch a synthetic `change` event via `chromedp.Evaluate`. Extract this into a test helper (`selectValueAndDispatchChange`) and reuse.
- `chromedp.Click` doesn't reliably trigger event-delegation handlers. **Fix:** `document.querySelector(...).click()` via `chromedp.Evaluate`.

**`Mount()` reading URL query params:** `Mount` runs on POST too, so guard URL reads with `if ctx.Action() == ""`. Also **validate** query param values against an allowed set — unknown values should silently fall back to the current state field (not 404, not flash error). Bookmarks with stale values shouldn't crash. Always call the filter function OUTSIDE the guard so initial render AND POSTs both populate the table:
```go
func (c *URLFiltersController) Mount(state URLFiltersState, ctx *Context) (URLFiltersState, error) {
    if ctx.Action() == "" {
        if s := ctx.QueryString("status"); validStatuses[s] { state.Status = s }
        if s := ctx.QueryString("sort"); validSorts[s] { state.Sort = s }
    }
    state.Items = filterItems(state.Status, state.Sort) // always
    return state, nil
}
```

**`Change()` on `<select>` and `<input type="search">`:** Auto-wired via the client with a 300ms debounce. Tests must use `WaitForCount` / `WaitForText` — **NEVER `chromedp.Sleep`**. The wait functions naturally outlast the debounce.

**Flash tag category coverage:** If a controller emits `FlashSuccess` OR `FlashInfo` OR `FlashError`, the template MUST render `FlashTag` for EVERY category the controller can emit. A missing `{{.lvt.FlashTag "info"}}` means the info flash is silently dropped and tests waiting for its text time out. Audit at template-write time, not after tests fail.

**Counting actual changes in bulk-update flows:** `BulkUpdate` should track a `changed := 0` counter and emit `"Updated N users"` (N = actual changes). When `changed == 0`, emit `"No changes"` as an `info` flash. Using `len(state.Users)` as the count is wrong — it reports "Updated 4 users" even when nothing changed. Under the persistent flash model (v0.8.19+), use `FlashExpiry` for both categories so flashes auto-dismiss:
```go
ctx.SetFlash("success", fmt.Sprintf("Updated %d users", changed), livetemplate.FlashExpiry(5*time.Second))
// or when nothing changed:
ctx.SetFlash("info", "No changes", livetemplate.FlashExpiry(5*time.Second))
```

**Parallel sample data for Session-pinned tests:** Session 1's `TestEditRow` pins on `sampleContacts()` returning exactly 4 entries. For Session 2's `TestActiveSearch` that needs 25 contacts, add a **parallel** `sampleContactDirectory()` — do NOT extend `sampleContacts()`, and do NOT modify its count. This preserves Session 1 test stability while letting Session 2 use a bigger dataset.

**CSS `output[data-flash]` padding:** `FlashTag` renders `<output role="status" data-flash="success">`. Pico defaults `<output>` to inline with no spacing, which collides with preceding form controls. Add this to `livetemplate.css` (or override per-pattern):
```css
output[data-flash] {
  display: block;
  margin-top: 1rem;
  padding: 0.5rem 0;
}
```

**Cross-handler navigation smoke test:** `cross_handler_nav_test.go` should include a regression test for every Session 1 + Session 2 pattern discovered to have cross-contamination bugs (currently: `Index_To_Delete_Row_No_Stale_Dom`). Add new subtests there when a new cross-handler desync is found.

**Local dev loop:**

- Run a specific pattern's tests: `GOWORK=off go test -v -race -timeout=10m ./patterns -run TestPatternName`
- Run against a locally-built client: `LVT_LOCAL_CLIENT=/abs/path/to/client/dist/livetemplate-client.browser.js GOWORK=off go test -v ./patterns`
- Run the app manually (Tier 1 fallback works even without JS): `GOWORK=off go run ./patterns`
- Run visual checks: `LVT_VISUAL_CHECK=true GOWORK=off go test -v ./patterns -run Visual_Check`
- **Test at least twice consecutively** before declaring a test "passing" — `TestActiveSearch/Clear_Query_Restores_All` surfaced as flaky in CI after appearing to pass locally. Running the full suite twice in a row catches order-dependent state leaks between tests sharing the same Chrome container.

**Per-session workflow reminders** (complements the Session workflow paragraph above):

- Always run the full `GOWORK=off go test -v -race ./patterns` suite locally before pushing — CI flakes happen, but local flakes should be investigated, not ignored.
- Create a worktree under `.worktrees/<session-name>` in **both repos if the session needs library or client changes**. Session 3 required parallel worktrees (examples + livetemplate) because the Session.TriggerAction wiring gap was discovered mid-session. From the `livetemplate/` repo root, run `grep -n "WithSession(newLocalSession" mount.go` at the START of any server-push session to verify the framework plumbing exists before writing patterns.
- **Always release any new library or client version BEFORE opening the examples PR** if the patterns depend on a fresh fix in either repo. CI fetches `@latest` from jsdelivr for the client and the `go.mod`-pinned version from the Go proxy for the library; neither will see an unreleased fix, so the PR will be DOA until the release propagates. Session 3 shipped `livetemplate v0.8.18` first, then opened the examples PR against it — that sequencing is required.
- AI code review workflow guidance (bot review loop convergence, explicit-decline pattern, "project guidance trumps bot suggestions") lives in [`livetemplate/CLAUDE.md`](../../CLAUDE.md) under the "AI Code Review Workflow" section — it's tooling-level workflow that applies to every PR in both repos, not patterns-specific technical guidance.
- [`livetemplate/examples#62`](https://github.com/livetemplate/examples/issues/62)–[`#68`](https://github.com/livetemplate/examples/issues/68) are open follow-ups from Session 1 review — address them in later sessions if touching the affected files, otherwise leave for Session 7 polish.

**Flash lifecycle — persistent until cleared (v0.8.19+):** Flash messages on WebSocket connections now persist across re-renders until explicitly cleared via `ctx.ClearFlash(key)` or auto-expired via `ctx.SetFlash(key, msg, livetemplate.FlashExpiry(duration))`. This is a breaking change from the pre-v0.8.19 behavior where flash auto-cleared after every render. HTTP connections remain one-shot regardless (per-request `connState` is GC'd after the handler returns). Three patterns for managing flash under the new model:
1. **Auto-expire (preferred for transient feedback):** `ctx.SetFlash("success", "Saved!", livetemplate.FlashExpiry(5*time.Second))` — the message is pruned on the next render after the expiry elapses.
2. **Explicit clear (for replacing one flash with another):** `ctx.ClearFlash("error")` before `ctx.SetFlash("success", "Done!")`. Always clear the opposing category before setting a new flash to avoid both success and error showing simultaneously.
3. **No expiry (for persistent status):** `ctx.SetFlash("warning", "Unsaved changes")` — stays visible until the controller explicitly calls `ctx.ClearFlash("warning")`.

Verify: `grep -n "FlashExpiry\|ClearFlash" context.go` (should show both public methods).

**`__navigate__` reserved action (v0.8.19 server, v0.8.26 client):** Same-pathname link clicks (query-param-only changes) are now routed through the WebSocket `__navigate__` action instead of an HTTP fetch. The client sends `{action: "__navigate__", data: {tab: "tab2"}}` and the server re-runs `Mount()` with the data as query parameters — no controller method required. `ctx.Action()` returns `""` inside the re-run Mount, so existing `if ctx.Action() == ""` guards work correctly. HTTP requests reject `__navigate__` with 400 (wrong transport). Patterns that benefit: #13 (URL-Preserved Filters), #19 (Tabs). Both use same-pathname `<a href="?param=value">` links. Cross-pathname links (Pattern #20 SPA Navigation) still use HTTP fetch + WebSocket reconnect. Verify: `grep -n "actionNavigate" mount.go action.go` shows the server-side handling.

**Client v0.8.33+ changelog (v0.8.23→v0.8.33, 10 patch releases):** Summary of client changes relevant to Sessions 4+:
- v0.8.21: Invoker Commands polyfill (`command`/`commandfor`) for Firefox/Safari dialog support
- v0.8.26: `__navigate__` in-band SPA navigation, `lvt-ignore`/`lvt-ignore-attrs` morphdom escape hatches, DOMParser fallback for `<script>` tags, **cross-pathname navigation always reconnects** (BREAKING)
- v0.8.28: Multiple same-name checkboxes send `[value1, value2]` array instead of boolean
- v0.8.29: Checkbox/radio checked state preserved across morphdom updates; override with `data-lvt-force-update`
- v0.8.30: Hash-driven deep linking — URL fragments activate `<dialog>`, `[popover]`, `<details>` via `history.pushState`; supports browser Back/Forward
- v0.8.31: Datalist preservation (defers morphdom while connected input has focus)
- v0.8.32: WebSocket resilience — proper promise settlement on CONNECTING/CLOSING/CLOSED, handler detach before socket close on disconnect
- v0.8.33: `lvt-scroll-sentinel` attribute replaces `id="scroll-sentinel"`, dialog child updates inside open dialogs (morphdom no longer skips entire subtree), `lvt-scroll-away` for scroll-to-top buttons, `data-lvt-target` for scroll effect targeting

**Hash-driven deep linking (client v0.8.30+):** URL fragments (`#id`) automatically activate `<dialog>` (via `showModal`), `[popover]` (via `showPopover`), and `<details>` (via `open` attribute) elements matching the hash. Supports invoker buttons, `<a href="#id">` links, and browser Back/Forward. Uses `history.pushState` (not `location.hash` assignment) to avoid double-activation errors. Source: `client/dom/hash-link.ts`. Patterns #17 (Modal Dialog) and #18 (Confirm Dialog) can be opened via direct URL like `/patterns/navigation/modal-dialog#edit-dialog`. The hash is cleared when the element deactivates (e.g., dialog close).

**`lvt-ignore` / `lvt-ignore-attrs` (client v0.8.26+):** Morphdom escape hatches. `lvt-ignore` skips the element and its entire subtree during diffing — use for third-party widgets (maps, rich-text editors) whose DOM is externally managed. `lvt-ignore-attrs` skips attribute diffing but still diffs children — use when client-set attributes (e.g., `open` on `<details>`) need to survive server updates. Both are checked on `fromEl` (live DOM), so both server templates and client JS can apply them. Override with `data-lvt-force-update` on the server template to resume diffing. See the full reference in `docs/references/client-attributes.md` under "Automatic Client-Side State Preservation."

**Dialog child updates (client v0.8.33+):** Prior to v0.8.33, morphdom skipped the entire subtree of an open `<dialog>` element. This meant form validation errors, input values, and other dynamic content inside an open dialog would not update until the dialog was closed and reopened. v0.8.33 fixes this by allowing child updates inside open dialogs while preserving the dialog's open/top-layer state. This is directly relevant to Patterns #17 (Modal Dialog) and #18 (Confirm Dialog) where forms inside dialogs need real-time validation feedback. Source: client PR #93.

**Checkbox array values (client v0.8.28+):** Multiple same-name checkboxes now send an array `[value1, value2]` instead of a boolean. Single checkboxes remain boolean (backward compatible). This affects Pattern #4 (Bulk Update) only if it uses same-name checkboxes — the current implementation uses per-user-ID names (`active-{id}`), which are unique per checkbox and therefore unaffected. If a future pattern groups checkboxes under a single name (e.g., `name="selectedIds"` for bulk selection), the server receives an array.

**`lvt-scroll-sentinel` attribute (client v0.8.33+):** Replaces `id="scroll-sentinel"` for infinite scroll sentinel detection. The attribute-based approach is preferred because it is declarative (no magic ID) and consistent with other `lvt-*` attributes. The old `id`-based approach continues to work for backward compatibility. Pattern #10 (Infinite Scroll) already uses `<div lvt-scroll-sentinel>` (migrated in PR #352/#353). The `load_more` action dispatch remains the same regardless of detection method. Source: client PR #92, `client/dom/observer-manager.ts`.

**`lvt-scroll-away` (client v0.8.33+, top edge in v0.8.36+):** The `lvt-scroll-away` attribute toggles a `.visible` CSS class based on the `data-lvt-target` container's distance from a named edge. Both edges are supported: `edge="bottom"` adds `.visible` while `scrollHeight - scrollTop - clientHeight > threshold` (user is far from the bottom — fits "more content below" affordances); `edge="top"` (v0.8.36+) adds `.visible` while `scrollTop > threshold` (user has scrolled away from the top — fits scroll-to-top buttons). Threshold is configurable via `--lvt-scroll-threshold` CSS custom property (default: **200px**). The `data-lvt-target` selector currently supports only `#id` and `closest:sel` — page-level (window) scroll is not yet a valid target, tracked at [livetemplate/client#104](https://github.com/livetemplate/client/issues/104). Source: `client/dom/scroll-away.ts`.

**Forms inside `<dialog>` — use `BindAndValidate`, not `ctx.ValidateForm`.** As of livetemplate v0.8.21, `ctx.ValidateForm()` reads its schema from `c.formSchema`, which is populated by `ExtractFormSchema(statics)` walking the template. Forms nested inside `<dialog>` elements aren't surfaced reliably by that walk — calling `ctx.ValidateForm()` returns `nil` and validation silently no-ops, so `{{.lvt.ErrorTag}}` and `{{.lvt.AriaInvalid}}` never trigger. Use the explicit struct shape that `examples/dialog-patterns` and Session 4's Modal Dialog use instead:
```go
var validate = validator.New()  // package-level singleton
type input struct {
    Name  string `json:"name"  validate:"required,min=3"`
    Email string `json:"email" validate:"required,email"`
}
func (c *Controller) Save(state State, ctx *Context) (State, error) {
    var in input
    if err := ctx.BindAndValidate(&in, validate); err != nil { return state, err }
    // ... mutate state from validated `in` ...
}
```
The framework's `BindAndValidate` populates field errors regardless of where the form is in the DOM. If you switch a non-dialog Session 4+ form from `BindAndValidate` to `ctx.ValidateForm`, **verify the field-error path empirically** — the schema-walk has subtle rules that aren't documented in the API.

**Button `name` shadows form `name` in action resolution.** When a form has `<form name="X">` and a submit button has `<button name="Y">`, the button's name wins (the framework dispatches action `Y`, not `X`). Two safe shapes:
- Pick a single canonical name: `<form name="delete"><button name="delete" value="{{.ID}}">…` (form name decorative-but-honest, button drives routing).
- Drop the form name entirely: `<form method="POST"><button name="delete" value="{{.ID}}">…` (no shadow at all).

Both ship in this codebase. The bot review in Session 4 PR #74 cycled between the two — pick one and stick with it per pattern.

**`t.Cleanup`-based restore for fetch-mock subtests.** When a subtest patches `window.fetch` (or any browser-global) to count HTTP hits — the canonical way to assert an in-band `__navigate__` action didn't fall back to HTTP — register the restore in `t.Cleanup`, NOT inline after the assertion. If a chromedp step fails mid-subtest, the inline restore never runs and the mocked `fetch` leaks into later subtests, producing cascading false failures. See `TestTabs/Tab_Switch_Uses_WebSocket_Not_HTTP` in `examples/patterns/patterns_test.go` for the canonical shape.

**Conditional event-listener placement (`lvt-on:window:keydown`).** Re-entrancy guards in the controller (`if state.Running { return state, nil }`) keep stray firings *correct*, but each firing still costs a WebSocket round-trip, server re-render, and morphdom diff. For state-dependent listeners — e.g. "/" opens a panel only when it's closed — render the binding only on the relevant template branch:
```html
{{if .PanelOpen}}
<article lvt-on:window:keydown="close" lvt-key="Escape">
{{else}}
<article lvt-on:window:keydown="open" lvt-key="/">
{{end}}
```
This was Session 4's Keyboard Shortcuts pattern. The bot review flagged it 4 times across rounds before being conceded; the cleanup is a 5-line template change for a meaningful saving.

**Local-Docker chromedp WS frame delivery quirk.** When running E2E tests locally against `chromedp/headless-shell` in Docker, **small WebSocket frames sent immediately after a larger initial render can take 5–6 seconds to reach the client**. Specifically: server `Connection.WriteMessage` returns instantly, but the client's `onmessage` doesn't fire until ~6s later. Tests like `TestLazyLoading/Data_Arrives_Via_Server_Push` time out locally but pass on GitHub Actions CI (where `Test All Examples` historically completes the same scenario in ~2s). If a goroutine-push pattern test times out locally, **don't dig into the framework** — it's almost certainly the Docker bridge (likely TCP slow-start / quickack interaction with small frames after a quiet idle gap). Run the same test on a host Chrome (not Docker) to confirm: with host chrome the server-push path completes in <50ms.

**FlashExpiry pruning runs before the render snapshot, not after.** Library v0.8.22+ (released as part of Session 5; see [livetemplate/livetemplate#359](https://github.com/livetemplate/livetemplate/pull/359)) moved `pruneExpiredFlash()` from a post-`sendUpdate` call site into the start of `getMessages()` — the single funnel every render path hits. Pre-v0.8.22 a flash with `FlashExpiry(d)` got one extra render after the deadline before being evicted from `connState.messages`, so users observed e.g. a 5-second auto-dismiss flash "staying one extra interaction past its deadline." The Session 5 Flash Messages pattern surfaced the bug by exercising the canonical pitch ("save, wait 5s, success disappears") end-to-end. Workaround on older library versions: trigger any second action after the deadline to re-snapshot. **Don't write demo text claiming "auto-expires after Ns" if your `go.mod` is below v0.8.22** — the user-visible behavior won't match. Verify with `grep -n "pruneExpiredFlash" mount.go`: under v0.8.22+ the only call is inside `getMessages()`; pre-v0.8.22 you'll see four post-`sendUpdate` call sites instead. Also: error renders now also prune (intentional — the deadline is a deadline regardless of whether the next action succeeded).

**`lvt-fx:highlight` leaves an empty `style=""` attribute after cleanup.** The animate directive's `animationend` handler at `client/dom/directives.ts:230-232` calls `removeAttribute('style')` when `style.length === 0`; the highlight branch (lines 178-189) does not. After a highlight cycle (50ms delay + duration), `style.backgroundColor` and `style.transition` are reset to empty strings, but the `style` attribute itself lingers as `style=""`. Functionally a no-op for CSS rendering, but strict validators (`runUIStandards` at `lvt/testing/chrome.go:67-71`, `ValidatePicoCSS` at `chrome.go:993-998`) reject any `[style]` regardless of content. The Session 5 `Highlight on Change` pattern works around this by intentionally **skipping `UI_Standards` for that test** (the pattern's whole premise is inline styling — the rule isn't a meaningful guarantee). Filed as [livetemplate/client#100](https://github.com/livetemplate/client/issues/100). When that ships, the `runStandardSubtests(t, ctx, false, ...)` skip in `TestHighlightOnChange` can be replaced with the standard `runStandardSubtests(...)` call.

**Don't assert `!hasAttribute('style')` for animate cleanup — assert `style.animation === ""` instead.** The animationend handler's `removeAttribute('style')` only fires when `style.length === 0`. Morphdom can transiently add or hold other style declarations on `<li data-key="...">` elements during a render, leaving `style` non-empty even after the animation property is cleared. The semantically correct end-of-animation signal is `style.animation === ""` directly. CI's "Test All Examples" job (a slower environment) caught this on the Session 5 PR — the local-Docker chromedp variant happened to clear the attribute fast enough that the brittle assertion passed. Session 5 fixed `Existing_Rows_Do_Not_Re_animate` to use the direct check; mirror that in any future animate-cleanup test.

**AI review loop convergence — "real bugs by round 4" heuristic.** Session 4's PR #74 went 11 rounds with the Claude review bot. The trajectory:

| Round | Real fixes | Style/docs | Declined repeats |
|-------|------------|-----------|------------------|
| 1     | 6          | 0         | 2                |
| 2     | 5          | 3         | 1                |
| 3     | 0          | 3         | 1                |
| 4     | 2 real bugs | 0        | 3                |
| 5–11  | 0          | 1–3 each  | 3–7 each (mostly repeats) |

Round 4's real bugs were the Tier-1 fallback regression (4 forms missing `method="POST"`) and the form-state assertion gap. After that, the bot generated reviews indefinitely on each push, surfacing only style nits, doc clarifications, and the occasional flip-flop on its own prior advice (round 6 said "rename form names to match buttons"; round 11 said "drop one of the matching names"). **The signal: if 2+ consecutive rounds produce zero new functional issues, stop.** Continuing past that yields churn, not convergence. Decline-with-PR-reply is the correct response — the bot reads prior comments before generating its next review, so a clear decline breaks the cycle.

**Headless Chrome resource limits + Chrome backgrounding heuristics together caused server-pushed-render flakes for ~all of Session 5 and Session 3 (latent).** Session 6's first reproducer found that any pattern using `session.TriggerAction(...)` from a goroutine (Server Push, Live Preview, Lazy Loading, Async Operations, Progress Bar — and intermittently any TriggerAction-driven render after the initial action's response) had server-pushed renders queue silently for ~7s and then burst, because:

1. `lvt/testing/chrome.go`'s `docker run --cpus 0.5` was below the threshold needed for headless Chrome to keep up with WS message processing AND morphdom application on test pages, AND
2. headless mode's renderer is never visible, so default Chrome treats every tab as backgrounded and applies aggressive WS/timer throttling on top.

A `MutationObserver` attached to `<article>` during a `chromedp.Sleep` showed the burst clearly on the Server Push 10-tick demo:

```
with --cpus 0.5 (the broken state):     t=27ms click, then GAP, t=7894-7900 burst of 7 ticks
without --cpus + throttle-disable flags: t=27ms click, t=4022 t=5022 t=6022 ... 1Hz cadence
```

Fix: drop `--cpus 0.5`, add `--shm-size 256m` and `--disable-background-timer-throttling --disable-renderer-backgrounding --disable-backgrounding-occluded-windows --disable-features=IntensiveWakeUpThrottling`. Released as [livetemplate/lvt#314](https://github.com/livetemplate/lvt/pull/314) → **lvt v0.1.4**. After bumping `examples/go.mod`, the Session 5 PR #79's flaky "Test All Examples" CI is also fixed (it was failing for the same root cause). When designing future `TriggerAction`-driven patterns, no test workaround is needed at v0.1.4+; before the bump, the only viable workaround was `chromedp.Sleep` + `Evaluate` (which the AI review explicitly rejects).

**Cross-connection updates are explicit.** Pattern #26 (Multi-User Refresh) calls `BroadcastAction("RefreshCounter", nil)` from `Increment`, and peers re-pull the shared counter from the controller. Nothing crosses connections unless the action asks for it.

**`lvt:"persist"` semantics: storage is keyed by SESSION GROUP, not connection.** Two tabs in the same browser share the group cookie → share persisted state. Pattern #28 (Presence) intentionally does NOT persist `Username`/`Joined` so two tabs joined as different users can coexist; Pattern #27 (Broadcasting) similarly leaves `Username` un-persisted. Pattern #29 (Reconnection Recovery) DOES persist `Counter`/`Notes` because that's the canonical demo. The decision is documented in `state_realtime.go` per-field comments — useful pedagogical contrast for readers learning the framework.

**`lvt-fx:highlight`'s ~550ms transition window survives chromedp polling at lvt v0.1.4 — barely.** With the chrome-throttling fix in place, the polled `WaitFor` for `style.transition.includes('background-color')` lands inside the directive's [50ms..550ms] window reliably in single-test runs but flakes ~10-20% under full-suite concurrency. The earlier `client#100` follow-up (skip-UI_Standards-for-highlight) is still open; once `directives.ts` adds the `removeAttribute('style')` cleanup parity with animate, the highlight test can also rely on a polled `WaitFor` that observes the cleaned state (rather than catching the brief transition window). Until then, treat occasional `TestHighlightOnChange` flakes in CI as known and re-run.

**`FlashExpiry` is render-driven — needs a goroutine nudge to actually fade.** Calling `ctx.SetFlash("success", msg, livetemplate.FlashExpiry(5*time.Second))` does NOT auto-fade the flash on the client at the deadline. `pruneExpiredFlash` runs inside `getMessages` on the next render — without a follow-up render at the deadline, the expired entry sits in the DOM until the user's next interaction. Pattern #25 (Flash Messages) had this nudge inline; Session 7 extracted it as `nudgeFlashExpiry(ctx, expiry)` in `handlers_feedback.go` and applied to #6, #7, #15, #16. Each calling controller MUST register a no-op `Refresh` action handler (the action the goroutine dispatches). The contract is documented on the helper itself; no compile-time enforcement, accepted as the trade-off vs. a shared base type.

**Pattern #15 (Progress Bar) iOS reconnect resilience: persist Progress+Done, NOT Running, retry-in-goroutine instead of Mount revival.** iPhone Safari kills WebSockets on app-switch. Pre-Session-7 Pattern #15 left the UI stuck at the last rendered Progress (no recovery). The first attempt — Mount revival when state.Running was persisted-true — surfaced an impossible state ("Run Again at 70%") because the original goroutine's retry loop and the Mount-spawned goroutine raced UpdateProgress writes: one would set Done=true, the other would overwrite Progress with a mid-flight value. The shipped fix: (a) persist `Progress`/`Done` only — Running stays per-connection so a stale "Running=true with no goroutine" can't happen (same trade-off Pattern #31 makes); (b) goroutine retries `TriggerAction` for ~5s/tick (50 attempts × 100ms) so brief disconnects below the client's 3s visibility-reconnect threshold are absorbed without revival; (c) `UpdateProgress` guards on `state.Done` already being true to drop stale ticks (defense-in-depth). Worst-case goroutine lifetime under permanent disconnect: `(progressTickRate + progressRetryWindow) × 10` ≈ 55s. Four new chromedp tests cover brief-disconnect-completes, long-disconnect-settles-cleanly, multi-cycle-no-impossible-state, done-survives-reconnect.

**iOS WebSocket "zombie" sockets — visibility-reconnect required (client v0.8.35).** When iPhone Safari backgrounds, iOS may keep the WebSocket reporting `OPEN` even after killing the underlying TCP connection. `onclose` never fires, so `autoReconnect` (which keys off `onclose`) doesn't trigger and the page receives no further updates. Client v0.8.35 added `visibilitychange`/`pageshow` handlers that always reconnect after >3s of background, regardless of `readyState`. Reconnecting a healthy connection is cheap (morphdom diffs produce zero DOM changes), and the always-reconnect-after-long-bg policy is the correct defense for the zombie case. The 3s threshold trades off against unnecessary reconnects on healthy desktop tab-switches; sub-3s windows still rely on `onclose` + `autoReconnect` (correct for desktop) OR per-pattern goroutine-side resilience (Pattern #15's retry covers this on iOS).

### Session 1: Scaffold + Index Page + Forms & Editing

**Scope:** App skeleton, shared layout, index page, patterns #1–7

- [x] Create `examples/patterns/` directory structure
- [x] `main.go` — router with all handler registrations
- [x] `templates/layout.tmpl` — shared HTML layout
- [x] `templates/index.tmpl` — categorized grid of all patterns with descriptions
- [x] Index handler with pattern metadata
- [x] `data.go` — in-memory sample data (contacts, users, items)
- [x] Per-category state files (`state_forms.go`, `state_lists.go`, etc.)
- [x] Implement Click To Edit (#1)
- [x] Implement Edit Row (#2)
- [x] Implement Inline Validation (#3)
- [x] Implement Bulk Update (#4)
- [x] Implement Reset User Input (#5)
- [x] Implement File Upload (#6)
- [x] Implement Preserving File Inputs (#7)
- [x] E2E tests for patterns #1–7 (incl. UI_Standards + Visual_Check)
- [x] Run app locally, wait for manual review signoff
- [x] Create PR (livetemplate/examples#59), update this tracker

### Session 2: Lists & Data + Search & Filtering

**Scope:** Patterns #8–13

- [x] Implement Delete Row (#8)
- [x] Implement Click To Load (#9)
- [x] Implement Infinite Scroll (#10)
- [x] Implement Value Select (#11)
- [x] Implement Active Search (#12)
- [x] Implement URL-Preserved Filters (#13)
- [x] E2E tests for patterns #8–13 (incl. UI_Standards + Visual_Check)
- [x] Update index page with patterns #8–13
- [x] Run app locally, wait for manual review signoff
- [x] Create PR (livetemplate/examples#69), update this tracker

### Session 3: Loading & Progress

**Scope:** Patterns #14–16

- [x] Implement Lazy Loading (#14)
- [x] Implement Progress Bar (#15)
- [x] Implement Async Operations (#16)
- [x] Verify goroutine cleanup on disconnect
- [x] E2E tests for patterns #14–16 (incl. UI_Standards + Visual_Check)
- [x] Update index page with patterns #14–16
- [x] Run app locally, wait for manual review signoff
- [x] Create PR, update this tracker — merged as livetemplate/examples#70 (plus library fix livetemplate/livetemplate#336 for Session.TriggerAction wiring)

### Session 4: Dialogs, Tabs & Navigation

**Scope:** Patterns #17–21

- [x] Verify invoker polyfill is active in current client build (Firefox/Safari) — confirmed working since client v0.8.21
- [x] Verify hash-driven deep linking works for dialogs (client v0.8.30+) — `<a href="#edit-dialog">` opens modal; `Open_Via_Hash_Link` subtest asserts `location.hash === "#edit-dialog"` after the link click, and `Browser_Back_Closes_Dialog` confirms `handlePopstate` correctly sweeps the open dialog when the hash unwinds.
- [x] Test dialog child updates inside open dialogs (client v0.8.33 fix) — `Submit_Invalid_Form_Stays_Open_With_Field_Errors` is the keystone subtest: empty submit → server returns field-level error → `{{.lvt.ErrorTag}}` + `aria-invalid="true"` render inside the still-open dialog.
- [x] Verify `__navigate__` in-band navigation for Tabs pattern (library v0.8.19+, client v0.8.26+) — `Tab_Switch_Uses_WebSocket_Not_HTTP` patches `window.fetch` and asserts zero HTTP hits during a `?tab=…` link click.
- [x] Test cross-pathname navigation reconnect behavior for SPA Navigation pattern (client v0.8.26+ breaking change) — `SPA_Navigation_Cross_Pathname_Reconnects` asserts the `data-lvt-id` wrapper attribute changes after a cross-handler link click (proving a fresh handler/wrapper, not in-place swap).
- [x] Implement Modal Dialog (#17)
- [x] Implement Confirm Dialog (#18)
- [x] Implement Tabs (HATEOAS) (#19)
- [x] Implement SPA Navigation (#20)
- [x] Implement Keyboard Shortcuts (#21)
- [x] E2E tests for patterns #17–21 (incl. UI_Standards + Visual_Check)
- [x] Update index page with patterns #17–21
- [x] Run app locally, wait for manual review signoff
- [x] Create PR, update this tracker — merged as [livetemplate/examples#74](https://github.com/livetemplate/examples/pull/74). Follow-ups filed: [examples#75](https://github.com/livetemplate/examples/issues/75) (couple `spaNavMaxStep` with template), [#76](https://github.com/livetemplate/examples/issues/76) (full ARIA tablist semantics), [#77](https://github.com/livetemplate/examples/issues/77) (XHR fallback in fetch-mock tests), [#78](https://github.com/livetemplate/examples/issues/78) (wire decorative search input).

### Session 5: Visual Feedback

**Scope:** Patterns #22–25

- [x] Verify library v0.8.19+ for `FlashExpiry`/`ClearFlash` API — bumped to v0.8.22 partway through; the Flash Messages pattern surfaced a real bug in v0.8.21's prune timing (see Implementation Notes below) and we shipped the fix upstream first.
- [x] Implement Animations (#22) — `lvt-fx:animate="fade|slide|scale"` with mode select; `AnimationItem.Mode` stored per-item so each row's `lvt-fx:animate` attribute reflects the mode it was added with (not the current select value).
- [x] Implement Loading States (#23) — three forms in one page: Tier 1 (auto `aria-busy` + fieldset disable), Tier 2a (`lvt-form:disable-with`), Tier 2b (`lvt-el:setAttr:on:pending`/`on:done`). All three submit the same `slowSave` action so users see the three presentations side-by-side.
- [x] Implement Highlight on Change (#24) — two `<div lvt-fx:highlight="flash">` cards sharing one counter to demonstrate the per-element-per-render-touching-subtree firing model. UI_Standards is intentionally skipped for this pattern (see Implementation Notes).
- [x] Implement Flash Messages (#25) — full lifecycle: `Save` (success with `FlashExpiry(5s)` or persistent error), `Notify` (persistent info), `DismissNotify` (`ClearFlash`). Demonstrates auto-expiry, manual clear, and the "clear opposing flash" pattern from `examples/flash-messages`.
- [x] E2E tests for patterns #22–25 (incl. UI_Standards + Visual_Check) — `TestAnimations`, `TestLoadingStates`, `TestHighlightOnChange`, `TestFlashMessages` with form-field state assertions per CLAUDE.md.
- [x] Update index page with patterns #22–25
- [x] Run app locally, wait for manual review signoff
- [x] Create PR, update this tracker — merged as [livetemplate/examples#79](https://github.com/livetemplate/examples/pull/79). Companion library fix [livetemplate/livetemplate#359](https://github.com/livetemplate/livetemplate/pull/359) released as v0.8.22. Follow-up: [livetemplate/client#100](https://github.com/livetemplate/client/issues/100) for `lvt-fx:highlight` cleanup parity with `lvt-fx:animate`.

### Session 6: Real-Time & Multi-User

**Scope:** Patterns #26–31

- [x] Implement Multi-User Refresh (#26) — `Increment` explicitly broadcasts `RefreshCounter` to peers in the same session group; `Mount()` seeds late-joiners from shared controller state.
- [x] Implement Broadcasting (#27) — `ctx.BroadcastAction("NewMessage", nil)` after lock release (deadlock prevention with connection registry mutex); `Mount()` snapshots `c.messages` for new connections; `Username` intentionally not `lvt:"persist"` so two tabs in one browser stay independent.
- [x] Implement Presence Tracking (#28) — Join/Leave + `BroadcastAction("PresenceChanged")`; `Mount()` seeds `OnlineCount` for late-joining tabs; documented limitations: close-tab leak (no `OnDisconnect` state/ctx) + same-username collision (map keys on username).
- [x] Implement Reconnection Recovery (#29) — `Counter` and `Notes` tagged `lvt:"persist"`; full page reload restores via session-group cookie. Template explainer covers `lvt:"persist"` (server-side, survives reconnect) vs `lvt-form:preserve` (client-side, survives in-DOM during other re-renders).
- [x] Implement Live Preview (#30) — `Change()` auto-binding (300ms debounce). Mirrors `live-preview/main.go`: `Change` does NOT write back to `state.Input` (cursor-reset prevention); separate `Submit` action commits the value.
- [x] Implement Server Push (#31) — `session.TriggerAction(...)` from background goroutine; checking the error return is the documented goroutine-cancellation pattern (no done channel needed). `Running` not persisted: post-reconnect the goroutine continues and the UI pops directly to "Last completed" rather than risking a stuck Running view.
- [x] E2E tests for patterns #26–31 — multi-tab tests (#26-28) use `chromedp.NewContext(parent)` for cookie-shared peer tabs. Empty-input guard tests use the "guard message" idiom (fire empty + known-good action, wait for known-good's effect) instead of wall-clock Sleep. `Late_Joiner_Sees_Current_Counter_On_Mount` regresses the `Mount()` necessity.
- [x] Update index page with patterns #26–31 — Category 7 entries flipped to `Implemented: true` in `data.go`.
- [x] Run app locally, wait for manual review signoff — user confirmed via /loop driving the Session 6 work.
- [x] Create PR, update this tracker — merged as [livetemplate/examples#80](https://github.com/livetemplate/examples/pull/80) at `56c14d1`. Companion test-infra fix [livetemplate/lvt#314](https://github.com/livetemplate/lvt/pull/314) released as **lvt v0.1.4** — drops `--cpus 0.5` from `StartDockerChrome`'s docker run + adds `--shm-size 256m` and Chrome backgrounding-disable flags. Without this, server-pushed renders queued silently for ~7s and burst, breaking any test using `session.TriggerAction`. AI-review loop converged at rounds 12-13 ("Approve with minor suggestions").

### Session 7: Polish + README + Final Review

**Scope:** README, final review, integration verification

- [x] `README.md` for the patterns example — quick start + 31-pattern catalog by category + architecture pointer + testing instructions; ~120 lines, in-app index page is the canonical catalog.
- [x] Run `./test-all.sh` to verify all examples still pass — full suite green (12/12 examples). Pre-existing counter test bug surfaced: `cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)` puts the new PORT FIRST so a leaked PORT env var from a prior shell can override it (Go exec uses last-occurrence). Not a Session 7 regression; filed as a follow-up if it causes flakes.
- [x] Audit all patterns using `ctx.SetFlash()` for correct `FlashExpiry`/`ClearFlash` usage — added `FlashExpiry(5s)` to success flashes in #6 (File Upload), #7 (Preserving File Inputs), #15 (Progress Bar), #16 (Async Operations) plus a `nudgeFlashExpiry` helper extracted from #25 that schedules the follow-up render at the deadline (`FlashExpiry` is render-driven; without the nudge, the expired flash sits in the DOM until the user's next interaction). Each affected controller gets a no-op `Refresh` action handler as the nudge target. **Policy:** success → expire after 5s; error → sticky; mutual exclusion (#4) → `ClearFlash` of the opposite key.
- [x] Consider adding `lvt-scroll-away` scroll-to-top button to Pattern #10 (Infinite Scroll) — **partially unblocked.** The top-edge primitive shipped as [livetemplate/client#102](https://github.com/livetemplate/client/issues/102) ([PR #103](https://github.com/livetemplate/client/pull/103), client **v0.8.36**); duplicate [livetemplate/client#97](https://github.com/livetemplate/client/issues/97) closed in the same cycle. Pattern #10 itself still defers, however: it uses page-level scroll (the document body), but `lvt-scroll-away`'s `data-lvt-target` selector accepts only `#id`/`closest:sel` — no window/document keyword. Filed [livetemplate/client#104](https://github.com/livetemplate/client/issues/104) for the page-level scroll target. Until that lands, Pattern #10's scroll-to-top affordance remains deferred; integrating it would require wrapping the list in an `overflow:auto` container, which would change Pattern #10's UX from "scroll the page" to "scroll a panel" — judged not worth that trade. Doc drift at line 2311 (above) updated in the same commit that records this resolution.
- [x] File GitHub issue for drag-and-drop feature — [livetemplate/client#101](https://github.com/livetemplate/client/issues/101).
- [x] Final review and polish — Session 1 review follow-ups closed: [#62](https://github.com/livetemplate/examples/issues/62) (double-title via conditional layout `<title>` + drop redundant index Title), [#63](https://github.com/livetemplate/examples/issues/63)/[#64](https://github.com/livetemplate/examples/issues/64)/[#65](https://github.com/livetemplate/examples/issues/65) (aria-labels on edit-row, reset-input, bulk-update inputs/checkboxes). [#66](https://github.com/livetemplate/examples/issues/66) (form wrapper) was already fixed in a prior session; [#67](https://github.com/livetemplate/examples/issues/67)/[#68](https://github.com/livetemplate/examples/issues/68) (constant-true `WaitFor` polls) were also already replaced with condition-based waits in earlier sessions.
- [x] Run app locally, wait for manual review signoff — user confirmed via /loop driving Session 7. iPhone-Safari testing surfaced two real bugs: (a) `progress bar hangs after Safari background` traced to **client-side**: WebSocket goes zombie/dead on iOS app-switch; landed [livetemplate/client#99](https://github.com/livetemplate/client/pull/99) — `visibilitychange`/`pageshow` reconnect with iOS zombie-socket defense — released as **client v0.8.35**. (b) After v0.8.35 the timer sometimes showed an impossible "Run Again at 70%" UI; root cause was a Mount-spawned competing goroutine racing the original retrying goroutine — fixed in Pattern #15 by removing Mount revival entirely, persisting `Progress`+`Done` (not `Running`), retrying TriggerAction in the goroutine for ~5s/tick to cover sub-3s backgrounds, and guarding `UpdateProgress` on `state.Done` for defense-in-depth. Four new chromedp tests cover the disconnect scenarios.
- [x] Create PR, update this tracker — merged as [livetemplate/examples#81](https://github.com/livetemplate/examples/pull/81). Companion library PR [livetemplate/client#99](https://github.com/livetemplate/client/pull/99) released as **client v0.8.35**. AI-review loop converged at rounds 4-5 (no NEW functional issues, only iterations on already-declined points).

**All 7 sessions complete. Patterns example is feature-complete and shipped.**

---

## Testing Strategy

All patterns must have chromedp E2E tests following the [Examples CLAUDE.md](https://github.com/livetemplate/examples/blob/main/CLAUDE.md) testing requirements:

1. **Exercise every controller method** — each action must be triggered in tests
2. **Assert full page state after mutations** — not just the changed element
3. **Use real browser interactions** — `chromedp.SendKeys` + `chromedp.Click`, not WebSocket API bypass
4. **Verify form field state** — check input cleared/retained after mutations
5. **Condition-based waits** — `e2etest.WaitFor()`/`e2etest.WaitForText()`, not `chromedp.Sleep`
6. **Test error paths** — validation failures, empty inputs
7. **UI standards check** — each pattern page must include a `UI_Standards` subtest that validates: no inline event handlers (`onclick`, `onchange`, etc.), no inline styles (except `<ins>`/`<del>` block pattern), `color-scheme` meta tag present, `lang="en"` set, container width <= 700px, Pico CSS conventions via `e2etest.ValidatePicoCSS()`, and shared CSS loading. See [live-preview example](https://github.com/livetemplate/examples/blob/main/live-preview/live_preview_test.go#L222) for the pattern.
8. **Visual check** — each pattern page must include a `Visual_Check` subtest using `e2etest.ValidateScreenshotWithLLM(t, ctx, "description of expected layout")`. This captures a browser screenshot and sends it to the Claude CLI for automated visual analysis (alignment, spacing, layout, readability, error state styling). Runs when `LVT_VISUAL_CHECK=true` is set. See [live-preview example](https://github.com/livetemplate/examples/blob/main/live-preview/live_preview_test.go#L257) for the pattern.

E2E tests must have access to: browser console logs, server logs, WebSocket messages, rendered HTML.

> **Test location:** E2E tests live in the examples repo at `examples/patterns/patterns_test.go`, not in the lvt repo. The lvt repo's `e2e/livetemplate_core_test.go` tests the core library — example-specific tests live alongside each example, consistent with all other examples (`todos/todos_test.go`, `chat/chat_test.go`, etc.).

---

## References

- [htmx.org/examples](https://htmx.org/examples/) — Source patterns
- [Phoenix LiveView docs](https://hexdocs.pm/phoenix_live_view/Phoenix.LiveView.html) — Additional patterns
- [FIRST_PRINCIPLES.md](../design/FIRST_PRINCIPLES.md) — Core design philosophy
- [Progressive Complexity Reference](../references/progressive-complexity-reference.md) — Tier 1 vs Tier 2
- [Client Attributes Reference](../references/client-attributes.md) — Complete `lvt-*` attribute listing
- [Examples CLAUDE.md](https://github.com/livetemplate/examples/blob/main/CLAUDE.md) — Example guidelines
