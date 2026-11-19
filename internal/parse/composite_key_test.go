package parse

import "testing"

// TestCompositeKey covers compositeKey's edge cases directly (#547).
func TestCompositeKey(t *testing.T) {
	nested := &TreeNode{Statics: []string{"-on"}}
	withRange := &TreeNode{Statics: []string{"", ""}, Range: NewRangeData([]interface{}{}, nil)}
	cases := []struct {
		name   string
		node   *TreeNode
		want   string
		wantOK bool
	}{
		{"prefix and two parts", &TreeNode{Statics: []string{`<li data-key="row-`, "-", `">x</li>`}, Dynamics: []interface{}{"0", "9"}}, "row-0-9", true},
		{"lone string dynamic", &TreeNode{Statics: []string{`<li data-key="`, `">x</li>`}, Dynamics: []interface{}{"7"}}, "", false},
		{"lone nested node", &TreeNode{Statics: []string{`<li data-key="`, `">x</li>`}, Dynamics: []interface{}{nested}}, "-on", true},
		{"nil dynamic in the value", &TreeNode{Statics: []string{`<li data-key="a-`, "-", `">x</li>`}, Dynamics: []interface{}{nil, "b"}}, "a--b", true},
		{"literal value", &TreeNode{Statics: []string{`<li data-key="fixed">`, `</li>`}, Dynamics: []interface{}{"x"}}, "", false},
		{"nested range in the value", &TreeNode{Statics: []string{`<li data-key="a-`, `">x</li>`}, Dynamics: []interface{}{withRange}}, "", false},
		{"no key attribute", &TreeNode{Statics: []string{`<li class="`, `">x</li>`}, Dynamics: []interface{}{"c"}}, "", false},
		{"unterminated value", &TreeNode{Statics: []string{`<li data-key="a-`, `-b`}, Dynamics: []interface{}{"x"}}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := compositeKey(tc.node)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("compositeKey = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
