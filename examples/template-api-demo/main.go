package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/livefir/statetemplate"
)

func main() {
	log.Println("🔄 StateTemplate HTML Template API Compatibility Demo")
	log.Println("====================================================")

	// Create renderer with functional options
	renderer := statetemplate.NewRenderer(
		statetemplate.WithWrapperTag("div"),
		statetemplate.WithIDPrefix("fragment-"),
		statetemplate.WithDebugMode(true),
	)

	// Demo 1: Parse - equivalent to template.Parse()
	log.Println("\n1️⃣  Demo: Parse method (equivalent to template.Parse)")
	templateContent := `<div>
	<h1>{{.Title}}</h1>
	<p>Welcome, {{.User.Name}}!</p>
	<p>You have {{.MessageCount}} messages.</p>
</div>`

	if err := renderer.Parse("main", templateContent); err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}
	log.Println("✅ Successfully parsed template using Parse method")

	// Demo 2: Create temporary template files for file-based parsing
	log.Println("\n2️⃣  Demo: File-based template parsing")

	// Create temp directory and files
	tempDir, err := os.MkdirTemp("", "statetemplate-demo")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Warning: failed to clean up temp directory: %v", err)
		}
	}()

	// Create sample template files
	headerTemplate := `<header>
	<h1>{{.Site.Name}}</h1>
	<nav>{{range .Navigation}}<a href="{{.URL}}">{{.Label}}</a>{{end}}</nav>
</header>`

	footerTemplate := `<footer>
	<p>&copy; {{.Site.Year}} {{.Site.Name}}</p>
</footer>`

	if err := os.WriteFile(filepath.Join(tempDir, "header.tmpl"), []byte(headerTemplate), 0644); err != nil {
		log.Fatalf("Failed to write header template: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "footer.tmpl"), []byte(footerTemplate), 0644); err != nil {
		log.Fatalf("Failed to write footer template: %v", err)
	}

	// Demo 3: ParseFiles - equivalent to template.ParseFiles()
	log.Println("\n3️⃣  Demo: ParseFiles method (equivalent to template.ParseFiles)")
	headerFile := filepath.Join(tempDir, "header.tmpl")
	footerFile := filepath.Join(tempDir, "footer.tmpl")

	if err := renderer.ParseFiles(headerFile, footerFile); err != nil {
		log.Fatalf("Failed to parse template files: %v", err)
	}
	log.Println("✅ Successfully parsed template files using ParseFiles method")

	// Demo 4: ParseGlob - equivalent to template.ParseGlob()
	log.Println("\n4️⃣  Demo: ParseGlob method (equivalent to template.ParseGlob)")

	// Create another template file
	sidebarTemplate := `<aside>
	<h3>Sidebar</h3>
	<ul>{{range .SidebarItems}}<li>{{.}}</li>{{end}}</ul>
</aside>`
	if err := os.WriteFile(filepath.Join(tempDir, "sidebar.tmpl"), []byte(sidebarTemplate), 0644); err != nil {
		log.Fatalf("Failed to write sidebar template: %v", err)
	}

	globPattern := filepath.Join(tempDir, "*.tmpl")

	// Create a new renderer for glob demo
	renderer2 := statetemplate.NewRenderer()
	if err := renderer2.ParseGlob(globPattern); err != nil {
		log.Fatalf("Failed to parse template glob: %v", err)
	}
	log.Println("✅ Successfully parsed template files using ParseGlob method")

	// Demo 5: Show stats from all parsed templates
	log.Println("\n5️⃣  Demo: Renderer statistics")
	stats := renderer.GetStats()
	log.Printf("📊 Template count: %d", stats.TemplateCount)
	log.Printf("📊 Total fragments: %d", stats.TotalFragments)
	log.Printf("📊 Fragments by type: %+v", stats.FragmentsByType)

	stats2 := renderer2.GetStats()
	log.Printf("📊 Glob renderer - Template count: %d", stats2.TemplateCount)
	log.Printf("📊 Glob renderer - Total fragments: %d", stats2.TotalFragments)

	// Demo 6: Render with sample data
	log.Println("\n6️⃣  Demo: Rendering with sample data")
	sampleData := map[string]interface{}{
		"Title":        "StateTemplate Demo",
		"User":         map[string]interface{}{"Name": "Alice"},
		"MessageCount": 5,
		"Site": map[string]interface{}{
			"Name": "Demo Site",
			"Year": 2024,
		},
		"Navigation": []map[string]interface{}{
			{"URL": "/home", "Label": "Home"},
			{"URL": "/about", "Label": "About"},
		},
		"SidebarItems": []string{"Item 1", "Item 2", "Item 3"},
	}

	html, err := renderer.SetInitialData(sampleData)
	if err != nil {
		log.Fatalf("Failed to render: %v", err)
	}

	log.Println("✅ Successfully rendered templates with fragment IDs:")
	fmt.Println("--- Rendered HTML ---")
	fmt.Println(html)
	fmt.Println("--- End of HTML ---")

	log.Println("\n🎉 Template API compatibility demo completed successfully!")
	log.Println("All standard html/template parsing methods are now supported:")
	log.Println("  • Parse(name, content)")
	log.Println("  • ParseFiles(filenames...)")
	log.Println("  • ParseGlob(pattern)")
	log.Println("  • ParseFS(fsys, patterns...) - also supported!")
}
