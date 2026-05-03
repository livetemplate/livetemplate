package livetemplate

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/compat"
)

func BenchmarkUserJourney(b *testing.B) {
	generator := NewActivityGenerator(42)
	journey := generator.GenerateJourney(100) // 100 activities

	templateStr := `<div>
        {{.title}}
        {{range .items}}<li>{{.text}}</li>{{end}}
        Count: {{.count}}
    </div>`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		simulator := NewStateSimulator()
		tmpl := &Template{
			templateStr: templateStr,
		}
		_, _ = tmpl.Parse(tmpl.templateStr)

		for j, activity := range journey {
			simulator.ApplyActivity(activity)
			state := simulator.GetState()

			if j == 0 {
				_ = tmpl.generateInitialTreeWithoutRegistry(state, templateStr)
			} else {
				newTree, _ := compat.ParseTemplateToTree("test", templateStr, state)
				tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
				tmpl.lastTree = newTree
			}
		}
	}
}
