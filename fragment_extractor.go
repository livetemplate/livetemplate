package statetemplate

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// TemplateFragment represents an extracted template fragment
type TemplateFragment struct {
	ID           string   // 6-character random ID
	Content      string   // The minimal HTML content with template expressions
	Dependencies []string // Field dependencies found in this fragment
	StartPos     int      // Start position in original template
	EndPos       int      // End position in original template
}

// FragmentExtractor handles automatic extraction of template fragments
type FragmentExtractor struct {
	analyzer *AdvancedTemplateAnalyzer
}

// NewFragmentExtractor creates a new fragment extractor
func NewFragmentExtractor() *FragmentExtractor {
	return &FragmentExtractor{
		analyzer: NewAdvancedTemplateAnalyzer(),
	}
}

// ExtractFragments automatically extracts minimal template fragments from template content
func (fe *FragmentExtractor) ExtractFragments(templateContent string) ([]*TemplateFragment, string, error) {
	var fragments []*TemplateFragment
	modifiedContent := templateContent

	// Find all template expressions {{.*}}
	expressionRegex := regexp.MustCompile(`\{\{[^}]*\}\}`)
	matches := expressionRegex.FindAllStringIndex(templateContent, -1)

	for _, match := range matches {
		start := match[0]
		end := match[1]

		// Extract the surrounding minimal HTML context
		fragment := fe.extractMinimalFragment(templateContent, start, end)
		if fragment != nil {
			fragments = append(fragments, fragment)
		}
	}

	// Replace fragments in the original template with template calls
	modifiedContent, err := fe.replaceFragmentsWithCalls(templateContent, fragments)
	if err != nil {
		return nil, "", err
	}

	return fragments, modifiedContent, nil
}

// extractMinimalFragment extracts the minimal HTML context around a template expression
func (fe *FragmentExtractor) extractMinimalFragment(content string, exprStart, exprEnd int) *TemplateFragment {
	// Find the minimal containing element or text node

	// Look backwards to find the start of the containing element or text
	fragmentStart := fe.findFragmentStart(content, exprStart)

	// Look forwards to find the end of the containing element or text
	fragmentEnd := fe.findFragmentEnd(content, exprEnd)

	// Extract the fragment content
	fragmentContent := strings.TrimSpace(content[fragmentStart:fragmentEnd])

	// Skip if fragment is too small or doesn't contain meaningful content
	if len(fragmentContent) < 3 || !strings.Contains(fragmentContent, "{{") {
		return nil
	}

	// Generate random 6-character ID
	id := fe.generateRandomID()

	// Analyze dependencies in this fragment
	dependencies := fe.analyzer.extractFieldReferences(fragmentContent)

	return &TemplateFragment{
		ID:           id,
		Content:      fragmentContent,
		Dependencies: dependencies,
		StartPos:     fragmentStart,
		EndPos:       fragmentEnd,
	}
}

// findFragmentStart finds the logical start of a template fragment
func (fe *FragmentExtractor) findFragmentStart(content string, exprStart int) int {
	// Look backwards for meaningful boundaries
	i := exprStart

	// Skip whitespace backwards to find start of text node or element
	for i > 0 && (content[i-1] == ' ' || content[i-1] == '\t' || content[i-1] == '\n' || content[i-1] == '\r') {
		i--
	}

	// If we hit a closing tag, go back to find the start of the text node
	if i > 0 && content[i-1] == '>' {
		// Find the end of the previous tag
		for i > 0 && content[i-1] != '>' {
			i--
		}
		// Skip whitespace after the tag
		for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
			i++
		}
		return i
	}

	// Otherwise, find the start of current text node or element
	for i > 0 {
		if content[i-1] == '>' || content[i-1] == '<' {
			break
		}
		i--
	}

	return i
}

// findFragmentEnd finds the logical end of a template fragment
func (fe *FragmentExtractor) findFragmentEnd(content string, exprEnd int) int {
	i := exprEnd

	// Skip whitespace forwards
	for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i++
	}

	// If we hit an opening tag, we're done
	if i < len(content) && content[i] == '<' {
		return i
	}

	// Otherwise, continue until we hit a tag or end of meaningful content
	for i < len(content) {
		if content[i] == '<' || content[i] == '>' {
			break
		}
		i++
	}

	return i
}

// generateRandomID generates a 6-character random ID
func (fe *FragmentExtractor) generateRandomID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback to simple counter if crypto/rand fails
		return fmt.Sprintf("frag%02d", len(b))
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}

	return string(b)
}

// replaceFragmentsWithCalls replaces extracted fragments with template calls
func (fe *FragmentExtractor) replaceFragmentsWithCalls(original string, fragments []*TemplateFragment) (string, error) {
	if len(fragments) == 0 {
		return original, nil
	}

	// Sort fragments by start position (descending) to replace from end to beginning
	// This prevents position shifts from affecting later replacements
	sortedFragments := make([]*TemplateFragment, len(fragments))
	copy(sortedFragments, fragments)

	// Simple bubble sort by StartPos (descending)
	for i := 0; i < len(sortedFragments)-1; i++ {
		for j := 0; j < len(sortedFragments)-i-1; j++ {
			if sortedFragments[j].StartPos < sortedFragments[j+1].StartPos {
				sortedFragments[j], sortedFragments[j+1] = sortedFragments[j+1], sortedFragments[j]
			}
		}
	}

	result := original

	// Replace each fragment with a template call
	for _, fragment := range sortedFragments {
		templateCall := fmt.Sprintf(`{{template "%s" .}}`, fragment.ID)
		result = result[:fragment.StartPos] + templateCall + result[fragment.EndPos:]
	}

	return result, nil
}

// AddFragmentsToTemplate adds extracted fragments as named templates to a template
func (fe *FragmentExtractor) AddFragmentsToTemplate(tmpl *template.Template, fragments []*TemplateFragment) error {
	for _, fragment := range fragments {
		_, err := tmpl.New(fragment.ID).Parse(fragment.Content)
		if err != nil {
			return fmt.Errorf("failed to add fragment %s: %w", fragment.ID, err)
		}
	}
	return nil
}

// ProcessTemplateWithFragments processes a template string and returns a template with fragments
func (fe *FragmentExtractor) ProcessTemplateWithFragments(name, templateContent string) (*template.Template, []*TemplateFragment, error) {
	// Extract fragments
	fragments, modifiedContent, err := fe.ExtractFragments(templateContent)
	if err != nil {
		return nil, nil, err
	}

	// Create the main template
	tmpl, err := template.New(name).Parse(modifiedContent)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse modified template: %w", err)
	}

	// Add fragments as named templates
	err = fe.AddFragmentsToTemplate(tmpl, fragments)
	if err != nil {
		return nil, nil, err
	}

	return tmpl, fragments, nil
}
