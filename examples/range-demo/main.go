package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Simple structures for demonstrating range operations
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ItemList struct {
	Items []Item `json:"items"`
}

func main() {
	log.Println("🔄 StateTemplate Range Fragment Demo")
	log.Println("===================================")

	// Create real-time renderer
	renderer := statetemplate.NewRenderer(
		statetemplate.WithWrapperTag("div"),
		statetemplate.WithIDPrefix("fragment-"),
		statetemplate.WithPreserveBlocks(true),
	)

	// Simple template with a range
	templateContent := `<div>
	<h2>Item List</h2>
	<ul>
		{{range .Items}}
			<li data-id="{{.ID}}">{{.Name}}</li>
		{{end}}
	</ul>
</div>`

	// Add template
	if err := renderer.Parse("main", templateContent); err != nil {
		log.Fatalf("Failed to add template: %v", err)
	}

	// Initial data with 2 items
	initialData := &ItemList{
		Items: []Item{
			{ID: "1", Name: "First Item"},
			{ID: "2", Name: "Second Item"},
		},
	}

	// Set initial data and get full HTML
	fullHTML, err := renderer.SetInitialData(initialData)
	if err != nil {
		log.Fatalf("Failed to set initial data: %v", err)
	}

	log.Println("\n🎯 Initial HTML with Range Items:")
	fmt.Println(fullHTML)

	// Start the renderer
	renderer.Start()
	defer renderer.Stop()

	// Get update channel
	updateChan := renderer.GetUpdateChannel()

	// Listen for updates
	go func() {
		log.Println("\n📡 Listening for range updates...")
		for update := range updateChan {
			log.Printf("🔄 Range Update Received:")
			log.Printf("   Fragment ID: %s", update.FragmentID)
			log.Printf("   Action: %s", update.Action)
			log.Printf("   HTML: %s", update.HTML)
			if update.RangeInfo != nil {
				log.Printf("   Item Key: %s", update.RangeInfo.ItemKey) //nolint:staticcheck
				if update.RangeInfo.ReferenceID != "" {                 //nolint:staticcheck
					log.Printf("   Reference ID: %s", update.RangeInfo.ReferenceID) //nolint:staticcheck
				}
			}

			updateJSON, _ := json.MarshalIndent(update, "   ", "  ")
			log.Printf("   JSON: %s\n", updateJSON)
		}
	}()

	time.Sleep(1 * time.Second)

	// Test 1: Add an item (should trigger append)
	log.Println("\n➕ Test 1: Adding a new item...")
	newData1 := &ItemList{
		Items: []Item{
			{ID: "1", Name: "First Item"},
			{ID: "2", Name: "Second Item"},
			{ID: "3", Name: "Third Item"}, // New item
		},
	}
	renderer.SendUpdate(newData1)
	time.Sleep(2 * time.Second)

	// Test 2: Remove an item (should trigger remove)
	log.Println("\n➖ Test 2: Removing an item...")
	newData2 := &ItemList{
		Items: []Item{
			{ID: "1", Name: "First Item"},
			{ID: "3", Name: "Third Item"}, // Removed ID: "2"
		},
	}
	renderer.SendUpdate(newData2)
	time.Sleep(2 * time.Second)

	// Test 3: Modify an item (should trigger replace)
	log.Println("\n✏️  Test 3: Modifying an item...")
	newData3 := &ItemList{
		Items: []Item{
			{ID: "1", Name: "Modified First Item"}, // Changed name
			{ID: "3", Name: "Third Item"},
		},
	}
	renderer.SendUpdate(newData3)
	time.Sleep(2 * time.Second)

	log.Println("\n✨ Range fragment demo complete!")
	log.Println("\n📝 Expected behaviors:")
	log.Println("   • Adding items should generate 'append' actions")
	log.Println("   • Removing items should generate 'remove' actions")
	log.Println("   • Modifying items should generate 'replace' actions")
	log.Println("   • Each action should target specific range item IDs")
}
