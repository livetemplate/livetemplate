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
	Title   string `json:"title"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	
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

// TestTemplateActionComments tests comment actions
func TestTemplateActionComments(t *testing.T) {
	t.Log("🧪 Testing Template Action: Comments")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	// Template with various comment types
	template := `<div>
	{{/* This is a basic comment */}}
	<h1>{{.Title}}</h1>
	{{- /* Comment with whitespace trimming */ -}}
	<p>{{.Message}}</p>
	{{/*
	Multi-line comment
	with multiple lines
	*/}}
	<span>Content after comments</span>
</div>`

	err := renderer.AddTemplate("comments", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title:   "Comment Test",
		Message: "Comments should not appear in output",
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate comments are not in output
	if strings.Contains(html, "This is a basic comment") {
		t.Error("❌ Basic comment appeared in output")
	}
	if strings.Contains(html, "Comment with whitespace trimming") {
		t.Error("❌ Trimming comment appeared in output") 
	}
	if strings.Contains(html, "Multi-line comment") {
		t.Error("❌ Multi-line comment appeared in output")
	}
	
	// Validate actual content is present
	if !strings.Contains(html, "Comment Test") {
		t.Error("❌ Title not found in output")
	}
	if !strings.Contains(html, "Comments should not appear in output") {
		t.Error("❌ Message not found in output")
	}

	t.Log("✅ Comment actions test passed")
}

// TestTemplateActionPipelineOutput tests basic pipeline output
func TestTemplateActionPipelineOutput(t *testing.T) {
	t.Log("🧪 Testing Template Action: Pipeline Output")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
	<h1>{{.Title}}</h1>
	<p>Message: {{.Message}}</p>
	<span>Count: {{.Count}}</span>
	<div>Score: {{.Score}}</div>
</div>`

	err := renderer.AddTemplate("pipeline", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title:   "Pipeline Test",
		Message: "Testing pipeline output",
		Count:   42,
		Score:   3.14159,
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate all pipeline outputs
	if !strings.Contains(html, "Pipeline Test") {
		t.Error("❌ Title pipeline output failed")
	}
	if !strings.Contains(html, "Testing pipeline output") {
		t.Error("❌ Message pipeline output failed")
	}
	if !strings.Contains(html, "42") {
		t.Error("❌ Count pipeline output failed")
	}
	if !strings.Contains(html, "3.14159") {
		t.Error("❌ Score pipeline output failed")
	}

	t.Log("✅ Pipeline output actions test passed")
}

// TestTemplateActionIfStatements tests if/else conditionals
func TestTemplateActionIfStatements(t *testing.T) {
	t.Log("🧪 Testing Template Action: If Statements")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
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
</div>`

	err := renderer.AddTemplate("if_statements", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test case 1: All true conditions
	data1 := &TDDTestData{
		IsVisible:  true,
		IsEnabled:  true,
		HasContent: true,
		Message:    "Content exists",
	}

	html1, err := renderer.SetInitialData(data1)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if !strings.Contains(html1, "Content is visible") {
		t.Error("❌ If true condition failed")
	}
	if !strings.Contains(html1, "Feature enabled") {
		t.Error("❌ If-else true branch failed")
	}
	if !strings.Contains(html1, "Content exists") {
		t.Error("❌ If with content failed")
	}

	// Test case 2: All false conditions
	data2 := &TDDTestData{
		IsVisible:  false,
		IsEnabled:  false,
		HasContent: false,
	}

	html2, err := renderer.SetInitialData(data2)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if strings.Contains(html2, "Content is visible") {
		t.Error("❌ If false condition should not render content")
	}
	if !strings.Contains(html2, "Feature disabled") {
		t.Error("❌ If-else false branch failed")
	}
	if !strings.Contains(html2, "No content available") {
		t.Error("❌ If-else false branch for content failed")
	}

	t.Log("✅ If statement actions test passed")
}

// TestTemplateActionIfElseChains tests if-else-if chains
func TestTemplateActionIfElseChains(t *testing.T) {
	t.Log("🧪 Testing Template Action: If-Else Chains")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
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

	err := renderer.AddTemplate("if_chains", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test various conditions
	testCases := []struct {
		name           string
		count          int
		score          float64
		expectedCount  string
		expectedScore  string
	}{
		{"Zero count", 0, 0, "No items", "Zero score"},
		{"One count", 1, 25.5, "One item", "Low score"},
		{"Few count", 5, 65.5, "Few items (5)", "Medium score"},
		{"Many count", 20, 95.0, "Many items (20)", "High score"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
		})
	}

	t.Log("✅ If-else chain actions test passed")
}

// TestTemplateActionRangeLoops tests range iterations
func TestTemplateActionRangeLoops(t *testing.T) {
	t.Log("🧪 Testing Template Action: Range Loops")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
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

	err := renderer.AddTemplate("range_loops", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test with populated data
	data1 := &TDDTestData{
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
	}

	html1, err := renderer.SetInitialData(data1)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate range outputs
	if !strings.Contains(html1, "First Item (1)") || !strings.Contains(html1, "Second Item (2)") {
		t.Error("❌ Range over items failed")
	}
	if !strings.Contains(html1, "go") || !strings.Contains(html1, "template") || !strings.Contains(html1, "test") {
		t.Error("❌ Range over tags failed")
	}
	if !strings.Contains(html1, "User 0: alice") || !strings.Contains(html1, "User 1: bob") {
		t.Error("❌ Range with index failed")
	}
	if !strings.Contains(html1, `class="number">1<`) || !strings.Contains(html1, `class="number">5<`) {
		t.Error("❌ Range over numbers failed")
	}

	// Test empty ranges (should trigger else)
	data2 := &TDDTestData{
		Items:   []TDDItem{},
		Tags:    []string{},
		Users:   []TDDUser{},
		Numbers: []int{},
	}

	html2, err := renderer.SetInitialData(data2)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if !strings.Contains(html2, "No tags") {
		t.Error("❌ Range else clause failed")
	}

	t.Log("✅ Range loop actions test passed")
}

// TestTemplateActionWithStatements tests with context changes
func TestTemplateActionWithStatements(t *testing.T) {
	t.Log("🧪 Testing Template Action: With Statements")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
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

	err := renderer.AddTemplate("with_statements", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test with populated data
	data1 := &TDDTestData{
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
	}

	html1, err := renderer.SetInitialData(data1)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if !strings.Contains(html1, "John Doe") || !strings.Contains(html1, "Software Developer") {
		t.Error("❌ With profile context failed")
	}
	if !strings.Contains(html1, "Theme: dark") || !strings.Contains(html1, "Language: en") {
		t.Error("❌ With settings context failed")
	}
	if !strings.Contains(html1, "Debug: true") || !strings.Contains(html1, "Log Level: info") {
		t.Error("❌ Nested with context failed")
	}

	// Test with missing data (should trigger else)
	data2 := &TDDTestData{
		Profile:  nil,
		Settings: nil,
	}

	html2, err := renderer.SetInitialData(data2)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if strings.Contains(html2, "John Doe") {
		t.Error("❌ With should not render when context is nil")
	}
	if !strings.Contains(html2, "No settings configured") {
		t.Error("❌ With else clause failed")
	}

	t.Log("✅ With statement actions test passed")
}

// TestTemplateActionVariableAssignment tests variable declarations and usage
func TestTemplateActionVariableAssignment(t *testing.T) {
	t.Log("🧪 Testing Template Action: Variable Assignment")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
	<!-- Variable assignment -->
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
	
	<!-- Range with variables -->
	{{range $index, $item := .Items}}
		{{$itemClass := "item"}}
		{{if .Active}}
			{{$itemClass = "item active"}}  
		{{end}}
		<div class="{{$itemClass}}">{{$index}}: {{.Name}}</div>
	{{end}}
	
	<!-- Variable scope test -->
	{{$outerVar := "outer"}}
	{{with .Profile}}
		{{$innerVar := "inner"}}
		<div>{{$outerVar}} - {{$innerVar}} - {{.DisplayName}}</div>
	{{end}}
</div>`

	err := renderer.AddTemplate("variables", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title: "Variable Test",
		Count: 3,
		Items: []TDDItem{
			{ID: "1", Name: "Active Item", Active: true},
			{ID: "2", Name: "Inactive Item", Active: false},
		},
		Profile: &TDDProfile{
			DisplayName: "Test User",
		},
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate variable usage
	if !strings.Contains(html, "Variable Test") {
		t.Error("❌ Variable assignment for title failed")
	}
	if !strings.Contains(html, "Item count: 3") {
		t.Error("❌ Variable assignment for count failed")
	}
	if !strings.Contains(html, "Found 3 items") {
		t.Error("❌ Variable condition usage failed")
	}
	if !strings.Contains(html, "0: Active Item") {
		t.Error("❌ Range variable usage failed")
	}
	if !strings.Contains(html, "outer - inner - Test User") {
		t.Error("❌ Variable scope test failed")
	}

	t.Log("✅ Variable assignment actions test passed")
}

// TestTemplateActionWhitespaceTrimming tests whitespace control
func TestTemplateActionWhitespaceTrimming(t *testing.T) {
	t.Log("🧪 Testing Template Action: Whitespace Trimming")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
	{{- .Title -}}
	
	{{- if .IsVisible -}}
		{{- .Message -}}
	{{- end -}}
	
	<span>{{- .Count -}}</span>
	
	{{- range .Tags -}}
		<tag>{{- . -}}</tag>
	{{- end -}}
</div>`

	err := renderer.AddTemplate("whitespace", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title:     "TrimTest",
		IsVisible: true,
		Message:   "NoSpaces",
		Count:     42,
		Tags:      []string{"tag1", "tag2"},
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Check that excessive whitespace is trimmed
	if strings.Contains(html, "  TrimTest") || strings.Contains(html, "TrimTest  ") {
		t.Error("❌ Title whitespace not properly trimmed")
	}
	if strings.Contains(html, "  NoSpaces") || strings.Contains(html, "NoSpaces  ") {
		t.Error("❌ Message whitespace not properly trimmed")
	}

	// Validate content is still present
	if !strings.Contains(html, "TrimTest") {
		t.Error("❌ Content lost during trimming")
	}
	if !strings.Contains(html, "NoSpaces") {
		t.Error("❌ Message lost during trimming")
	}

	t.Log("✅ Whitespace trimming actions test passed")
}

// TestTemplateActionFunctions tests built-in and comparison functions
func TestTemplateActionFunctions(t *testing.T) {
	t.Log("🧪 Testing Template Action: Functions")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	template := `<div>
	<!-- Comparison functions -->
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
	
	{{if and .IsVisible .IsEnabled}}
		<p>Both visible and enabled</p>
	{{end}}
	
	{{if or .HasContent .ShowDetails}}
		<p>Has content or show details</p>
	{{end}}
	
	<!-- Built-in functions -->
	<p>Items length: {{len .Items}}</p>
	<p>Tags length: {{len .Tags}}</p>
	
	{{if not .IsVisible}}
		<p>Not visible</p>
	{{end}}
	
	<!-- Print functions -->
	<p>{{printf "Formatted: %s (%d)" .Title .Count}}</p>
</div>`

	err := renderer.AddTemplate("functions", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title:       "Function Test",
		Count:       5,
		Score:       85.5,
		Threshold:   75.0,
		IsVisible:   true,
		IsEnabled:   true,
		HasContent:  false,
		ShowDetails: true,
		Items: []TDDItem{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		},
		Tags: []string{"a", "b", "c"},
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate function results
	if !strings.Contains(html, "Count equals 5") {
		t.Error("❌ eq function failed")
	}
	if !strings.Contains(html, "Title is not wrong") {
		t.Error("❌ ne function failed")
	}
	if !strings.Contains(html, "Score above threshold") {
		t.Error("❌ gt function failed")
	}
	if !strings.Contains(html, "Both visible and enabled") {
		t.Error("❌ and function failed")
	}
	if !strings.Contains(html, "Has content or show details") {
		t.Error("❌ or function failed")
	}
	if !strings.Contains(html, "Items length: 2") {
		t.Error("❌ len function for items failed")
	}
	if !strings.Contains(html, "Tags length: 3") {
		t.Error("❌ len function for tags failed")
	}
	if !strings.Contains(html, "Formatted: Function Test (5)") {
		t.Error("❌ printf function failed")
	}

	t.Log("✅ Function actions test passed")
}

// TestTemplateActionBlockDefinitions tests block definitions and overrides
func TestTemplateActionBlockDefinitions(t *testing.T) {
	t.Log("🧪 Testing Template Action: Block Definitions")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	// Template with block definitions (without overrides)
	template := `<div>
	<h1>{{.Title}}</h1>
	
	{{block "header" .}}
		<header>Header: {{.Title}}</header>
	{{end}}
	
	{{block "content" .}}
		<section>Content: {{.Message}}</section>
	{{end}}
	
	{{block "footer" .}}
		<footer>Default footer</footer>
	{{end}}
	
	{{block "sidebar" .}}
		<aside>Sidebar content</aside>
	{{end}}
</div>`

	err := renderer.AddTemplate("blocks", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
		Title:   "Block Test",
		Message: "Testing blocks",
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate block content (blocks are executed in place)
	if !strings.Contains(html, "Header: Block Test") {
		t.Error("❌ Block header execution failed")
	}
	if !strings.Contains(html, "Content: Testing blocks") {
		t.Error("❌ Block content execution failed")
	}
	if !strings.Contains(html, "Default footer") {
		t.Error("❌ Block footer execution failed")
	}
	if !strings.Contains(html, "Sidebar content") {
		t.Error("❌ Block sidebar execution failed")
	}

	t.Log("✅ Block definition actions test passed")
}

// TestTemplateActionRealTimeFragmentGeneration verifies all actions generate proper fragments
func TestTemplateActionRealTimeFragmentGeneration(t *testing.T) {
	t.Log("🧪 Testing Real-time Fragment Generation for All Actions")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	// Comprehensive template with all action types
	template := `<div>
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
</div>`

	err := renderer.AddTemplate("fragment_test", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
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
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Verify fragments are generated
	fragmentCount := renderer.GetFragmentCount()
	if fragmentCount == 0 {
		t.Error("❌ No fragments generated for complex template")
	}

	fragmentIDs := renderer.GetFragmentIDs()
	if len(fragmentIDs) == 0 {
		t.Error("❌ No fragment IDs generated")
	}

	// Verify HTML contains fragment IDs
	if !strings.Contains(html, `id="`) {
		t.Error("❌ No fragment IDs found in rendered HTML")
	}

	t.Logf("✅ Generated %d fragments for comprehensive template", fragmentCount)
	t.Logf("Fragment IDs: %+v", fragmentIDs)

	// Test real-time updates
	renderer.Start()
	defer renderer.Stop()

	updateChan := renderer.GetUpdateChannel()
	var updates []statetemplate.RealtimeUpdate

	// Collect updates
	go func() {
		timeout := time.After(3 * time.Second)
		for {
			select {
			case update := <-updateChan:
				updates = append(updates, update)
			case <-timeout:
				return
			}
		}
	}()

	// Trigger updates by changing data
	newData := *data
	newData.Message = "Updated message"
	newData.IsVisible = false
	newData.Count = 20
	renderer.SendUpdate(&newData)

	time.Sleep(1 * time.Second)

	if len(updates) == 0 {
		t.Error("❌ No real-time updates generated")
	} else {
		t.Logf("✅ Generated %d real-time updates", len(updates))
		for _, update := range updates {
			if update.FragmentID == "" {
				t.Error("❌ Update missing fragment ID")
			}
		}
	}

	t.Log("✅ Real-time fragment generation test passed")
}

// TestAllTemplateActionsTogether tests integration of all actions
func TestAllTemplateActionsTogether(t *testing.T) {
	t.Log("🧪 Testing All Template Actions Integration")
	
	renderer := statetemplate.NewRealtimeRenderer(nil)
	
	// Complex template using all action types together
	template := `<div class="app">
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
				{{with .Avatar}}
					<img src="{{.}}" alt="Avatar">
				{{else}}
					<div class="no-avatar">No avatar</div>
				{{end}}
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
</div>`

	err := renderer.AddTemplate("integration", template)
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	data := &TDDTestData{
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
	}

	html, err := renderer.SetInitialData(data)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	// Validate integration of all actions
	if !strings.Contains(html, "Integration Test App") {
		t.Error("❌ Title variable failed")
	}
	if !strings.Contains(html, "3 users online") {
		t.Error("❌ User count with len function failed")
	}
	if !strings.Contains(html, "Integration User") {
		t.Error("❌ Profile with-statement failed")
	}
	if !strings.Contains(html, "Active Item") {
		t.Error("❌ Range over items failed")
	}
	if !strings.Contains(html, "class=\"item active\"") {
		t.Error("❌ Conditional class in range failed")
	}
	if !strings.Contains(html, "Theme: dark") {
		t.Error("❌ Nested with-statement failed")
	}
	if !strings.Contains(html, "Debug Mode: Enabled") {
		t.Error("❌ Nested conditional failed")
	}
	if !strings.Contains(html, "Above threshold") {
		t.Error("❌ Function with conditional failed")
	}
	if !strings.Contains(html, "Score: 87.50 / 75.00") {
		t.Error("❌ Printf function failed")
	}

	// Verify fragment generation for complex template
	fragmentCount := renderer.GetFragmentCount()
	if fragmentCount < 5 {
		t.Errorf("❌ Expected at least 5 fragments, got %d", fragmentCount)
	}

	t.Log("✅ All template actions integration test passed")
	t.Logf("Generated %d fragments for complex integrated template", fragmentCount)
}
