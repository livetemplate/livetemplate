package commands

import (
	"fmt"
	"strings"

	"github.com/livefir/livetemplate/cmd/lvt/internal/seeder"
)

func Resource(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: list or describe <resource-name>")
	}

	command := args[0]

	switch command {
	case "list", "ls":
		return listResources()

	case "describe", "desc", "show":
		if len(args) < 2 {
			return fmt.Errorf("resource name required: lvt resource describe <resource-name>")
		}
		resourceName := args[1]
		return describeResource(resourceName)

	default:
		return fmt.Errorf("unknown command: %s (expected: list, describe)", command)
	}
}

func listResources() error {
	// Find schema file
	schemaPath, err := seeder.FindSchemaFile()
	if err != nil {
		return err
	}

	// Parse schema
	tables, err := seeder.ParseSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	if len(tables) == 0 {
		fmt.Println("No resources found in schema.")
		return nil
	}

	fmt.Println("Available resources:")
	for _, table := range tables {
		fieldCount := len(table.Columns)
		fmt.Printf("  %-20s (%d field%s)\n", table.Name, fieldCount, pluralize(fieldCount))
	}

	fmt.Println()
	fmt.Println("Use 'lvt resource describe <name>' to see details")

	return nil
}

func describeResource(resourceName string) error {
	// Find schema file
	schemaPath, err := seeder.FindSchemaFile()
	if err != nil {
		return err
	}

	// Parse schema
	tables, err := seeder.ParseSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	// Find the table
	table := seeder.FindTable(tables, resourceName)
	if table == nil {
		return fmt.Errorf("resource '%s' not found in schema", resourceName)
	}

	// Display resource details
	fmt.Printf("Resource: %s\n", table.Name)
	fmt.Printf("Table: %s\n", table.Name)
	fmt.Println()

	// Display fields
	fmt.Println("Fields:")
	maxNameLen := 0
	maxTypeLen := 0
	for _, col := range table.Columns {
		if len(col.Name) > maxNameLen {
			maxNameLen = len(col.Name)
		}
		if len(col.Type) > maxTypeLen {
			maxTypeLen = len(col.Type)
		}
	}

	for _, col := range table.Columns {
		constraints := []string{}
		if col.IsPrimary {
			constraints = append(constraints, "Primary Key")
		}
		if !col.Nullable {
			constraints = append(constraints, "NOT NULL")
		}

		constraintStr := ""
		if len(constraints) > 0 {
			constraintStr = fmt.Sprintf("(%s)", strings.Join(constraints, ", "))
		}

		// Generate example value
		example := seeder.GenerateExampleValue(col)

		fmt.Printf("  %-*s  %-*s  %-25s  Example: %s\n",
			maxNameLen, col.Name,
			maxTypeLen, col.Type,
			constraintStr,
			example,
		)
	}

	// Display indexes
	if len(table.Indexes) > 0 {
		fmt.Println()
		fmt.Println("Indexes:")
		for _, idx := range table.Indexes {
			fmt.Printf("  %s (%s)\n", idx.Name, strings.Join(idx.Columns, ", "))
		}
	}

	// Display sample commands
	fmt.Println()
	fmt.Println("Sample seed command:")
	fmt.Printf("  lvt seed %s --count 50\n", table.Name)

	return nil
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
