package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResourceGen_ExplicitTypes tests generating a resource with explicit type declarations
func TestResourceGen_ExplicitTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with explicit types
	t.Log("Generating products resource with explicit types...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "products",
		"name:string", "price:float", "quantity:int", "active:bool", "released_at:time"); err != nil {
		t.Fatalf("Failed to generate products: %v", err)
	}

	// Verify files created
	expectedFiles := []string{
		"internal/app/products/products.go",
		"internal/app/products/products.tmpl",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(appDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file not found: %s", file)
		}
	}

	// Verify migration contains all fields with correct types
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	schemaContent := string(schema)
	expectedInSchema := []string{
		"name TEXT",
		"price REAL",
		"quantity INTEGER",
		"active BOOLEAN",
		"released_at DATETIME",
	}

	for _, expected := range expectedInSchema {
		if !strings.Contains(schemaContent, expected) {
			t.Errorf("Schema missing expected field: %s", expected)
		}
	}

	t.Log("✅ Explicit types test passed")
}

// TestResourceGen_TypeInference tests generating a resource with type inference
func TestResourceGen_TypeInference(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with inferred types (no explicit :type)
	t.Log("Generating users resource with type inference...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "users",
		"name", "email", "age", "active", "created_at"); err != nil {
		t.Fatalf("Failed to generate users: %v", err)
	}

	// Verify schema contains inferred types
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	schemaContent := string(schema)
	// name -> string, email -> string, age -> int, active -> bool, created_at -> time
	expectedInSchema := []string{
		"name TEXT",           // inferred from "name"
		"email TEXT",          // inferred from "email"
		"age INTEGER",         // inferred from "age"
		"active BOOLEAN",      // inferred from "active"
		"created_at DATETIME", // inferred from "created_at"
	}

	for _, expected := range expectedInSchema {
		if !strings.Contains(schemaContent, expected) {
			t.Errorf("Schema missing inferred field: %s", expected)
		}
	}

	t.Log("✅ Type inference test passed")
}

// TestResourceGen_ForeignKey tests generating a resource with foreign key relationships
func TestResourceGen_ForeignKey(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate parent resource first
	if err := runLvtCommand(t, appDir, "gen", "resource", "authors", "name"); err != nil {
		t.Fatalf("Failed to generate authors: %v", err)
	}

	// Generate child resource with foreign key
	t.Log("Generating books resource with foreign key...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "books",
		"title", "author_id:references:authors"); err != nil {
		t.Fatalf("Failed to generate books: %v", err)
	}

	// Verify schema contains foreign key constraint
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	schemaContent := string(schema)
	if !strings.Contains(schemaContent, "FOREIGN KEY (author_id) REFERENCES authors(id)") {
		t.Error("Schema missing foreign key constraint")
	}

	t.Log("✅ Foreign key test passed")
}

// TestResourceGen_PaginationInfinite tests infinite scroll pagination (default)
func TestResourceGen_PaginationInfinite(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource (infinite is default)
	t.Log("Generating items resource with infinite pagination...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "items", "name"); err != nil {
		t.Fatalf("Failed to generate items: %v", err)
	}

	// Verify handler has infinite pagination mode
	handlerPath := filepath.Join(appDir, "internal/app/items/items.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	if !strings.Contains(string(handler), `PaginationMode: "infinite"`) {
		t.Error("Handler missing infinite pagination mode")
	}

	// Verify template has scroll sentinel
	tmplPath := filepath.Join(appDir, "internal/app/items/items.tmpl")
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	if !strings.Contains(string(tmpl), `id="scroll-sentinel"`) {
		t.Error("Template missing scroll-sentinel element")
	}

	t.Log("✅ Infinite pagination test passed")
}

// TestResourceGen_PaginationLoadMore tests load-more button pagination
func TestResourceGen_PaginationLoadMore(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with load-more pagination
	t.Log("Generating posts resource with load-more pagination...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "posts",
		"title", "--pagination", "load-more"); err != nil {
		t.Fatalf("Failed to generate posts: %v", err)
	}

	// Verify handler has load-more pagination mode
	handlerPath := filepath.Join(appDir, "internal/app/posts/posts.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	if !strings.Contains(string(handler), `PaginationMode: "load-more"`) {
		t.Error("Handler missing load-more pagination mode")
	}

	t.Log("✅ Load-more pagination test passed")
}

// TestResourceGen_PaginationPrevNext tests previous/next button pagination
func TestResourceGen_PaginationPrevNext(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with prev-next pagination
	t.Log("Generating articles resource with prev-next pagination...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "articles",
		"title", "--pagination", "prev-next", "--page-size", "10"); err != nil {
		t.Fatalf("Failed to generate articles: %v", err)
	}

	// Verify handler has prev-next pagination mode
	handlerPath := filepath.Join(appDir, "internal/app/articles/articles.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	handlerContent := string(handler)
	if !strings.Contains(handlerContent, `PaginationMode: "prev-next"`) {
		t.Error("Handler missing prev-next pagination mode")
	}

	// Verify page size (check for the number without quotes)
	if !strings.Contains(handlerContent, `PageSize:       10`) && !strings.Contains(handlerContent, `PageSize: 10`) {
		t.Error("Handler missing correct page size")
	}

	t.Log("✅ Prev-next pagination test passed")
}

// TestResourceGen_PaginationNumbers tests numbered pagination
func TestResourceGen_PaginationNumbers(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with numbers pagination
	t.Log("Generating entries resource with numbered pagination...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "entries",
		"title", "--pagination", "numbers"); err != nil {
		t.Fatalf("Failed to generate entries: %v", err)
	}

	// Verify handler has numbers pagination mode
	handlerPath := filepath.Join(appDir, "internal/app/entries/entries.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	if !strings.Contains(string(handler), `PaginationMode: "numbers"`) {
		t.Error("Handler missing numbers pagination mode")
	}

	t.Log("✅ Numbers pagination test passed")
}

// TestResourceGen_EditModeModal tests modal edit mode (default)
func TestResourceGen_EditModeModal(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource (modal is default)
	t.Log("Generating todos resource with modal edit mode...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "todos", "title"); err != nil {
		t.Fatalf("Failed to generate todos: %v", err)
	}

	// Verify template has modal elements
	tmplPath := filepath.Join(appDir, "internal/app/todos/todos.tmpl")
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	tmplContent := string(tmpl)
	if !strings.Contains(tmplContent, `lvt-modal-open="add-modal"`) {
		t.Error("Template missing modal for adding")
	}

	if !strings.Contains(tmplContent, `id="add-modal"`) && !strings.Contains(tmplContent, `id="edit-modal"`) {
		t.Error("Template missing modal elements")
	}

	t.Log("✅ Modal edit mode test passed")
}

// TestResourceGen_EditModePage tests page-based edit mode
func TestResourceGen_EditModePage(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with page edit mode
	t.Log("Generating notes resource with page edit mode...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "notes",
		"title", "--edit-mode", "page"); err != nil {
		t.Fatalf("Failed to generate notes: %v", err)
	}

	// Verify handler has page edit mode (check for URL routing logic)
	handlerPath := filepath.Join(appDir, "internal/app/notes/notes.go")
	handler, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("Failed to read handler: %v", err)
	}

	handlerContent := string(handler)
	// Page mode has URL routing logic and IsEditingMode field
	if !strings.Contains(handlerContent, `return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request)`) {
		t.Error("Handler missing page mode URL routing")
	}
	if !strings.Contains(handlerContent, `IsEditingMode:`) {
		t.Error("Handler missing IsEditingMode field (page mode)")
	}

	// Note: Both modal and page modes have the same template structure.
	// The difference is in the handler routing logic (checked above)

	t.Log("✅ Page edit mode test passed")
}

// TestResourceGen_TextareaFields tests text area field types
func TestResourceGen_TextareaFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with textarea fields (text type)
	t.Log("Generating docs resource with textarea fields...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "docs",
		"title", "content:text", "description:text"); err != nil {
		t.Fatalf("Failed to generate docs: %v", err)
	}

	// Verify template has textarea elements
	tmplPath := filepath.Join(appDir, "internal/app/docs/docs.tmpl")
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("Failed to read template: %v", err)
	}

	tmplContent := string(tmpl)
	contentTextareaCount := strings.Count(tmplContent, `<textarea`)
	if contentTextareaCount < 2 { // At least 2 textareas (content + description)
		t.Errorf("Expected at least 2 textarea elements, found %d", contentTextareaCount)
	}

	t.Log("✅ Textarea fields test passed")
}

// TestResourceGen_AllFieldTypes tests all supported field types in one resource
func TestResourceGen_AllFieldTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Build lvt binary

	// Create app
	appDir := createTestApp(t, tmpDir, "testapp", nil)

	// Generate resource with all field types
	t.Log("Generating records resource with all field types...")
	if err := runLvtCommand(t, appDir, "gen", "resource", "records",
		"name:string",
		"description:text",
		"count:int",
		"active:bool",
		"price:float",
		"created_at:time"); err != nil {
		t.Fatalf("Failed to generate records: %v", err)
	}

	// Verify schema contains all types correctly mapped
	schemaPath := filepath.Join(appDir, "internal/database/schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	schemaContent := string(schema)
	expectedMappings := map[string]string{
		"name":        "TEXT",
		"description": "TEXT",
		"count":       "INTEGER",
		"active":      "BOOLEAN",
		"price":       "REAL",
		"created_at":  "DATETIME",
	}

	for field, sqlType := range expectedMappings {
		if !strings.Contains(schemaContent, field+" "+sqlType) {
			t.Errorf("Schema missing correct mapping for %s -> %s", field, sqlType)
		}
	}

	// Verify handler was created
	handlerPath := filepath.Join(appDir, "internal/app/records/records.go")
	if _, err := os.Stat(handlerPath); os.IsNotExist(err) {
		t.Error("Handler file not created")
	}

	// Verify template was created
	tmplPath := filepath.Join(appDir, "internal/app/records/records.tmpl")
	if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
		t.Error("Template file not created")
	}

	t.Log("✅ All field types test passed")
}
