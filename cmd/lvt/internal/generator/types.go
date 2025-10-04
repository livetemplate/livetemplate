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
	Name    string
	GoType  string
	SQLType string
}

type AppData struct {
	AppName    string
	ModuleName string
}

var funcMap = template.FuncMap{
	"title": strings.Title,
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
}
