package render

import (
	"testing"
)

func TestExpandBracketAttributes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "two actions in bracket",
			input: `<div lvt-el:addClass:on:[save,delete]:pending="opacity-50">`,
			want:  `<div lvt-el:addClass:on:save:pending="opacity-50" lvt-el:addClass:on:delete:pending="opacity-50">`,
		},
		{
			name:  "three actions in bracket",
			input: `<div lvt-el:addClass:on:[a,b,c]:error="X">`,
			want:  `<div lvt-el:addClass:on:a:error="X" lvt-el:addClass:on:b:error="X" lvt-el:addClass:on:c:error="X">`,
		},
		{
			name:  "single action no bracket passthrough",
			input: `<div lvt-el:addClass:on:save:pending="X">`,
			want:  `<div lvt-el:addClass:on:save:pending="X">`,
		},
		{
			name:  "unscoped no action passthrough",
			input: `<div lvt-el:addClass:on:pending="X">`,
			want:  `<div lvt-el:addClass:on:pending="X">`,
		},
		{
			name:  "lvt-fx bracket expansion",
			input: `<div lvt-fx:highlight:on:[save,update]:success="flash">`,
			want:  `<div lvt-fx:highlight:on:save:success="flash" lvt-fx:highlight:on:update:success="flash">`,
		},
		{
			name:  "lvt-form bracket expansion boolean attr",
			input: `<form lvt-form:preserve:on:[create,edit]:error>`,
			want:  `<form lvt-form:preserve:on:create:error lvt-form:preserve:on:edit:error>`,
		},
		{
			name:  "multiple bracket attrs in same element",
			input: `<div lvt-el:addClass:on:[save,delete]:pending="loading" lvt-el:removeClass:on:[save,delete]:done="loading">`,
			want:  `<div lvt-el:addClass:on:save:pending="loading" lvt-el:addClass:on:delete:pending="loading" lvt-el:removeClass:on:save:done="loading" lvt-el:removeClass:on:delete:done="loading">`,
		},
		{
			name:  "no lvt attributes passthrough",
			input: `<div class="hello" id="world">content</div>`,
			want:  `<div class="hello" id="world">content</div>`,
		},
		{
			name:  "mixed bracket and regular attrs",
			input: `<button lvt-el:addClass:on:[save,delete]:pending="opacity-50" class="btn">Save</button>`,
			want:  `<button lvt-el:addClass:on:save:pending="opacity-50" lvt-el:addClass:on:delete:pending="opacity-50" class="btn">Save</button>`,
		},
		{
			name:  "actions with whitespace trimmed",
			input: `<div lvt-el:addClass:on:[save, delete]:pending="X">`,
			want:  `<div lvt-el:addClass:on:save:pending="X" lvt-el:addClass:on:delete:pending="X">`,
		},
		{
			name:  "done lifecycle state",
			input: `<div lvt-el:removeClass:on:[a,b]:done="loading">`,
			want:  `<div lvt-el:removeClass:on:a:done="loading" lvt-el:removeClass:on:b:done="loading">`,
		},
		{
			name:  "success lifecycle state",
			input: `<div lvt-el:toggleClass:on:[x,y]:success="active">`,
			want:  `<div lvt-el:toggleClass:on:x:success="active" lvt-el:toggleClass:on:y:success="active">`,
		},
		{
			name:  "empty action filtered out",
			input: `<div lvt-el:addClass:on:[save,,delete]:pending="X">`,
			want:  `<div lvt-el:addClass:on:save:pending="X" lvt-el:addClass:on:delete:pending="X">`,
		},
		{
			name:  "trailing comma filtered",
			input: `<div lvt-el:addClass:on:[save,]:pending="X">`,
			want:  `<div lvt-el:addClass:on:save:pending="X">`,
		},
		{
			name: "all empty actions returns match unchanged",
			// Degenerate syntax is preserved as-is rather than silently dropped,
			// serving as a visible signal that the bracket list is malformed.
			input: `<div lvt-el:addClass:on:[,,]:pending="X">`,
			want:  `<div lvt-el:addClass:on:[,,]:pending="X">`,
		},
		{
			name:  "single-quoted value expanded correctly",
			input: `<div lvt-el:addClass:on:[save,delete]:pending='opacity-50'>`,
			want:  `<div lvt-el:addClass:on:save:pending='opacity-50' lvt-el:addClass:on:delete:pending='opacity-50'>`,
		},
		{
			name:  "no lvt prefix passthrough",
			input: `<script>let cfg = { "on:[save]": true };</script>`,
			want:  `<script>let cfg = { "on:[save]": true };</script>`,
		},
		{
			name:  "hyphenated action names",
			input: `<div lvt-el:addClass:on:[create-todo,delete-item]:pending="loading">`,
			want:  `<div lvt-el:addClass:on:create-todo:pending="loading" lvt-el:addClass:on:delete-item:pending="loading">`,
		},
		{
			// Known limitation: unquoted attribute values are not captured by the
			// regex value group (which requires ="..." or ='...'). The regex matches
			// the prefix+bracket+state portion and expands it, leaving the unquoted
			// =value attached to the last expansion. This produces mangled output —
			// hence quoting is required (documented in client-attributes.md).
			name:  "unquoted value produces mangled output",
			input: `<div lvt-el:addClass:on:[a,b]:pending=loading>`,
			want:  `<div lvt-el:addClass:on:a:pending lvt-el:addClass:on:b:pending=loading>`, // mangled: first attr loses value
		},
		{
			// Known limitation: bracket syntax inside <script> blocks IS expanded
			// because the regex operates on raw template source, not parsed HTML.
			// In practice, lvt-el:/lvt-fx:/lvt-form: prefixes are specific enough
			// that false matches inside scripts are unlikely.
			name:  "bracket syntax inside script tag is expanded",
			input: `<script>let x = 'lvt-el:addClass:on:[a,b]:pending="X"';</script>`,
			want:  `<script>let x = 'lvt-el:addClass:on:a:pending="X" lvt-el:addClass:on:b:pending="X"';</script>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandBracketAttributes(tt.input)
			if got != tt.want {
				t.Errorf("ExpandBracketAttributes(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}
