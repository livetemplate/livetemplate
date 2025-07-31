package statetemplate

import (
	"strings"
	"testing"
	"time"
)

func TestRealtimeRenderer(t *testing.T) {
	// Create renderer
	renderer := NewRealtimeRenderer(nil)

	// Test data structures
	type TestData struct {
		Counter int    `json:"counter"`
		Message string `json:"message"`
	}

	// Add a simple template
	template := `<div>Count: {{.Counter}}, Message: {{.Message}}</div>`
	err := renderer.AddTemplate("test", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test initial data
	initialData := &TestData{
		Counter: 1,
		Message: "Hello",
	}

	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	if fullHTML == "" {
		t.Error("Expected non-empty initial HTML")
	}

	t.Logf("Initial HTML: %s", fullHTML)
}

func TestRealtimeRendererFragmentExtraction(t *testing.T) {
	renderer := NewRealtimeRenderer(&RealtimeConfig{
		WrapperTag:     "div",
		IDPrefix:       "frag-",
		PreserveBlocks: true,
	})

	// Template with multiple fragments
	template := `<section>
		<h1>{{.Title}}</h1>
		<p>Count: {{.Count}}</p>
		{{block "footer" .}}
			<footer>{{.Footer}}</footer>
		{{end}}
	</section>`

	err := renderer.AddTemplate("multi", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	fragmentCount := renderer.GetFragmentCount()
	if fragmentCount == 0 {
		t.Error("Expected fragments to be extracted")
	}

	fragmentIDs := renderer.GetFragmentIDs()
	if len(fragmentIDs["multi"]) == 0 {
		t.Error("Expected fragment IDs for template")
	}

	t.Logf("Fragment count: %d", fragmentCount)
	t.Logf("Fragment IDs: %v", fragmentIDs)
}

func TestRealtimeRendererUpdates(t *testing.T) {
	renderer := NewRealtimeRenderer(nil)

	// Test data
	type TestData struct {
		Counter int    `json:"counter"`
		Message string `json:"message"`
	}

	// Add template
	template := `<div>
		<span>Counter: {{.Counter}}</span>
		<span>Message: {{.Message}}</span>
	</div>`

	err := renderer.AddTemplate("updates", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Set initial data
	initialData := &TestData{Counter: 1, Message: "Initial"}
	_, err = renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Start renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Send an update
	newData := &TestData{Counter: 2, Message: "Updated"}
	renderer.SendUpdate(newData)

	// Wait for update with timeout
	select {
	case update := <-updateChan:
		if update.FragmentID == "" {
			t.Error("Expected fragment ID in update")
		}
		if update.HTML == "" {
			t.Error("Expected HTML content in update")
		}
		if update.Action != "replace" {
			t.Errorf("Expected action 'replace', got %s", update.Action)
		}
		t.Logf("Received update: ID=%s, HTML=%s", update.FragmentID, update.HTML)

	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for update")
	}
}

func TestRealtimeRendererMultipleTemplates(t *testing.T) {
	renderer := NewRealtimeRenderer(nil)

	// Add multiple templates
	template1 := `<header>{{.Title}}</header>`
	template2 := `<main>{{.Content}}</main>`

	err := renderer.AddTemplate("header", template1)
	if err != nil {
		t.Fatalf("Failed to add header template: %v", err)
	}

	err = renderer.AddTemplate("main", template2)
	if err != nil {
		t.Fatalf("Failed to add main template: %v", err)
	}

	// Test data
	type TestData struct {
		Title   string
		Content string
	}

	initialData := &TestData{
		Title:   "My Title",
		Content: "My Content",
	}

	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	// Should contain both templates
	if !strings.Contains(fullHTML, "My Title") {
		t.Error("Expected title in full HTML")
	}
	if !strings.Contains(fullHTML, "My Content") {
		t.Error("Expected content in full HTML")
	}

	t.Logf("Full HTML with multiple templates: %s", fullHTML)
}

func TestRealtimeRendererBlockNames(t *testing.T) {
	renderer := NewRealtimeRenderer(&RealtimeConfig{
		PreserveBlocks: true,
	})

	// Template with named block
	template := `<div>
		{{block "sidebar" .}}
			<aside>{{.SidebarContent}}</aside>
		{{end}}
	</div>`

	err := renderer.AddTemplate("blocks", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	fragmentIDs := renderer.GetFragmentIDs()
	ids := fragmentIDs["blocks"]

	// Should have fragment IDs, potentially including block name
	if len(ids) == 0 {
		t.Error("Expected fragment IDs")
	}

	t.Logf("Fragment IDs for blocks template: %v", ids)
}

func TestRealtimeRendererChangeDetection(t *testing.T) {
	renderer := NewRealtimeRenderer(nil)

	// Test nested data structure
	type User struct {
		Name  string
		Email string
	}

	type TestData struct {
		Counter int
		User    *User
	}

	template := `<div>
		<span>Counter: {{.Counter}}</span>
		<span>User: {{.User.Name}} ({{.User.Email}})</span>
	</div>`

	err := renderer.AddTemplate("nested", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Set initial data
	initialData := &TestData{
		Counter: 1,
		User: &User{
			Name:  "John",
			Email: "john@example.com",
		},
	}

	_, err = renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	renderer.Start()
	defer renderer.Stop()

	updateChan := renderer.GetUpdateChannel()

	// Update only user name
	newData := &TestData{
		Counter: 1, // Same
		User: &User{
			Name:  "Jane", // Changed
			Email: "john@example.com",
		},
	}

	renderer.SendUpdate(newData)

	// Should receive update for fragments containing User.Name
	select {
	case update := <-updateChan:
		if !strings.Contains(update.HTML, "Jane") {
			t.Errorf("Expected updated HTML to contain 'Jane', got: %s", update.HTML)
		}
		t.Logf("Nested change detected: %s", update.HTML)

	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for nested change update")
	}
}

func TestRealtimeRendererNoChanges(t *testing.T) {
	renderer := NewRealtimeRenderer(nil)

	type TestData struct {
		Counter int
		Message string
	}

	template := `<div>{{.Counter}}: {{.Message}}</div>`
	err := renderer.AddTemplate("nochange", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	initialData := &TestData{Counter: 1, Message: "Hello"}
	_, err = renderer.SetInitialData(initialData)
	if err != nil {
		t.Fatalf("Failed to set initial data: %v", err)
	}

	renderer.Start()
	defer renderer.Stop()

	updateChan := renderer.GetUpdateChannel()

	// Send identical data (no changes)
	sameData := &TestData{Counter: 1, Message: "Hello"}
	renderer.SendUpdate(sameData)

	// Should not receive any updates
	select {
	case update := <-updateChan:
		t.Errorf("Unexpected update received: %+v", update)

	case <-time.After(1 * time.Second):
		// Expected - no update should be sent
		t.Log("No update received for identical data (expected)")
	}
}
