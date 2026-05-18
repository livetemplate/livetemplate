package livetemplate

import (
	"regexp"
	"strings"
)

// Client-wired-action extraction.
//
// Spec deviation, pinned here (proposal §"Critical files" predicted an
// internal/parse/ node walk): lvt-on: / lvt-* action attributes are
// client-side only and never enter the Go parse AST, and button name= surfaces
// only via the runtime form-submission heuristic — none are tracked as parse
// nodes. So the wired-action set is a regex scan over the template statics,
// parallel to ExtractFormSchema (same input, same {{…}}-strip discipline). The
// V19-pinned set is exactly: a name= on a submit-capable <button> or
// <input type=submit|image>, and the action token of an lvt-on:<event>="X"
// attribute. Widening this set is a deliberate, reviewable change to this one
// function (and its test), not a moving target.

var (
	buttonElemRegex = regexp.MustCompile(`(?is)<button\b([^>]*)>`)
	inputElemRegex  = regexp.MustCompile(`(?is)<input\b([^>]*)>`)
	// lvt-on:<event>="value" — event is letters/hyphens, value captured raw.
	lvtOnAttrRegex = regexp.MustCompile(`(?is)\blvt-on:[a-zA-Z][\w-]*\s*=\s*"([^"]*)"`)
	// Leading method identifier of an lvt-on value ("Toggle('x')" → "Toggle").
	actionIdentRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
)

// extractWiredActionNames returns the set of action names wired to a client
// element in templateStr. {{…}} directives are stripped first (mirrors
// extractFormSchemaFromTemplateStr) so a dynamic name="{{.X}}" / lvt-on
// value collapses to empty and is skipped rather than recorded as a literal.
// Returns nil when nothing is wired (the accessor treats nil as "no match").
func extractWiredActionNames(templateStr string) map[string]struct{} {
	stripped := templateDirectiveRegex.ReplaceAllString(templateStr, "")
	set := make(map[string]struct{})

	// Submit-capable <button name="X">. A <button> defaults to type=submit;
	// only an explicit type=button / type=reset is non-submitting.
	for _, m := range buttonElemRegex.FindAllStringSubmatch(stripped, -1) {
		attrs := parseHTMLAttributes(m[1])
		switch strings.ToLower(attrs["type"]) {
		case "button", "reset":
			continue
		}
		if name := attrs["name"]; name != "" {
			set[name] = struct{}{}
		}
	}

	// <input type="submit"|"image" name="X"> (the only submitting input types).
	for _, m := range inputElemRegex.FindAllStringSubmatch(stripped, -1) {
		attrs := parseHTMLAttributes(m[1])
		switch strings.ToLower(attrs["type"]) {
		case "submit", "image":
			if name := attrs["name"]; name != "" {
				set[name] = struct{}{}
			}
		}
	}

	// lvt-on:<event>="Action" on any element.
	for _, m := range lvtOnAttrRegex.FindAllStringSubmatch(stripped, -1) {
		if ident := actionIdentRegex.FindString(strings.TrimSpace(m[1])); ident != "" {
			set[ident] = struct{}{}
		}
	}

	if len(set) == 0 {
		return nil
	}
	return set
}

// isClientWiredAction reports whether action is wired to a client element in
// this template. Used by ctx.Publish for the symmetry-collision warning. The
// set is immutable after parse (same contract as formSchema), so no lock.
func (t *Template) isClientWiredAction(action string) bool {
	if t == nil || t.wiredActions == nil {
		return false
	}
	_, ok := t.wiredActions[action]
	return ok
}
