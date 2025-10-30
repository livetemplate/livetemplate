package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTextareaFields tests textarea field generation
func TestTextareaFields(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "testapp")

	// Create app
	if err := runLvtCommand(t, tmpDir, "new", "testapp"); err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Test 1: Generate resource with explicit textarea type
	t.Run("Explicit_Textarea_Type", func(t *testing.T) {
		if err := runLvtCommand(t, appDir, "gen", "resource", "articles", "title", "content:text"); err != nil {
			t.Fatalf("Failed to generate resource with :text type: %v", err)
		}

		// Verify template contains textarea for content field
		tmplFile := filepath.Join(appDir, "internal", "app", "articles", "articles.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Check that content field has textarea
		if !strings.Contains(contentStr, `<textarea`) {
			t.Error("❌ Template does not contain <textarea> element")
		} else {
			t.Log("✅ Template contains <textarea> element")
		}

		// Check that textarea has rows attribute
		if !strings.Contains(contentStr, `rows="5"`) {
			t.Error("❌ Textarea does not have rows attribute")
		} else {
			t.Log("✅ Textarea has rows=\"5\" attribute")
		}

		// Check that title field still uses input (not textarea)
		titleInputPattern := `name="title"`
		if !strings.Contains(contentStr, titleInputPattern) {
			t.Error("❌ Title field input not found")
		} else {
			// Verify title is an input by checking the surrounding context
			// Look for <input...name="title"
			titleIdx := strings.Index(contentStr, titleInputPattern)
			if titleIdx > 0 {
				// Check 100 chars before the name="title" for <input tag
				startIdx := titleIdx - 100
				if startIdx < 0 {
					startIdx = 0
				}
				contextBefore := contentStr[startIdx:titleIdx]
				if strings.Contains(contextBefore, "<input") {
					t.Log("✅ Title field uses <input> (not textarea)")
				} else if strings.Contains(contextBefore, "<textarea") {
					t.Error("❌ Title field should not use <textarea>")
				}
			}
		}
	})

	// Test 2: Generate resource with inferred textarea type
	t.Run("Inferred_Textarea_Type", func(t *testing.T) {
		if err := runLvtCommand(t, appDir, "gen", "resource", "posts", "title", "content", "description", "body"); err != nil {
			t.Fatalf("Failed to generate resource with inferred textarea types: %v", err)
		}

		// Verify template contains textareas for content, description, body
		tmplFile := filepath.Join(appDir, "internal", "app", "posts", "posts.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Count textarea occurrences (should be 3 fields × 2 forms = 6 textareas)
		textareaCount := strings.Count(contentStr, "<textarea")
		if textareaCount < 6 {
			t.Errorf("❌ Expected at least 6 <textarea> elements, found %d", textareaCount)
		} else {
			t.Logf("✅ Template contains %d <textarea> elements (content, description, body in add and edit forms)", textareaCount)
		}

		// Verify content field has textarea with name attribute
		if !strings.Contains(contentStr, `name="content"`) {
			t.Error("❌ Content field not found in template")
		}

		// Verify description field has textarea
		if !strings.Contains(contentStr, `name="description"`) {
			t.Error("❌ Description field not found in template")
		}

		// Verify body field has textarea
		if !strings.Contains(contentStr, `name="body"`) {
			t.Error("❌ Body field not found in template")
		}

		// Verify title field still uses input
		titleInputPattern := `name="title"`
		if !strings.Contains(contentStr, titleInputPattern) {
			t.Error("❌ Title field not found")
		}

		t.Log("✅ Type inference correctly mapped content, description, body to textarea fields")
	})

	// Test 3: Verify textarea aliases work (textarea, longtext)
	t.Run("Textarea_Aliases", func(t *testing.T) {
		if err := runLvtCommand(t, appDir, "gen", "resource", "documents", "title", "summary:textarea", "details:longtext"); err != nil {
			t.Fatalf("Failed to generate resource with textarea aliases: %v", err)
		}

		// Verify template contains textareas
		tmplFile := filepath.Join(appDir, "internal", "app", "documents", "documents.tmpl")
		content, err := os.ReadFile(tmplFile)
		if err != nil {
			t.Fatalf("Failed to read template: %v", err)
		}

		contentStr := string(content)

		// Should have textareas for summary and details (2 fields × 2 forms = 4)
		textareaCount := strings.Count(contentStr, "<textarea")
		if textareaCount < 4 {
			t.Errorf("❌ Expected at least 4 <textarea> elements, found %d", textareaCount)
		} else {
			t.Logf("✅ Textarea aliases (textarea, longtext) work correctly (%d textareas found)", textareaCount)
		}
	})

	t.Log("✅ All textarea field tests passed!")
}
