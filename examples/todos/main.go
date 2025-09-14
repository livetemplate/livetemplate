package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/livefir/livetemplate"
)

// Todo represents a single todo item
type Todo struct {
	ID        string    `json:"ID"`
	Text      string    `json:"Text"`
	Completed bool      `json:"Completed"`
	CreatedAt time.Time `json:"CreatedAt"`
}

// TodoList represents the todo list data model
type TodoList struct {
	mu        sync.RWMutex
	Todos     []Todo `json:"Todos"`
	InputText string `json:"InputText"`
	ErrorText string `json:"ErrorText"`
	ShowError bool   `json:"ShowError"`
}

// NewTodoList creates a new todo list with initial state
func NewTodoList() *TodoList {
	return &TodoList{
		Todos:     []Todo{},
		InputText: "",
		ErrorText: "",
		ShowError: false,
	}
}

// AddTodo adds a new todo item if validation passes
func (t *TodoList) AddTodo(ctx *livetemplate.ActionContext) error {
	input := ctx.GetString("todo-input")
	log.Printf("AddTodo received input: %s", input)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear previous errors when user tries again
	t.ErrorText = ""
	t.ShowError = false

	// Validate input
	if len(input) < 3 {
		t.ErrorText = "Todo must be at least 3 characters long"
		t.ShowError = true
		// Keep the input text so user can edit it
		t.InputText = input
		return ctx.Data(t.toMap())
	}

	// Add the todo
	newTodo := Todo{
		ID:        uuid.New().String(),
		Text:      input,
		Completed: false,
		CreatedAt: time.Now(),
	}
	t.Todos = append(t.Todos, newTodo)

	// Clear input and error
	t.InputText = ""
	t.ErrorText = ""
	t.ShowError = false

	data := t.toMap()
	log.Printf("AddTodo returning data: %+v", data)
	return ctx.Data(data)
}

// RemoveTodo removes a todo item by ID
func (t *TodoList) RemoveTodo(ctx *livetemplate.ActionContext) error {
	todoID := ctx.GetString("todo-id")
	log.Printf("RemoveTodo called with todo-id: '%s'", todoID)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find and remove the todo
	initialCount := len(t.Todos)
	newTodos := []Todo{}
	for _, todo := range t.Todos {
		if todo.ID != todoID {
			newTodos = append(newTodos, todo)
		}
	}
	t.Todos = newTodos

	log.Printf("RemoveTodo: removed todo, count changed from %d to %d", initialCount, len(t.Todos))

	data := t.toMap()
	log.Printf("RemoveTodo returning data with %d todos", len(t.Todos))
	return ctx.Data(data)
}

// ToggleTodo toggles the completed status of a todo
func (t *TodoList) ToggleTodo(ctx *livetemplate.ActionContext) error {
	todoID := ctx.GetString("todo-id")

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find and toggle the todo
	for i := range t.Todos {
		if t.Todos[i].ID == todoID {
			t.Todos[i].Completed = !t.Todos[i].Completed
			break
		}
	}

	return ctx.Data(t.toMap())
}

// toMap converts the todo list to a map for template rendering (not exported, internal use)
func (t *TodoList) toMap() map[string]any {
	// Pass Todo structs directly - LiveTemplate works better with structs than converted maps
	return map[string]any{
		"Todos":     t.Todos,
		"InputText": t.InputText,
		"ErrorText": t.ErrorText,
		"ShowError": t.ShowError,
		"TodoCount": len(t.Todos),
	}
}

// ToMap converts the todo list to a map for template rendering (exported for external use)
func (t *TodoList) ToMap() map[string]any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.toMap()
}

type Server struct {
	app          *livetemplate.Application
	todoList     *TodoList
	templatePage *livetemplate.ApplicationPage
}

func NewServer() *Server {
	app, err := livetemplate.NewApplication()
	if err != nil {
		log.Fatal(err)
	}

	// Parse and auto-register the template
	_, err = app.ParseFiles("templates/index.html")
	if err != nil {
		log.Fatal("Failed to parse template:", err)
	}

	todoList := NewTodoList()

	// Create a template page with stable token
	templatePage, err := app.NewPage("index", todoList.ToMap())
	if err != nil {
		log.Fatal("Failed to create template page:", err)
	}

	// Register todoList as a data model with actions
	err = templatePage.RegisterDataModel(todoList)
	if err != nil {
		log.Fatal("Failed to register todo list data model:", err)
	}
	log.Printf("Registered todo list data model with actions")

	server := &Server{
		app:          app,
		todoList:     todoList,
		templatePage: templatePage,
	}

	log.Printf("Created template page with stable token: %s", templatePage.GetToken())

	return server
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := s.todoList.ToMap()
	// Add the token to the template data so client can connect to WebSocket
	data["Token"] = s.templatePage.GetToken()

	log.Printf("HTTP render with %d todos", len(s.todoList.Todos))

	// Render and serve the page
	if err := s.templatePage.ServeHTTP(w, data); err != nil {
		log.Printf("Error serving page: %v", err)
		http.Error(w, "Failed to serve page", http.StatusInternalServerError)
		return
	}

	log.Printf("Served page with stable token: %s", s.templatePage.GetToken())
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from: %s", r.RemoteAddr)

	// Get page from request
	page, err := s.app.GetPage(r)
	if err != nil {
		log.Printf("Failed to get page from WebSocket request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get page: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("WebSocket page retrieved successfully")

	// Upgrade to WebSocket
	upgrader := &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		HandshakeTimeout: 10 * time.Second,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	log.Printf("WebSocket connected with actions registered: %t", page.HasActions())

	// Set connection timeouts and handle cleanup
	defer func() {
		conn.Close()
		log.Printf("WebSocket connection closed")
	}()

	// Set read/write timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Handle messages with proper error checking
	for {
		var message livetemplate.ActionMessage

		// Reset read deadline on each message
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		err := conn.ReadJSON(&message)
		if err != nil {
			// Check if this is a normal close or timeout
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		log.Printf("Processing action: %s", message.Action)

		fragmentMap, err := page.HandleAction(context.Background(), &message)
		if err != nil {
			log.Printf("Action handler error: %v", err)
			continue
		}

		log.Printf("Generated %d fragment updates", len(fragmentMap))

		// Set write deadline and send response
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := conn.WriteJSON(fragmentMap); err != nil {
			log.Printf("WebSocket send error: %v", err)
			break
		}
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := NewServer()

	http.HandleFunc("/", server.handleHome)
	http.HandleFunc("/ws", server.handleWebSocket)

	// Serve the bundled LiveTemplate client library
	http.HandleFunc("/dist/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Serving bundled client to: %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.StripPrefix("/dist/", http.FileServer(http.Dir("../../client/dist/"))).ServeHTTP(w, r)
	})

	fmt.Printf("Todo app running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
