// Package generators provides random data generation for fuzz testing.
package generators

import (
	"fmt"
	"strings"

	"pgregory.net/rapid"
)

// TemplateConfig controls random template generation.
type TemplateConfig struct {
	MaxDepth        int      // Maximum nesting depth
	MaxFields       int      // Maximum number of fields per level
	MaxRanges       int      // Maximum number of ranges per template
	MaxConditionals int      // Maximum number of conditionals per template
	AllowWith       bool     // Allow {{with}} constructs
	AllowNested     bool     // Allow nested structures (ranges in ranges, etc.)
	KeyAttributes   []string // Key attribute patterns for range items
}

// DefaultTemplateConfig returns a balanced config for general fuzzing.
func DefaultTemplateConfig() TemplateConfig {
	return TemplateConfig{
		MaxDepth:        3,
		MaxFields:       5,
		MaxRanges:       2,
		MaxConditionals: 3,
		AllowWith:       true,
		AllowNested:     true,
		KeyAttributes: []string{
			`id="{{.ID}}"`,
			`data-key="{{.ID}}"`,
			`key="{{.ID}}"`,
			`data-lvt-key="{{.ID}}"`,
		},
	}
}

// SimpleTemplateConfig returns a config with no nesting for basic testing.
func SimpleTemplateConfig() TemplateConfig {
	return TemplateConfig{
		MaxDepth:        1,
		MaxFields:       3,
		MaxRanges:       1,
		MaxConditionals: 1,
		AllowWith:       false,
		AllowNested:     false,
		KeyAttributes: []string{
			`id="{{.ID}}"`,
		},
	}
}

// GeneratedTemplate represents a randomly generated template with its expected shape.
type GeneratedTemplate struct {
	Template string      // The template string
	Shape    *StateShape // Expected state shape for this template
}

// GenTemplate generates a random template with its corresponding state shape.
func GenTemplate(config TemplateConfig) *rapid.Generator[GeneratedTemplate] {
	return rapid.Custom(func(t *rapid.T) GeneratedTemplate {
		gen := &templateGenerator{
			config:     config,
			t:          t,
			fieldCount: 0,
			rangeCount: 0,
			condCount:  0,
		}

		shape := &StateShape{
			Fields: make(map[string]FieldType),
			Slices: make(map[string]SliceShape),
			Nested: make(map[string]*StateShape),
		}

		body := gen.generateBody(shape, 0)
		template := fmt.Sprintf("<div>%s</div>", body)

		return GeneratedTemplate{
			Template: template,
			Shape:    shape,
		}
	})
}

type templateGenerator struct {
	config     TemplateConfig
	t          *rapid.T
	fieldCount int
	rangeCount int
	condCount  int
}

func (g *templateGenerator) generateBody(shape *StateShape, depth int) string {
	if depth >= g.config.MaxDepth {
		return g.generateSimpleField(shape)
	}

	var parts []string

	// Decide what constructs to include
	numFields := rapid.IntRange(1, g.config.MaxFields).Draw(g.t, fmt.Sprintf("numFields_%d", depth))
	for i := 0; i < numFields; i++ {
		construct := g.chooseConstruct(depth)
		switch construct {
		case "field":
			parts = append(parts, g.generateFieldConstruct(shape))
		case "conditional":
			parts = append(parts, g.generateConditionalConstruct(shape, depth))
		case "range":
			parts = append(parts, g.generateRangeConstruct(shape, depth))
		case "with":
			parts = append(parts, g.generateWithConstruct(shape, depth))
		}
	}

	return strings.Join(parts, "\n")
}

func (g *templateGenerator) chooseConstruct(depth int) string {
	// Weight the construct types based on config and current state
	choices := []string{"field", "field", "field"} // Fields are most common

	if g.condCount < g.config.MaxConditionals {
		choices = append(choices, "conditional")
	}

	if g.rangeCount < g.config.MaxRanges && (g.config.AllowNested || depth == 0) {
		choices = append(choices, "range")
	}

	if g.config.AllowWith && depth < g.config.MaxDepth-1 {
		choices = append(choices, "with")
	}

	return rapid.SampledFrom(choices).Draw(g.t, fmt.Sprintf("construct_%d", g.fieldCount))
}

func (g *templateGenerator) generateSimpleField(shape *StateShape) string {
	g.fieldCount++
	name := fmt.Sprintf("Field%d", g.fieldCount)
	shape.Fields[name] = FieldString
	return fmt.Sprintf("<span>{{.%s}}</span>", name)
}

func (g *templateGenerator) generateFieldConstruct(shape *StateShape) string {
	g.fieldCount++
	name := fmt.Sprintf("Field%d", g.fieldCount)

	// Choose field type
	fieldType := rapid.SampledFrom([]FieldType{
		FieldString, FieldString, FieldString, // Weighted toward strings
		FieldInt,
		FieldBool,
	}).Draw(g.t, fmt.Sprintf("fieldType_%s", name))

	shape.Fields[name] = fieldType

	// Generate HTML wrapper
	tag := rapid.SampledFrom([]string{"span", "div", "p", "strong", "em"}).Draw(g.t, fmt.Sprintf("tag_%s", name))

	return fmt.Sprintf("<%s>{{.%s}}</%s>", tag, name, tag)
}

func (g *templateGenerator) generateConditionalConstruct(shape *StateShape, depth int) string {
	g.condCount++
	condName := fmt.Sprintf("Show%d", g.condCount)
	shape.Fields[condName] = FieldBool

	// Generate true branch
	trueBranch := g.generateConditionalBranch(shape, depth, "true")

	// Optionally generate else branch
	hasElse := rapid.Bool().Draw(g.t, fmt.Sprintf("hasElse_%s", condName))
	if hasElse {
		falseBranch := g.generateConditionalBranch(shape, depth, "false")
		return fmt.Sprintf("{{if .%s}}%s{{else}}%s{{end}}", condName, trueBranch, falseBranch)
	}

	return fmt.Sprintf("{{if .%s}}%s{{end}}", condName, trueBranch)
}

func (g *templateGenerator) generateConditionalBranch(shape *StateShape, _ int, branch string) string {
	// Generate simple content for branches to avoid over-complexity
	g.fieldCount++
	name := fmt.Sprintf("Branch%s%d", branch, g.fieldCount)
	shape.Fields[name] = FieldString
	return fmt.Sprintf("<span>{{.%s}}</span>", name)
}

func (g *templateGenerator) generateRangeConstruct(shape *StateShape, _ int) string {
	g.rangeCount++
	rangeName := fmt.Sprintf("Items%d", g.rangeCount)

	// Create item shape
	itemShape := &StateShape{
		Fields: map[string]FieldType{
			"ID": FieldString, // Always need ID for key tracking
		},
		Slices: make(map[string]SliceShape),
		Nested: make(map[string]*StateShape),
	}

	// Add 1-3 additional fields to item
	numItemFields := rapid.IntRange(1, 3).Draw(g.t, fmt.Sprintf("numItemFields_%s", rangeName))
	for i := 0; i < numItemFields; i++ {
		fieldName := rapid.SampledFrom([]string{"Text", "Name", "Label", "Value", "Title"}).Draw(g.t, fmt.Sprintf("itemField_%s_%d", rangeName, i))
		// Avoid duplicates
		if _, exists := itemShape.Fields[fieldName]; !exists {
			itemShape.Fields[fieldName] = FieldString
		}
	}

	// Optionally add a boolean field
	if rapid.Bool().Draw(g.t, fmt.Sprintf("hasItemBool_%s", rangeName)) {
		boolName := rapid.SampledFrom([]string{"Complete", "Active", "Selected", "Visible"}).Draw(g.t, fmt.Sprintf("itemBool_%s", rangeName))
		itemShape.Fields[boolName] = FieldBool
	}

	// Add to shape
	shape.Slices[rangeName] = SliceShape{
		ItemShape: itemShape,
		MinLen:    1, // Keep at least 1 to avoid empty range issues
		MaxLen:    10,
	}

	// Generate item template
	itemTemplate := g.generateRangeItemTemplate(itemShape)

	// Choose key attribute
	keyAttr := rapid.SampledFrom(g.config.KeyAttributes).Draw(g.t, fmt.Sprintf("keyAttr_%s", rangeName))

	return fmt.Sprintf("{{range .%s}}<div %s>%s</div>{{end}}", rangeName, keyAttr, itemTemplate)
}

func (g *templateGenerator) generateRangeItemTemplate(itemShape *StateShape) string {
	var parts []string

	// Generate field references for all fields except ID (which is in the key attribute)
	for name, ftype := range itemShape.Fields {
		if name == "ID" {
			continue // ID is in the key attribute
		}

		tag := "span"
		if ftype == FieldBool {
			// For bools, wrap in conditional
			parts = append(parts, fmt.Sprintf("{{if .%s}}<span class=\"%s\">yes</span>{{end}}", name, strings.ToLower(name)))
		} else {
			parts = append(parts, fmt.Sprintf("<%s>{{.%s}}</%s>", tag, name, tag))
		}
	}

	if len(parts) == 0 {
		return "<span>item</span>"
	}

	return strings.Join(parts, "")
}

func (g *templateGenerator) generateWithConstruct(shape *StateShape, _ int) string {
	g.fieldCount++
	withName := fmt.Sprintf("Obj%d", g.fieldCount)

	// Create nested shape
	nestedShape := &StateShape{
		Fields: make(map[string]FieldType),
		Slices: make(map[string]SliceShape),
		Nested: make(map[string]*StateShape),
	}

	// Add 1-2 fields to nested object
	numFields := rapid.IntRange(1, 2).Draw(g.t, fmt.Sprintf("numWithFields_%s", withName))
	for i := 0; i < numFields; i++ {
		fieldName := rapid.SampledFrom([]string{"Name", "Value", "Label", "Description"}).Draw(g.t, fmt.Sprintf("withField_%s_%d", withName, i))
		if _, exists := nestedShape.Fields[fieldName]; !exists {
			nestedShape.Fields[fieldName] = FieldString
		}
	}

	shape.Nested[withName] = nestedShape

	// Generate body
	var parts []string
	for name := range nestedShape.Fields {
		parts = append(parts, fmt.Sprintf("<span>{{.%s}}</span>", name))
	}
	body := strings.Join(parts, "")

	// Optionally add else
	hasElse := rapid.Bool().Draw(g.t, fmt.Sprintf("withHasElse_%s", withName))
	if hasElse {
		return fmt.Sprintf("{{with .%s}}<div>%s</div>{{else}}<div>no data</div>{{end}}", withName, body)
	}

	return fmt.Sprintf("{{with .%s}}<div>%s</div>{{end}}", withName, body)
}

// InferShapeFromTemplate attempts to infer a StateShape from a template string.
// This is a simplified parser that handles common patterns.
func InferShapeFromTemplate(template string) *StateShape {
	shape := &StateShape{
		Fields: make(map[string]FieldType),
		Slices: make(map[string]SliceShape),
		Nested: make(map[string]*StateShape),
	}

	// Extract field references: {{.FieldName}}
	extractFields(template, shape, "")

	return shape
}

func extractFields(template string, shape *StateShape, _ string) {
	// Simple regex-free extraction for common patterns
	// This handles: {{.Field}}, {{.Nested.Field}}, {{range .Items}}, etc.

	i := 0
	for i < len(template) {
		// Find next {{
		start := strings.Index(template[i:], "{{")
		if start == -1 {
			break
		}
		start += i

		// Find matching }}
		end := strings.Index(template[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		expr := strings.TrimSpace(template[start+2 : end-2])

		// Parse the expression
		if strings.HasPrefix(expr, "range ") {
			// Range construct: {{range .Items}}
			rangeExpr := strings.TrimPrefix(expr, "range ")
			rangeExpr = strings.TrimSpace(rangeExpr)
			if strings.HasPrefix(rangeExpr, ".") {
				name := strings.TrimPrefix(rangeExpr, ".")
				// Find the corresponding {{end}}
				rangeEnd := findMatchingEnd(template[end:], "range")
				if rangeEnd > 0 {
					itemTemplate := template[end : end+rangeEnd]
					itemShape := &StateShape{
						Fields: make(map[string]FieldType),
						Slices: make(map[string]SliceShape),
						Nested: make(map[string]*StateShape),
					}
					extractFields(itemTemplate, itemShape, "")
					shape.Slices[name] = SliceShape{
						ItemShape: itemShape,
						MinLen:    1,
						MaxLen:    10,
					}
				}
			}
		} else if strings.HasPrefix(expr, "if ") {
			// Conditional: {{if .Field}}
			condExpr := strings.TrimPrefix(expr, "if ")
			condExpr = strings.TrimSpace(condExpr)
			if strings.HasPrefix(condExpr, ".") {
				name := strings.TrimPrefix(condExpr, ".")
				shape.Fields[name] = FieldBool
			}
		} else if strings.HasPrefix(expr, "with ") {
			// With construct: {{with .Obj}}
			withExpr := strings.TrimPrefix(expr, "with ")
			withExpr = strings.TrimSpace(withExpr)
			if strings.HasPrefix(withExpr, ".") {
				name := strings.TrimPrefix(withExpr, ".")
				// Find body and extract nested fields
				withEnd := findMatchingEnd(template[end:], "with")
				if withEnd > 0 {
					bodyTemplate := template[end : end+withEnd]
					nestedShape := &StateShape{
						Fields: make(map[string]FieldType),
						Slices: make(map[string]SliceShape),
						Nested: make(map[string]*StateShape),
					}
					extractFields(bodyTemplate, nestedShape, "")
					shape.Nested[name] = nestedShape
				}
			}
		} else if strings.HasPrefix(expr, ".") && !strings.HasPrefix(expr, "..") {
			// Simple field reference: {{.Field}} or {{.Nested.Field}}
			fieldPath := strings.TrimPrefix(expr, ".")
			// Split by . for nested access
			parts := strings.Split(fieldPath, ".")
			if len(parts) == 1 && parts[0] != "" {
				shape.Fields[parts[0]] = FieldString
			}
		}

		i = end
	}
}

func findMatchingEnd(template string, construct string) int {
	depth := 1
	i := 0
	for i < len(template) {
		// Find next {{
		start := strings.Index(template[i:], "{{")
		if start == -1 {
			break
		}
		start += i

		// Find matching }}
		end := strings.Index(template[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		expr := strings.TrimSpace(template[start+2 : end-2])

		if strings.HasPrefix(expr, construct+" ") || expr == construct {
			depth++
		} else if expr == "end" {
			depth--
			if depth == 0 {
				return start
			}
		}

		i = end
	}
	return -1
}

// FixedTemplates returns a set of hand-crafted templates for deterministic testing.
// These cover common patterns and known edge cases.
func FixedTemplates() []GeneratedTemplate {
	return []GeneratedTemplate{
		// 1. Simple field only
		{
			Template: `<div>{{.Title}}</div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{"Title": FieldString},
				Slices: make(map[string]SliceShape),
				Nested: make(map[string]*StateShape),
			},
		},
		// 2. Multiple fields
		{
			Template: `<div><h1>{{.Title}}</h1><p>{{.Description}}</p><span>{{.Count}}</span></div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{
					"Title":       FieldString,
					"Description": FieldString,
					"Count":       FieldInt,
				},
				Slices: make(map[string]SliceShape),
				Nested: make(map[string]*StateShape),
			},
		},
		// 3. Simple conditional
		{
			Template: `<div>{{if .ShowMenu}}<nav>menu</nav>{{end}}</div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{"ShowMenu": FieldBool},
				Slices: make(map[string]SliceShape),
				Nested: make(map[string]*StateShape),
			},
		},
		// 4. Conditional with else
		{
			Template: `<div>{{if .LoggedIn}}<span>Welcome</span>{{else}}<span>Please login</span>{{end}}</div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{"LoggedIn": FieldBool},
				Slices: make(map[string]SliceShape),
				Nested: make(map[string]*StateShape),
			},
		},
		// 5. Simple range
		{
			Template: `<ul>{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}</ul>`,
			Shape: &StateShape{
				Fields: make(map[string]FieldType),
				Slices: map[string]SliceShape{
					"Items": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":   FieldString,
								"Text": FieldString,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 10,
					},
				},
				Nested: make(map[string]*StateShape),
			},
		},
		// 6. Range with multiple item fields
		{
			Template: `<div>{{range .Tasks}}<div data-key="{{.ID}}"><span>{{.Title}}</span>{{if .Complete}}<span>done</span>{{end}}</div>{{end}}</div>`,
			Shape: &StateShape{
				Fields: make(map[string]FieldType),
				Slices: map[string]SliceShape{
					"Tasks": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":       FieldString,
								"Title":    FieldString,
								"Complete": FieldBool,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 10,
					},
				},
				Nested: make(map[string]*StateShape),
			},
		},
		// 7. Fields + Range combination
		{
			Template: `<div><h1>{{.Title}}</h1>{{if .ShowItems}}<ul>{{range .Items}}<li id="{{.ID}}">{{.Name}}</li>{{end}}</ul>{{end}}</div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{
					"Title":     FieldString,
					"ShowItems": FieldBool,
				},
				Slices: map[string]SliceShape{
					"Items": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":   FieldString,
								"Name": FieldString,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 10,
					},
				},
				Nested: make(map[string]*StateShape),
			},
		},
		// 8. With construct
		{
			Template: `<div>{{with .User}}<span>{{.Name}}</span><span>{{.Email}}</span>{{end}}</div>`,
			Shape: &StateShape{
				Fields: make(map[string]FieldType),
				Slices: make(map[string]SliceShape),
				Nested: map[string]*StateShape{
					"User": {
						Fields: map[string]FieldType{
							"Name":  FieldString,
							"Email": FieldString,
						},
						Slices: make(map[string]SliceShape),
						Nested: make(map[string]*StateShape),
					},
				},
			},
		},
		// 9. Multiple ranges
		{
			Template: `<div><ul>{{range .Items}}<li id="{{.ID}}">{{.Text}}</li>{{end}}</ul><ul>{{range .Tags}}<li data-key="{{.ID}}">{{.Label}}</li>{{end}}</ul></div>`,
			Shape: &StateShape{
				Fields: make(map[string]FieldType),
				Slices: map[string]SliceShape{
					"Items": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":   FieldString,
								"Text": FieldString,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 10,
					},
					"Tags": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":    FieldString,
								"Label": FieldString,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 5,
					},
				},
				Nested: make(map[string]*StateShape),
			},
		},
		// 10. Complex: fields + conditional + range
		{
			Template: `<div><h1>{{.Title}}</h1>{{if .ShowMenu}}<nav>{{.MenuText}}</nav>{{end}}<ul>{{range .Items}}<li id="{{.ID}}">{{.Text}}{{if .Important}}<strong>!</strong>{{end}}</li>{{end}}</ul></div>`,
			Shape: &StateShape{
				Fields: map[string]FieldType{
					"Title":    FieldString,
					"ShowMenu": FieldBool,
					"MenuText": FieldString,
				},
				Slices: map[string]SliceShape{
					"Items": {
						ItemShape: &StateShape{
							Fields: map[string]FieldType{
								"ID":        FieldString,
								"Text":      FieldString,
								"Important": FieldBool,
							},
							Slices: make(map[string]SliceShape),
							Nested: make(map[string]*StateShape),
						},
						MinLen: 1,
						MaxLen: 10,
					},
				},
				Nested: make(map[string]*StateShape),
			},
		},
	}
}
