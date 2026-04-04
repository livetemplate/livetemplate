package render

import (
	"regexp"
	"strings"
)

// bracketAttrPattern matches lvt-{el|fx|form}:*:on:[actions]:state="value" attributes.
// It captures the prefix before the bracket, the comma-separated actions inside brackets,
// and the suffix after the bracket (state + optional ="value").
//
// Lifecycle states (pending, success, error, done) are the four core states defined by
// the client protocol. If new states are added, update this regex to match.
//
// Known limitation: unquoted attribute values are not matched.
// The +? quantifier requires at least one character in the method segment,
// rejecting malformed patterns like lvt-el::on:[save]:pending.
var bracketAttrPattern = regexp.MustCompile(
	`(lvt-(?:el|fx|form):[^=\s]+?:on:)\[([^\]]+)\](:(?:pending|success|error|done))` +
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
// This handles lvt-el:*, lvt-fx:*, and lvt-form:* prefixes.
// Attributes without brackets pass through unchanged.
//
// Called at template parse time (in parseInternal) so both the HTTP response path
// and the WebSocket tree path receive expanded attributes. Bracket syntax inside
// dynamic template blocks ({{range}}, {{if}}) is correctly expanded since the
// source is transformed before Go's template parser processes it.
//
// Note: expansion operates on raw template source text, not parsed HTML. This means
// bracket patterns inside <script> or <style> content will also be expanded if they
// match the regex. In practice, the lvt-el:/lvt-fx:/lvt-form: prefixes are specific
// enough that false matches are unlikely outside HTML attribute contexts.
func ExpandBracketAttributes(html string) string {
	// Fast path: skip regex when no bracket syntax is present.
	if !strings.Contains(html, ":on:[") {
		return html
	}

	return bracketAttrPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := bracketAttrPattern.FindStringSubmatch(match)
		// unreachable in practice; guards against future regex changes
		if len(parts) < 5 {
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
