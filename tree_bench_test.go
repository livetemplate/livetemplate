package livetemplate

import (
	"testing"

	"github.com/livetemplate/livetemplate/internal/keys"
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
			keyGen:      keys.NewGenerator(),
		}
		_, _ = tmpl.Parse(tmpl.templateStr)

		for j, activity := range journey {
			simulator.ApplyActivity(activity)
			state := simulator.GetState()

			if j == 0 {
				_, _ = tmpl.generateInitialTreeWithoutRegistry(templateStr, state)
			} else {
				newTree, _ := parseTemplateToTree("test", templateStr, state, tmpl.keyGen)
				tmpl.compareTreesAndGetChanges(tmpl.lastTree, newTree)
				tmpl.lastTree = newTree
			}
		}
	}
}
