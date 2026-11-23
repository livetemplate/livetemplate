package send

import (
	"sort"
	"strconv"
	"strings"
)

// InternStatics applies the tree-update specification's statics table to one
// message: every "s" array that occurs at least twice in tree is sent once, in
// a top-level "S" table, and each occurrence is replaced by its index into it.
// A range whose items are wrapped in a conditional repeats the same row
// statics in every item; this sends them once.
//
// The table belongs to this message only, so the client keeps nothing across
// updates. A message whose statics never repeat is left unchanged (no "S").
// Indexes follow a deterministic walk (sorted map keys), so the same tree
// always produces the same output. Only "s" keys are interned; the positional
// statics of an insert op are left as they are.
func InternStatics(tree map[string]interface{}) {
	if tree == nil {
		return
	}
	type occurrence struct {
		node map[string]interface{}
		key  string
	}
	var (
		occurrences []occurrence
		order       []string
		counts      = map[string]int{}
		arrays      = map[string][]interface{}{}
	)
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch x := v.(type) {
		case map[string]interface{}:
			if s, ok := x["s"].([]interface{}); ok {
				if key, ok := staticsKey(s); ok {
					if counts[key] == 0 {
						order = append(order, key)
						arrays[key] = s
					}
					counts[key]++
					occurrences = append(occurrences, occurrence{node: x, key: key})
				}
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				if k != "s" {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k])
			}
		case []interface{}:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(tree)

	index := map[string]int{}
	var table []interface{}
	for _, key := range order {
		if counts[key] > 1 {
			index[key] = len(table)
			table = append(table, arrays[key])
		}
	}
	if len(table) == 0 {
		return
	}
	for _, o := range occurrences {
		if i, ok := index[o.key]; ok {
			o.node["s"] = i
		}
	}
	tree["S"] = table
}

// staticsKey is an exact identity for a statics array (length-prefixed parts),
// and false when the array isn't all strings.
func staticsKey(s []interface{}) (string, bool) {
	var b strings.Builder
	for _, part := range s {
		str, ok := part.(string)
		if !ok {
			return "", false
		}
		b.WriteString(strconv.Itoa(len(str)))
		b.WriteByte(':')
		b.WriteString(str)
	}
	return b.String(), true
}
