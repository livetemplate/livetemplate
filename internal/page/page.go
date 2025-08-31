package page

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/livefir/livetemplate/internal/strategy"
)

// Page represents an isolated user session with stateless design
type Page struct {
	ID            string
	ApplicationID string
	TemplateHash  string
	template      *template.Template
	data          interface{}
	createdAt     time.Time
	lastAccessed  time.Time
	fragmentCache map[string]string
	treeGenerator *strategy.SimpleTreeGenerator
	config        *Config
	mu            sync.RWMutex
}

// Config defines Page configuration
type Config struct {
	MaxFragments    int // Default: 100
	MaxMemoryMB     int // Default: 10MB
	UpdateBatchSize int // Default: 20
}

// DefaultConfig returns secure default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxFragments:    100,
		MaxMemoryMB:     10,
		UpdateBatchSize: 20,
	}
}

// NewPage creates a new isolated Page instance
func NewPage(applicationID string, tmpl *template.Template, data interface{}, config *Config) (*Page, error) {
	if applicationID == "" {
		return nil, fmt.Errorf("applicationID cannot be empty")
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}
	if config == nil {
		config = DefaultConfig()
	}

	// Generate unique page ID
	pageID, err := generatePageID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate page ID: %w", err)
	}

	// Generate template hash for identification
	templateHash := generateTemplateHash(tmpl)

	// Create tree-based generator (now the only strategy)
	treeGenerator := strategy.NewSimpleTreeGenerator()

	page := &Page{
		ID:            pageID,
		ApplicationID: applicationID,
		TemplateHash:  templateHash,
		template:      tmpl,
		data:          data,
		createdAt:     time.Now(),
		lastAccessed:  time.Now(),
		fragmentCache: make(map[string]string),
		treeGenerator: treeGenerator,
		config:        config,
	}

	return page, nil
}

// Render generates the complete HTML output for the current page state
func (p *Page) Render() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var buf strings.Builder
	err := p.template.Execute(&buf, p.data)
	if err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// RenderFragments generates fragment updates for the given new data
func (p *Page) RenderFragments(ctx context.Context, newData interface{}) ([]*Fragment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update last accessed time
	p.lastAccessed = time.Now()

	// Generate fragments using tree-based strategy (now the only strategy)
	oldData := p.data

	// Extract template source for the tree generator
	templateSource, err := p.extractTemplateSource()
	if err != nil {
		return nil, fmt.Errorf("failed to extract template source: %w", err)
	}

	// Generate fragment ID based on template and data
	fragmentID := p.generateFragmentID(templateSource, oldData, newData)

	// Use tree generator to create fragment data
	startTime := time.Now()
	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("tree fragment generation failed: %w", err)
	}
	generationTime := time.Since(startTime)

	// Create fragment from tree result
	fragment := &Fragment{
		ID:       fragmentID,
		Strategy: "tree_based",
		Action:   "update_tree",
		Data:     treeResult,
		Metadata: &Metadata{
			GenerationTime:   generationTime,
			OriginalSize:     0,   // TODO: Calculate if needed for metrics
			CompressedSize:   0,   // TODO: Calculate if needed for metrics
			CompressionRatio: 0,   // TODO: Calculate if needed for metrics
			Strategy:         1,   // Tree-based strategy
			Confidence:       1.0, // Always confident with tree-based
			FallbackUsed:     false,
		},
	}

	fragments := []*Fragment{fragment}

	// Update current data state
	p.data = newData

	return fragments, nil
}

// SetData updates the page data state
func (p *Page) SetData(data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data = data
	p.lastAccessed = time.Now()
	return nil
}

// GetData returns the current page data
func (p *Page) GetData() interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data
}

// GetTemplate returns the page template
func (p *Page) GetTemplate() *template.Template {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.template
}

// UpdateLastAccessed updates the last accessed timestamp
func (p *Page) UpdateLastAccessed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAccessed = time.Now()
}

// IsExpired checks if the page has exceeded the TTL
func (p *Page) IsExpired(ttl time.Duration) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.lastAccessed) > ttl
}

// GetMemoryUsage estimates memory usage in bytes
func (p *Page) GetMemoryUsage() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Basic estimation - in production this would be more sophisticated
	var size int64

	// Template name and structure (rough estimate)
	if p.template != nil {
		size += int64(len(p.template.Name()) * 2) // Unicode estimation
	}

	// Data size (rough JSON serialization estimate)
	size += estimateDataSize(p.data)

	// Fragment cache
	for key, value := range p.fragmentCache {
		size += int64(len(key) + len(value))
	}

	// Fixed overhead for struct fields
	size += 200 // Rough estimate for IDs, timestamps, etc.

	return size
}

// GetMetrics returns page-specific metrics
func (p *Page) GetMetrics() Metrics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get basic metrics - unified generator doesn't expose detailed metrics yet
	// In future versions, we could add metrics to tree generation

	return Metrics{
		PageID:                p.ID,
		ApplicationID:         p.ApplicationID,
		CreatedAt:             p.createdAt,
		LastAccessed:          p.lastAccessed,
		Age:                   time.Since(p.createdAt),
		IdleTime:              time.Since(p.lastAccessed),
		MemoryUsage:           p.GetMemoryUsage(),
		FragmentCacheSize:     len(p.fragmentCache),
		TotalGenerations:      0, // Placeholder - unified generator doesn't track this yet
		SuccessfulGenerations: 0, // Placeholder - unified generator doesn't track this yet
		FailedGenerations:     0, // Placeholder - unified generator doesn't track this yet
		AverageGenerationTime: 0, // Placeholder - unified generator doesn't track this yet
		ErrorRate:             0, // Placeholder - unified generator doesn't track this yet
	}
}

// Close releases page resources
func (p *Page) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear caches
	p.fragmentCache = make(map[string]string)

	// Reset data reference
	p.data = nil

	return nil
}

// Fragment represents a generated update fragment
type Fragment struct {
	ID       string      `json:"id"`
	Strategy string      `json:"strategy"`
	Action   string      `json:"action"`
	Data     interface{} `json:"data"`
	Metadata *Metadata   `json:"metadata,omitempty"`
}

// Metadata contains performance information for fragments
type Metadata struct {
	GenerationTime   time.Duration `json:"generation_time"`
	OriginalSize     int           `json:"original_size"`
	CompressedSize   int           `json:"compressed_size"`
	CompressionRatio float64       `json:"compression_ratio"`
	Strategy         int           `json:"strategy_number"`
	Confidence       float64       `json:"confidence"`
	FallbackUsed     bool          `json:"fallback_used"`
}

// Metrics contains page performance data
type Metrics struct {
	PageID                string        `json:"page_id"`
	ApplicationID         string        `json:"application_id"`
	CreatedAt             time.Time     `json:"created_at"`
	LastAccessed          time.Time     `json:"last_accessed"`
	Age                   time.Duration `json:"age"`
	IdleTime              time.Duration `json:"idle_time"`
	MemoryUsage           int64         `json:"memory_usage"`
	FragmentCacheSize     int           `json:"fragment_cache_size"`
	TotalGenerations      int64         `json:"total_generations"`
	SuccessfulGenerations int64         `json:"successful_generations"`
	FailedGenerations     int64         `json:"failed_generations"`
	AverageGenerationTime time.Duration `json:"average_generation_time"`
	ErrorRate             float64       `json:"error_rate"`
}

// Helper functions

// generatePageID creates a cryptographically secure page ID
func generatePageID() (string, error) {
	bytes := make([]byte, 16) // 128-bit ID
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateTemplateHash creates a deterministic hash of the template
func generateTemplateHash(tmpl *template.Template) string {
	// Simple hash based on template name and defined templates
	// In production, this could include template content hash
	hasher := sha256.New()
	hasher.Write([]byte(tmpl.Name()))

	// Include associated template names for consistency
	for _, t := range tmpl.Templates() {
		hasher.Write([]byte(t.Name()))
	}

	return hex.EncodeToString(hasher.Sum(nil))[:16] // First 16 chars
}

// estimateDataSize provides rough estimate of data memory usage
func estimateDataSize(data interface{}) int64 {
	if data == nil {
		return 0
	}

	// Very rough estimation based on data type
	// In production, this would use reflection or serialization size
	switch v := data.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case map[string]interface{}:
		size := int64(0)
		for key, value := range v {
			size += int64(len(key))
			size += estimateDataSize(value)
		}
		return size
	case []interface{}:
		size := int64(0)
		for _, value := range v {
			size += estimateDataSize(value)
		}
		return size
	default:
		// Default estimation for unknown types
		return 100
	}
}

// extractTemplateSource extracts the source code from a template for analysis
func (p *Page) extractTemplateSource() (string, error) {
	// For now, we use a placeholder approach since Go templates don't expose their source
	// In a real implementation, we would need to store the original template source
	// or use template introspection techniques

	// Simple heuristic: use template name as a proxy for template content
	// This is a limitation of the current Go template system
	if p.template == nil {
		return "", fmt.Errorf("template is nil")
	}

	// Use template name as identifier - in production, we'd store actual source
	templateName := p.template.Name()
	if templateName == "" {
		templateName = "unnamed_template"
	}

	// Return a placeholder that represents this template
	return fmt.Sprintf("{{/* Template: %s */}}", templateName), nil
}

// generateFragmentID creates a deterministic fragment ID
func (p *Page) generateFragmentID(templateSource string, oldData, newData interface{}) string {
	// Use template hash and data signature to create deterministic ID
	dataHash := fmt.Sprintf("%v-%v", oldData, newData)
	combined := p.TemplateHash + "|" + templateSource + "|" + dataHash

	// Simple hash for fragment ID
	hash := fmt.Sprintf("%x", []byte(combined))
	if len(hash) > 16 {
		hash = hash[:16]
	}

	return fmt.Sprintf("fragment_%s", hash)
}
