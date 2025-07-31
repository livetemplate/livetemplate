package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/livefir/statetemplate"
)

// Example data structures
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Stats struct {
	UserCount    int `json:"user_count"`
	PostCount    int `json:"post_count"`
	LastUpdate   string `json:"last_update"`
}

type AppData struct {
	Title       string `json:"title"`
	CurrentUser *User  `json:"current_user"`
	Stats       *Stats `json:"stats"`
}

// Create example template files for demonstration
func createExampleTemplates() error {
	// Create a temporary directory for templates
	templatesDir := "example-templates"
	err := os.MkdirAll(templatesDir, 0755)
	if err != nil {
		return err
	}

	// Create header template
	headerContent := `<header class="app-header">
    <h1>{{.Title}}</h1>
    {{if .CurrentUser}}
        <div class="user-welcome">
            <span>Welcome, {{.CurrentUser.Name}}!</span>
            <small>({{.CurrentUser.Role}})</small>
        </div>
    {{end}}
</header>`

	err = os.WriteFile(filepath.Join(templatesDir, "header.html"), []byte(headerContent), 0644)
	if err != nil {
		return err
	}

	// Create sidebar template
	sidebarContent := `<aside class="sidebar">
    <div class="stats-widget">
        <h3>Site Statistics</h3>
        <ul>
            <li>Users: {{.Stats.UserCount}}</li>
            <li>Posts: {{.Stats.PostCount}}</li>
            <li>Last Update: {{.Stats.LastUpdate}}</li>
        </ul>
    </div>
    
    {{if .CurrentUser}}
        <div class="user-profile">
            <h3>Your Profile</h3>
            <p>Name: {{.CurrentUser.Name}}</p>
            <p>Email: {{.CurrentUser.Email}}</p>
            <p>Role: {{.CurrentUser.Role}}</p>
        </div>
    {{end}}
</aside>`

	err = os.WriteFile(filepath.Join(templatesDir, "sidebar.html"), []byte(sidebarContent), 0644)
	if err != nil {
		return err
	}

	// Create footer template
	footerContent := `<footer class="app-footer">
    <p>&copy; 2025 My Application</p>
    <div class="footer-stats">
        <span>{{.Stats.UserCount}} users registered</span>
        <span>{{.Stats.PostCount}} posts published</span>
    </div>
</footer>`

	err = os.WriteFile(filepath.Join(templatesDir, "footer.html"), []byte(footerContent), 0644)
	if err != nil {
		return err
	}

	log.Printf("✅ Created example templates in %s/", templatesDir)
	return nil
}

func main() {
	log.Println("🔥 File Parsing Example")
	log.Println("=====================")

	// Create example template files
	err := createExampleTemplates()
	if err != nil {
		log.Fatalf("Failed to create example templates: %v", err)
	}

	// Defer cleanup
	defer func() {
		os.RemoveAll("example-templates")
		log.Println("🧹 Cleaned up example templates")
	}()

	// Create template tracker
	tracker := statetemplate.NewTemplateTracker()

	// Example 1: Load templates from directory
	log.Println("\n📁 Loading templates from directory...")
	err = tracker.AddTemplatesFromDirectory("example-templates", ".html")
	if err != nil {
		log.Fatalf("Failed to load templates from directory: %v", err)
	}

	// Show loaded templates
	templates := tracker.GetTemplates()
	log.Printf("✅ Loaded %d templates from directory:", len(templates))
	for name := range templates {
		log.Printf("   - %s", name)
	}

	// Show dependencies
	deps := tracker.GetDependencies()
	log.Println("\n🔍 Template Dependencies:")
	for templateName, templateDeps := range deps {
		if len(templateDeps) > 0 {
			log.Printf("   %s depends on:", templateName)
			for dep := range templateDeps {
				log.Printf("     - %s", dep)
			}
		}
	}

	// Example 2: Load specific files by name
	log.Println("\n📝 Loading specific templates by file path...")
	
	// Create a new tracker for this example
	tracker2 := statetemplate.NewTemplateTracker()
	
	fileMap := map[string]string{
		"my-header":  "example-templates/header.html",
		"my-sidebar": "example-templates/sidebar.html",
		"my-footer":  "example-templates/footer.html",
	}

	err = tracker2.AddTemplatesFromFiles(fileMap)
	if err != nil {
		log.Fatalf("Failed to load templates from files: %v", err)
	}

	templates2 := tracker2.GetTemplates()
	log.Printf("✅ Loaded %d templates by file mapping:", len(templates2))
	for name := range templates2 {
		log.Printf("   - %s", name)
	}

	// Set up live updates
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	// Start live update processor
	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Handle update notifications
	go func() {
		for update := range updateChannel {
			log.Printf("🔄 Templates needing re-render: %v", update.TemplateNames)
			log.Printf("   Changed fields: %v", update.ChangedFields)
		}
	}()

	// Simulate data updates
	go func() {
		defer close(dataChannel)

		// Initial data
		initialData := &AppData{
			Title: "File Parsing Demo",
			CurrentUser: &User{
				ID:    1,
				Name:  "Alice Johnson",
				Email: "alice@example.com",
				Role:  "Admin",
			},
			Stats: &Stats{
				UserCount:  150,
				PostCount:  75,
				LastUpdate: time.Now().Format("15:04:05"),
			},
		}

		log.Println("\n📊 Sending initial data...")
		dataChannel <- statetemplate.DataUpdate{Data: initialData}
		time.Sleep(2 * time.Second)

		// Update user info
		log.Println("👤 Updating user information...")
		updatedData := *initialData
		updatedData.CurrentUser = &User{
			ID:    1,
			Name:  "Alice Smith", // Changed name
			Email: "alice.smith@example.com", // Changed email
			Role:  "Super Admin", // Changed role
		}
		updatedData.Stats.LastUpdate = time.Now().Format("15:04:05")

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData}
		time.Sleep(2 * time.Second)

		// Update only stats
		log.Println("📈 Updating statistics...")
		updatedData2 := updatedData
		updatedData2.Stats = &Stats{
			UserCount:  160, // Increased
			PostCount:  82,  // Increased
			LastUpdate: time.Now().Format("15:04:05"),
		}

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData2}
		time.Sleep(2 * time.Second)

		// Update title
		log.Println("📝 Updating title...")
		updatedData3 := updatedData2
		updatedData3.Title = "Advanced File Parsing Demo" // Changed title

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData3}
	}()

	// Let it run
	time.Sleep(10 * time.Second)
	
	log.Println("\n✅ File parsing example completed!")
	log.Println("   This demonstrates loading templates from:")
	log.Println("   - Directory (with file extension filtering)")
	log.Println("   - Specific file mappings (custom names)")
	log.Println("   - Automatic dependency tracking from file contents")
}
