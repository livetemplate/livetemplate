package livetemplate

import (
	"fmt"
	"strconv"
	"testing"
)

// BenchmarkLargeDocDiff — large-document review workload (prereview's
// rendered git diffs as provenance: many files × many lines, and each
// interaction — queueing a comment on one line — re-renders and diffs the
// whole document while only one line actually changed). Drives the full
// composite pipeline on a single connection; the per-op mutation is O(1),
// so this isolates how render+diff+serialize scale with document size.

type docLine struct {
	Num      int
	Text     string
	Comments int
}

type docFile struct {
	Name  string
	Lines []docLine
}

type largeDocState struct {
	Files []docFile
}

type largeDocController struct {
	files, lines int
}

func (c *largeDocController) Mount(state largeDocState, _ *Context) (largeDocState, error) {
	files := make([]docFile, c.files)
	for f := range files {
		lines := make([]docLine, c.lines)
		for l := range lines {
			lines[l] = docLine{Num: l + 1, Text: fmt.Sprintf("line %d of file %d", l+1, f)}
		}
		files[f] = docFile{Name: fmt.Sprintf("pkg/file_%d.go", f), Lines: lines}
	}
	state.Files = files
	return state, nil
}

// Comment increments the comment count on one line — prereview's queue-a-
// comment interaction. The count grows monotonically, so every op changes
// exactly one line's rendered content.
func (c *largeDocController) Comment(state largeDocState, ctx *Context) (largeDocState, error) {
	f, err := strconv.Atoi(ctx.GetString("file"))
	if err != nil {
		return state, err
	}
	l, err := strconv.Atoi(ctx.GetString("line"))
	if err != nil {
		return state, err
	}
	state.Files[f].Lines[l].Comments++
	return state, nil
}

// Two key strategies, same document: content-hash auto-keys (the default)
// vs explicit data-key. A commented line's content changes every op, so
// auto-keys churn on exactly the item that changed — data-key is the
// documented fix (CLAUDE.md "Best Practices: data-key in Range Templates").
const (
	largeDocTemplate      = `<div>{{range .Files}}<section><h3>{{.Name}}</h3>{{range .Lines}}<div class="line"><span>{{.Num}}</span> {{.Text}}{{if .Comments}} <em>{{.Comments}}</em>{{end}}</div>{{end}}</section>{{end}}</div>`
	largeDocTemplateKeyed = `<div>{{range .Files}}<section data-key="{{.Name}}"><h3>{{.Name}}</h3>{{range .Lines}}<div class="line" data-key="{{.Num}}"><span>{{.Num}}</span> {{.Text}}{{if .Comments}} <em>{{.Comments}}</em>{{end}}</div>{{end}}</section>{{end}}</div>`
)

func BenchmarkLargeDocDiff(b *testing.B) {
	discardLogs(b)
	sizes := []struct{ files, lines int }{
		{10, 100},  // 1k lines — a focused review
		{50, 200},  // 10k lines — a substantial branch diff
		{100, 500}, // 50k lines — a monster generated-code review
	}
	variants := []struct {
		name     string
		template string
	}{
		{"autokey", largeDocTemplate},
		{"datakey", largeDocTemplateKeyed},
	}
	for _, size := range sizes {
		for _, v := range variants {
			// Size axes share one segment (comma, not slash) so the CI
			// capacity-skip regex can match them in one path element.
			b.Run(fmt.Sprintf("files=%d,lines=%d/%s", size.files, size.lines, v.name), func(b *testing.B) {
				app := newCompositeApp(b, v.template,
					&largeDocController{files: size.files, lines: size.lines},
					AsState(&largeDocState{}))
				s := app.connect(b, "")
				frame := []byte(fmt.Sprintf(`{"action":"Comment","data":{"file":"%d","line":"%d"}}`,
					size.files/2, size.lines/2))
				startBytes := wireBytesTotal(s)

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					s.dispatch(b, frame)
				}
				b.StopTimer()
				b.ReportMetric(float64(wireBytesTotal(s)-startBytes)/float64(b.N), "wireB/op")
			})
		}
	}
}
