package livetemplate

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FormRule represents a validation rule inferred from HTML input attributes.
// Optional numeric bounds use a paired Has* boolean to signal "set" (the zero
// value is a valid bound, so a sentinel won't do).
type FormRule struct {
	Field        string
	Required     bool
	InputType    string // "email", "url", "number", "tel"
	MinLength    int
	MaxLength    int
	Min          float64
	Max          float64
	HasMinLength bool
	HasMaxLength bool
	HasMin       bool
	HasMax       bool
	Pattern      string         // raw pattern string
	PatternRe    *regexp.Regexp // pre-compiled pattern (nil if invalid or absent)
}

// FormSchema holds validation rules inferred from template statics.
type FormSchema struct {
	Rules []FormRule
	// NoValidateSubmitters is the set of submit-control name attributes that
	// carry the formnovalidate attribute (e.g. <button name="save-draft"
	// formnovalidate>). ValidateForm skips validation when the form's submitter
	// matches one. Only named submitters are recorded; a dynamic ({{...}}) name
	// is blanked before extraction, so it is not detected.
	NoValidateSubmitters map[string]bool
}

// inputAttrRegex matches HTML input/textarea/select elements and captures their
// attributes. Case-insensitive: raw template source may carry developer-authored
// mixed-case tags (statics rendered by html/template are lowercase, but the
// public ExtractFormSchema also takes hand-written statics).
var inputAttrRegex = regexp.MustCompile(`(?is)<(?:input|textarea|select)\b([^>]*)>`)

// Only submit controls (<button>, <input>) can carry formnovalidate — unlike
// inputAttrRegex, this deliberately excludes <select> and <textarea>.
var submitControlRegex = regexp.MustCompile(`(?is)<(button|input)\b([^>]*)>`)

var templateDirectiveRegex = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// name attrs containing any directive are blanked so partial-dynamic forms don't collapse to spurious literal rules.
var dynamicNameAttrRegex = regexp.MustCompile(`(?s)(\bname\s*=\s*")[^"]*\{\{.*?\}\}[^"]*"`)

func extractFormSchemaFromTemplateStr(templateStr string) *FormSchema {
	blanked := dynamicNameAttrRegex.ReplaceAllString(templateStr, `${1}"`)
	stripped := templateDirectiveRegex.ReplaceAllString(blanked, "")
	schema := ExtractFormSchema([]string{stripped})
	if schema == nil || (len(schema.Rules) == 0 && len(schema.NoValidateSubmitters) == 0) {
		return nil
	}
	return schema
}

// attrRegex matches individual HTML attributes.
// Handles double-quoted values only. This is sufficient because Go's html/template
// always renders attributes with double quotes in statics.
var attrRegex = regexp.MustCompile(`(\w[\w-]*)(?:\s*=\s*"([^"]*)")?`)

// ExtractFormSchema scans template statics for HTML validation attributes
// on <input>, <textarea>, and <select> elements.
//
// Known limitation: if a field's name attribute is a template expression (dynamic),
// it will be split across statics and may not be detected.
func ExtractFormSchema(statics []string) *FormSchema {
	schema := &FormSchema{}
	fullHTML := strings.Join(statics, "")

	matches := inputAttrRegex.FindAllStringSubmatch(fullHTML, -1)
	for _, match := range matches {
		attrs := parseHTMLAttributes(match[1])

		name := attrs["name"]
		if name == "" {
			continue
		}

		// Skip non-data controls: submit/image/button/reset inputs carry no field
		// value, so a `required` on one would raise a spurious error for a field
		// that is never in the POST body unless it is the submitter.
		switch strings.ToLower(attrs["type"]) {
		case "submit", "image", "button", "reset":
			continue
		}

		rule := FormRule{Field: name}

		if _, ok := attrs["required"]; ok {
			rule.Required = true
		}

		if t, ok := attrs["type"]; ok {
			rule.InputType = strings.ToLower(t)
		}

		if v, ok := attrs["minlength"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				rule.MinLength = n
				rule.HasMinLength = true
			}
		}

		if v, ok := attrs["maxlength"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				rule.MaxLength = n
				rule.HasMaxLength = true
			}
		}

		if v, ok := attrs["min"]; ok {
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				rule.Min = n
				rule.HasMin = true
			}
		}

		if v, ok := attrs["max"]; ok {
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				rule.Max = n
				rule.HasMax = true
			}
		}

		if v, ok := attrs["pattern"]; ok {
			rule.Pattern = v
			// HTML pattern attribute implicitly anchors the full string (^...$)
			rule.PatternRe, _ = regexp.Compile("^(?:" + v + ")$")
		}

		if rule.Required || rule.InputType == "email" || rule.InputType == "url" ||
			rule.HasMinLength || rule.HasMaxLength || rule.HasMin || rule.HasMax || rule.Pattern != "" {
			schema.Rules = append(schema.Rules, rule)
		}
	}

	for _, match := range submitControlRegex.FindAllStringSubmatch(fullHTML, -1) {
		tag := strings.ToLower(match[1])
		attrs := parseHTMLAttributes(match[2])
		if _, ok := attrs["formnovalidate"]; !ok {
			continue
		}
		// formnovalidate only submits — and so only skips validation — on a real
		// submit control: a <button> (type defaults to submit) that is not
		// type=button/reset, or an <input type=submit|image>.
		typ := strings.ToLower(attrs["type"])
		isSubmit := (tag == "button" && typ != "button" && typ != "reset") ||
			(tag == "input" && (typ == "submit" || typ == "image"))
		if !isSubmit {
			continue
		}
		name := attrs["name"]
		if name == "" {
			continue
		}
		if schema.NoValidateSubmitters == nil {
			schema.NoValidateSubmitters = make(map[string]bool)
		}
		schema.NoValidateSubmitters[name] = true
	}

	return schema
}

func parseHTMLAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)
	matches := attrRegex.FindAllStringSubmatch(attrStr, -1)
	for _, m := range matches {
		key := strings.ToLower(m[1])
		val := m[2]
		attrs[key] = val
	}
	return attrs
}

// Validate checks form data against the schema rules.
// Returns MultiError with field-level errors, or nil if valid.
func (s *FormSchema) Validate(data map[string]interface{}) error {
	if s == nil || len(s.Rules) == 0 {
		return nil
	}

	var errs MultiError

	for _, rule := range s.Rules {
		val, exists := data[rule.Field]
		strVal := ""
		if exists {
			strVal = fmt.Sprintf("%v", val)
		}

		fieldName := formatFieldName(rule.Field)

		if rule.Required && (!exists || strVal == "") {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s is required", fieldName)})
			continue
		}

		if !exists || strVal == "" {
			continue
		}

		if rule.InputType == "email" {
			// HTML input[type=email] only accepts bare addr-spec (user@host), not display names.
			addr, err := mail.ParseAddress(strVal)
			if err != nil || addr.Address != strVal {
				errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be a valid email address", fieldName)})
			}
		}

		if rule.InputType == "url" {
			u, err := url.Parse(strVal)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be a valid URL", fieldName)})
			}
		}

		// Use rune count for minlength/maxlength (HTML counts Unicode code points, not bytes)
		if rule.HasMinLength && utf8.RuneCountInString(strVal) < rule.MinLength {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at least %d characters", fieldName, rule.MinLength)})
		}

		if rule.HasMaxLength && utf8.RuneCountInString(strVal) > rule.MaxLength {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at most %d characters", fieldName, rule.MaxLength)})
		}

		if rule.HasMin || rule.HasMax {
			if numVal, err := strconv.ParseFloat(strVal, 64); err == nil {
				if rule.HasMin && numVal < rule.Min {
					errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at least %g", fieldName, rule.Min)})
				}
				if rule.HasMax && numVal > rule.Max {
					errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at most %g", fieldName, rule.Max)})
				}
			}
		}

		if rule.PatternRe != nil && !rule.PatternRe.MatchString(strVal) {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s is invalid", fieldName)})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
