package livetemplate

import (
	"strings"
	"testing"
)

// TestParse_RecursiveTemplate_ReturnsErrorNotCrash exercises the real
// user-facing path: New(...).Parse(recursiveSrc). Before the parse-time cycle
// guard, a self-referential {{template}} inlined forever and stack-overflowed
// here (a fatal, unrecoverable runtime error). The guard turns it into a clean
// error. A stack overflow cannot be recover()'d, so if the guard regresses this
// test binary dies outright — itself an unambiguous failure signal; there is no
// need for (and no point in) a recover()-based assertion.
func TestParse_RecursiveTemplate_ReturnsErrorNotCrash(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "direct self-recursion (file tree)",
			src: `{{define "treeNode"}}<li data-key="{{.Path}}"><span>{{.Name}}</span>` +
				`{{if .IsDir}}<ul>{{range .Children}}{{template "treeNode" .}}{{end}}</ul>{{end}}` +
				`</li>{{end}}` +
				`<ul>{{template "treeNode" .}}</ul>`,
		},
		{
			name: "mutual recursion",
			src: `{{define "a"}}<div>{{template "b" .}}</div>{{end}}` +
				`{{define "b"}}<span>{{template "a" .}}</span>{{end}}` +
				`{{template "a" .}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Must(New("test")).Parse(tt.src)
			if err == nil {
				t.Fatal("expected an error for a recursive template, got nil")
			}
			if !strings.Contains(err.Error(), "recursive") {
				t.Errorf("error should explain the recursion, got: %v", err)
			}
		})
	}
}

// TestParse_NonRecursiveComposition_StillWorks is the companion positive gate:
// ordinary (acyclic) {{define}}/{{template}} composition must keep parsing
// through the public API exactly as before the guard.
func TestParse_NonRecursiveComposition_StillWorks(t *testing.T) {
	src := `{{define "header"}}<h1>{{.Title}}</h1>{{end}}` +
		`{{template "header" .}}<main>{{.Body}}</main>`

	if _, err := Must(New("test")).Parse(src); err != nil {
		t.Fatalf("non-recursive composition must parse cleanly, got: %v", err)
	}
}
