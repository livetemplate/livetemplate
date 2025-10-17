package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

func GenerateResource(basePath, moduleName, resourceName string, fields []parser.Field, cssFramework, appMode, paginationMode string, pageSize int, editMode string) error {
	// Defaults
	if cssFramework == "" {
		cssFramework = "tailwind"
	}
	if appMode == "" {
		appMode = "multi"
	}
	if paginationMode == "" {
		paginationMode = "infinite"
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if editMode == "" {
		editMode = "modal"
	}

	// Load kit using KitLoader
	kitLoader := kits.DefaultLoader()
	kit, err := kitLoader.Load(cssFramework)
	if err != nil {
		return fmt.Errorf("failed to load kit %q: %w", cssFramework, err)
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
			IsTextarea:      f.IsTextarea,
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
		Kit:                  kit,
		CSSFramework:         cssFramework, // Keep for backward compatibility
		DevMode:              devMode,
		PaginationMode:       paginationMode,
		PageSize:             pageSize,
		EditMode:             editMode,
	}

	// Create resource directory
	resourceDir := filepath.Join(basePath, "internal", "app", resourceNameLower)
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}

	// Read templates using kit loader (checks project kits, user kits, then embedded)
	handlerTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/handler.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read handler template: %w", err)
	}

	// Load main template based on mode
	// With template flattening support, we can now use component-based templates
	var templateTmpl []byte
	if appMode == "multi" {
		// Load component-based template for multi-page apps
		// Template flattening will resolve all {{define}}/{{template}} constructs
		componentNames := []string{
			"layout.tmpl",
			"form.tmpl",
			"toolbar.tmpl",
			"table.tmpl",
			"pagination.tmpl",
			"search.tmpl",
			"stats.tmpl",
			"sort.tmpl",
			"detail.tmpl",
		}

		var fullTemplate string
		for _, compName := range componentNames {
			compTmpl, err := kitLoader.LoadKitComponent(cssFramework, compName)
			if err != nil {
				return fmt.Errorf("failed to load component %s: %w", compName, err)
			}
			fullTemplate += string(compTmpl) + "\n\n"
		}

		// Load the main template file that uses these components
		mainTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/template_components.tmpl.tmpl")
		if err != nil {
			return fmt.Errorf("failed to load main template: %w", err)
		}
		fullTemplate += string(mainTmpl)

		templateTmpl = []byte(fullTemplate)
	} else {
		// Single mode - use simple template
		templateTmpl, err = kitLoader.LoadKitTemplate(cssFramework, "resource/template.tmpl.tmpl")
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}
	}

	queriesTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/queries.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read queries template: %w", err)
	}

	testTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/test.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read test template: %w", err)
	}

	migrationTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/migration.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read migration template: %w", err)
	}

	schemaTmpl, err := kitLoader.LoadKitTemplate(cssFramework, "resource/schema.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read schema template: %w", err)
	}

	// Generate handler
	if err := generateFile(string(handlerTmpl), data, filepath.Join(resourceDir, resourceNameLower+".go"), kit); err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}

	// Generate template
	if err := generateFile(string(templateTmpl), data, filepath.Join(resourceDir, resourceNameLower+".tmpl"), kit); err != nil {
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
	if err := generateFile(string(migrationTmpl), data, migrationPath, kit); err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	// Also append to schema.sql for sqlc
	if err := appendToFile(string(schemaTmpl), data, filepath.Join(dbDir, "schema.sql"), "\n", kit); err != nil {
		return fmt.Errorf("failed to append to schema: %w", err)
	}

	// Append to queries.sql
	if err := appendToFile(string(queriesTmpl), data, filepath.Join(dbDir, "queries.sql"), "\n", kit); err != nil {
		return fmt.Errorf("failed to append to queries: %w", err)
	}

	// Generate consolidated test file (E2E + WebSocket)
	if err := generateFile(string(testTmpl), data, filepath.Join(resourceDir, resourceNameLower+"_test.go"), kit); err != nil {
		return fmt.Errorf("failed to generate test: %w", err)
	}

	// Inject router registration into main.go
	mainGoPath := findMainGo(basePath)
	if mainGoPath != "" {
		routes := []RouteInfo{
			{
				Path:        "/" + resourceNameLower,
				PackageName: resourceNameLower,
				HandlerCall: resourceNameLower + ".Handler(queries)",
				ImportPath:  moduleName + "/internal/app/" + resourceNameLower,
			},
		}

		// For page mode, also register wildcard route for resource detail URLs
		if editMode == "page" {
			routes = append(routes, RouteInfo{
				Path:        "/" + resourceNameLower + "/",
				PackageName: resourceNameLower,
				HandlerCall: resourceNameLower + ".Handler(queries)",
				ImportPath:  moduleName + "/internal/app/" + resourceNameLower,
			})
		}

		for _, route := range routes {
			if err := InjectRoute(mainGoPath, route); err != nil {
				// Log warning but don't fail - user can add route manually
				fmt.Printf("⚠️  Could not auto-inject route %s: %v\n", route.Path, err)
				fmt.Printf("   Please add manually: http.Handle(\"%s\", %s.Handler(queries))\n",
					route.Path, resourceNameLower)
			}
		}
	}

	// Register resource for home page
	if err := RegisterResource(basePath, data.ResourceName, "/"+resourceNameLower, "resource"); err != nil {
		fmt.Printf("⚠️  Could not register resource in home page: %v\n", err)
	}

	return nil
}

func generateFile(tmplStr string, data interface{}, outPath string, kit *kits.KitInfo) error {
	// Merge base funcMap with kit helpers
	funcs := make(template.FuncMap)
	for k, v := range funcMap {
		funcs[k] = v
	}

	// Use kit helpers if provided, otherwise fallback to static CSSHelpers() for backward compatibility
	if kit != nil && kit.Helpers != nil {
		// Get kit-specific helpers using the CSSHelpers interface
		// Note: Old templates pass framework parameter (e.g., [[csscdn .CSSFramework]])
		// but kit helpers don't need it since they're already kit-specific
		// We accept the parameter but ignore it for backward compatibility
		funcs["csscdn"] = func(args ...interface{}) string { return kit.Helpers.CSSCDN() }
		funcs["containerClass"] = func(args ...interface{}) string { return kit.Helpers.ContainerClass() }
		funcs["sectionClass"] = func(args ...interface{}) string { return kit.Helpers.SectionClass() }
		funcs["boxClass"] = func(args ...interface{}) string { return kit.Helpers.BoxClass() }
		funcs["titleClass"] = func(args ...interface{}) string { return kit.Helpers.TitleClass(1) }
		funcs["subtitleClass"] = func(args ...interface{}) string { return kit.Helpers.SubtitleClass() }
		funcs["fieldClass"] = func(args ...interface{}) string { return kit.Helpers.FieldClass() }
		funcs["labelClass"] = func(args ...interface{}) string { return kit.Helpers.LabelClass() }
		funcs["controlClass"] = func(args ...interface{}) string { return "" } // Not in interface, return empty
		funcs["inputClass"] = func(args ...interface{}) string { return kit.Helpers.InputClass() }
		funcs["inputErrorClass"] = func(args ...interface{}) string { return kit.Helpers.InputClass() } // Fallback to inputClass
		funcs["selectClass"] = func(args ...interface{}) string { return kit.Helpers.SelectClass() }
		funcs["selectWrapperClass"] = func(args ...interface{}) string { return "" } // Not in interface
		funcs["checkboxClass"] = func(args ...interface{}) string { return kit.Helpers.CheckboxClass() }
		funcs["textareaClass"] = func(args ...interface{}) string { return kit.Helpers.TextareaClass() }
		funcs["buttonClass"] = func(variant string, args ...interface{}) string { return kit.Helpers.ButtonClass(variant) }
		funcs["buttonGroupClass"] = func(args ...interface{}) string { return kit.Helpers.ButtonGroupClass() }
		funcs["formClass"] = func(args ...interface{}) string { return kit.Helpers.FormClass() }
		funcs["tableClass"] = func(args ...interface{}) string { return kit.Helpers.TableClass() }
		funcs["tableContainerClass"] = func(args ...interface{}) string { return kit.Helpers.TableContainerClass() }
		funcs["theadClass"] = func(args ...interface{}) string { return kit.Helpers.TheadClass() }
		funcs["thClass"] = func(args ...interface{}) string { return kit.Helpers.ThClass() }
		funcs["tbodyClass"] = func(args ...interface{}) string { return kit.Helpers.TbodyClass() }
		funcs["trClass"] = func(args ...interface{}) string { return kit.Helpers.TrClass() }
		funcs["tdClass"] = func(args ...interface{}) string { return kit.Helpers.TdClass() }
		funcs["textClass"] = func(args ...interface{}) string { return kit.Helpers.TextClass("") }
		funcs["textMutedClass"] = func(args ...interface{}) string { return kit.Helpers.TextMutedClass() }
		funcs["textPrimaryClass"] = func(args ...interface{}) string { return kit.Helpers.TextPrimaryClass() }
		funcs["textDangerClass"] = func(args ...interface{}) string { return kit.Helpers.TextDangerClass() }
		funcs["paginationClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationClass() }
		funcs["paginationButtonClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("") }
		funcs["paginationActiveClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("active") }
		funcs["paginationCurrentClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("current") }
		funcs["paginationInfoClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationListClass() }
		funcs["helpTextClass"] = func(args ...interface{}) string { return kit.Helpers.TextMutedClass() } // Use textMuted as fallback
		funcs["errorClass"] = func(args ...interface{}) string { return kit.Helpers.TextDangerClass() }   // Use textDanger as fallback
		funcs["loadingClass"] = func(args ...interface{}) string { return kit.Helpers.LoadingClass() }
		funcs["codeClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["listClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["listItemClass"] = func(args ...interface{}) string { return "" } // Not in interface
		funcs["linkClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["needsWrapper"] = func(args ...interface{}) bool { return kit.Helpers.NeedsWrapper() }
		funcs["needsArticle"] = func(args ...interface{}) bool { return kit.Helpers.NeedsArticle() }
		funcs["needsTableWrapper"] = func(args ...interface{}) bool {
			// Use TableContainerClass as a proxy for whether table wrapper is needed
			return kit.Helpers.TableContainerClass() != ""
		}
		funcs["tableWrapperClass"] = func(args ...interface{}) string { return kit.Helpers.TableContainerClass() }
		// Add dict helper from kit
		funcs["dict"] = kit.Helpers.Dict
		funcs["until"] = kit.Helpers.Until
		funcs["add"] = kit.Helpers.Add
	} else {
		// Fallback to static CSS helpers for backward compatibility
		for k, v := range CSSHelpers() {
			funcs[k] = v
		}
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

func appendToFile(tmplStr string, data interface{}, outPath, separator string, kit *kits.KitInfo) error {
	// Merge base funcMap with kit helpers
	funcs := make(template.FuncMap)
	for k, v := range funcMap {
		funcs[k] = v
	}

	// Use kit helpers if provided (same logic as generateFile)
	if kit != nil && kit.Helpers != nil {
		// Accept optional framework parameter for backward compatibility
		funcs["csscdn"] = func(args ...interface{}) string { return kit.Helpers.CSSCDN() }
		funcs["containerClass"] = func(args ...interface{}) string { return kit.Helpers.ContainerClass() }
		funcs["sectionClass"] = func(args ...interface{}) string { return kit.Helpers.SectionClass() }
		funcs["boxClass"] = func(args ...interface{}) string { return kit.Helpers.BoxClass() }
		funcs["titleClass"] = func(args ...interface{}) string { return kit.Helpers.TitleClass(1) }
		funcs["subtitleClass"] = func(args ...interface{}) string { return kit.Helpers.SubtitleClass() }
		funcs["fieldClass"] = func(args ...interface{}) string { return kit.Helpers.FieldClass() }
		funcs["labelClass"] = func(args ...interface{}) string { return kit.Helpers.LabelClass() }
		funcs["controlClass"] = func(args ...interface{}) string { return "" } // Not in interface, return empty
		funcs["inputClass"] = func(args ...interface{}) string { return kit.Helpers.InputClass() }
		funcs["inputErrorClass"] = func(args ...interface{}) string { return kit.Helpers.InputClass() } // Fallback to inputClass
		funcs["selectClass"] = func(args ...interface{}) string { return kit.Helpers.SelectClass() }
		funcs["selectWrapperClass"] = func(args ...interface{}) string { return "" } // Not in interface
		funcs["checkboxClass"] = func(args ...interface{}) string { return kit.Helpers.CheckboxClass() }
		funcs["textareaClass"] = func(args ...interface{}) string { return kit.Helpers.TextareaClass() }
		funcs["buttonClass"] = func(variant string, args ...interface{}) string { return kit.Helpers.ButtonClass(variant) }
		funcs["buttonGroupClass"] = func(args ...interface{}) string { return kit.Helpers.ButtonGroupClass() }
		funcs["formClass"] = func(args ...interface{}) string { return kit.Helpers.FormClass() }
		funcs["tableClass"] = func(args ...interface{}) string { return kit.Helpers.TableClass() }
		funcs["tableContainerClass"] = func(args ...interface{}) string { return kit.Helpers.TableContainerClass() }
		funcs["theadClass"] = func(args ...interface{}) string { return kit.Helpers.TheadClass() }
		funcs["thClass"] = func(args ...interface{}) string { return kit.Helpers.ThClass() }
		funcs["tbodyClass"] = func(args ...interface{}) string { return kit.Helpers.TbodyClass() }
		funcs["trClass"] = func(args ...interface{}) string { return kit.Helpers.TrClass() }
		funcs["tdClass"] = func(args ...interface{}) string { return kit.Helpers.TdClass() }
		funcs["textClass"] = func(args ...interface{}) string { return kit.Helpers.TextClass("") }
		funcs["textMutedClass"] = func(args ...interface{}) string { return kit.Helpers.TextMutedClass() }
		funcs["textPrimaryClass"] = func(args ...interface{}) string { return kit.Helpers.TextPrimaryClass() }
		funcs["textDangerClass"] = func(args ...interface{}) string { return kit.Helpers.TextDangerClass() }
		funcs["paginationClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationClass() }
		funcs["paginationButtonClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("") }
		funcs["paginationActiveClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("active") }
		funcs["paginationCurrentClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationButtonClass("current") }
		funcs["paginationInfoClass"] = func(args ...interface{}) string { return kit.Helpers.PaginationListClass() }
		funcs["helpTextClass"] = func(args ...interface{}) string { return kit.Helpers.TextMutedClass() } // Use textMuted as fallback
		funcs["errorClass"] = func(args ...interface{}) string { return kit.Helpers.TextDangerClass() }   // Use textDanger as fallback
		funcs["loadingClass"] = func(args ...interface{}) string { return kit.Helpers.LoadingClass() }
		funcs["codeClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["listClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["listItemClass"] = func(args ...interface{}) string { return "" } // Not in interface
		funcs["linkClass"] = func(args ...interface{}) string { return "" }     // Not in interface
		funcs["needsWrapper"] = func(args ...interface{}) bool { return kit.Helpers.NeedsWrapper() }
		funcs["needsArticle"] = func(args ...interface{}) bool { return kit.Helpers.NeedsArticle() }
		funcs["needsTableWrapper"] = func(args ...interface{}) bool {
			return kit.Helpers.TableContainerClass() != ""
		}
		funcs["tableWrapperClass"] = func(args ...interface{}) string { return kit.Helpers.TableContainerClass() }
		funcs["dict"] = kit.Helpers.Dict
		funcs["until"] = kit.Helpers.Until
		funcs["add"] = kit.Helpers.Add
	} else {
		for k, v := range CSSHelpers() {
			funcs[k] = v
		}
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
