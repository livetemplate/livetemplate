package parse

import (
	"html/template"
	"testing"
)

// FuzzFlattenTemplate guards the flatten path against inputs that make
// walkAndFlatten recurse without bound. Before the cycle guard, a
// self-referential {{template}} stack-overflowed here; the seed corpus pins
// exactly those shapes. FlattenTemplate must always return (a flattened string
// or a ParseError) and never panic — Go's fuzzer fails the run on any panic or
// fatal stack overflow.
func FuzzFlattenTemplate(f *testing.F) {
	seeds := []string{
		// Acyclic composition — must flatten fine (the common case).
		`{{define "h"}}<h1>{{.T}}</h1>{{end}}{{template "h" .}}<p>{{.B}}</p>`,
		// A diamond: same template invoked on non-nested paths, not a cycle.
		`{{define "leaf"}}<span>{{.}}</span>{{end}}` +
			`{{define "row"}}{{template "leaf" .A}}{{template "leaf" .B}}{{end}}` +
			`{{template "row" .}}`,
		// Direct self-recursion — previously overflowed at Parse.
		`{{define "r"}}<li>{{.N}}{{range .C}}{{template "r" .}}{{end}}</li>{{end}}` +
			`{{template "r" .}}`,
		// Mutual recursion.
		`{{define "a"}}{{template "b" .}}{{end}}` +
			`{{define "b"}}{{template "a" .}}{{end}}` +
			`{{template "a" .}}`,
		// Self-referential entry point (no {{define}}).
		`<div>{{template "main" .}}</div>`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		tmpl, err := template.New("main").Parse(src)
		if err != nil {
			t.Skip() // Only exercise inputs Go's own parser accepts.
		}
		// The contract under test: returns, never panics/overflows. The result
		// (string or error) is deliberately unused — the guard is what matters.
		_, _, _ = FlattenTemplate(tmpl)
	})
}
