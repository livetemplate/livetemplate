package statetemplate

import (
	"html/template"
	"reflect"
	"strings"
	"sync"
)

// TemplateTracker manages template dependencies and change detection
type TemplateTracker struct {
	templates map[string]*template.Template
	// Map template names to the data fields they depend on
	dependencies map[string]map[string]bool // template -> field paths -> true
	mu           sync.RWMutex
}

// NewTemplateTracker creates a new template tracker
func NewTemplateTracker() *TemplateTracker {
	return &TemplateTracker{
		templates:    make(map[string]*template.Template),
		dependencies: make(map[string]map[string]bool),
	}
}

// AddTemplate adds a template and analyzes its dependencies
func (tt *TemplateTracker) AddTemplate(name string, tmpl *template.Template) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.templates[name] = tmpl
	tt.dependencies[name] = make(map[string]bool)

	// Analyze template for field dependencies
	tt.analyzeDependencies(name, tmpl)
}

// analyzeDependencies extracts field dependencies from template
func (tt *TemplateTracker) analyzeDependencies(name string, tmpl *template.Template) {
	// This is a simplified implementation
	// In a real implementation, you'd parse the template AST to find all field accesses
	templateText := tmpl.Tree.Root.String()

	// Simple regex-based approach to find {{.FieldName}} patterns
	// This could be enhanced with proper AST parsing
	fields := extractFieldReferences(templateText)

	for _, field := range fields {
		tt.dependencies[name][field] = true
	}
}

// extractFieldReferences extracts field references from template text
func extractFieldReferences(templateText string) []string {
	var fields []string

	// This is a simplified implementation
	// Look for patterns like {{.Field}}, {{.Nested.Field}}, etc.
	// In practice, you'd want to use the template's parse tree

	lines := strings.Split(templateText, "\n")
	for _, line := range lines {
		// Simple pattern matching - this should be enhanced
		if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
			// Extract field references between {{ and }}
			start := strings.Index(line, "{{")
			end := strings.Index(line, "}}")
			if start != -1 && end != -1 && end > start {
				content := strings.TrimSpace(line[start+2 : end])
				if strings.HasPrefix(content, ".") {
					field := strings.TrimPrefix(content, ".")
					// Handle nested fields like .User.Name
					if !strings.Contains(field, " ") && field != "" {
						fields = append(fields, field)
					}
				}
			}
		}
	}

	return fields
}

// DataUpdate represents a data structure update
type DataUpdate struct {
	Data interface{}
}

// TemplateUpdate represents templates that need re-rendering
type TemplateUpdate struct {
	TemplateNames []string
	ChangedFields []string
}

// StartLiveUpdates starts the live update processor
func (tt *TemplateTracker) StartLiveUpdates(
	dataChannel <-chan DataUpdate,
	updateChannel chan<- TemplateUpdate,
) {
	var lastData interface{}

	for update := range dataChannel {
		changedFields := tt.detectChanges(lastData, update.Data)
		if len(changedFields) > 0 {
			affectedTemplates := tt.getAffectedTemplates(changedFields)

			if len(affectedTemplates) > 0 {
				updateChannel <- TemplateUpdate{
					TemplateNames: affectedTemplates,
					ChangedFields: changedFields,
				}
			}
		}
		lastData = update.Data
	}

	close(updateChannel)
}

// detectChanges compares two data structures and returns changed field paths
func (tt *TemplateTracker) detectChanges(oldData, newData interface{}) []string {
	if oldData == nil {
		// First update, consider all fields as changed
		return tt.extractAllFieldPaths(newData)
	}

	return tt.compareStructures("", oldData, newData)
}

// compareStructures recursively compares two structures
func (tt *TemplateTracker) compareStructures(prefix string, oldData, newData interface{}) []string {
	var changedFields []string

	if oldData == nil && newData == nil {
		return changedFields
	}

	if oldData == nil || newData == nil {
		// One is nil, the other isn't - everything changed
		return tt.extractAllFieldPaths(newData)
	}

	oldVal := reflect.ValueOf(oldData)
	newVal := reflect.ValueOf(newData)

	if oldVal.Type() != newVal.Type() {
		// Types differ, everything changed
		return tt.extractAllFieldPaths(newData)
	}

	switch oldVal.Kind() {
	case reflect.Struct:
		for i := 0; i < oldVal.NumField(); i++ {
			fieldName := oldVal.Type().Field(i).Name
			fieldPath := fieldName
			if prefix != "" {
				fieldPath = prefix + "." + fieldName
			}

			oldFieldVal := oldVal.Field(i).Interface()
			newFieldVal := newVal.Field(i).Interface()

			if !reflect.DeepEqual(oldFieldVal, newFieldVal) {
				if isComplexType(oldVal.Field(i)) {
					// Recursively check nested structures
					nestedChanges := tt.compareStructures(fieldPath, oldFieldVal, newFieldVal)
					changedFields = append(changedFields, nestedChanges...)
				} else {
					changedFields = append(changedFields, fieldPath)
				}
			}
		}
	case reflect.Ptr:
		if oldVal.IsNil() && newVal.IsNil() {
			return changedFields
		}
		if oldVal.IsNil() || newVal.IsNil() {
			return tt.extractAllFieldPaths(newData)
		}
		return tt.compareStructures(prefix, oldVal.Elem().Interface(), newVal.Elem().Interface())
	default:
		// Simple types
		if !reflect.DeepEqual(oldData, newData) {
			changedFields = append(changedFields, prefix)
		}
	}

	return changedFields
}

// isComplexType checks if a value is a complex type that needs recursive comparison
func isComplexType(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Struct, reflect.Ptr, reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}

// extractAllFieldPaths extracts all field paths from a data structure
func (tt *TemplateTracker) extractAllFieldPaths(data interface{}) []string {
	var paths []string
	tt.extractFieldPaths("", reflect.ValueOf(data), &paths)
	return paths
}

// extractFieldPaths recursively extracts field paths
func (tt *TemplateTracker) extractFieldPaths(prefix string, val reflect.Value, paths *[]string) {
	switch val.Kind() {
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			fieldName := val.Type().Field(i).Name
			fieldPath := fieldName
			if prefix != "" {
				fieldPath = prefix + "." + fieldName
			}

			if isComplexType(val.Field(i)) {
				tt.extractFieldPaths(fieldPath, val.Field(i), paths)
			} else {
				*paths = append(*paths, fieldPath)
			}
		}
	case reflect.Ptr:
		if !val.IsNil() {
			tt.extractFieldPaths(prefix, val.Elem(), paths)
		}
	default:
		if prefix != "" {
			*paths = append(*paths, prefix)
		}
	}
}

// getAffectedTemplates returns templates that depend on the changed fields
func (tt *TemplateTracker) getAffectedTemplates(changedFields []string) []string {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	affectedTemplates := make(map[string]bool)

	for templateName, dependencies := range tt.dependencies {
		for _, changedField := range changedFields {
			// Check if template depends on this field or any parent field
			if tt.templateDependsOnField(dependencies, changedField) {
				affectedTemplates[templateName] = true
				break
			}
		}
	}

	var result []string
	for templateName := range affectedTemplates {
		result = append(result, templateName)
	}

	return result
}

// templateDependsOnField checks if a template depends on a specific field
func (tt *TemplateTracker) templateDependsOnField(dependencies map[string]bool, changedField string) bool {
	// Direct dependency
	if dependencies[changedField] {
		return true
	}

	// Check if template depends on a parent field
	parts := strings.Split(changedField, ".")
	for i := 1; i <= len(parts); i++ {
		parentField := strings.Join(parts[:i], ".")
		if dependencies[parentField] {
			return true
		}
	}

	// Check if template depends on a child field
	for depField := range dependencies {
		if strings.HasPrefix(depField, changedField+".") {
			return true
		}
	}

	return false
}

// GetDependencies returns a copy of the current template dependencies
func (tt *TemplateTracker) GetDependencies() map[string]map[string]bool {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	result := make(map[string]map[string]bool)
	for templateName, deps := range tt.dependencies {
		result[templateName] = make(map[string]bool)
		for field, exists := range deps {
			result[templateName][field] = exists
		}
	}

	return result
}

// DetectChanges is a public method for testing change detection
func (tt *TemplateTracker) DetectChanges(oldData, newData interface{}) []string {
	return tt.detectChanges(oldData, newData)
}
