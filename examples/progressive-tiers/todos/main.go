// Progressive Complexity Demo: Todo App (Tier 1: Standard HTML)
//
// This demo shows LiveTemplate's progressive complexity model using
// ZERO lvt-* attributes. All action routing uses standard HTML:
//   - Form auto-submit (no attributes) → Submit() method
//   - button name="action" value="X" → X() method
//   - form name="X" → X() method
//
// Works at all three transport levels:
//   - No JS:         POST + PRG pattern (full page reload)
//   - JS + HTTP:     fetch POST + DOM patch
//   - JS + WebSocket: WS message + DOM patch
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/livetemplate/livetemplate"
)

var validate = validator.New()

// Todo represents a single todo item.
type Todo struct {
	ID    string
	Title string
	Done  bool
}

// TodoState is pure session data, cloned per user.
type TodoState struct {
	Items        []Todo
	ActiveFilter string
}

func (s TodoState) ActiveCount() int {
	count := 0
	for _, item := range s.Items {
		if !item.Done {
			count++
		}
	}
	return count
}

func (s TodoState) FilteredItems() []Todo {
	if s.ActiveFilter == "" || s.ActiveFilter == "all" {
		return s.Items
	}
	var filtered []Todo
	for _, item := range s.Items {
		if s.ActiveFilter == "active" && !item.Done {
			filtered = append(filtered, item)
		} else if s.ActiveFilter == "done" && item.Done {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// TodoController is a singleton holding dependencies.
type TodoController struct {
	Logger *slog.Logger
}

// Submit handles the default form submission (Tier 1: no lvt-submit attribute).
// The framework auto-routes forms without lvt-submit to this method.
func (c *TodoController) Submit(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input struct {
		Title string `json:"Title" validate:"required,min=1,max=200"`
	}
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.Items = append(state.Items, Todo{
		ID:    uuid.New().String()[:8],
		Title: input.Title,
	})
	c.Logger.Info("Todo added", slog.String("title", input.Title))
	return state, nil
}

// Toggle handles button name="action" value="toggle" (Tier 1: standard HTML routing).
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items[i].Done = !state.Items[i].Done
			break
		}
	}
	return state, nil
}

// Delete handles button name="action" value="delete" (Tier 1: standard HTML routing).
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	state.Items = slices.DeleteFunc(state.Items, func(t Todo) bool { return t.ID == id })
	return state, nil
}

// Filter handles form name="filter" (Tier 1: standard HTML routing).
func (c *TodoController) Filter(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.ActiveFilter = ctx.GetString("filter")
	return state, nil
}

func main() {
	controller := &TodoController{
		Logger: slog.Default(),
	}

	tmpl, err := livetemplate.New("todos")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpl.Parse(todoTemplate); err != nil {
		log.Fatal(err)
	}

	handler := tmpl.Handle(controller, livetemplate.AsState(&TodoState{
		Items: []Todo{
			{ID: "demo-1", Title: "Read the progressive complexity proposal", Done: false},
			{ID: "demo-2", Title: "Try zero-attribute forms", Done: false},
			{ID: "demo-3", Title: "Add lvt-* only when needed", Done: true},
		},
		ActiveFilter: "all",
	}))

	http.Handle("/", handler)

	addr := ":8080"
	fmt.Printf("Todo demo running at http://localhost%s\n", addr)
	fmt.Println("Uses ZERO lvt-* attributes — standard HTML only!")
	log.Fatal(http.ListenAndServe(addr, nil))
}

const todoTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Progressive Complexity: Todo Demo</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 0 20px; }
        h1 { color: #333; }
        .subtitle { color: #666; font-size: 14px; margin-top: -10px; }
        form { margin: 0; }
        .add-form { display: flex; gap: 8px; margin: 20px 0; }
        .add-form input[type="text"] { flex: 1; padding: 8px; border: 1px solid #ddd; border-radius: 4px; font-size: 16px; }
        .add-form button { padding: 8px 16px; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .error { color: #dc3545; font-size: 13px; margin-top: 4px; }
        ul { list-style: none; padding: 0; }
        li { display: flex; align-items: center; padding: 8px 0; border-bottom: 1px solid #eee; }
        li form { display: flex; align-items: center; width: 100%; }
        li span { flex: 1; }
        li .done { text-decoration: line-through; color: #999; }
        li button { margin-left: 8px; padding: 4px 8px; border: 1px solid #ddd; border-radius: 4px; background: white; cursor: pointer; font-size: 13px; }
        li button[value="delete"] { color: #dc3545; border-color: #dc3545; }
        li button[value="delete"]:hover { background: #dc3545; color: white; }
        .filters { display: flex; gap: 4px; margin: 16px 0; }
        .filters button { padding: 4px 12px; border: 1px solid #ddd; border-radius: 4px; background: white; cursor: pointer; }
        .filters button:hover { background: #f0f0f0; }
        .tier-label { font-size: 11px; color: #999; text-transform: uppercase; letter-spacing: 1px; margin-top: 24px; margin-bottom: 8px; }
    </style>
</head>
<body>
    <h1>Todos ({{.ActiveCount}} remaining)</h1>
    <p class="subtitle">Zero <code>lvt-*</code> attributes &mdash; standard HTML only</p>

    <!-- ==========================================
         TIER 1: Auto-submit form
         Works at all transport levels.
         ========================================== -->
    <div class="tier-label">Tier 1: auto-submit</div>
    <form method="POST" class="add-form">
        <input type="text" name="Title" placeholder="What needs to be done?" required>
        <button type="submit">Add</button>
    </form>
    {{if .lvt.HasError "title"}}
        <div class="error">{{.lvt.Error "title"}}</div>
    {{end}}

    <!-- ==========================================
         TIER 1: Standard HTML action routing
         button name="action" value="X" → X() method
         ========================================== -->
    <div class="tier-label">Tier 1: button name=&quot;action&quot;</div>
    <ul>
    {{range .FilteredItems}}
        <li data-key="{{.ID}}">
            <form method="POST">
                <input type="hidden" name="id" value="{{.ID}}">
                <span class="{{if .Done}}done{{end}}">{{.Title}}</span>
                <button type="submit" name="action" value="toggle">
                    {{if .Done}}Undo{{else}}Done{{end}}
                </button>
                <button type="submit" name="action" value="delete">Delete</button>
            </form>
        </li>
    {{end}}
    </ul>

    <!-- ==========================================
         TIER 1: Filter via form name
         form name="filter" → Filter() method
         ========================================== -->
    <div class="tier-label">Tier 1: form name</div>
    <form name="filter" method="POST" class="filters">
        <button type="submit" name="filter" value="all">All</button>
        <button type="submit" name="filter" value="active">Active</button>
        <button type="submit" name="filter" value="done">Done</button>
    </form>
</body>
</html>`
