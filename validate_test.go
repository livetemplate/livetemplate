package livetemplate

import (
	"embed"
	"testing"
)

// brokenComponentFS embeds a component whose body has a template syntax error,
// used to prove Validate reports a bad component SET as an infrastructure error
// (not as a Diagnostic against the validated text).
//
//go:embed testdata/validate/broken.gotemplate
var brokenComponentFS embed.FS

// componentSet returns the button/greeting component set the component-template
// tests embed (testComponentFS, defined in component_templates_test.go), reused
// here so Validate is exercised against real components.
func componentSet() *TemplateSet {
	return &TemplateSet{
		FS:        testComponentFS,
		Pattern:   "testdata/components/*.gotemplate",
		Namespace: "test",
	}
}

// TestValidate_Clean: a well-formed template with no components validates clean.
func TestValidate_Clean(t *testing.T) {
	diags, err := Validate("<div><p>{{.Name}}</p>{{range .Items}}<li>{{.}}</li>{{end}}</div>")
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected clean, got %d diagnostic(s): %+v", len(diags), diags)
	}
}

// TestValidate_ComponentResolves is the fidelity guard: a template that invokes
// a component must validate CLEAN when its component set is supplied. If this
// regresses, every component-using block false-positives — worse than the gap
// Validate closes.
func TestValidate_ComponentResolves(t *testing.T) {
	diags, err := Validate(
		`<div>{{template "test:button:v1" .}}</div>`,
		WithValidateComponents(componentSet()),
	)
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected a component-using template to validate clean, got %d diagnostic(s): %+v", len(diags), diags)
	}
}

// TestValidate_ComponentMissingSet: the same component reference WITHOUT its set
// is an unresolved template — a real Diagnostic, not clean.
func TestValidate_ComponentMissingSet(t *testing.T) {
	diags, err := Validate(`<div>{{template "test:button:v1" .}}</div>`)
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for an unresolved component, got %d: %+v", len(diags), diags)
	}
}

// TestValidate_UnclosedRange is the gap Validate exists to close: tinkerdown's
// own validate passes this today (it never runs the template parser on block
// content); Validate must catch it.
func TestValidate_UnclosedRange(t *testing.T) {
	diags, err := Validate("<ul>{{range .Items}}<li>{{.}}</li></ul>")
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic for an unclosed range, got %d: %+v", len(diags), diags)
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("expected SeverityError, got %v", diags[0].Severity)
	}
	if diags[0].Line != 1 {
		t.Errorf("expected line 1, got %d (msg: %q)", diags[0].Line, diags[0].Message)
	}
}

// TestValidate_UnknownFunction proves Validate enforces the function set: an
// undefined function is rejected with its name in the message.
func TestValidate_UnknownFunction(t *testing.T) {
	diags, err := Validate("<div>{{bogusFunc .Name}}</div>")
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for an unknown function, got %d: %+v", len(diags), diags)
	}
	if diags[0].Line != 1 {
		t.Errorf("expected line 1, got %d", diags[0].Line)
	}
}

// TestValidate_FrameworkBuiltinResolves is the "unreachable downstream" proof:
// a template using a framework builtin (lvtClientScriptURL) validates clean —
// a stdlib-only parser without livetemplate's funcMap would reject it as an
// undefined function, which is why this check cannot live downstream.
func TestValidate_FrameworkBuiltinResolves(t *testing.T) {
	diags, err := Validate(`<script src="{{lvtClientScriptURL}}"></script>`)
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected a framework-builtin template to validate clean, got %d: %+v", len(diags), diags)
	}
}

// TestValidate_LineNumber: an error deeper in the text reports the right line.
func TestValidate_LineNumber(t *testing.T) {
	// The unknown function is on line 3.
	src := "<div>\n  <p>ok</p>\n  {{bogusFunc}}\n</div>"
	diags, err := Validate(src)
	if err != nil {
		t.Fatalf("unexpected infrastructure error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Line != 3 {
		t.Errorf("expected line 3, got %d (msg: %q)", diags[0].Line, diags[0].Message)
	}
}

// TestValidate_BrokenComponentSetIsInfraError: a component set that itself fails
// to parse is the caller's problem — reported as an error, never a Diagnostic
// against the validated text.
func TestValidate_BrokenComponentSetIsInfraError(t *testing.T) {
	broken := &TemplateSet{
		FS:        brokenComponentFS,
		Pattern:   "testdata/validate/broken.gotemplate",
		Namespace: "test",
	}
	diags, err := Validate("<div>ok</div>", WithValidateComponents(broken))
	if err == nil {
		t.Fatalf("expected an infrastructure error for a broken component set, got diags=%+v", diags)
	}
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics alongside an infra error, got %+v", diags)
	}
}
