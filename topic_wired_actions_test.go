package livetemplate

import (
	"maps"
	"slices"
	"testing"
)

func wiredKeys(t *testing.T, tmpl string, declared ...string) []string {
	t.Helper()
	re, err := compileActionAttributes(declared)
	if err != nil {
		t.Fatalf("compileActionAttributes(%q): %v", declared, err)
	}
	return slices.Sorted(maps.Keys(extractWiredActionNames(tmpl, re)))
}

func TestExtractWiredActionNames(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		declared []string
		want     []string
	}{
		{"button name", `<button name="Save">x</button>`, nil, []string{"save"}},
		{"type=button is not submitting", `<button type="button" name="Save">x</button>`, nil, nil},
		{"input submit", `<input type="submit" name="AddItem">`, nil, []string{"add_item"}},
		{"lvt-on leading identifier", `<a lvt-on:click="Toggle('x')">x</a>`, nil, []string{"toggle"}},
		{"lvt-on single-quoted is not read", `<a lvt-on:click='Toggle'>x</a>`, nil, nil},
		{"declared attribute", `<div lvt-x:rating="SetRating"></div>`, []string{"lvt-x:rating"}, []string{"set_rating"}},
		{"undeclared attribute stays out", `<div lvt-x:rating="SetRating"></div>`, nil, nil},
		{"declared name in another case", `<div LVT-X:Rating="SetRating"></div>`, []string{"lvt-x:RATING"}, []string{"set_rating"}},
		{"declared leading identifier", `<div lvt-x:rate="Rate(3)"></div>`, []string{"lvt-x:rate"}, []string{"rate"}},
		{"declared dynamic value is skipped", `<div lvt-x:rating="{{.Action}}"></div>`, []string{"lvt-x:rating"}, nil},
		{"declared single-quoted is not read", `<div lvt-x:rating='SetRating'></div>`, []string{"lvt-x:rating"}, nil},
		{"declared inside range", `{{range .Items}}<div lvt-x:rating="SetRating"></div>{{end}}`, []string{"lvt-x:rating"}, []string{"set_rating"}},
		{"data- prefix does not match", `<div data-lvt-x:rating="SetRating"></div>`, []string{"lvt-x:rating"}, nil},
		{"-extra suffix does not match", `<div lvt-x:rating-extra="SetRating"></div>`, []string{"lvt-x:rating"}, nil},
		{"dot in a name matches literally", `<div lvt-xya="SetRating"></div>`, []string{"lvt-x.a"}, nil},
		{"dot in a name matches itself", `<div lvt-x.a="SetRating"></div>`, []string{"lvt-x.a"}, []string{"set_rating"}},
		{"two declared names", `<div lvt-x:a="One" lvt-x:ab="Two"></div>`, []string{"lvt-x:a", "lvt-x:ab"}, []string{"one", "two"}},
		// The scan reads raw template text, as the lvt-on: scan does, so a
		// match in a comment counts. That only widens a dev warning.
		{"declared inside a comment counts", `<!-- <div lvt-x:rating="SetRating"> -->`, []string{"lvt-x:rating"}, []string{"set_rating"}},
		{"nothing wired", `<p>hi</p>`, []string{"lvt-x:rating"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wiredKeys(t, tt.tmpl, tt.declared...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("wired keys = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompileActionAttributes(t *testing.T) {
	if re, err := compileActionAttributes(nil); re != nil || err != nil {
		t.Errorf("no names: got (%v, %v), want (nil, nil)", re, err)
	}
	if _, err := compileActionAttributes([]string{"lvt-x:Rating", "LVT-X:RATING"}); err != nil {
		t.Errorf("a name repeated in another case: %v, want no error", err)
	}
	for _, bad := range []string{"", " lvt-x:rating", "lvt-x rating", `lvt-x:"a`, "1lvt", "lvt-on:click", "LVT-ON:click"} {
		if _, err := compileActionAttributes([]string{bad}); err == nil {
			t.Errorf("compileActionAttributes(%q): want an error", bad)
		}
	}
}

// New compiles the declared names once, and per-session clones share the
// wired set Parse built from them.
func TestNew_WithActionAttributes(t *testing.T) {
	if _, err := New("test", WithActionAttributes()); err != nil {
		t.Errorf("WithActionAttributes() with no names: %v, want no error", err)
	}
	if _, err := New("test", WithActionAttributes("lvt-on:click")); err == nil {
		t.Error(`WithActionAttributes("lvt-on:click"): want an error from New`)
	}

	tmpl, err := New("test", WithActionAttributes("lvt-x:rating"))
	if err != nil {
		t.Fatal(err)
	}
	if tmpl, err = tmpl.Parse(`<div lvt-x:rating="SetRating"></div>`); err != nil {
		t.Fatal(err)
	}
	clone, err := tmpl.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if !clone.isClientWiredAction("setRating") {
		t.Error("a clone does not see the declared lvt-x:rating action")
	}
}
