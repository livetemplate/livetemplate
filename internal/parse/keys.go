package parse

import (
	"fmt"
	"strings"

	"github.com/livetemplate/livetemplate/internal/keys"
)

// rangeItemWithStatics holds an item tree for range processing.
type rangeItemWithStatics struct {
	tree *TreeNode
}

// hasExplicitKeyAttribute checks if any key attribute is present in the statics.
func hasExplicitKeyAttribute(statics []string) bool {
	_, _, ok := findKeyAttr(statics)
	return ok
}

// detectIDKey detects which dynamic position contains the item ID
// by scanning statics for key attribute patterns.
// Returns the integer index of the dynamic position containing the key.
func detectIDKey(statics []string) int {
	i, _, _ := findKeyAttr(statics)
	return i
}

// findKeyAttr returns the first static holding a key attribute and the text
// after that attribute's opening quote. Within a static the earliest attribute
// wins, so data-key=" beats the key=" inside it.
func findKeyAttr(statics []string) (i int, rest string, ok bool) {
	for i, static := range statics {
		pos, n := -1, 0
		for _, attr := range keys.DefaultKeyAttributes {
			if p := strings.Index(static, attr); p != -1 && (pos == -1 || p < pos) {
				pos, n = p, len(attr)
			}
		}
		if pos != -1 {
			return i, static[pos+n:], true
		}
	}
	return 0, "", false
}

// compositeKey returns the whole key-attribute value when it is more than one
// lone string dynamic: a static prefix or suffix, several dynamics, or a nested
// {{if}}. Items whose first part repeats then keep distinct keys (#547).
// Otherwise ("", false), and callers keep the position key.
func compositeKey(node *TreeNode) (string, bool) {
	i, rest, ok := findKeyAttr(node.Statics)
	if !ok || strings.Contains(rest, `"`) || i >= len(node.Dynamics) || i+1 >= len(node.Statics) {
		return "", false
	}
	if _, isString := node.Dynamics[i].(string); isString && rest == "" && strings.HasPrefix(node.Statics[i+1], `"`) {
		return "", false
	}
	var b strings.Builder
	b.WriteString(rest)
	for j := i; j < len(node.Dynamics) && j+1 < len(node.Statics); j++ {
		if !writeKeyPart(&b, node.Dynamics[j]) {
			return "", false
		}
		next := node.Statics[j+1]
		if q := strings.IndexByte(next, '"'); q >= 0 {
			b.WriteString(next[:q])
			return b.String(), true
		}
		b.WriteString(next)
	}
	return "", false
}

// writeKeyPart appends a dynamic's text to a key: a string as is, a nested
// node (a conditional inside the value) as its statics and dynamics
// interleaved. A nested range can't be part of a key.
func writeKeyPart(b *strings.Builder, v interface{}) bool {
	switch d := v.(type) {
	case nil:
	case string:
		b.WriteString(d)
	case *TreeNode:
		if d.HasRange() {
			return false
		}
		for k, s := range d.Statics {
			b.WriteString(s)
			if k < len(d.Dynamics) && !writeKeyPart(b, d.Dynamics[k]) {
				return false
			}
		}
	default:
		fmt.Fprint(b, d)
	}
	return true
}

// generateItemHash creates a stable hash for a range item based on its content.
func generateItemHash(item *TreeNode) string {
	if item == nil {
		return ""
	}
	return keys.GenerateItemHashFromSlice(item.Dynamics)
}

// wrappedItemKey returns the explicit data-key of a range item whose real keyed
// element is hidden one level down inside a parser-created wrapper — the shape
// both a recursive {{template}} range and an {{if}}-wrapped keyed range produce,
// where the item's own statics are empty and the <li data-key> lives in the
// nested child. ({{with}} does not wrap: handleWith delegates straight to
// walkAST, so it only shows this shape when its body independently contains an
// {{if}} or {{template}}.) It reads the key value *through* the wrapper without
// restructuring the item, so the item keeps a stable identity across deep edits
// (the key is the item's path, not a content hash of its subtree).
//
// The wrapper is identified by the tag createWrapper set, not by its shape.
// Shape cannot decide it: `["", ""]` statics plus one nested child describes a
// conditional wrapper, an invocation wrapper and a plain field node holding a
// tree alike, so a structural test also matches nodes the parser never wrapped
// and would key them off a descendant's data-key by coincidence.
//
// Any wrapper kind qualifies, deliberately. The child's real data-key is the
// right identity for a conditional-wrapped item just as much as an invocation-
// wrapped one, and narrowing this to WrapperInvocation would drop {{if}}-wrapped
// keyed ranges back to content hashing — a keying regression, not a fix.
//
// Returns ("", false) when the item carries a key attribute itself (the ordinary
// keyed-range path handles that), is not a wrapper, holds more than one nested
// child, or has no keyed child within the descent limit — callers then fall back
// to content hashing.
func wrappedItemKey(item *TreeNode) (string, bool) {
	if item == nil || hasExplicitKeyAttribute(item.Statics) {
		return "", false
	}
	// Descend while the node is a wrapper we created, stopping at the first
	// child that carries a key attribute. Nested constructs stack wrappers —
	// {{if}}{{if}}<li data-key=…> puts the keyed element two levels down — and
	// looking only one level down leaves those items on content hashes despite
	// their having a perfectly good explicit key.
	//
	// Descending is safe only because wrappers are tagged: each step checks a
	// node the parser marked, never a shape that merely resembles one, so this
	// does not reintroduce guessing.
	node := item
	for depth := 0; depth < maxWrapperDescent; depth++ {
		if !node.Wrapper.IsWrapper() {
			return "", false
		}
		child := soleNestedChild(node)
		if child == nil {
			return "", false
		}
		if i, _, ok := findKeyAttr(child.Statics); ok {
			if k, ok := compositeKey(child); ok {
				return k, true
			}
			if v, ok := child.GetDynamic(i); ok {
				if s, ok := v.(string); ok && s != "" {
					return s, true
				}
			}
			return "", false
		}
		node = child
	}
	return "", false
}

// maxWrapperDescent bounds the search above. Every level costs a lookup per item
// per render, and allItemKeys is all-or-nothing, so an item that never
// yields a key makes the whole range pay the full descent each time. Four is
// well past what real templates stack — {{if}}{{if}} is already unusual — while
// keeping the miss cost flat.
const maxWrapperDescent = 4

// soleNestedChild returns the one nested *TreeNode among node's dynamics, or nil
// if there is none or more than one. More than one means the node holds real
// content rather than just wrapping a child, so there is no single element for
// the item's identity to come from.
func soleNestedChild(node *TreeNode) *TreeNode {
	var child *TreeNode
	for _, d := range node.Dynamics {
		c, ok := d.(*TreeNode)
		if !ok {
			continue
		}
		if child != nil {
			return nil
		}
		child = c
	}
	return child
}

// allItemKeys returns keyOf for every item, or (nil, false) if any item has
// none. It is all-or-nothing so a range never mixes key kinds. It probes the
// first item before allocating: most ranges fail there, so they don't pay for an
// N-length slice thrown away on every render.
func allItemKeys(items []rangeItemWithStatics, keyOf func(*TreeNode) (string, bool)) ([]string, bool) {
	if len(items) == 0 {
		return nil, false
	}
	first, ok := keyOf(items[0].tree)
	if !ok {
		return nil, false
	}
	out := make([]string, len(items))
	out[0] = first
	for i := 1; i < len(items); i++ {
		key, ok := keyOf(items[i].tree)
		if !ok {
			return nil, false
		}
		out[i] = key
	}
	return out, true
}
