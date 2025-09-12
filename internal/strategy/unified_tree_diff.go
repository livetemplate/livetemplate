package strategy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// UnifiedTreeDiff creates a unified static/dynamic tree structure
// No HTML intrinsics knowledge, just template parsing and diffing
type UnifiedTreeDiff struct {
	templateSource string
	lastRendered   string
	staticHash     string
}

// NewUnifiedTreeDiff creates a new unified tree differ
func NewUnifiedTreeDiff() *UnifiedTreeDiff {
	return &UnifiedTreeDiff{}
}

// UnifiedTreeUpdate - Simple and elegant structure
type UnifiedTreeUpdate struct {
	// Static segments - sent on first render, cached forever
	// Can be as large as the full HTML if template has no dynamics
	S []string `json:"s,omitempty"`

	// Dynamic values - always sent when values change
	// Empty map means no dynamic values (pure static template)
	D map[string]interface{} `json:"d,omitempty"`

	// Hash of statics for cache validation (optional)
	H string `json:"h,omitempty"`
}

// Generate creates the optimal tree update
func (u *UnifiedTreeDiff) Generate(templateSource string, oldData, newData interface{}) (*UnifiedTreeUpdate, error) {
	// Store template for analysis
	if u.templateSource == "" || u.templateSource != templateSource {
		u.templateSource = templateSource
		u.staticHash = ""
	}

	// Render with new data
	tmpl, err := template.New("unified").Parse(templateSource)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, newData); err != nil {
		return nil, fmt.Errorf("template execute error: %v", err)
	}
	newRendered := buf.String()

	// Extract statics and dynamics from template
	statics, dynamics := u.extractStaticsAndDynamics(templateSource, newData)

	// First render or template changed - send statics
	if u.lastRendered == "" || u.staticHash == "" {
		u.lastRendered = newRendered
		u.staticHash = u.calculateHash(statics)

		return &UnifiedTreeUpdate{
			S: statics,
			D: dynamics,
			H: u.staticHash,
		}, nil
	}

	// Same output - no update needed
	if u.lastRendered == newRendered {
		return &UnifiedTreeUpdate{
			// Empty update - nothing changed
		}, nil
	}

	// Only dynamics changed - send just dynamics
	u.lastRendered = newRendered
	return &UnifiedTreeUpdate{
		D: dynamics,
	}, nil
}

// extractStaticsAndDynamics splits template into static and dynamic parts
func (u *UnifiedTreeDiff) extractStaticsAndDynamics(templateSource string, data interface{}) ([]string, map[string]interface{}) {
	// Find all template expressions
	exprRegex := regexp.MustCompile(`{{[^}]*}}`)
	matches := exprRegex.FindAllStringIndex(templateSource, -1)

	// If no template expressions, entire template is static
	if len(matches) == 0 {
		return []string{templateSource}, make(map[string]interface{})
	}

	var statics []string
	dynamics := make(map[string]interface{})
	lastEnd := 0

	// Extract static segments between expressions
	for i, match := range matches {
		// Static part before this expression
		if match[0] > lastEnd {
			statics = append(statics, templateSource[lastEnd:match[0]])
		} else if match[0] == lastEnd && i == 0 {
			// Empty static at beginning
			statics = append(statics, "")
		}

		// Evaluate the expression to get dynamic value
		expr := templateSource[match[0]:match[1]]
		value := u.evaluateExpression(expr, data)
		dynamics[fmt.Sprintf("%d", i)] = value

		lastEnd = match[1]
	}

	// Add remaining static content after last expression
	if lastEnd < len(templateSource) {
		statics = append(statics, templateSource[lastEnd:])
	} else {
		// Empty static at end
		statics = append(statics, "")
	}

	return statics, dynamics
}

// evaluateExpression evaluates a template expression
func (u *UnifiedTreeDiff) evaluateExpression(expr string, data interface{}) interface{} {
	// For complex expressions (if/range/etc), we treat them as static
	// and return the entire evaluated result
	if strings.Contains(expr, "if ") || strings.Contains(expr, "range ") {
		// Complex expression - evaluate and return as string
		tmpl, err := template.New("eval").Parse(expr)
		if err != nil {
			return ""
		}
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, data)
		return buf.String()
	}

	// Simple field expression - extract the value
	// Clean up the expression
	cleaned := strings.TrimSpace(strings.Trim(expr, "{}"))

	// Handle simple field access like .Name or .User.Name
	if strings.HasPrefix(cleaned, ".") {
		tmpl, err := template.New("field").Parse(expr)
		if err != nil {
			return ""
		}
		var buf bytes.Buffer
		_ = tmpl.Execute(&buf, data)
		return buf.String()
	}

	return ""
}

// calculateHash creates a hash of static segments
func (u *UnifiedTreeDiff) calculateHash(statics []string) string {
	h := md5.New()
	for _, s := range statics {
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))[:8] // Short hash
}

// IsEmpty returns true if update contains no changes
func (u *UnifiedTreeUpdate) IsEmpty() bool {
	return len(u.S) == 0 && len(u.D) == 0
}

// HasStatics returns true if update contains static segments
func (u *UnifiedTreeUpdate) HasStatics() bool {
	return len(u.S) > 0
}

// HasDynamics returns true if update contains dynamic values
func (u *UnifiedTreeUpdate) HasDynamics() bool {
	return len(u.D) > 0
}

// GetSize returns approximate size in bytes
func (u *UnifiedTreeUpdate) GetSize() int {
	size := 0
	for _, s := range u.S {
		size += len(s)
	}
	for k, v := range u.D {
		size += len(k) + len(fmt.Sprintf("%v", v)) + 10
	}
	if u.H != "" {
		size += len(u.H) + 10
	}
	return size
}

// String provides readable representation
func (u *UnifiedTreeUpdate) String() string {
	if u.IsEmpty() {
		return "UnifiedTreeUpdate: <empty - no changes>"
	}

	var parts []string

	if u.HasStatics() {
		parts = append(parts, fmt.Sprintf("Statics[%d]", len(u.S)))
		for i, s := range u.S {
			preview := s
			if len(preview) > 40 {
				preview = preview[:40] + "..."
			}
			parts = append(parts, fmt.Sprintf("  S[%d]: %q", i, preview))
		}
	}

	if u.HasDynamics() {
		parts = append(parts, fmt.Sprintf("Dynamics[%d]", len(u.D)))
		for k, v := range u.D {
			parts = append(parts, fmt.Sprintf("  D[%s]: %v", k, v))
		}
	}

	if u.H != "" {
		parts = append(parts, fmt.Sprintf("Hash: %s", u.H))
	}

	parts = append(parts, fmt.Sprintf("Size: %d bytes", u.GetSize()))

	return strings.Join(parts, "\n")
}

// Reconstruct builds the full HTML from statics and dynamics
// This is what the client would do
func (u *UnifiedTreeUpdate) Reconstruct(cachedStatics []string) string {
	// If we have new statics, use them
	statics := u.S
	if len(statics) == 0 && len(cachedStatics) > 0 {
		statics = cachedStatics
	}

	// If no dynamics, just join statics
	if len(u.D) == 0 {
		return strings.Join(statics, "")
	}

	// Interleave statics and dynamics
	var result strings.Builder
	for i := 0; i < len(statics); i++ {
		result.WriteString(statics[i])
		// Only insert dynamic value between static segments
		if i < len(statics)-1 {
			if dynValue, ok := u.D[fmt.Sprintf("%d", i)]; ok {
				result.WriteString(fmt.Sprintf("%v", dynValue))
			}
		}
	}

	return result.String()
}
