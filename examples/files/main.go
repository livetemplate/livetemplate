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
	UserCount  int    `json:"user_count"`
	PostCount  int    `json:"post_count"`
	LastUpdate string `json:"last_update"`
}

type AppData struct {
	Title       string `json:"title"`
	CurrentUser *User  `json:"current_user"`
	Stats       *Stats `json:"stats"`
}

// getTemplatesDir returns the correct path to the templates directory
// regardless of where the example is run from
func getTemplatesDir() string {
	// First, try relative to current working directory
	if _, err := os.Stat("templates"); err == nil {
		return "templates"
	}

	// If not found, try relative to project root
	if _, err := os.Stat("examples/files/templates"); err == nil {
		return "examples/files/templates"
	}

	// Default fallback
	return "templates"
}

func main() {
	log.Println("🔥 File Parsing Example")
	log.Println("=====================")

	// Create template tracker
	tracker := statetemplate.NewTemplateTracker()

	// Example 1: Load templates from directory
	log.Println("📁 Loading templates from directory...")
	templatesDir := getTemplatesDir() // Auto-detect correct templates path
	log.Printf("   Using templates directory: %s", templatesDir)
	err := tracker.AddTemplatesFromDirectory(templatesDir, ".html")
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
		"my-header":  filepath.Join(templatesDir, "header.html"),
		"my-sidebar": filepath.Join(templatesDir, "sidebar.html"),
		"my-footer":  filepath.Join(templatesDir, "footer.html"),
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
			Name:  "Alice Smith",             // Changed name
			Email: "alice.smith@example.com", // Changed email
			Role:  "Super Admin",             // Changed role
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
	log.Println()
	log.Println("🚀 This example demonstrates:")
	log.Println("   📁 Loading templates from a directory (with file extension filtering)")
	log.Println("   📝 Loading specific template files with custom names")
	log.Println("   🔍 Dependency analysis showing which data fields each template uses")
	log.Println("   ⚡ Live updates showing only affected templates when data changes")
	log.Println("   🎯 Real template files instead of programmatically generated ones")
}
