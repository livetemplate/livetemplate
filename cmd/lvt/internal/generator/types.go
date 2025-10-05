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
}

type FieldData struct {
	Name            string
	GoType          string
	SQLType         string
	IsReference     bool
	ReferencedTable string
	OnDelete        string
}

type AppData struct {
	AppName    string
	ModuleName string
}

var funcMap = template.FuncMap{
	"title":     strings.Title,
	"lower":     strings.ToLower,
	"upper":     strings.ToUpper,
	"camelCase": toCamelCase,
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
