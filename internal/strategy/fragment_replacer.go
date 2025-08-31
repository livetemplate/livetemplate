package strategy

import (
	"bytes"
	"fmt"
	"html/template"
)

// FragmentReplacementData represents a simple fragment replacement
type FragmentReplacementData struct {
	FragmentID string `json:"fragment_id"`
	HTML       string `json:"html"`
	Action     string `json:"action"` // "replace"
}

// FragmentReplacer handles complete fragment replacement for complex templates
type FragmentReplacer struct{}

// NewFragmentReplacer creates a new fragment replacer
func NewFragmentReplacer() *FragmentReplacer {
	return &FragmentReplacer{}
}

// GenerateReplacement creates a complete fragment replacement
func (fr *FragmentReplacer) GenerateReplacement(tmpl *template.Template, data interface{}, fragmentID string) (*FragmentReplacementData, error) {
	// Render the complete template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}

	return &FragmentReplacementData{
		FragmentID: fragmentID,
		HTML:       buf.String(),
		Action:     "replace",
	}, nil
}
