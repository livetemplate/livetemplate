package livetemplate

import (
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Client-attribute census.
//
// The server advertises which lvt-* attribute names appear in a template so the
// client can diff them against its handler registry and warn once, on first
// render, for any attribute nothing handles. That warning is the safety net for
// an attribute that would otherwise be silently inert — a typo, or a handler
// bundle the page forgot to load.
//
// This does NOT make the server interpret attributes. It collects names for a
// developer diagnostic and throws away everything else; nothing downstream
// branches on the result and the template renders byte-identically whether or
// not the census ran. Same posture as extractWiredActionNames, which already
// regex-scans statics for lvt-on: names to power a dev-time collision warning.
//
// Why the server does this rather than the client scanning the DOM: a DOM scan
// only sees attributes that have already rendered, so an lvt-fx:text-select
// inside an untaken {{if}} branch stays invisible until a user reaches it. The
// server sees the whole template, including unrendered branches, so the warning
// fires on the first render — which is exactly the case a missing handler bundle
// produces.
//
// The census is deliberately COMPLETE, not filtered: every lvt-* attribute name
// is reported, including core routing ones (lvt-on:, lvt-mod:, lvt-nav:). The
// server has no registry and no business deciding what "handled" means; the
// client owns that half. A client can always ignore a name it knows about, but
// it cannot recover one the server dropped.

var (
	// Any HTML start tag, capturing its attribute region. Mirrors the tag-scan
	// discipline of inputAttrRegex / buttonElemRegex in formvalidation.go, and
	// shares their known limitation: a `>` inside an attribute value ends the
	// region early. The remainder is then scanned as text rather than as a tag,
	// so the failure mode is a missed name, never a spurious one.
	//
	// Tag scoping is what keeps CSS class values (class="lvt-fade-in lvt-modal")
	// and wrapper IDs (data-lvt-id="lvt-06af684d6b9c7483") out of the census:
	// they live inside quoted values, and attrTokenRegex consumes values whole.
	htmlTagRegex = regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9-]*\b([^>]*)>`)

	// One attribute of a tag's attribute region: a name, optionally followed by
	// =value in double-quoted, single-quoted, or unquoted form. Matched
	// left-to-right and non-overlapping, so a value is consumed as part of its
	// own attribute and never rescanned as a name — the value is deliberately
	// not captured, only swallowed. Unlike attrRegex in formvalidation.go, the
	// name class admits ':' — lvt-fx:scroll and lvt-el:addClass:on:click are
	// single attribute names, not name + suffix.
	attrTokenRegex = regexp.MustCompile(`([^\s=/<>]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]*))?`)
)

// lvtAttributePrefix is the namespace every framework and custom client
// attribute shares.
//
// Names are reported in the case the template author wrote them, which makes
// case-insensitive comparison an obligation on the consumer, not an option.
// The DOM cannot preserve it: HTML parsers ASCII-lowercase attribute names, so
// an authored lvt-el:addClass reaches JavaScript only as lvt-el:addclass (the
// client already lowercases before matching). Reporting the author's spelling
// anyway is the point — a warning naming lvt-el:addclass sends the developer
// grepping for a string that appears nowhere in their template.
//
// data-lvt-* is deliberately NOT in the namespace. Those (data-lvt-id,
// data-lvt-loading, data-lvt-target, data-lvt-redact, data-lvt-toast-stack) are
// markers the framework emits for itself, not extension points an author writes
// and a handler claims, so censusing them could only ever produce a warning
// about the framework's own output.
const lvtAttributePrefix = "lvt-"

// lifecycleSuffixSeparator splits an attribute name from its action/state
// suffix. lvt-fx:animate:on:save:pending and lvt-el:addClass:on:click both
// reduce to the part before ":on:" — the handler-identifying name. Collapsing
// them matters because bracket expansion (internal/render/bracket_expand.go)
// turns one authored lvt-fx:animate:on:[save,delete]:pending into two
// attributes, and a census that reported both would grow with the action list
// while naming the same handler twice.
//
// The separator is unambiguous: lvt-on:click contains "-on:", not ":on:", so
// event-routing attributes are never truncated.
const lifecycleSuffixSeparator = ":on:"

// extractAttributeNames returns the sorted, deduplicated set of lvt-* attribute
// names appearing in attribute position in templateStr, each reduced to its
// handler-identifying form (see lifecycleSuffixSeparator).
//
// {{…}} directives are stripped first, mirroring extractWiredActionNames and
// extractFormSchemaFromTemplateStr. Returns nil when the template uses none, so
// the wire field stays absent under omitempty.
//
// Two limitations, both benign and both worth stating so the warning's absence
// is never read as proof of correctness:
//
//  1. Dynamic attribute names. lvt-fx:{{.Kind}} collapses to "lvt-fx:" under the
//     strip and is discarded, exactly as name="{{.X}}" already is for
//     wired-action extraction. An app building attribute names at render time
//     gets no census entry and therefore no warning.
//  2. Attributes only ever added client-side. The census reads the template, so
//     an attribute a script sets via setAttribute is invisible to it. That is
//     the right side to err on — a handler registered for it is not a mistake.
//
// Not a limitation: associated templates. parseInternal flattens
// {{define}}/{{template}}/{{block}} into templateStr before this runs, so an
// attribute inside an associated template is censused at its call site.
func extractAttributeNames(templateStr string) []string {
	if !strings.Contains(templateStr, lvtAttributePrefix) {
		return nil
	}

	stripped := templateDirectiveRegex.ReplaceAllString(templateStr, "")
	seen := make(map[string]struct{})

	for _, tag := range htmlTagRegex.FindAllStringSubmatch(stripped, -1) {
		// Most tags carry no lvt-* attribute, and tokenizing one allocates a
		// slice per attribute. Any lvt-* name contains the prefix, so a region
		// without it cannot yield a name.
		if !strings.Contains(tag[1], lvtAttributePrefix) {
			continue
		}
		for _, attr := range attrTokenRegex.FindAllStringSubmatch(tag[1], -1) {
			if name := normalizeAttributeName(attr[1]); name != "" {
				seen[name] = struct{}{}
			}
		}
	}

	// Sorted so the wire field is deterministic: Go map iteration order is
	// randomized per process, and an unsorted array would make every response
	// differ from the last for no reason and break any golden that pins meta.
	// Returns nil for an empty map, which is what keeps omitempty working.
	return slices.Sorted(maps.Keys(seen))
}

// normalizeAttributeName reduces one raw attribute name to its census form, or
// returns "" if it is not an lvt-* attribute the client could have a handler for.
func normalizeAttributeName(raw string) string {
	if !strings.HasPrefix(raw, lvtAttributePrefix) {
		return ""
	}
	raw, _, _ = strings.Cut(raw, lifecycleSuffixSeparator)
	// A bare namespace is what a dynamic name collapses to — limitation 1 above.
	if raw == lvtAttributePrefix || strings.HasSuffix(raw, ":") {
		return ""
	}
	return raw
}
