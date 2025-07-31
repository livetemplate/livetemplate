package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Example data structures
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Counter struct {
	Value       int    `json:"value"`
	LastUpdated string `json:"last_updated"`
	UpdateCount int    `json:"update_count"`
}

type Stats struct {
	TotalClicks   int `json:"total_clicks"`
	UniqueVisits  int `json:"unique_visits"`
	SessionLength int `json:"session_length"`
}

type AppData struct {
	Title   string   `json:"title"`
	User    *User    `json:"user"`
	Counter *Counter `json:"counter"`
	Stats   *Stats   `json:"stats"`
	Message string   `json:"message"`
}

func main() {
	log.Println("🔥 Fragment Extraction Example")
	log.Println("==============================")

	tracker := statetemplate.NewTemplateTracker()

	// Complex template with multiple data expressions
	templateContent := `
<div class="dashboard">
    <header class="header">
        <h1>{{.Title}}</h1>
        <p class="welcome">Welcome back, {{.User.Name}}!</p>
    </header>

    <div class="main-content">
        <div class="counter-section">
            <div class="counter-display">
                Current Count: {{.Counter.Value}}
            </div>
            <div class="counter-meta">
                Last updated: {{.Counter.LastUpdated}}
            </div>
            <div class="update-count">
                Total updates: {{.Counter.UpdateCount}}
            </div>
        </div>

        <div class="user-section">
            <div class="user-info">
                User: {{.User.Name}} ({{.User.Email}})
            </div>
            <div class="user-id">
                ID: {{.User.ID}}
            </div>
        </div>

        <div class="stats-section">
            <div class="clicks">
                Total clicks: {{.Stats.TotalClicks}}
            </div>
            <div class="visits">
                Unique visits: {{.Stats.UniqueVisits}}
            </div>
            <div class="session">
                Session length: {{.Stats.SessionLength}} minutes
            </div>
        </div>

        <div class="message-section">
            <div class="status-message">
                Status: {{.Message}}
            </div>
        </div>
    </div>

    <div class="static-content">
        <button id="increment">+</button>
        <button id="decrement">-</button>
        <p>These buttons don't contain template expressions, so they won't be extracted as fragments.</p>
    </div>
</div>`

	log.Println("📝 Original Template:")
	fmt.Printf("%s\n\n", templateContent)

	// Extract fragments automatically
	log.Println("🔧 Processing template with automatic fragment extraction...")
	tmpl, fragments, err := tracker.AddTemplateWithFragmentExtraction("dashboard", templateContent)
	if err != nil {
		log.Fatalf("Failed to process template: %v", err)
	}

	log.Printf("✨ Extracted %d template fragments:\n", len(fragments))
	for i, fragment := range fragments {
		log.Printf("  %d. Fragment ID: %s", i+1, fragment.ID)
		log.Printf("     Content: %q", fragment.Content)
		log.Printf("     Dependencies: %v", fragment.Dependencies)
		log.Printf("     Position: %d-%d\n", fragment.StartPos, fragment.EndPos)
	}

	// Show the modified template
	log.Println("🔄 Modified Template (with fragment calls):")
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		fmt.Printf("%s\n\n", tmpl.Tree.Root.String())
	}

	// Create test data
	testData := &AppData{
		Title: "Fragment Demo Dashboard",
		User: &User{
			ID:    42,
			Name:  "John Developer",
			Email: "john@example.com",
		},
		Counter: &Counter{
			Value:       15,
			LastUpdated: "2 seconds ago",
			UpdateCount: 23,
		},
		Stats: &Stats{
			TotalClicks:   1247,
			UniqueVisits:  89,
			SessionLength: 15,
		},
		Message: "All systems operational",
	}

	// Render the template
	log.Println("🎯 Rendering complete template:")
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, testData)
	if err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
	fmt.Printf("%s\n\n", buf.String())

	// Set up live updates for change detection
	dataChannel := make(chan statetemplate.DataUpdate, 10)
	updateChannel := make(chan statetemplate.TemplateUpdate, 10)

	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	go func() {
		for update := range updateChannel {
			log.Printf("🔄 Update notification:")
			log.Printf("   Templates to re-render: %v", update.TemplateNames)
			log.Printf("   Changed fields: %v", update.ChangedFields)
			
			// Show which fragments need updates
			deps := tracker.GetDependencies()
			log.Printf("   Fragments affected:")
			for _, fragment := range fragments {
				fragmentDeps := deps[fragment.ID]
				needsUpdate := false
				
				for _, changedField := range update.ChangedFields {
					if fragmentDeps[changedField] {
						needsUpdate = true
						break
					}
				}
				
				if needsUpdate {
					log.Printf("     ✅ %s: %q", fragment.ID, fragment.Content)
				}
			}
			log.Println()
		}
	}()

	// Simulate various data changes
	go func() {
		defer close(dataChannel)

		log.Println("📊 Simulating data changes...")
		
		// Initial data
		dataChannel <- statetemplate.DataUpdate{Data: testData}
		time.Sleep(2 * time.Second)

		// Change 1: Update only counter value
		log.Println("🔢 Changing counter value...")
		newData := *testData
		newData.Counter = &Counter{
			Value:       16, // Changed
			LastUpdated: "just now", // Changed
			UpdateCount: 24, // Changed
		}
		dataChannel <- statetemplate.DataUpdate{Data: &newData}
		time.Sleep(2 * time.Second)

		// Change 2: Update only user name
		log.Println("👤 Changing user name...")
		newData2 := newData
		newData2.User = &User{
			ID:    42,
			Name:  "Jane Developer", // Changed
			Email: "john@example.com",
		}
		dataChannel <- statetemplate.DataUpdate{Data: &newData2}
		time.Sleep(2 * time.Second)

		// Change 3: Update stats
		log.Println("📈 Updating statistics...")
		newData3 := newData2
		newData3.Stats = &Stats{
			TotalClicks:   1250, // Changed
			UniqueVisits:  91,   // Changed
			SessionLength: 15,
		}
		dataChannel <- statetemplate.DataUpdate{Data: &newData3}
		time.Sleep(2 * time.Second)

		// Change 4: Update message only
		log.Println("💬 Updating status message...")
		newData4 := newData3
		newData4.Message = "System update in progress" // Changed
		dataChannel <- statetemplate.DataUpdate{Data: &newData4}
	}()

	// Let the simulation run
	time.Sleep(10 * time.Second)
	
	log.Println("✅ Fragment extraction example completed!")
	log.Println()
	log.Println("🚀 Key Benefits Demonstrated:")
	log.Println("   ✨ Automatic extraction of minimal HTML fragments")
	log.Println("   🎯 Each fragment tracks only its specific dependencies")
	log.Println("   ⚡ Only affected fragments need re-rendering on data changes")
	log.Println("   🔧 No manual template restructuring required")
	log.Println("   📊 Efficient change detection at fragment granularity")
}
