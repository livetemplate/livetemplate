package livetemplate

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

// EnhancedTreeAnalyzer extends TreeUpdateAnalyzer with specification compliance checking
type EnhancedTreeAnalyzer struct {
	*TreeUpdateAnalyzer

	// Specification compliance tracking
	ComplianceEnabled bool
	FirstRenderSeen   bool
	SentStatics       map[string]bool
	LastTree          treeNode
	UpdateCount       int

	// Metrics tracking
	MetricsEnabled      bool
	TotalUpdates        int
	TotalBytesOriginal  int64
	TotalBytesOptimized int64
	UpdateTimes         []time.Duration
	ViolationCount      int

	// Performance profiling
	ProfilingEnabled bool
	TreeGenTime      time.Duration
	DiffTime         time.Duration
	SerializeTime    time.Duration
}

// NewEnhancedTreeAnalyzer creates an enhanced analyzer with all features
func NewEnhancedTreeAnalyzer() *EnhancedTreeAnalyzer {
	return &EnhancedTreeAnalyzer{
		TreeUpdateAnalyzer: &TreeUpdateAnalyzer{
			MinStaticSize: 100,
			Enabled:       true,
		},
		ComplianceEnabled: true,
		MetricsEnabled:    true,
		ProfilingEnabled:  true,
		SentStatics:       make(map[string]bool),
		UpdateTimes:       make([]time.Duration, 0),
	}
}

// SpecificationCompliance represents compliance check results
type SpecificationCompliance struct {
	Compliant          bool
	FirstRenderValid   bool
	UpdatesMinimal     bool
	RangesGranular     bool
	StaticsNotRepeated bool
	Violations         []string
}

// UpdateMetrics represents metrics for a single update
type UpdateMetrics struct {
	UpdateNumber      int
	OriginalSize      int
	OptimizedSize     int
	CompressionRatio  float64
	StaticsReused     int
	DynamicsChanged   int
	RangeOperations   int
	ProcessingTime    time.Duration
}

// AnalyzeWithCompliance performs full analysis including specification compliance
func (a *EnhancedTreeAnalyzer) AnalyzeWithCompliance(tree treeNode, templateName string, templateSource string, isFirstRender bool) (*SpecificationCompliance, *UpdateMetrics) {
	startTime := time.Now()
	a.UpdateCount++

	// Initialize compliance result
	compliance := &SpecificationCompliance{
		Compliant:  true,
		Violations: make([]string, 0),
	}

	// Initialize metrics
	metrics := &UpdateMetrics{
		UpdateNumber: a.UpdateCount,
	}

	// Specification compliance checks
	if a.ComplianceEnabled {
		if isFirstRender {
			compliance.FirstRenderValid = a.validateFirstRender(tree, compliance)
			a.FirstRenderSeen = true
			a.markStaticsSent(tree, "")
		} else {
			if !a.FirstRenderSeen {
				compliance.Compliant = false
				compliance.Violations = append(compliance.Violations, "Update received before first render")
			} else {
				compliance.UpdatesMinimal = a.validateMinimalUpdate(tree, a.LastTree, compliance)
				compliance.RangesGranular = a.validateRangeGranularity(tree, compliance)
				compliance.StaticsNotRepeated = a.validateNoRedundantStatics(tree, compliance)
			}
		}
	}

	// Calculate metrics
	if a.MetricsEnabled {
		metrics = a.calculateMetrics(tree, a.LastTree, isFirstRender)
		a.TotalUpdates++
		a.TotalBytesOriginal += int64(metrics.OriginalSize)
		a.TotalBytesOptimized += int64(metrics.OptimizedSize)
	}

	// Performance profiling
	if a.ProfilingEnabled {
		metrics.ProcessingTime = time.Since(startTime)
		a.UpdateTimes = append(a.UpdateTimes, metrics.ProcessingTime)
	}

	// Run original analyzer for efficiency issues
	if a.Enabled {
		a.AnalyzeUpdate(tree, templateName, templateSource)
	}

	// Update state for next analysis
	a.LastTree = tree

	// Set overall compliance
	compliance.Compliant = compliance.FirstRenderValid &&
		compliance.UpdatesMinimal &&
		compliance.RangesGranular &&
		compliance.StaticsNotRepeated &&
		len(compliance.Violations) == 0

	if !compliance.Compliant {
		a.ViolationCount++
	}

	return compliance, metrics
}

// validateFirstRender checks if first render follows specification
func (a *EnhancedTreeAnalyzer) validateFirstRender(tree treeNode, compliance *SpecificationCompliance) bool {
	// Must have statics
	statics, hasStatics := tree["s"].([]string)
	if !hasStatics {
		compliance.Violations = append(compliance.Violations,
			"SPEC VIOLATION: First render missing 's' (statics) key")
		return false
	}

	if len(statics) == 0 {
		compliance.Violations = append(compliance.Violations,
			"SPEC VIOLATION: First render has empty statics array")
		return false
	}

	// Count dynamics
	dynamicCount := 0
	for k := range tree {
		if k != "s" && k != "f" && k != "d" {
			if _, err := fmt.Sscanf(k, "%d", new(int)); err == nil {
				dynamicCount++
			}
		}
	}

	// Verify sequential keys
	for i := 0; i < dynamicCount; i++ {
		key := fmt.Sprintf("%d", i)
		if _, exists := tree[key]; !exists {
			compliance.Violations = append(compliance.Violations,
				fmt.Sprintf("SPEC VIOLATION: Missing sequential key '%s' in dynamics", key))
			return false
		}
	}

	return true
}

// validateMinimalUpdate checks if update contains only changes
func (a *EnhancedTreeAnalyzer) validateMinimalUpdate(tree, lastTree treeNode, compliance *SpecificationCompliance) bool {
	if lastTree == nil {
		return true // Can't validate without previous tree
	}

	// Check each field in update
	for k, newValue := range tree {
		if k == "f" {
			continue // Fingerprint is metadata
		}

		// Check if this field was in last tree
		oldValue, existed := lastTree[k]

		if existed && reflect.DeepEqual(oldValue, newValue) {
			compliance.Violations = append(compliance.Violations,
				fmt.Sprintf("SPEC VIOLATION: Update contains unchanged field '%s'", k))
			return false
		}
	}

	return true
}

// validateRangeGranularity checks if range updates use operations
func (a *EnhancedTreeAnalyzer) validateRangeGranularity(tree treeNode, compliance *SpecificationCompliance) bool {
	granular := true

	var checkNode func(node interface{}, path string)
	checkNode = func(node interface{}, path string) {
		switch v := node.(type) {
		case treeNode:
			// Check for range data
			if rangeData, hasRange := v["d"]; hasRange {
				if rangeSlice, ok := rangeData.([]interface{}); ok {
					if a.LastTree != nil && len(rangeSlice) > 0 {
						// Check if this is operations or full list
						fullNodes := 0
						operations := 0

						for _, item := range rangeSlice {
							if op, ok := item.([]interface{}); ok && len(op) > 0 {
								if opType, ok := op[0].(string); ok {
									if opType == "i" || opType == "u" || opType == "r" || opType == "o" {
										operations++
										continue
									}
								}
							}
							if _, ok := item.(map[string]interface{}); ok {
								fullNodes++
							}
						}

						// If we have full nodes and no operations, this might be non-granular
						if fullNodes > 1 && operations == 0 {
							compliance.Violations = append(compliance.Violations,
								fmt.Sprintf("SPEC VIOLATION: Range at '%s' sending full list (%d items) instead of operations",
									path, fullNodes))
							granular = false
						}
					}
				}
			}

			// Recurse into nested structures
			for k, v := range v {
				if k != "s" && k != "f" {
					newPath := path
					if newPath != "" {
						newPath += "."
					}
					newPath += k
					checkNode(v, newPath)
				}
			}

		case map[string]interface{}:
			checkNode(treeNode(v), path)
		}
	}

	checkNode(tree, "")
	return granular
}

// validateNoRedundantStatics checks that statics aren't resent
func (a *EnhancedTreeAnalyzer) validateNoRedundantStatics(tree treeNode, compliance *SpecificationCompliance) bool {
	valid := true

	var checkNode func(node interface{}, path string)
	checkNode = func(node interface{}, path string) {
		switch v := node.(type) {
		case treeNode:
			// Check for statics at this level
			if _, hasStatics := v["s"]; hasStatics {
				if a.SentStatics[path] {
					compliance.Violations = append(compliance.Violations,
						fmt.Sprintf("SPEC VIOLATION: Redundant statics sent for path '%s'", path))
					valid = false
				}
			}

			// Recurse
			for k, nested := range v {
				if k != "s" && k != "f" {
					newPath := path
					if newPath != "" {
						newPath += "."
					}
					newPath += k
					checkNode(nested, newPath)
				}
			}

		case map[string]interface{}:
			checkNode(treeNode(v), path)
		}
	}

	checkNode(tree, "")
	return valid
}

// markStaticsSent tracks which paths have sent statics
func (a *EnhancedTreeAnalyzer) markStaticsSent(tree treeNode, prefix string) {
	for k, value := range tree {
		fieldPath := prefix
		if fieldPath != "" {
			fieldPath += "."
		}
		fieldPath += k

		if k == "s" {
			a.SentStatics[prefix] = true
		}

		// Recursively mark nested structures
		switch v := value.(type) {
		case treeNode:
			a.markStaticsSent(v, fieldPath)
		case map[string]interface{}:
			a.markStaticsSent(treeNode(v), fieldPath)
		}
	}
}

// calculateMetrics calculates size and efficiency metrics
func (a *EnhancedTreeAnalyzer) calculateMetrics(tree, lastTree treeNode, isFirstRender bool) *UpdateMetrics {
	metrics := &UpdateMetrics{
		UpdateNumber: a.UpdateCount,
	}

	// Calculate tree size
	treeJSON, _ := json.Marshal(tree)
	metrics.OptimizedSize = len(treeJSON)

	// Estimate original size (full HTML that would be sent without optimization)
	if isFirstRender {
		metrics.OriginalSize = metrics.OptimizedSize // First render is baseline
	} else {
		// For updates, original would be full render
		// Estimate: current tree + all statics that weren't sent
		fullTree := a.reconstructFullTree(tree, lastTree)
		fullJSON, _ := json.Marshal(fullTree)
		metrics.OriginalSize = len(fullJSON)
	}

	// Calculate compression ratio
	if metrics.OriginalSize > 0 {
		metrics.CompressionRatio = 1.0 - (float64(metrics.OptimizedSize) / float64(metrics.OriginalSize))
	}

	// Count reused statics and changed dynamics
	metrics.StaticsReused = len(a.SentStatics)
	metrics.DynamicsChanged = a.countDynamics(tree)

	// Count range operations
	metrics.RangeOperations = a.countRangeOperations(tree)

	return metrics
}

// reconstructFullTree estimates what full tree would look like
func (a *EnhancedTreeAnalyzer) reconstructFullTree(update, lastTree treeNode) treeNode {
	if lastTree == nil {
		return update
	}

	// Merge update into last tree
	result := make(treeNode)

	// Copy all from last tree
	for k, v := range lastTree {
		result[k] = v
	}

	// Override with updates
	for k, v := range update {
		result[k] = v
	}

	return result
}

// countDynamics counts dynamic fields in tree
func (a *EnhancedTreeAnalyzer) countDynamics(tree treeNode) int {
	count := 0
	for k := range tree {
		if k != "s" && k != "f" && k != "d" {
			if _, err := fmt.Sscanf(k, "%d", new(int)); err == nil {
				count++
			}
		}
	}
	return count
}

// countRangeOperations counts range operations in tree
func (a *EnhancedTreeAnalyzer) countRangeOperations(tree interface{}) int {
	count := 0

	var countOps func(node interface{})
	countOps = func(node interface{}) {
		switch v := node.(type) {
		case []interface{}:
			for _, item := range v {
				if op, ok := item.([]interface{}); ok && len(op) > 0 {
					if opType, ok := op[0].(string); ok {
						if opType == "i" || opType == "u" || opType == "r" || opType == "o" {
							count++
						}
					}
				}
				countOps(item)
			}
		case treeNode:
			for _, value := range v {
				countOps(value)
			}
		case map[string]interface{}:
			for _, value := range v {
				countOps(value)
			}
		}
	}

	countOps(tree)
	return count
}

// GenerateReport generates a comprehensive analysis report
func (a *EnhancedTreeAnalyzer) GenerateReport() string {
	if a.TotalUpdates == 0 {
		return "No updates analyzed yet"
	}

	var report strings.Builder

	report.WriteString("=== LIVETEMPLATE TREE ANALYSIS REPORT ===\n\n")

	// Compliance Summary
	report.WriteString("SPECIFICATION COMPLIANCE:\n")
	complianceRate := float64(a.TotalUpdates-a.ViolationCount) / float64(a.TotalUpdates) * 100
	report.WriteString(fmt.Sprintf("  Compliance Rate: %.1f%%\n", complianceRate))
	report.WriteString(fmt.Sprintf("  Total Updates: %d\n", a.TotalUpdates))
	report.WriteString(fmt.Sprintf("  Violations: %d\n", a.ViolationCount))
	report.WriteString("\n")

	// Efficiency Metrics
	report.WriteString("EFFICIENCY METRICS:\n")
	if a.TotalBytesOriginal > 0 {
		compressionRatio := 1.0 - (float64(a.TotalBytesOptimized) / float64(a.TotalBytesOriginal))
		report.WriteString(fmt.Sprintf("  Average Compression: %.1f%%\n", compressionRatio*100))
		report.WriteString(fmt.Sprintf("  Total Bytes Saved: %d\n", a.TotalBytesOriginal-a.TotalBytesOptimized))
		report.WriteString(fmt.Sprintf("  Original Size: %d bytes\n", a.TotalBytesOriginal))
		report.WriteString(fmt.Sprintf("  Optimized Size: %d bytes\n", a.TotalBytesOptimized))
	}
	report.WriteString("\n")

	// Performance Metrics
	if a.ProfilingEnabled && len(a.UpdateTimes) > 0 {
		report.WriteString("PERFORMANCE METRICS:\n")

		// Calculate percentiles
		avgTime := time.Duration(0)
		for _, t := range a.UpdateTimes {
			avgTime += t
		}
		avgTime /= time.Duration(len(a.UpdateTimes))

		report.WriteString(fmt.Sprintf("  Average Update Time: %v\n", avgTime))

		// Find min/max
		minTime := a.UpdateTimes[0]
		maxTime := a.UpdateTimes[0]
		for _, t := range a.UpdateTimes {
			if t < minTime {
				minTime = t
			}
			if t > maxTime {
				maxTime = t
			}
		}
		report.WriteString(fmt.Sprintf("  Min Update Time: %v\n", minTime))
		report.WriteString(fmt.Sprintf("  Max Update Time: %v\n", maxTime))
		report.WriteString("\n")
	}

	// Recommendations
	report.WriteString("RECOMMENDATIONS:\n")
	if complianceRate < 100 {
		report.WriteString("  ⚠️ Review specification violations in logs\n")
	}
	if a.TotalBytesOptimized > a.TotalBytesOriginal/2 {
		report.WriteString("  ⚠️ Optimization could be improved - check template structure\n")
	}
	if a.ViolationCount > 0 {
		report.WriteString("  ⚠️ Fix specification violations before production\n")
	} else {
		report.WriteString("  ✅ All updates comply with specification\n")
	}

	report.WriteString("\n=== END REPORT ===\n")

	return report.String()
}

// LogCompliance logs compliance results in a structured format
func (a *EnhancedTreeAnalyzer) LogCompliance(compliance *SpecificationCompliance, metrics *UpdateMetrics) {
	if !compliance.Compliant {
		log.Println("=== SPECIFICATION VIOLATION DETECTED ===")
		log.Printf("Update #%d failed compliance check\n", metrics.UpdateNumber)

		for _, violation := range compliance.Violations {
			log.Printf("  ❌ %s\n", violation)
		}

		log.Println("\nCOMPLIANCE STATUS:")
		log.Printf("  First Render Valid: %v\n", compliance.FirstRenderValid)
		log.Printf("  Updates Minimal: %v\n", compliance.UpdatesMinimal)
		log.Printf("  Ranges Granular: %v\n", compliance.RangesGranular)
		log.Printf("  Statics Not Repeated: %v\n", compliance.StaticsNotRepeated)

		log.Println("=== END VIOLATION REPORT ===")
	}

	if a.MetricsEnabled && metrics != nil {
		log.Printf("Update #%d: %d→%d bytes (%.1f%% reduction), %d range ops, %dµs",
			metrics.UpdateNumber,
			metrics.OriginalSize,
			metrics.OptimizedSize,
			metrics.CompressionRatio*100,
			metrics.RangeOperations,
			metrics.ProcessingTime.Microseconds())
	}
}

// ValidateTreeStructure performs deep structural validation
func ValidateTreeStructure(tree treeNode) error {
	// Check for required structure
	hasStatics := false
	hasDynamics := false

	for k := range tree {
		if k == "s" {
			hasStatics = true
		} else if k != "f" && k != "d" {
			hasDynamics = true
		}
	}

	// Tree should have either statics or dynamics (or both)
	if !hasStatics && !hasDynamics {
		return fmt.Errorf("tree has neither statics nor dynamics")
	}

	// Validate numeric key sequence
	dynamicKeys := make([]int, 0)
	for k := range tree {
		if k != "s" && k != "f" && k != "d" {
			var keyNum int
			if _, err := fmt.Sscanf(k, "%d", &keyNum); err == nil {
				dynamicKeys = append(dynamicKeys, keyNum)
			}
		}
	}

	// Check for sequential keys
	for i := 0; i < len(dynamicKeys); i++ {
		found := false
		for _, key := range dynamicKeys {
			if key == i {
				found = true
				break
			}
		}
		if !found && len(dynamicKeys) > 0 {
			return fmt.Errorf("non-sequential dynamic keys: missing key %d", i)
		}
	}

	// Recursively validate nested structures
	for k, v := range tree {
		if k == "s" {
			// Validate statics array
			if statics, ok := v.([]string); ok {
				if len(statics) == 0 {
					return fmt.Errorf("empty statics array at key 's'")
				}
			} else {
				return fmt.Errorf("'s' key must be string array, got %T", v)
			}
		} else if k == "d" {
			// Validate range data
			if _, ok := v.([]interface{}); !ok {
				return fmt.Errorf("'d' key must be array, got %T", v)
			}
		} else if k != "f" {
			// Validate nested trees
			switch nested := v.(type) {
			case treeNode:
				if err := ValidateTreeStructure(nested); err != nil {
					return fmt.Errorf("nested tree at '%s': %w", k, err)
				}
			case map[string]interface{}:
				if err := ValidateTreeStructure(treeNode(nested)); err != nil {
					return fmt.Errorf("nested tree at '%s': %w", k, err)
				}
			}
		}
	}

	return nil
}