package livetemplate

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/fuzz/generators"
	"github.com/livetemplate/livetemplate/internal/fuzz/invariants"
	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
	"pgregory.net/rapid"
)

// FuzzDiffCorrectness tests that diff(oldTree, newTree) when applied to oldTree
// produces a tree equivalent to newTree.
//
// This is the core correctness property for the diff algorithm.
func FuzzDiffCorrectness(f *testing.F) {
	// Seed corpus with known mutation sequences
	f.Add(int64(12345), 10)
	f.Add(int64(67890), 20)
	f.Add(int64(11111), 5)
	f.Add(int64(99999), 50)

	f.Fuzz(func(t *testing.T, seed int64, numMutations int) {
		if numMutations < 1 {
			numMutations = 1
		}
		if numMutations > 100 {
			numMutations = 100
		}

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, fuzzTodoTemplate, generators.DefaultStateShape())
	})
}

// FuzzRangeOperations focuses on range-heavy state mutations.
func FuzzRangeOperations(f *testing.F) {
	f.Add(int64(12345), 20)
	f.Add(int64(67890), 30)

	f.Fuzz(func(t *testing.T, seed int64, numMutations int) {
		if numMutations < 1 {
			numMutations = 1
		}
		if numMutations > 100 {
			numMutations = 100
		}

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, fuzzRangeTemplate, generators.RangeHeavyStateShape())
	})
}

// FuzzKeyStability tests that item keys remain stable across renders.
func FuzzKeyStability(f *testing.F) {
	f.Add(int64(12345), 30)

	f.Fuzz(func(t *testing.T, seed int64, numMutations int) {
		if numMutations < 5 {
			numMutations = 5
		}
		if numMutations > 50 {
			numMutations = 50
		}

		rng := rand.New(rand.NewSource(seed))

		// Use range-heavy weights to stress key stability
		weights := mutations.RangeHeavyWeights

		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzRangeTemplate,
			generators.RangeHeavyStateShape(), weights)
	})
}

// runFuzzSession executes a fuzzing session with the given parameters.
func runFuzzSession(t *testing.T, rng *rand.Rand, seed int64, numMutations int,
	templateStr string, shape *generators.StateShape) {

	runFuzzSessionWithWeights(t, rng, seed, numMutations, templateStr, shape,
		mutations.DefaultWeights)
}

func runFuzzSessionWithWeights(t *testing.T, rng *rand.Rand, seed int64, numMutations int,
	templateStr string, shape *generators.StateShape, weights mutations.MutationWeights) {

	// Create template (matching existing test pattern)
	tmpl := &Template{
		templateStr: templateStr,
		keyGen:      compat.NewKeyGenerator(),
	}

	// Parse template
	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Skipf("Template parse error: %v", err)
	}

	// Generate initial state
	state := generators.GenStateSimple(rng, shape)

	// Create verifier
	verifier := invariants.NewVerifier(seed)
	// Enable DiffCorrectness check - oracle should now work correctly
	verifier.SkipDiffCorrectness = false

	// First render - use compat.ParseTemplateToTree for consistency with subsequent renders
	prevTree, err := compat.ParseTemplateToTree("test", templateStr, state, tmpl.keyGen)
	if err != nil {
		t.Fatalf("Initial render failed: %v", err)
	}

	// Convert to build.TreeNode for verifier (identity since treeNode = build.TreeNode)
	prevBuildTree := convertToBuildTree(prevTree)

	// Verify first render invariants
	if err := verifier.VerifyAll(nil, state, nil, prevBuildTree, prevBuildTree, true); err != nil {
		t.Fatalf("First render invariant violation: %v\nTemplate: %s\nState: %+v",
			err, templateStr, state)
	}

	// Set lastTree for subsequent diffs (IMPORTANT: must be set before loop)
	tmpl.lastTree = prevTree

	// Execute mutation sequence
	for i := 0; i < numMutations; i++ {
		// Generate mutation
		mutation := genMutationSimple(rng, shape, state, weights)
		verifier.RecordMutation(mutation)

		// Apply mutation to state
		oldState := mutations.DeepCopy(state)
		newState, err := mutations.Apply(state, mutation)
		if err != nil {
			// Some mutations may fail (e.g., removing from empty slice)
			// This is expected, continue with next mutation
			continue
		}
		state = newState

		// Parse new tree
		newTree, err := compat.ParseTemplateToTree("test", templateStr, state, tmpl.keyGen)
		if err != nil {
			t.Fatalf("Render failed after mutation %d (%s): %v\nState: %+v",
				i, mutation.String(), err, state)
		}

		// Compute diff
		diffTree := tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)

		// Convert to build.TreeNode for verifier
		newBuildTree := convertToBuildTree(newTree)
		diffBuildTree := convertToBuildTree(diffTree)

		// Verify all invariants
		if err := verifier.VerifyAll(oldState, state, prevBuildTree, newBuildTree, diffBuildTree, false); err != nil {
			// Log detailed failure information including diff tree for debugging
			t.Errorf("Invariant violation at mutation %d (seed=%d):\n"+
				"  Mutation: %s\n"+
				"  Error: %v\n"+
				"  Old state: %s\n"+
				"  New state: %s\n"+
				"  Diff tree: %s\n"+
				"  Old tree dynamics: %s\n"+
				"  New tree dynamics: %s",
				i, seed, mutation.String(), err,
				mustJSON(oldState), mustJSON(state),
				mustJSON(diffBuildTree),
				mustJSON(prevBuildTree.Dynamics),
				mustJSON(newBuildTree.Dynamics))
			return
		}

		tmpl.lastTree = newTree
		prevBuildTree = newBuildTree
	}
}

// convertToBuildTree converts internal treeNode to build.TreeNode.
// Since treeNode is a type alias for build.TreeNode, this is a simple cast.
func convertToBuildTree(tree *treeNode) *build.TreeNode {
	// treeNode is a type alias for build.TreeNode (see template.go line 138)
	// so they are identical types - no conversion needed
	return tree
}

// genMutationSimple generates a random mutation using standard rand.
func genMutationSimple(rng *rand.Rand, shape *generators.StateShape, state map[string]any,
	weights mutations.MutationWeights) mutations.Mutation {

	// Calculate cumulative weights
	type weightedMutation struct {
		mutType mutations.MutationType
		weight  float64
	}

	options := []weightedMutation{
		{mutations.MutSetField, weights.SetField},
		{mutations.MutToggleBool, weights.ToggleBool},
		{mutations.MutAppendSlice, weights.AppendSlice},
		{mutations.MutPrependSlice, weights.PrependSlice},
		{mutations.MutInsertSlice, weights.InsertSlice},
		{mutations.MutRemoveSlice, weights.RemoveSlice},
		{mutations.MutClearSlice, weights.ClearSlice},
		{mutations.MutReorderSlice, weights.ReorderSlice},
		{mutations.MutReverseSlice, weights.ReverseSlice},
		{mutations.MutUpdateItem, weights.UpdateItem},
		{mutations.MutKeyCollision, weights.KeyCollision},
	}

	// Select mutation type by weight
	choice := rng.Float64()
	cumulative := 0.0
	var selectedType mutations.MutationType

	for _, opt := range options {
		cumulative += opt.weight
		if choice < cumulative {
			selectedType = opt.mutType
			break
		}
	}

	if selectedType == "" {
		selectedType = mutations.MutSetField // Fallback
	}

	// Generate mutation based on type
	return genMutationOfType(rng, shape, state, selectedType)
}

func genMutationOfType(rng *rand.Rand, shape *generators.StateShape, state map[string]any,
	mutType mutations.MutationType) mutations.Mutation {

	// Get field and slice names
	var fieldNames []string
	for name := range shape.Fields {
		fieldNames = append(fieldNames, name)
	}
	var sliceNames []string
	for name := range shape.Slices {
		sliceNames = append(sliceNames, name)
	}
	var boolFields []string
	for name, ftype := range shape.Fields {
		if ftype == generators.FieldBool {
			boolFields = append(boolFields, name)
		}
	}

	switch mutType {
	case mutations.MutSetField:
		if len(fieldNames) == 0 {
			return mutations.Mutation{Type: mutations.MutSetField, Target: "Title", Value: "default"}
		}
		field := fieldNames[rng.Intn(len(fieldNames))]
		return mutations.Mutation{
			Type:   mutations.MutSetField,
			Target: field,
			Value:  genRandomString(rng),
		}

	case mutations.MutToggleBool:
		if len(boolFields) == 0 {
			return mutations.Mutation{Type: mutations.MutToggleBool, Target: "ShowMenu"}
		}
		return mutations.Mutation{
			Type:   mutations.MutToggleBool,
			Target: boolFields[rng.Intn(len(boolFields))],
		}

	case mutations.MutAppendSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		return mutations.Mutation{
			Type:   mutations.MutAppendSlice,
			Target: sliceName,
			Value:  generators.GenItemSimple(rng, shape.Slices[sliceName].ItemShape),
		}

	case mutations.MutPrependSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		return mutations.Mutation{
			Type:   mutations.MutPrependSlice,
			Target: sliceName,
			Value:  generators.GenItemSimple(rng, shape.Slices[sliceName].ItemShape),
		}

	case mutations.MutInsertSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		slice := getSliceFromState(state, sliceName)
		idx := 0
		if len(slice) > 0 {
			idx = rng.Intn(len(slice))
		}
		return mutations.Mutation{
			Type:   mutations.MutInsertSlice,
			Target: sliceName,
			Index:  idx,
			Value:  generators.GenItemSimple(rng, shape.Slices[sliceName].ItemShape),
		}

	case mutations.MutRemoveSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		slice := getSliceFromState(state, sliceName)
		// Don't remove if it would result in empty slice (not yet supported)
		if len(slice) <= 1 {
			return genMutationOfType(rng, shape, state, mutations.MutUpdateItem)
		}
		idx := rng.Intn(len(slice))
		return mutations.Mutation{
			Type:   mutations.MutRemoveSlice,
			Target: sliceName,
			Index:  idx,
		}

	case mutations.MutClearSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		return mutations.Mutation{
			Type:   mutations.MutClearSlice,
			Target: sliceNames[rng.Intn(len(sliceNames))],
		}

	case mutations.MutReorderSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		slice := getSliceFromState(state, sliceName)
		return mutations.Mutation{
			Type:   mutations.MutReorderSlice,
			Target: sliceName,
			Value:  generators.GenPermutationSimple(rng, len(slice)),
		}

	case mutations.MutReverseSlice:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		return mutations.Mutation{
			Type:   mutations.MutReverseSlice,
			Target: sliceNames[rng.Intn(len(sliceNames))],
		}

	case mutations.MutUpdateItem:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		slice := getSliceFromState(state, sliceName)
		if len(slice) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutAppendSlice)
		}
		return mutations.Mutation{
			Type:   mutations.MutUpdateItem,
			Target: sliceName,
			Index:  rng.Intn(len(slice)),
			Value:  map[string]any{"Text": genRandomString(rng)},
		}

	case mutations.MutKeyCollision:
		if len(sliceNames) == 0 {
			return genMutationOfType(rng, shape, state, mutations.MutSetField)
		}
		sliceName := sliceNames[rng.Intn(len(sliceNames))]
		return mutations.Mutation{
			Type:   mutations.MutKeyCollision,
			Target: sliceName,
		}

	default:
		return mutations.Mutation{
			Type:   mutations.MutSetField,
			Target: "Title",
			Value:  genRandomString(rng),
		}
	}
}

func getSliceFromState(state map[string]any, name string) []any {
	if val, exists := state[name]; exists {
		if slice, ok := val.([]map[string]any); ok {
			result := make([]any, len(slice))
			for i, v := range slice {
				result[i] = v
			}
			return result
		}
		if slice, ok := val.([]any); ok {
			return slice
		}
	}
	return nil
}

func genRandomString(rng *rand.Rand) string {
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	n := rng.Intn(3) + 1
	result := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			result += " "
		}
		result += words[rng.Intn(len(words))]
	}
	return result
}

func mustJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(data)
}

// Test templates (prefixed with fuzz to avoid collision with existing test vars)

var fuzzTodoTemplate = `<div>
	<h1>{{.Title}}</h1>
	<div>Count: {{.Count}}</div>
	{{if .ShowMenu}}
		<nav>Menu is visible</nav>
	{{end}}
	<ul>
	{{range .Items}}
		<li id="{{.ID}}">
			{{.Text}}
			{{if .Complete}}✓{{else}}○{{end}}
		</li>
	{{end}}
	</ul>
	<footer>Status: {{.Status}}</footer>
</div>`

var fuzzRangeTemplate = `<div>
	<h1>{{.Title}}</h1>
	<ul>
	{{range .Items}}
		<li id="{{.ID}}">{{.Text}}</li>
	{{end}}
	</ul>
	<div>
	{{range .Tags}}
		<span id="{{.ID}}" style="color:{{.Color}}">{{.Label}}</span>
	{{end}}
	</div>
</div>`

// Property-based tests using rapid

func TestFuzzDiffCorrectness_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(5, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, fuzzTodoTemplate, generators.DefaultStateShape())
	})
}

func TestFuzzRangeOperations_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 50).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzRangeTemplate,
			generators.RangeHeavyStateShape(), mutations.RangeHeavyWeights)
	})
}

func TestFuzzEdgeCases_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(5, 20).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzTodoTemplate,
			generators.DefaultStateShape(), mutations.EdgeCaseWeights)
	})
}

// Phase 2: Random Template Tests
// These tests use randomly generated templates instead of fixed templates.

// TestFuzzFixedTemplateCorpus_Property runs fuzz tests against all fixed templates.
func TestFuzzFixedTemplateCorpus_Property(t *testing.T) {
	templates := generators.FixedTemplates()

	for i, gt := range templates {
		name := fmt.Sprintf("template_%d", i)
		t.Run(name, func(t *testing.T) {
			rapid.Check(t, func(rt *rapid.T) {
				seed := rapid.Int64().Draw(rt, "seed")
				numMutations := rapid.IntRange(5, 20).Draw(rt, "numMutations")

				rng := rand.New(rand.NewSource(seed))
				runFuzzSession(t, rng, seed, numMutations, gt.Template, gt.Shape)
			})
		})
	}
}

// TestFuzzRandomTemplates_Property generates random templates and tests them.
func TestFuzzRandomTemplates_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random template with its shape
		gt := generators.GenTemplate(generators.DefaultTemplateConfig()).Draw(rt, "template")

		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(5, 15).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, gt.Template, gt.Shape)
	})
}

// TestFuzzSimpleRandomTemplates_Property uses simpler template config for faster testing.
func TestFuzzSimpleRandomTemplates_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a simple random template
		gt := generators.GenTemplate(generators.SimpleTemplateConfig()).Draw(rt, "template")

		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(3, 10).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, gt.Template, gt.Shape)
	})
}

// TestFuzzRandomTemplatesRangeHeavy_Property focuses on range operations with random templates.
func TestFuzzRandomTemplatesRangeHeavy_Property(t *testing.T) {
	// Use a template config that favors ranges
	config := generators.TemplateConfig{
		MaxDepth:        2,
		MaxFields:       3,
		MaxRanges:       3, // More ranges
		MaxConditionals: 1,
		AllowWith:       false,
		AllowNested:     false,
		KeyAttributes: []string{
			`id="{{.ID}}"`,
			`data-key="{{.ID}}"`,
		},
	}

	rapid.Check(t, func(rt *rapid.T) {
		gt := generators.GenTemplate(config).Draw(rt, "template")

		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, gt.Template, gt.Shape,
			mutations.RangeHeavyWeights)
	})
}

// =============================================================================
// TARGETED FUZZ TESTS FOR HISTORICAL BUG PATTERNS
// These tests exercise specific edge cases that caused bugs in the past.
// =============================================================================

// fuzzRangeElseTemplate tests range→else transitions (bug: a87fc41)
// When a range has items and then items are removed, the else clause should render.
var fuzzRangeElseTemplate = `<div>
	<h1>{{.Title}}</h1>
	{{range .Items}}
		<li id="{{.ID}}">{{.Text}}</li>
	{{else}}
		<p>No items found matching "{{$.SearchQuery}}"</p>
	{{end}}
</div>`

// fuzzCheckboxTemplate tests checkbox toggle with statics differing (bug: dc842a1)
// The "checked" attribute appears/disappears based on item.Complete.
var fuzzCheckboxTemplate = `<div>
	<h1>{{.Title}}</h1>
	<ul>
	{{range .Items}}
		<li id="{{.ID}}">
			<input type="checkbox" {{if .Complete}}checked{{end}}>
			<span>{{.Text}}</span>
		</li>
	{{end}}
	</ul>
</div>`

// fuzzEmptyRangeTemplate tests empty→items transitions (bug: 7b675b7)
// When a range starts empty and items are added, statics must be included.
var fuzzEmptyRangeTemplate = `<div>
	<h1>{{.Title}}</h1>
	<ul>
	{{range .Items}}
		<li id="{{.ID}}" class="item">{{.Text}}</li>
	{{end}}
	</ul>
	<div>Count: {{.Count}}</div>
</div>`

// fuzzHeterogeneousRangeTemplate tests heterogeneous range items (bug: b0d3545)
// Different items render different content based on item type.
var fuzzHeterogeneousRangeTemplate = `<div>
	<h1>{{.Title}}</h1>
	<ul>
	{{range .Items}}
		<li id="{{.ID}}">
			{{if .IsHeader}}
				<h2>{{.Text}}</h2>
			{{else if .IsLink}}
				<a href="{{.URL}}">{{.Text}}</a>
			{{else}}
				<span>{{.Text}}</span>
			{{end}}
		</li>
	{{end}}
	</ul>
</div>`

// fuzzNestedConditionalRangeTemplate tests nested conditionals in ranges
var fuzzNestedConditionalRangeTemplate = `<div>
	{{if .ShowList}}
		<ul>
		{{range .Items}}
			<li id="{{.ID}}">
				{{if .Important}}
					<strong>{{.Text}}</strong>
				{{else}}
					{{.Text}}
				{{end}}
			</li>
		{{end}}
		</ul>
	{{else}}
		<p>List is hidden</p>
	{{end}}
</div>`

// TestFuzzRangeElseTransitions_Property tests range→else transitions specifically.
// This bug caused repeated messages when items were cleared.
func TestFuzzRangeElseTransitions_Property(t *testing.T) {
	// Shape that includes SearchQuery for else clause
	shape := &generators.StateShape{
		Fields: map[string]generators.FieldType{
			"Title":       generators.FieldString,
			"SearchQuery": generators.FieldString,
		},
		Slices: map[string]generators.SliceShape{
			"Items": {
				MinLen: 0, // Allow empty to trigger else
				MaxLen: 5,
				ItemShape: &generators.StateShape{
					Fields: map[string]generators.FieldType{
						"ID":   generators.FieldString,
						"Text": generators.FieldString,
					},
				},
			},
		},
	}

	// Weights that favor clearing slices to test else transitions
	weights := mutations.MutationWeights{
		SetField:     0.1,
		ToggleBool:   0.0,
		AppendSlice:  0.15,
		PrependSlice: 0.05,
		InsertSlice:  0.05,
		RemoveSlice:  0.25, // Heavy remove
		ClearSlice:   0.2,  // Heavy clear to trigger else
		ReorderSlice: 0.05,
		ReverseSlice: 0.05,
		UpdateItem:   0.1,
		KeyCollision: 0.0,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzRangeElseTemplate, shape, weights)
	})
}

// TestFuzzCheckboxToggle_Property tests checkbox toggle detection.
// This bug occurred when statics differed between checked/unchecked states.
func TestFuzzCheckboxToggle_Property(t *testing.T) {
	// Shape with Complete boolean for checkbox state
	shape := &generators.StateShape{
		Fields: map[string]generators.FieldType{
			"Title": generators.FieldString,
		},
		Slices: map[string]generators.SliceShape{
			"Items": {
				MinLen: 1,
				MaxLen: 5,
				ItemShape: &generators.StateShape{
					Fields: map[string]generators.FieldType{
						"ID":       generators.FieldString,
						"Text":     generators.FieldString,
						"Complete": generators.FieldBool,
					},
				},
			},
		},
	}

	// Weights that favor toggling item fields
	weights := mutations.MutationWeights{
		SetField:     0.1,
		ToggleBool:   0.0, // Not applicable to items
		AppendSlice:  0.1,
		PrependSlice: 0.05,
		InsertSlice:  0.05,
		RemoveSlice:  0.1,
		ClearSlice:   0.0,
		ReorderSlice: 0.1,
		ReverseSlice: 0.05,
		UpdateItem:   0.45, // Heavy updates to toggle Complete
		KeyCollision: 0.0,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzCheckboxTemplate, shape, weights)
	})
}

// TestFuzzEmptyToItemsTransitions_Property tests empty→items transitions.
// This bug occurred when statics weren't properly included for first items.
func TestFuzzEmptyToItemsTransitions_Property(t *testing.T) {
	// Shape allowing empty start
	shape := &generators.StateShape{
		Fields: map[string]generators.FieldType{
			"Title": generators.FieldString,
			"Count": generators.FieldInt,
		},
		Slices: map[string]generators.SliceShape{
			"Items": {
				MinLen: 0, // Start empty
				MaxLen: 5,
				ItemShape: &generators.StateShape{
					Fields: map[string]generators.FieldType{
						"ID":   generators.FieldString,
						"Text": generators.FieldString,
					},
				},
			},
		},
	}

	// Weights that favor adding to empty slices
	weights := mutations.MutationWeights{
		SetField:     0.1,
		ToggleBool:   0.0,
		AppendSlice:  0.3, // Heavy append
		PrependSlice: 0.15,
		InsertSlice:  0.1,
		RemoveSlice:  0.1,
		ClearSlice:   0.1, // Clear to reset to empty
		ReorderSlice: 0.05,
		ReverseSlice: 0.05,
		UpdateItem:   0.05,
		KeyCollision: 0.0,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzEmptyRangeTemplate, shape, weights)
	})
}

// TestFuzzHeterogeneousRangeItems_Property tests ranges with per-item statics.
// This bug occurred when different items had different static structures.
func TestFuzzHeterogeneousRangeItems_Property(t *testing.T) {
	// Shape with multiple boolean flags for different item types
	shape := &generators.StateShape{
		Fields: map[string]generators.FieldType{
			"Title": generators.FieldString,
		},
		Slices: map[string]generators.SliceShape{
			"Items": {
				MinLen: 1,
				MaxLen: 5,
				ItemShape: &generators.StateShape{
					Fields: map[string]generators.FieldType{
						"ID":       generators.FieldString,
						"Text":     generators.FieldString,
						"URL":      generators.FieldString,
						"IsHeader": generators.FieldBool,
						"IsLink":   generators.FieldBool,
					},
				},
			},
		},
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSession(t, rng, seed, numMutations, fuzzHeterogeneousRangeTemplate, shape)
	})
}

// TestFuzzNestedConditionalRanges_Property tests nested conditionals in ranges.
func TestFuzzNestedConditionalRanges_Property(t *testing.T) {
	shape := &generators.StateShape{
		Fields: map[string]generators.FieldType{
			"ShowList": generators.FieldBool,
		},
		Slices: map[string]generators.SliceShape{
			"Items": {
				MinLen: 1,
				MaxLen: 5,
				ItemShape: &generators.StateShape{
					Fields: map[string]generators.FieldType{
						"ID":        generators.FieldString,
						"Text":      generators.FieldString,
						"Important": generators.FieldBool,
					},
				},
			},
		},
	}

	// Weights that favor toggling ShowList and Important
	weights := mutations.MutationWeights{
		SetField:     0.05,
		ToggleBool:   0.25, // Toggle ShowList
		AppendSlice:  0.1,
		PrependSlice: 0.05,
		InsertSlice:  0.05,
		RemoveSlice:  0.1,
		ClearSlice:   0.0,
		ReorderSlice: 0.1,
		ReverseSlice: 0.05,
		UpdateItem:   0.25, // Toggle Important on items
		KeyCollision: 0.0,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numMutations := rapid.IntRange(10, 30).Draw(rt, "numMutations")

		rng := rand.New(rand.NewSource(seed))
		runFuzzSessionWithWeights(t, rng, seed, numMutations, fuzzNestedConditionalRangeTemplate, shape, weights)
	})
}
