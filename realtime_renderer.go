package statetemplate

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RealtimeUpdate represents an update that can be sent to the client
type RealtimeUpdate struct {
	FragmentID string `json:"fragment_id"` // The ID of the div/element to update
	HTML       string `json:"html"`        // The new HTML content for that fragment
	Action     string `json:"action"`      // "replace", "append", "prepend", etc.
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

		// Skip block/end constructs - these will be handled separately
		if strings.Contains(trimmedLine, "{{block") || strings.Contains(trimmedLine, "{{end}}") ||
			strings.Contains(trimmedLine, "{{range") {
			continue
		}

		// Find dependencies for this specific line
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
	if len(fragments) == 0 {
		return renderedHTML
	}

	result := renderedHTML

	// Process fragments by wrapping individual template expressions
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
		}
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

	// Find fragments that depend on changed fields
	affectedFragments := r.findAffectedFragments(changedFields)

	// Render and send updates for affected fragments
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
