package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livefir/livetemplate"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

var validate = validator.New()

type TodoItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
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

type TodoState struct {
	Title          string     `json:"title"`
	Todos          []TodoItem `json:"todos"`
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

		// Generate unique ID
		id := fmt.Sprintf("todo-%d", time.Now().UnixNano())

		s.Todos = append(s.Todos, TodoItem{
			ID:        id,
			Text:      input.Text,
			Completed: false,
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

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Todo App starting...")

	// Create initial state
	state := &TodoState{
		Title:       "Todo App",
		Todos:       []TodoItem{},
		LastUpdated: formatTime(),
	}
	state.updateStats()

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
