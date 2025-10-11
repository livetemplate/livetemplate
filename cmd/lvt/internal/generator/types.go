package generator

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed templates/*
var templatesFS embed.FS

type ResourceData struct {
	PackageName          string
	ModuleName           string
	ResourceName         string // Input name, capitalized (e.g., "Users" or "User")
	ResourceNameLower    string // Input name, lowercase (e.g., "users" or "user")
	ResourceNameSingular string // Singular, capitalized (e.g., "User")
	ResourceNamePlural   string // Plural, capitalized (e.g., "Users")
	TableName            string // Plural table name (e.g., "users")
	Fields               []FieldData
	CSSFramework         string // CSS framework: "tailwind", "bulma", "pico", "none"
	DevMode              bool   // Use local client library instead of CDN
	PaginationMode       string // Pagination mode: "infinite", "load-more", "prev-next", "numbers"
	PageSize             int    // Page size for pagination
}

type FieldData struct {
	Name            string
	GoType          string
	SQLType         string
	IsReference     bool
	ReferencedTable string
	OnDelete        string
	IsTextarea      bool // true if field should render as textarea
}

type AppData struct {
	AppName      string
	ModuleName   string
	DevMode      bool   // Use local client library instead of CDN
	CSSFramework string // CSS framework for home page
}

var funcMap = template.FuncMap{
	"title":        strings.Title,
	"lower":        strings.ToLower,
	"upper":        strings.ToUpper,
	"camelCase":    toCamelCase,
	"displayField": getDisplayField,
}

// toCamelCase converts snake_case to CamelCase following Go conventions
// Common initialisms like ID, URL, HTTP are kept in all caps
func toCamelCase(s string) string {
	// Common initialisms that should be all caps
	initialisms := map[string]bool{
		"id": true, "url": true, "http": true, "https": true,
		"api": true, "uri": true, "sql": true, "json": true,
		"xml": true, "html": true, "css": true, "js": true,
	}

	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			lower := strings.ToLower(part)
			if initialisms[lower] {
				parts[i] = strings.ToUpper(part)
			} else {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
	}
	return strings.Join(parts, "")
}

// getDisplayField returns the primary display field from a list of fields
// Priority: title > name > id > first field
func getDisplayField(fields []FieldData) FieldData {
	if len(fields) == 0 {
		return FieldData{Name: "id", GoType: "string"}
	}

	// Check for "title" field first
	for _, field := range fields {
		if strings.ToLower(field.Name) == "title" {
			return field
		}
	}

	// Check for "name" field second
	for _, field := range fields {
		if strings.ToLower(field.Name) == "name" {
			return field
		}
	}

	// Check for "id" field third
	for _, field := range fields {
		if strings.ToLower(field.Name) == "id" {
			return field
		}
	}

	// Default to first field
	return fields[0]
}
