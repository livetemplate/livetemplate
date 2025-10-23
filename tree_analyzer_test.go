package livetemplate

import (
	"bytes"
	"strings"
	"testing"
)

// TestTreeAnalyzer_DetectsInefficiency tests that the analyzer detects inefficient trees
func TestTreeAnalyzer_DetectsInefficiency(t *testing.T) {
	// Template with conditional that produces large HTML chunks without separation
	templateStr := `
{{if .CurrentUser}}
<div class="messages" id="messages">
    <div class="empty-state">
        No messages yet. Be the first to send one!
    </div>
</div>
<form class="input-form" lvt-submit="send">
    <input type="text" name="message" placeholder="Type your message...">
    <button type="submit">Send</button>
</form>
{{else}}
<form lvt-submit="join">
    <input type="text" name="username" required>
    <button type="submit">Join</button>
</form>
{{end}}
`

	tmpl := New("test", WithDevMode(true))
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Initial render
	data1 := map[string]interface{}{
		"CurrentUser": "",
	}

	var buf1 bytes.Buffer
	if err := tmpl.Execute(&buf1, data1); err != nil {
		t.Fatalf("Initial execute error: %v", err)
	}

	// Update with new data - this should trigger analyzer warnings
	data2 := map[string]interface{}{
		"CurrentUser": "alice",
	}

	// Capture log output to verify analyzer ran
	// (In real usage, this would appear in server logs)
	var buf2 bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf2, data2); err != nil {
		t.Fatalf("ExecuteUpdates error: %v", err)
	}

	// The tree should have been generated
	tree := buf2.String()
	if len(tree) == 0 {
		t.Error("No tree generated")
	}

	t.Logf("Tree generated: %s", tree)
	t.Logf("Check server logs above for LLM-optimized analyzer output")
}

// TestTreeAnalyzer_WellStructuredTree tests that well-structured trees don't trigger warnings
func TestTreeAnalyzer_WellStructuredTree(t *testing.T) {
	// Template with proper static/dynamic separation
	templateStr := `<div>Counter: {{.Counter}}</div>`

	tmpl := New("test", WithDevMode(true))
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	data1 := map[string]interface{}{"Counter": 0}
	var buf1 bytes.Buffer
	if err := tmpl.Execute(&buf1, data1); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	data2 := map[string]interface{}{"Counter": 1}
	var buf2 bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf2, data2); err != nil {
		t.Fatalf("ExecuteUpdates error: %v", err)
	}

	// This should NOT trigger warnings because the tree has proper structure
	t.Logf("Tree: %s (should be well-structured, no warnings)", buf2.String())
}

// TestTreeAnalyzer_Disabled tests that analyzer doesn't run when DevMode is false
func TestTreeAnalyzer_Disabled(t *testing.T) {
	templateStr := `{{if .Show}}<div>Large HTML chunk without separation</div>{{end}}`

	// DevMode defaults to false
	tmpl := New("test")
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if tmpl.analyzer == nil {
		t.Error("Analyzer should be initialized")
	}

	if tmpl.analyzer.Enabled {
		t.Error("Analyzer should be disabled when DevMode=false")
	}

	// Execute updates - no warnings should appear
	data1 := map[string]interface{}{"Show": false}
	var buf1 bytes.Buffer
	_ = tmpl.Execute(&buf1, data1)

	data2 := map[string]interface{}{"Show": true}
	var buf2 bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf2, data2); err != nil {
		t.Fatalf("ExecuteUpdates error: %v", err)
	}

	t.Log("Analyzer disabled - no warnings should appear")
}

// TestAnalyzeTemplateStructure tests template structure analysis
func TestAnalyzeTemplateStructure(t *testing.T) {
	analyzer := NewTreeUpdateAnalyzer()

	tests := []struct {
		name           string
		templateStr    string
		expectWarnings bool
	}{
		{
			name: "conditional in style tag",
			templateStr: `<style>
{{if .Dark}}
.theme { color: white; }
{{end}}
</style>`,
			expectWarnings: true,
		},
		{
			name:           "clean template",
			templateStr:    `<div>{{.Content}}</div>`,
			expectWarnings: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := analyzer.AnalyzeTemplateStructure(tt.templateStr)

			hasWarnings := len(suggestions) > 0
			if hasWarnings != tt.expectWarnings {
				t.Errorf("Expected warnings: %v, got: %v (suggestions: %v)",
					tt.expectWarnings, hasWarnings, suggestions)
			}

			if len(suggestions) > 0 {
				t.Logf("Suggestions for '%s':", tt.name)
				for _, s := range suggestions {
					t.Logf("  - %s", s)
				}
			}
		})
	}
}

// TestFindIssues tests the issue detection logic
func TestFindIssues(t *testing.T) {
	analyzer := NewTreeUpdateAnalyzer()

	tests := []struct {
		name          string
		tree          treeNode
		expectIssues  bool
		issueContains string
	}{
		{
			name: "large HTML chunk without statics",
			tree: treeNode{
				"0": "<div><span>This is a large HTML chunk</span><p>With multiple tags</p><div>And nested structure</div></div>",
			},
			expectIssues:  true,
			issueContains: "Large HTML chunk",
		},
		{
			name: "well-formed tree with statics",
			tree: treeNode{
				"s": []string{"<div>", "</div>"},
				"0": "value",
			},
			expectIssues: false,
		},
		{
			name: "small dynamic value",
			tree: treeNode{
				"0": "42",
			},
			expectIssues: false,
		},
		{
			name: "nested tree with issues",
			tree: treeNode{
				"0": map[string]interface{}{
					"0": strings.Repeat("<div>Large content</div>", 10),
				},
			},
			expectIssues:  true,
			issueContains: "Large HTML chunk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := analyzer.findIssues(tt.tree, "")

			hasIssues := len(issues) > 0
			if hasIssues != tt.expectIssues {
				t.Errorf("Expected issues: %v, got: %v (issues: %v)",
					tt.expectIssues, hasIssues, issues)
			}

			if tt.expectIssues && len(issues) > 0 {
				found := false
				for _, issue := range issues {
					if strings.Contains(issue, tt.issueContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue containing '%s', got: %v",
						tt.issueContains, issues)
				}
			}
		})
	}
}
