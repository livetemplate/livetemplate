package render

import (
	"regexp"
	"strings"
)

// bracketAttrPattern matches lvt-<ns>:*:on:[actions]:state="value" attributes.
// It captures the prefix before the bracket, the comma-separated actions inside brackets,
// and the suffix after the bracket (state + optional ="value").
//
// The namespace is any lvt- prefix, not a fixed el|fx|form list, so an app-defined
// attribute gets the same bracket syntax the built-ins have —
// lvt-x:tooltip:on:[save,delete]:pending expands exactly like lvt-fx:animate does.
// Without that, custom attributes would be second-class purely because this regex
// enumerates the namespaces it knew about when it was written.
//
// Lifecycle states (pending, success, error, done) are the four core states defined by
// the client protocol. If new states are added, update this regex to match.
//
// Widening the namespace does mean lvt-on:, lvt-mod: and lvt-nav: now match in
// bracket context — measured, not assumed: lvt-on:click:on:[a,b]:pending is
// structurally a match, and RE2 has no lookahead to exclude it cheaply. That is
// harmless rather than a regression, because those three namespaces have no
// :on:[actions]:state form at all: such an attribute is already malformed, the
// client has no handler for either spelling, and expansion just turns one inert
// attribute into two. Excluding them would buy nothing and cost a special case
// that the next namespace would have to be added to.
//
// Known limitation: unquoted attribute values are not matched.
// The +? quantifier requires at least one character in the method segment,
// rejecting malformed patterns like lvt-el::on:[save]:pending.
var bracketAttrPattern = regexp.MustCompile(
	`(lvt-[a-zA-Z][a-zA-Z0-9-]*:[^=\s]+?:on:)\[([^\]]+)\](:(?:pending|success|error|done))` +
		`(="[^"]*"|='[^']*')?`,
)

// ExpandBracketAttributes expands multi-action bracket syntax in HTML or template source.
// For example:
//
//	lvt-el:addClass:on:[save,delete]:pending="opacity-50"
//
// becomes two separate attributes:
//
//	lvt-el:addClass:on:save:pending="opacity-50" lvt-el:addClass:on:delete:pending="opacity-50"
//
// Attributes without brackets pass through unchanged. See bracketAttrPattern for
// which attributes carry bracket syntax.
//
// Called at template parse time (in parseInternal) so both the HTTP response path
// and the WebSocket tree path receive expanded attributes. Bracket syntax inside
// dynamic template blocks ({{range}}, {{if}}) is correctly expanded since the
// source is transformed before Go's template parser processes it.
//
// Note: expansion operates on raw template source text, not parsed HTML. This means
// bracket patterns inside <script> or <style> content will also be expanded if they
// match the regex. In practice the full lvt-<ns>:<method>:on:[…]:<state> shape is
// specific enough that false matches are unlikely outside HTML attribute contexts;
// widening the namespace does not loosen that, since the anchor is the :on:[…]:state
// tail rather than the namespace.
//
// Template expressions inside bracket action lists (e.g. [{{.Action}},delete]) are
// detected and left unexpanded to avoid producing invalid attribute names.
func ExpandBracketAttributes(html string) string {
	// Fast path: skip regex when no bracket syntax is present.
	if !strings.Contains(html, ":on:[") {
		return html
	}

	return bracketAttrPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := bracketAttrPattern.FindStringSubmatch(match)
		// FindStringSubmatch returns nil (no match) or exactly 5 elements
		// (1 full match + 4 capture groups). ReplaceAllStringFunc only calls
		// us on matches, so parts is always 5 here.
		if len(parts) < 5 {
			return match
		}

		prefix := parts[1]  // e.g. "lvt-el:addClass:on:"
		actions := parts[2] // e.g. "save,delete"
		suffix := parts[3]  // e.g. ":pending"
		value := parts[4]   // e.g. `="opacity-50"`, `='opacity-50'`, or empty string

		actionList := strings.Split(actions, ",")
		validActions := make([]string, 0, len(actionList))
		for _, action := range actionList {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			// Template expressions (e.g. {{.Action}}) inside bracket lists would
			// produce invalid attribute names. Return the match unchanged.
			if strings.Contains(action, "{{") {
				return match
			}
			validActions = append(validActions, action)
		}
		if len(validActions) == 0 {
			return match
		}

		expanded := make([]string, len(validActions))
		for i, action := range validActions {
			expanded[i] = prefix + action + suffix + value
		}
		return strings.Join(expanded, " ")
	})
}
