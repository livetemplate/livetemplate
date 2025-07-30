package main

import (
	"html/template"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Example data structures for demonstration
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type AppData struct {
	Title       string `json:"title"`
	CurrentUser *User  `json:"current_user"`
	UserCount   int    `json:"user_count"`
	LastUpdate  string `json:"last_update"`
}

// Example usage function
func main() {
	// Create template tracker
	tracker := statetemplate.NewTemplateTracker()

	// Define some templates
	headerTemplate := template.Must(template.New("header").Parse(`
		<header>
			<h1>{{.Title}}</h1>
			<p>Welcome, {{.CurrentUser.Name}}!</p>
		</header>
	`))

	sidebarTemplate := template.Must(template.New("sidebar").Parse(`
		<aside>
			<p>Total Users: {{.UserCount}}</p>
			<p>Last Update: {{.LastUpdate}}</p>
		</aside>
	`))

	userProfileTemplate := template.Must(template.New("user-profile").Parse(`
		<div class="profile">
			<h2>{{.CurrentUser.Name}}</h2>
			<p>Email: {{.CurrentUser.Email}}</p>
			<p>ID: {{.CurrentUser.ID}}</p>
		</div>
	`))

	// Add templates to tracker
	tracker.AddTemplate("header", headerTemplate)
	tracker.AddTemplate("sidebar", sidebarTemplate)
	tracker.AddTemplate("user-profile", userProfileTemplate)

	// Create channels
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	// Start the live update processor in a goroutine
	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Start a goroutine to handle template updates
	go func() {
		for update := range updateChannel {
			log.Printf("Templates requiring re-render: %v", update.TemplateNames)
			log.Printf("Changed fields: %v", update.ChangedFields)

			// Here you would typically trigger the actual re-rendering
			// and push updates to the client (e.g., via WebSocket)
		}
	}()

	// Simulate data updates
	go func() {
		defer close(dataChannel)

		// Initial data
		initialData := &AppData{
			Title: "My App",
			CurrentUser: &User{
				ID:    1,
				Name:  "John Doe",
				Email: "john@example.com",
			},
			UserCount:  100,
			LastUpdate: time.Now().Format("15:04:05"),
		}

		dataChannel <- statetemplate.DataUpdate{Data: initialData}
		time.Sleep(2 * time.Second)

		// Update user name (should trigger header and user-profile re-render)
		updatedData := *initialData
		updatedData.CurrentUser = &User{
			ID:    1,
			Name:  "Jane Doe", // Changed name
			Email: "john@example.com",
		}
		updatedData.LastUpdate = time.Now().Format("15:04:05")

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData}
		time.Sleep(2 * time.Second)

		// Update only user count (should trigger only sidebar re-render)
		updatedData2 := updatedData
		updatedData2.UserCount = 105 // Changed user count
		updatedData2.LastUpdate = time.Now().Format("15:04:05")

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData2}
		time.Sleep(2 * time.Second)

		// Update title (should trigger only header re-render)
		updatedData3 := updatedData2
		updatedData3.Title = "My Updated App" // Changed title
		updatedData3.LastUpdate = time.Now().Format("15:04:05")

		dataChannel <- statetemplate.DataUpdate{Data: &updatedData3}
	}()

	// Let it run for a while
	time.Sleep(10 * time.Second)
}
