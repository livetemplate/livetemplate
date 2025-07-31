package statetemplate

import (
	"fmt"
	"html/template"
	"os"
)

// RunAllTests manually runs all tests and reports results
func RunAllTests() {
	fmt.Println("🧪 Running All Tests...")

	// Test 1: Template Tracker
	fmt.Print("  Testing TemplateTracker... ")
	if testTemplateTracker() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 2: Change Detection
	fmt.Print("  Testing Change Detection... ")
	if testChangeDetection() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 3: Advanced Analyzer
	fmt.Print("  Testing Advanced Analyzer... ")
	if testAdvancedAnalyzer() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 4: Template Files Loading
	fmt.Print("  Testing Template Files Loading... ")
	if testTemplateFilesLoading() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 5: Directory Loading
	fmt.Print("  Testing Directory Loading... ")
	if testDirectoryLoading() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 6: Nested Templates
	fmt.Print("  Testing Nested Templates... ")
	if testNestedTemplates() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 7: Block Templates
	fmt.Print("  Testing Block Templates... ")
	if testBlockTemplates() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	// Test 8: Fragment Extraction
	fmt.Print("  Testing Fragment Extraction... ")
	if testFragmentExtraction() {
		fmt.Println("✅ PASS")
	} else {
		fmt.Println("❌ FAIL")
	}

	fmt.Println("\n🎉 All tests completed!")
}

func testTemplateTracker() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testTemplateTracker: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()
	analyzer := NewAdvancedTemplateAnalyzer()

	tmpl := mustParseTemplate("test", `<div><h1>{{.Title}}</h1><p>{{.User.Name}}</p></div>`)
	analyzer.UpdateTemplateTracker(tracker, "test", tmpl)

	deps := tracker.GetDependencies()["test"]

	// Check if expected dependencies are found
	expectedDeps := []string{"Title", "User.Name"}
	for _, expectedDep := range expectedDeps {
		if !deps[expectedDep] {
			fmt.Printf("Missing dependency: %s\n", expectedDep)
			return false
		}
	}

	return true
}

func testChangeDetection() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testChangeDetection: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	type TestData struct {
		Name  string
		Count int
	}

	oldData := TestData{Name: "Old", Count: 1}
	newData := TestData{Name: "New", Count: 1}

	changes := tracker.DetectChanges(oldData, newData)

	// Should detect that Name changed
	if len(changes) != 1 || changes[0] != "Name" {
		fmt.Printf("Expected ['Name'], got %v\n", changes)
		return false
	}

	return true
}

func testAdvancedAnalyzer() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testAdvancedAnalyzer: %v\n", r)
		}
	}()

	analyzer := NewAdvancedTemplateAnalyzer()

	tmpl := mustParseTemplate("advanced", `
		{{if .User}}
			<h1>{{.User.Name}}</h1>
			{{range .User.Orders}}
				<p>{{.ID}}</p>
			{{end}}
		{{end}}
		<footer>{{.Footer.Text}}</footer>
	`)

	deps := analyzer.AnalyzeTemplate(tmpl)

	// Should detect multiple dependencies
	expectedDeps := []string{"User", "User.Name", "User.Orders", "Footer.Text"}
	found := make(map[string]bool)
	for _, dep := range deps {
		found[dep] = true
	}

	for _, expected := range expectedDeps {
		if !found[expected] {
			fmt.Printf("Missing dependency: %s in %v\n", expected, deps)
			return false
		}
	}

	return true
}

func mustParseTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Parse(text))
}

func testTemplateFilesLoading() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testTemplateFilesLoading: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	// Test loading individual files
	testFiles := map[string]string{
		"simple":      "testdata/simple.tmpl",
		"conditional": "testdata/conditional.tpl",
		"header":      "testdata/header.html",
	}

	// Check if files exist
	for _, path := range testFiles {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("Test file %s does not exist\n", path)
			return false
		}
	}

	// Load templates from files
	err := tracker.AddTemplatesFromFiles(testFiles)
	if err != nil {
		fmt.Printf("Failed to load template files: %v\n", err)
		return false
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	if len(templates) != len(testFiles) {
		fmt.Printf("Expected %d templates, got %d\n", len(testFiles), len(templates))
		return false
	}

	// Check dependencies for header template
	deps := tracker.GetDependencies()
	headerDeps := deps["header"]
	expectedHeaderDeps := []string{"Title", "Author", "User", "User.Name", "User.Email", "Navigation"}

	for _, expected := range expectedHeaderDeps {
		if !headerDeps[expected] {
			fmt.Printf("Header template missing dependency: %s\n", expected)
			return false
		}
	}

	return true
}

func testDirectoryLoading() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testDirectoryLoading: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	// Check if testdata directory exists
	if _, err := os.Stat("testdata"); os.IsNotExist(err) {
		fmt.Printf("testdata directory does not exist\n")
		return false
	}

	// Load all templates from testdata directory
	err := tracker.AddTemplatesFromDirectory("testdata")
	if err != nil {
		fmt.Printf("Failed to load templates from directory: %v\n", err)
		return false
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	if len(templates) < 5 { // We created at least 5 template files
		fmt.Printf("Expected at least 5 templates, got %d\n", len(templates))
		return false
	}

	// Check some specific templates exist
	expectedTemplates := []string{"header", "footer", "sidebar", "content", "user-dashboard"}
	for _, expected := range expectedTemplates {
		if _, exists := templates[expected]; !exists {
			fmt.Printf("Expected template %s not found\n", expected)
			return false
		}
	}

	// Test complex dependencies in sidebar template
	deps := tracker.GetDependencies()
	sidebarDeps := deps["sidebar"]
	expectedSidebarDeps := []string{"Stats.UserCount", "Stats.PostCount", "User.Name", "User.Avatar", "RecentActivity"}

	foundCount := 0
	for _, expected := range expectedSidebarDeps {
		if sidebarDeps[expected] {
			foundCount++
		}
	}

	if foundCount < 3 { // At least 3 of the expected dependencies should be found
		fmt.Printf("Sidebar template found only %d of %d expected dependencies\n", foundCount, len(expectedSidebarDeps))
		return false
	}

	return true
}

func testNestedTemplates() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testNestedTemplates: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	// Test loading the nested template file
	err := tracker.AddTemplateFromFile("nested", "testdata/nested-templates.html")
	if err != nil {
		fmt.Printf("Failed to load nested template: %v\n", err)
		return false
	}

	// Verify template was loaded
	templates := tracker.GetTemplates()
	if _, exists := templates["nested"]; !exists {
		fmt.Printf("Nested template not found in loaded templates\n")
		return false
	}

	// Check dependencies for nested template
	deps := tracker.GetDependencies()
	nestedDeps := deps["nested"]

	// Should find dependencies from main template and nested definitions
	expectedDeps := []string{
		"Page.Title", "Site.Name", "Navigation.Items", "User.Name", "User.Role",
		"Article.Title", "Article.Author.Name", "Article.PublishedAt", "Article.Category",
		"Article.Body", "Article.Tags", "Comments", "Site.Copyright.Year", "Site.Copyright.Owner",
		"Footer.Links", "Performance.LoadTime", "Performance.MemoryUsage",
	}

	foundCount := 0
	for _, expected := range expectedDeps {
		if nestedDeps[expected] {
			foundCount++
		}
	}

	if foundCount < 5 {
		fmt.Printf("Nested template should have found at least 5 dependencies, found %d\n", foundCount)
		return false
	}

	return true
}

func testBlockTemplates() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testBlockTemplates: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	// Test loading the block template files
	blockFiles := map[string]string{
		"base-template": "testdata/block-template.html",
		"post-child":    "testdata/post-child.html",
	}

	err := tracker.AddTemplatesFromFiles(blockFiles)
	if err != nil {
		fmt.Printf("Failed to load block templates: %v\n", err)
		return false
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	if len(templates) != 2 {
		fmt.Printf("Expected 2 block templates, got %d\n", len(templates))
		return false
	}

	// Check dependencies for base template
	deps := tracker.GetDependencies()
	baseDeps := deps["base-template"]

	// Should find dependencies from blocks
	expectedBaseDeps := []string{
		"Site.DefaultTitle", "Site.DefaultDescription", "Site.Name", "Navigation.MainItems",
		"Sidebar.RecentPosts", "Sidebar.Categories", "User.Name", "User.Notifications.Count",
		"User.Stats.PostCount", "FeaturedPosts", "Copyright.Year", "BuildInfo.Version",
	}

	baseFoundCount := 0
	for _, expected := range expectedBaseDeps {
		if baseDeps[expected] {
			baseFoundCount++
		}
	}

	// Check dependencies for child template
	childDeps := deps["post-child"]
	expectedChildDeps := []string{
		"Post.Title", "Site.Name", "Post.MetaDescription", "Post.FeaturedImage",
		"Post.Author.Name", "Post.Author.Avatar", "Post.Content", "Post.Tags",
		"RelatedPosts", "Post.ID", "User.ID",
	}

	childFoundCount := 0
	for _, expected := range expectedChildDeps {
		if childDeps[expected] {
			childFoundCount++
		}
	}

	if baseFoundCount < 4 {
		fmt.Printf("Base template should have found at least 4 dependencies, found %d\n", baseFoundCount)
		return false
	}

	if childFoundCount < 4 {
		fmt.Printf("Child template should have found at least 4 dependencies, found %d\n", childFoundCount)
		return false
	}

	return true
}

func testFragmentExtraction() bool {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in testFragmentExtraction: %v\n", r)
		}
	}()

	tracker := NewTemplateTracker()

	// Test template with fragments that should be extracted
	templateContent := `
    <div>
        <div>
            Count updated: {{ .updated }} seconds ago
        </div>

        <hr />
        <div>
            Count: {{ .count }}
        </div>
        <button id="increment-btn">+</button>
        <button id="decrement-btn">-</button>
        
        <p>User: {{ .user.name }}</p>
    </div>`

	// Add template with fragment extraction
	tmpl, fragments, err := tracker.AddTemplateWithFragmentExtraction("test-fragments", templateContent)
	if err != nil {
		fmt.Printf("Failed to add template with fragments: %v\n", err)
		return false
	}

	// Verify template was created
	if tmpl == nil {
		fmt.Printf("Template should not be nil\n")
		return false
	}

	// Verify fragments were extracted
	if len(fragments) == 0 {
		fmt.Printf("Expected fragments to be extracted\n")
		return false
	}

	// Check that fragments have valid IDs and content
	for _, fragment := range fragments {
		if len(fragment.ID) != 6 {
			fmt.Printf("Fragment should have 6-character ID, got %d: %s\n", len(fragment.ID), fragment.ID)
			return false
		}

		if fragment.Content == "" {
			fmt.Printf("Fragment should have content\n")
			return false
		}

		if len(fragment.Dependencies) == 0 {
			fmt.Printf("Fragment should have dependencies\n")
			return false
		}
	}

	// Verify dependencies were stored for fragments
	deps := tracker.GetDependencies()
	for _, fragment := range fragments {
		fragmentDeps, exists := deps[fragment.ID]
		if !exists {
			fmt.Printf("Fragment %s should have dependencies stored\n", fragment.ID)
			return false
		}

		if len(fragmentDeps) == 0 {
			fmt.Printf("Fragment %s should have non-empty dependencies\n", fragment.ID)
			return false
		}
	}

	return true
}
