package kits

// KitSource represents where a kit was loaded from
type KitSource string

const (
	SourceSystem    KitSource = "system"    // Built-in, embedded in lvt binary
	SourceLocal     KitSource = "local"     // User's custom kits
	SourceCommunity KitSource = "community" // From registry (future)
)

// KitTemplates defines which generator templates are included in a kit
type KitTemplates struct {
	Resource bool `yaml:"resource"` // Resource generator templates
	View     bool `yaml:"view"`     // View generator templates
	App      bool `yaml:"app"`      // App generator templates
}

// KitManifest represents the kit.yaml file structure
type KitManifest struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version"`
	CSSFramework string       `yaml:"css_framework"` // CSS framework used by this kit (tailwind, bulma, pico, none)
	Description  string       `yaml:"description"`
	Framework    string       `yaml:"framework,omitempty"` // Optional: for legacy CSS-specific kits
	Author       string       `yaml:"author,omitempty"`
	License      string       `yaml:"license,omitempty"`
	CDN          string       `yaml:"cdn,omitempty"`        // CDN link for CSS framework
	CustomCSS    string       `yaml:"custom_css,omitempty"` // Path to custom CSS file
	Tags         []string     `yaml:"tags,omitempty"`
	Components   []string     `yaml:"components,omitempty"` // List of component template names
	Templates    KitTemplates `yaml:"templates,omitempty"`  // Generator templates included
}

// KitInfo represents a loaded kit with its metadata and helpers
type KitInfo struct {
	// Manifest data
	Manifest KitManifest

	// Runtime data
	Source  KitSource  // Where this kit was loaded from
	Path    string     // Absolute path to kit directory
	Helpers CSSHelpers // CSS helper implementation
}

// KitSearchOptions defines options for searching/filtering kits
type KitSearchOptions struct {
	Source KitSource // Filter by source (empty = all)
	Query  string    // Search query for name/description/tags
}

// Validate checks if the kit manifest is valid
func (m *KitManifest) Validate() error {
	if m.Name == "" {
		return ErrInvalidManifest{Field: "name", Reason: "name is required"}
	}

	if m.Version == "" {
		return ErrInvalidManifest{Field: "version", Reason: "version is required"}
	}

	if m.Description == "" {
		return ErrInvalidManifest{Field: "description", Reason: "description is required"}
	}

	// Support both css_framework (new) and framework (legacy) for backwards compatibility
	framework := m.CSSFramework
	if framework == "" {
		framework = m.Framework
	}

	if framework == "" {
		return ErrInvalidManifest{Field: "css_framework/framework", Reason: "either css_framework or framework is required"}
	}

	// Note: We don't validate the framework value here to allow custom frameworks for testing/development
	// The loader will use system helpers for standard frameworks (tailwind, bulma, pico, none)
	// and return nil helpers for custom frameworks

	// Sync the fields for consistency (prefer CSSFramework)
	if m.CSSFramework == "" && m.Framework != "" {
		m.CSSFramework = m.Framework
	}

	return nil
}

// MatchesQuery checks if the kit matches a search query
func (m *KitManifest) MatchesQuery(query string) bool {
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

	// Search in framework
	if contains(m.Framework, query) {
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

// Implement Kit interface for KitInfo
func (k *KitInfo) Name() string {
	return k.Manifest.Name
}

func (k *KitInfo) Version() string {
	return k.Manifest.Version
}

func (k *KitInfo) GetHelpers() CSSHelpers {
	return k.Helpers
}

// SetHelpersForFramework sets CSS helpers based on framework name
// Used for CSS-agnostic kits that need helpers loaded dynamically
func (k *KitInfo) SetHelpersForFramework(framework string) error {
	if framework == "" {
		return ErrInvalidManifest{Field: "framework", Reason: "framework cannot be empty"}
	}

	// Already has helpers
	if k.Helpers != nil {
		return nil
	}

	// Load helpers based on framework
	helpers, err := LoadHelpersForFramework(framework)
	if err != nil {
		return err
	}

	k.Helpers = helpers
	return nil
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
