package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/config"
	"github.com/livefir/livetemplate/cmd/lvt/internal/kits"
)

func GenerateApp(appName, moduleName, kit string, devMode bool) error {
	// Sanitize app name
	appName = strings.ToLower(strings.TrimSpace(appName))
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	// Check if directory already exists
	if _, err := os.Stat(appName); err == nil {
		return fmt.Errorf("directory '%s' already exists", appName)
	}

	// Load kit using KitLoader
	kitLoader := kits.DefaultLoader()
	kitInfo, err := kitLoader.Load(kit)
	if err != nil {
		return fmt.Errorf("failed to load kit %q: %w", kit, err)
	}

	// Get CSS framework from kit manifest
	cssFramework := kitInfo.Manifest.CSSFramework

	// Module name is provided by caller (defaults to app name)
	data := AppData{
		AppName:      appName,
		ModuleName:   moduleName,
		DevMode:      devMode,
		Kit:          kitInfo,
		CSSFramework: cssFramework,
	}

	// Simple kit generates just 2 files
	if kit == "simple" {
		return generateSimpleApp(appName, moduleName, data, kitLoader, kitInfo)
	}

	// Create directory structure
	dirs := []string{
		appName,
		filepath.Join(appName, "cmd", appName),
		filepath.Join(appName, "internal", "app"),
		filepath.Join(appName, "internal", "app", "home"), // Home page directory
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

	// Read templates using kit loader (checks project kits, user kits, then embedded)
	mainGoTmpl, err := kitLoader.LoadKitTemplate(kit, "app/main.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read main.go template: %w", err)
	}

	goModTmpl, err := kitLoader.LoadKitTemplate(kit, "app/go.mod.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read go.mod template: %w", err)
	}

	dbGoTmpl, err := kitLoader.LoadKitTemplate(kit, "app/db.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read db.go template: %w", err)
	}

	sqlcYamlTmpl, err := kitLoader.LoadKitTemplate(kit, "app/sqlc.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read sqlc.yaml template: %w", err)
	}

	modelsGoTmpl, err := kitLoader.LoadKitTemplate(kit, "app/models.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read models.go template: %w", err)
	}

	homeGoTmpl, err := kitLoader.LoadKitTemplate(kit, "app/home.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read home.go template: %w", err)
	}

	homeTmplTmpl, err := kitLoader.LoadKitTemplate(kit, "app/home.tmpl.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read home.tmpl template: %w", err)
	}

	// Generate main.go
	if err := generateFile(string(mainGoTmpl), data, filepath.Join(appName, "cmd", appName, "main.go"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate go.mod
	if err := generateFile(string(goModTmpl), data, filepath.Join(appName, "go.mod"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	// Generate database/db.go
	if err := generateFile(string(dbGoTmpl), data, filepath.Join(appName, "internal", "database", "db.go"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate db.go: %w", err)
	}

	// Generate database/sqlc.yaml
	if err := generateFile(string(sqlcYamlTmpl), data, filepath.Join(appName, "internal", "database", "sqlc.yaml"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate sqlc.yaml: %w", err)
	}

	// Generate placeholder models.go (will be replaced by sqlc)
	if err := generateFile(string(modelsGoTmpl), data, filepath.Join(appName, "internal", "database", "models", "models.go"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate models.go: %w", err)
	}

	// Create empty schema.sql and queries.sql
	if err := os.WriteFile(filepath.Join(appName, "internal", "database", "schema.sql"), []byte("-- Database schema\n"), 0644); err != nil {
		return fmt.Errorf("failed to create schema.sql: %w", err)
	}

	if err := os.WriteFile(filepath.Join(appName, "internal", "database", "queries.sql"), []byte("-- Database queries\n"), 0644); err != nil {
		return fmt.Errorf("failed to create queries.sql: %w", err)
	}

	// Generate home page handler
	if err := generateFile(string(homeGoTmpl), data, filepath.Join(appName, "internal", "app", "home", "home.go"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate home.go: %w", err)
	}

	// Generate home page template
	if err := generateFile(string(homeTmplTmpl), data, filepath.Join(appName, "internal", "app", "home", "home.tmpl"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate home.tmpl: %w", err)
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

	// Create project config file
	projectConfig := &config.ProjectConfig{
		Kit:     kit,
		DevMode: devMode,
	}
	if err := config.SaveProjectConfig(appName, projectConfig); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	// Create empty .lvtresources file for tracking resources
	if err := os.WriteFile(filepath.Join(appName, ".lvtresources"), []byte("[]"), 0644); err != nil {
		return fmt.Errorf("failed to create .lvtresources: %w", err)
	}

	return nil
}

// generateSimpleApp generates a minimal 2-file app structure
func generateSimpleApp(appName, moduleName string, data AppData, kitLoader *kits.KitLoader, kitInfo *kits.KitInfo) error {
	// Create app directory
	if err := os.MkdirAll(appName, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", appName, err)
	}

	// Read templates
	mainGoTmpl, err := kitLoader.LoadKitTemplate("simple", "app/main.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read main.go template: %w", err)
	}

	indexTmplTmpl, err := kitLoader.LoadKitTemplate("simple", "app/index.tmpl.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read index.tmpl template: %w", err)
	}

	goModTmpl, err := kitLoader.LoadKitTemplate("simple", "app/go.mod.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read go.mod template: %w", err)
	}

	// Generate main.go
	if err := generateFile(string(mainGoTmpl), data, filepath.Join(appName, "main.go"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate main.go: %w", err)
	}

	// Generate {appname}.tmpl
	if err := generateFile(string(indexTmplTmpl), data, filepath.Join(appName, appName+".tmpl"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate %s.tmpl: %w", appName, err)
	}

	// Generate go.mod
	if err := generateFile(string(goModTmpl), data, filepath.Join(appName, "go.mod"), kitInfo); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	// Create README
	readme := fmt.Sprintf(`# %s

A simple LiveTemplate application.

## Getting Started

1. Run the server:
   `+"`"+`bash
   go run main.go
   `+"`"+`

2. Open http://localhost:8080

## Structure

- `+"`main.go`"+` - Application server and state management
- `+"`%s.tmpl`"+` - HTML template
- `+"`go.mod`"+` - Go module configuration

## Customization

Edit `+"`main.go`"+` to modify your state and actions.
Edit `+"`%s.tmpl`"+` to change the UI.

## Next Steps

For more complex apps with database and resources:
`+"`"+`bash
lvt new myapp --kit multi
`+"`"+`
`, appName, appName, appName)

	if err := os.WriteFile(filepath.Join(appName, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	// Create project config file
	projectConfig := &config.ProjectConfig{
		Kit:     "simple",
		DevMode: data.DevMode,
	}
	if err := config.SaveProjectConfig(appName, projectConfig); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	return nil
}

// ReadDevMode reads the dev_mode setting from .lvtrc in the current directory
// Returns false if .lvtrc doesn't exist or dev_mode is not set
func ReadDevMode(basePath string) bool {
	projectConfig, err := config.LoadProjectConfig(basePath)
	if err != nil {
		return false
	}
	return projectConfig.DevMode
}
