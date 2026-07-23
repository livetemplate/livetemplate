package livetemplate

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing/fstest"
)

// Severity classifies a Diagnostic.
type Severity int

const (
	// SeverityError marks a problem that prevents a template from being served:
	// a block that fails to parse is dropped at serve time and renders nothing.
	SeverityError Severity = iota
	// SeverityWarning is reserved for problems that degrade a template rather
	// than break it (the home for data-dependent checks a future sample-data
	// mode would surface — see the Validate doc).
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is one problem Validate found in a template. Line is the 1-based
// line in the supplied text (0 when the underlying parser did not report one).
//
// The shape is deliberately minimal: html/template's parse errors carry a line
// but no column, and the syntax-layer checks have no hint to offer, so neither
// field is included rather than shipped always-empty. A column and a hint would
// return as appended fields — so backward-compatibly — alongside the future
// sample-data mode, which builds on the reactive parser's positional errors
// (see Validate).
type Diagnostic struct {
	Line     int
	Severity Severity
	Message  string
}

type validateConfig struct {
	components []*TemplateSet
}

// ValidateOption configures Validate.
type ValidateOption func(*validateConfig)

// WithValidateComponents makes the given component template sets available to
// Validate, so a template that invokes {{template "ns:name" .}} resolves the
// same way it does at serve time. Pass the same sets you pass to
// New(WithComponentTemplates(...)); without them, a component reference is
// (correctly) reported as an unresolved template.
func WithValidateComponents(sets ...*TemplateSet) ValidateOption {
	return func(c *validateConfig) { c.components = append(c.components, sets...) }
}

const (
	validateName = "__lvt_validate__"
	validateFile = "__lvt_validate__.tmpl"
)

// Validate parses templateText the way the live renderer does — against the
// framework's real function set and any supplied component templates — and
// returns the problems found. An empty slice means the template parses cleanly
// and will be served rather than silently dropped.
//
// At most one Diagnostic is returned per call today: the underlying parser stops
// at the first error rather than recovering to find more, so the slice is length
// 0 or 1. The slice shape anticipates the multi-error reporting a future
// recovering pass could add.
//
// The returned error is reserved for infrastructure failures (a component set
// that itself fails to parse, an internal fault): a template that does not parse
// is always reported as a Diagnostic, never as an error. This mirrors the shape
// of a linter — problems in the input are data, not errors.
//
// Validate today checks the syntax/composition layer: unclosed or malformed
// actions ({{range}}, {{if}}, {{with}}), unknown functions (checked against the
// framework's real builtins, which a downstream caller cannot enumerate), and
// unresolved component/composition templates. Data-dependent problems that only
// surface when the template is executed against a value are out of scope until a
// sample-data mode is added (SeverityWarning is reserved for them).
//
// Each call re-parses the supplied component sets from their fs.FS, which suits
// pre-serve / dev-time validation rather than per-keystroke use against a large
// component library. Line numbers refer to the supplied text; HTML comments are
// stripped before parsing (matching serve), so a diagnostic below a multi-line
// HTML comment can report a line shifted by the comment's height — uncommon in
// generated blocks.
func Validate(templateText string, opts ...ValidateOption) ([]Diagnostic, error) {
	var cfg validateConfig
	for _, o := range opts {
		o(&cfg)
	}

	// Parse the text as an in-memory "file" through the same ParseFS path serve
	// uses (ParseFiles), which clones the component-bearing template so component
	// definitions resolve — the .Parse method would drop them.
	memFS := fstest.MapFS{validateFile: {Data: []byte(templateText)}}
	newOpts := []Option{
		WithComponentTemplates(cfg.components...),
		WithParseFS(memFS, validateFile),
	}

	if _, err := New(validateName, newOpts...); err != nil {
		// A failure to parse the caller's component SET is the caller's problem,
		// reported as an infrastructure error. Component templates parse before
		// the supplied text (New parses them first), so this wrapper is present
		// only for a bad set — never for a problem in templateText.
		if isComponentSetError(err) {
			return nil, fmt.Errorf("livetemplate.Validate: %w", err)
		}
		// Everything else is a failure to parse the supplied text: always a
		// Diagnostic, whatever shape it takes (a stdlib parse error, an
		// unresolved {{template}} composition, a wrapper-injection failure).
		return []Diagnostic{diagnosticFromError(err)}, nil
	}
	return nil, nil
}

func isComponentSetError(err error) bool {
	return strings.Contains(err.Error(), "failed to parse component templates")
}

// stdlibParseLoc matches the "template: NAME:LINE:" prefix html/template puts on
// a parse error, capturing the line. html/template does not report a column, so
// a "NAME:LINE:COL:" form (should a future Go emit one) is tolerated but the
// column is discarded rather than captured.
var stdlibParseLoc = regexp.MustCompile(`template: [^:]*:(\d+)(?::\d+)?: `)

// diagnosticFromError turns a text-parse failure into a Diagnostic. When the
// underlying html/template parser reported a "NAME:LINE:" location, the line and
// the parser's own trailing message are used; otherwise (e.g. an unresolved
// {{template}} composition, which carries no location) the line is 0 and the
// innermost wrapped message is used.
func diagnosticFromError(err error) Diagnostic {
	msg := err.Error()
	if loc := stdlibParseLoc.FindStringSubmatchIndex(msg); loc != nil {
		line, _ := strconv.Atoi(msg[loc[2]:loc[3]])
		return Diagnostic{Line: line, Severity: SeverityError, Message: strings.TrimSpace(msg[loc[1]:])}
	}
	return Diagnostic{Severity: SeverityError, Message: innermostMessage(err)}
}

// innermostMessage unwraps err to its deepest cause and returns that message —
// the framework's own wording, stripped of the New/ParseFS wrapper layers.
func innermostMessage(err error) string {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err.Error()
		}
		err = next
	}
}
