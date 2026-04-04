package render

import (
	"regexp"
	"strings"
)

// bracketAttrPattern matches lvt-{el|fx|form}:*:on:[actions]:state="value" attributes.
// It captures the prefix before the bracket, the comma-separated actions inside brackets,
// and the suffix after the bracket (state + optional ="value").
var bracketAttrPattern = regexp.MustCompile(
	`(lvt-(?:el|fx|form):[^=\s]*?:on:)\[([^\]]+)\](:(?:pending|success|error|done))` +
		`(="[^"]*")?`,
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
func ExpandBracketAttributes(html string) string {
	return bracketAttrPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := bracketAttrPattern.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}

		prefix := parts[1]  // e.g. "lvt-el:addClass:on:"
		actions := parts[2] // e.g. "save,delete"
		suffix := parts[3]  // e.g. ":pending"
		value := ""
		if len(parts) > 4 {
			value = parts[4] // e.g. `="opacity-50"` or empty
		}

		actionList := strings.Split(actions, ",")
		expanded := make([]string, len(actionList))
		for i, action := range actionList {
			action = strings.TrimSpace(action)
			expanded[i] = prefix + action + suffix + value
		}
		return strings.Join(expanded, " ")
	})
}
