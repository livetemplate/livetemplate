package statetemplate

import (
	"html/template"
	"regexp"
)

// AdvancedTemplateAnalyzer provides more sophisticated template dependency analysis
type AdvancedTemplateAnalyzer struct{}

// NewAdvancedTemplateAnalyzer creates a new advanced analyzer
func NewAdvancedTemplateAnalyzer() *AdvancedTemplateAnalyzer {
	return &AdvancedTemplateAnalyzer{}
}

// AnalyzeTemplate analyzes a template's text to extract field dependencies
func (ata *AdvancedTemplateAnalyzer) AnalyzeTemplate(tmpl *template.Template) []string {
	var dependencies []string

	if tmpl.Tree == nil || tmpl.Tree.Root == nil {
		return dependencies
	}

	// Get the template text
	templateText := tmpl.Tree.Root.String()

	// Use regex to find field references
	dependencies = ata.extractFieldReferences(templateText)

	// Remove duplicates and return
	return removeDuplicates(dependencies)
}

// extractFieldReferences uses regex to find field references in template text
func (ata *AdvancedTemplateAnalyzer) extractFieldReferences(templateText string) []string {
	var fields []string

	// Regex patterns for different template constructs
	patterns := []string{
		`\{\{\s*\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,         // {{.Field}} or {{.User.Name}}
		`\{\{\s*if\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,    // {{if .Field}}
		`\{\{\s*range\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`, // {{range .Field}}
		`\{\{\s*with\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,  // {{with .Field}}
		`template\s+"[^"]+"\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)`,  // template "name" .Field
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(templateText, -1)

		for _, match := range matches {
			if len(match) > 1 && match[1] != "" {
				fields = append(fields, match[1])
			}
		}
	}

	return fields
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// UpdateTemplateTracker creates a new template tracker with advanced analysis
func (ata *AdvancedTemplateAnalyzer) UpdateTemplateTracker(tt *TemplateTracker, name string, tmpl *template.Template) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.templates[name] = tmpl
	tt.dependencies[name] = make(map[string]bool)

	// Use advanced analysis to get dependencies
	deps := ata.AnalyzeTemplate(tmpl)
	for _, dep := range deps {
		tt.dependencies[name][dep] = true
	}
}
