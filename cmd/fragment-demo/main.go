package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/livefir/statetemplate"
)

func main() {
	fmt.Println("🔥 Template Fragment Extraction Demo")
	fmt.Println("====================================")

	tracker := statetemplate.NewTemplateTracker()

	// Example template with multiple template expressions
	templateContent := `
    <div class="counter-app">
        <div class="status">
            Count updated: {{ .Updated }} seconds ago
        </div>

        <hr />
        
        <div class="counter">
            Count: {{ .Count }}
        </div>
        
        <div class="user-info">
            User: {{ .User.Name }}
        </div>
        
        <div class="stats">
            Total clicks: {{ .Stats.TotalClicks }}
        </div>
        
        <button id="increment-btn">+</button>
        <button id="decrement-btn">-</button>
    </div>`

	fmt.Println("📝 Original Template:")
	fmt.Printf("%s\n\n", templateContent)

	// Extract fragments automatically
	tmpl, fragments, err := tracker.AddTemplateWithFragmentExtraction("counter-app", templateContent)
	if err != nil {
		log.Fatalf("Failed to process template: %v", err)
	}

	fmt.Printf("✨ Extracted %d template fragments:\n", len(fragments))
	for i, fragment := range fragments {
		fmt.Printf("  %d. ID: %s\n", i+1, fragment.ID)
		fmt.Printf("     Content: %q\n", fragment.Content)
		fmt.Printf("     Dependencies: %v\n", fragment.Dependencies)
		fmt.Println()
	}

	// Show modified template with fragment calls
	fmt.Println("🔄 Modified Template (with fragment calls):")
	var buf bytes.Buffer
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		fmt.Printf("%s\n\n", tmpl.Tree.Root.String())
	}

	// Test data
	type User struct {
		Name string
	}

	type Stats struct {
		TotalClicks int
	}

	type AppData struct {
		Updated string
		Count   int
		User    User
		Stats   Stats
	}

	data := AppData{
		Updated: "2",
		Count:   42,
		User:    User{Name: "John Doe"},
		Stats:   Stats{TotalClicks: 156},
	}

	// Execute the template
	fmt.Println("🎯 Rendered Output:")
	err = tmpl.Execute(&buf, data)
	if err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
	fmt.Printf("%s\n\n", buf.String())

	// Show how dependencies work for change detection
	fmt.Println("🔍 Testing Change Detection:")

	// Simulate data changes and show which fragments would need re-rendering
	oldData := data
	newData := data
	newData.Count = 43    // Change count
	newData.Updated = "0" // Change updated time

	changes := tracker.DetectChanges(oldData, newData)
	fmt.Printf("Changed fields: %v\n", changes)

	// Find which fragments need re-rendering
	deps := tracker.GetDependencies()
	fmt.Println("Fragments that need re-rendering:")

	for _, fragment := range fragments {
		fragmentDeps := deps[fragment.ID]
		needsUpdate := false

		for _, changedField := range changes {
			if fragmentDeps[changedField] {
				needsUpdate = true
				break
			}
		}

		if needsUpdate {
			fmt.Printf("  ✅ Fragment %s: %q\n", fragment.ID, fragment.Content)
		} else {
			fmt.Printf("  ⏸️  Fragment %s: %q (no update needed)\n", fragment.ID, fragment.Content)
		}
	}

	fmt.Println("\n🚀 This enables extremely efficient partial re-rendering!")
	fmt.Println("   Only the fragments with changed dependencies need to be re-rendered.")
}
