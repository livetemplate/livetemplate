package livetemplate

import (
	"fmt"
	"log"
	"strings"
)

// TreeUpdateAnalyzer analyzes tree updates and warns about inefficiencies
type TreeUpdateAnalyzer struct {
	// MinStaticSize is the minimum size of an HTML chunk to consider it "large"
	MinStaticSize int
	// Enabled controls whether analysis warnings are logged
	Enabled bool
}

// NewTreeUpdateAnalyzer creates a new analyzer with default settings
func NewTreeUpdateAnalyzer() *TreeUpdateAnalyzer {
	return &TreeUpdateAnalyzer{
		MinStaticSize: 100, // Warn if HTML chunks are > 100 chars
		Enabled:       true,
	}
}

// AnalyzeUpdate analyzes a tree update and logs warnings about inefficiencies
// Output is optimized for LLM consumption to provide context-rich recommendations
func (a *TreeUpdateAnalyzer) AnalyzeUpdate(tree treeNode, templateName string, templateSource string) {
	if !a.Enabled {
		return
	}

	issues := a.findDetailedIssues(tree, "", templateSource)
	if len(issues) > 0 {
		log.Println("=== LIVETEMPLATE TREE ANALYZER ===")
		log.Printf("Template: %s\n", templateName)
		log.Println("ISSUE: Inefficient tree structure detected")
		log.Println("\nPROBLEM:")
		log.Println("Large HTML chunks are being sent as dynamic values instead of being cached as static structure.")
		log.Println("This defeats LiveTemplate's optimization - the client must re-parse HTML on every update.")
		log.Println("\nDETAILS:")
		for _, issue := range issues {
			log.Println(issue)
		}
		log.Println("\nCONTEXT:")
		log.Println("LiveTemplate tree format:")
		log.Println(`  {\"s\": [\"<div>\", \"</div>\"], \"0\": \"value\"}  <- GOOD: Statics cached, only value updates`)
		log.Println(`  {\"0\": \"<div>value</div>\"}                     <- BAD: Entire HTML sent every update`)
		log.Println("\nRECOMMENDATION:")
		log.Println("Restructure template to separate static HTML structure from dynamic values.")
		log.Println("Use conditionals ({{if}}) or ranges ({{range}}) to create tree nodes with static separators.")
		log.Println("\nTO FIX:")
		log.Println("Provide the template source to an LLM with this analysis for specific restructuring suggestions.")
		log.Println("=== END ANALYZER OUTPUT ===")
	}
}

// TreeIssue describes a tree efficiency issue
type TreeIssue struct {
	Path        string
	IssueType   string
	Description string
	Size        int
}

// findDetailedIssues recursively finds efficiency issues with detailed context for LLMs
func (a *TreeUpdateAnalyzer) findDetailedIssues(tree treeNode, path string, templateSource string) []string {
	var issues []string

	// Check if this is a well-formed tree node
	hasStatics := false
	hasDynamics := false
	dynamicCount := 0

	for key := range tree {
		if key == "s" {
			hasStatics = true
		} else if key == "f" || key == "d" {
			continue
		} else {
			hasDynamics = true
			dynamicCount++
		}
	}

	// If we have dynamics but no statics, this could be a problem
	if hasDynamics && !hasStatics && dynamicCount > 0 {
		// Check each dynamic value
		for key, value := range tree {
			if key == "s" || key == "f" || key == "d" {
				continue
			}

			valuePath := path + "." + key
			if path == "" {
				valuePath = key
			}

			// Check if value is a string (HTML chunk)
			if htmlStr, ok := value.(string); ok {
				size := len(htmlStr)
				if size > a.MinStaticSize {
					// Check if it contains HTML tags (static structure)
					tagCount := strings.Count(htmlStr, "<")
					if tagCount > 2 {
						// Truncate HTML for display
						preview := htmlStr
						if len(preview) > 200 {
							preview = preview[:200] + "..."
						}
						// Escape for readability
						preview = strings.ReplaceAll(preview, "\n", "\\n")

						issue := fmt.Sprintf(
							"Field '%s': %d chars, %d HTML tags\n"+
								"  Generated tree: {\"%s\": \"%s\"}\n"+
								"  Problem: This HTML structure should be static (cached), not dynamic\n"+
								"  Impact: Client must parse %d chars of HTML on every update",
							valuePath, size, tagCount, key, preview, size,
						)
						issues = append(issues, issue)
					}
				}
			} else if nestedTree, ok := value.(map[string]interface{}); ok {
				// Recursively check nested trees
				nestedIssues := a.findDetailedIssues(nestedTree, valuePath, templateSource)
				issues = append(issues, nestedIssues...)
			}
		}
	}

	// Check for range constructs - detect full array sends vs incremental operations
	if rangeData, hasRange := tree["d"]; hasRange {
		if rangeSlice, ok := rangeData.([]interface{}); ok {
			// Count how many items are full tree nodes vs operations
			fullNodeCount := 0
			operationCount := 0

			for i, item := range rangeSlice {
				// Check if this is an operation array like ["i", key, data] or ["u", key, data]
				if opSlice, ok := item.([]interface{}); ok && len(opSlice) > 0 {
					if opType, ok := opSlice[0].(string); ok {
						if opType == "i" || opType == "u" || opType == "r" || opType == "o" {
							operationCount++
							continue
						}
					}
				}

				// Check if this is a full tree node (map)
				if itemMap, ok := item.(map[string]interface{}); ok {
					fullNodeCount++

					// Recursively check for issues in the item
					itemPath := fmt.Sprintf("%s.d[%d]", path, i)
					if path == "" {
						itemPath = fmt.Sprintf("d[%d]", i)
					}
					itemIssues := a.findDetailedIssues(itemMap, itemPath, templateSource)
					issues = append(issues, itemIssues...)
				}
			}

			// EFFICIENCY CHECK: If we have 2+ full nodes and 0 operations, this is likely a full array send
			// This defeats the incremental update optimization - should use insert/update/delete operations
			if fullNodeCount >= 2 && operationCount == 0 {
				rangePath := path
				if rangePath == "" {
					rangePath = "root"
				}
				issue := fmt.Sprintf(
					"Range at '%s': Sending full array (%d items) instead of incremental operations\n"+
						"  Generated tree: {\"d\": [%d full tree nodes]}\n"+
						"  Problem: Client must process all %d items, even if most are unchanged\n"+
						"  Impact: Defeats LiveTemplate's incremental update optimization\n"+
						"  Expected: Use insert/update/delete operations for changed items only:\n"+
						"    [\"i\", afterKey, newItem]  - Insert new item\n"+
						"    [\"u\", itemKey, updates]   - Update existing item\n"+
						"    [\"r\", itemKey]             - Remove item\n"+
						"  This typically happens when:\n"+
						"    1. Range is inside a conditional that switches branches\n"+
						"    2. Structure change detection doesn't recognize the range\n"+
						"  Fix: Ensure containsRangeConstruct() is used in structure comparison",
					rangePath, fullNodeCount, fullNodeCount, fullNodeCount,
				)
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

// findIssues recursively finds efficiency issues in a tree (simple version for tests)
func (a *TreeUpdateAnalyzer) findIssues(tree treeNode, path string) []string {
	var issues []string

	// Check if this is a well-formed tree node
	hasStatics := false
	hasDynamics := false
	dynamicCount := 0

	for key := range tree {
		if key == "s" {
			hasStatics = true
		} else if key == "f" {
			// Fingerprint is fine
			continue
		} else if key == "d" {
			// Range construct
			continue
		} else {
			hasDynamics = true
			dynamicCount++
		}
	}

	// If we have dynamics but no statics, this could be a problem
	if hasDynamics && !hasStatics && dynamicCount > 0 {
		// Check each dynamic value
		for key, value := range tree {
			if key == "s" || key == "f" || key == "d" {
				continue
			}

			valuePath := path + "." + key
			if path == "" {
				valuePath = key
			}

			// Check if value is a string (HTML chunk)
			if htmlStr, ok := value.(string); ok {
				size := len(htmlStr)
				if size > a.MinStaticSize {
					// Check if it contains HTML tags (static structure)
					tagCount := strings.Count(htmlStr, "<")
					if tagCount > 2 {
						issues = append(issues, fmt.Sprintf(
							"Field '%s': Large HTML chunk (%d chars, %d tags) without static/dynamic separation",
							valuePath, size, tagCount,
						))
					}
				}
			} else if nestedTree, ok := value.(map[string]interface{}); ok {
				// Recursively check nested trees
				nestedIssues := a.findIssues(nestedTree, valuePath)
				issues = append(issues, nestedIssues...)
			}
		}
	}

	// Check for range constructs without proper structure
	if rangeData, hasRange := tree["d"]; hasRange {
		if rangeSlice, ok := rangeData.([]interface{}); ok {
			for i, item := range rangeSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemPath := fmt.Sprintf("%s.d[%d]", path, i)
					if path == "" {
						itemPath = fmt.Sprintf("d[%d]", i)
					}
					itemIssues := a.findIssues(itemMap, itemPath)
					issues = append(issues, itemIssues...)
				}
			}
		}
	}

	// Check statics array size
	if statics, ok := tree["s"]; ok {
		if staticSlice, ok := statics.([]string); ok {
			totalStaticSize := 0
			for _, s := range staticSlice {
				totalStaticSize += len(s)
			}
			// This is actually good - large statics are cacheable
			// Don't warn about this
		}
	}

	return issues
}

// AnalyzeTemplateStructure provides suggestions for template optimization
func (a *TreeUpdateAnalyzer) AnalyzeTemplateStructure(templateStr string) []string {
	var suggestions []string

	// Check for conditionals in <head> or <style>
	if strings.Contains(templateStr, "<style>") && strings.Contains(templateStr, "{{if") {
		lines := strings.Split(templateStr, "\n")
		inStyle := false
		for i, line := range lines {
			if strings.Contains(line, "<style>") {
				inStyle = true
			}
			if inStyle && strings.Contains(line, "{{if") {
				suggestions = append(suggestions, fmt.Sprintf(
					"Line %d: Conditional logic in <style> tag won't update via WebSocket (outside wrapper)",
					i+1,
				))
			}
			if strings.Contains(line, "</style>") {
				inStyle = false
			}
		}
	}

	// Check for deeply nested conditionals
	nestedIfs := 0
	for _, line := range strings.Split(templateStr, "\n") {
		if strings.Contains(line, "{{if") {
			nestedIfs++
		}
		if strings.Contains(line, "{{end}}") {
			nestedIfs--
		}
		if nestedIfs > 3 {
			suggestions = append(suggestions, fmt.Sprintf(
				"Deep nesting detected (level %d) - consider extracting sub-templates",
				nestedIfs,
			))
			break
		}
	}

	return suggestions
}
