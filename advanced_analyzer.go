package statetemplate

import (
	"html/template"
	"regexp"
)

// advancedTemplateAnalyzer provides more sophisticated template dependency analysis
// Enhanced to handle variable assignments and their mappings to source fields
type advancedTemplateAnalyzer struct {
	variableMappings map[string]string // Maps variable names to their source fields
}

// NewadvancedTemplateAnalyzer creates a new advanced analyzer
func newAdvancedTemplateAnalyzer() *advancedTemplateAnalyzer {
	return &advancedTemplateAnalyzer{
		variableMappings: make(map[string]string),
	}
}

// AnalyzeTemplate analyzes a template's text to extract field dependencies
// Enhanced to handle variable assignments and map them back to source fields
func (ata *advancedTemplateAnalyzer) AnalyzeTemplate(tmpl *template.Template) []string {
	var dependencies []string

	if tmpl == nil {
		return dependencies
	}

	// Process the main template and build variable mappings
	if tmpl.Tree != nil && tmpl.Tree.Root != nil {
		templateText := tmpl.Tree.Root.String()

		// First, build variable mappings for the entire template
		ata.buildVariableMappingsFromTemplate(templateText)

		// Then extract field references using those mappings
		deps := ata.extractFieldReferencesWithVariables(templateText)
		dependencies = append(dependencies, deps...)
	}

	// Process associated templates
	associatedDeps := ata.processAssociatedTemplates(tmpl)
	dependencies = append(dependencies, associatedDeps...)

	// Remove duplicates and return
	return removeDuplicates(dependencies)
}

// buildVariableMappingsFromTemplate builds variable mappings for the entire template
func (ata *advancedTemplateAnalyzer) buildVariableMappingsFromTemplate(templateText string) {
	// Reset variable mappings for this analysis
	ata.variableMappings = make(map[string]string)

	// Remove comments first
	commentPattern := regexp.MustCompile(`\{\{/\*.*?\*/\}\}`)
	cleanText := commentPattern.ReplaceAllString(templateText, "")

	// Extract all variable assignments from the entire template
	ata.extractVariableAssignments(cleanText)
}

func (ata *advancedTemplateAnalyzer) processAssociatedTemplates(tmpl *template.Template) []string {
	var dependencies []string

	// Process all associated templates (from {{define}} blocks)
	for _, associatedTmpl := range tmpl.Templates() {
		if associatedTmpl != tmpl && associatedTmpl.Tree != nil && associatedTmpl.Tree.Root != nil {
			templateText := associatedTmpl.Tree.Root.String()
			deps := ata.extractFieldReferencesWithVariables(templateText)
			dependencies = append(dependencies, deps...)
		}
	}

	return dependencies
}

// extractFieldReferencesWithVariables extracts field references including variable assignments
// This is the enhanced version that maps variables back to their source fields
func (ata *advancedTemplateAnalyzer) extractFieldReferencesWithVariables(templateText string) []string {
	var fields []string

	// Remove comments first
	commentPattern := regexp.MustCompile(`\{\{/\*.*?\*/\}\}`)
	cleanText := commentPattern.ReplaceAllString(templateText, "")

	// Step 1: Find variable assignments like {{$var := .Field}} - only if no mappings exist
	if len(ata.variableMappings) == 0 {
		ata.extractVariableAssignments(cleanText)
	}

	// Step 2: Find direct field references
	directFields := ata.extractDirectFieldReferences(cleanText)
	fields = append(fields, directFields...)

	// Step 3: Find variable usages and map them back to source fields
	variableFields := ata.extractVariableUsages(cleanText)
	fields = append(fields, variableFields...)

	return fields
}

// extractFieldReferencesWithExistingMappings uses pre-built variable mappings for fragment analysis
func (ata *advancedTemplateAnalyzer) extractFieldReferencesWithExistingMappings(templateText string) []string {
	var fields []string

	// Remove comments first
	commentPattern := regexp.MustCompile(`\{\{/\*.*?\*/\}\}`)
	cleanText := commentPattern.ReplaceAllString(templateText, "")

	// Don't rebuild variable mappings - use existing ones

	// Find direct field references
	directFields := ata.extractDirectFieldReferences(cleanText)
	fields = append(fields, directFields...)

	// Find variable usages and map them back to source fields using existing mappings
	variableFields := ata.extractVariableUsages(cleanText)
	fields = append(fields, variableFields...)

	return fields
}

// extractVariableAssignments finds variable assignments and builds the mapping
func (ata *advancedTemplateAnalyzer) extractVariableAssignments(templateText string) {
	// Pattern for variable assignments: {{$var := .Field}}
	assignmentPattern := regexp.MustCompile(`\{\{\s*\$([A-Za-z][A-Za-z0-9_]*)\s*:=\s*\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`)
	matches := assignmentPattern.FindAllStringSubmatch(templateText, -1)

	for _, match := range matches {
		if len(match) > 2 {
			varName := match[1]   // Variable name (e.g., "title")
			fieldPath := match[2] // Field path (e.g., "Title" or "User.Name")
			ata.variableMappings[varName] = fieldPath
		}
	}
}

// extractDirectFieldReferences finds direct field references like {{.Field}}
func (ata *advancedTemplateAnalyzer) extractDirectFieldReferences(templateText string) []string {
	var fields []string

	// Regex patterns for direct field references
	patterns := []string{
		`\{\{\s*\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,                      // {{.Field}} or {{.User.Name}}
		`\{\{\s*if\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,                 // {{if .Field}}
		`\{\{\s*range\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,              // {{range .Field}}
		`\{\{\s*with\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,               // {{with .Field}}
		`\{\{\s*template\s+"[^"]+"\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`, // {{template "name" .Field}}
		`\{\{\s*block\s+"[^"]+"\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)\s*\}\}`,    // {{block "name" .Field}}
		`len\s+\.([A-Za-z][A-Za-z0-9_]*(?:\.[A-Za-z][A-Za-z0-9_]*)*)`,                              // len .Field (functions)
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

// extractVariableUsages finds variable usages and maps them back to source fields
func (ata *advancedTemplateAnalyzer) extractVariableUsages(templateText string) []string {
	var fields []string

	// Pattern for variable usage: {{$var}} (but not assignments)
	variablePattern := regexp.MustCompile(`\{\{\s*\$([A-Za-z][A-Za-z0-9_]*)\s*\}\}`)
	matches := variablePattern.FindAllStringSubmatch(templateText, -1)

	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1] // Variable name (e.g., "title")

			// Map variable back to its source field
			if sourceField, exists := ata.variableMappings[varName]; exists {
				fields = append(fields, sourceField)
			}
		}
	}

	return fields
}

// GetVariableMappings returns the current variable mappings
func (ata *advancedTemplateAnalyzer) GetVariableMappings() map[string]string {
	return ata.variableMappings
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

// UpdatetemplateTracker creates a new template tracker with advanced analysis
func (ata *advancedTemplateAnalyzer) UpdatetemplateTracker(tt *templateTracker, name string, tmpl *template.Template) {
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
