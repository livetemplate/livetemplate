package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/livefir/livetemplate/cmd/lvt/commands"
	"github.com/livefir/livetemplate/cmd/lvt/internal/config"
	"github.com/livefir/livetemplate/cmd/lvt/internal/ui"
)

// Version information (can be overridden at build time with -ldflags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse global flags (--config) before command
	command, args := parseGlobalFlags(os.Args[1:])

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
	case "parse":
		err = commands.Parse(args)
	case "resource", "res":
		err = commands.Resource(args)
	case "seed":
		err = commands.Seed(args)
	case "kits", "kit":
		err = commands.Kits(args)
	case "serve", "server":
		err = commands.Serve(args)
	case "version", "--version", "-v":
		printVersion()
		return
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

func printVersion() {
	fmt.Printf("lvt version %s\n", version)

	// Try to get build info from debug.ReadBuildInfo()
	if info, ok := debug.ReadBuildInfo(); ok {
		// Get VCS info if available
		var vcsRevision, vcsTime, vcsModified string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				vcsRevision = setting.Value
			case "vcs.time":
				vcsTime = setting.Value
			case "vcs.modified":
				vcsModified = setting.Value
			}
		}

		// Show commit if we have it
		if commit != "unknown" {
			fmt.Printf("commit: %s\n", commit)
		} else if vcsRevision != "" {
			// Shorten commit hash
			if len(vcsRevision) > 12 {
				vcsRevision = vcsRevision[:12]
			}
			fmt.Printf("commit: %s\n", vcsRevision)
		}

		// Show build timestamp - this is the actual binary build time
		if date != "unknown" {
			fmt.Printf("built: %s\n", date)
		} else if vcsTime != "" {
			// Parse and format VCS time (commit time, not build time)
			if t, err := time.Parse(time.RFC3339, vcsTime); err == nil {
				fmt.Printf("commit date: %s\n", t.Format("2006-01-02 15:04:05 MST"))
			}
		}

		// Show if working directory has uncommitted changes
		if vcsModified == "true" {
			fmt.Printf("modified: true (uncommitted changes)\n")
		}

		fmt.Printf("go: %s\n", info.GoVersion)
	}

	// If no build timestamp was injected, show when this binary could have been built
	if date == "unknown" {
		fmt.Printf("\nNote: Build without timestamp. To add build info, use:\n")
		fmt.Printf("  go build -ldflags \"-X main.date=$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)\" -o lvt\n")
	}
}

func printUsage() {
	fmt.Println("LiveTemplate CLI Generator")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  lvt [--config <path>] <command> [args...] Run command with optional config file")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  lvt new [<app-name>] [--module <name>]   Create a new LiveTemplate app")
	fmt.Println("  lvt gen [<resource> <field:type>...]      Generate CRUD resource with database")
	fmt.Println("  lvt gen view [<name>]                     Generate view-only handler")
	fmt.Println("  lvt migration <command>                   Manage database migrations")
	fmt.Println("  lvt resource <command>                    Inspect resources and schemas")
	fmt.Println("  lvt seed <resource> [--count N] [--cleanup]  Generate test data")
	fmt.Println("  lvt kits <command>                        Manage CSS framework kits")
	fmt.Println("  lvt serve [options]                       Start development server with hot reload")
	fmt.Println("  lvt parse <template-file>                 Validate and analyze template file")
	fmt.Println("  lvt version                               Show version information")
	fmt.Println()
	fmt.Println("Interactive Mode (no arguments):")
	fmt.Println("  lvt new              Launch interactive app creator")
	fmt.Println("  lvt gen              Launch interactive resource builder")
	fmt.Println("  lvt gen view         Launch interactive view creator")
	fmt.Println()
	fmt.Println("Direct Mode Examples:")
	fmt.Println("  lvt new myapp")
	fmt.Println("  lvt new myapp --module github.com/user/myapp")
	fmt.Println("  lvt gen users name:string email:string age:int")
	fmt.Println("  lvt gen users name email age              (types inferred)")
	fmt.Println("  lvt gen users name email --css bulma      (with CSS framework)")
	fmt.Println("  lvt gen products name price --mode single (single-page app mode)")
	fmt.Println("  lvt gen view counter")
	fmt.Println("  lvt gen view counter --css pico           (view with CSS framework)")
	fmt.Println()
	fmt.Println("Migration Commands:")
	fmt.Println("  lvt migration up                          Run pending migrations")
	fmt.Println("  lvt migration down                        Rollback last migration")
	fmt.Println("  lvt migration status                      Show migration status")
	fmt.Println("  lvt migration create <name>               Create new migration file")
	fmt.Println()
	fmt.Println("Resource Commands:")
	fmt.Println("  lvt resource list                         List all available resources")
	fmt.Println("  lvt resource describe <name>              Show detailed schema for a resource")
	fmt.Println()
	fmt.Println("Seed Commands:")
	fmt.Println("  lvt seed tasks --count 50                 Generate 50 test records")
	fmt.Println("  lvt seed tasks --cleanup                  Remove all test data")
	fmt.Println("  lvt seed tasks --count 30 --cleanup       Cleanup then seed 30 new records")
	fmt.Println()
	fmt.Println("Kits Commands:")
	fmt.Println("  lvt kits list                             List all available kits")
	fmt.Println("  lvt kits list --filter local              List only local kits")
	fmt.Println("  lvt kits list --format table              Output as table (default)")
	fmt.Println("  lvt kits create mykit                     Create a new CSS framework kit")
	fmt.Println("  lvt kits info tailwind                    Show kit details")
	fmt.Println("  lvt kits validate <path>                  Validate kit implementation")
	fmt.Println()
	fmt.Println("Serve Commands:")
	fmt.Println("  lvt serve                                 Start dev server (auto-detect mode)")
	fmt.Println("  lvt serve --port 8080                     Start on custom port")
	fmt.Println("  lvt serve --mode component                Force component development mode")
	fmt.Println("  lvt serve --mode kit                      Force kit development mode")
	fmt.Println("  lvt serve --mode app                      Force app development mode")
	fmt.Println("  lvt serve --no-browser                    Don't open browser automatically")
	fmt.Println("  lvt serve --no-reload                     Disable live reload")
	fmt.Println()
	fmt.Println("Type Mappings:")
	fmt.Println("  string  -> Go: string,     SQL: TEXT")
	fmt.Println("  int     -> Go: int64,      SQL: INTEGER")
	fmt.Println("  bool    -> Go: bool,       SQL: BOOLEAN")
	fmt.Println("  float   -> Go: float64,    SQL: REAL")
	fmt.Println("  time    -> Go: time.Time,  SQL: DATETIME")
	fmt.Println()
	fmt.Println("CSS Framework Options:")
	fmt.Println("  tailwind (default) - Tailwind CSS v4 utility-first framework")
	fmt.Println("  bulma              - Bulma component-based framework")
	fmt.Println("  pico               - Pico CSS semantic/classless framework")
	fmt.Println("  none               - No CSS framework (pure HTML)")
	fmt.Println()
	fmt.Println("App Mode Options:")
	fmt.Println("  multi (default)    - Multi-page app with full HTML layout")
	fmt.Println("  single             - Single-page app (components only, no layout)")
	fmt.Println()
	fmt.Println("Documentation:")
	fmt.Println("  Full documentation available at docs/ directory")
	fmt.Println("  - docs/user-guide.md           Getting started and usage")
	fmt.Println("  - docs/component-development.md  Creating custom components")
	fmt.Println("  - docs/kit-development.md       Creating custom kits")
	fmt.Println("  - docs/serve-guide.md           Development server guide")
	fmt.Println("  - docs/api-reference.md         Complete API reference")
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

// parseGlobalFlags parses global flags like --config and returns the command and remaining args
func parseGlobalFlags(args []string) (string, []string) {
	var filteredArgs []string
	var command string

	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			// Set the custom config path
			config.SetConfigPath(args[i+1])
			i++ // Skip the next argument (the path)
			continue
		}

		// First non-flag argument is the command
		if command == "" {
			command = args[i]
		} else {
			filteredArgs = append(filteredArgs, args[i])
		}
	}

	return command, filteredArgs
}
