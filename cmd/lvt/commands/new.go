package commands

import (
	"fmt"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

func New(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("app name required")
	}

	appName := args[0]

	fmt.Printf("Creating new LiveTemplate app: %s\n", appName)

	if err := generator.GenerateApp(appName); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ App created successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", appName)
	fmt.Println("  lvt gen users name:string email:string")
	fmt.Println("  cd internal/database && go run github.com/sqlc-dev/sqlc/cmd/sqlc generate && cd ../..")
	fmt.Printf("  go run cmd/%s/main.go\n", appName)
	fmt.Println()

	return nil
}
