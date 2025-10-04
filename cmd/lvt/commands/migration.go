package commands

import (
	"fmt"

	"github.com/livefir/livetemplate/cmd/lvt/internal/migration"
)

func Migration(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required: up, down, status, or create <name>")
	}

	command := args[0]

	// Create runner
	runner, err := migration.New()
	if err != nil {
		return err
	}
	defer runner.Close()

	switch command {
	case "up":
		fmt.Println("Running pending migrations...")
		if err := runner.Up(); err != nil {
			return err
		}
		fmt.Println("✅ All migrations applied successfully!")

	case "down":
		fmt.Println("Rolling back last migration...")
		if err := runner.Down(); err != nil {
			return err
		}
		fmt.Println("✅ Migration rolled back successfully!")

	case "status":
		fmt.Println("Migration status:")
		if err := runner.Status(); err != nil {
			return err
		}

	case "create":
		if len(args) < 2 {
			return fmt.Errorf("migration name required: lvt migration create <name>")
		}
		name := args[1]
		if err := runner.Create(name); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown command: %s (expected: up, down, status, create)", command)
	}

	return nil
}
