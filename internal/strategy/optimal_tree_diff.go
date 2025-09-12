package strategy

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
)

// OptimalTreeDiff combines DOM diffing with static/dynamic separation
// Best of all worlds: No HTML intrinsics + Static caching + Minimal updates
type OptimalTreeDiff struct {
	// Cache the template source for static extraction
	templateSource string
	// Cache the last rendered tree
	lastTree *html.Node
	// Cache the static segments from template
	staticSegments []string
	// Cache static hash for validation
	staticHash string
}

// NewOptimalTreeDiff creates a new optimal tree differ
func NewOptimalTreeDiff() *OptimalTreeDiff {
	return &OptimalTreeDiff{}
}

// OptimalTreeUpdate represents the most efficient update format
type OptimalTreeUpdate struct {
	FragmentID string `json:"fragment_id"`
	Type       string `json:"type"` // "full", "dynamic", "none"

	// For first render - includes static segments
	Statics    []string `json:"s,omitempty"`
	StaticHash string   `json:"static_hash,omitempty"`

	// Dynamic values - indexed by position
	Dynamics map[string]interface{} `json:"d,omitempty"`

	// For complex structural changes that can't be expressed as simple dynamics
	Patches []TreePatch `json:"patches,omitempty"`

	// Fallback for first render
	FullHTML string `json:"full_html,omitempty"`
}

// TreePatch represents a structural change
type TreePatch struct {
	Path []int  `json:"path"`
	Type string `json:"type"` // "replace", "add", "remove"
	HTML string `json:"html,omitempty"`
}

// GenerateOptimalUpdate creates the most efficient update possible
func (d *OptimalTreeDiff) GenerateOptimalUpdate(templateSource string, oldData, newData interface{}, fragmentID string) (*OptimalTreeUpdate, error) {
	// Store template source for static extraction
	if d.templateSource == "" {
		d.templateSource = templateSource
		d.extractStaticSegments(templateSource)
	}

	// Render new HTML
	tmpl, err := template.New("optimal").Parse(templateSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	var newBuf bytes.Buffer
	if err := tmpl.Execute(&newBuf, newData); err != nil {
		return nil, fmt.Errorf("failed to render template: %v", err)
	}
	newHTML := newBuf.String()

	// Parse new HTML tree
	newTree, err := d.parseHTMLToTree(newHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	// First render - send statics + dynamics
	if d.lastTree == nil {
		d.lastTree = newTree
		dynamics := d.extractDynamicValues(templateSource, newData)

		return &OptimalTreeUpdate{
			FragmentID: fragmentID,
			Type:       "full",
			Statics:    d.staticSegments,
			StaticHash: d.staticHash,
			Dynamics:   dynamics,
			FullHTML:   newHTML, // Fallback for clients that don't support tree updates
		}, nil
	}

	// Check if only dynamic values changed (most common case)
	if d.isStructurallyIdentical(d.lastTree, newTree) {
		// Extract only dynamic values
		dynamics := d.extractDynamicValues(templateSource, newData)

		// Check if dynamics actually changed
		oldDynamics := d.extractDynamicValues(templateSource, oldData)
		if d.dynamicsEqual(oldDynamics, dynamics) {
			return &OptimalTreeUpdate{
				FragmentID: fragmentID,
				Type:       "none",
			}, nil
		}

		d.lastTree = newTree
		return &OptimalTreeUpdate{
			FragmentID: fragmentID,
			Type:       "dynamic",
			Dynamics:   dynamics,
		}, nil
	}

	// Structural changes - need patches
	patches := d.generatePatches(d.lastTree, newTree, []int{})
	d.lastTree = newTree

	return &OptimalTreeUpdate{
		FragmentID: fragmentID,
		Type:       "dynamic",
		Dynamics:   d.extractDynamicValues(templateSource, newData),
		Patches:    patches,
	}, nil
}

// extractStaticSegments analyzes template to find static parts
func (d *OptimalTreeDiff) extractStaticSegments(templateSource string) {
	// Split template by {{ }} expressions
	parts := strings.Split(templateSource, "{{")
	d.staticSegments = []string{}

	for i, part := range parts {
		if i == 0 {
			// First part is always static
			d.staticSegments = append(d.staticSegments, part)
		} else {
			// Find end of expression
			if idx := strings.Index(part, "}}"); idx >= 0 {
				// Everything after }} is static
				staticPart := part[idx+2:]
				d.staticSegments = append(d.staticSegments, staticPart)
			}
		}
	}

	// Calculate hash of static segments
	h := md5.New()
	for _, seg := range d.staticSegments {
		h.Write([]byte(seg))
	}
	d.staticHash = hex.EncodeToString(h.Sum(nil))
}

// extractDynamicValues extracts all dynamic values from the data
func (d *OptimalTreeDiff) extractDynamicValues(templateSource string, data interface{}) map[string]interface{} {
	dynamics := make(map[string]interface{})

	// Parse template to find all expressions
	expressions := d.findTemplateExpressions(templateSource)

	// Evaluate each expression with the data
	for i, expr := range expressions {
		value := d.evaluateExpression(expr, data)
		dynamics[fmt.Sprintf("%d", i)] = value
	}

	return dynamics
}

// findTemplateExpressions finds all {{ }} expressions in template
func (d *OptimalTreeDiff) findTemplateExpressions(templateSource string) []string {
	var expressions []string

	remaining := templateSource
	for {
		start := strings.Index(remaining, "{{")
		if start == -1 {
			break
		}

		end := strings.Index(remaining[start:], "}}")
		if end == -1 {
			break
		}

		expr := remaining[start : start+end+2]
		expressions = append(expressions, expr)
		remaining = remaining[start+end+2:]
	}

	return expressions
}

// evaluateExpression evaluates a template expression with data
func (d *OptimalTreeDiff) evaluateExpression(expr string, data interface{}) interface{} {
	// Simple evaluation - in production, use proper template evaluation
	tmpl, err := template.New("eval").Parse(expr)
	if err != nil {
		return expr
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return expr
	}

	result := buf.String()
	// Try to preserve type information
	if result == "true" || result == "false" {
		return result == "true"
	}

	return result
}

// isStructurallyIdentical checks if two trees have the same structure
func (d *OptimalTreeDiff) isStructurallyIdentical(oldTree, newTree *html.Node) bool {
	if oldTree == nil || newTree == nil {
		return oldTree == newTree
	}

	// Check node type and tag
	if oldTree.Type != newTree.Type {
		return false
	}

	if oldTree.Type == html.ElementNode {
		if oldTree.Data != newTree.Data {
			return false
		}

		// Check same number of meaningful children
		oldChildren := d.getMeaningfulChildren(oldTree)
		newChildren := d.getMeaningfulChildren(newTree)

		if len(oldChildren) != len(newChildren) {
			return false
		}

		// Recursively check children
		for i := range oldChildren {
			if !d.isStructurallyIdentical(oldChildren[i], newChildren[i]) {
				return false
			}
		}
	}

	return true
}

// generatePatches creates patches for structural changes
func (d *OptimalTreeDiff) generatePatches(oldTree, newTree *html.Node, path []int) []TreePatch {
	var patches []TreePatch

	// This is simplified - in production, implement proper patch generation
	if !d.isStructurallyIdentical(oldTree, newTree) {
		patches = append(patches, TreePatch{
			Path: path,
			Type: "replace",
			HTML: d.nodeToHTML(newTree),
		})
	}

	return patches
}

// Helper methods

func (d *OptimalTreeDiff) parseHTMLToTree(htmlStr string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}

	// Find first element
	var findFirstElement func(*html.Node) *html.Node
	findFirstElement = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findFirstElement(c); result != nil {
				return result
			}
		}
		return nil
	}

	return findFirstElement(doc), nil
}

func (d *OptimalTreeDiff) getMeaningfulChildren(node *html.Node) []*html.Node {
	var children []*html.Node
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		children = append(children, c)
	}
	return children
}

func (d *OptimalTreeDiff) nodeToHTML(node *html.Node) string {
	var buf bytes.Buffer
	_ = html.Render(&buf, node)
	return buf.String()
}

func (d *OptimalTreeDiff) dynamicsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		if v2, ok := b[k]; !ok || fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			return false
		}
	}
	return true
}

// GetUpdateSize calculates the update size in bytes
func (u *OptimalTreeUpdate) GetUpdateSize() int {
	size := 0

	// Statics (only sent once)
	for _, s := range u.Statics {
		size += len(s)
	}

	// Dynamics
	for k, v := range u.Dynamics {
		size += len(k) + len(fmt.Sprintf("%v", v)) + 10
	}

	// Patches
	for _, p := range u.Patches {
		size += len(p.HTML) + 20
	}

	// Full HTML fallback
	size += len(u.FullHTML)

	return size
}

// String provides readable representation
func (u *OptimalTreeUpdate) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("OptimalTreeUpdate for fragment: %s", u.FragmentID))
	lines = append(lines, fmt.Sprintf("  Type: %s", u.Type))

	if len(u.Statics) > 0 {
		lines = append(lines, fmt.Sprintf("  Statics: %d segments (hash: %s)", len(u.Statics), u.StaticHash))
		for i, s := range u.Statics {
			preview := s
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			lines = append(lines, fmt.Sprintf("    S[%d]: %q", i, preview))
		}
	}

	if len(u.Dynamics) > 0 {
		lines = append(lines, fmt.Sprintf("  Dynamics: %d values", len(u.Dynamics)))
		for k, v := range u.Dynamics {
			lines = append(lines, fmt.Sprintf("    D[%s]: %v", k, v))
		}
	}

	if len(u.Patches) > 0 {
		lines = append(lines, fmt.Sprintf("  Patches: %d", len(u.Patches)))
		for i, p := range u.Patches {
			lines = append(lines, fmt.Sprintf("    %d. [%s] path=%v", i+1, p.Type, p.Path))
		}
	}

	return strings.Join(lines, "\n")
}
