package diff

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// FragmentCache represents the cache state for a specific fragment
type FragmentCache struct {
	lastRendered string
	staticHash   string
	hasStatics   bool
}

// Tree creates a unified static/dynamic tree structure
// No HTML intrinsics knowledge, just template parsing and diffing
type Tree struct {
	templateSource string
	fragmentCache  map[string]*FragmentCache // Per-fragment cache tracking
}

// NewTree creates a new unified tree differ
func NewTree() *Tree {
	return &Tree{
		fragmentCache: make(map[string]*FragmentCache),
	}
}

// Update - Simple and elegant structure with positional dynamics
type Update struct {
	// Static segments - sent on first render, cached forever
	S []string `json:"s,omitempty"`

	// Dynamic values by position - will be marshaled directly to root level
	Dynamics map[string]any `json:"-"`

	// Hash of statics for cache validation (optional)
	H string `json:"h,omitempty"`
}

// Generate creates the optimal tree update
func (u *Tree) Generate(templateSource string, oldData, newData any) (*Update, error) {
	return u.GenerateWithFragmentID(templateSource, oldData, newData, "default")
}

// GenerateWithFragmentID creates the optimal tree update for a specific fragment ID
func (u *Tree) GenerateWithFragmentID(templateSource string, oldData, newData any, fragmentID string) (*Update, error) {
	// Store template for analysis
	if u.templateSource == "" || u.templateSource != templateSource {
		u.templateSource = templateSource
		// Clear all fragment caches when template changes
		u.fragmentCache = make(map[string]*FragmentCache)
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

	// Inject lvt-id attribute into template source for consistent fragment generation
	templateWithLvtId := u.injectLvtIdIntoTemplate(templateSource, fragmentID)
	
	// Extract statics and dynamics from template with lvt-id
	statics, dynamics := u.extractStaticsAndDynamics(templateWithLvtId, newData)

	// Get or create fragment cache
	fragmentCache, exists := u.fragmentCache[fragmentID]
	if !exists {
		fragmentCache = &FragmentCache{}
		u.fragmentCache[fragmentID] = fragmentCache
	}

	// First render for this fragment - send statics
	if !fragmentCache.hasStatics {
		fragmentCache.lastRendered = newRendered
		fragmentCache.staticHash = u.calculateHash(statics)
		fragmentCache.hasStatics = true

		update := &Update{
			S:        statics,
			Dynamics: dynamics,
			H:        fragmentCache.staticHash,
		}
		return update, nil
	}

	// Same output - no update needed
	if fragmentCache.lastRendered == newRendered {
		return &Update{
			Dynamics: make(map[string]any),
		}, nil
	}

	// Only dynamics changed - send just dynamics (client has cached statics)
	fragmentCache.lastRendered = newRendered
	update := &Update{
		Dynamics: dynamics,
	}
	return update, nil
}

// extractStaticsAndDynamics splits template into static and dynamic parts
func (u *Tree) extractStaticsAndDynamics(templateSource string, data any) ([]string, map[string]any) {
	// Find all template expressions
	exprRegex := regexp.MustCompile(`{{[^}]*}}`)
	matches := exprRegex.FindAllStringIndex(templateSource, -1)

	// If no template expressions, entire template is static
	if len(matches) == 0 {
		return []string{templateSource}, make(map[string]any)
	}

	var statics []string
	dynamics := make(map[string]any)
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
func (u *Tree) evaluateExpression(expr string, data any) any {
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
func (u *Tree) calculateHash(statics []string) string {
	h := md5.New()
	for _, s := range statics {
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))[:8] // Short hash
}

// IsEmpty returns true if update contains no changes
func (u *Update) IsEmpty() bool {
	return len(u.S) == 0 && len(u.Dynamics) == 0
}

// HasStatics returns true if update contains static segments
func (u *Update) HasStatics() bool {
	return len(u.S) > 0
}

// HasDynamics returns true if update contains dynamic values
func (u *Update) HasDynamics() bool {
	return len(u.Dynamics) > 0
}

// GetSize returns approximate size in bytes
func (u *Update) GetSize() int {
	size := 0
	for _, s := range u.S {
		size += len(s)
	}
	for k, v := range u.Dynamics {
		size += len(k) + len(fmt.Sprintf("%v", v)) + 10
	}
	if u.H != "" {
		size += len(u.H) + 10
	}
	return size
}

// String provides readable representation
func (u *Update) String() string {
	if u.IsEmpty() {
		return "Update: <empty - no changes>"
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
		parts = append(parts, fmt.Sprintf("Dynamics[%d]", len(u.Dynamics)))
		for k, v := range u.Dynamics {
			parts = append(parts, fmt.Sprintf("  %s: %v", k, v))
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
func (u *Update) Reconstruct(cachedStatics []string) string {
	// If we have new statics, use them
	statics := u.S
	if len(statics) == 0 && len(cachedStatics) > 0 {
		statics = cachedStatics
	}

	// If no dynamics, just join statics
	if len(u.Dynamics) == 0 {
		return strings.Join(statics, "")
	}

	// Interleave statics and dynamics
	var result strings.Builder
	for i := 0; i < len(statics); i++ {
		result.WriteString(statics[i])
		// Only insert dynamic value between static segments
		if i < len(statics)-1 {
			if dynValue, ok := u.Dynamics[fmt.Sprintf("%d", i)]; ok {
				result.WriteString(fmt.Sprintf("%v", dynValue))
			}
		}
	}

	return result.String()
}

// MarshalJSON implements custom JSON marshaling for the flat positional structure
func (u *Update) MarshalJSON() ([]byte, error) {
	// Create a map with all fields
	result := make(map[string]any)

	// Add statics if present
	if len(u.S) > 0 {
		result["s"] = u.S
	}

	// Add hash if present
	if u.H != "" {
		result["h"] = u.H
	}

	// Add dynamics directly to root level
	for k, v := range u.Dynamics {
		result[k] = v
	}

	return json.Marshal(result)
}

// UnmarshalJSON implements custom JSON unmarshaling for the flat positional structure
func (u *Update) UnmarshalJSON(data []byte) error {
	// Parse into generic map first
	var temp map[string]any
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Extract known fields
	if s, ok := temp["s"]; ok {
		if sArray, ok := s.([]any); ok {
			u.S = make([]string, len(sArray))
			for i, v := range sArray {
				if str, ok := v.(string); ok {
					u.S[i] = str
				}
			}
		}
		delete(temp, "s")
	}

	if h, ok := temp["h"]; ok {
		if hStr, ok := h.(string); ok {
			u.H = hStr
		}
		delete(temp, "h")
	}

	// Everything else goes into Dynamics
	if u.Dynamics == nil {
		u.Dynamics = make(map[string]any)
	}
	for k, v := range temp {
		u.Dynamics[k] = v
	}

	return nil
}

// injectLvtIdIntoTemplate adds lvt-id attribute to elements that would get them during rendering
func (u *Tree) injectLvtIdIntoTemplate(templateSource string, fragmentID string) string {
	// For templates with style attributes or class attributes, inject lvt-id
	// This mirrors the logic in Page.injectLvtIds but works on template source
	
	// Look for div elements with style or class attributes (including ones with template expressions)
	// Updated regex to handle style attributes with template expressions like style="color: {{.Color}};"
	elementRegex := regexp.MustCompile(`<(div|span|p|h[1-6]|section|article|main)([^>]*?)(style="[^"]*"|class="[^"]*")([^>]*?)>`)
	
	result := elementRegex.ReplaceAllStringFunc(templateSource, func(match string) string {
		// Check if this element already has an lvt-id attribute
		if strings.Contains(match, "lvt-id=") {
			return match
		}
		
		// Insert lvt-id attribute before the closing >
		return strings.Replace(match, ">", fmt.Sprintf(` lvt-id="%s">`, fragmentID), 1)
	})
	
	return result
}
