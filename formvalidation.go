package livetemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	Pattern   string
}

// FormSchema holds validation rules inferred from template statics.
type FormSchema struct {
	Rules []FormRule
}

// inputAttrRegex matches HTML input/textarea/select elements and captures their attributes.
var inputAttrRegex = regexp.MustCompile(`<(?:input|textarea|select)\b([^>]*)>`)

// attrRegex matches individual HTML attributes (name="value" or bare attributes like "required").
var attrRegex = regexp.MustCompile(`(\w[\w-]*)(?:\s*=\s*"([^"]*)")?`)

// ExtractFormSchema scans template statics for HTML validation attributes
// on <input>, <textarea>, and <select> elements.
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
		}

		// Only add rules that have at least one validation attribute
		if rule.Required || rule.InputType == "email" || rule.InputType == "url" ||
			rule.MinLength >= 0 || rule.MaxLength >= 0 || rule.HasMin || rule.HasMax || rule.Pattern != "" {
			schema.Rules = append(schema.Rules, rule)
		}
	}

	return schema
}

// parseHTMLAttributes extracts attribute name-value pairs from an HTML element's attribute string.
func parseHTMLAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)
	matches := attrRegex.FindAllStringSubmatch(attrStr, -1)
	for _, m := range matches {
		key := strings.ToLower(m[1])
		val := m[2] // empty string for bare attributes like "required"
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

		if rule.InputType == "email" && !strings.Contains(strVal, "@") {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be a valid email address", fieldName)})
		}

		if rule.InputType == "url" && !strings.HasPrefix(strVal, "http") {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be a valid URL", fieldName)})
		}

		if rule.MinLength >= 0 && len(strVal) < rule.MinLength {
			errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s must be at least %d characters", fieldName, rule.MinLength)})
		}

		if rule.MaxLength >= 0 && len(strVal) > rule.MaxLength {
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

		if rule.Pattern != "" {
			re, err := regexp.Compile(rule.Pattern)
			if err == nil && !re.MatchString(strVal) {
				errs = append(errs, FieldError{Field: toSnakeCase(rule.Field), Message: fmt.Sprintf("%s is invalid", fieldName)})
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}
