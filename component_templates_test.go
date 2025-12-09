package livetemplate

import (
	"bytes"
	"embed"
	"html/template"
	"testing"
)

// Test embedded templates for component template tests
//
//go:embed testdata/components/*.gotemplate
var testComponentFS embed.FS

// TestWithComponentTemplates_Basic tests basic component template registration
func TestWithComponentTemplates_Basic(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
	}

	// Create template with component templates and a project template
	tmpl, err := New("test",
		WithComponentTemplates(set),
		WithParseFiles("testdata/project/main.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// The component templates should be available
	if tmpl.tmpl == nil {
		t.Fatal("Template not parsed")
	}

	// Check that the component template is available
	componentTmpl := tmpl.tmpl.Lookup("test:greeting:v1")
	if componentTmpl == nil {
		t.Error("Component template 'test:greeting:v1' not found")
	}
}

// TestWithComponentTemplates_Override tests that project templates can override component templates
func TestWithComponentTemplates_Override(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
	}

	// Create template with component templates
	tmpl, err := New("test",
		WithComponentTemplates(set),
		WithParseFiles("testdata/project/override.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Execute the template that overrides the component
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	output := buf.String()
	// The output should contain the project's override, not the component's original
	if !bytes.Contains([]byte(output), []byte("Override Version")) {
		t.Errorf("Expected override template to be used, got: %s", output)
	}
}

// TestWithComponentTemplates_WithFuncs tests component templates with custom functions
func TestWithComponentTemplates_WithFuncs(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
		Funcs: template.FuncMap{
			"testFunc": func() string {
				return "custom-function-result"
			},
		},
	}

	// Create template with component templates and custom funcs
	tmpl, err := New("test",
		WithComponentTemplates(set),
		WithParseFiles("testdata/project/with_funcs.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Execute the template that uses the custom function
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("custom-function-result")) {
		t.Errorf("Expected custom function to be available, got: %s", output)
	}
}

// TestWithComponentTemplates_MultipleComponentSets tests multiple component sets
func TestWithComponentTemplates_MultipleComponentSets(t *testing.T) {
	set1 := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/greeting.gotemplate",
		Namespace: "greeting",
	}
	set2 := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/button.gotemplate",
		Namespace: "button",
	}

	// Create template with multiple component template sets
	tmpl, err := New("test",
		WithComponentTemplates(set1, set2),
		WithParseFiles("testdata/project/main.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Both component templates should be available
	greetingTmpl := tmpl.tmpl.Lookup("test:greeting:v1")
	if greetingTmpl == nil {
		t.Error("Component template 'test:greeting:v1' not found")
	}

	buttonTmpl := tmpl.tmpl.Lookup("test:button:v1")
	if buttonTmpl == nil {
		t.Error("Component template 'test:button:v1' not found")
	}
}

// TestWithComponentTemplates_NilSet tests that nil sets are skipped
func TestWithComponentTemplates_NilSet(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
	}

	// Create template with a nil set in the middle
	tmpl, err := New("test",
		WithComponentTemplates(nil, set, nil),
		WithParseFiles("testdata/project/main.gotemplate"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// The non-nil component template should still be available
	componentTmpl := tmpl.tmpl.Lookup("test:greeting:v1")
	if componentTmpl == nil {
		t.Error("Component template 'test:greeting:v1' not found")
	}
}

// TestWithComponentTemplates_InvalidPattern tests error handling for invalid patterns
func TestWithComponentTemplates_InvalidPattern(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/nonexistent/*.gotemplate",
		Namespace: "test",
	}

	_, err := New("test",
		WithComponentTemplates(set),
		WithParseFiles("testdata/project/main.gotemplate"),
	)
	if err == nil {
		t.Fatal("Expected error for non-matching pattern, got nil")
	}
}

// TestTemplateSet_Creation tests TemplateSet struct creation
func TestTemplateSet_Creation(t *testing.T) {
	set := &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
		Funcs: template.FuncMap{
			"myFunc": func() string { return "test" },
		},
	}

	if set.Namespace != "test" {
		t.Errorf("Namespace = %q, want %q", set.Namespace, "test")
	}
	if set.Pattern != "testdata/components/*.gotemplate" {
		t.Errorf("Pattern = %q, want %q", set.Pattern, "testdata/components/*.gotemplate")
	}
	if set.Funcs == nil {
		t.Error("Funcs should not be nil")
	}
}
