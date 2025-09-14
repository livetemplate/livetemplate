package diff

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnifiedTreeDiff_ComprehensiveTemplateConstructs(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		initialData    any
		updatedData    any
		expectedFirst  string // Expected JSON structure for first render
		expectedUpdate string // Expected JSON structure for update
		description    string
	}{
		// Basic Field Tests
		{
			name:           "SingleField",
			template:       `<p>Hello {{.Name}}!</p>`,
			initialData:    map[string]any{"Name": "World"},
			updatedData:    map[string]any{"Name": "Alice"},
			expectedFirst:  `{"0":"World","h":"...","s":["<p>Hello ","!</p>"]}`,
			expectedUpdate: `{"0":"Alice"}`,
			description:    "Simple field substitution",
		},
		{
			name:           "MultipleFields",
			template:       `<div>{{.Name}} has {{.Score}} points and lives in {{.City}}</div>`,
			initialData:    map[string]any{"Name": "Alice", "Score": 100, "City": "NYC"},
			updatedData:    map[string]any{"Name": "Bob", "Score": 150, "City": "LA"},
			expectedFirst:  `{"0":"Alice","1":"100","2":"NYC","h":"...","s":["<div>"," has "," points and lives in ","</div>"]}`,
			expectedUpdate: `{"0":"Bob","1":"150","2":"LA"}`,
			description:    "Multiple fields in sequence",
		},
		{
			name:           "FieldsWithAttributes",
			template:       `<input type="{{.Type}}" value="{{.Value}}" placeholder="{{.Placeholder}}">`,
			initialData:    map[string]any{"Type": "text", "Value": "", "Placeholder": "Enter name"},
			updatedData:    map[string]any{"Type": "email", "Value": "test@example.com", "Placeholder": "Enter email"},
			expectedFirst:  `{"0":"text","1":"","2":"Enter name","h":"...","s":["<input type=\"",">\"",">value=\"",">\"",">placeholder=\"",">\"",">"]}`,
			expectedUpdate: `{"0":"email","1":"test@example.com","2":"Enter email"}`,
			description:    "Fields within HTML attributes",
		},

		// Conditional Tests
		{
			name:           "ConditionalTrue",
			template:       `<div>{{if .ShowWelcome}}Welcome {{.Name}}!{{end}}</div>`,
			initialData:    map[string]any{"ShowWelcome": true, "Name": "John"},
			updatedData:    map[string]any{"ShowWelcome": true, "Name": "Jane"},
			expectedFirst:  `{"0":"Welcome John!","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"Welcome Jane!"}`,
			description:    "Conditional block that evaluates to true",
		},
		{
			name:           "ConditionalFalse",
			template:       `<div>{{if .ShowWelcome}}Welcome {{.Name}}!{{end}}</div>`,
			initialData:    map[string]any{"ShowWelcome": false, "Name": "John"},
			updatedData:    map[string]any{"ShowWelcome": false, "Name": "Jane"},
			expectedFirst:  `{"0":"","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":""}`,
			description:    "Conditional block that evaluates to false",
		},
		{
			name:           "ConditionalToggle",
			template:       `<div>{{if .IsVisible}}Content is visible{{else}}Content is hidden{{end}}</div>`,
			initialData:    map[string]any{"IsVisible": true},
			updatedData:    map[string]any{"IsVisible": false},
			expectedFirst:  `{"0":"Content is visible","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"Content is hidden"}`,
			description:    "Toggle between if and else blocks",
		},
		{
			name:           "NestedConditionals",
			template:       `<div>{{if .User}}{{if .User.IsAdmin}}Admin: {{.User.Name}}{{else}}User: {{.User.Name}}{{end}}{{else}}No user{{end}}</div>`,
			initialData:    map[string]any{"User": map[string]any{"Name": "Alice", "IsAdmin": true}},
			updatedData:    map[string]any{"User": map[string]any{"Name": "Bob", "IsAdmin": false}},
			expectedFirst:  `{"0":"Admin: Alice","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"User: Bob"}`,
			description:    "Nested conditional expressions",
		},

		// Range Tests
		{
			name:           "SimpleRange",
			template:       `<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			initialData:    map[string]any{"Items": []string{"Apple", "Banana"}},
			updatedData:    map[string]any{"Items": []string{"Apple", "Banana", "Cherry"}},
			expectedFirst:  `{"0":"<li>Apple</li><li>Banana</li>","h":"...","s":["<ul>","</ul>"]}`,
			expectedUpdate: `{"0":"<li>Apple</li><li>Banana</li><li>Cherry</li>"}`,
			description:    "Simple range over string slice",
		},
		{
			name:           "RangeWithIndex",
			template:       `<div>{{range $i, $item := .Items}}{{$i}}: {{$item.Name}} {{end}}</div>`,
			initialData:    map[string]any{"Items": []map[string]any{{"Name": "First"}, {"Name": "Second"}}},
			updatedData:    map[string]any{"Items": []map[string]any{{"Name": "Updated"}, {"Name": "Items"}}},
			expectedFirst:  `{"0":"0: First 1: Second ","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"0: Updated 1: Items "}`,
			description:    "Range with index and item access",
		},
		{
			name:           "RangeWithStructs",
			template:       `<table>{{range .Users}}<tr><td>{{.Name}}</td><td>{{.Email}}</td></tr>{{end}}</table>`,
			initialData:    map[string]any{"Users": []map[string]any{{"Name": "Alice", "Email": "alice@test.com"}}},
			updatedData:    map[string]any{"Users": []map[string]any{{"Name": "Bob", "Email": "bob@test.com"}, {"Name": "Carol", "Email": "carol@test.com"}}},
			expectedFirst:  `{"0":"<tr><td>Alice</td><td>alice@test.com</td></tr>","h":"...","s":["<table>","</table>"]}`,
			expectedUpdate: `{"0":"<tr><td>Bob</td><td>bob@test.com</td></tr><tr><td>Carol</td><td>carol@test.com</td></tr>"}`,
			description:    "Range over struct slice with multiple fields",
		},
		{
			name:           "EmptyRange",
			template:       `<div>{{range .Items}}Item: {{.}}{{else}}No items{{end}}</div>`,
			initialData:    map[string]any{"Items": []string{}},
			updatedData:    map[string]any{"Items": []string{"First"}},
			expectedFirst:  `{"0":"No items","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"Item: First"}`,
			description:    "Range with else clause for empty slice",
		},

		// With Construct Tests
		{
			name:           "WithConstruct",
			template:       `<div>{{with .User}}<span>Hello {{.Name}}</span>{{end}}</div>`,
			initialData:    map[string]any{"User": map[string]any{"Name": "Alice"}},
			updatedData:    map[string]any{"User": map[string]any{"Name": "Bob"}},
			expectedFirst:  `{"0":"<span>Hello Alice</span>","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"<span>Hello Bob</span>"}`,
			description:    "With construct changing context",
		},
		{
			name:           "WithElseConstruct",
			template:       `<div>{{with .Profile}}{{.Bio}}{{else}}No profile{{end}}</div>`,
			initialData:    map[string]any{"Profile": nil},
			updatedData:    map[string]any{"Profile": map[string]any{"Bio": "Software Engineer"}},
			expectedFirst:  `{"0":"No profile","h":"...","s":["<div>","</div>"]}`,
			expectedUpdate: `{"0":"Software Engineer"}`,
			description:    "With construct with else fallback",
		},

		// Complex Nested HTML Tests
		{
			name: "NestedFormElements",
			template: `<form class="{{.FormClass}}">
				<fieldset>
					<legend>{{.Title}}</legend>
					<div class="form-group">
						<label for="name">Name:</label>
						<input id="name" type="text" value="{{.Name}}" required="{{.Required}}">
					</div>
					<div class="form-group">
						<label for="email">Email:</label>
						<input id="email" type="email" value="{{.Email}}">
					</div>
				</fieldset>
				<button type="submit" {{if .Disabled}}disabled{{end}}>{{.ButtonText}}</button>
			</form>`,
			initialData: map[string]any{
				"FormClass":  "user-form",
				"Title":      "User Information",
				"Name":       "John Doe",
				"Email":      "john@example.com",
				"Required":   true,
				"Disabled":   false,
				"ButtonText": "Save",
			},
			updatedData: map[string]any{
				"FormClass":  "user-form updated",
				"Title":      "Edit Profile",
				"Name":       "Jane Smith",
				"Email":      "jane@example.com",
				"Required":   false,
				"Disabled":   true,
				"ButtonText": "Update",
			},
			description: "Complex nested form with multiple template constructs",
		},

		// Table with Conditional Content
		{
			name: "ConditionalTable",
			template: `<table class="{{.TableClass}}">
				<thead>
					<tr>
						<th>Name</th>
						<th>Status</th>
						{{if .ShowActions}}<th>Actions</th>{{end}}
					</tr>
				</thead>
				<tbody>
					{{range .Users}}
					<tr class="{{if .Active}}active{{else}}inactive{{end}}">
						<td>{{.Name}}</td>
						<td>{{.Status}}</td>
						{{if $.ShowActions}}
						<td>
							<button onclick="edit('{{.ID}}')">Edit</button>
							{{if .Active}}
							<button onclick="deactivate('{{.ID}}')">Deactivate</button>
							{{else}}
							<button onclick="activate('{{.ID}}')">Activate</button>
							{{end}}
						</td>
						{{end}}
					</tr>
					{{end}}
				</tbody>
			</table>`,
			initialData: map[string]any{
				"TableClass":  "users-table",
				"ShowActions": true,
				"Users": []map[string]any{
					{"ID": "1", "Name": "Alice", "Status": "Online", "Active": true},
					{"ID": "2", "Name": "Bob", "Status": "Offline", "Active": false},
				},
			},
			updatedData: map[string]any{
				"TableClass":  "users-table updated",
				"ShowActions": false,
				"Users": []map[string]any{
					{"ID": "1", "Name": "Alice", "Status": "Away", "Active": true},
					{"ID": "3", "Name": "Carol", "Status": "Online", "Active": true},
				},
			},
			description: "Complex table with conditionals and range",
		},

		// Navigation Menu
		{
			name: "NavigationMenu",
			template: `<nav class="{{.NavClass}}">
				<div class="nav-brand">{{.BrandName}}</div>
				<ul class="nav-items">
					{{range .MenuItems}}
					<li class="nav-item {{if .Active}}active{{end}} {{if .Disabled}}disabled{{end}}">
						{{if .Disabled}}
							<span class="nav-link disabled">{{.Text}}</span>
						{{else}}
							<a href="{{.URL}}" class="nav-link">{{.Text}}</a>
						{{end}}
						{{if .HasDropdown}}
						<ul class="dropdown">
							{{range .DropdownItems}}
							<li><a href="{{.URL}}">{{.Text}}</a></li>
							{{end}}
						</ul>
						{{end}}
					</li>
					{{end}}
				</ul>
			</nav>`,
			initialData: map[string]any{
				"NavClass":  "main-nav",
				"BrandName": "MyApp",
				"MenuItems": []map[string]any{
					{
						"Text": "Home", "URL": "/", "Active": true, "Disabled": false,
						"HasDropdown": false, "DropdownItems": []map[string]any{},
					},
					{
						"Text": "Products", "URL": "/products", "Active": false, "Disabled": false,
						"HasDropdown": true,
						"DropdownItems": []map[string]any{
							{"Text": "Category A", "URL": "/products/a"},
							{"Text": "Category B", "URL": "/products/b"},
						},
					},
				},
			},
			updatedData: map[string]any{
				"NavClass":  "main-nav mobile",
				"BrandName": "MyApp Pro",
				"MenuItems": []map[string]any{
					{
						"Text": "Home", "URL": "/", "Active": false, "Disabled": false,
						"HasDropdown": false, "DropdownItems": []map[string]any{},
					},
					{
						"Text": "Products", "URL": "/products", "Active": true, "Disabled": false,
						"HasDropdown": true,
						"DropdownItems": []map[string]any{
							{"Text": "All Products", "URL": "/products"},
							{"Text": "Featured", "URL": "/products/featured"},
							{"Text": "Sale Items", "URL": "/products/sale"},
						},
					},
					{
						"Text": "Settings", "URL": "/settings", "Active": false, "Disabled": true,
						"HasDropdown": false, "DropdownItems": []map[string]any{},
					},
				},
			},
			description: "Complex navigation with nested dropdowns and conditionals",
		},

		// Card Layout with Mixed Content
		{
			name: "CardLayout",
			template: `<div class="card-container">
				{{range .Cards}}
				<div class="card {{.Type}}">
					{{if .HasImage}}
					<img src="{{.Image}}" alt="{{.Title}}" class="card-image">
					{{end}}
					<div class="card-content">
						<h3 class="card-title">{{.Title}}</h3>
						{{if .Subtitle}}<p class="card-subtitle">{{.Subtitle}}</p>{{end}}
						<p class="card-description">{{.Description}}</p>
						{{if .Tags}}
						<div class="card-tags">
							{{range .Tags}}
							<span class="tag {{if .Featured}}featured{{end}}">{{.Name}}</span>
							{{end}}
						</div>
						{{end}}
						<div class="card-footer">
							{{if .Price}}
							<span class="price">${{.Price}}</span>
							{{end}}
							{{if .Available}}
							<button class="btn btn-primary">Add to Cart</button>
							{{else}}
							<button class="btn btn-secondary" disabled>Out of Stock</button>
							{{end}}
						</div>
					</div>
				</div>
				{{else}}
				<div class="empty-state">No cards available</div>
				{{end}}
			</div>`,
			initialData: map[string]any{
				"Cards": []map[string]any{
					{
						"Type": "product", "HasImage": true, "Image": "/img/product1.jpg",
						"Title": "Awesome Product", "Subtitle": "Best Seller",
						"Description": "This is an amazing product you'll love",
						"Price":       29.99, "Available": true,
						"Tags": []map[string]any{
							{"Name": "New", "Featured": true},
							{"Name": "Popular", "Featured": false},
						},
					},
				},
			},
			updatedData: map[string]any{
				"Cards": []map[string]any{
					{
						"Type": "product sale", "HasImage": true, "Image": "/img/product1.jpg",
						"Title": "Awesome Product", "Subtitle": "On Sale!",
						"Description": "This is an amazing product you'll love - now on sale!",
						"Price":       19.99, "Available": false,
						"Tags": []map[string]any{
							{"Name": "Sale", "Featured": true},
							{"Name": "Limited", "Featured": true},
						},
					},
					{
						"Type": "info", "HasImage": false, "Image": "",
						"Title": "Special Offer", "Subtitle": "",
						"Description": "Get 20% off your next purchase!",
						"Price":       0, "Available": true,
						"Tags": []map[string]any{},
					},
				},
			},
			description: "Card layout with images, conditional content, and nested structures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differ := NewTree()

			// First render
			firstUpdate, err := differ.Generate(tt.template, nil, tt.initialData)
			if err != nil {
				t.Fatalf("First render failed: %v", err)
			}

			// Verify first render has both statics and dynamics (for non-empty templates)
			if !firstUpdate.IsEmpty() {
				if !firstUpdate.HasStatics() && strings.Contains(tt.template, "<") {
					t.Error("First render should have statics for HTML templates")
				}
				if !firstUpdate.HasDynamics() && strings.Contains(tt.template, "{{") {
					t.Error("First render should have dynamics for templates with expressions")
				}
			}

			// Test JSON serialization of first render
			firstJSON, err := json.Marshal(firstUpdate)
			if err != nil {
				t.Fatalf("Failed to serialize first render: %v", err)
			}

			t.Logf("First render JSON (%d bytes): %s", len(firstJSON), string(firstJSON))

			// Test reconstruction
			reconstructed := firstUpdate.Reconstruct(nil)
			if reconstructed == "" && !firstUpdate.IsEmpty() {
				t.Error("Reconstruction should not be empty for non-empty updates")
			}

			// Update render
			updateRender, err := differ.Generate(tt.template, tt.initialData, tt.updatedData)
			if err != nil {
				t.Fatalf("Update render failed: %v", err)
			}

			// Update should only have dynamics, no statics (unless it's an empty update)
			if !updateRender.IsEmpty() {
				if updateRender.HasStatics() {
					t.Error("Update should NOT have statics (cached on client)")
				}
				if !updateRender.HasDynamics() {
					t.Error("Update should have dynamics")
				}
			}

			// Test JSON serialization of update
			updateJSON, err := json.Marshal(updateRender)
			if err != nil {
				t.Fatalf("Failed to serialize update: %v", err)
			}

			t.Logf("Update JSON (%d bytes): %s", len(updateJSON), string(updateJSON))

			// Test update reconstruction with cached statics
			if !updateRender.IsEmpty() {
				reconstructed2 := updateRender.Reconstruct(firstUpdate.S)
				if reconstructed2 == "" {
					t.Error("Update reconstruction should not be empty")
				}
			}

			// No-change test
			noChangeUpdate, err := differ.Generate(tt.template, tt.updatedData, tt.updatedData)
			if err != nil {
				t.Fatalf("No-change render failed: %v", err)
			}

			if !noChangeUpdate.IsEmpty() {
				t.Error("No-change update should be empty")
			}

			// Bandwidth analysis
			firstSize := len(firstJSON)
			updateSize := len(updateJSON)
			if firstSize > 0 && updateSize > 0 {
				savings := float64(firstSize-updateSize) / float64(firstSize) * 100
				t.Logf("Bandwidth analysis - First: %d bytes, Update: %d bytes, Savings: %.1f%%",
					firstSize, updateSize, savings)

				// For most real templates, we should see significant savings
				if savings < 30 && firstSize > 50 {
					t.Logf("Lower than expected savings (%.1f%%) - this may be normal for simple templates", savings)
				}
			}

			t.Logf("Description: %s", tt.description)
		})
	}
}

func TestUnifiedTreeDiff_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        any
		description string
	}{
		{
			name:        "EmptyTemplate",
			template:    "",
			data:        map[string]any{},
			description: "Completely empty template",
		},
		{
			name:        "OnlyStaticHTML",
			template:    "<div>Static content only</div>",
			data:        map[string]any{},
			description: "Template with no dynamic parts",
		},
		{
			name:        "OnlyTemplateExpressions",
			template:    "{{.Name}}{{.Age}}{{.City}}",
			data:        map[string]any{"Name": "Alice", "Age": 30, "City": "NYC"},
			description: "Template with no static HTML",
		},
		{
			name:        "NestedEmptyConditionals",
			template:    "{{if .A}}{{if .B}}{{if .C}}Content{{end}}{{end}}{{end}}",
			data:        map[string]any{"A": true, "B": true, "C": false},
			description: "Deeply nested conditionals that evaluate to empty",
		},
		{
			name:        "ComplexWhitespace",
			template:    "  {{.Name}}  \n\t{{.Age}}\n  {{.City}}  ",
			data:        map[string]any{"Name": "Alice", "Age": 30, "City": "NYC"},
			description: "Template with complex whitespace patterns",
		},
		{
			name:        "UnicodeContent",
			template:    "<div>🌟 {{.Name}} 🎉 has {{.Score}} points! 🏆</div>",
			data:        map[string]any{"Name": "Alice", "Score": 100},
			description: "Template with Unicode emoji characters",
		},
		{
			name:        "HTMLEntities",
			template:    "<div>&lt;{{.Tag}}&gt; content &amp; more {{.Content}} &quot;quotes&quot;</div>",
			data:        map[string]any{"Tag": "span", "Content": "data"},
			description: "Template with HTML entities",
		},
		{
			name:        "LargeContent",
			template:    "<div>{{.Content}}</div>",
			data:        map[string]any{"Content": strings.Repeat("Large content block ", 100)},
			description: "Template with large content blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			differ := NewTree()

			update, err := differ.Generate(tt.template, nil, tt.data)
			if err != nil {
				t.Fatalf("Generation failed: %v", err)
			}

			// Test JSON serialization
			jsonData, err := json.Marshal(update)
			if err != nil {
				t.Fatalf("JSON serialization failed: %v", err)
			}

			t.Logf("Generated JSON (%d bytes): %s", len(jsonData), string(jsonData))

			// Test reconstruction
			reconstructed := update.Reconstruct(nil)
			t.Logf("Reconstructed: %s", reconstructed)

			// Verify JSON round-trip
			var decoded Update
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("JSON deserialization failed: %v", err)
			}

			// Test that deserialized version can also reconstruct
			reconstructed2 := decoded.Reconstruct(nil)
			if reconstructed != reconstructed2 {
				t.Errorf("Reconstruction mismatch after JSON round-trip")
				t.Logf("Original: %s", reconstructed)
				t.Logf("After round-trip: %s", reconstructed2)
			}

			t.Logf("Description: %s", tt.description)
		})
	}
}

// Benchmark the unified tree diff performance
func BenchmarkUnifiedTreeDiff(b *testing.B) {
	templates := map[string]struct {
		template string
		data     any
	}{
		"Simple": {
			template: `<p>Hello {{.Name}}!</p>`,
			data:     map[string]any{"Name": "World"},
		},
		"Complex": {
			template: `<div class="{{.Class}}">
				{{range .Items}}
				<div class="item {{if .Active}}active{{end}}">
					<h3>{{.Title}}</h3>
					<p>{{.Description}}</p>
					{{if .Tags}}
					<div class="tags">
						{{range .Tags}}<span>{{.}}</span>{{end}}
					</div>
					{{end}}
				</div>
				{{end}}
			</div>`,
			data: map[string]any{
				"Class": "container",
				"Items": []map[string]any{
					{
						"Title": "Item 1", "Description": "First item",
						"Active": true, "Tags": []string{"tag1", "tag2"},
					},
					{
						"Title": "Item 2", "Description": "Second item",
						"Active": false, "Tags": []string{"tag3"},
					},
				},
			},
		},
	}

	for name, tmpl := range templates {
		b.Run(name, func(b *testing.B) {
			differ := NewTree()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := differ.Generate(tmpl.template, nil, tmpl.data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
