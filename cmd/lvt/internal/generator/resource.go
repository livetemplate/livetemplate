package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

func GenerateResource(basePath, moduleName, resourceName string, fields []parser.Field, cssFramework, appMode string) error {
	// Defaults
	if cssFramework == "" {
		cssFramework = "tailwind"
	}
	if appMode == "" {
		appMode = "multi"
	}

	// Capitalize resource name and derive singular/plural forms
	resourceNameLower := strings.ToLower(resourceName)
	resourceName = strings.Title(resourceNameLower)

	// Derive singular and plural forms for struct/function names and table name
	resourceNameSingular := singularize(resourceNameLower)
	resourceNameSingularCap := strings.Title(resourceNameSingular)
	resourceNamePluralCap := strings.Title(pluralize(resourceNameSingular))
	tableName := pluralize(resourceNameSingular)

	// Convert parser.Field to FieldData
	var fieldData []FieldData
	for _, f := range fields {
		fieldData = append(fieldData, FieldData{
			Name:            f.Name,
			GoType:          f.GoType,
			SQLType:         f.SQLType,
			IsReference:     f.IsReference,
			ReferencedTable: f.ReferencedTable,
			OnDelete:        f.OnDelete,
		})
	}

	// Read dev mode setting from .lvtrc
	devMode := ReadDevMode(basePath)

	data := ResourceData{
		PackageName:          resourceNameLower,
		ModuleName:           moduleName,
		ResourceName:         resourceName,
		ResourceNameLower:    resourceNameLower,
		ResourceNameSingular: resourceNameSingularCap,
		ResourceNamePlural:   resourceNamePluralCap,
		TableName:            tableName,
		Fields:               fieldData,
		CSSFramework:         cssFramework,
		DevMode:              devMode,
	}

	// Create resource directory
	resourceDir := filepath.Join(basePath, "internal", "app", resourceNameLower)
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	// Initialize template loader for cascading template lookup
	loader := NewTemplateLoader()

	// Read templates using loader (checks custom templates first, falls back to embedded)
	handlerTmpl, err := loader.Load("resource/handler.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read handler template: %w", err)
	}

	// Load main template based on mode
	// With template flattening support, we can now use component-based templates
	var templateTmpl []byte
	if appMode == "multi" {
		// Load component-based template for multi-page apps
		// Template flattening will resolve all {{define}}/{{template}} constructs
		components := []string{
			"components/layout.tmpl",
			"components/form.tmpl",
			"components/table.tmpl",
			"components/pagination.tmpl",
			"components/search.tmpl",
			"components/stats.tmpl",
			"components/sort.tmpl",
			"resource/template_components.tmpl.tmpl",
		}

		var fullTemplate string
		for _, comp := range components {
			compTmpl, err := loader.Load(comp)
			if err != nil {
				return fmt.Errorf("failed to load component %s: %w", comp, err)
			}
			fullTemplate += string(compTmpl) + "\n\n"
		}
		templateTmpl = []byte(fullTemplate)
	} else {
		// Single mode - use simple template
		templateTmpl, err = loader.Load("resource/template.tmpl.tmpl")
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}
	}

	queriesTmpl, err := loader.Load("resource/queries.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read queries template: %w", err)
	}

	testTmpl, err := loader.Load("resource/test.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read test template: %w", err)
	}

	migrationTmpl, err := loader.Load("resource/migration.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read migration template: %w", err)
	}

	schemaTmpl, err := loader.Load("resource/schema.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read schema template: %w", err)
	}

	// Generate handler
	if err := generateFile(string(handlerTmpl), data, filepath.Join(resourceDir, resourceNameLower+".go")); err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}

	// Generate template
	if err := generateFile(string(templateTmpl), data, filepath.Join(resourceDir, resourceNameLower+".tmpl")); err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}

	// Generate migration file instead of appending to schema.sql
	dbDir := filepath.Join(basePath, "internal", "database")
	migrationsDir := filepath.Join(dbDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate unique timestamp for migration
	// Check if file exists and increment timestamp if needed to avoid conflicts
	timestamp := time.Now()
	migrationFilename := ""
	migrationPath := ""
	for {
		timestampStr := timestamp.Format("20060102150405")
		migrationFilename = fmt.Sprintf("%s_create_%s.sql", timestampStr, tableName)
		migrationPath = filepath.Join(migrationsDir, migrationFilename)

		// Check if any migration file exists with this timestamp prefix
		matches, _ := filepath.Glob(filepath.Join(migrationsDir, timestampStr+"_*.sql"))
		if len(matches) == 0 {
			break
		}

		// Increment by 1 second and try again
		timestamp = timestamp.Add(1 * time.Second)
	}
	if err := generateFile(string(migrationTmpl), data, migrationPath); err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	// Also append to schema.sql for sqlc
	if err := appendToFile(string(schemaTmpl), data, filepath.Join(dbDir, "schema.sql"), "\n"); err != nil {
		return fmt.Errorf("failed to append to schema: %w", err)
	}

	// Append to queries.sql
	if err := appendToFile(string(queriesTmpl), data, filepath.Join(dbDir, "queries.sql"), "\n"); err != nil {
		return fmt.Errorf("failed to append to queries: %w", err)
	}

	// Generate consolidated test file (E2E + WebSocket)
	if err := generateFile(string(testTmpl), data, filepath.Join(resourceDir, resourceNameLower+"_test.go")); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}

	// Inject router registration into main.go
	mainGoPath := findMainGo(basePath)
	if mainGoPath != "" {
		route := RouteInfo{
			Path:        "/" + resourceNameLower,
			PackageName: resourceNameLower,
			HandlerCall: resourceNameLower + ".Handler(queries)",
			ImportPath:  moduleName + "/internal/app/" + resourceNameLower,
		}
		if err := InjectRoute(mainGoPath, route); err != nil {
			// Log warning but don't fail - user can add route manually
			fmt.Printf("⚠️  Could not auto-inject route: %v\n", err)
			fmt.Printf("   Please add manually: http.Handle(\"/%s\", %s.Handler(queries))\n",
				resourceNameLower, resourceNameLower)
		}
	}

	// Register resource for home page
	if err := RegisterResource(basePath, data.ResourceName, "/"+resourceNameLower, "resource"); err != nil {
		fmt.Printf("⚠️  Could not register resource in home page: %v\n", err)
	}

	return nil
}

func generateFile(tmplStr string, data interface{}, outPath string) error {
	// Merge base funcMap with CSS helpers
	funcs := make(template.FuncMap)
	for k, v := range funcMap {
		funcs[k] = v
	}
	for k, v := range CSSHelpers() {
		funcs[k] = v
	}

	// Use custom delimiters to avoid conflicts with Go template syntax in the generated files
	tmpl, err := template.New("template").Delims("[[", "]]").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func appendToFile(tmplStr string, data interface{}, outPath, separator string) error {
	// Use custom delimiters to avoid conflicts with Go template syntax in the generated files
	tmpl, err := template.New("template").Delims("[[", "]]").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Open file for appending (create if doesn't exist)
	f, err := os.OpenFile(outPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Write separator and content
	if _, err := f.WriteString(separator); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// findMainGo finds the main.go file in cmd/* directory
func findMainGo(basePath string) string {
	// Try to find cmd/*/main.go
	cmdDir := filepath.Join(basePath, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			mainGoPath := filepath.Join(cmdDir, entry.Name(), "main.go")
			if _, err := os.Stat(mainGoPath); err == nil {
				return mainGoPath
			}
		}
	}

	return ""
}

// singularize handles basic English singularization
func singularize(word string) string {
	// Common irregular plurals (reverse map)
	irregulars := map[string]string{
		"people":   "person",
		"children": "child",
		"teeth":    "tooth",
		"feet":     "foot",
		"men":      "man",
		"women":    "woman",
		"mice":     "mouse",
	}
	if singular, ok := irregulars[word]; ok {
		return singular
	}

	// Words ending in ies -> y (e.g., categories -> category)
	if strings.HasSuffix(word, "ies") && len(word) > 3 {
		return word[:len(word)-3] + "y"
	}

	// Words ending in ses, xes, zes -> remove es (e.g., boxes -> box)
	if strings.HasSuffix(word, "ses") || strings.HasSuffix(word, "xes") || strings.HasSuffix(word, "zes") {
		return word[:len(word)-2]
	}

	// Words ending in ches, shes -> remove es (e.g., watches -> watch)
	if strings.HasSuffix(word, "ches") || strings.HasSuffix(word, "shes") {
		return word[:len(word)-2]
	}

	// Words ending in s -> remove s (e.g., users -> user)
	if strings.HasSuffix(word, "s") && len(word) > 1 {
		return word[:len(word)-1]
	}

	// Already singular
	return word
}

// pluralize handles basic English pluralization rules
func pluralize(word string) string {
	// If already ends in 's', return as-is
	if strings.HasSuffix(word, "s") {
		return word
	}

	// Common irregular plurals
	irregulars := map[string]string{
		"person": "people",
		"child":  "children",
		"tooth":  "teeth",
		"foot":   "feet",
		"man":    "men",
		"woman":  "women",
		"mouse":  "mice",
	}
	if plural, ok := irregulars[word]; ok {
		return plural
	}

	// Words ending in consonant + y -> ies
	if len(word) >= 2 && word[len(word)-1] == 'y' {
		preceding := word[len(word)-2]
		if preceding != 'a' && preceding != 'e' && preceding != 'i' && preceding != 'o' && preceding != 'u' {
			return word[:len(word)-1] + "ies"
		}
	}

	// Words ending in s, x, z, ch, sh -> es
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") ||
		strings.HasSuffix(word, "ch") || strings.HasSuffix(word, "sh") {
		return word + "es"
	}

	// Default: just add s
	return word + "s"
}
