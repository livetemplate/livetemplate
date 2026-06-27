package livetemplate

import (
	"bytes"
	"strings"
	"testing"
)

// TestExecuteUpdates_StripsHTMLComments is the regression test for #468: HTML
// comments must not survive into the static segments shipped to the client,
// matching html/template (which strips them during its escape pass). This is
// the issue's own repro promoted to an assertion.
func TestExecuteUpdates_StripsHTMLComments(t *testing.T) {
	const src = `<div><!-- SECRET-HTML-COMMENT -->{{.X}}{{/* SECRET-TMPL-COMMENT */}}</div>`

	tmpl := Must(New("cmt"))
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, map[string]any{"X": "hi"}); err != nil {
		t.Fatalf("ExecuteUpdates() failed: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "SECRET-HTML-COMMENT") {
		t.Errorf("HTML comment leaked into wire output (should be stripped like html/template):\n%s", out)
	}
	if !strings.Contains(out, "hi") {
		t.Errorf("dynamic value missing from output:\n%s", out)
	}
}

// TestExecuteUpdates_PreservesCommentLikeAttributeValue guards against the
// over-eager-regex failure mode: comment-like text inside an attribute value is
// real content and must be preserved.
func TestExecuteUpdates_PreservesCommentLikeAttributeValue(t *testing.T) {
	const src = `<div title="<!-- not a comment -->">{{.X}}</div>`

	tmpl := Must(New("attr"))
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteUpdates(&buf, map[string]any{"X": "hi"}); err != nil {
		t.Fatalf("ExecuteUpdates() failed: %v", err)
	}

	if !strings.Contains(buf.String(), "not a comment") {
		t.Errorf("comment-like attribute value was wrongly stripped:\n%s", buf.String())
	}
}

// TestParse_FullHTMLMarkerOnlyInComment_StillWraps guards the regression where
// isFullHTML was computed on the pre-strip text: a `<!DOCTYPE`/`<html` marker
// living only inside a comment made the full-document path run on what becomes
// a bare fragment after stripping, skipping the data-lvt-id wrapper and
// breaking client update targeting.
func TestParse_FullHTMLMarkerOnlyInComment_StillWraps(t *testing.T) {
	const src = `<!-- <!DOCTYPE html> --><div>{{.X}}</div>`

	tmpl := Must(New("dt"))
	if _, err := tmpl.Parse(src); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{"X": "hi"}); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	if !strings.Contains(buf.String(), "data-lvt-id") {
		t.Errorf("fragment (after comment strip) missing data-lvt-id wrapper:\n%s", buf.String())
	}
}

// TestParse_DefineInsideComment_NotResolved confirms a {{define}} living inside
// an HTML comment is removed (like html/template strips it) rather than resolved
// during composition. Comments are stripped before parsing, so the {{template}}
// call that references it is left undefined and parsing fails — instead of
// silently inlining the commented-out define. Regression for #468.
func TestParse_DefineInsideComment_NotResolved(t *testing.T) {
	const src = `<!-- {{define "footer"}}© Corp{{end}} -->{{template "footer" .}}`

	tmpl := Must(New("dc"))
	if _, err := tmpl.Parse(src); err == nil {
		t.Fatal("expected an error: a {{define}} inside a comment should be stripped, leaving template \"footer\" undefined")
	}
}

// TestExecuteUpdates_CommentMarkerInsideAction guards the regression where a
// literal `<!--` inside a {{...}} action was mistaken for an HTML comment: the
// first form silently dropped the action's output, the latter two failed to
// parse. All must round-trip like html/template (which never strips inside an
// action).
func TestExecuteUpdates_CommentMarkerInsideAction(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"literal comment in action output", `<div>{{"<!-- x -->"}}</div>`},
		{"comment marker in template comment", `<div>{{/* <!-- */}}{{.X}}</div>`},
		{"comment marker in string literal", `{{if eq .Sep "<!--"}}yes{{end}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl := Must(New("act"))
			if _, err := tmpl.Parse(c.src); err != nil {
				t.Fatalf("Parse() failed (should be valid like on stdlib): %v", err)
			}
			var buf bytes.Buffer
			if err := tmpl.ExecuteUpdates(&buf, map[string]any{"X": "hi", "Sep": "<!--"}); err != nil {
				t.Fatalf("ExecuteUpdates() failed: %v", err)
			}
		})
	}
}
