package livetemplate

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"testing"

	"github.com/livetemplate/livetemplate/internal/compat"
)

// BenchmarkUserJourney has been moved to e2e_bench_test.go
// This benchmark suite now focuses on fingerprint benchmarks only
func BenchmarkUserJourney(b *testing.B) {
	generator := NewActivityGenerator(42)
	journey := generator.GenerateJourney(100) // 100 activities

	templateStr := `<div>
        {{.title}}
        {{range .items}}<li>{{.text}}</li>{{end}}
        Count: {{.count}}
    </div>`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		simulator := NewStateSimulator()
		tmpl := &Template{
			templateStr: templateStr,
			keyGen:      compat.NewKeyGenerator(),
		}
		_, _ = tmpl.Parse(tmpl.templateStr)

		for j, activity := range journey {
			simulator.ApplyActivity(activity)
			state := simulator.GetState()

			if j == 0 {
				_, _ = tmpl.generateInitialTreeWithoutRegistry(templateStr, state)
			} else {
				newTree, _ := compat.ParseTemplateToTree("test", templateStr, state, tmpl.keyGen)
				tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
				tmpl.lastTree = newTree
			}
		}
	}
}

// BenchmarkFingerprint_Small_Old measures the legacy fingerprint implementation on a small tree.
func BenchmarkFingerprint_Small_Old(b *testing.B) {
	tree := createFlatTree(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Small_New measures the new fingerprint implementation on a small tree.
func BenchmarkFingerprint_Small_New(b *testing.B) {
	tree := createFlatTree(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_Medium_Old measures the legacy fingerprint implementation on a medium tree.
func BenchmarkFingerprint_Medium_Old(b *testing.B) {
	tree := createFlatTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Medium_New measures the new fingerprint implementation on a medium tree.
func BenchmarkFingerprint_Medium_New(b *testing.B) {
	tree := createFlatTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_Large_Old measures the legacy fingerprint implementation on a large tree.
func BenchmarkFingerprint_Large_Old(b *testing.B) {
	tree := createFlatTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Large_New measures the new fingerprint implementation on a large tree.
func BenchmarkFingerprint_Large_New(b *testing.B) {
	tree := createFlatTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_DeepNested_Old measures the legacy fingerprint implementation on a deep tree.
func BenchmarkFingerprint_DeepNested_Old(b *testing.B) {
	tree := createNestedTree(4, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_DeepNested_New measures the new fingerprint implementation on a deep tree.
func BenchmarkFingerprint_DeepNested_New(b *testing.B) {
	tree := createNestedTree(4, 3)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_Range100_Old measures the legacy fingerprint implementation on a range tree.
func BenchmarkFingerprint_Range100_Old(b *testing.B) {
	tree := createRangeTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Range100_New measures the new fingerprint implementation on a range tree.
func BenchmarkFingerprint_Range100_New(b *testing.B) {
	tree := createRangeTree(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_Range1000_Old measures the legacy fingerprint implementation on a larger range tree.
func BenchmarkFingerprint_Range1000_Old(b *testing.B) {
	tree := createRangeTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Range1000_New measures the new fingerprint implementation on a larger range tree.
func BenchmarkFingerprint_Range1000_New(b *testing.B) {
	tree := createRangeTree(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// BenchmarkFingerprint_Allocations_Old reports allocations for the legacy fingerprint implementation.
func BenchmarkFingerprint_Allocations_Old(b *testing.B) {
	tree := createFlatTree(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calculateFingerprintOld(tree)
	}
}

// BenchmarkFingerprint_Allocations_New reports allocations for the new fingerprint implementation.
func BenchmarkFingerprint_Allocations_New(b *testing.B) {
	tree := createFlatTree(100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = compat.CalculateFingerprint(mustFromMap(tree))
	}
}

// calculateFingerprintOld replicates the original fingerprint logic for comparison benchmarks.
func calculateFingerprintOld(tree map[string]interface{}) string {
	hasher := md5.New()

	if statics, exists := tree["s"]; exists {
		if staticsArray, ok := statics.([]string); ok {
			staticsJSON, _ := json.Marshal(staticsArray)
			hasher.Write(staticsJSON)
		}
	}

	var keys []string
	for k := range tree {
		if k != "s" && k != "f" {
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

	for _, k := range keys {
		value := tree[k]
		valueJSON, _ := json.Marshal(value)
		hasher.Write([]byte(k))
		hasher.Write(valueJSON)
	}

	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) >= 16 {
		return fullHash[:16]
	}
	return fullHash
}

// createNestedTree constructs a tree with nested children to exercise recursion in benchmarks.
func createNestedTree(depth, breadth int) map[string]interface{} {
	if depth == 0 {
		return map[string]interface{}{
			"s": []string{"<span>", "</span>"},
			"0": "leaf value",
		}
	}

	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
	}

	for i := 0; i < breadth; i++ {
		tree[strconv.Itoa(i)] = createNestedTree(depth-1, breadth)
	}

	return tree
}

// createFlatTree constructs a tree with many sibling nodes.
func createFlatTree(nodes int) map[string]interface{} {
	tree := map[string]interface{}{
		"s": []string{"<div>", "</div>"},
	}

	for i := 0; i < nodes; i++ {
		tree[strconv.Itoa(i)] = map[string]interface{}{
			"s": []string{"<p>", "</p>"},
			"0": "content " + strconv.Itoa(i),
		}
	}

	return tree
}

// createRangeTree constructs a tree mimicking range dynamics.
func createRangeTree(items int) map[string]interface{} {
	var rangeItems []interface{}
	for i := 0; i < items; i++ {
		rangeItems = append(rangeItems, map[string]interface{}{
			"1": "item-" + strconv.Itoa(i),
			"3": "Item description " + strconv.Itoa(i),
			"5": map[string]interface{}{
				"0": "Priority " + strconv.Itoa(i%3),
			},
		})
	}

	return map[string]interface{}{
		"s": []string{"<div>", "</div>"},
		"0": map[string]interface{}{
			"s": []string{"<ul>", "</ul>"},
			"d": rangeItems,
		},
	}
}
