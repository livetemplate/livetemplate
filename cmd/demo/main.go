package main

import (
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Demo-specific data structures
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

func main() {
	fmt.Println("🎯 State Template Live Update Demo")
	fmt.Println("===================================")

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

	// Add templates to tracker
	tracker.AddTemplate("header", headerTemplate)
	tracker.AddTemplate("sidebar", sidebarTemplate)

	// Create channels
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	// Start the live update processor
	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	// Handle template updates
	go func() {
		for update := range updateChannel {
			log.Printf("🔄 Templates requiring re-render: %v", update.TemplateNames)
			log.Printf("   Changed fields: %v", update.ChangedFields)
		}
	}()

	// Simulate data updates
	fmt.Println("\n📊 Sending data updates...")

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
	time.Sleep(1 * time.Second)

	// Update user name
	fmt.Println("   Updating user name...")
	updatedData := *initialData
	updatedData.CurrentUser = &User{
		ID:    1,
		Name:  "Jane Doe", // Changed
		Email: "john@example.com",
	}
	updatedData.LastUpdate = time.Now().Format("15:04:05")

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData}
	time.Sleep(1 * time.Second)

	// Update user count
	fmt.Println("   Updating user count...")
	updatedData2 := updatedData
	updatedData2.UserCount = 150 // Changed
	updatedData2.LastUpdate = time.Now().Format("15:04:05")

	dataChannel <- statetemplate.DataUpdate{Data: &updatedData2}

	close(dataChannel)
	time.Sleep(1 * time.Second)

	fmt.Println("\n✅ Demo completed!")
}
