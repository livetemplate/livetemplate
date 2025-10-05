package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectRoute(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a sample main.go
	mainGoContent := `package main

import (
	"log"
	"net/http"
	"os"

	"testapp/internal/database"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

func main() {
	log.Println("testapp starting...")

	// Initialize database
	dbPath := getDBPath()
	queries, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// TODO: Add routes here
	// Example: http.Handle("/users", users.Handler(queries))

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

func getDBPath() string {
	if os.Getenv("TEST_MODE") == "1" {
		return ":memory:"
	}
	return "app.db"
}
`

	mainGoPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("Failed to write test main.go: %v", err)
	}

	// Test injecting a route
	route := RouteInfo{
		Path:        "/users",
		PackageName: "users",
		HandlerCall: "users.Handler(queries)",
		ImportPath:  "testapp/internal/app/users",
	}

	if err := InjectRoute(mainGoPath, route); err != nil {
		t.Fatalf("InjectRoute failed: %v", err)
	}

	// Read result
	result, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}

	resultStr := string(result)

	// Debug: print result
	t.Logf("Result file contents:\n%s", resultStr)

	// Verify import was added
	if !strings.Contains(resultStr, `"testapp/internal/app/users"`) {
		t.Error("Import was not added")
	}

	// Verify route was added
	if !strings.Contains(resultStr, `http.Handle("/users", users.Handler(queries))`) {
		t.Error("Route was not added")
	}

	// Verify route is in correct location (after TODO)
	lines := strings.Split(resultStr, "\n")
	todoIndex := -1
	routeIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "TODO: Add routes here") {
			todoIndex = i
		}
		if strings.Contains(line, `http.Handle("/users"`) {
			routeIndex = i
		}
	}

	if todoIndex == -1 {
		t.Error("TODO comment not found")
	}
	if routeIndex == -1 {
		t.Error("Route not found")
	}
	if routeIndex <= todoIndex {
		t.Error("Route should be after TODO comment")
	}

	t.Log("✅ Route injection successful")
}

func TestInjectRoute_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	mainGoContent := `package main

import (
	"log"
	"net/http"
	"os"

	"testapp/internal/database"
	"testapp/internal/app/users"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

func main() {
	log.Println("testapp starting...")

	dbPath := getDBPath()
	queries, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// TODO: Add routes here
	// Example: http.Handle("/users", users.Handler(queries))
	http.Handle("/users", users.Handler(queries))

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

func getDBPath() string {
	if os.Getenv("TEST_MODE") == "1" {
		return ":memory:"
	}
	return "app.db"
}
`

	mainGoPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("Failed to write test main.go: %v", err)
	}

	// Try to inject the same route again
	route := RouteInfo{
		Path:        "/users",
		PackageName: "users",
		HandlerCall: "users.Handler(queries)",
		ImportPath:  "testapp/internal/app/users",
	}

	if err := InjectRoute(mainGoPath, route); err != nil {
		t.Fatalf("InjectRoute failed: %v", err)
	}

	// Read result
	result, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}

	t.Logf("Result after second injection:\n%s", string(result))

	// Count occurrences of the route (excluding comments)
	lines := strings.Split(string(result), "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, `http.Handle("/users", users.Handler(queries))`) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected route to appear exactly once (excluding comments), got %d occurrences", count)
	}

	t.Log("✅ Route injection is idempotent")
}

func TestInjectRoute_ViewHandler(t *testing.T) {
	tmpDir := t.TempDir()

	mainGoContent := `package main

import (
	"log"
	"net/http"
	"os"

	"testapp/internal/database"
	e2etest "github.com/livefir/livetemplate/internal/testing"
)

func main() {
	log.Println("testapp starting...")

	dbPath := getDBPath()
	queries, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// TODO: Add routes here

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

func getDBPath() string {
	if os.Getenv("TEST_MODE") == "1" {
		return ":memory:"
	}
	return "app.db"
}
`

	mainGoPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("Failed to write test main.go: %v", err)
	}

	// Test injecting a view route (no queries parameter)
	route := RouteInfo{
		Path:        "/counter",
		PackageName: "counter",
		HandlerCall: "counter.Handler()",
		ImportPath:  "testapp/internal/app/counter",
	}

	if err := InjectRoute(mainGoPath, route); err != nil {
		t.Fatalf("InjectRoute failed: %v", err)
	}

	result, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("Failed to read result: %v", err)
	}

	resultStr := string(result)

	// Verify import was added
	if !strings.Contains(resultStr, `"testapp/internal/app/counter"`) {
		t.Error("Import was not added")
	}

	// Verify route was added
	if !strings.Contains(resultStr, `http.Handle("/counter", counter.Handler())`) {
		t.Error("Route was not added")
	}

	t.Log("✅ View handler route injection successful")
}
