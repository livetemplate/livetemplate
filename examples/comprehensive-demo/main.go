package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Comprehensive data structures for testing all template actions
type User struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	IsActive bool     `json:"is_active"`
	Profile  *Profile `json:"profile,omitempty"`
}

type Profile struct {
	Bio     string `json:"bio"`
	Website string `json:"website,omitempty"`
}

type AppData struct {
	Title             string `json:"title"`
	CurrentUser       *User  `json:"current_user,omitempty"`
	Users             []User `json:"users"`
	ShowUserList      bool   `json:"show_user_list"`
	IsLoggedIn        bool   `json:"is_logged_in"`
	NotificationCount int    `json:"notification_count"`
}

func main() {
	log.Println("🌟 StateTemplate Comprehensive Template Actions Demo")
	log.Println("==================================================")

	// Create real-time renderer
	config := &statetemplate.Config{
		WrapperTag:     "div",
		IDPrefix:       "fragment-",
		PreserveBlocks: true,
	}
	renderer := statetemplate.NewRenderer(config)

	// Comprehensive template showcasing all actions
	templateContent := `<div>
	<h1>{{.Title}}</h1>
	
	<!-- Simple field output -->
	<p>App Status: Active</p>
	
	<!-- If/Else conditional -->
	{{if .IsLoggedIn}}
		<div class="user-section">
			<h2>Welcome back!</h2>
			{{if .CurrentUser}}
				<p>Hello, {{.CurrentUser.Name}}!</p>
			{{else}}
				<p>Hello, Guest!</p>
			{{end}}
		</div>
	{{else}}
		<div class="login-section">
			<h2>Please log in</h2>
			<button>Login</button>
		</div>
	{{end}}
	
	<!-- With context block -->
	{{with .CurrentUser}}
		<section class="current-user">
			<h3>Current User: {{.Name}}</h3>
			<p>Status: {{if .IsActive}}Active{{else}}Inactive{{end}}</p>
			{{with .Profile}}
				<div class="profile">
					<p>Bio: {{.Bio}}</p>
					{{if .Website}}
						<p>Website: <a href="{{.Website}}">{{.Website}}</a></p>
					{{end}}
				</div>
			{{else}}
				<p>No profile information available</p>
			{{end}}
		</section>
	{{end}}
	
	<!-- Range loop with conditionals -->
	{{if .ShowUserList}}
		<div class="users-list">
			<h2>All Users ({{len .Users}})</h2>
			{{if .Users}}
				<ul>
					{{range .Users}}
						<li data-id="{{.ID}}">
							{{.Name}} 
							{{if .IsActive}}
								<span class="status active">●</span>
							{{else}}
								<span class="status inactive">○</span>
							{{end}}
						</li>
					{{end}}
				</ul>
			{{else}}
				<p>No users found</p>
			{{end}}
		</div>
	{{end}}
	
	<!-- Notifications with conditional -->
	{{if gt .NotificationCount 0}}
		<div class="notifications">
			<p>You have {{.NotificationCount}} notifications</p>
		</div>
	{{end}}
</div>`

	// Add template
	if err := renderer.AddTemplate("comprehensive", templateContent); err != nil {
		log.Fatalf("Failed to add template: %v", err)
	}

	// Initial data showcasing various states
	initialData := &AppData{
		Title:             "Fragment Demo App",
		IsLoggedIn:        true,
		ShowUserList:      true,
		NotificationCount: 3,
		CurrentUser: &User{
			ID:       "user1",
			Name:     "Alice Johnson",
			IsActive: true,
			Profile: &Profile{
				Bio:     "Full-stack developer passionate about real-time web applications",
				Website: "https://alice.dev",
			},
		},
		Users: []User{
			{ID: "user1", Name: "Alice Johnson", IsActive: true},
			{ID: "user2", Name: "Bob Smith", IsActive: false},
			{ID: "user3", Name: "Carol White", IsActive: true},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		log.Fatalf("Failed to set initial data: %v", err)
	}

	log.Println("\n🎯 Initial HTML with All Template Actions:")
	fmt.Println(fullHTML)

	// Check fragment information
	fragmentDetails := renderer.GetFragmentDetails()
	fragmentIDs := renderer.GetFragmentIDs()

	log.Printf("\n📊 Fragment Analysis:")
	log.Printf("   Total fragments: %d", renderer.GetFragmentCount())
	log.Printf("   Fragment IDs: %+v", fragmentIDs)
	log.Printf("   Fragment details: %+v", fragmentDetails)

	// Start the renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Listen for updates
	go func() {
		log.Println("\n📡 Listening for comprehensive template updates...")
		updateCount := 0
		for update := range updateChan {
			updateCount++
			log.Printf("🔄 Update #%d Received:", updateCount)
			log.Printf("   Fragment ID: %s", update.FragmentID)
			log.Printf("   Action: %s", update.Action)
			log.Printf("   HTML Length: %d characters", len(update.HTML))
			if update.RangeInfo != nil {
				log.Printf("   Item Key: %s", update.RangeInfo.ItemKey)
				if update.RangeInfo.ReferenceID != "" {
					log.Printf("   Reference ID: %s", update.RangeInfo.ReferenceID)
				}
			}

			updateJSON, _ := json.MarshalIndent(update, "   ", "  ")
			log.Printf("   JSON: %s\n", updateJSON)
		}
	}()

	time.Sleep(1 * time.Second)

	// Test 1: Toggle login status (affects if/else blocks)
	log.Println("\n🧪 Test 1: Toggle login status...")
	newData1 := *initialData
	newData1.IsLoggedIn = false
	newData1.CurrentUser = nil
	renderer.SendUpdate(&newData1)
	time.Sleep(2 * time.Second)

	// Test 2: Add user profile (affects with blocks)
	log.Println("\n🧪 Test 2: Log in with different user...")
	newData2 := *initialData
	newData2.IsLoggedIn = true
	newData2.CurrentUser = &User{
		ID:       "user2",
		Name:     "Bob Smith",
		IsActive: false,
		Profile:  nil, // No profile to test else case
	}
	renderer.SendUpdate(&newData2)
	time.Sleep(2 * time.Second)

	// Test 3: Change notification count (affects conditional)
	log.Println("\n🧪 Test 3: Update notification count...")
	newData3 := *initialData
	newData3.NotificationCount = 0 // Should hide notifications
	renderer.SendUpdate(&newData3)
	time.Sleep(2 * time.Second)

	// Test 4: Toggle user list visibility
	log.Println("\n🧪 Test 4: Toggle user list visibility...")
	newData4 := *initialData
	newData4.ShowUserList = false
	renderer.SendUpdate(&newData4)
	time.Sleep(2 * time.Second)

	// Test 5: Modify user list (range + conditional changes)
	log.Println("\n🧪 Test 5: Modify user list...")
	newData5 := *initialData
	newData5.Users = []User{
		{ID: "user1", Name: "Alice Johnson", IsActive: false}, // Changed status
		{ID: "user4", Name: "David Brown", IsActive: true},    // New user
	}
	renderer.SendUpdate(&newData5)
	time.Sleep(2 * time.Second)

	log.Println("\n✨ Comprehensive template actions demo complete!")
	log.Println("\n📋 Template Actions Tested:")
	log.Println("   ✅ {{.Field}} - Simple field output")
	log.Println("   ✅ {{if condition}} {{else}} {{end}} - Conditional blocks")
	log.Println("   ✅ {{with .Object}} {{else}} {{end}} - Context blocks")
	log.Println("   ✅ {{range .Array}} {{end}} - Loop blocks")
	log.Println("   ✅ {{if gt .Count 0}} - Function calls with conditionals")
	log.Println("   ✅ Nested conditionals and complex combinations")
	log.Println("\n🎯 All major Go template actions now support granular fragments!")
}
