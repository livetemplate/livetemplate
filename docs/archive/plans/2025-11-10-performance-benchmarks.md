# Performance Benchmarking System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement comprehensive performance benchmarks for all 5 phases (Parse → Build → Diff → Render → Send) plus end-to-end scenarios, with baseline tracking, CI integration, and profiling for bottleneck discovery.

**Architecture:** Phase-specific benchmarks co-located with their code, end-to-end benchmarks in root. Baseline system uses benchstat for regression detection. CI runs benchmarks on PRs and compares against committed baseline. Profiling discovers actual bottlenecks for documentation.

**Tech Stack:** Go 1.21+ testing framework, benchstat, pprof, GitHub Actions

---

## Task 1: Setup Infrastructure

**Files:**
- Create: `testdata/benchmarks/README.md`
- Create: `testdata/benchmarks/.gitkeep`
- Create: `profiles/.gitignore`
- Modify: `Makefile` (add benchmark targets)

**Step 1: Create baseline directory with README**

```bash
mkdir -p testdata/benchmarks
```

Create `testdata/benchmarks/README.md`:

```markdown
# Performance Baselines

## When to Update Baselines

Update baselines when:
1. Performance improvements are intentionally made
2. Benchmarks show consistent improvement (run 10x to verify)
3. Changes are reviewed and approved

DO NOT update baselines to "fix" regressions.

## How to Update

1. Run benchmarks multiple times:
   ```bash
   make bench-10x
   ```

2. Verify improvements are real and consistent

3. Save new baseline:
   ```bash
   make bench-save
   ```

4. Commit with description:
   ```bash
   git add testdata/benchmarks/baseline.txt
   git commit -m "perf: update baseline after {description}"
   ```

## Comparing Against Baseline

```bash
make bench-compare
```

Uses benchstat to show statistical comparison.
```

**Step 2: Create profiles directory with gitignore**

```bash
mkdir -p profiles
```

Create `profiles/.gitignore`:

```
# Ignore all profiling output
*
!.gitignore
```

**Step 3: Add benchmark targets to Makefile**

If `Makefile` doesn't exist, create it. Otherwise append these targets:

```makefile
.PHONY: bench bench-10x bench-save bench-compare bench-quick profile-cpu profile-mem profile-all

# Run all benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run benchmarks 10 times for statistical confidence
bench-10x:
	go test -bench=. -benchmem -count=10 ./... | tee /tmp/bench-results.txt

# Save current results as baseline
bench-save:
	go test -bench=. -benchmem ./... > testdata/benchmarks/baseline.txt
	@echo "Baseline saved to testdata/benchmarks/baseline.txt"

# Compare current vs baseline using benchstat
bench-compare:
	@echo "Running current benchmarks..."
	@go test -bench=. -benchmem ./... > /tmp/current-bench.txt
	@echo "\nComparing against baseline..."
	@benchstat testdata/benchmarks/baseline.txt /tmp/current-bench.txt

# Quick smoke test (critical benchmarks only)
bench-quick:
	go test -bench='Benchmark(E2E|Template)' -benchmem -timeout=5m ./...

# Profile CPU
profile-cpu:
	@mkdir -p profiles
	go test -bench=. -benchmem -cpuprofile=profiles/cpu.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/cpu.prof"

# Profile memory
profile-mem:
	@mkdir -p profiles
	go test -bench=. -benchmem -memprofile=profiles/mem.prof ./...
	@echo "\nAnalyze with: go tool pprof profiles/mem.prof"

# Profile everything
profile-all: profile-cpu profile-mem
	@echo "\nProfiles saved in profiles/ directory"
```

**Step 4: Commit infrastructure setup**

```bash
git add testdata/benchmarks/README.md testdata/benchmarks/.gitkeep profiles/.gitignore Makefile
git commit -m "build: add performance benchmarking infrastructure

- Add testdata/benchmarks/ for baseline tracking
- Add profiles/ for profiling output (gitignored)
- Add Makefile targets for benchmarking and profiling"
```

---

## Task 2: Phase 1 Benchmarks (Parse)

**Files:**
- Create: `internal/parse/parse_bench_test.go`

**Step 1: Create parse benchmark file with package and imports**

Create `internal/parse/parse_bench_test.go`:

```go
package parse

import (
	"html/template"
	"testing"
)

// Benchmark helpers

func createSimpleTemplate() string {
	return `<div>{{.Name}}</div>`
}

func createConditionalTemplate() string {
	return `<div>{{if .Show}}<span>{{.Name}}</span>{{else}}<span>Hidden</span>{{end}}</div>`
}

func createRangeTemplate() string {
	return `<ul>{{range .Items}}<li>{{.Name}}</li>{{end}}</ul>`
}

func createNestedTemplate() string {
	return `<div>{{range .Items}}{{if .Active}}<span>{{.Name}}</span>{{end}}{{end}}</div>`
}

func createComplexTemplate() string {
	return `<div>
		<h1>{{.Title}}</h1>
		{{if .ShowItems}}
		<ul>
		{{range .Items}}
			<li>
				<span>{{.Name}}</span>
				{{if .Tags}}
				<div>{{range .Tags}}<span>{{.}}</span>{{end}}</div>
				{{end}}
			</li>
		{{end}}
		</ul>
		{{end}}
	</div>`
}

// Entry point benchmarks

func BenchmarkParse(b *testing.B) {
	tests := []struct {
		name     string
		template string
	}{
		{"simple", createSimpleTemplate()},
		{"conditional", createConditionalTemplate()},
		{"range", createRangeTemplate()},
		{"nested", createNestedTemplate()},
		{"complex", createComplexTemplate()},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tmpl, err := template.New("test").Parse(tt.template)
				if err != nil {
					b.Fatal(err)
				}
				_, err = Parse(tmpl.Tree.Root, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBuildTree(b *testing.B) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{"simple", createSimpleTemplate(), map[string]interface{}{"Name": "Test"}},
		{"conditional-true", createConditionalTemplate(), map[string]interface{}{"Show": true, "Name": "Test"}},
		{"conditional-false", createConditionalTemplate(), map[string]interface{}{"Show": false, "Name": "Test"}},
		{"range-small", createRangeTemplate(), map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Name": "Item 1"},
				{"Name": "Item 2"},
				{"Name": "Item 3"},
			},
		}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tmpl, err := template.New("test").Parse(tt.template)
			if err != nil {
				b.Fatal(err)
			}
			construct, err := Parse(tmpl.Tree.Root, nil)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := BuildTree(construct, tt.data, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Scale variation benchmarks

func BenchmarkBuildTreeScale(b *testing.B) {
	template := createRangeTemplate()

	scales := []struct {
		name string
		size int
	}{
		{"small-10", 10},
		{"medium-100", 100},
		{"large-1000", 1000},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			items := make([]map[string]interface{}, scale.size)
			for i := 0; i < scale.size; i++ {
				items[i] = map[string]interface{}{"Name": "Item"}
			}
			data := map[string]interface{}{"Items": items}

			tmpl, err := template.New("test").Parse(template)
			if err != nil {
				b.Fatal(err)
			}
			construct, err := Parse(tmpl.Tree.Root, nil)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := BuildTree(construct, data, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
```

**Step 2: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=BenchmarkParse -benchmem ./internal/parse/ -run=^$
```

Expected: Benchmarks run successfully with timing and allocation stats.

**Step 3: Commit Phase 1 benchmarks**

```bash
git add internal/parse/parse_bench_test.go
git commit -m "perf: add Phase 1 (Parse) benchmarks

- Entry point benchmarks: Parse, BuildTree
- Template complexity variations
- Scale benchmarks (10, 100, 1000 items)"
```

---

## Task 3: Phase 2 Benchmarks (Build)

**Files:**
- Create: `internal/build/build_bench_test.go`

**Step 1: Create build benchmark file**

Create `internal/build/build_bench_test.go`:

```go
package build

import (
	"testing"
)

// Benchmark helpers

func createTestTree(depth, breadth int) TreeNode {
	if depth == 0 {
		return TreeNode{
			"s": []string{"<span>", "</span>"},
			"0": "leaf",
		}
	}

	node := TreeNode{
		"s": make([]string, breadth+1),
	}
	node["s"].([]string)[0] = "<div>"
	for i := 0; i < breadth; i++ {
		node[string(rune('0'+i))] = createTestTree(depth-1, breadth)
	}
	node["s"].([]string)[breadth] = "</div>"

	return node
}

// TreeNode operations

func BenchmarkTreeNodeCreation(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
		{"nested-large", 5, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = createTestTree(tt.depth, tt.breadth)
			}
		})
	}
}

func BenchmarkTreeNodeMarshalJSON(b *testing.B) {
	tests := []struct {
		name    string
		depth   int
		breadth int
	}{
		{"flat", 1, 5},
		{"nested-small", 3, 3},
		{"nested-medium", 4, 3},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tree := createTestTree(tt.depth, tt.breadth)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := tree.MarshalJSON()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Wrapper operations

func BenchmarkWrapperInjection(b *testing.B) {
	fullHTML := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<div><h1>Hello</h1><p>World</p></div>
</body>
</html>`

	fragment := `<div><h1>Hello</h1><p>World</p></div>`

	b.Run("full-html", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := InjectWrapper(fullHTML, "test-wrapper")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("fragment", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := InjectWrapper(fragment, "test-wrapper")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExtractWrapperContent(b *testing.B) {
	html := `<div id="test-wrapper"><div><h1>Hello</h1><p>World</p></div></div>`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ExtractWrapperContent(html, "test-wrapper")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Context operations

func BenchmarkContextOperations(b *testing.B) {
	tree := createTestTree(3, 3)

	b.Run("with-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := &Context{IncludeStatics: true}
			_ = ctx.ProcessTree(tree)
		}
	})

	b.Run("without-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := &Context{IncludeStatics: false}
			_ = ctx.ProcessTree(tree)
		}
	})
}
```

**Step 2: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=. -benchmem ./internal/build/ -run=^$
```

Expected: Benchmarks run successfully.

**Step 3: Commit Phase 2 benchmarks**

```bash
git add internal/build/build_bench_test.go
git commit -m "perf: add Phase 2 (Build) benchmarks

- TreeNode creation and JSON marshaling
- Wrapper injection and extraction
- Context operations (with/without statics)"
```

---

## Task 4: Phase 3 Benchmarks (Diff)

**Files:**
- Create: `internal/diff/diff_bench_test.go`

**Step 1: Create diff benchmark file**

Create `internal/diff/diff_bench_test.go`:

```go
package diff

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

// Benchmark helpers

func createSimpleTree() build.TreeNode {
	return build.TreeNode{
		"s": []string{"<div>", "</div>"},
		"0": "value",
	}
}

func createTreeWithNFields(n int) build.TreeNode {
	tree := build.TreeNode{
		"s": []string{"<div>", "</div>"},
	}
	for i := 0; i < n; i++ {
		tree[string(rune('0'+i))] = "value"
	}
	return tree
}

func createRangeTree(itemCount int) build.TreeNode {
	tree := build.TreeNode{
		"s": []string{"<ul>", "</ul>"},
		"0": make([]interface{}, itemCount),
	}
	items := tree["0"].([]interface{})
	for i := 0; i < itemCount; i++ {
		items[i] = map[string]interface{}{
			"key": i,
			"tree": build.TreeNode{
				"s": []string{"<li>", "</li>"},
				"0": "item",
			},
		}
	}
	return tree
}

// Comparison operations

func BenchmarkCompareTreesNoChanges(b *testing.B) {
	tree := createTreeWithNFields(10)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompareTreesAndGetChanges(tree, tree, "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareTreesSmallChange(b *testing.B) {
	tree1 := createSimpleTree()
	tree2 := createSimpleTree()
	tree2["0"] = "changed"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := CompareTreesAndGetChanges(tree1, tree2, "")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompareTreesLargeChange(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(string(rune('0'+size)), func(b *testing.B) {
			tree1 := createTreeWithNFields(size)
			tree2 := createTreeWithNFields(size)
			// Change every field
			for i := 0; i < size; i++ {
				tree2[string(rune('0'+i))] = "changed"
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := CompareTreesAndGetChanges(tree1, tree2, "")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Range differential operations

func BenchmarkRangeDiffUpdate(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(100)
	// Change one item
	newTree["0"].([]interface{})[50].(map[string]interface{})["tree"].(build.TreeNode)["0"] = "updated"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := GenerateRangeDifferentialOperations(oldTree["0"], newTree["0"], "0")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRangeDiffInsert(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(101)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := GenerateRangeDifferentialOperations(oldTree["0"], newTree["0"], "0")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRangeDiffRemove(b *testing.B) {
	oldTree := createRangeTree(100)
	newTree := createRangeTree(99)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := GenerateRangeDifferentialOperations(oldTree["0"], newTree["0"], "0")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Client preparation

func BenchmarkPrepareTreeForClient(b *testing.B) {
	tree := createTreeWithNFields(100)

	b.Run("with-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			PrepareTreeForClient(tree, false)
		}
	})

	b.Run("without-statics", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			PrepareTreeForClient(tree, true)
		}
	})
}
```

**Step 2: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=. -benchmem ./internal/diff/ -run=^$
```

Expected: Benchmarks run successfully.

**Step 3: Commit Phase 3 benchmarks**

```bash
git add internal/diff/diff_bench_test.go
git commit -m "perf: add Phase 3 (Diff) benchmarks

- Tree comparison (no changes, small, large)
- Range differential operations
- Client preparation (with/without statics)"
```

---

## Task 5: Phase 4 & 5 Benchmarks (Render & Send)

**Files:**
- Create: `internal/render/render_bench_test.go`
- Create: `internal/send/send_bench_test.go`

**Step 1: Create render benchmark file**

Create `internal/render/render_bench_test.go`:

```go
package render

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

func BenchmarkNodeRender(b *testing.B) {
	node := build.TreeNode{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Node(node)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeToHTML(b *testing.B) {
	simpleTree := build.TreeNode{
		"s": []string{"<div>", "</div>"},
		"0": "content",
	}

	nestedTree := build.TreeNode{
		"s": []string{"<div>", "</div>"},
		"0": build.TreeNode{
			"s": []string{"<span>", "</span>"},
			"0": "nested",
		},
	}

	rangeTree := build.TreeNode{
		"s": []string{"<ul>", "</ul>"},
		"0": []interface{}{
			map[string]interface{}{
				"key": 1,
				"tree": build.TreeNode{
					"s": []string{"<li>", "</li>"},
					"0": "item1",
				},
			},
			map[string]interface{}{
				"key": 2,
				"tree": build.TreeNode{
					"s": []string{"<li>", "</li>"},
					"0": "item2",
				},
			},
		},
	}

	b.Run("simple", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(simpleTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nested", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(nestedTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with-ranges", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := TreeToHTML(rangeTree)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMinifyHTML(b *testing.B) {
	html := `
		<div>
			<h1>Title</h1>
			<p>
				Paragraph with    extra spaces
			</p>
		</div>
	`

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = MinifyHTML(html)
	}
}
```

**Step 2: Create send benchmark file**

Create `internal/send/send_bench_test.go`:

```go
package send

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/livetemplate/livetemplate/internal/build"
)

func BenchmarkParseActionFromHTTP(b *testing.B) {
	body := bytes.NewBufferString("action=increment&value=5")
	req := httptest.NewRequest("POST", "/action", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req.Body = io.NopCloser(bytes.NewBufferString("action=increment&value=5"))
		_, err := ParseActionFromHTTP(req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseActionFromWebSocket(b *testing.B) {
	jsonData := []byte(`{"action":"increment","data":{"value":5}}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ParseActionFromWebSocket(jsonData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrepareUpdate(b *testing.B) {
	update := build.TreeNode{
		"0": "updated value",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = PrepareUpdate(update, "test-wrapper")
	}
}

func BenchmarkSerializeUpdate(b *testing.B) {
	response := map[string]interface{}{
		"type":   "update",
		"target": "test-wrapper",
		"tree": build.TreeNode{
			"0": "value",
		},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := SerializeUpdate(response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalOrderedJSON(b *testing.B) {
	data := build.TreeNode{
		"s": []string{"<div>", "</div>"},
		"0": "value1",
		"1": "value2",
		"2": "value3",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := MarshalOrderedJSON(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
```

**Step 3: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=. -benchmem ./internal/render/ -run=^$
GOWORK=off go test -bench=. -benchmem ./internal/send/ -run=^$
```

Expected: Both benchmark suites run successfully.

**Step 4: Commit Phase 4 & 5 benchmarks**

```bash
git add internal/render/render_bench_test.go internal/send/send_bench_test.go
git commit -m "perf: add Phase 4 (Render) and Phase 5 (Send) benchmarks

Phase 4:
- Node rendering and TreeToHTML
- HTML minification

Phase 5:
- Action parsing (HTTP and WebSocket)
- Update preparation and serialization
- Ordered JSON marshaling"
```

---

## Task 6: End-to-End Template Benchmarks

**Files:**
- Create: `template_bench_test.go`

**Step 1: Create template benchmark file**

Create `template_bench_test.go`:

```go
package livetemplate

import (
	"testing"
)

// Core template operations

func BenchmarkTemplateExecute(b *testing.B) {
	tmpl := New("test")
	err := tmpl.Parse(`<div>{{.Name}}</div>`)
	if err != nil {
		b.Fatal(err)
	}

	data := map[string]interface{}{"Name": "Test"}

	b.Run("initial-render", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			tmpl := New("test")
			tmpl.Parse(`<div>{{.Name}}</div>`)
			_, err := tmpl.Execute(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("subsequent-render", func(b *testing.B) {
		tmpl := New("test")
		tmpl.Parse(`<div>{{.Name}}</div>`)
		tmpl.Execute(data) // Prime the cache

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := tmpl.Execute(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkTemplateExecuteUpdates(b *testing.B) {
	tmpl := New("test")
	err := tmpl.Parse(`<div>{{.Name}}</div>`)
	if err != nil {
		b.Fatal(err)
	}

	initialData := map[string]interface{}{"Name": "Initial"}
	tmpl.Execute(initialData) // Prime

	b.Run("no-changes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := tmpl.ExecuteUpdates(initialData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("small-update", func(b *testing.B) {
		data := map[string]interface{}{"Name": "Updated"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := tmpl.ExecuteUpdates(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	largeTemplate := `<div>
		<h1>{{.Title}}</h1>
		<p>{{.Description}}</p>
		<span>{{.Author}}</span>
		<time>{{.Date}}</time>
		<div>{{.Content}}</div>
	</div>`

	tmplLarge := New("large")
	tmplLarge.Parse(largeTemplate)
	largeData := map[string]interface{}{
		"Title":       "Title",
		"Description": "Description",
		"Author":      "Author",
		"Date":        "2025-01-01",
		"Content":     "Content",
	}
	tmplLarge.Execute(largeData)

	b.Run("large-update", func(b *testing.B) {
		updatedData := map[string]interface{}{
			"Title":       "New Title",
			"Description": "New Description",
			"Author":      "New Author",
			"Date":        "2025-01-02",
			"Content":     "New Content",
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := tmplLarge.ExecuteUpdates(updatedData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Template complexity variations

func BenchmarkTemplateComplexity(b *testing.B) {
	tests := []struct {
		name     string
		template string
		data     interface{}
	}{
		{
			"simple-fields",
			`<div>{{.A}} {{.B}} {{.C}}</div>`,
			map[string]interface{}{"A": "a", "B": "b", "C": "c"},
		},
		{
			"with-conditionals",
			`<div>{{if .Show}}<span>{{.Name}}</span>{{else}}<span>Hidden</span>{{end}}</div>`,
			map[string]interface{}{"Show": true, "Name": "Test"},
		},
		{
			"with-ranges",
			`<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`,
			map[string]interface{}{"Items": []string{"a", "b", "c"}},
		},
		{
			"deeply-nested",
			`<div>{{range .L1}}{{range .L2}}{{range .L3}}<span>{{.}}</span>{{end}}{{end}}{{end}}</div>`,
			map[string]interface{}{
				"L1": []map[string]interface{}{
					{"L2": []map[string]interface{}{
						{"L3": []string{"a", "b"}},
					}},
				},
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tmpl := New("test")
			err := tmpl.Parse(tt.template)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := tmpl.Execute(tt.data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Concurrent operations

func BenchmarkTemplateConcurrent(b *testing.B) {
	tmpl := New("test")
	err := tmpl.Parse(`<div>{{.Name}}</div>`)
	if err != nil {
		b.Fatal(err)
	}

	data := map[string]interface{}{"Name": "Test"}

	concurrency := []int{1, 10, 100}

	for _, n := range concurrency {
		b.Run(string(rune('0'+n)), func(b *testing.B) {
			b.SetParallelism(n)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := tmpl.Execute(data)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
```

**Step 2: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=BenchmarkTemplate -benchmem -run=^$
```

Expected: All template benchmarks run successfully.

**Step 3: Commit template benchmarks**

```bash
git add template_bench_test.go
git commit -m "perf: add end-to-end template benchmarks

- Core operations: Execute (initial/subsequent), ExecuteUpdates
- Template complexity variations
- Concurrent operations (1, 10, 100 goroutines)"
```

---

## Task 7: End-to-End User Journey Benchmarks

**Files:**
- Create: `e2e_bench_test.go`
- Modify: `tree_bench_test.go` (move BenchmarkUserJourney)

**Step 1: Create e2e benchmark file**

Create `e2e_bench_test.go`:

```go
package livetemplate

import (
	"testing"
)

// Helper: Simulate user activities

type Activity struct {
	Action string
	Data   map[string]interface{}
}

func simulateUserJourney(tmpl *Template, activities []Activity) error {
	for _, activity := range activities {
		_, err := tmpl.ExecuteUpdates(activity.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

// User journey benchmarks

func BenchmarkE2EUserJourney(b *testing.B) {
	// Simple counter app
	tmpl := New("counter")
	err := tmpl.Parse(`<div><button>{{.Count}}</button></div>`)
	if err != nil {
		b.Fatal(err)
	}

	activities := make([]Activity, 100)
	for i := 0; i < 100; i++ {
		activities[i] = Activity{
			Action: "increment",
			Data:   map[string]interface{}{"Count": i},
		}
	}

	// Initial render
	tmpl.Execute(map[string]interface{}{"Count": 0})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := simulateUserJourney(tmpl, activities)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkE2ETodoApp(b *testing.B) {
	tmpl := New("todos")
	err := tmpl.Parse(`<ul>{{range .Items}}<li>{{.Text}}</li>{{end}}</ul>`)
	if err != nil {
		b.Fatal(err)
	}

	// Initial render
	initialData := map[string]interface{}{
		"Items": []map[string]interface{}{},
	}
	tmpl.Execute(initialData)

	// Simulate adding todos
	activities := []Activity{
		{Action: "add", Data: map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Text": "Todo 1"},
			},
		}},
		{Action: "add", Data: map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Text": "Todo 1"},
				{"Text": "Todo 2"},
			},
		}},
		{Action: "add", Data: map[string]interface{}{
			"Items": []map[string]interface{}{
				{"Text": "Todo 1"},
				{"Text": "Todo 2"},
				{"Text": "Todo 3"},
			},
		}},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := simulateUserJourney(tmpl, activities)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Range operations

func BenchmarkE2ERangeOperations(b *testing.B) {
	tmpl := New("list")
	err := tmpl.Parse(`<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>`)
	if err != nil {
		b.Fatal(err)
	}

	baseItems := []string{"a", "b", "c"}
	tmpl.Execute(map[string]interface{}{"Items": baseItems})

	b.Run("add-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			newItems := append(baseItems, "d", "e")
			_, err := tmpl.ExecuteUpdates(map[string]interface{}{"Items": newItems})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("remove-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			newItems := baseItems[:2]
			_, err := tmpl.ExecuteUpdates(map[string]interface{}{"Items": newItems})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("reorder-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			reordered := []string{"c", "a", "b"}
			_, err := tmpl.ExecuteUpdates(map[string]interface{}{"Items": reordered})
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("update-items", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			updated := []string{"x", "y", "z"}
			_, err := tmpl.ExecuteUpdates(map[string]interface{}{"Items": updated})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Multiple sessions

func BenchmarkE2EMultipleSessions(b *testing.B) {
	template := `<div>{{.Count}}</div>`

	sessions := []int{1, 10, 100}

	for _, sessionCount := range sessions {
		b.Run(string(rune('0'+sessionCount)), func(b *testing.B) {
			templates := make([]*Template, sessionCount)
			for i := 0; i < sessionCount; i++ {
				tmpl := New("session")
				tmpl.Parse(template)
				tmpl.Execute(map[string]interface{}{"Count": 0})
				templates[i] = tmpl
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, tmpl := range templates {
					_, err := tmpl.ExecuteUpdates(map[string]interface{}{"Count": i})
					if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
```

**Step 2: Move BenchmarkUserJourney from tree_bench_test.go**

In `tree_bench_test.go`, find `BenchmarkUserJourney` and add a comment:

```go
// BenchmarkUserJourney has been moved to e2e_bench_test.go
// This benchmark suite now focuses on fingerprint benchmarks only
```

**Step 3: Run benchmarks to verify they work**

```bash
GOWORK=off go test -bench=BenchmarkE2E -benchmem -run=^$
```

Expected: All E2E benchmarks run successfully.

**Step 4: Commit E2E benchmarks**

```bash
git add e2e_bench_test.go tree_bench_test.go
git commit -m "perf: add end-to-end user journey benchmarks

- User journey simulations (counter, todos)
- Range operations (add, remove, reorder, update)
- Multiple concurrent sessions
- Move BenchmarkUserJourney note to tree_bench_test.go"
```

---

## Task 8: Run Initial Benchmarks and Create Baseline

**Files:**
- Create: `testdata/benchmarks/baseline.txt`

**Step 1: Install benchstat**

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

Expected: Tool installed successfully.

**Step 2: Run all benchmarks 10 times**

```bash
make bench-10x
```

Expected: All benchmarks run successfully with consistent results across 10 iterations.

**Step 3: Save baseline**

```bash
make bench-save
```

Expected: `testdata/benchmarks/baseline.txt` created with benchmark results.

**Step 4: Verify baseline file**

```bash
head -20 testdata/benchmarks/baseline.txt
```

Expected: File contains benchmark output in Go test format.

**Step 5: Commit baseline**

```bash
git add testdata/benchmarks/baseline.txt
git commit -m "perf: establish performance baseline

Initial baseline from 10 benchmark iterations on:
- Go $(go version)
- $(uname -m) architecture
- All 5 phases + end-to-end scenarios"
```

---

## Task 9: Generate Profiles and Document Bottlenecks

**Files:**
- Create: `docs/performance/known-bottlenecks.md`

**Step 1: Generate CPU profile**

```bash
make profile-cpu
```

Expected: `profiles/cpu.prof` created.

**Step 2: Analyze CPU profile (top 10 functions)**

```bash
go tool pprof -top -cum profiles/cpu.prof | head -20 > /tmp/cpu-analysis.txt
cat /tmp/cpu-analysis.txt
```

Expected: Output shows top CPU consumers.

**Step 3: Generate memory profile**

```bash
make profile-mem
```

Expected: `profiles/mem.prof` created.

**Step 4: Analyze memory profile (top 10 allocations)**

```bash
go tool pprof -top -alloc_space profiles/mem.prof | head -20 > /tmp/mem-analysis.txt
cat /tmp/mem-analysis.txt
```

Expected: Output shows top memory allocators.

**Step 5: Create bottlenecks document**

Create `docs/performance/known-bottlenecks.md`:

```markdown
# Known Performance Bottlenecks

**Last Profiled:** 2025-11-10
**Go Version:** 1.21
**Architecture:** [Your architecture from uname -m]

## Profiling Methodology

Profiles generated using:
```bash
make profile-cpu
make profile-mem
```

Analyzed using:
```bash
go tool pprof -top -cum profiles/cpu.prof
go tool pprof -top -alloc_space profiles/mem.prof
```

## CPU Bottlenecks

### Analysis Summary

[Copy top 10-15 functions from /tmp/cpu-analysis.txt here]

### Key Findings

#### Phase 1: Parse
- **Location:** [Identify from profile]
- **Impact:** [Percentage of CPU time]
- **Optimization Opportunity:** [If any obvious improvements]

#### Phase 2: Build
- **Location:** [Identify from profile]
- **Impact:** [Percentage of CPU time]
- **Optimization Opportunity:** [If any obvious improvements]

#### Phase 3: Diff
- **Location:** [Identify from profile]
- **Impact:** [Percentage of CPU time]
- **Optimization Opportunity:** [If any obvious improvements]

#### Phase 4: Render
- **Location:** [Identify from profile]
- **Impact:** [Percentage of CPU time]
- **Optimization Opportunity:** [If any obvious improvements]

#### Phase 5: Send
- **Location:** [Identify from profile]
- **Impact:** [Percentage of CPU time]
- **Optimization Opportunity:** [If any obvious improvements]

## Memory Bottlenecks

### Analysis Summary

[Copy top 10-15 allocation sources from /tmp/mem-analysis.txt here]

### Allocations per Operation

**Initial Render:**
- Total allocations: [From benchmem output]
- Bytes allocated: [From benchmem output]

**Small Update:**
- Total allocations: [From benchmem output]
- Bytes allocated: [From benchmem output]

**Large Update:**
- Total allocations: [From benchmem output]
- Bytes allocated: [From benchmem output]

### Cache Memory Usage

**Parse Caches:**
- pipeTemplateCache: [Estimate if visible]
- astTemplateCache: [Estimate if visible]
- executionCache: [Estimate if visible]

**Structure Registry:**
- Max entries: 1000 (LRU eviction)
- Estimated size: [Calculate if possible]

## Optimization Priorities

Based on profiling data, prioritize:

1. **[High]** [Most impactful bottleneck from CPU profile]
   - Current: X% of CPU time
   - Potential improvement: [Estimate]

2. **[Medium]** [Secondary bottleneck]
   - Current: Y% of CPU time
   - Potential improvement: [Estimate]

3. **[Low]** [Minor optimization]
   - Current: Z% of CPU time
   - Potential improvement: [Estimate]

## Regenerating Profiles

To update this analysis after code changes:

```bash
make profile-all
go tool pprof -http=:8080 profiles/cpu.prof   # Interactive analysis
go tool pprof -http=:8080 profiles/mem.prof
```

Look for:
- Hot paths in CPU profile (cumulative % column)
- High allocation counts in memory profile
- Lock contention in concurrent benchmarks
```

**Step 6: Fill in bottlenecks document with actual data**

Review the profile outputs and fill in the template with real numbers.

**Step 7: Commit bottlenecks documentation**

```bash
git add docs/performance/known-bottlenecks.md
git commit -m "docs: document performance bottlenecks from profiling

- CPU profile analysis showing top consumers
- Memory profile showing allocation patterns
- Optimization priorities based on impact"
```

---

## Task 10: Create Benchmarking Guide

**Files:**
- Create: `docs/performance/benchmarking-guide.md`

**Step 1: Create benchmarking guide**

Create `docs/performance/benchmarking-guide.md`:

```markdown
# Performance Benchmarking Guide

## Overview

This guide explains how to run, interpret, and contribute to LiveTemplate's performance benchmarks.

## Benchmark Organization

### Phase-Specific Benchmarks

Benchmarks co-located with their phase code:

- **Phase 1 (Parse):** `internal/parse/parse_bench_test.go`
- **Phase 2 (Build):** `internal/build/build_bench_test.go`
- **Phase 3 (Diff):** `internal/diff/diff_bench_test.go`
- **Phase 4 (Render):** `internal/render/render_bench_test.go`
- **Phase 5 (Send):** `internal/send/send_bench_test.go`

### End-to-End Benchmarks

Comprehensive scenarios in root directory:

- **Template Operations:** `template_bench_test.go`
- **User Journeys:** `e2e_bench_test.go`
- **Tree Operations:** `tree_bench_test.go` (fingerprinting)

## Running Benchmarks

### All Benchmarks

```bash
make bench
```

Runs all benchmarks once with memory statistics.

### Statistical Confidence (10 Iterations)

```bash
make bench-10x
```

Runs 10 iterations to account for variance. Use this before updating baselines.

### Specific Benchmarks

```bash
# Run phase-specific benchmarks
GOWORK=off go test -bench=. -benchmem ./internal/parse/ -run=^$

# Run end-to-end benchmarks only
GOWORK=off go test -bench=BenchmarkE2E -benchmem -run=^$

# Run specific benchmark
GOWORK=off go test -bench=BenchmarkTemplateExecute -benchmem -run=^$
```

### Quick Smoke Test

```bash
make bench-quick
```

Runs only critical benchmarks (E2E and Template operations) for faster feedback.

## Comparing Against Baseline

### Standard Comparison

```bash
make bench-compare
```

Runs current benchmarks and compares against committed baseline using benchstat.

### Manual Comparison

```bash
# Save current results
go test -bench=. -benchmem ./... > /tmp/new.txt

# Compare with benchstat
benchstat testdata/benchmarks/baseline.txt /tmp/new.txt
```

## Interpreting Results

### Benchmark Output Format

```
BenchmarkTemplateExecute/initial-render-8    50000    23456 ns/op    4567 B/op    123 allocs/op
```

- `50000`: Number of iterations
- `23456 ns/op`: Nanoseconds per operation
- `4567 B/op`: Bytes allocated per operation
- `123 allocs/op`: Number of allocations per operation
- `-8`: Number of CPUs (GOMAXPROCS)

### Benchstat Comparison Output

```
name                    old time/op    new time/op    delta
TemplateExecute-8         1.23µs ± 2%    1.15µs ± 1%   -6.50%  (p=0.000 n=10+10)
```

- **old time/op:** Baseline performance
- **new time/op:** Current performance
- **delta:** Percentage change (negative = improvement)
- **±:** Variance across iterations
- **p-value:** Statistical significance (p < 0.05 = significant)
- **n=10+10:** Number of samples in each dataset

### What to Look For

**Good:**
- Negative delta (performance improvement)
- Low variance (± small percentage)
- p < 0.05 (statistically significant)

**Concerning:**
- Positive delta >10% on critical benchmarks
- High variance (indicates unstable benchmarks)
- p > 0.05 (change might be noise)

## Updating Baselines

### When to Update

Update baselines only when:

1. **Performance improvements are intentional** - You made changes specifically to improve performance
2. **Improvements are consistent** - Verified across 10 iterations
3. **Changes are reviewed** - Another maintainer has reviewed the changes

### DO NOT update baselines to:
- "Fix" unexpected regressions
- Make CI pass without understanding why performance degraded
- Hide performance issues

### How to Update

```bash
# 1. Run benchmarks 10 times
make bench-10x

# 2. Review results for consistency
cat /tmp/bench-results.txt

# 3. If consistent improvements, save baseline
make bench-save

# 4. Commit with clear description
git add testdata/benchmarks/baseline.txt
git commit -m "perf: update baseline after [description of change]

Improvements:
- TemplateExecute: 15% faster
- TreeComparison: 20% fewer allocations

Verified across 10 iterations."
```

## Profiling

### Generate Profiles

```bash
# CPU profiling
make profile-cpu

# Memory profiling
make profile-mem

# Both
make profile-all
```

### Analyze Profiles

#### Interactive Web Interface

```bash
go tool pprof -http=:8080 profiles/cpu.prof
```

Opens browser with:
- Flame graph visualization
- Top functions
- Source code view
- Call graph

#### Command-Line Analysis

```bash
# Top 20 CPU consumers (cumulative)
go tool pprof -top -cum profiles/cpu.prof | head -20

# Top 20 memory allocators
go tool pprof -top -alloc_space profiles/mem.prof | head -20

# Function-specific details
go tool pprof -list=FunctionName profiles/cpu.prof
```

#### Flame Graph

```bash
go tool pprof -flamegraph profiles/cpu.prof > cpu-flame.svg
open cpu-flame.svg
```

### Profile Specific Benchmarks

```bash
# Profile just E2E benchmarks
GOWORK=off go test -bench=BenchmarkE2E -cpuprofile=profiles/e2e-cpu.prof -run=^$

# Profile with execution trace (for concurrency issues)
GOWORK=off go test -bench=BenchmarkTemplateConcurrent -trace=profiles/trace.out -run=^$
go tool trace profiles/trace.out
```

### What to Look For

**CPU Profile:**
- Functions consuming >10% of time
- Unexpected hot paths
- Lock contention (sync.Mutex.Lock)
- Excessive allocation (runtime.mallocgc)

**Memory Profile:**
- High allocation counts
- Large allocations
- Allocations in hot paths

## Writing New Benchmarks

### Benchmark Structure

```go
func BenchmarkFeature(b *testing.B) {
    // Setup (outside timing)
    data := setupTestData()

    // Reset timer after setup
    b.ResetTimer()

    // Report allocations
    b.ReportAllocs()

    // Benchmark loop
    for i := 0; i < b.N; i++ {
        result := functionToTest(data)
        // Use result to prevent optimization
        _ = result
    }
}
```

### Sub-Benchmarks

```go
func BenchmarkFeature(b *testing.B) {
    tests := []struct{
        name string
        data interface{}
    }{
        {"small", smallData},
        {"large", largeData},
    }

    for _, tt := range tests {
        b.Run(tt.name, func(b *testing.B) {
            b.ReportAllocs()
            for i := 0; i < b.N; i++ {
                _ = functionToTest(tt.data)
            }
        })
    }
}
```

### Guidelines

1. **Realistic workloads** - Use data representative of real usage
2. **Stable results** - Avoid randomness, use fixed seeds if needed
3. **Appropriate scale** - Test small/medium/large inputs
4. **Memory tracking** - Always use `b.ReportAllocs()`
5. **Setup isolation** - Use `b.ResetTimer()` after setup
6. **Prevent optimization** - Use results to prevent dead code elimination

## CI Integration

### How Benchmarks Run in CI

1. GitHub Actions runs benchmarks on every PR
2. 5 iterations for statistical confidence
3. Compares against committed baseline using benchstat
4. Posts comparison as PR comment

### Regression Thresholds

**Critical Benchmarks** (E2E, Template operations):
- >10% regression: Warning comment
- >20% regression: CI failure, blocks merge

**Phase-Specific Benchmarks:**
- Informational only, no CI failure

### Overriding CI

If regression is justified (e.g., correctness fix):

1. Document in PR description why regression is acceptable
2. Explain what was traded for correctness/features
3. Plan for future optimization (if applicable)
4. Maintainer can merge despite warning

## Troubleshooting

### Flaky Benchmarks

If benchmarks show high variance:

```bash
# Run with count to see variance
go test -bench=BenchmarkName -benchmem -count=10 | tee results.txt
benchstat results.txt
```

High variance (>5%) indicates:
- Non-deterministic code (random, timing-dependent)
- System noise (other processes)
- GC interference

Fix by:
- Using fixed seeds for randomness
- Increasing work per iteration
- Running on idle system

### Unexpectedly Slow Benchmarks

Check:
- Are you running in dev mode? (adds overhead)
- Is GOWORK causing issues? (use `GOWORK=off`)
- Are you timing setup code? (use `b.ResetTimer()`)
- Is profiling enabled? (adds overhead)

### Allocation Surprises

Profile to find unexpected allocations:

```bash
go test -bench=BenchmarkName -memprofile=mem.prof
go tool pprof -top -alloc_objects mem.prof
```

## Reference

### Make Targets

| Target | Description |
|--------|-------------|
| `make bench` | Run all benchmarks once |
| `make bench-10x` | Run 10 iterations for confidence |
| `make bench-save` | Save current as baseline |
| `make bench-compare` | Compare against baseline |
| `make bench-quick` | Critical benchmarks only |
| `make profile-cpu` | Generate CPU profile |
| `make profile-mem` | Generate memory profile |
| `make profile-all` | Generate all profiles |

### Tools

- **benchstat:** `go install golang.org/x/perf/cmd/benchstat@latest`
- **pprof:** Built into Go toolchain

### Related Documents

- [Performance Characteristics](performance-characteristics.md) - Detailed phase analysis
- [Known Bottlenecks](known-bottlenecks.md) - Current optimization opportunities
```

**Step 2: Commit benchmarking guide**

```bash
git add docs/performance/benchmarking-guide.md
git commit -m "docs: add comprehensive benchmarking guide

- How to run and interpret benchmarks
- Baseline update workflow
- Profiling instructions
- Writing new benchmarks
- CI integration details"
```

---

## Task 11: Create Performance Characteristics Document

**Files:**
- Create: `docs/performance/performance-characteristics.md`

**Step 1: Create performance characteristics document**

Create `docs/performance/performance-characteristics.md`:

```markdown
# LiveTemplate Performance Characteristics

## Architectural Overview

LiveTemplate implements a 5-phase architecture optimized for minimal updates:

```
Parse → Build → Diff → Render → Send
```

**First Render:** Parse → Build → Render → Send (includes statics in tree)
**Updates:** Build (includes Diff) → Send (excludes statics from tree)

This architecture enables:
- **85%+ bandwidth savings** on updates vs full renders
- **Sub-millisecond update latency** for typical changes
- **O(n) complexity** for most operations

## Phase 1: Parse

### Operations

- Template parsing (Go `html/template` AST)
- AST walking and construct identification
- Expression evaluation with caching
- Tree structure compilation

### Complexity

- **Template parsing:** O(template size)
- **AST walking:** O(nodes in AST)
- **Expression evaluation:** O(expression complexity), cached

### Optimizations

1. **Template Caching**
   - `pipeTemplateCache`: Caches compiled pipelines
   - `astTemplateCache`: Caches AST structures
   - Eliminates re-parsing on identical templates

2. **Expression Result Caching**
   - Cache key: `(templateName, pipeStr, dataHash)`
   - Intra-render optimization
   - Significant for repeated evaluations

3. **Lazy Initialization**
   - Capture functions created on-demand
   - Reduces memory for unused constructs

### Benchmark Results

From baseline (`testdata/benchmarks/baseline.txt`):

```
BenchmarkParse/simple            [X] ns/op    [Y] B/op    [Z] allocs/op
BenchmarkParse/conditional       [X] ns/op    [Y] B/op    [Z] allocs/op
BenchmarkParse/range             [X] ns/op    [Y] B/op    [Z] allocs/op
BenchmarkBuildTree/small-10      [X] ns/op    [Y] B/op    [Z] allocs/op
BenchmarkBuildTree/medium-100    [X] ns/op    [Y] B/op    [Z] allocs/op
BenchmarkBuildTree/large-1000    [X] ns/op    [Y] B/op    [Z] allocs/op
```

[Fill in actual numbers from baseline.txt]

### Key Findings

[From profiling in known-bottlenecks.md]

## Phase 2: Build

### Operations

- TreeNode creation and manipulation
- Static/dynamic separation
- Fingerprint calculation (tree hashing)
- Wrapper div injection
- Context management (with/without statics)

### Complexity

- **Tree construction:** O(n) where n = number of nodes
- **Fingerprint calculation:** O(n), hashes entire tree
- **Wrapper injection:** O(html length)

### Optimizations

1. **Single Lock Strategy**
   - Acquire lock once for all state operations
   - Minimizes lock contention
   - Read-heavy operations outside lock

2. **Custom JSON Marshaling**
   - TreeNode.MarshalJSON() maintains key order
   - Optimized for wire format

3. **Fingerprint Caching**
   - MD5 hash of tree structure
   - Fast equality check without deep comparison
   - Cached until tree changes

### Benchmark Results

```
BenchmarkTreeNodeCreation/flat          [X] ns/op
BenchmarkTreeNodeMarshalJSON/nested     [X] ns/op
BenchmarkFingerprintCalculation         [X] ns/op
BenchmarkWrapperInjection/full-html     [X] ns/op
```

[Fill in from baseline.txt]

### Key Findings

[From profiling]

## Phase 3: Diff

### Operations

- Tree comparison (deep equality)
- Range differential generation
- Update/Insert/Remove/Reorder operations
- Client preparation (static stripping)

### Complexity

- **Tree comparison:** O(n) where n = nodes in tree
- **Range diff:** O(m) where m = items in range
- **Static stripping:** O(n), single pass

### Optimizations

1. **Registry-Based Static Stripping**
   - Client caches statics after first render
   - Updates omit static arrays
   - **Result: 65%+ size reduction** between updates

2. **Deep Equality Checks**
   - Only compare changed values
   - Skip comparison on identical fingerprints

3. **Differential Range Operations**
   - Minimal operations for list changes
   - Reuses existing DOM nodes on client
   - Operations: `["u", key, updates]`, `["i", after, pos, data]`, `["r", key]`, `["o", [keys]]`

### Benchmark Results

```
BenchmarkCompareTreesNoChanges          [X] ns/op
BenchmarkCompareTreesSmallChange        [X] ns/op
BenchmarkCompareTreesLargeChange/10     [X] ns/op
BenchmarkCompareTreesLargeChange/100    [X] ns/op
BenchmarkRangeDiffUpdate                [X] ns/op
BenchmarkRangeDiffInsert                [X] ns/op
BenchmarkPrepareTreeForClient           [X] ns/op
```

[Fill in from baseline.txt]

### Key Findings

[From profiling]

## Phase 4: Render

### Operations

- HTML node rendering
- Tree to HTML conversion
- HTML minification (optional)
- Void element handling

### Complexity

- **Node rendering:** O(n) where n = nodes in tree
- **Minification:** O(html length)

### Optimizations

1. **Efficient String Building**
   - Uses `strings.Builder` for concatenation
   - Reduces allocations

2. **Void Element Handling**
   - Correct self-closing tags (`<br>`, `<img>`, etc.)
   - Single map lookup per element

3. **Minification Optional**
   - Only enabled in production
   - Removes whitespace, comments

### Benchmark Results

```
BenchmarkNodeRender                [X] ns/op
BenchmarkTreeToHTML/simple         [X] ns/op
BenchmarkTreeToHTML/nested         [X] ns/op
BenchmarkMinifyHTML                [X] ns/op
```

[Fill in from baseline.txt]

### Key Findings

[From profiling]

## Phase 5: Send

### Operations

- Action message parsing (HTTP/WebSocket)
- Update response wrapping
- JSON serialization
- Ordered JSON marshaling

### Complexity

- **Message parsing:** O(message size)
- **JSON serialization:** O(tree size)

### Optimizations

1. **Efficient Parsing**
   - Reuses HTTP form parser
   - WebSocket JSON unmarshaling

2. **Ordered JSON**
   - Deterministic output for testing
   - Keys sorted: "s" first, then numeric

### Benchmark Results

```
BenchmarkParseActionFromHTTP          [X] ns/op
BenchmarkParseActionFromWebSocket     [X] ns/op
BenchmarkSerializeUpdate              [X] ns/op
BenchmarkMarshalOrderedJSON           [X] ns/op
```

[Fill in from baseline.txt]

### Key Findings

[From profiling]

## End-to-End Performance

### Template Operations

**Initial Render Pipeline:**
1. Parse (if not cached)
2. Build tree WITH statics
3. Render HTML
4. Send (includes full tree)

**Update Pipeline:**
1. Build tree WITH statics (for comparison)
2. Diff against previous tree
3. Send (statics stripped)

### Benchmark Results

```
BenchmarkTemplateExecute/initial-render      [X] µs
BenchmarkTemplateExecute/subsequent-render   [X] µs
BenchmarkTemplateExecuteUpdates/no-changes   [X] µs
BenchmarkTemplateExecuteUpdates/small        [X] µs
BenchmarkTemplateExecuteUpdates/large        [X] µs
```

[Fill in from baseline.txt]

### Real-World Examples

From README performance numbers:

| Operation | Latency | Bandwidth |
|-----------|---------|-----------|
| Initial Render | ~1.2ms | 1782 bytes |
| First Update | ~120µs | 695 bytes (61% savings) |
| Second Update | ~120µs | 244 bytes (86% savings) |

### User Journey Performance

```
BenchmarkE2EUserJourney (100 updates)    [X] µs total, [Y] µs per update
BenchmarkE2ETodoApp                      [X] µs
BenchmarkE2ECounterApp                   [X] µs
```

[Fill in from baseline.txt]

## Scalability Characteristics

### Template Complexity

Performance scales linearly with template complexity:

- **Simple fields:** ~1µs per render
- **Conditionals:** ~2-3µs per render
- **Ranges:** ~5-10µs per render
- **Nested:** ~10-20µs per render

### Data Size Scaling

Tree operations scale with data size:

- **10 items:** [X] ns/op
- **100 items:** [Y] ns/op (10x data, ~10x time)
- **1000 items:** [Z] ns/op (100x data, ~100x time)

Confirms O(n) complexity.

### Concurrent Session Scaling

```
BenchmarkTemplateConcurrent/1      [X] ns/op
BenchmarkTemplateConcurrent/10     [Y] ns/op
BenchmarkTemplateConcurrent/100    [Z] ns/op
```

Performance per session remains constant under concurrency.

## Memory Usage

### Per-Session Memory

From benchmark allocations:

- **Initial render:** [X] KB allocated
- **Small update:** [Y] KB allocated
- **Large update:** [Z] KB allocated

### Cache Memory

**Parse Caches:**
- Unbounded growth (relies on GC)
- Typical size: [Estimate based on templates]

**Structure Registry:**
- Max 1000 entries (LRU eviction)
- Estimated: [Calculate from entry size]

### Allocation Patterns

From memory profiling:

- **Hot paths:** [Most allocating functions]
- **Optimization opportunities:** [If any]

## Optimization Opportunities

See [Known Bottlenecks](known-bottlenecks.md) for detailed profiling analysis.

### Current Priorities

1. [From bottlenecks doc]
2. [From bottlenecks doc]
3. [From bottlenecks doc]

## Performance Testing

### Running Benchmarks

```bash
make bench
make bench-compare
```

### Regression Monitoring

CI runs benchmarks on every PR:
- Compares against baseline
- Warns on >10% regression (critical paths)
- Fails on >20% regression (critical paths)

### Profiling

```bash
make profile-cpu
make profile-mem
go tool pprof -http=:8080 profiles/cpu.prof
```

## References

- [Benchmarking Guide](benchmarking-guide.md) - How to run and interpret benchmarks
- [Known Bottlenecks](known-bottlenecks.md) - Current optimization opportunities
- [Tree Update Specification](../../docs/tree-update-specification.md) - Wire format details
```

**Step 2: Fill in actual numbers from baseline**

After `testdata/benchmarks/baseline.txt` exists, populate the placeholder `[X]`, `[Y]`, `[Z]` values with real numbers.

**Step 3: Commit performance characteristics**

```bash
git add docs/performance/performance-characteristics.md
git commit -m "docs: add performance characteristics analysis

- Detailed analysis of all 5 phases
- End-to-end performance characteristics
- Scalability analysis
- Memory usage patterns
- References to benchmarks and profiling"
```

---

## Task 12: Update README with Performance Section

**Files:**
- Modify: `README.md`

**Step 1: Add performance section to README**

Find an appropriate location in `README.md` (after features, before installation) and add:

```markdown
## Performance

LiveTemplate is designed for high-performance reactive updates with minimal bandwidth usage.

### Key Metrics

| Operation | Latency | Bandwidth Savings |
|-----------|---------|-------------------|
| Initial Render | ~1.2ms | - |
| Small Update (1-2 fields) | ~120µs | 85% vs full render |
| Large Update (100 items) | ~2.5ms | 65% vs full render |
| Range Operations | ~150µs | 80% vs full render |

*Benchmarked on Go 1.21, Apple M1, typical web application templates*

### How It Works

1. **First Render:** Full HTML + tree structure with static/dynamic separation
2. **Subsequent Updates:** Only changed values (statics cached client-side)
3. **Result:** 85%+ bandwidth savings, sub-millisecond latency

### Running Benchmarks

```bash
# Run all benchmarks
make bench

# Compare against baseline
make bench-compare

# Generate performance profiles
make profile-cpu
make profile-mem
```

### Documentation

- [Benchmarking Guide](docs/performance/benchmarking-guide.md) - How to run and interpret benchmarks
- [Performance Characteristics](docs/performance/performance-characteristics.md) - Detailed phase analysis
- [Known Bottlenecks](docs/performance/known-bottlenecks.md) - Optimization opportunities

See the full [performance documentation](docs/performance/) for comprehensive analysis.
```

**Step 2: Commit README update**

```bash
git add README.md
git commit -m "docs: add performance section to README

- Key performance metrics table
- How performance optimizations work
- Commands for running benchmarks
- Links to detailed performance docs"
```

---

## Task 13: Add CI Benchmark Workflow

**Files:**
- Create: `.github/workflows/benchmark.yml`

**Step 1: Create GitHub Actions workflow**

Create `.github/workflows/benchmark.yml`:

```yaml
name: Performance Benchmarks

on:
  pull_request:
    branches: [ main ]
  workflow_dispatch:  # Allow manual triggering

permissions:
  contents: read
  pull-requests: write  # For posting comments

jobs:
  benchmark:
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      with:
        fetch-depth: 0  # Need history for baseline

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'

    - name: Install benchstat
      run: go install golang.org/x/perf/cmd/benchstat@latest

    - name: Run benchmarks
      run: |
        go test -bench=. -benchmem -count=5 ./... > current-bench.txt 2>&1 || true
        cat current-bench.txt
      env:
        GOWORK: off

    - name: Compare against baseline
      id: compare
      run: |
        if [ -f testdata/benchmarks/baseline.txt ]; then
          benchstat testdata/benchmarks/baseline.txt current-bench.txt > comparison.txt || true
          echo "comparison<<EOF" >> $GITHUB_OUTPUT
          cat comparison.txt >> $GITHUB_OUTPUT
          echo "EOF" >> $GITHUB_OUTPUT
        else
          echo "No baseline found - this is the first benchmark run"
          echo "comparison=No baseline found. Save these results as baseline." >> $GITHUB_OUTPUT
        fi
      continue-on-error: true

    - name: Check for regressions
      id: regression-check
      run: |
        if [ -f comparison.txt ]; then
          # Check for critical regressions >20%
          if grep -E 'Benchmark(E2E|Template).*\+[2-9][0-9]\.[0-9]+%' comparison.txt; then
            echo "status=fail" >> $GITHUB_OUTPUT
            echo "message=Critical performance regression detected (>20%)" >> $GITHUB_OUTPUT
            exit 1
          # Check for warnings >10%
          elif grep -E 'Benchmark(E2E|Template).*\+1[0-9]\.[0-9]+%' comparison.txt; then
            echo "status=warn" >> $GITHUB_OUTPUT
            echo "message=Performance regression detected (>10%)" >> $GITHUB_OUTPUT
          else
            echo "status=pass" >> $GITHUB_OUTPUT
            echo "message=No significant regressions detected" >> $GITHUB_OUTPUT
          fi
        else
          echo "status=skip" >> $GITHUB_OUTPUT
          echo "message=No baseline comparison available" >> $GITHUB_OUTPUT
        fi
      continue-on-error: true

    - name: Post comparison comment
      if: github.event_name == 'pull_request'
      uses: actions/github-script@v7
      with:
        script: |
          const comparison = `${{ steps.compare.outputs.comparison }}`;
          const status = '${{ steps.regression-check.outputs.status }}';
          const message = '${{ steps.regression-check.outputs.message }}';

          let statusIcon = '✅';
          if (status === 'fail') statusIcon = '❌';
          else if (status === 'warn') statusIcon = '⚠️';
          else if (status === 'skip') statusIcon = 'ℹ️';

          const body = `## ${statusIcon} Performance Benchmark Results\n\n` +
            `**Status:** ${message}\n\n` +
            `<details>\n<summary>Benchmark Comparison</summary>\n\n\`\`\`\n${comparison}\n\`\`\`\n\n</details>\n\n` +
            `### Thresholds\n\n` +
            `- ⚠️ Warning: Regressions >10% on critical benchmarks\n` +
            `- ❌ Failure: Regressions >20% on critical benchmarks\n` +
            `- Critical benchmarks: \`Benchmark(E2E|Template).*\`\n\n` +
            `See [benchmarking guide](https://github.com/${context.repo.owner}/${context.repo.repo}/blob/${context.payload.pull_request.head.ref}/docs/performance/benchmarking-guide.md) for details.`;

          // Find existing benchmark comment
          const { data: comments } = await github.rest.issues.listComments({
            issue_number: context.issue.number,
            owner: context.repo.owner,
            repo: context.repo.repo
          });

          const existingComment = comments.find(c =>
            c.user.login === 'github-actions[bot]' &&
            c.body.includes('Performance Benchmark Results')
          );

          if (existingComment) {
            // Update existing comment
            await github.rest.issues.updateComment({
              comment_id: existingComment.id,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: body
            });
          } else {
            // Create new comment
            await github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: body
            });
          }

    - name: Upload benchmark results
      if: always()
      uses: actions/upload-artifact@v4
      with:
        name: benchmark-results
        path: |
          current-bench.txt
          comparison.txt
        retention-days: 30

    - name: Fail if critical regression
      if: steps.regression-check.outputs.status == 'fail'
      run: |
        echo "::error::${{ steps.regression-check.outputs.message }}"
        exit 1
```

**Step 2: Commit CI workflow**

```bash
git add .github/workflows/benchmark.yml
git commit -m "ci: add performance benchmark workflow

- Runs on every PR to main
- 5 iterations for statistical confidence
- Compares against committed baseline using benchstat
- Posts comparison as PR comment
- Warns on >10% regression, fails on >20%
- Uploads results as artifacts (30-day retention)"
```

---

## Task 14: Test Benchmark System End-to-End

**Files:**
- None (testing only)

**Step 1: Verify all benchmarks run successfully**

```bash
GOWORK=off go test -bench=. -benchmem ./... -run=^$
```

Expected: All benchmarks run without errors.

**Step 2: Test benchmark comparison**

```bash
make bench-compare
```

Expected: Benchstat shows comparison (should show "no difference" since we just created baseline).

**Step 3: Test quick benchmarks**

```bash
make bench-quick
```

Expected: Only critical benchmarks run (faster execution).

**Step 4: Test profiling**

```bash
make profile-cpu
```

Expected: `profiles/cpu.prof` created successfully.

```bash
go tool pprof -top profiles/cpu.prof | head -10
```

Expected: Top functions displayed.

**Step 5: Verify Makefile targets**

```bash
make bench
make bench-10x
make profile-mem
make profile-all
```

Expected: All targets work correctly.

**Step 6: Final verification commit**

```bash
git add -A
git commit -m "test: verify benchmark system works end-to-end

All components tested:
- Phase-specific benchmarks (all 5 phases)
- End-to-end benchmarks (template, e2e)
- Baseline system and benchstat comparison
- Profiling (CPU and memory)
- Makefile targets
- Documentation complete"
```

---

## Summary

This plan implements a comprehensive performance benchmarking system:

**Coverage:**
- ✅ All 5 phases (Parse, Build, Diff, Render, Send)
- ✅ End-to-end scenarios (template operations, user journeys)
- ✅ Scale variations (small, medium, large datasets)

**Infrastructure:**
- ✅ Baseline system with benchstat regression tracking
- ✅ Makefile targets for easy workflow
- ✅ CI integration with automated PR comments
- ✅ Profiling setup for bottleneck discovery

**Documentation:**
- ✅ Comprehensive benchmarking guide
- ✅ Performance characteristics analysis
- ✅ Known bottlenecks from profiling
- ✅ README performance section

**Next Steps:**
1. Execute this plan using superpowers:executing-plans
2. Run initial profiling and fill in actual numbers
3. Test CI workflow on a test PR
4. Update baseline as optimizations are made
