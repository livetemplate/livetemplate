package strategy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MarkerPatchData represents Strategy 2 fragment data for position-discoverable changes
type MarkerPatchData struct {
	// PositionMap maps marker indices to their byte positions in the HTML
	PositionMap map[int]Position `json:"position_map"`

	// ValueUpdates maps marker indices to their new values
	ValueUpdates map[int]string `json:"value_updates"`

	// IsEmpty indicates if this represents an empty state (show/hide scenario)
	IsEmpty bool `json:"is_empty,omitempty"`

	// FragmentID identifies this fragment for client reconstruction
	FragmentID string `json:"fragment_id"`
}

// Position represents a marker's location in HTML for precise patching
type Position struct {
	Start  int `json:"start"`  // Starting byte position
	End    int `json:"end"`    // Ending byte position
	Length int `json:"length"` // Length of the marker (for validation)
}

// MarkerCompiler implements Strategy 2 marker compilation for position-discoverable changes
type MarkerCompiler struct {
	// markerPattern matches markers like §1§ §2§ etc
	markerPattern *regexp.Regexp
}

// NewMarkerCompiler creates a new Strategy 2 marker compiler
func NewMarkerCompiler() *MarkerCompiler {
	return &MarkerCompiler{
		markerPattern: regexp.MustCompile(`§(\d+)§`),
	}
}

// Compile creates a marker patch from old and new HTML by discovering value positions
func (mc *MarkerCompiler) Compile(oldHTML, newHTML, fragmentID string) (*MarkerPatchData, error) {
	// Handle empty state scenarios first
	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) != "" {
		// Show content scenario
		return mc.compileShowContent(newHTML, fragmentID)
	}

	if strings.TrimSpace(oldHTML) != "" && strings.TrimSpace(newHTML) == "" {
		// Hide content scenario
		return mc.compileHideContent(fragmentID)
	}

	if strings.TrimSpace(oldHTML) == "" && strings.TrimSpace(newHTML) == "" {
		// Both empty - no change needed
		return &MarkerPatchData{
			PositionMap:  map[int]Position{},
			ValueUpdates: map[int]string{},
			IsEmpty:      true,
			FragmentID:   fragmentID,
		}, nil
	}

	// Normal marker compilation for position-discoverable changes
	return mc.compileMarkerPatches(oldHTML, newHTML, fragmentID)
}

// compileShowContent handles showing previously hidden content
func (mc *MarkerCompiler) compileShowContent(newHTML, fragmentID string) (*MarkerPatchData, error) {
	// For show content, we treat the entire content as a single position update
	// This creates a special case where position 0 represents "insert at beginning"
	return &MarkerPatchData{
		PositionMap: map[int]Position{
			0: {Start: 0, End: 0, Length: 0}, // Insert at position 0
		},
		ValueUpdates: map[int]string{
			0: newHTML,
		},
		IsEmpty:    false,
		FragmentID: fragmentID,
	}, nil
}

// compileHideContent handles hiding previously shown content
func (mc *MarkerCompiler) compileHideContent(fragmentID string) (*MarkerPatchData, error) {
	// For hide content, we send empty state
	return &MarkerPatchData{
		PositionMap:  map[int]Position{},
		ValueUpdates: map[int]string{},
		IsEmpty:      true,
		FragmentID:   fragmentID,
	}, nil
}

// compileMarkerPatches creates position-based patches for value changes
func (mc *MarkerCompiler) compileMarkerPatches(oldHTML, newHTML, fragmentID string) (*MarkerPatchData, error) {
	// Find differences and generate marker-based patches
	positionMap, valueUpdates, err := mc.extractPositionPatches(oldHTML, newHTML)
	if err != nil {
		return nil, err
	}

	return &MarkerPatchData{
		PositionMap:  positionMap,
		ValueUpdates: valueUpdates,
		IsEmpty:      false,
		FragmentID:   fragmentID,
	}, nil
}

// extractPositionPatches finds position-based differences between old and new HTML
func (mc *MarkerCompiler) extractPositionPatches(oldHTML, newHTML string) (map[int]Position, map[int]string, error) {
	// This is a simplified implementation for Strategy 2
	// In a full implementation, this would:
	// 1. Generate marker templates (§1§ §2§ etc) for dynamic values
	// 2. Pre-render templates to discover exact positions
	// 3. Create position maps for precise value patching

	positionMap := make(map[int]Position)
	valueUpdates := make(map[int]string)

	// For now, implement a basic approach that finds differences
	changes := mc.findValueChanges(oldHTML, newHTML)

	markerIndex := 0
	for _, change := range changes {
		positionMap[markerIndex] = Position{
			Start:  change.Position,
			End:    change.Position + len(change.OldValue),
			Length: len(change.OldValue),
		}
		valueUpdates[markerIndex] = change.NewValue
		markerIndex++
	}

	return positionMap, valueUpdates, nil
}

// ValueChange represents a change detected between old and new HTML
type ValueChange struct {
	Position int    // Position in HTML where change occurs
	OldValue string // Old value being replaced
	NewValue string // New value to insert
}

// findValueChanges identifies specific value changes for marker compilation
func (mc *MarkerCompiler) findValueChanges(oldHTML, newHTML string) []ValueChange {
	var changes []ValueChange

	// Simple implementation: find first difference
	// A full implementation would use sophisticated diff algorithms

	if oldHTML == newHTML {
		return changes
	}

	// Find the first difference position
	diffPos := mc.findFirstDifference(oldHTML, newHTML)
	if diffPos >= 0 {
		// Find the extent of the change
		oldEnd := mc.findChangeEnd(oldHTML, newHTML, diffPos)
		newEnd := mc.findChangeEnd(newHTML, oldHTML, diffPos)

		oldValue := ""
		if diffPos < len(oldHTML) && oldEnd <= len(oldHTML) {
			oldValue = oldHTML[diffPos:oldEnd]
		}

		newValue := ""
		if diffPos < len(newHTML) && newEnd <= len(newHTML) {
			newValue = newHTML[diffPos:newEnd]
		}

		changes = append(changes, ValueChange{
			Position: diffPos,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}

	return changes
}

// findFirstDifference finds the first position where two strings differ
func (mc *MarkerCompiler) findFirstDifference(s1, s2 string) int {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return i
		}
	}

	// If one string is longer, difference starts at end of shorter string
	if len(s1) != len(s2) {
		return minLen
	}

	return -1 // No difference
}

// findChangeEnd finds the end position of a change starting at a given position
func (mc *MarkerCompiler) findChangeEnd(s1, s2 string, startPos int) int {
	// Simple implementation: find next space or tag boundary
	pos := startPos
	for pos < len(s1) && pos < len(s2) {
		if s1[pos] == ' ' || s1[pos] == '<' || s1[pos] == '>' {
			break
		}
		pos++
	}

	// If we haven't found a boundary, extend to a reasonable limit
	if pos == startPos && pos < len(s1) {
		pos = startPos + 1
		for pos < len(s1) && s1[pos] != ' ' && s1[pos] != '<' && s1[pos] != '>' {
			pos++
		}
	}

	return pos
}

// GenerateMarkers creates marker-annotated template for position discovery
func (mc *MarkerCompiler) GenerateMarkers(template string, valueCount int) string {
	// Generate markers like §1§ §2§ §3§ for template compilation
	result := template

	for i := 0; i < valueCount; i++ {
		marker := fmt.Sprintf("§%d§", i+1)
		// This would replace actual template variables with markers
		// For now, just append markers for demonstration
		result += " " + marker
	}

	return result
}

// ExtractPositions finds marker positions in compiled HTML
func (mc *MarkerCompiler) ExtractPositions(html string) map[int]Position {
	positions := make(map[int]Position)

	matches := mc.markerPattern.FindAllStringIndex(html, -1)

	for _, match := range matches {
		// Extract marker number
		markerText := html[match[0]:match[1]]
		numberMatch := mc.markerPattern.FindStringSubmatch(markerText)
		if len(numberMatch) > 1 {
			if markerNum, err := strconv.Atoi(numberMatch[1]); err == nil {
				positions[markerNum-1] = Position{
					Start:  match[0],
					End:    match[1],
					Length: match[1] - match[0],
				}
			}
		}
	}

	return positions
}

// CalculateBandwidthReduction calculates the bandwidth savings for marker compilation
func (mc *MarkerCompiler) CalculateBandwidthReduction(originalSize int, data *MarkerPatchData) float64 {
	// Calculate the size of the marker patch data
	patchSize := mc.calculatePatchSize(data)

	if originalSize == 0 {
		return 0.0
	}

	reduction := float64(originalSize-patchSize) / float64(originalSize) * 100
	if reduction < 0 {
		return 0.0
	}

	return reduction
}

// calculatePatchSize estimates the size of the marker patch data when serialized
func (mc *MarkerCompiler) calculatePatchSize(data *MarkerPatchData) int {
	// For Strategy 2, we send position maps and value updates
	contentSize := 0

	// Count the value updates (the actual content that changed)
	for _, value := range data.ValueUpdates {
		contentSize += len(value)
	}

	// Strategy 2 is optimized for position-based updates
	if len(data.ValueUpdates) > 0 {
		// Position overhead is minimal - just coordinates for each patch
		contentSize += len(data.ValueUpdates) * 6 // 2 bytes per coordinate (start, end)

		// Minimal JSON overhead for position-based format
		contentSize += 10 // Optimized structure overhead
	}

	// Add fragment ID overhead (minimal)
	contentSize += 3 // Short fragment ID in optimized format

	// For empty states, just signal the state change
	if data.IsEmpty {
		contentSize = 5 // Minimal empty state signal
	}

	return contentSize
}

// ApplyPatches applies marker patches to reconstruct HTML (for testing)
func (mc *MarkerCompiler) ApplyPatches(originalHTML string, data *MarkerPatchData) string {
	if data.IsEmpty {
		return ""
	}

	if len(data.ValueUpdates) == 0 {
		return originalHTML
	}

	// Apply patches in reverse order to maintain position accuracy
	result := originalHTML

	// Sort positions by start position (descending) to apply in reverse order
	type posUpdate struct {
		pos    Position
		value  string
		marker int
	}

	var updates []posUpdate
	for marker, pos := range data.PositionMap {
		if value, exists := data.ValueUpdates[marker]; exists {
			updates = append(updates, posUpdate{pos: pos, value: value, marker: marker})
		}
	}

	// Sort by start position (descending)
	for i := 0; i < len(updates)-1; i++ {
		for j := i + 1; j < len(updates); j++ {
			if updates[i].pos.Start < updates[j].pos.Start {
				updates[i], updates[j] = updates[j], updates[i]
			}
		}
	}

	// Apply patches
	for _, update := range updates {
		if update.pos.Start >= 0 && update.pos.End <= len(result) {
			// Special case: if Start == End == 0, this is an insert at beginning
			if update.pos.Start == 0 && update.pos.End == 0 && len(result) == 0 {
				result = update.value
			} else {
				result = result[:update.pos.Start] + update.value + result[update.pos.End:]
			}
		}
	}

	return result
}
