package kits

// KitSource represents where a kit was loaded from
type KitSource string

const (
	SourceSystem    KitSource = "system"    // Built-in, embedded in lvt binary
	SourceLocal     KitSource = "local"     // User's custom kits
	SourceCommunity KitSource = "community" // From registry (future)
)

// KitManifest represents the kit.yaml file structure
type KitManifest struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Framework   string   `yaml:"framework"` // e.g., "tailwind", "bulma", "pico", "none"
	Author      string   `yaml:"author,omitempty"`
	License     string   `yaml:"license,omitempty"`
	CDN         string   `yaml:"cdn,omitempty"`        // CDN link for CSS framework
	CustomCSS   string   `yaml:"custom_css,omitempty"` // Path to custom CSS file
	Tags        []string `yaml:"tags,omitempty"`
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

	if m.Framework == "" {
		return ErrInvalidManifest{Field: "framework", Reason: "framework is required"}
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
