package statetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RangeInfo contains range operation information.
// Only set for range operations (append, prepend, remove, insertafter, insertbefore).
type RangeInfo struct {
	ItemKey     string `json:"item_key"`               // Always present for range operations
	ReferenceID string `json:"reference_id,omitempty"` // Only for insertafter/insertbefore positioning
}

// Update represents an update that can be sent to the client.
// This is the primary output type for real-time template updates.
type Update struct {
	FragmentID string `json:"fragment_id"` // The ID of the div/element to update
	HTML       string `json:"html"`        // The new HTML content for that fragment
	Action     string `json:"action"`      // "replace", "append", "prepend", "remove", "insertafter", "insertbefore"

	// Range operation info - only set for range operations
	*RangeInfo `json:"range,omitempty"`
}

// rangeItem represents a single item within a range loop
type rangeItem struct {
	ID    string      // Unique ID for this specific item instance
	Index int         // Position in the array
	Key   string      // Unique key for the item (e.g., URL, ID field)
	Data  interface{} // The actual item data
	HTML  string      // Rendered HTML for this item
}

// rangeFragment represents a fragment that contains a range loop
type rangeFragment struct {
	*templateFragment
	RangePath    string                // The path to the array (e.g., "Navigation.MainItems")
	ItemTemplate string                // The template content for individual items
	Items        map[string]*rangeItem // Current items keyed by their unique key
	ContainerID  string                // ID of the container element
}

// Renderer handles real-time template rendering with fragment targeting
type Renderer struct {
	templates       map[string]*template.Template
	fragmentTracker *fragmentExtractor
	tracker         *templateTracker // For change detection
	currentData     interface{}
	dataMutex       sync.RWMutex
	updateChan      chan interface{}
	outputChan      chan Update
	running         bool
	stopChan        chan bool
	wrapperPattern  string                         // Pattern for wrapping fragments with IDs
	fragmentStore   map[string][]*templateFragment // Store fragments by template name
	rangeFragments  map[string][]*rangeFragment    // Store range-specific fragments by template name
}

// Config configures the real-time renderer
type Config struct {
	WrapperTag     string // HTML tag to wrap fragments (default: "div")
	IDPrefix       string // Prefix for fragment IDs (default: "fragment-")
	PreserveBlocks bool   // Whether to preserve block names as IDs when possible
}

// NewRenderer creates a new real-time renderer
func NewRenderer(config *Config) *Renderer {
	if config == nil {
		config = &Config{
			WrapperTag:     "div",
			IDPrefix:       "fragment-",
			PreserveBlocks: true,
		}
	}

	return &Renderer{
		templates:       make(map[string]*template.Template),
		fragmentTracker: newFragmentExtractor(),
		tracker:         newTemplateTracker(),
		updateChan:      make(chan interface{}, 100),
		outputChan:      make(chan Update, 100),
		stopChan:        make(chan bool),
		wrapperPattern:  fmt.Sprintf("<%s id=\"%%s\">%%s</%s>", config.WrapperTag, config.WrapperTag),
		fragmentStore:   make(map[string][]*templateFragment),
		rangeFragments:  make(map[string][]*rangeFragment),
	}
}

// AddTemplate adds a template for real-time rendering
func (r *Renderer) AddTemplate(name, content string) error {
	r.dataMutex.Lock()
	defer r.dataMutex.Unlock()

	// Parse the template
	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	r.templates[name] = tmpl

	// For now, create simple fragments based on template expressions
	// This is more reliable than the complex fragment extraction
	fragments := r.createSimpleFragments(content, name)

	// Store fragments for this template
	r.fragmentStore[name] = fragments

	// Also add to the tracker for dependency analysis
	r.tracker.AddTemplate(name, tmpl)

	return nil
}

// GetFragments returns the fragments for a template (for debugging)
func (r *Renderer) GetFragments(templateName string) ([]*templateFragment, bool) {
	r.dataMutex.Lock()
	defer r.dataMutex.Unlock()

	fragments, exists := r.fragmentStore[templateName]
	return fragments, exists
}

// createSimpleFragments creates fragments from template expressions
func (r *Renderer) createSimpleFragments(content, templateName string) []*templateFragment {
	var fragments []*templateFragment

	// Use the tracker's analyzer to find dependencies for the whole template
	tmpl, err := template.New("temp").Parse(content)
	if err != nil {
		return fragments
	}

	analyzer := newAdvancedTemplateAnalyzer()
	allDependencies := analyzer.AnalyzeTemplate(tmpl)

	// Create granular fragments for individual template expressions
	fragments = r.createGranularFragments(content, templateName, allDependencies)

	// Create range fragments for loop constructs
	rangeFragments := r.createrangeFragments(content, templateName, allDependencies)
	r.rangeFragments[templateName] = rangeFragments

	// Create conditional fragments for if/with blocks
	conditionalFragments := r.createconditionalFragments(content, templateName, allDependencies)
	for _, condFragment := range conditionalFragments {
		fragments = append(fragments, condFragment.templateFragment)
	}

	// Create template include fragments
	includeFragments := r.createtemplateIncludeFragments(content, templateName, allDependencies)
	for _, includeFragment := range includeFragments {
		fragments = append(fragments, includeFragment.templateFragment)
	}

	// Try to identify block fragments separately
	r.addBlockFragments(&fragments, content, templateName, allDependencies)

	// If no logical fragments were found, create a single fragment for the entire template
	if len(fragments) == 0 {
		fragment := &templateFragment{
			ID:           r.generateShortID(),
			Content:      content,
			Dependencies: allDependencies,
			StartPos:     0,
			EndPos:       len(content),
		}
		fragments = append(fragments, fragment)
	}

	return fragments
}

// createGranularFragments creates individual fragments for each template expression line
func (r *Renderer) createGranularFragments(content, templateName string, allDependencies []string) []*templateFragment {
	var fragments []*templateFragment

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and non-template lines
		if trimmedLine == "" || !strings.Contains(line, "{{") {
			continue
		}

		// Skip control flow constructs that don't produce direct output
		if strings.Contains(trimmedLine, "{{end}}") || strings.Contains(trimmedLine, "{{else}}") ||
			strings.Contains(trimmedLine, "{{break}}") || strings.Contains(trimmedLine, "{{continue}}") {
			continue
		}

		// Skip complex constructs that are handled separately
		if strings.Contains(trimmedLine, "{{block") || strings.Contains(trimmedLine, "{{range") ||
			strings.Contains(trimmedLine, "{{if") || strings.Contains(trimmedLine, "{{with") ||
			strings.Contains(trimmedLine, "{{template") {
			continue
		}

		// Skip variable assignments as they don't produce direct output
		if strings.Contains(trimmedLine, ":=") {
			continue
		}

		// Create fragments for lines that produce output with template data
		// This includes both HTML elements with template expressions and plain text with template expressions
		if (strings.Contains(line, "<") && strings.Contains(line, ">")) ||
			(strings.Contains(line, "{{") && strings.Contains(line, "}}") &&
				!strings.Contains(trimmedLine, "{{/*") && !strings.Contains(trimmedLine, "*/}}")) {
			// Find dependencies for this specific line
			lineDependencies := r.findLineDependencies(line, allDependencies)
			if len(lineDependencies) > 0 {
				// Before storing the fragment, resolve any template variables to their actual field references
				processedContent := r.resolveVariableReferences(line, content)

				fragment := &templateFragment{
					ID:           r.generateShortID(),
					Content:      processedContent,
					Dependencies: lineDependencies,
					StartPos:     i,
					EndPos:       i + 1,
				}
				fragments = append(fragments, fragment)
			}
		}
	}

	return fragments
}

// findLineDependencies finds which dependencies are used in a specific line
func (r *Renderer) findLineDependencies(line string, allDependencies []string) []string {
	var lineDeps []string

	for _, dep := range allDependencies {
		// Check for direct template references: {{.Field}}
		templateRef := fmt.Sprintf("{{.%s}}", dep)
		if strings.Contains(line, templateRef) {
			lineDeps = append(lineDeps, dep)
			continue
		}

		// Check for variable assignments: {{$var := .Field}}
		varAssignRef := fmt.Sprintf(":= .%s}}", dep)
		if strings.Contains(line, varAssignRef) {
			lineDeps = append(lineDeps, dep)
			continue
		}

		// Check for function calls on fields: {{len .Field}}
		funcCallRef := fmt.Sprintf(" .%s}}", dep)
		if strings.Contains(line, funcCallRef) {
			lineDeps = append(lineDeps, dep)
			continue
		}
	}

	// Also check if this line uses template variables that were assigned from dependencies
	// Look for {{$varname}} patterns and map them back to data dependencies
	variablePattern := regexp.MustCompile(`\{\{\$(\w+)\}\}`)
	varMatches := variablePattern.FindAllStringSubmatch(line, -1)

	for _, match := range varMatches {
		if len(match) > 1 {
			varName := match[1]
			// Find the dependency this variable was assigned from
			if dep := r.findVariableDependency(varName, allDependencies); dep != "" {
				lineDeps = append(lineDeps, dep)
			}
		}
	}

	return lineDeps
}

// findVariableDependency finds what data dependency a template variable was assigned from
// This is a simple heuristic that maps variable names to likely field names
func (r *Renderer) findVariableDependency(varName string, allDependencies []string) string {
	// Simple mapping: $title -> Title, $count -> Count, etc.
	expectedFieldName := strings.Title(varName)

	for _, dep := range allDependencies {
		if dep == expectedFieldName {
			return dep
		}
	}

	return ""
}

// resolveVariableReferences replaces template variables in a line with their actual field references
// For example, {{$title}} becomes {{.Title}} based on the variable assignment {{$title := .Title}}
func (r *Renderer) resolveVariableReferences(line, fullTemplate string) string {
	// Find all variable usages in the line: {{$varname}}
	variablePattern := regexp.MustCompile(`\{\{\$(\w+)\}\}`)

	result := line
	matches := variablePattern.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			fullVarUsage := match[0] // e.g., "{{$title}}"

			// Find the assignment for this variable in the full template
			if fieldRef := r.findVariableAssignment(varName, fullTemplate); fieldRef != "" {
				result = strings.ReplaceAll(result, fullVarUsage, fieldRef)
			}
		}
	}

	return result
}

// findVariableAssignment finds the field reference for a variable assignment
// For example, for $title in "{{$title := .Title}}", returns "{{.Title}}"
func (r *Renderer) findVariableAssignment(varName, fullTemplate string) string {
	// Look for assignment pattern: {{$varname := .Fieldname}}
	assignmentPattern := regexp.MustCompile(`\{\{\$` + regexp.QuoteMeta(varName) + `\s*:=\s*(\.\w+)\}\}`)
	matches := assignmentPattern.FindStringSubmatch(fullTemplate)

	if len(matches) > 1 {
		fieldRef := matches[1]        // e.g., ".Title"
		return "{{" + fieldRef + "}}" // Convert to "{{.Title}}"
	}

	return ""
}

// createrangeFragments creates fragments for range loops to enable granular list operations
func (r *Renderer) createrangeFragments(content, templateName string, allDependencies []string) []*rangeFragment {
	var rangeFragments []*rangeFragment

	lines := strings.Split(content, "\n")
	inRange := false
	var rangeStart, rangeEnd int
	var rangePath string
	var rangeContent []string

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect start of range block
		if strings.Contains(trimmedLine, "{{range") {
			inRange = true
			rangeStart = i
			rangeContent = []string{}

			// Extract the range path (e.g., ".Navigation.MainItems")
			// Handle both "{{range .Items}}" and "{{range $index, $item := .Items}}" syntax
			rangeRegex := regexp.MustCompile(`\{\{range\s+(?:\$\w+,\s*\$\w+\s*:=\s*)?\.([^}]+)\}\}`)
			matches := rangeRegex.FindStringSubmatch(trimmedLine)
			if len(matches) > 1 {
				rangePath = matches[1]
			}
			continue
		}

		// Collect content within range block
		if inRange {
			if strings.Contains(trimmedLine, "{{end}}") {
				rangeEnd = i
				inRange = false

				// Create range fragment
				if rangePath != "" && len(rangeContent) > 0 {
					containerID := r.generateShortID()
					itemTemplate := strings.Join(rangeContent, "\n")

					fragment := &rangeFragment{
						templateFragment: &templateFragment{
							ID:           containerID,
							Content:      strings.Join(lines[rangeStart:rangeEnd+1], "\n"),
							Dependencies: []string{rangePath},
							StartPos:     rangeStart,
							EndPos:       rangeEnd + 1,
						},
						RangePath:    rangePath,
						ItemTemplate: itemTemplate,
						Items:        make(map[string]*rangeItem),
						ContainerID:  containerID,
					}
					rangeFragments = append(rangeFragments, fragment)
				}
				rangePath = ""
				rangeContent = []string{}
			} else {
				rangeContent = append(rangeContent, line)
			}
		}
	}

	return rangeFragments
}

// conditionalFragment represents a fragment that contains conditional logic
type conditionalFragment struct {
	*templateFragment
	ConditionPath string // The path to the condition (e.g., "User.IsLoggedIn")
	TrueContent   string // Content when condition is true
	FalseContent  string // Content when condition is false (else block)
	FragmentType  string // "if", "with"
}

// templateIncludeFragment represents a fragment that includes another template
type templateIncludeFragment struct {
	*templateFragment
	TemplateName string // Name of the included template
	DataPath     string // Path to the data passed to template (optional)
}

// createconditionalFragments creates fragments for if/with conditional blocks
func (r *Renderer) createconditionalFragments(content, templateName string, allDependencies []string) []*conditionalFragment {
	var conditionalFragments []*conditionalFragment

	lines := strings.Split(content, "\n")

	// Track nested conditionals
	var conditionStack []map[string]interface{}

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect if conditions
		if strings.Contains(trimmedLine, "{{if") {
			conditionPath := r.extractConditionPath(trimmedLine, "if")
			if conditionPath != "" {
				conditionStack = append(conditionStack, map[string]interface{}{
					"type":      "if",
					"path":      conditionPath,
					"startLine": i,
					"content":   []string{},
				})
			}
		}

		// Detect with conditions
		if strings.Contains(trimmedLine, "{{with") {
			conditionPath := r.extractConditionPath(trimmedLine, "with")
			if conditionPath != "" {
				conditionStack = append(conditionStack, map[string]interface{}{
					"type":      "with",
					"path":      conditionPath,
					"startLine": i,
					"content":   []string{},
				})
			}
		}

		// Handle else blocks
		if strings.Contains(trimmedLine, "{{else}}") && len(conditionStack) > 0 {
			// Mark transition to else content in current condition
			currentCondition := conditionStack[len(conditionStack)-1]
			currentCondition["elseStartLine"] = i
			currentCondition["elseContent"] = []string{}
		}

		// Handle end blocks
		if strings.Contains(trimmedLine, "{{end}}") && len(conditionStack) > 0 {
			// Pop the most recent condition and create fragment
			currentCondition := conditionStack[len(conditionStack)-1]
			conditionStack = conditionStack[:len(conditionStack)-1]

			fragmentType := currentCondition["type"].(string)
			conditionPath := currentCondition["path"].(string)
			startLine := currentCondition["startLine"].(int)

			var trueContent, falseContent string
			if content, ok := currentCondition["content"].([]string); ok {
				trueContent = strings.Join(content, "\n")
			}
			if elseContent, ok := currentCondition["elseContent"].([]string); ok {
				falseContent = strings.Join(elseContent, "\n")
			}

			// Create conditional fragment
			fragment := &conditionalFragment{
				templateFragment: &templateFragment{
					ID:           r.generateShortID(),
					Content:      strings.Join(lines[startLine:i+1], "\n"),
					Dependencies: []string{conditionPath},
					StartPos:     startLine,
					EndPos:       i + 1,
				},
				ConditionPath: conditionPath,
				TrueContent:   trueContent,
				FalseContent:  falseContent,
				FragmentType:  fragmentType,
			}
			conditionalFragments = append(conditionalFragments, fragment)
		}

		// Collect content for active conditions
		if len(conditionStack) > 0 {
			currentCondition := conditionStack[len(conditionStack)-1]

			// Skip the opening condition line itself
			if i == currentCondition["startLine"].(int) {
				continue
			}

			// Add to appropriate content collection
			if _, hasElse := currentCondition["elseStartLine"]; hasElse {
				if elseContent, ok := currentCondition["elseContent"].([]string); ok {
					elseContent = append(elseContent, line)
					currentCondition["elseContent"] = elseContent
				}
			} else {
				if content, ok := currentCondition["content"].([]string); ok {
					content = append(content, line)
					currentCondition["content"] = content
				}
			}
		}
	}

	return conditionalFragments
}

// createtemplateIncludeFragments creates fragments for template inclusion blocks
func (r *Renderer) createtemplateIncludeFragments(content, templateName string, allDependencies []string) []*templateIncludeFragment {
	var includeFragments []*templateIncludeFragment

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect template inclusions
		if strings.Contains(trimmedLine, "{{template") {
			templateName, dataPath := r.extractTemplateIncludeInfo(trimmedLine)
			if templateName != "" {
				var dependencies []string
				if dataPath != "" {
					dependencies = []string{dataPath}
				}

				fragment := &templateIncludeFragment{
					templateFragment: &templateFragment{
						ID:           r.generateShortID(),
						Content:      line,
						Dependencies: dependencies,
						StartPos:     i,
						EndPos:       i + 1,
					},
					TemplateName: templateName,
					DataPath:     dataPath,
				}
				includeFragments = append(includeFragments, fragment)
			}
		}
	}

	return includeFragments
}

// extractConditionPath extracts the condition path from if/with statements
func (r *Renderer) extractConditionPath(line, conditionType string) string {
	// Patterns for different condition types
	var pattern string
	switch conditionType {
	case "if":
		pattern = `\{\{if\s+\.([^}]+)\}\}`
	case "with":
		pattern = `\{\{with\s+\.([^}]+)\}\}`
	default:
		return ""
	}

	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractTemplateIncludeInfo extracts template name and data path from template inclusion
func (r *Renderer) extractTemplateIncludeInfo(line string) (templateName, dataPath string) {
	// Pattern for template with data: {{template "name" .Data}}
	templateWithDataPattern := `\{\{template\s+"([^"]+)"\s+\.([^}]+)\}\}`
	re := regexp.MustCompile(templateWithDataPattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 2 {
		return matches[1], matches[2]
	}

	// Pattern for template without data: {{template "name"}}
	templatePattern := `\{\{template\s+"([^"]+)"\}\}`
	re = regexp.MustCompile(templatePattern)
	matches = re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1], ""
	}

	return "", ""
}

// generateShortID generates a short, unique 6-character ID
func (r *Renderer) generateShortID() string {
	// Use a simple approach: timestamp + random component
	timestamp := time.Now().UnixNano()

	// Create a 6-character alphanumeric ID
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	id := ""

	// Use timestamp for uniqueness
	for i := 0; i < 6; i++ {
		id += string(chars[timestamp%int64(len(chars))])
		timestamp = timestamp / int64(len(chars))
	}

	return id
}

// detectPatternFragments detects fragments based on common patterns
func (r *Renderer) detectPatternFragments(content, templateName string, allDependencies []string) []*templateFragment {
	var fragments []*templateFragment
	// This could be extended to detect common UI patterns
	// For now, we rely on granular line-based detection
	return fragments
}

// addBlockFragments identifies and adds block fragments
func (r *Renderer) addBlockFragments(fragments *[]*templateFragment, content, templateName string, allDeps []string) {
	blockRegex := regexp.MustCompile(`\{\{block\s+"([^"]+)"[^}]*\}\}(.*?)\{\{end\}\}`)
	matches := blockRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			blockName := match[1]
			blockContent := match[2]

			// Try to create sub-fragments within the block
			blockFragments := r.detectSubFragmentsInBlock(blockContent, templateName, blockName, allDeps)

			if len(blockFragments) > 0 {
				// Add sub-fragments
				*fragments = append(*fragments, blockFragments...)
			} else {
				// Add the entire block as a single fragment
				fragment := &templateFragment{
					ID:           blockName, // Use block name as ID
					Content:      blockContent,
					Dependencies: allDeps, // For simplicity, use all dependencies
					StartPos:     0,
					EndPos:       len(blockContent),
				}
				*fragments = append(*fragments, fragment)
			}
		}
	}
}

// detectSubFragmentsInBlock detects logical sub-fragments within a block
func (r *Renderer) detectSubFragmentsInBlock(blockContent, templateName, blockName string, allDeps []string) []*templateFragment {
	var fragments []*templateFragment

	// Group dependencies by their root field within this block
	fieldGroups := make(map[string][]string)

	for _, dep := range allDeps {
		parts := strings.Split(dep, ".")
		if len(parts) > 1 {
			rootField := parts[0]

			// Check if this dependency is used in the block content
			templateRef := fmt.Sprintf("{{.%s}}", dep)
			if strings.Contains(blockContent, templateRef) {
				fieldGroups[rootField] = append(fieldGroups[rootField], dep)
			}
		}
	}

	// Create fragments for each field group found in the block
	for rootField, fieldDeps := range fieldGroups {
		sectionContent := r.extractFieldSectionFromBlock(blockContent, fieldDeps)
		if sectionContent != "" && len(strings.TrimSpace(sectionContent)) > 0 {
			fragment := &templateFragment{
				ID:           fmt.Sprintf("%s-%s", blockName, strings.ToLower(rootField)),
				Content:      sectionContent,
				Dependencies: fieldDeps,
				StartPos:     0,
				EndPos:       len(sectionContent),
			}
			fragments = append(fragments, fragment)
		}
	}

	return fragments
}

// extractFieldSectionFromBlock extracts the template section within a block that uses the given field dependencies
func (r *Renderer) extractFieldSectionFromBlock(blockContent string, fieldDeps []string) string {
	lines := strings.Split(blockContent, "\n")
	var sectionLines []string
	inSection := false

	for _, line := range lines {
		hasFieldRef := false
		for _, dep := range fieldDeps {
			// Convert dependency to template syntax (e.g., Counter.Value -> {{.Counter.Value}})
			templateRef := fmt.Sprintf("{{.%s}}", dep)
			if strings.Contains(line, templateRef) {
				hasFieldRef = true
				break
			}
		}

		if hasFieldRef {
			inSection = true
			sectionLines = append(sectionLines, line)
		} else if inSection && strings.TrimSpace(line) == "" {
			// Include empty lines within sections
			sectionLines = append(sectionLines, line)
		} else if inSection && !hasFieldRef && strings.TrimSpace(line) != "" {
			// Check if this line is part of a multi-line construct (like nav with range)
			trimmedLine := strings.TrimSpace(line)
			if strings.Contains(trimmedLine, "{{range") || strings.Contains(trimmedLine, "{{end}}") ||
				strings.Contains(trimmedLine, "<nav>") || strings.Contains(trimmedLine, "</nav>") ||
				strings.Contains(trimmedLine, "<a href") {
				sectionLines = append(sectionLines, line)
			} else {
				// Stop the section when we hit unrelated content
				break
			}
		}
	}

	return strings.Join(sectionLines, "\n")
}

// generateSimpleID generates a simple random ID as fallback
func (r *Renderer) generateSimpleID() string {
	// Simple ID generation for fallback
	return fmt.Sprintf("frag-%d", time.Now().UnixNano()%1000000)
}

// SetInitialData sets the initial data and returns the full rendered HTML
func (r *Renderer) SetInitialData(data interface{}) (string, error) {
	r.dataMutex.Lock()
	r.currentData = data
	r.dataMutex.Unlock()

	return r.renderFullHTML()
}

// GetUpdateChannel returns the channel for receiving real-time updates
func (r *Renderer) GetUpdateChannel() <-chan Update {
	return r.outputChan
}

// SendUpdate sends new data that may trigger fragment updates
func (r *Renderer) SendUpdate(newData interface{}) {
	if !r.running {
		return
	}

	select {
	case r.updateChan <- newData:
	default:
		// Channel full, skip this update to prevent blocking
	}
}

// Start begins processing real-time updates
func (r *Renderer) Start() {
	r.running = true
	go r.processUpdates()
}

// Stop stops processing updates
func (r *Renderer) Stop() {
	r.running = false
	close(r.stopChan)
}

// renderFullHTML renders the complete HTML with fragment IDs
func (r *Renderer) renderFullHTML() (string, error) {
	if len(r.templates) == 0 {
		return "", fmt.Errorf("no templates added")
	}

	var fullHTML strings.Builder

	for name, tmpl := range r.templates {
		// First, render the template completely
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, r.currentData); err != nil {
			return "", fmt.Errorf("failed to execute template %s: %w", name, err)
		}
		renderedHTML := buf.String()

		// Then identify and wrap fragments in the rendered HTML
		wrappedHTML := r.wrapRenderedFragments(renderedHTML, name)
		fullHTML.WriteString(wrappedHTML)
	}

	return fullHTML.String(), nil
}

// wrapRenderedFragments identifies fragment sections in rendered HTML and wraps them with div IDs
func (r *Renderer) wrapRenderedFragments(renderedHTML, templateName string) string {
	fragments := r.fragmentStore[templateName]
	rangeFragments := r.rangeFragments[templateName]

	if len(fragments) == 0 && len(rangeFragments) == 0 {
		return renderedHTML
	}

	result := renderedHTML

	// Process regular fragments by wrapping individual template expressions
	for _, fragment := range fragments {
		fragmentID := fragment.ID
		if blockName := r.extractBlockName(fragment.Content); blockName != "" {
			fragmentID = blockName
		}

		// Handle different types of fragments
		if r.isCounterFieldFragment(fragment) {
			result = r.wrapCounterFieldFragment(result, fragment, fragmentID)
		} else if r.isSiteFragment(fragment) {
			result = r.wrapSiteFragment(result, fragmentID)
		} else if r.isNavigationFragment(fragment) {
			result = r.wrapNavigationFragment(result, fragmentID)
		} else if r.isconditionalFragment(fragment) {
			result = r.wrapconditionalFragment(result, fragment, fragmentID)
		} else if r.istemplateIncludeFragment(fragment) {
			result = r.wraptemplateIncludeFragment(result, fragment, fragmentID)
		} else {
			// Generic fragment wrapping for any template expressions
			result = r.wrapGenericFragment(result, fragment, fragmentID)
		}
	}

	// Process range fragments by wrapping individual items within the range
	for _, rangeFragment := range rangeFragments {
		result = r.wraprangeFragmentItems(result, rangeFragment)
	}

	return result
}

// isCounterFieldFragment checks if this fragment contains a specific counter field
func (r *Renderer) isCounterFieldFragment(fragment *templateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Counter.") {
			return true
		}
	}
	return false
}

// wrapCounterFieldFragment wraps individual counter field lines with div IDs
func (r *Renderer) wrapCounterFieldFragment(html string, fragment *templateFragment, fragmentID string) string {
	// Determine what type of counter field this is based on dependencies
	var pattern string

	for _, dep := range fragment.Dependencies {
		switch dep {
		case "Counter.Value":
			pattern = `(\s*Current Count: \d+)`
		case "Counter.LastUpdated":
			pattern = `(\s*Last updated: [^\n]+)`
		case "Counter.UpdateCount":
			pattern = `(\s*Total updates: \d+)`
		}

		if pattern != "" {
			re := regexp.MustCompile(pattern)
			replacement := fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID)
			html = re.ReplaceAllString(html, replacement)
			break
		}
	}

	return html
}

// isSiteFragment checks if this fragment contains site-related dependencies
func (r *Renderer) isSiteFragment(fragment *templateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Site.") {
			return true
		}
	}
	return false
}

// isNavigationFragment checks if this fragment contains navigation-related dependencies
func (r *Renderer) isNavigationFragment(fragment *templateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Navigation.") || dep == "URL" || dep == "Label" {
			return true
		}
	}
	return false
}

// isconditionalFragment checks if this fragment contains conditional logic (if/with)
func (r *Renderer) isconditionalFragment(fragment *templateFragment) bool {
	return strings.Contains(fragment.Content, "{{if") || strings.Contains(fragment.Content, "{{with")
}

// istemplateIncludeFragment checks if this fragment includes another template
func (r *Renderer) istemplateIncludeFragment(fragment *templateFragment) bool {
	return strings.Contains(fragment.Content, "{{template")
}

// wrapSiteFragment wraps the site heading with a div ID
func (r *Renderer) wrapSiteFragment(html, fragmentID string) string {
	// Look for h1 tags containing the site name
	re := regexp.MustCompile(`(\s*<h1>.*?</h1>)`)
	return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
}

// wrapNavigationFragment wraps the navigation section with a div ID
func (r *Renderer) wrapNavigationFragment(html, fragmentID string) string {
	// Look for nav tags and their content
	re := regexp.MustCompile(`(\s*<nav>.*?</nav>)`)
	return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
}

// wraprangeFragmentItems wraps individual items within a range fragment with unique IDs
func (r *Renderer) wraprangeFragmentItems(html string, rangeFragment *rangeFragment) string {
	// First, wrap the entire range container (e.g., <ul>, <nav>, etc.)
	html = r.wrapRangeContainer(html, rangeFragment)

	// Then wrap individual items based on the range type
	if strings.Contains(rangeFragment.RangePath, "Navigation") {
		return r.wrapNavigationItems(html, rangeFragment)
	}

	// Generic range item wrapping for any list items
	return r.wrapGenericrangeItems(html, rangeFragment)
}

// wrapNavigationItems wraps individual navigation items with unique IDs
func (r *Renderer) wrapNavigationItems(html string, rangeFragment *rangeFragment) string {
	// Pattern to match individual navigation links
	re := regexp.MustCompile(`(\s*<a href="([^"]*)"[^>]*>([^<]*)</a>)`)

	itemIndex := 0
	return re.ReplaceAllStringFunc(html, func(match string) string {
		itemID := r.generaterangeItemID(rangeFragment.ContainerID, itemIndex)
		itemIndex++

		// Extract the full <a> tag
		submatches := re.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			aTag := submatches[1]
			return fmt.Sprintf(`<div id="%s">%s</div>`, itemID, aTag)
		}
		return match
	})
}

// wrapRangeContainer wraps the container element (ul, ol, nav, etc.) with the range fragment ID
func (r *Renderer) wrapRangeContainer(html string, rangeFragment *rangeFragment) string {
	// For list containers, we should add the ID to the existing element rather than wrapping with a div
	// This maintains valid HTML structure

	listPatterns := []struct {
		pattern     string
		replacement string
	}{
		// Unordered lists - add ID to existing ul tag
		{`(<ul)([^>]*>)(.*?)(</ul>)`, fmt.Sprintf(`$1 id="%s"$2$3$4`, rangeFragment.ContainerID)},
		// Ordered lists - add ID to existing ol tag
		{`(<ol)([^>]*>)(.*?)(</ol>)`, fmt.Sprintf(`$1 id="%s"$2$3$4`, rangeFragment.ContainerID)},
		// Navigation elements - add ID to existing nav tag
		{`(<nav)([^>]*>)(.*?)(</nav>)`, fmt.Sprintf(`$1 id="%s"$2$3$4`, rangeFragment.ContainerID)},
	}

	for _, listPattern := range listPatterns {
		re := regexp.MustCompile(`(?s)` + listPattern.pattern) // (?s) makes . match newlines
		if re.MatchString(html) {
			return re.ReplaceAllString(html, listPattern.replacement)
		}
	}

	// Fallback: wrap with div for other container types
	containerPatterns := []string{
		`(<div[^>]*>)(.*?)(</div>)`, // Generic div containers
	}

	for _, pattern := range containerPatterns {
		re := regexp.MustCompile(`(?s)` + pattern) // (?s) makes . match newlines
		if re.MatchString(html) {
			return re.ReplaceAllString(html, fmt.Sprintf(`$1<div id="%s">$2</div>$3`, rangeFragment.ContainerID))
		}
	}

	return html
}

// wrapGenericrangeItems wraps individual range items with unique fragment IDs
func (r *Renderer) wrapGenericrangeItems(html string, rangeFragment *rangeFragment) string {
	itemIndex := 0

	// For list items, add the fragment ID directly to the li element
	liPattern := `(\s*<li)([^>]*)(>.*?</li>)`
	liRe := regexp.MustCompile(`(?s)` + liPattern)
	if liRe.MatchString(html) {
		return liRe.ReplaceAllStringFunc(html, func(match string) string {
			itemID := r.generaterangeItemID(rangeFragment.ContainerID, itemIndex)
			itemIndex++

			// Add the fragment ID to the li element
			submatches := liRe.FindStringSubmatch(match)
			if len(submatches) >= 4 {
				liStart := submatches[1]       // "<li"
				existingAttrs := submatches[2] // existing attributes
				liContent := submatches[3]     // ">content</li>"

				// Add the fragment ID to the existing attributes
				return fmt.Sprintf(`%s id="%s"%s%s`, liStart, itemID, existingAttrs, liContent)
			}
			return match
		})
	}

	// Fallback patterns for other item types that need div wrapping
	itemPatterns := []string{
		`(\s*<div[^>]*data-id[^>]*>.*?</div>)`, // Divs with data-id
		`(\s*<article[^>]*>.*?</article>)`,     // Article elements
		`(\s*<section[^>]*>.*?</section>)`,     // Section elements
	}

	for _, pattern := range itemPatterns {
		re := regexp.MustCompile(`(?s)` + pattern) // (?s) makes . match newlines
		if re.MatchString(html) {
			return re.ReplaceAllStringFunc(html, func(match string) string {
				itemID := r.generaterangeItemID(rangeFragment.ContainerID, itemIndex)
				itemIndex++

				// Wrap the entire matched element with a div containing the fragment ID
				return fmt.Sprintf(`<div id="%s">%s</div>`, itemID, match)
			})
		}
	}

	return html
}

// wrapconditionalFragment wraps conditional content (if/with blocks) with fragment IDs
func (r *Renderer) wrapconditionalFragment(html string, fragment *templateFragment, fragmentID string) string {
	// For conditional blocks, we need to wrap the rendered output
	// This is more complex as we need to detect what content was actually rendered

	// Look for common conditional content patterns
	conditionalPatterns := []string{
		`(\s*<div[^>]*class="[^"]*conditional[^"]*"[^>]*>.*?</div>)`, // Div with conditional class
		`(\s*<section[^>]*>.*?</section>)`,                           // Section elements
		`(\s*<p[^>]*>.*?</p>)`,                                       // Paragraph elements
		`(\s*<span[^>]*>.*?</span>)`,                                 // Span elements
	}

	for _, pattern := range conditionalPatterns {
		re := regexp.MustCompile(`(?s)` + pattern) // (?s) makes . match newlines
		if re.MatchString(html) {
			return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
		}
	}

	// Fallback: wrap any block-level content that might be from the conditional
	blockContentPattern := `(\s*<[^>]+>.*?</[^>]+>)`
	re := regexp.MustCompile(`(?s)` + blockContentPattern)
	if re.MatchString(html) {
		return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
	}

	return html
}

// wraptemplateIncludeFragment wraps template inclusion content with fragment IDs
func (r *Renderer) wraptemplateIncludeFragment(html string, fragment *templateFragment, fragmentID string) string {
	// Template inclusions render their content directly into the output
	// We need to wrap whatever content was rendered by the included template

	// Look for common template inclusion patterns
	includePatterns := []string{
		`(\s*<div[^>]*class="[^"]*template[^"]*"[^>]*>.*?</div>)`, // Div with template class
		`(\s*<article[^>]*>.*?</article>)`,                        // Article elements
		`(\s*<section[^>]*>.*?</section>)`,                        // Section elements
		`(\s*<aside[^>]*>.*?</aside>)`,                            // Aside elements
		`(\s*<header[^>]*>.*?</header>)`,                          // Header elements
		`(\s*<footer[^>]*>.*?</footer>)`,                          // Footer elements
	}

	for _, pattern := range includePatterns {
		re := regexp.MustCompile(`(?s)` + pattern) // (?s) makes . match newlines
		if re.MatchString(html) {
			return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
		}
	}

	// Fallback: wrap any content that might be from the template inclusion
	contentPattern := `(\s*<[^>]+>.*?</[^>]+>)`
	re := regexp.MustCompile(`(?s)` + contentPattern)
	if re.MatchString(html) {
		return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
	}

	return html
}

// wrapGenericFragment wraps any HTML elements that contain template variables
func (r *Renderer) wrapGenericFragment(html string, fragment *templateFragment, fragmentID string) string {
	// For generic fragments, we need to identify lines/elements that would change when data updates
	content := strings.TrimSpace(fragment.Content)

	// If this fragment has template variables, try to find the corresponding rendered content
	if strings.Contains(content, "{{") {
		// Create a pattern to match the rendered output based on the fragment's content line

		// Handle variable assignments (e.g., {{$title := .Title}})
		if strings.Contains(content, ":=") {
			// This is a variable assignment, skip wrapping as it doesn't produce output
			return html
		}

		// Handle direct template expressions that produce HTML output
		// Extract the HTML structure from the fragment content and match it precisely

		// Remove template expressions from the line to get the HTML structure pattern
		htmlPattern := content
		htmlPattern = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(htmlPattern, `[^<]*`)
		htmlPattern = strings.TrimSpace(htmlPattern)

		if htmlPattern != "" && strings.Contains(htmlPattern, "<") {
			// Convert the line pattern to a regex that matches the rendered output
			// Escape special regex characters except for our [^<]* placeholders
			htmlPattern = regexp.QuoteMeta(htmlPattern)
			htmlPattern = strings.ReplaceAll(htmlPattern, `\[\^<\]\*`, `[^<]*`)

			// Only wrap if this specific pattern hasn't been wrapped yet
			if !strings.Contains(html, fmt.Sprintf(`id="%s"`, fragmentID)) {
				re := regexp.MustCompile(htmlPattern)
				if re.MatchString(html) {
					match := re.FindString(html)
					if match != "" {
						replacement := fmt.Sprintf(`<div id="%s">%s</div>`, fragmentID, match)
						html = strings.Replace(html, match, replacement, 1)
					}
				}
			}
		}
	}

	return html
}

// elementContainsFragmentContent checks if an HTML element contains content related to fragment dependencies
func (r *Renderer) elementContainsFragmentContent(element string, fragment *templateFragment) bool {
	// This is a heuristic to match rendered HTML elements with their template fragment
	// For now, we'll be permissive and assume the first matching element pattern is ours
	// TODO: Could be made more sophisticated by analyzing the template structure
	return true
}

// extractBlockName extracts block name from template content
func (r *Renderer) extractBlockName(content string) string {
	blockRegex := regexp.MustCompile(`\{\{block\s+"([^"]+)"`)
	matches := blockRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// processUpdates processes incoming data updates and determines which fragments need updating
func (r *Renderer) processUpdates() {
	for {
		select {
		case newData := <-r.updateChan:
			r.handleDataUpdate(newData)
		case <-r.stopChan:
			return
		}
	}
}

// handleDataUpdate handles a single data update
func (r *Renderer) handleDataUpdate(newData interface{}) {
	r.dataMutex.Lock()
	oldData := r.currentData
	r.currentData = newData
	r.dataMutex.Unlock()

	// Find which fields changed using the tracker
	changedFields := r.tracker.DetectChanges(oldData, newData)
	if len(changedFields) == 0 {
		return // No changes
	}

	// Handle regular fragment updates
	affectedFragments := r.findAffectedFragments(changedFields)
	for _, fragmentInfo := range affectedFragments {
		update, err := r.renderFragmentUpdate(fragmentInfo, newData)
		if err != nil {
			continue // Skip failed renders
		}

		select {
		case r.outputChan <- update:
		default:
			// Channel full, skip to prevent blocking
		}
	}

	// Handle range fragment updates (for granular list operations)
	rangeUpdates := r.handleRangeUpdates(oldData, newData, changedFields)
	for _, update := range rangeUpdates {
		select {
		case r.outputChan <- update:
		default:
			// Channel full, skip to prevent blocking
		}
	}
}

// handleRangeUpdates processes changes to range fragments and generates granular list updates
func (r *Renderer) handleRangeUpdates(oldData, newData interface{}, changedFields []string) []Update {
	var updates []Update

	// Check each template's range fragments
	for _, rangeFragments := range r.rangeFragments {
		for _, rangeFragment := range rangeFragments {
			// Check if this range fragment's data changed
			if r.rangePathChanged(rangeFragment.RangePath, changedFields) {
				rangeUpdates := r.processRangeChanges(rangeFragment, oldData, newData)
				updates = append(updates, rangeUpdates...)
			}
		}
	}

	return updates
}

// rangePathChanged checks if any of the changed fields affects this range path
func (r *Renderer) rangePathChanged(rangePath string, changedFields []string) bool {
	for _, field := range changedFields {
		if strings.HasPrefix(field, rangePath) || strings.HasPrefix(rangePath, field) {
			return true
		}
	}
	return false
}

// processRangeChanges compares old and new range data to generate granular updates
func (r *Renderer) processRangeChanges(rangeFragment *rangeFragment, oldData, newData interface{}) []Update {
	var updates []Update

	// Extract the array data from both old and new data
	oldItems := r.extractRangeData(oldData, rangeFragment.RangePath)
	newItems := r.extractRangeData(newData, rangeFragment.RangePath)

	// For now, let's create a simple comparison based on indices
	// In a more sophisticated version, we'd use proper key-based tracking

	oldLen := len(oldItems)
	newLen := len(newItems)

	if newLen > oldLen {
		// Items were added
		for i := oldLen; i < newLen; i++ {
			itemHTML, err := r.renderrangeItem(rangeFragment, newItems[i], i)
			if err != nil {
				continue
			}

			itemID := r.generaterangeItemID(rangeFragment.ContainerID, i)

			// For simple item types (strings, numbers), use the item value as the key
			// For complex types, use the index-based key
			var itemKey string
			if itemStr, ok := newItems[i].(string); ok {
				itemKey = itemStr
			} else {
				itemKey = fmt.Sprintf("%s_%d", rangeFragment.ContainerID, i)
			}

			update := Update{
				FragmentID: itemID,
				HTML:       itemHTML,
				Action:     "append",
				RangeInfo: &RangeInfo{
					ItemKey: itemKey,
				},
			}
			updates = append(updates, update)
		}
	} else if newLen < oldLen {
		// Items were removed
		for i := newLen; i < oldLen; i++ {
			itemID := r.generaterangeItemID(rangeFragment.ContainerID, i)

			// For simple item types, use the item value as the key
			// For removed items, we need to get the old item value
			var itemKey string
			if i < len(oldItems) {
				if itemStr, ok := oldItems[i].(string); ok {
					itemKey = itemStr
				} else {
					itemKey = fmt.Sprintf("%s_%d", rangeFragment.ContainerID, i)
				}
			} else {
				itemKey = fmt.Sprintf("%s_%d", rangeFragment.ContainerID, i)
			}

			update := Update{
				FragmentID: itemID,
				Action:     "remove",
				RangeInfo: &RangeInfo{
					ItemKey: itemKey,
				},
			}
			updates = append(updates, update)
		}
	}

	// Check for modifications to existing items
	minLen := newLen
	if oldLen < newLen {
		minLen = oldLen
	}

	for i := 0; i < minLen; i++ {
		if !r.itemsEqual(oldItems[i], newItems[i]) {
			itemHTML, err := r.renderrangeItem(rangeFragment, newItems[i], i)
			if err != nil {
				continue
			}

			itemID := r.generaterangeItemID(rangeFragment.ContainerID, i)

			// For simple item types, use the item value as the key
			var itemKey string
			if itemStr, ok := newItems[i].(string); ok {
				itemKey = itemStr
			} else {
				itemKey = fmt.Sprintf("%s_%d", rangeFragment.ContainerID, i)
			}

			update := Update{
				FragmentID: itemID,
				HTML:       itemHTML,
				Action:     "replace",
				RangeInfo: &RangeInfo{
					ItemKey: itemKey,
				},
			}
			updates = append(updates, update)
		}
	}

	return updates
}

// fragmentInfo contains information about a fragment that needs updating
type fragmentInfo struct {
	ID           string
	TemplateName string
	Fragment     *templateFragment
}

// findAffectedFragments finds fragments that depend on the changed fields
func (r *Renderer) findAffectedFragments(changedFields []string) []fragmentInfo {
	var affected []fragmentInfo
	seen := make(map[string]bool) // Deduplicate fragments

	for templateName, fragments := range r.fragmentStore {
		for _, fragment := range fragments {
			// Check if this fragment depends on any changed field
			fragmentMatches := false
			for _, changedField := range changedFields {
				for _, dependency := range fragment.Dependencies {
					if r.fieldMatches(dependency, changedField) {
						fragmentMatches = true
						break
					}
				}
				if fragmentMatches {
					break
				}
			}

			if fragmentMatches {
				fragmentID := fragment.ID
				if blockName := r.extractBlockName(fragment.Content); blockName != "" {
					fragmentID = blockName
				}

				// Deduplicate by fragment ID
				fragmentKey := fmt.Sprintf("%s:%s", templateName, fragmentID)
				if !seen[fragmentKey] {
					seen[fragmentKey] = true
					affected = append(affected, fragmentInfo{
						ID:           fragmentID,
						TemplateName: templateName,
						Fragment:     fragment,
					})
				}
			}
		}
	}

	return affected
}

// fieldMatches checks if a dependency matches a changed field
func (r *Renderer) fieldMatches(dependency, changedField string) bool {
	// Direct match
	if dependency == changedField {
		return true
	}

	// Check if changed field is a parent of dependency
	if strings.HasPrefix(dependency, changedField+".") {
		return true
	}

	// Check if dependency is a parent of changed field
	if strings.HasPrefix(changedField, dependency+".") {
		return true
	}

	return false
}

// renderFragmentUpdate renders a single fragment update
func (r *Renderer) renderFragmentUpdate(fragmentInfo fragmentInfo, data interface{}) (Update, error) {
	// Create a temporary template for just this fragment
	tmpl, err := template.New("fragment").Parse(fragmentInfo.Fragment.Content)
	if err != nil {
		return Update{}, fmt.Errorf("failed to parse fragment template: %w", err)
	}

	// Render the fragment
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return Update{}, fmt.Errorf("failed to execute fragment template: %w", err)
	}

	return Update{
		FragmentID: fragmentInfo.ID,
		HTML:       buf.String(),
		Action:     "replace",
	}, nil
}

// GetFragmentCount returns the number of fragments across all templates
func (r *Renderer) GetFragmentCount() int {
	count := 0
	for _, fragments := range r.fragmentStore {
		count += len(fragments)
	}
	return count
}

// GetFragmentIDs returns all fragment IDs for debugging/inspection
func (r *Renderer) GetFragmentIDs() map[string][]string {
	result := make(map[string][]string)

	for templateName, fragments := range r.fragmentStore {
		var ids []string

		for _, fragment := range fragments {
			fragmentID := fragment.ID
			if blockName := r.extractBlockName(fragment.Content); blockName != "" {
				fragmentID = blockName
			}
			ids = append(ids, fragmentID)
		}

		result[templateName] = ids
	}

	return result
}

// GetFragmentDetails returns detailed information about fragments for debugging
func (r *Renderer) GetFragmentDetails() map[string]map[string][]string {
	result := make(map[string]map[string][]string)

	for templateName, fragments := range r.fragmentStore {
		result[templateName] = make(map[string][]string)

		for _, fragment := range fragments {
			fragmentID := fragment.ID
			if blockName := r.extractBlockName(fragment.Content); blockName != "" {
				fragmentID = blockName
			}
			result[templateName][fragmentID] = fragment.Dependencies
		}
	}

	return result
}

// extractRangeData extracts array data from a data structure using a dot-separated path
func (r *Renderer) extractRangeData(data interface{}, rangePath string) []interface{} {
	if data == nil {
		return nil
	}

	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Split the path and navigate (e.g., "Navigation.MainItems")
	parts := strings.Split(rangePath, ".")
	for _, part := range parts {
		if value.Kind() == reflect.Struct {
			value = value.FieldByName(part)
			if !value.IsValid() {
				return nil
			}
		} else {
			return nil
		}
	}

	// Convert slice/array to []interface{}
	if value.Kind() == reflect.Slice {
		result := make([]interface{}, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = value.Index(i).Interface()
		}
		return result
	}

	return nil
}

// renderrangeItem renders a single item within a range loop
func (r *Renderer) renderrangeItem(rangeFragment *rangeFragment, itemData interface{}, index int) (string, error) {
	// Replace range variables with appropriate data access
	// $item becomes . (current data context)
	// $index becomes the actual index value
	itemTemplate := rangeFragment.ItemTemplate
	itemTemplate = strings.ReplaceAll(itemTemplate, "$item", ".")
	itemTemplate = strings.ReplaceAll(itemTemplate, "$index", fmt.Sprintf("%d", index))

	// Create a temporary template for the item
	tmpl, err := template.New("item").Parse(itemTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse item template: %w", err)
	}

	// Render the item with its data
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, itemData); err != nil {
		return "", fmt.Errorf("failed to execute item template: %w", err)
	}

	return buf.String(), nil
}

// generaterangeItemID generates a unique ID for a range item
func (r *Renderer) generaterangeItemID(containerID string, index int) string {
	return fmt.Sprintf("%s-item-%d", containerID, index)
}

// itemsEqual compares two items to see if they are equal
func (r *Renderer) itemsEqual(oldItem, newItem interface{}) bool {
	return reflect.DeepEqual(oldItem, newItem)
}
