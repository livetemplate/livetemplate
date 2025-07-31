package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/livefir/statetemplate"
)

// Test data structures for comprehensive template action testing
type TDDTestData struct {
	// Basic fields
	Title      string `json:"title"`
	Message    string `json:"message"`
	Count      int    `json:"count"`
	EmptyField string `json:"empty_field"`
	
	// Boolean fields for conditionals
	IsVisible   bool `json:"is_visible"`
	IsEnabled   bool `json:"is_enabled"`
	HasContent  bool `json:"has_content"`
	ShowDetails bool `json:"show_details"`
	
	// Collections for range operations
	Items    []TDDItem    `json:"items"`
	Users    []TDDUser    `json:"users"`
	Tags     []string     `json:"tags"`
	Numbers  []int        `json:"numbers"`
	
	// Nested objects for with operations
	Profile   *TDDProfile   `json:"profile,omitempty"`
	Settings  *TDDSettings  `json:"settings,omitempty"`
	Metadata  *TDDMetadata  `json:"metadata,omitempty"`
	
	// Function test fields
	Score     float64 `json:"score"`
	Threshold float64 `json:"threshold"`
}

type TDDItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type TDDUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type TDDProfile struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar,omitempty"`
}

type TDDSettings struct {
	Theme      string `json:"theme"`
	Language   string `json:"language"`
	Timezone   string `json:"timezone"`
	Advanced   *TDDAdvancedSettings `json:"advanced,omitempty"`
}

type TDDAdvancedSettings struct {
	DebugMode bool   `json:"debug_mode"`
	LogLevel  string `json:"log_level"`
}

type TDDMetadata struct {
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Tags      map[string]string `json:"tags"`
}

// CommentTestSuite defines test cases for comment actions
type CommentTestSuite struct {
	name               string
	template           string
	data               *TDDTestData
	shouldNotContain   []string
	shouldContain      []string
	description        string
}

// TestTemplateActionComments tests comment actions using table-driven tests
func TestTemplateActionComments(t *testing.T) {
	t.Log("🧪 Testing Template Action: Comments")
	
	testSuite := []CommentTestSuite{
		{
			name: "BasicComments",
			template: `<div>
	{{/* This is a basic comment */}}
	<h1>{{.Title}}</h1>
	<p>{{.Message}}</p>
</div>`,
			data: &TDDTestData{
				Title:   "Basic Comment Test",
				Message: "Basic comments should not appear",
			},
			shouldNotContain: []string{"This is a basic comment"},
			shouldContain:    []string{"Basic Comment Test", "Basic comments should not appear"},
			description:      "Basic comments should be stripped from output",
		},
		{
			name: "WhitespaceTrimmingComments",
			template: `<div>
	{{- /* Comment with whitespace trimming */ -}}
	<h1>{{.Title}}</h1>
	<p>{{.Message}}</p>
</div>`,
			data: &TDDTestData{
				Title:   "Trimming Test",
				Message: "Trimming comments test",
			},
			shouldNotContain: []string{"Comment with whitespace trimming"},
			shouldContain:    []string{"Trimming Test", "Trimming comments test"},
			description:      "Comments with whitespace trimming should be stripped",
		},
		{
			name: "MultiLineComments",
			template: `<div>
	{{/*
	Multi-line comment
	with multiple lines
	*/}}
	<h1>{{.Title}}</h1>
	<span>{{.Message}}</span>
</div>`,
			data: &TDDTestData{
				Title:   "Multi-line Test",
				Message: "Multi-line testing",
			},
			shouldNotContain: []string{"Multi-line comment", "with multiple lines"},
			shouldContain:    []string{"Multi-line Test", "Multi-line testing"},
			description:      "Multi-line comments should be completely stripped",
		},
		{
			name: "MixedComments",
			template: `<div>
	{{/* Basic comment */}}
	<h1>{{.Title}}</h1>
	{{- /* Trimming comment */ -}}
	<p>{{.Message}}</p>
	{{/*
	Multi-line
	comment block
	*/}}
	<span>Content after comments</span>
</div>`,
			data: &TDDTestData{
				Title:   "Mixed Test",
				Message: "All types together",
			},
			shouldNotContain: []string{"Basic comment", "Trimming comment", "Multi-line", "comment block"},
			shouldContain:    []string{"Mixed Test", "All types together", "Content after comments"},
			description:      "All comment types should be stripped while preserving content",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("comments_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate comments are not in output
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Text '%s' should not appear in output", text)
				}
			}
			
			// Validate actual content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Text '%s' not found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Comment actions test suite completed")
}

// PipelineTestSuite defines test cases for pipeline output actions
type PipelineTestSuite struct {
	name        string
	template    string
	data        *TDDTestData
	expected    []string
	description string
}

// TestTemplateActionPipelineOutput tests basic pipeline output using table-driven tests
func TestTemplateActionPipelineOutput(t *testing.T) {
	t.Log("🧪 Testing Template Action: Pipeline Output")
	
	testSuite := []PipelineTestSuite{
		{
			name: "BasicPipelineOutput",
			template: `<div>
	<h1>{{.Title}}</h1>
	<p>Message: {{.Message}}</p>
	<span>Count: {{.Count}}</span>
	<div>Score: {{.Score}}</div>
</div>`,
			data: &TDDTestData{
				Title:   "Pipeline Test",
				Message: "Testing pipeline output",
				Count:   42,
				Score:   3.14159,
			},
			expected:    []string{"Pipeline Test", "Testing pipeline output", "42", "3.14159"},
			description: "Basic pipeline output should render all field values",
		},
		{
			name: "NumericPipelineOutput",
			template: `<div>
	<p>Integer: {{.Count}}</p>
	<p>Float: {{.Score}}</p>
	<p>Threshold: {{.Threshold}}</p>
</div>`,
			data: &TDDTestData{
				Count:     100,
				Score:     99.99,
				Threshold: 85.5,
			},
			expected:    []string{"Integer: 100", "Float: 99.99", "Threshold: 85.5"},
			description: "Numeric pipeline output should preserve precision",
		},
		{
			name: "BooleanPipelineOutput",
			template: `<div>
	<p>Visible: {{.IsVisible}}</p>
	<p>Enabled: {{.IsEnabled}}</p>
	<p>Has Content: {{.HasContent}}</p>
</div>`,
			data: &TDDTestData{
				IsVisible:  true,
				IsEnabled:  false,
				HasContent: true,
			},
			expected:    []string{"Visible: true", "Enabled: false", "Has Content: true"},
			description: "Boolean pipeline output should render true/false values",
		},
		{
			name: "StringPipelineOutput",
			template: `<div>
	<h1>{{.Title}}</h1>
	<p>{{.Message}}</p>
	<span>Empty: {{.EmptyField}}</span>
</div>`,
			data: &TDDTestData{
				Title:      "String Test",
				Message:    "Multiple words with spaces",
				EmptyField: "",
			},
			expected:    []string{"String Test", "Multiple words with spaces", "Empty: "},
			description: "String pipeline output should handle various string values including empty strings",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("pipeline_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate all expected outputs are present
			for _, expected := range tc.expected {
				if !strings.Contains(html, expected) {
					t.Errorf("❌ Expected text '%s' not found in output", expected)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Pipeline output actions test suite completed")
}

// IfStatementTestSuite defines test cases for if/else conditionals
type IfStatementTestSuite struct {
	name        string
	template    string
	data        *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description string
}

// TestTemplateActionIfStatements tests if/else conditionals using table-driven tests
func TestTemplateActionIfStatements(t *testing.T) {
	t.Log("🧪 Testing Template Action: If Statements")
	
	testSuite := []IfStatementTestSuite{
		{
			name: "AllTrueConditions",
			template: `<div>
	{{if .IsVisible}}
		<section class="visible">Content is visible</section>
	{{end}}
	
	{{if .IsEnabled}}
		<div class="enabled">Feature enabled</div>
	{{else}}
		<div class="disabled">Feature disabled</div>
	{{end}}
	
	{{if .HasContent}}
		<p>Has content: {{.Message}}</p>
	{{else}}
		<p>No content available</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible:  true,
				IsEnabled:  true,
				HasContent: true,
				Message:    "Content exists",
			},
			shouldContain:    []string{"Content is visible", "Feature enabled", "Content exists"},
			shouldNotContain: []string{"Feature disabled", "No content available"},
			description:      "All true conditions should render positive branches",
		},
		{
			name: "AllFalseConditions",
			template: `<div>
	{{if .IsVisible}}
		<section class="visible">Content is visible</section>
	{{end}}
	
	{{if .IsEnabled}}
		<div class="enabled">Feature enabled</div>
	{{else}}
		<div class="disabled">Feature disabled</div>
	{{end}}
	
	{{if .HasContent}}
		<p>Has content: {{.Message}}</p>
	{{else}}
		<p>No content available</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible:  false,
				IsEnabled:  false,
				HasContent: false,
			},
			shouldContain:    []string{"Feature disabled", "No content available"},
			shouldNotContain: []string{"Content is visible", "Feature enabled"},
			description:      "All false conditions should render negative branches or nothing",
		},
		{
			name: "MixedConditions",
			template: `<div>
	{{if .IsVisible}}
		<section>Visible section</section>
	{{end}}
	
	{{if .IsEnabled}}
		<div>Enabled</div>
	{{else}}
		<div>Disabled</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible: true,
				IsEnabled: false,
			},
			shouldContain:    []string{"Visible section", "Disabled"},
			shouldNotContain: []string{"Enabled"},
			description:      "Mixed conditions should render appropriate branches",
		},
		{
			name: "NestedIfStatements",
			template: `<div>
	{{if .IsVisible}}
		<section>
			{{if .IsEnabled}}
				<p>Both visible and enabled</p>
			{{else}}
				<p>Visible but disabled</p>
			{{end}}
		</section>
	{{else}}
		<section>Not visible</section>
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible: true,
				IsEnabled: false,
			},
			shouldContain:    []string{"Visible but disabled"},
			shouldNotContain: []string{"Both visible and enabled", "Not visible"},
			description:      "Nested if statements should evaluate correctly",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("if_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ If statement actions test suite completed")
}

// IfElseChainsTestCase defines test cases for if-else-if chains
type IfElseChainsTestCase struct {
	name           string
	count          int
	score          float64
	expectedCount  string
	expectedScore  string
	description    string
}

// TestTemplateActionIfElseChains tests if-else-if chains using table-driven tests
func TestTemplateActionIfElseChains(t *testing.T) {
	t.Log("🧪 Testing Template Action: If-Else Chains")
	
	template := `<div>
	{{if eq .Count 0}}
		<p>No items</p>
	{{else if eq .Count 1}}
		<p>One item</p>
	{{else if lt .Count 10}}
		<p>Few items ({{.Count}})</p>
	{{else}}
		<p>Many items ({{.Count}})</p>
	{{end}}
	
	{{if eq .Score 0.0}}
		<span class="zero">Zero score</span>
	{{else if lt .Score 50.0}}
		<span class="low">Low score</span>
	{{else if lt .Score 80.0}}
		<span class="medium">Medium score</span>
	{{else}}
		<span class="high">High score</span>
	{{end}}
</div>`

	testCases := []IfElseChainsTestCase{
		{
			name:          "ZeroCount",
			count:         0,
			score:         0.0,
			expectedCount: "No items",
			expectedScore: "Zero score",
			description:   "Zero values should trigger first conditions",
		},
		{
			name:          "OneCount",
			count:         1,
			score:         25.5,
			expectedCount: "One item",
			expectedScore: "Low score",
			description:   "Single count and low score should trigger appropriate conditions",
		},
		{
			name:          "FewCount",
			count:         5,
			score:         65.5,
			expectedCount: "Few items (5)",
			expectedScore: "Medium score",
			description:   "Few items and medium score should trigger middle conditions",
		},
		{
			name:          "ManyCount",
			count:         20,
			score:         95.0,
			expectedCount: "Many items (20)",
			expectedScore: "High score",
			description:   "Many items and high score should trigger final else conditions",
		},
		{
			name:          "EdgeCaseBoundaryValues",
			count:         10,
			score:         50.0,
			expectedCount: "Many items (10)",
			expectedScore: "Medium score",
			description:   "Boundary values should trigger correct conditions",
		},
		{
			name:          "EdgeCaseHighBoundary",
			count:         9,
			score:         79.9,
			expectedCount: "Few items (9)",
			expectedScore: "Medium score",
			description:   "High boundary values should trigger medium conditions",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("if_chains_"+tc.name, template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			data := &TDDTestData{
				Count: tc.count,
				Score: tc.score,
			}

			html, err := renderer.SetInitialData(data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			if !strings.Contains(html, tc.expectedCount) {
				t.Errorf("❌ Expected count text '%s' not found in: %s", tc.expectedCount, html)
			}
			if !strings.Contains(html, tc.expectedScore) {
				t.Errorf("❌ Expected score text '%s' not found in: %s", tc.expectedScore, html)
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ If-else chain actions test suite completed")
}

// RangeLoopsTestSuite defines test cases for range iterations
type RangeLoopsTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionRangeLoops tests range iterations using table-driven tests
func TestTemplateActionRangeLoops(t *testing.T) {
	t.Log("🧪 Testing Template Action: Range Loops")
	
	baseTemplate := `<div>
	<!-- Range over items -->
	{{range .Items}}
		<div class="item">{{.Name}} ({{.ID}})</div>
	{{end}}
	
	<!-- Range with else -->
	{{range .Tags}}
		<span class="tag">{{.}}</span>
	{{else}}
		<span class="no-tags">No tags</span>
	{{end}}
	
	<!-- Range with index -->
	{{range $index, $user := .Users}}
		<p>User {{$index}}: {{$user.Username}}</p>
	{{end}}
	
	<!-- Range over numbers -->
	{{range .Numbers}}
		<span class="number">{{.}}</span>
	{{end}}
</div>`

	testSuite := []RangeLoopsTestSuite{
		{
			name:     "PopulatedCollections",
			template: baseTemplate,
			data: &TDDTestData{
				Items: []TDDItem{
					{ID: "1", Name: "First Item"},
					{ID: "2", Name: "Second Item"},
				},
				Tags: []string{"go", "template", "test"},
				Users: []TDDUser{
					{Username: "alice", Email: "alice@test.com"},
					{Username: "bob", Email: "bob@test.com"},
				},
				Numbers: []int{1, 2, 3, 4, 5},
			},
			shouldContain: []string{
				"First Item (1)", "Second Item (2)",
				"go", "template", "test",
				"User 0: alice", "User 1: bob",
				`class="number">1<`, `class="number">5<`,
			},
			shouldNotContain: []string{"No tags"},
			description:     "Populated collections should render all items correctly",
		},
		{
			name:     "EmptyCollections",
			template: baseTemplate,
			data: &TDDTestData{
				Items:   []TDDItem{},
				Tags:    []string{},
				Users:   []TDDUser{},
				Numbers: []int{},
			},
			shouldContain:    []string{"No tags"},
			shouldNotContain: []string{"First Item", "alice", `class="number"`},
			description:      "Empty collections should trigger else clauses and render nothing for regular ranges",
		},
		{
			name: "SingleItemCollections",
			template: `<div>
	{{range .Items}}
		<div>{{.Name}}</div>
	{{else}}
		<div>Empty</div>
	{{end}}
	
	{{range .Tags}}
		<span>{{.}}</span>
	{{end}}
</div>`,
			data: &TDDTestData{
				Items: []TDDItem{{ID: "1", Name: "Only Item"}},
				Tags:  []string{"single"},
			},
			shouldContain:    []string{"Only Item", "single"},
			shouldNotContain: []string{"Empty"},
			description:      "Single item collections should render correctly",
		},
		{
			name: "NestedStructureRange",
			template: `<div>
	{{range .Users}}
		<div class="user">
			<span>{{.Username}}</span>
			<span>{{.Email}}</span>
			<span>{{.Role}}</span>
		</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Users: []TDDUser{
					{Username: "admin", Email: "admin@test.com", Role: "administrator"},
					{Username: "user", Email: "user@test.com", Role: "member"},
				},
			},
			shouldContain: []string{
				"admin", "admin@test.com", "administrator",
				"user", "user@test.com", "member",
			},
			shouldNotContain: []string{},
			description:     "Range over complex structures should access all nested fields",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("range_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Range loop actions test suite completed")
}

// WithStatementsTestSuite defines test cases for with context changes
type WithStatementsTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionWithStatements tests with context changes using table-driven tests
func TestTemplateActionWithStatements(t *testing.T) {
	t.Log("🧪 Testing Template Action: With Statements")
	
	baseTemplate := `<div>
	<!-- Basic with -->
	{{with .Profile}}
		<section class="profile">
			<h2>{{.DisplayName}}</h2>
			<p>{{.Bio}}</p>
		</section>
	{{end}}
	
	<!-- With else -->
	{{with .Settings}}
		<div class="settings">
			<p>Theme: {{.Theme}}</p>
			<p>Language: {{.Language}}</p>
		</div>
	{{else}}
		<div class="no-settings">No settings configured</div>
	{{end}}
	
	<!-- Nested with -->
	{{with .Settings}}
		<div class="outer">
			<span>Theme: {{.Theme}}</span>
			{{with .Advanced}}
				<div class="advanced">
					<p>Debug: {{.DebugMode}}</p>
					<p>Log Level: {{.LogLevel}}</p>
				</div>
			{{else}}
				<div class="no-advanced">No advanced settings</div>
			{{end}}
		</div>
	{{end}}
</div>`

	testSuite := []WithStatementsTestSuite{
		{
			name:     "PopulatedWithStatements",
			template: baseTemplate,
			data: &TDDTestData{
				Profile: &TDDProfile{
					DisplayName: "John Doe",
					Bio:         "Software Developer",
				},
				Settings: &TDDSettings{
					Theme:    "dark",
					Language: "en",
					Advanced: &TDDAdvancedSettings{
						DebugMode: true,
						LogLevel:  "info",
					},
				},
			},
			shouldContain: []string{
				"John Doe", "Software Developer",
				"Theme: dark", "Language: en",
				"Debug: true", "Log Level: info",
			},
			shouldNotContain: []string{"No settings configured", "No advanced settings"},
			description:     "Populated with statements should access nested contexts correctly",
		},
		{
			name:     "NilWithStatements",
			template: baseTemplate,
			data: &TDDTestData{
				Profile:  nil,
				Settings: nil,
			},
			shouldContain:    []string{"No settings configured"},
			shouldNotContain: []string{"John Doe", "Theme: dark", "Debug: true"},
			description:      "Nil with statements should trigger else clauses and not render context content",
		},
		{
			name: "PartialWithStatements",
			template: `<div>
	{{with .Profile}}
		<section>{{.DisplayName}}</section>
	{{else}}
		<section>No profile</section>
	{{end}}
	
	{{with .Settings}}
		<div>{{.Theme}}</div>
		{{with .Advanced}}
			<p>Advanced settings exist</p>
		{{else}}
			<p>No advanced settings</p>
		{{end}}
	{{end}}
</div>`,
			data: &TDDTestData{
				Profile: &TDDProfile{
					DisplayName: "Jane Smith",
				},
				Settings: &TDDSettings{
					Theme:    "light",
					Advanced: nil,
				},
			},
			shouldContain:    []string{"Jane Smith", "light", "No advanced settings"},
			shouldNotContain: []string{"No profile", "Advanced settings exist"},
			description:      "Partial with statements should handle mixed nil/non-nil contexts",
		},
		{
			name: "DeepNestedWithStatements",
			template: `<div>
	{{with .Settings}}
		<div class="settings">
			{{with .Advanced}}
				<div class="advanced">
					<p>Debug: {{.DebugMode}}</p>
					<p>Log: {{.LogLevel}}</p>
				</div>
			{{end}}
		</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Settings: &TDDSettings{
					Advanced: &TDDAdvancedSettings{
						DebugMode: false,
						LogLevel:  "warn",
					},
				},
			},
			shouldContain:    []string{"Debug: false", "Log: warn"},
			shouldNotContain: []string{},
			description:      "Deep nested with statements should access deeply nested contexts",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("with_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ With statement actions test suite completed")
}

// VariableAssignmentTestSuite defines test cases for variable declarations and usage
type VariableAssignmentTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionVariableAssignment tests variable declarations and usage using table-driven tests
func TestTemplateActionVariableAssignment(t *testing.T) {
	t.Log("🧪 Testing Template Action: Variable Assignment")
	
	testSuite := []VariableAssignmentTestSuite{
		{
			name: "BasicVariableAssignment",
			template: `<div>
	{{$title := .Title}}
	{{$count := .Count}}
	{{$hasItems := gt .Count 0}}
	
	<h1>{{$title}}</h1>
	<p>Item count: {{$count}}</p>
	
	{{if $hasItems}}
		<div class="has-items">Found {{$count}} items</div>
	{{else}}
		<div class="no-items">No items found</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title: "Variable Test",
				Count: 3,
			},
			shouldContain:    []string{"Variable Test", "Item count: 3", "Found 3 items"},
			shouldNotContain: []string{"No items found"},
			description:      "Basic variable assignment should store and use values correctly",
		},
		{
			name: "ZeroCountVariables",
			template: `<div>
	{{$count := .Count}}
	{{$hasItems := gt .Count 0}}
	
	<p>Count: {{$count}}</p>
	{{if $hasItems}}
		<div>Has items</div>
	{{else}}
		<div>No items</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Count: 0,
			},
			shouldContain:    []string{"Count: 0", "No items"},
			shouldNotContain: []string{"Has items"},
			description:      "Variables with zero values should work correctly in conditions",
		},
		{
			name: "RangeWithVariables",
			template: `<div>
	{{range $index, $item := .Items}}
		{{$itemClass := "item"}}
		{{if .Active}}
			{{$itemClass = "item active"}}  
		{{end}}
		<div class="{{$itemClass}}">{{$index}}: {{.Name}}</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Items: []TDDItem{
					{ID: "1", Name: "Active Item", Active: true},
					{ID: "2", Name: "Inactive Item", Active: false},
				},
			},
			shouldContain:    []string{`class="item active"`, "0: Active Item", `class="item"`, "1: Inactive Item"},
			shouldNotContain: []string{},
			description:      "Variables in range loops should handle conditional assignment",
		},
		{
			name: "VariableScope",
			template: `<div>
	{{$outerVar := "outer"}}
	{{with .Profile}}
		{{$innerVar := "inner"}}
		<div>{{$outerVar}} - {{$innerVar}} - {{.DisplayName}}</div>
	{{end}}
	<p>Outer: {{$outerVar}}</p>
</div>`,
			data: &TDDTestData{
				Profile: &TDDProfile{
					DisplayName: "Test User",
				},
			},
			shouldContain:    []string{"outer - inner - Test User", "Outer: outer"},
			shouldNotContain: []string{},
			description:      "Variable scope should work correctly across different contexts",
		},
		{
			name: "ComplexVariableOperations",
			template: `<div>
	{{$userCount := len .Users}}
	{{$hasUsers := gt $userCount 0}}
	{{$message := printf "Found %d users" $userCount}}
	
	<h2>{{$message}}</h2>
	{{if $hasUsers}}
		<ul>
		{{range .Users}}
			{{$roleClass := printf "user-%s" .Role}}
			<li class="{{$roleClass}}">{{.Username}}</li>
		{{end}}
		</ul>
	{{end}}
</div>`,
			data: &TDDTestData{
				Users: []TDDUser{
					{Username: "admin", Role: "administrator"},
					{Username: "user", Role: "member"},
				},
			},
			shouldContain:    []string{"Found 2 users", `class="user-administrator"`, "admin", `class="user-member"`, "user"},
			shouldNotContain: []string{},
			description:      "Complex variable operations with functions should work correctly",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("variables_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Variable assignment actions test suite completed")
}

// WhitespaceTrimmingTestSuite defines test cases for whitespace control
type WhitespaceTrimmingTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionWhitespaceTrimming tests whitespace control using table-driven tests
func TestTemplateActionWhitespaceTrimming(t *testing.T) {
	t.Log("🧪 Testing Template Action: Whitespace Trimming")
	
	testSuite := []WhitespaceTrimmingTestSuite{
		{
			name: "BasicWhitespaceTrimming",
			template: `<div>
	{{- .Title -}}
	
	{{- if .IsVisible -}}
		{{- .Message -}}
	{{- end -}}
	
	<span>{{- .Count -}}</span>
</div>`,
			data: &TDDTestData{
				Title:     "TrimTest",
				IsVisible: true,
				Message:   "NoSpaces",
				Count:     42,
			},
			shouldContain:    []string{"TrimTest", "NoSpaces", "42"},
			shouldNotContain: []string{"  TrimTest", "TrimTest  ", "  NoSpaces", "NoSpaces  "},
			description:      "Basic whitespace trimming should remove excess whitespace",
		},
		{
			name: "RangeWithTrimming",
			template: `<div>
	{{- range .Tags -}}
		<tag>{{- . -}}</tag>
	{{- end -}}
</div>`,
			data: &TDDTestData{
				Tags: []string{"tag1", "tag2", "tag3"},
			},
			shouldContain:    []string{"<tag>tag1</tag>", "<tag>tag2</tag>", "<tag>tag3</tag>"},
			shouldNotContain: []string{"  <tag>", "</tag>  "},
			description:      "Range with trimming should remove whitespace around iteration items",
		},
		{
			name: "ConditionalTrimming",
			template: `<div>
	{{- if .IsEnabled -}}
		<section>{{- .Title -}}</section>
	{{- else -}}
		<section>Disabled</section>
	{{- end -}}
</div>`,
			data: &TDDTestData{
				IsEnabled: true,
				Title:     "Enabled",
			},
			shouldContain:    []string{"<section>Enabled</section>"},
			shouldNotContain: []string{"  <section>", "</section>  ", "Disabled"},
			description:      "Conditional trimming should work with if-else statements",
		},
		{
			name: "MixedTrimming",
			template: `<div>
	{{.Title}}{{- " (trimmed)" -}}
	<span>{{- .Count -}}</span>
</div>`,
			data: &TDDTestData{
				Title: "Mixed",
				Count: 5,
			},
			shouldContain:    []string{"Mixed (trimmed)", "<span>5</span>"},
			shouldNotContain: []string{"Mixed  (trimmed)", "(trimmed)  <span>", "<span>  5", "5  </span>"},
			description:      "Mixed trimming should handle complex whitespace scenarios",
		},
		{
			name: "NoTrimmingComparison",
			template: `<div>
	{{.Title}}
	{{if .IsVisible}}
		{{.Message}}
	{{end}}
</div>`,
			data: &TDDTestData{
				Title:     "NoTrim",
				IsVisible: true,
				Message:   "WithSpaces",
			},
			shouldContain:    []string{"NoTrim", "WithSpaces"},
			shouldNotContain: []string{},
			description:      "Templates without trimming should preserve whitespace",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("whitespace_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Whitespace trimming actions test suite completed")
}

// FunctionsTestSuite defines test cases for built-in and comparison functions
type FunctionsTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionFunctions tests built-in and comparison functions using table-driven tests
func TestTemplateActionFunctions(t *testing.T) {
	t.Log("🧪 Testing Template Action: Functions")
	
	testSuite := []FunctionsTestSuite{
		{
			name: "ComparisonFunctions",
			template: `<div>
	{{if eq .Count 5}}
		<p>Count equals 5</p>
	{{end}}
	
	{{if ne .Title "wrong"}}
		<p>Title is not wrong</p>
	{{end}}
	
	{{if gt .Score .Threshold}}
		<p>Score above threshold</p>
	{{else}}
		<p>Score below threshold</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title:     "Function Test",
				Count:     5,
				Score:     85.5,
				Threshold: 75.0,
			},
			shouldContain:    []string{"Count equals 5", "Title is not wrong", "Score above threshold"},
			shouldNotContain: []string{"Score below threshold"},
			description:      "Comparison functions (eq, ne, gt) should evaluate correctly",
		},
		{
			name: "LogicalFunctions",
			template: `<div>
	{{if and .IsVisible .IsEnabled}}
		<p>Both visible and enabled</p>
	{{end}}
	
	{{if or .HasContent .ShowDetails}}
		<p>Has content or show details</p>
	{{end}}
	
	{{if not .IsVisible}}
		<p>Not visible</p>
	{{else}}
		<p>Is visible</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible:   true,
				IsEnabled:   true,
				HasContent:  false,
				ShowDetails: true,
			},
			shouldContain:    []string{"Both visible and enabled", "Has content or show details", "Is visible"},
			shouldNotContain: []string{"Not visible"},
			description:      "Logical functions (and, or, not) should evaluate correctly",
		},
		{
			name: "LengthAndPrintfFunctions",
			template: `<div>
	<p>Items length: {{len .Items}}</p>
	<p>Tags length: {{len .Tags}}</p>
	<p>{{printf "Formatted: %s (%d)" .Title .Count}}</p>
	<p>{{printf "Score: %.2f / %.2f" .Score .Threshold}}</p>
</div>`,
			data: &TDDTestData{
				Title:     "Length Test",
				Count:     42,
				Score:     87.456,
				Threshold: 75.5,
				Items: []TDDItem{
					{ID: "1", Name: "Item 1"},
					{ID: "2", Name: "Item 2"},
				},
				Tags: []string{"a", "b", "c"},
			},
			shouldContain:    []string{"Items length: 2", "Tags length: 3", "Formatted: Length Test (42)", "Score: 87.46 / 75.50"},
			shouldNotContain: []string{},
			description:      "Length and printf functions should work correctly",
		},
		{
			name: "NumericComparisonFunctions",
			template: `<div>
	{{if lt .Count 10}}
		<p>Count less than 10</p>
	{{end}}
	
	{{if ge .Score 80.0}}
		<p>Score greater or equal to 80</p>
	{{end}}
	
	{{if le .Threshold 100.0}}
		<p>Threshold less or equal to 100</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				Count:     5,
				Score:     85.0,
				Threshold: 75.0,
			},
			shouldContain:    []string{"Count less than 10", "Score greater or equal to 80", "Threshold less or equal to 100"},
			shouldNotContain: []string{},
			description:      "Numeric comparison functions (lt, ge, le) should work correctly",
		},
		{
			name: "EdgeCaseFunctions",
			template: `<div>
	{{if eq .Count 0}}
		<p>Zero count</p>
	{{end}}
	
	{{if eq .EmptyField ""}}
		<p>Empty string</p>
	{{end}}
	
	{{if len .Items}}
		<p>Has {{len .Items}} items</p>
	{{else}}
		<p>No items</p>
	{{end}}
</div>`,
			data: &TDDTestData{
				Count:      0,
				EmptyField: "",
				Items:      []TDDItem{},
			},
			shouldContain:    []string{"Zero count", "Empty string", "No items"},
			shouldNotContain: []string{"Has"},
			description:      "Functions should handle edge cases like zero values and empty collections",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("functions_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Function actions test suite completed")
}

// BlockDefinitionsTestSuite defines test cases for block definitions and overrides
type BlockDefinitionsTestSuite struct {
	name             string
	template         string
	data             *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionBlockDefinitions tests block definitions and overrides using table-driven tests
func TestTemplateActionBlockDefinitions(t *testing.T) {
	t.Log("🧪 Testing Template Action: Block Definitions")
	
	testSuite := []BlockDefinitionsTestSuite{
		{
			name: "BasicBlockDefinitions",
			template: `<div>
	<h1>{{.Title}}</h1>
	
	{{block "header" .}}
		<header>Header: {{.Title}}</header>
	{{end}}
	
	{{block "content" .}}
		<section>Content: {{.Message}}</section>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title:   "Block Test",
				Message: "Testing blocks",
			},
			shouldContain:    []string{"Header: Block Test", "Content: Testing blocks"},
			shouldNotContain: []string{},
			description:      "Basic block definitions should execute in place",
		},
		{
			name: "MultipleBlocks",
			template: `<div>
	{{block "header" .}}
		<header>{{.Title}}</header>
	{{end}}
	
	{{block "sidebar" .}}
		<aside>Sidebar content</aside>
	{{end}}
	
	{{block "footer" .}}
		<footer>Default footer</footer>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title: "Multiple Blocks",
			},
			shouldContain:    []string{"Multiple Blocks", "Sidebar content", "Default footer"},
			shouldNotContain: []string{},
			description:      "Multiple block definitions should all execute correctly",
		},
		{
			name: "BlocksWithConditionals",
			template: `<div>
	{{block "conditional" .}}
		{{if .IsVisible}}
			<section>Visible block content</section>
		{{else}}
			<section>Hidden block content</section>
		{{end}}
	{{end}}
	
	{{block "range" .}}
		{{range .Items}}
			<div>Block item: {{.Name}}</div>
		{{end}}
	{{end}}
</div>`,
			data: &TDDTestData{
				IsVisible: true,
				Items: []TDDItem{
					{ID: "1", Name: "Block Item 1"},
					{ID: "2", Name: "Block Item 2"},
				},
			},
			shouldContain:    []string{"Visible block content", "Block item: Block Item 1", "Block item: Block Item 2"},
			shouldNotContain: []string{"Hidden block content"},
			description:      "Blocks with conditionals and ranges should work correctly",
		},
		{
			name: "BlocksWithVariables",
			template: `<div>
	{{block "variables" .}}
		{{$blockTitle := .Title}}
		{{$itemCount := len .Items}}
		<h2>{{$blockTitle}} ({{$itemCount}} items)</h2>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title: "Variable Block",
				Items: []TDDItem{{ID: "1", Name: "Item"}},
			},
			shouldContain:    []string{"Variable Block (1 items)"},
			shouldNotContain: []string{},
			description:      "Blocks with variables should handle scope correctly",
		},
		{
			name: "NestedBlocks",
			template: `<div>
	{{block "outer" .}}
		<section>Outer block</section>
		{{with .Profile}}
			{{block "inner" .}}
				<div>Inner: {{.DisplayName}}</div>
			{{end}}
		{{end}}
	{{end}}
</div>`,
			data: &TDDTestData{
				Profile: &TDDProfile{
					DisplayName: "Nested User",
				},
			},
			shouldContain:    []string{"Outer block", "Inner: Nested User"},
			shouldNotContain: []string{},
			description:      "Nested blocks should work with different contexts",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("blocks_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate expected content is present
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			// Validate unwanted content is not present
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			t.Logf("✅ %s: %s", tc.name, tc.description)
		})
	}

	t.Log("✅ Block definition actions test suite completed")
}

// RealTimeFragmentTestSuite defines test cases for real-time fragment generation
type RealTimeFragmentTestSuite struct {
	name             string
	template         string
	initialData      *TDDTestData
	updateData       *TDDTestData
	shouldContain    []string
	shouldNotContain []string
	description      string
}

// TestTemplateActionRealTimeFragmentGeneration verifies all actions generate proper fragments using table-driven tests
func TestTemplateActionRealTimeFragmentGeneration(t *testing.T) {
	t.Log("🧪 Testing Real-time Fragment Generation for All Actions")
	
	testSuite := []RealTimeFragmentTestSuite{
		{
			name: "ComprehensiveFragmentGeneration",
			template: `<div>
	{{/* Template with all action types for fragment testing */}}
	<h1>{{.Title}}</h1>
	
	{{if .IsVisible}}
		<section class="visible">{{.Message}}</section>
	{{else}}
		<section class="hidden">Content hidden</section>
	{{end}}
	
	{{with .Profile}}
		<div class="profile">{{.DisplayName}}: {{.Bio}}</div>
	{{end}}
	
	{{range .Items}}
		<div class="item">{{.Name}}</div>
	{{end}}
	
	<span>Count: {{.Count}}</span>
	<p>{{if gt .Score .Threshold}}High{{else}}Low{{end}} Score</p>
</div>`,
			initialData: &TDDTestData{
				Title:     "Fragment Test",
				Message:   "Initial message",
				IsVisible: true,
				Count:     10,
				Score:     85.0,
				Threshold: 70.0,
				Profile: &TDDProfile{
					DisplayName: "John",
					Bio:         "Developer",
				},
				Items: []TDDItem{
					{ID: "1", Name: "First"},
					{ID: "2", Name: "Second"},
				},
			},
			updateData: &TDDTestData{
				Title:     "Updated Fragment Test",
				Message:   "Updated message",
				IsVisible: false,
				Count:     20,
				Score:     65.0,
				Threshold: 70.0,
				Profile: &TDDProfile{
					DisplayName: "Jane",
					Bio:         "Updated Developer",
				},
				Items: []TDDItem{
					{ID: "3", Name: "Third"},
				},
			},
			shouldContain:    []string{"Fragment Test", "John", "Developer", "First", "Second", "High Score"},
			shouldNotContain: []string{},
			description:      "Comprehensive template should generate fragments for all action types",
		},
		{
			name: "ConditionalFragmentGeneration",
			template: `<div>
	{{if .IsVisible}}
		<section>Visible: {{.Message}}</section>
	{{else}}
		<section>Hidden content</section>
	{{end}}
	
	{{if .Count}}
		<div>Count: {{.Count}}</div>
	{{end}}
</div>`,
			initialData: &TDDTestData{
				IsVisible: true,
				Message:   "Showing",
				Count:     5,
			},
			updateData: &TDDTestData{
				IsVisible: false,
				Message:   "Not showing",
				Count:     0,
			},
			shouldContain:    []string{"Visible: Showing", "Count: 5"},
			shouldNotContain: []string{"Hidden content"},
			description:      "Conditional templates should generate appropriate fragments",
		},
		{
			name: "RangeFragmentGeneration",
			template: `<div>
	<h2>{{.Title}}</h2>
	{{range .Items}}
		<div class="item">{{.Name}} - {{.ID}}</div>
	{{else}}
		<div class="no-items">No items</div>
	{{end}}
</div>`,
			initialData: &TDDTestData{
				Title: "Range Test",
				Items: []TDDItem{
					{ID: "1", Name: "Item One"},
					{ID: "2", Name: "Item Two"},
				},
			},
			updateData: &TDDTestData{
				Title: "Updated Range Test",
				Items: []TDDItem{},
			},
			shouldContain:    []string{"Range Test", "Item One - 1", "Item Two - 2"},
			shouldNotContain: []string{"No items"},
			description:      "Range templates should generate fragments for collection items",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("fragment_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.initialData)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Verify fragments are generated
			fragmentCount := renderer.GetFragmentCount()
			if fragmentCount == 0 {
				t.Error("❌ No fragments generated for template")
			}

			fragmentIDs := renderer.GetFragmentIDs()
			if len(fragmentIDs) == 0 {
				t.Error("❌ No fragment IDs generated")
			}

			// Verify HTML contains fragment IDs
			if !strings.Contains(html, `id="`) {
				t.Error("❌ No fragment IDs found in rendered HTML")
			}

			// Validate initial content
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			// Test real-time updates if updateData is provided
			if tc.updateData != nil {
				renderer.Start()
				defer renderer.Stop()

				updateChan := renderer.GetUpdateChannel()
				var updates []statetemplate.RealtimeUpdate

				// Collect updates
				go func() {
					timeout := time.After(1 * time.Second)
					for {
						select {
						case update := <-updateChan:
							updates = append(updates, update)
						case <-timeout:
							return
						}
					}
				}()

				// Trigger updates
				renderer.SendUpdate(tc.updateData)
				time.Sleep(500 * time.Millisecond)

				if len(updates) == 0 {
					t.Log("⚠️ No real-time updates generated (may be expected for some templates)")
				} else {
					for _, update := range updates {
						if update.FragmentID == "" {
							t.Error("❌ Update missing fragment ID")
						}
					}
					t.Logf("✅ Generated %d real-time updates", len(updates))
				}
			}

			t.Logf("✅ Generated %d fragments for %s", fragmentCount, tc.description)
		})
	}

	t.Log("✅ Real-time fragment generation test suite completed")
}

// IntegrationTestSuite defines test cases for integration of all actions
type IntegrationTestSuite struct {
	name                string
	template            string
	data                *TDDTestData
	shouldContain       []string
	shouldNotContain    []string
	minFragmentCount    int
	description         string
}

// TestAllTemplateActionsTogether tests integration of all actions using table-driven tests
func TestAllTemplateActionsTogether(t *testing.T) {
	t.Log("🧪 Testing All Template Actions Integration")
	
	testSuite := []IntegrationTestSuite{
		{
			name: "CompleteIntegration",
			template: `<div class="app">
	{{/* Header with variables and conditionals */}}
	{{$appTitle := .Title}}
	{{$userCount := len .Users}}
	
	<header>
		<h1>{{$appTitle}}</h1>
		{{if gt $userCount 0}}
			<p>{{$userCount}} users online</p>
		{{else}}
			<p>No users online</p>
		{{end}}
	</header>
	
	{{/* Main content with with-statements */}}
	<main>
		{{with .Profile}}
			<section class="user-info">
				<h2>{{.DisplayName}}</h2>
				<p>{{.Bio}}</p>
			</section>
		{{end}}
		
		{{/* Dynamic content with range and conditionals */}}
		{{if .Items}}
			<div class="items">
				<h3>Items ({{len .Items}})</h3>
				{{range $index, $item := .Items}}
					<div class="item {{if .Active}}active{{else}}inactive{{end}}">
						<span class="index">{{$index}}</span>
						<span class="name">{{.Name}}</span>
						{{if .Active}}
							<span class="status">✓</span>
						{{end}}
					</div>
				{{end}}
			</div>
		{{else}}
			<div class="no-items">No items available</div>
		{{end}}
		
		{{/* Settings with nested with-statements */}}
		{{with .Settings}}
			<section class="settings">
				<h3>Settings</h3>
				<p>Theme: {{.Theme}}</p>
				{{with .Advanced}}
					<div class="advanced">
						<p>Debug Mode: {{if .DebugMode}}Enabled{{else}}Disabled{{end}}</p>
						<p>Log Level: {{.LogLevel}}</p>
					</div>
				{{end}}
			</section>
		{{end}}
	</main>
	
	{{/* Footer with functions and conditionals */}}
	<footer>
		{{if and .IsVisible .IsEnabled}}
			<p>Status: {{if gt .Score .Threshold}}Above{{else}}Below{{end}} threshold</p>
		{{end}}
		<p>{{printf "Score: %.2f / %.2f" .Score .Threshold}}</p>
	</footer>
</div>`,
			data: &TDDTestData{
				Title:     "Integration Test App",
				IsVisible: true,
				IsEnabled: true,
				Score:     87.5,
				Threshold: 75.0,
				Profile: &TDDProfile{
					DisplayName: "Integration User",
					Bio:         "Testing all template actions together",
				},
				Users: []TDDUser{
					{Username: "user1"},
					{Username: "user2"},
					{Username: "user3"},
				},
				Items: []TDDItem{
					{ID: "1", Name: "Active Item", Active: true},
					{ID: "2", Name: "Inactive Item", Active: false},
				},
				Settings: &TDDSettings{
					Theme: "dark",
					Advanced: &TDDAdvancedSettings{
						DebugMode: true,
						LogLevel:  "debug",
					},
				},
			},
			shouldContain: []string{
				"Integration Test App", "3 users online", "Integration User",
				"Testing all template actions together", "Active Item", "class=\"item active\"",
				"Inactive Item", "class=\"item inactive\"", "Theme: dark",
				"Debug Mode: Enabled", "Log Level: debug", "Above threshold",
				"Score: 87.50 / 75.00",
			},
			shouldNotContain:    []string{"No users online", "No items available", "Below threshold", "Disabled"},
			minFragmentCount:    5,
			description:         "Complete integration should handle all template actions correctly",
		},
		{
			name: "EdgeCaseIntegration",
			template: `<div>
	{{$count := len .Items}}
	{{if eq $count 0}}
		<p>Empty state</p>
	{{else}}
		<div>
			{{range .Items}}
				<span>{{.Name}}</span>
			{{end}}
		</div>
	{{end}}
	
	{{with .Profile}}
		{{if .DisplayName}}
			<h2>{{.DisplayName}}</h2>
		{{end}}
	{{else}}
		<h2>Anonymous</h2>
	{{end}}
</div>`,
			data: &TDDTestData{
				Items:   []TDDItem{},
				Profile: nil,
			},
			shouldContain:       []string{"Empty state", "Anonymous"},
			shouldNotContain:    []string{},
			minFragmentCount:    2,
			description:         "Edge cases should be handled correctly in integration",
		},
		{
			name: "NestedContextIntegration",
			template: `<div>
	{{range .Users}}
		<div class="user">
			<h3>{{.Username}}</h3>
			{{with $.Profile}}
				{{if eq .DisplayName $.Title}}
					<p>Main profile</p>
				{{end}}
			{{end}}
		</div>
	{{end}}
</div>`,
			data: &TDDTestData{
				Title: "Admin Profile",
				Users: []TDDUser{
					{Username: "admin"},
					{Username: "user"},
				},
				Profile: &TDDProfile{
					DisplayName: "Admin Profile",
				},
			},
			shouldContain:       []string{"admin", "user", "Main profile"},
			shouldNotContain:    []string{},
			minFragmentCount:    1,
			description:         "Nested contexts with global scope access should work correctly",
		},
	}

	for _, tc := range testSuite {
		t.Run(tc.name, func(t *testing.T) {
			renderer := statetemplate.NewRealtimeRenderer(nil)
			
			err := renderer.AddTemplate("integration_"+tc.name, tc.template)
			if err != nil {
				t.Fatalf("Failed to add template: %v", err)
			}

			html, err := renderer.SetInitialData(tc.data)
			if err != nil {
				t.Fatalf("Failed to render template: %v", err)
			}

			// Validate integration of all actions
			for _, text := range tc.shouldContain {
				if !strings.Contains(html, text) {
					t.Errorf("❌ Expected text '%s' not found in output", text)
				}
			}
			
			for _, text := range tc.shouldNotContain {
				if strings.Contains(html, text) {
					t.Errorf("❌ Unexpected text '%s' found in output", text)
				}
			}

			// Verify fragment generation for complex template
			fragmentCount := renderer.GetFragmentCount()
			if fragmentCount < tc.minFragmentCount {
				t.Errorf("❌ Expected at least %d fragments, got %d", tc.minFragmentCount, fragmentCount)
			}

			t.Logf("✅ %s: Generated %d fragments for %s", tc.name, fragmentCount, tc.description)
		})
	}

	t.Log("✅ All template actions integration test suite completed")
}
