package statetemplate

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// templateTracker manages template dependencies and change detection
type templateTracker struct {
	templates map[string]*template.Template
	// Map template names to the data fields they depend on
	dependencies      map[string]map[string]bool     // template -> field paths -> true
	fragments         map[string][]*templateFragment // template -> fragments
	fragmentExtractor *fragmentExtractor
	mu                sync.RWMutex
}

// NewtemplateTracker creates a new template tracker
func newTemplateTracker() *templateTracker {
	return &templateTracker{
		templates:         make(map[string]*template.Template),
		dependencies:      make(map[string]map[string]bool),
		fragments:         make(map[string][]*templateFragment),
		fragmentExtractor: newFragmentExtractor(),
	}
}

// NewtemplateTrackerWithAnalyzer creates a new templateTracker with a shared analyzer
func newTemplateTrackerWithAnalyzer(analyzer *advancedTemplateAnalyzer) *templateTracker {
	return &templateTracker{
		templates:         make(map[string]*template.Template),
		dependencies:      make(map[string]map[string]bool),
		fragments:         make(map[string][]*templateFragment),
		fragmentExtractor: newFragmentExtractorWithAnalyzer(analyzer),
	}
}

// AddTemplate adds a template and analyzes its dependencies
func (tt *templateTracker) AddTemplate(name string, tmpl *template.Template) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	tt.templates[name] = tmpl
	tt.dependencies[name] = make(map[string]bool)

	// Analyze template for field dependencies
	tt.analyzeDependencies(name, tmpl)
}

// AddTemplateWithFragmentExtraction adds a template with automatic fragment extraction
func (tt *templateTracker) AddTemplateWithFragmentExtraction(name, templateContent string) (*template.Template, []*templateFragment, error) {
	// Process template and extract fragments
	tmpl, fragments, err := tt.fragmentExtractor.ProcessTemplateWithFragments(name, templateContent)
	if err != nil {
		return nil, nil, err
	}

	tt.mu.Lock()
	defer tt.mu.Unlock()

	// Store the template and fragments
	tt.templates[name] = tmpl
	tt.fragments[name] = fragments
	tt.dependencies[name] = make(map[string]bool)

	// Analyze dependencies for the main template
	tt.analyzeDependencies(name, tmpl)

	// Analyze dependencies for each fragment and store them as separate templates
	for _, fragment := range fragments {
		tt.dependencies[fragment.ID] = make(map[string]bool)
		for _, dep := range fragment.Dependencies {
			tt.dependencies[fragment.ID][dep] = true
		}
	}

	return tmpl, fragments, nil
}

// AddTemplateFromFile loads a template from a file and adds it to the tracker
func (tt *templateTracker) AddTemplateFromFile(name, filepath string) error {
	tmpl, err := template.ParseFiles(filepath)
	if err != nil {
		return fmt.Errorf("failed to parse template file %s: %w", filepath, err)
	}

	tt.AddTemplate(name, tmpl)
	return nil
}

// AddTemplatesFromFiles loads multiple templates from files and adds them to the tracker
func (tt *templateTracker) AddTemplatesFromFiles(files map[string]string) error {
	for name, filepath := range files {
		if err := tt.AddTemplateFromFile(name, filepath); err != nil {
			return err
		}
	}
	return nil
}

// AddTemplatesFromDirectory loads all template files from a directory
func (tt *templateTracker) AddTemplatesFromDirectory(dir string, extensions ...string) error {
	if len(extensions) == 0 {
		extensions = []string{".html", ".tmpl", ".tpl", ".gohtml"}
	}

	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if file has one of the allowed extensions
		ext := filepath.Ext(path)
		for _, allowedExt := range extensions {
			if ext == allowedExt {
				// Use relative path as template name
				relPath, err := filepath.Rel(dir, path)
				if err != nil {
					relPath = filepath.Base(path)
				}
				// Remove extension from name
				name := strings.TrimSuffix(relPath, ext)

				return tt.AddTemplateFromFile(name, path)
			}
		}

		return nil
	})
}

// AddTemplatesFromFS loads templates from an embedded filesystem
func (tt *templateTracker) AddTemplatesFromFS(fsys fs.FS, pattern string) error {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
	}

	for _, match := range matches {
		// Read the file content
		content, err := fs.ReadFile(fsys, match)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", match, err)
		}

		// Parse template from content
		tmpl, err := template.New(match).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse embedded template %s: %w", match, err)
		}

		// Use filename without extension as template name
		name := strings.TrimSuffix(filepath.Base(match), filepath.Ext(match))
		tt.AddTemplate(name, tmpl)
	}

	return nil
}

// AddTemplatesFromEmbedFS is a convenience method for working with embed.FS
func (tt *templateTracker) AddTemplatesFromEmbedFS(embedFS embed.FS, pattern string) error {
	return tt.AddTemplatesFromFS(embedFS, pattern)
}

// analyzeDependencies extracts field dependencies from template
func (tt *templateTracker) analyzeDependencies(name string, tmpl *template.Template) {
	// Use the advanced analyzer for better dependency extraction
	analyzer := newAdvancedTemplateAnalyzer()
	fields := analyzer.AnalyzeTemplate(tmpl)

	for _, field := range fields {
		tt.dependencies[name][field] = true
	}
}

// dataUpdate represents a data structure update
type dataUpdate struct {
	Data interface{}
}

// templateUpdate represents templates that need re-rendering
type templateUpdate struct {
	TemplateNames []string
	ChangedFields []string
}

// StartLiveUpdates starts the live update processor
func (tt *templateTracker) StartLiveUpdates(
	dataChannel <-chan dataUpdate,
	updateChannel chan<- templateUpdate,
) {
	var lastData interface{}

	for update := range dataChannel {
		changedFields := tt.detectChanges(lastData, update.Data)
		if len(changedFields) > 0 {
			affectedTemplates := tt.getAffectedTemplates(changedFields)

			if len(affectedTemplates) > 0 {
				updateChannel <- templateUpdate{
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
func (tt *templateTracker) detectChanges(oldData, newData interface{}) []string {
	if oldData == nil {
		// First update, consider all fields as changed
		return tt.extractAllFieldPaths(newData)
	}

	return tt.compareStructures("", oldData, newData)
}

// compareStructures recursively compares two structures
func (tt *templateTracker) compareStructures(prefix string, oldData, newData interface{}) []string {
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
	case reflect.Slice:
		// Handle slice changes
		if oldVal.Len() != newVal.Len() {
			// Length changed, mark the entire slice as changed
			if prefix != "" {
				changedFields = append(changedFields, prefix)
			}
		} else {
			// Same length, check individual elements
			for i := 0; i < oldVal.Len(); i++ {
				indexPath := prefix
				if prefix != "" {
					indexPath = fmt.Sprintf("%s[%d]", prefix, i)
				} else {
					indexPath = fmt.Sprintf("[%d]", i)
				}

				oldElem := oldVal.Index(i).Interface()
				newElem := newVal.Index(i).Interface()

				if !reflect.DeepEqual(oldElem, newElem) {
					if isComplexType(oldVal.Index(i)) {
						nestedChanges := tt.compareStructures(indexPath, oldElem, newElem)
						changedFields = append(changedFields, nestedChanges...)
					} else {
						changedFields = append(changedFields, indexPath)
					}
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
func (tt *templateTracker) extractAllFieldPaths(data interface{}) []string {
	var paths []string
	tt.extractFieldPaths("", reflect.ValueOf(data), &paths)
	return paths
}

// extractFieldPaths recursively extracts field paths
func (tt *templateTracker) extractFieldPaths(prefix string, val reflect.Value, paths *[]string) {
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
func (tt *templateTracker) getAffectedTemplates(changedFields []string) []string {
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
func (tt *templateTracker) templateDependsOnField(dependencies map[string]bool, changedField string) bool {
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
func (tt *templateTracker) GetDependencies() map[string]map[string]bool {
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
func (tt *templateTracker) DetectChanges(oldData, newData interface{}) []string {
	return tt.detectChanges(oldData, newData)
}

// GetTemplates returns a copy of all registered templates
func (tt *templateTracker) GetTemplates() map[string]*template.Template {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	result := make(map[string]*template.Template)
	for name, tmpl := range tt.templates {
		result[name] = tmpl
	}

	return result
}

// GetTemplate returns a specific template by name
func (tt *templateTracker) GetTemplate(name string) (*template.Template, bool) {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	tmpl, exists := tt.templates[name]
	return tmpl, exists
}

// GetFragments returns the fragments for a template
func (tt *templateTracker) GetFragments(templateName string) ([]*templateFragment, bool) {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	fragments, exists := tt.fragments[templateName]
	return fragments, exists
}

// GetAllFragments returns all fragments for all templates
func (tt *templateTracker) GetAllFragments() map[string][]*templateFragment {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	result := make(map[string][]*templateFragment)
	for name, fragments := range tt.fragments {
		fragmentsCopy := make([]*templateFragment, len(fragments))
		copy(fragmentsCopy, fragments)
		result[name] = fragmentsCopy
	}

	return result
}
