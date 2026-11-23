package send

import (
	"encoding/json"
	"strings"
	"testing"
)

func decode(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func encode(t *testing.T, m map[string]interface{}) string {
	t.Helper()
	b, err := MarshalOrderedJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInternStatics(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{
			name: "repeats across range items and op payloads",
			in:   `{"0":{"d":[{"0":{"0":"a","s":["<i>","</i>"]}},{"0":{"0":"b","s":["<i>","</i>"]}}],"s":["",""]},"1":[["u","k",{"0":{"0":"c","s":["<i>","</i>"]}}]],"s":["<p>","</p>"]}`,
			want: `{"0":{"d":[{"0":{"0":"a","s":0}},{"0":{"0":"b","s":0}}],"s":["",""]},"1":[["u","k",{"0":{"0":"c","s":0}}]],"S":[["<i>","</i>"]],"s":["<p>","</p>"]}`,
		},
		{
			name: "nothing repeats: unchanged",
			in:   `{"0":{"0":"a","s":["<i>","</i>"]},"s":["<p>","</p>"]}`,
			want: `{"0":{"0":"a","s":["<i>","</i>"]},"s":["<p>","</p>"]}`,
		},
		{
			name: "same parts, different split: distinct",
			in:   `{"0":{"s":["ab","c"]},"1":{"s":["a","bc"]}}`,
			want: `{"0":{"s":["ab","c"]},"1":{"s":["a","bc"]}}`,
		},
		{
			name: "the root shares an index too",
			in:   `{"0":{"s":["<b>","</b>"]},"s":["<b>","</b>"]}`,
			want: `{"0":{"s":0},"S":[["<b>","</b>"]],"s":0}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := decode(t, tc.in)
			InternStatics(m)
			if got := encode(t, m); got != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}

// Indexes follow content order, so the same tree always encodes the same way.
func TestInternStaticsDeterministic(t *testing.T) {
	in := `{"2":{"s":["x"]},"1":{"s":["y"]},"0":{"s":["x"]},"3":{"s":["y"]}}`
	var first string
	for i := range 20 {
		m := decode(t, in)
		InternStatics(m)
		got := encode(t, m)
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("not deterministic: %s vs %s", got, first)
		}
	}
}

func BenchmarkInternStatics(b *testing.B) {
	row := `{"0":{"0":"L1","1":"some text here","s":["<div class=\"line-row\"><span class=\"gutter\">","</span><span class=\"content\">","</span></div>"]}}`
	items := strings.TrimSuffix(strings.Repeat(row+",", 400), ",")
	src := `{"0":{"d":[` + items + `],"s":["",""]},"s":["<main>","</main>"]}`
	b.ReportAllocs()
	trees := make([]map[string]interface{}, b.N)
	for i := range trees {
		_ = json.Unmarshal([]byte(src), &trees[i])
	}
	b.ResetTimer()
	for i := range trees {
		InternStatics(trees[i])
	}
}
