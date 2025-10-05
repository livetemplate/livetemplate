package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

func Gen(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("resource name required")
	}

	// Check if "view" subcommand
	if args[0] == "view" {
		return GenView(args[1:])
	}

	// Parse flags
	cssFramework := "tailwind" // default
	appMode := "multi"         // default
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--css" && i+1 < len(args) {
			cssFramework = args[i+1]
			i++ // skip next arg
		} else if args[i] == "--mode" && i+1 < len(args) {
			appMode = args[i+1]
			i++ // skip next arg
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	if len(filteredArgs) < 1 {
		return fmt.Errorf("resource name required")
	}

	resourceName := filteredArgs[0]
	fieldArgs := filteredArgs[1:]

	if len(fieldArgs) == 0 {
		return fmt.Errorf("at least one field required (format: name:type)")
	}

	// Validate CSS framework
	validFrameworks := map[string]bool{"tailwind": true, "bulma": true, "pico": true, "none": true}
	if !validFrameworks[cssFramework] {
		return fmt.Errorf("invalid CSS framework: %s (valid: tailwind, bulma, pico, none)", cssFramework)
	}

	// Validate app mode
	validModes := map[string]bool{"multi": true, "single": true}
	if !validModes[appMode] {
		return fmt.Errorf("invalid mode: %s (valid: multi, single)", appMode)
	}

	// Parse fields with type inference support
	fields, err := parseFieldsWithInference(fieldArgs)
	if err != nil {
		return err
	}

	// Get module name from go.mod
	moduleName, err := getModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w (are you in a Go project?)", err)
	}

	// Get current directory
	basePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Printf("Generating CRUD resource: %s\n", resourceName)
	fmt.Printf("CSS Framework: %s\n", cssFramework)
	fmt.Printf("Fields: ")
	for i, f := range fields {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s:%s", f.Name, f.Type)
	}
	fmt.Println()

	if err := generator.GenerateResource(basePath, moduleName, resourceName, fields, cssFramework, appMode); err != nil {
		return err
	}

	resourceNameLower := strings.ToLower(resourceName)

	fmt.Println()
	fmt.Println("✅ Resource generated successfully!")
	fmt.Println()
	fmt.Println("Files created:")
	fmt.Printf("  internal/app/%s/%s.go\n", resourceNameLower, resourceNameLower)
	fmt.Printf("  internal/app/%s/%s.tmpl\n", resourceNameLower, resourceNameLower)
	fmt.Println()
	fmt.Println("Files updated:")
	fmt.Println("  internal/database/schema.sql")
	fmt.Println("  internal/database/queries.sql")
	fmt.Println()
	fmt.Println("Route auto-injected:")
	fmt.Printf("  http.Handle(\"/%s\", %s.Handler(queries))\n", resourceNameLower, resourceNameLower)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run migration:")
	fmt.Println("     lvt migration up")
	fmt.Println("  2. Run your app")
	fmt.Println()

	return nil
}

func GenView(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("view name required")
	}

	// Parse --css flag
	cssFramework := "tailwind" // default
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--css" && i+1 < len(args) {
			cssFramework = args[i+1]
			i++ // skip next arg
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	if len(filteredArgs) < 1 {
		return fmt.Errorf("view name required")
	}

	// Validate CSS framework
	validFrameworks := map[string]bool{"tailwind": true, "bulma": true, "pico": true, "none": true}
	if !validFrameworks[cssFramework] {
		return fmt.Errorf("invalid CSS framework: %s (valid: tailwind, bulma, pico, none)", cssFramework)
	}

	viewName := filteredArgs[0]

	// Get module name from go.mod
	moduleName, err := getModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w (are you in a Go project?)", err)
	}

	// Get current directory
	basePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Printf("Generating view-only handler: %s\n", viewName)
	fmt.Printf("CSS Framework: %s\n", cssFramework)

	if err := generator.GenerateView(basePath, moduleName, viewName, cssFramework); err != nil {
		return err
	}

	viewNameLower := strings.ToLower(viewName)

	fmt.Println()
	fmt.Println("✅ View generated successfully!")
	fmt.Println()
	fmt.Println("Files created:")
	fmt.Printf("  internal/app/%s/%s.go\n", viewNameLower, viewNameLower)
	fmt.Printf("  internal/app/%s/%s.tmpl\n", viewNameLower, viewNameLower)
	fmt.Printf("  internal/app/%s/%s_test.go\n", viewNameLower, viewNameLower)
	fmt.Println()
	fmt.Println("Route auto-injected:")
	fmt.Printf("  http.Handle(\"/%s\", %s.Handler())\n", viewNameLower, viewNameLower)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Customize handler: internal/app/%s/%s.go\n", viewNameLower, viewNameLower)
	fmt.Printf("  2. Edit template: internal/app/%s/%s.tmpl\n", viewNameLower, viewNameLower)
	fmt.Println("  3. Run your app")
	fmt.Println()

	return nil
}

func parseFieldsWithInference(fieldArgs []string) ([]parser.Field, error) {
	// Try parsing with type inference first
	fields := make([]parser.Field, 0, len(fieldArgs))

	for _, arg := range fieldArgs {
		var name, typ string

		// Check if it contains ":"
		if strings.Contains(arg, ":") {
			// Explicit type - use normal parser
			parts := strings.SplitN(arg, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid field format: %s (expected name:type)", arg)
			}
			name = strings.TrimSpace(parts[0])
			typ = strings.TrimSpace(parts[1])
		} else {
			// No type specified - infer from name
			name = strings.TrimSpace(arg)
			typ = inferTypeForDirectMode(name)
		}

		// Map to Go and SQL types
		goType, sqlType, err := parser.MapType(typ)
		if err != nil {
			return nil, fmt.Errorf("field '%s': %w", name, err)
		}

		// Create field with reference metadata
		field := parser.Field{
			Name:    name,
			Type:    typ,
			GoType:  goType,
			SQLType: sqlType,
		}

		// Parse reference metadata if it's a reference type
		if strings.HasPrefix(strings.ToLower(typ), "references:") {
			parts := strings.Split(typ, ":")
			if len(parts) < 2 {
				return nil, fmt.Errorf("field '%s': invalid references syntax, expected 'references:table_name'", name)
			}

			field.IsReference = true
			field.ReferencedTable = parts[1]
			field.OnDelete = "CASCADE" // Default

			// Check for custom on_delete action
			if len(parts) > 2 {
				action := strings.ToUpper(parts[2])
				switch action {
				case "CASCADE", "SET NULL", "RESTRICT", "NO ACTION", "SET_NULL":
					if action == "SET_NULL" {
						action = "SET NULL"
					}
					field.OnDelete = action
				default:
					return nil, fmt.Errorf("field '%s': invalid ON DELETE action '%s'", name, parts[2])
				}
			}
		}

		fields = append(fields, field)
	}

	return fields, nil
}

func inferTypeForDirectMode(fieldName string) string {
	lower := strings.ToLower(fieldName)

	// Exact matches for common field names
	exactMatches := map[string]string{
		"name": "string", "email": "string", "title": "string",
		"description": "string", "content": "string", "body": "string",
		"username": "string", "password": "string", "token": "string",
		"url": "string", "slug": "string", "path": "string",
		"address": "string", "city": "string", "state": "string",
		"country": "string", "phone": "string", "status": "string",

		"age": "int", "count": "int", "quantity": "int",
		"views": "int", "likes": "int", "shares": "int",
		"year": "int", "rating": "int",

		"price": "float", "amount": "float", "total": "float",
		"latitude": "float", "longitude": "float",

		"enabled": "bool", "active": "bool", "visible": "bool",
		"published": "bool", "deleted": "bool", "featured": "bool",

		"created_at": "time", "updated_at": "time", "deleted_at": "time",
		"published_at": "time", "expires_at": "time",
	}

	if t, ok := exactMatches[lower]; ok {
		return t
	}

	// Pattern matching for suffixes/prefixes
	if strings.HasSuffix(lower, "_at") || strings.HasSuffix(lower, "_date") ||
		strings.HasSuffix(lower, "_time") || strings.HasSuffix(lower, "date") {
		return "time"
	}

	if strings.HasPrefix(lower, "is_") || strings.HasPrefix(lower, "has_") ||
		strings.HasPrefix(lower, "can_") || strings.HasPrefix(lower, "should_") {
		return "bool"
	}

	if strings.HasSuffix(lower, "_count") || strings.HasSuffix(lower, "_number") ||
		strings.HasSuffix(lower, "_id") || strings.HasSuffix(lower, "id") {
		return "int"
	}

	if strings.HasSuffix(lower, "_price") || strings.HasSuffix(lower, "_amount") ||
		strings.HasSuffix(lower, "_total") || strings.HasSuffix(lower, "price") {
		return "float"
	}

	// Default to string
	return "string"
}

func getModuleName() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module name not found in go.mod")
}
