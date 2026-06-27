package compat

import "testing"

func TestStripHTMLComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no comments untouched", `<div>{{.X}}</div>`, `<div>{{.X}}</div>`},
		{"simple comment stripped", `<div><!-- secret -->{{.X}}</div>`, `<div>{{.X}}</div>`},
		{"comment with action stripped wholesale", `<!-- {{.X}} -->after`, `after`},
		{"action in attribute preserved", `<div class="{{.C}}">{{.X}}</div>`, `<div class="{{.C}}">{{.X}}</div>`},
		{
			"comment-like text in attribute preserved",
			`<div title="<!-- not a comment -->">x</div>`,
			`<div title="<!-- not a comment -->">x</div>`,
		},
		{
			"template comment preserved (parser strips it later)",
			`<div><!-- html -->{{/* tmpl */}}</div>`,
			`<div>{{/* tmpl */}}</div>`,
		},
		{"IE conditional comment stripped", `<div><!--[if IE]>x<![endif]--></div>`, `<div></div>`},
		// RAWTEXT/RCDATA: tokenizer does not treat <!-- --> as a comment inside
		// these elements, so it is left verbatim (matches html/template keeping
		// <style> comments; <script>/<textarea> are documented residuals).
		{"style comment kept (RAWTEXT)", `<style>/* x */<!-- d -->.a{}</style>`, `<style>/* x */<!-- d -->.a{}</style>`},
		{"script comment kept (RAWTEXT)", `<script>var a=1;<!-- c --></script>`, `<script>var a=1;<!-- c --></script>`},
		{"textarea comment kept (RCDATA)", `<textarea><!-- keep --></textarea>`, `<textarea><!-- keep --></textarea>`},
		// `<!--` inside a {{...}} action must NOT be treated as an HTML comment:
		// the action span is masked before stripping, matching html/template
		// (which never strips inside an action).
		{"comment marker in action string preserved", `<div>{{"<!-- x -->"}}</div>`, `<div>{{"<!-- x -->"}}</div>`},
		{"comment marker in template comment preserved", `<div>{{/* <!-- */}}{{.X}}</div>`, `<div>{{/* <!-- */}}{{.X}}</div>`},
		{"comment marker in string literal preserved", `{{if eq .Sep "<!--"}}y{{end}}`, `{{if eq .Sep "<!--"}}y{{end}}`},
		{"action with }} in string preserved while stripping comment", `<div>{{"a}}b"}}<!-- c --></div>`, `<div>{{"a}}b"}}</div>`},
		{"real comment stripped, action-internal marker kept", `<!-- gone -->{{"<!-- kept -->"}}`, `{{"<!-- kept -->"}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripHTMLComments(tc.in); got != tc.want {
				t.Errorf("StripHTMLComments(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}
