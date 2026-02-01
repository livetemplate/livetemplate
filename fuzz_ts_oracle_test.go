package livetemplate

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
	"github.com/livetemplate/livetemplate/internal/compat"
	"github.com/livetemplate/livetemplate/internal/fuzz/app"
	"github.com/livetemplate/livetemplate/internal/fuzz/invariants"
	"github.com/livetemplate/livetemplate/internal/fuzz/mutations"
	"github.com/livetemplate/livetemplate/internal/render"
	"pgregory.net/rapid"
)

var (
	globalTSOracle     *invariants.TypeScriptOracle
	globalTSOracleOnce sync.Once
	globalTSOracleErr  error
)

// getTSOracle returns the shared TypeScript oracle, initializing it if needed.
func getTSOracle(t *testing.T) *invariants.TypeScriptOracle {
	globalTSOracleOnce.Do(func() {
		clientDir, err := invariants.FindClientDir()
		if err != nil {
			globalTSOracleErr = err
			return
		}
		globalTSOracle, globalTSOracleErr = invariants.NewTypeScriptOracle(clientDir)
	})

	if globalTSOracleErr != nil {
		t.Skipf("TypeScript oracle not available: %v", globalTSOracleErr)
	}

	return globalTSOracle
}

// TestMain sets up and tears down global test resources.
func TestMain(m *testing.M) {
	code := m.Run()

	// Cleanup: close the TypeScript oracle if it was started
	if globalTSOracle != nil {
		globalTSOracle.Close()
	}

	os.Exit(code)
}

// runAsyncFuzzSessionWithTSOracle runs async fuzz tests using the TypeScript oracle.
func runAsyncFuzzSessionWithTSOracle(t *testing.T, rng *rand.Rand, seed int64, numCycles int, templateStr string, weights mutations.MutationWeights) {
	t.Helper()

	oracle := getTSOracle(t)

	tmpl := &Template{
		templateStr: templateStr,
		keyGen:      compat.NewKeyGenerator(),
	}

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Skipf("Template parse error: %v", err)
	}

	state := app.GenAsyncState(rng)

	verifier := invariants.NewVerifier(seed)

	initialTree, err := compat.ParseTemplateToTree("test", templateStr, state.ToMap(), tmpl.keyGen)
	if err != nil {
		t.Fatalf("Initial render failed: %v", err)
	}

	tmpl.lastTree = initialTree
	initialBuildTree := convertToBuildTree(initialTree)
	prevBuildTree := initialBuildTree
	oracleTreeMap := invariants.TreeToMap(initialBuildTree)

	if err := verifier.VerifyAll(nil, state.ToMap(), nil, initialBuildTree, initialBuildTree, true); err != nil {
		t.Fatalf("First render invariant violation: %v", err)
	}

	debugMode := true // Set to true to see diff details on error

	for cycle := 0; cycle < numCycles; cycle++ {
		oldStateMap := state.ToMap()

		mutation := app.GenAsyncMutation(rng, state, weights)
		if err := app.ApplyAsyncMutation(state, mutation); err != nil {
			continue
		}

		newTree, err := compat.ParseTemplateToTree("test", templateStr, state.ToMap(), tmpl.keyGen)
		if err != nil {
			t.Fatalf("Render failed at cycle %d: %v", cycle, err)
		}

		diffTree := tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
		newBuildTree := convertToBuildTree(newTree)
		diffBuildTree := convertToBuildTree(diffTree)

		// Apply diff using TypeScript oracle
		// Debug: inspect diffBuildTree before conversion
		if os.Getenv("DIFF_DEBUG") != "" {
			debugInspectDiffTree("diffBuildTree", diffBuildTree)
		}
		diffMap := invariants.TreeToMap(diffBuildTree)
		if os.Getenv("DIFF_DEBUG") != "" {
			diffJSON, _ := json.MarshalIndent(diffMap, "", "  ")
			fmt.Printf("[TEST_DEBUG] diffMap after TreeToMap:\n%s\n", string(diffJSON))
		}
		response, err := oracle.ApplyDiffRaw(oracleTreeMap, diffMap)
		if err != nil {
			t.Errorf("TypeScript oracle error at cycle %d (seed=%d, mutation=%s): %v", cycle, seed, mutation.Type, err)
			return
		}

		// Update oracle tree state for next iteration
		if response.Tree != nil {
			if newOracleMap, ok := response.Tree.(map[string]any); ok {
				oracleTreeMap = newOracleMap
			}
		}

		// Compare HTML output - this is what matters for correctness
		expectedMap := invariants.TreeToMap(newBuildTree)
		expectedHTML, err := render.TreeToHTML(expectedMap)
		if err != nil {
			t.Errorf("Failed to render expected tree at cycle %d: %v", cycle, err)
			return
		}

		if !htmlEquivalent(response.HTML, expectedHTML) {
			errMsg := "TypeScript oracle HTML diverged at cycle %d (seed=%d, mutation=%s)\n" +
				"  Oracle HTML:   %s\n" +
				"  Expected HTML: %s"
			if debugMode {
				diffJSON, _ := json.MarshalIndent(diffMap, "  ", "  ")
				oracleTreeJSON, _ := json.MarshalIndent(oracleTreeMap, "  ", "  ")
				errMsg += "\n  Diff sent:\n%s\n  Oracle tree state:\n%s"
				t.Errorf(errMsg, cycle, seed, mutation.Type,
					normalizeHTMLForTest(response.HTML), normalizeHTMLForTest(expectedHTML),
					string(diffJSON), string(oracleTreeJSON))
			} else {
				t.Errorf(errMsg, cycle, seed, mutation.Type,
					normalizeHTMLForTest(response.HTML), normalizeHTMLForTest(expectedHTML))
			}
			return
		}

		if err := verifier.VerifyAll(oldStateMap, state.ToMap(), prevBuildTree, newBuildTree, diffBuildTree, false); err != nil {
			t.Errorf("Invariant violation at cycle %d (seed=%d): %v", cycle, seed, err)
			return
		}

		tmpl.lastTree = newTree
		prevBuildTree = newBuildTree
	}
}

// runModalFuzzSessionWithTSOracle runs modal fuzz tests using the TypeScript oracle.
func runModalFuzzSessionWithTSOracle(t *testing.T, rng *rand.Rand, seed int64, numCycles int, templateStr string, weights mutations.MutationWeights) {
	t.Helper()

	oracle := getTSOracle(t)

	tmpl := &Template{
		templateStr: templateStr,
		keyGen:      compat.NewKeyGenerator(),
	}

	if _, err := tmpl.Parse(templateStr); err != nil {
		t.Skipf("Template parse error: %v", err)
	}

	state := app.GenModalState(rng)

	verifier := invariants.NewVerifier(seed)

	initialTree, err := compat.ParseTemplateToTree("test", templateStr, state.ToMap(), tmpl.keyGen)
	if err != nil {
		t.Fatalf("Initial render failed: %v", err)
	}

	tmpl.lastTree = initialTree
	initialBuildTree := convertToBuildTree(initialTree)
	prevBuildTree := initialBuildTree
	oracleTreeMap := invariants.TreeToMap(initialBuildTree)

	if err := verifier.VerifyAll(nil, state.ToMap(), nil, initialBuildTree, initialBuildTree, true); err != nil {
		t.Fatalf("First render invariant violation: %v", err)
	}

	for cycle := 0; cycle < numCycles; cycle++ {
		oldStateMap := state.ToMap()

		mutation := app.GenModalMutation(rng, state, weights)
		if err := app.ApplyModalMutation(state, mutation); err != nil {
			continue
		}

		newTree, err := compat.ParseTemplateToTree("test", templateStr, state.ToMap(), tmpl.keyGen)
		if err != nil {
			t.Fatalf("Render failed at cycle %d: %v", cycle, err)
		}

		diffTree := tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
		newBuildTree := convertToBuildTree(newTree)
		diffBuildTree := convertToBuildTree(diffTree)

		diffMap := invariants.TreeToMap(diffBuildTree)
		response, err := oracle.ApplyDiffRaw(oracleTreeMap, diffMap)
		if err != nil {
			t.Errorf("TypeScript oracle error at cycle %d (seed=%d, mutation=%s): %v", cycle, seed, mutation.Type, err)
			return
		}

		if response.Tree != nil {
			if newOracleMap, ok := response.Tree.(map[string]any); ok {
				oracleTreeMap = newOracleMap
			}
		}

		// Compare HTML output - this is what matters for correctness
		expectedMap := invariants.TreeToMap(newBuildTree)
		expectedHTML, err := render.TreeToHTML(expectedMap)
		if err != nil {
			t.Errorf("Failed to render expected tree at cycle %d: %v", cycle, err)
			return
		}

		if !htmlEquivalent(response.HTML, expectedHTML) {
			t.Errorf("TypeScript oracle HTML diverged at cycle %d (seed=%d, mutation=%s)\n"+
				"  Oracle HTML:   %s\n"+
				"  Expected HTML: %s",
				cycle, seed, mutation.Type, normalizeHTMLForTest(response.HTML), normalizeHTMLForTest(expectedHTML))
			return
		}

		if err := verifier.VerifyAll(oldStateMap, state.ToMap(), prevBuildTree, newBuildTree, diffBuildTree, false); err != nil {
			t.Errorf("Invariant violation at cycle %d (seed=%d): %v", cycle, seed, err)
			return
		}

		tmpl.lastTree = newTree
		prevBuildTree = newBuildTree
	}
}

// htmlEquivalent compares two HTML strings for semantic equivalence.
// It normalizes whitespace and handles minor formatting differences.
func htmlEquivalent(a, b string) bool {
	return normalizeHTMLForTest(a) == normalizeHTMLForTest(b)
}

// normalizeHTMLForTest normalizes HTML for comparison.
// This removes all whitespace between tags to handle rendering differences.
func normalizeHTMLForTest(html string) string {
	// Trim leading/trailing whitespace
	html = strings.TrimSpace(html)
	// Normalize newlines to spaces
	html = strings.ReplaceAll(html, "\r\n", " ")
	html = strings.ReplaceAll(html, "\n", " ")
	html = strings.ReplaceAll(html, "\t", " ")
	// Collapse multiple spaces to single space
	for strings.Contains(html, "  ") {
		html = strings.ReplaceAll(html, "  ", " ")
	}
	// Remove ALL whitespace between tags
	for strings.Contains(html, "> <") {
		html = strings.ReplaceAll(html, "> <", "><")
	}
	// Remove whitespace after opening tags
	for strings.Contains(html, "> ") {
		html = strings.ReplaceAll(html, "> ", ">")
	}
	// Remove whitespace before closing tags
	for strings.Contains(html, " <") {
		html = strings.ReplaceAll(html, " <", "<")
	}
	return html
}

// TestFuzzAsyncLoading_TSOracle tests async loading using TypeScript oracle.
// Phase 6 fix: The diff algorithm now correctly includes statics when conditional
// branches change within range items.
func TestFuzzAsyncLoading_TSOracle(t *testing.T) {

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numCycles := rapid.IntRange(15, 40).Draw(rt, "numCycles")

		rng := rand.New(rand.NewSource(seed))
		runAsyncFuzzSessionWithTSOracle(t, rng, seed, numCycles, app.AsyncTemplate, mutations.AsyncWeights)
	})
}

// TestFuzzAsyncItemStates_TSOracle tests per-item async states using TypeScript oracle.
func TestFuzzAsyncItemStates_TSOracle(t *testing.T) {

	weights := mutations.MutationWeights{
		StartItemLoad:  0.20,
		FinishItemLoad: 0.15,
		SetItemError:   0.15,
		ClearItemError: 0.10,
		AppendSlice:    0.15,
		RemoveSlice:    0.10,
		UpdateItem:     0.10,
		StartLoading:   0.05,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numCycles := rapid.IntRange(20, 50).Draw(rt, "numCycles")

		rng := rand.New(rand.NewSource(seed))
		runAsyncFuzzSessionWithTSOracle(t, rng, seed, numCycles, app.AsyncTemplate, weights)
	})
}

// TestFuzzAsyncOptimistic_TSOracle tests optimistic updates using TypeScript oracle.
func TestFuzzAsyncOptimistic_TSOracle(t *testing.T) {

	weights := mutations.MutationWeights{
		OptimisticUpdate: 0.25,
		RollbackUpdate:   0.15,
		StartItemLoad:    0.10,
		FinishItemLoad:   0.10,
		SetItemError:     0.10,
		AppendSlice:      0.15,
		UpdateItem:       0.10,
		StartLoading:     0.05,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numCycles := rapid.IntRange(20, 50).Draw(rt, "numCycles")

		rng := rand.New(rand.NewSource(seed))
		runAsyncFuzzSessionWithTSOracle(t, rng, seed, numCycles, app.AsyncTemplate, weights)
	})
}

// TestFuzzModalUpdateWhileOpen_TSOracle tests modal updates using TypeScript oracle.
// Fixed in Phase 6b: Added data-modal-id attribute outside conditional in ModalTemplate
// to ensure each modal has a unique key even when closed.
func TestFuzzModalUpdateWhileOpen_TSOracle(t *testing.T) {

	weights := mutations.MutationWeights{
		UpdateModal: 0.40,
		OpenModal:   0.15,
		CloseModal:  0.10,
		SwitchPanel: 0.15,
		TogglePanel: 0.10,
		AppendSlice: 0.05,
		CloseAll:    0.05,
	}

	rapid.Check(t, func(rt *rapid.T) {
		seed := rapid.Int64().Draw(rt, "seed")
		numCycles := rapid.IntRange(20, 50).Draw(rt, "numCycles")

		rng := rand.New(rand.NewSource(seed))
		runModalFuzzSessionWithTSOracle(t, rng, seed, numCycles, app.ModalTemplate, weights)
	})
}

// debugInspectDiffTree inspects a diff tree structure for debugging.
func debugInspectDiffTree(label string, tree *build.TreeNode) {
	fmt.Printf("[TEST_DEBUG] === %s ===\n", label)
	if tree == nil {
		fmt.Println("[TEST_DEBUG] tree is nil")
		return
	}
	fmt.Printf("[TEST_DEBUG] HasStatics: %v\n", tree.HasStatics())
	fmt.Printf("[TEST_DEBUG] HasDynamics: %v\n", tree.HasDynamics())
	fmt.Printf("[TEST_DEBUG] HasRange: %v\n", tree.HasRange())

	// Inspect dynamics
	for k, v := range tree.GetDynamics() {
		debugInspectValue(k, v, 1)
	}
}

// debugInspectValue recursively inspects a value.
func debugInspectValue(key string, value interface{}, depth int) {
	indent := strings.Repeat("  ", depth)
	switch val := value.(type) {
	case *build.TreeNode:
		fmt.Printf("[TEST_DEBUG] %skey=%s: *build.TreeNode hasStatics=%v statics=%v\n",
			indent, key, val.HasStatics(), val.Statics)
		for k, v := range val.GetDynamics() {
			debugInspectValue(k, v, depth+1)
		}
	case []interface{}:
		fmt.Printf("[TEST_DEBUG] %skey=%s: []interface{} len=%d\n", indent, key, len(val))
		for i, item := range val {
			debugInspectValue(fmt.Sprintf("[%d]", i), item, depth+1)
		}
	case map[string]interface{}:
		fmt.Printf("[TEST_DEBUG] %skey=%s: map[string]interface{}\n", indent, key)
		for k, v := range val {
			debugInspectValue(k, v, depth+1)
		}
	default:
		fmt.Printf("[TEST_DEBUG] %skey=%s: type=%T value=%v\n", indent, key, val, val)
	}
}
