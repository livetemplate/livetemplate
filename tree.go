package livetemplate

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/livefir/livetemplate/internal/build"
	"github.com/livefir/livetemplate/internal/parse"
	"golang.org/x/net/html"
)

// calculateFingerprint wraps internal/build.CalculateFingerprint for backward compatibility
func calculateFingerprint(tree *TreeNode) string {
	return build.CalculateFingerprint(tree)
}

// addFingerprintToTree wraps internal/build.AddFingerprintToTree for backward compatibility
func addFingerprintToTree(tree *TreeNode) *TreeNode {
	return build.AddFingerprintToTree(tree)
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

// parseTemplateToTree parses a template using the internal/parse package
// ctx is optional - if nil, defaults to first-render context (includes statics)
func parseTemplateToTree(templateStr string, data interface{}, keyGen *keyGenerator, ctx ...*TreeGenerationContext) (tree *TreeNode, err error) {
	// Recover from panics in template execution (can happen with fuzz-generated templates)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("template execution panic: %v", r)
		}
	}()

	// Get or create context
	var genCtx *TreeGenerationContext
	if len(ctx) > 0 {
		genCtx = ctx[0]
	}
	if genCtx == nil {
		genCtx = NewTreeGenerationContext()
	}

	// Parse template
	tmpl, err := parse.Parse(templateStr, genCtx.FuncMap)
	if err != nil {
		return nil, err
	}

	// Build tree using internal/parse package
	// Create adapter for keyGenerator to match parse.KeyGenerator interface
	keyGenAdapter := &keyGeneratorAdapter{kg: keyGen}
	return parse.BuildTree(tmpl, data, keyGenAdapter, genCtx)
}

// keyAttributeConfig defines which attributes to check for explicit keys (internal use only)
type keyAttributeConfig struct {
	AttributeNames []string
}

// defaultKeyAttributes provides sensible defaults for key attribute names (internal use only)
var defaultKeyAttributes = keyAttributeConfig{
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

// keyGenerator provides counter-based key generation for wrapper approach (internal use only)
type keyGenerator struct {
	counter      int
	usedKeys     map[string]bool    // Track used keys to prevent duplicates
	fallbackKeys []string           // Position-based fallback keys
	keyConfig    keyAttributeConfig // Configuration for key attribute names
}

// newKeyGenerator creates a new key generator for a template instance
func newKeyGenerator() *keyGenerator {
	return &keyGenerator{
		counter:      0,
		usedKeys:     make(map[string]bool),
		fallbackKeys: []string{},
		keyConfig:    defaultKeyAttributes,
	}
}

// nextKey generates the next sequential key
func (kg *keyGenerator) nextKey() string {
	kg.counter++
	return fmt.Sprintf("%d", kg.counter)
}

// reset resets the counter (useful for testing)
func (kg *keyGenerator) reset() {
	kg.counter = 0
	kg.usedKeys = make(map[string]bool)
	kg.fallbackKeys = []string{}
}

// keyGeneratorAdapter adapts keyGenerator to parse.KeyGenerator interface
type keyGeneratorAdapter struct {
	kg *keyGenerator
}

// Next implements parse.KeyGenerator interface
func (kga *keyGeneratorAdapter) Next() string {
	return kga.kg.nextKey()
}

// loadExistingKeys stores previous data and updates counter
func (kg *keyGenerator) loadExistingKeys(oldRangeData []interface{}) {
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

// globalKeyGenerator is the global key generator for template instances
var globalKeyGenerator = newKeyGenerator()

// resetKeyGenerator resets the global key generator for testing
func resetKeyGenerator() {
	globalKeyGenerator.reset()
}

// generateWrapperKey generates a simple wrapper key using provided generator
func generateWrapperKey(keyGen *keyGenerator) string {
	return keyGen.nextKey()
}

// detectIDKey detects which position in the dynamics contains the item ID
// by scanning the statics array for key attribute patterns
// Returns the position as a string (e.g., "1" for the second dynamic position)
// Returns "0" as default if no key attribute is found
func detectIDKey(statics []string) string {
	if len(statics) == 0 {
		return "0"
	}

	// Key attributes to search for (in priority order)
	keyAttrs := []string{
		"id=\"",
		"data-key=\"",
		"key=\"",
		"data-lvt-key=\"",
		"lvt-key=\"",
		"data-id=\"",
		"x-key=\"",
		"v-key=\"",
	}

	// Scan through statics array
	for i, static := range statics {
		// Check if this static contains a key attribute
		for _, attr := range keyAttrs {
			if strings.Contains(static, attr) {
				// The dynamic value after this static is the ID
				// Position i in statics means dynamic at position i+1
				// But we need to return the dynamic index, which starts at 0
				// So dynamic position is i (0-indexed in the dynamics)
				return fmt.Sprintf("%d", i)
			}
		}
	}

	// Default to position 0 if no key attribute found
	return "0"
}

// renderTreeToHTML renders a tree structure to HTML (used in tests)
func renderTreeToHTML(tree map[string]interface{}) (string, error) {
	// Check if this is a range comprehension (has "d" key with items)
	if itemsRaw, hasD := tree["d"]; hasD {
		return renderRangeComprehensionToHTML(tree, itemsRaw)
	}

	statics, ok := tree["s"].([]string)
	if !ok || len(statics) == 0 {
		return "", fmt.Errorf("invalid tree: no statics")
	}

	var result strings.Builder

	// Interleave statics and dynamics
	dynamicIndex := 0
	for i, static := range statics {
		result.WriteString(static)

		// After each static (except the last), add the corresponding dynamic
		if i < len(statics)-1 {
			dynKey := fmt.Sprintf("%d", dynamicIndex)
			if dynValue, exists := tree[dynKey]; exists {
				// Handle nested trees (like ranges)
				if nestedTree, ok := dynValue.(map[string]interface{}); ok {
					nestedHTML, err := renderTreeToHTML(nestedTree)
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
					// Also handle as map[string]interface{}
					nestedHTML, err := renderTreeToHTML(map[string]interface{}(nestedMap))
					if err != nil {
						return "", err
					}
					result.WriteString(nestedHTML)
				} else {
					// Simple value - convert to string
					result.WriteString(fmt.Sprintf("%v", dynValue))
				}
			}
			dynamicIndex++
		}
	}

	return result.String(), nil
}

// renderRangeComprehensionToHTML renders a range comprehension (with "d" and "s" keys) to HTML
func renderRangeComprehensionToHTML(tree map[string]interface{}, itemsRaw interface{}) (string, error) {
	// Get statics for the range items
	statics, ok := tree["s"].([]string)
	if !ok {
		return "", fmt.Errorf("range comprehension missing statics")
	}

	// Convert items to []interface{}
	var items []interface{}
	switch v := itemsRaw.(type) {
	case []interface{}:
		items = v
	case []map[string]interface{}:
		items = make([]interface{}, len(v))
		for i, item := range v {
			items[i] = item
		}
	default:
		return "", fmt.Errorf("unexpected items type: %T", itemsRaw)
	}

	var result strings.Builder

	// Render each item using the statics as template
	for _, itemRaw := range items {
		itemMap, ok := itemRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Interleave statics and item dynamics
		for i, static := range statics {
			result.WriteString(static)

			// After each static (except the last), add the corresponding dynamic
			if i < len(statics)-1 {
				dynKey := fmt.Sprintf("%d", i)
				if dynValue, exists := itemMap[dynKey]; exists {
					// Recursively render nested trees
					if nestedTree, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := renderTreeToHTML(nestedTree)
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else if nestedMap, ok := dynValue.(map[string]interface{}); ok {
						nestedHTML, err := renderTreeToHTML(map[string]interface{}(nestedMap))
						if err != nil {
							return "", err
						}
						result.WriteString(nestedHTML)
					} else {
						// Simple value
						result.WriteString(fmt.Sprintf("%v", dynValue))
					}
				}
			}
		}
	}

	return result.String(), nil
}
