package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/livetemplate/livetemplate/cmd/lvt/internal/kits"
	"github.com/livetemplate/livetemplate/cmd/lvt/internal/parser"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// GenerateSchema generates only database files (migration, schema, queries) without handler or template
func GenerateSchema(basePath, moduleName, tableName string, fields []parser.Field, kitName, cssFramework string) error {
	// Defaults
	if kitName == "" {
		kitName = "multi"
	}
	if cssFramework == "" {
		cssFramework = "tailwind"
	}

	// Load kit using KitLoader
	kitLoader := kits.DefaultLoader()
	kit, err := kitLoader.Load(kitName)
	if err != nil {
		return fmt.Errorf("failed to load kit %q: %w", kitName, err)
	}

	// Normalize table name
	tableNameLower := strings.ToLower(tableName)
	titleCaser := cases.Title(language.English)

	// Derive singular and plural forms for database table
	tableNameSingular := singularize(tableNameLower)
	tableNamePlural := pluralize(tableNameSingular)
	resourceNameSingularCap := titleCaser.String(tableNameSingular)
	resourceNamePluralCap := titleCaser.String(tableNamePlural)

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

	data := ResourceData{
		PackageName:          tableNameLower,
		ModuleName:           moduleName,
		ResourceName:         titleCaser.String(tableNameLower),
		ResourceNameLower:    tableNameLower,
		ResourceNameSingular: resourceNameSingularCap,
		ResourceNamePlural:   resourceNamePluralCap,
		TableName:            tableNamePlural,
		Fields:               fieldData,
		Kit:                  kit,
		CSSFramework:         cssFramework,
	}

	// Load templates
	migrationTmpl, err := kitLoader.LoadKitTemplate(kitName, "resource/migration.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read migration template: %w", err)
	}

	schemaTmpl, err := kitLoader.LoadKitTemplate(kitName, "resource/schema.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read schema template: %w", err)
	}

	queriesTmpl, err := kitLoader.LoadKitTemplate(kitName, "resource/queries.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read queries template: %w", err)
	}

	// Create database directory structure
	dbDir := filepath.Join(basePath, "internal", "database")
	migrationsDir := filepath.Join(dbDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate unique timestamp for migration
	timestamp := time.Now()
	migrationFilename := ""
	migrationPath := ""
	for {
		timestampStr := timestamp.Format("20060102150405")
		migrationFilename = fmt.Sprintf("%s_create_%s.sql", timestampStr, tableNamePlural)
		migrationPath = filepath.Join(migrationsDir, migrationFilename)

		// Check if any migration file exists with this timestamp prefix
		matches, _ := filepath.Glob(filepath.Join(migrationsDir, timestampStr+"_*.sql"))
		if len(matches) == 0 {
			break
		}

		// Increment by 1 second and try again
		timestamp = timestamp.Add(1 * time.Second)
	}

	// Generate migration file
	if err := generateFile(string(migrationTmpl), data, migrationPath, kit); err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	// Append to schema.sql for sqlc
	if err := appendToFile(string(schemaTmpl), data, filepath.Join(dbDir, "schema.sql"), "\n", kit); err != nil {
		return fmt.Errorf("failed to append to schema: %w", err)
	}

	// Append to queries.sql
	if err := appendToFile(string(queriesTmpl), data, filepath.Join(dbDir, "queries.sql"), "\n", kit); err != nil {
		return fmt.Errorf("failed to append to queries: %w", err)
	}

	// Run sqlc generate to create Go types
	fmt.Println("Running sqlc generate...")
	cmd := exec.Command("sqlc", "generate")
	cmd.Dir = basePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️  sqlc generate failed: %v\n", err)
		fmt.Printf("Output: %s\n", string(output))
		fmt.Println("You can run 'sqlc generate' manually later")
	} else {
		fmt.Println("✅ sqlc generate completed successfully")
	}

	// Register schema in resource tracker
	if err := RegisterResource(basePath, data.ResourceName, "", "schema"); err != nil {
		fmt.Printf("⚠️  Could not register schema in .lvtresources: %v\n", err)
	}

	return nil
}
