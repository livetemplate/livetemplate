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

**LiveTemplate (Tier 1):** The client has built-in IntersectionObserver support. A `<div id="scroll-sentinel">` at the end of the list triggers the `load_more` action automatically when it becomes visible. The framework routes `load_more` (snake_case) to the `LoadMore()` method.

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
    <div id="scroll-sentinel"><small>Loading more...</small></div>
    {{end}}
</article>
{{end}}
```

**Key features:** `scroll-sentinel` auto-triggers `load_more` (source: `client/dom/observer-manager.ts`), IntersectionObserver built into client

> **Limitation:** The client detects the sentinel by `id="scroll-sentinel"`, so only one infinite scroll list can exist per page. This is fine for the patterns app (one pattern per page).

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

**LiveTemplate (Tier 1):** Native `<dialog>` element with `command`/`commandfor` attributes (polyfilled for Firefox/Safari). Focus trapping, backdrop, and Escape key are all browser-native with `showModal()`.

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

**Key features:** Native `<dialog>`, `command`/`commandfor` (polyfilled), auto-close on form success

> **Polyfill:** The Invoker Commands API polyfill is bundled in the LiveTemplate client library (source: `client/dom/invoker-polyfill.ts`). No additional `<script>` tag is needed — the client detects `commandForElement` support and activates the polyfill automatically for Firefox and Safari.

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

**LiveTemplate (Tier 1):** `ctx.SetFlash()` stores messages that render as toasts. The toast component handles auto-dismiss and positioning.

**Also implemented in:** `flash-messages/`, `todos/`

```go
type FlashState struct {
    Title string
}

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
type SyncController struct {
    mu      sync.Mutex
    counter int
}

type SyncState struct {
    Title   string
    Counter int
}

// Sync is a reserved method name (mount.go:158, syncMethodName = "Sync").
// The framework auto-dispatches it to peer connections in the same session
// group after any action completes, without requiring an explicit
// BroadcastAction call. The state parameter is the peer's LOCAL state.
// The handler reads the shared counter from the controller (singleton)
// so all peers see the same value.
func (c *SyncController) Sync(state SyncState, ctx *livetemplate.Context) (SyncState, error) {
    c.mu.Lock()
    state.Counter = c.counter
    c.mu.Unlock()
    return state, nil
}

func (c *SyncController) Increment(state SyncState, ctx *livetemplate.Context) (SyncState, error) {
    c.mu.Lock()
    c.counter++
    state.Counter = c.counter
    c.mu.Unlock()
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

- Each pattern is its own handler with its own `data-lvt-id`. Client **v0.8.22+** handles the WebSocket disconnect/reconnect transparently — no workarounds needed.
- Use `@latest` CDN in templates (per project convention); do not pin a specific client version. This is an intentional tradeoff — the examples always demonstrate the current client, accepting the risk that a client release could break a demo until fixed. If a demo breaks after a client release, fix the demo (or the client), don't pin the version.

**Local dev loop:**

- Run a specific pattern's tests: `GOWORK=off go test -v -race -timeout=10m ./patterns -run TestPatternName`
- Run against a locally-built client: `LVT_LOCAL_CLIENT=/abs/path/to/client/dist/livetemplate-client.browser.js GOWORK=off go test -v ./patterns`
- Run the app manually (Tier 1 fallback works even without JS): `GOWORK=off go run ./patterns`
- Run visual checks: `LVT_VISUAL_CHECK=true GOWORK=off go test -v ./patterns -run Visual_Check`

**Per-session workflow reminders** (complements the Session workflow paragraph above):

- Always run the full `GOWORK=off go test -v -race ./patterns` suite locally before pushing — CI flakes happen, but local flakes should be investigated, not ignored.
- Create a worktree under `.worktrees/<session-name>` in the examples repo; never work on `main` directly.
- [`livetemplate/examples#62`](https://github.com/livetemplate/examples/issues/62)–[`#68`](https://github.com/livetemplate/examples/issues/68) are open follow-ups from Session 1 review — address them in later sessions if touching the affected files, otherwise leave for Session 7 polish.

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

- [ ] Implement Delete Row (#8)
- [ ] Implement Click To Load (#9)
- [ ] Implement Infinite Scroll (#10)
- [ ] Implement Value Select (#11)
- [ ] Implement Active Search (#12)
- [ ] Implement URL-Preserved Filters (#13)
- [ ] E2E tests for patterns #8–13 (incl. UI_Standards + Visual_Check)
- [ ] Update index page with patterns #8–13
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

### Session 3: Loading & Progress

**Scope:** Patterns #14–16

- [ ] Implement Lazy Loading (#14)
- [ ] Implement Progress Bar (#15)
- [ ] Implement Async Operations (#16)
- [ ] Verify goroutine cleanup on disconnect
- [ ] E2E tests for patterns #14–16 (incl. UI_Standards + Visual_Check)
- [ ] Update index page with patterns #14–16
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

### Session 4: Dialogs, Tabs & Navigation

**Scope:** Patterns #17–21

- [ ] Verify invoker polyfill is active in current client build (Firefox/Safari)
- [ ] Implement Modal Dialog (#17)
- [ ] Implement Confirm Dialog (#18)
- [ ] Implement Tabs (HATEOAS) (#19)
- [ ] Implement SPA Navigation (#20)
- [ ] Implement Keyboard Shortcuts (#21)
- [ ] E2E tests for patterns #17–21 (incl. UI_Standards + Visual_Check)
- [ ] Update index page with patterns #17–21
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

### Session 5: Visual Feedback

**Scope:** Patterns #22–25

- [ ] Implement Animations (#22)
- [ ] Implement Loading States (#23)
- [ ] Implement Highlight on Change (#24)
- [ ] Implement Flash Messages (#25)
- [ ] E2E tests for patterns #22–25 (incl. UI_Standards + Visual_Check)
- [ ] Update index page with patterns #22–25
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

### Session 6: Real-Time & Multi-User

**Scope:** Patterns #26–31

- [ ] Implement Multi-User Sync (#26)
- [ ] Implement Broadcasting (#27)
- [ ] Implement Presence Tracking (#28)
- [ ] Implement Reconnection Recovery (#29)
- [ ] Implement Live Preview (#30)
- [ ] Implement Server Push (#31)
- [ ] E2E tests for patterns #26–31 (incl. UI_Standards + Visual_Check)
- [ ] Update index page with patterns #26–31
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

### Session 7: Polish + README + Final Review

**Scope:** README, final review, integration verification

- [ ] `README.md` for the patterns example
- [ ] Run `./test-all.sh` to verify all examples still pass
- [ ] File GitHub issue for drag-and-drop feature (see [Future Features](#future-features))
- [ ] Final review and polish
- [ ] Run app locally, wait for manual review signoff
- [ ] Create PR, update this tracker

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
