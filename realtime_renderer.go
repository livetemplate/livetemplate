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

// RealtimeUpdate represents an update that can be sent to the client
type RealtimeUpdate struct {
	FragmentID  string `json:"fragment_id"`            // The ID of the div/element to update
	HTML        string `json:"html"`                   // The new HTML content for that fragment
	Action      string `json:"action"`                 // "replace", "append", "prepend", "insertafter", "insertbefore", "remove"
	TargetID    string `json:"target_id,omitempty"`    // For insertafter/insertbefore actions - which element to insert relative to
	ItemIndex   int    `json:"item_index,omitempty"`   // For range items - the index within the array
	ItemKey     string `json:"item_key,omitempty"`     // For range items - unique key for the item (e.g., URL for nav items)
	ContainerID string `json:"container_id,omitempty"` // For range operations - the ID of the containing range fragment
}

// RangeItem represents a single item within a range loop
type RangeItem struct {
	ID    string      // Unique ID for this specific item instance
	Index int         // Position in the array
	Key   string      // Unique key for the item (e.g., URL, ID field)
	Data  interface{} // The actual item data
	HTML  string      // Rendered HTML for this item
}

// RangeFragment represents a fragment that contains a range loop
type RangeFragment struct {
	*TemplateFragment
	RangePath    string                // The path to the array (e.g., "Navigation.MainItems")
	ItemTemplate string                // The template content for individual items
	Items        map[string]*RangeItem // Current items keyed by their unique key
	ContainerID  string                // ID of the container element
}

// RealtimeRenderer handles real-time template rendering with fragment targeting
type RealtimeRenderer struct {
	templates       map[string]*template.Template
	fragmentTracker *FragmentExtractor
	tracker         *TemplateTracker // For change detection
	currentData     interface{}
	dataMutex       sync.RWMutex
	updateChan      chan interface{}
	outputChan      chan RealtimeUpdate
	running         bool
	stopChan        chan bool
	wrapperPattern  string                         // Pattern for wrapping fragments with IDs
	fragmentStore   map[string][]*TemplateFragment // Store fragments by template name
	rangeFragments  map[string][]*RangeFragment    // Store range-specific fragments by template name
}

// RealtimeConfig configures the real-time renderer
type RealtimeConfig struct {
	WrapperTag     string // HTML tag to wrap fragments (default: "div")
	IDPrefix       string // Prefix for fragment IDs (default: "fragment-")
	PreserveBlocks bool   // Whether to preserve block names as IDs when possible
}

// NewRealtimeRenderer creates a new real-time renderer
func NewRealtimeRenderer(config *RealtimeConfig) *RealtimeRenderer {
	if config == nil {
		config = &RealtimeConfig{
			WrapperTag:     "div",
			IDPrefix:       "fragment-",
			PreserveBlocks: true,
		}
	}

	return &RealtimeRenderer{
		templates:       make(map[string]*template.Template),
		fragmentTracker: NewFragmentExtractor(),
		tracker:         NewTemplateTracker(),
		updateChan:      make(chan interface{}, 100),
		outputChan:      make(chan RealtimeUpdate, 100),
		stopChan:        make(chan bool),
		wrapperPattern:  fmt.Sprintf("<%s id=\"%%s\">%%s</%s>", config.WrapperTag, config.WrapperTag),
		fragmentStore:   make(map[string][]*TemplateFragment),
		rangeFragments:  make(map[string][]*RangeFragment),
	}
}

// AddTemplate adds a template for real-time rendering
func (r *RealtimeRenderer) AddTemplate(name, content string) error {
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

// createSimpleFragments creates fragments from template expressions
func (r *RealtimeRenderer) createSimpleFragments(content, templateName string) []*TemplateFragment {
	var fragments []*TemplateFragment

	// Use the tracker's analyzer to find dependencies for the whole template
	tmpl, err := template.New("temp").Parse(content)
	if err != nil {
		return fragments
	}

	analyzer := NewAdvancedTemplateAnalyzer()
	allDependencies := analyzer.AnalyzeTemplate(tmpl)

	// Create granular fragments for individual template expressions
	fragments = r.createGranularFragments(content, templateName, allDependencies)

	// Create range fragments for loop constructs
	rangeFragments := r.createRangeFragments(content, templateName, allDependencies)
	r.rangeFragments[templateName] = rangeFragments

	// Create conditional fragments for if/with blocks
	conditionalFragments := r.createConditionalFragments(content, templateName, allDependencies)
	for _, condFragment := range conditionalFragments {
		fragments = append(fragments, condFragment.TemplateFragment)
	}

	// Create template include fragments
	includeFragments := r.createTemplateIncludeFragments(content, templateName, allDependencies)
	for _, includeFragment := range includeFragments {
		fragments = append(fragments, includeFragment.TemplateFragment)
	}

	// Try to identify block fragments separately
	r.addBlockFragments(&fragments, content, templateName, allDependencies)

	// If no logical fragments were found, create a single fragment for the entire template
	if len(fragments) == 0 {
		fragment := &TemplateFragment{
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
func (r *RealtimeRenderer) createGranularFragments(content, templateName string, allDependencies []string) []*TemplateFragment {
	var fragments []*TemplateFragment

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
		} // Find dependencies for this specific line
		lineDependencies := r.findLineDependencies(line, allDependencies)
		if len(lineDependencies) > 0 {
			fragment := &TemplateFragment{
				ID:           r.generateShortID(),
				Content:      line,
				Dependencies: lineDependencies,
				StartPos:     i,
				EndPos:       i + 1,
			}
			fragments = append(fragments, fragment)
		}
	}

	return fragments
}

// findLineDependencies finds which dependencies are used in a specific line
func (r *RealtimeRenderer) findLineDependencies(line string, allDependencies []string) []string {
	var lineDeps []string

	for _, dep := range allDependencies {
		templateRef := fmt.Sprintf("{{.%s}}", dep)
		if strings.Contains(line, templateRef) {
			lineDeps = append(lineDeps, dep)
		}
	}

	return lineDeps
}

// createRangeFragments creates fragments for range loops to enable granular list operations
func (r *RealtimeRenderer) createRangeFragments(content, templateName string, allDependencies []string) []*RangeFragment {
	var rangeFragments []*RangeFragment

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
			rangeRegex := regexp.MustCompile(`\{\{range\s+\.([^}]+)\}\}`)
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

					fragment := &RangeFragment{
						TemplateFragment: &TemplateFragment{
							ID:           containerID,
							Content:      strings.Join(lines[rangeStart:rangeEnd+1], "\n"),
							Dependencies: []string{rangePath},
							StartPos:     rangeStart,
							EndPos:       rangeEnd + 1,
						},
						RangePath:    rangePath,
						ItemTemplate: itemTemplate,
						Items:        make(map[string]*RangeItem),
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

// ConditionalFragment represents a fragment that contains conditional logic
type ConditionalFragment struct {
	*TemplateFragment
	ConditionPath string // The path to the condition (e.g., "User.IsLoggedIn")
	TrueContent   string // Content when condition is true
	FalseContent  string // Content when condition is false (else block)
	FragmentType  string // "if", "with"
}

// TemplateIncludeFragment represents a fragment that includes another template
type TemplateIncludeFragment struct {
	*TemplateFragment
	TemplateName string // Name of the included template
	DataPath     string // Path to the data passed to template (optional)
}

// createConditionalFragments creates fragments for if/with conditional blocks
func (r *RealtimeRenderer) createConditionalFragments(content, templateName string, allDependencies []string) []*ConditionalFragment {
	var conditionalFragments []*ConditionalFragment

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
			fragment := &ConditionalFragment{
				TemplateFragment: &TemplateFragment{
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

// createTemplateIncludeFragments creates fragments for template inclusion blocks
func (r *RealtimeRenderer) createTemplateIncludeFragments(content, templateName string, allDependencies []string) []*TemplateIncludeFragment {
	var includeFragments []*TemplateIncludeFragment

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

				fragment := &TemplateIncludeFragment{
					TemplateFragment: &TemplateFragment{
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
func (r *RealtimeRenderer) extractConditionPath(line, conditionType string) string {
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
func (r *RealtimeRenderer) extractTemplateIncludeInfo(line string) (templateName, dataPath string) {
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
func (r *RealtimeRenderer) generateShortID() string {
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
func (r *RealtimeRenderer) detectPatternFragments(content, templateName string, allDependencies []string) []*TemplateFragment {
	var fragments []*TemplateFragment
	// This could be extended to detect common UI patterns
	// For now, we rely on granular line-based detection
	return fragments
}

// addBlockFragments identifies and adds block fragments
func (r *RealtimeRenderer) addBlockFragments(fragments *[]*TemplateFragment, content, templateName string, allDeps []string) {
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
				fragment := &TemplateFragment{
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
func (r *RealtimeRenderer) detectSubFragmentsInBlock(blockContent, templateName, blockName string, allDeps []string) []*TemplateFragment {
	var fragments []*TemplateFragment

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
			fragment := &TemplateFragment{
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
func (r *RealtimeRenderer) extractFieldSectionFromBlock(blockContent string, fieldDeps []string) string {
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
func (r *RealtimeRenderer) generateSimpleID() string {
	// Simple ID generation for fallback
	return fmt.Sprintf("frag-%d", time.Now().UnixNano()%1000000)
}

// SetInitialData sets the initial data and returns the full rendered HTML
func (r *RealtimeRenderer) SetInitialData(data interface{}) (string, error) {
	r.dataMutex.Lock()
	r.currentData = data
	r.dataMutex.Unlock()

	return r.renderFullHTML()
}

// GetUpdateChannel returns the channel for receiving real-time updates
func (r *RealtimeRenderer) GetUpdateChannel() <-chan RealtimeUpdate {
	return r.outputChan
}

// SendUpdate sends new data that may trigger fragment updates
func (r *RealtimeRenderer) SendUpdate(newData interface{}) {
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
func (r *RealtimeRenderer) Start() {
	r.running = true
	go r.processUpdates()
}

// Stop stops processing updates
func (r *RealtimeRenderer) Stop() {
	r.running = false
	close(r.stopChan)
}

// renderFullHTML renders the complete HTML with fragment IDs
func (r *RealtimeRenderer) renderFullHTML() (string, error) {
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
func (r *RealtimeRenderer) wrapRenderedFragments(renderedHTML, templateName string) string {
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
		} else if r.isConditionalFragment(fragment) {
			result = r.wrapConditionalFragment(result, fragment, fragmentID)
		} else if r.isTemplateIncludeFragment(fragment) {
			result = r.wrapTemplateIncludeFragment(result, fragment, fragmentID)
		}
	}

	// Process range fragments by wrapping individual items within the range
	for _, rangeFragment := range rangeFragments {
		result = r.wrapRangeFragmentItems(result, rangeFragment)
	}

	return result
}

// isCounterFieldFragment checks if this fragment contains a specific counter field
func (r *RealtimeRenderer) isCounterFieldFragment(fragment *TemplateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Counter.") {
			return true
		}
	}
	return false
}

// wrapCounterFieldFragment wraps individual counter field lines with div IDs
func (r *RealtimeRenderer) wrapCounterFieldFragment(html string, fragment *TemplateFragment, fragmentID string) string {
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
func (r *RealtimeRenderer) isSiteFragment(fragment *TemplateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Site.") {
			return true
		}
	}
	return false
}

// isNavigationFragment checks if this fragment contains navigation-related dependencies
func (r *RealtimeRenderer) isNavigationFragment(fragment *TemplateFragment) bool {
	for _, dep := range fragment.Dependencies {
		if strings.HasPrefix(dep, "Navigation.") || dep == "URL" || dep == "Label" {
			return true
		}
	}
	return false
}

// isConditionalFragment checks if this fragment contains conditional logic (if/with)
func (r *RealtimeRenderer) isConditionalFragment(fragment *TemplateFragment) bool {
	return strings.Contains(fragment.Content, "{{if") || strings.Contains(fragment.Content, "{{with")
}

// isTemplateIncludeFragment checks if this fragment includes another template
func (r *RealtimeRenderer) isTemplateIncludeFragment(fragment *TemplateFragment) bool {
	return strings.Contains(fragment.Content, "{{template")
}

// wrapSiteFragment wraps the site heading with a div ID
func (r *RealtimeRenderer) wrapSiteFragment(html, fragmentID string) string {
	// Look for h1 tags containing the site name
	re := regexp.MustCompile(`(\s*<h1>.*?</h1>)`)
	return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
}

// wrapNavigationFragment wraps the navigation section with a div ID
func (r *RealtimeRenderer) wrapNavigationFragment(html, fragmentID string) string {
	// Look for nav tags and their content
	re := regexp.MustCompile(`(\s*<nav>.*?</nav>)`)
	return re.ReplaceAllString(html, fmt.Sprintf(`<div id="%s">$1</div>`, fragmentID))
}

// wrapRangeFragmentItems wraps individual items within a range fragment with unique IDs
func (r *RealtimeRenderer) wrapRangeFragmentItems(html string, rangeFragment *RangeFragment) string {
	// First, wrap the entire range container (e.g., <ul>, <nav>, etc.)
	html = r.wrapRangeContainer(html, rangeFragment)

	// Then wrap individual items based on the range type
	if strings.Contains(rangeFragment.RangePath, "Navigation") {
		return r.wrapNavigationItems(html, rangeFragment)
	}

	// Generic range item wrapping for any list items
	return r.wrapGenericRangeItems(html, rangeFragment)
}

// wrapNavigationItems wraps individual navigation items with unique IDs
func (r *RealtimeRenderer) wrapNavigationItems(html string, rangeFragment *RangeFragment) string {
	// Pattern to match individual navigation links
	re := regexp.MustCompile(`(\s*<a href="([^"]*)"[^>]*>([^<]*)</a>)`)

	itemIndex := 0
	return re.ReplaceAllStringFunc(html, func(match string) string {
		itemID := r.generateRangeItemID(rangeFragment.ContainerID, itemIndex)
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
func (r *RealtimeRenderer) wrapRangeContainer(html string, rangeFragment *RangeFragment) string {
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

// wrapGenericRangeItems wraps individual range items with unique fragment IDs
func (r *RealtimeRenderer) wrapGenericRangeItems(html string, rangeFragment *RangeFragment) string {
	itemIndex := 0

	// For list items, add the fragment ID directly to the li element
	liPattern := `(\s*<li)([^>]*)(>.*?</li>)`
	liRe := regexp.MustCompile(`(?s)` + liPattern)
	if liRe.MatchString(html) {
		return liRe.ReplaceAllStringFunc(html, func(match string) string {
			itemID := r.generateRangeItemID(rangeFragment.ContainerID, itemIndex)
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
				itemID := r.generateRangeItemID(rangeFragment.ContainerID, itemIndex)
				itemIndex++

				// Wrap the entire matched element with a div containing the fragment ID
				return fmt.Sprintf(`<div id="%s">%s</div>`, itemID, match)
			})
		}
	}

	return html
}

// wrapConditionalFragment wraps conditional content (if/with blocks) with fragment IDs
func (r *RealtimeRenderer) wrapConditionalFragment(html string, fragment *TemplateFragment, fragmentID string) string {
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

// wrapTemplateIncludeFragment wraps template inclusion content with fragment IDs
func (r *RealtimeRenderer) wrapTemplateIncludeFragment(html string, fragment *TemplateFragment, fragmentID string) string {
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

// extractBlockName extracts block name from template content
func (r *RealtimeRenderer) extractBlockName(content string) string {
	blockRegex := regexp.MustCompile(`\{\{block\s+"([^"]+)"`)
	matches := blockRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// processUpdates processes incoming data updates and determines which fragments need updating
func (r *RealtimeRenderer) processUpdates() {
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
func (r *RealtimeRenderer) handleDataUpdate(newData interface{}) {
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
func (r *RealtimeRenderer) handleRangeUpdates(oldData, newData interface{}, changedFields []string) []RealtimeUpdate {
	var updates []RealtimeUpdate

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
func (r *RealtimeRenderer) rangePathChanged(rangePath string, changedFields []string) bool {
	for _, field := range changedFields {
		if strings.HasPrefix(field, rangePath) || strings.HasPrefix(rangePath, field) {
			return true
		}
	}
	return false
}

// processRangeChanges compares old and new range data to generate granular updates
func (r *RealtimeRenderer) processRangeChanges(rangeFragment *RangeFragment, oldData, newData interface{}) []RealtimeUpdate {
	var updates []RealtimeUpdate

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
			itemHTML, err := r.renderRangeItem(rangeFragment, newItems[i], i)
			if err != nil {
				continue
			}

			itemID := r.generateRangeItemID(rangeFragment.ContainerID, i)
			update := RealtimeUpdate{
				FragmentID:  itemID,
				HTML:        itemHTML,
				Action:      "append",
				ContainerID: rangeFragment.ContainerID,
				ItemIndex:   i,
			}
			updates = append(updates, update)
		}
	} else if newLen < oldLen {
		// Items were removed
		for i := newLen; i < oldLen; i++ {
			itemID := r.generateRangeItemID(rangeFragment.ContainerID, i)
			update := RealtimeUpdate{
				FragmentID:  itemID,
				Action:      "remove",
				ContainerID: rangeFragment.ContainerID,
				ItemIndex:   i,
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
			itemHTML, err := r.renderRangeItem(rangeFragment, newItems[i], i)
			if err != nil {
				continue
			}

			itemID := r.generateRangeItemID(rangeFragment.ContainerID, i)
			update := RealtimeUpdate{
				FragmentID:  itemID,
				HTML:        itemHTML,
				Action:      "replace",
				ContainerID: rangeFragment.ContainerID,
				ItemIndex:   i,
			}
			updates = append(updates, update)
		}
	}

	return updates
}

// FragmentInfo contains information about a fragment that needs updating
type FragmentInfo struct {
	ID           string
	TemplateName string
	Fragment     *TemplateFragment
}

// findAffectedFragments finds fragments that depend on the changed fields
func (r *RealtimeRenderer) findAffectedFragments(changedFields []string) []FragmentInfo {
	var affected []FragmentInfo
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
					affected = append(affected, FragmentInfo{
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
func (r *RealtimeRenderer) fieldMatches(dependency, changedField string) bool {
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
func (r *RealtimeRenderer) renderFragmentUpdate(fragmentInfo FragmentInfo, data interface{}) (RealtimeUpdate, error) {
	// Create a temporary template for just this fragment
	tmpl, err := template.New("fragment").Parse(fragmentInfo.Fragment.Content)
	if err != nil {
		return RealtimeUpdate{}, fmt.Errorf("failed to parse fragment template: %w", err)
	}

	// Render the fragment
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return RealtimeUpdate{}, fmt.Errorf("failed to execute fragment template: %w", err)
	}

	return RealtimeUpdate{
		FragmentID: fragmentInfo.ID,
		HTML:       buf.String(),
		Action:     "replace",
	}, nil
}

// GetFragmentCount returns the number of fragments across all templates
func (r *RealtimeRenderer) GetFragmentCount() int {
	count := 0
	for _, fragments := range r.fragmentStore {
		count += len(fragments)
	}
	return count
}

// GetFragmentIDs returns all fragment IDs for debugging/inspection
func (r *RealtimeRenderer) GetFragmentIDs() map[string][]string {
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
func (r *RealtimeRenderer) GetFragmentDetails() map[string]map[string][]string {
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
func (r *RealtimeRenderer) extractRangeData(data interface{}, rangePath string) []interface{} {
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

// renderRangeItem renders a single item within a range loop
func (r *RealtimeRenderer) renderRangeItem(rangeFragment *RangeFragment, itemData interface{}, index int) (string, error) {
	// Create a temporary template for the item
	tmpl, err := template.New("item").Parse(rangeFragment.ItemTemplate)
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

// generateRangeItemID generates a unique ID for a range item
func (r *RealtimeRenderer) generateRangeItemID(containerID string, index int) string {
	return fmt.Sprintf("%s-item-%d", containerID, index)
}

// itemsEqual compares two items to see if they are equal
func (r *RealtimeRenderer) itemsEqual(oldItem, newItem interface{}) bool {
	return reflect.DeepEqual(oldItem, newItem)
}
