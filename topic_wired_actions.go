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
//
// Normalization (Phase 2 — closes the Phase 1 round-4 limitation): the
// dispatcher resolves a client name to a method via methodNameToActions
// (camelCase + snake_case + exact — dispatch.go), and every one of those forms
// reduces to the SAME snake_case key (toSnakeCase("AddItem") ==
// toSnakeCase("addItem") == toSnakeCase("add_item")). So snake_case is the
// canonical "could dispatch to the same method" key. Both the stored wired set
// and the ctx.Publish action argument are canonicalized through it, so a
// style-mismatched collision — <button name="save"> vs
// ctx.Publish(t, "Save", …), both resolving to Save() — is now caught. Still a
// best-effort dev warning, not a correctness gate; canonicalization only
// widens what it catches, it never produces a spurious warning for two names
// that could not dispatch to one method.

var (
	buttonElemRegex = regexp.MustCompile(`(?is)<button\b([^>]*)>`)
	inputElemRegex  = regexp.MustCompile(`(?is)<input\b([^>]*)>`)
	// lvt-on:<event>="value" — event is letters/hyphens, value captured raw.
	lvtOnAttrRegex = regexp.MustCompile(`(?is)\blvt-on:[a-zA-Z][\w-]*\s*=\s*"([^"]*)"`)
	// Leading method identifier of an lvt-on value ("Toggle('x')" → "Toggle").
	actionIdentRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
)

// canonicalActionKey reduces an action/wired name to the snake_case form the
// dispatcher's methodNameToActions collapses every style to, so two names that
// could dispatch to the same controller method share one key. It is the same
// toSnakeCase the dispatcher uses (dispatch.go) — normalization stays exactly
// consistent with real dispatch.
func canonicalActionKey(name string) string { return toSnakeCase(name) }

// extractWiredActionNames returns the set of action names wired to a client
// element in templateStr, each stored under its canonicalActionKey so a
// style-mismatched ctx.Publish still collides. {{…}} directives are stripped
// first (mirrors extractFormSchemaFromTemplateStr) so a dynamic name="{{.X}}" /
// lvt-on value collapses to empty and is skipped rather than recorded.
// Returns nil when nothing is wired (the accessor treats nil as "no match").
func extractWiredActionNames(templateStr string) map[string]struct{} {
	stripped := templateDirectiveRegex.ReplaceAllString(templateStr, "")
	set := make(map[string]struct{})
	add := func(name string) {
		if name != "" {
			set[canonicalActionKey(name)] = struct{}{}
		}
	}

	// Submit-capable <button name="X">. A <button> defaults to type=submit;
	// only an explicit type=button / type=reset is non-submitting.
	for _, m := range buttonElemRegex.FindAllStringSubmatch(stripped, -1) {
		attrs := parseHTMLAttributes(m[1])
		switch strings.ToLower(attrs["type"]) {
		case "button", "reset":
			continue
		}
		add(attrs["name"])
	}

	// <input type="submit"|"image" name="X"> (the only submitting input types).
	for _, m := range inputElemRegex.FindAllStringSubmatch(stripped, -1) {
		attrs := parseHTMLAttributes(m[1])
		switch strings.ToLower(attrs["type"]) {
		case "submit", "image":
			add(attrs["name"])
		}
	}

	// lvt-on:<event>="Action" on any element.
	for _, m := range lvtOnAttrRegex.FindAllStringSubmatch(stripped, -1) {
		if ident := actionIdentRegex.FindString(strings.TrimSpace(m[1])); ident != "" {
			add(ident)
		}
	}

	if len(set) == 0 {
		return nil
	}
	return set
}

// isClientWiredAction reports whether action is wired to a client element in
// this template, comparing by canonicalActionKey so a style mismatch (e.g.
// Publish "Save" vs <button name="save">) still matches. Used by ctx.Publish
// for the symmetry-collision warning. The set is immutable after parse (same
// contract as formSchema), so no lock.
func (t *Template) isClientWiredAction(action string) bool {
	if t == nil || t.wiredActions == nil {
		return false
	}
	_, ok := t.wiredActions[canonicalActionKey(action)]
	return ok
}

// shouldWarnWiredCollision reports whether ctx.Publish should emit the
// symmetry-collision warning for action: true iff action is client-wired AND
// this is the first Publish of it (per template, app-global — the dedup store
// is shared by pointer across per-session clones). Without the dedup the warn
// fires on every Publish, so a per-second feed publishing a wired action name
// would flood the log; once-per-action matches the "first time it happens in
// dev" signal the spec intends (proposal §"Design constraints").
func (t *Template) shouldWarnWiredCollision(action string) bool {
	if !t.isClientWiredAction(action) {
		return false
	}
	if t.wiredCollisionWarned == nil {
		return true // no dedup store (e.g. hand-built Template) — fail loud
	}
	// Key the dedup on the canonical form so "Save" then "save" (one logical
	// collision) warns once, not once per style.
	_, loaded := t.wiredCollisionWarned.LoadOrStore(canonicalActionKey(action), struct{}{})
	return !loaded
}
