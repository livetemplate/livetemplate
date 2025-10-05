package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

func New(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("app name required")
	}

	appName := args[0]
	moduleName := appName // Default to app name

	// Check for --module flag
	for i := 1; i < len(args); i++ {
		if args[i] == "--module" && i+1 < len(args) {
			moduleName = args[i+1]
			break
		}
	}

	fmt.Printf("Creating new LiveTemplate app: %s\n", appName)

	// Check if we're inside another Go module
	isNested := false
	if _, err := os.Stat("go.mod"); err == nil {
		isNested = true
	}

	if err := generator.GenerateApp(appName, moduleName); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✅ App created successfully!")

	if isNested {
		fmt.Println()
		fmt.Println("⚠️  Warning: Creating app inside another Go module")
		fmt.Printf("   You'll need to use: GOWORK=off go run cmd/%s/main.go\n", appName)
		fmt.Println("   For production, create apps outside Go module directories")
	}

	fmt.Println()

	// Run go mod tidy to resolve and download dependencies
	fmt.Println("Installing dependencies...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = appName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Warning: failed to install dependencies: %v\n", err)
		fmt.Printf("   You can run it manually: cd %s && go mod tidy\n", appName)
	} else {
		fmt.Println("✅ Dependencies installed!")
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", appName)
	fmt.Println("  lvt gen users name:string email:string")
	fmt.Println("  lvt migration up")
	fmt.Printf("  go run cmd/%s/main.go\n", appName)
	fmt.Println()

	return nil
}
