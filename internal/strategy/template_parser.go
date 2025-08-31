package strategy

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// TemplateParser provides simplified template parsing for strategy selection
type TemplateParser struct{}

// NewTemplateParser creates a new template parser
func NewTemplateParser() *TemplateParser {
	return &TemplateParser{}
}

// ParseBoundaries parses template source and returns boundaries
func (tp *TemplateParser) ParseBoundaries(templateSource string) ([]TemplateBoundary, error) {
	// Find all template actions using regex
	actionRegex := regexp.MustCompile(`\{\{[^}]*\}\}`)
	matches := actionRegex.FindAllStringSubmatchIndex(templateSource, -1)

	if len(matches) == 0 {
		// Template has only static content
		return []TemplateBoundary{{
			Type:    StaticContent,
			Content: templateSource,
			Start:   0,
			End:     len(templateSource),
		}}, nil
	}

	var boundaries []TemplateBoundary
	lastEnd := 0

	for _, match := range matches {
		start, end := match[0], match[1]
		action := templateSource[start:end]

		// Add static content before this action
		if start > lastEnd {
			staticContent := templateSource[lastEnd:start]
			if staticContent != "" {
				boundaries = append(boundaries, TemplateBoundary{
					Type:    StaticContent,
					Content: staticContent,
					Start:   lastEnd,
					End:     start,
				})
			}
		}

		// Parse the action
		actionType, fieldPath := tp.parseAction(action)
		boundary := TemplateBoundary{
			Type:      actionType,
			Content:   action,
			Start:     start,
			End:       end,
			FieldPath: fieldPath,
		}

		boundaries = append(boundaries, boundary)
		lastEnd = end
	}

	// Add final static content
	if lastEnd < len(templateSource) {
		staticContent := templateSource[lastEnd:]
		if staticContent != "" {
			boundaries = append(boundaries, TemplateBoundary{
				Type:    StaticContent,
				Content: staticContent,
				Start:   lastEnd,
				End:     len(templateSource),
			})
		}
	}

	return boundaries, nil
}

// parseAction classifies a template action and extracts field path if applicable
func (tp *TemplateParser) parseAction(action string) (TemplateBoundaryType, string) {
	// Remove {{ and }} delimiters
	inner := strings.TrimSpace(action[2 : len(action)-2])

	// Comments
	if strings.HasPrefix(inner, "/*") && strings.HasSuffix(inner, "*/") {
		return Comment, ""
	}

	// Template definitions
	if strings.HasPrefix(inner, "define ") {
		return TemplateDefinition, ""
	}

	// Control structures
	if strings.HasPrefix(inner, "if ") {
		return ConditionalIf, inner[3:]
	}
	if strings.HasPrefix(inner, "range ") {
		return RangeLoop, inner[6:]
	}
	if strings.HasPrefix(inner, "with ") {
		return ContextWith, inner[5:]
	}
	if inner == "else" {
		return ConditionalElse, ""
	}
	if inner == "end" {
		return ConditionalEnd, ""
	}

	// Variable assignments
	if strings.Contains(inner, ":=") || strings.Contains(inner, "=") {
		return Variable, ""
	}

	// Template invocations
	if strings.HasPrefix(inner, "template ") {
		return TemplateInvocation, ""
	}

	// Block definitions
	if strings.HasPrefix(inner, "block ") {
		return BlockDefinition, ""
	}

	// Pipelines and functions
	if strings.Contains(inner, "|") {
		return Pipeline, ""
	}

	// Function calls (contains parentheses or multiple words)
	if strings.Contains(inner, "(") || len(strings.Fields(inner)) > 1 {
		return Function, ""
	}

	// Loop control
	if inner == "break" || inner == "continue" {
		return LoopControl, ""
	}

	// Simple field access - check if it looks like a field path
	if tp.isSimpleFieldPath(inner) {
		return SimpleField, inner
	}

	// Everything else is complex
	return Complex, ""
}

// isSimpleFieldPath checks if a string looks like a simple field path
func (tp *TemplateParser) isSimpleFieldPath(s string) bool {
	// Must start with .
	if !strings.HasPrefix(s, ".") {
		return false
	}

	// Remove leading dot
	path := s[1:]

	// Empty path (just ".") is simple
	if path == "" {
		return true
	}

	// Check each component of the path
	components := strings.Split(path, ".")
	for _, component := range components {
		if component == "" {
			return false
		}
		// Component should be a valid identifier (letters, numbers, underscore)
		if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(component) {
			return false
		}
	}

	return true
}

// EvaluateFieldPath evaluates a field path against data using reflection
func (tp *TemplateParser) EvaluateFieldPath(fieldPath string, data interface{}) (interface{}, error) {
	if data == nil {
		return nil, fmt.Errorf("data is nil")
	}

	// Handle root reference "."
	if fieldPath == "." || fieldPath == "" {
		return data, nil
	}

	// Remove leading dot if present
	fieldPath = strings.TrimPrefix(fieldPath, ".")

	// Split field path
	fields := strings.Split(fieldPath, ".")
	current := reflect.ValueOf(data)

	for _, field := range fields {
		if field == "" {
			continue
		}

		// Handle interface{} by getting the underlying value
		if current.Kind() == reflect.Interface && !current.IsNil() {
			current = current.Elem()
		}

		switch current.Kind() {
		case reflect.Map:
			// Handle map access
			key := reflect.ValueOf(field)
			mapValue := current.MapIndex(key)
			if !mapValue.IsValid() {
				return nil, fmt.Errorf("field %s not found in map", field)
			}
			current = mapValue

		case reflect.Struct:
			// Handle struct field access
			current = current.FieldByName(field)
			if !current.IsValid() {
				return nil, fmt.Errorf("field %s not found in struct", field)
			}

		case reflect.Ptr:
			// Handle pointer dereferencing
			if current.IsNil() {
				return nil, fmt.Errorf("pointer is nil when accessing field %s", field)
			}
			current = current.Elem()
			// Retry with dereferenced value
			return tp.EvaluateFieldPath(strings.Join(fields[0:], "."), current.Interface())

		default:
			return nil, fmt.Errorf("cannot access field %s on type %v", field, current.Kind())
		}
	}

	// Return the interface value
	if current.IsValid() {
		return current.Interface(), nil
	}

	return nil, fmt.Errorf("invalid field path result")
}
