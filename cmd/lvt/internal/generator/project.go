package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateApp(appName string) error {
	// Sanitize app name
	appName = strings.ToLower(strings.TrimSpace(appName))
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	// Check if directory already exists
	if _, err := os.Stat(appName); err == nil {
		return fmt.Errorf("directory '%s' already exists", appName)
	}

	// Default module name
	moduleName := appName

	data := AppData{
		AppName:    appName,
		ModuleName: moduleName,
	}

	// Create directory structure
	dirs := []string{
		appName,
		filepath.Join(appName, "cmd", appName),
		filepath.Join(appName, "internal", "app"),
		filepath.Join(appName, "internal", "database", "models"),
		filepath.Join(appName, "internal", "database", "migrations"),
		filepath.Join(appName, "internal", "shared"),
		filepath.Join(appName, "web", "assets"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize template loader for cascading template lookup
	loader := NewTemplateLoader()

	// Read templates using loader (checks custom templates first, falls back to embedded)
	mainGoTmpl, err := loader.Load("app/main.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read main.go template: %w", err)
	}

	goModTmpl, err := loader.Load("app/go.mod.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read go.mod template: %w", err)
	}

	dbGoTmpl, err := loader.Load("app/db.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read db.go template: %w", err)
	}

	sqlcYamlTmpl, err := loader.Load("app/sqlc.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read sqlc.yaml template: %w", err)
	}

	// Generate main.go
	if err := generateFile(string(mainGoTmpl), data, filepath.Join(appName, "cmd", appName, "main.go")); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate go.mod
	if err := generateFile(string(goModTmpl), data, filepath.Join(appName, "go.mod")); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	// Generate database/db.go
	if err := generateFile(string(dbGoTmpl), data, filepath.Join(appName, "internal", "database", "db.go")); err != nil {
		return fmt.Errorf("failed to generate db.go: %w", err)
	}

	// Generate database/sqlc.yaml
	if err := generateFile(string(sqlcYamlTmpl), data, filepath.Join(appName, "internal", "database", "sqlc.yaml")); err != nil {
		return fmt.Errorf("failed to generate sqlc.yaml: %w", err)
	}

	// Create empty schema.sql and queries.sql
	if err := os.WriteFile(filepath.Join(appName, "internal", "database", "schema.sql"), []byte("-- Database schema\n"), 0644); err != nil {
		return fmt.Errorf("failed to create schema.sql: %w", err)
	}

	if err := os.WriteFile(filepath.Join(appName, "internal", "database", "queries.sql"), []byte("-- Database queries\n"), 0644); err != nil {
		return fmt.Errorf("failed to create queries.sql: %w", err)
	}

	// Create README
	readme := fmt.Sprintf(`# %s

A LiveTemplate application.

## Getting Started

1. Generate a resource:
   `+"```"+`
   lvt gen users name:string email:string
   `+"```"+`

2. Run migrations:
   `+"```"+`
   lvt migration up
   `+"```"+`

3. Run sqlc to generate database code:
   `+"```"+`
   cd internal/database
   go run github.com/sqlc-dev/sqlc/cmd/sqlc generate
   cd ../..
   `+"```"+`

4. Run the server:
   `+"```"+`
   go run cmd/%s/main.go
   `+"```"+`

5. Open http://localhost:8080

## Project Structure

- `+"`cmd/%s/`"+` - Application entry point
- `+"`internal/app/`"+` - Handlers and templates
- `+"`internal/database/`"+` - Database layer with sqlc
- `+"`internal/database/migrations/`"+` - Database migrations
- `+"`internal/shared/`"+` - Shared utilities

## Database Migrations

Create a new migration:
`+"```"+`
lvt migration create add_user_avatar
`+"```"+`

Run pending migrations:
`+"```"+`
lvt migration up
`+"```"+`

Rollback last migration:
`+"```"+`
lvt migration down
`+"```"+`

Check migration status:
`+"```"+`
lvt migration status
`+"```"+`

## Testing

Run tests:
`+"```"+`
go test ./...
`+"```"+`
`, appName, appName, appName)

	if err := os.WriteFile(filepath.Join(appName, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	return nil
}
