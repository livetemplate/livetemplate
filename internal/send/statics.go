package send

import (
	"hash/maphash"
	"slices"
	"strings"
)

// InternStatics applies the statics table (tree-update spec §2.5) to one
// message in place. Indexes follow the arrays' content order, so the same tree
// always encodes the same way. The tree must be decoded JSON: a statics array is
// a []interface{} of strings, and any other "s" value is left as it is.
func InternStatics(tree map[string]interface{}) {
	type group struct {
		statics []interface{}
		nodes   []map[string]interface{}
	}
	groups := map[uint64][]*group{}
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch x := v.(type) {
		case map[string]interface{}:
			for k, child := range x {
				if k != "s" {
					walk(child)
					continue
				}
				s, ok := child.([]interface{})
				if !ok {
					continue
				}
				h, ok := hashStatics(s)
				if !ok {
					continue
				}
				i := slices.IndexFunc(groups[h], func(g *group) bool { return slices.Equal(g.statics, s) })
				if i < 0 {
					groups[h] = append(groups[h], &group{statics: s})
					i = len(groups[h]) - 1
				}
				groups[h][i].nodes = append(groups[h][i].nodes, x)
			}
		case []interface{}:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(tree)

	var repeated []*group
	for _, bucket := range groups {
		for _, g := range bucket {
			if len(g.nodes) > 1 {
				repeated = append(repeated, g)
			}
		}
	}
	if len(repeated) == 0 {
		return
	}
	slices.SortFunc(repeated, func(a, b *group) int { return compareStatics(a.statics, b.statics) })
	table := make([]interface{}, len(repeated))
	for i, g := range repeated {
		table[i] = g.statics
		for _, n := range g.nodes {
			n["s"] = i
		}
	}
	tree["S"] = table
}

var staticsSeed = maphash.MakeSeed()

// hashStatics hashes a statics array, length-prefixing each part so a
// different split of the same text hashes apart; false if a part isn't a string.
func hashStatics(s []interface{}) (uint64, bool) {
	var h maphash.Hash
	h.SetSeed(staticsSeed)
	for _, part := range s {
		str, ok := part.(string)
		if !ok {
			return 0, false
		}
		var n [8]byte
		for i, l := 0, len(str); i < 8; i, l = i+1, l>>8 {
			n[i] = byte(l)
		}
		_, _ = h.Write(n[:])
		_, _ = h.WriteString(str)
	}
	return h.Sum64(), true
}

// compareStatics orders statics arrays by content. Every part is a string,
// because hashStatics keeps any other array out of a group.
func compareStatics(a, b []interface{}) int {
	return slices.CompareFunc(a, b, func(x, y interface{}) int {
		return strings.Compare(x.(string), y.(string))
	})
}
