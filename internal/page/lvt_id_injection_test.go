package page

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLvtIDInjection(t *testing.T) {
	t.Run("DetectRegionsFromSource", func(t *testing.T) {
		// Use the actual todos template content
		templateSource := `<!DOCTYPE html>
<html data-theme="auto">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="color-scheme" content="light dark">
    <title>LiveTemplate Todos Demo</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@1/css/pico.min.css">
    <meta name="livetemplate-token" content="{{.Token}}">
</head>
<body>
    <main class="container">
        <h1>📝 Todo List</h1>
        
        <form data-lvt-action="addtodo" data-lvt-form="instant">
            <input
                type="text"
                id="todo-input"
                name="todo-input"
                placeholder="What needs to be done?"
                value="{{.InputText}}"
            >
        </form>
        
        <div style="display: {{if .ShowError}}block{{else}}none{{end}}">
            <mark>{{.ErrorText}}</mark>
        </div>

        <div class="todo-container">
            {{range .Todos}}
            <article>
                <label>
                    <input
                        type="checkbox"
                        {{if .Completed}}checked{{end}}
                        data-lvt-action="toggletodo"
                        data-lvt-params='{"todo-id": "{{.ID}}"}'
                    />
                    <span {{if .Completed}}style="text-decoration: line-through; color: var(--pico-muted-color);"{{end}}>
                        {{.Text}}
                    </span>
                </label>
                <button
                    class="secondary"
                    data-lvt-action="removetodo"
                    data-lvt-params='{"todo-id": "{{.ID}}"}'
                >
                    Remove
                </button>
            </article>
            {{end}}
        </div>

        <small>
            Total: {{.TodoCount}} {{if eq .TodoCount 1}}todo{{else}}todos{{end}}
        </small>
    </main>
</body>
</html>`

		// Detect regions
		regions, err := DetectTemplateRegionsFromSource(templateSource)
		require.NoError(t, err)
		require.NotEmpty(t, regions, "Should detect at least one region")

		t.Logf("Detected %d regions:", len(regions))

		// Track all IDs
		seenIDs := make(map[string]bool)
		duplicateIDs := make(map[string][]string)

		for i, region := range regions {
			t.Logf("  %d. ID=%s, Tag=%s, Source=%s", i+1, region.ID, region.ElementTag,
				region.TemplateSource[:min(50, len(region.TemplateSource))]+"...")

			if seenIDs[region.ID] {
				duplicateIDs[region.ID] = append(duplicateIDs[region.ID],
					region.ElementTag+" ("+region.TemplateSource[:min(30, len(region.TemplateSource))]+"...)")
			} else {
				seenIDs[region.ID] = true
			}
		}

		// Check for duplicates at region detection level
		if len(duplicateIDs) > 0 {
			t.Errorf("REGION DETECTION DUPLICATES FOUND:")
			for id, elements := range duplicateIDs {
				t.Errorf("  ID '%s' detected multiple times:", id)
				for _, elem := range elements {
					t.Errorf("    - %s", elem)
				}
			}
		}

		assert.Empty(t, duplicateIDs, "No duplicate IDs should be generated during region detection")
	})

	t.Run("HTMLExtraction", func(t *testing.T) {
		// Test extractRegionsFromHTML with sample HTML that has lvt-id attributes
		testHTML := `<html>
<head>
    <meta name="token" content="abc123" lvt-id="a1">
</head>
<body>
    <div class="container" lvt-id="a2">
        <input type="text" value="hello" lvt-id="todo-input">
        <div style="display: block" lvt-id="a3">
            <small lvt-id="a4">Error message</small>
        </div>
    </div>
</body>
</html>`

		// Create a page instance to test the method
		page := &Page{}

		// Extract regions from HTML
		regions := page.extractRegionsFromHTML(testHTML)

		t.Logf("Extracted %d regions from HTML", len(regions))

		// Check for uniqueness
		idCounts := make(map[string]int)
		for _, region := range regions {
			idCounts[region.ID]++
			t.Logf("Extracted region: ID=%s, Tag=%s", region.ID, region.ElementTag)
		}

		duplicates := make(map[string]int)
		for id, count := range idCounts {
			if count > 1 {
				duplicates[id] = count
			}
		}

		if len(duplicates) > 0 {
			t.Errorf("HTML EXTRACTION DUPLICATES FOUND:")
			for id, count := range duplicates {
				t.Errorf("  ID '%s' appears %d times", id, count)
			}
		}

		assert.Empty(t, duplicates, "All extracted region IDs should be unique")
		assert.True(t, len(regions) > 0, "Should extract at least one region")

		// Verify specific expected IDs
		expectedIDs := map[string]string{
			"a1":         "meta",
			"a2":         "div",
			"todo-input": "input",
			"a3":         "div",
			"a4":         "small",
		}

		extractedIDs := make(map[string]string)
		for _, region := range regions {
			extractedIDs[region.ID] = region.ElementTag
		}

		for expectedID, expectedTag := range expectedIDs {
			actualTag, found := extractedIDs[expectedID]
			assert.True(t, found, "Should find region with ID: %s", expectedID)
			if found {
				assert.Equal(t, expectedTag, actualTag, "Region ID %s should have tag %s", expectedID, expectedTag)
			}
		}
	})
}

// This test file focuses on internal page functionality
// Application-level tests should be in separate application test files
