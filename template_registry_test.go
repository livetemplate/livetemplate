package livetemplate

import (
	"html/template"
	"testing"
)

func TestTemplateRegistry(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	// Test RegisterTemplate
	tmpl := template.Must(template.New("test").Parse("Hello {{.Name}}!"))
	err = app.RegisterTemplate("greeting", tmpl)
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// Test GetRegisteredTemplates
	names := app.GetRegisteredTemplates()
	if len(names) != 1 || names[0] != "greeting" {
		t.Errorf("Expected ['greeting'], got %v", names)
	}

	// Test NewPageFromTemplate
	data := map[string]interface{}{"Name": "World"}
	page, err := app.NewPageFromTemplate("greeting", data)
	if err != nil {
		t.Fatalf("Failed to create page from template: %v", err)
	}
	defer page.Close()

	// Test rendering
	html, err := page.Render()
	if err != nil {
		t.Fatalf("Failed to render page: %v", err)
	}

	expected := "Hello World!"
	if html != expected {
		t.Errorf("Expected %q, got %q", expected, html)
	}

	// Test error cases
	err = app.RegisterTemplate("", tmpl)
	if err == nil {
		t.Error("Expected error for empty template name")
	}

	err = app.RegisterTemplate("nil-template", nil)
	if err == nil {
		t.Error("Expected error for nil template")
	}

	_, err = app.NewPageFromTemplate("non-existent", data)
	if err == nil {
		t.Error("Expected error for non-existent template")
	}
}

func TestRegisterTemplateFromFile(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	// Test with non-existent file
	err = app.RegisterTemplateFromFile("missing", "non-existent-file.html")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
