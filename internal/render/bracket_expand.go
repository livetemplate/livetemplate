package render

import (
	"regexp"
	"strings"
)

// bracketAttrPattern matches lvt-{el|fx|form}:*:on:[actions]:state="value" attributes.
// It captures the prefix before the bracket, the comma-separated actions inside brackets,
// and the suffix after the bracket (state + optional ="value").
//
// Known limitation: unquoted attribute values are not matched.
var bracketAttrPattern = regexp.MustCompile(
	`(lvt-(?:el|fx|form):[^=\s]*?:on:)\[([^\]]+)\](:(?:pending|success|error|done))` +
		`(="[^"]*"|='[^']*')?`,
)

// ExpandBracketAttributes expands multi-action bracket syntax in rendered HTML.
// For example:
//
//	lvt-el:addClass:on:[save,delete]:pending="opacity-50"
//
// becomes two separate attributes:
//
//	lvt-el:addClass:on:save:pending="opacity-50" lvt-el:addClass:on:delete:pending="opacity-50"
//
// This handles lvt-el:*, lvt-fx:*, and lvt-form:* prefixes.
// Attributes without brackets pass through unchanged.
//
// NOTE: This runs only on the HTTP response path (renderHTML). The WebSocket tree
// path (buildTree/buildTreeWithCache) builds from the template AST, not rendered
// HTML. Bracket syntax inside dynamic elements ({{range}}, {{if}}) will be expanded
// on initial HTTP render but not in subsequent WebSocket diff updates. Use individual
// attributes (not bracket syntax) for elements inside dynamic template blocks.
func ExpandBracketAttributes(html string) string {
	// Fast path: skip regex when no bracket syntax is present.
	if !strings.Contains(html, ":on:[") {
		return html
	}

	return bracketAttrPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := bracketAttrPattern.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		prefix := parts[1]  // e.g. "lvt-el:addClass:on:"
		actions := parts[2] // e.g. "save,delete"
		suffix := parts[3]  // e.g. ":pending"
		value := parts[4]   // e.g. `="opacity-50"` or empty string

		actionList := strings.Split(actions, ",")
		validActions := make([]string, 0, len(actionList))
		for _, action := range actionList {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
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
