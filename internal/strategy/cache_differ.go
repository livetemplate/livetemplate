package strategy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
)

// differ provides the main API for template processing with intelligent caching
// It starts with diffing approach and creates appropriate tree structures
type differ struct {
	templateStr    string
	tmpl           *template.Template
	lastData       interface{}
	lastHTML       string
	initialTree    treeNode  // Cache the initial tree structure
	hasInitialTree bool      // Track if we've established the structure
}

// newInternalDiffer creates a new cache-aware differ
func newInternalDiffer(templateStr string) (*differ, error) {
	tmpl, err := template.New("smart").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}
	
	return &differ{
		templateStr: templateStr,
		tmpl:        tmpl,
	}, nil
}

// GenerateTree generates tree structure with intelligent caching behavior:
// 1. First render: returns statics and dynamics for efficient initial rendering
// 2. Further renders: returns dynamics only unless statics need to change
func (s *differ) GenerateTree(data interface{}) ([]byte, error) {
	tree, err := s.generateTreeInternal(data)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(tree)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

// generateTreeInternal is the internal implementation that returns treeNode
func (s *differ) generateTreeInternal(data interface{}) (treeNode, error) {
	// Execute template with current data
	currentHTML, err := s.executeTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}
	
	// First render - no previous state
	if s.lastData == nil {
		s.lastData = data
		s.lastHTML = currentHTML
		return s.generateInitialTree(currentHTML, data)
	}
	
	// Subsequent renders - use diffing approach
	return s.generateDiffBasedTree(s.lastHTML, currentHTML, s.lastData, data)
}

// executeTemplate executes the template with given data
func (s *differ) executeTemplate(data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := s.tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// generateInitialTree creates tree with statics and dynamics for first render
func (s *differ) generateInitialTree(html string, data interface{}) (treeNode, error) {
	// For first render, try to create fine-grained tree for bandwidth efficiency
	tree, err := parseWithOriginalApproach(s.templateStr, data)
	if err != nil {
		// Fallback to simple tree if fine-grained approach fails
		tree = treeNode{
			"s": []string{"", ""},
			"0": html,
		}
	}
	
	// Cache the initial structure for future dynamics-only updates
	s.initialTree = tree
	s.hasInitialTree = true
	
	return tree, nil
}

// generateDiffBasedTree creates tree based on diff analysis
func (s *differ) generateDiffBasedTree(oldHTML, newHTML string, oldData, newData interface{}) (treeNode, error) {
	// Update cached state
	s.lastData = newData
	s.lastHTML = newHTML
	
	// No changes - return empty tree to indicate no update needed
	if oldHTML == newHTML {
		return treeNode{}, nil
	}
	
	// Try to generate dynamics-only tree using cached structure
	if s.hasInitialTree {
		return s.generateDynamicsOnlyTree(newData)
	}
	
	// Fallback to analyzing the change (shouldn't happen after first render)
	return s.analyzeChangeAndCreateTree(oldHTML, newHTML, oldData, newData)
}

// analyzeChangeAndCreateTree determines the best tree structure based on the type of change
func (s *differ) analyzeChangeAndCreateTree(oldHTML, newHTML string, oldData, newData interface{}) (treeNode, error) {
	// Find common prefix and suffix to understand change patterns
	commonPrefix := findCommonPrefix(oldHTML, newHTML)
	commonSuffix := findCommonSuffix(oldHTML, newHTML)
	
	// Calculate change boundaries
	changeStart := len(commonPrefix)
	changeEnd := len(newHTML) - len(commonSuffix)
	
	// If entire content changed, return full dynamic content
	if changeStart >= changeEnd || (changeStart == 0 && changeEnd == len(newHTML)) {
		return treeNode{
			"s": []string{"", ""},
			"0": newHTML,
		}, nil
	}
	
	// If we have stable prefix/suffix, create tree with static parts
	if commonPrefix != "" || commonSuffix != "" {
		dynamicPart := newHTML[changeStart:changeEnd]
		return treeNode{
			"s": []string{commonPrefix, commonSuffix},
			"0": dynamicPart,
		}, nil
	}
	
	// Default to full dynamic content
	return treeNode{
		"s": []string{"", ""},
		"0": newHTML,
	}, nil
}

// GetCurrentHTML returns the last rendered HTML for debugging/inspection
func (s *differ) GetCurrentHTML() string {
	return s.lastHTML
}

// ReconstructFromDynamics merges dynamics-only tree with cached static structure
// This simulates what a client would do when receiving dynamics-only updates
func (s *differ) ReconstructFromDynamics(dynamicsTree treeNode) string {
	if !s.hasInitialTree {
		// No cached structure, treat as complete tree
		return reconstructHTML(dynamicsTree)
	}
	
	// If dynamics tree has static structure, use it directly
	if _, hasStatics := dynamicsTree["s"]; hasStatics {
		return reconstructHTML(dynamicsTree)
	}
	
	// Merge dynamics with cached static structure
	mergedTree := s.mergeWithCachedStructure(dynamicsTree)
	return reconstructHTML(mergedTree)
}

// mergeWithCachedStructure combines dynamics-only tree with cached static structure
func (s *differ) mergeWithCachedStructure(dynamicsTree treeNode) treeNode {
	if s.initialTree == nil {
		return dynamicsTree
	}
	
	// Create a copy of the initial tree structure
	merged := make(treeNode)
	
	// Copy static structure from initial tree
	if statics, ok := s.initialTree["s"]; ok {
		merged["s"] = statics
	}
	
	// Add dynamics from new tree
	for key, value := range dynamicsTree {
		if key != "s" { // Don't override static structure
			merged[key] = value
		}
	}
	
	return merged
}

// generateDynamicsOnlyTree creates a tree with only dynamic values, using cached structure
func (s *differ) generateDynamicsOnlyTree(data interface{}) (treeNode, error) {
	// Try to use the fine-grained approach to extract only dynamic values
	newTree, err := parseWithOriginalApproach(s.templateStr, data)
	if err != nil {
		// If fine-grained fails, fall back to template execution
		html, execErr := s.executeTemplate(data)
		if execErr != nil {
			return nil, execErr
		}
		// Return only the dynamic content, no static structure
		return treeNode{"0": html}, nil
	}
	
	// Extract only the dynamic parts from the new tree, matching initial structure
	return s.extractDynamicsOnly(newTree), nil
}


// extractDynamicsOnly removes static parts and returns only dynamic values
func (s *differ) extractDynamicsOnly(tree treeNode) treeNode {
	dynamicsOnly := make(treeNode)
	
	// Copy all dynamic keys (numeric keys), skip static structure ("s")
	for key, value := range tree {
		if key != "s" {
			dynamicsOnly[key] = value
		}
	}
	
	return dynamicsOnly
}

// Reset clears the cache, causing next GenerateTree to behave like first render
func (s *differ) Reset() {
	s.lastData = nil
	s.lastHTML = ""
	s.initialTree = nil
	s.hasInitialTree = false
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