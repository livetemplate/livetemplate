package statetemplate

import (
	"html/template"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTemplateTracker(t *testing.T) {
	tracker := NewTemplateTracker()

	// Create a test template
	tmpl := template.Must(template.New("test").Parse(`
		<div>
			<h1>{{.Title}}</h1>
			<p>{{.User.Name}}</p>
			<span>{{.User.Email}}</span>
		</div>
	`))

	tracker.AddTemplate("test", tmpl)

	// Check if dependencies were extracted
	allDeps := tracker.GetDependencies()
	deps, exists := allDeps["test"]
	if !exists {
		t.Errorf("Template 'test' not found in dependencies")
		return
	}

	expectedDeps := []string{"Title", "User.Name", "User.Email"}

	for _, expectedDep := range expectedDeps {
		if !deps[expectedDep] {
			t.Errorf("Expected dependency %s not found", expectedDep)
		}
	}
}

func TestChangeDetection(t *testing.T) {
	tracker := NewTemplateTracker()

	type TestData struct {
		Name  string
		Count int
		User  struct {
			ID   int
			Name string
		}
	}

	oldData := TestData{
		Name:  "Old",
		Count: 1,
		User: struct {
			ID   int
			Name string
		}{ID: 1, Name: "John"},
	}

	newData := TestData{
		Name:  "New", // Changed
		Count: 1,     // Same
		User: struct {
			ID   int
			Name string
		}{ID: 1, Name: "Jane"}, // User.Name changed
	}

	changes := tracker.DetectChanges(oldData, newData)

	expectedChanges := []string{"Name", "User.Name"}

	if !reflect.DeepEqual(changes, expectedChanges) {
		t.Errorf("Expected changes %v, got %v", expectedChanges, changes)
	}
}

func TestAdvancedAnalyzer(t *testing.T) {
	analyzer := NewAdvancedTemplateAnalyzer()

	tmpl := template.Must(template.New("advanced").Parse(`
		{{if .User}}
			<h1>{{.User.Name}}</h1>
			{{range .User.Orders}}
				<p>{{.ID}}: {{.Amount}}</p>
			{{end}}
		{{end}}
		<footer>{{.Footer.Text}}</footer>
	`))

	deps := analyzer.AnalyzeTemplate(tmpl)

	expectedDeps := []string{"User", "User.Name", "User.Orders", "Footer.Text"}

	// Check if all expected dependencies are found
	for _, expected := range expectedDeps {
		found := false
		for _, dep := range deps {
			if dep == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected dependency %s not found in %v", expected, deps)
		}
	}
}

func TestLiveUpdates(t *testing.T) {
	tracker := NewTemplateTracker()

	// Add a test template
	tmpl := template.Must(template.New("live").Parse(`
		<div>{{.Message}}</div>
	`))
	tracker.AddTemplate("live", tmpl)

	dataChannel := make(chan DataUpdate, 2)
	updateChannel := make(chan TemplateUpdate, 2)

	// Start live updates in background
	go tracker.StartLiveUpdates(dataChannel, updateChannel)

	type TestData struct {
		Message string
	}

	// Send initial data
	dataChannel <- DataUpdate{Data: TestData{Message: "Hello"}}

	// Send updated data
	dataChannel <- DataUpdate{Data: TestData{Message: "World"}}

	close(dataChannel)

	// Check we get an update notification
	update := <-updateChannel

	if len(update.TemplateNames) != 1 || update.TemplateNames[0] != "live" {
		t.Errorf("Expected template 'live' to be updated, got %v", update.TemplateNames)
	}

	if len(update.ChangedFields) != 1 || update.ChangedFields[0] != "Message" {
		t.Errorf("Expected field 'Message' to be changed, got %v", update.ChangedFields)
	}
}

func TestTemplateTrackerFromFiles(t *testing.T) {
	// Create test directory and files
	tempDir := t.TempDir()

	// Create test templates
	templates := map[string]string{
		"header.html":  `<h1>{{.Title}}</h1><p>User: {{.User.Name}}</p>`,
		"footer.html":  `<footer>{{.Copyright}} - {{.Year}}</footer>`,
		"sidebar.html": `<div>{{.User.Email}} - {{.Stats.Count}}</div>`,
	}

	for filename, content := range templates {
		filepath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filepath, err)
		}
	}

	tracker := NewTemplateTracker()

	// Test AddTemplatesFromDirectory
	err := tracker.AddTemplatesFromDirectory(tempDir, ".html")
	if err != nil {
		t.Fatalf("Failed to load templates from directory: %v", err)
	}

	// Verify templates were loaded
	loadedTemplates := tracker.GetTemplates()
	if len(loadedTemplates) != 3 {
		t.Errorf("Expected 3 templates, got %d", len(loadedTemplates))
	}

	expectedTemplates := []string{"header", "footer", "sidebar"}
	for _, name := range expectedTemplates {
		if _, exists := loadedTemplates[name]; !exists {
			t.Errorf("Expected template %s to be loaded", name)
		}
	}

	// Test dependencies
	deps := tracker.GetDependencies()

	// Check header template dependencies
	headerDeps := deps["header"]
	if !headerDeps["Title"] || !headerDeps["User.Name"] {
		t.Errorf("Header template should depend on Title and User.Name, got %v", headerDeps)
	}

	// Check footer template dependencies
	footerDeps := deps["footer"]
	if !footerDeps["Copyright"] || !footerDeps["Year"] {
		t.Errorf("Footer template should depend on Copyright and Year, got %v", footerDeps)
	}
}

func TestTemplateTrackerFromFilesList(t *testing.T) {
	// Create test directory and files
	tempDir := t.TempDir()

	template1Path := filepath.Join(tempDir, "template1.html")
	template2Path := filepath.Join(tempDir, "template2.html")

	if err := os.WriteFile(template1Path, []byte(`<div>{{.Name}}</div>`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.WriteFile(template2Path, []byte(`<span>{{.Email}}</span>`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tracker := NewTemplateTracker()

	// Test AddTemplatesFromFiles
	files := map[string]string{
		"user-name":  template1Path,
		"user-email": template2Path,
	}

	err := tracker.AddTemplatesFromFiles(files)
	if err != nil {
		t.Fatalf("Failed to load templates from files: %v", err)
	}

	// Verify templates were loaded
	loadedTemplates := tracker.GetTemplates()
	if len(loadedTemplates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(loadedTemplates))
	}

	// Test individual template retrieval
	nameTemplate, exists := tracker.GetTemplate("user-name")
	if !exists {
		t.Error("Expected user-name template to exist")
	}
	if nameTemplate == nil {
		t.Error("Expected user-name template to not be nil")
	}

	emailTemplate, exists := tracker.GetTemplate("user-email")
	if !exists {
		t.Error("Expected user-email template to exist")
	}
	if emailTemplate == nil {
		t.Error("Expected user-email template to not be nil")
	}
}

func TestTemplateTrackerGetTemplate(t *testing.T) {
	tracker := NewTemplateTracker()

	// Add a test template
	tmpl := template.Must(template.New("test").Parse(`<div>{{.Message}}</div>`))
	tracker.AddTemplate("test-template", tmpl)

	// Test getting existing template
	retrieved, exists := tracker.GetTemplate("test-template")
	if !exists {
		t.Error("Expected template to exist")
	}
	if retrieved != tmpl {
		t.Error("Expected retrieved template to match original")
	}

	// Test getting non-existent template
	_, exists = tracker.GetTemplate("non-existent")
	if exists {
		t.Error("Expected non-existent template to not exist")
	}
}

func TestTemplateTrackerFromTestData(t *testing.T) {
	// Test loading templates from the testdata directory
	tracker := NewTemplateTracker()

	err := tracker.AddTemplatesFromDirectory("testdata")
	if err != nil {
		t.Fatalf("Failed to load templates from testdata: %v", err)
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	if len(templates) < 5 {
		t.Errorf("Expected at least 5 templates from testdata, got %d", len(templates))
	}

	// Check specific templates exist
	expectedTemplates := []string{"header", "footer", "sidebar", "content", "user-dashboard", "simple", "conditional"}
	for _, expected := range expectedTemplates {
		if _, exists := templates[expected]; !exists {
			t.Errorf("Expected template %s not found in loaded templates", expected)
		}
	}

	// Test dependencies for a complex template (sidebar)
	deps := tracker.GetDependencies()
	sidebarDeps := deps["sidebar"]

	// Should find various nested dependencies
	expectedDeps := []string{"Stats.UserCount", "User.Name", "User.Email", "RecentActivity"}
	foundCount := 0
	for _, expected := range expectedDeps {
		if sidebarDeps[expected] {
			foundCount++
		}
	}

	if foundCount < 2 {
		t.Errorf("Sidebar template should have found at least 2 dependencies, found %d", foundCount)
		t.Logf("All sidebar dependencies: %v", sidebarDeps)
	}
}

func TestNestedTemplateDefinitions(t *testing.T) {
	tracker := NewTemplateTracker()

	// Load the nested template file
	err := tracker.AddTemplateFromFile("nested", "testdata/nested-templates.html")
	if err != nil {
		t.Fatalf("Failed to load nested template: %v", err)
	}

	// Verify template was loaded
	templates := tracker.GetTemplates()
	if _, exists := templates["nested"]; !exists {
		t.Fatal("Nested template not found in loaded templates")
	}

	// Check dependencies
	deps := tracker.GetDependencies()
	nestedDeps := deps["nested"]

	// Should find dependencies from various parts of the nested template
	expectedDeps := []string{
		"Page.Title", "Site.Name", "Navigation.Items", "User.Name",
		"Article.Title", "Article.Author.Name", "Article.Body",
		"Comments", "Site.Copyright.Year", "Performance.LoadTime",
	}

	foundCount := 0
	for _, expected := range expectedDeps {
		if nestedDeps[expected] {
			foundCount++
		}
	}

	if foundCount < 5 {
		t.Errorf("Nested template should have found at least 5 dependencies, found %d", foundCount)
		t.Logf("Expected: %v", expectedDeps)
		t.Logf("Found dependencies: %v", nestedDeps)
	}
}

func TestBlockTemplates(t *testing.T) {
	tracker := NewTemplateTracker()

	// Load block template files
	blockFiles := map[string]string{
		"base-template": "testdata/block-template.html",
		"post-child":    "testdata/post-child.html",
	}

	err := tracker.AddTemplatesFromFiles(blockFiles)
	if err != nil {
		t.Fatalf("Failed to load block templates: %v", err)
	}

	// Verify templates were loaded
	templates := tracker.GetTemplates()
	if len(templates) != 2 {
		t.Errorf("Expected 2 block templates, got %d", len(templates))
	}

	deps := tracker.GetDependencies()

	// Test base template dependencies
	baseDeps := deps["base-template"]
	expectedBaseDeps := []string{
		"Site.DefaultTitle", "Site.Name", "Navigation.MainItems",
		"Sidebar.RecentPosts", "User.Name", "FeaturedPosts",
		"Copyright.Year", "BuildInfo.Version",
	}

	baseFoundCount := 0
	for _, expected := range expectedBaseDeps {
		if baseDeps[expected] {
			baseFoundCount++
		}
	}

	if baseFoundCount < 3 {
		t.Errorf("Base template should have found at least 3 dependencies, found %d", baseFoundCount)
		t.Logf("Base template dependencies: %v", baseDeps)
	}

	// Test child template dependencies
	childDeps := deps["post-child"]
	expectedChildDeps := []string{
		"Post.Title", "Post.MetaDescription", "Post.Author.Name",
		"Post.Content", "Post.Tags", "RelatedPosts",
	}

	childFoundCount := 0
	for _, expected := range expectedChildDeps {
		if childDeps[expected] {
			childFoundCount++
		}
	}

	if childFoundCount < 3 {
		t.Errorf("Child template should have found at least 3 dependencies, found %d", childFoundCount)
		t.Logf("Child template dependencies: %v", childDeps)
	}
}

func TestFragmentExtraction(t *testing.T) {
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
		t.Fatalf("Failed to add template with fragments: %v", err)
	}

	// Verify template was created
	if tmpl == nil {
		t.Fatal("Template should not be nil")
	}

	// Verify fragments were extracted
	if len(fragments) == 0 {
		t.Error("Expected fragments to be extracted")
	}

	// Check that fragments have valid IDs and content
	for i, fragment := range fragments {
		if len(fragment.ID) != 6 {
			t.Errorf("Fragment %d should have 6-character ID, got %d: %s", i, len(fragment.ID), fragment.ID)
		}

		if fragment.Content == "" {
			t.Errorf("Fragment %d should have content", i)
		}

		if len(fragment.Dependencies) == 0 {
			t.Errorf("Fragment %d should have dependencies", i)
		}

		t.Logf("Fragment %d: ID=%s, Content=%q, Dependencies=%v", i, fragment.ID, fragment.Content, fragment.Dependencies)
	}

	// Verify dependencies were stored for fragments
	deps := tracker.GetDependencies()
	for _, fragment := range fragments {
		fragmentDeps, exists := deps[fragment.ID]
		if !exists {
			t.Errorf("Fragment %s should have dependencies stored", fragment.ID)
		}

		if len(fragmentDeps) == 0 {
			t.Errorf("Fragment %s should have non-empty dependencies", fragment.ID)
		}
	}

	// Test retrieving fragments
	retrievedFragments, exists := tracker.GetFragments("test-fragments")
	if !exists {
		t.Error("Should be able to retrieve fragments for template")
	}

	if len(retrievedFragments) != len(fragments) {
		t.Errorf("Retrieved fragments count should match original: got %d, expected %d", len(retrievedFragments), len(fragments))
	}
}
