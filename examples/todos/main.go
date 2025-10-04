package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livefir/livetemplate"
	"github.com/livefir/livetemplate/examples/todos/db"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

var validate = validator.New()

// TodoItem is an alias for the database model
type TodoItem = db.Todo

type AddInput struct {
	Text string `json:"text" validate:"required,min=3"`
}

type ToggleInput struct {
	ID string `json:"id" validate:"required"`
}

type DeleteInput struct {
	ID string `json:"id" validate:"required"`
}

type SearchInput struct {
	Query string `json:"query"`
}

type SortInput struct {
	SortBy string `json:"sort_by"`
}

type PaginationInput struct {
	Page int `json:"page" validate:"required,min=1"`
}

type TodoState struct {
	Title          string      `json:"title"`
	Queries        *db.Queries `json:"-"` // Database queries (exported but not in JSON)
	SearchQuery    string      `json:"search_query"`
	SortBy         string      `json:"sort_by"`
	FilteredTodos  []TodoItem  `json:"filtered_todos"`
	CurrentPage    int         `json:"current_page"`
	PageSize       int         `json:"page_size"`
	TotalPages     int         `json:"total_pages"`
	PaginatedTodos []TodoItem  `json:"paginated_todos"`
	TotalCount     int         `json:"total_count"`
	CompletedCount int         `json:"completed_count"`
	RemainingCount int         `json:"remaining_count"`
	LastUpdated    string      `json:"last_updated"`
}

func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
	dbCtx := context.Background()

	switch ctx.Action {
	case "add":
		var input AddInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		// Generate unique ID and timestamp
		now := time.Now()
		id := fmt.Sprintf("todo-%d", now.UnixNano())

		// Insert into database
		_, err := s.Queries.CreateTodo(dbCtx, db.CreateTodoParams{
			ID:        id,
			Text:      input.Text,
			Completed: false,
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("failed to create todo: %w", err)
		}

		// Reload todos from database
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "toggle":
		var input ToggleInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		// Get current todo to toggle its completed status
		todo, err := s.Queries.GetTodoByID(dbCtx, input.ID)
		if err != nil {
			return fmt.Errorf("failed to get todo: %w", err)
		}

		// Update in database
		err = s.Queries.UpdateTodoCompleted(dbCtx, db.UpdateTodoCompletedParams{
			Completed: !todo.Completed,
			ID:        input.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to update todo: %w", err)
		}

		// Reload todos from database
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "delete":
		var input DeleteInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		// Delete from database
		err := s.Queries.DeleteTodo(dbCtx, input.ID)
		if err != nil {
			return fmt.Errorf("failed to delete todo: %w", err)
		}

		// Reload todos from database
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "search":
		var input SearchInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		s.SearchQuery = input.Query

		// Reload todos with new search filter
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "sort":
		var input SortInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		s.SortBy = input.SortBy

		// Reload todos with new sort order
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "next_page":
		if s.CurrentPage < s.TotalPages {
			s.CurrentPage++
		}

		// Reload todos to update pagination
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "prev_page":
		if s.CurrentPage > 1 {
			s.CurrentPage--
		}

		// Reload todos to update pagination
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "goto_page":
		var input PaginationInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		if input.Page >= 1 && input.Page <= s.TotalPages {
			s.CurrentPage = input.Page
		}

		// Reload todos to update pagination
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	case "clear_completed":
		// Delete all completed todos from database
		err := s.Queries.DeleteCompletedTodos(dbCtx)
		if err != nil {
			return fmt.Errorf("failed to delete completed todos: %w", err)
		}

		// Reload todos from database
		if err := s.loadTodos(dbCtx); err != nil {
			return err
		}

	default:
		log.Printf("Unknown action: %s", ctx.Action)
		return nil
	}

	// Update timestamp
	s.LastUpdated = formatTime()
	return nil
}

// Init implements livetemplate.StoreInitializer
// This is called when the store is cloned for a new session (e.g., page refresh)
func (s *TodoState) Init() error {
	return s.loadTodos(context.Background())
}

// loadTodos loads todos from database and updates computed fields
func (s *TodoState) loadTodos(ctx context.Context) error {
	// Get all todos from database
	todos, err := s.Queries.GetAllTodos(ctx)
	if err != nil {
		return fmt.Errorf("failed to load todos: %w", err)
	}

	// Apply search filter
	if s.SearchQuery == "" {
		s.FilteredTodos = todos
	} else {
		s.FilteredTodos = []TodoItem{}
		query := strings.ToLower(s.SearchQuery)
		for _, todo := range todos {
			if strings.Contains(strings.ToLower(todo.Text), query) {
				s.FilteredTodos = append(s.FilteredTodos, todo)
			}
		}
	}

	// Update statistics
	s.TotalCount = len(todos)
	s.CompletedCount = 0
	for _, todo := range todos {
		if todo.Completed {
			s.CompletedCount++
		}
	}
	s.RemainingCount = s.TotalCount - s.CompletedCount

	// Apply sorting and pagination
	s.applySorting()
	s.applyPagination()

	return nil
}

func (s *TodoState) updateStats() {
	// Stats are now calculated in loadTodos
	// This is kept for backward compatibility but does nothing
}

func (s *TodoState) updateFilteredTodos() {
	// Filtering is now done in loadTodos
	// This is kept for backward compatibility but does nothing
}

func (s *TodoState) applySorting() {
	switch s.SortBy {
	case "alphabetical":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(s.FilteredTodos[i].Text) < strings.ToLower(s.FilteredTodos[j].Text)
		})
	case "reverse_alphabetical":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(s.FilteredTodos[i].Text) > strings.ToLower(s.FilteredTodos[j].Text)
		})
	case "oldest_first":
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return s.FilteredTodos[i].CreatedAt.Before(s.FilteredTodos[j].CreatedAt)
		})
	default:
		// Default: newest first (reverse chronological)
		sort.Slice(s.FilteredTodos, func(i, j int) bool {
			return s.FilteredTodos[i].CreatedAt.After(s.FilteredTodos[j].CreatedAt)
		})
	}
}

func (s *TodoState) applyPagination() {
	// Calculate total pages
	if len(s.FilteredTodos) == 0 {
		s.TotalPages = 1
		s.CurrentPage = 1
		s.PaginatedTodos = []TodoItem{}
		return
	}

	s.TotalPages = int(math.Ceil(float64(len(s.FilteredTodos)) / float64(s.PageSize)))

	// Validate and adjust current page if needed
	if s.CurrentPage < 1 {
		s.CurrentPage = 1
	}
	if s.CurrentPage > s.TotalPages {
		s.CurrentPage = s.TotalPages
	}

	// Calculate start and end indices for current page
	start := (s.CurrentPage - 1) * s.PageSize
	end := start + s.PageSize
	if end > len(s.FilteredTodos) {
		end = len(s.FilteredTodos)
	}

	// Slice to get current page items
	s.PaginatedTodos = s.FilteredTodos[start:end]
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Todo App starting...")

	// Initialize database
	dbPath := GetDBPath()
	queries, err := InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer CloseDB()

	// Create initial state
	state := &TodoState{
		Title:       "Todo App",
		Queries:     queries,
		CurrentPage: 1,
		PageSize:    3,
		LastUpdated: formatTime(),
	}

	// Load initial todos from database
	if err := state.loadTodos(context.Background()); err != nil {
		log.Fatalf("Failed to load initial todos: %v", err)
	}

	// Create template - auto-discovers todos.tmpl
	tmpl := livetemplate.New("todos")

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	http.Handle("/", tmpl.Handle(state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
