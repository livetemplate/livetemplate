package components

import (
	"text/template"
)

// ComponentSource represents where a component was loaded from
type ComponentSource string

const (
	SourceSystem    ComponentSource = "system"    // Built-in, embedded in lvt binary
	SourceLocal     ComponentSource = "local"     // User's custom components
	SourceCommunity ComponentSource = "community" // From registry (future)
)

// ComponentCategory represents the category of a component
type ComponentCategory string

const (
	CategoryBase       ComponentCategory = "base"
	CategoryForm       ComponentCategory = "form"
	CategoryLayout     ComponentCategory = "layout"
	CategoryData       ComponentCategory = "data"
	CategoryNavigation ComponentCategory = "navigation"
	CategoryTable      ComponentCategory = "table"
	CategoryToolbar    ComponentCategory = "toolbar"
	CategoryDetail     ComponentCategory = "detail"
)

// InputType represents the type of an input parameter
type InputType string

const (
	InputTypeString InputType = "string"
	InputTypeInt    InputType = "int"
	InputTypeBool   InputType = "bool"
	InputTypeEnum   InputType = "enum"
	InputTypeObject InputType = "object"
	InputTypeArray  InputType = "array"
)

// ComponentInput defines an input parameter for a component
type ComponentInput struct {
	Name        string    `yaml:"name"`
	Type        InputType `yaml:"type"`
	Required    bool      `yaml:"required"`
	Default     string    `yaml:"default,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Enum        []string  `yaml:"enum,omitempty"` // Valid values for enum type
}

// ComponentManifest represents the component.yaml file structure
type ComponentManifest struct {
	Name         string            `yaml:"name"`
	Version      string            `yaml:"version"`
	Description  string            `yaml:"description"`
	Category     ComponentCategory `yaml:"category"`
	Author       string            `yaml:"author,omitempty"`
	License      string            `yaml:"license,omitempty"`
	Inputs       []ComponentInput  `yaml:"inputs,omitempty"`
	Dependencies []string          `yaml:"dependencies,omitempty"`
	Templates    []string          `yaml:"templates"`
	Tags         []string          `yaml:"tags,omitempty"`
}

// Component represents a loaded component with its metadata and templates
type Component struct {
	// Manifest data
	Manifest ComponentManifest

	// Runtime data
	Source    ComponentSource               // Where this component was loaded from
	Path      string                        // Absolute path to component directory
	Templates map[string]*template.Template // Parsed templates by filename
}

// ComponentSearchOptions defines options for searching/filtering components
type ComponentSearchOptions struct {
	Source   ComponentSource   // Filter by source (empty = all)
	Category ComponentCategory // Filter by category (empty = all)
	Query    string            // Search query for name/description/tags
}

// Validate checks if the component manifest is valid
func (m *ComponentManifest) Validate() error {
	if m.Name == "" {
		return ErrInvalidManifest{Field: "name", Reason: "name is required"}
	}

	if m.Version == "" {
		return ErrInvalidManifest{Field: "version", Reason: "version is required"}
	}

	if m.Description == "" {
		return ErrInvalidManifest{Field: "description", Reason: "description is required"}
	}

	// Validate category
	validCategories := map[ComponentCategory]bool{
		CategoryBase:       true,
		CategoryForm:       true,
		CategoryLayout:     true,
		CategoryData:       true,
		CategoryNavigation: true,
		CategoryTable:      true,
		CategoryToolbar:    true,
		CategoryDetail:     true,
	}
	if !validCategories[m.Category] {
		return ErrInvalidManifest{
			Field:  "category",
			Reason: "category must be one of: base, form, layout, data, navigation, table, toolbar, detail",
		}
	}

	// At least one template required
	if len(m.Templates) == 0 {
		return ErrInvalidManifest{Field: "templates", Reason: "at least one template is required"}
	}

	// Validate inputs
	for i, input := range m.Inputs {
		if input.Name == "" {
			return ErrInvalidManifest{
				Field:  "inputs",
				Reason: "input name is required",
				Index:  &i,
			}
		}

		validTypes := map[InputType]bool{
			InputTypeString: true,
			InputTypeInt:    true,
			InputTypeBool:   true,
			InputTypeEnum:   true,
			InputTypeObject: true,
			InputTypeArray:  true,
		}
		if !validTypes[input.Type] {
			return ErrInvalidManifest{
				Field:  "inputs",
				Reason: "input type must be one of: string, int, bool, enum, object, array",
				Index:  &i,
			}
		}

		// Enum type must have enum values
		if input.Type == InputTypeEnum && len(input.Enum) == 0 {
			return ErrInvalidManifest{
				Field:  "inputs",
				Reason: "enum type must specify enum values",
				Index:  &i,
			}
		}
	}

	return nil
}

// GetInput retrieves an input definition by name
func (m *ComponentManifest) GetInput(name string) *ComponentInput {
	for i := range m.Inputs {
		if m.Inputs[i].Name == name {
			return &m.Inputs[i]
		}
	}
	return nil
}

// HasDependency checks if the component depends on another component
func (m *ComponentManifest) HasDependency(name string) bool {
	for _, dep := range m.Dependencies {
		if dep == name {
			return true
		}
	}
	return false
}

// MatchesQuery checks if the component matches a search query
func (m *ComponentManifest) MatchesQuery(query string) bool {
	if query == "" {
		return true
	}

	// Search in name
	if contains(m.Name, query) {
		return true
	}

	// Search in description
	if contains(m.Description, query) {
		return true
	}

	// Search in tags
	for _, tag := range m.Tags {
		if contains(tag, query) {
			return true
		}
	}

	return false
}

// contains is a case-insensitive substring check
func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || stringContains(s, substr))
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
