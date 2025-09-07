package main

import (
	"context"
	"fmt"
	"html/template"
	"log"

	"github.com/livefir/livetemplate"
)

func main() {
	// Parse the template
	tmpl, err := template.ParseFiles("examples/counter/templates/index.html")
	if err != nil {
		log.Fatal("Failed to parse template:", err)
	}

	// Create application and page
	app, err := livetemplate.NewApplication()
	if err != nil {
		log.Fatal("Failed to create application:", err)
	}
	defer app.Close()

	data := map[string]any{
		"Counter": 0,
		"Color":   "color-orange",
	}

	page, err := app.NewApplicationPage(tmpl, data)
	if err != nil {
		log.Fatal("Failed to create page:", err)
	}
	defer page.Close()

	// Render initial HTML to see the lvt-id assignments
	html, err := page.Render()
	if err != nil {
		log.Fatal("Failed to render:", err)
	}

	fmt.Println("=== INITIAL HTML ===")
	fmt.Println(html)
	fmt.Println()

	// Now test fragment generation
	newData := map[string]any{
		"Counter": 1,
		"Color":   "color-red",
	}

	fmt.Println("=== GENERATING FRAGMENTS ===")
	fmt.Printf("Old data: %+v\n", data)
	fmt.Printf("New data: %+v\n", newData)

	fragments, err := page.RenderFragments(context.Background(), newData)
	if err != nil {
		log.Fatal("Failed to generate fragments:", err)
	}

	fmt.Printf("Generated %d fragments:\n", len(fragments))
	for i, frag := range fragments {
		fmt.Printf("  Fragment %d: ID=%s, Data=%+v\n",
			i+1, frag.ID, frag.Data)
	}
}
