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
	lastFingerprint string        // Fingerprint of the last generated tree for change detection
	keyGen          *KeyGenerator // Per-template key generation for wrapper approach
}

// New creates a new, undefined template with the given name.
// This matches the signature of html/template.New().
func New(name string) *Template {
	return &Template{
		name:   name,
		keyGen: NewKeyGenerator(),
	}
}

// resetKeyGeneration resets the key generator for a fresh render
func (t *Template) resetKeyGeneration() {
	if t.keyGen == nil {
		t.keyGen = NewKeyGenerator()
	} else {
		t.keyGen.Reset()
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
	// Initialize key generator if needed (but don't reset - keys should increment globally)
	if t.keyGen == nil {
		t.keyGen = NewKeyGenerator()
	}

	// Load existing key mappings from previous render if available
	if t.lastTree != nil {
		t.loadExistingKeyMappings(t.lastTree)
	}

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
	tree, err := parseTemplateToTree(templateContent, data, t.keyGen)
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

// compareTreesAndGetChanges compares two trees and returns only changed dynamics
func (t *Template) compareTreesAndGetChanges(oldTree, newTree TreeNode) TreeNode {
	changes := make(TreeNode)

	// First, find range constructs in both trees and match them by content signature
	rangeMatches := findRangeConstructMatches(oldTree, newTree)

	// Compare dynamic segments (skip statics "s" and fingerprint "f")
	for k, newValue := range newTree {
		if k == "s" || k == "f" {
			continue // Skip static segments and fingerprint
		}

		oldValue, exists := oldTree[k]
		if !exists {
			changes[k] = newValue
		} else if !deepEqual(oldValue, newValue) {
			// Check if this field has a range construct match
			if matchedOldField, isRangeMatch := rangeMatches[k]; isRangeMatch {
				// Get the corresponding old range construct
				oldRangeValue := oldTree[matchedOldField]
				// Generate differential operations for matched range constructs
				diffOps := generateRangeDifferentialOperations(oldRangeValue, newValue)
				if len(diffOps) > 0 {
					changes[k] = diffOps
				} else {
					// Fall back to full update if no differential operations
					changes[k] = newValue
				}
			} else {
				// Regular change detection for non-range values
				changes[k] = newValue
			}
		}
	}
	return changes
}

// findRangeConstructMatches finds range constructs in both trees and matches them by content signature
// Returns a map of newField -> oldField for range constructs that represent the same template construct
func findRangeConstructMatches(oldTree, newTree TreeNode) map[string]string {
	matches := make(map[string]string)

	// Find all range constructs in both trees
	oldRanges := findRangeConstructs(oldTree)
	newRanges := findRangeConstructs(newTree)

	// Match range constructs by their static template signature
	for newField, newRange := range newRanges {
		newSignature := getRangeSignature(newRange)

		for oldField, oldRange := range oldRanges {
			oldSignature := getRangeSignature(oldRange)

			// If signatures match, this is the same template construct
			if newSignature == oldSignature {
				matches[newField] = oldField
				break // Each new range should match at most one old range
			}
		}
	}

	return matches
}

// findRangeConstructs finds all range constructs in a tree
func findRangeConstructs(tree TreeNode) map[string]interface{} {
	ranges := make(map[string]interface{})

	for field, value := range tree {
		if field == "s" || field == "f" {
			continue // Skip static segments and fingerprint
		}

		if isRangeConstruct(value) {
			ranges[field] = value
		}
	}

	return ranges
}

// getRangeSignature creates a signature for a range construct based on its static template structure
// This signature should be the same for the same template construct regardless of data
func getRangeSignature(rangeValue interface{}) string {
	rangeMap, ok := rangeValue.(map[string]interface{})
	if !ok {
		return ""
	}

	// Use the static parts ("s") as the signature since they represent the template structure
	staticParts, exists := rangeMap["s"]
	if !exists {
		return ""
	}

	// Convert static parts to a string signature
	return fmt.Sprintf("%v", staticParts)
}

// deepEqual compares two values deeply
func deepEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// isRangeConstruct checks if a value is a range construct (has "d" and "s" keys)
func isRangeConstruct(value interface{}) bool {
	if valueMap, ok := value.(map[string]interface{}); ok {
		_, hasD := valueMap["d"]
		_, hasS := valueMap["s"]
		return hasD && hasS
	}
	return false
}

// extractItemKeys extracts the keys (field "0") from a slice of range items
func extractItemKeys(items []interface{}) []string {
	var keys []string
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					keys = append(keys, keyStr)
				}
			}
		}
	}
	return keys
}

// isPureReordering checks if the items are the same but just in different order
func isPureReordering(oldItems, newItems []interface{}, oldKeys, newKeys []string) bool {
	// Must have same number of items
	if len(oldKeys) != len(newKeys) {
		return false
	}

	// Check if keys are the same (just different order)
	oldKeySet := make(map[string]bool)
	newKeySet := make(map[string]bool)

	for _, k := range oldKeys {
		oldKeySet[k] = true
	}
	for _, k := range newKeys {
		newKeySet[k] = true
	}

	// If key sets don't match, it's not pure reordering
	if len(oldKeySet) != len(newKeySet) {
		return false
	}
	for k := range oldKeySet {
		if !newKeySet[k] {
			return false
		}
	}

	// Now check if the items with same keys have identical content
	oldItemsByKey := make(map[string]interface{})
	newItemsByKey := make(map[string]interface{})

	for _, item := range oldItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					oldItemsByKey[keyStr] = item
				}
			}
		}
	}

	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					newItemsByKey[keyStr] = item
				}
			}
		}
	}

	// Compare each item's content (excluding position-dependent fields)
	for key, oldItem := range oldItemsByKey {
		newItem, exists := newItemsByKey[key]
		if !exists {
			return false
		}

		// Compare items excluding position field (field "3" contains "#0:", "#1:", etc.)
		oldItemMap, ok1 := oldItem.(map[string]interface{})
		newItemMap, ok2 := newItem.(map[string]interface{})

		if !ok1 || !ok2 {
			// If we can't compare as maps, fall back to full comparison
			if !deepEqual(oldItem, newItem) {
				return false
			}
			continue
		}

		// Compare all fields except position field
		for field, oldValue := range oldItemMap {
			// Skip position field (field "3" contains positional display like "#0:")
			if field == "3" {
				continue
			}

			newValue, exists := newItemMap[field]
			if !exists || !deepEqual(oldValue, newValue) {
				return false
			}
		}

		// Also check that new item doesn't have extra fields (except position)
		for field := range newItemMap {
			if field == "3" {
				continue
			}
			if _, exists := oldItemMap[field]; !exists {
				return false
			}
		}
	}

	// Check if order actually changed
	for i := range oldKeys {
		if oldKeys[i] != newKeys[i] {
			return true // Same items, different order = pure reordering
		}
	}

	// Same items, same order = no change
	return false
}

// generateRangeDifferentialOperations generates differential operations for range constructs
func generateRangeDifferentialOperations(oldValue, newValue interface{}) []interface{} {
	var operations []interface{}

	oldRange, ok1 := oldValue.(map[string]interface{})
	newRange, ok2 := newValue.(map[string]interface{})

	if !ok1 || !ok2 {
		// Type conversion failed
		return operations
	}

	// Extract old and new item arrays
	oldItems, ok1 := oldRange["d"].([]interface{})
	newItems, ok2 := newRange["d"].([]interface{})

	// Try alternative type assertion if the first one fails
	if !ok1 {
		if oldMaps, ok := oldRange["d"].([]map[string]interface{}); ok {
			oldItems = make([]interface{}, len(oldMaps))
			for i, m := range oldMaps {
				oldItems[i] = m
			}
			ok1 = true
		}
	}

	if !ok2 {
		if newMaps, ok := newRange["d"].([]map[string]interface{}); ok {
			newItems = make([]interface{}, len(newMaps))
			for i, m := range newMaps {
				newItems[i] = m
			}
			ok2 = true
		}
	}

	if !ok1 || !ok2 {
		// Item extraction failed
		// Debug: could examine keys and types here if needed
		return operations
	}

	// Comparing old items vs new items

	// First, check if this is a pure reordering (same items, different order)
	oldKeys := extractItemKeys(oldItems)
	newKeys := extractItemKeys(newItems)

	if isPureReordering(oldItems, newItems, oldKeys, newKeys) {
		// Generate ordering operation
		return []interface{}{[]interface{}{"o", newKeys}}
	}

	// Create maps for easy lookup by keys
	oldItemsByKey := make(map[string]interface{})
	newItemsByKey := make(map[string]interface{})

	// Map old items by their auto-generated keys (field "0")
	for _, item := range oldItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					oldItemsByKey[keyStr] = item
				}
			}
		}
	}

	// Map new items by their auto-generated keys (field "0")
	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					newItemsByKey[keyStr] = item
				}
			}
		}
	}

	// Find removed items (in old but not in new)
	// Sort keys to ensure deterministic order
	sortedOldKeys := make([]string, 0, len(oldItemsByKey))
	for key := range oldItemsByKey {
		sortedOldKeys = append(sortedOldKeys, key)
	}
	sort.Strings(sortedOldKeys)

	for _, key := range sortedOldKeys {
		if _, exists := newItemsByKey[key]; !exists {
			operations = append(operations, []interface{}{"r", key})
		}
	}

	// Find updated items (in both, but changed)
	// Sort keys to ensure deterministic order
	sortedNewKeys := make([]string, 0, len(newItemsByKey))
	for key := range newItemsByKey {
		sortedNewKeys = append(sortedNewKeys, key)
	}
	sort.Strings(sortedNewKeys)

	for _, key := range sortedNewKeys {
		newItem := newItemsByKey[key]
		if oldItem, exists := oldItemsByKey[key]; exists {
			// Compare items and generate update operation if different
			changes := compareRangeItemsForChanges(oldItem, newItem)
			if len(changes) > 0 {
				operations = append(operations, []interface{}{"u", key, changes})
			}
		}
	}

	// Smart insertion pattern detection for added items
	addedKeys := findNewItems(oldItems, newItems)
	if len(addedKeys) > 0 {
		// Check if it's a complex pattern that should fall back to full state
		if isComplexInsertionPattern(addedKeys, oldItems, newItems) {
			// Fall back to full state replacement - return empty operations to trigger fallback
			return operations
		}

		// Check if all items are appended at the end (bulk append)
		if areAllItemsAtEnd(addedKeys, oldItems, newItems) {
			// Create array of new items in order
			var newItemsToAdd []interface{}
			for _, key := range addedKeys {
				if item, exists := newItemsByKey[key]; exists {
					newItemsToAdd = append(newItemsToAdd, item)
				}
			}
			operations = append(operations, []interface{}{"a", newItemsToAdd})
		} else {
			// Check if all items are at the same position (single-point insertion)
			if isSamePosition, targetKey, position := areAllItemsAtSamePosition(addedKeys, oldItems, newItems); isSamePosition {
				var newItemsToInsert []interface{}
				for _, key := range addedKeys {
					if item, exists := newItemsByKey[key]; exists {
						newItemsToInsert = append(newItemsToInsert, item)
					}
				}
				if targetKey == "" {
					operations = append(operations, []interface{}{"i", nil, position, newItemsToInsert})
				} else {
					operations = append(operations, []interface{}{"i", targetKey, position, newItemsToInsert})
				}
			} else {
				// Multiple individual insertions at different positions
				for _, key := range addedKeys {
					if newItem, exists := newItemsByKey[key]; exists {
						// Find position for this specific item
						for i, item := range newItems {
							if itemMap, ok := item.(map[string]interface{}); ok {
								if itemKey, exists := itemMap["0"]; exists {
									if keyStr, ok := itemKey.(string); ok && keyStr == key {
										// Determine insertion position
										if i == 0 {
											operations = append(operations, []interface{}{"i", nil, "start", newItem})
										} else if i == len(newItems)-1 {
											operations = append(operations, []interface{}{"a", newItem})
										} else {
											// Find the item before this one
											if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
												if prevKey, exists := prevItem["0"]; exists {
													if prevKeyStr, ok := prevKey.(string); ok {
														operations = append(operations, []interface{}{"i", prevKeyStr, "after", newItem})
													}
												}
											}
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return operations
}

// compareRangeItemsForChanges compares two range items and returns a map of field changes
func compareRangeItemsForChanges(oldItem, newItem interface{}) map[string]interface{} {
	changes := make(map[string]interface{})

	oldItemMap, ok1 := oldItem.(map[string]interface{})
	newItemMap, ok2 := newItem.(map[string]interface{})

	if !ok1 || !ok2 {
		return changes
	}

	// Compare each field (except the key field "0")
	for fieldKey, newValue := range newItemMap {
		if fieldKey == "0" {
			continue // Skip the auto-generated key field
		}

		oldValue, exists := oldItemMap[fieldKey]
		if !exists || !deepEqual(oldValue, newValue) {
			changes[fieldKey] = newValue
		}
	}

	return changes
}

// Smart pattern detection functions for enhanced insertion operations

// findNewItems returns keys of items that exist in new but not in old
func findNewItems(oldItems, newItems []interface{}) []string {
	oldKeys := make(map[string]bool)
	for _, item := range oldItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					oldKeys[keyStr] = true
				}
			}
		}
	}

	var newKeys []string
	for _, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					if !oldKeys[keyStr] {
						newKeys = append(newKeys, keyStr)
					}
				}
			}
		}
	}

	return newKeys
}

// areAllItemsAtEnd checks if all new items are appended at the end
func areAllItemsAtEnd(newKeys []string, oldItems, newItems []interface{}) bool {
	if len(newKeys) == 0 {
		return false
	}

	oldCount := len(oldItems)
	newCount := len(newItems)

	// Check if new items are exactly at the end positions
	for i, key := range newKeys {
		expectedIndex := oldCount + i
		if expectedIndex >= newCount {
			return false
		}

		// Get the item at this position in newItems
		if itemMap, ok := newItems[expectedIndex].(map[string]interface{}); ok {
			if itemKey, exists := itemMap["0"]; exists {
				if keyStr, ok := itemKey.(string); ok {
					if keyStr != key {
						return false
					}
				} else {
					return false
				}
			} else {
				return false
			}
		} else {
			return false
		}
	}

	return true
}

// areAllItemsAtSamePosition checks if all new items are inserted at the same position
func areAllItemsAtSamePosition(newKeys []string, oldItems, newItems []interface{}) (bool, string, string) {
	if len(newKeys) <= 1 {
		return false, "", "" // Single items don't need this optimization
	}

	// Find the first new item's position
	var firstNewIndex = -1
	var targetKey = ""
	var position = ""

	for i, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					// Check if this is a new key
					for _, newKey := range newKeys {
						if newKey == keyStr {
							if firstNewIndex == -1 {
								firstNewIndex = i
								// Determine the target and position
								if i > 0 {
									// Check the item before
									if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
										if prevKey, exists := prevItem["0"]; exists {
											if prevKeyStr, ok := prevKey.(string); ok {
												targetKey = prevKeyStr
												position = "after"
											}
										}
									}
								} else {
									// At the beginning
									targetKey = ""
									position = "start"
								}
							}
							break
						}
					}
				}
			}
		}
	}

	if firstNewIndex == -1 {
		return false, "", ""
	}

	// Verify all new items are contiguous starting from firstNewIndex
	for i, newKey := range newKeys {
		expectedIndex := firstNewIndex + i
		if expectedIndex >= len(newItems) {
			return false, "", ""
		}

		if itemMap, ok := newItems[expectedIndex].(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					if keyStr != newKey {
						return false, "", ""
					}
				} else {
					return false, "", ""
				}
			} else {
				return false, "", ""
			}
		} else {
			return false, "", ""
		}
	}

	return true, targetKey, position
}

// isComplexInsertionPattern checks if the insertion pattern is too complex for simple operations
func isComplexInsertionPattern(newKeys []string, oldItems, newItems []interface{}) bool {
	// Consider it complex if there are more than 3 separate insertion points
	const maxInsertionPoints = 3

	if len(newKeys) == 0 {
		return false
	}

	insertionPoints := make(map[string]bool)

	for i, item := range newItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if key, exists := itemMap["0"]; exists {
				if keyStr, ok := key.(string); ok {
					// Check if this is a new key
					for _, newKey := range newKeys {
						if newKey == keyStr {
							// Determine insertion point
							var insertionPoint string
							if i > 0 {
								if prevItem, ok := newItems[i-1].(map[string]interface{}); ok {
									if prevKey, exists := prevItem["0"]; exists {
										if prevKeyStr, ok := prevKey.(string); ok {
											insertionPoint = prevKeyStr + ":after"
										}
									}
								}
							} else {
								insertionPoint = "start"
							}
							insertionPoints[insertionPoint] = true
							break
						}
					}
				}
			}
		}
	}

	return len(insertionPoints) > maxInsertionPoints
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

// loadExistingKeyMappings loads existing key mappings from the last tree node
func (t *Template) loadExistingKeyMappings(lastTree TreeNode) {
	// Look for range data in the tree and load existing key mappings
	for _, value := range lastTree {
		if rangeData, ok := value.(map[string]interface{}); ok {
			// Check if this looks like range data with "d" field
			if dynData, exists := rangeData["d"]; exists {
				if dynSlice, ok := dynData.([]interface{}); ok {
					t.keyGen.LoadExistingKeys(dynSlice)
				}
			}
		}
	}
}
