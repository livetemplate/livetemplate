package commands

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/livetemplate/livetemplate"
)

// Parse validates a template file and shows detailed information
func Parse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("template file required\nUsage: lvt parse <template-file>")
	}

	templateFile := args[0]

	// Check if file exists
	if _, err := os.Stat(templateFile); os.IsNotExist(err) {
		return fmt.Errorf("template file not found: %s", templateFile)
	}

	// Read template file
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	templateStr := string(content)
	baseName := filepath.Base(templateFile)
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	fmt.Printf("Parsing template: %s\n", templateFile)
	fmt.Printf("Template name: %s\n", name)
	fmt.Println(strings.Repeat("=", 60))

	// Test 1: Standard html/template parsing
	fmt.Println("\n1. Testing html/template parsing...")
	tmpl, err := template.New(name).Parse(templateStr)
	if err != nil {
		fmt.Printf("   ❌ Parse error: %v\n", err)
		return fmt.Errorf("template parse failed")
	}
	fmt.Println("   ✅ Successfully parsed with html/template")

	// List defined templates
	fmt.Println("\n2. Defined templates:")
	templates := tmpl.Templates()
	if len(templates) > 0 {
		for _, t := range templates {
			fmt.Printf("   - %s\n", t.Name())
		}
	} else {
		fmt.Println("   (no nested templates)")
	}

	// Test 2: LiveTemplate parsing
	fmt.Println("\n3. Testing LiveTemplate parsing...")
	lvtTmpl := livetemplate.New(name)
	_, err = lvtTmpl.Parse(templateStr)
	if err != nil {
		fmt.Printf("   ❌ LiveTemplate parse error: %v\n", err)
		return fmt.Errorf("LiveTemplate parse failed")
	}
	fmt.Println("   ✅ Successfully parsed with LiveTemplate")

	// Test 3: Try to execute with sample data
	fmt.Println("\n4. Testing template execution...")
	testData := map[string]interface{}{
		"Title":          "Test Title",
		"CSSFramework":   "tailwind",
		"DevMode":        true,
		"AppName":        "TestApp",
		"Resources":      []interface{}{},
		"LastUpdated":    "2025-01-01",
		"TotalCount":     0,
		"CurrentPage":    1,
		"TotalPages":     1,
		"SearchQuery":    "",
		"SortBy":         "",
		"PaginatedPosts": []interface{}{},
		"FilteredPosts":  []interface{}{},
	}

	var testBuf strings.Builder
	err = lvtTmpl.Execute(&testBuf, testData)
	if err != nil {
		fmt.Printf("   ⚠️  Execution error: %v\n", err)
		fmt.Println("   (This might be OK if template expects specific data structure)")
	} else {
		htmlLen := len(testBuf.String())
		fmt.Printf("   ✅ Successfully executed (generated %d bytes of HTML)\n", htmlLen)
	}

	// Test 4: Check for common issues
	fmt.Println("\n5. Checking for common issues...")
	issues := []string{}

	// Check for mismatched blocks
	blockCount := strings.Count(templateStr, "{{block")
	endCount := strings.Count(templateStr, "{{end}}")
	if blockCount != endCount {
		issues = append(issues, fmt.Sprintf("Mismatched {{block}}/{{end}} count: %d blocks, %d ends", blockCount, endCount))
	}

	// Check for mismatched define
	defineCount := strings.Count(templateStr, "{{define")
	if defineCount > 0 && defineCount != endCount-blockCount {
		issues = append(issues, fmt.Sprintf("Potential {{define}}/{{end}} mismatch: %d defines", defineCount))
	}

	// Check for unclosed tags
	if strings.Count(templateStr, "{{") != strings.Count(templateStr, "}}") {
		issues = append(issues, "Unclosed template tags ({{ without }})")
	}

	// Check for template invocations without defines
	hasDefine := strings.Contains(templateStr, "{{define")
	hasTemplate := strings.Contains(templateStr, "{{template")
	if hasTemplate && !hasDefine {
		issues = append(issues, "Template invocations found but no {{define}} blocks")
	}

	if len(issues) > 0 {
		fmt.Println("   ⚠️  Potential issues found:")
		for _, issue := range issues {
			fmt.Printf("   - %s\n", issue)
		}
	} else {
		fmt.Println("   ✅ No common issues detected")
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ Template validation complete")
	fmt.Println("\nTemplate structure:")
	fmt.Printf("  - Size: %d bytes\n", len(templateStr))
	fmt.Printf("  - Lines: %d\n", strings.Count(templateStr, "\n")+1)
	fmt.Printf("  - Defines: %d\n", defineCount)
	fmt.Printf("  - Blocks: %d\n", blockCount)
	fmt.Printf("  - Template invocations: %d\n", strings.Count(templateStr, "{{template"))

	return nil
}
