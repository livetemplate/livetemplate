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
