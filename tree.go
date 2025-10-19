package livetemplate

import (
	"crypto/md5"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// minifyHTMLWhitespace removes unnecessary whitespace from HTML content
// while preserving space in important contexts
func ParseTemplateToTreeForTesting(templateStr string, data interface{}, keyGen *KeyGenerator) (TreeNode, error) {
	return parseTemplateToTree(templateStr, data, keyGen)
}

// TreeNode represents the tree-based static/dynamic structure
type TreeNode map[string]interface{}

// calculateFingerprint calculates a 64-bit fingerprint (MD5 hash) for a tree's statics and dynamics
// This allows detecting when a subtree has changed, similar to LiveView's optimization #2
func calculateFingerprint(tree TreeNode) string {
	// Create a canonical representation of the tree for hashing
	// Include both statics (template structure) and dynamics (data values)
	hasher := md5.New()

	// Add statics to hash (template structure)
	if statics, exists := tree["s"]; exists {
		if staticsArray, ok := statics.([]string); ok {
			staticsJSON, _ := json.Marshal(staticsArray)
			hasher.Write(staticsJSON)
		}
	}

	// Add dynamics to hash in sorted order for consistency
	var keys []string
	for k := range tree {
		if k != "s" && k != "f" { // Skip statics and fingerprint itself
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		num1, err1 := strconv.Atoi(keys[i])
		num2, err2 := strconv.Atoi(keys[j])
		if err1 == nil && err2 == nil {
			return num1 < num2
		}
		return keys[i] < keys[j]
	})

	// Add dynamic values to hash
	for _, k := range keys {
		value := tree[k]
		valueJSON, _ := json.Marshal(value)
		hasher.Write([]byte(k))
		hasher.Write(valueJSON)
	}

	// Return first 16 characters of hex (64 bits)
	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) >= 16 {
		return fullHash[:16]
	}
	return fullHash
}

// addFingerprintToTree adds the fingerprint to the tree for client-side tracking
// NOTE: This should be internal-only for conditional branch detection
func addFingerprintToTree(tree TreeNode) TreeNode {
	if len(tree) == 0 {
		return tree // Don't add fingerprint to empty trees
	}

	// For now, don't expose fingerprint to clients - keep it internal
	// fingerprint := calculateFingerprint(tree)
	// tree["f"] = fingerprint
	return tree
}

// generateRandomID generates a random ID for the wrapper div
func generateRandomID() string {
	b := make([]byte, 8)
	_, _ = cryptorand.Read(b)
	return "lvt-" + hex.EncodeToString(b)
}

// injectWrapperDiv injects a wrapper div around body content with the specified ID
// Excludes <script> tags from the wrapper to prevent them from being part of the dynamic content
func injectWrapperDiv(htmlDoc string, wrapperID string, loadingDisabled bool) string {
	// Find the body opening tag and extract the content between <body> and </body>
	bodyStart := strings.Index(htmlDoc, "<body")
	if bodyStart == -1 {
		// No body tag found, return as-is
		return htmlDoc
	}

	// Find the end of the body opening tag
	bodyTagEnd := strings.Index(htmlDoc[bodyStart:], ">")
	if bodyTagEnd == -1 {
		return htmlDoc
	}
	bodyTagEnd += bodyStart + 1

	// Find the closing body tag
	bodyEnd := strings.LastIndex(htmlDoc, "</body>")
	if bodyEnd == -1 {
		return htmlDoc
	}

	// Extract the body content
	bodyContent := htmlDoc[bodyTagEnd:bodyEnd]

	// Find the first <script tag to exclude scripts from the wrapper
	scriptStart := strings.Index(bodyContent, "<script")
	var contentToWrap, scriptsSection string
	if scriptStart != -1 {
		// Split content: wrap everything before first script, leave scripts outside
		contentToWrap = bodyContent[:scriptStart]
		scriptsSection = bodyContent[scriptStart:]
	} else {
		// No scripts found, wrap entire body content
		contentToWrap = bodyContent
		scriptsSection = ""
	}

	// Add loading attribute if not disabled
	loadingAttr := ""
	if !loadingDisabled {
		loadingAttr = ` data-lvt-loading="true"`
	}

	// Create the wrapper div with the specified ID and optional loading attribute
	wrappedContent := fmt.Sprintf(`<div data-lvt-id="%s"%s>%s</div>%s`, wrapperID, loadingAttr, contentToWrap, scriptsSection)

	// Reconstruct the HTML with the wrapper
	result := htmlDoc[:bodyTagEnd] + wrappedContent + htmlDoc[bodyEnd:]

	return result
}

// extractTemplateBodyContent extracts only the body content from a full HTML template
func extractTemplateBodyContent(templateStr string) string {
	// Find the body content between <body> and </body> tags
	bodyStart := strings.Index(templateStr, "<body>")
	if bodyStart == -1 {
		// No body tag found, return the template as-is
		return templateStr
	}

	bodyStart += len("<body>")
	bodyEnd := strings.LastIndex(templateStr, "</body>")
	if bodyEnd == -1 {
		// No closing body tag found, return from body start to end
		return strings.TrimSpace(templateStr[bodyStart:])
	}

	return strings.TrimSpace(templateStr[bodyStart:bodyEnd])
}

// extractTemplateContent extracts template content using wrapper ID with proper HTML parsing
func extractTemplateContent(input string, wrapperID string) string {
	if wrapperID == "" {
		// For standalone templates without wrapper, return as-is
		return input
	}

	// Parse HTML document
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		// If parsing fails, fallback to returning input as-is
		return input
	}

	// Find the div with the matching data-lvt-id
	wrapperDiv := findElementByDataLvtID(doc, wrapperID)
	if wrapperDiv == nil {
		// If wrapper not found, return the input as-is (shouldn't happen with proper injection)
		return input
	}

	// Extract content from the wrapper div
	var result strings.Builder
	for child := wrapperDiv.FirstChild; child != nil; child = child.NextSibling {
		renderNode(&result, child)
	}

	return result.String()
}

// findElementByDataLvtID recursively searches for an element with the given data-lvt-id
func findElementByDataLvtID(n *html.Node, targetID string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "data-lvt-id" && attr.Val == targetID {
				return n
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findElementByDataLvtID(child, targetID); found != nil {
			return found
		}
	}

	return nil
}

// renderNode recursively renders an HTML node and its children to a string builder
func renderNode(w *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.WriteString(n.Data)
	case html.ElementNode:
		w.WriteString("<")
		w.WriteString(n.Data)
		for _, attr := range n.Attr {
			w.WriteString(" ")
			w.WriteString(attr.Key)
			w.WriteString(`="`)
			w.WriteString(attr.Val)
			w.WriteString(`"`)
		}
		if isVoidHTMLElement(n.Data) {
			w.WriteString(">")
		} else {
			w.WriteString(">")
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				renderNode(w, child)
			}
			w.WriteString("</")
			w.WriteString(n.Data)
			w.WriteString(">")
		}
	}
}

// isVoidHTMLElement checks if an HTML element is void (self-closing)
func isVoidHTMLElement(tagName string) bool {
	voidElements := map[string]bool{
		"area": true, "base": true, "br": true, "col": true, "embed": true,
		"hr": true, "img": true, "input": true, "link": true, "meta": true,
		"param": true, "source": true, "track": true, "wbr": true,
	}
	return voidElements[strings.ToLower(tagName)]
}

// PreparedTemplate holds compile-time template analysis
type PreparedTemplate struct {
	Template    *template.Template
	TemplateStr string
	Structure   *TemplateStructure
	Fingerprint string
}

// TemplateStructure represents the static/dynamic structure identified at compile-time
type TemplateStructure struct {
	StaticSegments      []string             // Pure HTML segments that never change
	DynamicPlaceholders []DynamicPlaceholder // Placeholders for dynamic content
}

type DynamicPlaceholder struct {
	Index      int    // Position in the static segments array
	Expression string // The template expression (e.g., "{{.Title}}")
	Type       string // "field", "conditional", "range", etc.
}

// PrepareTemplate performs compile-time template preparation
func PrepareTemplate(templateStr string) (*PreparedTemplate, error) {
	tmpl, err := template.New("prepared").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	// Analyze template structure at compile-time (no execution)
	structure, err := analyzeTemplateStructure(templateStr)
	if err != nil {
		return nil, fmt.Errorf("template analysis error: %w", err)
	}

	// Create fingerprint of template structure (static parts)
	fingerprint := calculateTemplateFingerprint(templateStr)

	return &PreparedTemplate{
		Template:    tmpl,
		TemplateStr: templateStr,
		Structure:   structure,
		Fingerprint: fingerprint,
	}, nil
}

// analyzeTemplateStructure performs compile-time analysis to separate static and dynamic parts
func analyzeTemplateStructure(templateStr string) (*TemplateStructure, error) {
	// Find all template expressions
	re := regexp.MustCompile(`\{\{[^}]*\}\}`)
	matches := re.FindAllStringIndex(templateStr, -1)

	if len(matches) == 0 {
		// No template expressions - everything is static
		return &TemplateStructure{
			StaticSegments:      []string{templateStr},
			DynamicPlaceholders: []DynamicPlaceholder{},
		}, nil
	}

	var staticSegments []string
	var dynamicPlaceholders []DynamicPlaceholder

	currentPos := 0
	placeholderIndex := 0

	for _, match := range matches {
		start, end := match[0], match[1]

		// Add static part before this expression
		if start > currentPos {
			staticSegments = append(staticSegments, templateStr[currentPos:start])
		} else if len(staticSegments) == 0 {
			staticSegments = append(staticSegments, "")
		}

		// Extract the template expression
		expression := templateStr[start:end]

		// Classify the expression type
		exprType := classifyExpression(expression)

		// Add dynamic placeholder
		dynamicPlaceholders = append(dynamicPlaceholders, DynamicPlaceholder{
			Index:      placeholderIndex,
			Expression: expression,
			Type:       exprType,
		})

		placeholderIndex++
		currentPos = end
	}

	// Add final static part
	if currentPos < len(templateStr) {
		staticSegments = append(staticSegments, templateStr[currentPos:])
	} else {
		staticSegments = append(staticSegments, "")
	}

	return &TemplateStructure{
		StaticSegments:      staticSegments,
		DynamicPlaceholders: dynamicPlaceholders,
	}, nil
}

// classifyExpression determines the type of template expression
func classifyExpression(expr string) string {
	content := strings.TrimSpace(expr[2 : len(expr)-2])

	if strings.HasPrefix(content, "range ") {
		return "range"
	}
	if strings.HasPrefix(content, "if ") {
		return "conditional"
	}
	if content == "end" || content == "else" {
		return "control"
	}
	if strings.HasPrefix(content, ".") || strings.Contains(content, ".") {
		return "field"
	}

	return "other"
}

// RenderToTree performs runtime tree generation - proper static/dynamic separation
func (pt *PreparedTemplate) RenderToTree(data interface{}) (TreeNode, error) {
	// Use the previous working approach but fix the range handling
	// This maintains backward compatibility while addressing the core issue
	// Use global key generator for legacy PreparedTemplate (which doesn't have its own KeyGenerator)
	return parseTemplateToTree(pt.TemplateStr, data, globalKeyGenerator)
}

// normalizeTemplateSpacing normalizes spacing in template tags to prevent formatter issues
// Converts "{{ if .X }}" to "{{if .X}}" and "{{ range .Y }}" to "{{range .Y}}"
func normalizeTemplateSpacing(templateStr string) string {
	// Pattern to match template tags: {{ ... }}
	// Captures the content between {{ and }}
	re := regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

	return re.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Extract content between {{ and }}
		content := strings.TrimSpace(match[2 : len(match)-2])

		// Reconstruct with no spaces after {{ and before }}
		return "{{" + content + "}}"
	})
}

// parseTemplateToTree parses a template using the AST-based parser
func parseTemplateToTree(templateStr string, data interface{}, keyGen *KeyGenerator) (tree TreeNode, err error) {
	// Recover from panics in template execution (can happen with fuzz-generated templates)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("template execution panic: %v", r)
		}
	}()

	return parseTemplateToTreeAST(templateStr, data, keyGen)
}

// Decoupled Template Construct System
// Each Go template construct is handled independently

type ConstructType int

const (
	FieldType ConstructType = iota
	ConditionalType
	RangeType
	WithType
	TemplateType
	BlockType
	DefineType
)

// Construct interface - each construct type implements this
type Construct interface {
	Type() ConstructType
	Position() int
	Parse(templateStr string, startPos int) (Construct, int, error)
	Compile() (CompiledConstruct, error)
	Evaluate(data interface{}) (interface{}, error)
}

// CompiledConstruct represents a construct ready for fast evaluation
type CompiledConstruct interface {
	Evaluate(data interface{}) (interface{}, error)
	GetStaticParts() []string
}

// Field construct: {{.FieldName}}, {{.Object.Property}}
type FieldConstruct struct {
	Expression string
	Text       string // Original template text
	Pos        int
}

func (f *FieldConstruct) Type() ConstructType { return FieldType }
func (f *FieldConstruct) Position() int       { return f.Pos }

func (f *FieldConstruct) Parse(templateStr string, startPos int) (Construct, int, error) {
	// Implementation for parsing field constructs
	return f, startPos + len(f.Expression), nil
}

func (f *FieldConstruct) Compile() (CompiledConstruct, error) {
	return nil, fmt.Errorf("legacy Compile() not implemented for AST parser")
}

func (f *FieldConstruct) Evaluate(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("legacy Evaluate() not implemented for AST parser")
}

// Conditional construct: {{if .Condition}}...{{else}}...{{end}}
type ConditionalConstruct struct {
	Condition   string
	TrueBranch  []Construct
	FalseBranch []Construct
	Text        string // Original template text
	Pos         int
}

func (c *ConditionalConstruct) Type() ConstructType { return ConditionalType }
func (c *ConditionalConstruct) Position() int       { return c.Pos }

func (c *ConditionalConstruct) Parse(templateStr string, startPos int) (Construct, int, error) {
	return c, startPos, nil
}

func (c *ConditionalConstruct) Compile() (CompiledConstruct, error) {
	return nil, fmt.Errorf("legacy Compile() not implemented for AST parser")
}

func (c *ConditionalConstruct) Evaluate(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("legacy Evaluate() not implemented for AST parser")
}

// Range construct: {{range .Items}}...{{end}}
type RangeConstruct struct {
	Variable   string
	Collection string
	Body       []Construct
	Text       string // Original template text
	Pos        int
}

func (r *RangeConstruct) Type() ConstructType { return RangeType }
func (r *RangeConstruct) Position() int       { return r.Pos }

func (r *RangeConstruct) Parse(templateStr string, startPos int) (Construct, int, error) {
	return r, startPos, nil
}

func (r *RangeConstruct) Compile() (CompiledConstruct, error) {
	return nil, fmt.Errorf("legacy Compile() not implemented for AST parser")
}

func (r *RangeConstruct) Evaluate(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("legacy Evaluate() not implemented for AST parser")
}

// With construct: {{with .Object}}...{{end}}
type WithConstruct struct {
	Variable string
	Body     []Construct
	Text     string // Original template text
	Pos      int
}

func (w *WithConstruct) Type() ConstructType { return WithType }
func (w *WithConstruct) Position() int       { return w.Pos }

func (w *WithConstruct) Parse(templateStr string, startPos int) (Construct, int, error) {
	return w, startPos, nil
}

func (w *WithConstruct) Compile() (CompiledConstruct, error) {
	return nil, fmt.Errorf("legacy Compile() not implemented for AST parser")
}

func (w *WithConstruct) Evaluate(data interface{}) (interface{}, error) {
	return nil, fmt.Errorf("legacy Evaluate() not implemented for AST parser")
}

// Template invocation: {{template "name" .}}
type TemplateInvokeConstruct struct {
	Name string
	Data string
	Text string // Original template text
	Pos  int
}

func (t *TemplateInvokeConstruct) Type() ConstructType { return TemplateType }
func (t *TemplateInvokeConstruct) Position() int       { return t.Pos }

func (t *TemplateInvokeConstruct) Parse(templateStr string, startPos int) (Construct, int, error) {
	return t, startPos, nil
}

func (t *TemplateInvokeConstruct) Compile() (CompiledConstruct, error) {
	return &CompiledTemplateConstruct{Name: t.Name, Data: t.Data}, nil
}

func (t *TemplateInvokeConstruct) Evaluate(data interface{}) (interface{}, error) {
	// Template invocation evaluation would require template registry
	return fmt.Sprintf("{{template \"%s\" %s}}", t.Name, t.Data), nil
}

// Updated CompiledTemplate for new system
type CompiledTemplate struct {
	TemplateStr string
	Constructs  []Construct
	StaticParts []string
	Fingerprint string
}

// CompiledConstruct implementations

type CompiledFieldConstruct struct {
	Expression string
}

func (c *CompiledFieldConstruct) Evaluate(data interface{}) (interface{}, error) {
	return "", fmt.Errorf("legacy CompiledFieldConstruct not implemented for AST parser")
}

func (c *CompiledFieldConstruct) GetStaticParts() []string {
	return []string{} // Field constructs have no static parts
}

type CompiledConditionalConstruct struct {
	Condition string
}

func (c *CompiledConditionalConstruct) Evaluate(data interface{}) (interface{}, error) {
	return "", fmt.Errorf("legacy CompiledConditionalConstruct not implemented for AST parser")
}

func (c *CompiledConditionalConstruct) GetStaticParts() []string {
	return []string{} // Static parts handled at template level
}

type CompiledRangeConstruct struct {
	Collection string
}

func (c *CompiledRangeConstruct) Evaluate(data interface{}) (interface{}, error) {
	return "", fmt.Errorf("legacy CompiledRangeConstruct not implemented for AST parser")
}

func (c *CompiledRangeConstruct) GetStaticParts() []string {
	return []string{} // Range static parts extracted separately
}

type CompiledWithConstruct struct {
	Variable string
}

func (c *CompiledWithConstruct) Evaluate(data interface{}) (interface{}, error) {
	return "", fmt.Errorf("legacy CompiledWithConstruct not implemented for AST parser")
}

func (c *CompiledWithConstruct) GetStaticParts() []string {
	return []string{}
}

type CompiledTemplateConstruct struct {
	Name string
	Data string
}

func (c *CompiledTemplateConstruct) Evaluate(data interface{}) (interface{}, error) {
	return fmt.Sprintf("{{template \"%s\" %s}}", c.Name, c.Data), nil
}

func (c *CompiledTemplateConstruct) GetStaticParts() []string {
	return []string{}
}

// Helper functions for extracting template variables

func getFieldValue(data interface{}, fieldName string) (interface{}, error) {
	dataValue := reflect.ValueOf(data)

	// Handle maps
	if dataValue.Kind() == reflect.Map {
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("map must be map[string]interface{}")
		}
		value, exists := mapData[fieldName]
		if !exists {
			return nil, fmt.Errorf("field %s not found", fieldName)
		}
		return value, nil
	}

	// Dereference pointers
	if dataValue.Kind() == reflect.Ptr {
		dataValue = dataValue.Elem()
	}

	// Handle structs
	if dataValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("data must be struct or map")
	}

	field := dataValue.FieldByName(fieldName)
	if !field.IsValid() {
		return nil, fmt.Errorf("field %s not found", fieldName)
	}

	return field.Interface(), nil
}

// Helper functions

// KeyAttributeConfig defines which attributes to check for explicit keys
type KeyAttributeConfig struct {
	AttributeNames []string
}

// DefaultKeyAttributes provides sensible defaults for key attribute names
var DefaultKeyAttributes = KeyAttributeConfig{
	AttributeNames: []string{
		"key",
		"lvt-key",
		"data-key",
		"data-lvt-key",
		"data-id",
		"id",
		"x-key", // Alpine.js compatibility
		"v-key", // Vue.js compatibility
	},
}

// Simple counter-based key generation for wrapper approach
type KeyGenerator struct {
	counter      int
	usedKeys     map[string]bool    // Track used keys to prevent duplicates
	fallbackKeys []string           // Position-based fallback keys
	keyConfig    KeyAttributeConfig // Configuration for key attribute names
}

// NewKeyGenerator creates a new key generator for a template instance
func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{
		counter:      0,
		usedKeys:     make(map[string]bool),
		fallbackKeys: []string{},
		keyConfig:    DefaultKeyAttributes,
	}
}

// NextKey generates the next sequential key
func (kg *KeyGenerator) NextKey() string {
	kg.counter++
	return fmt.Sprintf("%d", kg.counter)
}

// Reset resets the counter (useful for testing)
func (kg *KeyGenerator) Reset() {
	kg.counter = 0
	kg.usedKeys = make(map[string]bool)
	kg.fallbackKeys = []string{}
}

// LoadExistingKeys stores previous data and updates counter
func (kg *KeyGenerator) LoadExistingKeys(oldRangeData []interface{}) {
	// Reset used keys tracking
	kg.usedKeys = make(map[string]bool)

	// Extract max key to update counter
	for _, item := range oldRangeData {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Track this key as used
			if keyValue, exists := itemMap["0"]; exists {
				if keyStr, ok := keyValue.(string); ok {
					kg.usedKeys[keyStr] = true

					// Update counter if it's a numeric key
					if keyInt, err := strconv.Atoi(keyStr); err == nil && keyInt > kg.counter {
						kg.counter = keyInt
					}
				}
			}
		}
	}
}

// Global key generator for template instances
var globalKeyGenerator = NewKeyGenerator()

// resetKeyGenerator resets the global key generator for testing
func resetKeyGenerator() {
	globalKeyGenerator.Reset()
}

// generateWrapperKey generates a simple wrapper key using provided generator
func generateWrapperKey(keyGen *KeyGenerator) string {
	return keyGen.NextKey()
}

// ParseTemplateToTree parses a template using existing working approach (exported for testing)
func ParseTemplateToTree(templateStr string, data interface{}) (TreeNode, error) {
	return parseTemplateToTree(templateStr, data, globalKeyGenerator)
}

// calculateTemplateFingerprint creates a fingerprint of template structure
func calculateTemplateFingerprint(templateStr string) string {
	hasher := md5.New()
	hasher.Write([]byte(templateStr))
	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) >= 16 {
		return fullHash[:16]
	}
	return fullHash
}
