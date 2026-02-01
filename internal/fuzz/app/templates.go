package app

// NestedRangeTemplate is the template for testing nested ranges.
// It tests key namespace collisions, statics at multiple nesting levels,
// and reorder operations at different hierarchy levels.
const NestedRangeTemplate = `<div class="tree-view">
{{range .Categories}}
    <div id="cat-{{.ID}}" class="category">
        <h3>{{.Name}}</h3>
        {{if .IsOpen}}
        <ul class="items">
        {{range .Items}}
            <li id="{{.ID}}" class="{{if .Complete}}done{{end}} priority-{{.Priority}}">
                <span class="title">{{.Title}}</span>
                {{if .Body}}<p class="body">{{.Body}}</p>{{end}}
            </li>
        {{else}}
            <li class="empty">No items in category</li>
        {{end}}
        </ul>
        {{end}}
    </div>
{{else}}
    <p class="empty">No categories</p>
{{end}}
</div>`

// AppTemplate is the template for testing application-level operations.
// It combines conditionals, ranges, and derived views.
const AppTemplate = `<div class="app">
    {{if .ShowSearch}}
    <div class="search">
        <input value="{{.SearchQuery}}" placeholder="Search...">
    </div>
    {{end}}

    {{if .ShowFilters}}
    <div class="filters">
        <button class="{{if eq .Filter "all"}}active{{end}}">All</button>
        <button class="{{if eq .Filter "active"}}active{{end}}">Active</button>
        <button class="{{if eq .Filter "completed"}}active{{end}}">Completed</button>
        <span>Sort: {{.SortBy}} {{.SortOrder}}</span>
    </div>
    {{end}}

    <ul class="items">
    {{range .FilteredItems}}
        <li id="{{.ID}}" class="{{if .Complete}}done{{end}} priority-{{.Priority}}">
            <span class="title">{{.Title}}</span>
            {{if .Body}}<p class="body">{{.Body}}</p>{{end}}
        </li>
    {{else}}
        <li class="empty">
            {{if ne $.SearchQuery ""}}
                No items match "{{$.SearchQuery}}"
            {{else if eq $.Filter "active"}}
                All items completed!
            {{else if eq $.Filter "completed"}}
                No completed items yet
            {{else}}
                No items yet
            {{end}}
        </li>
    {{end}}
    </ul>

    {{if ne .SelectedID ""}}
    <div class="detail-panel">
        <h2>Viewing: {{.SelectedID}}</h2>
    </div>
    {{end}}
</div>`

// PaginatedTemplate is the template for testing pagination.
// It tests page transitions, loading states, and key stability across pages.
const PaginatedTemplate = `<div class="paginated-list">
    <div class="page-info">
        <span>Page {{.Page}}</span>
        <span>{{.PageSize}} per page</span>
    </div>

    {{if .LoadingMore}}
    <div class="loading">Loading...</div>
    {{end}}

    <ul class="items">
    {{range .VisibleItems}}
        <li id="{{.ID}}" class="{{if .Complete}}done{{end}} priority-{{.Priority}}">
            <span class="title">{{.Title}}</span>
            {{if .Body}}<p class="body">{{.Body}}</p>{{end}}
        </li>
    {{else}}
        <li class="empty">No items on this page</li>
    {{end}}
    </ul>

    {{if .HasMore}}
    <button class="load-more">Load More</button>
    {{else}}
    <span class="end">End of list</span>
    {{end}}
</div>`

// ModalTemplate is the template for testing modal/panel state.
// It tests TreeNode-to-primitive transitions, conditional statics,
// and multiple conditionals changing simultaneously.
const ModalTemplate = `<div class="modal-container">
    {{if ne .ActivePanel "none"}}
    <div class="panel panel-{{.ActivePanel}}">
        <h3>{{.ActivePanel}} Panel</h3>
        <p>Panel content here</p>
    </div>
    {{end}}

    <div class="content">
        <p>Main content area</p>
    </div>

    {{if .HasOpenModal}}
    <div class="modal-backdrop">
        {{range .Modals}}
            <div data-modal-id="{{.ID}}" class="modal-wrapper">
            {{if .IsOpen}}
            <div class="modal priority-{{.Priority}}">
                <h2>{{.Title}}</h2>
                <p>{{.Content}}</p>
                <button>Close</button>
            </div>
            {{end}}
            </div>
        {{end}}
    </div>
    {{end}}

    <div class="modal-count">
        {{range .Modals}}
            <span class="modal-indicator {{if .IsOpen}}open{{else}}closed{{end}}">{{.ID}}</span>
        {{end}}
    </div>
</div>`

// FormTemplate is the template for testing form validation state.
// It tests error message appearing/disappearing (TreeNode transitions),
// multiple fields with different error states, and submit state changes.
const FormTemplate = `<form class="form {{if .Submitting}}submitting{{end}} {{if .Submitted}}submitted{{end}}">
    {{if .Submitted}}
    <div class="success-message">Form submitted successfully!</div>
    {{end}}

    {{if .HasErrors}}
    <div class="error-summary">Please fix the errors below</div>
    {{end}}

    {{range .Fields}}
    <div class="field {{if .HasError}}has-error{{end}} {{if .Touched}}touched{{end}}">
        <label>{{.Name}}</label>
        <input name="{{.Name}}" value="{{.Value}}" />
        {{if .HasError}}
        <span class="error">{{.Error}}</span>
        {{end}}
    </div>
    {{end}}

    {{if .Submitting}}
    <button disabled>Submitting...</button>
    {{else}}
    <button type="submit">Submit</button>
    {{end}}
</form>`

// AsyncTemplate is the template for testing async loading states.
// It tests loading→content→loading cycles, per-item loading states,
// and error state transitions.
const AsyncTemplate = `<div class="async-container">
    {{if .Loading}}
    <div class="global-loading">Loading...</div>
    {{end}}

    <ul class="items">
    {{range .Items}}
        <li id="{{.ID}}" class="{{if .Loading}}loading{{end}} {{if .HasError}}error{{end}}">
            {{if .Loading}}
            <span class="spinner">⏳</span>
            {{end}}

            <span class="title">{{.Title}}</span>

            {{if .HasError}}
            <span class="error-message">{{.Error}}</span>
            {{else}}
            <span class="status">{{if .Complete}}Done{{else}}Pending{{end}}</span>
            {{end}}
        </li>
    {{else}}
        <li class="empty">No items</li>
    {{end}}
    </ul>
</div>`

// NotificationTemplate is the template for testing notification queue.
// It tests add/dismiss operations, overflow handling, and auto-dismiss timers.
const NotificationTemplate = `<div class="notification-container">
    {{if .HasOverflow}}
    <div class="overflow-indicator">
        <span>+{{.OverflowCount}} more notifications</span>
    </div>
    {{end}}

    <ul class="notifications">
    {{range .Notifications}}
        <li id="notif-{{.ID}}" class="notification notification-{{.Type}}">
            <span class="message">{{.Message}}</span>
            {{if .AutoDismiss}}
            <span class="timer">{{.Timer}}s</span>
            {{end}}
            <button class="dismiss">×</button>
        </li>
    {{else}}
        <li class="empty">No notifications</li>
    {{end}}
    </ul>

    <div class="notification-footer">
        <span class="count">{{.TotalCount}} total</span>
        {{if gt .TotalCount 0}}
        <button class="dismiss-all">Dismiss All</button>
        {{end}}
    </div>
</div>`

// BulkTemplate is the template for testing bulk operations.
// It tests select all, bulk delete, batch updates, and selection management.
const BulkTemplate = `<div class="bulk-container">
    <div class="bulk-controls">
        <label class="select-all">
            <input type="checkbox" {{if .AllSelected}}checked{{end}} />
            Select All
        </label>
        {{if .HasSelection}}
        <span class="selected-count">{{.SelectedCount}} selected</span>
        <button class="bulk-delete">Delete Selected</button>
        <button class="bulk-complete">Mark Complete</button>
        {{end}}
    </div>

    <ul class="items">
    {{range .Items}}
        <li id="{{.ID}}" class="item {{if .IsSelected}}selected{{end}} {{if .Complete}}done{{end}}">
            <input type="checkbox" {{if .IsSelected}}checked{{end}} />
            <span class="title">{{.Title}}</span>
            <span class="priority priority-{{.Priority}}">{{.Priority}}</span>
        </li>
    {{else}}
        <li class="empty">No items</li>
    {{end}}
    </ul>

    <div class="bulk-footer">
        <span class="total">{{.TotalCount}} items</span>
    </div>
</div>`
