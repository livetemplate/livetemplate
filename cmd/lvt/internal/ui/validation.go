package ui

import (
	"regexp"
	"strings"
	"unicode"
)

// ValidationResult represents the result of input validation
type ValidationResult struct {
	Valid   bool
	Error   string
	Warning string
}

// Go reserved keywords
var goReservedKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

// SQL reserved words (common subset)
var sqlReservedWords = map[string]bool{
	"select": true, "insert": true, "update": true, "delete": true, "from": true,
	"where": true, "join": true, "left": true, "right": true, "inner": true,
	"outer": true, "on": true, "as": true, "table": true, "index": true,
	"create": true, "drop": true, "alter": true, "add": true, "column": true,
	"primary": true, "foreign": true, "key": true, "constraint": true,
	"order": true, "group": true, "having": true, "limit": true, "offset": true,
}

// Go identifier pattern: starts with letter or underscore, followed by letters, digits, or underscores
var goIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// IsValidGoIdentifier validates if a string is a valid Go identifier
func IsValidGoIdentifier(name string) (bool, string) {
	if name == "" {
		return false, "identifier cannot be empty"
	}

	if len(name) < 2 {
		return false, "identifier must be at least 2 characters"
	}

	if len(name) > 50 {
		return false, "identifier must be at most 50 characters"
	}

	if !goIdentifierPattern.MatchString(name) {
		if unicode.IsDigit(rune(name[0])) {
			return false, "identifier cannot start with a digit"
		}
		return false, "identifier can only contain letters, digits, and underscores"
	}

	if goReservedKeywords[name] {
		return false, "'" + name + "' is a Go reserved keyword"
	}

	return true, ""
}

// IsValidResourceName validates resource name (should be plural, lowercase)
func IsValidResourceName(name string) ValidationResult {
	result := ValidationResult{Valid: true}

	if name == "" {
		result.Valid = false
		result.Error = "resource name cannot be empty"
		return result
	}

	// Convert to lowercase for validation
	lower := strings.ToLower(name)

	valid, err := IsValidGoIdentifier(lower)
	if !valid {
		result.Valid = false
		result.Error = err

		// If it's a Go keyword that's also SQL, mention SQL in warning
		if goReservedKeywords[lower] && sqlReservedWords[lower] {
			result.Warning = "also a SQL reserved word"
		}
		return result
	}

	// Warn about SQL reserved words
	if sqlReservedWords[lower] {
		result.Warning = "'" + lower + "' is a SQL reserved word - consider renaming"
	}

	// Warn if contains uppercase (will be lowercased)
	if name != lower {
		result.Warning = "resource name will be converted to lowercase: '" + lower + "'"
	}

	return result
}

// IsValidFieldName validates field name (lowercase, snake_case allowed)
func IsValidFieldName(name string) ValidationResult {
	result := ValidationResult{Valid: true}

	if name == "" {
		result.Valid = false
		result.Error = "field name cannot be empty"
		return result
	}

	// Convert to lowercase for validation
	lower := strings.ToLower(name)

	valid, err := IsValidGoIdentifier(lower)
	if !valid {
		result.Valid = false
		result.Error = err
		return result
	}

	// Warn about SQL reserved words
	if sqlReservedWords[lower] {
		result.Warning = "'" + lower + "' is a SQL reserved word - may cause issues"
	}

	// Warn if contains uppercase (will be lowercased)
	if name != lower {
		result.Warning = "field name will be converted to lowercase: '" + lower + "'"
	}

	return result
}

// IsValidViewName validates view name
func IsValidViewName(name string) ValidationResult {
	result := ValidationResult{Valid: true}

	if name == "" {
		result.Valid = false
		result.Error = "view name cannot be empty"
		return result
	}

	// Convert to lowercase for validation
	lower := strings.ToLower(name)

	valid, err := IsValidGoIdentifier(lower)
	if !valid {
		result.Valid = false
		result.Error = err

		// If it's a Go keyword that's also SQL, mention SQL in warning
		if goReservedKeywords[lower] && sqlReservedWords[lower] {
			result.Warning = "also a SQL reserved word"
		}
		return result
	}

	// Warn about SQL reserved words
	if sqlReservedWords[lower] {
		result.Warning = "'" + lower + "' is a SQL reserved word - consider renaming"
	}

	// Warn if contains uppercase (will be lowercased)
	if name != lower {
		result.Warning = "view name will be converted to lowercase: '" + lower + "'"
	}

	return result
}

// IsValidModulePath validates Go module path
func IsValidModulePath(path string) ValidationResult {
	result := ValidationResult{Valid: true}

	if path == "" {
		result.Valid = false
		result.Error = "module path cannot be empty"
		return result
	}

	// Basic validation: should contain at least one /
	if !strings.Contains(path, "/") {
		result.Valid = false
		result.Error = "module path should be in format: domain.com/user/repo"
		return result
	}

	// Check for common patterns
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		result.Valid = false
		result.Error = "module path should not include http:// or https://"
		return result
	}

	// Warn about example.com
	if strings.HasPrefix(path, "example.com/") {
		result.Warning = "using example.com - consider using actual domain"
	}

	return result
}

// IsValidAppName validates app name
func IsValidAppName(name string) ValidationResult {
	result := ValidationResult{Valid: true}

	if name == "" {
		result.Valid = false
		result.Error = "app name cannot be empty"
		return result
	}

	// App names can contain hyphens for directory names
	// But warn if not a valid Go identifier
	normalized := strings.ReplaceAll(name, "-", "_")
	valid, err := IsValidGoIdentifier(normalized)
	if !valid {
		result.Valid = false
		result.Error = "app name must contain only letters, digits, hyphens, and underscores"
		if err != "" {
			result.Error = err
		}
		return result
	}

	// Warn if contains hyphens
	if strings.Contains(name, "-") {
		result.Warning = "app name contains hyphens (directory will use hyphens, package will use underscores)"
	}

	return result
}
