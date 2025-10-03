package main

import (
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
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

var validate = validator.New()

type TodoItem struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

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
	Title          string     `json:"title"`
	Todos          []TodoItem `json:"todos"`
	SearchQuery    string     `json:"search_query"`
	SortBy         string     `json:"sort_by"`
	FilteredTodos  []TodoItem `json:"filtered_todos"`
	CurrentPage    int        `json:"current_page"`
	PageSize       int        `json:"page_size"`
	TotalPages     int        `json:"total_pages"`
	PaginatedTodos []TodoItem `json:"paginated_todos"`
	TotalCount     int        `json:"total_count"`
	CompletedCount int        `json:"completed_count"`
	RemainingCount int        `json:"remaining_count"`
	LastUpdated    string     `json:"last_updated"`
}

func (s *TodoState) Change(ctx *livetemplate.ActionContext) error {
	switch ctx.Action {
	case "add":
		var input AddInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		// Generate unique ID and timestamp
		now := time.Now()
		id := fmt.Sprintf("todo-%d", now.UnixNano())

		s.Todos = append(s.Todos, TodoItem{
			ID:        id,
			Text:      input.Text,
			Completed: false,
			CreatedAt: now,
		})

	case "toggle":
		var input ToggleInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		for i := range s.Todos {
			if s.Todos[i].ID == input.ID {
				s.Todos[i].Completed = !s.Todos[i].Completed
				break
			}
		}

	case "delete":
		var input DeleteInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}

		for i, todo := range s.Todos {
			if todo.ID == input.ID {
				s.Todos = append(s.Todos[:i], s.Todos[i+1:]...)
				break
			}
		}

	case "search":
		var input SearchInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		s.SearchQuery = input.Query

	case "sort":
		var input SortInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		s.SortBy = input.SortBy

	case "next_page":
		if s.CurrentPage < s.TotalPages {
			s.CurrentPage++
		}

	case "prev_page":
		if s.CurrentPage > 1 {
			s.CurrentPage--
		}

	case "goto_page":
		var input PaginationInput
		if err := ctx.BindAndValidate(&input, validate); err != nil {
			return err
		}
		if input.Page >= 1 && input.Page <= s.TotalPages {
			s.CurrentPage = input.Page
		}

	case "clear_completed":
		remaining := []TodoItem{}
		for _, todo := range s.Todos {
			if !todo.Completed {
				remaining = append(remaining, todo)
			}
		}
		s.Todos = remaining

	default:
		log.Printf("Unknown action: %s", ctx.Action)
		return nil
	}

	// Update computed fields
	s.updateStats()
	s.updateFilteredTodos()
	s.LastUpdated = formatTime()
	return nil
}

func (s *TodoState) updateStats() {
	s.TotalCount = len(s.Todos)
	s.CompletedCount = 0

	for _, todo := range s.Todos {
		if todo.Completed {
			s.CompletedCount++
		}
	}

	s.RemainingCount = s.TotalCount - s.CompletedCount
}

func (s *TodoState) updateFilteredTodos() {
	if s.SearchQuery == "" {
		// Create a copy of Todos to avoid modifying the original when sorting
		s.FilteredTodos = make([]TodoItem, len(s.Todos))
		copy(s.FilteredTodos, s.Todos)
	} else {
		s.FilteredTodos = []TodoItem{}
		query := strings.ToLower(s.SearchQuery)
		for _, todo := range s.Todos {
			if strings.Contains(strings.ToLower(todo.Text), query) {
				s.FilteredTodos = append(s.FilteredTodos, todo)
			}
		}
	}

	s.applySorting()
	s.applyPagination()
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

	// Create initial state
	state := &TodoState{
		Title:       "Todo App",
		Todos:       []TodoItem{},
		CurrentPage: 1,
		PageSize:    3,
		LastUpdated: formatTime(),
	}
	state.updateStats()
	state.updateFilteredTodos()

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

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
