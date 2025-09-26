package livetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Template represents a live template with caching and tree-based optimization capabilities.
// It provides an API similar to html/template.Template but with additional ExecuteUpdates method
// for generating tree-based updates that can be efficiently transmitted to clients.
type Template struct {
	name            string
	templateStr     string
	tmpl            *template.Template
	wrapperID       string
	lastData        interface{}
	lastHTML        string
	lastTree        TreeNode // Store previous tree segments for comparison
	initialTree     TreeNode
	hasInitialTree  bool
	lastFingerprint string // Fingerprint of the last generated tree for change detection
}

// New creates a new, undefined template with the given name.
// This matches the signature of html/template.New().
func New(name string) *Template {
	return &Template{
		name: name,
	}
}

// Parse parses text as a template body for the template t.
// This matches the signature of html/template.Template.Parse().
func (t *Template) Parse(text string) (*Template, error) {
	// Store the template text for tree generation
	t.templateStr = text

	// Determine if this is a full HTML document
	isFullHTML := strings.Contains(text, "<!DOCTYPE") || strings.Contains(text, "<html")

	// Always generate wrapper ID for consistent update targeting
	t.wrapperID = generateRandomID()

	var templateContent string
	if isFullHTML {
		// Inject wrapper div around body content
		templateContent = injectWrapperDiv(text, t.wrapperID)
	} else {
		// For standalone templates, wrap the entire content
		templateContent = fmt.Sprintf(`<div data-lvt-id="%s">%s</div>`, t.wrapperID, text)
	}

	// Parse the template using html/template
	tmpl, err := template.New(t.name).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	t.tmpl = tmpl
	return t, nil
}

// ParseFiles parses the named files and associates the resulting templates with t.
// This matches the signature of html/template.Template.ParseFiles().
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	// Read the first file as the main template
	content, err := os.ReadFile(filenames[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filenames[0], err)
	}

	// Use the first file's base name as template name if not already set
	if t.name == "" {
		t.name = filepath.Base(filenames[0])
	}

	// Parse the main template
	_, err = t.Parse(string(content))
	if err != nil {
		return nil, err
	}

	// Parse additional files if provided (for template composition)
	if len(filenames) > 1 {
		for _, filename := range filenames[1:] {
			content, err := os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
			}

			// Parse additional templates into the same template set
			_, err = t.tmpl.Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
			}
		}
	}

	return t, nil
}

// ParseGlob parses the template definitions from the files identified by the pattern.
// This matches the signature of html/template.Template.ParseGlob().
func (t *Template) ParseGlob(pattern string) (*Template, error) {
	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern error: %w", err)
	}

	if len(filenames) == 0 {
		return nil, fmt.Errorf("no files match pattern: %s", pattern)
	}

	return t.ParseFiles(filenames...)
}

// Execute applies a parsed template to the specified data object,
// writing the output to wr. The template is rendered as a complete HTML page
// with wrapper injection for full HTML documents.
//
// Phase 1: For full HTML documents (containing <!DOCTYPE html> or <html>),
// the body content is automatically wrapped in a div with a randomly generated data-lvt-id.
// Phase 2: The complete HTML (with wrapper) is rendered and written to wr.
func (t *Template) Execute(wr io.Writer, data interface{}) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	// Execute the template with wrapper injection already applied during Parse
	err := t.tmpl.Execute(wr, data)
	if err != nil {
		return err
	}

	// Initialize caching state for future ExecuteUpdates calls
	// Execute template again to get HTML for caching
	var buf bytes.Buffer
	execErr := t.tmpl.Execute(&buf, data)
	if execErr != nil {
		// Don't fail the main Execute call if caching setup fails
		return nil
	}

	currentHTML := buf.String()

	// Extract content from wrapper for consistent caching
	var contentToCache string
	if t.wrapperID != "" {
		contentToCache = extractTemplateContent(currentHTML, t.wrapperID)
	} else {
		contentToCache = currentHTML
	}

	// Set up caching state
	t.lastData = data
	t.lastHTML = contentToCache

	// Generate and cache initial tree structure
	_, treeErr := t.generateInitialTree(currentHTML, data)
	if treeErr != nil {
		// Don't fail if tree generation fails, just skip caching
		return nil
	}

	return nil
}

// ExecuteUpdates generates a tree structure of static and dynamic content
// that can be used by JavaScript clients to update changed parts efficiently.
//
// Caching behavior:
// - First call: Returns complete tree with static structure ("s" key) and dynamic values
// - Subsequent calls: Returns only dynamic values that have changed (cache-aware)
//
// Tree generation phases:
// 1. Compile time: Template is analyzed to separate static/dynamic parts
// 2. Runtime: Dynamic parts are hydrated with data and compared with previous state
func (t *Template) ExecuteUpdates(wr io.Writer, data interface{}) error {
	if t.tmpl == nil {
		return fmt.Errorf("template not parsed")
	}

	tree, err := t.generateTreeInternal(data)
	if err != nil {
		return fmt.Errorf("tree generation failed: %w", err)
	}

	// Convert tree to ordered JSON with readable HTML (no escape sequences)
	jsonBytes, err := marshalOrderedJSON(tree)
	if err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}

	_, err = wr.Write(jsonBytes)
	return err
}

// generateTreeInternal is the internal implementation that returns TreeNode
func (t *Template) generateTreeInternal(data interface{}) (TreeNode, error) {
	// Execute template with current data
	currentHTML, err := t.executeTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}

	// First render - no previous state
	if t.lastData == nil {
		// Extract content from wrapper for consistent caching
		var contentToCache string
		if t.wrapperID != "" {
			contentToCache = extractTemplateContent(currentHTML, t.wrapperID)
		} else {
			contentToCache = currentHTML
		}

		t.lastData = data
		t.lastHTML = contentToCache
		return t.generateInitialTree(currentHTML, data)
	}

	// Subsequent renders - use diffing approach
	return t.generateDiffBasedTree(t.lastHTML, currentHTML, t.lastData, data)
}

// executeTemplate executes the template with given data
func (t *Template) executeTemplate(data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generateInitialTree creates tree with statics and dynamics for first render
func (t *Template) generateInitialTree(html string, data interface{}) (TreeNode, error) {
	// Extract content from wrapper if we have one
	var contentToAnalyze string
	if t.wrapperID != "" {
		contentToAnalyze = extractTemplateContent(html, t.wrapperID)
	} else {
		contentToAnalyze = html
	}

	// For first render, try the new full tree parser that preserves HTML structure
	var templateContent string
	if t.wrapperID != "" {
		templateContent = extractTemplateBodyContent(t.templateStr)
	} else {
		templateContent = t.templateStr
	}

	// Use the original parser - it maintains the correct invariant and handles dynamics properly
	tree, err := parseTemplateToTree(templateContent, data)
	if err != nil {
		// Fallback to HTML structure-based strategy
		tree = t.createHTMLStructureBasedTree(contentToAnalyze)
	}

	// Cache the initial structure for future dynamics-only updates
	t.initialTree = tree
	t.hasInitialTree = true

	// Store complete tree as the baseline for comparison
	t.lastTree = tree

	// Calculate and store initial fingerprint for change detection
	t.lastFingerprint = calculateFingerprint(tree)

	// Add fingerprint to tree for client-side tracking
	return addFingerprintToTree(tree), nil
}

// mergeTreeStructures intelligently merges original tree (accurate dynamics) with full tree (complete structure)
func (t *Template) mergeTreeStructures(originalTree, fullTree TreeNode) TreeNode {
	merged := make(TreeNode)

	// Use the full tree's complete static structure
	if fullStatics, hasFull := fullTree["s"].([]string); hasFull {
		merged["s"] = fullStatics
	} else if originalStatics, hasOriginal := originalTree["s"].([]string); hasOriginal {
		merged["s"] = originalStatics
	}

	// Use the original tree's accurate dynamic values
	// The original parser is better at evaluating conditional expressions
	for k, v := range originalTree {
		if k != "s" { // Skip static structure - we use full tree's structure
			merged[k] = v
		}
	}

	// If the original tree doesn't have enough dynamic values,
	// fill in missing ones from the full tree (this handles cases where
	// the original parser missed some dynamic content)
	if fullStatics, hasFull := fullTree["s"].([]string); hasFull {
		// Count empty segments in full tree structure (these need dynamic values)
		expectedDynamics := 0
		for _, segment := range fullStatics {
			if segment == "" {
				expectedDynamics++
			}
		}

		// Fill in missing dynamics from full tree
		for i := 0; i < expectedDynamics; i++ {
			dynamicKey := fmt.Sprintf("%d", i)
			if _, hasOriginal := originalTree[dynamicKey]; !hasOriginal {
				if fullValue, hasFull := fullTree[dynamicKey]; hasFull {
					merged[dynamicKey] = fullValue
				}
			}
		}
	}

	return merged
}

// generateDiffBasedTree creates tree based on diff analysis
func (t *Template) generateDiffBasedTree(oldHTML, newHTML string, oldData, newData interface{}) (TreeNode, error) {
	// Extract content from wrapper if we have one for proper comparison
	var oldContent, newContent string
	if t.wrapperID != "" {
		oldContent = extractTemplateContent(oldHTML, t.wrapperID)
		newContent = extractTemplateContent(newHTML, t.wrapperID)
	} else {
		oldContent = oldHTML
		newContent = newHTML
	}

	// Generate new complete tree for comparison
	if t.hasInitialTree {
		// Generate complete tree with current data
		newTree, err := ParseTemplateToTree(t.templateStr, newData)
		if err != nil {
			return nil, err
		}

		// Compare trees and get only changed dynamics
		changedTree := t.compareTreesAndGetChanges(t.lastTree, newTree)

		// If no changes, return empty
		if len(changedTree) == 0 {
			return TreeNode{}, nil
		}

		// Update cached state for next comparison
		t.lastData = newData
		t.lastHTML = newContent
		t.lastTree = newTree

		return changedTree, nil
	}

	// Fallback to analyzing the change (shouldn't happen after first render)
	tree, err := t.analyzeChangeAndCreateTree(oldContent, newContent, oldData, newData)
	if err != nil {
		return nil, err
	}

	// Calculate and store fingerprint for the new tree
	newFingerprint := calculateFingerprint(tree)
	t.lastFingerprint = newFingerprint

	// Update cached state AFTER successful tree generation (use extracted content)
	t.lastData = newData
	t.lastHTML = newContent

	// Add fingerprint to tree for client-side tracking
	return addFingerprintToTree(tree), nil
}

// getChangedSegments compares newTree with lastTree and returns only changed segments
func (t *Template) getChangedSegments(newTree TreeNode) TreeNode {
	changedTree := make(TreeNode)

	// If no previous tree, return all segments (first update)
	if t.lastTree == nil {
		return newTree
	}

	// Compare each segment in newTree with lastTree
	for key, newValue := range newTree {
		// Skip static structure - never include in dynamics-only updates
		if key == "s" {
			continue
		}

		lastValue, existed := t.lastTree[key]

		// Include segment if:
		// 1. It's new (didn't exist before)
		// 2. The value changed
		if !existed || !segmentValuesEqual(lastValue, newValue) {
			changedTree[key] = newValue
		}
	}

	return changedTree
}

// segmentValuesEqual compares two segment values for equality
func segmentValuesEqual(a, b interface{}) bool {
	// Simple comparison - could be enhanced for complex types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// getTreeWithoutStatics returns a copy of the tree without the static structure
func (t *Template) getTreeWithoutStatics(tree TreeNode) TreeNode {
	result := make(TreeNode)
	for key, value := range tree {
		if key != "s" {
			result[key] = value
		}
	}
	return result
}

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree TreeNode) TreeNode {
	changes := make(TreeNode)

	// Compare dynamic segments (skip statics "s" and fingerprint "f")
	for k, newValue := range newTree {
		if k == "s" || k == "f" {
			continue // Skip static segments and fingerprint
		}

		oldValue, exists := oldTree[k]
		if !exists || !deepEqual(oldValue, newValue) {
			changes[k] = newValue
		}
	}
	return changes
}

// deepEqual compares two values deeply
func deepEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// getTreeKeys returns the keys of a TreeNode for debugging

// generateDynamicsOnlyTree creates a tree with only dynamic values, using cached structure
func (t *Template) generateDynamicsOnlyTree(data interface{}) (TreeNode, error) {
	// Try to use the fine-grained approach to extract only dynamic values
	newTree, err := parseTemplateToTreeDynamicsOnly(t.templateStr, data)
	if err == nil {
		// Compare with previous tree and return only changed segments
		changedTree := t.getChangedSegments(newTree)
		// Store the new tree for next comparison
		t.lastTree = newTree
		return changedTree, nil
	}
	// If fine-grained fails, we need to leverage the cached structure more intelligently
	currentHTML, execErr := t.executeTemplate(data)
	if execErr != nil {
		return nil, execErr
	}

	// Extract content from wrapper if we have one
	var contentToAnalyze string
	if t.wrapperID != "" {
		contentToAnalyze = extractTemplateContent(currentHTML, t.wrapperID)
	} else {
		contentToAnalyze = currentHTML
	}

	// Compare against cached HTML to find what changed
	if contentToAnalyze == t.lastHTML {
		// No changes, return empty tree
		return TreeNode{}, nil
	}

	// If we have cached initial structure, use it to create a more intelligent dynamics-only tree
	if t.hasInitialTree {
		return t.createDynamicsFromCachedStructure(contentToAnalyze)
	}

	// Last resort: return entire content as single dynamic (should rarely happen)
	return TreeNode{"0": contentToAnalyze}, nil
}

// extractDynamicsOnly removes static parts and returns only dynamic values

// createDynamicsFromCachedStructure creates dynamics-only tree by leveraging cached static structure
func (t *Template) createDynamicsFromCachedStructure(currentHTML string) (TreeNode, error) {
	// Check if the initial tree used a simple structure that we can leverage
	if t.initialTree != nil {
		if statics, hasStatics := t.initialTree["s"]; hasStatics {
			if staticParts, ok := statics.([]string); ok && len(staticParts) == 2 {
				// Handle simple prefix/suffix structure
				prefix := staticParts[0]
				suffix := staticParts[1]

				// If current HTML matches the expected pattern, extract the dynamic part
				if strings.HasPrefix(currentHTML, prefix) && strings.HasSuffix(currentHTML, suffix) {
					start := len(prefix)
					end := len(currentHTML) - len(suffix)
					if start <= end {
						dynamicPart := currentHTML[start:end]
						return TreeNode{"0": dynamicPart}, nil
					}
				}
			}
		}
	}

	// Fallback to diff-based approach
	return t.analyzeChangeAndCreateDynamicsOnly(t.lastHTML, currentHTML)
}

// analyzeChangeAndCreateDynamicsOnly creates dynamics-only tree by analyzing HTML changes
func (t *Template) analyzeChangeAndCreateDynamicsOnly(oldHTML, newHTML string) (TreeNode, error) {
	// Find common prefix and suffix to understand what changed
	commonPrefix := findCommonPrefix(oldHTML, newHTML)
	commonSuffix := findCommonSuffix(oldHTML, newHTML)

	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)

	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		return TreeNode{"0": minifyHTML(newHTML)}, nil
	}

	// If we have minimal changes, try to return just the changed part
	if changeStart > 0 || changeEnd < len(newHTML) {
		dynamicPart := newHTML[changeStart:changeEnd]
		// Return dynamics-only tree with the changed content
		return TreeNode{"0": minifyHTML(dynamicPart)}, nil
	}

	// Default to full content
	return TreeNode{"0": minifyHTML(newHTML)}, nil
}

// analyzeChangeAndCreateTree determines the best tree structure based on the type of change
func (t *Template) analyzeChangeAndCreateTree(oldHTML, newHTML string, _, _ interface{}) (TreeNode, error) {
	// Find common prefix and suffix to understand change patterns
	commonPrefix := findCommonPrefix(oldHTML, newHTML)
	commonSuffix := findCommonSuffix(oldHTML, newHTML)

	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)

	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		return TreeNode{
			"s": []string{"", ""},
			"0": minifyHTML(newHTML),
		}, nil
	}

	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		dynamicPart := newHTML[changeStart:changeEnd]
		return TreeNode{
			"s": []string{commonPrefix, commonSuffix},
			"0": minifyHTML(dynamicPart),
		}, nil
	}

	// Default to full dynamic content
	return TreeNode{
		"s": []string{"", ""},
		"0": minifyHTML(newHTML),
	}, nil
}

// createHTMLStructureBasedTree implements deterministic segmentation strategies for HTML content
func (t *Template) createHTMLStructureBasedTree(html string) TreeNode {
	// Define block-level elements that create natural segment boundaries
	blockTags := []string{"<div", "<article", "<section", "<main", "<aside", "<nav", "<ul", "<ol", "<table"}

	// Find the positions of block elements
	var boundaries []int
	for _, tag := range blockTags {
		idx := 0
		for {
			pos := strings.Index(html[idx:], tag)
			if pos == -1 {
				break
			}
			boundaries = append(boundaries, idx+pos)
			idx = idx + pos + len(tag)
		}
	}

	// Sort boundaries
	if len(boundaries) > 0 {
		// Simple sort
		for i := 0; i < len(boundaries)-1; i++ {
			for j := i + 1; j < len(boundaries); j++ {
				if boundaries[i] > boundaries[j] {
					boundaries[i], boundaries[j] = boundaries[j], boundaries[i]
				}
			}
		}

		// Create segments based on boundaries
		const maxSegments = 8
		segmentSize := len(html) / maxSegments

		var statics []string
		var dynamics []interface{}
		lastPos := 0
		dynamicIndex := 0

		for i, boundary := range boundaries {
			// Only create a segment if it's large enough
			if boundary-lastPos > segmentSize || i == len(boundaries)-1 {
				if lastPos == 0 {
					// First segment is typically more static (head, nav, etc)
					statics = append(statics, html[lastPos:boundary])
				} else {
					// Create a dynamic segment
					statics = append(statics, "")
					dynamics = append(dynamics, html[lastPos:boundary])
					dynamicIndex++
				}
				lastPos = boundary
			}
		}

		// Add the final segment
		if lastPos < len(html) {
			statics = append(statics, "")
			dynamics = append(dynamics, html[lastPos:])
		}

		// Build the tree
		tree := TreeNode{"s": statics}
		for i, dyn := range dynamics {
			// Minify HTML content if it's a string containing HTML
			if strDyn, ok := dyn.(string); ok && strings.Contains(strDyn, "<") {
				dyn = minifyHTML(strDyn)
			}
			tree[fmt.Sprintf("%d", i)] = dyn
		}

		// If we got reasonable segmentation, use it
		if len(statics) > 2 && len(dynamics) > 0 {
			return tree
		}
	}

	// Fallback to single segment strategy
	return TreeNode{
		"s": []string{"", ""},
		"0": minifyHTML(html),
	}
}

// Helper functions for string analysis

// findCommonPrefix finds the longest common prefix between two strings
func findCommonPrefix(s1, s2 string) string {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}
	return s1[:minLen]
}

// findCommonSuffix finds the longest common suffix between two strings
func findCommonSuffix(s1, s2 string) string {
	len1, len2 := len(s1), len(s2)
	minLen := len1
	if len2 < minLen {
		minLen = len2
	}

	for i := 0; i < minLen; i++ {
		if s1[len1-1-i] != s2[len2-1-i] {
			return s1[len1-i:]
		}
	}
	return s1[len1-minLen:]
}

// marshalOrderedJSON marshals a TreeNode to JSON with keys in sorted order
func marshalOrderedJSON(tree TreeNode) ([]byte, error) {
	if tree == nil || len(tree) == 0 {
		return []byte("{}"), nil
	}

	var buf bytes.Buffer
	buf.WriteByte('{')

	// Sort keys numerically for proper ordering
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}

	// Custom sort to handle numeric keys properly
	sort.Slice(keys, func(i, j int) bool {
		// Try to parse as numbers first
		num1, err1 := strconv.Atoi(keys[i])
		num2, err2 := strconv.Atoi(keys[j])

		if err1 == nil && err2 == nil {
			// Both are numbers, sort numerically
			return num1 < num2
		}

		// If one or both are not numbers, sort lexicographically
		// But put "s" (static) first
		if keys[i] == "s" {
			return true
		}
		if keys[j] == "s" {
			return false
		}

		return keys[i] < keys[j]
	})

	first := true
	for _, key := range keys {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Write key
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')

		// Write value with no HTML escaping
		valueBytes, err := marshalValue(tree[key])
		if err != nil {
			return nil, err
		}
		buf.Write(valueBytes)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// marshalValue marshals a value to JSON with no HTML escaping
func marshalValue(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(value)
	if err != nil {
		return nil, err
	}

	// Remove trailing newline that Encode adds
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}
