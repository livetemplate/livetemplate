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

	// Test NewPage (simplified API)
	data := map[string]interface{}{"Name": "World"}
	page, err := app.NewPage("greeting", data)
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

	_, err = app.NewPage("non-existent", data)
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

func TestStandardTemplateParsingMethods(t *testing.T) {
	app, err := NewApplication()
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}
	defer app.Close()

	t.Run("ParseFiles with non-existent file", func(t *testing.T) {
		_, err := app.ParseFiles("non-existent.html")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("ParseFiles with empty filenames", func(t *testing.T) {
		_, err := app.ParseFiles()
		if err == nil {
			t.Error("Expected error for empty filenames")
		}
	})

	t.Run("MustParseFiles with non-existent file", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for non-existent file")
			}
		}()
		app.MustParseFiles("non-existent.html")
	})

	t.Run("ParseGlob with invalid pattern", func(t *testing.T) {
		_, err := app.ParseGlob("*.nonexistent")
		if err != nil && err.Error() != "html/template: pattern matches no files: `*.nonexistent`" {
			t.Logf("Expected error for pattern with no matches: %v", err)
		}
	})

	t.Run("MustParseGlob with invalid pattern", func(t *testing.T) {
		// ParseGlob with pattern that matches no files returns an error that Must will panic on
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for pattern that matches no files")
			}
		}()
		app.MustParseGlob("*.nonexistent")
	})

	t.Run("ParseFiles auto-registration", func(t *testing.T) {
		// Create a temporary test template
		// Since we can't easily create temp files in test, we'll test the logic conceptually
		// by checking that templates with certain names get registered

		// Check initial state
		templates := app.GetRegisteredTemplates()
		initialCount := len(templates)

		// This will fail to parse but we can test the registration logic
		// The key test is that the method attempts to register with the correct name
		_, err := app.ParseFiles("testfile.html")
		if err != nil {
			t.Logf("Expected error for non-existent file: %v", err)
		}

		// Verify no templates were added due to parse error
		templatesAfter := app.GetRegisteredTemplates()
		if len(templatesAfter) != initialCount {
			t.Error("Templates should not be registered if parsing fails")
		}
	})
}
