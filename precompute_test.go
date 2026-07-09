package livetemplate

import (
	"bytes"
	"strings"
	"sync/atomic"
	"testing"
)

// expensiveCalls counts invocations of ReportState.Expensive across a render so the
// test can assert whether an unreferenced method was eagerly precomputed.
var expensiveCalls int64

type ReportState struct {
	Name string
}

// Expensive is a zero-arg method that qualifies for precompute and records that it
// ran. In real code this would be the "expensive or side-effecting" method the eager
// precompute footgun warns about.
func (s ReportState) Expensive() string {
	atomic.AddInt64(&expensiveCalls, 1)
	return "report"
}

// TestPrecompute_UnreferencedMethodNotCalled is the footgun-closed proof: a template
// that never references .Expensive must not trigger the method during render, while a
// template that does reference it must.
func TestPrecompute_UnreferencedMethodNotCalled(t *testing.T) {
	t.Run("unreferenced method is skipped", func(t *testing.T) {
		atomic.StoreInt64(&expensiveCalls, 0)

		tmpl := Must(New("unref"))
		if _, err := tmpl.Parse(`<div>{{.Name}}</div>`); err != nil {
			t.Fatalf("Parse: %v", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, ReportState{Name: "hi"}); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if got := atomic.LoadInt64(&expensiveCalls); got != 0 {
			t.Errorf("Expensive was called %d times for a template that never references it", got)
		}
		if !strings.Contains(buf.String(), "hi") {
			t.Errorf("rendered output missing field value: %q", buf.String())
		}
	})

	t.Run("referenced method still runs and renders", func(t *testing.T) {
		atomic.StoreInt64(&expensiveCalls, 0)

		tmpl := Must(New("ref"))
		if _, err := tmpl.Parse(`<div>{{.Name}} {{.Expensive}}</div>`); err != nil {
			t.Fatalf("Parse: %v", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, ReportState{Name: "hi"}); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if got := atomic.LoadInt64(&expensiveCalls); got == 0 {
			t.Error("Expensive was never called for a template that references it")
		}
		if !strings.Contains(buf.String(), "report") {
			t.Errorf("rendered output missing method result: %q", buf.String())
		}
	})
}

// TestPrecompute_ScopingSurvivesClone guards the per-session render path: production
// renders through clones, not the master Template, so the allow-set must propagate
// through Clone. A Clone that dropped precomputeAllow would fall back to nil
// (precompute-all) and render identically — the optimization would silently revert
// with every test still green — so this asserts the skip specifically on the clone.
func TestPrecompute_ScopingSurvivesClone(t *testing.T) {
	tmpl := Must(New("clone"))
	if _, err := tmpl.Parse(`<div>{{.Name}}</div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	clone, err := tmpl.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	atomic.StoreInt64(&expensiveCalls, 0)
	var buf bytes.Buffer
	if err := clone.Execute(&buf, ReportState{Name: "hi"}); err != nil {
		t.Fatalf("Execute on clone: %v", err)
	}
	if got := atomic.LoadInt64(&expensiveCalls); got != 0 {
		t.Errorf("clone precomputed unreferenced Expensive %d times; allow-set did not survive Clone", got)
	}
}

// TestPrecompute_IndexStringLiteralRenders exercises the dynamic-access path the
// string-literal scanning exists for: {{index . "Expensive"}} reaches the method
// result via a string key, so "Expensive" must be in the allow-set and the value must
// render (not nil). This is the silent-wrong-render case the scanning prevents.
func TestPrecompute_IndexStringLiteralRenders(t *testing.T) {
	tmpl := Must(New("index"))
	if _, err := tmpl.Parse(`<div>{{index . "Expensive"}}</div>`); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	atomic.StoreInt64(&expensiveCalls, 0)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ReportState{Name: "hi"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := atomic.LoadInt64(&expensiveCalls); got == 0 {
		t.Error("Expensive was not precomputed for a {{index . \"Expensive\"}} reference")
	}
	if !strings.Contains(buf.String(), "report") {
		t.Errorf("index string-literal access rendered nil instead of the method result: %q", buf.String())
	}
}
