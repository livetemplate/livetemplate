package statetemplate

import (
	"fmt"
	"html/template"
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
