package strategy

import (
	"testing"
)

// TestExtendedTemplateConstructs tests additional Go template constructs
// not covered in the main test suite
func TestExtendedTemplateConstructs(t *testing.T) {
	testCases := []struct {
		name         string
		template     string
		data         interface{}
		expectedHTML string
		description  string
	}{
		{
			name:     "Variables",
			template: `{{$name := .User}}Hello {{$name}}!`,
			data:     map[string]interface{}{"User": "Alice"},
			expectedHTML: "Hello Alice!",
			description: "Variable declaration and usage",
		},
		{
			name:     "VariableWithLen",
			template: `{{$count := len .Items}}Total: {{$count}} items`,
			data:     map[string]interface{}{"Items": []string{"a", "b", "c"}},
			expectedHTML: "Total: 3 items",
			description: "Variable with len function",
		},
		{
			name:     "ComparisonFunctions",
			template: `{{if eq .Status "active"}}Active{{else}}Inactive{{end}}`,
			data:     map[string]interface{}{"Status": "active"},
			expectedHTML: "Active",
			description: "eq comparison function",
		},
		{
			name:     "GreaterThanComparison", 
			template: `{{if gt .Score 80}}High{{else}}Low{{end}}`,
			data:     map[string]interface{}{"Score": 85},
			expectedHTML: "High",
			description: "gt comparison function",
		},
		{
			name:     "LogicalAnd",
			template: `{{if and .IsActive .HasPermission}}Allowed{{else}}Denied{{end}}`,
			data:     map[string]interface{}{"IsActive": true, "HasPermission": true},
			expectedHTML: "Allowed",
			description: "and logical function",
		},
		{
			name:     "LogicalOr",
			template: `{{if or .IsAdmin .IsOwner}}Can Edit{{else}}Read Only{{end}}`,
			data:     map[string]interface{}{"IsAdmin": false, "IsOwner": true},
			expectedHTML: "Can Edit",
			description: "or logical function",
		},
		{
			name:     "PrintfFunction",
			template: `{{printf "Score: %.1f" .Score}}`,
			data:     map[string]interface{}{"Score": 85.67},
			expectedHTML: "Score: 85.7",
			description: "printf formatting function",
		},
		{
			name:     "NestedConditionals",
			template: `<div>{{if .User}}{{if .User.IsAdmin}}Admin{{else}}User{{end}}{{else}}Guest{{end}}</div>`,
			data:     map[string]interface{}{
				"User": map[string]interface{}{
					"IsAdmin": true,
				},
			},
			expectedHTML: "<div>Admin</div>",
			description: "Nested if statements",
		},
		{
			name:     "RangeWithIndex",
			template: `{{range $i, $item := .Items}}{{$i}}:{{$item}} {{end}}`,
			data:     map[string]interface{}{"Items": []string{"Apple", "Banana"}},
			expectedHTML: "0:Apple 1:Banana ",
			description: "Range with index and value variables",
		},
		{
			name:     "RangeWithValue",
			template: `{{range $item := .Items}}<span>{{$item}}</span>{{end}}`,
			data:     map[string]interface{}{"Items": []string{"X", "Y"}},
			expectedHTML: "<span>X</span><span>Y</span>",
			description: "Range with value variable only",
		},
		{
			name:     "RangeWithElse",
			template: `<ul>{{range .Items}}<li>{{.}}</li>{{else}}<li>Empty</li>{{end}}</ul>`,
			data:     map[string]interface{}{"Items": []string{}},
			expectedHTML: "<ul><li>Empty</li></ul>",
			description: "Range with else clause for empty slice",
		},
		{
			name:     "NestedFieldAccess",
			template: `{{.User.Profile.Name}}`,
			data:     map[string]interface{}{
				"User": map[string]interface{}{
					"Profile": map[string]interface{}{
						"Name": "John Doe",
					},
				},
			},
			expectedHTML: "John Doe",
			description: "Deep nested field access",
		},
		{
			name:     "Comments",
			template: `<div>{{/* This is a comment */}}{{.Content}}</div>`,
			data:     map[string]interface{}{"Content": "Hello"},
			expectedHTML: "<div>Hello</div>",
			description: "Template comments are ignored",
		},
		{
			name:     "IndexFunction",
			template: `{{index .Items 1}}`,
			data:     map[string]interface{}{"Items": []string{"First", "Second", "Third"}},
			expectedHTML: "Second",
			description: "index function to access slice element",
		},
		{
			name:     "IndexMap",
			template: `{{index .Map "key2"}}`,
			data:     map[string]interface{}{
				"Map": map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedHTML: "value2",
			description: "index function to access map value",
		},
		{
			name:     "RootVariable",
			template: `{{range .Items}}{{$.Title}}: {{.}} {{end}}`,
			data:     map[string]interface{}{
				"Title": "Item",
				"Items": []string{"A", "B"},
			},
			expectedHTML: "Item: A Item: B ",
			description: "Root variable access within range",
		},
		{
			name:     "RangeOverMap",
			template: `{{range $key, $value := .Settings}}{{$key}}={{$value}} {{end}}`,
			data:     map[string]interface{}{
				"Settings": map[string]string{
					"color": "blue",
					"size":  "large",
				},
			},
			expectedHTML: "color=blue size=large ",
			description: "Range over map with key-value pairs",
		},
		{
			name:     "NestedRanges",
			template: `{{range .Groups}}[{{range .Items}}{{.}}{{end}}]{{end}}`,
			data:     map[string]interface{}{
				"Groups": []map[string]interface{}{
					{"Items": []string{"1", "2"}},
					{"Items": []string{"A", "B"}},
				},
			},
			expectedHTML: "[12][AB]",
			description: "Nested range loops",
		},
		{
			name:     "ComplexNesting",
			template: `{{range .Users}}{{if .Active}}{{.Name}}{{if eq .Role "admin"}}*{{end}} {{end}}{{end}}`,
			data:     map[string]interface{}{
				"Users": []map[string]interface{}{
					{"Name": "Alice", "Active": true, "Role": "admin"},
					{"Name": "Bob", "Active": false, "Role": "user"},
					{"Name": "Carol", "Active": true, "Role": "user"},
				},
			},
			expectedHTML: "Alice* Carol ",
			description: "Complex nesting of range, if, and comparisons",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create differ
			diff, err := NewDiffer(tc.template)
			if err != nil {
				t.Fatalf("NewDiffer failed: %v", err)
			}

			// Generate tree
			tree, err := generateTreeForTest(diff, tc.data)
			if err != nil {
				t.Fatalf("GenerateTree failed: %v", err)
			}

			// Verify HTML reconstruction
			html := reconstructHTML(tree)
			if html != tc.expectedHTML {
				t.Errorf("HTML mismatch:\nExpected: %s\nGot: %s", tc.expectedHTML, html)
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}
}

// TestErrorHandlingExtended tests error conditions with various template constructs
func TestErrorHandlingExtended(t *testing.T) {
	testCases := []struct {
		name        string
		template    string
		data        interface{}
		shouldError bool
		description string
	}{
		{
			name:        "InvalidSyntax",
			template:    `{{.Name`,
			data:        map[string]interface{}{"Name": "Alice"},
			shouldError: true,
			description: "Unclosed template action",
		},
		{
			name:        "InvalidFunction",
			template:    `{{unknown .Name}}`,
			data:        map[string]interface{}{"Name": "Alice"},
			shouldError: true,
			description: "Unknown function call",
		},
		{
			name:        "NilPointer",
			template:    `{{.User.Name}}`,
			data:        map[string]interface{}{"User": nil},
			shouldError: true,
			description: "Nil pointer access",
		},
		{
			name:        "MissingField",
			template:    `{{.NonExistent}}`,
			data:        map[string]interface{}{"Name": "Alice"},
			shouldError: false, // Go templates are lenient with missing fields
			description: "Non-existent field access (produces empty string)",
		},
		{
			name:        "IndexOutOfBounds",
			template:    `{{index .Items 5}}`,
			data:        map[string]interface{}{"Items": []string{"A", "B"}},
			shouldError: true,
			description: "Index out of bounds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := NewDiffer(tc.template)
			
			if tc.shouldError {
				// Should fail at template parse or execution
				if err == nil {
					// Template parsed successfully, try execution
					_, execErr := generateTreeForTest(diff, tc.data)
					if execErr == nil {
						t.Errorf("Expected error for %s, but got none", tc.description)
					} else {
						t.Logf("✅ %s: Correctly failed with error: %v", tc.name, execErr)
					}
				} else {
					t.Logf("✅ %s: Correctly failed at parse time: %v", tc.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tc.description, err)
				} else {
					t.Logf("✅ %s: Correctly succeeded", tc.name)
				}
			}
		})
	}
}

// TestPerformanceEdgeCases tests performance with complex template scenarios  
func TestPerformanceEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		template    string
		dataGen     func() interface{}
		description string
	}{
		{
			name:     "LargeRange",
			template: `{{range .Items}}{{.}}{{end}}`,
			dataGen: func() interface{} {
				items := make([]string, 1000)
				for i := 0; i < 1000; i++ {
					items[i] = "item"
				}
				return map[string]interface{}{"Items": items}
			},
			description: "Large range with 1000 items",
		},
		{
			name:     "DeepNesting",
			template: `{{.L1.L2.L3.L4.L5.Value}}`,
			dataGen: func() interface{} {
				return map[string]interface{}{
					"L1": map[string]interface{}{
						"L2": map[string]interface{}{
							"L3": map[string]interface{}{
								"L4": map[string]interface{}{
									"L5": map[string]interface{}{
										"Value": "deep",
									},
								},
							},
						},
					},
				}
			},
			description: "Deep nested field access (5 levels)",
		},
		{
			name:     "ComplexConditionals",
			template: `{{if .A}}{{if .B}}{{if .C}}{{if .D}}{{if .E}}deep{{end}}{{end}}{{end}}{{end}}{{end}}`,
			dataGen: func() interface{} {
				return map[string]interface{}{
					"A": true, "B": true, "C": true, "D": true, "E": true,
				}
			},
			description: "Deeply nested conditionals",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := NewDiffer(tc.template)
			if err != nil {
				t.Fatalf("NewDiffer failed: %v", err)
			}

			data := tc.dataGen()
			
			// Measure performance
			tree, err := generateTreeForTest(diff, data)
			if err != nil {
				t.Fatalf("GenerateTree failed: %v", err)
			}

			// Verify reconstruction works
			html := reconstructHTML(tree)
			if html == "" {
				t.Errorf("Empty HTML result for %s", tc.description)
			}

			t.Logf("✅ %s: %s (HTML length: %d)", tc.name, tc.description, len(html))
		})
	}
}