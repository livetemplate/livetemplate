package livetemplate

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// UserActivity represents a single user action in a journey
type UserActivity struct {
	Type   string      `json:"type"`   // "visit", "add", "edit", "delete", "reorder", "toggle"
	Target string      `json:"target"` // field or item identifier
	Data   interface{} `json:"data"`   // action-specific data
}

// UserJourney represents a sequence of user activities
type UserJourney []UserActivity

// AppState represents a typical application state for testing
type AppState struct {
	Title       string      `json:"title"`
	Items       []Item      `json:"items"`
	ShowMenu    bool        `json:"show_menu"`
	Count       int         `json:"count"`
	Status      string      `json:"status"`
	User        *User       `json:"user,omitempty"`
	Settings    Settings    `json:"settings"`
	ComplexData interface{} `json:"complex_data"`
}

// Item represents a list item in the application
type Item struct {
	ID       string                 `json:"id"`
	Text     string                 `json:"text"`
	Complete bool                   `json:"complete"`
	Priority string                 `json:"priority"`
	Tags     []string               `json:"tags"`
	Metadata map[string]interface{} `json:"metadata"`
}

// User represents user information
type User struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Active bool   `json:"active"`
}

// Settings represents application settings
type Settings struct {
	Theme         string `json:"theme"`
	Notifications bool   `json:"notifications"`
	Language      string `json:"language"`
}

// UpdateValidator tracks and validates tree updates according to specification
type UpdateValidator struct {
	FirstRenderSeen bool
	SentStatics     map[string]bool // Track which fields have sent statics
	LastTree        treeNode
	LastState       interface{}
	UpdateCount     int
	Violations      []string
}

// NewUpdateValidator creates a new validator instance
func NewUpdateValidator() *UpdateValidator {
	return &UpdateValidator{
		SentStatics: make(map[string]bool),
		Violations:  make([]string, 0),
	}
}

// ValidateUpdate checks if an update follows the specification rules
func (v *UpdateValidator) ValidateUpdate(tree treeNode, state interface{}, isFirst bool) error {
	v.UpdateCount++

	if isFirst {
		// First render validation
		if err := v.validateFirstRender(tree); err != nil {
			v.Violations = append(v.Violations, fmt.Sprintf("Update %d (first): %v", v.UpdateCount, err))
			return err
		}
		v.FirstRenderSeen = true
		v.markStaticsSent(tree, "")
	} else {
		// Subsequent update validation
		if !v.FirstRenderSeen {
			err := fmt.Errorf("received update before first render")
			v.Violations = append(v.Violations, fmt.Sprintf("Update %d: %v", v.UpdateCount, err))
			return err
		}

		if err := v.validateSubsequentUpdate(tree, v.LastTree); err != nil {
			v.Violations = append(v.Violations, fmt.Sprintf("Update %d: %v", v.UpdateCount, err))
			return err
		}
	}

	v.LastTree = tree
	v.LastState = state
	return nil
}

// validateFirstRender ensures first render has complete statics
func (v *UpdateValidator) validateFirstRender(tree treeNode) error {
	// Must have statics array
	statics, hasStatics := tree["s"].([]string)
	if !hasStatics {
		return fmt.Errorf("first render missing 's' (statics) key")
	}

	if len(statics) == 0 {
		return fmt.Errorf("first render has empty statics array")
	}

	// Count dynamics (numeric keys)
	dynamicCount := 0
	for k := range tree {
		if k != "s" && k != "f" && k != "d" {
			if _, err := fmt.Sscanf(k, "%d", new(int)); err == nil {
				dynamicCount++
			}
		}
	}

	// Validate statics array length (should be dynamics + 1 typically)
	// This is a soft check as templates may vary
	if len(statics) < dynamicCount {
		return fmt.Errorf("statics array length %d < dynamic count %d", len(statics), dynamicCount)
	}

	return nil
}

// validateSubsequentUpdate ensures updates only contain changes
func (v *UpdateValidator) validateSubsequentUpdate(tree, lastTree treeNode) error {
	// Check for unnecessary statics
	for k, value := range tree {
		if k == "s" {
			// Statics should not be sent unless it's a new structure
			if v.SentStatics[k] {
				return fmt.Errorf("update contains statics for already-sent field %s", k)
			}
		}

		// For nested structures, check recursively
		if nestedTree, ok := value.(treeNode); ok {
			if _, hasStatics := nestedTree["s"]; hasStatics {
				fieldPath := k
				if v.SentStatics[fieldPath] {
					return fmt.Errorf("update contains nested statics for already-sent field %s", fieldPath)
				}
			}
		}
	}

	// Validate range operations are granular
	for k, value := range tree {
		if k == "d" || strings.HasSuffix(k, ".d") {
			if err := v.validateRangeOperations(value); err != nil {
				return fmt.Errorf("range operation validation failed: %w", err)
			}
		}
	}

	return nil
}

// validateRangeOperations ensures range updates are granular
func (v *UpdateValidator) validateRangeOperations(value interface{}) error {
	// Check if this is a range operation array
	if ops, ok := value.([]interface{}); ok {
		for _, op := range ops {
			if opArray, ok := op.([]interface{}); ok && len(opArray) > 0 {
				opType, _ := opArray[0].(string)
				switch opType {
				case "i", "r", "u", "o":
					// Valid granular operations
					continue
				default:
					// If it's not an operation, it might be a full item list
					// This would be a violation for updates
					if v.UpdateCount > 1 {
						return fmt.Errorf("non-granular range update detected (full list instead of operations)")
					}
				}
			}
		}
	}
	return nil
}

// markStaticsSent tracks which fields have sent their statics
func (v *UpdateValidator) markStaticsSent(tree treeNode, prefix string) {
	for k, value := range tree {
		fieldPath := prefix + k
		if k == "s" {
			v.SentStatics[prefix] = true
		}

		// Recursively mark nested structures
		if nestedTree, ok := value.(treeNode); ok {
			v.markStaticsSent(nestedTree, fieldPath+".")
		}
		if nestedMap, ok := value.(map[string]interface{}); ok {
			v.markStaticsSent(nestedMap, fieldPath+".")
		}
	}
}

// ActivityGenerator generates random user activities
type ActivityGenerator struct {
	Rand *rand.Rand
}

// NewActivityGenerator creates a new activity generator
func NewActivityGenerator(seed int64) *ActivityGenerator {
	return &ActivityGenerator{
		Rand: rand.New(rand.NewSource(seed)),
	}
}

// GenerateJourney creates a random user journey
func (g *ActivityGenerator) GenerateJourney(length int) UserJourney {
	journey := make(UserJourney, 0, length)

	// Always start with a visit
	journey = append(journey, UserActivity{
		Type: "visit",
		Data: nil,
	})

	// Generate random activities
	activityTypes := []string{"add", "edit", "delete", "reorder", "toggle", "update_field"}

	for i := 1; i < length; i++ {
		actType := activityTypes[g.Rand.Intn(len(activityTypes))]

		activity := UserActivity{
			Type: actType,
		}

		switch actType {
		case "add":
			activity.Target = "items"
			activity.Data = g.generateItem()
		case "edit":
			activity.Target = fmt.Sprintf("item_%d", g.Rand.Intn(10))
			activity.Data = map[string]interface{}{
				"text": g.generateText(),
			}
		case "delete":
			activity.Target = fmt.Sprintf("item_%d", g.Rand.Intn(10))
		case "reorder":
			activity.Target = "items"
			activity.Data = g.generateOrder(g.Rand.Intn(10) + 1)
		case "toggle":
			activity.Target = g.randomChoice([]string{"show_menu", "notifications", "active"})
		case "update_field":
			activity.Target = g.randomChoice([]string{"title", "count", "status"})
			activity.Data = g.generateFieldValue(activity.Target)
		}

		journey = append(journey, activity)
	}

	return journey
}

// generateItem creates a random item
func (g *ActivityGenerator) generateItem() Item {
	return Item{
		ID:       fmt.Sprintf("item_%d", g.Rand.Intn(10000)),
		Text:     g.generateText(),
		Complete: g.Rand.Float32() > 0.5,
		Priority: g.randomChoice([]string{"low", "medium", "high"}),
		Tags:     g.generateTags(),
		Metadata: g.generateMetadata(),
	}
}

// generateText creates random text content
func (g *ActivityGenerator) generateText() string {
	words := []string{"task", "todo", "item", "work", "project", "feature", "bug", "test"}
	count := g.Rand.Intn(5) + 1
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = words[g.Rand.Intn(len(words))]
	}
	return strings.Join(result, " ")
}

// generateTags creates random tags
func (g *ActivityGenerator) generateTags() []string {
	tags := []string{"urgent", "backend", "frontend", "bug", "feature", "docs"}
	count := g.Rand.Intn(3)
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = tags[g.Rand.Intn(len(tags))]
	}
	return result
}

// generateMetadata creates random metadata
func (g *ActivityGenerator) generateMetadata() map[string]interface{} {
	meta := make(map[string]interface{})
	if g.Rand.Float32() > 0.5 {
		meta["created_at"] = "2025-01-01"
	}
	if g.Rand.Float32() > 0.5 {
		meta["author"] = g.randomChoice([]string{"alice", "bob", "charlie"})
	}
	return meta
}

// generateOrder creates a random order array
func (g *ActivityGenerator) generateOrder(count int) []string {
	order := make([]string, count)
	for i := 0; i < count; i++ {
		order[i] = fmt.Sprintf("item_%d", i)
	}
	// Shuffle
	for i := range order {
		j := g.Rand.Intn(i + 1)
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// generateFieldValue creates a value for a field
func (g *ActivityGenerator) generateFieldValue(field string) interface{} {
	switch field {
	case "title":
		return g.generateText()
	case "count":
		return g.Rand.Intn(100)
	case "status":
		return g.randomChoice([]string{"active", "inactive", "pending", "complete"})
	default:
		return g.generateText()
	}
}

// randomChoice selects a random element from slice
func (g *ActivityGenerator) randomChoice(choices []string) string {
	return choices[g.Rand.Intn(len(choices))]
}

// StateSimulator simulates application state changes based on activities
type StateSimulator struct {
	State AppState
}

// NewStateSimulator creates a new state simulator
func NewStateSimulator() *StateSimulator {
	return &StateSimulator{
		State: AppState{
			Title:    "Test App",
			Items:    []Item{},
			ShowMenu: false,
			Count:    0,
			Status:   "active",
			Settings: Settings{
				Theme:         "light",
				Notifications: true,
				Language:      "en",
			},
		},
	}
}

// ApplyActivity applies a user activity to the state
func (s *StateSimulator) ApplyActivity(activity UserActivity) {
	switch activity.Type {
	case "visit":
		// Initial state already set
		return

	case "add":
		if item, ok := activity.Data.(Item); ok {
			s.State.Items = append(s.State.Items, item)
			s.State.Count = len(s.State.Items)
		}

	case "edit":
		// Find and edit item by target ID
		for i := range s.State.Items {
			if s.State.Items[i].ID == activity.Target {
				if updates, ok := activity.Data.(map[string]interface{}); ok {
					if text, ok := updates["text"].(string); ok {
						s.State.Items[i].Text = text
					}
				}
				break
			}
		}

	case "delete":
		// Remove item by target ID
		newItems := []Item{}
		for _, item := range s.State.Items {
			if item.ID != activity.Target {
				newItems = append(newItems, item)
			}
		}
		s.State.Items = newItems
		s.State.Count = len(s.State.Items)

	case "toggle":
		switch activity.Target {
		case "show_menu":
			s.State.ShowMenu = !s.State.ShowMenu
		case "notifications":
			s.State.Settings.Notifications = !s.State.Settings.Notifications
		case "active":
			if s.State.User != nil {
				s.State.User.Active = !s.State.User.Active
			}
		}

	case "update_field":
		switch activity.Target {
		case "title":
			if val, ok := activity.Data.(string); ok {
				s.State.Title = val
			}
		case "count":
			if val, ok := activity.Data.(int); ok {
				s.State.Count = val
			}
		case "status":
			if val, ok := activity.Data.(string); ok {
				s.State.Status = val
			}
		}
	}
}

// GetState returns a copy of the current state
func (s *StateSimulator) GetState() AppState {
	return s.State
}

// FuzzUserJourneys tests random user journey sequences
func FuzzUserJourneys(f *testing.F) {
	// Add seed corpus
	seedJourneys := []string{
		`[{"type":"visit"},{"type":"add","target":"items"}]`,
		`[{"type":"visit"},{"type":"toggle","target":"show_menu"}]`,
		`[{"type":"visit"},{"type":"add","target":"items"},{"type":"delete","target":"item_0"}]`,
	}

	for _, seed := range seedJourneys {
		f.Add(seed)
	}

	// Template for testing
	todoTemplate := `<div>
	<h1>{{.title}}</h1>
	<div>Count: {{.count}}</div>
	{{if .show_menu}}
		<nav>Menu is visible</nav>
	{{end}}
	<ul>
	{{range .items}}
		<li data-id="{{.id}}">
			{{.text}}
			{{if .complete}}✓{{else}}○{{end}}
			Priority: {{.priority}}
		</li>
	{{end}}
	</ul>
	<footer>Status: {{.status}}</footer>
</div>`

	f.Fuzz(func(t *testing.T, journeyJSON string) {
		// Parse journey
		var journey UserJourney
		if err := json.Unmarshal([]byte(journeyJSON), &journey); err != nil {
			t.Skip("Invalid journey JSON")
		}

		if len(journey) == 0 {
			t.Skip("Empty journey")
		}

		// Create validator and simulator
		validator := NewUpdateValidator()
		simulator := NewStateSimulator()

		// Create template
		tmpl := &Template{
			templateStr: todoTemplate,
			keyGen:      newKeyGenerator(),
		}

		// Parse template
		if _, err := tmpl.Parse(tmpl.templateStr); err != nil {
			t.Fatalf("Failed to parse template: %v", err)
		}

		// Execute journey
		for i, activity := range journey {
			// Apply activity to state
			simulator.ApplyActivity(activity)
			state := simulator.GetState()

			// Generate tree update
			var tree treeNode
			var err error

			if i == 0 && activity.Type == "visit" {
				// First render
				tree, err = tmpl.generateInitialTree(todoTemplate, state)
				if err != nil {
					t.Fatalf("Failed to generate initial tree: %v", err)
				}

				// Validate first render
				if err := validator.ValidateUpdate(tree, state, true); err != nil {
					t.Errorf("First render validation failed: %v", err)
				}
			} else {
				// Subsequent update
				if tmpl.lastTree == nil {
					t.Skip("No previous tree for comparison")
				}

				// Generate new tree and compare
				newTree, err := parseTemplateToTree(todoTemplate, state, tmpl.keyGen)
				if err != nil {
					t.Fatalf("Failed to generate tree: %v", err)
				}

				// Get changes only
				tree = tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)

				// Validate update
				if err := validator.ValidateUpdate(tree, state, false); err != nil {
					t.Errorf("Update %d validation failed: %v", i, err)
				}

				tmpl.lastTree = newTree
			}
		}

		// Check for any violations
		if len(validator.Violations) > 0 {
			t.Errorf("Specification violations found:\n%s",
				strings.Join(validator.Violations, "\n"))
		}
	})
}

// TestSpecificationCompliance runs specific compliance tests
func TestSpecificationCompliance(t *testing.T) {
	tests := []struct {
		name     string
		template string
		journey  UserJourney
		wantErr  bool
	}{
		{
			name:     "first_render_has_statics",
			template: `<div>{{.title}}</div>`,
			journey: UserJourney{
				{Type: "visit"},
			},
			wantErr: false,
		},
		{
			name:     "update_no_statics",
			template: `<div>{{.count}}</div>`,
			journey: UserJourney{
				{Type: "visit"},
				{Type: "update_field", Target: "count", Data: 5},
			},
			wantErr: false,
		},
		{
			name:     "range_insert_granular",
			template: `{{range .items}}<li>{{.text}}</li>{{end}}`,
			journey: UserJourney{
				{Type: "visit"},
				{Type: "add", Target: "items", Data: Item{ID: "1", Text: "First"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewUpdateValidator()
			simulator := NewStateSimulator()

			tmpl := &Template{
				templateStr: tt.template,
				keyGen:      newKeyGenerator(),
			}

			if _, err := tmpl.Parse(tmpl.templateStr); err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			for i, activity := range tt.journey {
				simulator.ApplyActivity(activity)
				state := simulator.GetState()

				var tree treeNode
				var err error

				if i == 0 {
					tree, err = tmpl.generateInitialTree(tt.template, state)
				} else {
					if tmpl.lastTree == nil {
						continue
					}
					newTree, _ := parseTemplateToTree(tt.template, state, tmpl.keyGen)
					tree = tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
					tmpl.lastTree = newTree
				}

				if err != nil && !tt.wantErr {
					t.Errorf("Unexpected error: %v", err)
				}

				if err := validator.ValidateUpdate(tree, state, i == 0); err != nil && !tt.wantErr {
					t.Errorf("Validation failed: %v", err)
				}
			}
		})
	}
}

// TestRangeOperationGranularity specifically tests range operation granularity
func TestRangeOperationGranularity(t *testing.T) {
	template := `{{range .items}}<div>{{.id}}: {{.text}}</div>{{end}}`

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}

	if _, err := tmpl.Parse(tmpl.templateStr); err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	// Initial state with items
	state1 := AppState{
		Items: []Item{
			{ID: "1", Text: "First"},
			{ID: "2", Text: "Second"},
		},
	}

	// Generate initial tree
	tree1, _ := parseTemplateToTree(template, state1, tmpl.keyGen)
	tmpl.lastTree = tree1

	// Add one item
	state2 := AppState{
		Items: []Item{
			{ID: "1", Text: "First"},
			{ID: "2", Text: "Second"},
			{ID: "3", Text: "Third"},
		},
	}

	tree2, _ := parseTemplateToTree(template, state2, tmpl.keyGen)
	changes := tmpl.compareTreesAndGetChanges(tree1, tree2)

	// Verify the update contains only an insert operation
	if rangeOps, ok := changes["0"].([]interface{}); ok {
		if len(rangeOps) != 1 {
			t.Errorf("Expected 1 range operation, got %d", len(rangeOps))
		}

		if op, ok := rangeOps[0].([]interface{}); ok {
			if op[0] != "i" {
				t.Errorf("Expected insert operation 'i', got %v", op[0])
			}
		}
	} else {
		// Check if it's sending the full list (violation)
		if fullList, ok := changes["0"].(map[string]interface{}); ok {
			if d, hasD := fullList["d"]; hasD {
				t.Errorf("Update sent full list 'd' instead of granular operation: %v", d)
			}
		}
	}
}

// BenchmarkUserJourney measures performance of user journey processing
func BenchmarkUserJourney(b *testing.B) {
	generator := NewActivityGenerator(42)
	journey := generator.GenerateJourney(100) // 100 activities

	template := `<div>
		{{.title}}
		{{range .items}}<li>{{.text}}</li>{{end}}
		Count: {{.count}}
	</div>`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		simulator := NewStateSimulator()
		tmpl := &Template{
			templateStr: template,
			keyGen:      newKeyGenerator(),
		}
		_, _ = tmpl.Parse(tmpl.templateStr)

		for j, activity := range journey {
			simulator.ApplyActivity(activity)
			state := simulator.GetState()

			if j == 0 {
				_, _ = tmpl.generateInitialTree(template, state)
			} else {
				newTree, _ := parseTemplateToTree(template, state, tmpl.keyGen)
				tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
				tmpl.lastTree = newTree
			}
		}
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "empty_to_content_transition",
			test: testEmptyToContent,
		},
		{
			name: "large_list_operations",
			test: testLargeList,
		},
		{
			name: "deep_nesting",
			test: testDeepNesting,
		},
		{
			name: "rapid_updates",
			test: testRapidUpdates,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func testEmptyToContent(t *testing.T) {
	template := `{{range .items}}{{.}}{{else}}No items{{end}}`

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}
	_, _ = tmpl.Parse(tmpl.templateStr)

	// Start with empty
	emptyState := AppState{Items: []Item{}}
	tree1, _ := parseTemplateToTree(template, emptyState, tmpl.keyGen)

	// Should show "No items"
	if tree1["0"] != "No items" {
		t.Errorf("Empty state should show 'No items', got %v", tree1["0"])
	}

	// Add items
	withItemsState := AppState{
		Items: []Item{{ID: "1", Text: "First"}},
	}
	tree2, _ := parseTemplateToTree(template, withItemsState, tmpl.keyGen)

	changes := tmpl.compareTreesAndGetChanges(tree1, tree2)

	// Should have the new structure
	if changes["0"] == nil {
		t.Error("Expected changes for empty to content transition")
	}
}

func testLargeList(t *testing.T) {
	template := `{{range .items}}<div>{{.id}}</div>{{end}}`

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}
	_, _ = tmpl.Parse(tmpl.templateStr)

	// Create large list
	items := make([]Item, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = Item{ID: fmt.Sprintf("item_%d", i)}
	}

	state := AppState{Items: items}
	tree, err := parseTemplateToTree(template, state, tmpl.keyGen)

	if err != nil {
		t.Fatalf("Failed to handle large list: %v", err)
	}

	// Verify structure
	if rangeData, ok := tree["0"].(map[string]interface{}); ok {
		if d, ok := rangeData["d"].([]interface{}); ok {
			if len(d) != 1000 {
				t.Errorf("Expected 1000 items, got %d", len(d))
			}
		}
	}
}

func testDeepNesting(t *testing.T) {
	// Build deeply nested template
	template := `{{if .l1}}{{if .l2}}{{if .l3}}{{if .l4}}{{if .l5}}
		{{if .l6}}{{if .l7}}{{if .l8}}{{if .l9}}{{if .l10}}
			Deep content
		{{end}}{{end}}{{end}}{{end}}{{end}}
	{{end}}{{end}}{{end}}{{end}}{{end}}`

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}
	_, _ = tmpl.Parse(tmpl.templateStr)

	state := map[string]interface{}{
		"l1": true, "l2": true, "l3": true, "l4": true, "l5": true,
		"l6": true, "l7": true, "l8": true, "l9": true, "l10": true,
	}

	tree, err := parseTemplateToTree(template, state, tmpl.keyGen)
	if err != nil {
		t.Fatalf("Failed to handle deep nesting: %v", err)
	}

	// Verify we can find the deep content
	found := findDeepContent(tree, "Deep content", 0, 10)
	if !found {
		t.Error("Failed to find deep content in nested structure")
	}
}

func findDeepContent(node interface{}, target string, depth, maxDepth int) bool {
	if depth > maxDepth {
		return false
	}

	switch v := node.(type) {
	case string:
		return strings.Contains(v, target)
	case treeNode:
		for _, val := range v {
			if findDeepContent(val, target, depth+1, maxDepth) {
				return true
			}
		}
	case map[string]interface{}:
		for _, val := range v {
			if findDeepContent(val, target, depth+1, maxDepth) {
				return true
			}
		}
	}
	return false
}

func testRapidUpdates(t *testing.T) {
	template := `<div>{{.count}}</div>`

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}
	_, _ = tmpl.Parse(tmpl.templateStr)

	validator := NewUpdateValidator()

	// Simulate rapid counter updates
	for i := 0; i < 100; i++ {
		state := AppState{Count: i}

		if i == 0 {
			tree, _ := tmpl.generateInitialTree(template, state)
			_ = validator.ValidateUpdate(tree, state, true)
		} else {
			newTree, _ := parseTemplateToTree(template, state, tmpl.keyGen)
			changes := tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)

			// Should only have the count change
			if len(changes) != 1 {
				t.Errorf("Update %d: Expected 1 change, got %d", i, len(changes))
			}

			_ = validator.ValidateUpdate(changes, state, false)
			tmpl.lastTree = newTree
		}
	}

	if len(validator.Violations) > 0 {
		t.Errorf("Rapid updates caused violations: %v", validator.Violations)
	}
}

// TestComplexScenarios tests complex real-world scenarios
func TestComplexScenarios(t *testing.T) {
	// Test a complex template with multiple dynamic regions
	template := `
<div class="app">
	<header>
		<h1>{{.title}}</h1>
		{{if .user}}
			<div class="user">Welcome {{.user.name}}</div>
		{{else}}
			<button>Login</button>
		{{end}}
	</header>

	<nav class="{{if .show_menu}}visible{{else}}hidden{{end}}">
		{{range .menu_items}}
			<a href="{{.link}}">{{.text}}</a>
		{{end}}
	</nav>

	<main>
		<section class="stats">
			<div>Total: {{.count}}</div>
			<div>Active: {{.active_count}}</div>
		</section>

		{{range .items}}
		<article data-id="{{.id}}" class="{{if .complete}}done{{end}}">
			<h3>{{.text}}</h3>
			{{if .tags}}
				<div class="tags">
					{{range .tags}}<span>{{.}}</span>{{end}}
				</div>
			{{end}}
		</article>
		{{end}}
	</main>

	<footer>{{.status}} | {{.settings.theme}}</footer>
</div>`

	// Create a journey that exercises all parts
	journey := UserJourney{
		{Type: "visit"},
		{Type: "update_field", Target: "title", Data: "My App"},
		{Type: "add", Target: "items", Data: Item{
			ID:   "1",
			Text: "First task",
			Tags: []string{"urgent"},
		}},
		{Type: "toggle", Target: "show_menu"},
		{Type: "add", Target: "items", Data: Item{
			ID:       "2",
			Text:     "Second task",
			Complete: true,
		}},
		{Type: "edit", Target: "1", Data: map[string]interface{}{
			"text": "Updated first task",
		}},
		{Type: "delete", Target: "2"},
	}

	tmpl := &Template{
		templateStr: template,
		keyGen:      newKeyGenerator(),
	}
	_, _ = tmpl.Parse(tmpl.templateStr)

	simulator := NewStateSimulator()
	validator := NewUpdateValidator()

	for i, activity := range journey {
		simulator.ApplyActivity(activity)
		state := simulator.GetState()

		if i == 0 {
			tree, _ := tmpl.generateInitialTree(template, state)
			if err := validator.ValidateUpdate(tree, state, true); err != nil {
				t.Errorf("Step %d failed: %v", i, err)
			}
		} else {
			newTree, _ := parseTemplateToTree(template, state, tmpl.keyGen)
			changes := tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)

			if err := validator.ValidateUpdate(changes, state, false); err != nil {
				t.Errorf("Step %d failed: %v", i, err)
			}

			tmpl.lastTree = newTree
		}
	}
}

// TestRegressionCases tests specific known issues
func TestRegressionCases(t *testing.T) {
	t.Run("mixed_template_with_ranges", func(t *testing.T) {
		// This was a known issue where templates with ranges + other dynamics failed
		template := `
			<h1>{{.title}}</h1>
			{{range .items}}<li>{{.}}</li>{{end}}
			<footer>{{.footer}}</footer>`

		tmpl := &Template{
			templateStr: template,
			keyGen:      newKeyGenerator(),
		}
		_, _ = tmpl.Parse(tmpl.templateStr)

		state := map[string]interface{}{
			"title":  "Test",
			"items":  []string{"A", "B", "C"},
			"footer": "Footer text",
		}

		tree, err := parseTemplateToTree(template, state, tmpl.keyGen)
		if err != nil {
			t.Fatalf("Failed to handle mixed template: %v", err)
		}

		// Should have all three dynamics working
		if tree["0"] != "Test" {
			t.Error("Title dynamic not working")
		}

		// The range should be at some numeric key
		foundRange := false
		foundFooter := false
		for _, v := range tree {
			if reflect.TypeOf(v).Kind() == reflect.Map {
				if m, ok := v.(map[string]interface{}); ok {
					if _, hasD := m["d"]; hasD {
						foundRange = true
					}
				}
			}
			if v == "Footer text" {
				foundFooter = true
			}
		}

		if !foundRange {
			t.Error("Range dynamic not working")
		}
		if !foundFooter {
			t.Error("Footer dynamic not working")
		}
	})
}
