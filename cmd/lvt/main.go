package main

import (
	"fmt"
	"os"

	"github.com/livefir/livetemplate/cmd/lvt/commands"
	"github.com/livefir/livetemplate/cmd/lvt/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	var err error

	switch command {
	case "new":
		if len(args) == 0 {
			// Interactive mode
			err = ui.NewAppInteractive()
		} else {
			// Direct mode
			err = commands.New(args)
		}
	case "gen":
		if len(args) == 0 {
			// Interactive resource mode
			err = ui.GenResourceInteractive()
		} else if args[0] == "view" && len(args) == 1 {
			// Interactive view mode
			err = ui.GenViewInteractive()
		} else {
			// Direct mode
			err = commands.Gen(args)
		}
	case "migration":
		err = commands.Migration(args)
	case "template":
		err = commands.Template(args)
	case "help", "--help", "-h":
		printUsage()
		return
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("LiveTemplate CLI Generator")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lvt new [<app-name>]                      Create a new LiveTemplate app")
	fmt.Println("  lvt gen [<resource> <field:type>...]      Generate CRUD resource with database")
	fmt.Println("  lvt gen view [<name>]                     Generate view-only handler")
	fmt.Println("  lvt migration <command>                   Manage database migrations")
	fmt.Println("  lvt template <command>                    Manage custom templates")
	fmt.Println()
	fmt.Println("Interactive Mode (no arguments):")
	fmt.Println("  lvt new              Launch interactive app creator")
	fmt.Println("  lvt gen              Launch interactive resource builder")
	fmt.Println("  lvt gen view         Launch interactive view creator")
	fmt.Println()
	fmt.Println("Direct Mode Examples:")
	fmt.Println("  lvt new myapp")
	fmt.Println("  lvt gen users name:string email:string age:int")
	fmt.Println("  lvt gen users name email age              (types inferred)")
	fmt.Println("  lvt gen view counter")
	fmt.Println()
	fmt.Println("Migration Commands:")
	fmt.Println("  lvt migration up                          Run pending migrations")
	fmt.Println("  lvt migration down                        Rollback last migration")
	fmt.Println("  lvt migration status                      Show migration status")
	fmt.Println("  lvt migration create <name>               Create new migration file")
	fmt.Println()
	fmt.Println("Template Commands:")
	fmt.Println("  lvt template copy resource                Copy resource templates to .lvt/templates/")
	fmt.Println("  lvt template copy view                    Copy view templates to .lvt/templates/")
	fmt.Println("  lvt template copy app                     Copy app templates to .lvt/templates/")
	fmt.Println("  lvt template copy all                     Copy all templates to .lvt/templates/")
	fmt.Println()
	fmt.Println("Type Mappings:")
	fmt.Println("  string  -> Go: string,     SQL: TEXT")
	fmt.Println("  int     -> Go: int64,      SQL: INTEGER")
	fmt.Println("  bool    -> Go: bool,       SQL: BOOLEAN")
	fmt.Println("  float   -> Go: float64,    SQL: REAL")
	fmt.Println("  time    -> Go: time.Time,  SQL: DATETIME")
}

func printGenUsage() {
	fmt.Println("Usage:")
	fmt.Println("  lvt gen <resource> <field:type>...  Generate CRUD resource")
	fmt.Println("  lvt gen view <name>                 Generate view-only handler")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  lvt gen users name:string email:string")
	fmt.Println("  lvt gen view counter")
}
