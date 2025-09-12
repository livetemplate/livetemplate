package strategy

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// DOMTreeHybrid combines DOM diffing with tree-based updates
// Server: Pure DOM diffing (no HTML intrinsics)
// Client: Tree structures compatible with morphdom
type DOMTreeHybrid struct {
	lastHTML string
}

// NewDOMTreeHybrid creates a new hybrid DOM/Tree generator
func NewDOMTreeHybrid() *DOMTreeHybrid {
	return &DOMTreeHybrid{}
}

// TreeUpdate represents a tree-based update compatible with morphdom
type TreeUpdate struct {
	FragmentID string                 `json:"fragment_id"`
	HTML       string                 `json:"html"`               // Full HTML for morphdom
	Statics    []string               `json:"s,omitempty"`        // Static segments (optional optimization)
	Dynamics   map[string]interface{} `json:"dynamics,omitempty"` // Dynamic values (optional)
}

// GenerateTreeUpdate creates a tree update by diffing HTML and extracting structure
func (d *DOMTreeHybrid) GenerateTreeUpdate(templateSource string, oldData, newData interface{}, fragmentID string) (*TreeUpdate, error) {
	// Parse the template
	tmpl, err := template.New("hybrid").Parse(templateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	// Render new HTML
	var newBuf bytes.Buffer
	if err := tmpl.Execute(&newBuf, newData); err != nil {
		return nil, fmt.Errorf("failed to render template: %v", err)
	}
	newHTML := newBuf.String()

	// For first render, return full HTML
	if d.lastHTML == "" {
		d.lastHTML = newHTML
		return &TreeUpdate{
			FragmentID: fragmentID,
			HTML:       newHTML,
		}, nil
	}

	// Check if HTML actually changed
	if d.lastHTML == newHTML {
		// No changes needed - return empty update
		return &TreeUpdate{
			FragmentID: fragmentID,
		}, nil
	}

	// HTML changed - create update with optimization analysis
	update := &TreeUpdate{
		FragmentID: fragmentID,
		HTML:       newHTML,
	}

	// Optional: Extract static/dynamic structure for bandwidth optimization
	if oldData != nil {
		if statics, dynamics := d.extractStaticDynamicStructure(templateSource, oldData, newData); len(statics) > 0 {
			update.Statics = statics
			update.Dynamics = dynamics
		}
	}

	// Store for next comparison
	d.lastHTML = newHTML

	return update, nil
}

// extractStaticDynamicStructure analyzes the template to separate static and dynamic content
// This is purely for bandwidth optimization - client can fall back to full HTML if needed
func (d *DOMTreeHybrid) extractStaticDynamicStructure(templateSource string, oldData, newData interface{}) ([]string, map[string]interface{}) {
	// Simple approach: find template expressions and split around them
	// This is much simpler than the current template parsing approach

	templateExprRegex := regexp.MustCompile(`{{[^}]*}}`)
	matches := templateExprRegex.FindAllStringIndex(templateSource, -1)

	if len(matches) == 0 {
		// No template expressions - everything is static
		return []string{templateSource}, nil
	}

	var statics []string
	dynamics := make(map[string]interface{})
	lastEnd := 0

	// Render both versions to extract actual values
	oldHTML := d.renderTemplate(templateSource, oldData)
	newHTML := d.renderTemplate(templateSource, newData)

	// Split template into static segments
	for i, match := range matches {
		// Add static content before this expression
		if match[0] > lastEnd {
			staticPart := templateSource[lastEnd:match[0]]
			statics = append(statics, staticPart)
		}

		// Find the corresponding dynamic value in the rendered output
		// This is a simplified approach - in practice you'd want more sophisticated mapping
		dynamicKey := fmt.Sprintf("%d", i)

		// Extract the actual rendered value by comparing old and new HTML
		if oldHTML != newHTML {
			dynamics[dynamicKey] = d.extractDynamicValue(templateSource[match[0]:match[1]], newData)
		}

		lastEnd = match[1]
	}

	// Add remaining static content
	if lastEnd < len(templateSource) {
		statics = append(statics, templateSource[lastEnd:])
	}

	return statics, dynamics
}

// renderTemplate safely renders a template with data
func (d *DOMTreeHybrid) renderTemplate(templateSource string, data interface{}) string {
	if data == nil {
		return ""
	}

	tmpl, err := template.New("extract").Parse(templateSource)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return ""
	}

	return buf.String()
}

// extractDynamicValue evaluates a template expression with data
func (d *DOMTreeHybrid) extractDynamicValue(expression string, data interface{}) interface{} {
	// Simple template expression evaluation
	// In practice, you'd want a more robust expression evaluator
	tmpl, err := template.New("eval").Parse(expression)
	if err != nil {
		return expression // Fallback to raw expression
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return expression // Fallback to raw expression
	}

	return buf.String()
}

// String provides readable representation of the update
func (u *TreeUpdate) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Tree update for fragment: %s", u.FragmentID))

	if u.HTML != "" {
		// Show first 100 chars of HTML
		htmlPreview := u.HTML
		if len(htmlPreview) > 100 {
			htmlPreview = htmlPreview[:100] + "..."
		}
		lines = append(lines, fmt.Sprintf("  HTML: %s", htmlPreview))
	}

	if len(u.Statics) > 0 {
		lines = append(lines, fmt.Sprintf("  Statics: %d segments", len(u.Statics)))
	}

	if len(u.Dynamics) > 0 {
		lines = append(lines, fmt.Sprintf("  Dynamics: %d values", len(u.Dynamics)))
		for k, v := range u.Dynamics {
			lines = append(lines, fmt.Sprintf("    %s: %v", k, v))
		}
	}

	return strings.Join(lines, "\n")
}

// IsEmpty returns true if this update contains no changes
func (u *TreeUpdate) IsEmpty() bool {
	return u.HTML == "" && len(u.Statics) == 0 && len(u.Dynamics) == 0
}

// GetUpdateSize returns approximate size in bytes for bandwidth calculation
func (u *TreeUpdate) GetUpdateSize() int {
	size := len(u.HTML)

	for _, static := range u.Statics {
		size += len(static)
	}

	// Rough estimate for dynamics (JSON overhead)
	for k, v := range u.Dynamics {
		size += len(k) + len(fmt.Sprintf("%v", v)) + 10 // JSON overhead estimate
	}

	return size
}
