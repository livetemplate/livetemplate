package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/livefir/statetemplate"
)

// Example data structures
type Counter struct {
	Value       int    `json:"value"`
	LastUpdated string `json:"last_updated"` // Use string instead of time.Time
	UpdateCount int    `json:"update_count"`
}

type Site struct {
	Name string `json:"name"`
}

type NavigationItem struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

type Navigation struct {
	MainItems []NavigationItem `json:"main_items"`
}

type PageData struct {
	Counter    *Counter    `json:"counter"`
	Site       *Site       `json:"site"`
	Navigation *Navigation `json:"navigation"`
}

func main() {
	log.Println("🌐 StateTemplate Real-time Web Example")
	log.Println("=====================================")

	// Create real-time renderer
	config := &statetemplate.RealtimeConfig{
		WrapperTag:     "div",
		IDPrefix:       "fragment-",
		PreserveBlocks: true,
	}
	renderer := statetemplate.NewRealtimeRenderer(config)

	// Define the template (same as in your example)
	templateContent := `<div>
	Current Count: {{.Counter.Value}}
	Last updated: {{.Counter.LastUpdated}}
	Total updates: {{.Counter.UpdateCount}}

	{{block "header" .}}
		<h1>{{.Site.Name}}</h1>
		<nav>
			{{range .Navigation.MainItems}}
				<a href="{{.URL}}">{{.Label}}</a>
			{{end}}
		</nav>
	{{end}}
</div>`

	// Add template
	if err := renderer.AddTemplate("main", templateContent); err != nil {
		log.Fatalf("Failed to add template: %v", err)
	}

	log.Printf("✅ Template added with %d fragments\n", renderer.GetFragmentCount())

	// Show fragment IDs and their dependencies
	fragmentIDs := renderer.GetFragmentIDs()
	fragmentDetails := renderer.GetFragmentDetails()
	for templateName, ids := range fragmentIDs {
		log.Printf("   Template '%s' has fragments: %v\n", templateName, ids)
		for _, fragmentID := range ids {
			if deps, ok := fragmentDetails[templateName][fragmentID]; ok {
				log.Printf("     Fragment '%s' depends on: %v\n", fragmentID, deps)
			}
		}
	}

	// Let's also check what the template analyzer detects
	tmpl, _ := template.New("debug").Parse(templateContent)
	analyzer := statetemplate.NewAdvancedTemplateAnalyzer()
	allDependencies := analyzer.AnalyzeTemplate(tmpl)
	log.Printf("   All template dependencies detected: %v\n", allDependencies)

	// Initial data
	initialData := &PageData{
		Counter: &Counter{
			Value:       42,
			LastUpdated: time.Now().Format("15:04:05"),
			UpdateCount: 0,
		},
		Site: &Site{
			Name: "My Awesome Site",
		},
		Navigation: &Navigation{
			MainItems: []NavigationItem{
				{URL: "/home", Label: "Home"},
				{URL: "/about", Label: "About"},
				{URL: "/contact", Label: "Contact"},
			},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		log.Fatalf("Failed to set initial data: %v", err)
	}

	log.Println("\n🎯 Initial Full HTML (ready to serve on page load):")
	log.Println("=" + strings.Repeat("=", 50))
	fmt.Println(fullHTML)
	log.Println("=" + strings.Repeat("=", 50))

	// Start the renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Listen for updates in a goroutine
	go func() {
		log.Println("\n📡 Listening for real-time updates...")
		for update := range updateChan {
			log.Printf("🔄 Fragment Update Received:")
			log.Printf("   Fragment ID: %s", update.FragmentID)
			log.Printf("   Action: %s", update.Action)
			log.Printf("   New HTML: %s", update.HTML)

			// In a real web application, you would send this to the client via WebSocket
			updateJSON, _ := json.MarshalIndent(update, "   ", "  ")
			log.Printf("   JSON for client: %s\n", updateJSON)
		}
	}()

	// Simulate real-time data changes
	log.Println("\n🚀 Simulating real-time data changes...")
	time.Sleep(1 * time.Second)

	// Change 1: Update counter value only
	log.Println("\n📊 Change 1: Updating counter value...")
	newData := *initialData
	newData.Counter = &Counter{
		Value:       43,
		LastUpdated: time.Now().Format("15:04:05"),
		UpdateCount: 1,
	}
	renderer.SendUpdate(&newData)
	time.Sleep(2 * time.Second)

	// Change 2: Update site name only
	log.Println("\n🏠 Change 2: Updating site name...")
	newData2 := newData
	newData2.Site = &Site{
		Name: "Updated Awesome Site",
	}
	renderer.SendUpdate(&newData2)
	time.Sleep(2 * time.Second)

	// Change 3: Add navigation item
	log.Println("\n🧭 Change 3: Adding navigation item...")
	newData3 := newData2
	newData3.Navigation = &Navigation{
		MainItems: []NavigationItem{
			{URL: "/home", Label: "Home"},
			{URL: "/about", Label: "About"},
			{URL: "/contact", Label: "Contact"},
			{URL: "/blog", Label: "Blog"}, // New item
		},
	}
	renderer.SendUpdate(&newData3)
	time.Sleep(2 * time.Second)

	// Change 4: Multiple changes at once
	log.Println("\n🔥 Change 4: Multiple updates at once...")
	newData4 := newData3
	newData4.Counter = &Counter{
		Value:       100,
		LastUpdated: time.Now().Format("15:04:05"),
		UpdateCount: 5,
	}
	newData4.Site = &Site{
		Name: "Super Awesome Site",
	}
	renderer.SendUpdate(&newData4)
	time.Sleep(2 * time.Second)

	log.Println("\n✨ Real-time rendering complete!")
	log.Println("\n📝 Summary:")
	log.Println("   • Only changed fragments are updated")
	log.Println("   • Each update includes fragment ID for targeting")
	log.Println("   • Updates are sent as JSON for easy client consumption")
	log.Println("   • Perfect for WebSocket-based real-time web applications")
}
