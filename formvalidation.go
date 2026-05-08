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
type FormRule struct {
	Field     string
	Required  bool
	InputType string // "email", "url", "number", "tel"
	MinLength int    // -1 if not set
	MaxLength int    // -1 if not set
	Min       float64
	Max       float64
	HasMin    bool
	HasMax    bool
	Pattern   string         // raw pattern string
	PatternRe *regexp.Regexp // pre-compiled pattern (nil if invalid or absent)
}

// FormSchema holds validation rules inferred from template statics.
type FormSchema struct {
	Rules []FormRule
}

// inputAttrRegex matches HTML input/textarea/select elements and captures their attributes.
var inputAttrRegex = regexp.MustCompile(`<(?:input|textarea|select)\b([^>]*)>`)

// templateDirectiveRegex matches Go template actions like {{...}} so we can strip
// them from the raw template source before form-attribute extraction. Dynamic
// attribute values (e.g. name="{{.Field}}") collapse to name="" and are then
// discarded by ExtractFormSchema, matching the documented limitation that
// dynamic field names are not auto-detected.
var templateDirectiveRegex = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

// dynamicNameAttrRegex matches a name attribute whose double-quoted value
// contains a {{...}} template directive (fully or partially dynamic). We blank
// the value before stripping directives so partially-dynamic names like
// name="user_{{.ID}}" do not collapse to a misleading literal (name="user_")
// and produce a wrong-field rule. Blanking forces ExtractFormSchema to skip
// the input entirely, matching the documented limitation that dynamic field
// names are not auto-detected.
var dynamicNameAttrRegex = regexp.MustCompile(`(?s)(\bname\s*=\s*")[^"]*\{\{.*?\}\}[^"]*"`)

// extractFormSchemaFromTemplateStr derives a FormSchema from the raw template
// source by stripping {{...}} directives and scanning the literal HTML for
// validation attributes. Returns nil if the template has no rules so callers
// can skip wiring entirely.
func extractFormSchemaFromTemplateStr(templateStr string) *FormSchema {
	// Pre-pass: blank any name attribute that contains a template directive
	// so partially-dynamic names are skipped instead of misdetected after the
	// global directive strip.
	blanked := dynamicNameAttrRegex.ReplaceAllString(templateStr, `${1}"`)
	stripped := templateDirectiveRegex.ReplaceAllString(blanked, "")
	schema := ExtractFormSchema([]string{stripped})
	if schema == nil || len(schema.Rules) == 0 {
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

		rule := FormRule{
			Field:     name,
			MinLength: -1,
			MaxLength: -1,
		}

		if _, ok := attrs["required"]; ok {
			rule.Required = true
		}

		if t, ok := attrs["type"]; ok {
			rule.InputType = strings.ToLower(t)
		}

		if v, ok := attrs["minlength"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				rule.MinLength = n
			}
		}

		if v, ok := attrs["maxlength"]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				rule.MaxLength = n
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
			rule.MinLength >= 0 || rule.MaxLength >= 0 || rule.HasMin || rule.HasMax || rule.Pattern != "" {
			schema.Rules = append(schema.Rules, rule)
		}
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
		if rule.MinLength >= 0 && utf8.RuneCountInString(strVal) < rule.MinLength {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at least %d characters", fieldName, rule.MinLength)})
		}

		if rule.MaxLength >= 0 && utf8.RuneCountInString(strVal) > rule.MaxLength {
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
