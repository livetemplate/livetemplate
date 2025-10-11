package livetemplate

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

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
	rand.Read(b)
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

// parseTemplateToTree parses a template using render → parse approach
func parseTemplateToTree(templateStr string, data interface{}, keyGen *KeyGenerator) (TreeNode, error) {
	// Normalize template spacing to handle formatter-added spaces
	templateStr = normalizeTemplateSpacing(templateStr)

	// Use the working old system for now - extract expressions and build tree

	// First render the template to get the final HTML
	tmpl, err := template.New("temp").Parse(templateStr)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	rendered := buf.String()

	// Extract expressions and build tree
	expressions := extractFlattenedExpressions(templateStr)

	return buildTreeFromExpressions(templateStr, rendered, expressions, data, keyGen)
}

// extractFlattenedExpressions extracts all template expressions with Phoenix LiveView optimization
func extractFlattenedExpressions(templateStr string) []TemplateExpression {
	var expressions []TemplateExpression

	// First pass: Detect conditional ranges (prioritize complex patterns)
	conditionalRanges := detectConditionalRanges(templateStr)

	// Convert conditional range patterns to expressions
	for _, pattern := range conditionalRanges {
		expressions = append(expressions, TemplateExpression{
			Text:  pattern.Text,
			Type:  "range", // Treat the entire conditional-range as a range comprehension
			Start: pattern.Start,
			End:   pattern.End,
		})
	}

	// Second pass: Detect simple ranges only if no conditional ranges were found
	if len(conditionalRanges) == 0 {
		simpleRanges := detectSimpleRanges(templateStr)

		// Convert simple ranges to expressions
		for _, pattern := range simpleRanges {
			expressions = append(expressions, TemplateExpression{
				Text:  pattern.Text,
				Type:  "range",
				Start: pattern.Start,
				End:   pattern.End,
			})
		}
	}

	// Third pass: Extract all other expressions that don't overlap with range patterns
	i := 0
	for i < len(templateStr) {
		// Find next template expression
		start := strings.Index(templateStr[i:], "{{")
		if start == -1 {
			break
		}
		start += i

		// Skip if this position is inside any detected range pattern
		if isInsideConditionalRange(start, conditionalRanges) {
			i = start + 2
			continue
		}

		// Only check simple ranges if we have them (when no conditional ranges were found)
		if len(conditionalRanges) == 0 {
			simpleRanges := detectSimpleRanges(templateStr)
			if isInsideSimpleRange(start, simpleRanges) {
				i = start + 2
				continue
			}
		}

		// Find the end of this expression
		end := strings.Index(templateStr[start+2:], "}}")
		if end == -1 {
			break
		}
		end += start + 4

		// Extract expression content
		exprContent := strings.TrimSpace(templateStr[start+2 : end-2])

		// Classify the expression
		if strings.HasPrefix(exprContent, "if ") {
			// Handle conditional - find the matching {{end}}
			condExpr, condEnd := extractConditionalBlock(templateStr, start)
			if condExpr.Text != "" && !isInsideConditionalRange(start, conditionalRanges) {
				expressions = append(expressions, condExpr)
			}
			i = condEnd
		} else if !strings.HasPrefix(exprContent, "range ") && !strings.HasPrefix(exprContent, "end") && !strings.HasPrefix(exprContent, "else") {
			// Simple field expression
			expressions = append(expressions, TemplateExpression{
				Text:  exprContent,
				Type:  "field",
				Start: start,
				End:   end,
			})
			i = end
		} else {
			i = end
		}
	}

	// Sort expressions by start position
	sort.Slice(expressions, func(i, j int) bool {
		return expressions[i].Start < expressions[j].Start
	})

	return expressions
}

// PhoenixPattern represents a conditional that wraps a range for Phoenix optimization
type ConditionalRange struct {
	Text  string
	Start int
	End   int
}

// detectSimpleRanges finds simple range patterns {{range .Items}}...{{end}}
func detectSimpleRanges(templateStr string) []ConditionalRange {
	var ranges []ConditionalRange
	i := 0

	for i < len(templateStr) {
		// Find range start
		rangeStart := strings.Index(templateStr[i:], "{{range ")
		if rangeStart == -1 {
			break
		}
		rangeStart += i

		// Find the closing }}
		rangeExprEnd := strings.Index(templateStr[rangeStart:], "}}")
		if rangeExprEnd == -1 {
			break
		}
		rangeExprEnd += rangeStart + 2

		// Find matching {{end}}
		endPos := findMatchingEndForExpression(templateStr, rangeExprEnd)
		if endPos == -1 {
			i = rangeStart + 1
			continue
		}

		// Create range pattern
		ranges = append(ranges, ConditionalRange{
			Text:  templateStr[rangeStart:endPos],
			Start: rangeStart,
			End:   endPos,
		})

		i = endPos
	}

	return ranges
}

// isInsideSimpleRange checks if position is inside any simple range
func isInsideSimpleRange(pos int, ranges []ConditionalRange) bool {
	for _, r := range ranges {
		if pos >= r.Start && pos < r.End {
			return true
		}
	}
	return false
}

// detectPhoenixConditionalRangePatterns finds conditionals that wrap ranges (Phoenix LiveView pattern)
func detectConditionalRanges(templateStr string) []ConditionalRange {
	var patterns []ConditionalRange

	// Look for patterns like {{if gt (len .Field) 0}}...{{range .Field}}...{{end}}...{{else}}...{{end}}
	// This is a Phoenix LiveView optimization for list rendering
	i := 0
	for i < len(templateStr) {
		// Find {{if ...}}
		ifStart := strings.Index(templateStr[i:], "{{if ")
		if ifStart == -1 {
			break
		}
		ifStart += i

		// Find the matching {{end}} for this {{if}}
		condExpr, condEnd := extractConditionalBlock(templateStr, ifStart)
		if condExpr.Text == "" {
			i = ifStart + 5
			continue
		}

		// Check if this is a TRUE Phoenix pattern:
		// 1. Must contain a {{range }}
		// 2. The {{if}} condition should check length/presence of a collection
		// 3. The {{range}} should be at the TOP LEVEL of the if block, not nested in another conditional
		if strings.Contains(condExpr.Text, "{{range ") {
			// Extract the if condition to see if it checks a collection length
			ifCondition := extractIfCondition(templateStr[ifStart:])

			// Check if condition mentions "len" or "gt" which are typical for Phoenix patterns
			if strings.Contains(ifCondition, "len ") || strings.Contains(ifCondition, "gt ") {
				// Check if the range is at the TOP LEVEL (not nested in another {{if}})
				isTopLevel := isRangeAtTopLevel(condExpr.Text)

				if isTopLevel {
					patterns = append(patterns, ConditionalRange{
						Text:  condExpr.Text,
						Start: ifStart,
						End:   condEnd,
					})
					i = condEnd
					continue
				}
			}
		}

		i = ifStart + 5
	}

	return patterns
}

// extractIfCondition extracts the condition from an {{if ...}} expression
func extractIfCondition(ifBlock string) string {
	start := strings.Index(ifBlock, "{{if ")
	if start == -1 {
		return ""
	}
	end := strings.Index(ifBlock[start:], "}}")
	if end == -1 {
		return ""
	}
	return ifBlock[start+5 : start+end]
}

// isRangeAtTopLevel checks if the range is at the top level of the conditional block
// (not nested inside another {{if}})
func isRangeAtTopLevel(condText string) bool {
	// Find the first {{range}} position
	rangePos := strings.Index(condText, "{{range ")
	if rangePos == -1 {
		return false
	}

	// Skip the opening {{if ...}} of this conditional block
	// We want to check if there are NESTED {{if}} blocks before the range,
	// not count the opening {{if}} of the block we're analyzing
	startPos := 0
	if strings.HasPrefix(condText, "{{if ") {
		// Find the end of the opening {{if ...}} expression
		endOfIf := strings.Index(condText, "}}")
		if endOfIf != -1 {
			startPos = endOfIf + 2
		}
	}

	// Check if there's a nested {{if}} between startPos and {{range}}
	// that hasn't been closed yet
	depth := 0
	pos := startPos
	for pos < rangePos {
		if strings.HasPrefix(condText[pos:], "{{if ") {
			depth++
			pos += 5
		} else if strings.HasPrefix(condText[pos:], "{{end}}") {
			depth--
			pos += 7
		} else {
			pos++
		}
	}

	// If depth > 0, the range is inside a nested {{if}}
	// If depth == 0, the range is at the top level (directly in the if/else branches)
	return depth == 0
}

// isInsidePhoenixPattern checks if a position is inside any Phoenix pattern
func isInsideConditionalRange(pos int, patterns []ConditionalRange) bool {
	for _, pattern := range patterns {
		if pos >= pattern.Start && pos < pattern.End {
			return true
		}
	}
	return false
}

// TemplateExpression represents a template expression with its type and position
type TemplateExpression struct {
	Text  string
	Type  string // "field", "conditional", "range"
	Start int
	End   int
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
	return &CompiledFieldConstruct{Expression: f.Expression}, nil
}

func (f *FieldConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateFieldExpression(f.Expression, data), nil
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
	return &CompiledConditionalConstruct{Condition: c.Condition}, nil
}

func (c *ConditionalConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateConditionalExpression(c.Condition, data), nil
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
	return &CompiledRangeConstruct{Collection: r.Collection}, nil
}

func (r *RangeConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateRangeBlock(r.Collection, data), nil
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
	return &CompiledWithConstruct{Variable: w.Variable}, nil
}

func (w *WithConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateFieldExpression(w.Variable, data), nil
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
	return evaluateFieldExpression(c.Expression, data), nil
}

func (c *CompiledFieldConstruct) GetStaticParts() []string {
	return []string{} // Field constructs have no static parts
}

type CompiledConditionalConstruct struct {
	Condition string
}

func (c *CompiledConditionalConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateConditionalExpression(c.Condition, data), nil
}

func (c *CompiledConditionalConstruct) GetStaticParts() []string {
	return []string{} // Static parts handled at template level
}

type CompiledRangeConstruct struct {
	Collection string
}

func (c *CompiledRangeConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateRangeBlock(c.Collection, data), nil
}

func (c *CompiledRangeConstruct) GetStaticParts() []string {
	return []string{} // Range static parts extracted separately
}

type CompiledWithConstruct struct {
	Variable string
}

func (c *CompiledWithConstruct) Evaluate(data interface{}) (interface{}, error) {
	return evaluateFieldExpression(c.Variable, data), nil
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

// CompileTemplate parses all constructs independently and prepares for fast evaluation
func CompileTemplate(templateStr string) (*CompiledTemplate, error) {
	constructs := parseAllConstructs(templateStr)
	staticParts := extractStaticPartsFromTemplate(templateStr, constructs)

	return &CompiledTemplate{
		TemplateStr: templateStr,
		Constructs:  constructs,
		StaticParts: staticParts,
		Fingerprint: calculateTemplateFingerprint(templateStr),
	}, nil
}

// Phase 2: Full Page Rendering

type RenderedPage struct {
	HTML             string
	CompiledTemplate *CompiledTemplate
	CurrentData      interface{}
	Fingerprint      string
}

// RenderPage generates complete HTML from template + data
func (ct *CompiledTemplate) RenderPage(data interface{}) (*RenderedPage, error) {
	// Use Go's template engine to render the complete HTML
	tmpl, err := template.New("page").Parse(ct.TemplateStr)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("template execution error: %w", err)
	}

	html := buf.String()

	return &RenderedPage{
		HTML:             html,
		CompiledTemplate: ct,
		CurrentData:      data,
		Fingerprint:      calculateDataFingerprint(data),
	}, nil
}

// Phase 3: Update Tree Generation (Direct TreeNode output)

// GenerateUpdateTree creates minimal update tree for data changes
func (rp *RenderedPage) GenerateUpdateTree(newData interface{}) (TreeNode, error) {
	return rp.CompiledTemplate.GenerateUpdateTree(rp.CurrentData, newData)
}

// GenerateUpdateTree creates update tree by comparing old and new data
func (ct *CompiledTemplate) GenerateUpdateTree(oldData, newData interface{}) (TreeNode, error) {
	// Build tree structure with static parts
	tree := TreeNode{
		"s": ct.StaticParts,
	}

	// Evaluate all constructs with new data and add their values
	for i, construct := range ct.Constructs {
		value, err := construct.Evaluate(newData)
		if err != nil {
			return nil, fmt.Errorf("construct evaluation error: %w", err)
		}
		tree[fmt.Sprintf("%d", i)] = value
	}

	return tree, nil
}

// Helper function for data fingerprinting
func calculateDataFingerprint(data interface{}) string {
	hasher := md5.New()

	// Create a simple representation of the data for hashing
	dataStr := fmt.Sprintf("%+v", data)
	hasher.Write([]byte(dataStr))

	return hex.EncodeToString(hasher.Sum(nil))
}

// parseAllConstructs finds all template constructs independently
func parseAllConstructs(templateStr string) []Construct {
	var constructs []Construct

	// Register all construct parsers
	parsers := []ConstructParser{
		&FieldParser{},
		&ConditionalParser{},
		&RangeParser{},
		&WithParser{},
		&TemplateInvokeParser{},
	}

	// Each parser handles its own construct type independently
	for _, parser := range parsers {
		found := parser.FindConstructs(templateStr)
		constructs = append(constructs, found...)
	}

	// Sort by position to maintain order
	sort.Slice(constructs, func(i, j int) bool {
		return constructs[i].Position() < constructs[j].Position()
	})

	return constructs
}

// ConstructParser interface for independent construct parsing
type ConstructParser interface {
	FindConstructs(templateStr string) []Construct
}

// extractStaticPartsFromTemplate extracts static parts based on construct positions
func extractStaticPartsFromTemplate(templateStr string, constructs []Construct) []string {
	var staticParts []string
	lastPos := 0

	for _, construct := range constructs {
		// Add static part before this construct
		if construct.Position() > lastPos {
			static := templateStr[lastPos:construct.Position()]
			if static != "" {
				staticParts = append(staticParts, static)
			}
		}
		lastPos = construct.Position() + len(getConstructText(construct))
	}

	// Add final static part
	if lastPos < len(templateStr) {
		final := templateStr[lastPos:]
		if final != "" {
			staticParts = append(staticParts, final)
		}
	}

	return staticParts
}

// getConstructText returns the original template text for a construct
func getConstructText(construct Construct) string {
	switch c := construct.(type) {
	case *FieldConstruct:
		return c.Text
	case *ConditionalConstruct:
		return c.Text
	case *RangeConstruct:
		return c.Text
	case *WithConstruct:
		return c.Text
	case *TemplateInvokeConstruct:
		return c.Text
	default:
		return ""
	}
}

// Concrete parser implementations for each construct type

// FieldParser handles simple field expressions: {{.Name}}
type FieldParser struct{}

func (p *FieldParser) FindConstructs(templateStr string) []Construct {
	var constructs []Construct
	re := regexp.MustCompile(`\{\{\s*\.[^}]+\s*\}\}`)
	matches := re.FindAllStringIndex(templateStr, -1)

	for _, match := range matches {
		expr := templateStr[match[0]:match[1]]
		// Skip if this is part of a complex construct (if/range/with)
		if !isPartOfComplexConstruct(templateStr, match[0]) {
			constructs = append(constructs, &FieldConstruct{
				Expression: expr,
				Text:       expr,
				Pos:        match[0],
			})
		}
	}
	return constructs
}

// ConditionalParser handles {{if}}...{{else}}...{{end}}
type ConditionalParser struct{}

func (p *ConditionalParser) FindConstructs(templateStr string) []Construct {
	var constructs []Construct
	i := 0
	for i < len(templateStr) {
		ifStart := strings.Index(templateStr[i:], "{{if ")
		if ifStart == -1 {
			break
		}
		ifStart += i

		// Extract the complete conditional block
		condExpr, condEnd := extractConditionalBlock(templateStr, ifStart)
		if condExpr.Text != "" {
			constructs = append(constructs, &ConditionalConstruct{
				Condition: extractCondition(condExpr.Text),
				Pos:       ifStart,
			})
			i = condEnd
		} else {
			i = ifStart + 5
		}
	}
	return constructs
}

// RangeParser handles {{range}}...{{end}}
type RangeParser struct{}

func (p *RangeParser) FindConstructs(templateStr string) []Construct {
	var constructs []Construct
	i := 0
	for i < len(templateStr) {
		rangeStart := strings.Index(templateStr[i:], "{{range ")
		if rangeStart == -1 {
			break
		}
		rangeStart += i

		// Extract the complete range block
		rangeExpr, rangeEnd := extractRangeBlock(templateStr, rangeStart)
		if rangeExpr.Text != "" {
			variable, collection := extractRangeVariables(rangeExpr.Text)
			constructs = append(constructs, &RangeConstruct{
				Variable:   variable,
				Collection: collection,
				Pos:        rangeStart,
			})
			i = rangeEnd
		} else {
			i = rangeStart + 8
		}
	}
	return constructs
}

// WithParser handles {{with}}...{{end}}
type WithParser struct{}

func (p *WithParser) FindConstructs(templateStr string) []Construct {
	var constructs []Construct
	i := 0
	for i < len(templateStr) {
		withStart := strings.Index(templateStr[i:], "{{with ")
		if withStart == -1 {
			break
		}
		withStart += i

		// Extract the complete with block
		withExpr, withEnd := extractWithBlock(templateStr, withStart)
		if withExpr.Text != "" {
			variable := extractWithVariable(withExpr.Text)
			constructs = append(constructs, &WithConstruct{
				Variable: variable,
				Pos:      withStart,
			})
			i = withEnd
		} else {
			i = withStart + 7
		}
	}
	return constructs
}

// TemplateInvokeParser handles {{template "name" .}}
type TemplateInvokeParser struct{}

func (p *TemplateInvokeParser) FindConstructs(templateStr string) []Construct {
	var constructs []Construct
	re := regexp.MustCompile(`\{\{\s*template\s+"([^"]+)"\s*([^}]*)\s*\}\}`)
	matches := re.FindAllStringSubmatchIndex(templateStr, -1)

	for _, match := range matches {
		name := templateStr[match[2]:match[3]]
		data := templateStr[match[4]:match[5]]
		constructs = append(constructs, &TemplateInvokeConstruct{
			Name: name,
			Data: data,
			Pos:  match[0],
		})
	}
	return constructs
}

// Helper functions for parsing

func isPartOfComplexConstruct(templateStr string, pos int) bool {
	// Look backwards for if/range/with keywords
	before := templateStr[:pos]
	if strings.Contains(before[max(0, len(before)-50):], "{{if ") ||
		strings.Contains(before[max(0, len(before)-50):], "{{range ") ||
		strings.Contains(before[max(0, len(before)-50):], "{{with ") {
		return true
	}
	return false
}

func extractCondition(conditionalText string) string {
	// Extract condition from {{if .Condition}}...{{end}}
	start := strings.Index(conditionalText, "{{if ")
	if start == -1 {
		return ""
	}
	end := strings.Index(conditionalText[start:], "}}")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(conditionalText[start+5 : start+end])
}

func extractRangeVariables(rangeText string) (string, string) {
	// Extract variables from {{range $var := .Collection}}
	start := strings.Index(rangeText, "{{range ")
	if start == -1 {
		return "", ""
	}
	end := strings.Index(rangeText[start:], "}}")
	if end == -1 {
		return "", ""
	}

	rangeHeader := strings.TrimSpace(rangeText[start+8 : start+end])
	if strings.Contains(rangeHeader, ":=") {
		parts := strings.Split(rangeHeader, ":=")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	// Simple range without variable assignment
	return "", strings.TrimSpace(rangeHeader)
}

func extractWithVariable(withText string) string {
	// Extract variable from {{with .Object}}
	start := strings.Index(withText, "{{with ")
	if start == -1 {
		return ""
	}
	end := strings.Index(withText[start:], "}}")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(withText[start+7 : start+end])
}

func extractWithBlock(templateStr string, start int) (TemplateExpression, int) {
	// Similar to extractRangeBlock but for {{with}}
	// This is a simplified version - full implementation would handle nesting
	withEnd := strings.Index(templateStr[start:], "{{end}}")
	if withEnd == -1 {
		return TemplateExpression{}, start
	}
	withEnd += start + 7

	return TemplateExpression{
		Text:  templateStr[start:withEnd],
		Type:  "with",
		Start: start,
		End:   withEnd,
	}, withEnd
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// parseRenderedWithPhoenixComprehensions implements render-then-parse with Phoenix comprehensions

// extractRangeBlock extracts a complete {{range}}...{{end}} block
func extractRangeBlock(templateStr string, start int) (TemplateExpression, int) {
	// Find the matching {{end}} for this range
	depth := 0
	i := start
	rangeContent := ""

	for i < len(templateStr) {
		if i+2 < len(templateStr) && templateStr[i:i+2] == "{{" {
			// Find the end of this template expression
			exprEnd := strings.Index(templateStr[i+2:], "}}")
			if exprEnd == -1 {
				break
			}
			exprEnd += i + 4

			expr := strings.TrimSpace(templateStr[i+2 : exprEnd-2])

			if strings.HasPrefix(expr, "range") || strings.HasPrefix(expr, "if") {
				depth++
			} else if expr == "end" {
				depth--
				if depth == 0 {
					// This is our matching {{end}}
					rangeContent = templateStr[start:exprEnd]
					return TemplateExpression{
						Text:  rangeContent,
						Type:  "range",
						Start: start,
						End:   exprEnd,
					}, exprEnd
				}
			}
			i = exprEnd
		} else {
			i++
		}
	}

	return TemplateExpression{}, start
}

// extractConditionalBlock extracts a complete {{if}}...{{end}} block
func extractConditionalBlock(templateStr string, start int) (TemplateExpression, int) {
	// Find the matching {{end}} for this conditional
	depth := 0
	i := start
	conditionalContent := ""

	for i < len(templateStr) {
		if i+2 < len(templateStr) && templateStr[i:i+2] == "{{" {
			// Find the end of this template expression
			exprEnd := strings.Index(templateStr[i+2:], "}}")
			if exprEnd == -1 {
				break
			}
			exprEnd += i + 4

			expr := strings.TrimSpace(templateStr[i+2 : exprEnd-2])

			if strings.HasPrefix(expr, "range") || strings.HasPrefix(expr, "if") {
				depth++
			} else if expr == "end" {
				depth--
				if depth == 0 {
					// This is our matching {{end}}
					conditionalContent = templateStr[start:exprEnd]
					return TemplateExpression{
						Text:  conditionalContent,
						Type:  "conditional",
						Start: start,
						End:   exprEnd,
					}, exprEnd
				}
			}
			i = exprEnd
		} else {
			i++
		}
	}

	return TemplateExpression{}, start
}

// buildTreeFromExpressions builds the tree by mapping rendered values to expression positions
func buildTreeFromExpressions(templateStr, rendered string, expressions []TemplateExpression, data interface{}, keyGen *KeyGenerator) (TreeNode, error) {
	tree := TreeNode{}
	var statics []string
	var dynamicIndex int

	currentPos := 0

	for _, expr := range expressions {
		// Add static content before this expression
		if expr.Start > currentPos {
			statics = append(statics, templateStr[currentPos:expr.Start])
		} else if len(statics) == dynamicIndex {
			statics = append(statics, "")
		}

		// Handle the expression based on type
		switch expr.Type {
		case "range":
			// Create Phoenix comprehension for range blocks
			comprehension, err := buildRangeComprehension(expr, data, keyGen)
			if err != nil {
				return nil, err
			}
			tree[fmt.Sprintf("%d", dynamicIndex)] = comprehension
			dynamicIndex++
			currentPos = expr.End

		case "conditional":
			// Evaluate the conditional expression
			value := evaluateConditionalBlock(expr, data)
			tree[fmt.Sprintf("%d", dynamicIndex)] = value
			dynamicIndex++
			currentPos = expr.End

		case "field":
			// Evaluate the field expression (need to wrap in {{}})
			templateText := fmt.Sprintf("{{%s}}", expr.Text)
			value := evaluateFieldExpression(templateText, data)
			tree[fmt.Sprintf("%d", dynamicIndex)] = fmt.Sprintf("%v", value)
			dynamicIndex++
			currentPos = expr.End
		}
	}

	// Add remaining static content
	if currentPos < len(templateStr) {
		statics = append(statics, templateStr[currentPos:])
	}

	// Ensure invariant: len(statics) = len(dynamics) + 1
	for len(statics) <= dynamicIndex {
		statics = append(statics, "")
	}

	// Minify static content to reduce size
	statics = minifyStatics(statics)
	tree["s"] = statics
	return tree, nil
}

// buildRangeComprehension creates a Phoenix comprehension for range expressions
func buildRangeComprehension(expr TemplateExpression, data interface{}, keyGen *KeyGenerator) (interface{}, error) {
	// Check if this is a Phoenix pattern (conditional wrapping a range)
	if strings.Contains(expr.Text, "{{if ") && strings.Contains(expr.Text, "{{range ") {
		return buildConditionalRange(expr, data, keyGen)
	}

	// Regular range comprehension
	return buildRegularRangeComprehension(expr, data, keyGen)
}

// buildPhoenixConditionalRangeComprehension handles {{if .Field}}...{{range}}...{{end}}...{{else}}...{{end}} patterns
func buildConditionalRange(expr TemplateExpression, data interface{}, keyGen *KeyGenerator) (interface{}, error) {
	// Extract the range field from the conditional-range pattern
	rangeField := extractRangeField(expr.Text)
	if rangeField == "" {
		return nil, fmt.Errorf("could not extract range field from Phoenix pattern: %s", expr.Text)
	}

	// Extract the slice data using reflection
	sliceData, err := getFieldValue(data, rangeField)
	if err != nil {
		return nil, fmt.Errorf("failed to get range data: %w", err)
	}

	// Handle empty slice case - return empty comprehension
	sliceValue := reflect.ValueOf(sliceData)
	if sliceValue.Kind() != reflect.Slice || sliceValue.Len() == 0 {
		return map[string]interface{}{
			"s": []string{},
			"d": []interface{}{},
		}, nil
	}

	// Extract wrapper HTML and range content
	prefixHTML, rangeContent, suffixHTML := extractRangeContentWithWrappers(expr.Text)

	// Fallback to simple extraction if wrapper extraction failed
	if rangeContent == "" {
		rangeContent = extractRangeContent(expr.Text)
	}

	// Check if wrapper HTML contains table tags
	hasTableWrapper := (strings.Contains(prefixHTML, "<table") || strings.Contains(prefixHTML, "<tbody") ||
		strings.Contains(prefixHTML, "<thead") || strings.Contains(prefixHTML, "<tfoot"))

	// Wrap range content with data-lvt-key wrapper div
	wrappedContent := wrapRangeContentWithKey(rangeContent)

	// Use generic template parsing to extract expressions from wrapped content
	innerExpressions := extractFlattenedExpressions(wrappedContent)

	// Extract static parts from wrapped content (includes wrapper div)
	var statics []string
	lastPos := 0
	for _, expr := range innerExpressions {
		// Add static part before this expression
		if expr.Start > lastPos {
			static := wrappedContent[lastPos:expr.Start]
			statics = append(statics, static)
		}
		lastPos = expr.End
	}
	// Add final static part
	if lastPos < len(wrappedContent) {
		statics = append(statics, wrappedContent[lastPos:])
	}

	// Include table wrapper HTML in statics if present
	// This ensures table/tbody/thead tags are included in the tree
	if hasTableWrapper && len(statics) > 0 {
		statics[0] = prefixHTML + statics[0]
		statics[len(statics)-1] = statics[len(statics)-1] + suffixHTML
	}

	// Replace key placeholder in statics with actual key injection point
	// The statics array will have the wrapper div structure with {{.__LVT_KEY__}} placeholder
	for i, static := range statics {
		if strings.Contains(static, "{{.__LVT_KEY__}}") {
			// Replace placeholder with template-like structure for key injection
			statics[i] = strings.ReplaceAll(static, "{{.__LVT_KEY__}}", "")
			// Split around the data-lvt-key attribute to create proper static segments
			if strings.Contains(statics[i], `data-lvt-key=""`) {
				parts := strings.Split(statics[i], `data-lvt-key=""`)
				if len(parts) == 2 {
					// Create new statics with key injection point
					newStatics := make([]string, 0, len(statics)+1)
					newStatics = append(newStatics, statics[:i]...)
					newStatics = append(newStatics, parts[0]+`data-lvt-key="`)
					newStatics = append(newStatics, `"`+parts[1])
					newStatics = append(newStatics, statics[i+1:]...)
					statics = newStatics
					break
				}
			}
		}
	}

	// Ensure we have at least one static part for the invariant
	if len(statics) == 0 {
		statics = []string{""}
	}

	// Process collection items with key injection
	var dynamics []map[string]interface{}
	for i := 0; i < sliceValue.Len(); i++ {
		item := sliceValue.Index(i).Interface()
		itemData := make(map[string]interface{})

		// First, evaluate all expressions to get the rendered content
		dynamicIndex := 0
		var keyFieldIndex int = -1

		// Process all expressions first to build the complete item
		for _, expr := range innerExpressions {
			// Skip the key placeholder expression for now
			if strings.Contains(expr.Text, ".__LVT_KEY__") {
				keyFieldIndex = dynamicIndex
				dynamicIndex++
				continue
			}

			// Evaluate expression in range context
			value := evaluateRangeExpression(expr.Text, item, i)
			itemData[fmt.Sprintf("%d", dynamicIndex)] = fmt.Sprintf("%v", value)
			dynamicIndex++
		}

		// Extract key from item data or generate fallback
		injectedKey := keyGen.extractKeyFromItem(item)

		// Add the key at the correct position
		if keyFieldIndex >= 0 {
			itemData[fmt.Sprintf("%d", keyFieldIndex)] = injectedKey
		}

		dynamics = append(dynamics, itemData)
	}

	// No need to store previous data with explicit key approach

	// Create the Phoenix comprehension structure
	comprehension := map[string]interface{}{
		"s": minifyStatics(statics),
		"d": dynamics,
	}

	return comprehension, nil
}

// buildRegularRangeComprehension handles regular {{range}}...{{end}} patterns
func buildRegularRangeComprehension(expr TemplateExpression, data interface{}, keyGen *KeyGenerator) (interface{}, error) {
	// Parse the range expression to find what field is being iterated
	rangeField := extractRangeFieldName(expr.Text)
	if rangeField == "" {
		return nil, fmt.Errorf("could not extract range field from: %s", expr.Text)
	}

	// Extract the slice data using reflection
	sliceData, err := getFieldValue(data, rangeField)
	if err != nil {
		return nil, fmt.Errorf("failed to get range data: %w", err)
	}

	sliceValue := reflect.ValueOf(sliceData)
	if sliceValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("range field %s is not a slice", rangeField)
	}

	// Convert slice to interface{} array
	var items []interface{}
	for i := 0; i < sliceValue.Len(); i++ {
		items = append(items, sliceValue.Index(i).Interface())
	}

	// Extract static parts from the range template content
	statics := []string{} // TODO: Replace with dynamic extraction

	// Generate dynamic data for each item
	dynamics := generateDynamicDataForItems(items, expr.Text)

	// Create the Phoenix comprehension structure
	comprehension := map[string]interface{}{
		"s": minifyStatics(statics),
		"d": dynamics,
	}

	return comprehension, nil
}

// extractRangeFieldFromPhoenixPattern extracts the field from a conditional-range pattern
func extractRangeField(conditionalText string) string {
	// Look for {{range $index, $todo := .Field}} pattern inside the conditional
	re := regexp.MustCompile(`\{\{\s*range\s+\$\w+,\s*\$\w+\s*:=\s*\.(\w+)`)
	matches := re.FindStringSubmatch(conditionalText)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback to simpler pattern {{range .Field}}
	re = regexp.MustCompile(`\{\{\s*range\s+\.(\w+)`)
	matches = re.FindStringSubmatch(conditionalText)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// extractRangeContentFromPhoenixPattern extracts just the range content from conditional-range pattern
func extractRangeContent(conditionalText string) string {
	// Find the range start
	rangeStart := strings.Index(conditionalText, "{{range ")
	if rangeStart == -1 {
		return ""
	}

	// Find the end of the range declaration {{range...}}
	rangeHeaderEnd := strings.Index(conditionalText[rangeStart:], "}}")
	if rangeHeaderEnd == -1 {
		return ""
	}
	rangeHeaderEnd += rangeStart + 2 // Move past the }}

	// Find the matching {{end}} for the range
	depth := 0
	i := rangeStart
	for i < len(conditionalText) {
		if i+2 < len(conditionalText) && conditionalText[i:i+2] == "{{" {
			// Find the end of this template expression
			exprEnd := strings.Index(conditionalText[i+2:], "}}")
			if exprEnd == -1 {
				break
			}
			exprEnd += i + 4
			expr := strings.TrimSpace(conditionalText[i+2 : exprEnd-2])

			if strings.HasPrefix(expr, "range") || strings.HasPrefix(expr, "if") {
				depth++
			} else if expr == "end" {
				depth--
				if depth == 0 { // depth 0 means we found the matching end for the range
					// Return just the content between {{range...}} and {{end}}
					return conditionalText[rangeHeaderEnd:i]
				}
			}
			i = exprEnd
		} else {
			i++
		}
	}

	return ""
}

// extractRangeContentWithWrappers extracts range content and wrapper HTML from conditional-range patterns
// Returns: (prefixHTML, rangeContent, suffixHTML)
// For {{if...}}<table><tbody>{{range...}}<tr>{{end}}</tbody></table>{{end}}
// Returns: ("<table><tbody>", "<tr>", "</tbody></table>")
func extractRangeContentWithWrappers(conditionalText string) (string, string, string) {
	// Find the {{if ...}} start and end
	ifStart := strings.Index(conditionalText, "{{if ")
	if ifStart == -1 {
		return "", "", ""
	}

	ifHeaderEnd := strings.Index(conditionalText[ifStart:], "}}")
	if ifHeaderEnd == -1 {
		return "", "", ""
	}
	ifHeaderEnd += ifStart + 2

	// Find the {{range...}} start
	rangeStart := strings.Index(conditionalText[ifHeaderEnd:], "{{range ")
	if rangeStart == -1 {
		return "", "", ""
	}
	rangeStart += ifHeaderEnd

	rangeHeaderEnd := strings.Index(conditionalText[rangeStart:], "}}")
	if rangeHeaderEnd == -1 {
		return "", "", ""
	}
	rangeHeaderEnd += rangeStart + 2

	// Find the matching {{end}} for the range
	depth := 0
	i := rangeStart
	rangeEndPos := -1
	for i < len(conditionalText) {
		if i+2 < len(conditionalText) && conditionalText[i:i+2] == "{{" {
			exprEnd := strings.Index(conditionalText[i+2:], "}}")
			if exprEnd == -1 {
				break
			}
			exprEnd += i + 4
			expr := strings.TrimSpace(conditionalText[i+2 : exprEnd-2])

			if strings.HasPrefix(expr, "range") || strings.HasPrefix(expr, "if") {
				depth++
			} else if expr == "end" {
				depth--
				if depth == 0 {
					rangeEndPos = i
					break
				}
			}
			i = exprEnd
		} else {
			i++
		}
	}

	if rangeEndPos == -1 {
		return "", "", ""
	}

	// Find the end of the range {{end}} tag
	rangeEndTagEnd := strings.Index(conditionalText[rangeEndPos:], "}}")
	if rangeEndTagEnd == -1 {
		return "", "", ""
	}
	rangeEndTagEnd += rangeEndPos + 2

	// Find the conditional's {{end}} or {{else}}
	// Start searching AFTER the range's {{end}} tag to find the conditional's {{end}}
	conditionalEndPos := -1
	depth = 0
	i = rangeEndTagEnd
	for i < len(conditionalText) {
		if i+2 < len(conditionalText) && conditionalText[i:i+2] == "{{" {
			exprEnd := strings.Index(conditionalText[i+2:], "}}")
			if exprEnd == -1 {
				break
			}
			exprEnd += i + 4
			expr := strings.TrimSpace(conditionalText[i+2 : exprEnd-2])

			if strings.HasPrefix(expr, "if") || strings.HasPrefix(expr, "range") {
				depth++
			} else if expr == "else" && depth == 0 {
				conditionalEndPos = i
				break
			} else if expr == "end" {
				if depth == 0 {
					conditionalEndPos = i
					break
				}
				depth--
			}
			i = exprEnd
		} else {
			i++
		}
	}

	if conditionalEndPos == -1 {
		return "", "", ""
	}

	// Extract the three parts
	prefixHTML := conditionalText[ifHeaderEnd:rangeStart]
	rangeContent := conditionalText[rangeHeaderEnd:rangeEndPos]

	suffixHTML := conditionalText[rangeEndTagEnd:conditionalEndPos]

	return prefixHTML, rangeContent, suffixHTML
}

// extractRangeFieldName extracts the field name from a range expression
func extractRangeFieldName(rangeText string) string {
	// Look for patterns like "{{range .Collection}}" in the range text
	re := regexp.MustCompile(`\{\{\s*range\s+\.(\w+)`)
	matches := re.FindStringSubmatch(rangeText)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// getFieldValue gets a field value from data using reflection
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

// generateDynamicDataForItems generates dynamic data for each item in the range
func generateDynamicDataForItems(items []interface{}, rangeText string) []map[string]interface{} {
	var dynamics []map[string]interface{}

	// Extract the template expressions from the range content
	rangeStartIdx := strings.Index(rangeText, "}}")
	if rangeStartIdx == -1 {
		return dynamics
	}
	rangeStartIdx += 2

	endIdx := strings.LastIndex(rangeText, "{{end")
	if endIdx == -1 {
		return dynamics
	}

	content := rangeText[rangeStartIdx:endIdx]

	// Find all template expressions
	re := regexp.MustCompile(`\{\{[^}]*\}\}`)
	expressions := re.FindAllString(content, -1)

	// Generate data for each item
	for _, item := range items {
		itemData := make(map[string]interface{})

		for j, expr := range expressions {
			// Evaluate the expression in the context of this item
			value := evaluateTemplateExpression(expr, item)
			itemData[fmt.Sprintf("%d", j)] = value
		}

		dynamics = append(dynamics, itemData)
	}

	return dynamics
}

// evaluateConditionalBlock evaluates a conditional if-else-end block
func evaluateConditionalBlock(expr TemplateExpression, data interface{}) string {
	// Now expr.Text contains the full conditional block like:
	// "{{if gt .Value 5}}active{{else}}inactive{{end}}"

	// Parse and execute the complete conditional template
	tmpl, err := template.New("conditional").Parse(expr.Text)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(buf.String())
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

// generateShortUUID creates a short random UUID for fallback keys
func generateShortUUID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// extractKeyFromItem extracts key from item data structure
func (kg *KeyGenerator) extractKeyFromItem(item interface{}) string {
	// Try to extract key from the data structure
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// Look for common key field names
		for _, fieldName := range []string{"ID", "Id", "Key", "LvtKey", "DataKey"} {
			field := v.FieldByName(fieldName)
			if field.IsValid() && field.CanInterface() {
				if keyStr := fmt.Sprintf("%v", field.Interface()); keyStr != "" && keyStr != "<nil>" {
					return keyStr
				}
			}
		}
	} else if v.Kind() == reflect.Map {
		// Look for key in map
		mapValue := v
		for _, keyName := range []string{"id", "ID", "key", "Key", "lvt_key", "data_key"} {
			if mapValue.MapIndex(reflect.ValueOf(keyName)).IsValid() {
				val := mapValue.MapIndex(reflect.ValueOf(keyName))
				if keyStr := fmt.Sprintf("%v", val.Interface()); keyStr != "" && keyStr != "<nil>" {
					return keyStr
				}
			}
		}
	}

	// Fallback: generate random UUID
	return generateShortUUID()
}

// extractKeyFromRangeItem extracts explicit key from template or generates fallback
func (kg *KeyGenerator) extractKeyFromRangeItem(itemHTML string, itemData interface{}) string {
	// Try each configured attribute name in order
	for _, attrName := range kg.keyConfig.AttributeNames {
		pattern := fmt.Sprintf(`\s%s="([^"]*)"`, regexp.QuoteMeta(attrName))
		re := regexp.MustCompile(pattern)

		if matches := re.FindStringSubmatch(itemHTML); len(matches) > 1 {
			key := matches[1]
			// If it's a template expression, evaluate it
			if strings.Contains(key, "{{") && strings.Contains(key, "}}") {
				// Extract the field expression and evaluate it
				return kg.evaluateKeyExpression(key, itemData)
			}
			return key
		}
	}

	// No explicit key found - generate random UUID fallback
	return generateShortUUID()
}

// evaluateKeyExpression evaluates a Go template expression for key
func (kg *KeyGenerator) evaluateKeyExpression(expr string, itemData interface{}) string {
	// Simple expression evaluation for {{.Field}} patterns
	re := regexp.MustCompile(`{{\.(\w+)}}`)
	matches := re.FindStringSubmatch(expr)
	if len(matches) > 1 {
		fieldName := matches[1]

		// Use reflection to get the field value
		v := reflect.ValueOf(itemData)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() == reflect.Struct {
			field := v.FieldByName(fieldName)
			if field.IsValid() {
				return fmt.Sprintf("%v", field.Interface())
			}
		}

		// If it's a map, try map access
		if v.Kind() == reflect.Map {
			mapVal := v.MapIndex(reflect.ValueOf(fieldName))
			if mapVal.IsValid() {
				return fmt.Sprintf("%v", mapVal.Interface())
			}
		}
	}

	// Fallback - return the expression as-is or generate UUID
	return generateShortUUID()
}

// getOrGenerateKey gets key for position or generates fallback
func (kg *KeyGenerator) getOrGenerateKey(position int, explicitKey string) string {
	if explicitKey != "" && !kg.usedKeys[explicitKey] {
		kg.usedKeys[explicitKey] = true
		return explicitKey
	}

	// Check for existing fallback key at this position
	if position < len(kg.fallbackKeys) && kg.fallbackKeys[position] != "" {
		key := kg.fallbackKeys[position]
		if !kg.usedKeys[key] {
			kg.usedKeys[key] = true
			return key
		}
	}

	// Generate new random key
	key := generateShortUUID()
	for kg.usedKeys[key] {
		key = generateShortUUID()
	}
	kg.usedKeys[key] = true

	// Expand slice if needed
	for len(kg.fallbackKeys) <= position {
		kg.fallbackKeys = append(kg.fallbackKeys, "")
	}
	kg.fallbackKeys[position] = key

	return key
}

// renderItemDataToHTML converts item data to HTML representation
func (kg *KeyGenerator) renderItemDataToHTML(itemData map[string]interface{}) string {
	// This is a simplified HTML reconstruction
	// In a real implementation, this would use the actual template structure
	var parts []string

	// Add the key as data-lvt-key attribute
	if keyValue, exists := itemData["0"]; exists {
		parts = append(parts, fmt.Sprintf(`<div data-lvt-key="%v">`, keyValue))
	} else {
		parts = append(parts, `<div>`)
	}

	// Add content fields in order
	for i := 1; i < 10; i++ { // Reasonable upper bound
		if value, exists := itemData[fmt.Sprintf("%d", i)]; exists {
			parts = append(parts, fmt.Sprintf("%v", value))
		}
	}

	parts = append(parts, `</div>`)
	return strings.Join(parts, "")
}

// extractKeyFromHTML uses regex to extract data-lvt-key value from HTML
func (kg *KeyGenerator) extractKeyFromHTML(html string) string {
	re := regexp.MustCompile(`data-lvt-key="([^"]*)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// removeKeyFromHTML removes data-lvt-key attributes for content comparison
func (kg *KeyGenerator) removeKeyFromHTML(html string) string {
	re := regexp.MustCompile(`\s*data-lvt-key="[^"]*"`)
	return re.ReplaceAllString(html, "")
}

// htmlContentMatches compares HTML content ignoring keys and whitespace
func (kg *KeyGenerator) htmlContentMatches(html1, html2 string) bool {
	// Normalize whitespace and compare
	normalize := func(s string) string {
		return regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(s), " ")
	}

	return normalize(html1) == normalize(html2)
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

// detectAndReuseKeyAttribute checks if the root element already has a key attribute
// (data-key, key, or id) with a template expression, and if so, replaces it with {{.__LVT_KEY__}}
// Returns the modified content and true if a key was found and reused, empty string and false otherwise
func detectAndReuseKeyAttribute(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)

	// Only process if there's a single root element
	if !hasSingleRootElement(trimmed) {
		return "", false
	}

	// Find the opening tag
	endOfTag := strings.Index(trimmed, ">")
	if endOfTag <= 0 {
		return "", false
	}

	openTag := trimmed[:endOfTag]
	rest := trimmed[endOfTag:]

	// Priority order: data-lvt-key (already optimized), data-key, key, id
	// Pattern matches: data-key="{{.Field}}" or key="{{expr}}" or id="{{...}}"
	keyAttrs := []string{"data-lvt-key", "data-key", "key", "id"}

	for _, attr := range keyAttrs {
		// Build regex pattern for this attribute
		// Matches: attr="{{...}}" (with template expression)
		pattern := regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*=\s*"(\{\{[^}]+\}\})"`, regexp.QuoteMeta(attr)))

		if matches := pattern.FindStringSubmatch(openTag); len(matches) > 1 {
			// Found a key attribute with template expression
			// Replace the template expression with {{.__LVT_KEY__}}
			newOpenTag := pattern.ReplaceAllString(openTag, fmt.Sprintf(`%s="{{.__LVT_KEY__}}"`, attr))
			return newOpenTag + rest, true
		}
	}

	return "", false
}

// wrapRangeContentWithKey wraps range content with a data-lvt-key wrapper div
// or injects the key into the root element if there's a single root element
func wrapRangeContentWithKey(content string) string {
	trimmed := strings.TrimSpace(content)

	// First, try to detect and reuse existing key attributes
	if reusedContent, found := detectAndReuseKeyAttribute(trimmed); found {
		return reusedContent
	}

	// Check if content has a single root HTML element
	if hasSingleRootElement(trimmed) {
		// Find the end of the opening tag
		endOfTag := strings.Index(trimmed, ">")
		if endOfTag > 0 {
			// Inject data-lvt-key attribute into the root element
			openTag := trimmed[:endOfTag]
			rest := trimmed[endOfTag:]
			return fmt.Sprintf(`%s data-lvt-key="{{.__LVT_KEY__}}"%s`, openTag, rest)
		}
	}

	// Default: wrap content in a div with a placeholder for the key
	return fmt.Sprintf(`<div data-lvt-key="{{.__LVT_KEY__}}">%s</div>`, content)
}

// hasSingleRootElement checks if HTML content has a single root element
func hasSingleRootElement(html string) bool {
	html = strings.TrimSpace(html)

	// Must start with an HTML tag
	if !strings.HasPrefix(html, "<") || strings.HasPrefix(html, "<!") {
		return false
	}

	// Find the tag name
	endOfTagName := strings.IndexAny(html[1:], " \t\n\r>")
	if endOfTagName == -1 {
		return false
	}

	tagName := html[1 : endOfTagName+1]

	// Self-closing tag (e.g., <img />, <br />)
	if strings.HasSuffix(strings.TrimSpace(html), "/>") {
		return true
	}

	// Find the closing tag
	closingTag := fmt.Sprintf("</%s>", tagName)
	closingIdx := strings.LastIndex(html, closingTag)

	if closingIdx == -1 {
		return false
	}

	// Check if anything significant exists after the closing tag
	afterClosing := strings.TrimSpace(html[closingIdx+len(closingTag):])

	// Only whitespace or comments after closing tag means single root element
	return afterClosing == "" || strings.HasPrefix(afterClosing, "<!--")
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

// evaluateConditionalExpression evaluates a conditional expression and returns the result
func evaluateConditionalExpression(expr string, data interface{}) string {
	// Create a template with the conditional
	tmplText := "{{" + expr + "}}"
	tmpl, err := template.New("conditional").Parse(tmplText)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return ""
	}

	return buf.String()
}

// evaluateTemplateExpression evaluates a template expression with given data
func evaluateTemplateExpression(expr string, data interface{}) interface{} {
	// Handle range blocks specially
	if strings.Contains(expr, "range") {
		return evaluateRangeBlock(expr, data)
	}

	// Handle conditional blocks
	if strings.Contains(expr, "if ") {
		// Create a TemplateExpression for the old interface
		tempExpr := TemplateExpression{
			Text: expr,
			Type: "conditional",
		}
		return evaluateConditionalBlock(tempExpr, data)
	}

	// Handle simple field expressions like {{.Name}}
	return evaluateFieldExpression(expr, data)
}

// evaluateRangeBlock evaluates a range block and returns rendered content
func evaluateRangeBlock(rangeExpr string, data interface{}) interface{} {
	// Execute just this range block to get its rendered output
	tmpl, err := template.New("range").Parse(rangeExpr)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return ""
	}

	return buf.String()
}

// evaluateRangeExpression evaluates expressions in the context of a range loop
func evaluateRangeExpression(expr string, item interface{}, index int) interface{} {
	// Keep original expression for conditionals
	originalExpr := strings.TrimSpace(expr)

	// Clean the expression for simple processing
	cleanExpr := originalExpr
	if strings.HasPrefix(cleanExpr, "{{") && strings.HasSuffix(cleanExpr, "}}") {
		cleanExpr = strings.TrimSpace(cleanExpr[2 : len(cleanExpr)-2])
	}

	// Handle index formatting: $index | printf "#%d"
	if strings.Contains(cleanExpr, "$index") && strings.Contains(cleanExpr, "printf") {
		if strings.Contains(cleanExpr, "#%d") {
			return fmt.Sprintf("#%d", index)
		}
		return fmt.Sprintf("%d", index)
	}

	// Handle field access on the range item
	if strings.HasPrefix(cleanExpr, ".") {
		return evaluateFieldExpression("{{"+cleanExpr+"}}", item)
	}

	// Handle conditionals in range context - use original expression with braces
	if strings.Contains(cleanExpr, "if ") {
		return evaluateConditionalInRangeContext(originalExpr, item)
	}

	// Default: return the expression as-is if we can't evaluate it
	return cleanExpr
}

// extractFieldFromCondition extracts the field name from a conditional expression like {{if .FieldName}}
func extractFieldFromCondition(expr string) string {
	// Look for pattern {{if .FieldName}}
	start := strings.Index(expr, "{{if .")
	if start == -1 {
		return ""
	}

	// Find the end of the field name (could be }}, space, or comparison operator)
	fieldStart := start + 6 // len("{{if .")
	fieldEnd := fieldStart

	for fieldEnd < len(expr) {
		ch := expr[fieldEnd]
		if ch == '}' || ch == ' ' || ch == '=' || ch == '!' || ch == '<' || ch == '>' {
			break
		}
		fieldEnd++
	}

	if fieldEnd > fieldStart {
		return expr[fieldStart:fieldEnd]
	}
	return ""
}

// evaluateCondition evaluates whether a value is "truthy" for conditionals
func evaluateCondition(value interface{}) bool {
	if value == nil {
		return false
	}

	// Handle different types
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0.0
	default:
		// For slices, arrays, maps, check if not empty
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		case reflect.Ptr:
			return !rv.IsNil()
		default:
			// Default: non-nil values are truthy
			return true
		}
	}
}

// evaluateEmbeddedFields evaluates field references within conditional content
func evaluateEmbeddedFields(content string, item interface{}) interface{} {
	// If the content contains {{.Field}}, evaluate it
	if strings.Contains(content, "{{.") && strings.Contains(content, "}}") {
		// Find and evaluate field expressions
		result := content

		// Find all {{.Field}} patterns
		re := regexp.MustCompile(`\{\{\.(\w+)\}\}`)
		matches := re.FindAllStringSubmatch(content, -1)

		for _, match := range matches {
			if len(match) >= 2 {
				fieldName := match[1]
				fieldValue, err := getFieldValue(item, fieldName)
				if err == nil && fieldValue != nil {
					// Replace the pattern with the field value
					result = strings.Replace(result, match[0], fmt.Sprintf("%v", fieldValue), -1)
				}
			}
		}

		return result
	}

	// Return content as-is if no field references
	return content
}

// evaluateConditionalInRangeContext evaluates conditionals within range items generically
func evaluateConditionalInRangeContext(expr string, item interface{}) interface{} {
	// Generic pattern matching for {{if .FieldName}}...{{else}}...{{end}}
	if strings.Contains(expr, "{{if ") && strings.Contains(expr, "{{else}}") {
		// Extract the field name from {{if .FieldName}}
		fieldName := extractFieldFromCondition(expr)
		if fieldName == "" {
			return ""
		}

		// Get field value dynamically
		fieldValue, err := getFieldValue(item, fieldName)
		if err != nil {
			return ""
		}

		// Evaluate the condition based on the field value
		conditionTrue := evaluateCondition(fieldValue)

		if conditionTrue {
			// Extract content between }}...{{else}}
			start := strings.Index(expr, "}}")
			end := strings.Index(expr, "{{else}}")
			if start != -1 && end != -1 && start < end {
				content := strings.TrimSpace(expr[start+2 : end])
				// If content contains field references, evaluate them
				return evaluateEmbeddedFields(content, item)
			}
		} else {
			// Extract content between {{else}}...{{end}}
			start := strings.Index(expr, "{{else}}")
			end := strings.Index(expr, "{{end}}")
			if start != -1 && end != -1 && start < end {
				content := strings.TrimSpace(expr[start+8 : end])
				// If content contains field references, evaluate them
				return evaluateEmbeddedFields(content, item)
			}
		}
		return ""
	}

	// Generic pattern matching for {{if .FieldName}}...{{end}} (no else clause)
	if strings.Contains(expr, "{{if ") && strings.Contains(expr, "{{end}}") {
		// Extract the field name from {{if .FieldName}}
		fieldName := extractFieldFromCondition(expr)
		if fieldName == "" {
			return ""
		}

		// Get field value dynamically
		fieldValue, err := getFieldValue(item, fieldName)
		if err != nil {
			return ""
		}

		// Evaluate the condition based on the field value
		if evaluateCondition(fieldValue) {
			// Extract content between }}...{{end}}
			start := strings.Index(expr, "}}")
			end := strings.Index(expr, "{{end}}")
			if start != -1 && end != -1 && start < end {
				content := strings.TrimSpace(expr[start+2 : end])
				// If content contains field references, evaluate them
				return evaluateEmbeddedFields(content, item)
			}
		}
		return ""
	}

	return ""
}

// evaluateFieldExpression evaluates a simple field expression like {{.Name}}
func evaluateFieldExpression(expr string, data interface{}) interface{} {
	// Execute just this field expression
	tmpl, err := template.New("field").Parse(expr)
	if err != nil {
		return ""
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return ""
	}

	return buf.String()
}

// parseTemplateHybrid handles both field expressions and range blocks in a unified way
func parseTemplateHybrid(templateStr string, data interface{}) (TreeNode, error) {
	result := make(TreeNode)

	// For templates with ranges, we need special handling but still extract individual fields
	// Check if this is a complex mixed template (has both ranges and individual fields)
	hasRange := strings.Contains(templateStr, "{{range")
	hasFields := countFieldExpressions(templateStr) > 0

	if hasRange && hasFields {
		// This is a complex mixed template - need to extract individual fields
		// while handling ranges appropriately
		return parseComplexMixedTemplate(templateStr, data)
	} else if hasRange {
		// Pure range template
		return parseTemplateWithRange(templateStr, data)
	}

	// Simple field template
	var staticParts []string
	var dynamicSegments []interface{}

	// First pass: identify all template expressions and their positions
	exprRegex := regexp.MustCompile(`\{\{[^}]+\}\}`)
	expressions := exprRegex.FindAllStringSubmatchIndex(templateStr, -1)

	currentPos := 0

	for _, match := range expressions {
		exprStart := match[0]
		exprEnd := match[1]
		expr := templateStr[exprStart:exprEnd]
		cleanExpr := strings.TrimSpace(expr[2 : len(expr)-2])

		// Skip control flow expressions (if, else, with, etc.)
		if strings.HasPrefix(cleanExpr, "if ") ||
			strings.HasPrefix(cleanExpr, "else") ||
			strings.HasPrefix(cleanExpr, "with ") ||
			strings.HasPrefix(cleanExpr, "define ") ||
			strings.HasPrefix(cleanExpr, "template ") ||
			strings.HasPrefix(cleanExpr, "block ") {
			continue
		}

		// This is a field expression - evaluate it
		value := evaluateTemplateExpression(cleanExpr, data)
		if value != nil {
			// Add static part before this expression
			if exprStart > currentPos {
				staticParts = append(staticParts, templateStr[currentPos:exprStart])
			} else if len(staticParts) == len(dynamicSegments) {
				// Need to add empty static part to maintain invariant
				staticParts = append(staticParts, "")
			}

			// Add dynamic value
			dynamicSegments = append(dynamicSegments, fmt.Sprintf("%v", value))

			currentPos = exprEnd
		}
	}

	// Add final static part
	if currentPos < len(templateStr) {
		staticParts = append(staticParts, templateStr[currentPos:])
	}

	// Ensure invariant: len(statics) == len(dynamics) + 1
	if len(staticParts) == len(dynamicSegments) {
		// Need one more static part
		staticParts = append(staticParts, "")
	} else if len(staticParts) < len(dynamicSegments) {
		// Add empty static parts to reach proper count
		for len(staticParts) < len(dynamicSegments)+1 {
			staticParts = append(staticParts, "")
		}
	} else if len(staticParts) > len(dynamicSegments)+1 {
		// Too many static parts - merge consecutive ones
		for len(staticParts) > len(dynamicSegments)+1 && len(staticParts) > 1 {
			lastIdx := len(staticParts) - 1
			staticParts[lastIdx-1] = staticParts[lastIdx-1] + staticParts[lastIdx]
			staticParts = staticParts[:lastIdx]
		}
	}

	// Final validation
	if len(staticParts) != len(dynamicSegments)+1 {
		return nil, fmt.Errorf("invariant violation: len(statics)=%d, len(dynamics)+1=%d",
			len(staticParts), len(dynamicSegments)+1)
	}

	// Minify static parts to reduce bandwidth
	minifiedStatics := make([]string, len(staticParts))
	for i, static := range staticParts {
		minifiedStatics[i] = minifyHTML(static)
	}

	// Build result
	result["s"] = minifiedStatics
	for i, segment := range dynamicSegments {
		result[strconv.Itoa(i)] = segment
	}

	return result, nil
}

// countFieldExpressions counts individual field expressions outside of range blocks
func countFieldExpressions(templateStr string) int {
	exprRegex := regexp.MustCompile(`\{\{[^}]+\}\}`)
	expressions := exprRegex.FindAllStringSubmatchIndex(templateStr, -1)

	// Track range blocks to exclude expressions inside them
	rangeDepth := 0
	count := 0

	for _, match := range expressions {
		expr := templateStr[match[0]:match[1]]
		cleanExpr := strings.TrimSpace(expr[2 : len(expr)-2])

		// Track range depth
		if strings.HasPrefix(cleanExpr, "range ") {
			rangeDepth++
			continue
		} else if cleanExpr == "end" {
			rangeDepth--
			continue
		}

		// Skip expressions inside range blocks
		if rangeDepth > 0 {
			continue
		}

		// Skip control flow expressions
		if strings.HasPrefix(cleanExpr, "if ") ||
			strings.HasPrefix(cleanExpr, "else") ||
			strings.HasPrefix(cleanExpr, "with ") ||
			strings.HasPrefix(cleanExpr, "define ") ||
			strings.HasPrefix(cleanExpr, "template ") ||
			strings.HasPrefix(cleanExpr, "block ") {
			continue
		}

		// This is a top-level field expression
		count++
	}
	return count
}

// parseComplexMixedTemplate handles templates with both ranges and individual fields
// For complex templates, we need to render the template and extract pure HTML
func parseComplexMixedTemplate(templateStr string, data interface{}) (TreeNode, error) {
	// The issue is that we can't leave template expressions in static parts
	// We need to render the template to get pure HTML
	// But this means the structure becomes data-dependent
	// For now, fallback to simple parsing
	return parseTemplateHybrid(templateStr, data)
}

// findMatchingEndForExpression finds the matching {{end}} for a control structure
func findMatchingEndForExpression(templateStr string, startPos int) int {
	depth := 1

	exprRegex := regexp.MustCompile(`\{\{[^}]+\}\}`)
	remaining := templateStr[startPos:]
	matches := exprRegex.FindAllStringSubmatchIndex(remaining, -1)

	for _, match := range matches[1:] { // Skip the first match (our starting expression)
		expr := remaining[match[0]:match[1]]
		cleanExpr := strings.TrimSpace(expr[2 : len(expr)-2])

		if strings.HasPrefix(cleanExpr, "if ") || strings.HasPrefix(cleanExpr, "range ") {
			depth++
		} else if cleanExpr == "end" {
			depth--
			if depth == 0 {
				return startPos + match[1]
			}
		}
	}

	return -1
}

// parseTemplateUsingRenderedHTML uses the actual rendered HTML to extract static/dynamic parts

// parseTemplateWithRange handles range templates specifically
func parseTemplateWithRange(templateStr string, data interface{}) (TreeNode, error) {
	result := make(TreeNode)

	// Find range block
	rangeStart := strings.Index(templateStr, "{{range")
	if rangeStart == -1 {
		return parseTemplateHybrid(templateStr, data)
	}

	// Extract range expression first
	rangeExprEnd := strings.Index(templateStr[rangeStart:], "}}")
	if rangeExprEnd == -1 {
		return nil, fmt.Errorf("malformed range expression")
	}

	// Find the end of range (start searching after the range header)
	rangeHeaderEnd := rangeStart + rangeExprEnd + 2
	rangeEndPos := findMatchingEnd(templateStr, rangeHeaderEnd)
	if rangeEndPos == -1 {
		return nil, fmt.Errorf("unmatched range block")
	}
	rangeExpr := templateStr[rangeStart+7 : rangeStart+rangeExprEnd] // Skip "{{range "
	rangeExpr = strings.TrimSpace(rangeExpr)

	// Static parts: before range, after range
	staticBefore := templateStr[:rangeStart]
	staticAfter := templateStr[rangeEndPos+7:] // Skip "{{end}}"

	staticParts := []string{staticBefore, staticAfter}

	// Evaluate range data
	rangeData := evaluateFieldAccess(rangeExpr, data)
	var dynamicSegments []interface{}

	if rangeData != nil {
		// Process range items - for now, serialize the range content
		// This maintains the invariant while providing range support
		switch v := rangeData.(type) {
		case []interface{}:
			if len(v) > 0 {
				// Generate content for range items
				rangeContentTemplate := templateStr[rangeHeaderEnd:rangeEndPos]
				var rangeOutput strings.Builder

				for _, item := range v {
					// Execute template content for each item
					rendered := executeTemplateContent(rangeContentTemplate, item)
					rangeOutput.WriteString(rendered)
				}
				dynamicSegments = append(dynamicSegments, rangeOutput.String())
			} else {
				dynamicSegments = append(dynamicSegments, "")
			}
		default:
			// Try to convert to slice using reflection
			rv := reflect.ValueOf(rangeData)
			if rv.Kind() == reflect.Slice {
				var rangeOutput strings.Builder
				rangeContentTemplate := templateStr[rangeHeaderEnd:rangeEndPos]

				for i := 0; i < rv.Len(); i++ {
					item := rv.Index(i).Interface()
					rendered := executeTemplateContent(rangeContentTemplate, item)
					rangeOutput.WriteString(rendered)
				}
				dynamicSegments = append(dynamicSegments, rangeOutput.String())
			} else {
				dynamicSegments = append(dynamicSegments, "")
			}
		}
	} else {
		dynamicSegments = append(dynamicSegments, "")
	}

	// Build result maintaining invariant len(statics) == len(dynamics) + 1
	// Minify static parts to reduce bandwidth
	minifiedStatics := make([]string, len(staticParts))
	for i, static := range staticParts {
		minifiedStatics[i] = minifyHTML(static)
	}

	result["s"] = minifiedStatics
	for i, segment := range dynamicSegments {
		result[strconv.Itoa(i)] = segment
	}

	return result, nil
}

// executeTemplateContent executes a template fragment with given data
func executeTemplateContent(templateContent string, data interface{}) string {
	// For simple execution, just replace template expressions with their values
	result := templateContent

	// Find all template expressions
	exprRegex := regexp.MustCompile(`\{\{[^}]+\}\}`)
	expressions := exprRegex.FindAllString(templateContent, -1)

	for _, expr := range expressions {
		cleanExpr := strings.TrimSpace(expr[2 : len(expr)-2])

		// Skip control flow expressions
		if strings.HasPrefix(cleanExpr, "if ") ||
			strings.HasPrefix(cleanExpr, "else") ||
			cleanExpr == "end" {
			continue
		}

		// Evaluate expression
		value := evaluateTemplateExpression(cleanExpr, data)
		if value != nil {
			result = strings.ReplaceAll(result, expr, fmt.Sprintf("%v", value))
		}
	}

	return result
}

// isSimpleRangeTemplate checks if template is primarily a range template

// RangeBlock represents a {{range}} block in the template
type RangeBlock struct {
	Start     int    // Start position in template
	End       int    // End position in template
	Variable  string // Range variable (e.g., ".Collection")
	Content   string // Content inside the range block
	FullBlock string // Full range block including {{range}} and {{end}}
}

// findMatchingEnd finds the {{end}} that matches a {{range}}
func findMatchingEnd(templateStr string, startPos int) int {
	depth := 1
	pos := startPos

	for pos < len(templateStr) && depth > 0 {
		// Look for next template expression
		nextExpr := strings.Index(templateStr[pos:], "{{")
		if nextExpr == -1 {
			break
		}

		pos += nextExpr
		exprEnd := strings.Index(templateStr[pos:], "}}")
		if exprEnd == -1 {
			break
		}

		expr := templateStr[pos : pos+exprEnd+2]
		cleanExpr := strings.TrimSpace(expr[2 : len(expr)-2])

		if strings.HasPrefix(cleanExpr, "range ") || strings.HasPrefix(cleanExpr, "if ") || strings.HasPrefix(cleanExpr, "with ") {
			depth++
		} else if cleanExpr == "end" {
			depth--
		}

		if depth == 0 {
			return pos
		}

		pos += exprEnd + 2
	}

	return -1
}

// parseTemplateWithRanges handles templates that contain range blocks

// evaluateFieldAccess evaluates simple field access like .Field or .Field.SubField
func evaluateFieldAccess(expr string, data interface{}) interface{} {
	// Remove the leading dot
	fieldPath := expr[1:]

	// Handle nested field access
	fields := strings.Split(fieldPath, ".")

	value := reflect.ValueOf(data)
	for value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
		value = value.Elem()
	}

	for _, field := range fields {
		if field == "" {
			continue
		}

		if value.Kind() == reflect.Struct {
			value = value.FieldByName(field)
			if !value.IsValid() {
				return nil
			}
		} else if value.Kind() == reflect.Map {
			mapValue := value.MapIndex(reflect.ValueOf(field))
			if !mapValue.IsValid() {
				return nil
			}
			value = mapValue
		} else {
			return nil
		}
	}

	if value.IsValid() {
		return value.Interface()
	}

	return nil
}
