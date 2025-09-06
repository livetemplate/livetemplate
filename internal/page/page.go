package page

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"text/template/parse"
	"time"
	"unsafe"

	"github.com/livefir/livetemplate/internal/strategy"
)

// Page represents an isolated user session with stateless design
type Page struct {
	ID             string
	ApplicationID  string
	TemplateHash   string
	template       *template.Template
	templateSource string // Store original template source for tree analysis
	data           interface{}
	createdAt      time.Time
	lastAccessed   time.Time
	fragmentCache  map[string]string
	treeGenerator  *strategy.SimpleTreeGenerator
	config         *Config
	regions        []TemplateRegion // Cache template regions to ensure consistent IDs
	mu             sync.RWMutex
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

	// CRITICAL: Clone the template to prevent contamination from shared usage
	// Each page gets its own isolated template copy
	clonedTmpl, err := tmpl.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone template: %w", err)
	}

	// Generate unique page ID
	pageID, err := generatePageID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate page ID: %w", err)
	}

	// Generate template hash for identification (use original for consistency)
	templateHash := generateTemplateHash(tmpl)

	// Create tree-based generator (now the only strategy)
	treeGenerator := strategy.NewSimpleTreeGenerator()

	page := &Page{
		ID:            pageID,
		ApplicationID: applicationID,
		TemplateHash:  templateHash,
		template:      clonedTmpl, // Use the cloned template
		data:          data,
		createdAt:     time.Now(),
		lastAccessed:  time.Now(),
		fragmentCache: make(map[string]string),
		treeGenerator: treeGenerator,
		config:        config,
	}

	// Extract and store original template source at creation time
	// This ensures we always have the clean, uncontaminated source
	// Extract from cloned template before any execution contaminates it
	templateSource, err := page.extractTemplateSourceFromTemplate(clonedTmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to extract template source: %w", err)
	}
	page.templateSource = templateSource

	// Regions will be detected and cached during first Render() call

	return page, nil
}

// Render generates the complete HTML output for the current page state with fragment annotations
func (p *Page) Render() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var buf strings.Builder
	err := p.template.Execute(&buf, p.data)
	if err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	html := buf.String()

	// Detect regions and cache them for fragment generation
	regions, err := p.detectTemplateRegions()
	if err != nil || len(regions) == 0 {
		// For simple templates or when region detection fails, use legacy approach
		return p.annotateLegacyHTML(html)
	}

	// Filter regions to only include those that can be properly annotated
	// (skip regions with nested HTML content that regex can't handle)
	var annotatedRegions []TemplateRegion
	annotatedHTML := html

	for _, region := range regions {
		// Try to annotate this region
		newHTML := p.annotateDynamicElement(annotatedHTML, region)
		// Check if annotation was successful (HTML changed)
		if newHTML != annotatedHTML {
			annotatedRegions = append(annotatedRegions, region)
			annotatedHTML = newHTML
		}
	}

	// Cache only the successfully annotated regions for fragment generation
	p.regions = annotatedRegions

	return annotatedHTML, nil
}

// FragmentOption configures fragment generation behavior
type FragmentOption func(*FragmentConfig)

// FragmentConfig controls fragment generation options
type FragmentConfig struct {
	IncludeMetadata bool // Whether to include performance metadata (default: false)
}

// WithMetadata enables metadata collection in fragments
func WithMetadata() FragmentOption {
	return func(config *FragmentConfig) {
		config.IncludeMetadata = true
	}
}

// RenderFragments generates fragment updates for the given new data
func (p *Page) RenderFragments(ctx context.Context, newData interface{}, opts ...FragmentOption) ([]*Fragment, error) {
	// Apply options
	config := &FragmentConfig{
		IncludeMetadata: false, // Default: no metadata for minimal payload
	}
	for _, opt := range opts {
		opt(config)
	}

	return p.renderFragmentsWithConfig(ctx, newData, config)
}

// renderFragmentsWithConfig generates fragments with the specified configuration
func (p *Page) renderFragmentsWithConfig(ctx context.Context, newData interface{}, config *FragmentConfig) ([]*Fragment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update last accessed time
	p.lastAccessed = time.Now()

	// Use cached template regions, detect them if not already cached
	regions := p.regions
	if len(regions) == 0 {
		// Try to detect regions if not already done
		detectedRegions, err := p.detectTemplateRegions()
		if err == nil && len(detectedRegions) > 0 {
			// Cache the detected regions for future use
			p.regions = detectedRegions
			regions = detectedRegions
		} else {
			// Fallback to original full-template approach for simple templates
			return p.renderFragmentsLegacyWithConfig(newData, config)
		}
	}

	// Generate fragments for each dynamic region
	var fragments []*Fragment
	var startTime time.Time
	if config.IncludeMetadata {
		startTime = time.Now()
	}

	for _, region := range regions {
		fragment, err := p.generateRegionFragmentWithConfig(region, newData, config)
		if err != nil {
			continue
		}
		fragments = append(fragments, fragment)
	}

	if len(fragments) == 0 {
		// If no region fragments were generated, fall back to legacy approach
		return p.renderFragmentsLegacyWithConfig(newData, config)
	}

	// Update metadata for all fragments if requested
	if config.IncludeMetadata {
		generationTime := time.Since(startTime)
		for _, frag := range fragments {
			if frag.Metadata != nil {
				frag.Metadata.GenerationTime = generationTime
			}
		}
	}

	// Update current data state
	p.data = newData

	return fragments, nil
}

// renderFragmentsLegacyWithConfig provides the original full-template fragment generation with config
func (p *Page) renderFragmentsLegacyWithConfig(newData interface{}, config *FragmentConfig) ([]*Fragment, error) {
	// Original implementation for simple templates
	oldData := p.data

	// Extract template source for the tree generator
	templateSource, err := p.extractTemplateSource()
	if err != nil {
		return nil, fmt.Errorf("failed to extract template source: %w", err)
	}

	// Generate fragment ID based on template and data
	fragmentID := p.generateFragmentID(templateSource, oldData, newData)

	// Use tree generator to create fragment data
	var startTime time.Time
	if config.IncludeMetadata {
		startTime = time.Now()
	}

	treeResult, err := p.treeGenerator.GenerateFromTemplateSource(templateSource, oldData, newData, fragmentID)
	if err != nil {
		return nil, fmt.Errorf("tree fragment generation failed: %w", err)
	}

	// Create fragment from tree result
	fragment := &Fragment{
		ID:       fragmentID,
		Data:     treeResult,
		Metadata: nil, // Will be set conditionally below
	}

	// Add metadata only if requested
	if config.IncludeMetadata {
		generationTime := time.Since(startTime)
		fragment.Metadata = &Metadata{
			GenerationTime:   generationTime,
			OriginalSize:     0,   // TODO: Calculate if needed for metrics
			CompressedSize:   0,   // TODO: Calculate if needed for metrics
			CompressionRatio: 0,   // TODO: Calculate if needed for metrics
			Strategy:         1,   // Tree-based strategy
			Confidence:       1.0, // Always confident with tree-based
			FallbackUsed:     false,
		}
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

// GetID returns the page ID
func (p *Page) GetID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ID
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
	if p.template == nil {
		return "", fmt.Errorf("template is nil")
	}

	// If we have stored template source, use it
	if p.templateSource != "" {
		return p.templateSource, nil
	}

	// Extract template source from the template's parse tree
	templateSource, err := p.extractTemplateSourceFromTemplate(p.template)
	if err != nil {
		return "", fmt.Errorf("failed to extract template source: %w", err)
	}

	if templateSource == "" {
		return "", fmt.Errorf("extracted template source is empty")
	}

	// Cache the extracted source
	p.templateSource = templateSource
	return templateSource, nil
}

// extractTemplateSourceFromTemplate attempts to extract original template source using reflection
func (p *Page) extractTemplateSourceFromTemplate(tmpl *template.Template) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("template is nil")
	}

	// Get the main template - use the template itself if named, or find the main one
	mainTemplate := tmpl
	if tmpl.Name() == "" {
		// If no name, try to find the first template
		templates := tmpl.Templates()
		if len(templates) == 0 {
			return "", fmt.Errorf("no templates found")
		}
		mainTemplate = templates[0]
	}

	// Try to access the html/template's parse tree directly
	val := reflect.ValueOf(mainTemplate)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// html/template has a Tree field directly accessible
	treeField := val.FieldByName("Tree")
	if !treeField.IsValid() {
		return "", fmt.Errorf("cannot access template Tree field")
	}

	// Use unsafe to access private field if necessary
	if !treeField.CanInterface() {
		treeField = reflect.NewAt(treeField.Type(), unsafe.Pointer(treeField.UnsafeAddr())).Elem()
	}

	if treeField.IsNil() {
		return "", fmt.Errorf("template parse tree is nil")
	}

	parseTree, ok := treeField.Interface().(*parse.Tree)
	if !ok {
		return "", fmt.Errorf("cannot cast to parse tree")
	}

	// Reconstruct template source from parse tree
	reconstructed := p.reconstructTemplateFromParseTree(parseTree)
	if reconstructed == "" {
		return "", fmt.Errorf("failed to reconstruct template source")
	}

	return reconstructed, nil
}

// reconstructTemplateFromParseTree reconstructs template source from parse tree
func (p *Page) reconstructTemplateFromParseTree(tree *parse.Tree) string {
	if tree == nil || tree.Root == nil {
		return ""
	}

	var result strings.Builder
	p.reconstructNodeRecursive(tree.Root, &result)
	return result.String()
}

// reconstructNodeRecursive recursively reconstructs template source from parse nodes
func (p *Page) reconstructNodeRecursive(node parse.Node, result *strings.Builder) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.ListNode:
		if n != nil && n.Nodes != nil {
			for _, child := range n.Nodes {
				p.reconstructNodeRecursive(child, result)
			}
		}
	case *parse.TextNode:
		if n != nil && len(n.Text) > 0 {
			result.Write(n.Text)
		}
	case *parse.ActionNode:
		if n != nil {
			result.WriteString("{{")
			if n.Pipe != nil {
				p.reconstructPipeNode(n.Pipe, result)
			}
			result.WriteString("}}")
		}
	case *parse.IfNode:
		if n != nil {
			result.WriteString("{{if ")
			if n.Pipe != nil {
				p.reconstructPipeNode(n.Pipe, result)
			}
			result.WriteString("}}")
			if n.List != nil {
				p.reconstructNodeRecursive(n.List, result)
			}
			if n.ElseList != nil {
				result.WriteString("{{else}}")
				p.reconstructNodeRecursive(n.ElseList, result)
			}
			result.WriteString("{{end}}")
		}
	case *parse.RangeNode:
		if n != nil {
			result.WriteString("{{range ")
			if n.Pipe != nil {
				p.reconstructPipeNode(n.Pipe, result)
			}
			result.WriteString("}}")
			if n.List != nil {
				p.reconstructNodeRecursive(n.List, result)
			}
			if n.ElseList != nil {
				result.WriteString("{{else}}")
				p.reconstructNodeRecursive(n.ElseList, result)
			}
			result.WriteString("{{end}}")
		}
	case *parse.WithNode:
		if n != nil {
			result.WriteString("{{with ")
			if n.Pipe != nil {
				p.reconstructPipeNode(n.Pipe, result)
			}
			result.WriteString("}}")
			if n.List != nil {
				p.reconstructNodeRecursive(n.List, result)
			}
			if n.ElseList != nil {
				result.WriteString("{{else}}")
				p.reconstructNodeRecursive(n.ElseList, result)
			}
			result.WriteString("{{end}}")
		}
	case *parse.TemplateNode:
		if n != nil {
			result.WriteString("{{template ")
			result.WriteString(fmt.Sprintf(`"%s"`, n.Name))
			if n.Pipe != nil {
				result.WriteString(" ")
				p.reconstructPipeNode(n.Pipe, result)
			}
			result.WriteString("}}")
		}
	default:
		// For other node types, use string representation as fallback
		if node != nil {
			result.WriteString(node.String())
		}
	}
}

// reconstructPipeNode reconstructs a pipeline from parse tree
func (p *Page) reconstructPipeNode(pipe *parse.PipeNode, result *strings.Builder) {
	if pipe == nil || len(pipe.Cmds) == 0 {
		return
	}

	for i, cmd := range pipe.Cmds {
		if i > 0 {
			result.WriteString(" | ")
		}
		for j, arg := range cmd.Args {
			if j > 0 {
				result.WriteString(" ")
			}
			result.WriteString(arg.String())
		}
	}
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

// SetTemplateSource sets the template source for the page
func (p *Page) SetTemplateSource(templateSource string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templateSource = templateSource
}

// annotateDynamicElement adds lvt-id attribute to a dynamic HTML element
func (p *Page) annotateDynamicElement(html string, region TemplateRegion) string {
	// The challenge: we need to find the rendered element (with actual values)
	// not the template source. For example:
	// Template: <div class="{{.class}}">Hello {{.Counter}}</div>
	// Rendered: <div class="item">Hello 4</div>

	// Check if the original attributes contain templates
	templatePattern := regexp.MustCompile(`\{\{[^}]+\}\}`)
	hasTemplateInAttributes := templatePattern.MatchString(region.OriginalAttrs)

	var pattern string
	var replacement string

	if hasTemplateInAttributes {
		// For attributes with templates, match any content within the same tag type
		// Pattern: <tag [any attributes]>content</tag> with capture groups for attributes and content
		pattern = fmt.Sprintf(`<%s([^>]*)>([^<]*)</%s>`,
			regexp.QuoteMeta(region.ElementTag),
			regexp.QuoteMeta(region.ElementTag))

		// Replacement: <tag [existing attributes] lvt-id="ID">content</tag>
		replacement = fmt.Sprintf(`<%s $1 lvt-id="%s">$2</%s>`,
			region.ElementTag,
			region.ID,
			region.ElementTag)
	} else {
		// For elements without template attributes, match exact attributes
		pattern = fmt.Sprintf(`<%s%s>([^<]*)</%s>`,
			regexp.QuoteMeta(region.ElementTag),
			regexp.QuoteMeta(region.OriginalAttrs),
			regexp.QuoteMeta(region.ElementTag))

		replacement = fmt.Sprintf(`<%s%s lvt-id="%s">$1</%s>`,
			region.ElementTag,
			region.OriginalAttrs,
			region.ID,
			region.ElementTag)
	}

	regex := regexp.MustCompile(pattern)

	// Check if this element matches our pattern
	matches := regex.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		// No matches found, return original HTML
		return html
	}

	// Replace all occurrences (usually just one per region)
	return regex.ReplaceAllString(html, replacement)
}

// annotateLegacyHTML provides legacy annotation for simple templates
func (p *Page) annotateLegacyHTML(html string) (string, error) {
	// Generate a fragment ID for the entire page content
	templateSource, err := p.extractTemplateSource()
	if err != nil {
		// If we can't extract template source, return HTML without annotations
		// This maintains backward compatibility
		return html, nil
	}

	fragmentID := p.generateFragmentID(templateSource, nil, p.data)

	// Find the main container and add fragment ID
	// Look for common container patterns
	annotatedHTML := html
	containerPatterns := []string{
		`<div class="container">`,
		`<body>`,
		`<div class="app">`,
		`<main>`,
	}

	for _, pattern := range containerPatterns {
		if strings.Contains(html, pattern) {
			// Insert data-fragment-id attribute
			replacement := strings.Replace(pattern, ">", fmt.Sprintf(` data-fragment-id="%s">`, fragmentID), 1)
			annotatedHTML = strings.Replace(html, pattern, replacement, 1)
			break
		}
	}

	return annotatedHTML, nil
}
