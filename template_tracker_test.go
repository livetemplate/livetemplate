package statetemplate

import (
	"html/template"
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
