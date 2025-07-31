package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/livefir/statetemplate"
)

// Example data structures for file parsing demo
type FileUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type FileStats struct {
	UserCount  int    `json:"user_count"`
	PostCount  int    `json:"post_count"`
	LastUpdate string `json:"last_update"`
}

type FileAppData struct {
	Title string    `json:"title"`
	User  *FileUser `json:"user"`
	Stats FileStats `json:"stats"`
}

func main() {
	log.Println("🗂️  Template File Parsing Example")
	log.Println("=================================")

	tracker := statetemplate.NewTemplateTracker()

	// Example 1: Load templates from directory
	log.Println("📁 Loading templates from testdata directory...")
	err := tracker.AddTemplatesFromDirectory("../testdata", ".html", ".tmpl", ".tpl")
	if err != nil {
		log.Fatalf("Failed to load templates from directory: %v", err)
	}

	templates := tracker.GetTemplates()
	log.Printf("✅ Loaded %d templates from directory", len(templates))
	for name := range templates {
		log.Printf("   - %s", name)
	}

	// Example 2: Load specific template files
	log.Println("\n📄 Loading specific template files...")

	// Create a temporary template file for demonstration
	tempDir, err := os.MkdirTemp("", "template_example")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a custom template file
	customTemplate := `
<div class="custom-widget">
	<h3>{{.Title}}</h3>
	<div class="user-info">
		<p>User: {{.User.Name}}</p>
		<p>Email: {{.User.Email}}</p>
	</div>
	<div class="stats">
		<span>Users: {{.Stats.UserCount}}</span>
		<span>Posts: {{.Stats.PostCount}}</span>
		<span>Updated: {{.Stats.LastUpdate}}</span>
	</div>
</div>`

	customPath := filepath.Join(tempDir, "custom-widget.html")
	err = os.WriteFile(customPath, []byte(customTemplate), 0644)
	if err != nil {
		log.Fatalf("Failed to create custom template: %v", err)
	}

	// Load the custom template
	err = tracker.AddTemplateFromFile("custom-widget", customPath)
	if err != nil {
		log.Fatalf("Failed to load custom template: %v", err)
	}

	// Example 3: Load multiple specific files
	log.Println("\n📋 Loading multiple specific files...")

	// Create another template file
	listTemplate := `
<ul class="item-list">
{{range .Items}}
	<li>{{.Name}} - {{.Value}}</li>
{{end}}
</ul>`

	listPath := filepath.Join(tempDir, "item-list.html")
	err = os.WriteFile(listPath, []byte(listTemplate), 0644)
	if err != nil {
		log.Fatalf("Failed to create list template: %v", err)
	}

	// Load multiple files at once
	fileMap := map[string]string{
		"my-custom-widget": customPath,
		"my-item-list":     listPath,
	}

	err = tracker.AddTemplatesFromFiles(fileMap)
	if err != nil {
		log.Fatalf("Failed to load template files: %v", err)
	}

	// Show all loaded templates
	allTemplates := tracker.GetTemplates()
	log.Printf("✅ Total templates loaded: %d", len(allTemplates))

	// Show dependencies for some templates
	log.Println("\n🔍 Template Dependencies Analysis:")
	deps := tracker.GetDependencies()

	exampleTemplates := []string{"custom-widget", "header", "sidebar"}
	for _, templateName := range exampleTemplates {
		if templateDeps, exists := deps[templateName]; exists {
			log.Printf("📊 Template '%s' dependencies:", templateName)
			for dep := range templateDeps {
				log.Printf("   - %s", dep)
			}
		}
	}

	// Test live updates with file-based templates
	log.Println("\n🔄 Testing Live Updates with File-based Templates...")

	dataChannel := make(chan statetemplate.DataUpdate, 5)
	updateChannel := make(chan statetemplate.TemplateUpdate, 5)

	// Start live updates
	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Handle updates
	go func() {
		for update := range updateChannel {
			log.Printf("🔄 Templates needing re-render: %v", update.TemplateNames)
			log.Printf("   Changed fields: %v", update.ChangedFields)
		}
	}()

	// Send test data
	testData := &FileAppData{
		Title: "File-based Templates Demo",
		User: &FileUser{
			ID:    42,
			Name:  "Alice Johnson",
			Email: "alice@example.com",
		},
		Stats: FileStats{
			UserCount:  250,
			PostCount:  89,
			LastUpdate: time.Now().Format("15:04:05"),
		},
	}

	dataChannel <- statetemplate.DataUpdate{Data: testData}
	time.Sleep(1 * time.Second)

	// Update user info
	updatedData := *testData
	updatedData.User = &FileUser{
		ID:    42,
		Name:  "Alice Smith",             // Changed name
		Email: "alice.smith@example.com", // Changed email
	}
	updatedData.Stats.LastUpdate = time.Now().Format("15:04:05")

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData}
	time.Sleep(1 * time.Second)

	// Update stats only
	updatedData2 := updatedData
	updatedData2.Stats.UserCount = 255
	updatedData2.Stats.PostCount = 91
	updatedData2.Stats.LastUpdate = time.Now().Format("15:04:05")

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData2}
	time.Sleep(1 * time.Second)

	close(dataChannel)
	time.Sleep(500 * time.Millisecond) // Let updates finish

	log.Println("\n✨ File parsing example completed!")
	log.Println("💡 This demonstrates how to load templates from:")
	log.Println("   - Entire directories with file extension filtering")
	log.Println("   - Individual template files")
	log.Println("   - Multiple files with custom naming")
	log.Println("   - Automatic dependency analysis for all loaded templates")
}
