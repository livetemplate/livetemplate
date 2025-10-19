package commands

import (
	"fmt"

	"github.com/livefir/livetemplate/cmd/lvt/internal/seeder"
)

func Seed(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("resource name required: lvt seed <resource-name> --count N [--cleanup]")
	}

	resourceName := args[0]

	// Parse flags
	var count int
	var cleanup bool
	var hasCount bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--count":
			if i+1 >= len(args) {
				return fmt.Errorf("--count requires a value")
			}
			i++
			if _, err := fmt.Sscanf(args[i], "%d", &count); err != nil {
				return fmt.Errorf("invalid count value: %s", args[i])
			}
			hasCount = true

		case "--cleanup":
			cleanup = true

		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	// Validate flags
	if !cleanup && !hasCount {
		return fmt.Errorf("either --count or --cleanup must be specified")
	}

	if hasCount && count <= 0 {
		return fmt.Errorf("count must be greater than 0")
	}

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

	// Create seeder
	s, err := seeder.New()
	if err != nil {
		return err
	}
	defer s.Close()

	// Perform cleanup if requested
	if cleanup {
		if err := s.Cleanup(table.Name); err != nil {
			return err
		}

		// If only cleanup was requested, we're done
		if !hasCount {
			return nil
		}

		fmt.Println()
	}

	// Perform seeding if count was specified
	if hasCount {
		if err := s.Seed(*table, count); err != nil {
			return err
		}

		// Show total test records
		totalTest, err := s.CountTestRecords(table.Name)
		if err == nil && totalTest > 0 {
			fmt.Printf("\nTotal test records in %s: %d\n", table.Name, totalTest)
		}
	}

	return nil
}
