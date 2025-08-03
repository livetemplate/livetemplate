package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/livefir/statetemplate"
)

// Item represents a simple list item for range demonstrations
type RangeDemoItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RangeDemoData contains a list of items for range template testing
type RangeDemoData struct {
	Items []RangeDemoItem `json:"items"`
	Title string          `json:"title"`
}

// CreateRangeDemo creates and returns a range demonstration
func CreateRangeDemo() *RangeDemoData {
	return &RangeDemoData{
		Title: "Range Demo Items",
		Items: []RangeDemoItem{
			{ID: 1, Name: "Item One", Value: "Value A"},
			{ID: 2, Name: "Item Two", Value: "Value B"},
			{ID: 3, Name: "Item Three", Value: "Value C"},
		},
	}
}

// RunRangeDemo demonstrates range fragment functionality
func RunRangeDemo(renderer *statetemplate.RealtimeRenderer) {
	log.Println("🔄 Range Demo Starting...")

	// Template with range functionality
	rangeTemplate := `
		<div>
			<h2>{{.Title}}</h2>
			<ul>
			{{range .Items}}
				<li data-id="{{.ID}}">{{.Name}}: {{.Value}}</li>
			{{end}}
			</ul>
		</div>
	`

	// Add the range template
	err := renderer.AddTemplate("range_demo", rangeTemplate)
	if err != nil {
		log.Printf("Error adding range template: %v", err)
		return
	}

	// Create demo data
	demoData := CreateRangeDemo()

	// Set initial data
	_, err = renderer.SetInitialData(demoData)
	if err != nil {
		log.Printf("Error setting initial data: %v", err)
		return
	}

	// Simulate data updates
	go func() {
		time.Sleep(2 * time.Second)

		// Add a new item
		demoData.Items = append(demoData.Items, RangeDemoItem{
			ID:    4,
			Name:  "Item Four",
			Value: "Value D",
		})

		log.Println("📝 Adding new item to range...")
		renderer.SendUpdate(demoData)

		time.Sleep(2 * time.Second)

		// Modify an existing item
		if len(demoData.Items) > 0 {
			demoData.Items[0].Value = "Updated Value A"
			log.Println("✏️ Updating existing item...")
			renderer.SendUpdate(demoData)
		}
	}()

	log.Println("✅ Range demo setup completed")
}

// SerializeDemoData converts demo data to JSON for debugging
func SerializeDemoData(data *RangeDemoData) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "Error serializing data"
	}
	return string(jsonData)
}
